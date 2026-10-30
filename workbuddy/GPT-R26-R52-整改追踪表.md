# GPT-R26 – GPT-R52 整改追踪表

> 审查依据：GPT 独立审查 v1.7 + v1.8
> 整改基线：`main` @ `be5439d`（阶段 0–4 已完成，CI 170 PASS / 0 FAIL）
> 建立时间：2026-10-30

## ⚠️ 前置阻塞项（必须先解决）

**`gpt/v1.7/` 与 `gpt/v1.8/` 的原始审查产物尚未入库。** 仓库现有审查材料只到 `gpt/v1.6/`（最新 Finding 为 `GPT-R25`），而本轮引用的是 `GPT-R26`–`GPT-R52`。缺失文件包括：

- `gpt/v1.7/审核结果.md`、`gpt/v1.7/findings.json`、`gpt/v1.7/verification/runtime-results.json`
- `gpt/v1.8/审核结果.md`、`gpt/v1.8/findings.json`、`gpt/v1.8/verification/runtime-results.json`

任务单明确要求"不得只阅读本任务单而跳过原始 Finding"。在原始产物入库前，本表的
**状态判定依据为：任务单描述 + 以当前代码为准的重新定位**（任务单亦要求"不得假定
审查报告中的代码行号仍然有效，应以当前代码为准重新定位"）。

**待办**：请 Owner 将上述文件提交到仓库，随后补齐每条 Finding 的 severity、证据行号、
原始描述，并把本表合并进 `findings.json`（保留原 ID，不重新编号）。

---

## 二、P1 阻断项

| ID | 问题 | 当前代码复核 | 状态 | 修复文件 | 回归测试 |
|---|---|---|---|---|---|
| **GPT-R26** | 转账必须作为完整操作处理；对侧缺失/错链/金额不匹配/账户不匹配必须拒绝整笔，不得退化为单边 | **确认存在**：`collectTransferGroup` 在对侧缺失时返回单侧并注释"避免悬空配对"，正是被禁止的退化 | 待修 | `service/tx.go` | 待补 |
| **GPT-R27** | 所有 TTL 时间转换必须防整数溢出；非法值必须显式报错 | **确认存在**：`recycle.go` 两处 `now - int64(days)*secondsPerDay` 无溢出检查，大值可溢出致 cutoff 变未来时间而误删全表；邀请 TTL 待核查 | 待修 | `service/recycle.go`、`service/member.go` | 待补 |
| **GPT-R28** | 敏感写操作必须在事务内重新验证权限（防 TOCTOU） | **确认存在**：`mutateVisibility` 在开事务**前**调 `RoleOf`；`ChangeRole`/`Remove`/`SetVisibility` 同样在事务外取角色 | 待修 | `service/tx.go`、`member.go`、`book.go` | 待补（barrier 确定性并发） |
| **GPT-R31** | 回收站保留期内的交易，其依赖的账户/分类不得被删除 | **确认存在**：`AccountService.Delete` 的 `used>0` 检查带 `deleted = 0`，未计入软删交易；分类侧待核查 | 待修 | `service/account.go`、`category.go` | 待补 |

## 三、P2/P3 问题

| ID | 问题 | 状态 | 备注 |
|---|---|---|---|
| GPT-R29 | `before_json` 须在事务内读取，防并发读到错旧值 | 待核查 | 与 R28 同批整改 |
| GPT-R30 | 交易更新审计；未实现则记录为未交付范围 | 待核查 | 当前无 PUT 路由（路由守卫已禁），倾向记为"未交付范围" |
| GPT-R33 | 账本更新（名称+可见性）必须原子化，审计同事务，广播仅在提交后 | 待核查 | `BookService.Update` |
| GPT-R34 | CLI/环境变量非法值不得静默回退；删数据前完成全部校验 | 待核查 | `cmd/server/main.go` 的 `envIntOr`/`parsePositiveInt` |
| GPT-R35 | SQLite 文件（DB/WAL/SHM）权限策略，不依赖 umask | 待核查 | `database.Init` |
| GPT-R36 | 外键约束：评估真实 FK 或显式记录不使用原因 | 待核查 | 需 Owner 裁决（涉及历史数据） |
| GPT-R37 | `migrate.Check` 必须只读，空库须明确失败而非建表 | 待核查 | `migrate.go` 的 `ensureVersionTable` |
| GPT-R38 | 迁移版本完整性：拒绝重复版本号、检测版本缺口与未知版本 | 待核查 | 不得只依赖 `MAX(version)` |
| GPT-R39 | `instance.meta` 持久性：临时文件 Sync + 父目录 Sync | 待核查 | `instance` 包 |
| GPT-R40 | 邀请码撤销能力，事务内校验权限，条件更新 | 待核查 | 新增 API |
| GPT-R41 | 邮箱绑定邀请 `bound_email` 完整实现，或 Owner 裁决不支持 | **待 Owner 裁决** | 涉及 PRD 范围 |
| GPT-R42 | 邀请生命周期审计（创建/兑换/撤销/过期/失败） | 待核查 | 依赖 R40 |
| GPT-R43 | 已删除账本不得继续成员管理/邀请 | 待核查 | `MemberService` 各入口 |
| GPT-R44 | 限流器内存有界，过期 key 清理，最大 key 数 | 待核查 | `limiter.go` |
| GPT-R45 | 注册输入边界：用户名长度口径、bcrypt 72 字节上限 | 待核查 | `auth.go` |
| GPT-R46 | 登录错误映射：凭证错 401 vs 内部错 500/503 | 待核查 | 与 [1.37] 登录审计改动相关 |
| GPT-R47 | 账户分组无环（parent_id 不得指向自身后代） | 待核查 | `account.go` |
| GPT-R48 | Pull 的 limit 须约束所有同步实体，统一版本流 | 待核查 | `sync.go` |
| GPT-R49 | WebSocket 订阅数量上限、去重、频率限制 | 待核查 | `ws` 包 |
| GPT-R50 | WebSocket 撤权竞态：generation 机制 | 待核查 | `ws` 包 |
| GPT-R51 | WebSocket 生命周期受 JWT 有效期约束 | 待核查 | `ws` 包 |
| GPT-R52 | 多端会话查看与撤销 | **待 Owner 裁决** | 需先提交方案与工作量评估 |

## 四、完成判定（任务单 §5.3）

只有同时满足以下五条才可标记 `resolved`：
1. 代码修复已提交；
2. 对应回归测试已提交；
3. 测试**实际运行并通过**（附 Actions run 链接与 PASS 数）；
4. 相关代码路径已复核；
5. 未引入新的数据一致性或权限问题。

仅"代码已改""测试编译通过""CI 其他步骤绿"均不足以标记 resolved。
`GPT-R52` 等裁决项保持裁决状态，不得擅自关闭。
