/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type SystemConfig = Definition & {
	active?: boolean;
	created_by?: string;
	created_dt?: string;
	data_type?: string;
	deleted_at?: DeletedAt;
	description?: string;
	group_name?: string;
	id?: number;
	key?: string;
	name?: string;
	sequence?: number;
	updated_by?: string;
	updated_dt?: string;
	value?: string;
}
