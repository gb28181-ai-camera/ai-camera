/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { Item } from './Item';

export type Report = Definition & {
	checked_at?: string;
	device_id?: number;
	gb_device_id?: string;
	items?: Item[];
	summary?: string;
}
