import React, { useCallback, useEffect, useState } from 'react'
import { Table, Button, Modal, Form, Input, InputNumber, message, Popconfirm } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { BasicController } from '../../api/generated/BasicController'
import type { Region } from '../../api/generated/definitions/Region'
import type { CreateRegionReq } from '../../api/generated/definitions/CreateRegionReq'
import { useAuthStore } from '../../store/authStore'

/** 区域表单值 */
interface RegionFormValues {
  name: string
  code?: string
  sequence?: number
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

/** 区域管理：设备的归属维度，几乎所有业务列表都支持按区域筛选 */
const RegionList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('basic:region:create')
  const canUpdate = permCodes.includes('basic:region:update')
  const canDelete = permCodes.includes('basic:region:delete')

  const [data, setData] = useState<Region[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Region | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<RegionFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await BasicController.pageRegions(page, pageSize, keyword)
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取区域列表失败')
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

  const openEdit = (row: Region) => {
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
    const payload: CreateRegionReq = {
      name: values.name,
      code: values.code,
      sequence: values.sequence,
      remark: values.remark,
    }
    setSaving(true)
    try {
      if (editing?.id) {
        await BasicController.updateRegion(String(editing.id), payload)
        message.success('修改成功')
      } else {
        await BasicController.createRegion(payload)
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

  const handleDelete = async (row: Region) => {
    try {
      await BasicController.deleteRegion(String(row.id))
      message.success('删除成功')
      fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  const columns: ColumnsType<Region> = [
    { title: '区域名称', dataIndex: 'name', width: 200, ellipsis: true },
    { title: '区域编码', dataIndex: 'code', width: 160 },
    { title: '排序', dataIndex: 'sequence', width: 80 },
    { title: '备注', dataIndex: 'remark', width: 240, ellipsis: true, render: (v?: string) => v || '-' },
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
      title="区域管理"
      subTitle="维护设备的区域归属，用于风险、告警与巡检的区域维度筛选"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增区域
          </Button>
        ) : null
      }
    >
      <SearchBar
        fields={[{ name: 'keyword', label: '区域名称', placeholder: '请输入区域名称/编码' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<Region>
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
        className="e2e-region-modal"
        title={editing ? '编辑区域' : '新增区域'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        confirmLoading={saving}
        destroyOnHidden
        width={640}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="区域名称" rules={[{ required: true, message: '请输入区域名称' }]}>
            <Input placeholder="请输入区域名称" maxLength={255} />
          </Form.Item>
          <Form.Item name="code" label="区域编码">
            <Input placeholder="请输入区域编码，如 REGION_EAST" maxLength={50} />
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

export default RegionList
