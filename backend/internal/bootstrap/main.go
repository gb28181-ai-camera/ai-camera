package bootstrap

import (
	"log"
	"os"
	"time"

	"github.com/boticz/ai-camera-backend/config"
	"github.com/boticz/ai-camera-backend/pkg/auth"
	"github.com/boticz/ai-camera-backend/pkg/database"
	"github.com/boticz/ai-camera-backend/pkg/perm"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// 全局单例：由 init() 在包导入时完成装配，业务代码通过访问器读取。
var (
	globalDB     *gorm.DB
	globalCfg    *config.Config
	globalJWT    *auth.JWTService
	globalCasbin *perm.CasbinService
)

// DB 返回全局数据库句柄。
func DB() *gorm.DB { return globalDB }

// Config 返回全局配置。
func Config() *config.Config { return globalCfg }

// JWTService 返回全局 JWT 服务。
func JWTService() *auth.JWTService { return globalJWT }

// CasbinService 返回全局 Casbin 服务；初始化失败时为 nil。
func CasbinService() *perm.CasbinService { return globalCasbin }

// ────────────────────────────────────────────────────────────────
// 测试注入点：测试工具用事务句柄覆盖全局 DB
// ────────────────────────────────────────────────────────────────

// SetDB 覆盖全局数据库句柄（测试用）。
func SetDB(db *gorm.DB) { globalDB = db }

// SetConfig 覆盖全局配置（测试用）。
func SetConfig(cfg *config.Config) { globalCfg = cfg }

// SetJWT 覆盖全局 JWT 服务（测试用）。
func SetJWT(jwt *auth.JWTService) { globalJWT = jwt }

// SetCasbin 覆盖全局 Casbin 服务（测试用）。
func SetCasbin(cs *perm.CasbinService) { globalCasbin = cs }

// ────────────────────────────────────────────────────────────────
// 初始化
// ────────────────────────────────────────────────────────────────

// Init 按序装配配置、日志、数据库、数据迁移、JWT 与 Casbin。
func Init(cfgPath string) {
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	globalCfg = cfg

	utils.InitLogger(&cfg.Log)

	db, err := database.NewPostgresDB(&cfg.Database)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	globalDB = db

	AutoMigrate()

	globalJWT = auth.NewJWTService(&cfg.JWT)

	// Casbin 初始化失败不阻塞启动：路由层会退化为放行中间件并打印显著告警。
	// 生产环境应通过健康检查与日志告警及时发现该状态。
	cs, err := perm.NewCasbinService(globalDB, cfg.Casbin.ModelPath)
	if err != nil {
		log.Printf("⚠️ 严重警告: Casbin初始化失败，所有接口权限校验将被跳过: %v", err)
		globalCasbin = nil
	} else {
		globalCasbin = cs
		interval := cfg.Casbin.AutoLoadInterval
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		cs.StartAutoReload(interval)
	}

	utils.Info("AI Camera 服务初始化完成")
}

// autoInit 读取 CONFIG_PATH（缺省 config/config.yaml）并执行 Init。
// 配置缺失时静默跳过，使 go test / go vet 等无配置场景不会中断。
func autoInit() {
	cfgPath := "config/config.yaml"
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		cfgPath = p
	}

	if _, err := config.LoadConfig(cfgPath); err != nil {
		log.Printf("初始化跳过: %v", err)
		return
	}
	Init(cfgPath)
}

func init() { autoInit() }
