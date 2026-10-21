import React, { useCallback, useEffect, useState } from 'react'
import {
  Button, DatePicker, Descriptions, Form, Image, Input, message, Modal, Popconfirm, Space, Table, Tag,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { CameraOutlined, DeleteOutlined, EyeOutlined, ToolOutlined } from '@ant-design/icons'
import type { Dayjs } from 'dayjs'
import axios from 'axios'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import DeviceSelect from '../../components/DeviceSelect'
import { GbAlarmController } from '../../api/generated/GbAlarmController'
import type { GbAlarmItem } from '../../api/generated/definitions/GbAlarmItem'
import type { GbAlarmDetailResp } from '../../api/generated/definitions/GbAlarmDetailResp'
import { useAuthStore } from '../../store/authStore'
import { formatDT } from '../../types/business'

const { RangePicker } = DatePicker

/** 国标报警类型编码 → 文案（GB/T 28181 AlarmType）。 */
const ALARM_TYPE_LABELS: Record<string, string> = {
  '1': '视频报警',
  '2': '设备报警',
  '3': '交通报警',
  '4': '报警传输',
  '5': '其他报警',
}

/** 国标报警级别 → 文案 / 颜色。 */
const PRIORITY_META: Record<string, { text: string; color: string }> = {
  '1': { text: '一级（特急）', color: 'red' },
  '2': { text: '二级（紧急）', color: 'orange' },
  '3': { text: '三级（一般）', color: 'blue' },
  '4': { text: '四级（其他）', color: 'default' },
}

/** 处理状态筛选（handled 为空串时由 NormalizeEmptyQuery 中间件剔除）。 */
const HANDLED_OPTIONS = [
  { label: '未处理', value: false },
  { label: '已处理', value: true },
]

/** 请求失败提示：优先展示后端返回的 message。 */
function apiErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError<{ message?: string }>(err)) {
    return err.response?.data?.message || err.message || fallback
  }
  return err instanceof Error ? err.message : fallback
}

function alarmTypeText(type?: string): string {
  if (!type) return '-'
  return ALARM_TYPE_LABELS[type] ?? type
}

function priorityMeta(priority?: string): { text: string; color: string } {
  if (!priority) return { text: '-', color: 'default' }
  return PRIORITY_META[priority] ?? { text: priority, color: 'default' }
}

interface HandleFormValues {
  remark: string
}

/**
 * 国标报警：设备通过 MANSCDP Alarm 主动上报的报警。
 *
 * 与「设备告警」区分：这里不参与 AI 风险识别链路，抓图复用 ZLM getSnap。
 * 支持详情、抓图、处理（填写处理意见）与批量/单条清空。
 */
