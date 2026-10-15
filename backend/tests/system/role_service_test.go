package system

import (
	"context"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	svc "github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 角色与授权（RoleService）：ps 策略与 g/g2/g3 三链继承的数据库侧
// ════════════════════════════════════════════════════════════════

// TestRoleService_Create 创建角色：全字段落库 + 菜单授权。
func TestRoleService_Create(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		roles := svc.NewRoleService()

		menu1 := pickID(t, tx, "ac_menu", "")
		menu2 := pickID(t, tx, "ac_menu", "id <> ?", menu1)
		code := uniq("zzq_role")

		got, err := roles.Create(ctx, &request.CreateRoleReq{
			Name: "ZZQ-系统管理员", Code: code, Sequence: 3, Description: "拥有部分权限",
			DataPermission: "dept", CrossDeptIDs: "2,3", Active: boolPtr(true),
			MenuIDs: []int64{menu1, menu2},
		})
		if err != nil {
			t.Fatalf("创建角色失败: %v", err)
		}

		r, err := roles.Get(ctx, got.ID)
		if err != nil {
			t.Fatalf("回读角色失败: %v", err)
		}
		if r.Name != "ZZQ-系统管理员" || r.Code != code || r.Sequence != 3 ||
			r.Description != "拥有部分权限" || r.DataPermission != "dept" || r.CrossDeptIDs != "2,3" {
			t.Errorf("字段不一致: %+v", r)
		}
		if r.Active == nil || !*r.Active {
			t.Errorf("active = %v", r.Active)
		}
		if r.CreatedBy != testUserName || r.CreatedDT.IsZero() {
			t.Errorf("审计字段缺失: created_by=%q created_dt=%v", r.CreatedBy, r.CreatedDT)
		}
		if len(r.Menus) != 2 {
			t.Errorf("已授权菜单数 = %d, 期望 2", len(r.Menus))
		}
		if n := countRows(t, tx, "ac_role_menu_rel", "role_id = ?", r.ID); n != 2 {
			t.Errorf("ac_role_menu_rel 条数 = %d, 期望 2", n)
		}

		// 缺省：data_permission 为 all、sequence 为 10、active 为 true
		def, err := roles.Create(ctx, &request.CreateRoleReq{Name: "ZZQ-默认角色", Code: uniq("zzq_def")})
		if err != nil {
			t.Fatalf("创建默认角色失败: %v", err)
		}
		if def.DataPermission != "all" {
			t.Errorf("默认 data_permission = %q, 期望 all", def.DataPermission)
		}
		if def.Sequence != 10 {
			t.Errorf("默认 sequence = %d, 期望 10", def.Sequence)
		}
	})
}

// TestRoleService_Create_DuplicateCode 角色编码唯一（code 是 Casbin 的 sub）。
func TestRoleService_Create_DuplicateCode(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		roles := svc.NewRoleService()

		code := uniq("zzq_dup_role")
		if _, err := roles.Create(ctx, &request.CreateRoleReq{Name: "ZZQ-角色1", Code: code}); err != nil {
			t.Fatalf("首次创建失败: %v", err)
		}
		_, err := roles.Create(ctx, &request.CreateRoleReq{Name: "ZZQ-角色2", Code: code})
		if err == nil {
			t.Fatal("重复编码应返回错误")
		}
		if n := countRows(t, tx, "ac_role", "code = ?", code); n != 1 {
			t.Errorf("同编码角色数 = %d, 期望 1", n)
		}
	})
}

// TestRoleService_Create_DataPermission 数据权限枚举校验：四值合法，非法值被拒。
func TestRoleService_Create_DataPermission(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		roles := svc.NewRoleService()

		for _, perm := range model.DataScopeOption {
			code := uniq("zzq_perm_" + perm)
			r, err := roles.Create(ctx, &request.CreateRoleReq{
				Name: "ZZQ-" + perm, Code: code, DataPermission: perm, CrossDeptIDs: "7,8",
			})
			if err != nil {
				t.Fatalf("data_permission=%s 应合法: %v", perm, err)
			}
			got, _ := roles.Get(ctx, r.ID)
			if got.DataPermission != perm {
				t.Errorf("data_permission = %q, 期望 %q", got.DataPermission, perm)
			}
			if got.CrossDeptIDs != "7,8" {
				t.Errorf("cross_dept_ids = %q, 期望原样保存", got.CrossDeptIDs)
			}
		}

		if _, err := roles.Create(ctx, &request.CreateRoleReq{Name: "ZZQ-非法", Code: uniq("zzq_bad"), DataPermission: "unknown"}); err == nil {
			t.Error("非法 data_permission 应返回错误")
		}

		// 更新时的枚举校验
		r := mustCreateRole(t, ctx, uniq("zzq_perm_upd"), "all")
		if err := roles.Update(ctx, r.ID, &request.UpdateRoleReq{Name: r.Name, DataPermission: "nope"}); err == nil {
			t.Error("更新非法 data_permission 应返回错误")
		}
	})
}

