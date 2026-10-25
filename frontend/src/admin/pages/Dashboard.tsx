import { useCallback, useEffect, useMemo, useState } from 'react'
import { Alert, Button, Card, Col, Row, Space, Spin, Statistic, Tag } from 'antd'
import {
  AlertOutlined,
  CheckCircleOutlined,
  CloudServerOutlined,
  DashboardOutlined,
  DisconnectOutlined,
  ReloadOutlined,
  VideoCameraOutlined,
} from '@ant-design/icons'
import ReactECharts from 'echarts-for-react'
import type { EChartsOption } from 'echarts'
import { useNavigate } from 'react-router-dom'
import { StatsController } from '../api/generated/StatsController'
import type { DashboardData } from '../api/generated/StatsController'
import { GbMediaServerController } from '../api/generated/GbMediaServerController'
import { GbOpsController } from '../api/generated/GbOpsController'
import type { GbMediaServerItem } from '../api/generated/definitions/GbMediaServerItem'
import type { GbMediaStreamListResp } from '../api/generated/definitions/GbMediaStreamListResp'
import PageContainer from '../components/PageContainer'
import EmptyState from '../components/EmptyState'
import { useAuthStore } from '../store/authStore'

/* ── 图表配色 ──────────────────────────────────────────────────────────
   分类色板按固定顺序取用（不循环）；单序列图表只用 slot 1。
   ──────────────────────────────────────────────────────────────────── */
const SERIES_COLORS = [
  '#2a78d6', // 1 blue
  '#eb6834', // 2 orange
  '#1baf7a', // 3 aqua
  '#eda100', // 4 yellow
  '#e87ba4', // 5 magenta
  '#008300', // 6 green
  '#4a3aa7', // 7 violet
  '#e34948', // 8 red
]
const SURFACE = '#ffffff'
const TEXT_SECONDARY = '#52514e'
const GRID_LINE = '#ececec'

interface ChartPoint {
  name: string
  value: number
}

/** 分类超过 8 项时折叠尾部为「其他」，避免色板被循环复用 */
function capCategories(items: ChartPoint[], max = 8): ChartPoint[] {
  if (items.length <= max) return items
  const sorted = [...items].sort((a, b) => b.value - a.value)
  const head = sorted.slice(0, max - 1)
  const restValue = sorted.slice(max - 1).reduce((sum, item) => sum + item.value, 0)
  return [...head, { name: '其他', value: restValue }]
}

function toPairs(items?: { name?: string; count?: number }[]): ChartPoint[] {
  if (!Array.isArray(items)) return []
  return items.map((item) => ({ name: item.name ?? '-', value: Number(item.count ?? 0) }))
}

/** 环形图：分类色板 + 2px 表面间隙 + 直接标签（保证不依赖颜色区分） */
function pieOption(data: ChartPoint[], seriesName: string): EChartsOption {
  return {
    color: SERIES_COLORS,
    tooltip: { trigger: 'item', formatter: '{b}<br/>{c}（{d}%）' },
    legend: {
      bottom: 0,
      icon: 'circle',
      itemWidth: 8,
      itemHeight: 8,
      textStyle: { color: TEXT_SECONDARY, fontSize: 12 },
    },
    series: [
      {
        name: seriesName,
        type: 'pie',
        radius: ['44%', '66%'],
        center: ['50%', '44%'],
        avoidLabelOverlap: true,
        itemStyle: { borderColor: SURFACE, borderWidth: 2 },
        label: { show: true, color: TEXT_SECONDARY, fontSize: 12, formatter: '{b} {d}%' },
        labelLine: { length: 8, length2: 8, lineStyle: { color: GRID_LINE } },
        data: capCategories(data),
      },
    ],
  }
}

/** 柱状图：单序列（标题已说明），4px 圆角数据端 */
function barOption(data: ChartPoint[], seriesName: string): EChartsOption {
  return {
    color: [SERIES_COLORS[0]],
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 8, right: 16, top: 16, bottom: 8, containLabel: true },
    xAxis: {
      type: 'category',
      data: data.map((item) => item.name),
      axisLine: { lineStyle: { color: GRID_LINE } },
      axisTick: { show: false },
      axisLabel: {
        color: TEXT_SECONDARY,
        fontSize: 12,
        interval: 0,
        rotate: data.length > 6 ? 30 : 0,
      },
    },
    yAxis: {
      type: 'value',
      splitLine: { lineStyle: { color: GRID_LINE } },
      axisLabel: { color: TEXT_SECONDARY, fontSize: 12 },
    },
    series: [
      {
        name: seriesName,
        type: 'bar',
        barMaxWidth: 28,
        itemStyle: { borderRadius: [4, 4, 0, 0] },
        data: data.map((item) => item.value),
      },
    ],
  }
}

