/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type WebhookConfig = Definition & {
	active?: boolean;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	/** EventTypes 订阅的事件类型，逗号分隔，如 "risk,alarm"。为空表示订阅全部事件。 */
	event_types?: string;
	id?: number;
	last_error?: string;
	/** 发送结果快照，便于在列表页直接观察通道健康度 */
	last_send_dt?: string;
	last_status?: string;
	msg_type?: string;
	name?: string;
	platform?: string;
	remark?: string;
	/** Secret 用于钉钉加签（HMAC-SHA256）或自定义平台的签名校验；飞书/企业微信可留空。 */
	secret?: string;
	sequence?: number;
	updated_by?: string;
	updated_dt?: string;
	url?: string;
}
