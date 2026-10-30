# FastLedger 工程实现核查报告 · D（第二轮，D-R07 起）

> 评审代号：**D**（仍是第一轮的同一审阅者，非新 AI）；意见编号：**D-R07 – D-R09**（不复用 D-R01–R06）
> 评审基线：`67da52a`（仓库 HEAD，2026-10-30）
> 评审对象：**真实可编译 Go 后端** `server/`（WBS 阶段 0/1/2：脚手架、认证/账本/成员、记账核心），`go build`/`go vet` 通过、`go test` 41/41 全绿（本轮已复跑确认）。
> 评审方法：先读 `workbuddy/FastLedger-D二次核查卡片.md`（上轮 6 条状态 + 9 焦点），再据 `workbuddy/多AI对齐规范.md` §2 十三字段、§3 严重度，逐行通读 `server/` 全部 service/api/migrate/database/version 与三个迁移脚本。
> 去重声明：已核对 GPT R01–R23、C-R01–C-R10、D-R01–R06 及 CHANGELOG [1.6]–[1.21]，**本报告不重复已裁决项**；"某功能尚未实现（属阶段 3/4/5）"不作为新 finding，仅在能证明设计错或已实现部分做错时才提。

---

## 0. 总体判断

| 维度 | 结论 |
|---|---|
| 核心会计逻辑（本轮 9 焦点） | **经逐行核对，全部正确，无设计错误、无"代码没照设计做"**。事务边界、幂等四项核对、BETWEEN 边界数学、转账禁单边、15 类符号、错误映射、0003 合法性、无隐藏入口、creator_uid 新建强制 >0，均站得住。 |
| 外围成员/邀请服务 | 发现 **3 个真实健壮性缺口**，集中在 `member.go`：邀请注册非原子（孤儿用户）、邀请"单次使用"条件更新不校验影响行数（并发可绕过）、改角色/移除不校验影响行数（并发假成功）。 |
| 数量与分级 | 3 条：**0×P0、0×P1、2×P2、1×P3**。无会计正确性缺陷、无安全直接绕过。 |
| 性质区分 | 3 条均为**"代码没照设计做 / 健壮性不足"**，不是"设计文档错"——文档（PRD §12.1、WBS、CHANGELOG）的口径本身正确，是实现没完全兑现。 |
| 运行证据边界 | 构建机无 gcc，**本报告全部为静态核查结论**，不能外推为真实 DB/WS/端到端验收（见第 4 节）。 |

> 一句话：**记账核心的"地基"质量高、可以信赖；问题都在"门廊"（成员/邀请）的边角加固上，修完不影响主线，且都不阻塞阶段 3 开工。**

---

## 1. 卡片第 6 节 9 个焦点的核对结论（先给"无问题"覆盖，避免重复劳动）

| # | 焦点 | D 核对结论 | 关键证据位置 |
|---|---|---|---|
| 1 | `tx.go` 事务边界 | ✅ 正确。`Begin` + `defer Rollback`，所有错误路径均 `return`；`Commit` 后再触发 `Rollback` 在 xorm/database-sql 下返回 `sql.ErrTxDone` 且被忽略，无害；不存在"先改余额再返回错误却不回滚"的路径。 | `service/tx.go` Create 主体 |
| 2 | 幂等四项核对 | ✅ 正确。占位 `ON CONFLICT(book_id, uid, client_uuid) DO NOTHING` 列集合与唯一索引完全一致；命中后走重放，对 hash / type / result_ids 数量正负 / version 四项核对，重放只读、**不碰余额、不递增版本**。 | `service/tx.go` 占位与重放段 |
| 3 | `BETWEEN` 边界数学 | ✅ 正确。`delta>0 → high=MaxInt64-delta` 等价于 `balance+delta ≤ MaxInt64`，闭区间不溢出；`delta<0 → low=MinInt64-delta`（`-delta` 为正、量级 ≤1e10，加法本身不溢出）。不误伤合法大额。 | `service/tx.go` createChangeBalance |
| 4 | 转账"禁单边"能否绕过 | ✅ 无法绕过。`validateCreate` 对 `type=3` 强制 `category_id=0`、对手账户存在且不同于本账户；`createTransferInTx` 两条记录 + 两侧余额 + `related_id` 互指全在同一事务，复用外层唯一版本。构造请求无法只落一条。 | `service/tx.go` validateCreate / createTransferInTx |
| 5 | 15 类符号表 | ✅ 与《枚举字典》一致。`IsLiabilityCategory` 为 `10..15`，`signedBalanceDelta` 对负债取反；`qa_extra_test` 已对全 15 类流入/流出断言。 | `model/ledger.go` L64–71；`qa_extra_test.go` L83–121 |
| 6 | 错误映射 400/409 | ✅ 正确。余额溢出/参数非法 → `ErrInvalidCreate`/`ErrInvalidArgument` → 400（重试无意义）；账户已删/版本冲突 → 409。未发现该 400 却映射成 409/500 的分支。 | `api/qa_api_test.go` mapSvcErr 测试 |
| 7 | `0003` 迁移合法性 | ✅ 合法且安全。三条 `ADD COLUMN ... NOT NULL DEFAULT 0`（SQLite 允许带非空默认值的 NOT NULL 加列），索引合理；不触碰、不删除旧回执。 | `migrations/0003_transactions_creator_uid.up.sql` |
| 8 | 隐藏第二入口 | ✅ 无。`router.go` 仅注册契约内路由，无 `/transfer`、`/sync/push` 等提前实现；交易三类型共用唯一 `POST /books/:id/transactions`。 | `api/router.go` L39–78 |
| 9 | `creator_uid=0` 哨兵 | ✅ 当前无已实现错误。新建交易经 `validateCreate` 强制 `uid>0` 并写入 `creator_uid=uid`，**不会产生新的 0**；当前也没有按 creator_uid 放行的可见性逻辑。0 仅来自 0003 历史行，阶段 3/4 必须显式处理（已在 WBS 注明，非新 finding）。 | `service/tx.go` validateCreate/newTxRow |

