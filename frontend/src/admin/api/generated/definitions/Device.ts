/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { AIModel } from './AIModel';
import type { DeletedAt } from './DeletedAt';
import type { Firmware } from './Firmware';
import type { IotCard } from './IotCard';
import type { Region } from './Region';

export type Device = Definition & {
	active?: boolean;
	ai_model?: AIModel;
	battery?: number;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	device_code?: string;
	device_mode?: string;
	firmware?: Firmware;
	/** 版本与模式 */
	firmware_id?: number;
	firmware_ver?: string;
	id?: number;
	info_interval?: number;
	iot_card?: IotCard;
	/** 物联网卡 */
	iot_card_id?: number;
	last_beat_dt?: string;
	latitude?: string;
	location?: string;
	longitude?: string;
	model_id?: number;
	model_ver?: string;
	/** 实时监控 */
	monitor_enabled?: boolean;
	name?: string;
	offline_dt?: string;
	online_dt?: string;
	/** 关联对象（列表展示名称用） */
	region?: Region;
	region_id?: number;
	remark?: string;
	/** 运行时状态（设备心跳/平台探测更新） */
	status?: string;
	storage_free?: number;
	storage_total?: number;
	stream_url?: string;
	system_ver?: string;
	updated_by?: string;
	updated_dt?: string;
	voltage?: string;
	work_mode?: string;
}
