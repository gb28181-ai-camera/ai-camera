/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { Device } from './Device';
import type { Region } from './Region';
import type { RiskType } from './RiskType';

export type VoiceConfig = Definition & {
	active?: boolean;
	audio_name?: string;
	audio_url?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	device?: Device;
	device_id?: number;
	id?: number;
	name?: string;
	region?: Region;
	region_id?: number;
	remark?: string;
	risk_type?: RiskType;
	risk_type_id?: number;
	updated_by?: string;
	updated_dt?: string;
	voice_type?: string;
}
