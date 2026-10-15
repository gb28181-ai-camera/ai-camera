# ============================================================================
# AI Camera 数据库镜像 —— PostgreSQL 16 + PostGIS + pgvector
#
# 两个扩展的用途：
#   · PostGIS  —— 设备 GIS 定位（ac_device.latitude/longitude）、
#                 国标移动位置轨迹（ac_gb_mobile_position）的地理查询
#   · pgvector —— 向量检索（AI 风险图片去重/相似检索等）
#
# 为什么需要自定义镜像：官方 postgis/postgis 不含 pgvector，
# 官方 pgvector/pgvector 不含 PostGIS，没有一个官方镜像同时提供两者。
# 这里以 postgis 为基底，从 PGDG 源补装 pgvector（postgis 镜像已配好 PGDG 源）。
#
# 构建：
#   docker build -t ai-camera-postgres:16 deploy/postgres
# 或直接由 docker-compose.yml 构建，无需手动执行。
# ============================================================================

FROM postgis/postgis:16-3.4

# PGDG 源已在官方 postgis 镜像中配置，postgresql-16-pgvector 由该源提供。
# 若构建环境访问 deb.debian.org / apt.postgresql.org 受限，可在此处换用内网镜像源。
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends postgresql-16-pgvector; \
    rm -rf /var/lib/apt/lists/*

# 首次初始化时创建扩展与第二个数据库（见 init/01-extensions.sql）。
# 该目录下的脚本仅在数据目录为空（首次启动）时执行一次。
COPY init/ /docker-entrypoint-initdb.d/

# 健康检查：供 docker compose 的 depends_on.condition 使用
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=6 \
    CMD pg_isready -U "${POSTGRES_USER:-aicamera}" -d "${POSTGRES_DB:-ai_camera_dev}" || exit 1
