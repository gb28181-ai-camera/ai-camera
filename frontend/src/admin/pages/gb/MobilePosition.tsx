import React, {
  forwardRef, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState,
} from 'react'
import {
  Button, Card, DatePicker, Empty, Form, InputNumber, message, Modal, Space, Table, Tag, Typography,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { AimOutlined, ReloadOutlined, SearchOutlined, SendOutlined } from '@ant-design/icons'
import type { Dayjs } from 'dayjs'
import axios from 'axios'
import ReactECharts from 'echarts-for-react'
import type { EChartsOption } from 'echarts'
import PageContainer from '../../components/PageContainer'
import OptionSelect from '../../components/OptionSelect'
import ActionButtons from '../../components/ActionButtons'
import { GbPositionController } from '../../api/generated/GbPositionController'
import { GbChannelController } from '../../api/generated/GbChannelController'
import type { GbPositionResp } from '../../api/generated/definitions/GbPositionResp'
import { useAuthStore } from '../../store/authStore'
import { formatDT } from '../../types/business'

const { RangePicker } = DatePicker

/** 请求失败提示：优先展示后端返回的 message。 */
function apiErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError<{ message?: string }>(err)) {
    return err.response?.data?.message || err.message || fallback
  }
  return err instanceof Error ? err.message : fallback
}

function fmtCoord(value?: number): string {
  if (value === undefined || value === null || Number.isNaN(Number(value))) return '-'
  return Number(value).toFixed(6)
}

function fmtMeasure(value?: number, suffix = ''): string {
  if (value === undefined || value === null || Number.isNaN(Number(value))) return '-'
  return `${Number(value).toFixed(1)}${suffix}`
}

interface HistoryQuery {
  channelId?: number
  start?: string
  end?: string
}

/**
 * 从位置记录中提取可绘制的经纬度。
 *
 * GB28181 设备未定位时会上报 0/0，这不是有效坐标；
 * 经纬度任一非数字时同样跳过，避免把无效点画进坐标系把范围撑坏。
 */
function toCoord(value?: number): number | null {
  if (value === undefined || value === null) return null
  const num = Number(value)
  return Number.isFinite(num) ? num : null
}

function coordOf(row: GbPositionResp): [number, number] | null {
  const lng = toCoord(row.longitude)
  const lat = toCoord(row.latitude)
  if (lng === null || lat === null) return null
  if (lng === 0 && lat === 0) return null
  return [lng, lat]
}

/** 轨迹按上报时间升序，保证折线顺序与运动方向一致。 */
function sortByReportTime(rows: GbPositionResp[]): GbPositionResp[] {
  return [...rows].sort((a, b) => {
    const ta = a.report_time ? new Date(a.report_time).getTime() : 0
    const tb = b.report_time ? new Date(b.report_time).getTime() : 0
    return ta - tb
  })
}

/** 统一坐标轴样式：scale=true 让范围贴合数据，不强制从 0 开始。 */
function coordAxisOption(name: string, nameGap: number) {
  return {
    type: 'value' as const,
    name,
    nameLocation: 'middle' as const,
    nameGap,
    scale: true,
    axisLabel: { formatter: (value: string | number) => Number(value).toFixed(4) },
  }
}

interface SubscribeFormValues {
  channel_id: number
  interval: number
}

/** 通道下拉框（数据源：`GET /web/gb/channels/options`）。 */
const channelFetcher = () => GbChannelController.gbChannelOptions()

interface SubscribeFormHandle {
  submit: () => Promise<void>
}

interface SubscribeFormContentProps {
  defaultChannelId?: number
  onSubmit: (values: SubscribeFormValues) => Promise<void>
}

/**
 * 位置订阅表单内容。
 *
 * 表单实例随 Modal children 一起挂载，父组件不再在弹窗打开前
 * resetFields/setFieldsValue（那会触发 useForm 未连接告警）。
 */
