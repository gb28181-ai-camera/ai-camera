/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { CreateWebhookReq } from './definitions/CreateWebhookReq';
import type { RBool } from './definitions/RBool';
import type { RPageWebhook } from './definitions/RPageWebhook';
import type { RWebhook } from './definitions/RWebhook';
import type { RWebhookPlatformList } from './definitions/RWebhookPlatformList';
import type { RWebhookTest } from './definitions/RWebhookTest';
import type { TestWebhookInlineReq } from './definitions/TestWebhookInlineReq';
import type { TestWebhookReq } from './definitions/TestWebhookReq';
import type { UpdateWebhookReq } from './definitions/UpdateWebhookReq';

export class WebhookController {

    static pageWebhooksPath: string = "/web/settings/webhooks";
    /// 分页查询 WebHook 配置
    static async pageWebhooks(page?: number, page_size?: number, keyword?: string, platform?: string, active?: boolean, headers?: any): Promise<RPageWebhook> {
      return request(`/web/settings/webhooks?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&platform=${platform ?? "" }&active=${active ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createWebhookPath: string = "/web/settings/webhooks";
    /// 新增 WebHook 配置
    static async createWebhook(data: CreateWebhookReq, headers?: any): Promise<RWebhook> {
      return request(`/web/settings/webhooks`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getWebhookPath: string = "/web/settings/webhooks/{id}";
    /// 查询 WebHook 配置详情
    static async getWebhook(id: string | number, headers?: any): Promise<RWebhook> {
      return request(`/web/settings/webhooks/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateWebhookPath: string = "/web/settings/webhooks/{id}";
    /// 编辑 WebHook 配置
    static async updateWebhook(id: string | number, data: UpdateWebhookReq, headers?: any): Promise<RBool> {
      return request(`/web/settings/webhooks/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteWebhookPath: string = "/web/settings/webhooks/{id}";
    /// 删除 WebHook 配置
    static async deleteWebhook(id: string | number, headers?: any): Promise<RBool> {
      return request(`/web/settings/webhooks/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static disableWebhookPath: string = "/web/settings/webhooks/{id}/disable";
    /// 停用 WebHook 配置
    static async disableWebhook(id: string | number, headers?: any): Promise<RBool> {
      return request(`/web/settings/webhooks/${id}/disable`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    static enableWebhookPath: string = "/web/settings/webhooks/{id}/enable";
    /// 启用 WebHook 配置
    static async enableWebhook(id: string | number, headers?: any): Promise<RBool> {
      return request(`/web/settings/webhooks/${id}/enable`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    static testWebhookPath: string = "/web/settings/webhooks/{id}/test";
    /// 向指定 WebHook 发送测试消息
    static async testWebhook(id: string | number, data?: TestWebhookReq, headers?: any): Promise<RWebhookTest> {
      return request(`/web/settings/webhooks/${id}/test`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static webhookPlatformsPath: string = "/web/settings/webhooks/platforms";
    /// 查询支持的 WebHook 平台
    static async webhookPlatforms(headers?: any): Promise<RWebhookPlatformList> {
      return request(`/web/settings/webhooks/platforms`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static testWebhookInlinePath: string = "/web/settings/webhooks/test";
    /// 即时测试 WebHook 配置（不落库）
    static async testWebhookInline(data: TestWebhookInlineReq, headers?: any): Promise<RWebhookTest> {
      return request(`/web/settings/webhooks/test`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }
}
