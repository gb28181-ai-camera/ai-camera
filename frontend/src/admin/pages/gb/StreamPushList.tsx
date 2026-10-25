import {
  forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState,
} from 'react'
import {
  Alert, Button, Col, Form, Input, Modal, Popconfirm, Row, Select, Space, Spin, Switch, Table, Tag, Upload,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { UploadFile } from 'antd/es/upload/interface'
import {
  CopyOutlined, DeleteOutlined, EditOutlined, InboxOutlined, PauseCircleOutlined, PlayCircleOutlined,
  PlusOutlined, UploadOutlined,
} from '@ant-design/icons'
import dayjs from 'dayjs'
import axios from 'axios'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import LivePlayerTabs from '../../components/LivePlayerTabs'
import { GbStreamPushController } from '../../api/generated/GbStreamPushController'
import { GbMediaServerController } from '../../api/generated/GbMediaServerController'
import { GbOpsController } from '../../api/generated/GbOpsController'
import { GbStreamImportController } from '../../api/generated/GbStreamImportController'
import type { GbStreamPushItem } from '../../api/generated/definitions/GbStreamPushItem'
import type { GbStreamPlayResp } from '../../api/generated/definitions/GbStreamPlayResp'
import type { GbStreamImportResultResp } from '../../api/generated/definitions/GbStreamImportResultResp'
import type { GbStreamImportRowError } from '../../api/generated/definitions/GbStreamImportRowError'
import type { CreateGbStreamPushReq } from '../../api/generated/definitions/CreateGbStreamPushReq'
import type { UpdateGbStreamPushReq } from '../../api/generated/definitions/UpdateGbStreamPushReq'
import { useAuthStore } from '../../store/authStore'

/** 推流配置表单值 */
interface StreamPushFormValues {
  name: string
  app: string
  stream: string
  active?: boolean
  media_server_id?: string
  self_push?: boolean
  start_offline_push?: boolean
  remark?: string
}

interface MediaServerOption {
  label: string
  value: string
}

/** 时间格式化 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = dayjs(v)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : v
}

/** 请求失败提示：优先展示后端 message（409 会说明推流未启用 / 节点不可达） */
function apiErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError<{ message?: string }>(err)) {
    return err.response?.data?.message || err.message || fallback
  }
  return err instanceof Error ? err.message : fallback
}

/** 播放地址：只透出实际返回的协议地址，空值不占用引擎位 */
function buildPlayUrls(
  info: GbStreamPlayResp | null,
): Partial<Record<'webrtc' | 'flv' | 'hls', string>> | undefined {
  if (!info) return undefined
  const urls: Partial<Record<'webrtc' | 'flv' | 'hls', string>> = {}
  if (info.webrtc) urls.webrtc = info.webrtc
  if (info.flv) urls.flv = info.flv
  if (info.hls) urls.hls = info.hls
  return urls
}

