"""PCM16 / Opus 编解码单测。

Opus 往返需要 PyAV（requirements 里已有）；本机未装时自动 skip。
"""

from __future__ import annotations

import math
import struct

import pytest

from audio.opus_codec import OpusCodec, opus_available
from ws.serializer import CodecError, OpusAudioCodec, Pcm16Codec, build_codec


def _tone(rate: int, samples: int, freq: float = 440.0) -> bytes:
    return b"".join(
        struct.pack("<h", int(12000 * math.sin(2 * math.pi * freq * i / rate)))
        for i in range(samples)
    )


def test_pcm16_passthrough_at_16k():
    codec = Pcm16Codec(input_rate=16000, output_rate=24000)
    pcm = _tone(16000, 320)
    assert codec.decode(pcm) == pcm
    assert codec.encode(pcm, 24000) == pcm


def test_pcm16_resamples_8k_to_16k():
    codec = Pcm16Codec(input_rate=8000, output_rate=24000)
    pcm = _tone(8000, 160)  # 20ms
    out = codec.decode(pcm)
    assert len(out) == 320 * 2  # 320 samples @16k


def test_pcm16_output_resample():
    codec = Pcm16Codec(input_rate=16000, output_rate=24000)
    pcm = _tone(16000, 320)  # 20ms @16k
    out = codec.encode(pcm, 16000)
    assert len(out) == 480 * 2  # 20ms @24k


def test_build_codec_pcm16():
    codec = build_codec("pcm16", input_rate=16000, output_rate=24000)
    assert isinstance(codec, Pcm16Codec)


def test_build_codec_unknown():
    with pytest.raises(CodecError):
        build_codec("mp3")


@pytest.mark.skipif(not opus_available(), reason="PyAV/libopus 不可用")
def test_opus_roundtrip():
    codec = OpusCodec(encode_rate=24000, decode_rate=16000)
    pcm = _tone(24000, 480)  # 20ms
    packet = codec.encode(pcm)
    assert packet, "20ms PCM 应该产出一个 Opus 包"

    decoded = codec.decode(packet)
    # libopus 有编码器延迟，第一包可能解出少量样本；关键是能解且长度合理。
    assert isinstance(decoded, bytes)
    assert len(decoded) % 2 == 0


@pytest.mark.skipif(not opus_available(), reason="PyAV/libopus 不可用")
def test_opus_adapter_roundtrip():
    codec = OpusAudioCodec(output_rate=24000)
    packet = codec.encode(_tone(24000, 480), 24000)
    assert packet
    assert codec.decode(packet) is not None


def test_opus_codec_handles_garbage():
    if not opus_available():
        pytest.skip("PyAV/libopus 不可用")
    codec = OpusCodec()
    # libopus 对非法包通常会做丢包补偿（PLC）而不是抛错；要求不崩溃即可。
    decoded = codec.decode(b"\x00\x01\x02")
    assert isinstance(decoded, bytes)
    assert len(decoded) % 2 == 0
