import { useCallback, useEffect, useState } from 'react'
import type { Key } from 'react'
import {
  Button, Card, Col, Form, Input, InputNumber, Modal, Popconfirm, Row, Space, Switch,
  Table, Tag, Tree, TreeSelect, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { TreeDataNode } from 'antd'
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import ActionButtons from '../../components/ActionButtons'
import UserSelect from '../../components/UserSelect'
import { DeptController } from '../../api/generated/DeptController'
import { UserController } from '../../api/generated/UserController'
import { useAuthStore } from '../../store/authStore'
import type { Dept } from '../../api/generated/definitions/Dept'

/** TreeSelect 节点：value 用于取值，key 为 rc-tree 必需的身份标识 */
interface DeptTreeSelectNode {
  key: number
  value: number
  title: string
  children?: DeptTreeSelectNode[]
}

/** 部门表单值 */
interface DeptFormValues {
  name: string
  code?: string
  parent_id?: number
  sequence: number
  leader_id?: number
  active?: boolean
  description?: string
}

/**
 * 部门管理：左侧部门树 + 右侧同级部门表格。
 * 部门编码 code 是 Casbin 的 g2 主体，用于「部门 → 角色」的权限继承。
 */
export default function DeptTree() {
  const [deptTree, setDeptTree] = useState<Dept[]>([])
  const [loading, setLoading] = useState(false)
  const [selectedId, setSelectedId] = useState<number | undefined>(undefined)
  const [selectedDept, setSelectedDept] = useState<Dept | null>(null)

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Dept | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<DeptFormValues>()
  // leader_id → 姓名：列表接口只返回负责人 ID，这里用用户选项映射为姓名
  const [userNames, setUserNames] = useState<Record<number, string>>({})

  const permCodes = useAuthStore((s) => s.permCodes)
  // permCodes 为空说明菜单权限尚未加载，此时不隐藏按钮，避免整页操作被锁死
  const can = (code: string) => permCodes.length === 0 || permCodes.includes(code)
  const canCreate = can('system:dept:create')
  const canUpdate = can('system:dept:update')
  const canDelete = can('system:dept:delete')

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await DeptController.treeDepts()
      if (res.code === 200) {
        setDeptTree(res.data ?? [])
      } else {
        message.error(res.message || '获取部门树失败')
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  // 用户选项：把部门负责人 ID 映射为姓名展示
  useEffect(() => {
    UserController.options().then((res) => {
      if (res.code !== 200) return
      const map: Record<number, string> = {}
      ;(res.data ?? []).forEach((item) => {
        if (typeof item.id === 'number') map[item.id] = String(item.name ?? '')
      })
      setUserNames(map)
    })
  }, [])

  /** 在树中按 ID 查找部门 */
  const findDept = (nodes: Dept[], id: number): Dept | null => {
    for (const node of nodes) {
      if (node.id === id) return node
      if (node.children && node.children.length > 0) {
        const hit = findDept(node.children, id)
        if (hit) return hit
      }
    }
    return null
  }

  const handleSelect = (keys: Key[]) => {
    const id = keys.length > 0 ? Number(keys[0]) : undefined
    setSelectedId(id)
    setSelectedDept(id ? findDept(deptTree, id) : null)
  }

  /** 树节点 → TreeSelect 数据（排除自身及其子孙，避免把父级选成自己） */
  const toTreeSelectData = (nodes: Dept[], excludeId?: number): DeptTreeSelectNode[] =>
    nodes
      .filter((node) => node.id !== excludeId)
      .map((node) => ({
        key: node.id ?? 0,
        value: node.id ?? 0,
        title: String(node.name ?? ''),
        children: node.children && node.children.length > 0 ? toTreeSelectData(node.children) : undefined,
      }))

  /** 树节点 → antd Tree 数据，节点标题内联操作按钮 */
  const toTreeData = (nodes: Dept[]): TreeDataNode[] =>
    nodes.map((node) => ({
      key: node.id ?? 0,
      title: (
        <Space size={4}>
          <span>{node.name}</span>
          {canUpdate && (
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={(event) => {
                event.stopPropagation()
                openEdit(node)
              }}
            />
          )}
          {canCreate && (
            <Button
              type="link"
              size="small"
              icon={<PlusOutlined />}
              onClick={(event) => {
                event.stopPropagation()
                openCreate(node)
              }}
            />
          )}
          {canDelete && (
            <Popconfirm title="确定删除?" onConfirm={() => handleDelete(node)}>
              <Button
                type="link"
                size="small"
                danger
                icon={<DeleteOutlined />}
                onClick={(event) => event.stopPropagation()}
              />
            </Popconfirm>
          )}
        </Space>
      ),
      children: node.children && node.children.length > 0 ? toTreeData(node.children) : undefined,
    }))

  const openCreate = (parent?: Dept | null) => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({
      parent_id: parent?.id ?? selectedId,
      sequence: 10,
      active: true,
    })
    setModalOpen(true)
  }

  const openEdit = (record: Dept) => {
    setEditing(record)
    form.setFieldsValue({
      name: record.name ?? '',
      code: record.code,
      parent_id: record.parent_id ?? undefined,
      sequence: record.sequence ?? 10,
      leader_id: record.leader_id ?? undefined,
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
        const res = await DeptController.updateDept(String(editing.id), values)
        if (res.code === 200) {
          message.success('更新成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '更新失败')
        }
      } else {
        const res = await DeptController.createDept(values)
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

  const handleDelete = async (record: Dept) => {
    try {
      const res = await DeptController.deleteDept(String(record.id))
      if (res.code === 200) {
        message.success('删除成功')
        if (selectedId === record.id) {
          setSelectedId(undefined)
          setSelectedDept(null)
        }
        fetchData()
      } else {
        message.error(res.message || '删除失败')
      }
    } catch {
      message.error('删除失败')
    }
  }

  const columns: ColumnsType<Dept> = [
    { title: '部门名称', dataIndex: 'name', key: 'name', width: 200 },
    {
      title: '部门编码', dataIndex: 'code', key: 'code', width: 150,
      render: (value: string) => (value ? <Tag color="blue">{value}</Tag> : '-'),
    },
    { title: '排序', dataIndex: 'sequence', key: 'sequence', width: 80 },
    {
      title: '负责人', dataIndex: 'leader_id', key: 'leader_id', width: 120,
      render: (value: number) => (value ? userNames[value] ?? `用户 #${value}` : '-'),
    },
    {
      title: '状态', key: 'active', width: 90,
      render: (_, record) => (record.active === false ? <Tag>停用</Tag> : <Tag color="green">启用</Tag>),
    },
    {
      title: '操作', key: 'action', width: 220, fixed: 'right' as const,
      render: (_, record) => (
        <ActionButtons>
          {canCreate && (
            <Button type="link" size="small" icon={<PlusOutlined />} onClick={() => openCreate(record)}>新增下级</Button>
          )}
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除?存在下级部门时不可删除。" onConfirm={() => handleDelete(record)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>删除</Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  // 右侧表格数据：选中节点时展示其下级，否则展示顶级部门
  const tableData = selectedDept?.children && selectedDept.children.length > 0 ? selectedDept.children : deptTree

  return (
    <PageContainer
      title="部门管理"
      subTitle="维护组织架构；部门编码用于「部门 → 角色」的权限继承"
      extra={canCreate ? (
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openCreate(selectedDept)}>新建部门</Button>
      ) : undefined}
    >
      <Row gutter={16}>
        <Col span={8}>
          <Card
            size="small"
            title="组织架构"
            extra={(
              <Button
                size="small"
                type="link"
                onClick={() => {
                  setSelectedId(undefined)
                  setSelectedDept(null)
                }}
              >
                全部
              </Button>
            )}
            loading={loading}
            styles={{ body: { maxHeight: 'calc(100vh - 260px)', overflow: 'auto' } }}
          >
            <Tree
              className="e2e-dept-tree"
              blockNode
              defaultExpandAll
              treeData={toTreeData(deptTree)}
              selectedKeys={selectedId ? [selectedId] : []}
              onSelect={(keys) => handleSelect(keys)}
            />
          </Card>
        </Col>
        <Col span={16}>
          <Card
            size="small"
            title={selectedDept ? `下级部门 - ${selectedDept.name ?? ''}` : '顶级部门'}
            styles={{ body: { padding: 0 } }}
          >
            <Table
              rowKey="id"
              size="small"
              scroll={{ x: 'max-content' }}
              columns={columns}
              dataSource={tableData}
              loading={loading}
              pagination={false}
            />
          </Card>
        </Col>
      </Row>

      <Modal
        className="e2e-dept-modal e2e-org-modal"
        title={editing ? '编辑部门' : '新建部门'}
        open={modalOpen}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
        confirmLoading={saving}
        width={720}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="部门名称" rules={[{ required: true, message: '请输入部门名称' }]}>
                <Input placeholder="如：华东区域" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="code" label="部门编码" extra="Casbin g2 主体，建议英文，如 east_area">
                <Input placeholder="如：east_area" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="parent_id" label="上级部门">
                <TreeSelect
                  allowClear
                  placeholder="不选则为顶级部门"
                  treeData={toTreeSelectData(deptTree, editing?.id)}
                  treeDefaultExpandAll
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="sequence" label="排序" rules={[{ required: true, message: '请输入排序' }]}>
                <InputNumber min={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="leader_id" label="负责人">
                <UserSelect placeholder="请选择负责人" />
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
