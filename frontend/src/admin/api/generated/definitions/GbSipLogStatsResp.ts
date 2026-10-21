/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { GbSipLogStatItem } from './GbSipLogStatItem';

export type GbSipLogStatsResp = Definition & {
	by_cmd_type?: GbSipLogStatItem[];
	by_method?: GbSipLogStatItem[];
	failed?: number;
	inbound?: number;
	outbound?: number;
	rejected?: number;
	success?: number;
	total?: number;
}
