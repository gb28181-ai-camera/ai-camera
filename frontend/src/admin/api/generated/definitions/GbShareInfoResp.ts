/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbShareInfoResp = Definition & {
	channel_id?: number;
	channel_name?: string;
	device_name?: string;
	expires_dt?: string;
	gb_channel_id?: string;
	play_endpoint?: string;
	protocols?: string[];
}
