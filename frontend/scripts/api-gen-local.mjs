#!/usr/bin/env node
/* eslint-disable no-console */
/**
 * =============================================================================
 * api-gen-local.mjs —— 自包含的前端 API 代码生成器（零外部依赖）
 * =============================================================================
 *
 * 背景：`scripts/api-gen.sh` 原本依赖外部 CLI `swagger2-doc-parser`（本机通常没有），
 * 导致 `src/admin/api/generated/**` 一直是手写占位实现。本脚本用 Node 内置能力
 * 直接消费 swagger 2.0 JSON，产出与 swagger2-doc-parser 风格一致的 Controller
 * 与 definitions，使整条链路在没有外部 CLI 时也能真正跑通、可复现。
 *
 * 用法：
 *   node scripts/api-gen-local.mjs --input <swagger.json> --out <输出目录> [选项]
 *
 * 选项：
 *   --input   <path>   swagger 2.0 JSON 文件（必填）
 *   --out     <dir>    输出目录，Controller 与 definitions/ 都写到这里（必填）
 *   --base-url <expr>  请求基址表达式，默认 `import.meta.env.VITE_API_BASE`
 *   --manifest <path>  preserve 清单（手动保留的方法/别名），默认 scripts/api-gen-preserve.mjs
 *   --group-mode <m>   `path`（默认，按 tag + 路径前缀细分到现有控制器文件）
 *                      或 `tag`（严格按 swagger tag 分组，一个 tag 一个 Controller）
 *   --enum-mode <m>    `wide`（默认，字面量联合 + `(string & {})` 放宽）/ `strict` / `comment`
 *   --required-mode <m> `strict`（默认，照抄 swagger required）/ `loose`（全部可选）
 *   --clean            生成前清空输出目录中的 *.ts（definitions/ 一并清空）
 *   --quiet            静默模式
 *
 * 详见 scripts/README.md。
 */

import fs from 'node:fs'
import path from 'node:path'
import url from 'node:url'
import process from 'node:process'

const HERE = path.dirname(url.fileURLToPath(import.meta.url))
const HTTP_METHODS = ['get', 'post', 'put', 'delete', 'patch', 'head', 'options']

/* ---------------------------------------------------------------------------
 * 1. 分组规则：把 swagger 的 tag / path 映射到现有前端控制器文件
 *
 * 现状（占位实现）把粗粒度的 tag（System 59 个、Setting 24 个）按资源拆成了
 * 多个控制器文件，前端 55 个页面文件依赖这套文件/类名。为了「页面代码只改方法名」
 * 这一硬性约束，默认 group-mode=path 会复刻这套拆分；--group-mode tag 则严格按
 * tag 分组，用于将来重构。
 * ------------------------------------------------------------------------- */

/** tag → 控制器类名（默认规则） */
const TAG_TO_CONTROLLER = {
  Alarm: 'AlarmController',
  Auth: 'AuthWebController',
  Basic: 'BasicController',
  Common: 'Common',
  Device: 'DeviceController',
  Inspect: 'InspectController',
  Profile: 'ProfileController',
  Risk: 'RiskController',
  Setting: 'SettingController',
  Stats: 'StatsController',
  Upgrade: 'UpgradeController',
}

/** tag + 路径前缀 → 控制器类名（优先于 TAG_TO_CONTROLLER） */
const PATH_TO_CONTROLLER = [
  { tag: 'System', prefix: /^\/web\/system\/configs(\/|$)/, controller: 'ConfigController' },
  { tag: 'System', prefix: /^\/web\/system\/depts(\/|$)/, controller: 'DeptController' },
  { tag: 'System', prefix: /^\/web\/system\/logs(\/|$)/, controller: 'OperationLogController' },
  { tag: 'System', prefix: /^\/web\/system\/menus(\/|$)/, controller: 'MenuController' },
  { tag: 'System', prefix: /^\/web\/system\/notifications(\/|$)/, controller: 'NotificationController' },
  { tag: 'System', prefix: /^\/web\/system\/posts(\/|$)/, controller: 'PostController' },
  { tag: 'System', prefix: /^\/web\/system\/roles(\/|$)/, controller: 'RoleController' },
  { tag: 'System', prefix: /^\/web\/system\/users(\/|$)/, controller: 'UserController' },
  { tag: 'Setting', prefix: /^\/web\/settings\/webhooks(\/|$)/, controller: 'WebhookController' },
  // 这几个 tag 下的非资源路径兜底
  { tag: 'System', prefix: /^\/web\/system\/[^/]+\/apis(\/|$)/, controller: 'MenuController' },
]

