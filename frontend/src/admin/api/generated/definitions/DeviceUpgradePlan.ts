/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { Device } from './Device';

export type DeviceUpgradePlan = Definition & {
	ai_model_id?: number;
	ai_model_name?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	device?: Device;
	device_code?: string;
	device_id?: number;
	device_name?: string;
	firmware_id?: number;
	firmware_name?: string;
	id?: number;
	location?: string;
	plan_dt?: string;
	region_id?: number;
	remark?: string;
	status?: string;
	updated_by?: string;
	updated_dt?: string;
}
