"""ZLM 音频接入与回推（**骨架**）。

## 诚实边界

本期（v0.2.0 模块 H+）控制面只做到：配置、会话登记、pipeline 装配与
G.711 ↔ PCM 的纯函数转换。**RTP 收发尚未接通** —— ``/health`` 与
``/session/start`` 的响应里 ``audio_bridge`` 恒为 ``"not_connected"``，
不要据此认为已经可以跟设备对讲。

## 完整链路（后续联调目标）

```text
设备麦克风
  → ZLM 收流（PCMA/G.711A，8kHz 单声道）
  → 从 ZLM 拉音频（HTTP-FLV / RTSP / 或 ZLM 的 startSendRtp 反向通道）
  → G.711A → PCM16（本文件的 pcma_to_pcm16）
  → Pipecat: VAD → STT → LLM → TTS
  → PCM16 → G.711A（pcm16_to_pcma）
  → ZLM startSendRtp 回推设备喇叭
```

**为什么本期不发 RTP**：``startSendRtp`` 的目标端口/SSRC/载荷类型必须与
设备 INVITE 协商出的 SDP 严格一致，且需要与 ZLM 的 RTP server 生命周期
联调（会话建立、设备 ACK、超时回收）。在音频桥接没有端到端验证之前，
控制面不臆造一个「看起来能对讲」的实现。

## 已实现（可单测）

- :func:`pcmu_decode` / :func:`pcmu_encode`（G.711 μ-law）
- :func:`pcma_decode` / :func:`pcma_encode`（G.711 A-law）
- :func:`pcm16_to_pcma` / :func:`pcma_to_pcm16`
- :func:`pcm16_to_pcmu` / :func:`pcmu_to_pcm16`
- :func:`pcmu_to_pcma` / :func:`pcma_to_pcmu`

全部为纯 Python、无第三方依赖，可直接用 ``python -m doctest`` 或
``python -c`` 验证。
"""

from __future__ import annotations

import struct
from abc import ABC, abstractmethod
from typing import Iterable, Iterator, List, Optional

AUDIO_BRIDGE_STATUS = "not_connected"

# G.711 的采样率与帧长。
G711_SAMPLE_RATE = 8000
G711_SAMPLES_PER_20MS = G711_SAMPLE_RATE // 50  # 160 samples = 20ms


def _iter_pcm16(pcm: bytes) -> Iterator[int]:
    if len(pcm) % 2 != 0:
        raise ValueError("PCM16 数据长度必须是偶数（16-bit 小端采样）")
    return (value for (value,) in struct.iter_unpack("<h", pcm))


def _pack_pcm16(samples: Iterable[int]) -> bytes:
    return b"".join(struct.pack("<h", max(-32768, min(32767, int(sample)))) for sample in samples)


# ── G.711 μ-law ──────────────────────────────────────────────────
_ULAW_BIAS = 0x84
_ULAW_CLIP = 32635


def pcmu_encode(sample: int) -> int:
    """单个 16-bit PCM 采样 → μ-law 字节。"""

    sign = 0x80 if sample < 0 else 0
    if sample < 0:
        sample = -sample
    sample = min(sample, _ULAW_CLIP) + _ULAW_BIAS

    exponent = 7
    mask = 0x4000
    while exponent > 0 and not (sample & mask):
        exponent -= 1
        mask >>= 1

    mantissa = (sample >> (exponent + 3)) & 0x0F
    return (~(sign | (exponent << 4) | mantissa)) & 0xFF


def pcmu_decode(byte: int) -> int:
    """μ-law 字节 → 单个 16-bit PCM 采样。"""

    value = (~byte) & 0xFF
    sign = value & 0x80
    exponent = (value >> 4) & 0x07
    mantissa = value & 0x0F
    sample = ((mantissa << 3) + _ULAW_BIAS) << exponent
    sample -= _ULAW_BIAS
    return -sample if sign else sample


# ── G.711 A-law ──────────────────────────────────────────────────
_ALAW_SEG_END = (0xFF, 0x1FF, 0x3FF, 0x7FF, 0xFFF, 0x1FFF, 0x3FFF, 0x7FFF)


def pcma_encode(sample: int) -> int:
    """单个 16-bit PCM 采样 → A-law 字节（G.711A）。"""

    if sample >= 0:
        mask = 0xD5
    else:
        mask = 0x55
        sample = -sample - 1

    if sample > 32767:
        sample = 32767

    segment = 8
    for index, end in enumerate(_ALAW_SEG_END):
        if sample <= end:
            segment = index
            break

    if segment >= 8:
        aval = 0x7F
    else:
        aval = segment << 4
        if segment < 2:
            aval |= (sample >> 4) & 0x0F
        else:
            aval |= (sample >> (segment + 3)) & 0x0F
    return aval ^ mask