/** operationId 缺失时的兜底方法名（swagger 未写 @ID 的两个上传接口） */
const OPERATION_ID_FALLBACK = {
  'post /web/upload': 'uploadFile',
  'post /web/upload/audio': 'uploadAudio',
}

/* ---------------------------------------------------------------------------
 * 2. CLI 参数
 * ------------------------------------------------------------------------- */

function parseArgs(argv) {
  const opts = {
    input: '',
    out: '',
    baseUrl: 'import.meta.env.VITE_API_BASE',
    manifest: path.join(HERE, 'api-gen-preserve.mjs'),
    groupMode: 'path',
    enumMode: 'wide',
    requiredMode: 'strict',
    clean: false,
    quiet: false,
  }
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i]
    const next = () => argv[++i]
    switch (a) {
      case '--input': opts.input = next(); break
      case '--out': opts.out = next(); break
      case '--base-url': opts.baseUrl = next(); break
      case '--manifest': opts.manifest = next(); break
      case '--group-mode': opts.groupMode = next(); break
      case '--enum-mode': opts.enumMode = next(); break
      case '--required-mode': opts.requiredMode = next(); break
      case '--clean': opts.clean = true; break
      case '--quiet': opts.quiet = true; break
      case '-h':
      case '--help':
        console.log(fs.readFileSync(url.fileURLToPath(import.meta.url), 'utf8').split('*/')[0])
        process.exit(0)
        break
      default:
        throw new Error(`未知参数: ${a}`)
    }
  }
  if (!opts.input) throw new Error('缺少 --input <swagger.json>')
  if (!opts.out) throw new Error('缺少 --out <输出目录>')
  if (!['path', 'tag'].includes(opts.groupMode)) throw new Error(`--group-mode 只能是 path 或 tag，收到 ${opts.groupMode}`)
  if (!['wide', 'strict', 'comment'].includes(opts.enumMode)) throw new Error(`--enum-mode 只能是 wide / strict / comment，收到 ${opts.enumMode}`)
  if (!['strict', 'loose'].includes(opts.requiredMode)) throw new Error(`--required-mode 只能是 strict / loose，收到 ${opts.requiredMode}`)
  return opts
}

/* ---------------------------------------------------------------------------
 * 3. 类型映射（swagger schema → TypeScript 类型表达式）
 * ------------------------------------------------------------------------- */

/** 取 $ref / definition key 的短名（`#/definitions/model.User`、`model.User` → `User`） */
function refName(ref) {
  const last = String(ref).split('/').pop()
  return last.includes('.') ? last.split('.').pop() : last
}

function isIdentifier(name) {
  return /^[A-Za-z_$][A-Za-z0-9_$]*$/.test(name)
}

/** enum 生成策略：wide（默认）/ strict / comment，见 parseArgs 与 README */
let ENUM_MODE = 'wide'

/**
 * required 生成策略：strict（默认，照抄 swagger 的 required 数组）/ loose（全部可选）。
 * loose 用于「页面已按全可选类型编写」的存量项目，避免构造半成品 payload 时被迫填空值。
 */
let REQUIRED_MODE = 'strict'

/** 取某个 definition 的 required 列表（loose 模式恒为空） */
function requiredOf(def) {
  return REQUIRED_MODE === 'loose' ? [] : (def.required || [])
}

