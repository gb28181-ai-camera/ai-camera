package bootstrap

import (
	"log"

	"github.com/boticz/ai-camera-backend/internal/model"
)

// AutoMigrate 执行数据库增量迁移。
//
// 设计约定：初次建表由 docs/开发文档/数据库设计/sql/ddl/*.sql 完成；
// 此处一次性调用 GORM AutoMigrate 做增量迁移（新增表、新增列、列类型变更）。
// 与逐表循环不同：任一 model 失败即中止（fail-fast），避免静默跳过导致表结构不一致。
func AutoMigrate() {
	if globalDB == nil {
		log.Fatalf("AutoMigrate 失败: 数据库未初始化")
	}

	models := []interface{}{
		// ── 系统基础 ──
		&model.Dept{},
		&model.User{},
		&model.Role{},
		&model.Menu{},
		&model.MenuAPI{},
		&model.Post{},
		&model.UserRoleRel{},
		&model.UserPostRel{},
		&model.PostRoleRel{},
		&model.DeptRoleRel{},
		&model.RoleMenuRel{},
		&model.SystemConfig{},
		&model.ApiKey{},
		&model.OperationLog{},
		&model.Notification{},
		&model.Sequence{},

		// ── 基础信息 ──
		&model.Region{},
		&model.RiskType{},
		&model.AlarmType{},

		// ── 资源管理 ──
		&model.DeviceMode{},
		&model.FirmwareType{},
		&model.Firmware{},
		&model.AIModel{},
		&model.IotCard{},
		&model.Device{},
		&model.IotCardReplaceLog{},
		&model.VoiceConfig{},
		&model.DeviceHistory{},

		// ── 风险与告警 ──
		&model.RiskRecord{},
		&model.AlarmRecord{},

		// ── 巡检 ──
		&model.InspectTask{},
		&model.InspectRecord{},

		// ── 升级管理 ──
		&model.DeviceUpgradePlan{},
		&model.DeviceUpgradeRecord{},

		// ── 告警设置 ──
		&model.AlarmSetting{},
		&model.RiskNotifySetting{},
		&model.ThirdPartyConfig{},
		&model.WebhookConfig{},

		// ── IOT 管理（v0.3.0）──
		// 设备复用 ac_device（Device 已在上方注册），此处只新增 6 张 IOT 表。
		// 顺序：产品 → 服务组 → 服务项 → 产品绑定 → 设备账号 → 设备事件，
		// 与 DDL 10-IOT管理.sql 保持一致，便于人工对照。
		&model.IotProduct{},
		&model.IotServiceGroup{},
		&model.IotServiceItem{},
		&model.IotProductService{},
		&model.IotDeviceAccount{},
		&model.IotDeviceEvent{},

		// ── 国标视频（GB/T 28181，v0.2.0）──
		// 注意：ac_gb_platform_group / ac_gb_platform_region 为纯关联表（无 active / 无 dept_id），
		// 仍在列表中以便 AutoMigrate 建表与索引。GbSipLog 不嵌 BaseModel（追加写日志表）。
		&model.GbChannel{},
		&model.GbGroup{},
		&model.GbMediaServer{},
		&model.GbStreamProxy{},
		&model.GbStreamPush{},
		&model.GbCloudRecord{},
		&model.GbRecordPlan{},
		&model.GbRecordPlanItem{},
		&model.GbAlarm{},
		&model.GbMobilePosition{},
		&model.GbSubscribe{},
		&model.GbSipLog{},
		&model.VoiceSession{},
		&model.VoiceProvider{},
		&model.GbPlatform{},
		&model.GbPlatformChannel{},
		&model.GbPlatformGroup{},
		&model.GbPlatformRegion{},

		// ── JT/T 1078 车载终端（v0.2.0，三期范围）──
	}

	if err := globalDB.AutoMigrate(models...); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
}
