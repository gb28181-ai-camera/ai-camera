/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateAlarmSettingReq } from './definitions/CreateAlarmSettingReq';
import type { CreateRiskNotifySettingReq } from './definitions/CreateRiskNotifySettingReq';
import type { RAlarmSetting } from './definitions/RAlarmSetting';
import type { RGetuiConfig } from './definitions/RGetuiConfig';
import type { RPageAlarmSetting } from './definitions/RPageAlarmSetting';
import type { RPageRiskNotifySetting } from './definitions/RPageRiskNotifySetting';
import type { RRiskNotifySetting } from './definitions/RRiskNotifySetting';
import type { RSmsConfig } from './definitions/RSmsConfig';
import type { UpdateAlarmSettingReq } from './definitions/UpdateAlarmSettingReq';
import type { UpdateGetuiReq } from './definitions/UpdateGetuiReq';
import type { UpdateRiskNotifySettingReq } from './definitions/UpdateRiskNotifySettingReq';
import type { UpdateSmsReq } from './definitions/UpdateSmsReq';

export class SettingController {

    static pageAlarmSettingsPath: string = "/web/settings/alarm-settings";
    /// 设备告警设置分页列表（仅当前登录用户）
    static async pageAlarmSettings(page?: number, page_size?: number, keyword?: string, alarm_type_id?: number, headers?: any): Promise<RPageAlarmSetting> {
      return request(`/web/settings/alarm-settings?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&alarm_type_id=${alarm_type_id ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createAlarmSettingPath: string = "/web/settings/alarm-settings";
    /// 创建设备告警设置（归属当前登录用户）
    static async createAlarmSetting(data: CreateAlarmSettingReq, headers?: any): Promise<RAlarmSetting> {
      return request(`/web/settings/alarm-settings`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static updateAlarmSettingPath: string = "/web/settings/alarm-settings/{id}";
    /// 更新设备告警设置（仅本人配置）
    static async updateAlarmSetting(id: string | number, data: UpdateAlarmSettingReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/settings/alarm-settings/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteAlarmSettingPath: string = "/web/settings/alarm-settings/{id}";
    /// 删除设备告警设置（仅本人配置）
    static async deleteAlarmSetting(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/settings/alarm-settings/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static disableAllAlarmPath: string = "/web/settings/alarm-settings/disable-all";
    /// 关闭全部设备告警通知（仅当前登录用户）
    static async disableAllAlarm(headers?: any): Promise<BaseResponse> {
      return request(`/web/settings/alarm-settings/disable-all`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    static getGetuiPath: string = "/web/settings/getui";
    /// 获取个推配置（敏感项脱敏）
    static async getGetui(headers?: any): Promise<RGetuiConfig> {
      return request(`/web/settings/getui`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateGetuiPath: string = "/web/settings/getui";
    /// 更新个推配置（空值与脱敏串不覆盖原值）
    static async updateGetui(data: UpdateGetuiReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/settings/getui`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static pageRiskNotifySettingsPath: string = "/web/settings/risk-notify-settings";
    /// 风险通知设置分页列表（仅当前登录用户）
    static async pageRiskNotifySettings(page?: number, page_size?: number, keyword?: string, headers?: any): Promise<RPageRiskNotifySetting> {
      return request(`/web/settings/risk-notify-settings?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createRiskNotifySettingPath: string = "/web/settings/risk-notify-settings";
    /// 创建风险通知设置（归属当前登录用户）
    static async createRiskNotifySetting(data: CreateRiskNotifySettingReq, headers?: any): Promise<RRiskNotifySetting> {
      return request(`/web/settings/risk-notify-settings`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static updateRiskNotifySettingPath: string = "/web/settings/risk-notify-settings/{id}";
    /// 更新风险通知设置（仅本人配置）
    static async updateRiskNotifySetting(id: string | number, data: UpdateRiskNotifySettingReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/settings/risk-notify-settings/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteRiskNotifySettingPath: string = "/web/settings/risk-notify-settings/{id}";
    /// 删除风险通知设置（仅本人配置）
    static async deleteRiskNotifySetting(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/settings/risk-notify-settings/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static disableAllRiskNotifyPath: string = "/web/settings/risk-notify-settings/disable-all";
    /// 关闭全部风险通知（仅当前登录用户）
    static async disableAllRiskNotify(headers?: any): Promise<BaseResponse> {
      return request(`/web/settings/risk-notify-settings/disable-all`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    static getSmsPath: string = "/web/settings/sms";
    /// 获取短信配置（敏感项脱敏）
    static async getSms(headers?: any): Promise<RSmsConfig> {
      return request(`/web/settings/sms`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateSmsPath: string = "/web/settings/sms";
    /// 更新短信配置（空值与脱敏串不覆盖原值）
    static async updateSms(data: UpdateSmsReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/settings/sms`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }
}
