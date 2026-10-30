# GPT 二次核查一页纸（GPT-R24 起）

> 你仍是 **GPT**（第三方独立评审）。你在 PRD 阶段做了两轮：**R01–R18**（首轮 18 项）与 **GPT-R19–GPT-R23**（v1.4 定向复核，含 `GPT-ADR01` 转账入口裁决）。
> 本轮是**第三次核查**，编号请**接着用 `GPT-R24` 起**（不要重排 `R01–R23`，不要复用）。
> 详细规则见 `workbuddy/多AI对齐规范.md`（颗粒度宪法）。

---

## 0. 本轮唯一的核心诉求（与上两轮不同）

**你上次能做而我们这次做不到的事：跑真实数据库。** 你的 `gpt/v1.4/verification/` 里有 CGO + 真实 SQLite 的故障注入、回滚、`go test -race` 证据；而本轮构建机**没有 C 编译器**，所以阶段 0/1/2 的代码**一次真实 SQLite 都没跑过**。

> **本轮最高价值 = 用你的运行能力，把我们 9 项"只能编译、无法运行"的盲区跑掉（见第 5 节），而不是再读一遍文档。**
> 静态审查当然也要，但请把**至少一半精力**放在"把 `server/` 真跑起来并给出断言结果"。

---

## 1. 一句话现状

项目已从**纯文档**进入**真实代码阶段**：`server/` 是一套**可编译的 Go 后端**（module `fastledger`，go 1.22），完成 WBS **阶段 0/1/2**（脚手架 → 认证/账本/成员 → 记账核心），**47 个单元测试全绿、零 skip**；**但没有任何真实 SQLite 运行时证据**。阶段 3（同步/WS/可见性）尚未开始。

> **最新变化（[1.25]，你读到的代码已包含）**：成员/邀请服务修了 4 条（`D-R07`/`D-R08`/`D-R09` + 主理人补充的 `WB-R01`）。其中两处**会影响你的集成测试**：
> 1. `MemberService.JoinViaInvite` 签名已变为 **`(bookID, code, username, password, clientIP)`** —— 原先忽略 `:id`，现在会校验邀请确属该账本；
> 2. 邀请入户限流改为**按 IP 分桶**（原为全局 key `"invite:join"`，属缺陷已修）。
> 若你要测邀请入户的原子性（建 user + 写 book_members + 标记邀请同事务）与"单次使用"的并发保护，请按新签名构造，并纳入第 5 节的验证集。

---

## 2. 你上两轮 23 条现在什么状态（**先看这个，避免重报**）

### 2.1 首轮 R01–R18（18 项，11×P1 + 7×P2）

全部已核实并落地，落在 `[1.2]`–`[1.5]` 的文档/骨架修复，以及本轮 `[1.18]`–`[1.21]` 的**真实代码实现**。逐条状态见 `gpt/CHANGELOG.md` 与根 `CHANGELOG.md`。**请勿重报**，除非你能证明"修的方向本身是错的"。

### 2.2 定向复核 GPT-R19–GPT-R23（5 项，全部已闭环）

| 你的编号 | 你提的问题 | 现在状态 | 落点 |
|---|---|---|---|
| `GPT-R19` | 幂等占位/回写错误被吞，返回了未成立的成功 | ✅ **真实代码已实现** | `tx.go:124` Create：`ON CONFLICT(book_id,uid,client_uuid) DO NOTHING` + `RowsAffected` 判定（0=replay / 1=新建 / 其他=错）；命中后**四项核对**（`request_hash` / `result_type` / `result_ids` 数量与正值 / `result_version>0`）；回执回写 `updated!=1` → **整笔回滚** |
| `GPT-R20` | `requestHash` 遗漏分类、渠道等已接受业务字段 | ✅ **真实代码已实现** | `tx.go:576` `createRequestHash`：域标签 `transaction.create/v1` + **完整 CreateRequest**（type/account/toAccount/amount/category/channel/txTime/tzOffset/comment/source），**仅排除 clientUuid**；QA 另做了"手写规范 JSON 自算 sha256 逐字节比对"的独立判据 |
| `GPT-R21` | `client_uuid` 必填仍只是注释约定 | ✅ **真实代码已实现** | `tx.go:531` `validateCreate` **服务层**强制（空/全空白/超 64 字节 → 400），不依赖 handler binding；handler 侧另有 `binding:"required"` 做双层 |
| `GPT-R22` | 版本号入口跨文档仍有两个函数 | ✅ **真实代码已实现** | `internal/version/version.go` 是**唯一实现** `Next(sess, bookID)`（事务内原子 +1 并回读），经 `version.Source` 接口注入；全仓无 `nextBookVersion` 残留 |
| `GPT-R23` | 双创建入口允许转账绕过双边实现 | ✅ **真实代码已落地** | 唯一公开入口 `TxService.Create`；内部 `createTransferInTx`（`tx.go:289`）**复用外层事务与版本**；`router_test.go` + `qa_api_test.go` 断言 `/transfer`、`/transactions/transfer`、`/transactions/batch` 路由均**不存在** |

