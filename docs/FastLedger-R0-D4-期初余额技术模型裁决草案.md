# FastLedger R0 · D-4 裁决草案：期初余额技术实现模型（定稿版）

> 裁决日期：2026-11-04（Owner 定稿，2026-11-05 修订）；**技术方案已确定、功能未实现、无运行时验收**
> 关联：R0 主报告 §8 D-4；PRD §4.5 / §6.3 / §10.1；`server/internal/service/account.go`；`tx.go`；`model/ledger.go`；`sync.go`
> 关联缺陷：R0-E-06

---

## 1. 文档目的与严格边界

本文件是 **D-4 的技术契约与裁决定稿**。仅记录核对结论、推荐数据模型、事务流程、历史交易规则、同步/审计/迁移契约、余额唯一性、受影响清单与回归矩阵。

**严格边界（不得越界）：**

- 不修改任何业务代码、数据库结构、API、PRD 核心需求或 WBS。
- 不启动 R1–R7，不实施 PWA。
- **明确：技术方案已确定，功能仍未实现；本文件不构成运行时验收，不声称任何能力已部署或已验证。**
- 若发现实质冲突，单独列出交 Owner，不自行改写。

---

## 2. 核对结论

- **PRD §4.5（line 242 / 244）**：账户属性含「初始余额」；「账户余额由交易实时计算，**也支持期初余额**」——需求已确定。
- **PRD §6.3（line 366）**：`accounts` 表当前无 `opening_balance` / 期初时间字段，印证缺口。
- **代码事实（源码实读）**：
  - `account.go:102–103` `Create` 注释「初始余额恒为 0，避免绕过交易入口直接写余额」；`account.go:146` 硬编码 `Balance: 0`——当前未满足 PRD 的根因，但注释精神（不绕过统一余额机制）必须保留。
  - `tx.go:376–393` `createChangeBalance` 是**余额变动的唯一实现**（单条原子 `UPDATE accounts SET balance = balance + ? ... WHERE ... AND balance BETWEEN ? AND ?`）。
  - `model/ledger.go:116` `Balance` 注释「整数分；**负债账户正=欠款**」。
  - `invest.go:544` 净资产按 `balance` 汇总；负债 `balance` 正=欠款须扣减。
- **同步事实（纠正上一版错误表述）**：
  - `sync.go:97` `SyncBatch.Accounts []model.Account`：**Pull 会下发账户实体**，新增字段**可能**随模型序列化下发，但**必须以实际 DTO/序列化测试确认**，不得直接断言"自动支持"。
  - `sync.go:114` `PushItem` 字段仅含 `clientUuid/txType/accountId/toAccountId/amount/categoryId/paymentChannel/txTime/timezoneOffset/comment/source`——**当前 Push 只支持交易，不支持账户创建**；`SyncService.Push`（`sync.go:430`）逐条 `toCreateRequest()` 调 `TxService.Create`。**不得把 Push 描述为已支持账户创建。**
- **冲突核查**：PRD 要求期初余额、实现强制 0——属"实现未满足需求"，非文档冲突。方案在不破坏统一余额机制前提下满足 PRD。

---

## 3. 推荐数据模型（A）

### 3.1 数据库（`accounts` 表，新增两列，迁移 0009）

```sql
ALTER TABLE accounts
  ADD COLUMN opening_balance    BIGINT NOT NULL DEFAULT 0,  -- 整数分；遵循 balance 符号约定（负债正=欠款）
  ADD COLUMN opening_balance_at BIGINT NULL;                 -- Unix 秒；NULL=旧账户无基线
```

### 3.2 `model.Account`（`model/ledger.go:110`）

```go
OpeningBalance   int64  `xorm:"notnull default 0" json:"openingBalance"`       // 整数分
OpeningBalanceAt *int64 `xorm:"null" json:"openingBalanceAt,omitempty"`        // Unix 秒；nil=旧账户无基线
```

### 3.3 入参 `AccountCreateInput`（`account.go:61`）

```go
OpeningBalance   int64  // 分；用户输入金额转为分；必须 ≥ 0
OpeningBalanceAt *int64 // Unix 秒；**新账户创建必须非 nil，即使 opening_balance=0**
```

### 3.4 语义

- `opening_balance_at IS NULL`：**旧账户**，无期初基线，继续沿用原余额逻辑（所有交易均影响 `balance`）。
- **新账户创建必须设置 `opening_balance_at`**，即使 `opening_balance=0`。
- 资产、负债均以**正数**输入；负债账户沿用"`balance` 正=欠款"。存储的 `opening_balance` 直接等于该正数（静态快照，**不经** `signedBalanceDelta`）。
- 例：信用卡开卡欠款 1000 元 → `opening_balance=+100000`（分），`balance` 初始化 `+100000`；净资产据此扣减。

---

## 4. 账户创建的事务流程（B）

