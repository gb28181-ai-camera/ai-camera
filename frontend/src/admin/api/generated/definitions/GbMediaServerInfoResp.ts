/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { GbMediaServerThreadLoad } from './GbMediaServerThreadLoad';

export type GbMediaServerInfoResp = Definition & {
	active_streams?: number;
	avg_load?: number;
	loads?: GbMediaServerThreadLoad[];
	max_load?: number;
	message?: string;
	online?: boolean;
	thread_count?: number;
	version?: string;
}
