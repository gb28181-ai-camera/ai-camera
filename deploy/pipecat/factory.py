"""Provider 字符串 → Pipecat Service 的映射表。

设计文档 §8.2「免改代码加 provider」的落点：新增一个 provider 只需要在
``PROVIDERS`` 里加一行，控制面与 ``/test`` 自动感知。

## 可选导入

Pipecat 的服务按**可选依赖**组织：``pipecat-ai`` 本体很小，某个 provider 的
SDK（``faster-whisper`` / ``deepgram-sdk`` / ``azure-cognitiveservices-speech`` …）
没装时，导入对应模块会失败。这里一律用 ``importlib`` 动态探测并捕获异常：

- 控制面**不会**因为某个 provider 的依赖缺失而启动失败；
- ``/test`` 会明确返回「未安装」及缺失的模块名；
- 只有 ``/session/start`` 真正用到该 provider 时才 400 失败。

## 版本说明

服务类路径按 ``pipecat-ai==1.13.0`` 实测核对。``Pipecat`` 在 1.x 调整过
模块布局，因此每个 provider 都允许配置多个候选路径（``candidates``），
探测时按顺序尝试，避免升级小版本就整体失效。
"""

from __future__ import annotations

import importlib
import threading
from dataclasses import dataclass
from functools import lru_cache
from typing import Dict, Iterable, Mapping, Optional, Tuple


class UnknownProviderError(ValueError):
    """未知 provider：明确报错，不静默回退。"""

    def __init__(self, category: str, provider: str, available: Iterable[str]):
        self.category = category
        self.provider = provider
        self.available = tuple(available)
        options = "、".join(self.available) if self.available else "（无）"
        super().__init__(f"未知的 {category} provider：{provider!r}；可选：{options}")


@dataclass(frozen=True)
class ProviderSpec:
    """一个 provider 的静态描述（不含密钥）。"""

    category: str
    code: str
    # 形如 "module.path:ClassName"；空表示由本文件内的轻量适配器实现。
    candidates: Tuple[str, ...] = ()
    # 需要额外探测的 Python 模块（provider 的 SDK）。
    dependencies: Tuple[str, ...] = ()
    # 装配时必填的参数名（内部参数名，不是配置键）。
    required_params: Tuple[str, ...] = ()
    requires_api_key: bool = False
    requires_endpoint: bool = False
    local: bool = False
    default_endpoint: Optional[str] = None
    # 本文件内适配器的类名（candidates 为空时使用）。
    adapter: Optional[str] = None
    note: str = ""


class EdgeTTSAdapter:
    """Edge TTS 的轻量适配层（**不发起网络请求**）。

    设计 §8.2 的默认 TTS 是零密钥的 Edge TTS，但 ``pipecat-ai==1.13.0``
    没有内置 Edge TTS service。这里提供一个可导入的适配器用于参数校验与
    可用性探测；真正的 ``TTSService`` 封装（把 ``edge_tts.Communicate`` 的
    MP3 分片转成 Pipecat 的 TTSAudioRawFrame）等音频桥接落地时补齐。

    当前阶段绝不假装已经能合成：``/health`` 的 ``audio_bridge`` 恒为
    ``not_connected``。
    """

    provider = "edge_tts"

    def __init__(self, voice: str = "zh-CN-XiaoxiaoNeural", speed: float = 1.0, **_extra: object):
        self.voice = voice
        self.speed = speed


# 本文件内可导入的适配器：类名 -> 类。
ADAPTERS: Dict[str, type] = {"EdgeTTSAdapter": EdgeTTSAdapter}


