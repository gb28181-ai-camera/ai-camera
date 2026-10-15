/**
 * 风险 / 告警 / 巡检 / 设备监控 / 统计分析页面共用的业务类型与展示字典。
 *
 * 约定：
 * - 接口出入参类型一律取自 `api/generated/definitions/*`（如 `RiskRecord`、`PageRiskReq`），
 *   本文件只补充「页面视图层」需要的类型、字典与纯函数，不重复定义接口结构。
 * - 禁止 `as any`：所有列表数据都通过生成的强类型读取。
 */
import dayjs from 'dayjs'
import type { EChartsOption } from 'echarts'

// ════════════════════════════════════════════════════════════════
// 通用类型
// ════════════════════════════════════════════════════════════════

/** ECharts 配置（统一从 echarts 透出，避免各页面重复引入） */
export type ChartOption = EChartsOption

/** 分页表格查询基参数 */
export interface PageQuery {
  page: number
  page_size: number
}

/** 视频播放器状态（风险录像 / 设备实时视频共用） */
export interface VideoState {
  open: boolean
  url: string
  /** 弹窗标题，缺省「视频播放」 */
  title?: string
  /** 是否为直播流，仅影响弹窗文案 */
  live?: boolean
}

/** 关闭状态的视频弹窗常量，用于初始化 state */
export const CLOSED_VIDEO: VideoState = { open: false, url: '' }

/** 图片预览状态 */
export interface ImagePreviewState {
  open: boolean
  url: string
  title?: string
}

/** 关闭状态的图片预览常量 */
export const CLOSED_IMAGE: ImagePreviewState = { open: false, url: '' }

/** 自动刷新频率选项（10 秒 / 30 秒 / 60 秒） */
export const REFRESH_INTERVAL_OPTIONS: { label: string; value: number }[] = [
  { label: '不自动刷新', value: 0 },
  { label: '10 秒', value: 10 },
  { label: '30 秒', value: 30 },
  { label: '60 秒', value: 60 },
]

// ════════════════════════════════════════════════════════════════
// 状态字典（风险 / 告警 / 巡检 / 设备 / 物联网卡）
// ════════════════════════════════════════════════════════════════

/** 处理状态字典：pending 未处理 / handled 已处理 */
export const HANDLE_STATUS_OPTIONS = [
  { label: '未处理', value: 'pending' },
  { label: '已处理', value: 'handled' },
]

/** 检测准确字典：空=待确认 / yes 准确 / no 误报 */
export const DETECT_ACCURATE_OPTIONS = [
  { label: '待确认', value: '' },
  { label: '是', value: 'yes' },
  { label: '否', value: 'no' },
]

/** 消息通知（已读）字典 */
export const READ_OPTIONS = [
  { label: '未读', value: false },
  { label: '已读', value: true },
]

/** 巡检任务状态字典 */
export const INSPECT_STATUS_OPTIONS = [
  { label: '待发布', value: 'pending' },
  { label: '已发布', value: 'published' },
  { label: '已完成', value: 'finished' },
  { label: '已取消', value: 'canceled' },
]

/** 设备运行状态字典 */
export const DEVICE_STATUS_OPTIONS = [
  { label: '在线', value: 'online' },
  { label: '离线', value: 'offline' },
]

/** 物联网卡状态字典 */
export const IOT_CARD_STATUS_OPTIONS = [
  { label: '正常', value: 'normal' },
  { label: '欠费', value: 'arrears' },
  { label: '到期', value: 'expired' },
  { label: '停用', value: 'disabled' },
]

/** 等级字典（风险等级 / 告警等级） */
export const LEVEL_OPTIONS = [
  { label: '高', value: 'high' },
  { label: '中', value: 'medium' },
  { label: '低', value: 'low' },
]

/** 等级 → 中文文案 */
export const LEVEL_LABELS: Record<string, string> = {
  high: '高',
  medium: '中',
  low: '低',
}

/** 等级 → 颜色（图表与标签共用） */
export const LEVEL_COLORS: Record<string, string> = {
  high: '#f5222d',
  medium: '#fa8c16',
  low: '#52c41a',
}

/** 处理状态 → 中文文案 */
export const HANDLE_STATUS_LABELS: Record<string, string> = {
  pending: '未处理',
  handled: '已处理',
}

/** 巡检任务状态 → 中文文案 */
export const INSPECT_STATUS_LABELS: Record<string, string> = {
  pending: '待发布',
  published: '已发布',
  finished: '已完成',
  canceled: '已取消',
}

