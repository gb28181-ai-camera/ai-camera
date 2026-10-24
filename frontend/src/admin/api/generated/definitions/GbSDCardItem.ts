/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbSDCardItem = Definition & {
	/** Capacity 总空间（MB）。 */
	capacity?: number;
	/** FreeSpace 剩余空间（MB）。 */
	free_space?: number;
	/** Status 卡状态。★ 标准未明确取值含义，原样透传。 */
	status?: number;
}
