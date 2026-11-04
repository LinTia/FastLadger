# FastLedger R0 · D-4 裁决草案：期初余额技术实现模型

> 裁决日期：2026-11-04（Owner 九原则于本日通过交互询问给出，本草案据此形成）
> 关联：R0 主报告 §8 D-4；PRD §4.5 / §6.3 / §10.1；`server/internal/service/account.go`；`tx.go`；`model/ledger.go`；`sync.go`
> 关联缺陷：R0-E-06（设置页/账户创建无期初余额入口）

---

## 1. 文档目的与严格边界

本文件是 **D-4 的技术契约与裁决草案**，仅记录核对结论、推荐数据模型、事务流程、同步/审计/迁移契约、受影响清单与回归矩阵。

**明确不在本文件范围内的动作（严格禁止越界）：**

- 不修改任何业务代码、数据库结构、API、PRD 核心需求或 WBS。
- 不启动 R1–R7，不实施 PWA。
- 不把"规划中的期初余额能力"描述为已经部署或已验证；凡规划项一律标注「未实现 / 待 R 阶段」。
- 若发现 PRD、落地方案、设计规范、部署交接之间存在**实质冲突**，单独列出交由 Owner 决定，不自行改写。

---

## 2. 核对结论（PRD / 代码 / 统计 / 同步）

- **PRD §4.5（line 242 / 244）**：账户属性明确含「初始余额」；并写明「账户余额由交易实时计算，**也支持期初余额**」——需求已确定，非开放问题。
- **PRD §6.3（line 366）**：`accounts` 表当前字段为 `id, book_id, parent_id, name, category, currency, balance, sync_version, deleted_at`——**无 `opening_balance` 列**，印证缺口。
- **代码事实（源码实读）**：
  - `account.go:102–103` `Create` 注释「初始余额恒为 0，避免绕过交易入口直接写余额」，且 `account.go:146` 硬编码 `Balance: 0`——这是当前"未满足 PRD"的根因，但注释精神（不绕过统一余额机制）必须保留。
  - `tx.go:376–393` `createChangeBalance` 是**余额变动的唯一实现**（单条原子 `UPDATE accounts SET balance = balance + ? ... WHERE ... AND balance BETWEEN ? AND ?`）；守卫 `TestQAS3SingleCreateEntry` 限制 `insertTxRow` 只走 `createSingleInTx/createTransferInTx`。
  - `model/ledger.go:116` `Balance` 注释「整数分；**负债账户正=欠款**」——符号约定必须被期初余额沿用。
  - `invest.go:544` 净资产按 `balance` 汇总；负债 `balance` 正=欠款须扣减。
  - `sync.go:97` `Accounts []model.Account` 直接复用模型序列化——账户实体随同步自动下发。
- **统计路径**：交易列表 / 报表 / 预算 / 分类统计均查询 `transactions` 表；`balance` 仅用于账户余额与净资产。因此「期初余额若不写入 `transactions`，天然不进入收支/预算/分类统计，但经 `balance` 参与账户余额与净资产」——与 Owner 原则 5 完全契合。
- **冲突核查结论**：PRD 要求期初余额（§4.5），当前实现强制 0（`account.go:102`）——这是**"实现未满足需求"**，不是 PRD/文档之间互相矛盾。方案在不破坏统一余额机制前提下满足 PRD，**无实质文档冲突**。

---

## 3. 推荐数据模型及字段定义（A）

### 3.1 数据库（`accounts` 表，新增两列，迁移 0009）

```sql
ALTER TABLE accounts
  ADD COLUMN opening_balance     BIGINT NOT NULL DEFAULT 0,   -- 整数分；遵循 balance 符号约定（负债正=欠款）
  ADD COLUMN opening_balance_date DATE   NOT NULL DEFAULT '0001-01-01'; -- 生效日期 YYYY-MM-DD
```

### 3.2 `model.Account`（`model/ledger.go:110`）

```go
OpeningBalance     int64  `xorm:"notnull default 0" json:"openingBalance"`      // 整数分；负债正=欠款
OpeningBalanceDate string `xorm:"varchar(10) default '0001-01-01'" json:"openingBalanceDate"` // YYYY-MM-DD
```

### 3.3 入参 `AccountCreateInput`（`account.go:61`）

```go
OpeningBalance     int64  // 单位：分；用户输入金额转为分；必须 ≥ 0
OpeningBalanceDate string // YYYY-MM-DD，可选；缺省为创建当日
```

- **符号约定（原则 4）**：资产与负债均以**正数**表达用户输入金额；存储的 `openingBalance` 直接等于该正数（**不**经 `signedBalanceDelta`，因为期初是静态快照而非现金流）。负债账户存储正=欠款，与 `balance` 符号约定一致。例：信用卡开卡欠款 1000 元 → `openingBalance = +100000`（分），`balance` 初始化为 `+100000`；净资产据此扣减 1000 元。

