/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbHomePositionResp = Definition & {
	/** Enabled 0 关闭 / 1 开启。 */
	enabled?: number;
	/** PresetIndex 归位调用的预置位编号 0–255。 */
	preset_index?: number;
	/** ResetTime 自动归位时间间隔（秒）。 */
	reset_time?: number;
	supported?: boolean;
}
