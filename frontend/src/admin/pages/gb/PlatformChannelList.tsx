import React, {
  forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState,
} from 'react'
import {
  Alert, Button, Empty, Form, Input, message, Modal, Popconfirm, Select, Space, Table, Tag,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { DeleteOutlined, EditOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import axios from 'axios'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import SelectorModal from '../../components/SelectorModal'
import type { SelectorFetchParams, SelectorFetchResult } from '../../components/SelectorModal'
import { GbPlatformController } from '../../api/generated/GbPlatformController'
import { GbChannelController } from '../../api/generated/GbChannelController'
import type { GbPlatformItem } from '../../api/generated/definitions/GbPlatformItem'
import type { GbPlatformChannelItem } from '../../api/generated/definitions/GbPlatformChannelItem'
import type { GbChannelItem } from '../../api/generated/definitions/GbChannelItem'
import { useAuthStore } from '../../store/authStore'
import { formatDT } from '../../types/business'

/** 请求失败提示：优先展示后端返回的 message。 */
function apiErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError<{ message?: string }>(err)) {
    return err.response?.data?.message || err.message || fallback
  }
  return err instanceof Error ? err.message : fallback
}

interface CustomFormValues {
  custom_name: string
  custom_device_id: string
}

interface CustomFormHandle {
  submit: () => Promise<void>
}

interface CustomFormContentProps {
  target: GbPlatformChannelItem | null
  onSubmit: (values: CustomFormValues) => Promise<void>
}

/**
 * 定制级联通道表单内容：form 与 Form 同挂载在 Modal children 中，
 * 父组件不再在弹窗打开前调用 resetFields/setFieldsValue。
 */
const CustomFormContent = forwardRef<CustomFormHandle, CustomFormContentProps>(
  function CustomFormContent({ target, onSubmit }, ref) {
    const [form] = Form.useForm<CustomFormValues>()

    useImperativeHandle(ref, () => ({
      async submit() {
        let values: CustomFormValues
        try {
          values = await form.validateFields()
        } catch {
          return
        }
        await onSubmit(values)
      },
    }), [form, onSubmit])

    return (
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          custom_name: target?.custom_name || target?.channel_name || '',
          custom_device_id: target?.custom_device_id || target?.gb_channel_id || '',
        }}
      >
        <Form.Item
          name="custom_name"
          label="定制名称"
          rules={[{ required: true, message: '请输入定制名称' }]}
        >
          <Input placeholder="上级平台看到的通道名称" maxLength={255} />
        </Form.Item>
        <Form.Item
          name="custom_device_id"
          label="定制编码"
          rules={[{ required: true, message: '请输入定制国标编码' }]}
        >
          <Input placeholder="上级平台看到的通道国标编码" maxLength={20} />
        </Form.Item>
      </Form>
    )
  },
)

