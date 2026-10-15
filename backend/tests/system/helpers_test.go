package system

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/config"
	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	svc "github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/pkg/auth"
	"github.com/boticz/ai-camera-backend/pkg/perm"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// rbacModelPath Casbin 模型文件相对测试包目录的路径。
const rbacModelPath = "../../config/rbac_model.conf"

// 种子数据：admin（id=1）为超级管理员，所属部门 id=1、角色 super_admin。
const (
	testUserID   int64  = 1
	testUserName string = "admin"
	testDeptID   int64  = 1
)

// 测试登录密码（与种子 admin 保持一致，便于复用 Login 用例）。
const testPassword = "admin123"

// init 注入测试用 JWT 服务。go test 的工作目录不含 config/，bootstrap.init()
// 会跳过初始化，因此必须在此显式装配，否则 AuthService.Login 无法签发 token。
func init() {
	bootstrap.SetJWT(auth.NewJWTService(&config.JWTConfig{Secret: "ai-camera-unit-test-secret", ExpireHours: 2}))
	syncSequences()
}

// seqTables 需要对齐自增序列的表（种子数据以显式 ID 插入，未推进序列）。
var seqTables = []string{
	"ac_menu", "ac_menu_api", "ac_role", "ac_user", "ac_post", "ac_dept", "ac_region",
	"ac_device", "ac_device_mode", "ac_device_history", "ac_iot_card", "ac_iot_card_replace_log",
	"ac_voice_config", "ac_firmware_type", "ac_firmware", "ac_ai_model",
	"ac_risk_type", "ac_alarm_type", "ac_risk_record", "ac_alarm_record",
	"ac_inspect_task", "ac_inspect_record",
	"ac_device_upgrade_plan", "ac_device_upgrade_record",
	"ac_alarm_setting", "ac_risk_notify_setting", "ac_thirdparty_config",
	"ac_system_config", "ac_notification", "ac_operation_log", "ac_api_key",
}

// syncSequences 把自增序列对齐到各表当前最大 ID。
//
// 背景：种子数据以显式 ID 写入（如 ac_menu_api.id=209）却不推进序列，
// 会使后续 INSERT 命中已存在的 ID 而报唯一约束冲突。setval 非事务性，
// 因此该修复对测试库（ai_camera_test）永久生效，且不影响业务代码。
func syncSequences() {
	db := testutil.InitTestDB()
	for _, tbl := range seqTables {
		sql := fmt.Sprintf(
			"SELECT setval(pg_get_serial_sequence('%s','id'), GREATEST("+
				"COALESCE(pg_sequence_last_value(pg_get_serial_sequence('%s','id')), 1), "+
				"COALESCE((SELECT MAX(id) FROM %s), 1)))",
			tbl, tbl, tbl,
		)
		if err := db.Exec(sql).Error; err != nil {
			log.Printf("测试序列对齐失败 %s: %v", tbl, err)
		}
	}
}

// testCtx 返回带 admin 登录上下文的 context（驱动审计字段回填）。
func testCtx() context.Context {
	return testutil.SetTestContext(testUserID, testUserName, testDeptID)
}

// ctxFor 构造指定用户/部门的上下文。
func ctxFor(userID int64, userName string, deptID int64) context.Context {
	return testutil.SetTestContext(userID, userName, deptID)
}

// dsAll 全量数据权限。
func dsAll() *utils.DataScope {
	return &utils.DataScope{UserID: testUserID, UserName: testUserName, DataPerm: "all"}
}

// dsPerm 构造指定级别的数据权限。
func dsPerm(permCode, userName string, deptID *int64, crossDeptIDs string) *utils.DataScope {
	return &utils.DataScope{
		UserID:       testUserID,
		UserName:     userName,
		DataPerm:     permCode,
		DeptID:       deptID,
		CrossDeptIDs: crossDeptIDs,
	}
}

func boolPtr(b bool) *bool { return &b }

func int64Ptr(v int64) *int64 { return &v }

// 种子数据以显式 ID 写入却不推进序列（如 ac_menu_api.id=209），
// 会导致后续 INSERT 命中已存在的 ID；这里在首个用例构造前对齐一次序列。
var (
	syncMu   sync.Mutex
	lastSync time.Time
)

// syncSequencesThrottled 按秒节流地把自增序列单调前推到 max(id)。
//
// 只前进、绝不回拨：回拨会让并行的测试包互相抢号（主键冲突甚至死锁）；
// 单调前推则能自愈另一进程 resetdb 造成的序列回退。
func syncSequencesThrottled() {
	syncMu.Lock()
	defer syncMu.Unlock()
	if time.Since(lastSync) < 100*time.Millisecond {
		return
	}
	syncSequences()
	lastSync = time.Now()
}

// uniq 生成唯一后缀，避免与上次运行残留数据或其它用例冲突（唯一索引列）。
var uniqCount int64

// uniq 生成进程内唯一后缀：Windows 时钟粒度可能使连续两次 UnixNano 取值相同，
// 叠加自增计数可确保唯一索引列（user_name / code / card_no 等）不会冲突。
func uniq(prefix string) string {
	syncSequencesThrottled()
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), atomic.AddInt64(&uniqCount, 1))
}

