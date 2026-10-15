import React, { useCallback, useEffect, useState } from 'react'
import {
  Badge, Button, DatePicker, Descriptions, Form, Image, Input, message, Modal, Popconfirm, Space, Table,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { BellOutlined, DeleteOutlined, EyeOutlined, ToolOutlined } from '@ant-design/icons'
import type { Dayjs } from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import ImageUpload from '../../components/ImageUpload'
import AlarmTypeSelect from '../../components/AlarmTypeSelect'
import RegionSelect from '../../components/RegionSelect'
import StatusTag from '../../components/StatusTag'
import { AlarmController } from '../../api/generated/AlarmController'
import type { AlarmRecord } from '../../api/generated/definitions/AlarmRecord'
import type { PageAlarmReq } from '../../api/generated/definitions/PageAlarmReq'
import type { HandleAlarmReq } from '../../api/generated/definitions/HandleAlarmReq'
import { useAuthStore } from '../../store/authStore'
import {
  HANDLE_STATUS_OPTIONS, LEVEL_LABELS, formatDT,
} from '../../types/business'

const { RangePicker } = DatePicker

/**
 * 设备告警：设备自身状态（离线 / 电量 / 流量）产生的告警查询与处理。
 *
 * - 点击「查看」打开详情，同时把未读消息通知转为已读；
 * - 「处理」填写处理结果与处理图片后状态由未处理转为已处理。
 */
const AlarmList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canHandle = permCodes.includes('alarm:handle')
  const canDelete = permCodes.includes('alarm:delete')

  const [data, setData] = useState<AlarmRecord[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [pendingCount, setPendingCount] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [keyword, setKeyword] = useState('')
  const [deviceCode, setDeviceCode] = useState('')
  const [alarmTypeID, setAlarmTypeID] = useState<number | undefined>()
  const [regionID, setRegionID] = useState<number | undefined>()
  const [status, setStatus] = useState<string | undefined>()
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null)

  const [detailOpen, setDetailOpen] = useState(false)
  const [detail, setDetail] = useState<AlarmRecord | null>(null)
  const [handleOpen, setHandleOpen] = useState(false)
  const [handleTarget, setHandleTarget] = useState<AlarmRecord | null>(null)
  const [handleImage, setHandleImage] = useState('')
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<HandleAlarmReq>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const query: PageAlarmReq = {
        page,
        page_size: pageSize,
        keyword: keyword || undefined,
        device_code: deviceCode || undefined,
        alarm_type_id: alarmTypeID,
        region_id: regionID,
        status: status || undefined,
        start_dt: range?.[0] ? range[0].format('YYYY-MM-DD HH:mm:ss') : undefined,
        end_dt: range?.[1] ? range[1].format('YYYY-MM-DD HH:mm:ss') : undefined,
      }
      const res = await AlarmController.pageAlarms(query)
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch {
      message.error('获取告警列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, deviceCode, alarmTypeID, regionID, status, range])

  const fetchPending = useCallback(async () => {
    try {
      const res = await AlarmController.alarmPendingCount()
      setPendingCount(Number(res.data ?? 0))
    } catch {
      /* 角标失败不阻塞列表 */
    }
  }, [])

  useEffect(() => { void fetchData() }, [fetchData])
  useEffect(() => { void fetchPending() }, [fetchPending])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setDeviceCode(values.device_code ?? '')
    setStatus(values.status || undefined)
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setDeviceCode('')
    setAlarmTypeID(undefined)
    setRegionID(undefined)
    setStatus(undefined)
    setRange(null)
    setPage(1)
  }

  /** 查看详情：未读消息通知转为已读 */
  const openDetail = async (row: AlarmRecord) => {
    setDetail(row)
    setDetailOpen(true)
    if (row.id && !row.read) {
      try {
        await AlarmController.markAlarmRead(row.id)
        void fetchData()
        void fetchPending()
      } catch {
        /* 已读标记失败不阻塞详情查看 */
      }
    }
  }

  const openHandle = (row: AlarmRecord) => {
    setHandleTarget(row)
    setHandleImage(row.handle_image ?? '')
    form.resetFields()
    setHandleOpen(true)
  }

  const submitHandle = async () => {
    const values = await form.validateFields()
    if (!handleTarget?.id) return
    setSaving(true)
    try {
      const payload: HandleAlarmReq = { handle_result: values.handle_result, handle_image: handleImage }
      await AlarmController.handleAlarm(handleTarget.id, payload)
      message.success('处理成功，状态已转为已处理')
      setHandleOpen(false)
      setDetailOpen(false)
      void fetchData()
      void fetchPending()
    } catch {
      message.error('处理失败')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (row: AlarmRecord) => {
    if (!row.id) return
    try {
      await AlarmController.deleteAlarm(row.id)
      message.success('删除成功')
      void fetchData()
      void fetchPending()
    } catch {
      message.error('删除失败')
    }
  }

  const columns: ColumnsType<AlarmRecord> = [
    {
      title: '告警类型',
      dataIndex: ['alarm_type', 'name'],
      width: 150,
      render: (_: unknown, row) => (
        <a onClick={() => { setAlarmTypeID(row.alarm_type_id); setPage(1) }}>{row.alarm_type?.name ?? '-'}</a>
      ),
    },
    { title: '所属区域', dataIndex: ['region', 'name'], width: 140, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '设备名称', dataIndex: ['device', 'name'], width: 150, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    {
      title: '设备编号',
      dataIndex: 'device_code',
      width: 150,
      ellipsis: true,
      render: (value: unknown) => (value ? <a onClick={() => { setDeviceCode(String(value)); setPage(1) }}>{String(value)}</a> : '-'),
    },
    { title: '设备位置', dataIndex: 'location', width: 180, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '告警内容', dataIndex: 'content', width: 220, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    {
      title: '告警等级',
      dataIndex: 'alarm_level',
      width: 100,
      render: (v?: string) => (v ? LEVEL_LABELS[v] ?? v : '-'),
    },
    { title: '状态', dataIndex: 'status', width: 100, render: (v?: string) => <StatusTag type="alarm" status={v} /> },
    {
      title: '消息通知',
      dataIndex: 'read',
      width: 100,
      render: (v?: boolean) => <StatusTag type="alarm" status={v ? 'handled' : 'pending'} />,
    },
    { title: '告警时间', dataIndex: 'occurred_dt', width: 170, render: (v?: string) => formatDT(v) },
    { title: '处理人', dataIndex: 'handler', width: 110, render: (v?: string) => v || '-' },
    { title: '处理时间', dataIndex: 'handled_dt', width: 170, render: (v?: string) => formatDT(v) },
    {
      title: '操作',
      key: 'action',
      width: 220,
      fixed: 'right',
      render: (_: unknown, row) => (
        <ActionButtons maxWidth={220}>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => void openDetail(row)}>
            查看
          </Button>
          {canHandle && (
            <Button
              type="link"
              size="small"
              icon={<ToolOutlined />}
              disabled={row.status === 'handled'}
              onClick={() => openHandle(row)}
            >
              处理
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除?" onConfirm={() => handleDelete(row)}>
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
      title="设备告警"
      subTitle="设备离线、电量、流量等异常告警的查询与处理"
      extra={
        <Badge count={pendingCount} overflowCount={999} color="red">
          <Button icon={<BellOutlined />}>待处理告警</Button>
        </Badge>
      }
    >
      <SearchBar
        fields={[
          { name: 'keyword', label: '设备名称', placeholder: '请输入设备名称' },
          { name: 'device_code', label: '设备编号', placeholder: '请输入设备编号' },
          { name: 'status', label: '状态', type: 'select', options: HANDLE_STATUS_OPTIONS },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Space wrap style={{ marginBottom: 16 }}>
        <AlarmTypeSelect
          style={{ width: 180 }}
          value={alarmTypeID}
          onChange={(v) => { setAlarmTypeID(v === undefined || v === null ? undefined : Number(v)); setPage(1) }}
        />
        <RegionSelect
          style={{ width: 180 }}
          value={regionID}
          onChange={(v) => { setRegionID(v === undefined || v === null ? undefined : Number(v)); setPage(1) }}
        />
        <RangePicker
          showTime
          value={range}
          placeholder={['告警时间起', '告警时间止']}
          onChange={(dates) => { setRange(dates); setPage(1) }}
        />
      </Space>

      <Table<AlarmRecord>
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

      {/* 告警详情（打开即把未读消息通知转为已读） */}
      <Modal
        className="e2e-alarm-detail-modal"
        title={`告警详情${detail?.id ? ` #${detail.id}` : ''}`}
        open={detailOpen}
        onCancel={() => setDetailOpen(false)}
        footer={
          <Space>
            <Button onClick={() => setDetailOpen(false)}>关闭</Button>
            {canHandle && (
              <Button type="primary" disabled={detail?.status === 'handled'} onClick={() => detail && openHandle(detail)}>
                处理
              </Button>
            )}
          </Space>
        }
        width={760}
      >
        {detail && (
          <>
            <Descriptions column={2} size="small" bordered>
              <Descriptions.Item label="告警类型">{detail.alarm_type?.name ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="告警等级">{detail.alarm_level ? LEVEL_LABELS[detail.alarm_level] ?? detail.alarm_level : '-'}</Descriptions.Item>
              <Descriptions.Item label="设备名称">{detail.device?.name ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="设备编号">{detail.device_code || '-'}</Descriptions.Item>
              <Descriptions.Item label="所属区域">{detail.region?.name ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="设备位置">{detail.location || '-'}</Descriptions.Item>
              <Descriptions.Item label="告警时间">{formatDT(detail.occurred_dt)}</Descriptions.Item>
              <Descriptions.Item label="消息通知">
                <StatusTag type="alarm" status={detail.read ? 'handled' : 'pending'} />
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <StatusTag type="alarm" status={detail.status} />
              </Descriptions.Item>
              <Descriptions.Item label="处理人">{detail.handler || '-'}</Descriptions.Item>
              <Descriptions.Item label="处理时间">{formatDT(detail.handled_dt)}</Descriptions.Item>
              <Descriptions.Item label="告警内容" span={2}>
                {detail.content || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="处理结果" span={2}>
                {detail.handle_result || '-'}
              </Descriptions.Item>
            </Descriptions>
            {detail.handle_image ? (
              <div style={{ marginTop: 16 }}>
                <div style={{ marginBottom: 8 }}>处理图片</div>
                <Image src={detail.handle_image} width={140} alt="处理图片" />
              </div>
            ) : null}
          </>
        )}
      </Modal>

      {/* 告警处理：处理结果 + 处理图片 */}
      <Modal
        className="e2e-alarm-handle-modal"
        title={`处理告警${handleTarget?.id ? ` #${handleTarget.id}` : ''}`}
        open={handleOpen}
        onCancel={() => setHandleOpen(false)}
        onOk={submitHandle}
        confirmLoading={saving}
        destroyOnHidden
        width={640}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="handle_result" label="处理结果" rules={[{ required: true, message: '请填写处理结果' }]}>
            <Input.TextArea rows={4} maxLength={500} showCount placeholder="请填写处理情况，如已更换电池并重启设备" />
          </Form.Item>
          <Form.Item label="处理图片">
            <ImageUpload value={handleImage} onChange={(v) => setHandleImage(Array.isArray(v) ? (v[0] ?? '') : v)} maxCount={1} />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  )
}

export default AlarmList
