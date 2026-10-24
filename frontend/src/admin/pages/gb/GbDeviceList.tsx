import { useCallback, useEffect, useState } from 'react'
import {
  Button, Descriptions, Modal, Popconfirm, Select, Space, Table, Tag, Typography, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  BellOutlined, DeleteOutlined, InfoCircleOutlined, QrcodeOutlined, SyncOutlined,
} from '@ant-design/icons'
import axios from 'axios'
import { QRCodeSVG } from 'qrcode.react'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import StatusTag from '../../components/StatusTag'
import { GbDeviceController } from '../../api/generated/GbDeviceController'
import { RegionController } from '../../api/generated/RegionController'
import type { GbDeviceItem } from '../../api/generated/definitions/GbDeviceItem'
import type { GbDeviceDetailResp } from '../../api/generated/definitions/GbDeviceDetailResp'
import type { GbActivationResp } from '../../api/generated/definitions/GbActivationResp'
import { useAuthStore } from '../../store/authStore'
import { DEVICE_STATUS_OPTIONS } from '../../types/business'

/** 注册状态字典（后端：pending / activated / rejected） */
const REGISTER_STATUS_OPTIONS = [
  { label: '待激活', value: 'pending' },
  { label: '已激活', value: 'activated' },
  { label: '已拒绝', value: 'rejected' },
]

/** 订阅类型字典（SIP SUBSCRIBE 的 Event 包类型） */
const SUBSCRIBE_TYPE_OPTIONS = [
  { label: '目录订阅（catalog）', value: 'catalog' },
  { label: '告警订阅（alarm）', value: 'alarm' },
  { label: '移动位置订阅（mobile_position）', value: 'mobile_position' },
]

/** 注册状态 → 文案 / 颜色 */
const REGISTER_STATUS_META: Record<string, { text: string; color: string }> = {
  pending: { text: '待激活', color: 'orange' },
  activated: { text: '已激活', color: 'green' },
  rejected: { text: '已拒绝', color: 'red' },
}

/** 时间格式化：后端返回 ISO 字符串，此处仅展示到分钟 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/**
 * 请求失败兜底提示。
 * 请求拦截器（api/request.ts）已对 HTTP 错误弹出后端 message，
 * 这里只补网络异常 / 非 HTTP 错误的提示，避免同一条错误提示两次。
 */
function notifyError(err: unknown, fallback: string) {
  if (axios.isAxiosError(err) && err.response) return
  message.error(fallback)
}

/** 带复制按钮的只读值 */
function CopyValue({ value }: { value?: string | number | null }) {
  if (value === undefined || value === null || value === '') return <span>-</span>
  return (
    <Typography.Text copyable={{ text: String(value) }} style={{ wordBreak: 'break-all' }}>
      {String(value)}
    </Typography.Text>
  )
}

/**
 * 国标设备：展示已配置国标接入（gb_device_id 非空）的设备，
 * 提供目录同步、订阅、激活凭证生成与删除。
 */
