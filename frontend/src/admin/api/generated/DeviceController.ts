/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreateDeviceModeReq } from './definitions/CreateDeviceModeReq';
import type { CreateDeviceReq } from './definitions/CreateDeviceReq';
import type { CreateIotCardReq } from './definitions/CreateIotCardReq';
import type { CreateVoiceConfigReq } from './definitions/CreateVoiceConfigReq';
import type { DeviceHeartbeatReq } from './definitions/DeviceHeartbeatReq';
import type { RDevice } from './definitions/RDevice';
import type { RDeviceHistoryChart } from './definitions/RDeviceHistoryChart';
import type { RDeviceMode } from './definitions/RDeviceMode';
import type { RIotCard } from './definitions/RIotCard';
import type { RIotCardReplaceLog } from './definitions/RIotCardReplaceLog';
import type { ROptionList } from './definitions/ROptionList';
import type { RPageDevice } from './definitions/RPageDevice';
import type { RPageDeviceHistory } from './definitions/RPageDeviceHistory';
import type { RPageDeviceMode } from './definitions/RPageDeviceMode';
import type { RPageIotCard } from './definitions/RPageIotCard';
import type { RPageIotCardReplaceLog } from './definitions/RPageIotCardReplaceLog';
import type { RPageVoiceConfig } from './definitions/RPageVoiceConfig';
import type { RSyncTime } from './definitions/RSyncTime';
import type { RVoiceConfig } from './definitions/RVoiceConfig';
import type { ReplaceIotCardReq } from './definitions/ReplaceIotCardReq';
import type { SwitchModeReq } from './definitions/SwitchModeReq';
import type { ToggleMonitorReq } from './definitions/ToggleMonitorReq';
import type { UpdateDeviceModeReq } from './definitions/UpdateDeviceModeReq';
import type { UpdateDeviceReq } from './definitions/UpdateDeviceReq';
import type { UpdateIotCardReq } from './definitions/UpdateIotCardReq';
import type { UpdateVoiceConfigReq } from './definitions/UpdateVoiceConfigReq';

export class DeviceController {

