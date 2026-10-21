/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { GbAlarmClearReq } from './definitions/GbAlarmClearReq';
import type { GbAlarmHandleReq } from './definitions/GbAlarmHandleReq';
import type { RGbAlarm } from './definitions/RGbAlarm';
import type { RGbSnapshot } from './definitions/RGbSnapshot';
import type { RPageGbAlarm } from './definitions/RPageGbAlarm';

export class GbAlarmController {

    static pageGbAlarmsPath: string = "/web/gb/alarms";
    /// 国标报警分页
    static async pageGbAlarms(page?: number, page_size?: number, keyword?: string, device_id?: number, channel_id?: number, alarm_type?: string, alarm_priority?: string, handled?: boolean, start_dt?: string, end_dt?: string, headers?: any): Promise<RPageGbAlarm> {
      return request(`/web/gb/alarms?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&device_id=${device_id ?? "" }&channel_id=${channel_id ?? "" }&alarm_type=${alarm_type ?? "" }&alarm_priority=${alarm_priority ?? "" }&handled=${handled ?? "" }&start_dt=${start_dt ?? "" }&end_dt=${end_dt ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static clearGbAlarmsPath: string = "/web/gb/alarms";
    /// 批量清除国标报警
    static async clearGbAlarms(data: GbAlarmClearReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/alarms`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getGbAlarmPath: string = "/web/gb/alarms/{id}";
    /// 国标报警详情
    static async getGbAlarm(id: string | number, headers?: any): Promise<RGbAlarm> {
      return request(`/web/gb/alarms/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static handleGbAlarmPath: string = "/web/gb/alarms/{id}/handle";
    /// 处理国标报警
    static async handleGbAlarm(id: string | number, data: GbAlarmHandleReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/alarms/${id}/handle`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static snapshotGbAlarmPath: string = "/web/gb/alarms/{id}/snapshot";
    /// 报警抓图
    static async snapshotGbAlarm(id: string | number, headers?: any): Promise<RGbSnapshot> {
      return request(`/web/gb/alarms/${id}/snapshot`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }
}
