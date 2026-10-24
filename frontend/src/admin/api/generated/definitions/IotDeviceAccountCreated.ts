/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { IotDeviceAccount } from './IotDeviceAccount';

export type IotDeviceAccountCreated = Definition & {
	account?: IotDeviceAccount;
	password?: string;
}
