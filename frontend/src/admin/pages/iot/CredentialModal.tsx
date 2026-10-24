import { useEffect, useMemo, useState } from 'react'
import { Alert, Button, Descriptions, Modal, Space, Table, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { CopyOutlined, EyeInvisibleOutlined, EyeOutlined } from '@ant-design/icons'
import type { IotCredentials } from '../../api/generated/definitions/IotCredentials'
import type { IotCredentialsGroup } from '../../api/generated/definitions/IotCredentialsGroup'
import type { IotServiceItem } from '../../api/generated/definitions/IotServiceItem'
import { CopyValue, copyText } from './shared'

/** 服务组类型文案（与列表页保持一致） */
const TYPE_LABEL: Record<string, string> = {
  gb28181: 'GB28181',
  webrtc: 'WebRTC',
  aichat: 'AI聊天',
}

/** 协议默认端口；服务项 port=0 时按协议取默认值（后端约定） */
const PROTOCOL_DEFAULT_PORT: Record<string, number> = {
  SIP: 5060,
  RTSP: 554,
  WEBRTC: 443,
  MQTT: 1883,
  WEBSOCKET: 443,
}

/** 取服务项的有效端口：显式端口优先，0/空按协议默认值 */
function resolvePort(item: IotServiceItem): number {
  if (item.port != null && item.port > 0) return item.port
  return PROTOCOL_DEFAULT_PORT[(item.protocol ?? '').toUpperCase()] ?? 0
}

/** 文本清单中的一行接入地址：`协议 地址:端口[路径]` */
function endpointLine(item: IotServiceItem): string {
  const protocol = item.protocol || '-'
  const host = (item.host ?? '').trim()
  if (!host) return `${protocol} （未配置服务地址）`
  const port = resolvePort(item)
  const rawPath = (item.path ?? '').trim()
  const path = rawPath ? (rawPath.startsWith('/') ? rawPath : `/${rawPath}`) : ''
  return `${protocol} ${host}${port > 0 ? `:${port}` : ''}${path}`
}

/**
 * 生成发给设备厂商的纯文本鉴权清单。
 *
 * 与「复制 JSON」互补：JSON 便于程序解析，文本便于直接粘贴到工单 / 聊天窗口。
 * 产品维度会列出全部服务组；服务组维度只有一组（不含 product 字段）。
 */
export function buildCredentialText(data: IotCredentials, fallbackTitle?: string): string {
  const lines: string[] = []
  const productName = data.product?.name || fallbackTitle
  const productCode = data.product?.code
  if (productName || productCode) {
    lines.push(`产品：${productName ?? '-'}${productCode ? `（${productCode}）` : ''}`)
    lines.push('')
  }

  for (const group of data.groups ?? []) {
    lines.push(`服务组：${group.name ?? '-'}（${group.code ?? '-'}） 类型：${TYPE_LABEL[group.type ?? ''] ?? group.type ?? '-'}`)
    lines.push(`用户名：${group.username ?? '-'}`)
    lines.push(`密码：${group.password ?? '-'}`)
    for (const item of group.items ?? []) lines.push(endpointLine(item))
    lines.push('')
  }

  if (data.mqtt?.url) lines.push(`MQTT URL：${data.mqtt.url}`)
  if (data.mqtt?.topic_prefix) lines.push(`MQTT Topic 前缀：${data.mqtt.topic_prefix}`)
  for (const url of data.turn?.urls ?? []) lines.push(`TURN URL：${url}`)
  if (data.turn?.realm) lines.push(`TURN Realm：${data.turn.realm}`)

  return lines.join('\n').trim()
}

export interface CredentialModalProps {
  open: boolean
  title: string
  data: IotCredentials | null
  onClose: () => void
}

/**
 * 明文服务鉴权信息弹窗（复制用）。
 *
 * 产品管理展示产品下全部服务组，服务组页展示单个服务组；两者共用同一视图，
 * 密码默认可见以便直接核对 / 复制，同时提供「显示/隐藏」切换防止误拍屏。
 */
export function CredentialModal({ open, title, data, onClose }: CredentialModalProps) {
  const [showPassword, setShowPassword] = useState(true)

  // 每次重新打开都回到默认可见，避免上一次的隐藏状态让用户以为没有密码。
  useEffect(() => {
    if (open) setShowPassword(true)
  }, [open])

  const text = useMemo(() => (data ? buildCredentialText(data) : ''), [data])
  const json = useMemo(() => (data ? JSON.stringify(data, null, 2) : ''), [data])

  const itemColumns: ColumnsType<IotServiceItem> = [
    {
      title: '协议', dataIndex: 'protocol', width: 100,
      render: (v?: string) => (v ? <Tag color="geekblue">{v}</Tag> : '-'),
    },
    { title: '地址', dataIndex: 'host', ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '端口', dataIndex: 'port', width: 90,
      render: (_, item) => {
        const port = resolvePort(item)
        return port > 0 ? port : '-'
      },
    },
    { title: '路径', dataIndex: 'path', ellipsis: true, render: (v?: string) => v || '-' },
  ]

  const renderGroup = (group: IotCredentialsGroup, index: number) => (
    <div key={`${group.code ?? group.name ?? 'group'}-${index}`} style={{ marginBottom: 20 }}>
      <Space size={8} style={{ marginBottom: 8 }}>
        <Tag color="geekblue">{TYPE_LABEL[group.type ?? ''] ?? group.type ?? '-'}</Tag>
        <Typography.Text strong>{group.name || '-'}</Typography.Text>
        <Typography.Text type="secondary">{group.code || '-'}</Typography.Text>
      </Space>
      <Descriptions
        bordered
        size="small"
        column={1}
        items={[
          { key: 'username', label: '用户名', children: <CopyValue value={group.username} /> },
          {
            key: 'password',
            label: '密码',
            children: (
              <Typography.Text
                style={{ fontFamily: 'monospace', wordBreak: 'break-all' }}
                copyable={showPassword && group.password ? { text: group.password } : false}
              >
                {showPassword ? (group.password || '-') : '••••••••'}
              </Typography.Text>
            ),
          },
        ]}
      />
      <Table<IotServiceItem>
        style={{ marginTop: 8 }}
        rowKey={(item, i) => String(item.id ?? i)}
        size="small"
        bordered
        columns={itemColumns}
        dataSource={group.items ?? []}
        pagination={false}
        locale={{ emptyText: '该服务组未配置服务项' }}
      />
    </div>
  )

  const mqtt = data?.mqtt
  const turn = data?.turn
  const hasMqttOrTurn = Boolean(mqtt?.url || mqtt?.topic_prefix || turn?.urls?.length || turn?.realm)

  return (
    <Modal
      className="e2e-iot-credentials-modal"
      title={title}
      open={open}
      onCancel={onClose}
      footer={<Button onClick={onClose}>关闭</Button>}
      width={860}
      destroyOnHidden
    >
      {data ? (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Alert
            type="warning"
            showIcon
            message="以下为明文鉴权信息，请仅发送给可信的设备厂商或运维人员，避免外泄。"
          />
          <Space wrap>
            <Button
              type="primary"
              icon={<CopyOutlined />}
              onClick={() => void copyText(json, 'JSON 已复制到剪贴板')}
            >
              复制 JSON
            </Button>
            <Button
              icon={<CopyOutlined />}
              onClick={() => void copyText(text, '鉴权文本已复制到剪贴板')}
            >
              复制文本
            </Button>
            <Button
              icon={showPassword ? <EyeInvisibleOutlined /> : <EyeOutlined />}
              onClick={() => setShowPassword((prev) => !prev)}
            >
              {showPassword ? '隐藏密码' : '显示密码'}
            </Button>
          </Space>

          {data.product?.name || data.product?.code ? (
            <Descriptions
              bordered
              size="small"
              column={2}
              items={[
                { key: 'product_name', label: '产品名称', children: data.product?.name || '-' },
                { key: 'product_code', label: '产品编码', children: data.product?.code || '-' },
              ]}
            />
          ) : null}

          {(data.groups ?? []).map(renderGroup)}
          {(data.groups ?? []).length === 0 ? (
            <Typography.Text type="secondary">该产品尚未绑定服务组，暂无服务组鉴权信息。</Typography.Text>
          ) : null}

          {hasMqttOrTurn ? (
            <Descriptions
              bordered
              size="small"
              column={1}
              title="MQTT / TURN"
              items={[
                ...(mqtt?.url
                  ? [{ key: 'mqtt_url', label: 'MQTT URL', children: <CopyValue value={mqtt.url} /> }]
                  : []),
                ...(mqtt?.topic_prefix
                  ? [{ key: 'mqtt_topic', label: 'MQTT Topic 前缀', children: <CopyValue value={mqtt.topic_prefix} /> }]
                  : []),
                ...((turn?.urls ?? []).map((url, index) => ({
                  key: `turn-${index}`,
                  label: `TURN URL ${index + 1}`,
                  children: <CopyValue value={url} />,
                }))),
                ...(turn?.realm
                  ? [{ key: 'turn_realm', label: 'TURN Realm', children: <CopyValue value={turn.realm} /> }]
                  : []),
              ]}
            />
          ) : null}
          <Typography.Text type="secondary">
            「复制文本」按「协议 地址:端口」逐行输出，可直接发给设备厂商；port 为 0 的服务项已按协议默认端口展开。
          </Typography.Text>
        </Space>
      ) : (
        <Typography.Text type="secondary">暂无可用的鉴权信息。</Typography.Text>
      )}
    </Modal>
  )
}
