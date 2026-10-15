import React, { useCallback, useEffect, useState } from 'react'
import { Table, Button, Modal, Form, Input, InputNumber, Select, Tag, message, Popconfirm } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { BasicController } from '../../api/generated/BasicController'
import type { AlarmType } from '../../api/generated/definitions/AlarmType'
import type { CreateAlarmTypeReq } from '../../api/generated/definitions/CreateAlarmTypeReq'
import { useAuthStore } from '../../store/authStore'

/** 告警等级下拉选项 */
const LEVEL_OPTIONS = [
  { value: 'high', label: '高' },
  { value: 'medium', label: '中' },
  { value: 'low', label: '低' },
]

/** 告警等级 Tag 配色 */
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

/** 告警类型表单值 */
interface AlarmTypeFormValues {
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

/** 渲染告警等级 Tag */
function renderLevel(level?: string) {
  if (!level) return '-'
  return <Tag color={LEVEL_COLOR[level] ?? 'default'}>{LEVEL_LABEL[level] ?? level}</Tag>
}

/**
 * 告警类型管理。
 * 平台内置离线告警（offline）、电量告警（battery）、流量告警（traffic），也可自定义扩展。
 */
const AlarmTypeList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('basic:alarm-type:create')
  const canUpdate = permCodes.includes('basic:alarm-type:update')
  const canDelete = permCodes.includes('basic:alarm-type:delete')

  const [data, setData] = useState<AlarmType[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [level, setLevel] = useState<string | undefined>(undefined)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<AlarmType | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<AlarmTypeFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await BasicController.pageAlarmTypes(page, pageSize, keyword, level)
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取告警类型列表失败')
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

  const openEdit = (row: AlarmType) => {
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
    const payload: CreateAlarmTypeReq = {
      name: values.name,
      code: values.code,
      level: values.level,
      sequence: values.sequence,
      description: values.description,
    }
    setSaving(true)
    try {
      if (editing?.id) {
        await BasicController.updateAlarmType(String(editing.id), payload)
        message.success('修改成功')
      } else {
        await BasicController.createAlarmType(payload)
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

  const handleDelete = async (row: AlarmType) => {
    try {
      await BasicController.deleteAlarmType(String(row.id))
      message.success('删除成功')
      fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  const columns: ColumnsType<AlarmType> = [
    { title: '告警名称', dataIndex: 'name', width: 220, ellipsis: true },
    { title: '告警编码', dataIndex: 'code', width: 160 },
    { title: '告警等级', dataIndex: 'level', width: 100, render: (v?: string) => renderLevel(v) },
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
      title="告警类型管理"
      subTitle="内置离线告警、电量告警、流量告警，可自定义扩展"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增告警类型
          </Button>
        ) : null
      }
    >
      <SearchBar
        fields={[{ name: 'keyword', label: '告警名称', placeholder: '请输入告警名称/编码' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <div style={{ marginBottom: 16 }}>
        <Select
          allowClear
          placeholder="告警等级"
          style={{ width: 160 }}
          options={LEVEL_OPTIONS}
          value={level}
          onChange={(v?: string) => {
            setLevel(v)
            setPage(1)
          }}
        />
      </div>

      <Table<AlarmType>
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
        className="e2e-alarm-type-modal"
        title={editing ? '编辑告警类型' : '新增告警类型'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        confirmLoading={saving}
        destroyOnHidden
        width={640}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="告警名称" rules={[{ required: true, message: '请输入告警名称' }]}>
            <Input placeholder="如：离线告警" maxLength={255} />
          </Form.Item>
          <Form.Item name="code" label="告警编码">
            <Input placeholder="如：offline" maxLength={50} />
          </Form.Item>
          <Form.Item name="level" label="告警等级">
            <Select options={LEVEL_OPTIONS} placeholder="请选择告警等级" />
          </Form.Item>
          <Form.Item name="sequence" label="排序">
            <InputNumber min={0} max={9999} style={{ width: '100%' }} placeholder="数值越小越靠前" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} maxLength={500} showCount placeholder="请输入告警描述" />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  )
}

export default AlarmTypeList
