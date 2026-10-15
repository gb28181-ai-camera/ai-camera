/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type OperationLog = Definition & {
	action?: string;
	cost_ms?: number;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	error_msg?: string;
	id?: number;
	ip?: string;
	location?: string;
	method?: string;
	module?: string;
	operator?: string;
	params?: string;
	path?: string;
	status?: number;
	updated_by?: string;
	updated_dt?: string;
}