在 `account.go:103` `Create` 现有事务内（`Begin/Commit` 已存在）：

1. 校验 `in.OpeningBalance >= 0`；`in.OpeningBalanceAt != nil`（新账户**必须**非 nil，缺省即拒绝，避免"无基线新账户"）。
2. 构建 `a`：
   - `a.OpeningBalance = in.OpeningBalance`；
   - `a.OpeningBalanceAt = in.OpeningBalanceAt`；
   - **`a.Balance = in.OpeningBalance`**（同一正数初始化当前余额；负债即 `+100000`）。
3. `sess.Insert(a)` 原子写入（`balance` 与 `opening_*` 同一 INSERT → 原则"原子完成"）。
4. 审计 `insertAudit(create, account)` 扩展 `details`，补 `openingBalance` / `openingBalanceAt`。
5. **不调用 `createChangeBalance`**：创建 INSERT 是设置初始余额的**唯一受控例外**（见 §8）。
6. `Update`（`account.go:179`）仍不支持改 `category`，也**不**暴露期初基线编辑（创建后不可直接编辑；未来余额纠正另立需求）。

---

## 5. 历史交易规则（Owner 定稿新增，关键）

### 5.1 基线判断（统一、集中）

对每个账户、每笔交易定义：

```
affectsBalance(acct, txTime) =
    acct.OpeningBalanceAt == nil          // 旧账户：沿用原逻辑，全部影响
 || txTime >= *acct.OpeningBalanceAt      // 新账户：等于/晚于基线才影响
```

### 5.2 规则

- 交易时间 **< `opening_balance_at`**：作为**历史记录保存、参与报表，但不改变当前 `balance`**。
- 交易时间 **≥ `opening_balance_at`**：正常影响 `balance`。
- **创建、编辑、删除、恢复必须统一应用同一基线判断**（单一路径，不得各写一套）。
- **转账时间不得早于转出/转入两个账户中较晚的 `opening_balance_at`**；否则**拒绝并提示**。判定：`cutoff = max(非 nil 的 two accounts' opening_balance_at)`，`txTime < cutoff → 拒绝`；两者均 nil 则无约束。

### 5.4 时间输入与 Unix 秒转换契约（实现前补充）

当前模型把 opening_balance_at 存为 Unix 秒，但尚未定义 UI 本地日期/时间到 Unix 秒的唯一转换路径。实现前必须补齐以下契约：

1. 若首期 UI 只提供“期初余额日期”，统一解释为用户所选日期在其本地时区的 **00:00:00**；不得由服务器时区解释该日期。
2. 客户端应依据所选日期在设备当前 IANA 时区规则下构造本地时间，并转换为 Unix 秒后提交；服务端只校验并存储该 Unix 秒，不得再叠加 offset 二次转换。若最终 API 选择传本地日期/时间 + offset，则必须改为服务端唯一转换，并禁止客户端预转换。
3. 现有交易 CreateRequest 含 TxTime 与 TimezoneOffset，但服务层 DTO 仅证明二者被分别接收/存储；不能据此推断 timezoneOffset 的正负号或它参与 txTime 转换。实现前须核对模型注释、各端构造逻辑并统一单位/符号，不能复制未经验证的假设。
4. opening_balance_at 与交易 txTime 均按 Unix 秒比较；边界测试覆盖基线前 1 秒、恰好相等、后 1 秒，以及跨日、UTC 正负偏移、夏令时切换地区、历史交易改期跨基线。
5. 若 MVP 只传固定 offset 而不传 IANA 时区 ID，必须声明限制：固定 offset 不能完整表达用户所选历史日期的夏令时规则；不得宣称支持任意时区历史日期的准确换算。优先采用设备时区规则在客户端生成 epoch 秒，offset 仅作审计/显示元数据。

**验收要求**：同一用户在相同本地日期创建的基线，不因服务端部署时区变化而改变；跨时区客户端必须按各自选择日期的本地午夜转换；服务端不做第二次偏移。
- **对历史交易的修改日期、删除和恢复，必须验证余额增减正确**：日期跨越基线（前↔后）时，按"旧是否影响 / 新是否影响"计算净增减，**不允许重复扣减或回滚**。

### 5.3 实现约束（不新增余额写入口）

- 基线判断是**服务层决策**；真正的余额增减仍**只**调用 `createChangeBalance`。
- 交易创建/修改/删除/恢复中，**不得散落独立余额 SQL**；统一经"基线判断 → 条件调用 `createChangeBalance`"。
- 例：删除一笔 `txTime < opening_balance_at` 的历史交易 → 不调用 `createChangeBalance`（它本就没改 `balance`）；删除一笔 `txTime >= opening_balance_at` 的交易 → 调用回滚。

---

## 6. 期初余额与普通交易的区别（C）