/**
 * swagger `enum` → TS 类型。
 * - wide（默认）  : `'a' | 'b' | (string & {})` —— 保留字面量提示/自动补全，同时允许后端新增取值
 *                   而不至于让前端页面（表单值常为 string）编译失败。
 * - strict        : `'a' | 'b'` —— 严格字面量联合，页面传入 string 时会报错。
 * - comment       : 退化为 string/number，字面量以注释列出。
 */
function enumType(values) {
  const allString = values.every((v) => typeof v === 'string')
  const base = allString ? 'string' : 'number'
  if (ENUM_MODE === 'comment') return base
  const literals = values.map((v) => (allString ? `'${String(v).replace(/'/g, "\\'")}'` : String(v))).join(' | ')
  if (ENUM_MODE === 'strict' || !allString) return literals
  return `${literals} | (${base} & {})`
}

/**
 * 把 swagger schema 映射为 TS 类型表达式。
 * @param {object} schema
 * @param {(name: string) => void} collectRef 收集需要 import 的类型名
 */
function tsType(schema, collectRef) {
  if (!schema || typeof schema !== 'object') return 'any'

  if (schema.$ref) {
    const n = refName(schema.$ref)
    collectRef(n)
    return n
  }

  // allOf: swagger2 里常见的「包装一个 $ref」写法
  if (Array.isArray(schema.allOf) && schema.allOf.length) {
    const parts = schema.allOf.map((s) => tsType(s, collectRef)).filter(Boolean)
    const uniq = [...new Set(parts)]
    return uniq.length ? uniq.join(' & ') : 'any'
  }

  if (Array.isArray(schema.enum) && schema.enum.length) {
    return enumType(schema.enum)
  }

  switch (schema.type) {
    case 'string':
      return 'string'
    case 'integer':
    case 'number':
    case 'float':
    case 'double':
      return 'number'
    case 'boolean':
      return 'boolean'
    case 'array': {
      const item = schema.items ? tsType(schema.items, collectRef) : 'any'
      // 联合类型数组需要加括号： ('a' | 'b')[]
      return /[|&]/.test(item) ? `(${item})[]` : `${item}[]`
    }
    case 'object':
    case undefined:
      if (schema.properties && Object.keys(schema.properties).length) {
        const body = Object.keys(schema.properties)
          .sort((a, b) => a.localeCompare(b))
          .map((k) => {
            const p = schema.properties[k]
            const opt = requiredOf(schema).includes(k) ? '' : '?'
            return `${k}${opt}: ${tsType(p, collectRef)}`
          })
          .join('; ')
        return `{ ${body} }`
      }
      // 无 properties 的 object（或完全空的 schema）→ 宽松类型
      if (schema.type === 'object' || Object.keys(schema).length === 0) return schema.type === 'object' ? 'Record<string, any>' : 'any'
      return 'any'
    default:
      return 'any'
  }
}

/* ---------------------------------------------------------------------------
 * 4. 从 swagger 提取 operation 列表
 * ------------------------------------------------------------------------- */

function sanitizeIdentifier(name) {
  let out = String(name).replace(/_\d+$/, '')          // 与 swagger2-doc-parser 一致：去掉 @ID 的 _2 后缀
  out = out.replace(/[^A-Za-z0-9_$]/g, '_')
  if (!/^[A-Za-z_$]/.test(out)) out = `_${out}`
  return out
}

