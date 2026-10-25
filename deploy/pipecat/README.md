# Pipecat 语音 Agent 控制面

> 所属模块：v0.2.0 设计文档 §8 / §H+（Pipecat 动态配置、控制面、默认零密钥方案）。
> 对应后端：`backend/internal/service/setting/voice_agent_service.go`、
> `backend/internal/service/gb28181/voice_service.go`。
> 对应前端：`frontend/src/admin/pages/gb/VoiceAgentConfig.tsx`。

Pipecat 是**库**不是服务。本目录在 Docker 里包一层薄 FastAPI 控制面，
除了配置下发与会话登记，还负责**真实的 RTP 音频桥**：

- 上行：ZLM `startSendRtp` → 本服务 UDP 端口 → 去 RTP 头 → PCMA → PCM16 → STT；
- 下行：TTS PCM16 → aiortc（强制 PCMA 编码）WebRTC push → ZLM 流 → `startSendRtp` → 设备喇叭。

---

## 1. 当前能力与边界（先看这里）

| 能力 | 状态 |
|:--|:--|
| 动态配置下发（`POST /config`） | ✅ 内存原子替换，下一个会话生效 |
| 配置读取 / 脱敏 | ✅ 敏感键返回 `****`，脱敏串不回写 |
| provider 字典 | ✅ STT 4 / LLM 3 / TTS 4 / VAD 2 |
| provider 依赖探测（`POST /test`） | ✅ 只做本地导入 + DNS，不发计费请求 |
| pipeline 装配（VAD→STT→LLM→TTS） | ✅ 参数映射、必填校验、真实 Pipeline 启动 |
| 上行 RTP 接收（PCMA → PCM16） | ✅ UDP 监听、序号重排、丢包容忍、半双工静音 |
| 下行 WebRTC push（PCM → PCMA） | ✅ aiortc（缺依赖时降级为 `degraded`） |
| 会话登记 / 停止 / 列表 | ✅ |
| G.711A / μ-law ↔ PCM16 | ✅ 纯 Python，可直接单测 |

`audio_bridge` 现在是**实测状态**，不是配置推断：

| 值 | 含义 |
|:--|:--|
| `not_connected` | 没有绑定 UDP / 会话未启动 |
| `degraded` | 只接通一半：例如收到设备音频但 WebRTC 下行失败，或反过来 |
| `connected` | 上行 UDP 正在收包 **且** 下行 WebRTC 已连接 **且** Pipecat Pipeline 已启动 |

> 只有 `connected` 才允许后端把 `agent_enabled` 落成 `true`。`degraded`
> 会被后端视为「Agent 不可用」，降级为纯对讲 —— 宁可明确告知听不到
> agent，也不假装成功。

---

## 2. 文件

| 文件 | 作用 |
|:--|:--|
| `main.py` | FastAPI 控制面：鉴权、配置、会话、测试端点、`/ws/voice`；`/session/start` 真正启动音频桥 |
| `factory.py` | provider 字符串 → Pipecat Service 的映射；可选导入探测 |
| `pipeline.py` | VAD → STT → LLM → TTS 装配、参数映射、`AudioPipelineRunner`；`build_ws_pipeline` 与按句流式 Edge TTS |
| `transport.py` | `RTPAudioReceiver`、WebRTCPCMAPusher、RTP 解析/排序、G.711 ↔ PCM16、重采样 |
| `config_store.py` | 线程安全配置快照，原子替换与脱敏 |
| `ws/protocol.py` | 小智兼容协议子集：hello/listen/abort/stt/llm/tts/mcp/tool/error/wake |
| `ws/auth.py` | `PIPECAT_TOKEN` + `X-Voice-Context` HMAC-SHA256 验签 |
| `ws/serializer.py` | Pipecat `FrameSerializer`：PCM16/Opus 协商、半双工闸门 |
| `ws/session.py` | 单连接会话编排、快速应答、MCP/记忆、看门狗、`/health` 统计 |
| `ws/ack.py` | 「我在」预合成缓存（PCM，跨会话复用） |
| `audio/opus_codec.py` | PyAV/libopus 裸帧编解码（ESP32 上/下行） |
| `voice_mcp/server_client.py` | Go `/internal/mcp` JSON-RPC 客户端 |
| `voice_mcp/device_bridge.py` | 同一 WS 上的设备端 MCP bridge（initialize/tools/list/tools/call） |
| `voice_memory/client.py` | Go 记忆检索/写入 HTTP 客户端 |
| `voice_memory/short_term.py` | 长期记忆注入 system prompt + 会话结束摘要 |
| `internal_http.py` | 标准库 urllib JSON POST（不打印正文/密钥） |
| `tests/` | 协议/鉴权/半双工/MCP/记忆/快速应答单测（89 个） |
| `requirements.txt` | 依赖（`pipecat-ai[anthropic,websocket]==1.13.0`、`aiortc`、`edge-tts` …） |
| `requirements-dev.txt` | 仅测试用：`pytest` / `pytest-asyncio` |
| `Dockerfile` | `python:3.11-slim` + ffmpeg + curl |

