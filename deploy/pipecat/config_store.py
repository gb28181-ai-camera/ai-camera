"""语音 Agent 控制面的内存配置存储。

设计要点（设计文档 §8.2 / §H+）：

1. **配置动态、会话不变**：``replace()`` 只替换内存中的配置快照，不触碰
   已经装配好的会话。下一个 ``/session/start`` 读取新快照，即「下一个会话生效」。
2. **原子替换**：整个 dict 在锁内构建完成后再一次性赋值，读端永远看不到
   半更新的状态。
3. **密钥只写不读**：所有读出口（``GET /config``、``/config`` 响应、``/health``）
   都经过 :func:`redact_dict`；敏感键返回 ``****``，不会回显真实值。
4. **脱敏串不回写**：后端读配置时拿到的是 ``****``；若客户端误把脱敏串推回来，
   敏感键的 ``****`` 值会被忽略，保留原值（与后端 ``saveThirdParty`` 语义一致）。
"""

from __future__ import annotations

import threading
import time
from dataclasses import dataclass, field
from typing import Any, Dict, Iterable, Mapping

# 与 backend/internal/service/setting/voice_agent_service.go 保持一致。
SECRET_KEYS = frozenset(
    {
        "stt_api_key",
        "llm_api_key",
        "tts_api_key",
        "pipecat_token",
    }
)

# 敏感键的通用判定：显式清单之外，再按命名兜底，避免以后新增密钥键漏脱敏。
_SECRET_MARK = "****"


def is_secret_key(key: str) -> bool:
    """判断配置键是否敏感（读取时必须脱敏）。"""

    k = key.strip().lower()
    if k in SECRET_KEYS:
        return True
    return (
        k.endswith("_api_key")
        or k.endswith("_token")
        or k.endswith("_secret")
        or k in {"token", "password", "secret", "api_key"}
    )


def mask_value(value: Any) -> str:
    """敏感值脱敏：有值返回 ``****``，空值返回空串。"""

    text = "" if value is None else str(value)
    return _SECRET_MARK if text else ""


def redact_dict(values: Mapping[str, Any]) -> Dict[str, str]:
    """返回脱敏后的配置副本（不修改入参）。"""

    out: Dict[str, str] = {}
    for key, value in values.items():
        if is_secret_key(key):
            out[key] = mask_value(value)
        else:
            out[key] = "" if value is None else str(value)
    return out


def is_masked(value: Any) -> bool:
    """判断值是否为脱敏串（后端展示值 ``****``）。"""

    return _SECRET_MARK in str(value or "")


# 设计 §8.2「默认配置（零外部密钥）」：本地 Whisper + Anthropic + Edge TTS。
# 这些默认值只在内存中补足；后端下发的同名键优先。
DEFAULT_CONFIG: Dict[str, str] = {
    "stt_provider": "local_whisper",
    "stt_model": "base",
    "stt_language": "zh",
    "stt_endpoint": "",
    "stt_api_key": "",
    "llm_provider": "anthropic",
    "llm_model": "claude-sonnet-5-5",
    "llm_endpoint": "",
    "llm_api_key": "",
    "llm_system_prompt": "",
    "llm_temperature": "0.7",
    "llm_max_tokens": "1024",
    "tts_provider": "edge_tts",
    "tts_voice": "zh-CN-XiaoxiaoNeural",
    "tts_endpoint": "",
    "tts_api_key": "",
    "tts_speed": "1.0",
    "vad_provider": "silero",
    "vad_params": '{"threshold":0.5,"min_silence_ms":300}',
    "barge_in": "false",
    "session_idle_timeout": "60",
    "max_session_sec": "600",
}

# 允许的键名：小写下划线。未知但合法的键会被接受（provider 的 extra_params 透传），
# 非法键直接 400，避免把任意字符串塞进配置。
_KEY_MAX_LEN = 64


class ConfigValidationError(ValueError):
    """配置不合法（HTTP 层映射为 400）。"""


@dataclass(frozen=True)
class ConfigUpdateResult:
    """一次配置替换的结果。"""

    revision: int
    updated_at: float
    applied_keys: tuple[str, ...] = ()
    ignored_masked_keys: tuple[str, ...] = ()
    unknown_keys: tuple[str, ...] = ()

    def as_dict(self) -> Dict[str, Any]:
        return {
            "revision": self.revision,
            "updated_at": self.updated_at,
            "applied_keys": list(self.applied_keys),
            "ignored_masked_keys": list(self.ignored_masked_keys),
            "unknown_keys": list(self.unknown_keys),
            "key_count": len(self.applied_keys),
        }


