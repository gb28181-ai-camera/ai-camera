package testutil

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/boticz/ai-camera-backend/config"
	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/database"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

var (
	once   sync.Once
	testDB *gorm.DB
)

// InitTestDB 初始化测试数据库连接（进程内只执行一次）。
//
// 连接参数优先取环境变量；缺失时回退读取 config/config.yaml
// （go test 的 CWD 是测试包目录，因此尝试多个相对路径）。
// 无论配置里写的是什么库名，**强制使用 ai_camera_test**，避免误连开发库。
func InitTestDB() *gorm.DB {
	once.Do(func() {
		var dbCfg config.DatabaseConfig

		host := getEnv("TEST_DB_HOST", "")
		port := getEnvInt("TEST_DB_PORT", 5432)
		user := getEnv("TEST_DB_USER", "")
		password := getEnv("TEST_DB_PASSWORD", "")
		dbname := getEnv("TEST_DB_NAME", "")

		if host == "" || user == "" || dbname == "" {
			// 回退：读配置文件
			for _, p := range []string{"config/config.yaml", "../../config/config.yaml", "../../../config/config.yaml"} {
				if cfg, err := config.LoadConfig(p); err == nil {
					dbCfg = cfg.Database
					break
				}
			}
			if dbCfg.Host == "" {
				log.Fatalf("测试数据库未配置：请设置 TEST_DB_HOST/TEST_DB_USER/TEST_DB_PASSWORD/TEST_DB_NAME")
			}
			dbCfg.DBName = "ai_camera_test" // 强制测试库
			dbCfg.MaxIdleConns = 1
			dbCfg.MaxOpenConns = 5
		} else {
			dbCfg = config.DatabaseConfig{
				Host:         host,
				Port:         port,
				User:         user,
				Password:     password,
				DBName:       dbname,
				SSLMode:      "disable",
				MaxIdleConns: 1,
				MaxOpenConns: 5,
			}
		}

		db, err := database.NewPostgresDB(&dbCfg)
		if err != nil {
			log.Fatalf("测试数据库连接失败: %v", err)
		}
		testDB = db

		// 让 repository 层拿到测试库
		bootstrap.SetDB(testDB)

		// 同步表结构
		bootstrap.AutoMigrate()

		// 测试期间静音日志
		utils.InitLogger(&config.LogConfig{Level: "fatal", Format: "console", Output: "console"})
	})

	return testDB
}

// WithTx 在事务中执行测试，结束后自动回滚，保证用例之间互不污染。
//
// 实现要点：把事务句柄写入 bootstrap 全局 DB，repository 通过 bootstrap.DB()
// 取到的就是该事务，因此所有写入都会被回滚。
// 副作用：会改写进程级全局变量，**测试不可并行**（不要调用 t.Parallel()）。
func WithTx(t *testing.T, fn func(ctx context.Context, tx *gorm.DB)) {
	t.Helper()

	db := InitTestDB()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("开启事务失败: %v", tx.Error)
	}

	bootstrap.SetDB(tx)
	ctx := context.Background()

	defer func() {
		tx.Rollback()
		bootstrap.SetDB(testDB)
	}()

	fn(ctx, tx)
}

// WithTxAndDB 与 WithTx 相同，但回调同时收到事务与原始库句柄。
func WithTxAndDB(t *testing.T, fn func(ctx context.Context, tx *gorm.DB, db *gorm.DB)) {
	t.Helper()

	db := InitTestDB()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("开启事务失败: %v", tx.Error)
	}

	bootstrap.SetDB(tx)
	ctx := context.Background()

	defer func() {
		tx.Rollback()
		bootstrap.SetDB(testDB)
	}()

	fn(ctx, tx, db)
}

// CleanTables 清空指定表（RESTART IDENTITY + CASCADE）。
func CleanTables(t *testing.T, tables ...string) {
	t.Helper()
	db := InitTestDB()
	for _, table := range tables {
		if err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table)).Error; err != nil {
			t.Fatalf("清空表 %s 失败: %v", table, err)
		}
	}
}

// SetTestContext 构造带有登录用户上下文的 context，用于触发 GORM 审计字段钩子
// （自动填充 created_by / updated_by / dept_id）。
func SetTestContext(userID int64, username string, deptID int64) context.Context {
	ctx := context.Background()
	ctx = model.WithUsername(ctx, username)
	ctx = model.WithUserID(ctx, userID)
	if deptID > 0 {
		ctx = model.WithDeptID(ctx, deptID)
	}
	return ctx
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
