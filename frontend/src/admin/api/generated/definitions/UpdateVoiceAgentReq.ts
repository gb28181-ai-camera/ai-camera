/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type UpdateVoiceAgentReq = Definition & {
	barge_in?: boolean;
	/** ExtraParams provider param_schema 中没有固定键的扩展参数（如 TTS 的 model、 Azure 的 region）。键为原始参数名（前端不带前缀），服务端统一加 `param_` 前缀落库，避免与上面的固定键冲突；nil 表示不修改。 */
	extra_params?: Record<string, any>;
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
