/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { Device } from './Device';

export type DeviceUpgradeRecord = Definition & {
	ai_model_id?: number;
	ai_model_ver?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	device?: Device;
	device_code?: string;
	device_id?: number;
	device_name?: string;
	error_msg?: string;
	firmware_id?: number;
	firmware_ver?: string;
	id?: number;
	iot_card_no?: string;
	location?: string;
	operator?: string;
	prev_ai_model_id?: number;
	prev_ai_model_ver?: string;
	prev_firmware_id?: number;
	prev_firmware_ver?: string;
	region_id?: number;
	status?: string;
	updated_by?: string;
	updated_dt?: string;
	upgrade_dt?: string;
	upgrade_type?: string;
}
