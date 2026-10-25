"""服务端 MCP 客户端 + 设备端 MCP bridge（mock HTTP / WS）。"""

from __future__ import annotations

import asyncio
from typing import Any, Dict, List, Mapping

import pytest

from voice_mcp import device_bridge, server_client


# ════════════════════════════════════════════════════════════════
# 设备端 bridge
# ════════════════════════════════════════════════════════════════


class FakeDevice:
    def __init__(self) -> None:
        self.sent: List[Mapping[str, Any]] = []
        self.bridge: device_bridge.DeviceMCPBridge | None = None
        self.reply = True

    async def send(self, payload: Mapping[str, Any]) -> None:
        self.sent.append(payload)
        if not self.reply or "method" not in payload:
            return
        await self._handle(payload)

    async def _handle(self, payload: Mapping[str, Any]) -> None:
        method = payload["method"]
        request_id = payload["id"]
        params = payload.get("params") or {}
        if method == "initialize":
            result = {
                "protocolVersion": "2024-11-05",
                "capabilities": {"tools": {}},
                "serverInfo": {"name": "fake-device", "version": "1"},
            }
        elif method == "tools/list":
            if not params.get("cursor"):
                result = {
                    "tools": [
                        {
                            "name": "screen.capture",
                            "description": "截屏",
                            "inputSchema": {
                                "type": "object",
                                "properties": {"quality": {"type": "integer"}},
                                "required": ["quality"],
                            },
                        }
                    ],
                    "nextCursor": "p2",
                }
            else:
                result = {"tools": [{"name": "volume.set", "description": "音量"}]}
        elif method == "tools/call":
            result = {
                "content": [{"type": "text", "text": f"ok:{params.get('name')}"}]
            }
        else:
            result = {}
        assert self.bridge is not None
        await self.bridge.handle_message(
            {"jsonrpc": "2.0", "id": request_id, "result": result}
        )


async def _make_bridge(reply: bool = True, timeout: float = 1.0):
    device = FakeDevice()
    device.reply = reply
    bridge = device_bridge.DeviceMCPBridge(device.send, timeout=timeout)
    device.bridge = bridge
    return device, bridge


def test_map_device_tool_name():
    assert device_bridge.map_device_tool_name("screen.capture") == "device_screen_capture"
    assert device_bridge.map_device_tool_name("volume.set") == "device_volume_set"
    assert device_bridge.map_device_tool_name("reboot") == "reboot"
    # 非法字符清理 + 不以字母开头时补 tool_
    assert device_bridge.map_device_tool_name("9.1") == "device_9_1"


@pytest.mark.asyncio
async def test_initialize_list_and_call():
    device, bridge = await _make_bridge()
    init = await bridge.initialize()
    assert init["protocolVersion"] == "2024-11-05"

    tools = await bridge.list_tools()
    assert [tool.wire_name for tool in tools] == ["screen.capture", "volume.set"]
    assert tools[0].llm_name == "device_screen_capture"
    assert tools[0].input_schema["required"] == ["quality"]

    result = await bridge.call_tool(tools[0], {"quality": 80})
    assert result.text == "ok:screen.capture"
    assert result.is_error is False

    sent_methods = [item["method"] for item in device.sent]
    assert sent_methods == ["initialize", "tools/list", "tools/list", "tools/call"]


@pytest.mark.asyncio
async def test_timeout_does_not_hang():
    device, bridge = await _make_bridge(reply=False, timeout=0.05)
    with pytest.raises(device_bridge.DeviceMCPTimeout):
        await asyncio.wait_for(bridge.call_tool("slow.tool", {}), timeout=1.0)
    assert bridge.pending == 0
    await bridge.close()


@pytest.mark.asyncio
async def test_close_fails_pending_calls():
    device, bridge = await _make_bridge(reply=False, timeout=5.0)
    task = asyncio.create_task(bridge.call_tool("slow.tool", {}))
    await asyncio.sleep(0.02)
    await bridge.close("断开")
    with pytest.raises(device_bridge.DeviceMCPError):
        await task


@pytest.mark.asyncio
async def test_device_initiated_ping_gets_response():
    device, bridge = await _make_bridge()
    handled = await bridge.handle_message(
        {"jsonrpc": "2.0", "id": 7, "method": "ping"}
    )
    assert handled is True
    assert device.sent[-1]["id"] == 7
    assert device.sent[-1]["result"] == {}


