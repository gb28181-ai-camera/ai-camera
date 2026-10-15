-- ================================================================
-- 01-系统基础 - DDL（16 张表）
-- AI Camera 智能监控平台 · v0.1.0
-- 覆盖：部门 / 用户 / 角色 / 菜单 / 菜单接口 / 岗位 / 权限关系 / 系统支撑
--
-- 说明：
--   1. 本文件是 GORM AutoMigrate 的等价基线结构，首次建库时执行；
--      后续新增列由 internal/bootstrap/migrate.go 的 AutoMigrate 增量补齐。
--   2. 列定义与 internal/model/*.go 的 gorm tag 逐字段对应，
--      修改模型后必须同步修改本文件（见 02-表关系全景图.md 第 5 节）。
--   3. 不写 DROP TABLE，全部使用 CREATE TABLE IF NOT EXISTS，可重复执行。
-- ================================================================

-- ----------------------------------------------------------------
-- ac_dept 部门（组织）；code 是 Casbin g2 主体
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_dept (
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
    parent_id         BIGINT,
    level             BIGINT        DEFAULT 1,
    path              TEXT,
    leader_id         BIGINT,
    description       TEXT
);

CREATE INDEX IF NOT EXISTS idx_ac_dept_deleted_at ON ac_dept (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_dept_parent_id  ON ac_dept (parent_id);

COMMENT ON TABLE  ac_dept                   IS '部门（组织）';
COMMENT ON COLUMN ac_dept.id                IS '主键';
COMMENT ON COLUMN ac_dept.deleted_at        IS '软删除时间';
COMMENT ON COLUMN ac_dept.created_by        IS '创建人';
COMMENT ON COLUMN ac_dept.created_dt        IS '创建时间';
COMMENT ON COLUMN ac_dept.updated_by        IS '更新人';
COMMENT ON COLUMN ac_dept.updated_dt        IS '更新时间';
COMMENT ON COLUMN ac_dept.active            IS '是否启用';
COMMENT ON COLUMN ac_dept.sequence          IS '排序';
COMMENT ON COLUMN ac_dept.name              IS '部门名称';
COMMENT ON COLUMN ac_dept.code              IS '部门编码(Casbin g2 主体)';
COMMENT ON COLUMN ac_dept.parent_id         IS '父部门ID';
COMMENT ON COLUMN ac_dept.level             IS '层级';
COMMENT ON COLUMN ac_dept.path              IS '层级路径';
COMMENT ON COLUMN ac_dept.leader_id         IS '负责人用户ID';
COMMENT ON COLUMN ac_dept.description       IS '描述';

-- ----------------------------------------------------------------
-- ac_user 平台登录账号；user_name 是 Casbin 主体（r.sub）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_user (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    name              VARCHAR(255)  NOT NULL,
    user_name         VARCHAR(100)  NOT NULL,
    password_hash     VARCHAR(255)  NOT NULL,
    email             VARCHAR(255),
    phone             VARCHAR(50),
    avatar            TEXT,
    dept_id           BIGINT,
    post_id           BIGINT,
    is_admin          BOOLEAN       DEFAULT FALSE,
    last_login        TIMESTAMPTZ,
    remark            VARCHAR(500)
);

CREATE INDEX        IF NOT EXISTS idx_ac_user_deleted_at ON ac_user (deleted_at);
CREATE INDEX        IF NOT EXISTS idx_ac_user_dept_id    ON ac_user (dept_id);
CREATE INDEX        IF NOT EXISTS idx_ac_user_post_id    ON ac_user (post_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_user_username   ON ac_user (user_name);

COMMENT ON TABLE  ac_user               IS '平台登录账号';
COMMENT ON COLUMN ac_user.id            IS '主键';
COMMENT ON COLUMN ac_user.deleted_at    IS '软删除时间';
COMMENT ON COLUMN ac_user.created_by    IS '创建人';
COMMENT ON COLUMN ac_user.created_dt    IS '创建时间';
COMMENT ON COLUMN ac_user.updated_by    IS '更新人';
COMMENT ON COLUMN ac_user.updated_dt    IS '更新时间';
COMMENT ON COLUMN ac_user.active        IS '是否启用';
COMMENT ON COLUMN ac_user.name          IS '姓名/显示名';
COMMENT ON COLUMN ac_user.user_name     IS '登录名';
COMMENT ON COLUMN ac_user.password_hash IS '密码哈希(bcrypt)';
COMMENT ON COLUMN ac_user.email         IS '邮箱';
COMMENT ON COLUMN ac_user.phone         IS '手机号';
COMMENT ON COLUMN ac_user.avatar        IS '头像';
COMMENT ON COLUMN ac_user.dept_id       IS '部门ID';
COMMENT ON COLUMN ac_user.post_id       IS '岗位ID';
COMMENT ON COLUMN ac_user.is_admin      IS '是否超级管理员';
COMMENT ON COLUMN ac_user.last_login    IS '最后登录时间';
COMMENT ON COLUMN ac_user.remark        IS '备注';

-- ----------------------------------------------------------------
-- ac_role 角色；code 是 Casbin 的 sub 值，必须唯一且稳定
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_role (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    sequence          BIGINT        DEFAULT 10,
    name              VARCHAR(255)  NOT NULL,
    code              VARCHAR(50)   NOT NULL,
    description       TEXT,
    data_permission   VARCHAR(20)   NOT NULL DEFAULT 'all',
    cross_dept_ids    TEXT
);

CREATE INDEX        IF NOT EXISTS idx_ac_role_deleted_at ON ac_role (deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_role_code       ON ac_role (code);

COMMENT ON TABLE  ac_role                 IS '角色';
COMMENT ON COLUMN ac_role.id              IS '主键';
COMMENT ON COLUMN ac_role.deleted_at      IS '软删除时间';
COMMENT ON COLUMN ac_role.created_by      IS '创建人';
COMMENT ON COLUMN ac_role.created_dt      IS '创建时间';
COMMENT ON COLUMN ac_role.updated_by      IS '更新人';
COMMENT ON COLUMN ac_role.updated_dt      IS '更新时间';
COMMENT ON COLUMN ac_role.active          IS '是否启用';
COMMENT ON COLUMN ac_role.sequence        IS '显示顺序';
COMMENT ON COLUMN ac_role.name            IS '角色名称';
COMMENT ON COLUMN ac_role.code            IS '权限字符(Casbin sub)';
COMMENT ON COLUMN ac_role.description     IS '描述';
COMMENT ON COLUMN ac_role.data_permission IS '数据权限 all/cross_dept/dept/personal';
COMMENT ON COLUMN ac_role.cross_dept_ids  IS '跨部门数据权限-部门ID列表';

-- ----------------------------------------------------------------
-- ac_menu 菜单/按钮；同时驱动前端侧边栏与按钮权限
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_menu (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    sequence          BIGINT        DEFAULT 10,
    name              VARCHAR(255)  NOT NULL,
    parent_id         BIGINT,
    level             BIGINT        DEFAULT 1,
    path              TEXT,
    icon              VARCHAR(100),
    component         VARCHAR(255),
    type              VARCHAR(20)   NOT NULL DEFAULT 'menu',
    platform          VARCHAR(10)   NOT NULL DEFAULT 'pc',
    permission_code   VARCHAR(100),
    description       TEXT,
    hidden            BOOLEAN       DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_ac_menu_deleted_at ON ac_menu (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_menu_parent_id  ON ac_menu (parent_id);

COMMENT ON TABLE  ac_menu                 IS '菜单/按钮';
COMMENT ON COLUMN ac_menu.id              IS '主键';
COMMENT ON COLUMN ac_menu.deleted_at      IS '软删除时间';
COMMENT ON COLUMN ac_menu.created_by      IS '创建人';
COMMENT ON COLUMN ac_menu.created_dt      IS '创建时间';
COMMENT ON COLUMN ac_menu.updated_by      IS '更新人';
COMMENT ON COLUMN ac_menu.updated_dt      IS '更新时间';
COMMENT ON COLUMN ac_menu.active          IS '是否启用';
COMMENT ON COLUMN ac_menu.sequence        IS '显示顺序';
COMMENT ON COLUMN ac_menu.name            IS '菜单名称';
COMMENT ON COLUMN ac_menu.parent_id       IS '父菜单ID';
COMMENT ON COLUMN ac_menu.level           IS '层级';
COMMENT ON COLUMN ac_menu.path            IS '路由路径';
COMMENT ON COLUMN ac_menu.icon            IS '图标';
COMMENT ON COLUMN ac_menu.component       IS '前端组件';
COMMENT ON COLUMN ac_menu.type            IS 'directory/menu/button';
COMMENT ON COLUMN ac_menu.platform        IS 'pc/mobile';
COMMENT ON COLUMN ac_menu.permission_code IS '权限标识(按钮级)';
COMMENT ON COLUMN ac_menu.description     IS '描述';
COMMENT ON COLUMN ac_menu.hidden          IS '是否隐藏';

-- ----------------------------------------------------------------
-- ac_menu_api 菜单绑定的后端接口；Casbin p 策略的唯一来源
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_menu_api (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    menu_id           BIGINT        NOT NULL,
    method            VARCHAR(10)   NOT NULL,
    path              VARCHAR(255)  NOT NULL,
    description       VARCHAR(500)
);

CREATE INDEX IF NOT EXISTS idx_ac_menu_api_deleted_at ON ac_menu_api (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_menu_api_menu_id    ON ac_menu_api (menu_id);

COMMENT ON TABLE  ac_menu_api             IS '菜单绑定的后端接口(Casbin p 策略来源)';
COMMENT ON COLUMN ac_menu_api.id          IS '主键';
COMMENT ON COLUMN ac_menu_api.deleted_at  IS '软删除时间';
COMMENT ON COLUMN ac_menu_api.created_by  IS '创建人';
COMMENT ON COLUMN ac_menu_api.created_dt  IS '创建时间';
COMMENT ON COLUMN ac_menu_api.updated_by  IS '更新人';
COMMENT ON COLUMN ac_menu_api.updated_dt  IS '更新时间';
COMMENT ON COLUMN ac_menu_api.menu_id     IS '菜单ID';
COMMENT ON COLUMN ac_menu_api.method      IS 'HTTP方法';
COMMENT ON COLUMN ac_menu_api.path        IS '接口路径';
COMMENT ON COLUMN ac_menu_api.description IS '说明';

-- ----------------------------------------------------------------
-- ac_post 岗位；code 是 Casbin g3 主体
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_post (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    sequence          BIGINT        DEFAULT 10,
    name              VARCHAR(255)  NOT NULL,
    code              VARCHAR(50)   NOT NULL,
    description       TEXT
);

CREATE INDEX        IF NOT EXISTS idx_ac_post_deleted_at ON ac_post (deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_post_code       ON ac_post (code);

COMMENT ON TABLE  ac_post               IS '岗位';
COMMENT ON COLUMN ac_post.id            IS '主键';
COMMENT ON COLUMN ac_post.deleted_at    IS '软删除时间';
COMMENT ON COLUMN ac_post.created_by    IS '创建人';
COMMENT ON COLUMN ac_post.created_dt    IS '创建时间';
COMMENT ON COLUMN ac_post.updated_by    IS '更新人';
COMMENT ON COLUMN ac_post.updated_dt    IS '更新时间';
COMMENT ON COLUMN ac_post.active        IS '是否启用';
COMMENT ON COLUMN ac_post.sequence      IS '排序';
COMMENT ON COLUMN ac_post.name          IS '岗位名称';
COMMENT ON COLUMN ac_post.code          IS '岗位编码(Casbin g3 主体)';
COMMENT ON COLUMN ac_post.description   IS '描述';

-- ----------------------------------------------------------------
-- ac_user_role_rel 用户-角色关联（Casbin g 策略来源）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_user_role_rel (
    id                BIGSERIAL     PRIMARY KEY,
    user_id           BIGINT        NOT NULL,
    role_id           BIGINT        NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_user_role ON ac_user_role_rel (user_id, role_id);

COMMENT ON TABLE  ac_user_role_rel          IS '用户-角色关联';
COMMENT ON COLUMN ac_user_role_rel.id       IS '主键';
COMMENT ON COLUMN ac_user_role_rel.user_id  IS '用户ID';
COMMENT ON COLUMN ac_user_role_rel.role_id  IS '角色ID';

-- ----------------------------------------------------------------
-- ac_user_post_rel 用户-岗位关联
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_user_post_rel (
    id                BIGSERIAL     PRIMARY KEY,
    user_id           BIGINT        NOT NULL,
    post_id           BIGINT        NOT NULL,
    is_primary        BOOLEAN       DEFAULT FALSE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_user_post ON ac_user_post_rel (user_id, post_id);

COMMENT ON TABLE  ac_user_post_rel            IS '用户-岗位关联';
COMMENT ON COLUMN ac_user_post_rel.id         IS '主键';
COMMENT ON COLUMN ac_user_post_rel.user_id    IS '用户ID';
COMMENT ON COLUMN ac_user_post_rel.post_id    IS '岗位ID';
COMMENT ON COLUMN ac_user_post_rel.is_primary IS '是否主岗位';

-- ----------------------------------------------------------------
-- ac_post_role_rel 岗位-角色关联（Casbin g3 策略来源）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_post_role_rel (
    id                BIGSERIAL     PRIMARY KEY,
    post_id           BIGINT        NOT NULL,
    role_id           BIGINT        NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_post_role ON ac_post_role_rel (post_id, role_id);

COMMENT ON TABLE  ac_post_role_rel          IS '岗位-角色关联';
COMMENT ON COLUMN ac_post_role_rel.id       IS '主键';
COMMENT ON COLUMN ac_post_role_rel.post_id  IS '岗位ID';
COMMENT ON COLUMN ac_post_role_rel.role_id  IS '角色ID';

-- ----------------------------------------------------------------
-- ac_dept_role_rel 部门-角色关联（Casbin g2 策略来源）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_dept_role_rel (
    id                BIGSERIAL     PRIMARY KEY,
    dept_id           BIGINT        NOT NULL,
    role_id           BIGINT        NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_dept_role ON ac_dept_role_rel (dept_id, role_id);

COMMENT ON TABLE  ac_dept_role_rel          IS '部门-角色关联';
COMMENT ON COLUMN ac_dept_role_rel.id       IS '主键';
COMMENT ON COLUMN ac_dept_role_rel.dept_id  IS '部门ID';
COMMENT ON COLUMN ac_dept_role_rel.role_id  IS '角色ID';

-- ----------------------------------------------------------------
-- ac_role_menu_rel 角色-菜单关联（角色授权主表）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_role_menu_rel (
    id                BIGSERIAL     PRIMARY KEY,
    role_id           BIGINT        NOT NULL,
    menu_id           BIGINT        NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_role_menu ON ac_role_menu_rel (role_id, menu_id);

COMMENT ON TABLE  ac_role_menu_rel          IS '角色-菜单关联';
COMMENT ON COLUMN ac_role_menu_rel.id       IS '主键';
COMMENT ON COLUMN ac_role_menu_rel.role_id  IS '角色ID';
COMMENT ON COLUMN ac_role_menu_rel.menu_id  IS '菜单ID';

-- ----------------------------------------------------------------
-- ac_system_config 系统参数
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_system_config (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    name              VARCHAR(255)  NOT NULL,
    code              VARCHAR(100)  NOT NULL,
    "value"           TEXT,
    group_name        VARCHAR(50),
    data_type         VARCHAR(20)   DEFAULT 'string',
    sequence          BIGINT        DEFAULT 10,
    description       TEXT
);

CREATE INDEX IF NOT EXISTS idx_ac_system_config_deleted_at ON ac_system_config (deleted_at);

COMMENT ON TABLE  ac_system_config             IS '系统参数';
COMMENT ON COLUMN ac_system_config.id          IS '主键';
COMMENT ON COLUMN ac_system_config.deleted_at  IS '软删除时间';
COMMENT ON COLUMN ac_system_config.created_by  IS '创建人';
COMMENT ON COLUMN ac_system_config.created_dt  IS '创建时间';
COMMENT ON COLUMN ac_system_config.updated_by  IS '更新人';
COMMENT ON COLUMN ac_system_config.updated_dt  IS '更新时间';
COMMENT ON COLUMN ac_system_config.active      IS '是否启用';
COMMENT ON COLUMN ac_system_config.name        IS '参数名称';
COMMENT ON COLUMN ac_system_config.code        IS '参数键';
COMMENT ON COLUMN ac_system_config."value"     IS '参数值';
COMMENT ON COLUMN ac_system_config.group_name  IS '分组';
COMMENT ON COLUMN ac_system_config.data_type   IS 'string/int/bool/json';
COMMENT ON COLUMN ac_system_config.sequence    IS '排序';
COMMENT ON COLUMN ac_system_config.description IS '说明';

-- ----------------------------------------------------------------
-- ac_api_key 开放接口密钥（设备回调等无用户上下文场景）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_api_key (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    user_id           BIGINT        NOT NULL,
    name              VARCHAR(100),
    key_prefix        VARCHAR(20),
    key_hash          VARCHAR(255),
    last_used_dt      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ac_api_key_deleted_at ON ac_api_key (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_api_key_user_id    ON ac_api_key (user_id);

COMMENT ON TABLE  ac_api_key              IS '开放接口密钥';
COMMENT ON COLUMN ac_api_key.id           IS '主键';
COMMENT ON COLUMN ac_api_key.deleted_at   IS '软删除时间';
COMMENT ON COLUMN ac_api_key.created_by   IS '创建人';
COMMENT ON COLUMN ac_api_key.created_dt   IS '创建时间';
COMMENT ON COLUMN ac_api_key.updated_by   IS '更新人';
COMMENT ON COLUMN ac_api_key.updated_dt   IS '更新时间';
COMMENT ON COLUMN ac_api_key.active       IS '是否启用';
COMMENT ON COLUMN ac_api_key.user_id      IS '归属用户ID';
COMMENT ON COLUMN ac_api_key.name         IS '名称';
COMMENT ON COLUMN ac_api_key.key_prefix   IS '密钥前缀(用于展示)';
COMMENT ON COLUMN ac_api_key.key_hash     IS '密钥SHA256';
COMMENT ON COLUMN ac_api_key.last_used_dt IS '最后使用时间';

-- ----------------------------------------------------------------
-- ac_operation_log 操作日志
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_operation_log (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    module            VARCHAR(100),
    action            VARCHAR(20),
    method            VARCHAR(10),
    path              VARCHAR(255),
    operator          VARCHAR(100),
    ip                VARCHAR(64),
    location          VARCHAR(255),
    status            BIGINT,
    error_msg         TEXT,
    params            TEXT,
    cost_ms           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_operation_log_deleted_at ON ac_operation_log (deleted_at);

COMMENT ON TABLE  ac_operation_log             IS '操作日志';
COMMENT ON COLUMN ac_operation_log.id          IS '主键';
COMMENT ON COLUMN ac_operation_log.deleted_at  IS '软删除时间';
COMMENT ON COLUMN ac_operation_log.created_by  IS '创建人';
COMMENT ON COLUMN ac_operation_log.created_dt  IS '创建时间';
COMMENT ON COLUMN ac_operation_log.updated_by  IS '更新人';
COMMENT ON COLUMN ac_operation_log.updated_dt  IS '更新时间';
COMMENT ON COLUMN ac_operation_log.module      IS '系统模块';
COMMENT ON COLUMN ac_operation_log.action      IS '操作类型';
COMMENT ON COLUMN ac_operation_log.method      IS '请求方式';
COMMENT ON COLUMN ac_operation_log.path        IS '请求路径';
COMMENT ON COLUMN ac_operation_log.operator    IS '操作人员';
COMMENT ON COLUMN ac_operation_log.ip          IS '主机';
COMMENT ON COLUMN ac_operation_log.location    IS '操作地点';
COMMENT ON COLUMN ac_operation_log.status      IS '状态 1成功 0失败';
COMMENT ON COLUMN ac_operation_log.error_msg   IS '错误信息';
COMMENT ON COLUMN ac_operation_log.params      IS '请求参数';
COMMENT ON COLUMN ac_operation_log.cost_ms     IS '耗时(毫秒)';

-- ----------------------------------------------------------------
-- ac_notification 系统通知（设备告警/风险通知/巡检通知）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_notification (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    user_id           BIGINT        NOT NULL,
    category          VARCHAR(20)   NOT NULL,
    title             VARCHAR(255),
    content           TEXT,
    ref_type          VARCHAR(50),
    ref_id            BIGINT,
    "read"            BOOLEAN       DEFAULT FALSE,
    read_dt           TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ac_notification_deleted_at ON ac_notification (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_notification_user_id    ON ac_notification (user_id);

COMMENT ON TABLE  ac_notification             IS '系统通知';
COMMENT ON COLUMN ac_notification.id          IS '主键';
COMMENT ON COLUMN ac_notification.deleted_at  IS '软删除时间';
COMMENT ON COLUMN ac_notification.created_by  IS '创建人';
COMMENT ON COLUMN ac_notification.created_dt  IS '创建时间';
COMMENT ON COLUMN ac_notification.updated_by  IS '更新人';
COMMENT ON COLUMN ac_notification.updated_dt  IS '更新时间';
COMMENT ON COLUMN ac_notification.user_id     IS '接收人';
COMMENT ON COLUMN ac_notification.category    IS 'alarm/risk/inspect';
COMMENT ON COLUMN ac_notification.title       IS '标题';
COMMENT ON COLUMN ac_notification.content     IS '内容';
COMMENT ON COLUMN ac_notification.ref_type    IS '关联业务类型';
COMMENT ON COLUMN ac_notification.ref_id      IS '关联业务ID';
COMMENT ON COLUMN ac_notification."read"      IS '是否已读';
COMMENT ON COLUMN ac_notification.read_dt     IS '已读时间';

-- ----------------------------------------------------------------
-- ac_sequence 序列号生成器
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_sequence (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    code              VARCHAR(50)   NOT NULL,
    name              VARCHAR(255),
    prefix            VARCHAR(20),
    current_val       BIGINT        DEFAULT 0,
    step              BIGINT        DEFAULT 1,
    date_pattern      VARCHAR(20),
    reset_by_day      BOOLEAN       DEFAULT TRUE
);

CREATE INDEX        IF NOT EXISTS idx_ac_sequence_deleted_at ON ac_sequence (deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_sequence_code       ON ac_sequence (code);

COMMENT ON TABLE  ac_sequence              IS '序列号生成器';
COMMENT ON COLUMN ac_sequence.id           IS '主键';
COMMENT ON COLUMN ac_sequence.deleted_at   IS '软删除时间';
COMMENT ON COLUMN ac_sequence.created_by   IS '创建人';
COMMENT ON COLUMN ac_sequence.created_dt   IS '创建时间';
COMMENT ON COLUMN ac_sequence.updated_by   IS '更新人';
COMMENT ON COLUMN ac_sequence.updated_dt   IS '更新时间';
COMMENT ON COLUMN ac_sequence.code         IS '序列编码';
COMMENT ON COLUMN ac_sequence.name         IS '序列名称';
COMMENT ON COLUMN ac_sequence.prefix       IS '前缀';
COMMENT ON COLUMN ac_sequence.current_val  IS '当前值';
COMMENT ON COLUMN ac_sequence.step         IS '步长';
COMMENT ON COLUMN ac_sequence.date_pattern IS '日期格式(如 20060102)';
COMMENT ON COLUMN ac_sequence.reset_by_day IS '是否按日重置';
