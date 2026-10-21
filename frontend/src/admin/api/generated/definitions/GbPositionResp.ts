/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbPositionResp = Definition & {
	altitude?: number;
	channel_id?: number;
	channel_name?: string;
	device_id?: number;
	device_name?: string;
	direction?: number;
	gb_channel_id?: string;
	id?: number;
	latitude?: number;
	longitude?: number;
	report_time?: string;
	speed?: number;
}
