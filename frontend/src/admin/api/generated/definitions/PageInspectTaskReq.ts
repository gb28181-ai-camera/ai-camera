/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
// 由 GET /web/inspects/tasks 的 query 参数生成（页面以查询对象传参）。
import type { Definition } from './Definition';

export type PageInspectTaskReq = Definition & {
	page?: number;
	page_size?: number;
	keyword?: string;
	region_id?: number;
	inspector_id?: number;
	status?: string;
	start_dt?: string;
	end_dt?: string;
}
