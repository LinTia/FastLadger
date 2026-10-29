# FastLedger 仓库独立审查报告

审查日期：2026-10-29（北京时间）。仓库：LinTia/FastLadger。

固定审查提交：`b8741465afa98a89ed99796c9e2c4a841b15a361`（main 当时快照）。本报告不是后续提交的自动背书。

结论：目前不宜把文档代码骨架直接用于真实家庭账目。发现 18 项可定位的设计、教学代码或契约问题，其中 11 项 P1、7 项 P2。部分问题并非“没有设计”，而是新决策只落到部分文档/函数，实际可照敲路径仍保留旧实现。

## 审查边界与方法

已先读 `workbuddy/第三方审阅者导读.md`，再核对 README、CHANGELOG、两份前轮评审、权威 PRD、落地方案、开发指南和枚举字典。原型只作页面/产品语义参考，并静态核对构建快照；未执行全入口 UI 测试，也未重新认定“缺页”或遥测问题。

仓库只有设计文档、Markdown 内代码骨架和原型构建产物，没有正式 Go/Flutter 项目、依赖锁定文件或可启动的生产后端。本次不把未实现的需求一概记为 Bug，不宣称已经攻破上线系统，也不把原型内存/模拟数据行为当作正式后端行为。

核查分三类：原文 DDL/SQL 在 SQLite 中运行；将文档明确的算法/协议重放成反例；对完整源码片段进行静态控制流与官方 API 核对。16 个验证场景均复现预期的反例，**这不是“项目功能测试全部通过”**。SQLite 运行版本见 `verification-results.json`。未执行 Go/Flutter 编译、真实 WS 服务、Excel 公式求值、真机 OCR 或线上攻防测试。

优先级按“若照此落地”的影响确定：P1 为应在处理真实账目/开放访问前解决的权限或数据正确性问题；P2 为需在相应功能验收前修复的局部错误或交付阻断。没有仅凭此设计仓库宣布生产 P0。

继续保留已定方案：MVP 多账本、整数分、端侧优先、人工确认、服务端权威与版本号、WAL、事务和原子余额更新；建议旨在把这些规则完整接入实现。

“事实已核实”与“维护者是否采纳”分开。所有新意见的处理状态初始为 **待确认**；维护者按导读在 CHANGELOG 更新为已采纳（含位置）或未采纳（含理由）。不直接改需求/业务代码，不冒称维护者已接受。

### 外部提交结果

上一轮尝试按六个修复领域创建 Issues，第一条请求返回 HTTP 403：`Resource not accessible by integration`，因此没有创建任何 Issue 或 MR。本轮按项目方的新指示，将审查报告、复现脚本、运行结果与六份 Issue 草稿整理到仓库的 `gpt/` 目录。问题处理状态仍全部为待确认，不因交付材料入库而变为已采纳或已修复。审查基线仍是上述固定提交，不把后续提交自动计入本轮验证范围。

## 问题总表

| 编号 | 优先级 | 问题 | 验证方式 |
|---|---|---|---|
| R01 | P1 | 账户引用未绑定账本，且余额更新零行仍返回成功 | 原文 SQL 实测 + Service 源码核对 |
| R02 | P1 | Transfer 忽略数据库错误，交易失败后仍可能提交余额 | SQLite 故障注入 + 原文控制流核对 |
| R03 | P1 | 信用卡负债使用资产增减规则，导致消费增加净资产 | 文档算法反例重放 |
| R04 | P1 | 幂等预检查存在竞态且未限定用户/账本 | 两条真实 SQLite 连接的确定性交错 |
| R05 | P1 | WebSocket 订阅信任客户端账本 ID，广播无法执行成员可见性 | 源码直接确认；未运行真实 WS 服务 |
| R06 | P1 | 全局版本机制未覆盖所有写实体，且部分构造遗漏 book_id | 原文 SQL 实测 + 写路径静态核对 |
| R07 | P1 | 以单条 WS 事件推进全局游标会永久漏同步；pull 缺一致快照约束 | 协议反例重放 + SQLite 读取交错 |
| R08 | P1 | JWT 签发仍使用硬编码密钥，未对接配置中的密钥 | 源码直接确认 |
| R09 | P1 | JWT 认证路径未执行令牌撤销与账号失效检查 | 完整认证函数源码核对 |
| R10 | P1 | 恢复旧备份回退版本计数，已有客户端游标无法发现新数据 | 真实 SQLite backup/restore + 原文增量谓词 |
| R11 | P1 | 交易服务未拒绝非法金额/类型，持仓解析错误被丢弃 | DDL + 算法反例；持仓源码核对 |
| R12 | P2 | 两套 SQL/模型字段和 WS 协议仍不一致 | 两套原文 DDL 实测 + 契约逐项对照 |
| R13 | P2 | 预算 SQL 漏算小类；总预算仍带 category_id 条件 | 原文 SQL 实测 |
| R14 | P2 | CSV 文本字段未防公式解释 | 导出源码 + 原始 CSV 字段保留验证；未在 Excel 中执行 |
| R15 | P2 | OCR 金额正则把千位分隔符金额截断为 1 元 | 原文正则的确定输入反例 |
| R16 | P2 | 数据库/同步示例含 API 编译错误，PRAGMA 初始化未覆盖连接池 | 官方 API/Go 规范核对 + 两 SQLite 连接实测 |
| R17 | P2 | MVP 报表/导出范围仍与权威 PRD 冲突 | 文档原文直接对照 |
| R18 | P2 | WS 订阅 map 并发读写，断连清理路径不完整 | 并发控制流静态确认 |

