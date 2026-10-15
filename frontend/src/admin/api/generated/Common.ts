/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { UploadResp } from './definitions/UploadResp';

export class Common {

    static uploadFilePath: string = "/web/upload";
    /// 上传图片或录像
    static async uploadFile(data?: any, headers?: any): Promise<UploadResp> {
      return request(`/web/upload`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: data,
        headers,
      });
    }

    static uploadAudioPath: string = "/web/upload/audio";
    /// 上传语音播报音频
    static async uploadAudio(data?: any, headers?: any): Promise<UploadResp> {
      return request(`/web/upload/audio`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: data,
        headers,
      });
    }
}
