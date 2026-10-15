/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type ReportAlarmReq = Definition & {
	alarm_level?: string;
	alarm_type_code: string;
	content?: string;
	device_code: string;
	occurred_dt?: string;
}
