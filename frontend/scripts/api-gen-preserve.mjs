/**
 * =============================================================================
 * api-gen-preserve.mjs —— 生成器的 preserve 清单（手写逻辑白名单）
 * =============================================================================
 *
 * 背景：`src/admin/api/generated/**` 里有一部分方法/类型是 swagger 无法表达的
 * 手写逻辑，例如：
 *   - 以「查询对象」而非位置参数传参的列表接口（`pageAlarms(query)`）
 *   - 返回 Blob 流 + `responseType: 'blob'` 的导出接口（`exportLogs`）
 *   - 后端路由存在但没有 swagger 注解的方法（`markInspectTaskRead`）
 *   - 为兼容公共组件命名而保留的别名（`options()` / `tree()` / `getCurrentUserMenus()`）
 *   - 页面直接 import 的辅助类型（`OperationLogQuery` / `DashboardData`）
 *
 * 这些内容登记在本文件里，生成器会：
 *   1. `methods[operationId]` —— 用清单源码**替换**同名生成方法（并自动补齐类型 import）
 *   2. `extraMethods[]`       —— 追加生成器不会产出的纯手写方法
 *   3. `fileBody[]`           —— 追加文件级声明（helper 函数、辅助类型）
 *   4. `imports[]`            —— 追加显式 import（一般不必写，会自动探测）
 *
 * 修改本文件等同于修改生成结果，请同步更新 scripts/README.md。
 */

/** 列表接口通用查询串拼装：跳过 undefined / null / 空串，避免后端把空值当成有效过滤条件 */
const QUERY_HELPER = `/**
 * 拼装查询串：跳过 undefined / null / 空串，避免后端把空值当作有效过滤条件。
 * 【手动保留，生成器不覆盖】swagger 无法表达「查询对象传参」这一约定，
 * 故 pageAlarms / pageRisks / pageInspectTasks / pageInspectRecords / riskStats /
 * alarmStats 等接口保持既有签名，避免改动 55 个页面文件的调用点。
 */
function q(params: Record<string, unknown>): string {
  const parts: string[] = [];
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null || v === '') continue
    parts.push(\`\${encodeURIComponent(k)}=\${encodeURIComponent(String(v))}\`)
  }
  return parts.length ? \`?\${parts.join('&')}\` : ''
}`

/** 手动保留的方法统一加这个前缀注释，便于 review 时快速定位 */
const KEEP = (reason) => `    // 【手动保留，生成器不覆盖】${reason}\n`

