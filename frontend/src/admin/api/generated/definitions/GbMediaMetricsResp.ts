/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { GbHostMetricsPayload } from './GbHostMetricsPayload';

export type GbMediaMetricsResp = Definition & {
	host?: GbHostMetricsPayload;
	media_server_id?: string;
	media_server_name?: string;
	message?: string;
	online?: boolean;
	sampled_dt?: string;
	stream_count?: number;
}
