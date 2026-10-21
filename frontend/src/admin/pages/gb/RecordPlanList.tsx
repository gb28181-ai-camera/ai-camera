import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Button,
  Col,
  Form,
  Input,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Table,
  Tag,
  TimePicker,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  ApartmentOutlined,
  DeleteOutlined,
  EditOutlined,
  MinusCircleOutlined,
  PlusOutlined,
} from '@ant-design/icons'
import dayjs, { type Dayjs } from 'dayjs'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { GbChannelController } from '../../api/generated/GbChannelController'
import { GbRecordPlanController } from '../../api/generated/GbRecordPlanController'
import type { GbRecordPlanItem } from '../../api/generated/definitions/GbRecordPlanItem'
import type { GbRecordPlanItemReq } from '../../api/generated/definitions/GbRecordPlanItemReq'
import type { OptionItem } from '../../api/generated/definitions/OptionItem'
import { useAuthStore } from '../../store/authStore'

/** 0 周日 ~ 6 周六（后端字段 week_day） */
const WEEK_DAY_OPTIONS = [
  { label: '周日', value: 0 },
  { label: '周一', value: 1 },
  { label: '周二', value: 2 },
  { label: '周三', value: 3 },
  { label: '周四', value: 4 },
  { label: '周五', value: 5 },
  { label: '周六', value: 6 },
]

const WEEK_DAY_LABEL: Record<number, string> = {
  0: '周日',
  1: '周一',
  2: '周二',
  3: '周三',
  4: '周四',
  5: '周五',
  6: '周六',
}

interface PlanItemFormValue {
  week_day?: number
  start_time?: Dayjs
  end_time?: Dayjs
}

interface PlanFormValues {
  name: string
  active: boolean
  snap: boolean
  remark?: string
  items?: PlanItemFormValue[]
}

/** HH:mm:ss 字符串 → Dayjs（不依赖 customParseFormat 插件） */
function timeToDayjs(value?: string): Dayjs | undefined {
  if (!value) return undefined
  const parsed = dayjs(`2000-01-01T${value}`)
  return parsed.isValid() ? parsed : undefined
}

function fmtDT(value?: string): string {
  if (!value) return '-'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : value
}

/**
 * 录像计划：维护一周内的录像时间段，并把计划绑定到国标通道。
 *
 * ⚠️ 本期只做配置：后端没有定时调度器，启用开关与时间段仅落库，
 * 不会自动触发设备录像。页面顶部照实提示，避免用户误以为已生效。
 */
