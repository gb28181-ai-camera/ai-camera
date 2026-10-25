"""WS 鉴权：token + HMAC 上下文。"""

from __future__ import annotations

import json
import time

import pytest

from ws import auth


SECRET = "test-context-secret"
TOKEN = "test-token-1234567890"


def _headers(context: str | None = None, token: str = TOKEN):
    headers = {"Authorization": f"Bearer {token}"}
    if context is not None:
        headers["X-Voice-Context"] = context
    return headers


def _context(**overrides):
    payload = {
        "session_id": "s1",
        "user_id": "u1",
        "device_id": "d1",
        "dept_id": "10",
        "scopes": ["voice", "device.read"],
        "exp": int(time.time()) + 60,
    }
    payload.update(overrides)
    return auth.sign_context(payload, SECRET)


def test_sign_and_authenticate():
    result = auth.authenticate(
        _headers(_context()), {}, expected_token=TOKEN, context_secret=SECRET
    )
    assert result.token_valid is True
    assert result.context_verified is True
    assert result.context.session_id == "s1"
    assert result.context.user_id == "u1"
    assert result.context.dept_id == "10"
    assert result.context.has_scope("voice")
    assert not result.context.has_scope("admin")
    assert result.context_header


def test_v1_prefix_is_accepted():
    header = "v1." + _context()
    result = auth.authenticate(
        _headers(header), None, expected_token=TOKEN, context_secret=SECRET
    )
    assert result.context.session_id == "s1"


def test_signature_over_raw_json_also_accepted():
    """兼容 Go 直接对原始 JSON 字节签名的实现。"""

    import hashlib
    import hmac as hmac_mod

    payload = {"session_id": "s-raw", "user_id": "u1", "exp": int(time.time()) + 60}
    raw = json.dumps(payload, separators=(",", ":")).encode()
    token = auth.b64url_encode(raw)
    signature = hmac_mod.new(SECRET.encode(), raw, hashlib.sha256).digest()
    header = f"{token}.{auth.b64url_encode(signature)}"
    result = auth.authenticate(
        _headers(header), {}, expected_token=TOKEN, context_secret=SECRET
    )
    assert result.context.session_id == "s-raw"


def test_tampered_payload_rejected():
    token_part, signature = _context().split(".")
    tampered = auth.b64url_encode(
        json.dumps(
            {"session_id": "s1", "user_id": "admin"}, separators=(",", ":")
        ).encode()
    )
    with pytest.raises(auth.AuthError) as exc:
        auth.authenticate(
            _headers(f"{tampered}.{signature}"),
            {},
            expected_token=TOKEN,
            context_secret=SECRET,
        )
    assert exc.value.code == "invalid_context"
    assert exc.value.status_code == 403


def test_expired_context_rejected():
    context = _context(exp=int(time.time()) - 1)
    with pytest.raises(auth.AuthError) as exc:
        auth.authenticate(
            _headers(context), {}, expected_token=TOKEN, context_secret=SECRET
        )
    assert exc.value.code == "context_expired"


def test_missing_context_when_secret_configured():
    with pytest.raises(auth.AuthError) as exc:
        auth.authenticate(
            _headers(), {}, expected_token=TOKEN, context_secret=SECRET
        )
    assert exc.value.code == "missing_context"


def test_context_without_secret_rejected():
    with pytest.raises(auth.AuthError) as exc:
        auth.authenticate(
            _headers(_context()), {}, expected_token=TOKEN, context_secret=""
        )
    assert exc.value.code == "context_secret_missing"


def test_token_only_dev_mode():
    result = auth.authenticate(
        _headers(), {"session_id": "dev-1"}, expected_token=TOKEN, context_secret=""
    )
    assert result.context_verified is False
    assert result.context.session_id == "dev-1"


def test_bad_token_rejected():
    with pytest.raises(auth.AuthError) as exc:
        auth.authenticate(
            _headers(_context(), token="wrong"), {}, expected_token=TOKEN, context_secret=SECRET
        )
    assert exc.value.code == "unauthorized"
    assert exc.value.status_code == 401


def test_query_token_only_when_enabled():
    with pytest.raises(auth.AuthError):
        auth.authenticate(
            {},
            {"token": TOKEN},
            expected_token=TOKEN,
            context_secret="",
            allow_query_token=False,
        )
    result = auth.authenticate(
        {},
        {"token": TOKEN, "session_id": "q1"},
        expected_token=TOKEN,
        context_secret="",
        allow_query_token=True,
    )
    assert result.context.session_id == "q1"


def test_x_pipecat_token_header_supported():
    result = auth.authenticate(
        {"X-Pipecat-Token": TOKEN},
        {},
        expected_token=TOKEN,
        context_secret="",
    )
    assert result.token_valid is True


def test_context_without_voice_scope_rejected():
    context = _context(scopes=["admin"])
    with pytest.raises(auth.AuthError) as exc:
        auth.authenticate(
            _headers(context), {}, expected_token=TOKEN, context_secret=SECRET
        )
    assert exc.value.code == "forbidden"
    assert exc.value.status_code == 403


def test_empty_scopes_allowed_for_compat():
    context = _context(scopes=[])
    result = auth.authenticate(
        _headers(context), {}, expected_token=TOKEN, context_secret=SECRET
    )
    assert result.context.session_id == "s1"


def test_malformed_context():
    with pytest.raises(auth.AuthError):
        auth.authenticate(
            _headers("not-a-valid-context"),
            {},
            expected_token=TOKEN,
            context_secret=SECRET,
        )
