import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Button, Card, Col, message, Row, Space, Statistic } from 'antd'
import { EnvironmentOutlined, ReloadOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import * as L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import PageContainer from '../../components/PageContainer'
import RegionSelect from '../../components/RegionSelect'
import EmptyState from '../../components/EmptyState'
import { DeviceController } from '../../api/generated/DeviceController'
import type { Device } from '../../api/generated/definitions/Device'
import { formatDT } from '../../types/business'

/**
 * 瓦片地址常量：以后换源（天地图 / 高德 / 内网离线瓦片）只需改这里。
 * 注意：更换为需要 key 的厂商时，需同时同步 attribution 与 maxZoom。
 */
const TILE_URL_TEMPLATE = 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png'
const TILE_ATTRIBUTION =
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
const TILE_MAX_ZOOM = 19

/** 无坐标设备时的默认视野（全国范围） */
const DEFAULT_CENTER: L.LatLngExpression = [34.5, 108.9]
const DEFAULT_ZOOM = 4

/** 判断设备是否具备有效经纬度（0 视为未配置） */
function hasCoord(device: Device): boolean {
  const lat = Number(device.latitude)
  const lng = Number(device.longitude)
  return Number.isFinite(lat) && Number.isFinite(lng) && lat !== 0 && lng !== 0
}

/**
 * 拼接弹窗 DOM。
 *
 * 用原生 DOM + textContent 填充，避免把设备名称/地址等用户可填字段当 HTML 注入；
 * 「查看风险」按钮绑定原生事件，直接复用 React Router 的 navigate。
 */
function buildPopupNode(device: Device, onGotoRisk: (device: Device) => void): HTMLElement {
  const root = document.createElement('div')
  root.style.minWidth = '220px'

  const rows: Array<[string, string]> = [
    ['设备编号', device.device_code || '-'],
    ['所属区域', device.region?.name ?? '-'],
    ['设备位置', device.location || '-'],
    ['运行状态', device.status === 'online' ? '在线' : device.status || '-'],
    ['电量', `${device.battery ?? 0}%`],
    ['最近心跳', formatDT(device.last_beat_dt)],
  ]

  rows.forEach(([label, value]) => {
    const row = document.createElement('div')
    row.style.display = 'flex'
    row.style.gap = '8px'
    row.style.fontSize = '12px'
    row.style.lineHeight = '20px'

    const labelCell = document.createElement('span')
    labelCell.textContent = label
    labelCell.style.color = '#8c8c8c'
    labelCell.style.flex = '0 0 64px'

    const valueCell = document.createElement('span')
    valueCell.textContent = value
    valueCell.style.color = '#262626'
    valueCell.style.flex = '1'
    valueCell.style.wordBreak = 'break-all'

    row.appendChild(labelCell)
    row.appendChild(valueCell)
    root.appendChild(row)
  })

  const action = document.createElement('button')
  action.type = 'button'
  action.textContent = '查看风险'
  action.className = 'ant-btn ant-btn-primary ant-btn-sm'
  action.style.marginTop = '10px'
  action.addEventListener('click', () => onGotoRisk(device))
  root.appendChild(action)

  return root
}

/**
 * GIS 查看：基于 Leaflet + OpenStreetMap 展示设备地理分布。
 *
 * - 只渲染具备有效经纬度的设备，未配置坐标的设备不在地图上出现；
 * - 标记颜色区分在线 / 离线，点击弹出设备信息并可直达风险监控；
 * - 容器尺寸变化时自动 `invalidateSize()`，卸载时 `map.remove()` 释放资源；
 * - 瓦片地址抽成常量，便于切换到其他瓦片源。
 */
const GisMap: React.FC = () => {
  const navigate = useNavigate()
  const [regionID, setRegionID] = useState<number | undefined>()
  const [devices, setDevices] = useState<Device[]>([])
  const [loading, setLoading] = useState(false)

  const containerRef = useRef<HTMLDivElement | null>(null)
  const mapRef = useRef<L.Map | null>(null)
  const markerLayerRef = useRef<L.LayerGroup | null>(null)
  /** 已适配过视野的区域标识，避免每次刷新都打断用户的手动缩放 */
  const fittedKeyRef = useRef('')

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

  const onlineCount = located.filter((d) => d.status === 'online').length

  /** 点击气泡中的「查看风险」→ 跳转风险监控并携带设备筛选 */
  const gotoRisk = useCallback(
    (device: Device) => {
      const params = new URLSearchParams()
      if (device.id) params.set('device_id', String(device.id))
      if (device.name) params.set('device_name', device.name)
      navigate(`/risk/monitor?${params.toString()}`)
    },
    [navigate],
  )

  /* 初始化地图（只执行一次） */
  useEffect(() => {
    const container = containerRef.current
    if (!container || mapRef.current) return undefined

    const map = L.map(container, {
      center: DEFAULT_CENTER,
      zoom: DEFAULT_ZOOM,
      zoomControl: true,
      attributionControl: true,
    })
    L.tileLayer(TILE_URL_TEMPLATE, {
      attribution: TILE_ATTRIBUTION,
      maxZoom: TILE_MAX_ZOOM,
    }).addTo(map)

    mapRef.current = map
    markerLayerRef.current = L.layerGroup().addTo(map)

    // 容器自适应：布局变化（侧边栏折叠、窗口缩放）时重算尺寸
    const resizeObserver = new ResizeObserver(() => {
      map.invalidateSize()
    })
    resizeObserver.observe(container)

    return () => {
      resizeObserver.disconnect()
      map.remove()
      mapRef.current = null
      markerLayerRef.current = null
      fittedKeyRef.current = ''
    }
  }, [])

  /* 数据变化时重建标记 */
  useEffect(() => {
    const map = mapRef.current
    const layer = markerLayerRef.current
    if (!map || !layer) return

    layer.clearLayers()
    if (located.length === 0) return

    const bounds = L.latLngBounds([])
    located.forEach((device) => {
      const lat = Number(device.latitude)
      const lng = Number(device.longitude)
      const online = device.status === 'online'

      const marker = L.circleMarker([lat, lng], {
        radius: 7,
        color: '#ffffff',
        weight: 2,
        fillColor: online ? '#52c41a' : '#8c8c8c',
        fillOpacity: 1,
      })
      marker.bindTooltip(device.name || device.device_code || '设备', { direction: 'top', offset: [0, -6] })
      marker.bindPopup(buildPopupNode(device, gotoRisk))
      marker.addTo(layer)
      bounds.extend([lat, lng])
    })

    // 仅在区域或点位数量变化时适配视野，避免刷新打断用户操作
    const fitKey = `${regionID ?? 'all'}-${located.length}`
    if (fitKey !== fittedKeyRef.current) {
      fittedKeyRef.current = fitKey
      if (located.length === 1) {
        map.setView(bounds.getCenter(), 14)
      } else {
        map.fitBounds(bounds.pad(0.2), { maxZoom: 15 })
      }
    }
  }, [located, gotoRisk, regionID])

  return (
    <PageContainer
      title="GIS查看"
      subTitle="按经纬度在 OpenStreetMap 上展示设备地理分布，点击图标查看设备信息"
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
              background: '#0b1b34',
            }}
          >
            <div
              ref={containerRef}
              className="e2e-gis-map"
              style={{ width: '100%', height: '100%' }}
            />

            {located.length === 0 ? (
              <div
                style={{
                  position: 'absolute', inset: 0, display: 'flex',
                  alignItems: 'center', justifyContent: 'center',
                  background: 'rgba(255,255,255,0.92)',
                  zIndex: 500,
                }}
              >
                <EmptyState
                  description={
                    <span style={{ color: '#595959' }}>
                      {devices.length === 0
                        ? '暂无设备数据，请先选择区域或前往设备管理新增设备'
                        : '所选区域内暂无设备配置经纬度，无法在地图上定位'}
                    </span>
                  }
                />
              </div>
            ) : null}
          </div>

          <div style={{ marginTop: 8, color: '#8c8c8c', fontSize: 12 }}>
            <EnvironmentOutlined style={{ color: '#52c41a', marginRight: 6 }} />
            绿色为在线设备，灰色为离线设备；未配置经纬度的设备不在图上显示。
          </div>
        </Col>
      </Row>
    </PageContainer>
  )
}

export default GisMap
