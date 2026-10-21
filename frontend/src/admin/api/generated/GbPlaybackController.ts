/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { GbDownloadReq } from './definitions/GbDownloadReq';
import type { GbPlaybackControlReq } from './definitions/GbPlaybackControlReq';
import type { GbPlaybackReq } from './definitions/GbPlaybackReq';
import type { GbPlaybackStopReq } from './definitions/GbPlaybackStopReq';
import type { RGbDownload } from './definitions/RGbDownload';
import type { RGbPlayback } from './definitions/RGbPlayback';
import type { RGbRecordQuery } from './definitions/RGbRecordQuery';
import type { RPageGbDownload } from './definitions/RPageGbDownload';

export class GbPlaybackController {

    static startGbPlaybackPath: string = "/web/gb/playback";
    /// 发起回放
    static async startGbPlayback(data: GbPlaybackReq, headers?: any): Promise<RGbPlayback> {
      return request(`/web/gb/playback`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static controlGbPlaybackPath: string = "/web/gb/playback/control";
    /// 回放控制
    static async controlGbPlayback(data: GbPlaybackControlReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/playback/control`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static pageGbDownloadsPath: string = "/web/gb/playback/download";
    /// 下载任务分页
    static async pageGbDownloads(page?: number, page_size?: number, keyword?: string, channel_id?: number, status?: string, headers?: any): Promise<RPageGbDownload> {
      return request(`/web/gb/playback/download?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&channel_id=${channel_id ?? "" }&status=${status ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static startGbDownloadPath: string = "/web/gb/playback/download";
    /// 发起录像下载
    static async startGbDownload(data: GbDownloadReq, headers?: any): Promise<RGbDownload> {
      return request(`/web/gb/playback/download`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static stopGbDownloadPath: string = "/web/gb/playback/download/{id}";
    /// 停止下载任务
    static async stopGbDownload(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/playback/download/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static queryGbPlaybackRecordsPath: string = "/web/gb/playback/records";
    /// 检索设备侧录像
    static async queryGbPlaybackRecords(channel_id: number, start_time: string, end_time: string, type?: string, headers?: any): Promise<RGbRecordQuery> {
      return request(`/web/gb/playback/records?channel_id=${channel_id ?? "" }&start_time=${start_time ?? "" }&end_time=${end_time ?? "" }&type=${type ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static stopGbPlaybackPath: string = "/web/gb/playback/stop";
    /// 停止回放
    static async stopGbPlayback(data: GbPlaybackStopReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/playback/stop`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }
}
