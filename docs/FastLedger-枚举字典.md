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

## 14. 审计动作与实体（`audit_logs.action` / `audit_logs.entity_type`）

> 依据 PRD §12.1 与 WBS 4.3：`audit_logs` 为 **append-only**，业务路径不得 UPDATE/DELETE；
> 唯一允许的删除是到期滚动清理（`fastledger db purge`，保留期 `EBK_AUDIT_RETENTION_DAYS`，默认 365 天）。

### 14.1 action

| 值 | 含义 | 归属 | 说明 |
|---|---|---|---|
| `create` | 创建 | 账本 | 交易创建（转账两侧各一条）、账本创建、账户/分类创建、邀请签发与兑换入户 |
| `update` | 更新 | 账本 | 账本改名、账户/分类改名或调整上级；交易更新仍未实现（延期） |
| `delete` | 删除 | 账本 | 交易软删（整笔，含转账对侧）、账本软删、账户/分类软删、邀请撤销 |
| `restore` | 恢复 | 账本 | **恢复一笔交易** |
| `login` | 登录 | 实例 | 登录**成功**时写，置于签发令牌之前 |
| `login_failed` | 登录失败 | 实例 | 加固项：登录失败时写，与**失败限流**捆绑（审计写在限流判定**之后**，因此超限请求不写行、写入速率被配额天然封顶，不会给攻击者一个"按请求放大审计表"的开关）。成功事件仍是 `login`，两者不可互相替代 |
| `permission_change` | 权限变更 | 账本 | 改成员角色 / 移除成员 |
| `config_change` | 配置变更 | 账本 | 可见性开关等账本配置（改名走 `update`，不占本值） |
| `backup_restore` | 备份恢复 | 实例 | `restore_epoch` 递增（整实例从备份恢复） |

**`backup_restore` 不得写成 `restore`**：`restore` 已被"恢复一笔交易"占用，混用会让
"这笔交易被恢复过"与"整个实例从备份恢复过"无法区分。

**`login_failed` 与 `login` 不得混用**：前者回答"有人在撞门户"（取证价值在于
"哪个来源在连续失败"），后者回答"谁成功登录了"（`actor_uid` 明确）。把失败也记成
`login` 会让"登录成功次数"这一最基础的运营口径失真。

### 14.2 entity_type

| 值 | 含义 |
|---|---|
| `transaction` | 交易 |
| `user` | 用户（登录等） |
| `book_member` | 账本成员（`entity_id` = 目标 uid；邀请事件里为 `invites.id`） |
| `book` | 账本（`entity_id` = book_id） |
| `account` | 账户（账本级共享配置，`entity_id` = accounts.id） |
| `category` | 分类（账本级共享配置，`entity_id` = categories.id） |
| `instance` | 实例（`entity_id` = 新 `restore_epoch`） |

账户与分类是账本共享配置，PRD §10.9「给定**任一写操作** … 可查到操作者、时间、实体与
变更前后状态」对它们同样成立：删掉一个账户会让历史交易的引用悬空、改掉一个分类会改变
统计口径，这些变更必须可追溯。写操作一律 `book_id` = 该账本（**不是**实例哨兵）。

### 14.3 两个 0 哨兵（必须显式处理，禁止当作"空/放行"）

`audit_logs.book_id` 与 `actor_uid` 都是 `NOT NULL`，没有"空"语义，故用 0 表示：

| 字段 | 0 的含义 | 用途 |
|---|---|---|
| `book_id = 0` | **实例级事件**，不隶属任何账本 | `login`、`login_failed`、`backup_restore` |
| `book_id = 0` | **无法归因的尝试**（与上一行不同来源，靠 payload 区分） | 未认证的邀请兑换：邀请不存在 / 不属于调用方自称的那个账本。此时 `before_json` 带 `"bookAttribution":"unattributable"`；真实归属未知，绝不写进 URL 里的那个 `bookID`，否则匿名者可以往任意账本的审计流灌行 |
| `actor_uid = 0` | **系统 / 运维操作**，无登录用户 | CLI 执行的 `instance bump`；以及未认证入口（邀请兑换）可归因到的唯一主体就是"系统" |

查询约定：`book_id = 0` 即实例级事件，不要把它当成某个真实账本；按账本查审计时应显式排除。
要区分"真正的实例级事件"与"无法归因的尝试"，看 `before_json` 里有没有 `bookAttribution`。

### 14.4 before_json

记录**变更前**状态（软删回答不了"删前是什么"，故必须落原文）。

- `create` / `login`：没有"变更前"状态——交易与账本留空字符串；
  账户/分类例外，记被创建实体的**关键字段**（理由见下方"create 为什么记关键字段"）；
- `login_failed`：没有"变更前实体"，只记失败诊断：`{"attempted":<截断后的用户名>,"reason":<失败原因>}`。
  **绝不记口令**（口令进审计等于把凭据落进备份与日志采集）；
