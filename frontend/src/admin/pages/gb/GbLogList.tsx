import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Alert, Button, Card, Space, Switch, Table, Tag, Tooltip, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { ClearOutlined, ReloadOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import axios from 'axios'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import EmptyState from '../../components/EmptyState'
import { GbOpsController } from '../../api/generated/GbOpsController'
import type { GbLogItem } from '../../api/generated/definitions/GbLogItem'
import { useAuthStore } from '../../store/authStore'
import { backoffDelay, buildSseUrl, sleep, streamSse } from './GbSse'

/** 日志级别（zap 小写；后端 level 作为「最低级别」过滤） */
const LEVEL_OPTIONS = [
  { label: '全部级别', value: '' },
  { label: 'DEBUG', value: 'debug' },
  { label: 'INFO', value: 'info' },
  { label: 'WARN', value: 'warn' },
  { label: 'ERROR', value: 'error' },
]

const LEVEL_COLORS: Record<string, string> = {
  debug: 'default',
  info: 'blue',
  warn: 'orange',
  error: 'red',
  dpanic: 'red',
  panic: 'red',
  fatal: 'red',
}

function levelColor(level?: string): string {
  return LEVEL_COLORS[(level ?? '').toLowerCase()] ?? 'default'
}

function fmtTime(value?: string): string {
  if (!value) return '-'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss.SSS') : value
}

function fmtClock(value?: string): string {
  if (!value) return '-'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('HH:mm:ss.SSS') : value
}

/** 请求失败提示：优先展示后端返回的 message */
function apiErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError<{ message?: string }>(err)) {
    return err.response?.data?.message || err.message || fallback
  }
  return err instanceof Error ? err.message : fallback
}

/** 实时流最多保留的条数（避免长时间挂机吃内存） */
const MAX_LIVE_LOGS = 500

type LiveStatus = 'connecting' | 'live' | 'reconnecting' | 'paused'

const LIVE_STATUS_META: Record<LiveStatus, { text: string; color: string }> = {
  connecting: { text: '连接中', color: 'blue' },
  live: { text: '实时中', color: 'green' },
  reconnecting: { text: '重连中', color: 'orange' },
  paused: { text: '已暂停', color: 'default' },
}

const logColumns: ColumnsType<GbLogItem> = [
  {
    title: '时间',
    dataIndex: 'time',
    width: 200,
    render: (value?: string) => fmtTime(value),
  },
  {
    title: '级别',
    dataIndex: 'level',
    width: 90,
    render: (value?: string) => <Tag color={levelColor(value)}>{(value || '-').toUpperCase()}</Tag>,
  },
  { title: 'Logger', dataIndex: 'logger', width: 200, ellipsis: true, render: (v?: string) => v || '-' },
  { title: '消息', dataIndex: 'message', ellipsis: true, render: (v?: string) => v || '-' },
  { title: '调用位置', dataIndex: 'caller', width: 240, ellipsis: true, render: (v?: string) => v || '-' },
  {
    title: '字段',
    dataIndex: 'fields',
    width: 220,
    ellipsis: true,
    render: (value?: Record<string, unknown>) => {
      if (!value || Object.keys(value).length === 0) return '-'
      let text = ''
      try {
        text = JSON.stringify(value)
      } catch {
        text = '[无法序列化]'
      }
      return (
        <Tooltip title={<pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{JSON.stringify(value, null, 2)}</pre>}>
          <span style={{ fontFamily: 'Consolas, Monaco, monospace' }}>{text}</span>
        </Tooltip>
      )
    },
  },
]

/**
 * 实时日志：上半部分为内存日志的分页检索，下半部分为 SSE 实时流。
 *
 * SSE 通过 `fetch` + `ReadableStream` 读取（EventSource 无法携带 JWT）；
 * 支持暂停/继续、清屏与断线指数退避重连，组件卸载或暂停时 abort。
 */
