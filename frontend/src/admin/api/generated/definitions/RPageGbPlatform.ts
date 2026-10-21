/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { PageGbPlatformResp } from './PageGbPlatformResp';

export type RPageGbPlatform = Definition & {
	code?: number;
	data?: PageGbPlatformResp;
	message?: string;
}
