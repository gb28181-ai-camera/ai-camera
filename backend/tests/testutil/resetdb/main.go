// resetdb 清空测试数据库的全部表。
//
//	cd backend && go run ./tests/testutil/resetdb
//
// 执行顺序：resetdb → initdb → 启动测试服务（config-test.yaml）
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	defaultHost   = "192.168.1.227"
	defaultPort   = 5432
	defaultUser   = "boticz"
	defaultDBName = "ai_camera_test"
)

func main() {
	host := env("TEST_DB_HOST", defaultHost)
	port := envInt("TEST_DB_PORT", defaultPort)
	user := env("TEST_DB_USER", defaultUser)
	// 密码不给默认值：避免明文凭据入库，缺失时直接失败并提示
	password := os.Getenv("TEST_DB_PASSWORD")
	if password == "" {
		log.Fatal("未设置环境变量 TEST_DB_PASSWORD（为避免密码进入版本库，此处不再提供默认值）")
	}
	dbname := env("TEST_DB_NAME", defaultDBName)

	if dbname != "ai_camera_test" {
		log.Fatalf("拒绝执行：resetdb 仅允许操作 ai_camera_test，当前为 %s", dbname)
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
		host, port, user, password, dbname)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("连接测试数据库失败: %v", err)
	}

	var tables []string
	err = db.Raw(`SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname = 'public' ORDER BY tablename`).
		Scan(&tables).Error
	if err != nil {
		log.Fatalf("查询表清单失败: %v", err)
	}

	if len(tables) == 0 {
		fmt.Println("✅ 数据库为空，无需清空")
		return
	}

	// 一条语句清空全部表，RESTART IDENTITY 重置自增序列，CASCADE 处理外键
	stmt := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(tables, ", "))
	if err := db.Exec(stmt).Error; err != nil {
		log.Fatalf("清空失败: %v", err)
	}

	fmt.Printf("✅ 已清空 %d 张表: %s\n", len(tables), dbname)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
