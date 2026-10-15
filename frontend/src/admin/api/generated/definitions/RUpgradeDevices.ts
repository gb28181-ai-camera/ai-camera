/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { UpgradeDevicesResp } from './UpgradeDevicesResp';

export type RUpgradeDevices = Definition & {
	code?: number;
	data?: UpgradeDevicesResp;
	message?: string;
}