### 2.3 你的历史编号复核（GPT-R03 / GPT-R06 / GPT-R16）

这三条你上轮"原编号复核"时报的未闭合分支，对应修复已进真实代码：R03（负债符号）→ `tx.go:334` `signedBalanceDelta` + `model.IsLiabilityCategory`（10~15 取反）；R06（转账回滚）→ `createTransferInTx` 全错误路径 `return nil, err` + `defer sess.Rollback()`；R16（DSL/DDL 契约）→ `0003` 迁移补列。

> **一句话**：`R01`–`R23` 全部闭环。**"某功能还没实现"不是新 finding**——阶段 3/4/5 的内容（同步 pull/push、WS、删除恢复审计、OCR、Flutter）确实还没写，请勿把它们算作缺陷，除非你能证明**设计本身错**或**已实现的部分实现错**。

---

## 3. 必读顺序

| # | 文件 | 为什么 |
|---|---|---|
| 1 | 本卡片 | 现状 + 你上轮结果 + 9 项待验盲区 + 审查焦点 |
| 2 | `workbuddy/多AI对齐规范.md` | §2 十三字段、§3 严重度、§4 证据标准、§6 口径优先级 |
| 3 | `CHANGELOG.md` **[1.14]–[1.24]** | 你上次的基线是 `9a3a2e3`（纯文档）；这 11 个版本是**代码落地**记录 |
| 4 | `docs/FastLedger-开发WBS.md` | 8 阶段任务分解 + §10 验收映射（判断"没实现"是否属计划内） |
| 5 | **`server/`（真实代码，本轮主战场）** | 见第 4 节落点表 |
| 6 | `docs/FastLedger-转账入口裁决.md` | 你自己写的 `GPT-ADR01`，请核对代码是否**忠实实现**了它 |

---

## 4. 阶段 0/1/2 落点表（读代码从这里进）

