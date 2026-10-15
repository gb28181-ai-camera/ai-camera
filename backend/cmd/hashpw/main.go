// hashpw 生成 bcrypt 密码哈希，用于初始化数据脚本或重置用户密码。
//
//	cd backend && go run ./cmd/hashpw admin123
//	cd backend && go run ./cmd/hashpw -verify '$2a$10$...' admin123
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	verify := flag.String("verify", "", "校验模式：给定哈希，判断后面的密码是否匹配")
	flag.Parse()

	args := flag.Args()

	if *verify != "" {
		if len(args) == 0 {
			log.Fatal("用法: go run ./cmd/hashpw -verify '<hash>' <password>")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(*verify), []byte(args[0])); err != nil {
			fmt.Println("❌ 不匹配")
			os.Exit(1)
		}
		fmt.Println("✅ 匹配")
		return
	}

	if len(args) == 0 {
		log.Fatal("用法: go run ./cmd/hashpw <password>")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(args[0]), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("生成失败: %v", err)
	}
	fmt.Println(string(hash))
}