# category -> code -> spec
PROVIDERS: Dict[str, Dict[str, ProviderSpec]] = {
    "stt": {
        "local_whisper": ProviderSpec(
            category="stt",
            code="local_whisper",
            candidates=("pipecat.services.whisper.stt:WhisperSTTService",),
            dependencies=("faster_whisper",),
            required_params=("model",),
            local=True,
            note="faster-whisper 本地识别，零外部计费",
        ),
        "openai": ProviderSpec(
            category="stt",
            code="openai",
            candidates=("pipecat.services.openai.stt:OpenAISTTService",),
            dependencies=("openai",),
            required_params=("model", "api_key"),
            requires_api_key=True,
            default_endpoint="https://api.openai.com/v1",
            note="OpenAI Whisper 云端识别",
        ),
        "deepgram": ProviderSpec(
            category="stt",
            code="deepgram",
            candidates=("pipecat.services.deepgram.stt:DeepgramSTTService",),
            dependencies=("deepgram",),
            required_params=("api_key",),
            requires_api_key=True,
            default_endpoint="https://api.deepgram.com",
            note="Deepgram 云端识别",
        ),
        "azure": ProviderSpec(
            category="stt",
            code="azure",
            candidates=("pipecat.services.azure.stt:AzureSTTService",),
            dependencies=("azure.cognitiveservices.speech",),
            required_params=("api_key", "endpoint"),
            requires_api_key=True,
            requires_endpoint=True,
            note="Azure 语音识别",
        ),
    },
    "llm": {
        "anthropic": ProviderSpec(
            category="llm",
            code="anthropic",
            candidates=("pipecat.services.anthropic.llm:AnthropicLLMService",),
            dependencies=("anthropic",),
            required_params=("api_key", "model"),
            requires_api_key=True,
            default_endpoint="https://api.anthropic.com",
            note="Anthropic Claude",
        ),
        "openai": ProviderSpec(
            category="llm",
            code="openai",
            candidates=("pipecat.services.openai.llm:OpenAILLMService",),
            dependencies=("openai",),
            required_params=("api_key", "model"),
            requires_api_key=True,
            default_endpoint="https://api.openai.com/v1",
            note="OpenAI GPT",
        ),
        "ollama": ProviderSpec(
            category="llm",
            code="ollama",
            candidates=("pipecat.services.ollama.llm:OLLamaLLMService",),
            dependencies=("ollama",),
            required_params=("model", "endpoint"),
            requires_endpoint=True,
            note="本地 Ollama（注意 Pipecat 类名拼写是 OLLama）",
        ),
    },
    "tts": {
        "edge_tts": ProviderSpec(
            category="tts",
            code="edge_tts",
            dependencies=("edge_tts",),
            required_params=("voice",),
            local=True,
            adapter="EdgeTTSAdapter",
            note="Edge TTS，零密钥；适配器待音频桥接完成后接入 Pipecat",
        ),
        "azure": ProviderSpec(
            category="tts",
            code="azure",
            candidates=("pipecat.services.azure.tts:AzureTTSService",),
            dependencies=("azure.cognitiveservices.speech",),
            required_params=("api_key", "voice"),
            requires_api_key=True,
            note="Azure 语音合成（区域参数可放在 tts_endpoint）",
        ),
        "elevenlabs": ProviderSpec(
            category="tts",
            code="elevenlabs",
            candidates=("pipecat.services.elevenlabs.tts:ElevenLabsTTSService",),
            dependencies=("elevenlabs",),
            required_params=("api_key", "voice"),
            requires_api_key=True,
            default_endpoint="https://api.elevenlabs.io",
            note="ElevenLabs 云端合成",
        ),
        "openai": ProviderSpec(
            category="tts",
            code="openai",
            candidates=("pipecat.services.openai.tts:OpenAITTSService",),
            dependencies=("openai",),
            required_params=("api_key",),
            requires_api_key=True,
            default_endpoint="https://api.openai.com/v1",
            note="OpenAI TTS",
        ),
    },
    "vad": {
        "webrtcvad": ProviderSpec(
            category="vad",
            code="webrtcvad",
            candidates=(
                "pipecat.audio.vad.webrtc_vad:WebRTCVADAnalyzer",
                # 1.13 把该类放进了 daily transport（导入会连带 daily 依赖）。
                "pipecat.transports.daily.transport:WebRTCVADAnalyzer",
            ),
            dependencies=("webrtcvad",),
            local=True,
            note="轻量 VAD；依赖 webrtcvad，旧版 Pipecat 路径可能不同",
        ),
        "silero": ProviderSpec(
            category="vad",
            code="silero",
            candidates=("pipecat.audio.vad.silero:SileroVADAnalyzer",),
            dependencies=("onnxruntime",),
            local=True,
            note="Pipecat 内置 Silero VAD（ONNX）",
        ),
    },
}

CATEGORIES: Tuple[str, ...] = ("stt", "llm", "tts", "vad")


