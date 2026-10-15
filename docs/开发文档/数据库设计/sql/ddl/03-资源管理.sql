-- ================================================================
-- 03-资源管理 - DDL（9 张表）
-- AI Camera 智能监控平台 · v0.1.0
-- 覆盖：设备模式 / 固件类型 / 固件 / AI模型 / 物联网卡 / 设备
--       / 物联网卡更换记录 / 语音配置 / 设备运行历史
--
-- 说明：
--   1. ac_firmware_type / ac_firmware / ac_ai_model 三张升级域字典表
--      同时被 ac_device 引用（当前固件/当前模型），因此归入本文件，
--      在 02-表关系全景图.md 中同时计入「资源管理」与「升级管理」。
--   2. ac_device 的运行时指标（电量/电压/存储/在线时间）是设备心跳的
--      最新快照；时序历史写入 ac_device_history。
--   3. ac_iot_card_replace_log / ac_device_history 为追加写入表，不做修改。
-- ================================================================

-- ----------------------------------------------------------------
-- ac_device_mode 设备工作模式（内置充电/全开/省电）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_device_mode (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    sequence          BIGINT        DEFAULT 10,
    name              VARCHAR(255)  NOT NULL,
    code              VARCHAR(50),
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_device_mode_deleted_at ON ac_device_mode (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_device_mode_dept_id    ON ac_device_mode (dept_id);

COMMENT ON TABLE  ac_device_mode             IS '设备工作模式';
COMMENT ON COLUMN ac_device_mode.id          IS '主键';
COMMENT ON COLUMN ac_device_mode.deleted_at  IS '软删除时间';
COMMENT ON COLUMN ac_device_mode.created_by  IS '创建人';
COMMENT ON COLUMN ac_device_mode.created_dt  IS '创建时间';
COMMENT ON COLUMN ac_device_mode.updated_by  IS '更新人';
COMMENT ON COLUMN ac_device_mode.updated_dt  IS '更新时间';
COMMENT ON COLUMN ac_device_mode.active      IS '是否启用';
COMMENT ON COLUMN ac_device_mode.sequence    IS '排序';
COMMENT ON COLUMN ac_device_mode.name        IS '模式名称';
COMMENT ON COLUMN ac_device_mode.code        IS '模式编码';
COMMENT ON COLUMN ac_device_mode.remark      IS '备注';
COMMENT ON COLUMN ac_device_mode.dept_id     IS '所属部门(数据权限)';

-- ----------------------------------------------------------------
-- ac_firmware_type 固件类型
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_firmware_type (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    sequence          BIGINT        DEFAULT 10,
    name              VARCHAR(255)  NOT NULL,
    code              VARCHAR(50),
    description       TEXT,
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_firmware_type_deleted_at ON ac_firmware_type (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_firmware_type_dept_id    ON ac_firmware_type (dept_id);

COMMENT ON TABLE  ac_firmware_type             IS '固件类型';
COMMENT ON COLUMN ac_firmware_type.id          IS '主键';
COMMENT ON COLUMN ac_firmware_type.deleted_at  IS '软删除时间';
COMMENT ON COLUMN ac_firmware_type.created_by  IS '创建人';
COMMENT ON COLUMN ac_firmware_type.created_dt  IS '创建时间';
COMMENT ON COLUMN ac_firmware_type.updated_by  IS '更新人';
COMMENT ON COLUMN ac_firmware_type.updated_dt  IS '更新时间';
COMMENT ON COLUMN ac_firmware_type.active      IS '是否启用';
COMMENT ON COLUMN ac_firmware_type.sequence    IS '排序';
COMMENT ON COLUMN ac_firmware_type.name        IS '固件类型';
COMMENT ON COLUMN ac_firmware_type.code        IS '类型编码';
COMMENT ON COLUMN ac_firmware_type.description IS '描述';
COMMENT ON COLUMN ac_firmware_type.dept_id     IS '所属部门(数据权限)';

-- ----------------------------------------------------------------
-- ac_firmware 固件
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_firmware (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    name              VARCHAR(255)  NOT NULL,
    version           VARCHAR(50)   NOT NULL,
    firmware_type_id  BIGINT,
    file_url          VARCHAR(500),
    file_size         BIGINT,
    md5               VARCHAR(64),
    release_dt        TIMESTAMPTZ,
    description       TEXT,
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_firmware_deleted_at       ON ac_firmware (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_firmware_firmware_type_id ON ac_firmware (firmware_type_id);
CREATE INDEX IF NOT EXISTS idx_ac_firmware_dept_id          ON ac_firmware (dept_id);

COMMENT ON TABLE  ac_firmware                  IS '固件';
COMMENT ON COLUMN ac_firmware.id               IS '主键';
COMMENT ON COLUMN ac_firmware.deleted_at       IS '软删除时间';
COMMENT ON COLUMN ac_firmware.created_by       IS '创建人';
COMMENT ON COLUMN ac_firmware.created_dt       IS '创建时间';
COMMENT ON COLUMN ac_firmware.updated_by       IS '更新人';
COMMENT ON COLUMN ac_firmware.updated_dt       IS '更新时间';
COMMENT ON COLUMN ac_firmware.active           IS '是否启用';
COMMENT ON COLUMN ac_firmware.name             IS '固件名称';
COMMENT ON COLUMN ac_firmware.version          IS '版本号';
COMMENT ON COLUMN ac_firmware.firmware_type_id IS '固件类型ID';
COMMENT ON COLUMN ac_firmware.file_url         IS '固件文件地址';
COMMENT ON COLUMN ac_firmware.file_size        IS '文件大小(字节)';
COMMENT ON COLUMN ac_firmware.md5              IS 'MD5校验值';
COMMENT ON COLUMN ac_firmware.release_dt       IS '发布时间';
COMMENT ON COLUMN ac_firmware.description      IS '更新说明';
COMMENT ON COLUMN ac_firmware.dept_id          IS '所属部门(数据权限)';

-- ----------------------------------------------------------------
-- ac_ai_model AI 识别模型（必须绑定适用的固件）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_ai_model (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    name              VARCHAR(255)  NOT NULL,
    version           VARCHAR(50)   NOT NULL,
    firmware_id       BIGINT,
    file_url          VARCHAR(500),
    file_size         BIGINT,
    md5               VARCHAR(64),
    release_dt        TIMESTAMPTZ,
    description       TEXT,
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_ai_model_deleted_at  ON ac_ai_model (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_ai_model_firmware_id ON ac_ai_model (firmware_id);
CREATE INDEX IF NOT EXISTS idx_ac_ai_model_dept_id     ON ac_ai_model (dept_id);

COMMENT ON TABLE  ac_ai_model               IS 'AI 识别模型';
COMMENT ON COLUMN ac_ai_model.id            IS '主键';
COMMENT ON COLUMN ac_ai_model.deleted_at    IS '软删除时间';
COMMENT ON COLUMN ac_ai_model.created_by    IS '创建人';
COMMENT ON COLUMN ac_ai_model.created_dt    IS '创建时间';
COMMENT ON COLUMN ac_ai_model.updated_by    IS '更新人';
COMMENT ON COLUMN ac_ai_model.updated_dt    IS '更新时间';
COMMENT ON COLUMN ac_ai_model.active        IS '是否启用';
COMMENT ON COLUMN ac_ai_model.name          IS '模型名称';
COMMENT ON COLUMN ac_ai_model.version       IS '版本号';
COMMENT ON COLUMN ac_ai_model.firmware_id   IS '适用固件ID';
COMMENT ON COLUMN ac_ai_model.file_url      IS '模型文件地址';
COMMENT ON COLUMN ac_ai_model.file_size     IS '文件大小(字节)';
COMMENT ON COLUMN ac_ai_model.md5           IS 'MD5校验值';
COMMENT ON COLUMN ac_ai_model.release_dt    IS '发布时间';
COMMENT ON COLUMN ac_ai_model.description   IS '更新说明';
COMMENT ON COLUMN ac_ai_model.dept_id       IS '所属部门(数据权限)';

-- ----------------------------------------------------------------
-- ac_iot_card 物联网卡
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_iot_card (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    name              VARCHAR(255)  NOT NULL,
    card_no           VARCHAR(100)  NOT NULL,
    region_id         BIGINT,
    device_id         BIGINT,
    traffic_total     BIGINT,
    traffic_used      BIGINT,
    traffic_left      BIGINT,
    stat_dt           TIMESTAMPTZ,
    expire_dt         TIMESTAMPTZ,
    status            VARCHAR(20)   DEFAULT 'normal',
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX        IF NOT EXISTS idx_ac_iot_card_deleted_at ON ac_iot_card (deleted_at);
CREATE INDEX        IF NOT EXISTS idx_ac_iot_card_region_id  ON ac_iot_card (region_id);
CREATE INDEX        IF NOT EXISTS idx_ac_iot_card_device_id  ON ac_iot_card (device_id);
CREATE INDEX        IF NOT EXISTS idx_ac_iot_card_dept_id    ON ac_iot_card (dept_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_iot_card_no         ON ac_iot_card (card_no);

COMMENT ON TABLE  ac_iot_card              IS '物联网卡';
COMMENT ON COLUMN ac_iot_card.id           IS '主键';
COMMENT ON COLUMN ac_iot_card.deleted_at   IS '软删除时间';
COMMENT ON COLUMN ac_iot_card.created_by   IS '创建人';
COMMENT ON COLUMN ac_iot_card.created_dt   IS '创建时间';
COMMENT ON COLUMN ac_iot_card.updated_by   IS '更新人';
COMMENT ON COLUMN ac_iot_card.updated_dt   IS '更新时间';
COMMENT ON COLUMN ac_iot_card.active       IS '是否启用';
COMMENT ON COLUMN ac_iot_card.name         IS '物联网卡名称';
COMMENT ON COLUMN ac_iot_card.card_no      IS '卡号';
COMMENT ON COLUMN ac_iot_card.region_id    IS '所属区域';
COMMENT ON COLUMN ac_iot_card.device_id    IS '所属设备';
COMMENT ON COLUMN ac_iot_card.traffic_total IS '流量总额(MB)';
COMMENT ON COLUMN ac_iot_card.traffic_used IS '已使用流量(MB)';
COMMENT ON COLUMN ac_iot_card.traffic_left IS '剩余流量(MB)';
COMMENT ON COLUMN ac_iot_card.stat_dt      IS '统计时间';
COMMENT ON COLUMN ac_iot_card.expire_dt    IS '到期时间';
COMMENT ON COLUMN ac_iot_card.status       IS 'normal/arrears/expired/disabled';
COMMENT ON COLUMN ac_iot_card.remark       IS '备注';
COMMENT ON COLUMN ac_iot_card.dept_id      IS '所属部门(数据权限)';

-- ----------------------------------------------------------------
-- ac_device AI 摄像头设备
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_device (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    name              VARCHAR(255)  NOT NULL,
    device_code       VARCHAR(100)  NOT NULL,
    region_id         BIGINT,
    location          VARCHAR(255),
    latitude          VARCHAR(32),
    longitude         VARCHAR(32),
    firmware_id       BIGINT,
    firmware_ver      VARCHAR(50),
    model_id          BIGINT,
    model_ver         VARCHAR(50),
    system_ver        VARCHAR(50),
    work_mode         VARCHAR(50),
    device_mode       VARCHAR(50),
    info_interval     BIGINT        DEFAULT 60,
    iot_card_id       BIGINT,
    status            VARCHAR(20)   DEFAULT 'offline',
    battery           BIGINT,
    voltage           VARCHAR(32),
    storage_total     BIGINT,
    storage_free      BIGINT,
    online_dt         TIMESTAMPTZ,
    offline_dt        TIMESTAMPTZ,
    last_beat_dt      TIMESTAMPTZ,
    monitor_enabled   BOOLEAN       DEFAULT FALSE,
    stream_url        VARCHAR(500),
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX        IF NOT EXISTS idx_ac_device_deleted_at ON ac_device (deleted_at);
CREATE INDEX        IF NOT EXISTS idx_ac_device_region_id  ON ac_device (region_id);
CREATE INDEX        IF NOT EXISTS idx_ac_device_dept_id    ON ac_device (dept_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_device_code       ON ac_device (device_code);

COMMENT ON TABLE  ac_device                IS 'AI 摄像头设备';
COMMENT ON COLUMN ac_device.id             IS '主键';
COMMENT ON COLUMN ac_device.deleted_at     IS '软删除时间';
COMMENT ON COLUMN ac_device.created_by     IS '创建人';
COMMENT ON COLUMN ac_device.created_dt     IS '创建时间';
COMMENT ON COLUMN ac_device.updated_by     IS '更新人';
COMMENT ON COLUMN ac_device.updated_dt     IS '更新时间';
COMMENT ON COLUMN ac_device.active         IS '是否启用';
COMMENT ON COLUMN ac_device.name           IS '设备名称';
COMMENT ON COLUMN ac_device.device_code    IS '设备编号';
COMMENT ON COLUMN ac_device.region_id      IS '所属区域';
COMMENT ON COLUMN ac_device.location       IS '设备位置';
COMMENT ON COLUMN ac_device.latitude       IS '纬度(GIS)';
COMMENT ON COLUMN ac_device.longitude      IS '经度(GIS)';
COMMENT ON COLUMN ac_device.firmware_id    IS '当前固件ID';
COMMENT ON COLUMN ac_device.firmware_ver   IS '当前固件版本(冗余便于列表展示)';
COMMENT ON COLUMN ac_device.model_id       IS '当前AI模型ID';
COMMENT ON COLUMN ac_device.model_ver      IS '当前模型版本';
COMMENT ON COLUMN ac_device.system_ver     IS '系统版本';
COMMENT ON COLUMN ac_device.work_mode      IS '设备工作模式';
COMMENT ON COLUMN ac_device.device_mode    IS '设备模式(充电/全开/省电)';
COMMENT ON COLUMN ac_device.info_interval  IS '信息上传间隔(秒)';
COMMENT ON COLUMN ac_device.iot_card_id    IS '物联网卡ID';
COMMENT ON COLUMN ac_device.status         IS '运行状态 online/offline';
COMMENT ON COLUMN ac_device.battery        IS '电量百分比';
COMMENT ON COLUMN ac_device.voltage        IS '电压';
COMMENT ON COLUMN ac_device.storage_total  IS '存储总容量(MB)';
COMMENT ON COLUMN ac_device.storage_free   IS '剩余存储容量(MB)';
COMMENT ON COLUMN ac_device.online_dt      IS '最近上线时间';
COMMENT ON COLUMN ac_device.offline_dt     IS '最近离线时间';
COMMENT ON COLUMN ac_device.last_beat_dt   IS '最近心跳时间';
COMMENT ON COLUMN ac_device.monitor_enabled IS '是否加入实时监控';
COMMENT ON COLUMN ac_device.stream_url     IS '实时视频流地址';
COMMENT ON COLUMN ac_device.remark         IS '备注';
COMMENT ON COLUMN ac_device.dept_id        IS '所属部门(数据权限)';

-- ----------------------------------------------------------------
-- ac_iot_card_replace_log 物联网卡更换记录（追加写入）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_iot_card_replace_log (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    device_id         BIGINT        NOT NULL,
    device_code       VARCHAR(100),
    region_id         BIGINT,
    location          VARCHAR(255),
    old_iot_card_id   BIGINT,
    new_iot_card_id   BIGINT,
    old_card_no       VARCHAR(100),
    new_card_no       VARCHAR(100),
    replace_dt        TIMESTAMPTZ,
    operator          VARCHAR(50),
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_iot_card_replace_log_deleted_at ON ac_iot_card_replace_log (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_iot_card_replace_log_device_id  ON ac_iot_card_replace_log (device_id);
CREATE INDEX IF NOT EXISTS idx_ac_iot_card_replace_log_dept_id    ON ac_iot_card_replace_log (dept_id);

COMMENT ON TABLE  ac_iot_card_replace_log                  IS '物联网卡更换记录';
COMMENT ON COLUMN ac_iot_card_replace_log.id               IS '主键';
COMMENT ON COLUMN ac_iot_card_replace_log.deleted_at       IS '软删除时间';
COMMENT ON COLUMN ac_iot_card_replace_log.created_by       IS '创建人';
COMMENT ON COLUMN ac_iot_card_replace_log.created_dt       IS '创建时间';
COMMENT ON COLUMN ac_iot_card_replace_log.updated_by       IS '更新人';
COMMENT ON COLUMN ac_iot_card_replace_log.updated_dt       IS '更新时间';
COMMENT ON COLUMN ac_iot_card_replace_log.device_id        IS '设备ID';
COMMENT ON COLUMN ac_iot_card_replace_log.device_code      IS '设备编号(冗余)';
COMMENT ON COLUMN ac_iot_card_replace_log.region_id        IS '所属区域';
COMMENT ON COLUMN ac_iot_card_replace_log.location         IS '设备位置(冗余)';
COMMENT ON COLUMN ac_iot_card_replace_log.old_iot_card_id  IS '原物联网卡ID';
COMMENT ON COLUMN ac_iot_card_replace_log.new_iot_card_id  IS '新物联网卡ID';
COMMENT ON COLUMN ac_iot_card_replace_log.old_card_no      IS '原卡号';
COMMENT ON COLUMN ac_iot_card_replace_log.new_card_no      IS '新卡号';
COMMENT ON COLUMN ac_iot_card_replace_log.replace_dt       IS '更换时间';
COMMENT ON COLUMN ac_iot_card_replace_log.operator         IS '操作人';
COMMENT ON COLUMN ac_iot_card_replace_log.remark           IS '备注';
COMMENT ON COLUMN ac_iot_card_replace_log.dept_id          IS '所属部门(数据权限)';

-- ----------------------------------------------------------------
-- ac_voice_config 语音配置（为风险类型配置播报音频）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_voice_config (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    name              VARCHAR(255)  NOT NULL,
    voice_type        VARCHAR(50),
    risk_type_id      BIGINT,
    region_id         BIGINT,
    device_id         BIGINT,
    audio_url         VARCHAR(500),
    audio_name        VARCHAR(255),
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_voice_config_deleted_at   ON ac_voice_config (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_voice_config_risk_type_id ON ac_voice_config (risk_type_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_config_region_id    ON ac_voice_config (region_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_config_device_id    ON ac_voice_config (device_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_config_dept_id      ON ac_voice_config (dept_id);

COMMENT ON TABLE  ac_voice_config               IS '语音配置';
COMMENT ON COLUMN ac_voice_config.id            IS '主键';
COMMENT ON COLUMN ac_voice_config.deleted_at    IS '软删除时间';
COMMENT ON COLUMN ac_voice_config.created_by    IS '创建人';
COMMENT ON COLUMN ac_voice_config.created_dt    IS '创建时间';
COMMENT ON COLUMN ac_voice_config.updated_by    IS '更新人';
COMMENT ON COLUMN ac_voice_config.updated_dt    IS '更新时间';
COMMENT ON COLUMN ac_voice_config.active        IS '是否启用';
COMMENT ON COLUMN ac_voice_config.name          IS '语音名称';
COMMENT ON COLUMN ac_voice_config.voice_type    IS '语音类型(关联风险类型编码)';
COMMENT ON COLUMN ac_voice_config.risk_type_id  IS '风险类型ID';
COMMENT ON COLUMN ac_voice_config.region_id     IS '所属区域';
COMMENT ON COLUMN ac_voice_config.device_id     IS '应用设备';
COMMENT ON COLUMN ac_voice_config.audio_url     IS '播报音频地址';
COMMENT ON COLUMN ac_voice_config.audio_name    IS '音频文件名';
COMMENT ON COLUMN ac_voice_config.remark        IS '备注';
COMMENT ON COLUMN ac_voice_config.dept_id       IS '所属部门(数据权限)';

-- ----------------------------------------------------------------
-- ac_device_history 设备运行历史（时序采样）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_device_history (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    device_id         BIGINT        NOT NULL,
    device_code       VARCHAR(100),
    iot_card_no       VARCHAR(100),
    region_id         BIGINT,
    location          VARCHAR(255),
    status            VARCHAR(20),
    battery           BIGINT,
    voltage           VARCHAR(32),
    storage_free      BIGINT,
    sampled_dt        TIMESTAMPTZ,
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_device_history_deleted_at ON ac_device_history (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_device_history_dept_id    ON ac_device_history (dept_id);
-- 曲线查询与一周回放均按「设备 + 采样时间」范围扫描，故建复合索引
CREATE INDEX IF NOT EXISTS idx_ac_device_history_device_dt
    ON ac_device_history (device_id, sampled_dt);

COMMENT ON TABLE  ac_device_history              IS '设备运行历史(时序采样)';
COMMENT ON COLUMN ac_device_history.id           IS '主键';
COMMENT ON COLUMN ac_device_history.deleted_at   IS '软删除时间';
COMMENT ON COLUMN ac_device_history.created_by   IS '创建人';
COMMENT ON COLUMN ac_device_history.created_dt   IS '创建时间';
COMMENT ON COLUMN ac_device_history.updated_by   IS '更新人';
COMMENT ON COLUMN ac_device_history.updated_dt   IS '更新时间';
COMMENT ON COLUMN ac_device_history.device_id    IS '设备ID';
COMMENT ON COLUMN ac_device_history.device_code  IS '设备编号(冗余)';
COMMENT ON COLUMN ac_device_history.iot_card_no  IS '流量卡号(冗余)';
COMMENT ON COLUMN ac_device_history.region_id    IS '所属区域';
COMMENT ON COLUMN ac_device_history.location     IS '设备位置(冗余)';
COMMENT ON COLUMN ac_device_history.status       IS '运行状态 online/offline';
COMMENT ON COLUMN ac_device_history.battery      IS '电量百分比';
COMMENT ON COLUMN ac_device_history.voltage      IS '电压';
COMMENT ON COLUMN ac_device_history.storage_free IS '剩余存储容量(MB)';
COMMENT ON COLUMN ac_device_history.sampled_dt   IS '采样时间';
COMMENT ON COLUMN ac_device_history.dept_id      IS '所属部门(数据权限)';
