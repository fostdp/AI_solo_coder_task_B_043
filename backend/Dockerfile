# =========================================
# 后端服务 Dockerfile
# 注意：构建上下文是项目根目录，不是 ./backend
# 构建命令: docker build -f backend/Dockerfile .
# =========================================

FROM golang:1.21-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /src

# 复制 go.mod 和下载依赖（利用缓存层）
COPY backend/go.mod backend/go.sum* ./
RUN go mod download || true

# 复制后端源码
COPY backend/ .

# 构建（禁用 CGO，静态链接）
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .

# =========================================
# 运行时镜像（最小化，alpine 约 20MB）
# =========================================
FROM alpine:latest

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app

# 复制二进制和配置
COPY --from=builder /src/server .
COPY --from=builder /src/.env* ./

# 复制前端静态资源（Go 内建静态文件服务兼容旧链接）
COPY frontend/ /frontend/

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# 暴露端口
EXPOSE 8080 6060 2112

CMD ["./server"]
