/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type CreateMenuReq = Definition & {
	active?: boolean;
	component?: string;
	description?: string;
	hidden?: boolean;
	icon?: string;
	name: string;
	parent_id?: number;
	path?: string;
	permission_code?: string;
	/** 枚举值: pc / mobile */
	platform?: 'pc' | 'mobile' | (string & {});
	sequence?: number;
	/** 枚举值: directory / menu / button */
	type: 'directory' | 'menu' | 'button' | (string & {});
}
