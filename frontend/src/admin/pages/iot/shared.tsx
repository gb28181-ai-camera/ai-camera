import type { ReactNode } from 'react'
import { Alert, Button, Descriptions, Modal, Typography, message } from 'antd'
import axios from 'axios'

/** 时间格式化：后端返回 ISO 字符串，此处仅展示到分钟（与 RegionList 保持一致） */
export function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/**
 * 请求失败兜底提示。
 * HTTP 错误已由 request 拦截器原样展示后端 message，这里只兜底非 HTTP 异常，避免重复提示。
 */
export function notifyError(err: unknown, fallback: string): void {
  if (axios.isAxiosError(err) && err.response) return
  message.error(fallback)
}

/**
 * 复制文本到剪贴板。
 *
 * `navigator.clipboard` 只在安全上下文（HTTPS / localhost）可用，
 * 内网 HTTP 部署或浏览器拒绝授权时会抛错，此时明确提示用户手动复制，
 * 而不是静默失败。
 */
export async function copyText(text: string, successMsg = '已复制到剪贴板'): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    message.success(successMsg)
    return true
  } catch {
    message.error('当前环境不支持自动复制，请手动选中内容复制')
    return false
  }
}

/** 带复制按钮的只读值 */
export function CopyValue({ value }: { value?: string | number | null }) {
  if (value === undefined || value === null || value === '') return <span>-</span>
  return (
    <Typography.Text copyable={{ text: String(value) }} style={{ wordBreak: 'break-all' }}>
      {String(value)}
    </Typography.Text>
  )
}

/** 一次性敏感信息条目 */
export interface SecretItem {
  label: string
  value?: string | number | null
  /** 是否提供复制按钮，默认 true */
  copy?: boolean
}

export interface SecretModalProps {
  open: boolean
  title: string
  /** E2E 定位类名，如 e2e-iot-product-share-modal */
  className?: string
  items: SecretItem[]
  /** 顶部警告文案，默认「关闭后无法再次查看」 */
  warning?: string
  extra?: ReactNode
  onClose: () => void
}

/**
 * 一次性敏感信息弹窗。
 *
 * 服务组凭据 / 设备主账号密码 / 设备接入密钥等只会在生成时返回一次，
 * 因此统一用醒目的警告条提示「关闭后无法再次查看」。
 */
export function SecretModal({ open, title, className, items, warning, extra, onClose }: SecretModalProps) {
  return (
    <Modal
      className={className}
      title={title}
      open={open}
      onCancel={onClose}
      footer={<Button onClick={onClose}>关闭</Button>}
      width={560}
      destroyOnHidden
    >
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        message={warning ?? '关闭后无法再次查看，请立即复制并妥善保存'}
      />
      <Descriptions
        bordered
        column={1}
        size="small"
        items={items.map((item, index) => ({
          key: `${item.label}-${index}`,
          label: item.label,
          children: item.copy === false ? (item.value ?? '-') : <CopyValue value={item.value} />,
        }))}
      />
      {extra}
    </Modal>
  )
}
