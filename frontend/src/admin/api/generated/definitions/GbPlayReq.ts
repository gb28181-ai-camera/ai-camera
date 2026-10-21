/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbPlayReq = Definition & {
	/** Protocol 播放协议：webrtc | flv | hls，缺省 webrtc。 协议只影响前端选择播放器，后端始终返回三种地址；保留该字段是为了 记录本次会话的偏好，并在状态查询时回显。 */
	protocol?: string;
	/** StreamID 可选：重用已有流。非空时后端会优先按该标识查找会话。 */
	stream_id?: string;
}
