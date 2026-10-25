-- ================================================================
-- 10-IOT管理 - DDL（7 张表 + coturn userdb + ac_device 扩展）
-- AI Camera 智能监控平台 · v0.3.0
-- 覆盖：IOT产品 / 服务组 / 服务 / 产品-服务组绑定 / 产品-服务凭据 /
--       设备账号 / 设备事件 / turnusers_lt
--
-- 设计依据：docs/需求文档/12-IOT管理模块.md
-- 契约：docs/开发文档/后端开发/00-接口契约.md §10.14（Web）/ §10.15（设备端）
--
-- 说明：
--   1. 【重要】设备**复用 ac_device**，不新建 IOT 设备表。
--      设备已被风险/告警/巡检/升级/国标等模块引用，再建一张表会产生
--      两套主键与两套生命周期。本文件只给 ac_device 追加 3 列。
--   2. 【重要】服务组是系统预置的三条分类数据（gb28181 / webrtc / aichat），
--      组本身**不持有凭据**；凭据落在「产品 × 服务」的 ac_iot_product_service。
--   3. ac_iot_service / ac_iot_product_service_group / ac_iot_product_service
--      均为可整体替换的关联/明细表，应用层替换时使用**硬删除**：
--      软删除的行仍会占用唯一索引。
--   4. ac_iot_device_event 为追加写表，不做更新与删除。
--   5. turnusers_lt 与 coturn `--userdb` 共用，由后端按产品×TURN 服务同步。
-- ================================================================


-- ================================================================
-- 第 -1 节：v0.3.0 结构迁移（仅对已存在旧结构的库生效）
-- ================================================================
-- 旧版：组级凭据 + ac_iot_service_item + ac_iot_product_service(产品-服务组)
-- 新版：组无凭据 + ac_iot_service + ac_iot_product_service_group + ac_iot_product_service(凭据)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_name = 'ac_iot_service_group' AND column_name = 'username'
    ) THEN
        ALTER TABLE ac_iot_service_group DROP COLUMN username;
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_name = 'ac_iot_service_group' AND column_name = 'password'
    ) THEN
        ALTER TABLE ac_iot_service_group DROP COLUMN password;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'ac_iot_service_item')
       AND NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'ac_iot_service') THEN
        ALTER TABLE ac_iot_service_item RENAME TO ac_iot_service;
        ALTER TABLE ac_iot_service ADD COLUMN IF NOT EXISTS code VARCHAR(50);
        ALTER TABLE ac_iot_service ADD COLUMN IF NOT EXISTS name VARCHAR(100);
        UPDATE ac_iot_service SET code = lower(protocol) WHERE code IS NULL;
        UPDATE ac_iot_service SET name = protocol WHERE name IS NULL;
        ALTER TABLE ac_iot_service ALTER COLUMN code SET NOT NULL;
        ALTER TABLE ac_iot_service ALTER COLUMN name SET NOT NULL;
        DROP INDEX IF EXISTS idx_ac_iot_service_item_group_protocol;
        ALTER TABLE ac_iot_service DROP CONSTRAINT IF EXISTS ac_iot_service_item_group_id_protocol_key;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_name = 'ac_iot_product_service' AND column_name = 'service_group_id'
    ) AND NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'ac_iot_product_service_group') THEN
        ALTER TABLE ac_iot_product_service RENAME TO ac_iot_product_service_group;
    END IF;
END
$$;


-- ================================================================
-- 第 0 节：ac_device 扩展（IOT 产品归属与设备端接入）
-- ================================================================
-- product_id        设备属于哪个 IOT 产品 → OTA 时据此下发服务授权
-- device_secret     设备端 OTA / 激活的唯一凭据（无 JWT），明文仅展示一次
-- activated_at      主账号首次激活时间（与 ac_iot_device_account.activated_at 对应）
-- ----------------------------------------------------------------

ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS product_id    BIGINT;
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS device_secret VARCHAR(255);
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS activated_at  TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_ac_device_product_id ON ac_device (product_id);