// withCasbin 在当前事务上装配 Casbin，并在用例结束后还原为 nil。
// 策略写入随事务一起回滚，因此不会污染 casbin_rule。
func withCasbin(t *testing.T, tx *gorm.DB) *perm.CasbinService {
	t.Helper()
	cs, err := perm.NewCasbinService(tx, rbacModelPath)
	if err != nil {
		t.Fatalf("初始化 Casbin 失败: %v", err)
	}
	bootstrap.SetCasbin(cs)
	t.Cleanup(func() { bootstrap.SetCasbin(nil) })
	return cs
}

// pickID 取某表按 id 升序的第一条 ID，避免对种子数据顺序敏感。
func pickID(t *testing.T, tx *gorm.DB, table, where string, args ...interface{}) int64 {
	t.Helper()
	q := tx.Table(table)
	if where != "" {
		q = q.Where(where, args...)
	}
	var id int64
	if err := q.Order("id ASC").Limit(1).Pluck("id", &id).Error; err != nil {
		t.Fatalf("查询 %s 主键失败: %v", table, err)
	}
	if id == 0 {
		t.Fatalf("表 %s 缺少可用数据（where=%s）", table, where)
	}
	return id
}

// countRows 统计满足条件的记录数。
func countRows(t *testing.T, tx *gorm.DB, table, where string, args ...interface{}) int64 {
	t.Helper()
	q := tx.Table(table)
	if where != "" {
		q = q.Where(where, args...)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		t.Fatalf("统计 %s 失败: %v", table, err)
	}
	return n
}

// softDeletedAt 读取记录的 deleted_at（忽略软删除条件）。
func softDeletedAt(t *testing.T, tx *gorm.DB, table string, id int64) *time.Time {
	t.Helper()
	var nt sql.NullTime
	row := tx.Raw("SELECT deleted_at FROM "+table+" WHERE id = ?", id).Row()
	if err := row.Scan(&nt); err != nil {
		t.Fatalf("读取 %s.deleted_at 失败: %v", table, err)
	}
	if nt.Valid {
		v := nt.Time
		return &v
	}
	return nil
}

// activeFlag 读取某表记录的 active 值。
func activeFlag(t *testing.T, tx *gorm.DB, table string, id int64) bool {
	t.Helper()
	var nb sql.NullBool
	row := tx.Raw("SELECT active FROM "+table+" WHERE id = ?", id).Row()
	if err := row.Scan(&nb); err != nil {
		t.Fatalf("读取 %s.active 失败: %v", table, err)
	}
	return nb.Valid && nb.Bool
}

// sqlText 读取某表的字符串列（NULL 映射为空串）。
func sqlText(t *testing.T, tx *gorm.DB, table, column string, id int64) string {
	t.Helper()
	var ns sql.NullString
	row := tx.Raw("SELECT "+column+" FROM "+table+" WHERE id = ?", id).Row()
	if err := row.Scan(&ns); err != nil {
		t.Fatalf("读取 %s.%s 失败: %v", table, column, err)
	}
	return ns.String
}

// casbinPolicyCount 统计 casbin_rule 中指定类型的策略条数。
func casbinPolicyCount(t *testing.T, tx *gorm.DB, pType, v0, v1 string) int64 {
	t.Helper()
	q := tx.Table("casbin_rule").Where("p_type = ?", pType)
	if v0 != "" {
		q = q.Where("v0 = ?", v0)
	}
	if v1 != "" {
		q = q.Where("v1 = ?", v1)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		t.Fatalf("统计 casbin_rule 失败: %v", err)
	}
	return n
}

// minute 时间比较容差。
const minute = time.Minute

// timeNow 当前时间（便于在用例中固定调用点）。
func timeNow() time.Time { return time.Now() }

// head 返回切片前 n 项（用于错误信息截断展示）。
func head(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

// walkMenus 深度优先遍历菜单树，对每个节点执行 fn。
func walkMenus(nodes []model.Menu, fn func(model.Menu)) {
	for _, n := range nodes {
		fn(n)
		walkMenus(n.Children, fn)
	}
}

// mustCreateUser 通过 UserService 创建用户（密码加密、审计字段回填均走真实路径）。
func mustCreateUser(t *testing.T, ctx context.Context, username string, req *request.CreateUserReq) *model.User {
	t.Helper()
	if req == nil {
		req = &request.CreateUserReq{}
	}
	req.Username = username
	if req.Name == "" {
		req.Name = "ZZQ-" + username
	}
	if req.Password == "" {
		req.Password = testPassword
	}
	u, err := svc.NewUserService().Create(ctx, req)
	if err != nil {
		t.Fatalf("创建用户 %s 失败: %v", username, err)
	}
	return u
}

// mustCreateRole 通过 RoleService 创建角色。
func mustCreateRole(t *testing.T, ctx context.Context, code string, dataPerm string) *model.Role {
	t.Helper()
	r, err := svc.NewRoleService().Create(ctx, &request.CreateRoleReq{
		Name:           "ZZQ-" + code,
		Code:           code,
		DataPermission: dataPerm,
		Active:         boolPtr(true),
	})
	if err != nil {
		t.Fatalf("创建角色 %s 失败: %v", code, err)
	}
	return r
}
