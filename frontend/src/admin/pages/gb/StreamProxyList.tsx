import {
  forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState,
} from 'react'
import {
  Alert, Button, Col, Divider, Form, Input, InputNumber, Modal, Popconfirm, Row, Select, Spin, Switch,
  Table, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  DeleteOutlined, EditOutlined, PauseCircleOutlined, PlayCircleOutlined, PlusOutlined,
} from '@ant-design/icons'
import dayjs from 'dayjs'
import axios from 'axios'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import LivePlayerTabs from '../../components/LivePlayerTabs'
import { GbStreamProxyController } from '../../api/generated/GbStreamProxyController'
import { GbMediaServerController } from '../../api/generated/GbMediaServerController'
import { GbOpsController } from '../../api/generated/GbOpsController'
import type { GbStreamProxyItem } from '../../api/generated/definitions/GbStreamProxyItem'
import type { GbStreamPlayResp } from '../../api/generated/definitions/GbStreamPlayResp'
import type { CreateGbStreamProxyReq } from '../../api/generated/definitions/CreateGbStreamProxyReq'
import type { UpdateGbStreamProxyReq } from '../../api/generated/definitions/UpdateGbStreamProxyReq'
import { useAuthStore } from '../../store/authStore'

/** 拉流代理表单值 */
interface StreamProxyFormValues {
  name: string
  app: string
  stream: string
  src_url: string
  timeout_ms?: number | null
  type?: string
  ffmpeg_cmd_key?: string
  rtsp_type?: number
  media_server_id?: string
  enable_audio?: boolean
  enable_mp4?: boolean
  enable?: boolean
  enable_disable_none_reader?: boolean
  enable_remove_none_reader?: boolean
  remark?: string
}

/** RTSP 拉流方式（模型注释：0 TCP / 1 UDP） */
const RTSP_TYPE_OPTIONS = [
  { label: 'TCP', value: 0 },
  { label: 'UDP', value: 1 },
]

/** InputNumber 空值统一转 undefined */
function num(v?: number | null): number | undefined {
  return typeof v === 'number' && !Number.isNaN(v) ? v : undefined
}

/** 空串统一转 undefined，避免把空值写进请求体 */
function text(v?: string | null): string | undefined {
  const trimmed = v?.trim()
  return trimmed ? trimmed : undefined
}

/** 请求失败提示：优先展示后端 message（409 会说明流未启动 / 节点不可达） */
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

/** 时间格式化 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = dayjs(v)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : v
}

/** 新建 / 编辑的初始值。 */
function buildStreamProxyInitialValues(editing: GbStreamProxyItem | null): Partial<StreamProxyFormValues> {
  if (!editing) {
    return {
      app: 'proxy',
      timeout_ms: 10000,
      type: 'default',
      rtsp_type: 0,
      enable_audio: true,
      enable_mp4: false,
      enable: true,
      enable_disable_none_reader: false,
      enable_remove_none_reader: false,
    }
  }
  return {
    name: editing.name ?? '',
    app: editing.app ?? '',
    stream: editing.stream ?? '',
    src_url: editing.src_url ?? '',
    timeout_ms: editing.timeout_ms ?? 10000,
    type: editing.type ?? 'default',
    ffmpeg_cmd_key: editing.ffmpeg_cmd_key ?? '',
    rtsp_type: editing.rtsp_type ?? 0,
    media_server_id: editing.media_server_id ?? undefined,
    enable_audio: editing.enable_audio ?? true,
    enable_mp4: editing.enable_mp4 ?? false,
    enable: editing.enable ?? true,
    enable_disable_none_reader: editing.enable_disable_none_reader ?? false,
    enable_remove_none_reader: editing.enable_remove_none_reader ?? false,
    remark: editing.remark ?? '',
  }
}

interface StreamProxyFormHandle {
  submit: () => Promise<void>
}

interface MediaServerOption {
  label: string
  value: string
}

