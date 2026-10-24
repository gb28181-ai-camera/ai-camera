/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { GbSDCardItem } from './GbSDCardItem';

export type GbSDCardStatusResp = Definition & {
	cards?: GbSDCardItem[];
	/** FormatProgress 格式化进度 0–100；0 表示未在格式化。 */
	format_progress?: number;
	/** Num 卡片数量。 */
	num?: number;
	/** Status 整体状态。★ 取值含义待真机确认，原样透传。 */
	status?: number;
}