## 逐项证据与修复建议

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

### R04 · P1 · 幂等预检查存在竞态且未限定用户/账本

验证方式：两条真实 SQLite 连接的确定性交错。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-新手开发完整指南.md · §13.1 L923–L940](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L923-L940)
- [docs/FastLedger-落地方案.md · §4.5 L670–L698](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L670-L698)
- [docs/FastLedger-落地方案.md · §3.5 L324–L327](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L324-L327)

原文片段：

```text
exist, _ := database.Engine.Where("client_uuid=?", tx.ClientUuid).Get(...)
if exist { return nil }
```

复现/核查步骤：

1. 请求 A、B 使用相同 client_uuid；先都在各自事务外查询，均得到不存在。
2. A 完成插入与扣余额后，B 按已通过的预检查继续提交；再用另一个账本重用此 UUID。

实际结果：同一请求被保存两条，10000 分余额变为 8000 分而非 9000 分。DDL 没有该幂等键的唯一约束；另一个账本的相同 UUID 会被直接当作成功而不入账。

预期结果：重试只产生一次业务变更，并返回同一个结果；不同用户/账本不能互相吞单；同一键的不同请求内容应明确冲突。

修复建议：建议改后规则：在写事务内创建带唯一约束的操作回执，键作用域明确到可信 uid/book_id/client_uuid，保存请求摘要及结果 ID；撞唯一约束后返回原结果。转账有两条实体行，不要简单给两侧共同使用的 UUID 加全局唯一索引；可单独建操作回执表，或明确定义逻辑交易主键与行角色。

验证边界：idempotency_check_then_insert、idempotency_scope 已运行；SQLite 串行写不能消除事务外预检查竞态。

与前轮关系：新增发现。

### R05 · P1 · WebSocket 订阅信任客户端账本 ID，广播无法执行成员可见性

验证方式：源码直接确认；未运行真实 WS 服务。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-新手开发完整指南.md · §14.1 L1012–L1016](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1012-L1016)
- [docs/FastLedger-新手开发完整指南.md · §14.1 L1037–L1047](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1037-L1047)
- [docs/FastLedger-新手开发完整指南.md · §14.2 L1054–L1059](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1054-L1059)
- [docs/FastLedger-新手开发完整指南.md · §14.3 L1065–L1088](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1065-L1088)
- [docs/FastLedger-产品需求文档-PRD.md · §7.3 L414–L417](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-产品需求文档-PRD.md#L414-L417)

原文片段：

```text
for _, b := range sub.Books { client.BookIds[b] = true }
if c.BookIds[bookId] { ... c.Send <- msg ... }
```

复现/核查步骤：

1. 源码核对：订阅消息 books 的每个值被直接写入 BookIds，中间没有调用 GetMemberRole。
2. 即使连接层先完成 JWT 登录和初始成员验证，仍核对同账本两名成员关闭互看后的广播分支。

实际结果：处理器接受任意客户端提供的账本 ID。Client 只保存 BookIds/Send，没有 uid/角色；广播把完整交易发给全部订阅者，无法依据 creator_uid 或账本可见性筛选。JWT 连接鉴权本身不能解决账本授权或关闭互看后的隐私隔离。

预期结果：每次订阅都由服务端验证当前成员资格；交易详情推送和 pull/list/export 使用相同可见性规则；移除成员或变更可见性后实时撤销访问。

修复建议：建议改后规则：连接绑定认证用户，订阅仅接收授权账本；广播前按接收者权限过滤，或仅发送不含敏感数据的失效通知，客户端再经授权接口拉取。连接存续期间处理成员移除、权限变化与令牌失效。关闭互看时不向无权成员发送金额、账户、备注等信息。

验证边界：按完整 Client/subscribe/BroadcastToBook 路径静态核对。没有把任意线上环境的可利用性当作已验证事实。

与前轮关系：前轮声称 WS 鉴权已补全；本次给出指南中遗漏成员授权与可见性过滤的具体路径。。

### R06 · P1 · 全局版本机制未覆盖所有写实体，且部分构造遗漏 book_id

验证方式：原文 SQL 实测 + 写路径静态核对。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-落地方案.md · §3.5 L305–L310](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L305-L310)
- [docs/FastLedger-落地方案.md · §6.1 L958–L980](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L958-L980)
- [docs/FastLedger-落地方案.md · §6.1 L1004–L1008](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L1004-L1008)
- [docs/FastLedger-新手开发完整指南.md · §13.1/13.2 L923–L995](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L923-L995)
- [docs/FastLedger-新手开发完整指南.md · §16.1 L1143–L1155](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1143-L1155)
- [docs/FastLedger-落地方案.md · §4.5 L724–L758](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L724-L758)