// TestRoleService_Get 角色详情逐字段核对。
func TestRoleService_Get(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		roles := svc.NewRoleService()

		code := uniq("zzq_role_get")
		created, err := roles.Create(ctx, &request.CreateRoleReq{
			Name: "ZZQ-详情角色", Code: code, Sequence: 4, Description: "d",
			DataPermission: "cross_dept", CrossDeptIDs: "5", Active: boolPtr(false),
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		r, err := roles.Get(ctx, created.ID)
		if err != nil {
			t.Fatalf("获取失败: %v", err)
		}
		if r.Name != "ZZQ-详情角色" || r.Code != code || r.Sequence != 4 || r.Description != "d" ||
			r.DataPermission != "cross_dept" || r.CrossDeptIDs != "5" {
			t.Errorf("字段不一致: %+v", r)
		}
		if r.Active == nil || *r.Active {
			t.Errorf("active = %v, 期望 false", r.Active)
		}
		if _, err := roles.Get(ctx, 999999999); err == nil {
			t.Error("不存在的角色应返回错误")
		}
	})
}

// TestRoleService_Update 角色更新：字段全部生效且 code 保持不变。
func TestRoleService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		roles := svc.NewRoleService()

		r := mustCreateRole(t, ctx, uniq("zzq_role_upd"), "all")
		if err := roles.Update(ctx, r.ID, &request.UpdateRoleReq{
			Name: "ZZQ-新名称", Sequence: 9, Description: "新描述",
			DataPermission: "personal", CrossDeptIDs: "11,12", Active: boolPtr(false),
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}
		got, _ := roles.Get(ctx, r.ID)
		if got.Name != "ZZQ-新名称" || got.Sequence != 9 || got.Description != "新描述" ||
			got.DataPermission != "personal" || got.CrossDeptIDs != "11,12" {
			t.Errorf("字段未全部更新: %+v", got)
		}
		if got.Active == nil || *got.Active {
			t.Errorf("active = %v, 期望 false", got.Active)
		}
		if got.Code != r.Code {
			t.Errorf("code 应保持不变: %q -> %q", r.Code, got.Code)
		}
		if got.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", got.UpdatedBy)
		}
	})
}

// TestRoleService_Delete 删除角色：软删除并清理四张关联表。
func TestRoleService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		roles := svc.NewRoleService()

		r := mustCreateRole(t, ctx, uniq("zzq_role_del"), "all")
		menu := pickID(t, tx, "ac_menu", "")
		dept, err := svc.NewDeptService().Create(ctx, &request.CreateDeptReq{Name: "ZZQ-部门", Code: uniq("ZZQ_DEPT")})
		if err != nil {
			t.Fatalf("创建部门失败: %v", err)
		}
		post, err := svc.NewPostService().Create(ctx, &request.CreatePostReq{Name: "ZZQ-岗位", Code: uniq("ZZQ_POST")})
		if err != nil {
			t.Fatalf("创建岗位失败: %v", err)
		}
		u := mustCreateUser(t, ctx, uniq("zzq_role_user"), nil)

		if err := roles.AssignMenus(ctx, r.ID, []int64{menu}); err != nil {
			t.Fatalf("分配菜单失败: %v", err)
		}
		if err := roles.AssignDepts(ctx, r.ID, []int64{dept.ID}); err != nil {
			t.Fatalf("分配部门失败: %v", err)
		}
		if err := roles.AssignPosts(ctx, r.ID, []int64{post.ID}); err != nil {
			t.Fatalf("分配岗位失败: %v", err)
		}
		if err := roles.AssignUsers(ctx, r.ID, []int64{u.ID}); err != nil {
			t.Fatalf("分配用户失败: %v", err)
		}

		if err := roles.Delete(ctx, r.ID); err != nil {
			t.Fatalf("删除角色失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_role", r.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		for _, tbl := range []string{"ac_role_menu_rel", "ac_dept_role_rel", "ac_post_role_rel", "ac_user_role_rel"} {
			if n := countRows(t, tx, tbl, "role_id = ?", r.ID); n != 0 {
				t.Errorf("%s 中该角色的关联应清理, got %d", tbl, n)
			}
		}
		if _, err := roles.Get(ctx, r.ID); err == nil {
			t.Error("软删除后不应可读")
		}
	})
}

