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
// 菜单管理（MenuService）与部门管理（DeptService）
// ════════════════════════════════════════════════════════════════

// ── MenuService ───────────────────────────────────────────────

// TestMenuService_Create 创建菜单：全字段落库，level 依据父菜单自动推导。
func TestMenuService_Create(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		root, err := menus.Create(ctx, &request.CreateMenuReq{
			Name: "ZZQ-目录", Path: "/zzq", Icon: "folder", Component: "Layout",
			Type: "directory", Platform: "pc", Sequence: 11, Description: "根目录",
			Active: boolPtr(true), Hidden: boolPtr(false),
		})
		if err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
		if root.Level != 1 {
			t.Errorf("根菜单 level = %d, 期望 1", root.Level)
		}
		if root.Name != "ZZQ-目录" || root.Path != "/zzq" || root.Icon != "folder" ||
			root.Component != "Layout" || root.Type != "directory" || root.Platform != "pc" ||
			root.Sequence != 11 || root.Description != "根目录" {
			t.Errorf("字段不一致: %+v", root)
		}
		if root.Active == nil || !*root.Active || root.Hidden == nil || *root.Hidden {
			t.Errorf("active/hidden 默认值异常: %v/%v", root.Active, root.Hidden)
		}

		child, err := menus.Create(ctx, &request.CreateMenuReq{
			Name: "ZZQ-子菜单", ParentID: &root.ID, Type: "menu", Platform: "pc", Sequence: 12,
		})
		if err != nil {
			t.Fatalf("创建子菜单失败: %v", err)
		}
		if child.Level != 2 {
			t.Errorf("子菜单 level = %d, 期望 2", child.Level)
		}
		if child.ParentID == nil || *child.ParentID != root.ID {
			t.Errorf("parent_id = %v", child.ParentID)
		}

		got, err := menus.Get(ctx, root.ID)
		if err != nil {
			t.Fatalf("回读菜单失败: %v", err)
		}
		if got.CreatedBy != testUserName || got.CreatedDT.IsZero() {
			t.Errorf("审计字段缺失: created_by=%q created_dt=%v", got.CreatedBy, got.CreatedDT)
		}

		// 父菜单不存在时拒绝
		if _, err := menus.Create(ctx, &request.CreateMenuReq{
			Name: "ZZQ-孤儿", ParentID: int64Ptr(999999999), Type: "menu", Platform: "pc",
		}); err == nil {
			t.Error("父菜单不存在时应返回错误")
		}
	})
}

// TestMenuService_Create_Button 按钮型菜单的 permission_code 正确落库并可由菜单树返回。
func TestMenuService_Create_Button(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		parent, err := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-风险", Type: "menu", Platform: "pc", Sequence: 13})
		if err != nil {
			t.Fatalf("创建父菜单失败: %v", err)
		}
		btn, err := menus.Create(ctx, &request.CreateMenuReq{
			Name: "ZZQ-处理按钮", ParentID: &parent.ID, Type: "button", Platform: "pc",
			PermissionCode: "risk:handle", Sequence: 14,
		})
		if err != nil {
			t.Fatalf("创建按钮失败: %v", err)
		}

		got, _ := menus.Get(ctx, btn.ID)
		if got.Type != "button" || got.PermissionCode != "risk:handle" {
			t.Errorf("按钮字段不一致: %+v", got)
		}
		if got.Level != 2 {
			t.Errorf("按钮 level = %d, 期望 2", got.Level)
		}

		// Tree 返回该节点且 permission_code 非空
		list, err := menus.Tree(ctx, "pc")
		if err != nil {
			t.Fatalf("查询菜单树失败: %v", err)
		}
		found := findMenu(list, btn.ID)
		if found == nil {
			t.Fatal("按钮节点应出现在菜单树中")
		}
		if found.PermissionCode != "risk:handle" {
			t.Errorf("permission_code = %q", found.PermissionCode)
		}
	})
}

// TestMenuService_Create_Type 菜单类型枚举：directory/menu/button 三种均可创建。
func TestMenuService_Create_Type(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		for i, tp := range []string{"directory", "menu", "button"} {
			m, err := menus.Create(ctx, &request.CreateMenuReq{
				Name: "ZZQ-类型" + tp, Type: tp, Platform: "pc", Sequence: 20 + i,
			})
			if err != nil {
				t.Fatalf("创建 type=%s 失败: %v", tp, err)
			}
			if m.Type != tp {
				t.Errorf("type = %q, 期望 %q", m.Type, tp)
			}
		}

		// 缺省 type 为 menu、platform 为 pc
		def, err := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-缺省类型", Sequence: 30})
		if err != nil {
			t.Fatalf("创建缺省菜单失败: %v", err)
		}
		if def.Type != "menu" {
			t.Errorf("缺省 type = %q, 期望 menu", def.Type)
		}
		if def.Platform != "pc" {
			t.Errorf("缺省 platform = %q, 期望 pc", def.Platform)
		}
	})
}

