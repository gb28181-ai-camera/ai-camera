import { useCallback, useEffect, useState } from 'react'
import {
  Alert, Button, Col, Descriptions, Form, Input, InputNumber, Modal, Popconfirm, Row, Select,
  Spin, Switch, Table, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  CheckCircleOutlined, DeleteOutlined, EditOutlined, InfoCircleOutlined, PlusOutlined,
} from '@ant-design/icons'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import ActionButtons from '../../components/ActionButtons'
import { GbMediaServerController } from '../../api/generated/GbMediaServerController'
import type { GbMediaServerItem } from '../../api/generated/definitions/GbMediaServerItem'
import type { GbMediaServerCheckResp } from '../../api/generated/definitions/GbMediaServerCheckResp'
import type { GbMediaServerInfoResp } from '../../api/generated/definitions/GbMediaServerInfoResp'
import type { GbMediaServerThreadLoad } from '../../api/generated/definitions/GbMediaServerThreadLoad'
import type { CreateGbMediaServerReq } from '../../api/generated/definitions/CreateGbMediaServerReq'
import type { UpdateGbMediaServerReq } from '../../api/generated/definitions/UpdateGbMediaServerReq'
import { useAuthStore } from '../../store/authStore'

/** 媒体节点表单值 */
interface MediaServerFormValues {
  name: string
  server_id: string
  type?: string
  ip?: string
  http_port?: number | null
  rtp_proxy_port?: number | null
  rtmp_port?: number | null
  rtsp_port?: number | null
  flv_port?: number | null
  record_day?: number | null
  hook_alive_interval?: number | null
  secret?: string
  default_server?: boolean
  active?: boolean
  remark?: string
}

/** 节点类型选项（当前仅对接 ZLMediaKit） */
const TYPE_OPTIONS = [{ label: 'ZLMediaKit', value: 'zlm' }]

/** 时间格式化 */
function fmtDT(v?: string): string {
  if (!v) return '-'
  const d = dayjs(v)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : v
}

/** InputNumber 空值统一转 undefined，避免把 null 写进请求体 */
function num(v?: number | null): number | undefined {
  return typeof v === 'number' && !Number.isNaN(v) ? v : undefined
}

/** 节点 API 地址：http://<ip>:<http_port> */
function apiAddress(row: GbMediaServerItem): string {
  if (!row.ip) return '-'
  return `http://${row.ip}${row.http_port ? `:${row.http_port}` : ''}`
}

/**
 * 媒体节点（ZLMediaKit）。
 *
 * 本项目按**单端口收流**模式部署（rtp_enable=false）：设备统一向节点的
 * RTP 收流端口打流，页面不提供多端口 RTP 范围配置。
 */
