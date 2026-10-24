import { useCallback, useEffect, useState } from 'react'
import {
  Button, Col, Form, Input, InputNumber, Modal, Popconfirm, Row, Select, Space, Spin, Switch,
  Table, Tag, Typography, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  DeleteOutlined, EditOutlined, KeyOutlined, PlusOutlined, SettingOutlined,
} from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { IOTController } from '../../api/generated/IOTController'
import type { IotServiceGroup } from '../../api/generated/definitions/IotServiceGroup'
import type { IotServiceItemReq } from '../../api/generated/definitions/IotServiceItemReq'
import type { IotServiceGroupCredentials } from '../../api/generated/definitions/IotServiceGroupCredentials'
import type { IotCredentials } from '../../api/generated/definitions/IotCredentials'
import type { CreateIotServiceGroupReq } from '../../api/generated/definitions/CreateIotServiceGroupReq'
import type { UpdateIotServiceGroupReq } from '../../api/generated/definitions/UpdateIotServiceGroupReq'
import { useAuthStore } from '../../store/authStore'
import { CredentialModal } from './CredentialModal'
import { SecretModal, notifyError } from './shared'

/** 服务组类型字典 */
const TYPE_OPTIONS = [
  { label: 'GB28181', value: 'gb28181' },
  { label: 'WebRTC', value: 'webrtc' },
  { label: 'AI聊天', value: 'aichat' },
]

const TYPE_META: Record<string, { label: string; color: string }> = {
  gb28181: { label: 'GB28181', color: 'geekblue' },
  webrtc: { label: 'WebRTC', color: 'blue' },
  aichat: { label: 'AI聊天', color: 'purple' },
}

/** 各类型服务组允许的协议白名单（与后端校验一致） */
const PROTOCOL_OPTIONS: Record<string, { label: string; value: string }[]> = {
  gb28181: [
    { label: 'SIP', value: 'SIP' },
    { label: 'RTSP', value: 'RTSP' },
  ],
  webrtc: [
    { label: 'WebRTC', value: 'WebRTC' },
    { label: 'MQTT', value: 'MQTT' },
    { label: 'RTSP', value: 'RTSP' },
  ],
  aichat: [
    { label: 'WebSocket', value: 'WebSocket' },
  ],
}

const ALL_PROTOCOLS = [
  { label: 'SIP', value: 'SIP' },
  { label: 'RTSP', value: 'RTSP' },
  { label: 'WebRTC', value: 'WebRTC' },
  { label: 'MQTT', value: 'MQTT' },
  { label: 'WebSocket', value: 'WebSocket' },
]

interface ServiceGroupFormValues {
  name: string
  code: string
  type: string
  description?: string
  active: boolean
}

interface ServiceItemFormValue {
  protocol?: string
  host?: string
  port?: number
  path?: string
  extra?: string
  active?: boolean
  sequence?: number
}

interface ServiceItemsFormValues {
  items: ServiceItemFormValue[]
}

/** 新建服务组默认值 */
const DEFAULT_GROUP_VALUES: Partial<ServiceGroupFormValues> = {
  type: 'gb28181',
  active: true,
}

/**
 * IOT 服务组。
 *
 * 服务组 → 服务项两级结构；组级用户名/密码由平台自动生成，
 * 读接口始终脱敏为 ****，只有「重置凭据」返回一次性明文。
 */
