/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbRecordPlanItemReq = Definition & {
	end_time?: string;
	start_time?: string;
	/** 0 周日 ~ 6 周六 */
	week_day?: number;
}