def pcma_decode(byte: int) -> int:
    """A-law 字节 → 单个 16-bit PCM 采样。"""

    value = byte ^ 0x55
    sample = (value & 0x0F) << 4
    segment = (value & 0x70) >> 4
    if segment == 0:
        sample += 8
    elif segment == 1:
        sample += 0x108
    else:
        sample += 0x108
        sample <<= segment - 1
    return sample if (value & 0x80) else -sample


# ── 批量转换（编码后的 1 字节 = 1 采样）──────────────────────────
def pcmu_to_pcm16(data: bytes) -> bytes:
    """:func:`pcmu_decode` 的批量版本。"""

    return _pack_pcm16(pcmu_decode(byte) for byte in data)


def pcm16_to_pcmu(pcm: bytes) -> bytes:
    """:func:`pcmu_encode` 的批量版本。"""

    return bytes(pcmu_encode(sample) for sample in _iter_pcm16(pcm))


def pcma_to_pcm16(data: bytes) -> bytes:
    """:func:`pcma_decode` 的批量版本。"""

    return _pack_pcm16(pcma_decode(byte) for byte in data)


def pcm16_to_pcma(pcm: bytes) -> bytes:
    """:func:`pcma_encode` 的批量版本。"""

    return bytes(pcma_encode(sample) for sample in _iter_pcm16(pcm))


def pcmu_to_pcma(data: bytes) -> bytes:
    """μ-law → A-law（经 PCM16 中转）。"""

    return pcm16_to_pcma(pcmu_to_pcm16(data))


def pcma_to_pcmu(data: bytes) -> bytes:
    """A-law → μ-law（经 PCM16 中转）。"""

    return pcm16_to_pcmu(pcma_to_pcm16(data))


def frame_duration_ms(payload_length: int, samples_per_second: int = G711_SAMPLE_RATE) -> float:
    """按字节数估算 G.711 帧时长（调试/日志用）。"""

    if samples_per_second <= 0:
        raise ValueError("samples_per_second 必须大于 0")
    return payload_length * 1000.0 / samples_per_second


# ── 传输接口 ────────────────────────────────────────────────────
class AudioTransport(ABC):
    """音频传输接口：读设备上行 PCM、写 agent 下行 PCM。"""

    @abstractmethod
    def read_pcm(self) -> Optional[bytes]:
        """读取一帧 16-bit PCM（小端、8kHz、单声道）；无数据返回 ``None``。"""

    @abstractmethod
    def write_pcm(self, pcm: bytes) -> None:
        """写入一帧 16-bit PCM。"""

    @abstractmethod
    def close(self) -> None:
        """释放资源。"""


class ZLMTransport(AudioTransport):
    """ZLM 音频传输骨架。

    参数：
        zlm_base_url: ZLM HTTP API 基址，例如 ``http://zlm:80``。
        secret: ZLM API secret（用于 ``startSendRtp``）。
        stream_id: 设备流 ID（``{app}/{stream}``）。
        sample_rate: 固定 8000（G.711 窄带）。

    当前所有收发方法均抛 :class:`NotImplementedError`，原因见模块文档。
    帧转换函数已就绪，联调时只需补齐：

    1. 从 ZLM 拉 PCMA 帧（推荐 RTP/RTSP 或 FLV 音频轨）；
    2. ``pcma_to_pcm16`` 后交给 Pipecat；
    3. TTS 输出 ``pcm16_to_pcma``；
    4. 调 ZLM ``/index/api/startSendRtp``，目标地址/端口/SSRC 取自设备协商的 SDP。
    """

    def __init__(
        self,
        zlm_base_url: str,
        secret: str,
        stream_id: str,
        sample_rate: int = G711_SAMPLE_RATE,
        **_extra: object,
    ) -> None:
        if not stream_id.strip():
            raise ValueError("stream_id 不能为空")
        self.zlm_base_url = zlm_base_url.rstrip("/")
        self.secret = secret
        self.stream_id = stream_id
        self.sample_rate = sample_rate
        self.closed = False

    def read_pcm(self) -> Optional[bytes]:
        raise NotImplementedError(
            "ZLM 音频上行尚未接通：本期只实现 G.711↔PCM 转换，"
            "RTP/FLV 拉流需与 ZLM 联调（见 deploy/pipecat/README.md）"
        )

    def write_pcm(self, pcm: bytes) -> None:
        raise NotImplementedError(
            "ZLM startSendRtp 回推尚未接通：需要设备 SDP 的目标地址/端口/SSRC，"
            "本期不伪造发送成功"
        )

    def close(self) -> None:
        self.closed = True


__all__: Iterable[str] = (
    "AUDIO_BRIDGE_STATUS",
    "AudioTransport",
    "G711_SAMPLE_RATE",
    "G711_SAMPLES_PER_20MS",
    "ZLMTransport",
    "frame_duration_ms",
    "pcma_decode",
    "pcma_encode",
    "pcma_to_pcm16",
    "pcma_to_pcmu",
    "pcm16_to_pcma",
    "pcm16_to_pcmu",
    "pcmu_decode",
    "pcmu_encode",
    "pcmu_to_pcm16",
    "pcmu_to_pcma",
)
