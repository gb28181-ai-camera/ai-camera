package database

import (
	"log"
	"time"

	"github.com/boticz/ai-camera-backend/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// NewPostgresDB 建立 PostgreSQL 连接池。
func NewPostgresDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: false, // 表名使用模型显式声明的 TableName()
		},
		Logger: logger.Default.LogMode(logger.Warn),
		// Casbin 角色授权在事务中重建策略，禁用默认事务可减少不必要的开销；
		// 业务侧需显式使用 tx 的场景由 WithTx / 事务中间件保证。
		SkipDefaultTransaction: false,
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), gormCfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 10
	}
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 100
	}
	lifetime := cfg.ConnMaxLifetime
	if lifetime <= 0 {
		lifetime = time.Hour
	}

	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetConnMaxLifetime(lifetime)

	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	log.Printf("PostgreSQL 连接成功: %s:%d/%s", cfg.Host, cfg.Port, cfg.DBName)
	return db, nil
}
