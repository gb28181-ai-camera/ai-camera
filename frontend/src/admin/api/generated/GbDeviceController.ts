/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { GbSubscribeActionReq } from './definitions/GbSubscribeActionReq';
import type { RGbActivation } from './definitions/RGbActivation';
import type { RGbDevice } from './definitions/RGbDevice';
import type { RPageGbDevice } from './definitions/RPageGbDevice';

export class GbDeviceController {

    static pageGbDevicesPath: string = "/web/gb/devices";
    /// 国标设备分页列表
    static async pageGbDevices(page?: number, page_size?: number, keyword?: string, register_status?: string, status?: string, region_id?: number, headers?: any): Promise<RPageGbDevice> {
      return request(`/web/gb/devices?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&register_status=${register_status ?? "" }&status=${status ?? "" }&region_id=${region_id ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static getGbDevicePath: string = "/web/gb/devices/{id}";
    /// 国标设备详情
    static async getGbDevice(id: string | number, headers?: any): Promise<RGbDevice> {
      return request(`/web/gb/devices/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static deleteGbDevicePath: string = "/web/gb/devices/{id}";
    /// 删除国标设备
    static async deleteGbDevice(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/devices/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static generateGbActivationPath: string = "/web/gb/devices/{id}/activation";
    /// 生成设备激活凭证
    static async generateGbActivation(id: string | number, headers?: any): Promise<RGbActivation> {
      return request(`/web/gb/devices/${id}/activation`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static subscribeGbDevicePath: string = "/web/gb/devices/{id}/subscribe";
    /// 订阅设备
    static async subscribeGbDevice(id: string | number, data: GbSubscribeActionReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/devices/${id}/subscribe`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static unsubscribeGbDevicePath: string = "/web/gb/devices/{id}/subscribe";
    /// 取消订阅设备
    static async unsubscribeGbDevice(id: string | number, type: string, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/devices/${id}/subscribe?type=${type ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static syncGbDeviceChannelPath: string = "/web/gb/devices/{id}/sync-channel";
    /// 同步设备目录
    static async syncGbDeviceChannel(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/devices/${id}/sync-channel`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }
}
