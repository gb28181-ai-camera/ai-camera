-- ================================================================
-- 15-语音Agent - DDL（8 张新表 + ac_voice_session 扩展）
-- AI Camera 智能监控平台 · v0.6.0 智能语音聊天
--
-- 设计依据：docs/敏捷开发/v0.6.0 智能语音聊天/00-设计与协议.md §7
-- 架构：Go WS 网关（/web|app|device/voice/ws）→ Pipecat /ws/voice
--       Pipecat → Go 内部 MCP（/internal/mcp）与记忆（/internal/voice/memory/*）
--
-- 说明：
--   1. 不新建语音会话表：ac_voice_session 已承载 GB28181 广播/对讲，
--      本文件只给它追加 8 列，避免同一会话两套主键与生命周期。
--   2. ac_voice_memory.embedding 使用 pgvector vector(1024)（默认 BGE-M3），
--      建 HNSW 余弦索引；pgvector 0.8.x 支持 HNSW，无需降级 ivfflat。
--   3. ac_voice_mcp_call_log 为追加写审计表，不做更新与删除。
--   4. 记忆去重：owner_key + content_hash 唯一（软删除行不参与）。
-- ================================================================

CREATE EXTENSION IF NOT EXISTS vector;


-- ================================================================
-- 第 0 节：ac_voice_session 扩展（WebSocket 传输与会话统计）
-- ----------------------------------------------------------------
-- transport       rtp（GB28181 广播/对讲）/ websocket（App/Web/设备）
-- client_id       App/Web/设备连接标识（hello.client.client_id）
-- agent_code      关联 ac_voice_agent.code
-- input_format    上行音频格式 pcm16/opus
-- output_format   下行音频格式 pcm16/opus
-- last_seen_at    最近活跃时间（Pipecat/Pipecat 心跳）
-- turn_count      对话轮次
-- close_code      WebSocket 关闭码（1000/1001/...）
-- ----------------------------------------------------------------

ALTER TABLE ac_voice_session ADD COLUMN IF NOT EXISTS transport     VARCHAR(20) DEFAULT 'rtp';
ALTER TABLE ac_voice_session ADD COLUMN IF NOT EXISTS client_id     VARCHAR(100);
ALTER TABLE ac_voice_session ADD COLUMN IF NOT EXISTS agent_code    VARCHAR(100);
ALTER TABLE ac_voice_session ADD COLUMN IF NOT EXISTS input_format  VARCHAR(20);
ALTER TABLE ac_voice_session ADD COLUMN IF NOT EXISTS output_format VARCHAR(20);
ALTER TABLE ac_voice_session ADD COLUMN IF NOT EXISTS last_seen_at  TIMESTAMPTZ;
ALTER TABLE ac_voice_session ADD COLUMN IF NOT EXISTS turn_count    INTEGER     DEFAULT 0;
ALTER TABLE ac_voice_session ADD COLUMN IF NOT EXISTS close_code    INTEGER     DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_ac_voice_session_transport ON ac_voice_session (transport);
CREATE INDEX IF NOT EXISTS idx_ac_voice_session_last_seen ON ac_voice_session (last_seen_at);

COMMENT ON COLUMN ac_voice_session.transport     IS '传输方式：rtp/websocket';
COMMENT ON COLUMN ac_voice_session.client_id     IS '客户端标识（App/Web/设备）';
COMMENT ON COLUMN ac_voice_session.agent_code    IS '语音Agent编码';
COMMENT ON COLUMN ac_voice_session.input_format  IS '上行音频格式：pcm16/opus';
COMMENT ON COLUMN ac_voice_session.output_format IS '下行音频格式：pcm16/opus';
COMMENT ON COLUMN ac_voice_session.last_seen_at  IS '最近活跃时间';
COMMENT ON COLUMN ac_voice_session.turn_count    IS '对话轮次';
COMMENT ON COLUMN ac_voice_session.close_code    IS 'WebSocket关闭码';


-- ================================================================
-- 第 1 节：ac_voice_agent（Agent 人格）
-- ----------------------------------------------------------------
-- 与 ac_thirdparty_config(Category='voice_agent') 分工：
--   配置表存 provider/密钥；本表存可多套并存的人格与提示词。
-- ----------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ac_voice_agent (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    sequence          INTEGER       DEFAULT 10,
    code              VARCHAR(100)  NOT NULL,
    name              VARCHAR(255)  NOT NULL,
    description       TEXT,
    greeting          TEXT,
    wake_word         VARCHAR(100),
    voice             VARCHAR(100),
    model             VARCHAR(100),
    system_prompt     TEXT,
    persona           TEXT,
    extra             TEXT,
    dept_id           BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_voice_agent_code       ON ac_voice_agent (code);
CREATE INDEX        IF NOT EXISTS idx_ac_voice_agent_dept_id    ON ac_voice_agent (dept_id);
CREATE INDEX        IF NOT EXISTS idx_ac_voice_agent_deleted_at ON ac_voice_agent (deleted_at);

COMMENT ON TABLE  ac_voice_agent               IS '语音Agent人格配置';
COMMENT ON COLUMN ac_voice_agent.code          IS 'Agent编码（唯一）';
COMMENT ON COLUMN ac_voice_agent.wake_word     IS '设备端唤醒词';
COMMENT ON COLUMN ac_voice_agent.voice         IS 'TTS音色';
COMMENT ON COLUMN ac_voice_agent.model         IS 'LLM模型';
COMMENT ON COLUMN ac_voice_agent.persona       IS '人格设定（JSON字符串）';


-- ================================================================
-- 第 2 节：ac_voice_mcp_server（MCP 服务端注册）
-- ----------------------------------------------------------------
-- 首期内置工具由 Go 自身提供（source=internal），本表为外部 MCP 预留。
-- token 为敏感字段，Web 读接口一律不返回。
-- ----------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ac_voice_mcp_server (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    code              VARCHAR(100)  NOT NULL,
    name              VARCHAR(255)  NOT NULL,
    transport         VARCHAR(20)   DEFAULT 'http',
    url               VARCHAR(500),
    token             VARCHAR(500),
    timeout_ms        INTEGER       DEFAULT 10000,
    description       TEXT,
    dept_id           BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_voice_mcp_server_code       ON ac_voice_mcp_server (code);
CREATE INDEX        IF NOT EXISTS idx_ac_voice_mcp_server_dept_id    ON ac_voice_mcp_server (dept_id);
CREATE INDEX        IF NOT EXISTS idx_ac_voice_mcp_server_deleted_at ON ac_voice_mcp_server (deleted_at);

COMMENT ON TABLE  ac_voice_mcp_server          IS 'MCP服务端注册';
COMMENT ON COLUMN ac_voice_mcp_server.transport IS '传输：http/stdio/ws';
COMMENT ON COLUMN ac_voice_mcp_server.token     IS '鉴权令牌（敏感，不返回）';


-- ================================================================
-- 第 3 节：ac_voice_mcp_tool（MCP 工具）
-- ----------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ac_voice_mcp_tool (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    server_id         BIGINT,
    source            VARCHAR(20)   DEFAULT 'internal',
    name              VARCHAR(100)  NOT NULL,
    wire_name         VARCHAR(100),
    display_name      VARCHAR(255),
    description       TEXT,
    input_schema      TEXT,
    risk_level        VARCHAR(20)   DEFAULT 'low',
    require_confirm   BOOLEAN       DEFAULT FALSE,
    enabled           BOOLEAN       DEFAULT TRUE,
    dept_id           BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_voice_mcp_tool_name       ON ac_voice_mcp_tool (name);
CREATE INDEX        IF NOT EXISTS idx_ac_voice_mcp_tool_server_id  ON ac_voice_mcp_tool (server_id);
CREATE INDEX        IF NOT EXISTS idx_ac_voice_mcp_tool_dept_id    ON ac_voice_mcp_tool (dept_id);
CREATE INDEX        IF NOT EXISTS idx_ac_voice_mcp_tool_deleted_at ON ac_voice_mcp_tool (deleted_at);

COMMENT ON TABLE  ac_voice_mcp_tool              IS 'MCP工具登记';
COMMENT ON COLUMN ac_voice_mcp_tool.name         IS '工具名（LLM可见，唯一）';
COMMENT ON COLUMN ac_voice_mcp_tool.wire_name    IS '原始工具名（设备侧映射用）';
COMMENT ON COLUMN ac_voice_mcp_tool.input_schema IS '输入JSON Schema';
COMMENT ON COLUMN ac_voice_mcp_tool.risk_level   IS '风险等级：low/medium/high';
COMMENT ON COLUMN ac_voice_mcp_tool.require_confirm IS '写操作是否要求 confirm=true';


-- ================================================================
-- 第 4 节：ac_voice_tool_permission（工具权限）
-- ----------------------------------------------------------------
-- subject_type=role 时 subject_id 为角色编码；其余为数字ID字符串。
-- 同一工具下存在任意规则时默认拒绝：必须命中 allow 且不命中 deny。
-- ----------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ac_voice_tool_permission (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    tool_id           BIGINT        NOT NULL,
    subject_type      VARCHAR(20)   NOT NULL,
    subject_id        VARCHAR(100)  NOT NULL,
    effect            VARCHAR(10)   NOT NULL DEFAULT 'allow',
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_voice_tool_permission_tool       ON ac_voice_tool_permission (tool_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_tool_permission_subject    ON ac_voice_tool_permission (subject_type, subject_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_tool_permission_dept_id    ON ac_voice_tool_permission (dept_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_tool_permission_deleted_at ON ac_voice_tool_permission (deleted_at);

COMMENT ON TABLE  ac_voice_tool_permission              IS 'MCP工具权限';
COMMENT ON COLUMN ac_voice_tool_permission.subject_type IS '主体类型：role/user/dept/device';
COMMENT ON COLUMN ac_voice_tool_permission.subject_id   IS '主体标识（角色编码或ID）';
COMMENT ON COLUMN ac_voice_tool_permission.effect       IS '效果：allow/deny';


-- ================================================================
-- 第 5 节：ac_voice_mcp_call_log（工具调用审计，追加写）
-- ----------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ac_voice_mcp_call_log (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    session_id        VARCHAR(100),
    user_id           BIGINT,
    device_id         BIGINT,
    server_code       VARCHAR(100),
    tool_name         VARCHAR(100)  NOT NULL,
    risk_level        VARCHAR(20),
    confirmed         BOOLEAN       DEFAULT FALSE,
    status            VARCHAR(20)   NOT NULL,
    args              TEXT,
    result            TEXT,
    error_msg         VARCHAR(1000),
    elapsed_ms        BIGINT,
    ip                VARCHAR(64),
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_voice_mcp_call_log_tool       ON ac_voice_mcp_call_log (tool_name);
CREATE INDEX IF NOT EXISTS idx_ac_voice_mcp_call_log_status     ON ac_voice_mcp_call_log (status);
CREATE INDEX IF NOT EXISTS idx_ac_voice_mcp_call_log_user       ON ac_voice_mcp_call_log (user_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_mcp_call_log_device     ON ac_voice_mcp_call_log (device_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_mcp_call_log_session    ON ac_voice_mcp_call_log (session_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_mcp_call_log_created_dt ON ac_voice_mcp_call_log (created_dt);
CREATE INDEX IF NOT EXISTS idx_ac_voice_mcp_call_log_dept_id    ON ac_voice_mcp_call_log (dept_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_mcp_call_log_deleted_at ON ac_voice_mcp_call_log (deleted_at);

COMMENT ON TABLE  ac_voice_mcp_call_log            IS 'MCP工具调用审计（追加写）';
COMMENT ON COLUMN ac_voice_mcp_call_log.status     IS '结果：ok/error/denied';
COMMENT ON COLUMN ac_voice_mcp_call_log.args       IS '调用参数（截断）';
COMMENT ON COLUMN ac_voice_mcp_call_log.result     IS '结果摘要（截断）';


-- ================================================================
-- 第 6 节：ac_voice_memory（pgvector 长期记忆）
-- ----------------------------------------------------------------
-- embedding 维度固定 1024（BGE-M3 默认）。未配置 embedding 服务时
-- embedding 为 NULL、embedding_status=failed，检索降级为时间/关键词。
-- HNSW 需要 pgvector >= 0.5.0；本项目镜像为 0.8.x。
-- ----------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ac_voice_memory (
    id                 BIGSERIAL     PRIMARY KEY,
    deleted_at         TIMESTAMPTZ,
    created_by         VARCHAR(50),
    created_dt         TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by         VARCHAR(50),
    updated_dt         TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    owner_type         VARCHAR(20)   NOT NULL,
    owner_id           BIGINT        DEFAULT 0,
    owner_key          VARCHAR(100)  NOT NULL,
    kind               VARCHAR(30)   DEFAULT 'fact',
    content            TEXT          NOT NULL,
    content_hash       VARCHAR(64)   NOT NULL,
    importance         INTEGER       DEFAULT 3,
    embedding          vector(1024),
    embedding_status   VARCHAR(20)   DEFAULT 'pending',
    source_session_id  VARCHAR(100),
    turn_id            VARCHAR(100),
    last_access_dt     TIMESTAMPTZ,
    expires_dt         TIMESTAMPTZ,
    user_id            BIGINT,
    dept_id            BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_voice_memory_owner       ON ac_voice_memory (owner_type, owner_key);
CREATE INDEX IF NOT EXISTS idx_ac_voice_memory_kind        ON ac_voice_memory (kind);
CREATE INDEX IF NOT EXISTS idx_ac_voice_memory_status      ON ac_voice_memory (embedding_status);
CREATE INDEX IF NOT EXISTS idx_ac_voice_memory_expires     ON ac_voice_memory (expires_dt);
CREATE INDEX IF NOT EXISTS idx_ac_voice_memory_user        ON ac_voice_memory (user_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_memory_dept_id     ON ac_voice_memory (dept_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_memory_deleted_at  ON ac_voice_memory (deleted_at);

-- 去重：同一归属下同一内容只保留一行（软删除行不参与）。
CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_voice_memory_dedup
    ON ac_voice_memory (owner_key, content_hash)
    WHERE deleted_at IS NULL;

-- 向量索引：余弦距离（<=>）。
CREATE INDEX IF NOT EXISTS idx_ac_voice_memory_embedding
    ON ac_voice_memory USING hnsw (embedding vector_cosine_ops);

COMMENT ON TABLE  ac_voice_memory                  IS '长期记忆（pgvector）';
COMMENT ON COLUMN ac_voice_memory.owner_type       IS '归属类型：user/device/dept/global';
COMMENT ON COLUMN ac_voice_memory.owner_key        IS '归属键：user:1 / device:2 / dept:3 / global';
COMMENT ON COLUMN ac_voice_memory.content_hash     IS '内容SHA-256（去重）';
COMMENT ON COLUMN ac_voice_memory.importance       IS '重要度 1-5';
COMMENT ON COLUMN ac_voice_memory.embedding        IS '向量（1024维，BGE-M3）';
COMMENT ON COLUMN ac_voice_memory.embedding_status IS 'embedding状态：ok/pending/failed';


-- ================================================================
-- 第 7 节：ac_voice_session_summary（会话摘要）
-- ================================================================

CREATE TABLE IF NOT EXISTS ac_voice_session_summary (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    session_id        VARCHAR(100)  NOT NULL,
    user_id           BIGINT,
    device_id         BIGINT,
    summary           TEXT,
    message_count     INTEGER       DEFAULT 0,
    started_dt        TIMESTAMPTZ,
    ended_dt          TIMESTAMPTZ,
    expires_dt        TIMESTAMPTZ,
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_voice_session_summary_session    ON ac_voice_session_summary (session_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_session_summary_user       ON ac_voice_session_summary (user_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_session_summary_device     ON ac_voice_session_summary (device_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_session_summary_expires    ON ac_voice_session_summary (expires_dt);
CREATE INDEX IF NOT EXISTS idx_ac_voice_session_summary_dept_id    ON ac_voice_session_summary (dept_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_session_summary_deleted_at ON ac_voice_session_summary (deleted_at);

COMMENT ON TABLE ac_voice_session_summary IS '语音会话摘要（短期记忆）';


-- ================================================================
-- 第 8 节：ac_voice_turn（逐轮记录，可选逐字稿）
-- ================================================================

CREATE TABLE IF NOT EXISTS ac_voice_turn (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    session_id        VARCHAR(100)  NOT NULL,
    turn_id           VARCHAR(100),
    seq               INTEGER       DEFAULT 0,
    role              VARCHAR(20)   NOT NULL,
    text              TEXT,
    stt_confidence    DOUBLE PRECISION DEFAULT 0,
    llm_emotion       VARCHAR(50),
    tool_name         VARCHAR(100),
    audio_ms          INTEGER       DEFAULT 0,
    started_dt        TIMESTAMPTZ,
    ended_dt          TIMESTAMPTZ,
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_voice_turn_session    ON ac_voice_turn (session_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_turn_turn_id    ON ac_voice_turn (turn_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_turn_dept_id    ON ac_voice_turn (dept_id);
CREATE INDEX IF NOT EXISTS idx_ac_voice_turn_deleted_at ON ac_voice_turn (deleted_at);

COMMENT ON TABLE  ac_voice_turn          IS '语音对话逐轮记录（不存原始音频）';
COMMENT ON COLUMN ac_voice_turn.role     IS '角色：user/assistant/system/tool';
COMMENT ON COLUMN ac_voice_turn.text     IS 'STT/LLM文本';
