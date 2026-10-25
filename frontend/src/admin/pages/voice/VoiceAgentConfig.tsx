import React, { useCallback, useEffect, useRef, useState } from 'react'
import {
  Alert, Button, Card, Col, Descriptions, Empty, Form, Input, InputNumber, Modal, Popconfirm, Row,
  Select, Space, Switch, Table, Tabs, Tag, Typography, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  ApiOutlined, CloudUploadOutlined, DeleteOutlined, DisconnectOutlined, DownloadOutlined,
  EditOutlined, EyeOutlined, LinkOutlined, ReloadOutlined, SaveOutlined,
} from '@ant-design/icons'
import axios from 'axios'
import dayjs from 'dayjs'
import PageContainer from '../../components/PageContainer'
import ActionButtons from '../../components/ActionButtons'
import { VoiceAgentController } from '../../api/generated/VoiceAgentController'
import type { VoiceProviderItem } from '../../api/generated/definitions/VoiceProviderItem'
import type { VoiceAgentConfigResp } from '../../api/generated/definitions/VoiceAgentConfigResp'
import type { UpdateVoiceAgentReq } from '../../api/generated/definitions/UpdateVoiceAgentReq'
import type { TestVoiceAgentReq } from '../../api/generated/definitions/TestVoiceAgentReq'
import type { VoiceMcpToolItem } from '../../api/generated/definitions/VoiceMcpToolItem'
import type { VoiceMcpLogItem } from '../../api/generated/definitions/VoiceMcpLogItem'
import type { VoiceMemoryItem } from '../../api/generated/definitions/VoiceMemoryItem'
import type { UpdateVoiceMcpToolReq } from '../../api/generated/definitions/UpdateVoiceMcpToolReq'
import { useAuthStore } from '../../store/authStore'

type Category = 'stt' | 'llm' | 'tts' | 'vad'
type TabKey = 'basic' | 'websocket' | 'mcp' | 'memory' | 'observe'

interface ParamFieldDef {
  key: string
  label: string
  type: 'text' | 'password' | 'number' | 'textarea'
  required?: boolean
  secret?: boolean
}

/** 扩展参数在表单里的字段名前缀，避免与固定字段重名。 */
const EXTRA_FIELD_PREFIX = 'extra_'

/** 表单字段名：extra_<category>_<schema key>。 */
function extraFieldName(category: Category, key: string): string {
  return `${EXTRA_FIELD_PREFIX}${category}_${key}`
}

interface VoiceAgentFormValues {
  stt_provider?: string
  stt_model?: string
  stt_endpoint?: string
  stt_api_key?: string
  stt_language?: string

  llm_provider?: string
  llm_model?: string
  llm_endpoint?: string
  llm_api_key?: string
  llm_system_prompt?: string
  llm_temperature?: number
  llm_max_tokens?: number

  tts_provider?: string
  tts_voice?: string
  tts_endpoint?: string
  tts_api_key?: string
  tts_speed?: number

  vad_provider?: string
  vad_params?: string

  barge_in?: boolean
  session_idle_timeout?: number
  max_session_sec?: number
  pipecat_endpoint?: string
  pipecat_token?: string

  /** v0.6.0 WebSocket */
  ws_enabled?: boolean
  ws_path?: string
  fast_ack_text?: string
  device_wake_word?: string

  /** v0.6.0 服务端 MCP */
  mcp_enabled?: boolean
  mcp_server_url?: string
  internal_mcp_token?: string

  /** v0.6.0 长期记忆 */
  memory_enabled?: boolean
  memory_top_k?: number
  memory_ttl_days?: number
  embedding_endpoint?: string
  embedding_model?: string
  embedding_api_key?: string

  /** 动态表单字段（vad_xxx）与潜在扩展参数 */
  [key: string]: string | number | boolean | undefined
}

/** param_schema 的键 → 接口固定字段（VAD 走 vad_params JSON，不在此表）。 */
const PARAM_FIELD_MAP: Record<Category, Record<string, string>> = {
  stt: {
    model: 'stt_model',
    endpoint: 'stt_endpoint',
    api_key: 'stt_api_key',
    language: 'stt_language',
  },
  llm: {
    model: 'llm_model',
    endpoint: 'llm_endpoint',
    api_key: 'llm_api_key',
    system_prompt: 'llm_system_prompt',
    temperature: 'llm_temperature',
    max_tokens: 'llm_max_tokens',
  },
  tts: {
    voice: 'tts_voice',
    endpoint: 'tts_endpoint',
    api_key: 'tts_api_key',
    speed: 'tts_speed',
  },
  vad: {},
}

/** provider 未配置 param_schema 时的兜底字段。 */
const FALLBACK_FIELDS: Record<Category, ParamFieldDef[]> = {
  stt: [
    { key: 'model', label: '模型', type: 'text', required: true },
    { key: 'language', label: '识别语言', type: 'text' },
    { key: 'endpoint', label: '服务地址', type: 'text' },
    { key: 'api_key', label: 'API Key', type: 'password', secret: true },
  ],
  llm: [
    { key: 'model', label: '模型', type: 'text', required: true },
    { key: 'endpoint', label: '服务地址', type: 'text' },
    { key: 'api_key', label: 'API Key', type: 'password', secret: true },
    { key: 'system_prompt', label: '系统提示词', type: 'textarea' },
    { key: 'temperature', label: '温度', type: 'number' },
    { key: 'max_tokens', label: '最大输出长度', type: 'number' },
  ],
  tts: [
    { key: 'voice', label: '音色', type: 'text', required: true },
    { key: 'endpoint', label: '服务地址', type: 'text' },
    { key: 'api_key', label: 'API Key', type: 'password', secret: true },
    { key: 'speed', label: '语速', type: 'number' },
  ],
  vad: [],
}

/** 记忆 owner 枚举（设计 §6.2）。 */
const MEMORY_OWNER_OPTIONS = [
  { label: '用户', value: 'user' },
  { label: '设备', value: 'device' },
  { label: '部门', value: 'dept' },
  { label: '全局', value: 'global' },
]

/** 请求失败提示：优先展示后端返回的 message。 */
function apiErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError<{ message?: string }>(err)) {
    return err.response?.data?.message || err.message || fallback
  }
  return err instanceof Error ? err.message : fallback
}

/** 解析 provider 的 param_schema（容错：非法 JSON 返回空数组）。 */
function parseParamSchema(raw?: string): ParamFieldDef[] {
  if (!raw) return []
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return []
  }
  if (!Array.isArray(parsed)) return []

  return parsed.flatMap((item) => {
    if (typeof item !== 'object' || item === null) return []
    const record = item as Record<string, unknown>
    if (typeof record.key !== 'string' || !record.key) return []
    const type = record.type === 'password' || record.type === 'number' || record.type === 'textarea'
      ? record.type
      : 'text'
    return [{
      key: record.key,
      label: typeof record.label === 'string' && record.label ? record.label : record.key,
      type,
      required: record.required === true,
      secret: record.secret === true,
    }]
  })
}

/** 解析 default_params / vad_params（容错：非法 JSON 返回空对象）。 */
function parseJsonObject(raw?: string): Record<string, string | number | boolean> {
  if (!raw) return {}
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return {}
  }
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) return {}

  const out: Record<string, string | number | boolean> = {}
  for (const [key, value] of Object.entries(parsed)) {
    if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
      out[key] = value
    }
  }
  return out
}

function renderParamInput(def: ParamFieldDef) {
  if (def.type === 'password' || def.secret) {
    return (
      <Input.Password
        placeholder="留空或含 **** 表示不修改"
        maxLength={500}
        autoComplete="new-password"
      />
    )
  }
  if (def.type === 'number') {
    return <InputNumber style={{ width: '100%' }} />
  }
  if (def.type === 'textarea') {
    return <Input.TextArea rows={3} maxLength={2000} showCount placeholder={`请输入${def.label}`} />
  }
  return <Input maxLength={500} placeholder={`请输入${def.label}`} />
}

/** 只写不读的密钥：空值或脱敏串不提交，避免覆盖后端已保存的真密钥。 */
function secretOrUndefined(value?: string): string | undefined {
  if (!value) return undefined
  if (value.includes('****')) return undefined
  return value
}

