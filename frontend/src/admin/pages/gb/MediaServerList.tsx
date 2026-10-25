import {
  forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState,
} from 'react'
import {
  Alert, Button, Card, Col, Descriptions, Divider, Empty, Form, Input, InputNumber, Modal, Popconfirm,
  Progress, Radio, Row, Select, Space, Spin, Switch, Table, Tabs, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  BarChartOutlined, CheckCircleOutlined, DeleteOutlined, EditOutlined, InfoCircleOutlined, PlusOutlined,
  VideoCameraOutlined,
} from '@ant-design/icons'
import ReactECharts from 'echarts-for-react'
import type { EChartsOption } from 'echarts'
import dayjs from 'dayjs'
import axios from 'axios'
import { useSearchParams } from 'react-router-dom'
import PageContainer from '../../components/PageContainer'
import ActionButtons from '../../components/ActionButtons'
import { GbMediaServerController } from '../../api/generated/GbMediaServerController'
import { GbOpsController } from '../../api/generated/GbOpsController'
import type { GbMediaServerItem } from '../../api/generated/definitions/GbMediaServerItem'
import type { GbMediaServerCheckResp } from '../../api/generated/definitions/GbMediaServerCheckResp'
import type { GbMediaServerInfoResp } from '../../api/generated/definitions/GbMediaServerInfoResp'
import type { GbMediaServerThreadLoad } from '../../api/generated/definitions/GbMediaServerThreadLoad'
import type { GbMediaMetricsResp } from '../../api/generated/definitions/GbMediaMetricsResp'
import type { GbMediaMetricPoint } from '../../api/generated/definitions/GbMediaMetricPoint'
import type { GbMediaStreamListResp } from '../../api/generated/definitions/GbMediaStreamListResp'
import type { GbMediaStreamItem } from '../../api/generated/definitions/GbMediaStreamItem'
import type { CreateGbMediaServerReq } from '../../api/generated/definitions/CreateGbMediaServerReq'
import type { UpdateGbMediaServerReq } from '../../api/generated/definitions/UpdateGbMediaServerReq'
import { useAuthStore } from '../../store/authStore'

/** 媒体节点表单值 */
interface MediaServerFormValues {
  name: string
  server_id: string
  type?: string
  ip?: string
  hook_ip?: string
  sdp_ip?: string
  stream_ip?: string
  http_port?: number | null
  http_ssl_port?: number | null
  rtp_proxy_port?: number | null
  rtp_enable?: boolean
  rtp_port_range?: string
  send_rtp_port_range?: string
  rtmp_port?: number | null
  rtsp_port?: number | null
  flv_port?: number | null
  record_path?: string
  record_day?: number | null
  transcode_suffix?: string
  record_assist_port?: number | null
  auto_config?: boolean
  hook_alive_interval?: number | null
  secret?: string
  default_server?: boolean
  active?: boolean
  remark?: string
}

/** 节点类型选项（当前仅对接 ZLMediaKit） */
const TYPE_OPTIONS = [{ label: 'ZLMediaKit', value: 'zlm' }]

/** 时间格式化 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = dayjs(v)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : v
}

/** InputNumber 空值统一转 undefined，避免把 null 写进请求体 */
function num(v?: number | null): number | undefined {
  return typeof v === 'number' && !Number.isNaN(v) ? v : undefined
}

/** 节点 API 地址：http://<ip>:<http_port> */
function apiAddress(row: GbMediaServerItem): string {
  if (!row.ip) return '-'
  return `http://${row.ip}${row.http_port ? `:${row.http_port}` : ''}`
}

/** 新建 / 编辑的初始值。 */
function buildMediaServerInitialValues(editing: GbMediaServerItem | null): Partial<MediaServerFormValues> {
  if (!editing) {
    return {
      type: 'zlm',
      // 本项目默认单端口收流（rtp_enable=false）
      rtp_enable: false,
      rtp_port_range: '30000-30500',
      record_day: 7,
      record_assist_port: 0,
      hook_alive_interval: 15,
      auto_config: false,
      default_server: false,
      active: true,
    }
  }
  return {
    name: editing.name ?? '',
    server_id: editing.server_id ?? '',
    type: editing.type || 'zlm',
    ip: editing.ip ?? '',
    hook_ip: editing.hook_ip ?? '',
    sdp_ip: editing.sdp_ip ?? '',
    stream_ip: editing.stream_ip ?? '',
    http_port: editing.http_port ?? undefined,
    http_ssl_port: editing.http_ssl_port ?? undefined,
    rtp_proxy_port: editing.rtp_proxy_port ?? undefined,
    rtp_enable: editing.rtp_enable ?? false,
    rtp_port_range: editing.rtp_port_range ?? '',
    send_rtp_port_range: editing.send_rtp_port_range ?? '',
    rtmp_port: editing.rtmp_port ?? undefined,
    rtsp_port: editing.rtsp_port ?? undefined,
    flv_port: editing.flv_port ?? undefined,
    record_path: editing.record_path ?? '',
    record_day: editing.record_day ?? 7,
    transcode_suffix: editing.transcode_suffix ?? '',
    record_assist_port: editing.record_assist_port ?? 0,
    auto_config: editing.auto_config ?? false,
    hook_alive_interval: editing.hook_alive_interval ?? 15,
    // secret 后端已脱敏为 ****；留空表示不修改
    secret: '',
    default_server: editing.default_server ?? false,
    active: editing.active ?? true,
    remark: editing.remark ?? '',
  }
}