const SubscribeFormContent = forwardRef<SubscribeFormHandle, SubscribeFormContentProps>(
  function SubscribeFormContent({ defaultChannelId, onSubmit }, ref) {
    const [form] = Form.useForm<SubscribeFormValues>()

    useImperativeHandle(ref, () => ({
      async submit() {
        let values: SubscribeFormValues
        try {
          values = await form.validateFields()
        } catch {
          return
        }
        await onSubmit(values)
      },
    }), [form, onSubmit])

    return (
      <Form
        form={form}
        layout="vertical"
        initialValues={{ channel_id: defaultChannelId, interval: 60 }}
      >
        <Form.Item
          name="channel_id"
          label="通道"
          rules={[{ required: true, message: '请选择通道' }]}
        >
          <OptionSelect placeholder="请选择要订阅位置的通道" fetcher={channelFetcher} />
        </Form.Item>
        <Form.Item
          name="interval"
          label="订阅间隔（秒）"
          rules={[{ required: true, message: '请输入订阅间隔' }]}
          extra="后端缺省为 60 秒；设备会按该间隔上报位置。"
        >
          <InputNumber min={1} max={3600} style={{ width: '100%' }} placeholder="60" />
        </Form.Item>
      </Form>
    )
  },
)

/**
 * 移动位置：国标移动设备（车载 / 单兵）上报的位置。
 *
 * - 上半部分：最新位置的 ECharts 散点图（x=经度，y=纬度）+ 通道标签；
 * - 下半部分：选中通道的历史轨迹折线（按上报时间升序）。
 *
 * 刻意不引入地图库：没有瓦片底图与投影换算，直接用经纬度当平面坐标，
 * 图上明确标注「无底图，仅坐标分布」；坐标轴 scale=true 让范围自适应数据。
 * 表格仍然保留，坐标精确值以表格为准。
 */
