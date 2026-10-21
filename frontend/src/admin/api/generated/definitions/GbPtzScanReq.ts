/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbPtzScanReq = Definition & {
	/** Action start | stop。 */
	action?: string;
	/** GroupID 扫描组号 1–255（部分设备用组号选择扫描路线）。 */
	group_id?: number;
}
