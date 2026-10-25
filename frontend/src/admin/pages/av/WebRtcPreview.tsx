import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  Badge,
  Button,
  Card,
  Col,
  Descriptions,
  Empty,
  Row,
  Select,
  Space,
  Statistic,
  Tag,
  Typography,
  message,
} from 'antd'
import {
  DisconnectOutlined,
  LinkOutlined,
  ReloadOutlined,
  VideoCameraOutlined,
} from '@ant-design/icons'
import { AvRoomController } from '../../api/generated/AvRoomController'
import { useAuthStore } from '../../store/authStore'

const { Text, Title } = Typography

/** 后端 /web/av/ws 推送的消息（与 Go internal/signal/room.Message 对齐）。 */
interface SignalMessage {
  type: string
  room_id?: string
  device_code?: string
  client_id?: string
  to?: string
  role?: string
  sdp?: string
  sdp_type?: string
  candidate?: RTCIceCandidateInit | null
  reason?: string
  code?: string
  message?: string
  device_online?: boolean
}

interface DeviceOption {
  id: number
  name?: string
  device_code?: string
  status?: string
}

interface LogLine {
  id: number
  at: string
  direction: 'in' | 'out' | 'state' | 'error'
  text: string
}

/**
 * 实时音视频 → 查看设备画面。
 *
 * 信令拓扑：
 *   浏览器 ──WS──▶ Go ──MQTT──▶ 设备
 * 媒体拓扑：
 *   设备 ──WebRTC（DTLS/SRTP，必要时经 TURN）──▶ 浏览器
 *
 * 设备是 offer 方：查看方只发 joinRoom / answer / iceCandidate / leaveRoom。
 * 设备重新 offer（ICE restart）时，本页会按新 offer 重新 answer。
 */
