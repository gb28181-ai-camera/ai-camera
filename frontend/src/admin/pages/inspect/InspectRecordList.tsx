import React, { useCallback, useEffect, useState } from 'react'
import {
  Alert, Button, DatePicker, Descriptions, Image, message, Modal, Space, Table,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { EyeOutlined } from '@ant-design/icons'
import type { Dayjs } from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import RegionSelect from '../../components/RegionSelect'
import UserSelect from '../../components/UserSelect'
import StatusTag from '../../components/StatusTag'
import { InspectController } from '../../api/generated/InspectController'
import type { InspectRecord } from '../../api/generated/definitions/InspectRecord'
import type { PageInspectRecordReq } from '../../api/generated/definitions/PageInspectRecordReq'
import { formatDT } from '../../types/business'

const { RangePicker } = DatePicker

/**
 * 巡检记录：巡检人在移动端处理完巡检任务后产生的记录（只读）。
 *
 * 记录保留任务快照字段（任务名称 / 区域 / 位置 / 巡检人 / 内容），
 * 因此任务被编辑或删除后记录仍可独立查看。
 */
const InspectRecordList: React.FC = () => {
  const [data, setData] = useState<InspectRecord[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [keyword, setKeyword] = useState('')
  const [regionID, setRegionID] = useState<number | undefined>()
  const [inspectorID, setInspectorID] = useState<number | undefined>()
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null)

  const [detailOpen, setDetailOpen] = useState(false)
  const [detail, setDetail] = useState<InspectRecord | null>(null)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const query: PageInspectRecordReq = {
        page,
        page_size: pageSize,
        keyword: keyword || undefined,
        region_id: regionID,
        inspector_id: inspectorID,
        start_dt: range?.[0] ? range[0].format('YYYY-MM-DD HH:mm:ss') : undefined,
        end_dt: range?.[1] ? range[1].format('YYYY-MM-DD HH:mm:ss') : undefined,
      }
      const res = await InspectController.pageInspectRecords(query)
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch {
      message.error('获取巡检记录失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, regionID, inspectorID, range])

  useEffect(() => { void fetchData() }, [fetchData])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setRegionID(undefined)
    setInspectorID(undefined)
    setRange(null)
    setPage(1)
  }

  /** 查看记录详情（详情内同时展示处理结果图与任务快照信息） */
  const openDetail = async (row: InspectRecord) => {
    setDetail(row)
    setDetailOpen(true)
    if (row.id) {
      try {
        const res = await InspectController.getInspectRecord(row.id)
        if (res.data) setDetail(res.data)
      } catch {
        /* 详情接口失败时沿用列表数据 */
      }
    }
  }

  const columns: ColumnsType<InspectRecord> = [
    { title: '任务名称', dataIndex: 'task_name', width: 180, ellipsis: true },
    { title: '所属区域', dataIndex: ['region', 'name'], width: 140, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '巡检位置', dataIndex: 'location', width: 180, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '巡检人', dataIndex: 'inspector', width: 110, render: (v?: string) => v || '-' },
    { title: '巡检内容', dataIndex: 'content', width: 220, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '巡检时间', dataIndex: 'plan_dt', width: 170, render: (v?: string) => formatDT(v) },
    {
      title: '处理结果图',
      dataIndex: 'result_image',
      width: 100,
      render: (value: unknown) =>
        value ? (
          <Image src={String(value)} width={48} height={48} style={{ objectFit: 'cover' }} />
        ) : (
          <span>-</span>
        ),
    },
    { title: '处理时间', dataIndex: 'handled_dt', width: 170, render: (v?: string) => formatDT(v) },
    { title: '处理结果', dataIndex: 'result', width: 240, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (v?: string) => <StatusTag type="inspect" status={v} />,
    },
    {
      title: '操作',
      key: 'action',
      width: 100,
      fixed: 'right',
      render: (_: unknown, row) => (
        <ActionButtons maxWidth={100}>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => void openDetail(row)}>
            查看
          </Button>
        </ActionButtons>
      ),
    },
  ]

  return (
    <PageContainer title="巡检记录" subTitle="巡检人处理完成后产生的巡检记录（只读）">
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="巡检记录由巡检人在移动端提交处理结果后自动生成，管理后台不提供新增、编辑与删除"
      />

      <SearchBar
        fields={[{ name: 'keyword', label: '任务名称', placeholder: '请输入任务名称' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Space wrap style={{ marginBottom: 16 }}>
        <RegionSelect
          style={{ width: 180 }}
          value={regionID}
          onChange={(v) => { setRegionID(v === undefined || v === null ? undefined : Number(v)); setPage(1) }}
        />
        <UserSelect
          style={{ width: 180 }}
          placeholder="请选择巡检人"
          value={inspectorID}
          onChange={(v) => { setInspectorID(v === undefined || v === null ? undefined : Number(v)); setPage(1) }}
        />
        <RangePicker
          showTime
          value={range}
          placeholder={['巡检时间起', '巡检时间止']}
          onChange={(dates) => { setRange(dates); setPage(1) }}
        />
      </Space>

      <Table<InspectRecord>
        rowKey="id"
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
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => { setPage(p); setPageSize(ps) },
        }}
      />

      {/* 巡检记录详情：任务快照 + 处理结果与图片 */}
      <Modal
        className="e2e-inspect-record-modal"
        title={`巡检记录详情${detail?.id ? ` #${detail.id}` : ''}`}
        open={detailOpen}
        onCancel={() => setDetailOpen(false)}
        footer={
          <Button onClick={() => setDetailOpen(false)}>关闭</Button>
        }
        width={760}
      >
        {detail && (
          <>
            <Descriptions column={2} size="small" bordered>
              <Descriptions.Item label="任务名称" span={2}>
                {detail.task_name || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="所属区域">{detail.region?.name ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="巡检位置">{detail.location || '-'}</Descriptions.Item>
              <Descriptions.Item label="巡检人">{detail.inspector || '-'}</Descriptions.Item>
              <Descriptions.Item label="巡检时间">{formatDT(detail.plan_dt)}</Descriptions.Item>
              <Descriptions.Item label="处理时间">{formatDT(detail.handled_dt)}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <StatusTag type="inspect" status={detail.status} />
              </Descriptions.Item>
              <Descriptions.Item label="巡检内容" span={2}>
                {detail.content || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="处理结果" span={2}>
                {detail.result || '-'}
              </Descriptions.Item>
            </Descriptions>
            <div style={{ marginTop: 16 }}>
              <div style={{ marginBottom: 8 }}>处理结果图</div>
              {detail.result_image ? (
                <Image src={detail.result_image} width={200} alt="处理结果图" />
              ) : (
                <span>-</span>
              )}
            </div>
          </>
        )}
      </Modal>
    </PageContainer>
  )
}

export default InspectRecordList
