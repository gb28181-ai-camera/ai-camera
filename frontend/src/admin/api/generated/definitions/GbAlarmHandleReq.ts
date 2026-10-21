/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbAlarmHandleReq = Definition & {
	/** Handled 为空时按 true 处理（处理接口的默认语义就是「已处理」）。 */
	handled?: boolean;
	remark?: string;
}