---

## 4. 账户创建的事务流程（B）

在 `account.go:103` `Create` 现有事务内（`sess.Begin()/Commit()` 已存在）：

1. 校验 `in.OpeningBalance >= 0`（否则 `ErrAccountInvalid`）；`openingBalanceDate` 格式校验（缺省为创建当日）。
2. 构建 `a` 时：
   - `a.OpeningBalance = in.OpeningBalance`（正数快照）；
   - `a.OpeningBalanceDate = in.OpeningBalanceDate`；
   - **`a.Balance = in.OpeningBalance`**（用同一正数初始化运行余额；对负债即 `+100000`）。
3. `sess.Insert(a)` 原子写入（balance 与 opening 同一条 INSERT 内完成 → **原则 6 原子性**）。
4. 审计 `insertAudit(create, entity=account)` 扩展 `details`，补 `openingBalance` 与 `openingBalanceDate`。
5. **不调用 `createChangeBalance`**：期初余额是账户初始态，不是交易驱动的余额变更；所有后续余额变动仍**只**经 `createChangeBalance`（及账户删/恢复的余额回滚）。→ 不引入第二套余额写入口（原则/特别要求）。
6. `Update`（`account.go:179`）仍不支持改 `category`，也**不**暴露 `openingBalance` 编辑（原则 8：创建后原则上不可直接编辑；后续余额纠正作为独立需求评估，不在此擅自扩展）。

---

## 5. 期初余额与普通交易的区别（C）

| 维度 | 期初余额 | 普通交易 |
|---|---|---|
| 存储 | `accounts.opening_balance` 列 + 创建时 seeding 的 `balance` 初值 | `transactions` 行 |
| 是否交易行 | 否 | 是 |
| 余额变更路径 | 仅创建 INSERT 一次 | 仅 `createChangeBalance`（原子 UPDATE） |
| 计入收入/支出/预算/分类统计 | **否**（不写 `transactions`） | 是 |
| 参与账户余额与净资产 | 是（经 `balance` 初值，由 `invest.go:544` 汇总） | 是（经 `balance` 累积） |
| 可编辑 | 创建时设定；创建后原则上不可直接编辑（原则 8） | 可改/删/恢复 |

**结论**：期初余额通过"创建时把 `balance` 初始化为 opening"间接参与余额与净资产，但**绝不**作为交易行出现，因此天然不污染收支/预算/分类统计（原则 5）。

---

## 6. 同步 / 审计 / 删除恢复契约（D）

- **增量同步**：`sync.go:97` `Accounts []model.Account` 直接序列化模型 → 新增 `openingBalance` / `openingBalanceDate` **自动随 pull/push 下发**，无需改同步结构。客户端（Flutter `lib/` + Web `server/web/dist/app.js`）数据模型须补两字段，离线/增量沿用现有 `syncVersion` 机制。
- **审计**：`insertAudit(create, account)` 在现 `name/category/currency/parentId` 基础上补 `openingBalance` / `openingBalanceDate`（A8 口径：创建留痕）。
- **软删除 / 恢复**：账户软删（`deleted=1`）时 `opening_balance` 列随行保留；恢复后原值恢复。余额回滚逻辑（remediation）只动 `balance` 列，不碰 `opening`。
- **离线幂等**：账户同步以 `id` / `client_uuid` 幂等；新增字段不参与冲突判定。
- **并发安全**：opening 仅在创建时写入一次，无并发写路径；交易余额并发仍由 `createChangeBalance` 的 `balance BETWEEN` 守卫 + `sync_version` 保证（原则 7）。

---

## 7. 历史数据迁移方案（E）

- **新迁移 0009**（`server/internal/migrate/migrations/0009_*.up.sql`）：按 §3.1 加两列，旧账户 `opening_balance` 默认 0、`balance` 保持原值。
- **正确性论证（原则 9）**：opening 只在**新建**账户时 seeding 运行余额；旧账户 `balance` 是其真实当前余额，opening 留 0 不影响任何计算——净资产仍 `= sum(balance)`，期初展示对旧账户显示「0 / 未设置」即可。**绝不**把旧 `balance` 回填进 `opening_balance`（否则若未来逻辑把两者相加会重复计算）。
- **迁移守卫**：遵循已发布迁移三道守卫（golden 指纹 + versionAssertions + `TestAssertionCoversAllArtifacts`），0009 必须同步登记，漏一个 CI 必红；已发布迁移（0001–0008）禁改一字节。

---

## 8. 受影响文件 / 函数 / API / 测试清单（F）

