import { useCallback, useEffect, useState } from 'react'
import {
  Button, Col, Form, Input, Modal, Popconfirm, Row, Select, Space, Table, Tag, Tooltip,
  Typography, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  ApiOutlined, ClusterOutlined, DeleteOutlined, DisconnectOutlined, EditOutlined, LinkOutlined,
  PlusOutlined, QrcodeOutlined, SafetyOutlined, ShareAltOutlined, StopOutlined, UserOutlined,
} from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import StatusTag from '../../components/StatusTag'
import { IOTController } from '../../api/generated/IOTController'
import type { Device } from '../../api/generated/definitions/Device'
import type { IotProduct } from '../../api/generated/definitions/IotProduct'
import type { IotDeviceAccount } from '../../api/generated/definitions/IotDeviceAccount'
import type { IotDeviceActivated } from '../../api/generated/definitions/IotDeviceActivated'
import type { IotDeviceSecret } from '../../api/generated/definitions/IotDeviceSecret'
import type { IotDeviceQR } from '../../api/generated/definitions/IotDeviceQR'
import type { IotDeviceShareToken } from '../../api/generated/definitions/IotDeviceShareToken'
import type { IotServiceGroupDetail } from '../../api/generated/definitions/IotServiceGroupDetail'
import type { CreateIotDeviceAccountReq } from '../../api/generated/definitions/CreateIotDeviceAccountReq'
import type { UpdateIotDeviceAccountReq } from '../../api/generated/definitions/UpdateIotDeviceAccountReq'
import { useAuthStore } from '../../store/authStore'
import { DeviceQrModal, ShareQrModal } from './DeviceQrModal'
import { SecretModal, fmtDT, notifyError } from './shared'

const STATUS_OPTIONS = [
  { label: '在线', value: 'online' },
  { label: '离线', value: 'offline' },
]

const ACCOUNT_STATUS_OPTIONS = [
  { label: '正常', value: 'active' },
  { label: '停用', value: 'disabled' },
]

interface DeviceProductFormValues {
  product_id?: number
}

interface AccountFormValues {
  username?: string
  password?: string
  nickname?: string
  phone?: string
  status?: string
}

/** 一次性凭据弹窗的数据 */
interface OneTimeCredential {
  title: string
  items: { label: string; value?: string | number | null }[]
  warning?: string
}

/**
 * IOT 设备管理。
 *
 * 设备数据复用 `ac_device`，本页只维护 IOT 视角的字段：
 * 所属产品、激活状态（主账号）、接入密钥与分享账号。
 */