@dataclass(frozen=True)
class ProbeResult:
    """provider 依赖探测结果。"""

    installed: bool
    resolved: Optional[str]
    missing: Tuple[str, ...]
    detail: str


def list_providers(category: str) -> Tuple[str, ...]:
    """返回某类别的全部 provider code（按声明顺序）。"""

    return tuple(PROVIDERS.get(category, {}).keys())


def provider_counts() -> Dict[str, int]:
    """``/health`` 使用：只返回类别与数量，不泄露 provider 细节与密钥。"""

    return {category: len(PROVIDERS.get(category, {})) for category in CATEGORIES}


def get_provider(category: str, code: str) -> ProviderSpec:
    """按类别与 code 取 spec；未知 provider 抛明确异常。"""

    category = (category or "").strip().lower()
    code = (code or "").strip().lower()
    if category not in PROVIDERS:
        raise UnknownProviderError(category, code, ())
    table = PROVIDERS[category]
    spec = table.get(code)
    if spec is None:
        raise UnknownProviderError(category, code, table.keys())
    return spec


# 同一进程内导入结果不会变化，缓存探测结果，避免 /test 反复触发重量级导入。
_probe_lock = threading.Lock()


@lru_cache(maxsize=64)
def _import_error(module: str) -> Optional[str]:
    try:
        importlib.import_module(module)
        return None
    except Exception as exc:  # ImportError / OSError（原生 SDK 加载失败）等
        return f"{type(exc).__name__}: {exc}"


def _resolve_candidate(candidate: str) -> Tuple[Optional[str], Optional[str]]:
    """导入 ``module:attr``，返回 (resolved, error)。"""

    module_name, _, attr = candidate.partition(":")
    error = _import_error(module_name)
    if error:
        return None, f"{module_name} 导入失败（{error}）"
    if not attr:
        return module_name, None
    try:
        module = importlib.import_module(module_name)
        getattr(module, attr)
    except Exception as exc:
        return None, f"{module_name}.{attr} 不可用（{type(exc).__name__}: {exc}）"
    return f"{module_name}.{attr}", None


def probe(spec: ProviderSpec) -> ProbeResult:
    """探测 provider 是否可用（只做本地导入，不发外部请求）。"""

    with _probe_lock:
        missing: list[str] = []
        errors: list[str] = []

        for dep in spec.dependencies:
            error = _import_error(dep)
            if error:
                missing.append(dep)
                errors.append(f"{dep}: {error}")

        resolved: Optional[str] = None
        if spec.candidates:
            for candidate in spec.candidates:
                resolved, error = _resolve_candidate(candidate)
                if resolved:
                    break
                if error:
                    errors.append(error)
            if not resolved:
                # 服务模块本身不可用（可能只是 Pipecat 版本路径变化）。
                missing.append(f"{spec.category}:{spec.code}:service")
        elif spec.adapter:
            adapter_cls = ADAPTERS.get(spec.adapter)
            if adapter_cls is None:
                errors.append(f"内置适配器缺失：{spec.adapter}")
                missing.append(spec.adapter)
            else:
                resolved = f"{__name__}:{spec.adapter}"
        else:
            errors.append("provider 未声明服务实现")
            missing.append(f"{spec.category}:{spec.code}:service")

        installed = resolved is not None and not missing
        if installed:
            detail = f"已就绪（{resolved}）"
        else:
            detail = "未安装或不可用：" + "；".join(errors or [", ".join(missing)])

        return ProbeResult(
            installed=installed,
            resolved=resolved,
            missing=tuple(missing),
            detail=detail,
        )


def describe(spec: ProviderSpec) -> Mapping[str, object]:
    """返回不含密钥的 provider 描述（/config、/session 的元信息）。"""

    return {
        "category": spec.category,
        "code": spec.code,
        "local": spec.local,
        "requires_api_key": spec.requires_api_key,
        "requires_endpoint": spec.requires_endpoint,
        "required_params": list(spec.required_params),
        "note": spec.note,
    }


__all__ = (
    "ADAPTERS",
    "CATEGORIES",
    "EdgeTTSAdapter",
    "PROVIDERS",
    "ProbeResult",
    "ProviderSpec",
    "UnknownProviderError",
    "describe",
    "get_provider",
    "list_providers",
    "probe",
    "provider_counts",
)
