/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbMergeTaskItem = Definition & {
	created_by?: string;
	created_dt?: string;
	duration_sec?: number;
	error_msg?: string;
	file_name?: string;
	file_path?: string;
	file_size?: number;
	finished_dt?: string;
	id?: number;
	name?: string;
	record_count?: number;
	record_ids?: string;
	started_dt?: string;
	status?: string;
	updated_dt?: string;
}
