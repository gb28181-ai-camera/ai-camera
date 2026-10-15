import { useCallback, useEffect, useState } from 'react'
import type { Key } from 'react'
import {
  Button, Col, Form, Input, InputNumber, Modal, Popconfirm, Row, Select, Space, Switch,
  Table, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { PostController } from '../../api/generated/PostController'
import { useAuthStore } from '../../store/authStore'
import type { Post } from '../../api/generated/definitions/Post'

/** 岗位表单值 */
interface PostFormValues {
  name: string
  code: string
  sequence: number
  active?: boolean
  description?: string
}

/**
 * 岗位管理：分页列表 + 增删改 + 启停。
 * 岗位编码 code 是 Casbin 的 g3 主体，用于「岗位 → 角色」的权限继承，必须唯一。
 */
export default function PostList() {
  const [data, setData] = useState<Post[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [search, setSearch] = useState<Record<string, unknown>>({})
  const [activeFilter, setActiveFilter] = useState<boolean | undefined>(undefined)
  const [selectedKeys, setSelectedKeys] = useState<Key[]>([])
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Post | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<PostFormValues>()

  const permCodes = useAuthStore((s) => s.permCodes)
  // permCodes 为空说明菜单权限尚未加载，此时不隐藏按钮，避免整页操作被锁死
  const can = (code: string) => permCodes.length === 0 || permCodes.includes(code)
  const canCreate = can('system:post:create')
  const canUpdate = can('system:post:update')
  const canDelete = can('system:post:delete')

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const keyword = String(search.keyword ?? '')
      const res = await PostController.pagePosts(page, pageSize, keyword || undefined, activeFilter)
      if (res.code === 200) {
        setData(res.data?.items ?? [])
        setTotal(res.data?.total ?? 0)
      } else {
        message.error(res.message || '获取岗位列表失败')
      }
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, search, activeFilter])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const handleSearch = (values: Record<string, unknown>) => {
    setSearch(values)
    setPage(1)
  }

  const handleReset = () => {
    setSearch({})
    setActiveFilter(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ sequence: 10, active: true })
    setModalOpen(true)
  }

  const openEdit = (record: Post) => {
    setEditing(record)
    form.setFieldsValue({
      name: record.name ?? '',
      code: record.code ?? '',
      sequence: record.sequence ?? 10,
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
        const res = await PostController.updatePost(String(editing.id), values)
        if (res.code === 200) {
          message.success('更新成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '更新失败')
        }
      } else {
        const res = await PostController.createPost(values)
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

  /** 岗位无独立启停端点，通过整体更新切换状态，需带上原有字段避免丢失 */
  const handleToggleActive = async (record: Post, checked: boolean) => {
    try {
      const res = await PostController.updatePost(String(record.id), {
        name: record.name,
        code: record.code,
        sequence: record.sequence,
        description: record.description,
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

  const handleDelete = async (record: Post) => {
    try {
      const res = await PostController.deletePost(String(record.id))
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
      message.warning('请先选择要删除的岗位')
      return
    }
    try {
      await Promise.all(selectedKeys.map((key) => PostController.deletePost(String(key))))
      message.success(`已删除 ${selectedKeys.length} 条记录`)
      setSelectedKeys([])
      fetchData()
    } catch {
      message.error('批量删除失败')
    }
  }

  const columns: ColumnsType<Post> = [
    { title: '岗位编号', dataIndex: 'id', key: 'id', width: 90 },
    {
      title: '岗位编码', dataIndex: 'code', key: 'code', width: 160,
      render: (value: string) => <Tag color="blue">{value}</Tag>,
    },
    { title: '岗位名称', dataIndex: 'name', key: 'name', width: 160 },
    { title: '排序', dataIndex: 'sequence', key: 'sequence', width: 80 },
    {
      title: '状态', key: 'active', width: 100,
      render: (_, record) => (
        <Switch
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
      title: '操作', key: 'action', width: 160, fixed: 'right' as const,
      render: (_, record) => (
        <ActionButtons>
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button>
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
      title="岗位管理"
      subTitle="维护岗位；岗位编码用于「岗位 → 角色」的权限继承"
      extra={(
        <Space>
          {canDelete && (
            <Popconfirm title={`确定删除选中的 ${selectedKeys.length} 条记录?`} onConfirm={handleBatchDelete}>
              <Button danger disabled={selectedKeys.length === 0}>批量删除</Button>
            </Popconfirm>
          )}
          {canCreate && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建岗位</Button>
          )}
        </Space>
      )}
    >
      <Row gutter={16} style={{ marginBottom: 12 }}>
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
              options={[{ label: '启用', value: true }, { label: '停用', value: false }]}
            />
          </Space>
        </Col>
      </Row>

      <SearchBar
        fields={[{ name: 'keyword', label: '岗位编码/岗位名称', placeholder: '请输入岗位编码或岗位名称' }]}
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
        className="e2e-post-modal"
        title={editing ? '编辑岗位' : '新建岗位'}
        open={modalOpen}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
        confirmLoading={saving}
        width={640}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="岗位名称" rules={[{ required: true, message: '请输入岗位名称' }]}>
                <Input placeholder="如：设备运维员" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="code"
                label="岗位编码"
                rules={[{ required: true, message: '请输入岗位编码' }]}
                extra="Casbin g3 主体，建议英文，如 device_ops"
              >
                <Input placeholder="如：device_ops" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="sequence" label="排序" rules={[{ required: true, message: '请输入排序' }]}>
                <InputNumber min={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="active" label="状态" valuePropName="checked">
                <Switch checkedChildren="启用" unCheckedChildren="停用" />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="description" label="描述">
                <Input.TextArea rows={3} placeholder="请输入描述" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>
    </PageContainer>
  )
}
