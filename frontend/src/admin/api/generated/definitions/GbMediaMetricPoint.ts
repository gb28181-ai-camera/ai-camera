/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbMediaMetricPoint = Definition & {
	cpu_percent?: number;
	disk_percent?: number;
	load1?: number;
	mem_percent?: number;
	net_rx_rate?: number;
	net_tx_rate?: number;
	online?: boolean;
	stream_count?: number;
	ts?: string;
}
