// initdb 向测试数据库导入初始数据脚本。
//
//	cd backend && go run ./tests/testutil/initdb
//
// 脚本路径：docs/开发文档/数据库设计/sql/init/01-初始数据.sql
// 执行前会关闭外键检查（session_replication_role = replica），使导入不依赖表顺序。
// 单条语句失败只告警不中断，便于重复执行时容错。
package main

import (
	"bufio"
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

	scriptRelPath      = "../docs/开发文档/数据库设计/sql/init/01-初始数据.sql"
	scriptFallbackPath = "docs/开发文档/数据库设计/sql/init/01-初始数据.sql"
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
		log.Fatalf("拒绝执行：initdb 仅允许操作 ai_camera_test，当前为 %s", dbname)
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
		host, port, user, password, dbname)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("连接测试数据库失败: %v", err)
	}

	path := scriptRelPath
	if _, err := os.Stat(path); err != nil {
		path = scriptFallbackPath
	}
	if _, err := os.Stat(path); err != nil {
		log.Fatalf("初始化脚本不存在: %s（请确认已运行 go run ./cmd/server 完成建表，且 DDL 脚本已就位）", scriptRelPath)
	}

	stmts, err := loadStatements(path)
	if err != nil {
		log.Fatalf("读取脚本失败: %v", err)
	}

	// 关闭外键与触发器检查，避免导入顺序问题
	if err := db.Exec("SET session_replication_role = 'replica'").Error; err != nil {
		log.Printf("⚠️ 关闭外键检查失败（可能权限不足，继续执行）: %v", err)
	}

	ok, failed := 0, 0
	for i, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			failed++
			fmt.Printf("  ⚠️ 第 %d 条语句失败: %v\n     %.120s\n", i+1, err, strings.ReplaceAll(s, "\n", " "))
			continue
		}
		ok++
	}

	if err := db.Exec("SET session_replication_role = 'origin'").Error; err != nil {
		log.Printf("⚠️ 恢复外键检查失败: %v", err)
	}

	fixed := syncSequences(db)

	fmt.Printf("✅ 导入完成: %d 条成功, %d 条失败 (%s)\n", ok, failed, path)
	fmt.Printf("✅ 已对齐 %d 个自增序列\n", fixed)
}

// syncSequences 把每张表的 id 序列推进到 max(id)。
//
// 初始化脚本用**显式 ID** 写入种子数据，不会推进 PostgreSQL 的自增序列，
// 导致后续 INSERT 从 1 开始分配主键并与种子数据冲突（duplicate key）。
// 这里逐表 setval 修正；GREATEST 保证只前推不回拨，重复执行安全。
func syncSequences(db *gorm.DB) int {
	// 通过「id 列的默认值是 nextval(...)」来识别自增主键，
	// 而不是用 pg_get_serial_sequence —— 后者会对系统目录表（如 pg_statistic）
	// 求值 'public.pg_statistic' 这个不存在的 relation 而报错。
	var tables []string
	err := db.Raw(`
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'id' AND a.attnum > 0
		JOIN pg_attrdef d ON d.adrelid = c.oid AND d.adnum = a.attnum
		WHERE c.relkind = 'r'
		  AND n.nspname = 'public'
		  AND pg_get_expr(d.adbin, d.adrelid) LIKE 'nextval(%'
		ORDER BY c.relname`).Scan(&tables).Error
	if err != nil {
		log.Printf("⚠️ 查询序列表失败: %v", err)
		return 0
	}

	fixed := 0
	for _, t := range tables {
		sql := fmt.Sprintf(`
			SELECT setval(
				pg_get_serial_sequence('public.%s', 'id'),
				GREATEST(COALESCE((SELECT MAX(id) FROM %s), 0), 1),
				COALESCE((SELECT MAX(id) FROM %s), 0) > 0
			)`, t, t, t)
		if err := db.Exec(sql).Error; err != nil {
			log.Printf("  ⚠️ 对齐 %s 序列失败: %v", t, err)
			continue
		}
		fixed++
	}
	return fixed
}

// loadStatements 按分号切分 SQL 脚本，跳过空行与 -- 注释行。
func loadStatements(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		stmts []string
		buf   strings.Builder
	)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}

		buf.WriteString(line)
		buf.WriteString("\n")

		if strings.HasSuffix(trimmed, ";") {
			s := strings.TrimSpace(buf.String())
			s = strings.TrimSuffix(s, ";")
			if strings.TrimSpace(s) != "" {
				stmts = append(stmts, s)
			}
			buf.Reset()
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return stmts, nil
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
