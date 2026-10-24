import {
  forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState,
} from 'react'
import {
  Alert, Button, Col, Form, Input, Modal, Popconfirm, Row, Space, Switch, Table, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  CopyOutlined, DeleteOutlined, EditOutlined, PauseCircleOutlined, PlayCircleOutlined, PlusOutlined,
} from '@ant-design/icons'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { GbStreamPushController } from '../../api/generated/GbStreamPushController'
import type { GbStreamPushItem } from '../../api/generated/definitions/GbStreamPushItem'
import type { CreateGbStreamPushReq } from '../../api/generated/definitions/CreateGbStreamPushReq'
import type { UpdateGbStreamPushReq } from '../../api/generated/definitions/UpdateGbStreamPushReq'
import { useAuthStore } from '../../store/authStore'

/** 推流配置表单值 */
interface StreamPushFormValues {
  name: string
  app: string
  stream: string
  active?: boolean
  remark?: string
}

/** 时间格式化 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = dayjs(v)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : v
}

/** 新建 / 编辑的初始值。 */
function buildStreamPushInitialValues(editing: GbStreamPushItem | null): Partial<StreamPushFormValues> {
  if (!editing) {
    return { app: 'push', active: true }
  }
  return {
    name: editing.name ?? '',
    app: editing.app ?? '',
    stream: editing.stream ?? '',
    active: editing.active ?? true,
    remark: editing.remark ?? '',
  }
}

interface StreamPushFormHandle {
  submit: () => Promise<void>
}

interface StreamPushFormContentProps {
  editing: GbStreamPushItem | null
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
  function StreamPushFormContent({ editing, onCopy, onSubmit }, ref) {
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
            <Form.Item name="active" label="启用配置" valuePropName="checked">
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

  const columns: ColumnsType<GbStreamPushItem> = [
    { title: '名称', dataIndex: 'name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '应用名', dataIndex: 'app', width: 120, render: (v?: string) => v || '-' },
    { title: '流 ID', dataIndex: 'stream', width: 150, ellipsis: true, render: (v?: string) => v || '-' },
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
      title: '操作', key: 'action', width: 300, fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={300}>
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
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增推流
          </Button>
        ) : null
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
          onCopy={(url) => void handleCopy(url)}
          onSubmit={handleSave}
        />
      </Modal>
    </PageContainer>
  )
}
