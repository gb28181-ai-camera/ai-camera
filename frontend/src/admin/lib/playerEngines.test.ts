import { describe, expect, it } from 'vitest'
import {
  buildEngineUrl,
  fallbackChain,
  isH265Codec,
  resolveDefaultEngine,
  resolveEngineUrl,
  resolvePlayerStaticUrl,
  toWebSocketBase,
} from './playerEngines'

describe('playerEngines / 默认引擎', () => {
  it('显式 defaultEngine 优先于 codec', () => {
    expect(resolveDefaultEngine({ defaultEngine: 'flv', codec: 'H265' })).toBe('flv')
    expect(resolveDefaultEngine({ defaultEngine: 'webrtc', codec: 'H265' })).toBe('webrtc')
  })

  it('codec=H265/HEVC 默认 h265web，其余默认 webrtc', () => {
    expect(resolveDefaultEngine({ codec: 'H265' })).toBe('h265web')
    expect(resolveDefaultEngine({ codec: 'h265' })).toBe('h265web')
    expect(resolveDefaultEngine({ codec: 'H.265' })).toBe('h265web')
    expect(resolveDefaultEngine({ codec: 'HEVC' })).toBe('h265web')
    expect(resolveDefaultEngine({ codec: 'H264' })).toBe('webrtc')
    expect(resolveDefaultEngine({})).toBe('webrtc')
  })

  it('isH265Codec 识别常见写法', () => {
    expect(isH265Codec('H265')).toBe(true)
    expect(isH265Codec('hevc')).toBe(true)
    expect(isH265Codec('H.265')).toBe(true)
    expect(isH265Codec('H264')).toBe(false)
    expect(isH265Codec(undefined)).toBe(false)
  })
})

describe('playerEngines / 回落链', () => {
  it('webrtc → flv → hls', () => {
    expect(fallbackChain('webrtc')).toEqual(['webrtc', 'flv', 'hls'])
  })

  it('h265web → hls → flv', () => {
    expect(fallbackChain('h265web')).toEqual(['h265web', 'hls', 'flv'])
  })

  it('flv 默认只回落到 hls，hls/jessibuca 不回落', () => {
    expect(fallbackChain('flv')).toEqual(['flv', 'hls'])
    expect(fallbackChain('hls')).toEqual(['hls'])
    expect(fallbackChain('jessibuca')).toEqual(['jessibuca'])
  })

  it('按可用引擎过滤回落链', () => {
    expect(fallbackChain('webrtc', ['webrtc', 'hls'])).toEqual(['webrtc', 'hls'])
    expect(fallbackChain('h265web', ['flv', 'h265web'])).toEqual(['h265web', 'flv'])
  })
})

describe('playerEngines / 地址映射', () => {
  const base = { app: 'rtp', stream: '0BEBC201', httpBase: 'http://192.168.1.227:8080/' }

  it('webrtc 使用 ZLM WHEP 接口', () => {
    expect(buildEngineUrl('webrtc', base)).toBe(
      'http://192.168.1.227:8080/index/api/webrtc?app=rtp&stream=0BEBC201&type=play',
    )
  })

  it('flv 与 hls 使用 HTTP 地址', () => {
    expect(buildEngineUrl('flv', base)).toBe('http://192.168.1.227:8080/rtp/0BEBC201.live.flv')
    expect(buildEngineUrl('hls', base)).toBe('http://192.168.1.227:8080/rtp/0BEBC201/hls.m3u8')
  })

  it('jessibuca / h265web 使用 ws-flv', () => {
    expect(buildEngineUrl('jessibuca', base)).toBe('ws://192.168.1.227:8080/rtp/0BEBC201.live.flv')
    expect(buildEngineUrl('h265web', base)).toBe('ws://192.168.1.227:8080/rtp/0BEBC201.live.flv')
    expect(buildEngineUrl('h265web', { ...base, httpBase: 'https://example.com' })).toBe(
      'wss://example.com/rtp/0BEBC201.live.flv',
    )
  })

  it('https → wss，已有 ws 协议保持不变', () => {
    expect(toWebSocketBase('https://example.com/')).toBe('wss://example.com')
    expect(toWebSocketBase('ws://example.com')).toBe('ws://example.com')
    expect(toWebSocketBase('wss://example.com')).toBe('wss://example.com')
  })

  it('缺少 httpBase / stream 时返回空串', () => {
    expect(buildEngineUrl('flv', { app: 'rtp', stream: 'x' })).toBe('')
    expect(buildEngineUrl('flv', { app: 'rtp', httpBase: 'http://x' })).toBe('')
  })

  it('urls 覆盖优先于直给 url，直给 url 优先于拼装', () => {
    const source = {
      ...base,
      url: 'http://direct.example/live.flv',
      urls: { hls: 'http://override.example/live.m3u8' },
    }
    expect(resolveEngineUrl('hls', source)).toBe('http://override.example/live.m3u8')
    expect(resolveEngineUrl('flv', source)).toBe('http://direct.example/live.flv')
    expect(resolveEngineUrl('webrtc', base)).toContain('/index/api/webrtc')
  })

  it('空地址返回空串（组件进入占位而不是报错）', () => {
    expect(resolveEngineUrl('webrtc', {})).toBe('')
  })
})

describe('playerEngines / 静态资源路径', () => {
  it('拼接 Vite BASE_URL（测试环境为 /）', () => {
    expect(resolvePlayerStaticUrl('static/js/h265web/h265web.js')).toBe(
      '/static/js/h265web/h265web.js',
    )
    expect(resolvePlayerStaticUrl('/static/js/jessibuca/jessibuca.js')).toBe(
      '/static/js/jessibuca/jessibuca.js',
    )
  })
})
