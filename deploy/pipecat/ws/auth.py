"""WebSocket 语音鉴权：共享 token + Go 注入的签名上下文。

架构上 Pipecat 不直接暴露公网，`/ws/voice` 的连接来自 Go 网关。网关负责
用户/设备鉴权，然后带上：

```http
Authorization: Bearer <PIPECAT_TOKEN>
X-Voice-Context: <base64url(payload)>.<base64url(HMAC-SHA256(secret, base64url(payload)))>
```

``payload`` 是 UTF-8 JSON：

```json
{"session_id":"s1","user_id":"u1","device_id":"d1","dept_id":"10",
 "scopes":["voice"],"exp":1760000000}
```

Pipecat **只信任签名内容**：客户端自己塞进查询参数或消息体里的
``user_id`` 一律忽略。``PIPECAT_CONTEXT_SECRET`` 未配置时，带上下文的
连接会被拒绝（无法验证签名）；本地调试可以只带 token（上下文标记为
未验证，能力受限）。

## 兼容格式

除 ``payload.sig`` 外，也接受 ``v1.payload.sig``（签名对象仍是 payload
段）。签名使用 base64url（无 padding）编码。
"""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import logging
import os
import time
import uuid
from dataclasses import dataclass, field
from typing import Any, Mapping, Optional, Sequence, Tuple

logger = logging.getLogger("ai-camera-pipecat.ws.auth")

WS_CLOSE_UNAUTHORIZED = 4401
WS_CLOSE_FORBIDDEN = 4403
WS_CLOSE_BAD_REQUEST = 4400


class AuthError(Exception):
    """鉴权失败（HTTP/WS 层映射为 401/403 + 关闭码）。"""

    def __init__(
        self,
        code: str,
        message: str,
        *,
        status_code: int = 401,
        close_code: int = WS_CLOSE_UNAUTHORIZED,
    ) -> None:
        super().__init__(message)
        self.code = code
        self.message = message
        self.status_code = status_code
        self.close_code = close_code


@dataclass(frozen=True)
class VoiceContext:
    """签名上下文（只来自 Go 网关）。"""

    session_id: str
    user_id: str = ""
    device_id: str = ""
    dept_id: str = ""
    scopes: Tuple[str, ...] = ()
    exp: int = 0
    client_kind: str = ""
    verified: bool = False
    raw: Mapping[str, Any] = field(default_factory=dict, repr=False)

    def has_scope(self, scope: str) -> bool:
        return not self.scopes or scope in self.scopes


@dataclass(frozen=True)
class AuthResult:
    token_valid: bool
    context: VoiceContext
    context_header: str = ""  # 原样转发给 Go 内部接口，避免重复签名
    context_verified: bool = False

    @property
    def session_id(self) -> str:
        return self.context.session_id


# ════════════════════════════════════════════════════════════════
# base64url
# ════════════════════════════════════════════════════════════════


