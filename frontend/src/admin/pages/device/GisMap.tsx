import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { Button, Card, Col, Descriptions, message, Popover, Row, Space, Statistic, Tag } from 'antd'
import { EnvironmentOutlined, ReloadOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import PageContainer from '../../components/PageContainer'
import RegionSelect from '../../components/RegionSelect'
import EmptyState from '../../components/EmptyState'
import StatusTag from '../../components/StatusTag'
import { DeviceController } from '../../api/generated/DeviceController'
import type { Device } from '../../api/generated/definitions/Device'
import { formatDT, gaugeColor } from '../../types/business'

/** 归一化后的打点坐标（百分比） */
interface MapPoint {
  device: Device
  left: number
  top: number
}

/** 判断设备是否具备有效经纬度 */
function hasCoord(device: Device): boolean {
  const lat = Number(device.latitude)
  const lng = Number(device.longitude)
  return Number.isFinite(lat) && Number.isFinite(lng) && lat !== 0 && lng !== 0
}

/**
 * GIS查看：设备地理分布。
 *
 * 不引入地图库 —— 用 CSS 网格底板 + 按经纬度归一化到百分比坐标的绝对定位图标呈现，
 * 悬停图标可查看设备信息并直达风险监控。
 */
const GisMap: React.FC = () => {
  const navigate = useNavigate()
  const [regionID, setRegionID] = useState<number | undefined>()
  const [devices, setDevices] = useState<Device[]>([])
  const [loading, setLoading] = useState(false)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      // GIS 视图需一次性拿到全部打点数据，page_size 取上限 200
      const res = await DeviceController.pageDevices(1, 200, undefined, regionID)
      setDevices(res.data?.items ?? [])
    } catch {
      message.error('获取设备分布数据失败')
    } finally {
      setLoading(false)
    }
  }, [regionID])

  useEffect(() => { void fetchData() }, [fetchData])

  const located = useMemo(() => devices.filter(hasCoord), [devices])

  /** 按经纬度范围归一化到 5% ~ 95% 的百分比坐标 */
  const points = useMemo<MapPoint[]>(() => {
    if (located.length === 0) return []
    const lats = located.map((d) => Number(d.latitude))
    const lngs = located.map((d) => Number(d.longitude))
    const minLat = Math.min(...lats)
    const maxLat = Math.max(...lats)
    const minLng = Math.min(...lngs)
    const maxLng = Math.max(...lngs)
    const latSpan = maxLat - minLat || 1
    const lngSpan = maxLng - minLng || 1
    return located.map((device) => {
      const lat = Number(device.latitude)
      const lng = Number(device.longitude)
      // 纬度越大越靠北 → 屏幕坐标越小
      const top = 5 + ((maxLat - lat) / latSpan) * 90
      const left = 5 + ((lng - minLng) / lngSpan) * 90
      return { device, left, top }
    })
  }, [located])

  const onlineCount = located.filter((d) => d.status === 'online').length

  /** 点击气泡中的「查看风险」→ 跳转风险监控并携带设备筛选 */
  const gotoRisk = (device: Device) => {
    const params = new URLSearchParams()
    if (device.id) params.set('device_id', String(device.id))
    if (device.name) params.set('device_name', device.name)
    navigate(`/risk/monitor?${params.toString()}`)
  }

  const popoverContent = (device: Device) => (
    <div style={{ width: 300 }}>
      <Descriptions column={1} size="small" bordered>
        <Descriptions.Item label="设备名称">{device.name || '-'}</Descriptions.Item>
        <Descriptions.Item label="设备编号">{device.device_code || '-'}</Descriptions.Item>
        <Descriptions.Item label="所属区域">{device.region?.name ?? '-'}</Descriptions.Item>
        <Descriptions.Item label="设备位置">{device.location || '-'}</Descriptions.Item>
        <Descriptions.Item label="运行状态">
          <StatusTag type="device" status={device.status} />
        </Descriptions.Item>
        <Descriptions.Item label="电量">
          <Tag color={gaugeColor(device.battery ?? 0)}>{device.battery ?? 0}%</Tag>
        </Descriptions.Item>
        <Descriptions.Item label="最近心跳">{formatDT(device.last_beat_dt)}</Descriptions.Item>
      </Descriptions>
      <Button type="primary" size="small" style={{ marginTop: 12 }} onClick={() => gotoRisk(device)}>
        查看风险
      </Button>
    </div>
  )

  return (
    <PageContainer
      title="GIS查看"
      subTitle="按经纬度展示设备地理分布，悬停图标查看设备信息"
      extra={
        <Button icon={<ReloadOutlined />} onClick={() => void fetchData()} loading={loading}>
          刷新
        </Button>
      }
    >
      <Row gutter={16}>
        <Col xs={24} lg={5}>
          <Card size="small" title="筛选" style={{ marginBottom: 16 }}>
            <Space direction="vertical" style={{ width: '100%' }}>
              <RegionSelect
                style={{ width: '100%' }}
                value={regionID}
                onChange={(v) => setRegionID(v === undefined || v === null ? undefined : Number(v))}
              />
              <Statistic title="设备总数" value={devices.length} />
              <Statistic title="已定位" value={located.length} />
              <Statistic title="在线设备" value={onlineCount} valueStyle={{ color: '#52c41a' }} />
            </Space>
          </Card>
        </Col>

        <Col xs={24} lg={19}>
          <div
            style={{
              position: 'relative',
              width: '100%',
              height: 560,
              borderRadius: 8,
              overflow: 'hidden',
              backgroundColor: '#0b1b34',
              backgroundImage:
                'linear-gradient(rgba(22,119,255,0.14) 1px, transparent 1px), linear-gradient(90deg, rgba(22,119,255,0.14) 1px, transparent 1px)',
              backgroundSize: '48px 48px',
            }}
          >
            {points.map(({ device, left, top }) => (
              <Popover
                key={device.id ?? `${device.device_code}-${left}-${top}`}
                content={popoverContent(device)}
                title={device.name ?? '设备'}
                trigger="hover"
                placement="top"
              >
                <EnvironmentOutlined
                  style={{
                    position: 'absolute',
                    left: `${left}%`,
                    top: `${top}%`,
                    transform: 'translate(-50%, -50%)',
                    fontSize: 24,
                    color: device.status === 'online' ? '#52c41a' : '#8c8c8c',
                    cursor: 'pointer',
                  }}
                />
              </Popover>
            ))}

            {points.length === 0 ? (
              <div
                style={{
                  position: 'absolute', inset: 0, display: 'flex',
                  alignItems: 'center', justifyContent: 'center',
                }}
              >
                <EmptyState
                  description={
                    <span style={{ color: '#bfbfbf' }}>
                      {devices.length === 0
                        ? '暂无设备数据，请先选择区域或前往设备管理新增设备'
                        : '所选区域内暂无设备配置经纬度，无法在地图上定位'}
                    </span>
                  }
                />
              </div>
            ) : null}

            <div style={{ position: 'absolute', right: 12, bottom: 12, color: '#7f9bc4', fontSize: 12 }}>
              图标位置由设备经纬度归一化计算，非真实底图
            </div>
          </div>
        </Col>
      </Row>
    </PageContainer>
  )
}

export default GisMap
