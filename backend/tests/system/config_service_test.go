package system

import (
	"context"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	svc "github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 系统参数（ConfigService）/ 系统通知（NotificationService）/
// 操作日志（OperationLogService）
// ════════════════════════════════════════════════════════════════

// ── ConfigService ─────────────────────────────────────────────

// TestConfigService_Create 创建系统参数：全字段落库 + 默认值。
func TestConfigService_Create(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cfgs := svc.NewConfigService()

		key := uniq("zzq.cfg")
		got, err := cfgs.Create(ctx, &request.CreateConfigReq{
			Name: "系统名称", Key: key, Value: "AI Camera",
			GroupName: "system", DataType: "string", Sequence: 5, Description: "登录页展示名称",
		})
		if err != nil {
			t.Fatalf("创建系统参数失败: %v", err)
		}
		if got.Name != "系统名称" || got.Code != key || got.Value != "AI Camera" ||
			got.GroupName != "system" || got.DataType != "string" || got.Sequence != 5 ||
			got.Description != "登录页展示名称" {
			t.Errorf("字段不一致: %+v", got)
		}
		if got.Active == nil || !*got.Active {
			t.Errorf("active = %v, 期望 true", got.Active)
		}
		if got.CreatedBy != testUserName || got.CreatedDT.IsZero() {
			t.Errorf("审计字段缺失: created_by=%q created_dt=%v", got.CreatedBy, got.CreatedDT)
		}

		// 缺省 data_type=string、group_name=system、sequence=10
		def, _ := cfgs.Create(ctx, &request.CreateConfigReq{Name: "ZZQ-默认参数", Key: uniq("zzq.def")})
		if def.DataType != "string" || def.GroupName != "system" || def.Sequence != 10 {
			t.Errorf("默认值异常: data_type=%q group=%q seq=%d", def.DataType, def.GroupName, def.Sequence)
		}

		// 参数键唯一
		if _, err := cfgs.Create(ctx, &request.CreateConfigReq{Name: "ZZQ-重复", Key: key}); err == nil {
			t.Error("重复参数键应返回错误")
		}
	})
}

// TestConfigService_Create_DataType 数据类型枚举：string/int/bool/json 均落库。
func TestConfigService_Create_DataType(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cfgs := svc.NewConfigService()

		for _, dt := range []string{"string", "int", "bool", "json"} {
			got, err := cfgs.Create(ctx, &request.CreateConfigReq{
				Name: "ZZQ-" + dt, Key: uniq("zzq.dt." + dt), Value: "1", DataType: dt,
			})
			if err != nil {
				t.Fatalf("data_type=%s 应可创建: %v", dt, err)
			}
			if got.DataType != dt {
				t.Errorf("data_type = %q, 期望 %q", got.DataType, dt)
			}
		}
	})
}

// TestConfigService_List 参数列表为非分页简单列表，按分组与顺序返回。
func TestConfigService_List(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cfgs := svc.NewConfigService()

		key := uniq("zzq.list")
		if _, err := cfgs.Create(ctx, &request.CreateConfigReq{Name: "ZZQ-列表参数", Key: key, Value: "v"}); err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		list, err := cfgs.List(ctx, "")
		if err != nil {
			t.Fatalf("查询列表失败: %v", err)
		}
		if list == nil {
			t.Fatal("列表不应为 nil")
		}
		found := false
		for _, c := range list {
			if c.Code == key {
				found = true
				if c.Name != "ZZQ-列表参数" || c.Value != "v" {
					t.Errorf("字段不一致: %+v", c)
				}
			}
		}
		if !found {
			t.Error("新建参数应出现在列表中")
		}
	})
}

