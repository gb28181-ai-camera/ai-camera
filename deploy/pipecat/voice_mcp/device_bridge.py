"""设备端 MCP bridge（同一条 WebSocket 上跑 JSON-RPC）。

小智设备在 ``hello.features.mcp=true`` 后，Pipecat 通过：

```json
{"type":"mcp","payload":{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}}
```

与设备完成 ``initialize → tools/list（cursor 分页）→ tools/call``。
设备工具会被映射成 Pipecat ``FunctionSchema``，交给 LLM；工具名含 ``.``
时转成 ``device_...``（例如 ``screen.capture`` → ``device_screen_capture``），
调用时还原成原始 ``wire_name``。

## 不阻塞音频

- 所有 RPC 都有超时（默认 6s），超时只让该次工具调用失败；
- 断线时 ``close()`` 立即让所有 pending 调用抛错；
- 初始化/列工具失败只记录日志，会话照常进行（没有设备工具而已）。
"""

from __future__ import annotations

import asyncio
import itertools
import logging
from dataclasses import dataclass, field
from typing import Any, Awaitable, Callable, Dict, List, Mapping, Optional, Sequence

from voice_mcp.server_client import (
    MCPCallResult,
    MCPClientError,
    _normalize_tool,
    _result_text,
    sanitize_tool_name,
)

logger = logging.getLogger("ai-camera-pipecat.mcp.device")

PROTOCOL_VERSION = "2024-11-05"
SendCallback = Callable[[Mapping[str, Any]], Awaitable[None]]


class DeviceMCPError(MCPClientError):
    """设备端 MCP 调用失败。"""


class DeviceMCPTimeout(DeviceMCPError):
    """设备未在超时内响应。"""


@dataclass(frozen=True)
class DeviceTool:
    """设备工具（wire name 与 LLM name 分离）。"""

    wire_name: str
    llm_name: str
    description: str = ""
    input_schema: Mapping[str, Any] = field(default_factory=dict)


def map_device_tool_name(wire_name: str) -> str:
    """设备工具名 → LLM FunctionSchema 名。

    含 ``.`` 的工具按设计映射成 ``device_<...>``；其余名字只做字符
    清洗（保持与设备侧一致，便于排查）。
    """

    raw = str(wire_name or "").strip()
    if "." in raw:
        return sanitize_tool_name("device_" + raw.replace(".", "_"))
    return sanitize_tool_name(raw)


def _to_device_tool(raw: Mapping[str, Any]) -> Optional[DeviceTool]:
    normalized = _normalize_tool(raw)
    if normalized is None:
        return None
    return DeviceTool(
        wire_name=normalized.name,
        llm_name=map_device_tool_name(normalized.name),
        description=normalized.description,
        input_schema=normalized.input_schema,
    )


