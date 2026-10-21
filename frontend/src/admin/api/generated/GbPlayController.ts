/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { GbPlayReq } from './definitions/GbPlayReq';
import type { RGbPlay } from './definitions/RGbPlay';
import type { RGbPlayStatus } from './definitions/RGbPlayStatus';
import type { RGbSnapshot } from './definitions/RGbSnapshot';

export class GbPlayController {

    static playGbChannelPath: string = "/web/gb/play/{channelId}";
    /// 发起实时点播
    static async playGbChannel(channelId: string | number, data?: GbPlayReq, headers?: any): Promise<RGbPlay> {
      return request(`/web/gb/play/${channelId}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static snapshotGbChannelPath: string = "/web/gb/play/{channelId}/snapshot";
    /// 通道抓图
    static async snapshotGbChannel(channelId: string | number, headers?: any): Promise<RGbSnapshot> {
      return request(`/web/gb/play/${channelId}/snapshot`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static getGbPlayStatusPath: string = "/web/gb/play/{channelId}/status";
    /// 查询通道点播状态
    static async getGbPlayStatus(channelId: string | number, headers?: any): Promise<RGbPlayStatus> {
      return request(`/web/gb/play/${channelId}/status`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static stopGbPlayPath: string = "/web/gb/play/{channelId}/stop";
    /// 停止实时点播
    static async stopGbPlay(channelId: string | number, data?: GbPlayReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/play/${channelId}/stop`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }
}
