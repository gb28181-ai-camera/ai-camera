/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateConfigReq = Definition & {
	active?: boolean;
	/** 枚举值: string / int / integer / float / bool / boolean / json / select */
	data_type?: 'string' | 'int' | 'integer' | 'float' | 'bool' | 'boolean' | 'json' | 'select' | (string & {});
	description?: string;
	group_name?: string;
	key?: string;
	name?: string;
	schema?: string;
	sequence?: number;
	value?: string;
}