@dataclass
class ConfigStore:
    """线程安全的配置快照存储。"""

    defaults: Mapping[str, str] = field(default_factory=lambda: dict(DEFAULT_CONFIG))
    _lock: threading.Lock = field(default_factory=threading.Lock, repr=False)
    _values: Dict[str, str] = field(init=False, repr=False)
    _revision: int = field(default=0, init=False)
    _updated_at: float = field(default=0.0, init=False)

    def __post_init__(self) -> None:
        self._values = dict(self.defaults)

    # ── 读 ────────────────────────────────────────────────────────
    def snapshot(self) -> Dict[str, str]:
        """返回配置快照（含真实密钥，仅供内部装配/测试，不直接出 HTTP）。"""

        with self._lock:
            return dict(self._values)

    def redacted(self) -> Dict[str, str]:
        """返回脱敏后的配置（HTTP 读接口使用）。"""

        return redact_dict(self.snapshot())

    def snapshot_with_meta(self) -> tuple[Dict[str, str], int, float]:
        """一次加锁返回 (配置快照, revision, updated_at)，避免读到中间状态。"""

        with self._lock:
            return dict(self._values), self._revision, self._updated_at

    def redacted_with_meta(self) -> Dict[str, Any]:
        """脱敏配置 + 版本信息（``GET /config`` 的 data 部分）。"""

        values, revision, updated_at = self.snapshot_with_meta()
        return {
            "config": redact_dict(values),
            "revision": revision,
            "updated_at": updated_at,
        }

    @property
    def revision(self) -> int:
        with self._lock:
            return self._revision

    @property
    def updated_at(self) -> float:
        with self._lock:
            return self._updated_at

    # ── 写 ────────────────────────────────────────────────────────
    def replace(self, payload: Mapping[str, Any]) -> ConfigUpdateResult:
        """校验并**原子替换**配置。

        - ``payload`` 必须是扁平的 ``str -> str`` 映射；
        - 敏感键收到含 ``****`` 的展示值时忽略（保留原值）；
        - 返回本次真正生效的键、被忽略的脱敏键与未知键。
        """

        if not isinstance(payload, Mapping):
            raise ConfigValidationError("配置必须是扁平的键值对象（map[string]string）")

        clean: Dict[str, str] = {}
        for raw_key, raw_value in payload.items():
            if not isinstance(raw_key, str):
                raise ConfigValidationError("配置键必须是字符串")
            key = raw_key.strip()
            if not key:
                raise ConfigValidationError("配置键不能为空")
            if len(key) > _KEY_MAX_LEN:
                raise ConfigValidationError(f"配置键过长（>{_KEY_MAX_LEN}）：{key[:32]}…")
            if not key.replace("_", "").isalnum() or not key[0].isalpha() or key != key.lower():
                raise ConfigValidationError(
                    f"配置键只允许小写字母、数字与下划线，且以字母开头：{key}"
                )
            if not isinstance(raw_value, str):
                raise ConfigValidationError(f"配置项 {key} 的值必须是字符串")
            clean[key] = raw_value

        now = time.time()
        with self._lock:
            merged = dict(self._values)
            applied: list[str] = []
            ignored_masked: list[str] = []
            unknown: list[str] = []

            for key, value in clean.items():
                if key not in merged:
                    unknown.append(key)
                if is_secret_key(key) and is_masked(value):
                    # 展示值不回写：密钥「只写不读」。
                    ignored_masked.append(key)
                    continue
                merged[key] = value
                applied.append(key)

            # 原子替换：锁内构建完 merged 后再整体赋值。
            self._values = merged
            self._revision += 1
            self._updated_at = now
            revision = self._revision

        return ConfigUpdateResult(
            revision=revision,
            updated_at=now,
            applied_keys=tuple(applied),
            ignored_masked_keys=tuple(ignored_masked),
            unknown_keys=tuple(unknown),
        )


__all__: Iterable[str] = (
    "ConfigStore",
    "ConfigUpdateResult",
    "ConfigValidationError",
    "DEFAULT_CONFIG",
    "SECRET_KEYS",
    "is_masked",
    "is_secret_key",
    "mask_value",
    "redact_dict",
)
