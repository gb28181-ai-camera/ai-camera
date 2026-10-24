"""设备语音的 RTP 收发与 G.711 ↔ PCM16 转换。

## 真实链路

```text
设备麦克风
  → ZLM 单端口 rtp/<SSRC 大写十六进制>（PCMA/8000）
  → ZLM /index/api/startSendRtp（上行，目标 = 本模块的 UDP 端口）
  → RTPAudioReceiver：去 RTP 头 → 序号重排 → PCMA → PCM16
  → Pipecat（VAD → STT → LLM → TTS）
  → PCM16 → WebRTCPCMAPusher（aiortc，强制 PCMA 编码）
  → ZLM /index/api/webrtc?type=push（stream = AI<设备流>）
  → ZLM /index/api/startSendRtp（下行，目标 = 设备 200 OK 的 m=audio 地址）
```

## 为什么下行选 WebRTC push（方案 A）

ZLM 的 `startSendRtp` 是「从 ZLM 内部已有流往外发」的模型，它**不接受**外部
直接写入的 UDP RTP 作为新流（单端口 RTP proxy 面向的是设备/GB28181 推流，
不是任意 RTP 发布端；多端口模式要 `openRtpServer`，本项目明确禁用）。
因此 Pipecat 必须先把 TTS 音频变成 ZLM 认识的一条流 —— 官方支持的入口就是
`/index/api/webrtc?type=push`。aiortc 负责把 PCM 编成 PCMA 轨道并完成
DTLS/ICE 握手；ZLM 收到后创建 `app/stream`，随后后端对它调用 `startSendRtp`。

> 方案 B（Pipecat 直接向 ZLM 的 10000/UDP 发 PCMA）没有采用：单端口 proxy
> 的流创建/编码探测面向设备侧的 PS/RTP，裸 PCMA 音频轨不是受支持的发布
> 路径，且失败时通常没有明确错误（表现为「流一直不存在」）。

## 状态语义

- `not_connected`：没有绑定 UDP/没有建立 WebRTC；
- `degraded`：只接通了一半（例如收到设备音频但下行推流失败，或反之）；
- `connected`：上行 UDP 正在收包 **且** 下行 WebRTC 已连接。

`audio_bridge` 是一个**实测状态**，不是「配置看起来对」的推断。
"""

from __future__ import annotations

import asyncio
import json
import logging
import queue
import socket
import struct
import threading
import time
import urllib.parse
import urllib.request
from abc import ABC, abstractmethod
from dataclasses import dataclass
from fractions import Fraction
from typing import Any, Dict, Iterable, Iterator, List, Optional, Tuple

logger = logging.getLogger("ai-camera-pipecat.transport")

AUDIO_BRIDGE_NOT_CONNECTED = "not_connected"
AUDIO_BRIDGE_DEGRADED = "degraded"
AUDIO_BRIDGE_CONNECTED = "connected"
# 向后兼容：旧代码只认识这一个常量。
AUDIO_BRIDGE_STATUS = AUDIO_BRIDGE_NOT_CONNECTED

# G.711 的采样率与帧长。
G711_SAMPLE_RATE = 8000
G711_SAMPLES_PER_20MS = G711_SAMPLE_RATE // 50  # 160 samples = 20ms
PCM16_BYTES_PER_20MS = G711_SAMPLES_PER_20MS * 2  # 320 bytes
PCMA_BYTES_PER_20MS = G711_SAMPLES_PER_20MS  # 160 bytes

# A-law 的静音字节。设备侧通常把 0xD5 当静音，连续发它可以保持
# RTP 会话存活而不产生噪声。
PCMA_SILENCE = 0xD5


# ════════════════════════════════════════════════════════════════
# G.711
# ════════════════════════════════════════════════════════════════


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


# 查表加速：8kHz 单声道每秒 8000 次转换，纯循环在 Python 里调用量不小。
# 表在导入时构建一次；函数仍然保留，方便单测与调试。
_PCMA_ENCODE_TABLE = bytes(pcma_encode(v) for v in range(-32768, 32768))
_PCMA_DECODE_TABLE = tuple(struct.pack("<h", pcma_decode(b)) for b in range(256))


def pcmu_to_pcm16(data: bytes) -> bytes:
    """:func:`pcmu_decode` 的批量版本。"""

    return _pack_pcm16(pcmu_decode(byte) for byte in data)


def pcm16_to_pcmu(pcm: bytes) -> bytes:
    """:func:`pcmu_encode` 的批量版本。"""

    return bytes(pcmu_encode(sample) for sample in _iter_pcm16(pcm))


def pcma_to_pcm16(data: bytes) -> bytes:
    """A-law → 16-bit PCM（小端、8000Hz、单声道）。"""

    return b"".join(_PCMA_DECODE_TABLE[byte] for byte in data)


def pcm16_to_pcma(pcm: bytes) -> bytes:
    """16-bit PCM → A-law。奇数字节直接报错，避免静默丢半个采样。"""

    if len(pcm) % 2 != 0:
        raise ValueError("PCM16 数据长度必须是偶数（16-bit 小端采样）")
    return bytes(_PCMA_ENCODE_TABLE[value + 32768] for (value,) in struct.iter_unpack("<h", pcm))


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


