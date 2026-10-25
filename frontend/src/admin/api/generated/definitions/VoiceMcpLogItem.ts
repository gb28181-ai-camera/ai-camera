/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type VoiceMcpLogItem = Definition & {
	args?: string;
	confirmed?: boolean;
	created_by?: string;
	created_dt?: string;
	device_id?: number;
	elapsed_ms?: number;
	error_msg?: string;
	id?: number;
	ip?: string;
	result?: string;
	risk_level?: string;
	server_code?: string;
	session_id?: string;
	status?: string;
	tool_name?: string;
	user_id?: number;
}
