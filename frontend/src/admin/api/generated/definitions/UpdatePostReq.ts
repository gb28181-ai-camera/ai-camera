/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdatePostReq = Definition & {
	active?: boolean;
	code?: string;
	description?: string;
	name?: string;
	role_ids?: number[];
	sequence?: number;
}
