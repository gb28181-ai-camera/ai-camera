/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type VoiceMemoryItem = Definition & {
	content?: string;
	content_hash?: string;
	created_by?: string;
	created_dt?: string;
	dept_id?: number;
	embedding_status?: string;
	expires_dt?: string;
	id?: number;
	importance?: number;
	kind?: string;
	last_access_dt?: string;
	owner_id?: number;
	owner_key?: string;
	owner_type?: string;
	source_session_id?: string;
	turn_id?: string;
	user_id?: number;
}
