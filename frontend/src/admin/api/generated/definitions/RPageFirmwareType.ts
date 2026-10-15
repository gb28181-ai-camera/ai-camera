/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { PageFirmwareTypeResp } from './PageFirmwareTypeResp';

export type RPageFirmwareType = Definition & {
	code?: number;
	data?: PageFirmwareTypeResp;
	message?: string;
}
