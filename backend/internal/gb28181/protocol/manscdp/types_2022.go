package manscdp

import (
	"encoding/xml"

	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/version"
)

// ════════════════════════════════════════════════════════════════
// GB/T 28181-2022 相对 2016 的 MANSCDP 扩展类型
//
// 依据：GB/T 28181-2022 附录 A（命令集）。命令名与字段名已在标准原文/公开
// 解读中核对；**仍未确认的标签名**在字段注释里显式标注「待确认」，不臆造。
//
// 编码/解码统一走 codec.go 的 Marshal / DecodeInto，因此自动继承：
//   - XML 声明与 GB2312/UTF-8 字节必须一致（设计 §2.6）；
//   - 无声明且非法 UTF-8 一律报错，不静默乱码。
//
// 未在结构体中声明的元素（标准后续版本或厂商私有扩展）由 encoding/xml 直接
// 忽略，不影响已知字段解析；需要保留原始 XML 的调用方可自行用 Envelope +
// innerxml 再解一次。**不臆造未核实的标签名**，存疑字段均在注释标 ★。
// ════════════════════════════════════════════════════════════════

// 2022 新增 CmdType。取值来自 version 包，避免两处字符串漂移。
const (
	CmdPTZPosition       = version.CommandPTZPosition
	CmdCruiseTrackList   = version.CommandCruiseTrackList
	CmdCruiseTrack       = version.CommandCruiseTrack
	CmdHomePosition      = version.CommandHomePosition
	CmdSDCardStatus      = version.CommandSDCardStatus
	CmdStorageCardStatus = version.CommandStorageCardStatus // 非标准别名，见 version 包注释
)

// 按 CmdType 原文命名的别名。
//
// 标准文档里的取值是 CruiseTrackListQuery / CruiseTrackQuery / HomePositionQuery，
// 而上面的短名沿用了本包 CmdCatalog / CmdDeviceInfo 的风格。两套名字指向同一
// 字符串，调用方按文档检索代码时不会找不到。
const (
	CmdPTZPositionQuery       = CmdPTZPosition
	CmdCruiseTrackListQuery   = CmdCruiseTrackList
	CmdCruiseTrackQuery       = CmdCruiseTrack
	CmdHomePositionQuery      = CmdHomePosition
	CmdSDCardStatusQuery      = CmdSDCardStatus
	CmdStorageCardStatusQuery = CmdStorageCardStatus
)

// ElementPTZPreciseCtrl 是 Control 报文里的 2022 子元素名。
// 注意它不是 CmdType：Control 的 CmdType 仍是 DeviceControl。
const ElementPTZPreciseCtrl = version.CommandPTZPreciseCtrl

// ────────────────────────────────────────────────────────────────
// 平台 → 设备：2022 查询
// ────────────────────────────────────────────────────────────────

// PTZPositionQuery PTZ 精准状态查询（A.2.4.13）。
// 报文形态与 2016 的通用 Query 相同，仅 CmdType 不同。
type PTZPositionQuery struct {
	XMLName  xml.Name `xml:"Query"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
}

// CruiseTrackListQuery 巡航轨迹列表查询（A.2.4.11 / 应答 A.2.6.13）。
type CruiseTrackListQuery struct {
	XMLName  xml.Name `xml:"Query"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
}

// CruiseTrackQuery 巡航轨迹查询（A.2.4.12 / 应答 A.2.6.14）。
//
// Number 指定轨迹编号，**从 0 开始**（0 = 第一条轨迹）。请求中的 Number
// 在公开摘录里未见完整 XSD，但查询单条轨迹必须携带编号，故按必选元素处理；
// 若真机报文不带 Number，解码后为 0，等价于查询第一条轨迹。
type CruiseTrackQuery struct {
	XMLName  xml.Name `xml:"Query"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	Number   int      `xml:"Number"`
}

// HomePositionQuery 看守位查询。
//
// 2022 把 2016 里的 HomePosition(string) 细化为结构体（见 HomePositionResponse）。
type HomePositionQuery struct {
	XMLName  xml.Name `xml:"Query"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
}

