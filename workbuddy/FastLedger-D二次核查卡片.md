# D 二次核查一页纸（D-R07 起）

> 你仍是 **D**（第四个 AI）。你已完成**第一轮文档/骨架评审**（`D-R01`–`D-R06`），那 6 条**全部属实并已修复**。
> 本轮是**第二次核查**，但审查对象变了：项目已从"纯文档"进入**真实代码阶段**。编号请**接着你的序列用 `D-R07` 起**（不要重排、不要复用 `D-R01`–`D-R06`）。
> 详细规则见 `workbuddy/多AI对齐规范.md`（颗粒度宪法）。

---

## 1. 一句话现状

`server/` 里已有一套**可编译的 Go 后端**，完成 WBS **阶段 0/1/2**（脚手架 → 认证/账本/成员 → 记账核心），**41 个单元测试全绿**；**但没有任何真实数据库的运行时证据**（构建机无 gcc）。阶段 3（同步/WS/可见性）尚未开始。

---

## 2. 你上一轮 6 条现在什么状态（**先看这个，避免重报**）

| 你的编号 | 你提的问题 | 现在状态 | 注意 |
|---|---|---|---|
| `D-R01` | 删除/恢复/审计已决策但 DDL/骨架/接口未落地 | ✅ **文档层已修**（[1.15]）：两文档补 `audit_logs` DDL、指南新增 §13.3 `TxService.Delete/Restore`、方案 §5.1 补 4 个接口 | ⚠️ **真实 Go 代码未实现**——属**阶段 4**，请勿重报"没实现" |
| `D-R02` | 迁移机制只有 CLI 名，缺设计 | ✅ **两处都修了**：方案新增 §4.7；且 `server/internal/migrate/` 已有**真实实现**（`schema_migrations` + embed + 事务内执行 + 仅 CLI 手动） | 可审实现正确性 |
| `D-R03` | WS 有 ping 无 pong/读截止 | ✅ **文档层已修**：`SetReadLimit(64KB)` + `SetReadDeadline(60s)` + `SetPongHandler` + 显式 `upgrader` | ⚠️ **真实 WS 代码未实现**——属**阶段 3** |
| `D-R04` | OCR handler 吞绑定错误、无校验/超时 | ✅ **文档层已修**：`ParseBill(ctx, text)`、绑定错误返 400、空/超长校验、`WithTimeout(10s)` | ⚠️ **真实 handler 未实现**——属**阶段 5** |
| `D-R05` | Flutter ApiClient 无超时/不判状态码 | ✅ **文档层已修**：持久 client + timeout + 状态码分支 + `ApiException`(含 409) | ⚠️ **Flutter 代码未实现** |
| `D-R06` | Pull 缺成员校验与批次上限 | ✅ **文档层已修 + 你当时提的分页问题已在 [1.16] 重写为 `safeVersion` 版本边界** | ⚠️ **真实 Pull 代码未实现**——属**阶段 3** |

> **一句话**：你上一轮的 6 条**全部闭环**；但其中 4 条只在**文档/骨架**层闭环，真实代码要等阶段 3/4/5。**"某功能还没实现"不是新 finding**，除非你能证明"设计本身有错"或"已实现的部分实现错了"。

---

## 3. 必读顺序（按此顺序）

| # | 文件 | 为什么 |
|---|---|---|
| 1 | 本卡片 | 现状 + 你上轮结果 + 审查焦点 |
| 2 | `workbuddy/多AI对齐规范.md` | §2 十三字段、§3 严重度、§4 证据标准、§6 口径优先级 |
| 3 | `CHANGELOG.md` **[1.13]–[1.23]** | **代码落地**（你上次看的是 [1.13] 之前） |
| 4 | `docs/FastLedger-开发WBS.md` | 8 阶段任务分解 + §10 验收映射 |
| 5 | **`server/`（真实代码）** | 本轮主战场 |
| 6 | `workbuddy/FastLedger-文档评审-C.md` | C 的 10 项（已裁决，勿重复提） |

---

## 4. 我已做完的事（阶段 0/1/2，附落点）

