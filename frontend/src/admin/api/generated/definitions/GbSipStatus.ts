/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbSipStatus = Definition & {
	advertise_addr?: string;
	domain?: string;
	embedded?: boolean;
	enabled?: boolean;
	listen_addr?: string;
	running?: boolean;
	transport?: string;
}
