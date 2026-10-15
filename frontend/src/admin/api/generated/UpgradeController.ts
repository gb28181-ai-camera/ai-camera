/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { BatchIDReq } from './definitions/BatchIDReq';
import type { CreateAIModelReq } from './definitions/CreateAIModelReq';
import type { CreateFirmwareReq } from './definitions/CreateFirmwareReq';
import type { CreateFirmwareTypeReq } from './definitions/CreateFirmwareTypeReq';
import type { RAIModel } from './definitions/RAIModel';
import type { RFirmware } from './definitions/RFirmware';
import type { RFirmwareType } from './definitions/RFirmwareType';
import type { ROptionList } from './definitions/ROptionList';
import type { RPageAIModel } from './definitions/RPageAIModel';
import type { RPageFirmware } from './definitions/RPageFirmware';
import type { RPageFirmwareType } from './definitions/RPageFirmwareType';
import type { RPageUpgradePlan } from './definitions/RPageUpgradePlan';
import type { RPageUpgradeRecord } from './definitions/RPageUpgradeRecord';
import type { RUpgradeDevices } from './definitions/RUpgradeDevices';
import type { UpdateAIModelReq } from './definitions/UpdateAIModelReq';
import type { UpdateFirmwareReq } from './definitions/UpdateFirmwareReq';
import type { UpdateFirmwareTypeReq } from './definitions/UpdateFirmwareTypeReq';
import type { UpgradeDevicesReq } from './definitions/UpgradeDevicesReq';

export class UpgradeController {

    static upgradeDevicesPath: string = "/web/upgrades/devices";
    /// 设备升级下发（现在升级 / 定时升级）
    static async upgradeDevices(data: UpgradeDevicesReq, headers?: any): Promise<RUpgradeDevices> {
      return request(`/web/upgrades/devices`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static pageFirmwareTypesPath: string = "/web/upgrades/firmware-types";
    /// 固件类型分页列表
    static async pageFirmwareTypes(page?: number, page_size?: number, keyword?: string, headers?: any): Promise<RPageFirmwareType> {
      return request(`/web/upgrades/firmware-types?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createFirmwareTypePath: string = "/web/upgrades/firmware-types";
    /// 创建固件类型
    static async createFirmwareType(data: CreateFirmwareTypeReq, headers?: any): Promise<RFirmwareType> {
      return request(`/web/upgrades/firmware-types`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static updateFirmwareTypePath: string = "/web/upgrades/firmware-types/{id}";
    /// 更新固件类型
    static async updateFirmwareType(id: string | number, data: UpdateFirmwareTypeReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/upgrades/firmware-types/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteFirmwareTypePath: string = "/web/upgrades/firmware-types/{id}";
    /// 删除固件类型
    static async deleteFirmwareType(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/upgrades/firmware-types/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static firmwareTypeOptionsPath: string = "/web/upgrades/firmware-types/options";
    /// 固件类型下拉选项
    static async firmwareTypeOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/upgrades/firmware-types/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static pageFirmwaresPath: string = "/web/upgrades/firmwares";
    /// 固件分页列表
    static async pageFirmwares(page?: number, page_size?: number, keyword?: string, firmware_type_id?: number, headers?: any): Promise<RPageFirmware> {
      return request(`/web/upgrades/firmwares?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&firmware_type_id=${firmware_type_id ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createFirmwarePath: string = "/web/upgrades/firmwares";
    /// 创建固件
    static async createFirmware(data: CreateFirmwareReq, headers?: any): Promise<RFirmware> {
      return request(`/web/upgrades/firmwares`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getFirmwarePath: string = "/web/upgrades/firmwares/{id}";
    /// 固件详情
    static async getFirmware(id: string | number, headers?: any): Promise<RFirmware> {
      return request(`/web/upgrades/firmwares/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateFirmwarePath: string = "/web/upgrades/firmwares/{id}";
    /// 更新固件
    static async updateFirmware(id: string | number, data: UpdateFirmwareReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/upgrades/firmwares/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteFirmwarePath: string = "/web/upgrades/firmwares/{id}";
    /// 删除固件
    static async deleteFirmware(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/upgrades/firmwares/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static firmwareOptionsPath: string = "/web/upgrades/firmwares/options";
    /// 固件下拉选项
    static async firmwareOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/upgrades/firmwares/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static pageModelsPath: string = "/web/upgrades/models";
    /// AI模型分页列表
    static async pageModels(page?: number, page_size?: number, keyword?: string, firmware_id?: number, headers?: any): Promise<RPageAIModel> {
      return request(`/web/upgrades/models?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&firmware_id=${firmware_id ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createAIModelPath: string = "/web/upgrades/models";
    /// 创建AI模型
    static async createAIModel(data: CreateAIModelReq, headers?: any): Promise<RAIModel> {
      return request(`/web/upgrades/models`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getAIModelPath: string = "/web/upgrades/models/{id}";
    /// AI模型详情
    static async getAIModel(id: string | number, headers?: any): Promise<RAIModel> {
      return request(`/web/upgrades/models/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateAIModelPath: string = "/web/upgrades/models/{id}";
    /// 更新AI模型
    static async updateAIModel(id: string | number, data: UpdateAIModelReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/upgrades/models/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteAIModelPath: string = "/web/upgrades/models/{id}";
    /// 删除AI模型
    static async deleteAIModel(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/upgrades/models/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static modelOptionsPath: string = "/web/upgrades/models/options";
    /// AI模型下拉选项
    static async modelOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/upgrades/models/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static pageUpgradePlansPath: string = "/web/upgrades/plans";
    /// 升级计划分页列表
    static async pageUpgradePlans(page?: number, page_size?: number, keyword?: string, region_id?: number, device_id?: number, status?: string, headers?: any): Promise<RPageUpgradePlan> {
      return request(`/web/upgrades/plans?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&region_id=${region_id ?? "" }&device_id=${device_id ?? "" }&status=${status ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static cancelUpgradePlanPath: string = "/web/upgrades/plans/{id}/cancel";
    /// 取消升级计划
    static async cancelUpgradePlan(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/upgrades/plans/${id}/cancel`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    static batchCancelUpgradePlansPath: string = "/web/upgrades/plans/cancel";
    /// 批量取消升级计划
    static async batchCancelUpgradePlans(data: BatchIDReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/upgrades/plans/cancel`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static pageUpgradeRecordsPath: string = "/web/upgrades/records";
    /// 升级记录分页列表
    static async pageUpgradeRecords(page?: number, page_size?: number, keyword?: string, region_id?: number, device_id?: number, start_dt?: string, end_dt?: string, headers?: any): Promise<RPageUpgradeRecord> {
      return request(`/web/upgrades/records?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&region_id=${region_id ?? "" }&device_id=${device_id ?? "" }&start_dt=${start_dt ?? "" }&end_dt=${end_dt ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }
}
