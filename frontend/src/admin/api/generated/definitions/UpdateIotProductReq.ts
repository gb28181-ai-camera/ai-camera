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
	/** DeviceType 为空表示不修改；非空时必须是 security / elderly。 */
	device_type?: string;
	name?: string;
	net_config_mode?: string;
	sequence?: number;
}
