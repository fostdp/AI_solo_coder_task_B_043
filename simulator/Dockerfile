# =========================================
# XRF 模拟器 Dockerfile
# 构建上下文是项目根目录，不是 ./simulator
# =========================================

FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /src

COPY simulator/go.mod simulator/go.sum* ./
RUN go mod download || true

COPY simulator/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o simulator .

# =========================================
# 运行时镜像
# =========================================
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata curl
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /src/simulator .

# 健康检查
HEALTHCHECK --interval=60s --timeout=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider ${API_BASE:-http://backend:8080}/api/stats || exit 1

CMD ["./simulator"]
