/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { Region } from './Region';

export type InspectRecord = Definition & {
	content?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	handled_dt?: string;
	id?: number;
	inspector?: string;
	inspector_id?: number;
	location?: string;
	plan_dt?: string;
	region?: Region;
	region_id?: number;
	result?: string;
	result_image?: string;
	status?: string;
	task_id?: number;
	task_name?: string;
	updated_by?: string;
	updated_dt?: string;
}
