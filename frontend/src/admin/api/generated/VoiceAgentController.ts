/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { RGbVoiceProviderList } from './definitions/RGbVoiceProviderList';
import type { RVoiceAgentConfig } from './definitions/RVoiceAgentConfig';
import type { RVoiceAgentTest } from './definitions/RVoiceAgentTest';
import type { TestVoiceAgentReq } from './definitions/TestVoiceAgentReq';
import type { UpdateVoiceAgentReq } from './definitions/UpdateVoiceAgentReq';

export class VoiceAgentController {

    static getVoiceAgentConfigPath: string = "/web/settings/voice-agent";
    /// 读取语音 Agent 配置
    static async getVoiceAgentConfig(headers?: any): Promise<RVoiceAgentConfig> {
      return request(`/web/settings/voice-agent`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateVoiceAgentConfigPath: string = "/web/settings/voice-agent";
    /// 保存语音 Agent 配置
    static async updateVoiceAgentConfig(data: UpdateVoiceAgentReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/settings/voice-agent`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static applyVoiceAgentConfigPath: string = "/web/settings/voice-agent/apply";
    /// 下发配置到 Pipecat
    static async applyVoiceAgentConfig(headers?: any): Promise<BaseResponse> {
      return request(`/web/settings/voice-agent/apply`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: null,
        headers,
      });
    }

    static listVoiceAgentProvidersPath: string = "/web/settings/voice-agent/providers";
    /// 语音 provider 字典
    static async listVoiceAgentProviders(category?: string, headers?: any): Promise<RGbVoiceProviderList> {
      return request(`/web/settings/voice-agent/providers?category=${category ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static testVoiceAgentPath: string = "/web/settings/voice-agent/test";
    /// provider 连通性测试
    static async testVoiceAgent(data: TestVoiceAgentReq, headers?: any): Promise<RVoiceAgentTest> {
      return request(`/web/settings/voice-agent/test`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }
}
