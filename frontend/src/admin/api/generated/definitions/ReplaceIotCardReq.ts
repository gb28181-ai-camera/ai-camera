/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type ReplaceIotCardReq = Definition & {
	device_id: number;
	new_iot_card_id: number;
	old_iot_card_id?: number;
	remark?: string;
}
