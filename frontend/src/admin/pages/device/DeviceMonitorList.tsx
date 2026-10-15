import React, { useCallback, useEffect, useState } from 'react'
import { Button, message, Progress, Space, Table, Tooltip } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { HistoryOutlined, VideoCameraOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import RegionSelect from '../../components/RegionSelect'
import StatusTag from '../../components/StatusTag'
import VideoPlayer from '../../components/VideoPlayer'
import { DeviceController } from '../../api/generated/DeviceController'
import { RiskController } from '../../api/generated/RiskController'
import type { Device } from '../../api/generated/definitions/Device'
import { useAuthStore } from '../../store/authStore'
import {
  CLOSED_VIDEO, DEVICE_STATUS_OPTIONS, formatDT, formatStorage, gaugeColor, usagePercent,
} from '../../types/business'
import type { VideoState } from '../../types/business'

/**
 * 设备监控：设备运行指标（电量 / 存储 / 在线离线时间）与实时视频、录像入口。
 *
 * - 「设备名称」点击跳转风险监控，并按该设备筛选风险数据；
 * - 「设备编号」列支持排序；
 * - 设备离线时禁用实时视频按钮。
 */
const DeviceMonitorList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canVideo = permCodes.includes('device:monitor:video')
  const navigate = useNavigate()

  const [data, setData] = useState<Device[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState<string | undefined>()
  const [regionID, setRegionID] = useState<number | undefined>()

  const [video, setVideo] = useState<VideoState>(CLOSED_VIDEO)

  // 跳转带入的 device_id / device_name（来自风险监控、GIS 查看等页面）
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const deviceID = params.get('device_id')
    const deviceName = params.get('device_name')
    if (deviceID || deviceName) {
      setKeyword(deviceName || deviceID || '')
      setPage(1)
    }
  }, [])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await DeviceController.pageDevices(page, pageSize, keyword || undefined, regionID, status)
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch {
      message.error('获取设备列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, regionID, status])

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
    setPage(1)
  }

  /** 点击设备名称 → 跳转风险监控并按该设备筛选 */
  const gotoRiskMonitor = (row: Device) => {
    const params = new URLSearchParams()
    if (row.id) params.set('device_id', String(row.id))
    if (row.name) params.set('device_name', row.name)
    navigate(`/risk/monitor?${params.toString()}`)
  }

  /** 查看实时视频：离线设备禁止播放 */
  const openLive = (row: Device) => {
    if (row.status === 'offline') {
      message.warning('设备离线，无法播放实时视频')
      return
    }
    if (!row.stream_url) {
      message.warning('未配置视频流地址')
      return
    }
    setVideo({ open: true, url: row.stream_url, title: `${row.name ?? ''} 实时视频`, live: true })
  }

  /** 查看录像：取该设备最近一条带录像的风险记录 */
  const openRecord = async (row: Device) => {
    if (!row.id) return
    try {
      const res = await RiskController.pageRisks({ page: 1, page_size: 20, device_id: row.id })
      const record = (res.data?.items ?? []).find((item) => Boolean(item.video_url))
      if (!record?.video_url) {
        message.warning('该设备暂无录像文件')
        return
      }
      setVideo({ open: true, url: record.video_url, title: `${row.name ?? ''} 历史录像` })
    } catch {
      message.error('获取录像失败')
    }
  }

  const columns: ColumnsType<Device> = [
    {
      title: '设备名称',
      dataIndex: 'name',
      width: 170,
      ellipsis: true,
      render: (_: unknown, row) => <a onClick={() => gotoRiskMonitor(row)}>{row.name ?? '-'}</a>,
    },
    {
      title: '设备编号',
      dataIndex: 'device_code',
      width: 160,
      ellipsis: true,
      sorter: (a, b) => String(a.device_code ?? '').localeCompare(String(b.device_code ?? '')),
      render: (v?: string) => v || '-',
    },
    { title: '所属区域', dataIndex: ['region', 'name'], width: 140, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '设备位置', dataIndex: 'location', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '电量',
      dataIndex: 'battery',
      width: 150,
      render: (value?: number) => (
        <Progress
          percent={value ?? 0}
          size="small"
          strokeColor={gaugeColor(value ?? 0)}
          format={(p) => `${p ?? 0}%`}
        />
      ),
    },
    {
      title: '存储容量',
      dataIndex: 'storage_free',
      width: 190,
      render: (_: unknown, row) => {
        const used = (row.storage_total ?? 0) - (row.storage_free ?? 0)
        const percent = usagePercent(used, row.storage_total)
        return (
          <Tooltip title={`总容量 ${formatStorage(row.storage_total)} / 剩余 ${formatStorage(row.storage_free)}`}>
            <Progress percent={percent} size="small" strokeColor={gaugeColor(100 - percent)} format={() => formatStorage(row.storage_free)} />
          </Tooltip>
        )
      },
    },
    {
      title: '在线-离线时间',
      dataIndex: 'online_dt',
      width: 230,
      render: (_: unknown, row) => (
        <div style={{ lineHeight: 1.6 }}>
          <div>上线：{formatDT(row.online_dt)}</div>
          <div>离线：{formatDT(row.offline_dt)}</div>
        </div>
      ),
    },
    {
      title: '运行状态',
      dataIndex: 'status',
      width: 100,
      render: (v?: string) => <StatusTag type="device" status={v} />,
    },
    {
      title: '实时视频',
      key: 'live',
      width: 130,
      render: (_: unknown, row) => (
        <Button
          type="link"
          size="small"
          icon={<VideoCameraOutlined />}
          disabled={!canVideo || row.status === 'offline'}
          onClick={() => openLive(row)}
        >
          查看实时视频
        </Button>
      ),
    },
    {
      title: '视频录像',
      key: 'record',
      width: 120,
      render: (_: unknown, row) => (
        <Button type="link" size="small" icon={<HistoryOutlined />} disabled={!canVideo} onClick={() => void openRecord(row)}>
          查看录像
        </Button>
      ),
    },
  ]

  return (
    <PageContainer title="设备监控" subTitle="设备运行指标与实时视频、录像入口">
      <SearchBar
        fields={[
          { name: 'keyword', label: '设备名称/编号', placeholder: '请输入设备名称或编号' },
          { name: 'status', label: '运行状态', type: 'select', options: DEVICE_STATUS_OPTIONS },
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
      </Space>

      <Table<Device>
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

      <VideoPlayer
        open={video.open}
        src={video.url}
        title={video.title}
        live={video.live}
        onCancel={() => setVideo(CLOSED_VIDEO)}
      />
    </PageContainer>
  )
}

export default DeviceMonitorList
