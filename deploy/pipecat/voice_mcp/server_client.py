"""服务端 MCP：HTTP JSON-RPC 调 Go ``/internal/mcp``。

协议（设计文档 §4）：

```http
POST /internal/mcp
Authorization: Bearer <INTERNAL_MCP_TOKEN>
Content-Type: application/json
```

支持 ``initialize`` / ``tools/list``（cursor 分页）/ ``tools/call``。
工具名里的 ``.`` 会映射成 LLM 可接受的字符（``device.list`` →
``device_list``），调用时再还原成 Go 侧的原始名字。

失败语义：连接失败/超时/工具报错都转成 :class:`MCPClientError`，由会话层
回给 LLM 一段可读的文本（例如「设备查询暂不可用」），**不阻塞音频链路**。
"""

from __future__ import annotations

import asyncio
import itertools
import logging
import re
from dataclasses import dataclass, field
from typing import Any, Dict, List, Mapping, Optional, Sequence

from internal_http import InternalHTTPError, join_url
from internal_http import post_json as _post_json

logger = logging.getLogger("ai-camera-pipecat.mcp.server")

MCP_PATH = "/internal/mcp"
MAX_PAGES = 20
MAX_TOOL_NAME_LEN = 64

_NAME_SAFE_RE = re.compile(r"[^0-9A-Za-z_-]")


class MCPClientError(RuntimeError):
    """MCP 调用失败（连接、协议、工具错误）。"""


def sanitize_tool_name(name: str, *, prefix: str = "") -> str:
    """把任意 MCP 工具名转成 LLM FunctionSchema 可用的名字。

    - 非 ``[0-9A-Za-z_-]`` 字符替换为 ``_``；
    - 保留可选的 ``prefix``（如 ``device_``）；
    - 截断到 64 字符；
    - 不以字母/下划线开头时补 ``tool_``。
    """

    raw = str(name or "").strip()
    safe = _NAME_SAFE_RE.sub("_", raw)
    if prefix and not safe.startswith(prefix):
        safe = prefix + safe
    safe = safe.strip("_") or "tool"
    if not (safe[0].isalpha() or safe[0] == "_"):
        safe = "tool_" + safe
    return safe[:MAX_TOOL_NAME_LEN]


@dataclass(frozen=True)
class MCPTool:
    """MCP 工具定义（LLM 侧名字与原 wire name 分开保存）。"""

    name: str  # wire name（Go 侧，例如 device.list）
    description: str = ""
    input_schema: Mapping[str, Any] = field(default_factory=dict)
    llm_name: str = ""

    def with_llm_name(self, name: str) -> "MCPTool":
        return MCPTool(
            name=self.name,
            description=self.description,
            input_schema=self.input_schema,
            llm_name=name,
        )


@dataclass(frozen=True)
class MCPCallResult:
    text: str
    is_error: bool = False
    raw: Mapping[str, Any] = field(default_factory=dict, repr=False)


def _normalize_tool(raw: Mapping[str, Any]) -> Optional[MCPTool]:
    if not isinstance(raw, Mapping):
        return None
    name = str(raw.get("name") or "").strip()
    if not name:
        return None
    schema = raw.get("inputSchema") or raw.get("input_schema") or {}
    if not isinstance(schema, Mapping):
        schema = {}
    return MCPTool(
        name=name,
        description=str(raw.get("description") or "").strip(),
        input_schema=dict(schema),
        llm_name=sanitize_tool_name(name),
    )


def _result_text(result: Mapping[str, Any]) -> str:
    """把 MCP tools/call 的 content 数组拼成纯文本。"""

    content = result.get("content")
    if isinstance(content, str):
        return content
    if isinstance(content, Sequence) and not isinstance(content, (bytes, bytearray)):
        parts: List[str] = []
        for item in content:
            if isinstance(item, str):
                parts.append(item)
            elif isinstance(item, Mapping):
                if item.get("type") == "text" and "text" in item:
                    parts.append(str(item.get("text") or ""))
                elif "content" in item:
                    parts.append(str(item.get("content") or ""))
        if parts:
            return "\n".join(part for part in parts if part)
    if "structuredContent" in result:
        return str(result["structuredContent"])
    return ""


def build_function_schema(
    name: str,
    description: str,
    input_schema: Mapping[str, Any],
    handler: Any,
) -> Any:
    """MCP 工具 → Pipecat ``FunctionSchema``。

    延迟导入 pipecat；缺失时抛 :class:`MCPClientError`。
    """

    try:
        from pipecat.adapters.schemas.function_schema import FunctionSchema  # type: ignore
    except Exception as exc:  # pragma: no cover - 依赖环境
        raise MCPClientError(f"当前 Pipecat 版本没有 FunctionSchema：{exc}") from exc

    schema = input_schema if isinstance(input_schema, Mapping) else {}
    properties = schema.get("properties")
    if not isinstance(properties, Mapping):
        properties = {}
    required = schema.get("required")
    if not isinstance(required, Sequence) or isinstance(required, (str, bytes)):
        required = []

    return FunctionSchema(
        name=sanitize_tool_name(name),
        description=description or name,
        properties=dict(properties),
        required=[str(item) for item in required],
        handler=handler,
    )


