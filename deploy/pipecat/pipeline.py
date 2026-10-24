"""VAD → STT → LLM → TTS 装配与真实音频管线运行。

`build_pipeline` 只负责**按当前配置生成描述并校验**（`/session/start` 的
400 契约）；`AudioPipelineRunner` 负责把描述变成真正跑起来的 Pipecat
Pipeline：

```text
RTPAudioReceiver (8k PCM)
  → [可选] 8k→16k 重采样
  → STT
  → LLM
  → TTS（Edge TTS 用本文件的 EdgeTTSProcessor）
  → OutputAudioRawFrame (24k PCM)
  → 重采样到 8k → WebRTCPCMAPusher → PCMA/RTP → ZLM
```

## 诚实边界

- 传输层（UDP RTP 收发、G.711、序号重排、WebRTC push）是真实实现，
  可以用 ``python transport.py`` 自测。
- Pipecat 的 provider 类名/构造参数在不同 1.x 小版本间有差异，本文件
  用 ``inspect.signature`` 过滤不支持的参数，并在装配失败时把 agent
  标为 ``unavailable``（音频桥仍可收，但不会说话），不会让控制面崩溃。
- 没有真实设备时只能验证「Pipecat 起服务 + 假 RTP 源灌包」这一段。
"""

from __future__ import annotations

import asyncio
import importlib
import inspect
import json
import logging
import subprocess
import threading
import time
from dataclasses import dataclass
from typing import Any, Dict, Iterable, List, Mapping, Optional, Tuple

import factory
import transport
from config_store import redact_dict

logger = logging.getLogger("ai-camera-pipecat.pipeline")

STAGE_ORDER: Tuple[str, ...] = ("vad", "stt", "llm", "tts")

