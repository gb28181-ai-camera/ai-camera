import React, { useCallback, useEffect, useState } from 'react'
import {
  Table, Button, Modal, Form, Input, Select, Space, Row, Col, Tag, Alert, message,
  Popconfirm, Switch, Radio, Checkbox, InputNumber, Tooltip,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, ReloadOutlined, ApiOutlined, SendOutlined,
} from '@ant-design/icons'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { WebhookController } from '../../api/generated/WebhookController'
import type { WebhookConfig } from '../../api/generated/definitions/WebhookConfig'
import type { CreateWebhookReq } from '../../api/generated/definitions/CreateWebhookReq'
import { useAuthStore } from '../../store/authStore'

/** 平台元信息：展示名称、标签色、机器人地址示例 */
const PLATFORM_META: Record<string, { label: string; color: string; urlHint: string }> = {
  feishu: { label: '飞书', color: 'blue', urlHint: 'https://open.feishu.cn/open-apis/bot/v2/hook/xxxxxxxx-xxxx-xxxx' },
  dingtalk: { label: '钉钉', color: 'cyan', urlHint: 'https://oapi.dingtalk.com/robot/send?access_token=xxxxxxxx' },
  wecom: { label: '企业微信', color: 'green', urlHint: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxxxxx' },
  custom: { label: '自定义', color: 'purple', urlHint: 'https://your-domain.com/webhook/receiver' },
}

/** 订阅事件选项（全不勾选 = 订阅全部） */
const EVENT_OPTIONS = [
  { value: 'risk', label: '风险' },
  { value: 'alarm', label: '设备告警' },
  { value: 'inspect', label: '巡检通知' },
  { value: 'device', label: '设备状态' },
]

/** 事件值 → 展示名称（由 EVENT_OPTIONS 派生，保持单一数据源） */
const EVENT_LABEL: Record<string, string> = Object.fromEntries(
  EVENT_OPTIONS.map((o) => [o.value, o.label]),
)

/** 消息格式标签 */
const MSG_TYPE_LABEL: Record<string, string> = { text: '文本', markdown: 'Markdown' }

/** 启用状态下拉选项（字符串值，便于 Select 与 allowClear 协作） */
const ACTIVE_OPTIONS = [
  { value: 'true', label: '已启用' },
  { value: 'false', label: '已停用' },
]

/** 消息推送配置表单值 */
interface WebhookFormValues {
  name: string
  platform: string
  url: string
  secret?: string
  /** 由后端 model.WebhookConfig 决定取值（text / markdown） */
  msg_type?: string
  event_types?: string[]
  sequence?: number
  active?: boolean
  remark?: string
}

/** 平台元信息（未知平台兜底） */
function platformMeta(platform?: string): { label: string; color: string; urlHint: string } {
  if (platform && PLATFORM_META[platform]) return PLATFORM_META[platform]
  return { label: platform || '-', color: 'default', urlHint: '' }
}

/** 启用状态筛选值 → 接口布尔参数 */
function activeToBool(v?: string): boolean | undefined {
  if (v === 'true') return true
  if (v === 'false') return false
  return undefined
}

/** 将后端的逗号分隔事件串解析为数组 */
function parseEventTypes(v?: string | null): string[] {
  if (!v) return []
  return v.split(',').map((s) => s.trim()).filter(Boolean)
}

/** 表单值 → 接口请求体 */
function toPayload(values: WebhookFormValues): CreateWebhookReq {
  return {
    name: values.name?.trim(),
    platform: values.platform,
    url: values.url?.trim(),
    secret: values.secret?.trim() || undefined,
    msg_type: values.msg_type ?? 'markdown',
    event_types: values.event_types ?? [],
    sequence: values.sequence ?? 10,
    active: values.active !== false,
    remark: values.remark,
  }
}

/** 消息推送配置：管理飞书 / 钉钉 / 企业微信 / 自定义 WebHook */
const WebhookList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.length === 0 || permCodes.includes('webhook:create')
  const canUpdate = permCodes.length === 0 || permCodes.includes('webhook:update')
  const canDelete = permCodes.length === 0 || permCodes.includes('webhook:delete')
  const canTest = permCodes.length === 0 || permCodes.includes('webhook:test')

  const [data, setData] = useState<WebhookConfig[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [platformFilter, setPlatformFilter] = useState<string | undefined>(undefined)
  const [activeFilter, setActiveFilter] = useState<string | undefined>(undefined)

  const [platformOptions, setPlatformOptions] = useState<{ value: string; label: string }[]>([])

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<WebhookConfig | null>(null)
  const [saving, setSaving] = useState(false)
  const [testingInline, setTestingInline] = useState(false)
  const [testingId, setTestingId] = useState<number | null>(null)
  const [togglingId, setTogglingId] = useState<number | null>(null)
  const [platform, setPlatform] = useState<string | undefined>(undefined)
  const [form] = Form.useForm<WebhookFormValues>()

  const showSecret = platform === 'dingtalk' || platform === 'custom'

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await WebhookController.pageWebhooks(
        page, pageSize, keyword, platformFilter, activeToBool(activeFilter),
      )
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取消息推送配置失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, platformFilter, activeFilter])

  useEffect(() => { fetchData() }, [fetchData])

  /** 平台下拉选项：来自后端，保证与后端取值一致 */
  const fetchPlatforms = useCallback(async () => {
    try {
      const res = await WebhookController.webhookPlatforms()
      const options = (res.data ?? [])
        .filter((item) => Boolean(item.value))
        .map((item) => ({ value: String(item.value), label: item.label ?? String(item.value) }))
      setPlatformOptions(options)
    } catch {
      message.error('获取平台选项失败')
    }
  }, [])

  useEffect(() => { fetchPlatforms() }, [fetchPlatforms])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setPlatformFilter(undefined)
    setActiveFilter(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ msg_type: 'markdown', sequence: 10, active: true, event_types: [] })
    setPlatform(undefined)
    setModalOpen(true)
  }

  const openEdit = (row: WebhookConfig) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      platform: row.platform ?? undefined,
      url: row.url ?? '',
      secret: row.secret ?? '',
      msg_type: row.msg_type ?? 'markdown',
      event_types: parseEventTypes(row.event_types),
      sequence: row.sequence ?? 10,
      active: row.active !== false,
      remark: row.remark ?? '',
    })
    setPlatform(row.platform ?? undefined)
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    const payload = toPayload(values)
    setSaving(true)
    try {
      if (editing?.id) {
        await WebhookController.updateWebhook(String(editing.id), payload)
        message.success('修改成功')
      } else {
        await WebhookController.createWebhook(payload)
        message.success('新增成功')
      }
      setModalOpen(false)
      fetchData()
    } catch {
      message.error('保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (row: WebhookConfig) => {
    if (!row.id) return
    try {
      await WebhookController.deleteWebhook(String(row.id))
      message.success('删除成功')
      fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  /** 对已保存的配置发送测试消息 */
  const handleTest = async (row: WebhookConfig) => {
    if (!row.id) return
    setTestingId(row.id)
    try {
      const res = await WebhookController.testWebhook(String(row.id))
      const ok = res.data?.success === true
      const text = res.data?.message || (ok ? '测试消息已发送' : '测试失败')
      if (ok) message.success(text)
      else message.error(text)
      fetchData()
    } catch {
      message.error('测试失败')
    } finally {
      setTestingId(null)
    }
  }

  /** 保存前先验证当前表单配置的连通性（不落库） */
  const handleTestInline = async () => {
    let values: WebhookFormValues
    try {
      values = await form.validateFields()
    } catch {
      return
    }
    setTestingInline(true)
    try {
      const res = await WebhookController.testWebhookInline(toPayload(values))
      const ok = res.data?.success === true
      const text = res.data?.message || (ok ? '测试消息已发送' : '测试失败')
      if (ok) message.success(text)
      else message.error(text)
    } catch {
      message.error('测试失败')
    } finally {
      setTestingInline(false)
    }
  }

  const handleToggle = async (row: WebhookConfig, checked: boolean) => {
    if (!row.id) return
    setTogglingId(row.id)
    try {
      if (checked) await WebhookController.enableWebhook(String(row.id))
      else await WebhookController.disableWebhook(String(row.id))
      message.success(checked ? '已启用' : '已停用')
      fetchData()
    } catch {
      message.error('操作失败')
    } finally {
      setTogglingId(null)
    }
  }

  const columns: ColumnsType<WebhookConfig> = [
    {
      title: '配置名称',
      dataIndex: 'name',
      width: 180,
      render: (v?: string) => v || '-',
    },
    {
      title: '平台',
      dataIndex: 'platform',
      width: 110,
      render: (v?: string) => {
        const meta = platformMeta(v)
        return <Tag color={meta.color}>{meta.label}</Tag>
      },
    },
    {
      title: '订阅事件',
      dataIndex: 'event_types',
      width: 220,
      render: (v?: string) => {
        const codes = parseEventTypes(v)
        if (!codes.length) return <Tag>全部事件</Tag>
        return (
          <>
            {codes.map((code) => (
              <Tag key={code} color="geekblue">
                {EVENT_LABEL[code] ?? code}
              </Tag>
            ))}
          </>
        )
      },
    },
    {
      title: '消息格式',
      dataIndex: 'msg_type',
      width: 110,
      render: (v?: string) => (v ? (MSG_TYPE_LABEL[v] ?? v) : '-'),
    },
    {
      title: '状态',
      dataIndex: 'active',
      width: 90,
      render: (_, row) => (
        <Switch
          size="small"
          checked={row.active !== false}
          loading={togglingId === row.id}
          disabled={!canUpdate}
          onChange={(checked) => handleToggle(row, checked)}
        />
      ),
    },
    {
      title: '最近推送',
      dataIndex: 'last_send_dt',
      width: 230,
      render: (_, row) => {
        if (!row.last_send_dt) return <span>-</span>
        const time = dayjs(row.last_send_dt).format('YYYY-MM-DD HH:mm:ss')
        const statusTag =
          row.last_status === 'success' ? (
            <Tag color="green">成功</Tag>
          ) : row.last_status === 'failed' ? (
            <Tag color="red">失败</Tag>
          ) : null
        const content = (
          <span>
            {time} {statusTag}
          </span>
        )
        if (row.last_status === 'failed' && row.last_error) {
          return (
            <Tooltip title={row.last_error}>
              <span style={{ cursor: 'help' }}>{content}</span>
            </Tooltip>
          )
        }
        return content
      },
    },
    {
      title: '操作',
      key: 'action',
      width: 220,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons>
          {canTest && (
            <Button
              type="link"
              size="small"
              icon={<SendOutlined />}
              loading={testingId === row.id}
              onClick={() => handleTest(row)}
            >
              测试
            </Button>
          )}
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
              编辑
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除该配置?" onConfirm={() => handleDelete(row)}>
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
      title="消息推送配置"
      subTitle="配置飞书 / 钉钉 / 企业微信 / 自定义 WebHook，接收风险、告警、巡检等业务通知"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchData} loading={loading}>
            刷新
          </Button>
          {canCreate && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新增配置
            </Button>
          )}
        </Space>
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="各平台配置差异"
        description={
          <>
            飞书、企业微信：直接填写群机器人的 WebHook 地址即可，无需签名密钥。
            钉钉：除 WebHook 地址外，还需填写「加签」密钥（SEC 开头）。
            自定义：平台会收到通用 JSON 报文，可选填接收方校验用的签名密钥。
            订阅事件全不勾选表示接收全部事件。
          </>
        }
      />

      <SearchBar
        fields={[{ name: 'keyword', label: '配置名称', placeholder: '请输入配置名称' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Space style={{ marginBottom: 16 }} wrap>
        <Select
          allowClear
          placeholder="平台"
          style={{ width: 160 }}
          value={platformFilter}
          options={platformOptions}
          onChange={(v?: string) => { setPlatformFilter(v); setPage(1) }}
        />
        <Select
          allowClear
          placeholder="启用状态"
          style={{ width: 160 }}
          value={activeFilter}
          options={ACTIVE_OPTIONS}
          onChange={(v?: string) => { setActiveFilter(v); setPage(1) }}
        />
      </Space>

      <Table<WebhookConfig>
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

      <Modal
        className="e2e-webhook-modal"
        title={editing ? '编辑消息推送配置' : '新增消息推送配置'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        destroyOnHidden
        width={760}
        footer={
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Button icon={<ApiOutlined />} loading={testingInline} onClick={handleTestInline}>
              测试连通性
            </Button>
            <Space>
              <Button onClick={() => setModalOpen(false)}>取消</Button>
              <Button type="primary" loading={saving} onClick={handleSave}>
                保存
              </Button>
            </Space>
          </div>
        }
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="配置名称" rules={[{ required: true, message: '请输入配置名称' }]}>
                <Input maxLength={64} placeholder="如：运维告警群" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="platform" label="平台" rules={[{ required: true, message: '请选择平台' }]}>
                <Select
                  placeholder="请选择平台"
                  options={platformOptions}
                  onChange={(v: string) => setPlatform(v)}
                />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="url" label="WebHook 地址" rules={[{ required: true, message: '请输入 WebHook 地址' }]}>
                <Input placeholder={platformMeta(platform).urlHint || '请先选择平台，再填写机器人 WebHook 地址'} />
              </Form.Item>
            </Col>
            {showSecret && (
              <Col span={24}>
                <Form.Item name="secret" label="签名密钥">
                  <Input.Password placeholder="请输入签名密钥" autoComplete="new-password" />
                </Form.Item>
                <Form.Item>
                  <span style={{ color: '#999', fontSize: 12 }}>
                    钉钉：填加签 SEC 开头的密钥；自定义：填接收方校验用的密钥。留空表示不签名，编辑时保持原值可留空。
                  </span>
                </Form.Item>
              </Col>
            )}
            <Col span={12}>
              <Form.Item name="msg_type" label="消息格式">
                <Radio.Group
                  options={[
                    { value: 'markdown', label: 'Markdown' },
                    { value: 'text', label: '文本' },
                  ]}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="sequence" label="排序">
                <InputNumber min={0} max={9999} style={{ width: '100%' }} placeholder="数值越小越靠前" />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="event_types" label="订阅事件">
                <Checkbox.Group options={EVENT_OPTIONS} />
              </Form.Item>
              <Form.Item>
                <span style={{ color: '#999', fontSize: 12 }}>
                  全不勾选表示订阅全部事件；勾选后仅推送所选事件类型的消息。
                </span>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="active" label="启用" valuePropName="checked">
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
      </Modal>
    </PageContainer>
  )
}

export default WebhookList
