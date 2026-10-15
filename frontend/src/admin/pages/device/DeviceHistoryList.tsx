import React, { useCallback, useEffect, useState } from 'react'
import {
  Alert, Button, DatePicker, Descriptions, message, Modal, Space, Table, Tabs, Tag,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { BarChartOutlined, LineChartOutlined } from '@ant-design/icons'
import type { Dayjs } from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import RegionSelect from '../../components/RegionSelect'
import StatusTag from '../../components/StatusTag'
import ChartCard from '../stats/ChartCard'
import { DeviceController } from '../../api/generated/DeviceController'
import type { DeviceHistory } from '../../api/generated/definitions/DeviceHistory'
import type { DeviceHistoryChartResp } from '../../api/generated/definitions/DeviceHistoryChartResp'
import { DEVICE_STATUS_OPTIONS, formatDT, formatStorage, formatNumber } from '../../types/business'
import type { ChartOption } from '../../types/business'
import { lineOption } from '../../types/business'

const { RangePicker } = DatePicker

/** 一周历史运行状态汇总行 */
interface WeekStatusRow {
  device_id?: number
  device_name: string
  device_code: string
  online_minutes: number
  offline_minutes: number
  samples: number
}

/** 双轴折线（状态/电量、电压/电量） */
function dualAxisOption(
  times: string[],
  left: { name: string; data: number[]; color: string },
  right: { name: string; data: number[]; color: string }
): ChartOption {
  return {
    tooltip: { trigger: 'axis' },
    legend: { bottom: 0 },
    grid: { left: 50, right: 50, top: 30, bottom: 56, containLabel: true },
    xAxis: { type: 'category', data: times, boundaryGap: false },
    yAxis: [
      { type: 'value', name: left.name, position: 'left' },
      { type: 'value', name: right.name, position: 'right' },
    ],
    series: [
      { name: left.name, type: 'line', smooth: true, data: left.data, itemStyle: { color: left.color }, yAxisIndex: 0 },
      { name: right.name, type: 'line', smooth: true, data: right.data, itemStyle: { color: right.color }, yAxisIndex: 1 },
    ],
  }
}

/**
 * 设备历史记录：设备运行时序采样数据与历史曲线。
 *
 * - 「查看图表」展示近 3 日的运行状态 / 电量 / 存储容量 / 状态与电量 / 电压与电量统计；
 * - 「查看历史运行状态」需先选择区域并点击查询后才可点击，展示设备一周的运行状态。
 */
const DeviceHistoryList: React.FC = () => {
  const [data, setData] = useState<DeviceHistory[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState<string | undefined>()
  const [regionID, setRegionID] = useState<number | undefined>()
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null)
  /** 是否已点击过查询（「查看历史运行状态」按钮的前置条件之一） */
  const [queried, setQueried] = useState(false)

  const [chartOpen, setChartOpen] = useState(false)
  const [chartLoading, setChartLoading] = useState(false)
  const [chartData, setChartData] = useState<DeviceHistoryChartResp | null>(null)
  const [chartRow, setChartRow] = useState<DeviceHistory | null>(null)

  const [weekOpen, setWeekOpen] = useState(false)
  const [weekLoading, setWeekLoading] = useState(false)
  const [weekRows, setWeekRows] = useState<WeekStatusRow[]>([])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await DeviceController.pageDeviceHistory(
        page,
        pageSize,
        undefined,
        regionID,
        status,
        keyword || undefined,
        range?.[0] ? range[0].format('YYYY-MM-DD HH:mm:ss') : undefined,
        range?.[1] ? range[1].format('YYYY-MM-DD HH:mm:ss') : undefined
      )
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch {
      message.error('获取设备历史记录失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, regionID, status, keyword, range])

  useEffect(() => { void fetchData() }, [fetchData])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setStatus(values.status || undefined)
    setQueried(true)
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setStatus(undefined)
    setRegionID(undefined)
    setRange(null)
    setQueried(false)
    setPage(1)
  }

  /** 查看图表：近 3 日运行状态 / 电量 / 存储容量 / 状态与电量 / 电压与电量 */
  const openChart = async (row: DeviceHistory) => {
    if (!row.device_id) {
      message.warning('该采样记录缺少设备信息')
      return
    }
    setChartRow(row)
    setChartData(null)
    setChartOpen(true)
    setChartLoading(true)
    try {
      const res = await DeviceController.deviceHistoryChart(String(row.device_id), 3)
      setChartData(res.data ?? null)
    } catch {
      message.error('获取历史曲线失败')
    } finally {
      setChartLoading(false)
    }
  }

  /** 查看历史运行状态（一周）：需已选区域并点击过查询 */
  const openWeekStatus = async () => {
    setWeekOpen(true)
    setWeekLoading(true)
    try {
      const rows: WeekStatusRow[] = []
      for (const item of data) {
        if (!item.device_id) continue
        try {
          const res = await DeviceController.deviceHistoryChart(String(item.device_id), 7)
          const payload = res.data
          rows.push({
            device_id: item.device_id,
            device_name: item.device?.name ?? '-',
            device_code: item.device_code ?? item.device?.device_code ?? '-',
            online_minutes: Number(payload?.status_summary?.online_minutes ?? 0),
            offline_minutes: Number(payload?.status_summary?.offline_minutes ?? 0),
            samples: payload?.status_points?.length ?? 0,
          })
        } catch {
          /* 单个设备失败不影响其余设备 */
        }
      }
      setWeekRows(rows)
    } finally {
      setWeekLoading(false)
    }
  }

  const statusTimes = (chartData?.status_points ?? []).map((p) => p.time ?? '')
  const batteryTimes = (chartData?.battery_points ?? []).map((p) => p.time ?? '')
  const voltageTimes = (chartData?.voltage_points ?? []).map((p) => p.time ?? '')
  const storageTimes = (chartData?.storage_points ?? []).map((p) => p.time ?? '')

  const columns: ColumnsType<DeviceHistory> = [
    { title: '设备名称', dataIndex: ['device', 'name'], width: 160, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '设备编号', dataIndex: 'device_code', width: 150, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '流量卡号', dataIndex: 'iot_card_no', width: 170, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '所属区域', dataIndex: ['region', 'name'], width: 140, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '设备位置', dataIndex: 'location', width: 170, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '电量', dataIndex: 'battery', width: 90, render: (v?: number) => (v === undefined || v === null ? '-' : `${v}%`) },
    { title: '电压', dataIndex: 'voltage', width: 90, render: (v?: string) => v || '-' },
    {
      title: '剩余存储容量',
      dataIndex: 'storage_free',
      width: 130,
      render: (v?: number) => formatStorage(v),
    },
    {
      title: '在线-离线时间',
      dataIndex: 'sampled_dt',
      width: 200,
      render: (_: unknown, row) => (
        <div style={{ lineHeight: 1.6 }}>
          <div>采样：{formatDT(row.sampled_dt)}</div>
          <div>状态：{row.status === 'online' ? '在线' : row.status === 'offline' ? '离线' : '-'}</div>
        </div>
      ),
    },
    { title: '运行状态', dataIndex: 'status', width: 100, render: (v?: string) => <StatusTag type="device" status={v} /> },
    {
      title: '历史曲线',
      key: 'chart',
      width: 120,
      fixed: 'right',
      render: (_: unknown, row) => (
        <ActionButtons maxWidth={120}>
          <Button type="link" size="small" icon={<BarChartOutlined />} onClick={() => void openChart(row)}>
            查看图表
          </Button>
        </ActionButtons>
      ),
    },
  ]

  const weekColumns: ColumnsType<WeekStatusRow> = [
    { title: '设备名称', dataIndex: 'device_name', width: 160, ellipsis: true },
    { title: '设备编号', dataIndex: 'device_code', width: 150, ellipsis: true },
    {
      title: '在线时长',
      dataIndex: 'online_minutes',
      width: 130,
      render: (v: number) => `${(v / 60).toFixed(1)} 小时`,
    },
    {
      title: '离线时长',
      dataIndex: 'offline_minutes',
      width: 130,
      render: (v: number) => `${(v / 60).toFixed(1)} 小时`,
    },
    { title: '采样点数', dataIndex: 'samples', width: 100 },
    {
      title: '在线率',
      key: 'rate',
      width: 110,
      render: (_: unknown, row) => {
        const sum = row.online_minutes + row.offline_minutes
        const rate = sum > 0 ? (row.online_minutes / sum) * 100 : 0
        return <Tag color={rate >= 90 ? 'green' : rate >= 60 ? 'orange' : 'red'}>{rate.toFixed(1)}%</Tag>
      },
    },
  ]

  return (
    <PageContainer
      title="设备历史记录"
      subTitle="设备心跳上报的时序采样数据与历史曲线"
      extra={
        <Space>
          <Button
            type="primary"
            icon={<LineChartOutlined />}
            disabled={!regionID || !queried}
            onClick={() => void openWeekStatus()}
          >
            查看历史运行状态
          </Button>
        </Space>
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="「查看历史运行状态」需先选择所属区域并点击「搜索」后再使用"
      />

      <SearchBar
        fields={[
          { name: 'keyword', label: '设备名称/编号', placeholder: '请输入设备名称或编号' },
          { name: 'status', label: '运行状态', type: 'select', options: DEVICE_STATUS_OPTIONS },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Space wrap style={{ marginBottom: 16 }}>
        <RegionSelect
          style={{ width: 180 }}
          value={regionID}
          onChange={(v) => { setRegionID(v === undefined || v === null ? undefined : Number(v)); setPage(1) }}
        />
        <RangePicker
          showTime
          value={range}
          placeholder={['采样时间起', '采样时间止']}
          onChange={(dates) => { setRange(dates); setPage(1) }}
        />
      </Space>

      <Table<DeviceHistory>
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 'max-content' }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => { setPage(p); setPageSize(ps) },
        }}
      />

      {/* 近 3 日历史曲线 */}
      <Modal
        className="e2e-device-chart-modal"
        title={`历史曲线 - ${chartRow?.device?.name ?? chartRow?.device_code ?? ''}`}
        open={chartOpen}
        onCancel={() => setChartOpen(false)}
        footer={<Button onClick={() => setChartOpen(false)}>关闭</Button>}
        width={980}
      >
        <Tabs
          items={[
            {
              key: 'status',
              label: '运行状态统计',
              children: (
                <>
                  <Descriptions column={2} size="small" style={{ marginBottom: 12 }}>
                    <Descriptions.Item label="在线时长">
                      {((chartData?.status_summary?.online_minutes ?? 0) / 60).toFixed(1)} 小时
                    </Descriptions.Item>
                    <Descriptions.Item label="离线时长">
                      {((chartData?.status_summary?.offline_minutes ?? 0) / 60).toFixed(1)} 小时
                    </Descriptions.Item>
                    <Descriptions.Item label="统计天数">{formatNumber(chartData?.days)}</Descriptions.Item>
                    <Descriptions.Item label="聚合粒度">{chartData?.granularity ?? '-'}</Descriptions.Item>
                  </Descriptions>
                  <ChartCard
                    title="运行状态曲线（1=在线，0=离线）"
                    loading={chartLoading}
                    height={320}
                    option={lineOption(statusTimes, [
                      { name: '运行状态', data: (chartData?.status_points ?? []).map((p) => Number(p.value ?? 0)), color: '#52c41a' },
                    ])}
                  />
                </>
              ),
            },
            {
              key: 'battery',
              label: '电量统计',
              children: (
                <ChartCard
                  title="电量统计（%）"
                  loading={chartLoading}
                  height={360}
                  option={lineOption(batteryTimes, [
                    { name: '电量', data: (chartData?.battery_points ?? []).map((p) => Number(p.value ?? 0)), color: '#1677ff' },
                  ])}
                />
              ),
            },
            {
              key: 'storage',
              label: '存储容量统计',
              children: (
                <ChartCard
                  title="剩余存储容量统计（MB）"
                  loading={chartLoading}
                  height={360}
                  option={lineOption(storageTimes, [
                    { name: '剩余存储', data: (chartData?.storage_points ?? []).map((p) => Number(p.value ?? 0)), color: '#722ed1' },
                  ])}
                />
              ),
            },
            {
              key: 'statusBattery',
              label: '状态与电量统计',
              children: (
                <ChartCard
                  title="状态与电量统计"
                  loading={chartLoading}
                  height={360}
                  option={dualAxisOption(
                    batteryTimes,
                    { name: '电量(%)', data: (chartData?.battery_points ?? []).map((p) => Number(p.value ?? 0)), color: '#1677ff' },
                    { name: '状态(1在线/0离线)', data: (chartData?.status_points ?? []).map((p) => Number(p.value ?? 0)), color: '#52c41a' }
                  )}
                />
              ),
            },
            {
              key: 'voltageBattery',
              label: '电压与电量统计',
              children: (
                <ChartCard
                  title="电压与电量统计"
                  loading={chartLoading}
                  height={360}
                  option={dualAxisOption(
                    voltageTimes,
                    { name: '电压(V)', data: (chartData?.voltage_points ?? []).map((p) => Number(p.value ?? 0)), color: '#fa8c16' },
                    { name: '电量(%)', data: (chartData?.battery_points ?? []).map((p) => Number(p.value ?? 0)), color: '#1677ff' }
                  )}
                />
              ),
            },
          ]}
        />
      </Modal>

      {/* 一周历史运行状态 */}
      <Modal
        className="e2e-device-week-modal"
        title="设备一周历史运行状态"
        open={weekOpen}
        onCancel={() => setWeekOpen(false)}
        footer={<Button onClick={() => setWeekOpen(false)}>关闭</Button>}
        width={880}
      >
        <Table<WeekStatusRow>
          rowKey={(row) => String(row.device_id ?? row.device_code)}
          size="small"
          loading={weekLoading}
          columns={weekColumns}
          dataSource={weekRows}
          scroll={{ x: 'max-content' }}
          pagination={false}
        />
      </Modal>
    </PageContainer>
  )
}

export default DeviceHistoryList
