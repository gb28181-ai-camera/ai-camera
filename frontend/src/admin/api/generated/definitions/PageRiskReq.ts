/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
// 由 GET /web/risks 的 query 参数生成（页面以查询对象传参）。
import type { Definition } from './Definition';

export type PageRiskReq = Definition & {
	page?: number;
	page_size?: number;
	keyword?: string;
	risk_type_id?: number;
	region_id?: number;
	device_id?: number;
	device_code?: string;
	status?: string;
	detect_accurate?: string;
	read?: boolean;
	start_dt?: string;
	end_dt?: string;
}