| 类型 | 路径 | 变更 |
|---|---|---|
| 模型 | `server/internal/model/ledger.go:110` | `Account` +2 字段 |
| 服务 | `server/internal/service/account.go:61` | `AccountCreateInput` +2 字段 |
| 服务 | `server/internal/service/account.go:103` `Create` | 种子 `balance` + 写 `opening_*` 列 + 审计扩展 |
| API | `server/internal/api/ledger_handler.go:124` | 解析请求 `openingBalance`/`openingBalanceDate` 填入入参（**后端真实落库，不只前端**） |
| 同步 | `server/internal/service/sync.go:97` | 复用模型，字段自动随同步；客户端模型须补字段 |
| 迁移 | `server/internal/migrate/migrations/0009_*.up.sql`（+ down/测试） | 加两列 + 守卫登记 |
| 客户端 | Flutter `lib/` + Web `server/web/dist/app.js` | 账户模型 + 创建表单加字段（R 阶段实现） |
| 文档 | API 契约 / `docs/FastLedger-Web设计规范.md` | 账户创建入参补充两字段 |
| 测试 | `account_test.go`（新增） | 原子创建 `balance==opening`、符号约定、负债正数=欠款 |
| 测试 | `tx_*` / `invest_*` / `core_consistency_probe_*` / `remediation_*` | 回归（直接 model 设 `Balance`，opening 默认 0，不受影响） |
| 测试 | 迁移 golden / `TestAssertionCoversAllArtifacts` | 登记 0009 |

---

## 9. 失败场景与回归测试矩阵（G）

| # | 失败/边界场景 | 期望 | 测试 |
|---|---|---|---|
| 1 | `openingBalance < 0` 或溢出 int64 | 创建拒绝（`ErrAccountInvalid` / 400） | 新增 |
| 2 | 创建后 `balance != openingBalance` | 断言相等（原子性） | 新增 |
| 3 | 负债账户开卡欠款 1000 → `balance=+100000`，净资产减 1000 | 符号正确 | 新增（扩展 `qa_extra_test`） |
| 4 | 期初被计入普通收入 | `transactions` 无新增行；报表/预算查询不含该金额 | 新增 |
| 5 | 删除/恢复账户后 `opening_*` 随行保留 | 软删再恢复，opening 不变 | 扩展 `tx_delete_integration_test` |
| 6 | 迁移后旧账户 `opening=0` 且 `balance` 不变 | 不重复计算 | 迁移测试 |
| 7 | 并发创建账户 + 并发交易不破坏 balance 一致性 | `createChangeBalance` 守卫覆盖；新增并发创建测试 | 扩展 |
| 8 | 同步 account DTO 含 opening 字段，离线/增量不丢 | sync 测试扩展 | 扩展 `sync_r48_test` |
| 9 | 创建后期初不可直接编辑 | `Update` 路径不暴露 opening | 既有 `Update` 测试覆盖 |

---

## 10. 特别要求自检（Owner 强调）

- ✅ **所有余额写入口检查**：现有仅 `createChangeBalance`（`tx.go:392`）与账户删/恢复的余额回滚。D-4 **不在任何地方新增 balance 写入口**；opening 仅在 `Create` 的 INSERT 中作为初始 `balance` 值，不构成"第二套余额变更机制"。
- ✅ **不只改前端**：后端 `account.go Create` + `ledger_handler.go:124` 真实落库 `opening_balance` 列。
- ✅ **不计入普通收入**：opening 是列非交易，天然排除（见 §5）。
- ✅ **不擅自改其他产品决策**：不碰 reports/budget/networth 逻辑（基于 `balance` 自动正确）；不改 category 语义；不加余额调整 UI；不加 opening 编辑入口。

---

## 11. D-4 裁决结论（采纳 Owner 九原则）

D-4 由"技术方案待确认"变更为 **已裁决（技术模型草案，见本文件）**：期初余额以独立 `opening_balance`（整数分）+ `opening_balance_date`（DATE）列实现；在账户创建事务内原子 seeding 运行余额、绝不经由 `createChangeBalance`、不写入 `transactions`；不参与收支/预算/分类统计但经 `balance` 参与账户余额与净资产；资产与负债均以正数表达、遵循现有 `balance` 符号约定；创建后原则上不可直接编辑（后续余额纠正为独立需求）；历史账户 `opening=0` 不重复计算。实现（后端字段/迁移/API/客户端/测试）属 R 阶段，不在 R0，按本契约排期。

---

## 12. 诚实声明

- 本草案为**技术契约与裁决**，**不含任何代码改动**，所列"建议新增"能力均标注「未实现 / 待 R 阶段」，未描述为已部署或已验证。
- 数据模型、事务流程、同步/审计/迁移契约来自 `account.go` / `tx.go` / `model/ledger.go` / `sync.go` 源码实读 + PRD §4.5 / §6.3。
- 凡规划项不得被第三方复核误读为已完成；R0 未通过 / 阻塞状态不因本裁决改变。
