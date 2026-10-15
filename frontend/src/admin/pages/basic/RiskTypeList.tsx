import React, { useCallback, useEffect, useState } from 'react'
import { Table, Button, Modal, Form, Input, InputNumber, Select, Tag, message, Popconfirm } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { BasicController } from '../../api/generated/BasicController'
import type { RiskType } from '../../api/generated/definitions/RiskType'
import type { CreateRiskTypeReq } from '../../api/generated/definitions/CreateRiskTypeReq'
import { useAuthStore } from '../../store/authStore'

/** 风险等级下拉选项 */
const LEVEL_OPTIONS = [
  { value: 'high', label: '高' },
  { value: 'medium', label: '中' },
  { value: 'low', label: '低' },
]

/** 风险等级 Tag 配色：高=红、中=橙、低=绿 */
const LEVEL_COLOR: Record<string, string> = {
  high: 'red',
  medium: 'orange',
  low: 'green',
}

const LEVEL_LABEL: Record<string, string> = {
  high: '高',
  medium: '中',
  low: '低',
}

/** 风险类型表单值 */
interface RiskTypeFormValues {
  name: string
  code?: string
  level?: string
  sequence?: number
  description?: string
}

/** 时间格式化 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 渲染风险等级 Tag */
function renderLevel(level?: string) {
  if (!level) return '-'
  return <Tag color={LEVEL_COLOR[level] ?? 'default'}>{LEVEL_LABEL[level] ?? level}</Tag>
}

/** 风险类型管理：AI 识别出的风险分类，等级用于分级统计与告警配色 */
const RiskTypeList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('basic:risk-type:create')
  const canUpdate = permCodes.includes('basic:risk-type:update')
  const canDelete = permCodes.includes('basic:risk-type:delete')

  const [data, setData] = useState<RiskType[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [level, setLevel] = useState<string | undefined>(undefined)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<RiskType | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<RiskTypeFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await BasicController.pageRiskTypes(page, pageSize, keyword, level)
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取风险类型列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, level])

  useEffect(() => { fetchData() }, [fetchData])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setLevel(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ level: 'medium', sequence: 10 })
    setModalOpen(true)
  }

  const openEdit = (row: RiskType) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      code: row.code ?? '',
      level: row.level ?? 'medium',
      sequence: row.sequence ?? 10,
      description: row.description ?? '',
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    const payload: CreateRiskTypeReq = {
      name: values.name,
      code: values.code,
      level: values.level,
      sequence: values.sequence,
      description: values.description,
    }
    setSaving(true)
    try {
      if (editing?.id) {
        await BasicController.updateRiskType(String(editing.id), payload)
        message.success('修改成功')
      } else {
        await BasicController.createRiskType(payload)
        message.success('新增成功')
      }
      setModalOpen(false)
      fetchData()
    } catch {
      message.error('保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (row: RiskType) => {
    try {
      await BasicController.deleteRiskType(String(row.id))
      message.success('删除成功')
      fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  const columns: ColumnsType<RiskType> = [
    { title: '风险类型', dataIndex: 'name', width: 220, ellipsis: true },
    { title: '风险编码', dataIndex: 'code', width: 160 },
    { title: '风险等级', dataIndex: 'level', width: 100, render: (v?: string) => renderLevel(v) },
    { title: '描述', dataIndex: 'description', width: 260, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '创建时间', dataIndex: 'created_dt', width: 170, render: (v?: string) => fmtDT(v) },
    {
      title: '操作',
      key: 'action',
      width: 160,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons>
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
              编辑
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除?" onConfirm={() => handleDelete(row)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  return (
    <PageContainer
      title="风险类型管理"
      subTitle="维护 AI 识别风险的分类与等级，等级决定首页统计与告警配色"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增风险类型
          </Button>
        ) : null
      }
    >
      <SearchBar
        fields={[{ name: 'keyword', label: '风险类型', placeholder: '请输入风险类型名称/编码' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <div style={{ marginBottom: 16 }}>
        <Select
          allowClear
          placeholder="风险等级"
          style={{ width: 160 }}
          options={LEVEL_OPTIONS}
          value={level}
          onChange={(v?: string) => {
            setLevel(v)
            setPage(1)
          }}
        />
      </div>

      <Table<RiskType>
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
        className="e2e-risk-type-modal"
        title={editing ? '编辑风险类型' : '新增风险类型'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        confirmLoading={saving}
        destroyOnHidden
        width={640}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="风险类型" rules={[{ required: true, message: '请输入风险类型名称' }]}>
            <Input placeholder="如：未佩戴安全帽" maxLength={255} />
          </Form.Item>
          <Form.Item name="code" label="风险编码">
            <Input placeholder="如：RISK_HELMET" maxLength={50} />
          </Form.Item>
          <Form.Item name="level" label="风险等级">
            <Select options={LEVEL_OPTIONS} placeholder="请选择风险等级" />
          </Form.Item>
          <Form.Item name="sequence" label="排序">
            <InputNumber min={0} max={9999} style={{ width: '100%' }} placeholder="数值越小越靠前" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} maxLength={500} showCount placeholder="请输入风险描述" />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  )
}

export default RiskTypeList
