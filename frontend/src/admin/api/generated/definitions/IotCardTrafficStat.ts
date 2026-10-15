/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type IotCardTrafficStat = Definition & {
	card_no?: string;
	device_name?: string;
	name?: string;
	region_name?: string;
	status?: string;
	traffic_left?: number;
	traffic_total?: number;
	traffic_used?: number;
}
