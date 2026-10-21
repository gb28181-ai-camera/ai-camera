/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type Track = Definition & {
	channels?: number;
	codec_id?: number;
	codec_id_name?: string;
	codec_type?: number;
	fps?: number;
	height?: number;
	ready?: boolean;
	sample_rate?: number;
	width?: number;
}
