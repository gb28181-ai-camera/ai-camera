-- ================================================================
-- 10-IOT管理 - DDL（6 张表 + ac_device 扩展）
-- AI Camera 智能监控平台 · v0.3.0
-- 覆盖：IOT产品 / 服务组 / 服务项 / 产品-服务组绑定 / 设备账号 / 设备事件
--
-- 设计依据：docs/需求文档/12-IOT管理模块.md
-- 契约：docs/开发文档/后端开发/00-接口契约.md §10.14（Web）/ §10.15（设备端）
--
-- 说明：
--   1. 【重要】设备**复用 ac_device**，不新建 IOT 设备表。
--      设备已被风险/告警/巡检/升级/国标等模块引用，再建一张表会产生
--      两套主键与两套生命周期。本文件只给 ac_device 追加 3 列。
--   2. 【重要】服务组凭据（username / password）是**组级**而不是设备级。
--      同一产品下的设备共用一组服务凭据，由平台自动生成；
--      Web 读接口脱敏（****），设备端 OTA 才返回明文。
--   3. ac_iot_product_service / ac_iot_service_item 为可整体替换的关联/明细表，
--      应用层替换时使用**硬删除**：软删除的行仍会占用唯一索引，
--      会导致同一服务组/协议解绑后无法重新绑定。
--   4. ac_iot_device_event 为追加写表，不做更新与删除。
-- ================================================================


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
-- ac_iot_service_group 服务组（组级凭据）
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
    username          VARCHAR(100)  NOT NULL,
    password          VARCHAR(255)  NOT NULL,
    description       TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_service_group_code       ON ac_iot_service_group (code);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_service_group_username   ON ac_iot_service_group (username);
CREATE INDEX        IF NOT EXISTS idx_ac_iot_service_group_deleted_at ON ac_iot_service_group (deleted_at);

COMMENT ON TABLE  ac_iot_service_group            IS 'IOT服务组';
COMMENT ON COLUMN ac_iot_service_group.id         IS '主键';
COMMENT ON COLUMN ac_iot_service_group.deleted_at IS '软删除时间';
COMMENT ON COLUMN ac_iot_service_group.created_by IS '创建人';
COMMENT ON COLUMN ac_iot_service_group.created_dt IS '创建时间';
COMMENT ON COLUMN ac_iot_service_group.updated_by IS '更新人';
COMMENT ON COLUMN ac_iot_service_group.updated_dt IS '更新时间';
COMMENT ON COLUMN ac_iot_service_group.active     IS '是否启用';
COMMENT ON COLUMN ac_iot_service_group.name       IS '服务组名称';
COMMENT ON COLUMN ac_iot_service_group.code       IS '服务组编码(唯一)';
COMMENT ON COLUMN ac_iot_service_group.type       IS '类型 gb28181/webrtc/aichat';
COMMENT ON COLUMN ac_iot_service_group.username   IS '组级登录用户名(自动生成,唯一)';
COMMENT ON COLUMN ac_iot_service_group.password   IS '组级登录密码(自动生成,敏感,接口返回脱敏)';
COMMENT ON COLUMN ac_iot_service_group.description IS '描述';


