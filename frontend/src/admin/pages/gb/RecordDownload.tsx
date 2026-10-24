import React, {
  forwardRef, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState,
} from 'react'
import {
  Alert,
  Button,
  DatePicker,
  Form,
  Modal,
  Popconfirm,
  Progress,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { CloseCircleOutlined, DownloadOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import dayjs, { type Dayjs } from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { GbChannelController } from '../../api/generated/GbChannelController'
import { GbPlaybackController } from '../../api/generated/GbPlaybackController'
import type { GbDownloadItem } from '../../api/generated/definitions/GbDownloadItem'
import type { OptionItem } from '../../api/generated/definitions/OptionItem'
import { useAuthStore } from '../../store/authStore'

const { RangePicker } = DatePicker

interface DownloadFormValues {
  channel_id: number
  range: [Dayjs, Dayjs]
}

interface DownloadFormHandle {
  submit: () => Promise<void>
}

interface DownloadFormContentProps {
  channelOptions: { label: string; value: number }[]
  onSubmit: (values: DownloadFormValues) => Promise<void>
}

/**
 * 开始下载表单内容。
 *
 * 表单实例与 Form 一起挂载在 Modal 的 children 中，避免 useForm 在
 * Modal 尚未渲染 Form 时被调用而产生控制台告警。
 */
const DownloadFormContent = forwardRef<DownloadFormHandle, DownloadFormContentProps>(
  function DownloadFormContent({ channelOptions, onSubmit }, ref) {
    const [form] = Form.useForm<DownloadFormValues>()

    useImperativeHandle(ref, () => ({
      async submit() {
        let values: DownloadFormValues
        try {
          values = await form.validateFields()
        } catch {
          return
        }
        await onSubmit(values)
      },
    }), [form, onSubmit])

    return (
      <Form
        form={form}
        layout="vertical"
        initialValues={{ range: [dayjs().startOf('day'), dayjs().endOf('day')] }}
      >
        <Form.Item
          name="channel_id"
          label="通道"
          rules={[{ required: true, message: '请选择通道' }]}
        >
          <Select<number>
            showSearch
            optionFilterProp="label"
            placeholder="请选择国标通道"
            options={channelOptions}
          />
        </Form.Item>
        <Form.Item
          name="range"
          label="录像时间段"
          rules={[{ required: true, message: '请选择录像时间段' }]}
        >
          <RangePicker showTime style={{ width: '100%' }} />
        </Form.Item>
        <Alert
          type="info"
          showIcon
          message="下载任务保存在服务进程内存，重启会丢失；本期只做单文件直接下载，不做合并打包。"
        />
      </Form>
    )
  },
)

function toGbTime(value: Dayjs): string {
  return value.format('YYYY-MM-DDTHH:mm:ss')
}

function fmtDT(value?: string): string {
  if (!value) return '-'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : value
}

function fmtTime(value?: string): string {
  if (!value) return '-'
  return value.replace('T', ' ')
}

function statusMeta(status?: string): { text: string; color: string } {
  switch (status) {
    case 'downloading':
      return { text: '下载中', color: 'blue' }
    case 'stopped':
      return { text: '已停止', color: 'default' }
    case 'completed':
      return { text: '已完成', color: 'green' }
    case 'failed':
      return { text: '失败', color: 'red' }
    default:
      return { text: status || '未知', color: 'default' }
  }
}

/**
 * 录像下载：设备侧录像按时间段下载到 ZLM 落盘。
 *
 * 后端边界（照实提示）：下载任务只保存在服务进程内存，重启即丢失；
 * 进度与文件大小尚无 ZLM 回调，本页按后端实际字段展示。
 */
const RecordDownload: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canStart = permCodes.includes('gb:record-download:start')
  const canCancel = permCodes.includes('gb:record-download:cancel')

  const [channelOptions, setChannelOptions] = useState<OptionItem[]>([])
  const [data, setData] = useState<GbDownloadItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState<string | undefined>()

  const [modalOpen, setModalOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const formRef = useRef<DownloadFormHandle>(null)

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

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbPlaybackController.pageGbDownloads(page, pageSize, keyword, undefined, status)
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      /* 拦截器已提示 */
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, status])

  useEffect(() => {
    void fetchData()
  }, [fetchData])

  // 下载任务在服务端异步推进，存在「下载中」任务时定时刷新
  const hasDownloading = data.some((item) => item.status === 'downloading')
  useEffect(() => {
    if (!hasDownloading) return undefined
    const timer = window.setInterval(() => {
      void fetchData()
    }, 5000)
    return () => window.clearInterval(timer)
  }, [hasDownloading, fetchData])

  const openCreate = () => setModalOpen(true)

  const handleCreate = async (values: DownloadFormValues) => {
    setSaving(true)
    try {
      await GbPlaybackController.startGbDownload({
        channel_id: values.channel_id,
        start_time: toGbTime(values.range[0]),
        end_time: toGbTime(values.range[1]),
      })
      message.success('下载任务已创建')
      setModalOpen(false)
      setPage(1)
      void fetchData()
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSaving(false)
    }
  }

  const handleCancel = async (row: GbDownloadItem) => {
    if (!row.id) return
    try {
      await GbPlaybackController.stopGbDownload(row.id)
      message.success('已停止下载任务')
      void fetchData()
    } catch {
      /* 拦截器已提示 */
    }
  }

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(String(values.keyword ?? ''))
    setStatus(values.status ? String(values.status) : undefined)
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setStatus(undefined)
    setPage(1)
  }

  const columns: ColumnsType<GbDownloadItem> = [
    {
      title: '通道',
      dataIndex: 'channel_name',
      width: 180,
      ellipsis: true,
      render: (_: string | undefined, row) =>
        row.channel_name || row.device_name || (row.channel_id ? `通道 ${row.channel_id}` : '-'),
    },
    {
      title: '时间段',
      key: 'timespan',
      width: 340,
      render: (_, row) => `${fmtTime(row.start_time)} ~ ${fmtTime(row.end_time)}`,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (value: string | undefined) => {
        const meta = statusMeta(value)
        return <Tag color={meta.color}>{meta.text}</Tag>
      },
    },
    {
      title: '进度',
      dataIndex: 'progress',
      width: 160,
      render: (value: number | undefined) => (
        <Progress
          percent={Math.max(0, Math.min(100, Math.round(value ?? 0)))}
          size="small"
          status="active"
        />
      ),
    },
    {
      title: '文件大小',
      key: 'file_size',
      width: 110,
      render: () => (
        <Tooltip title="后端下载任务当前未提供文件大小（等待 ZLM 录制回调）">
          <span>-</span>
        </Tooltip>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_dt',
      width: 170,
      render: (value: string | undefined) => fmtDT(value),
    },
    {
      title: '提示',
      dataIndex: 'message',
      width: 200,
      ellipsis: true,
      render: (value: string | undefined) => value || '-',
    },
    {
      title: '操作',
      key: 'action',
      width: 110,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={120}>
          {canCancel && row.status === 'downloading' ? (
            <Popconfirm title="确定取消该下载任务?" onConfirm={() => void handleCancel(row)}>
              <Button type="link" size="small" danger icon={<CloseCircleOutlined />}>
                取消
              </Button>
            </Popconfirm>
          ) : (
            <span style={{ color: '#bfbfbf', fontSize: 12 }}>-</span>
          )}
        </ActionButtons>
      ),
    },
  ]

  return (
    <PageContainer
      title="录像下载"
      subTitle="按通道与时间段把设备侧录像下载到媒体服务器"
      extra={
        <Button type="primary" icon={<PlusOutlined />} disabled={!canStart} onClick={openCreate}>
          开始下载
        </Button>
      }
    >
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 12 }}
        message="下载任务仅保存在服务进程内存，后端重启后会丢失"
        description="进度与文件大小依赖 ZLM 录制回调，本期后端尚未接入：进度可能一直为 0，文件大小显示为「-」。任务记录在重启后无法恢复。"
      />

      <SearchBar
        fields={[
          { name: 'keyword', label: '关键字', placeholder: '通道 / 设备 / 流名' },
          {
            name: 'status',
            label: '状态',
            type: 'select',
            options: [
              { label: '下载中', value: 'downloading' },
              { label: '已停止', value: 'stopped' },
            ],
          },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
        extra={
          <Button icon={<ReloadOutlined />} onClick={() => void fetchData()}>
            刷新
          </Button>
        }
      />

      <Table<GbDownloadItem>
        rowKey={(row) => String(row.id ?? '')}
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 'max-content' }}
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
      />

      <Modal
        className="e2e-gb-download-modal"
        title="开始录像下载"
        open={modalOpen}
        confirmLoading={saving}
        okText="开始下载"
        onOk={() => void formRef.current?.submit()}
        onCancel={() => setModalOpen(false)}
        destroyOnHidden
        width={560}
      >
        <DownloadFormContent
          ref={formRef}
          channelOptions={channelSelectOptions}
          onSubmit={handleCreate}
        />
      </Modal>
    </PageContainer>
  )
}

export default RecordDownload
