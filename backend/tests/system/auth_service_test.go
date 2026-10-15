package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	svc "github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 认证（AuthService）与个人中心（ProfileService）
// ════════════════════════════════════════════════════════════════

// ── AuthService ───────────────────────────────────────────────

// TestAuthService_Login 登录成功：签发 token 并返回完整登录载荷。
func TestAuthService_Login(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		login := svc.NewAuthService()

		role := mustCreateRole(t, ctx, uniq("zzq_auth_role"), "dept")
		name := uniq("zzq_login")
		u := mustCreateUser(t, ctx, name, &request.CreateUserReq{
			Password: "P@ss123456",
			Email:    "zzq@example.com",
			DeptID:   int64Ptr(testDeptID),
			RoleIDs:  []int64{role.ID},
		})

		data, err := login.Login(ctx, name, "P@ss123456")
		if err != nil {
			t.Fatalf("登录失败: %v", err)
		}
		if data.Token == "" {
			t.Fatal("token 不应为空")
		}
		if data.UserID != u.ID {
			t.Errorf("user_id = %d, 期望 %d", data.UserID, u.ID)
		}
		if data.Name != u.Name {
			t.Errorf("name = %q, 期望 %q", data.Name, u.Name)
		}
		if data.DeptID == nil || *data.DeptID != testDeptID {
			t.Errorf("dept_id = %v, 期望 %d", data.DeptID, testDeptID)
		}
		if data.DataPerm != "dept" {
			t.Errorf("data_perm = %q, 期望 dept（来自角色 data_permission）", data.DataPerm)
		}
		if data.CrossDeptIDs != role.CrossDeptIDs {
			t.Errorf("cross_dept_ids = %q, 期望 %q", data.CrossDeptIDs, role.CrossDeptIDs)
		}
		if data.Permissions == nil {
			t.Error("permissions 应为非 nil 切片")
		}

		// token 可被 ParseToken 解出，且 claims 与登录载荷一致
		claims, err := bootstrap.JWTService().ParseToken(data.Token)
		if err != nil {
			t.Fatalf("解析 token 失败: %v", err)
		}
		if claims.UserID != u.ID || claims.Username != name || claims.DataPerm != "dept" {
			t.Errorf("claims 不一致: %+v", claims)
		}
		if claims.RoleCode != role.Code {
			t.Errorf("claims.role_code = %q, 期望 %q", claims.RoleCode, role.Code)
		}
		if claims.DeptID != testDeptID {
			t.Errorf("claims.dept_id = %d, 期望 %d", claims.DeptID, testDeptID)
		}

		// 种子管理员：permissions 非空（回归：casbin_rule 列名误写会让集合静默为空）
		adminData, err := login.Login(ctx, testUserName, testPassword)
		if err != nil {
			t.Fatalf("管理员登录失败: %v", err)
		}
		if len(adminData.Permissions) == 0 {
			t.Error("超级管理员的 permissions 不应为空")
		}
		if adminData.DataPerm != "all" {
			t.Errorf("管理员 data_perm = %q, 期望 all", adminData.DataPerm)
		}
	})
}

// TestAuthService_Login_WrongPassword 密码错误返回统一模糊提示，不签发 token。
func TestAuthService_Login_WrongPassword(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		login := svc.NewAuthService()

		name := uniq("zzq_wrongpwd")
		mustCreateUser(t, ctx, name, &request.CreateUserReq{Password: "P@ss123456"})

		data, err := login.Login(ctx, name, "wrong-password")
		if err == nil {
			t.Fatal("密码错误应返回错误")
		}
		if data != nil {
			t.Error("登录失败时 LoginData 应为 nil")
		}
		if err.Error() != "用户名或密码错误" {
			t.Errorf("错误信息 = %q, 期望 用户名或密码错误", err.Error())
		}
	})
}

// TestAuthService_Login_UserNotFound 用户不存在的错误信息与密码错误完全一致（防用户名枚举）。
func TestAuthService_Login_UserNotFound(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		login := svc.NewAuthService()

		name := uniq("zzq_ghost")
		_, err := login.Login(ctx, name, testPassword)
		if err == nil {
			t.Fatal("用户不存在应返回错误")
		}
		if err.Error() != "用户名或密码错误" {
			t.Errorf("错误信息 = %q, 期望与密码错误一致（防用户名枚举）", err.Error())
		}
		if strings.Contains(err.Error(), "不存在") {
			t.Errorf("错误信息不应暴露用户是否存在: %v", err)
		}
	})
}

