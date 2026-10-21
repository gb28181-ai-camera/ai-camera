/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { RGbSipLog } from './definitions/RGbSipLog';
import type { RGbSipLogStats } from './definitions/RGbSipLogStats';
import type { RPageGbSipLog } from './definitions/RPageGbSipLog';

export class GbSipLogController {

    static pageGbSipLogsPath: string = "/web/gb/sip-logs";
    /// 信令日志分页列表
    static async pageGbSipLogs(page?: number, page_size?: number, gb_device_id?: string, gb_channel_id?: string, direction?: string, method?: string, cmd_type?: string, result?: string, start_dt?: string, end_dt?: string, headers?: any): Promise<RPageGbSipLog> {
      return request(`/web/gb/sip-logs?page=${page ?? "" }&page_size=${page_size ?? "" }&gb_device_id=${gb_device_id ?? "" }&gb_channel_id=${gb_channel_id ?? "" }&direction=${direction ?? "" }&method=${method ?? "" }&cmd_type=${cmd_type ?? "" }&result=${result ?? "" }&start_dt=${start_dt ?? "" }&end_dt=${end_dt ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static getGbSipLogPath: string = "/web/gb/sip-logs/{id}";
    /// 信令日志详情
    static async getGbSipLog(id: string | number, headers?: any): Promise<RGbSipLog> {
      return request(`/web/gb/sip-logs/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static gbSipLogStatsPath: string = "/web/gb/sip-logs/stats";
    /// 信令日志统计
    static async gbSipLogStats(gb_device_id?: string, direction?: string, method?: string, cmd_type?: string, result?: string, start_dt?: string, end_dt?: string, headers?: any): Promise<RGbSipLogStats> {
      return request(`/web/gb/sip-logs/stats?gb_device_id=${gb_device_id ?? "" }&direction=${direction ?? "" }&method=${method ?? "" }&cmd_type=${cmd_type ?? "" }&result=${result ?? "" }&start_dt=${start_dt ?? "" }&end_dt=${end_dt ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }
}
