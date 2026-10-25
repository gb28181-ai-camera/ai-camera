# AI Camera 后端服务 - 开发环境镜像
# ── 构建阶段 ──
FROM golang:1.26-alpine AS builder

# CI runner 连不上 proxy.golang.org，走阿里云镜像
ENV GOPROXY=https://mirrors.aliyun.com/goproxy/,direct
ENV GOSUMDB=off

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV GOOS=linux
ENV CGO_ENABLED=0
ENV GOARCH=amd64
# 同一镜像内产出两个入口：HTTP API 与 SIP 信令服务。
# 二者共享 model/repository/service，但扩缩容诉求相反（见 docker-compose.yml 注释）。
RUN go build -ldflags="-w -s" -o /app/ai-camera-api ./cmd/server/main.go && \
    go build -ldflags="-w -s" -o /app/ai-camera-sipsvc ./cmd/sipsvc/main.go

# ── 运行阶段 ──
FROM alpine:latest

# 切换 apk 镜像源并安装运行时依赖。
# ffmpeg 供「云端录像多段合并」使用（-c copy 无损拼接）；缺少时合并任务
# 会明确落为 failed，但镜像自带可避免部署后才发现能力缺失。
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata wget ffmpeg

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/ai-camera-api .
COPY --from=builder /app/ai-camera-sipsvc .
COPY --from=builder /build/static ./static
COPY --from=builder /build/docs ./docs
COPY --from=builder /build/config/rbac_model.conf ./config/rbac_model.conf

EXPOSE 8080

ENTRYPOINT ["/app/ai-camera-api"]
