/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbAlarmDetailResp = Definition & {
	alarm_description?: string;
	alarm_method?: number;
	alarm_priority?: string;
	alarm_time?: string;
	alarm_type?: string;
	channel_id?: number;
	channel_name?: string;
	created_dt?: string;
	device_id?: number;
	device_name?: string;
	gb_channel_id?: string;
	gb_device_id?: string;
	handled?: boolean;
	id?: number;
	latitude?: number;
	longitude?: number;
	record_path?: string;
	remark?: string;
	snap_path?: string;
	updated_dt?: string;
}