/** 物联网卡状态 → 中文文案 */
export const IOT_CARD_STATUS_LABELS: Record<string, string> = {
  normal: '正常',
  arrears: '欠费',
  expired: '到期',
  disabled: '停用',
}

// ════════════════════════════════════════════════════════════════
// 通用纯函数
// ════════════════════════════════════════════════════════════════

/** 时间格式化：空值返回 '-'，避免表格出现 undefined */
export function formatDT(value?: string | null, pattern = 'YYYY-MM-DD HH:mm:ss'): string {
  if (!value) return '-'
  const d = dayjs(value)
  return d.isValid() ? d.format(pattern) : '-'
}

/** 数值格式化：空值返回 '-' */
export function formatNumber(value?: number | null): string {
  return value === undefined || value === null ? '-' : String(value)
}

/** 存储容量（MB）格式化：小于 1024MB 显示 MB，否则显示 GB */
export function formatStorage(mb?: number | null): string {
  if (mb === undefined || mb === null) return '-'
  if (mb < 1024) return `${mb} MB`
  return `${(mb / 1024).toFixed(2)} GB`
}

/** 流量（MB）格式化，口径同存储容量 */
export function formatTraffic(mb?: number | null): string {
  return formatStorage(mb)
}

/** 百分比计算（保留 1 位小数，分母为 0 时返回 0） */
export function usagePercent(used?: number | null, total?: number | null): number {
  if (!total || total <= 0) return 0
  const value = ((used ?? 0) / total) * 100
  return Math.min(100, Math.max(0, Number(value.toFixed(1))))
}

/** 剩余百分比（电量 / 剩余容量进度条用） */
export function leftPercent(left?: number | null, total?: number | null): number {
  if (left === undefined || left === null) return 0
  if (!total || total <= 0) return Math.min(100, Math.max(0, left))
  return usagePercent(left, total)
}

/** 进度条颜色：>=60 绿 / >=20 橙 / 其余红 */
export function gaugeColor(percent: number): string {
  if (percent >= 60) return '#52c41a'
  if (percent >= 20) return '#fa8c16'
  return '#f5222d'
}

/** 名称-数量统计项 → 饼图 / 柱状图数据（{name, value}） */
export function toChartData(list?: { name?: string; count?: number }[]): { name: string; value: number }[] {
  if (!Array.isArray(list)) return []
  return list.map((item) => ({ name: item.name ?? '-', value: Number(item.count ?? 0) }))
}

/** 组装 ECharts 饼图 / 环形图配置 */
export function pieOption(data: { name: string; value: number }[], ring = false): ChartOption {
  return {
    tooltip: { trigger: 'item' },
    legend: { bottom: 0, type: 'scroll' },
    series: [
      {
        type: 'pie',
        radius: ring ? ['45%', '70%'] : '65%',
        center: ['50%', '45%'],
        data,
        emphasis: { itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0,0,0,0.3)' } },
        label: { formatter: '{b}: {c}' },
      },
    ],
  }
}

/** 组装 ECharts 柱状图配置（horizontal=true 时横向） */
export function barOption(
  categories: string[],
  values: number[],
  horizontal = false,
  color = '#1677ff'
): ChartOption {
  const categoryAxis = { type: 'category' as const, data: categories, axisLabel: { interval: 0, rotate: horizontal ? 0 : 30 } }
  const valueAxis = { type: 'value' as const }
  return {
    tooltip: { trigger: 'axis' },
    grid: { left: horizontal ? 110 : 40, right: 24, top: 24, bottom: horizontal ? 24 : 70, containLabel: true },
    xAxis: horizontal ? valueAxis : categoryAxis,
    yAxis: horizontal ? { ...categoryAxis, inverse: true } : valueAxis,
    series: [{ type: 'bar', data: values, barMaxWidth: 28, itemStyle: { color } }],
  }
}

/** 组装 ECharts 折线图配置（多系列） */
export function lineOption(
  categories: string[],
  series: { name: string; data: number[]; color?: string }[]
): ChartOption {
  return {
    tooltip: { trigger: 'axis' },
    legend: { bottom: 0 },
    grid: { left: 40, right: 24, top: 24, bottom: 56, containLabel: true },
    xAxis: { type: 'category', data: categories, boundaryGap: false },
    yAxis: { type: 'value' },
    series: series.map((item) => ({
      name: item.name,
      type: 'line',
      smooth: true,
      showSymbol: true,
      data: item.data,
      itemStyle: { color: item.color },
      areaStyle: { opacity: 0.08 },
    })),
  }
}
