# frontend/scripts —— 前端 API 代码生成

`src/admin/api/generated/**` 是**自动生成**的目录（Controller + definitions），
请勿手工修改；需要保留的手写逻辑登记在 `api-gen-preserve.mjs`。

## 快速开始

```bash
cd frontend

# 后端默认 8080
bash scripts/api-gen.sh

# 测试环境（config-test.yaml，监听 8081）
BACKEND_URL=http://localhost:8081 bash scripts/api-gen.sh

# 离线 / CI：不做 HTTP 拉取，直接用仓库里的 swagger JSON
SWAGGER_FILE=../backend/docs/web/web_swagger.json bash scripts/api-gen.sh
```

生成后自检：

```bash
npx tsc --noEmit && npx vitest run && npm run build
```

## 链路

```
后端 swaggo 注解（internal/api/handler/web/*.go）
  → cd backend && bash scripts/gen-swagger.sh
  → backend/docs/web/web_swagger.json          （后端运行时: GET /web/api-docs.json）
  → cd frontend && bash scripts/api-gen.sh
  → src/admin/api/generated/*.ts + definitions/*.ts
```

`api-gen.sh` 会自动选择生成器：

| 情况 | 生成器 |
|---|---|
| PATH 中存在 `swagger2-doc-parser` | 外部 CLI（`scripts/swagger-config-typescript.js` 配置） |
| 否则 | **回退** 到本仓库自带的 `node scripts/api-gen-local.mjs` |

> 背景：`swagger2-doc-parser` 是一个外部 CLI，开发机/CI 上通常不存在。
> 本地生成器是零依赖（仅用 Node 内置模块）的等价实现，保证整条链路可跑、可复现。

## 本地生成器 `api-gen-local.mjs`

```
node scripts/api-gen-local.mjs --input <swagger.json> --out <输出目录> [选项]
```

| 选项 | 默认 | 说明 |
|---|---|---|
| `--input <path>` | 必填 | swagger 2.0 JSON 文件 |
| `--out <dir>` | 必填 | 输出目录（Controller 与 `definitions/` 都写到这里） |
| `--base-url <expr>` | `import.meta.env.VITE_API_BASE` | 请求基址表达式 |
| `--manifest <path>` | `scripts/api-gen-preserve.mjs` | preserve 清单 |
| `--group-mode <m>` | `path` | `path`：按 tag + 路径前缀细分到现有控制器文件（默认，与页面调用点一致）<br>`tag`：严格按 swagger tag 分组，一个 tag 一个 Controller |
| `--enum-mode <m>` | `wide` | `wide`：`'a' \| 'b' \| (string & {})`，保留字面量提示又能接纳后端新增取值<br>`strict`：`'a' \| 'b'`（页面传 `string` 会报错）<br>`comment`：退化为 `string`，字面量写在注释里 |
| `--required-mode <m>` | `strict` | `strict`：照抄 swagger 的 `required`<br>`loose`：全部字段可选（页面大量构造半成品 payload 时更省事） |
| `--clean` | 关 | 生成前清空输出目录里的 `*.ts` |
| `--quiet` | 关 | 静默 |

### 生成规则

- **按 tag 分组**（`--group-mode=path` 时再按路径前缀细分，复刻现有 23 个控制器文件），
  静态类风格与 `swagger2-doc-parser` 一致：
  ```ts
  export class UserController {
    static pageUsersPath: string = "/web/system/users";
    /// 用户分页列表
    static async pageUsers(page?: number, page_size?: number, ..., headers?: any): Promise<RPageUser> {
      return request(`/web/system/users?page=${page ?? "" }&...`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }
  }
  ```
