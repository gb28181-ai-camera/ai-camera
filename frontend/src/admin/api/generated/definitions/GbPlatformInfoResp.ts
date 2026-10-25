/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { GbDefaultMediaServerSummary } from './GbDefaultMediaServerSummary';
import type { GbDependencyStatus } from './GbDependencyStatus';
import type { GbSipStatus } from './GbSipStatus';

export type GbPlatformInfoResp = Definition & {
	arch?: string;
	build_time?: string;
	build_version?: string;
	database?: GbDependencyStatus;
	default_media_server?: GbDefaultMediaServerSummary;
	go_version?: string;
	hostname?: string;
	os?: string;
	redis?: GbDependencyStatus;
	sip?: GbSipStatus;
	started_dt?: string;
	uptime_seconds?: number;
	version?: string;
}
