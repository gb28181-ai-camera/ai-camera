-- ================================================================
-- 04-风险与告警 - DDL（2 张表）
-- AI Camera 智能监控平台 · v0.1.0
-- 覆盖：AI 风险记录 / 设备告警记录
--
-- 说明：
--   1. 两张表结构平行：告警来自设备自身状态（离线/电量/流量），
--      风险来自 AI 识别；两者在「系统通知」(ac_notification) 中统一汇聚。
--   2. ac_risk_record.detect_accurate 为人工确认结果：
--      选「否」即误报 → status 直接置 handled；选「是」→ 仍需处理。
--   3. 冗余 device_code / location / risk_level / alarm_level 用于列表展示，
--      避免每次查询都回表关联 ac_device / ac_risk_type / ac_alarm_type。
-- ================================================================

-- ----------------------------------------------------------------
-- ac_risk_record AI 风险记录
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_risk_record (
    id                BIGSERIAL         PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ       DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ       DEFAULT CURRENT_TIMESTAMP,
    risk_type_id      BIGINT            NOT NULL,
    device_id         BIGINT            NOT NULL,
    device_code       VARCHAR(100),
    region_id         BIGINT,
    location          VARCHAR(255),
    image_url         VARCHAR(500),
    video_url         VARCHAR(500),
    risk_level        VARCHAR(20),
    detect_score      DECIMAL,
    occurred_dt       TIMESTAMPTZ,
    detect_accurate   VARCHAR(10),
    accurate_by       VARCHAR(50),
    accurate_dt       TIMESTAMPTZ,
    "read"            BOOLEAN           DEFAULT FALSE,
    read_dt           TIMESTAMPTZ,
    status            VARCHAR(20)       DEFAULT 'pending',
    handle_result     TEXT,
    handle_image      VARCHAR(500),
    handler           VARCHAR(50),
    handled_dt        TIMESTAMPTZ,
    voice_played      BOOLEAN           DEFAULT FALSE,
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_risk_record_deleted_at   ON ac_risk_record (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_risk_record_risk_type_id ON ac_risk_record (risk_type_id);
CREATE INDEX IF NOT EXISTS idx_ac_risk_record_device_id    ON ac_risk_record (device_id);
CREATE INDEX IF NOT EXISTS idx_ac_risk_record_region_id    ON ac_risk_record (region_id);
CREATE INDEX IF NOT EXISTS idx_ac_risk_record_occurred_dt  ON ac_risk_record (occurred_dt);
CREATE INDEX IF NOT EXISTS idx_ac_risk_record_status       ON ac_risk_record (status);
CREATE INDEX IF NOT EXISTS idx_ac_risk_record_dept_id      ON ac_risk_record (dept_id);

COMMENT ON TABLE  ac_risk_record                 IS 'AI 风险记录';
COMMENT ON COLUMN ac_risk_record.id              IS '主键';
COMMENT ON COLUMN ac_risk_record.deleted_at      IS '软删除时间';
COMMENT ON COLUMN ac_risk_record.created_by      IS '创建人';
COMMENT ON COLUMN ac_risk_record.created_dt      IS '创建时间';
COMMENT ON COLUMN ac_risk_record.updated_by      IS '更新人';
COMMENT ON COLUMN ac_risk_record.updated_dt      IS '更新时间';
COMMENT ON COLUMN ac_risk_record.risk_type_id    IS '风险类型ID';
COMMENT ON COLUMN ac_risk_record.device_id       IS '设备ID';
COMMENT ON COLUMN ac_risk_record.device_code     IS '设备编号(冗余)';
COMMENT ON COLUMN ac_risk_record.region_id       IS '所属区域';
COMMENT ON COLUMN ac_risk_record.location        IS '设备位置(冗余)';
COMMENT ON COLUMN ac_risk_record.image_url       IS '风险图片';
COMMENT ON COLUMN ac_risk_record.video_url       IS '风险录像';
COMMENT ON COLUMN ac_risk_record.risk_level      IS '风险等级(冗余)';
COMMENT ON COLUMN ac_risk_record.detect_score    IS 'AI 置信度';
COMMENT ON COLUMN ac_risk_record.occurred_dt     IS '发生时间';
COMMENT ON COLUMN ac_risk_record.detect_accurate IS '检测准确 yes/no';
COMMENT ON COLUMN ac_risk_record.accurate_by     IS '确认人';
COMMENT ON COLUMN ac_risk_record.accurate_dt     IS '确认时间';
COMMENT ON COLUMN ac_risk_record."read"          IS '消息通知是否已读';
COMMENT ON COLUMN ac_risk_record.read_dt         IS '已读时间';
COMMENT ON COLUMN ac_risk_record.status          IS '处理状态 pending/handled';
COMMENT ON COLUMN ac_risk_record.handle_result   IS '处理结果';
COMMENT ON COLUMN ac_risk_record.handle_image    IS '处理图片';
COMMENT ON COLUMN ac_risk_record.handler         IS '处理人';
COMMENT ON COLUMN ac_risk_record.handled_dt      IS '处理时间';
COMMENT ON COLUMN ac_risk_record.voice_played    IS '是否已语音播报';
COMMENT ON COLUMN ac_risk_record.dept_id         IS '所属部门(数据权限)';

-- ----------------------------------------------------------------
-- ac_alarm_record 设备告警记录
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_alarm_record (
    id                BIGSERIAL         PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ       DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ       DEFAULT CURRENT_TIMESTAMP,
    alarm_type_id     BIGINT            NOT NULL,
    device_id         BIGINT            NOT NULL,
    device_code       VARCHAR(100),
    region_id         BIGINT,
    location          VARCHAR(255),
    alarm_level       VARCHAR(20),
    content           TEXT,
    occurred_dt       TIMESTAMPTZ,
    "read"            BOOLEAN           DEFAULT FALSE,
    read_dt           TIMESTAMPTZ,
    status            VARCHAR(20)       DEFAULT 'pending',
    handle_result     TEXT,
    handle_image      VARCHAR(500),
    handler           VARCHAR(50),
    handled_dt        TIMESTAMPTZ,
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_alarm_record_deleted_at    ON ac_alarm_record (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_alarm_record_alarm_type_id ON ac_alarm_record (alarm_type_id);
CREATE INDEX IF NOT EXISTS idx_ac_alarm_record_device_id     ON ac_alarm_record (device_id);
CREATE INDEX IF NOT EXISTS idx_ac_alarm_record_region_id     ON ac_alarm_record (region_id);
CREATE INDEX IF NOT EXISTS idx_ac_alarm_record_occurred_dt   ON ac_alarm_record (occurred_dt);
CREATE INDEX IF NOT EXISTS idx_ac_alarm_record_status        ON ac_alarm_record (status);
CREATE INDEX IF NOT EXISTS idx_ac_alarm_record_dept_id       ON ac_alarm_record (dept_id);

COMMENT ON TABLE  ac_alarm_record               IS '设备告警记录';
COMMENT ON COLUMN ac_alarm_record.id            IS '主键';
COMMENT ON COLUMN ac_alarm_record.deleted_at    IS '软删除时间';
COMMENT ON COLUMN ac_alarm_record.created_by    IS '创建人';
COMMENT ON COLUMN ac_alarm_record.created_dt    IS '创建时间';
COMMENT ON COLUMN ac_alarm_record.updated_by    IS '更新人';
COMMENT ON COLUMN ac_alarm_record.updated_dt    IS '更新时间';
COMMENT ON COLUMN ac_alarm_record.alarm_type_id IS '告警类型ID';
COMMENT ON COLUMN ac_alarm_record.device_id     IS '设备ID';
COMMENT ON COLUMN ac_alarm_record.device_code   IS '设备编号(冗余)';
COMMENT ON COLUMN ac_alarm_record.region_id     IS '所属区域';
COMMENT ON COLUMN ac_alarm_record.location      IS '设备位置(冗余)';
COMMENT ON COLUMN ac_alarm_record.alarm_level   IS '告警等级(冗余)';
COMMENT ON COLUMN ac_alarm_record.content       IS '告警内容';
COMMENT ON COLUMN ac_alarm_record.occurred_dt   IS '告警时间';
COMMENT ON COLUMN ac_alarm_record."read"        IS '消息通知是否已读';
COMMENT ON COLUMN ac_alarm_record.read_dt       IS '已读时间';
COMMENT ON COLUMN ac_alarm_record.status        IS '处理状态 pending/handled';
COMMENT ON COLUMN ac_alarm_record.handle_result IS '处理结果';
COMMENT ON COLUMN ac_alarm_record.handle_image  IS '处理图片';
COMMENT ON COLUMN ac_alarm_record.handler       IS '处理人';
COMMENT ON COLUMN ac_alarm_record.handled_dt    IS '处理时间';
COMMENT ON COLUMN ac_alarm_record.dept_id       IS '所属部门(数据权限)';