---

## 2. 逐条新意见（规范 §2 十三字段）

### D-R07 · 邀请注册非原子：先独立提交 user，再写成员，失败留下孤儿用户

- `id`：D-R07
- `title`：JoinViaInvite 跨两个独立事务，第二步失败时已注册用户不回滚，产生无账本孤儿用户
- `severity`：**P2**
- `location`：`server/internal/service/member.go` §JoinViaInvite L197–242；其中 L216 调用注册、L221–240 为第二个事务；注册实现见 `server/internal/service/auth.go` §Register L57–99
- `quote`：
  - member.go L216：`u, err := s.auth.Register(username, password)`
  - member.go L221：`sess := s.engine.NewSession()`（注册已在 L216 独立提交后，才另开新事务）
  - auth.go L96：`if err := sess.Commit(); err != nil {`（Register 在自己的事务内先把 user 落库）
  - member.go L227–231：`if _, err := sess.Insert(&model.BookMember{...}); err != nil { _ = sess.Rollback(); return nil, err }`
- `problem`：设计要求"原子（同事务）建 `user` + `book_members`"（PRD L573、WBS L227、CHANGELOG L89 均明确"原子"）。但实现先调用 `auth.Register`——它在**独立事务**里 Begin/Commit 提交了 user；随后才另开一个事务写 `book_members` 并标记邀请。第二个事务任一语句失败会回滚成员/邀请，却**无法回滚已经提交的 user**。这是"代码没照设计做"，不是设计错。
- `reproduce`：
  1. 静态路径：`grep -n "auth.Register" server/internal/service/member.go` → L216，位于 L221 `NewSession` 之前，且 Register 内部自带 Commit（auth.go L96），两者不在同一事务；
  2. 故障注入设想（无 gcc 暂不能实跑）：令 L227 插入 `book_members` 返回错误（如磁盘/约束故障），函数走 L230 Rollback 后返回；此时查 `users` 表，该用户名已存在，而 `book_members` 无对应行。
- `actual`：第二步失败后，数据库留下一个**不属于任何账本的孤儿用户**；用户用同一邀请码重试时，`Register` 因用户名已占用返回 `ErrUserExists`，入户流程卡死，需换用户名或人工清理。
- `expected`：把"建 user + 写 book_members + 标记邀请"收敛进**同一个事务**：在 JoinViaInvite 的会话内完成用户名查重、bcrypt 哈希、插入 user（不调用会自行提交的 `auth.Register`，或为 Register 增加可传入外部 `*xorm.Session` 的事务内版本），再插成员、更新邀请，一起 Commit / 一起 Rollback。
- `impact`：不损坏财务数据，但破坏入户一致性与可用性：产生孤儿账号、重试即冲突，关闭公开注册后邀请是唯一入户通道，该路径报错会直接阻断新成员加入。
- `suggestion`：新增一个事务内注册助手（如 `registerInTx(sess, username, password)`，复用首用户判定逻辑但不自行 Commit），JoinViaInvite 全程在单一事务内调用；保留对公开注册入口仍走原 `auth.Register`。
- `status`：待确认
- `evidence`：（待复核 AI 填）D 自查：L216 与 L221 的事务边界、auth.go L96 的独立提交可由上述 grep 直接复现；历史评审未提过邀请注册孤儿用户（GPT 相关条目针对的是交易孤儿行，非 user 注册），无重复。

