/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type GbGroup = Definition & {
	active?: boolean;
	alias?: string;
	business_group?: string;
	civil_code?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	device_id?: string;
	id?: number;
	name?: string;
	parent_device_id?: string;
	parent_id?: number;
	remark?: string;
	sequence?: number;
	updated_by?: string;
	updated_dt?: string;
}
