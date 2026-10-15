-- ================================================================
-- 07-告警设置 - DDL（3 张表）
-- AI Camera 智能监控平台 · v0.1.0
-- 覆盖：设备告警设置 / 风险通知设置 / 第三方平台配置
--
-- 说明：
--   1. ac_alarm_setting 与 ac_risk_notify_setting 结构平行，
--      唯一维度差异为「告警类型」与「风险类型」。
--   2. 两张设置表按「用户 + 类型」唯一确定一条策略，
--      业务上只允许维护当前登录用户自己的数据（user_id = ctx.user_id）。
--   3. ac_thirdparty_config 以键值形式存储个推 / 腾讯云短信两组配置，
--      避免为固定配置各建一张窄表；secret=true 的值接口返回时脱敏。
-- ================================================================

-- ----------------------------------------------------------------
-- ac_alarm_setting 设备告警设置
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_alarm_setting (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    user_id           BIGINT        NOT NULL,
    alarm_type_id     BIGINT        NOT NULL,
    active            BOOLEAN       DEFAULT TRUE,
    notify_type       VARCHAR(20)   DEFAULT 'app',
    notify_freq       BIGINT        DEFAULT 0,
    notify_user_ids   TEXT,
    threshold         VARCHAR(50),
    remark            VARCHAR(500)
);

