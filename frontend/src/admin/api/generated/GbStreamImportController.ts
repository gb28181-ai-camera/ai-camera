/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { RGbStreamImportResult } from './definitions/RGbStreamImportResult';

export class GbStreamImportController {

    static importGbStreamPushesPath: string = "/web/gb/stream-pushes/import";
    /// 推流配置批量导入
    static async importGbStreamPushes(data?: any, headers?: any): Promise<RGbStreamImportResult> {
      return request(`/web/gb/stream-pushes/import`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: data,
        headers,
      });
    }
}
