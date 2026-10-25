/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { GbMergeTaskCreateReq } from './definitions/GbMergeTaskCreateReq';
import type { RGbMergeTask } from './definitions/RGbMergeTask';
import type { RPageGbMergeTask } from './definitions/RPageGbMergeTask';

export class GbCloudMergeController {

    static pageGbCloudMergeTasksPath: string = "/web/gb/cloud-records/merge-tasks";
    /// 合并任务分页
    static async pageGbCloudMergeTasks(page?: number, page_size?: number, status?: string, keyword?: string, headers?: any): Promise<RPageGbMergeTask> {
      return request(`/web/gb/cloud-records/merge-tasks?page=${page ?? "" }&page_size=${page_size ?? "" }&status=${status ?? "" }&keyword=${keyword ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createGbCloudMergeTaskPath: string = "/web/gb/cloud-records/merge-tasks";
    /// 创建云端录像合并任务
    static async createGbCloudMergeTask(data: GbMergeTaskCreateReq, headers?: any): Promise<RGbMergeTask> {
      return request(`/web/gb/cloud-records/merge-tasks`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getGbCloudMergeTaskPath: string = "/web/gb/cloud-records/merge-tasks/{id}";
    /// 合并任务详情
    static async getGbCloudMergeTask(id: string | number, headers?: any): Promise<RGbMergeTask> {
      return request(`/web/gb/cloud-records/merge-tasks/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static downloadGbCloudMergeTaskPath: string = "/web/gb/cloud-records/merge-tasks/{id}/download";
    /// 下载合并后的录像
    static async downloadGbCloudMergeTask(id: string | number, headers?: any): Promise<any> {
      return request(`/web/gb/cloud-records/merge-tasks/${id}/download`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }
}
