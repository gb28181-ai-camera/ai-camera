import { useCallback, useEffect, useState } from 'react'
import {
  Button, Col, Form, Input, InputNumber, Modal, Row, Select, Space, Spin, Switch, Table, Tag,
  Tooltip, Typography, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { EditOutlined, PlusOutlined, SettingOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { IOTController } from '../../api/generated/IOTController'
import type { IotServiceGroup } from '../../api/generated/definitions/IotServiceGroup'
import type { IotService } from '../../api/generated/definitions/IotService'
import type { IotServiceReq } from '../../api/generated/definitions/IotServiceReq'
import type { UpdateIotServiceGroupReq } from '../../api/generated/definitions/UpdateIotServiceGroupReq'
import { useAuthStore } from '../../store/authStore'
import { notifyError } from './shared'

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

/** 各类型服务组允许的协议白名单（与后端校验一致）。 */
const PROTOCOL_OPTIONS: Record<string, { label: string; value: string }[]> = {
  gb28181: [
    { label: 'SIP', value: 'SIP' },
    { label: 'RTSP', value: 'RTSP' },
  ],
  webrtc: [
    { label: 'WebRTC', value: 'WebRTC' },
    { label: 'MQTT', value: 'MQTT' },
    { label: 'RTSP', value: 'RTSP' },
    { label: 'TURN', value: 'TURN' },
  ],
  aichat: [{ label: 'WebSocket', value: 'WebSocket' }],
}

interface ServiceGroupFormValues {
  name: string
  code: string
  description?: string
  active: boolean
}

interface ServiceFormValue {
  code?: string
  name?: string
  protocol?: string
  host?: string
  port?: number
  path?: string
  extra?: string
  active?: boolean
  sequence?: number
}

interface ServicesFormValues {
  services: ServiceFormValue[]
}

/**
 * IOT 服务组。
 *
 * 服务组是系统预置的三条固定数据（gb28181 / webrtc / aichat），不开放新增/删除；
 * 页面只维护「组内服务」与组名称。服务凭据在产品页按「产品×服务」生成。
 *
 * 「实时音视频」「智能语音聊天」复用本页并通过 groupType 固定只显示对应服务组。
 */
export interface ServiceGroupListProps {
  groupType?: 'webrtc' | 'aichat'
  title?: string
  subTitle?: string
}

export default function ServiceGroupList({ groupType, title, subTitle }: ServiceGroupListProps = {}) {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canUpdate = permCodes.includes('iot:service-group:update')

  const [data, setData] = useState<IotServiceGroup[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [typeFilter, setTypeFilter] = useState<string | undefined>(groupType)

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<IotServiceGroup | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<ServiceGroupFormValues>()

  const [servicesOpen, setServicesOpen] = useState(false)
  const [servicesGroup, setServicesGroup] = useState<IotServiceGroup | null>(null)
  const [servicesLoading, setServicesLoading] = useState(false)
  const [servicesSaving, setServicesSaving] = useState(false)
  const [servicesForm] = Form.useForm<ServicesFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await IOTController.pageIotServiceGroups(
        page, pageSize, keyword || undefined, groupType ?? typeFilter ?? undefined, undefined,
      )
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch (err) {
      notifyError(err, '获取服务组列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, typeFilter, groupType])

  useEffect(() => { void fetchData() }, [fetchData])

  const handleSearch = (values: Record<string, unknown>) => {
    setKeyword(typeof values.keyword === 'string' ? values.keyword : '')
    if (!groupType) {
      setTypeFilter(typeof values.type === 'string' && values.type ? values.type : undefined)
    }
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    if (!groupType) setTypeFilter(undefined)
    setPage(1)
  }

  const openEdit = (row: IotServiceGroup) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      code: row.code ?? '',
      description: row.description ?? '',
      active: row.active ?? true,
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    if (editing?.id == null) return
    let values: ServiceGroupFormValues
    try {
      values = await form.validateFields()
    } catch {
      return
    }
    setSaving(true)
    try {
      // 编码与类型由系统固定，只提交可维护字段。
      const payload: UpdateIotServiceGroupReq = {
        name: values.name,
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
    } catch (err) {
      notifyError(err, '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const openServices = async (row: IotServiceGroup) => {
    if (row.id == null) return
    setServicesGroup(row)
    setServicesOpen(true)
    setServicesLoading(true)
    servicesForm.resetFields()
    try {
      const res = await IOTController.getIotServiceGroup(row.id)
      const services = res.data?.services ?? []
      servicesForm.setFieldsValue({
        services: services.map((svc: IotService) => ({
          code: svc.code ?? '',
          name: svc.name ?? '',
          protocol: svc.protocol ?? undefined,
          host: svc.host ?? '',
          port: svc.port ?? 0,
          path: svc.path ?? '',
          extra: svc.extra ?? '',
          active: svc.active ?? true,
          sequence: svc.sequence ?? 10,
        })),
      })
    } catch (err) {
      notifyError(err, '获取服务失败')
    } finally {
      setServicesLoading(false)
    }
  }

  const protocolOptions = (): { label: string; value: string }[] => {
    const groupType = servicesGroup?.type ?? ''
    return PROTOCOL_OPTIONS[groupType] ?? []
  }

  const validateUniqueCode = async (_rule: unknown, value?: string) => {
    if (!value) return
    const all = servicesForm.getFieldValue('services') as ServiceFormValue[] | undefined
    const count = (all ?? []).filter((item) => item?.code === value).length
    if (count > 1) throw new Error('同一服务组内服务编码不可重复')
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

  const handleSaveServices = async () => {
    if (servicesGroup?.id == null) return
    let values: ServicesFormValues
    try {
      values = await servicesForm.validateFields()
    } catch {
      return
    }
    const services: IotServiceReq[] = (values.services ?? []).map((item) => ({
      code: item.code?.trim() || '',
      name: item.name?.trim() || item.protocol || '',
      protocol: item.protocol || '',
      host: item.host?.trim() || undefined,
      port: Number(item.port ?? 0),
      path: item.path?.trim() || undefined,
      extra: item.extra?.trim() || undefined,
      active: item.active ?? true,
      sequence: Number(item.sequence ?? 10),
    }))
    setServicesSaving(true)
    try {
      const res = await IOTController.replaceIotServiceGroupServices(servicesGroup.id, { services })
      if (res.code === 200) {
        message.success('服务已保存')
        setServicesOpen(false)
        void fetchData()
      } else {
        message.error(res.message || '保存服务失败')
      }
    } catch (err) {
      notifyError(err, '保存服务失败')
    } finally {
      setServicesSaving(false)
    }
  }

  /** 被产品引用的服务组进入只读状态。 */
  const referencedCount = (row: IotServiceGroup): number => Number(row.referenced_by_products ?? 0)
  const isLocked = (row: IotServiceGroup): boolean => referencedCount(row) > 0
  const lockedTip = (row: IotServiceGroup): string =>
    `已被 ${referencedCount(row)} 个产品引用，请先在产品上解绑后再修改`

  const columns: ColumnsType<IotServiceGroup> = [
    { title: '名称', dataIndex: 'name', width: 200, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '编码', dataIndex: 'code', width: 140, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '类型', dataIndex: 'type', width: 110,
      render: (v?: string) => {
        const meta = v ? TYPE_META[v] : undefined
        return meta ? <Tag color={meta.color}>{meta.label}</Tag> : <Tag>{v || '-'}</Tag>
      },
    },
    {
      title: '被引用', key: 'referenced', width: 110, align: 'center',
      render: (_, row) => {
        const count = referencedCount(row)
        return count > 0
          ? <Tooltip title="被产品引用的服务组不可修改"><Tag color="orange">{count} 个产品</Tag></Tooltip>
          : <Typography.Text type="secondary">未引用</Typography.Text>
      },
    },
    {
      title: '启用', dataIndex: 'active', width: 90,
      render: (v?: boolean) => (v ? <Tag color="green">启用</Tag> : <Tag>停用</Tag>),
    },
    {
      title: '操作', key: 'action', width: 300, fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={300}>
          {canUpdate && (isLocked(row) ? (
            <Tooltip title={lockedTip(row)}>
              <span>
                <Button type="link" size="small" icon={<SettingOutlined />} disabled>
                  编辑服务
                </Button>
              </span>
            </Tooltip>
          ) : (
            <Button type="link" size="small" icon={<SettingOutlined />} onClick={() => void openServices(row)}>
              编辑服务
            </Button>
          ))}
          {canUpdate && (isLocked(row) ? (
            <Tooltip title={lockedTip(row)}>
              <span>
                <Button type="link" size="small" icon={<EditOutlined />} disabled>
                  编辑
                </Button>
              </span>
            </Tooltip>
          ) : (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
              编辑
            </Button>
          ))}
        </ActionButtons>
      ),
    },
  ]

  const groupTypeLabel = servicesGroup?.type ? (TYPE_META[servicesGroup.type]?.label ?? servicesGroup.type) : '-'

  return (
    <PageContainer
      title={
        title
        ?? (groupType === 'webrtc'
          ? '实时音视频服务设置'
          : groupType === 'aichat'
            ? '智能语音聊天服务设置'
            : '服务组')
      }
      subTitle={
        subTitle
        ?? (groupType === 'webrtc'
          ? '维护 WebRTC 服务组下的服务地址与端口；被产品引用后只读，需先解绑'
          : groupType === 'aichat'
            ? '维护 AIChat 服务组下的 WebSocket 接入地址；被产品引用后只读，需先解绑'
            : '系统预置 GB28181 / WebRTC / AIChat 三条服务组；被产品引用后只读，需先解绑')
      }
    >
      <SearchBar
        fields={[
          { name: 'keyword', label: '关键字', placeholder: '请输入名称/编码' },
          ...(groupType
            ? []
            : [{ name: 'type', label: '类型', type: 'select' as const, options: TYPE_OPTIONS }]),
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
        title="编辑服务组"
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
              <Form.Item name="code" label="服务组编码" extra="编码由系统固定，不可修改">
                <Input disabled />
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
        </Form>
      </Modal>

      <Modal
        className="e2e-iot-service-group-items-modal"
        title={servicesGroup ? `编辑服务 - ${servicesGroup.name ?? ''}` : '编辑服务'}
        open={servicesOpen}
        onCancel={() => setServicesOpen(false)}
        onOk={() => void handleSaveServices()}
        confirmLoading={servicesSaving}
        okButtonProps={{ disabled: servicesLoading }}
        width={980}
        destroyOnHidden
      >
        <Spin spinning={servicesLoading}>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
            服务组类型：{groupTypeLabel}；可用协议：{protocolOptions().map((p) => p.value).join('、')}。
            端口填 0 表示按协议取默认端口（SIP 5060 / RTSP 554 / WebRTC 443 / MQTT 1883 / WebSocket 443 / TURN 3478）。
          </Typography.Paragraph>
          <Form form={servicesForm} layout="vertical">
            <Form.List name="services">
              {(fields, { add, remove }) => (
                <Space direction="vertical" size={12} style={{ width: '100%' }}>
                  {fields.map(({ key, name, ...restField }) => (
                    <div key={key} style={{ border: '1px solid #f0f0f0', borderRadius: 6, padding: 12 }}>
                      <Row gutter={12}>
                        <Col span={4}>
                          <Form.Item
                            {...restField}
                            name={[name, 'code']}
                            label="编码"
                            rules={[
                              { required: true, message: '请输入服务编码' },
                              { validator: validateUniqueCode },
                            ]}
                          >
                            <Input placeholder="如 mqtt" maxLength={50} />
                          </Form.Item>
                        </Col>
                        <Col span={5}>
                          <Form.Item {...restField} name={[name, 'name']} label="名称">
                            <Input placeholder="如 MQTT 接入" maxLength={100} />
                          </Form.Item>
                        </Col>
                        <Col span={5}>
                          <Form.Item
                            {...restField}
                            name={[name, 'protocol']}
                            label="协议"
                            rules={[{ required: true, message: '请选择协议' }]}
                          >
                            <Select options={protocolOptions()} placeholder="请选择协议" />
                          </Form.Item>
                        </Col>
                        <Col span={5}>
                          <Form.Item {...restField} name={[name, 'host']} label="host">
                            <Input placeholder="服务地址" maxLength={255} />
                          </Form.Item>
                        </Col>
                        <Col span={3}>
                          <Form.Item {...restField} name={[name, 'port']} label="端口">
                            <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="0=默认" />
                          </Form.Item>
                        </Col>
                        <Col span={2}>
                          <Form.Item {...restField} name={[name, 'sequence']} label="排序">
                            <InputNumber min={0} max={9999} style={{ width: '100%' }} placeholder="10" />
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
                            <Input.TextArea rows={2} placeholder='如：{"tls":true}' />
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
                    添加服务
                  </Button>
                </Space>
              )}
            </Form.List>
          </Form>
        </Spin>
      </Modal>
    </PageContainer>
  )
}
