import React from 'react'
import { render, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import LivePlayer from './LivePlayer'

interface MockHlsInstance {
  loadSource: (url: string) => void
  attachMedia: (element: HTMLMediaElement) => void
  on: (event: string, handler: (event: unknown, data: unknown) => void) => void
  off: (event: string, handler: (event: unknown, data: unknown) => void) => void
  destroy: () => void
}

const hlsState = vi.hoisted(() => ({
  supported: true,
  instances: [] as MockHlsInstance[],
}))

vi.mock('hls.js', () => {
  class MockHls {
    static Events = { ERROR: 'hlsError' }

    static isSupported(): boolean {
      return hlsState.supported
    }

    loadSource = vi.fn()
    attachMedia = vi.fn()
    on = vi.fn()
    off = vi.fn()
    destroy = vi.fn()

    constructor() {
      hlsState.instances.push(this)
    }
  }

  return { default: MockHls }
})

const originalPlay = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'play')
const originalPause = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'pause')
const originalCanPlayType = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'canPlayType')

function stubPlay(): void {
  Object.defineProperty(HTMLMediaElement.prototype, 'play', {
    configurable: true,
    writable: true,
    value: vi.fn().mockResolvedValue(undefined),
  })
  Object.defineProperty(HTMLMediaElement.prototype, 'pause', {
    configurable: true,
    writable: true,
    value: vi.fn(),
  })
}

function stubCanPlayType(result: string): ReturnType<typeof vi.fn> {
  const spy = vi.fn(() => result)
  Object.defineProperty(HTMLMediaElement.prototype, 'canPlayType', {
    configurable: true,
    writable: true,
    value: spy,
  })
  return spy
}

describe('LivePlayer / HLS 分支', () => {
  beforeEach(() => {
    hlsState.supported = true
    hlsState.instances.length = 0
    stubPlay()
    stubCanPlayType('')
  })

  afterEach(() => {
    if (originalPlay) Object.defineProperty(HTMLMediaElement.prototype, 'play', originalPlay)
    if (originalPause) Object.defineProperty(HTMLMediaElement.prototype, 'pause', originalPause)
    if (originalCanPlayType) {
      Object.defineProperty(HTMLMediaElement.prototype, 'canPlayType', originalCanPlayType)
    }
    vi.clearAllMocks()
  })

  it('Hls.isSupported() 为 true 时用 hls.js 播放，卸载时 destroy', async () => {
    const url = 'http://media.example/rtp/ABC/hls.m3u8'
    const { container, unmount } = render(<LivePlayer url={url} protocol="hls" />)

    await waitFor(() => expect(hlsState.instances).toHaveLength(1))
    const instance = hlsState.instances[0]
    expect(instance.loadSource).toHaveBeenCalledWith(url)
    expect(instance.attachMedia).toHaveBeenCalledWith(expect.any(HTMLVideoElement))

    await waitFor(() => {
      expect(container.querySelector<HTMLElement>('.e2e-live-player')?.getAttribute('data-status')).toBe(
        'playing',
      )
    })

    unmount()
    expect(instance.destroy).toHaveBeenCalled()
  })

  it('MSE 不可用但支持原生 HLS 时退回 video.src', async () => {
    hlsState.supported = false
    const canPlayType = stubCanPlayType('maybe')
    const url = 'http://media.example/rtp/ABC/hls.m3u8'

    const { container } = render(<LivePlayer url={url} protocol="hls" />)

    await waitFor(() => expect(canPlayType).toHaveBeenCalledWith('application/vnd.apple.mpegurl'))
    expect(hlsState.instances).toHaveLength(0)
    await waitFor(() => {
      expect(container.querySelector('video')?.getAttribute('src')).toBe(url)
    })
  })

  it('hls.js 与原生 HLS 都不可用时给出明确错误', async () => {
    hlsState.supported = false
    stubCanPlayType('')
    const onError = vi.fn()

    const { container } = render(
      <LivePlayer url="http://media.example/rtp/ABC/hls.m3u8" protocol="hls" onError={onError} />,
    )

    await waitFor(() => expect(onError).toHaveBeenCalled())
    const error = onError.mock.calls[0]?.[0] as Error | undefined
    expect(error?.message).toMatch(/不支持 HLS/)

    await waitFor(() => {
      expect(container.querySelector<HTMLElement>('.e2e-live-player')?.getAttribute('data-status')).toBe(
        'error',
      )
    })
  })
})
