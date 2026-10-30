# 新 AI 一页纸对齐卡片（代号 E）

> 给**第五个审查 AI** 的快速上手卡。读完这一页，你就知道：该读什么、我已经做了什么、现在真实到什么程度、什么别重复提。
> 你的代号：**E**，意见编号用 **`E-Rxx`**（`E-R01`、`E-R02`…）。
> 详细规则仍以 `workbuddy/多AI对齐规范.md` 为准（本卡片只是索引 + 现状快照）。

---

## 1. 项目现在到哪了（一句话）

FastLedger（自托管家庭记账）已从"纯文档"进入**真实代码阶段**：`server/` 里有一套**可编译的 Go 后端**，完成 WBS **阶段 0/1/2**（脚手架 → 认证/账本/成员 → 记账核心），**41 个单元测试全绿**；**但没有任何真实数据库的运行时证据**（构建机无 gcc）。阶段 3（同步/WS/可见性）尚未开始。

---

## 2. 必读顺序（按此顺序，别跳）

| # | 文件 | 为什么读 |
|---|---|---|
| 1 | `workbuddy/多AI对齐规范.md` | **颗粒度宪法**：§2 十三字段模板、§1 编号、§3 严重度、§4 证据标准、§6 口径优先级 |
| 2 | `workbuddy/第三方审阅者导读.md` | 项目背景 + 已定决策清单 + 协作约定 |
| 3 | `CHANGELOG.md` **[1.6]–[1.21]** | **决策冻结（[1.6]–[1.12]）+ 代码落地（[1.13]–[1.21]）** |
| 4 | `docs/FastLedger-开发WBS.md` | 8 阶段任务分解 + §10 验收映射（阶段 0/1/2 已完成） |
| 5 | `server/`（真实代码） | 审代码从这里开始，见下节 |
| 6 | `workbuddy/FastLedger-文档评审-C.md` / `-D.md` | 前两轮评审（**已裁决/已修，勿重复提**） |

---

## 3. 我已经做完的事（按阶段，附落点）

### 阶段 0：工程脚手架（CHANGELOG [1.18]）
- `server/go.mod`（module `fastledger`，go 1.22；gin v1.10.0 / xorm v1.3.9 / go-sqlite3 v1.14.22）
- `internal/config/config.go`：`EBK_` 前缀；`EBK_JWT_SECRET` 空/占位/<32 位**启动即失败**
- `internal/database/database.go`：**DSN 逐连接**设 `WAL / busy_timeout=5000 / foreign_keys=on / synchronous=FULL`；`MaxOpenConns=1`；**启动校验三项 PRAGMA 真的生效**
- `internal/migrate/migrate.go` + `migrations/0001_init.up.sql`：`schema_migrations` 版本表、embed 脚本、**每个迁移在事务内执行**、默认**仅 CLI `db migrate`**（启动只检查版本、绝不自动改表）；15 张表
- `cmd/server/main.go`：`run` / `db migrate` / `user create` 子命令 + 优雅关机；`/healthz` 返回 `schemaVersion`

### 阶段 1：认证 / 账本 / 成员（[1.19]）
- `internal/service/jwt.go`：**手写 HS256**（不引第三方库）——限定 HS256、**拒绝 `alg=none`**、拒绝篡改/过期/错密钥、安全读 uid
- `internal/service/auth.go`：bcrypt 口令；登录校验账号已删/禁用；token 带 `jti`
- `internal/service/book.go`：账本 CRUD；创建时**同事务**初始化 `book_versions` + 写 owner 成员 + 落默认数据
- `internal/service/member.go`：权限矩阵（**admin 改删他人交易 = ✅**，按 [1.9] 裁决）、软删成员、**邀请码入户**（单次使用/7 天/绑角色用户名/限流）
- `internal/version/version.go`：**唯一** `VersionRepo.Next`（事务内原子 +1 并回读），经 `version.Source` 注入
- `migrations/0002_*.sql`：修正 `UNIQUE(book_id,uid)` 与软删冲突（改**部分唯一索引** `WHERE deleted=0`）+ `invites` 表

