-- ================================================================
-- 09-国标视频 - DDL（20 张表 + ac_device 扩展）
-- AI Camera 智能监控平台 · v0.2.0（设计稿，尚未实施）
-- 覆盖：国标通道 / 媒体节点 / 上级平台 / 级联通道 / 拉流代理 / 推流
--       / 云端录像 / 录像计划 / 录像计划明细 / 国标报警 / 移动位置
--       / 业务分组 / 信令日志 / 订阅记录 / 广播对讲会话
--       / 语音 provider 字典 / JT1078 终端 / JT1078 通道
--
-- 设计依据：docs/敏捷开发/v0.2.0 设备接入与国标视频/00-设计与工作量评估.md
-- 参考实现：wvp-GB28181-pro v2.7.4（数据库/2.7.4/初始化-mysql-2.7.4.sql，22 张 wvp_ 表）
--
-- 说明：
--   1. 【重要】ac_device 是设备权威数据源。国标 20 位编码写入新增列
--      gb_device_id，**不复用既有的 device_code**（业务编号，如 CAM_001）——
--      两者是不同的 ID 命名空间。SIP 白名单只认 gb_device_id。
--   2. 【重要】GB28181 是「1 设备 + N 通道」，云台与点播均作用于通道。
--      v0.1.0 的 ac_device 无通道概念，故新增 ac_gb_channel。
--      通道随设备级联校验：设备被拒绝时其通道一并失效。
--   3. 区域复用既有 ac_region，**不引入 wvp_common_region**，避免双区域体系。
--      ac_gb_channel.region_id / ac_gb_group 为此设计。
--   4. 语音配置**不建新表** —— 复用 ac_thirdparty_config 加
--      Category='voice_agent'（见 07-告警设置.sql 的该表定义）。
--      仅 provider 字典需要新表，用于驱动前端动态表单。
--   5. ac_gb_sip_log / ac_gb_subscribe / ac_voice_session / ac_voice_provider
--      为本项目新增（WVP 无对应表）；其余表语义对齐 wvp_ 同名表。
--   6. ac_gb_sip_log 为追加写入表，量级最大，需按时间分区或定期归档。
-- ================================================================


-- ================================================================
-- 第 0 节：ac_device 扩展（注册与激活）
-- ================================================================
-- 需求 1「设备注册、激活」与需求 2「未登记设备拒绝接入」的落点。
-- 管理员建档时平台生成 gb_device_id 与 sip_password，register_status 置 'pending'；
-- 设备首次 REGISTER 鉴权通过后置 'activated'。
-- ----------------------------------------------------------------

ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS gb_device_id      VARCHAR(20);
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS sip_password      VARCHAR(100);
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS sip_realm         VARCHAR(50);
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS sip_transport     VARCHAR(10)  DEFAULT 'UDP';
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS stream_mode       VARCHAR(20)  DEFAULT 'UDP';
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS charset           VARCHAR(20)  DEFAULT 'GB2312';
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS register_status   VARCHAR(20)  DEFAULT 'pending';
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS register_dt       TIMESTAMPTZ;
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS sip_expires       TIMESTAMPTZ;
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS media_server_id   VARCHAR(50)  DEFAULT 'auto';
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS channel_count     BIGINT       DEFAULT 0;
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS local_ip          VARCHAR(50);
ALTER TABLE ac_device ADD COLUMN IF NOT EXISTS sdp_ip            VARCHAR(50);

-- gb_device_id 为 NULL 的设备（未配置国标接入）不参与 SIP，故用部分唯一索引
CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_device_gb_device_id
    ON ac_device (gb_device_id) WHERE gb_device_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_ac_device_register_status ON ac_device (register_status);
CREATE INDEX IF NOT EXISTS idx_ac_device_sip_expires     ON ac_device (sip_expires);

COMMENT ON COLUMN ac_device.gb_device_id     IS '国标 20 位编码，SIP From 头携带；唯一（部分索引，NULL 不参与）';
COMMENT ON COLUMN ac_device.sip_password     IS '设备 SIP Digest 密码（敏感，接口返回时脱敏）';
COMMENT ON COLUMN ac_device.sip_realm        IS 'SIP 鉴权域，默认取 gb28181.sip_domain';
COMMENT ON COLUMN ac_device.sip_transport    IS 'SIP 传输：UDP/TCP';
COMMENT ON COLUMN ac_device.stream_mode      IS '媒体传输：UDP/TCP-PASSIVE/TCP-ACTIVE';
COMMENT ON COLUMN ac_device.charset          IS 'XML 报文编码：GB2312/UTF-8';
COMMENT ON COLUMN ac_device.register_status  IS '注册状态：pending/activated/rejected';
COMMENT ON COLUMN ac_device.register_dt      IS '首次注册成功时间';
COMMENT ON COLUMN ac_device.sip_expires      IS 'SIP 注册过期时间';
COMMENT ON COLUMN ac_device.media_server_id  IS '绑定媒体节点，auto 为自动选择';
COMMENT ON COLUMN ac_device.channel_count    IS '通道数（冗余，列表展示用）';
COMMENT ON COLUMN ac_device.local_ip         IS '设备级源 IP 覆盖（多网卡场景）';
COMMENT ON COLUMN ac_device.sdp_ip           IS '设备级 SDP IP 覆盖（NAT 场景）';


-- ================================================================
-- 第 1 节：国标通道与目录
-- ================================================================