export default function WebRtcPreview() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canConnect = permCodes.includes('av:preview:connect')

  const [devices, setDevices] = useState<DeviceOption[]>([])
  const [deviceID, setDeviceID] = useState<number | undefined>()
  const [loadingDevices, setLoadingDevices] = useState(false)
  const [connecting, setConnecting] = useState(false)
  const [connected, setConnected] = useState(false)
  const [roomID, setRoomID] = useState('')
  const [clientID, setClientID] = useState('')
  const [deviceOnline, setDeviceOnline] = useState(false)
  const [iceState, setIceState] = useState('new')
  const [pcState, setPcState] = useState('new')
  const [resolution, setResolution] = useState('-')
  const [fps, setFps] = useState(0)
  const [bitrate, setBitrate] = useState(0)
  const [logs, setLogs] = useState<LogLine[]>([])

  const videoRef = useRef<HTMLVideoElement | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const pcRef = useRef<RTCPeerConnection | null>(null)
  const pendingCandidates = useRef<RTCIceCandidateInit[]>([])
  const remoteReady = useRef(false)
  const statsTimer = useRef<number | null>(null)
  const lastBytes = useRef<{ bytes: number; at: number }>({ bytes: 0, at: 0 })
  const logSeq = useRef(0)

  const addLog = useCallback((direction: LogLine['direction'], text: string) => {
    logSeq.current += 1
    const line: LogLine = {
      id: logSeq.current,
      at: new Date().toLocaleTimeString(),
      direction,
      text,
    }
    setLogs((prev) => [line, ...prev].slice(0, 120))
  }, [])

  const loadDevices = useCallback(async () => {
    setLoadingDevices(true)
    try {
      const res = await AvRoomController.pageAvDevices(1, 200)
      const items = (res.data?.items ?? []) as DeviceOption[]
      setDevices(items)
      if (!deviceID && items.length > 0) setDeviceID(items[0].id)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '加载设备失败')
    } finally {
      setLoadingDevices(false)
    }
    // deviceID 只用于首次默认选中，不参与请求。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    void loadDevices()
  }, [loadDevices])

  const send = useCallback((payload: Record<string, unknown>) => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) return false
    const text = JSON.stringify(payload)
    ws.send(text)
    addLog('out', text)
    return true
  }, [addLog])

  const stopStats = useCallback(() => {
    if (statsTimer.current !== null) {
      window.clearInterval(statsTimer.current)
      statsTimer.current = null
    }
    lastBytes.current = { bytes: 0, at: 0 }
  }, [])

  const startStats = useCallback((pc: RTCPeerConnection) => {
    stopStats()
    statsTimer.current = window.setInterval(async () => {
      try {
        const report = await pc.getStats()
        report.forEach((item) => {
          if (item.type !== 'inbound-rtp' || item.kind !== 'video') return
          const stat = item as RTCInboundRtpStreamStats & {
            frameWidth?: number
            frameHeight?: number
            framesPerSecond?: number
            bytesReceived?: number
          }
          if (stat.frameWidth && stat.frameHeight) {
            setResolution(`${stat.frameWidth}×${stat.frameHeight}`)
          }
          setFps(Math.round(stat.framesPerSecond ?? 0))
          const bytes = Number(stat.bytesReceived ?? 0)
          const now = Date.now()
          if (lastBytes.current.at > 0 && now > lastBytes.current.at) {
            const kbps = ((bytes - lastBytes.current.bytes) * 8) / (now - lastBytes.current.at)
            setBitrate(Math.max(0, Math.round(kbps)))
          }
          lastBytes.current = { bytes, at: now }
        })
      } catch {
        // getStats 在连接关闭瞬间可能抛错，忽略即可。
      }
    }, 1000)
  }, [stopStats])

  const closePeer = useCallback(() => {
    stopStats()
    // 注意：不要清空 pendingCandidates。
    // 设备发 offer 前可能先发 ICE candidate；createPeer() 会调用本函数，
    // 若在这里清空，会把「先于 offer 到达的 candidate」丢掉。
    remoteReady.current = false
    if (pcRef.current) {
      pcRef.current.ontrack = null
      pcRef.current.onicecandidate = null
      pcRef.current.onconnectionstatechange = null
      pcRef.current.oniceconnectionstatechange = null
      pcRef.current.close()
      pcRef.current = null
    }
    if (videoRef.current) videoRef.current.srcObject = null
    setIceState('closed')
    setPcState('closed')
    setResolution('-')
    setFps(0)
    setBitrate(0)
  }, [stopStats])

  const flushCandidates = useCallback(async (pc: RTCPeerConnection) => {
    const queued = pendingCandidates.current.splice(0)
    for (const candidate of queued) {
      try {
        await pc.addIceCandidate(candidate)
      } catch (err) {
        addLog('error', `addIceCandidate 失败: ${String(err)}`)
      }
    }
  }, [addLog])

  const createPeer = useCallback(
    (iceServers: RTCIceServer[]) => {
      closePeer()
      const pc = new RTCPeerConnection({
        iceServers,
        bundlePolicy: 'max-bundle',
      })
      pcRef.current = pc

      pc.ontrack = (event) => {
        const stream = event.streams[0] ?? new MediaStream([event.track])
        if (videoRef.current) {
          videoRef.current.srcObject = stream
          void videoRef.current.play().catch(() => undefined)
        }
        addLog('state', `ontrack kind=${event.track.kind}`)
        startStats(pc)
      }
      pc.onicecandidate = (event) => {
        if (!event.candidate) return
        send({ type: 'iceCandidate', candidate: event.candidate.toJSON() })
      }
      pc.onconnectionstatechange = () => {
        setPcState(pc.connectionState)
        addLog('state', `connection=${pc.connectionState}`)
        if (pc.connectionState === 'failed') {
          addLog('error', 'WebRTC 连接失败，可点击「重新协商」或检查 TURN')
        }
      }
      pc.oniceconnectionstatechange = () => {
        setIceState(pc.iceConnectionState)
        addLog('state', `ice=${pc.iceConnectionState}`)
      }
      return pc
    },
    [addLog, closePeer, send, startStats],
  )

  const handleOffer = useCallback(
    async (sdp: string) => {
      const pc = pcRef.current
      if (!pc) {
        addLog('error', '收到 offer 但 PeerConnection 不存在')
        return
      }
      // 设备可能在中途重新 offer（ICE restart / 新增轨道）。
      if (pc.signalingState !== 'stable') {
        try {
          await pc.setRemoteDescription({ type: 'rollback' })
          addLog('state', '已 rollback 旧 offer，接受新 offer')
        } catch (err) {
          addLog('error', `rollback 失败: ${String(err)}`)
        }
      }
      await pc.setRemoteDescription({ type: 'offer', sdp })
      remoteReady.current = true
      await flushCandidates(pc)
      const answer = await pc.createAnswer()
      await pc.setLocalDescription(answer)
      send({ type: 'answer', sdp: answer.sdp, sdp_type: 'answer' })
      addLog('state', '已发送 answer')
    },
    [addLog, flushCandidates, send],
  )

  const handleSignal = useCallback(
    async (msg: SignalMessage, iceServers: RTCIceServer[]) => {
      switch (msg.type) {
        case 'ready':
          setRoomID(msg.room_id ?? '')
          setClientID(msg.client_id ?? '')
          setDeviceOnline(Boolean(msg.device_online))
          addLog('state', `ready room=${msg.room_id} client=${msg.client_id}`)
          // 查看方进入房间，发第一次 joinRoom。
          send({ type: 'joinRoom' })
          break
        case 'joinRoom':
          if (msg.role === 'device') {
            setDeviceOnline(true)
            addLog('state', '设备已进入房间')
            // 约定：查看方收到设备的 joinRoom 后再发一次 joinRoom，
            // 设备收到后创建 offer。
            send({ type: 'joinRoom' })
          }
          break
        case 'offer':
          if (!pcRef.current) createPeer(iceServers)
          await handleOffer(msg.sdp ?? '')
          break
        case 'answer':
          // 当前设计由设备发 offer，answer 只用于未来查看方主动协商。
          if (pcRef.current && pcRef.current.signalingState === 'have-local-offer') {
            await pcRef.current.setRemoteDescription({ type: 'answer', sdp: msg.sdp ?? '' })
            remoteReady.current = true
            await flushCandidates(pcRef.current)
          }
          break
        case 'iceCandidate':
          if (msg.candidate && msg.candidate.candidate) {
            if (pcRef.current && remoteReady.current) {
              await pcRef.current.addIceCandidate(msg.candidate).catch((err) => {
                addLog('error', `addIceCandidate 失败: ${String(err)}`)
              })
            } else {
              pendingCandidates.current.push(msg.candidate)
            }
          }
          break
        case 'leaveRoom':
          setDeviceOnline(false)
          addLog('state', `设备离开房间${msg.reason ? `（${msg.reason}）` : ''}`)
          break
        case 'error':
          addLog('error', `${msg.code ?? 'error'}: ${msg.message ?? ''}`)
          break
        default:
          addLog('state', `未知消息类型 ${msg.type}`)
      }
    },
    [addLog, createPeer, flushCandidates, handleOffer, send],
  )

  const disconnect = useCallback(
    (notify = true) => {
      if (notify) send({ type: 'leaveRoom', reason: 'user_closed' })
      const ws = wsRef.current
      wsRef.current = null
      if (ws) {
        ws.onopen = null
        ws.onmessage = null
        ws.onerror = null
        ws.onclose = null
        if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
          ws.close()
        }
      }
      closePeer()
      pendingCandidates.current = []
      remoteReady.current = false
      setConnected(false)
      setDeviceOnline(false)
      setRoomID('')
      setClientID('')
    },
    [closePeer, send],
  )

  const connect = useCallback(async () => {
    if (!deviceID) {
      message.warning('请先选择设备')
      return
    }
    disconnect(false)
    setConnecting(true)
    try {
      const res = await AvRoomController.createAvViewerTicket(deviceID)
      const info = res.data
      if (!info?.ticket || !info.ws_url) throw new Error('ticket 响应缺少 ws_url')

      const iceServers = (info.ice_servers ?? [])
        .filter((s) => (s.urls?.length ?? 0) > 0)
        .map((s) => ({
          urls: s.urls ?? [],
          username: s.username || undefined,
          credential: s.credential || undefined,
        })) as RTCIceServer[]

      pendingCandidates.current = []
      remoteReady.current = false
      createPeer(iceServers)
      setRoomID(info.room_id ?? '')
      setClientID(info.client_id ?? '')
      setDeviceOnline(Boolean(info.device_online))

      const sep = info.ws_url.includes('?') ? '&' : '?'
      const ws = new WebSocket(`${info.ws_url}${sep}ticket=${encodeURIComponent(info.ticket)}`)
      wsRef.current = ws
      ws.onopen = () => {
        setConnected(true)
        addLog('state', 'WebSocket 已连接')
      }
      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(String(event.data)) as SignalMessage
          addLog('in', String(event.data))
          void handleSignal(msg, iceServers)
        } catch (err) {
          addLog('error', `解析信令失败: ${String(err)}`)
        }
      }
      ws.onerror = () => addLog('error', 'WebSocket 错误')
      ws.onclose = (event) => {
        addLog('state', `WebSocket 关闭 code=${event.code}`)
        setConnected(false)
      }
    } catch (err) {
      message.error(err instanceof Error ? err.message : '连接失败')
      addLog('error', String(err))
    } finally {
      setConnecting(false)
    }
  }, [addLog, createPeer, deviceID, disconnect, handleSignal])

  const renegotiate = useCallback(() => {
    if (!send({ type: 'joinRoom', reason: 'renegotiate' })) {
      message.warning('连接尚未建立')
      return
    }
    addLog('state', '已请求设备重新协商')
    // 设备重新 offer 时可能复用同一个 PeerConnection；清空候选等待新 offer。
    pendingCandidates.current = []
    remoteReady.current = false
  }, [addLog, send])

  useEffect(() => {
    return () => {
      // 卸载时静默关闭，避免 React StrictMode 双调用产生额外 leaveRoom。
      const ws = wsRef.current
      wsRef.current = null
      if (ws) {
        try {
          ws.close()
        } catch {
          // ignore
        }
      }
      if (pcRef.current) {
        pcRef.current.close()
        pcRef.current = null
      }
      if (statsTimer.current !== null) window.clearInterval(statsTimer.current)
    }
  }, [])

  useEffect(() => {
    if (videoRef.current && videoRef.current.srcObject) {
      void videoRef.current.play().catch(() => undefined)
    }
  }, [connected])

  const statusText = useMemo(() => {
    if (connecting) return '连接中'
    if (!connected) return '未连接'
    if (pcState === 'connected') return '已连接'
    if (deviceOnline) return '等待媒体'
    return '等待设备'
  }, [connecting, connected, deviceOnline, pcState])

  return (
    <div className="e2e-av-preview">
      <Card
        title={
          <Space>
            <VideoCameraOutlined />
            <span>实时预览</span>
            <Badge
              status={pcState === 'connected' ? 'success' : connected ? 'processing' : 'default'}
              text={statusText}
            />
          </Space>
        }
        extra={
          <Space wrap>
            <Select
              className="e2e-av-device-select"
              style={{ minWidth: 260 }}
              placeholder="选择 WebRTC 设备"
              loading={loadingDevices}
              value={deviceID}
              onChange={setDeviceID}
              showSearch
              optionFilterProp="label"
              options={devices.map((d) => ({
                value: d.id,
                label: `${d.name ?? '未命名设备'}（${d.device_code ?? d.id}）`,
              }))}
              disabled={connected || connecting}
            />
            <Button
              className="e2e-av-connect-btn"
              type="primary"
              icon={<LinkOutlined />}
              loading={connecting}
              disabled={!canConnect || connected || !deviceID}
              onClick={() => void connect()}
            >
              连接
            </Button>
            <Button
              className="e2e-av-renegotiate-btn"
              icon={<ReloadOutlined />}
              disabled={!connected}
              onClick={renegotiate}
            >
              重新协商
            </Button>
            <Button
              className="e2e-av-disconnect-btn"
              danger
              icon={<DisconnectOutlined />}
              disabled={!connected}
              onClick={() => disconnect(true)}
            >
              断开
            </Button>
          </Space>
        }
      >
        {!canConnect && (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
            message="当前账号没有 av:preview:connect 权限，只能查看页面不能发起连接"
          />
        )}
        <Row gutter={16}>
          <Col xs={24} lg={16}>
            <div
              style={{
                background: '#000',
                borderRadius: 8,
                minHeight: 360,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                overflow: 'hidden',
              }}
            >
              <video
                ref={videoRef}
                className="e2e-av-video"
                autoPlay
                playsInline
                muted
                style={{ width: '100%', maxHeight: 560, objectFit: 'contain' }}
              />
              {!connected && (
                <div style={{ position: 'absolute', color: '#888' }}>
                  <Empty description="选择设备后点击「连接」" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                </div>
              )}
            </div>
            <Space size="large" style={{ marginTop: 16 }} wrap>
              <Statistic title="分辨率" value={resolution} />
              <Statistic title="帧率" value={fps} suffix="fps" />
              <Statistic title="码率" value={bitrate} suffix="kbps" />
              <Statistic
                title="ICE"
                value={iceState}
                valueStyle={{ fontSize: 18 }}
              />
              <Statistic
                title="Connection"
                value={pcState}
                valueStyle={{ fontSize: 18 }}
              />
            </Space>
          </Col>
          <Col xs={24} lg={8}>
            <Descriptions
              className="e2e-av-status"
              size="small"
              column={1}
              bordered
              items={[
                { key: 'room', label: '房间', children: roomID || '-' },
                { key: 'client', label: '查看方', children: clientID || '-' },
                {
                  key: 'device',
                  label: '设备',
                  children: (
                    <Tag color={deviceOnline ? 'green' : 'default'}>
                      {deviceOnline ? '在线' : '未进入房间'}
                    </Tag>
                  ),
                },
              ]}
            />
            <Title level={5} style={{ marginTop: 16 }}>
              信令日志
            </Title>
            <div
              className="e2e-av-log"
              data-testid="e2e-av-log"
              style={{
                height: 300,
                overflow: 'auto',
                background: '#fafafa',
                border: '1px solid #f0f0f0',
                borderRadius: 6,
                padding: 8,
                fontFamily: 'monospace',
                fontSize: 12,
              }}
            >
              {logs.length === 0 && <Text type="secondary">暂无日志</Text>}
              {logs.map((line) => (
                <div
                  key={line.id}
                  style={{
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-all',
                    color:
                      line.direction === 'error'
                        ? '#cf1322'
                        : line.direction === 'out'
                          ? '#0958d9'
                          : '#555',
                  }}
                >
                  {line.at} [{line.direction}] {line.text}
                </div>
              ))}
            </div>
          </Col>
        </Row>
      </Card>
    </div>
  )
}