class DeviceMCPBridge:
    """在单条 WS 上实现 JSON-RPC 客户端。"""

    def __init__(
        self,
        send: SendCallback,
        *,
        timeout: float = 6.0,
        protocol_version: str = PROTOCOL_VERSION,
        client_name: str = "ai-camera-pipecat",
        client_version: str = "0.6.0",
        max_pages: int = 10,
    ) -> None:
        self._send = send
        self.timeout = float(timeout)
        self.protocol_version = protocol_version
        self.client_name = client_name
        self.client_version = client_version
        self.max_pages = max(1, int(max_pages))
        self._ids = itertools.count(1)
        self._pending: Dict[str, "asyncio.Future[Mapping[str, Any]]"] = {}
        self._closed = False

    # ── 生命周期 ────────────────────────────────────────────────
    async def close(self, reason: str = "设备连接已关闭") -> None:
        self._closed = True
        for key, future in list(self._pending.items()):
            if not future.done():
                future.set_exception(DeviceMCPError(reason))
            self._pending.pop(key, None)

    @property
    def pending(self) -> int:
        return len(self._pending)

    # ── 发送/接收 ───────────────────────────────────────────────
    async def _rpc(
        self,
        method: str,
        params: Optional[Mapping[str, Any]] = None,
        *,
        timeout: Optional[float] = None,
    ) -> Mapping[str, Any]:
        if self._closed:
            raise DeviceMCPError("设备 MCP 已关闭")

        request_id = next(self._ids)
        key = str(request_id)
        loop = asyncio.get_running_loop()
        future: "asyncio.Future[Mapping[str, Any]]" = loop.create_future()
        self._pending[key] = future
        message = {
            "jsonrpc": "2.0",
            "id": request_id,
            "method": method,
            "params": dict(params or {}),
        }
        try:
            await self._send(message)
            response = await asyncio.wait_for(
                future, timeout=self.timeout if timeout is None else float(timeout)
            )
        except (asyncio.TimeoutError, TimeoutError) as exc:
            raise DeviceMCPTimeout(
                f"设备 MCP {method} 超时（{self.timeout if timeout is None else timeout}s）"
            ) from exc
        except DeviceMCPError:
            raise
        except Exception as exc:
            raise DeviceMCPError(f"设备 MCP {method} 发送失败：{type(exc).__name__}") from exc
        finally:
            self._pending.pop(key, None)

        error = response.get("error")
        if isinstance(error, Mapping):
            message_text = str(error.get("message") or "未知错误")
            raise DeviceMCPError(f"设备 MCP 错误 {error.get('code')}: {message_text}")
        result = response.get("result")
        if isinstance(result, Mapping):
            return result
        if result is None:
            return {}
        return {"value": result}

    async def handle_message(self, payload: Mapping[str, Any]) -> bool:
        """处理一条来自设备的 ``mcp.payload``。

        返回 True 表示已消费（响应或设备发起的请求），False 表示无关消息。
        """

        if not isinstance(payload, Mapping):
            return False

        msg_id = payload.get("id")
        has_response = "result" in payload or "error" in payload
        if has_response and msg_id is not None:
            future = self._pending.pop(str(msg_id), None)
            if future is None:
                return False
            if not future.done():
                future.set_result(payload)
            return True

        method = payload.get("method")
        if isinstance(method, str) and msg_id is not None:
            # 设备主动发请求：只实现最小集合，避免 bridge 静默卡住设备。
            if method == "ping":
                await self._send({"jsonrpc": "2.0", "id": msg_id, "result": {}})
            elif method == "initialize":
                await self._send(
                    {
                        "jsonrpc": "2.0",
                        "id": msg_id,
                        "result": {
                            "protocolVersion": self.protocol_version,
                            "capabilities": {"tools": {}},
                            "serverInfo": {
                                "name": self.client_name,
                                "version": self.client_version,
                            },
                        },
                    }
                )
            else:
                await self._send(
                    {
                        "jsonrpc": "2.0",
                        "id": msg_id,
                        "error": {"code": -32601, "message": f"不支持的方法 {method}"},
                    }
                )
            return True

        return False

    # ── MCP 方法 ────────────────────────────────────────────────
    async def initialize(self, *, timeout: Optional[float] = None) -> Mapping[str, Any]:
        return await self._rpc(
            "initialize",
            {
                "protocolVersion": self.protocol_version,
                "capabilities": {"tools": {}},
                "clientInfo": {"name": self.client_name, "version": self.client_version},
            },
            timeout=timeout,
        )

    async def list_tools(self, *, timeout: Optional[float] = None) -> List[DeviceTool]:
        tools: List[DeviceTool] = []
        cursor: Optional[str] = None
        for _ in range(self.max_pages):
            params: Dict[str, Any] = {}
            if cursor:
                params["cursor"] = cursor
            result = await self._rpc("tools/list", params, timeout=timeout)
            raw_tools = result.get("tools")
            if isinstance(raw_tools, Sequence) and not isinstance(raw_tools, (str, bytes)):
                for raw in raw_tools:
                    if isinstance(raw, Mapping):
                        tool = _to_device_tool(raw)
                        if tool is not None:
                            tools.append(tool)
            next_cursor = result.get("nextCursor") or result.get("cursor")
            if not next_cursor or str(next_cursor) == cursor:
                break
            cursor = str(next_cursor)
        return tools

    async def call_tool(
        self,
        tool: "DeviceTool | str",
        arguments: Optional[Mapping[str, Any]] = None,
        *,
        timeout: Optional[float] = None,
    ) -> MCPCallResult:
        wire_name = tool.wire_name if isinstance(tool, DeviceTool) else str(tool)
        result = await self._rpc(
            "tools/call",
            {"name": wire_name, "arguments": dict(arguments or {})},
            timeout=timeout,
        )
        return MCPCallResult(
            text=_result_text(result),
            is_error=bool(result.get("isError")),
            raw=result,
        )


__all__ = (
    "DeviceMCPBridge",
    "DeviceMCPError",
    "DeviceMCPTimeout",
    "DeviceTool",
    "map_device_tool_name",
)
