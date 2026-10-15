package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/config"
	"github.com/boticz/ai-camera-backend/internal/api/middleware"
	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/auth"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// AuthMiddleware
// ════════════════════════════════════════════════════════════════

// TestAuthMiddleware 覆盖认证中间件的全部短路分支与放行分支（不触碰数据库）。
func TestAuthMiddleware(t *testing.T) {
	middleware.SetJWTServiceForTest(testJWTService())
	mw := middleware.NewAuthMiddleware()

	t.Run("无 Authorization 头", func(t *testing.T) {
		rec, reached := runChain(t, http.MethodGet, "/web/profile", nil, nil, mw.Handler())
		resp := decodeResp(t, rec)
		if rec.Code != http.StatusUnauthorized || resp.Code != 401 {
			t.Errorf("期望 401，实际 status=%d code=%d", rec.Code, resp.Code)
		}
		if resp.Message != "未提供认证Token" {
			t.Errorf("期望 message=未提供认证Token，实际 %q", resp.Message)
		}
		if *reached {
			t.Error("未提供 Token 时不应放行到下游 handler")
		}
	})

	t.Run("认证格式错误", func(t *testing.T) {
		cases := map[string]string{
			"方案名错误(Token)": "Token abc.def.ghi",
			"只有 token 无方案": "abc.def.ghi",
			"方案名小写 bearer": "bearer abc.def.ghi",
			"方案名错误(Basic)": "Basic dXNlcjpwYXNz",
		}
		for name, header := range cases {
			name, header := name, header
			t.Run(name, func(t *testing.T) {
				rec, reached := runChain(t, http.MethodGet, "/web/profile",
					map[string]string{"Authorization": header}, nil, mw.Handler())
				resp := decodeResp(t, rec)
				if rec.Code != http.StatusUnauthorized || resp.Code != 401 {
					t.Errorf("期望 401，实际 status=%d code=%d", rec.Code, resp.Code)
				}
				if resp.Message != "认证格式错误" {
					t.Errorf("期望 message=认证格式错误，实际 %q", resp.Message)
				}
				if *reached {
					t.Error("格式错误时不应放行")
				}
			})
		}
	})

	t.Run("无效/过期 token", func(t *testing.T) {
		expiredSvc := auth.NewJWTService(&config.JWTConfig{Secret: testJWTSecret, ExpireHours: -1})
		expired, err := expiredSvc.GenerateToken(1, "u", "r", 1, "all", 1, "")
		if err != nil {
			t.Fatalf("签发过期 token 失败: %v", err)
		}
		otherSvc := auth.NewJWTService(&config.JWTConfig{Secret: "another-secret", ExpireHours: 1})
		otherSigned, err := otherSvc.GenerateToken(1, "u", "r", 1, "all", 1, "")
		if err != nil {
			t.Fatalf("签发异密钥 token 失败: %v", err)
		}

		cases := map[string]string{
			"非法字符串":   "Bearer not-a-jwt",
			"过期":      "Bearer " + expired,
			"签名密钥不匹配": "Bearer " + otherSigned,
			"三段式伪造":   "Bearer aaa.bbb.ccc",
			// 方案名正确但 token 为空：格式合法、内容非法，走「无效的Token」而非「认证格式错误」。
			"空 token": "Bearer ",
		}
		for name, header := range cases {
			name, header := name, header
			t.Run(name, func(t *testing.T) {
				rec, reached := runChain(t, http.MethodGet, "/web/profile",
					map[string]string{"Authorization": header}, nil, mw.Handler())
				resp := decodeResp(t, rec)
				if rec.Code != http.StatusUnauthorized || resp.Code != 401 {
					t.Errorf("期望 401，实际 status=%d code=%d", rec.Code, resp.Code)
				}
				if resp.Message != "无效的Token" {
					t.Errorf("期望 message=无效的Token，实际 %q", resp.Message)
				}
				if *reached {
					t.Error("非法 token 不应放行")
				}
			})
		}
	})

	t.Run("有效 token 放行并写满上下文", func(t *testing.T) {
		token := validToken(t)
		var captured *gin.Context
		rec, reached := runChain(t, http.MethodGet, "/web/profile", bearer(token),
			func(c *gin.Context) {
				captured = c
				c.Status(http.StatusOK)
			}, mw.Handler())

		if !*reached {
			t.Fatalf("有效 token 未放行，status=%d body=%s", rec.Code, rec.Body.String())
		}
		if captured == nil {
			t.Fatal("下游 handler 未取得 gin.Context")
		}
		if captured.IsAborted() {
			t.Error("有效 token 不应 Abort")
		}

		// 7 个 gin.Context 键：handler 侧（分页/数据权限）全部依赖它们。
		checks := []struct {
			key  string
			want interface{}
		}{
			{"user_id", testUserID},
			{"user_name", testUsername},
			{"role_code", testRoleCode},
			{"dept_id", testDeptID},
			{"data_perm", "all"},
			{"company_id", int64(1)},
			{"cross_dept_ids", "2,3"},
		}
		for _, ck := range checks {
			got := captured.Value(ck.key)
			if got != ck.want {
				t.Errorf("gin.Context[%s] = %v (%T)，期望 %v (%T)", ck.key, got, got, ck.want, ck.want)
			}
		}

		// Go Context 注入：驱动 GORM 审计字段（created_by / updated_by / dept_id）自动填充。
		goCtx := captured.Request.Context()
		if got := model.UsernameFromCtx(goCtx); got != testUsername {
			t.Errorf("Go Context 登录名 = %q，期望 %q", got, testUsername)
		}
		if got := model.DeptIDFromCtx(goCtx); got == nil || *got != testDeptID {
			t.Errorf("Go Context 部门 ID = %v，期望 %d", got, testDeptID)
		}
		if got := model.UserIDFromCtx(goCtx); got == nil || *got != testUserID {
			t.Errorf("Go Context 用户 ID = %v，期望 %d", got, testUserID)
		}
	})
}

