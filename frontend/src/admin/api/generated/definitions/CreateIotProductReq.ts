/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type CreateIotProductReq = Definition & {
	active?: boolean;
	allow_share?: boolean;
	auto_network?: boolean;
	/** 配网方式由产品配置：qrcode（无需配网）/ bluetooth / ap。 */
	category?: string;
	code: string;
	description?: string;
	name: string;
	net_config_mode?: string;
	sequence?: number;
	/** 产品可选的服务组（系统预置三条）。创建时同步绑定，并为组内服务生成凭据。 */
	service_group_ids?: number[];
}
