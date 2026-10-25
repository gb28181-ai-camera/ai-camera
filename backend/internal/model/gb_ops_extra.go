package model

// ════════════════════════════════════════════════════════════════
// 国标运维增强（Wave 3）—— 新增数据模型
//
// 本文件只放「WVP 对齐补齐」所需的两张表：
//   - ac_gb_record_merge_task：云端录像多段合并任务
//   - ac_gb_share_token：通道免登录分享令牌（库中只存 hash）
//
// 模型通过 init() 自注册到同级开发者的迁移清单（见 extra.go），
// 避免多人并行时共同修改 bootstrap/migrate.go 产生冲突。
//
// 注意：init SQL / DDL 由主会话统一补齐；本文件只保证 AutoMigrate
// 能建出与设计一致的表结构。
// ════════════════════════════════════════════════════════════════

import "time"

// 云端录像合并任务状态。
const (
	GbMergeStatusPending = "pending" // 已创建，等待执行
	GbMergeStatusRunning = "running" // ffmpeg 合并中
	GbMergeStatusSuccess = "success" // 合并完成
	GbMergeStatusFailed  = "failed"  // 合并失败（含未安装 ffmpeg）
)

// GbRecordMergeTask 云端录像多段合并任务。表 ac_gb_record_merge_task。
//
// 设计取舍：
//   - 合并是「一次性运维动作」，结果落成一个 MP4，不做切片与转码；
//   - 任务状态机 pending → running → success/failed，失败原因写 error_msg；
//   - RecordIDs 以逗号分隔的整数存储，避免再建一张关联表 —— 任务不参与
//     「按录像反查任务」的查询，关联表没有收益。
type GbRecordMergeTask struct {
	BaseModel
	Name   string `gorm:"column:name;size:255;comment:任务名称(默认按首段录像命名)" json:"name"`
	Status string `gorm:"column:status;size:20;default:pending;index;comment:状态 pending/running/success/failed" json:"status"`

	// RecordIDs 逗号分隔的 ac_gb_cloud_record.id，按合并顺序保存。
	RecordIDs   string `gorm:"column:record_ids;type:text;comment:合并的录像ID列表(逗号分隔,顺序即合并顺序)" json:"record_ids"`
	RecordCount int    `gorm:"column:record_count;default:0;comment:录像段数" json:"record_count"`

	FileName string `gorm:"column:file_name;size:255;comment:合并后的文件名" json:"file_name"`
	FilePath string `gorm:"column:file_path;size:1000;comment:合并后的文件路径" json:"file_path"`
	FileSize int64  `gorm:"column:file_size;comment:合并后的文件大小(字节)" json:"file_size"`
	// DurationSec 是各段 TimeLen 的合计（不读取合并后文件的真实时长）。
	DurationSec int64 `gorm:"column:duration_sec;comment:预计总时长(秒)" json:"duration_sec"`

	ErrorMsg string `gorm:"column:error_msg;size:2000;comment:失败原因" json:"error_msg"`

	StartedDT  *time.Time `gorm:"column:started_dt;comment:开始执行时间" json:"started_dt"`
	FinishedDT *time.Time `gorm:"column:finished_dt;comment:结束时间(成功或失败)" json:"finished_dt"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (GbRecordMergeTask) TableName() string { return "ac_gb_record_merge_task" }

// GbShareToken 通道免登录分享令牌。表 ac_gb_share_token。
//
// 安全设计：
//   - 令牌明文只在创建响应中返回一次，库中仅存 SHA-256（hex，64 位）；
//   - TokenPrefix 保存明文前 10 位，供管理页辨认「是哪一条令牌」；
//   - 匿名接口不复用用户 JWT，只按 token hash 独立校验；
//   - 过期与吊销分离：过期由 expires_dt 判断，吊销是显式动作（保留审计）。
type GbShareToken struct {
	BaseModel
	ChannelID int64 `gorm:"column:channel_id;not null;index;comment:通道ID(ac_gb_channel)" json:"channel_id"`

	// TokenHash 是令牌的 SHA-256 hex；唯一索引保证同一令牌只有一条有效记录。
	TokenHash string `gorm:"column:token_hash;size:64;not null;uniqueIndex:idx_ac_gb_share_token_hash;comment:令牌SHA-256(唯一)" json:"-"`
	// TokenPrefix 为明文前缀，便于排查时定位令牌（不足以还原明文）。
	TokenPrefix string `gorm:"column:token_prefix;size:16;comment:令牌明文前缀(展示用)" json:"token_prefix"`

	ExpiresDT time.Time  `gorm:"column:expires_dt;index;comment:过期时间" json:"expires_dt"`
	Revoked   *bool      `gorm:"column:revoked;default:false;comment:是否已吊销" json:"revoked"`
	RevokedDT *time.Time `gorm:"column:revoked_dt;comment:吊销时间" json:"revoked_dt"`

	UseCount   int64      `gorm:"column:use_count;default:0;comment:匿名播放次数" json:"use_count"`
	LastUsedDT *time.Time `gorm:"column:last_used_dt;comment:最近一次使用时间" json:"last_used_dt"`

	Remark string `gorm:"column:remark;size:500;comment:备注" json:"remark"`
	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (GbShareToken) TableName() string { return "ac_gb_share_token" }

func init() {
	RegisterModel(&GbRecordMergeTask{}, &GbShareToken{})
}
