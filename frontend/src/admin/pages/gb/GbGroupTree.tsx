import { useCallback, useEffect, useMemo, useState } from 'react'
import type { Key } from 'react'
import {
  Button, Card, Col, Descriptions, Empty, Form, Input, InputNumber, Modal, Popconfirm, Row, Space, Switch,
  Tag, Tree, TreeSelect, message,
} from 'antd'
import type { TreeDataNode } from 'antd'
import { ApartmentOutlined, DeleteOutlined, EditOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import axios from 'axios'
import PageContainer from '../../components/PageContainer'
import { GbGroupController } from '../../api/generated/GbGroupController'
import type { GbGroupTreeNode } from '../../api/generated/definitions/GbGroupTreeNode'
import type { CreateGbGroupReq } from '../../api/generated/definitions/CreateGbGroupReq'
import type { UpdateGbGroupReq } from '../../api/generated/definitions/UpdateGbGroupReq'
import { useAuthStore } from '../../store/authStore'

/** TreeSelect 节点 */
interface GroupTreeSelectNode {
  key: number
  value: number
  title: string
  children?: GroupTreeSelectNode[]
}

/** 业务分组表单值 */
interface GroupFormValues {
  name: string
  parent_id?: number
  sequence?: number
  device_id?: string
  parent_device_id?: string
  business_group?: string
  civil_code?: string
  alias?: string
  active?: boolean
  remark?: string
}

/** 时间格式化：后端返回 ISO 字符串，此处仅展示到分钟 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 请求失败兜底提示（HTTP 错误已由 request 拦截器提示后端 message，避免重复） */
function notifyError(err: unknown, fallback: string) {
  if (axios.isAxiosError(err) && err.response) return
  message.error(fallback)
}

/** 在树中按 ID 查找分组 */
function findGroup(nodes: GbGroupTreeNode[], id: number): GbGroupTreeNode | null {
  for (const node of nodes) {
    if (node.id === id) return node
    if (node.children && node.children.length > 0) {
      const hit = findGroup(node.children, id)
      if (hit) return hit
    }
  }
  return null
}

/**
 * 业务分组：与区域正交的树形组织维度，决定「按分组推送目录」的可见范围。
 * 左侧树 + 右侧详情；新增 / 编辑走弹窗，删除前确认且后端会校验子节点与通道引用。
 */
export default function GbGroupTree() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('gb:group:create')
  const canUpdate = permCodes.includes('gb:group:update')
  const canDelete = permCodes.includes('gb:group:delete')

  const [tree, setTree] = useState<GbGroupTreeNode[]>([])
  const [loading, setLoading] = useState(false)
  const [selectedID, setSelectedID] = useState<number | undefined>()

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<GbGroupTreeNode | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<GroupFormValues>()

  const selected = useMemo(
    () => (selectedID === undefined ? null : findGroup(tree, selectedID)),
    [tree, selectedID]
  )

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbGroupController.gbGroupTree()
      if (res.code === 200) {
        setTree(res.data ?? [])
      } else {
        message.error(res.message || '获取业务分组树失败')
      }
    } catch (err) {
      notifyError(err, '获取业务分组树失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void fetchData() }, [fetchData])

  /** 树节点 → TreeSelect 数据（排除自身及其子孙，避免父子成环） */
  const toTreeSelectData = (nodes: GbGroupTreeNode[], excludeId?: number): GroupTreeSelectNode[] =>
    nodes
      .filter((node) => node.id !== excludeId)
      .map((node) => ({
        key: Number(node.id ?? 0),
        value: Number(node.id ?? 0),
        title: String(node.name ?? ''),
        children: node.children && node.children.length > 0
          ? toTreeSelectData(node.children, excludeId)
          : undefined,
      }))

  /** 树节点 → antd Tree 数据 */
  const toTreeData = (nodes: GbGroupTreeNode[]): TreeDataNode[] =>
    nodes.map((node) => ({
      key: Number(node.id ?? 0),
      title: (
        <Space size={4}>
          <span>{node.name || `分组 #${node.id}`}</span>
          {node.active === false && <Tag>停用</Tag>}
        </Space>
      ),
      children: node.children && node.children.length > 0 ? toTreeData(node.children) : undefined,
    }))

  const handleSelect = (keys: Key[]) => {
    setSelectedID(keys.length > 0 ? Number(keys[0]) : undefined)
  }

  const openCreate = (parent?: GbGroupTreeNode | null) => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({
      parent_id: parent?.id,
      sequence: 10,
      active: true,
    })
    setModalOpen(true)
  }

  const openEdit = (node: GbGroupTreeNode) => {
    setEditing(node)
    form.resetFields()
    form.setFieldsValue({
      name: node.name ?? '',
      parent_id: node.parent_id ?? undefined,
      sequence: node.sequence ?? 10,
      device_id: node.device_id ?? '',
      parent_device_id: node.parent_device_id ?? '',
      business_group: node.business_group ?? '',
      civil_code: node.civil_code ?? '',
      alias: node.alias ?? '',
      active: node.active !== false,
      remark: node.remark ?? '',
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    let values: GroupFormValues
    try {
      values = await form.validateFields()
    } catch {
      return // 表单校验未通过，antd 已在字段下方展示原因
    }
    setSaving(true)
    try {
      if (editing?.id) {
        const payload: UpdateGbGroupReq = {
          name: values.name,
          // 后端约定：parent_id 传 0 表示提升为根分组
          parent_id: values.parent_id ?? 0,
          sequence: values.sequence,
          device_id: values.device_id,
          parent_device_id: values.parent_device_id,
          business_group: values.business_group,
          civil_code: values.civil_code,
          alias: values.alias,
          active: values.active,
          remark: values.remark,
        }
        const res = await GbGroupController.updateGbGroup(editing.id, payload)
        if (res.code === 200) {
          message.success('更新成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '更新失败')
        }
      } else {
        const payload: CreateGbGroupReq = {
          name: values.name,
          parent_id: values.parent_id,
          sequence: values.sequence,
          device_id: values.device_id,
          parent_device_id: values.parent_device_id,
          business_group: values.business_group,
          civil_code: values.civil_code,
          alias: values.alias,
          active: values.active,
          remark: values.remark,
        }
        const res = await GbGroupController.createGbGroup(payload)
        if (res.code === 200) {
          message.success('创建成功')
          setModalOpen(false)
          if (res.data?.id) setSelectedID(res.data.id)
          fetchData()
        } else {
          message.error(res.message || '创建失败')
        }
      }
    } catch (err) {
      notifyError(err, '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (node: GbGroupTreeNode) => {
    if (!node.id) return
    try {
      const res = await GbGroupController.deleteGbGroup(node.id)
      if (res.code === 200) {
        message.success('删除成功')
        if (selectedID === node.id) setSelectedID(undefined)
        fetchData()
      } else {
        // 后端会返回「存在子分组」/「仍被通道引用」等原因，原样展示
        message.error(res.message || '删除失败')
      }
    } catch (err) {
      notifyError(err, '删除失败')
    }
  }

  return (
    <PageContainer
      title="业务分组"
      subTitle="与区域正交的树形组织维度，决定「按分组推送目录」的可见范围"
      extra={(
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void fetchData()} loading={loading}>
            刷新
          </Button>
          {canCreate && (
            <Button type="primary" icon={<PlusOutlined />} onClick={() => openCreate(null)}>
              新建分组
            </Button>
          )}
        </Space>
      )}
    >
      <Row gutter={16}>
        <Col span={8}>
          <Card
            size="small"
            title="分组树"
            extra={(
              <Button
                size="small"
                type="link"
                onClick={() => setSelectedID(undefined)}
              >
                全部
              </Button>
            )}
            loading={loading}
            styles={{ body: { maxHeight: 'calc(100vh - 260px)', overflow: 'auto' } }}
          >
            {tree.length === 0 ? (
              <Empty description="暂无业务分组" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            ) : (
              <Tree
                className="e2e-gb-group-tree"
                blockNode
                defaultExpandAll
                treeData={toTreeData(tree)}
                selectedKeys={selectedID !== undefined ? [selectedID] : []}
                onSelect={(keys) => handleSelect(keys)}
              />
            )}
          </Card>
        </Col>
        <Col span={16}>
          <Card
            size="small"
            title={selected ? `分组详情 - ${selected.name ?? ''}` : '分组详情'}
            extra={selected ? (
              <Space>
                {canCreate && (
                  <Button size="small" icon={<PlusOutlined />} onClick={() => openCreate(selected)}>
                    新增子分组
                  </Button>
                )}
                {canUpdate && (
                  <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(selected)}>
                    编辑
                  </Button>
                )}
                {canDelete && (
                  <Popconfirm
                    title="确定删除该分组?"
                    description="存在子分组或仍被通道引用时不可删除。"
                    onConfirm={() => void handleDelete(selected)}
                  >
                    <Button size="small" danger icon={<DeleteOutlined />}>
                      删除
                    </Button>
                  </Popconfirm>
                )}
              </Space>
            ) : undefined}
          >
            {selected ? (
              <Descriptions
                bordered
                size="small"
                column={2}
                items={[
                  { key: 'name', label: '分组名称', children: selected.name || '-' },
                  { key: 'device_id', label: '分组国标编码', children: selected.device_id || '-' },
                  { key: 'parent_device_id', label: '父分组国标编码', children: selected.parent_device_id || '-' },
                  { key: 'business_group', label: '业务分组类型编码', children: selected.business_group || '-' },
                  { key: 'civil_code', label: '行政区划代码', children: selected.civil_code || '-' },
                  { key: 'alias', label: '别名', children: selected.alias || '-' },
                  { key: 'sequence', label: '排序', children: selected.sequence ?? '-' },
                  { key: 'parent_id', label: '父分组 ID', children: selected.parent_id ?? '-' },
                  {
                    key: 'active',
                    label: '状态',
                    children: selected.active === false ? <Tag>停用</Tag> : <Tag color="green">启用</Tag>,
                  },
                  { key: 'created_dt', label: '创建时间', children: fmtDT(selected.created_dt) },
                  { key: 'updated_dt', label: '更新时间', children: fmtDT(selected.updated_dt) },
                  { key: 'remark', label: '备注', children: selected.remark || '-', span: 2 },
                ]}
              />
            ) : (
              <Empty description="请选择左侧分组查看详情" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </Card>
        </Col>
      </Row>

      <Modal
        className="e2e-gb-group-modal"
        title={editing ? '编辑业务分组' : '新建业务分组'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => void handleSave()}
        confirmLoading={saving}
        width={720}
        destroyOnHidden
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="分组名称" rules={[{ required: true, message: '请输入分组名称' }]}>
                <Input placeholder="如：园区一组" maxLength={255} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="parent_id" label="上级分组">
                <TreeSelect
                  allowClear
                  placeholder="不选则为根分组"
                  treeData={toTreeSelectData(tree, editing?.id)}
                  treeDefaultExpandAll
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="device_id" label="分组国标编码" extra="GB/T 28181 业务分组编码">
                <Input placeholder="请输入分组国标编码" maxLength={20} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="parent_device_id" label="父分组国标编码">
                <Input placeholder="请输入父分组国标编码" maxLength={20} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="business_group" label="业务分组类型编码">
                <Input placeholder="请输入业务分组类型编码" maxLength={20} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="civil_code" label="行政区划代码">
                <Input placeholder="请输入行政区划代码" maxLength={20} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="alias" label="别名">
                <Input placeholder="请输入别名" maxLength={255} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="sequence" label="排序">
                <InputNumber min={0} max={9999} style={{ width: '100%' }} placeholder="数值越小越靠前" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="active" label="状态" valuePropName="checked">
                <Switch checkedChildren="启用" unCheckedChildren="停用" />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="remark" label="备注">
                <Input.TextArea rows={3} maxLength={500} showCount placeholder="请输入备注" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
        <div style={{ color: '#8c8c8c', fontSize: 12 }}>
          <ApartmentOutlined /> 删除分组前需先移除其子分组，并解除通道对该分组的引用。
        </div>
      </Modal>
    </PageContainer>
  )
}
