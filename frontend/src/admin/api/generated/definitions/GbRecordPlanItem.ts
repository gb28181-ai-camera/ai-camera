/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbRecordPlanItem = Definition & {
	active?: boolean;
	channel_count?: number;
	created_dt?: string;
	id?: number;
	item_count?: number;
	name?: string;
	remark?: string;
	snap?: boolean;
	updated_dt?: string;
}