# ════════════════════════════════════════════════════════════════
# RTP
# ════════════════════════════════════════════════════════════════


class RTPParseError(ValueError):
    """RTP 报文非法（截断、版本错误、padding 越界等）。"""


@dataclass(frozen=True)
class RTPPacket:
    """RTP 固定头 + 载荷。CSRC/扩展头解析时保留原文，便于回环测试。"""

    version: int = 2
    padding: bool = False
    extension: bool = False
    marker: bool = False
    payload_type: int = 8
    sequence_number: int = 0
    timestamp: int = 0
    ssrc: int = 0
    payload: bytes = b""
    csrc: Tuple[int, ...] = ()
    extension_data: bytes = b""
    padding_size: int = 0

    def marshal(self) -> bytes:
        if len(self.csrc) > 15:
            raise RTPParseError("CSRC 数量超过 RTP 头 4 bit 上限")
        size = 12 + len(self.csrc) * 4 + len(self.extension_data) + len(self.payload) + self.padding_size
        out = bytearray(size)
        out[0] = (2 << 6)
        if self.padding:
            out[0] |= 0x20
        if self.extension:
            out[0] |= 0x10
        out[0] |= len(self.csrc)
        out[1] = self.payload_type & 0x7F
        if self.marker:
            out[1] |= 0x80
        struct.pack_into(">HII", out, 2, self.sequence_number & 0xFFFF,
                         self.timestamp & 0xFFFFFFFF, self.ssrc & 0xFFFFFFFF)
        offset = 12
        for csrc in self.csrc:
            struct.pack_into(">I", out, offset, csrc & 0xFFFFFFFF)
            offset += 4
        if self.extension_data:
            out[offset:offset + len(self.extension_data)] = self.extension_data
            offset += len(self.extension_data)
        out[offset:offset + len(self.payload)] = self.payload
        if self.padding_size:
            out[size - 1] = self.padding_size
        return bytes(out)


def parse_rtp(data: bytes) -> RTPPacket:
    """解析 RTP；非法报文抛 :class:`RTPParseError`。

    所有长度字段都先与实际报文长度比较，避免损坏报文触发越界。
    """

    if len(data) < 12:
        raise RTPParseError("RTP 报文长度不足 12 字节")
    version = data[0] >> 6
    if version != 2:
        raise RTPParseError(f"RTP 版本号不是 2（实际 {version}）")

    padding = bool(data[0] & 0x20)
    extension = bool(data[0] & 0x10)
    csrc_count = data[0] & 0x0F
    offset = 12

    if csrc_count:
        need = offset + csrc_count * 4
        if len(data) < need:
            raise RTPParseError("CSRC 声明长度超出报文")
        csrc = tuple(struct.unpack_from(">I", data, offset + i * 4)[0] for i in range(csrc_count))
        offset = need
    else:
        csrc = ()

    extension_data = b""
    if extension:
        if len(data) < offset + 4:
            raise RTPParseError("扩展头声明长度超出报文")
        ext_words = struct.unpack_from(">H", data, offset + 2)[0]
        ext_len = 4 + ext_words * 4
        if len(data) < offset + ext_len:
            raise RTPParseError("扩展头数据超出报文")
        extension_data = data[offset:offset + ext_len]
        offset += ext_len

    end = len(data)
    padding_size = 0
    if padding:
        padding_size = data[end - 1]
        if padding_size == 0 or padding_size > end - offset:
            raise RTPParseError("RTP padding 计数非法")
        end -= padding_size

    payload = bytes(data[offset:end])
    marker = bool(data[1] & 0x80)
    payload_type = data[1] & 0x7F
    seq, timestamp, ssrc = struct.unpack_from(">HII", data, 2)
    return RTPPacket(
        version=version,
        padding=padding,
        extension=extension,
        marker=marker,
        payload_type=payload_type,
        sequence_number=seq,
        timestamp=timestamp,
        ssrc=ssrc,
        payload=payload,
        csrc=csrc,
        extension_data=extension_data,
        padding_size=padding_size,
    )


def build_rtp(sequence_number: int, timestamp: int, ssrc: int, payload: bytes,
              payload_type: int = 8, marker: bool = False) -> bytes:
    """封装一个最小 RTP 包（无 CSRC/扩展/padding）。"""

    header = bytearray(12)
    header[0] = 2 << 6
    header[1] = payload_type & 0x7F
    if marker:
        header[1] |= 0x80
    struct.pack_into(">HII", header, 2, sequence_number & 0xFFFF,
                     timestamp & 0xFFFFFFFF, ssrc & 0xFFFFFFFF)
    return bytes(header) + payload


