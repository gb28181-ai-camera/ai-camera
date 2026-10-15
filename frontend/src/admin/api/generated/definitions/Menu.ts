/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { MenuAPI } from './MenuAPI';

export type Menu = Definition & {
	active?: boolean;
	apis?: MenuAPI[];
	children?: Menu[];
	component?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	description?: string;
	hidden?: boolean;
	icon?: string;
	id?: number;
	level?: number;
	name?: string;
	parent_id?: number;
	path?: string;
	permission_code?: string;
	platform?: string;
	sequence?: number;
	type?: string;
	updated_by?: string;
	updated_dt?: string;
}
