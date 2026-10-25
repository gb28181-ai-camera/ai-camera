/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type VoiceAgentConfigResp = Definition & {
	barge_in?: boolean;
	device_wake_word?: string;
	embedding_api_key?: string;
	embedding_endpoint?: string;
	embedding_model?: string;
	/** ExtraParams 扩展参数：provider param_schema 中没有固定键的项。 键为原始参数名（已去掉落库时的 param_ 前缀），敏感键按 **** 脱敏。 */
	extra_params?: Record<string, any>;
	fast_ack_text?: string;
	internal_mcp_token?: string;
	llm_api_key?: string;
	llm_endpoint?: string;
	llm_max_tokens?: number;
	llm_model?: string;
	llm_provider?: string;
	llm_system_prompt?: string;
	llm_temperature?: number;
	max_session_sec?: number;
	mcp_enabled?: boolean;
	mcp_server_url?: string;
	memory_enabled?: boolean;
	memory_top_k?: number;
	memory_ttl_days?: number;
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
	/** ── v0.6.0 WebSocket / MCP / 记忆 ────────────────────────── */
	ws_enabled?: boolean;
	ws_path?: string;
}
