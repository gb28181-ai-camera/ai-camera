import React, { useCallback, useEffect, useState } from 'react'
import {
  Table, Button, Modal, Form, Input, InputNumber, Select, DatePicker, Space, Row, Col,
  message, Popconfirm,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import type { Dayjs } from 'dayjs'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import RegionSelect from '../../components/RegionSelect'
import StatusTag from '../../components/StatusTag'
import { DeviceController } from '../../api/generated/DeviceController'
import type { IotCard } from '../../api/generated/definitions/IotCard'
import type { CreateIotCardReq } from '../../api/generated/definitions/CreateIotCardReq'
import type { OptionItem } from '../../api/generated/definitions/OptionItem'
import { useAuthStore } from '../../store/authStore'

/** 物联网卡状态下拉选项 */
const STATUS_OPTIONS = [
  { value: 'normal', label: '正常' },
  { value: 'arrears', label: '欠费' },
  { value: 'expired', label: '到期' },
  { value: 'disabled', label: '停用' },
]

/** 物联网卡表单值 */
interface IotCardFormValues {
  name: string
  card_no: string
  region_id?: number
  device_id?: number
  traffic_total?: number
  traffic_used?: number
  traffic_left?: number
  expire_dt?: Dayjs
  status?: string
  remark?: string
}

/**
 * 流量格式化：入参单位为 MB，大于等于 1024MB 时折算为 GB 展示。
 */
function fmtTraffic(mb?: number): string {
  if (mb === undefined || mb === null) return '-'
  if (mb >= 1024) return `${(mb / 1024).toFixed(2)} GB`
  return `${mb} MB`
}

/** 时间格式化：仅展示到日 */
function fmtDate(v?: string): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** 物联网卡管理：卡号、流量与到期时间的台账 */
const IotCardList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('iot-card:create')
  const canUpdate = permCodes.includes('iot-card:update')
  const canDelete = permCodes.includes('iot-card:delete')

  const [data, setData] = useState<IotCard[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [regionId, setRegionId] = useState<number | undefined>(undefined)
  const [deviceId, setDeviceId] = useState<number | undefined>(undefined)
  const [deviceOptions, setDeviceOptions] = useState<OptionItem[]>([])

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<IotCard | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<IotCardFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await DeviceController.pageIotCards(page, pageSize, keyword, regionId, deviceId)
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取物联网卡列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, regionId, deviceId])

  useEffect(() => { fetchData() }, [fetchData])

  useEffect(() => {
    DeviceController.deviceOptions()
      .then((res) => setDeviceOptions(res.data ?? []))
      .catch(() => setDeviceOptions([]))
  }, [])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setRegionId(undefined)
    setDeviceId(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ status: 'normal', traffic_total: 0, traffic_used: 0, traffic_left: 0 })
    setModalOpen(true)
  }

  const openEdit = (row: IotCard) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      card_no: row.card_no ?? '',
      region_id: row.region_id,
      device_id: row.device_id,
      traffic_total: row.traffic_total ?? 0,
      traffic_used: row.traffic_used ?? 0,
      traffic_left: row.traffic_left ?? 0,
      expire_dt: row.expire_dt ? dayjs(row.expire_dt) : undefined,
      status: row.status ?? 'normal',
      remark: row.remark ?? '',
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    const payload: CreateIotCardReq = {
      name: values.name,
      card_no: values.card_no,
      region_id: values.region_id,
      device_id: values.device_id,
      traffic_total: values.traffic_total,
      traffic_used: values.traffic_used,
      traffic_left: values.traffic_left,
      expire_dt: values.expire_dt ? values.expire_dt.format('YYYY-MM-DD HH:mm:ss') : undefined,
      status: values.status,
      remark: values.remark,
    }
    setSaving(true)
    try {
      if (editing?.id) {
        await DeviceController.updateIotCard(String(editing.id), payload)
        message.success('修改成功')
      } else {
        await DeviceController.createIotCard(payload)
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

  const handleDelete = async (row: IotCard) => {
    try {
      await DeviceController.deleteIotCard(String(row.id))
      message.success('删除成功')
      fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  const columns: ColumnsType<IotCard> = [
    { title: '物联网卡名称', dataIndex: 'name', width: 180, ellipsis: true },
    { title: '卡号', dataIndex: 'card_no', width: 190 },
    { title: '所属区域', dataIndex: ['region', 'name'], width: 140, render: (_, row) => row.region?.name ?? '-' },
    { title: '所属设备', dataIndex: ['device', 'name'], width: 160, render: (_, row) => row.device?.name ?? '-' },
    { title: '流量总额', dataIndex: 'traffic_total', width: 120, render: (v?: number) => fmtTraffic(v) },
    { title: '剩余流量', dataIndex: 'traffic_left', width: 120, render: (v?: number) => fmtTraffic(v) },
    { title: '到期时间', dataIndex: 'expire_dt', width: 140, render: (v?: string) => fmtDate(v) },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (v?: string) => <StatusTag status={v ?? 'normal'} type="iotcard" />,
    },
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
      title="物联网卡管理"
      subTitle="设备流量卡台账，展示流量总额、剩余流量与到期时间"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增物联网卡
          </Button>
        ) : null
      }
    >
      <SearchBar
        fields={[{ name: 'keyword', label: '物联网卡名称/卡号', placeholder: '请输入卡名称或卡号' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Space wrap style={{ marginBottom: 16 }}>
        <div style={{ width: 190 }}>
          <RegionSelect
            placeholder="所属区域"
            value={regionId}
            onChange={(v?: number) => { setRegionId(v); setPage(1) }}
          />
        </div>
        <Select
          allowClear
          showSearch
          optionFilterProp="label"
          placeholder="所属设备"
          style={{ width: 200 }}
          value={deviceId}
          onChange={(v?: number) => { setDeviceId(v); setPage(1) }}
          options={deviceOptions.map((d) => ({ value: d.id ?? 0, label: d.name ?? '' }))}
        />
      </Space>

      <Table<IotCard>
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
        className="e2e-iot-card-modal"
        title={editing ? '编辑物联网卡' : '新增物联网卡'}
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
              <Form.Item name="name" label="物联网卡名称" rules={[{ required: true, message: '请输入物联网卡名称' }]}>
                <Input placeholder="如：移动卡A" maxLength={255} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="card_no" label="卡号" rules={[{ required: true, message: '请输入卡号' }]}>
                <Input placeholder="如：8986001234567890" maxLength={100} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="region_id" label="所属区域">
                <RegionSelect placeholder="请选择所属区域" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="device_id" label="所属设备">
                <Select
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  placeholder="请选择所属设备"
                  options={deviceOptions.map((d) => ({ value: d.id ?? 0, label: d.name ?? '' }))}
                />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="traffic_total" label="流量总额（MB）">
                <InputNumber min={0} style={{ width: '100%' }} placeholder="单位 MB" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="traffic_used" label="已用流量（MB）">
                <InputNumber min={0} style={{ width: '100%' }} placeholder="单位 MB" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="traffic_left" label="剩余流量（MB）">
                <InputNumber min={0} style={{ width: '100%' }} placeholder="单位 MB" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="expire_dt" label="到期时间">
                <DatePicker showTime style={{ width: '100%' }} placeholder="请选择到期时间" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="status" label="状态">
                <Select options={STATUS_OPTIONS} placeholder="请选择状态" />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="remark" label="备注">
                <Input.TextArea rows={2} maxLength={500} showCount placeholder="请输入备注" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>
    </PageContainer>
  )
}

export default IotCardList
