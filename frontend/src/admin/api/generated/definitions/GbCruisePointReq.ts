/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbCruisePointReq = Definition & {
	/** GroupID 巡航组号 1–255。 */
	group_id: number;
	/** PresetID 预置位编号 1–255。 */
	preset_id: number;
}
