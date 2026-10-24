/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type CreateIotProductReq = Definition & {
	active?: boolean;
	allow_share?: boolean;
	auto_network?: boolean;
	category?: string;
	code: string;
	description?: string;
	/** DeviceType 缺省 security；未知值直接拒绝，不做静默回退。 */
	device_type?: string;
	name: string;
	net_config_mode?: string;
	sequence?: number;
}
