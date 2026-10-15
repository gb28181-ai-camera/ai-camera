/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type CreateAlarmSettingReq = Definition & {
	active?: boolean;
	alarm_type_id: number;
	notify_freq?: number;
	notify_type?: string;
	notify_user_ids?: string;
	remark?: string;
	threshold?: string;
}
