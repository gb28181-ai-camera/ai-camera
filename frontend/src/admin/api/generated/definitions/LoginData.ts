/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type LoginData = Definition & {
	cross_dept_ids?: string;
	data_perm?: string;
	dept_id?: number;
	name?: string;
	permissions?: string[];
	token?: string;
	user_id?: number;
}
