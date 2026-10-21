import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Button,
  Empty,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Table,
  Tag,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  DeleteOutlined,
  DownloadOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  StarFilled,
  StarOutlined,
} from '@ant-design/icons'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import ActionButtons from '../../components/ActionButtons'
import LivePlayer from '../../components/LivePlayer'
import { GbChannelController } from '../../api/generated/GbChannelController'
import { GbCloudRecordController } from '../../api/generated/GbCloudRecordController'
import type { GbCloudRecordItem } from '../../api/generated/definitions/GbCloudRecordItem'
import type { GbCloudRecordPlayResp } from '../../api/generated/definitions/GbCloudRecordPlayResp'
import type { OptionItem } from '../../api/generated/definitions/OptionItem'
import { useAuthStore } from '../../store/authStore'

function fmtDT(value?: string | null): string {
  if (!value) return '-'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : value
}

function formatDuration(totalSeconds?: number): string {
  if (totalSeconds === undefined || totalSeconds === null || Number.isNaN(totalSeconds)) return '-'
  const sec = Math.max(0, Math.floor(totalSeconds))
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(Math.floor(sec / 3600))}:${pad(Math.floor((sec % 3600) / 60))}:${pad(sec % 60)}`
}

function formatBytes(size?: number): string {
  if (size === undefined || size === null || size <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
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
 * 云端录像：ZLM 落盘 MP4 的索引查询、播放、单文件下载、收藏与删除。
 *
 * 日期筛选项来自后端 `listDates`（只列出确实有录像的日期），
 * 避免用户选一个空日期后误以为「没有录像」。
 */
const CloudRecordList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canList = permCodes.includes('gb:cloud-record:list')
  const canPlay = permCodes.includes('gb:cloud-record:play')
  const canDownload = permCodes.includes('gb:cloud-record:download')
  const canCollect = permCodes.includes('gb:cloud-record:collect')
  const canDelete = permCodes.includes('gb:cloud-record:delete')

  const [channelOptions, setChannelOptions] = useState<OptionItem[]>([])
  const [channelId, setChannelId] = useState<number | undefined>()
  const [dates, setDates] = useState<string[]>([])
  const [date, setDate] = useState<string | undefined>()
  const [keyword, setKeyword] = useState('')
  const [keywordInput, setKeywordInput] = useState('')
  const [data, setData] = useState<GbCloudRecordItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [selectedIds, setSelectedIds] = useState<number[]>([])

  const [playOpen, setPlayOpen] = useState(false)
  const [playLoading, setPlayLoading] = useState(false)
  const [playInfo, setPlayInfo] = useState<GbCloudRecordPlayResp | null>(null)
  const [playError, setPlayError] = useState('')
  const [playTitle, setPlayTitle] = useState('云端录像')

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

  const loadDates = useCallback(async () => {
    try {
      const res = await GbCloudRecordController.listGbCloudRecordDates(undefined, channelId)
      const list = res.data ?? []
      setDates(list)
      setDate((current) => (current && !list.includes(current) ? undefined : current))
    } catch {
      setDates([])
    }
  }, [channelId])

  useEffect(() => {
    void loadDates()
  }, [loadDates])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const startDt = date ? `${date} 00:00:00` : undefined
      const endDt = date ? `${date} 23:59:59` : undefined
      const res = await GbCloudRecordController.pageGbCloudRecords(
        page,
        pageSize,
        keyword,
        undefined,
        undefined,
        undefined,
        channelId,
        undefined,
        undefined,
        startDt,
        endDt,
      )
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      /* 拦截器已提示 */
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, channelId, date])

  useEffect(() => {
    void fetchData()
  }, [fetchData])

  // 筛选条件变化时回到第 1 页，并清空批量选择（避免误删不可见的记录）
  useEffect(() => {
    setPage(1)
    setSelectedIds([])
  }, [channelId, date, keyword])

  useEffect(() => {
    setSelectedIds([])
  }, [page, pageSize])

  const handlePlay = async (row: GbCloudRecordItem) => {
    if (!row.id) return
    setPlayOpen(true)
    setPlayLoading(true)
    setPlayInfo(null)
    setPlayError('')
    setPlayTitle(
      `${row.channel_name || row.device_name || '云端录像'} · ${fmtDT(row.start_time)}`,
    )
    try {
      const res = await GbCloudRecordController.getGbCloudRecordPlay(row.id)
      if (res.data) {
        setPlayInfo(res.data)
      } else {
        setPlayError('后端未返回播放地址')
      }
    } catch (error) {
      setPlayError(extractErrorMessage(error, '获取云端录像播放地址失败'))
    } finally {
      setPlayLoading(false)
    }
  }

  const handleDownload = async (row: GbCloudRecordItem) => {
    if (!row.id) return
    try {
      const res = await GbCloudRecordController.getGbCloudRecordDownload(row.id)
      const url = res.data?.url
      if (!url) {
        message.warning(res.data?.message || '后端未返回下载地址')
        return
      }
      window.open(url, '_blank', 'noopener,noreferrer')
      if (res.data?.packaging === false) {
        message.info('本期仅支持单文件直接下载，不做合并/切片/打包')
      }
    } catch {
      /* 拦截器已提示 */
    }
  }

  const handleCollect = async (row: GbCloudRecordItem) => {
    if (!row.id) return
    try {
      await GbCloudRecordController.collectGbCloudRecords({
        ids: [row.id],
        collect: !row.collect,
      })
      message.success(row.collect ? '已取消收藏' : '已收藏')
      void fetchData()
    } catch {
      /* 拦截器已提示 */
    }
  }

  const handleDelete = async (ids: number[]) => {
    if (ids.length === 0) return
    try {
      await GbCloudRecordController.deleteGbCloudRecords({ ids })
      message.success('删除成功')
      setSelectedIds([])
      void fetchData()
      void loadDates()
    } catch {
      /* 拦截器已提示 */
    }
  }

  const handleBatchCollect = async (collect: boolean) => {
    if (selectedIds.length === 0) return
    try {
      await GbCloudRecordController.collectGbCloudRecords({ ids: selectedIds, collect })
      message.success(collect ? '批量收藏成功' : '批量取消收藏成功')
      setSelectedIds([])
      void fetchData()
    } catch {
      /* 拦截器已提示 */
    }
  }

  const columns: ColumnsType<GbCloudRecordItem> = [
    {
      title: '通道',
      dataIndex: 'channel_name',
      width: 180,
      ellipsis: true,
      render: (_: string | undefined, row) =>
        row.channel_name || row.device_name || (row.channel_id ? `通道 ${row.channel_id}` : '-'),
    },
    {
      title: '开始时间',
      dataIndex: 'start_time',
      width: 170,
      render: (value?: string | null) => fmtDT(value),
    },
    {
      title: '结束时间',
      dataIndex: 'end_time',
      width: 170,
      render: (value?: string | null) => fmtDT(value),
    },
    { title: '时长', dataIndex: 'time_len', width: 100, render: (value?: number) => formatDuration(value) },
    { title: '文件大小', dataIndex: 'file_size', width: 110, render: (value?: number) => formatBytes(value) },
    {
      title: '收藏',
      dataIndex: 'collect',
      width: 90,
      render: (value?: boolean) =>
        value ? <Tag color="gold">已收藏</Tag> : <Tag>未收藏</Tag>,
    },
    {
      title: '文件',
      dataIndex: 'file_name',
      width: 180,
      ellipsis: true,
      render: (value?: string) => value || '-',
    },
    {
      title: '操作',
      key: 'action',
      width: 300,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={300}>
          <Button
            type="link"
            size="small"
            icon={<PlayCircleOutlined />}
            disabled={!canPlay}
            onClick={() => void handlePlay(row)}
          >
            播放
          </Button>
          <Button
            type="link"
            size="small"
            icon={<DownloadOutlined />}
            disabled={!canDownload}
            onClick={() => void handleDownload(row)}
          >
            下载
          </Button>
          <Button
            type="link"
            size="small"
            icon={row.collect ? <StarFilled /> : <StarOutlined />}
            disabled={!canCollect}
            onClick={() => void handleCollect(row)}
          >
            {row.collect ? '取消收藏' : '收藏'}
          </Button>
          {canDelete && row.id ? (
            <Popconfirm
              title="确定删除该录像索引?"
              onConfirm={() => void handleDelete(row.id ? [row.id] : [])}
            >
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          ) : null}
        </ActionButtons>
      ),
    },
  ]

  return (
    <PageContainer
      title="云端录像"
      subTitle="媒体服务器落盘录像的检索、播放、下载与收藏"
      extra={
        <Space wrap>
          <Button
            icon={<ReloadOutlined />}
            disabled={!canList}
            onClick={() => {
              void loadDates()
              void fetchData()
            }}
          >
            刷新
          </Button>
          <Button
            disabled={!canCollect || selectedIds.length === 0}
            onClick={() => void handleBatchCollect(true)}
          >
            批量收藏
          </Button>
          <Button
            disabled={!canCollect || selectedIds.length === 0}
            onClick={() => void handleBatchCollect(false)}
          >
            批量取消收藏
          </Button>
          <Popconfirm
            title={`确定删除选中的 ${selectedIds.length} 条录像索引?`}
            disabled={!canDelete || selectedIds.length === 0}
            onConfirm={() => void handleDelete(selectedIds)}
          >
            <Button danger disabled={!canDelete || selectedIds.length === 0}>
              批量删除
            </Button>
          </Popconfirm>
        </Space>
      }
    >
      <Space wrap style={{ marginBottom: 12 }}>
        <Select<number>
          allowClear
          showSearch
          optionFilterProp="label"
          placeholder="全部通道"
          style={{ width: 260 }}
          value={channelId}
          options={channelSelectOptions}
          onChange={(value) => setChannelId(value)}
        />
        <Select<string>
          allowClear
          placeholder="全部日期（有录像的日期）"
          style={{ width: 220 }}
          value={date}
          options={dates.map((item) => ({ label: item, value: item }))}
          onChange={(value) => setDate(value)}
        />
        <Input.Search
          allowClear
          placeholder="通道 / 设备 / 文件名"
          style={{ width: 240 }}
          value={keywordInput}
          onChange={(event) => setKeywordInput(event.target.value)}
          onSearch={(value) => {
            setKeyword(value.trim())
            setPage(1)
          }}
        />
      </Space>

      <Table<GbCloudRecordItem>
        rowKey={(row) => String(row.id ?? `${row.stream ?? ''}-${row.start_time ?? ''}`)}
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 'max-content' }}
        rowSelection={{
          selectedRowKeys: selectedIds,
          onChange: (keys) => setSelectedIds(keys.map((key) => Number(key))),
        }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (count) => `共 ${count} 条`,
          onChange: (nextPage, nextPageSize) => {
            setPage(nextPage)
            setPageSize(nextPageSize)
          },
        }}
        locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无云端录像" /> }}
      />

      <Modal
        className="e2e-gb-cloud-play-modal"
        title={playTitle}
        open={playOpen}
        footer={null}
        width={900}
        destroyOnHidden
        onCancel={() => {
          setPlayOpen(false)
          setPlayInfo(null)
          setPlayError('')
        }}
      >
        {playError ? (
          <Alert type="error" showIcon message="播放失败" description={playError} style={{ marginBottom: 8 }} />
        ) : null}
        <div
          style={{
            height: 480,
            background: '#000',
            borderRadius: 6,
            overflow: 'hidden',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          {playLoading ? (
            <Spin />
          ) : playInfo ? (
            <LivePlayer
              app={playInfo.app}
              stream={playInfo.stream}
              httpBase={playInfo.http_base}
              protocol="webrtc"
              objectFit="contain"
              style={{ width: '100%', height: '100%' }}
              onError={(error) => setPlayError(error.message)}
              onProtocolChange={(used) => {
                setPlayError('')
                if (used === 'flv') message.info('WebRTC 连接失败，已自动切换为 HTTP-FLV')
              }}
            />
          ) : (
            <span style={{ color: '#8c8c8c' }}>暂无播放地址</span>
          )}
        </div>
      </Modal>
    </PageContainer>
  )
}

export default CloudRecordList
