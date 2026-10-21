/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbCloudRecordItem = Definition & {
	app?: string;
	call_id?: string;
	channel_id?: number;
	channel_name?: string;
	collect?: boolean;
	created_dt?: string;
	device_id?: number;
	device_name?: string;
	end_time?: string;
	file_name?: string;
	file_path?: string;
	file_size?: number;
	folder?: string;
	id?: number;
	media_server_id?: string;
	remark?: string;
	start_time?: string;
	stream?: string;
	time_len?: number;
	updated_dt?: string;
}
