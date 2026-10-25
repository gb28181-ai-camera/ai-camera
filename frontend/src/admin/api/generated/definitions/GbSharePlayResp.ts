/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbSharePlayResp = Definition & {
	app?: string;
	channel_id?: number;
	channel_name?: string;
	expires_in_sec?: number;
	flv?: string;
	hls?: string;
	protocol?: string;
	stream?: string;
	webrtc?: string;
	ws_flv?: string;
}
