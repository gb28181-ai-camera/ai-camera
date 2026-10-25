/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type IotProductServiceCredentials = Definition & {
	password?: string;
	product_id?: number;
	service_id?: number;
	username?: string;
}
