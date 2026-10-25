// Package manscdp 实现 GB/T 28181 的 MANSCDP XML 编解码。
//
// MANSCDP（Monitoring And Surveillance Command Description Protocol）是 GB28181
// 信令的正文格式，按根元素分三类：
//
//	<Query>    平台 → 设备：目录、设备信息、设备状态、录像查询……
//	<Response> 设备 → 平台：对上述各查询的应答
//	<Notify>   设备 → 平台：心跳、报警、移动位置，平台回 200 OK 即可，不需正文
//
// ⚠️ 中文（设备名、通道名）以 **GB2312** 字节进入 XML body（§2.6 结论六）。
// 本包的两条解码路径必须区分清楚：
//
//	声明了 encoding="GB2312"  → 由 Decoder.CharsetReader 转码；**不设置它会直接报错**，
//	                             报错文本是 `xml: encoding "GB2312" declared but
//	                             Decoder.CharsetReader is nil`，而不是产生乱码。
//	无声明 / 声明 UTF-8          → 静默按 UTF-8 解析，中文会变乱码并落库污染数据。
//
// 因此 Decode 在无声明时会按调用方给的 fallback charset 预转码，两条路径都不放过。
package manscdp

import "encoding/xml"

// CmdType 取值（GB/T 28181-2016 附录 A）。
const (
	CmdCatalog      = "Catalog"
	CmdDeviceInfo   = "DeviceInfo"
	CmdDeviceStatus = "DeviceStatus"
	CmdKeepalive    = "Keepalive"
	CmdAlarm        = "Alarm"

	// GB/T 28181-2022 新增（附录 A.2.4 / A.2.6）。
	// 取值必须与平台侧 internal/gb28181/protocol/version 完全一致，
	// 否则平台会按「无等待方」丢弃应答，表现为查询必超时。
	CmdPTZPosition          = "PTZPosition"
	CmdCruiseTrackListQuery = "CruiseTrackListQuery"
	CmdCruiseTrackQuery     = "CruiseTrackQuery"
	CmdHomePositionQuery    = "HomePositionQuery"
	CmdSDCardStatus         = "SDCardStatus"
)

// Envelope 只承载根元素与三个公共字段，用于「先看是什么报文，再决定解成哪个结构」。
type Envelope struct {
	XMLName  xml.Name
	CmdType  string `xml:"CmdType"`
	SN       int    `xml:"SN"`
	DeviceID string `xml:"DeviceID"`
}

// Keepalive 是设备心跳（Notify）。
type Keepalive struct {
	XMLName  xml.Name `xml:"Notify"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	Status   string   `xml:"Status,omitempty"` // OK / ERROR
}

// Query 是平台下发的查询报文。目录/设备信息/设备状态/录像查询的字段是这三个的超集，
// 模拟器只需按 CmdType 路由，因此统一用这个精简结构接收。
type Query struct {
	XMLName  xml.Name `xml:"Query"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
}

// Catalog 是目录响应（Response/Catalog）。
//
// 通道多时会按 SumNum 分片，由多条 MESSAGE 承载；每片复用同一个 SN。
type Catalog struct {
	XMLName    xml.Name   `xml:"Response"`
	CmdType    string     `xml:"CmdType"`
	SN         int        `xml:"SN"`
	DeviceID   string     `xml:"DeviceID"`
	SumNum     int        `xml:"SumNum"`
	DeviceList DeviceList `xml:"DeviceList"`
}

// DeviceList 是目录项集合。Num 是本片实际条数，SumNum 是总条数。
type DeviceList struct {
	Num   int           `xml:"Num,attr"`
	Items []CatalogItem `xml:"Item"`
}

