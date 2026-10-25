import { useCallback, useEffect, useState } from 'react'
import {
  Button, Col, Form, Input, InputNumber, Modal, Popconfirm, Radio, Row, Select, Space, Switch,
  Table, Tag, Transfer, Tree, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { TreeDataNode } from 'antd'
import {
  ApartmentOutlined, DeleteOutlined, EditOutlined, IdcardOutlined, PlusOutlined,
  SafetyCertificateOutlined, SettingOutlined, TeamOutlined,
} from '@ant-design/icons'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import UserSelect from '../../components/UserSelect'
import DeptSelect from '../../components/DeptSelect'
import { RoleController } from '../../api/generated/RoleController'
import { MenuController } from '../../api/generated/MenuController'
import { DeptController } from '../../api/generated/DeptController'
import { PostController } from '../../api/generated/PostController'
import { useAuthStore } from '../../store/authStore'
import type { Role } from '../../api/generated/definitions/Role'
import type { Menu } from '../../api/generated/definitions/Menu'
import type { Dept } from '../../api/generated/definitions/Dept'
import type { OptionItem } from '../../api/generated/definitions/OptionItem'

/** 角色表单值 */
interface RoleFormValues {
  name: string
  code: string
  sequence: number
  data_permission: string
  description?: string
}

/** 数据权限取值说明，与后端 model.DataScopeOption 一一对应 */
const DATA_PERM_LABELS: Record<string, string> = {
  all: '全部数据',
  cross_dept: '跨部门',
  dept: '本部门',
  personal: '仅个人',
}

const DATA_PERM_OPTIONS = Object.entries(DATA_PERM_LABELS).map(([value, label]) => ({ value, label }))

/**
 * 角色管理：分页列表 + 增删改 + 四类授权（菜单 / 部门 / 岗位 / 用户）+ 数据权限设置。
 * 权限字符 code 是 Casbin 的 sub，必须唯一且稳定，授权变更后由后端重建策略。
 */
export default function RoleList() {
  const [data, setData] = useState<Role[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [search, setSearch] = useState<Record<string, unknown>>({})
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Role | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<RoleFormValues>()

  // 授权弹窗公共数据源
  const [menuTree, setMenuTree] = useState<Menu[]>([])
  const [deptTree, setDeptTree] = useState<Dept[]>([])
  const [postOptions, setPostOptions] = useState<OptionItem[]>([])

  // 当前授权目标角色
  const [authTarget, setAuthTarget] = useState<Role | null>(null)
  const [assigning, setAssigning] = useState(false)

  // 菜单权限
  const [menuAuthOpen, setMenuAuthOpen] = useState(false)
  const [checkedMenuKeys, setCheckedMenuKeys] = useState<number[]>([])
  const [halfCheckedMenuKeys, setHalfCheckedMenuKeys] = useState<number[]>([])

  // 部门权限
  const [deptAuthOpen, setDeptAuthOpen] = useState(false)
  const [checkedDeptKeys, setCheckedDeptKeys] = useState<number[]>([])
  const [halfCheckedDeptKeys, setHalfCheckedDeptKeys] = useState<number[]>([])

  // 岗位权限
  const [postAuthOpen, setPostAuthOpen] = useState(false)
  const [postTargetKeys, setPostTargetKeys] = useState<string[]>([])

  // 用户权限
  const [userAuthOpen, setUserAuthOpen] = useState(false)
  const [userIds, setUserIds] = useState<number[]>([])

  // 数据权限
  const [dataPermOpen, setDataPermOpen] = useState(false)
  const [dataPerm, setDataPerm] = useState<string>('all')
  const [crossDeptIds, setCrossDeptIds] = useState<number[]>([])
  const [savingDataPerm, setSavingDataPerm] = useState(false)

  const permCodes = useAuthStore((s) => s.permCodes)
  // permCodes 为空说明菜单权限尚未加载，此时不隐藏按钮，避免整页操作被锁死
  const can = (code: string) => permCodes.length === 0 || permCodes.includes(code)
  const canCreate = can('system:role:create')
  const canUpdate = can('system:role:update')
  const canDelete = can('system:role:delete')
  const canAssign = can('system:role:assign')
  const canDataPerm = can('system:role:data')

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const keyword = String(search.keyword ?? '')
      const res = await RoleController.pageRoles(page, pageSize, keyword || undefined, undefined)
      if (res.code === 200) {
        setData(res.data?.items ?? [])
        setTotal(res.data?.total ?? 0)
      } else {
        message.error(res.message || '获取角色列表失败')
      }
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, search])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  // 授权弹窗数据源一次加载。菜单树取 pc 平台（管理后台授权范围）
  useEffect(() => {
    MenuController.treeMenus('pc').then((res) => {
      if (res.code === 200) setMenuTree(res.data ?? [])
    })
    DeptController.treeDepts().then((res) => {
      if (res.code === 200) setDeptTree(res.data ?? [])
    })
    PostController.options().then((res) => {
      if (res.code === 200) setPostOptions(res.data ?? [])
    })
  }, [])

  const handleSearch = (values: Record<string, unknown>) => {
    setSearch(values)
    setPage(1)
  }

  const handleReset = () => {
    setSearch({})
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ sequence: 10, data_permission: 'all' })
    setModalOpen(true)
  }

  const openEdit = (record: Role) => {
    setEditing(record)
    form.setFieldsValue({
      name: record.name,
      code: record.code,
      sequence: record.sequence ?? 10,
      data_permission: record.data_permission || 'all',
      description: record.description,
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    setSaving(true)
    try {
      if (editing) {
        const res = await RoleController.updateRole(String(editing.id), {
          name: values.name,
          code: values.code,
          sequence: values.sequence,
          data_permission: values.data_permission,
          // 后端无条件覆盖 Description/CrossDeptIDs，未带入会导致既有值被清空
          cross_dept_ids: editing.cross_dept_ids ?? '',
          description: values.description,
          active: editing.active !== false,
        })
        if (res.code === 200) {
          message.success('更新成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '更新失败')
        }
      } else {
        const res = await RoleController.createRole({
          name: values.name,
          code: values.code,
          sequence: values.sequence,
          description: values.description,
          data_permission: values.data_permission || 'all',
          active: true,
        })
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

  /** 行内启停：UpdateRoleReq 的字符串字段为空表示不修改，但 Description/CrossDeptIDs 会被覆盖，需带当前值 */
  const handleToggleActive = async (record: Role, checked: boolean) => {
    try {
      const res = await RoleController.updateRole(String(record.id), {
        name: record.name,
        code: record.code,
        sequence: record.sequence,
        description: record.description,
        data_permission: record.data_permission || 'all',
        cross_dept_ids: record.cross_dept_ids ?? '',
        active: checked,
      })
      if (res.code === 200) {
        message.success(checked ? '已启用' : '已停用')
      } else {
        message.error(res.message || '操作失败')
      }
      fetchData()
    } catch {
      message.error('操作失败')
    }
  }

  const handleDelete = async (record: Role) => {
    try {
      const res = await RoleController.deleteRole(String(record.id))
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

  /** 菜单树 → antd Tree 数据 */
  const toMenuTreeData = (nodes: Menu[]): TreeDataNode[] => nodes.map((node) => ({
    title: node.name,
    key: node.id ?? 0,
    children: node.children && node.children.length > 0 ? toMenuTreeData(node.children) : undefined,
  }))

  /** 部门树 → antd Tree 数据 */
  const toDeptTreeData = (nodes: Dept[]): TreeDataNode[] => nodes.map((node) => ({
    title: node.name,
    key: node.id ?? 0,
    children: node.children && node.children.length > 0 ? toDeptTreeData(node.children) : undefined,
  }))

  /** 打开菜单权限弹窗并回显已授权菜单 */
  const openMenuAuth = async (record: Role) => {
    setAuthTarget(record)
    setCheckedMenuKeys([])
    setHalfCheckedMenuKeys([])
    setMenuAuthOpen(true)
    const res = await RoleController.getRoleMenus(String(record.id))
    if (res.code === 200) setCheckedMenuKeys(res.data ?? [])
  }

  /** 打开部门权限弹窗并回显已关联部门 */
  const openDeptAuth = async (record: Role) => {
    setAuthTarget(record)
    setCheckedDeptKeys([])
    setHalfCheckedDeptKeys([])
    setDeptAuthOpen(true)
    const res = await RoleController.getRoleDepts(String(record.id))
    if (res.code === 200) setCheckedDeptKeys(res.data ?? [])
  }

  /** 打开岗位权限弹窗并回显已关联岗位 */
  const openPostAuth = async (record: Role) => {
    setAuthTarget(record)
    setPostTargetKeys([])
    setPostAuthOpen(true)
    const res = await RoleController.getRolePosts(String(record.id))
    if (res.code === 200) setPostTargetKeys((res.data ?? []).map(String))
  }

  /** 打开用户权限弹窗并回显已关联用户 */
  const openUserAuth = async (record: Role) => {
    setAuthTarget(record)
    setUserIds([])
    setUserAuthOpen(true)
    const res = await RoleController.getRoleUsers(String(record.id))
    if (res.code === 200) setUserIds(res.data ?? [])
  }

  /** 打开数据权限弹窗并回显当前设置 */
  const openDataPerm = (record: Role) => {
    setAuthTarget(record)
    setDataPerm(record.data_permission || 'all')
    // cross_dept_ids 为逗号分隔字符串，转成数组供多选回显
    const ids = String(record.cross_dept_ids ?? '')
      .split(',')
      .map((item) => Number(item.trim()))
      .filter((item) => Number.isFinite(item) && item > 0)
    setCrossDeptIds(ids)
    setDataPermOpen(true)
  }

  const handleAssignMenus = async () => {
    if (!authTarget) return
    setAssigning(true)
    try {
      // 半选的父节点也要提交，否则父级菜单不会出现在授权集合中
      const ids = Array.from(new Set([...checkedMenuKeys, ...halfCheckedMenuKeys]))
      const res = await RoleController.assignRoleMenus(String(authTarget.id), { ids })
      if (res.code === 200) {
        message.success('菜单权限已保存')
        setMenuAuthOpen(false)
      } else {
        message.error(res.message || '保存失败')
      }
    } catch {
      message.error('保存失败')
    } finally {
      setAssigning(false)
    }
  }

  const handleAssignDepts = async () => {
    if (!authTarget) return
    setAssigning(true)
    try {
      const ids = Array.from(new Set([...checkedDeptKeys, ...halfCheckedDeptKeys]))
      const res = await RoleController.assignRoleDepts(String(authTarget.id), { ids })
      if (res.code === 200) {
        message.success('部门权限已保存')
        setDeptAuthOpen(false)
      } else {
        message.error(res.message || '保存失败')
      }
    } catch {
      message.error('保存失败')
    } finally {
      setAssigning(false)
    }
  }

  const handleAssignPosts = async () => {
    if (!authTarget) return
    setAssigning(true)
    try {
      const res = await RoleController.assignRolePosts(String(authTarget.id), { ids: postTargetKeys.map(Number) })
      if (res.code === 200) {
        message.success('岗位权限已保存')
        setPostAuthOpen(false)
      } else {
        message.error(res.message || '保存失败')
      }
    } catch {
      message.error('保存失败')
    } finally {
      setAssigning(false)
    }
  }

  const handleAssignUsers = async () => {
    if (!authTarget) return
    setAssigning(true)
    try {
      const res = await RoleController.assignRoleUsers(String(authTarget.id), { ids: userIds })
      if (res.code === 200) {
        message.success('用户权限已保存')
        setUserAuthOpen(false)
      } else {
        message.error(res.message || '保存失败')
      }
    } catch {
      message.error('保存失败')
    } finally {
      setAssigning(false)
    }
  }

  /** 保存数据权限：连同角色基础字段一起提交，避免部分更新丢失名称等字段 */
  const handleSaveDataPerm = async () => {
    if (!authTarget) return
    setSavingDataPerm(true)
    try {
      const res = await RoleController.updateRole(String(authTarget.id), {
        name: authTarget.name,
        code: authTarget.code,
        sequence: authTarget.sequence,
        description: authTarget.description,
        active: authTarget.active,
        data_permission: dataPerm,
        cross_dept_ids: dataPerm === 'cross_dept' ? crossDeptIds.join(',') : '',
      })
      if (res.code === 200) {
        message.success('数据权限已保存')
        setDataPermOpen(false)
        fetchData()
      } else {
        message.error(res.message || '保存失败')
      }
    } catch {
      message.error('保存失败')
    } finally {
      setSavingDataPerm(false)
    }
  }

  const columns: ColumnsType<Role> = [
    { title: '角色编号', dataIndex: 'id', key: 'id', width: 90 },
    { title: '角色名称', dataIndex: 'name', key: 'name', width: 140 },
    {
      title: '权限字符', dataIndex: 'code', key: 'code', width: 150,
      render: (value: string) => <Tag color="blue">{value}</Tag>,
    },
    { title: '显示顺序', dataIndex: 'sequence', key: 'sequence', width: 90 },
    {
      title: '数据权限', dataIndex: 'data_permission', key: 'data_permission', width: 110,
      render: (value: string) => DATA_PERM_LABELS[value] ?? value ?? '-',
    },
    {
      title: '状态', key: 'active', width: 100,
      render: (_, record) => (
        <Switch
          className="e2e-role-active-switch"
          size="small"
          checked={record.active !== false}
          checkedChildren="启用"
          unCheckedChildren="停用"
          disabled={!canUpdate}
          onChange={(checked) => handleToggleActive(record, checked)}
        />
      ),
    },
    {
      title: '创建时间', dataIndex: 'created_dt', key: 'created_dt', width: 170,
      render: (value: string) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
    {
      title: '操作', key: 'action', width: 380, fixed: 'right' as const,
      render: (_, record) => (
        <ActionButtons>
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button>
          )}
          {canAssign && (
            <Button type="link" size="small" icon={<SafetyCertificateOutlined />} onClick={() => openMenuAuth(record)}>
              菜单权限
            </Button>
          )}
          {canDataPerm && (
            <Button type="link" size="small" icon={<SettingOutlined />} onClick={() => openDataPerm(record)}>
              数据权限
            </Button>
          )}
          {canAssign && (
            <Button type="link" size="small" icon={<ApartmentOutlined />} onClick={() => openDeptAuth(record)}>
              部门权限
            </Button>
          )}
          {canAssign && (
            <Button type="link" size="small" icon={<IdcardOutlined />} onClick={() => openPostAuth(record)}>
              岗位权限
            </Button>
          )}
          {canAssign && (
            <Button type="link" size="small" icon={<TeamOutlined />} onClick={() => openUserAuth(record)}>
              用户权限
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除?" onConfirm={() => handleDelete(record)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>删除</Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  return (
    <PageContainer
      title="角色管理"
      subTitle="维护角色、数据权限与菜单/部门/岗位/用户四类授权"
      extra={canCreate ? (
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建角色</Button>
      ) : undefined}
    >
      <SearchBar
        fields={[{ name: 'keyword', label: '角色名称/权限字符', placeholder: '请输入角色名称或权限字符' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table
        rowKey="id"
        size="small"
        scroll={{ x: 'max-content' }}
        columns={columns}
        dataSource={data}
        loading={loading}
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

      {/* 新增 / 编辑角色 */}
      <Modal
        className="e2e-role-modal"
        title={editing ? '编辑角色' : '新建角色'}
        open={modalOpen}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
        confirmLoading={saving}
        width={640}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="角色名称" rules={[{ required: true, message: '请输入角色名称' }]}>
                <Input placeholder="如：区域管理员" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="code"
                label="权限字符"
                rules={[{ required: true, message: '请输入权限字符' }]}
                extra="Casbin 权限主体，建议使用英文，如 area_admin"
              >
                <Input placeholder="如：area_admin" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="sequence" label="显示顺序" rules={[{ required: true, message: '请输入显示顺序' }]}>
                <InputNumber min={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="data_permission"
                label="数据权限"
                rules={[{ required: true, message: '请选择数据权限' }]}
                extra="跨部门的具体部门可在「数据权限」操作中设置"
              >
                <Select options={DATA_PERM_OPTIONS} />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="description" label="描述">
                <Input.TextArea rows={3} placeholder="请输入角色描述" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>

      {/* 菜单权限：树形勾选 */}
      <Modal
        className="e2e-role-menu-modal"
        title={`菜单权限 - ${authTarget?.name ?? ''}`}
        open={menuAuthOpen}
        onOk={handleAssignMenus}
        onCancel={() => setMenuAuthOpen(false)}
        confirmLoading={assigning}
        width={620}
      >
        <Tree
          checkable
          defaultExpandAll
          treeData={toMenuTreeData(menuTree)}
          checkedKeys={checkedMenuKeys}
          onCheck={(checked, info) => {
            setCheckedMenuKeys(Array.isArray(checked) ? (checked as number[]) : [])
            setHalfCheckedMenuKeys((info.halfCheckedKeys ?? []) as number[])
          }}
        />
      </Modal>

      {/* 部门权限：树形勾选 */}
      <Modal
        className="e2e-role-dept-modal"
        title={`部门权限 - ${authTarget?.name ?? ''}`}
        open={deptAuthOpen}
        onOk={handleAssignDepts}
        onCancel={() => setDeptAuthOpen(false)}
        confirmLoading={assigning}
        width={620}
      >
        <Tree
          checkable
          defaultExpandAll
          treeData={toDeptTreeData(deptTree)}
          checkedKeys={checkedDeptKeys}
          onCheck={(checked, info) => {
            setCheckedDeptKeys(Array.isArray(checked) ? (checked as number[]) : [])
            setHalfCheckedDeptKeys((info.halfCheckedKeys ?? []) as number[])
          }}
        />
      </Modal>

      {/* 岗位权限：穿梭框 */}
      <Modal
        className="e2e-role-post-modal"
        title={`岗位权限 - ${authTarget?.name ?? ''}`}
        open={postAuthOpen}
        onOk={handleAssignPosts}
        onCancel={() => setPostAuthOpen(false)}
        confirmLoading={assigning}
        width={720}
      >
        <Transfer
          dataSource={postOptions.map((item) => ({ key: String(item.id ?? ''), title: item.name ?? '' }))}
          targetKeys={postTargetKeys}
          onChange={(keys) => setPostTargetKeys(keys.map(String))}
          render={(item) => item.title ?? ''}
          titles={['全部岗位', '已关联岗位']}
          showSearch
          listStyle={{ width: 300, height: 360 }}
        />
      </Modal>

      {/* 用户权限：用户多选 */}
      <Modal
        className="e2e-role-user-modal"
        title={`用户权限 - ${authTarget?.name ?? ''}`}
        open={userAuthOpen}
        onOk={handleAssignUsers}
        onCancel={() => setUserAuthOpen(false)}
        confirmLoading={assigning}
        width={620}
      >
        <div style={{ marginBottom: 8 }}>选择归属于该角色的用户：</div>
        <UserSelect
          multiple
          value={userIds}
          onChange={(value) => setUserIds(Array.isArray(value) ? value : [])}
          placeholder="请选择用户"
        />
      </Modal>

      {/* 数据权限 */}
      <Modal
        className="e2e-role-dataperm-modal"
        title={`数据权限 - ${authTarget?.name ?? ''}`}
        open={dataPermOpen}
        onOk={handleSaveDataPerm}
        onCancel={() => setDataPermOpen(false)}
        confirmLoading={savingDataPerm}
        width={620}
      >
        <Radio.Group value={dataPerm} onChange={(e) => setDataPerm(e.target.value)}>
          <Space direction="vertical">
            <Radio value="all"><strong>全部数据</strong> — 可查看平台内全部数据</Radio>
            <Radio value="cross_dept"><strong>跨部门</strong> — 本部门 + 指定部门的数据</Radio>
            <Radio value="dept"><strong>本部门</strong> — 仅本部门数据</Radio>
            <Radio value="personal"><strong>仅个人</strong> — 仅本人创建的数据</Radio>
          </Space>
        </Radio.Group>
        {dataPerm === 'cross_dept' && (
          <div style={{ marginTop: 16 }}>
            <div style={{ marginBottom: 8 }}>选择可访问的部门：</div>
            <DeptSelect
              mode="multiple"
              value={crossDeptIds}
              onChange={(value) => setCrossDeptIds(Array.isArray(value) ? value : [])}
              placeholder="请选择部门"
            />
          </div>
        )}
      </Modal>
    </PageContainer>
  )
}