/** 折线图：单序列，2px 线宽，8px 圆点，十字准线 tooltip */
function lineOption(points: { date?: string; count?: number }[], seriesName: string): EChartsOption {
  return {
    color: [SERIES_COLORS[0]],
    tooltip: { trigger: 'axis', axisPointer: { type: 'line', lineStyle: { color: GRID_LINE } } },
    grid: { left: 8, right: 16, top: 16, bottom: 8, containLabel: true },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: points.map((point) => point.date ?? ''),
      axisLine: { lineStyle: { color: GRID_LINE } },
      axisTick: { show: false },
      axisLabel: { color: TEXT_SECONDARY, fontSize: 12 },
    },
    yAxis: {
      type: 'value',
      splitLine: { lineStyle: { color: GRID_LINE } },
      axisLabel: { color: TEXT_SECONDARY, fontSize: 12 },
    },
    series: [
      {
        name: seriesName,
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 8,
        lineStyle: { width: 2 },
        areaStyle: { opacity: 0.08 },
        data: points.map((point) => Number(point.count ?? 0)),
      },
    ],
  }
}

function ChartCard({
  title,
  option,
  empty,
  height = 280,
}: {
  title: string
  option: EChartsOption
  empty: boolean
  height?: number
}) {
  return (
    <Card size="small" title={title} style={{ height: '100%' }}>
      {empty ? (
        <EmptyState description="暂无数据" />
      ) : (
        <ReactECharts option={option} style={{ height }} opts={{ renderer: 'canvas' }} notMerge />
      )}
    </Card>
  )
}