// SDCardStatusQuery 存储卡状态查询（A.2.4.14）。
//
// 标准 CmdType 为 SDCardStatus。任务书写作 StorageCardStatus，这里同时提供
// 常量别名（CmdStorageCardStatus），但下发时**应使用 CmdSDCardStatus**。
type SDCardStatusQuery struct {
	XMLName  xml.Name `xml:"Query"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
}

// ────────────────────────────────────────────────────────────────
// 设备 → 平台：2022 查询应答
// ────────────────────────────────────────────────────────────────

// PTZPosition 是 A.2.6.15 定义的位置字段集合，应答与通知复用。
//
// 全部字段用 *float64：0 是合法角度/倍率（例如云台回正 Pan=0），必须能区分
// 「设备没上报」与「上报了 0」。omitempty 让未上报的字段不出现在 XML 里，
// 符合标准中 minOccurs="0" 的定义。
type PTZPosition struct {
	Pan                  *float64 `xml:"Pan,omitempty"`                  // 水平角度（度）
	Tilt                 *float64 `xml:"Tilt,omitempty"`                 // 垂直角度（度）
	Zoom                 *float64 `xml:"Zoom,omitempty"`                 // 变焦倍数
	HorizontalFieldAngle *float64 `xml:"HorizontalFieldAngle,omitempty"` // 水平视场角（度）★ 待真机确认
	VerticalFieldAngle   *float64 `xml:"VerticalFieldAngle,omitempty"`   // 垂直视场角（度）★ 待真机确认
	MaxViewDistance      *float64 `xml:"MaxViewDistance,omitempty"`      // 可视距离（米）★ 待真机确认
}

// PTZPositionResponse PTZ 精准状态查询应答（A.2.6.15）。
//
// 标准里 Pan/Tilt/Zoom 直接挂在 Response 下，不是嵌套在 <PTZPosition> 子元素里。
type PTZPositionResponse struct {
	XMLName  xml.Name `xml:"Response"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	PTZPosition
}

// PTZPositionNotify PTZ 精准位置变化通知（订阅后由设备主动上报）。
//
// 订阅类型取 `ptz_position`（见 model.GbSubscribePTZPosition）。通知报文
// CmdType 与查询同名，根元素为 Notify。★ 2022 原文对订阅通知的根元素与
// CmdType 未在公开摘录中完整给出，这里按「与 PTZPosition 应答同名字段」
// 实现；若真机使用不同 CmdType，仅需调整本类型与解码入口。
type PTZPositionNotify struct {
	XMLName  xml.Name `xml:"Notify"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	PTZPosition
}

// CruiseTrackListResponse 巡航轨迹列表应答（A.2.6.13）。
type CruiseTrackListResponse struct {
	XMLName  xml.Name `xml:"Response"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	SumNum   int      `xml:"SumNum"` // 轨迹总数
	// CruiseTrackList 在标准中 minOccurs="0"（设备无巡航能力时可整块省略）。
	CruiseTrackList *CruiseTrackList `xml:"CruiseTrackList,omitempty"`
}

// CruiseTrackList 轨迹集合。Num 是**属性**，未配置轨迹时为 0。
type CruiseTrackList struct {
	Num    int           `xml:"Num,attr"`
	Tracks []CruiseTrack `xml:"CruiseTrack"`
}

// CruiseTrack 一条巡航轨迹的摘要（列表查询只返回编号与名称）。
type CruiseTrack struct {
	Number int    `xml:"Number"`         // 轨迹编号，从 0 开始
	Name   string `xml:"Name,omitempty"` // 名称，最长 32 字节
}

