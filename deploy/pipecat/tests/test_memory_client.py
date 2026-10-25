"""记忆客户端 + 短期摘要单测。"""

from __future__ import annotations

from typing import Any, Dict, List

import pytest

from voice_memory import client as memory_client
from voice_memory import short_term
from voice_memory.client import MemoryItem


def _fake_post_factory(calls: List[Dict[str, Any]], response: Any):
    def fake_post(url, payload, *, headers=None, timeout=5.0):
        calls.append({"url": url, "payload": payload, "headers": headers})
        return response

    return fake_post


@pytest.mark.asyncio
async def test_retrieve_parses_nested_memories(monkeypatch):
    calls: List[Dict[str, Any]] = []
    monkeypatch.setattr(
        memory_client,
        "_post_json",
        _fake_post_factory(
            calls,
            {
                "data": {
                    "memories": [
                        {"content": "用户喜欢客厅的摄像头", "score": 0.93, "kind": "preference"},
                        {"text": "设备 D1 在二楼"},
                    ]
                }
            },
        ),
    )
    client = memory_client.MemoryClient(
        "http://backend:8081", "token", context_header="ctx", session_id="s1"
    )
    assert client.enabled
    items = await client.retrieve(
        session_id="s1", user_id=1, dept_id=10, query="摄像头", top_k=5
    )
    assert [item.content for item in items] == ["用户喜欢客厅的摄像头", "设备 D1 在二楼"]
    assert items[0].score == pytest.approx(0.93)
    assert items[0].kind == "preference"

    sent = calls[0]
    assert sent["url"] == "http://backend:8081/internal/voice/memory/retrieve"
    assert sent["payload"]["top_k"] == 5
    assert sent["payload"]["query"] == "摄像头"
    # Go MemoryRetrieveReq 只认 owner_keys（身份从签名上下文来）。
    assert sent["payload"]["owner_keys"] == ["user:1", "dept:10", "global"]
    assert "user_id" not in sent["payload"]
    assert sent["headers"]["Authorization"] == "Bearer token"
    assert sent["headers"]["X-Voice-Context"] == "ctx"


@pytest.mark.asyncio
async def test_write_payload(monkeypatch):
    calls: List[Dict[str, Any]] = []
    monkeypatch.setattr(
        memory_client, "_post_json", _fake_post_factory(calls, {"ok": True})
    )
    client = memory_client.MemoryClient("http://backend:8081", "token")
    ok = await client.write(
        session_id="s1",
        summary="用户想让助手每天早上播报天气。",
        user_id=1,
        turns=[{"role": "user", "content": "早上播报天气"}, {"role": "assistant", "content": "好的"}],
        message_count=2,
    )
    assert ok is True
    payload = calls[0]["payload"]
    assert calls[0]["url"].endswith("/internal/voice/memory/write")
    # Go MemoryWriteReq 必填 owner_type / content。
    assert payload["owner_type"] == "user"
    assert payload["owner_id"] == 1
    assert payload["kind"] == "summary"
    assert payload["content"].startswith("用户想让")
    assert payload["source_session_id"] == "s1"
    assert payload["message_count"] == 2
    assert len(payload["turns"]) == 2
    # 不存原始音频字段
    assert "audio" not in payload


@pytest.mark.asyncio
async def test_disabled_client():
    client = memory_client.MemoryClient("", "")
    assert client.enabled is False
    with pytest.raises(memory_client.MemoryClientError):
        await client.retrieve(session_id="s1")


def test_owner_keys_match_go_format():
    assert memory_client.build_owner_keys(user_id=1, dept_id=10) == [
        "user:1",
        "dept:10",
        "global",
    ]
    assert memory_client.build_owner_keys(device_id=2, user_id=1) == [
        "device:2",
        "user:1",
        "global",
    ]
    assert memory_client.build_owner_keys() == ["global"]
    assert memory_client.build_owner_keys(include_global=False) == []


def test_primary_owner_precedence():
    assert memory_client.primary_owner(device_id=2, user_id=1, dept_id=10) == ("user", 1)
    assert memory_client.primary_owner(device_id=2, dept_id=10) == ("device", 2)
    assert memory_client.primary_owner(dept_id=10) == ("dept", 10)
    assert memory_client.primary_owner() == ("global", 0)


@pytest.mark.asyncio
async def test_retrieve_parses_go_items(monkeypatch):
    calls: List[Dict[str, Any]] = []
    monkeypatch.setattr(
        memory_client,
        "_post_json",
        _fake_post_factory(
            calls,
            {
                "items": [
                    {
                        "id": 7,
                        "owner_type": "user",
                        "owner_key": "user:1",
                        "kind": "preference",
                        "content": "用户偏好中文",
                        "importance": 4,
                        "score": 0.88,
                        "created_dt": "2026-10-25T00:00:00Z",
                    }
                ]
            },
        ),
    )
    client = memory_client.MemoryClient("http://backend:8080", "token")
    items = await client.retrieve(session_id="s1", user_id=1, top_k=5)
    assert len(items) == 1
    assert items[0].content == "用户偏好中文"
    assert items[0].memory_id == "7"
    assert items[0].kind == "preference"
    assert items[0].importance == 4
    assert items[0].score == pytest.approx(0.88)
    assert items[0].created_at.startswith("2026-10-25")


def test_render_system_prompt_with_memories():
    prompt = short_term.render_system_prompt(
        "你是 AI 摄像头助手。",
        [MemoryItem(content="用户偏好中文"), MemoryItem(content="用户住在杭州")],
    )
    assert "AI 摄像头助手" in prompt
    assert "[长期记忆]" in prompt
    assert "- 用户偏好中文" in prompt
    assert "[/长期记忆]" in prompt


def test_render_system_prompt_without_memories_keeps_base():
    assert short_term.render_system_prompt("base", []) == "base"


def test_render_system_prompt_budget():
    prompt = short_term.render_system_prompt(
        "base",
        [MemoryItem(content="x" * 100), MemoryItem(content="y" * 100)],
        max_chars=50,
    )
    assert "y" not in prompt


def test_summarize_messages():
    messages = [
        {"role": "system", "content": "系统提示"},
        {"role": "user", "content": "打开客厅摄像头"},
        {"role": "assistant", "content": "好的，已打开。"},
        {"role": "user", "content": [{"type": "text", "text": "再关掉"}]},
    ]
    summary = short_term.summarize_messages(messages)
    assert "会话共 3 条消息" in summary
    assert "用户：打开客厅摄像头" in summary
    assert "助手：好的，已打开。" in summary
    assert "用户：再关掉" in summary
    assert "系统提示" not in summary


def test_summarize_messages_respects_budget():
    messages = [{"role": "user", "content": "x" * 500} for _ in range(20)]
    summary = short_term.summarize_messages(messages, max_chars=120)
    assert len(summary) <= 200
    assert summary


def test_collect_turns_skips_system():
    turns = short_term.collect_turns(
        [
            {"role": "system", "content": "sys"},
            {"role": "user", "content": "hi"},
            {"role": "assistant", "content": "hello"},
        ]
    )
    assert turns == [
        {"role": "user", "content": "hi"},
        {"role": "assistant", "content": "hello"},
    ]
