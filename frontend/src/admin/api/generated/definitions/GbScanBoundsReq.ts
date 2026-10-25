/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbScanBoundsReq = Definition & {
	/** GroupID 扫描组号 1–255。 */
	group_id: number;
	/** SetLeft 把当前位置设为左边界。 */
	set_left?: boolean;
	/** SetRight 把当前位置设为右边界。 */
	set_right?: boolean;
	/** Speed 扫描速度 1–4095。 */
	speed?: number;
}
