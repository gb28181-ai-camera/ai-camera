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
import os
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


def _split_resolved(resolved: str) -> Tuple[str, str]:
    """拆 ``module:Class`` / ``module.Class`` 两种 resolved 形式。

    ``factory.ProbeResult.resolved`` 对候选路径使用 ``module.Class``，而对
    内置适配器使用 ``module:Class``。旧代码只认冒号，导致候选 provider
    在真正实例化时 import 整串名字失败（RTP/WS 都受影响）。
    """

    text = str(resolved or "").strip()
    if not text:
        raise RuntimeError("provider 未解析出实现类")
    if ":" in text:
        module_name, _, attr = text.partition(":")
        if module_name and attr:
            return module_name, attr
    module_name, _, attr = text.rpartition(".")
    if not module_name or not attr:
        raise RuntimeError(f"无法解析 provider 实现：{text}")
    return module_name, attr


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
    module_name, attr = _split_resolved(probe.resolved)
    cls = getattr(importlib.import_module(module_name), attr)

    params = _collect_params(category, config)
    # 离线部署：容器内预置了 /models/faster-whisper-<size>，而配置里仍写
    # 「small」这类模型名。faster-whisper 默认会去 HuggingFace 下载，内网
    # 不可达；这里自动换成同名的本地目录，配置项无需暴露绝对路径。
    if category == "stt" and provider == "local_whisper":
        model = str(params.get("model") or "").strip()
        if model and "/" not in model and not os.path.isabs(model):
            local_dir = os.path.join("/models", f"faster-whisper-{model}")
            if os.path.isdir(local_dir):
                params["model"] = local_dir
                logger.info("STT 使用本地模型 model=%s dir=%s", model, local_dir)
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

    # 注意：这里保持 RTP 路径的既有行为（v0.2.0 在 pipecat 1.13 下会因
    # pair 模块迁移而标记 agent unavailable）。WS 路径使用
    # build_ws_pipeline 里的 1.13 正确 API，不复用本函数。
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
    module_name, attr = _split_resolved(probe.resolved)
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


# ════════════════════════════════════════════════════════════════
# WebSocket 语音管线（v0.6.0）
# ════════════════════════════════════════════════════════════════
#
# 与 RTP 路径共享 provider 装配/参数映射，但**不复用** AudioPipelineRunner：
# WS 的输入输出由 FastAPIWebsocketTransport 负责，音频格式在 serializer
# 里协商。RTP 路径（build_pipeline / AudioPipelineRunner）保持原样。

WS_INPUT_SAMPLE_RATE = 16000
WS_OUTPUT_SAMPLE_RATE = 24000
WS_SENTENCE_END_CHARS = "。！？!?；;\n"
WS_MAX_SENTENCE_CHARS = 120


@dataclass
class WSPipelineBundle:
    """WS 会话装配结果（供会话层运行/摘要/诊断）。"""

    session_id: str
    pipeline: Any
    worker: Any
    context: Any
    llm: Any
    user_aggregator: Any
    assistant_aggregator: Any
    stages: Tuple[str, ...]
    tools_count: int
    options: Mapping[str, Any]

    def describe(self) -> Dict[str, Any]:
        return {
            "session_id": self.session_id,
            "transport": "websocket",
            "pipeline": list(self.stages),
            "tools": self.tools_count,
            "options": dict(self.options),
        }


def split_sentences(buffer: str) -> Tuple[List[str], str]:
    """把 LLM 增量文本切成完整句子，返回 ``(sentences, remainder)``。

    句末标点包括中英文句号/问号/叹号/分号/换行。英文句点只有在后面是
    空白或结尾时才切分，避免把 ``3.14`` 切开。超长无标点片段按
    ``WS_MAX_SENTENCE_CHARS`` 硬切，保证 TTS 不会一直等。
    """

    text = buffer or ""
    sentences: List[str] = []
    start = 0
    index = 0
    while index < len(text):
        char = text[index]
        boundary = char in WS_SENTENCE_END_CHARS
        if not boundary and char == ".":
            boundary = index + 1 >= len(text) or text[index + 1].isspace()
        if boundary:
            sentence = text[start : index + 1].strip()
            if sentence:
                sentences.append(sentence)
            start = index + 1
        index += 1

    remainder = text[start:]
    while len(remainder) > WS_MAX_SENTENCE_CHARS:
        cut = remainder.rfind(" ", 0, WS_MAX_SENTENCE_CHARS)
        if cut <= 0:
            cut = WS_MAX_SENTENCE_CHARS
        sentence = remainder[:cut].strip()
        if sentence:
            sentences.append(sentence)
        remainder = remainder[cut:]
    return sentences, remainder


