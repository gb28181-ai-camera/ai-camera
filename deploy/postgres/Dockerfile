# ============================================================================
# AI Camera 数据库镜像 —— PostgreSQL 16 + PostGIS + pgvector
#
# 两个扩展的用途：
#   · PostGIS  —— 设备 GIS 定位（ac_device.latitude/longitude）、
#                 国标移动位置轨迹（ac_gb_mobile_position）的地理查询
#   · pgvector —— 向量检索（AI 风险图片去重/相似检索等）
#
# 为什么需要自定义镜像：没有官方镜像同时提供两者
#   （postgis/postgis 无 pgvector，pgvector/pgvector 无 PostGIS）。
#
# ⚠️ 基底选择是**实测**定的，且与直觉相反：
#   以 `pgvector/pgvector:pg16` 为基底、补装 PostGIS，而**不是**反过来。
#   原因：`postgis/postgis:16-3.4` 基于 Debian **bullseye**，而 PGDG 已停止
#   bullseye 支持 —— 该镜像内的 `/etc/apt/sources.list.d/pgdg.list` 指向
#   `bullseye-pgdg`，已无 Release 文件，导致 `apt-get update` 直接失败
#   （报 `does not have a Release file`），pgvector 装不上，镜像根本构不出来。
#   而 `pgvector/pgvector:pg16` 是 **bookworm**，其 `bookworm-pgdg` 源正常，
#   `postgresql-16-postgis-3` 可正常安装。
#
#   教训：官方镜像的「新旧」不能按版本号猜，必须实测其 Debian codename 与
#   PGDG 源是否仍然有效。
#
# 构建：
#   docker build -t ai-camera-postgres:16 deploy/postgres
# 或直接由 docker-compose.yml 构建，无需手动执行。
#
# 验证（起容器后应两个库都列出 postgis 与 vector）：
#   docker compose exec postgres psql -U aicamera -d ai_camera_dev -c '\dx'
# ============================================================================

FROM pgvector/pgvector:pg16

# PGDG 源已在官方 pgvector 镜像中配置为 bookworm-pgdg（有效）。
# 若构建环境访问 apt.postgresql.org 受限，可在此处换用内网镜像源。
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends postgresql-16-postgis-3; \
    rm -rf /var/lib/apt/lists/*

# 首次初始化时创建扩展与第二个数据库（见 init/01-extensions.sql）。
# 该目录下的脚本仅在数据目录为空（首次启动）时执行一次。
COPY init/ /docker-entrypoint-initdb.d/

# 健康检查：供 docker compose 的 depends_on.condition 使用
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=6 \
    CMD pg_isready -U "${POSTGRES_USER:-aicamera}" -d "${POSTGRES_DB:-ai_camera_dev}" || exit 1