### 阶段 0：工程脚手架（[1.18]）
- `server/go.mod`（module `fastledger`，go 1.22；gin v1.10.0 / xorm v1.3.9 / go-sqlite3 v1.14.22）
- `internal/config/`：`EBK_` 前缀；`EBK_JWT_SECRET` 空/占位/<32 位**启动即失败**
- `internal/database/database.go`：**DSN 逐连接**设 `WAL / busy_timeout=5000 / foreign_keys=on / synchronous=FULL`；`MaxOpenConns=1`；**启动校验三项 PRAGMA 真的生效**
- `internal/migrate/`：`schema_migrations` 版本表 + embed 脚本 + **事务内执行** + **仅 CLI `db migrate`**（启动只检查版本）
- `migrations/0001_init.up.sql`：15 张表
- `cmd/server/main.go`：`run` / `db migrate` / `user create` + 优雅关机；`/healthz` 返回 `schemaVersion`

### 阶段 1：认证 / 账本 / 成员（[1.19]）
- `internal/service/jwt.go`：**手写 HS256**——限定 HS256、**拒 `alg=none`**、拒篡改/过期/错密钥、安全读 uid
- `internal/service/auth.go`：bcrypt；登录校验已删/禁用；token 带 `jti`
- `internal/service/book.go`：账本 CRUD；创建时**同事务**初始化 `book_versions` + owner 成员 + 默认数据
- `internal/service/member.go`：权限矩阵（**admin 改删他人交易 = ✅**）、软删成员、**邀请码入户**（单次/7 天/绑角色/限流）
- `internal/version/`：**唯一** `VersionRepo.Next`，经 `version.Source` 注入
- `migrations/0002_*.sql`：`UNIQUE(book_id,uid)` 与软删冲突 → 改**部分唯一索引** `WHERE deleted=0`；新增 `invites`

### 阶段 2：记账核心（[1.20]/[1.21]）
- `internal/service/tx.go`：
  - **唯一创建入口** `Create(ctx, uid, bookID, CreateRequest)`
  - 转账 `createTransferInTx`：**复用外层事务与版本**（每命令仅一次 `versions.Next`）、两侧余额+两条记录同事务同版本、`related_id` 互指
  - 余额 `createChangeBalance`：**唯一写路径**，单条原子 UPDATE（`book_id` + `deleted=0` + `RowsAffected==1` + `balance BETWEEN` 溢出保护）
  - `signedBalanceDelta`：category **10~15 取反**（正=欠款）
  - 幂等：`ON CONFLICT(book_id,uid,client_uuid) DO NOTHING` + `RowsAffected` + 重读回执核对（hash/type/ids/version）
  - `createRequestHash`：域标签 `transaction.create/v1`、覆盖完整请求、**排除 clientUuid**
  - `validateCreate`：**服务层**强制 clientUuid、金额 `1..1e10` 分
- `internal/service/{account,category,guard,seed}.go`、`internal/api/*`、`migrations/0003_*.sql`（`creator_uid`/`source`/`timezone_offset` + 索引）

### 测试
`internal/service/{jwt,tx,qa_extra}_test.go` + `internal/api/{router,qa_api}_test.go` —— **41 用例 RUN=41 PASS=41 FAIL=0 SKIP=0**。

---

## 5. 当前"真实到了哪"（别高估）

### ✅ 已验证
- `gofmt -l .` 空；`go build ./...`、`go vet ./...` 通过；`go test ./...` 41/41
- `0001` DDL 经 **Python sqlite3 真实执行**，建出 15 张表
- 三项 PRAGMA 期望值实测正确：`journal_mode=wal`、`synchronous=2(FULL)`、`foreign_keys=1`
- 编译期抓出并修复：`engine.DB()` 返回 `*core.DB`（内嵌 `*sql.DB`）→ 需 `engine.DB().DB`

### ❌ 未验证（**本轮最该盯**）
构建机**无 gcc → CGO 不可用 → go-sqlite3 只能编译不能运行**。以下**全部没有真实运行时证据**：

