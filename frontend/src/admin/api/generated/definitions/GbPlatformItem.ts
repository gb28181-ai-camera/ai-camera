/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type GbPlatformItem = Definition & {
	active?: boolean;
	address?: string;
	as_message_channel?: boolean;
	auto_push_channel?: boolean;
	catalog_group?: number;
	catalog_with_group?: number;
	catalog_with_platform?: boolean;
	catalog_with_region?: number;
	/** 上级平台的 XML 编码。结论六（GB2312 转码）在级联出/入向同样适用， 不存该值则与 UTF-8 上级平台级联必然乱码。默认 GB2312（对齐 WVP PlatformController:118）。 */
	character_set?: string;
	civil_code?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	device_gb_id?: string;
	device_ip?: string;
	device_port?: number;
	enable?: boolean;
	expires?: number;
	id?: number;
	keep_timeout?: number;
	manufacturer?: string;
	model?: string;
	name?: string;
	password?: string;
	ptz?: boolean;
	register_way?: string;
	remark?: string;
	rtcp?: boolean;
	secrecy?: number;
	send_stream_ip?: string;
	server_gb_domain?: string;
	server_gb_id?: string;
	server_id?: string;
	server_ip?: string;
	server_port?: number;
	status?: string;
	transport?: string;
	updated_by?: string;
	updated_dt?: string;
	username?: string;
}
