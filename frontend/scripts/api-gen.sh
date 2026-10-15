#!/bin/bash
# =============================================================================
# AI Camera Web - API 代码生成脚本
# 用途：从后端 Swagger 文档生成 TypeScript 接口代码（管理后台使用）
# 执行目录必须是 frontend/
#
# 使用方式:
#   cd frontend
#   bash scripts/api-gen.sh                      # 后端默认 8080
#   BACKEND_URL=http://localhost:8081 bash scripts/api-gen.sh   # 测试环境
#
# 可覆盖的环境变量:
#   BACKEND_URL  后端地址，默认 http://localhost:8080（测试环境用 8081）
#   SWAGGER_FILE 跳过 HTTP 拉取，直接指定 swagger JSON 文件（如
#                ../backend/docs/web/web_swagger.json），用于离线/CI 场景
#
# 生成器选择（自动探测，无需手工指定）:
#   1. PATH 中存在 `swagger2-doc-parser`  → 走原流程（外部 CLI）
#   2. 否则                              → 回退到本仓库自带的
#                                          `node scripts/api-gen-local.mjs`
#                                          参数与 preserve 清单见 scripts/README.md
# =============================================================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  AI Camera Web - API 代码生成${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# 统一的输出目录 / 基址表达式（两种生成器共用）
WEB_OUT="src/admin/api/generated"
BASE_URL_EXPR="import.meta.env.VITE_API_BASE"

# =============================================================================
# Step 1: 取得 Swagger 文档
#   - 显式给了 SWAGGER_FILE  → 直接用（离线/CI）
#   - 否则先检查后端是否运行（curl /web/api-docs.json），再拉取
# =============================================================================
echo -e "${YELLOW}[Step 1/3] 获取 Swagger 文档...${NC}"

mkdir -p scripts
SWAGGER_JSON="scripts/api-docs.json"

if [ -n "${SWAGGER_FILE}" ]; then
    if [ ! -f "${SWAGGER_FILE}" ]; then
        echo -e "  ${RED}✗${NC} SWAGGER_FILE 不存在: ${SWAGGER_FILE}"
        exit 1
    fi
    cp "${SWAGGER_FILE}" "${SWAGGER_JSON}"
    echo -e "  ${GREEN}✓${NC} 使用本地 Swagger 文档: ${SWAGGER_FILE}"
else
    BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
    SWAGGER_URL="${BACKEND_URL}/web/api-docs.json"

    if curl -s -o /dev/null -w "%{http_code}" "$SWAGGER_URL" 2>/dev/null | grep -q "200"; then
        echo -e "  ${GREEN}✓${NC} 后端服务正在运行: $SWAGGER_URL"
    else
        echo -e "  ${RED}✗${NC} 后端服务未启动！请先启动后端:"
        echo -e "    cd backend && go run ./cmd/server/main.go"
        echo -e "  测试环境:"
        echo -e "    export CONFIG_PATH=\"config/config-test.yaml\" && cd backend && go run ./cmd/server/main.go"
        echo -e "  然后重新运行: BACKEND_URL=http://localhost:8081 bash scripts/api-gen.sh"
        echo -e "  （或用 SWAGGER_FILE=../backend/docs/web/web_swagger.json 跳过 HTTP 拉取）"
        exit 1
    fi

    if command -v swagger2-doc-parser >/dev/null 2>&1; then
        swagger2-doc-parser fetch "$SWAGGER_URL" -d "${SWAGGER_JSON}"
    else
        curl -sS "$SWAGGER_URL" -o "${SWAGGER_JSON}"
    fi

    if [ ! -s "${SWAGGER_JSON}" ]; then
        echo -e "  ${RED}✗${NC} 拉取失败: $SWAGGER_URL"
        exit 1
    fi
    echo -e "  ${GREEN}✓${NC} Swagger 文档已保存到: ${SWAGGER_JSON}"
fi

# =============================================================================
# Step 2: 选择生成器
# =============================================================================
echo -e "${YELLOW}[Step 2/3] 选择代码生成器...${NC}"

if command -v swagger2-doc-parser >/dev/null 2>&1; then
    GENERATOR="swagger2-doc-parser"
    echo -e "  ${GREEN}✓${NC} 检测到外部 CLI: $(command -v swagger2-doc-parser)"
else
    GENERATOR="local"
    echo -e "  ${YELLOW}!${NC} 未在 PATH 中找到 swagger2-doc-parser，回退到仓库自带的本地生成器:"
    echo -e "      node scripts/api-gen-local.mjs"
    echo -e "    （如需使用外部 CLI，请先安装 swagger2-doc-parser）"
fi

# =============================================================================
# Step 3: 生成 TypeScript 代码（管理后台）
# =============================================================================
echo -e "${YELLOW}[Step 3/3] 生成 TypeScript 代码（Web 端）...${NC}"

if [ "${GENERATOR}" = "swagger2-doc-parser" ]; then
    swagger2-doc-parser gen \
        -c scripts/swagger-config-typescript.js \
        -o "${WEB_OUT}" \
        -d "${SWAGGER_JSON}" \
        -h "${BASE_URL_EXPR}"
else
    node scripts/api-gen-local.mjs \
        --input "${SWAGGER_JSON}" \
        --out "${WEB_OUT}" \
        --base-url "${BASE_URL_EXPR}" \
        --manifest scripts/api-gen-preserve.mjs \
        --clean
fi

echo -e "  ${GREEN}✓${NC} TypeScript 代码已生成到: ${WEB_OUT}/"

# =============================================================================
# TODO(App 端 / Flutter): 尚未实现 Dart 生成器
# -----------------------------------------------------------------------------
# App 端 Swagger 文档位于   backend/docs/app/app_swagger.json
# （后端运行时可从    ${BACKEND_URL}/app/api-docs.json   取）
# 目标输出目录             app/lib/services/generated/
#
# 目前 app/ 下的 Dart 客户端仍是手写，本次范围不含 Dart 代码生成。
# 后续实现方式建议与 Web 端一致：新增 scripts/api-gen-local-dart.mjs，
# 在本脚本 Step 3 之后追加一段：
#
#   if [ "${GENERATOR}" = "local" ]; then
#       node scripts/api-gen-local-dart.mjs \
#           --input "${APP_SWAGGER_JSON}" \
#           --out ../app/lib/services/generated
#   fi
# =============================================================================
echo -e "  ${CYAN}注${NC} App 端（Flutter/Dart）生成器尚未实现，见本脚本末尾 TODO。"

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  代码生成完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "生成的接口代码使用 '../request' 发送请求（已有 axios + JWT 拦截器）"
echo "注意: 生成后请勿手动修改 generated/ 目录下的文件。"
if [ "${GENERATOR}" = "local" ]; then
    echo "需要保留的手写方法/别名请登记到 scripts/api-gen-preserve.mjs（详见 scripts/README.md）。"
fi
