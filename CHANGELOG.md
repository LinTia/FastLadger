# 更新日志（CHANGELOG）

本项目所有重要变更记录于此。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [1.13] - 2026-10-30

### 更新《多 AI 对齐规范》（为下一个独立评审 AI 对齐颗粒度）
- **§1 角色表**：C 由"待加入"更新为"已产出 `C-R01`–`C-R10`"（10 项，6×P2 + 4×P3）；新增 **D（第四个 AI）** 代号，编号用 `D-Rxx`，避免新 AI 与既有编号撞车。
- **§1 编号规则**：新增"已裁决项勿重复提"警示，要求提新意见前先查 CHANGELOG [1.6]–[1.12]。
- **§9 `workbuddy/` 清单**：补入 `FastLedger-文档评审-C.md`（C 的 10 项，标注"新 AI 必读，避免重复提"）、v1.3 / v1.4 / v1.5 说明；并注明 `docs/FastLedger-开发WBS.md` **不在本目录**。
- **§10 上手清单**：扩为 9 步，明确必读 CHANGELOG [1.6]–[1.12]、C 评审（勿重复提）、开发 WBS；新 AI 编号改用 `D-Rxx`。
- **§11 进度快照**：补齐缺失的 [1.6]–[1.12] 共 7 个版本；新增"【1.12】之后的当前口径"，指引新 AI 把火力转向**工程实现层新风险**（Gin 接线与 400/409、真实 WS 生命周期、Flutter 源码、OCR 后端契约、多实体端到端同步、迁移与删除 / 恢复验收）。
- **目的**：owner 拟引入新的独立评审 AI；本次更新确保其在颗粒度（§2 十三字段）、编号、严重度（§3）与证据标准（§4）上对齐，且不被过期信息误导而重复提已裁决项。

## [1.12] - 2026-10-30

### owner 裁决：§12.1 剩余 3 项待确认全部转为【决策】
- **权限变化与本地缓存重建**（已采纳建议方案）：事件收敛为 4 类（`visibility_changed` / `member_removed` / `member_role_changed` / `book_deleted`）且 WS 广播**脱敏**；客户端**定向重拉该账本**（`afterVersion=0`）而非清空本地库；可见性变化**不递增业务 `sync_version`**（非数据变更），改用事件携带 `force_full_resync=true`；被移除成员客户端**主动清除该账本本地数据**并停止订阅；隐私边界话术写入 §7.3 并常驻显示于开关旁。改动 `docs/FastLedger-产品需求文档-PRD.md` §12.1、§7.3。
- **实例管理员与账本 owner 边界**（已采纳）：两层角色**正交**（实例管理员 vs 账本 owner/admin/member），互不推导；整库备份含所有私账，故**下载仅限实例管理员**（文件权限 600 + 写审计），对普通 owner 提供**"按账本导出"**以机制上防越权；LLM 密钥修改仅实例管理员并写 `audit_logs(action=config_change)`；`users` 增 `is_instance_admin`，与 `book_members.role` 分离。改动 PRD §12.1。
- **关闭公开注册后的成员入户**（已采纳）：**邀请与公开注册解耦**，`EBK_ENABLE_REGISTER=false` 不影响邀请码注册，邀请为唯一入户通道；邀请码绑定账本 + 角色、**有效期默认 7 天**（`EBK_INVITE_TTL_DAYS`）、**单次使用**（`used_at`/`used_by`）、可选绑定邮箱或用户名（防链接转发）；限流：每账本每分钟生成 ≤5、同 IP 每小时尝试 ≤10，失败计审计；注册**原子**建 `user` + `book_members`；生成/使用/撤销/过期全周期审计。改动 PRD §12.1。
- **同步《开发WBS》**：附录 A 提案 1/2/3 由"待裁决"更新为"已采纳为【决策】"；阶段 1、3 的阻塞标记解除。
- **至此 §12.1 五项技术待确认全部转为【决策】**，PRD 已无遗留【待确认】阻塞项，可进入开发阶段（见《FastLedger-开发WBS.md》）。

