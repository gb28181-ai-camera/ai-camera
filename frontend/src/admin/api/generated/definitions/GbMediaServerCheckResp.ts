/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbMediaServerCheckResp = Definition & {
	http_port?: number;
	media_server_id?: string;
	message?: string;
	online?: boolean;
	response_elapsed_ms?: number;
	rtcp_port?: number;
	rtp_port?: number;
	secret_matched?: boolean;
	version?: string;
}
