import dayjs from 'dayjs'

/** 空值占位 */
export const EMPTY_TEXT = '-'

/**
 * 日期时间格式化。传入空值时返回 undefined（antd Table 直接渲染为空）。
 *
 * @example formatDateTime('2026-09-21T10:30:00Z') // '2026-09-21 10:30:00'
 */
export function formatDateTime(value?: string | number | Date | null, template = 'YYYY-MM-DD HH:mm:ss'): string | undefined {
  if (value === null || value === undefined || value === '') return undefined
  const d = dayjs(value)
  return d.isValid() ? d.format(template) : undefined
}

/** 仅日期，如 2026-09-21 */
export function formatDate(value?: string | number | Date | null): string | undefined {
  return formatDateTime(value, 'YYYY-MM-DD')
}

/** 仅时间，如 10:30:00 */
export function formatTime(value?: string | number | Date | null): string | undefined {
  return formatDateTime(value, 'HH:mm:ss')
}

/**
 * 文件大小格式化，保留 2 位小数。
 *
 * @example formatFileSize(1536)     // '1.5 KB'
 * @example formatFileSize(12345678) // '11.77 MB'
 */
export function formatFileSize(bytes?: number | null, decimals = 2): string {
  if (bytes === null || bytes === undefined || Number.isNaN(bytes)) return EMPTY_TEXT
  if (bytes === 0) return '0 B'
  if (bytes < 0) return EMPTY_TEXT

  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const base = 1024
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(base)), units.length - 1)
  const value = bytes / Math.pow(base, index)
  // B 不显示小数
  const fixed = index === 0 ? String(value) : value.toFixed(decimals)
  return `${trimZero(fixed)} ${units[index]}`
}

/** 去掉小数末尾多余的 0，如 '1.50' → '1.5'、'2.00' → '2' */
function trimZero(value: string): string {
  return value.includes('.') ? value.replace(/\.?0+$/, '') : value
}

/**
 * 物联网卡流量格式化：后端以 MB 存储，自动升位到 GB / TB。
 *
 * @example formatTraffic(512)    // '512 MB'
 * @example formatTraffic(2048)   // '2 GB'
 */
export function formatTraffic(mb?: number | null, decimals = 2): string {
  if (mb === null || mb === undefined || Number.isNaN(mb)) return EMPTY_TEXT
  if (mb < 0) return EMPTY_TEXT
  if (mb < 1024) return `${trimZero(mb.toFixed(decimals))} MB`

  const gb = mb / 1024
  if (gb < 1024) return `${trimZero(gb.toFixed(decimals))} GB`

  const tb = gb / 1024
  return `${trimZero(tb.toFixed(decimals))} TB`
}

/**
 * 时长格式化（秒 → `1天2小时3分4秒`）。
 *
 * @example formatDuration(90061) // '1天1小时1分1秒'
 */
export function formatDuration(seconds?: number | null): string {
  if (seconds === null || seconds === undefined || Number.isNaN(seconds) || seconds < 0) return EMPTY_TEXT
  const s = Math.floor(seconds)
  const day = Math.floor(s / 86400)
  const hour = Math.floor((s % 86400) / 3600)
  const minute = Math.floor((s % 3600) / 60)
  const second = s % 60

  const parts: string[] = []
  if (day > 0) parts.push(`${day}天`)
  if (hour > 0) parts.push(`${hour}小时`)
  if (minute > 0) parts.push(`${minute}分`)
  if (second > 0 || parts.length === 0) parts.push(`${second}秒`)
  return parts.join('')
}

/**
 * 百分比格式化，入参为 0~1 的小数。
 *
 * @example formatPercent(0.1234) // '12.34%'
 */
export function formatPercent(ratio?: number | null, decimals = 2): string {
  if (ratio === null || ratio === undefined || Number.isNaN(ratio)) return EMPTY_TEXT
  return `${(ratio * 100).toFixed(decimals)}%`
}

/** 数字千分位，如 1234567 → '1,234,567' */
export function formatNumber(value?: number | null): string {
  if (value === null || value === undefined || Number.isNaN(value)) return EMPTY_TEXT
  return value.toLocaleString('zh-CN')
}

/** 空文本兜底：null / undefined / '' → '-' */
export function emptyText(value?: string | number | null): string | number {
  if (value === null || value === undefined || value === '') return EMPTY_TEXT
  return value
}
