# FastLedger 部署交接说明（腾讯云轻量应用服务器）

> **交接对象**：腾讯云轻量应用服务器（Lighthouse）部署专家
> **交接人**：开发方
> **代码基线**：`main` @ `1a5429d`（CI 全绿；本地另有 2 个纯注释提交待推）
> **文档版本**：2026-10-31 · 依据代码实测核对，非设计稿口径

---

## 0. 一页速览（先看这张表）

| 维度 | 结论 |
|---|---|
| 运行形态 | **单个 Go 二进制 + 单个 SQLite 文件**，无 Redis / 无 MySQL / 无消息队列 / 无外部服务依赖 |
| 构建依赖 | Go 1.22+、**CGO + C 编译器（gcc）**、需要能下载 Go module |
| 运行时依赖 | 无（除 glibc 与数据目录读写权限） |
| 对外端口 | 应用自身只监听 **:8080（HTTP，无 TLS）**；对外须由反代提供 **80/443** |
| HTTPS | **应用不内置 TLS**，必须由 Caddy / Nginx 终结 |
| 数据库 | SQLite（WAL + `synchronous=FULL` + `MaxOpenConns=1`），文件即全部状态 |
| 密钥 | 仅 1 个必需密钥：`EBK_JWT_SECRET`（≥32 字符、随机、拒绝占位值） |
| 最低规格 | 1 vCPU / 1 GB / 40 GB SSD（**能跑**，但构建期吃内存，见 §8） |
| 推荐规格 | 2 vCPU / 2 GB / 60~80 GB SSD |
| 数据目录 | `${EBK_DATA_DIR}` 默认 `./data`，权限强制 **0700**，db/wal/shm 强制 **0600** |
| 首次部署关键点 | **必须先 `db migrate` 再启动服务**（服务启动只做只读校验，绝不自动建表/改表） |
| 最大风险 | **应用硬编码"不信任任何代理"，放在反代后面会让所有外网用户共用一个限流桶** —— 见 §11-P0 |

---

## 1. 运行形态

```
             80/443 (HTTPS)
   客户端 ──────────────────▶  Caddy / Nginx（同机或同 VPC）
                                   │  http://127.0.0.1:8080
                                   ▼
                        fastledger（单进程，:8080）
                                   │
                        ┌──────────┴──────────┐
                        ▼                     ▼
                 data/fastledger.db      data/instance.meta
                 (+ .db-wal / .db-shm)   (恢复世代标记)
```

要点：

- **单进程、单文件数据库**：进程本身无状态，所有状态在 `data/` 目录；备份 = 备份该目录（排除 `instance.meta`，见 §7）。
- **REST + WebSocket 共用同一端口**：REST 在 `/api/v1/*`，WS 在 `/api/v1/ws`。
- **无静态前端资源**：后端未嵌入 Web 前端（阶段 5 才做）；当前访问 `/` 无页面，属预期。
- **无 Dockerfile**：仓库无容器构建文件（原设计文档提过 docker-compose，但**未落地**，勿按设计稿执行）。

---

## 2. 环境与依赖

### 2.1 构建环境（编译机上）

| 项 | 要求 | 说明 |
|---|---|---|
| Go | **1.22+** | 以 `server/go.mod` 为准 |
| CGO | **必须 `CGO_ENABLED=1`** | `mattn/go-sqlite3` 强依赖 CGO |
| C 编译器 | gcc（Linux：`build-essential` / `gcc`） | 缺 gcc 时 Go 会静默降级 `CGO_ENABLED=0` |
| 网络 | 可访问 Go module 代理 | 国内建议 `GOPROXY=https://goproxy.cn,direct` |
| 内存 | 构建峰值约 1~2 GB | 1 GB 机器编译可能 OOM，见 §8 |

> **务必知道的坑**：`CGO_ENABLED=0` 时**构建能成功、二进制能产出**，但 SQLite 是 stub，运行即失败：
> `Binary was compiled with 'CGO_ENABLED=0', go-sqlite3 requires cgo to work`。
> **判据：能编译 ≠ 能跑**。验收必须看到 `db migrate` 成功 + `/healthz` 返回 `schemaVersion`。

### 2.2 运行时依赖

| 项 | 要求 |
|---|---|
| 操作系统 | Linux x86_64（与构建机同架构同 glibc 族；跨发行版尽量同版本） |
| 权限 | 数据目录读写 + 可执行文件执行；无需 root 运行（建议专用用户） |
| 时间 | 系统时钟需同步（NTP），JWT 过期、审计时间戳、限流窗口均依赖本地时钟 |