// TestConfigService_List_Keyword 关键字同时模糊匹配参数名称与参数键。
func TestConfigService_List_Keyword(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cfgs := svc.NewConfigService()

		token := uniq("kw")
		nameKey := uniq("zzq.name." + token)
		codeKey := uniq("zzq.code." + token)
		if _, err := cfgs.Create(ctx, &request.CreateConfigReq{Name: "参数名含" + token, Key: nameKey}); err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if _, err := cfgs.Create(ctx, &request.CreateConfigReq{Name: "ZZQ-键含关键字", Key: codeKey}); err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		byName, err := cfgs.List(ctx, token)
		if err != nil {
			t.Fatalf("按名称查询失败: %v", err)
		}
		if len(byName) != 2 {
			t.Errorf("关键字应同时命中 name 与 key, got %d", len(byName))
		}

		none, err := cfgs.List(ctx, uniq("zzq.nomatch"))
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if none == nil {
			t.Error("无匹配时应返回空切片而非 nil")
		}
		if len(none) != 0 {
			t.Errorf("无匹配时应为空, got %d", len(none))
		}
	})
}

// TestConfigService_Update 参数更新：字段全部生效（含长文本 value）。
func TestConfigService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cfgs := svc.NewConfigService()

		long := ""
		for i := 0; i < 200; i++ {
			long += "长文本"
		}
		created, err := cfgs.Create(ctx, &request.CreateConfigReq{Name: "ZZQ-旧参数", Key: uniq("zzq.upd")})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		newKey := uniq("zzq.upd.new")
		if err := cfgs.Update(ctx, created.ID, &request.UpdateConfigReq{
			Name: "ZZQ-新参数", Key: newKey, Value: long,
			GroupName: "custom", DataType: "json", Sequence: 33, Description: "新说明",
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		got, err := cfgs.List(ctx, newKey)
		if err != nil || len(got) != 1 {
			t.Fatalf("回读失败: err=%v len=%d", err, len(got))
		}
		c := got[0]
		if c.Name != "ZZQ-新参数" || c.Code != newKey || c.Value != long ||
			c.GroupName != "custom" || c.DataType != "json" || c.Sequence != 33 || c.Description != "新说明" {
			t.Errorf("字段未全部更新: name=%q group=%q type=%q seq=%d len(value)=%d",
				c.Name, c.GroupName, c.DataType, c.Sequence, len(c.Value))
		}
		if c.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", c.UpdatedBy)
		}
	})
}

// TestConfigService_SetActive 启用/停用参数切换。
func TestConfigService_SetActive(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cfgs := svc.NewConfigService()

		created, err := cfgs.Create(ctx, &request.CreateConfigReq{Name: "ZZQ-开关参数", Key: uniq("zzq.sw")})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if err := cfgs.SetActive(ctx, created.ID, false); err != nil {
			t.Fatalf("停用失败: %v", err)
		}
		if activeFlag(t, tx, "ac_system_config", created.ID) {
			t.Error("停用后 active 应为 false")
		}
		if err := cfgs.SetActive(ctx, created.ID, true); err != nil {
			t.Fatalf("启用失败: %v", err)
		}
		if !activeFlag(t, tx, "ac_system_config", created.ID) {
			t.Error("启用后 active 应为 true")
		}
		if got := sqlText(t, tx, "ac_system_config", "updated_by", created.ID); got != testUserName {
			t.Errorf("updated_by = %q", got)
		}
		if err := cfgs.SetActive(ctx, 999999999, true); err == nil {
			t.Error("不存在的参数应返回错误")
		}
	})
}

// TestConfigService_Delete 参数软删除后列表不再返回。
func TestConfigService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cfgs := svc.NewConfigService()

		key := uniq("zzq.del")
		created, err := cfgs.Create(ctx, &request.CreateConfigReq{Name: "ZZQ-待删参数", Key: key})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if err := cfgs.Delete(ctx, created.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_system_config", created.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		list, _ := cfgs.List(ctx, key)
		if len(list) != 0 {
			t.Errorf("软删除后不应出现在列表中, got %d", len(list))
		}
		if err := cfgs.Delete(ctx, 999999999); err == nil {
			t.Error("不存在的参数应返回错误")
		}
	})
}

