import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button, Col, Form, Input, InputNumber, Modal, Popconfirm, Row, Select, Space, Switch,
  Table, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import SearchBar from '../../components/SearchBar'
import ActionButtons from '../../components/ActionButtons'
import { ConfigController } from '../../api/generated/ConfigController'
import { useAuthStore } from '../../store/authStore'
import type { SystemConfig } from '../../api/generated/definitions/SystemConfig'

/** 参数类型选项，与后端 model.SystemConfig.DataType 取值一致（integer/boolean 为别名，落库时归一） */
const DATA_TYPE_OPTIONS = [
  { label: '字符串 string', value: 'string' },
  { label: '整数 int', value: 'int' },
  { label: '整数 integer（别名）', value: 'integer' },
  { label: '小数 float', value: 'float' },
  { label: '布尔 bool', value: 'bool' },
  { label: '布尔 boolean（别名）', value: 'boolean' },
  { label: 'JSON', value: 'json' },
  { label: '下拉 select', value: 'select' },
]

const NUMERIC_TYPES = new Set(['int', 'integer', 'float'])
const BOOL_TYPES = new Set(['bool', 'boolean'])

/** 系统参数表单值 */
interface ConfigFormValues {
  name: string
  key: string
  value?: string | number | boolean
  group_name?: string
  data_type: string
  sequence: number
  active?: boolean
  schema?: string
  description?: string
}

/** 解析 Schema 中的下拉选项，兼容 ["a","b"] 与 { options: [{ label, value }] } 两种写法 */
function parseSchemaOptions(schema?: string): { label: string; value: string }[] {
  if (!schema || !schema.trim()) return []
  try {
    const parsed: unknown = JSON.parse(schema)
    const raw = Array.isArray(parsed)
      ? parsed
      : (parsed && typeof parsed === 'object' && Array.isArray((parsed as { options?: unknown }).options)
        ? (parsed as { options: unknown[] }).options
        : [])
    return raw.flatMap((item) => {
      if (typeof item === 'string' || typeof item === 'number') {
        return [{ label: String(item), value: String(item) }]
      }
      if (item && typeof item === 'object') {
        const obj = item as { label?: unknown; value?: unknown }
        const value = obj.value ?? obj.label
        if (value === undefined || value === null) return []
        return [{ label: String(obj.label ?? value), value: String(value) }]
      }
      return []
    })
  } catch {
    return []
  }
}

/** 数据库字符串值 → 表单值（数字/布尔控件需要真实类型） */
function parseConfigFormValue(record: SystemConfig): string | number | boolean | undefined {
  const type = record.data_type ?? 'string'
  const raw = String(record.value ?? '')
  if (BOOL_TYPES.has(type)) return ['true', '1', 'yes'].includes(raw.trim().toLowerCase())
  if (NUMERIC_TYPES.has(type)) {
    if (!raw.trim()) return undefined
    const num = Number(raw)
    return Number.isFinite(num) ? num : undefined
  }
  return raw
}

/** 表单值 → 数据库字符串值 */
function serializeConfigValue(
  type: string,
  value: ConfigFormValues['value'],
): string | undefined {
  if (value === undefined || value === null) return undefined
  if (BOOL_TYPES.has(type)) return value ? 'true' : 'false'
  if (NUMERIC_TYPES.has(type)) return typeof value === 'number' ? String(value) : String(value).trim()
  return String(value)
}

/** JSON 文本校验（空值放行；非空必须是合法 JSON） */
const JSON_TEXT_VALIDATOR = {
  validator: (_: unknown, value?: string) => {
    if (!value || !value.trim()) return Promise.resolve()
    try {
      JSON.parse(value)
      return Promise.resolve()
    } catch {
      return Promise.reject(new Error('JSON 格式不正确'))
    }
  },
}

/**
 * 系统参数：非分页列表（客户端关键字过滤）+ 增删改 + 启停。
 * 参数键 key 唯一，供后端按 code 读取运行期配置。
 */
