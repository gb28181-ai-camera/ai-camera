#!/bin/sh
# ZLMediaKit 配置渲染入口（K8s 版）。
#
# 与 deploy/zlm/render-config.sh 的差异：
#   - 模板与输出路径可通过环境变量指定。K8s 的 ConfigMap 卷是**只读**的，
#     不能像 compose 那样把渲染结果写回模板所在目录；
#   - 因此模板挂到 /opt/media/tmpl（只读），输出写到 emptyDir /opt/media/conf。
#
# 为什么仍需这一层：ZLM 的 config.ini **不支持环境变量替换**（实测 ${VAR}
# 会原样保留），而 secret 与 hook 地址必须随环境变化。静态写死会导致
# 「本地联调正常、集群部署全部回调 401」。
set -eu

TPL="${ZLM_CONFIG_TPL:-/opt/media/tmpl/config.tmpl}"
OUT="${ZLM_CONFIG_OUT:-/opt/media/conf/config.ini}"

: "${ZLM_SECRET:?ZLM_SECRET 未设置（应与 ac_gb_media_server.secret 一致）}"
HOOK_BASE="${ZLM_HOOK_BASE:-http://ai-camera-backend:8080}"

# 只允许安全字符，避免 awk 替换语义（& 代表匹配串）与 URL 编码问题。
case "$ZLM_SECRET" in
  *[!A-Za-z0-9._-]*)
    echo "[render-config] ZLM_SECRET 含非法字符，仅允许 A-Za-z0-9 . _ -" >&2
    exit 1
    ;;
esac
case "$HOOK_BASE" in
  http://*|https://*) ;;
  *)
    echo "[render-config] ZLM_HOOK_BASE 必须以 http:// 或 https:// 开头" >&2
    exit 1
    ;;
esac

if [ ! -f "$TPL" ]; then
  echo "[render-config] 模板不存在: $TPL" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUT")"
awk -v secret="$ZLM_SECRET" -v hook="$HOOK_BASE" '
  { gsub(/__ZLM_SECRET__/, secret); gsub(/__HOOK_BASE__/, hook); print }
' "$TPL" > "$OUT"

echo "[render-config] 已生成 $OUT（hook base=$HOOK_BASE）"

# 工作目录沿用镜像默认的 /opt/media/bin，相对路径（default.pem、./www 等）不变。
exec ./MediaServer -s default.pem -c "$OUT" -l 0
