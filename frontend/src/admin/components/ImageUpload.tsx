import { useEffect, useMemo, useState } from 'react'
import { LoadingOutlined, PlusOutlined } from '@ant-design/icons'
import { Image, Upload, message } from 'antd'
import type { UploadFile, UploadProps } from 'antd'
import { Common } from '../api/generated/Common'

export interface ImageUploadProps {
  /** 图片地址；multiple 时为数组 */
  value?: string | string[]
  onChange?: (value: string | string[]) => void
  /** 最多上传数量，默认 1 */
  maxCount?: number
  /** 是否多选，默认 false */
  multiple?: boolean
  disabled?: boolean
  accept?: string
  /** 列表展示形态，默认 picture-card */
  listType?: UploadProps['listType']
  className?: string
}

function toUrlList(value?: string | string[]): string[] {
  if (Array.isArray(value)) return value.filter(Boolean)
  return value ? [value] : []
}

function toFileList(urls: string[]): UploadFile[] {
  return urls.map((url, index) => ({
    uid: `img-${index}-${url}`,
    name: url.split('/').pop() || `image-${index}`,
    status: 'done' as const,
    url,
  }))
}

/**
 * 图片上传：受控组件，value 为图片 URL（多图时为数组）。
 * 上传走 `Common.uploadFile`（POST `/web/upload`，multipart/form-data）。
 */
export default function ImageUpload({
  value,
  onChange,
  maxCount = 1,
  multiple = false,
  disabled,
  accept = 'image/*',
  listType = 'picture-card',
  className,
}: ImageUploadProps) {
  const urls = useMemo(() => toUrlList(value), [value])
  const [fileList, setFileList] = useState<UploadFile[]>(() => toFileList(urls))

  // 外部 value 变化时同步内部列表（相同则不重建，避免打断上传中的项）
  useEffect(() => {
    setFileList((prev) => {
      const prevUrls = prev.filter((item) => item.status === 'done').map((item) => item.url ?? '')
      const same = prevUrls.length === urls.length && prevUrls.every((url, index) => url === urls[index])
      return same ? prev : toFileList(urls)
    })
  }, [urls])

  const emit = (next: UploadFile[]) => {
    const nextUrls = next.filter((item) => item.status === 'done').map((item) => item.url ?? '').filter(Boolean)
    if (multiple || maxCount > 1) {
      onChange?.(nextUrls)
    } else {
      onChange?.(nextUrls[0] ?? '')
    }
  }

  const customRequest: UploadProps['customRequest'] = async (options) => {
    const file = options.file as File
    const formData = new FormData()
    formData.append('file', file)
    try {
      // 显式声明 multipart：否则 axios 会因默认 Content-Type 为 json 而把 FormData 转成 JSON
      const res = await Common.uploadFile(formData, { 'Content-Type': 'multipart/form-data' })
      const url = res?.data?.url ?? res?.data?.path ?? ''
      if (!url) {
        throw new Error(res?.message || '上传失败')
      }
      options.onSuccess?.(res)
      setFileList((prev) => {
        const next = prev.map((item) =>
          item.uid === (options.file as UploadFile).uid ? { ...item, status: 'done' as const, url } : item
        )
        emit(next)
        return next
      })
    } catch (err) {
      options.onError?.(err as Error)
      message.error('图片上传失败')
    }
  }

  const handleRemove: UploadProps['onRemove'] = (file) => {
    setFileList((prev) => {
      const next = prev.filter((item) => item.uid !== file.uid)
      emit(next)
      return next
    })
    return true
  }

  return (
    <Upload
      className={['image-upload', className].filter(Boolean).join(' ')}
      listType={listType}
      fileList={fileList}
      accept={accept}
      multiple={multiple}
      maxCount={maxCount}
      disabled={disabled}
      customRequest={customRequest}
      onRemove={handleRemove}
      onChange={({ fileList: next }) => setFileList(next)}
      onPreview={async (file) => {
        const src = file.url ?? file.thumbUrl
        if (src) window.open(src, '_blank')
      }}
    >
      {fileList.length >= maxCount ? null : (
        <div className="image-upload-trigger">
          {disabled ? <LoadingOutlined /> : <PlusOutlined />}
          <div style={{ marginTop: 8 }}>上传图片</div>
        </div>
      )}
    </Upload>
  )
}

/** 只读图片预览（详情页使用：单图缩略 + 点击放大） */
export function ImagePreview({ url, width = 80 }: { url?: string; width?: number }) {
  if (!url) return <span>-</span>
  return <Image src={url} width={width} />
}