/** 时间格式化。 */
function fmtDT(value?: string): string {
  if (!value) return '-'
  const d = dayjs(value)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : value
}

/** MCP 风险等级标签。 */
function riskTag(value?: string) {
  if (value === 'low') return <Tag color="green">低风险</Tag>
  if (value === 'medium') return <Tag color="orange">中风险</Tag>
  if (value === 'high') return <Tag color="red">高风险</Tag>
  return value ? <Tag>{value}</Tag> : <Tag>未知</Tag>
}

/** MCP 调用结果标签（ok / error / denied）。 */
function mcpStatusTag(value?: string) {
  if (value === 'ok') return <Tag color="green">ok</Tag>
  if (value === 'error') return <Tag color="red">error</Tag>
  if (value === 'denied') return <Tag color="orange">denied</Tag>
  return value ? <Tag>{value}</Tag> : '-'
}

/** Embedding 状态标签。 */
function embeddingTag(value?: string) {
  if (!value) return <Tag>未索引</Tag>
  if (value === 'ok' || value === 'done' || value === 'success') return <Tag color="green">{value}</Tag>
  if (value === 'failed' || value === 'error') return <Tag color="red">{value}</Tag>
  if (value === 'pending' || value === 'processing') return <Tag color="blue">{value}</Tag>
  return <Tag>{value}</Tag>
}

function SectionTitle({ title, action }: { title: string; action?: React.ReactNode }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', margin: '0 0 16px' }}>
      <Typography.Title level={5} style={{ margin: 0 }}>{title}</Typography.Title>
      {action}
    </div>
  )
}

/**
 * 浏览器端 WebSocket 地址：VITE_API_BASE 有配置时用其 host，否则同源。
 * `wsUrl` 优先使用 ticket 接口返回的 ws_url；手动粘贴 ticket 时回退到设计文档路径。
 */
function buildVoiceWsUrl(ticket: string, wsUrl?: string): string {
  // 后端 ws_url 是绝对地址（ws://host/web/voice/ws），优先直接使用。
  if (wsUrl && /^wss?:\/\//i.test(wsUrl)) {
    const absolute = new URL(wsUrl)
    absolute.searchParams.set('ticket', ticket)
    return absolute.toString()
  }
  const configured = import.meta.env.VITE_API_BASE
  // 开发态 Vite 代理默认不转发 WebSocket，直连后端端口（无 CORS 限制）。
  const base = configured
    ? new URL(configured)
    : import.meta.env.DEV
      ? new URL(`http://localhost:${import.meta.env.VITE_BACKEND_PORT || '8080'}`)
      : new URL(window.location.origin)
  const protocol = base.protocol === 'https:' ? 'wss:' : 'ws:'
  const path = wsUrl && wsUrl.startsWith('/') ? wsUrl : '/web/voice/ws'
  return `${protocol}//${base.host}${path}?ticket=${encodeURIComponent(ticket)}`
}

/**
 * 下载记忆 CSV。
 *
 * 生成器对 `text/csv` 文件流只声明返回 any，axios 会按 JSON 解析导致失败。
 * 复用 Controller 暴露的路径常量（不硬编码接口），手动携带 JWT 取 Blob，
 * 与 `GbCloudRecordDownload.downloadBinary` 的做法一致。
 */
