/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbSnapshotResp = Definition & {
	capture_dt?: string;
	file_name?: string;
	file_size?: number;
	media_server_id?: string;
	stream?: string;
	url?: string;
}