1. 事务真实回滚（转账第二笔失败 / 回执回写失败 / 余额影响 0 行）
2. `ON CONFLICT ... DO NOTHING` 的 `RowsAffected` 真实语义；SQLite 版本是否满足 UPSERT
3. 并发（`busy_timeout=5000` + `MaxOpenConns=1`）下同 clientUuid / 同账户的串行化
4. `xorm:"'type'"` 引号列在真实 INSERT/SELECT 的表现；`sess.Insert` 对零值的写入
5. `related_id` 双向配对是否真落库
6. **`0002`→`0003` 迁移在真实库的可执行性**（`ALTER TABLE ADD COLUMN` + 建索引）
7. PRD §10.1 / §10.3 / §10.4 **端到端**
8. `go test -race`
9. JWT→handler→service 的 Gin 全链路集成

---

## 6. 建议你的审查焦点（按性价比排序）

1. **`tx.go` 事务边界**：有没有错误路径「先改余额再返回错误」却没回滚？`defer sess.Rollback()` 与 `Commit` 在 xorm 下交互是否正确？
2. **幂等正确性**：`ON CONFLICT` 列集合与唯一索引是否完全一致？命中后四项核对有无漏洞（`result_ids` 数量/正负）？重放是否真不碰余额、不递增版本？
3. **`BETWEEN` 边界数学**：`delta>0 → high=Max-delta` 是否等价于"不溢出"？会否误伤合法大额？
4. **"禁单边"是否可绕过**：`validateCreate` 对 `type=3` 的约束能否被构造请求绕过？`related_id` 互指是否同事务完成？
5. **符号表**：15 类账户符号是否与《枚举字典》§2 完全一致？
6. **错误映射**：有无该 400 却映射成 409/500？（**余额溢出应 400**，重试无意义）
7. **`migrations/0003`**：`ALTER TABLE ADD COLUMN ... NOT NULL DEFAULT 0` 合法性、索引必要性、老库升级是否安全（**不得删旧回执**）
8. **隐藏第二入口**：`grep` 是否还有 `/transfer`、`/sync/push` 等提前实现
9. **`creator_uid=0` 哨兵**：阶段 3/4 做可见性与"仅本人可改"时是否会被当作放行条件（这是**高危越权**隐患）

---

## 7. 什么别重复提

| 来源 | 状态 |
|---|---|
| GPT `R01`–`R23` | 全部已核实并落地 |
| `C-R01`–`C-R10` | 已裁决（C-R03/05/06 落地 [1.9]） |
| **你自己的 `D-R01`–`D-R06`** | **全部已修**（[1.15]–[1.17]），见第 2 节 |
| 已定决策 | 多账本、增量同步、隐私定位、并发 409、整数分、`synchronous=FULL`、统一 `TxService.Create`、`VersionRepo.Next`、admin 可改删他人交易、回收站 30 天/审计 1 年、邀请 7 天单次 | 

**已知非阻塞遗留（不必重报，可提改进方案）**：
- P2-2：PRD §7.3「member 仅看本人记录」服务端过滤 → 计划**阶段 3**
- P3-3：`CreateRequest` 无 `Merchant`，`transactions.merchant` 恒空 → 计划**阶段 5**
- `creator_uid = 0` = 历史未知归属，阶段 3/4 必须显式处理

---

## 8. 提意见格式（否则会被退回）

每条 **`D-R07` 起**，必须含 `多AI对齐规范.md` §2 的 **13 个字段**：
`id / title / severity(P0–P3) / location(文件+章节+行号) / quote(原文片段) / problem / reproduce / actual / expected / impact / suggestion / status / evidence`

- **severity**：P0 核心闭环不成立/数据损坏/安全绕过；P1 安全/隐私/**会计正确性**；P2 跨文档一致性/健壮性；P3 优化
- **证据**：必须附**原文片段 + 精确位置**；禁止"可能/大概/看起来"；拿不准标 `[待确认]`
- **先搜重**：提之前 `grep` 一遍 `CHANGELOG.md` + `workbuddy/` + `gpt/`
- **请区分两类**：「设计文档错了」与「代码没照设计做」——我会分开处置（前者改文档，后者返工代码）

---

*本卡片由 WorkBuddy 于 [1.22] 之后编写，供 D 的第二次核查使用。若 `main` 已明显前进，以 `CHANGELOG.md` 与 `server/` 实际代码为准。*
