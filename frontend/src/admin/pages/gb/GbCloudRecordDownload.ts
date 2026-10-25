import { GbCloudRecordController } from '../../api/generated/GbCloudRecordController'
import { GbCloudMergeController } from '../../api/generated/GbCloudMergeController'

/**
 * 云端录像下载（单条直链 / 多选流式 zip）。
 *
 * 后端 `GET /web/gb/cloud-records/{id}/download` 的行为：
 * - `ids` 省略或只有 1 条 → 返回 JSON `{ url, packaging, ... }`；
 * - `ids` 多于 1 条 → 直接流式返回 zip，并通过 `X-Skipped` 告知预检失败的文件。
 *
 * 生成的 Controller 统一按 JSON 拆包，无法承载二进制流，因此多选场景复用
 * Controller 暴露的**路径常量**（不硬编码接口）构造地址，再手动携带 JWT 头下载。
 */

/** 解析 Content-Disposition 中的文件名。 */
function resolveFileName(contentDisposition: string | null): string | undefined {
  if (!contentDisposition) return undefined
  const match = /filename\*?=(?:UTF-8''|")?([^";]+)/i.exec(contentDisposition)
  const raw = match?.[1]?.trim()
  if (!raw) return undefined
  try {
    return decodeURIComponent(raw.replace(/^"|"$/g, ''))
  } catch {
    return raw.replace(/^"|"$/g, '')
  }
}

/** 带鉴权头下载二进制流并保存（生成器按 JSON 拆包，无法承载文件流）。 */
async function downloadBinary(path: string, fallbackName: string): Promise<Headers> {
  const base = import.meta.env.VITE_API_BASE || ''
  const token = localStorage.getItem('token')
  const response = await fetch(`${base}${path}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    cache: 'no-store',
  })

  if (!response.ok) {
    let reason = `下载失败（HTTP ${response.status}）`
    try {
      const body = (await response.json()) as { message?: string }
      if (body?.message) reason = body.message
    } catch {
      // 非 JSON 错误响应，沿用状态码提示
    }
    throw new Error(reason)
  }

  const blob = await response.blob()
  saveBlob(blob, resolveFileName(response.headers.get('Content-Disposition')) || fallbackName)
  return response.headers
}

/** 解析 `X-Skipped`（逗号分隔、URL 编码的文件名列表）。 */
function parseSkippedHeader(value: string | null): string[] {
  if (!value) return []
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => {
      try {
        return decodeURIComponent(item)
      } catch {
        return item
      }
    })
}

/** 触发浏览器保存 Blob。 */
function saveBlob(blob: Blob, fileName: string): void {
  const objectUrl = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = objectUrl
  link.download = fileName
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(objectUrl)
}

export interface CloudRecordDownloadResult {
  /** 预检失败被跳过的文件名（仅多选打包时有值） */
  skipped: string[]
}

/**
 * 下载一组云端录像。
 *
 * 单条：走 Controller 的 JSON 直链并在新标签页打开（与原有行为一致）。
 * 多条：请求后端流式 zip 并保存到本地。
 */
export async function downloadCloudRecords(ids: number[]): Promise<CloudRecordDownloadResult> {
  if (ids.length === 0) return { skipped: [] }

  if (ids.length === 1) {
    const res = await GbCloudRecordController.getGbCloudRecordDownload(ids[0])
    const url = res.data?.url
    if (!url) throw new Error(res.data?.message || '后端未返回下载地址')
    window.open(url, '_blank', 'noopener,noreferrer')
    return { skipped: [] }
  }

  const path = GbCloudRecordController.getGbCloudRecordDownloadPath.replace('{id}', String(ids[0]))
  const query = `ids=${encodeURIComponent(ids.join(','))}`
  const stamp = new Date().toISOString().slice(0, 19).replace(/[-:T]/g, '')
  const headers = await downloadBinary(`${path}?${query}`, `cloud-records-${stamp}.zip`)
  return { skipped: parseSkippedHeader(headers.get('X-Skipped')) }
}

/**
 * 下载合并任务输出文件。
 *
 * 合并成功后由后端 `c.FileAttachment` 直接返回 MP4 流（不是 JSON），
 * 因此同样走带鉴权头的二进制下载；任务未完成时后端返回 409。
 */
export async function downloadMergeTask(id: number): Promise<void> {
  const path = GbCloudMergeController.downloadGbCloudMergeTaskPath.replace('{id}', String(id))
  await downloadBinary(path, `merge-task-${id}.mp4`)
}
