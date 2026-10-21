/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { GbChannelGroupReq } from './definitions/GbChannelGroupReq';
import type { GbChannelRegionReq } from './definitions/GbChannelRegionReq';
import type { RGbChannel } from './definitions/RGbChannel';
import type { ROptionList } from './definitions/ROptionList';
import type { RPageGbChannel } from './definitions/RPageGbChannel';
import type { UpdateGbChannelReq } from './definitions/UpdateGbChannelReq';

export class GbChannelController {

    static pageGbChannelsPath: string = "/web/gb/channels";
    /// 国标通道分页列表
    static async pageGbChannels(page?: number, page_size?: number, keyword?: string, device_id?: number, region_id?: number, group_id?: number, status?: string, gb_device_id?: string, headers?: any): Promise<RPageGbChannel> {
      return request(`/web/gb/channels?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&device_id=${device_id ?? "" }&region_id=${region_id ?? "" }&group_id=${group_id ?? "" }&status=${status ?? "" }&gb_device_id=${gb_device_id ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static getGbChannelPath: string = "/web/gb/channels/{id}";
    /// 国标通道详情
    static async getGbChannel(id: string | number, headers?: any): Promise<RGbChannel> {
      return request(`/web/gb/channels/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateGbChannelPath: string = "/web/gb/channels/{id}";
    /// 编辑国标通道
    static async updateGbChannel(id: string | number, data: UpdateGbChannelReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/channels/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteGbChannelPath: string = "/web/gb/channels/{id}";
    /// 删除国标通道
    static async deleteGbChannel(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/channels/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static updateGbChannelGroupPath: string = "/web/gb/channels/{id}/group";
    /// 设置通道业务分组
    static async updateGbChannelGroup(id: string | number, data: GbChannelGroupReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/channels/${id}/group`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static updateGbChannelRegionPath: string = "/web/gb/channels/{id}/region";
    /// 设置通道区域
    static async updateGbChannelRegion(id: string | number, data: GbChannelRegionReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/channels/${id}/region`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static resetGbChannelPath: string = "/web/gb/channels/{id}/reset";
    /// 重置通道
    static async resetGbChannel(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/channels/${id}/reset`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static gbChannelOptionsPath: string = "/web/gb/channels/options";
    /// 通道下拉选项
    static async gbChannelOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/gb/channels/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }
}
