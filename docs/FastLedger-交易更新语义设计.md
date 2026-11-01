# FastLedger 交易更新语义设计（WBS 5.9b 前置裁决）

> 状态：已裁决，随实现一并交付。本文档是 CI 守卫 `TestQATransactionRoutesContract` 解除 `PUT` 禁令的依据——
> CHANGELOG [1.44] 约定：**先写本设计，再实现完整更新能力（转账对侧联动 + 余额重算 + 幂等摘要域 + 审计），完成后转 allowed**。

## 1. 路由与命令

```
PUT /api/v1/books/:id/transactions/:txId
Content-Type: application/json
Authorization: Bearer <token>
```

请求体（**全量替换**语义，PUT 不是 PATCH；所有业务字段必填，与创建命令对齐）：

| 字段 | 类型 | 约束 |
|---|---|---|
| clientUuid | string | 必填，≤64 字节，幂等键（op=update 空间） |
| txType | int | **必须与既有行一致**（1/2/3）；不一致 → 400 |
| amount | int64 | >0 且 ≤1 亿元（整数分，禁 float） |
| accountId | int64 | 必填，本账本未删除账户 |
| toAccountId | int64 | 仅转账必填，≠accountId；单笔必须为 0 |
| categoryId | int64 | 仅单笔必填，cat_type 与 txType 一致；转账必须为 0 |
| paymentChannel | int | 0~5（枚举字典 §3） |
| txTime | int64 | >0（Unix 秒） |
| timezoneOffset | int | -840~840 |
| comment | string | ≤255 字，合法 UTF-8 |
| baseVersion | int64 | 可选乐观锁：>0 时要求交易行 sync_version 相等，否则 409 |

**不可变字段**（任何途径不可改）：`txType`、`creatorUid`、`source`、`merchant`、行上的 `clientUuid`、`direction`、`relatedId`。

## 2. 类型不可改（裁决）

收入↔支出↔转账的互转必然伴随"转账配对行的诞生/销毁"，与《转账入口裁决》"唯一创建入口 + 整笔同生共死"两条不变量正面冲突：
单笔→转账需要凭空造出对侧行，转账→单笔需要销毁一侧（余额、审计、幂等回执都要解释"另一半去哪了"）。

MVP 裁决：**改类型 = 删除 + 重建**（两条路径均已实现：整笔软删 + 余额精确反向 + 幂等 + 审计）。
请求 txType 与既有行不一致时返回 400，文案引导用户"如需变更类型请删除后重新记账"。

## 3. 权限与可见性（与 Delete 完全同口径）

- 非成员 → 403；member 仅能改 `creator_uid == 本人` 的交易；`creator_uid=0`（遗留哨兵）仅 owner/admin 可改；
- 角色在**写事务内**重读（GPT-R28，防 TOCTOU），复用 `roleOfTx` / `CanEditOthersTx`；
- 可见性开关（关闭互看）不改编辑语义：编辑权由角色 + 归属决定，与阶段 4 删除一致（读路径 404 口径不受影响）。

## 4. 转账整笔联动（禁止单边更新）

复用 `collectTransferGroup(sess, bookID, row, wantLive=true)` 定位整笔（对侧必须存在、互指、四项对应，损坏即 409 整笔拒绝）。
更新内容双侧对称施加：

- 转出行：`account_id=新 from，to_account_id=新 to`；转入行镜像；
- `amount / tx_time / note / payment_channel / timezone_offset / sync_version / updated_at` 双侧同值；
- 更新后两侧仍满足 `verifyTransferPair`（由 group 数据构造保证，配对键 related_id 不动）。

单笔（type≠3）带 `related_id` 属数据损坏 → 整笔拒绝（复用现有守卫）。

## 5. 余额净额重算

原则：**差额计算、原子写入、只动变化账户**。

