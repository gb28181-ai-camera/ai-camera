import React, { useCallback, useEffect, useState } from 'react'
import {
  Table, Button, Modal, Form, Input, Select, Space, Row, Col, message, Popconfirm,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, EditOutlined, DeleteOutlined, PlayCircleOutlined, DownloadOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import RegionSelect from '../../components/RegionSelect'
import RiskTypeSelect from '../../components/RiskTypeSelect'
import AudioUpload from '../../components/AudioUpload'
import { DeviceController } from '../../api/generated/DeviceController'
import { BasicController } from '../../api/generated/BasicController'
import type { VoiceConfig } from '../../api/generated/definitions/VoiceConfig'
import type { CreateVoiceConfigReq } from '../../api/generated/definitions/CreateVoiceConfigReq'
import type { OptionItem } from '../../api/generated/definitions/OptionItem'
import { useAuthStore } from '../../store/authStore'

/** 语音配置表单值 */
interface VoiceConfigFormValues {
  name: string
  voice_type?: string
  risk_type_id?: number
  region_id?: number
  device_id?: number
  audio_url?: string
  remark?: string
}

/** 从音频地址中取出文件名 */
function fileNameOf(url?: string): string {
  if (!url) return ''
  const idx = url.lastIndexOf('/')
  return idx >= 0 ? url.slice(idx + 1) : url
}

/** 语音配置管理：为风险类型配置设备播报音频 */
const VoiceConfigList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('voice-config:create')
  const canUpdate = permCodes.includes('voice-config:update')
  const canDelete = permCodes.includes('voice-config:delete')

  const [data, setData] = useState<VoiceConfig[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [voiceType, setVoiceType] = useState<string | undefined>(undefined)
  const [deviceId, setDeviceId] = useState<number | undefined>(undefined)
  const [riskOptions, setRiskOptions] = useState<OptionItem[]>([])
  const [deviceOptions, setDeviceOptions] = useState<OptionItem[]>([])

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<VoiceConfig | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<VoiceConfigFormValues>()

  const [previewOpen, setPreviewOpen] = useState(false)
  const [previewUrl, setPreviewUrl] = useState('')

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await DeviceController.pageVoiceConfigs(page, pageSize, keyword, voiceType, undefined, deviceId)
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取语音配置列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, voiceType, deviceId])

  useEffect(() => { fetchData() }, [fetchData])

  useEffect(() => {
    BasicController.riskTypeOptions()
      .then((res) => setRiskOptions(res.data ?? []))
      .catch(() => setRiskOptions([]))
    DeviceController.deviceOptions()
      .then((res) => setDeviceOptions(res.data ?? []))
      .catch(() => setDeviceOptions([]))
  }, [])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setVoiceType(undefined)
    setDeviceId(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    setModalOpen(true)
  }

  const openEdit = (row: VoiceConfig) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      voice_type: row.voice_type ?? '',
      risk_type_id: row.risk_type_id,
      region_id: row.region_id,
      device_id: row.device_id,
      audio_url: row.audio_url ?? '',
      remark: row.remark ?? '',
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    const payload: CreateVoiceConfigReq = {
      name: values.name,
      voice_type: values.voice_type,
      risk_type_id: values.risk_type_id,
      region_id: values.region_id,
      device_id: values.device_id,
      audio_url: values.audio_url,
      audio_name: fileNameOf(values.audio_url),
      remark: values.remark,
    }
    setSaving(true)
    try {
      if (editing?.id) {
        await DeviceController.updateVoiceConfig(String(editing.id), payload)
        message.success('修改成功')
      } else {
        await DeviceController.createVoiceConfig(payload)
        message.success('新增成功')
      }
      setModalOpen(false)
      fetchData()
    } catch {
      message.error('保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (row: VoiceConfig) => {
    try {
      await DeviceController.deleteVoiceConfig(String(row.id))
      message.success('删除成功')
      fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  const openPreview = (url?: string) => {
    if (!url) {
      message.warning('该语音配置未上传音频文件')
      return
    }
    setPreviewUrl(url)
    setPreviewOpen(true)
  }

  const columns: ColumnsType<VoiceConfig> = [
    { title: '语音名称', dataIndex: 'name', width: 180, ellipsis: true },
    { title: '语音类型', dataIndex: 'voice_type', width: 140, render: (v?: string) => v || '-' },
    { title: '所属区域', dataIndex: ['region', 'name'], width: 140, render: (_, row) => row.region?.name ?? '-' },
    { title: '应用设备', dataIndex: ['device', 'name'], width: 160, render: (_, row) => row.device?.name ?? '-' },
    {
      title: '播报音频',
      key: 'audio',
      width: 360,
      render: (_, row) => {
        if (!row.audio_url) return '-'
        return (
          <Space direction="vertical" size={4}>
            {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
            <audio controls src={row.audio_url} style={{ height: 32, width: 240 }} />
            <Space size={12}>
              <a href={row.audio_url} target="_blank" rel="noreferrer" download>
                <DownloadOutlined /> 下载文件
              </a>
              <a onClick={() => openPreview(row.audio_url)}>
                <PlayCircleOutlined /> 试听
              </a>
            </Space>
          </Space>
        )
      },
    },
    {
      title: '操作',
      key: 'action',
      width: 160,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons>
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
              编辑
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除?" onConfirm={() => handleDelete(row)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  return (
    <PageContainer
      title="语音配置管理"
      subTitle="为风险类型配置设备现场播报的音频文件"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增语音配置
          </Button>
        ) : null
      }
    >
      <SearchBar
        fields={[{ name: 'keyword', label: '语音名称', placeholder: '请输入语音名称' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Space wrap style={{ marginBottom: 16 }}>
        <Select
          allowClear
          showSearch
          optionFilterProp="label"
          placeholder="语音类型"
          style={{ width: 180 }}
          value={voiceType}
          onChange={(v?: string) => { setVoiceType(v); setPage(1) }}
          options={riskOptions.map((r) => ({ value: r.code ?? '', label: r.name ?? '' }))}
        />
        <Select
          allowClear
          showSearch
          optionFilterProp="label"
          placeholder="所属设备"
          style={{ width: 200 }}
          value={deviceId}
          onChange={(v?: number) => { setDeviceId(v); setPage(1) }}
          options={deviceOptions.map((d) => ({ value: d.id ?? 0, label: d.name ?? '' }))}
        />
      </Space>

      <Table<VoiceConfig>
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

      <Modal
        className="e2e-voice-config-modal"
        title={editing ? '编辑语音配置' : '新增语音配置'}
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
              <Form.Item name="name" label="语音名称" rules={[{ required: true, message: '请输入语音名称' }]}>
                <Input placeholder="如：闯入告警播报" maxLength={255} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="voice_type" label="语音类型">
                <Input placeholder="关联风险类型编码，如 intrusion" maxLength={50} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="risk_type_id" label="风险类型">
                <RiskTypeSelect placeholder="请选择风险类型" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="region_id" label="所属区域">
                <RegionSelect placeholder="请选择所属区域" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="device_id" label="应用设备">
                <Select
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  placeholder="请选择应用设备"
                  options={deviceOptions.map((d) => ({ value: d.id ?? 0, label: d.name ?? '' }))}
                />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="audio_url" label="播报音频">
                <AudioUpload />
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

      {/* 音频试听 */}
      <Modal
        className="e2e-voice-preview-modal"
        title="音频试听"
        open={previewOpen}
        onCancel={() => setPreviewOpen(false)}
        footer={null}
        destroyOnHidden
        width={420}
      >
        {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
        <audio controls autoPlay src={previewUrl} style={{ width: '100%' }} />
      </Modal>
    </PageContainer>
  )
}

export default VoiceConfigList