export default function ConfigList() {
  const [data, setData] = useState<SystemConfig[]>([])
  const [loading, setLoading] = useState(false)
  const [search, setSearch] = useState<Record<string, unknown>>({})
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<SystemConfig | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<ConfigFormValues>()

  // 当前表单类型：决定参数值控件的形态（数字/布尔/JSON/下拉）。
  // 用本地 state 而非 Form.useWatch：弹窗懒渲染时 useWatch 会在未挂载的 form 上告警。
  const [dataType, setDataType] = useState('string')
  const [schemaText, setSchemaText] = useState<string | undefined>(undefined)
  const isBoolType = BOOL_TYPES.has(dataType)
  const isNumericType = NUMERIC_TYPES.has(dataType)
  const selectOptions = useMemo(() => parseSchemaOptions(schemaText), [schemaText])

  const permCodes = useAuthStore((s) => s.permCodes)
  // permCodes 为空说明菜单权限尚未加载，此时不隐藏按钮，避免整页操作被锁死
  const can = (code: string) => permCodes.length === 0 || permCodes.includes(code)
  const canCreate = can('system:config:create')
  const canUpdate = can('system:config:update')
  const canDelete = can('system:config:delete')

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await ConfigController.listConfigs()
      if (res.code === 200) {
        setData(res.data ?? [])
      } else {
        message.error(res.message || '获取系统参数失败')
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const handleSearch = (values: Record<string, unknown>) => setSearch(values)
  const handleReset = () => setSearch({})

  /** 参数列表接口不分页，关键字在客户端过滤（匹配参数名称/参数键/分组） */
  const keyword = String(search.keyword ?? '').trim().toLowerCase()
  const filteredData = useMemo(() => {
    if (!keyword) return data
    return data.filter((item) => [item.name, item.key, item.group_name]
      .some((field) => String(field ?? '').toLowerCase().includes(keyword)))
  }, [data, keyword])

  /** 按排序值升序展示，便于管理员调整顺序后立刻看到效果 */
  const sortedData = useMemo(
    () => [...filteredData].sort((a, b) => (a.sequence ?? 0) - (b.sequence ?? 0) || (a.id ?? 0) - (b.id ?? 0)),
    [filteredData],
  )

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    setDataType('string')
    setSchemaText(undefined)
    form.setFieldsValue({ data_type: 'string', sequence: 10, active: true })
    setModalOpen(true)
  }

  const openEdit = (record: SystemConfig) => {
    setEditing(record)
    setDataType(record.data_type ?? 'string')
    setSchemaText(record.schema)
    form.setFieldsValue({
      name: record.name ?? '',
      key: record.key ?? '',
      // 数字/布尔控件需要真实类型，不能直接塞字符串
      value: parseConfigFormValue(record),
      group_name: record.group_name,
      data_type: record.data_type ?? 'string',
      sequence: record.sequence ?? 10,
      active: record.active !== false,
      schema: record.schema,
      description: record.description,
    })
    setModalOpen(true)
  }

  const handleSave = async () => {
    const values = await form.validateFields()
    const serializedValue = serializeConfigValue(values.data_type, values.value)
    // 空 schema 不提交：后端仅在非空时更新，避免误清空既有配置
    const schema = values.schema?.trim()
    const base = {
      name: values.name,
      key: values.key,
      data_type: values.data_type,
      group_name: values.group_name,
      sequence: values.sequence,
      active: values.active,
      description: values.description,
      ...(serializedValue !== undefined ? { value: serializedValue } : {}),
      ...(schema ? { schema } : {}),
    }
    setSaving(true)
    try {
      if (editing) {
        const res = await ConfigController.updateConfig(String(editing.id), base)
        if (res.code === 200) {
          message.success('更新成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '更新失败')
        }
      } else {
        const res = await ConfigController.createConfig(base)
        if (res.code === 200 || res.code === 201) {
          message.success('创建成功')
          setModalOpen(false)
          fetchData()
        } else {
          message.error(res.message || '创建失败')
        }
      }
    } catch {
      message.error('保存失败，请检查网络后重试')
    } finally {
      setSaving(false)
    }
  }

  /** 启停走独立端点 */
  const handleToggleActive = async (record: SystemConfig, checked: boolean) => {
    try {
      const res = checked
        ? await ConfigController.enableConfig(String(record.id))
        : await ConfigController.disableConfig(String(record.id))
      if (res.code === 200) {
        message.success(checked ? '已启用' : '已停用')
      } else {
        message.error(res.message || '操作失败')
      }
      fetchData()
    } catch {
      message.error('操作失败')
    }
  }

  const handleDelete = async (record: SystemConfig) => {
    try {
      const res = await ConfigController.deleteConfig(String(record.id))
      if (res.code === 200) {
        message.success('删除成功')
        fetchData()
      } else {
        message.error(res.message || '删除失败')
      }
    } catch {
      message.error('删除失败')
    }
  }

  const columns: ColumnsType<SystemConfig> = [
    { title: '参数名称', dataIndex: 'name', key: 'name', width: 200 },
    {
      title: '参数键', dataIndex: 'key', key: 'key', width: 200,
      render: (value: string) => <Tag color="blue">{value}</Tag>,
    },
    { title: '参数值', dataIndex: 'value', key: 'value', width: 220, ellipsis: true },
    {
      title: '类型', dataIndex: 'data_type', key: 'data_type', width: 110,
      render: (value: string) => DATA_TYPE_OPTIONS.find((item) => item.value === value)?.label ?? value ?? '-',
    },
    { title: '分组', dataIndex: 'group_name', key: 'group_name', width: 120 },
    { title: '排序', dataIndex: 'sequence', key: 'sequence', width: 80 },
    {
      title: '状态', key: 'active', width: 100,
      render: (_, record) => (
        <Switch
          checked={record.active !== false}
          checkedChildren="启用"
          unCheckedChildren="停用"
          disabled={!canUpdate}
          onChange={(checked) => handleToggleActive(record, checked)}
        />
      ),
    },
    {
      title: '操作', key: 'action', width: 160, fixed: 'right' as const,
      render: (_, record) => (
        <ActionButtons>
          {canUpdate && (
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button>
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
      title="系统参数"
      subTitle="维护运行期可配置参数；参数键全局唯一"
      extra={canCreate ? (
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建参数</Button>
      ) : undefined}
    >
      <SearchBar
        fields={[{ name: 'keyword', label: '参数名称/参数键', placeholder: '请输入参数名称或参数键' }]}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={loading}
      />

      <Table
        rowKey="id"
        size="small"
        scroll={{ x: 'max-content' }}
        columns={columns}
        dataSource={sortedData}
        loading={loading}
        pagination={false}
      />

      <Modal
        className="e2e-config-modal"
        title={editing ? '编辑参数' : '新建参数'}
        open={modalOpen}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
        confirmLoading={saving}
        width={720}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="参数名称" rules={[{ required: true, message: '请输入参数名称' }]}>
                <Input placeholder="如：设备心跳超时(秒)" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="key" label="参数键" rules={[{ required: true, message: '请输入参数键' }]}>
                <Input placeholder="如：device.heartbeat.timeout" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="data_type" label="类型" rules={[{ required: true, message: '请选择类型' }]}>
                <Select
                  options={DATA_TYPE_OPTIONS}
                  onChange={(value: string) => {
                    setDataType(value)
                    // 类型切换后旧值可能不再适用（如数字 → 布尔），清空由用户重填
                    form.setFieldsValue({ value: undefined })
                  }}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="group_name" label="分组">
                <Input placeholder="如：device" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="sequence" label="排序" rules={[{ required: true, message: '请输入排序' }]}>
                <InputNumber min={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="active" label="状态" valuePropName="checked">
                <Switch checkedChildren="启用" unCheckedChildren="停用" />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item
                name="value"
                label="参数值"
                valuePropName={isBoolType ? 'checked' : 'value'}
                rules={dataType === 'json' ? [JSON_TEXT_VALIDATOR] : undefined}
                extra={
                  dataType === 'select' && selectOptions.length === 0
                    ? '请先在下方「配置 Schema」中提供 options'
                    : undefined
                }
              >
                {isBoolType ? (
                  <Switch checkedChildren="true" unCheckedChildren="false" />
                ) : isNumericType ? (
                  <InputNumber
                    style={{ width: '100%' }}
                    precision={dataType === 'float' ? 2 : 0}
                    placeholder="请输入数字"
                  />
                ) : dataType === 'json' ? (
                  <Input.TextArea rows={4} placeholder='{"key": "value"}' />
                ) : dataType === 'select' && selectOptions.length > 0 ? (
                  <Select allowClear options={selectOptions} placeholder="请选择参数值" />
                ) : (
                  <Input.TextArea rows={2} placeholder="请输入参数值" />
                )}
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item
                name="schema"
                label="配置 Schema"
                extra='可选 JSON；select 类型可写 {"options":[{"label":"开启","value":"on"}]} 或 {"options":["a","b"]}'
                rules={[JSON_TEXT_VALIDATOR]}
              >
                <Input.TextArea
                  rows={3}
                  placeholder='{"options": ["a", "b"]}'
                  onChange={(event) => setSchemaText(event.target.value)}
                />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="description" label="说明">
                <Input.TextArea rows={2} placeholder="请输入说明" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>
    </PageContainer>
  )
}