    static pageDevicesPath: string = "/web/devices";
    /// 设备分页列表
    static async pageDevices(page?: number, page_size?: number, keyword?: string, region_id?: number, status?: string, device_mode?: string, work_mode?: string, headers?: any): Promise<RPageDevice> {
      return request(`/web/devices?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&region_id=${region_id ?? "" }&status=${status ?? "" }&device_mode=${device_mode ?? "" }&work_mode=${work_mode ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createDevicePath: string = "/web/devices";
    /// 新增设备
    static async createDevice(data: CreateDeviceReq, headers?: any): Promise<RDevice> {
      return request(`/web/devices`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getDevicePath: string = "/web/devices/{id}";
    /// 设备详情
    static async getDevice(id: string | number, headers?: any): Promise<RDevice> {
      return request(`/web/devices/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateDevicePath: string = "/web/devices/{id}";
    /// 更新设备
    static async updateDevice(id: string | number, data: UpdateDeviceReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/devices/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteDevicePath: string = "/web/devices/{id}";
    /// 删除设备
    static async deleteDevice(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/devices/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static deviceHeartbeatPath: string = "/web/devices/{id}/heartbeat";
    /// 设备心跳上报
    static async deviceHeartbeat(id: string | number, data: DeviceHeartbeatReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/devices/${id}/heartbeat`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deviceHistoryChartPath: string = "/web/devices/{id}/history-chart";
    /// 设备历史曲线
    static async deviceHistoryChart(id: string | number, days?: number, headers?: any): Promise<RDeviceHistoryChart> {
      return request(`/web/devices/${id}/history-chart?days=${days ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static switchDeviceModePath: string = "/web/devices/{id}/mode";
    /// 切换设备模式
    static async switchDeviceMode(id: string | number, data: SwitchModeReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/devices/${id}/mode`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static toggleDeviceMonitorPath: string = "/web/devices/{id}/monitor";
    /// 加入/移出实时监控
    static async toggleDeviceMonitor(id: string | number, data: ToggleMonitorReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/devices/${id}/monitor`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static syncDeviceTimePath: string = "/web/devices/{id}/sync-time";
    /// 同步设备时间
    static async syncDeviceTime(id: string | number, headers?: any): Promise<RSyncTime> {
      return request(`/web/devices/${id}/sync-time`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: null,
        headers,
      });
    }

    static pageDeviceHistoryPath: string = "/web/devices/history";
    /// 设备历史记录分页列表
    static async pageDeviceHistory(page?: number, page_size?: number, device_id?: number, region_id?: number, status?: string, device_code?: string, start_dt?: string, end_dt?: string, headers?: any): Promise<RPageDeviceHistory> {
      return request(`/web/devices/history?page=${page ?? "" }&page_size=${page_size ?? "" }&device_id=${device_id ?? "" }&region_id=${region_id ?? "" }&status=${status ?? "" }&device_code=${device_code ?? "" }&start_dt=${start_dt ?? "" }&end_dt=${end_dt ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static pageIotCardReplaceLogsPath: string = "/web/devices/iot-card-replace-logs";
    /// 换卡记录分页列表
    static async pageIotCardReplaceLogs(page?: number, page_size?: number, region_id?: number, device_id?: number, device_code?: string, card_no?: string, start_dt?: string, end_dt?: string, headers?: any): Promise<RPageIotCardReplaceLog> {
      return request(`/web/devices/iot-card-replace-logs?page=${page ?? "" }&page_size=${page_size ?? "" }&region_id=${region_id ?? "" }&device_id=${device_id ?? "" }&device_code=${device_code ?? "" }&card_no=${card_no ?? "" }&start_dt=${start_dt ?? "" }&end_dt=${end_dt ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static replaceIotCardPath: string = "/web/devices/iot-card-replace-logs";
    /// 设备换卡
    static async replaceIotCard(data: ReplaceIotCardReq, headers?: any): Promise<RIotCardReplaceLog> {
      return request(`/web/devices/iot-card-replace-logs`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static pageIotCardsPath: string = "/web/devices/iot-cards";
    /// 物联网卡分页列表
    static async pageIotCards(page?: number, page_size?: number, keyword?: string, region_id?: number, device_id?: number, status?: string, headers?: any): Promise<RPageIotCard> {
      return request(`/web/devices/iot-cards?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&region_id=${region_id ?? "" }&device_id=${device_id ?? "" }&status=${status ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createIotCardPath: string = "/web/devices/iot-cards";
    /// 新增物联网卡
    static async createIotCard(data: CreateIotCardReq, headers?: any): Promise<RIotCard> {
      return request(`/web/devices/iot-cards`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static updateIotCardPath: string = "/web/devices/iot-cards/{id}";
    /// 更新物联网卡
    static async updateIotCard(id: string | number, data: UpdateIotCardReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/devices/iot-cards/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteIotCardPath: string = "/web/devices/iot-cards/{id}";
    /// 删除物联网卡
    static async deleteIotCard(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/devices/iot-cards/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static iotCardOptionsPath: string = "/web/devices/iot-cards/options";
    /// 物联网卡下拉选项
    static async iotCardOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/devices/iot-cards/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static pageDeviceModesPath: string = "/web/devices/modes";
    /// 设备模式分页列表
    static async pageDeviceModes(page?: number, page_size?: number, keyword?: string, headers?: any): Promise<RPageDeviceMode> {
      return request(`/web/devices/modes?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createDeviceModePath: string = "/web/devices/modes";
    /// 新增设备模式
    static async createDeviceMode(data: CreateDeviceModeReq, headers?: any): Promise<RDeviceMode> {
      return request(`/web/devices/modes`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static updateDeviceModePath: string = "/web/devices/modes/{id}";
    /// 更新设备模式
    static async updateDeviceMode(id: string | number, data: UpdateDeviceModeReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/devices/modes/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteDeviceModePath: string = "/web/devices/modes/{id}";
    /// 删除设备模式
    static async deleteDeviceMode(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/devices/modes/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static deviceModeOptionsPath: string = "/web/devices/modes/options";
    /// 设备模式下拉选项
    static async deviceModeOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/devices/modes/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static deviceOptionsPath: string = "/web/devices/options";
    /// 设备下拉选项
    static async deviceOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/devices/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static pageVoiceConfigsPath: string = "/web/devices/voice-configs";
    /// 语音配置分页列表
    static async pageVoiceConfigs(page?: number, page_size?: number, keyword?: string, voice_type?: string, region_id?: number, device_id?: number, headers?: any): Promise<RPageVoiceConfig> {
      return request(`/web/devices/voice-configs?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&voice_type=${voice_type ?? "" }&region_id=${region_id ?? "" }&device_id=${device_id ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createVoiceConfigPath: string = "/web/devices/voice-configs";
    /// 新增语音配置
    static async createVoiceConfig(data: CreateVoiceConfigReq, headers?: any): Promise<RVoiceConfig> {
      return request(`/web/devices/voice-configs`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static updateVoiceConfigPath: string = "/web/devices/voice-configs/{id}";
    /// 更新语音配置
    static async updateVoiceConfig(id: string | number, data: UpdateVoiceConfigReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/devices/voice-configs/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deleteVoiceConfigPath: string = "/web/devices/voice-configs/{id}";
    /// 删除语音配置
    static async deleteVoiceConfig(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/devices/voice-configs/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }
}