class SequenceBuffer:
    """按 RTP 序号做乱序重排，并容忍丢包。

    与 Go 侧 internal/gb28181/audio/sequence.go 同一策略：
    迟到的包丢弃（实时语音不做补发）；超过 ``capacity`` 的缺口强制推进，
    跳过的序号计入 ``lost``。序号 16 位会回绕，因此用有符号差值比较。
    """

    def __init__(self, capacity: int = 32) -> None:
        self.capacity = max(1, int(capacity))
        self._pending: Dict[int, RTPPacket] = {}
        self._next = 0
        self._started = False
        self.packets = 0
        self.duplicates = 0
        self.reordered = 0
        self.lost = 0

    @staticmethod
    def _delta(seq: int, base: int) -> int:
        return ((seq - base + 32768) & 0xFFFF) - 32768

    def push(self, packet: RTPPacket) -> List[RTPPacket]:
        if packet is None:
            return []
        self.packets += 1

        if not self._started:
            self._started = True
            self._next = packet.sequence_number
            out = [packet]
            self._next = (self._next + 1) & 0xFFFF
            out.extend(self._drain())
            return out

        delta = self._delta(packet.sequence_number, self._next)
        if delta < 0:
            self.duplicates += 1
            return []
        if delta == 0:
            out = [packet]
            self._next = (self._next + 1) & 0xFFFF
            out.extend(self._drain())
            return out

        if packet.sequence_number in self._pending:
            self.duplicates += 1
            return []
        self._pending[packet.sequence_number] = packet
        self.reordered += 1

        out: List[RTPPacket] = []
        while len(self._pending) > self.capacity:
            seq = self._smallest_pending()
            gap = (seq - self._next) & 0xFFFF
            if gap:
                self.lost += gap
            out.append(self._pending.pop(seq))
            self._next = (seq + 1) & 0xFFFF
            out.extend(self._drain())
        return out

    def flush(self) -> List[RTPPacket]:
        if not self._pending:
            return []
        seqs = sorted(self._pending, key=lambda seq: self._delta(seq, self._next))
        out: List[RTPPacket] = []
        for seq in seqs:
            gap = (seq - self._next) & 0xFFFF
            if gap:
                self.lost += gap
            out.append(self._pending.pop(seq))
            self._next = (seq + 1) & 0xFFFF
        return out

    def _drain(self) -> List[RTPPacket]:
        out: List[RTPPacket] = []
        while self._next in self._pending:
            out.append(self._pending.pop(self._next))
            self._next = (self._next + 1) & 0xFFFF
        return out

    def _smallest_pending(self) -> int:
        return min(self._pending, key=lambda seq: self._delta(seq, self._next))

    @property
    def pending(self) -> int:
        return len(self._pending)

    def stats(self) -> Dict[str, int]:
        return {
            "packets": self.packets,
            "duplicates": self.duplicates,
            "reordered": self.reordered,
            "lost": self.lost,
            "pending": len(self._pending),
        }


# ════════════════════════════════════════════════════════════════
# 重采样
# ════════════════════════════════════════════════════════════════


def resample_pcm16(pcm: bytes, src_rate: int, dst_rate: int) -> bytes:
    """线性插值重采样（纯 Python，无 numpy 依赖）。

    用途：STT 常要求 16kHz，而设备链路是 8kHz；TTS 常输出 24kHz，
    设备又只接受 8kHz PCMA。线性插值对语音足够，且实现可单测。
    """

    if src_rate <= 0 or dst_rate <= 0:
        raise ValueError("采样率必须大于 0")
    if src_rate == dst_rate:
        return pcm
    if len(pcm) % 2 != 0:
        raise ValueError("PCM16 数据长度必须是偶数")

    samples = list(_iter_pcm16(pcm))
    if not samples:
        return b""
    out_len = max(1, int(round(len(samples) * dst_rate / src_rate)))
    ratio = src_rate / dst_rate
    out: List[int] = []
    last = len(samples) - 1
    for i in range(out_len):
        pos = i * ratio
        idx = int(pos)
        frac = pos - idx
        a = samples[min(idx, last)]
        b = samples[min(idx + 1, last)]
        out.append(int(round(a + (b - a) * frac)))
    return struct.pack(f"<{len(out)}h", *out)


# ════════════════════════════════════════════════════════════════
# 传输接口
# ════════════════════════════════════════════════════════════════


class AudioTransport(ABC):
    """音频传输接口：读设备上行 PCM、写 agent 下行 PCM。"""

    @abstractmethod
    def read_pcm(self, timeout: float = 0.02) -> Optional[bytes]:
        """读取一帧 16-bit PCM（小端、8kHz、单声道）；无数据返回 ``None``。"""

    @abstractmethod
    def write_pcm(self, pcm: bytes) -> None:
        """写入一帧 16-bit PCM（小端、8kHz、单声道）。"""

    @abstractmethod
    def close(self) -> None:
        """释放资源。"""