export default function GbDeviceList() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canSync = permCodes.includes('gb:device:sync-channel')
  const canSubscribe = permCodes.includes('gb:device:subscribe')
  const canDelete = permCodes.includes('gb:device:delete')
  const canActivation = permCodes.includes('gb:device:activation')

  const [data, setData] = useState<GbDeviceItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [keyword, setKeyword] = useState('')
  const [registerStatus, setRegisterStatus] = useState<string | undefined>()
  const [status, setStatus] = useState<string | undefined>()
  const [regionID, setRegionID] = useState<number | undefined>()
  const [regionOptions, setRegionOptions] = useState<{ label: string; value: number }[]>([])

  // 详情弹窗
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detail, setDetail] = useState<GbDeviceDetailResp | null>(null)

  // 激活凭证弹窗
  const [activationOpen, setActivationOpen] = useState(false)
  const [activationLoading, setActivationLoading] = useState(false)
  const [activationDevice, setActivationDevice] = useState<GbDeviceItem | null>(null)
  const [activation, setActivation] = useState<GbActivationResp | null>(null)

  // 订阅弹窗
  const [subscribeOpen, setSubscribeOpen] = useState(false)
  const [subscribeDevice, setSubscribeDevice] = useState<GbDeviceItem | null>(null)
  const [subscribeType, setSubscribeType] = useState('catalog')
  const [subscribing, setSubscribing] = useState(false)

  const [syncingId, setSyncingId] = useState<number | null>(null)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbDeviceController.pageGbDevices(
        page, pageSize, keyword || undefined, registerStatus, status, regionID
      )
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch (err) {
      notifyError(err, '获取国标设备列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, registerStatus, status, regionID])

  useEffect(() => { void fetchData() }, [fetchData])

  // 区域下拉：筛选栏使用
  useEffect(() => {
    RegionController.options()
      .then((res) => {
        setRegionOptions(
          (res.data ?? [])
            .filter((item) => typeof item.id === 'number')
            .map((item) => ({ label: String(item.name ?? `区域 #${item.id}`), value: Number(item.id) }))
        )
      })
      .catch(() => setRegionOptions([]))
  }, [])

  const handleSearch = (values: Record<string, unknown>) => {
    setKeyword(typeof values.keyword === 'string' ? values.keyword : '')
    setRegisterStatus(typeof values.register_status === 'string' && values.register_status ? values.register_status : undefined)
    setStatus(typeof values.status === 'string' && values.status ? values.status : undefined)
    setRegionID(values.region_id === undefined || values.region_id === null || values.region_id === '' ? undefined : Number(values.region_id))
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setRegisterStatus(undefined)
    setStatus(undefined)
    setRegionID(undefined)
    setPage(1)
  }

  const openDetail = async (row: GbDeviceItem) => {
    if (!row.id) return
    setDetailOpen(true)
    setDetailLoading(true)
    setDetail(null)
    try {
      const res = await GbDeviceController.getGbDevice(row.id)
      if (res.data) {
        setDetail(res.data)
      } else {
        message.error(res.message || '获取设备详情失败')
        setDetailOpen(false)
      }
    } catch (err) {
      notifyError(err, '获取设备详情失败')
      setDetailOpen(false)
    } finally {
      setDetailLoading(false)
    }
  }

  const openActivation = async (row: GbDeviceItem) => {
    if (!row.id) return
    setActivationDevice(row)
    setActivationOpen(true)
    setActivationLoading(true)
    setActivation(null)
    try {
      const res = await GbDeviceController.generateGbActivation(row.id)
      if (res.data) {
        setActivation(res.data)
      } else {
        message.error(res.message || '生成激活凭证失败')
        setActivationOpen(false)
      }
    } catch (err) {
      notifyError(err, '生成激活凭证失败')
      setActivationOpen(false)
    } finally {
      setActivationLoading(false)
    }
  }

  const openSubscribe = (row: GbDeviceItem) => {
    setSubscribeDevice(row)
    setSubscribeType('catalog')
    setSubscribeOpen(true)
  }

  const handleSubscribe = async (action: 'subscribe' | 'unsubscribe') => {
    if (!subscribeDevice?.id) return
    setSubscribing(true)
    try {
      const res = action === 'subscribe'
        ? await GbDeviceController.subscribeGbDevice(subscribeDevice.id, { type: subscribeType })
        : await GbDeviceController.unsubscribeGbDevice(subscribeDevice.id, subscribeType)
      if (res.code === 200) {
        message.success(action === 'subscribe' ? '订阅指令已下发' : '取消订阅指令已下发')
        setSubscribeOpen(false)
      } else {
        message.error(res.message || '操作失败')
      }
    } catch (err) {
      notifyError(err, '操作失败')
    } finally {
      setSubscribing(false)
    }
  }

  const handleSync = async (row: GbDeviceItem) => {
    if (!row.id) return
    setSyncingId(row.id)
    try {
      const res = await GbDeviceController.syncGbDeviceChannel(row.id)
      if (res.code === 200) {
        message.success('目录同步指令已下发，设备目录将在收到响应后落库')
      } else {
        message.error(res.message || '同步失败')
      }
    } catch (err) {
      notifyError(err, '同步失败')
    } finally {
      setSyncingId(null)
    }
  }

  const handleDelete = async (row: GbDeviceItem) => {
    if (!row.id) return
    try {
      const res = await GbDeviceController.deleteGbDevice(row.id)
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

  const renderRegisterStatus = (value?: string) => {
    if (!value) return <span>-</span>
    const meta = REGISTER_STATUS_META[value] ?? { text: value, color: 'default' }
    return <Tag color={meta.color}>{meta.text}</Tag>
  }

  const columns: ColumnsType<GbDeviceItem> = [
    { title: '设备名称', dataIndex: 'name', width: 170, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '设备编号', dataIndex: 'device_code', width: 150, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '国标编码', dataIndex: 'gb_device_id', width: 190, ellipsis: true,
      render: (v?: string) => (v ? <Typography.Text copyable={{ text: v }}>{v}</Typography.Text> : '-'),
    },
    { title: '注册状态', dataIndex: 'register_status', width: 100, render: renderRegisterStatus },
    {
      title: '在线状态', dataIndex: 'status', width: 100,
      render: (v?: string) => <StatusTag type="device" status={v} />,
    },
    { title: '通道数', dataIndex: 'channel_count', width: 80, render: (v?: number) => (v === undefined ? '-' : v) },
    { title: '所属区域', dataIndex: 'region_name', width: 140, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '最近注册时间', dataIndex: 'register_dt', width: 170, render: (v?: string) => fmtDT(v) },
    {
      title: '操作',
      key: 'action',
      width: 360,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={340}>
          <Button type="link" size="small" icon={<InfoCircleOutlined />} onClick={() => void openDetail(row)}>
            详情
          </Button>
          {canSync && (
            <Button
              type="link"
              size="small"
              icon={<SyncOutlined />}
              loading={syncingId === row.id}
              onClick={() => void handleSync(row)}
            >
              同步通道
            </Button>
          )}
          {canSubscribe && (
            <Button type="link" size="small" icon={<BellOutlined />} onClick={() => openSubscribe(row)}>
              订阅/取消订阅
            </Button>
          )}
          {canActivation && (
            <Button type="link" size="small" icon={<QrcodeOutlined />} onClick={() => void openActivation(row)}>
              生成激活凭证
            </Button>
          )}
          {canDelete && (
            <Popconfirm
              title="确定删除该设备?"
              description="删除后其全部国标通道也会被一并删除。"
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
      title="国标设备"
      subTitle="通过 GB/T 28181 接入的设备；国标编码与 SIP 密码由平台生成并下发到设备"
    >
      <SearchBar
        fields={[
          { name: 'keyword', label: '关键字', placeholder: '设备名称/编号/国标编码' },
          { name: 'register_status', label: '注册状态', type: 'select', options: REGISTER_STATUS_OPTIONS },
          { name: 'status', label: '在线状态', type: 'select', options: DEVICE_STATUS_OPTIONS },
          { name: 'region_id', label: '所属区域', type: 'select', options: regionOptions },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<GbDeviceItem>
        className="e2e-gb-device-table"
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 'max-content' }}
        locale={{ emptyText: '暂无国标设备' }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => { setPage(p); setPageSize(ps) },
        }}
      />

      {/* 设备详情：13 个国标接入字段（sip_password 后端已脱敏） */}
      <Modal
        className="e2e-gb-device-detail-modal"
        title={`设备详情${detail?.name ? ` - ${detail.name}` : ''}`}
        open={detailOpen}
        loading={detailLoading}
        onCancel={() => setDetailOpen(false)}
        footer={<Button onClick={() => setDetailOpen(false)}>关闭</Button>}
        width={760}
        destroyOnHidden
      >
        <Descriptions
          bordered
          size="small"
          column={2}
          items={
            detail
              ? [
                  { key: 'name', label: '设备名称', children: detail.name || '-' },
                  { key: 'code', label: '设备编号', children: detail.device_code || '-' },
                  { key: 'gb_id', label: '国标编码', children: <CopyValue value={detail.gb_device_id} /> },
                  {
                    key: 'register_status',
                    label: '注册状态',
                    children: renderRegisterStatus(detail.register_status),
                  },
                  { key: 'status', label: '在线状态', children: <StatusTag type="device" status={detail.status} /> },
                  { key: 'region', label: '所属区域', children: detail.region_name || detail.region?.name || '-' },
                  { key: 'sip_password', label: 'SIP 密码', children: <CopyValue value={detail.sip_password} /> },
                  { key: 'sip_realm', label: 'SIP 域（realm）', children: detail.sip_realm || '-' },
                  { key: 'sip_transport', label: '传输协议', children: detail.sip_transport || '-' },
                  { key: 'sip_expires', label: '注册有效期', children: detail.sip_expires || '-' },
                  {
                    key: 'media_server',
                    label: '媒体节点',
                    children: detail.media_server_name || detail.media_server_id || '-',
                  },
                  { key: 'stream_mode', label: '流模式', children: detail.stream_mode || '-' },
                  { key: 'local_ip', label: '本地 IP', children: detail.local_ip || '-' },
                  { key: 'sdp_ip', label: 'SDP IP', children: detail.sdp_ip || '-' },
                  { key: 'charset', label: '字符集', children: detail.charset || '-' },
                  { key: 'location', label: '安装位置', children: detail.location || '-' },
                  { key: 'longitude', label: '经度', children: detail.longitude || '-' },
                  { key: 'latitude', label: '纬度', children: detail.latitude || '-' },
                  { key: 'register_dt', label: '最近注册时间', children: fmtDT(detail.register_dt) },
                  { key: 'updated_dt', label: '更新时间', children: fmtDT(detail.updated_dt) },
                  { key: 'remark', label: '备注', children: detail.remark || '-', span: 2 },
                ]
              : []
          }
        />
      </Modal>

      {/* 激活凭证：国标编码 / SIP 密码 / 域名 / 服务器地址 + 二维码内容 */}
      <Modal
        className="e2e-gb-device-activation-modal"
        title={`激活凭证${activationDevice?.name ? ` - ${activationDevice.name}` : ''}`}
        open={activationOpen}
        loading={activationLoading}
        onCancel={() => setActivationOpen(false)}
        footer={<Button onClick={() => setActivationOpen(false)}>关闭</Button>}
        width={680}
        destroyOnHidden
      >
        {activation ? (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <Descriptions
              bordered
              size="small"
              column={1}
              items={[
                { key: 'gb_id', label: '国标编码', children: <CopyValue value={activation.gb_device_id} /> },
                { key: 'sip_password', label: 'SIP 密码', children: <CopyValue value={activation.sip_password} /> },
                { key: 'sip_domain', label: 'SIP 域名', children: <CopyValue value={activation.sip_domain} /> },
                {
                  key: 'sip_server',
                  label: '服务器地址',
                  children: <CopyValue value={activation.sip_server} />,
                },
                { key: 'sip_port', label: 'SIP 端口', children: <CopyValue value={activation.sip_port} /> },
                { key: 'transport', label: '传输协议', children: activation.transport || '-' },
                {
                  key: 'register_status',
                  label: '当前注册状态',
                  children: renderRegisterStatus(activation.register_status),
                },
              ]}
            />
            <div>
              <div style={{ marginBottom: 8, fontWeight: 500 }}>激活二维码</div>
              {activation.qr_content ? (
                // qr_content 是后端拼好的 JSON 字符串，扫码端按约定解析；
                // 二维码与文本同时展示，扫码失败时仍可复制内容手工导入。
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <div
                    style={{
                      border: '1px solid #d9d9d9',
                      borderRadius: 6,
                      padding: 12,
                      background: '#fff',
                      display: 'inline-block',
                    }}
                  >
                    <QRCodeSVG
                      value={activation.qr_content}
                      size={180}
                      level="M"
                      title="设备激活二维码"
                    />
                  </div>
                  <div
                    style={{
                      border: '1px solid #d9d9d9',
                      borderRadius: 6,
                      padding: '8px 12px',
                      background: '#fafafa',
                    }}
                  >
                    <Typography.Paragraph
                      copyable={{ text: activation.qr_content }}
                      style={{ marginBottom: 0, fontFamily: 'monospace', wordBreak: 'break-all' }}
                    >
                      {activation.qr_content}
                    </Typography.Paragraph>
                  </div>
                </Space>
              ) : (
                <Typography.Text type="secondary">后端未返回二维码内容，请确认设备已生成激活凭证。</Typography.Text>
              )}
              <Typography.Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
                使用摄像头 App / 扫码工具扫描二维码即可导入国标接入信息；二维码内容为 JSON 字符串。
              </Typography.Text>
            </div>
          </Space>
        ) : null}
      </Modal>

      {/* 订阅 / 取消订阅 */}
      <Modal
        className="e2e-gb-device-subscribe-modal"
        title={`订阅管理${subscribeDevice?.name ? ` - ${subscribeDevice.name}` : ''}`}
        open={subscribeOpen}
        onCancel={() => setSubscribeOpen(false)}
        footer={(
          <Space>
            <Button onClick={() => setSubscribeOpen(false)}>取消</Button>
            <Button danger loading={subscribing} onClick={() => void handleSubscribe('unsubscribe')}>
              取消订阅
            </Button>
            <Button type="primary" loading={subscribing} onClick={() => void handleSubscribe('subscribe')}>
              订阅
            </Button>
          </Space>
        )}
        width={480}
        destroyOnHidden
      >
        <div style={{ marginBottom: 8 }}>订阅类型</div>
        <Select
          style={{ width: '100%' }}
          value={subscribeType}
          onChange={(value) => setSubscribeType(value)}
          options={SUBSCRIBE_TYPE_OPTIONS}
        />
        <Typography.Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
          订阅通过 SIP SUBSCRIBE 下发；SIP 信令服务未启动时接口会返回 503。
        </Typography.Text>
      </Modal>
    </PageContainer>
  )
}
