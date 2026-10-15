import React, { useCallback, useEffect, useState } from 'react'
import { Table, Button, Modal, Form, Input, InputNumber, message, Popconfirm } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { UpgradeController } from '../../api/generated/UpgradeController'
import type { FirmwareType } from '../../api/generated/definitions/FirmwareType'
import type { CreateFirmwareTypeReq } from '../../api/generated/definitions/CreateFirmwareTypeReq'
import { useAuthStore } from '../../store/authStore'

/** 固件类型表单值 */
interface FirmwareTypeFormValues {
  name: string
  code?: string
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

/** 固件类型管理：固件的分类维度，固件与 AI 模型均按类型归档 */
const FirmwareTypeList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('upgrade:firmware-type:create')
  const canUpdate = permCodes.includes('upgrade:firmware-type:update')
  const canDelete = permCodes.includes('upgrade:firmware-type:delete')

  const [data, setData] = useState<FirmwareType[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<FirmwareType | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<FirmwareTypeFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await UpgradeController.pageFirmwareTypes(page, pageSize, keyword)
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取固件类型列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword])

  useEffect(() => { fetchData() }, [fetchData])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ sequence: 10 })
    setModalOpen(true)
  }

  const openEdit = (row: FirmwareType) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      code: row.code ?? '',
      sequence: row.sequence ?? 10,
      description: row.description ?? '',
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    const payload: CreateFirmwareTypeReq = {
      name: values.name,
      code: values.code,
      sequence: values.sequence,
      description: values.description,
    }
    setSaving(true)
    try {
      if (editing?.id) {
        await UpgradeController.updateFirmwareType(String(editing.id), payload)
        message.success('修改成功')
      } else {
        await UpgradeController.createFirmwareType(payload)
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

  const handleDelete = async (row: FirmwareType) => {
    try {
      await UpgradeController.deleteFirmwareType(String(row.id))
      message.success('删除成功')
      fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  const columns: ColumnsType<FirmwareType> = [
    { title: '固件类型', dataIndex: 'name', width: 220, ellipsis: true },
    { title: '类型编码', dataIndex: 'code', width: 160 },
    { title: '描述', dataIndex: 'description', width: 300, ellipsis: true, render: (v?: string) => v || '-' },
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
      title="固件类型管理"
      subTitle="固件的分类维度，固件与 AI 模型均按类型归档"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增固件类型
          </Button>
        ) : null
      }
    >
      <SearchBar
        fields={[{ name: 'keyword', label: '固件类型', placeholder: '请输入固件类型名称/编码' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<FirmwareType>
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

      <Modal
        className="e2e-firmware-type-modal"
        title={editing ? '编辑固件类型' : '新增固件类型'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        confirmLoading={saving}
        destroyOnHidden
        width={640}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="固件类型" rules={[{ required: true, message: '请输入固件类型名称' }]}>
            <Input placeholder="如：主控固件" maxLength={255} />
          </Form.Item>
          <Form.Item name="code" label="类型编码">
            <Input placeholder="如：main" maxLength={50} />
          </Form.Item>
          <Form.Item name="sequence" label="排序">
            <InputNumber min={0} max={9999} style={{ width: '100%' }} placeholder="数值越小越靠前" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} maxLength={500} showCount placeholder="请输入描述" />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  )
}

export default FirmwareTypeList