### 2.3 第三方 Go 依赖（无需单独安装，随二进制静态链接）

`gin v1.10.0`、`xorm v1.3.9`、`gorilla/websocket v1.5.1`、`mattn/go-sqlite3`（CGO）、`golang.org/x/crypto`（bcrypt）。
除 SQLite 的 libc 依赖外，均为 Go 侧链接，不需要在服务器上装库。

---

## 3. 编译与启动

### 3.1 构建

```bash
# 在服务器上（或同架构构建机）执行
cd server
export CGO_ENABLED=1
go build -trimpath -ldflags "-s -w" -o fastledger ./cmd/server
```

- 不建议交叉编译：CGO + sqlite 交叉编译需目标平台 gcc 工具链，成本高于在服务器上直接构建。
- 产物大小约 20~30 MB（含 sqlite 与全部依赖，以实际为准）。

### 3.2 CLI 子命令（全部为同一二进制）

| 命令 | 作用 | 是否需要 JWT |
|---|---|---|
| `./fastledger db migrate` | **应用迁移**（首次部署必须先跑） | 否 |
| `./fastledger db purge [--dry-run]` | 硬清理回收站（默认 TTL 30 天）与过期审计 | 否 |
| `./fastledger instance bump` | 恢复备份后递增 `restore_epoch` | 否 |
| `./fastledger user create --username <名字> --password <口令>` | 建首个账号 | 否 |
| `./fastledger`（无参数） | 启动 HTTP 服务 | **是**（缺 JWT 直接拒绝启动） |

### 3.3 首次启动顺序（顺序错会失败）

```bash
export EBK_JWT_SECRET="$(openssl rand -base64 48)"   # ≥32 字符，否则拒绝启动
./fastledger db migrate        # 1) 建库建表（幂等，可重跑）
./fastledger user create --username admin --password '<强口令>'   # 2) 建账号
./fastledger                   # 3) 启动服务
```

> **为什么必须显式 migrate**：服务启动只做**只读**版本校验，未迁移会直接退出并提示先执行 `db migrate`。
> 这是刻意设计（避免启动时隐式改表），不是配置错误。

### 3.4 systemd 单元（建议）

```ini
[Unit]
Description=FastLedger
After=network-online.target

[Service]
User=fastledger
WorkingDirectory=/opt/fastledger
EnvironmentFile=/etc/fastledger/env
ExecStart=/opt/fastledger/fastledger
Restart=always
RestartSec=3
# 必须留出时间让 SQLite 做 WAL checkpoint
KillSignal=SIGTERM
TimeoutStopSec=30
# 数据目录在 /var/lib/fastledger
ReadWritePaths=/var/lib/fastledger

[Install]
WantedBy=multi-user.target
```

> 服务已实现 SIGINT/SIGTERM 优雅关闭。**禁止直接 SIGKILL**，否则 WAL 未 checkpoint，异常恢复时需依赖 SQLite 自身回放。

---

## 4. 端口与网络访问

| 端口 | 归属 | 暴露范围 | 说明 |
|---|---|---|---|
| **8080/tcp** | fastledger | **仅 127.0.0.1 / 内网** | 应用 HTTP + WebSocket；**绝不能对公网开放**（应用无 TLS） |
| **80/tcp** | Caddy/Nginx | 公网（或仅用于证书 HTTP-01） | 跳转 HTTPS / ACME 挑战 |
| **443/tcp** | Caddy/Nginx | 公网 | HTTPS 终结，反代到 127.0.0.1:8080 |
| 22/tcp | SSH | **仅限运维 IP 白名单** | 腾讯云轻量防火墙侧限制 |

轻量应用服务器**防火墙**建议（控制台 → 防火墙）：

| 规则 | 协议端口 | 来源 |
|---|---|---|
| SSH | TCP:22 | 你的固定 IP（不要 `0.0.0.0/0`） |
| HTTP | TCP:80 | `0.0.0.0/0` |
| HTTPS | TCP:443 | `0.0.0.0/0` |
| 应用端口 | TCP:8080 | **不添加规则**（默认全部拒绝即可） |

其他网络要求：

