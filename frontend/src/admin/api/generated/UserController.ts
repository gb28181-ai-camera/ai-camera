/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateUserReq } from './definitions/CreateUserReq';
import type { ROptionList } from './definitions/ROptionList';
import type { RPageUser } from './definitions/RPageUser';
import type { RUser } from './definitions/RUser';
import type { ResetPasswordReq } from './definitions/ResetPasswordReq';
import type { UpdateUserReq } from './definitions/UpdateUserReq';

export class UserController {

    static pageUsersPath: string = "/web/system/users";
    /// 用户分页列表
    static async pageUsers(page?: number, page_size?: number, keyword?: string, dept_id?: number, active?: boolean, post_id?: number, headers?: any): Promise<RPageUser> {
      return request(`/web/system/users?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&dept_id=${dept_id ?? "" }&active=${active ?? "" }&post_id=${post_id ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createUserPath: string = "/web/system/users";
    /// 创建用户
    static async createUser(data: CreateUserReq, headers?: any): Promise<RUser> {
      return request(`/web/system/users`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getUserPath: string = "/web/system/users/{id}";
    /// 用户详情
    static async getUser(id: string | number, headers?: any): Promise<RUser> {
      return request(`/web/system/users/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateUserPath: string = "/web/system/users/{id}";
    /// 更新用户
    static async updateUser(id: string | number, data: UpdateUserReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/users/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteUserPath: string = "/web/system/users/{id}";
    /// 删除用户
    static async deleteUser(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/users/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static disableUserPath: string = "/web/system/users/{id}/disable";
    /// 禁用用户
    static async disableUser(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/users/${id}/disable`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    static enableUserPath: string = "/web/system/users/{id}/enable";
    /// 启用用户
    static async enableUser(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/users/${id}/enable`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    static resetUserPasswordPath: string = "/web/system/users/{id}/reset-password";
    /// 重置用户密码
    static async resetUserPassword(id: string | number, data: ResetPasswordReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/users/${id}/reset-password`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static userOptionsPath: string = "/web/system/users/options";
    /// 用户下拉选项
    static async userOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/system/users/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    // 【手动保留，生成器不覆盖】UserSelect 依赖 options() 命名，转发到 userOptions()
    /// 用户下拉选项别名（与 userOptions 同一端点）
    static async options(headers?: any): Promise<ROptionList> {
      return UserController.userOptions(headers);
    }
}
