package response

import "github.com/boticz/ai-camera-backend/internal/model"

// PageAlarmRecordResp 告警记录分页载荷。
//
// Items 中的每条记录均预加载 AlarmType / Device / Region，
// 前端可直接读取 alarm_type.name、device.name、device.device_code、region.name。
type PageAlarmRecordResp struct {
	Items    []model.AlarmRecord `json:"items"`
	Total    int64               `json:"total" example:"64"`
	Page     int                 `json:"page" example:"1"`
	PageSize int                 `json:"page_size" example:"20"`
}

// RPageAlarmRecord 告警记录分页响应。
type RPageAlarmRecord struct {
	BaseResponse
	Data PageAlarmRecordResp `json:"data"`
}

// RAlarmRecord 告警记录详情响应。
type RAlarmRecord struct {
	BaseResponse
	Data model.AlarmRecord `json:"data"`
}
