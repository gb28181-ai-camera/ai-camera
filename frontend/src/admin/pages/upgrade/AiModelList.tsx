import React, { useCallback, useEffect, useState } from 'react'
import {
  Table, Button, Modal, Form, Input, InputNumber, Select, DatePicker, Space, Row, Col,
  message, Popconfirm,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, EditOutlined, DeleteOutlined, DownloadOutlined } from '@ant-design/icons'
import type { Dayjs } from 'dayjs'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { UpgradeController } from '../../api/generated/UpgradeController'
import type { AIModel } from '../../api/generated/definitions/AIModel'
import type { CreateAIModelReq } from '../../api/generated/definitions/CreateAIModelReq'
import type { OptionItem } from '../../api/generated/definitions/OptionItem'
import { useAuthStore } from '../../store/authStore'

/** AI 模型表单值 */
interface AiModelFormValues {
  name: string
  version: string
  firmware_id: number
  file_url?: string
  file_size?: number
  md5?: string
  release_dt?: Dayjs
  description?: string
}

/** 文件大小格式化 */
function fmtFileSize(bytes?: number): string {
  if (bytes === undefined || bytes === null || bytes <= 0) return '-'
  if (bytes >= 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(2)} MB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(2)} KB`
  return `${bytes} B`
}

/** 时间格式化 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 模型管理：AI 识别模型包，必须绑定适用固件（与固件管理数据关联） */
const AiModelList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('upgrade:model:create')
  const canUpdate = permCodes.includes('upgrade:model:update')
  const canDelete = permCodes.includes('upgrade:model:delete')

  const [data, setData] = useState<AIModel[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [firmwareId, setFirmwareId] = useState<number | undefined>(undefined)
  const [firmwareOptions, setFirmwareOptions] = useState<OptionItem[]>([])

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<AIModel | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<AiModelFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await UpgradeController.pageModels(page, pageSize, keyword, firmwareId)
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取模型列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, firmwareId])

  useEffect(() => { fetchData() }, [fetchData])

  // 适用固件下拉数据：与固件管理中的固件数据关联
  useEffect(() => {
    UpgradeController.firmwareOptions()
      .then((res) => setFirmwareOptions(res.data ?? []))
      .catch(() => setFirmwareOptions([]))
  }, [])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setFirmwareId(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ release_dt: dayjs() })
    setModalOpen(true)
  }

  const openEdit = (row: AIModel) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      version: row.version ?? '',
      firmware_id: row.firmware_id ?? 0,
      file_url: row.file_url ?? '',
      file_size: row.file_size,
      md5: row.md5 ?? '',
      release_dt: row.release_dt ? dayjs(row.release_dt) : undefined,
      description: row.description ?? '',
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    const payload: CreateAIModelReq = {
      name: values.name,
      version: values.version,
      firmware_id: values.firmware_id,
      file_url: values.file_url,
      file_size: values.file_size,
      md5: values.md5,
      release_dt: values.release_dt ? values.release_dt.format('YYYY-MM-DD HH:mm:ss') : undefined,
      description: values.description,
    }
    setSaving(true)
    try {
      if (editing?.id) {
        await UpgradeController.updateAIModel(String(editing.id), payload)
        message.success('修改成功')
      } else {
        await UpgradeController.createAIModel(payload)
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

  const handleDelete = async (row: AIModel) => {
    try {
      await UpgradeController.deleteAIModel(String(row.id))
      message.success('删除成功')
      fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  const columns: ColumnsType<AIModel> = [
    { title: '模型名称', dataIndex: 'name', width: 200, ellipsis: true },
    { title: '版本号', dataIndex: 'version', width: 110 },
    { title: '适用固件', dataIndex: ['firmware', 'name'], width: 200, ellipsis: true, render: (_, row) => row.firmware?.name ?? '-' },
    {
      title: '文件',
      dataIndex: 'file_url',
      width: 240,
      ellipsis: true,
      render: (v?: string) =>
        v ? (
          <a href={v} target="_blank" rel="noreferrer" download>
            <DownloadOutlined /> {v}
          </a>
        ) : (
          '-'
        ),
    },
    { title: '文件大小', dataIndex: 'file_size', width: 110, render: (v?: number) => fmtFileSize(v) },
    { title: 'MD5', dataIndex: 'md5', width: 260, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '发布时间', dataIndex: 'release_dt', width: 170, render: (v?: string) => fmtDT(v) },
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
      title="模型管理"
      subTitle="AI 识别模型包，必须绑定适用的固件版本"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增模型
          </Button>
        ) : null
      }
    >
      <SearchBar
        fields={[{ name: 'keyword', label: '模型名称', placeholder: '请输入模型名称/版本号' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Space wrap style={{ marginBottom: 16 }}>
        <Select
          allowClear
          showSearch
          optionFilterProp="label"
          placeholder="适用固件"
          style={{ width: 220 }}
          value={firmwareId}
          onChange={(v?: number) => { setFirmwareId(v); setPage(1) }}
          options={firmwareOptions.map((f) => ({ value: f.id ?? 0, label: f.name ?? '' }))}
        />
      </Space>

      <Table<AIModel>
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
        className="e2e-model-modal"
        title={editing ? '编辑模型' : '新增模型'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        confirmLoading={saving}
        destroyOnHidden
        width={760}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="模型名称" rules={[{ required: true, message: '请输入模型名称' }]}>
                <Input placeholder="如：安全帽识别" maxLength={255} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="version" label="版本号" rules={[{ required: true, message: '请输入版本号' }]}>
                <Input placeholder="如：v1.0.0" maxLength={50} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="firmware_id" label="适用固件" rules={[{ required: true, message: '请选择适用固件' }]}>
                <Select
                  showSearch
                  optionFilterProp="label"
                  placeholder="请选择适用固件"
                  options={firmwareOptions.map((f) => ({ value: f.id ?? 0, label: f.name ?? '' }))}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="release_dt" label="发布时间">
                <DatePicker showTime style={{ width: '100%' }} placeholder="请选择发布时间" />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="file_url" label="模型文件">
                <Input placeholder="如：/uploads/model/helmet.zip" maxLength={500} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="file_size" label="文件大小（字节）">
                <InputNumber min={0} style={{ width: '100%' }} placeholder="单位：字节" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="md5" label="MD5 校验值">
                <Input placeholder="请输入文件的 MD5" maxLength={64} />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="description" label="更新说明">
                <Input.TextArea rows={3} maxLength={500} showCount placeholder="请输入更新说明" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>
    </PageContainer>
  )
}

export default AiModelList