async def _synthesize_edge_pcm(text: str, voice: str, speed: float) -> bytes:
    """Edge TTS 合成一句话 → 24kHz PCM16。

    失败抛异常，由调用方丢弃该句（不中断整个回复）。
    """

    import edge_tts  # type: ignore

    rate = f"{int(round((float(speed) - 1.0) * 100)):+d}%"
    communicate = edge_tts.Communicate(text, voice, rate=rate)
    mp3 = bytearray()
    async for chunk in communicate.stream():
        if chunk.get("type") == "audio" and chunk.get("data"):
            mp3.extend(chunk["data"])
    if not mp3:
        return b""
    return await asyncio.to_thread(_mp3_to_pcm16, bytes(mp3))


def _make_streaming_edge_tts_processor(
    frame_processor_cls: Any,
    direction_cls: Any,
    config: Mapping[str, Any],
    *,
    sample_rate: int = WS_OUTPUT_SAMPLE_RATE,
) -> Any:
    """按句流式 Edge TTS。

    与 RTP 用的 ``EdgeTTSProcessor`` 不同：LLM 每输出一个句末标点就入队
    合成，不再等整段回复结束。合成任务串行执行，保证音频顺序；打断时
    通过 generation 丢弃过期句子。
    """

    voice = str(config.get("tts_voice") or "zh-CN-XiaoxiaoNeural").strip()
    try:
        speed = float(str(config.get("tts_speed") or "1.0"))
    except ValueError:
        speed = 1.0
    downstream = direction_cls.DOWNSTREAM
    logger_ = logging.getLogger("ai-camera-pipecat.pipeline.ws_tts")

    class SentenceStreamingEdgeTTSProcessor(frame_processor_cls):  # type: ignore[misc, valid-type]
        def __init__(self) -> None:
            super().__init__()
            self._buffer = ""
            self._queue: "asyncio.Queue[Any]" = asyncio.Queue()
            self._worker: Optional[asyncio.Task] = None
            self._generation = 0
            self._speaking = False
            self._idle = asyncio.Event()
            self._idle.set()

        # ── FrameProcessor ──────────────────────────────────────
        async def process_frame(self, frame: Any, direction: Any) -> None:
            await super().process_frame(frame, direction)
            name = type(frame).__name__

            if name == "LLMFullResponseStartFrame":
                self._generation += 1
                self._buffer = ""
                self._ensure_worker()
                await self.push_frame(frame, direction)
                return

            if name in {"TextFrame", "LLMTextFrame"}:
                if getattr(frame, "skip_tts", False):
                    # 情绪/代码等标记不朗读，但仍由 LLM tap 下发给客户端。
                    return
                text = getattr(frame, "text", "") or ""
                # 消费 LLM 文本；用 TTSTextFrame 下发实际朗读的句子，避免
                # assistant aggregator 重复记账。
                if text:
                    # 兼容不发送 LLMFullResponseStartFrame 的 provider。
                    self._ensure_worker()
                    self._buffer += text
                    sentences, self._buffer = split_sentences(self._buffer)
                    for sentence in sentences:
                        await self._queue.put((self._generation, sentence))
                return

            if name == "LLMFullResponseEndFrame":
                if self._buffer.strip():
                    await self._queue.put((self._generation, self._buffer.strip()))
                    self._buffer = ""
                await self._flush_worker()
                await self.push_frame(frame, direction)
                return

            if name == "InterruptionFrame":
                self._generation += 1
                self._buffer = ""
                self._drain_queue()
                self._speaking = False
                await self.push_frame(frame, direction)
                return

            if name in {"EndFrame", "CancelFrame"}:
                self._generation += 1
                self._drain_queue()
                await self._queue.put(None)
                try:
                    await asyncio.wait_for(self._idle.wait(), timeout=1.0)
                except (asyncio.TimeoutError, TimeoutError):
                    pass
                await self.push_frame(frame, direction)
                return

            await self.push_frame(frame, direction)

        async def cleanup(self) -> None:
            task = self._worker
            self._worker = None
            if task is not None and not task.done():
                task.cancel()
            self._drain_queue()
            await super().cleanup()

        # ── 内部 ────────────────────────────────────────────────
        def _ensure_worker(self) -> None:
            if self._worker is None or self._worker.done():
                self._idle.clear()
                self._worker = asyncio.create_task(self._worker_loop())

        def _drain_queue(self) -> None:
            while True:
                try:
                    self._queue.get_nowait()
                except asyncio.QueueEmpty:
                    break

        async def _flush_worker(self) -> None:
            self._ensure_worker()
            self._idle.clear()
            await self._queue.put(None)
            try:
                await asyncio.wait_for(self._idle.wait(), timeout=10.0)
            except (asyncio.TimeoutError, TimeoutError):
                logger_.warning("等待 TTS 队列清空超时，继续下发结束帧")

        async def _worker_loop(self) -> None:
            while True:
                item = await self._queue.get()
                if item is None:
                    if self._speaking:
                        self._speaking = False
                        try:
                            from pipecat.frames.frames import TTSStoppedFrame  # type: ignore

                            await self.push_frame(TTSStoppedFrame(), downstream)
                        except Exception:
                            pass
                    self._idle.set()
                    continue
                generation, sentence = item
                if generation != self._generation:
                    continue
                try:
                    await self._speak(generation, sentence)
                except Exception as exc:
                    logger_.warning(
                        "Edge TTS 句子合成失败（已跳过）：%s", type(exc).__name__
                    )

        async def _speak(self, generation: int, sentence: str) -> None:
            from pipecat.frames.frames import (  # type: ignore
                OutputAudioRawFrame,
                TTSStartedFrame,
                TTSTextFrame,
            )
            from pipecat.utils.text.base_text_aggregator import TextType  # type: ignore

            if generation != self._generation:
                return
            if not self._speaking:
                self._speaking = True
                await self.push_frame(TTSStartedFrame(), downstream)
            await self.push_frame(
                TTSTextFrame(text=sentence, text_type=TextType.SENTENCE), downstream
            )

            pcm = await _synthesize_edge_pcm(sentence, voice, speed)
            if generation != self._generation or not pcm:
                return
            await self.push_frame(
                OutputAudioRawFrame(
                    audio=pcm, sample_rate=sample_rate, num_channels=1
                ),
                downstream,
            )

    return SentenceStreamingEdgeTTSProcessor()


