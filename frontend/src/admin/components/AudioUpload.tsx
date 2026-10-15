import { useEffect, useMemo, useState } from 'react'
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons'
import { Button, Space, Upload, message } from 'antd'
import type { UploadProps } from 'antd'
import { Common } from '../api/generated/Common'

export interface AudioUploadProps {
  /** 音频地址 */
  value?: string
  onChange?: (value: string) => void
  disabled?: boolean
  accept?: string
  className?: string
}

/**
 * 音频上传：受控组件，value 为音频 URL。
 * 上传走 `Common.uploadAudio`（POST `/web/upload/audio`，multipart/form-data）。
 */
export default function AudioUpload({
  value,
  onChange,
  disabled,
  accept = 'audio/*',
  className,
}: AudioUploadProps) {
  const [loading, setLoading] = useState(false)
  const [fileName, setFileName] = useState<string>('')

  useEffect(() => {
    if (value) {
      setFileName(value.split('/').pop() || '音频文件')
    } else {
      setFileName('')
    }
  }, [value])

  const beforeUpload: UploadProps['beforeUpload'] = async (file) => {
    const formData = new FormData()
    formData.append('file', file)
    setLoading(true)
    try {
      const res = await Common.uploadAudio(formData, { 'Content-Type': 'multipart/form-data' })
      const url = res?.data?.url ?? res?.data?.path ?? ''
      if (!url) {
        throw new Error(res?.message || '上传失败')
      }
      onChange?.(url)
      setFileName(res?.data?.name ?? file.name)
    } catch {
      message.error('音频上传失败')
    } finally {
      setLoading(false)
    }
    // 阻止 antd 默认上传行为，已由 Common.uploadAudio 处理
    return Upload.LIST_IGNORE
  }

  return (
    <Space direction="vertical" className={['audio-upload', className].filter(Boolean).join(' ')} style={{ width: '100%' }}>
      {value ? <audio src={value} controls style={{ width: '100%', maxWidth: 360 }} /> : null}
      <Space>
        <Upload
          accept={accept}
          showUploadList={false}
          disabled={disabled}
          beforeUpload={beforeUpload}
          maxCount={1}
        >
          <Button icon={<PlusOutlined />} loading={loading} disabled={disabled}>
            {value ? '重新上传' : '上传音频'}
          </Button>
        </Upload>
        {value ? (
          <Button danger icon={<DeleteOutlined />} disabled={disabled} onClick={() => onChange?.('')}>
            移除
          </Button>
        ) : null}
        {fileName ? <span style={{ color: '#8c8c8c' }}>{fileName}</span> : null}
      </Space>
    </Space>
  )
}

/** 只读音频播放器（详情页使用） */
export function AudioPlayer({ url }: { url?: string }) {
  const memoUrl = useMemo(() => url ?? '', [url])
  if (!memoUrl) return <span>-</span>
  return <audio src={memoUrl} controls style={{ height: 32 }} />
}
