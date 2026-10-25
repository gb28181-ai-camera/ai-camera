/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbTimeStatisticsPoint = Definition & {
	/** IntervalSeconds 与上一条报文的间隔（秒）；首条为空序列时不产生点。 */
	interval_seconds?: number;
	/** Time 本条报文时间（本地时区，yyyy-MM-dd HH:mm:ss）。 */
	time?: string;
}