export default function ServiceGroupList() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('iot:service-group:create')
  const canUpdate = permCodes.includes('iot:service-group:update')
  const canDelete = permCodes.includes('iot:service-group:delete')

  const [data, setData] = useState<IotServiceGroup[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [typeFilter, setTypeFilter] = useState<string | undefined>(undefined)
  /** 服务组 → 服务项数量。列表接口不返回该字段，按当前页逐条查询。 */
  const [itemCounts, setItemCounts] = useState<Record<number, number>>({})

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<IotServiceGroup | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<ServiceGroupFormValues>()

  const [credOpen, setCredOpen] = useState(false)
  const [credGroup, setCredGroup] = useState<IotServiceGroup | null>(null)
  const [cred, setCred] = useState<IotServiceGroupCredentials | null>(null)

  const [detailOpen, setDetailOpen] = useState(false)
  const [detailLoading, setDetailLoading] = useState<number | null>(null)
  const [detailGroup, setDetailGroup] = useState<IotServiceGroup | null>(null)
  const [detailData, setDetailData] = useState<IotCredentials | null>(null)

  const [itemsOpen, setItemsOpen] = useState(false)
  const [itemsGroup, setItemsGroup] = useState<IotServiceGroup | null>(null)
  const [itemsLoading, setItemsLoading] = useState(false)
  const [itemsSaving, setItemsSaving] = useState(false)
  const [itemsForm] = Form.useForm<ServiceItemsFormValues>()

  const loadItemCounts = useCallback(async (rows: IotServiceGroup[]) => {
    const results = await Promise.all(rows.map(async (row) => {
      if (row.id == null) return null
      try {
        const res = await IOTController.getIotServiceGroup(row.id)
        return { id: row.id, count: res.data?.items?.length ?? 0 }
      } catch {
        return null
      }
    }))
    const next: Record<number, number> = {}
    for (const item of results) {
      if (item) next[item.id] = item.count
    }
    setItemCounts(next)
  }, [])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await IOTController.pageIotServiceGroups(
        page, pageSize, keyword || undefined, typeFilter || undefined, undefined,
      )
      const items = res.data?.items ?? []
      setData(items)
      setTotal(Number(res.data?.total ?? 0))
      void loadItemCounts(items)
    } catch (err) {
      notifyError(err, '获取服务组列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, typeFilter, loadItemCounts])

  useEffect(() => { void fetchData() }, [fetchData])

  const handleSearch = (values: Record<string, unknown>) => {
    setKeyword(typeof values.keyword === 'string' ? values.keyword : '')
    setTypeFilter(typeof values.type === 'string' && values.type ? values.type : undefined)
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setTypeFilter(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue(DEFAULT_GROUP_VALUES)
    setModalOpen(true)
  }

  const openEdit = (row: IotServiceGroup) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      code: row.code ?? '',
      type: row.type || 'gb28181',
      description: row.description ?? '',
      active: row.active ?? true,
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    let values: ServiceGroupFormValues
    try {
      values = await form.validateFields()
    } catch {
      return
    }

    setSaving(true)
    try {
      if (editing?.id != null) {
        const payload: UpdateIotServiceGroupReq = {
          name: values.name,
          code: values.code,
          type: values.type,
          description: values.description,
          active: values.active,
        }
        const res = await IOTController.updateIotServiceGroup(editing.id, payload)
        if (res.code === 200) {
          message.success('修改成功')
          setModalOpen(false)
          void fetchData()
        } else {
          message.error(res.message || '保存失败')
        }
      } else {
        const payload: CreateIotServiceGroupReq = {
          name: values.name,
          code: values.code,
          type: values.type,
          description: values.description,
          active: values.active,
        }
        const res = await IOTController.createIotServiceGroup(payload)
        if (res.code === 200) {
          message.success('新增成功')
          setModalOpen(false)
          void fetchData()
        } else {
          message.error(res.message || '保存失败')
        }
      }
    } catch (err) {
      notifyError(err, '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (row: IotServiceGroup) => {
    if (row.id == null) return
    try {
      const res = await IOTController.deleteIotServiceGroup(row.id)
      if (res.code === 200) {
        message.success('删除成功')
        void fetchData()
      } else {
        message.error(res.message || '删除失败')
      }
    } catch (err) {
      notifyError(err, '删除失败')
    }
  }

  const handleResetCredentials = async (row: IotServiceGroup) => {
    if (row.id == null) return
    try {
      const res = await IOTController.resetIotServiceGroupCredentials(row.id)
      if (res.code === 200 && res.data) {
        setCredGroup(row)
        setCred(res.data)
        setCredOpen(true)
        void fetchData()
      } else {
        message.error(res.message || '重置凭据失败')
      }
    } catch (err) {
      notifyError(err, '重置凭据失败')
    }
  }

  /** 拉取单个服务组的明文鉴权信息（仅 `iot:service-group:update` 可调用） */
  const handleCopyCredentials = async (row: IotServiceGroup) => {
    if (row.id == null) return
    setDetailLoading(row.id)
    try {
      const res = await IOTController.getIotServiceGroupCredentials(row.id)
      if (res.code === 200 && res.data) {
        // 单组接口返回 {group, mqtt}，这里归一化为产品维度的结构，复用同一个弹窗。
        setDetailData({
          groups: res.data.group ? [res.data.group] : [],
          mqtt: res.data.mqtt,
        })
        setDetailGroup(row)
        setDetailOpen(true)
      } else {
        message.error(res.message || '获取鉴权信息失败')
      }
    } catch (err) {
      notifyError(err, '获取鉴权信息失败')
    } finally {
      setDetailLoading(null)
    }
  }

  const openItems = async (row: IotServiceGroup) => {
    if (row.id == null) return
    setItemsGroup(row)
    setItemsOpen(true)
    setItemsLoading(true)
    itemsForm.resetFields()
    try {
      const res = await IOTController.getIotServiceGroup(row.id)
      const items = res.data?.items ?? []
      itemsForm.setFieldsValue({
        items: items.map((item) => ({
          protocol: item.protocol ?? undefined,
          host: item.host ?? '',
          port: item.port ?? 0,
          path: item.path ?? '',
          extra: item.extra ?? '',
          active: item.active ?? true,
          sequence: item.sequence ?? 10,
        })),
      })
    } catch (err) {
      notifyError(err, '获取服务项失败')
    } finally {
      setItemsLoading(false)
    }
  }

  const protocolOptions = (): { label: string; value: string }[] => {
    const groupType = itemsGroup?.type ?? ''
    return PROTOCOL_OPTIONS[groupType] ?? ALL_PROTOCOLS
  }

  const validateUniqueProtocol = async (_rule: unknown, value?: string) => {
    if (!value) return
    const all = itemsForm.getFieldValue('items') as ServiceItemFormValue[] | undefined
    const count = (all ?? []).filter((item) => item?.protocol === value).length
    if (count > 1) throw new Error('同一服务组内协议不可重复')
  }

  const validateExtra = async (_rule: unknown, value?: string) => {
    const text = (value ?? '').trim()
    if (!text) return
    try {
      JSON.parse(text)
    } catch {
      throw new Error('extra 必须是合法的 JSON')
    }
  }

  const handleSaveItems = async () => {
    if (itemsGroup?.id == null) return
    let values: ServiceItemsFormValues
    try {
      values = await itemsForm.validateFields()
    } catch {
      return
    }

    const items: IotServiceItemReq[] = (values.items ?? []).map((item) => ({
      protocol: item.protocol,
      host: item.host?.trim() || undefined,
      port: Number(item.port ?? 0),
      path: item.path?.trim() || undefined,
      extra: item.extra?.trim() || undefined,
      active: item.active ?? true,
      sequence: Number(item.sequence ?? 10),
    }))

    setItemsSaving(true)
    try {
      const res = await IOTController.replaceIotServiceGroupItems(itemsGroup.id, { items })
      if (res.code === 200) {
        message.success('服务项已保存')
        setItemsOpen(false)
        void fetchData()
      } else {
        message.error(res.message || '保存服务项失败')
      }
    } catch (err) {
      notifyError(err, '保存服务项失败')
    } finally {
      setItemsSaving(false)
    }
  }

  const columns: ColumnsType<IotServiceGroup> = [
    { title: '名称', dataIndex: 'name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '编码', dataIndex: 'code', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '类型', dataIndex: 'type', width: 110,
      render: (v?: string) => {
        const meta = v ? TYPE_META[v] : undefined
        return meta ? <Tag color={meta.color}>{meta.label}</Tag> : <Tag>{v || '-'}</Tag>
      },
    },
    {
      title: '用户名', dataIndex: 'username', width: 200, ellipsis: true,
      render: (v?: string) => (v ? <Typography.Text copyable={{ text: v }}>{v}</Typography.Text> : '-'),
    },
    {
      title: '密码', dataIndex: 'password', width: 100,
      render: (v?: string) => <Typography.Text type="secondary">{v || '****'}</Typography.Text>,
    },
    {
      title: '服务项数量', key: 'item_count', width: 120, align: 'center',
      render: (_, row) => (row.id != null && itemCounts[row.id] !== undefined ? itemCounts[row.id] : '-'),
    },
    {
      title: '启用', dataIndex: 'active', width: 90,
      render: (v?: boolean) => (v ? <Tag color="green">启用</Tag> : <Tag>停用</Tag>),
    },
    {
      title: '操作', key: 'action', width: 470, fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={470}>
          {canUpdate && (
            <Button
              type="link"
              size="small"
              icon={<KeyOutlined />}
              loading={detailLoading === row.id}
              onClick={() => void handleCopyCredentials(row)}
            >
              复制鉴权
            </Button>
          )}
          {canUpdate && (
            <Button type="link" size="small" icon={<SettingOutlined />} onClick={() => void openItems(row)}>
              编辑服务项
            </Button>
          )}
          {canUpdate && (
            <Popconfirm
              title="重置后旧用户名与密码立即失效，确定重置?"
              onConfirm={() => void handleResetCredentials(row)}
            >
              <Button type="link" size="small" icon={<KeyOutlined />}>
                重置凭据
              </Button>
            </Popconfirm>
          )}
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
              编辑
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除该服务组?" onConfirm={() => void handleDelete(row)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  const groupTypeLabel = itemsGroup?.type ? (TYPE_META[itemsGroup.type]?.label ?? itemsGroup.type) : '-'

  return (
    <PageContainer
      title="服务组"
      subTitle="维护设备接入服务组与服务项；组凭据由平台自动生成，读接口始终脱敏"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增服务组
          </Button>
        ) : null
      }
    >
      <SearchBar
        fields={[
          { name: 'keyword', label: '关键字', placeholder: '请输入名称/编码/用户名' },
          { name: 'type', label: '类型', type: 'select', options: TYPE_OPTIONS },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<IotServiceGroup>
        className="e2e-iot-service-group-table"
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
        className="e2e-iot-service-group-modal"
        title={editing ? '编辑服务组' : '新增服务组'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => void handleSave()}
        confirmLoading={saving}
        destroyOnHidden
        width={640}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="服务组名称" rules={[{ required: true, message: '请输入服务组名称' }]}>
                <Input placeholder="如：GB28181 服务组" maxLength={255} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="code" label="服务组编码" rules={[{ required: true, message: '请输入服务组编码' }]}>
                <Input placeholder="如：GB_GROUP_01" maxLength={100} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="type" label="类型" rules={[{ required: true, message: '请选择服务组类型' }]}>
                <Select options={TYPE_OPTIONS} placeholder="请选择服务组类型" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="active" label="启用" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="description" label="描述">
                <Input.TextArea rows={3} maxLength={500} showCount placeholder="请输入描述" />
              </Form.Item>
            </Col>
          </Row>
          {editing ? (
            <Typography.Text type="secondary">
              用户名与密码由平台自动生成；如需轮换请使用列表中的「重置凭据」。
            </Typography.Text>
          ) : null}
        </Form>
      </Modal>

      <Modal
        className="e2e-iot-service-group-items-modal"
        title={itemsGroup ? `编辑服务项 - ${itemsGroup.name ?? ''}` : '编辑服务项'}
        open={itemsOpen}
        onCancel={() => setItemsOpen(false)}
        onOk={() => void handleSaveItems()}
        confirmLoading={itemsSaving}
        okButtonProps={{ disabled: itemsLoading }}
        width={900}
        destroyOnHidden
      >
        <Spin spinning={itemsLoading}>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
            服务组类型：{groupTypeLabel}；可用协议：{protocolOptions().map((p) => p.value).join('、')}。
            端口填 0 表示按协议取默认端口（SIP 5060 / RTSP 554 / WebRTC 443 / MQTT 1883 / WebSocket 443）。
          </Typography.Paragraph>
          <Form form={itemsForm} layout="vertical">
            <Form.List name="items">
              {(fields, { add, remove }) => (
                <Space direction="vertical" size={12} style={{ width: '100%' }}>
                  {fields.map(({ key, name, ...restField }) => (
                    <div key={key} style={{ border: '1px solid #f0f0f0', borderRadius: 6, padding: 12 }}>
                      <Row gutter={12}>
                        <Col span={6}>
                          <Form.Item
                            {...restField}
                            name={[name, 'protocol']}
                            label="协议"
                            rules={[
                              { required: true, message: '请选择协议' },
                              { validator: validateUniqueProtocol },
                            ]}
                          >
                            <Select options={protocolOptions()} placeholder="请选择协议" />
                          </Form.Item>
                        </Col>
                        <Col span={8}>
                          <Form.Item {...restField} name={[name, 'host']} label="host">
                            <Input placeholder="服务地址" maxLength={255} />
                          </Form.Item>
                        </Col>
                        <Col span={5}>
                          <Form.Item {...restField} name={[name, 'port']} label="端口">
                            <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="0=默认" />
                          </Form.Item>
                        </Col>
                        <Col span={5}>
                          <Form.Item {...restField} name={[name, 'sequence']} label="排序">
                            <InputNumber min={0} max={9999} style={{ width: '100%' }} placeholder="默认 10" />
                          </Form.Item>
                        </Col>
                        <Col span={12}>
                          <Form.Item {...restField} name={[name, 'path']} label="path">
                            <Input placeholder="服务路径" maxLength={500} />
                          </Form.Item>
                        </Col>
                        <Col span={12}>
                          <Form.Item
                            {...restField}
                            name={[name, 'extra']}
                            label="extra（JSON 文本）"
                            rules={[{ validator: validateExtra }]}
                          >
                            <Input.TextArea rows={2} placeholder='如：{"app":"live"}' />
                          </Form.Item>
                        </Col>
                      </Row>
                      <Space>
                        <Form.Item
                          {...restField}
                          name={[name, 'active']}
                          label="启用"
                          valuePropName="checked"
                          style={{ marginBottom: 0 }}
                        >
                          <Switch />
                        </Form.Item>
                        <Button danger size="small" onClick={() => remove(name)}>
                          移除
                        </Button>
                      </Space>
                    </div>
                  ))}
                  <Button
                    type="dashed"
                    block
                    icon={<PlusOutlined />}
                    onClick={() => add({ port: 0, active: true, sequence: 10 })}
                  >
                    添加服务项
                  </Button>
                </Space>
              )}
            </Form.List>
          </Form>
        </Spin>
      </Modal>

      <SecretModal
        className="e2e-iot-service-group-credentials-modal"
        open={credOpen}
        title={credGroup ? `重置凭据 - ${credGroup.name ?? ''}` : '重置凭据'}
        warning="重置成功！旧凭据立即失效；新凭据仅显示一次，关闭后无法再次查看，请立即复制并妥善保存"
        items={[
          { label: '用户名', value: cred?.username },
          { label: '密码', value: cred?.password },
        ]}
        onClose={() => setCredOpen(false)}
      />

      <CredentialModal
        open={detailOpen}
        title={detailGroup ? `鉴权信息 - ${detailGroup.name ?? ''}` : '鉴权信息'}
        data={detailData}
        onClose={() => setDetailOpen(false)}
      />
    </PageContainer>
  )
}
