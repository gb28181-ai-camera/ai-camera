/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GenerateApiKeyResp = Definition & {
	full_key?: string;
	id?: number;
	key_prefix?: string;
	name?: string;
}