export default function MediaServerList() {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('gb:media-server:create')
  const canUpdate = permCodes.includes('gb:media-server:update')
  const canDelete = permCodes.includes('gb:media-server:delete')
  const canCheck = permCodes.includes('gb:media-server:check')

  const [data, setData] = useState<GbMediaServerItem[]>([])
  const [loading, setLoading] = useState(false)

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<GbMediaServerItem | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<MediaServerFormValues>()

  const [checkingId, setCheckingId] = useState<number | null>(null)
  const [checkOpen, setCheckOpen] = useState(false)
  const [checkResult, setCheckResult] = useState<GbMediaServerCheckResp | null>(null)

  const [infoOpen, setInfoOpen] = useState(false)
  const [infoLoading, setInfoLoading] = useState(false)
  const [infoResult, setInfoResult] = useState<GbMediaServerInfoResp | null>(null)
  const [infoNodeName, setInfoNodeName] = useState('')

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbMediaServerController.listGbMediaServers()
      if (res.code === 200) {
        setData(res.data ?? [])
      } else {
        message.error(res.message || '获取媒体节点列表失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({
      type: 'zlm',
      record_day: 7,
      hook_alive_interval: 15,
      default_server: false,
      active: true,
    })
    setModalOpen(true)
  }

  const openEdit = (row: GbMediaServerItem) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      server_id: row.server_id ?? '',
      type: row.type || 'zlm',
      ip: row.ip ?? '',
      http_port: row.http_port ?? undefined,
      rtp_proxy_port: row.rtp_proxy_port ?? undefined,
      rtmp_port: row.rtmp_port ?? undefined,
      rtsp_port: row.rtsp_port ?? undefined,
      flv_port: row.flv_port ?? undefined,
      record_day: row.record_day ?? 7,
      hook_alive_interval: row.hook_alive_interval ?? 15,
      // secret 后端已脱敏为 ****；留空表示不修改
      secret: '',
      default_server: row.default_server ?? false,
      active: row.active ?? true,
      remark: row.remark ?? '',
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    let values: MediaServerFormValues
    try {
      values = await form.validateFields()
    } catch {
      return
    }

    const common: UpdateGbMediaServerReq = {
      type: values.type,
      ip: values.ip?.trim(),
      http_port: num(values.http_port),
      rtp_proxy_port: num(values.rtp_proxy_port),
      rtmp_port: num(values.rtmp_port),
      rtsp_port: num(values.rtsp_port),
      flv_port: num(values.flv_port),
      record_day: num(values.record_day),
      hook_alive_interval: num(values.hook_alive_interval),
      default_server: values.default_server,
      active: values.active,
      remark: values.remark,
      // 单端口收流模式：不开放多端口 RTP 范围
      rtp_enable: false,
    }

    setSaving(true)
    try {
      if (editing?.id) {
        const payload: UpdateGbMediaServerReq = { ...common }
        if (values.secret) payload.secret = values.secret
        const res = await GbMediaServerController.updateGbMediaServer(String(editing.id), payload)
        if (res.code === 200) {
          message.success('修改成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '保存失败')
        }
      } else {
        const payload: CreateGbMediaServerReq = {
          ...common,
          name: values.name,
          server_id: values.server_id,
          secret: values.secret || undefined,
        }
        const res = await GbMediaServerController.createGbMediaServer(payload)
        if (res.code === 200) {
          message.success('新增成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '保存失败')
        }
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (row: GbMediaServerItem) => {
    if (!row.id) return
    try {
      const res = await GbMediaServerController.deleteGbMediaServer(String(row.id))
      if (res.code === 200) {
        message.success('删除成功')
        fetchData()
      } else {
        message.error(res.message || '删除失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    }
  }

  const handleCheck = async (row: GbMediaServerItem) => {
    if (!row.id) return
    setCheckingId(row.id)
    try {
      const res = await GbMediaServerController.checkGbMediaServer(String(row.id))
      if (res.code === 200) {
        setCheckResult(res.data ?? null)
        setCheckOpen(true)
      } else {
        message.error(res.message || '在线检查失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setCheckingId(null)
    }
  }

  const handleInfo = async (row: GbMediaServerItem) => {
    if (!row.id) return
    setInfoNodeName(row.name ?? '')
    setInfoResult(null)
    setInfoOpen(true)
    setInfoLoading(true)
    try {
      const res = await GbMediaServerController.getGbMediaServerInfo(String(row.id))
      if (res.code === 200) {
        setInfoResult(res.data ?? null)
      } else {
        message.error(res.message || '获取节点信息失败')
      }
    } catch {
      // request 拦截器已按后端 message 原样提示
    } finally {
      setInfoLoading(false)
    }
  }

  const columns: ColumnsType<GbMediaServerItem> = [
    { title: '节点名称', dataIndex: 'name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    { title: 'server_id', dataIndex: 'server_id', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '类型', dataIndex: 'type', width: 120,
      render: (v?: string) => (v ? <Tag color="geekblue">{v}</Tag> : '-'),
    },
    {
      title: 'API 地址', dataIndex: 'ip', width: 210,
      render: (_, row) => <span style={{ fontFamily: 'Consolas, Monaco, monospace' }}>{apiAddress(row)}</span>,
    },
    {
      title: '在线状态', dataIndex: 'online', width: 110,
      render: (v?: boolean) => (v ? <Tag color="green">在线</Tag> : <Tag color="red">离线</Tag>),
    },
    { title: '最近心跳', dataIndex: 'last_keepalive_dt', width: 170, render: (v?: string) => fmtDT(v) },
    {
      title: '默认节点', dataIndex: 'default_server', width: 100,
      render: (v?: boolean) => (v ? <Tag color="blue">默认</Tag> : '-'),
    },
    {
      title: '录像保留', dataIndex: 'record_day', width: 100,
      render: (v?: number) => (v != null ? `${v} 天` : '-'),
    },
    {
      title: '操作', key: 'action', width: 300, fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={300}>
          <Button
            type="link"
            size="small"
            icon={<InfoCircleOutlined />}
            onClick={() => handleInfo(row)}
          >
            节点信息
          </Button>
          {canCheck && (
            <Button
              type="link"
              size="small"
              icon={<CheckCircleOutlined />}
              loading={checkingId === row.id}
              onClick={() => handleCheck(row)}
            >
              在线检查
            </Button>
          )}
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
              编辑
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除该媒体节点?" onConfirm={() => handleDelete(row)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  const loadColumns: ColumnsType<GbMediaServerThreadLoad> = [
    { title: '线程', dataIndex: 'thread_name', render: (v?: string) => v || '-' },
    { title: '负载', dataIndex: 'load', width: 140, render: (v?: number) => (v != null ? v.toFixed(2) : '-') },
  ]

  return (
    <PageContainer
      title="媒体节点"
      subTitle="管理 ZLMediaKit 流媒体节点，供国标设备接入、拉流与录像使用"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增节点
          </Button>
        ) : null
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message="单端口收流模式"
        description="本项目按单端口收流部署（rtp_enable=false）：设备统一向节点的 RTP 收流端口打流，无需开启多端口 RTP 收流范围。"
      />

      <Table<GbMediaServerItem>
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 'max-content' }}
        pagination={false}
        locale={{ emptyText: '暂无媒体节点' }}
      />

      <Modal
        className="e2e-gb-media-server-modal"
        title={editing ? '编辑媒体节点' : '新增媒体节点'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        confirmLoading={saving}
        destroyOnHidden
        width={760}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="节点名称" rules={[{ required: true, message: '请输入节点名称' }]}>
                <Input placeholder="如：本地 ZLMediaKit" maxLength={255} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="server_id"
                label="server_id"
                rules={[{ required: true, message: '请输入节点唯一标识' }]}
                extra="需与 ZLM 配置的 mediaServerId 一致"
              >
                <Input placeholder="如：ai-camera-zlm-01" maxLength={50} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="type" label="类型">
                <Select options={TYPE_OPTIONS} placeholder="请选择节点类型" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="ip"
                label="API 地址（IP）"
                rules={[{ required: true, message: '请输入节点 API IP' }]}
                extra="后端调用 ZLM HTTP API 的地址"
              >
                <Input placeholder="如：127.0.0.1" maxLength={255} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="http_port" label="HTTP 端口">
                <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="如：8082" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="rtp_proxy_port" label="RTP 收流端口">
                <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="单端口收流端口" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="rtmp_port" label="RTMP 端口">
                <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="如：1935" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="rtsp_port" label="RTSP 端口">
                <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="如：554" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="flv_port" label="HTTP-FLV 端口">
                <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="如：8082" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="record_day" label="录像保留天数">
                <InputNumber min={1} max={365} style={{ width: '100%' }} placeholder="默认 7 天" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="hook_alive_interval" label="心跳间隔（秒）">
                <InputNumber min={5} max={300} style={{ width: '100%' }} placeholder="默认 15 秒" />
              </Form.Item>
            </Col>
            <Col span={16}>
              <Form.Item
                name="secret"
                label="API Secret"
                extra={editing ? '留空或 **** 表示不修改（后端已脱敏）' : 'ZLM 的 api.secret'}
              >
                <Input.Password
                  placeholder={editing ? '留空表示不修改' : '请输入 ZLM API secret'}
                  autoComplete="new-password"
                  maxLength={255}
                />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="default_server" label="默认节点" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="active" label="启用" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="remark" label="备注">
                <Input.TextArea rows={2} maxLength={500} showCount placeholder="请输入备注" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>

      <Modal
        className="e2e-gb-media-server-check-modal"
        title="节点连通性检查"
        open={checkOpen}
        onCancel={() => setCheckOpen(false)}
        footer={<Button onClick={() => setCheckOpen(false)}>关闭</Button>}
        width={600}
      >
        <Descriptions column={1} size="small" bordered>
          <Descriptions.Item label="检查结果">
            {checkResult?.online ? <Tag color="green">在线</Tag> : <Tag color="red">离线</Tag>}
          </Descriptions.Item>
          <Descriptions.Item label="节点标识">{checkResult?.media_server_id || '-'}</Descriptions.Item>
          <Descriptions.Item label="版本">{checkResult?.version || '-'}</Descriptions.Item>
          <Descriptions.Item label="HTTP 端口">{checkResult?.http_port ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="RTP / RTCP 端口">
            {checkResult?.rtp_port != null || checkResult?.rtcp_port != null
              ? `${checkResult?.rtp_port ?? '-'} / ${checkResult?.rtcp_port ?? '-'}`
              : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="secret 校验">
            {checkResult?.secret_matched ? <Tag color="green">匹配</Tag> : <Tag color="orange">未匹配</Tag>}
          </Descriptions.Item>
          <Descriptions.Item label="耗时">
            {checkResult?.response_elapsed_ms != null ? `${checkResult.response_elapsed_ms} ms` : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="消息">{checkResult?.message || '-'}</Descriptions.Item>
        </Descriptions>
      </Modal>

      <Modal
        className="e2e-gb-media-server-info-modal"
        title={infoNodeName ? `节点信息 - ${infoNodeName}` : '节点信息'}
        open={infoOpen}
        onCancel={() => setInfoOpen(false)}
        footer={<Button onClick={() => setInfoOpen(false)}>关闭</Button>}
        width={680}
      >
        <Spin spinning={infoLoading}>
          <Descriptions column={2} size="small" bordered style={{ marginBottom: 16 }}>
            <Descriptions.Item label="在线状态">
              {infoResult?.online ? <Tag color="green">在线</Tag> : <Tag color="red">离线</Tag>}
            </Descriptions.Item>
            <Descriptions.Item label="版本">{infoResult?.version || '-'}</Descriptions.Item>
            <Descriptions.Item label="活跃流数">{infoResult?.active_streams ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="线程数">{infoResult?.thread_count ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="平均负载">
              {infoResult?.avg_load != null ? infoResult.avg_load.toFixed(2) : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="最大负载">
              {infoResult?.max_load != null ? infoResult.max_load.toFixed(2) : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="消息" span={2}>{infoResult?.message || '-'}</Descriptions.Item>
          </Descriptions>
          <Table<GbMediaServerThreadLoad>
            rowKey={(row) => row.thread_name ?? String(row.load ?? '')}
            size="small"
            columns={loadColumns}
            dataSource={infoResult?.loads ?? []}
            pagination={false}
            scroll={{ y: 240 }}
            locale={{ emptyText: '暂无线程负载数据' }}
          />
        </Spin>
      </Modal>
    </PageContainer>
  )
}
