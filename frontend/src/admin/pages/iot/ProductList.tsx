import { useCallback, useEffect, useState } from 'react'
import {
  Button, Form, Input, Modal, Popconfirm, Select, Space, Switch, Table, Tag, Tooltip,
  Typography, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  DeleteOutlined, EditOutlined, KeyOutlined, PlusOutlined, QrcodeOutlined,
} from '@ant-design/icons'
import { QRCodeSVG } from 'qrcode.react'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { IOTController } from '../../api/generated/IOTController'
import type { IotProduct } from '../../api/generated/definitions/IotProduct'
import type { IotServiceGroup } from '../../api/generated/definitions/IotServiceGroup'
import type { IotShareQRResp } from '../../api/generated/definitions/IotShareQRResp'
import type { IotCredentials } from '../../api/generated/definitions/IotCredentials'
import type { CreateIotProductReq } from '../../api/generated/definitions/CreateIotProductReq'
import type { UpdateIotProductReq } from '../../api/generated/definitions/UpdateIotProductReq'
import { useAuthStore } from '../../store/authStore'
import { CredentialModal } from './CredentialModal'
import { notifyError } from './shared'

/** 产品表单值 */
interface ProductFormValues {
  name: string
  code: string
  category?: string
  description?: string
  auto_network: boolean
  net_config_mode: string
  allow_share: boolean
  active: boolean
  /** 产品可用的服务组（系统预置三条，多选）。 */
  service_group_ids: number[]
}

/** 配网方式字典 */
const NET_CONFIG_OPTIONS = [
  { label: '蓝牙配网', value: 'bluetooth' },
  { label: 'AP 配网', value: 'ap' },
  { label: '二维码配网', value: 'qrcode' },
]

const NET_CONFIG_LABEL: Record<string, string> = {
  bluetooth: '蓝牙',
  ap: 'AP',
  qrcode: '二维码',
}

/** 服务组类型字典（产品绑定服务组时展示） */
const SERVICE_TYPE_LABEL: Record<string, string> = {
  gb28181: 'GB28181',
  webrtc: 'WebRTC',
  aichat: 'AI聊天',
}

const ACTIVE_OPTIONS = [
  { label: '启用', value: true },
  { label: '停用', value: false },
]

/** 新表单默认值 */
function buildProductInitialValues(editing: IotProduct | null): Partial<ProductFormValues> {
  if (!editing) {
    return {      auto_network: false,
      net_config_mode: 'bluetooth',
      allow_share: false,
      active: true,
      service_group_ids: [],
    }
  }
  return {
    name: editing.name ?? '',
    code: editing.code ?? '',
    category: editing.category ?? '',
    description: editing.description ?? '',    auto_network: editing.auto_network ?? false,
    net_config_mode: editing.net_config_mode || 'bluetooth',
    allow_share: editing.allow_share ?? false,
    active: editing.active ?? true,
    service_group_ids: [],
  }
}

/**
 * IOT 产品管理。
 *
 * 产品是 IOT 模块的聚合根：决定设备的联网 / 配网方式、分享策略，
 * 并绑定设备 OTA 时下发的服务组。
 */
