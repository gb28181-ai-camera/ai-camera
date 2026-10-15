/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateDeptReq } from './definitions/CreateDeptReq';
import type { RDept } from './definitions/RDept';
import type { RDeptTree } from './definitions/RDeptTree';
import type { ROptionList } from './definitions/ROptionList';
import type { UpdateDeptReq } from './definitions/UpdateDeptReq';

export class DeptController {

    static treeDeptsPath: string = "/web/system/depts";
    /// 部门树列表
    static async treeDepts(headers?: any): Promise<RDeptTree> {
      return request(`/web/system/depts`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createDeptPath: string = "/web/system/depts";
    /// 新增部门
    static async createDept(data: CreateDeptReq, headers?: any): Promise<RDept> {
      return request(`/web/system/depts`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getDeptPath: string = "/web/system/depts/{id}";
    /// 部门详情
    static async getDept(id: string | number, headers?: any): Promise<RDept> {
      return request(`/web/system/depts/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateDeptPath: string = "/web/system/depts/{id}";
    /// 更新部门
    static async updateDept(id: string | number, data: UpdateDeptReq, headers?: any): Promise<RDept> {
      return request(`/web/system/depts/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteDeptPath: string = "/web/system/depts/{id}";
    /// 删除部门
    static async deleteDept(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/depts/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static deptOptionsPath: string = "/web/system/depts/options";
    /// 部门下拉选项
    static async deptOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/system/depts/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    // 【手动保留，生成器不覆盖】DeptSelect 依赖 options() 命名，转发到 deptOptions()
    /// 部门下拉选项别名（与 deptOptions 同一端点）
    static async options(headers?: any): Promise<ROptionList> {
      return DeptController.deptOptions(headers);
    }

    // 【手动保留，生成器不覆盖】DeptTree / RoleList 依赖 tree() 命名，转发到 treeDepts()
    /// 部门树别名（与 treeDepts 同一端点）
    static async tree(headers?: any): Promise<RDeptTree> {
      return DeptController.treeDepts(headers);
    }
}
