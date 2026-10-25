/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { BindIotProductServiceGroupsReq } from './definitions/BindIotProductServiceGroupsReq';
import type { CreateIotDeviceAccountReq } from './definitions/CreateIotDeviceAccountReq';
import type { CreateIotProductReq } from './definitions/CreateIotProductReq';
import type { CreateIotServiceGroupReq } from './definitions/CreateIotServiceGroupReq';
import type { RIotCredentials } from './definitions/RIotCredentials';
import type { RIotDeviceAccount } from './definitions/RIotDeviceAccount';
import type { RIotDeviceAccountCreated } from './definitions/RIotDeviceAccountCreated';
import type { RIotDeviceActivated } from './definitions/RIotDeviceActivated';
import type { RIotDeviceDetail } from './definitions/RIotDeviceDetail';
import type { RIotDeviceQR } from './definitions/RIotDeviceQR';
import type { RIotDeviceSecret } from './definitions/RIotDeviceSecret';
import type { RIotDeviceShareToken } from './definitions/RIotDeviceShareToken';
import type { RIotDeviceUnbind } from './definitions/RIotDeviceUnbind';
import type { RIotProduct } from './definitions/RIotProduct';
import type { RIotProductServiceCredentials } from './definitions/RIotProductServiceCredentials';
import type { RIotProductServiceList } from './definitions/RIotProductServiceList';
import type { RIotProductUpdated } from './definitions/RIotProductUpdated';
import type { RIotServiceGroup } from './definitions/RIotServiceGroup';
import type { RIotServiceGroupList } from './definitions/RIotServiceGroupList';
import type { RIotServiceList } from './definitions/RIotServiceList';
import type { RIotShareQR } from './definitions/RIotShareQR';
import type { RIotShareTokenRevoked } from './definitions/RIotShareTokenRevoked';
import type { RPageIotDevice } from './definitions/RPageIotDevice';
import type { RPageIotProduct } from './definitions/RPageIotProduct';
import type { RPageIotServiceGroup } from './definitions/RPageIotServiceGroup';
import type { ReplaceIotServicesReq } from './definitions/ReplaceIotServicesReq';
import type { UpdateIotDeviceAccountReq } from './definitions/UpdateIotDeviceAccountReq';
import type { UpdateIotDeviceProductReq } from './definitions/UpdateIotDeviceProductReq';
import type { UpdateIotProductReq } from './definitions/UpdateIotProductReq';
import type { UpdateIotServiceGroupReq } from './definitions/UpdateIotServiceGroupReq';

export class IOTController {

