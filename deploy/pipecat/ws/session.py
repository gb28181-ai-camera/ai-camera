"""``/ws/voice`` 单连接会话：协议、半双工、MCP、记忆、Pipecat 管线。

一个 WebSocket 连接 = 一个 ``WebSocketVoiceSession``。会话负责：

1. 读取 ``hello`` 并协商 PCM16/Opus；
2. 回 ``hello``（含 capabilities）；
3. 并行做长期记忆检索、服务端 MCP/设备端 MCP 初始化；
4. 用 ``FastAPIWebsocketTransport`` + ``PipelineWorker`` 跑 VAD→STT→LLM→TTS；
5. 处理 ``listen / abort / mcp / ping``，推送 ``stt / llm / tts / tool``；
6. 断开时写会话摘要。

## 与 RTP 路径的关系

RTP 的 ``/session/start``、``/sessions``、``/config`` 行为完全不变；本模块
只在 ``main.py`` 新增的 ``/ws/voice`` 里被调用。

## 线程/协程模型

Pipecat 的 transport 在自己的任务里读 WS，serializer 把文本回调给本类；
管线事件（STT/LLM/TTS 帧）由 tap 处理器回调。``/health``、``/sessions``
是同步端点，因此会话表用 ``threading.Lock``，会话对象自身只在事件循环里
修改。
"""

from __future__ import annotations

import asyncio
import logging
import os
import threading
import time
import uuid
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any, Dict, Iterable, List, Mapping, Optional, Sequence

import pipeline
from voice_mcp.device_bridge import DeviceMCPBridge, DeviceTool
from voice_mcp.server_client import (
    MCPServerClient,
    MCPTool,
    build_function_schema,
    sanitize_tool_name,
)
from voice_memory.client import MemoryClient, MemoryItem
from voice_memory import short_term
from ws import protocol
from ws.ack import QuickAckCache
from ws.auth import AuthResult, VoiceContext
from ws.serializer import (
    CodecError,
    HalfDuplexGate,
    build_codec,
    build_serializer_class,
)

logger = logging.getLogger("ai-camera-pipecat.ws")

SERVER_VERSION = "0.6.0"
MAX_CONCURRENT_WS = 100
MAX_TOOL_NAME_LEN = 64


# ════════════════════════════════════════════════════════════════
# 快速应答：进程级缓存
# ════════════════════════════════════════════════════════════════


async def _synthesize_ack(text: str, voice: str, speed: float) -> bytes:
    """快速应答合成器（Edge TTS → 24k PCM）。"""

    return await pipeline._synthesize_edge_pcm(text, voice, speed)  # noqa: SLF001


_ACK_CACHE = QuickAckCache(_synthesize_ack)


# ════════════════════════════════════════════════════════════════
# 会话登记（/health、/sessions）
# ════════════════════════════════════════════════════════════════


@dataclass
class WSSessionInfo:
    session_id: str
    user_id: str = ""
    device_id: str = ""
    dept_id: str = ""
    client_kind: str = ""
    client_id: str = ""
    input_format: str = "pcm16"
    output_format: str = "pcm16"
    listen_mode: str = "auto"
    remote: str = ""
    context_verified: bool = False
    started_at: str = field(default_factory=lambda: datetime.now(timezone.utc).isoformat())
    started_monotonic: float = field(default_factory=time.monotonic)
    last_seen: float = field(default_factory=time.monotonic)
    turn_count: int = 0
    audio_in_bytes: int = 0
    audio_out_bytes: int = 0
    pipeline: Tuple[str, ...] = ()
    tools: int = 0
    detail: str = ""

    def touch(self) -> None:
        self.last_seen = time.monotonic()

    def describe(self) -> Dict[str, Any]:
        data: Dict[str, Any] = {
            "session_id": self.session_id,
            "transport": "websocket",
            "user_id": self.user_id,
            "device_id": self.device_id,
            "dept_id": self.dept_id,
            "client_kind": self.client_kind,
            "client_id": self.client_id,
            "input_format": self.input_format,
            "output_format": self.output_format,
            "listen_mode": self.listen_mode,
            "remote": self.remote,
            "context_verified": self.context_verified,
            "started_at": self.started_at,
            "turn_count": self.turn_count,
            "audio_in_bytes": self.audio_in_bytes,
            "audio_out_bytes": self.audio_out_bytes,
            "pipeline": list(self.pipeline),
            "tools": self.tools,
            "idle_sec": round(time.monotonic() - self.last_seen, 1),
        }
        if self.detail:
            data["detail"] = self.detail
        return data