class RTPAudioReceiver(AudioTransport):
    """监听 UDP，把 ZLM 推来的 PCMA RTP 还原成 PCM16。

    参数全部由会话参数传入（设计 §H+）：端口、PT、SSRC 都不写死，
    这样同一进程可以同时服务多路设备/多个媒体节点。
    """

    def __init__(
        self,
        port: int = 0,
        host: str = "0.0.0.0",
        payload_type: int = 8,
        clock_rate: int = G711_SAMPLE_RATE,
        jitter_capacity: int = 32,
        expected_ssrc: Optional[int] = None,
        queue_max: int = 100,
        degraded_after_sec: float = 3.0,
    ) -> None:
        self.host = host
        self.port = int(port or 0)
        self.payload_type = int(payload_type)
        self.clock_rate = int(clock_rate)
        self.expected_ssrc = expected_ssrc
        self.degraded_after_sec = degraded_after_sec

        self._sock: Optional[socket.socket] = None
        self._thread: Optional[threading.Thread] = None
        self._stop = threading.Event()
        self._queue: "queue.Queue[bytes]" = queue.Queue(maxsize=max(1, int(queue_max)))
        self._jitter = SequenceBuffer(jitter_capacity)
        self._lock = threading.Lock()

        self._last_packet_at = 0.0
        self._last_flush_at = 0.0
        self._packets = 0
        self._bytes = 0
        self._ssrc: Optional[int] = None
        self._muted_until = 0.0
        self.started = False
        self.closed = False

    # ── 生命周期 ────────────────────────────────────────────────
    def start(self, port: Optional[int] = None) -> int:
        """绑定 UDP 并启动接收线程，返回实际端口（``port=0`` 时由系统分配）。"""

        if self.started and not self.closed:
            return self.port
        self.port = int(port if port is not None else self.port) or 0

        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        sock.settimeout(0.2)
        try:
            sock.bind((self.host, self.port))
        except OSError:
            sock.close()
            raise
        self.port = sock.getsockname()[1]
        self._sock = sock
        self.started = True
        self.closed = False
        self._stop.clear()

        self._thread = threading.Thread(
            target=self._receive_loop,
            name=f"rtp-audio-{self.port}",
            daemon=True,
        )
        self._thread.start()
        logger.info("RTP 上行已监听 udp://%s:%d（PT=%d）", self.host, self.port, self.payload_type)
        return self.port

    def close(self) -> None:
        self._stop.set()
        self.closed = True
        if self._sock is not None:
            try:
                self._sock.close()
            except OSError:
                pass
            self._sock = None
        if self._thread is not None and self._thread.is_alive():
            self._thread.join(timeout=1.0)
        self._thread = None
        logger.info("RTP 上行已关闭 port=%d", self.port)

    # ── 接收线程 ────────────────────────────────────────────────
    def _receive_loop(self) -> None:
        sock = self._sock
        if sock is None:
            return
        while not self._stop.is_set():
            try:
                data, _addr = sock.recvfrom(4096)
            except socket.timeout:
                # 长时间没有新包时把 pending 吐出：最后一个包的缺口不能永远等。
                self._flush_stale()
                continue
            except OSError:
                if self._stop.is_set():
                    return
                continue

            try:
                packet = parse_rtp(data)
            except RTPParseError as exc:
                logger.debug("丢弃非法 RTP：%s", exc)
                continue

            if packet.payload_type != self.payload_type:
                continue
            # 单端口模式下同一端口可能混入别的 SSRC；显式配置了期望值就过滤。
            if self.expected_ssrc is not None and packet.ssrc != self.expected_ssrc:
                continue

            with self._lock:
                self._ssrc = packet.ssrc
                self._packets += 1
                self._bytes += len(packet.payload)
                self._last_packet_at = time.monotonic()

            for ordered in self._jitter.push(packet):
                self._enqueue_payload(ordered.payload)

    def _flush_stale(self) -> None:
        now = time.monotonic()
        with self._lock:
            idle = now - self._last_packet_at
        if self._jitter.pending == 0:
            return
        # 200ms 没有新包才 flush，避免把正常乱序误判成断流。
        if idle < 0.2 or now - self._last_flush_at < 0.2:
            return
        self._last_flush_at = now
        for ordered in self._jitter.flush():
            self._enqueue_payload(ordered.payload)

    def _enqueue_payload(self, payload: bytes) -> None:
        # 按 20ms（160 字节）切帧，保证下游拿到固定节奏。
        for start in range(0, len(payload), PCMA_BYTES_PER_20MS):
            chunk = payload[start:start + PCMA_BYTES_PER_20MS]
            if not chunk:
                continue
            pcm = pcma_to_pcm16(chunk)
            try:
                self._queue.put_nowait(pcm)
            except queue.Full:
                # 下游（STT）处理不过来时丢最旧的帧，绝不无限堆积。
                try:
                    self._queue.get_nowait()
                except queue.Empty:
                    pass
                try:
                    self._queue.put_nowait(pcm)
                except queue.Full:
                    pass

    # ── 半双工 ──────────────────────────────────────────────────
    def set_uplink_muted(self, muted: bool, duration: Optional[float] = None) -> None:
        """静音上行（设计 §8.1.1 方案①）。

        agent 说话期间丢弃设备上行，避免「设备喇叭 → 设备麦克风 → STT」
        自激。``duration`` 是保持静音的秒数，到期自动恢复。
        """

        now = time.monotonic()
        if muted:
            until = now + float(duration or 0.0)
            self._muted_until = max(self._muted_until, until)
        else:
            self._muted_until = 0.0

    @property
    def uplink_muted(self) -> bool:
        return time.monotonic() < self._muted_until

    # ── 读取 ────────────────────────────────────────────────────
    def read_pcm(self, timeout: float = 0.02) -> Optional[bytes]:
        if not self.started or self.closed:
            return None
        if self.uplink_muted:
            # 静音期间仍消费队列，否则恢复后会一次性吐出陈旧音频。
            try:
                self._queue.get_nowait()
            except queue.Empty:
                pass
            return None
        try:
            return self._queue.get(timeout=max(0.0, timeout))
        except queue.Empty:
            return None

    def write_pcm(self, pcm: bytes) -> None:
        raise NotImplementedError("RTPAudioReceiver 只负责上行；下行请用 WebRTCPCMAPusher")

    # ── 状态 ────────────────────────────────────────────────────
    def status(self) -> str:
        if not self.started or self.closed:
            return AUDIO_BRIDGE_NOT_CONNECTED
        with self._lock:
            packets = self._packets
            last = self._last_packet_at
        if packets == 0:
            return AUDIO_BRIDGE_DEGRADED
        if time.monotonic() - last > self.degraded_after_sec:
            return AUDIO_BRIDGE_DEGRADED
        return AUDIO_BRIDGE_CONNECTED

    def stats(self) -> Dict[str, Any]:
        with self._lock:
            packets = self._packets
            payload_bytes = self._bytes
            last = self._last_packet_at
            ssrc = self._ssrc
        data = self._jitter.stats()
        data.update({
            "port": self.port,
            "ssrc": ssrc,
            "rtp_packets": packets,
            "payload_bytes": payload_bytes,
            "queue": self._queue.qsize(),
            "uplink_muted": self.uplink_muted,
            "last_packet_age_ms": round((time.monotonic() - last) * 1000, 1) if last else None,
        })
        return data