@pytest.mark.asyncio
async def test_unrelated_message_ignored():
    device, bridge = await _make_bridge()
    assert await bridge.handle_message({"foo": "bar"}) is False
    assert await bridge.handle_message({"jsonrpc": "2.0", "id": 99, "result": {}}) is False


# ════════════════════════════════════════════════════════════════
# 服务端 MCP 客户端
# ════════════════════════════════════════════════════════════════


def test_sanitize_tool_name():
    assert server_client.sanitize_tool_name("device.list") == "device_list"
    assert server_client.sanitize_tool_name("alarm.query") == "alarm_query"
    assert server_client.sanitize_tool_name("9bad") == "tool_9bad"
    assert len(server_client.sanitize_tool_name("x" * 200)) == 64


@pytest.mark.asyncio
async def test_server_initialize_list_and_call(monkeypatch):
    calls: List[Dict[str, Any]] = []

    def fake_post(url, payload, *, headers=None, timeout=5.0):
        calls.append({"url": url, "payload": payload, "headers": headers})
        method = payload["method"]
        if method == "initialize":
            return {"jsonrpc": "2.0", "id": payload["id"], "result": {"ok": True}}
        if method == "tools/list":
            cursor = (payload.get("params") or {}).get("cursor")
            if cursor:
                return {
                    "jsonrpc": "2.0",
                    "id": payload["id"],
                    "result": {"tools": [{"name": "alarm.query", "description": "告警"}]},
                }
            return {
                "jsonrpc": "2.0",
                "id": payload["id"],
                "result": {
                    "tools": [
                        {
                            "name": "device.list",
                            "description": "设备列表",
                            "inputSchema": {
                                "type": "object",
                                "properties": {"keyword": {"type": "string"}},
                            },
                        }
                    ],
                    "nextCursor": "p2",
                },
            }
        if method == "tools/call":
            return {
                "jsonrpc": "2.0",
                "id": payload["id"],
                "result": {"content": [{"type": "text", "text": "done"}]},
            }
        return {"jsonrpc": "2.0", "id": payload["id"], "error": {"code": -32601, "message": "no"}}

    monkeypatch.setattr(server_client, "_post_json", fake_post)
    client = server_client.MCPServerClient(
        "http://backend:8081",
        "internal-token",
        context_header="signed-context",
        session_id="s1",
    )
    assert client.enabled
    await client.initialize()
    tools = await client.list_tools()
    assert [tool.name for tool in tools] == ["device.list", "alarm.query"]
    assert tools[0].llm_name == "device_list"

    result = await client.call_tool("device.list", {"keyword": "客厅"})
    assert result.text == "done"

    assert calls[0]["url"] == "http://backend:8081/internal/mcp"
    headers = calls[0]["headers"]
    assert headers["Authorization"] == "Bearer internal-token"
    assert headers["X-Voice-Context"] == "signed-context"
    assert headers["X-Pipecat-Session-Id"] == "s1"
    # 第二次 tools/list 带 cursor
    list_calls = [call for call in calls if call["payload"]["method"] == "tools/list"]
    assert list_calls[0]["payload"]["params"] == {}
    assert list_calls[1]["payload"]["params"] == {"cursor": "p2"}


@pytest.mark.asyncio
async def test_server_error_propagates(monkeypatch):
    def fake_post(url, payload, *, headers=None, timeout=5.0):
        return {"jsonrpc": "2.0", "id": payload["id"], "error": {"code": 500, "message": "boom"}}

    monkeypatch.setattr(server_client, "_post_json", fake_post)
    client = server_client.MCPServerClient("http://backend:8081", "token")
    with pytest.raises(server_client.MCPClientError):
        await client.call_tool("x", {})


@pytest.mark.asyncio
async def test_server_disabled():
    client = server_client.MCPServerClient("", "")
    assert client.enabled is False
    with pytest.raises(server_client.MCPClientError):
        await client.initialize()


def test_build_function_schema_requires_pipecat():
    # 本测试环境不一定装 pipecat；没装时应给出明确错误而不是 ImportError 泄漏。
    try:
        schema = server_client.build_function_schema("x.y", "desc", {}, lambda p: None)
    except server_client.MCPClientError as exc:
        assert "FunctionSchema" in str(exc)
    else:
        assert schema.name == "x_y"