### 阶段 2：记账核心（[1.20]/[1.21]）
- `internal/service/tx.go`：**唯一创建入口** `Create(ctx, uid, bookID, CreateRequest)`；内部 `createSingleInTx` / `createTransferInTx`
  - 转账：**复用外层事务与版本**（每命令仅一次 `versions.Next`），两侧余额 + 两条记录同事务同版本，`related_id` 互指
  - 余额：**唯一写路径** `createChangeBalance`——单条原子 UPDATE，带 `book_id`、`deleted=0`、`RowsAffected==1`、`balance BETWEEN` 溢出保护
  - 负债符号：`signedBalanceDelta`，category **10~15 取反**（正=欠款）
  - 幂等：`ON CONFLICT(book_id,uid,client_uuid) DO NOTHING` + `RowsAffected` 判定 + 重读回执核对（hash/type/ids/version）
  - `createRequestHash`：域标签 `transaction.create/v1`，覆盖完整请求、**排除 clientUuid**
  - `validateCreate`：**服务层**强制 clientUuid、金额 `1..1e10` 分
- `internal/service/{account,category,guard,seed}.go`：账户/分类 CRUD、成员校验、新账本默认数据
- `internal/api/*`：账户/分类/交易 handler + 路由（仅 `POST /books/:id/transactions` 一个创建入口）、400/403/404/409 映射
- `migrations/0003_*.sql`：补 `creator_uid` / `source` / `timezone_offset` + `idx_tx_creator`（**未改 0001/0002**）

### 测试
`internal/service/{jwt,tx,qa_extra}_test.go`、`internal/api/{router,qa_api}_test.go` —— **41 用例，RUN=41 PASS=41 FAIL=0 SKIP=0**（含 JWT 安全、权限矩阵、限流、15 类账户符号、225 组转账守恒、hash 独立判据、27 条错误映射、路由唯一性）。

---

## 4. 当前"真实到了哪"（务必看，别高估）

### ✅ 已验证（有证据）
- `gofmt -l .` 空；`go build ./...`、`go vet ./...` 通过；`go test ./...` 41/41
- `0001` DDL 经 **Python sqlite3 真实执行**，建出 15 张表（含 `audit_logs`/`idempotency_records`/`book_versions`）
- 三项 PRAGMA 期望值实测正确：`journal_mode=wal`、`synchronous=2(FULL)`、`foreign_keys=1`
- 编译期抓出并修复的真实错误：`engine.DB()` 返回 `*core.DB`（内嵌 `*sql.DB`）→ 需 `engine.DB().DB`

### ❌ 未验证（**最该盯的地方**）
构建机**无 gcc → CGO 不可用 → `go-sqlite3` 只能编译不能运行**。因此以下**全部没有真实运行时证据**：

1. 事务真实回滚（转账第二笔失败、回执回写失败、余额影响 0 行）
2. `ON CONFLICT ... DO NOTHING` 的 `RowsAffected` 真实语义；SQLite 版本是否满足 UPSERT
3. 并发（`busy_timeout=5000` + `MaxOpenConns=1`）下同 clientUuid / 同账户的串行化行为
4. `xorm:"'type'"` 引号列在真实 INSERT/SELECT 的表现；`sess.Insert` 对零值（`deleted=0`/`to_account_id=0`）的写入
5. `related_id` 双向配对是否真落库
6. **`0002`→`0003` 迁移在真实库上的可执行性**（`ALTER TABLE ADD COLUMN` + 建索引）
7. PRD §10.1（手动记账）/§10.3（还款净资产守恒）/§10.4（渠道不重复）**端到端**
8. `go test -race`
9. JWT→handler→service 的 Gin 全链路集成

> 复跑命令（需有 gcc 的环境）：
> ```
> cd server
> export EBK_JWT_SECRET="$(openssl rand -base64 48)"
> go env -w CGO_ENABLED=1
> go run ./cmd/server db migrate     # 期望：应用 3 个，schema v3
> go test ./... -race -count=1
> ```

