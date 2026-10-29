# FastLedger 文档评审报告（C 视角 · 独立第三方审阅者）

> 审阅者代号：**C**（第三个 AI，按《多 AI 对齐规范》§1 加入）
> 评审基线：本地克隆仓库 `/c/Users/47006/WorkBuddy/fastledger/repo_tmp`（相当于 `main` 最新快照）
> 评审日期：2026-10-29
> 适用范围：全部 PRD 与配套文档的**梳理总结**、**跨文档一致性**、**文档质量与缺口**，不重复既有 Rxx / WB / B 项。

---

## ① 摘要（TL;DR）

我独立通读了 FastLedger 仓库的 README、CHANGELOG、全部 `workbuddy/` 评审与对齐材料、5 份核心文档（`docs/`）、原型说明、早期草稿 `workbuddy/PRD.md` 与 `gpt/` 第三方评审交付，并按《多 AI 对齐规范》§2–§6 的颗粒度、编号、证据、状态与口径优先级标准产出本评审。

**结论**：经过多轮（WB→GPT→v1.1–v1.5）评审，FastLedger 的**会计正确性引擎、隐私基线、同步幂等与转账入口**已趋于严谨且经隔离运行验证，文档体系成熟度较高。但仍存在 **10 项"新"问题（6 项 P2 + 4 项 P3）**，全部集中在**文档质量、跨文档口径一致性、枚举覆盖、版本号命名**层面，**未发现新的 P0/P1 级工程正确性或安全绕过缺陷**（核心闭环已被前轮验证）。

按严重度分布：**P0 = 0，P1 = 0，P2 = 6，P3 = 4**。

**Top 3 最关键问题**：
1. **C-R01 版本号双轨制冲突**——产品功能路线图（v0.1–v0.4/v0.5）与文档迭代 CHANGELOG（[1.0]–[1.7]）共用同一套 "v1.x" 命名，导致 "v1.0" 一词既指"PRD 文档首版"又指"产品成熟期"，易在协作/验收时产生歧义。
2. **C-R03 权限矩阵冲突**——PRD §7.2 与 §12 决策 #2 称 admin 可编辑/删除他人记录（✓），但落地方案 §5.5 称 admin "改/删他人交易：可配置，默认不可"，自相矛盾且直接关系家庭数据治理。
3. **C-R05 环境变量命名不一致**——新手指南 §22.5 用 `ENABLE_REGISTER=false`，而落地方案 §8.1 与 PRD §12.1 用 `EBK_ENABLE_REGISTER=false`；若真实代码取其一，则"关闭公开注册"这一安全开关可能在另一处配置下失效。

**给项目方的后续建议**：在 CHANGELOG 中显式区分"文档迭代版本"与"产品功能版本"两套编号；以单份权威表统一权限矩阵（落地方案 §5.5 应与 PRD §7.2/§12 决策对齐，且口径优先级已规定技术以落地方案为准，需明确是否推翻 PRD 决策 #2）；全仓统一环境变量 `EBK_` 前缀；把 `book_type`、`cat_type` 纳入枚举字典；为 `workbuddy/PRD.md` 增加醒目"已废弃"横幅。

---

## ② 文档梳理总结（逐份）

### 2.1 入口与协作层

| 文档 | 用途 | 核心内容 | 权威层级 | 相互关系 |
|---|---|---|---|---|
| `README.md` | 仓库入口、导航、快速开始 | 产品定位、特性、文档分工表、原型链接、技术栈、**版本路线（摘要）v0.1–v0.4** | 入口层，非规范 | 指向所有 `docs/`；引用 `gpt/v1.4/` |
| `CHANGELOG.md` | 变更裁决唯一出口 | 文档迭代记录 `[1.0]`–`[1.7]`（均 2026-10-29），每条含 finding 状态 | 变更裁决出口（规范 §8） | 记录 B/WB/GPT 的落地结论；版本头格式 `## [x.y] - YYYY-MM-DD` |
| `workbuddy/第三方审阅者导读.md` | 新审阅者对齐入口 | 项目背景、已定决策清单、协作约定、阅读顺序 | 协作约定（非设计权威） | 引用 README/CHANGELOG/workbuddy 评审 |
| `workbuddy/多AI对齐规范.md` | 协作宪法 | 角色编号体系、§2 最小颗粒度、§3 严重度、§4 证据、§5 状态机、§6 口径优先级、§7 共享术语 | **最高协作标准** | 所有 AI 意见须遵守；口径优先级：枚举字典 > 落地方案 > PRD > 新手指南 > 原型 |

### 2.2 评审与修复历史（均已处理，仅供追溯）

