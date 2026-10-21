import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Empty,
  Image,
  Modal,
  Select,
  Space,
  Spin,
  Tag,
  Tree,
  Typography,
  message,
} from 'antd'
import type { TreeDataNode } from 'antd'
import {
  AudioOutlined,
  CameraOutlined,
  PhoneOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  StopOutlined,
} from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import LivePlayer, { type LiveProtocol } from '../../components/LivePlayer'
import PtzControl, { type PtzDirection, type PtzPresetAction, type PtzRunAction, type PtzZoomAction } from '../../components/PtzControl'
import { GbDeviceController } from '../../api/generated/GbDeviceController'
import { GbChannelController } from '../../api/generated/GbChannelController'
import { GbPlayController } from '../../api/generated/GbPlayController'
import { GbPtzController } from '../../api/generated/GbPtzController'
import { GbVoiceController } from '../../api/generated/GbVoiceController'
import type { GbChannelItem } from '../../api/generated/definitions/GbChannelItem'
import type { GbDeviceItem } from '../../api/generated/definitions/GbDeviceItem'
import type { GbPlayResp } from '../../api/generated/definitions/GbPlayResp'
import type { GbSnapshotResp } from '../../api/generated/definitions/GbSnapshotResp'
import { useAuthStore } from '../../store/authStore'

/** 从 Axios 错误中提取后端中文提示（不改变响应数据结构，也不使用 as any） */
function extractErrorMessage(error: unknown, fallback: string): string {
  if (typeof error === 'object' && error !== null && 'response' in error) {
    const response = (error as { response?: { data?: { message?: unknown } } }).response
    const text = response?.data?.message
    if (typeof text === 'string' && text.trim()) return text
  }
  if (error instanceof Error && error.message) return error.message
  return fallback
}

/** 通道状态字典：国标设备多用 ON/OFF，兼容 online/offline */
function channelStatusMeta(status?: string): { text: string; color: string } {
  const key = (status ?? '').trim().toUpperCase()
  if (key === 'ON' || key === 'ONLINE' || key === 'ACTIVE') return { text: '在线', color: 'green' }
  if (key === 'OFF' || key === 'OFFLINE') return { text: '离线', color: 'red' }
  return { text: status || '未知', color: 'default' }
}

interface VoiceSessionRef {
  channelId: number
  sessionId: string
}

/**
 * 实时播放：左侧按设备分组的通道树，右侧 LivePlayer + PtzControl。
 *
 * 播放会话生命周期：
 *  - 切通道 / 离开页面时必须用返回的 stream_id 调 stopPlay，避免 ZLM 残留流；
 *  - 语音广播 / 对讲同样在切通道与卸载时兜底停止。
 */
