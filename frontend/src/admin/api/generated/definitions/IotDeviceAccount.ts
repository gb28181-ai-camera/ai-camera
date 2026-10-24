/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';

export type IotDeviceAccount = Definition & {
	activated_at?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	device_id?: number;
	id?: number;
	nickname?: string;
	phone?: string;
	/** PlatformUserID 账号所绑定的平台用户（App 认领/绑定后写入）。 为什么单独建列而不是复用 username：设备账号的 username 是**给设备端服务**用的 登录名，平台用户则是 App 的 JWT 主体，两者命名空间与生命周期都不同。 分享令牌尚未被认领时该列为 NULL；App 设备列表按此列过滤， 从而保证「一个用户只能看到自己认领/绑定的设备」。 */
	platform_user_id?: number;
	role?: string;
	share_token?: string;
	status?: string;
	updated_by?: string;
	updated_dt?: string;
	username?: string;
}
