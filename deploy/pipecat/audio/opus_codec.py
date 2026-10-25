"""Opus 编解码（PyAV / libopus）。

小智设备链路使用 **Opus 裸帧 v1**：WebSocket 每个二进制消息就是一段
Opus 包，没有长度前缀、没有容器。上行 16kHz、下行 24kHz（libopus 内部
会处理到 48kHz 的重采样）。

## 为什么用 PyAV

``av`` 已经作为 aiortc 的依赖出现在 requirements 里，因此不需要新增
原生依赖。PyAV 同时提供 libopus 编解码器和重采样器：

- 解码：libopus 固定输出 48kHz，再用 ``av.AudioResampler`` 转到 16kHz；
- 编码：输入 24kHz s16 单声道，``frame_size`` 为 480 samples（20ms）。

## 线程/协程安全

同一个 codec 实例可能同时被输入（解码）和输出（编码）路径调用。这里
用一把 ``threading.Lock`` 串行化，避免 PyAV 上下文被并发访问。锁内只做
很短的 CPU 工作，不会阻塞事件循环太久（单帧 20ms）。

## 失败语义

``av`` 缺失或 libopus 不可用时抛 :class:`OpusCodecUnavailable`，由上层
返回明确的 ``error`` 消息，而不是让整个进程崩溃。日志只记录异常类型，
不打印音频内容。
"""

from __future__ import annotations

import logging
import threading
from fractions import Fraction
from typing import List, Optional

logger = logging.getLogger("ai-camera-pipecat.audio.opus")

DEFAULT_ENCODE_RATE = 24000
DEFAULT_DECODE_RATE = 16000
DEFAULT_FRAME_MS = 20


class OpusCodecUnavailable(RuntimeError):
    """PyAV / libopus 不可用。"""


class OpusCodecError(RuntimeError):
    """编码或解码失败（数据非法、上下文状态异常）。"""


def _import_av():
    try:
        import av  # type: ignore
    except Exception as exc:  # ImportError / 原生库加载失败
        raise OpusCodecUnavailable(
            f"PyAV 不可用（requirements.txt 需安装 av）：{type(exc).__name__}: {exc}"
        ) from exc
    return av


def opus_available() -> bool:
    """探测当前环境能否创建 Opus 编解码器（不发网络请求）。"""

    try:
        av = _import_av()
        codec = av.CodecContext.create("libopus", "r")
        codec.close() if hasattr(codec, "close") else None
        return True
    except Exception:
        return False


