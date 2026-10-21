/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type VoiceProviderItem = Definition & {
	category?: string;
	code?: string;
	default_params?: string;
	id?: number;
	name?: string;
	param_schema?: string;
	remark?: string;
	sequence?: number;
}
