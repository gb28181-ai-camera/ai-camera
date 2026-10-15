/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { Device } from './Device';

export type DeviceHistory = Definition & {
	battery?: number;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	device?: Device;
	device_code?: string;
	device_id?: number;
	id?: number;
	iot_card_no?: string;
	location?: string;
	region_id?: number;
	sampled_dt?: string;
	status?: string;
	storage_free?: number;
	updated_by?: string;
	updated_dt?: string;
	voltage?: string;
}
