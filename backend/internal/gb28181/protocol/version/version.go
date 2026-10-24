// Package version 维护 GB/T 28181 的协议版本模型与能力表。
//
// 为什么单独成包：GB/T 28181-2022 在 2016 版基础上新增了一批 MANSCDP 命令
// （PTZ 精准位置、巡航轨迹、看守位、存储卡状态等）。平台下发这些命令前必须先
// 判断设备是否支持，否则 2016 设备会因「未知 CmdType」直接丢弃报文，现场表现为
// 「点了没反应」且没有任何错误可查。把版本判定收敛到一个纯函数包，既避免
// sipserver / service / API 各写一套判断，也方便后续按能力做灰度。
//
// 本包**不引用** manscdp：命令名以字符串常量在这里定义，manscdp 反过来引用本包。
// 这样版本判定可独立测试，也不会与报文结构形成循环依赖。
package version

import "strings"

// Version 是 GB/T 28181 的版本号。
//
// 用两位年份而不是 "2.0"：标准自身以年份区分版本（GB/T 28181-2016 / -2022），
// 现场设备资料、抓包备注也一律写年份，避免与「协议第 2 版」混淆。
type Version string

const (
	V2016 Version = "2016"
	V2022 Version = "2022"
)

// Feature 是平台关心的能力项。
//
// 只收录「需要做版本判定」的能力。Catalog（目录查询）这类 2016/2022 都有的基础
// 能力不需要进表；本表只回答「这个能力能否对这台设备用」。
type Feature string

const (
	// FeatureCatalogEvent 目录变更事件（Item/Event：ADD/UPDATE/DEL/ON/OFF/VLOST）。
	// 2016 版就已定义，本项目此前未解析；版本层面两个版本都支持。
	FeatureCatalogEvent Feature = "catalog_event"

	// ── 以下为 2022 新增（附录 A.2.4 / A.2.6 / A.2.3）──

	// FeaturePTZPosition PTZ 精准状态查询（A.2.4.13 / A.2.6.15）。
	FeaturePTZPosition Feature = "ptz_position"
	// FeaturePTZPositionNotify PTZ 精准位置变化订阅通知（Notify/PTZPosition）。
	FeaturePTZPositionNotify Feature = "ptz_position_notify"
	// FeatureCruiseTrackList 巡航轨迹列表查询（A.2.6.13）。
	FeatureCruiseTrackList Feature = "cruise_track_list"
	// FeatureCruiseTrack 巡航轨迹查询（A.2.6.14）。
	FeatureCruiseTrack Feature = "cruise_track"
	// FeatureHomePosition 看守位查询（HomePositionQuery）。
	FeatureHomePosition Feature = "home_position"
	// FeaturePTZPreciseCtrl PTZ 精准控制（Control/PTZPreciseCtrl）。
	FeaturePTZPreciseCtrl Feature = "ptz_precise_ctrl"
	// FeatureSDCardStatus 存储卡状态查询（A.2.4.14 / A.2.6.16）。
	FeatureSDCardStatus Feature = "sdcard_status"

	// ── 2022 新增媒体编码能力 ──
	//
	// 这里只做「版本能力判定」；具体 SDP 协商（H.265 / G.722.1 / AAC 的
	// rtpmap / fmtp）由 sdp 包与传输侧落地，不在本包职责内。
	FeatureVideoH265  Feature = "video_h265"
	FeatureAudioG7221 Feature = "audio_g722_1"
	FeatureAudioAAC   Feature = "audio_aac"
)

// capabilities 是唯一的能力事实来源。
//
// 刻意用显式表而不是「2022 ⊇ 2016」的推导：标准里个别能力在 2022 被细分
// （如 HomePosition 从 string 变为结构体），显式表能表达「同名但语义变化」，
// 也避免以后把「2022 新增」误当成「2016 也有」。
var capabilities = map[Version]map[Feature]bool{
	V2016: {
		FeatureCatalogEvent: true,
	},
	V2022: {
		FeatureCatalogEvent:      true,
		FeaturePTZPosition:       true,
		FeaturePTZPositionNotify: true,
		FeatureCruiseTrackList:   true,
		FeatureCruiseTrack:       true,
		FeatureHomePosition:      true,
		FeaturePTZPreciseCtrl:    true,
		FeatureSDCardStatus:      true,
		FeatureVideoH265:         true,
		FeatureAudioG7221:        true,
		FeatureAudioAAC:          true,
	},
}