interface StreamProxyFormContentProps {
  editing: GbStreamProxyItem | null
  mediaServerOptions: MediaServerOption[]
  onSubmit: (values: StreamProxyFormValues) => Promise<void>
}

/**
 * 拉流代理表单内容：form 与 Form 同挂载在 Modal children 中，
 * 避免父组件在 Form 渲染前调用 useForm 方法产生告警。
 */
const StreamProxyFormContent = forwardRef<StreamProxyFormHandle, StreamProxyFormContentProps>(
  function StreamProxyFormContent({ editing, mediaServerOptions, onSubmit }, ref) {
    const [form] = Form.useForm<StreamProxyFormValues>()

    useImperativeHandle(ref, () => ({
      async submit() {
        let values: StreamProxyFormValues
        try {
          values = await form.validateFields()
        } catch {
          return
        }
        await onSubmit(values)
      },
    }), [form, onSubmit])

    return (
      <Form form={form} layout="vertical" initialValues={buildStreamProxyInitialValues(editing)}>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
              <Input placeholder="如：大门摄像头拉流" maxLength={255} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="app"
              label="应用名"
              rules={[{ required: true, message: '请输入应用名' }]}
              extra="ZLM 的 app 名，与流 ID 组合后唯一"
            >
              <Input placeholder="如：proxy" maxLength={50} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="stream" label="流 ID" rules={[{ required: true, message: '请输入流 ID' }]}>
              <Input placeholder="如：camera01" maxLength={100} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="timeout_ms" label="拉流超时（毫秒）">
              <InputNumber min={1000} max={120000} step={1000} style={{ width: '100%' }} placeholder="默认 10000" />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item
              name="src_url"
              label="源地址"
              rules={[
                { required: true, message: '请输入源地址' },
                { pattern: /^(rtsp|rtmp|http|https):\/\//i, message: '源地址需以 rtsp://、rtmp:// 或 http(s):// 开头' },
              ]}
              extra="支持 rtsp / rtmp / http(s) 源地址"
            >
              <Input placeholder="如：rtsp://10.0.0.9:554/live" maxLength={500} />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Divider orientation="left" plain style={{ margin: '0 0 12px' }}>
              拉流与转封装
            </Divider>
          </Col>
          <Col span={12}>
            <Form.Item name="type" label="代理类型" extra="留空或 default 表示标准拉流代理">
              <Input placeholder="default" maxLength={20} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="media_server_id" label="媒体节点" extra="留空由后端自动分配（默认节点）">
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                placeholder="自动分配"
                options={mediaServerOptions}
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="rtsp_type" label="RTSP 拉流方式">
              <Select options={RTSP_TYPE_OPTIONS} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="ffmpeg_cmd_key" label="ffmpeg 命令模板键" extra="源需要 ffmpeg 转封装时填写，留空使用默认模板">
              <Input placeholder="留空使用默认模板" maxLength={50} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="enable" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="enable_audio" label="启用音频" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="enable_mp4" label="录制 MP4" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              name="enable_disable_none_reader"
              label="无人观看时禁用"
              valuePropName="checked"
              extra="ZLM 无观看者时暂停拉流"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              name="enable_remove_none_reader"
              label="无人观看时移除"
              valuePropName="checked"
              extra="ZLM 无观看者时释放该流"
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
 * 拉流代理：平台让 ZLM 主动从 rtsp/rtmp/http 源拉流。
 * 新增/编辑只落配置；Start/Stop 才真正调用 ZLM。
 */
export default function StreamProxyList() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('gb:stream-proxy:create')
  const canUpdate = permCodes.includes('gb:stream-proxy:update')
  const canDelete = permCodes.includes('gb:stream-proxy:delete')
  const canStart = permCodes.includes('gb:stream-proxy:start')
  const canStop = permCodes.includes('gb:stream-proxy:stop')
  const canPlay = permCodes.includes('gb:stream-proxy:play')

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
        /* 拦截器已提示；下拉为空时仍可留空自动分配 */
      })
  }, [])

  const [data, setData] = useState<GbStreamProxyItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [pulling, setPulling] = useState<boolean | undefined>(undefined)

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<GbStreamProxyItem | null>(null)
  const [saving, setSaving] = useState(false)
  const [actingId, setActingId] = useState<number | null>(null)
  const formRef = useRef<StreamProxyFormHandle>(null)

  /* ── 内嵌播放 ── */
  const [playOpen, setPlayOpen] = useState(false)
  const [playLoading, setPlayLoading] = useState(false)
  const [playInfo, setPlayInfo] = useState<GbStreamPlayResp | null>(null)
  const [playError, setPlayError] = useState('')
  const [playTitle, setPlayTitle] = useState('')

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbStreamProxyController.pageGbStreamProxies(
        page,
        pageSize,
        keyword || undefined,
        pulling,
        undefined,
      )
      if (res.code === 200) {
        setData(res.data?.items ?? [])
        setTotal(res.data?.total ?? 0)
      } else {
        message.error(res.message || '获取拉流代理失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, pulling])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const handleSearch = (values: Record<string, unknown>) => {
    setKeyword(String(values.keyword ?? ''))
    const p = values.pulling
    setPulling(p === 'true' ? true : p === 'false' ? false : undefined)
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setPulling(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    setModalOpen(true)
  }

  const openEdit = (row: GbStreamProxyItem) => {
    setEditing(row)
    setModalOpen(true)
  }

  const handleSave = async (values: StreamProxyFormValues) => {
    const payload: CreateGbStreamProxyReq = {
      name: values.name.trim(),
      app: values.app.trim(),
      stream: values.stream.trim(),
      src_url: values.src_url.trim(),
      timeout_ms: num(values.timeout_ms),
      type: text(values.type),
      ffmpeg_cmd_key: text(values.ffmpeg_cmd_key),
      rtsp_type: num(values.rtsp_type),
      media_server_id: text(values.media_server_id),
      enable_audio: values.enable_audio,
      enable_mp4: values.enable_mp4,
      enable: values.enable,
      enable_disable_none_reader: values.enable_disable_none_reader,
      enable_remove_none_reader: values.enable_remove_none_reader,
      remark: values.remark,
    }

    setSaving(true)
    try {
      if (editing?.id) {
        const updatePayload: UpdateGbStreamProxyReq = payload
        const res = await GbStreamProxyController.updateGbStreamProxy(
          String(editing.id),
          updatePayload,
        )
        if (res.code === 200) {
          message.success('修改成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '保存失败')
        }
      } else {
        const res = await GbStreamProxyController.createGbStreamProxy(payload)
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

  const handleDelete = async (row: GbStreamProxyItem) => {
    if (!row.id) return
    try {
      const res = await GbStreamProxyController.deleteGbStreamProxy(String(row.id))
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

  const handleStart = async (row: GbStreamProxyItem) => {
    if (!row.id) return
    setActingId(row.id)
    try {
      const res = await GbStreamProxyController.startGbStreamProxy(String(row.id))
      if (res.code === 200) {
        message.success('已启动拉流')
        fetchData()
      } else {
        message.error(res.message || '启动失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setActingId(null)
    }
  }

  const handleStop = async (row: GbStreamProxyItem) => {
    if (!row.id) return
    setActingId(row.id)
    try {
      const res = await GbStreamProxyController.stopGbStreamProxy(String(row.id))
      if (res.code === 200) {
        message.success('已停止拉流')
        fetchData()
      } else {
        message.error(res.message || '停止失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setActingId(null)
    }
  }

  /** 播放：流未启动 / 节点不可达时后端返回 409，弹窗内展示其后端 message */
  const handlePlay = async (row: GbStreamProxyItem) => {
    if (!row.id) return
    setPlayTitle(row.name || row.stream || '拉流代理')
    setPlayOpen(true)
    setPlayLoading(true)
    setPlayInfo(null)
    setPlayError('')
    try {
      const res = await GbOpsController.playGbStreamProxy(String(row.id))
      if (res.data) {
        setPlayInfo(res.data)
      } else {
        setPlayError(res.message || '后端未返回播放地址')
      }
    } catch (err) {
      setPlayError(apiErrorMessage(err, '获取播放地址失败（请先启动拉流代理）'))
    } finally {
      setPlayLoading(false)
    }
  }

  const columns: ColumnsType<GbStreamProxyItem> = [
    { title: '名称', dataIndex: 'name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '应用名', dataIndex: 'app', width: 120, render: (v?: string) => v || '-' },
    { title: '流 ID', dataIndex: 'stream', width: 150, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '源地址', dataIndex: 'src_url', width: 280, ellipsis: true,
      render: (v?: string) => (
        <span style={{ fontFamily: 'Consolas, Monaco, monospace' }} title={v}>{v || '-'}</span>
      ),
    },
    {
      title: '媒体节点', dataIndex: 'media_server_id', width: 160, ellipsis: true,
      render: (v?: string) => v || <span style={{ color: '#999' }}>自动分配</span>,
    },
    {
      title: 'RTSP 方式', dataIndex: 'rtsp_type', width: 100,
      render: (v?: number) => (v === 1 ? 'UDP' : 'TCP'),
    },
    {
      title: '启用', dataIndex: 'enable', width: 90,
      render: (v?: boolean) => (v ? <Tag color="green">启用</Tag> : <Tag>停用</Tag>),
    },
    {
      title: '状态', dataIndex: 'pulling', width: 110,
      render: (v?: boolean) => (
        v ? <Tag color="green">running</Tag> : <Tag color="default">stopped</Tag>
      ),
    },
    { title: '备注', dataIndex: 'remark', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '更新时间', dataIndex: 'updated_dt', width: 170, render: (v?: string) => fmtDT(v) },
    {
      title: '操作', key: 'action', width: 380, fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={380}>
          {canPlay && (
            <Button
              className="e2e-gb-stream-proxy-play-btn"
              type="link"
              size="small"
              icon={<PlayCircleOutlined />}
              onClick={() => void handlePlay(row)}
            >
              播放
            </Button>
          )}
          {canStart && !row.pulling && (
            <Button
              type="link"
              size="small"
              icon={<PlayCircleOutlined />}
              loading={actingId === row.id}
              onClick={() => handleStart(row)}
            >
              启动
            </Button>
          )}
          {canStop && row.pulling && (
            <Button
              type="link"
              size="small"
              icon={<PauseCircleOutlined />}
              loading={actingId === row.id}
              onClick={() => handleStop(row)}
            >
              停止
            </Button>
          )}
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
              编辑
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除该拉流代理?" onConfirm={() => handleDelete(row)}>
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
      title="拉流代理"
      subTitle="让流媒体节点主动从 rtsp/rtmp/http 源拉流；新增后需点击「启动」才会真正拉流"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增拉流代理
          </Button>
        ) : null
      }
    >
      <SearchBar
        fields={[
          { name: 'keyword', label: '关键字', placeholder: '名称 / app / 流ID / 源地址' },
          {
            name: 'pulling',
            label: '拉流状态',
            type: 'select',
            placeholder: '全部状态',
            options: [
              { label: 'running', value: 'true' },
              { label: 'stopped', value: 'false' },
            ],
          },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<GbStreamProxyItem>
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
        className="e2e-gb-stream-proxy-modal"
        title={editing ? '编辑拉流代理' : '新增拉流代理'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => void formRef.current?.submit()}
        confirmLoading={saving}
        destroyOnHidden
        width={760}
      >
        <StreamProxyFormContent
          ref={formRef}
          editing={editing}
          mediaServerOptions={mediaServerOptions}
          onSubmit={handleSave}
        />
      </Modal>

      {/* 内嵌播放：地址来自 playGbStreamProxy，409 时展示后端 message */}
      <Modal
        className="e2e-gb-stream-proxy-play-modal"
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
    </PageContainer>
  )
}