// TestMenuService_Tree 菜单树：父子嵌套与 level 深度一致。
func TestMenuService_Tree(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		root, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-T1", Type: "directory", Platform: "pc", Sequence: 40})
		mid, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-T2", ParentID: &root.ID, Type: "menu", Platform: "pc", Sequence: 41})
		leaf, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-T3", ParentID: &mid.ID, Type: "button", Platform: "pc", Sequence: 42})

		list, err := menus.Tree(ctx, "pc")
		if err != nil {
			t.Fatalf("查询菜单树失败: %v", err)
		}
		n1 := findMenu(list, root.ID)
		if n1 == nil {
			t.Fatal("根节点未出现在树中")
		}
		if len(n1.Children) == 0 {
			t.Fatal("根节点应有子节点")
		}
		n2 := findMenu(n1.Children, mid.ID)
		if n2 == nil {
			t.Fatal("第二层节点未挂到根节点下")
		}
		if len(n2.Children) == 0 {
			t.Fatal("第二层节点应有子节点")
		}
		n3 := findMenu(n2.Children, leaf.ID)
		if n3 == nil {
			t.Fatal("第三层节点未正确嵌套")
		}
		if n3.Level != 3 || n2.Level != 2 || n1.Level != 1 {
			t.Errorf("level 与嵌套深度不匹配: %d/%d/%d", n1.Level, n2.Level, n3.Level)
		}
		if len(n3.Children) != 0 {
			t.Error("叶子节点不应有子节点")
		}
	})
}

// TestMenuService_Tree_Platform 菜单树按平台过滤：pc 与 mobile 互不包含。
func TestMenuService_Tree_Platform(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		pc, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-PC", Type: "menu", Platform: "pc", Sequence: 50})
		mb, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-MB", Type: "menu", Platform: "mobile", Sequence: 51})

		pcTree, err := menus.Tree(ctx, "pc")
		if err != nil {
			t.Fatalf("查询 pc 菜单树失败: %v", err)
		}
		if findMenu(pcTree, pc.ID) == nil {
			t.Error("pc 菜单树应包含 pc 菜单")
		}
		if findMenu(pcTree, mb.ID) != nil {
			t.Error("pc 菜单树不应包含 mobile 菜单")
		}
		walkMenus(pcTree, func(m model.Menu) {
			if m.Platform != "pc" {
				t.Errorf("pc 树中出现平台 %q 的节点", m.Platform)
			}
		})

		mbTree, _ := menus.Tree(ctx, "mobile")
		if findMenu(mbTree, mb.ID) == nil {
			t.Error("mobile 菜单树应包含 mobile 菜单")
		}
		if findMenu(mbTree, pc.ID) != nil {
			t.Error("mobile 菜单树不应包含 pc 菜单")
		}

		// platform 为空时默认 pc
		defTree, _ := menus.Tree(ctx, "")
		if findMenu(defTree, pc.ID) == nil {
			t.Error("platform 缺省时应按 pc 返回")
		}
	})
}

// TestMenuService_Tree_Inactive 停用菜单不出现在树中。
func TestMenuService_Tree_Inactive(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		off, err := menus.Create(ctx, &request.CreateMenuReq{
			Name: "ZZQ-停用菜单", Type: "menu", Platform: "pc", Sequence: 60, Active: boolPtr(false),
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		child, _ := menus.Create(ctx, &request.CreateMenuReq{
			Name: "ZZQ-停用菜单的子节点", ParentID: &off.ID, Type: "button", Platform: "pc", Sequence: 61, Active: boolPtr(true),
		})

		list, err := menus.Tree(ctx, "pc")
		if err != nil {
			t.Fatalf("查询菜单树失败: %v", err)
		}
		if findMenu(list, off.ID) != nil {
			t.Error("停用菜单不应出现在树中")
		}
		// 父节点缺失时子节点被提升为根节点（避免丢节点）
		if findMenu(list, child.ID) == nil {
			t.Error("父菜单停用时子节点应被提升为根节点而非丢失")
		}
	})
}