// Normalize 把数据库 / 配置 / 设备上报里的任意版本写法归一为 V2016 或 V2022。
//
// 支持 "2016"、"2022"、"GB/T 28181-2022"、"GB28181—2022"、"v2022" 等写法。
// 空值或无法识别时一律回落到 V2016：
//
//	宁可少发 2022 命令，也不要对老设备发它不认识的 CmdType。
//
// 这是一条**安全默认值**。如果现场确认设备是 2022 但资料写错，可在收到
// 2022 独有命令时用 UpgradeOnCommand 自动升级（见下）。
func Normalize(s string) Version {
	t := strings.ToUpper(strings.TrimSpace(s))
	switch {
	case t == "":
		return V2016
	case strings.Contains(t, "2022"):
		return V2022
	case strings.Contains(t, "2016"):
		return V2016
	default:
		return V2016
	}
}

// Supports 判断版本 v 是否支持能力 f。
//
// 未知版本按 Normalize 的规则回落；未知能力返回 false（能力表是白名单）。
func Supports(v Version, f Feature) bool {
	return capabilities[Normalize(string(v))][f]
}

// Is2022Only 判断某能力是否只在 2022 版可用。
func Is2022Only(f Feature) bool {
	return !Supports(V2016, f) && Supports(V2022, f)
}

// ────────────────────────────────────────────────────────────────
// 2022 独有命令 / 元素名
// ────────────────────────────────────────────────────────────────

// 2022 新增的 CmdType 取值（与 manscdp 的常量保持一致）。
const (
	CommandPTZPosition       = "PTZPosition"
	CommandCruiseTrackList   = "CruiseTrackListQuery"
	CommandCruiseTrack       = "CruiseTrackQuery"
	CommandHomePosition      = "HomePositionQuery"
	CommandSDCardStatus      = "SDCardStatus"
	CommandStorageCardStatus = "StorageCardStatus"
	CommandPTZPreciseCtrl    = "PTZPreciseCtrl"
)

// commandFeatures 把报文里的名字映射到能力项。
//
// 说明：
//   - PTZPosition 既可能是查询/应答的 CmdType，也可能是 Notify 的 CmdType；
//     调用方用 Is2022Only(FeaturePTZPositionNotify) 区分通知场景。
//   - PTZPreciseCtrl 是 Control 报文的**子元素**，不是 CmdType；sipserver 解析
//     Control 时应把子元素名传进来（CmdType 仍是 DeviceControl，2016 也有）。
//   - StorageCardStatus 是任务书中出现的写法；标准原文的 CmdType 为
//     SDCardStatus（A.2.4.14）。两者都识别，避免厂商实现用别名时漏判。
var commandFeatures = map[string]Feature{
	CommandPTZPosition:       FeaturePTZPosition,
	CommandCruiseTrackList:   FeatureCruiseTrackList,
	CommandCruiseTrack:       FeatureCruiseTrack,
	CommandHomePosition:      FeatureHomePosition,
	CommandSDCardStatus:      FeatureSDCardStatus,
	CommandStorageCardStatus: FeatureSDCardStatus,
	CommandPTZPreciseCtrl:    FeaturePTZPreciseCtrl,
}

// FeatureOfCommand 返回命令/元素名对应的能力项。
func FeatureOfCommand(name string) (Feature, bool) {
	f, ok := commandFeatures[strings.TrimSpace(name)]
	return f, ok
}

// Is2022Command 判断 name 是否为 2022 独有命令 / 元素。
func Is2022Command(name string) bool {
	f, ok := FeatureOfCommand(name)
	return ok && Is2022Only(f)
}

// UpgradeOnCommand 在「收到 2022 独有命令」时把设备版本升级到 V2022。
//
// 用途（Wave 2 接线）：sipserver 解析到 PTZPosition / CruiseTrack* 等应答，
// 或解析 Control 时发现 PTZPreciseCtrl 子元素，可调用本函数更新
// ac_device.gb_version。本任务只提供判定函数，不改 sipserver。
//
// 已经是 V2022 时原样返回；未知命令保持原值。
func UpgradeOnCommand(current Version, name string) Version {
	if Normalize(string(current)) == V2022 {
		return V2022
	}
	if Is2022Command(name) {
		return V2022
	}
	return V2016
}
