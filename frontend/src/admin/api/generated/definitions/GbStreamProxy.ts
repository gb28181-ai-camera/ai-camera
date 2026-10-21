/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type GbStreamProxy = Definition & {
	active?: boolean;
	app?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	enable?: boolean;
	enable_audio?: boolean;
	enable_disable_none_reader?: boolean;
	enable_mp4?: boolean;
	enable_remove_none_reader?: boolean;
	ffmpeg_cmd_key?: string;
	id?: number;
	media_server_id?: string;
	name?: string;
	pulling?: boolean;
	remark?: string;
	rtsp_type?: number;
	src_url?: string;
	stream?: string;
	stream_key?: string;
	timeout_ms?: number;
	type?: string;
	updated_by?: string;
	updated_dt?: string;
}
