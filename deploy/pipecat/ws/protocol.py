"""小智兼容的 WebSocket 协议子集（v1）。

协议只覆盖本项目需要的最小集合：``hello / listen / abort / stt / llm /
tts / mcp / tool / error / wake / ping / pong`` + 二进制音频。所有消息都是
UTF-8 JSON 文本帧；音频永远是二进制帧。

## 上行音频

- ``pcm16``：App/Web，16-bit 小端单声道，上行 16kHz（允许 8kHz，内部
  重采样到 16kHz）；
- ``opus``：ESP32，小智 v1 裸帧（每个 WS 二进制消息 = 一个 Opus 包），
  上行 16kHz。

下行统一 24kHz，格式与客户端 ``hello.audio_params.format`` 一致。

## 设计约束

- 解析函数只做**结构校验**，不做业务副作用；
- 所有错误都是 :class:`ProtocolError`，带稳定的 ``code``，由会话层转成
  ``{"type":"error"}`` 下发；
- 文本字段有长度上限，避免恶意客户端把超大 JSON 塞进内存。
"""

from __future__ import annotations

import json
import re
from dataclasses import dataclass, field
from typing import Any, Dict, Mapping, Optional, Tuple

PROTOCOL_VERSION = 1

# 内部统一采样率：VAD/STT 16k，TTS/下行 24k。
INTERNAL_INPUT_RATE = 16000
INTERNAL_OUTPUT_RATE = 24000

SUPPORTED_FORMATS = ("pcm16", "opus")
SUPPORTED_INPUT_RATES = (8000, 16000)
SUPPORTED_FRAME_DURATION = (10, 20, 40, 60)

CLIENT_MESSAGE_TYPES = ("hello", "listen", "abort", "mcp", "ping")
SERVER_MESSAGE_TYPES = (
    "hello",
    "listen",
    "stt",
    "llm",
    "tts",
    "mcp",
    "tool",
    "error",
    "wake",
    "ping",
    "pong",
    "status",
)

# 单条 JSON 消息上限（不含音频，音频是独立二进制帧）。
MAX_MESSAGE_BYTES = 256 * 1024
MAX_LISTEN_TEXT = 200
MAX_ABORT_REASON = 200
MAX_HELLO_CLIENT_ID = 128

_SESSION_ID_RE = re.compile(r"^[0-9A-Za-z_.:-]{1,128}$")


class ProtocolError(ValueError):
    """协议错误（HTTP/WS 层映射为 ``error`` 消息 + 关闭码）。"""

    def __init__(self, code: str, message: str, retryable: bool = False) -> None:
        super().__init__(message)
        self.code = code
        self.message = message
        self.retryable = retryable

    def as_message(self) -> Dict[str, Any]:
        return error_message(self.code, self.message, self.retryable)


# ════════════════════════════════════════════════════════════════
# 数据结构
# ════════════════════════════════════════════════════════════════


@dataclass(frozen=True)
class AudioParams:
    """客户端/服务端 hello 中的音频协商参数。"""

    format: str = "pcm16"
    sample_rate: int = INTERNAL_INPUT_RATE
    channels: int = 1
    frame_duration: int = 20

    def validate(self, *, input_side: bool) -> "AudioParams":
        fmt = str(self.format or "").strip().lower()
        if fmt not in SUPPORTED_FORMATS:
            raise ProtocolError(
                "unsupported_audio_format",
                f"不支持的音频格式 {self.format!r}；可选 pcm16 / opus",
            )
        if int(self.channels) != 1:
            raise ProtocolError("unsupported_channels", "只支持单声道（channels=1）")
        if int(self.frame_duration) not in SUPPORTED_FRAME_DURATION:
            raise ProtocolError(
                "unsupported_frame_duration",
                f"不支持的 frame_duration={self.frame_duration}",
            )
        rate = int(self.sample_rate)
        if input_side:
            if fmt == "opus" and rate != INTERNAL_INPUT_RATE:
                raise ProtocolError(
                    "unsupported_sample_rate",
                    "Opus 上行采样率必须是 16000",
                )
            if fmt == "pcm16" and rate not in SUPPORTED_INPUT_RATES:
                raise ProtocolError(
                    "unsupported_sample_rate",
                    "PCM16 上行采样率只支持 8000 / 16000",
                )
        elif rate != INTERNAL_OUTPUT_RATE:
            raise ProtocolError(
                "unsupported_sample_rate",
                "服务端下行采样率固定 24000",
            )
        return AudioParams(
            format=fmt,
            sample_rate=rate,
            channels=1,
            frame_duration=int(self.frame_duration),
        )


