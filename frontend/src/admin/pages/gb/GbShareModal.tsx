import React, { useCallback, useEffect, useState } from 'react'
import {
  Alert, Button, Empty, Input, InputNumber, Modal, Popconfirm, Space, Table, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { CopyOutlined, DeleteOutlined, LinkOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import axios from 'axios'
import { GbShareController } from '../../api/generated/GbShareController'
import type { GbShareTokenItem } from '../../api/generated/definitions/GbShareTokenItem'
import type { GbShareTokenCreatedResp } from '../../api/generated/definitions/GbShareTokenCreatedResp'
import { useAuthStore } from '../../store/authStore'
import { formatDT } from '../../types/business'

/** 请求失败提示：优先展示后端返回的 message */
function apiErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError<{ message?: string }>(err)) {
    return err.response?.data?.message || err.message || fallback
  }
  return err instanceof Error ? err.message : fallback
}

/**
 * 生成可访问的分享链接。
 *
 * 管理后台的 `BASE_URL` 为 `/admin/`，分享页路由为 `/share/gb/:token`；
 * 生产环境拼成 `<origin>/admin/share/gb/<token>`。开发模式 BASE_URL 可能是
 * `./`，这里做一次归一化，避免拼出双斜杠或丢前缀。
 */
export function buildGbShareUrl(token: string): string {
  const base = import.meta.env.BASE_URL || '/'
  const path = `share/gb/${encodeURIComponent(token)}`

  if (base.startsWith('http')) {
    return new URL(path, base.endsWith('/') ? base : `${base}/`).toString()
  }

  let normalized = base.startsWith('/') ? base : `/${base}`
  if (!normalized.endsWith('/')) normalized = `${normalized}/`
  return `${window.location.origin}${normalized}${path}`
}

interface GbShareModalProps {
  open: boolean
  channelId?: number
  channelName?: string
  onClose: () => void
}

/**
 * 通道免登录分享管理：生成令牌、复制分享链接、查看与吊销令牌。
 *
 * 令牌明文只在创建响应中出现一次（后端只存哈希），因此创建成功后在顶部
 * 展示完整链接供复制；列表只展示前缀。
 */
