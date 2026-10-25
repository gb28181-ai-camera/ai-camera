package request

// DeviceReportMsg 是 MQTT 设备上报的统一消息体。
//
// 话题：iot/<group_code>/device/<device_code>/report
// 设备用「产品 × MQTT 服务」凭据发布；后端按 device_code + group_code 校验归属。
//
// type 取值：
//   - heartbeat：心跳，只刷新在线状态；
//   - battery：电量/电压/剩余存储；
//   - position：经纬度/速度/方向/海拔；
//   - alarm：告警（alarm_type_code 对应 ac_alarm_type.code）。
type DeviceReportMsg struct {
	Type          string `json:"type"`
	DeviceCode    string `json:"device_code"`
	Timestamp     int64  `json:"ts,omitempty"`
	OccurredDT    string `json:"occurred_dt,omitempty"`
	AlarmTypeCode string `json:"alarm_type_code,omitempty"`
	AlarmLevel    string `json:"alarm_level,omitempty"`
	Content       string `json:"content,omitempty"`

	Battery     *int   `json:"battery,omitempty"`
	Voltage     string `json:"voltage,omitempty"`
	StorageFree *int64 `json:"storage_free,omitempty"`

	Longitude *float64 `json:"longitude,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Speed     *float64 `json:"speed,omitempty"`
	Direction *float64 `json:"direction,omitempty"`
	Altitude  *float64 `json:"altitude,omitempty"`
}

// RegisterPushTokenReq App 上报个推 CID。
type RegisterPushTokenReq struct {
	CID        string `json:"cid" binding:"required" example:"cid-xxxx"`
	Platform   string `json:"platform" example:"android"`
	AppVersion string `json:"app_version" example:"0.6.0"`
}