| 维度 | 期初余额 | 普通交易 |
|---|---|---|
| 存储 | `accounts.opening_balance` / `opening_balance_at` + 创建时 seeding 的 `balance` | `transactions` 行 |
| 是否交易行 | 否 | 是 |
| 余额变更 | 仅创建 INSERT 一次 | 仅 `createChangeBalance`（且受基线判断约束） |
| 计入收入/支出/预算/分类统计 | **否**（不写 `transactions`） | 是 |
| 参与账户余额/净资产 | 是（经 `balance` 初值） | 是（经 `balance` 累积，受基线约束） |
| 可编辑 | 创建后不可直接编辑 | 可改/删/恢复 |

---

## 7. 同步 / 审计 / 删除恢复 / 并发契约（D）

### 7.1 同步（纠正上一版）

- **Pull**：`SyncBatch.Accounts` 下发 `model.Account`；新增字段**预期**随模型序列化下发，但**须以实际 DTO/序列化测试确认**（不得仅凭"复用模型"推断）。
- **Push**：**当前不支持账户创建**（`PushItem` 仅交易）。不得把 Push 描述为已支持账户创建。
- **必须单独定义账户创建同步命令**（离线账户创建契约，当前**未定义/未实现**）：
  - 命令载荷：`clientUuid`（幂等键）、账户字段（name/category/currency/parentId/openingBalance/openingBalanceAt）。
  - 幂等：同一 `clientUuid` 重放返回既有结果（成功/已存在），不重复建账户。
  - 重放/冲突：`clientUuid` 内容不一致 → 明确冲突错误；权限不足 → `FORBIDDEN`。
  - 恢复：离线队列在账户创建成功后按 `clientUuid` 确认；失败按错误码分支（可重试/作废/改投）。
  - **不允许只完成在线账户创建而遗漏离线账户创建契约。**
- 客户端（Flutter `lib/` + Web `server/web/dist/app.js`）模型须补 `openingBalance` / `openingBalanceAt`。

### 7.2 审计

`insertAudit(create, account)` 在现 `name/category/currency/parentId` 基础上补 `openingBalance` / `openingBalanceAt`。

### 7.3 软删除 / 恢复

账户软删时 `opening_*` 随行保留；恢复后原值恢复。交易删除/恢复按 §5 基线判断决定是否回滚 `balance`。

### 7.4 并发

期初仅在创建时写入一次，无并发写路径；交易余额并发仍由 `createChangeBalance` 的 `balance BETWEEN` 守卫 + `sync_version` 保证。

---

## 8. 余额唯一性（Owner 定稿明确）

- **账户创建 INSERT 是设置初始余额的唯一受控例外**。
- 账户创建之后，**所有余额变化统一经过现有余额变更机制**（`createChangeBalance`），并受 §5 基线判断约束。
- **不得在交易创建/修改/删除/恢复中散落独立余额 SQL**（不允许 `UPDATE accounts SET balance...` 出现在这些路径）。

---

## 9. 迁移规则（E）

- 旧账户：`opening_balance=0`、`opening_balance_at=NULL`；**原 `balance` 不得改写**。
- **禁止将旧 `balance` 回填为 `opening_balance`**（否则重复计算）。
- 已发布迁移（0001–0008）**不得修改**；新增迁移 0009 必须遵循项目 migration **golden 指纹 / version assertions / TestAssertionCoversAllArtifacts 守卫**，漏一个 CI 必红。
- 正确性：旧账户 `opening_balance_at=NULL` → 沿用原逻辑，所有交易影响 `balance`；`balance` 保持原值，净资产口径不变。

---

## 10. 受影响文件 / 函数 / API / 测试清单（F）

| 类型 | 路径 | 变更 |
|---|---|---|
| 模型 | `model/ledger.go:110` | `Account` +2 字段（`OpeningBalance`、`OpeningBalanceAt *int64`） |
| 服务 | `service/account.go:61` | `AccountCreateInput` +2 字段 |
| 服务 | `service/account.go:103` `Create` | 校验非 nil 基线 + 种子 `balance` + 写 `opening_*` + 审计扩展 |
| 服务 | `service/tx.go`（create/edit/delete/restore 路径） | 统一接入 §5 基线判断，条件调用 `createChangeBalance`；**不新增独立余额 SQL** |
| 服务 | `service/tx_update.go` | 日期跨基线时净增减计算 |
| API | `api/ledger_handler.go:124` | 解析 `openingBalance` / `openingBalanceAt` 填入入参（后端真实落库，不只前端） |
| 同步 | `service/sync.go` | Pull 字段确认测试；**新增账户创建同步命令**（离线契约） |
| 迁移 | `migrate/migrations/0009_*.up.sql` | 加两列 + 守卫登记 |
| 客户端 | Flutter `lib/` + Web `app.js` | 模型 + 创建表单 + 离线队列（R 阶段） |
| 测试 | `account_test.go`（新增） | 原子创建 `balance==opening`、基线必填、符号约定 |
| 测试 | `tx_*` / `tx_update_test` / `tx_delete_integration_test` | 基线判断、跨基线改期、历史交易删/恢复 |
| 测试 | `sync_r48_test` | Pull 字段；账户创建离线命令 |
| 测试 | 迁移 golden | 登记 0009 |

