/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateGbStreamProxyReq } from './definitions/CreateGbStreamProxyReq';
import type { RGbStreamProxy } from './definitions/RGbStreamProxy';
import type { RPageGbStreamProxy } from './definitions/RPageGbStreamProxy';
import type { UpdateGbStreamProxyReq } from './definitions/UpdateGbStreamProxyReq';

export class GbStreamProxyController {

    static pageGbStreamProxiesPath: string = "/web/gb/stream-proxies";
    /// 拉流代理分页
    static async pageGbStreamProxies(page?: number, page_size?: number, keyword?: string, pulling?: boolean, media_server_id?: string, headers?: any): Promise<RPageGbStreamProxy> {
      return request(`/web/gb/stream-proxies?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&pulling=${pulling ?? "" }&media_server_id=${media_server_id ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createGbStreamProxyPath: string = "/web/gb/stream-proxies";
    /// 新增拉流代理
    static async createGbStreamProxy(data: CreateGbStreamProxyReq, headers?: any): Promise<RGbStreamProxy> {
      return request(`/web/gb/stream-proxies`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static updateGbStreamProxyPath: string = "/web/gb/stream-proxies/{id}";
    /// 编辑拉流代理
    static async updateGbStreamProxy(id: string | number, data: UpdateGbStreamProxyReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/stream-proxies/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteGbStreamProxyPath: string = "/web/gb/stream-proxies/{id}";
    /// 删除拉流代理
    static async deleteGbStreamProxy(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/stream-proxies/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static startGbStreamProxyPath: string = "/web/gb/stream-proxies/{id}/start";
    /// 开始拉流
    static async startGbStreamProxy(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/stream-proxies/${id}/start`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static stopGbStreamProxyPath: string = "/web/gb/stream-proxies/{id}/stop";
    /// 停止拉流
    static async stopGbStreamProxy(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/stream-proxies/${id}/stop`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }
}