// TestMenuService_Update 菜单更新：字段全部生效，改 parent 后 level 重算。
func TestMenuService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		p1, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-P1", Type: "directory", Platform: "pc", Sequence: 70})
		p2, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-P2", ParentID: &p1.ID, Type: "directory", Platform: "pc", Sequence: 71})
		m, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-被更新", ParentID: &p1.ID, Type: "menu", Platform: "pc", Sequence: 72})

		if err := menus.Update(ctx, m.ID, &request.UpdateMenuReq{
			Name: "ZZQ-新名称", Path: "/new", Icon: "star", Component: "new/index",
			Sequence: 73, Hidden: boolPtr(true), PermissionCode: "zzq:new", Description: "新描述",
			Active: boolPtr(false),
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}
		got, _ := menus.Get(ctx, m.ID)
		if got.Name != "ZZQ-新名称" || got.Path != "/new" || got.Icon != "star" || got.Component != "new/index" ||
			got.Sequence != 73 || got.PermissionCode != "zzq:new" || got.Description != "新描述" {
			t.Errorf("字段未全部更新: %+v", got)
		}
		if got.Hidden == nil || !*got.Hidden {
			t.Errorf("hidden = %v, 期望 true", got.Hidden)
		}
		if got.Active == nil || *got.Active {
			t.Errorf("active = %v, 期望 false", got.Active)
		}
		if got.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", got.UpdatedBy)
		}

		// 改父菜单后 level 重算
		if err := menus.Update(ctx, m.ID, &request.UpdateMenuReq{Name: got.Name, ParentID: &p2.ID}); err != nil {
			t.Fatalf("改父菜单失败: %v", err)
		}
		moved, _ := menus.Get(ctx, m.ID)
		if moved.Level != 3 {
			t.Errorf("改父后 level = %d, 期望 3", moved.Level)
		}

		// 父菜单不能是自身 / 自身的子菜单
		if err := menus.Update(ctx, m.ID, &request.UpdateMenuReq{Name: moved.Name, ParentID: &m.ID}); err == nil {
			t.Error("父菜单为自身应返回错误")
		}
		if err := menus.Update(ctx, p1.ID, &request.UpdateMenuReq{Name: p1.Name, ParentID: &m.ID}); err == nil {
			t.Error("父菜单为自身子菜单应返回错误")
		}
	})
}

// TestMenuService_Delete 删除叶子菜单：软删除并清理角色关联。
func TestMenuService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		role := mustCreateRole(t, ctx, uniq("zzq_menu_del_role"), "all")
		m, err := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-待删菜单", Type: "menu", Platform: "pc", Sequence: 80})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if err := svc.NewRoleService().AssignMenus(ctx, role.ID, []int64{m.ID}); err != nil {
			t.Fatalf("分配菜单失败: %v", err)
		}

		if err := menus.Delete(ctx, m.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_menu", m.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		if n := countRows(t, tx, "ac_role_menu_rel", "menu_id = ?", m.ID); n != 0 {
			t.Errorf("角色-菜单关联应清理, got %d", n)
		}
		list, _ := menus.Tree(ctx, "pc")
		if findMenu(list, m.ID) != nil {
			t.Error("软删除后不应出现在菜单树中")
		}
	})
}

// TestMenuService_Delete_WithChildren 存在子菜单时拒绝删除（不得产生孤儿节点）。
func TestMenuService_Delete_WithChildren(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		parent, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-父菜单", Type: "directory", Platform: "pc", Sequence: 85})
		child, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-子菜单", ParentID: &parent.ID, Type: "menu", Platform: "pc", Sequence: 86})

		err := menus.Delete(ctx, parent.ID)
		if err == nil {
			t.Fatal("存在子菜单时应拒绝删除")
		}
		if dt := softDeletedAt(t, tx, "ac_menu", parent.ID); dt != nil {
			t.Error("拒绝删除时 deleted_at 应仍为空")
		}
		if n := countRows(t, tx, "ac_menu", "id = ? AND deleted_at IS NULL", child.ID); n != 1 {
			t.Error("子菜单应仍然存在")
		}
		// 先删子菜单再删父菜单可成功
		if err := menus.Delete(ctx, child.ID); err != nil {
			t.Fatalf("删除子菜单失败: %v", err)
		}
		if err := menus.Delete(ctx, parent.ID); err != nil {
			t.Errorf("删完子菜单后应可删除父菜单: %v", err)
		}
	})
}