---

### D-R08 · 邀请码"单次使用"条件更新不校验影响行数，并发下可被同一码重复入户

- `id`：D-R08
- `title`：标记邀请已用的 `UPDATE ... WHERE used_at=0` 不检查 RowsAffected，并发竞争下一个邀请码可被两个用户使用
- `severity`：**P2**
- `location`：`server/internal/service/member.go` §JoinViaInvite L233–237；表结构见 `server/internal/migrate/migrations/0002_invites_and_member_unique.up.sql` L11–25
- `quote`：
  - member.go L233–234：`if _, err := sess.Where("id = ? AND used_at = 0", inv.Id).` / `Cols("used_at", "used_by").Update(&model.Invite{UsedAt: now, UsedBy: u.Id}); err != nil {`
  - 0002 L18–19：`used_at INTEGER DEFAULT 0,` / `used_by INTEGER DEFAULT 0,`（邀请表无任何强制"只用一次"的数据库唯一约束）
- `problem`：邀请的"单次使用"在数据库层没有约束兜底（0002 中 `used_at` 是普通列），完全依赖应用层的条件更新。L206 先读 `used_at>0` 做判断、L233 再用 `WHERE used_at=0` 更新，这是典型 check-then-act；而 L233–237 只判断 `err != nil`，**不读取并核对 RowsAffected**。当邀请已被并发请求抢先标记时，这条 UPDATE 影响 0 行但 `err` 仍为 nil，代码继续 Commit。属"代码没照设计做"的并发健壮性缺陷。
- `reproduce`：
  1. 静态路径：`grep -n "RowsAffected" server/internal/service/member.go` → 在 JoinViaInvite 段无命中（对比 account.go L193/L242 同类写操作都有校验）；
  2. 并发设想（无 gcc 暂不能实跑）：取一个**未绑定用户名**的邀请码，两个不同用户名请求几乎同时进入；二者 L206 都读到 `used_at=0`，各自 Register 成功（不同用户名不冲突），随后两个事务串行提交，第二个事务的条件更新影响 0 行却被忽略，最终两条 `book_members` 都落库（部分唯一索引只约束同 `(book_id,uid)`，不同 uid 不互斥）。
- `actual`：在精确并发窗口内，一个本应单次使用的邀请码可让两个不同用户都加入同一账本，"单次使用"约束被绕过。
- `expected`：L233 的条件更新后读取 RowsAffected，**影响行数必须为 1**；为 0 说明邀请已被并发使用，应 Rollback 并返回 `ErrInviteInvalid`（或并发冲突）。可进一步在应用层对 `invites.id` 的写竞争加保护。
- `impact`：触发条件较窄——需邀请码未绑定用户名、被转发/泄露、且两请求在时间窗口内并发；家庭场景概率低。但一旦邀请码泄露，会放大为未授权入户（访问控制），故定 P2 而非 P3；绑定了用户名的邀请因 L209 强制匹配，不受此影响。
- `suggestion`：与 D-R07 同处修复时一并处理：条件更新后 `n, _ := res.RowsAffected(); if n != 1 { rollback; return ErrInviteInvalid }`，把"单次使用"从软约定改为硬校验。
- `status`：待确认
- `evidence`：（待复核 AI 填）D 自查：0002 表无单次约束、L233 未校验影响行数可 grep 复现；与 D-R07 根因相邻但性质不同（本条为并发安全，D-R07 为失败原子性），故独立编号。

---

### D-R09 · 改角色/移除成员不校验影响行数，并发下"目标已不存在"却返回成功

- `id`：D-R09
- `title`：ChangeRole/Remove 的 UPDATE 不核对 RowsAffected，目标在并发中已被移除时返回 nil 被当成功
- `severity`：**P3**
- `location`：`server/internal/service/member.go` §ChangeRole L132–135、§Remove L155–158
- `quote`：
  - L132–135：`_, err = s.engine.Where("book_id = ? AND uid = ? AND deleted = 0", bookID, targetUid).` / `Cols("role", "updated_at").` / `Update(&model.BookMember{Role: newRole, UpdatedAt: time.Now().Unix()})` / `return err`
  - L155–158：`_, err = s.engine.Where("book_id = ? AND uid = ? AND deleted = 0", bookID, targetUid).` / `Cols("deleted", "deleted_at", "updated_at").` / `Update(&model.BookMember{Deleted: true, DeletedAt: now, UpdatedAt: now})` / `return err`
