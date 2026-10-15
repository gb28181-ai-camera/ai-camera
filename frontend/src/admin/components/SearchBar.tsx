import { Button, Form, Input, Select, Space } from 'antd'
import { ReloadOutlined, SearchOutlined } from '@ant-design/icons'
import type { ReactNode } from 'react'

export interface SearchBarField {
  /** 表单字段名，会原样传给 onSearch 的 values */
  name: string
  /** 表单项标签 */
  label: string
  /** 占位提示 */
  placeholder?: string
  /**
   * 控件类型，默认 input。
   * select 为可选的扩展能力 —— 传 options 后渲染为下拉框。
   */
  type?: 'input' | 'select'
  /** type=select 时的选项 */
  options?: { label: string; value: string | number | boolean }[]
  allowClear?: boolean
}

export interface SearchBarProps {
  fields: SearchBarField[]
  /** 点击「搜索」时回调（表单值） */
  onSearch: (values: Record<string, any>) => void
  /** 点击「重置」时回调 */
  onReset?: () => void
  loading?: boolean
  extra?: ReactNode
}

/**
 * 通用搜索栏：antd inline Form + 搜索 / 重置按钮。
 *
 * @example
 * <SearchBar
 *   fields={[{ name: 'keyword', label: '设备名称' }]}
 *   onSearch={(v) => { setQuery(v); setPage(1) }}
 *   onReset={() => { setQuery({}); setPage(1) }}
 * />
 */
export default function SearchBar({ fields, onSearch, onReset, loading, extra }: SearchBarProps) {
  const [form] = Form.useForm()

  const handleReset = () => {
    form.resetFields()
    onReset?.()
  }

  return (
    <Form form={form} layout="inline" onFinish={onSearch} className="search-bar" style={{ rowGap: 8 }}>
      {fields.map((field) => (
        <Form.Item key={field.name} name={field.name} label={field.label}>
          {field.type === 'select' ? (
            <Select
              allowClear={field.allowClear !== false}
              placeholder={field.placeholder ?? `请选择${field.label}`}
              options={field.options}
              style={{ minWidth: 160 }}
            />
          ) : (
            <Input
              allowClear={field.allowClear !== false}
              placeholder={field.placeholder ?? `请输入${field.label}`}
              style={{ width: 180 }}
              onPressEnter={() => form.submit()}
            />
          )}
        </Form.Item>
      ))}
      <Form.Item>
        <Space>
          <Button type="primary" htmlType="submit" icon={<SearchOutlined />} loading={loading}>
            搜索
          </Button>
          <Button icon={<ReloadOutlined />} onClick={handleReset}>
            重置
          </Button>
          {extra}
        </Space>
      </Form.Item>
    </Form>
  )
}