## [1.11] - 2026-10-30

### 新增开发 WBS（展开版）+ 3 项待确认的建议方案
- **新增 `docs/FastLedger-开发WBS.md`**：《落地方案》§6.3 粗粒度 WBS（#1–#18）的展开版，含 8 个阶段（0 脚手架 → 7 运维验收）、任务 / 交付物 / 依赖 / 验收映射，以及 PRD §10 验收映射表。
- **本轮决策产生的新增工作已入 WBS**：阶段 4「删除 / 恢复 / 审计」；阶段 0.2 `synchronous=FULL`；阶段 7.1 `stop_grace_period: 30s`；阶段 7.3 恢复演练。
- **§12.1 剩余 3 项待确认各附建议方案**（WBS 附录 A 提案 1/2/3，**待 owner 裁决**）：权限变化与缓存重建；实例管理员与账本 owner 边界；关闭注册后的邀请入户。PRD §12.1 对应条目已加指针。
- 未改动任何既有【决策】；该 3 项仍为【待确认】，裁决后再转为【决策】并同步回 WBS 与《落地方案》。

## [1.10] - 2026-10-30

### owner 裁决：删除恢复审计转【决策】+ 断电持久性运维加固
- **§12.1「删除恢复与审计」由【待确认】转【决策】**：采纳 C 的设计建议（owner 确认：回收站 30 天、审计保留 1 年）。改动 `docs/FastLedger-产品需求文档-PRD.md` §12.1，含回收站与保留期、转账双边整笔删除/恢复、审计日志表 append-only、旧离线数据回流、恢复演练验收、统一 `TxService.Delete/Restore` 入口。
- **§8.5 部署与运维新增 4 条加固**：备份原子包含 WAL、优雅关机 checkpoint（`stop_grace_period`）、FULL 不替代备份且须恢复演练、恢复后递增 `restore_epoch`。
- **§10 新增 10.9 备份恢复与审计验收**（恢复演练、转账双边删除/恢复、审计可追溯）。
- **同步技术方案（遵循"设计改→方案同步"纪律）**：`docs/FastLedger-落地方案.md` §8.1 docker-compose 增 `stop_grace_period: 30s`；§8.3 备份与恢复增"恢复演练（必做）"与 WAL 注意事项。

## [1.9] - 2026-10-29

### owner 裁决：采纳 C-R03 / C-R05 / C-R06
- **C-R03 / P2 / 已采纳**：admin 改/删他人交易以 PRD §12 决策#2 为准。改动 `docs/FastLedger-落地方案.md` §5.5 权限矩阵：admin 列由"可配置"改为"✅"，并加注引用 §12 决策#2；member 列"默认不可"维持（与 PRD §7.2 member=✗ 一致）。
- **C-R05 / P2 / 已采纳**：全仓统一 `EBK_` 环境变量前缀。改动 `docs/FastLedger-新手开发完整指南.md` §22.5：`ENABLE_REGISTER=false` → `EBK_ENABLE_REGISTER=false`（与落地方案 §6.1、PRD §12.1 一致）。
- **C-R06 / P2 / 已采纳**：断电持久性定为 `synchronous=FULL`，不采用"书面豁免"。改动：指南 §22.4 initDB DSN `_synchronous=NORMAL`→`FULL`；落地方案 §6.1 补 `synchronous=FULL` 说明；PRD §12.1 该项由【待确认】转为【决策】。
- **§12.1 其余技术待确认**（权限变化缓存重建、实例管理员边界、关闭注册后入户、删除恢复审计）：维持【待确认】，待 owner 后续裁决。

## [1.8] - 2026-10-29