CREATE INDEX IF NOT EXISTS idx_ac_alarm_setting_deleted_at ON ac_alarm_setting (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_alarm_setting_user_type
    ON ac_alarm_setting (user_id, alarm_type_id);

COMMENT ON TABLE  ac_alarm_setting                 IS '设备告警设置';
COMMENT ON COLUMN ac_alarm_setting.id              IS '主键';
COMMENT ON COLUMN ac_alarm_setting.deleted_at      IS '软删除时间';
COMMENT ON COLUMN ac_alarm_setting.created_by      IS '创建人';
COMMENT ON COLUMN ac_alarm_setting.created_dt      IS '创建时间';
COMMENT ON COLUMN ac_alarm_setting.updated_by      IS '更新人';
COMMENT ON COLUMN ac_alarm_setting.updated_dt      IS '更新时间';
COMMENT ON COLUMN ac_alarm_setting.user_id         IS '配置所属用户';
COMMENT ON COLUMN ac_alarm_setting.alarm_type_id   IS '告警类型ID';
COMMENT ON COLUMN ac_alarm_setting.active          IS '是否启用(关闭所有告警通知)';
COMMENT ON COLUMN ac_alarm_setting.notify_type     IS '通知类型 app/sms/inner';
COMMENT ON COLUMN ac_alarm_setting.notify_freq     IS '报警频率(分钟)';
COMMENT ON COLUMN ac_alarm_setting.notify_user_ids IS '通知人员用户ID列表';
COMMENT ON COLUMN ac_alarm_setting.threshold       IS '阈值(电量%/流量MB)';
COMMENT ON COLUMN ac_alarm_setting.remark          IS '备注';

-- ----------------------------------------------------------------
-- ac_risk_notify_setting 风险通知设置
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_risk_notify_setting (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    user_id           BIGINT        NOT NULL,
    risk_type_id      BIGINT        NOT NULL,
    active            BOOLEAN       DEFAULT TRUE,
    notify_type       VARCHAR(20)   DEFAULT 'app',
    notify_freq       BIGINT        DEFAULT 0,
    notify_user_ids   TEXT,
    remark            VARCHAR(500)
);

CREATE INDEX IF NOT EXISTS idx_ac_risk_notify_setting_deleted_at ON ac_risk_notify_setting (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_risk_notify_user_type
    ON ac_risk_notify_setting (user_id, risk_type_id);

COMMENT ON TABLE  ac_risk_notify_setting                 IS '风险通知设置';
COMMENT ON COLUMN ac_risk_notify_setting.id              IS '主键';
COMMENT ON COLUMN ac_risk_notify_setting.deleted_at      IS '软删除时间';
COMMENT ON COLUMN ac_risk_notify_setting.created_by      IS '创建人';
COMMENT ON COLUMN ac_risk_notify_setting.created_dt      IS '创建时间';
COMMENT ON COLUMN ac_risk_notify_setting.updated_by      IS '更新人';
COMMENT ON COLUMN ac_risk_notify_setting.updated_dt      IS '更新时间';
COMMENT ON COLUMN ac_risk_notify_setting.user_id         IS '配置所属用户';
COMMENT ON COLUMN ac_risk_notify_setting.risk_type_id    IS '风险类型ID';
COMMENT ON COLUMN ac_risk_notify_setting.active          IS '是否启用(关闭所有风险通知)';
COMMENT ON COLUMN ac_risk_notify_setting.notify_type     IS '通知类型 app/sms/inner';
COMMENT ON COLUMN ac_risk_notify_setting.notify_freq     IS '报警频率(分钟)';
COMMENT ON COLUMN ac_risk_notify_setting.notify_user_ids IS '通知人员用户ID列表';
COMMENT ON COLUMN ac_risk_notify_setting.remark          IS '备注';

-- ----------------------------------------------------------------
-- ac_thirdparty_config 第三方平台配置
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_thirdparty_config (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    category          VARCHAR(50)   NOT NULL,
    name              VARCHAR(255),
    code              VARCHAR(100)  NOT NULL,
    "value"           TEXT,
    secret            BOOLEAN       DEFAULT FALSE,
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_thirdparty_config_deleted_at ON ac_thirdparty_config (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_thirdparty_config_category   ON ac_thirdparty_config (category);
CREATE INDEX IF NOT EXISTS idx_ac_thirdparty_config_dept_id    ON ac_thirdparty_config (dept_id);

COMMENT ON TABLE  ac_thirdparty_config             IS '第三方平台配置(个推/腾讯云短信)';
COMMENT ON COLUMN ac_thirdparty_config.id          IS '主键';
COMMENT ON COLUMN ac_thirdparty_config.deleted_at  IS '软删除时间';
COMMENT ON COLUMN ac_thirdparty_config.created_by  IS '创建人';
COMMENT ON COLUMN ac_thirdparty_config.created_dt  IS '创建时间';
COMMENT ON COLUMN ac_thirdparty_config.updated_by  IS '更新人';
COMMENT ON COLUMN ac_thirdparty_config.updated_dt  IS '更新时间';
COMMENT ON COLUMN ac_thirdparty_config.category    IS 'getui/sms';
COMMENT ON COLUMN ac_thirdparty_config.name        IS '配置名称';
COMMENT ON COLUMN ac_thirdparty_config.code        IS '配置键';
COMMENT ON COLUMN ac_thirdparty_config."value"     IS '配置值';
COMMENT ON COLUMN ac_thirdparty_config.secret      IS '是否敏感值(接口返回时脱敏)';
COMMENT ON COLUMN ac_thirdparty_config.remark      IS '备注';
COMMENT ON COLUMN ac_thirdparty_config.dept_id     IS '所属部门(数据权限)';


-- ================================================================
-- ac_webhook_config - 消息推送 WebHook 配置
--   支持飞书 / 钉钉 / 企业微信 / 自定义 WebHook 四种平台。
--   URL 最长 1000 字符（钉钉加签后的地址会显著变长）。
-- ================================================================
CREATE TABLE IF NOT EXISTS ac_webhook_config (
    id                BIGSERIAL       PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ     DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ     DEFAULT CURRENT_TIMESTAMP,

    active            BOOLEAN         DEFAULT TRUE,
    sequence          BIGINT          DEFAULT 10,
    name              VARCHAR(255)    NOT NULL,
    platform          VARCHAR(20)     NOT NULL DEFAULT 'custom',
    url               VARCHAR(1000)   NOT NULL,
    secret            VARCHAR(255),
    msg_type          VARCHAR(20)     DEFAULT 'text',
    event_types       VARCHAR(255),
    last_send_dt      TIMESTAMPTZ,
    last_status       VARCHAR(20),
    last_error        TEXT,
    remark            VARCHAR(500)
);

COMMENT ON TABLE  ac_webhook_config               IS '消息推送WebHook配置';
COMMENT ON COLUMN ac_webhook_config.active        IS '是否启用';
COMMENT ON COLUMN ac_webhook_config.sequence      IS '排序';
COMMENT ON COLUMN ac_webhook_config.name          IS '配置名称';
COMMENT ON COLUMN ac_webhook_config.platform      IS '平台 feishu/dingtalk/wecom/custom';
COMMENT ON COLUMN ac_webhook_config.url           IS 'WebHook 地址';
COMMENT ON COLUMN ac_webhook_config.secret        IS '签名密钥(钉钉加签/自定义)';
COMMENT ON COLUMN ac_webhook_config.msg_type      IS '消息格式 text/markdown';
COMMENT ON COLUMN ac_webhook_config.event_types   IS '订阅事件(逗号分隔,空=全部) risk/alarm/inspect/device';
COMMENT ON COLUMN ac_webhook_config.last_send_dt  IS '最近发送时间';
COMMENT ON COLUMN ac_webhook_config.last_status   IS '最近发送结果 success/failed';
COMMENT ON COLUMN ac_webhook_config.last_error    IS '最近失败原因';
COMMENT ON COLUMN ac_webhook_config.remark        IS '备注';

CREATE INDEX IF NOT EXISTS idx_ac_webhook_config_platform ON ac_webhook_config (platform);
CREATE INDEX IF NOT EXISTS idx_ac_webhook_config_active   ON ac_webhook_config (active);
