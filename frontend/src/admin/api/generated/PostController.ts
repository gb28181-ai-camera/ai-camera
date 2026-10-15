/* eslint-disable */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成（生成器：scripts/api-gen-local.mjs），
// 请勿手工修改 —— 任何手工改动都会在下次生成时丢失。
// 需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs。
import request from '../request';
import type { BaseResponse } from './definitions/BaseResponse';
import type { CreatePostReq } from './definitions/CreatePostReq';
import type { ROptionList } from './definitions/ROptionList';
import type { RPagePost } from './definitions/RPagePost';
import type { RPost } from './definitions/RPost';
import type { UpdatePostReq } from './definitions/UpdatePostReq';

export class PostController {

    static pagePostsPath: string = "/web/system/posts";
    /// 岗位分页列表
    static async pagePosts(page?: number, page_size?: number, keyword?: string, active?: boolean, headers?: any): Promise<RPagePost> {
      return request(`/web/system/posts?page=${page ?? "" }&page_size=${page_size ?? "" }&keyword=${keyword ?? "" }&active=${active ?? "" }`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static createPostPath: string = "/web/system/posts";
    /// 新增岗位
    static async createPost(data: CreatePostReq, headers?: any): Promise<RPost> {
      return request(`/web/system/posts`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "POST",
        data: JSON.stringify(data),
        headers,
      });
    }

    static getPostPath: string = "/web/system/posts/{id}";
    /// 岗位详情
    static async getPost(id: string | number, headers?: any): Promise<RPost> {
      return request(`/web/system/posts/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    static updatePostPath: string = "/web/system/posts/{id}";
    /// 更新岗位
    static async updatePost(id: string | number, data: UpdatePostReq, headers?: any): Promise<RPost> {
      return request(`/web/system/posts/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "PUT",
        data: JSON.stringify(data),
        headers,
      });
    }

    static deletePostPath: string = "/web/system/posts/{id}";
    /// 删除岗位
    static async deletePost(id: string | number, headers?: any): Promise<BaseResponse> {
      return request(`/web/system/posts/${id}`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "DELETE",
        headers,
      });
    }

    static postOptionsPath: string = "/web/system/posts/options";
    /// 岗位下拉选项
    static async postOptions(headers?: any): Promise<ROptionList> {
      return request(`/web/system/posts/options`, {
        baseURL: import.meta.env.VITE_API_BASE,
        method: "GET",
        headers,
      });
    }

    // 【手动保留，生成器不覆盖】3 个页面依赖 options() 命名，转发到 postOptions()
    /// 岗位下拉选项别名（与 postOptions 同一端点）
    static async options(headers?: any): Promise<ROptionList> {
      return PostController.postOptions(headers);
    }
}
