import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { Alert, Button, Result, Spin, Tag } from 'antd'
import { useParams } from 'react-router-dom'
import LivePlayerTabs from '../../components/LivePlayerTabs'
import { GbShareController } from '../../api/generated/GbShareController'
import type { GbShareInfoResp } from '../../api/generated/definitions/GbShareInfoResp'
import type { GbSharePlayResp } from '../../api/generated/definitions/GbSharePlayResp'
import { formatDT } from '../../types/business'

/**
 * 匿名分享播放页（`/share/gb/:token`，不在 ProtectedRoute 内）。
 *
 * 匿名接口 `/public/gb/shares/*` 的令牌无效 / 过期 / 吊销统一返回 401；
 * 而全局 axios 拦截器会在 401 时清 token 并跳转登录页，这会让匿名访客
 * 直接看到后台登录页。因此本页改为 **fetch + generated 路径常量** 直连匿名
 * 接口，自行把 401/404/429/503 映射成明确的中文提示，不改动公共拦截器。
 */

/** 匿名接口错误（带 HTTP 状态码，便于映射过期/吊销等提示） */
class ShareRequestError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ShareRequestError'
    this.status = status
  }
}

interface ApiEnvelope<T> {
  code?: number
  message?: string
  data?: T
}

async function publicRequest<T>(path: string, init?: RequestInit): Promise<T | null> {
  const base = import.meta.env.VITE_API_BASE || ''
  const headers: Record<string, string> = { ...(init?.headers as Record<string, string> | undefined) }
  // 只有带 body 的请求才声明 JSON，GET 保持简单请求（减少匿名端跨域预检）
  if (init?.body) headers['Content-Type'] = 'application/json'

  const response = await fetch(`${base}${path}`, { ...init, headers, cache: 'no-store' })

  let envelope: ApiEnvelope<T> | null = null
  try {
    envelope = (await response.json()) as ApiEnvelope<T>
  } catch {
    envelope = null
  }

  if (!response.ok || (envelope?.code !== undefined && envelope.code !== 200)) {
    throw new ShareRequestError(
      response.status,
      envelope?.message || `请求失败（HTTP ${response.status}）`,
    )
  }
  return envelope?.data ?? null
}

/** 把匿名接口错误翻译成访客能理解的提示 */
function shareErrorMessage(err: unknown): string {
  if (err instanceof ShareRequestError) {
    switch (err.status) {
      case 401:
        return '分享链接无效、已过期或已被吊销，请联系分享者重新生成'
      case 404:
        return '分享的通道不存在或已被删除'
      case 429:
        return '访问过于频繁，请稍后再试'
      case 503:
        return '信令服务暂不可用，请稍后重试'
      default:
        return err.message
    }
  }
  return err instanceof Error ? err.message : '网络异常，请稍后重试'
}

/** 只透出实际返回的协议地址，空值不占用引擎位 */
function buildPlayUrls(
  play: GbSharePlayResp | null,
): Partial<Record<'webrtc' | 'flv' | 'hls', string>> | undefined {
  if (!play) return undefined
  const urls: Partial<Record<'webrtc' | 'flv' | 'hls', string>> = {}
  if (play.webrtc) urls.webrtc = play.webrtc
  if (play.flv) urls.flv = play.flv
  if (play.hls) urls.hls = play.hls
  return urls
}

