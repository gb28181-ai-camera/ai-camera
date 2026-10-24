/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';

export type GbDownloadItem = Definition & {
	app?: string;
	channel_id?: number;
	channel_name?: string;
	created_dt?: string;
	device_id?: number;
	device_name?: string;
	end_time?: string;
	file_path?: string;
	/** FileSize 是已落盘片段的大小合计（来自 on_record_mp4 的真实值）。 */
	file_size?: number;
	id?: number;
	message?: string;
	/** Progress 为 nil 表示「进度未知」：ZLM 没有暴露录制中文件的实时字节数， 当既没有已落盘片段、又查不到在线流时，返回 null 比返回 0 更诚实。 */
	progress?: number;
	ssrc?: string;
	start_time?: string;
	status?: string;
	stopped_dt?: string;
	stream?: string;
	stream_id?: string;
}
