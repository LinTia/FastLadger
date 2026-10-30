## 2026-10-30 · v1.9 阶段 0–3 第二轮补充复核

重新抽查 v1.8 的 GPT-R35–GPT-R52，并新增 GPT-R53（P2）：账户父节点仍有活跃子账户时，AccountService.Delete 未检查子账户，允许删除父节点并留下悬空 parent_id。该问题不同于 R47 的父级环路与 R31 的交易恢复依赖。未发现足够证据新增其他 finding；既有风险仍待处理。静态审查，未运行 Go/SQLite/race/WebSocket/断电测试，未修改业务代码。报告、结构化 finding 和验证边界见 [v1.9](v1.9/审核结果.md)、[findings](v1.9/findings.json)、[runtime-results](v1.9/verification/runtime-results.json)。根 CHANGELOG 新增 [1.41]。

## 2026-10-30 · v1.8 阶段 0–3 回溯式代码复核

重新检查当前阶段 0–3 代码、PRD/WBS 契约与历史 Finding，新增 GPT-R35–GPT-R52 共 18 项（1 P1、15 P2、2 P3）。重点涉及 SQLite 文件权限/外键/迁移校验、恢复世代持久性、邀请撤销/邮箱绑定/审计、删除账本后的成员管理、限流器内存增长、认证输入边界、账户分组环、Pull 多实体分页与 WebSocket 订阅/鉴权生命周期。GPT-R52 为 PRD 会话撤销要求与 WBS 阶段分配不一致，等待 owner 裁决。

另扩展 GPT-R27（邀请 TTL 秒数乘法溢出）与 GPT-R34（环境变量非法值静默回退默认值）的适用范围。已复核 D-R07–D-R09、WB-R01、GPT-R24–R25 的历史修复，不重复登记；已知 [1.28] WebSocket 软删订阅遗留也不重复计数。

完整 13 字段报告、结构化 findings 与 not_run 验证计划见 [v1.8](v1.8/审核结果.md)、[findings.json](v1.8/findings.json)、[runtime-results.json](v1.8/verification/runtime-results.json)。本轮只提交审查文档，未修改业务代码；未执行本地 Go/SQLite/race/WebSocket 测试。

## 2026-10-30 · v1.7 阶段 4扩展复核

用户尚未安排 Workbuddy 修复；GPT 在原 GPT-R26–GPT-R30 基础上继续扩查，新增 GPT-R31–GPT-R34：P1 账户软删后交易无法恢复；P2 Restore 未清空 deleted_at；P2 账本名称+可见性组合更新非原子；P2 db purge 非法参数静默回退默认 TTL。另扩展 GPT-R28 授权 TOCTOU 涉及 BookService.Update/Delete 与 MemberService.CreateInvite。完整 13 字段与范围排除说明见 [v1.7 审核报告](v1.7/审核结果.md)，结构化数据见 [findings](v1.7/findings.json)。根 CHANGELOG 新增 [1.39]。

# GPT 审查意见处理记录

## 2026-10-30 · v1.6 最新改动审查

新增 GPT-R25（P2）：SQLite 驱动冒烟命令的 `internal/migrate` 与 `internal/database` 目标包均无测试文件，Actions 日志显示 `[no test files]`。owner 已采纳；尚未修复 CI/测试代码。完整 13 字段报告、结构化 finding 与日志摘要见 [v1.6](v1.6/审核结果.md)。根 CHANGELOG [1.32] 已同步，并更正 [1.31] 对专项 SQLite 验证范围的过度表述。

## 2026-10-29 · v1.4 定向复核 / 文档 v1.5

新增 GPT-R19–GPT-R23，回填 GPT-R03/GPT-R06/GPT-R16 原编号证据。唯一创建入口裁决和修订参考骨架已同步 docs/；完整处理状态与范围以根 [CHANGELOG](../CHANGELOG.md) 最新节和 [逐条意见](v1.4/findings.json) 为准。

真实验证工具及原/新结果位于 [v1.4/verification](v1.4/verification/README.md)。以下首轮记录为历史快照，不用旧“待确认”表覆盖最新处理结论。

## 2026-10-29 · 首次交付

审查基线：`b8741465afa98a89ed99796c9e2c4a841b15a361`。

本目录仅交付审查结果，不修改应用需求或业务实现。新增完整报告、结构化发现、反例脚本、16 个运行结果及六份 Issue 草稿。

“待确认”是维护者的处理决策状态，不表示报告中的证据均未验证。每项验证方法与限制见完整报告。
实施修复后，请在此表补充位置，并同步项目根 CHANGELOG；不采纳应说明理由，被证伪应保留误判更正记录。

| 编号 | 优先级 | 处理状态 | 问题 | 改动位置/理由 |
|---|---|---|---|---|
| R01 | P1 | 待确认 | 账户引用未绑定账本，且余额更新零行仍返回成功 | — |
| R02 | P1 | 待确认 | Transfer 忽略数据库错误，交易失败后仍可能提交余额 | — |
| R03 | P1 | 待确认 | 信用卡负债使用资产增减规则，导致消费增加净资产 | — |
| R04 | P1 | 待确认 | 幂等预检查存在竞态且未限定用户/账本 | — |
| R05 | P1 | 待确认 | WebSocket 订阅信任客户端账本 ID，广播无法执行成员可见性 | — |
| R06 | P1 | 待确认 | 全局版本机制未覆盖所有写实体，且部分构造遗漏 book_id | — |
| R07 | P1 | 待确认 | 以单条 WS 事件推进全局游标会永久漏同步；pull 缺一致快照约束 | — |
| R08 | P1 | 待确认 | JWT 签发仍使用硬编码密钥，未对接配置中的密钥 | — |
| R09 | P1 | 待确认 | JWT 认证路径未执行令牌撤销与账号失效检查 | — |
| R10 | P1 | 待确认 | 恢复旧备份回退版本计数，已有客户端游标无法发现新数据 | — |
| R11 | P1 | 待确认 | 交易服务未拒绝非法金额/类型，持仓解析错误被丢弃 | — |
| R12 | P2 | 待确认 | 两套 SQL/模型字段和 WS 协议仍不一致 | — |
| R13 | P2 | 待确认 | 预算 SQL 漏算小类；总预算仍带 category_id 条件 | — |
| R14 | P2 | 待确认 | CSV 文本字段未防公式解释 | — |
| R15 | P2 | 待确认 | OCR 金额正则把千位分隔符金额截断为 1 元 | — |
| R16 | P2 | 待确认 | 数据库/同步示例含 API 编译错误，PRAGMA 初始化未覆盖连接池 | — |
| R17 | P2 | 待确认 | MVP 报表/导出范围仍与权威 PRD 冲突 | — |
| R18 | P2 | 待确认 | WS 订阅 map 并发读写，断连清理路径不完整 | — |
