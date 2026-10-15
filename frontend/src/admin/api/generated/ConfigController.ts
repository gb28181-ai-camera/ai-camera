/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateConfigReq } from './definitions/CreateConfigReq';
import type { RSystemConfig } from './definitions/RSystemConfig';
import type { RSystemConfigList } from './definitions/RSystemConfigList';
import type { UpdateConfigReq } from './definitions/UpdateConfigReq';

export class ConfigController {

    static listConfigsPath: string = "/web/system/configs";
    /// 系统参数列表
    static async listConfigs(keyword?: string, headers?: any): Promise<RSystemConfigList> {
      return request(`/web/system/configs?keyword=${keyword ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createConfigPath: string = "/web/system/configs";
    /// 新增系统参数
    static async createConfig(data: CreateConfigReq, headers?: any): Promise<RSystemConfig> {
      return request(`/web/system/configs`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static updateConfigPath: string = "/web/system/configs/{id}";
    /// 更新系统参数
    static async updateConfig(id: string | number, data: UpdateConfigReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/configs/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteConfigPath: string = "/web/system/configs/{id}";
    /// 删除系统参数
    static async deleteConfig(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/configs/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static disableConfigPath: string = "/web/system/configs/{id}/disable";
    /// 停用系统参数
    static async disableConfig(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/configs/${id}/disable`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    static enableConfigPath: string = "/web/system/configs/{id}/enable";
    /// 启用系统参数
    static async enableConfig(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/configs/${id}/enable`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }
}