1. 每行旧施加量：`oldFlow = txRowFlow(旧行)` → `oldDelta = signedBalanceDelta(旧账户.Category, oldFlow)`；
2. 每行新施加量：同公式，账户/分类用新请求重新 `loadTxAccount` / `loadCategoryForType` 校验（未删、CNY、类型匹配；转账两侧同币种）；
3. 按 account_id 聚合 `netDelta = Σ new − Σ old`，**净额为 0 的账户跳过**（不碰 balance/sync_version/updated_at）；
4. 逐账户走 `createChangeBalance`（唯一余额写入路径：原子 UPDATE + 溢出边界 + RowsAffected 校验），全组共享一个新版本号。

负债账户符号由 `signedBalanceDelta` 统一处理，更新路径不引入第二套符号逻辑。

## 6. 乐观锁 baseVersion

- 缺省/0：last-write-wins（家庭场景低并发，Web 端 MVP 可接受）；
- >0：对整笔每一行 `UPDATE ... WHERE id=? AND book_id=? AND deleted=0 AND sync_version=?`，0 行 → `ErrConcurrentConflict`（409，客户端重拉后再试）。
- 行更新一律显式刷 `sync_version` 与 `updated_at`（GPT-C03：裸 SQL 绕过 xorm 自动列的教训）。

## 7. 幂等（op=update，无需迁移）

- 复用 `idempotency_records`：`op="update"`（新增常量 `IdemOpUpdate`）。`op` 为自由 VARCHAR(16)、无 CHECK 约束，唯一键 `(book_id, uid, op, client_uuid)` 天然容纳，**零迁移**；
- 摘要域 `transaction.update/v1`（与 create/mutate 域分离，同一 clientUuid 跨 op 天然不同）；
- 占位 `INSERT ... ON CONFLICT DO NOTHING` → 重读回执核对 `request_hash`：一致返回原版本 + `replayed=true`，不一致 `ErrCreateConflict`（409）；
- **重放不做终态校验**（与 delete/restore 的 GPT-C01 不同）：更新重放不产生任何写入，无余额双算风险；重放返回的是"该次更新的回执版本"（与 Create 重放同语义），后续其他更新各自持有独立回执，客户端游标由 pull 水位驱动。

## 8. 审计与版本

- 每行一条 `AuditActionUpdate` 审计，`before_json` = 变更前整行快照（与 delete/restore 同口径），与业务写入同事务；
- 整笔一次 `versions.Next`（单笔/转账共享一个版本），交易行与受影响账户行共用该版本；
- 提交后 handler 按 GPT-C04 口径广播：`replayed=false` 才 `notifyDataChanged`。

## 9. 错误映射（全部复用 mapSvcErr，零新增错误类型）

| 场景 | 错误 | HTTP |
|---|---|---|
| 类型变更/字段非法 | ErrInvalidCreate / ErrInvalidArgument | 400 |
| 非成员/改他人 | ErrForbidden | 403 |
| 不存在（含已删按 404 口径） | ErrTransactionNotFound | 404 |
| 同键不同内容 | ErrCreateConflict | 409 |
| baseVersion 过期 | ErrConcurrentConflict | 409 |
| 配对损坏/方向缺失 | 现有 409 映射 | 409 |

## 10. 守卫与测试清单

- `TestQATransactionRoutesContract`：`PUT /api/v1/books/:id/transactions/:txId` 移入 allowed；`/transfer`、`/batch`、`/sync/push` 继续禁止（第二条创建路径裁决不变）；
- 服务层测试（复用 testEnv）：单笔改金额净额、改账户搬运、负债账户符号、转账改金额双侧、转账改账户、类型变更 400、幂等重放（余额/版本不变）、同键冲突、member 越权 403、已删 404、baseVersion 冲突、审计 before_json、版本单次递增、空净额不碰账户行；
- 服务器全量 plain + race（api/service 单包）+ `gofmt -l` 为空。

## 11. Web 端（5.9b 同批）

交易列表增加"编辑"入口：预填表单（金额分↔元字符串转换禁浮点、分类/账户下拉、备注、时间），提交 `PUT` 携带随机 clientUuid 与列表行 syncVersion 作 baseVersion；409 时提示刷新重试。XSS 防护沿用全 textContent 口径。
