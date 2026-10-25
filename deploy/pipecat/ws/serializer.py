"""Pipecat FrameSerializer 的 WebSocket 实现 + 音频格式适配。

Pipecat 1.13 的 ``FastAPIWebsocketTransport`` 把每个 WS 消息交给
``FrameSerializer.deserialize()``（可以是异步），把输出帧交给
``FrameSerializer.serialize()``。本模块实现这两个方向：

- 上行文本：解析小智协议，交给会话层处理副作用（listen/abort/mcp/ping）；
- 上行二进制：按协商格式解码为 PCM16@16k，再包成 ``InputAudioRawFrame``；
- 下行音频：``OutputAudioRawFrame`` → PCM16 或裸 Opus 包；
- 下行文本：``OutputTransportMessageFrame`` → JSON 文本。

## 半双工

``HalfDuplexGate`` 用**实际写出的音频时长**延长静音窗口（再保留 300ms
尾巴），而不是看 TTS 的开始/结束事件。这样即使 TTS 帧已经排队、输出
传输还在按实时速率发送，上行也不会提前恢复。

## 为什么不在模块顶层 import pipecat

控制面在没有安装 pipecat 时仍应能启动（降级），测试也需要在纯 Python
环境跑协议/半双工逻辑。pipecat 的类在工厂函数里动态导入。
"""

from __future__ import annotations

import logging
import threading
import time
from typing import Any, Awaitable, Callable, Optional

import transport
from audio.opus_codec import OpusCodec, OpusCodecUnavailable, opus_available

logger = logging.getLogger("ai-camera-pipecat.ws.serializer")

INTERNAL_INPUT_RATE = 16000
INTERNAL_OUTPUT_RATE = 24000
DEFAULT_TAIL_SECS = 0.3
# 单个二进制音频帧上限：16k PCM16 约 0.5s / Opus 包远小于此。
MAX_AUDIO_FRAME_BYTES = 32 * 1024


class CodecError(RuntimeError):
    """音频格式协商失败（例如 Opus 依赖缺失）。"""


# ════════════════════════════════════════════════════════════════
# 音频 codec 适配
# ════════════════════════════════════════════════════════════════


class Pcm16Codec:
    """PCM16 直通 + 采样率转换。"""

    def __init__(self, input_rate: int = INTERNAL_INPUT_RATE, output_rate: int = INTERNAL_OUTPUT_RATE):
        self.input_rate = int(input_rate)
        self.output_rate = int(output_rate)

    def decode(self, data: bytes) -> bytes:
        """客户端 PCM16 → 内部 PCM16@16k。"""

        if not data:
            return b""
        if len(data) % 2 != 0:
            data = data[:-1]
        if self.input_rate == INTERNAL_INPUT_RATE:
            return data
        return transport.resample_pcm16(data, self.input_rate, INTERNAL_INPUT_RATE)

    def encode(self, pcm: bytes, sample_rate: int = INTERNAL_OUTPUT_RATE) -> bytes:
        """内部/下行 PCM16 → 协商的下行采样率。"""

        if not pcm:
            return b""
        rate = int(sample_rate or INTERNAL_OUTPUT_RATE)
        if rate == self.output_rate:
            return pcm
        return transport.resample_pcm16(pcm, rate, self.output_rate)

    def reset_output(self) -> None:
        pass

    def reset_input(self) -> None:
        pass

    def close(self) -> None:
        pass


class OpusAudioCodec:
    """裸 Opus 帧 ↔ PCM16。

    解码固定输出 16kHz（内部统一），编码输入固定 24kHz（下行统一）。
    """

    def __init__(self, output_rate: int = INTERNAL_OUTPUT_RATE):
        if not opus_available():
            raise OpusCodecUnavailable(
                "当前环境没有可用的 Opus 编解码器（需要 av/libopus）"
            )
        self.output_rate = int(output_rate)
        self._codec = OpusCodec(
            encode_rate=INTERNAL_OUTPUT_RATE,
            decode_rate=INTERNAL_INPUT_RATE,
        )

    def decode(self, data: bytes) -> bytes:
        return self._codec.decode(data)

    def encode(self, pcm: bytes, sample_rate: int = INTERNAL_OUTPUT_RATE) -> bytes:
        rate = int(sample_rate or INTERNAL_OUTPUT_RATE)
        if rate != INTERNAL_OUTPUT_RATE:
            pcm = transport.resample_pcm16(pcm, rate, INTERNAL_OUTPUT_RATE)
        if self.output_rate != INTERNAL_OUTPUT_RATE:
            # 目前下行固定 24k；保留这个分支避免未来改协商时静默出错。
            pcm = transport.resample_pcm16(pcm, INTERNAL_OUTPUT_RATE, self.output_rate)
        return self._codec.encode(pcm)

    def reset_output(self) -> None:
        self._codec.reset_output()

    def reset_input(self) -> None:
        self._codec.reset_input()

    def close(self) -> None:
        self._codec.close()


