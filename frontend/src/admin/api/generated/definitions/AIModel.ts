/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { Firmware } from './Firmware';

export type AIModel = Definition & {
	active?: boolean;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	description?: string;
	file_size?: number;
	file_url?: string;
	firmware?: Firmware;
	firmware_id?: number;
	id?: number;
	md5?: string;
	name?: string;
	release_dt?: string;
	updated_by?: string;
	updated_dt?: string;
	version?: string;
}
