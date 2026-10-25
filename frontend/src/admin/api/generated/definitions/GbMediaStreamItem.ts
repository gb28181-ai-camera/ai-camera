/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbMediaStreamItem = Definition & {
	alive_second?: number;
	app?: string;
	audio_codec?: string;
	bytes_speed?: number;
	codec?: string;
	create_stamp?: number;
	fps?: number;
	has_audio?: boolean;
	height?: number;
	origin_peer_ip?: string;
	origin_peer_port?: number;
	origin_type?: string;
	origin_url?: string;
	reader_count?: number;
	schema?: string;
	stream?: string;
	vhost?: string;
	width?: number;
}