- **域名 + 备案**：腾讯云轻量绑定域名做 HTTPS 需域名已完成备案（中国大陆机房）。
- WS 需要反代支持 Upgrade 头（Caddy 默认支持；Nginx 需显式 `Upgrade`/`Connection` 头）。
- 出网：应用本身**不需要**出网（无外部 API 调用）。OCR 若后续接云厂商，届时才需出网白名单。

---

## 5. 环境变量（全量，含默认值）

> 全部以 `EBK_` 前缀；**代码中不存在其他生产变量**（`EBK_TEST_*` 仅测试用）。
> 非法值采取 **fail-fast**（启动即失败），不会静默回落默认值。

| 变量 | 必填 | 默认值 | 取值范围 / 约束 | 说明 |
|---|---|---|---|---|
| `EBK_JWT_SECRET` | **是**（仅 serve） | 无 | ≥32 字符；拒绝占位值 `请改成随机长字符串` | JWT 签名密钥，泄露=可伪造任意用户 |
| `EBK_HTTP_PORT` | 否 | `8080` | 1–65535 整数 | 监听端口（监听 `:PORT`，所有网卡） |
| `EBK_DB_PATH` | 否 | `./data/fastledger.db` | 非空路径 | SQLite 文件路径 |
| `EBK_DATA_DIR` | 否 | `./data` | 非空路径 | `instance.meta` 所在目录；**必须与 DB 目录同一持久卷** |
| `EBK_ENABLE_REGISTER` | 否 | `false` | 仅 `true/false/1/0` 等严格布尔 | 公开注册开关；建好账号后**保持 false** |
| `EBK_RECYCLE_TTL_DAYS` | 否 | `30` | 正整数 | 回收站保留天数（`db purge` 按此硬删） |
| `EBK_AUDIT_RETENTION_DAYS` | 否 | `365` | 正整数 | 审计日志保留天数（`db purge` 清理） |
| `EBK_INVITE_TTL_DAYS` | 否 | `7` | 正整数 | 邀请码有效期 |
| `EBK_LOGIN_MAX_ATTEMPTS` | 否 | `10` | 正整数 | 登录失败限流配额 |
| `EBK_LOGIN_WINDOW_MINUTES` | 否 | `15` | 正整数，**上限 1440** | 限流窗口；超上限启动失败（防"一次打满永久锁死"） |
| `EBK_WS_MAX_CONN_PER_USER` | 否 | `10` | 正整数 | 单用户 WS 连接上限 |
| `EBK_WS_MAX_CONN_TOTAL` | 否 | `1000` | 正整数 | 实例 WS 连接总上限 |
| `EBK_SYNC_PUSH_PER_MINUTE` | 否 | `60` | 正整数 | 单用户同步推送限流 |
| `EBK_SYNC_PULL_PER_MINUTE` | 否 | `300` | 正整数 | 单用户同步拉取限流 |

**未实现、勿配置**（设计文档里出现过但代码中没有）：`EBK_LLM_*`、`EBK_TRUSTED_PROXIES`、TLS 相关变量。
> `EBK_TRUSTED_PROXIES` 是最关键的一项缺失，见 §11-P0。

**固定值（不可配置，需知晓）**：

| 项 | 值 |
|---|---|
| 访问令牌有效期 | **24 小时**（硬编码 `tokenTTL`） |
| 数据库连接数 | **1**（`MaxOpenConns=1`，串行化） |
| SQLite PRAGMA | `journal_mode=WAL`、`foreign_keys=on`、`synchronous=FULL`、`busy_timeout=5000` |
| 口令哈希 | bcrypt（自带盐，无独立 salt 列） |

`/etc/fastledger/env` 示例（权限 `0600`、属主 `fastledger`）：

```bash
EBK_JWT_SECRET=<openssl rand -base64 48 的输出>
EBK_DATA_DIR=/var/lib/fastledger/data
EBK_DB_PATH=/var/lib/fastledger/data/fastledger.db
EBK_HTTP_PORT=8080
EBK_ENABLE_REGISTER=false
```

---

## 6. 密钥与安全基线

