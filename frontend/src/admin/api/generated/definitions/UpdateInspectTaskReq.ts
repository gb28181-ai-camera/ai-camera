/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateInspectTaskReq = Definition & {
	content?: string;
	inspector_id: number;
	location?: string;
	name: string;
	plan_dt?: string;
	region_id?: number;
	remark?: string;
	status?: string;
}
