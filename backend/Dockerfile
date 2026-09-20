# Fast Pay 后端：单二进制 epay（含内嵌 SQLite，纯 Go 驱动可静态编译）。
# 构建：在仓库根执行  docker build -f backend/Dockerfile -t epay-backend .

# ---- 编译 ----
FROM golang:1.26-bookworm AS build
WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/epay .

# ---- 运行 ----
FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates tzdata \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/epay /usr/local/bin/epay

# /data 挂载卷：config.json（首次启动自动生成）与 funpay.db 都落这里
WORKDIR /data
ENV EPAY_CONFIG=/data/config.json
EXPOSE 9200
CMD ["epay"]
