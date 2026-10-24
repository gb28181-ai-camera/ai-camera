/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { Device } from './Device';
import type { IotDeviceAccount } from './IotDeviceAccount';
import type { IotServiceGroupDetail } from './IotServiceGroupDetail';

export type IotDeviceDetail = Definition & {
	accounts?: IotDeviceAccount[];
	device?: Device;
	/** QRContent 设备二维码内容，前端可直接渲染，无需再调 /qr。 */
	qr_content?: string;
	service_groups?: IotServiceGroupDetail[];
}
