/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { GbPositionSubscribeReq } from './definitions/GbPositionSubscribeReq';
import type { RGbPosition } from './definitions/RGbPosition';
import type { RGbPositionHistory } from './definitions/RGbPositionHistory';
import type { RGbPositionList } from './definitions/RGbPositionList';

export class GbPositionController {

    static pageGbPositionHistoryPath: string = "/web/gb/positions/history";
    /// 位置历史
    static async pageGbPositionHistory(page?: number, page_size?: number, device_id?: number, channel_id?: number, gb_channel_id?: string, start_dt?: string, end_dt?: string, headers?: any): Promise<RGbPositionHistory> {
      return request(`/web/gb/positions/history?page=${page ?? "" }&page_size=${page_size ?? "" }&device_id=${device_id ?? "" }&channel_id=${channel_id ?? "" }&gb_channel_id=${gb_channel_id ?? "" }&start_dt=${start_dt ?? "" }&end_dt=${end_dt ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static getGbPositionLatestPath: string = "/web/gb/positions/latest";
    /// 通道最新位置
    static async getGbPositionLatest(channel_id: number, headers?: any): Promise<RGbPosition> {
      return request(`/web/gb/positions/latest?channel_id=${channel_id ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static listGbPositionsRealtimePath: string = "/web/gb/positions/realtime";
    /// 实时位置列表
    static async listGbPositionsRealtime(headers?: any): Promise<RGbPositionList> {
      return request(`/web/gb/positions/realtime`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static subscribeGbPositionPath: string = "/web/gb/positions/subscribe";
    /// 订阅移动位置
    static async subscribeGbPosition(data: GbPositionSubscribeReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/positions/subscribe`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }
}
