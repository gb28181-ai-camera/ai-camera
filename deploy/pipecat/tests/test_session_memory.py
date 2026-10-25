"""会话结束时写摘要的最小验证（不启动 Pipecat）。"""

from __future__ import annotations

import pytest

from ws.auth import AuthResult, VoiceContext
from ws.session import WebSocketVoiceSession


class _DummyWebSocket:
    client = None

    async def send_text(self, *_args, **_kwargs) -> None:
        return None

    async def close(self, *_args, **_kwargs) -> None:
        return None


class _Bundle:
    class _Context:
        messages = [
            {"role": "system", "content": "系统提示"},
            {"role": "user", "content": "打开客厅摄像头"},
            {"role": "assistant", "content": "好的，已打开。"},
        ]

    context = _Context()


class _Memory:
    enabled = True

    def __init__(self) -> None:
        self.called = None

    async def write(self, **kwargs):
        self.called = kwargs
        return True


@pytest.mark.asyncio
async def test_write_memory_summary_calls_go():
    auth = AuthResult(
        token_valid=True,
        context=VoiceContext(
            session_id="s1",
            user_id="u1",
            device_id="d1",
            dept_id="10",
            verified=True,
        ),
        context_header="signed",
        context_verified=True,
    )
    session = WebSocketVoiceSession(_DummyWebSocket(), auth, {})
    memory = _Memory()
    session.memory = memory
    session._bundle = _Bundle()

    await session._write_memory_summary()

    assert memory.called is not None
    assert memory.called["session_id"] == "s1"
    assert memory.called["user_id"] == "u1"
    assert memory.called["dept_id"] == "10"
    assert "打开客厅摄像头" in memory.called["summary"]
    assert len(memory.called["turns"]) == 2
    assert memory.called["message_count"] == 3


class _Context:
    def __init__(self, messages=None) -> None:
        self.messages = list(messages or [])
        self.tools = None

    def set_messages(self, messages):
        self.messages = list(messages)

    def set_tools(self, tools):
        self.tools = list(tools)


class _LLM:
    def __init__(self) -> None:
        self.registered = {}

    def register_function(self, name, handler):
        self.registered[name] = handler


class _RuntimeBundle:
    def __init__(self, messages=None) -> None:
        self.context = _Context(messages)
        self.llm = _LLM()


def _session() -> WebSocketVoiceSession:
    auth = AuthResult(
        token_valid=True,
        context=VoiceContext(session_id="s3", user_id="u1", verified=True),
        context_header="",
        context_verified=True,
    )
    return WebSocketVoiceSession(_DummyWebSocket(), auth, {})


@pytest.mark.asyncio
async def test_apply_system_prompt_preserves_turns():
    session = _session()
    session._bundle = _RuntimeBundle(
        [
            {"role": "system", "content": "old"},
            {"role": "user", "content": "hi"},
        ]
    )
    session._apply_system_prompt("new prompt")
    messages = session._bundle.context.messages
    assert messages[0] == {"role": "system", "content": "new prompt"}
    assert messages[1]["content"] == "hi"


@pytest.mark.asyncio
async def test_load_memory_injects_prompt():
    from voice_memory.client import MemoryItem

    session = _session()
    session._bundle = _RuntimeBundle()

    class Memory:
        enabled = True

        async def retrieve(self, **_kwargs):
            return [MemoryItem(content="用户偏好中文")]

    session.memory = Memory()
    await session._load_memory_background()
    messages = session._bundle.context.messages
    assert messages and messages[0]["role"] == "system"
    assert "用户偏好中文" in messages[0]["content"]


@pytest.mark.asyncio
async def test_apply_tool_schemas_registers_handler():
    session = _session()
    session._bundle = _RuntimeBundle()

    class Schema:
        name = "device_screen_capture"
        handler = object()

    await session._apply_tool_schemas([Schema()])
    assert len(session._bundle.context.tools) == 1
    assert session._bundle.context.tools[0].name == "device_screen_capture"
    assert "device_screen_capture" in session._bundle.llm.registered
    assert session.info.tools == 1


@pytest.mark.asyncio
async def test_write_memory_summary_skips_without_turns():
    auth = AuthResult(
        token_valid=True,
        context=VoiceContext(session_id="s2", verified=True),
        context_header="",
        context_verified=True,
    )
    session = WebSocketVoiceSession(_DummyWebSocket(), auth, {})

    class EmptyBundle:
        class _Context:
            messages = [{"role": "system", "content": "sys"}]

        context = _Context()

    memory = _Memory()
    session.memory = memory
    session._bundle = EmptyBundle()

    await session._write_memory_summary()
    assert memory.called is None