/** 添加通道弹窗的表格列。 */
const channelColumns: ColumnsType<GbChannelItem> = [
  { title: '通道名称', dataIndex: 'name', width: 180, ellipsis: true, render: (v?: string) => v || '-' },
  { title: '国标编码', dataIndex: 'gb_channel_id', width: 190, ellipsis: true, render: (v?: string) => v || '-' },
  { title: '设备名称', dataIndex: 'device_name', width: 170, ellipsis: true, render: (v?: string) => v || '-' },
  { title: '设备编号', dataIndex: 'device_code', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
  { title: '所属区域', dataIndex: 'region_name', width: 130, ellipsis: true, render: (v?: string) => v || '-' },
  { title: '业务分组', dataIndex: 'group_name', width: 130, ellipsis: true, render: (v?: string) => v || '-' },
  {
    title: '状态',
    dataIndex: 'status',
    width: 90,
    render: (v?: string) => (v ? <Tag>{v === 'ON' ? '在线' : v}</Tag> : '-'),
  },
]

/**
 * 级联通道：维护某个上级平台「可见」的本地国标通道。
 *
 * 先选择上级平台，再分页查看已关联通道；支持添加、定制（名称 / 国标编码）与移除。
 */
const PlatformChannelList: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canAdd = permCodes.includes('gb:platform-channel:add')
  const canUpdate = permCodes.includes('gb:platform-channel:update')
  const canDelete = permCodes.includes('gb:platform-channel:delete')

  const [platforms, setPlatforms] = useState<GbPlatformItem[]>([])
  const [platformsLoading, setPlatformsLoading] = useState(false)
  const [platformId, setPlatformId] = useState<number | undefined>()

  const [data, setData] = useState<GbPlatformChannelItem[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')

  const [addOpen, setAddOpen] = useState(false)
  const [adding, setAdding] = useState(false)

  const [customOpen, setCustomOpen] = useState(false)
  const [customTarget, setCustomTarget] = useState<GbPlatformChannelItem | null>(null)
  const [customSaving, setCustomSaving] = useState(false)
  const customFormRef = useRef<CustomFormHandle>(null)

  const loadPlatforms = useCallback(async () => {
    setPlatformsLoading(true)
    try {
      const res = await GbPlatformController.pageGbPlatforms(1, 200)
      const items = res.data?.items ?? []
      setPlatforms(items)
      setPlatformId((prev) => prev ?? items[0]?.id)
    } catch (err) {
      message.error(apiErrorMessage(err, '获取上级平台列表失败'))
    } finally {
      setPlatformsLoading(false)
    }
  }, [])

  useEffect(() => { void loadPlatforms() }, [loadPlatforms])

  const fetchChannels = useCallback(async () => {
    if (!platformId) {
      setData([])
      setTotal(0)
      return
    }
    setLoading(true)
    try {
      const res = await GbPlatformController.pageGbPlatformChannels(
        platformId, page, pageSize, keyword || undefined,
      )
      setData(res.data?.items ?? [])
      setTotal(Number(res.data?.total ?? 0))
    } catch (err) {
      message.error(apiErrorMessage(err, '获取级联通道失败'))
    } finally {
      setLoading(false)
    }
  }, [platformId, page, pageSize, keyword])

  useEffect(() => { void fetchChannels() }, [fetchChannels])

  const handleSearch = (values: Record<string, unknown>) => {
    setKeyword(typeof values.keyword === 'string' ? values.keyword : '')
    setPage(1)
  }

  const handleReset = () => {
    setKeyword('')
    setPage(1)
  }

  /* ── 添加通道 ── */
  const fetchGbChannels = useCallback(async ({
    page: p, page_size, keyword: kw,
  }: SelectorFetchParams): Promise<SelectorFetchResult<GbChannelItem>> => {
    const res = await GbChannelController.pageGbChannels(p, page_size, kw)
    return { items: res.data?.items ?? [], total: Number(res.data?.total ?? 0) }
  }, [])

  const submitAdd = async (selected: GbChannelItem[]) => {
    if (!platformId) return
    const channelIds = selected.flatMap((item) => (item.id === undefined ? [] : [item.id]))
    if (channelIds.length === 0) {
      message.warning('请选择要添加的通道')
      return
    }
    setAdding(true)
    try {
      await GbPlatformController.addGbPlatformChannels(platformId, { channel_ids: channelIds })
      message.success('添加成功')
      setAddOpen(false)
      void fetchChannels()
    } catch (err) {
      message.error(apiErrorMessage(err, '添加级联通道失败'))
    } finally {
      setAdding(false)
    }
  }

  /* ── 定制通道 ── */
  const openCustom = (row: GbPlatformChannelItem) => {
    setCustomTarget(row)
    setCustomOpen(true)
  }

  const submitCustom = async (values: CustomFormValues) => {
    if (!platformId || !customTarget?.channel_id) return
    setCustomSaving(true)
    try {
      await GbPlatformController.updateGbPlatformChannelCustom(
        platformId,
        customTarget.channel_id,
        {
          custom_name: values.custom_name.trim(),
          custom_device_id: values.custom_device_id.trim(),
        },
      )
      message.success('修改成功')
      setCustomOpen(false)
      void fetchChannels()
    } catch (err) {
      message.error(apiErrorMessage(err, '修改级联通道失败'))
    } finally {
      setCustomSaving(false)
    }
  }

  const handleRemove = async (row: GbPlatformChannelItem) => {
    if (!platformId || !row.channel_id) return
    try {
      await GbPlatformController.removeGbPlatformChannels(platformId, {
        channel_ids: [row.channel_id],
      })
      message.success('移除成功')
      void fetchChannels()
    } catch (err) {
      message.error(apiErrorMessage(err, '移除级联通道失败'))
    }
  }

  const columns: ColumnsType<GbPlatformChannelItem> = [
    { title: '通道名称', dataIndex: 'channel_name', width: 170, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '国标编码', dataIndex: 'gb_channel_id', width: 190, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '设备名称', dataIndex: 'device_name', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '设备编号', dataIndex: 'device_code', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    {
      title: '定制名称',
      dataIndex: 'custom_name',
      width: 170,
      ellipsis: true,
      render: (v?: string) => v || '-',
    },
    {
      title: '定制编码',
      dataIndex: 'custom_device_id',
      width: 190,
      ellipsis: true,
      render: (v?: string) => v || '-',
    },
    { title: '更新时间', dataIndex: 'updated_dt', width: 170, render: (v?: string) => formatDT(v) },
    {
      title: '操作',
      key: 'action',
      width: 170,
      fixed: 'right',
      render: (_: unknown, row) => (
        <ActionButtons maxWidth={170}>
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openCustom(row)}>
              定制
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定将该通道从上级平台目录移除?" onConfirm={() => void handleRemove(row)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                移除
              </Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  return (
    <PageContainer
      title="级联通道"
      subTitle="维护上级平台可见的本地国标通道及其定制信息"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => { void loadPlatforms(); void fetchChannels() }} loading={loading || platformsLoading}>
            刷新
          </Button>
          {canAdd && (
            <Button
              type="primary"
              icon={<PlusOutlined />}
              disabled={!platformId}
              onClick={() => setAddOpen(true)}
            >
              添加通道
            </Button>
          )}
        </Space>
      }
    >
      <Space wrap style={{ marginBottom: 16 }}>
        <span>上级平台：</span>
        <Select<number>
          style={{ width: 320 }}
          placeholder="请选择上级平台"
          loading={platformsLoading}
          value={platformId}
          onChange={(value) => { setPlatformId(value); setPage(1) }}
          showSearch
          optionFilterProp="label"
          options={platforms.flatMap((item) => (
            item.id === undefined
              ? []
              : [{
                  label: `${item.name || '-'}（${item.server_gb_id || '-'}）`,
                  value: item.id,
                }]
          ))}
        />
      </Space>

      {!platformId && !platformsLoading ? (
        <Empty description="暂无可选上级平台，请先在「上级平台」页面新增平台" />
      ) : (
        <>
          <SearchBar
            fields={[{ name: 'keyword', label: '关键字', placeholder: '定制名称 / 定制编码' }]}
            onSearch={handleSearch}
            onReset={handleReset}
            loading={loading}
          />

          <Table<GbPlatformChannelItem>
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
        </>
      )}

      {/* 添加通道（多选） */}
      <SelectorModal<GbChannelItem>
        className="e2e-gb-platform-channel-add-modal"
        title="添加通道到上级平台"
        mode="multiple"
        open={addOpen}
        onCancel={() => setAddOpen(false)}
        onOk={(selected) => void submitAdd(selected)}
        columns={channelColumns}
        fetchData={fetchGbChannels}
        rowKey="id"
        searchPlaceholder="请输入通道名称 / 国标编码"
        width={1000}
        disabled={adding}
      />

      {/* 定制通道名称 / 国标编码 */}
      <Modal
        className="e2e-gb-platform-channel-modal"
        title={`定制级联通道${customTarget?.channel_name ? ` - ${customTarget.channel_name}` : ''}`}
        open={customOpen}
        onCancel={() => setCustomOpen(false)}
        onOk={() => void customFormRef.current?.submit()}
        confirmLoading={customSaving}
        destroyOnHidden
        width={640}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="定制编码全局唯一"
          description="同一通道推送给多个上级平台时，需要为其中一个平台修改定制编码。"
        />
        <CustomFormContent
          ref={customFormRef}
          target={customTarget}
          onSubmit={submitCustom}
        />
      </Modal>
    </PageContainer>
  )
}

export default PlatformChannelList
