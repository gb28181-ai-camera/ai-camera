package record

import "math"

// ProgressInput 是下载进度的计算输入。
//
// 数据来源（为什么是「估算」而不是「真实字节进度」）：
//   - CompletedSeconds：ac_gb_cloud_record.time_len 的合计。这个值是
//     ZLM 的 on_record_mp4 回调在**文件完整落盘后**给出的，属于真实值；
//   - LiveSeconds：ZLM getMediaList 的 aliveSecond。ZLM 没有暴露「当前
//     MP4 已写入多少字节」的接口，只能把流的存活时长当作当前片段已录制
//     时长的近似值；
//   - LiveKnown：是否真的查到了在线流。查不到时绝不能把 0 当作「0%」，
//     否则前端会把「设备没推流/节点不可达」显示成「进度为 0」。
type ProgressInput struct {
	CompletedSeconds float64
	LiveSeconds      float64
	LiveKnown        bool
	TotalSeconds     float64
	// Final 表示任务已经停止/完成。完成时允许显示 100%；
	// 仍在录制时最多显示 99%，避免「100% 但文件还没写完」的误导。
	Final bool
}

// ComputeProgress 返回 (百分比, 是否可知)。
//
// 可知性判定是这里最关键的一点：
//   - 时间窗非法（<=0）→ 不可知；
//   - 没有任何已完成片段，且没有查到在线流 → 不可知（返回 false）；
//   - 只要有一个来源有数据，就给出估算值。
func ComputeProgress(in ProgressInput) (float64, bool) {
	if in.TotalSeconds <= 0 {
		return 0, false
	}
	completed := math.Max(in.CompletedSeconds, 0)
	live := math.Max(in.LiveSeconds, 0)

	if !in.LiveKnown && completed <= 0 {
		return 0, false
	}

	recorded := completed
	if in.LiveKnown {
		// 已完成片段与当前流是同一段时间的两种口径，取较大值；
		// 但要夹在总时长内，防止 aliveSecond 把进度顶过 100%。
		recorded = math.Max(completed, math.Min(live, in.TotalSeconds))
	}

	p := recorded / in.TotalSeconds * 100
	if p < 0 {
		p = 0
	}
	if p >= 100 {
		if in.Final {
			return 100, true
		}
		// 还在录制：可能最后一个片段尚未回调，最多展示 99%。
		return 99, true
	}
	return math.Round(p*10) / 10, true
}
