/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateInspectTaskReq } from './definitions/CreateInspectTaskReq';
import type { PageInspectRecordReq } from './definitions/PageInspectRecordReq';
import type { PageInspectTaskReq } from './definitions/PageInspectTaskReq';
import type { RBool } from './definitions/RBool';
import type { RInspectRecord } from './definitions/RInspectRecord';
import type { RInspectTask } from './definitions/RInspectTask';
import type { RPageInspectRecord } from './definitions/RPageInspectRecord';
import type { RPageInspectTask } from './definitions/RPageInspectTask';
import type { UpdateInspectTaskReq } from './definitions/UpdateInspectTaskReq';

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

export class InspectController {

    static getInspectRecordPath: string = "/web/inspects/records/{id}";
    /// 巡检记录详情
    static async getInspectRecord(id: string | number, headers?: any): Promise<RInspectRecord> {
      return request(`/web/inspects/records/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createInspectTaskPath: string = "/web/inspects/tasks";
    /// 创建巡检任务
    static async createInspectTask(data: CreateInspectTaskReq, headers?: any): Promise<RInspectTask> {
      return request(`/web/inspects/tasks`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getInspectTaskPath: string = "/web/inspects/tasks/{id}";
    /// 巡检任务详情
    static async getInspectTask(id: string | number, headers?: any): Promise<RInspectTask> {
      return request(`/web/inspects/tasks/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateInspectTaskPath: string = "/web/inspects/tasks/{id}";
    /// 更新巡检任务
    static async updateInspectTask(id: string | number, data: UpdateInspectTaskReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/inspects/tasks/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteInspectTaskPath: string = "/web/inspects/tasks/{id}";
    /// 删除巡检任务
    static async deleteInspectTask(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/inspects/tasks/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static markInspectTaskReadPath: string = "/web/inspects/tasks/{id}/read";
    /// 标记巡检任务消息通知已读
    static async markInspectTaskRead(id: string | number, headers?: any): Promise<RBool> {
      return request(`/web/inspects/tasks/${id}/read`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    // 【手动保留，生成器不覆盖】查询对象传参，InspectRecordList 传入 PageInspectRecordReq
    static async pageInspectRecords(params?: PageInspectRecordReq, headers?: any): Promise<RPageInspectRecord> {
      return request(`/web/inspects/records${q({ ...(params || {}) })}`, {
        baseURL: import.meta.env.VITE_API_BASE, method: "GET", headers,
      })
    }

    // 【手动保留，生成器不覆盖】查询对象传参，InspectTaskList 传入 PageInspectTaskReq
    static async pageInspectTasks(params?: PageInspectTaskReq, headers?: any): Promise<RPageInspectTask> {
      return request(`/web/inspects/tasks${q({ ...(params || {}) })}`, {
        baseURL: import.meta.env.VITE_API_BASE, method: "GET", headers,
      })
    }
}
