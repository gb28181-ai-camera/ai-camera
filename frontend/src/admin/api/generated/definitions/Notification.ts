/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type Notification = Definition & {
	category?: string;
	content?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	id?: number;
	read?: boolean;
	read_dt?: string;
	ref_id?: number;
	ref_type?: string;
	title?: string;
	updated_by?: string;
	updated_dt?: string;
	user_id?: number;
}
