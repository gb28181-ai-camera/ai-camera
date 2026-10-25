/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type RiskSummaryResp = Definition & {
	accurate_no?: number;
	accurate_yes?: number;
	handled?: number;
	high?: number;
	pending?: number;
	today?: number;
	total?: number;
	unconfirmed?: number;
	unread?: number;
}