### 新增独立审阅者 C 文档评审（workbuddy/FastLedger-文档评审-C.md）
- 按《多 AI 对齐规范》§1 以"第三个 AI（C）"身份提交独立评审，10 项发现 C-R01~C-R10（6×P2 + 4×P3，均待确认）。
- 覆盖：PRD/落地方案/新手指南/枚举字典/转账裁决/README/CHANGELOG 的跨文档一致性、版本号双轨冲突、权限矩阵矛盾、环境变量前缀不一致、`synchronous=NORMAL` 断电持久性待确认未闭合等。
- 未采纳/未裁决：10 项均 `待确认`，等待 owner 裁定；未重提既有 GPT R01–R23 与 WB 各项。
- 报告全文见 `workbuddy/FastLedger-文档评审-C.md`。

## [1.7] - 2026-10-29

### README 补公网交互原型入口（WorkBuddy）
- 仓库导航表 `prototype/` 行用途列补充公网在线版链接，并新增"快速开始"小节顶部在线体验入口。
- 链接：https://a38ce1ad4e917321e.app.workbuddy.host/?sharecode=BPE6OFTPKE （由 WorkBuddy 托管，无需本地构建，含全部交互页面，演示账号已预填）。
- 注：该在线预览为沙箱托管静态服务，若下线以 WorkBuddy 重新发布的链接为准。

## [1.6] - 2026-10-29

### PRD §12 待确认决策化（WorkBuddy 复核后落地）
- 原 §12 的 8 项【待确认】按多 AI 评审与快速原型视角逐一拍板，转为【决策】：预算固定自然月、member 仅改本人记录、MVP 不做自动行情、MVP 仅 CNY、每日备份保留 7 份、成员上限 20、云端 LLM 兜底默认关闭、退出软删保留历史。
- 同步修订 §7.2 权限矩阵：原“编辑/删除他人记录”member 列【待确认】改为“✗（member 仅可编辑/删除本人记录）”，与 §12 决策对齐。
- 新增 §12.1“技术待确认（GPT 评审补充）”，收录 GPT 评审单列、且不阻塞 MVP 设计的 5 项技术开放项（权限变化缓存重建、管理员/owner 边界、关闭注册后的入户、断电持久性、删除恢复与审计）；云端兜底开关已由 §12 第 7 项决策覆盖。
- 阅读约定（L13）更新：第 12 章现汇总“决策结论与仍需技术拍板的事项”。

## [1.5] - 2026-10-29

### 性质与审查基线
- 按项目方请求，审核 `v1.4-修复说明.md` §6，并按 `多AI对齐规范.md` 的 13 个字段交付。基线 `9a3a2e38a6476450b04fea59c9cb49eb5dba7416`；本版本为**审核、跨文档裁决与可执行参考骨架修订**，不是完整生产应用发布。

### 裁决与函数签名（GPT-R23 / GPT-ADR01，已采纳；有效骨架已统一）
- 唯一公开领域入口：`TxService.Create(ctx context.Context, uid, bookID int64, req CreateRequest) (*CreateResult,error)`。HTTP 创建和 `sync/push` 每个离线创建项复用同一入口；内部 `createTransferInTx` 使用已有 session/version，不另开事务、占位或加版本。
- 旧 `AddTransaction`、独立可提交 `Transfer` 和模糊 `createRelatedTransfer` 退出有效骨架；历史记录/反例快照保留。规范见 `docs/FastLedger-转账入口裁决.md`，参考实现见指南 §13.1，方案 §5.1/§6.1、PRD §4.3.1 同步。
- 版本唯一实现为 `VersionRepo.Next`；创建与持仓使用 `VersionSource`。`AddHolding` 增加 `versions VersionSource` 末参，其调用和编译/真实数据库路径已验证。