原文片段：

```text
req.SyncVersion = newVersion
UPDATE accounts SET balance = balance + ?, updated_at = ? WHERE id = ? AND deleted = 0
AccountId: accountId, InstrumentId: instrumentId, ...
```

复现/核查步骤：

1. 账户 sync_version=5；按落地方案开启事务、版本增至 6、插交易 sync_version=6，再执行原文 ChangeBalance。
2. 从 afterVersion=5 拉账户变更；同时核对指南 AddTransaction、Transfer、AddHolding 构造及投资表字段。

实际结果：余额已变为 9000 分，但账户仍为版本 5，增量账户结果为空。指南 AddTransaction/Transfer 没有接入版本递增；Transfer 两侧交易和 AddHolding 均未填 BookId，Transfer 未填交易发生时间。instruments/holdings 没有 sync_version，holdings 也没有软删除墓碑字段，无法按已定通用协议同步这些 MVP 手动录入实体。

预期结果：一次业务事务中所有受影响实体都标记同一个新版本；转账两侧和账户一起同步；记录必须归属正确账本；支持删除增量。

修复建议：建议改后规则：把 bookID/uid 和 newVersion 作为写服务的必需参数，交易、两侧账户、对侧交易等全部在同一事务回写版本；所有启用同步的实体补 sync_version 与删除标记，或明确采用统一变更日志。不只在附录描述规则，要让指南的实际写函数调用它。

验证边界：balance_without_sync_version 已运行；其余字段与调用链为静态确认，未声称缺少已经存在的 book_versions 表。

与前轮关系：v1.1 已建计数器与 Next；本次是“机制存在但写路径未接全”的新增证据。。

### R07 · P1 · 以单条 WS 事件推进全局游标会永久漏同步；pull 缺一致快照约束

验证方式：协议反例重放 + SQLite 读取交错。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-落地方案.md · §3.5 L308–L310](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L308-L310)
- [docs/FastLedger-落地方案.md · §3.5 L324–L326](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L324-L326)
- [docs/FastLedger-新手开发完整指南.md · §14.1 L1041–L1046](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1041-L1046)
- [docs/FastLedger-新手开发完整指南.md · §22 L1495–L1497](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1495-L1497)
- [docs/FastLedger-落地方案.md · §6.1 L973–L980](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L973-L980)

原文片段：

```text
在线成员收到即更新并推进 lastSyncVersion
default: // 客户端缓冲满，跳过
服务端分别查各实体表 ... 更新 ... 为返回的最大值
```

复现/核查步骤：

1. 反例 A：256 条 Send 缓冲满时丢弃版本 257，之后版本 258 成功送达并把游标推进至 258；再 pull(afterVersion=258)。
2. 反例 B：pull 先读交易表，随后另一事务原子提交版本 6 的交易和账户；pull 再读账户表并推进游标至 6。

实际结果：A 中 257 永远不会再被 >258 查询返回。B 中第一次漏掉交易 6、却看到账户 6；下一次 >6 查询也取不到交易 6。转账两侧共享版本号时，单实体事件更不能代表该版本的全部实体已应用。

预期结果：游标必须表示已经完整应用的变更边界；WS 丢包、跨实体并发读取、同版本多行不能导致跳过数据。

修复建议：建议改后规则：区分 WS 最新通知版本与“完整 pull 已应用游标”；用 WS 触发从上次完整游标拉取。pull 在一致读快照中确定水位并读取所有实体；如分页，不能截断同一事务/同一版本的变更组。缓冲溢出要显式要求重同步或断开，不能静默丢弃后继续推进游标；重连采用先订阅缓冲再快照补齐等无窗口策略。无需 CRDT。

验证边界：ws_drop_then_advance、pull_cross_table_snapshot 已运行。B 是对当前协议允许的交错反例，不是声称已有 pull 服务执行时未使用事务。

与前轮关系：新增发现。

### R08 · P1 · JWT 签发仍使用硬编码密钥，未对接配置中的密钥

验证方式：源码直接确认。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-新手开发完整指南.md · §9.1 L663–L665](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L663-L665)
- [docs/FastLedger-新手开发完整指南.md · §10.1 L744–L744](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L744-L744)
- [docs/FastLedger-新手开发完整指南.md · §10.1 L778–L784](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L778-L784)
- [docs/FastLedger-新手开发完整指南.md · §10.2 L801–L812](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L801-L812)
- [docs/FastLedger-落地方案.md · §8.1 L1132–L1134](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L1132-L1134)