class WSSessionRegistry:
    """线程安全的 WS 会话表与累计计数。"""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._sessions: Dict[str, WSSessionInfo] = {}
        self._total_started = 0
        self._total_closed = 0
        self._auth_failures = 0
        self._protocol_errors = 0
        self._pipeline_errors = 0
        self._audio_in_bytes = 0
        self._audio_out_bytes = 0
        self._formats: Dict[str, int] = {}
        self._modes: Dict[str, int] = {}

    def add(self, info: WSSessionInfo) -> bool:
        with self._lock:
            if info.session_id in self._sessions:
                return False
            self._sessions[info.session_id] = info
            self._total_started += 1
            return True

    def pop(self, session_id: str) -> Optional[WSSessionInfo]:
        with self._lock:
            info = self._sessions.pop(session_id, None)
            if info is not None:
                self._total_closed += 1
            return info

    def list(self) -> List[WSSessionInfo]:
        with self._lock:
            return sorted(
                self._sessions.values(), key=lambda item: item.started_monotonic
            )

    def count(self) -> int:
        with self._lock:
            return len(self._sessions)

    def note_mode(self, mode: str) -> None:
        with self._lock:
            self._modes[mode] = self._modes.get(mode, 0) + 1

    def note_format(self, audio_format: str) -> None:
        with self._lock:
            self._formats[audio_format] = self._formats.get(audio_format, 0) + 1

    def add_audio(self, *, input_bytes: int = 0, output_bytes: int = 0) -> None:
        with self._lock:
            self._audio_in_bytes += max(0, int(input_bytes))
            self._audio_out_bytes += max(0, int(output_bytes))

    def record_auth_failure(self) -> None:
        with self._lock:
            self._auth_failures += 1

    def record_protocol_error(self) -> None:
        with self._lock:
            self._protocol_errors += 1

    def record_pipeline_error(self) -> None:
        with self._lock:
            self._pipeline_errors += 1

    def stats(self) -> Dict[str, Any]:
        with self._lock:
            return {
                "sessions": len(self._sessions),
                "total_started": self._total_started,
                "total_closed": self._total_closed,
                "auth_failures": self._auth_failures,
                "protocol_errors": self._protocol_errors,
                "pipeline_errors": self._pipeline_errors,
                "input_formats": dict(self._formats),
                "listen_modes": dict(self._modes),
                "audio_in_bytes": self._audio_in_bytes,
                "audio_out_bytes": self._audio_out_bytes,
            }

    def describe_all(self) -> List[Dict[str, Any]]:
        return [info.describe() for info in self.list()]


ws_sessions = WSSessionRegistry()


# ════════════════════════════════════════════════════════════════
# 小工具
# ════════════════════════════════════════════════════════════════


def _as_bool(value: Any, default: bool = False) -> bool:
    if isinstance(value, bool):
        return value
    text = str(value or "").strip().lower()
    if text in {"1", "true", "yes", "on"}:
        return True
    if text in {"0", "false", "no", "off"}:
        return False
    return default


def _as_int(value: Any, default: int) -> int:
    try:
        return int(str(value).strip())
    except (TypeError, ValueError):
        return default


def _as_float(value: Any, default: float) -> float:
    try:
        return float(str(value).strip())
    except (TypeError, ValueError):
        return default


def _go_internal_base_url(config: Mapping[str, Any]) -> str:
    return (
        str(config.get("go_internal_base_url") or "").strip()
        # compose 注入的是 PIPECAT_BACKEND_URL（见 .env.example）。
        or os.environ.get("PIPECAT_BACKEND_URL", "").strip()
        or os.environ.get("GO_INTERNAL_BASE_URL", "").strip()
        or os.environ.get("BACKEND_INTERNAL_BASE_URL", "").strip()
    )


def _internal_token() -> str:
    return (
        os.environ.get("INTERNAL_MCP_TOKEN", "").strip()
        or os.environ.get("INTERNAL_VOICE_TOKEN", "").strip()
        or os.environ.get("PIPECAT_INTERNAL_TOKEN", "").strip()
    )


def _unique_name(name: str, used: set) -> str:
    candidate = sanitize_tool_name(name)
    if candidate not in used:
        used.add(candidate)
        return candidate
    index = 2
    root = candidate[: MAX_TOOL_NAME_LEN - 3]
    while f"{root}_{index}" in used:
        index += 1
    candidate = f"{root}_{index}"
    used.add(candidate)
    return candidate


# ════════════════════════════════════════════════════════════════
# 会话
# ════════════════════════════════════════════════════════════════


