/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type IotDeviceActivated = Definition & {
	activated_at?: string;
	device_code?: string;
	password?: string;
	username?: string;
}
