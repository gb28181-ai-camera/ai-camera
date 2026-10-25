/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateGbChannelReq = Definition & {
	active?: boolean;
	address?: string;
	channel_type?: number;
	civil_code?: string;
	direction_type?: number;
	enable_broadcast?: boolean;
	group_id?: number;
	has_audio?: boolean;
	latitude?: number;
	/** 设备参数扩展（WVP 对齐）：经纬度、监视方位、位置/房间/用途/补光类型、保密属性。 全部使用指针：0 是合法取值，必须能区分「未传」与「传了 0」。 */
	longitude?: number;
	manufacturer?: string;
	model?: string;
	name?: string;
	owner?: string;
	parent_id?: string;
	password?: string;
	position_type?: number;
	ptz_type?: number;
	record_plan_id?: number;
	region_id?: number;
	remark?: string;
	room_type?: number;
	secrecy?: number;
	status?: string;
	stream_id?: string;
	supply_light_type?: number;
	use_type?: number;
}
