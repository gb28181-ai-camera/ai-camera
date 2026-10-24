/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { GbPresetReq } from './definitions/GbPresetReq';
import type { GbPtzCruiseReq } from './definitions/GbPtzCruiseReq';
import type { GbPtzPreciseReq } from './definitions/GbPtzPreciseReq';
import type { GbPtzReq } from './definitions/GbPtzReq';
import type { GbPtzScanReq } from './definitions/GbPtzScanReq';
import type { GbPtzWiperReq } from './definitions/GbPtzWiperReq';
import type { RGbCruiseTrackDetail } from './definitions/RGbCruiseTrackDetail';
import type { RGbCruiseTrackList } from './definitions/RGbCruiseTrackList';
import type { RGbHomePosition } from './definitions/RGbHomePosition';
import type { RGbPresetList } from './definitions/RGbPresetList';
import type { RGbPtzPosition } from './definitions/RGbPtzPosition';
import type { RGbSDCardStatus } from './definitions/RGbSDCardStatus';

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

    static listGbPtzCruiseTracksPath: string = "/web/gb/ptz/{channelId}/cruise-tracks";
    /// 巡航轨迹列表
    static async listGbPtzCruiseTracks(channelId: string | number, headers?: any): Promise<RGbCruiseTrackList> {
      return request(`/web/gb/ptz/${channelId}/cruise-tracks`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static getGbPtzCruiseTrackPath: string = "/web/gb/ptz/{channelId}/cruise-tracks/{number}";
    /// 巡航轨迹明细
    static async getGbPtzCruiseTrack(channelId: string | number, number: string | number, headers?: any): Promise<RGbCruiseTrackDetail> {
      return request(`/web/gb/ptz/${channelId}/cruise-tracks/${number}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static getGbPtzHomePositionPath: string = "/web/gb/ptz/{channelId}/home-position";
    /// 查询看守位
    static async getGbPtzHomePosition(channelId: string | number, headers?: any): Promise<RGbHomePosition> {
      return request(`/web/gb/ptz/${channelId}/home-position`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static gbPtzPositionPath: string = "/web/gb/ptz/{channelId}/position";
    /// 查询 PTZ 精准位置
    static async gbPtzPosition(channelId: string | number, headers?: any): Promise<RGbPtzPosition> {
      return request(`/web/gb/ptz/${channelId}/position`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static preciseGbPtzPath: string = "/web/gb/ptz/{channelId}/precise";
    /// PTZ 精准控制
    static async preciseGbPtz(channelId: string | number, data: GbPtzPreciseReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/ptz/${channelId}/precise`, {
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

    static gotoGbPtzPresetPath: string = "/web/gb/ptz/{channelId}/presets/{presetId}/goto";
    /// 调用预置位
    static async gotoGbPtzPreset(channelId: string | number, presetId: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/ptz/${channelId}/presets/${presetId}/goto`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
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

    static getGbPtzSDCardPath: string = "/web/gb/ptz/{channelId}/sdcard";
    /// 查询存储卡状态
    static async getGbPtzSDCard(channelId: string | number, headers?: any): Promise<RGbSDCardStatus> {
      return request(`/web/gb/ptz/${channelId}/sdcard`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
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
