import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button, Card, Col, DatePicker, Descriptions, Drawer, Row, Space, Spin, Statistic, Table, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { CopyOutlined, EyeOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import { GbSipLogController } from '../../api/generated/GbSipLogController'
import type { GbSipLog } from '../../api/generated/definitions/GbSipLog'
import type { GbSipLogStatsResp } from '../../api/generated/definitions/GbSipLogStatsResp'

const { RangePicker } = DatePicker

/** 常见 SIP 方法 */
const METHOD_OPTIONS = [
  { label: 'REGISTER', value: 'REGISTER' },
  { label: 'MESSAGE', value: 'MESSAGE' },
  { label: 'INVITE', value: 'INVITE' },
  { label: 'ACK', value: 'ACK' },
  { label: 'BYE', value: 'BYE' },
  { label: 'CANCEL', value: 'CANCEL' },
  { label: 'SUBSCRIBE', value: 'SUBSCRIBE' },
  { label: 'NOTIFY', value: 'NOTIFY' },
  { label: 'INFO', value: 'INFO' },
  { label: 'OPTIONS', value: 'OPTIONS' },
]

/** 处理结果 */
const RESULT_OPTIONS = [
  { label: 'success（成功）', value: 'success' },
  { label: 'failed（失败）', value: 'failed' },
  { label: 'rejected（拒绝）', value: 'rejected' },
]

/** 查询条件（已应用） */
interface SipLogQuery {
  gb_device_id?: string
  method?: string
  result?: string
  start_dt?: string
  end_dt?: string
}

/** 字符串值清理：空串转 undefined，交给全局中间件剔除空查询参数 */
function str(v: unknown): string | undefined {
  return typeof v === 'string' && v.trim() ? v.trim() : undefined
}

/** 时间格式化 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = dayjs(v)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : v
}

/** 方向标签 */
function directionTag(v?: string) {
  if (v === 'in') return <Tag color="blue">in</Tag>
  if (v === 'out') return <Tag color="purple">out</Tag>
  return v ? <Tag>{v}</Tag> : '-'
}

/** 结果标签 */
function resultTag(v?: string) {
  if (v === 'success') return <Tag color="green">success</Tag>
  if (v === 'failed') return <Tag color="red">failed</Tag>
  if (v === 'rejected') return <Tag color="orange">rejected</Tag>
  return v ? <Tag>{v}</Tag> : '-'
}

/**
 * 信令日志：只读。顶部按当前筛选条件展示统计，行点击/「详情」打开
 * 抽屉查看完整 SIP 报文。
 */
export default function SipLogList() {
  const [data, setData] = useState<GbSipLog[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [filters, setFilters] = useState<{ gb_device_id?: string; method?: string; result?: string }>({})
  const [range, setRange] = useState<[string, string] | null>(null)

  const [stats, setStats] = useState<GbSipLogStatsResp | null>(null)
  const [statsLoading, setStatsLoading] = useState(false)

  const [detailOpen, setDetailOpen] = useState(false)
  const [detail, setDetail] = useState<GbSipLog | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)

  const query = useMemo<SipLogQuery>(() => ({
    gb_device_id: filters.gb_device_id,
    method: filters.method,
    result: filters.result,
    start_dt: range?.[0],
    end_dt: range?.[1],
  }), [filters, range])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbSipLogController.pageGbSipLogs(
        page,
        pageSize,
        query.gb_device_id,
        undefined,
        undefined,
        query.method,
        undefined,
        query.result,
        query.start_dt,
        query.end_dt,
      )
      if (res.code === 200) {
        setData(res.data?.items ?? [])
        setTotal(res.data?.total ?? 0)
      } else {
        message.error(res.message || '获取信令日志失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, query])

  const fetchStats = useCallback(async () => {
    setStatsLoading(true)
    try {
      const res = await GbSipLogController.gbSipLogStats(
        query.gb_device_id,
        undefined,
        query.method,
        undefined,
        query.result,
        query.start_dt,
        query.end_dt,
      )
      if (res.code === 200) {
        setStats(res.data ?? null)
      } else {
        message.error(res.message || '获取信令统计失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setStatsLoading(false)
    }
  }, [query])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  useEffect(() => {
    fetchStats()
  }, [fetchStats])

  const handleSearch = (values: Record<string, unknown>) => {
    setFilters({
      gb_device_id: str(values.gb_device_id),
      method: str(values.method),
      result: str(values.result),
    })
    setPage(1)
  }

  const handleReset = () => {
    setFilters({})
    setRange(null)
    setPage(1)
  }

  const openDetail = async (row: GbSipLog) => {
    setDetail(row)
    setDetailOpen(true)
    if (!row.id) return
    setDetailLoading(true)
    try {
      const res = await GbSipLogController.getGbSipLog(String(row.id))
      if (res.code === 200 && res.data) setDetail(res.data)
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setDetailLoading(false)
    }
  }

  const handleCopyContent = async (content?: string) => {
    if (!content) {
      message.warning('无可复制的报文内容')
      return
    }
    try {
      await navigator.clipboard.writeText(content)
      message.success('已复制到剪贴板')
    } catch {
      message.warning('当前环境不支持自动复制，请手动选中复制')
    }
  }

  const columns: ColumnsType<GbSipLog> = [
    { title: '时间', dataIndex: 'created_dt', width: 170, render: (v?: string) => fmtDT(v) },
    { title: '方向', dataIndex: 'direction', width: 80, render: (v?: string) => directionTag(v) },
    {
      title: '方法', dataIndex: 'method', width: 120,
      render: (v?: string) => (v ? <Tag color="geekblue">{v}</Tag> : '-'),
    },
    { title: '命令类型', dataIndex: 'cmd_type', width: 130, render: (v?: string) => v || '-' },
    {
      title: 'SIP 状态码', dataIndex: 'status_code', width: 110,
      render: (v?: number) => (v != null ? v : '-'),
    },
    { title: '设备编码', dataIndex: 'gb_device_id', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '远端地址', dataIndex: 'remote_addr', width: 170, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '结果', dataIndex: 'result', width: 100, render: (v?: string) => resultTag(v) },
    { title: '原因', dataIndex: 'reason', width: 220, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '操作', key: 'action', width: 90, fixed: 'right',
      render: (_, row) => (
        <Button
          type="link"
          size="small"
          icon={<EyeOutlined />}
          onClick={(e) => {
            e.stopPropagation()
            openDetail(row)
          }}
        >
          详情
        </Button>
      ),
    },
  ]

  const statCards = [
    { title: '总日志数', value: stats?.total ?? 0, color: undefined as string | undefined },
    { title: '成功', value: stats?.success ?? 0, color: '#3f8600' },
    { title: '失败', value: stats?.failed ?? 0, color: '#cf1322' },
    { title: '拒绝', value: stats?.rejected ?? 0, color: '#d46b08' },
  ]

  return (
    <PageContainer
      title="信令日志"
      subTitle="记录国标 SIP 信令收发过程，用于排查注册、目录、invite 等交互问题"
    >
      <Row gutter={16} style={{ marginBottom: 12 }}>
        {statCards.map((item) => (
          <Col xs={12} sm={12} md={6} key={item.title}>
            <Card size="small" loading={statsLoading}>
              <Statistic
                title={item.title}
                value={item.value}
                valueStyle={item.color ? { color: item.color } : undefined}
              />
            </Card>
          </Col>
        ))}
      </Row>

      <Row gutter={16} style={{ marginBottom: 12 }}>
        <Col>
          <Space>
            <span>时间范围：</span>
            <RangePicker
              showTime
              value={range ? [dayjs(range[0]), dayjs(range[1])] : null}
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
          { name: 'gb_device_id', label: '设备编码', placeholder: '请输入设备国标编码' },
          {
            name: 'method',
            label: '方法',
            type: 'select',
            placeholder: '全部方法',
            options: METHOD_OPTIONS,
          },
          {
            name: 'result',
            label: '结果',
            type: 'select',
            placeholder: '全部结果',
            options: RESULT_OPTIONS,
          },
        ]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table<GbSipLog>
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 'max-content' }}
        onRow={(record) => ({
          onClick: () => openDetail(record),
          style: { cursor: 'pointer' },
        })}
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

      <Drawer
        className="e2e-gb-sip-log-drawer"
        title="信令日志详情"
        width={760}
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
      >
        <Spin spinning={detailLoading}>
          <Descriptions column={2} size="small" bordered>
            <Descriptions.Item label="时间">{fmtDT(detail?.created_dt)}</Descriptions.Item>
            <Descriptions.Item label="方向">{directionTag(detail?.direction)}</Descriptions.Item>
            <Descriptions.Item label="方法">{detail?.method || '-'}</Descriptions.Item>
            <Descriptions.Item label="命令类型">{detail?.cmd_type || '-'}</Descriptions.Item>
            <Descriptions.Item label="SIP 状态码">{detail?.status_code ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="结果">{resultTag(detail?.result)}</Descriptions.Item>
            <Descriptions.Item label="设备编码">{detail?.gb_device_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="通道编码">{detail?.gb_channel_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="远端地址">{detail?.remote_addr || '-'}</Descriptions.Item>
            <Descriptions.Item label="本地地址">{detail?.local_addr || '-'}</Descriptions.Item>
            <Descriptions.Item label="Call-ID" span={2}>{detail?.call_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="耗时">
              {detail?.elapsed_ms != null ? `${detail.elapsed_ms} ms` : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="原因">{detail?.reason || '-'}</Descriptions.Item>
          </Descriptions>

          <div
            style={{
              marginTop: 16,
              marginBottom: 8,
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
            }}
          >
            <strong>完整报文</strong>
            <Button size="small" icon={<CopyOutlined />} onClick={() => handleCopyContent(detail?.content)}>
              复制
            </Button>
          </div>
          <pre
            style={{
              margin: 0,
              maxHeight: 420,
              overflow: 'auto',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
              background: '#fafafa',
              border: '1px solid #f0f0f0',
              borderRadius: 4,
              padding: 12,
              fontFamily: 'Consolas, Monaco, monospace',
              fontSize: 12,
            }}
          >
            {detail?.content || '（无报文内容）'}
          </pre>
        </Spin>
      </Drawer>
    </PageContainer>
  )
}
