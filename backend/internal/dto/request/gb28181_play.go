package request

// ════════════════════════════════════════════════════════════════
// 国标视频（GB/T 28181）—— 实时播放 / 云台 / 语音请求类型
//
// 字段级契约见 docs/开发文档/后端开发/00-接口契约.md §15.1 / §15.2 / §15.4。
// 与本文件之外的 gb28181.go 分开，避免与已冻结的 CRUD 请求类型互相影响。
// ════════════════════════════════════════════════════════════════

// GbPlayReq 实时点播请求（契约 §15.1）。
type GbPlayReq struct {
	// Protocol 播放协议：webrtc | flv | hls，缺省 webrtc。
	//
	// 协议只影响前端选择播放器，后端始终返回三种地址；保留该字段是为了
	// 记录本次会话的偏好，并在状态查询时回显。
	Protocol string `json:"protocol" example:"webrtc"`
	// StreamID 可选：重用已有流。非空时后端会优先按该标识查找会话。
	StreamID string `json:"stream_id"`
}

// GbPtzReq 云台控制请求（契约 §15.2）。
//
// ★ Direction 与 Action 互斥：两者都传、或都不传，都必须报错。
// 允许「都不传」会让前端按钮失效变成静默无操作，这是最难排的一类问题。
type GbPtzReq struct {
	// Direction 八方向 + 停止：up|down|left|right|up-left|up-right|down-left|down-right|stop。
	Direction string `json:"direction" example:"up"`
	// Speed 速度量纲 1–8（1 最慢、8 最快），缺省 5。
	// 注意：不是 0–255，后端负责映射到 PTZCmd 的速度字节。
	Speed int `json:"speed" example:"5"`
	// Action 变焦/光圈/聚焦：zoom-in|zoom-out|zoom-stop|iris-in|iris-out|iris-stop|
	// focus-near|focus-far|focus-stop。
	Action string `json:"action" example:"zoom-in"`
}

// GbPresetReq 预置位请求（契约 §15.2）。
//
// Action=set（AddPreset）与路径删除（DeletePreset）。Name 是平台侧展示名，
// 持久化到 ac_gb_ptz_preset（表见 model.GbPtzPreset），不写入设备。
type GbPresetReq struct {
	PresetID int    `json:"preset_id" example:"1"`
	Name     string `json:"name" example:"大门"`
	Action   string `json:"action" example:"set"`
}

// GbPtzCruiseReq 巡航请求（契约 §15.2）。
type GbPtzCruiseReq struct {
	// Action start | stop。
	Action string `json:"action" example:"start"`
	// GroupID 巡航组号 1–255。
	GroupID int `json:"group_id" example:"1"`
}

// GbPtzScanReq 扫描请求（契约 §15.2）。
type GbPtzScanReq struct {
	// Action start | stop。
	Action string `json:"action" example:"start"`
	// GroupID 扫描组号 1–255（部分设备用组号选择扫描路线）。
	GroupID int `json:"group_id" example:"1"`
}

// GbPtzPreciseReq PTZ 精准控制请求（GB/T 28181-2022，A.2.3.1.11）。
//
// 与 2016 的 8 字节 PTZCmd「方向 + 速度」不同，PTZPreciseCtrl 是
// 「绝对角度 + 倍率」的定位控制，因此三个字段都是 double 且可只传其一。
//
// ★ 三个字段用指针：0 是合法值（云台回正 Pan=0、Tilt=0），必须能区分
// 「未传」与「传了 0」。三者全不传时服务层直接 400 —— 空指令既不会让设备
// 动作，也会让前端误以为「已经控制成功」。
type GbPtzPreciseReq struct {
	// Pan 水平角度（度）。取值范围由设备决定，平台不做臆测性限制。
	Pan *float64 `json:"pan" example:"10.5"`
	// Tilt 垂直角度（度）。
	Tilt *float64 `json:"tilt" example:"20"`
	// Zoom 变焦倍数（1 表示 1 倍）。
	Zoom *float64 `json:"zoom" example:"2.5"`
}

// GbPtzWiperReq 雨刷请求。
//
// 契约未单列字段，路由为 POST /ptz/:channelId/wiper；用 On 表达开/关，
// 同时兼容 query 参数 on=true/false。
type GbPtzWiperReq struct {
	On bool `json:"on" example:"true"`
}

// GbVoiceStartReq 语音会话启动请求。
//
// 广播忽略 AgentEnabled；对讲用它表达「是否接入 Pipecat 语音 Agent」。
// 控制面未配置/不可达时后端会自动降级为纯对讲，并在响应里回 agent_enabled=false。
type GbVoiceStartReq struct {
	AgentEnabled bool `json:"agent_enabled" example:"false"`
}

// GbVoiceStopReq 语音会话停止请求。
//
// 路由只带 channelId；session_id 可通过请求体或 query 传入，缺省时后端
// 按通道查找进行中的同类会话（前端只需点击「停止」即可）。
type GbVoiceStopReq struct {
	SessionID string `json:"session_id"`
}
