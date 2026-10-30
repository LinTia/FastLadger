# GPT-R26 – GPT-R52 整改追踪表

> 审查依据：GPT 独立审查 v1.7 + v1.8
> 整改基线：`main` @ `be5439d`（阶段 0–4 已完成，CI 170 PASS / 0 FAIL）
> 建立时间：2026-10-30

## 审查产物（已入库，2026-10-30 拉取）

- `gpt/v1.7/` —— GPT-R26 – R34（9 项；P1：R26/R27/R28/R31）
- `gpt/v1.8/` —— GPT-R35 – R52（18 项；P1：R40）
- `gpt/v1.9/` —— GPT-R53（1 项新增）；基线 `be5439d`，并复核 R27/R31/R34/R35/R47/R52

**合计 28 项 Finding（R26–R53），其中 P1 五项：R26、R27、R28、R31、R40。**
v1.9 的 review_notes 明确：R27 保留"invite TTL 溢出"范围补充；R34 保留"`config.envInt`
静默回退"范围补充；R47 与 R53、R31 与 R53 分别是不同问题。

> 说明：本表初建时 v1.7/v1.8 尚未入库，曾据此登记为阻塞项；现已解除。

---

## 二、P1 阻断项

| ID | 问题 | 当前代码复核 | 状态 | 修复文件 | 回归测试 |
|---|---|---|---|---|---|
| **GPT-R26** | 转账必须作为完整操作处理；对侧缺失/错链/金额不匹配/账户不匹配必须拒绝整笔，不得退化为单边 | **确认存在**：`collectTransferGroup` 在对侧缺失时返回单侧并注释"避免悬空配对"，正是被禁止的退化 | 待修 | `service/tx.go` | 待补 |
| **GPT-R27** | 所有 TTL 时间转换必须防整数溢出；非法值必须显式报错 | **已修复**（`ba80b0d`）| 新增 `service/ttl.go` 作唯一换算入口：`daysToSeconds`（`days≤maxRetentionDays` 上限即溢出防护）、`retentionCutoff`（拒绝早于纪元）、`expiryAfter`（前向 TTL）。邀请 TTL 改为构造时校验（`NewMemberService` 返回 error）。消掉 `daySeconds`/`secondsPerDay` 重复定义 | `service/ttl.go`、`recycle.go`、`member.go`、`cmd/server/main.go` | `ttl_test.go` 9 个用例，**本地实跑全过** |
| **GPT-R28** | 敏感写操作必须在事务内重新验证权限（防 TOCTOU） | **确认存在**：`mutateVisibility` 在开事务**前**调 `RoleOf`；`ChangeRole`/`Remove`/`SetVisibility` 同样在事务外取角色 | 待修 | `service/tx.go`、`member.go`、`book.go` | 待补（barrier 确定性并发） |
| **GPT-R31** | 回收站保留期内的交易，其依赖的账户/分类不得被删除 | **确认存在**：`AccountService.Delete` 的 `used>0` 检查带 `deleted = 0`，未计入软删交易；分类侧待核查 | 待修 | `service/account.go`、`category.go` | 待补 |

## 三、P2/P3 问题

| ID | 问题 | 状态 | 备注 |
|---|---|---|---|
| GPT-R29 | `before_json` 须在事务内读取，防并发读到错旧值 | 待核查 | 与 R28 同批整改 |
| GPT-R30 | 交易更新审计；未实现则记录为未交付范围 | 待核查 | 当前无 PUT 路由（路由守卫已禁），倾向记为"未交付范围" |
| GPT-R33 | 账本更新（名称+可见性）必须原子化，审计同事务，广播仅在提交后 | 待核查 | `BookService.Update` |
| GPT-R34 | CLI/环境变量非法值不得静默回退；删数据前完成全部校验 | **部分修复**：config 层已修（`ba80b0d`，`envInt` 非法即报错 + 保留期上限校验，`config_test.go` 7 用例本地全过）；**CLI 层整改中** | CLI 层由工程师处理：区分"未提供"与"提供但非法"，缺值/格式错/非正/越界/未知参数一律 fail-fast，且**任何 DELETE 前完成全部校验**；须断言非法输入下数据库行数不变 |
| GPT-R35 | SQLite 文件（DB/WAL/SHM）权限策略，不依赖 umask | **整改中** | 目录 0700、DB 与 sidecar 0600；新建与已存在实例都要收紧。**验证边界**：Windows 上 `Chmod` 语义不同，测试须在 `GOOS=windows` 显式跳过并说明，Linux（CI）上真实断言——不得为变绿而删除测试 |
| GPT-R36 | 外键约束：评估真实 FK 或显式记录不使用原因 | **Owner 裁决：不使用外键**（2026-10-30） | 理由与补偿控制已写入 `docs/FastLedger-落地方案.md` §0.1。核心：软删墓碑与 FK/级联直接冲突；替代为"事务 + 显式引用检查"（R26/R28/R31/R47/R53）。**验证须实际写入非法引用并断言被拒绝，不得用 `PRAGMA foreign_keys=1` 当证据** |
| GPT-R37 | `migrate.Check` 必须只读，空库须明确失败而非建表 | **整改中** | `ensureVersionTable` 只保留在 `Apply`；`Check` 只查 `sqlite_master`。测试断言空库调 `Check` 后不产生 `schema_migrations` |
| GPT-R38 | 迁移版本完整性：拒绝重复版本号、检测版本缺口与未知版本 | **整改中** | 不得只依赖 `MAX(version)`；须覆盖重复版本、版本缺口、未知版本、乱序文件名四类测试 |
| GPT-R39 | `instance.meta` 持久性：临时文件 Sync + 父目录 Sync | **整改中** | `Bump` 返回前须持久化文件内容与父目录 rename 项。**验证边界**：普通单元测试无法完全模拟断电，须如实记录 |
| GPT-R40 | 邀请码撤销能力，事务内校验权限，条件更新 | 待核查 | 新增 API |
| GPT-R41 | 邮箱绑定邀请 `bound_email` 完整实现 | **Owner 裁决：完整实现**（2026-10-30） | 需形成"创建→规范化→落库→兑换校验"闭环：支持 `boundEmail` 入参、兑换时校验邮箱匹配、统一规范化规则（大小写/空格）、审计（配合 R42）。**因与工程师并行的 member.go 改动冲突，排在服务核心批次完成后施工** |
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
| GPT-R53 | 删除仍有活跃子账户的父账户会留下悬空 `parent_id`（v1.9 新增，任务单未列） | **整改中**（与服务核心同批） | v1.9 明确：R53 与 R47（环路）、R31（交易引用账户）是**三个不同问题**，不得合并处理 |
| GPT-R52 | 多端会话查看与撤销 | **Owner 裁决：排期实现**（2026-10-30） | 已排入 **WBS 5.10**（客户端阶段，多端会话需真实客户端才有意义），并在 **PRD 新增 §10.10** 验收项（撤销后该设备 HTTP 401 + WS 主动断开 + 握手拒绝）。要点：`token_sessions` 持久化 `jti`、`Verify` 校验撤销态、WS 同样识别、过期会话清理。**不再作为"未安排项"** |

## 四、完成判定（任务单 §5.3）

只有同时满足以下五条才可标记 `resolved`：
1. 代码修复已提交；
2. 对应回归测试已提交；
3. 测试**实际运行并通过**（附 Actions run 链接与 PASS 数）；
4. 相关代码路径已复核；
5. 未引入新的数据一致性或权限问题。

仅"代码已改""测试编译通过""CI 其他步骤绿"均不足以标记 resolved。
`GPT-R52` 等裁决项保持裁决状态，不得擅自关闭。