---

## 11. 失败场景与回归测试矩阵（G）

| # | 场景 | 期望 | 测试 |
|---|---|---|---|
| 1 | `openingBalance < 0` / 溢出 / `openingBalanceAt` 缺失（新账户） | 创建拒绝（400） | 新增 |
| 2 | 创建后 `balance != openingBalance` | 断言相等 | 新增 |
| 3 | 负债开卡欠款 1000 → `balance=+100000`，净资产减 1000 | 符号正确 | 新增 |
| 4 | 期初计入普通收入 | `transactions` 无新增；报表/预算不含 | 新增 |
| 5 | 交易时间 < 基线 | 保存参与报表，不改 `balance` | 新增 |
| 6 | 交易时间 ≥ 基线 | 正常改 `balance` | 新增 |
| 7 | 转账时间早于两账户较晚基线 | 拒绝并提示 | 新增 |
| 8 | 改日期跨基线（前→后 / 后→前） | 净增减正确，不重复 | 新增 |
| 9 | 删除/恢复历史交易（< 基线） | 不回滚/不重复扣减 | 扩展 `tx_delete_integration_test` |
| 10 | 删除/恢复正常交易（≥ 基线） | 正确回滚/恢复 | 扩展 |
| 11 | 旧账户（`opening_balance_at=NULL`）迁移后 | 全交易影响 balance，`balance` 不变 | 迁移测试 |
| 12 | 离线账户创建重放/冲突/权限 | 幂等/明确错误码 | 新增 sync 测试 |
| 13 | 并发创建 + 并发交易 | `balance` 一致 | 扩展 |

---

## 12. 特别要求自检

- ✅ 账户创建 INSERT 是初始余额唯一受控例外；之后统一 `createChangeBalance`；无散落余额 SQL。
- ✅ 不只改前端：`account.go` + `ledger_handler.go` 真实落库。
- ✅ 不计入普通收入（列非交易）。
- ✅ 不擅自改其他产品决策（不碰 reports/budget/networth/category 语义）。
- ✅ 历史交易/转账基线规则已明确，创建/编辑/删除/恢复统一判断。
- ✅ 修正 Pull/Push 表述；Push 不支持账户创建；离线账户创建契约单独定义。

---

## 13. 裁决结论

D-4 **技术方案已确定**：期初余额基线（`opening_balance` 分）+ 生效时间戳（`opening_balance_at` Unix 秒，可 NULL）+ 当前余额物化值（`balance`）模型；创建事务内原子 seeding；历史交易按基线判断只记录不改余额；余额变更统一经现有机制；旧账户 `opening_balance_at=NULL` 沿用原逻辑、不重复计算。**功能仍未实现**，无运行时验收；实现属 R 阶段。

---

## 14. 未决技术问题（交 Owner / 后续实现确认）

1. **基线比较精度**：`opening_balance_at` 为 Unix 秒，与交易 `txTime` 的时区/秒级边界是否需对齐到日界？跨日交易与基线的判定口径需定。
2. **历史交易可见性标识**：报表中如何区分"历史（基线前）"与"常规"交易，是否需要 UI 标记/筛选。
3. **改期跨基线的原子性**：改期跨基线时"净增减"与版本号递增、审计写入是否同一事务（应同一事务，待实现确认）。
4. **离线账户创建命令协议**：`clientUuid` 幂等键、重放/冲突/权限/恢复的**具体 envelope 与错误码**需在 R 阶段定型（当前仅契约级）。
5. **`opening_balance_at` 时区来源**：客户端 `timezoneOffset` 与服务端存储的换算规则。
6. **迁移守卫登记**：0009 的 golden 指纹与 version assertions 需在实现时同步登记。
7. **旧账户语义回归**：`opening_balance_at=NULL` 时，编辑/删除/恢复是否**零改动**沿用现有逻辑（应零改动，待回归验证）。

> 以上为**未决技术问题**，不影响本轮"技术方案已确定"的裁决性质；实现前需逐项收敛。

---

## 15. 诚实声明

- 本文件为**技术契约与裁决**，**不含任何代码改动**；所列能力均「未实现 / 待 R 阶段」，**无运行时验收**。
- 数据模型/规则来自 `account.go` / `tx.go` / `tx_update.go` / `model/ledger.go` / `sync.go` 源码实读 + PRD §4.5 / §6.3。
- R0 未通过 / 阻塞状态不因本裁决改变；不得被误读为已完成。