class MCPServerClient:
    """Go 内部 MCP 的 JSON-RPC 客户端。"""

    def __init__(
        self,
        base_url: str,
        token: str,
        *,
        context_header: str = "",
        session_id: str = "",
        timeout: float = 8.0,
        max_pages: int = MAX_PAGES,
    ) -> None:
        self.base_url = (base_url or "").strip().rstrip("/")
        self.token = (token or "").strip()
        self.context_header = context_header
        self.session_id = session_id
        self.timeout = float(timeout)
        self.max_pages = max(1, int(max_pages))
        self._ids = itertools.count(1)
        self._initialized = False

    @property
    def enabled(self) -> bool:
        return bool(self.base_url and self.token)

    @property
    def endpoint(self) -> str:
        return join_url(self.base_url, MCP_PATH)

    def _headers(self) -> Dict[str, str]:
        headers = {"Authorization": f"Bearer {self.token}"}
        if self.context_header:
            # 让 Go 侧重新验签并使用同一份用户/部门上下文做权限判断。
            headers["X-Voice-Context"] = self.context_header
        if self.session_id:
            headers["X-Pipecat-Session-Id"] = self.session_id
        return headers

    async def _rpc(
        self,
        method: str,
        params: Optional[Mapping[str, Any]] = None,
        *,
        timeout: Optional[float] = None,
    ) -> Mapping[str, Any]:
        if not self.enabled:
            raise MCPClientError("服务端 MCP 未配置（缺少内部地址或 token）")

        request_id = f"pipecat-{next(self._ids)}"
        payload = {
            "jsonrpc": "2.0",
            "id": request_id,
            "method": method,
            "params": dict(params or {}),
        }
        try:
            response = await asyncio.to_thread(
                _post_json,
                self.endpoint,
                payload,
                headers=self._headers(),
                timeout=self.timeout if timeout is None else float(timeout),
            )
        except InternalHTTPError as exc:
            raise MCPClientError(str(exc)) from exc
        except Exception as exc:  # 兜底，避免线程池异常穿透
            raise MCPClientError(f"服务端 MCP 请求失败：{type(exc).__name__}") from exc

        if not isinstance(response, Mapping):
            raise MCPClientError("服务端 MCP 返回不是 JSON 对象")
        error = response.get("error")
        if isinstance(error, Mapping):
            message = str(error.get("message") or "未知错误")
            code = error.get("code")
            raise MCPClientError(f"服务端 MCP 错误 {code}: {message}")
        result = response.get("result")
        if isinstance(result, Mapping):
            return result
        if result is None:
            return {}
        return {"value": result}

    async def initialize(self) -> Mapping[str, Any]:
        result = await self._rpc(
            "initialize",
            {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {"name": "ai-camera-pipecat", "version": "0.6.0"},
            },
        )
        self._initialized = True
        return result

    async def list_tools(self) -> List[MCPTool]:
        """拉取全部工具（跟随 cursor，最多 ``max_pages`` 页）。"""

        tools: List[MCPTool] = []
        cursor: Optional[str] = None
        seen = 0
        while seen < self.max_pages:
            seen += 1
            params: Dict[str, Any] = {}
            if cursor:
                params["cursor"] = cursor
            result = await self._rpc("tools/list", params)
            raw_tools = result.get("tools")
            if isinstance(raw_tools, Sequence) and not isinstance(raw_tools, (str, bytes)):
                for raw in raw_tools:
                    tool = _normalize_tool(raw) if isinstance(raw, Mapping) else None
                    if tool is not None:
                        tools.append(tool)
            next_cursor = result.get("nextCursor") or result.get("cursor")
            if not next_cursor or str(next_cursor) == cursor:
                break
            cursor = str(next_cursor)
        return tools

    async def call_tool(
        self,
        name: str,
        arguments: Optional[Mapping[str, Any]] = None,
        *,
        timeout: Optional[float] = None,
    ) -> MCPCallResult:
        result = await self._rpc(
            "tools/call",
            {"name": str(name), "arguments": dict(arguments or {})},
            timeout=timeout,
        )
        return MCPCallResult(
            text=_result_text(result),
            is_error=bool(result.get("isError")),
            raw=result,
        )

    async def close(self) -> None:
        """HTTP 客户端无持久连接，保留接口便于统一清理。"""

        self._initialized = False


__all__ = (
    "MCPCallResult",
    "MCPClientError",
    "MCPServerClient",
    "MCPTool",
    "MCP_PATH",
    "build_function_schema",
    "sanitize_tool_name",
)
