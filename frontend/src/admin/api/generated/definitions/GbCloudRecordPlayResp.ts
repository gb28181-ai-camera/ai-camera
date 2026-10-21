/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbCloudRecordPlayResp = Definition & {
	app?: string;
	end_time?: string;
	file_name?: string;
	file_path?: string;
	file_size?: number;
	flv?: string;
	hls?: string;
	http_base?: string;
	id?: number;
	media_server_id?: string;
	rtmp?: string;
	rtsp?: string;
	start_time?: string;
	stream?: string;
	time_len?: number;
	webrtc?: string;
	ws_flv?: string;
}