### 新发现处理（GPT-R19–GPT-R23）
- **GPT-R19 / P1 / 已修复（参考骨架）**：回执占位、查询、结果回写不得吞错误；只对指定唯一键 `ON CONFLICT DO NOTHING`，重读后核对完整回执。结果回写失败整笔回滚。位置：指南 §13.1、裁决 §3。
- **GPT-R20 / P1 / 已修复（参考骨架）**：摘要统一为带域标签的完整 CreateRequest，覆盖分类/渠道等字段；回放显式核对 result_type，返回原 ID 和创建版本。位置：指南 §13.1、裁决 §3。
- **GPT-R21 / P2 / 已采纳**：创建键改为 DTO 必填与 Service 强制检查；JSON `clientUuid` / DB `client_uuid` 口径明确。Service 回归已通过，正式 Gin handler 的 400/409 接线仍待验收。位置：指南 §7.1/§13、裁决 §2。
- **GPT-R22 / P2 / 已修复（有效文档入口）**：指南的 nextBookVersion 实现删除，统一依赖方案 VersionRepo.Next，持仓与同步调用已传播。§3.5 SQL 仍是解释性算法，不是另一可调用实现。
- **GPT-R23 / P1 / 已修复（参考骨架）**：原 AddTransaction 接受 txType=3 却只插一条且不改余额的绕行反例已复现；当前所有创建类型只经 Create，转账一次写双边。

### 历史编号复核（不新增重复编号）
- **GPT-R03**：原信用卡还款分支仍给负债账户加余额，净资产 90000→70000 分的反例成立。统一分支按正欠款反转负债增量；四种资产/负债转账方向净资产守恒的隔离回归通过。此处关闭创建/还款分支，不宣称删除恢复已验收。
- **GPT-R06**：原转账账户版本未更新、交易 tx_time=0；本轮创建路径已将两侧账户、两条记录写同一版本和请求时间。全实体/端到端同步保持已采纳、待正式工程验证，不能据局部测试宣布全域关闭。
- **GPT-R16**：原初始化在实际编译器下出现 DB 返回值、Engine.QueryRow、外包接收者三处错误；本轮修正指南 §22.4，并让方案引用唯一初始化示例。修订 initDB 编译运行通过，验证 foreign_keys/WAL/连接池；整个工程构建仍待完成。

### 契约与验证材料
- 两份 DDL 同步 `idempotency_records` 与 `result_version`；枚举字典新增 result_type 和恢复世代字段契约。v1.4 说明加后续复核标记；多 AI 规范修正“18 项全 P1”和负欠款示例的文字错误，并更新进度范围。
- 完整 13 字段意见、原片段快照/摘要、真实 Go/xorm/SQLite 故障注入、修订回归、Hub 核心 race 与旧/新编译输出位于 `gpt/v1.4/`。测试数量及结果以 `verification/runtime-results.json` 为准。
- 验证使用最小表映射和 Hub 权限桩；不等同于正式 HTTP/WS/Flutter 或全应用端到端验收。已有试验库升级时须迁移回执新字段，不得删除旧回执以“解决”重试。

## [1.4] - 2026-10-29

### 背景
接手豆包 v1.3 的落地工作，针对《v1.3-复核-豆包落地核查》（workbuddy/）中标注的纰漏进行修正：**补齐转账幂等（P2，功能性数据重复风险）**、**统一版本号递增入口（P3）**、**强化幂等契约（P3）**，并明确 R18 描述合并口径。改动均落到 `docs/FastLedger-新手开发完整指南.md`。

### 修复（P2：转账幂等，闭环 R04 在转账路径的缺口）
- **`Transfer` 补齐对称幂等**：新增 `clientUuid string` 参数；`clientUuid` 非空时在事务内查 `idempotency_records`（ResultType=2 标记转账）→ 命中且 `transferHash(from,to,amount)` 一致则返回成功（不重复扣款）、内容不同则报冲突、不存在则占位（唯一索引兜底并发重试）；业务完成后回写 `result_ids`（两条交易 id）。此前 v1.3 的 `Transfer` 既不捕获 `clientUuid`、也不写回执，转账重试会产生重复转账——本次闭合该缺口。
- 新增 `transferHash(fromId, toId, amount)`，与 `AddTransaction` 的 `requestHash` 分离，避免不同类型互相误判为冲突。

