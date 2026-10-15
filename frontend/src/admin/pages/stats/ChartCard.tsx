import React from 'react'
import { Card, Empty } from 'antd'
import ReactECharts from 'echarts-for-react'
import type { ChartOption } from '../../types/business'

export interface ChartCardProps {
  /** 图表标题 */
  title: string
  /** 图表配置 */
  option: ChartOption
  /** 图表高度，默认 300 */
  height?: number
  /** 是否使用深色主题（可视化大屏使用） */
  dark?: boolean
  /** 加载中 */
  loading?: boolean
  /** 右上角附加内容 */
  extra?: React.ReactNode
  /** 无数据时的空态文案 */
  emptyText?: string
}

/**
 * 统计图表卡片：统一标题、高度与 echarts 主题，避免各页面重复配置。
 *
 * @example
 * <ChartCard title="按风险类型统计" option={pieOption(data, true)} height={320} />
 */
const ChartCard: React.FC<ChartCardProps> = ({
  title,
  option,
  height = 300,
  dark = false,
  loading,
  extra,
  emptyText = '暂无数据',
}) => {
  // 各系列均无数据时展示空态，避免出现空白坐标系
  const series = option.series
  const seriesList = Array.isArray(series) ? series : series ? [series] : []
  const hasData = seriesList.some((item) => {
    const data = (item as { data?: unknown }).data
    return Array.isArray(data) ? data.length > 0 : Boolean(data)
  })

  return (
    <Card
      size="small"
      title={title || undefined}
      extra={extra}
      loading={loading}
      styles={{
        body: { padding: 12, background: dark ? 'transparent' : undefined },
        header: dark ? { color: '#cfe3ff', background: 'transparent', borderBottom: '1px solid #12305c' } : undefined,
      }}
      style={dark ? { background: 'rgba(9,26,54,0.85)', border: '1px solid #12305c' } : undefined}
    >
      {hasData ? (
        <ReactECharts
          option={option}
          theme={dark ? 'dark' : undefined}
          style={{ height, background: 'transparent' }}
          notMerge
          lazyUpdate
        />
      ) : (
        <div style={{ height, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<span style={{ color: dark ? '#7f9bc4' : undefined }}>{emptyText}</span>} />
        </div>
      )}
    </Card>
  )
}

export default ChartCard
