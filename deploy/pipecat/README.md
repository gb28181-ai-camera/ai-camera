# Pipecat 语音 Agent 控制面

> 所属模块：v0.2.0 设计文档 §8 / §H+（Pipecat 动态配置、控制面、默认零密钥方案）。
> 对应后端：`backend/internal/service/setting/voice_agent_service.go`。
> 对应前端：`frontend/src/admin/pages/gb/VoiceAgentConfig.tsx`。

Pipecat 是**库**不是服务。本目录在 Docker 里包一层薄 FastAPI 控制面，负责
接收后台下发的 STT / LLM / TTS / VAD 配置、装配 pipeline、登记会话。

---

## 1. 当前能力与边界（先看这里）

| 能力 | 状态 |
|:--|:--|
| 动态配置下发（`POST /config`） | ✅ 内存原子替换，下一个会话生效 |
| 配置读取 / 脱敏 | ✅ 敏感键返回 `****`，脱敏串不回写 |
| provider 字典 | ✅ STT 4 / LLM 3 / TTS 4 / VAD 2 |
| provider 依赖探测（`POST /test`） | ✅ 只做本地导入 + DNS，不发计费请求 |
| pipeline 装配（VAD→STT→LLM→TTS） | ✅ 参数映射与必填校验 |
| 会话登记 / 停止 / 列表 | ✅ |
| **设备音频接入与回推** | ❌ **未接通**（`audio_bridge: "not_connected"`） |
| G.711A / μ-law ↔ PCM16 转换 | ✅ 纯 Python，可直接单测 |

> ⚠️ **不要把本期当成「已经能对讲」。** `transport.py` 只实现了帧格式转换与
> 接口骨架；RTP 收发（ZLM 拉流、`startSendRtp` 回推）需要与 ZLM 及设备 SDP
> 联调后才能标注可用。`/health` 与 `/session/start` 的响应都显式返回
> `audio_bridge: "not_connected"`。

---

## 2. 文件

| 文件 | 作用 |
|:--|:--|
| `main.py` | FastAPI 控制面：鉴权、配置、会话、测试端点 |
| `factory.py` | provider 字符串 → Pipecat Service 的映射；可选导入探测 |
| `pipeline.py` | VAD → STT → LLM → TTS 装配、参数映射、必填校验 |
| `transport.py` | `AudioTransport` 接口 + ZLM 骨架 + G.711 ↔ PCM16 |
| `config_store.py` | 线程安全配置快照，原子替换与脱敏 |
| `requirements.txt` | 依赖（`pipecat-ai==1.13.0`） |
| `Dockerfile` | `python:3.11-slim` + ffmpeg + curl |

---

## 3. 启动

### 3.1 Docker Compose（推荐）

```bash
# 仓库根目录
# .env 中设置：
#   PIPECAT_TOKEN=<随机串，建议 32 位以上>
docker compose --profile voice up -d --build pipecat

# 后台「语音 Agent 配置」中填写：
#   Pipecat 控制面地址：http://pipecat:8090
#   Pipecat 控制面令牌：与 .env 的 PIPECAT_TOKEN 完全相同
# 然后点「应用到 Pipecat」。
```

`pipecat` 服务只 `expose: 8090`，**不映射宿主机端口**，仅 docker 内网可达。

### 3.2 本地直接运行（调试用）

```bash
cd deploy/pipecat
PIPECAT_TOKEN='dev-token-at-least-16-chars' \
  python -m uvicorn main:app --host 127.0.0.1 --port 8090
```

**`PIPECAT_TOKEN` 未配置时进程直接退出**（fail-fast，没有默认放行模式）。

---

## 4. HTTP 契约

鉴权头二选一：

```http
X-Pipecat-Token: <token>
Authorization: Bearer <token>
```

后端 `Apply` 用的是后者。

| 方法 | 路径 | 鉴权 | 说明 |
|:--|:--|:--:|:--|
| GET | `/health` | 否 | `{status, version, providers:{stt,llm,tts,vad}=数量, audio_bridge}` |
| POST | `/config` | 是 | body 为 `map[string]string`，原子替换 |
| GET | `/config` | 是 | 当前配置（敏感键 `****`） |
| POST | `/session/start` | 是 | `{session_id, channel_id, mode}` |
| POST | `/session/stop` | 是 | `{session_id}` |
| GET | `/sessions` | 是 | 活跃会话列表 |
| POST | `/test` | 是 | `{category}`，可选 `{provider}` |

### 4.1 `POST /config`

请求体就是后端 `Apply` 直接推来的扁平 map（**键名见第 5 节**）。响应示例：

```json
{
  "ok": true,
  "revision": 3,
  "updated_at": 1760000000.0,
  "applied_keys": ["stt_provider", "llm_model", "..."],
  "ignored_masked_keys": ["llm_api_key"],
  "unknown_keys": [],
  "key_count": 21,
  "config": {
    "stt_provider": "local_whisper",
    "llm_api_key": "****",
    "...": "..."
  }
}
```

- 敏感键（`*_api_key`、`pipecat_token` 等）读取一律 `****`；
- 若收到含 `****` 的值，敏感键**忽略更新**（保留原值），与后端
  `saveThirdParty` 的脱敏串跳过语义一致；
- **配置替换不影响进行中的会话**，下一个 `/session/start` 生效。

### 4.2 `POST /session/start`

```json
{"session_id": "sess-001", "channel_id": "34020000001320000001", "mode": "half_duplex"}
```