---

## 3. 音频链路

### 3.1 上行（设备 → Pipecat）

```text
设备麦克风
  → ZLM 单端口 rtp_proxy:10000（PCMA RTP，SSRC = 平台 INVITE 分配值）
  → ZLM 流 rtp/<SSRC 大写十六进制>
  → 后端调用 ZLM startSendRtp(app=rtp, stream=<设备流>,
                              dst_url=<Pipecat UDP IP>, dst_port=<Pipecat 返回端口>,
                              pt=8, is_udp=1, only_audio=1)
  → RTPAudioReceiver：解析 RTP → SequenceBuffer 排序 → PCMA→PCM16
  → 8k→16k 重采样 → Pipecat STT
```

`RTPAudioReceiver` 的关键行为：

- 按 RTP 序号重排，迟到/重复包丢弃；pending 超过容量时强制推进并计入丢包；
- 按 160 字节（20ms）切帧，下游拿到固定节奏；
- 队列满时丢最旧帧，绝不无限堆积；
- `expected_ssrc` 由会话参数传入，单端口混流时用于过滤；
- 半双工：TTS 输出期间 `set_uplink_muted()` 丢弃上行，防止自激。

### 3.2 下行（Pipecat → 设备）

选 **方案 A：WebRTC push**。

```text
Pipecat TTS → PCM16（24k）
  → 降采样到 8k
  → aiortc AudioTrack（PCM16，20ms/包；setCodecPreferences 强制 PCMA）
  → POST http://zlm/index/api/webrtc?app=rtp&stream=AI<设备流>&type=push
  → ZLM 创建 rtp/AI<设备流>
  → 后端调用 startSendRtp(app=rtp, stream=AI<设备流>,
                          dst_url/dst_port = 设备 200 OK 的 m=audio,
                          ssrc = 平台 INVITE 分配值, pt=8)
  → 设备喇叭
```

**为什么不用方案 B（Pipecat 直接把 PCMA RTP 发到 ZLM 10000/UDP）**：

- ZLM 的 `startSendRtp` 是「从 ZLM 内部已有流往外发」的模型，它需要先有
  `app/stream`；外部往单端口 proxy 灌 RTP 的路径面向设备侧 PS/RTP 收流，
  裸 PCMA 音频轨不是受支持的发布方式，经常表现为「流一直不存在」且没有明确报错；
- 多端口模式本该用 `openRtpServer` 建流，但本项目明确**禁止**调用
  （单端口模式契约，见 `docs/开发文档/后端开发/00-接口契约.md` §15.5c）。

WebRTC push 是 ZLM 官方提供的外部发布入口，因此下行选 A。aiortc 未安装或
ZLM 拒绝 push 时，控制面不会崩溃：上行照常接收，`audio_bridge=degraded`。

> **编码坑**：aiortc 的轨道帧必须是 PCM16，不是 PCMA 字节；否则设备听到
> 的是噪声。本实现用 `RTCRtpSender.setCodecPreferences([PCMA])` 把协商
> 编码收窄为 PCMA，保证 ZLM 流与后端 `startSendRtp(pt=8)` 的假设一致。

---