const GbAlarmList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canHandle = permCodes.includes('gb:alarm:handle')
  const canSnap = permCodes.includes('gb:alarm:snap')
  const canDelete = permCodes.includes('gb:alarm:delete')

  const [data, setData] = useState<GbAlarmItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [keyword, setKeyword] = useState('')
  const [handled, setHandled] = useState<boolean | undefined>()
  const [deviceId, setDeviceId] = useState<number | undefined>()
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null)
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])

  const [detailOpen, setDetailOpen] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detail, setDetail] = useState<GbAlarmDetailResp | null>(null)

  const [snapOpen, setSnapOpen] = useState(false)
  const [snapUrl, setSnapUrl] = useState('')
  const [snappingId, setSnappingId] = useState<number | undefined>()

  const [handleOpen, setHandleOpen] = useState(false)
  const [handleTargetId, setHandleTargetId] = useState<number | undefined>()
  const [handleSubtitle, setHandleSubtitle] = useState('')
  const [saving, setSaving] = useState(false)
  const [handleForm] = Form.useForm<HandleFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbAlarmController.pageGbAlarms(
        page,
        pageSize,
        keyword || undefined,
        deviceId,
        undefined,
        undefined,
        undefined,
        handled,
        range?.[0]?.format('YYYY-MM-DD HH:mm:ss'),
        range?.[1]?.format('YYYY-MM-DD HH:mm:ss'),
      )
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch (err) {
      message.error(apiErrorMessage(err, '获取国标报警列表失败'))
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, deviceId, handled, range])

  useEffect(() => { void fetchData() }, [fetchData])

  const handleSearch = (values: Record<string, unknown>) => {
    setKeyword(typeof values.keyword === 'string' ? values.keyword : '')
    setHandled(typeof values.handled === 'boolean' ? values.handled : undefined)
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setHandled(undefined)
    setDeviceId(undefined)
    setRange(null)
    setSelectedRowKeys([])
    setPage(1)
  }

  const openDetail = async (row: GbAlarmItem) => {
    if (row.id === undefined) return
    setDetailOpen(true)
    setDetail(null)
    setDetailLoading(true)
    try {
      const res = await GbAlarmController.getGbAlarm(row.id)
      setDetail(res.data ?? null)
    } catch (err) {
      message.error(apiErrorMessage(err, '获取报警详情失败'))
    } finally {
      setDetailLoading(false)
    }
  }

  const openHandle = (row: GbAlarmItem | GbAlarmDetailResp) => {
    if (row.id === undefined) return
    setHandleTargetId(row.id)
    setHandleSubtitle([row.device_name, row.channel_name].filter(Boolean).join(' / '))
    handleForm.resetFields()
    setHandleOpen(true)
  }

  const submitHandle = async () => {
    if (handleTargetId === undefined) return
    const values = await handleForm.validateFields()
    setSaving(true)
    try {
      await GbAlarmController.handleGbAlarm(handleTargetId, { handled: true, remark: values.remark })
      message.success('处理成功')
      setHandleOpen(false)
      setDetailOpen(false)
      void fetchData()
    } catch (err) {
      message.error(apiErrorMessage(err, '处理报警失败'))
    } finally {
      setSaving(false)
    }
  }

  const handleSnapshot = async (row: GbAlarmItem | GbAlarmDetailResp) => {
    if (row.id === undefined) return
    setSnappingId(row.id)
    try {
      const res = await GbAlarmController.snapshotGbAlarm(row.id)
      const url = res.data?.url ?? ''
      if (!url) {
        message.error('抓图失败：后端未返回图片地址')
        return
      }
      message.success('抓图成功')
      setData((prev) => prev.map((item) => (
        item.id === row.id ? { ...item, snap_path: url } : item
      )))
      setDetail((prev) => (prev && prev.id === row.id ? { ...prev, snap_path: url } : prev))
      setSnapUrl(url)
      setSnapOpen(true)
    } catch (err) {
      message.error(apiErrorMessage(err, '抓图失败'))
    } finally {
      setSnappingId(undefined)
    }
  }

  const clearAlarms = async (ids: number[]) => {
    if (ids.length === 0) return
    try {
      await GbAlarmController.clearGbAlarms({ ids })
      message.success('清除成功')
      setSelectedRowKeys([])
      setDetailOpen(false)
      void fetchData()
    } catch (err) {
      message.error(apiErrorMessage(err, '清除报警失败'))
    }
  }

  const columns: ColumnsType<GbAlarmItem> = [
    {
      title: '报警时间',
      dataIndex: 'alarm_time',
      width: 170,
      render: (_: unknown, row) => formatDT(row.alarm_time || row.created_dt),
    },
    {
      title: '设备/通道名称',
      key: 'device_channel',
      width: 210,
      ellipsis: true,
      render: (_: unknown, row) => (
        <div>
          <div>{row.device_name || '-'}</div>
          <div style={{ color: '#8c8c8c', fontSize: 12 }}>
            {row.channel_name || row.gb_channel_id || '-'}
          </div>
        </div>
      ),
    },
    {
      title: '报警类型',
      dataIndex: 'alarm_type',
      width: 130,
      render: (value?: string) => <Tag color="blue">{alarmTypeText(value)}</Tag>,
    },
    {
      title: '等级',
      dataIndex: 'alarm_priority',
      width: 130,
      render: (value?: string) => {
        const meta = priorityMeta(value)
        return <Tag color={meta.color}>{meta.text}</Tag>
      },
    },
    {
      title: '处理状态',
      dataIndex: 'handled',
      width: 100,
      render: (value?: boolean) => (
        <Tag color={value ? 'green' : 'orange'}>{value ? '已处理' : '未处理'}</Tag>
      ),
    },
    {
      title: '处理人',
      dataIndex: 'updated_by',
      width: 110,
      render: (value?: string, row?: GbAlarmItem) => (row?.handled ? value || '-' : '-'),
    },
    {
      title: '操作',
      key: 'action',
      width: 300,
      fixed: 'right',
      render: (_: unknown, row) => (
        <ActionButtons maxWidth={300}>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => void openDetail(row)}>
            详情
          </Button>
          {canSnap && (
            <Button
              type="link"
              size="small"
              icon={<CameraOutlined />}
              loading={snappingId === row.id}
              onClick={() => void handleSnapshot(row)}
            >
              抓图
            </Button>
          )}
          {canHandle && (
            <Button
              type="link"
              size="small"
              icon={<ToolOutlined />}
              disabled={Boolean(row.handled)}
              onClick={() => openHandle(row)}
            >
              处理
            </Button>
          )}
          {canDelete && (
            <Popconfirm
              title="确定清空该条报警?"
              onConfirm={() => row.id !== undefined && void clearAlarms([row.id])}
            >
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                清空
              </Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  const detailMeta = detail ? priorityMeta(detail.alarm_priority) : null

  return (
    <PageContainer
      title="国标报警"
      subTitle="国标设备主动上报的报警查询、抓图与处理"
    >
      <SearchBar
        fields={[
          { name: 'keyword', label: '关键字', placeholder: '描述 / 国标编码 / 类型' },
          { name: 'handled', label: '处理状态', type: 'select', options: HANDLED_OPTIONS },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Space wrap style={{ marginBottom: 16 }}>
        <DeviceSelect
          style={{ width: 220 }}
          value={deviceId}
          onChange={(value) => { setDeviceId(value); setPage(1) }}
        />
        <RangePicker
          showTime
          value={range}
          placeholder={['报警时间起', '报警时间止']}
          onChange={(dates) => { setRange(dates); setPage(1) }}
        />
        {canDelete && (
          <Popconfirm
            title={`确定清空选中的 ${selectedRowKeys.length} 条报警?`}
            onConfirm={() => void clearAlarms(selectedRowKeys.map(Number))}
            disabled={selectedRowKeys.length === 0}
          >
            <Button danger icon={<DeleteOutlined />} disabled={selectedRowKeys.length === 0}>
              清空所选
            </Button>
          </Popconfirm>
        )}
      </Space>

      <Table<GbAlarmItem>
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 'max-content' }}
        rowSelection={canDelete ? {
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys),
        } : undefined}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => { setPage(p); setPageSize(ps) },
        }}
      />

      {/* 报警详情 */}
      <Modal
        className="e2e-gb-alarm-detail-modal"
        title={`国标报警详情${detail?.id ? ` #${detail.id}` : ''}`}
        open={detailOpen}
        onCancel={() => setDetailOpen(false)}
        loading={detailLoading}
        footer={
          <Space>
            <Button onClick={() => setDetailOpen(false)}>关闭</Button>
            {canSnap && detail && (
              <Button
                icon={<CameraOutlined />}
                loading={snappingId === detail.id}
                onClick={() => void handleSnapshot(detail)}
              >
                抓图
              </Button>
            )}
            {canHandle && detail && (
              <Button
                type="primary"
                disabled={Boolean(detail.handled)}
                onClick={() => openHandle(detail)}
              >
                处理
              </Button>
            )}
          </Space>
        }
        width={820}
      >
        {detail ? (
          <>
            <Descriptions column={2} size="small" bordered>
              <Descriptions.Item label="报警时间">{formatDT(detail.alarm_time || detail.created_dt)}</Descriptions.Item>
              <Descriptions.Item label="报警类型">{alarmTypeText(detail.alarm_type)}</Descriptions.Item>
              <Descriptions.Item label="报警等级">
                {detailMeta ? <Tag color={detailMeta.color}>{detailMeta.text}</Tag> : '-'}
              </Descriptions.Item>
              <Descriptions.Item label="处理状态">
                <Tag color={detail.handled ? 'green' : 'orange'}>{detail.handled ? '已处理' : '未处理'}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="设备名称">{detail.device_name || '-'}</Descriptions.Item>
              <Descriptions.Item label="通道名称">{detail.channel_name || '-'}</Descriptions.Item>
              <Descriptions.Item label="设备国标编码">{detail.gb_device_id || '-'}</Descriptions.Item>
              <Descriptions.Item label="通道国标编码">{detail.gb_channel_id || '-'}</Descriptions.Item>
              <Descriptions.Item label="报警方式">{detail.alarm_method ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="处理人">{detail.handled ? detail.updated_by || '-' : '-'}</Descriptions.Item>
              <Descriptions.Item label="经度">{detail.longitude ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="纬度">{detail.latitude ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="报警描述" span={2}>{detail.alarm_description || '-'}</Descriptions.Item>
              <Descriptions.Item label="备注" span={2}>{detail.remark || '-'}</Descriptions.Item>
            </Descriptions>
            {detail.snap_path ? (
              <div style={{ marginTop: 16 }}>
                <div style={{ marginBottom: 8 }}>关联抓图</div>
                <Image src={detail.snap_path} width={180} alt="报警抓图" />
              </div>
            ) : null}
          </>
        ) : null}
      </Modal>

      {/* 处理报警 */}
      <Modal
        className="e2e-gb-alarm-handle-modal"
        title={`处理国标报警${handleTargetId ? ` #${handleTargetId}` : ''}`}
        open={handleOpen}
        onCancel={() => setHandleOpen(false)}
        onOk={() => void submitHandle()}
        confirmLoading={saving}
        destroyOnHidden
        width={640}
      >
        <div style={{ color: '#8c8c8c', marginBottom: 12 }}>
          {handleSubtitle || '请填写处理意见'}
        </div>
        <Form form={handleForm} layout="vertical">
          <Form.Item
            name="remark"
            label="处理意见"
            rules={[{ required: true, message: '请填写处理意见' }]}
          >
            <Input.TextArea rows={4} maxLength={500} showCount placeholder="请填写处理情况" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 抓图预览 */}
      <Modal
        className="e2e-gb-alarm-snap-modal"
        title="报警抓图"
        open={snapOpen}
        onCancel={() => setSnapOpen(false)}
        footer={<Button onClick={() => setSnapOpen(false)}>关闭</Button>}
        width={760}
      >
        {snapUrl ? <Image src={snapUrl} alt="报警抓图" style={{ maxWidth: '100%' }} /> : null}
      </Modal>
    </PageContainer>
  )
}

export default GbAlarmList
