/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateConfigReq = Definition & {
	/** 枚举值: string / int / bool / json */
	data_type?: 'string' | 'int' | 'bool' | 'json' | (string & {});
	description?: string;
	group_name?: string;
	key?: string;
	name?: string;
	sequence?: number;
	value?: string;
}
