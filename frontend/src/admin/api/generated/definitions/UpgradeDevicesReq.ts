/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpgradeDevicesReq = Definition & {
	ai_model_id?: number;
	device_ids: number[];
	firmware_id?: number;
	plan_dt?: string;
	/** 枚举值: now / plan */
	upgrade_type: 'now' | 'plan' | (string & {});
}