COMMENT ON COLUMN ac_device.product_id    IS 'IOT产品ID';
COMMENT ON COLUMN ac_device.device_secret IS '设备接入密钥(敏感,接口返回时脱敏,明文仅展示一次)';
COMMENT ON COLUMN ac_device.activated_at  IS 'IOT主账号激活时间';


-- ----------------------------------------------------------------
-- ac_iot_product IOT 产品
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_iot_product (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    sequence          BIGINT        DEFAULT 10,
    name              VARCHAR(255)  NOT NULL,
    code              VARCHAR(100)  NOT NULL,
    category          VARCHAR(100),
    description       TEXT,
    auto_network      BOOLEAN       DEFAULT FALSE,
    net_config_mode   VARCHAR(20)   DEFAULT 'bluetooth',
    allow_share       BOOLEAN       DEFAULT FALSE,
    share_qr_content  TEXT
);

-- 已有库补列：CREATE TABLE IF NOT EXISTS 不会修改既有表结构。
-- 默认值 security 会把历史产品按安防相机解释（此前只有安防相机一款产品）。

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_product_code       ON ac_iot_product (code);
CREATE INDEX        IF NOT EXISTS idx_ac_iot_product_deleted_at ON ac_iot_product (deleted_at);

COMMENT ON TABLE  ac_iot_product                 IS 'IOT产品';
COMMENT ON COLUMN ac_iot_product.id              IS '主键';
COMMENT ON COLUMN ac_iot_product.deleted_at      IS '软删除时间';
COMMENT ON COLUMN ac_iot_product.created_by      IS '创建人';
COMMENT ON COLUMN ac_iot_product.created_dt      IS '创建时间';
COMMENT ON COLUMN ac_iot_product.updated_by      IS '更新人';
COMMENT ON COLUMN ac_iot_product.updated_dt      IS '更新时间';
COMMENT ON COLUMN ac_iot_product.active          IS '是否启用';
COMMENT ON COLUMN ac_iot_product.sequence        IS '排序';
COMMENT ON COLUMN ac_iot_product.name            IS '产品名称';
COMMENT ON COLUMN ac_iot_product.code            IS '产品编码(唯一)';
COMMENT ON COLUMN ac_iot_product.category        IS '产品分类';
COMMENT ON COLUMN ac_iot_product.description     IS '产品描述';
COMMENT ON COLUMN ac_iot_product.auto_network    IS '是否自动联网(4G设备)';
COMMENT ON COLUMN ac_iot_product.net_config_mode IS '配网方式 bluetooth/ap/qrcode';
COMMENT ON COLUMN ac_iot_product.allow_share     IS '是否允许分享';
COMMENT ON COLUMN ac_iot_product.share_qr_content IS '分享二维码内容(JSON字符串)';


-- ----------------------------------------------------------------
-- ac_iot_service_group 服务组（固定三条分类，不持有凭据）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_iot_service_group (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    name              VARCHAR(255)  NOT NULL,
    code              VARCHAR(100)  NOT NULL,
    type              VARCHAR(20)   NOT NULL,
    description       TEXT
);

-- 说明：服务组是**系统预置的三条固定数据**（code = gb28181 / webrtc / aichat），
-- 由初始化脚本写入；编码与类型不可修改，不开放新增/删除。
-- 凭据不在组上，而在「产品 × 服务」（ac_iot_product_service）。

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_service_group_code       ON ac_iot_service_group (code);
CREATE INDEX        IF NOT EXISTS idx_ac_iot_service_group_deleted_at ON ac_iot_service_group (deleted_at);

