/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbDeviceGuardReq = Definition & {
	/** Cmd 仅支持 SetGuard（布防）/ ResetGuard（撤防）。 */
	cmd: string;
}
