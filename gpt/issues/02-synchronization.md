# [审查] 同步一致性：幂等竞态、实体版本、完整游标和备份恢复

审查基线：`main@b8741465afa98a89ed99796c9e2c4a841b15a361`。按 `workbuddy/第三方审阅者导读.md` 完成前轮去重与权威文档对齐。

本仓库尚无正式后端工程。以下是**设计/教学代码/契约问题**，并非声称已经验证线上漏洞。SQL/算法反例与静态结论各自标注验证边界；不对原型生产安全作推断。

本 Issue 包含同一修复域的 R04, R06, R07, R10；处理状态均先为 **待确认**，请逐项更新 CHANGELOG 为“已采纳（含位置）/未采纳（含理由）”，并保留被证伪结论的纠正记录。

处理清单：

- [ ] R04：幂等预检查存在竞态且未限定用户/账本
- [ ] R06：全局版本机制未覆盖所有写实体，且部分构造遗漏 book_id
- [ ] R07：以单条 WS 事件推进全局游标会永久漏同步；pull 缺一致快照约束
- [ ] R10：恢复旧备份回退版本计数，已有客户端游标无法发现新数据

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

维护者复核说明：复现脚本及完整报告由本轮审阅交付；不直接替换业务方案。对已有修复的补充项已注明具体旧代码残留位置，避免把“新增机制存在”误认为“所有写路径均已闭合”。
