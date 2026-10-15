import React, { useCallback, useEffect, useState } from 'react'
import {
  Table, Button, Modal, Form, Input, Select, Space, Row, Col, Tag, Alert, message, Popconfirm,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, EditOutlined, DeleteOutlined, MutedOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import AlarmTypeSelect from '../../components/AlarmTypeSelect'
import UserSelect from '../../components/UserSelect'
import { SettingController } from '../../api/generated/SettingController'
import type { AlarmSetting } from '../../api/generated/definitions/AlarmSetting'
import type { CreateAlarmSettingReq } from '../../api/generated/definitions/CreateAlarmSettingReq'
import { useAuthStore } from '../../store/authStore'

/** 通知类型下拉选项 */
const NOTIFY_TYPE_OPTIONS = [
  { value: 'app', label: 'APP 推送' },
  { value: 'sms', label: '短信' },
  { value: 'inner', label: '站内通知' },
]

/** 报警频率下拉选项（单位：分钟） */
const NOTIFY_FREQ_OPTIONS = [
  { value: 0, label: '实时' },
  { value: 60, label: '每小时' },
  { value: 1440, label: '每天' },
]

const NOTIFY_TYPE_LABEL: Record<string, string> = {
  app: 'APP 推送',
  sms: '短信',
  inner: '站内通知',
}

/** 告警设置表单值 */
interface AlarmSettingFormValues {
  alarm_type_id: number
  notify_user_ids?: number[]
  notify_type?: string
  notify_freq?: number
  threshold?: string
  active?: boolean
  remark?: string
}

/** 报警频率展示 */
function fmtFreq(v?: number): string {
  if (v === undefined || v === null) return '-'
  if (v === 0) return '实时'
  if (v === 60) return '每小时'
  if (v === 1440) return '每天'
  return `${v} 分钟`
}

/** 将后端的逗号分隔用户 ID 串解析为数组 */
function parseUserIDs(v?: string): number[] {
  if (!v) return []
  return v.split(',').map((s) => Number(s.trim())).filter((n) => !Number.isNaN(n) && n > 0)
}

/** 设备告警设置：仅能配置当前登录用户自己的通知策略 */
const AlarmSettingList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('setting:alarm:create')
  const canUpdate = permCodes.includes('setting:alarm:update')
  const canDelete = permCodes.includes('setting:alarm:delete')
  const canDisableAll = permCodes.includes('setting:alarm:disable-all')

  const [data, setData] = useState<AlarmSetting[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [alarmTypeId, setAlarmTypeId] = useState<number | undefined>(undefined)

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<AlarmSetting | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<AlarmSettingFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await SettingController.pageAlarmSettings(page, pageSize, keyword, alarmTypeId)
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      message.error('获取设备告警设置失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, alarmTypeId])

  useEffect(() => { fetchData() }, [fetchData])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setAlarmTypeId(undefined)
    setPage(1)
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ notify_type: 'app', notify_freq: 0, active: true })
    setModalOpen(true)
  }

  const openEdit = (row: AlarmSetting) => {
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      alarm_type_id: row.alarm_type_id,
      notify_user_ids: parseUserIDs(row.notify_user_ids),
      notify_type: row.notify_type ?? 'app',
      notify_freq: row.notify_freq ?? 0,
      threshold: row.threshold ?? '',
      active: row.active !== false,
      remark: row.remark ?? '',
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    const payload: CreateAlarmSettingReq = {
      alarm_type_id: values.alarm_type_id,
      notify_user_ids: (values.notify_user_ids ?? []).join(','),
      notify_type: values.notify_type,
      notify_freq: values.notify_freq,
      threshold: values.threshold,
      active: values.active,
      remark: values.remark,
    }
    setSaving(true)
    try {
      if (editing?.id) {
        await SettingController.updateAlarmSetting(String(editing.id), payload)
        message.success('修改成功')
      } else {
        await SettingController.createAlarmSetting(payload)
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

  const handleDelete = async (row: AlarmSetting) => {
    try {
      await SettingController.deleteAlarmSetting(String(row.id))
      message.success('删除成功')
      fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  /** 关闭当前登录用户的全部设备告警通知 */
  const handleDisableAll = async () => {
    try {
      await SettingController.disableAllAlarm()
      message.success('已关闭所有告警通知')
      fetchData()
    } catch {
      message.error('操作失败')
    }
  }

  const columns: ColumnsType<AlarmSetting> = [
    { title: '告警类型', dataIndex: ['alarm_type', 'name'], width: 180, render: (_, row) => row.alarm_type?.name ?? '-' },
    {
      title: '通知人员',
      dataIndex: 'notify_user_ids',
      width: 260,
      render: (v?: string) => {
        const ids = parseUserIDs(v)
        if (!ids.length) return '-'
        return <UserSelect multiple value={ids} disabled style={{ minWidth: 220 }} />
      },
    },
    {
      title: '通知类型',
      dataIndex: 'notify_type',
      width: 120,
      render: (v?: string) => (v ? (NOTIFY_TYPE_LABEL[v] ?? v) : '-'),
    },
    { title: '报警频率', dataIndex: 'notify_freq', width: 110, render: (v?: number) => fmtFreq(v) },
    { title: '阈值', dataIndex: 'threshold', width: 100, render: (v?: string) => v || '-' },
    {
      title: '状态',
      dataIndex: 'active',
      width: 90,
      render: (v?: boolean) => (v === false ? <Tag color="red">已关闭</Tag> : <Tag color="green">已启用</Tag>),
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
      title="设备告警设置"
      subTitle="配置设备离线、电量、流量等告警的通知方式与频率"
      extra={
        <Space>
          {canDisableAll && (
            <Popconfirm title="确定关闭所有告警通知?" onConfirm={handleDisableAll}>
              <Button danger icon={<MutedOutlined />}>
                关闭所有告警
              </Button>
            </Popconfirm>
          )}
          {canCreate && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新增告警设置
            </Button>
          )}
        </Space>
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="仅能配置当前登录用户自己的设置"
        description="告警通知策略与登录账号绑定，列表中只展示当前用户的通知配置，无法查看或修改其他用户的设置。"
      />

      <SearchBar
        fields={[{ name: 'keyword', label: '告警类型/通知人员', placeholder: '请输入告警类型或通知人员' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Space wrap style={{ marginBottom: 16 }}>
        <div style={{ width: 200 }}>
          <AlarmTypeSelect
            placeholder="告警类型"
            value={alarmTypeId}
            onChange={(v?: number) => { setAlarmTypeId(v); setPage(1) }}
          />
        </div>
      </Space>

      <Table<AlarmSetting>
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
        className="e2e-alarm-setting-modal"
        title={editing ? '编辑告警设置' : '新增告警设置'}
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
              <Form.Item name="alarm_type_id" label="告警类型" rules={[{ required: true, message: '请选择告警类型' }]}>
                <AlarmTypeSelect placeholder="请选择告警类型" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="notify_user_ids" label="通知人员">
                <UserSelect multiple placeholder="请选择通知人员" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="notify_type" label="通知类型">
                <Select options={NOTIFY_TYPE_OPTIONS} placeholder="请选择通知类型" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="notify_freq" label="报警频率">
                <Select options={NOTIFY_FREQ_OPTIONS} placeholder="请选择报警频率" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="threshold" label="阈值" tooltip="电量告警为百分比（%），流量告警为 MB">
                <Input placeholder="如：20" maxLength={50} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="active" label="启用状态">
                <Select
                  options={[
                    { value: true, label: '启用' },
                    { value: false, label: '停用' },
                  ]}
                  placeholder="请选择启用状态"
                />
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
    </PageContainer>
  )
}

export default AlarmSettingList
