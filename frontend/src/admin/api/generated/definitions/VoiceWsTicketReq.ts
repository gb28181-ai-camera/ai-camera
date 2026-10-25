/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type VoiceWsTicketReq = Definition & {
	/** AgentCode 可选：指定 Agent 人格；为空使用默认人格。 */
	agent_code?: string;
	/** ClientID 可选：客户端自生成的连接标识。 */
	client_id?: string;
	/** DeviceCode 可选（仅 /app/voice/ws-ticket）：App 绑定的设备编号。 */
	device_code?: string;
}