COMMENT ON TABLE  ac_iot_service_group            IS 'IOT服务组(固定三条)';
COMMENT ON COLUMN ac_iot_service_group.id         IS '主键';
COMMENT ON COLUMN ac_iot_service_group.deleted_at IS '软删除时间';
COMMENT ON COLUMN ac_iot_service_group.created_by IS '创建人';
COMMENT ON COLUMN ac_iot_service_group.created_dt IS '创建时间';
COMMENT ON COLUMN ac_iot_service_group.updated_by IS '更新人';
COMMENT ON COLUMN ac_iot_service_group.updated_dt IS '更新时间';
COMMENT ON COLUMN ac_iot_service_group.active     IS '是否启用';
COMMENT ON COLUMN ac_iot_service_group.name       IS '服务组名称';
COMMENT ON COLUMN ac_iot_service_group.code       IS '服务组编码(唯一,固定)';
COMMENT ON COLUMN ac_iot_service_group.type       IS '类型 gb28181/webrtc/aichat';
COMMENT ON COLUMN ac_iot_service_group.description IS '描述';


-- ----------------------------------------------------------------
-- ac_iot_service 服务（组内具体接入项，按 code 唯一）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_iot_service (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    group_id          BIGINT        NOT NULL,
    code              VARCHAR(50)   NOT NULL,
    name              VARCHAR(100)  NOT NULL,
    protocol          VARCHAR(20)   NOT NULL,
    host              VARCHAR(255),
    port              BIGINT,
    path              VARCHAR(500),
    extra             TEXT,
    sequence          BIGINT        DEFAULT 10
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_service_grp_code
    ON ac_iot_service (group_id, code);
CREATE INDEX IF NOT EXISTS idx_ac_iot_service_group_id   ON ac_iot_service (group_id);
CREATE INDEX IF NOT EXISTS idx_ac_iot_service_deleted_at ON ac_iot_service (deleted_at);

COMMENT ON TABLE  ac_iot_service            IS 'IOT服务(组内具体接入项)';
COMMENT ON COLUMN ac_iot_service.id         IS '主键';
COMMENT ON COLUMN ac_iot_service.deleted_at IS '软删除时间';
COMMENT ON COLUMN ac_iot_service.created_by IS '创建人';
COMMENT ON COLUMN ac_iot_service.created_dt IS '创建时间';
COMMENT ON COLUMN ac_iot_service.updated_by IS '更新人';
COMMENT ON COLUMN ac_iot_service.updated_dt IS '更新时间';
COMMENT ON COLUMN ac_iot_service.active     IS '是否启用';
COMMENT ON COLUMN ac_iot_service.group_id   IS '服务组ID';
COMMENT ON COLUMN ac_iot_service.code       IS '服务编码(组内唯一)';
COMMENT ON COLUMN ac_iot_service.name       IS '服务名称';
COMMENT ON COLUMN ac_iot_service.protocol   IS '协议 SIP/RTSP/WebRTC/MQTT/WebSocket/TURN';
COMMENT ON COLUMN ac_iot_service.host       IS '服务地址';
COMMENT ON COLUMN ac_iot_service.port       IS '服务端口';
COMMENT ON COLUMN ac_iot_service.path       IS '服务路径';
COMMENT ON COLUMN ac_iot_service.extra      IS '扩展配置(JSON字符串)';
COMMENT ON COLUMN ac_iot_service.sequence   IS '排序';

-- 协议白名单由应用层校验（避免数据库 CHECK 约束随业务演进频繁变更）：
--   gb28181 → SIP / RTSP
--   webrtc  → WebRTC / MQTT / RTSP / TURN
--   aichat  → WebSocket


-- ----------------------------------------------------------------
-- ac_iot_product_service_group 产品-服务组绑定（纯关联表，整体替换）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_iot_product_service_group (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    product_id        BIGINT        NOT NULL,
    service_group_id  BIGINT        NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_product_service_group
    ON ac_iot_product_service_group (product_id, service_group_id);
CREATE INDEX IF NOT EXISTS idx_ac_iot_product_service_group_group_id ON ac_iot_product_service_group (service_group_id);
CREATE INDEX IF NOT EXISTS idx_ac_iot_product_service_group_deleted_at ON ac_iot_product_service_group (deleted_at);

COMMENT ON TABLE  ac_iot_product_service_group                  IS 'IOT产品-服务组绑定';
COMMENT ON COLUMN ac_iot_product_service_group.id               IS '主键';
COMMENT ON COLUMN ac_iot_product_service_group.deleted_at       IS '软删除时间';
COMMENT ON COLUMN ac_iot_product_service_group.product_id       IS '产品ID';
COMMENT ON COLUMN ac_iot_product_service_group.service_group_id IS '服务组ID';


-- ----------------------------------------------------------------
-- ac_iot_product_service 产品-服务凭据（产品 × 服务，含鉴权）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_iot_product_service (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    product_id        BIGINT        NOT NULL,
    service_id        BIGINT        NOT NULL,
    username          VARCHAR(100)  NOT NULL,
    password          VARCHAR(255)  NOT NULL,
    extra             TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_product_service
    ON ac_iot_product_service (product_id, service_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_product_service_username
    ON ac_iot_product_service (username);
CREATE INDEX IF NOT EXISTS idx_ac_iot_product_service_product_id ON ac_iot_product_service (product_id);
CREATE INDEX IF NOT EXISTS idx_ac_iot_product_service_service_id ON ac_iot_product_service (service_id);
CREATE INDEX IF NOT EXISTS idx_ac_iot_product_service_deleted_at ON ac_iot_product_service (deleted_at);

COMMENT ON TABLE  ac_iot_product_service            IS 'IOT产品-服务凭据';
COMMENT ON COLUMN ac_iot_product_service.id         IS '主键';
COMMENT ON COLUMN ac_iot_product_service.deleted_at IS '软删除时间';
COMMENT ON COLUMN ac_iot_product_service.active     IS '是否启用';
COMMENT ON COLUMN ac_iot_product_service.product_id IS '产品ID';
COMMENT ON COLUMN ac_iot_product_service.service_id IS '服务ID';
COMMENT ON COLUMN ac_iot_product_service.username   IS '服务登录用户名(自动生成,全局唯一)';
COMMENT ON COLUMN ac_iot_product_service.password   IS '服务登录密码(自动生成,敏感,接口返回脱敏)';
COMMENT ON COLUMN ac_iot_product_service.extra      IS '扩展配置(JSON字符串)';


-- ----------------------------------------------------------------
-- turnusers_lt coturn SQL userdb（与 coturn --userdb 共用）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS turnusers_lt (
    realm   VARCHAR(127) DEFAULT '',
    name    VARCHAR(512) DEFAULT '',
    hmackey VARCHAR(128) DEFAULT '',
    PRIMARY KEY (realm, name)
);

COMMENT ON TABLE turnusers_lt IS 'coturn 长期凭据用户表(MD5(username:realm:password))';

-- coturn SQL userdb 的 REST 密钥表（保留空表，满足 coturn 启动时结构检查）。
CREATE TABLE IF NOT EXISTS turn_secret (
    realm VARCHAR(127) DEFAULT '',
    value VARCHAR(256) DEFAULT '',
    PRIMARY KEY (realm, value)
);

COMMENT ON TABLE turn_secret IS 'coturn REST 短期凭据密钥表(userdb 结构兼容)';


-- ----------------------------------------------------------------
-- ac_iot_device_account 设备账号（主账号 / 分享账号）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_iot_device_account (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    device_id         BIGINT        NOT NULL,
    username          VARCHAR(100)  NOT NULL,
    password_hash     VARCHAR(255)  NOT NULL,
    role              VARCHAR(20)   NOT NULL DEFAULT 'shared',
    nickname          VARCHAR(100),
    phone             VARCHAR(50),
    status            VARCHAR(20)   NOT NULL DEFAULT 'active',
    share_token       VARCHAR(100),
    activated_at      TIMESTAMPTZ,
    platform_user_id  BIGINT
);

-- v0.3.1：老库补列（CREATE TABLE IF NOT EXISTS 不会为已存在的表加列）。
-- 分享令牌尚未认领时 platform_user_id 为 NULL；App 设备列表按此列过滤。
ALTER TABLE ac_iot_device_account ADD COLUMN IF NOT EXISTS platform_user_id BIGINT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_device_account_username    ON ac_iot_device_account (username);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_device_account_share_token ON ac_iot_device_account (share_token);
CREATE INDEX        IF NOT EXISTS idx_ac_iot_device_account_device      ON ac_iot_device_account (device_id);
CREATE INDEX        IF NOT EXISTS idx_ac_iot_device_account_deleted_at  ON ac_iot_device_account (deleted_at);
CREATE INDEX        IF NOT EXISTS idx_ac_iot_device_account_platform_user ON ac_iot_device_account (platform_user_id);

COMMENT ON TABLE  ac_iot_device_account               IS 'IOT设备账号';
COMMENT ON COLUMN ac_iot_device_account.id            IS '主键';
COMMENT ON COLUMN ac_iot_device_account.deleted_at    IS '软删除时间';
COMMENT ON COLUMN ac_iot_device_account.created_by    IS '创建人';
COMMENT ON COLUMN ac_iot_device_account.created_dt    IS '创建时间';
COMMENT ON COLUMN ac_iot_device_account.updated_by    IS '更新人';
COMMENT ON COLUMN ac_iot_device_account.updated_dt    IS '更新时间';
COMMENT ON COLUMN ac_iot_device_account.device_id     IS '设备ID(ac_device.id)';
COMMENT ON COLUMN ac_iot_device_account.username      IS '登录名(全局唯一)';
COMMENT ON COLUMN ac_iot_device_account.password_hash IS '密码哈希(bcrypt)';
COMMENT ON COLUMN ac_iot_device_account.role          IS '角色 owner(主账号)/shared(分享账号)';
COMMENT ON COLUMN ac_iot_device_account.nickname      IS '昵称';
COMMENT ON COLUMN ac_iot_device_account.phone         IS '手机号';
COMMENT ON COLUMN ac_iot_device_account.status        IS '状态 active/disabled';
COMMENT ON COLUMN ac_iot_device_account.share_token      IS '分享令牌(可空,唯一,认领后清空)';
COMMENT ON COLUMN ac_iot_device_account.activated_at     IS '激活时间';
COMMENT ON COLUMN ac_iot_device_account.platform_user_id IS '绑定的平台用户ID(ac_user.id,可空)';


-- ----------------------------------------------------------------
-- ac_iot_device_event 设备接入事件（追加写）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_iot_device_event (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    device_id         BIGINT        NOT NULL,
    event             VARCHAR(30)   NOT NULL,
    detail            TEXT,
    ip                VARCHAR(64)
);

CREATE INDEX IF NOT EXISTS idx_ac_iot_device_event_device ON ac_iot_device_event (device_id);
CREATE INDEX IF NOT EXISTS idx_ac_iot_device_event_dt     ON ac_iot_device_event (created_dt);

COMMENT ON TABLE  ac_iot_device_event            IS 'IOT设备接入事件(追加写)';
COMMENT ON COLUMN ac_iot_device_event.id         IS '主键';
COMMENT ON COLUMN ac_iot_device_event.deleted_at IS '软删除时间';
COMMENT ON COLUMN ac_iot_device_event.created_by IS '创建人';
COMMENT ON COLUMN ac_iot_device_event.created_dt IS '创建时间';
COMMENT ON COLUMN ac_iot_device_event.updated_by IS '更新人';
COMMENT ON COLUMN ac_iot_device_event.updated_dt IS '更新时间';
COMMENT ON COLUMN ac_iot_device_event.device_id  IS '设备ID(ac_device.id)';
COMMENT ON COLUMN ac_iot_device_event.event      IS 'ota/activate/account-create/secret-reset/unbind';
COMMENT ON COLUMN ac_iot_device_event.detail     IS '事件详情(JSON字符串)';
COMMENT ON COLUMN ac_iot_device_event.ip         IS '来源IP';
