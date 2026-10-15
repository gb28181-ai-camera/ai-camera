/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { Device } from './Device';
import type { Region } from './Region';

export type IotCardReplaceLog = Definition & {
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	device?: Device;
	device_code?: string;
	device_id?: number;
	id?: number;
	location?: string;
	new_card_no?: string;
	new_iot_card_id?: number;
	old_card_no?: string;
	old_iot_card_id?: number;
	operator?: string;
	region?: Region;
	region_id?: number;
	remark?: string;
	replace_dt?: string;
	updated_by?: string;
	updated_dt?: string;
}