const RecordPlanList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canCreate = permCodes.includes('gb:record-plan:create')
  const canUpdate = permCodes.includes('gb:record-plan:update')
  const canDelete = permCodes.includes('gb:record-plan:delete')
  const canBind = permCodes.includes('gb:record-plan:bind')

  const [data, setData] = useState<GbRecordPlanItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<GbRecordPlanItem | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<PlanFormValues>()

  const [bindOpen, setBindOpen] = useState(false)
  const [bindPlan, setBindPlan] = useState<GbRecordPlanItem | null>(null)
  const [bindChannelIds, setBindChannelIds] = useState<number[]>([])
  const [bindLoading, setBindLoading] = useState(false)
  const [bindSaving, setBindSaving] = useState(false)
  const [channelOptions, setChannelOptions] = useState<OptionItem[]>([])

  useEffect(() => {
    void GbChannelController.gbChannelOptions()
      .then((res) => setChannelOptions(res.data ?? []))
      .catch(() => {
        /* 拦截器已提示 */
      })
  }, [])

  const channelSelectOptions = useMemo(
    () =>
      channelOptions.flatMap((item) => {
        if (typeof item.id !== 'number') return []
        const code = item.code ? `（${item.code}）` : ''
        return [{ label: `${item.name ?? item.id}${code}`, value: item.id }]
      }),
    [channelOptions],
  )

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await GbRecordPlanController.pageGbRecordPlans(page, pageSize, keyword)
      setData(res.data?.items ?? [])
      setTotal(res.data?.total ?? 0)
    } catch {
      /* 拦截器已提示 */
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword])

  useEffect(() => {
    void fetchData()
  }, [fetchData])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({
      name: '',
      active: true,
      snap: false,
      remark: '',
      items: [
        {
          week_day: 1,
          start_time: timeToDayjs('08:00:00'),
          end_time: timeToDayjs('18:00:00'),
        },
      ],
    })
    setModalOpen(true)
  }

  const openEdit = async (row: GbRecordPlanItem) => {
    if (!row.id) return
    setEditing(row)
    form.resetFields()
    form.setFieldsValue({
      name: row.name ?? '',
      active: row.active ?? true,
      snap: row.snap ?? false,
      remark: row.remark ?? '',
      items: [],
    })
    setModalOpen(true)
    setDetailLoading(true)
    try {
      const res = await GbRecordPlanController.getGbRecordPlan(row.id)
      const detail = res.data
      if (detail) {
        form.setFieldsValue({
          name: detail.name ?? '',
          active: detail.active ?? true,
          snap: detail.snap ?? false,
          remark: detail.remark ?? '',
          items: (detail.items ?? []).map((item) => ({
            week_day: item.week_day ?? 0,
            start_time: timeToDayjs(item.start_time),
            end_time: timeToDayjs(item.end_time),
          })),
        })
      }
    } catch {
      // 详情拿不到时不要留一个空时间段表单，否则保存会清空原计划的时间段
      setModalOpen(false)
    } finally {
      setDetailLoading(false)
    }
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    const items: GbRecordPlanItemReq[] = (values.items ?? []).map((item) => ({
      week_day: item.week_day ?? 0,
      start_time: item.start_time?.format('HH:mm:ss') ?? '',
      end_time: item.end_time?.format('HH:mm:ss') ?? '',
    }))

    const invalid = items.find((item) => !item.start_time || !item.end_time || item.end_time <= item.start_time)
    if (invalid) {
      message.warning(`${WEEK_DAY_LABEL[invalid.week_day ?? 0] ?? '所选'}的时间段无效：结束时间必须晚于开始时间`)
      return
    }

    setSaving(true)
    try {
      if (editing?.id) {
        await GbRecordPlanController.updateGbRecordPlan(editing.id, {
          name: values.name,
          active: values.active,
          snap: values.snap,
          remark: values.remark,
          items,
        })
        message.success('修改成功')
      } else {
        await GbRecordPlanController.createGbRecordPlan({
          name: values.name,
          active: values.active,
          snap: values.snap,
          remark: values.remark,
          items,
        })
        message.success('新增成功')
      }
      setModalOpen(false)
      void fetchData()
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (row: GbRecordPlanItem) => {
    if (!row.id) return
    try {
      await GbRecordPlanController.deleteGbRecordPlan(row.id)
      message.success('删除成功')
      void fetchData()
    } catch {
      /* 拦截器已提示 */
    }
  }

  const openBind = async (row: GbRecordPlanItem) => {
    if (!row.id) return
    setBindPlan(row)
    setBindChannelIds([])
    setBindOpen(true)
    setBindLoading(true)
    try {
      const res = await GbRecordPlanController.getGbRecordPlan(row.id)
      setBindChannelIds(res.data?.channel_ids ?? [])
    } catch {
      /* 拦截器已提示 */
    } finally {
      setBindLoading(false)
    }
  }

  const handleBind = async () => {
    if (!bindPlan?.id) return
    setBindSaving(true)
    try {
      await GbRecordPlanController.linkGbRecordPlanChannels(bindPlan.id, {
        channel_ids: bindChannelIds,
      })
      message.success('通道绑定已更新')
      setBindOpen(false)
      void fetchData()
    } catch {
      /* 拦截器已提示 */
    } finally {
      setBindSaving(false)
    }
  }

  const columns: ColumnsType<GbRecordPlanItem> = [
    { title: '计划名称', dataIndex: 'name', width: 200, ellipsis: true },
    {
      title: '状态',
      dataIndex: 'active',
      width: 90,
      render: (value?: boolean) =>
        value ? <Tag color="green">启用</Tag> : <Tag>停用</Tag>,
    },
    { title: '时间段数', dataIndex: 'item_count', width: 100, render: (value?: number) => value ?? 0 },
    { title: '绑定通道', dataIndex: 'channel_count', width: 100, render: (value?: number) => value ?? 0 },
    {
      title: '抓图',
      dataIndex: 'snap',
      width: 80,
      render: (value?: boolean) => (value ? <Tag color="blue">开启</Tag> : '-'),
    },
    { title: '备注', dataIndex: 'remark', width: 220, ellipsis: true, render: (value?: string) => value || '-' },
    { title: '更新时间', dataIndex: 'updated_dt', width: 170, render: (value?: string) => fmtDT(value) },
    {
      title: '操作',
      key: 'action',
      width: 260,
      fixed: 'right',
      render: (_, row) => (
        <ActionButtons maxWidth={260}>
          {canUpdate ? (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => void openEdit(row)}>
              编辑
            </Button>
          ) : null}
          {canBind ? (
            <Button
              type="link"
              size="small"
              icon={<ApartmentOutlined />}
              onClick={() => void openBind(row)}
            >
              绑定通道
            </Button>
          ) : null}
          {canDelete ? (
            <Popconfirm title="确定删除该录像计划?" onConfirm={() => void handleDelete(row)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          ) : null}
        </ActionButtons>
      ),
    },
  ]

  return (
    <PageContainer
      title="录像计划"
      subTitle="维护一周内的录像时间段并绑定国标通道"
      extra={
        canCreate ? (
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增计划
          </Button>
        ) : null
      }
    >
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 12 }}
        message="本期只做配置，不做定时调度"
        description="后端尚未实现录像计划调度器：启用状态与时间段只会保存到数据库，不会自动向设备下发录像指令。"
      />

      <SearchBar
        fields={[{ name: 'keyword', label: '计划名称', placeholder: '请输入计划名称' }]}
        onSearch={(values) => {
          setKeyword(String(values.keyword ?? ''))
          setPage(1)
        }}
        onReset={() => {
          setKeyword('')
          setPage(1)
        }}
        loading={loading}
      />

      <Table<GbRecordPlanItem>
        rowKey={(row) => String(row.id ?? '')}
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
          showTotal: (count) => `共 ${count} 条`,
          onChange: (nextPage, nextPageSize) => {
            setPage(nextPage)
            setPageSize(nextPageSize)
          },
        }}
      />

      <Modal
        className="e2e-gb-record-plan-modal"
        title={editing ? '编辑录像计划' : '新增录像计划'}
        open={modalOpen}
        confirmLoading={saving}
        okButtonProps={{ disabled: detailLoading }}
        onOk={() => void handleSave()}
        onCancel={() => setModalOpen(false)}
        destroyOnHidden
        width={760}
      >
        <Spin spinning={detailLoading}>
          <Form form={form} layout="vertical">
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  name="name"
                  label="计划名称"
                  rules={[{ required: true, message: '请输入计划名称' }]}
                >
                  <Input placeholder="如：园区全天录像" maxLength={100} />
                </Form.Item>
              </Col>
              <Col span={6}>
                <Form.Item name="active" label="是否启用" valuePropName="checked">
                  <Switch checkedChildren="启用" unCheckedChildren="停用" />
                </Form.Item>
              </Col>
              <Col span={6}>
                <Form.Item name="snap" label="抓图" valuePropName="checked">
                  <Switch checkedChildren="开" unCheckedChildren="关" />
                </Form.Item>
              </Col>
            </Row>

            <Form.Item name="remark" label="备注">
              <Input.TextArea rows={2} maxLength={500} placeholder="请输入备注" />
            </Form.Item>

            <Form.Item label="时间段（按星期配置，可多条）" required>
              <Form.List
                name="items"
                rules={[
                  {
                    validator: async (_, value: PlanItemFormValue[] | undefined) => {
                      if (!value || value.length === 0) {
                        throw new Error('请至少添加一个时间段')
                      }
                    },
                  },
                ]}
              >
                {(fields, { add, remove }, { errors }) => (
                  <>
                    {fields.map((field) => (
                      <Row gutter={8} key={field.key} align="middle" style={{ marginBottom: 8 }}>
                        <Col span={5}>
                          <Form.Item
                            {...field}
                            name={[field.name, 'week_day']}
                            rules={[{ required: true, message: '请选择星期' }]}
                            style={{ marginBottom: 0 }}
                          >
                            <Select
                              placeholder="星期"
                              options={WEEK_DAY_OPTIONS}
                            />
                          </Form.Item>
                        </Col>
                        <Col span={7}>
                          <Form.Item
                            {...field}
                            name={[field.name, 'start_time']}
                            rules={[{ required: true, message: '请选择开始时间' }]}
                            style={{ marginBottom: 0 }}
                          >
                            <TimePicker format="HH:mm:ss" style={{ width: '100%' }} placeholder="开始时间" />
                          </Form.Item>
                        </Col>
                        <Col span={7}>
                          <Form.Item
                            {...field}
                            name={[field.name, 'end_time']}
                            rules={[{ required: true, message: '请选择结束时间' }]}
                            style={{ marginBottom: 0 }}
                          >
                            <TimePicker format="HH:mm:ss" style={{ width: '100%' }} placeholder="结束时间" />
                          </Form.Item>
                        </Col>
                        <Col span={5}>
                          <Button
                            type="text"
                            danger
                            icon={<MinusCircleOutlined />}
                            onClick={() => remove(field.name)}
                          >
                            删除
                          </Button>
                        </Col>
                      </Row>
                    ))}
                    <Button
                      type="dashed"
                      block
                      icon={<PlusOutlined />}
                      onClick={() =>
                        add({
                          week_day: 1,
                          start_time: timeToDayjs('08:00:00'),
                          end_time: timeToDayjs('18:00:00'),
                        })
                      }
                    >
                      添加时间段
                    </Button>
                    <Form.ErrorList errors={errors} />
                  </>
                )}
              </Form.List>
            </Form.Item>

            <Alert
              type="info"
              showIcon
              message="本期仅保存配置，不会触发定时录像；时间格式为 HH:mm:ss，week_day 0 表示周日。"
            />
          </Form>
        </Spin>
      </Modal>

      <Modal
        className="e2e-gb-record-plan-bind-modal"
        title={`绑定通道${bindPlan?.name ? `：${bindPlan.name}` : ''}`}
        open={bindOpen}
        confirmLoading={bindSaving}
        okButtonProps={{ disabled: bindLoading }}
        onOk={() => void handleBind()}
        onCancel={() => setBindOpen(false)}
        destroyOnHidden
        width={620}
      >
        <Spin spinning={bindLoading}>
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <div style={{ color: '#8c8c8c' }}>
              提交时会整体替换该计划已绑定的通道（后端 `linkChannels` 语义）。
            </div>
            <Select<number[]>
              mode="multiple"
              allowClear
              showSearch
              optionFilterProp="label"
              placeholder="请选择要绑定到该计划的通道"
              style={{ width: '100%' }}
              value={bindChannelIds}
              options={channelSelectOptions}
              onChange={(values) => setBindChannelIds(values)}
            />
            <div style={{ color: '#8c8c8c', fontSize: 12 }}>
              已选择 {bindChannelIds.length} 个通道
            </div>
          </Space>
        </Spin>
      </Modal>
    </PageContainer>
  )
}

export default RecordPlanList
