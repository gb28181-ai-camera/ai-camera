/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { ChangePasswordReq } from './definitions/ChangePasswordReq';
import type { GenerateApiKeyReq } from './definitions/GenerateApiKeyReq';
import type { RApiKeyList } from './definitions/RApiKeyList';
import type { RGenerateApiKey } from './definitions/RGenerateApiKey';
import type { RMenuList } from './definitions/RMenuList';
import type { RUser } from './definitions/RUser';
import type { UpdateProfileReq } from './definitions/UpdateProfileReq';

export class ProfileController {

    static getProfilePath: string = "/web/profile";
    /// 获取当前用户信息
    static async getProfile(headers?: any): Promise<RUser> {
      return request(`/web/profile`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateProfilePath: string = "/web/profile";
    /// 更新当前用户信息
    static async updateProfile(data: UpdateProfileReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/profile`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static listApiKeysPath: string = "/web/profile/api-keys";
    /// 获取当前用户 API Key 列表
    static async listApiKeys(headers?: any): Promise<RApiKeyList> {
      return request(`/web/profile/api-keys`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static generateApiKeyPath: string = "/web/profile/api-keys";
    /// 生成 API Key
    static async generateApiKey(data: GenerateApiKeyReq, headers?: any): Promise<RGenerateApiKey> {
      return request(`/web/profile/api-keys`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteApiKeyPath: string = "/web/profile/api-keys/{id}";
    /// 删除 API Key
    static async deleteApiKey(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/profile/api-keys/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static getProfileMenusPath: string = "/web/profile/menus";
    /// 获取当前用户菜单树
    static async getProfileMenus(headers?: any): Promise<RMenuList> {
      return request(`/web/profile/menus`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static changePasswordPath: string = "/web/profile/password";
    /// 修改当前用户密码
    static async changePassword(data: ChangePasswordReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/profile/password`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    // 【手动保留，生成器不覆盖】MainLayout 侧边栏依赖 getCurrentUserMenus() 命名，转发到 getProfileMenus()（GET /web/profile/menus）
    /// 菜单树别名（与 getProfileMenus 同一端点）
    static async getCurrentUserMenus(headers?: any): Promise<RMenuList> {
      return ProfileController.getProfileMenus(headers)
    }
}
