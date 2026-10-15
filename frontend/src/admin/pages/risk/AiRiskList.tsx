import React, { useCallback, useEffect, useState } from 'react'
import {
  Badge, Button, DatePicker, Form, Image, Input, message, Modal, Popconfirm, Radio, Space, Table,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  DeleteOutlined, EyeOutlined, ToolOutlined, VideoCameraOutlined, CheckCircleOutlined,
} from '@ant-design/icons'
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
  formatDT, LEVEL_LABELS,
} from '../../types/business'
import type { ImagePreviewState, VideoState } from '../../types/business'

const { RangePicker } = DatePicker

/**
 * AI风险：全部 AI 风险信息列表与处理。
 *
 * 关键分支（对应操作手册）：
 * - 检测准确点「否」→ 直接置为已处理（误报结案）；
 * - 检测准确点「是」→ 仅标记消息已读，状态仍为待处理，随后可点「处理」填写处理结果与图片。
 */
const AiRiskList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canHandle = permCodes.includes('risk:handle')
  const canAccurate = permCodes.includes('risk:accurate')
  const canDelete = permCodes.includes('risk:delete')

  const [data, setData] = useState<RiskRecord[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [pendingCount, setPendingCount] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  // 搜索条件（关键字与下拉条件分开维护，便于点击单元格筛选时局部覆盖）
  const [keyword, setKeyword] = useState('')
  const [deviceCode, setDeviceCode] = useState('')
  const [riskTypeID, setRiskTypeID] = useState<number | undefined>()
  const [regionID, setRegionID] = useState<number | undefined>()
  const [status, setStatus] = useState<string | undefined>()
  const [accurate, setAccurate] = useState<string | undefined>()
  const [level, setLevel] = useState<string | undefined>()
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null)

  // 弹窗状态
  const [imagePreview, setImagePreview] = useState<ImagePreviewState>(CLOSED_IMAGE)
  const [video, setVideo] = useState<VideoState>(CLOSED_VIDEO)
  const [handleOpen, setHandleOpen] = useState(false)
  const [handleTarget, setHandleTarget] = useState<RiskRecord | null>(null)
  const [handleImage, setHandleImage] = useState('')
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<HandleRiskReq>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const query: PageRiskReq = {
        page,
        page_size: pageSize,
        keyword: keyword || undefined,
        device_code: deviceCode || undefined,
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
  }, [page, pageSize, keyword, deviceCode, riskTypeID, regionID, status, accurate, level, range])

  const fetchPending = useCallback(async () => {
    try {
      const res = await RiskController.riskPendingCount()
      setPendingCount(Number(res.data ?? 0))
    } catch {
      /* 角标失败不阻塞列表 */
    }
  }, [])

  useEffect(() => { void fetchData() }, [fetchData])
  useEffect(() => { void fetchPending() }, [fetchPending])

  /** 重置到第一页（所有筛选条件变化时调用） */
  const resetPage = () => setPage(1)

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setDeviceCode(values.device_code ?? '')
    setStatus(values.status || undefined)
    setAccurate(values.detect_accurate ?? undefined)
    setLevel(values.risk_level || undefined)
    resetPage()
  }

  const handleReset = () => {
    setKeyword('')
    setDeviceCode('')
    setStatus(undefined)
    setAccurate(undefined)
    setLevel(undefined)
    setRiskTypeID(undefined)
    setRegionID(undefined)
    setRange(null)
    resetPage()
  }

  /** 点击「风险类型」单元格筛选该类型 */
  const filterByRiskType = (id?: number) => {
    setRiskTypeID(id)
    resetPage()
  }

  /** 点击「设备编号」单元格筛选该编号 */
  const filterByDeviceCode = (code?: string) => {
    setDeviceCode(code ?? '')
    resetPage()
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
      void fetchData()
      void fetchPending()
    } catch {
      message.error('处理失败')
    } finally {
      setSaving(false)
    }
  }

  /**
   * 检测准确确认。
   * accurate=no：误报，直接结案（已处理）；accurate=yes：仅转已读，仍需处理。
   */
  const confirmAccurate = async (row: RiskRecord, value: string) => {
    if (!row.id || !value) return
    try {
      const payload: UpdateAccurateReq = { accurate: value }
      await RiskController.updateRiskAccurate(row.id, payload)
      message.success(value === 'no' ? '已标记为误报，该条数据已转为已处理' : '已确认检测准确，消息通知转为已读')
      void fetchData()
      void fetchPending()
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
      void fetchPending()
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
        <a onClick={() => filterByRiskType(row.risk_type_id)}>{row.risk_type?.name ?? '-'}</a>
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
    { title: '设备名称', dataIndex: ['device', 'name'], width: 150, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    {
      title: '设备编号',
      dataIndex: 'device_code',
      width: 150,
      ellipsis: true,
      render: (value: unknown) => (value ? <a onClick={() => filterByDeviceCode(String(value))}>{String(value)}</a> : '-'),
    },
    { title: '所属区域', dataIndex: ['region', 'name'], width: 140, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '设备位置', dataIndex: 'location', width: 180, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '发生时间', dataIndex: 'occurred_dt', width: 170, render: (v?: string) => formatDT(v) },
    {
      title: '风险等级',
      dataIndex: 'risk_level',
      width: 100,
      render: (v?: string) => (v ? LEVEL_LABELS[v] ?? v : '-'),
    },
    {
      title: 'AI置信度',
      dataIndex: 'detect_score',
      width: 100,
      render: (v?: number) => (v === undefined || v === null ? '-' : `${(v * 100).toFixed(1)}%`),
    },
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
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (v?: string) => <StatusTag type="risk" status={v} />,
    },
    {
      title: '操作',
      key: 'action',
      width: 330,
      fixed: 'right',
      render: (_: unknown, row) => (
        <ActionButtons maxWidth={320}>
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
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            disabled={!row.device?.stream_url}
            onClick={() => setVideo({ open: true, url: row.device?.stream_url ?? '', title: '实时视频', live: true })}
          >
            查看实时视频
          </Button>
          {canAccurate && (
            <Radio.Group
              size="small"
              value={row.detect_accurate || ''}
              disabled={row.status === 'handled'}
              onChange={(e) => confirmAccurate(row, String(e.target.value))}
            >
              <Radio.Button value="yes">是</Radio.Button>
              <Radio.Button value="no">无</Radio.Button>
            </Radio.Group>
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
      title="AI风险"
      subTitle="设备 AI 识别的全部风险信息，支持检测准确确认与处理闭环"
      extra={
        <Space>
          <Badge count={pendingCount} overflowCount={999} color="red">
            <Button icon={<CheckCircleOutlined />}>待处理</Button>
          </Badge>
        </Space>
      }
    >
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
          onChange={(v) => { setRiskTypeID(v === undefined || v === null ? undefined : Number(v)); resetPage() }}
        />
        <RegionSelect
          style={{ width: 180 }}
          value={regionID}
          onChange={(v) => { setRegionID(v === undefined || v === null ? undefined : Number(v)); resetPage() }}
        />
        <RangePicker
          showTime
          value={range}
          placeholder={['发生时间起', '发生时间止']}
          onChange={(dates) => { setRange(dates); resetPage() }}
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

      {/* 风险图片放大预览 */}
      <Modal
        className="e2e-risk-image-modal"
        title={imagePreview.title ?? '风险图片'}
        open={imagePreview.open}
        onCancel={() => setImagePreview(CLOSED_IMAGE)}
        footer={null}
        width={760}
      >
        {imagePreview.url ? (
          <Image src={imagePreview.url} style={{ width: '100%' }} alt="风险图片" />
        ) : (
          <span>无图片</span>
        )}
      </Modal>

      {/* 风险处理：处理结果 + 处理图片 */}
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
          <Form.Item
            name="handle_result"
            label="处理结果"
            rules={[{ required: true, message: '请填写处理结果' }]}
          >
            <Input.TextArea rows={4} maxLength={500} showCount placeholder="请填写现场处理情况" />
          </Form.Item>
          <Form.Item label="处理图片">
            <ImageUpload value={handleImage} onChange={(v) => setHandleImage(Array.isArray(v) ? (v[0] ?? '') : v)} maxCount={1} />
          </Form.Item>
          {handleTarget?.image_url ? (
            <Form.Item label="风险原图">
              <Image src={handleTarget.image_url} width={120} alt="风险原图" />
            </Form.Item>
          ) : null}
        </Form>
      </Modal>

      {/* 录像 / 实时视频 */}
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

export default AiRiskList
