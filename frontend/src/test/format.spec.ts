import { describe, expect, it } from 'vitest'
import {
  formatDate,
  formatDateTime,
  formatDuration,
  formatFileSize,
  formatTraffic,
} from '../shared/utils/format'

describe('shared/utils/format', () => {
  it('formatDateTime 支持空值兜底与自定义模板', () => {
    expect(formatDateTime('2026-09-21 10:30:00')).toBe('2026-09-21 10:30:00')
    expect(formatDateTime(null)).toBeUndefined()
    expect(formatDateTime('')).toBeUndefined()
    expect(formatDateTime('not-a-date')).toBeUndefined()
  })

  it('formatDate 仅保留日期', () => {
    expect(formatDate('2026-09-21 10:30:00')).toBe('2026-09-21')
  })

  it('formatFileSize 按 1024 进制升位并去掉多余小数', () => {
    expect(formatFileSize(0)).toBe('0 B')
    expect(formatFileSize(512)).toBe('512 B')
    expect(formatFileSize(1536)).toBe('1.5 KB')
    expect(formatFileSize(12345678)).toBe('11.77 MB')
    expect(formatFileSize(null)).toBe('-')
  })

  it('formatTraffic 以 MB 为基准升位到 GB / TB', () => {
    expect(formatTraffic(512)).toBe('512 MB')
    expect(formatTraffic(2048)).toBe('2 GB')
    expect(formatTraffic(2 * 1024 * 1024)).toBe('2 TB')
    expect(formatTraffic(undefined)).toBe('-')
  })

  it('formatDuration 输出天/时/分/秒', () => {
    expect(formatDuration(90061)).toBe('1天1小时1分1秒')
    expect(formatDuration(0)).toBe('0秒')
    expect(formatDuration(-1)).toBe('-')
  })
})
