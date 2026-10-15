-- ================================================================
-- 06-升级管理 - DDL（2 张表）
-- AI Camera 智能监控平台 · v0.1.0
-- 覆盖：设备定时升级计划 / 设备升级记录
--
-- 说明：
--   1. 本模块的另外 3 张字典表（ac_firmware_type / ac_firmware /
--      ac_ai_model）被 ac_device 引用，已归入 03-资源管理.sql，
--      在此不重复建表。
--   2. ac_device_upgrade_plan 仅在用户选择「定时升级」时产生；
--      选择「现在升级」直接下发并写 ac_device_upgrade_record。
--   3. ac_device_upgrade_record 保留升级前的固件/模型版本（prev_*），
--      使同一设备升级两次后可对比版本变化。
-- ================================================================

-- ----------------------------------------------------------------
-- ac_device_upgrade_plan 设备定时升级计划
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_device_upgrade_plan (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    device_id         BIGINT        NOT NULL,
    device_code       VARCHAR(100),
    device_name       VARCHAR(255),
    region_id         BIGINT,
    location          VARCHAR(255),
    firmware_id       BIGINT,
    ai_model_id       BIGINT,
    firmware_name     VARCHAR(255),
    ai_model_name     VARCHAR(255),
    plan_dt           TIMESTAMPTZ,
    status            VARCHAR(20)   DEFAULT 'pending',
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_device_upgrade_plan_deleted_at ON ac_device_upgrade_plan (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_device_upgrade_plan_device_id  ON ac_device_upgrade_plan (device_id);
CREATE INDEX IF NOT EXISTS idx_ac_device_upgrade_plan_plan_dt    ON ac_device_upgrade_plan (plan_dt);
CREATE INDEX IF NOT EXISTS idx_ac_device_upgrade_plan_status     ON ac_device_upgrade_plan (status);
CREATE INDEX IF NOT EXISTS idx_ac_device_upgrade_plan_dept_id    ON ac_device_upgrade_plan (dept_id);

COMMENT ON TABLE  ac_device_upgrade_plan               IS '设备定时升级计划';
COMMENT ON COLUMN ac_device_upgrade_plan.id            IS '主键';
COMMENT ON COLUMN ac_device_upgrade_plan.deleted_at    IS '软删除时间';
COMMENT ON COLUMN ac_device_upgrade_plan.created_by    IS '创建人';
COMMENT ON COLUMN ac_device_upgrade_plan.created_dt    IS '创建时间';
COMMENT ON COLUMN ac_device_upgrade_plan.updated_by    IS '更新人';
COMMENT ON COLUMN ac_device_upgrade_plan.updated_dt    IS '更新时间';
COMMENT ON COLUMN ac_device_upgrade_plan.device_id     IS '设备ID';
COMMENT ON COLUMN ac_device_upgrade_plan.device_code   IS '设备编号(冗余)';
COMMENT ON COLUMN ac_device_upgrade_plan.device_name   IS '设备名称(冗余)';
COMMENT ON COLUMN ac_device_upgrade_plan.region_id     IS '所属区域';
COMMENT ON COLUMN ac_device_upgrade_plan.location      IS '设备位置(冗余)';
COMMENT ON COLUMN ac_device_upgrade_plan.firmware_id   IS '升级固件ID';
COMMENT ON COLUMN ac_device_upgrade_plan.ai_model_id   IS '升级模型ID';
COMMENT ON COLUMN ac_device_upgrade_plan.firmware_name IS '升级固件名称(冗余)';
COMMENT ON COLUMN ac_device_upgrade_plan.ai_model_name IS '升级模型名称(冗余)';
COMMENT ON COLUMN ac_device_upgrade_plan.plan_dt       IS '升级时间';
COMMENT ON COLUMN ac_device_upgrade_plan.status        IS 'pending/running/done/failed/canceled';
COMMENT ON COLUMN ac_device_upgrade_plan.remark        IS '备注';
COMMENT ON COLUMN ac_device_upgrade_plan.dept_id       IS '所属部门(数据权限)';

-- ----------------------------------------------------------------
-- ac_device_upgrade_record 设备升级记录
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_device_upgrade_record (
    id                   BIGSERIAL     PRIMARY KEY,
    deleted_at           TIMESTAMPTZ,
    created_by           VARCHAR(50),
    created_dt           TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by           VARCHAR(50),
    updated_dt           TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    device_id            BIGINT        NOT NULL,
    device_code          VARCHAR(100),
    device_name          VARCHAR(255),
    region_id            BIGINT,
    location             VARCHAR(255),
    iot_card_no          VARCHAR(100),
    prev_firmware_id     BIGINT,
    prev_firmware_ver    VARCHAR(50),
    prev_ai_model_id     BIGINT,
    prev_ai_model_ver    VARCHAR(50),
    firmware_id          BIGINT,
    firmware_ver         VARCHAR(50),
    ai_model_id          BIGINT,
    ai_model_ver         VARCHAR(50),
    upgrade_type         VARCHAR(20),
    upgrade_dt           TIMESTAMPTZ,
    status               VARCHAR(20)   DEFAULT 'done',
    operator             VARCHAR(50),
    error_msg            TEXT,
    dept_id              BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_device_upgrade_record_deleted_at ON ac_device_upgrade_record (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_device_upgrade_record_device_id  ON ac_device_upgrade_record (device_id);
CREATE INDEX IF NOT EXISTS idx_ac_device_upgrade_record_upgrade_dt ON ac_device_upgrade_record (upgrade_dt);
CREATE INDEX IF NOT EXISTS idx_ac_device_upgrade_record_dept_id    ON ac_device_upgrade_record (dept_id);

COMMENT ON TABLE  ac_device_upgrade_record                   IS '设备升级记录';
COMMENT ON COLUMN ac_device_upgrade_record.id                IS '主键';
COMMENT ON COLUMN ac_device_upgrade_record.deleted_at        IS '软删除时间';
COMMENT ON COLUMN ac_device_upgrade_record.created_by        IS '创建人';
COMMENT ON COLUMN ac_device_upgrade_record.created_dt        IS '创建时间';
COMMENT ON COLUMN ac_device_upgrade_record.updated_by        IS '更新人';
COMMENT ON COLUMN ac_device_upgrade_record.updated_dt        IS '更新时间';
COMMENT ON COLUMN ac_device_upgrade_record.device_id         IS '设备ID';
COMMENT ON COLUMN ac_device_upgrade_record.device_code       IS '设备编号(冗余)';
COMMENT ON COLUMN ac_device_upgrade_record.device_name       IS '设备名称(冗余)';
COMMENT ON COLUMN ac_device_upgrade_record.region_id         IS '所属区域';
COMMENT ON COLUMN ac_device_upgrade_record.location          IS '设备位置(冗余)';
COMMENT ON COLUMN ac_device_upgrade_record.iot_card_no       IS '流量卡号(冗余)';
COMMENT ON COLUMN ac_device_upgrade_record.prev_firmware_id  IS '上一固件ID';
COMMENT ON COLUMN ac_device_upgrade_record.prev_firmware_ver IS '上一固件版本';
COMMENT ON COLUMN ac_device_upgrade_record.prev_ai_model_id  IS '上一模型ID';
COMMENT ON COLUMN ac_device_upgrade_record.prev_ai_model_ver IS '上一模型版本';
COMMENT ON COLUMN ac_device_upgrade_record.firmware_id       IS '升级固件ID';
COMMENT ON COLUMN ac_device_upgrade_record.firmware_ver      IS '升级固件版本';
COMMENT ON COLUMN ac_device_upgrade_record.ai_model_id       IS '升级模型ID';
COMMENT ON COLUMN ac_device_upgrade_record.ai_model_ver      IS '升级模型版本';
COMMENT ON COLUMN ac_device_upgrade_record.upgrade_type      IS '升级方式 now/plan';
COMMENT ON COLUMN ac_device_upgrade_record.upgrade_dt        IS '升级时间';
COMMENT ON COLUMN ac_device_upgrade_record.status            IS '升级结果';
COMMENT ON COLUMN ac_device_upgrade_record.operator          IS '操作人';
COMMENT ON COLUMN ac_device_upgrade_record.error_msg         IS '失败原因';
COMMENT ON COLUMN ac_device_upgrade_record.dept_id           IS '所属部门(数据权限)';