// TestAuthService_Login_Disabled 已禁用用户拒绝登录。
func TestAuthService_Login_Disabled(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		login := svc.NewAuthService()

		name := uniq("zzq_disabled")
		mustCreateUser(t, ctx, name, &request.CreateUserReq{Password: "P@ss123456", Active: boolPtr(false)})

		data, err := login.Login(ctx, name, "P@ss123456")
		if err == nil {
			t.Fatal("已禁用用户应拒绝登录")
		}
		if data != nil {
			t.Error("禁用用户登录不得签发 token")
		}
		if !strings.Contains(err.Error(), "禁用") {
			t.Errorf("错误信息 = %q, 期望提示已被禁用", err.Error())
		}
	})
}

// TestAuthService_Login_UpdateLastLogin 登录成功写回 last_login 与 updated_by。
func TestAuthService_Login_UpdateLastLogin(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		login := svc.NewAuthService()

		name := uniq("zzq_lastlogin")
		u := mustCreateUser(t, ctx, name, &request.CreateUserReq{Password: "P@ss123456"})
		if u.LastLogin != nil {
			t.Fatalf("新建用户的 last_login 应为 nil, got %v", u.LastLogin)
		}

		before := timeNow()
		if _, err := login.Login(ctx, name, "P@ss123456"); err != nil {
			t.Fatalf("登录失败: %v", err)
		}

		var got model.User
		if err := tx.Where("id = ?", u.ID).First(&got).Error; err != nil {
			t.Fatalf("回读用户失败: %v", err)
		}
		if got.LastLogin == nil {
			t.Fatal("登录后 last_login 应非空")
		}
		if got.LastLogin.Before(before.Add(-1 * minute)) {
			t.Errorf("last_login(%v) 应接近当前时间", got.LastLogin)
		}
		if got.UpdatedBy != name {
			t.Errorf("updated_by = %q, 期望登录名 %q", got.UpdatedBy, name)
		}
	})
}

// TestAuthService_Login_PasswordHashed 创建用户时密码以 bcrypt 摘要存储，绝不落明文。
func TestAuthService_Login_PasswordHashed(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		login := svc.NewAuthService()

		const plain = "Abc@2026xyz"
		name := uniq("zzq_hash")
		u := mustCreateUser(t, ctx, name, &request.CreateUserReq{Password: plain})

		hash := sqlText(t, tx, "ac_user", "password_hash", u.ID)
		if hash == "" {
			t.Fatal("password_hash 不应为空")
		}
		if hash == plain {
			t.Fatal("password_hash 不得等于明文密码")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
			t.Errorf("bcrypt 校验明文应通过: %v", err)
		}
		// 摘要与明文均可登录（同一份哈希）
		if _, err := login.Login(ctx, name, plain); err != nil {
			t.Errorf("以明文登录应成功: %v", err)
		}
	})
}

// TestAuthService_Login_DataPerm 有效数据权限取自角色，无角色退化为 personal。
func TestAuthService_Login_DataPerm(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		login := svc.NewAuthService()

		for _, perm := range model.DataScopeOption {
			role := mustCreateRole(t, ctx, uniq("zzq_dp_"+perm), perm)
			if perm == "cross_dept" {
				if err := svc.NewRoleService().Update(ctx, role.ID, &request.UpdateRoleReq{
					Name: role.Name, DataPermission: perm, CrossDeptIDs: "2,3",
				}); err != nil {
					t.Fatalf("设置跨部门失败: %v", err)
				}
				if err := tx.Model(&model.Role{}).Where("id = ?", role.ID).Update("cross_dept_ids", "2,3").Error; err != nil {
					t.Fatalf("设置跨部门失败: %v", err)
				}
			}
			name := uniq("zzq_dp_user_" + perm)
			mustCreateUser(t, ctx, name, &request.CreateUserReq{Password: "P@ss123456", RoleIDs: []int64{role.ID}})

			data, err := login.Login(ctx, name, "P@ss123456")
			if err != nil {
				t.Fatalf("登录失败: %v", err)
			}
			if data.DataPerm != perm {
				t.Errorf("data_perm = %q, 期望 %q", data.DataPerm, perm)
			}
			if perm == "cross_dept" && data.CrossDeptIDs != "2,3" {
				t.Errorf("cross_dept_ids = %q, 期望 2,3", data.CrossDeptIDs)
			}
		}

		// 无角色的用户退化为最严格的 personal
		name := uniq("zzq_dp_none")
		mustCreateUser(t, ctx, name, &request.CreateUserReq{Password: "P@ss123456"})
		data, err := login.Login(ctx, name, "P@ss123456")
		if err != nil {
			t.Fatalf("登录失败: %v", err)
		}
		if data.DataPerm != "personal" {
			t.Errorf("无角色用户 data_perm = %q, 期望 personal", data.DataPerm)
		}
	})
}

