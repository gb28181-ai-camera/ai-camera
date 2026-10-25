/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbStreamPlayResp = Definition & {
	app?: string;
	flv?: string;
	hls?: string;
	id?: number;
	media_server_id?: string;
	name?: string;
	protocols?: string[];
	rtmp?: string;
	rtsp?: string;
	stream?: string;
	webrtc?: string;
	ws_flv?: string;
}
