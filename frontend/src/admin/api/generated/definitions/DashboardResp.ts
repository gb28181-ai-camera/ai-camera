/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeviceStat } from './DeviceStat';
import type { HandleStat } from './HandleStat';
import type { NameCount } from './NameCount';
import type { TrendPoint } from './TrendPoint';

export type DashboardResp = Definition & {
	/** AlarmTrend 告警趋势（近 7 天）。 */
	alarm_trend?: TrendPoint[];
	/** DeviceStat 设备统计：总数/在线/离线。 */
	device_stat?: DeviceStat;
	month_alarm_count?: number;
	month_alarm_handled_count?: number;
	month_handled_count?: number;
	month_risk_count?: number;
	month_risk_handled_count?: number;
	/** RegionRiskStats 区域风险统计（近 30 天）。 */
	region_risk_stats?: NameCount[];
	/** RiskHandleStats 风险处理统计（近 30 天）。 */
	risk_handle_stats?: HandleStat;
	/** RiskTrend 风险趋势（近 7 天）。 */
	risk_trend?: TrendPoint[];
	/** RiskTypeStats 风险类型分析（近 30 天）。 */
	risk_type_stats?: NameCount[];
	today_alarm_count?: number;
	today_alarm_handled_count?: number;
	today_handled_count?: number;
	today_risk_count?: number;
	today_risk_handled_count?: number;
	week_alarm_count?: number;
	week_alarm_handled_count?: number;
	week_handled_count?: number;
	week_risk_count?: number;
	week_risk_handled_count?: number;
}
