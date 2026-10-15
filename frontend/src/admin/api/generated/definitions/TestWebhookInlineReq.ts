/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type TestWebhookInlineReq = Definition & {
	active?: boolean;
	content?: string;
	event_types?: string[];
	/** 枚举值: text / markdown */
	msg_type?: 'text' | 'markdown' | (string & {});
	name: string;
	/** 枚举值: feishu / dingtalk / wecom / custom */
	platform: 'feishu' | 'dingtalk' | 'wecom' | 'custom' | (string & {});
	remark?: string;
	secret?: string;
	sequence?: number;
	title?: string;
	url: string;
}
