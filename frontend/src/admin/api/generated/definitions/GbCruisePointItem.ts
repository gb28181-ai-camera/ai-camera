/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbCruisePointItem = Definition & {
	/** PresetIndex 预置位编号。 */
	preset_index?: number;
	/** Speed 云台速度 1–15。 */
	speed?: number;
	/** StayTime 停留时间（秒）。 */
	stay_time?: number;
}
