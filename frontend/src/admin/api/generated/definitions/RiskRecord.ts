/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { Device } from './Device';
import type { Region } from './Region';
import type { RiskType } from './RiskType';

export type RiskRecord = Definition & {
	accurate_by?: string;
	accurate_dt?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept_id?: number;
	/** 检测准确（人工确认） */
	detect_accurate?: string;
	detect_score?: number;
	device?: Device;
	device_code?: string;
	device_id?: number;
	handle_image?: string;
	handle_result?: string;
	handled_dt?: string;
	handler?: string;
	id?: number;
	image_url?: string;
	location?: string;
	occurred_dt?: string;
	/** 消息通知 */
	read?: boolean;
	read_dt?: string;
	region?: Region;
	region_id?: number;
	risk_level?: string;
	risk_type?: RiskType;
	risk_type_id?: number;
	/** 处理 */
	status?: string;
	updated_by?: string;
	updated_dt?: string;
	video_url?: string;
	/** 语音播报（若该风险类型配置了语音） */
	voice_played?: boolean;
}
