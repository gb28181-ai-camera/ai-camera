/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type AlarmType = Definition & {
	active?: boolean;
	code?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	description?: string;
	id?: number;
	level?: string;
	name?: string;
	sequence?: number;
	updated_by?: string;
	updated_dt?: string;
}
