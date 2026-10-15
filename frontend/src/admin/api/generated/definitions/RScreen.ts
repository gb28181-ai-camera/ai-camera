/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { ScreenResp } from './ScreenResp';

export type RScreen = Definition & {
	code?: number;
	data?: ScreenResp;
	message?: string;
}
