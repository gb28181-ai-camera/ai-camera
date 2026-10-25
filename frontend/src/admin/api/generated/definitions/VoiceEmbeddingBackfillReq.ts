/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type VoiceEmbeddingBackfillReq = Definition & {
	/** Limit 本次最多处理多少条（默认 100，上限 500）。 */
	limit?: number;
}
