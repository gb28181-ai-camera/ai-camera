import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from 'react'
import {
  Button, Col, Form, Input, InputNumber, Modal, Popconfirm, Row, Select, Table, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons'
import axios from 'axios'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { GbSubscribeController } from '../../api/generated/GbSubscribeController'
import { GbDeviceController } from '../../api/generated/GbDeviceController'
import type { GbSubscribeItem } from '../../api/generated/definitions/GbSubscribeItem'
import type { CreateGbSubscribeReq } from '../../api/generated/definitions/CreateGbSubscribeReq'
import type { GbDeviceItem } from '../../api/generated/definitions/GbDeviceItem'
import { useAuthStore } from '../../store/authStore'

/** 订阅类型字典（与后端 validSubscribeType 一致） */
const SUBSCRIBE_TYPE_OPTIONS = [
  { label: '目录订阅', value: 'catalog' },
  { label: '告警订阅', value: 'alarm' },
  { label: '移动位置订阅', value: 'mobile_position' },
]

/** 订阅类型 → 展示文案 / 颜色 */
const TYPE_META: Record<string, { text: string; color: string }> = {
  catalog: { text: '目录订阅', color: 'blue' },
  alarm: { text: '告警订阅', color: 'orange' },
  mobile_position: { text: '移动位置订阅', color: 'purple' },
}

/** 订阅状态字典（后端：active / expired / failed） */
const SUBSCRIBE_STATUS_OPTIONS = [
  { label: '生效中', value: 'active' },
  { label: '已过期', value: 'expired' },
  { label: '失败', value: 'failed' },
]

/** 订阅状态 → 展示文案 / 颜色 */
const STATUS_META: Record<string, { text: string; color: string }> = {
  active: { text: '生效中', color: 'green' },
  expired: { text: '已过期', color: 'default' },
  failed: { text: '失败', color: 'red' },
}

/** 新增订阅表单值 */
interface SubscribeFormValues {
  device_id?: number
  gb_device_id?: string
  type: string
  expires?: number
  call_id?: string
  remark?: string
}

/** 表单子组件对外暴露的提交方法（由 Modal 的 onOk 调用）。 */
interface SubscribeFormHandle {
  submit: () => Promise<void>
}

interface SubscribeFormContentProps {
  deviceOptions: { label: string; value: number }[]
  devices: GbDeviceItem[]
  onSubmit: (values: SubscribeFormValues) => Promise<void>
}

/**
 * 新增订阅表单内容。
 *
 * form 实例放在这里而不是页面组件里：Modal 关闭时 children 不渲染
 * （destroyOnHidden），useForm 与 Form 同时挂载/卸载，避免
 * 「Instance created by useForm is not connected to any Form element」。
 */
const SubscribeFormContent = forwardRef<SubscribeFormHandle, SubscribeFormContentProps>(
  function SubscribeFormContent({ deviceOptions, devices, onSubmit }, ref) {
    const [form] = Form.useForm<SubscribeFormValues>()

    useImperativeHandle(ref, () => ({
      async submit() {
        let values: SubscribeFormValues
        try {
          values = await form.validateFields()
        } catch {
          return // 校验失败，antd 已在字段下方展示原因
        }
        await onSubmit(values)
      },
    }), [form, onSubmit])

    /** 选择设备后同步回填国标编码（也可手工只填国标编码） */
    const handleDeviceChange = (deviceID?: number) => {
      const device = devices.find((item) => item.id === deviceID)
      form.setFieldsValue({
        device_id: deviceID,
        gb_device_id: device?.gb_device_id || device?.device_code || '',
      })
    }

    return (
      <Form
        form={form}
        layout="vertical"
        initialValues={{ type: 'catalog', expires: 3600 }}
      >
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="device_id" label="国标设备" extra="选择后自动回填国标编码">
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                placeholder="请选择国标设备"
                options={deviceOptions}
                onChange={(value?: number) => handleDeviceChange(value)}
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="gb_device_id"
              label="国标设备编码"
              rules={[{ required: true, message: '请输入或选择国标设备编码' }]}
            >
              <Input placeholder="20 位国标编码" maxLength={20} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="type" label="订阅类型" rules={[{ required: true, message: '请选择订阅类型' }]}>
              <Select options={SUBSCRIBE_TYPE_OPTIONS} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="expires" label="间隔（秒）" extra="缺省 3600 秒">
              <InputNumber min={1} style={{ width: '100%' }} placeholder="3600" />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item name="call_id" label="Call-ID">
              <Input placeholder="可选，SIP 订阅的 Call-ID" maxLength={255} />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item name="remark" label="备注">
              <Input.TextArea rows={3} maxLength={500} showCount placeholder="请输入备注" />
            </Form.Item>
          </Col>
        </Row>
      </Form>
    )
  },
)

/** 时间格式化：后端返回 ISO 字符串，此处仅展示到分钟 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 请求失败兜底提示（HTTP 错误已由 request 拦截器提示后端 message，避免重复） */
function notifyError(err: unknown, fallback: string) {
  if (axios.isAxiosError(err) && err.response) return
  message.error(fallback)
}

/**
 * 国标订阅记录：登记平台侧的 SIP 订阅关系（catalog / alarm / mobile_position）。
 *
 * 注意：本页新增只登记订阅关系；真正向设备下发 SUBSCRIBE 使用国标设备页的「订阅」操作。
 */
export default function GbSubscribeList() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('gb:subscription:create')
  const canDelete = permCodes.includes('gb:subscription:delete')

  const [data, setData] = useState<GbSubscribeItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [gbDeviceID, setGbDeviceID] = useState('')
  const [type, setType] = useState<string | undefined>()
  const [status, setStatus] = useState<string | undefined>()

  // 新增订阅
  const [createOpen, setCreateOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const createFormRef = useRef<SubscribeFormHandle>(null)
  const [devices, setDevices] = useState<GbDeviceItem[]>([])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbSubscribeController.pageGbSubscribes(
        page, pageSize, gbDeviceID || undefined, type, status
      )
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch (err) {
      notifyError(err, '获取订阅记录失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, gbDeviceID, type, status])

  useEffect(() => { void fetchData() }, [fetchData])

  // 国标设备选项：新增订阅时选择设备，自动回填国标编码
  useEffect(() => {
    GbDeviceController.pageGbDevices(1, 500)
      .then((res) => setDevices(res.data?.items ?? []))
      .catch(() => setDevices([]))
  }, [])

  const deviceOptions = devices
    .filter((item) => typeof item.id === 'number')
    .map((item) => {
      const gbID = item.gb_device_id || item.device_code || ''
      const name = item.name || `设备 #${item.id}`
      return { label: gbID ? `${name}（${gbID}）` : name, value: Number(item.id) }
    })

  const handleSearch = (values: Record<string, unknown>) => {
    setGbDeviceID(typeof values.gb_device_id === 'string' ? values.gb_device_id : '')
    setType(typeof values.type === 'string' && values.type ? values.type : undefined)
    setStatus(typeof values.status === 'string' && values.status ? values.status : undefined)
    setPage(1)
  }

  const handleReset = () => {
    setGbDeviceID('')
    setType(undefined)
    setStatus(undefined)
    setPage(1)
  }

  const openCreate = () => setCreateOpen(true)

  const handleCreate = async (values: SubscribeFormValues) => {
    const payload: CreateGbSubscribeReq = {
      device_id: values.device_id,
      gb_device_id: values.gb_device_id || undefined,
      type: values.type,
      expires: values.expires,
      call_id: values.call_id || undefined,
      remark: values.remark || undefined,
    }
    setCreating(true)
    try {
      const res = await GbSubscribeController.createGbSubscribe(payload)
      if (res.code === 200) {
        message.success('新增订阅成功')
        setCreateOpen(false)
        fetchData()
      } else {
        message.error(res.message || '新增订阅失败')
      }
    } catch (err) {
      notifyError(err, '新增订阅失败')
    } finally {
      setCreating(false)
    }
  }

  const handleDelete = async (row: GbSubscribeItem) => {
    if (!row.id) return
    try {
      const res = await GbSubscribeController.deleteGbSubscribe(row.id)
      if (res.code === 200) {
        message.success('删除成功')
        fetchData()
      } else {
        message.error(res.message || '删除失败')
      }
    } catch (err) {
      notifyError(err, '删除失败')
    }
  }

  const columns: ColumnsType<GbSubscribeItem> = [
    { title: '设备名称', dataIndex: 'device_name', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '设备编码', dataIndex: 'gb_device_id', width: 190, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '订阅类型', dataIndex: 'type', width: 130,
      render: (v?: string) => {
        if (!v) return '-'
        const meta = TYPE_META[v] ?? { text: v, color: 'default' }
        return <Tag color={meta.color}>{meta.text}</Tag>
      },
    },
    {
      title: '间隔（秒）', dataIndex: 'expires', width: 100,
      render: (v?: number) => (v === undefined || v === null ? '-' : v),
    },
    { title: '到期时间', dataIndex: 'expire_dt', width: 170, render: (v?: string) => fmtDT(v) },
    {
      title: '状态', dataIndex: 'status', width: 100,
      render: (v?: string) => {
        if (!v) return '-'
        const meta = STATUS_META[v] ?? { text: v, color: 'default' }
        return <Tag color={meta.color}>{meta.text}</Tag>
      },
    },
    { title: 'Call-ID', dataIndex: 'call_id', width: 200, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '创建时间', dataIndex: 'created_dt', width: 170, render: (v?: string) => fmtDT(v) },
    { title: '备注', dataIndex: 'remark', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '操作',
      key: 'action',
      width: 120,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={120}>
          {canDelete && (
            <Popconfirm title="确定删除该订阅记录?" onConfirm={() => void handleDelete(row)}>
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
      title="订阅记录"
      subTitle="平台侧 SIP 订阅关系登记；下发订阅请使用国标设备页的「订阅」操作"
      extra={canCreate ? (
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          新增订阅
        </Button>
      ) : null}
    >
      <SearchBar
        fields={[
          { name: 'gb_device_id', label: '设备编码', placeholder: '请输入国标设备编码' },
          { name: 'type', label: '订阅类型', type: 'select', options: SUBSCRIBE_TYPE_OPTIONS },
          { name: 'status', label: '状态', type: 'select', options: SUBSCRIBE_STATUS_OPTIONS },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<GbSubscribeItem>
        className="e2e-gb-subscribe-table"
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 'max-content' }}
        locale={{ emptyText: '暂无订阅记录' }}
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
        className="e2e-gb-subscribe-create-modal"
        title="新增订阅"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={() => void createFormRef.current?.submit()}
        confirmLoading={creating}
        width={640}
        destroyOnHidden
      >
        <SubscribeFormContent
          ref={createFormRef}
          deviceOptions={deviceOptions}
          devices={devices}
          onSubmit={handleCreate}
        />
      </Modal>
    </PageContainer>
  )
}
