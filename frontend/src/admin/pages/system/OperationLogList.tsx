import { useCallback, useEffect, useState } from 'react'
import type { Key } from 'react'
import {
  Button, Col, DatePicker, Descriptions, Modal, Popconfirm, Row, Select, Space, Table, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { ClearOutlined, DeleteOutlined, DownloadOutlined, EyeOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { OperationLogController } from '../../api/generated/OperationLogController'
import type { OperationLogQuery } from '../../api/generated/OperationLogController'
import { useAuthStore } from '../../store/authStore'
import type { OperationLog } from '../../api/generated/definitions/OperationLog'

const { RangePicker } = DatePicker

/** 操作类型选项 */
const ACTION_OPTIONS = [
  { label: '新增', value: 'create' },
  { label: '修改', value: 'update' },
  { label: '删除', value: 'delete' },
  { label: '查询', value: 'query' },
  { label: '导出', value: 'export' },
]

/**
 * 操作日志：分页列表 + 详情 + 删除 + 清空 + 导出。
 * 请求参数（params）为 JSON 字符串，详情弹窗内格式化展示。
 */
export default function OperationLogList() {
  const [data, setData] = useState<OperationLog[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [search, setSearch] = useState<Record<string, unknown>>({})
  const [actionFilter, setActionFilter] = useState<string | undefined>(undefined)
  const [range, setRange] = useState<[string, string] | null>(null)
  const [selectedKeys, setSelectedKeys] = useState<Key[]>([])
  const [exporting, setExporting] = useState(false)

  const [detailOpen, setDetailOpen] = useState(false)
  const [detail, setDetail] = useState<OperationLog | null>(null)

  const permCodes = useAuthStore((s) => s.permCodes)
  // permCodes 为空说明菜单权限尚未加载，此时不隐藏按钮，避免整页操作被锁死
  const can = (code: string) => permCodes.length === 0 || permCodes.includes(code)
  const canDelete = can('system:log:delete')
  const canClear = can('system:log:clear')
  const canExport = can('system:log:export')

  /** 组装查询条件，列表与导出共用 */
  const buildQuery = useCallback((): OperationLogQuery => ({
    module: String(search.module ?? '') || undefined,
    operator: String(search.operator ?? '') || undefined,
    action: actionFilter,
    start_dt: range?.[0],
    end_dt: range?.[1],
  }), [search, actionFilter, range])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await OperationLogController.pageOperationLogs(page, pageSize, buildQuery())
      if (res.code === 200) {
        setData(res.data?.items ?? [])
        setTotal(res.data?.total ?? 0)
      } else {
        message.error(res.message || '获取操作日志失败')
      }
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, buildQuery])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const handleSearch = (values: Record<string, unknown>) => {
    setSearch(values)
    setPage(1)
  }

  const handleReset = () => {
    setSearch({})
    setActionFilter(undefined)
    setRange(null)
    setPage(1)
  }

  const openDetail = async (record: OperationLog) => {
    // 先用列表行数据渲染，再拉详情覆盖（详情含完整 params）
    setDetail(record)
    setDetailOpen(true)
    const res = await OperationLogController.getOperationLog(String(record.id))
    if (res.code === 200 && res.data) setDetail(res.data)
  }

  const handleDelete = async (record: OperationLog) => {
    try {
      const res = await OperationLogController.deleteOperationLog(String(record.id))
      if (res.code === 200) {
        message.success('删除成功')
        fetchData()
      } else {
        message.error(res.message || '删除失败')
      }
    } catch {
      message.error('删除失败')
    }
  }

  const handleBatchDelete = async () => {
    if (selectedKeys.length === 0) {
      message.warning('请先选择要删除的日志')
      return
    }
    try {
      await Promise.all(selectedKeys.map((key) => OperationLogController.deleteOperationLog(String(key))))
      message.success(`已删除 ${selectedKeys.length} 条记录`)
      setSelectedKeys([])
      fetchData()
    } catch {
      message.error('批量删除失败')
    }
  }

  const handleClear = async () => {
    try {
      const res = await OperationLogController.clearOperationLogs()
      if (res.code === 200) {
        message.success('已清空操作日志')
        setSelectedKeys([])
        fetchData()
      } else {
        message.error(res.message || '清空失败')
      }
    } catch {
      message.error('清空失败')
    }
  }

  /** 导出：以 blob 方式下载，保证请求携带鉴权头 */
  const handleExport = async () => {
    setExporting(true)
    try {
      const blob = await OperationLogController.exportOperationLogs(buildQuery())
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `操作日志_${dayjs().format('YYYYMMDDHHmmss')}.csv`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      window.URL.revokeObjectURL(url)
      message.success('导出成功')
    } catch {
      message.error('导出失败')
    } finally {
      setExporting(false)
    }
  }

  /** 请求参数为 JSON 字符串，格式化为可读文本；非 JSON 时原样展示 */
  const formatParams = (params?: string): string => {
    if (!params) return '-'
    try {
      return JSON.stringify(JSON.parse(params), null, 2)
    } catch {
      return params
    }
  }

  const columns: ColumnsType<OperationLog> = [
    { title: '日志编号', dataIndex: 'id', key: 'id', width: 90 },
    { title: '系统模块', dataIndex: 'module', key: 'module', width: 130 },
    {
      title: '操作类型', dataIndex: 'action', key: 'action', width: 100,
      render: (value: string) => {
        const label = ACTION_OPTIONS.find((item) => item.value === value)?.label ?? value
        return label ? <Tag color="blue">{label}</Tag> : '-'
      },
    },
    {
      title: '请求方式', dataIndex: 'method', key: 'method', width: 100,
      render: (value: string) => (value ? <Tag>{value}</Tag> : '-'),
    },
    { title: '操作人员', dataIndex: 'operator', key: 'operator', width: 120 },
    { title: '主机', dataIndex: 'ip', key: 'ip', width: 140 },
    { title: '操作地点', dataIndex: 'location', key: 'location', width: 140, ellipsis: true },
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 90,
      render: (value: number) => (value === 1 ? <Tag color="green">成功</Tag> : <Tag color="red">失败</Tag>),
    },
    {
      title: '操作时间', dataIndex: 'created_dt', key: 'created_dt', width: 170,
      render: (value: string) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
    {
      title: '操作', key: 'action-col', width: 160, fixed: 'right' as const,
      render: (_, record) => (
        <ActionButtons>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => openDetail(record)}>详情</Button>
          {canDelete && (
            <Popconfirm title="确定删除?" onConfirm={() => handleDelete(record)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>删除</Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  return (
    <PageContainer
      title="操作日志"
      subTitle="记录平台关键操作，支持按模块/人员/类型/时间检索与导出"
      extra={(
        <Space>
          {canExport && (
            <Button icon={<DownloadOutlined />} loading={exporting} onClick={handleExport}>导出</Button>
          )}
          {canDelete && (
            <Popconfirm title={`确定删除选中的 ${selectedKeys.length} 条记录?`} onConfirm={handleBatchDelete}>
              <Button danger disabled={selectedKeys.length === 0}>批量删除</Button>
            </Popconfirm>
          )}
          {canClear && (
            <Popconfirm title="确定清空全部操作日志?该操作不可恢复。" onConfirm={handleClear}>
              <Button danger icon={<ClearOutlined />}>清空</Button>
            </Popconfirm>
          )}
        </Space>
      )}
    >
      {/* 操作类型与时间范围非文本条件，SearchBar 仅承载模块/人员检索 */}
      <Row gutter={16} style={{ marginBottom: 12 }}>
        <Col>
          <Space>
            <span>操作类型：</span>
            <Select
              style={{ width: 140 }}
              allowClear
              placeholder="全部类型"
              value={actionFilter}
              onChange={(value: string | undefined) => {
                setActionFilter(value)
                setPage(1)
              }}
              options={ACTION_OPTIONS}
            />
          </Space>
        </Col>
        <Col>
          <Space>
            <span>操作时间：</span>
            <RangePicker
              showTime
              onChange={(_, dateStrings) => {
                setRange(dateStrings[0] && dateStrings[1] ? [dateStrings[0], dateStrings[1]] : null)
                setPage(1)
              }}
            />
          </Space>
        </Col>
      </Row>

      <SearchBar
        fields={[
          { name: 'module', label: '系统模块', placeholder: '请输入系统模块' },
          { name: 'operator', label: '操作人员', placeholder: '请输入操作人员' },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table
        rowKey="id"
        size="small"
        scroll={{ x: 'max-content' }}
        columns={columns}
        dataSource={data}
        loading={loading}
        rowSelection={{ selectedRowKeys: selectedKeys, onChange: (keys) => setSelectedKeys(keys) }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => {
            setPage(p)
            setPageSize(ps)
          },
        }}
      />

      <Modal
        className="e2e-operation-log-modal"
        title="操作日志详情"
        open={detailOpen}
        onCancel={() => setDetailOpen(false)}
        footer={<Button onClick={() => setDetailOpen(false)}>关闭</Button>}
        width={760}
      >
        <Descriptions column={2} size="small" bordered>
          <Descriptions.Item label="日志编号">{detail?.id ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="系统模块">{detail?.module ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="操作类型">
            {ACTION_OPTIONS.find((item) => item.value === detail?.action)?.label ?? detail?.action ?? '-'}
          </Descriptions.Item>
          <Descriptions.Item label="请求方式">{detail?.method ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="请求路径" span={2}>{detail?.path ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="操作人员">{detail?.operator ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="主机">{detail?.ip ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="操作地点">{detail?.location ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="耗时">{detail?.cost_ms != null ? `${detail.cost_ms} ms` : '-'}</Descriptions.Item>
          <Descriptions.Item label="状态">
            {detail?.status === 1 ? <Tag color="green">成功</Tag> : <Tag color="red">失败</Tag>}
          </Descriptions.Item>
          <Descriptions.Item label="操作时间">
            {detail?.created_dt ? dayjs(detail.created_dt).format('YYYY-MM-DD HH:mm:ss') : '-'}
          </Descriptions.Item>
          {detail?.error_msg ? (
            <Descriptions.Item label="错误信息" span={2}>
              <span style={{ color: '#cf1322' }}>{detail.error_msg}</span>
            </Descriptions.Item>
          ) : null}
          <Descriptions.Item label="请求参数" span={2}>
            <pre style={{ margin: 0, maxHeight: 260, overflow: 'auto', whiteSpace: 'pre-wrap' }}>
              {formatParams(detail?.params)}
            </pre>
          </Descriptions.Item>
        </Descriptions>
      </Modal>
    </PageContainer>
  )
}