## 4. 启动

### 4.1 Docker Compose（推荐）

```bash
# 仓库根目录
# .env 中设置：
#   PIPECAT_TOKEN=<随机串，建议 32 位以上>
docker compose --profile voice up -d --build pipecat
```

`pipecat` 服务只 `expose: 8090`，不映射宿主机端口。

音频桥需要两个方向的网络：

| 方向 | 端口 | 说明 |
|:--|:--|:--|
| backend → Pipecat HTTP | 8090/tcp | 配置/会话控制面 |
| ZLM → Pipecat RTP | **UDP 由 Pipecat 动态分配**（默认 20000–65535 中由内核选） | `startSendRtp` 的目标端口，会话建立时返回给后端 |
| Pipecat → ZLM HTTP | ZLM `http.port` | WebRTC push 的 offer/answer |

> 因为 Pipecat 与 ZLM 在同一个 Docker 自定义网络里，动态 UDP 端口无需
> `expose` 也能互通。**跨宿主机部署**时必须让两者同网，或固定
> `uplink_port` 并把该端口发布出去。

### 4.2 后端配置

后端按以下顺序读取 Pipecat 控制面地址与令牌：

1. `ac_thirdparty_config`（`Category='voice_agent'`）中的
   `pipecat_endpoint` / `pipecat_token`（后台「语音 Agent 配置」保存的值）；
2. 环境变量 `PIPECAT_BASE_URL` / `PIPECAT_TOKEN`（本地联调兜底）。

可选：

| 环境变量 | 作用 |
|:--|:--|
| `PIPECAT_UDP_HOST` | ZLM `startSendRtp` 使用的 Pipecat IP/主机名。未设置时从控制面地址解析 IPv4（容器名 `pipecat` → 容器 IP） |
| `PIPECAT_ZLM_BASE_URL` | 覆盖传给 Pipecat 的 ZLM 地址。`ac_gb_media_server.ip` 是从 backend 视角填的（本地常为 `127.0.0.1:8082`），Pipecat 容器里访问不到；compose 部署可设 `http://zlm:80` 而不改库 |
| `PIPECAT_LOG_LEVEL` | 控制面日志级别，默认 `INFO` |

### 4.3 本地直接运行（调试用）

```bash
cd deploy/pipecat
PIPECAT_TOKEN='dev-token-at-least-16-chars' \
  python -m uvicorn main:app --host 127.0.0.1 --port 8090
```

**`PIPECAT_TOKEN` 未配置时进程直接退出**（fail-fast，没有默认放行模式）。

---

## 5. HTTP 契约

鉴权头二选一：

```http
X-Pipecat-Token: <token>
Authorization: Bearer <token>
```

后端 `Apply` 用的是后者。

| 方法 | 路径 | 鉴权 | 说明 |
|:--|:--|:--:|:--|
| GET | `/health` | 否 | `{status, version, providers, audio_bridge, audio_bridge_sessions}` |
| POST | `/config` | 是 | body 为 `map[string]string`，原子替换 |
| GET | `/config` | 是 | 当前配置（敏感键 `****`） |
| POST | `/session/start` | 是 | 见 5.1 |
| POST | `/session/stop` | 是 | `{session_id}` |
| GET | `/sessions` | 是 | 活跃会话（含每路真实桥状态） |
| POST | `/test` | 是 | `{category}`，可选 `{provider}` |

### 5.1 `POST /session/start`

```json
{
  "session_id": "sess-001",
  "channel_id": "34020000001320000001",
  "mode": "half_duplex",
  "uplink_port": 0,
  "uplink_ssrc": 38200001,
  "downlink_stream": "AI0BEBC201",
  "downlink_ssrc": 38200001,
  "zlm_base_url": "http://zlm:80",
  "zlm_secret": "****",
  "zlm_app": "rtp"
}
```