### 修复（P3：统一版本号入口）
- **§22.1 消除重复实现**：原在 §22.1 手写了一份与 §13.1 `nextBookVersion` 完全相同的 `INSERT...ON CONFLICT(book_id)` upsert；改为统一调用 `nextBookVersion`，并加“禁止再手写”的约束说明，杜绝日后实现漂移。

### 修复（P3：强化幂等契约）
- 在 `AddTransaction` 入参处新增契约注释：所有经 handler 进入的写请求**必须携带 `client_uuid`**；未携带退化为非幂等（仅允许服务端内部直接调用）。明确端侧离线补传/重试务必带 `client_uuid`。

### 说明（R18 描述合并口径）
- R18（WS 并发锁顺序/清理）实际由 **v1.2** 修复；v1.3 仅补充并发清理细节，二者为同一修复，特此合并说明，避免 CHANGELOG 口径重复。

### 函数签名变更
- `Transfer` 增加 `clientUuid string` 末参（调用方：handler/路由需同步传入；离线补传/重试务必传端侧 `client_uuid`）。

### 说明
- 仍为 Markdown 设计与代码骨架，未跑真实编译；`Transfer` 幂等在真实 SQLite 唯一约束、并发重试、与 `AddTransaction` 共用 `idempotency_records` 表的运行时验证仍待执行（建议 `go test -race` + 集成测试覆盖转账重试幂等）。

## [1.3] - 2026-10-29

### 背景
处理 GPT 独立评审（`gpt/`，R01–R18）中 v1.2 未覆盖的全部 11 项，**全部采纳、无误判**。明细见 `workbuddy/v1.3-修复说明.md`。与 v1.1→v1.2 教训一致，所有改动均同步到开发者实际照敲的《新手开发完整指南》骨架。

### 修复（同步：R04/R07/R10）
- **R04 幂等**：新增 `idempotency_records` 表（`UNIQUE(book_id,uid,client_uuid)` + request_hash/result_ids），`AddTransaction` 在事务内查回执→相同返回/不同冲突/不存在占位，唯一索引兜底竞态；新增 `requestHash`。
- **R07 游标/快照**：WS 事件只通知、不直接推进游标；`Pull` 在同一只读事务（一致快照）内定水位并读取所有实体，新增 `SyncBatch`；分页不拆同一版本，缓冲溢出要求全量重同步。
- **R10 恢复世代**：新增独立于数据库、不随备份恢复的 `restore_epoch`（`data/instance.meta`）；恢复时递增，客户端发现世代不符即全量重建并核对离线队列。

### 修复（写入校验：R11/R13）
- **R11**：`AddHolding` 的 decimal 错误必须返回、校验非负，事务内校验账户/标的归属同账本并接版本号；`holdings` 补 `book_id/sync_version/deleted`。
- **R13 预算**：按 budget_type 分支，分类预算汇总"大类+全部小类"，总预算去掉分类谓词；半开区间；聚合改用 `SumInt`。

### 修复（安全：R14/R15）
- **R14 CSV 注入**：新增 `sanitizeCSVCell`，对以 `= + - @ \t \r` 开头的用户可控文本前置单引号；区分表格安全 CSV 与原始 JSON。
- **R15 OCR 千分位**：金额正则支持千分位，先全角规范化，整数方式转分（不用 double）；补普通/千分位/退款/多金额/异常样例。

### 修复（契约/范围/并发：R12/R16/R17/R18）
- **R12**：统一字段 `password_hash`/`members_can_view_others`（User 显式列映射）；WS 统一 `/api/v1/ws`、订阅 `books`、点号事件名、均带 version。
- **R16**：PRAGMA 改用 go-sqlite3 DSN（每连接生效）+ `SetMaxOpenConns(1)` + 启动校验；修正示例 API 编译错误。
- **R17**：统一 MVP 为 PRD 基线——v0.1 含基础报表（月度收支/分类占比）与 CSV/JSON 导出；高级图表/完整预算后置。
- **R18**：固定锁顺序 `Hub.mu→Client.mu`，清理恰好一次，`Send` 永不关闭，写循环加 ping，建议 race detector 验证。