| 关注点 | 文件:行 | 设计口径 |
|---|---|---|
| 唯一创建入口 | `server/internal/service/tx.go:124` | `Create(ctx, uid, bookID, CreateRequest)` |
| 幂等重放 | `tx.go:238` `replayReceipt` | 命中即返回原结果，**不碰余额、不递增版本** |
| 单笔写入 | `tx.go:270` `createSingleInTx` | 复用外层 sess |
| 转账双边 | `tx.go:289` `createTransferInTx` | 两侧余额+两条记录**同事务同版本**，`related_id` 互指 |
| 负债符号 | `tx.go:334` `signedBalanceDelta` | 10~15 取反（正=欠款） |
| 余额唯一写路径 | `tx.go:349` `createChangeBalance` | 单条原子 UPDATE，带 `book_id` + `deleted=0` + `RowsAffected==1` + `balance BETWEEN` 溢出保护 |
| 命令校验 | `tx.go:531` `validateCreate` | clientUuid 必填、金额 `1..1e10` 分、转账须 `toAccountId` 且 `category=0` |
| 幂等摘要 | `tx.go:576` `createRequestHash` | 域标签 + 完整请求，排除 clientUuid |
| 读接口 | `tx.go:456` List / `tx.go:506` Get | ⚠️ **目前只校验成员身份，未做 §7.3 可见性过滤**（已知，属阶段 3） |
| PRAGMA 初始化 | `server/internal/database/database.go` | DSN 逐连接设 WAL / `busy_timeout=5000` / `foreign_keys=on` / **`synchronous=FULL`**；`MaxOpenConns=1`；**启动校验三项 PRAGMA 真生效** |
| 迁移机制 | `server/internal/migrate/migrate.go` | `schema_migrations` + embed + **事务内执行** + **仅 CLI `db migrate`**（启动只检查版本，不自动改表） |
| 初始 DDL | `migrations/0001_init.up.sql` | 15 张表 |
| 成员唯一索引修复 | `migrations/0002_*.up.sql` | `UNIQUE(book_id,uid)` 与软删冲突 → 改**部分唯一索引** `WHERE deleted=0`；新增 `invites` |
| 审计归属补列 | `migrations/0003_*.up.sql` | `creator_uid` / `source` / `timezone_offset`（均 `NOT NULL DEFAULT 0`）+ `idx_tx_creator` |
| JWT | `internal/service/jwt.go` | **手写 HS256**：限 HS256、**拒 `alg=none`**、拒篡改/过期/错密钥 |
| 权限矩阵 | `internal/service/member.go` | **admin 改删他人交易 = ✅**（`[1.9]` 裁决，非原表"可配置"） |
| 错误映射 | `internal/api/common.go:39` `mapSvcErr` | 400/403/404/**409** 分档；未知错误 500 且不回显 SQL |

**构建（本机原样命令，供你对照环境差异）**：

```bash
export PATH="/c/Users/47006/WorkBuddy/fastledger/.tools/go/bin:$PATH"
cd server
GOTOOLCHAIN=local GOPROXY=https://goproxy.cn,direct GOSUMDB=off GOFLAGS=-mod=mod \
  go build ./... && go vet ./... && go test ./... -count=1
```

依赖版本已固定：gin v1.10.0 / xorm v1.3.9 / `mattn/go-sqlite3` v1.14.22 / `golang.org/x/crypto`。

---

## 5. ⭐ 本轮最该做的事：把下面 9 项跑成断言（附期望值）

本机 **`CGO_ENABLED=0`（无 gcc）**，`go-sqlite3` 只是 stub——实跑二进制会立刻报
`Binary was compiled with 'CGO_ENABLED=0', go-sqlite3 requires cgo to work. This is a stub`。
所以下面 9 项**全部没有运行时证据**。请在你的环境（有 CGO）建一个集成测试并把结果写进
`gpt/v1.5/verification/runtime-results.json`（沿用你上次的文件组织方式）。

| # | 要证明什么 | 建议断言（可直接照抄数值） |
|---|---|---|
| 1 | **同 `clientUuid` 重试不重复扣款** | 账户 A 初始 100000 分；同一请求提交 2 次 → 两次都返回**同一个 tx id / 同一 result_version**，第二次 `replayed=true`，A 余额**只减 10000**（=90000），`transactions` 行数 **1** |
| 2 | **同 `clientUuid` 异内容 → 409** | 第一次金额 10000，第二次 20000 → `ErrCreateConflict` → **409**，余额仍只减 10000 |
| 3 | **`ON CONFLICT ... DO NOTHING` 的 `RowsAffected` 真语义** | 打印两次的 `RowsAffected`：首次 **1**、重放 **0**。若 SQLite 版本不支持 UPSERT（<3.24）或 xorm 包装吞掉行数，此处会暴露 |
| 4 | **负债符号 / 净资产守恒** | 储蓄 1000 元 + 信用卡欠款 0；刷信用卡消费 100 元 → 信用卡 `balance=+10000`，**净资产 = 90000 分（900 元），不是 110000**；再做一笔"还款 100"（资产→负债转账）→ 净资产**仍 90000** |
| 5 | **转账双边原子回滚** | 构造第二笔插入失败（如临时把 `to_account_id` 指向不存在账户，或用触发器注入失败）→ 两条记录都不存在、**两侧余额都没变**、`idempotency_records` 无残留、`book_versions` 未递增 |
| 6 | **`related_id` 双向配对真落库** | 转账后两行：`out.related_id == in.id` **且** `in.related_id == out.id`；两行 `sync_version` **相同**；`type=3`、`category_id=0` |
| 7 | **`xorm:"'type'"` 引号列 + 零值写入** | `SELECT type, to_account_id, deleted, creator_uid FROM transactions` 真值与写入一致（`type` 是 SQL 关键字，这是高风险点）；`creator_uid` 必须等于当前用户，**不可为 0** |
| 8 | **`0002` → `0003` 迁移在真实库可执行** | 空库跑 `db migrate`：应用 3 个迁移、`schema_migrations` 有 3 行、`PRAGMA table_info(transactions)` 含 `creator_uid/source/timezone_offset`、`idx_tx_creator` 存在；再跑一次 → **幂等不改表**、`schemaVersion=3` |
| 9 | **并发 / 串行化** | `MaxOpenConns=1` + `busy_timeout=5000` 下：同 `clientUuid` 的 N 个并发请求 → 恰好 1 笔入账、其余返回原结果或 409，**不得出现 500 / 余额错乱**；不同 `clientUuid` 对同一账户并发 → 余额等于所有成功笔数的代数和 |

**顺带能验的（如果有余力）**：`go test -race ./...`；`user create --owner` 在无用户时可用、建好后再次调用失效；`/healthz` 返回 `schemaVersion`；JWT→handler→service 的真实 Gin 链路（我们只**直接调用函数**验过 `mapSvcErr`，不等于链路验过）。

> **验证边界要求（你自己的规矩，请沿用）**：测试 pass 的含义必须由测试名与报告明确说明；
> **不得**把"编译通过 / 隔离片段通过"描述为"整个后端验收通过"。

---

## 6. 静态审查焦点（按性价比排序）

1. **`GPT-ADR01` 是否被忠实实现**（这是你自己写的裁决书，请你当裁判）：唯一入口、事务边界、余额方向表、幂等回执契约四点，代码有没有打折扣？尤其 `createTransferInTx` 是否真的**没有**另开事务、**没有**再取一次版本。
2. **事务边界**（`tx.go:124`–`370`）：有没有错误路径"先改余额再 return error"却没回滚？`defer sess.Rollback()` 与 `Commit()` 在 xorm 下的交互是否正确（Commit 后再 Rollback 是否安全）？
3. **`BETWEEN` 溢出保护数学**（`tx.go:349` `createChangeBalance`，UPDATE 在 L358–360）：
   ```go
   low, high := int64(math.MinInt64), int64(math.MaxInt64)
   if delta > 0 { high -= delta }
   if delta < 0 { low -= delta }
   // ... WHERE ... AND balance BETWEEN ? AND ?
   ```
   请判定：`delta>0 → high=Max-delta` 是否**严格等价**于"不溢出"？`delta<0` 侧对称吗？会否误伤合法大额？`delta` 本身已被 `maxAmountCents=1e10` 限制，这是否足以保证 `high/low` 自身不溢出？
4. **"禁单边"能否被构造请求绕过**：`validateCreate` 对 `type=3` 的约束（必须有 `toAccountId`、`category` 必须 0）是否覆盖所有入口？`type=3` 但 `toAccountId == accountId`（自转）会怎样？`toAccountId` 指向已删账户/他人账户呢？
5. **错误映射语义**：有没有该 **400** 却映射成 409/500 的？（已知设计：**余额溢出应 400**，因为重试永不成功；并发/已删才 409。）三处曾经 500 的 `errors.New` 已于 `[1.20]` 改 400，请复验。
6. **`migrations/0003`**：`ALTER TABLE ADD COLUMN ... NOT NULL DEFAULT 0` 在**有数据的表**上是否安全？老库（已有 0001/0002 数据）升级会不会锁表/丢数据？**不得删旧回执**这条守住了吗？
7. **隐藏第二入口**：`grep` 一遍是否还有 `/transfer`、`/sync/push`、`AddTransaction`、独立 `Transfer(` 之类的提前实现或残留。
8. **`creator_uid = 0` 哨兵（高危）**：0 = 阶段 2 之前的历史"未知归属"。阶段 3/4 做可见性与"仅本人可改"时，若写成 `if creator_uid != uid { 拒绝 }` → 历史数据**谁都改不了**；若写成 `creator_uid == 0 || creator_uid == uid` → **任何人可改**（越权）。请判断现在是否有任何代码路径已经埋下这两种写法之一。
9. **`PRAGMA` 是否真按 DSN 逐连接生效**：`MaxOpenConns=1` 下不易暴露，但请确认不是"只在第一个连接上设过"。启动校验读的是**同一个 `*sql.DB`** 吗？（我们只验过期望值 `wal/2/1`，没验过连接池语义。）

---

## 7. 已知开放项（**不是"别重复提"，是"请你判定严重度"**）

这三项我们**知道且未修**，请你独立判定它们该是 P? 以及是否阻塞下一阶段：

| # | 开放项 | 我们的判断 | 想听你的 |
|---|---|---|---|
| A | **PRD §7.3「member 仅看本人记录」服务端未实现**：`tx.go:456` List / `:506` Get 只校验成员身份，没读 `books.members_can_view_others`，也没有 `creator_uid` 过滤 | 计划**阶段 3**（依赖刚补的 `creator_uid`） | 是否应在阶段 3 开工前就定下"可见性判定的唯一实现位置"？ |
| B | **`CreateRequest` 无 `Merchant`** → `transactions.merchant` 恒空，而 `List` 的 keyword 会 `LIKE merchant`（后半个条件恒不成立） | 计划**阶段 5**（OCR 识别链路一起补）。注意指南 §13.1 参考骨架**也缺**该字段，属文档缺口 | 死条件是否算缺陷？还是应先把 SQL 收敛为只查 `note`？ |
| C | **DDL 契约仍未对齐**：权威 DDL（`落地方案 §6.4` L699–700、`新手指南` L398–399）有 `related_account_id` / `related_amount`，但 `0001` 没有、`0003` 也**没补** | 未裁决（这是 `0003` 补列时漏掉的第三、四列） | 这两列是否必需？若必需，现在补 `0004` 还是等阶段 3 同步一起补？ |

另：`createChangeBalance` 里 `if n != 0` 那条分支对单行 UPDATE **不可达**（防御性代码），QA 已标记为"已知冗余、保留无害"——如果你认为该删，请给出理由。

另：**邀请撤销入口缺失**——PRD 要求"生成/使用/**撤销**/过期全周期审计"，但 `MemberService` 无 `RevokeInvite`，且 `落地方案 §5.1` 未定义撤销接口。我们**有意未加**（不擅自发明路由契约）。请判定：应在本轮补 `DELETE /books/{id}/members/invites/{inviteId}`，还是等阶段 3 连同同步一起补。