// TestUserRepo_GetUserEffectivePermissions 回归测试：casbin_rule 的策略类型列名为 p_type，
// 写成 ptype 会导致权限集合静默返回空。
func TestUserRepo_GetUserEffectivePermissions(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		repo := repository.NewUserRepo()

		perms := repo.GetUserEffectivePermissions(ctx, testUserID)
		if len(perms) == 0 {
			t.Fatal("admin 应能查到非空权限集合（检查 casbin_rule.p_type 列名）")
		}
		want := "GET:/web/basic/regions"
		found := false
		for _, p := range perms {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("权限集合应包含 %q, 实际前 5 项: %v", want, head(perms, 5))
		}
		// 与 casbin_rule 中的 p 策略条数对齐（去重后不超过总数）
		pCount := casbinPolicyCount(t, tx, "p", "super_admin", "")
		if pCount == 0 {
			t.Fatal("super_admin 的 p 策略不应为空")
		}
		if int64(len(perms)) > pCount {
			t.Errorf("权限数 %d 不应超过 p 策略数 %d", len(perms), pCount)
		}

		// 无角色的用户权限集合为空（且非 nil）
		empty := repo.GetUserEffectivePermissions(ctx, 999999999)
		if empty == nil {
			t.Error("无权限时应返回空切片而非 nil")
		}
		if len(empty) != 0 {
			t.Errorf("无角色用户不应有权限, got %v", empty)
		}
	})
}

// ── ProfileService ────────────────────────────────────────────

// TestProfileService_Get 个人资料详情逐字段核对，含部门/岗位名称。
func TestProfileService_Get(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		profile := svc.NewProfileService()

		post, err := svc.NewPostService().Create(ctx, &request.CreatePostReq{Name: "ZZQ-岗位", Code: uniq("ZZQ_POST"), Sequence: 5})
		if err != nil {
			t.Fatalf("创建岗位失败: %v", err)
		}
		name := uniq("zzq_profile")
		u := mustCreateUser(t, ctx, name, &request.CreateUserReq{
			Email: "zs@x.com", Phone: "13800000001", Avatar: "/upload/avatar.png",
			DeptID: int64Ptr(testDeptID), PostID: &post.ID, Remark: "个人备注", IsAdmin: boolPtr(false),
		})

		got, err := profile.Get(ctx, u.ID)
		if err != nil {
			t.Fatalf("获取个人资料失败: %v", err)
		}
		if got.ID != u.ID || got.Username != name || got.Name != u.Name {
			t.Errorf("基础字段不一致: %+v", got)
		}
		if got.Email != "zs@x.com" || got.Phone != "13800000001" || got.Avatar != "/upload/avatar.png" {
			t.Errorf("联系方式字段不一致: %+v", got)
		}
		if got.DeptID == nil || *got.DeptID != testDeptID {
			t.Errorf("dept_id = %v", got.DeptID)
		}
		if got.PostID == nil || *got.PostID != post.ID {
			t.Errorf("post_id = %v", got.PostID)
		}
		if got.Active == nil || !*got.Active {
			t.Errorf("active = %v", got.Active)
		}
		if got.IsAdmin == nil || *got.IsAdmin {
			t.Errorf("is_admin = %v", got.IsAdmin)
		}
		if got.Remark != "个人备注" {
			t.Errorf("remark = %q", got.Remark)
		}
		if got.CreatedDT.IsZero() {
			t.Error("created_dt 不应为零值")
		}
		if got.Dept == nil || got.Dept.Name == "" {
			t.Error("Preload 后 Dept.Name 应非空")
		}
		if got.Post == nil || got.Post.Name != "ZZQ-岗位" {
			t.Errorf("Preload 后 Post.Name = %v", got.Post)
		}
	})
}