export default function DeviceList() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canUpdate = permCodes.includes('iot:device:update')
  const canActivate = permCodes.includes('iot:device:activate')
  const canResetSecret = permCodes.includes('iot:device:secret-reset')
  const canUnbind = permCodes.includes('iot:device:unbind')
  const canAccountCreate = permCodes.includes('iot:device:account:create')
  const canAccountUpdate = permCodes.includes('iot:device:account:update')
  const canAccountDelete = permCodes.includes('iot:device:account:delete')

  const [data, setData] = useState<Device[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [productFilter, setProductFilter] = useState<number | undefined>(undefined)
  const [statusFilter, setStatusFilter] = useState<string | undefined>(undefined)
  const [products, setProducts] = useState<IotProduct[]>([])
  /** 设备 → 主账号用户名。列表接口不返回账号，按当前页逐条查询详情。 */
  const [ownerMap, setOwnerMap] = useState<Record<number, string>>({})

  const [productOpen, setProductOpen] = useState(false)
  const [productTarget, setProductTarget] = useState<Device | null>(null)
  const [productSaving, setProductSaving] = useState(false)
  const [productForm] = Form.useForm<DeviceProductFormValues>()

  const [accountOpen, setAccountOpen] = useState(false)
  const [accountDevice, setAccountDevice] = useState<Device | null>(null)
  const [accounts, setAccounts] = useState<IotDeviceAccount[]>([])
  const [accountsLoading, setAccountsLoading] = useState(false)

  const [accountFormOpen, setAccountFormOpen] = useState(false)
  const [accountEditing, setAccountEditing] = useState<IotDeviceAccount | null>(null)
  const [accountSaving, setAccountSaving] = useState(false)
  const [accountForm] = Form.useForm<AccountFormValues>()

  const [credentialOpen, setCredentialOpen] = useState(false)
  const [credential, setCredential] = useState<OneTimeCredential | null>(null)

  const [secretOpen, setSecretOpen] = useState(false)
  const [secret, setSecret] = useState<IotDeviceSecret | null>(null)

  const [qrOpen, setQrOpen] = useState(false)
  const [qrLoading, setQrLoading] = useState<number | null>(null)
  const [qrDevice, setQrDevice] = useState<Device | null>(null)
  const [qrData, setQrData] = useState<IotDeviceQR | null>(null)

  const [shareQrOpen, setShareQrOpen] = useState(false)
  const [shareLoading, setShareLoading] = useState<number | null>(null)
  const [shareQrDevice, setShareQrDevice] = useState<Device | null>(null)
  const [shareQrData, setShareQrData] = useState<IotDeviceShareToken | null>(null)
  const [revoking, setRevoking] = useState<number | null>(null)
  const [unbinding, setUnbinding] = useState<number | null>(null)

  const [servicesOpen, setServicesOpen] = useState(false)
  const [servicesLoading, setServicesLoading] = useState<number | null>(null)
  const [servicesDevice, setServicesDevice] = useState<Device | null>(null)
  const [services, setServices] = useState<IotServiceGroupDetail[]>([])

  const productOptions = products.flatMap((product) => (
    product.id == null
      ? []
      : [{ label: `${product.name ?? ''}（${product.code ?? ''}）`, value: product.id }]
  ))

  const fetchProducts = useCallback(async () => {
    try {
      const res = await IOTController.pageIotProducts(1, 200)
      setProducts(res.data?.items ?? [])
    } catch {
      // 产品下拉不可用不应阻断设备列表；行内「所属产品」仍由列表接口返回的 product 展示
    }
  }, [])

  const loadOwners = useCallback(async (rows: Device[]) => {
    const results = await Promise.all(rows.map(async (row) => {
      if (row.id == null) return null
      try {
        const res = await IOTController.getIotDevice(row.id)
        const owner = res.data?.accounts?.find((account) => account.role === 'owner')
        return { id: row.id, username: owner?.username ?? '' }
      } catch {
        return null
      }
    }))
    const next: Record<number, string> = {}
    for (const item of results) {
      if (item) next[item.id] = item.username
    }
    setOwnerMap(next)
  }, [])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await IOTController.pageIotDevices(
        page, pageSize, keyword || undefined, productFilter, statusFilter,
      )
      const items = res.data?.items ?? []
      setData(items)
      setTotal(Number(res.data?.total ?? 0))
      void loadOwners(items)
    } catch (err) {
      notifyError(err, '获取设备列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, productFilter, statusFilter, loadOwners])

  useEffect(() => { void fetchData() }, [fetchData])
  useEffect(() => { void fetchProducts() }, [fetchProducts])

  const handleSearch = (values: Record<string, unknown>) => {
    setKeyword(typeof values.keyword === 'string' ? values.keyword : '')
    setProductFilter(typeof values.product_id === 'number' ? values.product_id : undefined)
    setStatusFilter(typeof values.status === 'string' && values.status ? values.status : undefined)
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setProductFilter(undefined)
    setStatusFilter(undefined)
    setPage(1)
  }

  // ── 设置产品 ──────────────────────────────────────────────

  const openSetProduct = (row: Device) => {
    setProductTarget(row)
    productForm.resetFields()
    productForm.setFieldsValue({ product_id: row.product_id })
    setProductOpen(true)
  }

  const handleSaveProduct = async () => {
    if (productTarget?.id == null) return
    let values: DeviceProductFormValues
    try {
      values = await productForm.validateFields()
    } catch {
      return
    }

    setProductSaving(true)
    try {
      const res = await IOTController.updateIotDeviceProduct(productTarget.id, {
        product_id: values.product_id,
      })
      if (res.code === 200) {
        message.success('设置成功')
        setProductOpen(false)
        void fetchData()
      } else {
        message.error(res.message || '设置失败')
      }
    } catch (err) {
      notifyError(err, '设置失败')
    } finally {
      setProductSaving(false)
    }
  }

  // ── 账号管理 ──────────────────────────────────────────────

  const loadAccounts = useCallback(async (device: Device) => {
    if (device.id == null) return
    const deviceId = device.id
    setAccountsLoading(true)
    try {
      const res = await IOTController.getIotDevice(deviceId)
      const list = res.data?.accounts ?? []
      setAccounts(list)
      const owner = list.find((account) => account.role === 'owner')
      setOwnerMap((prev) => ({ ...prev, [deviceId]: owner?.username ?? '' }))
    } catch (err) {
      notifyError(err, '获取设备账号失败')
    } finally {
      setAccountsLoading(false)
    }
  }, [])

  const openAccounts = (row: Device) => {
    setAccountDevice(row)
    setAccounts([])
    setAccountOpen(true)
    void loadAccounts(row)
  }

  const openAccountCreate = () => {
    setAccountEditing(null)
    accountForm.resetFields()
    accountForm.setFieldsValue({ status: 'active' })
    setAccountFormOpen(true)
  }

  const openAccountEdit = (account: IotDeviceAccount) => {
    setAccountEditing(account)
    accountForm.resetFields()
    accountForm.setFieldsValue({
      username: account.username ?? '',
      nickname: account.nickname ?? '',
      phone: account.phone ?? '',
      status: account.status || 'active',
      password: '',
    })
    setAccountFormOpen(true)
  }

  const handleSaveAccount = async () => {
    if (accountDevice?.id == null) return
    let values: AccountFormValues
    try {
      values = await accountForm.validateFields()
    } catch {
      return
    }

    setAccountSaving(true)
    try {
      if (accountEditing?.id != null) {
        const payload: UpdateIotDeviceAccountReq = {
          username: values.username?.trim() || undefined,
          nickname: values.nickname?.trim() || undefined,
          phone: values.phone?.trim() || undefined,
          status: values.status,
        }
        if (values.password) payload.password = values.password
        const res = await IOTController.updateIotDeviceAccount(
          accountDevice.id, accountEditing.id, payload,
        )
        if (res.code === 200) {
          message.success('修改成功')
          setAccountFormOpen(false)
          void loadAccounts(accountDevice)
        } else {
          message.error(res.message || '保存失败')
        }
      } else {
        const payload: CreateIotDeviceAccountReq = {
          username: values.username?.trim() || undefined,
          password: values.password || undefined,
          role: 'shared',
          nickname: values.nickname?.trim() || undefined,
          phone: values.phone?.trim() || undefined,
          status: values.status,
        }
        const res = await IOTController.createIotDeviceAccount(accountDevice.id, payload)
        if (res.code === 200) {
          message.success('新增成功')
          setAccountFormOpen(false)
          void loadAccounts(accountDevice)
          const plain = res.data?.password || values.password
          if (plain) {
            setCredential({
              title: '分享账号已创建',
              warning: '密码仅显示一次，关闭后无法再次查看，请立即复制并妥善保存',
              items: [
                { label: '用户名', value: res.data?.account?.username ?? values.username },
                { label: '密码', value: plain },
                { label: '设备', value: accountDevice.name || accountDevice.device_code },
              ],
            })
            setCredentialOpen(true)
          }
        } else {
          message.error(res.message || '保存失败')
        }
      }
    } catch (err) {
      notifyError(err, '保存失败')
    } finally {
      setAccountSaving(false)
    }
  }

  const handleDeleteAccount = async (account: IotDeviceAccount) => {
    if (accountDevice?.id == null || account.id == null) return
    try {
      const res = await IOTController.deleteIotDeviceAccount(accountDevice.id, account.id)
      if (res.code === 200) {
        message.success('删除成功')
        void loadAccounts(accountDevice)
      } else {
        message.error(res.message || '删除失败')
      }
    } catch (err) {
      notifyError(err, '删除失败')
    }
  }

  // ── 激活 / 重置密钥 ───────────────────────────────────────

  const handleActivate = async (row: Device) => {
    if (row.id == null) return
    if (row.activated_at) {
      message.info('该设备已激活，请勿重复激活')
      return
    }
    try {
      const res = await IOTController.activateIotDevice(row.id)
      if (res.code === 200 && res.data) {
        const activated: IotDeviceActivated = res.data
        if (activated.password) {
          setCredential({
            title: '设备激活成功',
            warning: '主账号密码仅显示一次，关闭后无法再次查看，请立即复制并妥善保存',
            items: [
              { label: '用户名', value: activated.username },
              { label: '密码', value: activated.password },
              { label: '设备编号', value: activated.device_code },
              { label: '激活时间', value: fmtDT(activated.activated_at) },
            ],
          })
          setCredentialOpen(true)
        } else {
          message.info(`该设备已激活，主账号：${activated.username || '-'}`)
        }
        void fetchData()
      } else {
        message.error(res.message || '激活失败')
      }
    } catch (err) {
      notifyError(err, '激活失败')
    }
  }

  const handleResetSecret = async (row: Device) => {
    if (row.id == null) return
    try {
      const res = await IOTController.resetIotDeviceSecret(row.id)
      if (res.code === 200 && res.data) {
        setSecret(res.data)
        setSecretOpen(true)
        void fetchData()
      } else {
        message.error(res.message || '重置密钥失败')
      }
    } catch (err) {
      notifyError(err, '重置密钥失败')
    }
  }

  // ── 设备二维码 / 分享码 ───────────────────────────────────

  const handleDeviceQR = async (row: Device) => {
    if (row.id == null) return
    setQrLoading(row.id)
    try {
      const res = await IOTController.getIotDeviceQR(row.id)
      if (res.code === 200 && res.data) {
        setQrDevice(row)
        setQrData(res.data)
        setQrOpen(true)
      } else {
        message.error(res.message || '获取设备二维码失败')
      }
    } catch (err) {
      notifyError(err, '获取设备二维码失败')
    } finally {
      setQrLoading(null)
    }
  }

  const handleCreateShare = async (row: Device) => {
    if (row.id == null) return
    setShareLoading(row.id)
    try {
      const res = await IOTController.createIotDeviceShareToken(row.id)
      if (res.code === 200 && res.data) {
        setShareQrDevice(row)
        setShareQrData(res.data)
        setShareQrOpen(true)
        // 分享码创建时会生成占位分享账号，账号列表需要刷新。
        if (accountDevice?.id === row.id) void loadAccounts(row)
      } else {
        message.error(res.message || '生成分享码失败')
      }
    } catch (err) {
      notifyError(err, '生成分享码失败')
    } finally {
      setShareLoading(null)
    }
  }

  const handleRevokeShare = async (row: Device) => {
    if (row.id == null) return
    setRevoking(row.id)
    try {
      const res = await IOTController.revokeIotDeviceShareToken(row.id)
      if (res.code === 200) {
        message.success(`已撤销 ${res.data?.revoked ?? 0} 个未认领分享码`)
        if (accountDevice?.id === row.id) void loadAccounts(row)
      } else {
        message.error(res.message || '撤销分享码失败')
      }
    } catch (err) {
      notifyError(err, '撤销分享码失败')
    } finally {
      setRevoking(null)
    }
  }

  /** 查看设备已授权的服务组（来自产品绑定，OTA 下发同一份数据）。 */
  const openServices = async (row: Device) => {
    if (row.id == null) return
    setServicesDevice(row)
    setServices([])
    setServicesOpen(true)
    setServicesLoading(row.id)
    try {
      const res = await IOTController.getIotDevice(row.id)
      setServices(res.data?.service_groups ?? [])
    } catch (err) {
      notifyError(err, '获取设备服务授权失败')
    } finally {
      setServicesLoading(null)
    }
  }

  /** 后台解绑：删除全部账号（含主账号）并清除激活状态。 */
  const handleUnbind = async (row: Device) => {
    if (row.id == null) return
    setUnbinding(row.id)
    try {
      const res = await IOTController.unbindIotDevice(row.id)
      if (res.code === 200) {
        message.success(`已解绑，删除 ${res.data?.accounts ?? 0} 个账号`)
        void fetchData()
      } else {
        message.error(res.message || '解绑失败')
      }
    } catch (err) {
      notifyError(err, '解绑失败')
    } finally {
      setUnbinding(null)
    }
  }

  // ── 表格 ──────────────────────────────────────────────────

  const columns: ColumnsType<Device> = [
    { title: '设备名称', dataIndex: 'name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '设备编号', dataIndex: 'device_code', width: 170, ellipsis: true,
      render: (v?: string) => (v ? <Typography.Text copyable={{ text: v }}>{v}</Typography.Text> : '-'),
    },
    {
      title: '所属产品', dataIndex: 'product', width: 180, ellipsis: true,
      render: (_, row) => (row.product?.name
        ? <span>{row.product.name}{row.product.code ? `（${row.product.code}）` : ''}</span>
        : <Typography.Text type="secondary">未绑定</Typography.Text>),
    },
    {
      title: '在线状态', dataIndex: 'status', width: 100,
      render: (v?: string) => <StatusTag type="device" status={v ?? 'offline'} />,
    },
    {
      title: '激活状态', dataIndex: 'activated_at', width: 160,
      render: (v?: string) => (v
        ? <Tag color="green" title={fmtDT(v)}>已激活</Tag>
        : <Tag>未激活</Tag>),
    },
    {
      title: '主账号', key: 'owner', width: 180, ellipsis: true,
      render: (_, row) => {
        const owner = row.id != null ? ownerMap[row.id] : undefined
        return owner ? <Typography.Text copyable={{ text: owner }}>{owner}</Typography.Text> : '-'
      },
    },
    {
      title: '启用', dataIndex: 'active', width: 90,
      render: (v?: boolean) => (v ? <Tag color="green">启用</Tag> : <Tag>停用</Tag>),
    },
    {
      title: '操作', key: 'action', width: 760, fixed: 'right',
      render: (_, row) => {
        // 产品关闭分享时后端会拒绝生成分享码；按钮同步禁用，避免无效点击。
        // 未绑定产品（历史设备）不阻断，遵循后端规则。
        const shareAllowed = row.product?.allow_share !== false
        return (
          <ActionButtons maxWidth={760}>
            <Button type="link" size="small" icon={<UserOutlined />} onClick={() => openAccounts(row)}>
              账号管理
            </Button>
            <Button
              type="link"
              size="small"
              icon={<ClusterOutlined />}
              loading={servicesLoading === row.id}
              onClick={() => void openServices(row)}
            >
              服务授权
            </Button>
            {canUpdate && (
              <Button
                type="link"
                size="small"
                icon={<QrcodeOutlined />}
                loading={qrLoading === row.id}
                onClick={() => void handleDeviceQR(row)}
              >
                设备二维码
              </Button>
            )}
            {canAccountCreate && (
              <Tooltip title={shareAllowed ? undefined : '该产品未开启允许分享'}>
                <span>
                  <Button
                    type="link"
                    size="small"
                    icon={<ShareAltOutlined />}
                    disabled={!shareAllowed}
                    loading={shareLoading === row.id}
                    onClick={() => void handleCreateShare(row)}
                  >
                    分享码
                  </Button>
                </span>
              </Tooltip>
            )}
          {canAccountDelete && (
            <Popconfirm
              title="撤销后该设备所有未认领的分享码立即失效，已认领的分享账号不受影响。确定撤销?"
              onConfirm={() => void handleRevokeShare(row)}
            >
              <Button
                type="link"
                size="small"
                danger
                icon={<StopOutlined />}
                loading={revoking === row.id}
              >
                撤销分享码
              </Button>
            </Popconfirm>
          )}
          {canUpdate && (
            <Button type="link" size="small" icon={<LinkOutlined />} onClick={() => openSetProduct(row)}>
              设置产品
            </Button>
          )}
          {canActivate && (
            <Tooltip title={row.activated_at ? '设备已激活，请勿重复激活' : undefined}>
              <span>
                <Button
                  type="link"
                  size="small"
                  icon={<ApiOutlined />}
                  onClick={() => void handleActivate(row)}
                >
                  激活设备
                </Button>
              </span>
            </Tooltip>
          )}
            {canResetSecret && (
              <Popconfirm
                title="重置后旧密钥立即失效，设备需重新配置新密钥。确定重置?"
                onConfirm={() => void handleResetSecret(row)}
              >
                <Button type="link" size="small" danger icon={<SafetyOutlined />}>
                  重置密钥
                </Button>
              </Popconfirm>
            )}
            {canUnbind && (
              <Popconfirm
                title="解绑将删除该设备全部账号（含主账号）并清除激活状态，设备需重新激活。确定?"
                onConfirm={() => void handleUnbind(row)}
              >
                <Button
                  type="link"
                  size="small"
                  danger
                  icon={<DisconnectOutlined />}
                  loading={unbinding === row.id}
                >
                  解绑设备
                </Button>
              </Popconfirm>
            )}
          </ActionButtons>
        )
      },
    },
  ]

  const accountColumns: ColumnsType<IotDeviceAccount> = [
    {
      title: '用户名', dataIndex: 'username', width: 180, ellipsis: true,
      render: (v?: string) => (v ? <Typography.Text copyable={{ text: v }}>{v}</Typography.Text> : '-'),
    },
    { title: '昵称', dataIndex: 'nickname', width: 120, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '手机号', dataIndex: 'phone', width: 130, render: (v?: string) => v || '-' },
    {
      title: '角色', dataIndex: 'role', width: 100,
      render: (v?: string) => (v === 'owner' ? <Tag color="gold">主账号</Tag> : <Tag color="blue">分享账号</Tag>),
    },
    {
      title: '状态', dataIndex: 'status', width: 90,
      render: (v?: string) => (v === 'disabled' ? <Tag>停用</Tag> : <Tag color="green">正常</Tag>),
    },
    { title: '激活时间', dataIndex: 'activated_at', width: 160, render: (v?: string) => fmtDT(v) },
    {
      title: '操作', key: 'action', width: 170,
      render: (_, account) => {
        if (account.role === 'owner') {
          return <Typography.Text type="secondary">主账号不可删除</Typography.Text>
        }
        return (
          <ActionButtons maxWidth={170}>
            {canAccountUpdate && (
              <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openAccountEdit(account)}>
                编辑
              </Button>
            )}
            {canAccountDelete && (
              <Popconfirm title="确定删除该分享账号?" onConfirm={() => void handleDeleteAccount(account)}>
                <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                  删除
                </Button>
              </Popconfirm>
            )}
          </ActionButtons>
        )
      },
    },
  ]

  return (
    <PageContainer
      title="设备管理"
      subTitle="维护设备的所属产品、主账号激活与接入密钥；设备数据与平台设备共用一套"
    >
      <SearchBar
        fields={[
          { name: 'keyword', label: '关键字', placeholder: '请输入设备名称/编号/位置' },
          { name: 'product_id', label: '所属产品', type: 'select', options: productOptions },
          { name: 'status', label: '在线状态', type: 'select', options: STATUS_OPTIONS },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<Device>
        className="e2e-iot-device-table"
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
        className="e2e-iot-device-product-modal"
        title={productTarget ? `设置产品 - ${productTarget.name ?? ''}` : '设置产品'}
        open={productOpen}
        onCancel={() => setProductOpen(false)}
        onOk={() => void handleSaveProduct()}
        confirmLoading={productSaving}
        destroyOnHidden
        width={560}
      >
        <Form form={productForm} layout="vertical">
          <Form.Item
            name="product_id"
            label="所属产品"
            extra="留空表示解绑产品；解绑后设备 OTA 将不再下发服务组。"
          >
            <Select
              allowClear
              showSearch
              optionFilterProp="label"
              options={productOptions}
              placeholder="请选择产品"
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        className="e2e-iot-device-account-modal"
        title={accountDevice ? `账号管理 - ${accountDevice.name ?? accountDevice.device_code ?? ''}` : '账号管理'}
        open={accountOpen}
        onCancel={() => setAccountOpen(false)}
        footer={<Button onClick={() => setAccountOpen(false)}>关闭</Button>}
        width={920}
        destroyOnHidden
      >
        <Space style={{ marginBottom: 12 }}>
          {canAccountCreate && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openAccountCreate}>
              新增分享账号
            </Button>
          )}
        </Space>
        <Table<IotDeviceAccount>
          className="e2e-iot-device-account-table"
          rowKey="id"
          size="small"
          loading={accountsLoading}
          columns={accountColumns}
          dataSource={accounts}
          pagination={false}
          scroll={{ x: 'max-content' }}
          locale={{ emptyText: '暂无账号，设备尚未激活' }}
        />
      </Modal>

      <Modal
        className="e2e-iot-device-account-form-modal"
        title={accountEditing ? '编辑分享账号' : '新增分享账号'}
        open={accountFormOpen}
        onCancel={() => setAccountFormOpen(false)}
        onOk={() => void handleSaveAccount()}
        confirmLoading={accountSaving}
        destroyOnHidden
        width={620}
      >
        <Form form={accountForm} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="username"
                label="登录名"
                extra={accountEditing ? undefined : '留空则由系统自动生成'}
              >
                <Input placeholder={accountEditing ? '登录名' : '留空自动生成'} maxLength={100} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="password"
                label="密码"
                extra={accountEditing ? '留空表示不修改' : '留空则由系统生成 16 位随机密码'}
              >
                <Input.Password
                  placeholder={accountEditing ? '留空表示不修改' : '留空自动生成'}
                  autoComplete="new-password"
                  maxLength={100}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="nickname" label="昵称">
                <Input placeholder="如：家人账号" maxLength={100} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="phone" label="手机号">
                <Input placeholder="请输入手机号" maxLength={50} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="status" label="状态">
                <Select options={ACCOUNT_STATUS_OPTIONS} placeholder="请选择状态" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>

      <SecretModal
        className="e2e-iot-device-account-credential-modal"
        open={credentialOpen}
        title={credential?.title ?? '一次性凭据'}
        warning={credential?.warning}
        items={credential?.items ?? []}
        onClose={() => setCredentialOpen(false)}
      />

      <SecretModal
        className="e2e-iot-device-secret-modal"
        open={secretOpen}
        title={secret ? `接入密钥 - ${secret.device_code ?? ''}` : '接入密钥'}
        warning="设备接入密钥仅显示一次，关闭后无法再次查看；请立即同步到设备，否则设备将无法继续 OTA / 激活"
        items={[
          { label: '设备编号', value: secret?.device_code },
          { label: '设备密钥', value: secret?.device_secret },
        ]}
        onClose={() => setSecretOpen(false)}
      />

      <DeviceQrModal
        open={qrOpen}
        deviceLabel={qrDevice?.name || qrDevice?.device_code}
        data={qrData}
        onClose={() => setQrOpen(false)}
      />

      <ShareQrModal
        open={shareQrOpen}
        deviceLabel={shareQrDevice?.name || shareQrDevice?.device_code}
        data={shareQrData}
        onClose={() => setShareQrOpen(false)}
      />

      <Modal
        className="e2e-iot-device-services-modal"
        title={servicesDevice ? `服务授权 - ${servicesDevice.name ?? servicesDevice.device_code ?? ''}` : '服务授权'}
        open={servicesOpen}
        onCancel={() => setServicesOpen(false)}
        footer={<Button onClick={() => setServicesOpen(false)}>关闭</Button>}
        width={820}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          设备通过所属产品获得服务组授权；OTA 下发的服务项与这里展示的一致。
        </Typography.Paragraph>
        <Table<IotServiceGroupDetail>
          className="e2e-iot-device-services-table"
          rowKey={(row) => String(row.id ?? row.code ?? '')}
          size="small"
          loading={servicesLoading != null}
          dataSource={services}
          pagination={false}
          scroll={{ x: 'max-content' }}
          locale={{ emptyText: '设备未绑定产品，或产品尚未绑定服务组' }}
          columns={[
            { title: '服务组名称', dataIndex: 'name', width: 180, render: (v?: string) => v || '-' },
            { title: '编码', dataIndex: 'code', width: 170, render: (v?: string) => v || '-' },
            {
              title: '类型', dataIndex: 'type', width: 110,
              render: (v?: string) => <Tag color="geekblue">{v || '-'}</Tag>,
            },
            {
              title: '服务', key: 'services',
              render: (_, row) => {
                const services = row.services ?? []
                if (services.length === 0) return <Typography.Text type="secondary">未配置</Typography.Text>
                return (
                  <Space size={[4, 4]} wrap>
                    {services.map((item, index) => (
                      <Tag key={`${item.id ?? item.code ?? index}`} color={item.active ? 'blue' : 'default'}>
                        {item.name || item.protocol}
                        {item.host ? ` ${item.host}` : ''}
                        {item.port ? `:${item.port}` : ''}
                      </Tag>
                    ))}
                  </Space>
                )
              },
            },
            {
              title: '启用', dataIndex: 'active', width: 90,
              render: (v?: boolean) => (v ? <Tag color="green">启用</Tag> : <Tag>停用</Tag>),
            },
          ]}
        />
      </Modal>
    </PageContainer>
  )
}
