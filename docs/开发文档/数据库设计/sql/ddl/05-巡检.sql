-- ================================================================
-- 05-巡检 - DDL（2 张表）
-- AI Camera 智能监控平台 · v0.1.0
-- 覆盖：巡检任务 / 巡检记录
--
-- 说明：
--   1. 任务发布后 status=published，被指派的巡检人在移动端处理后
--      任务移除（status=finished）并生成一条 ac_inspect_record。
--   2. ac_inspect_record 保留任务快照字段（task_name / region_id /
--      location / inspector / content），使任务被改动或删除后记录仍可独立查看。
-- ================================================================

-- ----------------------------------------------------------------
-- ac_inspect_task 巡检任务
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_inspect_task (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    name              VARCHAR(255)  NOT NULL,
    region_id         BIGINT,
    location          VARCHAR(255),
    inspector_id      BIGINT        NOT NULL,
    content           TEXT,
    plan_dt           TIMESTAMPTZ,
    status            VARCHAR(20)   DEFAULT 'published',
    "read"            BOOLEAN       DEFAULT FALSE,
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_inspect_task_deleted_at   ON ac_inspect_task (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_inspect_task_region_id    ON ac_inspect_task (region_id);
CREATE INDEX IF NOT EXISTS idx_ac_inspect_task_inspector_id ON ac_inspect_task (inspector_id);
CREATE INDEX IF NOT EXISTS idx_ac_inspect_task_plan_dt      ON ac_inspect_task (plan_dt);
CREATE INDEX IF NOT EXISTS idx_ac_inspect_task_status       ON ac_inspect_task (status);
CREATE INDEX IF NOT EXISTS idx_ac_inspect_task_dept_id      ON ac_inspect_task (dept_id);

COMMENT ON TABLE  ac_inspect_task               IS '巡检任务';
COMMENT ON COLUMN ac_inspect_task.id            IS '主键';
COMMENT ON COLUMN ac_inspect_task.deleted_at    IS '软删除时间';
COMMENT ON COLUMN ac_inspect_task.created_by    IS '创建人';
COMMENT ON COLUMN ac_inspect_task.created_dt    IS '创建时间';
COMMENT ON COLUMN ac_inspect_task.updated_by    IS '更新人';
COMMENT ON COLUMN ac_inspect_task.updated_dt    IS '更新时间';
COMMENT ON COLUMN ac_inspect_task.active        IS '是否启用';
COMMENT ON COLUMN ac_inspect_task.name          IS '任务名称';
COMMENT ON COLUMN ac_inspect_task.region_id     IS '所属区域';
COMMENT ON COLUMN ac_inspect_task.location      IS '巡检位置';
COMMENT ON COLUMN ac_inspect_task.inspector_id  IS '巡检人用户ID';
COMMENT ON COLUMN ac_inspect_task.content       IS '巡检内容';
COMMENT ON COLUMN ac_inspect_task.plan_dt       IS '巡检时间';
COMMENT ON COLUMN ac_inspect_task.status        IS 'pending/published/finished/canceled';
COMMENT ON COLUMN ac_inspect_task."read"        IS '消息通知是否已读';
COMMENT ON COLUMN ac_inspect_task.remark        IS '备注';
COMMENT ON COLUMN ac_inspect_task.dept_id       IS '所属部门(数据权限)';

-- ----------------------------------------------------------------
-- ac_inspect_record 巡检记录
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_inspect_record (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    task_id           BIGINT,
    task_name         VARCHAR(255),
    region_id         BIGINT,
    location          VARCHAR(255),
    inspector_id      BIGINT,
    inspector         VARCHAR(50),
    content           TEXT,
    plan_dt           TIMESTAMPTZ,
    result            TEXT,
    result_image      VARCHAR(500),
    handled_dt        TIMESTAMPTZ,
    status            VARCHAR(20)   DEFAULT 'finished',
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_inspect_record_deleted_at   ON ac_inspect_record (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_inspect_record_task_id      ON ac_inspect_record (task_id);
CREATE INDEX IF NOT EXISTS idx_ac_inspect_record_region_id    ON ac_inspect_record (region_id);
CREATE INDEX IF NOT EXISTS idx_ac_inspect_record_inspector_id ON ac_inspect_record (inspector_id);
CREATE INDEX IF NOT EXISTS idx_ac_inspect_record_dept_id      ON ac_inspect_record (dept_id);

COMMENT ON TABLE  ac_inspect_record              IS '巡检记录';
COMMENT ON COLUMN ac_inspect_record.id           IS '主键';
COMMENT ON COLUMN ac_inspect_record.deleted_at   IS '软删除时间';
COMMENT ON COLUMN ac_inspect_record.created_by   IS '创建人';
COMMENT ON COLUMN ac_inspect_record.created_dt   IS '创建时间';
COMMENT ON COLUMN ac_inspect_record.updated_by   IS '更新人';
COMMENT ON COLUMN ac_inspect_record.updated_dt   IS '更新时间';
COMMENT ON COLUMN ac_inspect_record.task_id      IS '来源巡检任务ID';
COMMENT ON COLUMN ac_inspect_record.task_name    IS '任务名称(冗余)';
COMMENT ON COLUMN ac_inspect_record.region_id    IS '所属区域';
COMMENT ON COLUMN ac_inspect_record.location     IS '巡检位置';
COMMENT ON COLUMN ac_inspect_record.inspector_id IS '巡检人用户ID';
COMMENT ON COLUMN ac_inspect_record.inspector    IS '巡检人(冗余)';
COMMENT ON COLUMN ac_inspect_record.content      IS '巡检内容';
COMMENT ON COLUMN ac_inspect_record.plan_dt      IS '巡检时间';
COMMENT ON COLUMN ac_inspect_record.result       IS '处理结果';
COMMENT ON COLUMN ac_inspect_record.result_image IS '处理结果图';
COMMENT ON COLUMN ac_inspect_record.handled_dt   IS '处理时间';
COMMENT ON COLUMN ac_inspect_record.status       IS '状态';
COMMENT ON COLUMN ac_inspect_record.dept_id      IS '所属部门(数据权限)';