// TestProfileService_Update 个人资料更新全字段生效，且不可提权字段不被改动。
func TestProfileService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		profile := svc.NewProfileService()

		name := uniq("zzq_prof_upd")
		u := mustCreateUser(t, ctx, name, &request.CreateUserReq{
			Email: "old@x.com", Phone: "13900000000", Avatar: "/old.png",
			DeptID: int64Ptr(testDeptID), IsAdmin: boolPtr(false), Remark: "旧备注",
		})
		oldHash := sqlText(t, tx, "ac_user", "password_hash", u.ID)

		if err := profile.Update(ctx, u.ID, &request.UpdateProfileReq{
			Name: "张三", Email: "zs@x.com", Phone: "13800000001", Avatar: "/upload/a.png", Remark: "新备注",
		}); err != nil {
			t.Fatalf("更新个人资料失败: %v", err)
		}

		got, err := profile.Get(ctx, u.ID)
		if err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if got.Name != "张三" || got.Email != "zs@x.com" || got.Phone != "13800000001" ||
			got.Avatar != "/upload/a.png" || got.Remark != "新备注" {
			t.Errorf("字段未全部更新: %+v", got)
		}
		if got.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q, 期望 %q", got.UpdatedBy, testUserName)
		}
		if got.Username != name {
			t.Errorf("user_name 不应被改动: %q", got.Username)
		}
		if got.IsAdmin == nil || *got.IsAdmin {
			t.Errorf("is_admin 不应被改动: %v", got.IsAdmin)
		}
		if got.Active == nil || !*got.Active {
			t.Errorf("active 不应被改动: %v", got.Active)
		}
		if got.DeptID == nil || *got.DeptID != testDeptID {
			t.Errorf("dept_id 不应被改动: %v", got.DeptID)
		}
		if sqlText(t, tx, "ac_user", "password_hash", u.ID) != oldHash {
			t.Error("password_hash 不应被个人资料更新改动")
		}
	})
}

// TestProfileService_Update_PrivilegeEscalation UpdateProfileReq 不暴露提权字段，回读必须保持原值。
func TestProfileService_Update_PrivilegeEscalation(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		profile := svc.NewProfileService()

		otherDept := pickID(t, tx, "ac_dept", "")
		name := uniq("zzq_prof_esc")
		u := mustCreateUser(t, ctx, name, &request.CreateUserReq{
			DeptID: int64Ptr(otherDept), IsAdmin: boolPtr(false), Active: boolPtr(true),
		})

		// 即使请求体被夹带额外字段，也被 DTO 结构体忽略（编译期即不可构造）
		if err := profile.Update(ctx, u.ID, &request.UpdateProfileReq{Name: "提权尝试"}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		got, _ := profile.Get(ctx, u.ID)
		if got.IsAdmin == nil || *got.IsAdmin {
			t.Error("is_admin 应保持 false（提权防护）")
		}
		if got.Active == nil || !*got.Active {
			t.Error("active 应保持 true（提权防护）")
		}
		if got.DeptID == nil || *got.DeptID != otherDept {
			t.Errorf("dept_id 应保持原值 %d, got %v", otherDept, got.DeptID)
		}
	})
}

// TestProfileService_ChangePassword 改密后新旧密码的生效性翻转。
func TestProfileService_ChangePassword(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		profile := svc.NewProfileService()
		login := svc.NewAuthService()

		name := uniq("zzq_chpwd")
		u := mustCreateUser(t, ctx, name, &request.CreateUserReq{Password: "old@2026"})
		oldHash := sqlText(t, tx, "ac_user", "password_hash", u.ID)

		const newPwd = "abc@2026"
		if err := profile.ChangePassword(ctx, u.ID, &request.ChangePasswordReq{OldPassword: "old@2026", NewPassword: newPwd}); err != nil {
			t.Fatalf("修改密码失败: %v", err)
		}

		newHash := sqlText(t, tx, "ac_user", "password_hash", u.ID)
		if newHash == oldHash {
			t.Fatal("password_hash 应随改密而变化")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(newHash), []byte(newPwd)); err != nil {
			t.Errorf("新密码应校验通过: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(newHash), []byte("old@2026")); err == nil {
			t.Error("旧密码不应再校验通过")
		}
		// updated_by 需在登录用例之前断言：登录会刷新 last_login/updated_by
		if got := sqlText(t, tx, "ac_user", "updated_by", u.ID); got != testUserName {
			t.Errorf("updated_by = %q, 期望 %q", got, testUserName)
		}
		if _, err := login.Login(ctx, name, newPwd); err != nil {
			t.Errorf("新密码应可登录: %v", err)
		}
		if _, err := login.Login(ctx, name, "old@2026"); err == nil {
			t.Error("旧密码不应可登录")
		}
	})
}

