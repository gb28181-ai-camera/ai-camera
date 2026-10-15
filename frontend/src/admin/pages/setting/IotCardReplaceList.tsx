import React, { useCallback, useEffect, useState } from 'react'
import {
  Table, Button, Modal, Form, Input, Select, DatePicker, Space, Row, Col, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { SwapOutlined } from '@ant-design/icons'
import type { Dayjs } from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import RegionSelect from '../../components/RegionSelect'
import DeviceSelect from '../../components/DeviceSelect'
import { DeviceController } from '../../api/generated/DeviceController'
import type { Device } from '../../api/generated/definitions/Device'
import type { IotCardReplaceLog } from '../../api/generated/definitions/IotCardReplaceLog'
import type { ReplaceIotCardReq } from '../../api/generated/definitions/ReplaceIotCardReq'
import type { OptionItem } from '../../api/generated/definitions/OptionItem'
import { useAuthStore } from '../../store/authStore'

const { RangePicker } = DatePicker

/** 换卡登记表单值（设备与当前卡由选择器带入，不在表单内） */
interface ReplaceFormValues {
  new_iot_card_id: number
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

/** 物联网卡更换记录：追加式台账，用于追溯设备历史使用过的卡 */
const IotCardReplaceList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canReplace = permCodes.includes('iot-card:replace')

  const [data, setData] = useState<IotCardReplaceLog[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [regionId, setRegionId] = useState<number | undefined>(undefined)
  const [filterDeviceId, setFilterDeviceId] = useState<number | undefined>(undefined)
  const [range, setRange] = useState<[string, string] | undefined>(undefined)
  const [cardOptions, setCardOptions] = useState<OptionItem[]>([])
  const [deviceOptions, setDeviceOptions] = useState<OptionItem[]>([])

  const [modalOpen, setModalOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [pickedDevice, setPickedDevice] = useState<Device | null>(null)
  const [devicePickerOpen, setDevicePickerOpen] = useState(false)
  const [form] = Form.useForm<ReplaceFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await DeviceController.pageIotCardReplaceLogs(
        page, pageSize, regionId, filterDeviceId, keyword, undefined, range?.[0], range?.[1],
      )
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取换卡记录失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, regionId, filterDeviceId, keyword, range])

  useEffect(() => { fetchData() }, [fetchData])

  useEffect(() => {
    DeviceController.iotCardOptions()
      .then((res) => setCardOptions(res.data ?? []))
      .catch(() => setCardOptions([]))
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
    setFilterDeviceId(undefined)
    setRange(undefined)
    setPage(1)
  }

  const openReplace = () => {
    form.resetFields()
    setPickedDevice(null)
    setModalOpen(true)
  }

  /** 换卡登记：设备选定后由设备行自动带出其当前绑定的物联网卡 */
  const handleSave = async () => {
    if (!pickedDevice?.id) {
      message.warning('请先选择需要换卡的设备')
      return
    }
    const values = await form.validateFields()
    const payload: ReplaceIotCardReq = {
      device_id: pickedDevice.id,
      old_iot_card_id: pickedDevice.iot_card_id,
      new_iot_card_id: values.new_iot_card_id,
      remark: values.remark,
    }
    setSaving(true)
    try {
      await DeviceController.replaceIotCard(payload)
      message.success('换卡登记成功')
      setModalOpen(false)
      fetchData()
    } catch {
      message.error('换卡登记失败')
    } finally {
      setSaving(false)
    }
  }

  const columns: ColumnsType<IotCardReplaceLog> = [
    { title: '设备名称', dataIndex: ['device', 'name'], width: 170, render: (_, row) => row.device?.name ?? '-' },
    { title: '设备编号', dataIndex: 'device_code', width: 150, render: (v?: string) => v || '-' },
    { title: '所属区域', dataIndex: ['region', 'name'], width: 140, render: (_, row) => row.region?.name ?? '-' },
    { title: '设备位置', dataIndex: 'location', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '原物联网卡', dataIndex: 'old_card_no', width: 180, render: (v?: string) => v || <Tag>无</Tag> },
    { title: '新物联网卡', dataIndex: 'new_card_no', width: 180, render: (v?: string) => v || '-' },
    { title: '更换时间', dataIndex: 'replace_dt', width: 170, render: (v?: string) => fmtDT(v) },
    { title: '操作人', dataIndex: 'operator', width: 120, render: (v?: string) => v || '-' },
  ]

  const currentCardText = pickedDevice
    ? (pickedDevice.iot_card?.card_no ?? pickedDevice.iot_card?.name ?? '该设备当前未绑定物联网卡')
    : '请先选择设备'

  return (
    <PageContainer
      title="物联网卡更换记录"
      subTitle="设备换卡流水，用于追溯某台设备历史使用过的卡"
      extra={
        canReplace ? (
          <Button type="primary" icon={<SwapOutlined />} onClick={openReplace}>
            更换记录登记
          </Button>
        ) : null
      }
    >
      <SearchBar
        fields={[{ name: 'keyword', label: '设备编号', placeholder: '请输入设备编号' }]}
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
          placeholder="设备名称"
          style={{ width: 200 }}
          value={filterDeviceId}
          onChange={(v?: number) => { setFilterDeviceId(v); setPage(1) }}
          options={deviceOptions.map((d) => ({ value: d.id ?? 0, label: d.name ?? '' }))}
        />
        <RangePicker
          showTime
          placeholder={['更换开始时间', '更换结束时间']}
          onChange={(values) => {
            if (values && values[0] && values[1]) {
              const [s, e] = values as [Dayjs, Dayjs]
              setRange([s.format('YYYY-MM-DD HH:mm:ss'), e.format('YYYY-MM-DD HH:mm:ss')])
            } else {
              setRange(undefined)
            }
            setPage(1)
          }}
        />
      </Space>

      <Table<IotCardReplaceLog>
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

      {/* 换卡登记：选择设备 → 自动带出当前卡 → 选择新卡 */}
      <Modal
        className="e2e-iot-card-replace-modal"
        title="更换记录登记"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        confirmLoading={saving}
        destroyOnHidden
        width={640}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={24}>
              <Form.Item label="选择设备" required>
                <Space>
                  <Button className="e2e-iot-card-replace-device-btn" onClick={() => setDevicePickerOpen(true)}>
                    {pickedDevice ? `${pickedDevice.name ?? ''} (${pickedDevice.device_code ?? ''})` : '请选择设备'}
                  </Button>
                  {pickedDevice && (
                    <Button type="link" onClick={() => setPickedDevice(null)}>
                      清除
                    </Button>
                  )}
                </Space>
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item label="当前物联网卡">
                <Input value={currentCardText} disabled />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="new_iot_card_id" label="新物联网卡" rules={[{ required: true, message: '请选择新物联网卡' }]}>
                <Select
                  showSearch
                  optionFilterProp="label"
                  placeholder="请选择要更换的新卡"
                  options={cardOptions.map((c) => ({
                    value: c.id ?? 0,
                    label: `${c.name ?? ''}${c.code ? ` (${c.code})` : ''}`,
                  }))}
                />
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

      {/* 设备选择弹窗（受控，Modal 类名 e2e-device-select） */}
      <DeviceSelect
        open={devicePickerOpen}
        onCancel={() => setDevicePickerOpen(false)}
        onOk={(devices) => {
          if (devices.length) setPickedDevice(devices[0])
          setDevicePickerOpen(false)
        }}
      />
    </PageContainer>
  )
}

export default IotCardReplaceList
