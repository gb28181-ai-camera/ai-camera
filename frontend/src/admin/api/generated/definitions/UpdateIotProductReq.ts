/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateIotProductReq = Definition & {
	active?: boolean;
	allow_share?: boolean;
	auto_network?: boolean;
	/** 配网方式为空表示不修改。 */
	category?: string;
	code?: string;
	description?: string;
	name?: string;
	net_config_mode?: string;
	sequence?: number;
	/** nil 表示不修改服务组绑定；空数组表示清空；非空表示整体替换。 */
	service_group_ids?: number[];
}