原文片段：

```text
var jwtSecret = []byte("改成你自己的密钥")
return token.SignedString(jwtSecret)
func JWTAuth(secret string) gin.HandlerFunc
```

复现/核查步骤：

1. 对照配置 jwt.secret / EBK_JWT_SECRET 与 Login 签发变量来源。
2. 在实现联调时给 JWTAuth 传配置密钥，检查 Login 返回 token 的签名密钥是否相同。

实际结果：签发函数确定使用包级字面量，与传入中间件的密钥没有数据流关系。若中间件读配置，合法登录 token 无法通过；若也照抄公开字面量，签名密钥可预测。第二句为明确条件结果，不声称生产正在使用该值。

预期结果：签发、验证、刷新均使用同一份经校验的配置；空值或示例占位值应拒绝启动。

修复建议：建议改后规则：把密钥随配置注入 AuthService 与 JWTAuth，移除源码中的示例 fallback；启动校验密钥强度与占位值。密钥轮换/令牌失效行为另行明确。

验证边界：静态数据流确认；未生成或尝试任何真实用户凭证。

与前轮关系：前轮已声称改为环境变量；该结论没有传播到可照敲指南。。

### R09 · P1 · JWT 认证路径未执行令牌撤销与账号失效检查

验证方式：完整认证函数源码核对。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-产品需求文档-PRD.md · §4.1 L186–L189](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-产品需求文档-PRD.md#L186-L189)
- [docs/FastLedger-产品需求文档-PRD.md · §8.2 L432–L435](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-产品需求文档-PRD.md#L432-L435)
- [docs/FastLedger-落地方案.md · §4.5 L710–L716](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L710-L716)
- [docs/FastLedger-新手开发完整指南.md · §10.1 L766–L784](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L766-L784)
- [docs/FastLedger-新手开发完整指南.md · §10.2 L808–L819](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L808-L819)

原文片段：

```text
jwt.ParseWithClaims(tokenStr, claims, ... )
c.Set("uid", int64(claims["uid"].(float64)))
c.Next()
```

复现/核查步骤：

1. 逐行跟踪 JWTAuth 从 ParseWithClaims 到 c.Next 的路径，并核对 Login claims。
2. 实现联调应覆盖：有效 token 登录后，撤销该会话/禁用或删除用户，再请求受保护接口。

实际结果：所示中间件只验签/标准 claims，不查询 auth_tokens.revoked 或用户状态；Login 也没有会话 ID/令牌记录。仅修改撤销表不会影响此鉴权函数。uid 还通过无检查类型断言读取，异常结构的已签名 claims 会 panic，而不是返回 401。

预期结果：PRD 承诺的可撤销会话应在失效后拒绝访问，WS 同样失效；无效身份字段稳定返回 401。

修复建议：建议改后规则：采用可查询/撤销的会话标识（如 jti）或不透明令牌哈希，把用户状态和会话状态校验接入共同认证层；安全解析必需 uid、exp，限定允许签名算法与用途。刷新流程不能复活已撤销会话。保留已有 token_hash 设计，不要求 JWT 本身改为明文落库。

验证边界：这是已确认的教学认证路径缺口；因为服务未实现，撤销接口到后端的完整联调尚未执行。

与前轮关系：新增发现。

### R10 · P1 · 恢复旧备份回退版本计数，已有客户端游标无法发现新数据

验证方式：真实 SQLite backup/restore + 原文增量谓词。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-落地方案.md · §8.3 L1174–L1178](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L1174-L1178)
- [docs/FastLedger-落地方案.md · §3.5 L303–L311](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L303-L311)
- [docs/FastLedger-新手开发完整指南.md · §22 L1495–L1497](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1495-L1497)

原文片段：

```text
恢复：停容器 → 替换 db 文件 → 启动
sync_version > afterVersion
```

复现/核查步骤：

1. 数据库版本为 5 时备份，随后客户端同步到版本 10。
2. 恢复版本 5 的备份，再创建一笔版本 6 的交易；原客户端使用 afterVersion=10 拉取。

实际结果：新建交易存在，但查询为空；客户端还可能保留备份时间之后、服务端已经没有的数据。当前恢复步骤没有同步世代变化或要求客户端重建缓存。

预期结果：恢复后客户端能辨认数据库已回退，重新建立一致缓存，并对旧离线队列给出明确处理。

修复建议：建议改后规则：协议加入独立的实例/账本同步世代 restoreEpoch；恢复操作显式生成新世代（不能随旧备份一起恢复旧值）。客户端发现世代不符后执行可审阅的全量重建与离线队列核对。不要仅等计数器再次超过旧游标，也不要未经核对重放旧离线队列。

验证边界：restore_rewinds_sync_counter 已运行；没有损坏或覆盖任何真实用户数据库。

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

### R12 · P2 · 两套 SQL/模型字段和 WS 协议仍不一致

验证方式：两套原文 DDL 实测 + 契约逐项对照。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-落地方案.md · §4.5 L574–L600](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L574-L600)
- [docs/FastLedger-新手开发完整指南.md · §6.4 L295–L317](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L295-L317)
- [docs/FastLedger-新手开发完整指南.md · §9.3 L709–L718](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L709-L718)
- [docs/FastLedger-落地方案.md · §5.2 L831–L838](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L831-L838)
- [docs/FastLedger-新手开发完整指南.md · §7.4 L572–L576](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L572-L576)

