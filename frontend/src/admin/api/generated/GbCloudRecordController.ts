/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { GbCloudRecordBatchReq } from './definitions/GbCloudRecordBatchReq';
import type { GbCloudRecordCollectReq } from './definitions/GbCloudRecordCollectReq';
import type { RGbCloudRecordDates } from './definitions/RGbCloudRecordDates';
import type { RGbCloudRecordDownload } from './definitions/RGbCloudRecordDownload';
import type { RGbCloudRecordPlay } from './definitions/RGbCloudRecordPlay';
import type { RPageGbCloudRecord } from './definitions/RPageGbCloudRecord';

export class GbCloudRecordController {

    static pageGbCloudRecordsPath: string = "/web/gb/cloud-records";
    /// 云端录像分页
    static async pageGbCloudRecords(page?: number, page_size?: number, keyword?: string, app?: string, stream?: string, device_id?: number, channel_id?: number, media_server_id?: string, collect?: boolean, start_dt?: string, end_dt?: string, headers?: any): Promise<RPageGbCloudRecord> {
      return request(`/web/gb/cloud-records?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&app=${app ?? "" }&stream=${stream ?? "" }&device_id=${device_id ?? "" }&channel_id=${channel_id ?? "" }&media_server_id=${media_server_id ?? "" }&collect=${collect ?? "" }&start_dt=${start_dt ?? "" }&end_dt=${end_dt ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static deleteGbCloudRecordsPath: string = "/web/gb/cloud-records";
    /// 批量删除录像索引
    static async deleteGbCloudRecords(data: GbCloudRecordBatchReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/cloud-records`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getGbCloudRecordDownloadPath: string = "/web/gb/cloud-records/{id}/download";
    /// 云端录像下载地址
    static async getGbCloudRecordDownload(id: string | number, headers?: any): Promise<RGbCloudRecordDownload> {
      return request(`/web/gb/cloud-records/${id}/download`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static getGbCloudRecordPlayPath: string = "/web/gb/cloud-records/{id}/play";
    /// 云端录像播放地址
    static async getGbCloudRecordPlay(id: string | number, headers?: any): Promise<RGbCloudRecordPlay> {
      return request(`/web/gb/cloud-records/${id}/play`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static collectGbCloudRecordsPath: string = "/web/gb/cloud-records/collect";
    /// 批量收藏/取消收藏
    static async collectGbCloudRecords(data: GbCloudRecordCollectReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/cloud-records/collect`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static listGbCloudRecordDatesPath: string = "/web/gb/cloud-records/dates";
    /// 有录像的日期列表
    static async listGbCloudRecordDates(device_id?: number, channel_id?: number, media_server_id?: string, start_dt?: string, end_dt?: string, headers?: any): Promise<RGbCloudRecordDates> {
      return request(`/web/gb/cloud-records/dates?device_id=${device_id ?? "" }&channel_id=${channel_id ?? "" }&media_server_id=${media_server_id ?? "" }&start_dt=${start_dt ?? "" }&end_dt=${end_dt ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }
}
