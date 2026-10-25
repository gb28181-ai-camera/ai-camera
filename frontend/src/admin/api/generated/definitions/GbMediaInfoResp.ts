/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbMediaInfoResp = Definition & {
	alive_second?: number;
	app?: string;
	/** AudioCodec / HasAudio 音频轨信息。 */
	audio_codec?: string;
	/** Bitrate 码率（bit/s）= ZLM bytesSpeed × 8。 */
	bitrate?: number;
	/** 视频轨信息（无视频轨时为空/0）。 */
	codec?: string;
	fps?: number;
	has_audio?: boolean;
	height?: number;
	media_server_id?: string;
	reader_count?: number;
	schema?: string;
	stream?: string;
	/** 播放会话 Call-ID */
	stream_id?: string;
	total_reader_count?: number;
	width?: number;
}
