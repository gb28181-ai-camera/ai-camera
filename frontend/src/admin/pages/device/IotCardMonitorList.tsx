import React, { useCallback, useEffect, useState } from 'react'
import { Button, message, Progress, Space, Table, Tag, Tooltip } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { ClearOutlined, DesktopOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import RegionSelect from '../../components/RegionSelect'
import DeviceSelect from '../../components/DeviceSelect'
import StatusTag from '../../components/StatusTag'
import { DeviceController } from '../../api/generated/DeviceController'
import type { IotCard } from '../../api/generated/definitions/IotCard'
import { IOT_CARD_STATUS_OPTIONS, formatDT, formatTraffic, gaugeColor, usagePercent } from '../../types/business'

/**
 * 物联网卡监控：只读的流量监控视图（增删改在资源管理 → 物联网卡管理中完成）。
 *
 * 流量口径：总额 `traffic_total` / 已用 `traffic_used` / 剩余 `traffic_left`，单位 MB。
 */
const IotCardMonitorList: React.FC = () => {
  const [data, setData] = useState<IotCard[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState<string | undefined>()
  const [regionID, setRegionID] = useState<number | undefined>()
  const [deviceID, setDeviceID] = useState<number | undefined>()
  const [deviceName, setDeviceName] = useState('')
  const [deviceSelectOpen, setDeviceSelectOpen] = useState(false)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await DeviceController.pageIotCards(
        page,
        pageSize,
        keyword || undefined,
        regionID,
        deviceID,
        status
      )
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch {
      message.error('获取物联网卡列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, regionID, deviceID, status])

  useEffect(() => { void fetchData() }, [fetchData])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setStatus(values.status || undefined)
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setStatus(undefined)
    setRegionID(undefined)
    setDeviceID(undefined)
    setDeviceName('')
    setPage(1)
  }

  const columns: ColumnsType<IotCard> = [
    { title: '物联网卡名称', dataIndex: 'name', width: 170, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '物联网卡号', dataIndex: 'card_no', width: 200, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '设备编号',
      dataIndex: ['device', 'device_code'],
      width: 150,
      ellipsis: true,
      render: (v: unknown) => (v ? String(v) : '-'),
    },
    { title: '所属区域', dataIndex: ['region', 'name'], width: 140, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '所属设备', dataIndex: ['device', 'name'], width: 160, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    {
      title: '流量总额',
      dataIndex: 'traffic_total',
      width: 120,
      render: (v?: number) => formatTraffic(v),
    },
    {
      title: '剩余流量',
      dataIndex: 'traffic_left',
      width: 120,
      render: (v?: number) => formatTraffic(v),
    },
    {
      title: '使用流量',
      dataIndex: 'traffic_used',
      width: 190,
      render: (_: unknown, row) => {
        const percent = usagePercent(row.traffic_used, row.traffic_total)
        return (
          <Tooltip title={`已用 ${formatTraffic(row.traffic_used)} / 总额 ${formatTraffic(row.traffic_total)}`}>
            <Progress
              percent={percent}
              size="small"
              strokeColor={gaugeColor(100 - percent)}
              format={() => formatTraffic(row.traffic_used)}
            />
          </Tooltip>
        )
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (v?: string) => <StatusTag type="iotcard" status={v} />,
    },
    { title: '统计时间', dataIndex: 'stat_dt', width: 170, render: (v?: string) => formatDT(v) },
    {
      title: '到期时间',
      dataIndex: 'expire_dt',
      width: 170,
      render: (v?: string) => {
        if (!v) return '-'
        const expired = new Date(v).getTime() < Date.now()
        return <Tag color={expired ? 'red' : 'default'}>{formatDT(v, 'YYYY-MM-DD')}</Tag>
      },
    },
  ]

  return (
    <PageContainer title="物联网卡监控" subTitle="物联网卡流量总额、已用与剩余流量监控（只读）">
      <SearchBar
        fields={[
          { name: 'keyword', label: '物联网卡', placeholder: '请输入物联网卡名称/卡号' },
          { name: 'status', label: '状态', type: 'select', options: IOT_CARD_STATUS_OPTIONS },
        ]}
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
        <Button
          icon={<DesktopOutlined />}
          onClick={() => setDeviceSelectOpen(true)}
        >
          {deviceName || '选择所属设备'}
        </Button>
        {deviceID ? (
          <Button
            icon={<ClearOutlined />}
            onClick={() => { setDeviceID(undefined); setDeviceName(''); setPage(1) }}
          >
            清除设备筛选
          </Button>
        ) : null}
      </Space>

      <Table<IotCard>
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

      {/* 所属设备筛选：复用公共设备选择器 */}
      <DeviceSelect
        open={deviceSelectOpen}
        regionId={regionID}
        onCancel={() => setDeviceSelectOpen(false)}
        onOk={(rows) => {
          const picked = rows[0]
          setDeviceID(picked?.id ?? undefined)
          setDeviceName(picked?.name ?? '')
          setDeviceSelectOpen(false)
          setPage(1)
        }}
      />
    </PageContainer>
  )
}

export default IotCardMonitorList
