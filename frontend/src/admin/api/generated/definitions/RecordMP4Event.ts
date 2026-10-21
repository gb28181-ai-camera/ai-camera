/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type RecordMP4Event = Definition & {
	app?: string;
	file_name?: string;
	file_path?: string;
	file_size?: number;
	folder?: string;
	mediaServerId?: string;
	params?: string;
	start_time?: number;
	stream?: string;
	time_len?: number;
	url?: string;
	vhost?: string;
}