-- ----------------------------------------------------------------
-- ac_iot_service_item 服务项（组内每种协议唯一）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_iot_service_item (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    group_id          BIGINT        NOT NULL,
    protocol          VARCHAR(20)   NOT NULL,
    host              VARCHAR(255),
    port              BIGINT,
    path              VARCHAR(500),
    extra             TEXT,
    sequence          BIGINT        DEFAULT 10
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_service_item_group_protocol
    ON ac_iot_service_item (group_id, protocol);
CREATE INDEX IF NOT EXISTS idx_ac_iot_service_item_group_id   ON ac_iot_service_item (group_id);
CREATE INDEX IF NOT EXISTS idx_ac_iot_service_item_deleted_at ON ac_iot_service_item (deleted_at);

COMMENT ON TABLE  ac_iot_service_item            IS 'IOT服务项';
COMMENT ON COLUMN ac_iot_service_item.id         IS '主键';
COMMENT ON COLUMN ac_iot_service_item.deleted_at IS '软删除时间';
COMMENT ON COLUMN ac_iot_service_item.created_by IS '创建人';
COMMENT ON COLUMN ac_iot_service_item.created_dt IS '创建时间';
COMMENT ON COLUMN ac_iot_service_item.updated_by IS '更新人';
COMMENT ON COLUMN ac_iot_service_item.updated_dt IS '更新时间';
COMMENT ON COLUMN ac_iot_service_item.active     IS '是否启用';
COMMENT ON COLUMN ac_iot_service_item.group_id   IS '服务组ID';
COMMENT ON COLUMN ac_iot_service_item.protocol   IS '协议 SIP/RTSP/WebRTC/MQTT/WebSocket';
COMMENT ON COLUMN ac_iot_service_item.host       IS '服务地址';
COMMENT ON COLUMN ac_iot_service_item.port       IS '服务端口';
COMMENT ON COLUMN ac_iot_service_item.path       IS '服务路径';
COMMENT ON COLUMN ac_iot_service_item.extra      IS '扩展配置(JSON字符串)';
COMMENT ON COLUMN ac_iot_service_item.sequence   IS '排序';

-- 协议白名单由应用层校验（避免数据库 CHECK 约束随业务演进频繁变更）：
--   gb28181 → SIP / RTSP
--   webrtc  → WebRTC / MQTT / RTSP
--   aichat  → WebSocket


-- ----------------------------------------------------------------
-- ac_iot_product_service 产品-服务组绑定（纯关联表，整体替换）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_iot_product_service (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    product_id        BIGINT        NOT NULL,
    service_group_id  BIGINT        NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_product_service
    ON ac_iot_product_service (product_id, service_group_id);
CREATE INDEX IF NOT EXISTS idx_ac_iot_product_service_group_id ON ac_iot_product_service (service_group_id);
CREATE INDEX IF NOT EXISTS idx_ac_iot_product_service_deleted_at ON ac_iot_product_service (deleted_at);

COMMENT ON TABLE  ac_iot_product_service                  IS 'IOT产品-服务组绑定';
COMMENT ON COLUMN ac_iot_product_service.id               IS '主键';
COMMENT ON COLUMN ac_iot_product_service.deleted_at       IS '软删除时间';
COMMENT ON COLUMN ac_iot_product_service.created_by       IS '创建人';
COMMENT ON COLUMN ac_iot_product_service.created_dt       IS '创建时间';
COMMENT ON COLUMN ac_iot_product_service.updated_by       IS '更新人';
COMMENT ON COLUMN ac_iot_product_service.updated_dt       IS '更新时间';
COMMENT ON COLUMN ac_iot_product_service.product_id       IS '产品ID';
COMMENT ON COLUMN ac_iot_product_service.service_group_id IS '服务组ID';


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
    activated_at      TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_device_account_username    ON ac_iot_device_account (username);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_device_account_share_token ON ac_iot_device_account (share_token);
CREATE INDEX        IF NOT EXISTS idx_ac_iot_device_account_device      ON ac_iot_device_account (device_id);
CREATE INDEX        IF NOT EXISTS idx_ac_iot_device_account_deleted_at  ON ac_iot_device_account (deleted_at);

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
COMMENT ON COLUMN ac_iot_device_account.share_token   IS '分享令牌(可空,唯一)';
COMMENT ON COLUMN ac_iot_device_account.activated_at  IS '激活时间';


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
COMMENT ON COLUMN ac_iot_device_event.event      IS 'ota/activate/account-create/secret-reset';
COMMENT ON COLUMN ac_iot_device_event.detail     IS '事件详情(JSON字符串)';
COMMENT ON COLUMN ac_iot_device_event.ip         IS '来源IP';
