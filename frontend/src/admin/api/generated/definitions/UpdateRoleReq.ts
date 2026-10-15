/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateRoleReq = Definition & {
	active?: boolean;
	code?: string;
	cross_dept_ids?: string;
	data_permission?: string;
	description?: string;
	menu_ids?: number[];
	name?: string;
	sequence?: number;
}
