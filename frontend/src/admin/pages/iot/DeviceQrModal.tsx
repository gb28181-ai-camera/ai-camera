import { Alert, Button, Descriptions, Modal, Space, Typography } from 'antd'
import { CopyOutlined } from '@ant-design/icons'
import { QRCodeSVG } from 'qrcode.react'
import type { IotDeviceQR } from '../../api/generated/definitions/IotDeviceQR'
import type { IotDeviceShareToken } from '../../api/generated/definitions/IotDeviceShareToken'
import { CopyValue, copyText } from './shared'

/** 二维码预览块：白底 + 边框，避免暗色主题下扫码失败 */
function QrPreview({ value, title }: { value: string; title: string }) {
  return (
    <div
      style={{
        border: '1px solid #d9d9d9',
        borderRadius: 6,
        padding: 12,
        background: '#fff',
        display: 'inline-block',
      }}
    >
      <QRCodeSVG value={value} size={200} level="M" title={title} />
    </div>
  )
}

export interface DeviceQrModalProps {
  open: boolean
  onClose: () => void
  /** 设备名称或编号，用于标题 */
  deviceLabel?: string
  data: IotDeviceQR | null
}

/**
 * 设备二维码。
 *
 * `qr_content` 由后端拼成 `https://boticz.cn/qrDevice/<product>/<device>`，
 * 二维码本身**不含密钥**：App 扫码即视为持有设备，再用设备密钥 / 分享令牌完成绑定。
 */
export function DeviceQrModal({ open, onClose, deviceLabel, data }: DeviceQrModalProps) {
  return (
    <Modal
      className="e2e-iot-device-qr-modal"
      title={deviceLabel ? `设备二维码 - ${deviceLabel}` : '设备二维码'}
      open={open}
      onCancel={onClose}
      footer={<Button onClick={onClose}>关闭</Button>}
      width={560}
      destroyOnHidden
    >
      {data?.qr_content ? (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Alert
            type="warning"
            showIcon
            message="扫码即视为持有设备，请勿随意外发或公开张贴。"
          />
          <div style={{ textAlign: 'center' }}>
            <QrPreview value={data.qr_content} title="设备二维码" />
          </div>
          <Descriptions
            bordered
            size="small"
            column={1}
            items={[
              { key: 'product_code', label: '产品编码', children: <CopyValue value={data.product_code} /> },
              { key: 'device_code', label: '设备编码', children: <CopyValue value={data.device_code} /> },
            ]}
          />
          <Space wrap>
            <Button
              icon={<CopyOutlined />}
              onClick={() => void copyText(data.qr_content ?? '', '二维码内容已复制到剪贴板')}
            >
              复制二维码内容
            </Button>
          </Space>
          <Typography.Text type="secondary" style={{ wordBreak: 'break-all', fontFamily: 'monospace' }}>
            {data.qr_content}
          </Typography.Text>
        </Space>
      ) : (
        <Typography.Text type="secondary">后端未返回二维码内容，请检查设备是否已绑定产品。</Typography.Text>
      )}
    </Modal>
  )
}

export interface ShareQrModalProps {
  open: boolean
  onClose: () => void
  /** 设备名称或编号，用于标题 */
  deviceLabel?: string
  data: IotDeviceShareToken | null
}

/**
 * 设备一次性分享码。
 *
 * token 绑定一个尚未被认领的 shared 账号；App 认领后服务端清空 token，
 * 同一二维码不能再次使用，因此弹窗需要醒目标注「一次性」。
 */
export function ShareQrModal({ open, onClose, deviceLabel, data }: ShareQrModalProps) {
  return (
    <Modal
      className="e2e-iot-share-qr-modal"
      title={deviceLabel ? `分享码 - ${deviceLabel}` : '分享码'}
      open={open}
      onCancel={onClose}
      footer={<Button onClick={onClose}>关闭</Button>}
      width={560}
      destroyOnHidden
    >
      {data?.qr_content ? (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Alert
            type="error"
            showIcon
            message="一次性分享码：被认领后立即失效，同一二维码不可重复使用；请勿公开传播。"
          />
          <div style={{ textAlign: 'center' }}>
            <QrPreview value={data.qr_content} title="设备分享码" />
          </div>
          <Descriptions
            bordered
            size="small"
            column={1}
            items={[
              { key: 'token', label: '分享 Token', children: <CopyValue value={data.token} /> },
              { key: 'device_id', label: '设备 ID', children: <CopyValue value={data.device_id} /> },
              { key: 'account_id', label: '占位账号 ID', children: <CopyValue value={data.account_id} /> },
            ]}
          />
          <Space wrap>
            <Button
              icon={<CopyOutlined />}
              onClick={() => void copyText(data.token ?? '', '分享 Token 已复制到剪贴板')}
            >
              复制 Token
            </Button>
            <Button
              icon={<CopyOutlined />}
              onClick={() => void copyText(data.qr_content ?? '', '分享码内容已复制到剪贴板')}
            >
              复制二维码内容
            </Button>
          </Space>
          <Typography.Text type="secondary" style={{ wordBreak: 'break-all', fontFamily: 'monospace' }}>
            {data.qr_content}
          </Typography.Text>
        </Space>
      ) : (
        <Typography.Text type="secondary">暂未生成分享码。</Typography.Text>
      )}
    </Modal>
  )
}
