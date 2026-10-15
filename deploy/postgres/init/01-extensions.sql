-- ================================================================
-- AI Camera 数据库初始化 —— 扩展与多库创建
--
-- 仅在数据目录为空（首次启动容器）时执行一次。
-- 执行身份：POSTGRES_USER，默认库：POSTGRES_DB（ai_camera_dev）。
--
-- 两个扩展：
--   postgis —— 地理空间（设备 GIS 定位、国标移动位置轨迹）
--   vector  —— 向量检索（pgvector，AI 风险图片相似检索）
-- ================================================================

-- ----------------------------------------------------------------
-- 1. 主库（POSTGRES_DB，默认 ai_camera_dev）创建扩展
-- ----------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS vector;

-- ----------------------------------------------------------------
-- 2. 测试库 —— 与本项目「开发库 / 测试库分离」的约定一致
--    后端单测与冒烟测试必须使用 ai_camera_test，禁止操作开发库。
--    CREATE DATABASE 不能在事务中执行，用 \gexec 做条件创建。
-- ----------------------------------------------------------------
SELECT 'CREATE DATABASE ai_camera_test OWNER ' || quote_ident(current_user)
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'ai_camera_test')\gexec

\connect ai_camera_test
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS vector;

-- 回到主库，避免后续脚本在错误的库上执行
\connect :"POSTGRES_DB"

-- ----------------------------------------------------------------
-- 3. 自检输出（首次启动时在日志中可见）
-- ----------------------------------------------------------------
DO $$
DECLARE
    db   text;
    ext  text;
BEGIN
    RAISE NOTICE '── AI Camera 数据库初始化自检 ──';
    FOR db, ext IN
        SELECT d.datname, e.extname
        FROM pg_database d
        JOIN pg_extension e ON e.extname IN ('postgis', 'vector')
        WHERE d.datname IN ('ai_camera_dev', 'ai_camera_test')
        ORDER BY d.datname, e.extname
    LOOP
        RAISE NOTICE '  % → %', db, ext;
    END LOOP;
END
$$;
