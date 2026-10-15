import React, { useCallback, useEffect, useState } from 'react'
import { Button, Card, Col, DatePicker, message, Radio, Row, Space, Statistic } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import type { Dayjs } from 'dayjs'
import PageContainer from '../../components/PageContainer'
import ChartCard from './ChartCard'
import { StatsController } from '../../api/generated/StatsController'
import type { AlarmStatsResp } from '../../api/generated/definitions/AlarmStatsResp'
import { LEVEL_LABELS, barOption, lineOption, pieOption, toChartData } from '../../types/business'

const { RangePicker } = DatePicker

/** 时间范围快捷选项 */
const QUICK_RANGES: { label: string; days: number }[] = [
  { label: '今日', days: 1 },
  { label: '近 7 天', days: 7 },
  { label: '近 30 天', days: 30 },
  { label: '近 90 天', days: 90 },
]

/**
 * 设备告警分析：按时间段的维度化告警统计。
 *
 * 维度：告警类型（饼图）、所属区域（柱状图）、告警等级（环形图）、
 * 设备 Top10（横向柱状图）与逐日趋势（折线图）。
 */
const AlarmStats: React.FC = () => {
  const [range, setRange] = useState<[Dayjs, Dayjs]>([
    dayjs().subtract(29, 'day').startOf('day'),
    dayjs().endOf('day'),
  ])
  const [quickDays, setQuickDays] = useState(30)
  const [data, setData] = useState<AlarmStatsResp | null>(null)
  const [loading, setLoading] = useState(false)

  const fetchData = useCallback(async (start: Dayjs, end: Dayjs) => {
    setLoading(true)
    try {
      const res = await StatsController.alarmStats({
        start_dt: start.format('YYYY-MM-DD HH:mm:ss'),
        end_dt: end.format('YYYY-MM-DD HH:mm:ss'),
      })
      setData(res.data ?? null)
    } catch {
      message.error('获取告警统计数据失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchData(range[0], range[1])
  }, [fetchData, range])

  const applyQuickRange = (days: number) => {
    setQuickDays(days)
    setRange([dayjs().subtract(days - 1, 'day').startOf('day'), dayjs().endOf('day')])
  }

  const total = Number(data?.total ?? 0)
  const handled = Number(data?.handled ?? 0)
  const pending = Number(data?.pending ?? 0)
  const handledRate = total > 0 ? ((handled / total) * 100).toFixed(1) : '0.0'

  const typeData = toChartData(data?.by_type)
  const regionData = toChartData(data?.by_region)
  const levelData = toChartData(data?.by_level).map((item) => ({
    name: LEVEL_LABELS[item.name] ?? item.name,
    value: item.value,
  }))
  const deviceData = toChartData(data?.by_device)
  const trend = data?.trend ?? []

  return (
    <PageContainer
      title="设备告警分析"
      subTitle="按时间段统计告警的类型、区域、等级、设备分布与趋势"
      extra={
        <Space wrap>
          <Radio.Group
            value={quickDays}
            optionType="button"
            options={QUICK_RANGES.map((item) => ({ label: item.label, value: item.days }))}
            onChange={(e) => applyQuickRange(Number(e.target.value))}
          />
          <RangePicker
            showTime
            value={range}
            allowClear={false}
            onChange={(dates) => {
              if (dates?.[0] && dates[1]) setRange([dates[0], dates[1]])
            }}
          />
          <Button
            type="primary"
            icon={<ReloadOutlined />}
            loading={loading}
            onClick={() => void fetchData(range[0], range[1])}
          >
            查询
          </Button>
        </Space>
      }
    >
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col xs={24} sm={8}>
          <Card size="small">
            <Statistic title="告警总数" value={total} />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card size="small">
            <Statistic title="已处理" value={handled} valueStyle={{ color: '#52c41a' }} suffix={`(${handledRate}%)`} />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card size="small">
            <Statistic title="未处理" value={pending} valueStyle={{ color: '#fa8c16' }} />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <ChartCard title="按告警类型统计" loading={loading} option={pieOption(typeData)} height={320} />
        </Col>
        <Col xs={24} lg={12}>
          <ChartCard
            title="按所属区域统计"
            loading={loading}
            option={barOption(regionData.map((i) => i.name), regionData.map((i) => i.value), false, '#13c2c2')}
            height={320}
          />
        </Col>
        <Col xs={24} lg={12}>
          <ChartCard title="按告警等级统计" loading={loading} option={pieOption(levelData, true)} height={320} />
        </Col>
        <Col xs={24} lg={12}>
          <ChartCard
            title="按设备统计 Top10"
            loading={loading}
            option={barOption(deviceData.map((i) => i.name), deviceData.map((i) => i.value), true, '#eb2f96')}
            height={320}
          />
        </Col>
        <Col span={24}>
          <ChartCard
            title="告警趋势"
            loading={loading}
            height={320}
            option={lineOption(trend.map((item) => item.date ?? ''), [
              { name: '告警数', data: trend.map((item) => Number(item.count ?? 0)), color: '#fa541c' },
            ])}
          />
        </Col>
      </Row>
    </PageContainer>
  )
}

export default AlarmStats
