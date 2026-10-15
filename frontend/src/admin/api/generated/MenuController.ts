/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateMenuAPIReq } from './definitions/CreateMenuAPIReq';
import type { CreateMenuReq } from './definitions/CreateMenuReq';
import type { RMenu } from './definitions/RMenu';
import type { RMenuAPI } from './definitions/RMenuAPI';
import type { RMenuAPIList } from './definitions/RMenuAPIList';
import type { RMenuList } from './definitions/RMenuList';
import type { UpdateMenuReq } from './definitions/UpdateMenuReq';

export class MenuController {

    static treeMenusPath: string = "/web/system/menus";
    /// 菜单树列表
    static async treeMenus(platform?: string, headers?: any): Promise<RMenuList> {
      return request(`/web/system/menus?platform=${platform ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createMenuPath: string = "/web/system/menus";
    /// 新增菜单
    static async createMenu(data: CreateMenuReq, headers?: any): Promise<RMenu> {
      return request(`/web/system/menus`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getMenuPath: string = "/web/system/menus/{id}";
    /// 菜单详情
    static async getMenu(id: string | number, headers?: any): Promise<RMenu> {
      return request(`/web/system/menus/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateMenuPath: string = "/web/system/menus/{id}";
    /// 更新菜单
    static async updateMenu(id: string | number, data: UpdateMenuReq, headers?: any): Promise<RMenu> {
      return request(`/web/system/menus/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteMenuPath: string = "/web/system/menus/{id}";
    /// 删除菜单
    static async deleteMenu(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/menus/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static listMenuApisPath: string = "/web/system/menus/{id}/apis";
    /// 菜单接口列表
    static async listMenuApis(id: string | number, headers?: any): Promise<RMenuAPIList> {
      return request(`/web/system/menus/${id}/apis`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createMenuApiPath: string = "/web/system/menus/{id}/apis";
    /// 新增菜单接口
    static async createMenuApi(id: string | number, data: CreateMenuAPIReq, headers?: any): Promise<RMenuAPI> {
      return request(`/web/system/menus/${id}/apis`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteMenuApiPath: string = "/web/system/menus/apis/{apiId}";
    /// 删除菜单接口
    static async deleteMenuApi(apiId: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/menus/apis/${apiId}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    // 【手动保留，生成器不覆盖】页面依赖 tree() 命名，转发到 treeMenus()
    /// 菜单树别名（与 treeMenus 同一端点）
    static async tree(platform?: string, headers?: any): Promise<RMenuList> {
      return MenuController.treeMenus(platform, headers);
    }
}