// TestProfileService_ChangePassword_WrongOld 原密码错误时拒绝且密码哈希不变。
func TestProfileService_ChangePassword_WrongOld(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		profile := svc.NewProfileService()

		name := uniq("zzq_chpwd_wrong")
		u := mustCreateUser(t, ctx, name, &request.CreateUserReq{Password: "old@2026"})
		oldHash := sqlText(t, tx, "ac_user", "password_hash", u.ID)

		err := profile.ChangePassword(ctx, u.ID, &request.ChangePasswordReq{OldPassword: "wrong", NewPassword: "new@2026"})
		if err == nil {
			t.Fatal("原密码错误应返回错误")
		}
		if err.Error() != "原密码错误" {
			t.Errorf("错误信息 = %q", err.Error())
		}
		if sqlText(t, tx, "ac_user", "password_hash", u.ID) != oldHash {
			t.Error("原密码错误时 password_hash 不应被修改")
		}
	})
}

// TestProfileService_Menus 个人中心菜单树：仅 PC 平台、仅启用、按钮节点带权限码。
func TestProfileService_Menus(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		profile := svc.NewProfileService()

		menus, err := profile.Menus(ctx, testUserName)
		if err != nil {
			t.Fatalf("查询菜单失败: %v", err)
		}
		if len(menus) == 0 {
			t.Fatal("超级管理员应能看到菜单")
		}

		var total, buttons, withChildren int
		walkMenus(menus, func(m model.Menu) {
			total++
			if m.Platform != "pc" {
				t.Errorf("菜单 %d(%s) 的平台为 %q, 期望 pc", m.ID, m.Name, m.Platform)
			}
			if m.Active != nil && !*m.Active {
				t.Errorf("菜单 %d(%s) 已停用却出现在树中", m.ID, m.Name)
			}
			if m.Type == "button" {
				buttons++
				if m.PermissionCode == "" {
					t.Errorf("按钮菜单 %d(%s) 缺少 permission_code", m.ID, m.Name)
				}
			}
			if len(m.Children) > 0 {
				withChildren++
			}
		})
		if total < 2 {
			t.Errorf("菜单节点数 = %d, 期望 > 1", total)
		}
		if buttons == 0 {
			t.Error("菜单树应包含按钮型节点（驱动前端按钮鉴权）")
		}
		if withChildren == 0 {
			t.Error("菜单树应包含父子嵌套")
		}

		// 无角色的用户返回空数组而非 nil
		empty, err := profile.Menus(ctx, uniq("zzq_no_such_user"))
		if err != nil {
			t.Fatalf("查询不存在用户的菜单不应返回错误: %v", err)
		}
		if empty == nil {
			t.Error("应返回空数组而非 nil")
		}
		if len(empty) != 0 {
			t.Errorf("无角色用户不应有菜单, got %d", len(empty))
		}
	})
}

// TestProfileService_ListApiKeys 只列出本人的 Key，且不回显摘要。
func TestProfileService_ListApiKeys(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		profile := svc.NewProfileService()

		a := mustCreateUser(t, ctx, uniq("zzq_key_a"), nil)
		b := mustCreateUser(t, ctx, uniq("zzq_key_b"), nil)

		if _, err := profile.GenerateApiKey(ctx, a.ID, "设备网关A"); err != nil {
			t.Fatalf("生成密钥失败: %v", err)
		}
		keys, err := profile.ListApiKeys(ctx, a.ID)
		if err != nil {
			t.Fatalf("列出密钥失败: %v", err)
		}
		if len(keys) != 1 {
			t.Fatalf("密钥数 = %d, 期望 1", len(keys))
		}
		k := keys[0]
		if k.Name != "设备网关A" || k.KeyPrefix == "" {
			t.Errorf("密钥字段不完整: %+v", k)
		}
		if k.UserID != a.ID {
			t.Errorf("user_id = %d, 期望 %d", k.UserID, a.ID)
		}
		if k.LastUsedDT != nil {
			t.Errorf("新建密钥的 last_used_dt 应为 nil, got %v", k.LastUsedDT)
		}

		bKeys, _ := profile.ListApiKeys(ctx, b.ID)
		if len(bKeys) != 0 {
			t.Errorf("他人密钥不应可见, got %d", len(bKeys))
		}
	})
}

