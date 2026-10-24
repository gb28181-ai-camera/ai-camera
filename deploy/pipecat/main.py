"""Pipecat 语音 Agent 控制面（FastAPI）。

设计文档 §8.2「Pipecat 侧控制面」。从 v0.2.1 起，除配置/会话登记外，
`/session/start` 会**真正启动** RTP 音频桥与 Pipecat Pipeline：

| 端点 | 鉴权 | 作用 |
|:--|:--:|:--|
| ``GET /health`` | 否 | 存活 + provider 数量 + **真实**音频桥状态 |
| ``POST /config`` | 是 | 后端 ``Apply`` 下发配置，原子替换内存快照 |
| ``GET /config`` | 是 | 读当前配置（敏感键脱敏） |
| ``POST /session/start`` | 是 | 启动 UDP 接收 + WebRTC 推流 + Pipeline |
| ``POST /session/stop`` | 是 | 停止并清理会话 |
| ``GET /sessions`` | 是 | 活跃会话与各自的音频桥状态 |
| ``POST /test`` | 是 | provider 最小连通性检查（不发外部计费请求） |

## 上行/下行参数由调用方传入

`/session/start` 接受 ``uplink_port`` / ``uplink_ssrc`` /
``downlink_stream`` / ``downlink_ssrc`` / ``zlm_base_url`` 等字段。
这些值来自后端的 GB28181 会话（SSRC、设备流名）与媒体节点配置 ——
Pipecat 侧不写死端口或流名，因此同一进程可以服务多路设备。

## 鉴权

共享 token 来自环境变量 ``PIPECAT_TOKEN``，支持：

- ``X-Pipecat-Token: <token>``
- ``Authorization: Bearer <token>``

**未配置 token 时进程拒绝启动**（fail-fast），不存在「默认放行」的开发模式。
后端 ``voice_agent_service.go`` 的 ``Apply`` 使用 ``Authorization: Bearer``，
token 取自 ``ac_thirdparty_config`` 的 ``pipecat_token``。
"""

from __future__ import annotations

import concurrent.futures
import hmac
import logging
import os
import socket
import threading
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional
from urllib.parse import urlparse

import factory
import pipeline
import transport
from config_store import ConfigStore, ConfigValidationError
from fastapi import Body, Depends, FastAPI, Header, HTTPException
from pydantic import BaseModel, Field

VERSION = "0.2.0"
SERVICE_NAME = "ai-camera-pipecat"

