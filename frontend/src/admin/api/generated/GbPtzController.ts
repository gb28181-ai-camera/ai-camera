/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { GbPresetReq } from './definitions/GbPresetReq';
import type { GbPtzCruiseReq } from './definitions/GbPtzCruiseReq';
import type { GbPtzReq } from './definitions/GbPtzReq';
import type { GbPtzScanReq } from './definitions/GbPtzScanReq';
import type { GbPtzWiperReq } from './definitions/GbPtzWiperReq';
import type { RGbPresetList } from './definitions/RGbPresetList';

export class GbPtzController {

    static controlGbPtzPath: string = "/web/gb/ptz/{channelId}";
    /// 云台控制
    static async controlGbPtz(channelId: string | number, data: GbPtzReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/ptz/${channelId}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static cruiseGbPtzPath: string = "/web/gb/ptz/{channelId}/cruise";
    /// 巡航控制
    static async cruiseGbPtz(channelId: string | number, data: GbPtzCruiseReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/ptz/${channelId}/cruise`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static listGbPtzPresetsPath: string = "/web/gb/ptz/{channelId}/presets";
    /// 预置位列表
    static async listGbPtzPresets(channelId: string | number, headers?: any): Promise<RGbPresetList> {
      return request(`/web/gb/ptz/${channelId}/presets`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static addGbPtzPresetPath: string = "/web/gb/ptz/{channelId}/presets";
    /// 设置预置位
    static async addGbPtzPreset(channelId: string | number, data: GbPresetReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/ptz/${channelId}/presets`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteGbPtzPresetPath: string = "/web/gb/ptz/{channelId}/presets/{presetId}";
    /// 删除预置位
    static async deleteGbPtzPreset(channelId: string | number, presetId: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/ptz/${channelId}/presets/${presetId}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static scanGbPtzPath: string = "/web/gb/ptz/{channelId}/scan";
    /// 自动扫描控制
    static async scanGbPtz(channelId: string | number, data: GbPtzScanReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/ptz/${channelId}/scan`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static wiperGbPtzPath: string = "/web/gb/ptz/{channelId}/wiper";
    /// 雨刷控制
    static async wiperGbPtz(channelId: string | number, data?: GbPtzWiperReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/ptz/${channelId}/wiper`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }
}
