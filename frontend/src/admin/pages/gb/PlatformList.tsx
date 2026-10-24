import React, {
  forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState,
} from 'react'
import {
  Alert, Button, Form, Input, InputNumber, message, Modal, Popconfirm, Select, Space, Switch, Table, Tag, Transfer,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  ApartmentOutlined, DeleteOutlined, EditOutlined, PlusOutlined, ReloadOutlined, ShareAltOutlined,
} from '@ant-design/icons'
import axios from 'axios'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { GbPlatformController } from '../../api/generated/GbPlatformController'
import { GbGroupController } from '../../api/generated/GbGroupController'
import { BasicController } from '../../api/generated/BasicController'
import type { GbPlatformItem } from '../../api/generated/definitions/GbPlatformItem'
import type { CreateGbPlatformReq } from '../../api/generated/definitions/CreateGbPlatformReq'
import type { UpdateGbPlatformReq } from '../../api/generated/definitions/UpdateGbPlatformReq'
import { useAuthStore } from '../../store/authStore'
import { formatDT } from '../../types/business'

const STATUS_OPTIONS = [
  { label: '在线（ON）', value: 'ON' },
  { label: '离线（OFF）', value: 'OFF' },
]

const TRANSPORT_OPTIONS = [
  { label: 'UDP', value: 'UDP' },
  { label: 'TCP', value: 'TCP' },
]

/** 请求失败提示：优先展示后端返回的 message。 */
function apiErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError<{ message?: string }>(err)) {
    return err.response?.data?.message || err.message || fallback
  }
  return err instanceof Error ? err.message : fallback
}

/** 上级平台表单值 */
interface PlatformFormValues {
  name: string
  server_gb_id: string
  server_gb_domain?: string
  server_ip?: string
  server_port?: number
  username?: string
  password?: string
  transport?: string
  enable?: boolean
}

/** 新建 / 编辑的初始值。 */
function buildPlatformInitialValues(editing: GbPlatformItem | null): Partial<PlatformFormValues> {
  if (!editing) {
    return { transport: 'UDP', server_port: 5060, enable: true }
  }
  return {
    name: editing.name ?? '',
    server_gb_id: editing.server_gb_id ?? '',
    server_gb_domain: editing.server_gb_domain ?? '',
    server_ip: editing.server_ip ?? '',
    server_port: editing.server_port ?? 5060,
    username: editing.username ?? '',
    // password 由后端脱敏返回，编辑时不回填；留空表示不修改。
    password: undefined,
    transport: editing.transport ?? 'UDP',
    enable: editing.enable ?? true,
  }
}

interface PlatformFormHandle {
  submit: () => Promise<void>
}

interface PlatformFormContentProps {
  editing: GbPlatformItem | null
  onSubmit: (values: PlatformFormValues) => Promise<void>
}

/**
 * 上级平台表单内容。
 *
 * form 与 Form 同挂载在 Modal children 中（而非页面组件），
 * 避免弹窗未打开时调用 resetFields 触发 useForm 未连接告警。
 */