/** 首页看板：`GET /web/stats/dashboard` */
export default function Dashboard() {
  const navigate = useNavigate()
  const permCodes = useAuthStore((s) => s.permCodes)
  // 无媒体节点流会话权限时隐藏该卡片，避免无权限账号触发 403
  const canViewStreams = permCodes.includes('gb:media-server:streams')

  const [data, setData] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState(true)

  /* ── 有流通道（默认媒体节点的当前流会话） ── */
  const [streamServer, setStreamServer] = useState<GbMediaServerItem | null>(null)
  const [streamData, setStreamData] = useState<GbMediaStreamListResp | null>(null)
  const [streamLoading, setStreamLoading] = useState(false)
  const [streamError, setStreamError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await StatsController.dashboard()
      if (res?.code === 200 && res.data) {
        setData(res.data)
      }
    } catch {
      /* 响应拦截器已统一提示 */
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  /** 汇总默认媒体节点（无默认时取第一个在线节点）的当前流会话 */
  const loadStreams = useCallback(async () => {
    setStreamLoading(true)
    try {
      const servers = await GbMediaServerController.listGbMediaServers()
      const list = servers.data ?? []
      const preferred = list.find((item) => item.default_server)
        ?? list.find((item) => item.online)
        ?? list[0]
      if (!preferred?.id) {
        setStreamServer(null)
        setStreamData(null)
        setStreamError('')
        return
      }
      setStreamServer(preferred)
      // 节点明确离线时不请求流会话，避免 409 触发全局错误提示
      if (preferred.online === false) {
        setStreamData(null)
        setStreamError('默认媒体节点离线，暂无流会话')
        return
      }
      const res = await GbOpsController.listGbMediaServerStreams(preferred.id)
      setStreamData(res.data ?? null)
      setStreamError('')
    } catch {
      setStreamError('获取有流通道失败（媒体节点可能不可达）')
      setStreamData(null)
    } finally {
      setStreamLoading(false)
    }
  }, [])

  useEffect(() => {
    if (canViewStreams) void loadStreams()
  }, [canViewStreams, loadStreams])

  const deviceStat = data?.device_stat
  const riskTypePairs = useMemo(() => toPairs(data?.risk_type_stats), [data])
  const regionPairs = useMemo(() => toPairs(data?.region_risk_stats), [data])
  const riskTrend = useMemo(() => data?.risk_trend ?? [], [data])
  const alarmTrend = useMemo(() => data?.alarm_trend ?? [], [data])

  const handleStat = data?.risk_handle_stats
  const handlePairs = useMemo<ChartPoint[]>(
    () => [
      { name: '待处理', value: Number(handleStat?.pending ?? 0) },
      { name: '已处理', value: Number(handleStat?.handled ?? 0) },
    ],
    [handleStat]
  )

  if (loading && !data) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', padding: '80px 0' }}>
        <Spin size="large" tip="加载中..." />
      </div>
    )
  }

  return (
    <PageContainer title="首页看板" subTitle="风险、设备与告警总览">
      {/* 预警 / 处理统计（今日 / 本周 / 本月） */}
      <div className="dashboard-stat-row">
        <Card size="small">
          <Statistic
            title="今日预警"
            value={data?.today_risk_count ?? 0}
            prefix={<AlertOutlined style={{ color: SERIES_COLORS[1] }} />}
            suffix="起"
          />
          <div style={{ color: TEXT_SECONDARY, marginTop: 4 }}>
            告警 {data?.today_alarm_count ?? 0} 起 · 处理 {data?.today_handled_count ?? 0} 起
          </div>
        </Card>
        <Card size="small">
          <Statistic
            title="今日处理"
            value={data?.today_handled_count ?? 0}
            prefix={<CheckCircleOutlined style={{ color: SERIES_COLORS[2] }} />}
            suffix="起"
          />
          <div style={{ color: TEXT_SECONDARY, marginTop: 4 }}>
            风险 {data?.today_risk_handled_count ?? 0} · 告警 {data?.today_alarm_handled_count ?? 0}
          </div>
        </Card>
        <Card size="small">
          <Statistic
            title="本周预警"
            value={data?.week_risk_count ?? 0}
            prefix={<AlertOutlined style={{ color: SERIES_COLORS[1] }} />}
            suffix="起"
          />
          <div style={{ color: TEXT_SECONDARY, marginTop: 4 }}>
            告警 {data?.week_alarm_count ?? 0} 起 · 处理 {data?.week_handled_count ?? 0} 起
          </div>
        </Card>
        <Card size="small">
          <Statistic
            title="本周处理"
            value={data?.week_handled_count ?? 0}
            prefix={<CheckCircleOutlined style={{ color: SERIES_COLORS[2] }} />}
            suffix="起"
          />
          <div style={{ color: TEXT_SECONDARY, marginTop: 4 }}>
            风险 {data?.week_risk_handled_count ?? 0} · 告警 {data?.week_alarm_handled_count ?? 0}
          </div>
        </Card>
        <Card size="small">
          <Statistic
            title="本月预警"
            value={data?.month_risk_count ?? 0}
            prefix={<AlertOutlined style={{ color: SERIES_COLORS[1] }} />}
            suffix="起"
          />
          <div style={{ color: TEXT_SECONDARY, marginTop: 4 }}>
            告警 {data?.month_alarm_count ?? 0} 起 · 处理 {data?.month_handled_count ?? 0} 起
          </div>
        </Card>
        <Card size="small">
          <Statistic
            title="本月处理"
            value={data?.month_handled_count ?? 0}
            prefix={<CheckCircleOutlined style={{ color: SERIES_COLORS[2] }} />}
            suffix="起"
          />
          <div style={{ color: TEXT_SECONDARY, marginTop: 4 }}>
            风险 {data?.month_risk_handled_count ?? 0} · 告警 {data?.month_alarm_handled_count ?? 0}
          </div>
        </Card>
      </div>

      {/* 设备统计 */}
      <Card size="small" title="设备统计">
        <Row gutter={16}>
          <Col span={8}>
            <Statistic
              title="设备总数"
              value={deviceStat?.total ?? 0}
              prefix={<CloudServerOutlined style={{ color: SERIES_COLORS[0] }} />}
              suffix="台"
            />
          </Col>
          <Col span={8}>
            <Statistic
              title="在线设备"
              value={deviceStat?.online ?? 0}
              prefix={<DashboardOutlined style={{ color: SERIES_COLORS[2] }} />}
              suffix="台"
            />
          </Col>
          <Col span={8}>
            <Statistic
              title="离线设备"
              value={deviceStat?.offline ?? 0}
              prefix={<DisconnectOutlined style={{ color: SERIES_COLORS[7] }} />}
              suffix="台"
            />
          </Col>
        </Row>
      </Card>

      {/* 有流通道：默认媒体节点的当前流会话（前 5 条） */}
      {canViewStreams ? (
      <Card
        className="e2e-dashboard-streams-card"
        size="small"
        title="有流通道"
        style={{ marginTop: 12 }}
        extra={
          <Space size={4}>
            <Button
              type="link"
              size="small"
              icon={<ReloadOutlined />}
              loading={streamLoading}
              onClick={() => void loadStreams()}
            >
              刷新
            </Button>
            <Button
              type="link"
              size="small"
              disabled={!streamServer?.id}
              onClick={() => {
                const id = streamServer?.id
                navigate(id ? `/gb/ops/media-server?streamServerId=${id}` : '/gb/ops/media-server')
              }}
            >
              查看全部
            </Button>
          </Space>
        }
      >
        {streamError ? (
          <Alert type="warning" showIcon message={streamError} style={{ marginBottom: 12 }} />
        ) : null}
        <Spin spinning={streamLoading}>
          <Row gutter={16}>
            <Col xs={24} sm={8}>
              <Statistic
                title="当前流数量"
                value={streamData?.total ?? streamData?.items?.length ?? 0}
                suffix="路"
                prefix={<VideoCameraOutlined style={{ color: SERIES_COLORS[0] }} />}
              />
              <div style={{ color: TEXT_SECONDARY, marginTop: 4, fontSize: 12 }}>
                节点：{streamServer?.name || '未配置默认节点'}
              </div>
            </Col>
            <Col xs={24} sm={16}>
              {(streamData?.items?.length ?? 0) === 0 ? (
                <EmptyState description="当前没有流会话" />
              ) : (
                <div>
                  {(streamData?.items ?? []).slice(0, 5).map((item) => (
                    <div
                      key={`${item.app ?? ''}-${item.stream ?? ''}`}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        gap: 8,
                        padding: '4px 0',
                        borderBottom: '1px solid #f0f0f0',
                      }}
                    >
                      <span style={{ minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        <span style={{ fontFamily: 'Consolas, Monaco, monospace' }}>
                          {item.app || '-'}/{item.stream || '-'}
                        </span>
                        <Tag style={{ marginLeft: 8 }}>{item.schema || '-'}</Tag>
                        {item.codec ? <Tag>{item.codec}</Tag> : null}
                      </span>
                      <span style={{ color: TEXT_SECONDARY, flex: '0 0 auto', fontSize: 12 }}>
                        {item.width && item.height ? `${item.width}×${item.height}` : '-'}
                        {' · '}观看 {item.reader_count ?? 0}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </Col>
          </Row>
        </Spin>
      </Card>
      ) : null}

      {/* 分析图表 */}
      <Row gutter={[12, 12]}>
        <Col xs={24} lg={12}>
          <ChartCard
            title="风险类型分析（近 30 天）"
            empty={riskTypePairs.length === 0}
            option={pieOption(riskTypePairs, '风险类型')}
          />
        </Col>
        <Col xs={24} lg={12}>
          <ChartCard
            title="风险处理统计（近 30 天）"
            empty={handlePairs.every((item) => item.value === 0)}
            option={pieOption(handlePairs, '风险处理')}
          />
        </Col>
        <Col xs={24} lg={12}>
          <ChartCard
            title="区域风险统计（近 30 天）"
            empty={regionPairs.length === 0}
            option={barOption(regionPairs, '风险数')}
          />
        </Col>
        <Col xs={24} lg={12}>
          <ChartCard
            title="风险分析（近 7 天）"
            empty={riskTrend.length === 0}
            option={lineOption(riskTrend, '风险数')}
          />
        </Col>
        <Col xs={24}>
          <ChartCard
            title="告警分析（近 7 天）"
            empty={alarmTrend.length === 0}
            option={lineOption(alarmTrend, '告警数')}
            height={260}
          />
        </Col>
      </Row>

      {!loading && !data ? (
        <Card size="small">
          <EmptyState
            description="看板数据加载失败"
            extra={<span style={{ color: TEXT_SECONDARY }}>请确认后端服务已启动后刷新页面</span>}
          />
        </Card>
      ) : null}
    </PageContainer>
  )
}