const LivePlay: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canPlay = permCodes.includes('gb:live:play')
  const canPtz = permCodes.includes('gb:live:ptz')
  const canSnap = permCodes.includes('gb:live:snap')
  const canBroadcast = permCodes.includes('gb:live:broadcast')
  const canTalk = permCodes.includes('gb:live:talk')

  const [devices, setDevices] = useState<GbDeviceItem[]>([])
  const [channels, setChannels] = useState<GbChannelItem[]>([])
  const [treeLoading, setTreeLoading] = useState(false)
  const [selectedChannel, setSelectedChannel] = useState<GbChannelItem | null>(null)
  const [playInfo, setPlayInfo] = useState<GbPlayResp | null>(null)
  const [protocol, setProtocol] = useState<LiveProtocol>('webrtc')
  const [starting, setStarting] = useState(false)
  const [playError, setPlayError] = useState('')
  const [snapshot, setSnapshot] = useState<GbSnapshotResp | null>(null)
  const [broadcasting, setBroadcasting] = useState(false)
  const [talking, setTalking] = useState(false)

  /** 当前播放会话的通道主键与 SIP Call-ID（停止播放必须回传 stream_id） */
  const playChannelRef = useRef<number | null>(null)
  const streamIDRef = useRef('')
  /** 请求序号：切通道 / 停止时递增，用来丢弃迟到的 play 响应并停掉其会话 */
  const playRequestRef = useRef(0)
  /** 语音会话（广播 / 对讲） */
  const broadcastRef = useRef<VoiceSessionRef | null>(null)
  const talkRef = useRef<VoiceSessionRef | null>(null)

  const loadTree = useCallback(async () => {
    setTreeLoading(true)
    try {
      const [deviceRes, channelRes] = await Promise.all([
        GbDeviceController.pageGbDevices(1, 500),
        GbChannelController.pageGbChannels(1, 1000),
      ])
      setDevices(deviceRes.data?.items ?? [])
      setChannels(channelRes.data?.items ?? [])
    } catch {
      message.error('获取国标设备与通道失败')
    } finally {
      setTreeLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadTree()
  }, [loadTree])

  /** 停止实时播放：仅当拿到了后端返回的 stream_id 时才发 stop 请求 */
  const stopPlaySession = useCallback(async () => {
    const channelId = playChannelRef.current
    const streamId = streamIDRef.current
    playChannelRef.current = null
    streamIDRef.current = ''
    setPlayInfo(null)
    if (channelId && streamId) {
      try {
        await GbPlayController.stopGbPlay(channelId, { stream_id: streamId })
      } catch {
        // 会话已不存在时后端幂等返回；网络错误由拦截器提示，这里不重复弹窗
      }
    }
  }, [])

  /** 停止语音广播 / 对讲（切通道与卸载时兜底） */
  const stopVoiceSessions = useCallback(async () => {
    const broadcast = broadcastRef.current
    const talk = talkRef.current
    broadcastRef.current = null
    talkRef.current = null
    setBroadcasting(false)
    setTalking(false)
    if (broadcast) {
      try {
        await GbVoiceController.stopGbBroadcast(broadcast.channelId, { session_id: broadcast.sessionId })
      } catch {
        /* 尽力而为 */
      }
    }
    if (talk) {
      try {
        await GbVoiceController.stopGbTalk(talk.channelId, { session_id: talk.sessionId })
      } catch {
        /* 尽力而为 */
      }
    }
  }, [])

  // 离开页面：必须清理播放与语音会话，避免 ZLM / SIP 侧残留
  useEffect(
    () => () => {
      // 卸载时让在途 play 响应失效，避免页面已离开才建流
      playRequestRef.current += 1
      void stopPlaySession()
      void stopVoiceSessions()
    },
    [stopPlaySession, stopVoiceSessions],
  )

  /** 用户主动停止：先让在途 play 请求失效，再按 stream_id 停流 */
  const handleStop = () => {
    playRequestRef.current += 1
    void stopPlaySession()
  }

  const handlePlay = useCallback(
    async (channel: GbChannelItem) => {
      if (!channel.id) return
      if (!canPlay) {
        message.warning('当前账号没有实时播放权限')
        return
      }
      if (playChannelRef.current === channel.id && streamIDRef.current) {
        return
      }

      setSelectedChannel(channel)
      setPlayError('')
      setSnapshot(null)
      setStarting(true)

      // 每次播放分配独立序号；切通道 / 停止会让旧序号失效
      const requestId = playRequestRef.current + 1
      playRequestRef.current = requestId

      // 先停掉上一路（切通道必须停流）
      await stopPlaySession()
      await stopVoiceSessions()

      try {
        const res = await GbPlayController.playGbChannel(channel.id, { protocol })
        const data = res.data
        // 请求期间用户又切了通道 / 点了停止：本次响应已过期，立即停掉迟到建立的流
        if (playRequestRef.current !== requestId) {
          if (data?.stream_id) {
            void GbPlayController.stopGbPlay(channel.id, { stream_id: data.stream_id }).catch(() => {
              /* 尽力清理 */
            })
          }
          return
        }
        if (!data) {
          setPlayError('后端未返回播放地址')
          return
        }
        playChannelRef.current = channel.id
        streamIDRef.current = data.stream_id ?? ''
        setPlayInfo(data)
      } catch (error) {
        setPlayError(extractErrorMessage(error, '发起实时播放失败'))
      } finally {
        setStarting(false)
      }
    },
    [canPlay, protocol, stopPlaySession, stopVoiceSessions],
  )

  const handleTreeSelect = (keys: React.Key[]) => {
    const key = String(keys[0] ?? '')
    if (!key.startsWith('channel-')) return
    const channelId = Number(key.slice('channel-'.length))
    const target = channels.find((item) => item.id === channelId)
    if (target) void handlePlay(target)
  }

  const handleSnapshot = async () => {
    const channelId = selectedChannel?.id
    if (!channelId) return
    if (!canSnap) {
      message.warning('当前账号没有抓图权限')
      return
    }
    try {
      const res = await GbPlayController.snapshotGbChannel(channelId)
      if (res.data?.url) {
        setSnapshot(res.data)
        message.success('抓图成功')
      } else {
        message.warning('抓图失败：后端未返回图片地址')
      }
    } catch {
      // 拦截器已展示后端中文错误
    }
  }

  const handleBroadcast = async () => {
    const channelId = selectedChannel?.id
    if (!channelId) return
    if (!canBroadcast) {
      message.warning('当前账号没有语音广播权限')
      return
    }
    if (broadcasting) {
      const session = broadcastRef.current
      broadcastRef.current = null
      setBroadcasting(false)
      try {
        await GbVoiceController.stopGbBroadcast(channelId, { session_id: session?.sessionId })
      } catch {
        /* 尽力而为 */
      }
      return
    }
    try {
      const res = await GbVoiceController.startGbBroadcast(channelId)
      broadcastRef.current = { channelId, sessionId: res.data?.session_id ?? '' }
      setBroadcasting(true)
    } catch {
      // 拦截器已展示后端中文错误
    }
    message.info('语音回推为尽力而为，需音频源支持')
  }

  const handleTalk = async () => {
    const channelId = selectedChannel?.id
    if (!channelId) return
    if (!canTalk) {
      message.warning('当前账号没有语音对讲权限')
      return
    }
    if (talking) {
      const session = talkRef.current
      talkRef.current = null
      setTalking(false)
      try {
        await GbVoiceController.stopGbTalk(channelId, { session_id: session?.sessionId })
      } catch {
        /* 尽力而为 */
      }
      return
    }
    try {
      const res = await GbVoiceController.startGbTalk(channelId, { agent_enabled: false })
      talkRef.current = { channelId, sessionId: res.data?.session_id ?? '' }
      setTalking(true)
    } catch {
      // 拦截器已展示后端中文错误
    }
    message.info('语音回推为尽力而为，需音频源支持')
  }

  const ptzChannelId = selectedChannel?.id

  const handlePtzCommand = (direction: PtzDirection, speed: number) => {
    if (!ptzChannelId) return
    void GbPtzController.controlGbPtz(ptzChannelId, { direction, speed }).catch(() => {
      /* 拦截器已提示 */
    })
  }

  const handlePtzZoom = (action: PtzZoomAction, speed: number) => {
    if (!ptzChannelId) return
    void GbPtzController.controlGbPtz(ptzChannelId, { action, speed }).catch(() => {
      /* 拦截器已提示 */
    })
  }

  const handlePtzPreset = (action: PtzPresetAction, presetIndex: number) => {
    if (!ptzChannelId) return
    if (action === 'goto') {
      // 本期后端只提供预置位设置 / 删除，没有「调用预置位」端点，不能复用云台方向接口
      message.info('本期接口仅支持预置位设置与删除，暂不支持调用预置位')
      return
    }
    if (action === 'set') {
      void GbPtzController.addGbPtzPreset(ptzChannelId, {
        action: 'set',
        preset_id: presetIndex,
        name: `预置位${presetIndex}`,
      }).catch(() => {
        /* 拦截器已提示 */
      })
      return
    }
    void GbPtzController.deleteGbPtzPreset(ptzChannelId, presetIndex).catch(() => {
      /* 拦截器已提示 */
    })
  }

  const handlePtzRun = (kind: 'cruise' | 'scan', action: PtzRunAction) => {
    if (!ptzChannelId) return
    if (kind === 'cruise') {
      void GbPtzController.cruiseGbPtz(ptzChannelId, { action, group_id: 1 }).catch(() => {
        /* 拦截器已提示 */
      })
      return
    }
    void GbPtzController.scanGbPtz(ptzChannelId, { action, group_id: 1 }).catch(() => {
      /* 拦截器已提示 */
    })
  }

  const treeData = useMemo<TreeDataNode[]>(() => {
    const groups = devices.map((device) => ({
      key: `device-${device.id ?? device.gb_device_id ?? device.device_code ?? device.name}`,
      selectable: false,
      title: (
        <Space size={4}>
          <span>{device.name ?? device.device_code ?? device.gb_device_id ?? '未命名设备'}</span>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {device.channel_count ?? 0} 路
          </Typography.Text>
        </Space>
      ),
      children: channels
        .filter((channel) => channel.device_id === device.id)
        .map((channel) => ({
          key: `channel-${channel.id}`,
          title: (
            <Space size={4}>
              <span>{channel.name ?? channel.gb_channel_id ?? `通道 ${channel.id}`}</span>
              <Tag color={channelStatusMeta(channel.status).color} style={{ marginInlineEnd: 0 }}>
                {channelStatusMeta(channel.status).text}
              </Tag>
            </Space>
          ),
          isLeaf: true,
        })),
    }))

    // 兜底：device_id 缺失的通道单独成组，避免在树里「消失」
    const orphan = channels.filter(
      (channel) => !devices.some((device) => device.id !== undefined && device.id === channel.device_id),
    )
    if (orphan.length > 0) {
      groups.push({
        key: 'device-orphan',
        selectable: false,
        title: <span>未关联设备</span>,
        children: orphan.map((channel) => ({
          key: `channel-${channel.id}`,
          title: (
            <Space size={4}>
              <span>{channel.name ?? channel.gb_channel_id ?? `通道 ${channel.id}`}</span>
              <Tag color={channelStatusMeta(channel.status).color} style={{ marginInlineEnd: 0 }}>
                {channelStatusMeta(channel.status).text}
              </Tag>
            </Space>
          ),
          isLeaf: true,
        })),
      })
    }
    return groups
  }, [devices, channels])

  return (
    <PageContainer title="实时播放" subTitle="国标设备实时视频、云台控制与语音对讲">
      <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
        <Card
          size="small"
          title="设备 / 通道"
          style={{ width: 300, flex: '0 0 300px' }}
          extra={
            <Button
              size="small"
              type="text"
              icon={<ReloadOutlined />}
              loading={treeLoading}
              onClick={() => void loadTree()}
            >
              刷新
            </Button>
          }
        >
          {treeLoading && channels.length === 0 ? (
            <div style={{ padding: 32, textAlign: 'center' }}>
              <Spin size="small" />
            </div>
          ) : treeData.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无国标设备" />
          ) : (
            <Tree
              className="e2e-gb-channel-tree"
              blockNode
              showLine
              treeData={treeData}
              selectedKeys={selectedChannel?.id ? [`channel-${selectedChannel.id}`] : []}
              onSelect={handleTreeSelect}
            />
          )}
        </Card>

        <div style={{ flex: 1, minWidth: 0 }}>
          <Card
            size="small"
            title={
              selectedChannel ? (
                <Space size={8}>
                  <span>{selectedChannel.name ?? selectedChannel.gb_channel_id ?? '通道'}</span>
                  <Tag color={channelStatusMeta(selectedChannel.status).color}>
                    {channelStatusMeta(selectedChannel.status).text}
                  </Tag>
                </Space>
              ) : (
                '播放窗口'
              )
            }
            extra={
              <Space wrap>
                <Select<LiveProtocol>
                  size="small"
                  value={protocol}
                  style={{ width: 150 }}
                  options={[
                    { label: 'WebRTC（低延迟）', value: 'webrtc' },
                    { label: 'HTTP-FLV（兼容）', value: 'flv' },
                  ]}
                  onChange={(value) => {
                    setProtocol(value)
                    setPlayError('')
                  }}
                />
                <Button
                  size="small"
                  icon={<PlayCircleOutlined />}
                  disabled={!selectedChannel || !canPlay}
                  loading={starting}
                  onClick={() => selectedChannel && void handlePlay(selectedChannel)}
                >
                  播放
                </Button>
                <Button
                  size="small"
                  icon={<StopOutlined />}
                  disabled={!playInfo && !starting}
                  onClick={handleStop}
                >
                  停止
                </Button>
                <Button
                  size="small"
                  icon={<CameraOutlined />}
                  disabled={!selectedChannel || !canSnap}
                  onClick={() => void handleSnapshot()}
                >
                  抓图
                </Button>
                <Button
                  size="small"
                  icon={<AudioOutlined />}
                  type={broadcasting ? 'primary' : 'default'}
                  disabled={!selectedChannel || !canBroadcast}
                  onClick={() => void handleBroadcast()}
                >
                  {broadcasting ? '停止广播' : '语音广播'}
                </Button>
                <Button
                  size="small"
                  icon={<PhoneOutlined />}
                  type={talking ? 'primary' : 'default'}
                  disabled={!selectedChannel || !canTalk}
                  onClick={() => void handleTalk()}
                >
                  {talking ? '停止对讲' : '语音对讲'}
                </Button>
              </Space>
            }
          >
            {playError ? (
              <Alert
                type="error"
                showIcon
                style={{ marginBottom: 8 }}
                message="播放失败"
                description={
                  <span>
                    {playError}
                    {protocol === 'webrtc' ? '（WebRTC 不通时可切换为 HTTP-FLV 重试）' : ''}
                  </span>
                }
              />
            ) : null}

            <div
              style={{
                height: 480,
                background: '#000',
                borderRadius: 6,
                overflow: 'hidden',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              {playInfo ? (
                <LivePlayer
                  app={playInfo.app}
                  stream={playInfo.stream}
                  httpBase={playInfo.http_base}
                  protocol={protocol}
                  objectFit="contain"
                  style={{ width: '100%', height: '100%' }}
                  onError={(error) => setPlayError(error.message)}
                  onProtocolChange={(used) => {
                    setPlayError('')
                    if (used === 'flv') {
                      message.info('WebRTC 连接失败，已自动切换为 HTTP-FLV')
                    }
                  }}
                />
              ) : (
                <div style={{ color: '#8c8c8c', textAlign: 'center', padding: 24 }}>
                  {starting ? (
                    <Spin />
                  ) : (
                    <>
                      <div>请从左侧选择通道</div>
                      <div style={{ marginTop: 6, fontSize: 12 }}>
                        播放返回的 stream_id 会在切换通道或离开页面时用于停止会话
                      </div>
                    </>
                  )}
                </div>
              )}
            </div>
          </Card>

          <Card
            size="small"
            title="云台控制"
            style={{ marginTop: 12 }}
            extra={
              canPtz ? null : <Typography.Text type="secondary">无云台操作权限</Typography.Text>
            }
          >
            {canPtz ? (
              <PtzControl
                disabled={!selectedChannel}
                onCommand={handlePtzCommand}
                onZoom={handlePtzZoom}
                onPreset={handlePtzPreset}
                onCruise={(action) => handlePtzRun('cruise', action)}
                onScan={(action) => handlePtzRun('scan', action)}
              />
            ) : (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前账号未授予云台控制权限" />
            )}
          </Card>
        </div>
      </div>

      <Modal
        className="e2e-gb-snapshot-modal"
        title="通道抓图"
        open={Boolean(snapshot)}
        footer={null}
        width={760}
        onCancel={() => setSnapshot(null)}
        destroyOnHidden
      >
        {snapshot?.url ? (
          <div>
            <Image src={snapshot.url} alt="通道抓图" style={{ width: '100%' }} />
            <div style={{ marginTop: 8, color: '#8c8c8c', fontSize: 12 }}>
              抓图时间：{snapshot.capture_dt || '-'}
              {snapshot.file_name ? ` · 文件：${snapshot.file_name}` : ''}
            </div>
          </div>
        ) : (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无抓图" />
        )}
      </Modal>
    </PageContainer>
  )
}

export default LivePlay
