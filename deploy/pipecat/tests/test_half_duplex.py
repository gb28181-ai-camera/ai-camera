"""半双工闸门：播放期间丢弃上行 + 300ms 尾巴 + abort。"""

from __future__ import annotations

from ws.serializer import HalfDuplexGate


def test_requires_listening():
    gate = HalfDuplexGate()
    assert gate.allow_uplink(now=100.0) is False
    gate.begin_listen()
    assert gate.allow_uplink(now=100.0) is True
    gate.end_listen()
    assert gate.allow_uplink(now=100.0) is False


def test_output_audio_mutes_for_duration_plus_tail():
    gate = HalfDuplexGate(tail_secs=0.3)
    gate.begin_listen()
    # 24000Hz 单声道 20ms = 480 samples = 960 bytes。
    gate.note_output_audio(960, 24000, now=10.0)
    assert gate.allow_uplink(now=10.0) is False
    assert gate.allow_uplink(now=10.29) is False
    assert gate.allow_uplink(now=10.33) is True


def test_tts_start_drops_uplink_before_audio():
    gate = HalfDuplexGate(tail_secs=0.3)
    gate.begin_listen()
    gate.begin_output()
    # 即使还没有任何音频写出，TTS start→stop 之间也必须丢弃。
    assert gate.allow_uplink(now=10.0) is False
    assert gate.speaking is True
    gate.close_output(now=10.0)
    assert gate.allow_uplink(now=10.1) is False  # tail
    assert gate.allow_uplink(now=10.31) is True


def test_abort_clears_output_active():
    gate = HalfDuplexGate()
    gate.begin_listen()
    gate.begin_output()
    assert gate.allow_uplink(now=20.0) is False
    gate.abort()
    assert gate.allow_uplink(now=20.0) is True


def test_close_output_keeps_tail():
    gate = HalfDuplexGate(tail_secs=0.3)
    gate.begin_listen()
    gate.close_output(now=20.0)
    assert gate.allow_uplink(now=20.29) is False
    assert gate.allow_uplink(now=20.31) is True


def test_only_extends_never_shortens():
    gate = HalfDuplexGate(tail_secs=0.3)
    gate.begin_listen()
    gate.note_output_audio(960 * 10, 24000, now=30.0)  # 200ms + 300ms tail
    before = gate.muted_until
    gate.close_output(now=30.1)
    assert gate.muted_until == before


def test_abort_unmutes_immediately():
    gate = HalfDuplexGate(tail_secs=0.3)
    gate.begin_listen()
    gate.note_output_audio(960, 24000, now=40.0)
    assert gate.allow_uplink(now=40.0) is False
    generation = gate.generation
    gate.abort()
    assert gate.allow_uplink(now=40.0) is True
    assert gate.generation == generation + 1


def test_barge_in_allows_uplink_while_speaking():
    gate = HalfDuplexGate(barge_in=True)
    gate.begin_listen()
    gate.note_output_audio(960, 24000, now=50.0)
    assert gate.allow_uplink(now=50.0) is True
    # 但没在 listen 状态时仍然丢弃。
    gate.end_listen()
    assert gate.allow_uplink(now=50.0) is False


def test_reset():
    gate = HalfDuplexGate()
    gate.begin_listen()
    gate.note_output_audio(960, 24000, now=60.0)
    gate.reset()
    assert gate.listening is False
    assert gate.allow_uplink(now=60.0) is False
