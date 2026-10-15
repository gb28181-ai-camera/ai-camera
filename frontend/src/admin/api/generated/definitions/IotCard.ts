/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { Device } from './Device';
import type { Region } from './Region';

export type IotCard = Definition & {
	active?: boolean;
	card_no?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	/** 反向关联：ac_iot_card.device_id 与 ac_device.iot_card_id 互为引用， 若两侧都交给 GORM 建外键会形成循环依赖导致 AutoMigrate 失败。 这里用 -:migration 保留 Preload 能力但不参与建表；外键约束由 docs/开发文档/数据库设计/sql/ddl 下的 DDL 脚本用 ALTER TABLE 补齐。 */
	device?: Device;
	device_id?: number;
	expire_dt?: string;
	id?: number;
	name?: string;
	region?: Region;
	region_id?: number;
	remark?: string;
	stat_dt?: string;
	status?: string;
	traffic_left?: number;
	traffic_total?: number;
	traffic_used?: number;
	updated_by?: string;
	updated_dt?: string;
}