- `delete` / `restore`：序列化被操作的**整行**（含 `deleted` 变更前取值）；
- `permission_change`：`{"uid":<目标uid>,"role":<变更前角色>}`；
- `config_change`：`{"membersCanViewOthers":<变更前bool>}`；
- `backup_restore`：`{"restoreEpoch":<变更前世代>}`。

各实体的具体落法（新增写入点必须照此填写，否则"变更前后状态"这一半就丢了）：

| 实体 | 动作 | before_json 内容 |
|---|---|---|
| `transaction` | `create` | 空 |
| `transaction` | `delete` / `restore` | 该笔交易整行（转账两侧各一条，`related_id` 可互查） |
| `book` | `create` | 空 |
| `book` | `delete` | **账本整行**（`books` 表模型未带 json tag，故字段名为 Go 字段名：`Id`/`Name`/`OwnerUid`/`BookType`/`DefaultCurrency`/`MembersCanViewOthers`/`Deleted`/`CreatedAt`/`UpdatedAt`/`DeletedAt`）。必须含 `"Deleted":false`——它就是"删除前状态"的直接证据；`deleted_at` 之类只在删除后才有的值不得出现在这里 |
| `book` | `update` | `{"name":<变更前名称>}`，仅在名称**实际变化**时写；改可见性开关仍走 `config_change` |
| `account` | `create` | 被创建实体的关键字段：`{"name","category","currency","parentId"}` |
| `account` | `update` | `{"name":<变更前名称>,"parentId":<变更前上级>}` |
| `account` | `delete` | **账户整行**（带 json tag，camelCase）：`id`/`name`/`category`/`currency`/`balance`/`parentId`/`deleted:false` 等 |
| `category` | `create` | 被创建实体的关键字段：`{"name","parentId","catType"}` |
| `category` | `update` | `{"name":<变更前名称>,"parentId":<变更前上级>,"catType":<收支类型>}` |
| `category` | `delete` | **分类整行**：`id`/`name`/`parentId`/`catType` 等 |

**create 为什么记关键字段**：账本与交易的 create 留空，是因为"这本账叫什么"能从
`books` 行直接读到，而交易行本身字段齐全、`clientUuid` 还能对上幂等记录。
账户/分类不同：它们属于账本共享配置，**同名的历史行会被反复重建**（删了再建一个同名的），
只凭 `entity_id` 回查当前行无法回答"当时建的是哪一类账户、挂在哪个上级下"。
在 create 行里带上关键字段，等于给配置变更留了一条不依赖当前数据形态的时间线。
这是对"create 留空"的一处**有意例外**，不是遗漏。

**为什么没有 after_json**：审计表是 append-only，只记"变更前"。要回答"变更后是什么"，
用 `at` + `entity_id` 去读当前业务行即可；业务行已被软删的，`deleted`/`deleted_at` 就是
变更后的证据。多写一份 after 快照只会让同一事实有两处可能不一致的来源。

**读与写的顺序要求**：delete 的整行快照必须在**同一事务内、UPDATE 之前**读出
（GPT-R29 的"先读后写"教训）。事务外读或 UPDATE 之后再读，`before_json` 记到的
可能是并发改过、甚至已经是删除后的值——那样这份审计就只是在自欺欺人。

### 14.5 关联口径：限流桶键的来源聚合（非枚举，但与 §14.3 的 `book_id = 0` 同属"归属"问题）

邀请入户与登录失败两条限流的桶键都按**来源网段**聚合：

| 来源形态 | 聚合粒度 | 桶键示例 |
|---|---|---|
| IPv4 | `/24` | `203.0.113.0/24` |
| IPv6 | `/64` | `2001:db8:1::/64` |
| IPv4-mapped IPv6（`::ffff:a.b.c.d`） | 先 `Unmap()` 再按 IPv4 的 `/24` | `203.0.113.0/24` |
| 不可解析 / 空 | 共享 `unknown` 桶（fail-closed，绝不因解析失败而拿到新配额） | `unknown` |

**明确取舍**：同一 `/24`（IPv4）或 `/64`（IPv6）内的多台设备**共享**一个配额。
理由与代价：IPv6 隐私扩展（RFC 4941）让一台机器在自己 `/64` 内可轮转出海量地址，
若按完整地址分桶，配额会被地址数摊薄到形同虚设；家庭自托管场景下，同一出口路由器
后面的设备本来就该算同一个来源，故接受"同网段共享配额"。两条限流**必须同口径**——
登录侧与邀请侧粒度不一致时，运维按一侧的经验推断另一侧行为会得出错误结论。

失败尝试的审计里 `clientIP` 字段记录的就是这个**聚合后的桶键**（不是原始地址）：
取证只需要"哪个网段在撞"，不需要也不应该把一个家庭网络里某台设备的确切地址长期留在库里。

---

## 维护规则

1. 新增枚举：在对应表末尾追加，**不复用、不插队**，并在本文档注明启用版本。
2. 重命名：数据库数值不变，只改代码常量名与展示文案。
3. 废弃：保留数值与文档，标注"已废弃"，不删除（避免历史数据无法解释）。
4. 各端同步：改完本表，需同步更新 Go 常量、Flutter 常量、原型下拉项与建表 SQL 注释。
