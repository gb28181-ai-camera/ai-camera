/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { IotCardTrafficStat } from './IotCardTrafficStat';

export type TrafficStatsResp = Definition & {
	/** Items 全部物联网卡流量明细（剩余流量升序）。 */
	items?: IotCardTrafficStat[];
	left_traffic?: number;
	total_traffic?: number;
	used_traffic?: number;
}
