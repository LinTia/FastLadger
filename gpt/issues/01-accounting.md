# [审查] 账务写入：账本引用校验、转账回滚、负债符号与非法参数

审查基线：`main@b8741465afa98a89ed99796c9e2c4a841b15a361`。按 `workbuddy/第三方审阅者导读.md` 完成前轮去重与权威文档对齐。

本仓库尚无正式后端工程。以下是**设计/教学代码/契约问题**，并非声称已经验证线上漏洞。SQL/算法反例与静态结论各自标注验证边界；不对原型生产安全作推断。

本 Issue 包含同一修复域的 R01, R02, R03, R11；处理状态均先为 **待确认**，请逐项更新 CHANGELOG 为“已采纳（含位置）/未采纳（含理由）”，并保留被证伪结论的纠正记录。

处理清单：

- [ ] R01：账户引用未绑定账本，且余额更新零行仍返回成功
- [ ] R02：Transfer 忽略数据库错误，交易失败后仍可能提交余额
- [ ] R03：信用卡负债使用资产增减规则，导致消费增加净资产
- [ ] R11：交易服务未拒绝非法金额/类型，持仓解析错误被丢弃

### R01 · P1 · 账户引用未绑定账本，且余额更新零行仍返回成功

验证方式：原文 SQL 实测 + Service 源码核对。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-落地方案.md · §6.1 L1004–L1008](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L1004-L1008)
- [docs/FastLedger-新手开发完整指南.md · §13.1 L946–L959](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L946-L959)
- [docs/FastLedger-新手开发完整指南.md · §13.2 L969–L984](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L969-L984)
- [docs/FastLedger-落地方案.md · §4.4 L562–L566](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L562-L566)

原文片段：

```text
UPDATE accounts SET balance = balance + ?, updated_at = ? WHERE id = ? AND deleted = 0
_, err := sess.Exec(...)
return err
```

复现/核查步骤：

1. 建立账本 A、B；A 的成员发起一条 book_id=A、account_id 指向 B 的交易，执行文档中的 ChangeBalance SQL。
2. 再以不存在的 account_id=999 插入交易并执行相同余额 SQL；保留其 affected rows 结果。

实际结果：B 的账户从 10000 分变为 9000 分。不存在的账户更新影响 0 行，但 SQL 无错误，交易孤儿行仍可提交。完整 DDL 没有 REFERENCES，PRAGMA foreign_keys=ON 也不会补出这些约束。仅检查用户是路径账本成员不足以阻止该情况。

预期结果：路径账本、交易账本、资金账户、转账对侧账户、分类等引用必须一致且有效；不存在/已删除引用应报错并整体回滚。

修复建议：建议改后规则：Service 接受可信的 bookID/uid，在同一事务中校验全部关联实体；余额 UPDATE 带 book_id 与有效状态条件，检查 RowsAffected()==1。禁止从请求体信任 book_id/creator_uid。账户、分类等写入口同样适用。可用同账本复合外键作第二层保护，但不能代替角色与状态校验。

验证边界：cross_book_account、missing_account_zero_affected 已在 SQLite 运行；未声称上线 HTTP 接口已被越权。

与前轮关系：新增发现。

### R02 · P1 · Transfer 忽略数据库错误，交易失败后仍可能提交余额

