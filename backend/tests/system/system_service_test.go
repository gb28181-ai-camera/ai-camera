package system

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	svc "github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 用户管理（UserService）与岗位管理（PostService）
// ════════════════════════════════════════════════════════════════

// ── UserService ───────────────────────────────────────────────

// TestUserService_Create 创建用户：全字段落库 + 角色关联。
func TestUserService_Create(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		users := svc.NewUserService()

		post, err := svc.NewPostService().Create(ctx, &request.CreatePostReq{Name: "ZZQ-岗位", Code: uniq("ZZQ_POST")})
		if err != nil {
			t.Fatalf("创建岗位失败: %v", err)
		}
		role := mustCreateRole(t, ctx, uniq("zzq_user_role"), "all")
		username := uniq("zzq_create_user")

		u, err := users.Create(ctx, &request.CreateUserReq{
			Name: "张三", Username: username, Password: "P@ss123456",
			Email: "zs@example.com", Phone: "13800138000", Avatar: "/uploads/avatar/1.png",
			DeptID: int64Ptr(testDeptID), PostID: &post.ID, Active: boolPtr(true),
			IsAdmin: boolPtr(false), Remark: "新入职", RoleIDs: []int64{role.ID},
		})
		if err != nil {
			t.Fatalf("创建用户失败: %v", err)
		}

		got, err := users.Get(ctx, u.ID)
		if err != nil {
			t.Fatalf("回读用户失败: %v", err)
		}
		if got.Name != "张三" || got.Username != username || got.Email != "zs@example.com" ||
			got.Phone != "13800138000" || got.Avatar != "/uploads/avatar/1.png" || got.Remark != "新入职" {
			t.Errorf("字段不一致: %+v", got)
		}
		if got.DeptID == nil || *got.DeptID != testDeptID {
			t.Errorf("dept_id = %v", got.DeptID)
		}
		if got.PostID == nil || *got.PostID != post.ID {
			t.Errorf("post_id = %v", got.PostID)
		}
		if got.Active == nil || !*got.Active {
			t.Errorf("active = %v, 期望 true", got.Active)
		}
		if got.IsAdmin == nil || *got.IsAdmin {
			t.Errorf("is_admin = %v, 期望 false", got.IsAdmin)
		}
		if got.CreatedBy != testUserName || got.CreatedDT.IsZero() {
			t.Errorf("审计字段缺失: created_by=%q created_dt=%v", got.CreatedBy, got.CreatedDT)
		}
		if len(got.Roles) != 1 || got.Roles[0].ID != role.ID {
			t.Errorf("Roles 关联未加载: %+v", got.Roles)
		}
		if n := countRows(t, tx, "ac_user_role_rel", "user_id = ?", u.ID); n != 1 {
			t.Errorf("ac_user_role_rel 条数 = %d, 期望 1", n)
		}
		if n := countRows(t, tx, "ac_user_post_rel", "user_id = ?", u.ID); n != 1 {
			t.Errorf("ac_user_post_rel 条数 = %d, 期望 1", n)
		}
	})
}

// TestUserService_Create_DuplicateUsername 登录名唯一。
func TestUserService_Create_DuplicateUsername(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		users := svc.NewUserService()

		username := uniq("zzq_dup_user")
		if _, err := users.Create(ctx, &request.CreateUserReq{Name: "ZZQ-1", Username: username, Password: "P@ss123456"}); err != nil {
			t.Fatalf("首次创建失败: %v", err)
		}
		_, err := users.Create(ctx, &request.CreateUserReq{Name: "ZZQ-2", Username: username, Password: "P@ss123456"})
		if err == nil {
			t.Fatal("重复登录名应返回错误")
		}
		if !strings.Contains(err.Error(), "已存在") {
			t.Errorf("错误信息应含唯一约束提示: %v", err)
		}
		if n := countRows(t, tx, "ac_user", "user_name = ?", username); n != 1 {
			t.Errorf("同登录名用户数 = %d, 期望 1", n)
		}
	})
}

