// dbtool 数据库诊断小工具：执行任意 SQL 并打印结果。
//
//	cd backend && go run ./cmd/dbtool "SELECT * FROM ac_user LIMIT 5"
//	cd backend && go run ./cmd/dbtool -f script.sql
//
// 连接参数取 TEST_DB_* 环境变量，缺省指向 ai_camera_test。
// 出于安全考虑，**拒绝连接 ai_camera_dev 以外的生产库**之外的一切连接需要显式指定 -allow-any。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	file := flag.String("f", "", "从文件读取 SQL")
	allowAny := flag.Bool("allow-any", false, "允许连接非测试库")
	flag.Parse()

	dbname := env("TEST_DB_NAME", "ai_camera_test")
	if !*allowAny && dbname != "ai_camera_test" {
		log.Fatalf("拒绝执行：dbtool 默认仅允许 ai_camera_test，当前为 %s（如需绕过请加 -allow-any）", dbname)
	}

	// 密码不给默认值：避免明文凭据入库，缺失时直接失败并提示
	password := os.Getenv("TEST_DB_PASSWORD")
	if password == "" {
		log.Fatal("未设置环境变量 TEST_DB_PASSWORD（为避免密码进入版本库，此处不再提供默认值）")
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
		env("TEST_DB_HOST", "192.168.1.227"),
		envInt("TEST_DB_PORT", 5432),
		env("TEST_DB_USER", "boticz"),
		password,
		dbname,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	fmt.Printf("已连接 %s\n\n", dbname)

	var sqlText string
	if *file != "" {
		b, err := os.ReadFile(*file)
		if err != nil {
			log.Fatalf("读取文件失败: %v", err)
		}
		sqlText = string(b)
	} else {
		sqlText = strings.Join(flag.Args(), " ")
	}
	if strings.TrimSpace(sqlText) == "" {
		log.Fatal("用法: go run ./cmd/dbtool \"SELECT ...\" 或 go run ./cmd/dbtool -f script.sql")
	}

	// 查询类语句走 Rows()，其余走 Exec()
	if isQuery(sqlText) {
		rows, err := db.Raw(sqlText).Rows()
		if err != nil {
			log.Fatalf("查询失败: %v", err)
		}
		defer rows.Close()

		cols, _ := rows.Columns()
		fmt.Println(strings.Join(cols, " | "))

		count := 0
		for rows.Next() {
			vals := make([]interface{}, len(cols))
			ptrs := make([]interface{}, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				log.Fatalf("扫描失败: %v", err)
			}
			out := make([]string, len(cols))
			for i, v := range vals {
				if v == nil {
					out[i] = "<NULL>"
					continue
				}
				switch t := v.(type) {
				case []byte:
					out[i] = string(t)
				default:
					out[i] = fmt.Sprintf("%v", t)
				}
			}
			fmt.Println(strings.Join(out, " | "))
			count++
		}
		fmt.Printf("\n(%d 行)\n", count)
		return
	}

	if err := db.Exec(sqlText).Error; err != nil {
		log.Fatalf("执行失败: %v", err)
	}
	fmt.Println("✅ 执行成功")
}

// isQuery 粗略判断是否为查询语句。
func isQuery(sql string) bool {
	s := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(s, "SELECT") || strings.HasPrefix(s, "WITH") || strings.HasPrefix(s, "SHOW")
}

// 保证 bufio 被引用（保留逐行执行扩展点）。
var _ = bufio.NewReader

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
