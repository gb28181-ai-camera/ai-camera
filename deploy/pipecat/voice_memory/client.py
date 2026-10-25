"""长期记忆的 Go 内部 HTTP 客户端。

端点（设计文档 §8）：

- ``POST /internal/voice/memory/retrieve`` — 会话开始取 top-k；
- ``POST /internal/voice/memory/write`` — 会话结束写摘要。

请求都带 ``Authorization: Bearer <INTERNAL_MCP_TOKEN>`` 与
``X-Voice-Context``（若网关注入了签名上下文）。

请求体与 Go DTO 严格对齐：

- retrieve：``{"owner_keys":["user:1","dept:10","global"],"query":"","top_k":5}``
  （``request.MemoryRetrieveReq``，owner_keys 必填）；
- write：``{"owner_type":"user","owner_id":1,"kind":"summary","content":"...",
  "importance":3,"source_session_id":"s1"}``（``request.MemoryWriteReq``）。

响应兼容 ``{"items":[...]}`` / ``{"memories":[...]}`` / ``{"data":{...}}``；
解析失败只记录长度与类型，不打印记忆内容。
"""

from __future__ import annotations

import asyncio
import logging
from dataclasses import dataclass, field
from typing import Any, Dict, List, Mapping, Optional, Sequence

from internal_http import InternalHTTPError, join_url
from internal_http import post_json as _post_json

logger = logging.getLogger("ai-camera-pipecat.memory")

RETRIEVE_PATH = "/internal/voice/memory/retrieve"
WRITE_PATH = "/internal/voice/memory/write"

MAX_PROMPT_CHARS = 1600
MAX_MEMORY_CONTENT_CHARS = 400


class MemoryClientError(RuntimeError):
    """记忆接口失败（连接、超时、非 2xx）。"""


@dataclass(frozen=True)
class MemoryItem:
    content: str
    score: Optional[float] = None
    kind: str = ""
    owner: str = ""
    memory_id: str = ""
    created_at: str = ""
    importance: int = 0
    raw: Mapping[str, Any] = field(default_factory=dict, repr=False)


def _as_float(value: Any) -> Optional[float]:
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def _as_int_id(value: Any) -> int:
    """签名上下文里的 user_id/device_id/dept_id 在 Go 侧是 int64。"""

    try:
        return int(str(value).strip())
    except (TypeError, ValueError):
        return 0


def build_owner_keys(
    *,
    user_id: Any = 0,
    device_id: Any = 0,
    dept_id: Any = 0,
    include_global: bool = True,
) -> List[str]:
    """构造 Go ``MemoryRetrieveReq.owner_keys``。

    与 Go ``ownerKey()`` 的格式一致：``user:1`` / ``device:2`` /
    ``dept:10`` / ``global``。设备上下文优先（设备记忆最具体）。
    """

    keys: List[str] = []
    device = _as_int_id(device_id)
    user = _as_int_id(user_id)
    dept = _as_int_id(dept_id)
    if device > 0:
        keys.append(f"device:{device}")
    if user > 0:
        keys.append(f"user:{user}")
    if dept > 0:
        keys.append(f"dept:{dept}")
    if include_global:
        keys.append("global")
    return keys


def primary_owner(
    *,
    user_id: Any = 0,
    device_id: Any = 0,
    dept_id: Any = 0,
) -> tuple[str, int]:
    """摘要写入时的 owner：用户 > 设备 > 部门 > global。

    App/Web 会话有登录用户时写 user（跨设备连续、避免设备共享泄漏）；
    纯设备会话没有 user_id，才落到 device；部门/全局是兜底。
    """

    user = _as_int_id(user_id)
    device = _as_int_id(device_id)
    dept = _as_int_id(dept_id)
    if user > 0:
        return "user", user
    if device > 0:
        return "device", device
    if dept > 0:
        return "dept", dept
    return "global", 0


def _memory_from_item(item: Any) -> Optional[MemoryItem]:
    if isinstance(item, str):
        text = item.strip()
        return MemoryItem(content=text) if text else None
    if not isinstance(item, Mapping):
        return None
    content = (
        item.get("content")
        or item.get("text")
        or item.get("memory")
        or item.get("summary")
        or ""
    )
    text = str(content).strip()
    if not text:
        return None
    if len(text) > MAX_MEMORY_CONTENT_CHARS:
        text = text[:MAX_MEMORY_CONTENT_CHARS] + "…"
    return MemoryItem(
        content=text,
        score=_as_float(item.get("score") or item.get("similarity")),
        kind=str(item.get("kind") or item.get("type") or ""),
        owner=str(
            item.get("owner")
            or item.get("owner_type")
            or item.get("owner_key")
            or ""
        ),
        memory_id=str(item.get("id") or item.get("memory_id") or ""),
        created_at=str(
            item.get("created_at")
            or item.get("created_dt")
            or item.get("updated_at")
            or ""
        ),
        importance=int(item.get("importance") or 0),
        raw=dict(item),
    )


def _extract_memories(payload: Any) -> List[MemoryItem]:
    """兼容 ``{memories:[]}`` / ``{data:{memories:[]}}`` / ``[]``。"""

    candidates: Sequence[Any] = ()
    if isinstance(payload, Mapping):
        for key in ("memories", "items", "results", "list"):
            value = payload.get(key)
            if isinstance(value, Sequence) and not isinstance(value, (str, bytes)):
                candidates = value
                break
        else:
            data = payload.get("data")
            if isinstance(data, Mapping):
                return _extract_memories(data)
            if isinstance(data, Sequence) and not isinstance(data, (str, bytes)):
                candidates = data
    elif isinstance(payload, Sequence) and not isinstance(payload, (str, bytes)):
        candidates = payload

    out: List[MemoryItem] = []
    for item in candidates:
        memory = _memory_from_item(item)
        if memory is not None:
            out.append(memory)
    return out


