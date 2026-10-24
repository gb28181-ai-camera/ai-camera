// Package manscdp 实现 GB/T 28181 MANSCDP XML 报文的**服务端**编解码。
//
// MANSCDP（Monitoring And Surveillance Command Description Protocol）按根元素分三类：
//
//	<Query>    平台 → 设备：目录、设备信息、设备状态、录像查询、配置下载……
//	<Response> 设备 → 平台：对上述查询的应答（目录可能按 SumNum 跨多条 MESSAGE）
//	<Notify>   设备 → 平台：心跳、报警、移动位置；平台回 200 OK 即可
//	<Control>  平台 → 设备：PTZ、录像控制、设备控制
//
// 与 tools/gbsim/manscdp 的分工：那边只需要「解平台查询、编设备应答」，
// 这边是平台侧，需要「编查询/控制、解应答/通知」，因此结构体集合不同。
// 两边保持报文格式一致（联调即靠这个一致性）。
//
// ⚠️ 中文以 GB2312 字节进入 XML body（设计 §2.6）。本包的所有编解码都经过
// charset 包，**声明与字节必须一致**；无声明且非法 UTF-8 的报文一律报错，
// 由调用方记 ac_gb_sip_log 后拒绝，绝不允许静默解成乱码落库。
package manscdp

import "encoding/xml"

// CmdType 取值（GB/T 28181-2016 附录 A）。
//
// 注意 Control 类报文的 CmdType 恒为 DeviceControl，
// 具体是哪条指令由 PTZCmd / RecordCmd / TeleBoot 等子元素区分。
const (
	CmdCatalog        = "Catalog"
	CmdDeviceInfo     = "DeviceInfo"
	CmdDeviceStatus   = "DeviceStatus"
	CmdKeepalive      = "Keepalive"
	CmdAlarm          = "Alarm"
	CmdMobilePosition = "MobilePosition"
	CmdControl        = "DeviceControl"
	CmdRecordInfo     = "RecordInfo"
	CmdBroadcast      = "Broadcast"
	CmdConfigDownload = "ConfigDownload"
	CmdPresetQuery    = "PresetQuery"
	CmdMediaStatus    = "MediaStatus"
)

// Control 子元素取值。
const (
	RecordCmdStart = "Record"     // 开始录像
	RecordCmdStop  = "StopRecord" // 停止录像
	GuardCmdSet    = "SetGuard"
	GuardCmdReset  = "ResetGuard"
	AlarmCmdReset  = "ResetAlarm"
	IFameCmdSend   = "Send"
	IFameCmdStop   = "Stop"
	TeleBoot       = "Boot"
)

// 会话名（SDP s=），回放复用同一套 SDP 构造。
const (
	SessionNamePlay     = "Play"
	SessionNamePlayback = "Playback"
	SessionNameDownload = "Download"
)

// Envelope 是「先看是什么报文，再决定解成哪个结构」的公共字段。
//
// XMLName 不带 tag，因此 Query / Response / Notify 三种根元素都能解进来。
type Envelope struct {
	XMLName  xml.Name
	CmdType  string `xml:"CmdType"`
	SN       int    `xml:"SN"`
	DeviceID string `xml:"DeviceID"`

	// 以下两个字段不参与 XML 编解码，用于把解码现场的字符集信息带给调用方。
	DeclaredCharset string `xml:"-"`
	CharsetMismatch bool   `xml:"-"`
}

// Root 返回报文的根元素名（Query / Response / Notify / Control）。
func (e *Envelope) Root() string {
	if e == nil {
		return ""
	}
	return e.XMLName.Local
}

// ────────────────────────────────────────────────────────────────
// 平台 → 设备：查询
// ────────────────────────────────────────────────────────────────

