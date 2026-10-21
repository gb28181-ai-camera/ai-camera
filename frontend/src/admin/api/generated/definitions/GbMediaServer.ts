/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type GbMediaServer = Definition & {
	active?: boolean;
	auto_config?: boolean;
	created_by?: string;
	created_dt?: string;
	default_server?: boolean;
	deleted_at?: DeletedAt;
	dept_id?: number;
	flv_port?: number;
	flv_ssl_port?: number;
	hook_alive_interval?: number;
	hook_ip?: string;
	/** 端口 */
	http_port?: number;
	http_ssl_port?: number;
	id?: number;
	/** 四个地址：用途不同，多网卡/NAT 场景必须分开配置 */
	ip?: string;
	jtt_proxy_port?: number;
	last_keepalive_dt?: string;
	mp4_port?: number;
	mp4_ssl_port?: number;
	name?: string;
	/** 健康 */
	online?: boolean;
	/** 云端录像的合并/切片/打包下载由独立的 Assist 服务承担（默认端口 18081）。 0 表示未启用，此时云端录像只支持单文件直接下载（见设计文档 §14.5）。 */
	record_assist_port?: number;
	record_day?: number;
	/** 录像 */
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
	/** 收流 */
	secret?: string;
	send_rtp_port_range?: string;
	server_id?: string;
	stream_ip?: string;
	transcode_suffix?: string;
	type?: string;
	updated_by?: string;
	updated_dt?: string;
	ws_flv_port?: number;
	ws_flv_ssl_port?: number;
}
