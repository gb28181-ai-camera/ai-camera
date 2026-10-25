/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type VoiceEmbeddingTestResp = Definition & {
	dim?: number;
	elapsed_ms?: number;
	endpoint?: string;
	message?: string;
	model?: string;
	success?: boolean;
}
