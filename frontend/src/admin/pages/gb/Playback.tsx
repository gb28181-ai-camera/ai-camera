import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  DatePicker,
  Empty,
  Select,
  Slider,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlayCircleOutlined, StopOutlined } from '@ant-design/icons'
import dayjs, { type Dayjs } from 'dayjs'
import PageContainer from '../../components/PageContainer'
import ActionButtons from '../../components/ActionButtons'
import LivePlayer from '../../components/LivePlayer'
import { GbChannelController } from '../../api/generated/GbChannelController'
import { GbPlaybackController } from '../../api/generated/GbPlaybackController'
import type { GbRecordItem } from '../../api/generated/definitions/GbRecordItem'
import type { GbPlaybackResp } from '../../api/generated/definitions/GbPlaybackResp'
import type { OptionItem } from '../../api/generated/definitions/OptionItem'
import { useAuthStore } from '../../store/authStore'

const { RangePicker } = DatePicker

/** 后端 / 设备要求的时间格式：2006-01-02T15:04:05 */
function toGbTime(value: Dayjs): string {
  return value.format('YYYY-MM-DDTHH:mm:ss')
}

/** 秒 → HH:mm:ss */
function formatDuration(totalSeconds?: number): string {
  if (totalSeconds === undefined || totalSeconds === null || Number.isNaN(totalSeconds)) return '-'
  const sec = Math.max(0, Math.floor(totalSeconds))
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(Math.floor(sec / 3600))}:${pad(Math.floor((sec % 3600) / 60))}:${pad(sec % 60)}`
}

/** 展示时间：设备上报的原样字符串优先，避免时区二次转换 */
function fmtTime(value?: string): string {
  if (!value) return '-'
  const text = value.replace('T', ' ')
  return text.length > 19 ? text.slice(0, 19) : text
}

function extractErrorMessage(error: unknown, fallback: string): string {
  if (typeof error === 'object' && error !== null && 'response' in error) {
    const response = (error as { response?: { data?: { message?: unknown } } }).response
    const text = response?.data?.message
    if (typeof text === 'string' && text.trim()) return text
  }
  if (error instanceof Error && error.message) return error.message
  return fallback
}

/**
 * 录像回放：选择通道与时间窗 → RecordInfo 检索 → 发起 Playback INVITE。
 *
 * ⚠️ 回放控制走**带内 SIP INFO + MANSRTSP**（接口 `/web/gb/playback/control`），
 * 与云台的带外 MESSAGE 是两套路径，本页不复用云台接口。
 */
const Playback: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canQuery = permCodes.includes('gb:playback:list')
  const canPlay = permCodes.includes('gb:playback:play')
  const canControl = permCodes.includes('gb:playback:control')

  const [channelOptions, setChannelOptions] = useState<OptionItem[]>([])
  const [channelId, setChannelId] = useState<number | undefined>()
  const [range, setRange] = useState<[Dayjs, Dayjs] | null>([dayjs().startOf('day'), dayjs().endOf('day')])
  const [records, setRecords] = useState<GbRecordItem[]>([])
  const [querying, setQuerying] = useState(false)
  const [timeoutFlag, setTimeoutFlag] = useState(false)
  const [session, setSession] = useState<GbPlaybackResp | null>(null)
  const [starting, setStarting] = useState(false)
  const [videoError, setVideoError] = useState('')
  const [paused, setPaused] = useState(false)
  const [speed, setSpeed] = useState(1)
  const [position, setPosition] = useState(0)

  const streamIDRef = useRef('')
  /** 请求序号：切通道 / 停止时递增，丢弃迟到的回放响应并清理其会话 */
  const playRequestRef = useRef(0)

  useEffect(() => {
    void GbChannelController.gbChannelOptions()
      .then((res) => setChannelOptions(res.data ?? []))
      .catch(() => {
        /* 拦截器已提示 */
      })
  }, [])

  const channelSelectOptions = useMemo(
    () =>
      channelOptions.flatMap((item) => {
        if (typeof item.id !== 'number') return []
        const code = item.code ? `（${item.code}）` : ''
        return [{ label: `${item.name ?? item.id}${code}`, value: item.id }]
      }),
    [channelOptions],
  )

  /** 停止回放：必须回传发起回放时拿到的 stream_id */
  const stopPlayback = useCallback(async () => {
    const streamId = streamIDRef.current
    streamIDRef.current = ''
    setSession(null)
    setPaused(false)
    setPosition(0)
    if (streamId) {
      try {
        await GbPlaybackController.stopGbPlayback({ stream_id: streamId })
      } catch {
        // 会话不存在时后端幂等；网络错误由拦截器提示
      }
    }
  }, [])

  // 离开页面：停止回放，避免设备继续推流
  useEffect(
    () => () => {
      playRequestRef.current += 1
      void stopPlayback()
    },
    [stopPlayback],
  )

  /** 用户主动停止：让在途回放请求失效后再停流 */
  const handleStop = () => {
    playRequestRef.current += 1
    void stopPlayback()
  }

  const handleQuery = async () => {
    if (!channelId) {
      message.warning('请先选择通道')
      return
    }
    if (!range) {
      message.warning('请选择时间范围')
      return
    }
    setQuerying(true)
    setTimeoutFlag(false)
    try {
      const res = await GbPlaybackController.queryGbPlaybackRecords(
        channelId,
        toGbTime(range[0]),
        toGbTime(range[1]),
      )
      setRecords(res.data?.items ?? [])
      if (res.data?.timeout) {
        setTimeoutFlag(true)
        message.warning('设备响应超时，返回结果可能不完整')
      } else if ((res.data?.items ?? []).length === 0) {
        message.info('该时间范围内没有录像')
      }
    } catch {
      setRecords([])
    } finally {
      setQuerying(false)
    }
  }

  const handlePlayRecord = async (record: GbRecordItem) => {
    if (!canPlay) {
      message.warning('当前账号没有回放权限')
      return
    }
    if (!channelId || !record.start_time || !record.end_time) {
      message.warning('录像片段缺少开始/结束时间')
      return
    }
    setStarting(true)
    setVideoError('')
    const requestId = playRequestRef.current + 1
    playRequestRef.current = requestId
    await stopPlayback()
    try {
      const res = await GbPlaybackController.startGbPlayback({
        channel_id: channelId,
        start_time: record.start_time,
        end_time: record.end_time,
      })
      const data = res.data
      // 请求期间用户已切通道 / 停止：清理迟到建立的回放会话
      if (playRequestRef.current !== requestId) {
        if (data?.stream_id) {
          void GbPlaybackController.stopGbPlayback({ stream_id: data.stream_id }).catch(() => {
            /* 尽力清理 */
          })
        }
        return
      }
      if (!data) {
        setVideoError('后端未返回回放地址')
        return
      }
      streamIDRef.current = data.stream_id ?? ''
      setSession(data)
      setPaused(false)
      setSpeed(1)
      setPosition(0)
    } catch (error) {
      setVideoError(extractErrorMessage(error, '发起回放失败'))
    } finally {
      setStarting(false)
    }
  }

  const durationSeconds = useMemo(() => {
    if (!session?.start_time || !session?.end_time) return 0
    const start = dayjs(session.start_time)
    const end = dayjs(session.end_time)
    if (!start.isValid() || !end.isValid()) return 0
    return Math.max(0, end.diff(start, 'second'))
  }, [session])

  // 无进度回调：用一个本地计时器驱动进度条（仅 UI 参考值，不作为精确播放位置）
  useEffect(() => {
    if (!session || paused) return undefined
    const timer = window.setInterval(() => {
      setPosition((prev) => (durationSeconds > 0 ? Math.min(durationSeconds, prev + 1) : prev))
    }, 1000)
    return () => window.clearInterval(timer)
  }, [session, paused, durationSeconds])

  const sendControl = async (payload: {
    action: 'pause' | 'resume' | 'seek' | 'speed'
    seek_time?: string
    scale?: number
  }) => {
    const streamId = streamIDRef.current
    if (!streamId) return
    try {
      await GbPlaybackController.controlGbPlayback({ stream_id: streamId, ...payload })
    } catch {
      // 拦截器已展示后端中文错误
    }
  }

  const handlePauseResume = async () => {
    if (!session) return
    const nextPaused = !paused
    await sendControl({ action: nextPaused ? 'pause' : 'resume' })
    setPaused(nextPaused)
  }

  const handleSeek = async (value: number) => {
    if (!session?.start_time) return
    const seekTime = dayjs(session.start_time).add(value, 'second').format('YYYY-MM-DDTHH:mm:ss')
    await sendControl({ action: 'seek', seek_time: seekTime })
    setPosition(value)
  }

  const handleSpeedChange = async (value: number) => {
    setSpeed(value)
    await sendControl({ action: 'speed', scale: value })
  }

  const columns: ColumnsType<GbRecordItem> = [
    { title: '开始时间', dataIndex: 'start_time', width: 180, render: (v?: string) => fmtTime(v) },
    { title: '结束时间', dataIndex: 'end_time', width: 180, render: (v?: string) => fmtTime(v) },
    { title: '时长', dataIndex: 'time_len', width: 110, render: (v?: number) => formatDuration(v) },
    {
      title: '类型',
      dataIndex: 'type',
      width: 100,
      render: (v?: string) => (v ? <Tag>{v}</Tag> : '-'),
    },
    { title: '名称', dataIndex: 'name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '文件路径', dataIndex: 'file_path', ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '操作',
      key: 'action',
      width: 110,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={120}>
          <Button
            type="link"
            size="small"
            icon={<PlayCircleOutlined />}
            disabled={!canPlay || !channelId}
            onClick={() => void handlePlayRecord(row)}
          >
            播放
          </Button>
        </ActionButtons>
      ),
    },
  ]

  return (
    <PageContainer title="录像回放" subTitle="按通道与时间范围检索设备侧录像，支持暂停、拖动与倍速">
      <Card size="small">
        <Space wrap align="center">
          <Select<number>
            allowClear
            showSearch
            optionFilterProp="label"
            placeholder="请选择通道"
            style={{ width: 280 }}
            value={channelId}
            options={channelSelectOptions}
            onChange={(value) => {
              if (value !== channelId) {
                playRequestRef.current += 1
                void stopPlayback()
              }
              setChannelId(value)
            }}
          />
          <RangePicker
            showTime
            style={{ width: 400 }}
            value={range}
            onChange={(values) => {
              setRange(values && values[0] && values[1] ? [values[0], values[1]] : null)
            }}
          />
          <Button type="primary" loading={querying} disabled={!canQuery} onClick={() => void handleQuery()}>
            查询录像
          </Button>
        </Space>
      </Card>

      {timeoutFlag ? (
        <Alert
          type="warning"
          showIcon
          style={{ marginTop: 12 }}
          message="设备响应超时"
          description="已返回当前收到的部分录像片段，可稍后重新查询。"
        />
      ) : null}

      <Table<GbRecordItem>
        style={{ marginTop: 12 }}
        rowKey={(row) => `${row.file_path ?? ''}-${row.start_time ?? ''}-${row.end_time ?? ''}`}
        size="small"
        loading={querying}
        columns={columns}
        dataSource={records}
        scroll={{ x: 'max-content' }}
        pagination={false}
        locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="请选择通道与时间范围后查询" /> }}
      />

      <Card
        size="small"
        title="回放窗口"
        style={{ marginTop: 12 }}
        extra={
          session ? (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {fmtTime(session.start_time)} ~ {fmtTime(session.end_time)}
            </Typography.Text>
          ) : null
        }
      >
        {videoError ? (
          <Alert
            type="error"
            showIcon
            style={{ marginBottom: 8 }}
            message="回放失败"
            description={videoError}
          />
        ) : null}

        <div
          style={{
            height: 420,
            background: '#000',
            borderRadius: 6,
            overflow: 'hidden',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          {session ? (
            <LivePlayer
              app={session.app}
              stream={session.stream}
              httpBase={session.http_base}
              protocol="webrtc"
              objectFit="contain"
              style={{ width: '100%', height: '100%' }}
              onError={(error) => setVideoError(error.message)}
              onProtocolChange={(used) => {
                setVideoError('')
                if (used === 'flv') message.info('WebRTC 连接失败，已自动切换为 HTTP-FLV')
              }}
            />
          ) : (
            <span style={{ color: '#8c8c8c' }}>
              {starting ? '正在发起回放…' : '从上方录像片段中点击「播放」'}
            </span>
          )}
        </div>

        <div style={{ marginTop: 12 }}>
          <Space wrap align="center" style={{ marginBottom: 4 }}>
            <Button
              size="small"
              type={paused ? 'primary' : 'default'}
              disabled={!session || !canControl}
              onClick={() => void handlePauseResume()}
            >
              {paused ? '继续' : '暂停'}
            </Button>
            <Button
              size="small"
              icon={<StopOutlined />}
              disabled={!session}
              onClick={handleStop}
            >
              停止回放
            </Button>
            <span style={{ color: '#8c8c8c', fontSize: 12 }}>倍速</span>
            <Select<number>
              size="small"
              style={{ width: 90 }}
              value={speed}
              disabled={!session || !canControl}
              options={[0.5, 1, 2, 4].map((value) => ({ label: `${value}x`, value }))}
              onChange={(value) => void handleSpeedChange(value)}
            />
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {formatDuration(position)} / {formatDuration(durationSeconds)}
            </Typography.Text>
          </Space>
          <Slider
            min={0}
            max={Math.max(1, durationSeconds)}
            value={Math.min(position, Math.max(1, durationSeconds))}
            disabled={!session || !canControl || durationSeconds <= 0}
            tooltip={{ formatter: (value) => formatDuration(value) }}
            onChange={(value: number) => setPosition(value)}
            onAfterChange={(value: number) => void handleSeek(value)}
          />
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            回放控制通过带内 SIP INFO（MANSRTSP）下发，与云台控制不是同一接口。
          </Typography.Text>
        </div>
      </Card>
    </PageContainer>
  )
}

export default Playback
