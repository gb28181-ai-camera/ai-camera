/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { GbVoiceStartReq } from './definitions/GbVoiceStartReq';
import type { GbVoiceStopReq } from './definitions/GbVoiceStopReq';
import type { RGbVoiceSession } from './definitions/RGbVoiceSession';

export class GbVoiceController {

    static startGbBroadcastPath: string = "/web/gb/voice/{channelId}/broadcast/start";
    /// 开始语音广播
    static async startGbBroadcast(channelId: string | number, data?: GbVoiceStartReq, headers?: any): Promise<RGbVoiceSession> {
      return request(`/web/gb/voice/${channelId}/broadcast/start`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static stopGbBroadcastPath: string = "/web/gb/voice/{channelId}/broadcast/stop";
    /// 停止语音广播
    static async stopGbBroadcast(channelId: string | number, data?: GbVoiceStopReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/voice/${channelId}/broadcast/stop`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static startGbTalkPath: string = "/web/gb/voice/{channelId}/talk/start";
    /// 开始语音对讲
    static async startGbTalk(channelId: string | number, data?: GbVoiceStartReq, headers?: any): Promise<RGbVoiceSession> {
      return request(`/web/gb/voice/${channelId}/talk/start`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static stopGbTalkPath: string = "/web/gb/voice/{channelId}/talk/stop";
    /// 停止语音对讲
    static async stopGbTalk(channelId: string | number, data?: GbVoiceStopReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/gb/voice/${channelId}/talk/stop`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }
}