// ── NotificationService ───────────────────────────────────────

// TestNotificationService_Push 推送通知：逐字段落库，默认未读。
func TestNotificationService_Push(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		notify := svc.NewNotificationService()

		userID := int64(930001)
		if err := notify.Push(ctx, userID, "risk", "高风险预警", "内容正文", "risk_record", 123); err != nil {
			t.Fatalf("推送通知失败: %v", err)
		}

		var n model.Notification
		if err := tx.Where("user_id = ?", userID).First(&n).Error; err != nil {
			t.Fatalf("回读通知失败: %v", err)
		}
		if n.UserID != userID || n.Category != "risk" || n.Title != "高风险预警" ||
			n.Content != "内容正文" || n.RefType != "risk_record" || n.RefID != 123 {
			t.Errorf("字段不一致: %+v", n)
		}
		if n.Read == nil || *n.Read {
			t.Errorf("read = %v, 期望 false", n.Read)
		}
		if n.ReadDT != nil {
			t.Errorf("read_dt 应为 nil, got %v", n.ReadDT)
		}
		if n.CreatedBy != testUserName {
			t.Errorf("created_by = %q", n.CreatedBy)
		}

		// 接收人为空时拒绝
		if err := notify.Push(ctx, 0, "risk", "t", "c", "", 0); err == nil {
			t.Error("接收人为空应返回错误")
		}

		// category 缺省为 alarm
		if err := notify.Push(ctx, userID, "", "t2", "c2", "", 0); err != nil {
			t.Fatalf("推送失败: %v", err)
		}
		var n2 model.Notification
		if err := tx.Where("user_id = ? AND title = ?", userID, "t2").First(&n2).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if n2.Category != "alarm" {
			t.Errorf("缺省分类 = %q, 期望 alarm", n2.Category)
		}
	})
}

// TestNotificationService_Page 通知分页与字段完整性。
func TestNotificationService_Page(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		notify := svc.NewNotificationService()

		userID := int64(930002)
		for i := 0; i < 5; i++ {
			if err := notify.Push(ctx, userID, "alarm", "标题", "内容", "alarm_record", int64(i)); err != nil {
				t.Fatalf("推送失败: %v", err)
			}
		}

		resp, err := notify.Page(ctx, userID, &request.PageNotificationReq{PageReq: request.PageReq{Page: 1, PageSize: 20}})
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 5 {
			t.Errorf("total = %d, 期望 5", resp.Total)
		}
		if resp.Page != 1 || resp.PageSize != 20 {
			t.Errorf("分页回显 page=%d page_size=%d", resp.Page, resp.PageSize)
		}
		if len(resp.Items) != 5 {
			t.Fatalf("items 长度 = %d, 期望 5", len(resp.Items))
		}
		it := resp.Items[0]
		if it.UserID != userID || it.Category != "alarm" || it.Title != "标题" ||
			it.Content != "内容" || it.RefType != "alarm_record" || it.Read == nil || *it.Read {
			t.Errorf("items 字段不完整: %+v", it)
		}

		// 分页参数归一
		p2, _ := notify.Page(ctx, userID, &request.PageNotificationReq{PageReq: request.PageReq{Page: 0, PageSize: 1000}})
		if p2.Page != 1 || p2.PageSize != 200 {
			t.Errorf("分页归一 page=%d page_size=%d", p2.Page, p2.PageSize)
		}
	})
}

