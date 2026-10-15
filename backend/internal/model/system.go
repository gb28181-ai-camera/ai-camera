package model

import "time"

// ════════════════════════════════════════════════════════════════
// 系统管理：用户 / 角色 / 菜单 / 部门 / 岗位 / 权限关系
// ════════════════════════════════════════════════════════════════

// User 平台登录账号。表 ac_user。
type User struct {
	BaseModel
	Active       *bool      `gorm:"column:active;default:true;comment:是否启用" json:"active"`
	Name         string     `gorm:"column:name;size:255;not null;comment:姓名/显示名" json:"name"`
	Username     string     `gorm:"column:user_name;size:100;not null;uniqueIndex:idx_ac_user_username;comment:登录名" json:"user_name"`
	PasswordHash string     `gorm:"column:password_hash;size:255;not null;comment:密码哈希(bcrypt)" json:"-"`
	Email        string     `gorm:"column:email;size:255;comment:邮箱" json:"email"`
	Phone        string     `gorm:"column:phone;size:50;comment:手机号" json:"phone"`
	Avatar       string     `gorm:"column:avatar;type:text;comment:头像" json:"avatar"`
	DeptID       *int64     `gorm:"column:dept_id;index;comment:部门ID" json:"dept_id"`
	PostID       *int64     `gorm:"column:post_id;index;comment:岗位ID" json:"post_id"`
	IsAdmin      *bool      `gorm:"column:is_admin;default:false;comment:是否超级管理员" json:"is_admin"`
	LastLogin    *time.Time `gorm:"column:last_login;comment:最后登录时间" json:"last_login"`
	Remark       string     `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	Dept  *Dept  `gorm:"foreignKey:DeptID;references:ID" json:"dept,omitempty"`
	Post  *Post  `gorm:"foreignKey:PostID;references:ID" json:"post,omitempty"`
	Roles []Role `gorm:"many2many:ac_user_role_rel;foreignKey:ID;joinForeignKey:user_id;references:ID;joinReferences:role_id" json:"roles,omitempty"`
}

func (User) TableName() string { return "ac_user" }

// Role 角色。表 ac_role。Code 是 Casbin 的 sub 值，必须唯一且稳定。
type Role struct {
	BaseModel
	Active         *bool  `gorm:"column:active;default:true" json:"active"`
	Sequence       int    `gorm:"column:sequence;default:10;comment:显示顺序" json:"sequence"`
	Name           string `gorm:"column:name;size:255;not null;comment:角色名称" json:"name"`
	Code           string `gorm:"column:code;size:50;not null;uniqueIndex:idx_ac_role_code;comment:权限字符(Casbin sub)" json:"code"`
	Description    string `gorm:"column:description;type:text;comment:描述" json:"description"`
	DataPermission string `gorm:"column:data_permission;size:20;not null;default:all;comment:数据权限 all/cross_dept/dept/personal" json:"data_permission"`
	CrossDeptIDs   string `gorm:"column:cross_dept_ids;type:text;comment:跨部门数据权限-部门ID列表" json:"cross_dept_ids"`

	Menus []Menu `gorm:"many2many:ac_role_menu_rel;foreignKey:ID;joinForeignKey:role_id;references:ID;joinReferences:menu_id" json:"menus,omitempty"`
}

func (Role) TableName() string { return "ac_role" }

// Menu 菜单/按钮。表 ac_menu。
// Type 取值：directory（目录）/ menu（菜单）/ button（按钮）。
// 按钮型菜单的 PermissionCode 即前端用于控制显隐的权限标识。
type Menu struct {
	BaseModel
	Active         *bool  `gorm:"column:active;default:true" json:"active"`
	Sequence       int    `gorm:"column:sequence;default:10;comment:显示顺序" json:"sequence"`
	Name           string `gorm:"column:name;size:255;not null;comment:菜单名称" json:"name"`
	ParentID       *int64 `gorm:"column:parent_id;index;comment:父菜单ID" json:"parent_id"`
	Level          int    `gorm:"column:level;default:1;comment:层级" json:"level"`
	Path           string `gorm:"column:path;type:text;comment:路由路径" json:"path"`
	Icon           string `gorm:"column:icon;size:100;comment:图标" json:"icon"`
	Component      string `gorm:"column:component;size:255;comment:前端组件" json:"component"`
	Type           string `gorm:"column:type;size:20;not null;default:menu;comment:directory/menu/button" json:"type"`
	Platform       string `gorm:"column:platform;size:10;not null;default:pc;comment:pc/mobile" json:"platform"`
	PermissionCode string `gorm:"column:permission_code;size:100;comment:权限标识(按钮级)" json:"permission_code"`
	Description    string `gorm:"column:description;type:text;comment:描述" json:"description"`
	Hidden         *bool  `gorm:"column:hidden;default:false;comment:是否隐藏" json:"hidden"`

	Children []Menu    `gorm:"-" json:"children,omitempty"`
	APIs     []MenuAPI `gorm:"foreignKey:MenuID;references:ID" json:"apis,omitempty"`
}

func (Menu) TableName() string { return "ac_menu" }

// MenuAPI 菜单绑定的后端接口，是 Casbin p 策略的唯一来源。表 ac_menu_api。
type MenuAPI struct {
	BaseModel
	MenuID      int64  `gorm:"column:menu_id;not null;index;comment:菜单ID" json:"menu_id"`
	Method      string `gorm:"column:method;size:10;not null;comment:HTTP方法" json:"method"`
	Path        string `gorm:"column:path;size:255;not null;comment:接口路径" json:"path"`
	Description string `gorm:"column:description;size:500;comment:说明" json:"description"`
}

func (MenuAPI) TableName() string { return "ac_menu_api" }

// Dept 部门（组织）。表 ac_dept。Code 是 Casbin 的 g2 主体。
type Dept struct {
	BaseModel
	Active      *bool  `gorm:"column:active;default:true" json:"active"`
	Sequence    int    `gorm:"column:sequence;default:10;comment:排序" json:"sequence"`
	Name        string `gorm:"column:name;size:255;not null;comment:部门名称" json:"name"`
	Code        string `gorm:"column:code;size:50;comment:部门编码(Casbin g2 主体)" json:"code"`
	ParentID    *int64 `gorm:"column:parent_id;index;comment:父部门ID" json:"parent_id"`
	Level       int    `gorm:"column:level;default:1;comment:层级" json:"level"`
	Path        string `gorm:"column:path;type:text;comment:层级路径" json:"path"`
	LeaderID    *int64 `gorm:"column:leader_id;comment:负责人用户ID" json:"leader_id"`
	Description string `gorm:"column:description;type:text;comment:描述" json:"description"`

	Children []Dept `gorm:"-" json:"children,omitempty"`
}

func (Dept) TableName() string { return "ac_dept" }

// Post 岗位。表 ac_post。Code 是 Casbin 的 g3 主体。
type Post struct {
	BaseModel
	Active      *bool   `gorm:"column:active;default:true" json:"active"`
	Sequence    int     `gorm:"column:sequence;default:10;comment:排序" json:"sequence"`
	Name        string  `gorm:"column:name;size:255;not null;comment:岗位名称" json:"name"`
	Code        string  `gorm:"column:code;size:50;not null;uniqueIndex:idx_ac_post_code;comment:岗位编码(Casbin g3 主体)" json:"code"`
	Description string  `gorm:"column:description;type:text;comment:描述" json:"description"`
	RoleIDs     []int64 `gorm:"-" json:"role_ids,omitempty"`
}

func (Post) TableName() string { return "ac_post" }

// ────────────────────────────────────────────────────────────────
// 权限关系表（无 BaseModel，仅主键 + 唯一索引）
// ────────────────────────────────────────────────────────────────

// UserRoleRel 用户-角色关联。表 ac_user_role_rel。
type UserRoleRel struct {
	ID     int64 `gorm:"primarykey" json:"id"`
	UserID int64 `gorm:"column:user_id;not null;uniqueIndex:idx_ac_user_role" json:"user_id"`
	RoleID int64 `gorm:"column:role_id;not null;uniqueIndex:idx_ac_user_role" json:"role_id"`
}

func (UserRoleRel) TableName() string { return "ac_user_role_rel" }

// UserPostRel 用户-岗位关联。表 ac_user_post_rel。
type UserPostRel struct {
	ID        int64 `gorm:"primarykey" json:"id"`
	UserID    int64 `gorm:"column:user_id;not null;uniqueIndex:idx_ac_user_post" json:"user_id"`
	PostID    int64 `gorm:"column:post_id;not null;uniqueIndex:idx_ac_user_post" json:"post_id"`
	IsPrimary bool  `gorm:"column:is_primary;default:false;comment:是否主岗位" json:"is_primary"`
}

func (UserPostRel) TableName() string { return "ac_user_post_rel" }

// PostRoleRel 岗位-角色关联。表 ac_post_role_rel。
type PostRoleRel struct {
	ID     int64 `gorm:"primarykey" json:"id"`
	PostID int64 `gorm:"column:post_id;not null;uniqueIndex:idx_ac_post_role" json:"post_id"`
	RoleID int64 `gorm:"column:role_id;not null;uniqueIndex:idx_ac_post_role" json:"role_id"`
}

func (PostRoleRel) TableName() string { return "ac_post_role_rel" }

// DeptRoleRel 部门-角色关联。表 ac_dept_role_rel。
type DeptRoleRel struct {
	ID     int64 `gorm:"primarykey" json:"id"`
	DeptID int64 `gorm:"column:dept_id;not null;uniqueIndex:idx_ac_dept_role" json:"dept_id"`
	RoleID int64 `gorm:"column:role_id;not null;uniqueIndex:idx_ac_dept_role" json:"role_id"`
}

func (DeptRoleRel) TableName() string { return "ac_dept_role_rel" }

// RoleMenuRel 角色-菜单关联。表 ac_role_menu_rel。
type RoleMenuRel struct {
	ID     int64 `gorm:"primarykey" json:"id"`
	RoleID int64 `gorm:"column:role_id;not null;uniqueIndex:idx_ac_role_menu" json:"role_id"`
	MenuID int64 `gorm:"column:menu_id;not null;uniqueIndex:idx_ac_role_menu" json:"menu_id"`
}

func (RoleMenuRel) TableName() string { return "ac_role_menu_rel" }

// ────────────────────────────────────────────────────────────────
// 系统支撑
// ────────────────────────────────────────────────────────────────

// SystemConfig 系统参数。表 ac_system_config。
type SystemConfig struct {
	BaseModel
	Active      *bool  `gorm:"column:active;default:true" json:"active"`
	Name        string `gorm:"column:name;size:255;not null;comment:参数名称" json:"name"`
	Code        string `gorm:"column:code;size:100;not null;comment:参数键" json:"key"`
	Value       string `gorm:"column:value;type:text;comment:参数值" json:"value"`
	GroupName   string `gorm:"column:group_name;size:50;comment:分组" json:"group_name"`
	DataType    string `gorm:"column:data_type;size:20;default:string;comment:string/int/bool/json" json:"data_type"`
	Sequence    int    `gorm:"column:sequence;default:10" json:"sequence"`
	Description string `gorm:"column:description;type:text;comment:说明" json:"description"`
}

func (SystemConfig) TableName() string { return "ac_system_config" }

// ApiKey 开放接口密钥，用于设备回调等无用户上下文的场景。表 ac_api_key。
type ApiKey struct {
	BaseModel
	Active     *bool      `gorm:"column:active;default:true" json:"active"`
	UserID     int64      `gorm:"column:user_id;not null;index" json:"user_id"`
	Name       string     `gorm:"column:name;size:100;comment:名称" json:"name"`
	KeyPrefix  string     `gorm:"column:key_prefix;size:20;comment:密钥前缀(用于展示)" json:"key_prefix"`
	KeyHash    string     `gorm:"column:key_hash;size:255;comment:密钥SHA256" json:"-"`
	LastUsedDT *time.Time `gorm:"column:last_used_dt;comment:最后使用时间" json:"last_used_dt"`
}

func (ApiKey) TableName() string { return "ac_api_key" }

// OperationLog 操作日志。表 ac_operation_log。
type OperationLog struct {
	BaseModel
	Module   string `gorm:"column:module;size:100;comment:系统模块" json:"module"`
	Action   string `gorm:"column:action;size:20;comment:操作类型" json:"action"`
	Method   string `gorm:"column:method;size:10;comment:请求方式" json:"method"`
	Path     string `gorm:"column:path;size:255;comment:请求路径" json:"path"`
	Operator string `gorm:"column:operator;size:100;comment:操作人员" json:"operator"`
	IP       string `gorm:"column:ip;size:64;comment:主机" json:"ip"`
	Location string `gorm:"column:location;size:255;comment:操作地点" json:"location"`
	Status   int    `gorm:"column:status;comment:状态 1成功 0失败" json:"status"`
	ErrorMsg string `gorm:"column:error_msg;type:text;comment:错误信息" json:"error_msg"`
	Params   string `gorm:"column:params;type:text;comment:请求参数" json:"params"`
	CostMs   int64  `gorm:"column:cost_ms;comment:耗时(毫秒)" json:"cost_ms"`
}

func (OperationLog) TableName() string { return "ac_operation_log" }

// Notification 系统通知（设备告警/风险通知/巡检通知）。表 ac_notification。
type Notification struct {
	BaseModel
	UserID   int64      `gorm:"column:user_id;not null;index;comment:接收人" json:"user_id"`
	Category string     `gorm:"column:category;size:20;not null;comment:alarm/risk/inspect" json:"category"`
	Title    string     `gorm:"column:title;size:255;comment:标题" json:"title"`
	Content  string     `gorm:"column:content;type:text;comment:内容" json:"content"`
	RefType  string     `gorm:"column:ref_type;size:50;comment:关联业务类型" json:"ref_type"`
	RefID    int64      `gorm:"column:ref_id;comment:关联业务ID" json:"ref_id"`
	Read     *bool      `gorm:"column:read;default:false;comment:是否已读" json:"read"`
	ReadDT   *time.Time `gorm:"column:read_dt;comment:已读时间" json:"read_dt"`
}

func (Notification) TableName() string { return "ac_notification" }

// Sequence 序列号生成器。表 ac_sequence。
type Sequence struct {
	BaseModel
	Code        string `gorm:"column:code;size:50;not null;uniqueIndex:idx_ac_sequence_code;comment:序列编码" json:"code"`
	Name        string `gorm:"column:name;size:255;comment:序列名称" json:"name"`
	Prefix      string `gorm:"column:prefix;size:20;comment:前缀" json:"prefix"`
	CurrentVal  int64  `gorm:"column:current_val;default:0;comment:当前值" json:"current_val"`
	Step        int    `gorm:"column:step;default:1;comment:步长" json:"step"`
	DatePattern string `gorm:"column:date_pattern;size:20;comment:日期格式(如 20060102)" json:"date_pattern"`
	ResetByDay  *bool  `gorm:"column:reset_by_day;default:true;comment:是否按日重置" json:"reset_by_day"`
}

func (Sequence) TableName() string { return "ac_sequence" }

// DataScopeOption 数据权限可选项，供角色页「数据权限」使用。
// 与 Role.DataPermission 取值一一对应。
var DataScopeOption = []string{"all", "cross_dept", "dept", "personal"}