| 文档 | 内容 | 状态 | 关系 |
|---|---|---|---|
| `workbuddy/FastLedger-项目档案评审与优化建议.md` | WB 第一轮评审（v1.0 档案），提出 P0 多账本/同步版本号、P1 遥测/salt/email/GPS 等 | 已落地（CHANGELOG v1.1） | 历史基线 |
| `workbuddy/FastLedger-v1.1-复核与下轮提示.md` | WB 复核 v1.1：14 项属实、1 项误判收回（两级分类） | 已记录 | 历史 |
| `workbuddy/FastLedger-GPT评审复核.md` | WB 对 GPT R01–R18 逐条核实，全部属实 | 已记录 | 历史 |
| `workbuddy/v1.2/v1.3/v1.4-修复说明.md` | 各轮落地明细（设计→骨架闭环） | 已合 | 抄送副本 |
| `workbuddy/v1.5-核对与就绪评估.md` | WB 复核 GPT 真实运行核验；判定"能开发（照骨架）但非可部署" | 已记录 | 历史 |

### 2.3 核心设计文档（权威技术内容）

| 文档 | 用途 | 核心内容 | 权威层级 | 相互关系 |
|---|---|---|---|---|
| `docs/FastLedger-产品需求文档-PRD.md` | 当前权威 PRD（评审稿 v1.0） | 术语、目标/KPI、范围与路线图、功能需求、数据模型摘要、权限、验收、§12 决策 | **需求基线**（口径优先级第 3） | 范围冲突以 PRD 为准；字段语义受枚举字典约束，技术实现受落地方案约束 |
| `docs/FastLedger-落地方案.md` | 8 环节研发可执行方案 | 关键决策表、需求/产品/架构/DB/API/编码/测试/部署 8 环节；完整建表 SQL；WBS | **权威技术设计**（口径优先级第 2） | 技术选型唯一权威；被 PRD/新手指南/裁决引用 |
| `docs/FastLedger-新手开发完整指南.md` | 可照敲代码骨架（2357 行） | 分层、JWT、统一 `TxService.Create`、转账 `createTransferInTx`、WS、同步 Pull、预算、OCR、初始化 | **代码骨架**（须随设计同步，优先级第 4） | 由落地方案派生；血泪教训 v1.1→v1.2 要求"设计改→骨架必改" |
| `docs/FastLedger-枚举字典.md` | 枚举唯一权威源 | TxType/AccountCategory/PaymentChannel/…/IdempotencyResultType/恢复世代 等 13 节 | **字段/符号契约最高**（优先级第 1） | 所有端/文档/DB 以本表为准；新增枚举只能末尾追加 |
| `docs/FastLedger-转账入口裁决.md`（GPT-ADR01） | 唯一 Create 入口裁决 | 统一 `TxService.Create`、HTTP/同步契约、幂等回执、余额方向表 | 落地方案 §6.1 的技术细化 | 由落地方案 §6.1 引用；已闭合 v1.4 双入口分歧 |
| `docs/ezbookkeeping-代码导读.md` | 开源参考导读 | ezbookkeeping 分层、模型、转换器、LLM 抽象 | 参考（非约束） | 仅借鉴，不直接照搬 |

### 2.4 原型与废弃草稿

| 文档 | 用途 | 核心内容 | 权威层级 | 相互关系 |
|---|---|---|---|---|
| `prototype/` + `prototype/README.md` | 13 页可交互原型（构建产物快照） | 本地静态服务、演示账号、零外部请求离线 | 演示（不约束实现，优先级第 5） | 已删遥测；仓库内无可编辑源 |
| `workbuddy/PRD.md` | 早期由 AI 起草的标准 PRD 基线 | 需求基线早期版（MVP 不含导出/报表/投资、KPI≤5s、2 角色） | **非正式口径**（仅追溯） | 已被 `docs/PRD.md` 取代；导读 §7 注明"仅供追溯" |

### 2.5 第三方评审交付

| 文档 | 用途 | 权威层级 |
|---|---|---|
| `gpt/FastLedger-review-b874146.md`、`gpt/findings.json`、`gpt/CHANGELOG.md`、`gpt/issues/*.md`、`gpt/v1.4/*` | GPT 独立评审与真实运行核验（R01–R23） | 第三方发现，WB 已核实；不在本仓库设计权威体系内 |

---

## ③ 去重说明

评审前已按《多 AI 对齐规范》§1、§5 要求，对 `CHANGELOG.md` + `workbuddy/` + `gpt/` 做全文 grep 与逐条比对，确认以下既有结论**未重复**：

- **WB 第一轮评审（评审与优化建议）**：多账本 MVP 口径、同步全局版本号机制、salt 列、email 可选、GPS 移除、ChangeBalance 原子性、WAL、`parent_id`、decimal、首个 owner 引导、遥测、原型源、枚举字典、KPI 度量、CHANGELOG——均已在 v1.1 落地，本案不涉及。
- **GPT R01–R18 / R19–R23**：账本绑定、转账吞错、信用卡负债符号、幂等、WS 鉴权/游标、JWT、恢复世代、非法金额、两套 DDL/WS 契约、预算 SQL、CSV 注入、OCR 千分位、PRAGMA、MVP 范围、WS 并发、统一 Create、版本入口去重等——均已在 v1.2–v1.5 处理/核实，本案不重提。
- **已定决策（第三方审阅者导读 §5）**：多账本 MVP、增量同步、隐私定位、并发 409、整数分、OCR<3s/整笔≤15s、email 可选/无 GPS/password_hash 无独立 salt、§12 八项决策、统一入口 `TxService.Create`/`VersionRepo.Next`——本案尊重，不重开辩论。