// TestUserService_Create_PasswordHashed 密码以 bcrypt 摘要存储。
func TestUserService_Create_PasswordHashed(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		const plain = "abc@2026"

		u := mustCreateUser(t, ctx, uniq("zzq_pwd_user"), &request.CreateUserReq{Password: plain})
		hash := sqlText(t, tx, "ac_user", "password_hash", u.ID)
		if hash == plain {
			t.Fatal("password_hash 不得等于明文")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
			t.Errorf("bcrypt 校验应通过: %v", err)
		}
		// password_hash 的 JSON tag 为 -，序列化后不得出现在响应中
		raw, err := json.Marshal(u)
		if err != nil {
			t.Fatalf("序列化用户失败: %v", err)
		}
		if strings.Contains(string(raw), "password_hash") || strings.Contains(string(raw), hash) {
			t.Errorf("序列化结果不应包含密码摘要: %s", string(raw))
		}
	})
}

// TestUserService_Page_PreloadName 列表 Preload 部门/岗位名称而非仅 ID。
func TestUserService_Page_PreloadName(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		users := svc.NewUserService()

		dept, err := svc.NewDeptService().Create(ctx, &request.CreateDeptReq{Name: "ZZQ-部门", Code: uniq("ZZQ_DEPT")})
		if err != nil {
			t.Fatalf("创建部门失败: %v", err)
		}
		post, err := svc.NewPostService().Create(ctx, &request.CreatePostReq{Name: "ZZQ-岗位", Code: uniq("ZZQ_POST")})
		if err != nil {
			t.Fatalf("创建岗位失败: %v", err)
		}
		keyword := uniq("zzq_preload")
		mustCreateUser(t, ctx, keyword, &request.CreateUserReq{DeptID: &dept.ID, PostID: &post.ID})

		resp, err := users.Page(ctx, &request.PageUserReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: keyword}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 1 || len(resp.Items) != 1 {
			t.Fatalf("total=%d items=%d", resp.Total, len(resp.Items))
		}
		item := resp.Items[0]
		if item.Dept == nil || item.Dept.Name != "ZZQ-部门" {
			t.Errorf("Dept.Name 未回显: %+v", item.Dept)
		}
		if item.Post == nil || item.Post.Name != "ZZQ-岗位" {
			t.Errorf("Post.Name 未回显: %+v", item.Post)
		}
		if resp.Page != 1 || resp.PageSize != 20 {
			t.Errorf("分页回显 page=%d page_size=%d", resp.Page, resp.PageSize)
		}
	})
}

// TestUserService_Page_Filters 各过滤条件命中数正确，组合为 AND。
func TestUserService_Page_Filters(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		users := svc.NewUserService()

		keyword := uniq("zzq_filter")
		post, _ := svc.NewPostService().Create(ctx, &request.CreatePostReq{Name: "ZZQ-岗位", Code: uniq("ZZQ_POST")})
		mustCreateUser(t, ctx, keyword+"_a", &request.CreateUserReq{Phone: "13800001111", PostID: &post.ID, Active: boolPtr(true)})
		mustCreateUser(t, ctx, keyword+"_b", &request.CreateUserReq{Phone: "13800002222", Active: boolPtr(false)})

		byKeyword, err := users.Page(ctx, &request.PageUserReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: keyword}, dsAll())
		if err != nil {
			t.Fatalf("按关键字分页失败: %v", err)
		}
		if byKeyword.Total != 2 {
			t.Errorf("关键字命中 %d, 期望 2", byKeyword.Total)
		}

		active := true
		byActive, _ := users.Page(ctx, &request.PageUserReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: keyword, Active: &active}, dsAll())
		if byActive.Total != 1 {
			t.Errorf("active 命中 %d, 期望 1", byActive.Total)
		}

		byPost, _ := users.Page(ctx, &request.PageUserReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, PostID: &post.ID}, dsAll())
		if byPost.Total != 1 {
			t.Errorf("post_id 命中 %d, 期望 1", byPost.Total)
		}

		// 手机号模糊匹配
		byPhone, _ := users.Page(ctx, &request.PageUserReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: "13800002222"}, dsAll())
		if byPhone.Total != 1 {
			t.Errorf("phone 命中 %d, 期望 1", byPhone.Total)
		}

		byDept, _ := users.Page(ctx, &request.PageUserReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: keyword, DeptID: int64Ptr(testDeptID)}, dsAll())
		if byDept.Total != 2 {
			t.Errorf("dept_id 命中 %d, 期望 2", byDept.Total)
		}

		// 组合条件为 AND
		byBoth, _ := users.Page(ctx, &request.PageUserReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: keyword, PostID: &post.ID}, dsAll())
		if byBoth.Total != 1 {
			t.Errorf("组合条件命中 %d, 期望 1（AND）", byBoth.Total)
		}

		none, _ := users.Page(ctx, &request.PageUserReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: uniq("zzq_none")}, dsAll())
		if none.Total != 0 || len(none.Items) != 0 {
			t.Errorf("无匹配时 total=%d items=%d", none.Total, len(none.Items))
		}
	})
}