| 项 | 要求 |
|---|---|
| JWT 密钥 | 32+ 字符随机；**不入库、不进 git、不进备份包说明文档**；建议 `openssl rand -base64 48` |
| 密钥轮换 | 目前**无平滑轮换机制**（轮换=所有用户需重新登录），需与专家确认可接受性 |
| 运行用户 | 专用非 root 用户（如 `fastledger`），仅对数据目录有读写权限 |
| 数据目录权限 | 应用会自动把目录收紧到 `0700`、db/wal/shm 收紧到 `0600`；勿强行放宽 |
| 公开注册 | 建好账号后 `EBK_ENABLE_REGISTER=false` |
| 审计 | 登录成功/失败、权限变更、账本/账户/分类写操作、备份恢复均落审计；默认保留 365 天 |
| 限流 | 登录失败 10 次/15 分钟（按来源网段聚合）；邀请 10 次/小时；同步 push 60、pull 300 每分钟每用户 |
| WS 鉴权 | 支持 Header 或 `?token=`；**`?token=` 会进反代 access log，必须在 Caddy/Nginx 侧对该参数脱敏** |
| TLS 参数 | 由反代负责；应用侧不看 HTTPS，勿在应用层加 TLS 终止 |

---

## 7. 数据存储与持久化

### 7.1 需要持久化的内容

| 路径（默认） | 内容 | 丢失后果 |
|---|---|---|
| `${EBK_DATA_DIR}/fastledger.db` | 全部业务数据 | 全部数据丢失 |
| `.../fastledger.db-wal`、`...-shm` | WAL 与共享内存 | 未 checkpoint 的事务丢失（复制备份时必须一并处理，见下） |
| `${EBK_DATA_DIR}/instance.meta` | 恢复世代 `restore_epoch` | 客户端增量同步判断错乱（**必须随数据一起演进，但不随备份回滚**） |

> `instance.meta` 的特殊性：**备份包里必须排除它**；恢复数据文件后要执行 `instance bump` 递增世代，客户端才会丢弃旧游标做全量重建。这是防"恢复后被旧缓存覆盖"的关键动作。

### 7.2 备份要求

| 项 | 要求 |
|---|---|
| 方式 | **不要直接 `cp` 数据库文件**（WAL 未 checkpoint 会丢事务）。用 `sqlite3 db ".backup out.db"` 或 `VACUUM INTO` |
| 频率 | 建议每日 1 次 + 每次升级前 1 次 |
| 保留 | 14~30 份（与审计 365 天、回收站 30 天口径配合） |
| 异地 | 建议同步到对象存储（COS）或另一台机器 |
| 恢复演练 | **必做**，建议每月：新实例恢复 → 校验记录数与关键汇总 → `instance bump` → 客户端全量重建后抽查 |
| 备份未验证 | 等于没有备份（本项目文档明确此原则） |

### 7.3 容量估算（家庭场景）

| 项 | 估算 |
|---|---|
| 交易量 | 2~5 人家庭，每月 200~500 笔 |
| 数据库增长 | 3 年 < 200 MB（含审计；审计按 365 天滚动清理） |
| 备份占用 | 每份 < 50 MB（压缩后更小），30 份 < 1.5 GB |
| 结论 | **磁盘 40 GB 足够，60~80 GB 更从容**（含系统、快照、日志） |

### 7.4 定时任务（必须配）

```cron
# 每日 03:10 硬清理回收站与过期审计（否则数据只增不减）
10 3 * * * cd /opt/fastledger && ./fastledger db purge >> /var/log/fastledger/purge.log 2>&1
# 每日 03:30 一致性备份
30 3 * * * sqlite3 /var/lib/fastledger/data/fastledger.db ".backup '/var/backups/fastledger-$(date +\%F).db'"
```

> `db purge` 支持 `--dry-run` 预览将删除的条数与账本明细，建议首次先预览再正式执行。

---

## 8. 配置规格建议

| 档位 | 规格 | 适用 | 备注 |
|---|---|---|---|
| **最低可跑** | 1 vCPU / 1 GB / 40 GB SSD / 3 Mbps | 1~2 人自用、仅 API 访问 | **构建期易 OOM**：建议加 1 GB swap，或改为在本地/CI 构建后传二进制 |
| **推荐** | 2 vCPU / 2 GB / 60~80 GB SSD / 4~6 Mbps | 家庭 3~5 人、多设备同步 | 构建与运行都从容；留出快照空间 |
| 高可用 | 不在本期范围 | — | 单机 SQLite，无主从；靠快照 + 备份恢复 |

资源画像（实测口径，供 sizing 参考）：

