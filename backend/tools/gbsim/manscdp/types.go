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

// NewKeepalive 构造一条心跳报文。
func NewKeepalive(deviceID string, sn int) *Keepalive {
	return &Keepalive{
		CmdType:  CmdKeepalive,
		SN:       sn,
		DeviceID: deviceID,
		Status:   "OK",
	}
}
