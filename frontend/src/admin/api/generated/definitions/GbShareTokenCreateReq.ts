/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbShareTokenCreateReq = Definition & {
	remark?: string;
	/** TTLSeconds 缺省 3600，允许范围 [60, 86400]。 */
	ttl_seconds?: number;
}