### 函数签名变更
- `AddHolding` 增加 `uid, bookID` 首参；新增 `Pull(bookID, afterVersion) (*SyncBatch, error)`、`requestHash`、`sanitizeCSVCell`、模型 `IdempotencyRecord`。

### 说明
- 仍为 Markdown 设计与代码骨架，未跑真实 Go/Flutter 编译；`go test -race`、DSN 实际行为、Excel 重存等运行时验证待执行，未声称已验证。

## [1.2] - 2026-10-29

### 背景
根据《FastLedger-GPT评审复核》（workbuddy/）的逐条复核结论，将 GPT 评审（`gpt/`，R01–R18）中已确认的高优问题落实到**开发者实际照敲的骨架**（`新手开发完整指南`），并校准会计与隐私两条线。本轮采纳 7 项（R01/R02/R03/R05/R06/R08/R09），其余标注为待办/部分处理。

### 修复（同步 v1.1 正确实现到新手指南骨架）
- **交易/转账骨架对齐落地方案**：`新手开发完整指南` §13.1/§13.2 原仍是“先读后写余额、错误被吞、无版本号”的旧实现，已替换为与落地方案 §6.1 一致的写法——单条**原子 UPDATE** 改余额（带 `book_id` 与影响行数检查）、写事务内**递增 `book_versions` 并回写 `sync_version`**、转账两侧交易均带 `book_id`/`sync_version`（R02/R06）。
- **账户/交易绑定账本**：写路径的 `book_id` 一律来自可信参数（handler 解析），并校验资金账户/转账两侧账户归属同一账本，余额 UPDATE 带 `book_id` 与 `deleted=0` 且检查 `RowsAffected==1`，杜绝跨账本改余额与零行误判成功（R01）。

### 修复（会计：R03 信用卡负债符号）
- 新增 `signedBalanceDelta`：负债账户（category 10~15）消费使余额**增加**（欠更多）、还款使其**减少**，与《枚举字典》§2“正=欠款”约定一致；§16.2 汇总口径不变（净资产 = 资产 − 负债）。同步在《枚举字典》§2 写明余额符号约定，落地方案 §6.1 注明 `signedAmount` 必须按此实现。修复后：储蓄 1000、信用欠款 0，刷卡 100 后净资产 900（不再虚增为 1100），还款后仍为 900。

### 修复（隐私：R05 WebSocket 授权与可见性）
- WS 连接绑定认证 `uid`；订阅经 `Hub.Subscribe` 受锁更新，且每个账本 ID 都先由服务端校验成员资格，**不再信任客户端上报的账本 ID**；广播前按接收者可见性过滤，关闭互看时只发脱敏通知（不含金额/账户/备注），由客户端再走授权接口拉取明细（R05）。
- 连接读/写循环共用取消上下文、统一 `Unregister`+`Close`，订阅写统一加锁，闭合并发清理缺陷（R18）。

### 修复（认证：R08/R09）
- 移除 `新手开发完整指南` §10.1 中硬编码的 `var jwtSecret = []byte("改成你自己的密钥")`；改为 `InitAuth(secret)` 从配置注入，空值/占位值/过短则启动即失败（R08）。中间件 `JWTAuth()` 不再接受外部密钥参数，只读注入的同一份。
- `JWTAuth` 增加：限定 HS256（拒绝 alg=none）、安全读取 `uid`（非法 claims 返回 401 而非 panic）、校验账号是否已删除/禁用（R09）；登录写入 `jti` 并预留令牌吊销查询位。

