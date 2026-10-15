/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { DashboardResp } from './definitions/DashboardResp';
import type { RAlarmStats } from './definitions/RAlarmStats';
import type { RDashboard } from './definitions/RDashboard';
import type { RDeviceStatus } from './definitions/RDeviceStatus';
import type { RRiskStats } from './definitions/RRiskStats';
import type { RScreen } from './definitions/RScreen';
import type { RTrafficStats } from './definitions/RTrafficStats';
import type { StatsQueryReq } from './definitions/StatsQueryReq';

/**
 * 拼装查询串：跳过 undefined / null / 空串，避免后端把空值当作有效过滤条件。
 * 【手动保留，生成器不覆盖】swagger 无法表达「查询对象传参」这一约定，
 * 故 pageAlarms / pageRisks / pageInspectTasks / pageInspectRecords / riskStats /
 * alarmStats 等接口保持既有签名，避免改动 55 个页面文件的调用点。
 */
function q(params: Record<string, unknown>): string {
  const parts: string[] = [];
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null || v === '') continue
    parts.push(`${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
  }
  return parts.length ? `?${parts.join('&')}` : ''
}
/** 首页看板数据类型别名（Dashboard.tsx 直接 import 这个名字） */
export type DashboardData = DashboardResp

export class StatsController {

    static dashboardPath: string = "/web/stats/dashboard";
    /// 首页看板
    static async dashboard(headers?: any): Promise<RDashboard> {
      return request(`/web/stats/dashboard`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static deviceStatusPath: string = "/web/stats/device-status";
    /// 设备状态统计
    static async deviceStatus(headers?: any): Promise<RDeviceStatus> {
      return request(`/web/stats/device-status`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static screenPath: string = "/web/stats/screen";
    /// 可视化大屏
    static async screen(headers?: any): Promise<RScreen> {
      return request(`/web/stats/screen`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static trafficStatsPath: string = "/web/stats/traffic";
    /// 物联网卡流量监控
    static async trafficStats(headers?: any): Promise<RTrafficStats> {
      return request(`/web/stats/traffic`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    // 【手动保留，生成器不覆盖】查询对象传参，AlarmStats 传入 StatsQueryReq
    static async alarmStats(params?: StatsQueryReq, headers?: any): Promise<RAlarmStats> {
      return request(`/web/stats/alarm${q({ ...(params || {}) })}`, {
        baseURL: import.meta.env.VITE_API_BASE, method: "GET", headers,
      })
    }

    // 【手动保留，生成器不覆盖】查询对象传参，RiskStats / ScreenPage 传入 StatsQueryReq
    static async riskStats(params?: StatsQueryReq, headers?: any): Promise<RRiskStats> {
      return request(`/web/stats/risk${q({ ...(params || {}) })}`, {
        baseURL: import.meta.env.VITE_API_BASE, method: "GET", headers,
      })
    }
}
