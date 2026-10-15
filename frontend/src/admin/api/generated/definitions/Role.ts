/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { Menu } from './Menu';

export type Role = Definition & {
	active?: boolean;
	code?: string;
	created_by?: string;
	created_dt?: string;
	cross_dept_ids?: string;
	data_permission?: string;
	deleted_at?: DeletedAt;
	description?: string;
	id?: number;
	menus?: Menu[];
	name?: string;
	sequence?: number;
	updated_by?: string;
	updated_dt?: string;
}
