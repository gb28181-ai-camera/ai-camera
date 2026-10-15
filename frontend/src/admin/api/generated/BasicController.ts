/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { CreateAlarmTypeReq } from './definitions/CreateAlarmTypeReq';
import type { CreateRegionReq } from './definitions/CreateRegionReq';
import type { CreateRiskTypeReq } from './definitions/CreateRiskTypeReq';
import type { RAlarmType } from './definitions/RAlarmType';
import type { RBool } from './definitions/RBool';
import type { ROptionList } from './definitions/ROptionList';
import type { RPageAlarmType } from './definitions/RPageAlarmType';
import type { RPageRegion } from './definitions/RPageRegion';
import type { RPageRiskType } from './definitions/RPageRiskType';
import type { RRegion } from './definitions/RRegion';
import type { RRiskType } from './definitions/RRiskType';
import type { UpdateAlarmTypeReq } from './definitions/UpdateAlarmTypeReq';
import type { UpdateRegionReq } from './definitions/UpdateRegionReq';
import type { UpdateRiskTypeReq } from './definitions/UpdateRiskTypeReq';

export class BasicController {

    static pageAlarmTypesPath: string = "/web/basic/alarm-types";
    /// 告警类型分页列表
    static async pageAlarmTypes(page?: number, page_size?: number, keyword?: string, level?: string, headers?: any): Promise<RPageAlarmType> {
      return request(`/web/basic/alarm-types?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&level=${level ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createAlarmTypePath: string = "/web/basic/alarm-types";
    /// 新增告警类型
    static async createAlarmType(data: CreateAlarmTypeReq, headers?: any): Promise<RAlarmType> {
      return request(`/web/basic/alarm-types`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getAlarmTypePath: string = "/web/basic/alarm-types/{id}";
    /// 告警类型详情
    static async getAlarmType(id: string | number, headers?: any): Promise<RAlarmType> {
      return request(`/web/basic/alarm-types/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateAlarmTypePath: string = "/web/basic/alarm-types/{id}";
    /// 修改告警类型
    static async updateAlarmType(id: string | number, data: UpdateAlarmTypeReq, headers?: any): Promise<RBool> {
      return request(`/web/basic/alarm-types/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteAlarmTypePath: string = "/web/basic/alarm-types/{id}";
    /// 删除告警类型
    static async deleteAlarmType(id: string | number, headers?: any): Promise<RBool> {
      return request(`/web/basic/alarm-types/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static alarmTypeOptionsPath: string = "/web/basic/alarm-types/options";
    /// 告警类型下拉选项
    static async alarmTypeOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/basic/alarm-types/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static pageRegionsPath: string = "/web/basic/regions";
    /// 区域分页列表
    static async pageRegions(page?: number, page_size?: number, keyword?: string, headers?: any): Promise<RPageRegion> {
      return request(`/web/basic/regions?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createRegionPath: string = "/web/basic/regions";
    /// 新增区域
    static async createRegion(data: CreateRegionReq, headers?: any): Promise<RRegion> {
      return request(`/web/basic/regions`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getRegionPath: string = "/web/basic/regions/{id}";
    /// 区域详情
    static async getRegion(id: string | number, headers?: any): Promise<RRegion> {
      return request(`/web/basic/regions/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateRegionPath: string = "/web/basic/regions/{id}";
    /// 修改区域
    static async updateRegion(id: string | number, data: UpdateRegionReq, headers?: any): Promise<RBool> {
      return request(`/web/basic/regions/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteRegionPath: string = "/web/basic/regions/{id}";
    /// 删除区域
    static async deleteRegion(id: string | number, headers?: any): Promise<RBool> {
      return request(`/web/basic/regions/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static regionOptionsPath: string = "/web/basic/regions/options";
    /// 区域下拉选项
    static async regionOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/basic/regions/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static pageRiskTypesPath: string = "/web/basic/risk-types";
    /// 风险类型分页列表
    static async pageRiskTypes(page?: number, page_size?: number, keyword?: string, level?: string, headers?: any): Promise<RPageRiskType> {
      return request(`/web/basic/risk-types?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&level=${level ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createRiskTypePath: string = "/web/basic/risk-types";
    /// 新增风险类型
    static async createRiskType(data: CreateRiskTypeReq, headers?: any): Promise<RRiskType> {
      return request(`/web/basic/risk-types`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getRiskTypePath: string = "/web/basic/risk-types/{id}";
    /// 风险类型详情
    static async getRiskType(id: string | number, headers?: any): Promise<RRiskType> {
      return request(`/web/basic/risk-types/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateRiskTypePath: string = "/web/basic/risk-types/{id}";
    /// 修改风险类型
    static async updateRiskType(id: string | number, data: UpdateRiskTypeReq, headers?: any): Promise<RBool> {
      return request(`/web/basic/risk-types/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteRiskTypePath: string = "/web/basic/risk-types/{id}";
    /// 删除风险类型
    static async deleteRiskType(id: string | number, headers?: any): Promise<RBool> {
      return request(`/web/basic/risk-types/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static riskTypeOptionsPath: string = "/web/basic/risk-types/options";
    /// 风险类型下拉选项
    static async riskTypeOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/basic/risk-types/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }
}
