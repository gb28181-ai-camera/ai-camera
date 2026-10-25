package system

import (
	"context"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	svc "github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 菜单管理树范围 / 菜单→角色授权 / 参数 active+schema+类型别名 /
// 菜单父级类型校验（对照 boticz-mes 的账号与权限模块）
// ════════════════════════════════════════════════════════════════

// TestMenuService_Tree_IncludeInactiveAllPlatform 管理端树范围：
// include_inactive 控制是否返回停用菜单，platform=all 控制是否同时返回 PC 与移动端。
func TestMenuService_Tree_IncludeInactiveAllPlatform(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		pcDir, err := menus.Create(ctx, &request.CreateMenuReq{
			Name: "ZZQ-树范围-PC目录", Type: "directory", Platform: "pc", Sequence: 201,
		})
		if err != nil {
			t.Fatalf("创建 PC 目录失败: %v", err)
		}
		pcOff, err := menus.Create(ctx, &request.CreateMenuReq{
			Name: "ZZQ-树范围-停用菜单", Type: "menu", Platform: "pc", Sequence: 202,
			Active: boolPtr(false),
		})
		if err != nil {
			t.Fatalf("创建停用菜单失败: %v", err)
		}
		mobile, err := menus.Create(ctx, &request.CreateMenuReq{
			Name: "ZZQ-树范围-移动端目录", Type: "directory", Platform: "mobile", Sequence: 203,
		})
		if err != nil {
			t.Fatalf("创建移动端菜单失败: %v", err)
		}

		// 默认行为：仅启用 PC 菜单（角色授权树语义）。
		def, err := menus.Tree(ctx, "pc")
		if err != nil {
			t.Fatalf("默认菜单树失败: %v", err)
		}
		if findMenu(def, pcDir.ID) == nil {
			t.Error("默认菜单树应包含启用 PC 目录")
		}
		if findMenu(def, pcOff.ID) != nil {
			t.Error("默认菜单树不应包含停用菜单")
		}
		if findMenu(def, mobile.ID) != nil {
			t.Error("默认菜单树不应包含移动端菜单")
		}

		// include_inactive=true + platform=pc：含停用 PC，不含移动端。
		pcAll, err := menus.TreeWithOptions(ctx, "pc", true)
		if err != nil {
			t.Fatalf("PC 全量菜单树失败: %v", err)
		}
		if findMenu(pcAll, pcDir.ID) == nil || findMenu(pcAll, pcOff.ID) == nil {
			t.Error("include_inactive=true 时应包含启用与停用 PC 菜单")
		}
		if findMenu(pcAll, mobile.ID) != nil {
			t.Error("platform=pc 时不应包含移动端菜单")
		}

		// platform=all + 仅启用：含 PC 与移动端，不含停用。
		activeAll, err := menus.TreeWithOptions(ctx, "all", false)
		if err != nil {
			t.Fatalf("全平台启用菜单树失败: %v", err)
		}
		if findMenu(activeAll, pcDir.ID) == nil || findMenu(activeAll, mobile.ID) == nil {
			t.Error("platform=all 时应包含 PC 与移动端启用菜单")
		}
		if findMenu(activeAll, pcOff.ID) != nil {
			t.Error("include_inactive=false 时不应包含停用菜单")
		}

		// 管理页组合：include_inactive=true + platform=all 返回全部菜单。
		all, err := menus.TreeWithOptions(ctx, "all", true)
		if err != nil {
			t.Fatalf("全量菜单树失败: %v", err)
		}
		for _, id := range []int64{pcDir.ID, pcOff.ID, mobile.ID} {
			if findMenu(all, id) == nil {
				t.Errorf("include_inactive=true&platform=all 应包含菜单 %d", id)
			}
		}
	})
}

