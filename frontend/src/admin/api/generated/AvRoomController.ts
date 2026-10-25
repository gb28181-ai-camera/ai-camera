/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { RAvViewerTicket } from './definitions/RAvViewerTicket';
import type { RPageIotDevice } from './definitions/RPageIotDevice';

export class AvRoomController {

    static pageAvDevicesPath: string = "/web/av/devices";
    /// 实时音视频设备下拉
    static async pageAvDevices(page?: number, page_size?: number, keyword?: string, headers?: any): Promise<RPageIotDevice> {
      return request(`/web/av/devices?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createAvViewerTicketPath: string = "/web/av/devices/{id}/viewer-ticket";
    /// 查看设备画面 ticket
    static async createAvViewerTicket(id: string | number, headers?: any): Promise<RAvViewerTicket> {
      return request(`/web/av/devices/${id}/viewer-ticket`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static getAvWsPath: string = "/web/av/ws";
    /// 实时音视频信令 WebSocket
    static async getAvWs(ticket: string, headers?: any): Promise<any> {
      return request(`/web/av/ws?ticket=${ticket ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }
}
