/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { IceServer } from './IceServer';

export type AvViewerTicketResp = Definition & {
	client_id?: string;
	device_code?: string;
	device_id?: number;
	device_name?: string;
	device_online?: boolean;
	expires_in?: number;
	group_code?: string;
	ice_servers?: IceServer[];
	room_id?: string;
	ticket?: string;
	ws_url?: string;
}