// TestMenuService_CreateAPI 新增菜单接口：字段落库并可被 ListAPIs 读回。
func TestMenuService_CreateAPI(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		m, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-接口菜单", Type: "menu", Platform: "pc", Sequence: 90})
		api, err := menus.CreateAPI(ctx, m.ID, &request.CreateMenuAPIReq{
			Method: "GET", Path: "/web/basic/regions", Description: "分页查询区域",
		})
		if err != nil {
			t.Fatalf("新增菜单接口失败: %v", err)
		}
		if api.MenuID != m.ID || api.Method != "GET" || api.Path != "/web/basic/regions" || api.Description != "分页查询区域" {
			t.Errorf("字段不一致: %+v", api)
		}
		if _, err := menus.CreateAPI(ctx, 999999999, &request.CreateMenuAPIReq{Method: "GET", Path: "/x"}); err == nil {
			t.Error("菜单不存在时应返回错误")
		}
	})
}

// TestMenuService_ListAPIs 查询菜单绑定的接口列表。
func TestMenuService_ListAPIs(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		m, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-接口列表", Type: "menu", Platform: "pc", Sequence: 91})
		if _, err := menus.CreateAPI(ctx, m.ID, &request.CreateMenuAPIReq{Method: "GET", Path: "/web/zzq/a"}); err != nil {
			t.Fatalf("新增失败: %v", err)
		}
		if _, err := menus.CreateAPI(ctx, m.ID, &request.CreateMenuAPIReq{Method: "POST", Path: "/web/zzq/a"}); err != nil {
			t.Fatalf("新增失败: %v", err)
		}

		list, err := menus.ListAPIs(ctx, m.ID)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("接口数 = %d, 期望 2", len(list))
		}
		for _, a := range list {
			if a.MenuID != m.ID || a.Method == "" || a.Path == "" {
				t.Errorf("接口字段不完整: %+v", a)
			}
		}
		if _, err := menus.ListAPIs(ctx, 999999999); err == nil {
			t.Error("菜单不存在时应返回错误")
		}
	})
}

// TestMenuService_DeleteAPI 删除菜单接口（软删除），并校验归属。
func TestMenuService_DeleteAPI(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		m1, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-M1", Type: "menu", Platform: "pc", Sequence: 92})
		m2, _ := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-M2", Type: "menu", Platform: "pc", Sequence: 93})
		api, err := menus.CreateAPI(ctx, m1.ID, &request.CreateMenuAPIReq{Method: "GET", Path: "/web/zzq/del"})
		if err != nil {
			t.Fatalf("新增失败: %v", err)
		}

		// 归属校验
		if err := menus.DeleteAPI(ctx, m2.ID, api.ID); err == nil {
			t.Error("接口不属于该菜单时应返回错误")
		}
		if dt := softDeletedAt(t, tx, "ac_menu_api", api.ID); dt != nil {
			t.Error("归属校验失败时不应删除")
		}

		// menuID 传 0 时按接口自身归属处理
		if err := menus.DeleteAPI(ctx, 0, api.ID); err != nil {
			t.Fatalf("删除接口失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_menu_api", api.ID); dt == nil {
			t.Error("deleted_at 应非空")
		}
		list, _ := menus.ListAPIs(ctx, m1.ID)
		if len(list) != 0 {
			t.Errorf("软删除后不应出现在接口列表中, got %d", len(list))
		}
	})
}

// ── DeptService ───────────────────────────────────────────────

// TestDeptService_Create 创建部门：全字段落库。
func TestDeptService_Create(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		depts := svc.NewDeptService()

		leader := pickID(t, tx, "ac_user", "")
		code := uniq("ZZQ_DEPT")
		got, err := depts.Create(ctx, &request.CreateDeptReq{
			Name: "生产部", Code: code, Sequence: 7, LeaderID: &leader,
			Description: "生产管理部门", Active: boolPtr(true),
		})
		if err != nil {
			t.Fatalf("创建部门失败: %v", err)
		}
		if got.Name != "生产部" || got.Code != code || got.Sequence != 7 || got.Description != "生产管理部门" {
			t.Errorf("字段不一致: %+v", got)
		}
		if got.LeaderID == nil || *got.LeaderID != leader {
			t.Errorf("leader_id = %v", got.LeaderID)
		}
		if got.Active == nil || !*got.Active || got.CreatedBy != testUserName || got.CreatedDT.IsZero() {
			t.Errorf("默认值/审计字段异常: %+v", got)
		}
		if got.Level != 1 || got.Path != "/"+code {
			t.Errorf("level/path = %d/%q, 期望 1//%s", got.Level, got.Path, code)
		}

		// 缺省 sequence 为 10
		def, _ := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-默认部门", Code: uniq("ZZQ_DEFD")})
		if def.Sequence != 10 {
			t.Errorf("默认 sequence = %d, 期望 10", def.Sequence)
		}
	})
}

