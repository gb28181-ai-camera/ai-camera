/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbChannelItem = Definition & {
	active?: boolean;
	address?: string;
	channel_type?: number;
	civil_code?: string;
	created_dt?: string;
	device_code?: string;
	device_id?: number;
	device_name?: string;
	direction_type?: number;
	enable_broadcast?: boolean;
	gb_channel_id?: string;
	gb_device_id?: string;
	group_id?: number;
	group_name?: string;
	has_audio?: boolean;
	id?: number;
	is_custom?: boolean;
	latitude?: number;
	/** 设备参数扩展（WVP 对齐）。 */
	longitude?: number;
	manufacturer?: string;
	model?: string;
	name?: string;
	owner?: string;
	parent_id?: string;
	parental?: number;
	position_type?: number;
	ptz_type?: number;
	record_plan_id?: number;
	record_plan_name?: string;
	region_id?: number;
	region_name?: string;
	remark?: string;
	room_type?: number;
	secrecy?: number;
	status?: string;
	stream_id?: string;
	supply_light_type?: number;
	sync_dt?: string;
	updated_dt?: string;
	use_type?: number;
}
