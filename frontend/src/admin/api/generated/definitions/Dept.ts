/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type Dept = Definition & {
	active?: boolean;
	children?: Dept[];
	code?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	description?: string;
	id?: number;
	leader_id?: number;
	level?: number;
	name?: string;
	parent_id?: number;
	path?: string;
	sequence?: number;
	updated_by?: string;
	updated_dt?: string;
}