验证方式：SQLite 故障注入 + 原文控制流核对。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-新手开发完整指南.md · §13.2 L969–L995](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L969-L995)
- [docs/FastLedger-产品需求文档-PRD.md · §9 L460–L460](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-产品需求文档-PRD.md#L460-L460)

原文片段：

```text
sess.Begin()
sess.ID(fromId).Cols("balance").Update(from)
sess.Insert(out)
sess.Insert(in)
return sess.Commit()
```

复现/核查步骤：

1. 初始两账户余额 10000/0 分，模拟转账 2000 分。
2. 给 transactions 加一条测试触发器，在 INSERT 时 RAISE(ABORT, ...)，按文档控制流忽略插入错误并 Commit。

实际结果：SQLite 事务提交成功；余额为 8000/2000 分，交易记录为 0 条。文档也未检查 Begin、Get、Update、关联关系回写错误，Get 的 has=false 同样被忽略。事务存在不等于业务自动回滚。

预期结果：任一必要步骤失败，函数返回错误；余额、两侧交易、关联字段与版本号全部保持原状。

修复建议：建议改后规则：用统一事务封装执行转账，每个 SQL 检查错误及必要的影响行数；失败即返回并回滚，提交成功后才发事件。把指南的先读后写余额实现替换为已确定的原子 ChangeBalance；此处是已修方案未同步到指南的新证据。

验证边界：transfer_ignored_insert_errors 已运行。验证的是 SQLite 的语句错误/提交语义与文档控制流，未运行 Go/xorm 服务。

与前轮关系：前轮称转账事务问题已解决；本次定位的是指南仍吞错及旧余额写法，不否认落地方案已有事务框架。。

### R03 · P1 · 信用卡负债使用资产增减规则，导致消费增加净资产

验证方式：文档算法反例重放。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-新手开发完整指南.md · §13.1 L946–L956](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L946-L956)
- [docs/FastLedger-新手开发完整指南.md · §13.2 L974–L999](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L974-L999)
- [docs/FastLedger-新手开发完整指南.md · §16.2 L1173–L1189](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1173-L1189)
- [docs/FastLedger-枚举字典.md · §2 L34–L41](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-枚举字典.md#L34-L41)
- [docs/FastLedger-落地方案.md · §4.1 L376–L378](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L376-L378)

原文片段：

```text
if tx.TxType == 2 { account.Balance -= tx.Amount }
to.Balance += amount
netAssets: totalAssets - totalLiab
```

复现/核查步骤：

1. 储蓄余额 1000 元，信用卡欠款 0 元；用信用卡消费 100 元。
2. 按指南新增交易和资产汇总算法计算，再从储蓄卡还款 100 元。

实际结果：消费后信用卡 balance=-100 元，净资产变成 1100 元；还款后净资产又变成 900 元。一笔纯转账改变了净资产，消费则使其增加。

预期结果：消费后净资产 900 元；还款前后均为 900 元。负债增加/减少与展示、汇总采用同一种符号约定。

修复建议：建议改后规则：明确存储负债为正欠款还是有符号余额，并统一消费、收入/退款、转入/转出、期初值、删除/恢复、净资产公式。若用正欠款，负债账户消费增加欠款，还款减少欠款；若用有符号余额，汇总时不能再次按正欠款扣减。保留整数分，不需要推翻既定账户分类。

验证边界：credit_card_sign 已重放，输出消费后 110000 分、还款后 90000 分，预期始终为 90000 分。

与前轮关系：新增发现。

### R11 · P1 · 交易服务未拒绝非法金额/类型，持仓解析错误被丢弃

验证方式：DDL + 算法反例；持仓源码核对。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-产品需求文档-PRD.md · §10.1 L475–L476](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-产品需求文档-PRD.md#L475-L476)
- [docs/FastLedger-新手开发完整指南.md · §13.1 L923–L962](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L923-L962)
- [docs/FastLedger-新手开发完整指南.md · §13.2 L969–L995](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L969-L995)
- [docs/FastLedger-新手开发完整指南.md · §16.1 L1143–L1154](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1143-L1154)
- [docs/FastLedger-落地方案.md · §4.5 L674–L678](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L674-L678)

原文片段：

```text
account.Balance -= tx.Amount
qty, _ := decimal.NewFromString(quantity)
price, _ := decimal.NewFromString(currentPrice)
```

复现/核查步骤：

1. 调用写服务记一条 amount=-1000 的支出；再插入 tx_type=99 的交易。
2. 静态核对 AddHolding 对非法 quantity/currentPrice 的错误处理。

实际结果：10000 分余额因负支出增加至 11000 分；未知类型可以插交易，但没有对应余额变化。表中没有金额/枚举 CHECK，Service 也不拒绝。持仓十进制转换错误被忽略，仍构造持仓并尝试 Insert；并非所有不合法输入都能被当前代码阻止。

预期结果：服务端拒绝非法金额、启用范围外的枚举和非法持仓数值，事务前/内校验失败不修改任何数据；不能仅依赖 UI。

修复建议：建议改后规则：统一 DTO 与 Service 校验金额正值、上限、整数分、枚举白名单、数量/价格十进制有效性、范围和符号；明确退款等负金额业务的专门语义。decimal 的错误必须返回；计算到 int64 前做溢出和舍入检查，必要时配数据库 CHECK。

验证边界：invalid_transaction_values 已运行；非法 decimal 路径为静态确认，未虚构某个 decimal 版本的具体零值/报错结果。

与前轮关系：新增发现。

维护者复核说明：复现脚本及完整报告由本轮审阅交付；不直接替换业务方案。对已有修复的补充项已注明具体旧代码残留位置，避免把“新增机制存在”误认为“所有写路径均已闭合”。
