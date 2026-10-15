/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { AssignIDsReq } from './definitions/AssignIDsReq';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateRoleReq } from './definitions/CreateRoleReq';
import type { RIDList } from './definitions/RIDList';
import type { ROptionList } from './definitions/ROptionList';
import type { RPageRole } from './definitions/RPageRole';
import type { RRole } from './definitions/RRole';
import type { UpdateRoleReq } from './definitions/UpdateRoleReq';

export class RoleController {

    static pageRolesPath: string = "/web/system/roles";
    /// 角色分页列表
    static async pageRoles(page?: number, page_size?: number, keyword?: string, active?: boolean, headers?: any): Promise<RPageRole> {
      return request(`/web/system/roles?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&active=${active ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createRolePath: string = "/web/system/roles";
    /// 创建角色
    static async createRole(data: CreateRoleReq, headers?: any): Promise<RRole> {
      return request(`/web/system/roles`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getRolePath: string = "/web/system/roles/{id}";
    /// 角色详情
    static async getRole(id: string | number, headers?: any): Promise<RRole> {
      return request(`/web/system/roles/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateRolePath: string = "/web/system/roles/{id}";
    /// 更新角色
    static async updateRole(id: string | number, data: UpdateRoleReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/roles/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteRolePath: string = "/web/system/roles/{id}";
    /// 删除角色
    static async deleteRole(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/roles/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static getRoleDeptsPath: string = "/web/system/roles/{id}/depts";
    /// 获取角色关联的部门
    static async getRoleDepts(id: string | number, headers?: any): Promise<RIDList> {
      return request(`/web/system/roles/${id}/depts`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static assignRoleDeptsPath: string = "/web/system/roles/{id}/depts";
    /// 分配角色部门
    static async assignRoleDepts(id: string | number, data: AssignIDsReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/roles/${id}/depts`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getRoleMenusPath: string = "/web/system/roles/{id}/menus";
    /// 获取角色已授权的菜单
    static async getRoleMenus(id: string | number, headers?: any): Promise<RIDList> {
      return request(`/web/system/roles/${id}/menus`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static assignRoleMenusPath: string = "/web/system/roles/{id}/menus";
    /// 分配角色菜单
    static async assignRoleMenus(id: string | number, data: AssignIDsReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/roles/${id}/menus`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getRolePostsPath: string = "/web/system/roles/{id}/posts";
    /// 获取角色关联的岗位
    static async getRolePosts(id: string | number, headers?: any): Promise<RIDList> {
      return request(`/web/system/roles/${id}/posts`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static assignRolePostsPath: string = "/web/system/roles/{id}/posts";
    /// 分配角色岗位
    static async assignRolePosts(id: string | number, data: AssignIDsReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/roles/${id}/posts`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getRoleUsersPath: string = "/web/system/roles/{id}/users";
    /// 获取角色关联的用户
    static async getRoleUsers(id: string | number, headers?: any): Promise<RIDList> {
      return request(`/web/system/roles/${id}/users`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static assignRoleUsersPath: string = "/web/system/roles/{id}/users";
    /// 分配角色用户
    static async assignRoleUsers(id: string | number, data: AssignIDsReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/roles/${id}/users`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static roleOptionsPath: string = "/web/system/roles/options";
    /// 角色下拉选项
    static async roleOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/system/roles/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    // 【手动保留，生成器不覆盖】RoleList / UserList 依赖 options() 命名，转发到 roleOptions()
    /// 角色下拉选项别名（与 roleOptions 同一端点）
    static async options(headers?: any): Promise<ROptionList> {
      return RoleController.roleOptions(headers);
    }
}
