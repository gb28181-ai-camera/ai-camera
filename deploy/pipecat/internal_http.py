"""Pipecat → Go 内部 HTTP 的最小 JSON 客户端。

只使用标准库 ``urllib``（在 ``asyncio.to_thread`` 中执行），避免为两个
内部端点新增 ``httpx``/``aiohttp`` 运行时依赖。

安全约定：
- **不打印请求/响应正文**（可能含用户对话/记忆内容）；
- 错误消息里只保留 HTTP 状态码与截断后的错误描述；
- 调用方负责把 ``Authorization`` 头传进来，本模块不读取密钥。
"""

from __future__ import annotations

import json
import socket
import urllib.error
import urllib.request
from typing import Any, Mapping, Optional

MAX_ERROR_BODY = 300


class InternalHTTPError(RuntimeError):
    """内部 HTTP 调用失败（连接、超时、非 2xx、非法 JSON）。"""

    def __init__(self, message: str, *, status: Optional[int] = None) -> None:
        super().__init__(message)
        self.status = status


def join_url(base_url: str, path: str) -> str:
    """拼接 base URL 与路径；base 已包含该 path 时直接返回。"""

    base = (base_url or "").strip().rstrip("/")
    if not base:
        return ""
    normalized = "/" + path.strip("/")
    if base.endswith(normalized):
        return base
    if "://" not in base:
        base = "http://" + base
    return base + normalized


def post_json(
    url: str,
    payload: Mapping[str, Any],
    *,
    headers: Optional[Mapping[str, str]] = None,
    timeout: float = 5.0,
) -> Any:
    """POST JSON，返回解析后的 JSON（任意类型）。

    失败抛 :class:`InternalHTTPError`；调用方应当把它当作**非致命**错误
    处理（MCP/记忆不可用时音频链路继续）。
    """

    if not url:
        raise InternalHTTPError("内部接口地址未配置")

    body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    request_headers = {"Content-Type": "application/json", "Accept": "application/json"}
    if headers:
        request_headers.update({str(k): str(v) for k, v in headers.items() if v})

    request = urllib.request.Request(url, data=body, headers=request_headers, method="POST")
    try:
        with urllib.request.urlopen(request, timeout=float(timeout)) as response:
            raw = response.read()
    except urllib.error.HTTPError as exc:
        detail = ""
        try:
            detail = exc.read().decode("utf-8", "replace").strip()[:MAX_ERROR_BODY]
        except Exception:
            detail = ""
        message = f"内部接口 HTTP {exc.code}"
        if detail:
            message += f"：{detail}"
        raise InternalHTTPError(message, status=exc.code) from exc
    except (urllib.error.URLError, socket.timeout, TimeoutError, OSError) as exc:
        reason = getattr(exc, "reason", None) or exc
        raise InternalHTTPError(f"内部接口请求失败：{type(reason).__name__}: {reason}") from exc

    if not raw:
        return None
    try:
        return json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise InternalHTTPError("内部接口返回非 JSON") from exc


__all__ = ("InternalHTTPError", "join_url", "post_json")
