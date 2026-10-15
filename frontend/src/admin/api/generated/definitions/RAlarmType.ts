/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { AlarmType } from './AlarmType';

export type RAlarmType = Definition & {
	code?: number;
	data?: AlarmType;
	message?: string;
}