/** 空串统一转 undefined，避免把空值写进请求体 */
function text(v?: string | null): string | undefined {
  const trimmed = v?.trim()
  return trimmed ? trimmed : undefined
}

/** 请求失败提示：优先展示后端返回的 message（如媒体节点不可达） */
function apiErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError<{ message?: string }>(err)) {
    return err.response?.data?.message || err.message || fallback
  }
  return err instanceof Error ? err.message : fallback
}

function formatBytes(value?: number): string {
  if (value === undefined || value === null || value < 0) return '-'
  if (value < 1024) return `${value} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let size = value / 1024
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index += 1
  }
  return `${size.toFixed(1)} ${units[index]}`
}

function formatRate(value?: number): string {
  if (value === undefined || value === null) return '-'
  return `${formatBytes(value)}/s`
}

/** 秒数 → 天/时/分/秒（流存活时长、平台运行时长共用） */
function formatAlive(totalSeconds?: number): string {
  if (totalSeconds === undefined || totalSeconds === null || totalSeconds < 0) return '-'
  const seconds = Math.floor(totalSeconds)
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const rest = seconds % 60
  if (days > 0) return `${days} 天 ${hours} 小时`
  if (hours > 0) return `${hours} 小时 ${minutes} 分`
  if (minutes > 0) return `${minutes} 分 ${rest} 秒`
  return `${rest} 秒`
}

function round1(value?: number): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? Math.round(value * 10) / 10 : null
}

/** 进度条百分比：空值按 0，限制在 0~100 */
function toPercent(value?: number): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) return 0
  return Math.max(0, Math.min(100, Math.round(value)))
}

const METRIC_COLORS = {
  cpu: '#2a78d6',
  mem: '#1baf7a',
  disk: '#eda100',
  rx: '#2a78d6',
  tx: '#eb6834',
}

/** 统一图表的骨架样式 */
const CHART_BASE: EChartsOption = {
  tooltip: { trigger: 'axis' },
  legend: { bottom: 0, itemWidth: 10, itemHeight: 8, textStyle: { color: '#52514e', fontSize: 12 } },
  grid: { left: 8, right: 16, top: 24, bottom: 36, containLabel: true },
}

function trendLabels(points: GbMediaMetricPoint[]): string[] {
  return points.map((point) => {
    const parsed = point.ts ? dayjs(point.ts) : null
    return parsed && parsed.isValid() ? parsed.format('HH:mm') : ''
  })
}

/** CPU / 内存 / 磁盘 使用率趋势（%） */
function percentTrendOption(points: GbMediaMetricPoint[]): EChartsOption {
  return {
    ...CHART_BASE,
    color: [METRIC_COLORS.cpu, METRIC_COLORS.mem, METRIC_COLORS.disk],
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: trendLabels(points),
      axisTick: { show: false },
      axisLine: { lineStyle: { color: '#ececec' } },
      axisLabel: { color: '#52514e', fontSize: 12, hideOverlap: true },
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: 100,
      axisLabel: { color: '#52514e', fontSize: 12, formatter: '{value}%' },
      splitLine: { lineStyle: { color: '#ececec' } },
    },
    series: [
      {
        name: 'CPU',
        type: 'line',
        smooth: true,
        showSymbol: false,
        lineStyle: { width: 2 },
        areaStyle: { opacity: 0.06 },
        data: points.map((point) => round1(point.cpu_percent)),
      },
      {
        name: '内存',
        type: 'line',
        smooth: true,
        showSymbol: false,
        lineStyle: { width: 2 },
        areaStyle: { opacity: 0.06 },
        data: points.map((point) => round1(point.mem_percent)),
      },
      {
        name: '磁盘',
        type: 'line',
        smooth: true,
        showSymbol: false,
        lineStyle: { width: 2 },
        areaStyle: { opacity: 0.06 },
        data: points.map((point) => round1(point.disk_percent)),
      },
    ],
  }
}

/** 网络收发速率趋势（字节/秒） */
function netTrendOption(points: GbMediaMetricPoint[]): EChartsOption {
  return {
    ...CHART_BASE,
    color: [METRIC_COLORS.rx, METRIC_COLORS.tx],
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: trendLabels(points),
      axisTick: { show: false },
      axisLine: { lineStyle: { color: '#ececec' } },
      axisLabel: { color: '#52514e', fontSize: 12, hideOverlap: true },
    },
    yAxis: {
      type: 'value',
      min: 0,
      axisLabel: {
        color: '#52514e',
        fontSize: 12,
        formatter: (value: number) => formatBytes(value),
      },
      splitLine: { lineStyle: { color: '#ececec' } },
    },
    series: [
      {
        name: '接收',
        type: 'line',
        smooth: true,
        showSymbol: false,
        lineStyle: { width: 2 },
        areaStyle: { opacity: 0.06 },
        data: points.map((point) => round1(point.net_rx_rate)),
      },
      {
        name: '发送',
        type: 'line',
        smooth: true,
        showSymbol: false,
        lineStyle: { width: 2 },
        areaStyle: { opacity: 0.06 },
        data: points.map((point) => round1(point.net_tx_rate)),
      },
    ],
  }
}

type DetailTab = 'metrics' | 'streams'

interface DetailState {
  server: GbMediaServerItem
  tab: DetailTab
}

/** 流会话列（schema / 编码 / 分辨率 / 观看数 / 码率 / 存活时长） */
const streamColumns: ColumnsType<GbMediaStreamItem> = [
  { title: '应用名', dataIndex: 'app', width: 110, render: (v?: string) => v || '-' },
  {
    title: '流 ID',
    dataIndex: 'stream',
    width: 170,
    ellipsis: true,
    render: (v?: string) => (
      <span style={{ fontFamily: 'Consolas, Monaco, monospace' }}>{v || '-'}</span>
    ),
  },
  {
    title: '协议',
    dataIndex: 'schema',
    width: 90,
    render: (v?: string) => (v ? <Tag color="geekblue">{v}</Tag> : '-'),
  },
  {
    title: '编码',
    key: 'codec',
    width: 130,
    render: (_, row) => {
      const audio = row.audio_codec ? ` / ${row.audio_codec}` : ''
      return row.codec ? `${row.codec}${audio}` : '-'
    },
  },
  {
    title: '分辨率 / 帧率',
    key: 'resolution',
    width: 150,
    render: (_, row) => {
      const size = row.width && row.height ? `${row.width}×${row.height}` : '-'
      const fps = row.fps != null ? ` @${Math.round(row.fps)}fps` : ''
      return `${size}${fps}`
    },
  },
  {
    title: '观看数',
    dataIndex: 'reader_count',
    width: 90,
    render: (v?: number) => (v ?? 0),
  },
  {
    title: '码率',
    dataIndex: 'bytes_speed',
    width: 120,
    render: (v?: number) => formatRate(v),
  },
  {
    title: '存活时长',
    dataIndex: 'alive_second',
    width: 130,
    render: (v?: number) => formatAlive(v),
  },
  {
    title: '来源',
    key: 'origin',
    width: 240,
    ellipsis: true,
    render: (_, row) => {
      if (row.origin_url) return row.origin_url
      if (row.origin_peer_ip) return `${row.origin_peer_ip}${row.origin_peer_port ? `:${row.origin_peer_port}` : ''}`
      return row.origin_type || '-'
    },
  },
]

interface MediaServerFormHandle {
  submit: () => Promise<void>
}

interface MediaServerFormContentProps {
  editing: GbMediaServerItem | null
  onSubmit: (values: MediaServerFormValues) => Promise<void>
}

/**
 * 媒体节点表单内容。
 *
 * form 与 Form 一起挂在 Modal children 内（destroyOnHidden 时一起卸载），
 * 父组件不再在弹窗打开前调用 resetFields/setFieldsValue。
 */
const MediaServerFormContent = forwardRef<MediaServerFormHandle, MediaServerFormContentProps>(
  function MediaServerFormContent({ editing, onSubmit }, ref) {
    const [form] = Form.useForm<MediaServerFormValues>()

    useImperativeHandle(ref, () => ({
      async submit() {
        let values: MediaServerFormValues
        try {
          values = await form.validateFields()
        } catch {
          return
        }
        await onSubmit(values)
      },
    }), [form, onSubmit])

    return (
      <Form form={form} layout="vertical" initialValues={buildMediaServerInitialValues(editing)}>
        <Divider orientation="left" plain style={{ marginTop: 0 }}>基础信息</Divider>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="name" label="节点名称" rules={[{ required: true, message: '请输入节点名称' }]}>
              <Input placeholder="如：本地 ZLMediaKit" maxLength={255} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="server_id"
              label="server_id"
              rules={[{ required: true, message: '请输入节点唯一标识' }]}
              extra="需与 ZLM 配置的 mediaServerId 一致"
            >
              <Input placeholder="如：ai-camera-zlm-01" maxLength={50} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="type" label="类型">
              <Select options={TYPE_OPTIONS} placeholder="请选择节点类型" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="auto_config" label="自动配置" valuePropName="checked" extra="由平台自动下发/同步节点配置">
              <Switch />
            </Form.Item>
          </Col>
        </Row>

        <Divider orientation="left" plain>网络与地址</Divider>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              name="ip"
              label="API 地址（IP）"
              rules={[{ required: true, message: '请输入节点 API IP' }]}
              extra="后端调用 ZLM HTTP API 的地址"
            >
              <Input placeholder="如：127.0.0.1" maxLength={255} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="hook_ip" label="回调地址（hook_ip）" extra="ZLM 回调本服务使用的地址，留空沿用 API 地址">
              <Input placeholder="如：10.0.0.2" maxLength={50} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="sdp_ip" label="SDP 通告地址（sdp_ip）" extra="SDP 中下发给设备的 IP，多网卡/NAT 场景必填">
              <Input placeholder="如：公网或专网出口 IP" maxLength={50} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="stream_ip" label="播放地址（stream_ip）" extra="对外拼接播放地址使用的 IP">
              <Input placeholder="如：10.0.0.2" maxLength={50} />
            </Form.Item>
          </Col>
        </Row>

        <Divider orientation="left" plain>端口</Divider>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item name="http_port" label="HTTP 端口">
              <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="如：8082" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="http_ssl_port" label="HTTPS(SSL) 端口">
              <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="如：8443" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="rtp_proxy_port" label="RTP 收流端口">
              <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="单端口收流端口" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="rtmp_port" label="RTMP 端口">
              <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="如：1935" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="rtsp_port" label="RTSP 端口">
              <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="如：554" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="flv_port" label="HTTP-FLV 端口">
              <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="如：8082" />
            </Form.Item>
          </Col>
        </Row>

        <Divider orientation="left" plain>RTP 收流</Divider>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item
              name="rtp_enable"
              label="多端口 RTP 收流"
              valuePropName="checked"
              extra="关闭为单端口收流（本项目默认）"
            >
              <Switch checkedChildren="多端口" unCheckedChildren="单端口" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="rtp_port_range" label="RTP 收流端口段" extra="仅多端口模式生效，如 30000-30500">
              <Input placeholder="30000-30500" maxLength={50} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="send_rtp_port_range" label="语音回推端口段" extra="语音对讲使用的发送端口段">
              <Input placeholder="如：40000-40500" maxLength={50} />
            </Form.Item>
          </Col>
        </Row>

        <Divider orientation="left" plain>录像与转码</Divider>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item name="record_day" label="录像保留天数">
              <InputNumber min={1} max={365} style={{ width: '100%' }} placeholder="默认 7 天" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="record_assist_port" label="Assist 服务端口" extra="0 表示未启用，云端录像仅支持单文件下载">
              <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="0（未启用）" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="transcode_suffix" label="转码后缀" extra="如 h264；用于区分转码后的流">
              <Input placeholder="留空表示不转码" maxLength={255} />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item name="record_path" label="录像存储路径" extra="ZLM 落盘目录，留空使用节点默认配置">
              <Input placeholder="如：/opt/media/bin/www/record" maxLength={500} />
            </Form.Item>
          </Col>
        </Row>

        <Divider orientation="left" plain>健康检查与其他</Divider>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item name="hook_alive_interval" label="心跳间隔（秒）">
              <InputNumber min={5} max={300} style={{ width: '100%' }} placeholder="默认 15 秒" />
            </Form.Item>
          </Col>
          <Col span={16}>
            <Form.Item
              name="secret"
              label="API Secret"
              extra={editing ? '留空或 **** 表示不修改（后端已脱敏）' : 'ZLM 的 api.secret'}
            >
              <Input.Password
                placeholder={editing ? '留空表示不修改' : '请输入 ZLM API secret'}
                autoComplete="new-password"
                maxLength={255}
              />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="default_server" label="默认节点" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="active" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item name="remark" label="备注">
              <Input.TextArea rows={2} maxLength={500} showCount placeholder="请输入备注" />
            </Form.Item>
          </Col>
        </Row>
      </Form>
    )
  },
)

/**
 * 媒体节点（ZLMediaKit）。
 *
 * 本项目按**单端口收流**模式部署（rtp_enable=false）：设备统一向节点的
 * RTP 收流端口打流，页面不提供多端口 RTP 范围配置。
 */
export default function MediaServerList() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('gb:media-server:create')
  const canUpdate = permCodes.includes('gb:media-server:update')
  const canDelete = permCodes.includes('gb:media-server:delete')
  const canCheck = permCodes.includes('gb:media-server:check')
  const canMetrics = permCodes.includes('gb:media-server:metrics')
  const canStreams = permCodes.includes('gb:media-server:streams')

  const [data, setData] = useState<GbMediaServerItem[]>([])
  const [loading, setLoading] = useState(false)

  /* ── 指标 / 流会话详情弹窗 ── */
  const [searchParams, setSearchParams] = useSearchParams()
  const [detail, setDetail] = useState<DetailState | null>(null)
  const [metrics, setMetrics] = useState<GbMediaMetricsResp | null>(null)
  const [metricsLoading, setMetricsLoading] = useState(false)
  const [metricsError, setMetricsError] = useState('')
  const [historyRange, setHistoryRange] = useState<'1h' | '24h'>('1h')
  const [historyPoints, setHistoryPoints] = useState<GbMediaMetricPoint[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)
  const [historyError, setHistoryError] = useState('')
  const [streamsData, setStreamsData] = useState<GbMediaStreamListResp | null>(null)
  const [streamsLoading, setStreamsLoading] = useState(false)
  const [streamsError, setStreamsError] = useState('')
  const autoOpenHandledRef = useRef(false)

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<GbMediaServerItem | null>(null)
  const [saving, setSaving] = useState(false)
  const formRef = useRef<MediaServerFormHandle>(null)

  const [checkingId, setCheckingId] = useState<number | null>(null)
  const [checkOpen, setCheckOpen] = useState(false)
  const [checkResult, setCheckResult] = useState<GbMediaServerCheckResp | null>(null)

  const [infoOpen, setInfoOpen] = useState(false)
  const [infoLoading, setInfoLoading] = useState(false)
  const [infoResult, setInfoResult] = useState<GbMediaServerInfoResp | null>(null)
  const [infoNodeName, setInfoNodeName] = useState('')

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbMediaServerController.listGbMediaServers()
      if (res.code === 200) {
        setData(res.data ?? [])
      } else {
        message.error(res.message || '获取媒体节点列表失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const openDetail = useCallback((server: GbMediaServerItem, tab: DetailTab) => {
    // 每次打开都重置数据，避免看到上一次节点的指标
    setDetail({ server, tab })
    setMetrics(null)
    setMetricsError('')
    setHistoryPoints([])
    setHistoryError('')
    setHistoryRange('1h')
    setStreamsData(null)
    setStreamsError('')
  }, [])

  const closeDetail = () => setDetail(null)

  const switchDetailTab = (key: string) => {
    setDetail((prev) => (prev ? { ...prev, tab: key === 'streams' ? 'streams' : 'metrics' } : prev))
  }

  /** Dashboard「有流通道」通过 ?streamServerId= 跳转，列表加载完成后自动打开流会话 */
  useEffect(() => {
    if (autoOpenHandledRef.current || data.length === 0) return
    const raw = searchParams.get('streamServerId')
    if (!raw) return
    autoOpenHandledRef.current = true
    const target = data.find((item) => String(item.id) === raw)
    setSearchParams({}, { replace: true })
    if (target && canStreams) openDetail(target, 'streams')
  }, [data, searchParams, setSearchParams, canStreams, openDetail])

  /** 当前指标：弹窗打开且停在「指标」页时每 5 秒轮询，关闭/切页即停止 */
  useEffect(() => {
    if (!detail || detail.tab !== 'metrics') return undefined
    const id = detail.server.id
    if (!id) return undefined

    let cancelled = false
    const load = async () => {
      setMetricsLoading(true)
      try {
        const res = await GbOpsController.getGbMediaServerMetrics(id)
        if (cancelled) return
        setMetrics(res.data ?? null)
        setMetricsError('')
      } catch (err) {
        if (!cancelled) setMetricsError(apiErrorMessage(err, '获取节点指标失败'))
      } finally {
        if (!cancelled) setMetricsLoading(false)
      }
    }

    void load()
    const timer = window.setInterval(() => { void load() }, 5000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [detail])

  /** 指标趋势（1h / 24h）：打开时加载，之后每 30 秒刷新一次 */
  useEffect(() => {
    if (!detail || detail.tab !== 'metrics') return undefined
    const id = detail.server.id
    if (!id) return undefined

    let cancelled = false
    const load = async () => {
      setHistoryLoading(true)
      try {
        const res = await GbOpsController.getGbMediaServerMetricsHistory(id, historyRange)
        if (cancelled) return
        setHistoryPoints(res.data?.items ?? [])
        setHistoryError('')
      } catch (err) {
        if (!cancelled) setHistoryError(apiErrorMessage(err, '获取指标趋势失败'))
      } finally {
        if (!cancelled) setHistoryLoading(false)
      }
    }

    void load()
    const timer = window.setInterval(() => { void load() }, 30000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [detail, historyRange])

  /** 流会话：弹窗打开且停在「流会话」页时每 5 秒轮询，关闭/切页即停止 */
  useEffect(() => {
    if (!detail || detail.tab !== 'streams') return undefined
    const id = detail.server.id
    if (!id) return undefined

    let cancelled = false
    const load = async () => {
      setStreamsLoading(true)
      try {
        const res = await GbOpsController.listGbMediaServerStreams(id)
        if (cancelled) return
        setStreamsData(res.data ?? null)
        setStreamsError('')
      } catch (err) {
        if (!cancelled) setStreamsError(apiErrorMessage(err, '获取流会话失败'))
      } finally {
        if (!cancelled) setStreamsLoading(false)
      }
    }

    void load()
    const timer = window.setInterval(() => { void load() }, 5000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [detail])

  const openCreate = () => {
    setEditing(null)
    setModalOpen(true)
  }

  const openEdit = (row: GbMediaServerItem) => {
    setEditing(row)
    setModalOpen(true)
  }

  const handleSave = async (values: MediaServerFormValues) => {
    const common: UpdateGbMediaServerReq = {
      type: values.type,
      ip: values.ip?.trim(),
      hook_ip: text(values.hook_ip),
      sdp_ip: text(values.sdp_ip),
      stream_ip: text(values.stream_ip),
      http_port: num(values.http_port),
      http_ssl_port: num(values.http_ssl_port),
      rtp_proxy_port: num(values.rtp_proxy_port),
      // 单端口 / 多端口收流由表单显式控制（项目默认关闭）
      rtp_enable: values.rtp_enable,
      rtp_port_range: text(values.rtp_port_range),
      send_rtp_port_range: text(values.send_rtp_port_range),
      rtmp_port: num(values.rtmp_port),
      rtsp_port: num(values.rtsp_port),
      flv_port: num(values.flv_port),
      record_path: text(values.record_path),
      record_day: num(values.record_day),
      transcode_suffix: text(values.transcode_suffix),
      // 0 表示未启用，需要显式提交
      record_assist_port: num(values.record_assist_port),
      auto_config: values.auto_config,
      hook_alive_interval: num(values.hook_alive_interval),
      default_server: values.default_server,
      active: values.active,
      remark: values.remark,
    }

    setSaving(true)
    try {
      if (editing?.id) {
        const payload: UpdateGbMediaServerReq = { ...common }
        if (values.secret) payload.secret = values.secret
        const res = await GbMediaServerController.updateGbMediaServer(String(editing.id), payload)
        if (res.code === 200) {
          message.success('修改成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '保存失败')
        }
      } else {
        const payload: CreateGbMediaServerReq = {
          ...common,
          name: values.name,
          server_id: values.server_id,
          secret: values.secret || undefined,
        }
        const res = await GbMediaServerController.createGbMediaServer(payload)
        if (res.code === 200) {
          message.success('新增成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '保存失败')
        }
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (row: GbMediaServerItem) => {
    if (!row.id) return
    try {
      const res = await GbMediaServerController.deleteGbMediaServer(String(row.id))
      if (res.code === 200) {
        message.success('删除成功')
        fetchData()
      } else {
        message.error(res.message || '删除失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    }
  }

  const handleCheck = async (row: GbMediaServerItem) => {
    if (!row.id) return
    setCheckingId(row.id)
    try {
      const res = await GbMediaServerController.checkGbMediaServer(String(row.id))
      if (res.code === 200) {
        setCheckResult(res.data ?? null)
        setCheckOpen(true)
      } else {
        message.error(res.message || '在线检查失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setCheckingId(null)
    }
  }

  const handleInfo = async (row: GbMediaServerItem) => {
    if (!row.id) return
    setInfoNodeName(row.name ?? '')
    setInfoResult(null)
    setInfoOpen(true)
    setInfoLoading(true)
    try {
      const res = await GbMediaServerController.getGbMediaServerInfo(String(row.id))
      if (res.code === 200) {
        setInfoResult(res.data ?? null)
      } else {
        message.error(res.message || '获取节点信息失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setInfoLoading(false)
    }
  }

  const columns: ColumnsType<GbMediaServerItem> = [
    { title: '节点名称', dataIndex: 'name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: 'server_id', dataIndex: 'server_id', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '类型', dataIndex: 'type', width: 120,
      render: (v?: string) => (v ? <Tag color="geekblue">{v}</Tag> : '-'),
    },
    {
      title: 'API 地址', dataIndex: 'ip', width: 210,
      render: (_, row) => <span style={{ fontFamily: 'Consolas, Monaco, monospace' }}>{apiAddress(row)}</span>,
    },
    {
      title: '在线状态', dataIndex: 'online', width: 110,
      render: (v?: boolean) => (v ? <Tag color="green">在线</Tag> : <Tag color="red">离线</Tag>),
    },
    { title: '最近心跳', dataIndex: 'last_keepalive_dt', width: 170, render: (v?: string) => fmtDT(v) },
    {
      title: '默认节点', dataIndex: 'default_server', width: 100,
      render: (v?: boolean) => (v ? <Tag color="blue">默认</Tag> : '-'),
    },
    {
      title: '录像保留', dataIndex: 'record_day', width: 100,
      render: (v?: number) => (v != null ? `${v} 天` : '-'),
    },
    {
      title: '操作', key: 'action', width: 500, fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={500}>
          {canMetrics && (
            <Button
              className="e2e-gb-media-server-metrics-btn"
              type="link"
              size="small"
              icon={<BarChartOutlined />}
              onClick={() => openDetail(row, 'metrics')}
            >
              指标
            </Button>
          )}
          {canStreams && (
            <Button
              className="e2e-gb-media-server-streams-btn"
              type="link"
              size="small"
              icon={<VideoCameraOutlined />}
              onClick={() => openDetail(row, 'streams')}
            >
              流会话
            </Button>
          )}
          <Button
            type="link"
            size="small"
            icon={<InfoCircleOutlined />}
            onClick={() => handleInfo(row)}
          >
            节点信息
          </Button>
          {canCheck && (
            <Button
              type="link"
              size="small"
              icon={<CheckCircleOutlined />}
              loading={checkingId === row.id}
              onClick={() => handleCheck(row)}
            >
              在线检查
            </Button>
          )}
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
              编辑
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除该媒体节点?" onConfirm={() => handleDelete(row)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  const loadColumns: ColumnsType<GbMediaServerThreadLoad> = [
    { title: '线程', dataIndex: 'thread_name', render: (v?: string) => v || '-' },
    { title: '负载', dataIndex: 'load', width: 140, render: (v?: number) => (v != null ? v.toFixed(2) : '-') },
  ]

  const host = metrics?.host

  const metricsPane = (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      {metricsError ? (
        <Alert type="error" showIcon message="指标获取失败" description={metricsError} />
      ) : null}
      {metrics?.message ? <Alert type="warning" showIcon message={metrics.message} /> : null}

      <Card
        size="small"
        title="当前指标"
        extra={
          <Space size={8}>
            {metrics?.online ? <Tag color="green">在线</Tag> : <Tag color="red">离线</Tag>}
            <span style={{ color: '#8c8c8c', fontSize: 12 }}>
              流数 {metrics?.stream_count ?? '-'} · 采样 {metrics?.sampled_dt ? fmtDT(metrics.sampled_dt) : '-'}
            </span>
          </Space>
        }
      >
        <Spin spinning={metricsLoading && !metrics}>
          <Row gutter={24}>
            <Col span={8}>
              <div style={{ marginBottom: 4 }}>
                CPU 使用率{host?.cpu_cores ? `（${host.cpu_cores} 核）` : ''}
              </div>
              <Progress percent={toPercent(host?.cpu_percent)} status="active" />
              <div style={{ color: '#8c8c8c', fontSize: 12 }}>
                负载 {round1(host?.load1) ?? '-'} / {round1(host?.load5) ?? '-'} / {round1(host?.load15) ?? '-'}
              </div>
            </Col>
            <Col span={8}>
              <div style={{ marginBottom: 4 }}>内存使用率</div>
              <Progress percent={toPercent(host?.mem_percent)} status="active" strokeColor="#1baf7a" />
              <div style={{ color: '#8c8c8c', fontSize: 12 }}>
                {formatBytes(host?.mem_used)} / {formatBytes(host?.mem_total)}
              </div>
            </Col>
            <Col span={8}>
              <div style={{ marginBottom: 4 }}>磁盘使用率</div>
              <Progress percent={toPercent(host?.disk_percent)} status="active" strokeColor="#eda100" />
              <div style={{ color: '#8c8c8c', fontSize: 12 }}>
                {formatBytes(host?.disk_used)} / {formatBytes(host?.disk_total)}
              </div>
            </Col>
          </Row>

          <Divider style={{ margin: '12px 0' }} />

          <Descriptions column={3} size="small">
            <Descriptions.Item label="网络接收">
              {formatRate(host?.net_rx_rate)}
              <span style={{ color: '#8c8c8c' }}>（累计 {formatBytes(host?.net_rx_bytes)}）</span>
            </Descriptions.Item>
            <Descriptions.Item label="网络发送">
              {formatRate(host?.net_tx_rate)}
              <span style={{ color: '#8c8c8c' }}>（累计 {formatBytes(host?.net_tx_bytes)}）</span>
            </Descriptions.Item>
            <Descriptions.Item label="系统">{host?.os_info || '-'}</Descriptions.Item>
          </Descriptions>
        </Spin>
      </Card>

      <Card
        size="small"
        title="趋势"
        extra={
          <Radio.Group
            size="small"
            optionType="button"
            buttonStyle="solid"
            value={historyRange}
            onChange={(event) => setHistoryRange(event.target.value === '24h' ? '24h' : '1h')}
            options={[
              { label: '最近 1 小时', value: '1h' },
              { label: '最近 24 小时', value: '24h' },
            ]}
          />
        }
      >
        {historyError ? (
          <Alert type="error" showIcon message={historyError} style={{ marginBottom: 12 }} />
        ) : null}
        {historyLoading && historyPoints.length === 0 ? (
          <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
        ) : historyPoints.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无趋势数据" />
        ) : (
          <>
            <div style={{ color: '#52514e', fontSize: 12, marginBottom: 4 }}>CPU / 内存 / 磁盘使用率（%）</div>
            <ReactECharts option={percentTrendOption(historyPoints)} style={{ height: 230 }} notMerge />
            <div style={{ color: '#52514e', fontSize: 12, marginBottom: 4 }}>网络收发速率</div>
            <ReactECharts option={netTrendOption(historyPoints)} style={{ height: 200 }} notMerge />
          </>
        )}
      </Card>
    </Space>
  )

  const streamsPane = (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Alert
        type="info"
        showIcon
        message="每 5 秒自动刷新"
        description="关闭弹窗或切换到「指标」页后停止轮询。"
      />
      {streamsError ? (
        <Alert type="error" showIcon message="流会话获取失败" description={streamsError} />
      ) : null}
      {streamsData?.message ? <Alert type="warning" showIcon message={streamsData.message} /> : null}
      <Table<GbMediaStreamItem>
        className="e2e-gb-media-stream-table"
        rowKey={(row) => `${row.app ?? ''}-${row.stream ?? ''}`}
        size="small"
        loading={streamsLoading}
        columns={streamColumns}
        dataSource={streamsData?.items ?? []}
        pagination={false}
        scroll={{ x: 'max-content', y: 420 }}
        locale={{ emptyText: '当前无流会话' }}
      />
    </Space>
  )

  const detailTabItems = [
    ...(canMetrics
      ? [{
          key: 'metrics',
          label: <span className="e2e-gb-media-server-metrics-tab">指标</span>,
          children: metricsPane,
        }]
      : []),
    ...(canStreams
      ? [{
          key: 'streams',
          label: <span className="e2e-gb-media-server-streams-tab">流会话</span>,
          children: streamsPane,
        }]
      : []),
  ]

  return (
    <PageContainer
      title="媒体节点"
      subTitle="管理 ZLMediaKit 流媒体节点，供国标设备接入、拉流与录像使用"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增节点
          </Button>
        ) : null
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message="单端口收流模式"
        description="本项目默认按单端口收流部署（rtp_enable=false）：设备统一向节点的 RTP 收流端口打流；确需多端口收流时可在编辑弹窗中打开「多端口 RTP 收流」并配置端口段。"
      />

      <Table<GbMediaServerItem>
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 'max-content' }}
        pagination={false}
        locale={{ emptyText: '暂无媒体节点' }}
      />

      <Modal
        className="e2e-gb-media-server-modal"
        title={editing ? '编辑媒体节点' : '新增媒体节点'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => void formRef.current?.submit()}
        confirmLoading={saving}
        destroyOnHidden
        width={900}
      >
        <MediaServerFormContent ref={formRef} editing={editing} onSubmit={handleSave} />
      </Modal>

      <Modal
        className="e2e-gb-media-server-check-modal"
        title="节点连通性检查"
        open={checkOpen}
        onCancel={() => setCheckOpen(false)}
        footer={<Button onClick={() => setCheckOpen(false)}>关闭</Button>}
        width={600}
      >
        <Descriptions column={1} size="small" bordered>
          <Descriptions.Item label="检查结果">
            {checkResult?.online ? <Tag color="green">在线</Tag> : <Tag color="red">离线</Tag>}
          </Descriptions.Item>
          <Descriptions.Item label="节点标识">{checkResult?.media_server_id || '-'}</Descriptions.Item>
          <Descriptions.Item label="版本">{checkResult?.version || '-'}</Descriptions.Item>
          <Descriptions.Item label="HTTP 端口">{checkResult?.http_port ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="RTP / RTCP 端口">
            {checkResult?.rtp_port != null || checkResult?.rtcp_port != null
              ? `${checkResult?.rtp_port ?? '-'} / ${checkResult?.rtcp_port ?? '-'}`
              : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="secret 校验">
            {checkResult?.secret_matched ? <Tag color="green">匹配</Tag> : <Tag color="orange">未匹配</Tag>}
          </Descriptions.Item>
          <Descriptions.Item label="耗时">
            {checkResult?.response_elapsed_ms != null ? `${checkResult.response_elapsed_ms} ms` : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="消息">{checkResult?.message || '-'}</Descriptions.Item>
        </Descriptions>
      </Modal>

      <Modal
        className="e2e-gb-media-server-info-modal"
        title={infoNodeName ? `节点信息 - ${infoNodeName}` : '节点信息'}
        open={infoOpen}
        onCancel={() => setInfoOpen(false)}
        footer={<Button onClick={() => setInfoOpen(false)}>关闭</Button>}
        width={680}
      >
        <Spin spinning={infoLoading}>
          <Descriptions column={2} size="small" bordered style={{ marginBottom: 16 }}>
            <Descriptions.Item label="在线状态">
              {infoResult?.online ? <Tag color="green">在线</Tag> : <Tag color="red">离线</Tag>}
            </Descriptions.Item>
            <Descriptions.Item label="版本">{infoResult?.version || '-'}</Descriptions.Item>
            <Descriptions.Item label="活跃流数">{infoResult?.active_streams ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="线程数">{infoResult?.thread_count ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="平均负载">
              {infoResult?.avg_load != null ? infoResult.avg_load.toFixed(2) : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="最大负载">
              {infoResult?.max_load != null ? infoResult.max_load.toFixed(2) : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="消息" span={2}>{infoResult?.message || '-'}</Descriptions.Item>
          </Descriptions>
          <Table<GbMediaServerThreadLoad>
            rowKey={(row) => row.thread_name ?? String(row.load ?? '')}
            size="small"
            columns={loadColumns}
            dataSource={infoResult?.loads ?? []}
            pagination={false}
            scroll={{ y: 240 }}
            locale={{ emptyText: '暂无线程负载数据' }}
          />
        </Spin>
      </Modal>

      {/* 节点指标 / 流会话：打开时轮询，关闭即停止 */}
      <Modal
        className="e2e-gb-media-server-detail-modal"
        title={
          detail
            ? `${detail.tab === 'metrics' ? '节点指标' : '流会话'} - ${detail.server.name || detail.server.server_id || ''}`
            : '节点详情'
        }
        open={detail !== null}
        onCancel={closeDetail}
        footer={<Button onClick={closeDetail}>关闭</Button>}
        width={1040}
        destroyOnHidden
      >
        <Tabs
          activeKey={detail?.tab ?? 'metrics'}
          onChange={switchDetailTab}
          items={detailTabItems}
        />
      </Modal>
    </PageContainer>
  )
}