// TestUserService_Page_DataScope 用户列表数据权限：可见行数随权限收敛。
func TestUserService_Page_DataScope(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		users := svc.NewUserService()
		depts := svc.NewDeptService()

		// ac_user.dept_id 带外键约束，必须使用真实部门
		var deptIDs []int64
		for i := 0; i < 3; i++ {
			d, err := depts.Create(ctx, &request.CreateDeptReq{Name: "ZZQ-权限部门", Code: uniq("ZZQ_DSDEPT")})
			if err != nil {
				t.Fatalf("创建部门失败: %v", err)
			}
			deptIDs = append(deptIDs, d.ID)
		}
		deptA, deptB, deptC := deptIDs[0], deptIDs[1], deptIDs[2]

		seed := []struct {
			dept    int64
			creator string
			suffix  string
		}{
			{deptA, "ds_user_a", "A1"}, {deptA, "ds_user_b", "A2"},
			{deptB, "ds_user_b", "B1"}, {deptB, "ds_user_b", "B2"},
			{deptC, "ds_user_c", "C1"}, {deptC, "ds_user_c", "C2"},
		}
		for _, s := range seed {
			m := &model.User{
				Name: "ZZQ-DS-" + s.suffix, Username: uniq("zzq_ds_" + s.suffix),
				PasswordHash: "x", Active: boolPtr(true), IsAdmin: boolPtr(false),
				DeptID:    int64Ptr(s.dept),
				BaseModel: model.BaseModel{CreatedBy: s.creator, UpdatedBy: s.creator},
			}
			if err := tx.Create(m).Error; err != nil {
				t.Fatalf("预置数据失败: %v", err)
			}
		}
		q := request.PageUserReq{PageReq: request.PageReq{Page: 1, PageSize: 200}}
		cross, err := users.Page(ctx, &q, dsPerm("cross_dept", "ds_user_b", &deptA, fmt.Sprintf("%d", deptB)))
		if err != nil {
			t.Fatalf("cross_dept 分页失败: %v", err)
		}
		dept, _ := users.Page(ctx, &q, dsPerm("dept", "ds_user_b", &deptA, ""))
		personal, _ := users.Page(ctx, &q, dsPerm("personal", "ds_user_a", &deptA, ""))
		all, _ := users.Page(ctx, &q, dsAll())

		if cross.Total != 4 {
			t.Errorf("cross_dept total = %d, 期望 4", cross.Total)
		}
		if dept.Total != 2 {
			t.Errorf("dept total = %d, 期望 2", dept.Total)
		}
		if personal.Total != 1 {
			t.Errorf("personal total = %d, 期望 1", personal.Total)
		}
		if personal.Total > dept.Total || dept.Total > cross.Total || cross.Total > all.Total {
			t.Errorf("可见行数应满足 personal <= dept <= cross_dept <= all: %d/%d/%d/%d",
				personal.Total, dept.Total, cross.Total, all.Total)
		}
	})
}

// TestUserService_Options 用户下拉选项只返回启用用户。
func TestUserService_Options(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		users := svc.NewUserService()

		on := mustCreateUser(t, ctx, uniq("zzq_opt_on"), &request.CreateUserReq{Active: boolPtr(true)})
		off := mustCreateUser(t, ctx, uniq("zzq_opt_off"), &request.CreateUserReq{Active: boolPtr(false)})

		items, err := users.Options(ctx)
		if err != nil {
			t.Fatalf("查询选项失败: %v", err)
		}
		seen := map[int64]bool{}
		for _, it := range items {
			seen[it.ID] = true
			if it.ID <= 0 || it.Name == "" {
				t.Errorf("选项字段不完整: %+v", it)
			}
		}
		if !seen[on.ID] {
			t.Error("启用用户应出现在选项中")
		}
		if seen[off.ID] {
			t.Error("停用用户不应出现在选项中")
		}
	})
}