原文片段：

```text
password_hash / password
members_can_view_others / members_visible
/ws + subscribe.bookId + transaction.created
/api/v1/ws + subscribe.books + transaction_created
```

复现/核查步骤：

1. 分别执行落地方案 §4.5 与指南 §6.4 的完整 SQL。
2. 对方案库执行 SELECT password FROM users；对指南库执行 SELECT members_can_view_others FROM books。
3. 对照两文档 WS 路径、订阅消息字段和事件名称。

实际结果：分别报 no such column: password、no such column: members_can_view_others。指南 User.Password 未显式映射 password_hash。WS 三处契约同时不同，分别照文档开发的客户端和服务端不能直接互通；需明确唯一口径。

预期结果：按导读优先级，技术方案与枚举字典决定一份权威字段/接口契约，指南复用且可执行。

修复建议：建议改后规则：统一密码哈希列与账本可见性列；给模型明确列/表映射，发布规范的 OpenAPI/WS schema 示例，统一路径、订阅结构、事件名、version 字段与错误响应。指南减少复制 DDL，引用权威 schema 或同源生成，修订 CHANGELOG 记录迁移影响。

验证边界：schema_contract_drift 已运行；WS 为源码/文档静态差异，未用未经实现的接口做假联调。

与前轮关系：新增发现。

### R13 · P2 · 预算 SQL 漏算小类；总预算仍带 category_id 条件

验证方式：原文 SQL 实测。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-落地方案.md · §4.3 L521–L523](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L521-L523)
- [docs/FastLedger-落地方案.md · §4.3 L538–L546](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L538-L546)
- [docs/FastLedger-新手开发完整指南.md · §18.1 L1254–L1266](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1254-L1266)
- [docs/FastLedger-新手开发完整指南.md · §18.1 L1287–L1287](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1287-L1287)

原文片段：

```text
WHERE book_id=? AND category_id=? AND tx_type=2
总预算（budget_type=2）统计时不按分类
```

复现/核查步骤：

1. 给大类 10 创建预算 10000 分，小类 11 的 parent_id=10。
2. 小类发生支出 12000 分，执行 BudgetProgress 中原文 SQL；再对总预算 category_id=0 执行相同函数。

实际结果：大类预算已用 0 分、总预算已用 0 分，预期两者均为 12000 分。代码没有总预算分支，小类也未汇总。

预期结果：分类预算包括该大类及子类支出；总预算不限定分类；删除/修改交易后同样正确。

修复建议：建议改后规则：按 budget_type 分支组装查询。分类预算收集父类和子类 ID（或递归查询），总预算去掉分类谓词；使用明确账本时区下的 [periodStart,periodEnd) 区间避免边界重复，统一启用状态与可见性策略。

验证边界：budget_descendants_and_total 已运行；这是查询错误，不依赖原型 UI。

与前轮关系：新增发现。

### R14 · P2 · CSV 文本字段未防公式解释