// TestMenuService_AssignRoles 菜单→角色覆盖式写入，并重建 Casbin p 策略。
func TestMenuService_AssignRoles(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cs := withCasbin(t, tx)
		menus := svc.NewMenuService()

		role1 := mustCreateRole(t, ctx, uniq("zzq_menu_role_1"), "all")
		role2 := mustCreateRole(t, ctx, uniq("zzq_menu_role_2"), "all")

		m, err := menus.Create(ctx, &request.CreateMenuReq{
			Name: "ZZQ-菜单角色授权", Type: "menu", Platform: "pc", Sequence: 210,
		})
		if err != nil {
			t.Fatalf("创建菜单失败: %v", err)
		}
		api, err := menus.CreateAPI(ctx, m.ID, &request.CreateMenuAPIReq{
			Method: "GET", Path: "/web/zzq/menu-role", Description: "菜单角色授权测试接口",
		})
		if err != nil {
			t.Fatalf("创建菜单接口失败: %v", err)
		}

		// 授权 role1：关系落库 + p 策略生效。
		if err := menus.AssignRoles(ctx, m.ID, []int64{role1.ID}); err != nil {
			t.Fatalf("授权角色失败: %v", err)
		}
		if n := countRows(t, tx, "ac_role_menu_rel", "menu_id = ? AND role_id = ?", m.ID, role1.ID); n != 1 {
			t.Errorf("role1 菜单关系条数 = %d, 期望 1", n)
		}
		if n := casbinPolicyCount(t, tx, "p", role1.Code, api.Path); n != 1 {
			t.Errorf("role1 p 策略条数 = %d, 期望 1", n)
		}
		ok, err := cs.Enforce(role1.Code, api.Path, "GET")
		if err != nil {
			t.Fatalf("Enforce 失败: %v", err)
		}
		if !ok {
			t.Error("role1 应被允许访问菜单接口")
		}

		// GET 返回已授权角色的 id/code/name。
		roles, err := menus.GetRoles(ctx, m.ID)
		if err != nil {
			t.Fatalf("查询菜单角色失败: %v", err)
		}
		if len(roles) != 1 || roles[0].ID != role1.ID || roles[0].Code != role1.Code || roles[0].Name != role1.Name {
			t.Errorf("菜单角色列表不符: %+v", roles)
		}

		// 覆盖为 role2：role1 关系与策略均应移除。
		if err := menus.AssignRoles(ctx, m.ID, []int64{role2.ID}); err != nil {
			t.Fatalf("覆盖授权失败: %v", err)
		}
		if n := countRows(t, tx, "ac_role_menu_rel", "menu_id = ? AND role_id = ?", m.ID, role1.ID); n != 0 {
			t.Errorf("role1 关系应被覆盖, got %d", n)
		}
		if n := countRows(t, tx, "ac_role_menu_rel", "menu_id = ? AND role_id = ?", m.ID, role2.ID); n != 1 {
			t.Errorf("role2 关系条数 = %d, 期望 1", n)
		}
		if n := casbinPolicyCount(t, tx, "p", role1.Code, api.Path); n != 0 {
			t.Errorf("role1 p 策略应被清理, got %d", n)
		}
		if n := casbinPolicyCount(t, tx, "p", role2.Code, api.Path); n != 1 {
			t.Errorf("role2 p 策略条数 = %d, 期望 1", n)
		}

		// 空列表表示清空授权。
		if err := menus.AssignRoles(ctx, m.ID, nil); err != nil {
			t.Fatalf("清空授权失败: %v", err)
		}
		if n := countRows(t, tx, "ac_role_menu_rel", "menu_id = ?", m.ID); n != 0 {
			t.Errorf("清空后关系条数 = %d, 期望 0", n)
		}
		if n := casbinPolicyCount(t, tx, "p", role2.Code, api.Path); n != 0 {
			t.Errorf("清空后 role2 p 策略应被清理, got %d", n)
		}

		// 角色 / 菜单不存在时拒绝。
		if err := menus.AssignRoles(ctx, m.ID, []int64{999999999}); err == nil {
			t.Error("角色不存在时应返回错误")
		}
		if err := menus.AssignRoles(ctx, 999999999, []int64{role2.ID}); err == nil {
			t.Error("菜单不存在时应返回错误")
		}
	})
}

// TestConfigService_Create_ActiveSchema 创建参数时正确写入 active/schema，并归一类型别名。
func TestConfigService_Create_ActiveSchema(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cfgs := svc.NewConfigService()

		key := uniq("zzq.cfg.schema")
		schema := `{"type":"integer","minimum":0,"maximum":100}`
		got, err := cfgs.Create(ctx, &request.CreateConfigReq{
			Name: "ZZQ-Schema 参数", Key: key, Value: "10",
			DataType: "integer", Schema: schema, Active: boolPtr(false),
		})
		if err != nil {
			t.Fatalf("创建参数失败: %v", err)
		}
		if got.Active == nil || *got.Active {
			t.Errorf("active = %v, 期望 false", got.Active)
		}
		if got.Schema != schema {
			t.Errorf("schema = %q, 期望 %q", got.Schema, schema)
		}
		if got.DataType != "int" {
			t.Errorf("data_type = %q, 期望别名归一为 int", got.DataType)
		}

		// 数据库侧确认 active / schema 均落库。
		if activeFlag(t, tx, "ac_system_config", got.ID) {
			t.Error("数据库中 active 应为 false")
		}
		if v := sqlText(t, tx, "ac_system_config", "schema", got.ID); v != schema {
			t.Errorf("数据库 schema = %q, 期望 %q", v, schema)
		}

		// boolean 别名归一为 bool；active 缺省 true。
		def, err := cfgs.Create(ctx, &request.CreateConfigReq{
			Name: "ZZQ-Bool 参数", Key: uniq("zzq.cfg.bool"), DataType: "boolean", Value: "true",
		})
		if err != nil {
			t.Fatalf("创建 bool 参数失败: %v", err)
		}
		if def.DataType != "bool" {
			t.Errorf("data_type = %q, 期望 bool", def.DataType)
		}
		if def.Active == nil || !*def.Active {
			t.Errorf("active 缺省应为 true, got %v", def.Active)
		}
	})
}

