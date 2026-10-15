package request

// ════════════════════════════════════════════════════════════════
// 升级管理：固件类型 / 固件 / AI模型 / 升级下发 / 升级计划 / 升级记录
// ════════════════════════════════════════════════════════════════

// ── 固件类型 ──

// PageFirmwareTypeReq 固件类型分页查询参数。
type PageFirmwareTypeReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword" example:"主控固件"`
}

// CreateFirmwareTypeReq 创建固件类型。
type CreateFirmwareTypeReq struct {
	Name        string `json:"name" binding:"required" example:"主控固件"`
	Code        string `json:"code" example:"main"`
	Sequence    int    `json:"sequence" example:"10"`
	Description string `json:"description" example:"设备主控程序固件"`
	Active      *bool  `json:"active" example:"true"`
}

// UpdateFirmwareTypeReq 更新固件类型。
type UpdateFirmwareTypeReq struct {
	Name        string `json:"name" binding:"required" example:"主控固件"`
	Code        string `json:"code" example:"main"`
	Sequence    int    `json:"sequence" example:"10"`
	Description string `json:"description" example:"设备主控程序固件"`
	Active      *bool  `json:"active" example:"true"`
}

// ── 固件 ──

// PageFirmwareReq 固件分页查询参数。
type PageFirmwareReq struct {
	PageReq
	Keyword        string `form:"keyword" json:"keyword" example:"v1.2.0"`
	FirmwareTypeID *int64 `form:"firmware_type_id" json:"firmware_type_id" example:"1"`
}

// CreateFirmwareReq 创建固件。
type CreateFirmwareReq struct {
	Name           string `json:"name" binding:"required" example:"主控固件V1.2.0"`
	Version        string `json:"version" binding:"required" example:"v1.2.0"`
	FirmwareTypeID *int64 `json:"firmware_type_id" example:"1"`
	FileURL        string `json:"file_url" example:"/uploads/firmware/v1.2.0.bin"`
	FileSize       int64  `json:"file_size" example:"1048576"`
	Md5            string `json:"md5" example:"d41d8cd98f00b204e9800998ecf8427e"`
	ReleaseDT      string `json:"release_dt" example:"2026-09-20 10:00:00"`
	Description    string `json:"description" example:"修复夜间离线问题"`
	Active         *bool  `json:"active" example:"true"`
}

// UpdateFirmwareReq 更新固件。
type UpdateFirmwareReq struct {
	Name           string `json:"name" binding:"required" example:"主控固件V1.2.0"`
	Version        string `json:"version" binding:"required" example:"v1.2.0"`
	FirmwareTypeID *int64 `json:"firmware_type_id" example:"1"`
	FileURL        string `json:"file_url" example:"/uploads/firmware/v1.2.0.bin"`
	FileSize       int64  `json:"file_size" example:"1048576"`
	Md5            string `json:"md5" example:"d41d8cd98f00b204e9800998ecf8427e"`
	ReleaseDT      string `json:"release_dt" example:"2026-09-20 10:00:00"`
	Description    string `json:"description" example:"修复夜间离线问题"`
	Active         *bool  `json:"active" example:"true"`
}

// ── AI 模型 ──

// PageAIModelReq AI模型分页查询参数。
type PageAIModelReq struct {
	PageReq
	Keyword    string `form:"keyword" json:"keyword" example:"安全帽识别"`
	FirmwareID *int64 `form:"firmware_id" json:"firmware_id" example:"1"`
}

// CreateAIModelReq 创建AI模型。模型必须关联适用固件。
type CreateAIModelReq struct {
	Name        string `json:"name" binding:"required" example:"安全帽识别"`
	Version     string `json:"version" binding:"required" example:"v1.0.0"`
	FirmwareID  *int64 `json:"firmware_id" binding:"required" example:"1"`
	FileURL     string `json:"file_url" example:"/uploads/model/helmet.zip"`
	FileSize    int64  `json:"file_size" example:"20971520"`
	Md5         string `json:"md5" example:"d41d8cd98f00b204e9800998ecf8427e"`
	ReleaseDT   string `json:"release_dt" example:"2026-09-20 10:00:00"`
	Description string `json:"description" example:"识别未佩戴安全帽行为"`
	Active      *bool  `json:"active" example:"true"`
}

// UpdateAIModelReq 更新AI模型。
type UpdateAIModelReq struct {
	Name        string `json:"name" binding:"required" example:"安全帽识别"`
	Version     string `json:"version" binding:"required" example:"v1.0.0"`
	FirmwareID  *int64 `json:"firmware_id" binding:"required" example:"1"`
	FileURL     string `json:"file_url" example:"/uploads/model/helmet.zip"`
	FileSize    int64  `json:"file_size" example:"20971520"`
	Md5         string `json:"md5" example:"d41d8cd98f00b204e9800998ecf8427e"`
	ReleaseDT   string `json:"release_dt" example:"2026-09-20 10:00:00"`
	Description string `json:"description" example:"识别未佩戴安全帽行为"`
	Active      *bool  `json:"active" example:"true"`
}

// ── 升级下发 ──

// UpgradeDevicesReq 设备升级下发（现在升级 / 定时升级）。
type UpgradeDevicesReq struct {
	DeviceIDs   []int64 `json:"device_ids" binding:"required,min=1" example:"1,2,3"`
	FirmwareID  *int64  `json:"firmware_id" example:"1"`
	AIModelID   *int64  `json:"ai_model_id" example:"2"`
	UpgradeType string  `json:"upgrade_type" binding:"required,oneof=now plan" example:"now"`
	PlanDT      string  `json:"plan_dt" example:"2026-09-25 02:00:00"`
}

// ── 升级计划 / 升级记录 ──

// PageUpgradePlanReq 升级计划分页查询参数。
type PageUpgradePlanReq struct {
	PageReq
	Keyword  string `form:"keyword" json:"keyword" example:"CAM-0001"`
	RegionID *int64 `form:"region_id" json:"region_id" example:"1"`
	DeviceID *int64 `form:"device_id" json:"device_id" example:"10"`
	Status   string `form:"status" json:"status" example:"pending"`
}

// PageUpgradeRecordReq 升级记录分页查询参数。时间区间按升级时间 upgrade_dt 过滤。
type PageUpgradeRecordReq struct {
	PageReq
	Keyword  string `form:"keyword" json:"keyword" example:"CAM-0001"`
	RegionID *int64 `form:"region_id" json:"region_id" example:"1"`
	DeviceID *int64 `form:"device_id" json:"device_id" example:"10"`
	StartDT  string `form:"start_dt" json:"start_dt" example:"2026-09-01"`
	EndDT    string `form:"end_dt" json:"end_dt" example:"2026-09-30"`
}
