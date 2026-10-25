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
		// 设备复用 ac_device（Device 已在上方注册），此处只新增 7 张 IOT 表。
		// 顺序：产品 → 服务组 → 服务 → 产品-服务组 → 产品-服务 → 设备账号 → 设备事件，
		// 与 DDL 10-IOT管理.sql 保持一致，便于人工对照。
		&model.IotProduct{},
		&model.IotServiceGroup{},
		&model.IotService{},
		&model.IotProductServiceGroup{},
		&model.IotProductService{},
		&model.IotDeviceAccount{},
		&model.IotDeviceEvent{},
		// coturn SQL userdb（产品×TURN 服务凭据同步目标）。
		&model.TurnUserLT{},
		&model.TurnSecret{},

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

	// 模块自注册模型：新增模块在自己的 model 文件里调用 model.RegisterModel(&X{})，
	// 无需改动本文件，避免多人/多 agent 并行开发时集中修改同一处列表。
	models = append(models, model.ExtraModels()...)

	if err := globalDB.AutoMigrate(models...); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
}
