/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateGbStreamProxyReq = Definition & {
	active?: boolean;
	app?: string;
	enable?: boolean;
	enable_audio?: boolean;
	enable_disable_none_reader?: boolean;
	enable_mp4?: boolean;
	enable_remove_none_reader?: boolean;
	ffmpeg_cmd_key?: string;
	media_server_id?: string;
	name?: string;
	remark?: string;
	rtsp_type?: number;
	src_url?: string;
	stream?: string;
	timeout_ms?: number;
	type?: string;
}
