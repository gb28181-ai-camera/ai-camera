/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbPtzPreciseReq = Definition & {
	/** Pan 水平角度（度）。取值范围由设备决定，平台不做臆测性限制。 */
	pan?: number;
	/** Tilt 垂直角度（度）。 */
	tilt?: number;
	/** Zoom 变焦倍数（1 表示 1 倍）。 */
	zoom?: number;
}