本案 **C-R01–C-R10** 均为上述清单之外的新发现或既有"待确认"项的**未闭合状态追踪**（仅 C-R06 属于对 PRD §12.1 已列待确认项的"代码仍矛盾、未闭合"追踪，已在去重中标注，不构成新编号占用）。

---

## ④ Findings 清单（§2 全字段，C-Rxx，P0→P3）

> 严重度分布：**P0 = 0 条；P1 = 0 条；P2 = 6 条（C-R01~C-R06）；P3 = 4 条（C-R07~C-R10）**。

---

### C-R01 · P2 · 版本号双轨制冲突：产品功能版本（v0.1–v0.5）与文档迭代版本（[1.0]–[1.7]）共用 "v1.x" 命名

- **location**：`README.md` L26、L65–L72；`CHANGELOG.md` L5/L12/L20/L47/L71/L101/L128/L165；`docs/FastLedger-产品需求文档-PRD.md` L164–L170；`docs/FastLedger-落地方案.md` L1179–L1185
- **quote**：
  - README L67–L70：`- **v0.1（MVP）**... - **v0.2**... - **v0.3**... - **v0.4**...`
  - README L26：`| [`gpt/v1.4/`](gpt/v1.4/) | GPT 定向审核与真实运行核验 | v1.4 §6 复核...`
  - CHANGELOG L5：`## [1.7] - 2026-10-29`
  - PRD L170：`| v1.0+ | 打磨与增强 | xlsx/PDF 导出、预算结转...`
- **problem**：仓库内存在两套并行的 "v 编号"：①**产品功能路线图**用 v0.1→v0.5（README/PRD §3.3/落地方案 §8.6）；②**文档与档案迭代**用 `[1.0]`→`[1.7]`（CHANGELOG）。两者在 "v1.0" 一词上直接碰撞——CHANGELOG `[1.0]` 指"PRD 文档首版"，而 PRD §3.3 把 "v1.0+" 用于"产品成熟期打磨"。读者（尤其新成员/协作者）无法从字面区分"这是文档改到第几版"还是"产品做到哪个阶段"，验收与排期易混淆。
- **reproduce**：grep `v0\.` 与 `## \[1\.`：README 同时出现 v0.1–v0.4（路线图）与 v1.4（文档引用）；CHANGELOG 全为 [1.0]–[1.7]；PRD 既写 v0.1–v0.4 又写 v1.0+。
- **actual**：同一字符串 "v1.0" 在仓库内表达两种不同含义，且无任何文档声明两套编号的区分。
- **expected**：应明确区分"文档/档案迭代版本"与"产品功能版本"，并声明映射关系，避免 `v1.x` 双关。
- **impact**：协作与验收口径混乱；外部审阅者/贡献者可能误判项目成熟度（以为 [1.7] 是产品 v1.7）。
- **suggestion**：在 CHANGELOG 顶部与 README 增加一段"版本说明"：明确 `[x.y]` 为**文档/档案迭代版本**（仅本仓库文档演进），`v0.x/v1.x` 为**产品功能路线版本**；或把文档迭代改为独立前缀（如 `doc-1.0`）。
- **status**：待确认
- **evidence**：C 核实：grep 全仓确认 README 路线图 v0.1–v0.4（L67–70）与 README 文档引用 v1.4（L26）、CHANGELOG `[1.0]`–`[1.7]`（8 个版本头均为 2026-10-29）、PRD §3.3 v0.1–v0.4 及 v1.0+（L164–170）并存，无任何区分声明。

---

### C-R02 · P2 · 仓库名拼写不一致：`FastLadger`（gpt 第三方评审交付物）vs `FastLedger`（全仓正式文档）

- **location**：`gpt/FastLedger-review-b874146.md` L3；`gpt/issues/01-accounting.md` L22–L94 等多处；`gpt/issues/02-synchronization.md`、`03-authentication-websocket.md`、`04-contracts.md` 等同源 URL
- **quote**：
  - gpt/FastLedger-review-b874146.md L3：`审查日期：2026-10-29（北京时间）。仓库：LinTia/FastLadger。`
  - gpt 各 issue：`https://github.com/LinTia/FastLadger/blob/b8741465.../docs/FastLedger-产品需求文档-PRD.md#L...`