// Query 是通用的查询报文。Catalog / DeviceInfo / DeviceStatus /
// ConfigDownload / PresetQuery 的字段都是它的子集。
type Query struct {
	XMLName  xml.Name `xml:"Query"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
}

// RecordInfoQuery 录像查询（比通用 Query 多时间窗与类型）。
type RecordInfoQuery struct {
	XMLName   xml.Name `xml:"Query"`
	CmdType   string   `xml:"CmdType"`
	SN        int      `xml:"SN"`
	DeviceID  string   `xml:"DeviceID"`
	StartTime string   `xml:"StartTime"`
	EndTime   string   `xml:"EndTime"`
	Secrecy   int      `xml:"Secrecy,omitempty"`
	Type      string   `xml:"Type,omitempty"`
}

// ConfigDownloadQuery 配置下载查询。
type ConfigDownloadQuery struct {
	XMLName    xml.Name `xml:"Query"`
	CmdType    string   `xml:"CmdType"`
	SN         int      `xml:"SN"`
	DeviceID   string   `xml:"DeviceID"`
	ConfigType string   `xml:"ConfigType"`
}

// ────────────────────────────────────────────────────────────────
// 平台 → 设备：控制
// ────────────────────────────────────────────────────────────────

// Control 是设备控制报文。各类指令共用一个结构，按字段是否为空区分。
//
// PTZCmd 是 **8 字节二进制指令**（0xA5 0x0F 0x01 ... 校验和），不是文本。
// XML 本身不允许这种字节，但国标就是这么规定的，WVP 同样把原始字节塞进元素内容。
// 因此编码时用占位符替换（见 codec.go），解码时保持原始字节。
type Control struct {
	XMLName  xml.Name `xml:"Control"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`

	PTZCmd    []byte `xml:"-"`                   // 8 字节原始指令，编码时特殊处理
	RecordCmd string `xml:"RecordCmd,omitempty"` // Record / StopRecord
	TeleBoot  string `xml:"TeleBoot,omitempty"`  // Boot
	GuardCmd  string `xml:"GuardCmd,omitempty"`  // SetGuard / ResetGuard
	AlarmCmd  string `xml:"AlarmCmd,omitempty"`  // ResetAlarm
	IFameCmd  string `xml:"IFameCmd,omitempty"`  // Send / Stop
	DragZoom  string `xml:"DragZoom,omitempty"`  // 拖拽放大

	// PTZPrecise 是 GB/T 28181-2022 的精准控制子元素（A.2.3.1.11）。
	//
	// 与 PTZCmd 并列而不是替换：2016 设备只认 PTZCmd，2022 设备两者都认。
	// 是否使用本字段由 version.Supports(V2022, FeaturePTZPreciseCtrl) 判定。
	PTZPrecise *PTZPreciseCtrl `xml:"PTZPreciseCtrl,omitempty"`
}

// ────────────────────────────────────────────────────────────────
// 平台 → 设备：语音广播
// ────────────────────────────────────────────────────────────────

// Broadcast 是语音广播/对讲通知（Notify/Broadcast）。
type Broadcast struct {
	XMLName  xml.Name `xml:"Notify"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	SourceID string   `xml:"SourceID,omitempty"`
	TargetID string   `xml:"TargetID,omitempty"`
}

// ────────────────────────────────────────────────────────────────
// 设备 → 平台：通知
// ────────────────────────────────────────────────────────────────