logging.basicConfig(
    level=os.environ.get("PIPECAT_LOG_LEVEL", "INFO").upper(),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger(SERVICE_NAME)

# ── 鉴权：token 未配置则拒绝启动 ─────────────────────────────────
_TOKEN = os.environ.get("PIPECAT_TOKEN", "").strip()
if not _TOKEN:
    raise RuntimeError(
        "PIPECAT_TOKEN 未配置：Pipecat 控制面拒绝启动。"
        "请在 .env 中设置 PIPECAT_TOKEN，并与后台「语音 Agent 配置」中的"
        "「Pipecat 控制面令牌」保持一致。"
    )

_AUTH_SCHEMES = ("bearer",)
MIN_TOKEN_LEN = 16
if len(_TOKEN) < MIN_TOKEN_LEN:
    logger.warning(
        "PIPECAT_TOKEN 长度仅 %d 字符，建议 ≥%d（共享 token 太短易被猜中）",
        len(_TOKEN),
        MIN_TOKEN_LEN,
    )

app = FastAPI(
    title=SERVICE_NAME,
    version=VERSION,
    docs_url=None,
    redoc_url=None,
    openapi_url=None,
)

store = ConfigStore()
_STARTED_AT = time.time()


# ── 鉴权依赖 ────────────────────────────────────────────────────
def require_token(
    x_pipecat_token: Optional[str] = Header(default=None, alias="X-Pipecat-Token"),
    authorization: Optional[str] = Header(default=None),
) -> None:
    """校验共享 token；支持 X-Pipecat-Token 与 Bearer 两种形式。"""

    supplied = (x_pipecat_token or "").strip()
    if not supplied and authorization:
        scheme, _, credential = authorization.partition(" ")
        if scheme.strip().lower() in _AUTH_SCHEMES:
            supplied = credential.strip()

    # 固定时间比较，避免按字节提前返回泄露信息。
    if not supplied or not hmac.compare_digest(supplied, _TOKEN):
        raise HTTPException(
            status_code=401,
            detail="未授权：Pipecat 控制面 token 无效",
            headers={"WWW-Authenticate": "Bearer"},
        )


# ── 会话登记 ────────────────────────────────────────────────────
@dataclass
class SessionState:
    session_id: str
    channel_id: str
    mode: str
    started_at: str
    started_monotonic: float
    bundle: pipeline.PipelineBundle
    config_revision: int
    receiver: transport.RTPAudioReceiver
    sender: Optional[transport.WebRTCPCMAPusher]
    runner: pipeline.AudioPipelineRunner
    downlink_stream: str = ""
    detail: str = ""

    def bridge(self) -> Dict[str, Any]:
        return self.runner.status()

    def describe(self) -> Dict[str, Any]:
        status = self.bridge()
        data = self.bundle.describe(audio_bridge=status["audio_bridge"])
        data.update(
            {
                "channel_id": self.channel_id,
                "started_at": self.started_at,
                "config_revision": self.config_revision,
                "uplink_port": self.receiver.port,
                "downlink_stream": self.downlink_stream,
                # 只有「音频桥 connected + pipeline 已起来」才允许后端
                # 把 agent_enabled 落成 true。
                "agent_ready": self.runner.agent_ready,
                "audio_bridge_detail": status,
            }
        )
        if self.detail:
            data["detail"] = self.detail
        return data


class SessionRegistry:
    """线程安全的活跃会话表（FastAPI 同步端点运行在线程池中）。"""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._sessions: Dict[str, SessionState] = {}

    def add(self, state: SessionState) -> bool:
        with self._lock:
            if state.session_id in self._sessions:
                return False
            self._sessions[state.session_id] = state
            return True

    def pop(self, session_id: str) -> Optional[SessionState]:
        with self._lock:
            return self._sessions.pop(session_id, None)

    def list(self) -> List[SessionState]:
        with self._lock:
            return sorted(
                self._sessions.values(),
                key=lambda state: state.started_monotonic,
            )

    def count(self) -> int:
        with self._lock:
            return len(self._sessions)


sessions = SessionRegistry()


# ── 请求模型 ────────────────────────────────────────────────────
class SessionStartRequest(BaseModel):
    session_id: str = Field(min_length=1, max_length=128)
    channel_id: str = Field(default="", max_length=128)
    # half_duplex（默认，设计 §8.1.1）/ full_duplex（实验项）/ ptt
    mode: str = Field(default="half_duplex", max_length=32)

    # ── 音频桥参数（全部由后端会话传入，不写死）──────────────
    uplink_port: int = Field(default=0, ge=0, le=65535)
    uplink_ssrc: Optional[int] = Field(default=None, ge=0, le=0xFFFFFFFF)
    downlink_stream: str = Field(default="", max_length=100)
    downlink_ssrc: Optional[int] = Field(default=None, ge=0, le=0xFFFFFFFF)
    zlm_base_url: str = Field(default="", max_length=255)
    # secret 只在内存中使用，不写日志。
    zlm_secret: str = Field(default="", max_length=255)
    zlm_app: str = Field(default="rtp", max_length=50)


class SessionStopRequest(BaseModel):
    session_id: str = Field(min_length=1, max_length=128)


class TestRequest(BaseModel):
    category: str = Field(min_length=1, max_length=16)
    # 可选：不传则使用当前配置中的 provider。
    provider: Optional[str] = Field(default=None, max_length=64)


# ── 健康检查（无需鉴权）────────────────────────────────────────
def aggregate_bridge_status() -> str:
    """把全部活跃会话的桥状态合并成一个可读值。

    - 没有任何会话：not_connected（没有桥可谈）；
    - 任一 connected：connected（至少一路是完整闭环）；
    - 否则只要还有 degraded：degraded；
    - 全部 not_connected：not_connected。
    """

    items = sessions.list()
    if not items:
        return transport.AUDIO_BRIDGE_NOT_CONNECTED
    statuses = [item.bridge()["audio_bridge"] for item in items]
    if transport.AUDIO_BRIDGE_CONNECTED in statuses:
        return transport.AUDIO_BRIDGE_CONNECTED
    if transport.AUDIO_BRIDGE_DEGRADED in statuses:
        return transport.AUDIO_BRIDGE_DEGRADED
    return transport.AUDIO_BRIDGE_NOT_CONNECTED


@app.get("/health")
def health() -> Dict[str, Any]:
    """存活探针。

    ``providers`` 只返回类别与数量（不返回 provider 明细，更不返回密钥）；
    ``audio_bridge`` 是**当前活跃会话的实测状态**，不是配置推断。
    """

    items = sessions.list()
    counts = {
        transport.AUDIO_BRIDGE_CONNECTED: 0,
        transport.AUDIO_BRIDGE_DEGRADED: 0,
        transport.AUDIO_BRIDGE_NOT_CONNECTED: 0,
    }
    for item in items:
        status = item.bridge()["audio_bridge"]
        counts[status] = counts.get(status, 0) + 1

    return {
        "status": "ok",
        "service": SERVICE_NAME,
        "version": VERSION,
        "providers": factory.provider_counts(),
        "audio_bridge": aggregate_bridge_status(),
        "audio_bridge_sessions": counts,
        "config_revision": store.revision,
        "sessions": sessions.count(),
        "uptime_sec": round(time.time() - _STARTED_AT, 1),
    }


# ── 配置 ────────────────────────────────────────────────────────
@app.post("/config", dependencies=[Depends(require_token)])
def update_config(payload: Dict[str, Any] = Body(...)) -> Dict[str, Any]:
    """接收后端 ``Apply`` 下发的配置，原子替换内存快照。

    返回本次生效的键；敏感键在 ``config`` 中以 ``****`` 呈现。
    """

    try:
        result = store.replace(payload)
    except ConfigValidationError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    logger.info(
        "配置已替换 revision=%d applied=%d ignored_masked=%d unknown=%d",
        result.revision,
        len(result.applied_keys),
        len(result.ignored_masked_keys),
        len(result.unknown_keys),
    )

    body: Dict[str, Any] = {"ok": True}
    body.update(result.as_dict())
    body["config"] = store.redacted()
    return body


@app.get("/config", dependencies=[Depends(require_token)])
def get_config() -> Dict[str, Any]:
    """返回当前配置（敏感键脱敏）。"""

    body: Dict[str, Any] = {"ok": True}
    body.update(store.redacted_with_meta())
    return body


# ── 会话 ────────────────────────────────────────────────────────
def _build_audio_transport(req: SessionStartRequest) -> tuple[
    transport.RTPAudioReceiver,
    Optional[transport.WebRTCPCMAPusher],
    str,
]:
    """创建并启动上行接收 + 下行推流。

    返回 ``(receiver, sender, detail)``。上行失败是硬错误（没有它整个
    闭环不成立）；下行失败只降级（用户仍能说话，只是听不到 agent），
    并把原因放进 detail，供后端/日志定位。
    """

    receiver = transport.RTPAudioReceiver(
        port=req.uplink_port,
        expected_ssrc=req.uplink_ssrc or None,
    )
    try:
        receiver.start(req.uplink_port)
    except OSError as exc:
        raise HTTPException(status_code=500, detail=f"绑定 RTP 上行端口失败：{exc}") from exc

    sender: Optional[transport.WebRTCPCMAPusher] = None
    detail = ""
    if req.downlink_stream.strip() and req.zlm_base_url.strip():
        try:
            sender = transport.WebRTCPCMAPusher(
                zlm_base_url=req.zlm_base_url,
                secret=req.zlm_secret,
                app=req.zlm_app or "rtp",
                stream=req.downlink_stream.strip(),
                ssrc=req.downlink_ssrc or 0,
            )
            if not sender.start():
                detail = "WebRTC 下行未连接（aiortc 缺失或 ZLM WebRTC push 失败）"
                logger.warning("%s stream=%s", detail, req.downlink_stream)
                sender.close()
                sender = None
        except Exception as exc:
            detail = f"WebRTC 下行启动失败：{type(exc).__name__}: {exc}"
            logger.warning("%s", detail, exc_info=True)
            if sender is not None:
                sender.close()
            sender = None
    else:
        detail = "未提供 downlink_stream/zlm_base_url，跳过下行推流"

    return receiver, sender, detail


@app.post("/session/start", dependencies=[Depends(require_token)])
def start_session(req: SessionStartRequest) -> Dict[str, Any]:
    """按**当前**配置装配 pipeline，并真正启动音频桥与 Pipeline。

    已存在的会话不受 ``/config`` 影响；要换配置请 stop 后重新 start。
    """

    session_id = req.session_id.strip()
    mode = req.mode.strip().lower()

    if sessions.count() > 0:
        # 不是硬限制，只用于日志/排查。
        logger.info("当前活跃会话数 %d，准备启动 %s", sessions.count(), session_id)

    config, revision, _updated_at = store.snapshot_with_meta()

    try:
        bundle = pipeline.build_pipeline(
            config=config,
            session_id=session_id,
            mode=mode,
            config_revision=revision,
        )
    except pipeline.PipelineConfigError as exc:
        # 中文原因直接返回给调用方（/session/start 的 400 契约）。
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except factory.UnknownProviderError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    receiver, sender, detail = _build_audio_transport(req)
    runner = pipeline.AudioPipelineRunner(
        bundle=bundle,
        config=config,
        receiver=receiver,
        sender=sender,
        half_duplex=mode == "half_duplex",
    )
    runner.start()

    state = SessionState(
        session_id=session_id,
        channel_id=req.channel_id.strip(),
        mode=mode,
        started_at=datetime.now(timezone.utc).isoformat(),
        started_monotonic=time.monotonic(),
        bundle=bundle,
        config_revision=revision,
        receiver=receiver,
        sender=sender,
        runner=runner,
        downlink_stream=req.downlink_stream.strip(),
        detail=detail,
    )
    if not sessions.add(state):
        # 409 前必须释放刚分配的 UDP 端口与 WebRTC 连接。
        runner.stop()
        if sender is not None:
            sender.close()
        receiver.close()
        raise HTTPException(status_code=409, detail=f"会话 {session_id} 已存在，请先停止")

    logger.info(
        "会话启动 session=%s channel=%s mode=%s pipeline=%s audio_bridge=%s agent=%s",
        session_id,
        state.channel_id,
        mode,
        pipeline.summary(bundle),
        state.bridge()["audio_bridge"],
        runner.agent_state,
    )

    body = {"ok": True, "started_at": state.started_at}
    body.update(state.describe())
    return body


@app.post("/session/stop", dependencies=[Depends(require_token)])
def stop_session(req: SessionStopRequest) -> Dict[str, Any]:
    """停止并清理会话（幂等：重复停止返回 404 之外的明确结果）。"""

    state = sessions.pop(req.session_id.strip())
    if state is None:
        raise HTTPException(status_code=404, detail=f"会话 {req.session_id} 不存在")

    final_bridge = state.bridge()["audio_bridge"]
    state.runner.stop()
    if state.sender is not None:
        state.sender.close()
    state.receiver.close()

    duration = round(time.monotonic() - state.started_monotonic, 3)
    logger.info("会话停止 session=%s duration=%.3fs", state.session_id, duration)

    return {
        "ok": True,
        "session_id": state.session_id,
        "stopped": True,
        "duration_sec": duration,
        "audio_bridge": final_bridge,
    }


@app.get("/sessions", dependencies=[Depends(require_token)])
def list_sessions() -> Dict[str, Any]:
    """列出活跃会话（含每路的真实音频桥状态）。"""

    items = [state.describe() for state in sessions.list()]
    return {
        "ok": True,
        "count": len(items),
        "sessions": items,
        "audio_bridge": aggregate_bridge_status(),
    }


# ── provider 连通性测试 ────────────────────────────────────────
def _resolve_endpoint(endpoint: str, timeout: float = 3.0) -> tuple[bool, str]:
    """仅做 DNS 解析（不发起 HTTP 请求，不产生计费调用）。"""

    raw = endpoint.strip()
    if not raw:
        return False, "未配置 endpoint"

    parsed = urlparse(raw if "://" in raw else f"http://{raw}")
    host = parsed.hostname
    if not host:
        return False, f"endpoint 无法解析出主机名：{raw}"
    default_port = 443 if parsed.scheme == "https" else 80
    port = parsed.port or default_port

    def _lookup() -> None:
        socket.getaddrinfo(host, port, type=socket.SOCK_STREAM)

    executor = concurrent.futures.ThreadPoolExecutor(max_workers=1)
    future = executor.submit(_lookup)
    try:
        future.result(timeout=timeout)
        return True, f"{host}:{port} 可解析"
    except concurrent.futures.TimeoutError:
        return False, f"{host} 解析超时（>{timeout:.0f}s）"
    except socket.gaierror as exc:
        return False, f"{host} 无法解析（{exc}）"
    except Exception as exc:  # 兜底：测试结果而非接口故障
        return False, f"{host} 解析失败（{type(exc).__name__}: {exc}）"
    finally:
        # 不等待卡住的 DNS 线程，避免 /test 被系统解析器拖死。
        executor.shutdown(wait=False)


@app.post("/test", dependencies=[Depends(require_token)])
def test_provider(req: TestRequest) -> Dict[str, Any]:
    """最小连通性检查。

    - 本地 provider：检查 Python 依赖是否导入成功；
    - 云端 provider：检查必要密钥是否存在、endpoint（配置值或默认值）是否可解析；
    - **不发起任何外部计费调用**，也不验证密钥的真实有效性。
    """

    started = time.monotonic()
    category = req.category.strip().lower()
    if category not in factory.CATEGORIES:
        raise HTTPException(
            status_code=400,
            detail=f"category 仅支持 {' / '.join(factory.CATEGORIES)}",
        )

    config = store.snapshot()
    provider = (req.provider or config.get(f"{category}_provider") or "").strip().lower()
    if not provider:
        return {
            "ok": False,
            "detail": f"未配置 {category}_provider",
            "category": category,
            "provider": "",
            "elapsed_ms": 0,
        }

    try:
        spec = factory.get_provider(category, provider)
    except factory.UnknownProviderError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    probe = factory.probe(spec)
    problems: list[str] = []
    notes: list[str] = []

    if not probe.installed:
        problems.append(f"依赖未安装：{probe.detail}")
    else:
        notes.append("依赖已导入")

    api_key = str(config.get(f"{category}_api_key") or "").strip()
    if spec.requires_api_key:
        if api_key:
            notes.append("API Key 已配置")
        else:
            problems.append(f"缺少 {category}_api_key")

    endpoint = str(config.get(f"{category}_endpoint") or "").strip()
    if not endpoint and spec.default_endpoint:
        endpoint = spec.default_endpoint
    if endpoint:
        if spec.local:
            notes.append("本地 provider，endpoint 仅作可选覆盖")
        else:
            resolvable, detail = _resolve_endpoint(endpoint)
            if resolvable:
                notes.append(detail)
            else:
                problems.append(detail)
    elif spec.requires_endpoint:
        problems.append(f"缺少 {category}_endpoint")
    elif spec.local:
        notes.append("本地 provider，无需联网检查")
    else:
        notes.append("未配置 endpoint，使用 SDK 默认地址（未做 DNS 检查）")

    ok = not problems
    if ok:
        detail_text = "连通性检查通过：" + "；".join(notes)
    else:
        detail_text = "连通性检查未通过：" + "；".join(problems)

    return {
        "ok": ok,
        "detail": detail_text,
        "category": category,
        "provider": provider,
        "installed": probe.installed,
        "service": probe.resolved,
        "requires_api_key": spec.requires_api_key,
        "endpoint": endpoint or None,
        "notes": notes,
        "problems": problems,
        "elapsed_ms": round((time.monotonic() - started) * 1000, 1),
    }


__all__ = (
    "aggregate_bridge_status",
    "app",
    "health",
    "list_sessions",
    "start_session",
    "stop_session",
    "test_provider",
)