def _make_ws_tts_processor(
    frame_processor_cls: Any,
    direction_cls: Any,
    config: Mapping[str, Any],
) -> Any:
    provider = str(config.get("tts_provider") or "").strip().lower()
    if provider and provider != "edge_tts":
        return _instantiate_service("tts", config)
    return _make_streaming_edge_tts_processor(frame_processor_cls, direction_cls, config)


def _make_frame_tap(frame_processor_cls: Any, stage: str, callback: Any) -> Any:
    """把 Pipecat 帧转发给会话层（不吞帧）。"""

    class WSTapProcessor(frame_processor_cls):  # type: ignore[misc, valid-type]
        def __init__(self) -> None:
            super().__init__()
            self._stage = stage

        async def process_frame(self, frame: Any, direction: Any) -> None:
            await super().process_frame(frame, direction)
            try:
                await callback(self._stage, frame, direction)
            except Exception:
                logger.debug("WS 帧回调失败 stage=%s", self._stage, exc_info=True)
            await self.push_frame(frame, direction)

    return WSTapProcessor()


def _make_user_turn_params(
    vad: Any,
    listen_mode: str,
) -> Dict[str, Any]:
    """构造 ``LLMUserAggregatorParams`` 的关键字参数。

    - auto：VAD 起句 + 语音超时断句（不走需要下载模型的 smart-turn）；
    - manual：外部 listen/start/stop 驱动回合，但 VAD 仍保留给
      SegmentedSTTService 切分音频。
    """

    params: Dict[str, Any] = {"vad_analyzer": vad}
    try:
        if listen_mode == "manual" or vad is None:
            from pipecat.turns.user_turn_strategies import (  # type: ignore
                ExternalUserTurnStrategies,
            )

            params["user_turn_strategies"] = ExternalUserTurnStrategies(
                enable_interruptions=False
            )
        else:
            from pipecat.turns.user_start.transcription_user_turn_start_strategy import (  # type: ignore
                TranscriptionUserTurnStartStrategy,
            )
            from pipecat.turns.user_start.vad_user_turn_start_strategy import (  # type: ignore
                VADUserTurnStartStrategy,
            )
            from pipecat.turns.user_stop.speech_timeout_user_turn_stop_strategy import (  # type: ignore
                SpeechTimeoutUserTurnStopStrategy,
            )
            from pipecat.turns.user_turn_strategies import UserTurnStrategies  # type: ignore

            params["user_turn_strategies"] = UserTurnStrategies(
                # 半双工：用户说话不打断 bot（barge_in 默认关闭）。
                start=[
                    VADUserTurnStartStrategy(enable_interruptions=False),
                    TranscriptionUserTurnStartStrategy(enable_interruptions=False),
                ],
                stop=[SpeechTimeoutUserTurnStopStrategy(user_speech_timeout=0.6)],
            )
    except Exception as exc:
        logger.warning("无法配置用户回合策略，回退 Pipecat 默认值：%s", exc)
    return params