const GbLogList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canView = permCodes.includes('gb:log:view')
  const canStream = permCodes.includes('gb:log:stream')

  /* ── 历史检索 ── */
  const [data, setData] = useState<GbLogItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [level, setLevel] = useState<string | undefined>()
  const [keyword, setKeyword] = useState('')

  /* ── 实时流 ── */
  const [liveLogs, setLiveLogs] = useState<GbLogItem[]>([])
  const [paused, setPaused] = useState(false)
  const [liveStatus, setLiveStatus] = useState<LiveStatus>('paused')
  const [liveError, setLiveError] = useState('')
  const listRef = useRef<HTMLDivElement | null>(null)
  const seenRef = useRef<Set<string>>(new Set())

  const fetchData = useCallback(async () => {
    if (!canView) return
    setLoading(true)
    try {
      const res = await GbOpsController.pageGbLogs(page, pageSize, level, keyword || undefined)
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch (err) {
      message.error(apiErrorMessage(err, '获取日志失败'))
    } finally {
      setLoading(false)
    }
  }, [canView, page, pageSize, level, keyword])

  useEffect(() => { void fetchData() }, [fetchData])

  const handleSearch = (values: Record<string, unknown>) => {
    const nextLevel = typeof values.level === 'string' && values.level ? values.level : undefined
    setLevel(nextLevel)
    setKeyword(typeof values.keyword === 'string' ? values.keyword.trim() : '')
    setPage(1)
  }

  const handleReset = () => {
    setLevel(undefined)
    setKeyword('')
    setPage(1)
  }

  /** 追加实时日志（按 id 去重，重连回放不会重复） */
  const appendLog = useCallback((item: GbLogItem) => {
    const key = item.id !== undefined && item.id !== null
      ? `id:${item.id}`
      : `${item.time ?? ''}|${item.logger ?? ''}|${item.message ?? ''}`
    if (seenRef.current.has(key)) return
    if (seenRef.current.size > MAX_LIVE_LOGS * 4) seenRef.current.clear()
    seenRef.current.add(key)
    setLiveLogs((prev) => {
      const next = [...prev, item]
      return next.length > MAX_LIVE_LOGS ? next.slice(next.length - MAX_LIVE_LOGS) : next
    })
  }, [])

  /** SSE 连接：暂停 / 无权限 / 卸载时 abort；断线指数退避重连（上限 30s） */
  useEffect(() => {
    if (!canStream || paused) {
      setLiveStatus('paused')
      return undefined
    }

    const controller = new AbortController()
    let attempt = 0

    const run = async () => {
      while (!controller.signal.aborted) {
        setLiveStatus(attempt === 0 ? 'connecting' : 'reconnecting')
        try {
          await streamSse(
            buildSseUrl(GbOpsController.streamGbLogsPath, { level, keyword }),
            controller.signal,
            (message) => {
              if (message.event === 'ping') return
              if (message.event !== 'log') return
              let item: GbLogItem
              try {
                item = JSON.parse(message.data) as GbLogItem
              } catch {
                return
              }
              attempt = 0
              setLiveError('')
              setLiveStatus('live')
              appendLog(item)
            },
          )
        } catch (err) {
          if (controller.signal.aborted) break
          setLiveError(err instanceof Error ? err.message : '实时日志连接中断')
        }
        if (controller.signal.aborted) break

        attempt += 1
        setLiveStatus('reconnecting')
        await sleep(backoffDelay(attempt, 1500, 30000), controller.signal)
      }
    }

    void run()
    return () => controller.abort()
  }, [canStream, paused, level, keyword, appendLog])

  /** 新日志到达时，若用户仍在底部则自动滚动 */
  useEffect(() => {
    const element = listRef.current
    if (!element || paused) return
    const nearBottom = element.scrollHeight - element.scrollTop - element.clientHeight < 80
    if (nearBottom) element.scrollTop = element.scrollHeight
  }, [liveLogs, paused])

  const clearLive = () => {
    setLiveLogs([])
    seenRef.current.clear()
  }

  const statusMeta = LIVE_STATUS_META[liveStatus]

  return (
    <PageContainer
      title="实时日志"
      subTitle="内存日志检索与 SSE 实时日志流（保留最近 5000 条）"
      extra={
        <Button
          icon={<ReloadOutlined />}
          loading={loading}
          disabled={!canView}
          onClick={() => void fetchData()}
        >
          刷新
        </Button>
      }
    >
      <div className="e2e-gb-log-page">
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Card size="small" title="日志检索">
            {!canView ? (
              <Alert type="warning" showIcon message="当前账号无日志检索权限（gb:log:view）" />
            ) : (
              <>
                <SearchBar
                  fields={[
                    { name: 'level', label: '级别', type: 'select', options: LEVEL_OPTIONS, placeholder: '全部级别' },
                    { name: 'keyword', label: '关键字', placeholder: '消息 / Logger / 调用位置' },
                  ]}
                  onSearch={handleSearch}
                  onReset={handleReset}
                  loading={loading}
                />
                <Table<GbLogItem>
                  rowKey={(row, index) => String(row.id ?? `${row.time ?? ''}-${index}`)}
                  size="small"
                  loading={loading}
                  columns={logColumns}
                  dataSource={data}
                  scroll={{ x: 'max-content' }}
                  pagination={{
                    current: page,
                    pageSize,
                    total,
                    showSizeChanger: true,
                    showTotal: (count) => `共 ${count} 条`,
                    onChange: (nextPage, nextSize) => {
                      setPage(nextPage)
                      setPageSize(nextSize)
                    },
                  }}
                  locale={{ emptyText: <EmptyState description="暂无日志" /> }}
                />
              </>
            )}
          </Card>

          <Card
            size="small"
            title={
              <Space size={8}>
                实时日志
                <Tag className="e2e-gb-log-live-status" color={statusMeta.color}>{statusMeta.text}</Tag>
              </Space>
            }
            extra={
              <Space>
                <Switch
                  className="e2e-gb-log-live-toggle"
                  checked={!paused}
                  disabled={!canStream}
                  onChange={(checked) => setPaused(!checked)}
                  checkedChildren="实时"
                  unCheckedChildren="暂停"
                />
                <Button
                  className="e2e-gb-log-live-clear"
                  size="small"
                  icon={<ClearOutlined />}
                  onClick={clearLive}
                >
                  清屏
                </Button>
              </Space>
            }
          >
            {!canStream ? (
              <Alert type="warning" showIcon message="当前账号无实时日志权限（gb:log:stream）" />
            ) : (
              <>
                {liveError && liveStatus !== 'live' ? (
                  <Alert
                    type="warning"
                    showIcon
                    message="实时日志连接中断，正在自动重连"
                    description={liveError}
                    style={{ marginBottom: 8 }}
                  />
                ) : null}
                <div
                  ref={listRef}
                  className="e2e-gb-log-live-list"
                  style={{
                    maxHeight: 420,
                    minHeight: 180,
                    overflow: 'auto',
                    background: '#0b1b34',
                    borderRadius: 6,
                    padding: 8,
                    fontFamily: 'Consolas, Monaco, monospace',
                    fontSize: 12,
                    lineHeight: '20px',
                  }}
                >
                  {liveLogs.length === 0 ? (
                    paused ? (
                      <div style={{ color: '#8c8c8c', textAlign: 'center', padding: 32 }}>
                        实时日志已暂停，点击右上角开关继续
                      </div>
                    ) : (
                      <div style={{ color: '#8c8c8c', textAlign: 'center', padding: 32 }}>
                        {liveStatus === 'live'
                          ? '已连接，等待新日志…（建连时会回放最近 50 条）'
                          : '正在连接实时日志…'}
                      </div>
                    )
                  ) : (
                    liveLogs.map((item, index) => (
                      <div
                        key={`${item.id ?? 'x'}-${item.time ?? ''}-${index}`}
                        style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}
                      >
                        <span style={{ color: '#7f9bc4', flex: '0 0 auto' }}>{fmtClock(item.time)}</span>
                        <Tag
                          color={levelColor(item.level)}
                          style={{ marginInlineEnd: 0, flex: '0 0 auto', lineHeight: '18px' }}
                        >
                          {(item.level || '-').toUpperCase()}
                        </Tag>
                        <span style={{ color: '#c9d7ee', flex: '0 0 auto' }}>{item.logger || '-'}</span>
                        <span style={{ color: '#f0f4fb', wordBreak: 'break-all' }}>{item.message}</span>
                        {item.caller ? (
                          <span style={{ color: '#7f9bc4', marginLeft: 'auto', flex: '0 0 auto' }}>
                            {item.caller}
                          </span>
                        ) : null}
                      </div>
                    ))
                  )}
                </div>
              </>
            )}
          </Card>
        </Space>
      </div>
    </PageContainer>
  )
}

export default GbLogList
