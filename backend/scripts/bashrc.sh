export PATH=$PATH:/usr/local/go/bin
export GOPATH=/home/neme/go
export PATH=$PATH:$GOPATH/bin
# CI runner 主机连不上 proxy.golang.org，统一走阿里云镜像
export GOPROXY=https://mirrors.aliyun.com/goproxy/,direct
export GOSUMDB=off
