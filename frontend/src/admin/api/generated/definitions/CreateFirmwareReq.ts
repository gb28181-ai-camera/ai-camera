/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type CreateFirmwareReq = Definition & {
	active?: boolean;
	description?: string;
	file_size?: number;
	file_url?: string;
	firmware_type_id?: number;
	md5?: string;
	name: string;
	release_dt?: string;
	version: string;
}
