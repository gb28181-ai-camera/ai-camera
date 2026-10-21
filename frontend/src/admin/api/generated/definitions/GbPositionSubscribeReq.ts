/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbPositionSubscribeReq = Definition & {
	channel_id: number;
	/** Interval 订阅间隔（秒），<=0 时使用 60 秒。 */
	interval?: number;
}
