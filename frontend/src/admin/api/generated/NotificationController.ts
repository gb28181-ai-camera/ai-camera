/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { RPageNotification } from './definitions/RPageNotification';
import type { RUnreadCount } from './definitions/RUnreadCount';

export class NotificationController {

    static pageNotificationsPath: string = "/web/system/notifications";
    /// 站内通知分页列表
    static async pageNotifications(page?: number, page_size?: number, category?: string, read?: boolean, headers?: any): Promise<RPageNotification> {
      return request(`/web/system/notifications?page=${page ?? "" }&page_size=${page_size ?? "" }&category=${category ?? "" }&read=${read ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static deleteNotificationPath: string = "/web/system/notifications/{id}";
    /// 删除通知
    static async deleteNotification(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/notifications/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static markNotificationReadPath: string = "/web/system/notifications/{id}/read";
    /// 标记通知已读
    static async markNotificationRead(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/notifications/${id}/read`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    static markAllNotificationsReadPath: string = "/web/system/notifications/read-all";
    /// 全部标记已读
    static async markAllNotificationsRead(headers?: any): Promise<BaseResponse> {
      return request(`/web/system/notifications/read-all`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    static unreadNotificationCountPath: string = "/web/system/notifications/unread-count";
    /// 未读通知数量
    static async unreadNotificationCount(headers?: any): Promise<RUnreadCount> {
      return request(`/web/system/notifications/unread-count`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }
}
