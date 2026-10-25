/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { GbSharePlayReq } from './definitions/GbSharePlayReq';
import type { GbShareTokenCreateReq } from './definitions/GbShareTokenCreateReq';
import type { RGbShareInfo } from './definitions/RGbShareInfo';
import type { RGbSharePlay } from './definitions/RGbSharePlay';
import type { RGbShareTokenCreated } from './definitions/RGbShareTokenCreated';
import type { RGbShareTokenList } from './definitions/RGbShareTokenList';

export class GbShareController {

    static getPublicGbShareInfoPath: string = "/public/gb/shares/{token}";
    /// 匿名分享信息
    static async getPublicGbShareInfo(token: string | number, headers?: any): Promise<RGbShareInfo> {
      return request(`/public/gb/shares/${token}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static playPublicGbSharePath: string = "/public/gb/shares/{token}/play";
    /// 匿名分享点播
    static async playPublicGbShare(token: string | number, data?: GbSharePlayReq, headers?: any): Promise<RGbSharePlay> {
      return request(`/public/gb/shares/${token}/play`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static listGbShareTokensPath: string = "/web/gb/channels/{id}/share-tokens";
    /// 分享令牌列表
    static async listGbShareTokens(id: string | number, headers?: any): Promise<RGbShareTokenList> {
      return request(`/web/gb/channels/${id}/share-tokens`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createGbShareTokenPath: string = "/web/gb/channels/{id}/share-tokens";
    /// 生成通道分享令牌
    static async createGbShareToken(id: string | number, data?: GbShareTokenCreateReq, headers?: any): Promise<RGbShareTokenCreated> {
      return request(`/web/gb/channels/${id}/share-tokens`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static revokeGbShareTokenPath: string = "/web/gb/channels/{id}/share-tokens/{tokenId}";
    /// 吊销分享令牌
    static async revokeGbShareToken(id: string | number, tokenId: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/channels/${id}/share-tokens/${tokenId}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }
}
