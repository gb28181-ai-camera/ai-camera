import React, { useCallback, useEffect, useState } from 'react'
import { Button, Col, Descriptions, Empty, Image, message, Modal, Row, Select, Space, Switch, Table, Tag } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { ReloadOutlined, VideoCameraOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import PageContainer from '../../components/PageContainer'
import RegionSelect from '../../components/RegionSelect'
import DeviceSelect from '../../components/DeviceSelect'
import StatusTag from '../../components/StatusTag'
import EmptyState from '../../components/EmptyState'
import { RiskController } from '../../api/generated/RiskController'
import { DeviceController } from '../../api/generated/DeviceController'
import type { RiskRecord } from '../../api/generated/definitions/RiskRecord'
import type { Device } from '../../api/generated/definitions/Device'
import { useAuthStore } from '../../store/authStore'
import { REFRESH_INTERVAL_OPTIONS, formatDT, LEVEL_LABELS } from '../../types/business'

/**
 * AI监控：选择区域与摄像头后，同屏展示实时视频画面与该设备的实时风险信息。
 *
 * - 必须先选区域与摄像头；未选择时展示引导空态；
 * - 点击风险条目打开详情弹窗，「去处理」跳转风险监控并携带风险 ID。
 */
const AiMonitor: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canMonitorSwitch = permCodes.includes('device:monitor:view')
  const navigate = useNavigate()

  const [regionID, setRegionID] = useState<number | undefined>()
  const [device, setDevice] = useState<Device | null>(null)
  const [deviceSelectOpen, setDeviceSelectOpen] = useState(false)

  const [risks, setRisks] = useState<RiskRecord[]>([])
  const [loading, setLoading] = useState(false)
  const [refreshSec, setRefreshSec] = useState(30)

  const [detailOpen, setDetailOpen] = useState(false)
  const [detail, setDetail] = useState<RiskRecord | null>(null)

  const fetchRisks = useCallback(async () => {
    if (!device?.id) {
      setRisks([])
      return
    }
    setLoading(true)
    try {
      const res = await RiskController.pageRisks({
        page: 1,
        page_size: 10,
        device_id: device.id,
        region_id: regionID,
      })
      setRisks(res.data?.items ?? [])
    } catch {
      message.error('获取该摄像头的风险信息失败')
    } finally {
      setLoading(false)
    }
  }, [device, regionID])

  useEffect(() => { void fetchRisks() }, [fetchRisks])

  // 视频窗口与风险列表按同一节奏轮询
  useEffect(() => {
    if (!device?.id || refreshSec <= 0) return
    const timer = window.setInterval(() => { void fetchRisks() }, refreshSec * 1000)
    return () => { window.clearInterval(timer) }
  }, [refreshSec, device, fetchRisks])

  /** 切换设备监控开关（是否出现在设备监控的实时视频列表中） */
  const toggleMonitor = async (enabled: boolean) => {
    if (!device?.id) return
    try {
      await DeviceController.toggleDeviceMonitor(String(device.id), { enabled })
      setDevice({ ...device, monitor_enabled: enabled })
      message.success(enabled ? '已加入实时监控' : '已移出实时监控')
    } catch {
      message.error('切换监控状态失败')
    }
  }

  /** 打开风险详情并标记消息已读 */
  const openDetail = async (row: RiskRecord) => {
    setDetail(row)
    setDetailOpen(true)
    if (row.id && !row.read) {
      try {
        await RiskController.markRiskRead(row.id)
        void fetchRisks()
      } catch {
        /* 已读失败不阻塞 */
      }
    }
  }

  const columns: ColumnsType<RiskRecord> = [
    { title: '风险类型', dataIndex: ['risk_type', 'name'], width: 120, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    {
      title: '风险图片',
      dataIndex: 'image_url',
      width: 76,
      render: (value: unknown) =>
        value ? (
          <Image src={String(value)} width={48} height={48} style={{ objectFit: 'cover' }} preview={false} />
        ) : (
          <span>-</span>
        ),
    },
    { title: '发生时间', dataIndex: 'occurred_dt', width: 160, render: (v?: string) => formatDT(v) },
    {
      title: '等级',
      dataIndex: 'risk_level',
      width: 70,
      render: (v?: string) => (v ? <Tag>{LEVEL_LABELS[v] ?? v}</Tag> : '-'),
    },
    { title: '状态', dataIndex: 'status', width: 90, render: (v?: string) => <StatusTag type="risk" status={v} /> },
  ]

  const streamUrl = device?.stream_url ?? ''
  const offline = device?.status === 'offline'

  return (
    <PageContainer
      title="AI监控"
      subTitle="选择区域与摄像头，实时视频与该设备的 AI 风险信息同屏联动"
      extra={
        <Space>
          <span style={{ color: '#888' }}>刷新频率</span>
          <Select
            style={{ width: 140 }}
            value={refreshSec}
            options={REFRESH_INTERVAL_OPTIONS.filter((item) => item.value > 0)}
            onChange={(v: number) => setRefreshSec(v)}
          />
          <Button icon={<ReloadOutlined />} onClick={() => void fetchRisks()}>
            刷新
          </Button>
        </Space>
      }
    >
      <Space wrap style={{ marginBottom: 16 }}>
        <RegionSelect
          style={{ width: 200 }}
          value={regionID}
          onChange={(v) => {
            setRegionID(v === undefined || v === null ? undefined : Number(v))
            // 切换区域时清空已选摄像头，避免区域与设备不匹配
            setDevice(null)
          }}
        />
        <Button
          type="primary"
          icon={<VideoCameraOutlined />}
          disabled={!regionID}
          onClick={() => setDeviceSelectOpen(true)}
        >
          {device ? `摄像头：${device.name ?? ''}` : '选择摄像头'}
        </Button>
        {device ? (
          <>
            <span style={{ color: '#888' }}>
              {device.device_code ? `编号 ${device.device_code}` : ''} {offline ? '（离线）' : ''}
            </span>
            {canMonitorSwitch && (
              <span>
                加入监控
                <Switch
                  style={{ marginLeft: 8 }}
                  checked={Boolean(device.monitor_enabled)}
                  onChange={(checked) => void toggleMonitor(checked)}
                />
              </span>
            )}
          </>
        ) : null}
      </Space>

      <Row gutter={16}>
        <Col xs={24} lg={14}>
          <div
            style={{
              position: 'relative',
              background: '#000',
              borderRadius: 8,
              minHeight: 360,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              overflow: 'hidden',
            }}
          >
            {device && streamUrl && !offline ? (
              <video
                src={streamUrl}
                autoPlay
                muted
                controls
                playsInline
                style={{ width: '100%', maxHeight: 480, display: 'block' }}
              />
            ) : (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={
                  <span style={{ color: '#bfbfbf' }}>
                    {!device
                      ? '请先选择区域与摄像头'
                      : offline
                        ? '设备离线，无法播放实时视频'
                        : '未配置视频流地址（无信号）'}
                  </span>
                }
              />
            )}
            {device ? (
              <div
                style={{
                  position: 'absolute', left: 12, top: 12, color: '#fff',
                  background: 'rgba(0,0,0,0.45)', padding: '2px 8px', borderRadius: 4,
                }}
              >
                {device.name ?? ''}
              </div>
            ) : null}
          </div>
        </Col>

        <Col xs={24} lg={10}>
          <div style={{ marginBottom: 8, fontWeight: 500 }}>实时风险信息</div>
          {device ? (
            <Table<RiskRecord>
              rowKey="id"
              size="small"
              loading={loading}
              columns={columns}
              dataSource={risks}
              scroll={{ x: 'max-content', y: 420 }}
              pagination={false}
              onRow={(row) => ({ onClick: () => void openDetail(row), style: { cursor: 'pointer' } })}
            />
          ) : (
            <EmptyState description="请先选择区域与摄像头" />
          )}
        </Col>
      </Row>

      {/* 风险详情：可跳转风险监控页完成检测确认与处理 */}
      <Modal
        className="e2e-risk-detail-modal"
        title={`风险详情${detail?.id ? ` #${detail.id}` : ''}`}
        open={detailOpen}
        onCancel={() => setDetailOpen(false)}
        footer={
          <Space>
            <Button onClick={() => setDetailOpen(false)}>关闭</Button>
            <Button
              type="primary"
              disabled={!detail?.id}
              onClick={() => {
                setDetailOpen(false)
                navigate(`/risk/monitor?risk_id=${detail?.id ?? ''}`)
              }}
            >
              去处理
            </Button>
          </Space>
        }
        width={760}
      >
        {detail && (
          <>
            <Descriptions column={2} size="small" bordered>
              <Descriptions.Item label="风险类型">{detail.risk_type?.name ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="风险等级">{detail.risk_level ? LEVEL_LABELS[detail.risk_level] ?? detail.risk_level : '-'}</Descriptions.Item>
              <Descriptions.Item label="设备名称">{detail.device?.name ?? device?.name ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="设备编号">{detail.device_code || device?.device_code || '-'}</Descriptions.Item>
              <Descriptions.Item label="所属区域">{detail.region?.name ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="设备位置">{detail.location || device?.location || '-'}</Descriptions.Item>
              <Descriptions.Item label="发生时间">{formatDT(detail.occurred_dt)}</Descriptions.Item>
              <Descriptions.Item label="AI置信度">
                {detail.detect_score === undefined || detail.detect_score === null
                  ? '-'
                  : `${(detail.detect_score * 100).toFixed(1)}%`}
              </Descriptions.Item>
              <Descriptions.Item label="检测准确">
                {detail.detect_accurate === 'yes' ? '是' : detail.detect_accurate === 'no' ? '否' : '待确认'}
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <StatusTag type="risk" status={detail.status} />
              </Descriptions.Item>
            </Descriptions>
            {detail.image_url ? (
              <div style={{ marginTop: 16 }}>
                <div style={{ marginBottom: 8 }}>风险图片</div>
                <Image src={detail.image_url} width={200} alt="风险图片" />
              </div>
            ) : null}
          </>
        )}
      </Modal>

      {/* 摄像头选择：复用公共设备选择器（Modal 类名 e2e-device-select） */}
      <DeviceSelect
        open={deviceSelectOpen}
        regionId={regionID}
        onCancel={() => setDeviceSelectOpen(false)}
        onOk={(rows) => {
          setDevice(rows[0] ?? null)
          setDeviceSelectOpen(false)
        }}
      />
    </PageContainer>
  )
}

export default AiMonitor
