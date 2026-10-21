FROM python:3.11-slim

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

# ffmpeg：Pipecat 常用（音频重采样/转码）；curl：容器 healthcheck。
RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 先装依赖，利用镜像层缓存
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 8090

# /health 无需鉴权，容器内自检即可
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8090/health || exit 1

# 单进程即可：控制面内存态（配置/会话）不做多副本共享。
# 需要多副本时必须先把会话与配置外置到 Redis，本期不做。
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8090"]
