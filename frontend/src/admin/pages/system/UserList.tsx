import { useCallback, useEffect, useState } from 'react'
import type { Key } from 'react'
import {
  Button, Col, Form, Input, Modal, Popconfirm, Row, Select, Space, Switch, Table, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { DeleteOutlined, EditOutlined, KeyOutlined, PlusOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import DeptSelect from '../../components/DeptSelect'
import { UserController } from '../../api/generated/UserController'
import { RoleController } from '../../api/generated/RoleController'
import { PostController } from '../../api/generated/PostController'
import { useAuthStore } from '../../store/authStore'
import type { User } from '../../api/generated/definitions/User'
import type { OptionItem } from '../../api/generated/definitions/OptionItem'
import type { CreateUserReq } from '../../api/generated/definitions/CreateUserReq'
import type { UpdateUserReq } from '../../api/generated/definitions/UpdateUserReq'

/** 用户表单值 */
interface UserFormValues {
  user_name: string
  name: string
  /** 与后端 request.CreateUserReq.password 对齐（仅新增时提交，编辑时不使用） */
  password: string
  email?: string
  phone?: string
  dept_id?: number
  post_id?: number
  role_ids?: number[]
  remark?: string
}

/** 重置密码表单值 */
interface ResetPasswordFormValues {
  new_password: string
  confirm_password: string
}

/**
 * 用户管理：分页列表 + 增删改 + 启停 + 重置密码。
 * 登录名（user_name）创建后不可修改，由后端唯一索引约束。
 */
export default function UserList() {
  const [data, setData] = useState<User[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [search, setSearch] = useState<Record<string, unknown>>({})
  const [deptId, setDeptId] = useState<number | undefined>(undefined)
  const [activeFilter, setActiveFilter] = useState<boolean | undefined>(undefined)
  const [selectedKeys, setSelectedKeys] = useState<Key[]>([])
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<User | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<UserFormValues>()

  const [resetOpen, setResetOpen] = useState(false)
  const [resetTarget, setResetTarget] = useState<User | null>(null)
  const [resetting, setResetting] = useState(false)
  const [resetForm] = Form.useForm<ResetPasswordFormValues>()

  const [roleOptions, setRoleOptions] = useState<OptionItem[]>([])
  const [postOptions, setPostOptions] = useState<OptionItem[]>([])

  const permCodes = useAuthStore((s) => s.permCodes)
  // permCodes 为空说明菜单权限尚未加载（如开发态直接进入页面），此时不隐藏按钮，避免整页操作被锁死
  const can = (code: string) => permCodes.length === 0 || permCodes.includes(code)
  const canCreate = can('system:user:create')
  const canUpdate = can('system:user:update')
  const canDelete = can('system:user:delete')
  const canReset = can('system:user:reset')

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const keyword = String(search.keyword ?? '')
      const res = await UserController.pageUsers(page, pageSize, keyword || undefined, deptId, activeFilter)
      if (res.code === 200) {
        setData(res.data?.items ?? [])
        setTotal(res.data?.total ?? 0)
      } else {
        message.error(res.message || '获取用户列表失败')
      }
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, search, deptId, activeFilter])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  // 岗位与角色下拉：一次加载，供表单多选使用
  useEffect(() => {
    PostController.options().then((res) => {
      if (res.code === 200) setPostOptions(res.data ?? [])
    })
    RoleController.options().then((res) => {
      if (res.code === 200) setRoleOptions(res.data ?? [])
    })
  }, [])

  const handleSearch = (values: Record<string, unknown>) => {
    setSearch(values)
    setPage(1)
  }

  const handleReset = () => {
    setSearch({})
    setDeptId(undefined)
    setActiveFilter(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    setModalOpen(true)
  }

  const openEdit = (record: User) => {
    setEditing(record)
    // 只提取表单字段，避免把 dept / post / roles 关联对象整体带入
    form.setFieldsValue({
      user_name: record.user_name,
      name: record.name,
      email: record.email,
      phone: record.phone,
      dept_id: record.dept_id ?? undefined,
      post_id: record.post_id ?? undefined,
      role_ids: (record.roles ?? []).map((role) => role.id).filter((id): id is number => typeof id === 'number'),
      remark: record.remark,
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    setSaving(true)
    try {
      if (editing) {
        // 编辑：登录名不可修改，也不提交密码字段
        const payload: UpdateUserReq = {
          name: values.name,
          email: values.email,
          phone: values.phone,
          dept_id: values.dept_id,
          post_id: values.post_id,
          role_ids: values.role_ids,
          remark: values.remark,
        }
        const res = await UserController.updateUser(String(editing.id), payload)
        if (res.code === 200) {
          message.success('更新成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '更新失败')
        }
      } else {
        const payload: CreateUserReq = {
          user_name: values.user_name,
          password: values.password,
          name: values.name,
          email: values.email,
          phone: values.phone,
          dept_id: values.dept_id,
          post_id: values.post_id,
          role_ids: values.role_ids,
          remark: values.remark,
        }
        const res = await UserController.createUser(payload)
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

  /** 启用 / 禁用用户 */
  const handleToggleActive = async (record: User, checked: boolean) => {
    try {
      const res = checked
        ? await UserController.enableUser(String(record.id))
        : await UserController.disableUser(String(record.id))
      if (res.code === 200) {
        message.success(checked ? '已启用' : '已禁用')
      } else {
        message.error(res.message || '操作失败')
      }
      fetchData()
    } catch {
      message.error('操作失败')
    }
  }

  const handleDelete = async (record: User) => {
    try {
      const res = await UserController.deleteUser(String(record.id))
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

  const handleBatchDelete = async () => {
    if (selectedKeys.length === 0) {
      message.warning('请先选择要删除的用户')
      return
    }
    try {
      await Promise.all(selectedKeys.map((key) => UserController.deleteUser(String(key))))
      message.success(`已删除 ${selectedKeys.length} 条记录`)
      setSelectedKeys([])
      fetchData()
    } catch {
      message.error('批量删除失败')
    }
  }

  const openResetPassword = (record: User) => {
    setResetTarget(record)
    resetForm.resetFields()
    setResetOpen(true)
  }

  const handleResetPassword = async () => {
    if (!resetTarget) return
    const values = await resetForm.validateFields()
    setResetting(true)
    try {
      const res = await UserController.resetUserPassword(String(resetTarget.id), { new_password: values.new_password })
      if (res.code === 200) {
        message.success('密码重置成功')
        setResetOpen(false)
      } else {
        message.error(res.message || '重置失败')
      }
    } catch {
      message.error('重置失败，请检查网络后重试')
    } finally {
      setResetting(false)
    }
  }

  const columns: ColumnsType<User> = [
    { title: '用户名', dataIndex: 'user_name', key: 'user_name', width: 120 },
    { title: '姓名', dataIndex: 'name', key: 'name', width: 100 },
    {
      title: '部门', key: 'dept', width: 120,
      render: (_, record) => record.dept?.name ?? '-',
    },
    {
      title: '岗位', key: 'post', width: 120,
      render: (_, record) => (record.post?.name ? <Tag color="blue">{record.post.name}</Tag> : <Tag>未分配</Tag>),
    },
    {
      title: '角色', key: 'roles', width: 200,
      render: (_, record) => {
        const roles = record.roles ?? []
        if (roles.length === 0) return <Tag>未分配</Tag>
        return roles.map((role) => <Tag key={role.id} color="green">{role.name}</Tag>)
      },
    },
    { title: '邮箱', dataIndex: 'email', key: 'email', width: 180, ellipsis: true },
    { title: '手机号', dataIndex: 'phone', key: 'phone', width: 130 },
    {
      title: '状态', key: 'active', width: 90,
      render: (_, record) => (
        <Switch
          checked={record.active !== false}
          checkedChildren="启用"
          unCheckedChildren="禁用"
          disabled={!canUpdate}
          onChange={(checked) => handleToggleActive(record, checked)}
        />
      ),
    },
    {
      title: '最后登录', dataIndex: 'last_login', key: 'last_login', width: 170,
      render: (value: string) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
    {
      title: '操作', key: 'action', width: 220, fixed: 'right' as const,
      render: (_, record) => (
        <ActionButtons>
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button>
          )}
          {canReset && (
            <Button type="link" size="small" icon={<KeyOutlined />} onClick={() => openResetPassword(record)}>重置密码</Button>
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
      title="用户管理"
      subTitle="维护平台登录账号、部门岗位归属与角色授权"
      extra={(
        <Space>
          {canDelete && (
            <Popconfirm title={`确定删除选中的 ${selectedKeys.length} 条记录?`} onConfirm={handleBatchDelete}>
              <Button danger disabled={selectedKeys.length === 0}>批量删除</Button>
            </Popconfirm>
          )}
          {canCreate && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建用户</Button>
          )}
        </Space>
      )}
    >
      {/* 部门与状态为选择型筛选项，SearchBar 仅承载文本检索 */}
      <Row gutter={16} style={{ marginBottom: 12 }}>
        <Col>
          <Space>
            <span>部门：</span>
            <DeptSelect
              value={deptId}
              onChange={(value) => {
                setDeptId(typeof value === 'number' ? value : undefined)
                setPage(1)
              }}
              placeholder="全部部门"
            />
          </Space>
        </Col>
        <Col>
          <Space>
            <span>状态：</span>
            <Select
              style={{ width: 140 }}
              allowClear
              placeholder="全部状态"
              value={activeFilter}
              onChange={(value: boolean | undefined) => {
                setActiveFilter(value)
                setPage(1)
              }}
              options={[{ label: '启用', value: true }, { label: '禁用', value: false }]}
            />
          </Space>
        </Col>
      </Row>

      <SearchBar
        fields={[{ name: 'keyword', label: '用户名/姓名', placeholder: '请输入用户名或姓名' }]}
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
        rowSelection={{ selectedRowKeys: selectedKeys, onChange: (keys) => setSelectedKeys(keys) }}
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
        className="e2e-user-modal"
        title={editing ? '编辑用户' : '新建用户'}
        open={modalOpen}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
        confirmLoading={saving}
        width={720}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="user_name"
                label="用户名"
                rules={[{ required: true, message: '请输入用户名' }]}
              >
                <Input disabled={Boolean(editing)} placeholder="登录名，创建后不可修改" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="name" label="姓名" rules={[{ required: true, message: '请输入姓名' }]}>
                <Input placeholder="请输入姓名" />
              </Form.Item>
            </Col>
            {!editing && (
              <Col span={12}>
                <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
                  <Input.Password placeholder="请输入初始密码" />
                </Form.Item>
              </Col>
            )}
            <Col span={12}>
              <Form.Item name="email" label="邮箱" rules={[{ type: 'email', message: '邮箱格式不正确' }]}>
                <Input placeholder="请输入邮箱" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="phone" label="手机号">
                <Input placeholder="请输入手机号" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="dept_id" label="部门">
                <DeptSelect placeholder="请选择部门" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="post_id" label="岗位">
                <Select
                  allowClear
                  placeholder="请选择岗位"
                  options={postOptions.map((item) => ({ label: item.name, value: item.id }))}
                />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="role_ids" label="拥有角色">
                <Select
                  mode="multiple"
                  allowClear
                  placeholder="请选择角色"
                  options={roleOptions.map((item) => ({ label: item.name, value: item.id }))}
                />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="remark" label="备注">
                <Input.TextArea rows={3} placeholder="请输入备注" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>

      <Modal
        className="e2e-user-resetpwd-modal"
        title={`重置密码 - ${resetTarget?.name ?? ''}`}
        open={resetOpen}
        onOk={handleResetPassword}
        onCancel={() => setResetOpen(false)}
        confirmLoading={resetting}
        width={520}
      >
        <Form form={resetForm} layout="vertical">
          <Form.Item
            name="new_password"
            label="新密码"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 6, message: '密码长度不少于 6 位' },
            ]}
          >
            <Input.Password placeholder="请输入新密码" />
          </Form.Item>
          <Form.Item
            name="confirm_password"
            label="确认新密码"
            dependencies={['new_password']}
            rules={[
              { required: true, message: '请再次输入新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('new_password') === value) return Promise.resolve()
                  return Promise.reject(new Error('两次输入的密码不一致'))
                },
              }),
            ]}
          >
            <Input.Password placeholder="请再次输入新密码" />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  )
}
