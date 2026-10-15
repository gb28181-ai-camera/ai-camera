import React, { useCallback, useEffect, useState } from 'react'
import {
  Alert, Button, Col, DatePicker, Descriptions, Form, Input, message, Modal, Popconfirm, Row, Select, Space, Table,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { DeleteOutlined, EditOutlined, EyeOutlined, PlusOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import type { Dayjs } from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import RegionSelect from '../../components/RegionSelect'
import UserSelect from '../../components/UserSelect'
import StatusTag from '../../components/StatusTag'
import { InspectController } from '../../api/generated/InspectController'
import type { InspectTaskItem } from '../../api/generated/definitions/InspectTaskItem'
import type { PageInspectTaskReq } from '../../api/generated/definitions/PageInspectTaskReq'
import type { CreateInspectTaskReq } from '../../api/generated/definitions/CreateInspectTaskReq'
import type { UpdateInspectTaskReq } from '../../api/generated/definitions/UpdateInspectTaskReq'
import { useAuthStore } from '../../store/authStore'
import { INSPECT_STATUS_LABELS, INSPECT_STATUS_OPTIONS, formatDT } from '../../types/business'

const { RangePicker } = DatePicker

/** 巡检任务表单值 */
interface InspectTaskFormValues {
  name: string
  region_id?: number
  location?: string
  /** 与后端 request.CreateInspectTaskReq.inspector_id 对齐（必填） */
  inspector_id: number
  content?: string
  plan_dt?: Dayjs
  status?: string
  remark?: string
}

/** 弹窗模式：查看（只读）/ 新增 / 编辑 */
type ModalMode = 'view' | 'create' | 'edit'

/**
 * 巡检任务管理：创建巡检任务并指派巡检人。
 *
 * 任务的「完成」动作只在移动端发生（`PUT /app/inspect-tasks/:id/finish`），
 * 管理后台只负责创建、编辑、删除与查看。
 */
const InspectTaskList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('inspect:task:create')
  const canUpdate = permCodes.includes('inspect:task:update')
  const canDelete = permCodes.includes('inspect:task:delete')

  const [data, setData] = useState<InspectTaskItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [keyword, setKeyword] = useState('')
  const [regionID, setRegionID] = useState<number | undefined>()
  const [inspectorID, setInspectorID] = useState<number | undefined>()
  const [status, setStatus] = useState<string | undefined>()
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null)

  const [modalOpen, setModalOpen] = useState(false)
  const [mode, setMode] = useState<ModalMode>('create')
  const [current, setCurrent] = useState<InspectTaskItem | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<InspectTaskFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const query: PageInspectTaskReq = {
        page,
        page_size: pageSize,
        keyword: keyword || undefined,
        region_id: regionID,
        inspector_id: inspectorID,
        status: status || undefined,
        start_dt: range?.[0] ? range[0].format('YYYY-MM-DD HH:mm:ss') : undefined,
        end_dt: range?.[1] ? range[1].format('YYYY-MM-DD HH:mm:ss') : undefined,
      }
      const res = await InspectController.pageInspectTasks(query)
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch {
      message.error('获取巡检任务失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword, regionID, inspectorID, status, range])

  useEffect(() => { void fetchData() }, [fetchData])

  const handleSearch = (values: Record<string, string>) => {
    setKeyword(values.keyword ?? '')
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setRegionID(undefined)
    setInspectorID(undefined)
    setStatus(undefined)
    setRange(null)
    setPage(1)
  }

  const openCreate = () => {
    setMode('create')
    setCurrent(null)
    form.resetFields()
    form.setFieldsValue({ status: 'published', plan_dt: dayjs().add(1, 'day').startOf('hour') })
    setModalOpen(true)
  }

  /**
   * 查看任务详情。
   *
   * 操作手册 6.1：点击查看后「该消息通知由未读转为已读」，
   * 故打开弹窗的同时调用任务级已读接口（后端幂等），并刷新列表回显「已读」。
   */
  const openView = async (row: InspectTaskItem) => {
    setMode('view')
    setCurrent(row)
    setModalOpen(true)

    if (!row.id || row.read) return
    try {
      await InspectController.markInspectTaskRead(row.id)
      // 弹窗内「消息通知」与列表列同步为已读
      setCurrent((prev) => (prev && prev.id === row.id ? { ...prev, read: true } : prev))
      void fetchData()
    } catch {
      message.error('标记已读失败')
    }
  }

  const openEdit = (row: InspectTaskItem) => {
    setMode('edit')
    setCurrent(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      region_id: row.region_id ?? undefined,
      location: row.location ?? '',
      inspector_id: row.inspector_id ?? undefined,
      content: row.content ?? '',
      plan_dt: row.plan_dt ? dayjs(row.plan_dt) : undefined,
      status: row.status ?? 'published',
      remark: row.remark ?? '',
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    const payload: CreateInspectTaskReq = {
      name: values.name,
      region_id: values.region_id,
      location: values.location,
      inspector_id: values.inspector_id,
      content: values.content,
      plan_dt: values.plan_dt ? values.plan_dt.format('YYYY-MM-DD HH:mm:ss') : undefined,
      status: values.status || 'published',
      remark: values.remark,
    }
    setSaving(true)
    try {
      if (mode === 'edit' && current?.id) {
        const updatePayload: UpdateInspectTaskReq = payload
        await InspectController.updateInspectTask(current.id, updatePayload)
        message.success('修改成功')
      } else {
        await InspectController.createInspectTask(payload)
        message.success('新增成功，已生成巡检通知')
      }
      setModalOpen(false)
      void fetchData()
    } catch {
      message.error('保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (row: InspectTaskItem) => {
    if (!row.id) return
    try {
      await InspectController.deleteInspectTask(row.id)
      message.success('删除成功')
      void fetchData()
    } catch {
      message.error('删除失败')
    }
  }

  const columns: ColumnsType<InspectTaskItem> = [
    { title: '任务名称', dataIndex: 'name', width: 180, ellipsis: true },
    { title: '所属区域', dataIndex: ['region', 'name'], width: 140, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '巡检位置', dataIndex: 'location', width: 180, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '巡检人', dataIndex: 'inspector_name', width: 110, render: (v?: string) => v || '-' },
    { title: '巡检内容', dataIndex: 'content', width: 240, ellipsis: true, render: (v: unknown) => (v ? String(v) : '-') },
    { title: '巡检时间', dataIndex: 'plan_dt', width: 170, render: (v?: string) => formatDT(v) },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (v?: string) => <StatusTag type="inspect" status={v} />,
    },
    {
      title: '消息通知',
      dataIndex: 'read',
      width: 100,
      render: (v?: boolean) => <span>{v ? '已读' : '未读'}</span>,
    },
    { title: '备注', dataIndex: 'remark', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '创建时间', dataIndex: 'created_dt', width: 170, render: (v?: string) => formatDT(v) },
    {
      title: '操作',
      key: 'action',
      width: 200,
      fixed: 'right',
      render: (_: unknown, row) => (
        <ActionButtons maxWidth={200}>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => { void openView(row) }}>
            查看
          </Button>
          {canUpdate && (
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              disabled={row.status === 'finished'}
              onClick={() => openEdit(row)}
            >
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
      title="巡检任务管理"
      subTitle="创建巡检任务并指派巡检人，任务完成由移动端提交"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增巡检任务
          </Button>
        ) : null
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="任务发布后由巡检人登录移动端处理，处理后任务移除并生成巡检记录"
      />

      <SearchBar
        fields={[
          { name: 'keyword', label: '任务名称', placeholder: '请输入任务名称' },
          { name: 'status', label: '状态', type: 'select', options: INSPECT_STATUS_OPTIONS },
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
        <UserSelect
          style={{ width: 180 }}
          placeholder="请选择巡检人"
          value={inspectorID}
          onChange={(v) => { setInspectorID(v === undefined || v === null ? undefined : Number(v)); setPage(1) }}
        />
        <RangePicker
          showTime
          value={range}
          placeholder={['巡检时间起', '巡检时间止']}
          onChange={(dates) => { setRange(dates); setPage(1) }}
        />
      </Space>

      <Table<InspectTaskItem>
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

      {/* 巡检任务：查看（只读）/ 新增 / 编辑 共用同一弹窗 */}
      <Modal
        className="e2e-inspect-task-modal"
        title={mode === 'view' ? '巡检任务详情' : mode === 'edit' ? '编辑巡检任务' : '新增巡检任务'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={mode === 'view' ? undefined : handleSave}
        confirmLoading={saving}
        destroyOnHidden
        footer={
          mode === 'view'
            ? [
                <Button key="close" onClick={() => setModalOpen(false)}>
                  关闭
                </Button>,
                canUpdate ? (
                  <Button
                    key="edit"
                    type="primary"
                    disabled={current?.status === 'finished'}
                    onClick={() => current && openEdit(current)}
                  >
                    编辑
                  </Button>
                ) : null,
              ]
            : undefined
        }
        width={720}
      >
        {mode === 'view' && current ? (
          <Descriptions column={2} size="small" bordered>
            <Descriptions.Item label="任务名称" span={2}>
              {current.name || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="所属区域">{current.region?.name ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="巡检位置">{current.location || '-'}</Descriptions.Item>
            <Descriptions.Item label="巡检人">{current.inspector_name || '-'}</Descriptions.Item>
            <Descriptions.Item label="巡检时间">{formatDT(current.plan_dt)}</Descriptions.Item>
            <Descriptions.Item label="状态">
              {current.status ? INSPECT_STATUS_LABELS[current.status] ?? current.status : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="消息通知">{current.read ? '已读' : '未读'}</Descriptions.Item>
            <Descriptions.Item label="巡检内容" span={2}>
              {current.content || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="备注" span={2}>
              {current.remark || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="创建人">{current.created_by || '-'}</Descriptions.Item>
            <Descriptions.Item label="创建时间">{formatDT(current.created_dt)}</Descriptions.Item>
          </Descriptions>
        ) : (
          <Form form={form} layout="vertical">
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item name="name" label="任务名称" rules={[{ required: true, message: '请输入任务名称' }]}>
                  <Input placeholder="请输入任务名称" maxLength={255} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="region_id" label="所属区域">
                  <RegionSelect style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="location" label="巡检位置">
                  <Input placeholder="请输入巡检位置" maxLength={255} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="inspector_id" label="巡检人" rules={[{ required: true, message: '请选择巡检人' }]}>
                  <UserSelect style={{ width: '100%' }} placeholder="请选择巡检人" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="plan_dt" label="巡检时间" rules={[{ required: true, message: '请选择巡检时间' }]}>
                  <DatePicker showTime style={{ width: '100%' }} placeholder="请选择巡检时间" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="status" label="状态">
                  <Select options={INSPECT_STATUS_OPTIONS} placeholder="请选择状态" />
                </Form.Item>
              </Col>
              <Col span={24}>
                <Form.Item name="content" label="巡检内容">
                  <Input.TextArea rows={3} maxLength={500} showCount placeholder="请输入巡检内容" />
                </Form.Item>
              </Col>
              <Col span={24}>
                <Form.Item name="remark" label="备注">
                  <Input.TextArea rows={2} maxLength={500} showCount placeholder="请输入备注" />
                </Form.Item>
              </Col>
            </Row>
          </Form>
        )}
      </Modal>
    </PageContainer>
  )
}

export default InspectTaskList