// TestUserService_Get 用户详情逐字段核对且 Roles 关联已加载。
func TestUserService_Get(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		users := svc.NewUserService()

		role := mustCreateRole(t, ctx, uniq("zzq_get_role"), "all")
		name := uniq("zzq_get_user")
		u := mustCreateUser(t, ctx, name, &request.CreateUserReq{
			Email: "a@b.com", Phone: "13700000000", DeptID: int64Ptr(testDeptID),
			Remark: "r", RoleIDs: []int64{role.ID},
		})

		got, err := users.Get(ctx, u.ID)
		if err != nil {
			t.Fatalf("获取失败: %v", err)
		}
		if got.ID != u.ID || got.Username != name || got.Email != "a@b.com" ||
			got.Phone != "13700000000" || got.Remark != "r" {
			t.Errorf("字段不一致: %+v", got)
		}
		if got.DeptID == nil || *got.DeptID != testDeptID {
			t.Errorf("dept_id = %v", got.DeptID)
		}
		if got.Active == nil || !*got.Active || got.IsAdmin == nil || *got.IsAdmin {
			t.Errorf("active/is_admin 异常: %v/%v", got.Active, got.IsAdmin)
		}
		if len(got.Roles) != 1 {
			t.Errorf("Roles 应为 1 条, got %d", len(got.Roles))
		}
		if _, err := users.Get(ctx, 999999999); err == nil {
			t.Error("不存在的用户应返回错误")
		}
	})
}

// TestUserService_Update 用户更新：可改字段全部生效，登录名保持不可变。
func TestUserService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		users := svc.NewUserService()

		post, _ := svc.NewPostService().Create(ctx, &request.CreatePostReq{Name: "ZZQ-岗位", Code: uniq("ZZQ_POST")})
		name := uniq("zzq_upd_user")
		u := mustCreateUser(t, ctx, name, &request.CreateUserReq{DeptID: int64Ptr(testDeptID)})

		if err := users.Update(ctx, u.ID, &request.UpdateUserReq{
			Name: "李四", Username: "should_be_ignored", Email: "ls@x.com", Phone: "13900000009",
			Avatar: "/a.png", DeptID: int64Ptr(testDeptID), PostID: &post.ID,
			Active: boolPtr(false), IsAdmin: boolPtr(true), Remark: "已更新",
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		got, _ := users.Get(ctx, u.ID)
		if got.Name != "李四" || got.Email != "ls@x.com" || got.Phone != "13900000009" ||
			got.Avatar != "/a.png" || got.Remark != "已更新" {
			t.Errorf("字段未全部更新: %+v", got)
		}
		if got.Username != name {
			t.Errorf("登录名不可修改: %q -> %q", name, got.Username)
		}
		if got.PostID == nil || *got.PostID != post.ID {
			t.Errorf("post_id = %v", got.PostID)
		}
		if got.Active == nil || *got.Active {
			t.Errorf("active = %v, 期望 false", got.Active)
		}
		if got.IsAdmin == nil || !*got.IsAdmin {
			t.Errorf("is_admin = %v, 期望 true", got.IsAdmin)
		}
		if got.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q, 期望 %q", got.UpdatedBy, testUserName)
		}
	})
}

// TestUserService_SetActive 启用/禁用切换并影响登录。
func TestUserService_SetActive(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		users := svc.NewUserService()
		login := svc.NewAuthService()

		name := uniq("zzq_setactive")
		u := mustCreateUser(t, ctx, name, &request.CreateUserReq{Password: "P@ss123456", Active: boolPtr(true)})

		if err := users.SetActive(ctx, u.ID, false); err != nil {
			t.Fatalf("禁用失败: %v", err)
		}
		if activeFlag(t, tx, "ac_user", u.ID) {
			t.Error("禁用后 active 应为 false")
		}
		if _, err := login.Login(ctx, name, "P@ss123456"); err == nil {
			t.Error("禁用后不应可登录")
		}
		if got := sqlText(t, tx, "ac_user", "updated_by", u.ID); got != testUserName {
			t.Errorf("updated_by = %q", got)
		}

		if err := users.SetActive(ctx, u.ID, true); err != nil {
			t.Fatalf("启用失败: %v", err)
		}
		if !activeFlag(t, tx, "ac_user", u.ID) {
			t.Error("启用后 active 应为 true")
		}
		if _, err := login.Login(ctx, name, "P@ss123456"); err != nil {
			t.Errorf("启用后应可登录: %v", err)
		}

		if err := users.SetActive(ctx, 999999999, false); err == nil {
			t.Error("不存在的用户应返回错误")
		}
	})
}