function fallbackOperationId(method, p) {
  const key = `${method.toLowerCase()} ${p}`
  if (OPERATION_ID_FALLBACK[key]) return OPERATION_ID_FALLBACK[key]
  const segs = p.replace(/^\/web\//, '').split('/').filter((s) => s && !s.startsWith('{'))
  return method.toLowerCase() + segs.map((s) => s.charAt(0).toUpperCase() + s.slice(1).replace(/[^A-Za-z0-9]/g, '')).join('')
}

function controllerFor(tag, p, groupMode) {
  if (groupMode === 'tag') {
    if (TAG_TO_CONTROLLER[tag]) return TAG_TO_CONTROLLER[tag]
    return `${tag}Controller`
  }
  for (const rule of PATH_TO_CONTROLLER) {
    if (rule.tag === tag && rule.prefix.test(p)) return rule.controller
  }
  if (TAG_TO_CONTROLLER[tag]) return TAG_TO_CONTROLLER[tag]
  return `${tag}Controller`
}

function collectOperations(swagger, groupMode) {
  const ops = []
  for (const [p, item] of Object.entries(swagger.paths || {})) {
    for (const m of Object.keys(item)) {
      if (!HTTP_METHODS.includes(m)) continue
      const op = item[m] || {}
      const tag = (op.tags && op.tags[0]) || 'Default'
      const operationId = op.operationId ? sanitizeIdentifier(op.operationId) : fallbackOperationId(m, p)
      ops.push({
        path: p,
        method: m,
        rawMethod: m.toUpperCase(),
        operationId,
        tag,
        controller: controllerFor(tag, p, groupMode),
        summary: op.summary || op.description || '',
        description: op.description || '',
        parameters: op.parameters || [],
        responses: op.responses || {},
      })
    }
  }
  // 稳定排序：先控制器，再路径，再方法
  const order = { get: 0, post: 1, put: 2, patch: 3, delete: 4 }
  ops.sort((a, b) =>
    a.controller.localeCompare(b.controller) ||
    a.path.localeCompare(b.path) ||
    (order[a.method] ?? 9) - (order[b.method] ?? 9))
  return ops
}

/** 取第一个 2xx 响应 */
function successResponse(op) {
  const codes = Object.keys(op.responses).filter((c) => /^2\d\d$/.test(c)).sort()
  return codes.length ? op.responses[codes[0]] : undefined
}

/* ---------------------------------------------------------------------------
 * 5. 生成 Controller 源码
 * ------------------------------------------------------------------------- */

/**
 * 按 `paramOrder` 重排 query 参数：清单里声明的名字优先并保持给定顺序，
 * 其余按 swagger 原顺序追加。用于避免「位置传参」的既有调用点被静默错位
 * （例如 pageUsers 的 dept_id / active 顺序）。
 */
function applyParamOrder(params, order) {
  if (!Array.isArray(order) || !order.length) return params
  const byName = new Map(params.map((p) => [p.name, p]))
  const head = order.map((n) => byName.get(n)).filter(Boolean)
  const headNames = new Set(head.map((p) => p.name))
  return [...head, ...params.filter((p) => !headNames.has(p.name))]
}

function buildMethod(op, baseUrl, collectRef, paramOrder) {
  const queryParams = applyParamOrder(op.parameters.filter((x) => x.in === 'query'), paramOrder)
  const bodyParam = op.parameters.find((x) => x.in === 'body')
  const formParams = op.parameters.filter((x) => x.in === 'formData')

  // ── 路径模板 + 参数表 ──────────────────────────────────────────────
  const args = []
  let url = op.path.replace(/\{([^}]+)\}/g, (_all, name) => {
    args.push(`${name}: string | number`)
    return `\${${name}}`
  })

  const queryString = []
  for (const q of queryParams) {
    const optional = q.required ? '' : '?'
    args.push(`${q.name}${optional}: ${tsType({ type: q.type, items: q.items }, collectRef)}`)
    queryString.push(`${q.name}=\${${q.name} ?? "" }`)
  }
  if (queryString.length) url += `?${queryString.join('&')}`

  let dataLine = ''
  if (bodyParam) {
    const t = bodyParam.schema ? tsType(bodyParam.schema, collectRef) : 'any'
    args.push(`data${bodyParam.required ? '' : '?'}: ${t}`)
    dataLine = 'data: JSON.stringify(data),'
  } else if (formParams.length) {
    // 文件上传等 multipart 接口：保留 data 形参，由调用方传 FormData
    args.push('data?: any')
    dataLine = 'data: data,'
  } else if (['post', 'put', 'patch'].includes(op.method)) {
    dataLine = 'data: null,'
  }
  args.push('headers?: any')

  const respSchema = successResponse(op)
  const respType = respSchema && respSchema.schema ? tsType(respSchema.schema, collectRef) : 'any'

  const summary = op.summary ? `    /// ${op.summary}` : ''

  return [
    `    static ${op.operationId}Path: string = "${op.path}";`,
    summary,
    `    static async ${op.operationId}(${args.join(', ')}): Promise<${respType}> {`,
    `      return request(\`${url}\`, {`,
    `        baseURL: ${baseUrl},`,
    `        method: "${op.rawMethod}",`,
    dataLine ? `        ${dataLine}` : null,
    `        headers,`,
    `      });`,
    `    }`,
  ].filter((l) => l !== null).join('\n')
}