// TestProfileService_GenerateApiKey 明文仅返回一次，库中只存 SHA256 摘要与前缀。
func TestProfileService_GenerateApiKey(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		profile := svc.NewProfileService()

		u := mustCreateUser(t, ctx, uniq("zzq_keygen"), nil)
		resp, err := profile.GenerateApiKey(ctx, u.ID, "设备网关")
		if err != nil {
			t.Fatalf("生成密钥失败: %v", err)
		}
		if resp.FullKey == "" {
			t.Fatal("明文 Key 仅此一次返回，不应为空")
		}
		if len(resp.FullKey) != 64 {
			t.Errorf("明文 Key 长度 = %d, 期望 64 位 hex", len(resp.FullKey))
		}
		if resp.KeyPrefix != resp.FullKey[:8] {
			t.Errorf("key_prefix = %q, 期望明文前 8 位 %q", resp.KeyPrefix, resp.FullKey[:8])
		}

		sum := sha256.Sum256([]byte(resp.FullKey))
		wantHash := hex.EncodeToString(sum[:])

		var k model.ApiKey
		if err := tx.Where("id = ?", resp.ID).First(&k).Error; err != nil {
			t.Fatalf("回读密钥失败: %v", err)
		}
		if k.UserID != u.ID {
			t.Errorf("user_id = %d, 期望 %d", k.UserID, u.ID)
		}
		if k.Name != "设备网关" {
			t.Errorf("name = %q", k.Name)
		}
		if k.KeyHash != wantHash {
			t.Errorf("key_hash 应等于明文 SHA256: %q", k.KeyHash)
		}
		if k.KeyHash == resp.FullKey {
			t.Error("库中不得存明文")
		}
		if k.Active == nil || !*k.Active {
			t.Errorf("active = %v, 期望 true", k.Active)
		}
		if k.LastUsedDT != nil {
			t.Errorf("last_used_dt 应为 nil, got %v", k.LastUsedDT)
		}
	})
}

// TestProfileService_DeleteApiKey 删除本人的 Key：软删除且列表不再返回。
func TestProfileService_DeleteApiKey(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		profile := svc.NewProfileService()

		u := mustCreateUser(t, ctx, uniq("zzq_keydel"), nil)
		resp, _ := profile.GenerateApiKey(ctx, u.ID, "待删除")

		if err := profile.DeleteApiKey(ctx, u.ID, resp.ID); err != nil {
			t.Fatalf("删除密钥失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_api_key", resp.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		keys, _ := profile.ListApiKeys(ctx, u.ID)
		if len(keys) != 0 {
			t.Errorf("软删除后不应出现在列表中, got %d", len(keys))
		}
	})
}

// TestProfileService_DeleteApiKey_OtherUser 越权删除他人密钥被拒绝。
func TestProfileService_DeleteApiKey_OtherUser(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		profile := svc.NewProfileService()

		a := mustCreateUser(t, ctx, uniq("zzq_key_owner"), nil)
		b := mustCreateUser(t, ctx, uniq("zzq_key_other"), nil)
		resp, _ := profile.GenerateApiKey(ctx, a.ID, "A的密钥")

		err := profile.DeleteApiKey(ctx, b.ID, resp.ID)
		if err == nil {
			t.Fatal("删除他人密钥应被拒绝")
		}
		if !strings.Contains(err.Error(), "无权") {
			t.Errorf("错误信息 = %q, 期望提示无权操作", err.Error())
		}
		if dt := softDeletedAt(t, tx, "ac_api_key", resp.ID); dt != nil {
			t.Error("越权删除时 deleted_at 应仍为空")
		}
		if keys, _ := profile.ListApiKeys(ctx, a.ID); len(keys) != 1 {
			t.Errorf("A 的密钥不应减少, got %d", len(keys))
		}
	})
}