const PlatformFormContent = forwardRef<PlatformFormHandle, PlatformFormContentProps>(
  function PlatformFormContent({ editing, onSubmit }, ref) {
    const [form] = Form.useForm<PlatformFormValues>()

    useImperativeHandle(ref, () => ({
      async submit() {
        let values: PlatformFormValues
        try {
          values = await form.validateFields()
        } catch {
          return
        }
        await onSubmit(values)
      },
    }), [form, onSubmit])

    return (
      <Form form={form} layout="vertical" initialValues={buildPlatformInitialValues(editing)}>
        <Form.Item name="name" label="平台名称" rules={[{ required: true, message: '请输入平台名称' }]}>
          <Input placeholder="请输入上级平台名称" maxLength={100} />
        </Form.Item>
        <Form.Item
          name="server_gb_id"
          label="平台编码"
          rules={[{ required: true, message: '请输入平台国标编码' }]}
        >
          <Input placeholder="上级平台 SIP 国标编码，如 34020000002000000001" maxLength={20} />
        </Form.Item>
        <Form.Item name="server_gb_domain" label="SIP 域">
          <Input placeholder="上级平台的 SIP 域（可选）" maxLength={50} />
        </Form.Item>
        <Space size={16} style={{ display: 'flex' }}>
          <Form.Item name="server_ip" label="地址" style={{ flex: 1 }}>
            <Input placeholder="上级平台 IP / 域名" maxLength={100} />
          </Form.Item>
          <Form.Item name="server_port" label="端口" style={{ width: 160 }}>
            <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="5060" />
          </Form.Item>
        </Space>
        <Form.Item name="username" label="用户名">
          <Input placeholder="SIP 注册用户名" maxLength={50} autoComplete="off" />
        </Form.Item>
        <Form.Item
          name="password"
          label="密码"
          extra={editing ? '后端脱敏返回；留空表示不修改' : 'SIP 注册密码（可选）'}
        >
          <Input.Password
            placeholder={editing ? '留空表示不修改' : '请输入密码'}
            maxLength={100}
            autoComplete="new-password"
          />
        </Form.Item>
        <Form.Item name="transport" label="传输协议">
          <Select options={TRANSPORT_OPTIONS} />
        </Form.Item>
        <Form.Item name="enable" label="启用状态" valuePropName="checked">
          <Switch checkedChildren="启用" unCheckedChildren="停用" />
        </Form.Item>
      </Form>
    )
  },
)

type ScopeKind = 'groups' | 'regions'

interface ScopeItem {
  key: string
  title: string
}

/**
 * 上级平台：本平台作为下级向上级平台级联的配置。
 *
 * 本期只做「配置 + 映射」：不发起 REGISTER、不推送目录、不处理上级 INVITE。
 * 分组 / 区域可见范围决定后续「按分组 / 按区域推送目录」的目录内容。
 */