// Keepalive 是设备心跳（Notify/Keepalive）。
type Keepalive struct {
	XMLName  xml.Name `xml:"Notify"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	Status   string   `xml:"Status,omitempty"` // OK / ERROR
}

// Alarm 是设备主动上报的报警（Notify/Alarm）。
type Alarm struct {
	XMLName          xml.Name `xml:"Notify"`
	CmdType          string   `xml:"CmdType"`
	SN               int      `xml:"SN"`
	DeviceID         string   `xml:"DeviceID"`
	AlarmPriority    string   `xml:"AlarmPriority"`
	AlarmMethod      string   `xml:"AlarmMethod"` // 数字串，1–9
	AlarmTime        string   `xml:"AlarmTime"`
	AlarmDescription string   `xml:"AlarmDescription,omitempty"`
	AlarmType        string   `xml:"AlarmType,omitempty"`
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

// MediaStatus 是媒体状态通知（Notify/MediaStatus），本期只记录不落库。
type MediaStatus struct {
	XMLName     xml.Name `xml:"Notify"`
	CmdType     string   `xml:"CmdType"`
	SN          int      `xml:"SN"`
	DeviceID    string   `xml:"DeviceID"`
	NotifyType  string   `xml:"NotifyType"`
	DeviceTime  string   `xml:"DeviceTime"`
	VideoStream string   `xml:"VideoStream,omitempty"`
	AudioStream string   `xml:"AudioStream,omitempty"`
	OnlineNum   int      `xml:"OnlineNum,omitempty"`
}

// ────────────────────────────────────────────────────────────────
// 设备 → 平台：查询应答
// ────────────────────────────────────────────────────────────────

// CatalogResponse 目录响应。通道多时按 SumNum 分片，由多条 MESSAGE 承载，
// 每片复用同一个 SN；Num 是本片条数，SumNum 是总条数。
type CatalogResponse struct {
	XMLName    xml.Name   `xml:"Response"`
	CmdType    string     `xml:"CmdType"`
	SN         int        `xml:"SN"`
	DeviceID   string     `xml:"DeviceID"`
	SumNum     int        `xml:"SumNum"`
	DeviceList DeviceList `xml:"DeviceList"`
}

// DeviceList 目录项集合。
type DeviceList struct {
	Num   int           `xml:"Num,attr"`
	Items []CatalogItem `xml:"Item"`
}

// CatalogItem 一条通道。字段顺序对齐 GB/T 28181-2016 附录 A.2.6.1。
type CatalogItem struct {
	DeviceID     string `xml:"DeviceID"`
	Name         string `xml:"Name"`
	Manufacturer string `xml:"Manufacturer,omitempty"`
	Model        string `xml:"Model,omitempty"`
	Owner        string `xml:"Owner,omitempty"`
	CivilCode    string `xml:"CivilCode,omitempty"`
	Address      string `xml:"Address,omitempty"`
	Parental     int    `xml:"Parental"`
	ParentID     string `xml:"ParentID,omitempty"`
	RegisterWay  int    `xml:"RegisterWay"`
	Secrecy      int    `xml:"Secrecy"`
	Status       string `xml:"Status,omitempty"` // ON / OFF
	// Event 是目录变更事件（ADD/UPDATE/DEL/ON/OFF/VLOST，2016 已定义、2022 沿用）。
	//
	// 与 Status 的区别：Status 是「当前状态」，Event 是「这次发生了什么变化」。
	// 两者可能并存（例如 Event=ADD 且 Status=ON）。语义映射见 catalog_event.go。
	Event       string  `xml:"Event,omitempty"`
	Longitude   float64 `xml:"Longitude,omitempty"`
	Latitude    float64 `xml:"Latitude,omitempty"`
	ChannelType int     `xml:"Info>ChannelType,omitempty"`
	PTZType     int     `xml:"Info>PTZType,omitempty"`
}

// DeviceInfoResponse 设备信息响应。
type DeviceInfoResponse struct {
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

// DeviceStatusResponse 设备状态响应。
type DeviceStatusResponse struct {
	XMLName    xml.Name `xml:"Response"`
	CmdType    string   `xml:"CmdType"`
	SN         int      `xml:"SN"`
	DeviceID   string   `xml:"DeviceID"`
	Result     string   `xml:"Result"`
	Online     string   `xml:"Online,omitempty"`
	Status     string   `xml:"Status,omitempty"`
	Encode     string   `xml:"Encode,omitempty"`
	Record     string   `xml:"Record,omitempty"`
	DeviceTime string   `xml:"DeviceTime,omitempty"`
}

// RecordInfoResponse 录像查询响应。
type RecordInfoResponse struct {
	XMLName    xml.Name   `xml:"Response"`
	CmdType    string     `xml:"CmdType"`
	SN         int        `xml:"SN"`
	DeviceID   string     `xml:"DeviceID"`
	Name       string     `xml:"Name,omitempty"`
	SumNum     int        `xml:"SumNum"`
	RecordList RecordList `xml:"RecordList"`
}

// RecordList 录像条目集合。
type RecordList struct {
	Num   int          `xml:"Num,attr"`
	Items []RecordItem `xml:"Item"`
}

// RecordItem 一条录像记录。
type RecordItem struct {
	DeviceID  string `xml:"DeviceID"`
	Name      string `xml:"Name,omitempty"`
	FilePath  string `xml:"FilePath,omitempty"`
	Address   string `xml:"Address,omitempty"`
	StartTime string `xml:"StartTime"`
	EndTime   string `xml:"EndTime"`
	Secrecy   int    `xml:"Secrecy,omitempty"`
	Type      string `xml:"Type,omitempty"`
}

// PresetQueryResponse 预置位查询响应。
type PresetQueryResponse struct {
	XMLName    xml.Name   `xml:"Response"`
	CmdType    string     `xml:"CmdType"`
	SN         int        `xml:"SN"`
	DeviceID   string     `xml:"DeviceID"`
	PresetList PresetList `xml:"PresetList"`
}

// PresetList 预置位集合。
type PresetList struct {
	Num   int          `xml:"Num,attr"`
	Items []PresetItem `xml:"Item"`
}

// PresetItem 一条预置位。
type PresetItem struct {
	PresetID   int    `xml:"PresetID"`
	PresetName string `xml:"PresetName,omitempty"`
}

// ConfigDownloadResponse 配置下载响应。
type ConfigDownloadResponse struct {
	XMLName    xml.Name `xml:"Response"`
	CmdType    string   `xml:"CmdType"`
	SN         int      `xml:"SN"`
	DeviceID   string   `xml:"DeviceID"`
	Result     string   `xml:"Result"`
	ConfigType string   `xml:"ConfigType,omitempty"`
	BasicParam string   `xml:"BasicParam,omitempty"`
}
