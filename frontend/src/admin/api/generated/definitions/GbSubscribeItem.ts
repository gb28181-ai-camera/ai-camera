/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type GbSubscribeItem = Definition & {
	call_id?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	device_id?: number;
	/** DeviceName 归属设备名称（列表展示用，关联 ac_device）。 */
	device_name?: string;
	expire_dt?: string;
	expires?: number;
	gb_device_id?: string;
	id?: number;
	remark?: string;
	status?: string;
	subscribe_dt?: string;
	type?: string;
	updated_by?: string;
	updated_dt?: string;
}