async function downloadVoiceMemoriesCsv(query: {
  keyword?: string
  owner_type?: string
}): Promise<void> {
  const params = new URLSearchParams()
  if (query.owner_type) params.set('owner_type', query.owner_type)
  if (query.keyword) params.set('keyword', query.keyword)
  const search = params.toString()
  const base = import.meta.env.VITE_API_BASE || ''
  const token = localStorage.getItem('token')
  const response = await fetch(
    `${base}${VoiceAgentController.exportVoiceMemoriesPath}${search ? `?${search}` : ''}`,
    {
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
      cache: 'no-store',
    },
  )

  if (!response.ok) {
    let reason = `导出失败（HTTP ${response.status}）`
    try {
      const body = (await response.json()) as { message?: string }
      if (body?.message) reason = body.message
    } catch {
      // 非 JSON 错误响应，沿用状态码提示。
    }
    throw new Error(reason)
  }

  const blob = await response.blob()
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `voice-memories-${dayjs().format('YYYYMMDD-HHmmss')}.csv`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

/** 客户端随机 ID（hello.client.client_id）。 */
function randomClientId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `web-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

/** 首帧 hello（设计 §3.1）。Web 端固定 PCM16。 */
function buildClientHello() {
  return {
    type: 'hello',
    version: 1,
    transport: 'websocket',
    features: { mcp: true, aec: false, wake_word: 'device' },
    audio_params: { format: 'pcm16', sample_rate: 16000, channels: 1, frame_duration: 20 },
    client: { kind: 'web', client_id: randomClientId(), device_id: '' },
  }
}

interface VoiceServerHello {
  session_id?: string
  transport?: string
  audio_params?: Record<string, unknown>
  capabilities?: Record<string, unknown>
}

const WS_STATE_LABEL: Record<string, { color: string; text: string }> = {
  idle: { color: 'default', text: '未连接' },
  connecting: { color: 'processing', text: '连接中' },
  open: { color: 'success', text: '已连接' },
  closed: { color: 'default', text: '已断开' },
  error: { color: 'error', text: '连接失败' },
}

/**
 * 语音 Agent 配置（v0.6.0）。
 *
 * Tab 结构：基础（STT/LLM/TTS/VAD）/ WebSocket / MCP / 记忆 / 观测。
 * 配置仍存于 ac_thirdparty_config（Category='voice_agent'），密钥只写不读；
 * MCP 工具、调用日志与记忆列表走 VoiceAgentController 的 v0.6.0 新方法。
 */
const VoiceAgentConfig: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canView = permCodes.includes('voice:agent:view')
  const canUpdate = permCodes.includes('voice:agent:update')
  const canTest = permCodes.includes('voice:agent:test')
  const canApply = permCodes.includes('voice:agent:apply')

  const [activeKey, setActiveKey] = useState<TabKey>('basic')
  const [providers, setProviders] = useState<VoiceProviderItem[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [applying, setApplying] = useState(false)
  const [testingCategory, setTestingCategory] = useState<Category | undefined>()
  const [form] = Form.useForm<VoiceAgentFormValues>()

  const sttProvider = Form.useWatch('stt_provider', form)
  const llmProvider = Form.useWatch('llm_provider', form)
  const ttsProvider = Form.useWatch('tts_provider', form)
  const vadProvider = Form.useWatch('vad_provider', form)
  const pipecatEndpoint = Form.useWatch('pipecat_endpoint', form)
  const wsEnabled = Form.useWatch('ws_enabled', form)

  // ── WebSocket 连接测试 ──
  const [wsTicket, setWsTicket] = useState('')
  const [ticketLoading, setTicketLoading] = useState(false)
  const [wsState, setWsState] = useState<'idle' | 'connecting' | 'open' | 'closed' | 'error'>('idle')
  const [serverHello, setServerHello] = useState<VoiceServerHello | null>(null)
  const [wsLines, setWsLines] = useState<string[]>([])
  const wsRef = useRef<WebSocket | null>(null)

  // ── MCP ──
  const [mcpTools, setMcpTools] = useState<VoiceMcpToolItem[]>([])
  const [mcpToolTotal, setMcpToolTotal] = useState(0)
  const [mcpToolPage, setMcpToolPage] = useState(1)
  const [mcpToolPageSize, setMcpToolPageSize] = useState(20)
  const [mcpToolsLoading, setMcpToolsLoading] = useState(false)
  const [mcpToolsError, setMcpToolsError] = useState('')
  const [mcpToolEditing, setMcpToolEditing] = useState<VoiceMcpToolItem | null>(null)
  const [mcpToolSaving, setMcpToolSaving] = useState(false)
  const [mcpToolForm] = Form.useForm<UpdateVoiceMcpToolReq>()

  const [mcpLogs, setMcpLogs] = useState<VoiceMcpLogItem[]>([])
  const [mcpLogTotal, setMcpLogTotal] = useState(0)
  const [mcpLogPage, setMcpLogPage] = useState(1)
  const [mcpLogPageSize, setMcpLogPageSize] = useState(20)
  const [mcpLogLoading, setMcpLogLoading] = useState(false)
  const [mcpLogError, setMcpLogError] = useState('')

  // ── 记忆 ──
  const [memories, setMemories] = useState<VoiceMemoryItem[]>([])
  const [memoryTotal, setMemoryTotal] = useState(0)
  const [memoryPage, setMemoryPage] = useState(1)
  const [memoryPageSize, setMemoryPageSize] = useState(20)
  const [memoryKeyword, setMemoryKeyword] = useState('')
  const [memoryOwnerType, setMemoryOwnerType] = useState<string | undefined>()
  const [memoryQuery, setMemoryQuery] = useState<{ keyword?: string; owner_type?: string }>({})
  const [memoryLoading, setMemoryLoading] = useState(false)
  const [memoryError, setMemoryError] = useState('')
  const [memoryDetail, setMemoryDetail] = useState<VoiceMemoryItem | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [configRes, providerRes] = await Promise.all([
        VoiceAgentController.getVoiceAgentConfig(),
        VoiceAgentController.listVoiceAgentProviders(),
      ])
      const list = providerRes.data ?? []
      setProviders(list)

      const config: VoiceAgentConfigResp | undefined = configRes.data
      if (config) {
        form.setFieldsValue({
          stt_provider: config.stt_provider ?? '',
          stt_model: config.stt_model ?? '',
          stt_endpoint: config.stt_endpoint ?? '',
          stt_api_key: config.stt_api_key ?? '',
          stt_language: config.stt_language ?? '',

          llm_provider: config.llm_provider ?? '',
          llm_model: config.llm_model ?? '',
          llm_endpoint: config.llm_endpoint ?? '',
          llm_api_key: config.llm_api_key ?? '',
          llm_system_prompt: config.llm_system_prompt ?? '',
          llm_temperature: config.llm_temperature,
          llm_max_tokens: config.llm_max_tokens,

          tts_provider: config.tts_provider ?? '',
          tts_voice: config.tts_voice ?? '',
          tts_endpoint: config.tts_endpoint ?? '',
          tts_api_key: config.tts_api_key ?? '',
          tts_speed: config.tts_speed,

          vad_provider: config.vad_provider ?? '',
          vad_params: config.vad_params ?? '',

          barge_in: config.barge_in ?? false,
          session_idle_timeout: config.session_idle_timeout,
          max_session_sec: config.max_session_sec,
          pipecat_endpoint: config.pipecat_endpoint ?? '',
          pipecat_token: config.pipecat_token ?? '',

          // v0.6.0 新增键：后端尚未返回时使用设计文档的默认值。
          ws_enabled: config.ws_enabled ?? true,
          ws_path: config.ws_path ?? '/ws/voice',
          fast_ack_text: config.fast_ack_text ?? '我在',
          device_wake_word: config.device_wake_word ?? '',

          mcp_enabled: config.mcp_enabled ?? true,
          mcp_server_url: config.mcp_server_url ?? '',
          internal_mcp_token: config.internal_mcp_token ?? '',

          memory_enabled: config.memory_enabled ?? true,
          memory_top_k: config.memory_top_k ?? 6,
          memory_ttl_days: config.memory_ttl_days ?? 180,
          embedding_endpoint: config.embedding_endpoint ?? '',
          embedding_model: config.embedding_model ?? 'BGE-M3',
          embedding_api_key: config.embedding_api_key ?? '',
        })

        // VAD 参数在接口里是 JSON 字符串，按当前 provider 的 schema 拆成扁平字段。
        const vadCode = config.vad_provider ?? 'silero'
        const vadProviderItem = list.find((item) => item.category === 'vad' && item.code === vadCode)
        const vadSchema = parseParamSchema(vadProviderItem?.param_schema)
        const vadValues = parseJsonObject(config.vad_params)
        for (const def of vadSchema) {
          if (def.key in vadValues) {
            form.setFieldValue(`vad_${def.key}`, vadValues[def.key])
          }
        }

        // 扩展参数回显：只回填当前 provider schema 中「没有固定键」的项。
        const extras = config.extra_params ?? {}
        for (const category of ['stt', 'llm', 'tts'] as const) {
          const code = config[`${category}_provider`]
          const providerItem = list.find((item) => item.category === category && item.code === code)
          const schema = parseParamSchema(providerItem?.param_schema)
          for (const def of schema) {
            if (PARAM_FIELD_MAP[category][def.key.toLowerCase()]) continue
            if (!(def.key in extras)) continue
            const value = def.type === 'number' ? Number(extras[def.key]) : extras[def.key]
            form.setFieldValue(extraFieldName(category, def.key), value)
          }
        }
      }
    } catch (err) {
      message.error(apiErrorMessage(err, '获取语音 Agent 配置失败'))
    } finally {
      setLoading(false)
    }
  }, [form])

  useEffect(() => {
    if (canView) void load()
  }, [canView, load])

  // 页面卸载时释放 WebSocket。
  useEffect(() => () => {
    wsRef.current?.close()
    wsRef.current = null
  }, [])

  // ── MCP / 记忆的数据加载 ──

  const loadMcpTools = useCallback(async (page = mcpToolPage, pageSize = mcpToolPageSize) => {
    setMcpToolsLoading(true)
    setMcpToolsError('')
    try {
      const res = await VoiceAgentController.pageVoiceMcpTools(page, pageSize)
      if (res.code !== 200) {
        setMcpToolsError(res.message || '加载 MCP 工具失败')
      } else {
        setMcpTools(res.data?.items ?? [])
        setMcpToolTotal(res.data?.total ?? 0)
      }
    } catch (err) {
      setMcpToolsError(apiErrorMessage(err, '加载 MCP 工具失败'))
    } finally {
      setMcpToolsLoading(false)
    }
  }, [mcpToolPage, mcpToolPageSize])

  const loadMcpLogs = useCallback(async (page = mcpLogPage, pageSize = mcpLogPageSize) => {
    setMcpLogLoading(true)
    setMcpLogError('')
    try {
      const res = await VoiceAgentController.pageVoiceMcpLogs(page, pageSize)
      if (res.code !== 200) {
        setMcpLogError(res.message || '加载 MCP 调用日志失败')
      } else {
        setMcpLogs(res.data?.items ?? [])
        setMcpLogTotal(res.data?.total ?? 0)
      }
    } catch (err) {
      setMcpLogError(apiErrorMessage(err, '加载 MCP 调用日志失败'))
    } finally {
      setMcpLogLoading(false)
    }
  }, [mcpLogPage, mcpLogPageSize])

  const loadMemories = useCallback(async (
    page = memoryPage,
    pageSize = memoryPageSize,
    query = memoryQuery,
  ) => {
    setMemoryLoading(true)
    setMemoryError('')
    try {
      const res = await VoiceAgentController.pageVoiceMemories(
        page,
        pageSize,
        query.owner_type,
        undefined,
        undefined,
        query.keyword,
      )
      if (res.code !== 200) {
        setMemoryError(res.message || '加载记忆列表失败')
      } else {
        setMemories(res.data?.items ?? [])
        setMemoryTotal(res.data?.total ?? 0)
      }
    } catch (err) {
      setMemoryError(apiErrorMessage(err, '加载记忆列表失败'))
    } finally {
      setMemoryLoading(false)
    }
  }, [memoryPage, memoryPageSize, memoryQuery])

  const handleTabChange = (key: string) => {
    const tab = key as TabKey
    setActiveKey(tab)
    if (tab === 'mcp') {
      void loadMcpTools()
      void loadMcpLogs(1, mcpLogPageSize)
    } else if (tab === 'memory') {
      void loadMemories(1, memoryPageSize)
    }
  }

  // ── 基础配置的 provider 动态表单 ──

  const findProvider = (category: Category, code?: string): VoiceProviderItem | undefined => {
    if (!code) return undefined
    return providers.find((item) => item.category === category && item.code === code)
  }

  const providerOptions = (category: Category) => providers
    .filter((item) => item.category === category && item.code)
    .map((item) => ({ label: item.name || item.code || '', value: item.code || '' }))

  /** 切换 provider 时把 default_params 填入对应字段。 */
  const applyProviderDefaults = (category: Category, code?: string) => {
    const provider = findProvider(category, code)
    if (!provider) return
    const defaults = parseJsonObject(provider.default_params)
    const schema = parseParamSchema(provider.param_schema)

    if (category === 'vad') {
      for (const def of schema) {
        if (def.key in defaults) {
          form.setFieldValue(`vad_${def.key}`, defaults[def.key])
        }
      }
      return
    }

    const fieldMap = PARAM_FIELD_MAP[category]
    for (const def of schema) {
      const target = fieldMap[def.key.toLowerCase()]
      if (target) {
        if (def.key in defaults) {
          form.setFieldValue(target, defaults[def.key])
        }
        continue
      }
      form.setFieldValue(
        extraFieldName(category, def.key),
        def.key in defaults ? defaults[def.key] : undefined,
      )
    }
  }

  /** 按 provider 的 param_schema 动态渲染参数表单。 */
  const renderCategoryFields = (category: Exclude<Category, 'vad'>, providerCode?: string): React.ReactNode => {
    const schema = parseParamSchema(findProvider(category, providerCode)?.param_schema)
    const defs = schema.length > 0 ? schema : FALLBACK_FIELDS[category]
    const fieldMap = PARAM_FIELD_MAP[category]
    const nodes: React.ReactNode[] = []

    for (const def of defs) {
      const target = fieldMap[def.key.toLowerCase()]
      const name = target ?? extraFieldName(category, def.key)
      nodes.push(
        <Col xs={24} md={def.type === 'textarea' ? 24 : 12} key={`${category}-${def.key}`}>
          <Form.Item
            name={name}
            label={def.label}
            rules={def.required ? [{ required: true, message: `请输入${def.label}` }] : undefined}
            extra={target ? undefined : 'provider 扩展参数'}
          >
            {renderParamInput(def)}
          </Form.Item>
        </Col>,
      )
    }

    return <>{nodes}</>
  }

  /** VAD 参数：有 schema 时渲染扁平字段，否则退化为 JSON 文本域。 */
  const renderVadFields = (providerCode?: string): React.ReactNode => {
    const schema = parseParamSchema(findProvider('vad', providerCode)?.param_schema)
    if (schema.length === 0) {
      return (
        <Col span={24}>
          <Form.Item name="vad_params" label="VAD 参数（JSON）">
            <Input.TextArea
              rows={3}
              maxLength={2000}
              placeholder='例如 {"threshold":0.5,"min_silence_ms":300}'
            />
          </Form.Item>
        </Col>
      )
    }
    return (
      <>
        {schema.map((def) => (
          <Col xs={24} md={12} key={`vad-${def.key}`}>
            <Form.Item
              name={`vad_${def.key}`}
              label={def.label}
              rules={def.required ? [{ required: true, message: `请输入${def.label}` }] : undefined}
            >
              {renderParamInput(def)}
            </Form.Item>
          </Col>
        ))}
      </>
    )
  }

  // ── 保存 / 测试 / 应用 ──

  const handleSave = async () => {
    try {
      await form.validateFields()
    } catch {
      message.warning('请先补全「基础」页的必填项')
      return
    }
    const values = form.getFieldsValue(true) as VoiceAgentFormValues

    // VAD 参数：有 schema 时把扁平字段序列化回 JSON，否则沿用原文本。
    const vadSchema = parseParamSchema(findProvider('vad', values.vad_provider)?.param_schema)
    let vadParams: string | undefined
    if (vadSchema.length > 0) {
      const params: Record<string, string | number | boolean> = {}
      for (const def of vadSchema) {
        const value = values[`vad_${def.key}`]
        if (value !== undefined && value !== '') {
          params[def.key] = value
        }
      }
      vadParams = Object.keys(params).length > 0 ? JSON.stringify(params) : undefined
    } else {
      vadParams = values.vad_params
    }

    // 扩展参数：只收集当前 provider schema 中「没有固定键」且有值的字段。
    const extraParams: Record<string, string> = {}
    for (const category of ['stt', 'llm', 'tts'] as const) {
      const providerCode = values[`${category}_provider`]
      const providerItem = providers.find(
        (item) => item.category === category && item.code === providerCode,
      )
      const schema = parseParamSchema(providerItem?.param_schema)
      for (const def of schema) {
        if (PARAM_FIELD_MAP[category][def.key.toLowerCase()]) continue
        const raw = values[extraFieldName(category, def.key)]
        if (raw === undefined || raw === null || raw === '') continue
        extraParams[def.key] = String(raw)
      }
    }

    const payload: UpdateVoiceAgentReq = {
      stt_provider: values.stt_provider,
      stt_model: values.stt_model,
      stt_endpoint: values.stt_endpoint,
      stt_api_key: values.stt_api_key,
      stt_language: values.stt_language,

      llm_provider: values.llm_provider,
      llm_model: values.llm_model,
      llm_endpoint: values.llm_endpoint,
      llm_api_key: values.llm_api_key,
      llm_system_prompt: values.llm_system_prompt,
      llm_temperature: typeof values.llm_temperature === 'number' ? values.llm_temperature : undefined,
      llm_max_tokens: typeof values.llm_max_tokens === 'number' ? values.llm_max_tokens : undefined,

      tts_provider: values.tts_provider,
      tts_voice: values.tts_voice,
      tts_endpoint: values.tts_endpoint,
      tts_api_key: values.tts_api_key,
      tts_speed: typeof values.tts_speed === 'number' ? values.tts_speed : undefined,

      vad_provider: values.vad_provider,
      vad_params: vadParams,

      barge_in: values.barge_in,
      session_idle_timeout: typeof values.session_idle_timeout === 'number' ? values.session_idle_timeout : undefined,
      max_session_sec: typeof values.max_session_sec === 'number' ? values.max_session_sec : undefined,

      pipecat_endpoint: values.pipecat_endpoint,
      pipecat_token: values.pipecat_token,

      // v0.6.0 新增配置键（后端重新生成/实现后生效）
      ws_enabled: values.ws_enabled,
      ws_path: values.ws_path,
      fast_ack_text: values.fast_ack_text,
      device_wake_word: values.device_wake_word,

      mcp_enabled: values.mcp_enabled,
      mcp_server_url: values.mcp_server_url,
      internal_mcp_token: secretOrUndefined(values.internal_mcp_token),

      memory_enabled: values.memory_enabled,
      memory_top_k: typeof values.memory_top_k === 'number' ? values.memory_top_k : undefined,
      memory_ttl_days: typeof values.memory_ttl_days === 'number' ? values.memory_ttl_days : undefined,
      embedding_endpoint: values.embedding_endpoint,
      embedding_model: values.embedding_model,
      embedding_api_key: secretOrUndefined(values.embedding_api_key),

      extra_params: Object.keys(extraParams).length > 0 ? extraParams : undefined,
    }

    setSaving(true)
    try {
      await VoiceAgentController.updateVoiceAgentConfig(payload)
      message.success('保存成功，配置在下一个会话生效')
      void load()
    } catch (err) {
      message.error(apiErrorMessage(err, '保存语音 Agent 配置失败'))
    } finally {
      setSaving(false)
    }
  }

  const handleTest = async (category: Category) => {
    const provider = form.getFieldValue(`${category}_provider`)
    const endpoint = form.getFieldValue(`${category}_endpoint`)
    const apiKey = form.getFieldValue(`${category}_api_key`)

    const payload: TestVoiceAgentReq = {
      category,
      provider: typeof provider === 'string' ? provider : undefined,
      endpoint: typeof endpoint === 'string' ? endpoint : undefined,
      api_key: typeof apiKey === 'string' && apiKey && !apiKey.includes('****') ? apiKey : undefined,
    }

    setTestingCategory(category)
    try {
      const res = await VoiceAgentController.testVoiceAgent(payload)
      const data = res.data
      const parts: string[] = []
      if (data?.message) parts.push(data.message)
      if (typeof data?.elapsed_ms === 'number') parts.push(`${data.elapsed_ms} ms`)
      if (data?.success) {
        message.success(parts.length > 0 ? parts.join('，') : '连通性测试通过')
      } else {
        message.warning(parts.length > 0 ? parts.join('，') : '连通性测试未通过')
      }
    } catch (err) {
      message.error(apiErrorMessage(err, '连通性测试失败'))
    } finally {
      setTestingCategory(undefined)
    }
  }

  const handleApply = async () => {
    setApplying(true)
    try {
      await VoiceAgentController.applyVoiceAgentConfig()
      message.success('已下发到 Pipecat，配置在下一个会话生效')
    } catch (err) {
      message.error(apiErrorMessage(err, '应用配置失败'))
    } finally {
      setApplying(false)
    }
  }

  const testButton = (category: Category) => (
    canTest ? (
      <Button
        size="small"
        icon={<ApiOutlined />}
        loading={testingCategory === category}
        onClick={() => void handleTest(category)}
      >
        测试连通性
      </Button>
    ) : undefined
  )

  // ── WebSocket 连接测试 ──

  const appendWsLine = (line: string) => {
    const stamped = `[${dayjs().format('HH:mm:ss')}] ${line}`
    setWsLines((prev) => [...prev.slice(-199), stamped])
  }

  const disconnectWs = () => {
    const socket = wsRef.current
    wsRef.current = null
    if (socket) {
      socket.onopen = null
      socket.onmessage = null
      socket.onerror = null
      socket.onclose = null
      socket.close()
    }
    setWsState((prev) => (prev === 'idle' ? 'idle' : 'closed'))
  }

  const handleWsMessage = async (data: unknown) => {
    let text: string
    if (typeof data === 'string') {
      text = data
    } else if (data instanceof Blob) {
      text = await data.text()
    } else {
      text = String(data)
    }
    appendWsLine(`← ${text}`)
    try {
      const parsed = JSON.parse(text) as VoiceServerHello & { type?: string }
      if (parsed.type === 'hello') {
        setServerHello(parsed)
      }
    } catch {
      // 非 JSON 消息（如原始音频帧）只记录原文。
    }
  }

  const connectWs = (ticket: string, wsUrl?: string) => {
    disconnectWs()
    setServerHello(null)
    setWsLines([])
    setWsState('connecting')
    appendWsLine(`连接 ${buildVoiceWsUrl(ticket, wsUrl).replace(/ticket=[^&]+/, 'ticket=***')}`)

    let socket: WebSocket
    try {
      socket = new WebSocket(buildVoiceWsUrl(ticket, wsUrl))
    } catch (err) {
      setWsState('error')
      appendWsLine(`创建 WebSocket 失败：${apiErrorMessage(err, '未知错误')}`)
      return
    }
    wsRef.current = socket

    const timeout = window.setTimeout(() => {
      if (wsRef.current === socket && socket.readyState !== WebSocket.OPEN) {
        setWsState('error')
        appendWsLine('连接超时（10s），未完成握手')
        socket.close()
      }
    }, 10000)

    socket.onopen = () => {
      window.clearTimeout(timeout)
      setWsState('open')
      appendWsLine('WebSocket 已连接，发送 hello')
      socket.send(JSON.stringify(buildClientHello()))
    }
    socket.onmessage = (event) => {
      void handleWsMessage(event.data)
    }
    socket.onerror = () => {
      setWsState('error')
      appendWsLine('WebSocket 发生错误')
    }
    socket.onclose = (event) => {
      window.clearTimeout(timeout)
      setWsState((prev) => (prev === 'error' ? 'error' : 'closed'))
      appendWsLine(`连接已关闭 code=${event.code}${event.reason ? ` reason=${event.reason}` : ''}`)
      if (wsRef.current === socket) wsRef.current = null
    }
  }

  const handleConnect = async () => {
    if (!canTest) return
    let ticket = wsTicket.trim()
    let wsUrl: string | undefined
    if (!ticket) {
      setTicketLoading(true)
      try {
        const res = await VoiceAgentController.createWebVoiceWsTicket({
          client_id: randomClientId(),
        })
        if (res.code !== 200 || !res.data?.ticket) {
          message.error(res.message || '后端未返回 WebSocket ticket')
          return
        }
        ticket = res.data.ticket
        wsUrl = res.data.ws_url
        // 一次性 ticket 用完即弃：不写回输入框，避免下次误用已失效的 ticket。
      } catch (err) {
        message.error(apiErrorMessage(err, '获取 WebSocket ticket 失败'))
        return
      } finally {
        setTicketLoading(false)
      }
    }
    if (!ticket) {
      message.warning('请先获取或粘贴一次性 ticket')
      return
    }
    connectWs(ticket, wsUrl)
  }

  // ── MCP 工具编辑 ──

  const openMcpToolEditor = (row: VoiceMcpToolItem) => {
    setMcpToolEditing(row)
    mcpToolForm.setFieldsValue({
      enabled: row.enabled ?? true,
      risk_level: row.risk_level ?? 'low',
      require_confirm: row.require_confirm ?? false,
    })
  }

  const handleSaveMcpTool = async () => {
    const values = await mcpToolForm.validateFields()
    if (mcpToolEditing?.id === undefined) {
      message.warning('工具 ID 缺失，无法更新')
      return
    }
    setMcpToolSaving(true)
    try {
      await VoiceAgentController.updateVoiceMcpTool(mcpToolEditing.id, values)
      message.success('MCP 工具配置已保存')
      setMcpToolEditing(null)
      void loadMcpTools(mcpToolPage, mcpToolPageSize)
    } catch (err) {
      message.error(apiErrorMessage(err, '保存 MCP 工具失败'))
    } finally {
      setMcpToolSaving(false)
    }
  }

  // ── 记忆操作 ──

  const handleMemorySearch = () => {
    const query = {
      keyword: memoryKeyword.trim() || undefined,
      owner_type: memoryOwnerType,
    }
    setMemoryQuery(query)
    setMemoryPage(1)
    void loadMemories(1, memoryPageSize, query)
  }

  const handleMemoryReset = () => {
    setMemoryKeyword('')
    setMemoryOwnerType(undefined)
    setMemoryQuery({})
    setMemoryPage(1)
    void loadMemories(1, memoryPageSize, {})
  }

  const handleDeleteMemory = async (row: VoiceMemoryItem) => {
    if (row.id === undefined) {
      message.warning('记忆 ID 缺失，无法删除')
      return
    }
    try {
      await VoiceAgentController.deleteVoiceMemory(row.id)
      message.success('记忆已删除')
      void loadMemories(memoryPage, memoryPageSize)
    } catch (err) {
      message.error(apiErrorMessage(err, '删除记忆失败'))
    }
  }

  const handleExportMemories = async () => {
    try {
      // 后端返回 CSV 文件流（UTF-8 BOM），复用生成器暴露的路径常量 + JWT fetch。
      await downloadVoiceMemoriesCsv({
        keyword: memoryQuery.keyword,
        owner_type: memoryQuery.owner_type,
      })
      message.success('导出成功')
      void loadMemories(memoryPage, memoryPageSize)
    } catch (err) {
      message.error(apiErrorMessage(err, '导出记忆失败'))
    }
  }

  // ── 列定义 ──

  const mcpToolColumns: ColumnsType<VoiceMcpToolItem> = [
    {
      title: '工具',
      dataIndex: 'name',
      key: 'name',
      render: (_: unknown, row) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{row.name || '-'}</Typography.Text>
          {row.display_name ? <Typography.Text type="secondary">{row.display_name}</Typography.Text> : null}
        </Space>
      ),
    },
    { title: '说明', dataIndex: 'description', key: 'description', ellipsis: true, render: (v?: string) => v || '-' },
    { title: '来源', dataIndex: 'source', key: 'source', width: 100, render: (v?: string) => (v ? <Tag>{v}</Tag> : '-') },
    { title: '风险等级', dataIndex: 'risk_level', key: 'risk_level', width: 110, render: riskTag },
    {
      title: '启用',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 90,
      render: (v?: boolean) => (v ? <Tag color="green">启用</Tag> : <Tag>停用</Tag>),
    },
    {
      title: '需确认',
      dataIndex: 'require_confirm',
      key: 'require_confirm',
      width: 100,
      render: (v?: boolean) => (v ? <Tag color="orange">需确认</Tag> : <Tag>直接执行</Tag>),
    },
    { title: '创建时间', dataIndex: 'created_dt', key: 'created_dt', width: 170, render: fmtDT },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (_: unknown, row) => (
        <ActionButtons maxWidth={80}>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            className="e2e-voice-mcp-tool-edit-btn"
            disabled={!canUpdate || row.id === undefined}
            onClick={() => openMcpToolEditor(row)}
          >
            编辑
          </Button>
        </ActionButtons>
      ),
    },
  ]

  const mcpLogColumns: ColumnsType<VoiceMcpLogItem> = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 80 },
    { title: '会话', dataIndex: 'session_id', key: 'session_id', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '工具', dataIndex: 'tool_name', key: 'tool_name', width: 160, render: (v?: string) => v || '-' },
    { title: '用户 ID', dataIndex: 'user_id', key: 'user_id', width: 90, render: (v?: number | null) => (v ?? '-') },
    { title: '风险等级', dataIndex: 'risk_level', key: 'risk_level', width: 110, render: riskTag },
    {
      title: '已确认',
      dataIndex: 'confirmed',
      key: 'confirmed',
      width: 90,
      render: (v?: boolean) => (v ? <Tag color="blue">是</Tag> : <Tag>否</Tag>),
    },
    { title: '结果', dataIndex: 'status', key: 'status', width: 90, render: mcpStatusTag },
    { title: '耗时', dataIndex: 'elapsed_ms', key: 'elapsed_ms', width: 100, render: (v?: number) => (typeof v === 'number' ? `${v} ms` : '-') },
    { title: '错误信息', dataIndex: 'error_msg', key: 'error_msg', ellipsis: true, render: (v?: string) => v || '-' },
    { title: '时间', dataIndex: 'created_dt', key: 'created_dt', width: 170, render: fmtDT },
  ]

  const memoryColumns: ColumnsType<VoiceMemoryItem> = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 80 },
    {
      title: '归属',
      key: 'owner',
      width: 150,
      render: (_: unknown, row) => (
        <Space direction="vertical" size={0}>
          <Tag>{row.owner_type || '-'}</Tag>
          <Typography.Text type="secondary">{row.owner_key || '-'}</Typography.Text>
        </Space>
      ),
    },
    { title: '类型', dataIndex: 'kind', key: 'kind', width: 100, render: (v?: string) => v || '-' },
    {
      title: '内容',
      dataIndex: 'content',
      key: 'content',
      ellipsis: true,
      render: (v?: string) => v || '-',
    },
    { title: '重要度', dataIndex: 'importance', key: 'importance', width: 90, render: (v?: number) => (typeof v === 'number' ? v : '-') },
    { title: '向量状态', dataIndex: 'embedding_status', key: 'embedding_status', width: 120, render: embeddingTag },
    { title: '来源会话', dataIndex: 'source_session_id', key: 'source_session_id', width: 160, ellipsis: true, render: (v?: string) => v || '-' },
    { title: '创建时间', dataIndex: 'created_dt', key: 'created_dt', width: 170, render: fmtDT },
    { title: '过期时间', dataIndex: 'expires_dt', key: 'expires_dt', width: 170, render: fmtDT },
    {
      title: '操作',
      key: 'action',
      width: 160,
      render: (_: unknown, row) => (
        <ActionButtons maxWidth={150}>
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            className="e2e-voice-memory-detail-btn"
            onClick={() => setMemoryDetail(row)}
          >
            详情
          </Button>
          <Popconfirm
            title="删除该条记忆？"
            description="删除后不可恢复，后续会话不会再检索到。"
            okText="删除"
            cancelText="取消"
            disabled={!canUpdate}
            onConfirm={() => void handleDeleteMemory(row)}
          >
            <Button
              type="link"
              size="small"
              danger
              icon={<DeleteOutlined />}
              className="e2e-voice-memory-delete-btn"
              disabled={!canUpdate}
            >
              删除
            </Button>
          </Popconfirm>
        </ActionButtons>
      ),
    },
  ]

  // ── Tab 内容 ──

  const renderBasicTab = () => (
    <>
      <SectionTitle title="STT 语音识别" action={testButton('stt')} />
      <Row gutter={24}>
        <Col xs={24} md={12}>
          <Form.Item
            name="stt_provider"
            label="STT Provider"
            rules={[{ required: true, message: '请选择 STT Provider' }]}
          >
            <Select
              placeholder="请选择 STT Provider"
              options={providerOptions('stt')}
              onChange={(value: string) => applyProviderDefaults('stt', value)}
            />
          </Form.Item>
        </Col>
        {renderCategoryFields('stt', sttProvider)}
      </Row>

      <SectionTitle title="LLM 对话模型" action={testButton('llm')} />
      <Row gutter={24}>
        <Col xs={24} md={12}>
          <Form.Item
            name="llm_provider"
            label="LLM Provider"
            rules={[{ required: true, message: '请选择 LLM Provider' }]}
          >
            <Select
              placeholder="请选择 LLM Provider"
              options={providerOptions('llm')}
              onChange={(value: string) => applyProviderDefaults('llm', value)}
            />
          </Form.Item>
        </Col>
        {renderCategoryFields('llm', llmProvider)}
      </Row>

      <SectionTitle title="TTS 语音合成" action={testButton('tts')} />
      <Row gutter={24}>
        <Col xs={24} md={12}>
          <Form.Item
            name="tts_provider"
            label="TTS Provider"
            rules={[{ required: true, message: '请选择 TTS Provider' }]}
          >
            <Select
              placeholder="请选择 TTS Provider"
              options={providerOptions('tts')}
              onChange={(value: string) => applyProviderDefaults('tts', value)}
            />
          </Form.Item>
        </Col>
        {renderCategoryFields('tts', ttsProvider)}
      </Row>

      <SectionTitle title="VAD 语音活动检测" action={testButton('vad')} />
      <Row gutter={24}>
        <Col xs={24} md={12}>
          <Form.Item
            name="vad_provider"
            label="VAD Provider"
            rules={[{ required: true, message: '请选择 VAD Provider' }]}
          >
            <Select
              placeholder="请选择 VAD Provider"
              options={providerOptions('vad')}
              onChange={(value: string) => applyProviderDefaults('vad', value)}
            />
          </Form.Item>
        </Col>
        {renderVadFields(vadProvider)}
      </Row>

      <SectionTitle title="运行参数与 Pipecat 控制面" />
      <Row gutter={24}>
        <Col xs={24} md={12}>
          <Form.Item name="session_idle_timeout" label="会话空闲超时（秒）">
            <InputNumber min={1} max={86400} style={{ width: '100%' }} placeholder="60" />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item name="max_session_sec" label="会话最大时长（秒）">
            <InputNumber min={1} max={86400} style={{ width: '100%' }} placeholder="600" />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item
            name="barge_in"
            label="允许打断（barge_in）"
            valuePropName="checked"
            extra="半双工场景建议保持关闭，避免设备回声触发自激。"
          >
            <Switch checkedChildren="开启" unCheckedChildren="关闭" />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item name="pipecat_endpoint" label="Pipecat 控制面地址">
            <Input placeholder="例如 http://pipecat:8000" maxLength={255} />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item
            name="pipecat_token"
            label="Pipecat 控制面令牌"
            extra="留空或含 **** 表示不修改"
          >
            <Input.Password placeholder="留空或含 **** 表示不修改" maxLength={500} autoComplete="new-password" />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  const renderWebSocketTab = () => (
    <Row gutter={24}>
      <Col xs={24} md={12}>
        <Form.Item
          name="ws_enabled"
          label="启用 WebSocket 接入"
          valuePropName="checked"
          extra="关闭后 App / Web / 设备只能走 GB28181 RTP 通道。"
        >
          <Switch checkedChildren="启用" unCheckedChildren="停用" />
        </Form.Item>
      </Col>
      <Col xs={24} md={12}>
        <Form.Item
          name="ws_path"
          label="Pipecat WebSocket 路径"
          extra="默认 /ws/voice；管理端连接测试固定走网关 /web/voice/ws。"
        >
          <Input maxLength={255} placeholder="/ws/voice" />
        </Form.Item>
      </Col>
      <Col xs={24} md={12}>
        <Form.Item
          name="fast_ack_text"
          label="快速应答话术"
          extra="设备端唤醒命中后立即播报的短句（P2）。"
        >
          <Input maxLength={100} placeholder="我在" />
        </Form.Item>
      </Col>
      <Col xs={24} md={12}>
        <Form.Item
          name="device_wake_word"
          label="设备唤醒词"
          extra="仅登记设备端 KWS 的唤醒词，服务端不跑唤醒检测。"
        >
          <Input maxLength={100} placeholder="例如：你好小智" />
        </Form.Item>
      </Col>

      <Col span={24}>
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="半双工与回声控制"
          description="TTS start → stop 期间服务端丢弃上行并保留 300ms 尾巴；客户端应在 tts start 时暂停麦克风。默认关闭 barge_in，避免设备喇叭回声触发自激。"
        />
      </Col>

      <Col span={24}>
        <Card
          size="small"
          title="连接测试"
          extra={
            <Space>
              <Tag color={WS_STATE_LABEL[wsState].color}>{WS_STATE_LABEL[wsState].text}</Tag>
              <Button
                size="small"
                icon={<LinkOutlined />}
                className="e2e-voice-connect-btn"
                loading={ticketLoading || wsState === 'connecting'}
                disabled={!canTest}
                onClick={() => void handleConnect()}
              >
                获取 Ticket 并连接
              </Button>
              <Button
                size="small"
                icon={<DisconnectOutlined />}
                className="e2e-voice-disconnect-btn"
                disabled={!canTest || wsState === 'idle' || wsState === 'closed'}
                onClick={disconnectWs}
              >
                断开
              </Button>
            </Space>
          }
        >
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message="浏览器仅验证信令链路，不采集、不发送音频"
            description="点击后先调用 POST /web/voice/ws-ticket 获取一次性 ticket，再连接后端返回的 ws_url（或默认 /web/voice/ws）并发送 hello。ticket 有效期 60 秒且一次性。"
          />
          <Space.Compact style={{ width: '100%', marginBottom: 12 }}>
            <Input.Password
              value={wsTicket}
              onChange={(e) => setWsTicket(e.target.value)}
              placeholder="一次性 ticket（接入后自动获取，也可手动粘贴）"
              maxLength={500}
              autoComplete="off"
              disabled={!canTest || wsState === 'connecting' || wsState === 'open'}
            />
          </Space.Compact>

          {serverHello ? (
            <Descriptions size="small" bordered column={2} style={{ marginBottom: 12 }}>
              <Descriptions.Item label="session_id">{serverHello.session_id || '-'}</Descriptions.Item>
              <Descriptions.Item label="transport">{serverHello.transport || '-'}</Descriptions.Item>
              <Descriptions.Item label="音频参数" span={2}>
                <Typography.Text code>{JSON.stringify(serverHello.audio_params ?? {})}</Typography.Text>
              </Descriptions.Item>
              <Descriptions.Item label="能力" span={2}>
                <Typography.Text code>{JSON.stringify(serverHello.capabilities ?? {})}</Typography.Text>
              </Descriptions.Item>
            </Descriptions>
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="尚未收到服务端 hello" />
          )}

          <Typography.Text type="secondary">信令日志</Typography.Text>
          <pre
            data-testid="e2e-voice-ws-log"
            style={{
              margin: '8px 0 0',
              maxHeight: 220,
              overflow: 'auto',
              padding: 12,
              borderRadius: 6,
              background: 'rgba(0, 0, 0, 0.04)',
              fontSize: 12,
              whiteSpace: 'pre-wrap',
            }}
          >
            {wsLines.length > 0 ? wsLines.join('\n') : '（暂无消息）'}
          </pre>
        </Card>
      </Col>
    </Row>
  )

  const renderMcpTab = () => (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="服务端 MCP"
        description="Pipecat 通过内部 HTTP JSON-RPC 调用 Go；工具实现从签名上下文注入 user_id/dept_id，写操作按风险等级要求确认。"
      />

      <Row gutter={24}>
        <Col xs={24} md={12}>
          <Form.Item
            name="mcp_enabled"
            label="启用服务端 MCP"
            valuePropName="checked"
            extra="关闭后 LLM 不可调用设备查询 / 云台 / 通知等工具。"
          >
            <Switch checkedChildren="启用" unCheckedChildren="停用" />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item
            name="mcp_server_url"
            label="内部 MCP 地址"
            extra="Pipecat 访问 Go 的 /internal/mcp 完整地址（仅内网）。"
          >
            <Input maxLength={255} placeholder="例如 http://backend:8080/internal/mcp" />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item
            name="internal_mcp_token"
            label="内部 MCP Token"
            extra="只写不读，读接口返回 ****；留空或含 **** 表示不修改。"
          >
            <Input.Password placeholder="留空或含 **** 表示不修改" maxLength={500} autoComplete="new-password" />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle
        title="工具列表"
        action={
          <Button
            size="small"
            icon={<ReloadOutlined />}
            className="e2e-voice-mcp-refresh-btn"
            loading={mcpToolsLoading}
            disabled={!canView}
            onClick={() => void loadMcpTools()}
          >
            刷新
          </Button>
        }
      />
      {mcpToolsError ? (
        <Alert type="info" showIcon style={{ marginBottom: 12 }} message={mcpToolsError} />
      ) : null}
      <Table<VoiceMcpToolItem>
        className="e2e-voice-mcp-tool-table"
        rowKey={(row) => String(row.id ?? row.name ?? '')}
        size="small"
        columns={mcpToolColumns}
        dataSource={mcpTools}
        loading={mcpToolsLoading}
        scroll={{ x: 1080 }}
        pagination={{
          current: mcpToolPage,
          pageSize: mcpToolPageSize,
          total: mcpToolTotal,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条`,
          onChange: (page, pageSize) => {
            setMcpToolPage(page)
            setMcpToolPageSize(pageSize)
            void loadMcpTools(page, pageSize)
          },
        }}
        locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无工具数据" /> }}
      />

      <SectionTitle
        title="调用日志"
        action={
          <Button
            size="small"
            icon={<ReloadOutlined />}
            className="e2e-voice-mcp-log-refresh-btn"
            loading={mcpLogLoading}
            disabled={!canView}
            onClick={() => void loadMcpLogs(mcpLogPage, mcpLogPageSize)}
          >
            刷新
          </Button>
        }
      />
      {mcpLogError ? (
        <Alert type="info" showIcon style={{ marginBottom: 12 }} message={mcpLogError} />
      ) : null}
      <Table<VoiceMcpLogItem>
        className="e2e-voice-mcp-log-table"
        rowKey={(row) => String(row.id ?? `${row.session_id}-${row.created_dt}`)}
        size="small"
        columns={mcpLogColumns}
        dataSource={mcpLogs}
        loading={mcpLogLoading}
        scroll={{ x: 1080 }}
        pagination={{
          current: mcpLogPage,
          pageSize: mcpLogPageSize,
          total: mcpLogTotal,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条`,
          onChange: (page, pageSize) => {
            setMcpLogPage(page)
            setMcpLogPageSize(pageSize)
            void loadMcpLogs(page, pageSize)
          },
        }}
        locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无调用日志" /> }}
      />

      <Modal
        title="编辑 MCP 工具"
        className="e2e-voice-mcp-tool-modal"
        open={mcpToolEditing !== null}
        onCancel={() => setMcpToolEditing(null)}
        onOk={() => void handleSaveMcpTool()}
        confirmLoading={mcpToolSaving}
        okText="保存"
        cancelText="取消"
        destroyOnHidden
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message={mcpToolEditing?.name ? `工具：${mcpToolEditing.name}` : '工具'}
          description="风险等级 ≥ 中或需确认时，LLM 调用前必须经过用户确认。"
        />
        <Form form={mcpToolForm} layout="vertical">
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch checkedChildren="启用" unCheckedChildren="停用" />
          </Form.Item>
          <Form.Item name="risk_level" label="风险等级">
            <Select
              options={[
                { label: '低风险', value: 'low' },
                { label: '中风险（需确认）', value: 'medium' },
                { label: '高风险（需确认）', value: 'high' },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="require_confirm"
            label="调用前需用户确认"
            valuePropName="checked"
          >
            <Switch checkedChildren="需要" unCheckedChildren="不需要" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  )

  const renderMemoryTab = () => (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="长期记忆"
        description="记忆使用 pgvector 检索；未配置 Embedding 时按时间/关键词检索并标记 embedding_status=failed。默认保留 180 天，不存原始音频。"
      />

      <Row gutter={24}>
        <Col xs={24} md={12}>
          <Form.Item
            name="memory_enabled"
            label="启用长期记忆"
            valuePropName="checked"
            extra="关闭后不再抽取/检索跨会话事实与偏好。"
          >
            <Switch checkedChildren="启用" unCheckedChildren="停用" />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item
            name="memory_top_k"
            label="会话开始检索条数（memory_top_k）"
            extra="设计建议 5-8。"
          >
            <InputNumber min={0} max={50} style={{ width: '100%' }} placeholder="6" />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item name="memory_ttl_days" label="记忆保留天数（memory_ttl_days）">
            <InputNumber min={1} max={3650} style={{ width: '100%' }} placeholder="180" />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item
            name="embedding_endpoint"
            label="Embedding 服务地址"
            extra="OpenAI-compatible /v1/embeddings；留空则降级为关键词检索。"
          >
            <Input maxLength={255} placeholder="例如 http://bge-m3:9997/v1/embeddings" />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item name="embedding_model" label="Embedding 模型">
            <Input maxLength={100} placeholder="BGE-M3" />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item
            name="embedding_api_key"
            label="Embedding API Key"
            extra="只写不读，读接口返回 ****；留空或含 **** 表示不修改。"
          >
            <Input.Password placeholder="留空或含 **** 表示不修改" maxLength={500} autoComplete="new-password" />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle title="记忆列表" />
      <Space wrap style={{ marginBottom: 12 }}>
        <Input
          value={memoryKeyword}
          onChange={(e) => setMemoryKeyword(e.target.value)}
          onPressEnter={handleMemorySearch}
          placeholder="按内容关键字搜索"
          allowClear
          style={{ width: 240 }}
        />
        <Select
          value={memoryOwnerType}
          onChange={setMemoryOwnerType}
          placeholder="归属类型"
          allowClear
          style={{ width: 150 }}
          options={MEMORY_OWNER_OPTIONS}
        />
        <Button
          type="primary"
          className="e2e-voice-memory-search-btn"
          disabled={!canView}
          onClick={handleMemorySearch}
        >
          搜索
        </Button>
        <Button className="e2e-voice-memory-reset-btn" onClick={handleMemoryReset}>
          重置
        </Button>
        <Button
          icon={<DownloadOutlined />}
          className="e2e-voice-memory-export-btn"
          disabled={!canView}
          onClick={() => void handleExportMemories()}
        >
          导出
        </Button>
        <Button
          icon={<ReloadOutlined />}
          className="e2e-voice-memory-refresh-btn"
          loading={memoryLoading}
          disabled={!canView}
          onClick={() => void loadMemories(memoryPage, memoryPageSize)}
        >
          刷新
        </Button>
      </Space>
      {memoryError ? (
        <Alert type="info" showIcon style={{ marginBottom: 12 }} message={memoryError} />
      ) : null}
      <Table<VoiceMemoryItem>
        className="e2e-voice-memory-table"
        rowKey={(row) => String(row.id ?? `${row.owner_type}-${row.created_dt}`)}
        size="small"
        columns={memoryColumns}
        dataSource={memories}
        loading={memoryLoading}
        scroll={{ x: 1200 }}
        pagination={{
          current: memoryPage,
          pageSize: memoryPageSize,
          total: memoryTotal,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条`,
          onChange: (page, pageSize) => {
            setMemoryPage(page)
            setMemoryPageSize(pageSize)
            void loadMemories(page, pageSize)
          },
        }}
        locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无记忆" /> }}
      />

      <Modal
        title="记忆详情"
        className="e2e-voice-memory-modal"
        open={memoryDetail !== null}
        footer={<Button onClick={() => setMemoryDetail(null)}>关闭</Button>}
        onCancel={() => setMemoryDetail(null)}
      >
        <Descriptions size="small" bordered column={1}>
          <Descriptions.Item label="ID">{memoryDetail?.id ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="归属">
            {memoryDetail?.owner_type || '-'} / {memoryDetail?.owner_key || '-'}
          </Descriptions.Item>
          <Descriptions.Item label="类型">{memoryDetail?.kind || '-'}</Descriptions.Item>
          <Descriptions.Item label="内容">
            <Typography.Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>
              {memoryDetail?.content || '-'}
            </Typography.Paragraph>
          </Descriptions.Item>
          <Descriptions.Item label="重要度">{memoryDetail?.importance ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="来源会话">{memoryDetail?.source_session_id || '-'}</Descriptions.Item>
          <Descriptions.Item label="向量状态">{embeddingTag(memoryDetail?.embedding_status)}</Descriptions.Item>
          <Descriptions.Item label="创建时间">{fmtDT(memoryDetail?.created_dt)}</Descriptions.Item>
          <Descriptions.Item label="最后召回">{fmtDT(memoryDetail?.last_access_dt)}</Descriptions.Item>
          <Descriptions.Item label="过期时间">{fmtDT(memoryDetail?.expires_dt)}</Descriptions.Item>
        </Descriptions>
      </Modal>
    </>
  )

  const renderObserveTab = () => (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="Pipecat 观测"
        description="后端 v0.6.0 当前未提供 /web/voice/health 与 /web/voice/sessions；本页展示已保存的控制面配置。控制面不可达时 App/设备端会降级为纯 GB28181 通道。"
      />

      <Descriptions size="small" bordered column={2} style={{ marginBottom: 16 }}>
        <Descriptions.Item label="当前控制面地址">{pipecatEndpoint || '未配置'}</Descriptions.Item>
        <Descriptions.Item label="WS 接入">
          {wsEnabled ? <Tag color="green">已启用</Tag> : <Tag>已停用</Tag>}
        </Descriptions.Item>
        <Descriptions.Item label="配置存储">ac_thirdparty_config（Category=voice_agent）</Descriptions.Item>
        <Descriptions.Item label="生效方式">保存后点击「应用到 Pipecat」，下一个会话生效</Descriptions.Item>
      </Descriptions>

      <Space style={{ marginBottom: 16 }}>
        <Button
          icon={<CloudUploadOutlined />}
          className="e2e-voice-apply-btn"
          loading={applying}
          disabled={!canApply}
          onClick={() => void handleApply()}
        >
          应用到 Pipecat
        </Button>
      </Space>
    </>
  )

  return (
    <PageContainer
      title="语音 Agent 配置"
      subTitle="Pipecat 的 STT / LLM / TTS / VAD、WebSocket、MCP、记忆与观测"
      extra={
        <Space>
          <Button
            icon={<ReloadOutlined />}
            className="e2e-voice-refresh-btn"
            onClick={() => void load()}
            loading={loading}
            disabled={!canView}
          >
            刷新
          </Button>
          {canApply && (
            <Button
              icon={<CloudUploadOutlined />}
              onClick={() => void handleApply()}
              loading={applying}
            >
              应用到 Pipecat
            </Button>
          )}
          {canUpdate && (
            <Button
              type="primary"
              icon={<SaveOutlined />}
              className="e2e-voice-save-btn"
              onClick={() => void handleSave()}
              loading={saving}
            >
              保存
            </Button>
          )}
        </Space>
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="配置存于 ac_thirdparty_config，密钥只写不读"
        description="保存后点击「应用到 Pipecat」，配置在下一个会话生效。默认半双工（barge_in 关闭）以避免设备喇叭回声导致自激。"
      />

      {!canView ? (
        <Alert type="warning" showIcon message="无查看权限" description="当前账号缺少 voice:agent:view 权限码。" />
      ) : (
        <Card loading={loading}>
          <Form form={form} layout="vertical" disabled={!canUpdate}>
            <Tabs
              className="e2e-voice-agent-tabs"
              activeKey={activeKey}
              onChange={handleTabChange}
              items={[
                { key: 'basic', label: '基础', children: renderBasicTab() },
                { key: 'websocket', label: 'WebSocket', children: renderWebSocketTab() },
                { key: 'mcp', label: 'MCP', children: renderMcpTab() },
                { key: 'memory', label: '记忆', children: renderMemoryTab() },
                { key: 'observe', label: '观测', children: renderObserveTab() },
              ]}
            />
          </Form>
        </Card>
      )}
    </PageContainer>
  )
}

export default VoiceAgentConfig
