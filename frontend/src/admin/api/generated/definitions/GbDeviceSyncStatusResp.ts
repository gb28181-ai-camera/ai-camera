/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbDeviceSyncStatusResp = Definition & {
	current?: number;
	device_id?: number;
	error?: string;
	/** FinishedDT 本轮同步结束时间（成功落库或失败/超时）。 */
	finished_dt?: string;
	gb_device_id?: string;
	/** StartedDT 本轮同步开始时间（收到第一个分片时）。 */
	started_dt?: string;
	/** Status idle（无同步记录）/ syncing / success / failed。 */
	status?: string;
	total?: number;
}
