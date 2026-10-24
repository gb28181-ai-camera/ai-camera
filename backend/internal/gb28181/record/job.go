// Package record 实现国标录像计划的定时调度。
//
// 设计边界：
//   - 调度器只做「到点启动 / 到点停止」这一件事，不负责计划 CRUD；
//   - 云端录像走 ZLM startRecord（需要先有一条在线流，因此复用实时点播编排）；
//   - 设备录像走 MANSCDP DeviceControl/RecordCmd（设备本地录像，best-effort）；
//   - 所有状态都在进程内，重启后从「当前时刻」重新判断，不补录已经过去的时段。
//
// 为什么不在服务层直接起 goroutine：GB28181 的业务 Service 会在每个 HTTP
// handler 里被构造，把调度逻辑塞进去既难单测也容易起多份。这里用独立的
// Scheduler + 纯函数窗口匹配，服务层只负责触发一次 EnsureStarted。
package record

import (
	"fmt"
	"strings"
	"time"
)

// Window 是一条录像计划时间段（对应 ac_gb_record_plan_item）。
//
// WeekDay 采用数据库约定：0=周日 … 6=周六（与 Go 的 time.Weekday 数值一致）。
// StartTime/EndTime 为 HH:mm:ss；StartTime > EndTime 表示**跨天时段**。
type Window struct {
	PlanID int64
	ItemID int64
	// WeekDay 是时段**开始那天**的星期。跨天时段的后半段落在次日，
	// 匹配时要用「前一天」去对齐，而不是次日自身的 WeekDay。
	WeekDay   int64
	StartTime string
	EndTime   string
	// Snap 计划期间是否抓图。当前调度只记录该配置，不驱动抓图，
	// 因为抓图依赖「已有在线流」，与录像共用同一路流，后续可在这里扩展。
	Snap bool
}

// Key 是窗口的唯一标识。
//
// 用 plan+item 而不是通道：同一通道换了计划或编辑了时间段后 item 会重建，
// Key 随之变化，调度器据此判定「旧任务需要停止、新任务需要启动」。
func (w Window) Key() string {
	return fmt.Sprintf("%d:%d", w.PlanID, w.ItemID)
}

// clock 把 HH:mm:ss / HH:mm 解析成「当天第几秒」。
func clock(v string) (int64, error) {
	v = strings.TrimSpace(v)
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, err := time.Parse(layout, v); err == nil {
			return int64(t.Hour()*3600 + t.Minute()*60 + t.Second()), nil
		}
	}
	return 0, fmt.Errorf("时间 %q 格式应为 HH:mm:ss", v)
}

// secondsOfDay 返回 now 在当天的秒数。
func secondsOfDay(now time.Time) int64 {
	return int64(now.Hour()*3600 + now.Minute()*60 + now.Second())
}

// ActiveAt 判断 now 是否落在窗口内。
//
// 跨天规则（例如 22:00–06:00，WeekDay=周一）：
//   - 周一 22:00–24:00 属于该窗口；
//   - 周二 00:00–06:00 也属于该窗口（用「前一天」对齐 WeekDay）。
//
// 结束时刻不包含在内（半开区间），与常见排班/录像计划语义一致：
// 06:00:00 整点由下一条时段（如果有）接管，避免两段重叠。
func (w Window) ActiveAt(now time.Time) bool {
	start, end, err := w.bounds()
	if err != nil || start == end {
		// 配置非法时按「不激活」处理：调度器不应该因为一条坏数据停摆。
		return false
	}
	nowSec := secondsOfDay(now)
	today := int64(now.Weekday())

	if start < end {
		return today == w.WeekDay && nowSec >= start && nowSec < end
	}

	// 跨天：前半段在 WeekDay 当天，后半段在次日。
	if today == w.WeekDay && nowSec >= start {
		return true
	}
	prevDay := (today + 6) % 7
	return prevDay == w.WeekDay && nowSec < end
}

// EndAt 返回当前窗口的**实际结束时刻**（带日期）。
//
// 调用方用它计算 maxSecond（ZLM 单段时长）与调度器的预期停止时间。
// 只有在 ActiveAt(now) 为 true 时调用才有意义。
func (w Window) EndAt(now time.Time) time.Time {
	start, end, err := w.bounds()
	if err != nil || start == end {
		return now
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if start < end {
		return day.Add(time.Duration(end) * time.Second)
	}
	// 跨天：当天凌晨属于「昨天开始」的尾段，结束在今天；否则结束在明天。
	if secondsOfDay(now) < end {
		return day.Add(time.Duration(end) * time.Second)
	}
	return day.AddDate(0, 0, 1).Add(time.Duration(end) * time.Second)
}

// Remaining 返回距离窗口结束还有多久；已结束返回 0。
func (w Window) Remaining(now time.Time) time.Duration {
	d := w.EndAt(now).Sub(now)
	if d < 0 {
		return 0
	}
	return d
}

func (w Window) bounds() (int64, int64, error) {
	start, err := clock(w.StartTime)
	if err != nil {
		return 0, 0, err
	}
	end, err := clock(w.EndTime)
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

// Target 是一个「通道 + 当前命中的时间段」。
type Target struct {
	ChannelID   int64
	ChannelName string
	DeviceID    int64
	DeviceName  string
	DeviceCode  string
	GbDeviceID  string
	GbChannelID string
	// DeviceStatus 来自 ac_device.status；离线设备不启动，等下一轮扫描再试。
	DeviceStatus string
	Window       Window
}

// PickActive 从同一通道的多个时间段里挑出当前生效的一条。
//
// 计划里允许存在重叠时段（例如 08–12 与 10–14）。同一通道同一时刻只能有
// 一路录制，这里按「结束得最晚」优先，保证最长覆盖；完全相同时按 ItemID
// 升序，保证结果是确定的（避免 map 遍历顺序导致每次重启选到不同条目）。
func PickActive(candidates []Target, now time.Time) (Target, bool) {
	var (
		best    Target
		bestEnd time.Time
		found   bool
	)
	for _, t := range candidates {
		if !t.Window.ActiveAt(now) {
			continue
		}
		end := t.Window.EndAt(now)
		if !found || end.After(bestEnd) || (end.Equal(bestEnd) && t.Window.ItemID < best.Window.ItemID) {
			best, bestEnd, found = t, end, true
		}
	}
	return best, found
}

// GroupByChannel 把加载结果按通道分组，供 PickActive 使用。
func GroupByChannel(targets []Target) map[int64][]Target {
	out := make(map[int64][]Target, len(targets))
	for _, t := range targets {
		out[t.ChannelID] = append(out[t.ChannelID], t)
	}
	return out
}
