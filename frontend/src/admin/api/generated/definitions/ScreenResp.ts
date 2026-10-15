/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { AlarmRecord } from './AlarmRecord';
import type { DeviceStat } from './DeviceStat';
import type { IotCardTrafficStat } from './IotCardTrafficStat';
import type { NameCount } from './NameCount';
import type { RiskRecord } from './RiskRecord';

export type ScreenResp = Definition & {
	/** AlarmRealtime 最新 10 条设备告警（已预载 alarm_type / device / region 名称）。 */
	alarm_realtime?: AlarmRecord[];
	/** AlarmStats 告警类型统计（近 30 天）。 */
	alarm_stats?: NameCount[];
	/** DeviceMonitor 设备监控：总数/在线/离线。 */
	device_monitor?: DeviceStat;
	/** RiskRealtime 最新 10 条风险（已预载 risk_type / device / region 名称）。 */
	risk_realtime?: RiskRecord[];
	/** RiskStats 风险类型统计（近 30 天）。 */
	risk_stats?: NameCount[];
	today_alarm_count?: number;
	today_risk_count?: number;
	/** TrafficMonitor 流量监控：剩余流量最少的 10 张物联网卡。 */
	traffic_monitor?: IotCardTrafficStat[];
}
