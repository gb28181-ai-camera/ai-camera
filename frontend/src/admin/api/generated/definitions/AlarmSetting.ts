/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { AlarmType } from './AlarmType';
import type { DeletedAt } from './DeletedAt';

export type AlarmSetting = Definition & {
	active?: boolean;
	alarm_type?: AlarmType;
	alarm_type_id?: number;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	id?: number;
	notify_freq?: number;
	notify_type?: string;
	/** 通知人员（多个用户 ID，逗号分隔） */
	notify_user_ids?: string;
	remark?: string;
	/** 阈值：电量告警阈值（%）、流量告警阈值（MB），按告警类型取用 */
	threshold?: string;
	updated_by?: string;
	updated_dt?: string;
	user_id?: number;
}