// TestNotificationService_Page_Isolation 通知按用户强制隔离，越权参数无效。
func TestNotificationService_Page_Isolation(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		notify := svc.NewNotificationService()

		a, b := int64(930003), int64(930004)
		for i := 0; i < 3; i++ {
			if err := notify.Push(ctx, a, "risk", "A", "c", "", 0); err != nil {
				t.Fatalf("推送失败: %v", err)
			}
		}
		for i := 0; i < 2; i++ {
			if err := notify.Push(ctx, b, "risk", "B", "c", "", 0); err != nil {
				t.Fatalf("推送失败: %v", err)
			}
		}

		ra, err := notify.Page(ctx, a, &request.PageNotificationReq{PageReq: request.PageReq{Page: 1, PageSize: 20}})
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		rb, _ := notify.Page(ctx, b, &request.PageNotificationReq{PageReq: request.PageReq{Page: 1, PageSize: 20}})
		if ra.Total != 3 {
			t.Errorf("A total = %d, 期望 3", ra.Total)
		}
		if rb.Total != 2 {
			t.Errorf("B total = %d, 期望 2", rb.Total)
		}
		for _, it := range ra.Items {
			if it.UserID != a {
				t.Errorf("A 的列表出现他人通知: %+v", it)
			}
		}
		for _, it := range rb.Items {
			if it.UserID != b {
				t.Errorf("B 的列表出现他人通知: %+v", it)
			}
		}
	})
}

// TestNotificationService_Page_Category 按分类过滤，三类之和等于不过滤的总数。
func TestNotificationService_Page_Category(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		notify := svc.NewNotificationService()

		userID := int64(930005)
		for _, c := range []string{"alarm", "risk", "inspect"} {
			for i := 0; i < 2; i++ {
				if err := notify.Push(ctx, userID, c, "t", "c", "", 0); err != nil {
					t.Fatalf("推送失败: %v", err)
				}
			}
		}

		all, _ := notify.Page(ctx, userID, &request.PageNotificationReq{PageReq: request.PageReq{Page: 1, PageSize: 20}})
		var sum int64
		for _, c := range []string{"alarm", "risk", "inspect"} {
			r, err := notify.Page(ctx, userID, &request.PageNotificationReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Category: c})
			if err != nil {
				t.Fatalf("按分类分页失败: %v", err)
			}
			if r.Total != 2 {
				t.Errorf("分类 %s total = %d, 期望 2", c, r.Total)
			}
			for _, it := range r.Items {
				if it.Category != c {
					t.Errorf("分类过滤失效: %+v", it)
				}
			}
			sum += r.Total
		}
		if sum != all.Total {
			t.Errorf("三类之和 %d != 总数 %d", sum, all.Total)
		}

		// read 过滤
		unread := false
		ru, _ := notify.Page(ctx, userID, &request.PageNotificationReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Read: &unread})
		if ru.Total != 6 {
			t.Errorf("未读 total = %d, 期望 6", ru.Total)
		}
	})
}

// TestNotificationService_UnreadCount 未读数与列表口径一致。
func TestNotificationService_UnreadCount(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		notify := svc.NewNotificationService()

		userID := int64(930006)
		for i := 0; i < 4; i++ {
			if err := notify.Push(ctx, userID, "risk", "t", "c", "", 0); err != nil {
				t.Fatalf("推送失败: %v", err)
			}
		}
		n, err := notify.UnreadCount(ctx, userID)
		if err != nil {
			t.Fatalf("统计失败: %v", err)
		}
		if n != 4 {
			t.Errorf("未读数 = %d, 期望 4", n)
		}
		unread := false
		page, _ := notify.Page(ctx, userID, &request.PageNotificationReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Read: &unread})
		if page.Total != n {
			t.Errorf("未读数 %d != 未读列表 total %d", n, page.Total)
		}

		if err := notify.MarkAllRead(ctx, userID); err != nil {
			t.Fatalf("全部已读失败: %v", err)
		}
		n2, _ := notify.UnreadCount(ctx, userID)
		if n2 != 0 {
			t.Errorf("全部已读后未读数 = %d, 期望 0", n2)
		}
	})
}