---

## 5. 什么**别**重复提（已裁决/已修）

| 来源 | 内容 | 状态 |
|---|---|---|
| GPT `R01`–`R23` | 见 `gpt/` 与 `gpt/v1.4/` | 全部已核实并落地 |
| `C-R01`–`C-R10` | 见 `workbuddy/FastLedger-文档评审-C.md` | 已裁决（C-R03/05/06 落地 [1.9]） |
| `D-R01`–`D-R06` | 见 `workbuddy/FastLedger-文档评审-D.md` | 已修（[1.15]）；D 自修 Login/Pull（[1.16]） |
| 已定决策 | 多账本、增量同步、隐私定位、并发 409、整数分、`synchronous=FULL`、统一 `TxService.Create`、`VersionRepo.Next`、admin 可改删他人交易、回收站 30 天/审计 1 年、邀请 7 天单次 | **勿重开辩论**（除非有新证据） |

**已知非阻塞遗留（不必重复报告，但可提改进）**：
- P2-2：PRD §7.3「member 仅看本人记录」服务端过滤 → 计划**阶段 3**
- P3-3：`CreateRequest` 无 `Merchant`，`transactions.merchant` 恒空 → 计划**阶段 5**
- `creator_uid = 0` = 历史未知归属；阶段 3/4 必须**显式处理 0**（视为仅 owner/admin），**禁止当放行条件**

---

## 6. 建议你的审查焦点（按性价比排序）

1. **`internal/service/tx.go` 的事务边界**：有没有哪条错误路径会「先改了余额再返回错误」而没回滚？`defer sess.Rollback()` 与 `Commit` 的交互在 xorm 下是否正确？
2. **幂等的正确性**：`ON CONFLICT` 的列集合是否与唯一索引完全一致？命中后的四项核对是否有漏洞（例如 `result_ids` 数量/正负）？重放路径是否真的不碰余额、不递增版本？
3. **`createChangeBalance` 的 `BETWEEN` 边界数学**：`delta>0 → high=Max-delta` 的判据是否等价于"不溢出"？会不会误伤合法大额？
4. **转账的"禁单边"是否真的无法绕过**：`validateCreate` 对 `type=3` 的约束能否被构造请求绕过？两条记录的 `related_id` 互指是否在同事务内完成？
5. **`signedBalanceDelta` 与净资产口径**：15 类账户的符号表是否与《枚举字典》§2 完全一致？
6. **`internal/api/common.go` 的错误映射**：有没有该 400 却映射成 409/500 的？（注意：**余额溢出应 400 而非 409**，因为重试无意义）
7. **`migrations/0003`**：`ALTER TABLE ADD COLUMN ... NOT NULL DEFAULT 0` 在 SQLite 上的合法性、索引是否必要、对老库升级是否安全（不得删旧回执）。
8. **重复入口/隐藏路径**：`grep` 是否还有第二个交易创建入口（如 `/transfer`、`/sync/push` 提前实现）。

---

## 7. 提意见的格式（否则我会退回）

每条 **`E-Rxx`** 必须含 `多AI对齐规范.md` §2 的 **13 个字段**：
`id / title / severity(P0–P3) / location(文件+章节+行号) / quote(原文片段) / problem / reproduce / actual / expected / impact / suggestion / status / evidence`

- **severity 标尺**：P0 核心闭环不成立/数据损坏/安全绕过；P1 安全/隐私/**会计正确性**；P2 跨文档一致性/健壮性；P3 优化。
- **证据**：必须附**原文片段 + 精确位置**；禁止"可能/大概/看起来"；拿不准标 `[待确认]`。
- **先搜重**：提之前 `grep` 一遍 `CHANGELOG.md` + `workbuddy/` + `gpt/`。
- **请特别区分**：「设计文档说错了」与「代码没照设计做」——这两类我会分开处置。

---

*本卡片由 WorkBuddy 于 [1.21] 之后编写；若你看到 `main` 已明显前进，请以 `CHANGELOG.md` 与 `server/` 实际代码为准。*
