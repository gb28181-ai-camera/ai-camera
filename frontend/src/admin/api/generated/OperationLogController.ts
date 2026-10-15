/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { ROperationLog } from './definitions/ROperationLog';
import type { RPageOperationLog } from './definitions/RPageOperationLog';

/** 操作日志查询参数（OperationLogList 直接 import 这个名字） */
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
    return `module=${query?.module ?? "" }&operator=${query?.operator ?? "" }&action=${query?.action ?? "" }&start_dt=${query?.start_dt ?? "" }&end_dt=${query?.end_dt ?? "" }`;
}

export class OperationLogController {

    static getOperationLogPath: string = "/web/system/logs/{id}";
    /// 操作日志详情
    static async getOperationLog(id: string | number, headers?: any): Promise<ROperationLog> {
      return request(`/web/system/logs/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static deleteOperationLogPath: string = "/web/system/logs/{id}";
    /// 删除操作日志
    static async deleteOperationLog(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/logs/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static clearOperationLogsPath: string = "/web/system/logs/clear";
    /// 清空操作日志
    static async clearOperationLogs(headers?: any): Promise<BaseResponse> {
      return request(`/web/system/logs/clear`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    // 【手动保留，生成器不覆盖】第 3 个参数是查询对象，OperationLogList 传入 buildQuery()
    static async pageOperationLogs(page: number, page_size: number, query?: OperationLogQuery, headers?: any): Promise<RPageOperationLog> {
      return request(`/web/system/logs?page=${page ?? "" }&page_size=${page_size ?? "" }&${buildLogQuery(query)}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    // 【手动保留，生成器不覆盖】返回 CSV 文件流：swagger 只声明 string，实际需要 responseType=blob
    static async exportOperationLogs(query?: OperationLogQuery, headers?: any): Promise<Blob> {
      return request(`/web/system/logs/export?${buildLogQuery(query)}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        responseType: "blob",
        headers,
      });
    }

    // 【手动保留，生成器不覆盖】纯前端拼接导出地址（window.open 场景不带鉴权头），后端无对应 operation
    static buildExportLogsUrl(query?: OperationLogQuery): string {
      const base = import.meta.env.VITE_API_BASE || "";
      return `${base}/web/system/logs/export?${buildLogQuery(query)}`;
    }
}
