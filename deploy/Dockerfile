# FastLedger 单镜像（WBS 7.1）。
#
# 两个硬约束决定本文件的形态：
#  1. mattn/go-sqlite3 **需要 CGO**——必须用带 gcc 的构建镜像，且运行时
#     glibc/musl 必须与构建镜像一致：这里 builder 与 runtime 同用 alpine（musl），
#     避免常见的"bookworm 构建 → alpine 运行"链接失败；
#  2. 时区口径：服务端月份边界/导出固定 CST（time.FixedZone，UTC+8），
#     不读系统 tzdata——镜像无需装时区包，部署环境无关。
#
# 构建上下文 = 仓库根（server/ 是 module 根，web 静态资源经 go:embed 内嵌）：
#   docker build -f deploy/Dockerfile -t fastledger:latest .
#
# GOPROXY 指向国内镜像：CI/本地构建环境差异下保证依赖可拉取。

# ---- 构建阶段 ----
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache gcc musl-dev
ENV CGO_ENABLED=1 \
    GOOS=linux \
    GOPROXY=https://goproxy.cn,direct

WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
RUN go build -trimpath -ldflags "-s -w" -o /out/fastledger ./cmd/server

# ---- 运行阶段 ----
FROM alpine:3.20

# ca-certificates：HTTPS 出站（WBS 5.8 LLM 兜底）；wget：healthcheck（busybox 自带）
RUN apk add --no-cache ca-certificates && \
    addgroup -S fl && adduser -S fl -G fl

COPY --from=builder /out/fastledger /usr/local/bin/fastledger

# 数据目录：挂卷持久化；属主给运行用户（umask 由应用自收紧为 077）
RUN mkdir -p /data && chown fl:fl /data
VOLUME ["/data"]
USER fl

ENV EBK_HTTP_PORT=8080 \
    EBK_DB_PATH=/data/fastledger.db \
    EBK_DATA_DIR=/data
EXPOSE 8080

# healthz：进程内自检 + schemaVersion/restoreEpoch 暴露
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1 || exit 1

# 优雅关机：应用监听 SIGTERM 并 srv.Shutdown（main.go）；
# compose 侧 stop_grace_period: 30s——SQLite 关闭最后连接时自动 WAL checkpoint，
# WAL 中未 checkpoint 的已提交事务在进程正常退出时落回主库（WBS 7.1 约束）。
#
# 启动即默认 HTTP 服务（无参数）；子命令容器化用法：
#   docker run --rm -v fl-data:/data fastledger db migrate
ENTRYPOINT ["fastledger"]
