"""VAD → STT → LLM → TTS 装配。

控制面只负责**按当前配置生成 pipeline 描述并校验**；音频桥接尚未接通
（见 ``transport.py``），因此这里不实例化、不启动 Pipecat 的 processor 任务。
这样做的两个好处：

- ``/session/start`` 秒级返回，不会因为加载 Whisper 权重而阻塞；
- 依赖缺失/参数不全时能在接音频之前就给出明确的中文错误。

等 ``transport.py`` 的 RTP 收发联调完成，在 :func:`build_pipeline` 的末尾
按 ``stages`` 顺序构造 ``Pipeline([...])`` 即可，配置映射不需要再改 ——
这正是设计 §H+「层次 2：会话热重建」的落点。
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Any, Dict, Iterable, List, Mapping, Optional, Tuple

import factory
from config_store import redact_dict

STAGE_ORDER: Tuple[str, ...] = ("vad", "stt", "llm", "tts")

# 内部参数名 → 配置键（与 backend voice_agent_service.go 的键名一一对应）。
PARAM_SOURCES: Dict[str, Dict[str, str]] = {
    "stt": {
        "model": "stt_model",
        "language": "stt_language",
        "endpoint": "stt_endpoint",
        "api_key": "stt_api_key",
    },
    "llm": {
        "model": "llm_model",
        "endpoint": "llm_endpoint",
        "api_key": "llm_api_key",
        "system_prompt": "llm_system_prompt",
        "temperature": "llm_temperature",
        "max_tokens": "llm_max_tokens",
    },
    "tts": {
        "voice": "tts_voice",
        "endpoint": "tts_endpoint",
        "api_key": "tts_api_key",
        "speed": "tts_speed",
    },
}

# 需要转成数值的参数（配置统一是字符串）。
NUMERIC_PARAMS: Dict[str, Dict[str, type]] = {
    "llm": {"temperature": float, "max_tokens": int},
    "tts": {"speed": float},
}

# vad_params 里已知的数值字段。
VAD_NUMERIC_KEYS = frozenset({"threshold", "min_silence_ms", "aggressiveness", "min_speech_ms"})


class PipelineConfigError(ValueError):
    """pipeline 装配失败（HTTP 层映射为 400 + 中文原因）。"""


@dataclass(frozen=True)
class PipelineBundle:
    """一个会话装配结果的快照。"""

    session_id: str
    config_revision: int
    mode: str
    stages: Tuple[Dict[str, Any], ...]
    options: Mapping[str, Any]

    @property
    def stage_names(self) -> List[str]:
        return [f"{stage['stage']}:{stage['provider']}" for stage in self.stages]

    def describe(self) -> Dict[str, Any]:
        return {
            "session_id": self.session_id,
            "config_revision": self.config_revision,
            "mode": self.mode,
            "pipeline": self.stage_names,
            "stages": [dict(stage) for stage in self.stages],
            "options": dict(self.options),
            "audio_bridge": "not_connected",
        }


def _as_bool(value: Any, default: bool = False) -> bool:
    if isinstance(value, bool):
        return value
    text = str(value or "").strip().lower()
    if text in {"1", "true", "yes", "on"}:
        return True
    if text in {"0", "false", "no", "off"}:
        return False
    return default


def _as_int(value: Any, default: int) -> int:
    try:
        return int(str(value).strip())
    except (TypeError, ValueError):
        return default


def _coerce_numeric(category: str, name: str, raw: Any) -> Any:
    caster = NUMERIC_PARAMS.get(category, {}).get(name)
    if caster is None:
        return raw
    try:
        return caster(str(raw).strip())
    except (TypeError, ValueError) as exc:
        source = PARAM_SOURCES[category].get(name, name)
        raise PipelineConfigError(f"配置项 {source} 必须是数字，当前值 {raw!r}") from exc


def _collect_params(category: str, config: Mapping[str, Any]) -> Dict[str, Any]:
    """把扁平配置映射为 provider 的构造参数。"""

    if category == "vad":
        raw = str(config.get("vad_params") or "").strip()
        if not raw:
            return {}
        try:
            parsed = json.loads(raw)
        except json.JSONDecodeError as exc:
            raise PipelineConfigError(f"vad_params 不是合法 JSON：{exc}") from exc
        if not isinstance(parsed, dict):
            raise PipelineConfigError("vad_params 必须是 JSON 对象，例如 {\"threshold\":0.5}")
        params: Dict[str, Any] = {}
        for key, value in parsed.items():
            if not isinstance(key, str):
                raise PipelineConfigError("vad_params 的键必须是字符串")
            if key in VAD_NUMERIC_KEYS:
                try:
                    params[key] = float(value) if key == "threshold" else int(value)
                except (TypeError, ValueError) as exc:
                    raise PipelineConfigError(
                        f"vad_params.{key} 必须是数字，当前值 {value!r}"
                    ) from exc
            else:
                params[key] = value
        return params

    params = {}
    for target, source in PARAM_SOURCES[category].items():
        raw = config.get(source)
        if raw is None or str(raw).strip() == "":
            continue
        params[target] = _coerce_numeric(category, target, raw)
    return params


def _missing_params(spec: factory.ProviderSpec, params: Mapping[str, Any]) -> List[str]:
    missing = []
    for name in spec.required_params:
        value = params.get(name)
        if value is None or (isinstance(value, str) and not value.strip()):
            missing.append(PARAM_SOURCES.get(spec.category, {}).get(name, name))
    return missing


def build_pipeline(
    config: Mapping[str, Any],
    session_id: str,
    mode: str = "half_duplex",
    config_revision: int = 0,
) -> PipelineBundle:
    """按配置装配 pipeline 描述。

    抛 :class:`PipelineConfigError` 的场景（均由 ``/session/start`` 转为 400）：

    - provider 未配置或未知；
    - provider 必填参数缺失（如云端 provider 没有 API Key）；
    - provider 的 Python 依赖未安装（如 ``faster-whisper``）。
    """

    if not session_id.strip():
        raise PipelineConfigError("session_id 不能为空")
    if mode not in {"half_duplex", "full_duplex", "ptt"}:
        raise PipelineConfigError(
            f"不支持的会话模式 {mode!r}；可选 half_duplex / full_duplex / ptt"
        )

    stages: List[Dict[str, Any]] = []
    for category in STAGE_ORDER:
        provider = str(config.get(f"{category}_provider") or "").strip().lower()
        if not provider:
            raise PipelineConfigError(f"未配置 {category}_provider，无法装配 pipeline")

        try:
            spec = factory.get_provider(category, provider)
        except factory.UnknownProviderError as exc:
            raise PipelineConfigError(str(exc)) from exc

        params = _collect_params(category, config)
        missing = _missing_params(spec, params)
        if missing:
            raise PipelineConfigError(
                f"{category.upper()} provider {provider} 缺少必填参数：{'、'.join(missing)}"
            )

        probe = factory.probe(spec)
        if not probe.installed:
            raise PipelineConfigError(
                f"{category.upper()} provider {provider} 依赖未安装：{probe.detail}"
            )

        stages.append(
            {
                "stage": category,
                "provider": provider,
                "service": probe.resolved,
                # 描述里同样脱敏，避免 API Key 出现在 /session/start 响应中。
                "params": redact_dict(params),
                "installed": True,
            }
        )

    # 回声消除结论（设计 §8.1.1）：默认半双工。只有显式请求 full_duplex
    # 且配置允许时才打开 barge_in；ptt 模式本就没有打断语义。
    barge_in = mode == "full_duplex" and _as_bool(config.get("barge_in"), False)

    options = {
        "barge_in": barge_in,
        "half_duplex": mode == "half_duplex",
        "session_idle_timeout": _as_int(config.get("session_idle_timeout"), 60),
        "max_session_sec": _as_int(config.get("max_session_sec"), 600),
        "audio_bridge": "not_connected",
    }

    return PipelineBundle(
        session_id=session_id,
        config_revision=config_revision,
        mode=mode,
        stages=tuple(stages),
        options=options,
    )


def summary(bundle: PipelineBundle) -> str:
    """日志用的一行摘要（不含密钥）。"""

    return " → ".join(bundle.stage_names)


__all__: Iterable[str] = (
    "PARAM_SOURCES",
    "PipelineBundle",
    "PipelineConfigError",
    "STAGE_ORDER",
    "build_pipeline",
    "summary",
)
