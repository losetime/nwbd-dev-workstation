# ---------- 构建阶段 ----------
FROM golang:1.26-alpine AS build

# 模块代理：国内构建默认走 goproxy.cn，可覆盖
ARG GO_PROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOPROXY=${GO_PROXY}

WORKDIR /src

# 先拉依赖，源码变化时复用依赖层
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && go build -trimpath -ldflags="-s -w" -o /out/consumer ./cmd/consumer

# ---------- 运行阶段 ----------
FROM alpine:3.21

ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
RUN sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${ALPINE_MIRROR}|g" /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Shanghai
WORKDIR /app

COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/consumer /usr/local/bin/consumer
COPY entrypoint.sh /app/

RUN chmod +x /app/entrypoint.sh

EXPOSE 8000

# 静态二进制 + busybox wget，无需额外工具
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8000/api/health || exit 1

ENTRYPOINT ["/app/entrypoint.sh"]