---

## 8. 什么别重复提

| 来源 | 状态 |
|---|---|
| **你的 `R01`–`R18`** | 全部已核实并落地（首轮 11×P1 + 7×P2） |
| **你的 `GPT-R19`–`GPT-R23`** | **全部已在真实代码中实现**，见第 2.2 节落点表 |
| `C-R01`–`C-R10` | 已裁决；C-R03/C-R05/C-R06 落地 `[1.9]` |
| `D-R01`–`D-R06` | 已修 `[1.15]`–`[1.17]`（`audit_logs` DDL、迁移机制、WS 存活、OCR 契约、Flutter 网络层、Pull 分页） |
| 已定决策 | 多账本、增量同步、隐私定位、并发 409、整数分、`synchronous=FULL`、统一 `TxService.Create`、`VersionRepo.Next`、**admin 可改删他人交易**、回收站 30 天 / 审计 1 年、邀请 7 天单次、金额上界 1 亿元/笔 |

**阶段 3/4/5 尚未实现的内容**（同步 pull/push、WebSocket Hub、删除/恢复/审计、报表导出、OCR、Flutter）——**不构成新 finding**。

---

## 9. 提意见格式（否则会被退回）

每条 **`GPT-R24` 起**，必须含 `多AI对齐规范.md` §2 的 **13 个字段**：
`id / title / severity(P0–P3) / location(文件路径 + 章节 + 行号) / quote(原文片段) / problem / reproduce / actual / expected / impact / suggestion / status / evidence`