- 方法名 = swagger `operationId`（后端 `@ID`），并去掉 `swagger2-doc-parser` 约定的 `_N` 后缀
- `query` 参数按 `parameters(in:query)` 顺序逐个拼进查询串，未传值用 `?? ""`
- `path` 参数用模板字符串插值，类型为 `string | number`（页面既有传 `String(id)` 也有传裸 `number`）
- `body` 参数用 `data: JSON.stringify(data)`；无 body 的 POST/PUT/PATCH 生成 `data: null`
- 每个方法上方输出 swagger `summary` 作为 `///` 注释
- definitions：一个 definition 一个文件，`export type X = Definition & { ... }`，
  字段名与可选性照抄 swagger；类型映射
  `integer/number→number`、`string→string`、`boolean→boolean`、`array→T[]`、`$ref→类型名`、`object→Record<string, any>`、无类型→`any`
- 生成物中**不允许出现 `as any`**（当前为 0 处）

## preserve 清单 `api-gen-preserve.mjs`

生成器无法表达的**手写逻辑**登记在这里，生成时会原样保留：

| 键 | 作用 |
|---|---|
| `queryDefinitions` | `{ 类型名: operationId }`，按该操作的 query 参数生成 `definitions/{类型名}.ts`（swagger 里 GET 的 query 是散开的，没有 definition，但页面用对象传参） |
| `controllers.<Ctrl>.methods.<operationId>` | 用清单源码**替换**同名生成方法（如「查询对象传参」的列表接口、返回 `Blob` 的导出接口） |
| `controllers.<Ctrl>.extraMethods[]` | 追加生成器不会产出的方法（兼容别名 `options()` / `tree()` / `getCurrentUserMenus()`、无 swagger 注解的路由） |
| `controllers.<Ctrl>.fileBody[]` | 追加文件级声明（`q()` helper、`OperationLogQuery`、`DashboardData`） |
| `controllers.<Ctrl>.paramOrder.<operationId>` | 重排 query 参数顺序，保护既有「位置传参」调用点不被静默错位 |
| `controllers.<Ctrl>.imports[]` / `headerNotes[]` | 追加显式 import / 文件头说明 |

手写源码里引用到的类型会在生成时自动补 `import type`，无需在清单里重复列。

### 当前登记的手写逻辑

- **查询对象传参**（7 个）：`pageAlarms` / `pageRisks` / `pageInspectTasks` / `pageInspectRecords` /
  `riskStats` / `alarmStats` / `pageOperationLogs`（沿用既有签名，避免改动 55 个页面文件的调用点）
- **Blob 导出**（1 个）：`exportOperationLogs`（swagger 只声明 `string`，实际需要 `responseType: 'blob'`）
- **兼容别名**（7 个方法 + 3 个纯别名文件）：`UserController.options` / `PostController.options` /
  `DeptController.options` / `DeptController.tree` / `RoleController.options` / `MenuController.tree` /
  `ProfileController.getCurrentUserMenus`，以及 `RegionController` / `RiskTypeController` / `AlarmTypeController`
- **页面直接 import 的辅助类型**（2 个）：`OperationLogQuery`、`DashboardData`
- **参数顺序保护**（1 个）：`UserController.pageUsers`（`dept_id` 在 `active` 之前）
- **查询对象类型**（5 个）：`PageAlarmReq` / `PageRiskReq` / `PageInspectTaskReq` /
  `PageInspectRecordReq` / `StatsQueryReq`

### 清单健康检查

生成器会检查 preserve 清单是否失效并打印 `⚠` 提示（不会静默出错）：

- `methods.<operationId>` 找不到对应 operation（后端补了 swagger 注解后就该删掉这条）
- `extraMethods` 与生成方法重名（此时跳用手写实现、改用生成结果，避免重复标识符）

历史示例：`InspectController.markInspectTaskRead` 曾因后端漏写 swagger 注解登记为手写方法，
后端补上注解后该条已从清单移除，改由生成器产出。

## App 端（Flutter/Dart）

未实现。App 端 swagger 在 `backend/docs/app/app_swagger.json`，目标输出 `app/lib/services/generated/`。
后续建议比照 Web 端新增 `scripts/api-gen-local-dart.mjs`，并在 `api-gen.sh` 末尾追加调用
（脚本里已留 TODO 注释）。
