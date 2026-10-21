/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type GbStreamPush = Definition & {
	active?: boolean;
	app?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	id?: number;
	media_server_id?: string;
	name?: string;
	push_time?: string;
	pushing?: boolean;
	remark?: string;
	self_push?: boolean;
	start_offline_push?: boolean;
	status?: string;
	stream?: string;
	updated_by?: string;
	updated_dt?: string;
}
