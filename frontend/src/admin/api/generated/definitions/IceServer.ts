/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type IceServer = Definition & {
	credential?: string;
	expires_at?: number;
	realm?: string;
	ttl?: number;
	urls?: string[];
	username?: string;
}
