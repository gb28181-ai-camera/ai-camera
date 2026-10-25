/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type VoiceMcpToolItem = Definition & {
	active?: boolean;
	created_dt?: string;
	description?: string;
	display_name?: string;
	enabled?: boolean;
	id?: number;
	input_schema?: string;
	name?: string;
	require_confirm?: boolean;
	risk_level?: string;
	server_id?: number;
	source?: string;
	wire_name?: string;
}
