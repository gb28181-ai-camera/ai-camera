import React, { useCallback, useEffect, useState } from 'react'
import { Alert, Button, Card, Col, Descriptions, Empty, Row, Space, Spin, Tag, message } from 'antd'
import { InfoCircleOutlined, ReloadOutlined } from '@ant-design/icons'
import axios from 'axios'
import PageContainer from '../../components/PageContainer'
import { GbOpsController } from '../../api/generated/GbOpsController'
import type { GbPlatformInfoResp } from '../../api/generated/definitions/GbPlatformInfoResp'
import type { GbDependencyStatus } from '../../api/generated/definitions/GbDependencyStatus'
import { useAuthStore } from '../../store/authStore'
import { formatDT } from '../../types/business'

/** 请求失败提示：优先展示后端返回的 message */
function apiErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError<{ message?: string }>(err)) {
    return err.response?.data?.message || err.message || fallback
  }
  return err instanceof Error ? err.message : fallback
}

/** 秒数 → 天/时/分 */
function formatUptime(totalSeconds?: number): string {
  if (totalSeconds === undefined || totalSeconds === null || totalSeconds < 0) return '-'
  const seconds = Math.floor(totalSeconds)
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const rest = seconds % 60
  if (days > 0) return `${days} 天 ${hours} 小时 ${minutes} 分`
  if (hours > 0) return `${hours} 小时 ${minutes} 分`
  if (minutes > 0) return `${minutes} 分 ${rest} 秒`
  return `${rest} 秒`
}

interface DependencyCardProps {
  title: string
  status?: GbDependencyStatus
}

/** 依赖组件（数据库 / Redis）状态卡片 */
function DependencyCard({ title, status }: DependencyCardProps) {
  return (
    <Card
      size="small"
      title={title}
      extra={
        status?.online ? <Tag color="green">在线</Tag> : <Tag color="red">离线</Tag>
      }
    >
      <Descriptions column={1} size="small">
        <Descriptions.Item label="已配置">
          {status?.configured ? <Tag color="green">是</Tag> : <Tag>否</Tag>}
        </Descriptions.Item>
        <Descriptions.Item label="地址">{status?.address || '-'}</Descriptions.Item>
        <Descriptions.Item label="延迟">
          {status?.latency_ms != null ? `${status.latency_ms} ms` : '-'}
        </Descriptions.Item>
        <Descriptions.Item label="消息">{status?.message || '-'}</Descriptions.Item>
      </Descriptions>
    </Card>
  )
}

/**
 * 平台运行信息：版本、构建、运行时长、SIP 信令状态、默认媒体节点与依赖状态。
 *
 * 数据来自 `GET /web/gb/platform-info`，属于只读页面；多副本部署时各副本
 * 返回自身进程与所在主机的信息。
 */
