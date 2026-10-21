/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbVoiceSessionResp = Definition & {
	/** AgentEnabled 是否接入 Pipecat 语音 Agent；控制面不可达时降级为 false。 */
	agent_enabled?: boolean;
	/** AudioCodec 固定 G.711A。 */
	audio_codec?: string;
	/** Mode 半双工/全双工。一期默认 half_duplex（设计 §8.1.1：全双工必然自激）。 */
	mode?: string;
	session_id?: string;
	/** SSRC 回推音频的 SSRC（10 位十进制）。 */
	ssrc?: string;
	start_dt?: string;
	/** Type broadcast | talk。 */
	type?: string;
}
