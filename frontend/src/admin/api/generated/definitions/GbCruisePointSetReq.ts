/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbCruisePointSetReq = Definition & {
	group_id: number;
	/** Speed 巡航速度，标准 12 bit 量纲（1–4095）。 */
	speed?: number;
	/** StayTime 巡航点停留时间（秒），标准 12 bit 量纲（1–4095）。 */
	stay_time?: number;
}
