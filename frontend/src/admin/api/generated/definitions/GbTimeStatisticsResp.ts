/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { GbTimeStatisticsPoint } from './GbTimeStatisticsPoint';

export type GbTimeStatisticsResp = Definition & {
	/** AverageSeconds / MaxSeconds / MinSeconds 间隔统计（秒）。 */
	average_seconds?: number;
	/** Count 参与统计的间隔数（报文数 - 1）。 */
	count?: number;
	max_seconds?: number;
	min_seconds?: number;
	points?: GbTimeStatisticsPoint[];
	/** Type keepalive | register。 */
	type?: string;
}
