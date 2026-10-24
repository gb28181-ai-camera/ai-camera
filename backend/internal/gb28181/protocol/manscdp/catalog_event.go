package manscdp

import (
	"encoding/xml"
	"strings"
)

// ════════════════════════════════════════════════════════════════
// Catalog 目录变更事件（Item/Event）
//
// Event 在 GB/T 28181-2016 就已定义，2022 沿用；本项目此前只解析 Status，
// 导致「目录订阅推送的增删改事件」被当成普通全量目录，通道删除/视频丢失
// 无法反映到平台。这里补齐事件常量与语义映射，供 device manager / sipserver
// 在 Catalog 通知处理里使用（Wave 2 接线）。
//
// ⚠️ Event 与 Status 是**两个维度**：
//   - Event  表示「这次发生了什么变化」（ADD/DEL/UPDATE/ON/OFF/VLOST/DEFECT）；
//   - Status 表示「变化后的当前状态」（ON/OFF）。
// 二者可以同时出现，例如 Event=ADD 且 Status=ON。
// ════════════════════════════════════════════════════════════════

// 通道状态取值（Item/Status）。与 ac_gb_channel.status 的枚举一致。
const (
	StatusOn  = "ON"
	StatusOff = "OFF"
)

// CatalogNotify 是目录订阅通知（Notify/Catalog）。
//
// 为什么需要独立类型：CatalogResponse 的根元素固定为 Response，而**订阅推送**
// 的目录变更用 Notify 根元素。两者正文结构一致，但根元素不同，不能共用一个
// XMLName。Event 主要出现在 Notify 中（增量变更），全量查询 Response 通常没有。
type CatalogNotify struct {
	XMLName    xml.Name   `xml:"Notify"`
	CmdType    string     `xml:"CmdType"`
	SN         int        `xml:"SN"`
	DeviceID   string     `xml:"DeviceID"`
	SumNum     int        `xml:"SumNum"`
	DeviceList DeviceList `xml:"DeviceList"`
}

// 目录事件取值（GB/T 28181-2016/2022 附录 A.2.6.1）。
const (
	CatalogEventAdd    = "ADD"    // 增加目录项
	CatalogEventUpdate = "UPDATE" // 目录项属性更新
	CatalogEventDel    = "DEL"    // 删除目录项
	CatalogEventOn     = "ON"     // 设备/通道上线
	CatalogEventOff    = "OFF"    // 设备/通道离线
	CatalogEventVLost  = "VLOST"  // 视频丢失（通道仍在，但无视频信号）
	CatalogEventDefect = "DEFECT" // 设备/通道故障（2022 摘录中与 VLOST 并列）
)

// CatalogEventEffect 描述一个事件对本地通道记录的影响。
//
// 用指针表达「不改变」：Online == nil 表示该事件不涉及状态变更（例如未知事件），
// 调用方应保持数据库原值，而不是把 nil 当成 false 写回。
type CatalogEventEffect struct {
	// Known 事件是否被识别。false 时调用方应记日志并忽略，避免把未知语义
	// 误当成删除/离线。
	Known bool
	// Online 非 nil 时表示事件要求把通道状态改为该值。
	Online *bool
	// Delete 为 true 表示应从本地删除该目录项（软删除）。
	Delete bool
}

// ParseCatalogEvent 把 XML 里的 Event 值映射为通道状态变更语义。
//
// 映射规则（任务书钉死的基线）：
//
//	ADD / UPDATE / ON   → 在线（新增/更新/上线）
//	OFF / VLOST / DEFECT→ 离线（离线/视频丢失/故障）
//	DEL                 → 离线 + 删除
//	空 / 未知           → Known=false，不改变状态
//
// 大小写与空白都做归一：部分设备会发 "add" 或带换行的值。
func ParseCatalogEvent(event string) CatalogEventEffect {
	switch strings.ToUpper(strings.TrimSpace(event)) {
	case CatalogEventAdd, CatalogEventUpdate, CatalogEventOn:
		return CatalogEventEffect{Known: true, Online: boolPtr(true)}
	case CatalogEventOff, CatalogEventVLost, CatalogEventDefect:
		return CatalogEventEffect{Known: true, Online: boolPtr(false)}
	case CatalogEventDel:
		return CatalogEventEffect{Known: true, Online: boolPtr(false), Delete: true}
	default:
		return CatalogEventEffect{}
	}
}

// ResolveCatalogItemState 解析一条目录项应落到本地的状态。
//
// 优先使用 Event；Event 为空时退回 Status（全量目录查询没有 Event，只有
// Status，这是本项目的常规同步路径）。两者都没有时返回 Known=false。
//
// 注意：Event=UPDATE 时若 Status=OFF，本函数按任务书基线返回在线。如果现场
// 证明「UPDATE + OFF」应视为离线，调用方可以优先读 item.Status 自行覆盖 ——
// 这里把两种原始字段都保留在 CatalogItem 上，避免协议层替业务做过度推断。
func ResolveCatalogItemState(item CatalogItem) CatalogEventEffect {
	if strings.TrimSpace(item.Event) != "" {
		return ParseCatalogEvent(item.Event)
	}
	switch strings.ToUpper(strings.TrimSpace(item.Status)) {
	case StatusOn:
		return CatalogEventEffect{Known: true, Online: boolPtr(true)}
	case StatusOff:
		return CatalogEventEffect{Known: true, Online: boolPtr(false)}
	default:
		return CatalogEventEffect{}
	}
}

// boolPtr 返回 bool 的地址，便于构造「有值」的 Online 字段。
func boolPtr(v bool) *bool { return &v }
