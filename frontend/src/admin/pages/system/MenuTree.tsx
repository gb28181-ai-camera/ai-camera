import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button, Col, Form, Input, InputNumber, Modal, Popconfirm, Row, Select, Space, Spin, Switch,
  Table, Tag, TreeSelect, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { DeleteOutlined, EditOutlined, LinkOutlined, PlusOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import DynamicIcon from '../../components/DynamicIcon'
import { MenuController } from '../../api/generated/MenuController'
import { useAuthStore } from '../../store/authStore'
import type { Menu } from '../../api/generated/definitions/Menu'
import type { MenuAPI } from '../../api/generated/definitions/MenuAPI'

/** 菜单类型选项，与后端 model.Menu.Type 取值一致 */
const MENU_TYPE_OPTIONS = [
  { label: '目录', value: 'directory' },
  { label: '菜单', value: 'menu' },
  { label: '按钮', value: 'button' },
]

const PLATFORM_OPTIONS = [
  { label: 'PC 管理后台', value: 'pc' },
  { label: '移动端', value: 'mobile' },
]

/** TreeSelect 节点：value 用于取值，key 为 rc-tree 必需的身份标识 */
interface MenuTreeSelectNode {
  key: number
  value: number
  title: string
  children?: MenuTreeSelectNode[]
}

/** 菜单表单值 */
interface MenuFormValues {
  parent_id?: number
  name: string
  type: string
  platform: string
  path?: string
  component?: string
  icon?: string
  permission_code?: string
  sequence: number
  hidden?: boolean
  active?: boolean
  description?: string
}

/** 接口绑定表单值 */
interface MenuApiFormValues {
  method: string
  path: string
  description?: string
}

/**
 * 表格行：菜单节点 + 层级（level 仅用于缩进，不落库）。
 *
 * 说明：菜单树被平铺成同层级的行后再交给 Table（见 visibleRows），
 * 因此行上不会再携带 `children` 数组 —— 这一点很关键，见 visibleRows 的注释。
 */
type MenuRow = Omit<Menu, 'children'> & { level: number }

/** 展开行内接口列表的单元格样式（原生表格，样式对齐 antd 表格的小尺寸密度） */
const API_CELL_STYLE: React.CSSProperties = {
  padding: '8px 8px',
  borderBottom: '1px solid #f0f0f0',
  fontSize: 12,
}

/** 展开行内接口列表的表头单元格样式 */
const API_HEAD_STYLE: React.CSSProperties = {
  ...API_CELL_STYLE,
  background: '#fafafa',
  fontWeight: 600,
  textAlign: 'left',
}

/**
 * 菜单管理：树形表格 + 增删改 + 接口绑定。
 * 菜单树同时驱动侧边栏与按钮级权限（permission_code）；按钮绑定的接口是 Casbin p 策略的唯一来源。
 */
export default function MenuTree() {
  const [data, setData] = useState<Menu[]>([])
  const [loading, setLoading] = useState(false)
  const [search, setSearch] = useState<Record<string, unknown>>({})
  const [statusFilter, setStatusFilter] = useState<boolean | undefined>(undefined)

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Menu | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<MenuFormValues>()

  // 展开行接口绑定
  const [expandedKeys, setExpandedKeys] = useState<number[]>([])
  const [apiMap, setApiMap] = useState<Record<number, MenuAPI[]>>({})
  const [apiLoadingId, setApiLoadingId] = useState<number | undefined>(undefined)
  const [apiModalOpen, setApiModalOpen] = useState(false)
  const [apiTarget, setApiTarget] = useState<Menu | null>(null)
  const [apiSaving, setApiSaving] = useState(false)
  const [apiForm] = Form.useForm<MenuApiFormValues>()

  const permCodes = useAuthStore((s) => s.permCodes)
  // permCodes 为空说明菜单权限尚未加载，此时不隐藏按钮，避免整页操作被锁死
  const can = (code: string) => permCodes.length === 0 || permCodes.includes(code)
  const canCreate = can('system:menu:create')
  const canUpdate = can('system:menu:update')
  const canDelete = can('system:menu:delete')
  const canBindApi = can('system:menu:api')

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await MenuController.treeMenus()
      if (res.code === 200) {
        setData(res.data ?? [])
      } else {
        message.error(res.message || '获取菜单树失败')
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const handleSearch = (values: Record<string, unknown>) => setSearch(values)
  const handleReset = () => {
    setSearch({})
    setStatusFilter(undefined)
  }

  /** 客户端过滤：命中节点或其子孙命中均保留，保证树结构完整 */
  const keyword = String(search.keyword ?? '').trim().toLowerCase()
  const filteredData = useMemo(() => {
    const walk = (nodes: Menu[]): Menu[] => {
      const result: Menu[] = []
      for (const node of nodes) {
        const children = node.children && node.children.length > 0 ? walk(node.children) : []
        const nameMatch = !keyword || String(node.name ?? '').toLowerCase().includes(keyword)
        const active = node.active !== false
        const statusMatch = statusFilter === undefined || active === statusFilter
        if ((nameMatch && statusMatch) || children.length > 0) {
          result.push({ ...node, children: children.length > 0 ? children : undefined })
        }
      }
      return result
    }
    return walk(data)
  }, [data, keyword, statusFilter])

  /**
   * 树 → 平铺行（dataSource 不再携带 `children`）。
   *
   * 为什么不直接把树交给 Table：rc-table 对「树形数据 + expandedRowRender」会打印两条弃用告警
   * （`` `expandedRowRender` should not use with nested Table `` 与 `` `expandIconColumnIndex` is deprecated ``）
   * —— 前者在 expandedRowRender 与 `children` 数组并存时触发，后者由 antd 对 nest 型表格
   * 回填 expandIconColumnIndex 引起。故这里自行完成树的展开：展开某个节点时把其子节点并入可见行，
   * 层级由 level 决定缩进；展开行本身只承载「已绑定接口」（见 renderApiList）。
   *
   * 过滤命中时（filteredData 已裁剪为「命中节点或其子孙命中」）直接展示全部层级，
   * 避免命中的深层节点因父级未展开而看不见。
   */
  const visibleRows = useMemo<MenuRow[]>(() => {
    const rows: MenuRow[] = []
    const showAllLevels = keyword !== '' || statusFilter !== undefined
    const walk = (nodes: Menu[], level: number) => {
      for (const node of nodes) {
        // 必须彻底摘掉 children，否则 antd/rc-table 仍会把该表判定为树形表格并输出弃用告警
        const { children, ...rest } = node
        rows.push({ ...rest, level })
        if (!children || children.length === 0) continue
        if (showAllLevels || expandedKeys.includes(rest.id ?? 0)) walk(children, level + 1)
      }
    }
    walk(filteredData, 0)
    return rows
  }, [filteredData, expandedKeys, keyword, statusFilter])

  /** 菜单树 → TreeSelect 数据（可排除指定节点及其子孙，避免把父级选成自己） */
  const toTreeSelectData = (nodes: Menu[], excludeId?: number): MenuTreeSelectNode[] =>
    nodes
      .filter((node) => node.id !== excludeId)
      .map((node) => ({
        key: node.id ?? 0,
        value: node.id ?? 0,
        title: String(node.name ?? ''),
        children: node.children && node.children.length > 0 ? toTreeSelectData(node.children) : undefined,
      }))

  const openCreate = (parent?: Menu) => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({
      parent_id: parent?.id,
      type: 'menu',
      platform: 'pc',
      sequence: 10,
      active: true,
      hidden: false,
    })
    setModalOpen(true)
  }

  const openEdit = (record: Menu) => {
    setEditing(record)
    form.setFieldsValue({
      parent_id: record.parent_id ?? undefined,
      name: record.name ?? '',
      type: record.type ?? 'menu',
      platform: record.platform ?? 'pc',
      path: record.path,
      component: record.component,
      icon: record.icon,
      permission_code: record.permission_code,
      sequence: record.sequence ?? 10,
      hidden: record.hidden === true,
      active: record.active !== false,
      description: record.description,
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    setSaving(true)
    try {
      if (editing) {
        const res = await MenuController.updateMenu(String(editing.id), values)
        if (res.code === 200) {
          message.success('更新成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '更新失败')
        }
      } else {
        const res = await MenuController.createMenu(values)
        if (res.code === 200 || res.code === 201) {
          message.success('创建成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '创建失败')
        }
      }
    } catch {
      message.error('保存失败，请检查网络后重试')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (record: Menu) => {
    try {
      const res = await MenuController.deleteMenu(String(record.id))
      if (res.code === 200) {
        message.success('删除成功')
        fetchData()
      } else {
        message.error(res.message || '删除失败')
      }
    } catch {
      message.error('删除失败')
    }
  }

  /** 展开行时按需加载该菜单绑定的接口 */
  const loadApis = async (menuId?: number) => {
    if (!menuId) return
    setApiLoadingId(menuId)
    try {
      const res = await MenuController.listMenuApis(String(menuId))
      if (res.code === 200) {
        setApiMap((prev) => ({ ...prev, [menuId]: res.data ?? [] }))
      }
    } finally {
      setApiLoadingId(undefined)
    }
  }

  const openApiModal = (record: Menu) => {
    setApiTarget(record)
    apiForm.resetFields()
    apiForm.setFieldsValue({ method: 'GET' })
    setApiModalOpen(true)
  }

  const handleCreateApi = async () => {
    if (!apiTarget) return
    const values = await apiForm.validateFields()
    setApiSaving(true)
    try {
      const res = await MenuController.createMenuApi(String(apiTarget.id), values)
      if (res.code === 200 || res.code === 201) {
        message.success('接口绑定成功')
        setApiModalOpen(false)
        loadApis(apiTarget.id)
      } else {
        message.error(res.message || '绑定失败')
      }
    } catch {
      message.error('绑定失败，请检查网络后重试')
    } finally {
      setApiSaving(false)
    }
  }

  const handleDeleteApi = async (record: Menu, apiId?: number) => {
    if (!apiId) return
    try {
      const res = await MenuController.deleteMenuApi(String(apiId))
      if (res.code === 200) {
        message.success('已解绑')
        loadApis(record.id)
      } else {
        message.error(res.message || '解绑失败')
      }
    } catch {
      message.error('解绑失败')
    }
  }

  /**
   * 展开行内容：该菜单已绑定的后端接口。
   *
   * 用原生 `<table>` 而非嵌套 `<Table>`：嵌套 Table 属于 rc-table 明确不推荐的用法，
   * 且会让控制台出现组件用法告警。`.e2e-menu-api-table` 为 E2E 定位锚点（04c-menu.spec.ts）。
   */
  const renderApiList = (menu: Menu) => {
    const list = apiMap[menu.id ?? 0] ?? []
    return (
      <div>
        <div style={{ marginBottom: 8 }}>
          <Space>
            <span>已绑定接口：</span>
            {canBindApi && (
              <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => openApiModal(menu)}>
                新增接口
              </Button>
            )}
          </Space>
        </div>
        <table className="e2e-menu-api-table" style={{ width: '100%', borderCollapse: 'collapse', background: '#fff' }}>
          <thead>
            <tr>
              <th style={{ ...API_HEAD_STYLE, width: 100 }}>请求方式</th>
              <th style={{ ...API_HEAD_STYLE, width: 320 }}>接口路径</th>
              <th style={API_HEAD_STYLE}>说明</th>
              <th style={{ ...API_HEAD_STYLE, width: 100 }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {apiLoadingId === menu.id ? (
              <tr>
                <td style={API_CELL_STYLE} colSpan={4}>
                  <Spin size="small" />
                </td>
              </tr>
            ) : list.length === 0 ? (
              <tr>
                <td style={{ ...API_CELL_STYLE, color: '#999' }} colSpan={4}>
                  暂无绑定接口
                </td>
              </tr>
            ) : (
              list.map((api) => (
                <tr key={api.id}>
                  <td style={API_CELL_STYLE}>
                    <Tag color="blue">{api.method}</Tag>
                  </td>
                  <td style={API_CELL_STYLE}>{api.path}</td>
                  <td style={API_CELL_STYLE}>{api.description || '-'}</td>
                  <td style={API_CELL_STYLE}>
                    {canBindApi && (
                      <Popconfirm title="确定解绑该接口?" onConfirm={() => handleDeleteApi(menu, api.id)}>
                        <Button type="link" size="small" danger icon={<DeleteOutlined />}>解绑</Button>
                      </Popconfirm>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    )
  }

  const columns: ColumnsType<MenuRow> = [
    {
      title: '菜单名称', dataIndex: 'name', key: 'name', width: 220,
      render: (value: string, record) => (
        // 平铺行按 level 缩进，保持原有树形的层级观感
        <Space size={6} style={{ paddingLeft: record.level * 20 }}>
          <DynamicIcon name={record.icon} />
          <span>{value}</span>
        </Space>
      ),
    },
    { title: '路由唯一键', dataIndex: 'path', key: 'path', width: 180 },
    { title: '组件', dataIndex: 'component', key: 'component', width: 180, ellipsis: true },
    { title: '排序', dataIndex: 'sequence', key: 'sequence', width: 70 },
    {
      title: '按钮类型', dataIndex: 'type', key: 'type', width: 100,
      render: (value: string) => {
        const label = MENU_TYPE_OPTIONS.find((item) => item.value === value)?.label ?? value
        const color = value === 'button' ? 'orange' : value === 'directory' ? 'purple' : 'blue'
        return <Tag color={color}>{label}</Tag>
      },
    },
    {
      title: '链接', key: 'link', width: 90,
      render: (_, record) => {
        const path = String(record.path ?? '')
        // 仅外链（http/https）可直接打开，内部路由由框架接管
        if (!/^https?:\/\//i.test(path)) return '-'
        return (
          <a href={path} target="_blank" rel="noreferrer">
            <LinkOutlined /> 打开
          </a>
        )
      },
    },
    {
      title: '权限标识', dataIndex: 'permission_code', key: 'permission_code', width: 200,
      render: (value: string) => (value ? <Tag>{value}</Tag> : '-'),
    },
    {
      title: '状态', key: 'active', width: 80,
      render: (_, record) => (record.active === false ? <Tag>停用</Tag> : <Tag color="green">启用</Tag>),
    },
    {
      title: '操作', key: 'action', width: 300, fixed: 'right' as const,
      render: (_, record) => (
        <ActionButtons>
          {canCreate && (
            <Button type="link" size="small" icon={<PlusOutlined />} onClick={() => openCreate(record)}>新增下级</Button>
          )}
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button>
          )}
          {canBindApi && (
            <Button type="link" size="small" onClick={() => openApiModal(record)}>绑定接口</Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除?存在子菜单时将一并删除。" onConfirm={() => handleDelete(record)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>删除</Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  return (
    <PageContainer
      title="菜单管理"
      subTitle="维护菜单树、按钮级权限标识与后端接口绑定"
      extra={canCreate ? (
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openCreate()}>新建菜单</Button>
      ) : undefined}
    >
      <Row gutter={16} style={{ marginBottom: 12 }}>
        <Col>
          <Space>
            <span>状态：</span>
            <Select
              style={{ width: 140 }}
              allowClear
              placeholder="全部状态"
              value={statusFilter}
              onChange={(value: boolean | undefined) => setStatusFilter(value)}
              options={[{ label: '启用', value: true }, { label: '停用', value: false }]}
            />
          </Space>
        </Col>
      </Row>

      <SearchBar
        fields={[{ name: 'keyword', label: '菜单名称', placeholder: '请输入菜单名称' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<MenuRow>
        rowKey="id"
        size="small"
        scroll={{ x: 'max-content' }}
        columns={columns}
        dataSource={visibleRows}
        loading={loading}
        pagination={false}
        expandable={{
          // 展开行展示该菜单绑定的后端接口（Casbin p 策略来源）
          expandedRowKeys: expandedKeys,
          expandedRowRender: (record) => renderApiList(record),
          onExpand: (expanded, record) => {
            const id = record.id ?? 0
            if (expanded) {
              loadApis(record.id)
              // 受控展开：父级展开后其子菜单并入可见行（visibleRows 依据 expandedKeys 计算）
              setExpandedKeys((prev) => (prev.includes(id) ? prev : [...prev, id]))
            } else {
              setExpandedKeys((prev) => prev.filter((key) => key !== id))
            }
          },
        }}
      />

      {/* 菜单新增 / 编辑 */}
      <Modal
        className="e2e-menu-modal"
        title={editing ? '编辑菜单' : '新建菜单'}
        open={modalOpen}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
        confirmLoading={saving}
        width={760}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="parent_id" label="父菜单">
                <TreeSelect
                  allowClear
                  placeholder="不选则为顶级菜单"
                  treeData={toTreeSelectData(data, editing?.id)}
                  treeDefaultExpandAll
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="type" label="菜单类型" rules={[{ required: true, message: '请选择菜单类型' }]}>
                <Select options={MENU_TYPE_OPTIONS} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="name" label="菜单名称" rules={[{ required: true, message: '请输入菜单名称' }]}>
                <Input placeholder="如：用户管理" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="platform" label="所属平台" rules={[{ required: true, message: '请选择平台' }]}>
                <Select options={PLATFORM_OPTIONS} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="path" label="路由唯一键" extra="目录/菜单需填写，如 /system/user">
                <Input placeholder="如：/system/user" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="component" label="组件" extra="前端组件路径，如 system/UserList">
                <Input placeholder="如：system/UserList" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="icon" label="图标名" extra="如 dashboard / UserOutlined">
                <Input placeholder="如：user" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="permission_code" label="权限标识" extra="按钮级权限，如 system:user:create">
                <Input placeholder="如：system:user:create" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="sequence" label="显示顺序" rules={[{ required: true, message: '请输入显示顺序' }]}>
                <InputNumber min={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="active" label="状态" valuePropName="checked">
                <Switch checkedChildren="启用" unCheckedChildren="停用" />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="hidden" label="是否隐藏" valuePropName="checked">
                <Switch checkedChildren="隐藏" unCheckedChildren="显示" />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="description" label="描述">
                <Input.TextArea rows={2} placeholder="请输入描述" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>

      {/* 菜单接口绑定 */}
      <Modal
        className="e2e-menu-api-modal"
        title={`绑定接口 - ${apiTarget?.name ?? ''}`}
        open={apiModalOpen}
        onOk={handleCreateApi}
        onCancel={() => setApiModalOpen(false)}
        confirmLoading={apiSaving}
        width={560}
      >
        <Form form={apiForm} layout="vertical">
          <Form.Item name="method" label="请求方式" rules={[{ required: true, message: '请选择请求方式' }]}>
            <Select
              options={[
                { label: 'GET', value: 'GET' },
                { label: 'POST', value: 'POST' },
                { label: 'PUT', value: 'PUT' },
                { label: 'DELETE', value: 'DELETE' },
              ]}
            />
          </Form.Item>
          <Form.Item name="path" label="接口路径" rules={[{ required: true, message: '请输入接口路径' }]}>
            <Input placeholder="如：/web/system/users" />
          </Form.Item>
          <Form.Item name="description" label="说明">
            <Input placeholder="如：用户分页列表" />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  )
}
