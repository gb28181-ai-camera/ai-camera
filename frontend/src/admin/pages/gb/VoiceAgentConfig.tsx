import React, { useCallback, useEffect, useState } from 'react'
import {
  Alert, Button, Card, Col, Form, Input, InputNumber, message, Row, Select, Space, Switch, Typography,
} from 'antd'
import {
  ApiOutlined, CloudUploadOutlined, ReloadOutlined, SaveOutlined,
} from '@ant-design/icons'
import axios from 'axios'
import PageContainer from '../../components/PageContainer'
import { VoiceAgentController } from '../../api/generated/VoiceAgentController'
import type { VoiceProviderItem } from '../../api/generated/definitions/VoiceProviderItem'
import type { VoiceAgentConfigResp } from '../../api/generated/definitions/VoiceAgentConfigResp'
import type { UpdateVoiceAgentReq } from '../../api/generated/definitions/UpdateVoiceAgentReq'
import type { TestVoiceAgentReq } from '../../api/generated/definitions/TestVoiceAgentReq'
import { useAuthStore } from '../../store/authStore'

type Category = 'stt' | 'llm' | 'tts' | 'vad'

interface ParamFieldDef {
  key: string
  label: string
  type: 'text' | 'password' | 'number' | 'textarea'
  required?: boolean
  secret?: boolean
}

/**
 * 后端新增的扩展参数字段。
 *
 * 生成的 `VoiceAgentConfigResp` / `UpdateVoiceAgentReq` 还没有 extra_params
 * （需要重新执行 api-gen.sh）；这里用交叉类型做**前置声明**，不改生成文件。
 * 由于 extra_params 是可选属性，原类型可直接赋值给交叉类型，无需 as 断言。
 */
type VoiceAgentConfigRespWithExtras = VoiceAgentConfigResp & {
  extra_params?: Record<string, string>
}
type UpdateVoiceAgentReqWithExtras = UpdateVoiceAgentReq & {
  extra_params?: Record<string, string>
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

function SectionTitle({ title, action }: { title: string; action?: React.ReactNode }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', margin: '0 0 16px' }}>
      <Typography.Title level={5} style={{ margin: 0 }}>{title}</Typography.Title>
      {action}
    </div>
  )
}

/**
 * 语音 Agent（Pipecat）动态配置。
 *
 * 配置复用 ac_thirdparty_config（Category='voice_agent'）：
 * - 密钥只写不读，后端返回 ****，提交 **** / 空串时保持原值；
 * - provider 下拉来自 /providers 字典，参数表单由 provider 的 param_schema 动态渲染；
 * - 保存后需「应用到 Pipecat」，配置在下一个会话生效。
 */
const VoiceAgentConfig: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canView = permCodes.includes('voice:agent:view')
  const canUpdate = permCodes.includes('voice:agent:update')
  const canTest = permCodes.includes('voice:agent:test')
  const canApply = permCodes.includes('voice:agent:apply')

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

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [configRes, providerRes] = await Promise.all([
        VoiceAgentController.getVoiceAgentConfig(),
        VoiceAgentController.listVoiceAgentProviders(),
      ])
      const list = providerRes.data ?? []
      setProviders(list)

      const config: VoiceAgentConfigRespWithExtras | undefined = configRes.data
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
        // 后端返回的键已去掉 param_ 前缀，直接与 schema.key 对齐。
        const extras = config.extra_params ?? {}
        for (const category of ['stt', 'llm', 'tts'] as const) {
          const code = config[`${category}_provider`]
          const providerItem = list.find((item) => item.category === category && item.code === code)
          const schema = parseParamSchema(providerItem?.param_schema)
          for (const def of schema) {
            if (PARAM_FIELD_MAP[category][def.key.toLowerCase()]) continue
            if (!(def.key in extras)) continue
            // InputNumber 需要 number；文本/密码保持字符串。
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
      // 没有固定键的 schema 项走 ExtraParams 表单字段。
      // 新 provider 未提供默认值时清空，避免上一个 provider 的扩展值串台。
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
      // 没有固定键的 schema 项（如 TTS 的 model、Azure 的 region）渲染为
      // 扩展参数字段，提交时进 ExtraParams、回显时从 config.extra_params 读取。
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

  const handleSave = async () => {
    const values = await form.validateFields()

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
    // 键保持 provider 原始 key（后端负责加 param_ 前缀落库）。
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

    const payload: UpdateVoiceAgentReqWithExtras = {
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
      // **** 是脱敏串；不传时后端使用库中已保存的密钥。
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
        disabled={false}
        loading={testingCategory === category}
        onClick={() => void handleTest(category)}
      >
        测试连通性
      </Button>
    ) : undefined
  )

  return (
    <PageContainer
      title="语音 Agent 配置"
      subTitle="Pipecat 的 STT / LLM / TTS / VAD 动态配置"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading} disabled={!canView}>
            刷新
          </Button>
          {canApply && (
            <Button icon={<CloudUploadOutlined />} onClick={() => void handleApply()} loading={applying}>
              应用到 Pipecat
            </Button>
          )}
          {canUpdate && (
            <Button type="primary" icon={<SaveOutlined />} onClick={() => void handleSave()} loading={saving}>
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
        <Card loading={loading} style={{ maxWidth: 900 }}>
          <Form form={form} layout="vertical" disabled={!canUpdate}>
            {/* STT */}
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

            {/* LLM */}
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

            {/* TTS */}
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

            {/* VAD */}
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

            {/* 运行参数 / Pipecat 控制面 */}
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
          </Form>
        </Card>
      )}
    </PageContainer>
  )
}

export default VoiceAgentConfig
