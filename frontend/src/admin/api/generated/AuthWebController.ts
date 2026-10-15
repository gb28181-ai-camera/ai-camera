/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { LoginReq } from './definitions/LoginReq';
import type { RLogin } from './definitions/RLogin';

export class AuthWebController {

    static loginPath: string = "/web/auth/login";
    /// 管理后台登录
    static async login(data: LoginReq, headers?: any): Promise<RLogin> {
      return request(`/web/auth/login`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }
}
