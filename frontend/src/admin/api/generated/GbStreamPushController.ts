/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateGbStreamPushReq } from './definitions/CreateGbStreamPushReq';
import type { RGbStreamPush } from './definitions/RGbStreamPush';
import type { RPageGbStreamPush } from './definitions/RPageGbStreamPush';
import type { UpdateGbStreamPushReq } from './definitions/UpdateGbStreamPushReq';

export class GbStreamPushController {

    static pageGbStreamPushesPath: string = "/web/gb/stream-pushes";
    /// 推流分页
    static async pageGbStreamPushes(page?: number, page_size?: number, keyword?: string, pushing?: boolean, media_server_id?: string, status?: string, headers?: any): Promise<RPageGbStreamPush> {
      return request(`/web/gb/stream-pushes?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&pushing=${pushing ?? "" }&media_server_id=${media_server_id ?? "" }&status=${status ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createGbStreamPushPath: string = "/web/gb/stream-pushes";
    /// 新增推流配置
    static async createGbStreamPush(data: CreateGbStreamPushReq, headers?: any): Promise<RGbStreamPush> {
      return request(`/web/gb/stream-pushes`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static updateGbStreamPushPath: string = "/web/gb/stream-pushes/{id}";
    /// 编辑推流配置
    static async updateGbStreamPush(id: string | number, data: UpdateGbStreamPushReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/stream-pushes/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteGbStreamPushPath: string = "/web/gb/stream-pushes/{id}";
    /// 删除推流配置
    static async deleteGbStreamPush(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/stream-pushes/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static startGbStreamPushPath: string = "/web/gb/stream-pushes/{id}/start";
    /// 启用推流
    static async startGbStreamPush(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/stream-pushes/${id}/start`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static stopGbStreamPushPath: string = "/web/gb/stream-pushes/{id}/stop";
    /// 停止推流
    static async stopGbStreamPush(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/stream-pushes/${id}/stop`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }
}