const AUTO_HEADER = [
  '/* eslint-disable */',
  '// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），',
  '// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。',
  '// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。',
]

/** swagger 里所有 definition 的短名（用于给手写源码自动补类型 import） */
let DEFINITION_NAMES = new Set()

/** preserve 清单的失效/冲突提示，生成结束后统一打印 */
const WARNINGS = []

function generateController(controller, ops, opts, manifest) {
  const refs = new Set()
  const collectRef = (n) => refs.add(n)

  const preserved = new Set(Object.keys(manifest.methods || {}))     // 生成器跳过、改用清单源码的 operationId
  const generated = ops.filter((o) => !preserved.has(o.operationId))

  const methodSources = generated.map((op) => buildMethod(op, opts.baseUrl, collectRef, (manifest.paramOrder || {})[op.operationId]))

  // 清单里登记的手写方法（生成器无法表达的签名/返回类型）
  const manual = []
  const consumed = new Set()
  for (const op of ops) {
    const src = manifest.methods && manifest.methods[op.operationId]
    if (src) {
      manual.push(src)
      consumed.add(op.operationId)
    }
  }

  // 失效检查：methods 键必须对应真实 operationId（后端补注解后应改回生成）
  for (const key of Object.keys(manifest.methods || {})) {
    if (!consumed.has(key)) {
      WARNINGS.push(`${controller}: preserve 清单 methods.${key} 找不到对应的 swagger operationId —— 已忽略，请清理该条`)
    }
  }

  // 冲突检查：extraMethods 不能和生成方法重名（否则产生重复标识符）
  const declared = new Set(generated.map((o) => o.operationId))
  for (const src of manifest.extraMethods || []) {
    const names = [...src.matchAll(/static\s+(?:async\s+)?(\w+)\s*\(/g)].map((m) => m[1])
    const clash = names.find((n) => declared.has(n))
    if (clash) {
      WARNINGS.push(`${controller}: preserve 清单 extraMethods.${clash} 与 swagger 生成方法重名 —— 已跳该手写实现，改用生成结果`)
      continue
    }
    manual.push(src)
  }

  // 保持与 swagger 一致的顺序：生成的方法按 op 顺序，手写方法排在最后
  const body = [...methodSources, ...manual].join('\n\n')

  // 手写源码里引用的类型自动补 import（避免清单里重复列一遍类型名）
  const manualText = [...manual, ...(manifest.fileBody || [])].join('\n')
  for (const name of DEFINITION_NAMES) {
    if (refs.has(name)) continue
    if (new RegExp(`\\b${name}\\b`).test(manualText)) refs.add(name)
  }

  const imports = [...refs].filter(isIdentifier).sort().map((n) => `import type { ${n} } from './definitions/${n}';`)
  for (const extra of manifest.imports || []) imports.push(extra)

  const lines = [...AUTO_HEADER]
  for (const note of manifest.headerNotes || []) lines.push(note)
  lines.push("import request from '../request';")
  if (imports.length) lines.push(...imports)
  if (manifest.fileBody && manifest.fileBody.length) {
    lines.push('')
    lines.push(...manifest.fileBody)
  }
  lines.push('')
  lines.push(`export class ${controller} {`)
  lines.push('')
  lines.push(body)
  lines.push('}')
  lines.push('')
  return lines.join('\n')
}

/* ---------------------------------------------------------------------------
 * 6. 生成 definitions
 * ------------------------------------------------------------------------- */

function generateDefinition(name, def, allNames) {
  const refs = new Set()
  const collectRef = (n) => refs.add(n)

  if (def.enum && def.enum.length) {
    // 顶层枚举 → 字面量联合类型
    const union = enumType(def.enum)
    return [
      '/* eslint-disable prettier/prettier */',
      `// ${name}`,
      `export type ${name} = ${union}`,
      '',
    ].join('\n')
  }

  const props = []
  const required = requiredOf(def)
  for (const key of Object.keys(def.properties || {}).sort((a, b) => a.localeCompare(b))) {
    const p = def.properties[key]
    const optional = required.includes(key) ? '' : '?'
    const notes = []
    if (p.description) notes.push(`\t/** ${String(p.description).replace(/\*\//g, '*\\/').replace(/\s*\n\s*/g, ' ')} */`)
    if (p.enum && p.enum.length) notes.push(`\t/** 枚举值: ${p.enum.join(' / ')} */`)
    props.push([...notes, `\t${key}${optional}: ${tsType(p, collectRef)};`].join('\n'))
  }

  const imports = [...refs]
    .filter((n) => isIdentifier(n) && n !== name && allNames.has(n))
    .sort()
    .map((n) => `import type { ${n} } from './${n}';`)

  return [
    '/* eslint-disable prettier/prettier */',
    '// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。',
    "import type { Definition } from './Definition';",
    ...imports,
    '',
    `export type ${name} = Definition & {`,
    props.join('\n'),
    '}',
    '',
  ].join('\n')
}

/**
 * 「查询对象」类型：swagger 里 GET 的 query 参数是散开的，没有对应的 definition，
 * 但页面习惯用一个对象传参（`pageAlarms(query)`）。清单里的 queryDefinitions
 * 声明 `{ 类型名: operationId }`，由该 operation 的 query 参数反向生成一个
 * definitions/{类型名}.ts，保证页面 import 得到、字段与后端一致。
 */
function generateQueryDefinition(name, op, allNames) {
  const refs = new Set()
  const collectRef = (n) => refs.add(n)
  const props = op.parameters
    .filter((x) => x.in === 'query')
    .map((q) => {
      const optional = q.required ? '' : '?'
      return `\t${q.name}${optional}: ${tsType({ type: q.type, items: q.items, enum: q.enum }, collectRef)};`
    })
  const imports = [...refs]
    .filter((n) => isIdentifier(n) && n !== name && allNames.has(n))
    .sort()
    .map((n) => `import type { ${n} } from './${n}';`)

  return [
    '/* eslint-disable prettier/prettier */',
    '// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。',
    `// 由 ${op.rawMethod} ${op.path} 的 query 参数生成（页面以查询对象传参）。`,
    "import type { Definition } from './Definition';",
    ...imports,
    '',
    `export type ${name} = Definition & {`,
    props.join('\n'),
    '}',
    '',
  ].join('\n')
}

/* ---------------------------------------------------------------------------
 * 7. main
 * ------------------------------------------------------------------------- */

/** 读取 preserve 清单（ESM 模块，默认导出普通对象） */
async function loadManifestAsync(p) {
  if (!fs.existsSync(p)) return {}
  const mod = await import(url.pathToFileURL(p).href)
  return mod.default || mod || {}
}

async function main() {
  const opts = parseArgs(process.argv.slice(2))
  const log = (...a) => { if (!opts.quiet) console.log(...a) }

  const swaggerPath = path.resolve(opts.input)
  if (!fs.existsSync(swaggerPath)) throw new Error(`swagger 文件不存在: ${swaggerPath}`)
  const swagger = JSON.parse(fs.readFileSync(swaggerPath, 'utf8'))
  if (!swagger.paths) throw new Error(`不是合法的 swagger 2.0 文档（缺少 paths）: ${swaggerPath}`)

  const manifest = await loadManifestAsync(path.resolve(opts.manifest))
  const controllers = manifest.controllers || {}
  ENUM_MODE = opts.enumMode
  REQUIRED_MODE = opts.requiredMode

  const ops = collectOperations(swagger, opts.groupMode)
  const outDir = path.resolve(opts.out)
  const defsDir = path.join(outDir, 'definitions')

  if (opts.clean && fs.existsSync(outDir)) {
    for (const f of fs.readdirSync(outDir)) {
      if (f.endsWith('.ts')) fs.rmSync(path.join(outDir, f), { force: true })
    }
    if (fs.existsSync(defsDir)) {
      for (const f of fs.readdirSync(defsDir)) {
        if (f.endsWith('.ts')) fs.rmSync(path.join(defsDir, f), { force: true })
      }
    }
  }
  fs.mkdirSync(defsDir, { recursive: true })

  // ── definitions ──
  const allNames = new Set(Object.keys(swagger.definitions || {}).map(refName))
  DEFINITION_NAMES = allNames
  fs.writeFileSync(path.join(defsDir, 'Definition.ts'), 'export type Definition = Record<string, any>\n', 'utf8')
  let defCount = 0
  for (const [rawName, def] of Object.entries(swagger.definitions || {})) {
    const name = refName(rawName)
    if (!isIdentifier(name) || name === 'Definition') continue
    fs.writeFileSync(path.join(defsDir, `${name}.ts`), generateDefinition(name, def, allNames), 'utf8')
    defCount++
  }

  // 清单声明的「查询对象」类型（swagger 无对应 definition）
  const opById = new Map(ops.map((o) => [o.operationId, o]))
  for (const [name, operationId] of Object.entries(manifest.queryDefinitions || {})) {
    if (!isIdentifier(name)) throw new Error(`queryDefinitions 类型名非法: ${name}`)
    const op = opById.get(operationId)
    if (!op) throw new Error(`queryDefinitions.${name} 指向的 operationId 不存在: ${operationId}`)
    allNames.add(name)
    DEFINITION_NAMES = allNames
    fs.writeFileSync(path.join(defsDir, `${name}.ts`), generateQueryDefinition(name, op, allNames), 'utf8')
    defCount++
  }

  // ── controllers ──
  const byController = new Map()
  for (const op of ops) {
    if (!byController.has(op.controller)) byController.set(op.controller, [])
    byController.get(op.controller).push(op)
  }
  // 清单里声明了 controller 但 swagger 没有对应 operation 的（纯别名文件）
  for (const name of Object.keys(controllers)) {
    if (!byController.has(name)) byController.set(name, [])
  }

  const written = []
  for (const [controller, list] of [...byController.entries()].sort((a, b) => a[0].localeCompare(b[0]))) {
    if (!/^[A-Za-z_$][A-Za-z0-9_$]*$/.test(controller)) throw new Error(`非法控制器名: ${controller}`)
    const src = generateController(controller, list, opts, controllers[controller] || {})
    fs.writeFileSync(path.join(outDir, `${controller}.ts`), src, 'utf8')
    written.push({ controller, count: list.length })
  }

  log(`  源文档   : ${path.relative(process.cwd(), swaggerPath)}`)
  log(`  输出目录 : ${path.relative(process.cwd(), outDir)}`)
  log(`  分组模式 : ${opts.groupMode}`)
  log(`  基址表达式: ${opts.baseUrl}`)
  log(`  ✓ ${written.length} 个 Controller / ${ops.length} 个 operation`)
  log(`  ✓ ${defCount} 个 definition`)
  for (const w of written) log(`      - ${w.controller}.ts (${w.count})`)
  if (WARNINGS.length) {
    console.warn('')
    console.warn(`  ⚠ preserve 清单有 ${WARNINGS.length} 条提示:`)
    for (const w of WARNINGS) console.warn(`      - ${w}`)
  }
}

main().catch((err) => {
  console.error(`\n生成失败: ${err.message}`)
  process.exit(1)
})
