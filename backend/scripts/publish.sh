#!/bin/bash
# ================================================================
# AI Camera 后端 K8s 滚动重启
# 用法: cd backend && bash scripts/publish.sh [--prod]
# ================================================================

set -e

DEPLOYMENT_NAME="ai-camera-api"
NAMESPACE="boticz"
CONTEXT="kubernetes-admin@kubernetes-dev"

if [ "$1" == "--prod" ]; then
  CONTEXT="kubernetes-admin@kubernetes-prod"
fi

echo "=========================================="
echo "  发布 $DEPLOYMENT_NAME (namespace: $NAMESPACE)"
echo "  集群上下文: $CONTEXT"
echo "=========================================="

kubectl config use-context "$CONTEXT"
kubectl rollout restart deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE"
kubectl wait --for=condition=available --timeout=60s deployment/"$DEPLOYMENT_NAME" -n "$NAMESPACE"

echo "✅ 发布完成"
