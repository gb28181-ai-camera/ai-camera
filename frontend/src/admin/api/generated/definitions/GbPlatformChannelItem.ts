/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbPlatformChannelItem = Definition & {
	channel_id?: number;
	channel_name?: string;
	created_dt?: string;
	custom_address?: string;
	custom_channel_type?: number;
	custom_civil_code?: string;
	custom_device_id?: string;
	custom_manufacturer?: string;
	custom_model?: string;
	custom_name?: string;
	custom_owner?: string;
	custom_parent_id?: string;
	custom_parental?: number;
	custom_ptz_type?: number;
	custom_status?: string;
	device_code?: string;
	device_name?: string;
	gb_channel_id?: string;
	id?: number;
	platform_id?: number;
	updated_dt?: string;
}
