/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { RiskType } from './RiskType';

export type RiskNotifySetting = Definition & {
	active?: boolean;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	id?: number;
	notify_freq?: number;
	notify_type?: string;
	notify_user_ids?: string;
	remark?: string;
	risk_type?: RiskType;
	risk_type_id?: number;
	updated_by?: string;
	updated_dt?: string;
	user_id?: number;
}
