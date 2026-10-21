/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type OriginSock = Definition & {
	identifier?: string;
	local_ip?: string;
	local_port?: number;
	peer_ip?: string;
	peer_port?: number;
}
