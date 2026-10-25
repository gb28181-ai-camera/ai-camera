import React, { useCallback, useEffect, useState } from 'react'
import {
  Alert, Button, DatePicker, Descriptions, Form, Image, Input, message, Modal, Popconfirm, Radio,
  Segmented, Select, Space, Table,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  DeleteOutlined, EyeOutlined, MonitorOutlined, ToolOutlined, VideoCameraOutlined,
} from '@ant-design/icons'
import { useNavigate, useSearchParams } from 'react-router-dom'
import type { Dayjs } from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import ImageUpload from '../../components/ImageUpload'
import RiskTypeSelect from '../../components/RiskTypeSelect'
import RegionSelect from '../../components/RegionSelect'
import StatusTag from '../../components/StatusTag'
import VideoPlayer from '../../components/VideoPlayer'
import { RiskController } from '../../api/generated/RiskController'
import type { RiskRecord } from '../../api/generated/definitions/RiskRecord'
import type { PageRiskReq } from '../../api/generated/definitions/PageRiskReq'
import type { HandleRiskReq } from '../../api/generated/definitions/HandleRiskReq'
import type { UpdateAccurateReq } from '../../api/generated/definitions/UpdateAccurateReq'
import { useAuthStore } from '../../store/authStore'
import {
  CLOSED_IMAGE, CLOSED_VIDEO, DETECT_ACCURATE_OPTIONS, HANDLE_STATUS_OPTIONS, LEVEL_OPTIONS,
  REFRESH_INTERVAL_OPTIONS, formatDT, LEVEL_LABELS,
} from '../../types/business'
import type { ImagePreviewState, VideoState } from '../../types/business'

const { RangePicker } = DatePicker

/**
 * 风险监控：功能最全的风险列表。
 *
 * - 「风险类型」「设备名称」「设备编号」单元格可点击，直接按该值筛选；
 * - 「设备名称」点击跳转设备监控（携带 device_id）；
 * - 支持刷新频率设置（10 / 30 / 60 秒），按间隔自动拉取列表；
 * - 打开详情时自动把未读消息通知转为已读。
 */