// CruiseTrackResponse 巡航轨迹查询应答（A.2.6.14）。
type CruiseTrackResponse struct {
	XMLName  xml.Name `xml:"Response"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	Number   int      `xml:"Number"`
	Name     string   `xml:"Name,omitempty"`
	// CruisePointList 标准中 minOccurs="0"。
	CruisePointList *CruisePointList `xml:"CruisePointList,omitempty"`
}

// CruisePointList 轨迹点集合。Num 是属性，未配置时为 0。
type CruisePointList struct {
	Num    int           `xml:"Num,attr"`
	Points []CruisePoint `xml:"CruisePoint"`
}

// CruisePoint 一个巡航点（预置位 + 停留时间 + 云台速度）。
type CruisePoint struct {
	PresetIndex int `xml:"PresetIndex"` // 预置位编号
	StayTime    int `xml:"StayTime"`    // 停留时间（秒）
	Speed       int `xml:"Speed"`       // 云台速度 1–15
}

// HomePositionResponse 看守位查询应答。
//
// 字段名已由标准 A.2.6.12 的 XSD 摘录确认：
//
//	<HomePosition><Enabled/><ResetTime/><PresetIndex/></HomePosition>
//
// Enabled: 0 关闭 / 1 开启；ResetTime 单位秒；PresetIndex 0–255。
// 设备不支持时整个 HomePosition 块省略（指针为 nil）。
type HomePositionResponse struct {
	XMLName      xml.Name      `xml:"Response"`
	CmdType      string        `xml:"CmdType"`
	SN           int           `xml:"SN"`
	DeviceID     string        `xml:"DeviceID"`
	HomePosition *HomePosition `xml:"HomePosition,omitempty"`
}

// HomePosition 看守位配置。
type HomePosition struct {
	Enabled     int `xml:"Enabled"`     // 0 关闭 / 1 开启
	ResetTime   int `xml:"ResetTime"`   // 自动归位时间间隔（秒）
	PresetIndex int `xml:"PresetIndex"` // 调用预置位编号 0–255
}

// SDCardStatusResponse 存储卡状态查询应答（A.2.6.16）。
//
// ⚠️ 本结构是**尽力实现**：标准 CmdType（SDCardStatus）与容量字段
// （Capacity/FreeSpace，单位 MB）已确认；SDCardStatusInfo 内部的分组元素名
// （SDCard 列表项、Num 属性、Status/FormatProgress 的层级）公开资料不完整，
// 标注「待确认」。解码时未知字段被忽略；Wave 2 联调应按真机抓包校正标签名。
type SDCardStatusResponse struct {
	XMLName          xml.Name          `xml:"Response"`
	CmdType          string            `xml:"CmdType"`
	SN               int               `xml:"SN"`
	DeviceID         string            `xml:"DeviceID"`
	SDCardStatusInfo *SDCardStatusInfo `xml:"SDCardStatusInfo,omitempty"`
}

// SDCardStatusInfo 存储卡状态块。
type SDCardStatusInfo struct {
	// Num 卡片数量。★ 待确认：标准摘录未给出属性名，按列表惯例用 Num。
	Num int `xml:"Num,attr"`
	// Status 整体状态。★ 待确认：取值含义（正常/异常/格式化中）未确认。
	Status int `xml:"Status,omitempty"`
	// FormatProgress 格式化进度 0–100。已由公开实现确认；负值/缺省表示未在格式化。
	FormatProgress int `xml:"FormatProgress,omitempty"`
	// Cards 具体存储卡列表。★ 待确认：列表项元素名与层级未确认。
	Cards []SDCardInfo `xml:"SDCard"`
}

// SDCardInfo 单张存储卡。
type SDCardInfo struct {
	Capacity  int64 `xml:"Capacity"`         // 总空间（MB）— 已确认
	FreeSpace int64 `xml:"FreeSpace"`        // 剩余空间（MB）— 已确认
	Status    int   `xml:"Status,omitempty"` // ★ 待确认
}

// ────────────────────────────────────────────────────────────────
// 平台 → 设备：2022 PTZ 精准控制
// ────────────────────────────────────────────────────────────────

// PTZPreciseCtrl 是 Control/PTZPreciseCtrl 子元素（A.2.3.1.11）。
//
// 与 2016 的 8 字节 PTZCmd 的区别：PTZCmd 是「方向 + 速度」的相对控制，
// PTZPreciseCtrl 是「绝对角度 + 倍率」的定位控制。全部字段可选、double。
type PTZPreciseCtrl struct {
	Pan  *float64 `xml:"Pan,omitempty"`  // 水平角度（度）
	Tilt *float64 `xml:"Tilt,omitempty"` // 垂直角度（度）
	Zoom *float64 `xml:"Zoom,omitempty"` // 变焦倍数
}

// ────────────────────────────────────────────────────────────────
// 构造器
// ────────────────────────────────────────────────────────────────

// NewPTZPositionQuery 构造 PTZ 精准状态查询。
func NewPTZPositionQuery(deviceID string, sn int) *PTZPositionQuery {
	return &PTZPositionQuery{CmdType: CmdPTZPosition, SN: sn, DeviceID: deviceID}
}

// NewCruiseTrackListQuery 构造巡航轨迹列表查询。
func NewCruiseTrackListQuery(deviceID string, sn int) *CruiseTrackListQuery {
	return &CruiseTrackListQuery{CmdType: CmdCruiseTrackList, SN: sn, DeviceID: deviceID}
}

// NewCruiseTrackQuery 构造巡航轨迹查询。number 从 0 开始。
func NewCruiseTrackQuery(deviceID string, sn, number int) *CruiseTrackQuery {
	return &CruiseTrackQuery{CmdType: CmdCruiseTrack, SN: sn, DeviceID: deviceID, Number: number}
}

// NewHomePositionQuery 构造看守位查询。
func NewHomePositionQuery(deviceID string, sn int) *HomePositionQuery {
	return &HomePositionQuery{CmdType: CmdHomePosition, SN: sn, DeviceID: deviceID}
}

// NewSDCardStatusQuery 构造存储卡状态查询。
//
// 使用标准 CmdType（SDCardStatus），而不是任务书里的 StorageCardStatus。
func NewSDCardStatusQuery(deviceID string, sn int) *SDCardStatusQuery {
	return &SDCardStatusQuery{CmdType: CmdSDCardStatus, SN: sn, DeviceID: deviceID}
}

// NewPTZPreciseControl 构造 PTZ 精准控制报文。
//
// 三个值都会下发（绝对定位通常一次给全）。若只需调整其中一个轴，可直接构造
// Control{PTZPrecise: &PTZPreciseCtrl{...}}，未设置的指针会被省略。
func NewPTZPreciseControl(deviceID string, sn int, pan, tilt, zoom float64) *Control {
	return &Control{
		CmdType:  CmdControl, // 仍是 DeviceControl，2022 只新增子元素
		SN:       sn,
		DeviceID: deviceID,
		PTZPrecise: &PTZPreciseCtrl{
			Pan:  &pan,
			Tilt: &tilt,
			Zoom: &zoom,
		},
	}
}
