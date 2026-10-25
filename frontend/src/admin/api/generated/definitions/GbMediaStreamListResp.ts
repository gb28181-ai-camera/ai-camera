/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { GbMediaStreamItem } from './GbMediaStreamItem';

export type GbMediaStreamListResp = Definition & {
	items?: GbMediaStreamItem[];
	media_server_id?: string;
	message?: string;
	online?: boolean;
	total?: number;
}
