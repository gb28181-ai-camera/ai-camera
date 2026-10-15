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
import type { Firmware } from '../../api/generated/definitions/Firmware'
import type { CreateFirmwareReq } from '../../api/generated/definitions/CreateFirmwareReq'
import type { OptionItem } from '../../api/generated/definitions/OptionItem'
import { useAuthStore } from '../../store/authStore'

/** 固件表单值 */
interface FirmwareFormValues {
  name: string
  version: string
  firmware_type_id?: number
  file_url?: string
  file_size?: number
  md5?: string
  release_dt?: Dayjs
  description?: string
}

/**
 * 文件大小格式化：入参单位为字节，按 KB / MB / GB 逐级折算。
 */
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

/** 固件管理：设备升级所用的固件包台账 */
const FirmwareList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('upgrade:firmware:create')
  const canUpdate = permCodes.includes('upgrade:firmware:update')
  const canDelete = permCodes.includes('upgrade:firmware:delete')

  const [data, setData] = useState<Firmware[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [typeId, setTypeId] = useState<number | undefined>(undefined)
  const [typeOptions, setTypeOptions] = useState<OptionItem[]>([])

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Firmware | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<FirmwareFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await UpgradeController.pageFirmwares(page, pageSize, keyword, typeId)
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取固件列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, typeId])

  useEffect(() => { fetchData() }, [fetchData])

  useEffect(() => {
    UpgradeController.firmwareTypeOptions()
      .then((res) => setTypeOptions(res.data ?? []))
      .catch(() => setTypeOptions([]))
  }, [])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setTypeId(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ release_dt: dayjs() })
    setModalOpen(true)
  }

  const openEdit = (row: Firmware) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      version: row.version ?? '',
      firmware_type_id: row.firmware_type_id,
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
    const payload: CreateFirmwareReq = {
      name: values.name,
      version: values.version,
      firmware_type_id: values.firmware_type_id,
      file_url: values.file_url,
      file_size: values.file_size,
      md5: values.md5,
      release_dt: values.release_dt ? values.release_dt.format('YYYY-MM-DD HH:mm:ss') : undefined,
      description: values.description,
    }
    setSaving(true)
    try {
      if (editing?.id) {
        await UpgradeController.updateFirmware(String(editing.id), payload)
        message.success('修改成功')
      } else {
        await UpgradeController.createFirmware(payload)
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

  const handleDelete = async (row: Firmware) => {
    try {
      await UpgradeController.deleteFirmware(String(row.id))
      message.success('删除成功')
      fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  const columns: ColumnsType<Firmware> = [
    { title: '固件名称', dataIndex: 'name', width: 200, ellipsis: true },
    { title: '版本号', dataIndex: 'version', width: 110 },
    { title: '固件类型', dataIndex: ['firmware_type', 'name'], width: 140, render: (_, row) => row.firmware_type?.name ?? '-' },
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
      title="固件管理"
      subTitle="设备升级所用的固件包，文件地址可直接填写或粘贴上传后的 URL"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增固件
          </Button>
        ) : null
      }
    >
      <SearchBar
        fields={[{ name: 'keyword', label: '固件名称', placeholder: '请输入固件名称/版本号' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Space wrap style={{ marginBottom: 16 }}>
        <Select
          allowClear
          showSearch
          optionFilterProp="label"
          placeholder="固件类型"
          style={{ width: 190 }}
          value={typeId}
          onChange={(v?: number) => { setTypeId(v); setPage(1) }}
          options={typeOptions.map((t) => ({ value: t.id ?? 0, label: t.name ?? '' }))}
        />
      </Space>

      <Table<Firmware>
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
        className="e2e-firmware-modal"
        title={editing ? '编辑固件' : '新增固件'}
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
              <Form.Item name="name" label="固件名称" rules={[{ required: true, message: '请输入固件名称' }]}>
                <Input placeholder="如：主控固件V1.2.0" maxLength={255} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="version" label="版本号" rules={[{ required: true, message: '请输入版本号' }]}>
                <Input placeholder="如：v1.2.0" maxLength={50} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="firmware_type_id" label="固件类型">
                <Select
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  placeholder="请选择固件类型"
                  options={typeOptions.map((t) => ({ value: t.id ?? 0, label: t.name ?? '' }))}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="release_dt" label="发布时间">
                <DatePicker showTime style={{ width: '100%' }} placeholder="请选择发布时间" />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item
                name="file_url"
                label="固件文件"
                tooltip="可直接填写文件 URL，或先通过上传接口上传后粘贴返回的地址"
              >
                <Input placeholder="如：/uploads/firmware/v1.2.0.bin" maxLength={500} />
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

export default FirmwareList
