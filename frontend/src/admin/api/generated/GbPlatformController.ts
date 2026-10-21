/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateGbPlatformReq } from './definitions/CreateGbPlatformReq';
import type { GbPlatformChannelIDsReq } from './definitions/GbPlatformChannelIDsReq';
import type { RGbPlatform } from './definitions/RGbPlatform';
import type { RGbPlatformCreated } from './definitions/RGbPlatformCreated';
import type { RPageGbPlatform } from './definitions/RPageGbPlatform';
import type { RPageGbPlatformChannel } from './definitions/RPageGbPlatformChannel';
import type { SetGbPlatformGroupsReq } from './definitions/SetGbPlatformGroupsReq';
import type { SetGbPlatformRegionsReq } from './definitions/SetGbPlatformRegionsReq';
import type { UpdateGbPlatformChannelReq } from './definitions/UpdateGbPlatformChannelReq';
import type { UpdateGbPlatformReq } from './definitions/UpdateGbPlatformReq';

export class GbPlatformController {

    static pageGbPlatformsPath: string = "/web/gb/platforms";
    /// 上级平台分页
    static async pageGbPlatforms(page?: number, page_size?: number, keyword?: string, status?: string, headers?: any): Promise<RPageGbPlatform> {
      return request(`/web/gb/platforms?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&status=${status ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createGbPlatformPath: string = "/web/gb/platforms";
    /// 新增上级平台
    static async createGbPlatform(data: CreateGbPlatformReq, headers?: any): Promise<RGbPlatformCreated> {
      return request(`/web/gb/platforms`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getGbPlatformPath: string = "/web/gb/platforms/{id}";
    /// 上级平台详情
    static async getGbPlatform(id: string | number, headers?: any): Promise<RGbPlatform> {
      return request(`/web/gb/platforms/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateGbPlatformPath: string = "/web/gb/platforms/{id}";
    /// 编辑上级平台
    static async updateGbPlatform(id: string | number, data: UpdateGbPlatformReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/platforms/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteGbPlatformPath: string = "/web/gb/platforms/{id}";
    /// 删除上级平台
    static async deleteGbPlatform(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/platforms/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static pageGbPlatformChannelsPath: string = "/web/gb/platforms/{id}/channels";
    /// 级联通道分页
    static async pageGbPlatformChannels(id: string | number, page?: number, page_size?: number, keyword?: string, channel_id?: number, headers?: any): Promise<RPageGbPlatformChannel> {
      return request(`/web/gb/platforms/${id}/channels?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&channel_id=${channel_id ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static addGbPlatformChannelsPath: string = "/web/gb/platforms/{id}/channels";
    /// 添加级联通道
    static async addGbPlatformChannels(id: string | number, data: GbPlatformChannelIDsReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/platforms/${id}/channels`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static removeGbPlatformChannelsPath: string = "/web/gb/platforms/{id}/channels";
    /// 移除级联通道
    static async removeGbPlatformChannels(id: string | number, data: GbPlatformChannelIDsReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/platforms/${id}/channels`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        data: JSON.stringify(data),
        headers,
      });
    }

    static updateGbPlatformChannelCustomPath: string = "/web/gb/platforms/{id}/channels/{channelId}";
    /// 修改级联通道定制信息
    static async updateGbPlatformChannelCustom(id: string | number, channelId: string | number, data: UpdateGbPlatformChannelReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/platforms/${id}/channels/${channelId}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static setGbPlatformGroupsPath: string = "/web/gb/platforms/{id}/groups";
    /// 设置分组可见范围
    static async setGbPlatformGroups(id: string | number, data: SetGbPlatformGroupsReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/platforms/${id}/groups`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static setGbPlatformRegionsPath: string = "/web/gb/platforms/{id}/regions";
    /// 设置区域可见范围
    static async setGbPlatformRegions(id: string | number, data: SetGbPlatformRegionsReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/platforms/${id}/regions`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }
}