| 字段 | 说明 |
|:--|:--|
| `uplink_port` | 本服务监听的 UDP 端口；`0` = 系统分配，响应里返回实际值 |
| `uplink_ssrc` | 期望的 RTP SSRC；为空则不过滤（单端口混流时建议传） |
| `downlink_stream` | Pipecat 推给 ZLM 的流名（后端用 `AI<设备流>` 推导） |
| `downlink_ssrc` | 下行 RTP 的 SSRC（设备 INVITE 协商值） |
| `zlm_base_url` / `zlm_secret` | WebRTC push 的目标节点与 API secret |
| `mode` | `half_duplex`（默认）/ `full_duplex`（实验）/ `ptt` |

成功响应（节选）：

```json
{
  "ok": true,
  "session_id": "sess-001",
  "mode": "half_duplex",
  "uplink_port": 41234,
  "downlink_stream": "AI0BEBC201",
  "pipeline": ["vad:silero", "stt:local_whisper", "llm:anthropic", "tts:edge_tts"],
  "audio_bridge": "connected",
  "agent_ready": true,
  "audio_bridge_detail": {
    "audio_bridge": "connected",
    "uplink": "connected",
    "downlink": "connected",
    "agent": "running",
    "transport": {"rtp_packets": 0, "lost": 0, "queue": 0}
  }
}
```

**顺序很重要**：后端必须先拿到 `uplink_port`，再调用 ZLM `startSendRtp`
把设备音频推过来；下行则要等 WebRTC push 的 answer 返回后再调
`startSendRtp`（后端已做 5 次、每次 300ms 的重试）。

> **`audio_bridge` 与 `agent_ready` 的区别**：`/session/start` 返回时设备
> 还没开始推流，`audio_bridge` 通常是 `degraded`（没有收到 RTP 包）。
> 后端是否启用 Agent 看的是 `agent_ready`：UDP 已监听 + 下行 WebRTC 已连接
> + Pipeline 可运行。等后端调完上行 `startSendRtp`、设备开始发流后，
> `/sessions` 与 `/health` 会变成 `connected`。若此处用 `audio_bridge`
> 作为开关，后端就永远不会建立上行，形成死锁。

失败（400）示例：

```json
{"detail": "LLM provider openai 缺少必填参数：llm_api_key"}
{"detail": "STT provider local_whisper 依赖未安装：未安装或不可用：faster_whisper ..."}
```

上行 UDP 绑定失败返回 500；下行失败**不**返回失败，而是
`audio_bridge=degraded` + `detail` 说明原因。

---

## 6. 与后端 `Apply` 的配置契约

后端 `Apply` 把 `ac_thirdparty_config` 中 `Category='voice_agent'` 的全部键
转成 `map[string]string` POST 到 `{pipecat_endpoint}/config`，并**排除**
`pipecat_endpoint` 与 `pipecat_token`（避免把控制面 token 回传）。

控制面接受并使用的键（与 `voice_agent_service.go` 常量逐条对应）：

```
stt_provider | stt_model | stt_endpoint | stt_api_key | stt_language
llm_provider | llm_model | llm_endpoint | llm_api_key | llm_system_prompt |
llm_temperature | llm_max_tokens
tts_provider | tts_voice | tts_endpoint | tts_api_key | tts_speed
vad_provider | vad_params
barge_in | session_idle_timeout | max_session_sec
```

可选扩展键（不写库也能通过 `/config` 透传）：

| 键 | 默认 | 作用 |
|:--|:--|:--|
| `stt_sample_rate` | `16000` | 送入 STT 的采样率；设备 8k 会上采样 |

`vad_params` 是 JSON 字符串，例如
`{"threshold":0.5,"min_silence_ms":300}`；非法 JSON 会让 `/session/start`
返回 400。

### 6.1 默认配置（零外部密钥）

| 类别 | 默认 provider | 依赖 |
|:--|:--|:--|
| STT | `local_whisper` | `faster-whisper`（已列入 requirements，体积较大） |
| LLM | `anthropic` | `anthropic` SDK + `llm_api_key`（**唯一需要密钥的默认项**） |
| TTS | `edge_tts` | `edge-tts`（免费；本目录内置 `EdgeTTSProcessor`） |
| VAD | `silero` | Pipecat 内置 ONNX + `onnxruntime` |

---

## 7. 验证

