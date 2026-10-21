/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type StreamEvent = Definition & {
	app?: string;
	id?: string;
	ip?: string;
	local_port?: number;
	mediaServerId?: string;
	params?: string;
	port?: number;
	re_use_port?: boolean;
	schema?: string;
	ssrc?: string;
	stream?: string;
	/** 以下字段仅 on_rtp_server_timeout 使用。 */
	stream_id?: string;
	tcpMode?: number;
	vhost?: string;
}
