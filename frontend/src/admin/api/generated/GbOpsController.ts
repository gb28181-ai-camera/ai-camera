/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { RGbMediaMetrics } from './definitions/RGbMediaMetrics';
import type { RGbMediaMetricsHistory } from './definitions/RGbMediaMetricsHistory';
import type { RGbMediaStreamList } from './definitions/RGbMediaStreamList';
import type { RGbPlatformInfo } from './definitions/RGbPlatformInfo';
import type { RGbStreamPlay } from './definitions/RGbStreamPlay';
import type { RPageGbLog } from './definitions/RPageGbLog';

export class GbOpsController {

    static streamGbAlarmsPath: string = "/web/gb/alarms/stream";
    /// 报警实时推送（SSE）
    static async streamGbAlarms(headers?: any): Promise<string> {
      return request(`/web/gb/alarms/stream`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static pageGbLogsPath: string = "/web/gb/logs";
    /// 日志检索
    static async pageGbLogs(page?: number, page_size?: number, level?: string, keyword?: string, headers?: any): Promise<RPageGbLog> {
      return request(`/web/gb/logs?page=${page ?? "" }&page_size=${page_size ?? "" }&level=${level ?? "" }&keyword=${keyword ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static streamGbLogsPath: string = "/web/gb/logs/stream";
    /// 实时日志（SSE）
    static async streamGbLogs(level?: string, keyword?: string, headers?: any): Promise<string> {
      return request(`/web/gb/logs/stream?level=${level ?? "" }&keyword=${keyword ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static getGbMediaServerMetricsPath: string = "/web/gb/media-servers/{id}/metrics";
    /// 媒体节点当前指标
    static async getGbMediaServerMetrics(id: string | number, headers?: any): Promise<RGbMediaMetrics> {
      return request(`/web/gb/media-servers/${id}/metrics`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static getGbMediaServerMetricsHistoryPath: string = "/web/gb/media-servers/{id}/metrics/history";
    /// 媒体节点指标历史
    static async getGbMediaServerMetricsHistory(id: string | number, range?: string, headers?: any): Promise<RGbMediaMetricsHistory> {
      return request(`/web/gb/media-servers/${id}/metrics/history?range=${range ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static listGbMediaServerStreamsPath: string = "/web/gb/media-servers/{id}/streams";
    /// 媒体节点流会话
    static async listGbMediaServerStreams(id: string | number, headers?: any): Promise<RGbMediaStreamList> {
      return request(`/web/gb/media-servers/${id}/streams`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static getGbPlatformInfoPath: string = "/web/gb/platform-info";
    /// 平台运行信息
    static async getGbPlatformInfo(headers?: any): Promise<RGbPlatformInfo> {
      return request(`/web/gb/platform-info`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static playGbStreamProxyPath: string = "/web/gb/stream-proxies/{id}/play";
    /// 拉流代理播放地址
    static async playGbStreamProxy(id: string | number, headers?: any): Promise<RGbStreamPlay> {
      return request(`/web/gb/stream-proxies/${id}/play`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static playGbStreamPushPath: string = "/web/gb/stream-pushes/{id}/play";
    /// 推流播放地址
    static async playGbStreamPush(id: string | number, headers?: any): Promise<RGbStreamPlay> {
      return request(`/web/gb/stream-pushes/${id}/play`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }
}
