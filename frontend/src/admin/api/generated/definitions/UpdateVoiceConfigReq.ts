/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateVoiceConfigReq = Definition & {
	active?: boolean;
	audio_name?: string;
	audio_url?: string;
	device_id?: number;
	name?: string;
	region_id?: number;
	remark?: string;
	risk_type_id?: number;
	voice_type?: string;
}
