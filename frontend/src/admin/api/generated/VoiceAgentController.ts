/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { RGbVoiceProviderList } from './definitions/RGbVoiceProviderList';
import type { RPageVoiceMcpLog } from './definitions/RPageVoiceMcpLog';
import type { RPageVoiceMcpTool } from './definitions/RPageVoiceMcpTool';
import type { RPageVoiceMemory } from './definitions/RPageVoiceMemory';
import type { RVoiceAgentConfig } from './definitions/RVoiceAgentConfig';
import type { RVoiceAgentTest } from './definitions/RVoiceAgentTest';
import type { RVoiceWsTicket } from './definitions/RVoiceWsTicket';
import type { TestVoiceAgentReq } from './definitions/TestVoiceAgentReq';
import type { UpdateVoiceAgentReq } from './definitions/UpdateVoiceAgentReq';
import type { UpdateVoiceMcpToolReq } from './definitions/UpdateVoiceMcpToolReq';
import type { VoiceWsTicketReq } from './definitions/VoiceWsTicketReq';

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

    static pageVoiceMcpLogsPath: string = "/web/voice/mcp/logs";
    /// MCP 调用审计
    static async pageVoiceMcpLogs(page?: number, page_size?: number, tool_name?: string, status?: string, user_id?: number, device_id?: number, session_id?: string, start_dt?: string, end_dt?: string, headers?: any): Promise<RPageVoiceMcpLog> {
      return request(`/web/voice/mcp/logs?page=${page ?? "" }&page_size=${page_size ?? "" }&tool_name=${tool_name ?? "" }&status=${status ?? "" }&user_id=${user_id ?? "" }&device_id=${device_id ?? "" }&session_id=${session_id ?? "" }&start_dt=${start_dt ?? "" }&end_dt=${end_dt ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static pageVoiceMcpToolsPath: string = "/web/voice/mcp/tools";
    /// MCP 工具列表
    static async pageVoiceMcpTools(page?: number, page_size?: number, keyword?: string, source?: string, risk_level?: string, enabled?: boolean, headers?: any): Promise<RPageVoiceMcpTool> {
      return request(`/web/voice/mcp/tools?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&source=${source ?? "" }&risk_level=${risk_level ?? "" }&enabled=${enabled ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updateVoiceMcpToolPath: string = "/web/voice/mcp/tools/{id}";
    /// 更新 MCP 工具策略
    static async updateVoiceMcpTool(id: string | number, data: UpdateVoiceMcpToolReq, headers?: any): Promise<BaseResponse> {
      return request(`/web/voice/mcp/tools/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static pageVoiceMemoriesPath: string = "/web/voice/memories";
    /// 语音记忆列表
    static async pageVoiceMemories(page?: number, page_size?: number, owner_type?: string, owner_key?: string, kind?: string, keyword?: string, embedding_status?: string, headers?: any): Promise<RPageVoiceMemory> {
      return request(`/web/voice/memories?page=${page ?? "" }&page_size=${page_size ?? "" }&owner_type=${owner_type ?? "" }&owner_key=${owner_key ?? "" }&kind=${kind ?? "" }&keyword=${keyword ?? "" }&embedding_status=${embedding_status ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static deleteVoiceMemoryPath: string = "/web/voice/memories/{id}";
    /// 删除语音记忆
    static async deleteVoiceMemory(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/voice/memories/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static exportVoiceMemoriesPath: string = "/web/voice/memories/export";
    /// 导出语音记忆
    static async exportVoiceMemories(owner_type?: string, owner_key?: string, kind?: string, keyword?: string, headers?: any): Promise<any> {
      return request(`/web/voice/memories/export?owner_type=${owner_type ?? "" }&owner_key=${owner_key ?? "" }&kind=${kind ?? "" }&keyword=${keyword ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createWebVoiceWsTicketPath: string = "/web/voice/ws-ticket";
    /// 申请语音 WS ticket
    static async createWebVoiceWsTicket(data?: VoiceWsTicketReq, headers?: any): Promise<RVoiceWsTicket> {
      return request(`/web/voice/ws-ticket`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }
}
