/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbPtzCruiseReq = Definition & {
	/** Action start | stop。 */
	action?: string;
	/** GroupID 巡航组号 1–255。 */
	group_id?: number;
}
