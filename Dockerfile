# syntax=docker/dockerfile:1

# ---------- 构建阶段 ----------
FROM golang:1.25-alpine AS builder
WORKDIR /app

# 先复制依赖清单，充分利用构建缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并静态编译（纯 Go SQLite 驱动，无需 CGO）
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/blacklist-index .

# ---------- 运行阶段 ----------
FROM alpine:3.20
# tzdata 用于时区加载，ca-certificates 用于外链/HTTPS 校验
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/blacklist-index /app/blacklist-index
RUN mkdir -p /data && chown -R app:app /data /app

# 以非 root 用户运行
USER app

ENV DATA_DIR=/data \
    PORT=8080 \
    TIMEZONE=Asia/Shanghai

VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["/app/blacklist-index"]