class WebRTCPCMAPusher(AudioTransport):
    """把 8kHz PCM16 通过 aiortc 推送到 ZLM WebRTC push 入口。

    关键点：轨道里放的是 **PCM16**，不是 PCMA 字节。aiortc 的 RTP 层会按
    SDP 协商出的编码（本类会优先只提供 PCMA）把 PCM 编成 G.711A。若把
    A-law 字节直接塞进 ``AudioFrame(format="s16")``，设备听到的会是噪声。

    aiortc 是可选依赖：没装时 ``start()`` 返回 False 并把状态标为
    ``degraded``，上行接收仍然可用，但设备喇叭不会有声音 —— 这比
    「假装推流成功」可诊断得多。
    """

    def __init__(
        self,
        zlm_base_url: str,
        secret: str,
        app: str,
        stream: str,
        ssrc: int = 0,
        sample_rate: int = G711_SAMPLE_RATE,
        connect_timeout: float = 5.0,
    ) -> None:
        if not stream.strip():
            raise ValueError("stream 不能为空")
        self.zlm_base_url = zlm_base_url.rstrip("/")
        self.secret = secret or ""
        self.app = app or "rtp"
        self.stream = stream.strip()
        self.ssrc = int(ssrc or 0)
        self.sample_rate = int(sample_rate)
        self.connect_timeout = float(connect_timeout)

        # 500 帧 = 10 秒。TTS 往往一次返回整段音频（Edge TTS 尤其如此），
        # 队列太小会把前半句丢掉；10 秒已覆盖单次回复的常见长度。
        self._pcm_queue: "queue.Queue[Optional[bytes]]" = queue.Queue(maxsize=500)
        self._pcm_residual = bytearray()
        self._write_lock = threading.Lock()
        self._thread: Optional[threading.Thread] = None
        self._loop: Optional[asyncio.AbstractEventLoop] = None
        self._pc: Any = None
        self._track: Any = None
        self._ready = threading.Event()
        self._stop = threading.Event()
        self._peer_state = "new"
        self._error = ""
        self.started = False
        self.closed = False
        self._sent_bytes = 0

    # ── 生命周期 ────────────────────────────────────────────────
    def start(self) -> bool:
        """启动推流线程并等待 WebRTC 连接；成功返回 True。"""

        if self.started:
            return self._peer_state in {"connected", "completed"}
        self.started = True
        self.closed = False
        self._stop.clear()
        self._thread = threading.Thread(target=self._thread_main, name=f"webrtc-push-{self.stream}", daemon=True)
        self._thread.start()

        deadline = time.monotonic() + self.connect_timeout
        while time.monotonic() < deadline:
            if self._peer_state in {"connected", "completed"}:
                logger.info("WebRTC 下行已连接 stream=%s", self.stream)
                return True
            if self._peer_state in {"failed", "closed"}:
                logger.warning("WebRTC 下行连接失败 stream=%s：%s", self.stream, self._error or self._peer_state)
                return False
            time.sleep(0.05)

        logger.warning("WebRTC 下行连接超时 stream=%s（%.1fs）", self.stream, self.connect_timeout)
        return False

    def close(self) -> None:
        self._stop.set()
        try:
            self._pcm_queue.put_nowait(None)
        except queue.Full:
            pass
        loop = self._loop
        if loop is not None and loop.is_running():
            # 先在事件循环里关 pc/track，再停 loop；直接 stop 会让
            # finally 里的 run_until_complete 抛 "loop stopped" 而漏关连接。
            try:
                future = asyncio.run_coroutine_threadsafe(self._async_close(), loop)
                future.result(timeout=2.0)
            except Exception:
                pass
            try:
                loop.call_soon_threadsafe(loop.stop)
            except RuntimeError:
                pass
        if self._thread is not None and self._thread.is_alive():
            self._thread.join(timeout=2.0)
        self._thread = None
        self._pc = None
        self._track = None
        self.closed = True

    # ── 写入 ────────────────────────────────────────────────────
    def write_pcm(self, pcm: bytes) -> None:
        """写入 8kHz PCM16，按 20ms（320 字节）切帧后排队。"""

        if not self.started or self.closed or len(pcm) == 0:
            return
        if len(pcm) % 2 != 0:
            # TTS 分片可能落在半个采样上；丢掉尾部 1 字节，下一帧会接上。
            pcm = pcm[:-1]
        if not pcm:
            return

        with self._write_lock:
            self._pcm_residual.extend(pcm)
            self._sent_bytes += len(pcm)
            while len(self._pcm_residual) >= PCM16_BYTES_PER_20MS:
                chunk = bytes(self._pcm_residual[:PCM16_BYTES_PER_20MS])
                del self._pcm_residual[:PCM16_BYTES_PER_20MS]
                # 上限 400 帧（8 秒）；只有严重积压（下游长时间不取）才丢旧包。
                while self._pcm_queue.qsize() > 400:
                    try:
                        self._pcm_queue.get_nowait()
                    except queue.Empty:
                        break
                try:
                    self._pcm_queue.put_nowait(chunk)
                except queue.Full:
                    pass

    def read_pcm(self, timeout: float = 0.02) -> Optional[bytes]:
        return None

    # ── 线程与异步 ──────────────────────────────────────────────
    def _thread_main(self) -> None:
        loop = asyncio.new_event_loop()
        self._loop = loop
        asyncio.set_event_loop(loop)
        try:
            loop.run_until_complete(self._async_start())
            if not self._stop.is_set():
                loop.run_forever()
        except Exception as exc:  # 依赖缺失/握手失败：明确记录，不吞掉
            if not self._stop.is_set():
                self._error = f"{type(exc).__name__}: {exc}"
                self._peer_state = "failed"
                logger.warning("WebRTC 下行启动失败：%s", self._error, exc_info=True)
        finally:
            try:
                if self._pc is not None and not loop.is_closed():
                    loop.run_until_complete(self._async_close())
            except Exception:
                pass
            pending = asyncio.all_tasks(loop)
            for task in pending:
                task.cancel()
            if pending:
                loop.run_until_complete(asyncio.gather(*pending, return_exceptions=True))
            loop.close()
            self._loop = None

    async def _async_start(self) -> None:
        try:
            from aiortc import RTCPeerConnection, RTCSessionDescription
        except Exception as exc:  # ModuleNotFoundError / 原生依赖加载失败
            raise RuntimeError(f"aiortc 不可用（requirements.txt 需安装 aiortc）：{exc}") from exc

        pc = RTCPeerConnection()
        self._pc = pc

        @pc.on("connectionstatechange")
        def _on_state() -> None:
            self._peer_state = pc.connectionState
            logger.info("WebRTC 下行状态 stream=%s state=%s", self.stream, self._peer_state)

        track = self._make_track()
        self._track = track
        sender = pc.addTrack(track)
        self._prefer_pcma(sender)

        offer = await pc.createOffer()
        await pc.setLocalDescription(offer)
        answer_sdp = await asyncio.to_thread(self._post_offer, pc.localDescription.sdp)
        await pc.setRemoteDescription(RTCSessionDescription(sdp=answer_sdp, type="answer"))
        self._ready.set()

        # 等待 DTLS/ICE 完成；start() 在另一个线程轮询 _peer_state。
        deadline = self._loop.time() + self.connect_timeout if self._loop else self.connect_timeout
        while not self._stop.is_set():
            if self._peer_state in {"connected", "completed", "failed", "closed"}:
                return
            await asyncio.sleep(0.05)
            if self._loop and self._loop.time() > deadline:
                return

    async def _async_close(self) -> None:
        pc = self._pc
        track = self._track
        if track is not None:
            try:
                track.stop()
            except Exception:
                pass
        if pc is not None:
            try:
                await pc.close()
            except Exception:
                pass

    @staticmethod
    def _prefer_pcma(sender: Any) -> None:
        """只提供 PCMA 编码，确保 ZLM 侧流就是 G.711A。

        为什么必须限制：ZLM 的 WebRTC push 默认会和 Opus 协商；如果流是
        Opus，后端仍然用 ``pt=8`` 调 startSendRtp，设备收到的就是「标成
        PCMA 的 Opus」—— 表现为刺耳噪声。aiortc 的 setCodecPreferences
        允许在 createOffer 前收窄编码列表。
        """

        try:
            from aiortc.rtcrtpsender import RTCRtpSender  # type: ignore

            capabilities = RTCRtpSender.getCapabilities("audio")
            pcma = [codec for codec in capabilities.codecs if codec.name.upper() == "PCMA"]
            if not pcma:
                logger.warning("aiortc 没有 PCMA 编码能力，ZLM 侧可能协商成 Opus")
                return
            sender.setCodecPreferences(pcma)
            logger.info("WebRTC 音频编码已限定为 PCMA")
        except Exception as exc:  # 旧版 aiortc 没有该 API
            logger.warning("无法限定 WebRTC 编码为 PCMA（%s），依赖默认协商", exc)

    def _make_track(self) -> Any:
        from aiortc import MediaStreamTrack
        from av import AudioFrame

        pusher = self

        class _AudioTrack(MediaStreamTrack):
            kind = "audio"

            def __init__(self) -> None:
                super().__init__()
                self._timestamp = 0
                self._deadline = 0.0

            async def recv(self) -> AudioFrame:
                loop = asyncio.get_running_loop()
                if self._deadline == 0.0:
                    self._deadline = loop.time()
                delay = self._deadline - loop.time()
                if delay > 0:
                    await asyncio.sleep(delay)

                try:
                    pcm = await asyncio.wait_for(
                        asyncio.to_thread(pusher._pcm_queue.get, True, 0.02),
                        timeout=0.04,
                    )
                except (asyncio.TimeoutError, queue.Empty):
                    pcm = None
                if pcm is None:
                    # 无 TTS 时持续发 PCM 静音，保持 ZLM 流存活（startSendRtp
                    # 依赖流存在；停发会让流在 rtp_proxy.timeoutSec 后消失）。
                    pcm = b"\x00" * PCM16_BYTES_PER_20MS

                frame = AudioFrame(format="s16", layout="mono", samples=G711_SAMPLES_PER_20MS)
                frame.sample_rate = pusher.sample_rate
                frame.pts = self._timestamp
                frame.time_base = Fraction(1, pusher.sample_rate)
                frame.planes[0].update(bytes(pcm))
                self._timestamp += G711_SAMPLES_PER_20MS

                self._deadline += 0.02
                now = loop.time()
                if self._deadline < now - 0.5:
                    # 长时间没有取帧（例如订阅者慢），不要追赶积压。
                    self._deadline = now + 0.02
                return frame

        return _AudioTrack()

    def _post_offer(self, sdp: str) -> str:
        query = urllib.parse.urlencode({
            "app": self.app,
            "stream": self.stream,
            "type": "push",
        })
        if self.secret:
            # 部分 ZLM 版本要求 push 带 secret；带上没有副作用。
            query += "&" + urllib.parse.urlencode({"secret": self.secret})
        url = f"{self.zlm_base_url}/index/api/webrtc?{query}"
        request = urllib.request.Request(
            url,
            data=sdp.encode("utf-8"),
            headers={"Content-Type": "application/sdp"},
            method="POST",
        )
        with urllib.request.urlopen(request, timeout=self.connect_timeout) as response:
            body = response.read().decode("utf-8", "replace")

        text = body.strip()
        if text.startswith("{"):
            try:
                payload = json.loads(text)
            except json.JSONDecodeError as exc:
                raise RuntimeError(f"ZLM WebRTC push 返回非法 JSON：{text[:200]}") from exc
            if int(payload.get("code", 0)) != 0:
                raise RuntimeError(f"ZLM WebRTC push 失败：code={payload.get('code')} msg={payload.get('msg')}")
            answer = payload.get("sdp") or ""
            if not answer:
                raise RuntimeError("ZLM WebRTC push 响应缺少 sdp 字段")
            return answer
        if not text:
            raise RuntimeError("ZLM WebRTC push 返回空响应")
        return text

    # ── 状态 ────────────────────────────────────────────────────
    def status(self) -> str:
        if not self.started or self.closed:
            return AUDIO_BRIDGE_NOT_CONNECTED
        if self._peer_state in {"connected", "completed"}:
            return AUDIO_BRIDGE_CONNECTED
        return AUDIO_BRIDGE_DEGRADED

    def stats(self) -> Dict[str, Any]:
        return {
            "stream": self.stream,
            "app": self.app,
            "ssrc": self.ssrc,
            "peer_state": self._peer_state,
            "sent_pcm_bytes": self._sent_bytes,
            "queued": self._pcm_queue.qsize(),
            "error": self._error or None,
        }


