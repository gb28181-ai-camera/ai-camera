/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateGbStreamPushReq = Definition & {
	active?: boolean;
	app?: string;
	media_server_id?: string;
	name?: string;
	remark?: string;
	self_push?: boolean;
	start_offline_push?: boolean;
	stream?: string;
}