// TestAuthMiddleware_AuditFieldsFlowIntoDB 断言 AuthMiddleware 注入的 Go Context
// 真的能让 GORM 钩子把 created_by / updated_by 落库。
//
// 这是上一条「Context 注入」断言的端到端补强：中间件写对上下文还不够，
// 必须真的影响持久化。需要数据库。
func TestAuthMiddleware_AuditFieldsFlowIntoDB(t *testing.T) {
	middleware.SetJWTServiceForTest(testJWTService())
	mw := middleware.NewAuthMiddleware()
	token := validToken(t)

	testutil.WithTx(t, func(_ context.Context, tx *gorm.DB) {
		var captured *gin.Context
		_, reached := runChain(t, http.MethodGet, "/web/profile", bearer(token),
			func(c *gin.Context) {
				captured = c
				c.Status(http.StatusOK)
			}, mw.Handler())
		if !*reached || captured == nil {
			t.Fatal("有效 token 未放行")
		}

		// 用中间件产出的 context 走一次真实的 GORM 写入。
		probe := model.OperationLog{
			Module: "api", Action: "新增", Method: http.MethodPost,
			Path: "/web/__audit_probe", Status: 1,
		}
		if err := tx.WithContext(captured.Request.Context()).Create(&probe).Error; err != nil {
			t.Fatalf("写入审计探针失败: %v", err)
		}

		var stored model.OperationLog
		if err := tx.First(&stored, probe.ID).Error; err != nil {
			t.Fatalf("回读审计探针失败: %v", err)
		}
		if stored.CreatedBy != testUsername {
			t.Errorf("created_by = %q，期望 %q（中间件注入的 Go Context 未生效）", stored.CreatedBy, testUsername)
		}
		if stored.UpdatedBy != testUsername {
			t.Errorf("updated_by = %q，期望 %q", stored.UpdatedBy, testUsername)
		}
	})
}

// ════════════════════════════════════════════════════════════════
// CasbinMiddleware
// ════════════════════════════════════════════════════════════════

