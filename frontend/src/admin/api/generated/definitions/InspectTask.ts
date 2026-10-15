/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { Region } from './Region';

export type InspectTask = Definition & {
	active?: boolean;
	content?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	id?: number;
	inspector_id?: number;
	location?: string;
	name?: string;
	plan_dt?: string;
	read?: boolean;
	region?: Region;
	region_id?: number;
	remark?: string;
	status?: string;
	updated_by?: string;
	updated_dt?: string;
}
