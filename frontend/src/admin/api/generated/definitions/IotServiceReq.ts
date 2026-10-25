/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type IotServiceReq = Definition & {
	active?: boolean;
	code: string;
	extra?: string;
	host?: string;
	name: string;
	path?: string;
	port?: number;
	protocol: string;
	sequence?: number;
}