class ZLMTransport(AudioTransport):
    """兼容旧接口的 ZLM 组合传输。

    新代码直接用 :class:`RTPAudioReceiver` + :class:`WebRTCPCMAPusher`；
    保留这个类是为了不破坏已有调用方与文档示例。
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
        self.stream_id = stream_id.strip()
        self.sample_rate = sample_rate

        app = str(_extra.get("app") or "")
        stream = self.stream_id
        if "/" in stream:
            app, stream = stream.split("/", 1)
        app = app or "rtp"
        ssrc = int(_extra.get("ssrc") or 0)
        uplink_port = int(_extra.get("uplink_port") or 0)

        self.receiver = RTPAudioReceiver(port=uplink_port)
        self.sender = WebRTCPCMAPusher(
            zlm_base_url=self.zlm_base_url,
            secret=self.secret,
            app=app,
            stream=stream,
            ssrc=ssrc,
            sample_rate=sample_rate,
        )
        self.started = False

    def start(self, port: Optional[int] = None) -> int:
        actual = self.receiver.start(port)
        self.started = True
        self.sender.start()
        return actual

    def read_pcm(self, timeout: float = 0.02) -> Optional[bytes]:
        return self.receiver.read_pcm(timeout)

    def write_pcm(self, pcm: bytes) -> None:
        self.sender.write_pcm(pcm)

    def close(self) -> None:
        self.sender.close()
        self.receiver.close()
        self.started = False

    def status(self) -> str:
        up = self.receiver.status()
        down = self.sender.status()
        if up == AUDIO_BRIDGE_CONNECTED and down == AUDIO_BRIDGE_CONNECTED:
            return AUDIO_BRIDGE_CONNECTED
        if up == AUDIO_BRIDGE_NOT_CONNECTED and down == AUDIO_BRIDGE_NOT_CONNECTED:
            return AUDIO_BRIDGE_NOT_CONNECTED
        return AUDIO_BRIDGE_DEGRADED


def combine_bridge_status(uplink: str, downlink: str) -> str:
    """合并上行/下行状态：两边都 connected 才算闭环。"""

    if uplink == AUDIO_BRIDGE_CONNECTED and downlink == AUDIO_BRIDGE_CONNECTED:
        return AUDIO_BRIDGE_CONNECTED
    if uplink == AUDIO_BRIDGE_NOT_CONNECTED and downlink == AUDIO_BRIDGE_NOT_CONNECTED:
        return AUDIO_BRIDGE_NOT_CONNECTED
    return AUDIO_BRIDGE_DEGRADED


# ════════════════════════════════════════════════════════════════
# 自测（无第三方依赖）
# ════════════════════════════════════════════════════════════════


def _self_test() -> None:
    """``python transport.py`` 直接运行的最小回归（不依赖 aiortc/pipecat）。"""

    # 1) G.711 字节往返稳定。
    for value in range(256):
        pcm = pcma_to_pcm16(bytes([value]))
        assert pcm16_to_pcma(pcm)[0] == value, f"A-law 往返不稳定: {value:#04x}"

    # 2) RTP 封装/解析。
    packet = parse_rtp(build_rtp(7, 1234, 0x0BEBC201, b"\xd5\x55", marker=True))
    assert packet.sequence_number == 7 and packet.ssrc == 0x0BEBC201
    assert packet.payload == b"\xd5\x55" and packet.marker

    # 3) 乱序：101 先于 102，输出 101,102。
    buf = SequenceBuffer(capacity=8)
    first = RTPPacket(sequence_number=100)
    second = RTPPacket(sequence_number=102)
    third = RTPPacket(sequence_number=101)
    assert [p.sequence_number for p in buf.push(first)] == [100]
    assert buf.push(second) == []
    assert [p.sequence_number for p in buf.push(third)] == [101, 102]

    # 4) 重采样长度合理。
    pcm = b"\x00\x00" * 160
    assert len(resample_pcm16(pcm, 8000, 16000)) == 640
    assert len(resample_pcm16(pcm, 24000, 8000)) == 106

    # 5) 下行写入必须按 20ms(320 字节 PCM16) 切帧；把整段 TTS 当成一帧
    #    会让 aiortc 的 AudioFrame 缓冲区越界。
    pusher = WebRTCPCMAPusher("http://127.0.0.1:1", "", "rtp", "AI0BEBC201")
    pusher.started = True
    pusher.write_pcm(b"\x00" * 640)
    chunks = [pusher._pcm_queue.get_nowait() for _ in range(2)]
    assert all(len(chunk) == PCM16_BYTES_PER_20MS for chunk in chunks), [len(c) for c in chunks]

    print("transport self-test OK")


if __name__ == "__main__":
    _self_test()


__all__: Iterable[str] = (
    "AUDIO_BRIDGE_CONNECTED",
    "AUDIO_BRIDGE_DEGRADED",
    "AUDIO_BRIDGE_NOT_CONNECTED",
    "AUDIO_BRIDGE_STATUS",
    "AudioTransport",
    "G711_SAMPLE_RATE",
    "G711_SAMPLES_PER_20MS",
    "PCM16_BYTES_PER_20MS",
    "PCMA_BYTES_PER_20MS",
    "PCMA_SILENCE",
    "RTPAudioReceiver",
    "RTPPacket",
    "RTPParseError",
    "SequenceBuffer",
    "WebRTCPCMAPusher",
    "ZLMTransport",
    "build_rtp",
    "combine_bridge_status",
    "frame_duration_ms",
    "parse_rtp",
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
    "resample_pcm16",
)
