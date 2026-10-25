/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { GbMediaMetricPoint } from './GbMediaMetricPoint';

export type GbMediaMetricsHistoryResp = Definition & {
	interval_sec?: number;
	items?: GbMediaMetricPoint[];
	media_server_id?: string;
	range?: string;
}