-- ----------------------------------------------------------------
-- ac_gb_channel 国标通道（设备下的子设备，云台/点播的作用对象）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_channel (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    -- 归属设备
    device_id         BIGINT        NOT NULL,
    device_code       VARCHAR(100),
    gb_device_id      VARCHAR(20),
    -- 通道身份
    gb_channel_id     VARCHAR(20)   NOT NULL,
    name              VARCHAR(255),
    manufacturer      VARCHAR(100),
    model             VARCHAR(100),
    owner             VARCHAR(100),
    civil_code        VARCHAR(20),
    address           VARCHAR(255),
    parental          BIGINT        DEFAULT 0,
    parent_id         VARCHAR(20),
    -- 能力
    channel_type      BIGINT        DEFAULT 0,
    ptz_type          BIGINT        DEFAULT 0,
    has_audio         BOOLEAN       DEFAULT FALSE,
    status            VARCHAR(20)   DEFAULT 'OFF',
    -- 组织
    region_id         BIGINT,
    group_id          BIGINT,
    -- 流与录像
    stream_id         VARCHAR(100),
    record_plan_id    BIGINT,
    enable_broadcast  BOOLEAN       DEFAULT FALSE,
    -- 设备上报的属性（对应「设备参数」页面；WVP 的 wvp_device_channel 有 85 列，
    -- 此处保留国标 2016 常用项。若后续需完整对齐，按 WVP 逐列补齐）
    block             VARCHAR(20),
    safety_way        VARCHAR(50),
    register_way      VARCHAR(20),
    cert_num          VARCHAR(50),
    certifiable       BIGINT,
    err_code          BIGINT,
    end_time          BIGINT,
    secrecy           BIGINT,
    ip_address        VARCHAR(50),
    port              BIGINT,
    password          VARCHAR(100),
    longitude         DOUBLE PRECISION,
    latitude          DOUBLE PRECISION,
    position_type     BIGINT,
    room_type         BIGINT,
    use_type          BIGINT,
    supply_light_type BIGINT,
    direction_type    BIGINT,
    resolution        VARCHAR(50),
    download_speed    VARCHAR(50),
    sub_count         BIGINT,
    gps_time          TIMESTAMPTZ,
    stream_identification VARCHAR(100),
    map_level         BIGINT,
    -- 用户定制值：WVP 用「gb_* 存设备上报原值、无前缀列存用户定制值」的双列语义。
    -- 本期**不保留**该语义（简化为单列，用户改名直接覆盖），已在设计文档 §11+ 记为「行为不同」。
    -- 若要恢复，需为 name/manufacturer/model/owner/civil_code/address/parental/parent_id
    -- 各补一个 gb_* 列。
    is_custom         BOOLEAN       DEFAULT FALSE,

    -- 目录同步
    sync_dt           TIMESTAMPTZ,
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_gb_channel_gb_channel_id ON ac_gb_channel (gb_channel_id) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_ac_gb_channel_device_id     ON ac_gb_channel (device_id);
CREATE INDEX        IF NOT EXISTS idx_ac_gb_channel_region_id     ON ac_gb_channel (region_id);
CREATE INDEX        IF NOT EXISTS idx_ac_gb_channel_group_id      ON ac_gb_channel (group_id);
CREATE INDEX        IF NOT EXISTS idx_ac_gb_channel_deleted_at    ON ac_gb_channel (deleted_at);
CREATE INDEX        IF NOT EXISTS idx_ac_gb_channel_dept_id       ON ac_gb_channel (dept_id);

COMMENT ON TABLE  ac_gb_channel                 IS '国标通道（设备下的子设备）';
COMMENT ON COLUMN ac_gb_channel.device_id       IS '归属设备 ID（ac_device）';
COMMENT ON COLUMN ac_gb_channel.device_code     IS '设备编号（冗余）';
COMMENT ON COLUMN ac_gb_channel.gb_device_id    IS '归属设备的国标编码（冗余）';
COMMENT ON COLUMN ac_gb_channel.gb_channel_id   IS '通道国标 20 位编码，唯一';
COMMENT ON COLUMN ac_gb_channel.name            IS '通道名称（GB2312 转码后存储）';
COMMENT ON COLUMN ac_gb_channel.manufacturer    IS '厂商';
COMMENT ON COLUMN ac_gb_channel.model           IS '型号';
COMMENT ON COLUMN ac_gb_channel.owner           IS '归属单位';
COMMENT ON COLUMN ac_gb_channel.civil_code      IS '行政区划代码';
COMMENT ON COLUMN ac_gb_channel.address         IS '安装地址';
COMMENT ON COLUMN ac_gb_channel.parental        IS '是否有子设备：0 无 / 1 有';
COMMENT ON COLUMN ac_gb_channel.parent_id       IS '父通道国标编码';
COMMENT ON COLUMN ac_gb_channel.channel_type    IS '通道类型：0 普通 / 1 球机 / 2 半球 / 3 固定枪机 等';
COMMENT ON COLUMN ac_gb_channel.ptz_type        IS '云台类型：0 不支持 / 1 球机 / 2 半球 等';
COMMENT ON COLUMN ac_gb_channel.has_audio       IS '是否支持音频';
COMMENT ON COLUMN ac_gb_channel.status          IS '通道状态：ON/OFF（来自目录与设备状态）';
COMMENT ON COLUMN ac_gb_channel.region_id       IS '所属区域（复用 ac_region）';
COMMENT ON COLUMN ac_gb_channel.group_id        IS '所属业务分组（ac_gb_group）';
COMMENT ON COLUMN ac_gb_channel.stream_id       IS '绑定流 ID';
COMMENT ON COLUMN ac_gb_channel.record_plan_id  IS '绑定录像计划';
COMMENT ON COLUMN ac_gb_channel.enable_broadcast IS '是否允许语音广播';
COMMENT ON COLUMN ac_gb_channel.sync_dt         IS '最近一次目录同步时间';
COMMENT ON COLUMN ac_gb_channel.password        IS '设备上报的通道密码（敏感，接口返回时脱敏）';
COMMENT ON COLUMN ac_gb_channel.dept_id         IS '所属部门（数据权限）';


-- ----------------------------------------------------------------
-- ac_gb_group 业务分组树（国标特有，与区域正交）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_group (
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
    device_id         VARCHAR(20),
    parent_device_id  VARCHAR(20),
    business_group    VARCHAR(20),
    civil_code        VARCHAR(20),
    alias             VARCHAR(255),
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_gb_group_parent_id  ON ac_gb_group (parent_id);
CREATE INDEX IF NOT EXISTS idx_ac_gb_group_deleted_at ON ac_gb_group (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_gb_group_dept_id    ON ac_gb_group (dept_id);

COMMENT ON TABLE  ac_gb_group                  IS '国标业务分组（树形）';
COMMENT ON COLUMN ac_gb_group.name             IS '分组名称';
COMMENT ON COLUMN ac_gb_group.parent_id        IS '父分组 ID';
COMMENT ON COLUMN ac_gb_group.device_id        IS '分组国标编码';
COMMENT ON COLUMN ac_gb_group.parent_device_id IS '父分组国标编码';
COMMENT ON COLUMN ac_gb_group.business_group   IS '业务分组类型编码';
COMMENT ON COLUMN ac_gb_group.alias            IS '别名';
COMMENT ON COLUMN ac_gb_group.dept_id          IS '所属部门（数据权限）';


-- ================================================================
-- 第 2 节：媒体节点与流
-- ================================================================

-- ----------------------------------------------------------------
-- ac_gb_media_server 流媒体节点（ZLMediaKit 实例）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_media_server (
    id                    BIGSERIAL     PRIMARY KEY,
    deleted_at            TIMESTAMPTZ,
    created_by            VARCHAR(50),
    created_dt            TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by            VARCHAR(50),
    updated_dt            TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active                BOOLEAN       DEFAULT TRUE,
    name                  VARCHAR(255)  NOT NULL,
    server_id             VARCHAR(100)  NOT NULL,
    type                  VARCHAR(20)   DEFAULT 'zlm',
    -- 三个地址：用途不同，多网卡/NAT 场景必须分开配置
    ip                    VARCHAR(50),
    hook_ip               VARCHAR(50),
    sdp_ip                VARCHAR(50),
    stream_ip             VARCHAR(50),
    -- 端口
    http_port             BIGINT,
    http_ssl_port         BIGINT,
    rtmp_port             BIGINT,
    rtmp_ssl_port         BIGINT,
    rtsp_port             BIGINT,
    rtsp_ssl_port         BIGINT,
    flv_port              BIGINT,
    flv_ssl_port          BIGINT,
    ws_flv_port           BIGINT,
    ws_flv_ssl_port       BIGINT,
    mp4_port              BIGINT,
    mp4_ssl_port          BIGINT,
    -- 收流
    secret                VARCHAR(100),
    rtp_enable            BOOLEAN       DEFAULT TRUE,
    rtp_port_range        VARCHAR(50)   DEFAULT '30000-30500',
    rtp_proxy_port        BIGINT,
    jtt_proxy_port        BIGINT,
    send_rtp_port_range   VARCHAR(50),
    -- 云端录像的合并/切片/打包下载由独立的 Assist 服务承担（WVP 为独立进程，
    -- 默认端口 18081）。0 表示未启用；此时云端录像只支持单文件直接下载。
    record_assist_port    BIGINT        DEFAULT 0,
    -- 录像
    record_path           VARCHAR(500),
    record_day            BIGINT        DEFAULT 7,
    transcode_suffix      VARCHAR(255),
    -- 健康
    online                BOOLEAN       DEFAULT FALSE,
    last_keepalive_dt     TIMESTAMPTZ,
    hook_alive_interval   BIGINT        DEFAULT 15,
    default_server        BOOLEAN       DEFAULT FALSE,
    auto_config           BOOLEAN       DEFAULT FALSE,
    remark                VARCHAR(500),
    dept_id               BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_gb_media_server_server_id ON ac_gb_media_server (server_id) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_ac_gb_media_server_deleted_at ON ac_gb_media_server (deleted_at);
CREATE INDEX        IF NOT EXISTS idx_ac_gb_media_server_online     ON ac_gb_media_server (online);

COMMENT ON TABLE  ac_gb_media_server                     IS '流媒体节点（ZLMediaKit 实例）';
COMMENT ON COLUMN ac_gb_media_server.server_id           IS '节点唯一标识（ZLM general.mediaServerId）';
COMMENT ON COLUMN ac_gb_media_server.type                IS '节点类型，默认 zlm';
COMMENT ON COLUMN ac_gb_media_server.ip                  IS 'ZLM API 访问地址';
COMMENT ON COLUMN ac_gb_media_server.hook_ip             IS 'ZLM 回调本服务的地址（ZLM 所在网络视角）';
COMMENT ON COLUMN ac_gb_media_server.sdp_ip              IS 'SDP 中通告给设备的 IP（通常为局域网地址）';
COMMENT ON COLUMN ac_gb_media_server.stream_ip           IS '对外播放地址 IP（可能为公网/域名）';
COMMENT ON COLUMN ac_gb_media_server.secret              IS 'ZLM API 密钥（敏感）';
COMMENT ON COLUMN ac_gb_media_server.rtp_enable          IS '是否启用多端口 RTP 收流模式';
COMMENT ON COLUMN ac_gb_media_server.rtp_port_range      IS 'RTP 收流端口段，如 30000-30500';
COMMENT ON COLUMN ac_gb_media_server.send_rtp_port_range IS '语音回推端口段';
COMMENT ON COLUMN ac_gb_media_server.record_path         IS '录像存储路径';
COMMENT ON COLUMN ac_gb_media_server.record_day          IS '录像保留天数';
COMMENT ON COLUMN ac_gb_media_server.online              IS '节点在线状态（来自 on_server_keepalive）';
COMMENT ON COLUMN ac_gb_media_server.last_keepalive_dt   IS '最近一次心跳时间';
COMMENT ON COLUMN ac_gb_media_server.default_server      IS '是否默认节点';
COMMENT ON COLUMN ac_gb_media_server.dept_id             IS '所属部门（数据权限）';


-- ----------------------------------------------------------------
-- ac_gb_stream_proxy 拉流代理（平台主动从外部拉流）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_stream_proxy (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    name              VARCHAR(255)  NOT NULL,
    type              VARCHAR(20)   DEFAULT 'default',
    app               VARCHAR(50)   NOT NULL,
    stream            VARCHAR(100)  NOT NULL,
    src_url           VARCHAR(500)  NOT NULL,
    timeout_ms        BIGINT        DEFAULT 10000,
    ffmpeg_cmd_key    VARCHAR(50),
    rtsp_type         BIGINT        DEFAULT 0,
    media_server_id   VARCHAR(50),
    enable_audio      BOOLEAN       DEFAULT TRUE,
    enable_mp4        BOOLEAN       DEFAULT FALSE,
    enable             BOOLEAN      DEFAULT TRUE,
    enable_disable_none_reader BOOLEAN DEFAULT FALSE,
    enable_remove_none_reader  BOOLEAN DEFAULT FALSE,
    pulling           BOOLEAN       DEFAULT FALSE,
    stream_key        VARCHAR(100),
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_gb_stream_proxy_app_stream ON ac_gb_stream_proxy (app, stream) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_ac_gb_stream_proxy_deleted_at ON ac_gb_stream_proxy (deleted_at);
CREATE INDEX        IF NOT EXISTS idx_ac_gb_stream_proxy_pulling    ON ac_gb_stream_proxy (pulling);

COMMENT ON TABLE  ac_gb_stream_proxy             IS '拉流代理（平台主动从外部拉流）';
COMMENT ON COLUMN ac_gb_stream_proxy.app         IS 'ZLM app 名';
COMMENT ON COLUMN ac_gb_stream_proxy.stream      IS 'ZLM stream 名（app+stream 唯一）';
COMMENT ON COLUMN ac_gb_stream_proxy.src_url     IS '源地址（rtsp/rtmp/http 等）';
COMMENT ON COLUMN ac_gb_stream_proxy.timeout_ms  IS '拉流超时（毫秒）';
COMMENT ON COLUMN ac_gb_stream_proxy.ffmpeg_cmd_key IS 'ffmpeg 命令模板键';
COMMENT ON COLUMN ac_gb_stream_proxy.rtsp_type   IS 'RTSP 拉流方式：0 TCP / 1 UDP';
COMMENT ON COLUMN ac_gb_stream_proxy.enable_audio IS '是否保留音频';
COMMENT ON COLUMN ac_gb_stream_proxy.enable_mp4  IS '是否录制 MP4';
COMMENT ON COLUMN ac_gb_stream_proxy.pulling     IS '是否正在拉流';
COMMENT ON COLUMN ac_gb_stream_proxy.dept_id     IS '所属部门（数据权限）';


-- ----------------------------------------------------------------
-- ac_gb_stream_push 推流（国标流推送到 ZLM 之外）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_stream_push (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    name              VARCHAR(255)  NOT NULL,
    app               VARCHAR(50)   NOT NULL,
    stream            VARCHAR(100)  NOT NULL,
    media_server_id   VARCHAR(50),
    push_time         TIMESTAMPTZ,
    status            VARCHAR(20)   DEFAULT 'OFF',
    pushing           BOOLEAN       DEFAULT FALSE,
    self_push         BOOLEAN       DEFAULT FALSE,
    start_offline_push BOOLEAN      DEFAULT FALSE,
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_gb_stream_push_app_stream ON ac_gb_stream_push (app, stream) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_ac_gb_stream_push_deleted_at ON ac_gb_stream_push (deleted_at);

COMMENT ON TABLE  ac_gb_stream_push        IS '推流记录（推送到 ZLM 之外的平台）';
COMMENT ON COLUMN ac_gb_stream_push.app    IS 'ZLM app 名';
COMMENT ON COLUMN ac_gb_stream_push.stream IS 'ZLM stream 名';
COMMENT ON COLUMN ac_gb_stream_push.status IS '推流状态：ON/OFF';
COMMENT ON COLUMN ac_gb_stream_push.pushing IS '是否正在推流';
COMMENT ON COLUMN ac_gb_stream_push.self_push IS '是否本平台自身推流';
COMMENT ON COLUMN ac_gb_stream_push.dept_id IS '所属部门（数据权限）';


-- ================================================================
-- 第 3 节：录像
-- ================================================================

-- ----------------------------------------------------------------
-- ac_gb_cloud_record 云端录像（ZLM 落盘的 MP4）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_cloud_record (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    app               VARCHAR(50),
    stream            VARCHAR(100),
    call_id           VARCHAR(255),
    device_id         BIGINT,
    channel_id        BIGINT,
    start_time        TIMESTAMPTZ,
    end_time          TIMESTAMPTZ,
    media_server_id   VARCHAR(50),
    file_name         VARCHAR(255),
    folder            VARCHAR(500),
    file_path         VARCHAR(1000),
    file_size         BIGINT,
    time_len          BIGINT,
    collect           BOOLEAN       DEFAULT FALSE,
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_gb_cloud_record_time       ON ac_gb_cloud_record (start_time);
CREATE INDEX IF NOT EXISTS idx_ac_gb_cloud_record_app_stream ON ac_gb_cloud_record (app, stream);
CREATE INDEX IF NOT EXISTS idx_ac_gb_cloud_record_deleted_at ON ac_gb_cloud_record (deleted_at);

COMMENT ON TABLE  ac_gb_cloud_record           IS '云端录像（ZLM 落盘 MP4 的索引）';
COMMENT ON COLUMN ac_gb_cloud_record.app       IS 'ZLM app 名';
COMMENT ON COLUMN ac_gb_cloud_record.stream    IS 'ZLM stream 名';
COMMENT ON COLUMN ac_gb_cloud_record.call_id   IS '会话 Call-ID';
COMMENT ON COLUMN ac_gb_cloud_record.start_time IS '录像开始时间';
COMMENT ON COLUMN ac_gb_cloud_record.end_time  IS '录像结束时间';
COMMENT ON COLUMN ac_gb_cloud_record.file_path IS '文件路径';
COMMENT ON COLUMN ac_gb_cloud_record.file_size IS '文件大小（字节）';
COMMENT ON COLUMN ac_gb_cloud_record.time_len  IS '时长（秒）';
COMMENT ON COLUMN ac_gb_cloud_record.collect   IS '是否已收藏（收藏的不自动清理）';
COMMENT ON COLUMN ac_gb_cloud_record.dept_id   IS '所属部门（数据权限）';


-- ----------------------------------------------------------------
-- ac_gb_record_plan 录像计划（周计划）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_record_plan (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    name              VARCHAR(255)  NOT NULL,
    snap              BOOLEAN       DEFAULT FALSE,
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_gb_record_plan_deleted_at ON ac_gb_record_plan (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_gb_record_plan_dept_id    ON ac_gb_record_plan (dept_id);

COMMENT ON TABLE  ac_gb_record_plan      IS '录像计划（周计划）';
COMMENT ON COLUMN ac_gb_record_plan.name IS '计划名称';
COMMENT ON COLUMN ac_gb_record_plan.snap IS '计划期间是否同时抓图';
COMMENT ON COLUMN ac_gb_record_plan.dept_id IS '所属部门（数据权限）';


-- ----------------------------------------------------------------
-- ac_gb_record_plan_item 录像计划明细（一周内的时间段）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_record_plan_item (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    plan_id           BIGINT        NOT NULL,
    week_day          BIGINT        NOT NULL,
    start_time        VARCHAR(20)   NOT NULL,
    end_time          VARCHAR(20)   NOT NULL,
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_gb_record_plan_item_plan_id    ON ac_gb_record_plan_item (plan_id);
CREATE INDEX IF NOT EXISTS idx_ac_gb_record_plan_item_deleted_at ON ac_gb_record_plan_item (deleted_at);

COMMENT ON TABLE  ac_gb_record_plan_item            IS '录像计划明细';
COMMENT ON COLUMN ac_gb_record_plan_item.plan_id    IS '所属计划';
COMMENT ON COLUMN ac_gb_record_plan_item.week_day   IS '星期：0 周日 ~ 6 周六';
COMMENT ON COLUMN ac_gb_record_plan_item.start_time IS '开始时间 HH:mm:ss';
COMMENT ON COLUMN ac_gb_record_plan_item.end_time   IS '结束时间 HH:mm:ss';


-- ================================================================
-- 第 4 节：报警、位置、订阅
-- ================================================================

-- ----------------------------------------------------------------
-- ac_gb_alarm 国标报警（设备主动上报，与平台侧 ac_alarm_record 区分）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_alarm (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    device_id         BIGINT,
    channel_id        BIGINT,
    gb_device_id      VARCHAR(20),
    gb_channel_id     VARCHAR(20),
    alarm_priority    VARCHAR(20),
    alarm_method      BIGINT,
    alarm_type        VARCHAR(50),
    alarm_time        TIMESTAMPTZ,
    alarm_description VARCHAR(500),
    longitude         DOUBLE PRECISION,
    latitude          DOUBLE PRECISION,
    snap_path         VARCHAR(500),
    record_path       VARCHAR(500),
    handled           BOOLEAN       DEFAULT FALSE,
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_gb_alarm_alarm_time  ON ac_gb_alarm (alarm_time);
CREATE INDEX IF NOT EXISTS idx_ac_gb_alarm_channel_id  ON ac_gb_alarm (channel_id);
CREATE INDEX IF NOT EXISTS idx_ac_gb_alarm_deleted_at  ON ac_gb_alarm (deleted_at);
CREATE INDEX IF NOT EXISTS idx_ac_gb_alarm_dept_id     ON ac_gb_alarm (dept_id);

COMMENT ON TABLE  ac_gb_alarm                   IS '国标报警（设备主动上报）';
COMMENT ON COLUMN ac_gb_alarm.device_id         IS '归属设备';
COMMENT ON COLUMN ac_gb_alarm.channel_id        IS '归属通道';
COMMENT ON COLUMN ac_gb_alarm.gb_device_id      IS '设备国标编码（冗余）';
COMMENT ON COLUMN ac_gb_alarm.gb_channel_id     IS '通道国标编码（冗余）';
COMMENT ON COLUMN ac_gb_alarm.alarm_priority    IS '报警级别：1 一级(特急)/2 二级(紧急)/3 三级(一般)/4 四级(其他)';
COMMENT ON COLUMN ac_gb_alarm.alarm_method      IS '报警方式：1 电话/2 短信/3 网络/4 设备';
COMMENT ON COLUMN ac_gb_alarm.alarm_type        IS '报警类型编码';
COMMENT ON COLUMN ac_gb_alarm.alarm_time        IS '报警时间';
COMMENT ON COLUMN ac_gb_alarm.alarm_description IS '报警描述';
COMMENT ON COLUMN ac_gb_alarm.snap_path         IS '关联抓图';
COMMENT ON COLUMN ac_gb_alarm.record_path       IS '关联录像';
COMMENT ON COLUMN ac_gb_alarm.handled           IS '是否已处理';
COMMENT ON COLUMN ac_gb_alarm.dept_id           IS '所属部门（数据权限）';


-- ----------------------------------------------------------------
-- ac_gb_mobile_position 移动位置（移动设备/车载上报，支撑 GIS 轨迹）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_mobile_position (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    device_id         BIGINT,
    channel_id        BIGINT,
    gb_channel_id     VARCHAR(20),
    report_time       TIMESTAMPTZ,
    longitude         DOUBLE PRECISION,
    latitude          DOUBLE PRECISION,
    altitude          DOUBLE PRECISION,
    speed             DOUBLE PRECISION,
    direction         DOUBLE PRECISION,
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_gb_mobile_position_time       ON ac_gb_mobile_position (report_time);
CREATE INDEX IF NOT EXISTS idx_ac_gb_mobile_position_channel_id ON ac_gb_mobile_position (channel_id);
CREATE INDEX IF NOT EXISTS idx_ac_gb_mobile_position_deleted_at ON ac_gb_mobile_position (deleted_at);

COMMENT ON TABLE  ac_gb_mobile_position              IS '移动位置（GIS 轨迹数据源）';
COMMENT ON COLUMN ac_gb_mobile_position.channel_id   IS '归属通道';
COMMENT ON COLUMN ac_gb_mobile_position.gb_channel_id IS '通道国标编码（冗余）';
COMMENT ON COLUMN ac_gb_mobile_position.report_time  IS '上报时间';
COMMENT ON COLUMN ac_gb_mobile_position.longitude    IS '经度';
COMMENT ON COLUMN ac_gb_mobile_position.latitude     IS '纬度';
COMMENT ON COLUMN ac_gb_mobile_position.altitude     IS '海拔（米）';
COMMENT ON COLUMN ac_gb_mobile_position.speed        IS '速度（km/h）';
COMMENT ON COLUMN ac_gb_mobile_position.direction    IS '方向（度）';


-- ----------------------------------------------------------------
-- ac_gb_subscribe 订阅记录（目录/报警/位置订阅，RFC 3265）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_subscribe (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    device_id         BIGINT,
    gb_device_id      VARCHAR(20),
    type              VARCHAR(20)   NOT NULL,
    call_id           VARCHAR(255),
    expires           BIGINT,
    subscribe_dt      TIMESTAMPTZ,
    expire_dt         TIMESTAMPTZ,
    status            VARCHAR(20)   DEFAULT 'active',
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_gb_subscribe_gb_device_id ON ac_gb_subscribe (gb_device_id);
CREATE INDEX IF NOT EXISTS idx_ac_gb_subscribe_expire_dt    ON ac_gb_subscribe (expire_dt);
CREATE INDEX IF NOT EXISTS idx_ac_gb_subscribe_deleted_at   ON ac_gb_subscribe (deleted_at);

COMMENT ON TABLE  ac_gb_subscribe            IS '国标订阅记录（目录/报警/位置）';
COMMENT ON COLUMN ac_gb_subscribe.type       IS '订阅类型：catalog/alarm/mobile_position';
COMMENT ON COLUMN ac_gb_subscribe.call_id    IS '订阅会话 Call-ID';
COMMENT ON COLUMN ac_gb_subscribe.expires    IS '订阅有效期（秒）';
COMMENT ON COLUMN ac_gb_subscribe.subscribe_dt IS '订阅时间';
COMMENT ON COLUMN ac_gb_subscribe.expire_dt  IS '过期时间（到期需续订）';
COMMENT ON COLUMN ac_gb_subscribe.status     IS '状态：active/expired/failed';


-- ----------------------------------------------------------------
-- ac_gb_sip_log 信令日志（排障用；写入量最大，需归档策略）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_sip_log (
    id                BIGSERIAL     PRIMARY KEY,
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    gb_device_id      VARCHAR(20),
    gb_channel_id     VARCHAR(20),
    direction         VARCHAR(10)   NOT NULL,
    method            VARCHAR(20),
    cmd_type          VARCHAR(30),
    status_code       BIGINT,
    remote_addr       VARCHAR(100),
    local_addr        VARCHAR(100),
    call_id           VARCHAR(255),
    content           TEXT,
    result            VARCHAR(20),
    reason            VARCHAR(500),
    elapsed_ms        BIGINT
);

CREATE INDEX IF NOT EXISTS idx_ac_gb_sip_log_created_dt   ON ac_gb_sip_log (created_dt);
CREATE INDEX IF NOT EXISTS idx_ac_gb_sip_log_gb_device_id ON ac_gb_sip_log (gb_device_id);
CREATE INDEX IF NOT EXISTS idx_ac_gb_sip_log_result       ON ac_gb_sip_log (result);

COMMENT ON TABLE  ac_gb_sip_log               IS '国标信令日志（排障用，追加写入）';
COMMENT ON COLUMN ac_gb_sip_log.gb_device_id  IS '设备国标编码';
COMMENT ON COLUMN ac_gb_sip_log.direction     IS '方向：in 接收 / out 发送';
COMMENT ON COLUMN ac_gb_sip_log.method        IS 'SIP 方法：REGISTER/MESSAGE/INVITE/ACK/BYE/INFO';
COMMENT ON COLUMN ac_gb_sip_log.cmd_type      IS 'MANSCDP 命令类型：Catalog/Keepalive/Alarm 等';
COMMENT ON COLUMN ac_gb_sip_log.status_code   IS 'SIP 响应码：200/401/403 等';
COMMENT ON COLUMN ac_gb_sip_log.remote_addr   IS '远端地址（设备）';
COMMENT ON COLUMN ac_gb_sip_log.local_addr    IS '本地地址';
COMMENT ON COLUMN ac_gb_sip_log.call_id       IS 'Call-ID';
COMMENT ON COLUMN ac_gb_sip_log.content       IS '报文内容（截断存储）';
COMMENT ON COLUMN ac_gb_sip_log.result        IS '处理结果：success/failed';
COMMENT ON COLUMN ac_gb_sip_log.reason        IS '拒绝原因等说明';
COMMENT ON COLUMN ac_gb_sip_log.elapsed_ms    IS '处理耗时（毫秒）';


-- ================================================================
-- 第 5 节：语音（广播 / 对讲 / provider 字典）
-- ================================================================

-- ----------------------------------------------------------------
-- ac_voice_session 语音广播/对讲/语音 Agent 会话
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_voice_session (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    session_id        VARCHAR(100)  NOT NULL,
    type              VARCHAR(20)   NOT NULL,
    device_id         BIGINT,
    channel_id        BIGINT,
    gb_channel_id     VARCHAR(20),
    user_id           BIGINT,
    media_server_id   VARCHAR(50),
    app               VARCHAR(50),
    stream            VARCHAR(100),
    ssrc              VARCHAR(20),
    audio_codec       VARCHAR(20),
    agent_enabled     BOOLEAN       DEFAULT FALSE,
    start_dt          TIMESTAMPTZ,
    end_dt            TIMESTAMPTZ,
    status            VARCHAR(20)   DEFAULT 'active',
    end_reason        VARCHAR(500),
    dept_id           BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_voice_session_session_id ON ac_voice_session (session_id) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_ac_voice_session_channel_id ON ac_voice_session (channel_id);
CREATE INDEX        IF NOT EXISTS idx_ac_voice_session_status     ON ac_voice_session (status);
CREATE INDEX        IF NOT EXISTS idx_ac_voice_session_deleted_at ON ac_voice_session (deleted_at);

COMMENT ON TABLE  ac_voice_session              IS '语音广播/对讲/语音 Agent 会话';
COMMENT ON COLUMN ac_voice_session.session_id   IS '会话唯一标识';
COMMENT ON COLUMN ac_voice_session.type         IS '会话类型：broadcast 广播 / talk 对讲 / agent 语音Agent';
COMMENT ON COLUMN ac_voice_session.gb_channel_id IS '通道国标编码';
COMMENT ON COLUMN ac_voice_session.user_id      IS '发起用户（广播/对讲）';
COMMENT ON COLUMN ac_voice_session.ssrc         IS '回推音频的 SSRC';
COMMENT ON COLUMN ac_voice_session.audio_codec  IS '音频编码：G.711A/PCM/Opus';
COMMENT ON COLUMN ac_voice_session.agent_enabled IS '是否接入语音 Agent（Pipecat）';
COMMENT ON COLUMN ac_voice_session.status       IS '状态：active/ended/failed';
COMMENT ON COLUMN ac_voice_session.end_reason   IS '结束原因';


-- ----------------------------------------------------------------
-- ac_voice_provider 语音 provider 字典（驱动前端动态表单）
-- ----------------------------------------------------------------
-- STT/LLM/TTS 的具体参数值存 ac_thirdparty_config（Category='voice_agent'），
-- 本表只存「有哪些 provider 可选、各自需要哪些参数」，
-- 前端据此渲染下拉与动态表单字段（param_schema）。
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_voice_provider (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    sequence          BIGINT        DEFAULT 10,
    category          VARCHAR(20)   NOT NULL,
    code              VARCHAR(50)   NOT NULL,
    name              VARCHAR(255)  NOT NULL,
    param_schema      TEXT,
    default_params    TEXT,
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_voice_provider_category_code ON ac_voice_provider (category, code) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_ac_voice_provider_deleted_at    ON ac_voice_provider (deleted_at);

COMMENT ON TABLE  ac_voice_provider              IS '语音 provider 字典（stt/llm/tts/vad）';
COMMENT ON COLUMN ac_voice_provider.category     IS '类别：stt/llm/tts/vad';
COMMENT ON COLUMN ac_voice_provider.code         IS 'provider 编码：local_whisper/openai/deepgram/anthropic/edge_tts 等';
COMMENT ON COLUMN ac_voice_provider.name         IS '展示名';
COMMENT ON COLUMN ac_voice_provider.param_schema IS 'JSON：该 provider 需要哪些参数，驱动前端动态表单渲染';
COMMENT ON COLUMN ac_voice_provider.default_params IS 'JSON：默认参数值';


-- ================================================================
-- 第 6 节：国标级联（上级平台）
-- ================================================================

-- ----------------------------------------------------------------
-- ac_gb_platform 上级平台（本平台作为下级注册到上级）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_platform (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    enable            BOOLEAN       DEFAULT TRUE,
    name              VARCHAR(255)  NOT NULL,
    server_gb_id      VARCHAR(20)   NOT NULL,
    server_gb_domain  VARCHAR(20),
    server_ip         VARCHAR(50),
    server_port       BIGINT,
    device_gb_id      VARCHAR(20),
    device_ip         VARCHAR(50),
    device_port       BIGINT,
    username          VARCHAR(100),
    password          VARCHAR(100),
    expires           BIGINT,
    keep_timeout      BIGINT,
    transport         VARCHAR(10)   DEFAULT 'UDP',
    civil_code        VARCHAR(20),
    status            VARCHAR(20)   DEFAULT 'OFF',
    catalog_group     BIGINT        DEFAULT 0,
    register_way      VARCHAR(20)   DEFAULT 'local',
    as_message_channel BOOLEAN      DEFAULT FALSE,
    -- 上级平台的 XML 编码。结论六（GB2312 转码）在级联出/入向同样适用，
    -- 不存该值则与 UTF-8 上级平台级联必然乱码。默认 GB2312（对齐 WVP PlatformController:118）
    character_set     VARCHAR(50)   DEFAULT 'GB2312',
    ptz               BOOLEAN       DEFAULT TRUE,
    rtcp              BOOLEAN       DEFAULT FALSE,
    server_id         VARCHAR(50),
    manufacturer      VARCHAR(100),
    model             VARCHAR(100),
    address           VARCHAR(255),
    secrecy           BIGINT        DEFAULT 0,
    catalog_with_platform BOOLEAN   DEFAULT TRUE,
    catalog_with_group BIGINT       DEFAULT 0,
    catalog_with_region BIGINT      DEFAULT 0,
    auto_push_channel BOOLEAN       DEFAULT FALSE,
    send_stream_ip    VARCHAR(50),
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_gb_platform_server_gb_id ON ac_gb_platform (server_gb_id) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_ac_gb_platform_status       ON ac_gb_platform (status);
CREATE INDEX        IF NOT EXISTS idx_ac_gb_platform_deleted_at   ON ac_gb_platform (deleted_at);

COMMENT ON TABLE  ac_gb_platform                    IS '上级平台（国标级联）';
COMMENT ON COLUMN ac_gb_platform.server_gb_id       IS '上级平台国标编码，唯一';
COMMENT ON COLUMN ac_gb_platform.server_gb_domain   IS '上级平台域';
COMMENT ON COLUMN ac_gb_platform.device_gb_id       IS '本平台在该上级的国标编码';
COMMENT ON COLUMN ac_gb_platform.username           IS '注册用户名';
COMMENT ON COLUMN ac_gb_platform.password           IS '注册密码（敏感）';
COMMENT ON COLUMN ac_gb_platform.expires            IS '注册有效期（秒）';
COMMENT ON COLUMN ac_gb_platform.keep_timeout       IS '心跳超时（秒）';
COMMENT ON COLUMN ac_gb_platform.status             IS '在线状态：ON/OFF';
COMMENT ON COLUMN ac_gb_platform.register_way       IS '注册方式：local 本地 / remote 远程';
COMMENT ON COLUMN ac_gb_platform.auto_push_channel  IS '是否自动推送通道';
COMMENT ON COLUMN ac_gb_platform.send_stream_ip     IS '推流使用的 IP';
COMMENT ON COLUMN ac_gb_platform.dept_id            IS '所属部门（数据权限）';


-- ----------------------------------------------------------------
-- ac_gb_platform_channel 级联通道（本平台推送给上级的通道）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_platform_channel (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    platform_id       BIGINT        NOT NULL,
    channel_id        BIGINT        NOT NULL,
    custom_device_id  VARCHAR(20)   NOT NULL,
    custom_name       VARCHAR(255),
    custom_manufacturer VARCHAR(100),
    custom_model      VARCHAR(100),
    custom_owner      VARCHAR(100),
    custom_civil_code VARCHAR(20),
    custom_address    VARCHAR(255),
    custom_parental   BIGINT        DEFAULT 0,
    custom_parent_id  VARCHAR(20),
    custom_status     VARCHAR(20)   DEFAULT 'OFF',
    custom_ptz_type   BIGINT        DEFAULT 0,
    custom_channel_type BIGINT      DEFAULT 0,
    dept_id           BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_gb_platform_channel_custom_id ON ac_gb_platform_channel (custom_device_id) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_ac_gb_platform_channel_platform  ON ac_gb_platform_channel (platform_id);
CREATE INDEX        IF NOT EXISTS idx_ac_gb_platform_channel_channel   ON ac_gb_platform_channel (channel_id);
CREATE INDEX        IF NOT EXISTS idx_ac_gb_platform_channel_deleted_at ON ac_gb_platform_channel (deleted_at);

COMMENT ON TABLE  ac_gb_platform_channel              IS '级联通道（推送至上级的通道及其定制信息）';
COMMENT ON COLUMN ac_gb_platform_channel.platform_id  IS '上级平台 ID';
COMMENT ON COLUMN ac_gb_platform_channel.channel_id   IS '本地通道 ID';
COMMENT ON COLUMN ac_gb_platform_channel.custom_device_id IS '在该上级使用的国标编码，唯一';
COMMENT ON COLUMN ac_gb_platform_channel.custom_name  IS '定制名称（覆盖本地名称）';


-- ----------------------------------------------------------------
-- ac_gb_platform_group 级联平台 ↔ 业务分组（决定「按分组推送目录」的可见范围）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_platform_group (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    platform_id       BIGINT        NOT NULL,
    group_id          BIGINT        NOT NULL
);

-- 唯一约束与 WVP 的 uk_wvp_platform_group_platform_id_group_id 对齐
CREATE UNIQUE INDEX IF NOT EXISTS uk_ac_gb_platform_group_platform_group
    ON ac_gb_platform_group (platform_id, group_id);
CREATE INDEX IF NOT EXISTS idx_ac_gb_platform_group_deleted_at ON ac_gb_platform_group (deleted_at);

COMMENT ON TABLE  ac_gb_platform_group             IS '级联平台与业务分组的可见范围映射';
COMMENT ON COLUMN ac_gb_platform_group.platform_id IS '上级平台 ID（ac_gb_platform）';
COMMENT ON COLUMN ac_gb_platform_group.group_id    IS '业务分组 ID（ac_gb_group）';


-- ----------------------------------------------------------------
-- ac_gb_platform_region 级联平台 ↔ 区域（决定「按区域推送目录」的可见范围）
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_gb_platform_region (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    platform_id       BIGINT        NOT NULL,
    region_id         BIGINT        NOT NULL
);

-- 唯一约束与 WVP 的 uk_wvp_platform_region_platform_id_group_id 对齐
CREATE UNIQUE INDEX IF NOT EXISTS uk_ac_gb_platform_region_platform_region
    ON ac_gb_platform_region (platform_id, region_id);
CREATE INDEX IF NOT EXISTS idx_ac_gb_platform_region_deleted_at ON ac_gb_platform_region (deleted_at);

COMMENT ON TABLE  ac_gb_platform_region             IS '级联平台与区域的可见范围映射';
COMMENT ON COLUMN ac_gb_platform_region.platform_id IS '上级平台 ID（ac_gb_platform）';
COMMENT ON COLUMN ac_gb_platform_region.region_id   IS '区域 ID（ac_region）';


-- ================================================================
-- 第 7 节：JT/T 1078 车载终端
-- ================================================================
-- 与 AI 相机场景关联较弱，属「全量平替」范围，建议三期实施。
-- ----------------------------------------------------------------

-- ----------------------------------------------------------------
-- ac_jt_terminal JT/T 1078 车载终端
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_jt_terminal (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    active            BOOLEAN       DEFAULT TRUE,
    phone_number      VARCHAR(50)   NOT NULL,
    terminal_id       VARCHAR(50),
    province_id       BIGINT,
    city_id           BIGINT,
    maker_id          VARCHAR(20),
    terminal_model    VARCHAR(50),
    terminal_sim      VARCHAR(50),
    plate_color       BIGINT,
    plate_no          VARCHAR(50),
    plate_type        VARCHAR(50),
    longitude         DOUBLE PRECISION,
    latitude          DOUBLE PRECISION,
    status            VARCHAR(20)   DEFAULT 'OFF',
    register_time     TIMESTAMPTZ,
    media_server_id   VARCHAR(50),
    sdp_ip            VARCHAR(50),
    remark            VARCHAR(500),
    dept_id           BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_jt_terminal_phone   ON ac_jt_terminal (phone_number) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_ac_jt_terminal_deleted_at ON ac_jt_terminal (deleted_at);

COMMENT ON TABLE  ac_jt_terminal               IS 'JT/T 1078 车载终端';
COMMENT ON COLUMN ac_jt_terminal.phone_number  IS '终端手机号（BCD 编码，唯一）';
COMMENT ON COLUMN ac_jt_terminal.terminal_id   IS '终端 ID';
COMMENT ON COLUMN ac_jt_terminal.province_id   IS '省域 ID';
COMMENT ON COLUMN ac_jt_terminal.city_id       IS '市县域 ID';
COMMENT ON COLUMN ac_jt_terminal.maker_id      IS '制造商 ID';
COMMENT ON COLUMN ac_jt_terminal.plate_color   IS '车牌颜色';
COMMENT ON COLUMN ac_jt_terminal.plate_no      IS '车牌号';
COMMENT ON COLUMN ac_jt_terminal.status        IS '在线状态：ON/OFF';


-- ----------------------------------------------------------------
-- ac_jt_channel JT/T 1078 通道
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ac_jt_channel (
    id                BIGSERIAL     PRIMARY KEY,
    deleted_at        TIMESTAMPTZ,
    created_by        VARCHAR(50),
    created_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    updated_by        VARCHAR(50),
    updated_dt        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    terminal_db_id    BIGINT        NOT NULL,
    channel_id        BIGINT        NOT NULL,
    name              VARCHAR(255),
    has_audio         BOOLEAN       DEFAULT FALSE,
    status            VARCHAR(20)   DEFAULT 'OFF',
    dept_id           BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ac_jt_channel_terminal_ch ON ac_jt_channel (terminal_db_id, channel_id) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_ac_jt_channel_deleted_at  ON ac_jt_channel (deleted_at);

COMMENT ON TABLE  ac_jt_channel               IS 'JT/T 1078 通道';
COMMENT ON COLUMN ac_jt_channel.terminal_db_id IS '所属终端（ac_jt_terminal.id）';
COMMENT ON COLUMN ac_jt_channel.channel_id    IS '通道号（终端内唯一）';
COMMENT ON COLUMN ac_jt_channel.name          IS '通道名称';
COMMENT ON COLUMN ac_jt_channel.has_audio     IS '是否支持音频';

-- ================================================================
-- DDL 结束（20 张新表 + ac_device 扩展 13 列）
--
-- 配套变更（不在本文件，但必须同步执行）：
--   1. docs/开发文档/数据库设计/sql/init/01-初始数据.sql
--      —— 追加新页面的 ac_menu（含 type='button' 权限行）、
--         ac_menu_api、ac_role_menu_rel 授权，
--         以及 ac_voice_provider 字典初始数据。
--   2. internal/model/gb28181.go —— 对应 GORM 模型（AutoMigrate 用）。
--   3. docs/开发文档/数据库设计/01-数据字典与枚举值.md —— 新增枚举。
--   4. docs/开发文档/数据库设计/02-表关系全景图.md —— 新增表关系。
-- ================================================================
