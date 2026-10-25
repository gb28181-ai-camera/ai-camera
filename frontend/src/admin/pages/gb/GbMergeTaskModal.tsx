import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert, Button, Input, Modal, Select, Space, Table, Tag, Tooltip, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { DownloadOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import axios from 'axios'
import dayjs from 'dayjs'
import { GbCloudMergeController } from '../../api/generated/GbCloudMergeController'
import type { GbMergeTaskItem } from '../../api/generated/definitions/GbMergeTaskItem'
import { useAuthStore } from '../../store/authStore'
import { downloadMergeTask } from './GbCloudRecordDownload'

const STATUS_OPTIONS = [
  { label: '全部状态', value: '' },
  { label: '等待中', value: 'pending' },
  { label: '进行中', value: 'running' },
  { label: '已完成', value: 'success' },
  { label: '失败', value: 'failed' },
]

const STATUS_META: Record<string, { text: string; color: string }> = {
  pending: { text: '等待中', color: 'default' },
  running: { text: '进行中', color: 'blue' },
  success: { text: '已完成', color: 'green' },
  failed: { text: '失败', color: 'red' },
}

function statusMeta(status?: string): { text: string; color: string } {
  if (!status) return { text: '-', color: 'default' }
  return STATUS_META[status] ?? { text: status, color: 'default' }
}

function fmtDT(value?: string): string {
  if (!value) return '-'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : value
}

function formatDuration(totalSeconds?: number): string {
  if (totalSeconds === undefined || totalSeconds === null) return '-'
  const seconds = Math.max(0, Math.floor(totalSeconds))
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(Math.floor(seconds / 3600))}:${pad(Math.floor((seconds % 3600) / 60))}:${pad(seconds % 60)}`
}

function formatBytes(value?: number): string {
  if (value === undefined || value === null || value <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index += 1
  }
  return `${size.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

function apiErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError<{ message?: string }>(err)) {
    return err.response?.data?.message || err.message || fallback
  }
  return err instanceof Error ? err.message : fallback
}

interface GbMergeTaskModalProps {
  open: boolean
  /** 云端录像页当前勾选的录像 ID（顺序即合并顺序，服务端会按开始时间再排序） */
  selectedIds: number[]
  onClose: () => void
}

/**
 * 云端录像合并任务：把多段 MP4 用 ffmpeg concat 合成一个文件。
 *
 * - 创建：使用云端录像页当前勾选的 >=2 条录像；
 * - 列表：按状态筛选 + 关键字检索，存在 pending/running 任务时每 5 秒轮询；
 * - 下载：成功任务通过带 JWT 的二进制下载保存 MP4；失败任务展示 error_msg
 *   （如「未安装 ffmpeg」）。
 */
