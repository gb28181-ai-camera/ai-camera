import {
  forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState,
} from 'react'
import {
  Button, Col, Form, Input, InputNumber, Modal, Popconfirm, Row, Switch, Table, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  DeleteOutlined, EditOutlined, PauseCircleOutlined, PlayCircleOutlined, PlusOutlined,
} from '@ant-design/icons'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { GbStreamProxyController } from '../../api/generated/GbStreamProxyController'
import type { GbStreamProxyItem } from '../../api/generated/definitions/GbStreamProxyItem'
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
  enable_audio?: boolean
  enable_mp4?: boolean
  enable?: boolean
  remark?: string
}

/** InputNumber 空值统一转 undefined */
function num(v?: number | null): number | undefined {
  return typeof v === 'number' && !Number.isNaN(v) ? v : undefined
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
      enable_audio: true,
      enable_mp4: false,
      enable: true,
    }
  }
  return {
    name: editing.name ?? '',
    app: editing.app ?? '',
    stream: editing.stream ?? '',
    src_url: editing.src_url ?? '',
    timeout_ms: editing.timeout_ms ?? 10000,
    enable_audio: editing.enable_audio ?? true,
    enable_mp4: editing.enable_mp4 ?? false,
    enable: editing.enable ?? true,
    remark: editing.remark ?? '',
  }
}

interface StreamProxyFormHandle {
  submit: () => Promise<void>
}

interface StreamProxyFormContentProps {
  editing: GbStreamProxyItem | null
  onSubmit: (values: StreamProxyFormValues) => Promise<void>
}

/**
 * 拉流代理表单内容：form 与 Form 同挂载在 Modal children 中，
 * 避免父组件在 Form 渲染前调用 useForm 方法产生告警。
 */
const StreamProxyFormContent = forwardRef<StreamProxyFormHandle, StreamProxyFormContentProps>(
  function StreamProxyFormContent({ editing, onSubmit }, ref) {
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
      enable_audio: values.enable_audio,
      enable_mp4: values.enable_mp4,
      enable: values.enable,
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
      title: '操作', key: 'action', width: 300, fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={300}>
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
        width={680}
      >
        <StreamProxyFormContent ref={formRef} editing={editing} onSubmit={handleSave} />
      </Modal>
    </PageContainer>
  )
}