`mode`：`half_duplex`（默认，设计 §8.1.1 回声消除结论）、`full_duplex`
（实验项，需同时配置 `barge_in=true`）、`ptt`。

成功响应：

```json
{
  "ok": true,
  "session_id": "sess-001",
  "mode": "half_duplex",
  "started_at": "2026-10-21T10:00:00+00:00",
  "config_revision": 3,
  "pipeline": ["vad:silero", "stt:local_whisper", "llm:anthropic", "tts:edge_tts"],
  "stages": [{"stage": "vad", "provider": "silero", "params": {}, "installed": true}],
  "options": {"barge_in": false, "half_duplex": true,
              "session_idle_timeout": 60, "max_session_sec": 600,
              "audio_bridge": "not_connected"},
  "audio_bridge": "not_connected"
}
```

失败（400）示例：

```json
{"detail": "LLM provider openai 缺少必填参数：llm_api_key"}
{"detail": "STT provider local_whisper 依赖未安装：未安装或不可用：faster_whisper: ModuleNotFoundError ..."}
```

### 4.3 `POST /test`

`category` ∈ `stt / llm / tts / vad`。检查内容：

- 本地 provider：Python 依赖是否导入成功；
- 云端 provider：必要密钥是否存在、endpoint（配置值或 provider 默认值）能否 DNS 解析；
- **不发起任何外部计费调用**，也不验证密钥真实有效性。

---

## 5. 与后端 `Apply` 的配置契约

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

`vad_params` 是 JSON 字符串，例如
`{"threshold":0.5,"min_silence_ms":300}`；非法 JSON 会让 `/session/start`
返回 400。

`param_schema` / `extra_params` 未在固定键中的扩展键会被接受并透传
（响应里列在 `unknown_keys`），用于「不改代码加 provider」。

### 5.1 默认配置（零外部密钥）

| 类别 | 默认 provider | 依赖 |
|:--|:--|:--|
| STT | `local_whisper` | `faster-whisper`（已列入 requirements，体积较大） |
| LLM | `anthropic` | `anthropic` SDK + `llm_api_key`（**唯一需要密钥的默认项**） |
| TTS | `edge_tts` | `edge-tts`（免费） |
| VAD | `silero` | Pipecat 内置 ONNX + `onnxruntime` |

> 默认 LLM 仍需要配置 `llm_api_key`；没有密钥时 `/session/start` 会明确返回
> 400「LLM provider anthropic 缺少必填参数：llm_api_key」。

### 5.2 可选依赖缺失时的行为

- 控制面**照常启动**（provider 模块全部按可选导入处理）；
- `GET /health` 正常；
- `POST /test` 返回 `ok=false`，`detail` 写明缺失模块；
- `POST /session/start` 返回 400 中文原因，不会静默回退到其他 provider。

---

## 6. 音频桥接（未完成）

`transport.py` 已实现：

- `pcmu_encode/decode`（G.711 μ-law）
- `pcma_encode/decode`（G.711 A-law）
- `pcm16 ↔ pcma/pcmu` 批量转换
- `AudioTransport` 抽象与 `ZLMTransport` 骨架

完整链路（联调时补齐）：

```
ZLM 收流(PCMA) → 拉音频 → pcma_to_pcm16 → Pipecat(VAD→STT→LLM→TTS)
→ pcm16_to_pcma → ZLM startSendRtp → 设备喇叭
```

需要与 ZLM 联调：设备 SDP 的目标地址/端口/SSRC、`startSendRtp` 生命周期、
20ms 帧节奏与半双工静音上行策略。在这些验证完成前，`ZLMTransport.read_pcm()`
与 `write_pcm()` 会抛 `NotImplementedError`，不会假装发送成功。

---

## 7. 验证

```bash
# 语法检查（本机无 Python 时用容器）
MSYS_NO_PATHCONV=1 docker run --rm \
  -v "D:/work2026/ai-camera/deploy/pipecat:/app" -w /app python:3.11-slim \
  sh -c "python -m py_compile main.py factory.py pipeline.py transport.py config_store.py"

# G.711 往返检查
MSYS_NO_PATHCONV=1 docker run --rm \
  -v "D:/work2026/ai-camera/deploy/pipecat:/app" -w /app python:3.11-slim \
  python -c "import transport; pcm=transport.pcma_to_pcm16(bytes([0xd5,0x55,0xaa])); print(len(pcm), transport.pcm16_to_pcma(pcm).hex())"

# 构建
docker compose --profile voice build pipecat
```

健康检查：

```bash
docker compose --profile voice exec pipecat curl -s http://127.0.0.1:8090/health
```

（或从同网络的其他容器访问 `http://pipecat:8090/health`。）

---

## 8. 常见问题

| 现象 | 原因 / 处理 |
|:--|:--|
| 容器启动即退出 | 未设置 `PIPECAT_TOKEN`（fail-fast）；检查 `.env` |
| 后台「应用到 Pipecat」401 | `.env` 的 `PIPECAT_TOKEN` 与后台「控制面令牌」不一致 |
| 下发超时 / connection refused | 后台控制面地址应为 `http://pipecat:8090`（容器内网），不是 127.0.0.1 |
| `/session/start` 400「依赖未安装」 | 该 provider 的 SDK 没装；见 `requirements.txt` 可选注释 |
| `/test` DNS 失败 | 容器无法解析外网域名；检查 Docker DNS / 代理 |
| 默认 TTS 报 Edge 适配器未就绪 | `edge-tts` 未安装；重新构建镜像 |
| 会话列表为空但刚 start | 控制面是**单进程内存态**，容器重启会清空；多副本不共享 |