class WebSocketVoiceSession:
    """一个 ``/ws/voice`` 连接。"""

    def __init__(
        self,
        websocket: Any,
        auth: AuthResult,
        config: Mapping[str, Any],
        *,
        config_revision: int = 0,
        remote: str = "",
    ) -> None:
        self.websocket = websocket
        self.auth = auth
        self.config = dict(config)
        self.config_revision = config_revision
        self.session_id = auth.session_id or f"ws-{uuid.uuid4().hex[:16]}"
        self.context: VoiceContext = auth.context

        self.info = WSSessionInfo(
            session_id=self.session_id,
            user_id=self.context.user_id,
            device_id=self.context.device_id,
            dept_id=self.context.dept_id,
            client_kind=self.context.client_kind,
            remote=remote,
            context_verified=auth.context_verified,
        )

        self._websocket = websocket
        self._raw_send_lock = asyncio.Lock()
        self._transport: Any = None
        self._input: Any = None
        self._output: Any = None
        self._worker: Any = None
        self._runner: Any = None
        self._bundle: Any = None
        self._serializer: Any = None

        self._ready = asyncio.Event()
        # 输出 transport 真正 set_transport_ready 后才会置位；在此之前
        # 直接写 WS（transport 还没有任何音频要发，不存在并发发送问题）。
        self._output_ready = asyncio.Event()
        self._context_lock = asyncio.Lock()
        self._closed = False
        self._shutting_down = False
        self._watchdog_task: Optional[asyncio.Task] = None
        self._device_tools_task: Optional[asyncio.Task] = None
        self._ack_task: Optional[asyncio.Task] = None
        self._cleanup_done = False

        self.input_format = "pcm16"
        self.output_format = "pcm16"
        self.input_rate = protocol.INTERNAL_INPUT_RATE
        self.output_rate = protocol.INTERNAL_OUTPUT_RATE
        self.codec: Any = None

        self.gate = HalfDuplexGate(
            tail_secs=0.3,
            barge_in=_as_bool(self.config.get("barge_in"), False),
        )
        self.turn_id = 0
        self.listen_mode = "auto"
        self._acked_turn = 0
        self._mode_counted = False

        # 配置键与 Go voice_agent_service.go 对齐：fast_ack_text /
        # memory_top_k / memory_enabled / mcp_enabled 都由 Apply 下发。
        self.ack_text = (
            str(
                self.config.get("fast_ack_text")
                or self.config.get("voice_ack_text")
                or ""
            ).strip()
            or os.environ.get("PIPECAT_ACK_TEXT", "").strip()
            or "我在"
        )
        self.ack_timeout = _as_float(self.config.get("voice_ack_timeout"), 1.5)
        self.memory_top_k = _as_int(
            self.config.get("memory_top_k")
            or self.config.get("voice_memory_top_k"),
            5,
        )
        self.memory_feature_enabled = _as_bool(
            self.config.get("memory_enabled"), True
        )
        self.mcp_feature_enabled = _as_bool(self.config.get("mcp_enabled"), True)
        self.mcp_timeout = _as_float(self.config.get("voice_mcp_timeout"), 6.0)

        base_url = _go_internal_base_url(self.config)
        token = _internal_token()
        self.memory = MemoryClient(
            base_url,
            token,
            context_header=auth.context_header,
            session_id=self.session_id,
            timeout=4.0,
        )
        self.server_mcp = MCPServerClient(
            base_url,
            token,
            context_header=auth.context_header,
            session_id=self.session_id,
            timeout=min(8.0, max(2.0, self.mcp_timeout)),
        )
        self.device_bridge = DeviceMCPBridge(
            self._send_device_mcp_payload,
            timeout=self.mcp_timeout,
            client_version=SERVER_VERSION,
        )

        self._server_tools: List[MCPTool] = []
        self._device_tools: List[DeviceTool] = []
        self._tool_schemas: List[Any] = []
        self._used_tool_names: set = set()
        self._memories: List[MemoryItem] = []
        self._hello: Optional[protocol.ClientHello] = None

    # ── 生命周期 ────────────────────────────────────────────────
    async def run(self) -> None:
        max_sessions = _as_int(
            self.config.get("ws_max_sessions"), MAX_CONCURRENT_WS
        )
        if max_sessions > 0 and ws_sessions.count() >= max_sessions:
            await self._close(1013, "ws_capacity")
            return

        if not ws_sessions.add(self.info):
            await self._close(1012, "duplicate_session")
            return

        try:
            hello = await self._receive_hello()
            self._hello = hello
            await self._configure_from_hello(hello)
            await self._send_raw(
                protocol.server_hello(
                    self.session_id,
                    audio_format=self.output_format,
                    output_rate=self.output_rate,
                    capabilities={
                        "mcp": True,
                        "memory": self.memory_feature_enabled and self.memory.enabled,
                        "barge_in": self.gate.barge_in,
                    },
                )
            )
            ws_path = str(self.config.get("ws_path") or "/ws/voice").strip()
            if ws_path.rstrip("/") != "/ws/voice":
                logger.warning(
                    "配置 ws_path=%s 与 Pipecat 固定路由 /ws/voice 不一致；"
                    "请把 Go 侧 ws_path 改回 /ws/voice",
                    ws_path,
                )
            await _ACK_CACHE.prewarm(
                self.ack_text,
                voice=str(self.config.get("tts_voice") or "zh-CN-XiaoxiaoNeural"),
                speed=_as_float(self.config.get("tts_speed"), 1.0),
            )
            logger.info(
                "WS 会话开始 session=%s kind=%s format=%s->%s client=%s",
                self.session_id,
                hello.client_kind or "unknown",
                self.input_format,
                self.output_format,
                hello.client_id or "-",
            )
            await self._run_pipeline()
        except _SessionDisconnected:
            logger.info("WS 客户端在握手前断开 session=%s", self.session_id)
        except protocol.ProtocolError as exc:
            ws_sessions.record_protocol_error()
            await self._safe_send(exc.as_message())
            await self._close(4400, exc.code)
        except CodecError as exc:
            ws_sessions.record_protocol_error()
            await self._safe_send(
                protocol.error_message("unsupported_audio_format", str(exc))
            )
            await self._close(4400, "unsupported_audio_format")
        except pipeline.PipelineConfigError as exc:
            ws_sessions.record_pipeline_error()
            await self._safe_send(
                protocol.error_message("pipeline_unavailable", str(exc), retryable=False)
            )
            await self._close(1011, "pipeline_unavailable")
        except RuntimeError as exc:
            # provider 实例化 / Pipecat API 缺失。消息由装配层保证不含密钥。
            ws_sessions.record_pipeline_error()
            await self._safe_send(
                protocol.error_message(
                    "pipeline_unavailable", str(exc)[:300], retryable=False
                )
            )
            await self._close(1011, "pipeline_unavailable")
        except Exception as exc:  # 装配/运行异常：不让进程崩溃
            ws_sessions.record_pipeline_error()
            logger.warning(
                "WS 会话异常 session=%s：%s: %s",
                self.session_id,
                type(exc).__name__,
                exc,
                exc_info=True,
            )
            await self._safe_send(
                protocol.error_message("internal_error", "语音会话内部错误", retryable=True)
            )
            await self._close(1011, "internal_error")
        finally:
            await self._cleanup()

    async def _receive_hello(self) -> protocol.ClientHello:
        timeout = _as_float(self.config.get("ws_hello_timeout"), 10.0)
        try:
            raw = await asyncio.wait_for(self.websocket.receive_text(), timeout=timeout)
        except (asyncio.TimeoutError, TimeoutError) as exc:
            raise protocol.ProtocolError("hello_timeout", "等待 hello 超时") from exc
        except Exception as exc:
            if "disconnect" in type(exc).__name__.lower():
                raise _SessionDisconnected() from exc
            raise protocol.ProtocolError(
                "hello_required", "首帧必须是 hello JSON 文本帧"
            ) from exc

        message = protocol.parse_message(raw)
        if message.type != "hello" or message.hello is None:
            raise protocol.ProtocolError("hello_required", "首帧必须是 hello")
        return message.hello

    async def _configure_from_hello(self, hello: protocol.ClientHello) -> None:
        self.input_format = hello.audio_params.format
        self.input_rate = hello.audio_params.sample_rate
        # 下行格式跟随上行协商：PCM16 客户端收 PCM16，ESP32 收 Opus。
        self.output_format = self.input_format
        self.output_rate = protocol.INTERNAL_OUTPUT_RATE
        self.codec = build_codec(
            self.input_format, input_rate=self.input_rate, output_rate=self.output_rate
        )
        self.info.input_format = self.input_format
        self.info.output_format = self.output_format
        ws_sessions.note_format(self.input_format)
        if hello.client_kind:
            self.info.client_kind = hello.client_kind
        if hello.client_id:
            self.info.client_id = hello.client_id
        if hello.device_id:
            self.info.device_id = hello.device_id
        self.info.touch()

    async def _run_pipeline(self) -> None:
        transport, serializer = self._build_transport()
        self._transport = transport
        self._serializer = serializer
        self._input = transport.input()
        self._output = transport.output()

        # 不在启动路径上做任何外部 IO：记忆检索 / 服务端 MCP / 设备 MCP
        # 都在管线启动后由 _load_extras_background 并发补齐。这样 Go 内部
        # 接口不可用时也不会拖慢首音频。
        system_prompt = str(self.config.get("llm_system_prompt") or "").strip()
        self._used_tool_names = set()
        tool_schemas: List[Any] = []
        self._tool_schemas = list(tool_schemas)

        bundle = pipeline.build_ws_pipeline(
            config=self.config,
            session_id=self.session_id,
            transport=transport,
            on_frame=self._on_pipeline_frame,
            system_prompt=system_prompt,
            tools=tool_schemas,
            listen_mode=self.listen_mode,
            config_revision=self.config_revision,
            input_rate=protocol.INTERNAL_INPUT_RATE,
            output_rate=self.output_rate,
        )
        self._bundle = bundle
        self._worker = bundle.worker
        self.info.pipeline = tuple(bundle.stages)
        self.info.tools = len(tool_schemas)

        try:
            from pipecat.workers.runner import WorkerRunner  # type: ignore
        except Exception as exc:
            raise RuntimeError(f"当前 pipecat 版本没有 WorkerRunner：{exc}") from exc

        runner = WorkerRunner(handle_sigint=False, handle_sigterm=False)
        self._runner = runner
        await runner.add_workers(bundle.worker)
        logger.info(
            "WS 管线启动 session=%s pipeline=%s tools=%d",
            self.session_id,
            " → ".join(bundle.stages),
            len(tool_schemas),
        )
        await runner.run()

    def _build_transport(self):
        from pipecat.serializers.base_serializer import FrameSerializer  # type: ignore
        from pipecat.transports.websocket.fastapi import (  # type: ignore
            FastAPIWebsocketParams,
            FastAPIWebsocketTransport,
        )

        serializer_cls = build_serializer_class(FrameSerializer, self)
        serializer = serializer_cls()
        params = FastAPIWebsocketParams(
            audio_in_enabled=True,
            audio_out_enabled=True,
            audio_in_sample_rate=protocol.INTERNAL_INPUT_RATE,
            audio_out_sample_rate=self.output_rate,
            audio_in_channels=1,
            audio_out_channels=1,
            # 20ms 一个下行分片，正好是 Opus 的一帧。
            audio_out_10ms_chunks=2,
            add_wav_header=False,
            serializer=serializer,
        )
        transport = FastAPIWebsocketTransport(self.websocket, params)
        transport.event_handler("on_client_connected")(self._on_client_connected)
        transport.event_handler("on_client_disconnected")(self._on_client_disconnected)
        transport.event_handler("on_session_timeout")(self._on_session_timeout)
        return transport, serializer

    # ── transport 事件 ──────────────────────────────────────────
    async def _on_client_connected(self, _transport: Any, _websocket: Any) -> None:
        self._ready.set()
        self.info.touch()
        if self._watchdog_task is None or self._watchdog_task.done():
            self._watchdog_task = asyncio.create_task(self._watchdog())
            self._watchdog_task.add_done_callback(_consume_task_exception)
        # 记忆 + 服务端 MCP + 设备端 MCP 都放到后台：不阻塞首音频。
        if self._device_tools_task is None or self._device_tools_task.done():
            self._device_tools_task = asyncio.create_task(
                self._load_extras_background()
            )
            self._device_tools_task.add_done_callback(_consume_task_exception)

    async def _on_client_disconnected(self, _transport: Any, _websocket: Any) -> None:
        task = asyncio.create_task(self._shutdown("client_disconnected"))
        task.add_done_callback(_consume_task_exception)

    async def _on_session_timeout(self, _transport: Any, _websocket: Any) -> None:
        task = asyncio.create_task(self._shutdown("session_timeout"))
        task.add_done_callback(_consume_task_exception)

    async def _watchdog(self) -> None:
        # WS 默认不强制断开：ESP32 可能长时间待命等唤醒词。需要成本上限
        # 时显式配置 `ws_max_session_sec` / `ws_idle_timeout`；RTP 的
        # `max_session_sec` / `session_idle_timeout` 语义保持不变。
        max_sec = _as_int(self.config.get("ws_max_session_sec"), 0)
        idle_sec = _as_int(self.config.get("ws_idle_timeout"), 0)
        if max_sec <= 0 and idle_sec <= 0:
            return
        while not self._closed:
            await asyncio.sleep(1.0)
            now = time.monotonic()
            if max_sec > 0 and now - self.info.started_monotonic >= max_sec:
                await self._send_control(
                    protocol.error_message("session_timeout", "会话已达最大时长", True)
                )
                await self._shutdown("max_session_sec")
                return
            if idle_sec > 0 and now - self.info.last_seen >= idle_sec:
                await self._send_control(
                    protocol.error_message("idle_timeout", "会话空闲超时", True)
                )
                await self._shutdown("idle_timeout")
                return

    async def _shutdown(self, reason: str) -> None:
        if self._shutting_down or self._cleanup_done:
            return
        self._shutting_down = True
        self._closed = True
        logger.info("WS 会话关闭 session=%s reason=%s", self.session_id, reason)
        runner = self._runner
        if runner is not None:
            try:
                await asyncio.wait_for(runner.cancel(reason=reason), timeout=3.0)
            except Exception:
                pass

    # ── 上下文/工具准备（后台，均不阻塞首音频）──────────────────
    async def _load_extras_background(self) -> None:
        """记忆 / 服务端 MCP / 设备 MCP 并发补齐。"""

        await asyncio.gather(
            self._load_memory_background(),
            self._load_server_tools_background(),
            self._load_device_tools_background(),
            return_exceptions=True,
        )

    async def _load_memory_background(self) -> None:
        if (
            not self.memory_feature_enabled
            or not self.memory.enabled
            or self._bundle is None
        ):
            return
        try:
            memories = await asyncio.wait_for(
                self.memory.retrieve(
                    session_id=self.session_id,
                    # 身份只来自签名上下文；hello.client.device_id 是客户端
                    # 自报值，不能用于记忆归属。
                    user_id=self.context.user_id,
                    device_id=self.context.device_id,
                    dept_id=self.context.dept_id,
                    query="",
                    top_k=self.memory_top_k,
                ),
                timeout=3.0,
            )
        except Exception as exc:
            logger.info("长期记忆检索不可用：%s", type(exc).__name__)
            return
        if not memories:
            return

        self._memories = list(memories)
        prompt = short_term.render_system_prompt(
            str(self.config.get("llm_system_prompt") or ""),
            memories,
            max_memories=max(1, self.memory_top_k),
        )
        async with self._context_lock:
            self._apply_system_prompt(prompt)
        logger.info(
            "长期记忆已注入 session=%s count=%d", self.session_id, len(memories)
        )

    def _apply_system_prompt(self, prompt: str) -> None:
        """把 system prompt 放到消息列表头部，保留已有对话。

        记忆是异步拿到的，可能晚于第一条用户消息；这里只替换/插入
        system 消息，不丢弃上下文。
        """

        if self._bundle is None:
            return
        context = self._bundle.context
        try:
            current = list(getattr(context, "messages", []) or [])
            if current and isinstance(current[0], Mapping) and current[0].get("role") == "system":
                current = current[1:]
            messages = (
                [{"role": "system", "content": prompt}] if prompt.strip() else []
            ) + current
            context.set_messages(messages)
        except Exception:
            logger.info("注入 system prompt 失败", exc_info=True)

    async def _load_server_tools_background(self) -> None:
        if (
            not self.mcp_feature_enabled
            or not self.server_mcp.enabled
            or self._bundle is None
        ):
            return
        try:
            await asyncio.wait_for(self.server_mcp.initialize(), timeout=3.0)
            tools = await asyncio.wait_for(self.server_mcp.list_tools(), timeout=4.0)
        except Exception as exc:
            logger.info("服务端 MCP 不可用：%s", type(exc).__name__)
            return
        if not tools:
            return
        schemas = self._build_server_schemas(tools)
        if not schemas:
            return
        self._server_tools = list(tools)
        await self._apply_tool_schemas(schemas)
        logger.info(
            "服务端 MCP 工具已注册 session=%s count=%d", self.session_id, len(schemas)
        )

    async def _load_device_tools_background(self) -> None:
        """管线启动后增量发现设备工具。

        工具描述与 handler 注册到现有 ``LLMContext``；即使失败/超时，
        当前对话仍可继续（没有设备工具而已）。
        """

        if not self._hello or not self._hello.mcp_enabled:
            return
        if self._worker is None or self._bundle is None:
            return
        try:
            await asyncio.wait_for(
                self.device_bridge.initialize(), timeout=min(3.0, self.mcp_timeout)
            )
            tools = await asyncio.wait_for(
                self.device_bridge.list_tools(), timeout=min(4.0, self.mcp_timeout)
            )
        except Exception as exc:
            logger.info("设备端 MCP 不可用：%s", type(exc).__name__)
            return

        if not tools:
            return
        schemas = self._build_device_schemas(tools)
        if not schemas:
            return
        self._device_tools = list(tools)
        await self._apply_tool_schemas(schemas)
        logger.info(
            "设备端 MCP 工具已注册 session=%s count=%d", self.session_id, len(schemas)
        )

    async def _apply_tool_schemas(self, schemas: Sequence[Any]) -> None:
        if self._bundle is None:
            return
        async with self._context_lock:
            self._tool_schemas.extend(schemas)
            self.info.tools = len(self._tool_schemas)
            try:
                self._bundle.context.set_tools(self._tool_schemas)
            except Exception:
                logger.info("更新 LLMContext tools 失败", exc_info=True)
            llm = getattr(self._bundle, "llm", None)
            for schema in schemas:
                name = getattr(schema, "name", "")
                handler = getattr(schema, "handler", None)
                if not name or handler is None:
                    continue
                try:
                    # FunctionSchema 自带 handler 时，Pipecat 在上下文更新后
                    # 会自动注册；这里显式再注册一次，覆盖运行期增量工具。
                    llm.register_function(name, handler)
                except Exception:
                    logger.info("注册工具 handler 失败 tool=%s", name)

    def _build_server_schemas(self, server_tools: Sequence[MCPTool]) -> List[Any]:
        schemas: List[Any] = []
        for tool in server_tools:
            try:
                llm_name = _unique_name(
                    tool.llm_name or tool.name, self._used_tool_names
                )
                handler = self._server_tool_handler(tool)
                schemas.append(
                    build_function_schema(
                        llm_name, tool.description, tool.input_schema, handler
                    )
                )
            except Exception as exc:
                logger.info("服务端工具 %s 未能注册：%s", tool.name, type(exc).__name__)
        return schemas

    def _build_device_schemas(self, device_tools: Sequence[DeviceTool]) -> List[Any]:
        schemas: List[Any] = []
        for tool in device_tools:
            try:
                llm_name = _unique_name(tool.llm_name, self._used_tool_names)
                handler = self._device_tool_handler(tool)
                schemas.append(
                    build_function_schema(
                        llm_name, tool.description, tool.input_schema, handler
                    )
                )
            except Exception as exc:
                logger.info("设备工具 %s 未能注册：%s", tool.wire_name, type(exc).__name__)
        return schemas

    def _server_tool_handler(self, tool: MCPTool):
        async def handler(params: Any) -> None:
            await self._call_server_tool(tool, params)

        return handler

    def _device_tool_handler(self, tool: DeviceTool):
        async def handler(params: Any) -> None:
            await self._call_device_tool(tool, params)

        return handler

    async def _call_server_tool(self, tool: MCPTool, params: Any) -> None:
        name = tool.name
        text = "工具调用失败"
        await self._send_tool("calling", name=name, display=tool.description or name)
        try:
            result = await self.server_mcp.call_tool(
                name, _arguments(params), timeout=min(8.0, self.mcp_timeout)
            )
            text = result.text or "（工具没有返回内容）"
            if result.is_error:
                await self._send_tool("error", name=name, error=text[:200])
            else:
                await self._send_tool("result", name=name)
        except Exception as exc:
            text = f"工具调用失败：{str(exc)[:200]}"
            await self._send_tool("error", name=name, error=text)
            logger.info("服务端工具调用失败 tool=%s error=%s", name, type(exc).__name__)
        await _safe_result_callback(params, text)

    async def _call_device_tool(self, tool: DeviceTool, params: Any) -> None:
        name = tool.wire_name
        text = "设备工具调用失败"
        await self._send_tool("calling", name=name, display=tool.description or name)
        try:
            result = await self.device_bridge.call_tool(
                tool, _arguments(params), timeout=min(8.0, self.mcp_timeout)
            )
            text = result.text or "（设备没有返回内容）"
            if result.is_error:
                await self._send_tool("error", name=name, error=text[:200])
            else:
                await self._send_tool("result", name=name)
        except Exception as exc:
            text = f"设备工具调用失败：{str(exc)[:200]}"
            await self._send_tool("error", name=name, error=text)
            logger.info("设备工具调用失败 tool=%s error=%s", name, type(exc).__name__)
        await _safe_result_callback(params, text)

    # ── 上行消息 ────────────────────────────────────────────────
    async def handle_text(self, raw: str) -> None:
        """serializer 回调：解析控制消息（永不抛异常）。"""

        try:
            message = protocol.parse_message(raw)
        except protocol.ProtocolError as exc:
            ws_sessions.record_protocol_error()
            await self._send_control(exc.as_message())
            return
        except Exception:
            logger.warning("WS 控制消息处理失败", exc_info=True)
            return

        self.info.touch()
        try:
            if message.type == "hello":
                await self._send_control(
                    protocol.error_message("hello_already_done", "hello 只能发送一次")
                )
            elif message.type == "listen" and message.listen is not None:
                await self._handle_listen(message.listen)
            elif message.type == "abort" and message.abort is not None:
                await self._handle_abort(message.abort)
            elif message.type == "mcp" and message.mcp is not None:
                handled = await self.device_bridge.handle_message(message.mcp.payload)
                if not handled:
                    logger.debug("忽略未匹配的设备 MCP 消息")
            elif message.type == "ping":
                await self._send_control(protocol.pong_message(message.ping_ts))
        except Exception:
            logger.warning("WS 控制消息副作用失败", exc_info=True)

    async def _handle_listen(self, command: protocol.ListenCommand) -> None:
        if command.mode and command.mode != self.listen_mode:
            self.listen_mode = command.mode
            self.info.listen_mode = command.mode
        effective_mode = command.mode or self.listen_mode
        if not self._mode_counted:
            self._mode_counted = True
            ws_sessions.note_mode(effective_mode)

        if command.state in ("detect", "start"):
            was_listening = self.gate.listening
            if not was_listening:
                self.turn_id += 1
            self.gate.begin_listen()
            self.info.turn_count = self.turn_id

            if command.state == "detect":
                await self._send_control(protocol.wake_message(command.text))

            if self._acked_turn != self.turn_id:
                self._acked_turn = self.turn_id
                # 不阻塞 WS 接收循环：合成/缓存未命中时最多 1.5s。
                if self._ack_task is None or self._ack_task.done():
                    self._ack_task = asyncio.create_task(self._send_quick_ack())
                    self._ack_task.add_done_callback(_consume_task_exception)

            if effective_mode == "manual":
                await self._push_user_frame("UserStartedSpeakingFrame")
            return

        if command.state == "stop":
            self.gate.end_listen()
            await self._push_user_frame("UserStoppedSpeakingFrame")

    async def _handle_abort(self, command: protocol.AbortCommand) -> None:
        self.turn_id += 1
        self._acked_turn = 0
        self.gate.abort()
        if self._ack_task is not None and not self._ack_task.done():
            self._ack_task.cancel()
        try:
            self.codec.reset_output()
        except Exception:
            pass

        if self._worker is not None:
            try:
                from pipecat.frames.frames import InterruptionFrame  # type: ignore

                await self._worker.queue_frame(InterruptionFrame())
            except Exception:
                pass

        await self._send_control(
            protocol.tts_message("stop", turn_id=self.turn_id - 1)
        )
        logger.info(
            "WS abort session=%s reason=%s", self.session_id, (command.reason or "-")[:40]
        )

    async def _push_user_frame(self, frame_name: str) -> None:
        if self._worker is None:
            return
        try:
            import pipecat.frames.frames as frames  # type: ignore

            frame_cls = getattr(frames, frame_name)
            await self._worker.queue_frame(frame_cls())
        except Exception:
            logger.debug("无法注入 %s", frame_name, exc_info=True)

    # ── 快速应答 ────────────────────────────────────────────────
    async def _send_quick_ack(self) -> None:
        text = self.ack_text
        turn = self.turn_id
        voice = str(self.config.get("tts_voice") or "zh-CN-XiaoxiaoNeural")
        speed = _as_float(self.config.get("tts_speed"), 1.0)

        await self._send_control(protocol.tts_message("start", turn_id=turn))
        await self._send_control(
            protocol.tts_message("sentence_start", text=text, turn_id=turn)
        )

        # 等输出 transport 就绪一小会儿（通常 <100ms）；超时仍先发文字。
        try:
            await asyncio.wait_for(self._output_ready.wait(), timeout=0.5)
        except (asyncio.TimeoutError, TimeoutError):
            pass

        pcm = await _ACK_CACHE.get(text, voice=voice, speed=speed, timeout=self.ack_timeout)
        if pcm and self._output is not None and self._output_ready.is_set():
            try:
                from pipecat.frames.frames import OutputAudioRawFrame  # type: ignore

                await self._output.queue_frame(
                    OutputAudioRawFrame(
                        audio=pcm,
                        sample_rate=protocol.INTERNAL_OUTPUT_RATE,
                        num_channels=1,
                    )
                )
            except Exception:
                logger.debug("快速应答音频入队失败", exc_info=True)

        # stop 用有序消息：排在音频之后，客户端不会提前结束 TTS 状态。
        await self._send_control(
            protocol.tts_message("stop", turn_id=turn), ordered=True
        )

    # ── Pipecat 帧事件 ──────────────────────────────────────────
    async def _on_pipeline_frame(self, stage: str, frame: Any, _direction: Any) -> None:
        name = type(frame).__name__
        if name == "OutputTransportReadyFrame":
            # 输出 transport 已注册默认 media sender，可以安全入队。
            self._output_ready.set()
            return
        if stage == "stt":
            if name == "InterimTranscriptionFrame":
                await self._send_control(
                    protocol.stt_message(
                        getattr(frame, "text", "") or "",
                        is_final=False,
                        turn_id=self.turn_id,
                    )
                )
            elif name == "TranscriptionFrame":
                confidence = _frame_confidence(frame)
                language = getattr(frame, "language", None)
                if language is not None:
                    language = str(getattr(language, "value", language) or "") or None
                await self._send_control(
                    protocol.stt_message(
                        getattr(frame, "text", "") or "",
                        is_final=True,
                        confidence=confidence,
                        turn_id=self.turn_id,
                        language=language,
                    )
                )
            return

        if stage == "llm":
            if name in ("TextFrame", "LLMTextFrame"):
                if getattr(frame, "skip_tts", False):
                    return
                text = getattr(frame, "text", "") or ""
                if text:
                    await self._send_control(
                        protocol.llm_message(
                            text, emotion=str(getattr(frame, "emotion", "") or "") or None
                        )
                    )
            # 工具状态由 handler 自己发（能拿到 wire name）；这里不再重复
            # 处理 FunctionCallInProgress/ResultFrame，避免客户端收到两份。
            return

        if stage == "tts":
            if name == "TTSStartedFrame":
                # 真实回复开始：立即丢弃上行（TTS start→stop），并取消还在
                # 等待合成的快速应答，避免它的 tts stop 提前打开麦克风门闸。
                self.gate.begin_output()
                if self._ack_task is not None and not self._ack_task.done():
                    self._ack_task.cancel()
                await self._send_control(
                    protocol.tts_message("start", turn_id=self.turn_id)
                )
            elif name == "TTSTextFrame":
                text = getattr(frame, "text", "") or ""
                if text:
                    await self._send_control(
                        protocol.tts_message(
                            "sentence_start", text=text, turn_id=self.turn_id
                        )
                    )
            elif name == "TTSStoppedFrame":
                self.gate.close_output()
                await self._send_control(
                    protocol.tts_message("stop", turn_id=self.turn_id), ordered=True
                )

    # ── 发送 ────────────────────────────────────────────────────
    async def _send_control(self, payload: Mapping[str, Any], *, ordered: bool = False) -> None:
        """发送控制消息。

        - ``ordered=False``：紧急帧，立即写出；
        - ``ordered=True``：走输出音频队列，保证排在已排队的音频之后
          （``tts stop`` 必须等音频播完）。
        """

        if self._closed:
            return
        # 输出 transport 还没 set_transport_ready 时必须直接写 WS：
        # 此时默认 media sender 尚未注册，入队的 message 会被丢弃；设备
        # MCP initialize 也会因为等 StartFrame 而与接收循环互等。
        if self._output is not None and self._output_ready.is_set():
            try:
                if ordered:
                    from pipecat.frames.frames import (  # type: ignore
                        OutputTransportMessageFrame,
                    )

                    await self._output.queue_frame(
                        OutputTransportMessageFrame(message=dict(payload))
                    )
                else:
                    from pipecat.frames.frames import (  # type: ignore
                        OutputTransportMessageUrgentFrame,
                    )

                    await self._output.queue_frame(
                        OutputTransportMessageUrgentFrame(message=dict(payload))
                    )
                return
            except Exception:
                logger.debug("控制消息入队失败，回退直接发送", exc_info=True)

        await self._safe_send(payload)

    async def _safe_send(self, payload: Mapping[str, Any]) -> None:
        try:
            await self._send_raw(payload)
        except Exception:
            logger.debug("WS 直接发送失败", exc_info=True)

    async def _send_raw(self, payload: Mapping[str, Any]) -> None:
        text = protocol.encode_message(payload)
        async with self._raw_send_lock:
            await self.websocket.send_text(text)

    async def _send_tool(
        self,
        state: str,
        *,
        name: str,
        display: Optional[str] = None,
        error: Optional[str] = None,
    ) -> None:
        await self._send_control(
            protocol.tool_message(state, name=name, display=display, error=error)
        )

    async def _send_device_mcp_payload(self, payload: Mapping[str, Any]) -> None:
        await self._send_control(protocol.mcp_message(payload))

    # ── SerializerHooks ─────────────────────────────────────────
    def note_input_audio(self, num_bytes: int) -> None:
        self.info.audio_in_bytes += max(0, int(num_bytes))
        self.info.touch()
        ws_sessions.add_audio(input_bytes=num_bytes)

    def note_output_audio(self, num_bytes: int, sample_rate: int) -> None:
        self.gate.note_output_audio(num_bytes, sample_rate)
        self.info.audio_out_bytes += max(0, int(num_bytes))
        self.info.touch()
        ws_sessions.add_audio(output_bytes=num_bytes)

    def on_interruption(self) -> None:
        self.info.touch()

    # ── 清理 ────────────────────────────────────────────────────
    async def _cleanup(self) -> None:
        if self._cleanup_done:
            return
        self._cleanup_done = True
        self._closed = True

        if self._watchdog_task is not None:
            self._watchdog_task.cancel()
            self._watchdog_task = None
        if self._ack_task is not None:
            self._ack_task.cancel()
            self._ack_task = None

        # 先让 pending 的设备 MCP 调用收到明确错误（后台任务会消费掉），
        # 再取消仍在等 HTTP 的 extras 任务。
        await self.device_bridge.close()
        if self._device_tools_task is not None:
            self._device_tools_task.cancel()
            self._device_tools_task = None

        runner = self._runner
        if runner is not None:
            try:
                await asyncio.wait_for(runner.cancel(reason="session_cleanup"), timeout=3.0)
            except Exception:
                pass

        # 会话结束写摘要（不存原始音频；失败只记日志）。
        try:
            await self._write_memory_summary()
        except Exception as exc:
            logger.info("会话摘要写入失败：%s", type(exc).__name__)

        if self.codec is not None:
            try:
                self.codec.close()
            except Exception:
                pass

        ws_sessions.pop(self.session_id)
        try:
            await self.websocket.close()
        except Exception:
            pass
        logger.info(
            "WS 会话结束 session=%s turns=%d in=%d out=%d",
            self.session_id,
            self.info.turn_count,
            self.info.audio_in_bytes,
            self.info.audio_out_bytes,
        )

    async def _write_memory_summary(self) -> None:
        if (
            not self.memory_feature_enabled
            or not self.memory.enabled
            or self._bundle is None
        ):
            return
        context = getattr(self._bundle, "context", None)
        messages = list(getattr(context, "messages", []) or [])
        summary = short_term.summarize_messages(messages)
        turns = short_term.collect_turns(messages)
        if not summary and not turns:
            return
        await asyncio.wait_for(
            self.memory.write(
                session_id=self.session_id,
                summary=summary,
                user_id=self.context.user_id,
                device_id=self.context.device_id,
                dept_id=self.context.dept_id,
                turns=turns,
                message_count=len(messages),
                source="pipecat",
            ),
            timeout=3.0,
        )

    async def _close(self, code: int, reason: str) -> None:
        self._closed = True
        try:
            await self.websocket.close(code=code, reason=reason[:120])
        except Exception:
            pass