### 待办（本轮未处理，建议后续版本逐项裁决）
- R04 幂等：需在 DDL 为 `(book_id, client_uuid)` 建唯一索引（或独立操作回执表），并在事务内捕获唯一冲突返回原结果。
- R07 WS 游标/一致快照、R10 备份恢复世代、R11 持仓 decimal 错误返回与 `AddHolding` 补 `book_id`/`sync_version`、R12 两套 DDL/WS 契约统一、R13 预算 SQL、R14 CSV 注入、R15 OCR 千分位、R16 PRAGMA 连接池作用域、R17 MVP 报表/导出范围一致性：详见 `gpt/` 评审，维持“待确认/待处理”状态。

### 说明
- 本轮改动均为**设计文档与 Markdown 代码骨架**，不依赖真实 Go/Flutter 编译；相关函数签名变化（如 `AddTransaction(uid, bookID, tx)`、`Transfer(uid, bookID, ...)`、`JWTAuth()`）已在对应章节注明，handler/路由调用需同步调整。

## [1.1] - 2026-10-29

### 背景
本版本根据第三方（含 AI）交叉评审《FastLedger-项目档案评审与优化建议》逐条核查后修复，共处理 15 项确认问题（1 项评审意见经核实为误判，未采纳）。

### 修复（P0）
- **统一多账本 MVP 口径**：PRD 原写"单账本"，与落地方案、用户原始需求矛盾。现统一为 **MVP 即支持多账本**（默认"家庭公共账本"，可新建"个人私账"并切换），同步更新 PRD §3.2、版本路线图与 README。
- **补全增量同步的全局版本号机制**：新增 `book_versions` 表（账本级、单调递增），写事务内原子 +1 并回写实体 `sync_version`；`sync/pull` 按 `afterVersion` 拉取。修复了原 `version` 恒为 1 导致增量同步不成立的问题。

### 修复（P1）
- 用户表移除独立 `salt` 列：bcrypt/argon2 已内含随机盐，仅保留 `password_hash`。
- `users.email` 改为**可选**（自托管常无 SMTP），并对非空邮箱做唯一索引。
- 移除 `transactions.geo_lng/geo_lat`：不再采集 GPS，与隐私定位一致。
- 区分录入耗时两个口径：**OCR 响应 < 3 秒**（设计目标，不含人工）与**完整一笔 ≤ 15 秒**（验收阈值）。
- 明确并发冲突策略：服务端权威（最后修改 + 版本号），冲突时用户选覆盖/放弃/另存；PRD 不再标"待确认"。
- 可见性补充成员级覆盖字段说明（`book_members.can_view_others` 为预留）。
- 修正原型页面数口径（13 个核心页面/路由）。

### 修复（P2）
- 明确 `accounts.parent_id` 仅用于 UI 分组、不参与余额计算。
- 持仓数量/价格以字符串存储，运算走十进制库（shopspring/decimal），禁止 float。
- 明确 `ChangeBalance` 为单条原子 UPDATE，避免并发丢更新。
- SQLite 连接初始化开启 `WAL` / `busy_timeout` / `foreign_keys`。
- 补全首个 owner 的初始化引导（首启向导或 CLI `user create --owner`）。

### 新增
- 《FastLedger 统一枚举字典》：所有枚举值的唯一权威来源。
- 本 CHANGELOG。
- README 补充文档分工与导航。

### 原型
- **删除残留的第三方遥测脚本**（slardar `ibytedapm.com`、tea `bytescm.com`），经验证原型现**零外部请求、可完全离线**运行。
- 说明：原型为构建产物的导出快照，仓库内暂无可编辑源，已在 prototype/README 注明。

### 未采纳（经核实为误判）
- 评审 3.3 称"两级分类、我的设置找不到入口"：实际入口在"我的"页（`/profile`），`/categories` 两级分类与设置页均完整存在。

## [1.0] - 2026-10-29
### 新增
- 首版 PRD（评审稿）、8 环节落地方案、新手开发完整指南、ezbookkeeping 代码导读。
- 完整可交互原型（13 个核心页面）。
