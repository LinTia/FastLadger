# FastLedger 部署与运维手册（WBS 7.1–7.3）

> 生产参考环境：腾讯云轻量 2C2G（Ubuntu 24.04），应用用户 `fastledger`，
> 数据 `/opt/fastledger/data`（0700/0600），systemd `fastledger`，schema v8。

## 1. 部署单元（WBS 7.1）

### 1.1 Docker 单镜像

```bash
# 构建上下文 = 仓库根（web 静态资源经 go:embed 内嵌进二进制）
docker build -f deploy/Dockerfile -t fastledger:latest .

# 启动（数据持久化于命名卷 fl-data）
docker compose -f deploy/docker-compose.yml up -d

# 迁移 / 运维子命令（容器化）
docker run --rm -v fl-data:/data fastledger:latest db migrate
```

- 镜像：多阶段构建，builder 与 runtime 同为 alpine（musl）——`mattn/go-sqlite3`
  需要 CGO，glibc/musl 必须一致；
- `stop_grace_period: 30s`：SIGTERM → `srv.Shutdown`（10s 超时）→ SQLite 关闭
  最后连接时自动 WAL checkpoint，未落盘事务回写主库；
- `HEALTHCHECK`：`/healthz`（含 schemaVersion / restoreEpoch）；
- 非 root 运行（fl 用户），应用启动自收紧 umask 077。

### 1.2 外网 HTTPS（需要域名，当前未启用）

生产服务器为腾讯云轻量**固定公网 IP**，尚未提供域名——Caddy 服务声明在
`profiles: ["tls"]` 下，不阻塞默认部署。域名确定后：

```bash
CADDY_DOMAIN=ledger.example.com docker compose -f deploy/docker-compose.yml --profile tls up -d
```

Caddy 向 Let's Encrypt 自动签发/续期证书，`--from <域名> --to app:8080` 反代。

### 1.3 systemd 直跑（当前生产形态）

当前生产未用容器（服务器未安装 docker），systemd 直跑 + 二进制替换部署，
口径与容器一致：`Restart=on-failure`、`TimeoutStopSec=30`（等效 stop_grace_period）。

## 2. 每日备份（WBS 7.2 / PRD §12 决策#5）

- `deploy/backup.sh`：SQLite **online backup API**（等价 `.backup`），
  只读 URI 打开源库，与应用并发安全；**绝不做裸文件复制**
  （WAL 未 checkpoint 的已提交事务会丢，WBS 7.2 红线）；
- 调度：`/etc/cron.d/fastledger`（root）：

```
10 3 * * * root su -s /bin/bash fastledger -c "set -a; . /opt/fastledger/env; /opt/fastledger/fastledger db purge" >> /var/log/fastledger-purge.log 2>&1
30 3 * * * root /opt/fastledger/backup.sh >> /var/log/fastledger-backup.log 2>&1
```

- 保留：每日 1 份，**滚动保留最近 7 份**（PRD §12 决策#5；原 30 天口径已按 PRD 收紧）；
- 备份目录：`/var/backups/fastledger/db-YYYY-MM-DD.db`。

## 3. 恢复演练（WBS 7.3 / PRD §10.9，每月一次）

> ⚠️ 两个实战验证过的坑（2026-11-02 演练实测）：
> 1. `instance bump` 会把"备份恢复"审计事件写入 `EBK_DB_PATH` 指向的库——
>    **必须同时传 `EBK_DB_PATH` 与 `EBK_DATA_DIR`**，否则审计静默落到 cwd 的
>    新空库（仅 stderr 警告，不失败）；
> 2. 正确顺序是 **恢复 → bump → 再启动**：restoreEpoch 在实例启动时读入内存，
>    先启动后 bump 的话 healthz 仍显示旧值（客户端全量重建信号要到下次重启才可见）。

步骤（全部在**不影响生产**的前提下进行，端到端起临时实例）：

```bash
# 1) 取最新备份恢复到临时目录
D=/var/backups/fastledger; B=$(ls -1 $D/db-*.db | sort | tail -1)
T=/tmp/drill-$(date +%s); mkdir -p $T/data
cp "$B" $T/data/fastledger.db && chmod 600 $T/data/fastledger.db
chown -R fastledger:fastledger $T    # 临时实例以 fastledger 运行，目录必须可写（WAL/instance.meta）

# 2) 递增 restore_epoch（恢复后、启动前）——必须带 EBK_DB_PATH
sudo -u fastledger env EBK_DB_PATH=$T/data/fastledger.db EBK_DATA_DIR=$T/data \
  /opt/fastledger/fastledger instance bump

# 3) 起临时实例（不同端口 + 临时数据目录 + 临时 JWT secret）
sudo -u fastledger env EBK_HTTP_PORT=18080 EBK_DB_PATH=$T/data/fastledger.db \
  EBK_DATA_DIR=$T/data EBK_JWT_SECRET=drill-secret-0123456789abcdef012345 \
  /opt/fastledger/fastledger &   # 记 PID

# 4) 校验：healthz 应报 up + restoreEpoch=2（若备份时为 1）+ schemaVersion；
#    记录数与生产核对（users/books/book_members/transactions/audit_logs/
#    token_sessions）——audit_logs 允许恢复库 = 生产 + 1（bump 落的"备份恢复"审计）
#    抽查审计可读性（SELECT COUNT(1), MAX(at) FROM audit_logs）

# 5) 停止并清理：kill <PID>; rm -rf $T；确认生产 healthz 的 restoreEpoch 未变
```

**验收**：healthz up、记录数一致（审计 +1 为恢复事件）、restore_epoch 递增、
审计可读可追溯。演练记录归档于 `docs/FastLedger-阶段7-运维与验收报告.md`
（每次演练追加一行）。

## 4. 测试与验收（WBS 7.4 / 7.5）

- `go test -race` 全量在 GitHub CI（Linux runner 自带 gcc）+ 服务器 run_tests.sh；
- §10.1–§10.9 端到端验收映射与证据见 `docs/FastLedger-阶段7-运维与验收报告.md`。