// TestConfigService_Update_ActiveSchemaDataType 更新参数：
// active 仅在显式传入时修改；schema/data_type 正确更新；类型别名归一。
func TestConfigService_Update_ActiveSchemaDataType(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cfgs := svc.NewConfigService()

		key := uniq("zzq.cfg.updschema")
		created, err := cfgs.Create(ctx, &request.CreateConfigReq{
			Name: "ZZQ-更新参数", Key: key, Value: "1", DataType: "int",
			Schema: `{"type":"integer"}`, Active: boolPtr(true),
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		newSchema := `{"type":"string","maxLength":32}`
		if err := cfgs.Update(ctx, created.ID, &request.UpdateConfigReq{
			Name: "ZZQ-更新参数-新", DataType: "boolean", Schema: newSchema, Active: boolPtr(false),
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		var got struct {
			DataType string
			Schema   string
		}
		if err := tx.Table("ac_system_config").Select("data_type, schema").
			Where("id = ?", created.ID).Scan(&got).Error; err != nil {
			t.Fatalf("回读参数失败: %v", err)
		}
		if got.DataType != "bool" {
			t.Errorf("data_type = %q, 期望 bool", got.DataType)
		}
		if got.Schema != newSchema {
			t.Errorf("schema = %q, 期望 %q", got.Schema, newSchema)
		}
		if activeFlag(t, tx, "ac_system_config", created.ID) {
			t.Error("active 应被更新为 false")
		}

		// 未传 active / data_type / schema 时保持原值。
		if err := cfgs.Update(ctx, created.ID, &request.UpdateConfigReq{Name: "ZZQ-仅改名"}); err != nil {
			t.Fatalf("仅改名失败: %v", err)
		}
		if activeFlag(t, tx, "ac_system_config", created.ID) {
			t.Error("未传 active 时不应改变原值 false")
		}
		if v := sqlText(t, tx, "ac_system_config", "data_type", created.ID); v != "bool" {
			t.Errorf("未传 data_type 时应保持 bool, got %q", v)
		}
		if v := sqlText(t, tx, "ac_system_config", "schema", created.ID); v != newSchema {
			t.Errorf("未传 schema 时应保持原值, got %q", v)
		}

		// integer 别名更新为 int。
		if err := cfgs.Update(ctx, created.ID, &request.UpdateConfigReq{DataType: "integer"}); err != nil {
			t.Fatalf("更新 data_type 失败: %v", err)
		}
		if v := sqlText(t, tx, "ac_system_config", "data_type", created.ID); v != "int" {
			t.Errorf("data_type = %q, 期望 int", v)
		}
	})
}

// TestConfigService_InvalidDataType 非法数据类型在创建/更新时均返回错误。
func TestConfigService_InvalidDataType(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cfgs := svc.NewConfigService()

		if _, err := cfgs.Create(ctx, &request.CreateConfigReq{
			Name: "ZZQ-非法类型", Key: uniq("zzq.cfg.bad"), DataType: "date",
		}); err == nil {
			t.Error("非法 data_type 创建应返回错误")
		}

		created, err := cfgs.Create(ctx, &request.CreateConfigReq{
			Name: "ZZQ-合法类型", Key: uniq("zzq.cfg.ok"), DataType: "string",
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if err := cfgs.Update(ctx, created.ID, &request.UpdateConfigReq{DataType: "object"}); err == nil {
			t.Error("非法 data_type 更新应返回错误")
		}
		// 失败更新不得改写原值。
		if v := sqlText(t, tx, "ac_system_config", "data_type", created.ID); v != "string" {
			t.Errorf("失败更新后 data_type = %q, 期望 string", v)
		}
	})
}

// TestMenuService_ParentTypeRules 菜单父级类型规则：目录→目录、菜单→目录、按钮→菜单；
// 根节点仍允许任意类型（兼容历史顶级菜单）。创建与更新都校验。
func TestMenuService_ParentTypeRules(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		menus := svc.NewMenuService()

		rootDir, err := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-规则-根目录", Type: "directory", Platform: "pc", Sequence: 220})
		if err != nil {
			t.Fatalf("创建根目录失败: %v", err)
		}
		rootMenu, err := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-规则-根菜单", Type: "menu", Platform: "pc", Sequence: 221})
		if err != nil {
			t.Fatalf("创建根菜单失败: %v", err)
		}
		rootButton, err := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-规则-根按钮", Type: "button", Platform: "pc", Sequence: 222})
		if err != nil {
			t.Fatalf("历史顶级按钮应可创建: %v", err)
		}

		// 合法挂载。
		dirSub, err := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-规则-子目录", ParentID: &rootDir.ID, Type: "directory", Platform: "pc", Sequence: 223})
		if err != nil {
			t.Fatalf("目录挂目录应允许: %v", err)
		}
		menuSub, err := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-规则-子菜单", ParentID: &rootDir.ID, Type: "menu", Platform: "pc", Sequence: 224})
		if err != nil {
			t.Fatalf("菜单挂目录应允许: %v", err)
		}
		btnSub, err := menus.Create(ctx, &request.CreateMenuReq{Name: "ZZQ-规则-子按钮", ParentID: &rootMenu.ID, Type: "button", Platform: "pc", Sequence: 225})
		if err != nil {
			t.Fatalf("按钮挂菜单应允许: %v", err)
		}

		// 非法创建。
		invalid := []struct {
			name    string
			req     *request.CreateMenuReq
			wantErr bool
		}{
			{"目录挂菜单", &request.CreateMenuReq{Name: "ZZQ-非法1", ParentID: &rootMenu.ID, Type: "directory", Platform: "pc"}, true},
			{"目录挂按钮", &request.CreateMenuReq{Name: "ZZQ-非法2", ParentID: &rootButton.ID, Type: "directory", Platform: "pc"}, true},
			{"菜单挂菜单", &request.CreateMenuReq{Name: "ZZQ-非法3", ParentID: &rootMenu.ID, Type: "menu", Platform: "pc"}, true},
			{"菜单挂按钮", &request.CreateMenuReq{Name: "ZZQ-非法4", ParentID: &rootButton.ID, Type: "menu", Platform: "pc"}, true},
			{"按钮挂目录", &request.CreateMenuReq{Name: "ZZQ-非法5", ParentID: &rootDir.ID, Type: "button", Platform: "pc"}, true},
			{"按钮挂按钮", &request.CreateMenuReq{Name: "ZZQ-非法6", ParentID: &rootButton.ID, Type: "button", Platform: "pc"}, true},
		}
		for _, tc := range invalid {
			if _, err := menus.Create(ctx, tc.req); (err != nil) != tc.wantErr {
				t.Errorf("%s: err=%v, wantErr=%v", tc.name, err, tc.wantErr)
			}
		}

		// 非法更新（把节点移动到不合法父级）。
		if err := menus.Update(ctx, menuSub.ID, &request.UpdateMenuReq{Name: menuSub.Name, ParentID: &rootButton.ID}); err == nil {
			t.Error("菜单移动到按钮下应失败")
		}
		if err := menus.Update(ctx, dirSub.ID, &request.UpdateMenuReq{Name: dirSub.Name, ParentID: &rootMenu.ID}); err == nil {
			t.Error("目录移动到菜单下应失败")
		}
		if err := menus.Update(ctx, btnSub.ID, &request.UpdateMenuReq{Name: btnSub.Name, ParentID: &rootDir.ID}); err == nil {
			t.Error("按钮移动到目录下应失败")
		}

		// 仅改类型也校验现有父级：菜单改为按钮，但父级仍是目录 → 失败。
		if err := menus.Update(ctx, menuSub.ID, &request.UpdateMenuReq{Type: "button"}); err == nil {
			t.Error("菜单在目录下改为按钮应失败")
		}
		// 同时改类型并移动到合法父级 → 成功。
		if err := menus.Update(ctx, menuSub.ID, &request.UpdateMenuReq{Type: "button", ParentID: &rootMenu.ID}); err != nil {
			t.Errorf("按钮移动到菜单下应成功: %v", err)
		}
		moved, err := menus.Get(ctx, menuSub.ID)
		if err != nil {
			t.Fatalf("回读菜单失败: %v", err)
		}
		if moved.Type != "button" || moved.ParentID == nil || *moved.ParentID != rootMenu.ID || moved.Level != 2 {
			t.Errorf("类型/父级/层级不符: type=%s parent=%v level=%d", moved.Type, moved.ParentID, moved.Level)
		}
	})
}
