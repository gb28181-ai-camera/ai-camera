-- ================================================================
-- 02-基础信息 - DDL（3 张表）
-- AI Camera 智能监控平台 · v0.1.0
-- 覆盖：区域 / 风险类型 / 告警类型
--
-- 说明：三张表结构同构（name + code + level/remark + dept_id），
--       分别作为设备归属维度、AI 风险分类、设备告警分类的字典表。
--       区域是设备/风险/告警/巡检的核心归属维度，几乎所有业务列表
--       都支持按 ac_region 筛选。
-- ================================================================

-- ----------------------------------------------------------------
-- ac_region 区域
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_region (
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

CREATE INDEX IF NOT EXISTS idx_ac_region_deleted_at ON ac_region (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_region_dept_id    ON ac_region (dept_id);

COMMENT ON TABLE  ac_region             IS '区域';
COMMENT ON COLUMN ac_region.id          IS '主键';
COMMENT ON COLUMN ac_region.deleted_at  IS '软删除时间';
COMMENT ON COLUMN ac_region.created_by  IS '创建人';
COMMENT ON COLUMN ac_region.created_dt  IS '创建时间';
COMMENT ON COLUMN ac_region.updated_by  IS '更新人';
COMMENT ON COLUMN ac_region.updated_dt  IS '更新时间';
COMMENT ON COLUMN ac_region.active      IS '是否启用';
COMMENT ON COLUMN ac_region.sequence    IS '排序';
COMMENT ON COLUMN ac_region.name        IS '区域名称';
COMMENT ON COLUMN ac_region.code        IS '区域编码';
COMMENT ON COLUMN ac_region.remark      IS '备注';
COMMENT ON COLUMN ac_region.dept_id     IS '所属部门(数据权限)';

-- ----------------------------------------------------------------
-- ac_risk_type 风险类型
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_risk_type (
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
    level             VARCHAR(20)   DEFAULT 'medium',
    description       TEXT,
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_risk_type_deleted_at ON ac_risk_type (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_risk_type_dept_id    ON ac_risk_type (dept_id);

COMMENT ON TABLE  ac_risk_type              IS '风险类型';
COMMENT ON COLUMN ac_risk_type.id           IS '主键';
COMMENT ON COLUMN ac_risk_type.deleted_at   IS '软删除时间';
COMMENT ON COLUMN ac_risk_type.created_by   IS '创建人';
COMMENT ON COLUMN ac_risk_type.created_dt   IS '创建时间';
COMMENT ON COLUMN ac_risk_type.updated_by   IS '更新人';
COMMENT ON COLUMN ac_risk_type.updated_dt   IS '更新时间';
COMMENT ON COLUMN ac_risk_type.active       IS '是否启用';
COMMENT ON COLUMN ac_risk_type.sequence     IS '排序';
COMMENT ON COLUMN ac_risk_type.name         IS '风险类型名称';
COMMENT ON COLUMN ac_risk_type.code         IS '风险编码';
COMMENT ON COLUMN ac_risk_type.level        IS '风险等级 high/medium/low';
COMMENT ON COLUMN ac_risk_type.description  IS '描述';
COMMENT ON COLUMN ac_risk_type.dept_id      IS '所属部门(数据权限)';

-- ----------------------------------------------------------------
-- ac_alarm_type 告警类型
-- 平台内置离线(offline)/电量(battery)/流量(traffic)三条，可扩展
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_alarm_type (
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
    level             VARCHAR(20)   DEFAULT 'medium',
    description       TEXT,
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_alarm_type_deleted_at ON ac_alarm_type (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_alarm_type_dept_id    ON ac_alarm_type (dept_id);

COMMENT ON TABLE  ac_alarm_type              IS '告警类型';
COMMENT ON COLUMN ac_alarm_type.id           IS '主键';
COMMENT ON COLUMN ac_alarm_type.deleted_at   IS '软删除时间';
COMMENT ON COLUMN ac_alarm_type.created_by   IS '创建人';
COMMENT ON COLUMN ac_alarm_type.created_dt   IS '创建时间';
COMMENT ON COLUMN ac_alarm_type.updated_by   IS '更新人';
COMMENT ON COLUMN ac_alarm_type.updated_dt   IS '更新时间';
COMMENT ON COLUMN ac_alarm_type.active       IS '是否启用';
COMMENT ON COLUMN ac_alarm_type.sequence     IS '排序';
COMMENT ON COLUMN ac_alarm_type.name         IS '告警名称';
COMMENT ON COLUMN ac_alarm_type.code         IS '告警编码';
COMMENT ON COLUMN ac_alarm_type.level        IS '告警等级 high/medium/low';
COMMENT ON COLUMN ac_alarm_type.description  IS '描述';
COMMENT ON COLUMN ac_alarm_type.dept_id      IS '所属部门(数据权限)';