- **problem**：第三方 GPT 评审交付物中所有 GitHub 引用均写作 `github.com/LinTia/FastLadger`（"Ledger" 误拼为 "Ladger"），而本仓库全部正式文档（README/CHANGELOG/docs/workbuddy）与目录名均使用正确拼写 `FastLedger`。该拼写差异会导致评审报告内的所有溯源链接 404（真实仓库应为 `LinTia/FastLedger`），削弱 R01–R23 的"可定位"价值。
- **reproduce**：`grep -rn "FastLadger" gpt/` → 命中 gpt/FastLedger-review-b874146.md L3 与 gpt/issues/*.md 中全部 `github.com/LinTia/FastLadger/...` 链接；对比 `grep -rn "FastLedger" README.md CHANGELOG.md docs/ workbuddy/` 全部为正确拼写。
- **actual**：gpt/ 下约 40+ 处链接与 1 处仓库声明使用错误拼写 `FastLadger`。
- **expected**：第三方评审内的仓库名与链接应与项目正式拼写 `FastLedger` 一致，链接可点开。
- **impact**：评审溯源链接失效；外部读者可能误以为存在拼写不同的独立仓库；轻度损害评审交付物的可信度。
- **suggestion**：在 `gpt/` 目录（或 README）补一段说明："评审报告撰写时仓库临时命名为 LinTia/FastLadger，正式项目名为 FastLedger，链接中的 FastLadger 为笔误，请按 FastLedger 对应本地路径查阅"；或将链接改为相对路径。
- **status**：待确认
- **evidence**：C 核实：grep 确认 `FastLadger` 仅出现在 `gpt/` 目录（评审交付物），正式文档零命中；链接域名确为 `github.com/LinTia/FastLadger`。

---

### C-R03 · P2 · 权限矩阵冲突：PRD §7.2/§12 决策 #2 称 admin 可改删他人记录，落地方案 §5.5 称 admin"默认不可"

- **location**：`docs/FastLedger-产品需求文档-PRD.md` L405、L531；`docs/FastLedger-落地方案.md` L925
- **quote**：
  - PRD L405：`| 编辑/删除他人记录 | ✓ | ✓ | ✗（member 仅可编辑/删除本人记录） |`
  - PRD L531：`2. 【决策】编辑/删除他人记录：member 仅可编辑/删除本人记录；owner 与 admin 可管理本账本全部记录（最小权限原则，见 §7.2 权限矩阵）。`
  - 落地方案 L925：`| 改/删他人交易 | ✅ | 可配置 | 默认不可 |`
- **problem**：PRD 在 §7.2 权限矩阵与 §12 决策 #2 两处明确"admin 可编辑/删除他人记录（✓）"，但落地方案 §5.5 权限校验矩阵写"admin 改/删他人交易：可配置，默认不可"。同一角色能力在两份权威文档中直接矛盾。且按《多 AI 对齐规范》§6 口径优先级，**技术/实现以落地方案为准**，意味着实际实现应"默认不可"，但这与 PRD 已拍板的决策 #2（admin 可管理全部记录）冲突——属于"已定决策被低优先级文档反向改写"的反例。
- **reproduce**：并排比对 PRD §7.2 第 2 行（admin=✓）与落地方案 §5.5 第 5 行（admin=默认不可）。
- **actual**：admin 对他人交易的编辑/删除权在 PRD 与落地方案之间不一致，且 PRD §12 已将其列为【决策】。
- **expected**：两份文档应对齐；若保留"admin 可管理全部记录"的决策，则落地方案 §5.5 应改为 ✓（或显式标注"经账本设置可下放"），并在 CHANGELOG 记录裁决。
- **impact**：家庭数据治理边界模糊；开发者按落地方案实现会得到"admin 默认不可"，与产品决策背离，验收时产生争执。
- **suggestion**：以 PRD §12 决策 #2 为基准，修订落地方案 §5.5 第 5 行为 `✅（可经账本设置下放）` 并加注；或在 §12.1 明确"admin 对他人交易默认不可、owner 可授权"，再反向更新 PRD §7.2/§12。需在 CHANGELOG 留痕。
- **status**：待确认
- **evidence**：C 核实：PRD L405/L531 与落地方案 L925 原文比对确认矛盾；口径优先级 §6 规定技术以落地方案为准，但 §12 决策 #2 为 PRD 已拍板项，二者需裁决。

---

### C-R04 · P2 · 落地方案内部版本引用不一致：§4.2 用 "v2/v3"，§8.6 路线图用 "v0.3/v0.4"

- **location**：`docs/FastLedger-落地方案.md` L16、L1183–L1184
- **quote**：
  - 落地方案 L16（决策记录·投资与资产）：`v2 自动行情（A股/港股/基金净值）、v3 完整资产合计`
  - 落地方案 L1183：`| v0.3 | 投资自动估值 | **行情 Provider 实现（A股/港股/基金净值/黄金）**...`
  - 落地方案 L1184：`| v0.4 | 家庭资产合计 | **资产/净资产总览（本币折算）**...`
- **problem**：落地方案**同一文档内部**对投资自动行情与资产合计的版本归属自相矛盾：决策记录（L16）称其为 "v2/v3"，而 §8.6 路线图（L1183–L1184）与 PRD §3.3（v0.3/v0.4）将其归为 "v0.3/v0.4"。读者无法判断该能力到底落在哪个迭代。
- **reproduce**：grep `v2\|v3\|v0.3\|v0.4` 落地方案 → L16 出现 v2/v3，L1183–L1184 出现 v0.3/v0.4。
- **actual**：同一能力在 L16 标 v2/v3，在 L1183–L1184 标 v0.3/v0.4。
- **expected**：全文档统一为同一版本编号（建议与 PRD §3.3 一致，采用 v0.3/v0.4）。
- **impact**：范围基线混乱，排期与验收无法对齐；与 PRD 路线图不一致，触发 C-R01 的版本双轨问题进一步恶化。
- **suggestion**：将 L16 的 "v2/v3" 改为 "v0.3/v0.4"，或在 L16 加注"对应 §8.6 的 v0.3/v0.4"。
- **status**：待确认
- **evidence**：C 核实：落地方案 L16 与 L1183–L1184 原文比对确认内部矛盾。

---

### C-R05 · P2 · 环境变量命名不一致：新手指南 `ENABLE_REGISTER` vs 落地方案/PRD `EBK_ENABLE_REGISTER`

- **location**：`docs/FastLedger-新手开发完整指南.md` L2162；`docs/FastLedger-落地方案.md` L1119；`docs/FastLedger-产品需求文档-PRD.md` L545
- **quote**：
  - 新手指南 L2162：`建好后该入口失效；`ENABLE_REGISTER=false` 不影响它。`
  - 落地方案 L1119：`- EBK_ENABLE_REGISTER=false        # 初始账号建好后关闭公开注册`
  - PRD L545：`关闭公开注册后的成员入户：`EBK_ENABLE_REGISTER=false` 时...`
- **problem**：全仓配置变量统一采用 `EBK_` 前缀（落地方案 §8.1 出现 `EBK_HTTP_PORT`、`EBK_JWT_SECRET`、`EBK_LLM_PROVIDER` 等），唯独新手指南 §22.5 描述首个 owner 引导时使用 `ENABLE_REGISTER=false`（缺 `EBK_` 前缀）。若真实代码以某一文档的变量名为准，则另一处配置的"关闭公开注册"这一安全开关将不生效，导致自托管实例在期望关闭注册时仍可公开注册，扩大攻击面。
- **reproduce**：`grep -n "ENABLE_REGISTER\|EBK_ENABLE_REGISTER" docs/` → 仅新手指南命中 `ENABLE_REGISTER`（L2162），落地方案 L1119 与 PRD L545 命中 `EBK_ENABLE_REGISTER`。
- **actual**：新手指南与落地方案/PRD 对同一开关使用不同变量名（是否带 `EBK_` 前缀）。
- **expected**：全仓统一为 `EBK_ENABLE_REGISTER`，与 `EBK_` 前缀约定一致。
- **impact**：潜在安全配置失效风险（公开注册可能未被关闭）；且与"隐私/自托管安全"定位相悖。
- **suggestion**：将新手指南 L2162 的 `ENABLE_REGISTER=false` 改为 `EBK_ENABLE_REGISTER=false`，与落地方案/PRD 对齐。
- **status**：待确认
- **evidence**：C 核实：grep 确认新手指南仅 L2162 出现 `ENABLE_REGISTER`，其余变量均为 `EBK_` 前缀；落地方案 L1119、PRD L545 均用 `EBK_ENABLE_REGISTER`。

---

### C-R06 · P2 · "断电持久性"待确认项未闭合，且权威骨架仍硬编码 `_synchronous=NORMAL`

- **location**：`docs/FastLedger-产品需求文档-PRD.md` L546；`docs/FastLedger-新手开发完整指南.md` L2131（§22.4 initDB）
- **quote**：
  - PRD L546：`【待确认】断电持久性：落地方案 §6.1 的 \`synchronous=NORMAL\` 在掉电时可能丢失最近已提交事务，需与"无数据丢失"质量目标对齐（建议 MVP 改 FULL 或配套恢复策略）。`
  - 新手指南 L2131：`"_synchronous=NORMAL"        // WAL 下安全且更快`
- **problem**：PRD §12.1 已把"NORMAL 在掉电下可能丢已提交事务"列为**待确认**风险项，并指出与 §2.3 质量目标"数据可靠性 ≥ 99%、无数据丢失"冲突；但作为"权威技术设计/代码骨架"的落地方案/新手指南 `initDB` 仍直接硬编码 `_synchronous=NORMAL`，未提供 FULL 切换或补偿恢复策略。该待确认项处于"已提出问题但未闭合、代码仍用风险默认值"的悬空状态。
- **reproduce**：比对 PRD §12.1 待确认 #4 与新手指南 §22.4 DSN 字符串。
- **actual**：风险已标注为待确认，但骨架默认仍是 NORMAL，未闭合。
- **expected**：在 MVP 前明确裁决：改为 `FULL` 或给出"NORMAL + 定期 checkpoint/备份 + 单机家庭负载可承受"的书面合理性说明，并同步到 initDB 注释。
- **impact**：在家庭 NAS 意外断电场景下，最近一笔已"提交"的交易可能丢失，与"无数据丢失"质量目标直接冲突；属可靠性缺陷，若 MVP 不解决会落入 P1 区间。
- **suggestion**：在 CHANGELOG 关闭该项（采纳 FULL 或记录豁免理由）；若保留 NORMAL，请在 §22.4 注释中写明"家庭单机、WAL、每日备份，已评估断电窗口可承受"，并将 PRD §12.1 该条从【待确认】转为【决策】。
- **status**：待确认（对既有 PRD §12.1 待确认 #4 的未闭合追踪）
- **evidence**：C 核实：PRD L546 标注 NORMAL 为待确认风险；新手指南 L2131 initDB DSN 确为 `_synchronous=NORMAL`；落地方案 §6.1（L1005）直接引用该 initDB，未改默认值。

---

### C-R07 · P3 · PRD §6.2 字段名与权威 DDL 漂移，且 PRD 内部自相矛盾（owner_id / transaction_time）

- **location**：`docs/FastLedger-产品需求文档-PRD.md` L363、L368、L84；`docs/FastLedger-落地方案.md` L596、L696；`docs/FastLedger-新手开发完整指南.md` L312、L395
- **quote**：
  - PRD L363：`| books | id, name, owner_id, visibility, created_at |`
  - PRD L368：`| transactions | ... transaction_time ... |`
  - PRD L84：`按 \`transactions.tx_time\` 统计每个用户当天有记录的去重天数`
  - 落地方案 L596：`owner_uid        INTEGER NOT NULL,`；L696：`tx_time           INTEGER NOT NULL,`
- **problem**：PRD §6.2 "关键表（字段级摘要）"使用 `owner_id`（应为 `owner_uid`）与 `transaction_time`（应为 `tx_time`），与落地方案/新手指南的实际列名不符；更严重的是 PRD **自身** §6.2 写 `transaction_time`、§2.3 写 `tx_time`，同一列在作者文档内命名都不一致。作为"需求基线"，字段名漂移会让开发者/前端对接时无所适从。
- **reproduce**：grep `owner_id|owner_uid`、`transaction_time|tx_time` 全 docs/。
- **actual**：PRD §6.2 用 `owner_id`/`transaction_time`，PRD §2.3、落地方案、新手指南用 `owner_uid`/`tx_time`。
- **expected**：PRD §6.2 字段名应与权威 DDL（落地方案 §4.5）完全一致，且 PRD 内部不出现两个名字。
- **impact**：文档可读性下降；前端/后端对接时可能按错误字段名生成代码。
- **suggestion**：将 PRD §6.2 的 `owner_id`→`owner_uid`、`transaction_time`→`tx_time`，并与 §2.3 对齐。
- **status**：待确认
- **evidence**：C 核实：grep 确认 PRD L363/L368 与 L84 命名不一致，且落地方案 L596/L696、新手指南 L312/L395 均为 `owner_uid`/`tx_time`。

---

### C-R08 · P3 · 枚举字典覆盖不全：`books.book_type`、`categories.cat_type` 未纳入唯一权威源

- **location**：`docs/FastLedger-落地方案.md` L599、L672；`docs/FastLedger-枚举字典.md`（全文）
- **quote**：
  - 落地方案 L599：`book_type        TINYINT DEFAULT 1,        -- 1=个人 2=家庭（预留）`
  - 落地方案 L672：`cat_type    TINYINT NOT NULL,             -- 1=收入 2=支出`
  - 枚举字典：全文无 `book_type`/`cat_type` 任何条目
- **problem**：落地方案 §4.5 DDL 定义了 `books.book_type`（1=个人/2=家庭）与 `categories.cat_type`（1=收入/2=支出）两个枚举型字段，但《统一枚举字典》未收录二者。违反"各端枚举值对齐、枚举字典为唯一权威来源"的设计初衷（README 文档分工表明确枚举字典管"各端枚举值对齐"）。
- **reproduce**：`grep -n "book_type\|cat_type" docs/FastLedger-枚举字典.md` → 无命中；`grep -n "book_type\|cat_type" docs/FastLedger-落地方案.md` → L599、L672。
- **actual**：两个枚举字段仅在 DDL 注释中定义，未进枚举字典。
- **expected**：在枚举字典中新增 `book_type`（个人/家庭）与 `cat_type`（收入/支出）两节，作为唯一权威。
- **impact**：各端（Flutter/Web/原型）可能自行定义取值，导致跨端不一致（尤其 `cat_type` 影响收支汇总口径）。
- **suggestion**：在枚举字典新增第 14/15 节分别定义 `BookType`、`CategoryType` 及取值与启用版本。
- **status**：待确认
- **evidence**：C 核实：枚举字典 13 节（§1–§13）不含 book_type/cat_type；落地方案 §4.5 注释给出其取值。

---

### C-R09 · P3 · 早期废弃草稿 `workbuddy/PRD.md` 与现行 `docs/PRD` 口径漂移，且仍留仓库缺醒目警告

- **location**：`workbuddy/PRD.md` L33、L82–L86、L176–L178；`docs/FastLedger-产品需求文档-PRD.md`（现行）
- **quote**：
  - workbuddy/PRD.md L33：`| 单笔录入耗时 | 从打开记账到确认保存 | ≤ 5 秒（理想 3 秒） |`
  - workbuddy/PRD.md L176–L178：`- 必含：账本+成员、账户+分类、交易...；暂不含（移至 v0.2+）：投资模块、预算、导出、iOS 全局截图闭环、2FA。`
  - 现行 docs/PRD L73：`录入耗时 | 完整一笔（触发到入账，含人工确认）≤ 15 秒`；L154–L157：MVP 含"基础报表、CSV/JSON 导出、投资/资产模块的数据骨架"
- **problem**：`workbuddy/PRD.md` 是早期基线草稿，其 MVP 范围（不含导出/报表/投资骨架）、成功指标（≤5s vs 现行 ≤15s）、角色（仅管理员/普通成员 2 角色）均与现行 `docs/FastLedger-产品需求文档-PRD.md` 冲突。虽《第三方审阅者导读》§7 注明其"仅供追溯、非正式口径"，但该文件仍置于 `workbuddy/` 目录且文件名同为 `PRD.md`，缺少醒目"已废弃/以 docs/PRD 为准"横幅，新读者极易误将旧口径当作现行需求。
- **reproduce**：比对 workbuddy/PRD.md L33/L178 与 docs/PRD L73/L154–157。
- **actual**：旧草稿与现行 PRD 在 MVP 范围、KPI、角色三处冲突，且旧文件未明确标记废弃。
- **expected**：在 workbuddy/PRD.md 顶部加醒目横幅："⚠️ 本文件为早期探索草稿，已被 docs/FastLedger-产品需求文档-PRD.md 取代，非正式口径，请勿据此开发"。
- **impact**：新成员/AI 误读旧 MVP 范围或 KPI，导致需求基线混乱。
- **suggestion**：给 workbuddy/PRD.md 加废弃横幅（如上），或移至 `workbuddy/archive/` 子目录。
- **status**：待确认
- **evidence**：C 核实：workbuddy/PRD.md L33/L82–L86/L176–L178 与 docs/PRD L73/L154–157 字段级比对确认冲突；导读 §7 仅文字注明"仅供追溯"。

---

### C-R10 · P3 · CHANGELOG 全部条目日期同为 2026-10-29，未区分文档迭代与真实时间线

- **location**：`CHANGELOG.md` L5、L12、L20、L47、L71、L101、L128、L165
- **quote**：`## [1.7] - 2026-10-29` / `## [1.6] - 2026-10-29` / `## [1.5] - 2026-10-29` / `## [1.4] - 2026-10-29` / `## [1.3] - 2026-10-29` / `## [1.2] - 2026-10-29` / `## [1.1] - 2026-10-29` / `## [1.0] - 2026-10-29`
- **problem**：8 个文档迭代版本头 `[1.0]`–`[1.7]` 的日期**全部为 2026-10-29**，与"本仓库为单日设计文档汇总"的事实相符，但未作任何声明。外部读者可能误以为这是跨越多次真实发布的演进时间线，夸大项目成熟度；亦与 C-R01 的版本双轨问题叠加，加剧"v1.x 是什么"的困惑。
- **reproduce**：`grep -n "^- 2026-10-29\|## \[1" CHANGELOG.md` → 8 个版本头日期一致。
- **actual**：所有版本头日期均为同一天。
- **expected**：在 CHANGELOG 顶部说明"本文记录的是文档/档案迭代，均于 2026-10-29 单日汇总，不代表产品发布时间线"；或仅在首个版本头保留真实日期、其余标注"同日汇总"。
- **impact**：可信度与可读性受损；与 C-R01 共同造成版本认知混乱。
- **suggestion**：CHANGELOG 顶部增加说明段（见 C-R01 建议的"版本说明"合并处理）。
- **status**：待确认
- **evidence**：C 核实：grep 确认 8 个版本头日期均为 2026-10-29。

---

## ⑤ 改进建议（含改后文本 + 位置）

> 遵循《第三方审阅者导读》§6.6：直接给"改后文本 + 位置"，便于采纳并写进 CHANGELOG。

**建议 1（对应 C-R01/C-R04/C-R10）：在 CHANGELOG.md 顶部与 README 增加"版本说明"段**
- 位置：`CHANGELOG.md` 第 1 行之后；`README.md` "版本路线（摘要）"小节前。
- 改后文本：
  ```
  > 版本说明：本文档 `[x.y]` 为**文档/档案迭代版本**（仅记录本仓库设计文档的演进），
  > 与产品功能路线版本（v0.1 MVP → v0.5 → v1.0+，见 PRD §3.3 与落地方案 §8.6）相互独立。
  > 各 `[x.y]` 均于 2026-10-29 单日汇总生成，不代表真实发布时间线。
  ```
- 同步：将 `docs/FastLedger-落地方案.md` L16 的 `v2/v3` 改为 `v0.3/v0.4`（对应 C-R04）。

**建议 2（对应 C-R03）：对齐权限矩阵，明确 admin 对他人交易的能力**
- 位置：`docs/FastLedger-落地方案.md` L925。
- 改后文本（二选一，推荐 A）：
  - A（与 PRD §12 决策 #2 对齐）：`| 改/删他人交易 | ✅ | ✅（可经账本设置下放） | 默认不可（由 owner 授权） |`
  - B（若采纳落地方案"默认不可"）：同步改 PRD L405/L531 与 §12，并在 CHANGELOG 记录"推翻决策 #2"。

**建议 3（对应 C-R05）：统一环境变量前缀**
- 位置：`docs/FastLedger-新手开发完整指南.md` L2162。
- 改后文本：`建好后该入口失效；`EBK_ENABLE_REGISTER=false` 不影响它。`

**建议 4（对应 C-R06）：闭合断电持久性待确认项**
- 位置：`docs/FastLedger-新手开发完整指南.md` L2131（注释）与 `docs/FastLedger-产品需求文档-PRD.md` L546。
- 改后文本（若保留 NORMAL）：
  - 新手指南 L2131 注释追加：`// 家庭单机部署 + WAL + 每日备份，已评估断电窗口可承受；如要求更强持久性改 _synchronous=FULL`
  - PRD L546：将该条由【待确认】改为【决策】并引用上述理由；或在 CHANGELOG 记录"采纳 FULL"。

**建议 5（对应 C-R07）：修正 PRD §6.2 字段名**
- 位置：`docs/FastLedger-产品需求文档-PRD.md` L363、L368。
- 改后文本：L363 `books | id, name, owner_uid, visibility, created_at |`（并补全 `book_type/default_currency` 等已建字段）；L368 把 `transaction_time` 改为 `tx_time`。

**建议 6（对应 C-R08）：补全枚举字典**
- 位置：`docs/FastLedger-枚举字典.md` 末尾（§13 之后新增 §14、§15）。
- 改后文本：
  ```
  ## 14. 账本类型 BookType（`books.book_type`）
  | 值 | 名称 |
  |---|---|
  | 1 | Personal 个人 |
  | 2 | Family 家庭（预留） |

  ## 15. 分类类型 CategoryType（`categories.cat_type`）
  | 值 | 名称 |
  |---|---|
  | 1 | Income 收入 |
  | 2 | Expense 支出 |
  ```

**建议 7（对应 C-R09）：为废弃草稿加醒目横幅**
- 位置：`workbuddy/PRD.md` 第 1 行之后。
- 改后文本：`> ⚠️ 本文件为早期探索草稿，已被 \`docs/FastLedger-产品需求文档-PRD.md\` 取代，非正式口径，请勿据此开发。`

**建议 8（对应 C-R02）：修正 gpt/ 内仓库名拼写**
- 位置：`gpt/FastLedger-review-b874146.md` L3 与各 `gpt/issues/*.md` 的 GitHub 链接。
- 改后文本：将 `LinTia/FastLadger` 统一改为 `LinTia/FastLedger`，或在 `gpt/README.md` 增加笔误说明并将链接改为相对路径。

---

## ⑥ 待确认与遗留

1. **需项目方裁决的口径冲突（C-R03）**：admin 对他人交易的改/删权，PRD §12 决策 #2 与落地方案 §5.5 二选一，建议以 PRD 决策 #2 为准并改落地方案，留 CHANGELOG 记录。
2. **待闭合的技术待确认（C-R06）**：`synchronous=NORMAL` 的断电持久性风险，MVP 前必须给出口（FULL 或书面豁免），否则与"无数据丢失"质量目标冲突。
3. **版本编号体系（C-R01/C-R04/C-R10）**：建议一次性统一产品功能版本与文档迭代版本两套命名，避免后续协作持续踩坑。
4. **未做运行时验证的已知空白（沿用 v1.5 结论，非新发现）**：Gin Handler 接线、真实 WS 生命周期、Flutter 源码、OCR 后端契约、转账删除/恢复、迁移与端到端同步仍待正式工程验收；本评审不重复，仅提醒这些仍是"能开发但非可部署"的边界。
5. **本评审未覆盖**：因聚焦文档质量与跨文档一致性，未对 `gpt/v1.4/verification/` 的隔离测试做二次复跑；未对原型 13 页交互做逐页功能核验（按导读 §6.3，原型改交互需回原工具，不在本仓库范围）。

---

*评审人：C（第三个 AI，独立审阅者）｜评审基线：仓库 `main` 本地快照｜方法：逐文档通读 + grep 原文/行号取证 + 与 CHANGELOG/gpt/workbuddy 去重｜遵循《多 AI 对齐规范》§2–§6。*
