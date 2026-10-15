/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { AlarmType } from './AlarmType';
import type { DeletedAt } from './DeletedAt';
import type { Device } from './Device';
import type { Region } from './Region';

export type AlarmRecord = Definition & {
	alarm_level?: string;
	alarm_type?: AlarmType;
	alarm_type_id?: number;
	content?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	device?: Device;
	device_code?: string;
	device_id?: number;
	handle_image?: string;
	handle_result?: string;
	handled_dt?: string;
	handler?: string;
	id?: number;
	location?: string;
	occurred_dt?: string;
	read?: boolean;
	read_dt?: string;
	region?: Region;
	region_id?: number;
	status?: string;
	updated_by?: string;
	updated_dt?: string;
}
