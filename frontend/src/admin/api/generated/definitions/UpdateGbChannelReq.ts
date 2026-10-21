/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateGbChannelReq = Definition & {
	active?: boolean;
	address?: string;
	channel_type?: number;
	civil_code?: string;
	enable_broadcast?: boolean;
	group_id?: number;
	has_audio?: boolean;
	manufacturer?: string;
	model?: string;
	name?: string;
	owner?: string;
	parent_id?: string;
	password?: string;
	ptz_type?: number;
	record_plan_id?: number;
	region_id?: number;
	remark?: string;
	status?: string;
	stream_id?: string;
}
