"""快速应答缓存单测。"""

from __future__ import annotations

import asyncio

import pytest

from ws.ack import QuickAckCache


@pytest.mark.asyncio
async def test_cache_reuses_pcm():
    calls = []

    async def synth(text: str, voice: str, speed: float) -> bytes:
        calls.append((text, voice, speed))
        return b"\x00\x00" * 10

    cache = QuickAckCache(synth, default_text="我在")
    first = await cache.get(voice="v1", speed=1.0)
    second = await cache.get(voice="v1", speed=1.0)
    assert first == second == b"\x00\x00" * 10
    assert len(calls) == 1
    assert cache.cached(voice="v1", speed=1.0)


@pytest.mark.asyncio
async def test_different_voice_not_shared():
    calls = []

    async def synth(text: str, voice: str, speed: float) -> bytes:
        calls.append(voice)
        return voice.encode()

    cache = QuickAckCache(synth)
    assert await cache.get(voice="v1", speed=1.0) == b"v1"
    assert await cache.get(voice="v2", speed=1.0) == b"v2"
    assert calls == ["v1", "v2"]


@pytest.mark.asyncio
async def test_timeout_returns_none_but_task_keeps_running():
    started = asyncio.Event()

    async def slow_synth(text: str, voice: str, speed: float) -> bytes:
        started.set()
        await asyncio.sleep(0.1)
        return b"pcm"

    cache = QuickAckCache(slow_synth)
    assert await cache.get(voice="v", speed=1.0, timeout=0.01) is None
    await started.wait()
    await asyncio.sleep(0.15)
    assert cache.cached(voice="v", speed=1.0)
    assert await cache.get(voice="v", speed=1.0) == b"pcm"


@pytest.mark.asyncio
async def test_failure_returns_none():
    async def bad_synth(text: str, voice: str, speed: float) -> bytes:
        raise RuntimeError("edge tts down")

    cache = QuickAckCache(bad_synth)
    assert await cache.get(voice="v", speed=1.0) is None


@pytest.mark.asyncio
async def test_prewarm_does_not_block():
    async def slow_synth(text: str, voice: str, speed: float) -> bytes:
        await asyncio.sleep(0.2)
        return b"pcm"

    cache = QuickAckCache(slow_synth)
    loop = asyncio.get_running_loop()
    started = loop.time()
    await cache.prewarm(voice="v", speed=1.0)
    assert loop.time() - started < 0.15
