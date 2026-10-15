/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateRiskTypeReq = Definition & {
	active?: boolean;
	code?: string;
	description?: string;
	/** 枚举值: high / medium / low */
	level?: 'high' | 'medium' | 'low' | (string & {});
	name: string;
	sequence?: number;
}
