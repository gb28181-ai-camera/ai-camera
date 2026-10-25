# 用完整版 Python 镜像而不是 -slim：
#   onnxruntime 依赖 libgomp.so.1，slim 镜像没有；在构建期 apt-get 安装会
#   额外依赖 Debian 源可达性（离线/受限网络下会直接构建失败）。
#   完整版镜像自带 GCC 运行库，构建更可复现。
FROM python:3.11

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    EMBEDDING_CACHE_DIR=/data/models

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY main.py .

EXPOSE 8091

HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=20 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8091/health', timeout=3)"

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8091", "--workers", "1"]
