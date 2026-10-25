-- ============================================================================
-- 16. 设备 MQTT 上报（v0.6.0）
--
-- 适用设备：适老化智能相机（WebRTC 服务组）通过 MQTT 上报
--   告警 / 电量 / 位置 / 心跳。
--
-- 为什么单独建位置表：
--   ac_gb_mobile_position 以 gb_device_id 为键，属于 GB28181 协议域；
--   MQTT 设备没有国标编码，复用会污染国标查询与数据权限。
-- ============================================================================

CREATE TABLE IF NOT EXISTS ac_iot_device_position (
    id           BIGSERIAL   PRIMARY KEY,
    deleted_at   TIMESTAMPTZ,
    created_by   VARCHAR(50),
    created_dt   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_by   VARCHAR(50),
    updated_dt   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    device_id    BIGINT      NOT NULL,
    device_code  VARCHAR(100),
    longitude    DOUBLE PRECISION NOT NULL,
    latitude     DOUBLE PRECISION NOT NULL,
    speed        DOUBLE PRECISION DEFAULT 0,
    direction    DOUBLE PRECISION DEFAULT 0,
    altitude     DOUBLE PRECISION DEFAULT 0,
    reported_dt  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    dept_id      BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_iot_device_position_deleted_at
    ON ac_iot_device_position (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_iot_device_position_device_dt
    ON ac_iot_device_position (device_id, reported_dt DESC);
CREATE INDEX IF NOT EXISTS idx_ac_iot_device_position_dept_id
    ON ac_iot_device_position (dept_id);

COMMENT ON TABLE  ac_iot_device_position IS 'MQTT 设备位置上报历史';
COMMENT ON COLUMN ac_iot_device_position.device_id   IS '设备ID';
COMMENT ON COLUMN ac_iot_device_position.device_code IS '设备编号(冗余)';
COMMENT ON COLUMN ac_iot_device_position.longitude   IS '经度';
COMMENT ON COLUMN ac_iot_device_position.latitude    IS '纬度';
COMMENT ON COLUMN ac_iot_device_position.speed       IS '速度(km/h)';
COMMENT ON COLUMN ac_iot_device_position.direction   IS '方向(0-360)';
COMMENT ON COLUMN ac_iot_device_position.altitude    IS '海拔(m)';
COMMENT ON COLUMN ac_iot_device_position.reported_dt IS '上报时间';

-- ── App 离线推送令牌（个推 CID）────────────────────────────────
CREATE TABLE IF NOT EXISTS ac_user_push_token (
    id           BIGSERIAL   PRIMARY KEY,
    deleted_at   TIMESTAMPTZ,
    created_by   VARCHAR(50),
    created_dt   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_by   VARCHAR(50),
    updated_dt   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    user_id      BIGINT      NOT NULL,
    cid          VARCHAR(128) NOT NULL,
    platform     VARCHAR(20),
    app_version  VARCHAR(50),
    last_seen_dt TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_user_push_token_cid
    ON ac_user_push_token (cid);
CREATE INDEX IF NOT EXISTS idx_ac_user_push_token_user
    ON ac_user_push_token (user_id);
CREATE INDEX IF NOT EXISTS idx_ac_user_push_token_deleted_at
    ON ac_user_push_token (deleted_at);

COMMENT ON TABLE  ac_user_push_token IS 'App 推送令牌(个推 CID)';
COMMENT ON COLUMN ac_user_push_token.user_id IS '平台用户ID';
COMMENT ON COLUMN ac_user_push_token.cid     IS '个推客户端ID';
COMMENT ON COLUMN ac_user_push_token.platform IS 'ios/android';
COMMENT ON COLUMN ac_user_push_token.app_version IS 'App版本';
COMMENT ON COLUMN ac_user_push_token.last_seen_dt IS '最近上报时间';
