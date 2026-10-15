import { useCallback, useEffect, useState } from 'react'
import {
  Badge, Button, Descriptions, Modal, Popconfirm, Space, Table, Tabs, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { CheckOutlined, DeleteOutlined, EyeOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import ActionButtons from '../../components/ActionButtons'
import StatusTag from '../../components/StatusTag'
import { NotificationController } from '../../api/generated/NotificationController'
import { useAuthStore } from '../../store/authStore'
import type { Notification } from '../../api/generated/definitions/Notification'

/** 通知类型 → 中文名。category 取值 alarm/risk/inspect */
const CATEGORY_LABELS: Record<string, string> = {
  alarm: '设备告警',
  risk: '风险通知',
  inspect: '巡检通知',
}

/** 通知类型 → StatusTag 的 type 取值 */
const CATEGORY_TAG_TYPES: Record<string, 'alarm' | 'risk' | 'inspect'> = {
  alarm: 'alarm',
  risk: 'risk',
  inspect: 'inspect',
}

/**
 * 系统通知：按类型/未读筛选的分页列表。
 * 通知由后端在告警、风险、巡检流程中写入（ac_notification），未读数用于顶部红点。
 */
export default function NotificationList() {
  const [data, setData] = useState<Notification[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  // tab: all / alarm / risk / inspect / unread
  const [tab, setTab] = useState<string>('all')
  const [unreadCount, setUnreadCount] = useState(0)
  const [markingAll, setMarkingAll] = useState(false)

  const [detailOpen, setDetailOpen] = useState(false)
  const [detail, setDetail] = useState<Notification | null>(null)

  const permCodes = useAuthStore((s) => s.permCodes)
  // permCodes 为空说明菜单权限尚未加载，此时不隐藏按钮，避免整页操作被锁死
  const can = (code: string) => permCodes.length === 0 || permCodes.includes(code)
  const canMark = can('system:notify:mark')
  const canDelete = can('system:notify:delete')

  /** tab 转换为接口查询条件：alarm/risk/inspect 走 category，unread 走 read=false */
  const buildFilter = (): { category?: string; read?: boolean } => {
    if (tab === 'unread') return { read: false }
    if (tab === 'all') return {}
    return { category: tab }
  }

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const filter = buildFilter()
      const res = await NotificationController.pageNotifications(page, pageSize, filter.category, filter.read)
      if (res.code === 200) {
        setData(res.data?.items ?? [])
        setTotal(res.data?.total ?? 0)
      } else {
        message.error(res.message || '获取通知列表失败')
      }
    } finally {
      setLoading(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize, tab])

  const fetchUnreadCount = useCallback(async () => {
    const res = await NotificationController.unreadNotificationCount()
    if (res.code === 200) setUnreadCount(res.data ?? 0)
  }, [])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  useEffect(() => {
    fetchUnreadCount()
  }, [fetchUnreadCount])

  const handleMarkRead = async (record: Notification) => {
    if (record.read === true) return
    try {
      const res = await NotificationController.markNotificationRead(String(record.id))
      if (res.code === 200) {
        message.success('已标记为已读')
        fetchData()
        fetchUnreadCount()
      }
    } catch {
      message.error('操作失败')
    }
  }

  const handleMarkAllRead = async () => {
    setMarkingAll(true)
    try {
      const res = await NotificationController.markAllNotificationsRead()
      if (res.code === 200) {
        message.success('已全部标记为已读')
        fetchData()
        fetchUnreadCount()
      } else {
        message.error(res.message || '操作失败')
      }
    } catch {
      message.error('操作失败')
    } finally {
      setMarkingAll(false)
    }
  }

  const handleDelete = async (record: Notification) => {
    try {
      const res = await NotificationController.deleteNotification(String(record.id))
      if (res.code === 200) {
        message.success('删除成功')
        fetchData()
        fetchUnreadCount()
      } else {
        message.error(res.message || '删除失败')
      }
    } catch {
      message.error('删除失败')
    }
  }

  /** 查看详情并顺带标记已读 */
  const openDetail = async (record: Notification) => {
    setDetail(record)
    setDetailOpen(true)
    await handleMarkRead(record)
  }

  const columns: ColumnsType<Notification> = [
    {
      title: '类型', dataIndex: 'category', key: 'category', width: 120,
      render: (value: string) => (
        <StatusTag status={CATEGORY_LABELS[value] ?? value ?? '-'} type={CATEGORY_TAG_TYPES[value] ?? 'alarm'} />
      ),
    },
    { title: '标题', dataIndex: 'title', key: 'title', width: 220, ellipsis: true },
    { title: '内容', dataIndex: 'content', key: 'content', width: 320, ellipsis: true },
    {
      title: '关联对象', key: 'ref', width: 150,
      render: (_, record) => (record.ref_type ? `${record.ref_type} #${record.ref_id ?? ''}` : '-'),
    },
    {
      title: '状态', key: 'read', width: 100,
      render: (_, record) => (record.read === true ? <Tag>已读</Tag> : <Tag color="red">未读</Tag>),
    },
    {
      title: '时间', dataIndex: 'created_dt', key: 'created_dt', width: 170,
      render: (value: string) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
    {
      title: '操作', key: 'action', width: 180, fixed: 'right' as const,
      render: (_, record) => (
        <ActionButtons>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => openDetail(record)}>查看</Button>
          {canMark && record.read !== true && (
            <Button type="link" size="small" icon={<CheckOutlined />} onClick={() => handleMarkRead(record)}>
              标记已读
            </Button>
          )}
          {canDelete && (
            <Popconfirm title="确定删除?" onConfirm={() => handleDelete(record)}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>删除</Button>
            </Popconfirm>
          )}
        </ActionButtons>
      ),
    },
  ]

  return (
    <PageContainer
      title="系统通知"
      subTitle="设备告警、风险与巡检通知的站内消息"
      extra={canMark ? (
        <Space>
          <Badge count={unreadCount} size="small" offset={[-2, 2]}>
            <span style={{ color: '#8c8c8c' }}>未读</span>
          </Badge>
          <Button icon={<CheckOutlined />} loading={markingAll} onClick={handleMarkAllRead}>全部已读</Button>
        </Space>
      ) : undefined}
    >
      <Tabs
        activeKey={tab}
        onChange={(key) => {
          setTab(key)
          setPage(1)
        }}
        items={[
          { key: 'all', label: '全部' },
          { key: 'alarm', label: '设备告警' },
          { key: 'risk', label: '风险通知' },
          { key: 'inspect', label: '巡检通知' },
          { key: 'unread', label: `未读(${unreadCount})` },
        ]}
      />

      <Table
        rowKey="id"
        size="small"
        scroll={{ x: 'max-content' }}
        columns={columns}
        dataSource={data}
        loading={loading}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => {
            setPage(p)
            setPageSize(ps)
          },
        }}
      />

      <Modal
        className="e2e-notification-modal"
        title="通知详情"
        open={detailOpen}
        onCancel={() => setDetailOpen(false)}
        footer={<Button onClick={() => setDetailOpen(false)}>关闭</Button>}
        width={640}
      >
        <Descriptions column={1} size="small" bordered>
          <Descriptions.Item label="类型">
            {CATEGORY_LABELS[detail?.category ?? ''] ?? detail?.category ?? '-'}
          </Descriptions.Item>
          <Descriptions.Item label="标题">{detail?.title ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="内容">{detail?.content ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="关联对象">
            {detail?.ref_type ? `${detail.ref_type} #${detail.ref_id ?? ''}` : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="状态">
            {detail?.read === true ? '已读' : '未读'}
          </Descriptions.Item>
          <Descriptions.Item label="时间">
            {detail?.created_dt ? dayjs(detail.created_dt).format('YYYY-MM-DD HH:mm:ss') : '-'}
          </Descriptions.Item>
        </Descriptions>
      </Modal>
    </PageContainer>
  )
}