    static pageIotDevicesPath: string = "/web/iot/devices";
    /// IOT设备分页列表
    static async pageIotDevices(page?: number, page_size?: number, keyword?: string, product_id?: number, status?: string, headers?: any): Promise<RPageIotDevice> {
      return request(`/web/iot/devices?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&product_id=${product_id ?? "" }&status=${status ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static getIotDevicePath: string = "/web/iot/devices/{id}";
    /// IOT设备详情
    static async getIotDevice(id: string | number, headers?: any): Promise<RIotDeviceDetail> {
      return request(`/web/iot/devices/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createIotDeviceAccountPath: string = "/web/iot/devices/{id}/accounts";
    /// 新增设备账号
    static async createIotDeviceAccount(id: string | number, data: CreateIotDeviceAccountReq, headers?: any): Promise<RIotDeviceAccountCreated> {
      return request(`/web/iot/devices/${id}/accounts`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static updateIotDeviceAccountPath: string = "/web/iot/devices/{id}/accounts/{accountId}";
    /// 更新设备账号
    static async updateIotDeviceAccount(id: string | number, accountId: string | number, data: UpdateIotDeviceAccountReq, headers?: any): Promise<RIotDeviceAccount> {
      return request(`/web/iot/devices/${id}/accounts/${accountId}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteIotDeviceAccountPath: string = "/web/iot/devices/{id}/accounts/{accountId}";
    /// 删除设备账号
    static async deleteIotDeviceAccount(id: string | number, accountId: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/iot/devices/${id}/accounts/${accountId}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static activateIotDevicePath: string = "/web/iot/devices/{id}/activate";
    /// 手动激活设备主账号
    static async activateIotDevice(id: string | number, headers?: any): Promise<RIotDeviceActivated> {
      return request(`/web/iot/devices/${id}/activate`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static updateIotDeviceProductPath: string = "/web/iot/devices/{id}/product";
    /// 设置设备所属产品
    static async updateIotDeviceProduct(id: string | number, data: UpdateIotDeviceProductReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/iot/devices/${id}/product`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getIotDeviceQRPath: string = "/web/iot/devices/{id}/qr";
    /// 设备二维码
    static async getIotDeviceQR(id: string | number, headers?: any): Promise<RIotDeviceQR> {
      return request(`/web/iot/devices/${id}/qr`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static resetIotDeviceSecretPath: string = "/web/iot/devices/{id}/reset-secret";
    /// 重置设备接入密钥
    static async resetIotDeviceSecret(id: string | number, headers?: any): Promise<RIotDeviceSecret> {
      return request(`/web/iot/devices/${id}/reset-secret`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static createIotDeviceShareTokenPath: string = "/web/iot/devices/{id}/share-token";
    /// 生成设备分享令牌
    static async createIotDeviceShareToken(id: string | number, headers?: any): Promise<RIotDeviceShareToken> {
      return request(`/web/iot/devices/${id}/share-token`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static revokeIotDeviceShareTokenPath: string = "/web/iot/devices/{id}/share-token";
    /// 撤销设备分享令牌
    static async revokeIotDeviceShareToken(id: string | number, headers?: any): Promise<RIotShareTokenRevoked> {
      return request(`/web/iot/devices/${id}/share-token`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static unbindIotDevicePath: string = "/web/iot/devices/{id}/unbind";
    /// 解绑设备（删除全部账号）
    static async unbindIotDevice(id: string | number, headers?: any): Promise<RIotDeviceUnbind> {
      return request(`/web/iot/devices/${id}/unbind`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static pageIotProductsPath: string = "/web/iot/products";
    /// IOT产品分页列表
    static async pageIotProducts(page?: number, page_size?: number, keyword?: string, active?: boolean, headers?: any): Promise<RPageIotProduct> {
      return request(`/web/iot/products?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&active=${active ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createIotProductPath: string = "/web/iot/products";
    /// 新增IOT产品
    static async createIotProduct(data: CreateIotProductReq, headers?: any): Promise<RIotProduct> {
      return request(`/web/iot/products`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getIotProductPath: string = "/web/iot/products/{id}";
    /// IOT产品详情
    static async getIotProduct(id: string | number, headers?: any): Promise<RIotProduct> {
      return request(`/web/iot/products/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateIotProductPath: string = "/web/iot/products/{id}";
    /// 更新IOT产品
    static async updateIotProduct(id: string | number, data: UpdateIotProductReq, headers?: any): Promise<RIotProductUpdated> {
      return request(`/web/iot/products/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteIotProductPath: string = "/web/iot/products/{id}";
    /// 删除IOT产品
    static async deleteIotProduct(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/iot/products/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static getIotProductCredentialsPath: string = "/web/iot/products/{id}/credentials";
    /// 产品明文鉴权信息
    static async getIotProductCredentials(id: string | number, headers?: any): Promise<RIotCredentials> {
      return request(`/web/iot/products/${id}/credentials`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static getIotProductServiceGroupsPath: string = "/web/iot/products/{id}/service-groups";
    /// 查询产品已绑定服务组
    static async getIotProductServiceGroups(id: string | number, headers?: any): Promise<RIotServiceGroupList> {
      return request(`/web/iot/products/${id}/service-groups`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static bindIotProductServiceGroupsPath: string = "/web/iot/products/{id}/service-groups";
    /// 绑定产品服务组
    static async bindIotProductServiceGroups(id: string | number, data: BindIotProductServiceGroupsReq, headers?: any): Promise<RIotServiceGroupList> {
      return request(`/web/iot/products/${id}/service-groups`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getIotProductServicesPath: string = "/web/iot/products/{id}/services";
    /// 产品服务凭据列表
    static async getIotProductServices(id: string | number, headers?: any): Promise<RIotProductServiceList> {
      return request(`/web/iot/products/${id}/services`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static resetIotProductServiceCredentialsPath: string = "/web/iot/products/{id}/services/{serviceId}/reset-credentials";
    /// 重置产品服务凭据
    static async resetIotProductServiceCredentials(id: string | number, serviceId: string | number, headers?: any): Promise<RIotProductServiceCredentials> {
      return request(`/web/iot/products/${id}/services/${serviceId}/reset-credentials`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static generateIotProductShareQRPath: string = "/web/iot/products/{id}/share-qr";
    /// 生成产品分享二维码内容
    static async generateIotProductShareQR(id: string | number, headers?: any): Promise<RIotShareQR> {
      return request(`/web/iot/products/${id}/share-qr`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static pageIotServiceGroupsPath: string = "/web/iot/service-groups";
    /// IOT服务组分页列表
    static async pageIotServiceGroups(page?: number, page_size?: number, keyword?: string, type?: string, active?: boolean, headers?: any): Promise<RPageIotServiceGroup> {
      return request(`/web/iot/service-groups?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&type=${type ?? "" }&active=${active ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createIotServiceGroupPath: string = "/web/iot/service-groups";
    /// 新增IOT服务组
    static async createIotServiceGroup(data: CreateIotServiceGroupReq, headers?: any): Promise<RIotServiceGroup> {
      return request(`/web/iot/service-groups`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getIotServiceGroupPath: string = "/web/iot/service-groups/{id}";
    /// IOT服务组详情
    static async getIotServiceGroup(id: string | number, headers?: any): Promise<RIotServiceGroup> {
      return request(`/web/iot/service-groups/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateIotServiceGroupPath: string = "/web/iot/service-groups/{id}";
    /// 更新IOT服务组
    static async updateIotServiceGroup(id: string | number, data: UpdateIotServiceGroupReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/iot/service-groups/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteIotServiceGroupPath: string = "/web/iot/service-groups/{id}";
    /// 删除IOT服务组
    static async deleteIotServiceGroup(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/iot/service-groups/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static replaceIotServiceGroupServicesPath: string = "/web/iot/service-groups/{id}/services";
    /// 整体替换服务组服务
    static async replaceIotServiceGroupServices(id: string | number, data: ReplaceIotServicesReq, headers?: any): Promise<RIotServiceList> {
      return request(`/web/iot/service-groups/${id}/services`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }
}
