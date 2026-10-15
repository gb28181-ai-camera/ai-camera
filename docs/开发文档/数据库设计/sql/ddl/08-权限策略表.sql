-- ================================================================
-- 08-权限策略表 - DDL（1 张表）
-- AI Camera 智能监控平台 · v0.1.0
-- 覆盖：casbin_rule（Casbin 策略持久化表）
--
-- 说明：
--   1. 本表由 gorm-adapter v3（github.com/casbin/gorm-adapter/v3）在
--      perm.NewCasbinService() 首次运行时**自动创建**，
--      正式环境无需手工建表。本文件给出等价 DDL，
--      用于「先建库、后跑服务」的部署顺序，或需要在建库阶段
--      直接灌入策略的离线场景。
--   2. 表结构对应 gorm-adapter 的 CasbinRule 模型：
--      p_type + v0..v5 六个变长字段承载不同策略类型的参数。
--   3. 本项目实际使用的列：
--        p  策略：v0 = 角色编码(role.code)、v1 = 接口路径、v2 = HTTP 方法
--        g  策略：v0 = 登录名(user.user_name)、v1 = 角色编码
--        g2 策略：v0 = 部门编码(dept.code)、v1 = 角色编码
--        g3 策略：v0 = 岗位编码(post.code)、v1 = 角色编码
--      v3 ~ v5 由适配器保留，本项目不使用。
--   4. 若执行本文件时表已由适配器创建，CREATE TABLE IF NOT EXISTS 直接跳过。
--   5. **列名必须是 p_type（带下划线），不能写成 ptype** —— 此为实测确认结论：
--      gorm-adapter 的 CasbinRule.PType 字段经 GORM 命名策略映射为列名 p_type，
--      GORM 按列名匹配字段，列名不一致时 PType 会读到零值 ""，
--      casbin 随后在 persist.LoadPolicyArray 的 key[:1] 处 panic
--      （slice bounds out of range [:1] with length 0），后端直接启动失败。
--      实测：列名 ptype -> 读到 PType=""；列名 p_type -> 读到 PType="p"。
-- ================================================================

CREATE TABLE IF NOT EXISTS casbin_rule (
    id                BIGSERIAL     PRIMARY KEY,
    p_type             VARCHAR(100),
    v0                VARCHAR(100),
    v1                VARCHAR(100),
    v2                VARCHAR(100),
    v3                VARCHAR(100),
    v4                VARCHAR(100),
    v5                VARCHAR(100)
);

-- 适配器按 p_type + v0..v5 建唯一索引，保证策略不重复写入
CREATE UNIQUE INDEX IF NOT EXISTS idx_casbin_rule
    ON casbin_rule (p_type, v0, v1, v2, v3, v4, v5);

COMMENT ON TABLE  casbin_rule       IS 'Casbin 策略表(gorm-adapter 自动创建)';
COMMENT ON COLUMN casbin_rule.id    IS '主键';
COMMENT ON COLUMN casbin_rule.p_type IS '策略类型 p/g/g2/g3';
COMMENT ON COLUMN casbin_rule.v0    IS '参数0：p=角色编码 / g=登录名 / g2=部门编码 / g3=岗位编码';
COMMENT ON COLUMN casbin_rule.v1    IS '参数1：p=接口路径 / g,g2,g3=角色编码';
COMMENT ON COLUMN casbin_rule.v2    IS '参数2：p=HTTP方法(GET/POST/PUT/DELETE)';
COMMENT ON COLUMN casbin_rule.v3    IS '保留字段';
COMMENT ON COLUMN casbin_rule.v4    IS '保留字段';
COMMENT ON COLUMN casbin_rule.v5    IS '保留字段';