// TestRoleService_Delete_CleanCasbin 删除角色同时清理 p/g/g2/g3 策略。
func TestRoleService_Delete_CleanCasbin(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cs := withCasbin(t, tx)
		roles := svc.NewRoleService()

		r := mustCreateRole(t, ctx, uniq("zzq_role_casbin_del"), "all")
		menu := pickID(t, tx, "ac_menu", "")
		dept, _ := svc.NewDeptService().Create(ctx, &request.CreateDeptReq{Name: "ZZQ-部门", Code: uniq("ZZQ_DEPT")})
		post, _ := svc.NewPostService().Create(ctx, &request.CreatePostReq{Name: "ZZQ-岗位", Code: uniq("ZZQ_POST")})
		u := mustCreateUser(t, ctx, uniq("zzq_casbin_user"), nil)

		if err := roles.AssignMenus(ctx, r.ID, []int64{menu}); err != nil {
			t.Fatalf("分配菜单失败: %v", err)
		}
		if err := roles.AssignDepts(ctx, r.ID, []int64{dept.ID}); err != nil {
			t.Fatalf("分配部门失败: %v", err)
		}
		if err := roles.AssignPosts(ctx, r.ID, []int64{post.ID}); err != nil {
			t.Fatalf("分配岗位失败: %v", err)
		}
		if err := roles.AssignUsers(ctx, r.ID, []int64{u.ID}); err != nil {
			t.Fatalf("分配用户失败: %v", err)
		}

		if casbinPolicyCount(t, tx, "g", u.Username, r.Code) != 1 {
			t.Fatal("应生成 g 策略")
		}
		if casbinPolicyCount(t, tx, "g2", dept.Code, r.Code) != 1 {
			t.Fatal("应生成 g2 策略")
		}
		if casbinPolicyCount(t, tx, "g3", post.Code, r.Code) != 1 {
			t.Fatal("应生成 g3 策略")
		}

		if err := roles.Delete(ctx, r.ID); err != nil {
			t.Fatalf("删除角色失败: %v", err)
		}
		for _, pt := range []string{"p", "g", "g2", "g3"} {
			if n := casbinPolicyCount(t, tx, pt, "", r.Code); n != 0 {
				t.Errorf("删除角色后 %s 策略应清空, got %d", pt, n)
			}
		}
		if ok, _ := cs.Enforce(u.Username, "/web/basic/regions", "GET"); ok {
			t.Error("角色删除后用户不应再继承其权限")
		}
	})
}

// TestRoleService_Page 角色分页（ac_role 无 dept_id，不做行级过滤）。
func TestRoleService_Page(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		roles := svc.NewRoleService()

		prefix := uniq("zzq_pagerole")
		for i := 0; i < 3; i++ {
			if _, err := roles.Create(ctx, &request.CreateRoleReq{Name: prefix, Code: prefix + "_" + string(rune('a'+i)), Active: boolPtr(i%2 == 0)}); err != nil {
				t.Fatalf("创建角色失败: %v", err)
			}
		}

		resp, err := roles.Page(ctx, &request.PageRoleReq{PageReq: request.PageReq{Page: 1, PageSize: 10}, Keyword: prefix})
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 3 || len(resp.Items) != 3 {
			t.Fatalf("total=%d items=%d, 期望 3/3", resp.Total, len(resp.Items))
		}
		if resp.Page != 1 || resp.PageSize != 10 {
			t.Errorf("分页回显 page=%d page_size=%d", resp.Page, resp.PageSize)
		}

		active := true
		byActive, err := roles.Page(ctx, &request.PageRoleReq{PageReq: request.PageReq{Page: 1, PageSize: 10}, Keyword: prefix, Active: &active})
		if err != nil {
			t.Fatalf("按 active 分页失败: %v", err)
		}
		if byActive.Total != 2 {
			t.Errorf("active=true 的条数 = %d, 期望 2", byActive.Total)
		}
	})
}

