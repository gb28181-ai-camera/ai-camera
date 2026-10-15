/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type TestWebhookReq = Definition & {
	content?: string;
	/** Title / Content 为空时使用默认的测试文案。 */
	title?: string;
}