const GbShareModal: React.FC<GbShareModalProps> = ({ open, channelId, channelName, onClose }) => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canList = permCodes.includes('gb:share:list')
  const canCreate = permCodes.includes('gb:share:create')
  const canRevoke = permCodes.includes('gb:share:revoke')

  const [tokens, setTokens] = useState<GbShareTokenItem[]>([])
  const [loading, setLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [revokingId, setRevokingId] = useState<number | null>(null)
  const [ttlSeconds, setTtlSeconds] = useState<number>(3600)
  const [remark, setRemark] = useState('')
  const [created, setCreated] = useState<GbShareTokenCreatedResp | null>(null)

  const loadTokens = useCallback(async () => {
    if (!channelId || !canList) {
      setTokens([])
      return
    }
    setLoading(true)
    try {
      const res = await GbShareController.listGbShareTokens(channelId)
      setTokens(res.data ?? [])
    } catch (err) {
      message.error(apiErrorMessage(err, '获取分享令牌失败'))
    } finally {
      setLoading(false)
    }
  }, [channelId, canList])

  useEffect(() => {
    if (!open) return
    setCreated(null)
    setRemark('')
    setTtlSeconds(3600)
    void loadTokens()
  }, [open, loadTokens])

  const handleCreate = async () => {
    if (!channelId) return
    const ttl = ttlSeconds ?? 3600
    if (ttl < 60 || ttl > 86400) {
      message.warning('有效期需在 60 ~ 86400 秒之间')
      return
    }
    setCreating(true)
    try {
      const res = await GbShareController.createGbShareToken(channelId, {
        ttl_seconds: ttl,
        remark: remark.trim() || undefined,
      })
      setCreated(res.data ?? null)
      message.success('分享令牌已生成，请及时复制链接')
      void loadTokens()
    } catch (err) {
      message.error(apiErrorMessage(err, '生成分享令牌失败'))
    } finally {
      setCreating(false)
    }
  }

  const handleRevoke = async (row: GbShareTokenItem) => {
    if (!channelId || !row.id) return
    setRevokingId(row.id)
    try {
      await GbShareController.revokeGbShareToken(channelId, row.id)
      message.success('已吊销该分享令牌')
      void loadTokens()
    } catch (err) {
      message.error(apiErrorMessage(err, '吊销分享令牌失败'))
    } finally {
      setRevokingId(null)
    }
  }

  const handleCopy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text)
      message.success('分享链接已复制')
    } catch {
      message.warning('当前环境不支持自动复制，请手动选中复制')
    }
  }

  const shareUrl = created?.token ? buildGbShareUrl(created.token) : ''

  const columns: ColumnsType<GbShareTokenItem> = [
    {
      title: '令牌前缀',
      dataIndex: 'token_prefix',
      width: 150,
      render: (value?: string) => (
        <span style={{ fontFamily: 'Consolas, Monaco, monospace' }}>{value ? `${value}…` : '-'}</span>
      ),
    },
    { title: '备注', dataIndex: 'remark', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '有效期至', dataIndex: 'expires_dt', width: 170, render: (v?: string) => formatDT(v) },
    {
      title: '使用次数',
      dataIndex: 'use_count',
      width: 100,
      render: (v?: number) => v ?? 0,
    },
    { title: '最近使用', dataIndex: 'last_used_dt', width: 170, render: (v?: string) => formatDT(v) },
    { title: '创建人', dataIndex: 'created_by', width: 110, render: (v?: string) => v || '-' },
    { title: '创建时间', dataIndex: 'created_dt', width: 170, render: (v?: string) => formatDT(v) },
    {
      title: '操作',
      key: 'action',
      width: 110,
      fixed: 'right',
      render: (_, row) =>
        canRevoke && row.id ? (
          <Popconfirm
            title="吊销后该链接立即失效，确定继续?"
            onConfirm={() => void handleRevoke(row)}
          >
            <Button
              className="e2e-gb-share-revoke-btn"
              type="link"
              size="small"
              danger
              icon={<DeleteOutlined />}
              loading={revokingId === row.id}
            >
              吊销
            </Button>
          </Popconfirm>
        ) : (
          <span style={{ color: '#bfbfbf', fontSize: 12 }}>-</span>
        ),
    },
  ]

  return (
    <Modal
      className="e2e-gb-share-modal"
      title={`分享管理${channelName ? ` - ${channelName}` : ''}`}
      open={open}
      onCancel={onClose}
      footer={<Button onClick={onClose}>关闭</Button>}
      width={1000}
      destroyOnHidden
    >
      {!channelId ? (
        <Empty description="请先在页面顶部选择一个通道" />
      ) : (
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          {!canList ? (
            <Alert type="warning" showIcon message="当前账号无分享令牌查看权限（gb:share:list）" />
          ) : null}

          {canCreate ? (
            <Alert
              type="info"
              showIcon
              message="生成免登录分享链接"
              description="令牌明文只在生成时展示一次，请立即复制；过期或吊销后匿名链接立即失效。"
              action={
                <Space size={8}>
                  <InputNumber
                    min={60}
                    max={86400}
                    step={600}
                    value={ttlSeconds}
                    onChange={(value) => setTtlSeconds(typeof value === 'number' ? value : 3600)}
                    addonAfter="秒"
                    style={{ width: 150 }}
                  />
                  <Input
                    placeholder="备注（可选）"
                    value={remark}
                    onChange={(event) => setRemark(event.target.value)}
                    maxLength={100}
                    style={{ width: 180 }}
                  />
                  <Button
                    className="e2e-gb-share-create-btn"
                    type="primary"
                    icon={<PlusOutlined />}
                    loading={creating}
                    onClick={() => void handleCreate()}
                  >
                    生成
                  </Button>
                </Space>
              }
            />
          ) : null}

          {created?.token ? (
            <Alert
              className="e2e-gb-share-token-result"
              type="success"
              showIcon
              message="分享链接已生成（仅本次可见）"
              description={
                <Space direction="vertical" size={4} style={{ width: '100%' }}>
                  <span style={{ wordBreak: 'break-all', fontFamily: 'Consolas, Monaco, monospace' }}>
                    {shareUrl}
                  </span>
                  <Space size={8}>
                    <Button
                      className="e2e-gb-share-copy-btn"
                      size="small"
                      icon={<CopyOutlined />}
                      onClick={() => void handleCopy(shareUrl)}
                    >
                      复制链接
                    </Button>
                    <Tag icon={<LinkOutlined />} color="blue" style={{ marginInlineEnd: 0 }}>
                      有效期至 {formatDT(created.expires_dt)}
                    </Tag>
                  </Space>
                </Space>
              }
            />
          ) : null}

          <Space style={{ width: '100%', justifyContent: 'flex-end' }}>
            <Button
              size="small"
              icon={<ReloadOutlined />}
              loading={loading}
              disabled={!canList}
              onClick={() => void loadTokens()}
            >
              刷新
            </Button>
          </Space>

          <Table<GbShareTokenItem>
            size="small"
            rowKey={(row) => String(row.id ?? row.token_prefix ?? '')}
            loading={loading}
            columns={columns}
            dataSource={tokens}
            scroll={{ x: 'max-content' }}
            pagination={false}
            locale={{ emptyText: canList ? '暂无有效分享令牌' : '无查看权限' }}
          />
        </Space>
      )}
    </Modal>
  )
}

export default GbShareModal
