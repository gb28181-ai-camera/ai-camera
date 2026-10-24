# AI Camera MQTT broker（mochi-mqtt + 后端 RPC 鉴权）
FROM golang:1.23-alpine AS builder

ENV GOPROXY=https://mirrors.aliyun.com/goproxy/,direct
ENV GOSUMDB=off
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /out/mochi-mqtt .

FROM alpine:3.20
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata wget
ENV TZ=Asia/Shanghai
COPY --from=builder /out/mochi-mqtt /usr/local/bin/mochi-mqtt

# 1883 MQTT TCP；8083 MQTT over WebSocket
EXPOSE 1883 8083
ENTRYPOINT ["/usr/local/bin/mochi-mqtt"]
