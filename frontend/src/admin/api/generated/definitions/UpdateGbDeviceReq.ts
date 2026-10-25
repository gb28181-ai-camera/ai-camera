/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateGbDeviceReq = Definition & {
	active?: boolean;
	charset?: string;
	gb_device_id?: string;
	gb_version?: string;
	/** Latitude / Longitude 沿用 ac_device 的 varchar 列，格式校验在 service 层。 */
	latitude?: string;
	local_ip?: string;
	location?: string;
	longitude?: string;
	media_server_id?: string;
	name?: string;
	remark?: string;
	sdp_ip?: string;
	sip_realm?: string;
	sip_transport?: string;
	stream_mode?: string;
}
