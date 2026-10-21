/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateGbMediaServerReq = Definition & {
	active?: boolean;
	auto_config?: boolean;
	default_server?: boolean;
	flv_port?: number;
	flv_ssl_port?: number;
	hook_alive_interval?: number;
	hook_ip?: string;
	http_port?: number;
	http_ssl_port?: number;
	ip?: string;
	jtt_proxy_port?: number;
	mp4_port?: number;
	mp4_ssl_port?: number;
	name?: string;
	record_assist_port?: number;
	record_day?: number;
	record_path?: string;
	remark?: string;
	rtmp_port?: number;
	rtmp_ssl_port?: number;
	rtp_enable?: boolean;
	rtp_port_range?: string;
	rtp_proxy_port?: number;
	rtsp_port?: number;
	rtsp_ssl_port?: number;
	sdp_ip?: string;
	secret?: string;
	send_rtp_port_range?: string;
	server_id?: string;
	stream_ip?: string;
	transcode_suffix?: string;
	type?: string;
	ws_flv_port?: number;
	ws_flv_ssl_port?: number;
}
