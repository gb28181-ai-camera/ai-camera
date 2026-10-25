"""协议解析/构造单测（不依赖 pipecat / FastAPI）。"""

from __future__ import annotations

import json

import pytest

from ws import protocol


def _hello(**audio_overrides):
    audio = {
        "format": "pcm16",
        "sample_rate": 16000,
        "channels": 1,
        "frame_duration": 20,
    }
    audio.update(audio_overrides)
    return json.dumps(
        {
            "type": "hello",
            "version": 1,
            "transport": "websocket",
            "features": {"mcp": True},
            "audio_params": audio,
            "client": {"kind": "app", "client_id": "c1", "device_id": "d1"},
        }
    )


def test_parse_hello_pcm16():
    parsed = protocol.parse_message(_hello())
    assert parsed.type == "hello"
    hello = parsed.hello
    assert hello is not None
    assert hello.audio_params.format == "pcm16"
    assert hello.audio_params.sample_rate == 16000
    assert hello.mcp_enabled is True
    assert hello.client_kind == "app"
    assert hello.device_id == "d1"


def test_parse_hello_pcm16_8k_allowed():
    parsed = protocol.parse_message(_hello(sample_rate=8000))
    assert parsed.hello.audio_params.sample_rate == 8000


def test_parse_hello_opus():
    parsed = protocol.parse_message(_hello(format="opus"))
    assert parsed.hello.audio_params.format == "opus"


@pytest.mark.parametrize("fmt", ["wav", "mp3"])
def test_parse_hello_bad_format(fmt):
    with pytest.raises(protocol.ProtocolError) as exc:
        protocol.parse_message(_hello(format=fmt))
    assert exc.value.code == "unsupported_audio_format"


def test_parse_hello_opus_requires_16k():
    with pytest.raises(protocol.ProtocolError) as exc:
        protocol.parse_message(_hello(format="opus", sample_rate=8000))
    assert exc.value.code == "unsupported_sample_rate"


def test_parse_hello_requires_single_channel():
    with pytest.raises(protocol.ProtocolError) as exc:
        protocol.parse_message(_hello(channels=2))
    assert exc.value.code == "unsupported_channels"


def test_parse_hello_rejects_future_version():
    payload = json.loads(_hello())
    payload["version"] = 9
    with pytest.raises(protocol.ProtocolError) as exc:
        protocol.parse_message(json.dumps(payload))
    assert exc.value.code == "unsupported_version"


def test_parse_listen_modes():
    parsed = protocol.parse_message('{"type":"listen","state":"detect","text":"小智"}')
    assert parsed.listen.state == "detect"
    # mode 省略时沿用当前值，不能默认成 auto 把 manual 会话改掉。
    assert parsed.listen.mode == ""

    parsed = protocol.parse_message('{"type":"listen","state":"start","mode":"manual"}')
    assert parsed.listen.mode == "manual"


@pytest.mark.parametrize("state", ["begin", "end", ""])
def test_parse_listen_bad_state(state):
    with pytest.raises(protocol.ProtocolError) as exc:
        protocol.parse_message(json.dumps({"type": "listen", "state": state}))
    assert exc.value.code == "invalid_listen"


def test_parse_abort():
    parsed = protocol.parse_message('{"type":"abort","reason":"user"}')
    assert parsed.abort.reason == "user"


def test_parse_mcp_requires_jsonrpc():
    good = {
        "type": "mcp",
        "payload": {"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": {}},
    }
    parsed = protocol.parse_message(json.dumps(good))
    assert parsed.mcp.payload["method"] == "tools/list"

    bad = {"type": "mcp", "payload": {"jsonrpc": "1.0", "id": 1, "method": "x"}}
    with pytest.raises(protocol.ProtocolError) as exc:
        protocol.parse_message(json.dumps(bad))
    assert exc.value.code == "invalid_jsonrpc"


def test_parse_ping_pong():
    parsed = protocol.parse_message('{"type":"ping","ts":123}')
    assert parsed.type == "ping"
    assert parsed.ping_ts == 123
    assert protocol.pong_message(123) == {"type": "pong", "ts": 123}


def test_parse_unknown_type():
    with pytest.raises(protocol.ProtocolError) as exc:
        protocol.parse_message('{"type":"speak","text":"hi"}')
    assert exc.value.code == "unsupported_type"


def test_parse_oversized():
    raw = json.dumps({"type": "listen", "state": "start", "text": "x" * (protocol.MAX_MESSAGE_BYTES + 1)})
    with pytest.raises(protocol.ProtocolError) as exc:
        protocol.parse_message(raw)
    assert exc.value.code == "message_too_large"


def test_server_message_builders():
    hello = protocol.server_hello(
        "s1", audio_format="opus", capabilities={"memory": False}
    )
    assert hello["session_id"] == "s1"
    assert hello["audio_params"]["format"] == "opus"
    assert hello["audio_params"]["sample_rate"] == 24000
    assert hello["capabilities"]["mcp"] is True
    assert hello["capabilities"]["memory"] is False
    assert hello["capabilities"]["barge_in"] is False

    assert protocol.stt_message("hi", is_final=True, confidence=0.91234, turn_id=2) == {
        "type": "stt",
        "text": "hi",
        "is_final": True,
        "confidence": 0.9123,
        "turn_id": 2,
    }
    assert protocol.llm_message("你") == {"type": "llm", "text": "你"}
    assert protocol.tts_message("stop", turn_id=1) == {
        "type": "tts",
        "state": "stop",
        "turn_id": 1,
    }
    assert protocol.tool_message("calling", name="device.list")["state"] == "calling"
    assert protocol.error_message("x", "boom")["retryable"] is False
    assert protocol.wake_message("小智", confidence=0.5)["state"] == "detected"
    assert protocol.mcp_message({"jsonrpc": "2.0"})["type"] == "mcp"
