/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateIotProductReq = Definition & {
	active?: boolean;
	allow_share?: boolean;
	auto_network?: boolean;
	category?: string;
	code?: string;
	description?: string;
	name?: string;
	net_config_mode?: string;
	sequence?: number;
}
