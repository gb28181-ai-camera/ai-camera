package model

import "time"

// ════════════════════════════════════════════════════════════════
// 巡检中心：巡检任务 / 巡检记录
// ════════════════════════════════════════════════════════════════

// 巡检任务状态常量。
//
// 任务发布后 status=published，由被指派的巡检人在移动端处理后任务移除
// （status=finished）并生成一条 InspectRecord。
const (
	InspectStatusPending   = "pending"   // 待发布
	InspectStatusPublished = "published" // 已发布（待巡检）
	InspectStatusFinished  = "finished"  // 已完成（已转巡检记录）
	InspectStatusCanceled  = "canceled"  // 已取消
)

// InspectTask 巡检任务。表 ac_inspect_task。
type InspectTask struct {
	BaseModel
	Active      *bool     `gorm:"column:active;default:true" json:"active"`
	Name        string    `gorm:"column:name;size:255;not null;comment:任务名称" json:"name"`
	RegionID    *int64    `gorm:"column:region_id;index;comment:所属区域" json:"region_id"`
	Location    string    `gorm:"column:location;size:255;comment:巡检位置" json:"location"`
	InspectorID int64     `gorm:"column:inspector_id;not null;index;comment:巡检人用户ID" json:"inspector_id"`
	Content     string    `gorm:"column:content;type:text;comment:巡检内容" json:"content"`
	PlanDT      time.Time `gorm:"column:plan_dt;index;comment:巡检时间" json:"plan_dt"`
	Status      string    `gorm:"column:status;size:20;default:published;index;comment:pending/published/finished/canceled" json:"status"`
	Read        *bool     `gorm:"column:read;default:false;comment:消息通知是否已读" json:"read"`
	Remark      string    `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64  `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
	Region *Region `gorm:"foreignKey:RegionID;references:ID" json:"region,omitempty"`
}

func (InspectTask) TableName() string { return "ac_inspect_task" }

// InspectRecord 巡检记录。表 ac_inspect_record。
// 由巡检人在 App 端提交后生成，保留任务快照字段（任务名称/区域/位置/巡检人/内容）
// 以便任务被改动或删除后记录仍可独立查看。
type InspectRecord struct {
	BaseModel
	TaskID      int64      `gorm:"column:task_id;index;comment:来源巡检任务ID" json:"task_id"`
	TaskName    string     `gorm:"column:task_name;size:255;comment:任务名称(冗余)" json:"task_name"`
	RegionID    *int64     `gorm:"column:region_id;index;comment:所属区域" json:"region_id"`
	Location    string     `gorm:"column:location;size:255;comment:巡检位置" json:"location"`
	InspectorID int64      `gorm:"column:inspector_id;index;comment:巡检人用户ID" json:"inspector_id"`
	Inspector   string     `gorm:"column:inspector;size:50;comment:巡检人(冗余)" json:"inspector"`
	Content     string     `gorm:"column:content;type:text;comment:巡检内容" json:"content"`
	PlanDT      time.Time  `gorm:"column:plan_dt;comment:巡检时间" json:"plan_dt"`
	Result      string     `gorm:"column:result;type:text;comment:处理结果" json:"result"`
	ResultImage string     `gorm:"column:result_image;size:500;comment:处理结果图" json:"result_image"`
	HandledDT   *time.Time `gorm:"column:handled_dt;comment:处理时间" json:"handled_dt"`
	Status      string     `gorm:"column:status;size:20;default:finished;comment:状态" json:"status"`

	DeptID *int64  `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
	Region *Region `gorm:"foreignKey:RegionID;references:ID" json:"region,omitempty"`
}

func (InspectRecord) TableName() string { return "ac_inspect_record" }