// CatalogItem 是目录中的一条通道。
// 元素名与顺序对齐 GB/T 28181-2016 附录 A.2.6.1 与 WVP 的 DeviceChannel 定义。
type CatalogItem struct {
	DeviceID     string  `xml:"DeviceID"`
	Name         string  `xml:"Name"`
	Manufacturer string  `xml:"Manufacturer,omitempty"`
	Model        string  `xml:"Model,omitempty"`
	Owner        string  `xml:"Owner,omitempty"`
	CivilCode    string  `xml:"CivilCode,omitempty"`
	Address      string  `xml:"Address,omitempty"`
	Parental     int     `xml:"Parental"`
	ParentID     string  `xml:"ParentID,omitempty"`
	RegisterWay  int     `xml:"RegisterWay"`
	Secrecy      int     `xml:"Secrecy"`
	Status       string  `xml:"Status,omitempty"` // ON / OFF
	Longitude    float64 `xml:"Longitude,omitempty"`
	Latitude     float64 `xml:"Latitude,omitempty"`
	PTZType      int     `xml:"Info>PTZType,omitempty"`
}

// DeviceInfo 是设备信息响应（Response/DeviceInfo）。
type DeviceInfo struct {
	XMLName      xml.Name `xml:"Response"`
	CmdType      string   `xml:"CmdType"`
	SN           int      `xml:"SN"`
	DeviceID     string   `xml:"DeviceID"`
	Result       string   `xml:"Result"`
	DeviceName   string   `xml:"DeviceName,omitempty"`
	Manufacturer string   `xml:"Manufacturer,omitempty"`
	Model        string   `xml:"Model,omitempty"`
	Firmware     string   `xml:"Firmware,omitempty"`
	Channel      int      `xml:"Channel,omitempty"`
}

// DeviceStatus 是设备状态响应（Response/DeviceStatus）。
type DeviceStatus struct {
	XMLName    xml.Name `xml:"Response"`
	CmdType    string   `xml:"CmdType"`
	SN         int      `xml:"SN"`
	DeviceID   string   `xml:"DeviceID"`
	Result     string   `xml:"Result"`
	Online     string   `xml:"Online,omitempty"`
	Status     string   `xml:"Status,omitempty"`
	Encode     string   `xml:"Encode,omitempty"`
	DeviceTime string   `xml:"DeviceTime,omitempty"`
}

// Alarm 是设备主动上报的报警（Notify/Alarm）。
type Alarm struct {
	XMLName          xml.Name `xml:"Notify"`
	CmdType          string   `xml:"CmdType"`
	SN               int      `xml:"SN"`
	DeviceID         string   `xml:"DeviceID"`
	AlarmPriority    string   `xml:"AlarmPriority"`
	AlarmMethod      string   `xml:"AlarmMethod"`
	AlarmTime        string   `xml:"AlarmTime"`
	AlarmDescription string   `xml:"AlarmDescription,omitempty"`
	Longitude        float64  `xml:"Longitude,omitempty"`
	Latitude         float64  `xml:"Latitude,omitempty"`
}

// MobilePosition 是移动位置上报（Notify/MobilePosition）。
type MobilePosition struct {
	XMLName   xml.Name `xml:"Notify"`
	CmdType   string   `xml:"CmdType"`
	SN        int      `xml:"SN"`
	DeviceID  string   `xml:"DeviceID"`
	Time      string   `xml:"Time,omitempty"`
	Longitude float64  `xml:"Longitude"`
	Latitude  float64  `xml:"Latitude"`
	Speed     float64  `xml:"Speed,omitempty"`
	Direction float64  `xml:"Direction,omitempty"`
	Altitude  float64  `xml:"Altitude,omitempty"`
}

// ────────────────────────────────────────────────────────────────
// GB/T 28181-2022 查询应答
//
// 字段名与平台侧 internal/gb28181/protocol/manscdp/types_2022.go 逐字一致：
// 模拟器的价值就是「用已知正确的报文喂平台」，两边字段漂移会让回归测试失效。
// ────────────────────────────────────────────────────────────────

// PTZPositionResponse 是 PTZ 精准状态查询应答（A.2.6.15）。
//
// Pan/Tilt/Zoom 直接挂在 Response 下，不是嵌套在 <PTZPosition> 里。
type PTZPositionResponse struct {
	XMLName  xml.Name `xml:"Response"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	Pan      float64  `xml:"Pan"`
	Tilt     float64  `xml:"Tilt"`
	Zoom     float64  `xml:"Zoom"`
}

// CruiseTrackQuery 是平台下发的单条巡航轨迹查询（A.2.4.12）。
type CruiseTrackQuery struct {
	XMLName  xml.Name `xml:"Query"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	Number   int      `xml:"Number"`
}

