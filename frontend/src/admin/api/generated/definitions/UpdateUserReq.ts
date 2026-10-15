/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateUserReq = Definition & {
	active?: boolean;
	avatar?: string;
	dept_id?: number;
	email?: string;
	is_admin?: boolean;
	name?: string;
	password?: string;
	phone?: string;
	post_id?: number;
	remark?: string;
	role_ids?: number[];
	user_name?: string;
}
