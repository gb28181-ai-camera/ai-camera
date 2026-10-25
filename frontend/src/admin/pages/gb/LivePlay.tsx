import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Empty,
  Image,
  InputNumber,
  List,
  Modal,
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
import LivePlayerTabs from '../../components/LivePlayerTabs'
import GbPtzAdvancedModal from '../../components/GbPtzAdvancedModal'
import PtzControl, { type PtzDirection, type PtzPresetAction, type PtzRunAction, type PtzZoomAction } from '../../components/PtzControl'
import { GbDeviceController } from '../../api/generated/GbDeviceController'
import { GbChannelController } from '../../api/generated/GbChannelController'
import { GbPlayController } from '../../api/generated/GbPlayController'
import { GbPtzController } from '../../api/generated/GbPtzController'
import { GbVoiceController } from '../../api/generated/GbVoiceController'
import type { GbChannelItem } from '../../api/generated/definitions/GbChannelItem'
import type { GbCruiseTrackDetailResp } from '../../api/generated/definitions/GbCruiseTrackDetailResp'
import type { GbCruiseTrackItem } from '../../api/generated/definitions/GbCruiseTrackItem'
import type { GbDeviceItem } from '../../api/generated/definitions/GbDeviceItem'
import type { GbHomePositionResp } from '../../api/generated/definitions/GbHomePositionResp'
import type { GbPlayResp } from '../../api/generated/definitions/GbPlayResp'
import type { GbPtzPositionResp } from '../../api/generated/definitions/GbPtzPositionResp'
import type { GbPtzPreciseReq } from '../../api/generated/definitions/GbPtzPreciseReq'
import type { GbPresetItem } from '../../api/generated/definitions/GbPresetItem'
import type { GbSDCardStatusResp } from '../../api/generated/definitions/GbSDCardStatusResp'
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