```bash
# 1) 语法检查（本机无 Python 时用容器）
MSYS_NO_PATHCONV=1 docker run --rm \
  -v "D:/work2026/ai-camera/deploy/pipecat:/app" -w /app python:3.11-slim \
  sh -c "python -m py_compile main.py factory.py pipeline.py transport.py config_store.py"

# 2) 无第三方依赖的传输层自测（G.711 往返、RTP 排序、重采样）
MSYS_NO_PATHCONV=1 docker run --rm \
  -v "D:/work2026/ai-camera/deploy/pipecat:/app" -w /app python:3.11-slim \
  python transport.py

# 3) 构建（会安装 pipecat/aiortc，时间较长）
docker compose --profile voice build pipecat
```

只有 Pipecat 与假 RTP 源、没有真实设备时的联调：

```bash
# 起控制面（不装 provider 也能验证传输层；/session/start 会因缺 provider 400）
docker compose --profile voice up -d pipecat

# 用 ffmpeg 生成一段 PCMA RTP 灌到控制面 UDP 端口，观察 /sessions 的
# transport.rtp_packets / lost / queue 变化：
ffmpeg -re -f lavfi -i sine=frequency=440:sample_rate=8000 \
  -c:a pcm_alaw -payload_type 8 -ssrc 12345 \
  -f rtp rtp://<pipecat-ip>:<uplink_port>
```

健康检查：

```bash
docker compose --profile voice exec pipecat curl -s http://127.0.0.1:8090/health
# → {"status":"ok", ..., "audio_bridge":"connected"|"degraded"|"not_connected"}
```

---

## 8. 常见问题

| 现象 | 原因 / 处理 |
|:--|:--|
| 容器启动即退出 | 未设置 `PIPECAT_TOKEN`（fail-fast）；检查 `.env` |
| 后台「应用到 Pipecat」401 | `.env` 的 `PIPECAT_TOKEN` 与后台「控制面令牌」不一致 |
| 下发超时 / connection refused | 后台控制面地址应为 `http://pipecat:8090`（容器内网），不是 127.0.0.1 |
| `/session/start` 400「依赖未安装」 | 该 provider 的 SDK 没装；见 `requirements.txt` 可选注释 |
| `audio_bridge=degraded`，`downlink=not_connected` | aiortc 未安装、ZLM WebRTC push 被拒、或 `zlm_base_url`/`secret` 不对 |
| `audio_bridge=degraded`，`uplink=degraded` | ZLM 还没把设备音频推过来：检查后端 `startSendRtp` 日志、设备是否真的在发流 |
| 设备听不到声音但 `connected` | 检查设备 200 OK 的 `m=audio` 地址/端口、PT 是否为 8、设备喇叭音量 |
| 对讲自激（agent 听到自己） | 默认半双工；确认 `mode=half_duplex`，不要开 `barge_in` |
| `/test` DNS 失败 | 容器无法解析外网域名；检查 Docker DNS / 代理 |
| 会话列表为空但刚 start | 控制面是**单进程内存态**，容器重启会清空；多副本不共享 |

---

## 9. 未真机验证的部分

本期没有真实 GB28181 设备与真实 provider 密钥，以下只能在联调环境确认：

1. 设备 200 OK 的 `m=audio` 地址/端口与平台分配的 SSRC 是否严格匹配；
2. ZLM WebRTC push 在目标版本上的实际响应格式与 `secret` 要求；
3. `startSendRtp` 与 WebRTC push 流注册之间的竞态窗口（已加 5×200ms 重试）；
4. faster-whisper / Anthropic / Edge TTS 的真实延迟与打断行为；
5. 设备喇叭侧的回声时延，以及 300ms 静音尾巴是否足够。

---

## 10. WebSocket 语音 `/ws/voice`（v0.6.0）

同一个 Pipecat 进程同时支持：

- **RTP 桥**（GB28181，`/session/start`）——行为与 v0.2.0 完全一致；
- **WebSocket 语音**（App/Web/ESP32，`/ws/voice`）——本节。

### 10.1 连接与鉴权

```http
GET /ws/voice
Authorization: Bearer <PIPECAT_TOKEN>
X-Voice-Context: <base64url(payload)>.<base64url(HMAC-SHA256(secret, base64url(payload)))>
```

