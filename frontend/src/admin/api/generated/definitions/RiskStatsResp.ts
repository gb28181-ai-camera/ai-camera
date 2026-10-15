/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { NameCount } from './NameCount';
import type { TrendPoint } from './TrendPoint';

export type RiskStatsResp = Definition & {
	/** ByDevice 按设备分组统计（Top 10）。 */
	by_device?: NameCount[];
	/** ByLevel 按风险等级分组统计，name 取 high/medium/low。 */
	by_level?: NameCount[];
	/** ByRegion 按所属区域分组统计。 */
	by_region?: NameCount[];
	/** ByType 按风险类型分组统计。 */
	by_type?: NameCount[];
	handled?: number;
	pending?: number;
	total?: number;
	/** Trend 区间内逐日风险趋势。 */
	trend?: TrendPoint[];
}