class MemoryClient:
    """长期记忆读写客户端（失败不阻塞音频）。"""

    def __init__(
        self,
        base_url: str,
        token: str,
        *,
        context_header: str = "",
        session_id: str = "",
        timeout: float = 4.0,
    ) -> None:
        self.base_url = (base_url or "").strip().rstrip("/")
        self.token = (token or "").strip()
        self.context_header = context_header
        self.session_id = session_id
        self.timeout = float(timeout)

    @property
    def enabled(self) -> bool:
        return bool(self.base_url and self.token)

    def _headers(self) -> Dict[str, str]:
        headers = {"Authorization": f"Bearer {self.token}"}
        if self.context_header:
            headers["X-Voice-Context"] = self.context_header
        if self.session_id:
            headers["X-Pipecat-Session-Id"] = self.session_id
        return headers

    async def _post(self, path: str, payload: Mapping[str, Any]) -> Any:
        if not self.enabled:
            raise MemoryClientError("记忆接口未配置（缺少内部地址或 token）")
        try:
            return await asyncio.to_thread(
                _post_json,
                join_url(self.base_url, path),
                payload,
                headers=self._headers(),
                timeout=self.timeout,
            )
        except InternalHTTPError as exc:
            raise MemoryClientError(str(exc)) from exc
        except Exception as exc:
            raise MemoryClientError(f"记忆接口请求失败：{type(exc).__name__}") from exc

    async def retrieve(
        self,
        *,
        session_id: str = "",
        user_id: Any = 0,
        device_id: Any = 0,
        dept_id: Any = 0,
        query: str = "",
        top_k: int = 5,
        owner_keys: Optional[Sequence[str]] = None,
    ) -> List[MemoryItem]:
        """检索 top-k。

        Go 的 ``MemoryRetrieveReq`` 要求 ``owner_keys``（``user:1`` /
        ``device:2`` / ``dept:10`` / ``global``），身份来自签名上下文；
        这里从已验证的 user/device/dept 推导。
        """

        keys = list(owner_keys or []) or build_owner_keys(
            user_id=user_id, device_id=device_id, dept_id=dept_id
        )
        if not keys:
            raise MemoryClientError("owner_keys 不能为空")
        payload = {
            "owner_keys": keys,
            "query": query or "",
            "top_k": max(1, min(int(top_k), 50)),
        }
        result = await self._post(RETRIEVE_PATH, payload)
        memories = _extract_memories(result)
        logger.info(
            "长期记忆检索完成 session=%s owners=%d count=%d",
            session_id,
            len(keys),
            len(memories),
        )
        return memories

    async def write(
        self,
        *,
        session_id: str = "",
        summary: str = "",
        user_id: Any = 0,
        device_id: Any = 0,
        dept_id: Any = 0,
        owner_type: str = "",
        owner_id: int = 0,
        kind: str = "summary",
        importance: int = 3,
        turn_id: str = "",
        turns: Optional[Sequence[Mapping[str, Any]]] = None,
        message_count: int = 0,
        source: str = "pipecat",
    ) -> bool:
        """写一条长期记忆（会话摘要）。

        Go 的 ``MemoryWriteReq`` 必填 ``owner_type`` + ``content``；
        ``owner_type`` 缺省时按 设备 > 用户 > 部门 > global 推导。
        ``turns`` / ``message_count`` 作为扩展字段一起发送（Go 会忽略未知
        字段），供后续写入逐轮记忆时使用。
        """

        content = str(summary or "").strip()
        if not content:
            raise MemoryClientError("content 不能为空")
        if not owner_type:
            owner_type, owner_id = primary_owner(
                user_id=user_id, device_id=device_id, dept_id=dept_id
            )
        payload: Dict[str, Any] = {
            "owner_type": str(owner_type).strip().lower(),
            "owner_id": int(owner_id or 0),
            "kind": kind or "summary",
            "content": content,
            "importance": max(1, min(int(importance or 3), 5)),
            "source_session_id": session_id,
            "turn_id": turn_id or "",
            # 扩展字段（当前 Go 接口忽略；保留为后续逐轮写入的兼容载荷）。
            "turns": [
                {
                    "role": str(turn.get("role") or ""),
                    "content": str(turn.get("content") or "")[:MAX_PROMPT_CHARS],
                }
                for turn in (turns or [])
            ],
            "message_count": int(message_count),
            "source": source,
        }
        await self._post(WRITE_PATH, payload)
        logger.info(
            "长期记忆写入完成 session=%s owner=%s:%d messages=%d summary_chars=%d",
            session_id,
            payload["owner_type"],
            payload["owner_id"],
            message_count,
            len(content),
        )
        return True

    async def close(self) -> None:
        """无持久连接，保留接口便于统一清理。"""

        return None


__all__ = (
    "MAX_PROMPT_CHARS",
    "MemoryClient",
    "MemoryClientError",
    "MemoryItem",
    "RETRIEVE_PATH",
    "WRITE_PATH",
    "build_owner_keys",
    "primary_owner",
)
