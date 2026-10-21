/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbSipLog = Definition & {
	call_id?: string;
	cmd_type?: string;
	content?: string;
	created_dt?: string;
	direction?: string;
	elapsed_ms?: number;
	gb_channel_id?: string;
	gb_device_id?: string;
	id?: number;
	local_addr?: string;
	method?: string;
	reason?: string;
	remote_addr?: string;
	result?: string;
	status_code?: number;
}
