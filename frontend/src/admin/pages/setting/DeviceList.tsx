import React, { useCallback, useEffect, useState } from 'react'
import {
  Table, Button, Modal, Form, Input, InputNumber, Select, Switch, Space, Row, Col,
  message, Popconfirm, Tag,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, EditOutlined, DeleteOutlined, SyncOutlined, ThunderboltOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import RegionSelect from '../../components/RegionSelect'
import StatusTag from '../../components/StatusTag'
import { DeviceController } from '../../api/generated/DeviceController'
import type { Device } from '../../api/generated/definitions/Device'
import type { CreateDeviceReq } from '../../api/generated/definitions/CreateDeviceReq'
import type { OptionItem } from '../../api/generated/definitions/OptionItem'
import { useAuthStore } from '../../store/authStore'

/** 设备运行状态下拉选项 */
const STATUS_OPTIONS = [
  { value: 'online', label: '在线' },
  { value: 'offline', label: '离线' },
]

/** 设备表单值 */
interface DeviceFormValues {
  name: string
  device_code: string
  region_id?: number
  location?: string
  latitude?: string
  longitude?: string
  work_mode?: string
  device_mode?: string
  iot_card_id?: number
  info_interval?: number
  stream_url?: string
  remark?: string
}

/** 切换模式弹窗的表单值 */
interface SwitchModeFormValues {
  mode: string
}

/** 设备管理：AI 摄像头设备的台账、模式切换与实时监控开关 */
const DeviceList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('device:create')
  const canUpdate = permCodes.includes('device:update')
  const canDelete = permCodes.includes('device:delete')
  const canSyncTime = permCodes.includes('device:sync-time')
  const canSwitchMode = permCodes.includes('device:switch-mode')
  const canMonitor = permCodes.includes('device:monitor')

  const [data, setData] = useState<Device[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState<string | undefined>(undefined)
  const [regionId, setRegionId] = useState<number | undefined>(undefined)
  const [deviceMode, setDeviceMode] = useState<string | undefined>(undefined)
  const [modeOptions, setModeOptions] = useState<OptionItem[]>([])
  const [cardOptions, setCardOptions] = useState<OptionItem[]>([])
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Device | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<DeviceFormValues>()

  const [modeModalOpen, setModeModalOpen] = useState(false)
  const [modeTarget, setModeTarget] = useState<Device | null>(null)
  const [modeSaving, setModeSaving] = useState(false)
  const [modeForm] = Form.useForm<SwitchModeFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await DeviceController.pageDevices(page, pageSize, keyword, regionId, status, deviceMode)
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取设备列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, regionId, status, deviceMode])

  useEffect(() => { fetchData() }, [fetchData])

  // 设备模式与物联网卡下拉数据（仅加载一次）
  useEffect(() => {
    DeviceController.deviceModeOptions()
      .then((res) => setModeOptions(res.data ?? []))
      .catch(() => setModeOptions([]))
    DeviceController.iotCardOptions()
      .then((res) => setCardOptions(res.data ?? []))
      .catch(() => setCardOptions([]))
  }, [])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setStatus(undefined)
    setRegionId(undefined)
    setDeviceMode(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ info_interval: 60 })
    setModalOpen(true)
  }

  const openEdit = (row: Device) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      device_code: row.device_code ?? '',
      region_id: row.region_id,
      location: row.location ?? '',
      latitude: row.latitude ?? '',
      longitude: row.longitude ?? '',
      work_mode: row.work_mode ?? '',
      device_mode: row.device_mode ?? '',
      iot_card_id: row.iot_card_id,
      info_interval: row.info_interval ?? 60,
      stream_url: row.stream_url ?? '',
      remark: row.remark ?? '',
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    const payload: CreateDeviceReq = {
      name: values.name,
      device_code: values.device_code,
      region_id: values.region_id,
      location: values.location,
      latitude: values.latitude,
      longitude: values.longitude,
      work_mode: values.work_mode,
      device_mode: values.device_mode,
      iot_card_id: values.iot_card_id,
      info_interval: values.info_interval,
      stream_url: values.stream_url,
      remark: values.remark,
    }
    setSaving(true)
    try {
      if (editing?.id) {
        await DeviceController.updateDevice(String(editing.id), payload)
        message.success('修改成功')
      } else {
        await DeviceController.createDevice(payload)
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

  const handleDelete = async (row: Device) => {
    try {
      await DeviceController.deleteDevice(String(row.id))
      message.success('删除成功')
      fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  /** 批量删除：逐个调用删除接口，全部完成后刷新列表 */
  const handleBatchDelete = async () => {
    if (!selectedRowKeys.length) return
    try {
      await Promise.all(selectedRowKeys.map((k) => DeviceController.deleteDevice(String(k))))
      message.success(`已删除 ${selectedRowKeys.length} 台设备`)
      setSelectedRowKeys([])
      fetchData()
    } catch {
      message.error('批量删除失败')
    }
  }

  /** 同步设备时间：成功后提示服务器返回的时间 */
  const handleSyncTime = async (row: Device) => {
    try {
      const res = await DeviceController.syncDeviceTime(String(row.id))
      message.success(`时间同步成功，服务器时间：${res.data?.sync_time ?? '-'}`)
      fetchData()
    } catch {
      message.error('时间同步失败')
    }
  }

  /** 设备监控开关 */
  const handleToggleMonitor = async (row: Device, enabled: boolean) => {
    try {
      await DeviceController.toggleDeviceMonitor(String(row.id), { enabled })
      message.success(enabled ? '已加入实时监控' : '已移出实时监控')
      fetchData()
    } catch {
      message.error('操作失败')
    }
  }

  const openModeSwitch = (row: Device) => {
    setModeTarget(row)
    modeForm.resetFields()
    modeForm.setFieldsValue({ mode: row.device_mode ?? '' })
    setModeModalOpen(true)
  }

  const handleModeSave = async () => {
    const values = await modeForm.validateFields()
    if (!modeTarget?.id) return
    setModeSaving(true)
    try {
      await DeviceController.switchDeviceMode(String(modeTarget.id), { mode: values.mode })
      message.success('设备模式切换成功')
      setModeModalOpen(false)
      fetchData()
    } catch {
      message.error('模式切换失败')
    } finally {
      setModeSaving(false)
    }
  }

  const columns: ColumnsType<Device> = [
    { title: '设备名称', dataIndex: 'name', width: 180, ellipsis: true },
    {
      title: '设备编号',
      dataIndex: 'device_code',
      width: 160,
      sorter: (a, b) => (a.device_code ?? '').localeCompare(b.device_code ?? ''),
    },
    { title: '所属区域', dataIndex: ['region', 'name'], width: 140, render: (_, row) => row.region?.name ?? '-' },
    { title: '设备位置', dataIndex: 'location', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '模型',
      key: 'model',
      width: 200,
      render: (_, row) => (row.ai_model?.name ? `${row.ai_model.name}${row.model_ver ? ` / ${row.model_ver}` : ''}` : (row.model_ver || '-')),
    },
    { title: '系统版本', dataIndex: 'system_ver', width: 110, render: (v?: string) => v || '-' },
    { title: '设备工作模式', dataIndex: 'work_mode', width: 120, render: (v?: string) => v || '-' },
    {
      title: '设备模式',
      dataIndex: 'device_mode',
      width: 120,
      render: (v: string | undefined, row) =>
        row.device_mode ? (
          <Tag
            color="blue"
            style={{ cursor: canSwitchMode ? 'pointer' : 'default' }}
            onClick={() => canSwitchMode && openModeSwitch(row)}
          >
            {row.device_mode}
          </Tag>
        ) : (
          <a onClick={() => canSwitchMode && openModeSwitch(row)}>未设置</a>
        ),
    },
    {
      title: '信息间隔',
      dataIndex: 'info_interval',
      width: 100,
      render: (v?: number) => (v ? `${v} 秒` : '-'),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (v?: string) => <StatusTag status={v ?? 'offline'} type="device" />,
    },
    {
      title: '设备监控',
      key: 'monitor',
      width: 100,
      render: (_, row) => (
        <Switch
          size="small"
          checked={row.monitor_enabled ?? false}
          disabled={!canMonitor}
          onChange={(checked) => handleToggleMonitor(row, checked)}
        />
      ),
    },
    {
      title: '操作',
      key: 'action',
      width: 260,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons>
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
              编辑
            </Button>
          )}
          {canSyncTime && (
            <Button type="link" size="small" icon={<SyncOutlined />} onClick={() => handleSyncTime(row)}>
              同步时间
            </Button>
          )}
          {canSwitchMode && (
            <Button type="link" size="small" icon={<ThunderboltOutlined />} onClick={() => openModeSwitch(row)}>
              切换模式
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
      title="设备管理"
      subTitle="AI 摄像头设备台账，支持模式切换、时间同步与实时监控开关"
      extra={
        <Space>
          {canDelete && (
            <Popconfirm title={`确定删除选中的 ${selectedRowKeys.length} 台设备?`} onConfirm={handleBatchDelete} disabled={!selectedRowKeys.length}>
              <Button danger disabled={!selectedRowKeys.length}>
                批量删除
              </Button>
            </Popconfirm>
          )}
          {canCreate && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新增设备
            </Button>
          )}
        </Space>
      }
    >
      <SearchBar
        fields={[{ name: 'keyword', label: '设备名称/编号', placeholder: '请输入设备名称或设备编号' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Space wrap style={{ marginBottom: 16 }}>
        <Select
          allowClear
          placeholder="设备状态"
          style={{ width: 150 }}
          options={STATUS_OPTIONS}
          value={status}
          onChange={(v?: string) => { setStatus(v); setPage(1) }}
        />
        <div style={{ width: 190 }}>
          <RegionSelect
            placeholder="所属区域"
            value={regionId}
            onChange={(v?: number) => { setRegionId(v); setPage(1) }}
          />
        </div>
        <Select
          allowClear
          placeholder="设备模式"
          style={{ width: 170 }}
          value={deviceMode}
          onChange={(v?: string) => { setDeviceMode(v); setPage(1) }}
          options={modeOptions.map((m) => ({ value: m.name ?? '', label: m.name ?? '' }))}
        />
      </Space>

      <Table<Device>
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 'max-content' }}
        rowSelection={{
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys),
        }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => { setPage(p); setPageSize(ps) },
        }}
      />

      {/* 新增 / 编辑设备 */}
      <Modal
        className="e2e-device-modal"
        title={editing ? '编辑设备' : '新增设备'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        confirmLoading={saving}
        destroyOnHidden
        width={820}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="设备名称" rules={[{ required: true, message: '请输入设备名称' }]}>
                <Input placeholder="如：1号摄像头" maxLength={255} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="device_code" label="设备编号" rules={[{ required: true, message: '请输入设备编号' }]}>
                <Input placeholder="如：CAM001" maxLength={100} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="region_id" label="所属区域" rules={[{ required: true, message: '请选择所属区域' }]}>
                <RegionSelect placeholder="请选择所属区域" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="location" label="设备位置">
                <Input placeholder="如：东门入口" maxLength={255} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="longitude" label="经度">
                <Input placeholder="如：120.123456" maxLength={32} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="latitude" label="纬度">
                <Input placeholder="如：30.123456" maxLength={32} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="work_mode" label="设备工作模式">
                <Input placeholder="如：正常" maxLength={50} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="device_mode" label="设备模式">
                <Select
                  allowClear
                  placeholder="请选择设备模式"
                  options={modeOptions.map((m) => ({ value: m.name ?? '', label: m.name ?? '' }))}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="iot_card_id" label="物联网卡">
                <Select
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  placeholder="请选择物联网卡"
                  options={cardOptions.map((c) => ({ value: c.id ?? 0, label: c.name ?? '' }))}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="info_interval" label="信息间隔（秒）">
                <InputNumber min={1} max={86400} style={{ width: '100%' }} placeholder="默认 60 秒" />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="stream_url" label="流地址">
                <Input placeholder="如：rtsp://192.168.1.10/live" maxLength={500} />
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

      {/* 切换设备模式 */}
      <Modal
        className="e2e-device-mode-switch-modal"
        title={`切换设备模式${modeTarget?.name ? ` - ${modeTarget.name}` : ''}`}
        open={modeModalOpen}
        onCancel={() => setModeModalOpen(false)}
        onOk={handleModeSave}
        confirmLoading={modeSaving}
        destroyOnHidden
        width={480}
      >
        <Form form={modeForm} layout="vertical">
          <Form.Item name="mode" label="设备模式" rules={[{ required: true, message: '请选择设备模式' }]}>
            <Select
              placeholder="请选择设备模式"
              options={modeOptions.map((m) => ({ value: m.name ?? '', label: m.name ?? '' }))}
            />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  )
}

export default DeviceList