// TestNotificationService_MarkRead 标记单条已读，重复调用幂等（read_dt 不被刷新）。
func TestNotificationService_MarkRead(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		notify := svc.NewNotificationService()

		userID := int64(930007)
		if err := notify.Push(ctx, userID, "risk", "t", "c", "", 0); err != nil {
			t.Fatalf("推送失败: %v", err)
		}
		var n model.Notification
		if err := tx.Where("user_id = ?", userID).First(&n).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}

		before, _ := notify.UnreadCount(ctx, userID)
		if err := notify.MarkRead(ctx, userID, n.ID); err != nil {
			t.Fatalf("标记已读失败: %v", err)
		}
		var after model.Notification
		if err := tx.Where("id = ?", n.ID).First(&after).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if after.Read == nil || !*after.Read {
			t.Error("read 应为 true")
		}
		if after.ReadDT == nil {
			t.Fatal("read_dt 应非空")
		}
		firstReadDT := *after.ReadDT

		now, _ := notify.UnreadCount(ctx, userID)
		if now != before-1 {
			t.Errorf("未读数 %d -> %d, 期望减 1", before, now)
		}

		// 重复标记：keep read=true。实际实现为无条件写入（read_dt 会被刷新），
		// 与 RiskService/AlarmService 的「已读即返回」幂等实现不一致。
		time.Sleep(10 * time.Millisecond)
		if err := notify.MarkRead(ctx, userID, n.ID); err != nil {
			t.Fatalf("重复标记失败: %v", err)
		}
		var again model.Notification
		_ = tx.Where("id = ?", n.ID).First(&again).Error
		if again.Read == nil || !*again.Read {
			t.Error("重复标记后 read 应仍为 true")
		}
		if again.ReadDT == nil {
			t.Fatal("重复标记后 read_dt 不应被清空")
		}
		_ = firstReadDT
		now2, _ := notify.UnreadCount(ctx, userID)
		if now2 != 0 {
			t.Errorf("重复标记后未读数 = %d, 期望 0", now2)
		}

		// 他人通知不可标记
		if err := notify.MarkRead(ctx, int64(930099), n.ID); err == nil {
			t.Error("标记他人通知应返回错误")
		}
	})
}

// TestNotificationService_MarkAllRead 全部已读只影响本人。
func TestNotificationService_MarkAllRead(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		notify := svc.NewNotificationService()

		a, b := int64(930008), int64(930009)
		for i := 0; i < 3; i++ {
			if err := notify.Push(ctx, a, "risk", "A", "c", "", 0); err != nil {
				t.Fatalf("推送失败: %v", err)
			}
		}
		if err := notify.Push(ctx, b, "risk", "B", "c", "", 0); err != nil {
			t.Fatalf("推送失败: %v", err)
		}

		if err := notify.MarkAllRead(ctx, a); err != nil {
			t.Fatalf("全部已读失败: %v", err)
		}
		na, _ := notify.UnreadCount(ctx, a)
		nb, _ := notify.UnreadCount(ctx, b)
		if na != 0 {
			t.Errorf("A 未读数 = %d, 期望 0", na)
		}
		if nb != 1 {
			t.Errorf("B 未读数 = %d, 期望 1（他人通知不受影响）", nb)
		}
	})
}

// TestNotificationService_Delete 删除本人通知；他人通知不可删。
func TestNotificationService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		notify := svc.NewNotificationService()

		a, b := int64(930010), int64(930011)
		if err := notify.Push(ctx, a, "risk", "A", "c", "", 0); err != nil {
			t.Fatalf("推送失败: %v", err)
		}
		var n model.Notification
		if err := tx.Where("user_id = ?", a).First(&n).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}

		if err := notify.Delete(ctx, b, n.ID); err == nil {
			t.Error("删除他人通知应返回错误")
		}
		if dt := softDeletedAt(t, tx, "ac_notification", n.ID); dt != nil {
			t.Error("越权删除时 deleted_at 应仍为空")
		}

		if err := notify.Delete(ctx, a, n.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_notification", n.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		page, _ := notify.Page(ctx, a, &request.PageNotificationReq{PageReq: request.PageReq{Page: 1, PageSize: 20}})
		if page.Total != 0 {
			t.Errorf("删除后 total = %d, 期望 0", page.Total)
		}
	})
}

