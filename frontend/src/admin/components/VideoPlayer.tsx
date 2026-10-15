import { Modal } from 'antd'

export interface VideoPlayerProps {
  /** 视频地址（mp4 / HLS / 直播流） */
  src?: string
  /** 受控开关 */
  open: boolean
  onCancel: () => void
  title?: string
  width?: number
  /** 是否自动播放，默认 true */
  autoPlay?: boolean
  /** 是否为直播流（直播流沿用同一播放器，仅影响文案） */
  live?: boolean
}

/**
 * 视频播放弹窗：播放 mp4 录像或直播流。
 * Modal 类名为 `e2e-video-modal`（E2E 定位用）。
 *
 * @example
 * <VideoPlayer open={open} src={record.video_url} onCancel={() => setOpen(false)} />
 */
export default function VideoPlayer({
  src,
  open,
  onCancel,
  title = '视频播放',
  width = 900,
  autoPlay = true,
  live = false,
}: VideoPlayerProps) {
  return (
    <Modal
      className="e2e-video-modal"
      title={live ? `${title}（直播）` : title}
      open={open}
      onCancel={onCancel}
      footer={null}
      width={width}
      destroyOnHidden
      styles={{ body: { padding: 0, background: '#000' } }}
    >
      <video
        src={src}
        controls
        autoPlay={autoPlay}
        playsInline
        muted={autoPlay}
        style={{ width: '100%', maxHeight: '70vh', background: '#000', display: 'block' }}
      />
    </Modal>
  )
}