# STT 的输入采样率。设备链路是 8k，但 faster-whisper / 云端 STT 通常
# 按 16k 建模；由输入处理器做一次线性重采样。
DEFAULT_STT_SAMPLE_RATE = 16000
# TTS 输出采样率（Edge TTS 默认 24k）。输出处理器负责降到 8k。
DEFAULT_TTS_SAMPLE_RATE = 24000

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

    def describe(self, audio_bridge: Optional[str] = None) -> Dict[str, Any]:
        options = dict(self.options)
        if audio_bridge is not None:
            options["audio_bridge"] = audio_bridge
        return {
            "session_id": self.session_id,
            "config_revision": self.config_revision,
            "mode": self.mode,
            "pipeline": self.stage_names,
            "stages": [dict(stage) for stage in self.stages],
            "options": options,
            "audio_bridge": options.get("audio_bridge", transport.AUDIO_BRIDGE_NOT_CONNECTED),
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
        "audio_bridge": transport.AUDIO_BRIDGE_NOT_CONNECTED,
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


# ════════════════════════════════════════════════════════════════
# 运行时
# ════════════════════════════════════════════════════════════════


class AudioPipelineRunner:
    """把 transport 与 Pipecat Pipeline 组合起来的会话运行时。

    生命周期：
      - ``start()`` 立即返回，模型加载/ provider 连接在后台线程中进行；
      - ``status()`` 是唯一的真实状态来源（主线程/HTTP 线程可安全调用）；
      - ``stop()`` 取消 Pipeline 并等待线程退出，不负责关闭 transport
        （transport 由会话管理器统一关闭，避免重复释放）。
    """

    def __init__(
        self,
        bundle: PipelineBundle,
        config: Mapping[str, Any],
        receiver: transport.RTPAudioReceiver,
        sender: Optional[transport.WebRTCPCMAPusher] = None,
        half_duplex: bool = True,
    ) -> None:
        self.bundle = bundle
        self.config = dict(config)
        self.receiver = receiver
        self.sender = sender
        self.half_duplex = half_duplex

        self.agent_state = "starting"
        self.agent_detail = ""
        self._stop = threading.Event()
        self._thread: Optional[threading.Thread] = None
        self._loop: Optional[asyncio.AbstractEventLoop] = None
        self._task: Any = None

    # ── 生命周期 ────────────────────────────────────────────────
    def start(self) -> None:
        if self._thread is not None:
            return
        self._thread = threading.Thread(
            target=self._thread_main,
            name=f"pipecat-pipeline-{self.bundle.session_id}",
            daemon=True,
        )
        self._thread.start()
        # 给同步构建失败（缺 pipecat / 缺 provider 类）一个快速暴露窗口。
        # 模型加载可能很久，不能用长超时阻塞 /session/start。
        deadline = time.monotonic() + 1.5
        while time.monotonic() < deadline:
            if self.agent_state in {"running", "unavailable"}:
                break
            time.sleep(0.02)

    def stop(self) -> None:
        self._stop.set()
        loop = self._loop
        if loop is not None and loop.is_running():
            try:
                loop.call_soon_threadsafe(loop.stop)
            except RuntimeError:
                pass
        if self._thread is not None and self._thread.is_alive():
            self._thread.join(timeout=3.0)
        self._thread = None
        self.agent_state = "stopped"

    def _thread_main(self) -> None:
        try:
            self._run_pipeline()
        except Exception as exc:  # 装配/运行失败：标记 degraded，不拖垮控制面
            self.agent_state = "unavailable"
            self.agent_detail = f"{type(exc).__name__}: {exc}"
            logger.warning(
                "语音管线不可用 session=%s：%s",
                self.bundle.session_id,
                self.agent_detail,
                exc_info=True,
            )

    # ── 状态 ────────────────────────────────────────────────────
    def status(self) -> Dict[str, Any]:
        uplink = self.receiver.status()
        downlink = self.sender.status() if self.sender is not None else transport.AUDIO_BRIDGE_NOT_CONNECTED
        bridge = transport.combine_bridge_status(uplink, downlink)

        # 传输通了但 agent 起不来（例如 pipecat API 不匹配）时不能报
        # connected —— 那会让后端误以为「设备能听到 agent 说话」。
        if self.agent_state in {"unavailable", "failed", "stopped"} and bridge == transport.AUDIO_BRIDGE_CONNECTED:
            bridge = transport.AUDIO_BRIDGE_DEGRADED

        data: Dict[str, Any] = {
            "audio_bridge": bridge,
            "uplink": uplink,
            "downlink": downlink,
            "agent": self.agent_state,
            "detail": self.agent_detail or None,
            "transport": self.receiver.stats(),
        }
        if self.sender is not None:
            data["downlink_transport"] = self.sender.stats()
        return data

    @property
    def agent_ready(self) -> bool:
        """Pipecat 自报「可以开始对讲」的就绪信号。

        为什么不能看 ``audio_bridge``：``/session/start`` 返回时后端还没
        调 ``startSendRtp``，设备音频一帧都没到，上行必然是 degraded。
        如果这里要求 connected，后端就会降级、不再建立上行，形成死锁。

        就绪条件：
          - UDP 接收端已绑定（端口已返回给后端）；
          - 下行 WebRTC 已连接（否则 agent 说话也听不见）；
          - 管线不是 unavailable/failed/stopped（starting 允许，模型可能还在加载）。
        """

        if self.agent_state in {"unavailable", "failed", "stopped"}:
            return False
        if not self.receiver.started:
            return False
        if self.sender is None:
            return False
        return self.sender.status() == transport.AUDIO_BRIDGE_CONNECTED

    # ── 真正的 Pipecat Pipeline ─────────────────────────────────
    def _run_pipeline(self) -> None:
        # 延迟导入：控制面在没有安装 pipecat 时也能启动（降级）。
        from pipecat.frames.frames import (  # type: ignore
            CancelFrame,
            EndFrame,
            InputAudioRawFrame,
            OutputAudioRawFrame,
            StartFrame,
        )
        from pipecat.pipeline.pipeline import Pipeline  # type: ignore
        from pipecat.pipeline.task import PipelineParams, PipelineTask  # type: ignore
        from pipecat.processors.frame_processor import FrameDirection, FrameProcessor  # type: ignore

        self.agent_state = "starting"
        stt = _instantiate_service("stt", self.config)
        llm = _instantiate_service("llm", self.config)
        tts = self._make_tts_processor(FrameProcessor, FrameDirection)
        vad = _instantiate_vad(self.config)

        input_proc = self._make_input_processor(
            FrameProcessor, FrameDirection, InputAudioRawFrame, StartFrame, EndFrame, CancelFrame
        )
        output_proc = self._make_output_processor(
            FrameProcessor, FrameDirection, OutputAudioRawFrame
        )
        user_agg, assistant_agg = _make_context_aggregators(self.config)

        pipeline = Pipeline([input_proc, stt, user_agg, llm, tts, output_proc, assistant_agg])
        params = _make_pipeline_params(PipelineParams, vad)

        task = PipelineTask(pipeline, params=params)
        self._task = task

        loop = asyncio.new_event_loop()
        self._loop = loop
        asyncio.set_event_loop(loop)
        self.agent_state = "running"
        logger.info(
            "语音管线已启动 session=%s pipeline=%s",
            self.bundle.session_id,
            summary(self.bundle),
        )
        try:
            from pipecat.pipeline.runner import PipelineRunner  # type: ignore

            # 1.x 的 PipelineRunner.run 是协程；个别版本提供同步包装。
            # 两种形态都兼容，避免「升级一个小版本就整体起不来」。
            result = PipelineRunner().run(task)
            if inspect.isawaitable(result):
                loop.run_until_complete(result)
        finally:
            self.agent_state = "stopped"
            try:
                loop.run_until_complete(loop.shutdown_asyncgens())
            finally:
                loop.close()
                self._loop = None

    def _make_input_processor(
        self,
        frame_processor_cls: Any,
        direction_cls: Any,
        input_frame_cls: Any,
        start_frame_cls: Any,
        end_frame_cls: Any,
        cancel_frame_cls: Any,
    ) -> Any:
        receiver = self.receiver
        stop_event = self._stop
        stt_rate = _as_int(self.config.get("stt_sample_rate"), DEFAULT_STT_SAMPLE_RATE)
        downstream = direction_cls.DOWNSTREAM

        class RTPInputProcessor(frame_processor_cls):  # type: ignore[misc, valid-type]
            def __init__(self) -> None:
                super().__init__()
                self._pump_task: Optional[asyncio.Task] = None

            async def process_frame(self, frame: Any, direction: Any) -> None:
                await super().process_frame(frame, direction)
                if isinstance(frame, start_frame_cls):
                    if self._pump_task is None:
                        self._pump_task = asyncio.create_task(self._pump())
                elif isinstance(frame, (end_frame_cls, cancel_frame_cls)):
                    if self._pump_task is not None:
                        self._pump_task.cancel()
                        self._pump_task = None
                await self.push_frame(frame, direction)

            async def _pump(self) -> None:
                while not stop_event.is_set():
                    pcm = await asyncio.to_thread(receiver.read_pcm, 0.02)
                    if not pcm:
                        await asyncio.sleep(0.005)
                        continue
                    if stt_rate != transport.G711_SAMPLE_RATE:
                        pcm = transport.resample_pcm16(pcm, transport.G711_SAMPLE_RATE, stt_rate)
                    frame = input_frame_cls(audio=pcm, sample_rate=stt_rate, num_channels=1)
                    await self.push_frame(frame, downstream)

        return RTPInputProcessor()

    def _make_output_processor(self, frame_processor_cls: Any, direction_cls: Any, output_frame_cls: Any) -> Any:
        sender = self.sender
        receiver = self.receiver
        half_duplex = self.half_duplex
        downstream = direction_cls.DOWNSTREAM

        class TTSOutputProcessor(frame_processor_cls):  # type: ignore[misc, valid-type]
            async def process_frame(self, frame: Any, direction: Any) -> None:
                await super().process_frame(frame, direction)
                # EdgeTTSProcessor 直接发 OutputAudioRawFrame；标准 TTS
                # service 发的是 TTSAudioRawFrame。两者都带 audio/sample_rate。
                if isinstance(frame, output_frame_cls) or type(frame).__name__ in {
                    "TTSAudioRawFrame",
                    "AudioRawFrame",
                }:
                    pcm = getattr(frame, "audio", b"") or b""
                    rate = int(getattr(frame, "sample_rate", DEFAULT_TTS_SAMPLE_RATE) or DEFAULT_TTS_SAMPLE_RATE)
                    if pcm:
                        if rate != transport.G711_SAMPLE_RATE:
                            pcm = transport.resample_pcm16(pcm, rate, transport.G711_SAMPLE_RATE)
                        if sender is not None:
                            sender.write_pcm(pcm)
                        if half_duplex:
                            # 设计 §8.1.1 方案①：agent 说话期间静音上行。
                            # 多留 300ms 尾巴，覆盖设备喇叭/网络的残余回声。
                            duration = len(pcm) / 2 / transport.G711_SAMPLE_RATE + 0.3
                            receiver.set_uplink_muted(True, duration)
                await self.push_frame(frame, direction)

        return TTSOutputProcessor()

    def _make_tts_processor(self, frame_processor_cls: Any, direction_cls: Any) -> Any:
        provider = str(self.config.get("tts_provider") or "").strip().lower()
        if provider and provider != "edge_tts":
            return _instantiate_service("tts", self.config)
        return _make_edge_tts_processor(frame_processor_cls, direction_cls, self.config)


# ── provider 实例化 ────────────────────────────────────────────


def _instantiate_service(category: str, config: Mapping[str, Any]) -> Any:
    """按 factory 的 provider 描述实例化 Pipecat Service。

    不同 1.x 小版本的构造参数名不同（``api_key`` / ``key`` /
    ``base_url`` / ``endpoint``），这里用 ``inspect.signature`` 过滤：
    只传构造函数真正接受的键，其余忽略。必填参数已由 build_pipeline 校验。
    """

    provider = str(config.get(f"{category}_provider") or "").strip().lower()
    spec = factory.get_provider(category, provider)
    probe = factory.probe(spec)
    if not probe.installed or not probe.resolved:
        raise RuntimeError(f"{category.upper()} provider {provider} 不可用：{probe.detail}")
    module_name, _, attr = probe.resolved.partition(":")
    cls = getattr(importlib.import_module(module_name), attr)

    params = _collect_params(category, config)
    kwargs = {
        "api_key": params.get("api_key"),
        "model": params.get("model"),
        "language": params.get("language"),
        "voice": params.get("voice"),
        "base_url": params.get("endpoint"),
        "endpoint": params.get("endpoint"),
        "speed": params.get("speed"),
        "temperature": params.get("temperature"),
        "max_tokens": params.get("max_tokens"),
    }
    kwargs = {k: v for k, v in kwargs.items() if v not in (None, "")}

    try:
        signature = inspect.signature(cls.__init__)
        accepted = set(signature.parameters)
        has_kwargs = any(p.kind == inspect.Parameter.VAR_KEYWORD for p in signature.parameters.values())
        if not has_kwargs:
            kwargs = {k: v for k, v in kwargs.items() if k in accepted}
        return cls(**kwargs)
    except (TypeError, ValueError) as exc:
        raise RuntimeError(f"{category.upper()} provider {provider} 实例化失败：{exc}") from exc


def _make_context_aggregators(config: Mapping[str, Any]) -> Tuple[Any, Any]:
    """构造 LLM context + user/assistant aggregator。

    Pipecat 1.x 在这个位置改过模块名，因此按候选路径依次尝试；
    全部失败时明确报错，让会话降级而不是静默跑一个没有上下文的 pipeline。
    """

    candidates = (
        ("pipecat.processors.aggregators.llm_context", "LLMContext", "LLMContextAggregatorPair"),
        ("pipecat.processors.aggregators.openai_llm_context", "OpenAILLMContext", "OpenAIContextAggregatorPair"),
    )
    last_error = ""
    for module_name, context_name, pair_name in candidates:
        try:
            module = importlib.import_module(module_name)
            context_cls = getattr(module, context_name)
            pair_cls = getattr(module, pair_name)
        except (ImportError, AttributeError) as exc:
            last_error = f"{module_name}: {exc}"
            continue

        context = context_cls()
        prompt = str(config.get("llm_system_prompt") or "").strip()
        if prompt:
            # 不同版本的方法名不同：set_messages / set_message 都试一下。
            for method_name in ("set_messages", "set_message"):
                method = getattr(context, method_name, None)
                if callable(method):
                    try:
                        method([{"role": "system", "content": prompt}])
                        break
                    except Exception:
                        continue

        pair = pair_cls(context)
        user = getattr(pair, "user", None) or getattr(pair, "user_context", None)
        assistant = getattr(pair, "assistant", None) or getattr(pair, "assistant_context", None)
        if callable(user):
            user = user()
        if callable(assistant):
            assistant = assistant()
        if user is not None and assistant is not None:
            return user, assistant
        last_error = f"{module_name}: aggregator pair 缺少 user/assistant"

    raise RuntimeError(f"当前 pipecat 版本找不到 LLM context aggregator（{last_error}）")


def _instantiate_vad(config: Mapping[str, Any]) -> Any:
    """实例化 VAD analyzer（默认 Silero）。

    VAD 不是「锦上添花」：STT 需要知道一句话什么时候结束，否则
    Whisper 会一直等下去。provider 依赖已由 build_pipeline 探测，
    这里构造失败直接让管线降级。
    """

    provider = str(config.get("vad_provider") or "").strip().lower()
    if not provider:
        return None
    spec = factory.get_provider("vad", provider)
    probe = factory.probe(spec)
    if not probe.installed or not probe.resolved:
        raise RuntimeError(f"VAD provider {provider} 不可用：{probe.detail}")
    module_name, _, attr = probe.resolved.partition(":")
    cls = getattr(importlib.import_module(module_name), attr)

    params = _collect_params("vad", config)
    kwargs = dict(params)
    kwargs.setdefault("sample_rate", DEFAULT_STT_SAMPLE_RATE)
    try:
        signature = inspect.signature(cls.__init__)
        has_kwargs = any(p.kind == inspect.Parameter.VAR_KEYWORD for p in signature.parameters.values())
        if not has_kwargs:
            kwargs = {k: v for k, v in kwargs.items() if k in signature.parameters}
        return cls(**kwargs)
    except TypeError:
        try:
            return cls()
        except TypeError:
            return None


def _make_pipeline_params(params_cls: Any, vad: Any = None) -> Any:
    """构造 PipelineParams；小版本字段差异时退回默认值。"""

    attempts: List[Dict[str, Any]] = []
    base = {
        "allow_interruptions": False,
        "audio_in_sample_rate": DEFAULT_STT_SAMPLE_RATE,
        "audio_out_sample_rate": DEFAULT_TTS_SAMPLE_RATE,
    }
    if vad is not None:
        attempts.append({**base, "vad_analyzer": vad})
    attempts.append(base)
    attempts.append({"allow_interruptions": False})
    attempts.append({})

    for kwargs in attempts:
        try:
            return params_cls(**kwargs)
        except TypeError:
            continue
    return params_cls()


def _make_edge_tts_processor(frame_processor_cls: Any, direction_cls: Any, config: Mapping[str, Any]) -> Any:
    """Edge TTS 的 FrameProcessor 实现。

    Pipecat 1.13 没有内置 Edge TTS service（见 factory.py 的说明），
    因此这里直接消费 LLM 输出的文本帧，用 ``edge_tts`` 合成 MP3，
    再经 ffmpeg 转成 24k PCM，最后以 ``OutputAudioRawFrame`` 推给
    输出处理器。它和标准 TTSService 的差异只是「没有内置分句/打断」，
    对半双工 MVP 足够。
    """

    voice = str(config.get("tts_voice") or "zh-CN-XiaoxiaoNeural").strip()
    try:
        speed = float(str(config.get("tts_speed") or "1.0"))
    except ValueError:
        speed = 1.0
    rate = f"{int(round((speed - 1.0) * 100)):+d}%"
    downstream = direction_cls.DOWNSTREAM

    class EdgeTTSProcessor(frame_processor_cls):  # type: ignore[misc, valid-type]
        def __init__(self) -> None:
            super().__init__()
            self._text: List[str] = []
            self._speaking = False

        async def process_frame(self, frame: Any, direction: Any) -> None:
            await super().process_frame(frame, direction)
            name = type(frame).__name__
            if name in {"TextFrame", "LLMTextFrame"}:
                text = getattr(frame, "text", "") or ""
                if text:
                    self._text.append(text)
            elif name in {"LLMFullResponseEndFrame", "EndFrame"}:
                text = "".join(self._text).strip()
                self._text.clear()
                if text:
                    asyncio.create_task(self._speak(text))
            await self.push_frame(frame, direction)

        async def _speak(self, text: str) -> None:
            if self._speaking:
                return
            self._speaking = True
            try:
                import edge_tts  # type: ignore

                communicate = edge_tts.Communicate(text, voice, rate=rate)
                mp3 = bytearray()
                async for chunk in communicate.stream():
                    if chunk.get("type") == "audio" and chunk.get("data"):
                        mp3.extend(chunk["data"])
                if not mp3:
                    logger.warning("Edge TTS 未返回音频：%s", text[:40])
                    return
                pcm = await asyncio.to_thread(_mp3_to_pcm16, bytes(mp3))
                frame = _make_output_frame(pcm, DEFAULT_TTS_SAMPLE_RATE)
                await self.push_frame(frame, downstream)
            except Exception as exc:
                logger.warning("Edge TTS 合成失败：%s", exc, exc_info=True)
            finally:
                self._speaking = False

    # OutputAudioRawFrame 在当前版本里可能要求 num_channels；延迟导入。
    from pipecat.frames.frames import OutputAudioRawFrame  # type: ignore

    def _make_output_frame(pcm: bytes, sample_rate: int) -> Any:
        return OutputAudioRawFrame(audio=pcm, sample_rate=sample_rate, num_channels=1)

    return EdgeTTSProcessor()


def _mp3_to_pcm16(mp3: bytes) -> bytes:
    """用 ffmpeg 把 MP3 转成 24kHz/单声道/s16le。"""

    process = subprocess.run(
        [
            "ffmpeg", "-hide_banner", "-loglevel", "error",
            "-i", "pipe:0",
            "-f", "s16le", "-acodec", "pcm_s16le", "-ac", "1", "-ar", str(DEFAULT_TTS_SAMPLE_RATE),
            "pipe:1",
        ],
        input=mp3,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if process.returncode != 0:
        raise RuntimeError(f"ffmpeg 转码失败：{process.stderr.decode('utf-8', 'replace')[:300]}")
    return process.stdout


__all__: Iterable[str] = (
    "AudioPipelineRunner",
    "DEFAULT_STT_SAMPLE_RATE",
    "DEFAULT_TTS_SAMPLE_RATE",
    "PARAM_SOURCES",
    "PipelineBundle",
    "PipelineConfigError",
    "STAGE_ORDER",
    "build_pipeline",
    "summary",
)