const GbMergeTaskModal: React.FC<GbMergeTaskModalProps> = ({ open, selectedIds, onClose }) => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('gb:cloud-record:merge')
  const canDownload = permCodes.includes('gb:cloud-record:merge-download')

  const [tasks, setTasks] = useState<GbMergeTaskItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [status, setStatus] = useState<string>('')
  const [keyword, setKeyword] = useState('')

  const [taskName, setTaskName] = useState('')
  const [creating, setCreating] = useState(false)
  const [downloadingId, setDownloadingId] = useState<number | null>(null)

  const fetchTasks = useCallback(async () => {
    if (!open) return
    setLoading(true)
    try {
      const res = await GbCloudMergeController.pageGbCloudMergeTasks(
        page,
        pageSize,
        status || undefined,
        keyword.trim() || undefined,
      )
      setTasks(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch (err) {
      message.error(apiErrorMessage(err, '获取合并任务失败'))
    } finally {
      setLoading(false)
    }
  }, [open, page, pageSize, status, keyword])

  useEffect(() => {
    void fetchTasks()
  }, [fetchTasks])

  // 只轮询有未完任务的列表；关闭弹窗时 cleanup
  const hasPending = useMemo(
    () => tasks.some((task) => task.status === 'pending' || task.status === 'running'),
    [tasks],
  )
  useEffect(() => {
    if (!open || !hasPending) return undefined
    const timer = window.setInterval(() => { void fetchTasks() }, 5000)
    return () => window.clearInterval(timer)
  }, [open, hasPending, fetchTasks])

  useEffect(() => {
    if (!open) {
      setTaskName('')
      setPage(1)
      setStatus('')
      setKeyword('')
      setTasks([])
    }
  }, [open])

  const handleCreate = async () => {
    if (selectedIds.length < 2) {
      message.warning('请先在云端录像列表中勾选至少 2 条录像')
      return
    }
    setCreating(true)
    try {
      const res = await GbCloudMergeController.createGbCloudMergeTask({
        ids: selectedIds,
        name: taskName.trim() || undefined,
      })
      const task = res.data
      if (task?.status === 'failed') {
        message.error(`合并任务创建后立即失败：${task.error_msg || '未知原因'}`)
      } else {
        message.success('合并任务已创建，正在后台处理')
      }
      setTaskName('')
      setPage(1)
      void fetchTasks()
    } catch (err) {
      message.error(apiErrorMessage(err, '创建合并任务失败'))
    } finally {
      setCreating(false)
    }
  }

  const handleDownload = async (row: GbMergeTaskItem) => {
    if (!row.id) return
    setDownloadingId(row.id)
    try {
      await downloadMergeTask(row.id)
      message.success('已开始下载合并文件')
    } catch (err) {
      message.error(err instanceof Error ? err.message : '下载合并文件失败')
    } finally {
      setDownloadingId(null)
    }
  }

  const columns: ColumnsType<GbMergeTaskItem> = [
    { title: '任务', dataIndex: 'name', width: 200, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (value?: string) => {
        const meta = statusMeta(value)
        return <Tag color={meta.color}>{meta.text}</Tag>
      },
    },
    { title: '录像数', dataIndex: 'record_count', width: 90, render: (v?: number) => v ?? '-' },
    { title: '总时长', dataIndex: 'duration_sec', width: 110, render: (v?: number) => formatDuration(v) },
    { title: '文件大小', dataIndex: 'file_size', width: 110, render: (v?: number) => formatBytes(v) },
    {
      title: '失败原因',
      dataIndex: 'error_msg',
      width: 220,
      ellipsis: true,
      render: (value?: string) =>
        value ? (
          <Tooltip title={value}>
            <span style={{ color: '#cf1322' }}>{value}</span>
          </Tooltip>
        ) : (
          '-'
        ),
    },
    { title: '创建时间', dataIndex: 'created_dt', width: 170, render: (v?: string) => fmtDT(v) },
    { title: '完成时间', dataIndex: 'finished_dt', width: 170, render: (v?: string) => fmtDT(v) },
    {
      title: '操作',
      key: 'action',
      width: 110,
      fixed: 'right',
      render: (_, row) =>
        canDownload && row.status === 'success' && row.id ? (
          <Button
            className="e2e-gb-merge-download-btn"
            type="link"
            size="small"
            icon={<DownloadOutlined />}
            loading={downloadingId === row.id}
            onClick={() => void handleDownload(row)}
          >
            下载
          </Button>
        ) : (
          <span style={{ color: '#bfbfbf', fontSize: 12 }}>-</span>
        ),
    },
  ]

  return (
    <Modal
      className="e2e-gb-merge-task-modal"
      title="云端录像合并任务"
      open={open}
      onCancel={onClose}
      footer={<Button onClick={onClose}>关闭</Button>}
      width={1080}
      destroyOnHidden
    >
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message="合并由后端调用 ffmpeg concat 完成"
          description="选择至少 2 段录像生成一个 MP4；若服务端未安装 ffmpeg，任务会立即失败并在「失败原因」中说明。任务在后台异步执行，列表会自动刷新。"
        />

        {canCreate ? (
          <Space wrap>
            <Tag color={selectedIds.length >= 2 ? 'blue' : 'default'} style={{ marginInlineEnd: 0 }}>
              已选 {selectedIds.length} 条录像
            </Tag>
            <Input
              placeholder="任务名称（可选）"
              value={taskName}
              maxLength={100}
              onChange={(event) => setTaskName(event.target.value)}
              style={{ width: 240 }}
            />
            <Button
              className="e2e-gb-merge-create-btn"
              type="primary"
              icon={<PlusOutlined />}
              loading={creating}
              disabled={selectedIds.length < 2}
              onClick={() => void handleCreate()}
            >
              创建合并任务
            </Button>
          </Space>
        ) : null}

        <Space wrap>
          <Input.Search
            allowClear
            placeholder="任务名 / 文件名"
            style={{ width: 240 }}
            onSearch={(value) => {
              setKeyword(value)
              setPage(1)
            }}
          />
          <Select<string>
            value={status}
            style={{ width: 140 }}
            onChange={(value) => {
              setStatus(value)
              setPage(1)
            }}
            options={STATUS_OPTIONS}
          />
          <Button
            icon={<ReloadOutlined />}
            loading={loading}
            onClick={() => void fetchTasks()}
          >
            刷新
          </Button>
        </Space>

        <Table<GbMergeTaskItem>
          className="e2e-gb-merge-task-table"
          size="small"
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={tasks}
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
          locale={{ emptyText: '暂无合并任务' }}
        />
      </Space>
    </Modal>
  )
}

export default GbMergeTaskModal
