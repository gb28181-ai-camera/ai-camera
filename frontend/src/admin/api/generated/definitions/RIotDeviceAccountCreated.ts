/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { IotDeviceAccountCreated } from './IotDeviceAccountCreated';

export type RIotDeviceAccountCreated = Definition & {
	code?: number;
	data?: IotDeviceAccountCreated;
	message?: string;
}