// TestUserService_ResetPassword 重置密码后旧密码失效、新密码生效。
func TestUserService_ResetPassword(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		users := svc.NewUserService()
		login := svc.NewAuthService()

		name := uniq("zzq_reset")
		u := mustCreateUser(t, ctx, name, &request.CreateUserReq{Password: "old@2026"})
		oldHash := sqlText(t, tx, "ac_user", "password_hash", u.ID)

		const newPwd = "newPass@2026"
		if err := users.ResetPassword(ctx, u.ID, newPwd); err != nil {
			t.Fatalf("重置密码失败: %v", err)
		}
		newHash := sqlText(t, tx, "ac_user", "password_hash", u.ID)
		if newHash == oldHash {
			t.Fatal("password_hash 应变化")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(newHash), []byte(newPwd)); err != nil {
			t.Errorf("新密码应校验通过: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(newHash), []byte("old@2026")); err == nil {
			t.Error("旧密码不应再通过")
		}
		if _, err := login.Login(ctx, name, "old@2026"); err == nil {
			t.Error("旧密码不应可登录")
		}
		if _, err := login.Login(ctx, name, newPwd); err != nil {
			t.Errorf("新密码应可登录: %v", err)
		}
		if err := users.ResetPassword(ctx, 999999999, newPwd); err == nil {
			t.Error("不存在的用户应返回错误")
		}
	})
}

// TestUserService_Delete 用户软删除并清理角色/岗位关联。
func TestUserService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		users := svc.NewUserService()

		role := mustCreateRole(t, ctx, uniq("zzq_del_role"), "all")
		post, _ := svc.NewPostService().Create(ctx, &request.CreatePostReq{Name: "ZZQ-岗位", Code: uniq("ZZQ_POST")})
		u := mustCreateUser(t, ctx, uniq("zzq_del_user"), &request.CreateUserReq{RoleIDs: []int64{role.ID}, PostID: &post.ID})

		if err := users.Delete(ctx, u.ID); err != nil {
			t.Fatalf("删除用户失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_user", u.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		if _, err := users.Get(ctx, u.ID); err == nil {
			t.Error("软删除后 Get 应返回错误")
		}
		if n := countRows(t, tx, "ac_user_role_rel", "user_id = ?", u.ID); n != 0 {
			t.Errorf("角色关联应清理, got %d", n)
		}
		if n := countRows(t, tx, "ac_user_post_rel", "user_id = ?", u.ID); n != 0 {
			t.Errorf("岗位关联应清理, got %d", n)
		}
		if err := users.Delete(ctx, 999999999); err == nil {
			t.Error("不存在的用户应返回错误")
		}
	})
}

// ── PostService ───────────────────────────────────────────────

// TestPostService_Create 创建岗位：全字段落库 + 角色关联。
func TestPostService_Create(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		posts := svc.NewPostService()

		role := mustCreateRole(t, ctx, uniq("zzq_post_role"), "all")
		code := uniq("ZZQ_POST_C")
		got, err := posts.Create(ctx, &request.CreatePostReq{
			Name: "巡检员", Code: code, Sequence: 6, Description: "负责设备巡检",
			Active: boolPtr(true), RoleIDs: []int64{role.ID},
		})
		if err != nil {
			t.Fatalf("创建岗位失败: %v", err)
		}
		if got.Name != "巡检员" || got.Code != code || got.Sequence != 6 || got.Description != "负责设备巡检" {
			t.Errorf("字段不一致: %+v", got)
		}
		if got.Active == nil || !*got.Active || got.CreatedBy != testUserName || got.CreatedDT.IsZero() {
			t.Errorf("默认值/审计字段异常: %+v", got)
		}
		if n := countRows(t, tx, "ac_post_role_rel", "post_id = ?", got.ID); n != 1 {
			t.Errorf("岗位-角色关联条数 = %d, 期望 1", n)
		}

		// 缺省 sequence=10 且不传 active 时默认启用
		def, _ := posts.Create(ctx, &request.CreatePostReq{Name: "ZZQ-默认岗位", Code: uniq("ZZQ_POST_D")})
		if def.Sequence != 10 {
			t.Errorf("默认 sequence = %d, 期望 10", def.Sequence)
		}
		if def.Active == nil || !*def.Active {
			t.Errorf("默认 active = %v, 期望 true", def.Active)
		}
	})
}