const GbSharePlayPage: React.FC = () => {
  const { token } = useParams<{ token: string }>()

  const [info, setInfo] = useState<GbShareInfoResp | null>(null)
  const [play, setPlay] = useState<GbSharePlayResp | null>(null)
  const [loading, setLoading] = useState(true)
  const [playLoading, setPlayLoading] = useState(false)
  const [error, setError] = useState('')
  const [playError, setPlayError] = useState('')

  const loadInfo = useCallback(async () => {
    if (!token) {
      setError('分享链接缺少令牌')
      setLoading(false)
      return
    }
    setLoading(true)
    setError('')
    setPlay(null)
    setPlayError('')
    try {
      const path = GbShareController.getPublicGbShareInfoPath.replace(
        '{token}',
        encodeURIComponent(token),
      )
      const data = await publicRequest<GbShareInfoResp>(path)
      setInfo(data)
    } catch (err) {
      setError(shareErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => {
    void loadInfo()
  }, [loadInfo])

  const loadPlay = useCallback(async () => {
    if (!token) return
    setPlayLoading(true)
    setPlayError('')
    try {
      const path = GbShareController.playPublicGbSharePath.replace(
        '{token}',
        encodeURIComponent(token),
      )
      const data = await publicRequest<GbSharePlayResp>(path, {
        method: 'POST',
        body: JSON.stringify({ protocol: 'webrtc' }),
      })
      setPlay(data)
      if (!data) setPlayError('后端未返回播放地址')
    } catch (err) {
      setPlayError(shareErrorMessage(err))
    } finally {
      setPlayLoading(false)
    }
  }, [token])

  useEffect(() => {
    if (!info) return
    void loadPlay()
  }, [info, loadPlay])

  const urls = useMemo(() => buildPlayUrls(play), [play])

  return (
    <div
      className="e2e-gb-share-play-page"
      style={{
        minHeight: '100vh',
        display: 'flex',
        flexDirection: 'column',
        background: '#0b1220',
        color: '#e6ecf7',
      }}
    >
      <header
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 12,
          padding: '14px 24px',
          borderBottom: '1px solid rgba(255,255,255,0.08)',
        }}
      >
        <div style={{ minWidth: 0 }}>
          <div style={{ fontSize: 16, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {info?.channel_name || '国标视频分享'}
          </div>
          <div style={{ fontSize: 12, color: '#8aa0c0', marginTop: 2 }}>
            {[info?.device_name, info?.gb_channel_id].filter(Boolean).join(' · ') || '匿名分享'}
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flex: '0 0 auto' }}>
          {info?.expires_dt ? (
            <Tag color="blue" style={{ marginInlineEnd: 0 }}>
              有效期至 {formatDT(info.expires_dt)}
            </Tag>
          ) : null}
          {play?.protocol ? <Tag style={{ marginInlineEnd: 0 }}>{play.protocol.toUpperCase()}</Tag> : null}
        </div>
      </header>

      <main
        style={{
          flex: 1,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          padding: 16,
          minHeight: 0,
        }}
      >
        {loading ? (
          <Spin size="large" tip="正在验证分享链接..." />
        ) : error ? (
          <Result
            className="e2e-gb-share-play-error"
            status="warning"
            title="无法播放该分享"
            subTitle={error}
            extra={<Button onClick={() => void loadInfo()}>重试</Button>}
          />
        ) : playLoading ? (
          <Spin size="large" tip="正在获取播放地址..." />
        ) : play ? (
          <div
            className="e2e-gb-share-play-player"
            style={{
              width: 'min(1360px, 100%)',
              height: 'calc(100vh - 120px)',
              minHeight: 360,
              background: '#000',
              borderRadius: 8,
              overflow: 'hidden',
            }}
          >
            <LivePlayerTabs
              app={play.app}
              stream={play.stream}
              urls={urls}
              objectFit="contain"
              style={{ width: '100%', height: '100%' }}
              onError={(err) => setPlayError(err.message)}
            />
          </div>
        ) : (
          <Result
            className="e2e-gb-share-play-error"
            status="info"
            title="暂无可用播放地址"
            subTitle={playError || '请稍后重试，或联系分享者确认通道是否在线'}
            extra={<Button onClick={() => void loadPlay()}>重新获取</Button>}
          />
        )}
      </main>

      {play && playError ? (
        <Alert
          className="e2e-gb-share-play-notice"
          type="warning"
          banner
          showIcon
          message={`播放异常：${playError}`}
        />
      ) : null}

      <footer style={{ padding: '10px 24px', fontSize: 12, color: '#61748f', textAlign: 'center' }}>
        该页面为免登录分享，链接过期或吊销后自动失效。
      </footer>
    </div>
  )
}

export default GbSharePlayPage
