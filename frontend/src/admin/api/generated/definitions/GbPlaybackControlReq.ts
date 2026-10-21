/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbPlaybackControlReq = Definition & {
	/** pause | resume | seek | speed */
	action: string;
	/** Scale 仅 Action=speed 时必填，1.0 为原速。 */
	scale?: number;
	/** SeekTime 仅 Action=seek 时必填，格式 "2006-01-02T15:04:05"。 */
	seek_time?: string;
	stream_id: string;
}