// newTestEnforcer 用**生产 model 文件**构造内存 enforcer（不接数据库）。
//
// 直接加载 config/rbac_model.conf 是关键：本组用例校验的是 matcher 表达式本身，
// 而不是中间件的转发逻辑。
func newTestEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	e, err := casbin.NewEnforcer(rbacModelRel)
	if err != nil {
		t.Fatalf("加载 Casbin model %s 失败: %v", rbacModelRel, err)
	}
	return e
}

// TestCasbinMiddleware 覆盖放行/拒绝与 matcher 语义（不触碰数据库）。
func TestCasbinMiddleware(t *testing.T) {
	middleware.SetJWTServiceForTest(testJWTService())
	authMW := middleware.NewAuthMiddleware()
	token := validToken(t)

	e := newTestEnforcer(t)
	if _, err := e.AddPolicy(testRoleCode, "/web/allowed", http.MethodGet); err != nil {
		t.Fatalf("添加 p 策略失败: %v", err)
	}
	if _, err := e.AddPolicy(testRoleCode, "/web/devices/:id", http.MethodGet); err != nil {
		t.Fatalf("添加 p 策略失败: %v", err)
	}
	// 角色继承链 g：用户登录名 → 角色编码。
	if _, err := e.AddGroupingPolicy(testUsername, testRoleCode); err != nil {
		t.Fatalf("添加 g 策略失败: %v", err)
	}

	// 适配 CasbinMiddleware 需要的三元签名；直接使用 enforcer 保证 matcher 语义真实。
	enforce := func(sub, obj, act string) (bool, error) { return e.Enforce(sub, obj, act) }
	casbinMW := middleware.CasbinMiddleware(enforce)

	t.Run("有权限放行", func(t *testing.T) {
		rec, reached := runChain(t, http.MethodGet, "/web/allowed", bearer(token), nil,
			authMW.Handler(), casbinMW)
		if !*reached {
			t.Fatalf("有权限却被拦截：status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("无权限 403 权限不足", func(t *testing.T) {
		rec, reached := runChain(t, http.MethodGet, "/web/forbidden", bearer(token), nil,
			authMW.Handler(), casbinMW)
		resp := decodeResp(t, rec)
		if rec.Code != http.StatusForbidden || resp.Code != 403 {
			t.Errorf("期望 403，实际 status=%d code=%d", rec.Code, resp.Code)
		}
		if resp.Message != "权限不足" {
			t.Errorf("期望 message=权限不足，实际 %q", resp.Message)
		}
		if *reached {
			t.Error("无权限不应放行")
		}
	})

	// ── 关键回归：matcher 必须校验 obj 与 act ──
	//
	// 主体只有 (tester, /web/allowed, GET) 一条策略：
	//   · 换方法（POST 同一路径）必须被拒；
	//   · 换路径（GET 另一路径）必须被拒。
	// 若 matcher 退化成「只校验角色成员」（如 m = g(r.sub, p.sub) 而丢掉
	// keyMatch2(r.obj, p.obj) 与 r.act == p.act），下面两条会失败。
	t.Run("matcher 校验 act：POST 同一路径被拒", func(t *testing.T) {
		rec, reached := runChain(t, http.MethodPost, "/web/allowed", bearer(token), nil,
			authMW.Handler(), casbinMW)
		if *reached {
			t.Fatalf("matcher 未校验 act：角色成员即可 POST /web/allowed（status=%d）", rec.Code)
		}
		resp := decodeResp(t, rec)
		if rec.Code != http.StatusForbidden || resp.Message != "权限不足" {
			t.Errorf("期望 403 权限不足，实际 status=%d code=%d message=%q",
				rec.Code, resp.Code, resp.Message)
		}
	})

	t.Run("matcher 校验 obj：GET 另一路径被拒", func(t *testing.T) {
		rec, reached := runChain(t, http.MethodGet, "/web/allowed/other", bearer(token), nil,
			authMW.Handler(), casbinMW)
		if *reached {
			t.Fatalf("matcher 未校验 obj：角色成员即可访问任意路径（status=%d）", rec.Code)
		}
		resp := decodeResp(t, rec)
		if rec.Code != http.StatusForbidden || resp.Message != "权限不足" {
			t.Errorf("期望 403 权限不足，实际 status=%d code=%d message=%q",
				rec.Code, resp.Code, resp.Message)
		}
	})

	t.Run("matcher keyMatch2 支持 :id 路由参数", func(t *testing.T) {
		rec, reached := runChain(t, http.MethodGet, "/web/devices/42", bearer(token), nil,
			authMW.Handler(), casbinMW)
		if !*reached {
			t.Errorf("p 策略 /web/devices/:id 应覆盖 /web/devices/42，实际 status=%d body=%s",
				rec.Code, rec.Body.String())
		}

		// 同一策略不得外溢到更深的路径。
		rec, reached = runChain(t, http.MethodGet, "/web/devices/42/mode", bearer(token), nil,
			authMW.Handler(), casbinMW)
		if *reached {
			t.Errorf("p 策略 /web/devices/:id 不应覆盖 /web/devices/42/mode（status=%d）", rec.Code)
		}
	})

	t.Run("缺少登录名 401", func(t *testing.T) {
		// 不挂 AuthMiddleware，CasbinMiddleware 取不到 user_name。
		rec, reached := runChain(t, http.MethodGet, "/web/allowed", nil, nil, casbinMW)
		resp := decodeResp(t, rec)
		if rec.Code != http.StatusUnauthorized || resp.Code != 401 {
			t.Errorf("期望 401，实际 status=%d code=%d", rec.Code, resp.Code)
		}
		if resp.Message != "未提供认证Token" {
			t.Errorf("期望 message=未提供认证Token，实际 %q", resp.Message)
		}
		if *reached {
			t.Error("缺少主体不应放行")
		}
	})

	t.Run("enforce 出错 500 权限校验失败", func(t *testing.T) {
		failing := middleware.CasbinMiddleware(func(_, _, _ string) (bool, error) {
			return false, context.DeadlineExceeded
		})
		rec, reached := runChain(t, http.MethodGet, "/web/allowed", bearer(token), nil,
			authMW.Handler(), failing)
		resp := decodeResp(t, rec)
		if rec.Code != http.StatusInternalServerError || resp.Code != 500 {
			t.Errorf("期望 500，实际 status=%d code=%d", rec.Code, resp.Code)
		}
		if resp.Message != "权限校验失败" {
			t.Errorf("期望 message=权限校验失败，实际 %q", resp.Message)
		}
		if *reached {
			t.Error("enforce 出错不应放行（fail-closed）")
		}
	})
}

// TestCasbinMiddleware_DegradesToPassThroughWhenServiceMissing 覆盖路由层的降级路径。
//
// bootstrap.Init() 在 Casbin 初始化失败时把 globalCasbin 置 nil，web.go/app.go
// 检测到后改用 pass-through 中间件（bootstrap 会打印显著告警）。
// 测试进程未执行 Init，globalCasbin 天然为 nil，正好验证该分支。
func TestCasbinMiddleware_DegradesToPassThroughWhenServiceMissing(t *testing.T) {
	if bootstrap.CasbinService() != nil {
		t.Skip("测试进程已初始化 Casbin，无法覆盖降级路径")
	}

	r := buildRouter(t)
	token := validToken(t)

	testutil.WithTx(t, func(_ context.Context, _ *gorm.DB) {
		// 选一个不依赖「当前用户在库中是否存在」的受保护列表端点：
		// /web/profile 会按 JWT 里的 user_id 回查用户表，测试库里不一定有该用户。
		rec := doRequest(r, http.MethodGet, "/web/devices/modes", token, "")
		resp := decodeResp(t, rec)
		if rec.Code == http.StatusForbidden {
			t.Fatalf("Casbin 服务为 nil 时路由层应降级放行，实际 403: %s", resp.Message)
		}
		if strings.Contains(resp.Message, "权限校验失败") {
			t.Fatalf("Casbin 服务为 nil 时不应报权限校验失败: %s", resp.Message)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("期望降级放行并返回 200，实际 status=%d code=%d message=%q",
				rec.Code, resp.Code, resp.Message)
		}
	})
}

// ════════════════════════════════════════════════════════════════
// NormalizeEmptyQuery
// ════════════════════════════════════════════════════════════════

// normalizeProbeReq 复刻 PageXxxReq 里最容易踩坑的三类筛选项：
// 指针布尔、指针数值、普通字符串。
type normalizeProbeReq struct {
	Keyword string `form:"keyword"`
	Active  *bool  `form:"active"`
	DeptID  *int64 `form:"dept_id"`
}

// normalizeProbeResp 终端 handler 回显的绑定结果 + 中间件改写后的 RawQuery。
type normalizeProbeResp struct {
	Keyword  string `json:"keyword"`
	Active   *bool  `json:"active"`
	DeptID   *int64 `json:"dept_id"`
	RawQuery string `json:"raw_query"`
}

// TestNormalizeEmptyQuery 覆盖「空查询参数导致列表恒为空」的回归（不触碰数据库）。
//
// 背景：前端 Controller 会把未填写的筛选项拼成 `keyword=&active=`，
// `active=` 被绑定为「非 nil 的 false」，于是 WHERE active = false → 列表恒为 0 条。
func TestNormalizeEmptyQuery(t *testing.T) {
	// 终端 handler 绑定查询串并回显，同时暴露中间件改写后的 RawQuery。
	terminal := func(c *gin.Context) {
		var req normalizeProbeReq
		if err := c.ShouldBindQuery(&req); err != nil {
			utils.BadRequest(c, "参数错误: "+err.Error())
			return
		}
		utils.Success(c, normalizeProbeResp{
			Keyword:  req.Keyword,
			Active:   req.Active,
			DeptID:   req.DeptID,
			RawQuery: c.Request.URL.RawQuery,
		})
	}

	bind := func(t *testing.T, target string) normalizeProbeResp {
		t.Helper()
		rec, reached := runChain(t, http.MethodGet, target, nil, terminal,
			middleware.NormalizeEmptyQuery())
		if !*reached {
			t.Fatalf("中间件未放行，status=%d body=%s", rec.Code, rec.Body.String())
		}
		resp := decodeResp(t, rec)
		var out normalizeProbeResp
		if err := json.Unmarshal(resp.Data, &out); err != nil {
			t.Fatalf("解析 data 失败: %v, data=%s", err, string(resp.Data))
		}
		return out
	}

	t.Run("空值参数被剔除：active 绑定为 nil", func(t *testing.T) {
		out := bind(t, "/web/system/users?keyword=&active=")
		if out.Active != nil {
			t.Errorf("active = *%v，期望 nil（空值参数未被剔除 → 列表恒为空）", *out.Active)
		}
		if out.Keyword != "" {
			t.Errorf("keyword = %q，期望空串", out.Keyword)
		}
	})

	t.Run("纯空白参数同样被剔除", func(t *testing.T) {
		out := bind(t, "/web/system/users?keyword=%20%20&dept_id=%20")
		if out.DeptID != nil {
			t.Errorf("dept_id = *%d，期望 nil（纯空白未被剔除）", *out.DeptID)
		}
		if out.Keyword != "" {
			t.Errorf("keyword = %q，期望空串", out.Keyword)
		}
	})

	t.Run("显式 false 必须保留为 *false", func(t *testing.T) {
		out := bind(t, "/web/system/users?active=false")
		if out.Active == nil {
			t.Fatal("active = nil，期望指向 false 的指针（显式传值被误剔除）")
		}
		if *out.Active {
			t.Errorf("active = true，期望 false")
		}
	})

	t.Run("显式 true 必须保留", func(t *testing.T) {
		out := bind(t, "/web/system/users?active=true")
		if out.Active == nil || !*out.Active {
			t.Errorf("active = %v，期望指向 true 的指针", out.Active)
		}
	})

	t.Run("数字 0 与字符串值不得被剔除", func(t *testing.T) {
		out := bind(t, "/web/system/users?dept_id=0&keyword=abc")
		if out.DeptID == nil || *out.DeptID != 0 {
			t.Errorf("dept_id = %v，期望指向 0 的指针", out.DeptID)
		}
		if out.Keyword != "abc" {
			t.Errorf("keyword = %q，期望 abc", out.Keyword)
		}
	})

	t.Run("混合：保留有值项并剔除空值项", func(t *testing.T) {
		out := bind(t, "/web/system/users?keyword=abc&active=&dept_id=2")
		if out.Keyword != "abc" {
			t.Errorf("keyword = %q，期望 abc", out.Keyword)
		}
		if out.Active != nil {
			t.Errorf("active = *%v，期望 nil", *out.Active)
		}
		if out.DeptID == nil || *out.DeptID != 2 {
			t.Errorf("dept_id = %v，期望指向 2 的指针", out.DeptID)
		}
	})

	t.Run("无查询串时不报错", func(t *testing.T) {
		out := bind(t, "/web/system/users")
		if out.Active != nil || out.DeptID != nil {
			t.Errorf("无查询串时指针字段应为 nil，实际 active=%v dept_id=%v", out.Active, out.DeptID)
		}
	})

	t.Run("RawQuery 被真实改写", func(t *testing.T) {
		out := bind(t, "/web/system/users?keyword=&active=")
		if out.RawQuery != "" {
			t.Errorf("RawQuery = %q，期望被清空（空值参数已剔除）", out.RawQuery)
		}
	})
}

// ════════════════════════════════════════════════════════════════
// ApiKeyAuth
// ════════════════════════════════════════════════════════════════

// TestApiKeyAuth 覆盖设备上报通道的密钥校验（需要数据库，事务内回滚）。
func TestApiKeyAuth(t *testing.T) {
	const (
		activeKey   = "api-key-active-0123456789"
		disabledKey = "api-key-disabled-0123456789"
	)

	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testutil.SetTestContext(testUserID, testUsername, testDeptID)
		active, disabled := true, false
		keys := []model.ApiKey{
			{
				Active: &active, UserID: testUserID, Name: "ZZQ-启用",
				KeyPrefix: "api-key-active", KeyHash: middleware.HashApiKey(activeKey),
			},
			{
				Active: &disabled, UserID: testUserID, Name: "ZZQ-停用",
				KeyPrefix: "api-key-disabled", KeyHash: middleware.HashApiKey(disabledKey),
			},
		}
		for i := range keys {
			if err := tx.WithContext(ctx).Create(&keys[i]).Error; err != nil {
				t.Fatalf("创建测试 API Key 失败: %v", err)
			}
		}
		keyID := keys[0].ID

		// 前置条件：新建密钥的 last_used_dt 必须为 NULL，
		// 否则后面「刷新」的断言会失去意义。
		var created model.ApiKey
		if err := tx.First(&created, keyID).Error; err != nil {
			t.Fatalf("回读密钥失败: %v", err)
		}
		if created.LastUsedDT != nil {
			t.Fatalf("新建密钥的 last_used_dt 应为 NULL，实际 %v", created.LastUsedDT)
		}

		mw := middleware.ApiKeyAuth()

		t.Run("缺少 X-Api-Key 401", func(t *testing.T) {
			rec, reached := runChain(t, http.MethodPost, "/web/risks/report", nil, nil, mw)
			resp := decodeResp(t, rec)
			if rec.Code != http.StatusUnauthorized || resp.Code != 401 {
				t.Errorf("期望 401，实际 status=%d code=%d", rec.Code, resp.Code)
			}
			if resp.Message != "缺少API Key" {
				t.Errorf("期望 message=缺少API Key，实际 %q", resp.Message)
			}
			if *reached {
				t.Error("缺少密钥不应放行")
			}
		})

		t.Run("无效 key 401", func(t *testing.T) {
			rec, reached := runChain(t, http.MethodPost, "/web/risks/report",
				map[string]string{"X-Api-Key": "totally-wrong"}, nil, mw)
			resp := decodeResp(t, rec)
			if rec.Code != http.StatusUnauthorized || resp.Code != 401 {
				t.Errorf("期望 401，实际 status=%d code=%d", rec.Code, resp.Code)
			}
			if resp.Message != "无效的API Key" {
				t.Errorf("期望 message=无效的API Key，实际 %q", resp.Message)
			}
			if *reached {
				t.Error("无效密钥不应放行")
			}
		})

		t.Run("已停用 key 401", func(t *testing.T) {
			rec, reached := runChain(t, http.MethodPost, "/web/risks/report",
				map[string]string{"X-Api-Key": disabledKey}, nil, mw)
			resp := decodeResp(t, rec)
			if rec.Code != http.StatusUnauthorized || resp.Message != "无效的API Key" {
				t.Errorf("停用密钥应被拒，实际 status=%d message=%q", rec.Code, resp.Message)
			}
			if *reached {
				t.Error("停用密钥不应放行")
			}
		})

		t.Run("有效 key 放行并写入主体", func(t *testing.T) {
			var captured *gin.Context
			rec, reached := runChain(t, http.MethodPost, "/web/risks/report",
				map[string]string{"X-Api-Key": activeKey},
				func(c *gin.Context) {
					captured = c
					c.Status(http.StatusOK)
				}, mw)
			if !*reached {
				t.Fatalf("有效密钥未放行：status=%d body=%s", rec.Code, rec.Body.String())
			}
			if got := captured.GetInt64("user_id"); got != testUserID {
				t.Errorf("user_id = %d，期望 %d", got, testUserID)
			}
			if got := captured.GetInt64("api_key_id"); got != keyID {
				t.Errorf("api_key_id = %d，期望 %d", got, keyID)
			}
		})

		t.Run("有效 key 刷新 last_used_dt", func(t *testing.T) {
			var after model.ApiKey
			if err := tx.First(&after, keyID).Error; err != nil {
				t.Fatalf("回读密钥失败: %v", err)
			}
			// 写入的是 gorm.Expr("NOW()")；若被当作字符串字面量，PostgreSQL 会
			// 拒绝转换，更新失败，表现为 last_used_dt 始终为 NULL。
			if after.LastUsedDT == nil {
				t.Fatal("last_used_dt 仍为 NULL：gorm.Expr(\"NOW()\") 未生效")
			}
		})

		t.Run("无效 key 不刷新 last_used_dt", func(t *testing.T) {
			// 用第二个（停用）密钥做对照：校验失败不应产生任何写入。
			var other model.ApiKey
			if err := tx.First(&other, keys[1].ID).Error; err != nil {
				t.Fatalf("回读密钥失败: %v", err)
			}
			if other.LastUsedDT != nil {
				t.Errorf("校验失败的 key 不应刷新 last_used_dt，实际 %v", other.LastUsedDT)
			}
		})
	})
}

