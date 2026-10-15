/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeviceChartPoint } from './DeviceChartPoint';
import type { DeviceStatusSummary } from './DeviceStatusSummary';

export type DeviceHistoryChartResp = Definition & {
	battery_points?: DeviceChartPoint[];
	days?: number;
	device_id?: number;
	granularity?: string;
	status_points?: DeviceChartPoint[];
	status_summary?: DeviceStatusSummary;
	storage_points?: DeviceChartPoint[];
	voltage_points?: DeviceChartPoint[];
}
