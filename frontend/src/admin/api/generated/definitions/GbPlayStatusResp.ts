/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbPlayStatusResp = Definition & {
	app?: string;
	channel_id?: number;
	flv?: string;
	hls?: string;
	http_base?: string;
	media_server_id?: string;
	playing?: boolean;
	ready?: boolean;
	ssrc?: string;
	start_dt?: string;
	stream?: string;
	stream_id?: string;
	webrtc?: string;
}