def b64url_encode(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def b64url_decode(text: str) -> bytes:
    raw = str(text or "").strip()
    if not raw:
        return b""
    padding = "=" * (-len(raw) % 4)
    try:
        return base64.urlsafe_b64decode(raw + padding)
    except Exception as exc:
        raise AuthError("invalid_context", "上下文 base64url 解码失败", status_code=401) from exc


# ════════════════════════════════════════════════════════════════
# 签名/验签
# ════════════════════════════════════════════════════════════════


def sign_context(payload: Mapping[str, Any], secret: str) -> str:
    """生成 ``payload.sig`` 形式的上下文（测试/文档/网关参考实现）。"""

    if not secret:
        raise ValueError("secret 不能为空")
    body = json.dumps(payload, ensure_ascii=False, separators=(",", ":"), sort_keys=True)
    token = b64url_encode(body.encode("utf-8"))
    signature = hmac.new(secret.encode("utf-8"), token.encode("ascii"), hashlib.sha256).digest()
    return f"{token}.{b64url_encode(signature)}"


def _split_context(value: str) -> Tuple[str, str]:
    parts = [part for part in str(value or "").strip().split(".") if part]
    if len(parts) == 3 and parts[0].lower() in ("v1", "1"):
        parts = parts[1:]
    if len(parts) != 2:
        raise AuthError(
            "invalid_context",
            "X-Voice-Context 格式应为 base64url(payload).base64url(signature)",
        )
    return parts[0], parts[1]


def parse_voice_context(
    header_value: str,
    secret: str,
    *,
    now: Optional[float] = None,
) -> VoiceContext:
    """验证并解析 ``X-Voice-Context``。

    只返回签名覆盖的字段；``exp`` 为 0 表示不过期。
    """

    if not secret:
        raise AuthError(
            "context_secret_missing",
            "服务端未配置 PIPECAT_CONTEXT_SECRET，拒绝未签名的上下文",
            status_code=401,
        )

    token, signature = _split_context(header_value)
    try:
        supplied = b64url_decode(signature)
    except AuthError as exc:
        raise AuthError("invalid_context", "上下文签名格式非法") from exc

    # 标准格式：对 base64url(payload) 字符串签名；同时兼容 Go 侧直接对
    # 原始 JSON 字节签名的实现（两者都由同一密钥保护）。
    candidates = [
        hmac.new(secret.encode("utf-8"), token.encode("ascii"), hashlib.sha256).digest()
    ]
    try:
        candidates.append(
            hmac.new(secret.encode("utf-8"), b64url_decode(token), hashlib.sha256).digest()
        )
    except AuthError:
        pass
    if not any(hmac.compare_digest(expected, supplied) for expected in candidates):
        raise AuthError("invalid_context", "上下文签名校验失败", status_code=403, close_code=WS_CLOSE_FORBIDDEN)

    try:
        data = json.loads(b64url_decode(token).decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise AuthError("invalid_context", "上下文 payload 不是合法 JSON") from exc
    if not isinstance(data, Mapping):
        raise AuthError("invalid_context", "上下文 payload 必须是 JSON 对象")

    session_id = str(data.get("session_id") or data.get("sid") or "").strip()
    if not session_id:
        raise AuthError("invalid_context", "上下文缺少 session_id", status_code=403, close_code=WS_CLOSE_FORBIDDEN)

    try:
        exp = int(data.get("exp") or 0)
    except (TypeError, ValueError):
        exp = 0
    current = time.time() if now is None else float(now)
    if exp > 0 and exp < current:
        raise AuthError("context_expired", "上下文已过期", status_code=401)

    scopes_raw = data.get("scopes") or ()
    if isinstance(scopes_raw, str):
        scopes: Sequence[str] = tuple(
            item for item in scopes_raw.replace(",", " ").split() if item
        )
    elif isinstance(scopes_raw, (list, tuple)):
        scopes = tuple(str(item) for item in scopes_raw if str(item).strip())
    else:
        scopes = ()

    return VoiceContext(
        session_id=session_id,
        user_id=str(data.get("user_id") or "").strip(),
        device_id=str(data.get("device_id") or "").strip(),
        dept_id=str(data.get("dept_id") or data.get("dept") or "").strip(),
        scopes=scopes,
        exp=exp,
        client_kind=str(data.get("client_kind") or data.get("kind") or "").strip(),
        verified=True,
        raw=dict(data),
    )


# ════════════════════════════════════════════════════════════════
# 请求鉴权
# ════════════════════════════════════════════════════════════════


def _header(headers: Mapping[str, Any], name: str) -> str:
    """大小写不敏感地取 header（Starlette Headers 本身已不区分）。"""

    value = headers.get(name)
    if value is None:
        lowered = name.lower()
        for key, candidate in headers.items():
            if str(key).lower() == lowered:
                value = candidate
                break
    if value is None:
        return ""
    return str(value).strip()


def _query(query: Mapping[str, Any], name: str) -> str:
    value = query.get(name)
    if value is None:
        return ""
    if isinstance(value, (list, tuple)):
        value = value[0] if value else ""
    return str(value).strip()


def extract_bearer(headers: Mapping[str, Any]) -> str:
    """从 Authorization 或 X-Pipecat-Token 取 token。"""

    authorization = _header(headers, "authorization")
    if authorization:
        scheme, _, credential = authorization.partition(" ")
        if scheme.strip().lower() == "bearer":
            return credential.strip()
    return _header(headers, "x-pipecat-token")


def authenticate(
    headers: Mapping[str, Any],
    query: Optional[Mapping[str, Any]],
    *,
    expected_token: str,
    context_secret: str,
    now: Optional[float] = None,
    allow_query_token: Optional[bool] = None,
) -> AuthResult:
    """校验共享 token 与签名上下文。

    - token 不匹配 → 401（``AuthError``）；
    - 配置了 secret 但缺上下文 → 401；
    - 配置了 secret 且上下文验签失败 → 403；
    - 未配置 secret：允许 token-only 连接，``context_verified=False``，
      仅供本地调试；生产必须配置 secret。
    """

    if allow_query_token is None:
        allow_query_token = os.environ.get("PIPECAT_ALLOW_QUERY_TOKEN", "").strip().lower() in {
            "1",
            "true",
            "yes",
            "on",
        }

    supplied = extract_bearer(headers)
    if not supplied and allow_query_token and query is not None:
        supplied = _query(query, "token")
    if not supplied or not expected_token or not hmac.compare_digest(supplied, expected_token):
        raise AuthError("unauthorized", "语音 WebSocket token 无效")

    header_value = _header(headers, "x-voice-context")
    if context_secret:
        if not header_value:
            raise AuthError(
                "missing_context",
                "缺少 X-Voice-Context：Pipecat 只接受 Go 网关签名的上下文",
            )
        context = parse_voice_context(header_value, context_secret, now=now)
        if context.scopes and "voice" not in context.scopes:
            raise AuthError(
                "forbidden",
                "签名上下文缺少 voice scope",
                status_code=403,
                close_code=WS_CLOSE_FORBIDDEN,
            )
        return AuthResult(
            token_valid=True,
            context=context,
            context_header=header_value,
            context_verified=True,
        )

    if header_value:
        raise AuthError(
            "context_secret_missing",
            "收到 X-Voice-Context 但服务端未配置 PIPECAT_CONTEXT_SECRET，拒绝未验证上下文",
        )

    logger.warning(
        "PIPECAT_CONTEXT_SECRET 未配置：仅校验共享 token，语音会话上下文未经验证（仅限本地调试）"
    )
    session_id = _query(query or {}, "session_id") or f"ws-{uuid.uuid4().hex[:16]}"
    return AuthResult(
        token_valid=True,
        context=VoiceContext(session_id=session_id, verified=False),
        context_header="",
        context_verified=False,
    )


__all__ = (
    "AuthError",
    "AuthResult",
    "VoiceContext",
    "WS_CLOSE_BAD_REQUEST",
    "WS_CLOSE_FORBIDDEN",
    "WS_CLOSE_UNAUTHORIZED",
    "authenticate",
    "b64url_decode",
    "b64url_encode",
    "extract_bearer",
    "parse_voice_context",
    "sign_context",
)