@dataclass(frozen=True)
class ClientHello:
    version: int
    transport: str
    features: Mapping[str, Any]
    audio_params: AudioParams
    client: Mapping[str, Any]

    @property
    def client_kind(self) -> str:
        return str(self.client.get("kind") or "").strip().lower()

    @property
    def client_id(self) -> str:
        return str(self.client.get("client_id") or "").strip()

    @property
    def device_id(self) -> str:
        return str(self.client.get("device_id") or "").strip()

    @property
    def mcp_enabled(self) -> bool:
        return bool(self.features.get("mcp"))


@dataclass(frozen=True)
class ListenCommand:
    state: str
    mode: str = "auto"
    text: str = ""


@dataclass(frozen=True)
class AbortCommand:
    reason: str = ""


@dataclass(frozen=True)
class McpCommand:
    payload: Mapping[str, Any]


@dataclass(frozen=True)
class ParsedMessage:
    """统一的客户端消息解析结果。"""

    type: str
    hello: Optional[ClientHello] = None
    listen: Optional[ListenCommand] = None
    abort: Optional[AbortCommand] = None
    mcp: Optional[McpCommand] = None
    ping_ts: Any = None


# ════════════════════════════════════════════════════════════════
# 解析
# ════════════════════════════════════════════════════════════════


def _require_mapping(value: Any, field_name: str) -> Mapping[str, Any]:
    if not isinstance(value, Mapping):
        raise ProtocolError("invalid_message", f"{field_name} 必须是 JSON 对象")
    return value


def _optional_str(value: Any, field_name: str, max_len: int) -> str:
    if value is None:
        return ""
    if not isinstance(value, str):
        raise ProtocolError("invalid_message", f"{field_name} 必须是字符串")
    text = value.strip()
    if len(text) > max_len:
        raise ProtocolError("invalid_message", f"{field_name} 过长（>{max_len}）")
    return text


def parse_message(raw: str) -> ParsedMessage:
    """解析一条客户端文本消息。

    抛 :class:`ProtocolError` 的场景：超大消息、非法 JSON、未知 type、
    字段类型/取值不合法。
    """

    if not isinstance(raw, str):
        raise ProtocolError("invalid_message", "文本帧必须是 JSON 字符串")
    if len(raw.encode("utf-8")) > MAX_MESSAGE_BYTES:
        raise ProtocolError("message_too_large", "消息超过 256KB 上限")

    try:
        payload = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise ProtocolError("invalid_json", f"JSON 解析失败：{exc.msg}") from exc

    if not isinstance(payload, dict):
        raise ProtocolError("invalid_message", "消息必须是 JSON 对象")

    msg_type = payload.get("type")
    if not isinstance(msg_type, str) or not msg_type.strip():
        raise ProtocolError("missing_type", "消息缺少 type 字段")
    msg_type = msg_type.strip().lower()
    if msg_type not in CLIENT_MESSAGE_TYPES:
        raise ProtocolError(
            "unsupported_type",
            f"不支持的消息类型 {msg_type!r}；可选 {' / '.join(CLIENT_MESSAGE_TYPES)}",
        )

    if msg_type == "hello":
        return ParsedMessage(type="hello", hello=parse_hello(payload))
    if msg_type == "listen":
        return ParsedMessage(type="listen", listen=parse_listen(payload))
    if msg_type == "abort":
        return ParsedMessage(type="abort", abort=parse_abort(payload))
    if msg_type == "mcp":
        return ParsedMessage(type="mcp", mcp=parse_mcp(payload))
    return ParsedMessage(type="ping", ping_ts=payload.get("ts"))


