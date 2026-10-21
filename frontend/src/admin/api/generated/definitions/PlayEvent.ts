/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type PlayEvent = Definition & {
	app?: string;
	id?: string;
	ip?: string;
	mediaServerId?: string;
	params?: string;
	port?: number;
	schema?: string;
	stream?: string;
	vhost?: string;
}
