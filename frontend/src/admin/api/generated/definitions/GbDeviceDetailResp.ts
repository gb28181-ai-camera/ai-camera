/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { IotServiceGroupBrief } from './IotServiceGroupBrief';
import type { Region } from './Region';

export type GbDeviceDetailResp = Definition & {
	active?: boolean;
	channel_count?: number;
	charset?: string;
	created_dt?: string;
	device_code?: string;
	/** ── 国标接入 13 列 ── */
	gb_device_id?: string;
	gb_version?: string;
	id?: number;
	latitude?: string;
	local_ip?: string;
	location?: string;
	longitude?: string;
	media_server_id?: string;
	/** MediaServerName 绑定节点的展示名（media_server_id 为 auto 时为空）。 */
	media_server_name?: string;
	name?: string;
	product_code?: string;
	/** ── IOT 归属（同一台设备可同时属于 GB28181 + AIChat 等多个服务组）── */
	product_id?: number;
	product_name?: string;
	region?: Region;
	region_id?: number;
	region_name?: string;
	register_dt?: string;
	register_status?: string;
	remark?: string;
	sdp_ip?: string;
	service_groups?: IotServiceGroupBrief[];
	sip_expires?: string;
	sip_password?: string;
	sip_realm?: string;
	sip_transport?: string;
	status?: string;
	stream_mode?: string;
	updated_dt?: string;
}
