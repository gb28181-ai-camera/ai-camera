/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbPtzReq = Definition & {
	/** Action 变焦/光圈/聚焦：zoom-in|zoom-out|zoom-stop|iris-in|iris-out|iris-stop| focus-near|focus-far|focus-stop。 */
	action?: string;
	/** Direction 八方向 + 停止：up|down|left|right|up-left|up-right|down-left|down-right|stop。 */
	direction?: string;
	/** Speed 速度量纲 1–8（1 最慢、8 最快），缺省 5。 注意：不是 0–255，后端负责映射到 PTZCmd 的速度字节。 */
	speed?: number;
}