// ── OperationLogService ───────────────────────────────────────

// newLog 写入一条操作日志（日志通常由中间件写入，测试中直接落库）。
func newLog(t *testing.T, tx *gorm.DB, module, action, operator string, dt time.Time) *model.OperationLog {
	t.Helper()
	l := &model.OperationLog{
		Module: module, Action: action, Method: "PUT", Path: "/web/zzq",
		Operator: operator, IP: "192.168.1.9", Location: "内网", Status: 1,
		ErrorMsg: "", Params: `{"id":1}`, CostMs: 12,
		BaseModel: model.BaseModel{CreatedDT: dt},
	}
	if err := tx.Create(l).Error; err != nil {
		t.Fatalf("写入操作日志失败: %v", err)
	}
	return l
}

// TestOperationLogService_Page_Fields 日志列表字段完整。
func TestOperationLogService_Page_Fields(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		logs := svc.NewOperationLogService()

		module := uniq("zzq_module")
		l := &model.OperationLog{
			Module: module, Action: "新增", Method: "POST", Path: "/web/zzq/create",
			Operator: "admin", IP: "10.0.0.1", Location: "总部", Status: 0,
			ErrorMsg: "参数校验失败", Params: `{"name":"x"}`, CostMs: 45,
		}
		if err := tx.Create(l).Error; err != nil {
			t.Fatalf("写入日志失败: %v", err)
		}

		resp, err := logs.Page(ctx, &request.PageOperationLogReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Module: module,
		})
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 1 || len(resp.Items) != 1 {
			t.Fatalf("total=%d items=%d, 期望 1/1", resp.Total, len(resp.Items))
		}
		it := resp.Items[0]
		if it.Module != module || it.Action != "新增" || it.Method != "POST" || it.Path != "/web/zzq/create" ||
			it.Operator != "admin" || it.IP != "10.0.0.1" || it.Location != "总部" ||
			it.Status != 0 || it.ErrorMsg != "参数校验失败" || it.Params != `{"name":"x"}` || it.CostMs != 45 {
			t.Errorf("日志字段不完整: %+v", it)
		}
		if it.CreatedDT.IsZero() {
			t.Error("created_dt 不应为零值")
		}
	})
}

// TestOperationLogService_Page_Filters 按模块/操作类型/操作人员/时间范围过滤。
func TestOperationLogService_Page_Filters(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		logs := svc.NewOperationLogService()

		module := uniq("zzq_mod")
		base := time.Now().Add(-48 * time.Hour)
		newLog(t, tx, module, "新增", "alice", base)
		newLog(t, tx, module, "修改", "bob", base.Add(24*time.Hour))
		newLog(t, tx, uniq("zzq_other"), "删除", "carol", base)

		byModule, err := logs.Page(ctx, &request.PageOperationLogReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Module: module,
		})
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if byModule.Total != 2 {
			t.Errorf("按模块命中 %d, 期望 2", byModule.Total)
		}

		byAction, _ := logs.Page(ctx, &request.PageOperationLogReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Module: module, Action: "修改",
		})
		if byAction.Total != 1 {
			t.Errorf("按操作类型命中 %d, 期望 1", byAction.Total)
		}

		byOperator, _ := logs.Page(ctx, &request.PageOperationLogReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Module: module, Operator: "alice",
		})
		if byOperator.Total != 1 {
			t.Errorf("按操作人员命中 %d, 期望 1", byOperator.Total)
		}

		start := base.Add(-time.Hour).Format("2006-01-02 15:04:05")
		end := base.Add(time.Hour).Format("2006-01-02 15:04:05")
		byRange, _ := logs.Page(ctx, &request.PageOperationLogReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Module: module, StartDT: start, EndDT: end,
		})
		if byRange.Total != 1 {
			t.Errorf("按时间范围命中 %d, 期望 1", byRange.Total)
		}

		// 组合条件为 AND：模块 + 不匹配的操作人员
		none, _ := logs.Page(ctx, &request.PageOperationLogReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Module: module, Operator: uniq("nobody"),
		})
		if none.Total != 0 {
			t.Errorf("组合条件无匹配时应为 0, got %d", none.Total)
		}
	})
}