// TestRoleService_Options 角色下拉选项只返回启用项，Code 为 Casbin 角色编码。
func TestRoleService_Options(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		roles := svc.NewRoleService()

		on := mustCreateRole(t, ctx, uniq("zzq_opt_on"), "all")
		off := mustCreateRole(t, ctx, uniq("zzq_opt_off"), "all")
		if err := tx.Model(&model.Role{}).Where("id = ?", off.ID).Update("active", false).Error; err != nil {
			t.Fatalf("停用角色失败: %v", err)
		}

		items, err := roles.Options(ctx)
		if err != nil {
			t.Fatalf("查询选项失败: %v", err)
		}
		seen := map[int64]bool{}
		for _, it := range items {
			seen[it.ID] = true
			if it.ID <= 0 || it.Name == "" || it.Code == "" {
				t.Errorf("选项字段不完整: %+v", it)
			}
		}
		if !seen[on.ID] {
			t.Error("启用角色应出现在选项中")
		}
		if seen[off.ID] {
			t.Error("停用角色不应出现在选项中")
		}
	})
}

// TestRoleService_AssignMenus 覆盖式授权菜单，重复调用幂等。
func TestRoleService_AssignMenus(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		roles := svc.NewRoleService()

		r := mustCreateRole(t, ctx, uniq("zzq_am"), "all")
		m1 := pickID(t, tx, "ac_menu", "")
		m2 := pickID(t, tx, "ac_menu", "id > ?", m1)
		m3 := pickID(t, tx, "ac_menu", "id > ?", m2)
		ids := []int64{m1, m2, m3}

		if err := roles.AssignMenus(ctx, r.ID, ids); err != nil {
			t.Fatalf("分配菜单失败: %v", err)
		}
		if n := countRows(t, tx, "ac_role_menu_rel", "role_id = ?", r.ID); n != 3 {
			t.Fatalf("关联条数 = %d, 期望 3", n)
		}

		got, err := roles.GetMenus(ctx, r.ID)
		if err != nil {
			t.Fatalf("读取已授权菜单失败: %v", err)
		}
		if len(got) != 3 {
			t.Errorf("GetMenus 条数 = %d, 期望 3", len(got))
		}

		// 幂等：重复同一列表不产生重复行
		if err := roles.AssignMenus(ctx, r.ID, ids); err != nil {
			t.Fatalf("重复分配失败: %v", err)
		}
		if n := countRows(t, tx, "ac_role_menu_rel", "role_id = ?", r.ID); n != 3 {
			t.Errorf("重复分配后关联条数 = %d, 期望 3", n)
		}
	})
}

// TestRoleService_AssignMenus_Replace 后一次分配覆盖前一次（不是并集）。
func TestRoleService_AssignMenus_Replace(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		roles := svc.NewRoleService()

		r := mustCreateRole(t, ctx, uniq("zzq_amr"), "all")
		m1 := pickID(t, tx, "ac_menu", "")
		m2 := pickID(t, tx, "ac_menu", "id > ?", m1)
		m3 := pickID(t, tx, "ac_menu", "id > ?", m2)

		if err := roles.AssignMenus(ctx, r.ID, []int64{m1, m2}); err != nil {
			t.Fatalf("首次分配失败: %v", err)
		}
		if err := roles.AssignMenus(ctx, r.ID, []int64{m2, m3}); err != nil {
			t.Fatalf("二次分配失败: %v", err)
		}
		if n := countRows(t, tx, "ac_role_menu_rel", "role_id = ?", r.ID); n != 2 {
			t.Fatalf("关联条数 = %d, 期望 2（替换式而非并集）", n)
		}
		if n := countRows(t, tx, "ac_role_menu_rel", "role_id = ? AND menu_id = ?", r.ID, m1); n != 0 {
			t.Error("被替换掉的菜单关联应被删除")
		}
		got, _ := roles.GetMenus(ctx, r.ID)
		if len(got) != 2 {
			t.Errorf("GetMenus = %v, 期望 2 项", got)
		}
	})
}

