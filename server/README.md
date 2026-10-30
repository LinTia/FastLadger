# FastLedger 服务端（阶段 0 脚手架）

对应《开发 WBS》**阶段 0：工程脚手架**。目标是把"文档骨架"变成**真实可编译、可启动**的 Go 工程底座，为阶段 1（认证/账本）打底。

## 目录结构

```
server/
├── go.mod / go.sum
├── cmd/server/main.go              # 入口：run（HTTP）与 db migrate（迁移）
├── internal/
│   ├── config/config.go            # EBK_ 环境变量加载 + JWT 密钥 fail-fast 校验
│   ├── database/database.go        # initDB：DSN 逐连接 PRAGMA + 启动校验
│   └── migrate/migrate.go          # 版本化迁移（schema_migrations + 事务内执行）
│   └── migrate/migrations/
│       └── 0001_init.up.sql        # 初始全量表（含 audit_logs、idempotency_records）
```

## 构建与运行

> ⚠️ `mattn/go-sqlite3` 依赖 **CGO**，构建环境需要 C 编译器：
> - Linux：`apt install -y gcc`（或 `build-essential`）
> - Windows：安装 MinGW-w64 / w64devkit，并保证 `gcc` 在 PATH
> - macOS：安装 Xcode Command Line Tools

```bash
cd server

# 1) 首次建库（手动迁移；服务启动不会自动改表）
export EBK_JWT_SECRET="$(openssl rand -base64 48)"   # ≥32 位，否则拒绝启动
go run ./cmd/server db migrate

# 2) 启动服务（当前仅 /healthz）
go run ./cmd/server
# 或
go build -o fastledger ./cmd/server && ./fastledger

# 3) 验证
curl http://localhost:8080/healthz
# => {"code":0,"msg":"ok","data":{"schemaVersion":1}}
```

## 排错：`CGO_ENABLED=0` 的假成功

环境无 gcc 时，Go 会自动把 `CGO_ENABLED` 置为 `0`。此时**构建能过、也能链接出二进制**，但 `go-sqlite3` 只是个 stub，数据库实际不可用。运行会看到：

```
数据库初始化失败：读取 foreign_keys 失败：Binary was compiled with 'CGO_ENABLED=0',
go-sqlite3 requires cgo to work. This is a stub
```

这说明**启动 PRAGMA 校验生效了**——它在数据库不可用时快速失败，而不是带着坏连接继续跑。修复：装好 gcc 并显式开启 CGO 重新构建：

```bash
go env -w CGO_ENABLED=1
go build -o fastledger ./cmd/server
```

> 判据：**"能编译" ≠ "能跑"**。阶段 0 的验收应是 `db migrate` 成功建表 + `/healthz` 返回 `schemaVersion`，而不是仅仅 `go build` 无报错。

## 环境变量（统一 `EBK_` 前缀，见 C-R05）

| 变量 | 默认值 | 说明 |
|---|---|---|
| `EBK_HTTP_PORT` | `8080` | 监听端口 |
| `EBK_DB_PATH` | `./data/fastledger.db` | SQLite 路径（目录会自动创建） |
| `EBK_JWT_SECRET` | **必填** | 空值 / 占位值 / <32 位 → **启动即失败** |
| `EBK_ENABLE_REGISTER` | `false` | 关闭后仅邀请码可入户（`false` 不影响首个 owner 初始化） |
| `EBK_RECYCLE_TTL_DAYS` | `30` | 回收站保留天数（PRD §12.1） |
| `EBK_INVITE_TTL_DAYS` | `7` | 邀请码有效期（PRD §12.1） |

## 本阶段已落地的设计约束

| 约束 | 落地位置 |
|---|---|
| PRAGMA 走 **DSN 逐连接**（不能 `db.Exec`），并启动校验生效 | `internal/database` |
| `synchronous=FULL`（断电不丢已提交事务，对齐"无数据丢失"） | `internal/database` |
| 写引擎 `MaxOpenConns=1`（SQLite 单写连接，家庭负载串行最稳） | `internal/database` |
| `EBK_` 前缀 + JWT 密钥 fail-fast | `internal/config` |
| **迁移仅 CLI 手动执行**，启动只检查版本、绝不静默改表 | `internal/migrate` + `cmd/server` |
| 迁移在**事务内**执行，失败回滚；老库升级不删旧回执 | `internal/migrate` |
| `audit_logs`（append-only）、`idempotency_records`、`book_versions` | `0001_init.up.sql` |
| 优雅关机（配合部署 `stop_grace_period ≥ 30s`） | `cmd/server/main.go` |

## 阶段 1 已实现（认证 / 账本 / 成员）

- **认证**：`POST /api/v1/auth/register`（受 `EBK_ENABLE_REGISTER` 控制）、`POST /api/v1/auth/login`（返回 accessToken，含 jti）、`GET /api/v1/users/me`
- **JWT**：手写 HS256 实现——限定 HS256、**拒绝 `alg=none`**、拒绝篡改/过期/错误密钥、安全读取 uid（非法 claims 返 401 而非 panic）、登录校验账号是否删除/禁用（R08/R09）
- **账本**：`GET/POST /api/v1/books`、`GET/PUT/DELETE /api/v1/books/:id`（**删除仅 owner**，admin 不可）
- **成员**：`GET /books/:id/members`、`POST /books/:id/members/invite`、`PUT/DELETE /books/:id/members/:uid`（软删保留历史）
- **邀请入户**：`POST /books/:id/members/join`——不受注册开关限制；单次使用、默认 7 天、可绑角色/用户名、生成与使用限流（PRD §12.1）
- **版本**：`internal/version` 是**唯一**的 `VersionRepo.Next` 实现，经 `VersionSource` 注入（禁止再手写）
- **CLI**：`fastledger user create --username u --password p [--owner]`（`--owner` 仅在无任何用户时可用，建好后失效）

### 运行测试

```bash
go test ./internal/service/ -v
# 覆盖：JWT 安全（alg=none/篡改/过期/错密钥）、权限矩阵（admin ✅）、邀请限流
# 无需数据库、无需 CGO
```

## 尚未实现（后续阶段）

- 阶段 2：统一 `TxService.Create`、转账双边原子、幂等回执、Gin 400/409 接线
- 阶段 3：同步 pull/push、WebSocket Hub、可见性（`members_can_view_others` 字段已就位）
- 阶段 4：删除/恢复/审计（`TxService.Delete/Restore`）
- 阶段 5+：客户端与 OCR、报表导出、投资资产

> 本目录代码是**工程底座**，实现细节须与《落地方案》《新手开发完整指南》《转账入口裁决》《枚举字典》保持一致；口径冲突时按《多AI对齐规范》§6 优先级裁决。
