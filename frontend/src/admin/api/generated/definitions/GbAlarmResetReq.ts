/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbAlarmResetReq = Definition & {
	/** ChannelID 可选：传通道主键时 DeviceID 用通道国标编码，否则用设备国标编码。 */
	channel_id?: number;
}
