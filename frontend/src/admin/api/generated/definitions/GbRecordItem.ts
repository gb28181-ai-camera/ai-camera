/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbRecordItem = Definition & {
	address?: string;
	channel_id?: number;
	device_id?: number;
	end_time?: string;
	file_path?: string;
	gb_channel_id?: string;
	name?: string;
	secrecy?: number;
	start_time?: string;
	time_len?: number;
	type?: string;
}