/** 2022 应答里的可选数值：设备未上报时显示「—」，不能显示成 0。 */
function formatOptionalNumber(value?: number): string {
  return typeof value === 'number' ? String(value) : '—'
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
  /** 雨刷（PTZCmd 0x8C/0x8D）；权限码由种子数据补入 ac_menu */
  const canWiper = permCodes.includes('gb:ptz:wiper')
  // 云台高级操作（巡航点 / 扫描边界 / 预置位重命名）
  const canPresetUpdate = permCodes.includes('gb:ptz:preset-update')
  const canCruisePointCreate = permCodes.includes('gb:ptz:cruise-point:create')
  const canCruisePointUpdate = permCodes.includes('gb:ptz:cruise-point:update')
  const canCruisePointDelete = permCodes.includes('gb:ptz:cruise-point:delete')
  const canScanBounds = permCodes.includes('gb:ptz:scan-bounds')
  const canPtzAdvanced =
    canCruisePointCreate || canCruisePointUpdate || canCruisePointDelete || canScanBounds
  // GB/T 28181-2022 扩展按钮（权限码已写入 ac_menu，仅 2022 设备可用）
  const canPosition = permCodes.includes('gb:live:ptz-position')
  const canPrecise = permCodes.includes('gb:live:ptz-precise')
  const canCruiseTrack = permCodes.includes('gb:live:cruise-track')
  const canHomePosition = permCodes.includes('gb:live:home-position')
  const canSDCard = permCodes.includes('gb:live:sdcard')
  const canAny2022 =
    canPosition || canPrecise || canCruiseTrack || canHomePosition || canSDCard

  const [devices, setDevices] = useState<GbDeviceItem[]>([])
  const [channels, setChannels] = useState<GbChannelItem[]>([])
  const [treeLoading, setTreeLoading] = useState(false)
  const [selectedChannel, setSelectedChannel] = useState<GbChannelItem | null>(null)
  const [playInfo, setPlayInfo] = useState<GbPlayResp | null>(null)
  const [starting, setStarting] = useState(false)
  const [playError, setPlayError] = useState('')
  const [snapshot, setSnapshot] = useState<GbSnapshotResp | null>(null)
  const [broadcasting, setBroadcasting] = useState(false)
  const [talking, setTalking] = useState(false)
  /** 平台侧预置位（含名称），随通道切换重新拉取 */
  const [presets, setPresets] = useState<GbPresetItem[]>([])
  const [presetsLoading, setPresetsLoading] = useState(false)
  /** 云台高级操作（巡航点 / 扫描边界）弹窗 */
  const [advancedOpen, setAdvancedOpen] = useState(false)

  // ── GB/T 28181-2022 扩展查询状态 ──
  const [ptzPosition, setPtzPosition] = useState<GbPtzPositionResp | null>(null)
  const [positionLoading, setPositionLoading] = useState(false)
  const [preciseOpen, setPreciseOpen] = useState(false)
  const [precisePan, setPrecisePan] = useState<number | null>(null)
  const [preciseTilt, setPreciseTilt] = useState<number | null>(null)
  const [preciseZoom, setPreciseZoom] = useState<number | null>(null)
  const [preciseSubmitting, setPreciseSubmitting] = useState(false)
  const [cruiseTracks, setCruiseTracks] = useState<GbCruiseTrackItem[]>([])
  const [cruiseDetails, setCruiseDetails] = useState<Record<number, GbCruiseTrackDetailResp>>({})
  const [cruiseQueried, setCruiseQueried] = useState(false)
  const [cruiseLoading, setCruiseLoading] = useState(false)
  const [cruiseDetailLoading, setCruiseDetailLoading] = useState<number | null>(null)
  const [homePosition, setHomePosition] = useState<GbHomePositionResp | null>(null)
  const [homeLoading, setHomeLoading] = useState(false)
  const [sdCard, setSdCard] = useState<GbSDCardStatusResp | null>(null)
  const [sdCardLoading, setSdCardLoading] = useState(false)
  /** 2016 设备提示：必须常驻 Alert，不能只弹 toast（一闪而过会被当成按钮失效）。 */
  const [compat2022, setCompat2022] = useState('')

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

  /**
   * 拉取平台侧预置位列表（含名称）。
   *
   * 名称来自 `ac_gb_ptz_preset`，设备侧只认编号；列表失败不影响云台其他操作，
   * 因此这里静默降级为空列表（按钮仍可按 1..8 编号操作）。
   */
  const loadPresets = useCallback(async (channelId: number) => {
    setPresetsLoading(true)
    try {
      const res = await GbPtzController.listGbPtzPresets(channelId)
      setPresets(res.data ?? [])
    } catch {
      setPresets([])
    } finally {
      setPresetsLoading(false)
    }
  }, [])

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

  // 切换通道时清空 2022 查询结果：不同设备的轨迹/看守位/存储卡没有可比性。
  useEffect(() => {
    setPtzPosition(null)
    setCruiseTracks([])
    setCruiseDetails({})
    setCruiseQueried(false)
    setHomePosition(null)
    setSdCard(null)
    setCompat2022('')
    setPreciseOpen(false)
    setAdvancedOpen(false)
  }, [selectedChannel?.id])

  // 切换通道时重新拉取预置位名称；未选通道或无云台权限则清空
  useEffect(() => {
    const channelId = selectedChannel?.id
    if (!channelId || !canPtz) {
      setPresets([])
      return
    }
    void loadPresets(channelId)
  }, [selectedChannel?.id, canPtz, loadPresets])

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
        // 后端始终返回 webrtc / flv / hls 三个地址；协议选择交给前端播放器页签，
        // 切换引擎不再重新 INVITE（re-INVITE 会重建 SIP 会话并造成黑屏）。
        const res = await GbPlayController.playGbChannel(channel.id, {})
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
    [canPlay, stopPlaySession, stopVoiceSessions],
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

  /**
   * 预置位：调用 / 设置 / 删除。
   *
   * 设置时必须带名称（平台侧持久化到 `ac_gb_ptz_preset`，不下发设备），
   * 不允许再写死「预置位N」；设置/删除成功后刷新列表，让按钮直接显示名称。
   */
  const handlePtzPreset = async (
    action: PtzPresetAction,
    presetIndex: number,
    name?: string,
  ) => {
    if (!ptzChannelId) return
    try {
      if (action === 'goto') {
        // 调用预置位走 PTZCmd 0x82，与设置(0x81)/删除(0x83)是三条独立命令码。
        // 必须有这个分支，否则「预置位」只有存没有用。
        await GbPtzController.gotoGbPtzPreset(ptzChannelId, presetIndex)
        return
      }
      if (action === 'set') {
        const presetName = name?.trim()
        if (!presetName) {
          message.warning('请先填写预置位名称')
          return
        }
        await GbPtzController.addGbPtzPreset(ptzChannelId, {
          action: 'set',
          preset_id: presetIndex,
          name: presetName,
        })
        message.success(`预置位 ${presetIndex} 已设置`)
        await loadPresets(ptzChannelId)
        return
      }
      await GbPtzController.deleteGbPtzPreset(ptzChannelId, presetIndex)
      message.success(`预置位 ${presetIndex} 已删除`)
      await loadPresets(ptzChannelId)
    } catch {
      /* 拦截器已提示 */
    }
  }

  /**
   * 重命名预置位：只更新平台侧 `ac_gb_ptz_preset.name`，不下发设备指令，
   * 因此设备离线也可改名。
   */
  const handleRenamePreset = async (presetIndex: number, name: string) => {
    if (!ptzChannelId) return
    const presetName = name?.trim()
    if (!presetName) {
      message.warning('请先填写预置位新名称')
      return
    }
    try {
      await GbPtzController.renameGbPtzPreset(ptzChannelId, presetIndex, { name: presetName })
      message.success(`预置位 ${presetIndex} 已重命名`)
      await loadPresets(ptzChannelId)
    } catch {
      /* 拦截器已提示 */
    }
  }

  /** 巡航 / 扫描：组号由调用方选择（后端 group_id 1–255） */
  const handlePtzRun = (kind: 'cruise' | 'scan', action: PtzRunAction, groupId: number) => {
    if (!ptzChannelId) return
    if (kind === 'cruise') {
      void GbPtzController.cruiseGbPtz(ptzChannelId, { action, group_id: groupId }).catch(() => {
        /* 拦截器已提示 */
      })
      return
    }
    void GbPtzController.scanGbPtz(ptzChannelId, { action, group_id: groupId }).catch(() => {
      /* 拦截器已提示 */
    })
  }

  /**
   * 雨刷开关。
   *
   * 后端 DTO 是 `GbPtzWiperReq{on: boolean}`（PTZCmd 0x8C 开 / 0x8D 关），
   * 字段名以 `backend/internal/dto/request/gb28181_play.go` 为准。
   */
  const handlePtzWiper = (on: boolean) => {
    if (!ptzChannelId) return
    if (!canWiper) {
      message.warning('当前账号没有雨刷控制权限')
      return
    }
    void GbPtzController.wiperGbPtz(ptzChannelId, { on }).catch(() => {
      /* 拦截器已提示 */
    })
  }

  // ────────────────────────────────────────────────────────────────
  // GB/T 28181-2022 扩展查询
  //
  // 这些请求是同步等待设备应答（后端最长 5 秒），因此按钮要带 loading，
  // 且版本不支持必须用常驻 Alert 说明；设备超时/网络错误交给全局拦截器提示。
  // ────────────────────────────────────────────────────────────────

  const require2022Channel = (): number | null => {
    const id = selectedChannel?.id
    if (!id) {
      message.warning('请先选择通道')
      return null
    }
    return id
  }

  const handle2022Failure = (error: unknown, fallback: string) => {
    const text = extractErrorMessage(error, fallback)
    if (text.includes('GB28181-2016') || text.includes('版本不支持')) {
      setCompat2022(text)
      return
    }
    // 504（设备超时）与网络错误已经由 request.ts 的拦截器统一 toast。
  }

  const handleQueryPosition = async () => {
    const channelId = require2022Channel()
    if (!channelId) return
    setPositionLoading(true)
    try {
      const res = await GbPtzController.gbPtzPosition(channelId)
      setPtzPosition(res.data ?? null)
      setCompat2022('')
    } catch (error) {
      handle2022Failure(error, '查询 PTZ 位置失败')
    } finally {
      setPositionLoading(false)
    }
  }

  const handleOpenPrecise = () => {
    if (!require2022Channel()) return
    setPreciseOpen(true)
  }

  const handlePreciseSubmit = async () => {
    const channelId = require2022Channel()
    if (!channelId) return
    if (precisePan === null && preciseTilt === null && preciseZoom === null) {
      message.warning('pan / tilt / zoom 至少填写一个')
      return
    }
    const body: GbPtzPreciseReq = {}
    if (precisePan !== null) body.pan = precisePan
    if (preciseTilt !== null) body.tilt = preciseTilt
    if (preciseZoom !== null) body.zoom = preciseZoom

    setPreciseSubmitting(true)
    try {
      await GbPtzController.preciseGbPtz(channelId, body)
      setPreciseOpen(false)
      setCompat2022('')
      message.success('精准控制指令已下发')
    } catch (error) {
      handle2022Failure(error, 'PTZ 精准控制失败')
    } finally {
      setPreciseSubmitting(false)
    }
  }

  const handleQueryCruiseTracks = async () => {
    const channelId = require2022Channel()
    if (!channelId) return
    setCruiseLoading(true)
    try {
      const res = await GbPtzController.listGbPtzCruiseTracks(channelId)
      setCruiseTracks(res.data ?? [])
      setCruiseDetails({})
      setCruiseQueried(true)
      setCompat2022('')
    } catch (error) {
      handle2022Failure(error, '查询巡航轨迹失败')
    } finally {
      setCruiseLoading(false)
    }
  }

  const handleQueryCruiseDetail = async (number: number) => {
    const channelId = require2022Channel()
    if (!channelId || cruiseDetails[number]) return
    setCruiseDetailLoading(number)
    try {
      const res = await GbPtzController.getGbPtzCruiseTrack(channelId, number)
      const detail = res.data
      if (detail) {
        setCruiseDetails((prev) => ({ ...prev, [number]: detail }))
      }
    } catch (error) {
      handle2022Failure(error, '查询巡航轨迹明细失败')
    } finally {
      setCruiseDetailLoading(null)
    }
  }

  const handleQueryHomePosition = async () => {
    const channelId = require2022Channel()
    if (!channelId) return
    setHomeLoading(true)
    try {
      const res = await GbPtzController.getGbPtzHomePosition(channelId)
      setHomePosition(res.data ?? null)
      setCompat2022('')
    } catch (error) {
      handle2022Failure(error, '查询看守位失败')
    } finally {
      setHomeLoading(false)
    }
  }

  const handleQuerySDCard = async () => {
    const channelId = require2022Channel()
    if (!channelId) return
    setSdCardLoading(true)
    try {
      const res = await GbPtzController.getGbPtzSDCard(channelId)
      setSdCard(res.data ?? null)
      setCompat2022('')
    } catch (error) {
      handle2022Failure(error, '查询存储卡状态失败')
    } finally {
      setSdCardLoading(false)
    }
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
                  {playInfo?.codec ? <Tag color="blue">{playInfo.codec}</Tag> : null}
                </Space>
              ) : (
                '播放窗口'
              )
            }
            extra={
              <Space wrap>
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
                    {'（可在播放器页签切换引擎，或等待默认引擎自动回落）'}
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
                <LivePlayerTabs
                  app={playInfo.app}
                  stream={playInfo.stream}
                  httpBase={playInfo.http_base}
                  // 后端一次返回三种地址；页签切换只换前端引擎，不重新 INVITE
                  urls={{
                    webrtc: playInfo.webrtc,
                    flv: playInfo.flv,
                    hls: playInfo.hls,
                  }}
                  codec={playInfo.codec}
                  allowSwitch
                  objectFit="contain"
                  style={{ width: '100%', height: '100%' }}
                  onError={(error) => setPlayError(error.message)}
                  onEngineChange={(engine, reason) => {
                    setPlayError('')
                    if (reason === 'fallback') {
                      message.info(`默认播放引擎不可用，已自动切换为 ${engine}`)
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
              canPtz ? (
                canPtzAdvanced ? (
                  <Button
                    size="small"
                    className="e2e-gb-ptz-advanced-btn"
                    disabled={!selectedChannel}
                    onClick={() => setAdvancedOpen(true)}
                  >
                    高级操作
                  </Button>
                ) : null
              ) : (
                <Typography.Text type="secondary">无云台操作权限</Typography.Text>
              )
            }
          >
            {canPtz ? (
              <PtzControl
                disabled={!selectedChannel}
                presets={presets}
                presetsLoading={presetsLoading}
                onRenamePreset={canPresetUpdate ? handleRenamePreset : undefined}
                onCommand={handlePtzCommand}
                onZoom={handlePtzZoom}
                onPreset={handlePtzPreset}
                onCruise={(action, groupId) => handlePtzRun('cruise', action, groupId)}
                onScan={(action, groupId) => handlePtzRun('scan', action, groupId)}
                onWiper={canWiper ? handlePtzWiper : undefined}
              />
            ) : (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前账号未授予云台控制权限" />
            )}
          </Card>

          {/* GB/T 28181-2022 扩展：仅在设备为 2022 时可用；2016 设备后端会明确拒绝。 */}
          <Card
            size="small"
            title="2022 能力"
            style={{ marginTop: 12 }}
            extra={
              <Typography.Text type="secondary">
                GB/T 28181-2022 · 同步查询（最长 5 秒）
              </Typography.Text>
            }
          >
            {canAny2022 ? (
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                {compat2022 ? (
                  <Alert
                    className="e2e-gb-ptz2022-compat"
                    type="warning"
                    showIcon
                    message="该设备为 GB28181-2016，不支持 2022 扩展"
                    description={compat2022}
                  />
                ) : null}

                <Space wrap size={[8, 8]}>
                  {canPosition ? (
                    <Button
                      size="small"
                      className="e2e-gb-ptz-position"
                      loading={positionLoading}
                      disabled={!selectedChannel}
                      onClick={() => void handleQueryPosition()}
                    >
                      实时位置
                    </Button>
                  ) : null}
                  {canPrecise ? (
                    <Button
                      size="small"
                      className="e2e-gb-ptz-precise"
                      disabled={!selectedChannel}
                      onClick={handleOpenPrecise}
                    >
                      精准控制
                    </Button>
                  ) : null}
                  {canCruiseTrack ? (
                    <Button
                      size="small"
                      className="e2e-gb-ptz-cruise"
                      loading={cruiseLoading}
                      disabled={!selectedChannel}
                      onClick={() => void handleQueryCruiseTracks()}
                    >
                      巡航轨迹
                    </Button>
                  ) : null}
                  {canHomePosition ? (
                    <Button
                      size="small"
                      className="e2e-gb-ptz-home"
                      loading={homeLoading}
                      disabled={!selectedChannel}
                      onClick={() => void handleQueryHomePosition()}
                    >
                      看守位
                    </Button>
                  ) : null}
                  {canSDCard ? (
                    <Button
                      size="small"
                      className="e2e-gb-ptz-sdcard"
                      loading={sdCardLoading}
                      disabled={!selectedChannel}
                      onClick={() => void handleQuerySDCard()}
                    >
                      存储卡
                    </Button>
                  ) : null}
                </Space>

                {/* 实时位置 */}
                {ptzPosition ? (
                  <Descriptions size="small" column={3} bordered>
                    <Descriptions.Item label="Pan（水平角度）">
                      {formatOptionalNumber(ptzPosition.pan)}°
                    </Descriptions.Item>
                    <Descriptions.Item label="Tilt（垂直角度）">
                      {formatOptionalNumber(ptzPosition.tilt)}°
                    </Descriptions.Item>
                    <Descriptions.Item label="Zoom（倍率）">
                      {formatOptionalNumber(ptzPosition.zoom)}
                    </Descriptions.Item>
                  </Descriptions>
                ) : null}

                {/* 巡航轨迹：列表 + 按需展开明细 */}
                {cruiseQueried ? (
                  <List
                    size="small"
                    bordered
                    dataSource={cruiseTracks}
                    locale={{ emptyText: '设备未上报巡航轨迹' }}
                    renderItem={(track) => {
                      const number = track.number ?? 0
                      const detail = cruiseDetails[number]
                      return (
                        <List.Item
                          actions={[
                            <Button
                              key="detail"
                              size="small"
                              className="e2e-gb-ptz-cruise-detail"
                              loading={cruiseDetailLoading === number}
                              onClick={() => void handleQueryCruiseDetail(number)}
                            >
                              明细
                            </Button>,
                          ]}
                        >
                          <Space direction="vertical" size={2}>
                            <span>
                              #{number} {track.name || '未命名轨迹'}
                            </span>
                            {detail ? (
                              <Typography.Text type="secondary">
                                预置点：
                                {(detail.points ?? [])
                                  .map(
                                    (point) =>
                                      `#${formatOptionalNumber(point.preset_index)} 停留${formatOptionalNumber(point.stay_time)}s 速度${formatOptionalNumber(point.speed)}`,
                                  )
                                  .join('；') || '无'}
                              </Typography.Text>
                            ) : null}
                          </Space>
                        </List.Item>
                      )
                    }}
                  />
                ) : null}

                {/* 看守位：supported=false 表示设备未上报该块，不是「已关闭」 */}
                {homePosition ? (
                  <Descriptions size="small" column={4} bordered>
                    <Descriptions.Item label="是否支持">
                      {homePosition.supported ? '支持' : '设备未上报'}
                    </Descriptions.Item>
                    <Descriptions.Item label="启用状态">
                      {homePosition.supported
                        ? homePosition.enabled === 1
                          ? '已启用'
                          : '已关闭'
                        : '—'}
                    </Descriptions.Item>
                    <Descriptions.Item label="归位预置位">
                      {homePosition.supported
                        ? formatOptionalNumber(homePosition.preset_index)
                        : '—'}
                    </Descriptions.Item>
                    <Descriptions.Item label="归位间隔">
                      {homePosition.supported
                        ? `${formatOptionalNumber(homePosition.reset_time)} 秒`
                        : '—'}
                    </Descriptions.Item>
                  </Descriptions>
                ) : null}

                {/* 存储卡：容量单位 MB */}
                {sdCard ? (
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <Descriptions size="small" column={3} bordered>
                      <Descriptions.Item label="卡片数量">
                        {formatOptionalNumber(sdCard.num)}
                      </Descriptions.Item>
                      <Descriptions.Item label="整体状态">
                        {formatOptionalNumber(sdCard.status)}
                      </Descriptions.Item>
                      <Descriptions.Item label="格式化进度">
                        {formatOptionalNumber(sdCard.format_progress)}%
                      </Descriptions.Item>
                    </Descriptions>
                    <List
                      size="small"
                      bordered
                      dataSource={sdCard.cards ?? []}
                      locale={{ emptyText: '设备未上报存储卡' }}
                      renderItem={(card, index) => (
                        <List.Item>
                          <Space size={16} wrap>
                            <span>卡 {index + 1}</span>
                            <span>容量 {formatOptionalNumber(card.capacity)} MB</span>
                            <span>剩余 {formatOptionalNumber(card.free_space)} MB</span>
                            <span>状态 {formatOptionalNumber(card.status)}</span>
                          </Space>
                        </List.Item>
                      )}
                    />
                  </Space>
                ) : null}
              </Space>
            ) : (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="当前账号未授予 2022 扩展查询权限"
              />
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

      <Modal
        className="e2e-gb-ptz2022-modal"
        title="PTZ 精准控制（GB/T 28181-2022）"
        open={preciseOpen}
        okText="下发"
        confirmLoading={preciseSubmitting}
        onOk={() => void handlePreciseSubmit()}
        onCancel={() => setPreciseOpen(false)}
        destroyOnHidden
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="绝对角度 + 倍率定位；至少填写一项，留空表示不调整该轴。"
          />
          <div>
            <div style={{ marginBottom: 4 }}>Pan（水平角度，度）</div>
            <InputNumber
              className="e2e-gb-ptz2022-pan"
              style={{ width: '100%' }}
              value={precisePan}
              placeholder="例如 10.5"
              onChange={(value) => setPrecisePan(value)}
            />
          </div>
          <div>
            <div style={{ marginBottom: 4 }}>Tilt（垂直角度，度）</div>
            <InputNumber
              className="e2e-gb-ptz2022-tilt"
              style={{ width: '100%' }}
              value={preciseTilt}
              placeholder="例如 20"
              onChange={(value) => setPreciseTilt(value)}
            />
          </div>
          <div>
            <div style={{ marginBottom: 4 }}>Zoom（变焦倍数）</div>
            <InputNumber
              className="e2e-gb-ptz2022-zoom"
              style={{ width: '100%' }}
              min={0}
              value={preciseZoom}
              placeholder="例如 2.5"
              onChange={(value) => setPreciseZoom(value)}
            />
          </div>
        </Space>
      </Modal>

      {/* 云台高级操作：巡航点（0x84–0x87）与扫描边界（0x89/0x8A） */}
      <GbPtzAdvancedModal
        open={advancedOpen}
        channelId={ptzChannelId}
        presets={presets}
        canCreatePoint={canCruisePointCreate}
        canUpdatePoint={canCruisePointUpdate}
        canDeletePoint={canCruisePointDelete}
        canScanBounds={canScanBounds}
        onClose={() => setAdvancedOpen(false)}
      />
    </PageContainer>
  )
}

export default LivePlay