验证方式：导出源码 + 原始 CSV 字段保留验证；未在 Excel 中执行。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-新手开发完整指南.md · §17.1 L1222–L1233](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1222-L1233)
- [docs/FastLedger-产品需求文档-PRD.md · §4.9 L267–L271](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-产品需求文档-PRD.md#L267-L271)

原文片段：

```text
w.Write([]string{ ..., t.Comment })
```

复现/核查步骤：

1. 把备注设为无害字符串 =1+1，按代码路径通过标准 CSV writer 导出。
2. 检查 CSV 文本字段，确认仍以 = 开头；后续需在目标 Excel/LibreOffice 版本验证打开与重存行为。

实际结果：备注中的 =1+1 原样保留；语法引号转义没有中和公式前缀。已确认导出编码缺少防护，未声称实测发生外传或命令执行。

预期结果：用户可控文本在面向表格软件的导出中按文本处理；公式能力不应由备注、分类名或账户名意外触发。

修复建议：建议改后规则：对所有用户可控文本字段制定并测试表格软件安全的文本编码策略，覆盖 =/+/−/@ 与前导控制字符；区分给程序消费的原始数据导出与给表格软件查看的安全导出。不能把加 BOM 或普通 csv 引号当作公式防护；保留 JSON 作为原始数据通道。参考 OWASP CSV Injection，兼容性测试包括保存后重开。

验证边界：csv_formula_unneutralized 已运行；仅验证原始字段，不冒充 Excel 端到端复现。

与前轮关系：新增发现。

### R15 · P2 · OCR 金额正则把千位分隔符金额截断为 1 元

验证方式：原文正则的确定输入反例。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-新手开发完整指南.md · §21.5 L1473–L1481](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1473-L1481)
- [docs/FastLedger-产品需求文档-PRD.md · §4.3.2 L215–L218](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-产品需求文档-PRD.md#L215-L218)

原文片段：

```text
RegExp(r'[¥￥]\s*([0-9]+\.?[0-9]*)').firstMatch(text)
```

复现/核查步骤：

1. 给规则解析器输入：微信支付\n付款成功\n¥ 1,234.56。
2. 运行相同正则并按其金额转换分支换算。

实际结果：捕获 1，预填 amount=100 分，而不是 123456 分。人工确认降低了直接错账风险，但不能修正解析规则本身；本次没有把人工确认当作缺失。

预期结果：正确识别合法千位分组与两位小数；歧义或异常格式回退人工，不产出高置信错误金额。

修复建议：建议改后规则：规范化全角/空格与合法千位分隔格式，完整校验金额 token，再用十进制/整数字符串转分；不能无条件删任意逗号，也不要用第一个金额替代付款金额定位。加入普通、千位、负数/退款、多金额与异常分组样例。保留人工确认底线。

验证边界：ocr_thousands_separator 已运行；输入是合成 OCR 文本，没有测试真实截图 OCR 准确率。

与前轮关系：新增发现。

### R16 · P2 · 数据库/同步示例含 API 编译错误，PRAGMA 初始化未覆盖连接池

验证方式：官方 API/Go 规范核对 + 两 SQLite 连接实测。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-新手开发完整指南.md · §22.2 L1519–L1521](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1519-L1521)
- [docs/FastLedger-新手开发完整指南.md · §22.4 L1531–L1540](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1531-L1540)
- [docs/FastLedger-落地方案.md · §6.1 L1012–L1018](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L1012-L1018)
- [docs/FastLedger-新手开发完整指南.md · §18.1 L1262–L1266](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1262-L1266)

原文片段：

```text
txs, _ := ...Find(&[]models.Transaction{})
db, _ := engine.DB()
func (db *xorm.Engine) pragmas() { ... }
```

复现/核查步骤：

1. 对照 xorm 官方签名：Find 返回一个 error；Engine.DB 返回一个 *core.DB。
2. 核对本项目包不能给外包 xorm.Engine 定义方法；SQL(...).Scan 也不在 xorm Session API 中。
3. 在同一数据库打开两连接，仅第一条执行 foreign_keys=ON，再分别查询 PRAGMA。

实际结果：上述两处多值赋值与签名不符，外包接收者方法不合法。两连接的 foreign_keys 结果为 [1,0]，说明 engine.DB().Exec 一次不能保证以后每个池连接启用此设置。PRAGMA Exec 错误也被忽略。

预期结果：指南代码使用真实可编译 API；每条物理连接均具备要求的设置，初始化失败明确报错。

修复建议：建议改后代码形态：db := engine.DB()；txs := []models.Transaction{}; err := sess.Where(...).Find(&txs)；将 pragmas 改为本包函数或包装器方法。用驱动 DSN/ConnectHook 为每个连接设置 _foreign_keys、_busy_timeout 等，WAL 设置并检查返回值；预算聚合改用选定版本实际支持的 Get/SumInt/QueryRow.Scan 方式。固定依赖版本，把教程抽取为编译样例纳入 CI。

验证边界：pragma_connection_scope 已运行；编译阻断按官方签名/语言规范静态确认，当前环境没有 Go/Flutter，因此未声称执行过 go test。

与前轮关系：v1.1 增加了 PRAGMA 文本；本次指出该初始化示例可执行性与连接池作用域，不重复声称“完全没开 WAL”。。

### R17 · P2 · MVP 报表/导出范围仍与权威 PRD 冲突

