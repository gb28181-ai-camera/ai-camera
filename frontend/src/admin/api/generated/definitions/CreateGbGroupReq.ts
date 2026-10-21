/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type CreateGbGroupReq = Definition & {
	active?: boolean;
	alias?: string;
	business_group?: string;
	civil_code?: string;
	device_id?: string;
	name: string;
	parent_device_id?: string;
	parent_id?: number;
	remark?: string;
	sequence?: number;
}
