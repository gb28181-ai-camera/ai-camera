import { useCallback, useEffect, useState } from 'react'
import { Alert, Button, Descriptions, Modal, Space, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { QrcodeOutlined, ReloadOutlined } from '@ant-design/icons'
import axios from 'axios'
import { QRCodeSVG } from 'qrcode.react'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import StatusTag from '../../components/StatusTag'
import { GbDeviceController } from '../../api/generated/GbDeviceController'
import type { GbDeviceItem } from '../../api/generated/definitions/GbDeviceItem'
import type { GbActivationResp } from '../../api/generated/definitions/GbActivationResp'
import { useAuthStore } from '../../store/authStore'

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
 * 设备激活：集中处理注册状态为 pending 的设备。
 *
 * 流程：平台建档 → 生成国标编码与 SIP 密码 → 设备发起 REGISTER → 状态变 activated。
 */
export default function DeviceActivationList() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canActivation = permCodes.includes('gb:device:activation')

  const [data, setData] = useState<GbDeviceItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')

  const [activationOpen, setActivationOpen] = useState(false)
  const [activationLoading, setActivationLoading] = useState(false)
  const [activationDevice, setActivationDevice] = useState<GbDeviceItem | null>(null)
  const [activation, setActivation] = useState<GbActivationResp | null>(null)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      // 只列出待激活设备；页面权限为 gb:activation:list
      const res = await GbDeviceController.pageGbDevices(
        page, pageSize, keyword || undefined, 'pending'
      )
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch (err) {
      notifyError(err, '获取待激活设备失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword])

  useEffect(() => { void fetchData() }, [fetchData])

  const handleSearch = (values: Record<string, unknown>) => {
    setKeyword(typeof values.keyword === 'string' ? values.keyword : '')
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setPage(1)
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

  const columns: ColumnsType<GbDeviceItem> = [
    { title: '设备名称', dataIndex: 'name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '设备编号', dataIndex: 'device_code', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '国标编码', dataIndex: 'gb_device_id', width: 190, ellipsis: true,
      render: (v?: string) => (v ? <Typography.Text copyable={{ text: v }}>{v}</Typography.Text> : <Tag>待生成</Tag>),
    },
    { title: '所属区域', dataIndex: 'region_name', width: 150, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '在线状态', dataIndex: 'status', width: 100,
      render: (v?: string) => <StatusTag type="device" status={v} />,
    },
    { title: '最近注册时间', dataIndex: 'register_dt', width: 170, render: (v?: string) => fmtDT(v) },
    {
      title: '操作',
      key: 'action',
      width: 160,
      fixed: 'right',
      render: (_, row) => (
        <Button
          type="link"
          size="small"
          icon={<QrcodeOutlined />}
          disabled={!canActivation}
          onClick={() => void openActivation(row)}
        >
          生成凭证
        </Button>
      ),
    },
  ]

  return (
    <PageContainer
      title="设备激活"
      subTitle="为待激活设备生成并下发国标接入凭证"
      extra={(
        <Button icon={<ReloadOutlined />} onClick={() => void fetchData()} loading={loading}>
          刷新
        </Button>
      )}
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="设备激活流程"
        description={(
          <ol style={{ margin: 0, paddingLeft: 20, lineHeight: 1.9 }}>
            <li>平台建档：在国标设备页面维护设备基础信息（设备名称、编号、所属区域等）。</li>
            <li>生成凭证：点击「生成凭证」，平台生成（或返回已有的）国标编码与 SIP 密码。</li>
            <li>设备注册：将国标编码、SIP 密码、SIP 域名与服务器地址配置到设备，设备向平台发送 REGISTER。</li>
            <li>激活完成：注册成功后设备注册状态自动变为「已激活」，可从国标设备页进行目录同步与订阅。</li>
          </ol>
        )}
      />

      <SearchBar
        fields={[{ name: 'keyword', label: '关键字', placeholder: '设备名称/编号/国标编码' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<GbDeviceItem>
        className="e2e-gb-activation-table"
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 'max-content' }}
        locale={{ emptyText: '暂无待激活设备（注册状态为 pending）' }}
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
        className="e2e-gb-activation-modal"
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
                { key: 'device', label: '设备名称', children: activationDevice?.name || '-' },
                { key: 'gb_id', label: '国标编码', children: <CopyValue value={activation.gb_device_id} /> },
                { key: 'sip_password', label: 'SIP 密码', children: <CopyValue value={activation.sip_password} /> },
                { key: 'sip_domain', label: 'SIP 域名', children: <CopyValue value={activation.sip_domain} /> },
                { key: 'sip_server', label: '服务器地址', children: <CopyValue value={activation.sip_server} /> },
                { key: 'sip_port', label: 'SIP 端口', children: <CopyValue value={activation.sip_port} /> },
                { key: 'transport', label: '传输协议', children: activation.transport || '-' },
                {
                  key: 'register_status',
                  label: '注册状态',
                  children: activation.register_status === 'activated'
                    ? <Tag color="green">已激活</Tag>
                    : <Tag color="orange">待激活</Tag>,
                },
              ]}
            />
            <div>
              <div style={{ marginBottom: 8, fontWeight: 500 }}>激活二维码</div>
              {activation.qr_content ? (
                // 与另外两个激活入口保持一致：二维码 + 文本双展示。
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
                <Typography.Text type="secondary">后端未返回二维码内容，请重新生成激活凭证。</Typography.Text>
              )}
            </div>
          </Space>
        ) : null}
      </Modal>
    </PageContainer>
  )
}
