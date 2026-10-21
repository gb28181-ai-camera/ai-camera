/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateGbGroupReq } from './definitions/CreateGbGroupReq';
import type { RGbGroup } from './definitions/RGbGroup';
import type { RGbGroupList } from './definitions/RGbGroupList';
import type { RGbGroupTree } from './definitions/RGbGroupTree';
import type { UpdateGbGroupReq } from './definitions/UpdateGbGroupReq';

export class GbGroupController {

    static listGbGroupsPath: string = "/web/gb/groups";
    /// 业务分组列表
    static async listGbGroups(headers?: any): Promise<RGbGroupList> {
      return request(`/web/gb/groups`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createGbGroupPath: string = "/web/gb/groups";
    /// 新增业务分组
    static async createGbGroup(data: CreateGbGroupReq, headers?: any): Promise<RGbGroup> {
      return request(`/web/gb/groups`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static updateGbGroupPath: string = "/web/gb/groups/{id}";
    /// 编辑业务分组
    static async updateGbGroup(id: string | number, data: UpdateGbGroupReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/groups/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteGbGroupPath: string = "/web/gb/groups/{id}";
    /// 删除业务分组
    static async deleteGbGroup(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/groups/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static gbGroupTreePath: string = "/web/gb/groups/tree";
    /// 业务分组树
    static async gbGroupTree(headers?: any): Promise<RGbGroupTree> {
      return request(`/web/gb/groups/tree`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }
}
