import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import {
  Button, Descriptions, Divider, Dropdown, Empty, Form, Input, InputNumber, Modal, Popconfirm, Progress, Select,
  Space, Spin, Table, Tag, Typography, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  BellOutlined, DeleteOutlined, EditOutlined, InfoCircleOutlined, LineChartOutlined, MoreOutlined,
  QrcodeOutlined, SyncOutlined,
} from '@ant-design/icons'
import axios from 'axios'
import { QRCodeSVG } from 'qrcode.react'
import ReactECharts from 'echarts-for-react'
import type { EChartsOption } from 'echarts'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import StatusTag from '../../components/StatusTag'
import { GbDeviceController } from '../../api/generated/GbDeviceController'
import { GbChannelController } from '../../api/generated/GbChannelController'
import { GbMediaServerController } from '../../api/generated/GbMediaServerController'
import { RegionController } from '../../api/generated/RegionController'
import type { GbDeviceItem } from '../../api/generated/definitions/GbDeviceItem'
import type { GbDeviceDetailResp } from '../../api/generated/definitions/GbDeviceDetailResp'
import type { GbActivationResp } from '../../api/generated/definitions/GbActivationResp'
import type { GbChannelItem } from '../../api/generated/definitions/GbChannelItem'
import type { GbDeviceSyncStatusResp } from '../../api/generated/definitions/GbDeviceSyncStatusResp'
import type { GbTimeStatisticsResp } from '../../api/generated/definitions/GbTimeStatisticsResp'
import type { UpdateGbDeviceReq } from '../../api/generated/definitions/UpdateGbDeviceReq'
import { useAuthStore } from '../../store/authStore'
import { DEVICE_STATUS_OPTIONS } from '../../types/business'

/** 注册状态字典（后端：pending / activated / rejected） */
const REGISTER_STATUS_OPTIONS = [
  { label: '待激活', value: 'pending' },
  { label: '已激活', value: 'activated' },
  { label: '已拒绝', value: 'rejected' },
]

/** 订阅类型字典（SIP SUBSCRIBE 的 Event 包类型） */
const SUBSCRIBE_TYPE_OPTIONS = [
  { label: '目录订阅（catalog）', value: 'catalog' },
  { label: '告警订阅（alarm）', value: 'alarm' },
  { label: '移动位置订阅（mobile_position）', value: 'mobile_position' },
]

/** 注册状态 → 文案 / 颜色 */
const REGISTER_STATUS_META: Record<string, { text: string; color: string }> = {
  pending: { text: '待激活', color: 'orange' },
  activated: { text: '已激活', color: 'green' },
  rejected: { text: '已拒绝', color: 'red' },
}

/** 目录同步状态 → 文案 / 颜色（后端：idle / syncing / success / failed） */
const SYNC_STATUS_META: Record<string, { text: string; color: string }> = {
  idle: { text: '未开始', color: 'default' },
  syncing: { text: '同步中', color: 'processing' },
  success: { text: '同步成功', color: 'green' },
  failed: { text: '同步失败', color: 'red' },
}

/** 时间统计类型（后端：keepalive / register） */
const TIME_STAT_TYPE_OPTIONS = [
  { label: '心跳间隔（Keepalive）', value: 'keepalive' },
  { label: '注册间隔（REGISTER）', value: 'register' },
]

/** 取值与后端 model 常量一致：UDP / TCP */
const SIP_TRANSPORT_OPTIONS = [
  { label: 'UDP', value: 'UDP' },
  { label: 'TCP', value: 'TCP' },
]

/** 取值与后端 model.GbCharset* 一致 */
const CHARSET_OPTIONS = [
  { label: 'GB2312', value: 'GB2312' },
  { label: 'UTF-8', value: 'UTF-8' },
]

/** 取值与后端 model.GbStreamMode* 一致 */
const STREAM_MODE_OPTIONS = [
  { label: 'UDP', value: 'UDP' },
  { label: 'TCP 被动（TCP-PASSIVE）', value: 'TCP-PASSIVE' },
  { label: 'TCP 主动（TCP-ACTIVE）', value: 'TCP-ACTIVE' },
]

/** 取值与后端 model.GbVersion* 一致 */
const GB_VERSION_OPTIONS = [
  { label: 'GB/T 28181-2016', value: '2016' },
  { label: 'GB/T 28181-2022', value: '2022' },
]

/** 编辑设备表单值（字段与 UpdateGbDeviceReq 对齐，仅前端表单结构） */
interface DeviceFormValues {
  name?: string
  gb_device_id?: string
  sip_realm?: string
  sip_transport?: string
  charset?: string
  media_server_id?: string
  stream_mode?: string
  local_ip?: string
  sdp_ip?: string
  location?: string
  longitude?: string
  latitude?: string
  remark?: string
  gb_version?: string
}

/** 详情 → 表单初始值 */
function buildDeviceFormValues(detail: GbDeviceDetailResp | null): DeviceFormValues {
  if (!detail) return {}
  return {
    name: detail.name ?? '',
    gb_device_id: detail.gb_device_id ?? '',
    sip_realm: detail.sip_realm ?? '',
    sip_transport: detail.sip_transport || undefined,
    charset: detail.charset || undefined,
    media_server_id: detail.media_server_id || 'auto',
    stream_mode: detail.stream_mode || undefined,
    local_ip: detail.local_ip ?? '',
    sdp_ip: detail.sdp_ip ?? '',
    location: detail.location ?? '',
    longitude: detail.longitude ?? '',
    latitude: detail.latitude ?? '',
    remark: detail.remark ?? '',
    gb_version: detail.gb_version || undefined,
  }
}

