/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { AIModel } from './AIModel';
import type { DeletedAt } from './DeletedAt';
import type { Firmware } from './Firmware';
import type { IotCard } from './IotCard';
import type { IotProduct } from './IotProduct';
import type { IotServiceGroupBrief } from './IotServiceGroupBrief';
import type { Region } from './Region';

export type Device = Definition & {
	activated_at?: string;
	active?: boolean;
	ai_model?: AIModel;
	battery?: number;
	channel_count?: number;
	charset?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	device_code?: string;
	device_mode?: string;
	device_secret?: string;
	firmware?: Firmware;
	/** 版本与模式 */
	firmware_id?: number;
	firmware_ver?: string;
	/** ── 国标接入（GB28181，v0.2.0 新增 13 列 + 2022 扩展 gb_version）────── ⚠️ gb_device_id 与 device_code 是**两个不同的 ID 命名空间**： device_code 为业务编号（如 CAM_001），供 /web/devices/:id/heartbeat 使用； gb_device_id 为 20 位国标编码，是 SIP From 头携带的值。 二者不得互相复用。gb_device_id 为空的设备不参与 SIP（信令层回 403）。 唯一约束为**部分唯一索引**（WHERE gb_device_id IS NOT NULL AND deleted_at IS NULL）， 无法用 GORM tag 表达，由 DDL 脚本创建；AutoMigrate 只负责列本身。 依据：docs/开发文档/数据库设计/sql/ddl/09-国标视频.sql 第 0 节。 */
	gb_device_id?: string;
	gb_version?: string;
	id?: number;
	info_interval?: number;
	iot_card?: IotCard;
	/** 物联网卡 */
	iot_card_id?: number;
	last_beat_dt?: string;
	latitude?: string;
	local_ip?: string;
	location?: string;
	longitude?: string;
	media_server_id?: string;
	model_id?: number;
	model_ver?: string;
	/** 实时监控 */
	monitor_enabled?: boolean;
	name?: string;
	offline_dt?: string;
	online_dt?: string;
	product?: IotProduct;
	/** ── IOT 管理接入（v0.3.0 新增 3 列）────────────────────────── 为什么复用本表：设备已被风险/告警/巡检/升级/国标等模块引用， 新建 IOT 设备表会导致同一台物理设备出现两套主键与两套生命周期。 IOT 只是在设备上追加「属于哪个产品、接入密钥、何时激活」。 device_secret 是设备端 OTA/激活的唯一凭据（无 JWT），属敏感字段： Web 读接口必须以 **** 返回，写接口收到空值/**** 时保持原值。 */
	product_id?: number;
	/** 关联对象（列表展示名称用） */
	region?: Region;
	region_id?: number;
	register_dt?: string;
	register_status?: string;
	remark?: string;
	sdp_ip?: string;
	/** ServiceGroups 非持久化：设备所属产品绑定的服务组摘要，列表页批量填充。 */
	service_groups?: IotServiceGroupBrief[];
	sip_expires?: string;
	sip_password?: string;
	sip_realm?: string;
	sip_transport?: string;
	/** 运行时状态（设备心跳/平台探测更新） */
	status?: string;
	storage_free?: number;
	storage_total?: number;
	stream_mode?: string;
	stream_url?: string;
	system_ver?: string;
	updated_by?: string;
	updated_dt?: string;
	voltage?: string;
	work_mode?: string;
}
