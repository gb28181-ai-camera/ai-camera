#!/bin/bash
# ================================================================
# AI Camera 后端镜像构建与推送
# 用法: cd backend && bash scripts/build.sh [--prod]
#   (无参数) 构建并推送开发镜像
#   --prod   构建并推送生产镜像
# ================================================================

set -e

REGISTRY="dev-registry.boticz.com:5000"
DOCKERFILE="Dockerfile"

if [ "$1" == "--prod" ]; then
  REGISTRY="registry.boticz.cn:5000"
  DOCKERFILE="Dockerfile.prod"
fi

IMAGE_NAME="$REGISTRY/boticz/ai-camera-api:latest"

echo "=========================================="
echo "  AI Camera 后端镜像构建"
echo "  镜像: $IMAGE_NAME"
echo "  Dockerfile: $DOCKERFILE"
echo "=========================================="

docker build -t "$IMAGE_NAME" -f "$DOCKERFILE" .
docker push "$IMAGE_NAME"

echo "✅ 构建并推送完成: $IMAGE_NAME"
