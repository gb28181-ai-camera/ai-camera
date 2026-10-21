/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateGbSubscribeReq } from './definitions/CreateGbSubscribeReq';
import type { RGbSubscribe } from './definitions/RGbSubscribe';
import type { RPageGbSubscribe } from './definitions/RPageGbSubscribe';

export class GbSubscribeController {

    static pageGbSubscribesPath: string = "/web/gb/subscribes";
    /// 订阅记录分页列表
    static async pageGbSubscribes(page?: number, page_size?: number, gb_device_id?: string, type?: string, status?: string, headers?: any): Promise<RPageGbSubscribe> {
      return request(`/web/gb/subscribes?page=${page ?? "" }&page_size=${page_size ?? "" }&gb_device_id=${gb_device_id ?? "" }&type=${type ?? "" }&status=${status ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createGbSubscribePath: string = "/web/gb/subscribes";
    /// 新增订阅记录
    static async createGbSubscribe(data: CreateGbSubscribeReq, headers?: any): Promise<RGbSubscribe> {
      return request(`/web/gb/subscribes`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteGbSubscribePath: string = "/web/gb/subscribes/{id}";
    /// 删除订阅记录
    static async deleteGbSubscribe(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/subscribes/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }
}
