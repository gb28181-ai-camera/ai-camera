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
RUN go build -ldflags="-w -s" -o /app/ai-camera-api ./cmd/server/main.go

# ── 运行阶段 ──
FROM alpine:latest

# 切换 apk 镜像源并安装运行时依赖
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata wget

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/ai-camera-api .
COPY --from=builder /build/static ./static
COPY --from=builder /build/docs ./docs
COPY --from=builder /build/config/rbac_model.conf ./config/rbac_model.conf

EXPOSE 8080

ENTRYPOINT ["/app/ai-camera-api"]
