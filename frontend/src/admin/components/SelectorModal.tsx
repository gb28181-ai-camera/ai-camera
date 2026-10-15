import { useCallback, useEffect, useState } from 'react'
import { Button, Input, Modal, Space, Table } from 'antd'
import type { ColumnsType } from 'antd/es/table'

export interface SelectorFetchParams {
  page: number
  page_size: number
  /** 关键字（弹窗顶部搜索框） */
  keyword?: string
}

export interface SelectorFetchResult<T> {
  items: T[]
  total: number
}

export interface SelectorModalProps<T extends Record<string, any>> {
  /**
   * 透传到 Modal（`.e2e-xxx`）与触发按钮（`.e2e-xxx-btn`）。
   * 例：className="e2e-device-select" → Modal 为 `.e2e-device-select`，按钮为 `.e2e-device-select-btn`。
   */
  className?: string
  /** 弹窗标题 */
  title: string
  /** 表格列定义 */
  columns: ColumnsType<T>
  /** 数据加载函数 */
  fetchData: (params: SelectorFetchParams) => Promise<SelectorFetchResult<T>>
  /** 行唯一键字段名 */
  rowKey: string
  /** 单选 / 多选，默认 single */
  mode?: 'single' | 'multiple'
  /** 受控开关；不传则由组件内部管理（并通过触发按钮打开） */
  open?: boolean
  onCancel: () => void
  /** 确定回调，始终回传选中的行数组 */
  onOk: (selected: T[]) => void
  /** 受控模式下由外部接管打开/关闭 */
  onOpenChange?: (open: boolean) => void
  width?: number
  searchPlaceholder?: string
  /** 传入后渲染触发按钮（类名为 `${className}-btn`） */
  triggerText?: string
  disabled?: boolean
}

/**
 * 通用选择弹窗（单选 / 多选），内置关键字搜索 + 分页 + 已选高亮。
 *
 * 用法一（非受控，自带触发按钮）：
 * ```tsx
 * <SelectorModal className="e2e-device-select" triggerText="选择设备" title="选择设备"
 *   columns={columns} rowKey="id" fetchData={fetch} onOk={setRows} onCancel={() => {}} />
 * ```
 * 用法二（受控）：
 * ```tsx
 * <SelectorModal className="e2e-device-select" title="选择设备" open={open}
 *   columns={columns} rowKey="id" fetchData={fetch} onOk={onPick}
 *   onCancel={() => setOpen(false)} />
 * ```
 */
export default function SelectorModal<T extends Record<string, any>>({
  className,
  title,
  columns,
  fetchData,
  rowKey,
  mode = 'single',
  open,
  onCancel,
  onOk,
  onOpenChange,
  width = 880,
  searchPlaceholder = '请输入关键字',
  triggerText,
  disabled,
}: SelectorModalProps<T>) {
  const isControlled = open !== undefined
  const [innerOpen, setInnerOpen] = useState(false)
  const visible = isControlled ? Boolean(open) : innerOpen

  const [items, setItems] = useState<T[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [keyword, setKeyword] = useState('')
  const [loading, setLoading] = useState(false)
  const [selectedRows, setSelectedRows] = useState<T[]>([])

  const openModal = () => {
    if (disabled) return
    if (isControlled) {
      onOpenChange?.(true)
    } else {
      setInnerOpen(true)
    }
  }

  const closeModal = () => {
    if (isControlled) {
      onOpenChange?.(false)
    } else {
      setInnerOpen(false)
    }
    onCancel()
  }

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await fetchData({ page, page_size: pageSize, keyword: keyword || undefined })
      setItems(Array.isArray(res?.items) ? res.items : [])
      setTotal(Number(res?.total ?? 0))
    } catch {
      setItems([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }, [fetchData, page, pageSize, keyword])

  // 打开时重置选择并加载首页
  useEffect(() => {
    if (!visible) return
    setSelectedRows([])
    setKeyword('')
    setPage(1)
  }, [visible])

  useEffect(() => {
    if (!visible) return
    void load()
  }, [visible, load])

  const handleOk = () => {
    onOk(selectedRows)
    closeModal()
  }

  const modalNode = (
    <Modal
      className={className}
      title={title}
      open={visible}
      onCancel={closeModal}
      onOk={handleOk}
      width={width}
      okText="确定"
      cancelText="取消"
      maskClosable={false}
    >
      <Space direction="vertical" style={{ width: '100%' }} size={12}>
        <Input.Search
          allowClear
          placeholder={searchPlaceholder}
          onSearch={(value) => {
            setPage(1)
            setKeyword(value.trim())
          }}
          style={{ maxWidth: 280 }}
        />
        <Table<T>
          rowKey={rowKey}
          size="small"
          loading={loading}
          columns={columns}
          dataSource={items}
          scroll={{ x: 'max-content' }}
          rowSelection={{
            type: mode === 'multiple' ? 'checkbox' : 'radio',
            selectedRowKeys: selectedRows.map((row) => row[rowKey]),
            onChange: (_keys, rows) => setSelectedRows(rows),
          }}
          onRow={(record) => ({
            onClick: () => {
              if (mode === 'multiple') return
              setSelectedRows([record])
            },
          })}
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
      </Space>
    </Modal>
  )

  if (isControlled && !triggerText) {
    return modalNode
  }

  return (
    <>
      {triggerText ? (
        <Button
          className={className ? `${className}-btn` : undefined}
          onClick={openModal}
          disabled={disabled}
        >
          {triggerText}
        </Button>
      ) : null}
      {modalNode}
    </>
  )
}