// ════════════════════════════════════════════════════════════════
// OperationLog
// ════════════════════════════════════════════════════════════════

// TestOperationLog 覆盖审计中间件的记录策略与请求体重放（需要数据库，事务内回滚）。
func TestOperationLog(t *testing.T) {
	mw := middleware.OperationLog()

	testutil.WithTx(t, func(_ context.Context, tx *gorm.DB) {
		countByPath := func(t *testing.T, path string) int64 {
			t.Helper()
			var n int64
			if err := tx.Model(&model.OperationLog{}).Where("path = ?", path).Count(&n).Error; err != nil {
				t.Fatalf("统计操作日志失败: %v", err)
			}
			return n
		}
		lastByPath := func(t *testing.T, path string) model.OperationLog {
			t.Helper()
			var row model.OperationLog
			if err := tx.Where("path = ?", path).Order("id DESC").First(&row).Error; err != nil {
				t.Fatalf("读取操作日志失败: %v", err)
			}
			return row
		}

		t.Run("GET 不记录", func(t *testing.T) {
			const path = "/web/__oplog_get_probe"
			_, _ = runChain(t, http.MethodGet, path, nil,
				func(c *gin.Context) { c.Status(http.StatusOK) }, mw)
			if n := countByPath(t, path); n != 0 {
				t.Errorf("GET 不应写入操作日志，实际新增 %d 行", n)
			}
		})

		t.Run("POST/PUT/DELETE 记录并映射动作", func(t *testing.T) {
			cases := []struct {
				method string
				path   string
				action string
			}{
				{http.MethodPost, "/web/__oplog_post_probe", "新增"},
				{http.MethodPut, "/web/__oplog_put_probe", "修改"},
				{http.MethodDelete, "/web/__oplog_delete_probe", "删除"},
			}
			for _, c := range cases {
				rec, _ := runChain(t, c.method, c.path, nil,
					func(ctx *gin.Context) { utils.Success(ctx, nil) }, mw)
				if rec.Code != http.StatusOK {
					t.Fatalf("%s %s 期望 200，实际 %d", c.method, c.path, rec.Code)
				}

				if n := countByPath(t, c.path); n != 1 {
					t.Fatalf("%s %s 应写入 1 行操作日志，实际 %d 行", c.method, c.path, n)
				}
				row := lastByPath(t, c.path)
				if row.Method != c.method {
					t.Errorf("method = %q，期望 %q", row.Method, c.method)
				}
				if row.Action != c.action {
					t.Errorf("action = %q，期望 %q", row.Action, c.action)
				}
				if row.Status != 1 {
					t.Errorf("status = %d，期望 1（成功）", row.Status)
				}
				// moduleFromPath：/web/<module>/... 的第二段即模块名。
				wantModule := strings.TrimPrefix(strings.TrimPrefix(c.path, "/web/"), "/app/")
				if row.Module != wantModule {
					t.Errorf("module = %q，期望从路径第二段推导得到 %q", row.Module, wantModule)
				}
				if row.Operator != "anonymous" {
					t.Errorf("operator = %q，期望 anonymous（未挂认证中间件）", row.Operator)
				}
			}
		})

		t.Run("失败请求记录 status=0 与错误文案", func(t *testing.T) {
			const path = "/web/__oplog_fail_probe"
			_, _ = runChain(t, http.MethodPost, path, nil,
				func(c *gin.Context) { utils.BadRequest(c, "故意失败") }, mw)

			if n := countByPath(t, path); n != 1 {
				t.Fatalf("应写入 1 行操作日志，实际 %d 行", n)
			}
			row := lastByPath(t, path)
			if row.Status != 0 {
				t.Errorf("status = %d，期望 0（失败）", row.Status)
			}
			if row.ErrorMsg == "" {
				t.Error("error_msg 为空，失败请求应记录错误文案")
			}
		})

		t.Run("请求体先记录后被下游完整读取", func(t *testing.T) {
			const path = "/web/__oplog_body_probe"
			const body = `{"name":"审计探针","value":42}`

			var seen string
			rec, reached := runChainWithBody(t, http.MethodPost, path, body, nil,
				func(c *gin.Context) {
					raw, err := io.ReadAll(c.Request.Body)
					if err != nil {
						t.Errorf("下游读取 body 失败: %v", err)
					}
					seen = string(raw)
					utils.Success(c, nil)
				}, mw)

			if !*reached {
				t.Fatalf("未放行到下游 handler，status=%d", rec.Code)
			}
			// 中间件读走 body 后必须写回，否则下游 handler 读到空 body。
			if seen != body {
				t.Errorf("下游读到的 body = %q，期望 %q（中间件未重放请求体）", seen, body)
			}
			row := lastByPath(t, path)
			if row.Params != body {
				t.Errorf("params = %q，期望 %q", row.Params, body)
			}
		})

		t.Run("超长请求体按上限截断", func(t *testing.T) {
			const path = "/web/__oplog_bigbody_probe"
			big := `{"payload":"` + strings.Repeat("x", 8192) + `"}`

			_, reached := runChainWithBody(t, http.MethodPost, path, big, nil,
				func(c *gin.Context) {
					raw, _ := io.ReadAll(c.Request.Body)
					if len(raw) != 4096 {
						t.Errorf("下游读到的 body 长度 = %d，期望被截断为 4096", len(raw))
					}
					utils.Success(c, nil)
				}, mw)
			if !*reached {
				t.Fatal("未放行到下游 handler")
			}

			row := lastByPath(t, path)
			if len(row.Params) != 4096 {
				t.Errorf("params 长度 = %d，期望截断到 4096", len(row.Params))
			}
		})
	})
}