// TestPostService_Create_DuplicateCode 岗位编码唯一（g3 主体）。
func TestPostService_Create_DuplicateCode(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		posts := svc.NewPostService()

		code := uniq("ZZQ_POST_DUP")
		if _, err := posts.Create(ctx, &request.CreatePostReq{Name: "ZZQ-岗位1", Code: code}); err != nil {
			t.Fatalf("首次创建失败: %v", err)
		}
		if _, err := posts.Create(ctx, &request.CreatePostReq{Name: "ZZQ-岗位2", Code: code}); err == nil {
			t.Fatal("重复编码应返回错误")
		}
		if n := countRows(t, tx, "ac_post", "code = ?", code); n != 1 {
			t.Errorf("同编码岗位数 = %d, 期望 1", n)
		}
	})
}

// TestPostService_Page 岗位分页与 active 过滤。
func TestPostService_Page(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		posts := svc.NewPostService()

		prefix := uniq("ZZQ_PAGE_POST")
		if _, err := posts.Create(ctx, &request.CreatePostReq{Name: prefix + "-A", Code: prefix + "_A", Description: "d", Active: boolPtr(true)}); err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if _, err := posts.Create(ctx, &request.CreatePostReq{Name: prefix + "-B", Code: prefix + "_B", Active: boolPtr(false)}); err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		resp, err := posts.Page(ctx, &request.PagePostReq{PageReq: request.PageReq{Page: 1, PageSize: 10}, Keyword: prefix})
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 2 || len(resp.Items) != 2 {
			t.Fatalf("total=%d items=%d, 期望 2/2", resp.Total, len(resp.Items))
		}
		if resp.Page != 1 || resp.PageSize != 10 {
			t.Errorf("分页回显 page=%d page_size=%d", resp.Page, resp.PageSize)
		}
		for _, it := range resp.Items {
			if it.Name == "" || it.Code == "" || it.Sequence == 0 || it.Active == nil {
				t.Errorf("items 字段不完整: %+v", it)
			}
		}

		active := true
		byActive, _ := posts.Page(ctx, &request.PagePostReq{PageReq: request.PageReq{Page: 1, PageSize: 10}, Keyword: prefix, Active: &active})
		if byActive.Total != 1 {
			t.Errorf("active=true 命中 %d, 期望 1", byActive.Total)
		}
	})
}

// TestPostService_Options 岗位下拉选项。
func TestPostService_Options(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		posts := svc.NewPostService()

		on, err := posts.Create(ctx, &request.CreatePostReq{Name: "ZZQ-启用岗位", Code: uniq("ZZQ_OPT_POST_ON"), Active: boolPtr(true)})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		off, err := posts.Create(ctx, &request.CreatePostReq{Name: "ZZQ-停用岗位", Code: uniq("ZZQ_OPT_POST_OFF"), Active: boolPtr(false)})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		items, err := posts.Options(ctx)
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
			t.Error("启用岗位应出现在选项中")
		}
		if seen[off.ID] {
			t.Error("停用岗位不应出现在选项中")
		}
	})
}

// TestPostService_Get 岗位详情含关联角色 ID。
func TestPostService_Get(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		posts := svc.NewPostService()

		role := mustCreateRole(t, ctx, uniq("zzq_getpost_role"), "all")
		created, _ := posts.Create(ctx, &request.CreatePostReq{Name: "ZZQ-详情岗位", Code: uniq("ZZQ_GPOST"), RoleIDs: []int64{role.ID}})

		got, err := posts.Get(ctx, created.ID)
		if err != nil {
			t.Fatalf("获取失败: %v", err)
		}
		if got.Name != "ZZQ-详情岗位" || got.Code != created.Code {
			t.Errorf("字段不一致: %+v", got)
		}
		if len(got.RoleIDs) != 1 || got.RoleIDs[0] != role.ID {
			t.Errorf("RoleIDs = %v, 期望 [%d]", got.RoleIDs, role.ID)
		}
		if _, err := posts.Get(ctx, 999999999); err == nil {
			t.Error("不存在的岗位应返回错误")
		}
	})
}

