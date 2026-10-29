# FastLedger · 统一枚举字典

> 作用：所有枚举值的**唯一权威来源**。后端、客户端、原型、文档、数据库都以本表为准；新增枚举值只能在末尾追加，不得复用已废弃的数值。
>
> 约定：数据库列用 `TINYINT` 存储；Go 端定义为带 `String()` 方法的常量类型；Flutter 端用对应常量类。**0 一般表示"未指定/默认"，业务枚举从 1 开始。**

---

## 1. 交易类型 TxType（`transactions.tx_type`）

| 值 | 名称 | 说明 |
|---|---|---|
| 1 | Income 收入 | 资金流入 |
| 2 | Expense 支出 | 资金流出 |
| 3 | Transfer 转账 | 账户间划转，不计收支，成对（related_id） |
| 4 | TradeBuy 买入（v0.3 启用） | 投资买入，减少现金、增加持仓 |
| 5 | TradeSell 卖出（v0.3 启用） | 投资卖出，增加现金、减少持仓 |
| 6 | Dividend 分红（v0.3 启用） | 持仓分红 |
| 7 | Interest 利息（v0.3 启用） | 存款/债券利息 |

## 2. 账户分类 AccountCategory（`accounts.category`）

| 值 | 名称 | 资产/负债 | 备注 |
|---|---|---|---|
| 1 | Cash 现金 | 资产 | 含外币现金 |
| 2 | DemandDeposit 活期储蓄 | 资产 | 储蓄卡/活期 |
| 3 | TimeDeposit 定期存款 | 资产 | 预留 |
| 4 | EWallet 电子钱包 | 资产 | 微信零钱/支付宝余额 |
| 5 | Investment 投资账户 | 资产 | 券商/基金/债券/贵金属账户 |
| 6 | PhysicalAsset 实物资产 | 资产 | 实物黄金/房产/汽车/贵重物品 |
| 7 | Insurance 保险/年金 | 资产 | 预留 |
| 8 | OtherAsset 其他资产 | 资产 | |
| 9 | Receivable 应收款 | 资产 | 预留 |
| 10 | CreditCard 信用卡 | 负债 | |
| 11 | ConsumerLoan 信用消费贷 | 负债 | 花呗/白条等 |
| 12 | Mortgage 房贷 | 负债 | |
| 13 | CarLoan 车贷 | 负债 | |
| 14 | OtherDebt 其他借款 | 负债 | |
| 15 | Payable 应付款 | 负债 | 预留 |

> 判定规则：值 1~9 为资产，10~15 为负债。净资产 = Σ(1~9 余额) − Σ(10~15 余额)。
>
> **余额符号约定（负债）**：负债类账户（10~15）的 `balance` 存储为 **正=欠款**（如信用卡欠款 100 元 → `balance = +10000` 分）。因此一笔信用卡消费使 `balance` **增加**（欠得更多），还款（储蓄卡→信用卡的转账）使 `balance` **减少**（欠得更少）。该约定与 §13.1 `signedBalanceDelta`、§16.2 汇总口径一致；**请勿用“负余额”表示欠款**，否则会与上述汇总公式冲突，导致消费虚增净资产（见评审 R03）。

## 3. 支付渠道 PaymentChannel（`transactions.payment_channel`）

| 值 | 名称 |
|---|---|
| 0 | Direct 直接支付（默认，无第三方通道） |
| 1 | WeChat 微信 |
| 2 | Alipay 支付宝 |
| 3 | UnionPay 云闪付 |
| 4 | POS 刷卡 |
| 5 | Other 其他（预留） |

> 支付渠道只是"走账方式"标签，不持有资金；资金以 `account_id` 为准。

## 4. 标的类型 InstrumentType（`instruments.instrument_type`）

| 值 | 名称 |
|---|---|
| 1 | Stock 股票 |
| 2 | Fund 基金 |
| 3 | Bond 债券 |
| 4 | PreciousMetal 贵金属 |
| 5 | Forex 外汇 |
| 6 | Other 其他 |

## 5. 市场 Market（`instruments.market`）

| 值 | 名称 |
|---|---|
| 0 | 未指定 |
| 1 | A股（沪深） |
| 2 | 港股 |
| 3 | 美股 |
| 4 | 场外基金（预留） |
| 5 | 其他市场（预留） |

## 6. 角色 Role（`book_members.role`）

| 值 | 名称 | 权限 |
|---|---|---|
| 1 | Owner 拥有者 | 最高，唯一，不可被移除 |
| 2 | Admin 管理员 | 管成员/分类/账户/设置（不能动 owner） |
| 3 | Member 普通成员 | 记账、查看（受可见性限制）、管本人记录 |

## 7. 预算类型 BudgetType（`budgets.budget_type`）

| 值 | 名称 |
|---|---|
| 1 | CategoryBudget 分类预算 |
| 2 | TotalBudget 总预算 |

## 8. 周期类型 PeriodType（`budgets.period_type`）

| 值 | 名称 |
|---|---|
| 1 | Monthly 月（MVP） |
| 2 | Quarterly 季（预留） |
| 3 | Yearly 年（预留） |
| 4 | Weekly 周（预留） |

## 9. 令牌类型 TokenType（`auth_tokens.token_type`）

| 值 | 名称 |
|---|---|
| 1 | Login 登录令牌 |
| 2 | API 长期令牌 |
| 3 | WS 连接令牌 |

## 10. 数据来源 Source（`transactions.source`）

| 值 | 名称 |
|---|---|
| 0 | Manual 手动 |
| 1 | Recognition 识别 |
| 2 | Import 导入（预留） |

## 11. 可见性开关

| 位置 | 字段 | 粒度 |
|---|---|---|
| 账本级 | `books.members_can_view_others` | MVP 启用，控制全账本默认是否互看 |
| 成员级 | `book_members.can_view_others` | 预留，未来单独覆盖某人 |

---

## 12. 幂等结果类型 IdempotencyResultType（`idempotency_records.result_type`）

| 值 | 名称 | 说明 |
|---|---|---|
| 1 | SingleTransaction | 收入或支出；result_ids 包含一个交易 ID |
| 2 | Transfer | 转账；result_ids 按转出、转入顺序包含两个交易 ID |

0 不是已完成结果类型，不能用它判定重试成功。唯一键仍为 `(book_id,uid,client_uuid)`，不把 result_type 加入唯一键，否则同键跨类型会重复执行。回放必须同时核对 result_type、完整请求摘要及非空结果。

`result_version` 是创建时的账本版本，完成回执必须为正；`result_ids` 与该版本在同一事务回写。该字段是水位，不是新增交易类型枚举。

## 13. 恢复世代字段契约（非枚举）

`instance_id` 为实例固定标识；`restore_epoch` 为每次恢复时递增的正整数，存放于独立的实例元数据，不随旧数据库备份回退。客户端以 `(instance_id,restore_epoch,book_id)` 区分游标世代，不能将 restore_epoch 当作交易 sync_version 或预设枚举值。

---

## 维护规则

1. 新增枚举：在对应表末尾追加，**不复用、不插队**，并在本文档注明启用版本。
2. 重命名：数据库数值不变，只改代码常量名与展示文案。
3. 废弃：保留数值与文档，标注"已废弃"，不删除（避免历史数据无法解释）。
4. 各端同步：改完本表，需同步更新 Go 常量、Flutter 常量、原型下拉项与建表 SQL 注释。