const MobilePosition: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canSubscribe = permCodes.includes('gb:position:subscribe')

  /* ── 最新位置 ── */
  const [realtime, setRealtime] = useState<GbPositionResp[]>([])
  const [realtimeLoading, setRealtimeLoading] = useState(false)

  const fetchRealtime = useCallback(async () => {
    setRealtimeLoading(true)
    try {
      const res = await GbPositionController.listGbPositionsRealtime()
      setRealtime(res.data ?? [])
    } catch (err) {
      message.error(apiErrorMessage(err, '获取最新位置失败'))
    } finally {
      setRealtimeLoading(false)
    }
  }, [])

  useEffect(() => { void fetchRealtime() }, [fetchRealtime])

  /* ── 历史轨迹 ── */
  const [historyChannelId, setHistoryChannelId] = useState<number | undefined>()
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null)
  const [historyQuery, setHistoryQuery] = useState<HistoryQuery>({})
  const [history, setHistory] = useState<GbPositionResp[]>([])
  const [historyTotal, setHistoryTotal] = useState(0)
  const [historyPage, setHistoryPage] = useState(1)
  const [historyPageSize, setHistoryPageSize] = useState(20)
  const [historyLoading, setHistoryLoading] = useState(false)

  /**
   * 轨迹图数据：与表格分页解耦，一次取最多 500 个点。
   *
   * 表格按 20 条分页，若直接用当前页画折线，翻页时轨迹会「跳段」；
   * 因此单独拉一次大页数据供图表使用，表格仍保持分页。
   */
  const [track, setTrack] = useState<GbPositionResp[]>([])
  const [trackLoading, setTrackLoading] = useState(false)

  useEffect(() => {
    if (!historyQuery.channelId) {
      setTrack([])
      return undefined
    }
    let cancelled = false
    setTrackLoading(true)
    GbPositionController.pageGbPositionHistory(
      1,
      500,
      undefined,
      historyQuery.channelId,
      undefined,
      historyQuery.start,
      historyQuery.end,
    )
      .then((res) => {
        if (!cancelled) setTrack(sortByReportTime(res.data?.items ?? []))
      })
      .catch((err) => {
        if (!cancelled) {
          setTrack([])
          message.error(apiErrorMessage(err, '查询轨迹图数据失败'))
        }
      })
      .finally(() => {
        if (!cancelled) setTrackLoading(false)
      })
    return () => { cancelled = true }
  }, [historyQuery])

  const fetchHistory = useCallback(async () => {
    if (!historyQuery.channelId) {
      setHistory([])
      setHistoryTotal(0)
      return
    }
    setHistoryLoading(true)
    try {
      const res = await GbPositionController.pageGbPositionHistory(
        historyPage,
        historyPageSize,
        undefined,
        historyQuery.channelId,
        undefined,
        historyQuery.start,
        historyQuery.end,
      )
      setHistory(res.data?.items ?? [])
      setHistoryTotal(Number(res.data?.total ?? 0))
    } catch (err) {
      message.error(apiErrorMessage(err, '查询历史轨迹失败'))
    } finally {
      setHistoryLoading(false)
    }
  }, [historyQuery, historyPage, historyPageSize])

  useEffect(() => { void fetchHistory() }, [fetchHistory])

  const queryHistory = () => {
    if (!historyChannelId) {
      message.warning('请先选择通道')
      return
    }
    setHistoryPage(1)
    setHistoryQuery({
      channelId: historyChannelId,
      start: range?.[0]?.format('YYYY-MM-DD HH:mm:ss'),
      end: range?.[1]?.format('YYYY-MM-DD HH:mm:ss'),
    })
  }

  const viewChannelHistory = (row: GbPositionResp) => {
    if (!row.channel_id) return
    setHistoryChannelId(row.channel_id)
    setHistoryPage(1)
    setHistoryQuery({ channelId: row.channel_id })
  }

  /* ── 位置订阅 ── */
  const [subscribeOpen, setSubscribeOpen] = useState(false)
  const [subscribeChannelId, setSubscribeChannelId] = useState<number | undefined>()
  const [subscribing, setSubscribing] = useState(false)
  const subscribeFormRef = useRef<SubscribeFormHandle>(null)

  const openSubscribe = (channelId?: number) => {
    setSubscribeChannelId(channelId)
    setSubscribeOpen(true)
  }

  const submitSubscribe = async (values: SubscribeFormValues) => {
    setSubscribing(true)
    try {
      await GbPositionController.subscribeGbPosition({
        channel_id: values.channel_id,
        interval: values.interval,
      })
      message.success('位置订阅指令已发送')
      setSubscribeOpen(false)
    } catch (err) {
      message.error(apiErrorMessage(err, '订阅位置失败'))
    } finally {
      setSubscribing(false)
    }
  }

  /** 最新位置散点图：每个通道一个点，标签显示通道名称。 */
  const realtimeOption = useMemo<EChartsOption>(() => {
    const points = realtime.flatMap((row) => {
      const pair = coordOf(row)
      if (!pair) return []
      const label = row.channel_name || row.gb_channel_id || `通道 ${row.channel_id ?? '-'}`
      return [{ name: label, value: pair }]
    })
    return {
      title: {
        text: '最新位置坐标分布',
        subtext: '无底图，仅坐标分布（x=经度，y=纬度）',
        left: 'center',
        textStyle: { fontSize: 14 },
        subtextStyle: { color: '#8c8c8c', fontSize: 12 },
      },
      tooltip: { trigger: 'item' },
      grid: { left: 70, right: 40, top: 70, bottom: 60 },
      xAxis: coordAxisOption('经度', 28),
      yAxis: coordAxisOption('纬度', 44),
      series: [
        {
          type: 'scatter',
          name: '最新位置',
          data: points,
          symbolSize: 14,
          itemStyle: { color: '#1677ff' },
          // {b} 是数据项 name（通道名称），比回调更易维护。
          label: { show: true, position: 'top', formatter: '{b}', fontSize: 12 },
          emphasis: { scale: true },
        },
      ],
    }
  }, [realtime])

  /** 历史轨迹折线：按上报时间升序连接同一通道的坐标点。 */
  const historyOption = useMemo<EChartsOption>(() => {
    const points = track.flatMap((row) => {
      const pair = coordOf(row)
      if (!pair) return []
      const time = row.report_time ? formatDT(row.report_time) : ''
      return [{ name: time, value: pair }]
    })
    return {
      title: {
        text: '历史轨迹',
        subtext: '无底图，仅坐标轨迹（x=经度，y=纬度）',
        left: 'center',
        textStyle: { fontSize: 14 },
        subtextStyle: { color: '#8c8c8c', fontSize: 12 },
      },
      tooltip: { trigger: 'item' },
      grid: { left: 70, right: 40, top: 70, bottom: 60 },
      xAxis: coordAxisOption('经度', 28),
      yAxis: coordAxisOption('纬度', 44),
      series: [
        {
          type: 'line',
          name: '历史轨迹',
          data: points,
          symbolSize: 6,
          smooth: false,
          lineStyle: { width: 2, color: '#fa8c16' },
          itemStyle: { color: '#fa8c16' },
        },
      ],
    }
  }, [track])

  const realtimePointCount = useMemo(
    () => realtime.filter((row) => coordOf(row) !== null).length,
    [realtime],
  )
  const trackPointCount = useMemo(
    () => track.filter((row) => coordOf(row) !== null).length,
    [track],
  )

  const positionColumns: ColumnsType<GbPositionResp> = [
    { title: '设备名称', dataIndex: 'device_name', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '通道名称', dataIndex: 'channel_name', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '国标通道编码', dataIndex: 'gb_channel_id', width: 170, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '上报时间', dataIndex: 'report_time', width: 170, render: (v?: string) => formatDT(v) },
    { title: '经度', dataIndex: 'longitude', width: 130, render: (v?: number) => fmtCoord(v) },
    { title: '纬度', dataIndex: 'latitude', width: 130, render: (v?: number) => fmtCoord(v) },
    { title: '海拔', dataIndex: 'altitude', width: 100, render: (v?: number) => fmtMeasure(v, ' m') },
    { title: '速度', dataIndex: 'speed', width: 110, render: (v?: number) => fmtMeasure(v, ' km/h') },
    { title: '方向', dataIndex: 'direction', width: 90, render: (v?: number) => fmtMeasure(v, '°') },
  ]

  const realtimeColumns: ColumnsType<GbPositionResp> = [
    ...positionColumns,
    {
      title: '操作',
      key: 'action',
      width: 190,
      fixed: 'right',
      render: (_: unknown, row) => (
        <ActionButtons maxWidth={190}>
          <Button
            type="link"
            size="small"
            icon={<AimOutlined />}
            onClick={() => viewChannelHistory(row)}
          >
            历史轨迹
          </Button>
          {canSubscribe && (
            <Button
              type="link"
              size="small"
              icon={<SendOutlined />}
              onClick={() => openSubscribe(row.channel_id)}
            >
              订阅
            </Button>
          )}
        </ActionButtons>
      ),
    },
  ]

  const historyColumns: ColumnsType<GbPositionResp> = positionColumns

  return (
    <PageContainer
      title="移动位置"
      subTitle="国标移动设备的位置上报、最新位置与历史轨迹"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void fetchRealtime()} loading={realtimeLoading}>
            刷新最新位置
          </Button>
          {canSubscribe && (
            <Button type="primary" icon={<SendOutlined />} onClick={() => openSubscribe(historyChannelId)}>
              订阅位置
            </Button>
          )}
        </Space>
      }
    >
      <Card
        size="small"
        title="最新位置"
        style={{ marginBottom: 16 }}
        extra={<Tag color="blue">{realtime.length} 个通道</Tag>}
      >
        {realtimePointCount > 0 ? (
          <ReactECharts
            className="e2e-gb-position-realtime-chart"
            option={realtimeOption}
            style={{ height: 320, marginBottom: 12 }}
            opts={{ renderer: 'canvas' }}
            notMerge
          />
        ) : (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="暂无有效经纬度，无法绘制坐标分布"
            style={{ margin: '24px 0' }}
          />
        )}
        <Table<GbPositionResp>
          rowKey={(row) => String(row.id ?? `${row.channel_id ?? ''}-${row.report_time ?? ''}`)}
          size="small"
          loading={realtimeLoading}
          columns={realtimeColumns}
          dataSource={realtime}
          scroll={{ x: 'max-content' }}
          pagination={{
            pageSize: 10,
            showSizeChanger: false,
            showTotal: (t) => `共 ${t} 条`,
          }}
        />
      </Card>

      <Card size="small" title="历史轨迹">
        <Space wrap style={{ marginBottom: 16 }}>
          <OptionSelect
            style={{ width: 260 }}
            placeholder="请选择通道"
            value={historyChannelId}
            onChange={(value: unknown) => {
              const next = typeof value === 'number' ? value : value ? Number(value) : undefined
              setHistoryChannelId(next !== undefined && Number.isFinite(next) ? next : undefined)
            }}
            fetcher={channelFetcher}
          />
          <RangePicker
            showTime
            value={range}
            placeholder={['上报时间起', '上报时间止']}
            onChange={(dates) => setRange(dates)}
          />
          <Button type="primary" icon={<SearchOutlined />} onClick={queryHistory} loading={historyLoading}>
            查询轨迹
          </Button>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => { setRange(null); setHistoryPage(1); setHistoryQuery({}) }}
          >
            重置
          </Button>
        </Space>

        <div style={{ marginBottom: 8 }}>
          <Typography.Text type="secondary">
            轨迹图最多展示最近 500 个坐标点；无底图，仅坐标分布（x=经度，y=纬度）。
          </Typography.Text>
        </div>
        {!historyQuery.channelId ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="请选择通道并查询历史轨迹"
            style={{ margin: '24px 0' }}
          />
        ) : trackLoading ? (
          <div style={{ height: 320, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Typography.Text type="secondary">轨迹加载中…</Typography.Text>
          </div>
        ) : trackPointCount > 0 ? (
          <ReactECharts
            className="e2e-gb-position-history-chart"
            option={historyOption}
            style={{ height: 320, marginBottom: 12 }}
            opts={{ renderer: 'canvas' }}
            notMerge
          />
        ) : (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="所选通道在该时间段内暂无有效坐标"
            style={{ margin: '24px 0' }}
          />
        )}

        <Table<GbPositionResp>
          rowKey={(row) => String(row.id ?? `${row.channel_id ?? ''}-${row.report_time ?? ''}`)}
          size="small"
          loading={historyLoading}
          columns={historyColumns}
          dataSource={history}
          scroll={{ x: 'max-content' }}
          locale={{ emptyText: historyQuery.channelId ? '暂无轨迹数据' : '请选择通道并查询历史轨迹' }}
          pagination={{
            current: historyPage,
            pageSize: historyPageSize,
            total: historyTotal,
            showSizeChanger: true,
            showTotal: (t) => `共 ${t} 条`,
            onChange: (p, ps) => { setHistoryPage(p); setHistoryPageSize(ps) },
          }}
        />
      </Card>

      <Modal
        className="e2e-gb-position-subscribe-modal"
        title="订阅移动位置"
        open={subscribeOpen}
        onCancel={() => setSubscribeOpen(false)}
        onOk={() => void subscribeFormRef.current?.submit()}
        confirmLoading={subscribing}
        destroyOnHidden
        width={560}
      >
        <SubscribeFormContent
          ref={subscribeFormRef}
          defaultChannelId={subscribeChannelId}
          onSubmit={submitSubscribe}
        />
      </Modal>
    </PageContainer>
  )
}

export default MobilePosition
