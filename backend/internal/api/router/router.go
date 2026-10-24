package router

import (
	"net/http"

	"github.com/boticz/ai-camera-backend/internal/api/middleware"
	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/gb28181/metrics"
	"github.com/gin-gonic/gin"
)

// SetupRouter 构建 Gin 引擎并注册全部路由。
//
// 全局单例（配置、数据库、JWT、Casbin）已在 bootstrap 包 init() 阶段完成装配，
// 因此此处直接通过访问器读取即可。
func SetupRouter() *gin.Engine {
	cfg := bootstrap.Config()

	gin.SetMode(ginMode(cfg.Server.Mode))

	// gin.Default() 已自带 Recovery 与 Logger 中间件
	r := gin.Default()

	// 剔除查询串中的空值参数：前端生成的 Controller 会把未填写的筛选项
	// 拼成 `keyword=&active=`，而 `active=` 会被绑定为「非 nil 的 false」，
	// 导致列表恒为空。全局中间件一处修复，覆盖全部接口。
	r.Use(middleware.NormalizeEmptyQuery())

	// 上传文件静态访问（图片/录像/音频）
	uploadDir := "./uploads"
	if cfg != nil && cfg.Media.UploadDir != "" {
		uploadDir = cfg.Media.UploadDir
	}
	r.Static("/uploads", uploadDir)

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "version": "v0.1.0"})
	})

	// Prometheus 抓取端点。
	//
	// 刻意不挂 JWT / Casbin：Prometheus 没有用户会话，且 /metrics 不返回业务数据。
	// 生产环境必须由网络层限制来源（K8s NetworkPolicy / Ingress 白名单），
	// 不要把该路径暴露到公网 —— 指标会泄露设备规模与故障形态。
	r.GET("/metrics", func(c *gin.Context) {
		c.Data(http.StatusOK, metrics.ContentType, []byte(metrics.Render()))
	})

	// 注册管理后台（含官网同源接口）路由
	registerWebRoutes(r)

	// 注册移动端路由
	registerAppRoutes(r)

	// 注册 ZLMediaKit 回调路由（独立路由组，不挂 JWT/Casbin）
	registerMediaRoutes(r)

	// 注册 Swagger 文档路由
	if cfg == nil || cfg.Swagger.Enabled {
		registerSwaggerRoutes(r)
	}

	return r
}

// ginMode 把配置中的 mode 映射为 gin 的常量，非法值回退 debug。
func ginMode(mode string) string {
	switch mode {
	case gin.ReleaseMode:
		return gin.ReleaseMode
	case gin.TestMode:
		return gin.TestMode
	default:
		return gin.DebugMode
	}
}