- `PIPECAT_TOKEN`：与控制面共享 token（环境变量，必填）。
- `X-Voice-Context`：Go 网关签名的上下文，密钥 `PIPECAT_CONTEXT_SECRET`：

  ```json
  {"session_id":"s1","user_id":"u1","device_id":"d1","dept_id":"10",
   "scopes":["voice"],"exp":1760000000}
  ```

- Pipecat **只信任签名内容**；未配置 `PIPECAT_CONTEXT_SECRET` 时，带上下文的
  连接会被拒绝，只允许 token-only（仅本地调试，日志会告警）。
- 兼容 `v1.payload.sig` 前缀格式；默认签名覆盖 base64url(payload) 字符串，
  同时兼容 Go 直接对原始 JSON 字节签名的实现。

### 10.2 协议（小智兼容子集）

客户端首帧 `hello`：

```json
{"type":"hello","version":1,"transport":"websocket",
 "features":{"mcp":true},
 "audio_params":{"format":"pcm16","sample_rate":16000,"channels":1,"frame_duration":20},
 "client":{"kind":"app","client_id":"...","device_id":"..."}}
```

| 方向 | type | 说明 |
|:--|:--|:--|
| C→S | `listen` | `state=detect\|start\|stop`，`mode=auto\|manual`（可省略） |
| C→S | `abort` | 立即停止 LLM/TTS、清空输出队列 |
| C→S | `mcp` | `payload`：JSON-RPC 2.0（设备端 MCP） |
| C→S | `ping` | 服务端回 `pong` |
| S→C | `hello` | 下行格式/采样率（24000）与 capabilities |
| S→C | `wake` / `stt` / `llm` / `tts` / `tool` / `mcp` / `error` | 见 `ws/protocol.py` |

音频：

| 客户端 | 上行 | 下行 | 说明 |
|:--|:--|:--|:--|
| App/Web | PCM16 s16le 16k | PCM16 s16le 24k | 8k 上行也接受并重采样 |
| ESP32 | Opus 裸帧 16k | Opus 裸帧 24k | 每个二进制 WS 消息 = 一个 Opus 包（v1） |

内部统一 PCM16 16k（STT）/ 24k（TTS）。

### 10.3 半双工与快速应答

- `listen/detect` 或 `listen/start` 后立即下发快速应答（默认「我在」，
  由 Go 配置 `fast_ack_text` 覆盖）的 `tts` 文本 + 预合成音频
  （进程级缓存，首次会后台合成；未命中时最多等 `voice_ack_timeout`，
  超时只发文字）。
- TTS 播放期间丢弃上行；静音窗口按**实际写出的音频时长 + 300ms 尾巴**
  延长（`ws/serializer.py::HalfDuplexGate`）。
- `abort`：推进 generation、清空输出编码缓冲、`tts stop`，上行立即可用。
- `barge_in` 默认关闭，协议 capability 会如实返回。

### 10.4 MCP

- **服务端 MCP**：`POST {PIPECAT_BACKEND_URL}/internal/mcp`，
  `Authorization: Bearer <INTERNAL_MCP_TOKEN>`，实现
  `initialize / tools/list（cursor）/ tools/call`；工具名中的 `.` 映射为 `_`。
  `mcp_enabled=false` 时不注册服务端工具（设备端 MCP 仍可用）。
- **设备端 MCP**：管线启动后后台执行
  `initialize → tools/list → tools/call`；工具名含 `.` 时映射为
  `device_...`（`screen.capture` → `device_screen_capture`），调用时还原
  `wire_name`。超时/断线只让该次工具失败，不影响音频。
- 工具结果通过 `params.result_callback(text)` 回给 LLM；同时向客户端发
  `tool` 的 `calling/result/error` 状态。

### 10.5 记忆

