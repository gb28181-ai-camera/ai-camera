import React, { useCallback, useEffect, useState } from 'react'
import { Table, Button, Modal, Form, Input, InputNumber, Alert, message, Popconfirm } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { DeviceController } from '../../api/generated/DeviceController'
import type { DeviceMode } from '../../api/generated/definitions/DeviceMode'
import type { CreateDeviceModeReq } from '../../api/generated/definitions/CreateDeviceModeReq'
import { useAuthStore } from '../../store/authStore'

/** 设备模式表单值 */
interface DeviceModeFormValues {
  name: string
  code?: string
  sequence?: number
  remark?: string
}

/** 时间格式化 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 设备模式管理：平台内置充电模式、全开模式、省电模式，可扩展 */
const DeviceModeList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('device-mode:create')
  const canUpdate = permCodes.includes('device-mode:update')
  const canDelete = permCodes.includes('device-mode:delete')

  const [data, setData] = useState<DeviceMode[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<DeviceMode | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<DeviceModeFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await DeviceController.pageDeviceModes(page, pageSize, keyword)
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取设备模式列表失败')
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

  const openEdit = (row: DeviceMode) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      code: row.code ?? '',
      sequence: row.sequence ?? 10,
      remark: row.remark ?? '',
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    const payload: CreateDeviceModeReq = {
      name: values.name,
      code: values.code,
      sequence: values.sequence,
      remark: values.remark,
    }
    setSaving(true)
    try {
      if (editing?.id) {
        await DeviceController.updateDeviceMode(String(editing.id), payload)
        message.success('修改成功')
      } else {
        await DeviceController.createDeviceMode(payload)
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

  const handleDelete = async (row: DeviceMode) => {
    try {
      await DeviceController.deleteDeviceMode(String(row.id))
      message.success('删除成功')
      fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  const columns: ColumnsType<DeviceMode> = [
    { title: '模式名称', dataIndex: 'name', width: 200, ellipsis: true },
    { title: '模式编码', dataIndex: 'code', width: 160 },
    { title: '备注', dataIndex: 'remark', width: 280, ellipsis: true, render: (v?: string) => v || '-' },
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
      title="设备模式管理"
      subTitle="设备工作模式字典，供设备模式切换与筛选使用"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增设备模式
          </Button>
        ) : null
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="平台内置三种设备模式：充电模式、全开模式、省电模式；删除内置模式会影响设备正常切换，请谨慎操作。"
      />

      <SearchBar
        fields={[{ name: 'keyword', label: '模式名称', placeholder: '请输入模式名称/编码' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<DeviceMode>
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
        className="e2e-device-mode-modal"
        title={editing ? '编辑设备模式' : '新增设备模式'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        confirmLoading={saving}
        destroyOnHidden
        width={640}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="模式名称" rules={[{ required: true, message: '请输入模式名称' }]}>
            <Input placeholder="如：充电模式" maxLength={255} />
          </Form.Item>
          <Form.Item name="code" label="模式编码">
            <Input placeholder="如：charge" maxLength={50} />
          </Form.Item>
          <Form.Item name="sequence" label="排序">
            <InputNumber min={0} max={9999} style={{ width: '100%' }} placeholder="数值越小越靠前" />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={3} maxLength={500} showCount placeholder="请输入备注" />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  )
}

export default DeviceModeList
