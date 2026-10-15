/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type ReportRiskReq = Definition & {
	detect_score?: number;
	device_code: string;
	image_url?: string;
	occurred_dt?: string;
	risk_type_code: string;
	video_url?: string;
}
