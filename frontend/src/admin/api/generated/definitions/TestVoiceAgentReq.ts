/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type TestVoiceAgentReq = Definition & {
	/** APIKey 为空时使用库中已保存的密钥；支持只测连通性而不回显密钥。 */
	api_key?: string;
	/** Category 取值 stt / llm / tts / vad；为空时按 llm 处理。 */
	category?: string;
	/** Endpoint 为空时使用库中已保存的 endpoint。 */
	endpoint?: string;
	/** Provider 为空时使用库中已保存的对应 provider。 */
	provider?: string;
}