def parse_hello(payload: Mapping[str, Any]) -> ClientHello:
    version = payload.get("version", PROTOCOL_VERSION)
    try:
        version = int(version)
    except (TypeError, ValueError) as exc:
        raise ProtocolError("invalid_hello", "hello.version 必须是整数") from exc
    if version != PROTOCOL_VERSION:
        raise ProtocolError(
            "unsupported_version", f"不支持的协议版本 {version}；当前为 {PROTOCOL_VERSION}"
        )

    transport = str(payload.get("transport") or "websocket").strip().lower()
    if transport != "websocket":
        raise ProtocolError("invalid_hello", "hello.transport 只支持 websocket")

    audio_raw = _require_mapping(payload.get("audio_params") or {}, "audio_params")
    try:
        params = AudioParams(
            format=str(audio_raw.get("format") or "pcm16"),
            sample_rate=int(audio_raw.get("sample_rate") or INTERNAL_INPUT_RATE),
            channels=int(audio_raw.get("channels") or 1),
            frame_duration=int(audio_raw.get("frame_duration") or 20),
        )
    except (TypeError, ValueError) as exc:
        raise ProtocolError("invalid_hello", "audio_params 字段必须是整数") from exc
    params = params.validate(input_side=True)

    features = _require_mapping(payload.get("features") or {}, "features")
    client = _require_mapping(payload.get("client") or {}, "client")
    client_id = _optional_str(client.get("client_id"), "client.client_id", MAX_HELLO_CLIENT_ID)
    if client_id and not _SESSION_ID_RE.match(client_id):
        # client_id 只是标识，不做强校验；仅拒绝控制字符/空格。
        raise ProtocolError("invalid_hello", "client.client_id 含非法字符")

    return ClientHello(
        version=version,
        transport=transport,
        features=features,
        audio_params=params,
        client=client,
    )


def parse_listen(payload: Mapping[str, Any]) -> ListenCommand:
    state = str(payload.get("state") or "").strip().lower()
    if state not in ("detect", "start", "stop"):
        raise ProtocolError(
            "invalid_listen", "listen.state 只支持 detect / start / stop"
        )
    # mode 可以省略（例如 listen/stop 通常不带）；空字符串表示「沿用当前」。
    mode = str(payload.get("mode") or "").strip().lower()
    if mode not in ("", "auto", "manual"):
        raise ProtocolError("invalid_listen", "listen.mode 只支持 auto / manual")
    text = _optional_str(payload.get("text"), "listen.text", MAX_LISTEN_TEXT)
    return ListenCommand(state=state, mode=mode, text=text)


def parse_abort(payload: Mapping[str, Any]) -> AbortCommand:
    reason = _optional_str(payload.get("reason"), "abort.reason", MAX_ABORT_REASON)
    return AbortCommand(reason=reason)


def parse_mcp(payload: Mapping[str, Any]) -> McpCommand:
    inner = _require_mapping(payload.get("payload"), "mcp.payload")
    validate_jsonrpc(inner)
    return McpCommand(payload=inner)


def validate_jsonrpc(payload: Mapping[str, Any]) -> None:
    if str(payload.get("jsonrpc") or "") != "2.0":
        raise ProtocolError("invalid_jsonrpc", "JSON-RPC 版本必须是 2.0")
    method = payload.get("method")
    has_result = "result" in payload or "error" in payload
    if method is None and not has_result:
        raise ProtocolError("invalid_jsonrpc", "JSON-RPC 消息缺少 method 或 result")
    if method is not None and not isinstance(method, str):
        raise ProtocolError("invalid_jsonrpc", "JSON-RPC method 必须是字符串")
    msg_id = payload.get("id")
    if msg_id is not None and not isinstance(msg_id, (str, int)):
        raise ProtocolError("invalid_jsonrpc", "JSON-RPC id 必须是字符串或整数")


# ════════════════════════════════════════════════════════════════
# 构造（服务端 → 客户端）
# ════════════════════════════════════════════════════════════════


def encode_message(payload: Mapping[str, Any]) -> str:
    """JSON 序列化（紧凑、保留中文，日志里只记录 type）。"""

    return json.dumps(payload, ensure_ascii=False, separators=(",", ":"))