const PlatformInfo: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canView = permCodes.includes('gb:platform-info:view')

  const [data, setData] = useState<GbPlatformInfoResp | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbOpsController.getGbPlatformInfo()
      setData(res.data ?? null)
      setError('')
    } catch (err) {
      const text = apiErrorMessage(err, '获取平台信息失败')
      setError(text)
      message.error(text)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (canView) void load()
  }, [canView, load])

  if (!canView) {
    return (
      <PageContainer title="平台信息" subTitle="国标服务运行状态与依赖信息">
        <Card size="small">
          <Empty description="当前账号无「平台信息」查看权限（gb:platform-info:view）" />
        </Card>
      </PageContainer>
    )
  }

  const sip = data?.sip
  const mediaServer = data?.default_media_server

  return (
    <PageContainer
      title="平台信息"
      subTitle="国标服务运行状态、版本与依赖信息（只读）"
      extra={
        <Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>
          刷新
        </Button>
      }
    >
      <div className="e2e-gb-platform-info">
        <Spin spinning={loading && !data}>
          {error && !data ? (
            <Alert
              type="error"
              showIcon
              message="平台信息加载失败"
              description={error}
              style={{ marginBottom: 12 }}
            />
          ) : null}

          <Row gutter={[12, 12]}>
            <Col xs={24} lg={12}>
              <Card size="small" title="版本与构建" extra={<InfoCircleOutlined style={{ color: '#8c8c8c' }} />}>
                <Descriptions column={2} size="small">
                  <Descriptions.Item label="版本">{data?.version || '-'}</Descriptions.Item>
                  <Descriptions.Item label="构建版本">{data?.build_version || '-'}</Descriptions.Item>
                  <Descriptions.Item label="构建时间">{formatDT(data?.build_time)}</Descriptions.Item>
                  <Descriptions.Item label="Go 版本">{data?.go_version || '-'}</Descriptions.Item>
                  <Descriptions.Item label="操作系统">{data?.os || '-'}</Descriptions.Item>
                  <Descriptions.Item label="架构">{data?.arch || '-'}</Descriptions.Item>
                  <Descriptions.Item label="主机名">{data?.hostname || '-'}</Descriptions.Item>
                  <Descriptions.Item label="启动时间">{formatDT(data?.started_dt)}</Descriptions.Item>
                  <Descriptions.Item label="运行时长" span={2}>
                    {formatUptime(data?.uptime_seconds)}
                    {data?.uptime_seconds != null ? (
                      <span style={{ color: '#8c8c8c' }}>（{data.uptime_seconds} 秒）</span>
                    ) : null}
                  </Descriptions.Item>
                </Descriptions>
              </Card>
            </Col>

            <Col xs={24} lg={12}>
              <Card
                size="small"
                title="SIP 信令"
                extra={
                  <Space size={4}>
                    {sip?.enabled ? <Tag color="blue">已启用</Tag> : <Tag>未启用</Tag>}
                    {sip?.running ? <Tag color="green">运行中</Tag> : <Tag color="red">未运行</Tag>}
                    {sip?.embedded ? <Tag color="geekblue">内嵌</Tag> : <Tag>独立进程</Tag>}
                  </Space>
                }
              >
                <Descriptions column={1} size="small">
                  <Descriptions.Item label="监听地址">{sip?.listen_addr || '-'}</Descriptions.Item>
                  <Descriptions.Item label="通告地址">{sip?.advertise_addr || '-'}</Descriptions.Item>
                  <Descriptions.Item label="SIP 域">{sip?.domain || '-'}</Descriptions.Item>
                  <Descriptions.Item label="传输协议">{sip?.transport || '-'}</Descriptions.Item>
                </Descriptions>
              </Card>
            </Col>

            <Col xs={24} lg={12}>
              <Card
                size="small"
                title="默认媒体节点"
                extra={
                  mediaServer
                    ? (mediaServer.online ? <Tag color="green">在线</Tag> : <Tag color="red">离线</Tag>)
                    : <Tag>未配置</Tag>
                }
              >
                {mediaServer ? (
                  <Descriptions column={1} size="small">
                    <Descriptions.Item label="节点名称">{mediaServer.name || '-'}</Descriptions.Item>
                    <Descriptions.Item label="server_id">{mediaServer.server_id || '-'}</Descriptions.Item>
                    <Descriptions.Item label="API 地址">
                      {mediaServer.ip
                        ? `http://${mediaServer.ip}${mediaServer.http_port ? `:${mediaServer.http_port}` : ''}`
                        : '-'}
                    </Descriptions.Item>
                    <Descriptions.Item label="播放地址">{mediaServer.stream_ip || '-'}</Descriptions.Item>
                    <Descriptions.Item label="最近心跳">
                      {formatDT(mediaServer.last_keepalive_dt)}
                    </Descriptions.Item>
                  </Descriptions>
                ) : (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有默认媒体节点" />
                )}
              </Card>
            </Col>

            <Col xs={24} lg={12}>
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                <DependencyCard title="数据库" status={data?.database} />
                <DependencyCard title="Redis" status={data?.redis} />
              </Space>
            </Col>
          </Row>
        </Spin>
      </div>
    </PageContainer>
  )
}

export default PlatformInfo
