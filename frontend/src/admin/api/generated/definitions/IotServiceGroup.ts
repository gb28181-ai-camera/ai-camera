/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type IotServiceGroup = Definition & {
	active?: boolean;
	code?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	description?: string;
	id?: number;
	name?: string;
	/** ReferencedByProducts 引用该服务组的产品数量（非数据库列）。 服务组一旦被产品绑定就进入**只读**状态：设备 OTA 正在使用它下面的服务， 修改服务或地址会让已联网设备拿到不一致的配置。必须先在产品上解绑。 */
	referenced_by_products?: number;
	type?: string;
	updated_by?: string;
	updated_dt?: string;
}