// CruiseTrackListResponse 是巡航轨迹列表应答（A.2.6.13）。
type CruiseTrackListResponse struct {
	XMLName         xml.Name         `xml:"Response"`
	CmdType         string           `xml:"CmdType"`
	SN              int              `xml:"SN"`
	DeviceID        string           `xml:"DeviceID"`
	SumNum          int              `xml:"SumNum"`
	CruiseTrackList *CruiseTrackList `xml:"CruiseTrackList,omitempty"`
}

// CruiseTrackList 轨迹集合；Num 是属性。
type CruiseTrackList struct {
	Num    int           `xml:"Num,attr"`
	Tracks []CruiseTrack `xml:"CruiseTrack"`
}

// CruiseTrack 一条轨迹的摘要。
type CruiseTrack struct {
	Number int    `xml:"Number"`
	Name   string `xml:"Name,omitempty"`
}

// CruiseTrackResponse 是单条巡航轨迹应答（A.2.6.14）。
type CruiseTrackResponse struct {
	XMLName         xml.Name         `xml:"Response"`
	CmdType         string           `xml:"CmdType"`
	SN              int              `xml:"SN"`
	DeviceID        string           `xml:"DeviceID"`
	Number          int              `xml:"Number"`
	Name            string           `xml:"Name,omitempty"`
	CruisePointList *CruisePointList `xml:"CruisePointList,omitempty"`
}

// CruisePointList 轨迹点集合；Num 是属性。
type CruisePointList struct {
	Num    int           `xml:"Num,attr"`
	Points []CruisePoint `xml:"CruisePoint"`
}

// CruisePoint 一个巡航点（预置位 + 停留时间 + 速度）。
type CruisePoint struct {
	PresetIndex int `xml:"PresetIndex"`
	StayTime    int `xml:"StayTime"`
	Speed       int `xml:"Speed"`
}

// HomePositionResponse 是看守位查询应答（A.2.6.12）。
type HomePositionResponse struct {
	XMLName      xml.Name      `xml:"Response"`
	CmdType      string        `xml:"CmdType"`
	SN           int           `xml:"SN"`
	DeviceID     string        `xml:"DeviceID"`
	HomePosition *HomePosition `xml:"HomePosition,omitempty"`
}

// HomePosition 看守位配置。
type HomePosition struct {
	Enabled     int `xml:"Enabled"`
	ResetTime   int `xml:"ResetTime"`
	PresetIndex int `xml:"PresetIndex"`
}

// SDCardStatusResponse 是存储卡状态应答（A.2.6.16）。
type SDCardStatusResponse struct {
	XMLName          xml.Name          `xml:"Response"`
	CmdType          string            `xml:"CmdType"`
	SN               int               `xml:"SN"`
	DeviceID         string            `xml:"DeviceID"`
	SDCardStatusInfo *SDCardStatusInfo `xml:"SDCardStatusInfo,omitempty"`
}

// SDCardStatusInfo 存储卡状态块。
type SDCardStatusInfo struct {
	Num            int          `xml:"Num,attr"`
	Status         int          `xml:"Status,omitempty"`
	FormatProgress int          `xml:"FormatProgress,omitempty"`
	Cards          []SDCardInfo `xml:"SDCard"`
}

// SDCardInfo 单张存储卡；容量单位 MB。
type SDCardInfo struct {
	Capacity  int64 `xml:"Capacity"`
	FreeSpace int64 `xml:"FreeSpace"`
	Status    int   `xml:"Status,omitempty"`
}

// NewKeepalive 构造一条心跳报文。
func NewKeepalive(deviceID string, sn int) *Keepalive {
	return &Keepalive{
		CmdType:  CmdKeepalive,
		SN:       sn,
		DeviceID: deviceID,
		Status:   "OK",
	}
}