export default {
  /**
   * 「查询对象」类型：swagger 的 GET query 参数是散开的，没有 definition，
   * 但页面用对象传参。这里声明 `{ 类型名: operationId }`，生成器按该操作的
   * query 参数反向生成 definitions/{类型名}.ts。
   */
  queryDefinitions: {
    PageAlarmReq: 'pageAlarms',
    PageRiskReq: 'pageRisks',
    PageInspectTaskReq: 'pageInspectTasks',
    PageInspectRecordReq: 'pageInspectRecords',
    StatsQueryReq: 'riskStats',
  },

  controllers: {
    /* ── 告警：列表按查询对象传参 ───────────────────────────────── */
    AlarmController: {
      fileBody: [QUERY_HELPER],
      methods: {
        pageAlarms: `${KEEP('查询对象传参，AlarmList 传入 PageAlarmReq')}    static async pageAlarms(params?: PageAlarmReq, headers?: any): Promise<RPageAlarmRecord> {
      return request(\`/web/alarms\${q({ ...(params || {}) })}\`, {
        baseURL: import.meta.env.VITE_API_BASE, method: "GET", headers,
      })
    }`,
      },
    },

    /* ── 风险：列表按查询对象传参 ───────────────────────────────── */
    RiskController: {
      fileBody: [QUERY_HELPER],
      methods: {
        pageRisks: `${KEEP('查询对象传参，4 个页面传入 PageRiskReq')}    static async pageRisks(params?: PageRiskReq, headers?: any): Promise<RPageRiskRecord> {
      return request(\`/web/risks\${q({ ...(params || {}) })}\`, {
        baseURL: import.meta.env.VITE_API_BASE, method: "GET", headers,
      })
    }`,
      },
    },

    /* ── 巡检：列表按查询对象传参 + 一个无 swagger 注解的路由 ───── */
    InspectController: {
      fileBody: [QUERY_HELPER],
      methods: {
        pageInspectTasks: `${KEEP('查询对象传参，InspectTaskList 传入 PageInspectTaskReq')}    static async pageInspectTasks(params?: PageInspectTaskReq, headers?: any): Promise<RPageInspectTask> {
      return request(\`/web/inspects/tasks\${q({ ...(params || {}) })}\`, {
        baseURL: import.meta.env.VITE_API_BASE, method: "GET", headers,
      })
    }`,
        pageInspectRecords: `${KEEP('查询对象传参，InspectRecordList 传入 PageInspectRecordReq')}    static async pageInspectRecords(params?: PageInspectRecordReq, headers?: any): Promise<RPageInspectRecord> {
      return request(\`/web/inspects/records\${q({ ...(params || {}) })}\`, {
        baseURL: import.meta.env.VITE_API_BASE, method: "GET", headers,
      })
    }`,
      },
      // 注：`markInspectTaskRead`（PUT /web/inspects/tasks/{id}/read）曾因后端漏写 swagger
      // 注解而在此登记为手写方法；后端补上注解后已由生成器正常产出，此条已移除。
    },

    /* ── 统计：两个查询对象传参接口 + 页面引用的 DashboardData ─── */
    StatsController: {
      fileBody: [
        QUERY_HELPER,
        `/** 首页看板数据类型别名（Dashboard.tsx 直接 import 这个名字） */
export type DashboardData = DashboardResp`,
      ],
      methods: {
        riskStats: `${KEEP('查询对象传参，RiskStats / ScreenPage 传入 StatsQueryReq')}    static async riskStats(params?: StatsQueryReq, headers?: any): Promise<RRiskStats> {
      return request(\`/web/stats/risk\${q({ ...(params || {}) })}\`, {
        baseURL: import.meta.env.VITE_API_BASE, method: "GET", headers,
      })
    }`,
        alarmStats: `${KEEP('查询对象传参，AlarmStats 传入 StatsQueryReq')}    static async alarmStats(params?: StatsQueryReq, headers?: any): Promise<RAlarmStats> {
      return request(\`/web/stats/alarm\${q({ ...(params || {}) })}\`, {
        baseURL: import.meta.env.VITE_API_BASE, method: "GET", headers,
      })
    }`,
      },
    },

    /* ── 操作日志：查询对象 + Blob 导出 + 页面引用的 OperationLogQuery ── */
    OperationLogController: {
      fileBody: [
        `/** 操作日志查询参数（OperationLogList 直接 import 这个名字） */
export interface OperationLogQuery {
    module?: string;
    operator?: string;
    action?: string;
    /** 开始时间，RFC3339 或 yyyy-MM-dd HH:mm:ss */
    start_dt?: string;
    /** 结束时间 */
    end_dt?: string;
}

/** 拼接操作日志查询串（导出与列表共用） */
function buildLogQuery(query?: OperationLogQuery): string {
    return \`module=\${query?.module ?? "" }&operator=\${query?.operator ?? "" }&action=\${query?.action ?? "" }&start_dt=\${query?.start_dt ?? "" }&end_dt=\${query?.end_dt ?? "" }\`;
}`,
      ],
      methods: {
        pageOperationLogs: `${KEEP('第 3 个参数是查询对象，OperationLogList 传入 buildQuery()')}    static async pageOperationLogs(page: number, page_size: number, query?: OperationLogQuery, headers?: any): Promise<RPageOperationLog> {
      return request(\`/web/system/logs?page=\${page ?? "" }&page_size=\${page_size ?? "" }&\${buildLogQuery(query)}\`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }`,
        exportOperationLogs: `${KEEP('返回 CSV 文件流：swagger 只声明 string，实际需要 responseType=blob')}    static async exportOperationLogs(query?: OperationLogQuery, headers?: any): Promise<Blob> {
      return request(\`/web/system/logs/export?\${buildLogQuery(query)}\`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        responseType: "blob",
        headers,
      });
    }`,
      },
      extraMethods: [
        `${KEEP('纯前端拼接导出地址（window.open 场景不带鉴权头），后端无对应 operation')}    static buildExportLogsUrl(query?: OperationLogQuery): string {
      const base = import.meta.env.VITE_API_BASE || "";
      return \`\${base}/web/system/logs/export?\${buildLogQuery(query)}\`;
    }`,
      ],
    },

    /* ── 以下均为「别名」：公共组件 / 页面依赖的旧命名，保留以兼容调用点 ── */

    UserController: {
      // 位置传参保护：UserList.tsx 调用 pageUsers(page, pageSize, keyword, deptId, activeFilter)，
      // swagger 的 query 顺序是 active 在前、dept_id 在后，直接用会静默错位。
      paramOrder: {
        pageUsers: ['page', 'page_size', 'keyword', 'dept_id', 'active', 'post_id'],
      },
      extraMethods: [
        `${KEEP('UserSelect 依赖 options() 命名，转发到 userOptions()')}    /// 用户下拉选项别名（与 userOptions 同一端点）
    static async options(headers?: any): Promise<ROptionList> {
      return UserController.userOptions(headers);
    }`,
      ],
    },
    PostController: {
      extraMethods: [
        `${KEEP('3 个页面依赖 options() 命名，转发到 postOptions()')}    /// 岗位下拉选项别名（与 postOptions 同一端点）
    static async options(headers?: any): Promise<ROptionList> {
      return PostController.postOptions(headers);
    }`,
      ],
    },
    DeptController: {
      extraMethods: [
        `${KEEP('DeptSelect 依赖 options() 命名，转发到 deptOptions()')}    /// 部门下拉选项别名（与 deptOptions 同一端点）
    static async options(headers?: any): Promise<ROptionList> {
      return DeptController.deptOptions(headers);
    }`,
        `${KEEP('DeptTree / RoleList 依赖 tree() 命名，转发到 treeDepts()')}    /// 部门树别名（与 treeDepts 同一端点）
    static async tree(headers?: any): Promise<RDeptTree> {
      return DeptController.treeDepts(headers);
    }`,
      ],
    },
    RoleController: {
      extraMethods: [
        `${KEEP('RoleList / UserList 依赖 options() 命名，转发到 roleOptions()')}    /// 角色下拉选项别名（与 roleOptions 同一端点）
    static async options(headers?: any): Promise<ROptionList> {
      return RoleController.roleOptions(headers);
    }`,
      ],
    },
    MenuController: {
      extraMethods: [
        `${KEEP('页面依赖 tree() 命名，转发到 treeMenus()')}    /// 菜单树别名（与 treeMenus 同一端点）
    static async tree(platform?: string, headers?: any): Promise<RMenuList> {
      return MenuController.treeMenus(platform, headers);
    }`,
      ],
    },

    /* ── 纯别名控制器：swagger 里没有独立 operation，只有转发方法 ── */
    RegionController: {
      headerNotes: ['// 【手动保留，生成器不覆盖】本文件只有从 BasicController 转发的兼容别名。'],
      extraMethods: [
        `${KEEP('RegionSelect 依赖 RegionController.options()，转发到 BasicController.regionOptions()')}    static optionsPath: string = '/web/basic/regions/options'

    /// 区域下拉选项
    static async options(): Promise<ROptionList> {
      return BasicController.regionOptions()
    }`,
      ],
      imports: ["import { BasicController } from './BasicController';"],
    },
    RiskTypeController: {
      headerNotes: ['// 【手动保留，生成器不覆盖】本文件只有从 BasicController 转发的兼容别名。'],
      extraMethods: [
        `${KEEP('RiskTypeSelect 依赖 RiskTypeController.options()，转发到 BasicController.riskTypeOptions()')}    static optionsPath: string = '/web/basic/risk-types/options'

    /// 风险类型下拉选项
    static async options(): Promise<ROptionList> {
      return BasicController.riskTypeOptions()
    }`,
      ],
      imports: ["import { BasicController } from './BasicController';"],
    },
    AlarmTypeController: {
      headerNotes: ['// 【手动保留，生成器不覆盖】本文件只有从 BasicController 转发的兼容别名。'],
      extraMethods: [
        `${KEEP('AlarmTypeSelect 依赖 AlarmTypeController.options()，转发到 BasicController.alarmTypeOptions()')}    static optionsPath: string = '/web/basic/alarm-types/options'

    /// 告警类型下拉选项
    static async options(): Promise<ROptionList> {
      return BasicController.alarmTypeOptions()
    }`,
      ],
      imports: ["import { BasicController } from './BasicController';"],
    },

    /* ── 个人中心：MainLayout 依赖的兼容别名 ─────────────────────── */
    ProfileController: {
      extraMethods: [
        `${KEEP('MainLayout 侧边栏依赖 getCurrentUserMenus() 命名，转发到 getProfileMenus()（GET /web/profile/menus）')}    /// 菜单树别名（与 getProfileMenus 同一端点）
    static async getCurrentUserMenus(headers?: any): Promise<RMenuList> {
      return ProfileController.getProfileMenus(headers)
    }`,
      ],
    },
  },
}