def build_ws_pipeline(
    config: Mapping[str, Any],
    session_id: str,
    transport: Any,
    *,
    on_frame: Any,
    system_prompt: str = "",
    tools: Iterable[Any] = (),
    listen_mode: str = "auto",
    config_revision: int = 0,
    input_rate: int = WS_INPUT_SAMPLE_RATE,
    output_rate: int = WS_OUTPUT_SAMPLE_RATE,
) -> WSPipelineBundle:
    """装配 WS 语音管线。

    ``on_frame(stage, frame, direction)`` 会被三个 tap 调用（stt/llm/tts），
    用于向客户端推送字幕/TTS 状态；回调异常不会影响音频。

    与 RTP 的 ``build_pipeline`` 共用 provider 校验/实例化，确保两套链路
    的配置语义一致。装配失败抛 :class:`PipelineConfigError` 或
    :class:`RuntimeError`（会话层转成 ``error`` 消息）。
    """

    if not str(session_id or "").strip():
        raise PipelineConfigError("session_id 不能为空")

    # 先复用 RTP 的校验（provider 存在、必填参数、依赖已安装）。
    base = build_pipeline(
        config=config,
        session_id=session_id,
        mode="half_duplex",
        config_revision=config_revision,
    )

    try:
        from pipecat.pipeline.pipeline import Pipeline  # type: ignore
        from pipecat.pipeline.worker import PipelineParams, PipelineWorker  # type: ignore
        from pipecat.processors.aggregators.llm_context import LLMContext  # type: ignore
        from pipecat.processors.aggregators.llm_response_universal import (  # type: ignore
            LLMAssistantAggregatorParams,
            LLMContextAggregatorPair,
            LLMUserAggregatorParams,
        )
        from pipecat.processors.frame_processor import FrameDirection, FrameProcessor  # type: ignore
    except Exception as exc:
        raise RuntimeError(
            f"当前 pipecat 版本缺少 WebSocket 管线 API（需要 1.13）：{exc}"
        ) from exc

    stt = _instantiate_service("stt", config)
    llm = _instantiate_service("llm", config)
    tts = _make_ws_tts_processor(FrameProcessor, FrameDirection, config)
    vad = _instantiate_vad(config)

    user_params = LLMUserAggregatorParams(
        **_make_user_turn_params(vad, str(listen_mode or "auto").lower())
    )
    assistant_params = LLMAssistantAggregatorParams(
        enable_auto_context_summarization=True
    )

    messages = None
    if str(system_prompt or "").strip():
        messages = [{"role": "system", "content": system_prompt.strip()}]

    tool_list = [tool for tool in tools if tool is not None]
    context_kwargs: Dict[str, Any] = {}
    if tool_list:
        context_kwargs["tools"] = tool_list
    context = LLMContext(messages=messages, **context_kwargs)

    user_agg, assistant_agg = LLMContextAggregatorPair(
        context,
        user_params=user_params,
        assistant_params=assistant_params,
    )

    tap_stt = _make_frame_tap(FrameProcessor, "stt", on_frame)
    tap_llm = _make_frame_tap(FrameProcessor, "llm", on_frame)
    tap_tts = _make_frame_tap(FrameProcessor, "tts", on_frame)

    pipeline_obj = Pipeline(
        [
            transport.input(),
            stt,
            tap_stt,
            user_agg,
            llm,
            tap_llm,
            tts,
            tap_tts,
            transport.output(),
            assistant_agg,
        ]
    )

    worker = PipelineWorker(
        pipeline_obj,
        params=PipelineParams(
            audio_in_sample_rate=int(input_rate),
            audio_out_sample_rate=int(output_rate),
            enable_metrics=False,
        ),
        # 小智协议不需要 RTVI；关掉可避免额外的 rtvi-ai 消息。
        enable_rtvi=False,
        # WS 会话有自己的 idle/max 看门狗（见 ws/session.py），关闭框架
        # 默认的 idle 取消，避免用户长时间不说话被意外踢掉。
        idle_timeout_secs=None,
        cancel_on_idle_timeout=False,
    )

    options = dict(base.options)
    options.update(
        {
            "listen_mode": str(listen_mode or "auto").lower(),
            "input_sample_rate": int(input_rate),
            "output_sample_rate": int(output_rate),
            "tools": len(tool_list),
        }
    )

    return WSPipelineBundle(
        session_id=session_id,
        pipeline=pipeline_obj,
        worker=worker,
        context=context,
        llm=llm,
        user_aggregator=user_agg,
        assistant_aggregator=assistant_agg,
        stages=tuple(base.stage_names),
        tools_count=len(tool_list),
        options=options,
    )


__all__: Iterable[str] = (
    "AudioPipelineRunner",
    "DEFAULT_STT_SAMPLE_RATE",
    "DEFAULT_TTS_SAMPLE_RATE",
    "PARAM_SOURCES",
    "PipelineBundle",
    "PipelineConfigError",
    "STAGE_ORDER",
    "WS_INPUT_SAMPLE_RATE",
    "WS_OUTPUT_SAMPLE_RATE",
    "WSPipelineBundle",
    "build_pipeline",
    "build_ws_pipeline",
    "split_sentences",
    "summary",
)
