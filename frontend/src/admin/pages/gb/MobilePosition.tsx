import React, { useCallback, useEffect, useState } from 'react'
import {
  Button, Card, DatePicker, Form, InputNumber, message, Modal, Space, Table, Tag,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { AimOutlined, ReloadOutlined, SearchOutlined, SendOutlined } from '@ant-design/icons'
import type { Dayjs } from 'dayjs'
import axios from 'axios'
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

interface SubscribeFormValues {
  channel_id: number
  interval: number
}

/** 通道下拉框（数据源：`GET /web/gb/channels/options`）。 */
const channelFetcher = () => GbChannelController.gbChannelOptions()

/**
 * 移动位置：国标移动设备（车载 / 单兵）上报的位置。
 *
 * - 上半部分展示每个通道的最新位置（`realtime`）；
 * - 下半部分按通道与时间范围查询历史轨迹（`history`）。
 *
 * 本期不引入地图底图，直接用表格展示经纬度与运动信息。
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
  const [subscribing, setSubscribing] = useState(false)
  const [subscribeForm] = Form.useForm<SubscribeFormValues>()

  const openSubscribe = (channelId?: number) => {
    subscribeForm.resetFields()
    subscribeForm.setFieldsValue({ channel_id: channelId, interval: 60 })
    setSubscribeOpen(true)
  }

  const submitSubscribe = async () => {
    const values = await subscribeForm.validateFields()
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
        onOk={() => void submitSubscribe()}
        confirmLoading={subscribing}
        destroyOnHidden
        width={560}
      >
        <Form form={subscribeForm} layout="vertical">
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
      </Modal>
    </PageContainer>
  )
}

export default MobilePosition