const PlatformList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('gb:platform:create')
  const canUpdate = permCodes.includes('gb:platform:update')
  const canDelete = permCodes.includes('gb:platform:delete')
  const canSetGroups = permCodes.includes('gb:platform:groups')
  const canSetRegions = permCodes.includes('gb:platform:regions')

  const [data, setData] = useState<GbPlatformItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState<string | undefined>()

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<GbPlatformItem | null>(null)
  const [saving, setSaving] = useState(false)
  const formRef = useRef<PlatformFormHandle>(null)

  const [scopeKind, setScopeKind] = useState<ScopeKind | null>(null)
  const [scopeTarget, setScopeTarget] = useState<GbPlatformItem | null>(null)
  const [scopeItems, setScopeItems] = useState<ScopeItem[]>([])
  const [scopeKeys, setScopeKeys] = useState<string[]>([])
  const [scopeLoading, setScopeLoading] = useState(false)
  const [scopeSaving, setScopeSaving] = useState(false)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbPlatformController.pageGbPlatforms(page, pageSize, keyword || undefined, status)
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch (err) {
      message.error(apiErrorMessage(err, '获取上级平台列表失败'))
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, status])

  useEffect(() => { void fetchData() }, [fetchData])

  const handleSearch = (values: Record<string, unknown>) => {
    setKeyword(typeof values.keyword === 'string' ? values.keyword : '')
    setStatus(typeof values.status === 'string' && values.status ? values.status : undefined)
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setStatus(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    setModalOpen(true)
  }

  const openEdit = (row: GbPlatformItem) => {
    setEditing(row)
    setModalOpen(true)
  }

  const submit = async (values: PlatformFormValues) => {
    const base: CreateGbPlatformReq = {
      name: values.name.trim(),
      server_gb_id: values.server_gb_id.trim(),
      server_gb_domain: values.server_gb_domain?.trim(),
      server_ip: values.server_ip?.trim(),
      server_port: values.server_port,
      username: values.username?.trim(),
      transport: values.transport,
      enable: values.enable,
      active: values.enable,
    }
    setSaving(true)
    try {
      if (editing?.id) {
        const payload: UpdateGbPlatformReq = { ...base }
        // 留空表示不修改密码，不提交空串。
        if (values.password) payload.password = values.password
        await GbPlatformController.updateGbPlatform(editing.id, payload)
        message.success('修改成功')
      } else {
        const payload: CreateGbPlatformReq = { ...base }
        if (values.password) payload.password = values.password
        await GbPlatformController.createGbPlatform(payload)
        message.success('新增成功')
      }
      setModalOpen(false)
      void fetchData()
    } catch (err) {
      message.error(apiErrorMessage(err, '保存上级平台失败'))
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (row: GbPlatformItem) => {
    if (!row.id) return
    try {
      await GbPlatformController.deleteGbPlatform(row.id)
      message.success('删除成功')
      void fetchData()
    } catch (err) {
      message.error(apiErrorMessage(err, '删除上级平台失败'))
    }
  }

  /** 打开分组 / 区域可见范围设置（穿梭框）。 */
  const openScope = async (row: GbPlatformItem, kind: ScopeKind) => {
    if (!row.id) return
    setScopeTarget(row)
    setScopeKind(kind)
    setScopeItems([])
    setScopeKeys([])
    setScopeLoading(true)
    try {
      const detailRes = await GbPlatformController.getGbPlatform(row.id)
      const ids = kind === 'groups'
        ? detailRes.data?.group_ids ?? []
        : detailRes.data?.region_ids ?? []
      setScopeKeys(ids.map(String))

      if (kind === 'groups') {
        const res = await GbGroupController.listGbGroups()
        setScopeItems((res.data ?? []).flatMap((item) => (
          item.id === undefined
            ? []
            : [{ key: String(item.id), title: item.name || String(item.id) }]
        )))
      } else {
        const res = await BasicController.pageRegions(1, 200)
        setScopeItems((res.data?.items ?? []).flatMap((item) => (
          item.id === undefined
            ? []
            : [{ key: String(item.id), title: item.name || String(item.id) }]
        )))
      }
    } catch (err) {
      message.error(apiErrorMessage(err, '加载可见范围失败'))
      setScopeKind(null)
    } finally {
      setScopeLoading(false)
    }
  }

  const closeScope = () => {
    setScopeKind(null)
    setScopeTarget(null)
    setScopeItems([])
    setScopeKeys([])
  }

  const saveScope = async () => {
    if (!scopeTarget?.id || !scopeKind) return
    const ids = scopeKeys
      .map((key) => Number(key))
      .filter((id) => Number.isFinite(id) && id > 0)
    setScopeSaving(true)
    try {
      if (scopeKind === 'groups') {
        await GbPlatformController.setGbPlatformGroups(scopeTarget.id, { group_ids: ids })
        message.success('分组可见范围已保存')
      } else {
        await GbPlatformController.setGbPlatformRegions(scopeTarget.id, { region_ids: ids })
        message.success('区域可见范围已保存')
      }
      closeScope()
    } catch (err) {
      message.error(apiErrorMessage(err, '保存可见范围失败'))
    } finally {
      setScopeSaving(false)
    }
  }

  const columns: ColumnsType<GbPlatformItem> = [
    { title: '平台名称', dataIndex: 'name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '平台编码', dataIndex: 'server_gb_id', width: 190, ellipsis: true, render: (v?: string) => v || '-' },
    { title: 'SIP 域', dataIndex: 'server_gb_domain', width: 150, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '地址',
      key: 'address',
      width: 190,
      ellipsis: true,
      render: (_: unknown, row) => (
        row.server_ip ? `${row.server_ip}${row.server_port ? `:${row.server_port}` : ''}` : '-'
      ),
    },
    { title: '用户名', dataIndex: 'username', width: 120, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '传输协议',
      dataIndex: 'transport',
      width: 100,
      render: (v?: string) => <Tag color={v === 'TCP' ? 'geekblue' : 'default'}>{v || 'UDP'}</Tag>,
    },
    {
      title: '启用状态',
      dataIndex: 'enable',
      width: 100,
      render: (v?: boolean) => <Tag color={v === false ? 'default' : 'green'}>{v === false ? '停用' : '启用'}</Tag>,
    },
    {
      title: '在线状态',
      dataIndex: 'status',
      width: 100,
      render: (v?: string) => <Tag color={v === 'ON' ? 'green' : 'default'}>{v === 'ON' ? '在线' : '离线'}</Tag>,
    },
    { title: '创建时间', dataIndex: 'created_dt', width: 170, render: (v?: string) => formatDT(v) },
    {
      title: '操作',
      key: 'action',
      width: 330,
      fixed: 'right',
      render: (_: unknown, row) => (
        <ActionButtons maxWidth={330}>
          {canSetGroups && (
            <Button type="link" size="small" icon={<ApartmentOutlined />} onClick={() => void openScope(row, 'groups')}>
              推送分组
            </Button>
          )}
          {canSetRegions && (
            <Button type="link" size="small" icon={<ShareAltOutlined />} onClick={() => void openScope(row, 'regions')}>
              推送区域
            </Button>
          )}
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
              编辑
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除该上级平台?" onConfirm={() => void handleDelete(row)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  const scopeTitle = scopeKind === 'groups' ? '推送分组（可见范围）' : '推送区域（可见范围）'
  const scopeTitles: [string, string] = scopeKind === 'groups'
    ? ['可选分组', '已选分组']
    : ['可选区域', '已选区域']

  return (
    <PageContainer
      title="上级平台"
      subTitle="国标级联上级平台配置与目录可见范围映射"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void fetchData()} loading={loading}>
            刷新
          </Button>
          {canCreate && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新增上级平台
            </Button>
          )}
        </Space>
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="本期级联只做配置与映射"
        description="保存平台配置、通道映射及分组/区域可见范围；不向上级平台发起 REGISTER、不推送目录，也不处理上级 INVITE。"
      />

      <SearchBar
        fields={[
          { name: 'keyword', label: '平台', placeholder: '名称 / 国标编码' },
          { name: 'status', label: '在线状态', type: 'select', options: STATUS_OPTIONS },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<GbPlatformItem>
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

      {/* 新增 / 编辑上级平台 */}
      <Modal
        className="e2e-gb-platform-modal"
        title={editing ? '编辑上级平台' : '新增上级平台'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => void formRef.current?.submit()}
        confirmLoading={saving}
        destroyOnHidden
        width={720}
      >
        <PlatformFormContent ref={formRef} editing={editing} onSubmit={submit} />
      </Modal>

      {/* 分组 / 区域可见范围（穿梭框） */}
      <Modal
        className={scopeKind === 'groups' ? 'e2e-gb-platform-groups-modal' : 'e2e-gb-platform-regions-modal'}
        title={`${scopeTitle}${scopeTarget?.name ? ` - ${scopeTarget.name}` : ''}`}
        open={scopeKind !== null}
        onCancel={closeScope}
        onOk={() => void saveScope()}
        confirmLoading={scopeSaving}
        okButtonProps={{ disabled: scopeLoading }}
        destroyOnHidden
        width={820}
      >
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="保存后将覆盖该平台当前的可见范围"
          description="未选择任何项表示该平台不可见任何分组/区域。"
        />
        <Transfer
          dataSource={scopeItems}
          titles={scopeTitles}
          targetKeys={scopeKeys}
          onChange={(keys) => setScopeKeys(keys.map(String))}
          render={(item) => item.title ?? item.key}
          listStyle={{ width: 340, height: 380 }}
          showSearch
          filterOption={(inputValue, item) => (
            (item.title ?? '').toLowerCase().includes(inputValue.toLowerCase())
          )}
          disabled={scopeLoading}
        />
      </Modal>
    </PageContainer>
  )
}

export default PlatformList
