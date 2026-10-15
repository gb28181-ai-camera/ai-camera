import React, { useEffect, useMemo, useState } from 'react'
import { Button, Radio, Space, Tag } from 'antd'
import { CloseOutlined, VideoCameraOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import RegionSelect from '../../components/RegionSelect'
import DeviceSelect from '../../components/DeviceSelect'
import EmptyState from '../../components/EmptyState'
import StatusTag from '../../components/StatusTag'
import type { Device } from '../../api/generated/definitions/Device'

/** 分屏布局：窗口数量 */
type LayoutSize = 1 | 4 | 9

const LAYOUT_OPTIONS: { label: string; value: LayoutSize }[] = [
  { label: '单屏', value: 1 },
  { label: '4 宫格', value: 4 },
  { label: '9 宫格', value: 9 },
]

/**
 * 实时监控：单屏 / 4 宫格 / 9 宫格视频墙。
 *
 * - 必须先选择区域与摄像头才会显示视频画面；
 * - 离线设备窗口显示「设备离线」提示，未配置流地址显示「无信号」。
 */
const RealtimeMonitor: React.FC = () => {
  const [regionID, setRegionID] = useState<number | undefined>()
  const [layout, setLayout] = useState<LayoutSize>((Number(localStorage.getItem('monitor_layout')) as LayoutSize) || 4)
  const [devices, setDevices] = useState<Device[]>([])
  const [deviceSelectOpen, setDeviceSelectOpen] = useState(false)

  // 布局持久化，刷新后保持上次选择
  useEffect(() => {
    localStorage.setItem('monitor_layout', String(layout))
  }, [layout])

  /** 切换布局时按顺序保留前面的摄像头，超出窗口数量的自动释放 */
  const windows = useMemo(() => Array.from({ length: layout }, (_, index) => devices[index] ?? null), [layout, devices])

  const removeWindow = (index: number) => {
    setDevices((prev) => prev.filter((_, i) => i !== index))
  }

  const columns = layout === 1 ? 1 : layout === 4 ? 2 : 3

  return (
    <PageContainer
      title="实时监控"
      subTitle="多路摄像头的实时视频墙，支持单屏 / 4 宫格 / 9 宫格"
      extra={
        <Radio.Group
          value={layout}
          optionType="button"
          buttonStyle="solid"
          options={LAYOUT_OPTIONS}
          onChange={(e) => setLayout(e.target.value as LayoutSize)}
        />
      }
    >
      <Space wrap style={{ marginBottom: 16 }}>
        <RegionSelect
          style={{ width: 200 }}
          value={regionID}
          onChange={(v) => {
            setRegionID(v === undefined || v === null ? undefined : Number(v))
            // 切换区域后已选摄像头可能不属于该区域，直接清空
            setDevices([])
          }}
        />
        <Button
          type="primary"
          icon={<VideoCameraOutlined />}
          disabled={!regionID}
          onClick={() => setDeviceSelectOpen(true)}
        >
          {devices.length > 0 ? `已选 ${devices.length} 路摄像头` : '选择摄像头'}
        </Button>
        {!regionID ? <span style={{ color: '#888' }}>请先选择区域</span> : null}
      </Space>

      {devices.length === 0 ? (
        <EmptyState description="请先选择区域与摄像头，随后才会显示视频监控画面" />
      ) : (
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
            gap: 8,
          }}
        >
          {windows.map((device, index) => (
            <div
              key={device ? `device-${device.id ?? index}` : `empty-${index}`}
              style={{
                position: 'relative',
                background: '#000',
                borderRadius: 6,
                height: layout === 1 ? 480 : layout === 4 ? 280 : 200,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                overflow: 'hidden',
              }}
            >
              {device ? (
                device.stream_url && device.status !== 'offline' ? (
                  <video
                    src={device.stream_url}
                    autoPlay
                    muted
                    controls
                    playsInline
                    style={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }}
                  />
                ) : (
                  <div style={{ color: '#bfbfbf', textAlign: 'center' }}>
                    <div>{device.status === 'offline' ? '设备离线' : '无信号'}</div>
                    <div style={{ marginTop: 4, fontSize: 12 }}>{device.name ?? ''}</div>
                  </div>
                )
              ) : (
                <span style={{ color: '#595959' }}>空闲窗口</span>
              )}

              {device ? (
                <>
                  <div style={{ position: 'absolute', left: 8, top: 8, display: 'flex', gap: 8, alignItems: 'center' }}>
                    <Tag color="blue">{device.name ?? '-'}</Tag>
                    <StatusTag type="device" status={device.status} />
                  </div>
                  <Button
                    size="small"
                    type="text"
                    icon={<CloseOutlined style={{ color: '#fff' }} />}
                    style={{ position: 'absolute', right: 4, top: 4 }}
                    onClick={() => removeWindow(index)}
                  />
                </>
              ) : null}
            </div>
          ))}
        </div>
      )}

      {/* 摄像头多选：复用公共设备选择器 */}
      <DeviceSelect
        open={deviceSelectOpen}
        mode="multiple"
        regionId={regionID}
        title="选择摄像头（可多选）"
        onCancel={() => setDeviceSelectOpen(false)}
        onOk={(rows) => {
          setDevices(rows.slice(0, layout))
          setDeviceSelectOpen(false)
        }}
      />
    </PageContainer>
  )
}

export default RealtimeMonitor