// TestPostService_Update 岗位更新：字段全部生效，角色关联可覆盖。
func TestPostService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		posts := svc.NewPostService()

		r1 := mustCreateRole(t, ctx, uniq("zzq_upd_r1"), "all")
		r2 := mustCreateRole(t, ctx, uniq("zzq_upd_r2"), "all")
		created, _ := posts.Create(ctx, &request.CreatePostReq{Name: "ZZQ-旧岗位", Code: uniq("ZZQ_UPOST"), RoleIDs: []int64{r1.ID}})

		if err := posts.Update(ctx, created.ID, &request.UpdatePostReq{
			Name: "ZZQ-新岗位", Sequence: 15, Description: "新描述", Active: boolPtr(false),
			RoleIDs: []int64{r2.ID},
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		got, _ := posts.Get(ctx, created.ID)
		if got.Name != "ZZQ-新岗位" || got.Sequence != 15 || got.Description != "新描述" {
			t.Errorf("字段未全部更新: %+v", got)
		}
		if got.Active == nil || *got.Active {
			t.Errorf("active = %v, 期望 false", got.Active)
		}
		if got.Code != created.Code {
			t.Errorf("code 不应变化: %q", got.Code)
		}
		if got.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", got.UpdatedBy)
		}
		if len(got.RoleIDs) != 1 || got.RoleIDs[0] != r2.ID {
			t.Errorf("角色关联未覆盖: %v", got.RoleIDs)
		}
	})
}

// TestPostService_Delete 岗位删除：有用户时拒绝，无用户时软删除。
func TestPostService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		posts := svc.NewPostService()

		// 有用户引用 -> 拒绝
		used, _ := posts.Create(ctx, &request.CreatePostReq{Name: "ZZQ-在用岗位", Code: uniq("ZZQ_USED_POST")})
		mustCreateUser(t, ctx, uniq("zzq_post_user"), &request.CreateUserReq{PostID: &used.ID})
		if err := posts.Delete(ctx, used.ID); err == nil {
			t.Fatal("有用户引用的岗位应拒绝删除")
		}
		if dt := softDeletedAt(t, tx, "ac_post", used.ID); dt != nil {
			t.Error("拒绝删除时 deleted_at 应仍为空")
		}

		// 无用户 -> 软删除并清理角色关联
		role := mustCreateRole(t, ctx, uniq("zzq_delpost_role"), "all")
		free, _ := posts.Create(ctx, &request.CreatePostReq{Name: "ZZQ-空闲岗位", Code: uniq("ZZQ_FREE_POST"), RoleIDs: []int64{role.ID}})
		if err := posts.Delete(ctx, free.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_post", free.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		if n := countRows(t, tx, "ac_post_role_rel", "post_id = ?", free.ID); n != 0 {
			t.Errorf("岗位-角色关联应清理, got %d", n)
		}
		resp, _ := posts.Page(ctx, &request.PagePostReq{PageReq: request.PageReq{Page: 1, PageSize: 10}, Keyword: "ZZQ-空闲岗位"})
		if resp.Total != 0 {
			t.Errorf("软删除后不应出现在分页中, got %d", resp.Total)
		}
	})
}

// TestPostService_AssignRoles 岗位-角色关联写入 ac_post_role_rel。
func TestPostService_AssignRoles(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		posts := svc.NewPostService()

		r1 := mustCreateRole(t, ctx, uniq("zzq_ar_r1"), "all")
		r2 := mustCreateRole(t, ctx, uniq("zzq_ar_r2"), "all")
		created, _ := posts.Create(ctx, &request.CreatePostReq{Name: "ZZQ-关联岗位", Code: uniq("ZZQ_ARPOST")})

		if err := posts.Update(ctx, created.ID, &request.UpdatePostReq{
			Name: created.Name, RoleIDs: []int64{r1.ID, r2.ID},
		}); err != nil {
			t.Fatalf("分配角色失败: %v", err)
		}
		if n := countRows(t, tx, "ac_post_role_rel", "post_id = ?", created.ID); n != 2 {
			t.Errorf("关联条数 = %d, 期望 2", n)
		}
		got, _ := posts.Get(ctx, created.ID)
		if len(got.RoleIDs) != 2 {
			t.Errorf("RoleIDs 条数 = %d, 期望 2", len(got.RoleIDs))
		}

		// 传空数组表示清空
		if err := posts.Update(ctx, created.ID, &request.UpdatePostReq{Name: created.Name, RoleIDs: []int64{}}); err != nil {
			t.Fatalf("清空角色失败: %v", err)
		}
		if n := countRows(t, tx, "ac_post_role_rel", "post_id = ?", created.ID); n != 0 {
			t.Errorf("清空后关联条数 = %d, 期望 0", n)
		}
	})
}
