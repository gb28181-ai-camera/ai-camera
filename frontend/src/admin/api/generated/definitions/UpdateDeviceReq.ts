/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateDeviceReq = Definition & {
	active?: boolean;
	device_code?: string;
	device_mode?: string;
	firmware_id?: number;
	firmware_ver?: string;
	info_interval?: number;
	iot_card_id?: number;
	latitude?: string;
	location?: string;
	longitude?: string;
	model_id?: number;
	model_ver?: string;
	monitor_enabled?: boolean;
	name?: string;
	region_id?: number;
	remark?: string;
	storage_total?: number;
	stream_url?: string;
	system_ver?: string;
	work_mode?: string;
}
