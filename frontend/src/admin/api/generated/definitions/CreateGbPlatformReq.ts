/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type CreateGbPlatformReq = Definition & {
	active?: boolean;
	address?: string;
	as_message_channel?: boolean;
	auto_push_channel?: boolean;
	catalog_with_group?: number;
	catalog_with_platform?: boolean;
	catalog_with_region?: number;
	character_set?: string;
	civil_code?: string;
	device_gb_id?: string;
	device_ip?: string;
	device_port?: number;
	enable?: boolean;
	expires?: number;
	keep_timeout?: number;
	manufacturer?: string;
	model?: string;
	name: string;
	password?: string;
	ptz?: boolean;
	register_way?: string;
	remark?: string;
	rtcp?: boolean;
	secrecy?: number;
	send_stream_ip?: string;
	server_gb_domain?: string;
	server_gb_id: string;
	server_id?: string;
	server_ip?: string;
	server_port?: number;
	transport?: string;
	username?: string;
}