# ════════════════════════════════════════════════════════════════
# 辅助
# ════════════════════════════════════════════════════════════════


class _SessionDisconnected(Exception):
    """客户端在 hello 前断开（不是协议错误）。"""


def _arguments(params: Any) -> Dict[str, Any]:
    arguments = getattr(params, "arguments", None)
    if isinstance(arguments, Mapping):
        return dict(arguments)
    return {}


def _frame_confidence(frame: Any) -> Optional[float]:
    value = getattr(frame, "confidence", None)
    if value is not None:
        try:
            return float(value)
        except (TypeError, ValueError):
            return None
    result = getattr(frame, "result", None)
    if isinstance(result, Mapping):
        for key in ("confidence", "probability", "score"):
            if key in result:
                try:
                    return float(result[key])
                except (TypeError, ValueError):
                    return None
    return None


def _consume_task_exception(task: asyncio.Task) -> None:
    """后台任务异常不冒泡到事件循环（避免 "exception was never retrieved"）。"""

    if task.cancelled():
        return
    try:
        exc = task.exception()
    except Exception:
        return
    if exc is not None:
        logger.debug("后台任务失败：%s", type(exc).__name__)


async def _safe_result_callback(params: Any, text: str) -> None:
    """把工具结果回给 LLM；回调异常不能影响音频。"""

    callback = getattr(params, "result_callback", None)
    if callback is None:
        return
    try:
        await callback(text or "")
    except Exception:
        logger.info("工具结果回写 LLM 失败", exc_info=True)


__all__ = (
    "WebSocketVoiceSession",
    "WSSessionInfo",
    "WSSessionRegistry",
    "SERVER_VERSION",
    "ws_sessions",
)