- 常驻内存：约 30~80 MB（Go + SQLite 页缓存默认值）
- CPU：空闲近乎 0；单次请求毫秒级
- **并发上限由 `MaxOpenConns=1` 决定**：数据库访问串行，写入吞吐与磁盘 fsync 强相关（`synchronous=FULL`）。家庭规模完全够，但**不要把它当多租户 SaaS 用**
- 磁盘 IO：WAL + FULL 同步，建议 SSD（轻量默认即 SSD）

---

## 9. 部署步骤（可直接执行）

### 阶段 A：主机与前置

1. 购买轻量服务器（2C2G / 60GB，系统建议 Ubuntu 22.04 或 Debian 12）。
2. 控制台防火墙：只放行 22（限 IP）、80、443。
3. 系统初始化：
   ```bash
   apt update && apt install -y build-essential sqlite3 curl
   useradd -r -s /usr/sbin/nologin fastledger
   mkdir -p /opt/fastledger /var/lib/fastledger/data /var/log/fastledger /etc/fastledger
   chown -R fastledger:fastledger /var/lib/fastledger /var/log/fastledger
   ```
4. 安装 Go 1.22+（或按 §11-P1 建议改为"CI 构建产物 + 仅上传二进制"）。

### 阶段 B：构建与首次初始化

5. 取代码（`git clone` 或上传源码包）→ `cd server`。
6. `export CGO_ENABLED=1 && go build -trimpath -ldflags "-s -w" -o fastledger ./cmd/server`。
7. 写 `/etc/fastledger/env`（`chmod 600`、`chown fastledger`），含 `EBK_JWT_SECRET`（`openssl rand -base64 48`）。
8. 以 `fastledger` 用户执行：`./fastledger db migrate` → 应输出「本次应用 N 个，当前 schema v6」。
9. `./fastledger user create --username admin --password '<强口令>'`。
10. `curl -s http://127.0.0.1:8080/healthz` → `{"code":0,"msg":"ok","data":{"schemaVersion":6}}`。

### 阶段 C：常驻与反代

11. 安装 systemd 单元（§3.4）→ `systemctl enable --now fastledger`。
12. 安装 Caddy（推荐，自动 HTTPS）：

    ```caddyfile
    ledger.example.com {
        reverse_proxy 127.0.0.1:8080
        # ?token= 脱敏（WS 鉴权回退参数，见 §6）
        log {
            output file /var/log/caddy/ledger.log
            format filter {
                wrap json
                fields {
                    request>uri query {
                        replace token REDACTED
                    }
                }
            }
        }
    }
    ```

13. 配置域名解析 → 等待证书签发 → `https://ledger.example.com/healthz` 应返回同上 JSON。
14. 配置 cron（§7.4）与备份目录。

> ⚠️ 阶段 C 完成后**先别宣布上线**：反向代理会让应用看到的客户端 IP 全部变成 `127.0.0.1`，登录限流会退化为"全站共享 10 次/15 分钟"。这就是 §11 的 **P0**，必须先解决。

---

## 10. 验收清单

### 10.1 基础可用

- [ ] `db migrate` 输出「当前 schema v6」，重复执行幂等（第二次输出「本次应用 0 个」）
- [ ] `curl 127.0.0.1:8080/healthz` 返回 `schemaVersion: 6`
- [ ] 用 `user create` 建的账号能 `POST /api/v1/auth/login` 拿到 token
- [ ] 带 token 请求 `GET /api/v1/users/me` 返回本人信息
- [ ] 错误口令 11 次：前 10 次 401，第 11 次 **429**
- [ ] `EBK_ENABLE_REGISTER=false` 时 `POST /api/v1/auth/register` 被拒

### 10.2 网络与反代

- [ ] 公网 `https://域名/healthz` 正常，HTTP 自动跳 HTTPS
- [ ] 证书有效且自动续期（Caddy 日志可见 ACME 成功）
- [ ] **公网直接访问 `http://IP:8080` 不通**（防火墙未放行）
- [ ] WS 握手成功：`wss://域名/api/v1/ws?token=<token>` 可建立连接
- [ ] 反代日志中 `token` 参数已脱敏（不出现明文 JWT）

### 10.3 数据与持久化

- [ ] 重启服务后数据仍在（`systemctl restart fastledger`）
- [ ] 数据库文件权限为 `0600`、目录 `0700`
- [ ] 手工备份 + 恢复到新库可用；恢复后执行 `instance bump` 有递增输出
- [ ] `db purge --dry-run` 预览数与正式执行数一致
- [ ] cron 已生效（次日查看日志有清理与备份记录）

