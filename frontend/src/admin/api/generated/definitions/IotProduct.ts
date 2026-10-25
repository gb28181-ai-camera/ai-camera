/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type IotProduct = Definition & {
	active?: boolean;
	allow_share?: boolean;
	auto_network?: boolean;
	category?: string;
	code?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	description?: string;
	id?: number;
	name?: string;
	net_config_mode?: string;
	sequence?: number;
	share_qr_content?: string;
	updated_by?: string;
	updated_dt?: string;
}
