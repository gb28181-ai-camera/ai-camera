/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type CreateMenuAPIReq = Definition & {
	description?: string;
	/** 枚举值: GET / POST / PUT / DELETE / PATCH */
	method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH' | (string & {});
	path: string;
}