### 10.4 安全

- [ ] `EBK_JWT_SECRET` 长度 ≥32 且不是占位值（故意配短应启动失败）
- [ ] 服务以非 root 用户运行（`ps -o user= -p <pid>`）
- [ ] 22 端口仅运维 IP 可达

---

## 11. 需要与专家确认 / 对齐的事项

### P0（阻塞上线，必须先决策）

**P0-1 反向代理会导致限流退化为"全站共享一个桶"**
- 现象：应用 `api/router.go:44` **硬编码** `SetTrustedProxies(nil)`（不信任任何代理），`ClientIP()` 恒取 TCP 对端。放在 Caddy/Nginx 后面时，所有外网请求的对端都是 `127.0.0.1`。
- 后果：登录失败限流键变成同一个 `login:127.0.0.0/24` 桶 → **全家 10 次/15 分钟共享配额**，一人输错几次就锁住所有人；邀请兑换限流同理（10 次/小时共享）。攻击者也能借此拖垮正常用户。
- 选项：
  - **(a) 推荐**：开发方新增 `EBK_TRUSTED_PROXIES`（如 `127.0.0.1/32`），启动时 `SetTrustedProxies(...)`；**约半天工作量，含测试**。
  - (b) 不加反代、应用直连公网：**不可接受**（无 TLS，凭证与 token 明文传输）。
  - (c) 反代与增强限流都不做：不可接受（一个家人误输密码即锁全家）。
- **需要专家确认**：反代是否与应用于**同一台主机**（决定 CIDR 填 `127.0.0.1/32` 还是 VPC 网段）。

**P0-2 是否接受"单机无高可用"**
- 单机 SQLite + 单进程，机器故障即为停机，恢复依赖快照/备份（RTO 约 10~30 分钟人工介入，RPO = 上次备份）。
- **需要确认**：这个可用性档位是否符合预期（家庭自用通常可接受，但要写明）。

### P1（上线前应完成）

| 编号 | 事项 | 说明 |
|---|---|---|
| P1-1 | 构建方式定稿 | 在 1~2 GB 小机上直接 `go build` 有 OOM 风险；建议在 CI 产出 linux/amd64 二进制（仓库已有 GitHub Actions），服务器只放二进制。**需专家确认是否接受"服务器不含源码"的部署形态** |
| P1-2 | 域名与备案 | 大陆机房做 HTTPS 需备案域名；若无备案域名，需确认是否改用非 443 端口 + 自签证书（体验差，不推荐） |
| P1-3 | 备份落点 | 本机 `/var/backups` 与机器同毁；建议加 COS 异地。需确认是否开通对象存储与费用预期 |
| P1-4 | 监控告警 | 最小方案：`/healthz` 外部探测 + 磁盘使用率告警 + 服务进程存活。需确认是否接入腾讯云云监控/告警 |
| P1-5 | 日志与脱敏 | 反代 access log 的 `?token=` 必须脱敏（§9 步骤 12）；需确认 Caddy 配置评审 |
| P1-6 | 定时任务（`db purge`） | 不配则数据只增不减；需确认 cron 落点与失败告警方式 |

### P2（可后置，但需登记）

| 编号 | 事项 | 说明 |
|---|---|---|
| P2-1 | 容器化 | 当前无 Dockerfile；若专家方偏好容器，需开发方补镜像（含 gcc 构建阶段 + 运行阶段双阶段） |
| P2-2 | 密钥轮换机制 | 目前轮换 = 全员重新登录；如需平滑轮换需增加多密钥验证期 |
| P2-3 | 多端会话撤销（WBS 5.10） | **正在开发中，尚未合入**。上线后用户暂无法"踢掉其他设备"。上线时间需与此功能对齐 |
| P2-4 | OCR 与客户端 | 阶段 5 内容，本期不部署；若后续接云 OCR，需要出网白名单与密钥管理方案 |
| P2-5 | 数据导出 | 目前无 `/export` 接口（PRD 有要求，未实现）；运维视角的"数据可带走"暂靠直接拷 db 文件 |

---

## 12. 已知风险与边界条件

### 12.1 部署风险

