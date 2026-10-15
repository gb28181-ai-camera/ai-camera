# ============================================================================
# AI Camera 前端镜像
# 一个 npm 工程、两个 Web 构建目标：
#   - 管理后台  index.html    → dist/          （base /admin/）
#   - 官网介绍页 website.html → dist-website/  （base /）
# 镜像内由 nginx 同时托管两者：/admin/ → 管理后台，/ → 官网。
# ============================================================================

FROM node:22-alpine AS builder

WORKDIR /app

# 国内镜像加速
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN npm config set registry https://registry.npmmirror.com
ENV ELECTRON_MIRROR=https://npmmirror.com/mirrors/electron/

# 依赖层单独缓存
COPY package.json ./
RUN npm install

COPY . .

# build:admin 含 tsc --noEmit；build:website 仅 vite build
RUN npm run build

# ============================================================
# 生产镜像：nginx:alpine 托管静态文件
# ============================================================
FROM nginx:alpine

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 官网产物放到站点根，管理后台产物放到 /admin 子目录
COPY --from=builder /app/dist-website /usr/share/nginx/html
COPY --from=builder /app/dist /usr/share/nginx/html/admin

COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 8080

CMD ["nginx", "-g", "daemon off;"]