/** 时间格式化到秒（设备同步/统计接口返回 yyyy-MM-dd HH:mm:ss 或 ISO） */
function fmtDTSeconds(value?: string): string {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

/** 时间格式化：后端返回 ISO 字符串，此处仅展示到分钟 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/**
 * 请求失败兜底提示。
 * 请求拦截器（api/request.ts）已对 HTTP 错误弹出后端 message，
 * 这里只补网络异常 / 非 HTTP 错误的提示，避免同一条错误提示两次。
 */
function notifyError(err: unknown, fallback: string) {
  if (axios.isAxiosError(err) && err.response) return
  message.error(fallback)
}

/** 带复制按钮的只读值 */
function CopyValue({ value }: { value?: string | number | null }) {
  if (value === undefined || value === null || value === '') return <span>-</span>
  return (
    <Typography.Text copyable={{ text: String(value) }} style={{ wordBreak: 'break-all' }}>
      {String(value)}
    </Typography.Text>
  )
}

/**
 * 国标设备：展示已配置国标接入（gb_device_id 非空）的设备，
 * 提供目录同步、订阅、激活凭证生成与删除。
 */
export default function GbDeviceList() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canSync = permCodes.includes('gb:device:sync-channel')
  const canSubscribe = permCodes.includes('gb:device:subscribe')
  const canDelete = permCodes.includes('gb:device:delete')
  const canActivation = permCodes.includes('gb:device:activation')
  const canUpdate = permCodes.includes('gb:device:update')
  const canReboot = permCodes.includes('gb:device:reboot')
  const canGuard = permCodes.includes('gb:device:guard')
  const canAlarmReset = permCodes.includes('gb:device:alarm-reset')
  const canTimeStatistics = permCodes.includes('gb:device:time-statistics')

  const [data, setData] = useState<GbDeviceItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [keyword, setKeyword] = useState('')
  const [registerStatus, setRegisterStatus] = useState<string | undefined>()
  const [status, setStatus] = useState<string | undefined>()
  const [regionID, setRegionID] = useState<number | undefined>()
  const [regionOptions, setRegionOptions] = useState<{ label: string; value: number }[]>([])

  // 详情弹窗
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detail, setDetail] = useState<GbDeviceDetailResp | null>(null)

  // 激活凭证弹窗
  const [activationOpen, setActivationOpen] = useState(false)
  const [activationLoading, setActivationLoading] = useState(false)
  const [activationDevice, setActivationDevice] = useState<GbDeviceItem | null>(null)
  const [activation, setActivation] = useState<GbActivationResp | null>(null)

  // 订阅弹窗
  const [subscribeOpen, setSubscribeOpen] = useState(false)
  const [subscribeDevice, setSubscribeDevice] = useState<GbDeviceItem | null>(null)
  const [subscribeType, setSubscribeType] = useState('catalog')
  const [subscribing, setSubscribing] = useState(false)

  const [syncingId, setSyncingId] = useState<number | null>(null)

  // 编辑设备
  const [form] = Form.useForm<DeviceFormValues>()
  const [editOpen, setEditOpen] = useState(false)
  const [editing, setEditing] = useState<GbDeviceDetailResp | null>(null)
  const [editLoading, setEditLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [mediaServerOptions, setMediaServerOptions] = useState<{ label: string; value: string }[]>([
    { label: '自动分配（auto）', value: 'auto' },
  ])

  // 目录同步进度
  const [syncOpen, setSyncOpen] = useState(false)
  const [syncDevice, setSyncDevice] = useState<GbDeviceItem | null>(null)
  const [syncStatus, setSyncStatus] = useState<GbDeviceSyncStatusResp | null>(null)
  const syncTimerRef = useRef<number | null>(null)
  const syncStoppedRef = useRef(true)
  /** 连续 idle 次数：设备尚未开始上报时保持轮询，超过阈值再停 */
  const syncIdleRef = useRef(0)

  // 报警复位（可指定通道）
  const [alarmOpen, setAlarmOpen] = useState(false)
  const [alarmDevice, setAlarmDevice] = useState<GbDeviceItem | null>(null)
  const [alarmChannels, setAlarmChannels] = useState<GbChannelItem[]>([])
  const [alarmChannelID, setAlarmChannelID] = useState<number | undefined>()
  const [alarmLoading, setAlarmLoading] = useState(false)

  // 时间统计
  const [statsOpen, setStatsOpen] = useState(false)
  const [statsDevice, setStatsDevice] = useState<GbDeviceItem | null>(null)
  const [statsType, setStatsType] = useState('keepalive')
  const [statsCount, setStatsCount] = useState(50)
  const [statsLoading, setStatsLoading] = useState(false)
  const [stats, setStats] = useState<GbTimeStatisticsResp | null>(null)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbDeviceController.pageGbDevices(
        page, pageSize, keyword || undefined, registerStatus, status, regionID
      )
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch (err) {
      notifyError(err, '获取国标设备列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, registerStatus, status, regionID])

  useEffect(() => { void fetchData() }, [fetchData])

  // 区域下拉：筛选栏使用
  useEffect(() => {
    RegionController.options()
      .then((res) => {
        setRegionOptions(
          (res.data ?? [])
            .filter((item) => typeof item.id === 'number')
            .map((item) => ({ label: String(item.name ?? `区域 #${item.id}`), value: Number(item.id) }))
        )
      })
      .catch(() => setRegionOptions([]))
  }, [])

  const handleSearch = (values: Record<string, unknown>) => {
    setKeyword(typeof values.keyword === 'string' ? values.keyword : '')
    setRegisterStatus(typeof values.register_status === 'string' && values.register_status ? values.register_status : undefined)
    setStatus(typeof values.status === 'string' && values.status ? values.status : undefined)
    setRegionID(values.region_id === undefined || values.region_id === null || values.region_id === '' ? undefined : Number(values.region_id))
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setRegisterStatus(undefined)
    setStatus(undefined)
    setRegionID(undefined)
    setPage(1)
  }

  const openDetail = async (row: GbDeviceItem) => {
    if (!row.id) return
    setDetailOpen(true)
    setDetailLoading(true)
    setDetail(null)
    try {
      const res = await GbDeviceController.getGbDevice(row.id)
      if (res.data) {
        setDetail(res.data)
      } else {
        message.error(res.message || '获取设备详情失败')
        setDetailOpen(false)
      }
    } catch (err) {
      notifyError(err, '获取设备详情失败')
      setDetailOpen(false)
    } finally {
      setDetailLoading(false)
    }
  }

  /** 编辑：重新拉取详情后回填表单（列表项不含全部 SIP/媒体字段） */
  const openEdit = async (row: GbDeviceItem) => {
    if (!row.id) return
    setEditing(null)
    setEditOpen(true)
    setEditLoading(true)
    try {
      const res = await GbDeviceController.getGbDevice(row.id)
      if (res.data) {
        setEditing(res.data)
        loadMediaServerOptions(res.data)
      } else {
        message.error(res.message || '获取设备详情失败')
        setEditOpen(false)
      }
    } catch (err) {
      notifyError(err, '获取设备详情失败')
      setEditOpen(false)
    } finally {
      setEditLoading(false)
    }
  }

  // 详情变化后回填表单（Modal 打开时 Form 可能尚未挂载，用 effect 统一处理）
  useEffect(() => {
    if (editOpen && editing) {
      form.setFieldsValue(buildDeviceFormValues(editing))
    }
  }, [editOpen, editing, form])

  /**
   * 媒体节点下拉：auto + 已配置节点（后端按 server_id 校验）。
   * 仅在打开编辑弹窗时拉取，避免无媒体节点权限的账号在进入页面时就收到 403。
   */
  const loadMediaServerOptions = (detail: GbDeviceDetailResp) => {
    const fallback: { label: string; value: string }[] = [{ label: '自动分配（auto）', value: 'auto' }]
    if (detail.media_server_id && detail.media_server_id !== 'auto') {
      fallback.push({
        label: detail.media_server_name || detail.media_server_id,
        value: detail.media_server_id,
      })
    }
    setMediaServerOptions(fallback)
    GbMediaServerController.listGbMediaServers()
      .then((res) => {
        const servers = (res.data ?? [])
          .filter((item) => Boolean(item.server_id))
          .map((item) => ({
            label: item.name ? `${item.name}（${item.server_id}）` : String(item.server_id),
            value: String(item.server_id),
          }))
        const merged = new Map<string, string>()
        merged.set('auto', '自动分配（auto）')
        for (const item of fallback.slice(1)) merged.set(item.value, item.label)
        for (const item of servers) merged.set(item.value, item.label)
        setMediaServerOptions(Array.from(merged.entries()).map(([value, label]) => ({ value, label })))
      })
      .catch(() => {
        /* 无媒体节点权限时保留 auto + 当前值，不阻断编辑 */
      })
  }

  const handleSaveEdit = async () => {
    if (!editing?.id) return
    let values: DeviceFormValues
    try {
      values = await form.validateFields()
    } catch {
      return
    }

    // 后端对 SIP 域/传输/字符集/流模式/版本做严格校验：留空时不下发（保持原值），
    // 其余字符串字段允许清空（local_ip / 经纬度等）。
    const payload: UpdateGbDeviceReq = {
      name: values.name?.trim() ?? '',
      local_ip: values.local_ip?.trim() ?? '',
      sdp_ip: values.sdp_ip?.trim() ?? '',
      location: values.location?.trim() ?? '',
      longitude: values.longitude?.trim() ?? '',
      latitude: values.latitude?.trim() ?? '',
      remark: values.remark?.trim() ?? '',
      media_server_id: values.media_server_id?.trim() || 'auto',
    }
    if (values.gb_device_id?.trim()) payload.gb_device_id = values.gb_device_id.trim()
    if (values.sip_realm?.trim()) payload.sip_realm = values.sip_realm.trim()
    if (values.sip_transport) payload.sip_transport = values.sip_transport
    if (values.charset) payload.charset = values.charset
    if (values.stream_mode) payload.stream_mode = values.stream_mode
    if (values.gb_version) payload.gb_version = values.gb_version

    setSaving(true)
    try {
      const res = await GbDeviceController.updateGbDevice(editing.id, payload)
      if (res.code === 200) {
        message.success('保存成功')
        setEditOpen(false)
        fetchData()
      } else {
        message.error(res.message || '保存失败')
      }
    } catch (err) {
      notifyError(err, '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const openActivation = async (row: GbDeviceItem) => {
    if (!row.id) return
    setActivationDevice(row)
    setActivationOpen(true)
    setActivationLoading(true)
    setActivation(null)
    try {
      const res = await GbDeviceController.generateGbActivation(row.id)
      if (res.data) {
        setActivation(res.data)
      } else {
        message.error(res.message || '生成激活凭证失败')
        setActivationOpen(false)
      }
    } catch (err) {
      notifyError(err, '生成激活凭证失败')
      setActivationOpen(false)
    } finally {
      setActivationLoading(false)
    }
  }

  const openSubscribe = (row: GbDeviceItem) => {
    setSubscribeDevice(row)
    setSubscribeType('catalog')
    setSubscribeOpen(true)
  }

  const handleSubscribe = async (action: 'subscribe' | 'unsubscribe') => {
    if (!subscribeDevice?.id) return
    setSubscribing(true)
    try {
      const res = action === 'subscribe'
        ? await GbDeviceController.subscribeGbDevice(subscribeDevice.id, { type: subscribeType })
        : await GbDeviceController.unsubscribeGbDevice(subscribeDevice.id, subscribeType)
      if (res.code === 200) {
        message.success(action === 'subscribe' ? '订阅指令已下发' : '取消订阅指令已下发')
        setSubscribeOpen(false)
      } else {
        message.error(res.message || '操作失败')
      }
    } catch (err) {
      notifyError(err, '操作失败')
    } finally {
      setSubscribing(false)
    }
  }

  /** 停止同步轮询（幂等） */
  const stopSyncPolling = useCallback(() => {
    syncStoppedRef.current = true
    if (syncTimerRef.current !== null) {
      window.clearInterval(syncTimerRef.current)
      syncTimerRef.current = null
    }
  }, [])

  // 离开页面时停止轮询，避免组件卸载后继续请求
  useEffect(() => () => stopSyncPolling(), [stopSyncPolling])

  /** 拉取一次同步进度；success / failed 时自动停止轮询 */
  const pollSyncStatus = useCallback(async (deviceID: number) => {
    try {
      const res = await GbDeviceController.getGbDeviceSyncStatus(deviceID)
      const data = res.data ?? null
      setSyncStatus(data)
      if (data?.status === 'success' || data?.status === 'failed') {
        stopSyncPolling()
        if (data.status === 'success') message.success('目录同步完成')
        if (data.status === 'failed') message.error(data.error || '目录同步失败')
        return
      }
      // 目录同步是「平台发 MESSAGE → 设备异步回目录」，请求刚下发时后端可能还是 idle，
      // 不能立即停轮询；连续 idle 超过 20 次（约 30 秒）才判定设备未响应。
      if (!data || data.status === 'idle' || !data.status) {
        syncIdleRef.current += 1
        if (syncIdleRef.current >= 20) {
          stopSyncPolling()
          message.warning('未收到目录同步进度，请确认设备在线且 SIP 服务正常')
        }
      } else {
        syncIdleRef.current = 0
      }
    } catch {
      // 轮询失败（设备被删 / SIP 服务不可用）时停止，错误已由拦截器提示
      stopSyncPolling()
    }
  }, [stopSyncPolling])

  const handleSync = async (row: GbDeviceItem) => {
    if (!row.id) return
    setSyncingId(row.id)
    stopSyncPolling()
    syncIdleRef.current = 0
    setSyncDevice(row)
    setSyncStatus(null)
    setSyncOpen(true)
    try {
      const res = await GbDeviceController.syncGbDeviceChannel(row.id)
      if (res.code === 200) {
        // 首轮立即查一次，之后每 1.5 秒轮询；已结束则不再启动定时器
        syncStoppedRef.current = false
        await pollSyncStatus(row.id)
        if (!syncStoppedRef.current) {
          syncTimerRef.current = window.setInterval(() => {
            void pollSyncStatus(row.id as number)
          }, 1500)
        }
      } else {
        message.error(res.message || '同步失败')
        setSyncOpen(false)
      }
    } catch (err) {
      notifyError(err, '同步失败')
      setSyncOpen(false)
    } finally {
      setSyncingId(null)
    }
  }

  const closeSyncModal = () => {
    stopSyncPolling()
    setSyncOpen(false)
  }

  /** 设备操作：重启 / 布防 / 撤防（二次确认） */
  const runConfirmedAction = (title: string, content: string, run: () => Promise<void>) => {
    Modal.confirm({
      className: 'e2e-gb-device-action-confirm-modal',
      title,
      content,
      okText: '确定',
      cancelText: '取消',
      onOk: async () => {
        try {
          await run()
        } catch {
          // 错误已由请求拦截器提示
        }
      },
    })
  }

  const handleReboot = (row: GbDeviceItem) => {
    if (!row.id) return
    runConfirmedAction(
      '重启设备',
      `确定重启设备「${row.name || row.device_code || row.id}」？设备会短暂离线，随后重新注册。`,
      async () => {
        await GbDeviceController.rebootGbDevice(row.id as number)
        message.success('重启指令已下发')
      },
    )
  }

  const handleGuard = (row: GbDeviceItem, guard: boolean) => {
    if (!row.id) return
    runConfirmedAction(
      guard ? '设备布防' : '设备撤防',
      `确定对设备「${row.name || row.device_code || row.id}」执行${guard ? '布防' : '撤防'}？`,
      async () => {
        await GbDeviceController.guardGbDevice(row.id as number, { cmd: guard ? 'SetGuard' : 'ResetGuard' })
        message.success(`${guard ? '布防' : '撤防'}指令已下发`)
      },
    )
  }

  /** 报警复位：打开弹窗选择是否指定通道 */
  const openAlarmReset = async (row: GbDeviceItem) => {
    if (!row.id) return
    setAlarmDevice(row)
    setAlarmChannelID(undefined)
    setAlarmChannels([])
    setAlarmOpen(true)
    setAlarmLoading(true)
    try {
      const res = await GbChannelController.pageGbChannels(1, 1000, undefined, row.id)
      setAlarmChannels(res.data?.items ?? [])
    } catch (err) {
      notifyError(err, '获取设备通道失败')
    } finally {
      setAlarmLoading(false)
    }
  }

  const handleAlarmReset = async () => {
    if (!alarmDevice?.id) return
    setAlarmLoading(true)
    try {
      const res = await GbDeviceController.alarmResetGbDevice(
        alarmDevice.id,
        alarmChannelID ? { channel_id: alarmChannelID } : {},
      )
      if (res.code === 200) {
        message.success('报警复位指令已下发')
        setAlarmOpen(false)
      } else {
        message.error(res.message || '报警复位失败')
      }
    } catch (err) {
      notifyError(err, '报警复位失败')
    } finally {
      setAlarmLoading(false)
    }
  }

  /** 时间统计 */
  const loadTimeStatistics = async (deviceID: number, type: string, count: number) => {
    setStatsLoading(true)
    try {
      const res = await GbDeviceController.getGbDeviceTimeStatistics(deviceID, type, count)
      setStats(res.data ?? null)
    } catch (err) {
      notifyError(err, '获取时间统计失败')
      setStats(null)
    } finally {
      setStatsLoading(false)
    }
  }

  const openTimeStatistics = (row: GbDeviceItem) => {
    if (!row.id) return
    setStatsDevice(row)
    setStatsType('keepalive')
    setStatsCount(50)
    setStats(null)
    setStatsOpen(true)
    void loadTimeStatistics(row.id, 'keepalive', 50)
  }

  /** 设备操作下拉菜单（按权限过滤） */
  const deviceActionItems = useMemo(() => {
    const items: { key: string; label: ReactNode; icon?: ReactNode }[] = []
    if (canReboot) {
      items.push({
        key: 'reboot',
        label: <span className="e2e-gb-device-action-reboot">重启设备</span>,
      })
    }
    if (canGuard) {
      items.push({
        key: 'guard-set',
        label: <span className="e2e-gb-device-action-guard">布防</span>,
      })
      items.push({
        key: 'guard-reset',
        label: <span className="e2e-gb-device-action-unguard">撤防</span>,
      })
    }
    if (canAlarmReset) {
      items.push({
        key: 'alarm-reset',
        label: <span className="e2e-gb-device-action-alarm-reset">报警复位</span>,
      })
    }
    if (canTimeStatistics) {
      items.push({
        key: 'time-statistics',
        label: <span className="e2e-gb-device-action-time-stats">时间统计</span>,
        icon: <LineChartOutlined />,
      })
    }
    return items
  }, [canReboot, canGuard, canAlarmReset, canTimeStatistics])

  const handleDeviceAction = (row: GbDeviceItem, key: string) => {
    switch (key) {
      case 'reboot':
        handleReboot(row)
        break
      case 'guard-set':
        handleGuard(row, true)
        break
      case 'guard-reset':
        handleGuard(row, false)
        break
      case 'alarm-reset':
        void openAlarmReset(row)
        break
      case 'time-statistics':
        openTimeStatistics(row)
        break
      default:
        break
    }
  }

  /** 时间统计折线图：x=采样时间，y=与上一条报文的间隔（秒） */
  const statsOption = useMemo<EChartsOption>(() => {
    const points = stats?.points ?? []
    return {
      tooltip: { trigger: 'axis' },
      grid: { left: 52, right: 24, top: 36, bottom: 56 },
      xAxis: {
        type: 'category',
        data: points.map((point) => point.time ?? ''),
        axisLabel: { rotate: 30, fontSize: 10 },
      },
      yAxis: { type: 'value', name: '秒' },
      series: [
        {
          name: '间隔（秒）',
          type: 'line',
          showSymbol: false,
          smooth: true,
          areaStyle: { opacity: 0.15 },
          data: points.map((point) => point.interval_seconds ?? 0),
        },
      ],
    }
  }, [stats])

  const handleDelete = async (row: GbDeviceItem) => {
    if (!row.id) return
    try {
      const res = await GbDeviceController.deleteGbDevice(row.id)
      if (res.code === 200) {
        message.success('删除成功')
        fetchData()
      } else {
        message.error(res.message || '删除失败')
      }
    } catch (err) {
      notifyError(err, '删除失败')
    }
  }

  const renderRegisterStatus = (value?: string) => {
    if (!value) return <span>-</span>
    const meta = REGISTER_STATUS_META[value] ?? { text: value, color: 'default' }
    return <Tag color={meta.color}>{meta.text}</Tag>
  }

  const columns: ColumnsType<GbDeviceItem> = [
    { title: '设备名称', dataIndex: 'name', width: 170, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '设备编号', dataIndex: 'device_code', width: 150, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '国标编码', dataIndex: 'gb_device_id', width: 190, ellipsis: true,
      render: (v?: string) => (v ? <Typography.Text copyable={{ text: v }}>{v}</Typography.Text> : '-'),
    },
    { title: '注册状态', dataIndex: 'register_status', width: 100, render: renderRegisterStatus },
    {
      title: '在线状态', dataIndex: 'status', width: 100,
      render: (v?: string) => <StatusTag type="device" status={v} />,
    },
    { title: '通道数', dataIndex: 'channel_count', width: 80, render: (v?: number) => (v === undefined ? '-' : v) },
    { title: '所属区域', dataIndex: 'region_name', width: 140, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '最近注册时间', dataIndex: 'register_dt', width: 170, render: (v?: string) => fmtDT(v) },
    {
      title: '操作',
      key: 'action',
      width: 420,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={400}>
          <Button type="link" size="small" icon={<InfoCircleOutlined />} onClick={() => void openDetail(row)}>
            详情
          </Button>
          {canUpdate && (
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              className="e2e-gb-device-edit-btn"
              onClick={() => void openEdit(row)}
            >
              编辑
            </Button>
          )}
          {canSync && (
            <Button
              type="link"
              size="small"
              icon={<SyncOutlined />}
              loading={syncingId === row.id}
              onClick={() => void handleSync(row)}
            >
              同步通道
            </Button>
          )}
          {deviceActionItems.length > 0 && (
            <Dropdown
              menu={{
                items: deviceActionItems,
                onClick: ({ key }) => handleDeviceAction(row, key),
              }}
            >
              <Button type="link" size="small" icon={<MoreOutlined />} className="e2e-gb-device-ops">
                设备操作
              </Button>
            </Dropdown>
          )}
          {canSubscribe && (
            <Button type="link" size="small" icon={<BellOutlined />} onClick={() => openSubscribe(row)}>
              订阅/取消订阅
            </Button>
          )}
          {canActivation && (
            <Button type="link" size="small" icon={<QrcodeOutlined />} onClick={() => void openActivation(row)}>
              生成激活凭证
            </Button>
          )}
          {canDelete && (
            <Popconfirm
              title="确定删除该设备?"
              description="删除后其全部国标通道也会被一并删除。"
              onConfirm={() => void handleDelete(row)}
            >
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  return (
    <PageContainer
      title="国标设备"
      subTitle="通过 GB/T 28181 接入的设备；国标编码与 SIP 密码由平台生成并下发到设备"
    >
      <SearchBar
        fields={[
          { name: 'keyword', label: '关键字', placeholder: '设备名称/编号/国标编码' },
          { name: 'register_status', label: '注册状态', type: 'select', options: REGISTER_STATUS_OPTIONS },
          { name: 'status', label: '在线状态', type: 'select', options: DEVICE_STATUS_OPTIONS },
          { name: 'region_id', label: '所属区域', type: 'select', options: regionOptions },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<GbDeviceItem>
        className="e2e-gb-device-table"
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 'max-content' }}
        locale={{ emptyText: '暂无国标设备' }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => { setPage(p); setPageSize(ps) },
        }}
      />

      {/* 设备详情：13 个国标接入字段（sip_password 后端已脱敏） */}
      <Modal
        className="e2e-gb-device-detail-modal"
        title={`设备详情${detail?.name ? ` - ${detail.name}` : ''}`}
        open={detailOpen}
        loading={detailLoading}
        onCancel={() => setDetailOpen(false)}
        footer={<Button onClick={() => setDetailOpen(false)}>关闭</Button>}
        width={760}
        destroyOnHidden
      >
        {detail ? (
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            {/* 基础信息 */}
            <Descriptions
              className="e2e-gb-device-detail-basic"
              title="基础信息"
              bordered
              size="small"
              column={2}
              items={[
                { key: 'name', label: '设备名称', children: detail.name || '-' },
                { key: 'code', label: '设备编号', children: detail.device_code || '-' },
                { key: 'gb_id', label: '国标编码', children: <CopyValue value={detail.gb_device_id} /> },
                {
                  key: 'register_status',
                  label: '注册状态',
                  children: renderRegisterStatus(detail.register_status),
                },
                { key: 'status', label: '在线状态', children: <StatusTag type="device" status={detail.status} /> },
                { key: 'region', label: '所属区域', children: detail.region_name || detail.region?.name || '-' },
                { key: 'register_dt', label: '最近注册时间', children: fmtDT(detail.register_dt) },
                { key: 'updated_dt', label: '更新时间', children: fmtDT(detail.updated_dt) },
                { key: 'remark', label: '备注', children: detail.remark || '-', span: 2 },
              ]}
            />

            {/* SIP 参数（sip_password 后端已脱敏） */}
            <Descriptions
              className="e2e-gb-device-detail-sip"
              title="SIP 参数"
              bordered
              size="small"
              column={2}
              items={[
                { key: 'sip_password', label: 'SIP 密码', children: <CopyValue value={detail.sip_password} /> },
                { key: 'sip_realm', label: 'SIP 域（realm）', children: detail.sip_realm || '-' },
                { key: 'sip_transport', label: '传输协议', children: detail.sip_transport || '-' },
                { key: 'sip_expires', label: '注册有效期', children: detail.sip_expires || '-' },
              ]}
            />

            {/* 媒体参数 */}
            <Descriptions
              className="e2e-gb-device-detail-media"
              title="媒体参数"
              bordered
              size="small"
              column={2}
              items={[
                {
                  key: 'media_server',
                  label: '媒体节点',
                  children: detail.media_server_name || detail.media_server_id || '-',
                },
                { key: 'stream_mode', label: '流模式', children: detail.stream_mode || '-' },
                { key: 'local_ip', label: '本地 IP', children: detail.local_ip || '-' },
                { key: 'sdp_ip', label: 'SDP IP', children: detail.sdp_ip || '-' },
                { key: 'charset', label: '字符集', children: detail.charset || '-' },
              ]}
            />

            {/* 位置信息 */}
            <Descriptions
              className="e2e-gb-device-detail-location"
              title="位置信息"
              bordered
              size="small"
              column={2}
              items={[
                { key: 'location', label: '安装位置', children: detail.location || '-', span: 2 },
                { key: 'longitude', label: '经度', children: detail.longitude || '-' },
                { key: 'latitude', label: '纬度', children: detail.latitude || '-' },
              ]}
            />

            {/*
              第二批（设备编辑 / 重启 / 布防撤防 / 报警复位 / 目录同步进度）的按钮位预留：
              对应接口尚未生成前端 Controller，此处不渲染不可用按钮，避免误导。
            */}
          </Space>
        ) : null}
      </Modal>

      {/* 激活凭证：国标编码 / SIP 密码 / 域名 / 服务器地址 + 二维码内容 */}
      <Modal
        className="e2e-gb-device-activation-modal"
        title={`激活凭证${activationDevice?.name ? ` - ${activationDevice.name}` : ''}`}
        open={activationOpen}
        loading={activationLoading}
        onCancel={() => setActivationOpen(false)}
        footer={<Button onClick={() => setActivationOpen(false)}>关闭</Button>}
        width={680}
        destroyOnHidden
      >
        {activation ? (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <Descriptions
              bordered
              size="small"
              column={1}
              items={[
                { key: 'gb_id', label: '国标编码', children: <CopyValue value={activation.gb_device_id} /> },
                { key: 'sip_password', label: 'SIP 密码', children: <CopyValue value={activation.sip_password} /> },
                { key: 'sip_domain', label: 'SIP 域名', children: <CopyValue value={activation.sip_domain} /> },
                {
                  key: 'sip_server',
                  label: '服务器地址',
                  children: <CopyValue value={activation.sip_server} />,
                },
                { key: 'sip_port', label: 'SIP 端口', children: <CopyValue value={activation.sip_port} /> },
                { key: 'transport', label: '传输协议', children: activation.transport || '-' },
                {
                  key: 'register_status',
                  label: '当前注册状态',
                  children: renderRegisterStatus(activation.register_status),
                },
              ]}
            />
            <div>
              <div style={{ marginBottom: 8, fontWeight: 500 }}>激活二维码</div>
              {activation.qr_content ? (
                // qr_content 是后端拼好的 JSON 字符串，扫码端按约定解析；
                // 二维码与文本同时展示，扫码失败时仍可复制内容手工导入。
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <div
                    style={{
                      border: '1px solid #d9d9d9',
                      borderRadius: 6,
                      padding: 12,
                      background: '#fff',
                      display: 'inline-block',
                    }}
                  >
                    <QRCodeSVG
                      value={activation.qr_content}
                      size={180}
                      level="M"
                      title="设备激活二维码"
                    />
                  </div>
                  <div
                    style={{
                      border: '1px solid #d9d9d9',
                      borderRadius: 6,
                      padding: '8px 12px',
                      background: '#fafafa',
                    }}
                  >
                    <Typography.Paragraph
                      copyable={{ text: activation.qr_content }}
                      style={{ marginBottom: 0, fontFamily: 'monospace', wordBreak: 'break-all' }}
                    >
                      {activation.qr_content}
                    </Typography.Paragraph>
                  </div>
                </Space>
              ) : (
                <Typography.Text type="secondary">后端未返回二维码内容，请确认设备已生成激活凭证。</Typography.Text>
              )}
              <Typography.Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
                使用摄像头 App / 扫码工具扫描二维码即可导入国标接入信息；二维码内容为 JSON 字符串。
              </Typography.Text>
            </div>
          </Space>
        ) : null}
      </Modal>

      {/* 订阅 / 取消订阅 */}
      <Modal
        className="e2e-gb-device-subscribe-modal"
        title={`订阅管理${subscribeDevice?.name ? ` - ${subscribeDevice.name}` : ''}`}
        open={subscribeOpen}
        onCancel={() => setSubscribeOpen(false)}
        footer={(
          <Space>
            <Button onClick={() => setSubscribeOpen(false)}>取消</Button>
            <Button danger loading={subscribing} onClick={() => void handleSubscribe('unsubscribe')}>
              取消订阅
            </Button>
            <Button type="primary" loading={subscribing} onClick={() => void handleSubscribe('subscribe')}>
              订阅
            </Button>
          </Space>
        )}
        width={480}
        destroyOnHidden
      >
        <div style={{ marginBottom: 8 }}>订阅类型</div>
        <Select
          style={{ width: '100%' }}
          value={subscribeType}
          onChange={(value) => setSubscribeType(value)}
          options={SUBSCRIBE_TYPE_OPTIONS}
        />
        <Typography.Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
          订阅通过 SIP SUBSCRIBE 下发；SIP 信令服务未启动时接口会返回 503。
        </Typography.Text>
      </Modal>

      {/* 编辑设备：SIP / 媒体 / 位置三组字段 */}
      <Modal
        className="e2e-gb-device-edit-modal"
        title={`编辑设备${editing?.name ? ` - ${editing.name}` : ''}`}
        open={editOpen}
        loading={editLoading}
        onCancel={() => setEditOpen(false)}
        onOk={() => void handleSaveEdit()}
        confirmLoading={saving}
        width={760}
        forceRender
        destroyOnHidden
      >
        <Form form={form} layout="vertical" initialValues={buildDeviceFormValues(editing)}>
          <Divider orientation="left" style={{ margin: '0 0 12px' }}>基础信息</Divider>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Form.Item
              name="name"
              label="设备名称"
              rules={[{ required: true, message: '请输入设备名称' }]}
            >
              <Input className="e2e-gb-device-edit-name" maxLength={255} placeholder="请输入设备名称" />
            </Form.Item>
            <Form.Item
              name="gb_device_id"
              label="国标编码"
              rules={[{ pattern: /^\d{20}$/, message: '国标编码必须是 20 位数字' }]}
              extra={editing?.status === 'online' ? '设备在线时后端不允许修改国标编码' : undefined}
            >
              <Input
                className="e2e-gb-device-edit-gb-id"
                maxLength={20}
                disabled={editing?.status === 'online'}
                placeholder="20 位数字"
              />
            </Form.Item>
            <Form.Item name="gb_version" label="国标协议版本">
              <Select className="e2e-gb-device-edit-version" allowClear options={GB_VERSION_OPTIONS} placeholder="请选择版本" />
            </Form.Item>
          </div>

          <Divider orientation="left" style={{ margin: '4px 0 12px' }}>SIP 参数</Divider>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Form.Item
              name="sip_realm"
              label="SIP 域（realm）"
              rules={[{ pattern: /^\d{10}$/, message: 'SIP 域必须是 10 位数字' }]}
            >
              <Input className="e2e-gb-device-edit-realm" maxLength={10} placeholder="10 位数字" />
            </Form.Item>
            <Form.Item name="sip_transport" label="传输协议">
              <Select className="e2e-gb-device-edit-transport" allowClear options={SIP_TRANSPORT_OPTIONS} placeholder="请选择传输协议" />
            </Form.Item>
            <Form.Item name="charset" label="字符集">
              <Select className="e2e-gb-device-edit-charset" allowClear options={CHARSET_OPTIONS} placeholder="请选择字符集" />
            </Form.Item>
          </div>

          <Divider orientation="left" style={{ margin: '4px 0 12px' }}>媒体参数</Divider>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Form.Item name="media_server_id" label="媒体节点">
              <Select
                className="e2e-gb-device-edit-media-server"
                options={mediaServerOptions}
                placeholder="自动分配"
              />
            </Form.Item>
            <Form.Item name="stream_mode" label="媒体流传输方式">
              <Select className="e2e-gb-device-edit-stream-mode" allowClear options={STREAM_MODE_OPTIONS} placeholder="请选择传输方式" />
            </Form.Item>
            {/* IP 合法性交给后端 net.ParseIP 校验（支持 IPv6），前端不重复限制 */}
            <Form.Item name="local_ip" label="本地 IP（local_ip）">
              <Input className="e2e-gb-device-edit-local-ip" placeholder="留空表示自动" />
            </Form.Item>
            <Form.Item name="sdp_ip" label="SDP IP（sdp_ip）">
              <Input className="e2e-gb-device-edit-sdp-ip" placeholder="留空表示自动" />
            </Form.Item>
          </div>

          <Divider orientation="left" style={{ margin: '4px 0 12px' }}>位置信息</Divider>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Form.Item name="location" label="安装位置">
              <Input className="e2e-gb-device-edit-location" maxLength={255} placeholder="请输入安装位置" />
            </Form.Item>
            <Form.Item
              name="longitude"
              label="经度"
              rules={[
                {
                  validator: async (_, value?: string) => {
                    if (!value) return
                    const n = Number(value)
                    if (Number.isNaN(n) || n < -180 || n > 180) throw new Error('经度必须在 -180 到 180 之间')
                  },
                },
              ]}
            >
              <Input className="e2e-gb-device-edit-longitude" placeholder="例如 116.397128" />
            </Form.Item>
            <Form.Item
              name="latitude"
              label="纬度"
              rules={[
                {
                  validator: async (_, value?: string) => {
                    if (!value) return
                    const n = Number(value)
                    if (Number.isNaN(n) || n < -90 || n > 90) throw new Error('纬度必须在 -90 到 90 之间')
                  },
                },
              ]}
            >
              <Input className="e2e-gb-device-edit-latitude" placeholder="例如 39.916527" />
            </Form.Item>
          </div>

          <Form.Item name="remark" label="备注" style={{ marginBottom: 0 }}>
            <Input.TextArea className="e2e-gb-device-edit-remark" rows={3} maxLength={500} showCount placeholder="请输入备注" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 目录同步进度：成功 / 失败自动停止轮询 */}
      <Modal
        className="e2e-gb-device-sync-modal"
        title={`目录同步${syncDevice?.name ? ` - ${syncDevice.name}` : ''}`}
        open={syncOpen}
        onCancel={closeSyncModal}
        footer={<Button onClick={closeSyncModal}>关闭</Button>}
        width={560}
        destroyOnHidden
      >
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <div>
            <div style={{ marginBottom: 8, fontSize: 12, color: '#8c8c8c' }}>
              {syncStatus?.status === 'syncing'
                ? '设备目录同步中，请稍候…'
                : syncStatus?.status === 'success'
                  ? '目录同步完成'
                  : syncStatus?.status === 'failed'
                    ? '目录同步失败'
                    : '等待设备上报目录'}
            </div>
            <Progress
              className="e2e-gb-device-sync-progress"
              percent={
                syncStatus?.total && syncStatus.total > 0
                  ? Math.min(100, Math.round(((syncStatus.current ?? 0) / syncStatus.total) * 100))
                  : 0
              }
              status={
                syncStatus?.status === 'failed'
                  ? 'exception'
                  : syncStatus?.status === 'success'
                    ? 'success'
                    : 'active'
              }
            />
          </div>
          <Descriptions
            className="e2e-gb-device-sync-descriptions"
            bordered
            size="small"
            column={2}
            items={[
              {
                key: 'status',
                label: '同步状态',
                children: (() => {
                  const meta = SYNC_STATUS_META[syncStatus?.status ?? 'idle'] ?? {
                    text: syncStatus?.status || '未知',
                    color: 'default',
                  }
                  return <Tag color={meta.color}>{meta.text}</Tag>
                })(),
              },
              { key: 'total', label: '通道总数', children: syncStatus?.total ?? '-' },
              { key: 'current', label: '已同步', children: syncStatus?.current ?? '-' },
              { key: 'error', label: '错误信息', children: syncStatus?.error || '-' },
              { key: 'started', label: '开始时间', children: fmtDTSeconds(syncStatus?.started_dt) },
              { key: 'finished', label: '结束时间', children: fmtDTSeconds(syncStatus?.finished_dt) },
            ]}
          />
        </Space>
      </Modal>

      {/* 报警复位：可选指定通道，不选表示复位设备本身 */}
      <Modal
        className="e2e-gb-device-alarm-reset-modal"
        title={`报警复位${alarmDevice?.name ? ` - ${alarmDevice.name}` : ''}`}
        open={alarmOpen}
        onCancel={() => setAlarmOpen(false)}
        onOk={() => void handleAlarmReset()}
        confirmLoading={alarmLoading}
        okText="下发复位"
        width={480}
        destroyOnHidden
      >
        <div style={{ marginBottom: 8 }}>复位通道（可选）</div>
        <Select
          className="e2e-gb-device-alarm-reset-channel"
          allowClear
          showSearch
          optionFilterProp="label"
          loading={alarmLoading}
          style={{ width: '100%' }}
          placeholder="不选表示复位设备本身"
          value={alarmChannelID}
          onChange={(value?: number) => setAlarmChannelID(value)}
          options={alarmChannels
            .filter((channel) => typeof channel.id === 'number')
            .map((channel) => ({
              label: channel.name || channel.gb_channel_id || `通道 #${channel.id}`,
              value: Number(channel.id),
            }))}
        />
        <Typography.Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
          SIP 信令服务未启动时接口会返回 503。
        </Typography.Text>
      </Modal>

      {/* 心跳 / 注册间隔统计 */}
      <Modal
        className="e2e-gb-device-time-statistics-modal"
        title={`时间统计${statsDevice?.name ? ` - ${statsDevice.name}` : ''}`}
        open={statsOpen}
        onCancel={() => setStatsOpen(false)}
        footer={<Button onClick={() => setStatsOpen(false)}>关闭</Button>}
        width={820}
        destroyOnHidden
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Space wrap>
            <span style={{ fontSize: 12, color: '#8c8c8c' }}>类型</span>
            <Select
              className="e2e-gb-device-time-stat-type"
              style={{ width: 220 }}
              value={statsType}
              options={TIME_STAT_TYPE_OPTIONS}
              onChange={(value) => {
                setStatsType(value)
                if (statsDevice?.id) void loadTimeStatistics(statsDevice.id, value, statsCount)
              }}
            />
            <span style={{ fontSize: 12, color: '#8c8c8c' }}>条数</span>
            <InputNumber
              className="e2e-gb-device-time-stat-count"
              min={2}
              max={500}
              precision={0}
              value={statsCount}
              style={{ width: 100 }}
              onChange={(value) => {
                const next = typeof value === 'number' ? value : 50
                setStatsCount(next)
                if (statsDevice?.id) void loadTimeStatistics(statsDevice.id, statsType, next)
              }}
            />
          </Space>

          <Descriptions
            className="e2e-gb-device-time-stat-summary"
            bordered
            size="small"
            column={4}
            items={[
              { key: 'count', label: '样本数', children: stats?.count ?? '-' },
              { key: 'avg', label: '平均间隔', children: stats?.average_seconds !== undefined ? `${stats.average_seconds} 秒` : '-' },
              { key: 'min', label: '最小间隔', children: stats?.min_seconds !== undefined ? `${stats.min_seconds} 秒` : '-' },
              { key: 'max', label: '最大间隔', children: stats?.max_seconds !== undefined ? `${stats.max_seconds} 秒` : '-' },
            ]}
          />

          {statsLoading ? (
            <div style={{ padding: 32, textAlign: 'center' }}>
              <Spin size="small" />
            </div>
          ) : (stats?.points?.length ?? 0) > 0 ? (
            <ReactECharts
              className="e2e-gb-device-time-stat-chart"
              option={statsOption}
              style={{ height: 320 }}
              opts={{ renderer: 'canvas' }}
              notMerge
            />
          ) : (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="暂无统计样本：设备需至少上报两条同类型报文"
            />
          )}
        </Space>
      </Modal>
    </PageContainer>
  )
}