| 风险 | 触发条件 | 影响 | 缓解 |
|---|---|---|---|
| 反代下 ClientIP 失真 | 放在 Caddy/Nginx 后 | 限流退化为全站一个桶（§11-P0-1） | 必须先做 P0-1 |
| `CGO_ENABLED=0` 假成功 | 构建机缺 gcc | 二进制能跑起来但数据库不可用 | 构建后立即 `db migrate` 验证 |
| 未迁移就启动 | 忘记跑 `db migrate` | 进程退出（提示先迁移） | 部署脚本里串起来 |
| 直接 `cp` 备份 | WAL 未 checkpoint | 丢最近事务 | 用 `.backup` / `VACUUM INTO` |
| 恢复后未 `instance bump` | 恢复备份后直接启动 | 客户端带旧游标增量，出现脏读/覆盖 | 恢复流程固化：停服 → 换库 → `instance bump` → 启动 |
| SIGKILL | 强制杀进程 | WAL 未 checkpoint | `TimeoutStopSec=30`，禁 `kill -9` |
| 8080 对公网暴露 | 防火墙误放行 | 明文 HTTP，凭证泄露 | 防火墙默认拒绝；验收含"公网 8080 不通" |
| 磁盘写满 | 未配 `db purge` 与备份清理 | SQLite 写入失败，服务不可用 | cron + 磁盘告警 |
| 机器时间漂移 | 未开 NTP | JWT 判定异常、限流窗口错乱 | 启用 chrony/systemd-timesyncd |

### 12.2 系统边界条件（写清楚，免得当成 Bug）

1. **数据库访问是串行的**（`MaxOpenConns=1`）：并发请求排队，压测数字会很低，这是 SQLite 单写者的必然结果，不是性能缺陷。
2. **令牌有效期固定 24 小时**，无 refresh token：到期需重新登录（客户端体验后续优化）。
3. **无多端会话撤销**（P2-3 进行中）：token 一旦签发，在 24 小时内无法作废。
4. **IPv6 下限流粒度是 /64**（不是单地址）：同一 /64 内多设备共享失败配额，属刻意取舍（防地址轮转绕过）。
5. **审计里的来源 IP 记的是聚合网段**（如 `203.0.113.0/24`），不是精确地址；精确地址只在 429 的服务端日志里出现（保留短窗口）。排查时不要以为"审计查不到 IP 是故障"。
6. **迁移只进不退**：库中若存在程序不认识的更高版本，服务会拒绝启动并提示"降级运行"，`db migrate` 也无法修复（需升级程序或恢复备份）。
7. **审计保留 365 天、回收站 30 天**为默认值，改动需同步改 cron 参数，否则清理范围与预期不符。
8. **无内置 TLS、无静态前端、无 Dockerfile**：三项都需在部署侧补齐或明确不做。
9. **Windows 侧的文件权限断言在 Linux 上不适用**：测试里的 5 条 SKIP 是 Windows ACL 用例，与部署无关。
10. **当前 schema 为 v6**，另有一项会话管理迁移（0008）在开发中；部署文档与验收里的版本号需在合入后同步递增。

---

## 13. 附：给专家的最短执行路径

```bash
# 1) 主机准备
apt update && apt install -y build-essential sqlite3 openssl
useradd -r -s /usr/sbin/nologin fastledger

# 2) 构建
cd /opt/fastledger && git clone <repo> . && cd server
CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o ../fastledger ./cmd/server
cd ..

# 3) 配置
install -d -m 700 -o fastledger -g fastledger /var/lib/fastledger/data
printf 'EBK_JWT_SECRET=%s\nEBK_DATA_DIR=/var/lib/fastledger/data\nEBK_DB_PATH=/var/lib/fastledger/data/fastledger.db\n' \
  "$(openssl rand -base64 48)" > /etc/fastledger/env
chmod 600 /etc/fastledger/env && chown fastledger /etc/fastledger/env

# 4) 初始化并启动
sudo -u fastledger ./fastledger db migrate
sudo -u fastledger ./fastledger user create --username admin --password '<强口令>'
systemctl enable --now fastledger
curl -s http://127.0.0.1:8080/healthz

# 5) 反代（P0-1 解决后再开放公网）
#    Caddyfile 见 §9 步骤 12
```

**交接判断标准**：§10 四组验收全部勾选 + §11 的 P0/P1 有明确结论，即视为部署颗粒度对齐完毕。

---

**文档维护**：本文件随代码演进。已知待同步项：会话管理迁移（0008）合入后，§5「固定值」、§10 版本号、§12.2 第 10 条需更新。