/** 导入失败行表格列 */
const importErrorColumns: ColumnsType<GbStreamImportRowError> = [
  { title: '行号', dataIndex: 'line', width: 80, render: (v?: number) => (v ?? '-') },
  { title: '名称', dataIndex: 'name', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
  { title: '应用名', dataIndex: 'app', width: 110, ellipsis: true, render: (v?: string) => v || '-' },
  { title: '流 ID', dataIndex: 'stream', width: 150, ellipsis: true, render: (v?: string) => v || '-' },
  { title: '失败原因', dataIndex: 'reason', ellipsis: true, render: (v?: string) => v || '-' },
]

/** 新建 / 编辑的初始值。 */
function buildStreamPushInitialValues(editing: GbStreamPushItem | null): Partial<StreamPushFormValues> {
  if (!editing) {
    return { app: 'push', active: true, self_push: false, start_offline_push: false }
  }
  return {
    name: editing.name ?? '',
    app: editing.app ?? '',
    stream: editing.stream ?? '',
    active: editing.active ?? true,
    media_server_id: editing.media_server_id ?? undefined,
    self_push: editing.self_push ?? false,
    start_offline_push: editing.start_offline_push ?? false,
    remark: editing.remark ?? '',
  }
}

interface StreamPushFormHandle {
  submit: () => Promise<void>
}

interface StreamPushFormContentProps {
  editing: GbStreamPushItem | null
  mediaServerOptions: MediaServerOption[]
  onCopy: (url?: string) => void
  onSubmit: (values: StreamPushFormValues) => Promise<void>
}

/**
 * 推流配置表单内容。
 *
 * form 与 Form 同在 Modal children 中挂载，父组件不再提前调用
 * resetFields/setFieldsValue，消除 useForm 未连接告警。
 */
const StreamPushFormContent = forwardRef<StreamPushFormHandle, StreamPushFormContentProps>(
  function StreamPushFormContent({ editing, mediaServerOptions, onCopy, onSubmit }, ref) {
    const [form] = Form.useForm<StreamPushFormValues>()

    useImperativeHandle(ref, () => ({
      async submit() {
        let values: StreamPushFormValues
        try {
          values = await form.validateFields()
        } catch {
          return
        }
        await onSubmit(values)
      },
    }), [form, onSubmit])

    return (
      <Form form={form} layout="vertical" initialValues={buildStreamPushInitialValues(editing)}>
        {editing ? (
          <Form.Item label="推流地址（平台生成）" extra="由媒体节点配置自动生成，不可编辑">
            <Space.Compact style={{ width: '100%' }}>
              <Input value={editing.push_url ?? ''} readOnly placeholder="暂无可用地址" />
              <Button icon={<CopyOutlined />} onClick={() => onCopy(editing.push_url)}>
                复制
              </Button>
            </Space.Compact>
          </Form.Item>
        ) : null}
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
              <Input placeholder="如：大门编码器" maxLength={255} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="app"
              label="应用名"
              rules={[{ required: true, message: '请输入应用名' }]}
              extra="ZLM 的 app 名，与流 ID 组合后唯一"
            >
              <Input placeholder="如：push" maxLength={50} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="stream" label="流 ID" rules={[{ required: true, message: '请输入流 ID' }]}>
              <Input placeholder="如：encoder01" maxLength={100} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="media_server_id" label="媒体节点" extra="推流地址按该节点生成，留空使用默认节点">
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                placeholder="默认节点"
                options={mediaServerOptions}
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="active" label="启用配置" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="self_push"
              label="本平台自身推流"
              valuePropName="checked"
              extra="推流源来自本平台自身的通道"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="start_offline_push"
              label="离线自动推流"
              valuePropName="checked"
              extra="流离线后自动重新推流"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item name="remark" label="备注">
              <Input.TextArea rows={2} maxLength={500} showCount placeholder="请输入备注" />
            </Form.Item>
          </Col>
        </Row>
      </Form>
    )
  },
)

/**
 * 推流配置。
 *
 * ★ 本期后端没有「向外部持续推流」的能力：Start/Stop 只维护配置状态，
 * 真正的推流由外部编码器按平台生成的 push_url 推入 ZLM。
 * 因此页面文案统一使用「启用/停用配置」，不使用「正在推流」。
 */
export default function StreamPushList() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('gb:stream-push:create')
  const canUpdate = permCodes.includes('gb:stream-push:update')
  const canDelete = permCodes.includes('gb:stream-push:delete')
  const canStart = permCodes.includes('gb:stream-push:start')
  const canStop = permCodes.includes('gb:stream-push:stop')
  const canPlay = permCodes.includes('gb:stream-push:play')
  const canImport = permCodes.includes('gb:stream-push:import')

  /** 媒体节点下拉（server_id 为接口入参） */
  const [mediaServerOptions, setMediaServerOptions] = useState<MediaServerOption[]>([])

  useEffect(() => {
    void GbMediaServerController.listGbMediaServers()
      .then((res) => {
        setMediaServerOptions((res.data ?? []).flatMap((item) => (
          item.server_id
            ? [{ label: `${item.name || item.server_id}（${item.server_id}）`, value: item.server_id }]
            : []
        )))
      })
      .catch(() => {
        /* 拦截器已提示；下拉为空时仍可使用默认节点 */
      })
  }, [])

  const [data, setData] = useState<GbStreamPushItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState<string | undefined>(undefined)

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<GbStreamPushItem | null>(null)
  const [saving, setSaving] = useState(false)
  const [actingId, setActingId] = useState<number | null>(null)
  const formRef = useRef<StreamPushFormHandle>(null)

  /* ── 内嵌播放 ── */
  const [playOpen, setPlayOpen] = useState(false)
  const [playLoading, setPlayLoading] = useState(false)
  const [playInfo, setPlayInfo] = useState<GbStreamPlayResp | null>(null)
  const [playError, setPlayError] = useState('')
  const [playTitle, setPlayTitle] = useState('')

  /* ── 批量导入 ── */
  const [importOpen, setImportOpen] = useState(false)
  const [importFile, setImportFile] = useState<File | null>(null)
  const [importFileList, setImportFileList] = useState<UploadFile[]>([])
  const [importing, setImporting] = useState(false)
  const [importResult, setImportResult] = useState<GbStreamImportResultResp | null>(null)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbStreamPushController.pageGbStreamPushes(
        page,
        pageSize,
        keyword || undefined,
        undefined,
        undefined,
        status,
      )
      if (res.code === 200) {
        setData(res.data?.items ?? [])
        setTotal(res.data?.total ?? 0)
      } else {
        message.error(res.message || '获取推流配置失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, status])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const handleSearch = (values: Record<string, unknown>) => {
    setKeyword(String(values.keyword ?? ''))
    const s = values.status
    setStatus(s === 'ON' || s === 'OFF' ? s : undefined)
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setStatus(undefined)
    setPage(1)
  }

  const handleCopy = async (url?: string) => {
    if (!url) {
      message.warning('该配置暂无可用的推流地址')
      return
    }
    try {
      await navigator.clipboard.writeText(url)
      message.success('推流地址已复制')
    } catch {
      message.warning('当前环境不支持自动复制，请手动选中复制')
    }
  }

  const openCreate = () => {
    setEditing(null)
    setModalOpen(true)
  }

  const openEdit = (row: GbStreamPushItem) => {
    setEditing(row)
    setModalOpen(true)
  }

  const handleSave = async (values: StreamPushFormValues) => {
    const payload: CreateGbStreamPushReq = {
      name: values.name.trim(),
      app: values.app.trim(),
      stream: values.stream.trim(),
      active: values.active,
      media_server_id: values.media_server_id?.trim() || undefined,
      self_push: values.self_push,
      start_offline_push: values.start_offline_push,
      remark: values.remark,
    }

    setSaving(true)
    try {
      if (editing?.id) {
        const updatePayload: UpdateGbStreamPushReq = payload
        const res = await GbStreamPushController.updateGbStreamPush(String(editing.id), updatePayload)
        if (res.code === 200) {
          message.success('修改成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '保存失败')
        }
      } else {
        const res = await GbStreamPushController.createGbStreamPush(payload)
        if (res.code === 200) {
          message.success('新增成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '保存失败')
        }
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (row: GbStreamPushItem) => {
    if (!row.id) return
    try {
      const res = await GbStreamPushController.deleteGbStreamPush(String(row.id))
      if (res.code === 200) {
        message.success('删除成功')
        fetchData()
      } else {
        message.error(res.message || '删除失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    }
  }

  const handleStart = async (row: GbStreamPushItem) => {
    if (!row.id) return
    setActingId(row.id)
    try {
      const res = await GbStreamPushController.startGbStreamPush(String(row.id))
      if (res.code === 200) {
        message.success('已启用推流配置')
        fetchData()
      } else {
        message.error(res.message || '启用失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setActingId(null)
    }
  }

  const handleStop = async (row: GbStreamPushItem) => {
    if (!row.id) return
    setActingId(row.id)
    try {
      const res = await GbStreamPushController.stopGbStreamPush(String(row.id))
      if (res.code === 200) {
        message.success('已停用推流配置')
        fetchData()
      } else {
        message.error(res.message || '停用失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setActingId(null)
    }
  }

  /** 播放：推流未启用 / 流不在节点上时后端返回 409，弹窗内展示后端 message */
  const handlePlay = async (row: GbStreamPushItem) => {
    if (!row.id) return
    setPlayTitle(row.name || row.stream || '推流配置')
    setPlayOpen(true)
    setPlayLoading(true)
    setPlayInfo(null)
    setPlayError('')
    try {
      const res = await GbOpsController.playGbStreamPush(String(row.id))
      if (res.data) {
        setPlayInfo(res.data)
      } else {
        setPlayError(res.message || '后端未返回播放地址')
      }
    } catch (err) {
      setPlayError(apiErrorMessage(err, '获取播放地址失败（请先启用推流配置）'))
    } finally {
      setPlayLoading(false)
    }
  }

  const openImport = () => {
    setImportFile(null)
    setImportFileList([])
    setImportResult(null)
    setImportOpen(true)
  }

  const closeImport = () => {
    if (importing) return
    setImportOpen(false)
    setImportFile(null)
    setImportFileList([])
    setImportResult(null)
  }

  const handleImport = async () => {
    if (!importFile) {
      message.warning('请选择要导入的 CSV / TSV 文件')
      return
    }
    const formData = new FormData()
    formData.append('file', importFile, importFile.name)

    setImporting(true)
    setImportResult(null)
    try {
      // request 实例默认 Content-Type 为 application/json，会让 axios 把 FormData
      // 序列化成 JSON；显式声明 multipart 后 axios 在浏览器环境会移除该头，
      // 由浏览器补上带 boundary 的 multipart/form-data。
      const res = await GbStreamImportController.importGbStreamPushes(
        formData,
        { 'Content-Type': 'multipart/form-data' },
      )
      const result = res.data ?? null
      setImportResult(result)
      const successCount = result?.success_count ?? 0
      const failedCount = result?.failed_count ?? 0
      if (successCount > 0) {
        message.success(`成功导入 ${successCount} 条推流配置`)
        void fetchData()
      }
      if (failedCount > 0) message.warning(`${failedCount} 行导入失败，请查看失败明细`)
      if (successCount === 0 && failedCount === 0) message.info('文件中没有可导入的数据')
    } catch (err) {
      message.error(apiErrorMessage(err, '导入失败'))
    } finally {
      setImporting(false)
    }
  }

  const columns: ColumnsType<GbStreamPushItem> = [
    { title: '名称', dataIndex: 'name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '应用名', dataIndex: 'app', width: 120, render: (v?: string) => v || '-' },
    { title: '流 ID', dataIndex: 'stream', width: 150, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '媒体节点', dataIndex: 'media_server_id', width: 160, ellipsis: true,
      render: (v?: string) => v || <span style={{ color: '#999' }}>默认节点</span>,
    },
    {
      title: '推流地址', dataIndex: 'push_url', width: 360,
      render: (v?: string) => (
        v ? (
          <Space size={4}>
            <span style={{ fontFamily: 'Consolas, Monaco, monospace' }}>{v}</span>
            <Button type="link" size="small" icon={<CopyOutlined />} onClick={() => handleCopy(v)}>
              复制
            </Button>
          </Space>
        ) : (
          <span style={{ color: '#999' }}>暂无（请检查媒体节点配置）</span>
        )
      ),
    },
    {
      title: '状态', dataIndex: 'status', width: 110,
      render: (v?: string) => (
        v === 'ON' ? <Tag color="green">已启用</Tag> : <Tag color="default">已停用</Tag>
      ),
    },
    {
      title: '启用配置', dataIndex: 'active', width: 100,
      render: (v?: boolean) => (v ? <Tag color="blue">启用</Tag> : <Tag>停用</Tag>),
    },
    { title: '备注', dataIndex: 'remark', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '启用时间', dataIndex: 'push_time', width: 170, render: (v?: string) => fmtDT(v) },
    {
      title: '操作', key: 'action', width: 380, fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={380}>
          {canPlay && (
            <Button
              className="e2e-gb-stream-push-play-btn"
              type="link"
              size="small"
              icon={<PlayCircleOutlined />}
              onClick={() => void handlePlay(row)}
            >
              播放
            </Button>
          )}
          {canStart && row.status !== 'ON' && (
            <Button
              type="link"
              size="small"
              icon={<PlayCircleOutlined />}
              loading={actingId === row.id}
              onClick={() => handleStart(row)}
            >
              启用
            </Button>
          )}
          {canStop && row.status === 'ON' && (
            <Button
              type="link"
              size="small"
              icon={<PauseCircleOutlined />}
              loading={actingId === row.id}
              onClick={() => handleStop(row)}
            >
              停用
            </Button>
          )}
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
              编辑
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除该推流配置?" onConfirm={() => handleDelete(row)}>
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
      title="推流"
      subTitle="配置外部编码器推流到流媒体节点，推流地址由平台按节点自动生成"
      extra={
        <Space>
          {canImport && (
            <Button
              className="e2e-gb-stream-push-import-btn"
              icon={<UploadOutlined />}
              onClick={openImport}
            >
              导入
            </Button>
          )}
          {canCreate && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新增推流
            </Button>
          )}
        </Space>
      }
    >
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 12 }}
        message="启用 / 停用仅维护配置状态，当前不会向外部目标推流"
        description="后端尚未接入持续推流能力：Start/Stop 只把配置置为 ON/OFF，并不会真正向外部推流。请复制下方 push_url，在外部编码器中手动配置推流。"
      />

      <SearchBar
        fields={[
          { name: 'keyword', label: '关键字', placeholder: '名称 / app / 流ID' },
          {
            name: 'status',
            label: '状态',
            type: 'select',
            placeholder: '全部状态',
            options: [
              { label: '已启用', value: 'ON' },
              { label: '已停用', value: 'OFF' },
            ],
          },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<GbStreamPushItem>
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
          onChange: (p, ps) => {
            setPage(p)
            setPageSize(ps)
          },
        }}
      />

      <Modal
        className="e2e-gb-stream-push-modal"
        title={editing ? '编辑推流配置' : '新增推流配置'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => void formRef.current?.submit()}
        confirmLoading={saving}
        destroyOnHidden
        width={640}
      >
        <StreamPushFormContent
          ref={formRef}
          editing={editing}
          mediaServerOptions={mediaServerOptions}
          onCopy={(url) => void handleCopy(url)}
          onSubmit={handleSave}
        />
      </Modal>

      {/* 内嵌播放：地址来自 playGbStreamPush，409 时展示后端 message */}
      <Modal
        className="e2e-gb-stream-push-play-modal"
        title={`播放 - ${playTitle}`}
        open={playOpen}
        footer={null}
        width={900}
        destroyOnHidden
        onCancel={() => {
          setPlayOpen(false)
          setPlayInfo(null)
          setPlayError('')
        }}
      >
        {playError ? (
          <Alert
            type="error"
            showIcon
            message="无法播放"
            description={playError}
            style={{ marginBottom: 8 }}
          />
        ) : null}
        <div
          style={{
            height: 480,
            background: '#000',
            borderRadius: 6,
            overflow: 'hidden',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          {playLoading ? (
            <Spin />
          ) : playInfo ? (
            <LivePlayerTabs
              app={playInfo.app}
              stream={playInfo.stream}
              urls={buildPlayUrls(playInfo)}
              objectFit="contain"
              style={{ width: '100%', height: '100%' }}
              onError={(error) => setPlayError(error.message)}
            />
          ) : (
            <span style={{ color: '#8c8c8c' }}>暂无播放地址</span>
          )}
        </div>
      </Modal>

      {/* 推流配置批量导入（CSV / TSV） */}
      <Modal
        className="e2e-gb-stream-push-import-modal"
        title="导入推流配置"
        open={importOpen}
        onCancel={closeImport}
        onOk={() => void handleImport()}
        okText="开始导入"
        confirmLoading={importing}
        width={780}
        maskClosable={!importing}
        destroyOnHidden
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="支持 CSV（逗号）与 TSV（制表符），单次最多 1000 行、4MB"
          description="列：name/名称、app/应用名、stream/流ID、media_server_id/媒体节点、push_url/推流地址、gb_channel_id/通道编码、remark/备注。无表头时按 name,app,stream,media_server_id,remark 顺序解析；不支持 xlsx。"
        />
        <Upload.Dragger
          accept=".csv,.tsv,text/csv,text/tab-separated-values"
          maxCount={1}
          fileList={importFileList}
          disabled={importing}
          beforeUpload={(file, list) => {
            setImportFileList(list.slice(-1))
            setImportFile(file)
            return false
          }}
          onRemove={() => {
            setImportFileList([])
            setImportFile(null)
          }}
        >
          <p className="ant-upload-drag-icon"><InboxOutlined /></p>
          <p className="ant-upload-text">点击或拖拽 CSV / TSV 文件到此处</p>
          <p className="ant-upload-hint">文件只在点击「开始导入」后上传</p>
        </Upload.Dragger>

        {importResult ? (
          <div className="e2e-gb-stream-push-import-result" style={{ marginTop: 16 }}>
            <Alert
              type={(importResult.failed_count ?? 0) > 0 ? 'warning' : 'success'}
              showIcon
              style={{ marginBottom: 12 }}
              message={`共 ${importResult.total ?? 0} 行：成功 ${importResult.success_count ?? 0}，失败 ${importResult.failed_count ?? 0}`}
            />
            {(importResult.errors?.length ?? 0) > 0 ? (
              <Table<GbStreamImportRowError>
                size="small"
                rowKey={(row) => `${row.line ?? row.row ?? ''}-${row.stream ?? ''}-${row.reason ?? ''}`}
                columns={importErrorColumns}
                dataSource={importResult.errors}
                pagination={false}
                scroll={{ y: 240 }}
                locale={{ emptyText: '无失败明细' }}
              />
            ) : null}
          </div>
        ) : null}
      </Modal>
    </PageContainer>
  )
}
