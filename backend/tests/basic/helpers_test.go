package basic

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// 测试基座约定的种子数据：admin（id=1）为超级管理员，所属部门 id=1。
const (
	testUserID   int64  = 1
	testUserName string = "admin"
	testDeptID   int64  = 1
)

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

func init() { syncSequences() }

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

// syncSequences 把自增序列对齐到各表当前最大 ID。
//
// 背景：种子数据以显式 ID 写入却不推进序列，会使后续 INSERT 命中已存在的 ID
// 而报唯一约束冲突。setval 非事务性，因此该修复对测试库永久生效。
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
	syncSequencesThrottled()
	return testutil.SetTestContext(testUserID, testUserName, testDeptID)
}

// ctxFor 构造指定用户/部门的上下文，用于数据权限与审计字段用例。
func ctxFor(userID int64, userName string, deptID int64) context.Context {
	return testutil.SetTestContext(userID, userName, deptID)
}

// dsAll 全量数据权限。
func dsAll() *utils.DataScope {
	syncSequencesThrottled()
	return &utils.DataScope{UserID: testUserID, UserName: testUserName, DataPerm: "all"}
}

// dsPerm 构造指定级别的数据权限。
func dsPerm(perm, userName string, deptID *int64, crossDeptIDs string) *utils.DataScope {
	syncSequencesThrottled()
	return &utils.DataScope{
		UserID:       testUserID,
		UserName:     userName,
		DataPerm:     perm,
		DeptID:       deptID,
		CrossDeptIDs: crossDeptIDs,
	}
}

func boolPtr(b bool) *bool { return &b }

func int64Ptr(v int64) *int64 { return &v }

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

// softDeletedAt 读取记录的 deleted_at（忽略软删除条件），用于断言软删除。
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
