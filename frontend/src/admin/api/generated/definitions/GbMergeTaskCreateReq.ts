/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbMergeTaskCreateReq = Definition & {
	/** IDs 为 ac_gb_cloud_record 主键，顺序即合并顺序；服务端会按录像开始时间排序。 */
	ids: number[];
	name?: string;
}
