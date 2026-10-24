import {
  forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState,
} from 'react'
import {
  Button, Col, Form, Input, InputNumber, Modal, Popconfirm, Row, Select, Switch, Table, Tag, Typography, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  DeleteOutlined, EditOutlined, FolderOutlined, GlobalOutlined, ReloadOutlined,
} from '@ant-design/icons'
import axios from 'axios'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import RegionSelect from '../../components/RegionSelect'
import { GbChannelController } from '../../api/generated/GbChannelController'
import { GbGroupController } from '../../api/generated/GbGroupController'
import { GbRecordPlanController } from '../../api/generated/GbRecordPlanController'
import { DeviceController } from '../../api/generated/DeviceController'
import { RegionController } from '../../api/generated/RegionController'
import type { GbChannelItem } from '../../api/generated/definitions/GbChannelItem'
import type { GbChannelDetailResp } from '../../api/generated/definitions/GbChannelDetailResp'
import type { UpdateGbChannelReq } from '../../api/generated/definitions/UpdateGbChannelReq'
import { useAuthStore } from '../../store/authStore'

/** 通道在线状态字典（设备侧上报 ON/OFF） */
const CHANNEL_STATUS_OPTIONS = [
  { label: '在线', value: 'ON' },
  { label: '离线', value: 'OFF' },
]

/** 通道编辑表单值 */
interface ChannelFormValues {
  name?: string
  manufacturer?: string
  model?: string
  owner?: string
  civil_code?: string
  address?: string
  parent_id?: string
  channel_type?: number | null
  ptz_type?: number | null
  status?: string
  stream_id?: string
  has_audio?: boolean
  enable_broadcast?: boolean
  active?: boolean
  record_plan_id?: number | null
  remark?: string
}

/** 编辑通道的初始值（详情在打开弹窗前已拉取）。 */
function buildChannelInitialValues(editing: GbChannelDetailResp | null): Partial<ChannelFormValues> {
  if (!editing) return {}
  return {
    name: editing.name ?? '',
    manufacturer: editing.manufacturer ?? '',
    model: editing.model ?? '',
    owner: editing.owner ?? '',
    civil_code: editing.civil_code ?? '',
    address: editing.address ?? '',
    parent_id: editing.parent_id ?? '',
    channel_type: editing.channel_type ?? null,
    ptz_type: editing.ptz_type ?? null,
    status: editing.status ?? 'ON',
    stream_id: editing.stream_id ?? '',
    has_audio: editing.has_audio ?? false,
    enable_broadcast: editing.enable_broadcast ?? false,
    active: editing.active !== false,
    record_plan_id: editing.record_plan_id ?? null,
    remark: editing.remark ?? '',
  }
}

interface ChannelFormHandle {
  submit: () => Promise<void>
}

interface ChannelFormContentProps {
  editing: GbChannelDetailResp | null
  recordPlanOptions: { label: string; value: number }[]
  onSubmit: (values: ChannelFormValues) => Promise<void>
}

/**
 * 编辑通道表单内容。
 *
 * 表单实例与 Form 同挂载在 Modal children 内；详情已在父组件拉取完成后
 * 再打开弹窗，因此这里用 initialValues 一次性回填即可。
 */