- **severity**：P0 核心闭环不成立 / 数据损坏 / 安全绕过；P1 安全 / 隐私 / **会计正确性**；P2 跨文档一致性 / 健壮性 / 可维护性；P3 优化。
- **证据**：附**原文片段 + 精确位置**；运行类结论必须附**命令 + 退出码 + 关键输出行**；禁止"可能/大概/看起来"；拿不准标 `[待确认]`。
- **先搜重**：提之前 `grep` 一遍 `CHANGELOG.md` + `workbuddy/` + `gpt/`。
- **请区分三类**，我会**分开处置**：
  1. **设计文档错了** → 改 `docs/`
  2. **代码没照设计做** → 返工 `server/`
  3. **设计对、代码对，但验证不足** → 补测试（这一类请直接给可执行的断言）
- **状态**：新意见起始为 `待确认`，由 owner 裁决后转 `已采纳/未采纳/误判/已修复`。

---

## 10. 交付物建议

1. `gpt/v1.5/审核结果.md`：`GPT-R24` 起的意见（13 字段）
2. `gpt/v1.5/findings.json`：结构化，便于 `grep` 去重
3. **`gpt/v1.5/verification/runtime-results.json`**：第 5 节 9 项的**真实运行结果** ← 本轮最有价值的一份
4. 报告中请显式写明**验证边界**：哪些跑了真库、哪些只做了静态审查、哪些因环境（如无 gcc）没跑成

---

*本卡片由 WorkBuddy 于 `[1.24]` 编写，供 GPT 第三次核查使用。若 `main` 已明显前进，以 `CHANGELOG.md` 与 `server/` 实际代码为准。*
