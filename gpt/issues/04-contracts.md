# [审查] 文档契约：数据库/WS 口径、初始化代码与 MVP 范围漂移

审查基线：`main@b8741465afa98a89ed99796c9e2c4a841b15a361`。按 `workbuddy/第三方审阅者导读.md` 完成前轮去重与权威文档对齐。

本仓库尚无正式后端工程。以下是**设计/教学代码/契约问题**，并非声称已经验证线上漏洞。SQL/算法反例与静态结论各自标注验证边界；不对原型生产安全作推断。

本 Issue 包含同一修复域的 R12, R16, R17；处理状态均先为 **待确认**，请逐项更新 CHANGELOG 为“已采纳（含位置）/未采纳（含理由）”，并保留被证伪结论的纠正记录。

处理清单：

- [ ] R12：两套 SQL/模型字段和 WS 协议仍不一致
- [ ] R16：数据库/同步示例含 API 编译错误，PRAGMA 初始化未覆盖连接池
- [ ] R17：MVP 报表/导出范围仍与权威 PRD 冲突

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

维护者复核说明：复现脚本及完整报告由本轮审阅交付；不直接替换业务方案。对已有修复的补充项已注明具体旧代码残留位置，避免把“新增机制存在”误认为“所有写路径均已闭合”。

官方参考：[xorm API](https://pkg.go.dev/xorm.io/xorm)、[Go 方法接收者规范](https://go.dev/ref/spec#Method_declarations)、[SQLite 外键逐连接启用](https://www.sqlite.org/foreignkeys.html#fk_enable)、[go-sqlite3 DSN](https://github.com/mattn/go-sqlite3#connection-string)。