// TestDeptService_Create_Level 层级推导：level 1/2/3，path 逐级拼接。
func TestDeptService_Create_Level(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		depts := svc.NewDeptService()

		c1, c2, c3 := uniq("D1"), uniq("D2"), uniq("D3")
		root, err := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-顶级", Code: c1})
		if err != nil {
			t.Fatalf("创建顶级部门失败: %v", err)
		}
		sub, err := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-子级", Code: c2, ParentID: &root.ID})
		if err != nil {
			t.Fatalf("创建子部门失败: %v", err)
		}
		grand, err := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-孙级", Code: c3, ParentID: &sub.ID})
		if err != nil {
			t.Fatalf("创建孙部门失败: %v", err)
		}

		if root.Level != 1 || sub.Level != 2 || grand.Level != 3 {
			t.Errorf("level = %d/%d/%d, 期望 1/2/3", root.Level, sub.Level, grand.Level)
		}
		if root.Path != "/"+c1 {
			t.Errorf("root.path = %q, 期望 /%s", root.Path, c1)
		}
		if sub.Path != root.Path+"/"+c2 {
			t.Errorf("sub.path = %q, 期望 %s", sub.Path, root.Path+"/"+c2)
		}
		if grand.Path != sub.Path+"/"+c3 {
			t.Errorf("grand.path = %q, 期望 %s", grand.Path, sub.Path+"/"+c3)
		}
		if sub.ParentID == nil || *sub.ParentID != root.ID {
			t.Errorf("sub.parent_id = %v", sub.ParentID)
		}

		// 上级部门不存在时拒绝
		if _, err := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-孤儿", Code: uniq("D4"), ParentID: int64Ptr(999999999)}); err == nil {
			t.Error("上级部门不存在时应返回错误")
		}
	})
}

// TestDeptService_Create_Code 部门编码（Casbin g2 主体）落库且唯一。
func TestDeptService_Create_Code(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		depts := svc.NewDeptService()

		code := uniq("D-PROD")
		d, err := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-生产部", Code: code})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if d.Code != code {
			t.Errorf("code = %q, 期望 %q", d.Code, code)
		}
		if _, err := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-重复", Code: code}); err == nil {
			t.Error("重复部门编码应返回错误")
		}
		if n := countRows(t, tx, "ac_dept", "code = ?", code); n != 1 {
			t.Errorf("同编码部门数 = %d, 期望 1", n)
		}
	})
}

// TestDeptService_Tree 部门树：父子嵌套正确，顶级 parent_id 为 nil。
func TestDeptService_Tree(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		depts := svc.NewDeptService()

		root, _ := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-T1", Code: uniq("DT1")})
		sub, _ := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-T2", Code: uniq("DT2"), ParentID: &root.ID})
		grand, _ := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-T3", Code: uniq("DT3"), ParentID: &sub.ID})

		list, err := depts.Tree(ctx)
		if err != nil {
			t.Fatalf("查询部门树失败: %v", err)
		}
		n1 := findDept(list, root.ID)
		if n1 == nil {
			t.Fatal("根部门未出现在树中")
		}
		n2 := findDept(n1.Children, sub.ID)
		if n2 == nil {
			t.Fatal("子部门未挂到根部门下")
		}
		n3 := findDept(n2.Children, grand.ID)
		if n3 == nil {
			t.Fatal("孙部门未正确嵌套")
		}
		if n1.ParentID != nil {
			t.Errorf("顶级部门 parent_id 应为 nil, got %v", n1.ParentID)
		}
	})
}

// TestDeptService_Options 部门下拉选项只返回启用部门。
func TestDeptService_Options(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		depts := svc.NewDeptService()

		on, _ := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-启用部门", Code: uniq("OPT_ON"), Active: boolPtr(true)})
		off, _ := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-停用部门", Code: uniq("OPT_OFF"), Active: boolPtr(false)})

		items, err := depts.Options(ctx)
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
			t.Error("启用部门应出现在选项中")
		}
		if seen[off.ID] {
			t.Error("停用部门不应出现在选项中")
		}
	})
}

