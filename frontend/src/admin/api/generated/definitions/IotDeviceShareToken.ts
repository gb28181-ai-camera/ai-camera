/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type IotDeviceShareToken = Definition & {
	account_id?: number;
	device_id?: number;
	qr_content?: string;
	token?: string;
}
