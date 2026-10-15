#!/bin/bash
# ================================================================
# AI Camera API 文档生成脚本
# 分别从 router/web.go / router/app.go 独立生成 Swagger JSON 文档
# 执行目录: backend/
# 使用方法: cd backend && bash scripts/gen-swagger.sh
# ================================================================

set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

# 统一 swag v2（与 go.mod github.com/swaggo/swag/v2 一致）
# @latest 解析为 v2.0.0（aliyun 镜像下 @v2.0.0 精确版本解析有问题，@latest 可用）
go install github.com/swaggo/swag/v2/cmd/swag@latest
export PATH="$(go env GOPATH)/bin:$PATH"

# 先确保新增依赖进模块缓存，避免 --parseDependency 解析失败
go mod download

echo "=========================================="
echo "  AI Camera API 文档生成"
echo "=========================================="

# ── 1. 从 router/web.go 生成 Web 端 Swagger 文档 ──
echo ""
echo "▶ 步骤 1/2: 生成 Web 端 Swagger 文档..."
# 注意：不能 rm -rf docs/web —— router 通过 swagger.go 引入 docs/web 包，删掉后
# --parseDependency 解析会报 "cannot find all dependencies"。swag 会覆盖输出。
swag init \
  --generalInfo ../../router/web.go \
  --dir internal/api/handler/web \
  --output docs/web \
  --parseDependency \
  --parseInternal \
  --instanceName web

echo "  → docs/web/swagger.json"

# ── 2. 从 router/app.go 生成 App 端 Swagger 文档 ──
echo ""
echo "▶ 步骤 2/2: 生成 App 端 Swagger 文档..."
swag init \
  --generalInfo ../../router/app.go \
  --dir internal/api/handler/app \
  --output docs/app \
  --parseDependency \
  --parseInternal \
  --instanceName app

echo "  → docs/app/swagger.json"

echo ""
echo "=========================================="
echo "  生成完成！"
echo "=========================================="
echo "  Web 端:  http://localhost:8080/swagger-ui       (JSON: /web/api-docs.json)"
echo "  App 端:  http://localhost:8080/app/api-docs.json"
echo ""
echo "  下一步（生成前端 TypeScript 客户端）:"
echo "    npm run dev        # 启动后端"
echo "    cd frontend && bash scripts/api-gen.sh"
