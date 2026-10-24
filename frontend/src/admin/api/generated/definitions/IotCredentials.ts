/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { IotCredentialsGroup } from './IotCredentialsGroup';
import type { IotCredentialsProduct } from './IotCredentialsProduct';
import type { IotMqttInfo } from './IotMqttInfo';
import type { IotTurnInfo } from './IotTurnInfo';

export type IotCredentials = Definition & {
	groups?: IotCredentialsGroup[];
	mqtt?: IotMqttInfo;
	product?: IotCredentialsProduct;
	turn?: IotTurnInfo;
}
