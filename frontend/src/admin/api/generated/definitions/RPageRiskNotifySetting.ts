/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { PageRiskNotifySettingResp } from './PageRiskNotifySettingResp';

export type RPageRiskNotifySetting = Definition & {
	code?: number;
	data?: PageRiskNotifySettingResp;
	message?: string;
}
