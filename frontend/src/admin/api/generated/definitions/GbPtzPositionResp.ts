/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbPtzPositionResp = Definition & {
	/** HorizontalFieldAngle 水平视场角（度）。★ manscdp 中标注「待真机确认」， 设备不上报时省略。 */
	horizontal_field_angle?: number;
	/** MaxViewDistance 可视距离（米）。★ 待真机确认。 */
	max_view_distance?: number;
	/** Pan 水平角度（度）。设备未上报时省略（与「上报了 0」区分）。 */
	pan?: number;
	/** Tilt 垂直角度（度）。设备未上报时省略。 */
	tilt?: number;
	/** VerticalFieldAngle 垂直视场角（度）。★ 待真机确认。 */
	vertical_field_angle?: number;
	/** Zoom 变焦倍数。设备未上报时省略。 */
	zoom?: number;
}
