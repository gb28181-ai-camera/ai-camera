#!/bin/sh
# ZLMediaKit 配置渲染入口。
#
# 为什么需要这一层：
#   ZLM 的 config.ini **不支持环境变量替换**（实测把 `${VAR}` 原样保留），
#   而 secret 与 hook 回调地址必须随部署环境变化。静态写死会出现
#   「本地联调正常、容器部署全部回调 401」这种很难定位的问题。
#
# 输入：
#   /opt/media/conf/config.tmpl   （compose 挂载自 deploy/zlm/config.ini，只读）
#   ZLM_SECRET                    必填，与 ac_gb_media_server.secret 一致
#   ZLM_HOOK_BASE                 可选，平台回调基址，默认 host.docker.internal:8081
# 输出：
#   /opt/media/conf/config.ini    （容器可写层，每次启动重新生成）
#
# 注意：输出**不能**写回模板挂载点 —— 那会把当次环境的密钥写进仓库文件。
set -eu

TPL=/opt/media/conf/config.tmpl
OUT=/opt/media/conf/config.ini

: "${ZLM_SECRET:?ZLM_SECRET 未设置（应与 ac_gb_media_server.secret 一致）}"
HOOK_BASE="${ZLM_HOOK_BASE:-http://host.docker.internal:8081}"

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

awk -v secret="$ZLM_SECRET" -v hook="$HOOK_BASE" '
  { gsub(/__ZLM_SECRET__/, secret); gsub(/__HOOK_BASE__/, hook); print }
' "$TPL" > "$OUT"

echo "[render-config] 已生成 $OUT（hook base=$HOOK_BASE）"

# 工作目录沿用镜像默认的 /opt/media/bin，相对路径（default.pem、./www 等）不变。
exec ./MediaServer -s default.pem -c "$OUT" -l 0