const ChannelFormContent = forwardRef<ChannelFormHandle, ChannelFormContentProps>(
  function ChannelFormContent({ editing, recordPlanOptions, onSubmit }, ref) {
    const [form] = Form.useForm<ChannelFormValues>()

    useImperativeHandle(ref, () => ({
      async submit() {
        let values: ChannelFormValues
        try {
          values = await form.validateFields()
        } catch {
          return
        }
        await onSubmit(values)
      },
    }), [form, onSubmit])

    return (
      <Form form={form} layout="vertical" initialValues={buildChannelInitialValues(editing)}>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="name" label="通道名称">
              <Input placeholder="请输入通道名称" maxLength={255} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="status" label="通道状态">
              <Select options={CHANNEL_STATUS_OPTIONS} placeholder="请选择通道状态" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="manufacturer" label="厂商">
              <Input placeholder="请输入厂商" maxLength={100} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="model" label="型号">
              <Input placeholder="请输入型号" maxLength={100} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="owner" label="设备归属">
              <Input placeholder="请输入设备归属" maxLength={100} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="civil_code" label="行政区划代码">
              <Input placeholder="请输入行政区划代码" maxLength={20} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="parent_id" label="父级编码">
              <Input placeholder="请输入父级通道编码" maxLength={20} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="stream_id" label="流 ID">
              <Input placeholder="请输入流 ID" maxLength={100} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="channel_type" label="通道类型">
              <InputNumber min={0} style={{ width: '100%' }} placeholder="GB/T 28181 通道类型" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="ptz_type" label="云台类型">
              <InputNumber min={0} style={{ width: '100%' }} placeholder="GB/T 28181 云台类型" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="record_plan_id" label="录像计划">
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                placeholder="不选表示不绑定录像计划"
                options={recordPlanOptions}
              />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item name="address" label="安装地址">
              <Input placeholder="请输入安装地址" maxLength={255} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="has_audio" label="含音频" valuePropName="checked">
              <Switch checkedChildren="是" unCheckedChildren="否" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="enable_broadcast" label="启用广播" valuePropName="checked">
              <Switch checkedChildren="是" unCheckedChildren="否" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="active" label="启用通道" valuePropName="checked">
              <Switch checkedChildren="启用" unCheckedChildren="停用" />
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

/** 通道在线状态标签 */
function ChannelStatusTag({ value }: { value?: string }) {
  if (!value) return <span>-</span>
  const key = value.toUpperCase()
  if (key === 'ON') return <Tag color="green">在线</Tag>
  if (key === 'OFF') return <Tag color="red">离线</Tag>
  return <Tag>{value}</Tag>
}

/**
 * 国标通道：设备目录同步落库后的通道列表。
 * 支持按设备 / 区域 / 状态筛选，维护区域与业务分组归属。
 */
export default function GbChannelList() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canUpdate = permCodes.includes('gb:channel:update')
  const canRegion = permCodes.includes('gb:channel:region')
  const canGroup = permCodes.includes('gb:channel:group')
  const canReset = permCodes.includes('gb:channel:reset')
  const canDelete = permCodes.includes('gb:channel:delete')

  const [data, setData] = useState<GbChannelItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [keyword, setKeyword] = useState('')
  const [deviceID, setDeviceID] = useState<number | undefined>()
  const [regionID, setRegionID] = useState<number | undefined>()
  const [status, setStatus] = useState<string | undefined>()

  const [deviceOptions, setDeviceOptions] = useState<{ label: string; value: number }[]>([])
  const [regionOptions, setRegionOptions] = useState<{ label: string; value: number }[]>([])
  const [groupOptions, setGroupOptions] = useState<{ label: string; value: number }[]>([])
  const [recordPlanOptions, setRecordPlanOptions] = useState<{ label: string; value: number }[]>([])

  // 编辑弹窗
  const [editOpen, setEditOpen] = useState(false)
  const [editing, setEditing] = useState<GbChannelDetailResp | null>(null)
  const [saving, setSaving] = useState(false)
  const formRef = useRef<ChannelFormHandle>(null)

  // 设置区域 / 设置分组弹窗
  const [regionOpen, setRegionOpen] = useState(false)
  const [regionTarget, setRegionTarget] = useState<GbChannelItem | null>(null)
  const [regionValue, setRegionValue] = useState<number | undefined>()
  const [groupOpen, setGroupOpen] = useState(false)
  const [groupTarget, setGroupTarget] = useState<GbChannelItem | null>(null)
  const [groupValue, setGroupValue] = useState<number | undefined>()
  const [setting, setSetting] = useState(false)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbChannelController.pageGbChannels(
        page, pageSize, keyword || undefined, deviceID, regionID, undefined, status
      )
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch (err) {
      notifyError(err, '获取国标通道列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, deviceID, regionID, status])

  useEffect(() => { void fetchData() }, [fetchData])

  // 下拉选项：设备 / 区域 / 分组 / 录像计划
  useEffect(() => {
    DeviceController.deviceOptions()
      .then((res) => {
        setDeviceOptions(
          (res.data ?? [])
            .filter((item) => typeof item.id === 'number')
            .map((item) => {
              const name = String(item.name ?? '')
              const code = String(item.code ?? '')
              return { label: name && code ? `${name}（${code}）` : name || code || `设备 #${item.id}`, value: Number(item.id) }
            })
        )
      })
      .catch(() => setDeviceOptions([]))

    RegionController.options()
      .then((res) => {
        setRegionOptions(
          (res.data ?? [])
            .filter((item) => typeof item.id === 'number')
            .map((item) => ({ label: String(item.name ?? `区域 #${item.id}`), value: Number(item.id) }))
        )
      })
      .catch(() => setRegionOptions([]))

    GbGroupController.listGbGroups()
      .then((res) => {
        setGroupOptions(
          (res.data ?? [])
            .filter((item) => typeof item.id === 'number')
            .map((item) => ({ label: String(item.name ?? `分组 #${item.id}`), value: Number(item.id) }))
        )
      })
      .catch(() => setGroupOptions([]))

    GbRecordPlanController.pageGbRecordPlans(1, 200)
      .then((res) => {
        setRecordPlanOptions(
          (res.data?.items ?? [])
            .filter((item) => typeof item.id === 'number')
            .map((item) => ({ label: String(item.name ?? `计划 #${item.id}`), value: Number(item.id) }))
        )
      })
      .catch(() => setRecordPlanOptions([]))
  }, [])

  const handleSearch = (values: Record<string, unknown>) => {
    setKeyword(typeof values.keyword === 'string' ? values.keyword : '')
    setDeviceID(values.device_id === undefined || values.device_id === null || values.device_id === '' ? undefined : Number(values.device_id))
    setRegionID(values.region_id === undefined || values.region_id === null || values.region_id === '' ? undefined : Number(values.region_id))
    setStatus(typeof values.status === 'string' && values.status ? values.status : undefined)
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setDeviceID(undefined)
    setRegionID(undefined)
    setStatus(undefined)
    setPage(1)
  }

  const openEdit = async (row: GbChannelItem) => {
    if (!row.id) return
    try {
      const res = await GbChannelController.getGbChannel(row.id)
      if (!res.data) {
        message.error(res.message || '获取通道详情失败')
        return
      }
      setEditing(res.data)
      setEditOpen(true)
    } catch (err) {
      notifyError(err, '获取通道详情失败')
    }
  }

  const handleSaveEdit = async (values: ChannelFormValues) => {
    if (!editing?.id) return
    const payload: UpdateGbChannelReq = {
      name: values.name ?? '',
      manufacturer: values.manufacturer ?? '',
      model: values.model ?? '',
      owner: values.owner ?? '',
      civil_code: values.civil_code ?? '',
      address: values.address ?? '',
      parent_id: values.parent_id ?? '',
      channel_type: values.channel_type ?? undefined,
      ptz_type: values.ptz_type ?? undefined,
      status: values.status ?? '',
      stream_id: values.stream_id ?? '',
      has_audio: values.has_audio,
      enable_broadcast: values.enable_broadcast,
      active: values.active,
      record_plan_id: values.record_plan_id ?? 0,
      remark: values.remark ?? '',
    }
    setSaving(true)
    try {
      const res = await GbChannelController.updateGbChannel(String(editing.id), payload)
      if (res.code === 200) {
        message.success('保存成功')
        setEditOpen(false)
        fetchData()
      } else {
        message.error(res.message || '保存失败')
      }
    } catch (err) {
      notifyError(err, '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const openRegion = (row: GbChannelItem) => {
    setRegionTarget(row)
    setRegionValue(row.region_id ?? undefined)
    setRegionOpen(true)
  }

  const handleSetRegion = async () => {
    if (!regionTarget?.id) return
    setSetting(true)
    try {
      const res = await GbChannelController.updateGbChannelRegion(regionTarget.id, {
        // 后端约定：null / 0 表示清空归属区域
        region_id: regionValue ?? 0,
      })
      if (res.code === 200) {
        message.success('区域设置成功')
        setRegionOpen(false)
        fetchData()
      } else {
        message.error(res.message || '设置区域失败')
      }
    } catch (err) {
      notifyError(err, '设置区域失败')
    } finally {
      setSetting(false)
    }
  }

  const openGroup = (row: GbChannelItem) => {
    setGroupTarget(row)
    setGroupValue(row.group_id ?? undefined)
    setGroupOpen(true)
  }

  const handleSetGroup = async () => {
    if (!groupTarget?.id) return
    setSetting(true)
    try {
      const res = await GbChannelController.updateGbChannelGroup(groupTarget.id, {
        // 后端约定：null / 0 表示清空归属分组
        group_id: groupValue ?? 0,
      })
      if (res.code === 200) {
        message.success('分组设置成功')
        setGroupOpen(false)
        fetchData()
      } else {
        message.error(res.message || '设置分组失败')
      }
    } catch (err) {
      notifyError(err, '设置分组失败')
    } finally {
      setSetting(false)
    }
  }

  const handleResetChannel = async (row: GbChannelItem) => {
    if (!row.id) return
    try {
      const res = await GbChannelController.resetGbChannel(row.id)
      if (res.code === 200) {
        message.success('通道已重置')
        fetchData()
      } else {
        message.error(res.message || '重置失败')
      }
    } catch (err) {
      notifyError(err, '重置失败')
    }
  }

  const handleDelete = async (row: GbChannelItem) => {
    if (!row.id) return
    try {
      const res = await GbChannelController.deleteGbChannel(row.id)
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

  const columns: ColumnsType<GbChannelItem> = [
    { title: '通道名称', dataIndex: 'name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '国标通道编码', dataIndex: 'gb_channel_id', width: 190, ellipsis: true,
      render: (v?: string) => (v ? <Typography.Text copyable={{ text: v }}>{v}</Typography.Text> : '-'),
    },
    { title: '所属设备', dataIndex: 'device_name', width: 170, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '在线状态', dataIndex: 'status', width: 100,
      render: (v?: string) => <ChannelStatusTag value={v} />,
    },
    { title: '所属区域', dataIndex: 'region_name', width: 140, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '业务分组', dataIndex: 'group_name', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '录像计划', dataIndex: 'record_plan_name', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '同步时间', dataIndex: 'sync_dt', width: 170, render: (v?: string) => fmtDT(v),
    },
    {
      title: '操作',
      key: 'action',
      width: 360,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={340}>
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => void openEdit(row)}>
              编辑
            </Button>
          )}
          {canRegion && (
            <Button type="link" size="small" icon={<GlobalOutlined />} onClick={() => openRegion(row)}>
              设置区域
            </Button>
          )}
          {canGroup && (
            <Button type="link" size="small" icon={<FolderOutlined />} onClick={() => openGroup(row)}>
              设置分组
            </Button>
          )}
          {canReset && (
            <Popconfirm
              title="确定重置该通道?"
              description="名称、区域、分组、录像计划与备注等平台侧字段将被清空，等待下次目录同步重新填充。"
              onConfirm={() => void handleResetChannel(row)}
            >
              <Button type="link" size="small" icon={<ReloadOutlined />}>
                重置
              </Button>
            </Popconfirm>
          )}
          {canDelete && (
            <Popconfirm
              title="确定删除?"
              description="下次目录同步时若设备仍上报该通道，通道会重新出现。"
              onConfirm={() => void handleDelete(row)}
            >
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
      title="国标通道"
      subTitle="设备目录同步落库后的通道；区域与业务分组决定按区域/分组的目录推送范围"
    >
      <SearchBar
        fields={[
          { name: 'keyword', label: '关键字', placeholder: '通道名称/编码/设备编号' },
          { name: 'device_id', label: '所属设备', type: 'select', options: deviceOptions },
          { name: 'status', label: '在线状态', type: 'select', options: CHANNEL_STATUS_OPTIONS },
          { name: 'region_id', label: '所属区域', type: 'select', options: regionOptions },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<GbChannelItem>
        className="e2e-gb-channel-table"
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 'max-content' }}
        locale={{ emptyText: '暂无国标通道，请先在国标设备页执行「同步通道」' }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => { setPage(p); setPageSize(ps) },
        }}
      />

      {/* 编辑通道 */}
      <Modal
        className="e2e-gb-channel-edit-modal"
        title={`编辑通道${editing?.name ? ` - ${editing.name}` : ''}`}
        open={editOpen}
        onCancel={() => setEditOpen(false)}
        onOk={() => void formRef.current?.submit()}
        confirmLoading={saving}
        width={760}
        destroyOnHidden
      >
        <ChannelFormContent
          ref={formRef}
          editing={editing}
          recordPlanOptions={recordPlanOptions}
          onSubmit={handleSaveEdit}
        />
      </Modal>

      {/* 设置区域 */}
      <Modal
        className="e2e-gb-channel-region-modal"
        title={`设置区域${regionTarget?.name ? ` - ${regionTarget.name}` : ''}`}
        open={regionOpen}
        onCancel={() => setRegionOpen(false)}
        onOk={() => void handleSetRegion()}
        confirmLoading={setting}
        width={480}
        destroyOnHidden
      >
        <div style={{ marginBottom: 8 }}>所属区域</div>
        <RegionSelect
          style={{ width: '100%' }}
          value={regionValue}
          onChange={(value) => setRegionValue(value === undefined || value === null ? undefined : Number(value))}
        />
        <Typography.Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
          清空选择表示解除区域归属。
        </Typography.Text>
      </Modal>

      {/* 设置业务分组 */}
      <Modal
        className="e2e-gb-channel-group-modal"
        title={`设置业务分组${groupTarget?.name ? ` - ${groupTarget.name}` : ''}`}
        open={groupOpen}
        onCancel={() => setGroupOpen(false)}
        onOk={() => void handleSetGroup()}
        confirmLoading={setting}
        width={480}
        destroyOnHidden
      >
        <div style={{ marginBottom: 8 }}>业务分组</div>
        <Select
          allowClear
          showSearch
          optionFilterProp="label"
          style={{ width: '100%' }}
          placeholder="请选择业务分组"
          value={groupValue}
          onChange={(value?: number) => setGroupValue(value)}
          options={groupOptions}
        />
        <Typography.Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
          清空选择表示解除分组归属。
        </Typography.Text>
      </Modal>
    </PageContainer>
  )
}