// TestRoleService_AssignMenus_Empty 清空授权后 p 策略全清、继承判定失败。
func TestRoleService_AssignMenus_Empty(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cs := withCasbin(t, tx)
		roles := svc.NewRoleService()

		r := mustCreateRole(t, ctx, uniq("zzq_am_empty"), "all")
		menu := pickID(t, tx, "ac_menu", "")
		u := mustCreateUser(t, ctx, uniq("zzq_am_empty_user"), nil)
		if err := roles.AssignUsers(ctx, r.ID, []int64{u.ID}); err != nil {
			t.Fatalf("分配用户失败: %v", err)
		}
		if err := roles.AssignMenus(ctx, r.ID, []int64{menu}); err != nil {
			t.Fatalf("分配菜单失败: %v", err)
		}

		if err := roles.AssignMenus(ctx, r.ID, []int64{}); err != nil {
			t.Fatalf("清空授权失败: %v", err)
		}
		if n := countRows(t, tx, "ac_role_menu_rel", "role_id = ?", r.ID); n != 0 {
			t.Errorf("清空后关联条数 = %d, 期望 0", n)
		}
		if n := casbinPolicyCount(t, tx, "p", r.Code, ""); n != 0 {
			t.Errorf("清空后 p 策略应移除, got %d", n)
		}
		got, _ := roles.GetMenus(ctx, r.ID)
		if len(got) != 0 {
			t.Errorf("GetMenus 应为空, got %v", got)
		}
		if ok, _ := cs.Enforce(u.Username, "/web/basic/regions", "GET"); ok {
			t.Error("清空授权后不应再放行")
		}
	})
}

// TestRoleService_AssignMenus_SyncCasbin 角色→菜单→菜单API 三级推导生成 p 策略。
func TestRoleService_AssignMenus_SyncCasbin(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cs := withCasbin(t, tx)
		roles := svc.NewRoleService()
		menus := svc.NewMenuService()

		r := mustCreateRole(t, ctx, uniq("zzq_sync"), "all")
		u := mustCreateUser(t, ctx, uniq("zzq_sync_user"), nil)

		// 菜单 + 菜单接口是 p 策略的唯一来源
		menu, err := menus.Create(ctx, &request.CreateMenuReq{
			Name: "ZZQ-区域查询", Type: "button", Platform: "pc",
			PermissionCode: "basic:region:list", Sequence: 99,
		})
		if err != nil {
			t.Fatalf("创建菜单失败: %v", err)
		}
		if _, err := menus.CreateAPI(ctx, menu.ID, &request.CreateMenuAPIReq{
			Method: "GET", Path: "/web/basic/regions", Description: "分页查询区域",
		}); err != nil {
			t.Fatalf("创建菜单接口失败: %v", err)
		}

		if err := roles.AssignUsers(ctx, r.ID, []int64{u.ID}); err != nil {
			t.Fatalf("分配用户失败: %v", err)
		}
		if err := roles.AssignMenus(ctx, r.ID, []int64{menu.ID}); err != nil {
			t.Fatalf("分配菜单失败: %v", err)
		}

		if n := casbinPolicyCount(t, tx, "p", r.Code, ""); n != 1 {
			t.Fatalf("p 策略条数 = %d, 期望 1", n)
		}
		var p struct {
			V1 string
			V2 string
		}
		if err := tx.Table("casbin_rule").Where("p_type = ? AND v0 = ?", "p", r.Code).
			Select("v1, v2").Scan(&p).Error; err != nil {
			t.Fatalf("读取 p 策略失败: %v", err)
		}
		if p.V1 != "/web/basic/regions" || p.V2 != "GET" {
			t.Errorf("p 策略 = (%q, %q), 期望 (/web/basic/regions, GET)", p.V1, p.V2)
		}

		ok, err := cs.Enforce(u.Username, "/web/basic/regions", "GET")
		if err != nil {
			t.Fatalf("Enforce 失败: %v", err)
		}
		if !ok {
			t.Error("已授权接口应放行")
		}
		denied, _ := cs.Enforce(u.Username, "/web/system/users", "DELETE")
		if denied {
			t.Error("未授权接口不应放行")
		}
	})
}

// TestRoleService_GetMenus 读回已授权菜单集合与写入一致。
func TestRoleService_GetMenus(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		roles := svc.NewRoleService()

		r := mustCreateRole(t, ctx, uniq("zzq_gm"), "all")
		m1 := pickID(t, tx, "ac_menu", "")
		m2 := pickID(t, tx, "ac_menu", "id > ?", m1)
		if err := roles.AssignMenus(ctx, r.ID, []int64{m1, m2}); err != nil {
			t.Fatalf("分配菜单失败: %v", err)
		}

		got, err := roles.GetMenus(ctx, r.ID)
		if err != nil {
			t.Fatalf("读取失败: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("条数 = %d, 期望 2", len(got))
		}
		set := map[int64]bool{got[0]: true, got[1]: true}
		if !set[m1] || !set[m2] {
			t.Errorf("菜单集合不一致: %v", got)
		}
	})
}

