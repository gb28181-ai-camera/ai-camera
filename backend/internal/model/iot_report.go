package model

import "time"

// IotDevicePosition 是 WebRTC/AIChat 设备通过 MQTT 上报的位置历史。
//
// 为什么不复用 ac_gb_mobile_position：那张表以 gb_device_id 为键，属于
// GB28181 协议域；MQTT 设备没有国标编码，两者混用会让查询与数据权限失真。
// 当前设备的最新位置同时冗余在 ac_device.latitude/longitude，便于列表直接展示。
type IotDevicePosition struct {
	BaseModel
	DeviceID   int64     `gorm:"column:device_id;not null;index:idx_ac_iot_device_position_device_dt,priority:1;comment:设备ID" json:"device_id"`
	DeviceCode string    `gorm:"column:device_code;size:100;comment:设备编号(冗余)" json:"device_code"`
	Longitude  float64   `gorm:"column:longitude;not null;comment:经度" json:"longitude"`
	Latitude   float64   `gorm:"column:latitude;not null;comment:纬度" json:"latitude"`
	Speed      float64   `gorm:"column:speed;comment:速度(km/h)" json:"speed"`
	Direction  float64   `gorm:"column:direction;comment:方向(0-360)" json:"direction"`
	Altitude   float64   `gorm:"column:altitude;comment:海拔(m)" json:"altitude"`
	ReportedDT time.Time `gorm:"column:reported_dt;index:idx_ac_iot_device_position_device_dt,priority:2;comment:上报时间" json:"reported_dt"`
	DeptID     *int64    `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

// TableName 表名。
func (IotDevicePosition) TableName() string { return "ac_iot_device_position" }

func init() {
	RegisterModel(&IotDevicePosition{})
}