// TestDeptService_Get 部门详情逐字段核对。
func TestDeptService_Get(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		depts := svc.NewDeptService()

		code := uniq("GET_DEPT")
		created, _ := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-详情部门", Code: code, Description: "d", Sequence: 3})
		got, err := depts.Get(ctx, created.ID)
		if err != nil {
			t.Fatalf("获取失败: %v", err)
		}
		if got.Name != "ZZQ-详情部门" || got.Code != code || got.Description != "d" || got.Sequence != 3 {
			t.Errorf("字段不一致: %+v", got)
		}
		if got.Level != 1 || got.Path != "/"+code {
			t.Errorf("level/path = %d/%q", got.Level, got.Path)
		}
		if got.ParentID != nil {
			t.Errorf("parent_id 应为 nil, got %v", got.ParentID)
		}
		if _, err := depts.Get(ctx, 999999999); err == nil {
			t.Error("不存在的部门应返回错误")
		}
	})
}

// TestDeptService_Update 部门更新：字段全部生效，未传 parent 时 level/path 保持。
func TestDeptService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		depts := svc.NewDeptService()

		leader := pickID(t, tx, "ac_user", "")
		created, err := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-旧部门", Code: uniq("UPD_DEPT")})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		oldPath, oldLevel := created.Path, created.Level

		if err := depts.Update(ctx, created.ID, &request.UpdateDeptReq{
			Name: "ZZQ-新部门", Sequence: 21, LeaderID: &leader, Description: "新描述", Active: boolPtr(false),
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		got, _ := depts.Get(ctx, created.ID)
		if got.Name != "ZZQ-新部门" || got.Sequence != 21 || got.Description != "新描述" {
			t.Errorf("字段未全部更新: %+v", got)
		}
		if got.LeaderID == nil || *got.LeaderID != leader {
			t.Errorf("leader_id = %v", got.LeaderID)
		}
		if got.Active == nil || *got.Active {
			t.Errorf("active = %v, 期望 false", got.Active)
		}
		if got.Path != oldPath || got.Level != oldLevel {
			t.Errorf("未传 parent 时 path/level 不应变化: %q/%d -> %q/%d", oldPath, oldLevel, got.Path, got.Level)
		}
		if got.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", got.UpdatedBy)
		}

		// 上级部门不能是自身
		if err := depts.Update(ctx, created.ID, &request.UpdateDeptReq{Name: got.Name, ParentID: &created.ID}); err == nil {
			t.Error("上级部门为自身应返回错误")
		}
	})
}

// TestDeptService_Delete 部门删除：有下级或用户时拒绝，否则软删除。
func TestDeptService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		depts := svc.NewDeptService()

		// 有子部门 -> 拒绝
		parent, _ := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-父部门", Code: uniq("DEL_P")})
		if _, err := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-子部门", Code: uniq("DEL_C"), ParentID: &parent.ID}); err != nil {
			t.Fatalf("创建子部门失败: %v", err)
		}
		if err := depts.Delete(ctx, parent.ID); err == nil {
			t.Fatal("存在子部门时应拒绝删除")
		}

		// 有用户 -> 拒绝
		withUser, _ := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-有用户部门", Code: uniq("DEL_U")})
		mustCreateUser(t, ctx, uniq("zzq_dept_user"), &request.CreateUserReq{DeptID: &withUser.ID})
		if err := depts.Delete(ctx, withUser.ID); err == nil {
			t.Fatal("存在用户时应拒绝删除")
		}

		// 无子无用户 -> 软删除
		free, _ := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-空闲部门", Code: uniq("DEL_F")})
		if err := depts.Delete(ctx, free.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_dept", free.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		tree, _ := depts.Tree(ctx)
		if findDept(tree, free.ID) != nil {
			t.Error("软删除后不应出现在部门树中")
		}
	})
}

// ── 树查找工具 ────────────────────────────────────────────────

// findMenu 在菜单树中按 ID 查找节点（深度优先）。
func findMenu(nodes []model.Menu, id int64) *model.Menu {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
		if found := findMenu(nodes[i].Children, id); found != nil {
			return found
		}
	}
	return nil
}

// findDept 在部门树中按 ID 查找节点（深度优先）。
func findDept(nodes []model.Dept, id int64) *model.Dept {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
		if found := findDept(nodes[i].Children, id); found != nil {
			return found
		}
	}
	return nil
}
