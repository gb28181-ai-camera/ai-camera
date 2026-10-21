/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { GbRecordPlanChannelItem } from './GbRecordPlanChannelItem';
import type { GbRecordPlanItemResp } from './GbRecordPlanItemResp';

export type GbRecordPlanDetailResp = Definition & {
	active?: boolean;
	channel_count?: number;
	channel_ids?: number[];
	channels?: GbRecordPlanChannelItem[];
	created_dt?: string;
	id?: number;
	item_count?: number;
	items?: GbRecordPlanItemResp[];
	name?: string;
	remark?: string;
	snap?: boolean;
	updated_dt?: string;
}
