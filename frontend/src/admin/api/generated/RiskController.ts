/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { HandleRiskReq } from './definitions/HandleRiskReq';
import type { PageRiskReq } from './definitions/PageRiskReq';
import type { RCount } from './definitions/RCount';
import type { RPageRiskRecord } from './definitions/RPageRiskRecord';
import type { RRiskRecord } from './definitions/RRiskRecord';
import type { ReportRiskReq } from './definitions/ReportRiskReq';
import type { UpdateAccurateReq } from './definitions/UpdateAccurateReq';

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

export class RiskController {

    static getRiskPath: string = "/web/risks/{id}";
    /// 查询风险详情
    static async getRisk(id: string | number, headers?: any): Promise<RRiskRecord> {
      return request(`/web/risks/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static deleteRiskPath: string = "/web/risks/{id}";
    /// 删除风险记录
    static async deleteRisk(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/risks/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static updateRiskAccuratePath: string = "/web/risks/{id}/accurate";
    /// 确认检测准确性
    static async updateRiskAccurate(id: string | number, data: UpdateAccurateReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/risks/${id}/accurate`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static handleRiskPath: string = "/web/risks/{id}/handle";
    /// 处理风险
    static async handleRisk(id: string | number, data: HandleRiskReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/risks/${id}/handle`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static markRiskReadPath: string = "/web/risks/{id}/read";
    /// 标记风险通知已读
    static async markRiskRead(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/risks/${id}/read`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    static riskPendingCountPath: string = "/web/risks/pending-count";
    /// 已确认待处理风险数量
    static async riskPendingCount(headers?: any): Promise<RCount> {
      return request(`/web/risks/pending-count`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static reportRiskPath: string = "/web/risks/report";
    /// 设备上报风险
    static async reportRisk(data: ReportRiskReq, headers?: any): Promise<RRiskRecord> {
      return request(`/web/risks/report`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    // 【手动保留，生成器不覆盖】查询对象传参，4 个页面传入 PageRiskReq
    static async pageRisks(params?: PageRiskReq, headers?: any): Promise<RPageRiskRecord> {
      return request(`/web/risks${q({ ...(params || {}) })}`, {
        baseURL: import.meta.env.VITE_API_BASE, method: "GET", headers,
      })
    }
}
