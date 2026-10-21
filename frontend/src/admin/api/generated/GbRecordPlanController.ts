/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateGbRecordPlanReq } from './definitions/CreateGbRecordPlanReq';
import type { LinkGbRecordPlanChannelsReq } from './definitions/LinkGbRecordPlanChannelsReq';
import type { RGbRecordPlan } from './definitions/RGbRecordPlan';
import type { RGbRecordPlanCreated } from './definitions/RGbRecordPlanCreated';
import type { RPageGbRecordPlan } from './definitions/RPageGbRecordPlan';
import type { UpdateGbRecordPlanReq } from './definitions/UpdateGbRecordPlanReq';

export class GbRecordPlanController {

    static pageGbRecordPlansPath: string = "/web/gb/record-plans";
    /// 录像计划分页
    static async pageGbRecordPlans(page?: number, page_size?: number, keyword?: string, headers?: any): Promise<RPageGbRecordPlan> {
      return request(`/web/gb/record-plans?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createGbRecordPlanPath: string = "/web/gb/record-plans";
    /// 新增录像计划
    static async createGbRecordPlan(data: CreateGbRecordPlanReq, headers?: any): Promise<RGbRecordPlanCreated> {
      return request(`/web/gb/record-plans`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getGbRecordPlanPath: string = "/web/gb/record-plans/{id}";
    /// 录像计划详情
    static async getGbRecordPlan(id: string | number, headers?: any): Promise<RGbRecordPlan> {
      return request(`/web/gb/record-plans/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateGbRecordPlanPath: string = "/web/gb/record-plans/{id}";
    /// 编辑录像计划
    static async updateGbRecordPlan(id: string | number, data: UpdateGbRecordPlanReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/record-plans/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteGbRecordPlanPath: string = "/web/gb/record-plans/{id}";
    /// 删除录像计划
    static async deleteGbRecordPlan(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/record-plans/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static linkGbRecordPlanChannelsPath: string = "/web/gb/record-plans/{id}/channels";
    /// 计划绑定通道
    static async linkGbRecordPlanChannels(id: string | number, data: LinkGbRecordPlanChannelsReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/record-plans/${id}/channels`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }
}