const RiskMonitorList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canHandle = permCodes.includes('risk:handle')
  const canAccurate = permCodes.includes('risk:accurate')
  const canDelete = permCodes.includes('risk:delete')
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()

  const [data, setData] = useState<RiskRecord[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [keyword, setKeyword] = useState('')
  const [deviceCode, setDeviceCode] = useState('')
  const [deviceID, setDeviceID] = useState<number | undefined>()
  const [riskTypeID, setRiskTypeID] = useState<number | undefined>()
  const [regionID, setRegionID] = useState<number | undefined>()
  // 风险监控是「待处理处置队列」：默认只看未处理；切到「全部」可查已处理历史。
  const [status, setStatus] = useState<string | undefined>('pending')
  const [accurate, setAccurate] = useState<string | undefined>()
  const [level, setLevel] = useState<string | undefined>()
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null)

  /** 刷新频率（秒），0 表示不自动刷新 */
  const [refreshSec, setRefreshSec] = useState(0)

  const [detailOpen, setDetailOpen] = useState(false)
  const [detail, setDetail] = useState<RiskRecord | null>(null)
  const [imagePreview, setImagePreview] = useState<ImagePreviewState>(CLOSED_IMAGE)
  const [video, setVideo] = useState<VideoState>(CLOSED_VIDEO)
  const [handleOpen, setHandleOpen] = useState(false)
  const [handleTarget, setHandleTarget] = useState<RiskRecord | null>(null)
  const [handleImage, setHandleImage] = useState('')
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<HandleRiskReq>()

  // 来自其他页面的跳转参数：device_id（设备筛选）/ risk_id（直接打开详情）
  const jumpDeviceID = searchParams.get('device_id')
  const jumpRiskID = searchParams.get('risk_id')

  useEffect(() => {
    if (jumpDeviceID) {
      setDeviceID(Number(jumpDeviceID))
      setPage(1)
    }
    if (searchParams.get('device_name')) {
      setKeyword(searchParams.get('device_name') ?? '')
      setPage(1)
    }
  }, [jumpDeviceID, searchParams])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const query: PageRiskReq = {
        page,
        page_size: pageSize,
        keyword: keyword || undefined,
        device_code: deviceCode || undefined,
        device_id: deviceID,
        risk_type_id: riskTypeID,
        region_id: regionID,
        status: status || undefined,
        detect_accurate: accurate,
        risk_level: level || undefined,
        start_dt: range?.[0] ? range[0].format('YYYY-MM-DD HH:mm:ss') : undefined,
        end_dt: range?.[1] ? range[1].format('YYYY-MM-DD HH:mm:ss') : undefined,
      }
      const res = await RiskController.pageRisks(query)
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch {
      message.error('获取风险列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, deviceCode, deviceID, riskTypeID, regionID, status, accurate, level, range])

  useEffect(() => { void fetchData() }, [fetchData])

  // 刷新频率：按选中的间隔自动拉取，组件卸载 / 频率变更时清理定时器
  useEffect(() => {
    if (refreshSec <= 0) return
    const timer = window.setInterval(() => { void fetchData() }, refreshSec * 1000)
    return () => { window.clearInterval(timer) }
  }, [refreshSec, fetchData])

  /** 查看详情：同时把未读消息通知转为已读 */
  const openDetail = useCallback(async (row: RiskRecord) => {
    setDetail(row)
    setDetailOpen(true)
    if (row.id && !row.read) {
      try {
        await RiskController.markRiskRead(row.id)
        void fetchData()
      } catch {
        /* 已读标记失败不阻塞详情查看 */
      }
    }
  }, [fetchData])

  // 跳转带入 risk_id 时自动打开该风险详情
  useEffect(() => {
    if (!jumpRiskID) return
    const load = async () => {
      try {
        const res = await RiskController.getRisk(jumpRiskID)
        if (res.data) await openDetail(res.data)
      } catch {
        message.error('获取风险详情失败')
      }
    }
    void load()
    // 仅在跳转参数变化时触发
  }, [jumpRiskID])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setDeviceCode(values.device_code ?? '')
    setStatus(values.status || undefined)
    setAccurate(values.detect_accurate ?? undefined)
    setLevel(values.risk_level || undefined)
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setDeviceCode('')
    setDeviceID(undefined)
    setRiskTypeID(undefined)
    setRegionID(undefined)
    setStatus('pending')
    setAccurate(undefined)
    setLevel(undefined)
    setRange(null)
    setPage(1)
  }

  /** 点击「设备名称」→ 跳转设备监控并按该设备筛选 */
  const gotoDeviceMonitor = (row: RiskRecord) => {
    const params = new URLSearchParams()
    if (row.device_id) params.set('device_id', String(row.device_id))
    if (row.device?.name) params.set('device_name', row.device.name)
    navigate(`/device/monitor?${params.toString()}`)
  }

  const openHandle = (row: RiskRecord) => {
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
      const payload: HandleRiskReq = { handle_result: values.handle_result, handle_image: handleImage }
      await RiskController.handleRisk(handleTarget.id, payload)
      message.success('处理成功，状态已转为已处理')
      setHandleOpen(false)
      setDetailOpen(false)
      void fetchData()
    } catch {
      message.error('处理失败')
    } finally {
      setSaving(false)
    }
  }

  /** 检测准确：否 → 直接结案；是 → 仅转已读待处理 */
  const confirmAccurate = async (row: RiskRecord, value: string) => {
    if (!row.id || !value) return
    try {
      const payload: UpdateAccurateReq = { accurate: value }
      await RiskController.updateRiskAccurate(row.id, payload)
      message.success(value === 'no' ? '已标记为误报' : '已确认检测准确，可继续处理')
      setDetailOpen(false)
      void fetchData()
    } catch {
      message.error('确认检测准确性失败')
    }
  }

  const handleDelete = async (row: RiskRecord) => {
    if (!row.id) return
    try {
      await RiskController.deleteRisk(row.id)
      message.success('删除成功')
      void fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  const columns: ColumnsType<RiskRecord> = [
    {
      title: '风险类型',
      dataIndex: ['risk_type', 'name'],
      width: 150,
      render: (_: unknown, row) => (
        <a onClick={() => { setRiskTypeID(row.risk_type_id); setPage(1) }}>{row.risk_type?.name ?? '-'}</a>
      ),
    },
    {
      title: '风险图片',
      dataIndex: 'image_url',
      width: 90,
      render: (value: unknown, row) =>
        value ? (
          <Image
            src={String(value)}
            width={48}
            height={48}
            style={{ objectFit: 'cover', cursor: 'pointer' }}
            preview={false}
            onClick={() => setImagePreview({ open: true, url: String(value), title: `风险图片 #${row.id ?? ''}` })}
          />
        ) : (
          <span>-</span>
        ),
    },
    {
      title: '设备名称',
      dataIndex: ['device', 'name'],
      width: 150,
      ellipsis: true,
      render: (_: unknown, row) =>
        row.device?.name ? <a onClick={() => gotoDeviceMonitor(row)}>{row.device.name}</a> : '-',
    },
    {
      title: '设备编号',
      dataIndex: 'device_code',
      width: 150,
      ellipsis: true,
      render: (value: unknown) => (value ? <a onClick={() => { setDeviceCode(String(value)); setPage(1) }}>{String(value)}</a> : '-'),
    },
    { title: '所属区域', dataIndex: ['region', 'name'], width: 140, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '发生时间', dataIndex: 'occurred_dt', width: 170, render: (v?: string) => formatDT(v) },
    {
      title: '检测准确',
      dataIndex: 'detect_accurate',
      width: 100,
      render: (v?: string) => (v === 'yes' ? '是' : v === 'no' ? '否' : '待确认'),
    },
    {
      title: '消息通知',
      dataIndex: 'read',
      width: 100,
      render: (v?: boolean) => <StatusTag type="risk" status={v ? 'accurate' : 'pending'} />,
    },
    { title: '状态', dataIndex: 'status', width: 100, render: (v?: string) => <StatusTag type="risk" status={v} /> },
    {
      title: '操作',
      key: 'action',
      width: 320,
      fixed: 'right',
      render: (_: unknown, row) => (
        <ActionButtons maxWidth={320}>
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
          <Button
            type="link"
            size="small"
            icon={<VideoCameraOutlined />}
            disabled={!row.video_url}
            onClick={() => setVideo({ open: true, url: row.video_url ?? '', title: '风险录像' })}
          >
            查看录像
          </Button>
          <Button type="link" size="small" icon={<MonitorOutlined />} onClick={() => gotoDeviceMonitor(row)}>
            设备监控
          </Button>
          <Button
            type="link"
            size="small"
            disabled={!row.device?.stream_url}
            onClick={() => setVideo({ open: true, url: row.device?.stream_url ?? '', title: '实时监控', live: true })}
          >
            实时监控
          </Button>
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
      title="风险监控处置"
      subTitle="待处理风险处置队列：复核检测准确、填写处理结果、跳转设备监控；全部历史见「AI风险台账」"
      extra={
        <Space wrap>
          <Segmented
            value={status === 'pending' ? 'pending' : 'all'}
            options={[
              { label: '待处理', value: 'pending' },
              { label: '全部', value: 'all' },
            ]}
            onChange={(v) => { setStatus(v === 'pending' ? 'pending' : undefined); setPage(1) }}
          />
          <span style={{ color: '#888' }}>刷新频率</span>
          <Select
            style={{ width: 140 }}
            value={refreshSec}
            options={REFRESH_INTERVAL_OPTIONS}
            onChange={(v: number) => setRefreshSec(v)}
          />
          <Button onClick={() => void fetchData()}>刷新</Button>
        </Space>
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="本页默认只显示未处理风险"
        description="处置完成后记录会从队列消失；如需按时间追溯全部风险（含已处理、误报），请前往「AI风险台账」。"
        action={
          <Button size="small" onClick={() => navigate('/risk/ai-risk')}>
            查看全部历史
          </Button>
        }
      />
      <SearchBar
        fields={[
          { name: 'keyword', label: '设备名称', placeholder: '请输入设备名称' },
          { name: 'device_code', label: '设备编号', placeholder: '请输入设备编号' },
          { name: 'status', label: '状态', type: 'select', options: HANDLE_STATUS_OPTIONS },
          { name: 'detect_accurate', label: '检测准确', type: 'select', options: DETECT_ACCURATE_OPTIONS },
          { name: 'risk_level', label: '风险等级', type: 'select', options: LEVEL_OPTIONS },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Space wrap style={{ marginBottom: 16 }}>
        <RiskTypeSelect
          style={{ width: 180 }}
          value={riskTypeID}
          onChange={(v) => { setRiskTypeID(v === undefined || v === null ? undefined : Number(v)); setPage(1) }}
        />
        <RegionSelect
          style={{ width: 180 }}
          value={regionID}
          onChange={(v) => { setRegionID(v === undefined || v === null ? undefined : Number(v)); setPage(1) }}
        />
        <RangePicker
          showTime
          value={range}
          placeholder={['发生时间起', '发生时间止']}
          onChange={(dates) => { setRange(dates); setPage(1) }}
        />
      </Space>

      <Table<RiskRecord>
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

      {/* 风险详情（打开即把未读消息通知转为已读） */}
      <Modal
        className="e2e-risk-detail-modal"
        title={`风险详情${detail?.id ? ` #${detail.id}` : ''}`}
        open={detailOpen}
        onCancel={() => setDetailOpen(false)}
        footer={
          <Space>
            <Button onClick={() => setDetailOpen(false)}>关闭</Button>
            {canAccurate && detail?.status !== 'handled' && (
              <Radio.Group
                value={detail?.detect_accurate || ''}
                onChange={(e) => detail && confirmAccurate(detail, String(e.target.value))}
              >
                <Radio.Button value="yes">检测准确</Radio.Button>
                <Radio.Button value="no">误报</Radio.Button>
              </Radio.Group>
            )}
            {canHandle && (
              <Button type="primary" disabled={detail?.status === 'handled'} onClick={() => detail && openHandle(detail)}>
                处理
              </Button>
            )}
          </Space>
        }
        width={820}
      >
        {detail && (
          <>
            <Descriptions column={2} size="small" bordered>
              <Descriptions.Item label="风险类型">{detail.risk_type?.name ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="风险等级">{detail.risk_level ? LEVEL_LABELS[detail.risk_level] ?? detail.risk_level : '-'}</Descriptions.Item>
              <Descriptions.Item label="设备名称">{detail.device?.name ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="设备编号">{detail.device_code || '-'}</Descriptions.Item>
              <Descriptions.Item label="所属区域">{detail.region?.name ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="设备位置">{detail.location || '-'}</Descriptions.Item>
              <Descriptions.Item label="发生时间">{formatDT(detail.occurred_dt)}</Descriptions.Item>
              <Descriptions.Item label="AI置信度">
                {detail.detect_score === undefined || detail.detect_score === null
                  ? '-'
                  : `${(detail.detect_score * 100).toFixed(1)}%`}
              </Descriptions.Item>
              <Descriptions.Item label="检测准确">
                {detail.detect_accurate === 'yes' ? '是' : detail.detect_accurate === 'no' ? '否' : '待确认'}
              </Descriptions.Item>
              <Descriptions.Item label="确认人">{detail.accurate_by || '-'}</Descriptions.Item>
              <Descriptions.Item label="消息通知">
                <StatusTag type="risk" status={detail.read ? 'accurate' : 'pending'} />
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <StatusTag type="risk" status={detail.status} />
              </Descriptions.Item>
              <Descriptions.Item label="处理人">{detail.handler || '-'}</Descriptions.Item>
              <Descriptions.Item label="处理时间">{formatDT(detail.handled_dt)}</Descriptions.Item>
              <Descriptions.Item label="处理结果" span={2}>
                {detail.handle_result || '-'}
              </Descriptions.Item>
            </Descriptions>
            <div style={{ marginTop: 16, display: 'flex', gap: 24, flexWrap: 'wrap' }}>
              <div>
                <div style={{ marginBottom: 8 }}>风险图片</div>
                {detail.image_url ? (
                  <Image
                    src={detail.image_url}
                    width={120}
                    style={{ cursor: 'pointer' }}
                    preview={false}
                    onClick={() => setImagePreview({ open: true, url: detail.image_url ?? '', title: `风险图片 #${detail.id ?? ''}` })}
                  />
                ) : (
                  <span>-</span>
                )}
              </div>
              <div>
                <div style={{ marginBottom: 8 }}>处理图片</div>
                {detail.handle_image ? <Image src={detail.handle_image} width={120} /> : <span>-</span>}
              </div>
            </div>
          </>
        )}
      </Modal>

      {/* 风险图片放大预览 */}
      <Modal
        className="e2e-risk-image-modal"
        title={imagePreview.title ?? '风险图片'}
        open={imagePreview.open}
        onCancel={() => setImagePreview(CLOSED_IMAGE)}
        footer={null}
        width={760}
      >
        {imagePreview.url ? <Image src={imagePreview.url} style={{ width: '100%' }} alt="风险图片" /> : <span>无图片</span>}
      </Modal>

      {/* 风险处理 */}
      <Modal
        className="e2e-risk-handle-modal"
        title={`处理风险${handleTarget?.id ? ` #${handleTarget.id}` : ''}`}
        open={handleOpen}
        onCancel={() => setHandleOpen(false)}
        onOk={submitHandle}
        confirmLoading={saving}
        destroyOnHidden
        width={640}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="handle_result" label="处理结果" rules={[{ required: true, message: '请填写处理结果' }]}>
            <Input.TextArea rows={4} maxLength={500} showCount placeholder="请填写现场处理情况" />
          </Form.Item>
          <Form.Item label="处理图片">
            <ImageUpload value={handleImage} onChange={(v) => setHandleImage(Array.isArray(v) ? (v[0] ?? '') : v)} maxCount={1} />
          </Form.Item>
        </Form>
      </Modal>

      <VideoPlayer
        open={video.open}
        src={video.url}
        title={video.title}
        live={video.live}
        onCancel={() => setVideo(CLOSED_VIDEO)}
      />
    </PageContainer>
  )
}

export default RiskMonitorList
