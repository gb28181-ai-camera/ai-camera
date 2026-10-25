"""快速应答（默认「我在」）的预合成缓存。

设备唤醒后要求**立即**有反馈，不能等 STT/LLM。这里的策略：

1. 连接建立后立刻在后台触发一次合成（prewarm）；
2. ``listen/detect`` / ``listen/start`` 时优先读缓存；
3. 缓存未命中时也不阻塞太久（默认 2s 超时），文字消息先发；
4. 缓存键包含 voice/speed/text，跨会话复用（同一个进程内）。

缓存只存 PCM16，不存原始音频文件。Opus 客户端在每次会话里即时编码，
因为 0.5s 的短句编码成本可以忽略。
"""

from __future__ import annotations

import asyncio
import logging
import time
from typing import Awaitable, Callable, Dict, Optional

logger = logging.getLogger("ai-camera-pipecat.ws.ack")

AckSynthesizer = Callable[[str, str, float], Awaitable[bytes]]


class QuickAckCache:
    """进程级异步缓存：同一句话只合成一次。"""

    def __init__(self, synthesizer: AckSynthesizer, default_text: str = "我在") -> None:
        self._synthesizer = synthesizer
        self.default_text = default_text
        self._futures: Dict[str, asyncio.Future] = {}
        self._created_at: Dict[str, float] = {}
        self._tasks: set = set()

    @staticmethod
    def _key(text: str, voice: str, speed: float) -> str:
        return f"{voice}|{speed:.3f}|{text}"

    async def prewarm(self, text: Optional[str] = None, *, voice: str, speed: float) -> None:
        """触发后台合成但不等待（缓存未命中时创建任务）。"""

        try:
            await self.get(text, voice=voice, speed=speed, timeout=0.01)
        except Exception:
            pass

    async def get(
        self,
        text: Optional[str] = None,
        *,
        voice: str,
        speed: float,
        timeout: float = 2.0,
    ) -> Optional[bytes]:
        """取 PCM16（24kHz）；合成失败/超时返回 None。"""

        phrase = (text or self.default_text).strip() or self.default_text
        key = self._key(phrase, voice, float(speed))

        future = self._futures.get(key)
        if future is None:
            loop = asyncio.get_running_loop()
            future = loop.create_future()
            self._futures[key] = future
            self._created_at[key] = time.monotonic()
            task = asyncio.create_task(self._run(phrase, voice, float(speed), future, key))
            self._tasks.add(task)
            task.add_done_callback(self._tasks.discard)

        try:
            result = await asyncio.wait_for(asyncio.shield(future), timeout=max(0.01, timeout))
        except (asyncio.TimeoutError, TimeoutError):
            logger.info("快速应答未在 %.1fs 内合成完成，跳过音频（文字已发送）", timeout)
            return None
        except Exception:
            return None
        return result or None

    async def _run(
        self,
        phrase: str,
        voice: str,
        speed: float,
        future: "asyncio.Future[Optional[bytes]]",
        key: str,
    ) -> None:
        try:
            pcm = await self._synthesizer(phrase, voice, speed)
        except Exception as exc:
            logger.warning("快速应答合成失败：%s", type(exc).__name__)
            pcm = b""
        if future.cancelled():
            return
        if not future.done():
            future.set_result(pcm or None)
        else:  # pragma: no cover - 防御性
            return
        if not pcm:
            # 失败的结果不长期缓存，允许下一次重试（但保留 30s 避免风暴）。
            asyncio.get_running_loop().call_later(
                30.0, lambda: self._evict(key)
            )

    def _evict(self, key: str) -> None:
        future = self._futures.get(key)
        if future is None:
            return
        if future.done() and not future.cancelled():
            try:
                if future.result():
                    return  # 成功的结果永久缓存
            except Exception:
                pass
        self._futures.pop(key, None)
        self._created_at.pop(key, None)

    def cached(self, text: Optional[str] = None, *, voice: str, speed: float) -> bool:
        future = self._futures.get(self._key((text or self.default_text), voice, speed))
        return bool(future is not None and future.done() and not future.cancelled() and future.result())

    def clear(self) -> None:
        for future in self._futures.values():
            if not future.done():
                future.cancel()
        self._futures.clear()
        self._created_at.clear()


__all__ = ("AckSynthesizer", "QuickAckCache")