export default function ProductList() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('iot:product:create')
  const canUpdate = permCodes.includes('iot:product:update')
  const canDelete = permCodes.includes('iot:product:delete')

  const [data, setData] = useState<IotProduct[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [activeFilter, setActiveFilter] = useState<boolean | undefined>(undefined)
  /** 产品 → 已绑定服务组数量。列表接口不返回该字段，按当前页逐条查询。 */
  const [groupCounts, setGroupCounts] = useState<Record<number, number>>({})

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<IotProduct | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<ProductFormValues>()
  const [shareOpen, setShareOpen] = useState(false)
  const [sharing, setSharing] = useState<number | null>(null)
  const [shareProduct, setShareProduct] = useState<IotProduct | null>(null)
  const [shareQR, setShareQR] = useState<IotShareQRResp | null>(null)

  const [credOpen, setCredOpen] = useState(false)
  const [credLoading, setCredLoading] = useState<number | null>(null)
  const [credProduct, setCredProduct] = useState<IotProduct | null>(null)
  const [credData, setCredData] = useState<IotCredentials | null>(null)

  /** 系统预置的三条服务组，供产品表单多选。 */
  const [allGroups, setAllGroups] = useState<IotServiceGroup[]>([])

  /** 按当前页产品批量统计已绑定服务组数量 */
  const loadGroupCounts = useCallback(async (rows: IotProduct[]) => {
    const results = await Promise.all(rows.map(async (row) => {
      if (row.id == null) return null
      try {
        const res = await IOTController.getIotProductServiceGroups(row.id)
        return { id: row.id, count: res.data?.length ?? 0 }
      } catch {
        return null
      }
    }))
    const next: Record<number, number> = {}
    for (const item of results) {
      if (item) next[item.id] = item.count
    }
    setGroupCounts(next)
  }, [])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await IOTController.pageIotProducts(page, pageSize, keyword || undefined, activeFilter)
      const items = res.data?.items ?? []
      setData(items)
      setTotal(Number(res.data?.total ?? 0))
      void loadGroupCounts(items)
    } catch (err) {
      notifyError(err, '获取产品列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, activeFilter, loadGroupCounts])

  /** 服务组是系统预置的三条，这里一次拉全量用于表单多选。 */
  const fetchGroups = useCallback(async () => {
    try {
      const res = await IOTController.pageIotServiceGroups(1, 200)
      setAllGroups(res.data?.items ?? [])
    } catch (err) {
      notifyError(err, '获取服务组失败')
    }
  }, [])

  useEffect(() => { void fetchData() }, [fetchData])
  useEffect(() => { void fetchGroups() }, [fetchGroups])

  const handleSearch = (values: Record<string, unknown>) => {
    setKeyword(typeof values.keyword === 'string' ? values.keyword : '')
    setActiveFilter(typeof values.active === 'boolean' ? values.active : undefined)
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setActiveFilter(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue(buildProductInitialValues(null))
    setModalOpen(true)
  }

  const openEdit = async (row: IotProduct) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue(buildProductInitialValues(row))
    setModalOpen(true)
    if (row.id == null) return
    try {
      const res = await IOTController.getIotProductServiceGroups(row.id)
      form.setFieldValue(
        'service_group_ids',
        (res.data ?? []).flatMap((g) => (g.id == null ? [] : [g.id])),
      )
    } catch (err) {
      notifyError(err, '获取产品服务组失败')
    }
  }

  const handleSave = async () => {
    let values: ProductFormValues
    try {
      values = await form.validateFields()
    } catch {
      return
    }

    setSaving(true)
    try {
      if (editing?.id != null) {
        const payload: UpdateIotProductReq = {
          name: values.name,
          code: values.code,
          category: values.category,
          description: values.description,
          auto_network: values.auto_network,
          net_config_mode: values.net_config_mode,
          allow_share: values.allow_share,
          active: values.active,
          // 服务组随产品表单整体替换；后端同步产品×服务凭据。
          service_group_ids: values.service_group_ids ?? [],
        }
        const res = await IOTController.updateIotProduct(editing.id, payload)
        if (res.code === 200) {
          message.success('修改成功')
          setModalOpen(false)
          void fetchData()
        } else {
          message.error(res.message || '保存失败')
        }
      } else {
        const payload: CreateIotProductReq = {
          name: values.name,
          code: values.code,
          category: values.category,
          description: values.description,
          auto_network: values.auto_network,
          net_config_mode: values.net_config_mode,
          allow_share: values.allow_share,
          active: values.active,
          service_group_ids: values.service_group_ids ?? [],
        }
        const res = await IOTController.createIotProduct(payload)
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

  const handleDelete = async (row: IotProduct) => {
    if (row.id == null) return
    try {
      const res = await IOTController.deleteIotProduct(row.id)
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

  const handleShare = async (row: IotProduct) => {
    if (row.id == null) return
    if (!row.allow_share) {
      message.warning('该产品未开启允许分享')
      return
    }
    setSharing(row.id)
    try {
      const res = await IOTController.generateIotProductShareQR(row.id)
      if (res.code === 200 && res.data) {
        setShareProduct(row)
        setShareQR(res.data)
        setShareOpen(true)
        void fetchData()
      } else {
        message.error(res.message || '生成分享二维码失败')
      }
    } catch (err) {
      notifyError(err, '生成分享二维码失败')
    } finally {
      setSharing(null)
    }
  }

  /** 拉取产品下全部服务组的明文鉴权信息（仅 `iot:product:update` 可调用） */
  const handleCopyCredentials = async (row: IotProduct) => {
    if (row.id == null) return
    setCredLoading(row.id)
    try {
      const res = await IOTController.getIotProductCredentials(row.id)
      if (res.code === 200 && res.data) {
        setCredProduct(row)
        setCredData(res.data)
        setCredOpen(true)
      } else {
        message.error(res.message || '获取鉴权信息失败')
      }
    } catch (err) {
      notifyError(err, '获取鉴权信息失败')
    } finally {
      setCredLoading(null)
    }
  }

  const columns: ColumnsType<IotProduct> = [
    { title: '产品名称', dataIndex: 'name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '产品编码', dataIndex: 'code', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '分类', dataIndex: 'category', width: 120, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '自动联网（4G）', dataIndex: 'auto_network', width: 130,
      render: (v?: boolean) => (v ? <Tag color="green">是</Tag> : <Tag>否</Tag>),
    },
    {
      title: '配网方式', dataIndex: 'net_config_mode', width: 110,
      render: (v?: string) => (v ? <Tag color="geekblue">{NET_CONFIG_LABEL[v] ?? v}</Tag> : '-'),
    },
    {
      title: '允许分享', dataIndex: 'allow_share', width: 100,
      render: (v?: boolean) => (v ? <Tag color="blue">是</Tag> : <Tag>否</Tag>),
    },
    {
      title: '绑定服务组数', key: 'group_count', width: 120, align: 'center',
      render: (_, row) => (row.id != null && groupCounts[row.id] !== undefined ? groupCounts[row.id] : '-'),
    },
    {
      title: '启用状态', dataIndex: 'active', width: 100,
      render: (v?: boolean) => (v ? <Tag color="green">启用</Tag> : <Tag>停用</Tag>),
    },
    {
      title: '操作', key: 'action', width: 520, fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={520}>
          {canUpdate && (
            <Button
              type="link"
              size="small"
              icon={<KeyOutlined />}
              loading={credLoading === row.id}
              onClick={() => void handleCopyCredentials(row)}
            >
              复制鉴权
            </Button>
          )}
          {canUpdate && (
            <Tooltip title={row.allow_share ? undefined : '该产品未开启允许分享'}>
              <span>
                <Button
                  type="link"
                  size="small"
                  icon={<QrcodeOutlined />}
                  disabled={!row.allow_share}
                  loading={sharing === row.id}
                  onClick={() => void handleShare(row)}
                >
                  分享二维码
                </Button>
              </span>
            </Tooltip>
          )}
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
              编辑
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除该产品?" onConfirm={() => void handleDelete(row)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  // 服务组由管理员按产品实际能力选择，平台不按「设备类型」过滤。
  /** 系统预置三条服务组，产品表单多选。 */
  const groupOptions = allGroups.flatMap((group) => (
    group.id == null
      ? []
      : [{
        label: `${group.name ?? ''}（${SERVICE_TYPE_LABEL[group.type ?? ''] ?? group.type ?? group.code ?? ''}）`,
        value: group.id,
      }]
  ))

  return (
    <PageContainer
      title="产品管理"
      subTitle="维护 IOT 产品、配网方式与分享策略，并配置设备的服务组"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增产品
          </Button>
        ) : null
      }
    >
      <SearchBar
        fields={[
          { name: 'keyword', label: '关键字', placeholder: '请输入产品名称/编码/分类' },
          { name: 'active', label: '启用状态', type: 'select', options: ACTIVE_OPTIONS },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<IotProduct>
        className="e2e-iot-product-table"
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
        className="e2e-iot-product-modal"
        title={editing ? '编辑产品' : '新增产品'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => void handleSave()}
        confirmLoading={saving}
        destroyOnHidden
        width={680}
      >
        <Form form={form} layout="vertical">
          <Space size={16} style={{ width: '100%' }} align="start">
            <Form.Item
              name="name"
              label="产品名称"
              rules={[{ required: true, message: '请输入产品名称' }]}
              style={{ width: 300 }}
            >
              <Input placeholder="如：4G 球机" maxLength={255} />
            </Form.Item>
            <Form.Item
              name="code"
              label="产品编码"
              rules={[{ required: true, message: '请输入产品编码' }]}
              style={{ width: 300 }}
            >
              <Input placeholder="如：CAM_4G_01" maxLength={100} />
            </Form.Item>
          </Space>
          <Space size={16} style={{ width: '100%' }} align="start">
            <Form.Item name="net_config_mode" label="配网方式" style={{ width: 300 }}>
              <Select options={NET_CONFIG_OPTIONS} placeholder="请选择配网方式" />
            </Form.Item>
          </Space>
          <Space size={16} style={{ width: '100%' }} align="start">
            <Form.Item name="category" label="产品分类" style={{ width: 300 }}>
              <Input placeholder="如：摄像头" maxLength={100} />
            </Form.Item>
          </Space>
          <Form.Item
            name="service_group_ids"
            label="服务组"
            extra="系统预置三条服务组；产品与设备通过它获得接入配置。被产品引用后服务组不可修改。"
            rules={[{ required: true, type: 'array', min: 1, message: '请选择至少一个服务组' }]}
          >
            <Select
              className="e2e-iot-product-groups-select"
              mode="multiple"
              allowClear
              placeholder="请选择服务组"
              options={groupOptions}
            />
          </Form.Item>
          <Form.Item name="description" label="产品描述">
            <Input.TextArea rows={3} maxLength={500} showCount placeholder="请输入产品描述" />
          </Form.Item>
          <Space size={32}>
            <Form.Item name="auto_network" label="自动联网（4G 设备）" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="allow_share" label="允许分享" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="active" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
        </Form>
      </Modal>

      <Modal
        className="e2e-iot-product-share-modal"
        title={shareProduct ? `分享二维码 - ${shareProduct.name ?? ''}` : '分享二维码'}
        open={shareOpen}
        onCancel={() => setShareOpen(false)}
        footer={<Button onClick={() => setShareOpen(false)}>关闭</Button>}
        width={520}
        destroyOnHidden
      >
        {shareQR?.content ? (
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <div
              style={{
                border: '1px solid #d9d9d9',
                borderRadius: 6,
                padding: 12,
                background: '#fff',
                display: 'inline-block',
              }}
            >
              <QRCodeSVG value={shareQR.content} size={180} level="M" title="产品分享二维码" />
            </div>
            <div
              style={{
                border: '1px solid #d9d9d9',
                borderRadius: 6,
                padding: '8px 12px',
                background: '#fafafa',
              }}
            >
              <Typography.Paragraph
                copyable={{ text: shareQR.content }}
                style={{ marginBottom: 0, fontFamily: 'monospace', wordBreak: 'break-all' }}
              >
                {shareQR.content}
              </Typography.Paragraph>
            </div>
            {shareQR.product_code ? (
              <Typography.Text type="secondary">产品编码：{shareQR.product_code}</Typography.Text>
            ) : null}
          </Space>
        ) : (
          <Typography.Text type="secondary">暂无二维码内容，请重新生成。</Typography.Text>
        )}
      </Modal>

      <CredentialModal
        open={credOpen}
        title={credProduct ? `鉴权信息 - ${credProduct.name ?? ''}` : '鉴权信息'}
        data={credData}
        onClose={() => setCredOpen(false)}
      />
    </PageContainer>
  )
}
