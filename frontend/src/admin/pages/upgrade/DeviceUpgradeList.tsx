import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Table, Button, Modal, Form, Input, Select, Radio, DatePicker, Space, Tabs, Row, Col,
  Tag, message, Popconfirm,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { RocketOutlined, StopOutlined } from '@ant-design/icons'
import type { Dayjs } from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import RegionSelect from '../../components/RegionSelect'
import StatusTag from '../../components/StatusTag'
import { DeviceController } from '../../api/generated/DeviceController'
import { UpgradeController } from '../../api/generated/UpgradeController'
import type { Device } from '../../api/generated/definitions/Device'
import type { DeviceUpgradePlan } from '../../api/generated/definitions/DeviceUpgradePlan'
import type { DeviceUpgradeRecord } from '../../api/generated/definitions/DeviceUpgradeRecord'
import type { UpgradeDevicesReq } from '../../api/generated/definitions/UpgradeDevicesReq'
import type { OptionItem } from '../../api/generated/definitions/OptionItem'
import { useAuthStore } from '../../store/authStore'

/** 批量升级表单值 */
interface UpgradeFormValues {
  firmware_id?: number
  ai_model_id?: number
  upgrade_type: 'now' | 'plan'
  plan_dt?: Dayjs
}

/** 时间格式化 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 设备升级管理：可升级设备 / 升级计划 / 升级记录 三个视图 */
const DeviceUpgradeList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canIssue = permCodes.includes('upgrade:issue')
  const canCancel = permCodes.includes('upgrade:cancel')

  const [activeTab, setActiveTab] = useState('devices')
  const [modeOptions, setModeOptions] = useState<OptionItem[]>([])
  const [firmwareOptions, setFirmwareOptions] = useState<OptionItem[]>([])
  const [modelOptions, setModelOptions] = useState<OptionItem[]>([])

  // ── 可升级设备 ──
  const [devices, setDevices] = useState<Device[]>([])
  const [deviceLoading, setDeviceLoading] = useState(false)
  const [deviceTotal, setDeviceTotal] = useState(0)
  const [devicePage, setDevicePage] = useState(1)
  const [devicePageSize, setDevicePageSize] = useState(20)
  const [deviceKeyword, setDeviceKeyword] = useState('')
  const [deviceRegionId, setDeviceRegionId] = useState<number | undefined>(undefined)
  const [deviceMode, setDeviceMode] = useState<string | undefined>(undefined)
  const [firmwareFilter, setFirmwareFilter] = useState<number | undefined>(undefined)
  const [selectedDeviceKeys, setSelectedDeviceKeys] = useState<React.Key[]>([])

  // ── 升级计划 ──
  const [plans, setPlans] = useState<DeviceUpgradePlan[]>([])
  const [planLoading, setPlanLoading] = useState(false)
  const [planTotal, setPlanTotal] = useState(0)
  const [planPage, setPlanPage] = useState(1)
  const [planPageSize, setPlanPageSize] = useState(20)
  const [selectedPlanKeys, setSelectedPlanKeys] = useState<React.Key[]>([])

  // ── 升级记录 ──
  const [records, setRecords] = useState<DeviceUpgradeRecord[]>([])
  const [recordLoading, setRecordLoading] = useState(false)
  const [recordTotal, setRecordTotal] = useState(0)
  const [recordPage, setRecordPage] = useState(1)
  const [recordPageSize, setRecordPageSize] = useState(20)

  const [modalOpen, setModalOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<UpgradeFormValues>()

  const fetchDevices = useCallback(async () => {
    setDeviceLoading(true)
    try {
      const res = await DeviceController.pageDevices(devicePage, devicePageSize, deviceKeyword, deviceRegionId, undefined, deviceMode)
      setDevices(res.data?.items ?? [])
      setDeviceTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取设备列表失败')
    } finally {
      setDeviceLoading(false)
    }
  }, [devicePage, devicePageSize, deviceKeyword, deviceRegionId, deviceMode])

  const fetchPlans = useCallback(async () => {
    setPlanLoading(true)
    try {
      const res = await UpgradeController.pageUpgradePlans(planPage, planPageSize)
      setPlans(res.data?.items ?? [])
      setPlanTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取升级计划失败')
    } finally {
      setPlanLoading(false)
    }
  }, [planPage, planPageSize])

  const fetchRecords = useCallback(async () => {
    setRecordLoading(true)
    try {
      const res = await UpgradeController.pageUpgradeRecords(recordPage, recordPageSize)
      setRecords(res.data?.items ?? [])
      setRecordTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取升级记录失败')
    } finally {
      setRecordLoading(false)
    }
  }, [recordPage, recordPageSize])

  useEffect(() => {
    if (activeTab === 'devices') fetchDevices()
    else if (activeTab === 'plans') fetchPlans()
    else fetchRecords()
  }, [activeTab, fetchDevices, fetchPlans, fetchRecords])

  useEffect(() => {
    DeviceController.deviceModeOptions()
      .then((res) => setModeOptions(res.data ?? []))
      .catch(() => setModeOptions([]))
    UpgradeController.firmwareOptions()
      .then((res) => setFirmwareOptions(res.data ?? []))
      .catch(() => setFirmwareOptions([]))
    UpgradeController.modelOptions()
      .then((res) => setModelOptions(res.data ?? []))
      .catch(() => setModelOptions([]))
  }, [])

  const handleSearch = (values: Record<string, string>) => {
    setDeviceKeyword(values.keyword ?? '')
    setDevicePage(1)
  }

  const handleReset = () => {
    setDeviceKeyword('')
    setDeviceRegionId(undefined)
    setDeviceMode(undefined)
    setFirmwareFilter(undefined)
    setDevicePage(1)
  }

  /**
   * 设备列表按固件筛选。
   * 后端设备分页接口未提供固件筛选参数，故此处对当前页数据做前端过滤（界面已标注）。
   */
  const displayedDevices = useMemo(() => {
    if (!firmwareFilter) return devices
    return devices.filter((d) => d.firmware_id === firmwareFilter)
  }, [devices, firmwareFilter])

  const openUpgrade = () => {
    form.resetFields()
    form.setFieldsValue({ upgrade_type: 'now' })
    setModalOpen(true)
  }

  /** 批量升级下发（现在升级 / 定时升级） */
  const handleUpgrade = async () => {
    const values = await form.validateFields()
    if (values.upgrade_type === 'plan' && !values.plan_dt) {
      message.warning('定时升级必须选择定时时间')
      return
    }
    const payload: UpgradeDevicesReq = {
      device_ids: selectedDeviceKeys.map((k) => Number(k)),
      firmware_id: values.firmware_id,
      ai_model_id: values.ai_model_id,
      upgrade_type: values.upgrade_type,
      plan_dt: values.plan_dt ? values.plan_dt.format('YYYY-MM-DD HH:mm:ss') : undefined,
    }
    setSaving(true)
    try {
      const res = await UpgradeController.upgradeDevices(payload)
      const done = res.data?.success ?? 0
      message.success(values.upgrade_type === 'now' ? `已下发升级，成功 ${done} 台` : '定时升级计划已创建')
      setModalOpen(false)
      setSelectedDeviceKeys([])
      fetchDevices()
      if (values.upgrade_type === 'plan') fetchPlans()
    } catch {
      message.error('升级下发失败')
    } finally {
      setSaving(false)
    }
  }

  /** 取消单条升级计划 */
  const handleCancelPlan = async (row: DeviceUpgradePlan) => {
    try {
      await UpgradeController.cancelUpgradePlan(String(row.id))
      message.success('已取消升级计划')
      fetchPlans()
    } catch {
      message.error('取消失败')
    }
  }

  /** 批量取消升级计划 */
  const handleBatchCancel = async () => {
    if (!selectedPlanKeys.length) return
    try {
      await UpgradeController.batchCancelUpgradePlans({ ids: selectedPlanKeys.map((k) => Number(k)) })
      message.success(`已取消 ${selectedPlanKeys.length} 条升级计划`)
      setSelectedPlanKeys([])
      fetchPlans()
    } catch {
      message.error('批量取消失败')
    }
  }

  const deviceColumns: ColumnsType<Device> = [
    { title: '设备名称', dataIndex: 'name', width: 170, ellipsis: true },
    { title: '设备编号', dataIndex: 'device_code', width: 150 },
    { title: '所属区域', dataIndex: ['region', 'name'], width: 140, render: (_, row) => row.region?.name ?? '-' },
    { title: '设备位置', dataIndex: 'location', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '当前固件',
      key: 'firmware',
      width: 180,
      render: (_, row) => row.firmware?.name ?? row.firmware_ver ?? '-',
    },
    {
      title: '当前模型',
      key: 'model',
      width: 180,
      render: (_, row) => row.ai_model?.name ?? row.model_ver ?? '-',
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (v?: string) => <StatusTag status={v ?? 'offline'} type="device" />,
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons>
          <Button
            type="link"
            size="small"
            disabled={!canIssue}
            onClick={() => {
              setSelectedDeviceKeys(row.id ? [row.id] : [])
              openUpgrade()
            }}
          >
            升级
          </Button>
        </ActionButtons>
      ),
    },
  ]

  const planColumns: ColumnsType<DeviceUpgradePlan> = [
    { title: '设备名称', dataIndex: 'device_name', width: 170, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '设备编号', dataIndex: 'device_code', width: 150, render: (v?: string) => v || '-' },
    { title: '所属区域', dataIndex: 'region_id', width: 140, render: (v?: number) => (v ? String(v) : '-') },
    { title: '设备位置', dataIndex: 'location', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '升级固件', dataIndex: 'firmware_name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '升级模型', dataIndex: 'ai_model_name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '升级时间', dataIndex: 'plan_dt', width: 170, render: (v?: string) => fmtDT(v) },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (v?: string) => <StatusTag status={v ?? 'pending'} type="upgrade" />,
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons>
          {canCancel && (
            <Popconfirm title="确定取消该升级计划?" onConfirm={() => handleCancelPlan(row)}>
              <Button type="link" size="small" danger icon={<StopOutlined />}>
                取消升级
              </Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  const recordColumns: ColumnsType<DeviceUpgradeRecord> = [
    { title: '设备名称', dataIndex: 'device_name', width: 170, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '设备编号', dataIndex: 'device_code', width: 150, render: (v?: string) => v || '-' },
    { title: '所属区域', dataIndex: 'region_id', width: 140, render: (v?: number) => (v ? String(v) : '-') },
    { title: '设备位置', dataIndex: 'location', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '流量卡', dataIndex: 'iot_card_no', width: 170, render: (v?: string) => v || '-' },
    { title: '上一固件', dataIndex: 'prev_firmware_ver', width: 120, render: (v?: string) => v || '-' },
    { title: '上一模型', dataIndex: 'prev_ai_model_ver', width: 120, render: (v?: string) => v || '-' },
    { title: '升级固件', dataIndex: 'firmware_ver', width: 120, render: (v?: string) => v || '-' },
    { title: '升级模型', dataIndex: 'ai_model_ver', width: 120, render: (v?: string) => v || '-' },
    { title: '升级时间', dataIndex: 'upgrade_dt', width: 170, render: (v?: string) => fmtDT(v) },
    {
      title: '结果',
      dataIndex: 'status',
      width: 100,
      render: (v?: string) => <StatusTag status={v ?? 'done'} type="upgrade" />,
    },
  ]

  const devicesTab = (
    <>
      <SearchBar
        fields={[{ name: 'keyword', label: '设备名称/编号', placeholder: '请输入设备名称或设备编号' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={deviceLoading}
      />
      <Space wrap style={{ marginBottom: 16 }}>
        <div style={{ width: 190 }}>
          <RegionSelect
            placeholder="所属区域"
            value={deviceRegionId}
            onChange={(v?: number) => { setDeviceRegionId(v); setDevicePage(1) }}
          />
        </div>
        <Select
          allowClear
          placeholder="设备模式"
          style={{ width: 170 }}
          value={deviceMode}
          onChange={(v?: string) => { setDeviceMode(v); setDevicePage(1) }}
          options={modeOptions.map((m) => ({ value: m.name ?? '', label: m.name ?? '' }))}
        />
        <Select
          allowClear
          showSearch
          optionFilterProp="label"
          placeholder="当前固件（本页筛选）"
          style={{ width: 220 }}
          value={firmwareFilter}
          onChange={(v?: number) => setFirmwareFilter(v)}
          options={firmwareOptions.map((f) => ({ value: f.id ?? 0, label: f.name ?? '' }))}
        />
        {canIssue && (
          <Button
            type="primary"
            icon={<RocketOutlined />}
            disabled={!selectedDeviceKeys.length}
            onClick={openUpgrade}
          >
            批量升级{selectedDeviceKeys.length ? `（${selectedDeviceKeys.length}）` : ''}
          </Button>
        )}
      </Space>
      <Table<Device>
        rowKey="id"
        size="small"
        loading={deviceLoading}
        columns={deviceColumns}
        dataSource={displayedDevices}
        scroll={{ x: 'max-content' }}
        rowSelection={{ selectedRowKeys: selectedDeviceKeys, onChange: (keys) => setSelectedDeviceKeys(keys) }}
        pagination={{
          current: devicePage,
          pageSize: devicePageSize,
          total: deviceTotal,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => { setDevicePage(p); setDevicePageSize(ps) },
        }}
      />
    </>
  )

  const plansTab = (
    <>
      {canCancel && (
        <div style={{ marginBottom: 16 }}>
          <Popconfirm
            title={`确定取消选中的 ${selectedPlanKeys.length} 条升级计划?`}
            onConfirm={handleBatchCancel}
            disabled={!selectedPlanKeys.length}
          >
            <Button danger disabled={!selectedPlanKeys.length}>
              批量取消
            </Button>
          </Popconfirm>
        </div>
      )}
      <Table<DeviceUpgradePlan>
        rowKey="id"
        size="small"
        loading={planLoading}
        columns={planColumns}
        dataSource={plans}
        scroll={{ x: 'max-content' }}
        rowSelection={{ selectedRowKeys: selectedPlanKeys, onChange: (keys) => setSelectedPlanKeys(keys) }}
        pagination={{
          current: planPage,
          pageSize: planPageSize,
          total: planTotal,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => { setPlanPage(p); setPlanPageSize(ps) },
        }}
      />
    </>
  )

  const recordsTab = (
    <Table<DeviceUpgradeRecord>
      rowKey="id"
      size="small"
      loading={recordLoading}
      columns={recordColumns}
      dataSource={records}
      scroll={{ x: 'max-content' }}
      pagination={{
        current: recordPage,
        pageSize: recordPageSize,
        total: recordTotal,
        showSizeChanger: true,
        showTotal: (t) => `共 ${t} 条`,
        onChange: (p, ps) => { setRecordPage(p); setRecordPageSize(ps) },
      }}
    />
  )

  return (
    <PageContainer title="设备升级管理" subTitle="设备固件与 AI 模型的批量升级下发、计划排期与升级记录追溯">
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          { key: 'devices', label: '可升级设备', children: devicesTab },
          { key: 'plans', label: '升级计划', children: plansTab },
          { key: 'records', label: '升级记录', children: recordsTab },
        ]}
      />

      {/* 批量升级下发 */}
      <Modal
        className="e2e-device-upgrade-modal"
        title={`批量升级${selectedDeviceKeys.length ? `（已选 ${selectedDeviceKeys.length} 台设备）` : ''}`}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleUpgrade}
        confirmLoading={saving}
        destroyOnHidden
        width={680}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="firmware_id" label="升级固件">
                <Select
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  placeholder="请选择升级固件"
                  options={firmwareOptions.map((f) => ({ value: f.id ?? 0, label: f.name ?? '' }))}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="ai_model_id" label="升级模型">
                <Select
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  placeholder="请选择升级模型"
                  options={modelOptions.map((m) => ({ value: m.id ?? 0, label: m.name ?? '' }))}
                />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="upgrade_type" label="升级方式" rules={[{ required: true, message: '请选择升级方式' }]}>
                <Radio.Group>
                  <Radio value="now">现在升级</Radio>
                  <Radio value="plan">定时升级</Radio>
                </Radio.Group>
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item
                noStyle
                shouldUpdate={(prev, cur) => prev.upgrade_type !== cur.upgrade_type}
              >
                {({ getFieldValue }) =>
                  getFieldValue('upgrade_type') === 'plan' ? (
                    <Form.Item
                      name="plan_dt"
                      label="定时时间"
                      rules={[{ required: true, message: '请选择定时升级时间' }]}
                    >
                      <DatePicker showTime style={{ width: '100%' }} placeholder="请选择定时升级时间" />
                    </Form.Item>
                  ) : null
                }
              </Form.Item>
            </Col>
            <Col span={24}>
              <Tag color="blue">提示：选择「现在升级」将立即下发并在升级记录中留痕；「定时升级」会生成可取消的升级计划。</Tag>
            </Col>
          </Row>
        </Form>
      </Modal>
    </PageContainer>
  )
}

export default DeviceUpgradeList
