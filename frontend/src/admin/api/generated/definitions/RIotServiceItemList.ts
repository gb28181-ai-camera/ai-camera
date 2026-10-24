/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { IotServiceItemListResp } from './IotServiceItemListResp';

export type RIotServiceItemList = Definition & {
	code?: number;
	data?: IotServiceItemListResp;
	message?: string;
}
