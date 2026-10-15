/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type CreateDeptReq = Definition & {
	active?: boolean;
	code?: string;
	description?: string;
	leader_id?: number;
	name: string;
	parent_id?: number;
	sequence?: number;
}