class OpusCodec:
    """裸 Opus 帧 ↔ PCM16 转换器。

    参数：
        encode_rate: 编码输入采样率（下行 24000）。
        decode_rate: 解码输出采样率（内部统一 16000）。
        bitrate: 编码码率（bps）。
        frame_ms: 编码帧长（毫秒），只支持 2.5/5/10/20/40/60；
            当前固定 20ms。
    """

    def __init__(
        self,
        encode_rate: int = DEFAULT_ENCODE_RATE,
        decode_rate: int = DEFAULT_DECODE_RATE,
        bitrate: int = 24000,
        frame_ms: int = DEFAULT_FRAME_MS,
    ) -> None:
        if encode_rate <= 0 or decode_rate <= 0:
            raise ValueError("采样率必须大于 0")
        if frame_ms <= 0:
            raise ValueError("frame_ms 必须大于 0")

        self.encode_rate = int(encode_rate)
        self.decode_rate = int(decode_rate)
        self.bitrate = int(bitrate)
        self.frame_ms = int(frame_ms)
        self.frame_samples = int(self.encode_rate * self.frame_ms / 1000)
        self.frame_bytes = self.frame_samples * 2

        self._lock = threading.Lock()
        self._encoder = None
        self._decoder = None
        self._resampler = None
        self._encode_residual = bytearray()
        self._encode_pts = 0
        self._decoder_broken = False

    # ── 懒加载 ──────────────────────────────────────────────────
    def _ensure_encoder(self):
        if self._encoder is not None:
            return self._encoder
        av = _import_av()
        try:
            encoder = av.CodecContext.create("libopus", "w")
            encoder.sample_rate = self.encode_rate
            encoder.layout = "mono"
            encoder.format = "s16"
            encoder.bit_rate = self.bitrate
            encoder.open()
        except Exception as exc:
            raise OpusCodecUnavailable(f"创建 Opus 编码器失败：{type(exc).__name__}: {exc}") from exc
        self._encoder = encoder
        return encoder

    def _ensure_decoder(self):
        if self._decoder is not None and not self._decoder_broken:
            return self._decoder
        av = _import_av()
        try:
            decoder = av.CodecContext.create("libopus", "r")
            resampler = av.AudioResampler(format="s16", layout="mono", rate=self.decode_rate)
        except Exception as exc:
            raise OpusCodecUnavailable(f"创建 Opus 解码器失败：{type(exc).__name__}: {exc}") from exc
        self._decoder = decoder
        self._resampler = resampler
        self._decoder_broken = False
        return decoder

    # ── 解码（上行）──────────────────────────────────────────────
    def decode(self, packet: bytes) -> bytes:
        """一个裸 Opus 包 → PCM16（``decode_rate``，单声道小端）。

        非法包返回空 bytes（调用方丢帧），不抛异常中断音频链路。
        """

        if not packet:
            return b""
        av = _import_av()
        with self._lock:
            try:
                decoder = self._ensure_decoder()
                out = bytearray()
                # av.Packet 会复制 payload；不解析容器，整个消息视为一个 Opus 包。
                for frame in decoder.decode(av.Packet(packet)):
                    for resampled in self._resampler.resample(frame):
                        raw = bytes(resampled.planes[0])
                        out.extend(raw[: resampled.samples * 2])
                return bytes(out)
            except Exception as exc:
                # 丢一个坏包比让接收循环退出更安全；下次重建上下文，
                # 避免解码器状态被损坏后持续失败。
                self._decoder_broken = True
                logger.debug("Opus 解码失败，丢弃该帧：%s", type(exc).__name__)
                return b""

    # ── 编码（下行）──────────────────────────────────────────────
    def encode(self, pcm: bytes) -> bytes:
        """PCM16（``encode_rate``）→ 一个 Opus 包。

        输入按 20ms 切帧。内部保留不足一帧的尾巴；因此当输入不是
        ``frame_bytes`` 整数倍时，本次可能返回空 bytes，下一次调用会补齐。
        对于 20ms 的输出分片（输出传输配置为 20ms），每次调用恰好一个包。
        """

        if not pcm:
            return b""
        if len(pcm) % 2 != 0:
            pcm = pcm[:-1]
        if not pcm:
            return b""
        av = _import_av()
        with self._lock:
            encoder = self._ensure_encoder()
            self._encode_residual.extend(pcm)
            packets: List[bytes] = []
            while len(self._encode_residual) >= self.frame_bytes:
                chunk = bytes(self._encode_residual[: self.frame_bytes])
                del self._encode_residual[: self.frame_bytes]
                try:
                    frame = av.AudioFrame(
                        format="s16", layout="mono", samples=self.frame_samples
                    )
                    frame.sample_rate = self.encode_rate
                    frame.pts = self._encode_pts
                    frame.time_base = Fraction(1, self.encode_rate)
                    frame.planes[0].update(chunk)
                    self._encode_pts += self.frame_samples
                    packets.extend(bytes(packet) for packet in encoder.encode(frame))
                except Exception as exc:
                    raise OpusCodecError(f"Opus 编码失败：{type(exc).__name__}: {exc}") from exc
            # 正常 20ms 一帧，这里最多一个包。若出现多包，保留最后一个由
            # 下一次调用返回会破坏时间顺序，因此记录告警并只发第一个。
            if len(packets) > 1:
                logger.warning("Opus 单次编码返回 %d 个包，只发送第一个", len(packets))
            return packets[0] if packets else b""

    def flush(self) -> bytes:
        """冲刷编码器/解码器缓冲（会话结束时调用）。"""

        with self._lock:
            self._encode_residual.clear()
            encoder = self._encoder
            if encoder is not None:
                try:
                    packets = [bytes(packet) for packet in encoder.encode(None)]
                    return packets[0] if packets else b""
                except Exception:
                    return b""
            return b""

    def reset_output(self) -> None:
        """丢弃下行未编码的尾巴与编码器状态（abort 时调用）。"""

        with self._lock:
            self._encode_residual.clear()
            self._encode_pts = 0
            encoder = self._encoder
            self._encoder = None
            if encoder is not None:
                try:
                    encoder.close()
                except Exception:
                    pass

    def reset_input(self) -> None:
        """丢弃解码器状态（重连或长静音后调用）。"""

        with self._lock:
            self._decoder = None
            self._resampler = None
            self._decoder_broken = False

    def close(self) -> None:
        self.reset_output()
        self.reset_input()

    @property
    def pending_bytes(self) -> int:
        with self._lock:
            return len(self._encode_residual)


__all__ = (
    "DEFAULT_DECODE_RATE",
    "DEFAULT_ENCODE_RATE",
    "DEFAULT_FRAME_MS",
    "OpusCodec",
    "OpusCodecError",
    "OpusCodecUnavailable",
    "opus_available",
)