def build_codec(
    audio_format: str,
    input_rate: int = INTERNAL_INPUT_RATE,
    output_rate: int = INTERNAL_OUTPUT_RATE,
):
    """按 hello 协商结果创建 codec。

    失败时抛 :class:`CodecError`，消息里直接说明缺什么（不打印音频/密钥）。
    """

    fmt = str(audio_format or "").strip().lower()
    if fmt == "pcm16":
        return Pcm16Codec(input_rate=input_rate, output_rate=output_rate)
    if fmt == "opus":
        try:
            return OpusAudioCodec(output_rate=output_rate)
        except OpusCodecUnavailable as exc:
            raise CodecError(str(exc)) from exc
    raise CodecError(f"不支持的音频格式 {audio_format!r}；可选 pcm16 / opus")


# ════════════════════════════════════════════════════════════════
# 半双工闸门
# ════════════════════════════════════════════════════════════════


class HalfDuplexGate:
    """TTS 播放期间丢弃上行，并保留 300ms 尾巴。

    - ``begin_listen()`` / ``end_listen()``：客户端 listen/start|detect|stop；
    - ``note_output_audio()``：每写出一段下行音频，延长静音窗口；
    - ``close_output()``：TTS stop 时再补一个尾巴；
    - ``abort()``：立即取消静音，允许用户马上说话；
    - ``barge_in=True``：协议预留开关，默认关闭。
    """

    def __init__(
        self,
        *,
        tail_secs: float = DEFAULT_TAIL_SECS,
        barge_in: bool = False,
    ) -> None:
        self.tail_secs = max(0.0, float(tail_secs))
        self.barge_in = bool(barge_in)
        self._lock = threading.Lock()
        self._listening = False
        self._muted_until = 0.0
        # TTS start→stop 期间无条件丢弃上行（即使音频还没开始写）。
        self._output_active = False
        self._generation = 0

    def begin_listen(self) -> None:
        with self._lock:
            self._listening = True

    def end_listen(self) -> None:
        with self._lock:
            self._listening = False

    def begin_output(self) -> None:
        """TTS start：立即丢弃上行，直到 ``close_output`` + 尾巴。"""

        if self.barge_in:
            return
        with self._lock:
            self._output_active = True

    def note_output_audio(
        self,
        num_bytes: int,
        sample_rate: int,
        *,
        now: Optional[float] = None,
    ) -> float:
        """按 PCM16 字节数计算时长并延长静音窗口，返回静音截止时刻。"""

        if self.barge_in or num_bytes <= 0 or sample_rate <= 0:
            return self._muted_until
        current = time.monotonic() if now is None else float(now)
        duration = (num_bytes / 2.0) / float(sample_rate)
        until = current + duration + self.tail_secs
        with self._lock:
            self._muted_until = max(self._muted_until, until)
            return self._muted_until

    def close_output(self, *, now: Optional[float] = None) -> float:
        """TTS stop：结束无条件丢弃，并保留尾巴。"""

        if self.barge_in:
            return self._muted_until
        current = time.monotonic() if now is None else float(now)
        with self._lock:
            self._output_active = False
            self._muted_until = max(self._muted_until, current + self.tail_secs)
            return self._muted_until

    def suppress(self, seconds: float, *, now: Optional[float] = None) -> None:
        """手动静音一段时间（快速应答等场景）。"""

        if self.barge_in or seconds <= 0:
            return
        current = time.monotonic() if now is None else float(now)
        with self._lock:
            self._muted_until = max(self._muted_until, current + seconds)

    def abort(self) -> None:
        """打断当前输出：解除静音并推进 generation。"""

        with self._lock:
            self._muted_until = 0.0
            self._output_active = False
            self._generation += 1

    def allow_uplink(self, *, now: Optional[float] = None) -> bool:
        if self.barge_in:
            return self._listening
        current = time.monotonic() if now is None else float(now)
        with self._lock:
            if self._output_active:
                return False
            return self._listening and current >= self._muted_until

    @property
    def listening(self) -> bool:
        with self._lock:
            return self._listening

    @property
    def speaking(self) -> bool:
        with self._lock:
            return self._output_active or time.monotonic() < self._muted_until

    @property
    def muted_until(self) -> float:
        with self._lock:
            return self._muted_until

    @property
    def generation(self) -> int:
        with self._lock:
            return self._generation

    def reset(self) -> None:
        with self._lock:
            self._listening = False
            self._muted_until = 0.0
            self._output_active = False
            self._generation += 1


