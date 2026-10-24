/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type IotServiceItem = Definition & {
	active?: boolean;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	extra?: string;
	group_id?: number;
	host?: string;
	id?: number;
	path?: string;
	port?: number;
	protocol?: string;
	sequence?: number;
	updated_by?: string;
	updated_dt?: string;
}