def server_hello(
    session_id: str,
    *,
    audio_format: str = "pcm16",
    output_rate: int = INTERNAL_OUTPUT_RATE,
    frame_duration: int = 20,
    capabilities: Optional[Mapping[str, Any]] = None,
) -> Dict[str, Any]:
    caps = dict(capabilities or {})
    caps.setdefault("mcp", True)
    caps.setdefault("memory", True)
    caps.setdefault("barge_in", False)
    return {
        "type": "hello",
        "session_id": session_id,
        "transport": "websocket",
        "audio_params": {
            "format": audio_format,
            "sample_rate": int(output_rate),
            "channels": 1,
            "frame_duration": int(frame_duration),
        },
        "capabilities": caps,
    }


def stt_message(
    text: str,
    *,
    is_final: bool,
    confidence: Optional[float] = None,
    turn_id: Optional[int] = None,
    language: Optional[str] = None,
) -> Dict[str, Any]:
    body: Dict[str, Any] = {
        "type": "stt",
        "text": text,
        "is_final": bool(is_final),
    }
    if confidence is not None:
        body["confidence"] = round(float(confidence), 4)
    if turn_id is not None:
        body["turn_id"] = turn_id
    if language:
        body["language"] = language
    return body


def llm_message(text: str, *, emotion: Optional[str] = None) -> Dict[str, Any]:
    body: Dict[str, Any] = {"type": "llm", "text": text}
    if emotion:
        body["emotion"] = emotion
    return body


def tts_message(
    state: str,
    *,
    text: Optional[str] = None,
    turn_id: Optional[int] = None,
) -> Dict[str, Any]:
    if state not in ("start", "sentence_start", "stop"):
        raise ValueError("tts state 只支持 start / sentence_start / stop")
    body: Dict[str, Any] = {"type": "tts", "state": state}
    if text is not None:
        body["text"] = text
    if turn_id is not None:
        body["turn_id"] = turn_id
    return body


def tool_message(
    state: str,
    *,
    name: str,
    display: Optional[str] = None,
    error: Optional[str] = None,
) -> Dict[str, Any]:
    if state not in ("calling", "result", "error"):
        raise ValueError("tool state 只支持 calling / result / error")
    body: Dict[str, Any] = {"type": "tool", "state": state, "name": name}
    if display:
        body["display"] = display
    if error:
        body["error"] = error
    return body


def error_message(code: str, message: str, retryable: bool = False) -> Dict[str, Any]:
    return {
        "type": "error",
        "code": code,
        "message": message,
        "retryable": bool(retryable),
    }


def wake_message(text: str = "", *, confidence: Optional[float] = None) -> Dict[str, Any]:
    body: Dict[str, Any] = {"type": "wake", "state": "detected", "text": text}
    if confidence is not None:
        body["confidence"] = round(float(confidence), 4)
    return body


def mcp_message(payload: Mapping[str, Any]) -> Dict[str, Any]:
    return {"type": "mcp", "payload": dict(payload)}


def pong_message(ts: Any = None) -> Dict[str, Any]:
    body: Dict[str, Any] = {"type": "pong"}
    if ts is not None:
        body["ts"] = ts
    return body


def status_message(**fields: Any) -> Dict[str, Any]:
    body: Dict[str, Any] = {"type": "status"}
    body.update(fields)
    return body


__all__ = (
    "AbortCommand",
    "AudioParams",
    "CLIENT_MESSAGE_TYPES",
    "ClientHello",
    "INTERNAL_INPUT_RATE",
    "INTERNAL_OUTPUT_RATE",
    "ListenCommand",
    "MAX_MESSAGE_BYTES",
    "McpCommand",
    "PROTOCOL_VERSION",
    "ParsedMessage",
    "ProtocolError",
    "SERVER_MESSAGE_TYPES",
    "SUPPORTED_FORMATS",
    "encode_message",
    "error_message",
    "llm_message",
    "mcp_message",
    "parse_hello",
    "parse_listen",
    "parse_abort",
    "parse_mcp",
    "parse_message",
    "pong_message",
    "server_hello",
    "status_message",
    "stt_message",
    "tool_message",
    "tts_message",
    "validate_jsonrpc",
    "wake_message",
)