# ════════════════════════════════════════════════════════════════
# Serializer 工厂
# ════════════════════════════════════════════════════════════════


class SerializerHooks:
    """会话层需要提供给 serializer 的最小接口。"""

    gate: HalfDuplexGate
    codec: Any

    async def handle_text(self, raw: str) -> None:  # pragma: no cover - 接口声明
        raise NotImplementedError

    def note_input_audio(self, num_bytes: int) -> None:  # pragma: no cover
        pass

    def note_output_audio(self, num_bytes: int, sample_rate: int) -> None:  # pragma: no cover
        pass

    def on_interruption(self) -> None:  # pragma: no cover
        pass


def build_serializer_class(base_cls: type, hooks: SerializerHooks) -> type:
    """动态构造 ``FrameSerializer`` 子类。

    参数 ``base_cls`` 由调用方从 ``pipecat.serializers.base_serializer``
    导入；本函数不直接依赖 pipecat，方便测试时传一个假基类。
    """

    class VoiceWebSocketSerializer(base_cls):  # type: ignore[misc, valid-type]
        """小智协议子集的 FrameSerializer。"""

        async def serialize(self, frame: Any):
            # 延迟导入：只有真正跑 Pipecat 时才需要这些类。
            try:
                if self.should_ignore_frame(frame):
                    # 过滤 RTVI 等 Pipecat 自带协议消息，避免污染小智协议。
                    return None
            except Exception:
                pass
            from pipecat.frames.frames import (
                InterruptionFrame,
                OutputAudioRawFrame,
                OutputTransportMessageFrame,
                OutputTransportMessageUrgentFrame,
            )

            if isinstance(frame, InterruptionFrame):
                try:
                    hooks.codec.reset_output()
                    hooks.on_interruption()
                except Exception:
                    logger.debug("打断时清理输出失败", exc_info=True)
                return None

            if isinstance(frame, OutputAudioRawFrame):
                pcm = getattr(frame, "audio", b"") or b""
                if not pcm:
                    return None
                rate = int(getattr(frame, "sample_rate", 0) or INTERNAL_OUTPUT_RATE)
                try:
                    payload = hooks.codec.encode(pcm, rate)
                except Exception as exc:
                    logger.warning("下行音频编码失败：%s", type(exc).__name__)
                    return None
                if payload:
                    hooks.note_output_audio(len(pcm), rate)
                return payload or None

            if isinstance(
                frame, (OutputTransportMessageFrame, OutputTransportMessageUrgentFrame)
            ):
                message = getattr(frame, "message", None)
                if not isinstance(message, dict):
                    return None
                # 紧凑 JSON；调用方保证 message 可序列化。
                import json

                try:
                    return json.dumps(message, ensure_ascii=False, separators=(",", ":"))
                except (TypeError, ValueError):
                    logger.warning("下行消息不是合法 JSON，已丢弃")
                    return None

            return None

        async def deserialize(self, data: Any):
            from pipecat.frames.frames import InputAudioRawFrame

            if isinstance(data, (bytes, bytearray)):
                if len(data) > MAX_AUDIO_FRAME_BYTES:
                    logger.debug("丢弃超大音频帧：%d bytes", len(data))
                    return None
                if not hooks.gate.allow_uplink():
                    # 半双工：播放期间直接丢弃，避免自激。
                    return None
                try:
                    pcm = hooks.codec.decode(bytes(data))
                except Exception as exc:
                    logger.debug("上行音频解码失败，丢弃：%s", type(exc).__name__)
                    return None
                if not pcm:
                    return None
                hooks.note_input_audio(len(pcm))
                return InputAudioRawFrame(
                    audio=pcm,
                    sample_rate=INTERNAL_INPUT_RATE,
                    num_channels=1,
                )

            if isinstance(data, str):
                try:
                    await hooks.handle_text(data)
                except Exception:
                    # 控制面绝不能因为一条坏消息让接收循环退出。
                    logger.warning("处理上行文本失败", exc_info=True)
                return None

            return None

    return VoiceWebSocketSerializer


__all__ = (
    "CodecError",
    "HalfDuplexGate",
    "INTERNAL_INPUT_RATE",
    "INTERNAL_OUTPUT_RATE",
    "MAX_AUDIO_FRAME_BYTES",
    "OpusAudioCodec",
    "Pcm16Codec",
    "SerializerHooks",
    "build_codec",
    "build_serializer_class",
)