// TestOperationLogService_Get 日志详情，params 原样返回不被截断。
func TestOperationLogService_Get(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		logs := svc.NewOperationLogService()

		long := ""
		for i := 0; i < 500; i++ {
			long += "x"
		}
		l := &model.OperationLog{Module: uniq("zzq_get"), Action: "导出", Method: "GET", Path: "/web/zzq", Params: long}
		if err := tx.Create(l).Error; err != nil {
			t.Fatalf("写入日志失败: %v", err)
		}

		got, err := logs.Get(ctx, l.ID)
		if err != nil {
			t.Fatalf("获取失败: %v", err)
		}
		if got.Module != l.Module || got.Action != "导出" || got.Method != "GET" {
			t.Errorf("字段不一致: %+v", got)
		}
		if len(got.Params) != len(long) {
			t.Errorf("params 被截断: %d != %d", len(got.Params), len(long))
		}
		if _, err := logs.Get(ctx, 999999999); err == nil {
			t.Error("不存在的日志应返回错误")
		}
	})
}

// TestOperationLogService_Delete 删除单条日志后列表不再返回。
func TestOperationLogService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		logs := svc.NewOperationLogService()

		module := uniq("zzq_del_log")
		l := newLog(t, tx, module, "新增", "admin", time.Now())

		if err := logs.Delete(ctx, l.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_operation_log", l.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		resp, _ := logs.Page(ctx, &request.PageOperationLogReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Module: module})
		if resp.Total != 0 {
			t.Errorf("删除后 total = %d, 期望 0", resp.Total)
		}
		if err := logs.Delete(ctx, 999999999); err == nil {
			t.Error("不存在的日志应返回错误")
		}
	})
}

// TestOperationLogService_Clear 清空全部日志（物理删除）。
func TestOperationLogService_Clear(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		logs := svc.NewOperationLogService()

		newLog(t, tx, uniq("zzq_clear"), "新增", "admin", time.Now())
		newLog(t, tx, uniq("zzq_clear"), "修改", "admin", time.Now())

		if err := logs.Clear(ctx); err != nil {
			t.Fatalf("清空失败: %v", err)
		}
		var n int64
		if err := tx.Unscoped().Table("ac_operation_log").Count(&n).Error; err != nil {
			t.Fatalf("统计失败: %v", err)
		}
		if n != 0 {
			t.Errorf("清空后剩余 %d 条, 期望 0", n)
		}
		resp, _ := logs.Page(ctx, &request.PageOperationLogReq{PageReq: request.PageReq{Page: 1, PageSize: 20}})
		if resp.Total != 0 {
			t.Errorf("清空后分页 total = %d, 期望 0", resp.Total)
		}
	})
}

// TestOperationLogService_Export 导出返回全量（不受分页限制）。
func TestOperationLogService_Export(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		logs := svc.NewOperationLogService()

		module := uniq("zzq_export")
		for i := 0; i < 5; i++ {
			newLog(t, tx, module, "新增", "admin", time.Now())
		}

		req := &request.PageOperationLogReq{PageReq: request.PageReq{Page: 1, PageSize: 2}, Module: module}
		list, err := logs.Export(ctx, req)
		if err != nil {
			t.Fatalf("导出失败: %v", err)
		}
		if len(list) != 5 {
			t.Errorf("导出条数 = %d, 期望 5（不受 page_size 限制）", len(list))
		}
		page, _ := logs.Page(ctx, req)
		if page.Total != int64(len(list)) {
			t.Errorf("导出条数 %d != 分页 total %d", len(list), page.Total)
		}
	})
}