// TestRoleService_AssignDepts 部门继承（g2 链）：数据 + Casbin 双向一致。
func TestRoleService_AssignDepts(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		withCasbin(t, tx)
		roles := svc.NewRoleService()
		depts := svc.NewDeptService()

		r := mustCreateRole(t, ctx, uniq("zzq_g2"), "all")
		d1, err := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-部门1", Code: uniq("ZZQ_D1")})
		if err != nil {
			t.Fatalf("创建部门失败: %v", err)
		}
		d2, err := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-部门2", Code: uniq("ZZQ_D2")})
		if err != nil {
			t.Fatalf("创建部门失败: %v", err)
		}

		if err := roles.AssignDepts(ctx, r.ID, []int64{d1.ID, d2.ID}); err != nil {
			t.Fatalf("分配部门失败: %v", err)
		}
		if n := countRows(t, tx, "ac_dept_role_rel", "role_id = ?", r.ID); n != 2 {
			t.Errorf("关联条数 = %d, 期望 2", n)
		}
		if n := casbinPolicyCount(t, tx, "g2", d1.Code, r.Code); n != 1 {
			t.Errorf("应生成 g2 策略 (v0=%s, v1=%s)", d1.Code, r.Code)
		}

		got, err := roles.GetDepts(ctx, r.ID)
		if err != nil {
			t.Fatalf("读取失败: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("GetDepts 条数 = %d, 期望 2", len(got))
		}
	})
}

// TestRoleService_AssignPosts 岗位继承（g3 链）：数据 + Casbin 双向一致。
func TestRoleService_AssignPosts(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		withCasbin(t, tx)
		roles := svc.NewRoleService()
		posts := svc.NewPostService()

		r := mustCreateRole(t, ctx, uniq("zzq_g3"), "all")
		p1, err := posts.Create(ctx, &request.CreatePostReq{Name: "ZZQ-岗位1", Code: uniq("ZZQ_P1")})
		if err != nil {
			t.Fatalf("创建岗位失败: %v", err)
		}

		if err := roles.AssignPosts(ctx, r.ID, []int64{p1.ID}); err != nil {
			t.Fatalf("分配岗位失败: %v", err)
		}
		if n := countRows(t, tx, "ac_post_role_rel", "role_id = ?", r.ID); n != 1 {
			t.Errorf("关联条数 = %d, 期望 1", n)
		}
		if n := casbinPolicyCount(t, tx, "g3", p1.Code, r.Code); n != 1 {
			t.Errorf("应生成 g3 策略 (v0=%s, v1=%s)", p1.Code, r.Code)
		}
		got, err := roles.GetPosts(ctx, r.ID)
		if err != nil {
			t.Fatalf("读取失败: %v", err)
		}
		if len(got) != 1 || got[0] != p1.ID {
			t.Errorf("GetPosts = %v, 期望 [%d]", got, p1.ID)
		}
	})
}

// TestRoleService_AssignUsers 用户直授（g 链）：数据 + Casbin 双向一致。
func TestRoleService_AssignUsers(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		withCasbin(t, tx)
		roles := svc.NewRoleService()

		r := mustCreateRole(t, ctx, uniq("zzq_g"), "all")
		u1 := mustCreateUser(t, ctx, uniq("zzq_g_user1"), nil)
		u2 := mustCreateUser(t, ctx, uniq("zzq_g_user2"), nil)

		if err := roles.AssignUsers(ctx, r.ID, []int64{u1.ID, u2.ID}); err != nil {
			t.Fatalf("分配用户失败: %v", err)
		}
		if n := countRows(t, tx, "ac_user_role_rel", "role_id = ?", r.ID); n != 2 {
			t.Errorf("关联条数 = %d, 期望 2", n)
		}
		if n := casbinPolicyCount(t, tx, "g", u1.Username, r.Code); n != 1 {
			t.Errorf("应生成 g 策略 (v0=%s, v1=%s)", u1.Username, r.Code)
		}

		got, err := roles.GetUsers(ctx, r.ID)
		if err != nil {
			t.Fatalf("读取失败: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("GetUsers 条数 = %d, 期望 2", len(got))
		}

		// 覆盖式：再次分配仅保留新集合
		if err := roles.AssignUsers(ctx, r.ID, []int64{u2.ID}); err != nil {
			t.Fatalf("覆盖分配失败: %v", err)
		}
		if n := countRows(t, tx, "ac_user_role_rel", "role_id = ?", r.ID); n != 1 {
			t.Errorf("覆盖后关联条数 = %d, 期望 1", n)
		}
		if n := casbinPolicyCount(t, tx, "g", u1.Username, r.Code); n != 0 {
			t.Error("被移除用户的 g 策略应清理")
		}
	})
}
