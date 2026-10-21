/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbPlaybackResp = Definition & {
	app?: string;
	channel_id?: number;
	end_time?: string;
	flv?: string;
	hls?: string;
	http_base?: string;
	media_server_id?: string;
	ssrc?: string;
	start_time?: string;
	stream?: string;
	stream_id?: string;
	webrtc?: string;
	ws_flv?: string;
}
