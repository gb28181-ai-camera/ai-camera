/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbCruiseTrackItem = Definition & {
	/** Name 轨迹名称。 */
	name?: string;
	/** Number 轨迹编号，从 0 开始。 */
	number?: number;
}
