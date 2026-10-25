/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbDefaultMediaServerSummary = Definition & {
	http_port?: number;
	id?: number;
	ip?: string;
	last_keepalive_dt?: string;
	name?: string;
	online?: boolean;
	server_id?: string;
	stream_ip?: string;
}
