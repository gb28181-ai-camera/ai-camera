/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbShareTokenItem = Definition & {
	channel_id?: number;
	created_by?: string;
	created_dt?: string;
	expires_dt?: string;
	id?: number;
	last_used_dt?: string;
	remark?: string;
	revoked?: boolean;
	token_prefix?: string;
	use_count?: number;
}
