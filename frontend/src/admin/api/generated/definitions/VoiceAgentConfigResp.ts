/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type VoiceAgentConfigResp = Definition & {
	barge_in?: boolean;
	llm_api_key?: string;
	llm_endpoint?: string;
	llm_max_tokens?: number;
	llm_model?: string;
	llm_provider?: string;
	llm_system_prompt?: string;
	llm_temperature?: number;
	max_session_sec?: number;
	pipecat_endpoint?: string;
	pipecat_token?: string;
	session_idle_timeout?: number;
	stt_api_key?: string;
	stt_endpoint?: string;
	stt_language?: string;
	stt_model?: string;
	stt_provider?: string;
	tts_api_key?: string;
	tts_endpoint?: string;
	tts_provider?: string;
	tts_speed?: number;
	tts_voice?: string;
	vad_params?: string;
	vad_provider?: string;
}
