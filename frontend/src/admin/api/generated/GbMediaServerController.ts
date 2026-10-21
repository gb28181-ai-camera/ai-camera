/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateGbMediaServerReq } from './definitions/CreateGbMediaServerReq';
import type { RGbMediaServer } from './definitions/RGbMediaServer';
import type { RGbMediaServerCheck } from './definitions/RGbMediaServerCheck';
import type { RGbMediaServerCreated } from './definitions/RGbMediaServerCreated';
import type { RGbMediaServerInfo } from './definitions/RGbMediaServerInfo';
import type { RGbMediaServerList } from './definitions/RGbMediaServerList';
import type { UpdateGbMediaServerReq } from './definitions/UpdateGbMediaServerReq';

export class GbMediaServerController {

    static listGbMediaServersPath: string = "/web/gb/media-servers";
    /// 媒体节点列表
    static async listGbMediaServers(headers?: any): Promise<RGbMediaServerList> {
      return request(`/web/gb/media-servers`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createGbMediaServerPath: string = "/web/gb/media-servers";
    /// 新增媒体节点
    static async createGbMediaServer(data: CreateGbMediaServerReq, headers?: any): Promise<RGbMediaServerCreated> {
      return request(`/web/gb/media-servers`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getGbMediaServerPath: string = "/web/gb/media-servers/{id}";
    /// 媒体节点详情
    static async getGbMediaServer(id: string | number, headers?: any): Promise<RGbMediaServer> {
      return request(`/web/gb/media-servers/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateGbMediaServerPath: string = "/web/gb/media-servers/{id}";
    /// 编辑媒体节点
    static async updateGbMediaServer(id: string | number, data: UpdateGbMediaServerReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/media-servers/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteGbMediaServerPath: string = "/web/gb/media-servers/{id}";
    /// 删除媒体节点
    static async deleteGbMediaServer(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/media-servers/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static checkGbMediaServerPath: string = "/web/gb/media-servers/{id}/check";
    /// 媒体节点连通性检查
    static async checkGbMediaServer(id: string | number, headers?: any): Promise<RGbMediaServerCheck> {
      return request(`/web/gb/media-servers/${id}/check`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static getGbMediaServerInfoPath: string = "/web/gb/media-servers/{id}/info";
    /// 媒体节点运行信息
    static async getGbMediaServerInfo(id: string | number, headers?: any): Promise<RGbMediaServerInfo> {
      return request(`/web/gb/media-servers/${id}/info`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }
}
