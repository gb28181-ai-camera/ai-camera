/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbHostMetricsPayload = Definition & {
	cpu_cores?: number;
	cpu_percent?: number;
	disk_percent?: number;
	disk_total?: number;
	disk_used?: number;
	load1?: number;
	load15?: number;
	load5?: number;
	mem_percent?: number;
	mem_total?: number;
	mem_used?: number;
	net_rx_bytes?: number;
	net_rx_rate?: number;
	net_tx_bytes?: number;
	net_tx_rate?: number;
	/** OSInfo 形如 "linux/amd64" 或 "windows/amd64"。 */
	os_info?: string;
}