- 会话开始：`POST {PIPECAT_BACKEND_URL}/internal/voice/memory/retrieve`
  取 top-k（Go 配置 `memory_top_k`，默认 5），拼进 system prompt 的
  `[长期记忆]` 块；`memory_enabled=false` 时跳过。
  请求体与 Go `MemoryRetrieveReq` 对齐：
  `{"owner_keys":["user:1","dept:10","global"],"query":"","top_k":5}`
  （owner key 来自签名上下文，格式 `user:<id>` / `device:<id>` /
  `dept:<id>` / `global`）。
- 会话内：`LLMContext` + `LLMContextSummarizer`
  （`enable_auto_context_summarization=True`）。
- 会话结束：`POST /internal/voice/memory/write` 写确定性摘要。
  请求体与 Go `MemoryWriteReq` 对齐：
  `{"owner_type":"user","owner_id":1,"kind":"summary","content":"...",
    "importance":3,"source_session_id":"s1"}`，owner 按 用户 > 设备 >
  部门 > global 选择（有登录用户时写 user，纯设备会话写 device）。
  `turns`/`message_count` 作为扩展字段附带。
  不存原始音频。

### 10.6 环境变量

| 变量 | 必填 | 作用 |
|:--|:--:|:--|
| `PIPECAT_TOKEN` | 是 | 控制面/WS 共享 token（未配置进程拒绝启动） |
| `PIPECAT_CONTEXT_SECRET` | 生产必填 | `X-Voice-Context` HMAC 密钥 |
| `INTERNAL_MCP_TOKEN` | 否 | Go 内部 MCP/记忆接口 token |
| `PIPECAT_BACKEND_URL` | 否 | compose 注入的 Go 后端地址，如 `http://backend:8080`（也支持 `GO_INTERNAL_BASE_URL` / `/config` 的 `go_internal_base_url`） |
| `PIPECAT_ACK_TEXT` | 否 | 快速应答文本，默认「我在」 |
| `PIPECAT_ALLOW_QUERY_TOKEN` | 否 | `true` 时允许 `?token=`（仅调试） |
| `PIPECAT_ALLOWED_ORIGINS` | 否 | 逗号分隔的 Origin 白名单 |

Go `Apply` 下发的 v0.6.0 键会被 Pipecat 使用：`fast_ack_text`（快速应答）、
`memory_enabled`、`memory_top_k`、`mcp_enabled`、`barge_in`。
Pipecat 额外可选键：`go_internal_base_url`、`voice_ack_timeout`、
`voice_mcp_timeout`、`ws_max_sessions`、`ws_hello_timeout`、
`ws_idle_timeout`、`ws_max_session_sec`（后两者默认 0 = WS 不因空闲/时长
自动断开；RTP 的 `session_idle_timeout` / `max_session_sec` 语义不变）。
`ws_path` 必须是 `/ws/voice`（Pipecat 路由固定），不一致会打告警。

### 10.7 `/health` 与 `/sessions`

`/health` 新增（旧字段保持不变，`sessions` 仍是 RTP 会话数）：

```json
{"transports":{"rtp":{"sessions":0,"audio_bridge":"not_connected"},
               "ws":{"sessions":1,"total_started":3,"total_closed":2,
                     "input_formats":{"pcm16":3},"listen_modes":{"auto":3}}},
 "sessions_rtp":0,"sessions_ws":1,"sessions_total":1}
```

`/sessions` 的 `sessions`/`count` 仍只含 RTP；WS 会话在 `ws_sessions` /
`ws_count`，并新增 `total_count` 与 `transports`。

### 10.8 测试

```bash
cd deploy/pipecat
pip install -r requirements-dev.txt

# 89 个纯 Python 单测（协议/鉴权/半双工/MCP/记忆/快速应答；Opus 需要 av）
python -m pytest tests -q

# 语法检查（本机无 Python 时用容器）
MSYS_NO_PATHCONV=1 docker run --rm \
  -v "D:/work2026/ai-camera/deploy/pipecat:/app" -w /app python:3.11-slim \
  sh -c "find . -name '*.py' | xargs python -m py_compile"
```

> **包名说明**：设备端 MCP 与记忆包分别叫 `voice_mcp/`、`voice_memory/`，
> 而不是 `mcp/`、`memory/`。生产镜像以后可能安装官方 `mcp` SDK，顶层
> 同名目录会把它遮蔽。