验证方式：文档原文直接对照。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-产品需求文档-PRD.md · §3.2 L146–L157](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-产品需求文档-PRD.md#L146-L157)
- [docs/FastLedger-产品需求文档-PRD.md · §3.3 L166–L170](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-产品需求文档-PRD.md#L166-L170)
- [docs/FastLedger-落地方案.md · §1.4 L70–L77](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L70-L77)
- [docs/FastLedger-落地方案.md · §8.6 L1195–L1198](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L1195-L1198)
- [docs/FastLedger-新手开发完整指南.md · §27 L1695–L1698](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1695-L1698)

原文片段：

```text
PRD MVP：基础报表（月度收支、分类占比）、CSV/JSON 导出
落地方案：图表统计/报表（后置）
v0.2：账单导出（CSV/JSON）
```

复现/核查步骤：

1. 对照三份文档 v0.1/v0.2 范围，按导读“需求/范围以 PRD 为准”裁决。

实际结果：同一版 MVP 在 PRD 必须能看基础报表和导出，在落地方案/指南却可不交付直到 v0.2。这会使多个 AI 分别生成互相冲突的开发任务与验收结论。

预期结果：所有文档统一 PRD 中的 MVP 基线，后置功能只指增强能力。

修复建议：建议改后文本：v0.1 包含基础月度收支、分类占比和 CSV/JSON 导出；v0.2 增强报表、完整预算、标签及 Web 体验。按此更新落地方案 §1.4/§8.6、指南 §27、WBS 和退出条件。若改变 PRD，需明确产品决策并同步记录，而非默默以后置表覆盖权威范围。

验证边界：静态差异已确认；不重复已解决的“多账本 MVP”议题。

与前轮关系：新增发现。

### R18 · P2 · WS 订阅 map 并发读写，断连清理路径不完整

验证方式：并发控制流静态确认。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-新手开发完整指南.md · §14.1 L1037–L1046](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1037-L1046)
- [docs/FastLedger-新手开发完整指南.md · §14.3 L1074–L1097](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1074-L1097)

原文片段：

```text
client.BookIds[b] = true
if c.BookIds[bookId] { ... }
for msg := range client.Send { ... }
```

复现/核查步骤：

1. 让同一连接的读 goroutine 更新订阅，同时另一业务 goroutine 执行 BroadcastToBook 读取 BookIds。
2. 核对只发生读断连时：Unregister 是否关闭/取消 Send 读循环；写失败时是否统一 Unregister 和关闭连接。

实际结果：订阅写 map 未取得 Hub 的锁，广播读 map 时的 RLock 无法保护该写，形成数据竞争。读错误路径只 Unregister，不关闭或取消写循环，Send 也没有关闭路径；写错误返回又没有统一注销/Close。可留下阻塞的 handler/连接资源。

预期结果：订阅状态所有访问共享同一同步机制；连接任一方向结束都取消另一方向并恰好清理一次。

修复建议：建议改后规则：经 Hub 的受锁方法更新订阅，或使用每 Client 的锁/单 goroutine 所有权；读写循环共用取消上下文，统一 defer unregister/close。关闭 Send 与广播发送也需用同一所有权协议避免向已关闭 channel 写；增加读写 deadline、心跳和消息大小限制。用 go test -race 与反复连接/断开资源测试验证。

验证边界：根据 Go 并发规则与所示访问路径静态确认；没有声称运行过 race detector 或观测过生产崩溃。

与前轮关系：新增发现。

## 待确认事项（不计入 18 项已确认问题）

这些尚不能从当前仓库确认行为，不能写成已经发生的漏洞：

- **权限变化与本地缓存**：互看从开变关、成员被移除/重新加入时，离线缓存、已同步版本以及新获得的可见范围如何重建。历史已经看过或导出的数据无法被服务端“收回”，需要准确说明隐私边界。
- **实例管理员与账本 owner 的边界**：PRD §7.2 L410 把系统设置/备份列在账本角色矩阵中；任何用户都能新建账本成为 owner。需要明确谁能下载整库备份、改 LLM 密钥，不能据此直接断言普通账本 owner 已能获得所有私账。
- **关闭公开注册后的成员入户路径**：首 owner 后 EBK_ENABLE_REGISTER=false，邀请链接是否可注册新用户、邀请码的有效期/单次使用/角色绑定与限流怎样定义。目前只见约定与接口，无足够实现证据判定邀请绕过。
- **可选云端兜底的完整开关**：PRD §12 L535 本来就把默认开启/关闭列为待确认。仍需明确告知/同意、服务端与客户端开关、OCR 文本含账户/姓名等敏感内容时的处理、调用超时和额度限制；不把“只传文字”解释为不含隐私。
- **断电持久性**：落地方案 §6.1 L1018 的 synchronous=NORMAL 是性能/持久性取舍，SQLite 官方说明其 WAL 模式可能在掉电时失去最近已提交事务。需与“无数据丢失”质量目标明确对齐，考虑 FULL 或配套电源/恢复策略；本次未做断电故障注入，也不声称会导致数据库损坏。
- **删除恢复与审计**：软删除不自动等于完整修改审计。恢复入口、恢复转账的双边规则、保留期限和旧离线数据回流仍需定义；现阶段不把未展开的全部恢复 UI 认作实测 Bug。

## 修复顺序与验收建议

1. 先统一权威 schema/API 与可编译样例，明确负债符号，补账本引用/参数校验和完整事务回滚。
2. 接入共同认证与授权层，确保 HTTP、WS、同步、统计、导出都遵循同一数据权限，撤销即时生效。
3. 把幂等回执、所有实体版本回写、完整 pull 水位和恢复世代一起验证；只补一个版本计数器不足以证明同步闭环成立。
4. 修预算、OCR、CSV，并把反例转为正式实现的集成用例；真实应用至少做双用户双账本、并发重试、故障注入、权限变化与备份恢复联调。

## 官方依据（用于静态/API 判断）

- [xorm 官方 API：Engine.DB、Session.Find、聚合接口](https://pkg.go.dev/xorm.io/xorm)
- [Go 规范：方法声明与接收者](https://go.dev/ref/spec#Method_declarations)
- [Go 官方：未保护的 map 并发读写](https://go.dev/doc/articles/race_detector#Typical_Data_Races)
- [SQLite：外键须逐连接启用](https://www.sqlite.org/foreignkeys.html#fk_enable)
- [SQLite：PRAGMA 与 synchronous](https://www.sqlite.org/pragma.html)
- [go-sqlite3：驱动连接参数](https://github.com/mattn/go-sqlite3#connection-string)
- [OWASP：CSV Injection](https://community.owasp.org/attacks/CSV_Injection)

## 复现材料

`reproduce_review.py` 只用 Python 标准库，从指定仓库的两份 Markdown 提取 SQL，构造合成数据并保存反例结果。运行方式：

```bash
git clone https://github.com/LinTia/FastLadger.git
git -C FastLadger checkout b8741465afa98a89ed99796c9e2c4a841b15a361
python3 reproduce_review.py --repo FastLadger --json verification-results.json
```

脚本不是未来正式应用的回归测试替代品，也不会修改传入的仓库或连接任何真实账务服务。16 个场景与 18 个发现不是一一对应：一项发现可有多个反例，静态结论也不伪造运行结果。

## CHANGELOG 待确认记录建议

以下为维护者可直接接入的新审查记录，未自动改变此前处理结论：

- **R01 / P1 / 待确认**：账户引用未绑定账本，且余额更新零行仍返回成功。采纳后补充改动位置；未采纳则保留理由。
- **R02 / P1 / 待确认**：Transfer 忽略数据库错误，交易失败后仍可能提交余额。采纳后补充改动位置；未采纳则保留理由。
- **R03 / P1 / 待确认**：信用卡负债使用资产增减规则，导致消费增加净资产。采纳后补充改动位置；未采纳则保留理由。
- **R04 / P1 / 待确认**：幂等预检查存在竞态且未限定用户/账本。采纳后补充改动位置；未采纳则保留理由。
- **R05 / P1 / 待确认**：WebSocket 订阅信任客户端账本 ID，广播无法执行成员可见性。采纳后补充改动位置；未采纳则保留理由。
- **R06 / P1 / 待确认**：全局版本机制未覆盖所有写实体，且部分构造遗漏 book_id。采纳后补充改动位置；未采纳则保留理由。
- **R07 / P1 / 待确认**：以单条 WS 事件推进全局游标会永久漏同步；pull 缺一致快照约束。采纳后补充改动位置；未采纳则保留理由。
- **R08 / P1 / 待确认**：JWT 签发仍使用硬编码密钥，未对接配置中的密钥。采纳后补充改动位置；未采纳则保留理由。
- **R09 / P1 / 待确认**：JWT 认证路径未执行令牌撤销与账号失效检查。采纳后补充改动位置；未采纳则保留理由。
- **R10 / P1 / 待确认**：恢复旧备份回退版本计数，已有客户端游标无法发现新数据。采纳后补充改动位置；未采纳则保留理由。
- **R11 / P1 / 待确认**：交易服务未拒绝非法金额/类型，持仓解析错误被丢弃。采纳后补充改动位置；未采纳则保留理由。
- **R12 / P2 / 待确认**：两套 SQL/模型字段和 WS 协议仍不一致。采纳后补充改动位置；未采纳则保留理由。
- **R13 / P2 / 待确认**：预算 SQL 漏算小类；总预算仍带 category_id 条件。采纳后补充改动位置；未采纳则保留理由。
- **R14 / P2 / 待确认**：CSV 文本字段未防公式解释。采纳后补充改动位置；未采纳则保留理由。
- **R15 / P2 / 待确认**：OCR 金额正则把千位分隔符金额截断为 1 元。采纳后补充改动位置；未采纳则保留理由。
- **R16 / P2 / 待确认**：数据库/同步示例含 API 编译错误，PRAGMA 初始化未覆盖连接池。采纳后补充改动位置；未采纳则保留理由。
- **R17 / P2 / 待确认**：MVP 报表/导出范围仍与权威 PRD 冲突。采纳后补充改动位置；未采纳则保留理由。
- **R18 / P2 / 待确认**：WS 订阅 map 并发读写，断连清理路径不完整。采纳后补充改动位置；未采纳则保留理由。