- `problem`：两个方法在写之前都用 `RoleOf` 读了目标状态，但读与写之间目标可能已被另一个请求移除（软删 `deleted=1`）。此时带 `deleted=0` 条件的 UPDATE 影响 **0 行**，但 SQL 不报错，`err==nil`，函数直接 `return err` 被上层当作成功。属健壮性缺口（代码没照项目自身既有约定做）。
- `reproduce`：
  1. 静态路径：L132、L155 均用 `_` 丢弃了返回的受影响计数，函数体无 `RowsAffected` 判定；
  2. 对比同仓 `account.go` L187–195（Update 后 `if n != 1 { return ErrConcurrentConflict }`）与 L238–244（Delete 后 `if n != 1 { return ErrAccountNotFound }`），说明项目既定规范就是核对 1 行，成员服务漏了。
- `actual`：并发下对一个已被移除的成员执行改角色/移除，接口返回成功响应，但实际没有任何数据变更，调用方拿到与事实不符的结果。
- `expected`：取 Update 返回的计数，`n != 1` 时返回明确错误（目标已不在 → `ErrNotMember`/`ErrConcurrentConflict`），由 mapSvcErr 映射为 404/409，而非静默成功。
- `impact`：不造成数据损坏或越权（写条件本身带 `deleted=0`，不会误改），仅在并发管理成员时产生"假成功"，影响管理操作的可预期性，故 P3。
- `suggestion`：两处统一改为接收计数并校验 `n==1`，与 account.go 对齐；可在后续成员管理测试中补一个"目标已删"的用例。
- `status`：待确认
- `evidence`：（待复核 AI 填）D 自查：L132/L155 丢弃计数、与 account.go 不一致可 grep 复现；历史评审未提过成员角色变更的 RowsAffected，无重复。

---

## 3. 三类问题的性质归类（按 owner 要求区分）

| 编号 | 性质 | 说明 |
|---|---|---|
| D-R07 | 代码没照设计做 | 文档明确"原子同事务"，实现拆成两个独立事务 |
| D-R08 | 代码没照设计做（并发健壮性） | "单次使用"无库约束、应用层条件更新又不校验结果 |
| D-R09 | 代码没照设计做（健壮性） | 项目自身 account.go 已立"核对 1 行"规范，member.go 未对齐 |

> 未发现"设计文档本身有错"的情形。PRD §12/§12.1、WBS、枚举字典的口径无需改动。

---

## 4. 验证边界（防止把静态结论当运行验收）

- 本轮已复跑确认：`go build ./...`、`go vet ./...` 通过，`go test ./...` 全绿（纯函数/逻辑测试，无 DB 依赖）。
- 构建机**无 gcc → CGO 不可用 → go-sqlite3 只能编译不能运行**。因此以下仍**无真实运行时证据**，本报告不背书：
  - 事务真实回滚（含 D-R07 修复后的原子性）、UPSERT `ON CONFLICT` 的 RowsAffected 真实语义；
  - D-R08/D-R09 所依赖的并发竞争与 `busy_timeout` 串行化、`go test -race`；
  - `related_id` 双向落库、0002/0003 在真实库的迁移可执行性；
  - JWT→handler→service 全链路、PRD §10.1/§10.3/§10.4 端到端。
- 建议在具备 gcc 的环境（或 Docker 多阶段构建）补一组**真实 SQLite 集成测试**，优先覆盖：事务回滚、幂等重放、并发邀请/成员操作、账户余额 RowsAffected==1。

---

## 5. 对"能否继续推进落地"的判断

- **可以继续推进，且不建议被这 3 条阻塞阶段 3 开工。** 记账核心（资金/账目/余额/转账/幂等）这一最易错、最该先稳的部分经核查无会计正确性缺陷；新发现的 3 条都在成员/邀请的边角，风险可控、修复量小。
- 建议处置顺序：
  1. **D-R07、D-R08 合并修复**（同在 JoinViaInvite，收敛为单一事务 + 两处 RowsAffected 硬校验），可在阶段 3 启动前顺手完成，避免把入户缺陷带到后面；
  2. **D-R09 随阶段 3 成员管理一起改**，并补真实 SQLite 集成测试；
  3. 阶段 3（同步/WS/可见性）开工时，**务必把 `creator_uid=0` 历史行的显式处理写进可见性/仅本人过滤逻辑**，杜绝其被当作放行条件（这是卡片焦点 9 的前置提醒，非新 finding）。
- 仍需 owner 知晓的最大盲区不是"有没有 bug"，而是**整体缺真实运行时证据**：41 个单测全绿 ≠ 数据库/WS/端到端通过。落地排期里应尽早安排一次带 gcc 的集成/端到端验收，再谈可部署。
