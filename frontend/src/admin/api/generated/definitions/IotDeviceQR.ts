/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type IotDeviceQR = Definition & {
	device_code?: string;
	device_id?: number;
	product_code?: string;
	qr_content?: string;
}
