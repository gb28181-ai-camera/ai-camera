/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { HandleAlarmReq } from './definitions/HandleAlarmReq';
import type { PageAlarmReq } from './definitions/PageAlarmReq';
import type { RAlarmRecord } from './definitions/RAlarmRecord';
import type { RCount } from './definitions/RCount';
import type { RPageAlarmRecord } from './definitions/RPageAlarmRecord';
import type { ReportAlarmReq } from './definitions/ReportAlarmReq';

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

export class AlarmController {

    static getAlarmPath: string = "/web/alarms/{id}";
    /// 查询告警详情
    static async getAlarm(id: string | number, headers?: any): Promise<RAlarmRecord> {
      return request(`/web/alarms/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static deleteAlarmPath: string = "/web/alarms/{id}";
    /// 删除告警记录
    static async deleteAlarm(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/alarms/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static handleAlarmPath: string = "/web/alarms/{id}/handle";
    /// 处理告警
    static async handleAlarm(id: string | number, data: HandleAlarmReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/alarms/${id}/handle`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static markAlarmReadPath: string = "/web/alarms/{id}/read";
    /// 标记告警通知已读
    static async markAlarmRead(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/alarms/${id}/read`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    static alarmPendingCountPath: string = "/web/alarms/pending-count";
    /// 未处理告警数量
    static async alarmPendingCount(headers?: any): Promise<RCount> {
      return request(`/web/alarms/pending-count`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static reportAlarmPath: string = "/web/alarms/report";
    /// 设备上报告警
    static async reportAlarm(data: ReportAlarmReq, headers?: any): Promise<RAlarmRecord> {
      return request(`/web/alarms/report`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    // 【手动保留，生成器不覆盖】查询对象传参，AlarmList 传入 PageAlarmReq
    static async pageAlarms(params?: PageAlarmReq, headers?: any): Promise<RPageAlarmRecord> {
      return request(`/web/alarms${q({ ...(params || {}) })}`, {
        baseURL: import.meta.env.VITE_API_BASE, method: "GET", headers,
      })
    }
}
