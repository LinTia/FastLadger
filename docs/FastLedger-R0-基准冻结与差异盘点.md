# FastLedger R0：基准冻结与差异盘点

> 文档性质：**R0 阶段交付（盘点，非代码完成）**。本文件只做基准冻结、逐页差异盘点、根因分类与裁决清单，**不修改任何业务代码 / PRD / 落地方案 / 枚举字典 / WBS**。
> 依据：`docs/FastLedger-原型还原与PWA分阶段实施总方案.md` §5「R0」、§6「强制交付格式」；`workbuddy/多AI对齐规范.md`（颗粒度/编号/状态/文档权威）。
> 编制：实施 AI（WorkBuddy）。编制日期：2026-11-04。
> **独立复核更正**：本文件部分结论已由 [`FastLedger-R0-独立补漏与复核记录.md`](FastLedger-R0-独立补漏与复核记录.md) 修订；凡原文与复核记录冲突，以复核记录为准。复核仅为静态审查，未运行测试、未采集截图，R0 尚未通过。

---

## 0. 开工前回执（§6.1 强制）

| 项 | 内容 |
|---|---|
| 当前基线 SHA | 见 §0.2「证据版本基准」：**本轮整改执行基准** `273cf08...`（fetch 时 `main` HEAD）；业务代码证据基准 `bff6a2b...`；运行时采集时历史文档 HEAD `557e4d2...`；截图对应后端二进制代码 SHA **未独立验证**；`1cd56ab...` 为历史快照、不再作为本轮有效代码基准。不同证据资产的基准不合并、不互相推定。 |
| 工作分支 | 本次为 GitHub 默认分支只读复核；本地工作分支/工作区状态未验证。 |
| 关键决策文档（原编制回执所列） | `workbuddy/多AI对齐规范.md`、`workbuddy/第三方审阅者导读.md`、根 `CHANGELOG.md`、原型还原与 PWA 总方案、手机 PWA 指导书、Web 设计规范、交易更新语义设计、两版原型 README。原回执未列 PRD、落地方案、开发 WBS、枚举字典；不能据此证明原编制者已逐项核对。独立复核已另行对照这些文件，详见补漏记录。 |
| 本次范围 | R0-A ~ R0-G（页面/交互盘点、两版原型差异、当前实现与后端 API 映射、差异根因、截图与覆盖率方法、Owner 裁决清单） |
| **明确不做** | 不改 `server/`、`client/`、`server/web/dist/` 业务代码；不改 PRD/落地方案/枚举字典/WBS；不做 PWA 代码；不做 UI 重构；不给"还原度百分比"主观估值（见 §1.3） |
| 相关证据源 | `prototype/`（构建产物）、`prototype-v2/index.html`、`server/web/dist/{index.html,app.js,style.css}`、`server/internal/api/router.go`、`docs/FastLedger-Web设计规范.md` §8 |

### 0.1 与 GPT 评审的对齐说明（"与 GPT 对齐"）

1. **接受 GPT 的口径纠偏**：总方案 §3.3 明确不得把交接文档中未经评分矩阵证明的"50–55% 还原度"当作事实。本 R0 **不再复述该百分比**，改用"页面 × 关键交互 × 状态 × 断点"的加权矩阵（§7）统计，百分比须公布分母/权重/证据后才可出现。
2. **最终前端基准（2026-11-04 Owner 裁决，原 D-1）**：页面/流程/交互以 `prototype/` 为唯一基准；视觉风格/组件细节以 `prototype-v2/` + `docs/FastLedger-Web设计规范.md` + `design-tokens.json` 为补充；`prototype-v2/` 不得仅凭视觉覆盖原型业务流程，冲突先登记。
3. **区分"后端有能力"与"前端已接通"**：总方案 §3.1 指出不得把"有同名页面/能调 API"冒充原型还原。本 R0 的 R0-D 对每条后端能力标注"前端是否调用"，避免误判。
4. **不动已定决策**（导读 §5）：多账本、同步版本号、整数分、软删除/恢复/审计、隐私定位（图片不上传）等一律不改，仅为视觉一致不得破坏。
5. **本地分支状态**：原编制者报告其前端提交已 rebase 至当时 GPT `main` 之上；本次独立复核无法读取其本地工作区，也无法验证是否干净或是否存在未推送提交。GitHub 端只确认本报告列明的远端提交。

---

### 0.2 证据版本基准（RV-02 纠偏，[1.68] 全文复核）

> 不同证据资产的基准**不合并、不互相推定**；本表为 §0 / §3.1 / §5.3 / §7.5 / §10.4 的统一口径。
> **本轮整改执行基准**是执行前 `git fetch` 得到的 `origin/main` HEAD；本提交落地后会产生新 SHA，
> 该新 SHA 记录于根 `CHANGELOG.md` [1.68] 与 `docs/FastLedger-R0-第三次整改交付回执.md`，
> **不在本表循环写入尚未生成的自身 SHA**。

| 证据类别 | 基准 / 状态 | 说明 |
|---|---|---|
| 本轮整改执行基准（文档） | `273cf0869917565b60f7da1dd2b562764de8d405`（`273cf08`，第三次整改任务单 + [1.67] 落地时 `main` HEAD） | 仅表示本轮开工时的文档版本；**审计文档变更不代表业务代码基准发生变化** |
| 业务代码证据基准 | `bff6a2b351bf5a039cdf7c762ec2693fb330cd00` | §3.1 / §5.3 的 `file:line` 源码取证均按此提交实读；其后各提交相对它仅有文档 / 资产变更，**无业务代码变更** |
| 运行时证据采集时历史文档 HEAD | `557e4d223064c149cdf1063429a2fdd79158e609`（`557e4d2`） | 5 张 PNG 与 3 个 JSON 采集当时的文档 HEAD，**仅为历史时点标记，不是当前报告版本，也不是代码基准**；见 `docs/r0-screenshots/runtime/README.md` |
| 截图 / 运行时对应的后端二进制代码 SHA | **未独立验证** | 服务二进制构建于 `2026-11-04 01:13:59 +0800`；实测 HTTP 提供的 `/app.js`、`/index.html` 与仓库 HEAD 一致、`/style.css` 仅 CRLF 差异，但**后端二进制对应的代码 SHA 未独立验证**，不得推定等于 `bff6a2b` |
| 历史快照 | `1cd56ab7830d2d3f33faf7fc5372d3824c088ae0` | **历史快照，不再作为本轮有效代码基准**；仅用于解释 [1.61] 当时的回执记录 |

> **全文 SHA 语义复核（[1.68]）**：`557e4d2` 在主报告仅保留于本表（采集时历史 HEAD）、§5.3 引言 / §5.3.0（声明源码取证不按该文档 HEAD）、§10.3（[1.65] 历史记录）与 runtime README（证据时点）；`CHANGELOG.md` [1.65] / [1.67] 中的 `557e4d2`、`ddbd1ae` 均为**历史条目**，不改写；外部审阅产物《独立复核补充 RV-01至04》的 `575eae3` 为其自身复核时点基准，按原文保留。命中清单见 `docs/FastLedger-R0-凭证安全核验记录.md` §6 与交付回执 §4。

---

## 1. 基准与权威顺序

### 1.1 权威顺序（引用总方案 §2.1 / 规范 §6）

```
枚举字典 > 落地方案 > PRD > 开发WBS/新手指南 > 原型与设计规范（页面/交互/视觉目标参考）
```

原型与上位业务契约冲突时，**先登记冲突并请求 Owner 裁决**，实现方不得自行选边。

### 1.2 两套原型的定位

| 来源 | 形态 | 定位（本 R0 采用） |
|---|---|---|
| `prototype/` | 在线原型导出的构建产物快照（`assets/index-DNvhksAn.js` 等，**仓库内无可编辑源码**） | 页面清单 / 业务流程 / 交互基准 |
| `prototype-v2/` | 依 Web 设计规范手写的单文件 `index.html`（65KB，可读） | 视觉/交互参考；**不得仅凭 v2 覆盖原版流程** |

### 1.3 量化口径（禁止主观估值）

- 覆盖率 = Σ(页面权重 × 交互达成 × 状态达成 × 断点达成) ÷ 分母；分母、权重、证据必须随报告公布。
- 无证据不得给结论；不确定一律标 `[待确认]`（规范 §4）。

---

## 2. R0-A：页面、导航与入口清单

### 2.1 原型 13 页（来自 `prototype/README.md` 与构建产物取证）

> 取证：`prototype/assets/index-DNvhksAn.js` 中命中「首页/账单/记一笔/转账/截图识别/交易详情/账户/分类/投资资产/预算/报表/导出/家庭成员/我的/设置」等文案；`prototype/README.md` 明列 13 页。

| # | 原型页面 | 原生入口 | 当前 Web 路由（hash） | 当前 render 函数 | 现状 |
|---|---|---|---|---|---|
| 1 | 登录 | 根 | `#`（无 token） | `renderLogin` L161 | 已实现 |
| 2 | 首页账单 | 底部 Tab 首页 | `#b/:id`（默认） | `renderHome` L1929 + `renderBillStream` L1872 | 已实现（清单见 §6 R0-E-02） |
| 3 | 截图识别记账 | 记一笔 → 截图识别 | Web 无路由 | 无 Web OCR 调用 | **Web 不实现符合 PRD §5.2；Flutter 端 OCR 不在本次 Web R0 范围内** |
| 4 | 手动记账/转账 | 记一笔（中央按钮） | `#b/:id`（弹窗 `#formDlg`） | 记一笔 L529 / `openTxEditForm` L583 | 已实现 |
| 5 | 交易详情 | 账单行点击 | `#b/:id/tx/:txId` | `renderTxDetail` L1974 | 已实现 |
| 6 | 账户 | 我的 → 账户管理 | `#b/:id/accounts`、`#b/:id/account/:accId` | `renderAccounts` L647 / `renderAccountDetail` L1768 | 已实现 |
| 7 | 两级分类 | 我的 → 分类管理 | `#b/:id/categories` | `renderCategories` L2069 | 已实现 |
| 8 | 投资资产 | 我的 → 投资资产 | `#b/:id/invest` | `renderInvest` L1246 | 已实现（对齐度待截图核验） |
| 9 | 预算 | 我的 → 预算 | `#b/:id/budget` | `renderBudget` L1108 | 已实现 |
| 10 | 报表 | 底部 Tab 报表 | `#b/:id/report` | `renderReport` L804 | 已实现 |
| 11 | 导出 | 我的 → 数据导出 | `#b/:id/export` | `renderExport` L933 | 已实现 |
| 12 | 家庭成员 | 我的 → 家庭成员 | `#b/:id/members` | `renderMembers` L1636 | 已实现 |
| 13 | 我的设置 | 底部 Tab 我的 / 设置 | `#b/:id/mine`、`#settings` | `renderMine` L2138 / `renderSettings` L1518 | 已实现 |

**原型 README 明确**：分类管理 / 数据导出 / 家庭成员 / 设置 位于"我的"页（`/profile`）的二级菜单中 —— 即**二级页应挂在"我的"中枢下**，而非平铺顶栏（见 R0-E-01）。

### 2.2 当前实现的额外入口（原型 13 页之外）

| 入口 | 路由 | render | 依据 | 定位 |
|---|---|---|---|---|
| 账本列表 | `#books` | `renderBooks` L234 | PRD 多账本（已定决策） | 合理扩展，保留 |
| 交易总表 | `#b/:id/txns` | `renderTxns` L337 | 桌面浏览/检索/批量删除 | 扩展，保留（非原型页） |
| 回收站 | `#b/:id/recycle` | `renderRecycle` L751 | PRD 软删除/回收站（已定决策） | 扩展，保留 |

### 2.3 导航结构对照（关键差异）

| 维度 | 设计规范 §8.1 / prototype-v2 | 当前 `server/web/dist/index.html` | 结论 |
|---|---|---|---|
| 桌面顶栏 | 首页 / 报表 / 账户 / 预算 / 我的（**5 项**） | 账本 / 交易 / 报表 / 预算 / 资产 / 导出 / 对账 / 成员 / 回收站 / 设置（**10 项平铺**） | **不符**（R0-E-01） |
| 移动底栏 | 首页 / 记一笔（中央）/ 报表 / 我的（**4 Tab**） | 首页 / 账单 / 记一笔（FAB）/ 报表 / 我的（**5 Tab**） | **不符**（多"账单"，R0-E-01） |
| 二级页归属 | 账户/分类/预算/投资/成员/导出/设置 挂"我的" | 平铺在顶栏 | **不符**（R0-E-01） |

---

## 3. R0-B：逐页交互 / 状态矩阵

> 依据：`docs/FastLedger-Web设计规范.md` §8.2「关键页面交互规格」+ `prototype-v2/index.html` 结构取证。原型 `prototype/` 为构建产物，交互以 v2 + 规范为准并标 `[待截图核验]`。

| 页面 | 必备交互（规范 §8.2） | 当前实现证据 | 状态（空/加载/错误/权限/离线） | 结论 |
|---|---|---|---|---|
| 登录 | 预填、pending、错误红字、成功进首页 | `renderLogin` L161；`POST /auth/login`/`/auth/register` | 错误红字/pending [待确认] | 基本具备，细节待验 |
| 首页账单 | 概览三卡联动、按日分组账单、行点击进详情、筛选条、骨架屏 | `renderHome` L1947–1965；`renderBillStream` L1872–1927：**骨架屏与按日小计已实现**；行点击→`#b/:id/tx/:tid` | 首页无筛选；账单行缺账户/渠道；**请求失败时骨架屏未清理（本机隔离复验已观察到；R0-E-35 缺陷尚未关闭，见 §7.5）**；日小计仅基于首批 60 条 | **部分具备**（见 R0-E-02、20、21、34；R0-E-35 本机隔离复验已完成 / 缺陷尚未关闭（见 §7.5 标准口径）） |
| 记一笔 | 支出/收入/转账分段、转账双账户、必填校验红字、pending→toast→返回首页顶部插入 | `#formDlg`（`fType` 为普通 select；含金额/账户/对方账户/分类/日期/备注）；`POST /transactions` | 缺支付渠道/商户输入；类型不是分段控件；其余校验/pending 需浏览器验证 | **部分具备**（见 R0-E-23、29） |
| 截图识别 | 三级漏斗 → 确认弹窗 → 确认入账（绝不自动入账） | Web 无入口/无 OCR 调用 | — | **PRD §5.2 明确 Web 不做 OCR；Flutter 端能力另行审计** |
| 交易详情 | 全字段、编辑、删除二次确认→toast+余额回滚 | `renderTxDetail` L1974–2015：展示账户/分类/日期/备注；编辑/删除入口存在 | 未展示 paymentChannel；备注非空时 merchant 不显示；删除文案称无法恢复，与回收站/恢复能力矛盾 | **部分具备**（见 R0-E-22、24） |
| 账户 | 账户卡（余额/类型徽标）、净资产汇总、正负余额色 | `renderAccounts` L647–735；账户 CRUD | 账户列表无净资产摘要（净资产在 `renderInvest` L1246）；无期初余额输入 | **部分具备**（见 R0-E-06、28） |
| 分类 | 大类卡片→子类列表、分类色板 | `renderCategories` L2069；`GET /categories?tree=1` + CRUD | 删除引用计数 [待确认] | 具备 |
| 预算 | 总进度条 + 信封列表、正常/接近/超支三态 | `renderBudget` L1108；`GET/POST/PUT/DELETE /budgets` | 三态色 [待确认] | 具备，视觉待验 |
| 报表 | 月度柱状（chart-1 主色）+ 分类占比环形（conic）+ 月份切换 | `renderReport` L804；`GET /reports/monthly?months=6` + `/reports/categories` | 空态 [待确认] | 具备，视觉待验 |
| 家庭成员 | 角色徽标、账本级可见性开关 + 常驻隐私提示、邀请管理 | `renderMembers` L1636–1775：成员 CRUD、创建邀请码；逐成员显示 `mb.canViewOthers` | 无账本级开关、无常驻隐私提示；MVP 权威字段为 `books.members_can_view_others`，成员级字段仅预留；无邀请码撤销 UI | **部分具备**（见 R0-E-19、26） |
| 导出 | 筛选 + 预览 + CSV/JSON | `renderExport` L933（时间范围预设 + "共N条"预览 + `GET /export` blob 下载 + `X-Export-Count/Truncated`） | 空态/超大截断 [待确认] | 具备 |
| 设置 | 个人资料、账号安全、API Token、服务器与备份、退出登录 | `renderSettings` L1518–1635：API Token 管理 + 退出；邮箱资料编辑在 `renderMine` L2138–2215 | 资料能力存在但入口在“我的”；设置缺改密 UI、会话管理 UI（R0-E-14）、服务器/备份区块 | **部分具备**（见 R0-E-17、18、14） |
| 我的 | 菜单中枢 | `renderMine` L2138：头像 + 资料编辑（改邮箱）+ **8 个二级入口**（我的账本/账户/成员/导出/报表/预算/资产/回收站） | — | 具备（8 入口；与平铺顶栏重复，R0-E-01） |

**状态覆盖缺口（R0-E-12）修正**：须区分两个判断：①「首页账单流已实现骨架屏」——已有源码与隔离运行时 pending 截图证据（`docs/r0-screenshots/runtime/01-pending-390x844.png`），不得再列为「骨架屏缺失」；②「特定请求失败时是否正确清理骨架屏」——**R0-E-35 标准口径：本机隔离运行时复验已完成；失败态观察到骨架屏未清理（`.skel`=4、`aria-busy=true`）；待 Owner/CI 独立复跑及慢速网络、有数据态、多角色等场景验证；缺陷尚未关闭。** 本轮隔离实测（§7.5）已观察到失败后骨架仍在，但属单次本机实测，**未关闭缺陷**。空态已有局部实现；网络错误、权限不足、离线、同步中、冲突仍未形成逐页证据矩阵，需逐页验证并截图。

### 3.1 逐控件交互清单（二次整改重做版）

> **取证基线（业务代码证据基准，见 §0.2）**：`bff6a2b351bf5a039cdf7c762ec2693fb330cd00`（执行时 `git fetch` 已重试成功，本地 0 ahead / 0 behind）；源码行号全部按该提交重新定位：`server/web/dist/index.html`（151 行）、`server/web/dist/app.js`（2241 行）。**该 SHA 不是截图采集对应代码版本的推定依据**（§0.2 / §7.5）。
> **本轮纠偏**：原 §3.1 把多个独立控件合并成一行（如金额/账户/分类/日期/备注合并、FAB 与 Tab 合并、账户增改删合并、「我的」8 入口合并），且误引不存在的 `#loginForm`。本版**每个独立输入框 / 按钮 / Tab / 菜单 / 弹窗动作单独一行**，统一 13 字段。
> **字段口径**：`代码位置` = DOM selector 或 JS 函数 + 业务代码证据基准 `bff6a2b` 的行号；`验证证据` = `C:` 静态源码定位、`S:` 本轮 Edge 真实浏览器截图（路径相对 `docs/r0-screenshots/`）、或明确的「未采集 / grep 无命中」；`当前状态` 区分 `已实现` / `已实现但缺陷` / `缺失` / `待验证` / `不适用`，并关联原 R0-E 编号（不另起重复编号）。**标「运行时未验证」即未验证，不用「—」掩盖。**
> **RV-03 字段语义词表**（本版统一消歧）：`N/A（不适用）` = 该字段对本控件确实不适用；`见 I-XXX` = 行为由该**真实存在**的交互条目覆盖；`未验证` = 无运行时证据；`未实现` = 代码中无对应能力；`待确认` = 需求/产品行为尚未裁决。`代码位置` 列形如 `—（grep 无命中）` 为带注解的「未实现」，不是语义不明的裸 `—`；本版已消除逐控件表中全部裸 `—` 单元格（228 行 / 13 字段不变）。
> **截图范围**：本轮共采集 43 张真实截图 + 5 张 RV-01 隔离运行时截图（`docs/r0-screenshots/runtime/`，含 pending / rejected / 恢复态与真实 CDP 日志），覆盖 4 个视口 × 16 个页面/弹层；但**仅首屏/空态/错误态**，绝大多数成功/失败交互路径、加载态、权限不足态、离线态、原型侧对照截图仍未采集，见 §7.5 阻塞清单。因此本清单的 `S:` 证据只证明「控件/页面在此状态下可见」，**不证明交互成功路径已验收**。

#### 3.1.1 登录 / 注册 / 加入账本

> `renderLogin`（app.js L161–231）动态生成**无 id** 表单；`index.html` 中 `#loginForm`/`#registerForm`/`#joinForm` 均 grep 无命中（原 §3.1 L121 引用 `#loginForm` 属证据错误，本轮更正）。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-LOGIN-01 | 登录 | 用户名输入 | app.js L178（变量 u `<input>`） | 无 token | placeholder=用户名, autocomplete=username, required | 键入 | 输入框显示文本 | 浏览器 required 提示 | 无 | 留在登录页 | C:app.js L178；S:login/390x844/normal.png（首屏可见输入框） | 已实现｜输入态运行时未验证 |
| I-LOGIN-02 | 登录 | 密码输入 | app.js L179–181（变量 p） | 无 token | type=password, required；注册模式 minLength=8，placeholder「密码（至少 8 位）」 | 键入 | 密文显示 | required / 不足 8 位提示 | 无 | 留在登录页 | C:app.js L179–181；S:login/390x844/normal.png | 已实现｜输入态运行时未验证 |
| I-LOGIN-03 | 注册 | 确认密码输入 | app.js L184–185（变量 p2） | 注册模式 | type=password, placeholder=确认密码, required | 键入 | 密文显示 | required 提示 | 与 p 比对见 I-LOGIN-07 | 留在注册页 | C:app.js L184–185；注册态截图未采集 | 已实现｜运行时未验证 |
| I-LOGIN-04 | 登录/注册 | 密码明文切换 | —（grep 无命中） | 登录或注册 | 未实现 | 点击显隐切换 | 应切换 type=text/password | 未实现 | 未实现 | 未实现 | 缺失：app.js 全文 `showPassword\|明文\|type="text"` 无命中；p.type 恒为 password（L179/L184） | **缺失**｜本轮二次复核新增 R0-E-39 |
| I-LOGIN-05 | 登录 | 登录提交按钮 | app.js L182（变量 b「登 录」） | 未登录，表单有效 | N/A（不适用） | 点击 → form submit | 触发 I-LOGIN-06 | 见 I-LOGIN-13 | 见 I-LOGIN-06 | 见 I-LOGIN-06 | C:app.js L182；S:login/390x844/normal.png | 已实现｜运行时未验证 |
| I-LOGIN-06 | 登录 | 登录提交行为 | app.js L205–218 | 表单有效 | btnPending「登录中…」L207 | POST /auth/login {username,password} L209 | data.accessToken→state.token + localStorage fl_token/fl_whoami + toast「登录成功，欢迎回来！」L210–214；hash=#books L215 | toast(err.message,"err") L216；按钮 finally 恢复 L217 | 写入 localStorage；401 由 api() L105–109 清会话回登录页 | #books | C:app.js L205–218；运行时：登录 API 调用见 `docs/r0-screenshots/runtime/README.md` 复现步骤（本轮实测取到 accessToken）；UI 提交截图未采集 | 已实现｜UI 运行时未验证 |
| I-LOGIN-07 | 注册 | 注册提交行为 | app.js L187–202 | 注册模式；两次密码一致 | 先校验 p.value!==p2.value→toast「两次输入的密码不一致」L189；btnPending L190 | POST /auth/register L192 → 成功后 POST /auth/login L193（自动登录） | toast「注册成功，已自动登录！」+ 写 localStorage + hash=#books L194–199 | 密码不一致 toast（不请求）；注册/登录失败 toast(err.message,"err") L200 | 写入 localStorage | #books | C:app.js L187–202；注册态截图未采集（受 EBK_ENABLE_REGISTER 开关影响） | 已实现｜运行时未验证 |
| I-LOGIN-08 | 登录 | 注册入口切换 | app.js L220–228（toggle `<a>`） | 登录/注册页 | N/A（不适用） | 点击「没有账号？注册新账号」/「已有账号？返回登录」→ renderLogin(mode) L227 | 重渲染为对应表单（注册多出确认密码、按钮变「注 册」） | 未验证 | 无（原地重绘，无 hash 变化） | 留在登录/注册页 | C:app.js L220–228；S:login/390x844/normal.png（可见「没有账号？注册新账号」入口） | 已实现｜截图仅见入口，切换后未采集 |
| I-LOGIN-09 | 登录/注册 | 注册关闭 / 返回登录 | app.js L227 | 注册模式 | N/A（不适用） | 点击「已有账号？返回登录」 | 重渲染登录表单 | N/A（不适用） | N/A（不适用） | 登录页 | C:app.js L227；切换后截图未采集 | 已实现｜运行时未验证 |
| I-LOGIN-10 | 登录 | 加入账本入口 | —（grep 无命中） | 未登录 | 未实现 | 应存在第三入口 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：app.js `joinForm\|joinLedger\|redeemInvite` 无命中；renderLogin 仅 login/register 两模式（L161–231）；但 renderMembers L1667 文案「对方在登录页选择「加入账本」并输入邀请码即可」为悬空引导 | **缺失**｜本轮二次复核新增 R0-E-38 |
| I-LOGIN-11 | 登录 | 邀请码输入与提交 | —（grep 无命中） | 未登录 | 未实现 | 应输入邀请码并兑换 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：app.js 全文无 `/invite/.../redeem` 或 `/members/join` 调用；后端 POST /books/:id/members/join 公开（router.go L128）前端未接 | **缺失**｜R0-E-38 |
| I-LOGIN-12 | 登录 | 邀请码失效 / 异常反馈 | —（无控件） | 未登录 | 未实现 | 未实现 | 未实现 | 无错误提示实现 | 未实现 | 未实现 | 缺失：依赖 I-LOGIN-11，控件与反馈均不存在 | **缺失**｜R0-E-38 |
| I-LOGIN-13 | 登录/注册 | 登录 / 注册失败反馈 | app.js L216 / L200 | 请求失败 | N/A（不适用） | N/A（不适用） | 未验证 | toast 显示后端 msg（err 变体） | 按钮 finally 恢复（L201/L217） | 停留当前页 | C:app.js L200,L216,L110；错误态截图未采集 | 已实现｜运行时未验证 |
| I-LOGIN-14 | 登录/注册 | 重复提交 / pending 恢复 | app.js L56（btnPending）+ L201/L217 finally | 提交中 | 按钮禁用 + 文案切换 | N/A（不适用） | 未验证 | 失败也恢复按钮 | N/A（不适用） | 未验证 | C:app.js L56,L201,L217；运行时未验证 | 已实现｜代码层｜运行时未验证 |
| I-LOGIN-15 | 全局 | 顶栏退出 | index.html L25 / app.js L155 | 已登录 | N/A（不适用） | 点击 #logoutBtn | clearSession() + route() | 未验证 | 清 localStorage fl_token/fl_whoami | 登录页 | C:index.html L25, app.js L155；运行时未验证 | 已实现｜运行时未验证 |

#### 3.1.2 账本列表 `#books`

> `renderBooks` app.js L234–303；截图：`books/390x844/normal.png`、`books/1280x800/normal.png`。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-BOOKS-01 | 账本列表 | 账本行链接 | app.js L250–255 | 已登录 | N/A（不适用） | 点击 → #b/{id} | 进入账本首页 | N/A（不适用） | N/A（不适用） | #b/{id} | C:app.js L250–255；S:books/390x844/normal.png | 已实现｜截图已见 |
| I-BOOKS-02 | 账本列表 | 账本删除按钮 | app.js L260–276 | 列表加载后 | confirmDialog 二次确认 L263–267 | DELETE /books/:id L271 | toast「账本已删除」+ renderBooks() L272–273 | toast(err.message,"err") L274 | 重绘本页 | 本页 | C:app.js L260–276；删除态未采集 | 已实现｜运行时未验证｜R0-E-16 |
| I-BOOKS-03 | 账本列表 | 删除取消 | app.js L268 | 确认框打开 | N/A（不适用） | 点「取消」/ if(!okD) return | 不删除 | N/A（不适用） | 无 | 本页 | C:app.js L268 | 已实现｜运行时未验证 |
| I-BOOKS-04 | 账本列表 | 新账本名称输入 | app.js L283–284 | N/A（不适用） | placeholder=新账本名称, required, maxLength=40 | 键入 | 显示文本 | required 提示 | N/A（不适用） | 未验证 | C:app.js L283–284；S:books/390x844/normal.png | 已实现｜截图已见 |
| I-BOOKS-05 | 账本列表 | 账本类型下拉 | app.js L285–288 | N/A（不适用） | option 1=个人账本 / 2=家庭账本 | 选择 | 选中值 | 未验证 | N/A（不适用） | 未验证 | C:app.js L285–288；S:books/390x844/normal.png | 已实现｜截图已见 |
| I-BOOKS-06 | 账本列表 | 创建账本提交 | app.js L289–300 | 名称非空 | btnPending「创建中…」L293 | POST /books {name,bookType} L295 | toast「账本已创建」L296；hash=#b/{bk.id} L297 | toast(err.message,"err") L298 | 新建账本写入 | #b/{id} | C:app.js L289–300；运行时未验证 | 已实现｜运行时未验证 |
| I-BOOKS-07 | 账本列表 | 空态 | app.js L242–248 | 无账本 | N/A（不适用） | N/A（不适用） | 「还没有账本——在下面创建第一个吧。」 | 未验证 | N/A（不适用） | 本页 | C:app.js L242–248；本轮本地库有 1 个账本，空态未采集 | 已实现｜空态未采集 |
| I-BOOKS-08 | 全局 | 品牌点击 | index.html L11 / app.js L156 | 已登录 | N/A（不适用） | 点击 #brand | hash=#books | 未验证 | N/A（不适用） | #books | C:index.html L11, app.js L156 | 已实现｜运行时未验证 |
| I-BOOKS-09 | 全局 | 顶栏「账本」 | index.html L13（#navBooks） | 已登录，有账本 | N/A（不适用） | 点击 | hash=#books | 未验证 | N/A（不适用） | #books | C:index.html L13；S:books/1280x800/normal.png | 已实现｜截图已见｜R0-E-01 |
| I-BOOKS-10 | 账本列表 | 账本重命名 UI | —（grep 无命中） | 已登录 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：app.js 无 `api("PUT","/books/"` 调用；PUT 仅用于 accounts/transactions/budgets/categories/holdings/instruments/members/users/me；后端 PUT /books/:id 已有（router.go L161） | **缺失**｜R0-E-32 |
| I-BOOKS-11 | 账本列表 | 账本详情 / 账本设置页入口 | —（grep 无命中） | 已登录，已选账本 | 未实现 | 应存在承载账本设置的独立入口 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：`route()` 分支（app.js L2200–2239）无账本设置 hash；账本重命名（R0-E-32）与账本级可见性开关（R0-E-19）均无承载页面，仅散落在成员页/账本列表 | **缺失**｜R0-E-19 / R0-E-32 |

#### 3.1.3 首页 `#b/:id`

> `renderHome` L1929–1971 + `renderBillStream` L1872–1926；截图 `home/{360x800,390x844,430x932,1280x800}/empty.png` 与 `home/390x844/error.png`。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-HOME-01 | 首页 | 账本切换控件（本页内） | —（无） | 已选账本 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：renderHome L1929–1971 无账本 select；切换只能经 #brand/#navBooks → #books（I-BOOKS-08/09） | **缺失**｜R0-E-01 |
| I-HOME-02 | 首页 | 月份切换控件（首页） | 未实现 | 已选账本 | 待确认 | 待确认 | 待确认 | 待确认 | 待确认 | N/A（不适用） | **RV-04 查证（无明确需求依据）**：`docs/FastLedger-产品需求文档-PRD.md` §3.1 L132「首页与账单＝本月概览、账单流、筛选」、§4.2 L192–198（仅本月三卡/按日分组/筛选/记一笔/实时联动）、§4.8 L261–266（「切换时间范围」属报表页）；`docs/FastLedger-Web设计规范.md` §8.1 L305 首页＝本月概览+账单流+筛选+记一笔、§8.2 L323 首页必备交互无月份切换、L330「报表…+ 月份切换」；`prototype-v2/index.html` `pHome()` L685–715 只有类型 chips 与本月三卡，无月份切换；`prototype/assets/index-DNvhksAn.js` 「上月」仅出现在导出页时间预设（this_month/last_month/this_quarter/custom）；`prototype/README.md` 13 页清单无此交互。**结论：PRD / 原型 / 设计规范均无「首页必须提供月份切换控件」条款，月份切换属报表页。** | **待 Owner 裁决的交互建议**（移出已确认缺陷统计、不计入 R0 缺陷数；不得擅自扩大需求范围）｜RV-04 |
| I-HOME-03 | 首页 | 本月收入卡 | app.js L1961（mkCard） | 月报返回 | GET /reports/monthly?months=2 L1949 | 渲染 cur.income | 显示 +¥… | 月报异常静默降级 {income:0,expense:0}（catch(_){} L1949） | 无 | 本页 | C:app.js L1949,L1961；S:home/390x844/empty.png（显示 +¥0.00） | 已实现｜截图已见 |
| I-HOME-04 | 首页 | 本月支出卡 | app.js L1962 | 月报返回 | 同上 | 渲染 cur.expense | 显示 -¥… | 同上 | 无 | 本页 | C:app.js L1962；S:home/390x844/empty.png（显示 -¥0.00） | 已实现｜截图已见 |
| I-HOME-05 | 首页 | 结余卡 | app.js L1963 + L1951 | 账户列表返回 | `for (const a of accs) balance += a.balance \|\| 0` | Σ 账户余额并标「结余」+ 小字「净资产」 | 显示 ¥… | 同上 | 无 | 本页 | C:app.js L1951,L1963；S:home/390x844/empty.png（第三卡显示 结余 ¥0.00 / 净资产） | **已实现但财务口径错误**｜R0-E-20（Σ账户余额 ≠ 本月结余，更 ≠ 净资产；净资产公式见 service/invest.go L541–569） |
| I-HOME-06 | 首页 | 首页筛选入口 / 筛选条 | —（首页无） | 已选账本 | N/A（不适用） | N/A（不适用） | 未验证 | 未验证 | N/A（不适用） | 未验证 | 缺失：renderHome 全文无 filters；筛选能力在交易总表 `.filters`（L352–366：关键词/类型/账户/分类/起止日期/查询/重置），不可笼统说「全局缺失」 | **缺失（首页）**｜R0-E-02 / R0-E-21 |
| I-HOME-07 | 首页 | 每条账单行 | app.js L1907–1923 | 有交易数据 | N/A（不适用） | 点击 row → #b/{id}/tx/{tx.id} L1922 | 进入交易详情 | 未验证 | 见 I-TXDET-01 | #b/:id/tx/:txId | C:app.js L1907–1923；本轮本地库 transactions total=0，行点击未采集 | 已实现｜运行时未验证（需有数据） |
| I-HOME-08 | 首页 | 账单行内容（分类/备注/商户/时间/金额） | app.js L1911–1920 | 有交易数据 | N/A（不适用） | N/A（不适用） | 显示分类或转账账户、备注/商户、金额 | 未验证 | N/A（不适用） | 详情 | C:app.js L1911–1920 | 已实现｜缺账户/支付渠道标签｜R0-E-21 |
| I-HOME-09 | 首页 | 桌面「＋ 记一笔」按钮 | app.js L1942–1943（.only-desktop） | 已选账本 | N/A（不适用） | 点击 → openTxForm(bookId,()=>route()) | 打开 #formDlg | 未验证 | 提交后 route() 重绘 | 本页 | C:app.js L1942–1943；S:add-transaction/1280x800/normal.png（桌面弹窗） | 已实现｜截图已见 |
| I-HOME-10 | 首页 | 中央 FAB「＋」 | index.html L33 / app.js L1868 | 有账本（showNav L122 控制显隐） | N/A（不适用） | 点击 → openTxForm(bookId,()=>route()) | 打开 #formDlg | 未验证 | 提交后重绘 | 本页 | C:index.html L33, app.js L1868；S:add-transaction/390x844/normal.png（FAB 点击后弹窗） | 已实现｜截图已见｜R0-E-01 |
| I-HOME-11 | 首页 | 底部 Tab「首页」 | index.html L31 / app.js L1862–1866 | 已选账本 | N/A（不适用） | 点击 .tab[data-path=""] | hash=#b/{id} | N/A（不适用） | N/A（不适用） | #b/{id} | C:index.html L31；S:home/390x844/empty.png（底栏 首页 高亮） | 已实现｜截图已见｜R0-E-01 |
| I-HOME-12 | 首页 | 底部 Tab「账单」 | index.html L32 / app.js L1865 | 已选账本 | N/A（不适用） | 点击 | hash=#b/{id}/txns | N/A（不适用） | N/A（不适用） | 交易总表 | C:index.html L32；S:home/390x844/empty.png | 已实现｜截图已见｜R0-E-01 |
| I-HOME-13 | 首页 | 底部 Tab「报表」 | index.html L34 / app.js L1865 | 已选账本 | N/A（不适用） | 点击 | hash=#b/{id}/report | N/A（不适用） | N/A（不适用） | 报表 | C:index.html L34；S:home/390x844/empty.png | 已实现｜截图已见 |
| I-HOME-14 | 首页 | 底部 Tab「我的」 | index.html L35 / app.js L1865 | 已选账本 | N/A（不适用） | 点击 | hash=#b/{id}/mine | N/A（不适用） | N/A（不适用） | 我的 | C:index.html L35；S:home/390x844/empty.png | 已实现｜截图已见 |
| I-HOME-15 | 首页 | 账单加载态（骨架屏） | app.js L1874–1876（sk, aria-busy=true, 4 条） | 进入首页 / 刷新 | N/A（不适用） | N/A（不适用） | 骨架占位 | 见 I-HOME-17 | N/A（不适用） | 本页 | C:app.js L1874–1876；S:runtime/01-pending-390x844.png、04-pending-1280x800.png（隔离拦截目标请求后实测骨架占位，见 §7.5） | 已实现｜pending 态已采集（隔离复验）｜R0-E-02 |
| I-HOME-16 | 首页 | 账单空态 | app.js L1882–1887 | 无交易 | N/A（不适用） | N/A（不适用） | 「还没有账单——点下方「＋ 记一笔」开始记录。」 | 未验证 | N/A（不适用） | 本页 | C:app.js L1882–1887；S:home/390x844/empty.png（可见空态文案） | 已实现｜截图已见 |
| I-HOME-17 | 首页 | 账单错误态 | app.js L1878–1879（catch 仅 toast 并 return） | API 失败 | N/A（不适用） | GET /transactions?page=1&count=60 失败 | N/A（不适用） | toast(e.message,"err")；代码未清除骨架屏 | 无 | 本页 | C:app.js L1878–1879；S:home/390x844/error.png（旧轮泛化拦截 `/api/v1/*`：主体空白 + `Failed to fetch` toast，因依赖请求同时被打断）；S:runtime/02-rejected-network-390x844.png、03-rejected-503-390x844.png（本轮仅拦截目标请求：`.skel`=4 仍在 + toast，见 `dom-snapshots.json`） | **本机隔离复验已完成 / 缺陷尚未关闭（见 §7.5 标准口径）｜R0-E-35**（本轮隔离实测见 §7.5；单次本机实测不关闭缺陷，须 Owner/CI 独立复跑） |
| I-HOME-18 | 首页 | 按日分组标题 + 日小计 | app.js L1897–1905（gt / daySum） | 有交易数据 | 仅基于首批 60 条（L1878 count=60） | 收入加 / 支出减 | 显示「今天/昨天/前天/N 天前」+ 日合计 | 未验证 | N/A（不适用） | 本页 | C:app.js L1897–1905；有数据态未采集 | 已实现｜分页边界缺陷｜R0-E-34 |

#### 3.1.4 记一笔弹窗 `#formDlg`

> `openTxForm` app.js L530–580；`index.html` L38–59；截图 `add-transaction/{360x800,390x844,430x932,1280x800}/normal.png`。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-ADD-01 | 记一笔 | 打开弹窗 | app.js L530–541 | 有账本/账户/分类 | fType=2、fillSelect 三下拉、fDate=今天、清空 fNote/fAmount、syncFormFields()、showModal() | 由 FAB/桌面按钮触发 | 显示 #formDlg | 未验证 | N/A（不适用） | 本页（弹层） | C:app.js L530–541；S:add-transaction/390x844/normal.png | 已实现｜截图已见 |
| I-ADD-02 | 记一笔 | 类型选择 `#fType` | index.html L42 / app.js L532,L547 | 弹窗打开 | 值 2=支出 / 1=收入 / 3=转账；**普通 `<select>`** | change → syncFormFields() L542–546 | t=3 显 #fToRow 隐 #fCatRow，否则相反 | 未验证 | 见 I-ADD-05 | 弹层 | C:index.html L42, app.js L542–547；S:add-transaction/390x844/normal.png（可见「类型 支出」下拉） | 已实现**但形态不符**（非分段控件）｜R0-E-05 / R0-E-29 |
| I-ADD-03 | 记一笔 | 金额输入 `#fAmount` | index.html L48 / app.js L538,L551–552 | 弹窗打开 | str2cents 正则 `^(\d+)(?:\.(\d{1,2}))?$`；cents===null 或 <=0 → toast「金额格式不合法（最多两位小数）」L552 | 键入 | 显示文本 | toast err，不关闭弹窗 | N/A（不适用） | 弹层 | C:index.html L48, app.js L551–552；S:add-transaction/390x844/normal.png（可见 0.00 占位） | 已实现｜校验失败态未采集 |
| I-ADD-04 | 记一笔 | 资金账户 `#fAccount` | index.html L49 / app.js L533,L557 | 弹窗打开 | 非转账时 !body.accountId → toast「请选择账户」L568 | 选择 | 选中账户 | toast err | N/A（不适用） | 弹层 | C:index.html L49, app.js L568；S:add-transaction/390x844/normal.png（可见「账户 现金」） | 已实现｜校验失败态未采集 |
| I-ADD-05 | 记一笔 | 转入账户 `#fTo`（#fToRow hidden） | index.html L50 / app.js L534,L544,L566–567 | 类型=转账 | !toAccountId 或 toAccountId===accountId → toast「转账需选择不同的两个账户」L567 | 选择 | 选中账户 | toast err | N/A（不适用） | 弹层 | C:index.html L50, app.js L566–567；转账态截图未采集 | 已实现｜运行时未验证 |
| I-ADD-06 | 记一笔 | 分类 `#fCategory` | index.html L51 / app.js L535,L545,L559 | 非转账 | 转账时 categoryId=0 | 选择 | 选中分类 | 未验证 | 见 I-ADD-02 | 弹层 | C:index.html L51；S:add-transaction/390x844/normal.png（可见「分类 工资」） | 已实现｜截图已见 |
| I-ADD-07 | 记一笔 | 日期 `#fDate` | index.html L52 / app.js L536,L560 | 弹窗打开 | type=date required；dateStartUnix(...,false) | 选择 | 选中日期 | 浏览器 required | N/A（不适用） | 弹层 | C:index.html L52；S:add-transaction/390x844/normal.png（可见 2026/11/04） | 已实现｜截图已见 |
| I-ADD-08 | 记一笔 | 备注 `#fNote` | index.html L53 / app.js L537,L562 | 弹窗打开 | maxlength=255 → comment | 键入 | 显示文本 | 未验证 | N/A（不适用） | 弹层 | C:index.html L53；S:add-transaction/390x844/normal.png（可见「可选」占位） | 已实现｜截图已见 |
| I-ADD-09 | 记一笔 | 支付渠道选择器 | —（grep 无命中） | 弹窗打开 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：记一笔 body L554–564 无 paymentChannel；`paymentChannel` 全文仅 7 处命中（L425/449/620/929/1914/1916/2004），均为展示/导出/编辑只读继承；S:add-transaction/390x844/normal.png 可见无该字段 | **缺失**｜R0-E-23 |
| I-ADD-10 | 记一笔 | 商户输入 | —（grep 无命中） | 弹窗打开 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：body 无 merchant；`merchant` 全文仅展示/导出命中；后端 service/tx.go CreateRequest 亦无 Merchant 字段（R0-E-37 端到端缺口） | **缺失**｜R0-E-37 |
| I-ADD-11 | 记一笔 | 提交保存 `#fSubmit` | index.html L56 / app.js L549–579 | 校验通过 | btnPending「保存中…」L570 | POST /books/:id/transactions（body L554–564：clientUuid/txType/accountId/amount/categoryId/txTime/timezoneOffset/comment/source=0）L572 | toast「已记账」L573 + 关闭弹窗 L574–575 + onDone() L576 | toast(err.message,"err") L578 | onDone=重绘列表/首页 | 首页/账单流 | C:app.js L549–579；提交态未采集 | 已实现｜运行时未验证 |
| I-ADD-12 | 记一笔 | 取消 `#fCancel` | index.html L55 / app.js L548 | 弹窗打开 | N/A（不适用） | 点击 | dlg.close() + 加 hidden，不提交 | N/A（不适用） | 无 | 本页 | C:index.html L55, app.js L548；S:add-transaction/390x844/normal.png（可见「取消」） | 已实现｜运行时未验证 |
| I-ADD-13 | 记一笔 | 弹窗关闭（Esc / ×） | —（无显式绑定） | 弹窗打开 | N/A（不适用） | 按 Esc / 点原生关闭 | 应关闭 | 待确认 | N/A（不适用） | 本页 | 缺失：仅 #fCancel 自定义关闭，无 cancel/close 事件绑定，依赖 `<dialog>` 原生行为 | **待验证**｜运行时未验证 |
| I-ADD-14 | 记一笔 | 重复提交防护 | app.js L56 + L570–577 finally | 提交中 | 按钮禁用 | N/A（不适用） | 未验证 | 失败也恢复按钮 | N/A（不适用） | 弹层 | C:app.js L56,L570–577；运行时未验证 | 已实现（代码层）｜运行时未验证 |
| I-ADD-15 | 记一笔 | 表单校验失败汇总 | app.js L552 / L567 / L568 | 金额/转账双账户/账户三项 | N/A（不适用） | 提交触发 | 未验证 | toast err、弹窗不关闭、输入保留 | N/A（不适用） | 弹层 | C:app.js L552,L567,L568；校验失败态未采集 | 已实现｜运行时未验证｜R0-E-05 |

#### 3.1.5 交易详情 `#b/:id/tx/:txId`

> `renderTxDetail` app.js L1974–2026；本轮本地库 transactions total=0，**该页无真实数据可截图**（阻塞项，见 §7.5）。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-TXDET-01 | 交易详情 | 详情加载 | app.js L1982；后端锚点 router.go L194 | 进入 hash | N/A（不适用） | GET /transactions/:txId | 渲染详情 | toast(err.message,"err") + hash=#b/{id}/txns L1983 | N/A（不适用） | 账单页（失败时） | C:app.js L1982–1983 + C:router.go L194（前后端双向锚点）；截图未采集（无数据） | 已实现｜运行时未验证 |
| I-TXDET-02 | 交易详情 | 返回 | app.js L1979（wireBottomNav(bookId,"/txns")） | 详情页 | N/A（不适用） | 底部 Tab / 浏览器返回 | 未验证 | 未验证 | N/A（不适用） | #b/{id}/txns | C:app.js L1979 | 已实现｜运行时未验证 |
| I-TXDET-03 | 交易详情 | 完整字段展示 | app.js L1995–2006（addRow） | tx 就绪 | N/A（不适用） | N/A（不适用） | 账户/分类/日期/备注（转账显示 A→B） | 未验证 | N/A（不适用） | 本页 | C:app.js L1995–2006；截图未采集（无数据） | **部分具备**｜R0-E-03 / R0-E-22 |
| I-TXDET-04 | 交易详情 | paymentChannel 展示 | —（无 addRow 行） | tx 就绪 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：addRow 无渠道行 | **缺失**｜R0-E-22 / R0-E-23 |
| I-TXDET-05 | 交易详情 | 商户与备注显示逻辑 | app.js L2004 | tx 就绪 | `tx.note \|\| (tx.merchant?"商户："+tx.merchant:"—")` | N/A（不适用） | 备注非空时显示备注 | 备注非空时 merchant 被吞，不显示 | N/A（不适用） | 本页 | C:app.js L2004 | **已实现但缺陷**｜R0-E-22 |
| I-TXDET-06 | 交易详情 | 编辑按钮 | app.js L2008–2009 | 详情页 | N/A（不适用） | 点击 → openTxEditForm(bookId,tx,()=>renderTxDetail(...)) | 打开 #editDlg | 未验证 | 保存后重绘详情 | 详情 | C:app.js L2008–2009；截图未采集 | 已实现｜运行时未验证｜R0-E-03 |
| I-TXDET-07 | 交易详情 | 编辑表单打开 / 类型只读 | app.js L583–608（openTxEditForm） | 详情页点编辑 | eType 只读文本 L585；转账显 eToRow/隐 eCatRow L596–606 | N/A（不适用） | 回填 eAmount/eDate/eNote | 未验证 | N/A（不适用） | 弹层 | C:app.js L583–608 | 已实现｜运行时未验证｜R0-E-23 |
| I-TXDET-08 | 交易详情 | 编辑保存 | app.js L610–641（#txEditForm onsubmit） | 弹窗打开 | str2cents L612–613；转账双账户 L628；非转账 !accountId\|\|!categoryId → toast「请选择账户与分类」L629 | PUT /transactions/:txId（body L614–625 含 paymentChannel: tx.paymentChannel\|\|0 只读继承、baseVersion）L633 | toast「已保存修改」+ 关弹窗 + onDone() L634–637 | toast(err.message + 并发提示,"err") L639 | onDone=重绘详情 | 详情 | C:app.js L610–641；运行时未验证 | 已实现｜运行时未验证｜R0-E-23 |
| I-TXDET-09 | 交易详情 | 编辑失败 / 409 并发 | app.js L639 | PUT 409 | N/A（不适用） | N/A（不适用） | 未验证 | toast 并发提示 | N/A（不适用） | 弹层保留 | C:app.js L639；运行时未验证 | 已实现｜运行时未验证 |
| I-TXDET-10 | 交易详情 | 编辑取消 `#eCancel` | index.html L71 / app.js L609 | 弹窗打开 | N/A（不适用） | 点击 | 关闭弹窗 | 未验证 | N/A（不适用） | 详情 | C:index.html L71, app.js L609 | 已实现｜运行时未验证 |
| I-TXDET-11 | 交易详情 | 删除按钮 | app.js L2010–2023 | 详情页 | N/A（不适用） | 点击 → 二次确认 + DELETE | 未验证 | 未验证 | N/A（不适用） | 未验证 | C:app.js L2010–2023；截图未采集 | 已实现｜运行时未验证｜R0-E-03 |
| I-TXDET-12 | 交易详情 | 删除二次确认 | app.js L2012–2016（confirmDialog） | 点删除 | 文案「删除后将无法恢复，账户余额会相应调整。」 | 确认/取消 | 确认后 DELETE | 未验证 | N/A（不适用） | 未验证 | C:app.js L2012–2016 | **已实现但文案与软删除/回收站矛盾**｜R0-E-24 |
| I-TXDET-13 | 交易详情 | 删除取消 | app.js L2017（if(!ok) return） | 确认框打开 | N/A（不适用） | 点「取消」 | 不删除 | 未验证 | 无 | 详情 | C:app.js L2017 | 已实现｜运行时未验证 |
| I-TXDET-14 | 交易详情 | 删除提交 | app.js L2020 | 已确认 | btnPending L2018 | DELETE /transactions/:id?clientUuid=uuid() | toast「已删除」+ hash=#b/{id}/txns L2021 | toast(e.message,"err") L2022 | N/A（不适用） | 账单页 | C:app.js L2020–2022；运行时未验证 | 已实现｜运行时未验证｜R0-E-24 |

#### 3.1.6 交易总表 `#b/:id/txns`

> `renderTxns` app.js L337–527；截图 `transactions/{390x844,1280x800}/empty.png`。首页筛选能力实际落在此页（`.filters` L352–366）。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-TXNS-01 | 交易总表 | 关键词搜索 | app.js L355（kw input） | N/A（不适用） | placeholder=关键词搜索 | 键入 + 查询 | state.filter.keyword 生效 | 未验证 | N/A（不适用） | 本页 | C:app.js L355；S:transactions/390x844/empty.png | 已实现｜截图已见 |
| I-TXNS-02 | 交易总表 | 类型筛选 | app.js L356–358（typeSel） | N/A（不适用） | 1/2/3 + 全部类型 | 选择 | 过滤 | 未验证 | N/A（不适用） | 本页 | C:app.js L356–358 | 已实现｜截图已见 |
| I-TXNS-03 | 交易总表 | 账户筛选 | app.js L359（accSel） | N/A（不适用） | 全部账户 + 账户列表 | 选择 | 过滤 | 未验证 | N/A（不适用） | 本页 | C:app.js L359 | 已实现｜截图已见 |
| I-TXNS-04 | 交易总表 | 分类筛选 | app.js L360（catSel） | N/A（不适用） | 全部分类 + 分类列表 | 选择 | 过滤 | 未验证 | N/A（不适用） | 本页 | C:app.js L360 | 已实现｜截图已见 |
| I-TXNS-05 | 交易总表 | 开始日期 | app.js L361（dFrom type=date） | N/A（不适用） | dateStartUnix(minDate,false) L396 | 选择 | 过滤 | 未验证 | N/A（不适用） | 本页 | C:app.js L361 | 已实现｜截图已见 |
| I-TXNS-06 | 交易总表 | 结束日期 | app.js L362（dTo type=date） | N/A（不适用） | dateStartUnix(maxDate,true) L396 | 选择 | 过滤 | 未验证 | N/A（不适用） | 本页 | C:app.js L362 | 已实现｜截图已见 |
| I-TXNS-07 | 交易总表 | 查询按钮 | app.js L363 / L523（applyFilters） | 筛选条件已填 | 组装 state.filter L520 | 点击 → GET /transactions?page&count&keyword&type&accountId&categoryId&minTime&maxTime L399 | renderTable(data) L401；页码回 1、scrollTo 顶部 L402 | load().catch → toast(e.message,"err") L521 | N/A（不适用） | 本页 | C:app.js L363,L519–522；运行时未验证 | 已实现｜运行时未验证 |
| I-TXNS-08 | 交易总表 | 重置按钮 | app.js L364 / L524 | N/A（不适用） | N/A（不适用） | 点击 → applyFilters() | 清空 6 个控件并回到全量第一页 | 见 I-TXNS-07 | N/A（不适用） | 本页 | C:app.js L364,L524 | 已实现｜运行时未验证 |
| I-TXNS-09 | 交易总表 | 回车提交 | app.js L516–518（6 控件 keydown Enter） | 筛选控件聚焦 | N/A（不适用） | 按 Enter | applyFilters() | 未验证 | N/A（不适用） | 本页 | C:app.js L516–518 | 已实现｜运行时未验证 |
| I-TXNS-10 | 交易总表 | 全选本页 | app.js L370 / L483–490（allChk） | 有行 | N/A（不适用） | 勾选 | 勾选 tbody 全部 checkbox；state.sel 增删 | 未验证 | N/A（不适用） | 本页 | C:app.js L370,L483–490 | 已实现｜运行时未验证 |
| I-TXNS-11 | 交易总表 | 行勾选 | app.js L433–437（chk change） | 每行 | N/A（不适用） | 勾选/取消 | state.sel.add/delete(tx.id)；refreshSel() 更新按钮态 L378–381 | 未验证 | N/A（不适用） | 本页 | C:app.js L433–437 | 已实现｜运行时未验证 |
| I-TXNS-12 | 交易总表 | 删除选中 | app.js L372 / L491–514 | state.sel.size>0 L492 | confirmDialog 文案「将删除选中的 N 条交易，删除后可在回收站恢复。」L494–499 | 逐条 DELETE /transactions/:id?clientUuid=uuid() L506 | toast「删除完成：成功 X，失败 Y」L510 + page=1 + await load() L511–512 | 单条失败计入 failN | 重绘列表 | 本页 | C:app.js L491–514；运行时未验证 | 已实现｜文案与详情页矛盾（R0-E-24） |
| I-TXNS-13 | 交易总表 | 删除选中取消 | app.js L500（if(!yes) return） | 确认框 | N/A（不适用） | 取消 | 不删除 | 未验证 | 无 | 本页 | C:app.js L500 | 已实现｜运行时未验证 |
| I-TXNS-14 | 交易总表 | ＋ 记一笔 | app.js L373 / L515 | N/A（不适用） | N/A（不适用） | 点击 → openTxForm(bookId,load) | 打开弹窗；提交后 load() | 未验证 | N/A（不适用） | 本页 | C:app.js L373,L515 | 已实现｜运行时未验证 |
| I-TXNS-15 | 交易总表 | 行编辑 | app.js L454–455（editBtn） | 每行 | N/A（不适用） | 点击 → openTxEditForm(bookId,tx,load) | 打开编辑弹窗；保存后 load() | 未验证 | N/A（不适用） | 本页 | C:app.js L454–455 | 已实现｜运行时未验证 |
| I-TXNS-16 | 交易总表 | 上一页 / 下一页 | app.js L472–478（prev/next） | data.page>1 / page*count<total | N/A（不适用） | 点击 | state.filter.page±1 + load() | 未验证 | N/A（不适用） | 本页 | C:app.js L472–478 | 已实现｜运行时未验证 |
| I-TXNS-17 | 交易总表 | 骨架屏 | app.js L406–416 / L388（renderSkeleton, 5 条, aria-busy=true） | 加载前 | N/A（不适用） | N/A（不适用） | 骨架占位 | 未验证 | N/A（不适用） | 本页 | C:app.js L406–416；加载态未采集 | 已实现｜加载态未采集｜R0-E-02 |
| I-TXNS-18 | 交易总表 | 空态 | app.js L460–468 | 无结果 | N/A（不适用） | N/A（不适用） | 「没有符合条件的交易…」 | 未验证 | N/A（不适用） | 本页 | C:app.js L460–468；S:transactions/390x844/empty.png | 已实现｜截图已见 |
| I-TXNS-19 | 交易总表 | 加载失败 | app.js L526（try/catch toast） | API 失败 | N/A（不适用） | N/A（不适用） | 未验证 | 仅 toast(e.message,"err") | N/A（不适用） | 本页 | C:app.js L526；错误态未采集 | 已实现｜运行时未验证｜R0-E-12 |

#### 3.1.7 账户 `#b/:id/accounts`

> `renderAccounts` L647–712 / `openAcctForm` L715–748 / `#acctDlg` index.html L134–148；截图 `accounts/{360x800,390x844,430x932,1280x800}/normal.png`。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-ACCT-01 | 账户 | ＋ 新增账户 | app.js L657–658（addBtn） | 对账页 | 见 I-ACCT-08 | 点击 → openAcctForm(bookId,null) | 打开 #acctDlg | 见 I-ACCT-12 | 见 I-ACCT-13 | 弹层 | C:app.js L657–658；S:accounts/390x844/normal.png | 已实现｜截图已见 |
| I-ACCT-02 | 账户 | 账户行（名称/类型/余额） | app.js L668–673 | loadBookMeta L649 | N/A（不适用） | N/A（不适用） | 显示 ACC_CAT_NAME、cents2str 余额、正负色 L672 | 未验证 | N/A（不适用） | 本页 | C:app.js L668–673；S:accounts/390x844/normal.png | 已实现｜截图已见 |
| I-ACCT-03 | 账户 | 编辑按钮 | app.js L676–677（edit） | 每行 | N/A（不适用） | 点击 → openAcctForm(bookId,a) | 打开 #acctDlg（回填） | 未验证 | N/A（不适用） | 弹层 | C:app.js L676–677 | 已实现｜运行时未验证 |
| I-ACCT-04 | 账户 | 删除按钮 | app.js L678–689（del） | 每行 | confirmDialog L680–684 | DELETE /books/:id/accounts/:id L687 | toast「账户已删除」+ await renderAccounts(bookId) | toast(e.message,"err") L688（后端 409 引用保护） | 重绘本页余额 | 本页 | C:app.js L678–689；运行时未验证 | 已实现｜运行时未验证 |
| I-ACCT-05 | 账户 | 删除取消 | app.js L685（if(!ok) return） | 确认框 | N/A（不适用） | 取消 | 不删除 | N/A（不适用） | 无 | 本页 | C:app.js L685 | 已实现｜运行时未验证 |
| I-ACCT-06 | 账户 | 删除引用保护 | app.js L688 + 后端 409 | 账户被交易引用 | N/A（不适用） | 确认删除 | 未验证 | toast 后端 ErrAccountInUse → 409 | N/A（不适用） | 本页 | C:app.js L688；common.go mapSvcErr（见 §5.3） | 已实现｜运行时未验证 |
| I-ACCT-07 | 账户 | 期初余额输入 | —（grep 无命中） | 新增账户弹窗 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：app.js `initialBalance\|期初\|opening\|initBalance\|初始` No matches；新增 body L737 仅 name/category/currency；后端 AccountService.Create 固定 Balance:0（account.go L21） | **缺失**｜R0-E-06 / R0-E-30 |
| I-ACCT-08 | 账户 | 账户名称 `#acctName` | index.html L137 / app.js L719,L731–732 | 弹窗 | name.trim() 空 → #acctErr「请输入账户名称」L732 | 键入 | 显示文本 | err-msg | N/A（不适用） | 弹层 | C:index.html L137, app.js L731–732 | 已实现｜运行时未验证 |
| I-ACCT-09 | 账户 | 账户类型 `#acctCat` | index.html L139 / app.js L720–722 | 弹窗 | 编辑时 disabled=editing L722（后端仅改名称） | 选择 | 选中类型 | 未验证 | N/A（不适用） | 弹层 | C:index.html L139, app.js L720–722 | 已实现｜运行时未验证 |
| I-ACCT-10 | 账户 | 币种 `#acctCur` | index.html L141 / app.js L723–724 | 弹窗 | 默认 CNY；编辑时 disabled L724 | 键入 | 显示文本 | 未验证 | N/A（不适用） | 弹层 | C:index.html L141, app.js L723–724 | 已实现｜运行时未验证 |
| I-ACCT-11 | 账户 | 取消 `#acctCancel` | index.html L144 / app.js L728 | 弹窗 | N/A（不适用） | 点击 | 关闭 | 未验证 | 无 | 本页 | C:index.html L144, app.js L728 | 已实现｜运行时未验证 |
| I-ACCT-12 | 账户 | 保存 `#acctSubmit` | index.html L145 / app.js L729–747 | 名称非空 | btnPending L739 | 编辑 PUT /accounts/:id {name} L741；新增 POST /accounts {name,category,currency} L742 | toast「账户已创建/已更新」L743 + 关弹窗 L744 + await renderAccounts L745 | #acctErr.textContent=err.message L746 | 重绘本页余额 | 本页 | C:app.js L729–747；运行时未验证 | 已实现｜运行时未验证 |
| I-ACCT-13 | 账户 | 余额刷新 | app.js L687 / L745 | 增删后 | N/A（不适用） | renderAccounts(bookId) | loadBookMeta 重拉，余额列表更新 | 未验证 | N/A（不适用） | 本页 | C:app.js L687,L745 | 已实现｜运行时未验证 |
| I-ACCT-14 | 账户 | 本页净资产摘要 | —（在 renderInvest L1255–1276） | 对账页 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：净资产四卡在投资页 | **缺失**｜R0-E-28 |
| I-ACCT-15 | 账户 | 投资账户行点击 → 详情 | app.js L693–700 | category===5 | 排除 .row-actions L697 | 点击 → #b/{id}/account/{a.id} L698 | 进入投资账户详情 | 未验证 | N/A（不适用） | #b/:id/account/:accId | C:app.js L693–700；S:account-detail/390x844/normal.png | 已实现｜截图已见 |

#### 3.1.8 分类 `#b/:id/categories`

> `renderCategories` L2069–2135 / `openCatForm` L2029–2055 / `deleteCategory` L2056–2068；截图 `categories/{390x844,1280x800}/normal.png`。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-CAT-01 | 分类 | ＋ 新增大类 | app.js L2084–2085（addTop） | 分类页 | N/A（不适用） | 点击 → openCatForm(bookId,null) | 打开 #catDlg | 未验证 | N/A（不适用） | 弹层 | C:app.js L2084–2085；S:categories/390x844/normal.png | 已实现｜截图已见｜R0-E-04 |
| I-CAT-02 | 分类 | ＋ 小类 | app.js L2108–2109（addSub） | 每个大类行 | 传 {parentId:top.id,catType:top.catType} | 点击 → openCatForm | 打开 #catDlg（预置上级） | 未验证 | N/A（不适用） | 弹层 | C:app.js L2108–2109 | 已实现｜运行时未验证 |
| I-CAT-03 | 分类 | 大类编辑 | app.js L2110–2111 | 每行 | N/A（不适用） | 点击 → openCatForm(bookId,top) | 打开弹窗回填 | 未验证 | N/A（不适用） | 弹层 | C:app.js L2110–2111 | 已实现｜运行时未验证 |
| I-CAT-04 | 分类 | 子类编辑 | app.js L2123–2124（ke） | 每个子类 | N/A（不适用） | 点击 → openCatForm(bookId,kid) | 打开弹窗回填 | 未验证 | N/A（不适用） | 弹层 | C:app.js L2123–2124 | 已实现｜运行时未验证 |
| I-CAT-05 | 分类 | 大类删除 | app.js L2112–2113 | 每行 | N/A（不适用） | 点击 → deleteCategory(bookId,top) | 进入 I-CAT-07 引用确认 | 未验证 | N/A（不适用） | 本页 | C:app.js L2112–2113 | 已实现｜运行时未验证 |
| I-CAT-06 | 分类 | 子类删除 | app.js L2125–2126（kd） | 每个子类 | N/A（不适用） | 点击 → deleteCategory(bookId,kid) | 进入 I-CAT-07 引用确认 | 未验证 | N/A（不适用） | 本页 | C:app.js L2125–2126 | 已实现｜运行时未验证 |
| I-CAT-07 | 分类 | 引用保护确认 | app.js L2056–2068（deleteCategory） | 删除前 | 先 GET /transactions?categoryId=&count=1 取 n L2058；confirmDialog 文案「删除「…」后，n 笔账单将归入「其他」分类，且不可恢复。」L2061 | 确认 → DELETE /categories/:id L2066 | toast「分类已删除」+ await renderCategories(bookId) | toast(e.message,"err") L2067 | 重绘分类树 | 本页 | C:app.js L2056–2068；运行时未验证 | 已实现｜运行时未验证｜R0-E-04 |
| I-CAT-08 | 分类 | 删除取消 | app.js L2064（if(!ok) return） | 确认框 | N/A（不适用） | 取消 | 不删除 | 未验证 | 无 | 本页 | C:app.js L2064 | 已实现｜运行时未验证 |
| I-CAT-09 | 分类 | 名称 `#catName` | index.html L122 / app.js L2032,L2043–2044 | 弹窗 | name.trim() 空 → #catErr「请输入分类名称」L2044 | 键入 | 显示文本 | err-msg | N/A（不适用） | 弹层 | C:index.html L122, app.js L2043–2044 | 已实现｜运行时未验证 |
| I-CAT-10 | 分类 | 类型 `#catType` | index.html L124 / app.js L2033 | 弹窗 | 支出(2)/收入(1) | 选择 | 选中类型 | 未验证 | N/A（不适用） | 弹层 | C:index.html L124, app.js L2033 | 已实现｜运行时未验证 |
| I-CAT-11 | 分类 | 上级大类 `#catParent` | index.html L126 / app.js L2034–2036 | 弹窗 | fillSelect(...,"无（作为大类）")，顶级填 0 | 选择 | 选中上级 | 未验证 | N/A（不适用） | 弹层 | C:index.html L126, app.js L2034–2036 | 已实现｜运行时未验证 |
| I-CAT-12 | 分类 | 取消 `#catCancel` | index.html L129 / app.js L2040 | 弹窗 | N/A（不适用） | 点击 | 关闭 | 未验证 | 无 | 本页 | C:index.html L129, app.js L2040 | 已实现｜运行时未验证 |
| I-CAT-13 | 分类 | 保存 `#catSubmit` | index.html L130 / app.js L2041–2054 | 名称非空 | btnPending L2046 | 编辑 PUT /categories/:id L2048；新增 POST /categories L2049，body {name,catType,parentId} L2045 | toast「分类已添加/已更新」L2050 + 关弹窗 L2051 + await renderCategories L2052 | #catErr.textContent=err.message L2053 | 重绘 | 本页 | C:app.js L2041–2054；运行时未验证 | 已实现｜运行时未验证 |
| I-CAT-14 | 分类 | 树展开 / 收起 | —（无事件） | 分类页 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：renderGroup L2091–2131 一次性平铺大类+子类，无折叠/展开按钮或事件 | **缺失**｜R0-E-04 |
| I-CAT-15 | 分类 | 空态 | app.js L2097–2100（cat-empty） | 无分类 | N/A（不适用） | N/A（不适用） | 「暂无，点击上方「新增大类」添加。」 | 未验证 | N/A（不适用） | 本页 | C:app.js L2097–2100；本轮有默认分类，空态未采集 | 已实现｜空态未采集 |

#### 3.1.9 成员 `#b/:id/members`

> `renderMembers` app.js L1636–1765；截图 `members/{390x844,1280x800}/normal.png`。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-MEM-01 | 成员 | 邀请成员按钮 | app.js L1647–1677（invBtn） | 成员页 | N/A（不适用） | 点击 → 打开 openInvestDialog | 显示邀请弹窗 | 未验证 | N/A（不适用） | 弹层 | C:app.js L1647–1677；S:members/390x844/normal.png | 已实现｜截图已见 |
| I-MEM-02 | 成员 | 角色下拉 | app.js L1649–1652（roleSel） | 邀请弹窗 | 3=普通成员 / 2=管理员 | 选择 | 选中角色 | 未验证 | N/A（不适用） | 弹层 | C:app.js L1649–1652 | 已实现｜运行时未验证 |
| I-MEM-03 | 成员 | 绑定用户名 | app.js L1653（bound input） | 邀请弹窗 | placeholder「可选：限定用户名（空 = 任何人凭码加入）」 | 键入 | 显示文本 | 未验证 | N/A（不适用） | 弹层 | C:app.js L1653 | 已实现｜运行时未验证 |
| I-MEM-04 | 成员 | 邀请提交 | app.js L1658–1659 | 邀请弹窗 | {role,boundUsername} | POST /books/:id/members/invite | 返回 {code,role,expiresAt}（**无 inviteId**） | 未验证 | N/A（不适用） | 弹层 | C:app.js L1658–1659 | 已实现｜响应缺 inviteId｜R0-E-26 |
| I-MEM-05 | 成员 | 邀请码展示 | app.js L1660–1666 | 生成成功 | 复用 investDlg；#iSubmit 隐藏 L1674 | N/A（不适用） | 弹窗显示 code + 引导文案 L1667 | 未验证 | N/A（不适用） | 弹层 | C:app.js L1660–1667 | 已实现｜运行时未验证 |
| I-MEM-06 | 成员 | 复制邀请码 | app.js L1668–1672（cp） | 邀请码已展示 | navigator.clipboard.writeText | 点击 | toast「邀请码已复制」L1670 | toast「复制失败，请手动选中复制」L1671 | N/A（不适用） | 弹层 | C:app.js L1668–1672 | 已实现｜运行时未验证 |
| I-MEM-07 | 成员 | 邀请撤销 UI | —（grep 无命中） | 成员页 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：app.js `revoke` 仅命中 L1593/1596–1606（Token 撤销）、L1085（URL.revokeObjectURL）；无邀请撤销按钮、无 /invite/:inviteId/revoke 调用 | **缺失**｜R0-E-26 |
| I-MEM-08 | 成员 | 成员列表 | app.js L1707 / L1683–1684 | 成员页 | GET /members + GET /users/me | N/A（不适用） | 表头 成员姓名/角色/可见性/操作 | 未验证 | N/A（不适用） | 本页 | C:app.js L1683–1684,L1707；S:members/390x844/normal.png | 已实现｜截图已见｜R0-E-11 |
| I-MEM-09 | 成员 | 角色徽标 | app.js L1715–1717（badge） | 每行 | 1=所有者/2=管理员/3=普通成员 | N/A（不适用） | 显示徽标 | 未验证 | N/A（不适用） | 本页 | C:app.js L1715–1717；S:members/390x844/normal.png | 已实现｜视觉待验收｜R0-E-11 |
| I-MEM-10 | 成员 | 可见性徽标（只读） | app.js L1719–1720（vbadge） | 每行 | 只读 mb.canViewOthers | N/A（不适用） | 显示「已开启/已关闭账本可见」 | 未验证 | N/A（不适用） | 本页 | C:app.js L1719–1720；S:members/390x844/normal.png | **仅只读徽标，非开关；字段粒度为成员级而非账本级**｜R0-E-19 / R0-E-27 |
| I-MEM-11 | 成员 | 账本可见性开关 | —（grep 无命中） | 成员页 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：无 checkbox/toggle、无 PUT /books/:id/visibility 调用；`canViewOthers` 全文仅 L1719–1720；权威字段为 books.members_can_view_others | **缺失**｜R0-E-19 / R0-E-27 |
| I-MEM-12 | 成员 | 设为管理员 | app.js L1724–1732（bUp） | role===3 | N/A（不适用） | PUT /members/:uid {role:2} L1727 | toast「已设为管理员」+ renderMembers(bookId) | toast(err.message,"err") L1730 | 重绘本页 | 本页 | C:app.js L1724–1732 | 已实现｜运行时未验证 |
| I-MEM-13 | 成员 | 降级为成员 | app.js L1734–1742（bDown） | role===2 | N/A（不适用） | PUT /members/:uid {role:3} L1737 | toast「已降级为成员」+ 重绘 | toast(err.message,"err") L1740 | 重绘本页 | 本页 | C:app.js L1734–1742 | 已实现｜运行时未验证 |
| I-MEM-14 | 成员 | 移除成员 | app.js L1745–1757（bDel） | role!==1 | confirmDialog L1747–1749 | DELETE /members/:uid L1752 | toast「成员已移除」+ renderMembers(bookId) | toast(err.message,"err") L1755 | 重绘本页 | 本页 | C:app.js L1745–1757 | 已实现｜运行时未验证 |
| I-MEM-15 | 成员 | 移除取消 | app.js L1750（if(!okD) return） | 确认框 | N/A（不适用） | 取消 | 不移除 | 未验证 | 无 | 本页 | C:app.js L1750 | 已实现｜运行时未验证 |
| I-MEM-16 | 成员 | 家庭信息块 | app.js L1686–1703（fblock） | 成员页 | N/A（不适用） | N/A（不适用） | 显示家庭名/成员数/类型/创建时间 | 未验证 | N/A（不适用） | 本页 | C:app.js L1686–1703；S:members/390x844/normal.png | 已实现｜截图已见 |

#### 3.1.10 我的 `#b/:id/mine`

> `renderMine` app.js L2138–2198；截图 `mine/{390x844,1280x800}/normal.png`。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-MINE-01 | 我的 | 头像 / 用户名 | app.js L2148–2153 | N/A（不适用） | 取 state.whoami | N/A（不适用） | 显示 | 未验证 | N/A（不适用） | 本页 | C:app.js L2148–2153；S:mine/390x844/normal.png | 已实现｜截图已见 |
| I-MINE-02 | 我的 | 用户名（只读） | app.js L2162–2164（vu） | GET /users/me L2157 | 注释「用户名按后端设计为只读」L2155 | N/A（不适用） | 显示用户名 | 未验证 | N/A（不适用） | 本页 | C:app.js L2155,L2162–2164；S:mine/390x844/normal.png | 已实现｜截图已见｜R0-E-17 |
| I-MINE-03 | 我的 | 邮箱输入 | app.js L2168（eInput type=email） | N/A（不适用） | placeholder「未设置」 | 键入 | 显示文本 | 浏览器 type=email 校验 | N/A（不适用） | 本页 | C:app.js L2168；S:mine/390x844/normal.png | 已实现｜截图已见 |
| I-MINE-04 | 我的 | 邮箱保存 | app.js L2169–2174（eSave） | N/A（不适用） | btnPending L2171 | PUT /users/me {email} L2172 | toast「资料已更新」L2172 | toast(err.message,"err") L2173 | N/A（不适用） | 本页 | C:app.js L2169–2174；运行时未验证 | 已实现｜入口在「我的」而非设置页｜R0-E-17 |
| I-MINE-05 | 我的 | 二级入口：我的账本 | app.js L2180–2196（links[0] → #books） | N/A（不适用） | N/A（不适用） | 点击 | hash=#books | 未验证 | N/A（不适用） | #books | C:app.js L2180–2196；S:mine/390x844/normal.png | 已实现｜截图已见｜R0-E-01 |
| I-MINE-06 | 我的 | 二级入口：账户 | app.js L2180–2196（→ #b/{id}/accounts） | N/A（不适用） | N/A（不适用） | 点击 | 进账户页 | 未验证 | N/A（不适用） | accounts | C:app.js L2180–2196 | 已实现｜截图已见 |
| I-MINE-07 | 我的 | 二级入口：家庭成员 | app.js L2180–2196（→ members） | N/A（不适用） | N/A（不适用） | 点击 | 进成员页 | 未验证 | N/A（不适用） | members | C:app.js L2180–2196 | 已实现｜截图已见 |
| I-MINE-08 | 我的 | 二级入口：导出数据 | app.js L2180–2196（→ export） | N/A（不适用） | N/A（不适用） | 点击 | 进导出页 | 未验证 | N/A（不适用） | export | C:app.js L2180–2196 | 已实现｜截图已见 |
| I-MINE-09 | 我的 | 二级入口：报表 | app.js L2180–2196（→ report） | N/A（不适用） | N/A（不适用） | 点击 | 进报表页 | 未验证 | N/A（不适用） | report | C:app.js L2180–2196 | 已实现｜截图已见 |
| I-MINE-10 | 我的 | 二级入口：预算 | app.js L2180–2196（→ budget） | N/A（不适用） | N/A（不适用） | 点击 | 进预算页 | 未验证 | N/A（不适用） | budget | C:app.js L2180–2196 | 已实现｜截图已见 |
| I-MINE-11 | 我的 | 二级入口：资产 | app.js L2180–2196（→ invest） | N/A（不适用） | N/A（不适用） | 点击 | 进投资页 | 未验证 | N/A（不适用） | invest | C:app.js L2180–2196 | 已实现｜截图已见 |
| I-MINE-12 | 我的 | 二级入口：回收站 | app.js L2180–2196（→ recycle） | N/A（不适用） | N/A（不适用） | 点击 | 进回收站 | 未验证 | N/A（不适用） | recycle | C:app.js L2180–2196 | 已实现｜截图已见 |
| I-MINE-13 | 我的 | 改密入口 | —（grep 无命中） | 我的/设置 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：app.js `password` 仅 L179/180/184/185/192/193/209（登录注册）；无 /users/me/password 调用 | **缺失**｜R0-E-18 |
| I-MINE-14 | 我的 | 会话管理入口 | —（grep 无命中） | 我的/设置 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：app.js `session\|会话` 仅 L3 头部注释 | **缺失**｜R0-E-14 / R0-E-31 |

#### 3.1.11 设置 `#settings`

> `renderSettings` app.js L1518–1633；截图 `settings/{360x800,390x844,430x932,1280x800}/normal.png`。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-SET-01 | 设置 | 生成新 Token | app.js L1539–1547（genBtn） | 设置页 | dlgInput「用途备注」L1541 | POST /users/me/tokens {name} L1543 | toast「Token 已生成——请立即复制保存」+ renderSettingsFresh(data.token,data.item) L1544–1545 | 由 openInvestDialog 捕获 | 显示明文 + 刷新列表 | 本页 | C:app.js L1539–1547；S:settings/390x844/normal.png | 已实现｜截图已见 |
| I-SET-02 | 设置 | Token 明文展示 | app.js L1555–1557（code, user-select:all） | 生成后 | N/A（不适用） | N/A（不适用） | 显示明文（仅一次） | 未验证 | N/A（不适用） | 本页 | C:app.js L1555–1557 | 已实现｜运行时未验证 |
| I-SET-03 | 设置 | 复制 Token | app.js L1558–1562（cp） | 生成后 | navigator.clipboard.writeText(plaintext) | 点击 | toast「Token 已复制」L1560 | toast「复制失败，请手动选中复制」L1561 | N/A（不适用） | 本页 | C:app.js L1558–1562 | 已实现｜运行时未验证 |
| I-SET-04 | 设置 | Token 列表 | app.js L1573–1617（loadTokens） | 设置页 | N/A（不适用） | GET /users/me/tokens L1574 | 表列 备注/创建时间/最近使用/状态 | 未验证 | N/A（不适用） | 本页 | C:app.js L1573–1617；S:settings/390x844/normal.png | 已实现｜截图已见 |
| I-SET-05 | 设置 | 撤销 Token | app.js L1597–1609（b） | !it.revokedAt | confirmDialog L1599–1602 | DELETE /users/me/tokens/:id L1605 | toast「Token 已撤销」+ loadTokens() | toast(err.message,"err") L1608 | 刷新列表 | 本页 | C:app.js L1597–1609 | 已实现｜运行时未验证 |
| I-SET-06 | 设置 | 撤销取消 | app.js L1603（if(!okD) return） | 确认框 | N/A（不适用） | 取消 | 不撤销 | 未验证 | 无 | 本页 | C:app.js L1603 | 已实现｜运行时未验证 |
| I-SET-07 | 设置 | Token 空态 | app.js L1576–1579 | 无 Token | N/A（不适用） | N/A（不适用） | 「还没有长期 Token。」 | 未验证 | N/A（不适用） | 本页 | C:app.js L1576–1579；S:settings/390x844/normal.png（本轮无 Token） | 已实现｜截图已见 |
| I-SET-08 | 设置 | 退出登录 | app.js L1623–1630（ob） | 设置页 | confirmDialog L1625 | clearSession() + toast「已退出登录」+ route() L1627–1629 | 未验证 | 未验证 | 清 localStorage | 登录页 | C:app.js L1623–1630；S:settings/390x844/normal.png | 已实现｜截图已见 |
| I-SET-09 | 设置 | 个人资料入口（设置页内） | —（无） | 设置页 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：资料编辑在 renderMine L2165–2175；设置页无资料入口 | **缺失**｜R0-E-17 |
| I-SET-10 | 设置 | 改密 UI | —（grep 无命中） | 设置页 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：见 I-MINE-13；后端 POST /users/me/password 已有（router.go L139） | **缺失**｜R0-E-18 |
| I-SET-11 | 设置 | 会话管理 UI | —（grep 无命中） | 设置页 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：见 I-MINE-14；后端 /auth/sessions 已有（router.go L154–156） | **缺失**｜R0-E-14 / R0-E-31 |
| I-SET-12 | 设置 | 服务器 / 备份区块 | —（grep 无命中） | 设置页 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 未实现 | 缺失：app.js `server\|服务器\|备份\|backup` 仅 L991/L1048 导出格式描述文案；renderSettings L1518–1633 无相关卡片 | **缺失**｜R0-E-17 / D-8 |

#### 3.1.12 报表 `#b/:id/report`

> `renderReport` app.js L804–907；截图 `report/{390x844,1280x800}/empty.png`。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-RPT-01 | 报表 | 「本月」按钮 | app.js L817–824（mkMonthBtn 0） | 报表页 | reportMonthOffset===off 决定 active | 点击 → offset=0 + renderReport L821 | 重绘本页（本月） | 未验证 | 切换月份 | 本页 | C:app.js L817–824；S:report/390x844/empty.png | 已实现｜截图已见｜R0-E-09 |
| I-RPT-02 | 报表 | 「上月」按钮 | app.js L817–824（mkMonthBtn 1） | 报表页 | 同上 | 点击 → offset=1 + renderReport | 重绘本页（上月） | 未验证 | 切换月份 | 本页 | C:app.js L817–824；S:report/390x844/empty.png | 已实现｜截图已见｜R0-E-09 |
| I-RPT-03 | 报表 | 月份计算 | app.js L918–924（monthShift） | N/A（不适用） | 本地时区 YYYY-MM | N/A（不适用） | 生成月份字符串 | 未验证 | N/A（不适用） | 本页 | C:app.js L918–924 | 已实现｜运行时未验证 |
| I-RPT-04 | 报表 | 收支趋势数据 | app.js L831（GET /reports/monthly?months=6） | 报表页 | trend.items\|\|[] L839 | N/A（不适用） | 柱状图数据 | 未验证 | N/A（不适用） | 本页 | C:app.js L831；S:report/390x844/empty.png | 已实现｜截图已见 |
| I-RPT-05 | 报表 | 柱状图 | app.js L837–863（bars/mkBar, maxV 归一化） | 有数据 | N/A（不适用） | N/A（不适用） | 渲染柱子 | 未验证 | N/A（不适用） | 本页 | C:app.js L837–863；有数据态未采集 | 已实现｜视觉待验收｜R0-E-09 |
| I-RPT-06 | 报表 | 图例 | app.js L864–867 / L909–915（mkLegend） | N/A（不适用） | N/A（不适用） | N/A（不适用） | 收入/支出色块 | 未验证 | N/A（不适用） | 本页 | C:app.js L864–867,L909–915 | 已实现｜视觉待验收｜R0-E-09 |
| I-RPT-07 | 报表 | 分类占比数据 | app.js L871（GET /reports/categories?type=2&month=） | 月份确定 | share.items\|\|[] L876 | N/A（不适用） | 排行表数据 | 未验证 | N/A（不适用） | 本页 | C:app.js L871；S:report/390x844/empty.png | 已实现｜截图已见 |
| I-RPT-08 | 报表 | 分类排行表 | app.js L885–905（分类/笔数/金额/占比, Math.round） | 有数据 | N/A（不适用） | N/A（不适用） | 渲染表格 | 未验证 | N/A（不适用） | 本页 | C:app.js L885–905；有数据态未采集 | 已实现｜视觉待验收｜R0-E-09 |
| I-RPT-09 | 报表 | 空态 | app.js L877–881 | 无数据 | N/A（不适用） | N/A（不适用） | 「该月暂无支出记录。」 | 未验证 | N/A（不适用） | 本页 | C:app.js L877–881；S:report/390x844/empty.png | 已实现｜截图已见 |

#### 3.1.13 预算 `#b/:id/budget`

> `renderBudget` L1108–1179 / `openBudgetDialog` L1182–1236；截图 `budget/{390x844,1280x800}/empty.png`。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-BUD-01 | 预算 | 新增预算 | app.js L1120–1122（addBtn） | 预算页 | N/A（不适用） | 点击 → openBudgetDialog(bookId,null) | 打开 #budgetDlg | 未验证 | N/A（不适用） | 弹层 | C:app.js L1120–1122；S:budget/390x844/empty.png | 已实现｜截图已见｜R0-E-10 |
| I-BUD-02 | 预算 | 预算行（名称/三态徽标/进度条） | app.js L1138–1155 | GET /budgets L1129 | BUDGET_STATUS_NAME L1105 | N/A（不适用） | 显示三态徽标与进度 | 未验证 | N/A（不适用） | 本页 | C:app.js L1138–1155；有数据态未采集 | 已实现｜三态色待截图｜R0-E-10 |
| I-BUD-03 | 预算 | 编辑 | app.js L1157–1159（edit） | 每行 | N/A（不适用） | 点击 → openBudgetDialog(bookId,it) | 打开弹窗回填 | 未验证 | N/A（不适用） | 弹层 | C:app.js L1157–1159 | 已实现｜运行时未验证 |
| I-BUD-04 | 预算 | 删除 | app.js L1160–1173（del） | 每行 | confirmDialog L1163–1166 | DELETE /budgets/:id L1169 | toast「预算已删除」+ renderBudget L1170–1171 | toast(err.message,"err") L1172 | 重绘 | 本页 | C:app.js L1160–1173 | 已实现｜运行时未验证 |
| I-BUD-05 | 预算 | 删除取消 | app.js L1167（if(!okD) return） | 确认框 | N/A（不适用） | 取消 | 不删除 | 未验证 | 无 | 本页 | C:app.js L1167 | 已实现｜运行时未验证 |
| I-BUD-06 | 预算 | 空态 | app.js L1132–1137 | 无预算 | N/A（不适用） | N/A（不适用） | 「还没有预算——先设一个本月总预算…」 | 未验证 | N/A（不适用） | 本页 | C:app.js L1132–1137；S:budget/390x844/empty.png | 已实现｜截图已见 |
| I-BUD-07 | 预算 | 类型 `#bType` | index.html L91 / app.js L1190–1193 | 弹窗 | 编辑时 disabled=!!edit L1193 | 选择 | 选中类型 | 未验证 | N/A（不适用） | 弹层 | C:index.html L91, app.js L1190–1193 | 已实现｜运行时未验证 |
| I-BUD-08 | 预算 | 分类 `#bCategory` | index.html L96 / app.js L1197–1206 | 非总预算 | 仅列 catType===2 支出分类 L1200；编辑总预算时 catRow 隐藏 L1194 | 选择 | 选中分类 | 未验证 | N/A（不适用） | 弹层 | C:index.html L96, app.js L1197–1206 | 已实现｜运行时未验证 |
| I-BUD-09 | 预算 | 金额 `#bAmount` | index.html L97 / app.js L1187,L1213–1214 | 弹窗 | str2cents；!cents\|\|cents<=0 → #bErr「请输入大于 0 的金额」L1214 | 键入 | 显示文本 | err-msg | N/A（不适用） | 弹层 | C:index.html L97, app.js L1213–1214 | 已实现｜运行时未验证 |
| I-BUD-10 | 预算 | 结转 `#bRollover` | index.html L99 / app.js L1188,L1215 | 弹窗 | 文案「结转上月余额（预留，MVP 不生效）」 | 勾选 | 提交 rollover | 未验证 | N/A（不适用） | 弹层 | C:index.html L99, app.js L1215 | 已实现（MVP 不生效）｜运行时未验证 |
| I-BUD-11 | 预算 | 取消 `#bCancel` | index.html L103 / app.js L1210 | 弹窗 | N/A（不适用） | 点击 → closeBudgetDlg() L1237–1241 | 关闭 | 未验证 | 无 | 本页 | C:index.html L103, app.js L1210 | 已实现｜运行时未验证 |
| I-BUD-12 | 预算 | 保存 `#bSubmit` | index.html L104 / app.js L1211–1235 | 金额合法 | btnPending L1216 | 编辑 PUT /budgets/:id {amount,rollover,name:""} L1220；新增 POST /budgets {amount,rollover,budgetType,categoryId,name} L1226 | toast「预算已创建/已更新」+ closeBudgetDlg() + renderBudget L1229–1230 | #bErr.textContent=err.message + toast L1232–1233 | 重绘 | 本页 | C:app.js L1211–1235 | 已实现｜运行时未验证 |

#### 3.1.14 投资 / 资产 `#b/:id/invest`

> `renderInvest` L1246–1393 + open*Dialog；截图 `invest/{390x844,1280x800}/empty.png`。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-INV-01 | 投资 | 资产总览 4 卡 | app.js L1255–1276（mkCell） | GET /net-worth L1255 | N/A（不适用） | N/A（不适用） | 总资产/总负债/持仓市值/家庭净资产 | 未验证 | N/A（不适用） | 本页 | C:app.js L1255–1276；S:invest/390x844/empty.png | 已实现｜位置在投资页｜R0-E-28 / R0-E-07 |
| I-INV-02 | 投资 | 添加持仓 | app.js L1288–1289（addHolding） | 持仓区 | N/A（不适用） | 点击 → openHoldingDialog(bookId) | 打开弹窗 | 未验证 | N/A（不适用） | 弹层 | C:app.js L1288–1289；S:invest/390x844/empty.png | 已实现｜截图已见｜R0-E-07 |
| I-INV-03 | 投资 | 持仓编辑 | app.js L1321–1322（bEdit） | 每行 | N/A（不适用） | 点击 → openHoldingDialog(bookId,it) | 打开弹窗回填 | 未验证 | N/A（不适用） | 弹层 | C:app.js L1321–1322 | 已实现｜运行时未验证 |
| I-INV-04 | 投资 | 录价 | app.js L1323–1324（bPrice） | 每行 | N/A（不适用） | 点击 → openPriceDialog(...) | 打开录价弹窗 | 未验证 | N/A（不适用） | 弹层 | C:app.js L1323–1324 | 已实现｜运行时未验证 |
| I-INV-05 | 投资 | 持仓删除 | app.js L1325–1334（bDel） | 每行 | confirmDialog L1327 | DELETE /holdings/:id L1330 | toast「持仓已删除」+ renderInvest | toast(err.message,"err") L1333 | 重绘 | 本页 | C:app.js L1325–1334 | 已实现｜运行时未验证 |
| I-INV-06 | 投资 | 新增标的 | app.js L1349–1350（addInstr） | 标的库 | N/A（不适用） | 点击 → openInstrumentDialog(bookId) | 打开弹窗 | 未验证 | N/A（不适用） | 弹层 | C:app.js L1349–1350；S:invest/390x844/empty.png | 已实现｜截图已见 |
| I-INV-07 | 投资 | 标的编辑 | app.js L1374–1375（bEdit） | 每行 | N/A（不适用） | 点击 → openInstrumentDialog(bookId,it) | 打开弹窗回填 | 未验证 | N/A（不适用） | 弹层 | C:app.js L1374–1375 | 已实现｜运行时未验证 |
| I-INV-08 | 投资 | 标的删除 | app.js L1376–1385（bDel） | 每行 | confirmDialog 文案「仍有持仓引用的标的无法删除。」L1378 | DELETE /instruments/:id L1381 | toast「标的已删除」+ renderInvest | toast(err.message,"err") L1384（409） | 重绘 | 本页 | C:app.js L1376–1385 | 已实现｜运行时未验证 |
| I-INV-09 | 投资 | 空态（持仓 / 标的） | app.js L1292–1297 / L1353–1358 | 无数据 | N/A（不适用） | N/A（不适用） | 空态文案 | 未验证 | N/A（不适用） | 本页 | C:app.js L1292–1297,L1353–1358；S:invest/390x844/empty.png | 已实现｜截图已见 |
| I-INV-10 | 投资 | 通用弹窗字段容器 | app.js L1396–1422（openInvestDialog → #investFields L1401–1407） | N/A（不适用） | 动态注入 | N/A（不适用） | 渲染字段 | 未验证 | N/A（不适用） | 弹层 | C:app.js L1396–1422 | 已实现｜运行时未验证 |
| I-INV-11 | 投资 | 取消 `#iCancel` | index.html L114 / app.js L1410 | 弹窗 | N/A（不适用） | 点击 → closeInvestDlg() L1423–1427 | 关闭 | 未验证 | 无 | 本页 | C:index.html L114, app.js L1410 | 已实现｜运行时未验证 |
| I-INV-12 | 投资 | 保存 `#iSubmit` | index.html L115 / app.js L1411–1421 | N/A（不适用） | btnPending L1413 | await onSubmit() L1415 + closeInvestDlg() L1416 | 关闭 | #iErr.textContent=err.message + toast L1418–1419 | N/A（不适用） | 本页 | C:app.js L1411–1421 | 已实现｜运行时未验证 |
| I-INV-13 | 投资 | 标的字段（名称/类型/市场/代码） | app.js L1435–1443 | 标的弹窗 | 编辑回填 | POST/PUT /instruments L1449/L1452 | toast + renderInvest L1450/L1453/L1455 | 通用弹窗捕获 | 重绘 | 本页 | C:app.js L1435–1455 | 已实现｜运行时未验证 |
| I-INV-14 | 投资 | 持仓字段（账户/标的/数量/成本/现价） | app.js L1460–1480 | 持仓弹窗 | 标的异步拉取 L1469；编辑时账户/标的 disabled；str2cents 成本 L1486 | POST/PUT /holdings L1497/L1492 | toast + renderInvest L1500 | 通用弹窗捕获 | 重绘 | 本页 | C:app.js L1460–1500 | 已实现｜运行时未验证 |
| I-INV-15 | 投资 | 录价字段（最新价） | app.js L1505 | 录价弹窗 | N/A（不适用） | POST /instruments/:id/price {price} L1509 | toast「行情已更新…」+ renderInvest L1511–1512 | 通用弹窗捕获 | 现价/市值刷新 | 本页 | C:app.js L1505–1512 | 已实现｜运行时未验证 |
| I-INV-16 | 投资账户详情 | 返回对账 | app.js L1783–1785（back） | #b/:id/account/:accId | N/A（不适用） | 点击 → #b/{id}/accounts | 未验证 | 未验证 | N/A（不适用） | 对账页 | C:app.js L1783–1785；S:account-detail/390x844/normal.png | 已实现｜截图已见 |

#### 3.1.15 导出 `#b/:id/export`

> `renderExport` app.js L933–1095；截图 `export/{390x844,1280x800}/normal.png`。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-EXP-01 | 导出 | 时间预设「本月」 | app.js L950–953 / L970（setRange this） | 导出页 | N/A（不适用） | 点击 → setRange("this") | 高亮 + 填日期 | 未验证 | N/A（不适用） | 本页 | C:app.js L950–970；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-02 | 导出 | 时间预设「上月」 | app.js L950 / L961–969 | 导出页 | N/A（不适用） | 点击 → setRange("last") | 高亮 + 填日期 | 未验证 | N/A（不适用） | 本页 | C:app.js L961–969；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-03 | 导出 | 时间预设「本季度」 | app.js L950 / L966 | 导出页 | N/A（不适用） | 点击 → setRange("quarter") | 高亮 + 填日期 | 未验证 | N/A（不适用） | 本页 | C:app.js L966；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-04 | 导出 | 时间预设「自定义」 | app.js L950 / L967 | 导出页 | 不自动填日期 | 点击 | 高亮 custom | 未验证 | N/A（不适用） | 本页 | C:app.js L967；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-05 | 导出 | 开始日期 `#xFrom` | app.js L955 | 导出页 | change 时切到 custom L971–973 | 选择 | 填入日期 | 未验证 | N/A（不适用） | 本页 | C:app.js L955,L971–973；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-06 | 导出 | 结束日期 `#xTo` | app.js L956 | 导出页 | 同上 | 选择 | 填入日期 | 未验证 | N/A（不适用） | 本页 | C:app.js L956；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-07 | 导出 | 收支类型 | app.js L979–982（typeSel） | 导出页 | 0 全部/1 收入/2 支出/3 转账 | 选择 | 选中类型 | 未验证 | N/A（不适用） | 本页 | C:app.js L979–982；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-08 | 导出 | 格式 CSV | app.js L991–995（radio value=csv 默认 checked） | 导出页 | N/A（不适用） | 选中 | 选中 CSV | 未验证 | N/A（不适用） | 本页 | C:app.js L991–995；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-09 | 导出 | 格式 JSON | app.js L991–995（radio value=json） | 导出页 | N/A（不适用） | 选中 | 选中 JSON | 未验证 | N/A（不适用） | 本页 | C:app.js L991–995；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-10 | 导出 | 字段勾选：time | app.js L927–931 / L1011–1018（EXPORT_FIELDS） | 导出页 | 默认全选 L1014 | 勾选/取消 | 字段列表变化 | 未验证 | N/A（不适用） | 本页 | C:app.js L927–931,L1011–1018；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-11 | 导出 | 字段勾选：type | 同上 | 导出页 | 默认全选 | 勾选/取消 | 字段列表变化 | 未验证 | N/A（不适用） | 本页 | C:同上；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-12 | 导出 | 字段勾选：amount | 同上 | 导出页 | 默认全选 | 勾选/取消 | 字段列表变化 | 未验证 | N/A（不适用） | 本页 | C:同上；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-13 | 导出 | 字段勾选：category | 同上 | 导出页 | 默认全选 | 勾选/取消 | 字段列表变化 | 未验证 | N/A（不适用） | 本页 | C:同上；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-14 | 导出 | 字段勾选：account | 同上 | 导出页 | 默认全选 | 勾选/取消 | 字段列表变化 | 未验证 | N/A（不适用） | 本页 | C:同上；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-15 | 导出 | 字段勾选：toAccount | 同上 | 导出页 | 默认全选 | 勾选/取消 | 字段列表变化 | 未验证 | N/A（不适用） | 本页 | C:同上；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-16 | 导出 | 字段勾选：channel | 同上 | 导出页 | 默认全选 | 勾选/取消 | 字段列表变化 | 未验证 | N/A（不适用） | 本页 | C:同上；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-17 | 导出 | 字段勾选：merchant | 同上 | 导出页 | 默认全选 | 勾选/取消 | 字段列表变化 | 未验证 | N/A（不适用） | 本页 | C:同上；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-18 | 导出 | 字段勾选：note | 同上 | 导出页 | 默认全选 | 勾选/取消 | 字段列表变化 | 未验证 | N/A（不适用） | 本页 | C:同上；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-19 | 导出 | 预览 | app.js L1022 / L1024–1038 / L1093 | 导出页 | btnPending L1031 | GET /transactions?…&count=1&page=1 L1033 | 「共 N 条记录将导出。」L1035 | 「预览失败：…」L1036 | N/A（不适用） | 本页 | C:app.js L1024–1038；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-20 | 导出 | 自动预览 | app.js L1094（updatePreview()） | 进入页 | N/A（不适用） | N/A（不适用） | 未验证 | 未验证 | N/A（不适用） | 本页 | C:app.js L1094；S:export/390x844/normal.png | 已实现｜截图已见 |
| I-EXP-21 | 导出 | 导出 CSV | app.js L1042 / L1091（btnCsv） | 至少 1 字段 | !fields.length → toast「请至少勾选一个导出字段」L1054 | 原生 fetch /export?format=csv&fields=… L1062 → blob | 下载 FastLedger_导出_YYYYMMDD.csv L1081 + toast「已导出 N 条记录」L1086 | toast(err.message,"err") L1088 | N/A（不适用） | 本页 | C:app.js L1042–1088；下载动作未采集 | 已实现｜运行时未验证 |
| I-EXP-22 | 导出 | 导出 JSON | app.js L1043 / L1092（btnJson） | 至少 1 字段 | 同上 | doExport("json") | 下载 .json | 同上 | N/A（不适用） | 本页 | C:app.js L1043,L1092；运行时未验证 | 已实现｜运行时未验证 |
| I-EXP-23 | 导出 | 空数据提示 | app.js L1070–1071（X-Export-Count===0） | 导出返回 0 条 | N/A（不适用） | N/A（不适用） | 未验证 | toast「没有可导出的数据」info | N/A（不适用） | 本页 | C:app.js L1070–1071；未采集 | 已实现｜运行时未验证 |
| I-EXP-24 | 导出 | 截断提示 | app.js L1072–1074（X-Export-Truncated==="1"） | 结果 >10000 条 | N/A（不适用） | N/A（不适用） | 未验证 | toast「结果超过 10000 条，已截断」info | N/A（不适用） | 本页 | C:app.js L1072–1074；未采集 | 已实现｜运行时未验证 |

#### 3.1.16 回收站 `#b/:id/recycle`

> `renderRecycle` app.js L751–799；截图 `recycle/{390x844,1280x800}/empty.png`。

| ID | 页面 | 控件 / 动作 | 代码位置 | 前置条件 | 输入与校验 | 操作 | 成功结果 | 失败结果 | 数据联动 | 返回路径 | 验证证据 | 当前状态 / 关联 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| I-REC-01 | 回收站 | 加载 | app.js L766（GET /recycle?page=1&count=100） | 进入页 | N/A（不适用） | N/A（不适用） | 行渲染 L767–789 | 未验证 | N/A（不适用） | 本页 | C:app.js L766；S:recycle/390x844/empty.png | 已实现｜截图已见｜R0-E-16 |
| I-REC-02 | 回收站 | 恢复按钮 | app.js L778–787（btn） | 每行 | btnPending L780 | POST /transactions/:id/restore?clientUuid=uuid() L782 | toast「已恢复」+ 行 fade-out 后 tr.remove() L783–785 | toast(e.message,"err"); done() L786 | 行移除 | 本页 | C:app.js L778–787；有数据态未采集 | 已实现｜运行时未验证 |
| I-REC-03 | 回收站 | 空态 | app.js L791–796 | 无数据 | N/A（不适用） | N/A（不适用） | 「回收站为空——…」 | 未验证 | N/A（不适用） | 本页 | C:app.js L791–796；S:recycle/390x844/empty.png | 已实现｜截图已见 |

**逐控件清单统计**：16 个分组、228 条独立控件/动作；其中「缺失」项均给出 grep 无命中证据并关联 R0-E 编号；`I-HOME-02` 经 RV-04 查证无需求依据，已改为「待 Owner 裁决的交互建议」，**不计入已确认缺陷统计**。本轮二次复核**新发现 2 项**（R0-E-38 加入账本/邀请码兑换断口、R0-E-39 密码明文切换缺失），详见 `docs/FastLedger-R0-独立补漏与复核记录.md` §9，非重复编号。

**状态判定**：§3.1 已按单一控件拆分并可定位到 DOM/JS 行号，但「运行时未验证」项仍大量存在、截图仅覆盖部分状态；按任务单自检规则，**§3.1 仍未达到「每个成功/失败路径均有记录且可验证」的完整通过条件**，不得据此宣称 R0 通过。

**逐控件交互清单状态（二次整改后）**：本版共 16 个页面分组、228 条独立控件/动作，每条 13 字段；原 §3.1 把多个独立控件合并成一行（金额/账户/分类/日期/备注合并、FAB 与 Tab 合并、账户增改删合并、「我的」8 入口合并）的写法已全部拆开，误引的 `#loginForm` 已更正为「`renderLogin` 动态生成、无 id」。本轮二次复核新增两项独立缺口：**R0-E-38**（登录页无「加入账本」入口/邀请码兑换，而 `renderMembers` L1667 有悬空引导，邀请链路在 Web 端断开）、**R0-E-39**（密码明文切换缺失）；另补 I-BOOKS-11「账本详情/账本设置页入口缺失」，关联 R0-E-19 / R0-E-32。

**证据边界（不得夸大）**：`C:` 为静态源码定位；`S:` 为本轮 Edge 真实浏览器截图，仅证明「控件/页面在该状态下可见」，**不证明交互成功路径已验收**。当前 43 张截图只覆盖 normal / empty / error 首屏，另 5 张 RV-01 隔离运行时截图仅覆盖首页交易列表请求的 pending / 失败 / 恢复态；绝大多数成功/失败交互路径、加载态、权限不足态、离线态、同步/冲突态与原型侧对照截图均未采集（见 §7.5 与 `docs/r0-screenshots/README.md`）。因此**本清单不构成逐控件运行时验收通过**，标「运行时未验证」即未验证；R0 截图门槛仍为未完成 / 阻塞。

---

## 4. R0-C：`prototype/` 与 `prototype-v2/` 差异对照

| 差异点 | `prototype/`（旧，13 页） | `prototype-v2/`（新，规范示范） | 是否影响业务语义 | 建议基准 |
|---|---|---|---|---|
| 页面数 | 13 页（含**投资资产**独立页） | README 称"12 视图"；HTML 含「净资产」（账户视图汇总），**独立"投资资产"页是否存在 `[待确认]`** | 是（投资/持仓是否独立管理） | 以 `prototype/` 为准，v2 补视觉 |
| 记账入口 | "手动记账/转账" | "记一笔"（支出/收入/转账分段 + 截图识别入口） | 否（同一能力，措辞/手势不同） | 交互按 v2，能力按 prototype |
| 截图识别 | 有（截图识别记账页） | 有（三级漏斗演示，绝不自动入账） | Web OCR 由 PRD §5.2 明确不做；PWA 浏览器端本地 OCR 是否新增另行裁决 | PWA 扩展待确认 |
| 二级页归属 | 分类/导出/成员/设置在"我的"下 | 同左（我的为菜单中枢） | 否 | 一致 |
| 导航 | 13 页原型 | 桌面 5 项 / 移动 4 Tab | 否（结构一致） | 采用 v2/规范 |
| 投资资产 | 独立页（持仓、实物、净资产） | 净资产存在于账户视图，独立页 `[待确认]` | **是** | 依已裁决 D-1（流程以 prototype/ 为准），是否独立成页见 R0-G D-5 |

> **结论**：两版核心业务流程一致；**唯一影响业务语义的差异**是「投资资产是否独立成页」与「截图识别范围」。其余为措辞/手势/视觉差异，按 §2.2 规则处理，最终由 Owner 在 R0-G 确认基准。

---

## 5. R0-D：当前实现 → 路由/函数/API 映射，与后端 API 能力矩阵

### 5.1 前端路由 / render 函数映射（`server/web/dist/app.js`）

| hash 路由 | render 函数 | 行号 | 调用的主要 API |
|---|---|---|---|
| `#`（无 token） | `renderLogin` | L161 | `POST /auth/login`、`POST /auth/register`、`GET /users/me` |
| `#books` | `renderBooks` | L234 | `GET /books`、`POST /books`、`DELETE /books/:id` |
| `#b/:id` | `renderHome` + `renderBillStream` | L1929 / L1872 | `GET /reports/monthly?months=2`、`GET /accounts`、`GET /transactions?page=1&count=60` |
| `#b/:id/txns` | `renderTxns` | L337 | `GET /transactions?...`（筛选/分页）、批量删除 |
| `#b/:id/tx/:txId` | `renderTxDetail` | L1974 | `GET /transactions/:txId`、`PUT`（编辑）、`DELETE`（删除） |
| `#b/:id/accounts` | `renderAccounts` | L647 | `GET/POST/PUT/DELETE /accounts` |
| `#b/:id/account/:accId` | `renderAccountDetail` | L1768 | `GET /accounts/:aid` |
| `#b/:id/categories` | `renderCategories` | L2069 | `GET /categories?tree=1`、`POST/PUT/DELETE /categories` |
| `#b/:id/recycle` | `renderRecycle` | L751 | `GET /recycle`、`POST /transactions/:txId/restore` |
| `#b/:id/report` | `renderReport` | L804 | `GET /reports/monthly?months=6`、`GET /reports/categories` |
| `#b/:id/export` | `renderExport` | L933 | `GET /transactions`（预览）、`GET /export`（下载） |
| `#b/:id/budget` | `renderBudget` | L1108 | `GET/POST/PUT/DELETE /budgets` |
| `#b/:id/invest` | `renderInvest` | L1246 | `GET/POST/PUT/DELETE /instruments`、`/holdings`、`GET /net-worth`、`POST /price` |
| `#b/:id/members` | `renderMembers` | L1636 | `GET /members`、`POST /invite`、`PUT/DELETE /members/:uid` |
| `#b/:id/mine` | `renderMine` | L2138 | `GET/PUT /users/me`（改邮箱） |
| `#settings` | `renderSettings` | L1518 | `GET/POST/DELETE /users/me/tokens` |

### 5.2 后端 API 能力矩阵（`server/internal/api/router.go` L109–L235）

| 能力域 | 方法 & 路径 | 鉴权 | 前端已接通？ |
|---|---|---|---|
| 健康 | `GET /healthz` | 公开 | — |
| 认证 | `POST /auth/register`、`POST /auth/login` | 公开（注册受 `EBK_ENABLE_REGISTER`） | ✅ |
| 当前用户 | `GET/PUT /users/me`、`POST /users/me/password` | JWT | ✅（改邮箱）；改密 [待确认] |
| PAT | `GET/POST /users/me/tokens`、`DELETE /users/me/tokens/:tokenId` | JWT | ✅ |
| 会话 | `GET /auth/sessions`、`DELETE /auth/sessions[/:jti]` | JWT | ❌（前端未接，WBS 5.10 后端已有） |
| OCR | `POST /ocr/recognize`、`/ocr/parse`、`/ocr/llm-parse` | JWT | ❌（Web 不做，见 R0-E-08） |
| 账本 | `GET/POST /books`、`GET/PUT/DELETE /books/:id`、`PUT /books/:id/visibility` | JWT | ✅ |
| 成员 | `GET /books/:id/members`、`POST .../invite`、`POST .../invite/:inviteId/revoke`、`PUT/DELETE .../members/:uid` | JWT | ✅（撤销邀请 [待确认]） |
| 账户 | `GET/POST /books/:id/accounts`、`GET/PUT/DELETE .../accounts/:aid` | JWT | ✅ |
| 分类 | `GET/POST /books/:id/categories`、`PUT/DELETE .../categories/:cid` | JWT | ✅ |
| 交易 | `GET/POST /books/:id/transactions`、`GET/PUT/DELETE .../transactions/:txId`、`POST .../:txId/restore` | JWT | ✅（PUT 依 [1.44] 已开放，语义见 `交易更新语义设计.md`） |
| 回收站 | `GET /books/:id/recycle` | JWT | ✅ |
| 报表 | `GET /books/:id/reports/monthly`、`/reports/categories` | JWT | ✅ |
| 导出 | `GET /books/:id/export` | JWT | ✅ |
| 预算 | `GET/POST /books/:id/budgets`、`PUT/DELETE .../budgets/:budgetId` | JWT | ✅ |
| 投资 | `GET/POST /books/:id/instruments`、`PUT/DELETE .../instruments/:id`、`POST .../:id/price`、`GET/POST /holdings`、`PUT/DELETE /holdings/:id`、`GET /net-worth` | JWT | ✅ |
| 同步 | `GET /books/:id/sync/pull`、`POST /books/:id/sync/push` | JWT | ❌（当前 Web `app.js` 未调用） |
| WS | `GET /api/v1/ws` | JWT | ❌（后端路由存在；当前 Web `index.html` 仅加载 `/app.js`，`app.js` 未发现 WebSocket 客户端） |

**关键结论（修正）**：后端已有多组 API 路由，但路由存在不等于服务层、权限、DTO、测试均完整；本矩阵尚未达到总方案要求的契约粒度。当前 Web 未发现 WS 客户端或 sync pull/push 调用；OCR 的 Web 范围由 PRD §5.2 明确为不做；会话、可见性和邀请撤销等 API/能力不能据此推断前端已接通。
- `PUT /transactions/:txId` 已开放且是**全量替换 + 类型不可改**（改类型=删除+重建），R1 不得伪造类型变更。

### 5.3 后端 API 权限 / DTO / 测试矩阵（源码取证重做）

> **本矩阵为 R0 二次整改重做版，整体替换 [1.62] 初稿**。全部 `file:line` 均以 §0.2 所列**业务代码证据基准** `bff6a2b`（非运行时采集时历史文档 HEAD `557e4d2`、非当前报告版本、非截图运行时二进制）的源码实读为准；权限来自真实 service / guard 函数，DTO 来自 handler 与 struct 定义，错误码追溯 `mapSvcErr` / `mapLoginErr` / `mapAuthErr` / OCR 独立映射，测试证据列具体测试函数 + `file:line` + 断言类型，未找到独立命名测试的接口明确写「未覆盖」。

#### 5.3.0 取证基线

- **业务代码证据基准 = `bff6a2b351bf5a039cdf7c762ec2693fb330cd00`**（§0.2；执行时 `git rev-parse HEAD` 复核，`fetch` 已复核 0/0；运行时采集时历史文档 HEAD 为 `557e4d2`（非当前报告版本），两者不同且不可互相推定）。本矩阵所有 `file:line` 均以该提交为准，路径相对仓库根 `<repo-root>/`。
- **本矩阵是静态源码取证，不是运行时验证**：权限判定、DTO 字段、错误码映射、测试函数名与断言类型均来自业务代码证据基准 `bff6a2b` 的源码实读（`Read` / `Grep`），**本矩阵自身未做逐路由集成测试运行确认**；本轮二次整改已在 §7.6 真实执行 `go test ./...`（10 包 `ok` / 2 包无测试文件 / `FAIL` 0 / `EXIT_CODE=0`），但按任务单口径 **全量绿不等于逐路由测试覆盖证据**，不得用它或「CI 绿」替代逐用例断言。凡「未经集成测试运行确认」的部分逐项标注：测试证据列中「**未覆盖：未找到独立命名测试**」表示该接口在 `server/**/*_test.go` 中未找到独立命名的测试用例；已列出测试名的行仅表示**源码中存在该命名的测试用例及其断言类型**，不代表本次已执行通过。主表「测试证据」列不出现「CI 绿」「待测」等笼统结论（§5.3.3 与附录引用旧表错值属逐条纠错对照，不作本表结论）。
- **路径前缀**：除 `GET /healthz`(router.go:109) 外，全部路由挂在 `v1 := r.Group("/api/v1")`(router.go:122) 下，真实路径 = `/api/v1` + 组内路径。旧表把 WS 写作 `/ws` 属错值（见 §5.3.3 #1）。
- **鉴权分组**：公开 v1 组 `register/login/join`(router.go:124/125/128)；`GET /api/v1/ws` 用独立 `wsAuth(d.AuthService,d.PATs)`(router.go:133，刻意不挂 JWTAuth 组，注释 130-133，浏览器 WS 无法设 Authorization 头，需 Header→`?token=` 回退)；其余全部 `auth := v1.Group("", JWTAuth(d.AuthService, d.PATs))`(router.go:135)。`JWTAuth` 解析 `Authorization: Bearer`，`flt_` 前缀走 PAT 分支且 `patSvc==nil` fail-closed 401(middleware.go:52-93)。全局 `gin.Recovery()`(55)、`r.SetTrustedProxies(nil)`(52-54)、`NoRoute(newWebHandler())`(105-107)。**注册开关** `enableRegister = d.Config.EnableRegister`(router.go:74)，关闭时 403(auth_handler.go:51-54)；**PAT 开关** `if ph != nil`(141，`d.PATs==nil` 不注册 94-97)。
- **条件挂载（按「路由已注册」计，生产 Deps 全量注入）**：`if ph != nil`(141)、`if rh != nil`(205，`d.Reports==nil` 不挂载 79-82)、`if bh2 != nil`(212，`d.Budgets==nil` 不挂载 84-87)、`if ih != nil`(220，`d.Invest==nil` 不挂载 89-92)。
- **角色常量**：`model.go:8-10` → `RoleOwner=1 / RoleAdmin=2 / RoleMember=3`。
- **响应包装**：`ok(c,data)` → `c.JSON(200,{"code":0,"msg":"ok","data":data})`(common.go:17-19)；`fail(c,status,msg)` → `{"code":status,"msg":msg}`(common.go:22-24，**失败响应无 data 字段**)。例外：`GET /api/v1/books/:id/export` 直接 `c.Data(200,ctype,body)`(report_handler.go:138) 文件流 + 响应头，不走信封；`GET /healthz` 用 `c.JSON(200, gin.H{...})`(router.go:119) 直写同形信封。
- **绑定口径**：`bindJSON`=`c.ShouldBindJSON`(common.go:27-33，失败 400「请求格式错误」)；路径 ID 用 `bookID()`(book_handler.go:19) / `pathID()`(ledger_handler.go:39)；查询参数用 `queryInt64/queryInt`(ledger_handler.go:49/63)。**`binding:"required"` 仅出现在 `service.CreateRequest.clientUuid`(tx.go:100) 与 `service.UpdateRequest.clientUuid`(tx_update.go:38)**；`PushItem`(sync.go:114-126) 刻意不带；其余必填/枚举/默认值在 service 层校验（bookType 默认 family `book.go:46-48`、months 1~24 默认 6 `report.go:76-81`、CreateInvite role 默认 member `member.go:354-356`）。
- **错误码列口径**：`M`=`mapSvcErr`(common.go:39-136)、`L`=`mapLoginErr`(**auth_handler.go:103-122，不在 common.go**)、`A`=`mapAuthErr`(middleware.go:34-40)、`O`=OCR 独立映射(ocr_handler.go:112-123 / 96-107)。列内写「常量→HTTP『文案』(file:line)」，完整全表见 §5.3.2。`ok()` 信封与 `fail()` 无 data、`export` 文件流三处例外已在响应列逐行标注。
- **Web 接入判定口径（按路由逐条，三类）**：`✓` = 在 `server/web/dist/app.js` 中找到该路由的直接 `api("METHOD", path)` / `fetch(API+path)` 调用；`间接` = 无直接调用但前端以其它方式达到等价效果；`未接入` = 全文无该路由调用。前端 `const API="/api/v1"`(app.js:9)、`fetch(API+path,opt)`(app.js:102)、`return data.data`(app.js:111)，与后端 `v1` 组及 `ok()` 信封一一对齐，路径比对无前缀歧义；`api()` 返回的 `data.data` 即 `ok()` 信封 `data` 字段，故前端直接取 `data.accessToken`(app.js:194)、`data.code`(app.js:1665)。
  - **统计（本次以 `router.go` 源码实读 + `app.js` grep 结果重新逐条计数，不引用任何二手数字）**：`router.go` L109–235 共 **63 条**注册路由（含 `GET /healthz`）：`✓` 直接接入 **47** 条、`间接` **1** 条、`未接入` **15** 条。**不含 `/healthz`** 的 62 条业务路由：`✓` 47、`间接` 1、`未接入` 14。
  - `/healthz` 是**非 UI 的公开健康检查端点**：计入 63 条路由登记，但**不纳入 UI 控件验收分母**；§3.1 按「控件」计，不单列 `/healthz` 控件行（无 UI 控件），仅作 R0-E-17「设置页缺服务器/备份区块」证据链备注。两者分母不同、结论一致，**禁止把本表「未接入」条数与 §3.1 控件条目数直接对齐**。
  - `#19 GET /api/v1/books/:id` 判为「**间接**」：前端从未单独调用 `GET /books/{id}`；`app.js:342` 等处以 `books.find((b)=>b.id===bookId)` 从 `GET /books` 列表本地取 book 对象，并调用 `/books/:id/...` 子资源。前端源码中 `renderHome`/`openBookFlow` 仅调用 `/books/:id/...` 子资源，未出现单独 `GET /books/{id}` 调用。旧表该行 Web 列写 `✅`，应更正为「间接」。
  - `#38 GET /api/v1/books/:id/transactions/:txId` 归类「**口径对齐**」：前端 `app.js:1982` 已单独调用该详情接口（含失败分支 L1983），与列表 `GET /transactions` 分列，不是同一行合并；本表补后端锚点 `router.go:194`，与前端 `app.js:1982` 形成双向锚点，Web 判定 `✓` 不变。

#### 5.3.1 主表：router.go L109–235 全部 63 条注册路由（每条独立一行，不按能力域合并省略）

> 权限列写真实 service 方法名 + guard 函数 + `file:line`；无账本成员判定的域明确写「仅 JWTAuth，无账本角色判定」。请求/响应字段来自 handler 与 DTO struct 实读。

| 能力域 | 方法 & 路径（router.go:行） | 鉴权 | 权限判定（服务函数 + file:line） | 允许角色 | 请求关键字段 | 响应关键字段（data 内） | 错误码（映射来源） | 测试证据 | Web 接入 |
|---|---|---|---|---|---|---|---|---|---|
| 健康 | `GET /healthz` (109) | 公开（无 v1 前缀） | 无鉴权、无角色判定（匿名 handler 只读 `d.SchemaVersion`/`d.Instance`） | — | — | `{status:"up",schemaVersion,instanceId?,restoreEpoch?}`；`c.JSON` 直写同形信封(router.go:119-120) | — | 未覆盖：未找到独立命名测试 | 未接入（非 UI 端点，不计入 UI 控件分母） |
| 鉴权-注册 | `POST /api/v1/auth/register` (124) | 公开 v1；**注册开关** `enableRegister`(74)，关闭→403(auth_handler.go:51-54) | 仅公开入口，**无账本角色判定**；`AuthService.Register`(auth.go) | 任意（受开关） | `{username,password}`(auth_handler.go:55-58) | **`currentUserDTO`**=`{id,username,email,isInstanceAdmin}`(middleware.go:197-204)；**不返回 accessToken** | `M`: ErrUserExists→409『用户名已存在』(common.go:109-110)；ErrWeakPassword→400『密码至少 8 位』(120-121)；ErrPasswordTooLong→400『密码过长：最多 72 字节…』(122-125)；ErrUsernameTooLong→400(126-128)；ErrAuthUnavailable→503(129-131)；default→500『服务内部错误』(132-134)。开关关闭→403 | `TestR45RegisterInputErrorsReturn400` api/auth_handler_r45_test.go:137（DTO 校验 400）；`TestR45RegisterPassesRawInputToService`:190（透传）；`TestR45RegisterDisabledReturns403`:219（开关 403）；`TestR45RegisterInternalErrorStays500ButLeaksNothing`:205（500 不泄露） | ✓ app.js:192 |
| 鉴权-登录 | `POST /api/v1/auth/login` (125) | 公开 v1 | 仅公开入口，**无账本角色判定**；`AuthService.LoginFromIP`(auth.go) | 任意 | `{username,password}`(auth_handler.go:73-76) | **仅 `{accessToken}`**(auth_handler.go:88)；不含 user/currentUserDTO | `L`(auth_handler.go:103-122): ErrBadCredentials/ErrAccountDisabled→401『用户名或密码错误』(105-107)；ErrRateLimited→429『登录尝试过于频繁，请稍后再试』(108-114)；ErrAuthUnavailable→503『服务暂时不可用，请稍后重试』(115-117)；default→500『服务内部错误，请稍后重试』(118-120) | `TestR46CredentialErrorsAreUniform401` api/auth_handler_r46_test.go:26（401 统一防枚举）；`TestA6RateLimitedIs429` api/auth_handler_a6_test.go:23（429）；`TestR46LoginSuccessUnchanged`:134（正常）；`TestR46BadJSONStill400`:147（DTO 校验 400） | ✓ app.js:193,209 |
| 邀请入户 | `POST /api/v1/books/:id/members/join` (128) | 公开 v1（注释 127：不受 EBK_ENABLE_REGISTER 限制） | **未认证入口，无成员资格/角色判定**；`MemberService.JoinViaInvite`(member.go:774)，IP 限流 10 次/小时(782-784) | 持邀请码的任意访客 | **`{code,username,email,password}`**(book_handler.go:186-191，JSON body；旧表误写 `code` 在 query) | **`currentUserDTO`**(book_handler.go:202)；**不返回 accessToken / 不签发 token** | `M`: ErrInviteInvalid→400『邀请码无效或已过期』(common.go:111-112)；ErrInviteBoundMismatch→400『邀请码已绑定指定用户，信息不匹配』(113-114)；ErrBookFull→409『该账本成员已达上限（20）』(115-117)；ErrRateLimited→429(118-119)；ErrUserExists→409(109-110) | `TestJoinViaInviteSignatureRequiresBookAndIP` service/member_test.go:92（签名/账本存活）；`TestInviteJoinRateLimitIsPerIP`:49（限流分桶）；`TestR41JoinAcceptsEmail` api/book_handler_r41_test.go:75（DTO 新字段）；`TestR43JoinChecksBookInsideTransaction` service/remediation_invite_unit_test.go:559（账本存活） | 未接入（后端已交付 join 路由，Web 无兑换入口；app.js `join` 仅 L1055 `fields.join(",")`，renderLogin 仅 login/register 两模式 L227，L1667 仅说明文字） |
| WS | `GET /api/v1/ws` (133) | **独立 `wsAuth`**(router.go:133；Header→`?token=` 回退 middleware.go:112-164)，不在 JWTAuth 组 | 仅握手鉴权，**无账本角色判定**；订阅阶段由 hub checker `CanViewDetail` 把关 | 任意已认证用户 | `Authorization: Bearer` 或 `?token=`(middleware.go:119) | WS 帧协议（`ws.ServeWS`） | `A`/JWTAuth: 无 token→401『缺少登录凭证』(middleware.go:119)；缺 Authorization→401『缺少登录凭证』(56)；格式错→401『凭证格式错误』(62)；ErrTokenRevoked→401『会话已撤销或已失效，请重新登录』(middleware.go:35-38) | `TestQAS3WSAuthNeverBypasses` api/qa_stage3_test.go:74（wsAuth 不可绕过）；`TestWSAuthQueryTokenFallback` api/stage3_test.go:60（?token= 回退）；`TestSubscribeRejectsNonMember` ws/hub_test.go:84（越权）；`TestQAS3ServeWSSubscribeAndBroadcast` ws/qa_servews_test.go:58（正常） | 未接入（app.js 全文无 WebSocket / `/ws` 客户端；R0-E-15/25 关联） |
| 当前用户 | `GET /api/v1/users/me` (136) | JWTAuth | **仅 JWTAuth，无账本角色判定**；`AuthService.GetByID`(auth.go) | 本人 | — | `currentUserDTO`=`{id,username,email,isInstanceAdmin}`(auth_handler.go:193；middleware.go:197-204) | `A`: 其余→401『登录已失效，请重新登录』(middleware.go:39)；ErrTokenRevoked→401『会话已撤销或已失效，请重新登录』(35-38) | 未覆盖：未找到独立命名测试（仅 `TestPATLifecycleAPI` api/token_handler_test.go:123 以 PAT 间接断言 200+username） | ✓ app.js:1684,2157 |
| 当前用户 | `PUT /api/v1/users/me` (138) | JWTAuth | **仅 JWTAuth，无账本角色判定**；`AuthService.UpdateEmail`(profile.go)；用户名不可改(auth_handler.go:196-199) | 本人 | `{email}`(auth_handler.go:201-203) | `currentUserDTO`(auth_handler.go:212) | `M`: ErrEmailTaken→409『该邮箱已被其他账号使用』(common.go:90-91)；ErrInvalidArgument→400『请求参数非法』(92-96) | 未覆盖：未找到独立命名测试（仅 service `TestUpdateEmail` service/profile_test.go:13） | ✓ app.js:2172 |
| 改密 | `POST /api/v1/users/me/password` (139) | JWTAuth | **仅 JWTAuth，无账本角色判定**；`AuthService.ChangePassword`(profile.go) 撤销全部会话 | 本人 | `{oldPassword,newPassword}`(auth_handler.go:220-223) | **`{revokedSessions:int, message:"密码已修改，所有设备已下线，请重新登录"}`**(auth_handler.go:232) | `M`: ErrWeakPassword→400『密码至少 8 位』(common.go:120-121)；ErrPasswordTooLong→400(122-125)；ErrInvalidArgument→400(92-96)；ErrBadCredentials→401(经 mapSvcErr 无此分支，实际由 service 返回) | 未覆盖：未找到独立命名测试（仅 service `TestChangePassword` service/profile_test.go:57；`{revokedSessions,message}` 响应体无专门断言） | 未接入（R0-E-18；`password` 仅命中登录注册表单 L179/184/192/193/209） |
| PAT | `GET /api/v1/users/me/tokens` (142) | JWTAuth；**条件** `if ph != nil`(141) | **仅 JWTAuth，无账本角色判定**；`PATService.List`(token.go) | 本人 | — | 裸 `[]PATItem`=`{id,name,createdAt,expiresAt,revokedAt,lastUsedAt}`(token_handler.go:36；token.go:55-63，**不含哈希/明文**) | `A`: 401(middleware.go:39) | `TestPATLifecycleAPI` api/token_handler_test.go:95（端到端：列表无哈希明文） | ✓ app.js:1574 |
| PAT | `POST /api/v1/users/me/tokens` (143) | JWTAuth；条件 `ph != nil` | **仅 JWTAuth，无账本角色判定**；`PATService.Create` | 本人 | `{name}`(token_handler.go:40-56) | **`{token:明文, item:PATItem}`**(token_handler.go:52-55，明文仅此一次) | `M`: ErrInvalidArgument→400『请求参数非法』(common.go:92-96)；ErrPATNotFound→404『Token 不存在』(87-89) | `TestPATLifecycleAPI` api/token_handler_test.go:95（`{token,item}`）；`TestPATCreateAndVerify` service/token_test.go:16（正常） | ✓ app.js:1543 |
| PAT | `DELETE /api/v1/users/me/tokens/:tokenId` (144) | JWTAuth；条件 `ph != nil` | **仅 JWTAuth，无账本角色判定**；`PATService.Revoke` | 本人 | `c.Param("tokenId")`→`pathID`(token_handler.go:60) | `{revoked:true}`(token_handler.go:68) | `M`: ErrPATNotFound→404『Token 不存在』(common.go:87-89，跨用户/不存在/已撤销统一 404) | `TestPATLifecycleAPI` api/token_handler_test.go:95（撤销后 401、跨用户 404） | ✓ app.js:1605 |
| OCR | `POST /api/v1/ocr/recognize` (146) | JWTAuth | **仅 JWTAuth，无账本角色判定**；`OCRService.Recognize` | 任意已认证 | multipart `FormFile("image")`(ocr_handler.go:40)，限 maxBytes+64KB(23) | `*OCRRecognizeResult`=`{engine,lines:[{text,confidence}]}`(ocr.go:32-35) | `O`(ocr_handler.go:112-123): ErrOCRRateLimited→429『OCR 请求过于频繁，请稍后再试』；ErrOCRImageTooLarge→413『图片超过大小限制』；ErrOCRInvalid→400『请求参数非法』；default→503『OCR 服务暂时不可用，请稍后重试』；内联 24-30/51-53 另有 413/400 | `TestOCRE2EWithValidJWT` api/ocr_handler_test.go:130（正常）；`TestOCRRoutesRegisteredBehindAuth`:21（401）；`TestRecognizeDownstreamContract` service/ocr_test.go:85（DTO 字段） | 未接入（PRD §5.2 明确 Web 不做 OCR；R0-E-08） |
| OCR | `POST /api/v1/ocr/parse` (147) | JWTAuth | **仅 JWTAuth，无账本角色判定**；`OCRService.Parse` | 任意已认证 | `parseOCRRequest{text}`(ocr_handler.go:16-18) | `*OCRParseResult`=`{candidates:[{amountCents,direction,occurredDate,description,confidence}]}`(ocr.go:36-45) | `O`: ErrOCRInvalid→400『请求参数非法』(ocr_handler.go:112-123)；default→503 | `TestParseOCRTextRules` service/ocr_test.go:15（规则）；`TestParseResponseShape`:187（DTO 字段） | 未接入（R0-E-08） |
| OCR | `POST /api/v1/ocr/llm-parse` (149) | JWTAuth（默认关闭，需 `consent=true`，注释 148） | **仅 JWTAuth，无账本角色判定**；`OCRService.LLMParse` | 任意已认证 | `llmParseRequest{text,consent:bool}`(ocr_handler.go:80-83)，consent 必须 true(97-98) | `*OCRParseResult`(ocr_handler.go:110) | `O`(ocr_handler.go:96-107): ErrLLMConsentRequired→400（明示同意文案）；ErrLLMDisabled→503『云端文本兜底未启用（服务端未配置）』；ErrOCRRateLimited→429；ErrOCRInvalid→400；default→503 | `TestLLMParseRouteBehindAuth`/`ConsentAndDisabled`/`UpstreamOK` api/ocr_llm_handler_test.go:14,35,70（401 / consent 400+未启用 503 / 正常）；`TestLLMParseHappyPath` service/ocr_llm_test.go:70 | 未接入（R0-E-08） |
| 会话 | `GET /api/v1/auth/sessions` (154) | JWTAuth | **仅 JWTAuth，无账本角色判定**；`SessionService.List` | 本人 | query `includeRevoked=="1"`(session_handler.go:75) | **`{sessions:[{id,jti,createdAt,expiresAt,revokedAt,lastSeenAt,userAgent,current}]}`**(session_handler.go:85；sessionDTO 48-57)；**非裸数组** | `A`: 401(middleware.go:39) | `TestSessionListMarksCurrent` api/session_api_test.go:209（正常 current 标记） | 未接入（R0-E-14；`sessions` 全无命中） |
| 会话 | `DELETE /api/v1/auth/sessions` (155) | JWTAuth | **仅 JWTAuth，无账本角色判定**；`SessionService.RevokeOthers` | 本人 | — | **`{revoked:int, jtiList:[]}`**(session_handler.go:133) | `A`: 401(middleware.go:39) | `TestRevokeOthersKeepsCurrent` api/session_api_test.go:324（正常）；`TestRevokeOtherSessionsKeepsCurrent` service/session_test.go:227 | 未接入（R0-E-14） |
| 会话 | `DELETE /api/v1/auth/sessions/:jti` (156) | JWTAuth | **仅 JWTAuth，无账本角色判定**；`SessionService.RevokeOne` | 本人 | `c.Param("jti")` | **`{revoked:true,jti,revokedAt,current}`**(session_handler.go:110-115)；跨用户/不存在 404(101-104) | `ErrSessionNotFound`→404『会话不存在』(session_handler.go:101-104，**不经 mapSvcErr**)；`A`: 401 | `TestRevokeSessionThenHTTP401` api/session_api_test.go:237（撤销后 401）；`TestRevokeSessionCrossUserIs404`:305（越权 404）；`TestRevokeSessionCrossUserIsNotFound` service/session_test.go:170 | 未接入（R0-E-14） |
| 账本-列表 | `GET /api/v1/books` (158) | JWTAuth | `BookService.List`(book.go:107)：SQL `INNER JOIN book_members m ... WHERE m.uid=?`(112-114) 隐式成员过滤，**无 guard 函数调用** | 全体成员（仅返回所属） | — | `[]model.Book`(book_handler.go:50；`make(...,0,8)` 保证空数组非 null book.go:108-110)；字段 `{id,name,ownerUid,bookType,defaultCurrency,membersCanViewOthers,deleted,createdAt,updatedAt}`(model.go:43-53) | `A`: 401(middleware.go:39) | 未覆盖：未找到独立命名测试（`TestRouterRegistersLedgerRoutes` api/router_test.go:13 仅覆盖路由注册） | ✓ app.js:241 等多处 |
| 账本-创建 | `POST /api/v1/books` (159) | JWTAuth | `BookService.Create`(book.go:40)：**无成员资格判定**（任何登录用户）；创建者写 `RoleOwner`(74-76) | 任意登录用户 | `{name,bookType}`(book_handler.go:29-32)；bookType 非 personal/family 默认 family(46-48)、默认 CNY(61)、`MembersCanViewOthers:true`(62) | `*model.Book`(book_handler.go:41) | `M`: ErrInvalidArgument→400『请求参数非法』(common.go:92-96)；ErrUserExists 不适用 | `TestAuthzBookCreateMustLeaveAudit` authzreview/authz_review_test.go:191（创建留审计）；`TestEventEnvelope*` api/stage3_rework_test.go:122（WS 事件契约） | ✓ app.js:295 |
| 账本-详情 | `GET /api/v1/books/:id` (160) | JWTAuth | `bh.get` 先 `MemberService.RoleOf`(book_handler.go:59-62，member.go:112-122) 成员资格复核，再 `BookService.Get`(book.go:119，仅 `deleted=0` 查库，**无角色判定**) | 全体成员 | `c.Param("id")`→`bookID` | `*model.Book`(book_handler.go:68) | `M`: ErrBookNotFound→404『账本不存在』(common.go:66-67)；ErrNotMember→403『无权限』(43-46) | 未覆盖：未找到独立命名测试（单资源 GET；见 §5.3.1 主表对应行） | **间接**（前端无单独 `GET /books/{id}`；以 `GET /books` 列表 + `books.find((b)=>b.id===bookId)` app.js:342 本地取 book，并调 `/books/:id/...` 子资源） |
| 账本-改名 | `PUT /api/v1/books/:id` (161) | JWTAuth | `BookService.Update`(book.go:174)：`loadBookTx`(184，先 404) → **`roleOfTx` guard.go:35-48**(188) → `if role!=RoleOwner && role!=RoleAdmin → ErrForbidden`(192-194)；可见性字段委托 `SetVisibility(...,sess)`(231) 同事务单写 | **owner/admin** | `{name, membersCanViewOthers:*bool}`(book_handler.go:82-85，指针区分未传/显式 false) | **`{id}`**(book_handler.go:97)；仅 `res.VisibilityChanged` 时广播(94-96) | `M`: ErrForbidden→403『无权限』(common.go:43-46)；ErrBookNotFound→404『账本不存在』(66-67)；ErrEntityDeleted→409『目标已被删除，无法对该数据进行操作』(51-53) | `TestR33BookUpdateNameAndVisibilityAtomic` service/remediation_integration_test.go:735（原子性）；`TestR33VisibilityBroadcastOnlyAfterCommit` api/remediation_r33_test.go:20（广播时机）；`TestR33UpdateUsesSingleServiceCall`:57（单写实现） | 未接入（R0-E-32；前端无 `PUT /books/:id`） |
| 账本-删除 | `DELETE /api/v1/books/:id` (162) | JWTAuth | `BookService.Delete`(book.go:355)：**`roleOfTx` guard.go:35-48**(363) → `if role != model.RoleOwner → ErrForbidden`(367-369)；注释 342 明确「admin 不可删除账本」 | **仅 owner**（不是 owner/admin） | `c.Param("id")` | **`{id}`**(book_handler.go:112) | `M`: ErrForbidden→403『无权限』(common.go:43-46)；ErrBookNotFound→404(66-67) | `TestAuthzBookDeleteMustLeaveAudit` authzreview/authz_review_test.go:150（审计）；`TestAuthzCanViewDetailDeletedBookMustFail`:230（已删账本失败关闭） | ✓ app.js:271 |
| 账本-可见性 | `PUT /api/v1/books/:id/visibility` (165) | JWTAuth | `BookService.SetVisibility`(book.go:278)：`loadBookTx`(299) → **`roleOfTx` guard.go:35-48**(303) → `if role!=RoleOwner && role!=RoleAdmin → ErrForbidden`(307-309)；不递增 sync_version(265-267)、幂等(310-313)、审计 config_change(329) | **owner/admin** | `visibilityRequest{membersCanViewOthers:*bool}`(visibility_handler.go:18-20)；缺字段 400(31-35) | **`{id, membersCanViewOthers, changed}`**(visibility_handler.go:47) | `M`: ErrForbidden→403『无权限』(common.go:43-46)；ErrBookNotFound→404(66-67) | `TestVisibilityRequiresField` api/stage3_test.go:200（DTO 校验 400 缺 membersCanViewOthers）；`TestR28BookVisibilityRejectsStaleRole` service/remediation_integration_test.go:414（事务内重读角色/TOCTOU）；`TestSetVisibilityDoesNotBumpVersion` service/visibility_test.go:21；`TestCanViewDetailFailsClosed`:79 | 未接入（R0-E-19/27；`canViewOthers` 仅 L1719-1720 只读徽标） |
| 成员-列表 | `GET /api/v1/books/:id/members` (167) | JWTAuth | `MemberService.List`(member.go:197)：`requireActiveBookTx`(200，账本存活)；**handler 侧另加 `RoleOf` 成员复核**(book_handler.go:120) | 全体成员 | `c.Param("id")` | `[]MemberDTO`=`{uid,username,role,canViewOthers,isInstanceAdmin}`(member.go:140-146；book_handler.go:129) | `M`: ErrNotMember→403『无权限』(common.go:43-46)；ErrEntityDeleted→409(51-53) | 未覆盖：未找到独立命名测试 | ✓ app.js:1683 |
| 成员-邀请 | `POST /api/v1/books/:id/members/invite` (168) | JWTAuth | `MemberService.CreateInvite`(member.go:353)：`requireActiveBookTx`(377，**先 404 后 403**) → **`roleOfTx` guard.go:35-48**(380) → `CanManageMembers` member.go:130-132(384)；role 非 admin/member 默认 member(354-356)、邮箱/用户名二选一(1041-1051)、限流 5 次/分钟(361-363) | **owner/admin** | `{role:int8,boundUsername,boundEmail}`(book_handler.go:139-143) | **`{code,role,expiresAt}`**(book_handler.go:152-156)；**无 inviteId**（前端只消费 `data.code` app.js:1665 → R0-E-26） | `M`: ErrForbidden→403『无权限』(common.go:43-46)；ErrInviteBoundMismatch→400(113-114)；ErrRateLimited→429(118-119)；ErrBookFull→409(115-117) | `TestR41CreateInviteAcceptsBoundEmail` api/book_handler_r41_test.go:53（DTO 新字段）；`TestR41EmailBindingNormalizedAndMatched` service/remediation_invite_integration_test.go:140（正常）；`TestR28CreateInviteRejectsStaleRole` service/remediation_integration_test.go:451（TOCTOU）；`TestMapSvcErrInviteFailuresDoNotLeak` api/invite_lifecycle_api_test.go:90（错误码不泄露） | ✓ app.js:1658 |
| 成员-撤邀请 | `POST /api/v1/books/:id/members/invite/:inviteId/revoke` (175) | JWTAuth（动作段风格，避开与 `DELETE .../:uid` 路由树冲突，注释 169-175） | `MemberService.RevokeInvite`(member.go:441)：**`roleOfTx` guard.go:35-48**(449)；**跨账本伪装 404**(451-463，注释「统一表现为邀请不存在，防枚举」)；本账本权限不足**保留 403**(466-470)；`CanManageMembers`(466)；条件 UPDATE + RowsAffected(483-495) | **owner/admin**（跨账本→404、本账本 member→403） | `c.Param("inviteId")`→`pathID` | **`{inviteId:int64, revoked:true}`**(book_handler.go:176) | `M`: ErrForbidden→403『无权限』(common.go:43-46)；ErrInviteInvalid→400『邀请码无效或已过期』(111-112)；ErrInviteNotFound 包装 ErrInviteInvalid→400(member.go:99-109) | `TestR40RevokeInviteRouteRegistered`/`RejectsInvalidID`/`RejectsInvalidBookID` api/invite_lifecycle_api_test.go:29,53,73（路由/400）；`TestR40RevokeRequiresManagerRole` service/remediation_invite_integration_test.go:364（越权 403）；`TestR40CrossBookRevokeRejected`:433（跨账本 404）；`TestR40RevokeAndRedeemRaceExactlyOneWins`:455（并发互斥） | 未接入（R0-E-26；`revoke` 仅 token 撤销 L1605、`URL.revokeObjectURL` L1085；`inviteId` 全无命中） |
| 成员-改角色 | `PUT /api/v1/books/:id/members/:uid` (176) | JWTAuth | `MemberService.ChangeRole`(member.go:240)：**`roleOfTx` guard.go:35-48**(257) + `CanManageMembers` member.go:130-132(261)；newRole 只允许 admin/member(241-244)；`targetRole==RoleOwner→ErrCannotOperateOwner`(268-270)；admin 不能动 owner(271-273)；`n!=1→ErrConcurrentConflict`(281-285) | **owner/admin，且不能操作/提为 owner** | `c.Param("uid")` + `{role:int8}`(book_handler.go:215-217) | **`{uid,role}`**(book_handler.go:227) | `M`: ErrCannotOperateOwner→403『不能对账本所有者执行该操作』(common.go:47-48)；ErrForbidden→403(43-46)；ErrConcurrentConflict→409『数据已被并发修改，请重试』(64-65) | `TestChangeRoleWritesAudit` service/audit_integration_test.go:87（审计）；`TestR28MemberChangeRoleRejectsStaleRole` service/remediation_integration_test.go:356（事务内重读角色） | ✓ app.js:1727,1737 |
| 成员-移除 | `DELETE /api/v1/books/:id/members/:uid` (177) | JWTAuth | `MemberService.Remove`(member.go:299)：**`roleOfTx` guard.go:35-48**(308) + `CanManageMembers`(312)；`targetRole==RoleOwner→ErrCannotOperateOwner`(319-321)；`n!=1→409`(330-333) | **owner/admin，不能移除 owner** | `c.Param("uid")` | **`{uid}`**(book_handler.go:246) | `M`: ErrCannotOperateOwner→403『不能对账本所有者执行该操作』(common.go:47-48)；ErrConcurrentConflict→409(64-65) | `TestRemoveMemberWritesAudit` service/audit_integration_test.go:108（审计）；`TestR28MemberRemoveRejectsStaleRole` service/remediation_integration_test.go:388（TOCTOU） | ✓ app.js:1752 |
| 账户-列表 | `GET /api/v1/books/:id/accounts` (180) | JWTAuth | `AccountService.List`(account.go:75)：**`requireMember` guard.go:51-55**(76)（只读路径）；**不叠加交易可见性过滤** | 全体成员 | `c.Param("id")` | `[]model.Account`=`{id,bookId,name,category,currency,balance,parentId,syncVersion,deleted,createdAt,updatedAt}`(model/ledger.go:111-122) | `M`: ErrNotMember→403『无权限』(common.go:43-46)；ErrBookNotFound→404(66-67) | `TestWA8AccountAudit` service/audit_r2_test.go:228（审计）；`TestAuthzAccountAndCategoryWritesMustLeaveAudit` authzreview/authz_review_test.go:551 | ✓ app.js:308 |
| 账户-创建 | `POST /api/v1/books/:id/accounts` (181) | JWTAuth | `AccountService.Create`(account.go:103)：**`requireManagerTx` guard.go:59-76**(123)；附 `assertNoAccountCycle`(132) | **owner/admin** | `accountCreateReq{name,category,currency,parentId}`(ledger_handler.go:73-78)；前端仅 `{name,category,currency}`、**无 Balance/期初余额**(app.js:737 → R0-E-06/30，服务固定 Balance:0) | `*model.Account`(ledger_handler.go) | `M`: ErrForbidden→403『无权限』(common.go:43-46)；ErrAccountCycle→400『账户层级不能成环：上级不能是自己的下级』(97-99)；ErrAccountInvalid→400(92-96) | `TestR47AccountCycleDetected` service/account_r47_test.go:61（400 成环）；`TestR47ConcurrentOppositeMovesCannotFormCycle`:201（并发） | ✓ app.js:742 |
| 账户-详情 | `GET /api/v1/books/:id/accounts/:aid` (182) | JWTAuth | `AccountService.Get`(account.go:87)：**`requireMember` guard.go:51-55**(88) | 全体成员 | `c.Param("aid")`→`pathID` | `*model.Account` | `M`: ErrAccountNotFound→404『账户不存在』(common.go:68-69)；ErrNotMember→403(43-46) | 未覆盖：未找到独立命名测试（单资源 GET；见 §5.3.1 主表对应行） | ✓ app.js:1779 |
| 账户-更新 | `PUT /api/v1/books/:id/accounts/:aid` (183) | JWTAuth | `AccountService.Update`(account.go:179)：**`requireManagerTx` guard.go:59-76**(187) | **owner/admin** | `accountUpdateReq{name:*string,parentId:*int64}`(ledger_handler.go:80-83) | `*model.Account` | `M`: ErrAccountCycle→400(common.go:97-99)；ErrAccountNotFound→404(68-69)；ErrForbidden→403(43-46) | `TestR47ConcurrentOppositeMovesCannotFormCycle` service/account_r47_test.go:201（并发）；`TestWA8AccountAudit` service/audit_r2_test.go:228 | ✓ app.js:741 |
| 账户-删除 | `DELETE /api/v1/books/:id/accounts/:aid` (184) | JWTAuth | `AccountService.Delete`(account.go:260)：**`requireManagerTx` guard.go:59-76**(268) | **owner/admin**（不是 owner-only，也不是「成员」） | `c.Param("aid")` | **`{id:accountID}`**(ledger_handler.go:181) | `M`: ErrAccountInUse→409『账户仍存在交易记录，无法删除』(common.go:100-101)；ErrAccountHasChildren→409『该账户下仍有子账户，无法删除』(102-104)；ErrAccountNotFound→404(68-69) | 未覆盖：未找到独立命名测试（HTTP 层；service `TestR31AccountDeleteBlockedBySoftDeletedTx` service/remediation_integration_test.go:594 覆盖 409 引用约束、`TestR53AccountDeleteRejectsActiveChildren` service/remediation_unit_test.go:501 覆盖 409 子账户） | ✓ app.js:687 |
| 分类-列表 | `GET /api/v1/books/:id/categories` (187；`?tree=1|"true"` 走 Tree) | JWTAuth | `CategoryService.List`(category.go:63)：**`requireMember` guard.go:51-55**(64)；`Tree`(75) 委托 `List`(76) | 全体成员 | query `tree=1|true`(category_handler.go:27) | 默认 `[]model.Category`=`{id,bookId,name,parentId,catType,syncVersion,deleted,createdAt,updatedAt}`(model/ledger.go:129-138)；`?tree=1` → `[]CategoryTreeNode`=嵌入 `model.Category` + `{children:[]model.Category}`(category.go:57-60) | `M`: ErrNotMember→403『无权限』(common.go:43-46)；ErrBookNotFound→404(66-67) | 未覆盖：未找到独立命名测试（`?tree=1` 树形分支无专门测试，`CategoryTreeNode` 无专门断言；见 §5.3.1 主表对应行） | ✓ app.js:309；`?tree=1` app.js:2089 |
| 分类-创建 | `POST /api/v1/books/:id/categories` (188) | JWTAuth | `CategoryService.Create`(category.go:110)：**`requireManagerTx` guard.go:59-76**(126) | **owner/admin** | `categoryCreateReq{name,parentId,catType}`(category_handler.go:10-14) | `*model.Category` | `M`: ErrCategoryInvalid→400『请求参数非法』(common.go:92-96)；ErrForbidden→403(43-46) | `TestWA8CategoryAudit` service/audit_r2_test.go:288（审计）；`TestWA8RejectedWritesLeaveNoAudit`:345（拒绝写） | ✓ app.js:2049 |
| 分类-更新 | `PUT /api/v1/books/:id/categories/:cid` (189) | JWTAuth | `CategoryService.Update`(category.go:168)：**`requireManagerTx` guard.go:59-76**(176) | **owner/admin** | `categoryUpdateReq{name:*string,parentId:*int64,catType:*int8}`(category_handler.go:16-20) | `*model.Category` | `M`: ErrCategoryNotFound→404『分类不存在』(common.go:70-71)；ErrForbidden→403(43-46) | `TestWA8CategoryAudit` service/audit_r2_test.go:288（审计） | ✓ app.js:2048 |
| 分类-删除 | `DELETE /api/v1/books/:id/categories/:cid` (190) | JWTAuth | `CategoryService.Delete`(category.go:255)：**`requireManagerTx` guard.go:59-76**(263) | **owner/admin** | `c.Param("cid")` | **`{id:categoryID}`**(category_handler.go:110) | `M`: ErrCategoryInUse→409『分类仍存在交易记录，无法删除』(common.go:105-106)；ErrCategoryHasChildren→409『该分类下仍有子分类，无法删除』(107-108)；ErrCategoryNotFound→404(70-71) | 未覆盖：未找到独立命名测试（HTTP 层；service `TestR31CategoryDeleteBlockedBySoftDeletedTx` service/remediation_integration_test.go:657 覆盖 409 引用约束） | ✓ app.js:2066 |
| 交易-列表 | `GET /api/v1/books/:id/transactions` (193) | JWTAuth | `TxService.List`(tx.go:504)：`canViewAllForTx` visibility.go:87-92(505，内含 `requireMember`) + `txVisibilityCond` visibility.go:103-108(512) | 全体成员 + 可见性过滤（owner/admin 全看；member 关互看时只看本人 `creator_uid=? AND creator_uid<>0`） | query `maxTime,minTime,type,accountId,categoryId,keyword,page,count`(tx_handler.go:43-98；**参数名是 `type` 不是 `txType`**)；`type` 越界 400(60-63) | **`{items:[]model.Transaction,total,page,count}`**(tx_handler.go:97)；`model.Transaction`(model/ledger.go:152-173) 含 `creatorUid,txType,amount,direction,clientUuid,syncVersion,deleted` | `M`: ErrNotMember→403『无权限』(common.go:43-46)；ErrInvalidArgument→400(92-96)；`A`: 401 | `TestQATransactionRoutesContract` api/qa_api_test.go:263（路由契约）；`TestListFilterByTypeQuotedColumn` service/tx_integration_test.go:317（列表过滤）；`TestProbeCore2VisibilityAppliesToRecycleBin` service/core_consistency_probe2_test.go:363（可见性）；`TestQAS3VisibilityCondMatrix` service/qa_stage3_test.go:37（源码守卫） | ✓ app.js:399,1033,1878,2058 |
| 交易-详情 | `GET /api/v1/books/:id/transactions/:txId` (194) | JWTAuth | `TxService.Get`(tx.go:569)：`canViewAllForTx` visibility.go:87-92(570) + `txVisibilityCond` visibility.go:103-108(576)；不可见→`ErrTransactionNotFound`→404(585-587，刻意不泄露存在性) | 全体成员 + 可见性过滤 | `c.Param("txId")` | `*model.Transaction`(tx_handler.go:114)；**无 merchant 字段 → R0-E-37** | `M`: ErrTransactionNotFound→404『交易不存在』(common.go:72-73)；ErrNotMember→403(43-46) | 未覆盖：未找到独立命名测试（单资源 GET；见 §5.3.1 主表对应行，CRUD flow 间接覆盖） | ✓ app.js:1982（**口径对齐**：前端已单列详情 GET，与列表分列；本表补后端锚点 router.go:194 形成双向锚点） |
| 交易-创建 | `POST /api/v1/books/:id/transactions` (195) | JWTAuth | `TxService.Create`(tx.go:147)：**`requireMemberTx` guard.go:14-27**(170)，ErrNotMember 转 `ErrCreateForbidden`(171-173) | **全体成员**（不区分角色） | `service.CreateRequest`(tx.go:99-111)=`{clientUuid(binding:"required"),txType,accountId,toAccountId,amount,categoryId,paymentChannel,txTime,timezoneOffset,comment,source}`；**无 merchant 字段(R0-E-37)** | `*CreateResult`(tx.go:114-119)=`{resultType,transactionIds,version,replayed}` | `M`: ErrCreateConflict→409『同一 clientUuid 已用于不同内容，请检查请求或更换 clientUuid』(common.go:49-50)；ErrInvalidCreate→400『请求参数非法』(92-96)；ErrAccountCycle 不适用；ErrCreateForbidden→403(43-46) | `TestQACreateTransactionRejectsBadBody` api/qa_api_test.go:204（DTO 校验 400，实读 205-231 断言 `w.Code==400`）；`TestValidateCreateRequiresClientUuid` service/tx_test.go:36（DTO 校验 400）；`TestIdempotencySameClientUUIDNoDoubleCharge` service/tx_integration_test.go:222（幂等）；`TestConcurrentDistinctClientUUIDConsistentBalance`:285（并发） | ✓ app.js:572 |
| 交易-更新 | `PUT /api/v1/books/:id/transactions/:txId` (197) | JWTAuth | `TxService.UpdateReceipt`(tx_update.go:59)：**`roleOfTx` guard.go:35-48**(80) + `canEditOthers := CanEditOthersTx(role)` member.go:135-137(87) | **owner/admin 可改他人；member 仅本人** | `service.UpdateRequest`(tx_update.go:37-49)=CreateRequest 11 字段 + `{clientUuid(binding:"required"),...,baseVersion:int64}`；全量替换、类型不可改 | `*UpdateResult`(tx_update.go:52-56)=`{version,replayed,transactionIds}` | `M`: ErrConcurrentConflict→409『数据已被并发修改，请重试』(common.go:64-65，baseVersion 乐观锁)；ErrForbidden→403(43-46)；ErrCreateConflict→409(49-50)；ErrTransactionNotFound→404(72-73) | `TestUpdateSingleAmountNetBalance` service/tx_update_test.go:77（正常）；`TestUpdateReplayIdempotent`:266（幂等）；`TestUpdatePermission`:306（越权）；`TestUpdateDeletedTx404`:338（404）；`TestUpdateBaseVersionOptimisticLock`:353（409 乐观锁）；`TestUpdateValidation`:377（DTO 校验） | ✓ app.js:633 |
| 交易-删除 | `DELETE /api/v1/books/:id/transactions/:txId` (199) | JWTAuth | `DeleteReceipt`(tx.go:755) → `mutateVisibility`(tx.go:780)：**`roleOfTx` guard.go:35-48**(805) + `CanEditOthersTx` member.go:135-137(813)；`for _, r := range group { if !canEditOthers && r.CreatorUid != uid → ErrForbidden }`(848-852) | **owner/admin 可删他人；member 仅本人** | **`?clientUuid=` 走 `c.Query`**(DELETE 不带 body，tx_handler.go:149-150 注释)；前端 `uuid()` 生成(app.js:506,2020) | **`{version:int64}`**(tx_handler.go:173) | `M`: ErrForbidden→403『无权限』(common.go:43-46)；ErrTransactionNotFound→404(72-73)；ErrTxDirectionMissing→409『该交易记录的方向标记异常，无法删除或恢复…』(54-58)；ErrTransferGroupIncomplete→409(59-61) | `TestDeletePermissionMemberCannotDeleteOthers` service/tx_delete_integration_test.go:212（**越权 403**，member 删他人 ErrForbidden、admin 可删，实读 219-238）；`TestDeleteIdempotentSameClientUUID`:81（幂等）；`TestR28TxDeleteRestoreRejectStaleRole` service/remediation_integration_test.go:292（TOCTOU） | ✓ app.js:506,2020 |
| 交易-恢复 | `POST /api/v1/books/:id/transactions/:txId/restore` (200) | JWTAuth | `RestoreReceipt`(tx.go:769) → `mutateVisibility`(tx.go:780)：同删除，`roleOfTx`(805) + `CanEditOthersTx`(813) + 逐行判定(848-852) | **owner/admin 可恢复他人；member 仅本人** | **`?clientUuid=` 走 `c.Query`**(tx_handler.go:179-197)；前端 `uuid()`(app.js:782) | **`{version:int64}`**(tx_handler.go:196) | `M`: ErrForbidden→403(43-46)；ErrTransferGroupIncomplete→409(59-61)；ErrIncompleteReceipt→409(62-63) | `TestRestoreReversesDelete` service/tx_delete_integration_test.go:185（正常）；`TestR26RestoreRejectsBrokenPairNoPartialRestore` service/remediation_integration_test.go:181（409 配对损坏）；`TestR32RestoreClearsDeletedAt`:477（墓碑） | ✓ app.js:782 |
| 回收站 | `GET /api/v1/books/:id/recycle` (202) | JWTAuth | `TxService.ListDeleted`(recycle.go:23)：`canViewAllForTx` visibility.go:87-92(24) + `txVisibilityCond` visibility.go:103-108(30) | 全体成员 + 可见性过滤 | query `page,count`(前端 `?page=1&count=100` app.js:766) | **`{items:[]model.Transaction,total,page,count}`**(tx_handler.go:222)；**分页对象，非裸 `[tx]` 数组** | `M`: ErrNotMember→403『无权限』(common.go:43-46)；ErrInvalidArgument→400(92-96) | `TestRecycleBinListsDeleted` service/tx_delete_integration_test.go:242（正常）；`TestProbeCore2VisibilityAppliesToRecycleBin` service/core_consistency_probe2_test.go:363（可见性）；`TestQAS3VisibilityCondWiredIntoAllReadPaths` service/qa_stage3_test.go:407（源码守卫） | ✓ app.js:766（消费 `data.items`） |
| 报表-月报 | `GET /api/v1/books/:id/reports/monthly` (206) | JWTAuth；条件 `if rh != nil`(205) | `ReportService.Monthly`(report.go:71)：`canViewAllForTx` visibility.go:87-92(72) + `txVisibilityCond` visibility.go:103-108(90) | 全体成员 + 可见性过滤 | query `months`（service 归一化 1~24、默认 6，report.go:76-81） | **`{items:[]MonthPoint}`**，MonthPoint=`{month:"2006-01",income:int64,expense:int64}`(report.go:53-57；report_handler.go:33) | `M`: ErrNotMember→403『无权限』(common.go:43-46)；ErrInvalidArgument→400(92-96) | `TestReportRoutesBehindAuth` api/report_handler_test.go:151（401）；`TestReportMonthlyAndCategoriesAPI`:180（正常 DTO）；`TestReportMonthlyAggregates` service/report_test.go:46（聚合）；`TestReportMonthlyVisibility`:77（可见性） | ✓ app.js:831,1949 |
| 报表-分类占比 | `GET /api/v1/books/:id/reports/categories` (207) | JWTAuth；条件 `rh != nil` | `ReportService.CategoryShare`(report.go:116)：`canViewAllForTx`(124) + `txVisibilityCond`(130)；txType 只允许收入/支出(117-119) | 全体成员 + 可见性过滤 | query `month`（缺省=当月 CST，report_handler.go:41-45）、`type` | **`{month,items:[]CategorySlice}`**，CategorySlice=`{categoryId,name,total,count}`(report.go:60-65；report_handler.go:55)；**旧表误写 `{categoryId,amount,ratio}`** | `M`: ErrNotMember→403(43-46)；ErrInvalidArgument→400(92-96) | `TestReportMonthlyAndCategoriesAPI` api/report_handler_test.go:180（正常 DTO）；`TestReportCategoryShare` service/report_test.go:126（占比） | ✓ app.js:871 |
| 导出 | `GET /api/v1/books/:id/export` (208) | JWTAuth；条件 `rh != nil` | `ReportService.ExportRows`(report.go:186)：`canViewAllForTx`(187) + `txVisibilityCond`(193) | 全体成员 + 可见性过滤 | query `format=csv|json`(report_handler.go:66-70)、`type,accountId,categoryId,minTime,maxTime,fields`（`fields` 经 `service.ParseFields` 校验 95-99） | **直接文件流 `c.Data(200,ctype,body)`**(report_handler.go:138)，**不走 `ok()` 信封**；头 `X-Export-Truncated:"1"`(113)、`X-Export-Count`(116)、`Content-Disposition: attachment; filename=...`(136-137)、`Content-Type: text/csv; charset=utf-8` 或 `application/json; charset=utf-8`(124/127)；错误仍走 `fail()` 信封。`ExportRow`(report.go:177-182) | `M`: ErrNotMember→403(43-46)；ErrInvalidArgument→400(92-96)；format/fields 非法→400(report_handler.go:66-70,95-99) | `TestExportAPICsvAndJson` api/report_handler_test.go:234（文件流 + 响应头：Content-Type text/csv、CSV BOM、Content-Disposition、fields 白名单、format/fields 非法 400、导出零副作用，实读 239-291）；`TestExportRowsAndTruncation` service/report_test.go:171（截断）；`TestBuildCSVAndSanitize`:227（CSV 转义） | ✓ app.js:1062（**原生 fetch**，因取文件流不走 `api()` 的 JSON 解包） |
| 预算-列表 | `GET /api/v1/books/:id/budgets` (213) | JWTAuth；条件 `if bh2 != nil`(212) | `BudgetService.List`(budget.go:240)：**`requireMember` guard.go:51-55**(241) + `canViewAllForTx` visibility.go:87-92(251) + `spent()` 内 `txVisibilityCond`(309) | 全体成员 + 已用按可见性过滤 | `c.Param("id")` | **`{items:[]BudgetView}`**，BudgetView=嵌入 `model.Budget` + `{categoryName,spent:int64,percent:int,status:"normal|near|over"}`(budget.go:60-66；budget_handler.go:25) | `M`: ErrNotMember→403(43-46)；ErrInvalidArgument→400(92-96) | `TestBudgetRoutesBehindAuth` api/budget_handler_test.go:111（401，4 个路由，实读 113-124）；`TestBudgetCRUDFlow`:127（列表 `{items}`、删除后 `items:[]`）；`TestBudgetProgressVisibility` service/budget_test.go:199（可见性） | ✓ app.js:1129 |
| 预算-创建 | `POST /api/v1/books/:id/budgets` (214) | JWTAuth；条件 `bh2 != nil` | `BudgetService.Create`(budget.go:92)：**`requireManagerTx` guard.go:59-76**(103) | **owner/admin** | `budgetReq{name,budgetType,categoryId,periodType,amount,rollover,enabled:*bool}`(budget_handler.go:28-36) | `*model.Budget`(model/budget.go:25-37；budget_handler.go:60) | `M`: ErrBudgetConflict→409『该分类已存在生效预算（或总预算已设置）』(common.go:85-86)；ErrForbidden→403(43-46)；ErrInvalidArgument→400(92-96) | `TestBudgetCRUDFlow` api/budget_handler_test.go:127（正常 / **409 重复** / `periodType=2`→400）；`TestBudgetPermissionsAndValidation` service/budget_test.go:74（**越权 403 + DTO 校验 400**） | ✓ app.js:1226 |
| 预算-更新 | `PUT /api/v1/books/:id/budgets/:budgetId` (215) | JWTAuth；条件 `bh2 != nil` | `BudgetService.Update`(budget.go:150)：**`requireManagerTx` guard.go:59-76**(164) | **owner/admin** | 仅 `name/amount/rollover/enabled`(budget_handler.go:76-81)，**不接受 budgetType/categoryId/periodType** | `*model.Budget`(budget_handler.go:86) | `M`: ErrBudgetNotFound→404『预算不存在』(common.go:74-75)；ErrForbidden→403(43-46) | `TestBudgetCRUDFlow` api/budget_handler_test.go:127；`TestBudgetPermissionsAndValidation` service/budget_test.go:74 | ✓ app.js:1220 |
| 预算-删除 | `DELETE /api/v1/books/:id/budgets/:budgetId` (216) | JWTAuth；条件 `bh2 != nil` | `BudgetService.Delete`(budget.go:203)：**`requireManagerTx` guard.go:59-76**(211) | **owner/admin** | `c.Param("budgetId")` | **`{deleted:true}`**(budget_handler.go:102) | `M`: ErrBudgetNotFound→404(74-75)；ErrForbidden→403(43-46) | `TestBudgetCRUDFlow` api/budget_handler_test.go:127（删除后 `items:[]`）；`TestBudgetPermissionsAndValidation` service/budget_test.go:74 | ✓ app.js:1169 |
| 投资-标的列表 | `GET /api/v1/books/:id/instruments` (221) | JWTAuth；条件 `if ih != nil`(220) | `InvestService.ListInstruments`(invest.go:213)：**`requireMember` guard.go:51-55**(214) | 全体成员 | `c.Param("id")` | **`{items:[]model.Instrument}`**(invest_handler.go:27；model/invest.go:36-46) | `M`: ErrNotMember→403(43-46)；ErrBookNotFound→404(66-67) | `TestInvestRoutesBehindAuth` api/invest_handler_test.go:109（401）；`TestInvestCRUDAndNetWorthFlow`:128（端到端）；`TestInstrumentCrudAndReferenceProtection` service/invest_test.go:18（引用保护） | ✓ app.js:1344,1469 |
| 投资-标的创建 | `POST /api/v1/books/:id/instruments` (222) | JWTAuth；条件 `ih != nil` | `InvestService.CreateInstrument`(invest.go:77)：**`requireManagerTx` guard.go:59-76**(90) | **owner/admin**（旧表误写「成员」） | `instrumentReq{name,type,market,code}`(invest_handler.go:30-35) | `*model.Instrument` | `M`: ErrForbidden→403(43-46)；ErrInvalidArgument→400(92-96) | `TestInvestCRUDAndNetWorthFlow` api/invest_handler_test.go:128；`TestInstrumentCrudAndReferenceProtection` service/invest_test.go:18 | ✓ app.js:1452 |
| 投资-标的更新 | `PUT /api/v1/books/:id/instruments/:instrumentId` (223) | JWTAuth；条件 `ih != nil` | `InvestService.UpdateInstrument`(invest.go:119)：**`requireManagerTx` guard.go:59-76**(132) | **owner/admin** | `instrumentReq`(invest_handler.go:30-35) | `*model.Instrument` | `M`: ErrInstrumentNotFound→404『标的不存在』(common.go:76-77)；ErrForbidden→403(43-46) | `TestInvestCRUDAndNetWorthFlow` api/invest_handler_test.go:128 | ✓ app.js:1449 |
| 投资-标的删除 | `DELETE /api/v1/books/:id/instruments/:instrumentId` (224) | JWTAuth；条件 `ih != nil` | `InvestService.DeleteInstrument`(invest.go:169)：**`requireManagerTx` guard.go:59-76**(176) | **owner/admin** | `c.Param("instrumentId")` | **`{deleted:true}`**(invest_handler.go:92) | `M`: ErrInstrumentInUse→409『该标的下仍有持仓记录，无法删除』(common.go:80-81)；ErrInstrumentNotFound→404(76-77) | `TestInvestCRUDAndNetWorthFlow` api/invest_handler_test.go:128（重复删→404）；`TestInstrumentCrudAndReferenceProtection` service/invest_test.go:18 | ✓ app.js:1381 |
| 投资-行情 | `POST /api/v1/books/:id/instruments/:instrumentId/price` (225) | JWTAuth；条件 `ih != nil` | `InvestService.SetPrice`(invest.go:475)：**`requireManagerTx` guard.go:59-76**(492) | **owner/admin**（旧表误写「成员」） | **`{price:string, priceTime:int64}`**(invest_handler.go:105-108；旧表误写 `price,asOf?`) | **`{updated:true}`**(invest_handler.go:116；旧表误写 `{}`) | `M`: ErrInvalidPrice→400『数量或价格非法（十进制字符串，数量>0、价格≥0）』(common.go:82-84)；ErrInstrumentNotFound→404(76-77)；ErrForbidden→403(43-46) | 未覆盖：未找到独立命名测试（HTTP 层；service `TestSetPriceRefreshesHoldings` service/invest_test.go:103、api `TestInvestCRUDAndNetWorthFlow` api/invest_handler_test.go:167 间接覆盖） | ✓ app.js:1509 |
| 投资-持仓列表 | `GET /api/v1/books/:id/holdings` (226) | JWTAuth；条件 `ih != nil` | `InvestService.ListHoldings`(invest.go:422)：**`requireMember` guard.go:51-55**(423) | 全体成员 | `c.Param("id")` | **`{items:[]HoldingView}`**，HoldingView=嵌入 `model.Holding` + `{instrumentName,instrumentType,marketValue:int64,profitLoss:int64}`(invest.go:413-419；invest_handler.go:131)；**单条市值字段名是 `marketValue`，不是 `holdingMarketValue`** | `M`: ErrNotMember→403(43-46)；ErrBookNotFound→404(66-67) | `TestInvestCRUDAndNetWorthFlow` api/invest_handler_test.go:128（市值 135000）；`TestHoldingDecimalAndValidation` service/invest_test.go:52（DTO 校验） | ✓ app.js:1283,1797 |
| 投资-持仓创建 | `POST /api/v1/books/:id/holdings` (227) | JWTAuth；条件 `ih != nil` | `InvestService.AddHolding`(invest.go:235)：**`requireManagerTx` guard.go:59-76**(262) | **owner/admin**（旧表误写「成员」） | `holdingReq{accountId,instrumentId,quantity:string,costAmount:int64,currentPrice:string}`(invest_handler.go:134-140；旧表误写 `cost`) | `*model.Holding`(model/holding.go:11-22) | `M`: ErrInvalidQuantity→400『数量或价格非法…』(common.go:82-84)；ErrInstrumentNotFound→404(76-77)；ErrForbidden→403(43-46) | `TestInvestCRUDAndNetWorthFlow` api/invest_handler_test.go:128（`quantity:"abc"`→400）；`TestHoldingDecimalAndValidation` service/invest_test.go:52 | ✓ app.js:1497 |
| 投资-持仓更新 | `PUT /api/v1/books/:id/holdings/:holdingId` (228) | JWTAuth；条件 `ih != nil` | `InvestService.UpdateHolding`(invest.go:318)：**`requireManagerTx` guard.go:59-76**(342) | **owner/admin** | `holdingReq`(invest_handler.go:134-140) | `*model.Holding` | `M`: ErrHoldingNotFound→404『持仓不存在』(common.go:78-79)；ErrInvalidQuantity→400(82-84) | `TestInvestCRUDAndNetWorthFlow` api/invest_handler_test.go:128 | ✓ app.js:1492 |
| 投资-持仓删除 | `DELETE /api/v1/books/:id/holdings/:holdingId` (229) | JWTAuth；条件 `ih != nil` | `InvestService.DeleteHolding`(invest.go:376)：**`requireManagerTx` guard.go:59-76**(383) | **owner/admin** | `c.Param("holdingId")` | **`{deleted:true}`**(invest_handler.go:198) | `M`: ErrHoldingNotFound→404(78-79)；ErrForbidden→403(43-46) | `TestInvestCRUDAndNetWorthFlow` api/invest_handler_test.go:128（重复删→404） | ✓ app.js:1330 |
| 投资-净资产 | `GET /api/v1/books/:id/net-worth` (230) | JWTAuth；条件 `ih != nil` | `InvestService.NetWorth`(invest.go:546)：**`requireMember` guard.go:51-55**(547)；只读 holdings/accounts，不过滤可见性 | 全体成员 | `c.Param("id")` | **`*NetWorthSummary`**=`{assets:[]NetWorthEntry,liabilities:[]NetWorthEntry,holdingsValue:int64,totalAssets:int64,totalLiabilities:int64,netWorth:int64}`(invest.go:535-542；invest_handler.go:213)；NetWorthEntry=`{category,total,count}`(528-532)；**字段是 `holdingsValue`，无 `holdingMarketValue`** | `M`: ErrNotMember→403(43-46)；ErrBookNotFound→404(66-67) | `TestInvestCRUDAndNetWorthFlow` api/invest_handler_test.go:128（**net-worth `holdingsValue`**，实读 196-209 断言 `json:"holdingsValue"`）；`TestNetWorthFormula` service/invest_test.go:141（净资产公式） | ✓ app.js:1255（消费 `nw.holdingsValue` L1273） |
| 同步-拉 | `GET /api/v1/books/:id/sync/pull` (234) | JWTAuth | `SyncService.Pull`(sync.go:303)：**`requireMemberTx` guard.go:14-27**(345，非成员 403) + `canViewDetailTx` visibility.go:51-81(349，同快照可见性) + `txVisibilityCond` visibility.go:103-108(399，仅交易叠加)；先限流(309) | 全体成员 + 可见性过滤 | query **`afterVersion,limit`**(sync_handler.go:24-43；旧表误写 `since`) | `*SyncBatch`(sync.go:91-106)=`{highVersion,transactions:[]model.Transaction,accounts:[]model.Account,categories:[]model.Category,holdings:[]model.Holding,restoreEpoch,instanceId}`；**旧表误写「增量实体」** | `M`: ErrNotMember→403『无权限』(common.go:43-46)；ErrInvalidArgument→400(92-96)；ErrRateLimited→429『操作过于频繁，请稍后再试』(118-119) | `TestPullRejectsBadLimit` api/stage3_test.go:189（整批 DTO 校验 400）；`TestQAS3PullUninitializedServiceIs400` api/qa_stage3_test.go:105（400）；`TestSyncBatchJSONContract` service/sync_test.go:218（DTO 契约）；`TestR48MergeSafeVersion*` service/sync_r48_test.go:28,101,157,233（批次边界/版本） | 未接入（R0-E-15/25；`/sync` 全无命中） |
| 同步-推 | `POST /api/v1/books/:id/sync/push` (235) | JWTAuth | `SyncService.Push`(sync.go:430)：**无独立权限函数**，逐条委托 `s.txs.Create(...)`(458) → 继承 `TxService.Create` 的 `requireMemberTx` guard.go:14-27（全体成员）；整批非法 items 空/超长/未初始化→ErrInvalidArgument 400(440-448) | 全体成员 | **`pushRequest{items:[]service.PushItem}`**(sync_handler.go:50-52)；PushItem(sync.go:114-126)=与 CreateRequest 同 12 字段但**不带 binding 标签**（避免单条缺 clientUuid 升级整批 400） | **`{results:[]PushResult}`**(sync_handler.go:84)；PushResult(sync.go:146-161)=`{clientUuid,status,id,transactionIds?,version,errorCode?,message?}`，status 枚举 `created/replayed/conflict/invalid/forbidden/error`(149)；**旧表误写 `{ok}`** | `M`: ErrInvalidArgument→400『请求参数非法』(common.go:92-96)；ErrRateLimited→429(118-119)；逐条 status 内 `invalid/forbidden/error` 不升级整批 HTTP | `TestPushRejectsEmptyItems`/`TooManyItems`/`BadJSONBody` api/stage3_test.go:146,157,178（整批 DTO 校验 400）；`TestPushRejectsWholeBatchInvalid` service/sync_test.go:165（400）；`TestPushPerItemIsolation` service/qa_stage3_test.go:407（逐条隔离）；`TestZZ6PushThrottleCountsPerCall` authzreview/zz_authz_adversarial_test.go:297（限流按调用计） | 未接入（R0-E-15/25；`/sync` 全无命中） |


#### 5.3.2 错误映射全表（主表「错误码」列的展开依据）

**`M` = `mapSvcErr`(common.go:39-136) 完整映射**（逐条为源码实读，非「✗待测」）：

| 错误常量（`service.*`） | HTTP | msg 文案 | common.go:行 |
|---|---|---|---|
| ErrForbidden / ErrNotMember / ErrCreateForbidden | 403 | 无权限 | 43-46 |
| ErrCannotOperateOwner | 403 | 不能对账本所有者执行该操作 | 47-48 |
| ErrCreateConflict | 409 | 同一 clientUuid 已用于不同内容，请检查请求或更换 clientUuid | 49-50 |
| ErrEntityDeleted | 409 | 目标已被删除，无法对该数据进行操作 | 51-53 |
| ErrTxDirectionMissing | 409 | 该交易记录的方向标记异常，无法删除或恢复，请联系实例管理员修复数据 | 54-58 |
| ErrTransferGroupIncomplete | 409 | 该转账记录配对不完整，无法整笔删除或恢复，请联系实例管理员修复数据 | 59-61 |
| ErrIncompleteReceipt | 409 | 该 clientUuid 已有不完整回执，请更换 clientUuid 或联系实例管理员处理 | 62-63 |
| ErrConcurrentConflict | 409 | 数据已被并发修改，请重试 | 64-65 |
| ErrBookNotFound | 404 | 账本不存在 | 66-67 |
| ErrAccountNotFound | 404 | 账户不存在 | 68-69 |
| ErrCategoryNotFound | 404 | 分类不存在 | 70-71 |
| ErrTransactionNotFound | 404 | 交易不存在 | 72-73 |
| ErrBudgetNotFound | 404 | 预算不存在 | 74-75 |
| ErrInstrumentNotFound | 404 | 标的不存在 | 76-77 |
| ErrHoldingNotFound | 404 | 持仓不存在 | 78-79 |
| ErrInstrumentInUse | 409 | 该标的下仍有持仓记录，无法删除 | 80-81 |
| ErrInvalidQuantity / ErrInvalidPrice | 400 | 数量或价格非法（十进制字符串，数量>0、价格≥0） | 82-84 |
| ErrBudgetConflict | 409 | 该分类已存在生效预算（或总预算已设置） | 85-86 |
| ErrPATNotFound | 404 | Token 不存在 | 87-89 |
| ErrEmailTaken | 409 | 该邮箱已被其他账号使用 | 90-91 |
| ErrInvalidCreate / ErrInvalidArgument / ErrAccountInvalid / ErrCategoryInvalid | 400 | 请求参数非法 | 92-96 |
| ErrAccountCycle | 400 | 账户层级不能成环：上级不能是自己的下级 | 97-99 |
| ErrAccountInUse | 409 | 账户仍存在交易记录，无法删除 | 100-101 |
| ErrAccountHasChildren | 409 | 该账户下仍有子账户，无法删除 | 102-104 |
| ErrCategoryInUse | 409 | 分类仍存在交易记录，无法删除 | 105-106 |
| ErrCategoryHasChildren | 409 | 该分类下仍有子分类，无法删除 | 107-108 |
| ErrUserExists | 409 | 用户名已存在 | 109-110 |
| ErrInviteInvalid | 400 | 邀请码无效或已过期 | 111-112 |
| ErrInviteBoundMismatch | 400 | 邀请码已绑定指定用户，信息不匹配 | 113-114 |
| ErrBookFull | 409 | 该账本成员已达上限（20） | 115-117 |
| ErrRateLimited | 429 | 操作过于频繁，请稍后再试 | 118-119 |
| ErrWeakPassword | 400 | 密码至少 8 位 | 120-121 |
| ErrPasswordTooLong | 400 | 密码过长：最多 72 字节（超出部分 bcrypt 会丢弃） | 122-125 |
| ErrUsernameTooLong | 400 | 用户名长度超限：1~64 个字符 | 126-128 |
| ErrAuthUnavailable | 503 | 服务暂时不可用，请稍后重试 | 129-131 |
| default | 500 | 服务内部错误（不回显内部错误） | 132-134 |

**`L` = `mapLoginErr`（`auth_handler.go:103-122`，**不在 common.go**）**：`ErrBadCredentials`/`ErrAccountDisabled`→401『用户名或密码错误』(105-107，不区分账号存在性)；`ErrRateLimited`→429『登录尝试过于频繁，请稍后再试』(108-114，含节流日志 `logRateLimited`)；`ErrAuthUnavailable`→503『服务暂时不可用，请稍后重试』(115-117)；default→500『服务内部错误，请稍后重试』(118-120)。3/4 先记安全日志 `logLoginInternalError`(175-185，不记用户名/口令/令牌)。

**`A` = `mapAuthErr`（`middleware.go:34-40`，鉴权中间件统一 401）**：`ErrTokenRevoked`→401『会话已撤销或已失效，请重新登录』(35-38)；其余→401『登录已失效，请重新登录』(39)。`JWTAuth` 自身分支：缺 Authorization→401『缺少登录凭证』(56)、格式错→401『凭证格式错误』(62)、PAT 但 `patSvc==nil`→401(69)；`wsAuth` 无 token→401『缺少登录凭证』(middleware.go:119)。

**`O` = OCR 独立错误映射（不经 mapSvcErr）**：`ocr.writeError`(ocr_handler.go:112-123)：`ErrOCRRateLimited`→429『OCR 请求过于频繁，请稍后再试』；`ErrOCRImageTooLarge`→413『图片超过大小限制』；`ErrOCRInvalid`→400『请求参数非法』；default→503『OCR 服务暂时不可用，请稍后重试』。`llmParse` 内联(96-107)：`ErrLLMConsentRequired`→400（明示同意文案）；`ErrLLMDisabled`→503『云端文本兜底未启用（服务端未配置）』；`ErrOCRRateLimited`→429；`ErrOCRInvalid`→400；default→503。`recognize` 内联(24-30/51-53)：multipart 过大→413『图片请求过大』、格式错→400、超 maxBytes→413『图片超过大小限制』。

**补充**：`ErrSessionNotFound` 在 `session_handler.go:101-104` 单独映射 404『会话不存在』（不经 mapSvcErr）；`ErrInviteAlreadyUsed/AlreadyRevoked/Expired/NotFound/BookUnavailable` 均以 `%w` 包装 `ErrInviteInvalid`(member.go:99-109)，故统一落 400。

#### 5.3.3 22 条易错点逐条纠正（旧 §5.3 → 源码正确值）

| # | 文档 §5.3 错值 | 源码正确值 | 证据 `file:line` |
|---|---|---|---|
| 1 | WS 路径 `/ws` | **`/api/v1/ws`**（挂 `v1` 组下，独立 `wsAuth`，不在 JWTAuth 组） | router.go:122,133；middleware.go:112 |
| 2 | 持仓市值 `holdingMarketValue` | 持仓列表项 = **`marketValue`**(`HoldingView.MarketValue`)；净资产汇总持仓总市值 = **`holdingsValue`**(`NetWorthSummary.HoldingsValue`)；两处都无 `holdingMarketValue` | invest.go:417,538；测试 api/invest_handler_test.go:178(`json:"marketValue"`)、:201(`json:"holdingsValue"`) |
| 3 | 回收站返回 `[tx]` 裸数组 | **分页对象 `{items:[...],total,page,count}`** | tx_handler.go:222；recycle.go:23 |
| 4 | 删除/恢复返回 `{}` | **`{"version": ver}`** | tx_handler.go:173,196 |
| 5 | `sessions` 返回裸数组 / 字段名不符 | **`{sessions:[{id,jti,createdAt,expiresAt,revokedAt,lastSeenAt,userAgent,current}]}`**；`revokeOthers`=`{revoked:int,jtiList:[]}`；`revokeOne`=`{revoked,jti,revokedAt,current}` | session_handler.go:85,110-115,133；sessionDTO 48-57 |
| 6 | `register` 返回 `accessToken` | **返回 `currentUserDTO`**=`{id,username,email,isInstanceAdmin}`；**只有 `login` 返回 `{accessToken}`** | auth_handler.go:69,88；middleware.go:197-204 |
| 7 | `join` 返回 accessToken | **返回 `currentUserDTO`**（与 register 同）；join 是未认证入口，不签发 token | book_handler.go:202；router.go:128 |
| 8 | 改密响应 `{}` | **`{revokedSessions:int, message:"密码已修改，所有设备已下线，请重新登录"}`** | auth_handler.go:232 |
| 9 | 撤邀请响应 `{}` | **`{inviteId:int64, revoked:true}`** | book_handler.go:176 |
| 10 | net-worth 字段 `holdingMarketValue` | **`{assets,liabilities,holdingsValue,totalAssets,totalLiabilities,netWorth}`**，`holdingsValue` 为持仓总市值（分） | invest.go:535-542 |
| 11 | export 返回 `{code,msg,data}` 信封 | **直接文件流**（`c.Data`）+ 头 `Content-Disposition`/`Content-Type`/`X-Export-Count`/可选 `X-Export-Truncated`；错误仍走 `fail()` 信封 | report_handler.go:113-138 |
| 12 | `PUT /books/:id`、`PUT /books/:id/visibility` 响应 `book` | `update`→**`{id}`**；`setVisibility`→**`{id,membersCanViewOthers,changed}`**；`remove`→**`{id}`** | book_handler.go:97,112；visibility_handler.go:47 |
| 13 | `DELETE /books/:id` 角色 = owner/admin | **仅 owner**（`role != RoleOwner` → ErrForbidden）；`Update`/`SetVisibility` 才是 owner/admin | book.go:367-369；book.go:192-194,307-309 |
| 14 | 成员管理角色笼统 owner/admin | `changeRole`/`remove`/`createInvite`/`revokeInvite` = owner/admin(`CanManageMembers`)，且不能操作/提为 owner；`revokeInvite` 跨账本伪装 404、本账本 member 保留 403 | member.go:130-132,261,268-273,312,319-321,384,449-470 |
| 15 | 交易改/删/恢复权限笼统「成员」 | **owner/admin 可改删他人(`CanEditOthersTx`)；member 仅本人**；`Create` 则全体成员(`requireMemberTx`) | member.go:135-137；tx.go:170,805-813,848-852；tx_update.go:80-87 |
| 16 | 写共享配置（accounts/categories/budgets/invest）权限笼统「成员」 | 写 = **owner/admin**(`requireManagerTx`)；读 = **全体成员**(`requireMember`)；可见性读路径另叠加 `canViewAllForTx`+`txVisibilityCond` | guard.go:59-76；account.go:76/123；category.go:64/126；budget.go:241/103；invest.go:90/214 |
| 17 | 报表/导出/回收站/预算已用/sync pull 权限笼统「成员」 | 全部**读路径**：`canViewAllForTx`（内含 requireMember）或 Pull 的 `requireMemberTx`+`canViewDetailTx`，再叠加 `txVisibilityCond`；owner/admin 全看，member 关互看时只看本人 | visibility.go:87-92,103-108；report.go:72/124/187；recycle.go:24；budget.go:251/309；sync.go:345/349/399 |
| 18 | `currentUserDTO` 字段 `id,username,email` | **`{id:int64,username,email,isInstanceAdmin:bool}`**（不含 disabled/deleted/role） | middleware.go:197-204 |
| 19 | `login` 错误一律 401 | 分四类：凭证 401 / 限流 **429** / `ErrAuthUnavailable` **503** / 其余 **500**，且不泄露内部细节 | auth_handler.go:103-122 |
| 20 | 失败响应带 data | `fail()` 只有 `{code:status,msg}`，**无 data** | common.go:22-24 |
| 21 | request 用 `binding:"required"` 全面校验 | 仅 `CreateRequest.clientUuid`(tx.go:100) 与 `UpdateRequest.clientUuid`(tx_update.go:38) 带；`PushItem` 刻意**不带**(sync.go:110-113)；其余在 service 层（bookType 默认 family book.go:46-48、months 1~24 默认 6 report.go:76-81、CreateInvite role 默认 member member.go:354-356） | tx.go:100；tx_update.go:38；sync.go:110-113 |
| 22 | `mapLoginErr` 在 common.go | **在 auth_handler.go:103-122**（common.go 只有 mapSvcErr） | auth_handler.go:103；common.go:39 |

#### 5.3.4 矩阵结论

1. **本表完成的是源码级契约盘点，不等于 R0 通过、不等于运行时验收**。所有权限判定、DTO 字段、错误码映射、测试函数名与断言类型均来自业务代码证据基准 `bff6a2b` 源码实读；**本表未做逐路由集成测试运行确认**；本轮 `go test ./...` 全量绿见 §7.6，按任务单口径不构成逐路由覆盖证据。「测试证据」列只说明源码中存在该命名用例及其断言类型，逐项已在 §5.3.0 声明口径。
2. **已有逐用例测试证据的域**（主表「测试证据」列列出具体 `测试名 file:line（断言类型）`）：auth(register/login，`TestR45*`/`TestR46*`/`TestA6*`，含 DTO 校验 400、开关 403、401 统一、429、500 不泄露)、sessions(`TestSession*`/`TestRevoke*`，正常/越权 404/撤销后 401)、PAT(`TestPATLifecycleAPI` 端到端)、OCR(`TestOCR*`/`TestLLMParse*`/`TestParse*`，401/400/413/429/503/DTO)、books/visibility(`TestVisibilityRequiresField`/`TestR28*`/`TestR33*`/`TestSetVisibility*`/`TestRouterRegisters*`，DTO 校验/TOCTOU/原子性/版本不递增)、members/invites(`TestR40Revoke*`/`TestR41*`/`TestR42*`/`TestR43*`，越权 403/跨账本 404/并发互斥/DTO 校验/审计脱敏)、accounts/categories(`TestR47*`/`TestR31*`/`TestWA8*`，400 成环/并发/409 引用)、transactions/recycle(`TestQACreateTransactionRejectsBadBody`/`TestValidateCreateRequiresClientUuid`/`TestIdempotency*`/`TestConcurrent*`/`TestDeletePermissionMemberCannotDeleteOthers`/`TestUpdate*`/`TestRecycleBinListsDeleted`，DTO 校验 400/幂等/并发/越权 403/409 乐观锁)、reports/export(`TestExportAPICsvAndJson`/`TestReport*`，文件流+响应头/聚合/CSV 转义)、budgets(`TestBudgetCRUDFlow`/`TestBudgetPermissionsAndValidation`，409 重复/越权 403/DTO 校验 400)、invest(`TestInvestCRUDAndNetWorthFlow`/`TestNetWorthFormula`，`holdingsValue` 字段/净资产公式)、sync(`TestPushRejects*`/`TestSyncBatchJSONContract`/`TestR48*`，整批 400/DTO 契约/版本边界)、ws(`TestSubscribeRejectsNonMember`/`TestQAS3WSAuthNeverBypasses`，越权/wsAuth 不可绕过)。
3. **明确「未覆盖：未找到独立命名测试」的接口**（主表逐行标注，非「CI 绿」替代）：`GET /healthz`；`GET /users/me`、`PUT /users/me`、`POST /users/me/password` 的 HTTP handler 层（仅 service `TestUpdateEmail`/`TestChangePassword` 间接覆盖，`{revokedSessions,message}` 无专门断言）；单资源 GET `GET /books/:id`、`GET /books/:id/accounts/:aid`、`GET /books/:id/transactions/:txId`（CRUD flow 间接覆盖）；`GET /books/:id/categories?tree=1` 树形分支（`CategoryTreeNode` 无专门测试）；`DELETE /books/:id/accounts/:aid`、`DELETE /books/:id/categories/:cid` 的 HTTP 层（service 层删除约束有覆盖）；`POST /books/:id/instruments/:instrumentId/price` 的 HTTP 层（service/api 间接覆盖）；`GET /books`、`GET /books/:id/members` 无独立 DTO 断言测试。「未找到独立命名测试」不代表运行时无覆盖，仅表示源码中未找到该接口独立命名的用例。
4. **Web 未接入并关联 R0-E 编号**（按路由逐条，三类统计见 §5.3.0）：改密 `POST /users/me/password`(R0-E-18)；会话 3 条 `GET/DELETE /auth/sessions`、`DELETE /auth/sessions/:jti`(R0-E-14)；可见性 `PUT /books/:id/visibility`(R0-E-19/27)；撤邀请 `POST .../invite/:inviteId/revoke`(R0-E-26，且邀请生成响应无 inviteId 导致 Web 端无法撤销)；同步 `GET /books/:id/sync/pull` 与 `POST /books/:id/sync/push`(R0-E-15/25)；OCR 3 条 `recognize/parse/llm-parse`(Web 不做，R0-E-08)；`PUT /books/:id` 改名(R0-E-32)；WS `GET /api/v1/ws`(R0-E-15/25 关联)；邀请入户 `POST /books/:id/members/join`（后端 `router.go:128` 已交付、Web 完全无兑换入口，与 R0-E-26 分列验收：一个「兑不了」、一个「撤不了」）。`GET /healthz` 为非 UI 端点，计入 63 条路由但不纳入 UI 控件验收分母（R0-E-17 证据链备注）。`GET /books/:id` 判「间接」（前端以 `GET /books` 列表 + `books.find()` 本地取 book，未单独调详情）。
5. **统计口径（本次实读重算，不引用二手数字）**：63 条注册路由（含 `/healthz`）→ `✓` 47 / `间接` 1 / `未接入` 15；不含 `/healthz` 的 62 条业务路由 → `✓` 47 / `间接` 1 / `未接入` 14。§3.1 按「控件」计、分母不同，**禁止与本表未接入条数直接对齐**。
6. **权限口径**：写共享配置（accounts/categories/budgets/invest）为 owner/admin(`requireManagerTx`)，读为全体成员(`requireMember`/`canViewAllForTx`+`txVisibilityCond`)；`DELETE /books/:id` 仅 owner，`Update`/`SetVisibility` owner/admin；交易创建全体成员(`requireMemberTx`)，改/删/恢复 owner/admin 可跨人、member 仅本人(`CanEditOthersTx`)；`POST /books` 无成员资格判定（任何登录用户，创建者写 RoleOwner）。以上均为源码实读的函数级判定，非「按路由语义推定」。

---

#### 附：旧表纠错对照（[1.62] 初稿旧 §5.3 → 新值 → 证据 `file:line`）

> 下表「旧行」列的 L233–L278 等行号指**基准提交 `bff6a2b`（[1.62] 初稿）当时**的文档行号，不是当前文档行号；旧 §5.3 已被本重做版整体替换，此处仅为逐条纠错回溯。表中「✗待测」「CI 绿」等字样均为**旧表错值原文引用**，不作为本轮证据或结论。

> 旧表 L233 表头说明「角色权限列基于路由语义与已知 PRD/服务注释推定」「具体 code↔错误映射待测试补全（标 ✗待测）」「运行时权限边界需集成测试确认」——新表已全部改为**真实 service 函数名 + guard 函数 + `file:line`** 与**具体常量→HTTP→文案**，并删除全部「✗待测」「CI 绿」。旧表 L278 矩阵结论「三类证据仍标注 ✗待测」不再成立。

| 旧行 | 旧表错值 | 新值（本重做版） | 证据 `file:line` |
|---|---|---|---|
| L237 | `/healthz` 未标「非 UI、不计入控件分母」 | 非 UI 公开健康检查，计入 63 条路由但**不纳入 UI 控件验收分母**；Web 判「未接入」 | router.go:109；本表 §5.3.0 口径 |
| L238 | register 响应 `accessToken,user`；错误 `400/409 ✗待测`；测试「CI 绿 ✗逐用例」 | 响应 **`currentUserDTO`**（不返回 accessToken）；错误 `ErrUserExists→409`/`ErrWeakPassword→400`/`ErrPasswordTooLong→400`/开关 403；测试 `TestR45RegisterInputErrorsReturn400` 等 4 个 | auth_handler.go:69；middleware.go:197-204；common.go:109-128；api/auth_handler_r45_test.go:137,190,205,219 |
| L239 | login 响应 `accessToken,user`；错误 `401 ✗待测` | 响应**仅 `{accessToken}`**；错误四类 401/429/503/500；测试 `TestR46*`/`TestA6*` | auth_handler.go:88,103-122；api/auth_handler_r46_test.go:26,65,134,147 |
| L240 | join 请求 `code`(query/body)、响应 `book,member`、错误 `400/403 ✗待测` | 请求 **`{code,username,email,password}`**(JSON body)、响应 **`currentUserDTO`**、错误 `ErrInviteInvalid→400`/`ErrBookFull→409`/`ErrRateLimited→429` | book_handler.go:186-191,202；common.go:111-119；member.go:774 |
| L241 | WS 路径 **`GET /ws`** | **`GET /api/v1/ws`**（`v1` 组 + 独立 `wsAuth`）；补 `TestQAS3WSAuthNeverBypasses` 等 | router.go:122,133；middleware.go:112；api/qa_stage3_test.go:74 |
| L242 | `GET/PUT /users/me` 响应 `id,username,email`（缺 isInstanceAdmin）；测试「CI 绿」 | 响应补 **`isInstanceAdmin`**；测试改为「未覆盖：未找到独立命名测试」 | middleware.go:197-204；auth_handler.go:193,212 |
| L243 | 改密响应 `{}`（撤销全部会话）；测试「✗待测」 | 响应 **`{revokedSessions,message}`**；测试「未覆盖：未找到独立命名测试」 | auth_handler.go:232；service/profile_test.go:57 |
| L244 | PAT 响应 `token,id` | `list` 裸 `[]PATItem`；`create` **`{token,item}`**；`revoke` `{revoked:true}` | token_handler.go:36,52-55,68；token.go:55-63 |
| L245 | 会话 GET 响应 `[{jti,device,createdAt,current}]` | **`{sessions:[{id,jti,createdAt,expiresAt,revokedAt,lastSeenAt,userAgent,current}]}`**（包装对象，非裸数组） | session_handler.go:85；sessionDTO 48-57 |
| L246 | 会话 DELETE 响应 `{}` | `revokeOthers` **`{revoked,jtiList}`**；`revokeOne` **`{revoked,jti,revokedAt,current}`** | session_handler.go:110-115,133 |
| L247 | OCR 错误 `400/413 ✗待测`、测试「✗待测」 | 错误补 **429/503**（`ErrOCRRateLimited`/default）；测试 `TestOCRE2EWithValidJWT` 等 | ocr_handler.go:112-123；api/ocr_handler_test.go:21,130,230 |
| L248 | 账本列表测试「CI 绿」 | 「未覆盖：未找到独立命名测试」（仅路由注册测试间接确认） | api/router_test.go:13 |
| L249 | 建账本错误 `400 ✗待测`、测试「CI 绿」 | `ErrInvalidArgument→400`；测试 `TestAuthzBookCreateMustLeaveAudit` | common.go:92-96；authzreview/authz_review_test.go:191 |
| L250 | 账本详情 Web **`✅`** | **「间接」**（前端无单独 `GET /books/{id}`，以 `GET /books` 列表 + `books.find()` 本地取）；测试「未覆盖」 | app.js:342 等；book_handler.go:59-68 |
| L251 | 改/删账本角色 **`owner/admin`**（合并）、响应 `book`、Web「✅(GET 接…)」 | **Delete 仅 owner**、Update owner/admin；响应 `update`/`remove` 均 **`{id}`**；Web 改/删均**未接入**(R0-E-32) | book.go:192-194,367-369；book_handler.go:97,112 |
| L252 | 可见性请求 `membersCanViewOthers`(bool)、响应 `book` | 请求 **`*bool` 指针**（缺字段 400）、响应 **`{id,membersCanViewOthers,changed}`** | visibility_handler.go:18-20,31-35,47 |
| L253 | 成员列表响应 `[{uid,username,role,canViewOthers}]` | `MemberDTO` 补 **`isInstanceAdmin`** | member.go:140-146 |
| L254 | 邀请测试「✗待测」 | 补 `TestR41CreateInviteAcceptsBoundEmail`/`TestR41EmailBindingNormalizedAndMatched`/`TestR28CreateInviteRejectsStaleRole` | api/book_handler_r41_test.go:53；service/remediation_invite_integration_test.go:140；service/remediation_integration_test.go:451 |
| L255 | 撤邀请响应 `{}` | **`{inviteId,revoked}`**；权限含跨账本 404/本账本 member 403；补 `TestR40*` 系列 | book_handler.go:176；member.go:441-470；service/remediation_invite_integration_test.go:341,364,386,433,455 |
| L256 | 改/删成员响应 `{}`、Web「✅(生成邀请)」 | 响应 `changeRole` **`{uid,role}`**、`removeMember` **`{uid}`**；Web 判定 ✓（改角色/移除成员，非「生成邀请」） | book_handler.go:227,246；app.js:1727,1737,1752 |
| L257 | 账户读权限「成员(受可见性)」 | 读 = **`requireMember` 全体成员，不叠加交易可见性过滤** | account.go:75-76；guard.go:51-55 |
| L258 | 账户写权限「成员(受可见性;删可能 owner)」 | 写（含删）= **owner/admin** `requireManagerTx`，与可见性无关、非 owner-only | account.go:123,187,268；guard.go:59-76 |
| L259 | 分类列表仅列 `?tree=1` 分支、错误 `403 ✗待测` | 补无参数分支 `[]model.Category` 与 `?tree=1` → `[]CategoryTreeNode`；错误 `ErrNotMember→403`/`ErrBookNotFound→404` | category_handler.go:27；category.go:57-60；common.go:43-46,66-67 |
| L260 | 分类写权限「成员」、测试「CI 绿」 | 写 = **owner/admin** `requireManagerTx`；测试 `TestWA8CategoryAudit` | category.go:126,176,263；service/audit_r2_test.go:288 |
| L261 | 交易列表 query 写 `txType` | 实际参数名 **`type`**（`maxTime,minTime,type,accountId,categoryId,keyword,page,count`） | tx_handler.go:43-98 |
| L262 | 交易详情响应含 **`merchant`** | **`model.Transaction` 无 merchant 字段**（R0-E-37）；测试「未覆盖：未找到独立命名测试」 | model/ledger.go:152-173 |
| L263 | 交易创建错误 `400/409 ✗待测`、测试「CI 绿」 | 错误 `ErrCreateConflict→409`/`ErrInvalidCreate→400`；测试 `TestQACreateTransactionRejectsBadBody`/`TestIdempotency*`/`TestConcurrent*` | common.go:49-50,92-96；api/qa_api_test.go:204；service/tx_integration_test.go:222,285 |
| L264 | 交易更新测试仅列 `TestQATransactionRoutesContract` | 补 `TestUpdate*`（正常/幂等/越权/404/409 乐观锁/DTO 校验 400） | service/tx_update_test.go:77,115,145,164,185,242,266,290,306,338,353,377,406 |
| L265 | 删除/恢复请求「—」、响应 **`{}`**、测试「CI 绿」 | 请求 **`?clientUuid=` 走 c.Query**；响应 **`{version}`**；测试 `TestDeletePermissionMemberCannotDeleteOthers`(越权 403)/`TestRestoreReversesDelete` | tx_handler.go:149-150,173,196；service/tx_delete_integration_test.go:212,185 |
| L266 | 回收站响应 **`[tx]`** | **`{items,total,page,count}`** 分页对象 | tx_handler.go:222 |
| L267 | 分类占比响应 `{items:[{categoryId,amount,ratio}]}` | **`{month,items:[{categoryId,name,total,count}]}`** | report.go:60-65；report_handler.go:55 |
| L268 | 导出错误 `403/400 ✗待测`、测试「✗待测」 | 错误 `ErrNotMember→403`/`ErrInvalidArgument→400`；测试 `TestExportAPICsvAndJson`（文件流+响应头） | common.go:43-46,92-96；api/report_handler_test.go:234 |
| L269 | 预算错误 `400/403 ✗待测`、测试「✗待测」 | 错误 `ErrBudgetConflict→409`/`ErrBudgetNotFound→404`；测试 `TestBudgetCRUDFlow`/`TestBudgetPermissionsAndValidation` | common.go:74-75,85-86；api/budget_handler_test.go:111,127；service/budget_test.go:74 |
| L270 | 投资-标的权限「成员」 | 读 = 全体成员 `requireMember`；**写 = owner/admin `requireManagerTx`**（读写分离） | invest.go:90,132,176,214；guard.go:59-76 |
| L271 | 投资-持仓权限「成员」、请求 `instrumentId,quantity,cost,currentPrice?`、响应 `holding` | 写 = owner/admin；请求 **`{accountId,instrumentId,quantity,costAmount,currentPrice}`**；列表响应 **`{items:[]HoldingView}`**（含 `marketValue`） | invest_handler.go:134-140,131；invest.go:413-419；invest.go:262,342,383 |
| L272 | 行情请求 `price,asOf?`、响应 `{}`、权限「成员」 | 请求 **`{price:string,priceTime:int64}`**；响应 **`{updated:true}`**；权限 **owner/admin** | invest_handler.go:105-108,116；invest.go:492 |
| L273 | 净资产 `{totalAssets,totalLiabilities,holdingMarketValue,netWorth}` | **`{assets,liabilities,holdingsValue,totalAssets,totalLiabilities,netWorth}`**（`holdingsValue`，无 `holdingMarketValue`） | invest.go:535-542；api/invest_handler_test.go:196-209 |
| L274 | 同步拉请求 `since`、响应「增量实体」、错误 `403 ✗待测` | 请求 **`afterVersion,limit`**；响应 **`SyncBatch`** 具体字段；错误 `ErrNotMember→403`/`ErrInvalidArgument→400`/`ErrRateLimited→429` | sync_handler.go:24-43；sync.go:91-106；common.go:43-46,92-96,118-119 |
| L275 | 同步推请求「增量实体」、响应 **`{ok}`** | 请求 **`{items:[]PushItem}`**；响应 **`{results:[]PushResult}`**（status 六枚举） | sync_handler.go:50-52,84；sync.go:114-126,146-161 |
| L233/278 | 表头「角色权限列基于路由语义推定」「错误码 ✗待测」「CI 绿」「三类证据仍标注 ✗待测」 | 全部改为真实 service/guard 函数 + `file:line`、具体常量→HTTP→文案、具体测试函数名 + 断言类型；「未覆盖：未找到独立命名测试」逐行标注，**无「CI 绿」「待测」** | 本重做版 §5.3.0–§5.3.4 全文 |

---

## 6. R0-E：逐项差异与根因分类

> 说明：下表为 **R0 盘点项**（gap），不是已定性的缺陷 finding；转入 R1 实施时须按 `规范 §2` 补全 13 字段并分配前缀编号（如 `WB-Rxx`）。`severity` 预标便于排期。

| ID | 差异 | severity | 位置/证据 | 根因分类 | 目标 |
|---|---|---|---|---|---|
| R0-E-01 | 导航 IA 不符：桌面 10 项平铺 vs 规范 5 项；移动 5 Tab vs 规范 4 Tab；二级页未挂"我的" | **P1** | `dist/index.html` topbar/bottomNav；规范 §8.1 L301–316 | 视觉/交互（导航结构） | 改为 4 Tab + "我的"中枢；桌面 5 项 |
| R0-E-02 | 首页账单：筛选条缺失；骨架屏与日小计已实现，不得重复报缺失 | P2 | `renderHome` L1947–1965、`renderBillStream` L1872–1927；规范 §8.2 | UI/查询能力缺失 | 首页筛选条；骨架屏/日小计保留并补失败态与全量小计边界 |
| R0-E-03 | 交易详情：全字段/编辑/删除回滚已实现，但状态与视觉未逐项验收 | P1 | `renderTxDetail` L1974 | UI 对齐（待截图） | 全字段 + 二次确认 + toast + 余额回滚 |
| R0-E-04 | 两级分类：已接 CRUD，但色板/大类卡片视觉与默认分类策略待核 | P1 | `renderCategories` L2069；`GET /categories?tree=1` | UI 对齐 / 业务规则 | 大类卡片 + 子类 + 色板 + 引用保护 |
| R0-E-05 | 记一笔：类型切换能力存在，但当前为普通 select；校验/pending 仍需实测 | P2 | `index.html` L40–49；`openTxForm` L529–580；规范 §8.2 | UI 交互差异 | 改为可访问的分段控件；双账户显隐与校验按类型验收 |
| R0-E-06 | 账户：PRD 要求期初余额，但创建服务固定 `Balance:0`，前端无输入入口 | P1 | PRD §4.5；`service/account.go` Create；`index.html` 账户弹窗 | 业务契约未落实 | 保留期初余额需求；实现方案见 D-4，禁止直接计入普通收入 |
| R0-E-07 | 投资资产：`renderInvest` 存在但独立成页/持仓交互对齐未证 | P1 | `renderInvest` L1246；prototype/ 有独立页 | UI 对齐 / 基准冲突 | 依原型对齐持仓/实物/净资产 |
| R0-E-08 | Web OCR 无实现：这是 PRD §5.2 明确规定的“不做”，不是 Web 缺陷；PWA 浏览器端本地 OCR 属范围扩展 | P3 | PRD §5.2；设计规范截图页；PWA 指导书 | 产品范围边界 | Web 维持无 OCR；如新增 PWA 本地 OCR，先裁决并修订 PRD/WBS |
| R0-E-09 | 报表：柱状+环形+月份切换已实现，视觉对齐未验收 | P2 | `renderReport` L804 | UI 对齐（待截图） | 主题色柱状 + conic 环形 + 切换 |
| R0-E-10 | 预算：三态（正常/接近/超支）色未验收 | P2 | `renderBudget` L1108 | UI 对齐（待截图） | 三态色 + 总进度 + 信封 |
| R0-E-11 | 成员角色徽标的视觉/状态仍待截图验收；可见性开关与隐私提示统一见 R0-E-19，不在本项重复统计 | P3 | `renderMembers` L1636–1775 | 视觉验收待做 | 对照 owner/admin/member 徽标和窄屏表格布局采集截图 |
| R0-E-12 | 状态覆盖缺口：局部空态/加载骨架已存在，但网络错误、权限不足、离线、同步中、冲突尚未逐页验收 | P2 | 全局；首页加载失败见 R0-E-35 | 健壮性/UX | 建立逐页状态矩阵；失败态必须清除骨架并提供可恢复反馈 |
| R0-E-13 | 设计 token 落地：`style.css` 与 `design-tokens.json` 一致性未逐项核 | P2 | `dist/style.css`；规范 §9 L338 | 视觉（token） | 全量 token 对齐，禁散落硬编码 |
| R0-E-14 | 会话管理 UI 缺失（后端 WBS 5.10 已交付 API） | P2 | `router.go` `/auth/sessions`；前端无调用 | 前端未接 API | 设置页加"登录设备/会话"管理 |
| R0-E-15 | 当前 Web 未接 WS 客户端，也未使用 sync pull/push；后端有路由不代表 Web 已同步 | P2 | `index.html` L1–75；`app.js` 全文；`router.go` L125–135、L215–235 | Web 客户端同步能力缺失 | 明确本阶段 Web 在线/实时边界；按 R 阶段实现 WS 通知 + pull 补漏或明确不纳入 |
| R0-E-16 | 交易总表为原型外的辅助入口，需确认其独立入口/导航归属；多账本与回收站不得列为可删扩展 | P3 | `renderTxns` L337；`renderBooks` L234；`renderRecycle` L751；PRD 多账本/软删除要求 | IA/范围管理 | 账本列表与回收站按已定需求保留；交易总表仅裁决入口归属 |
| R0-E-17 | 设置页仅有 API Token/退出；个人资料能力在“我的”页，服务器/备份区块缺失 | P2 | `renderSettings` L1518–1635；`renderMine` L2138–2215；设计规范 §8.1–8.2 | 信息架构/入口缺失 | 按裁决后的 IA 补资料入口、服务器与备份状态区块；不误报资料能力完全缺失 |
| R0-E-18 | 改密 UI 缺失（后端 `POST /users/me/password` 已存在，前端无入口） | P2 | `router.go` L139；`dist/app.js` 无 `/users/me/password` 调用（"密码"仅登录/注册表单 L179/L184） | 前端未接 API | 设置/个人资料加改密入口（旧密码+新密码） |
| R0-E-19 | 成员账本可见性 UI 缺失：无 `PUT /books/:id/visibility` 调用；逐成员 `canViewOthers` 徽标不等于 MVP 账本级状态 | P1 | `renderMembers` L1636–1775；`router.go` visibility 路由；PRD §7.3；枚举字典 §11 | 隐私控制/字段粒度错误 | 使用 `books.members_can_view_others` 提供账本级开关 + 常驻隐私提示 |

---

## 7. R0-F：截图/测试基线与量化覆盖率方法

### 7.1 视口与断点

| 类别 | 视口 | 说明 |
|---|---|---|
| 手机 | 360×800、390×844、430×932 | 覆盖窄屏/主流/大屏，含 safe-area |
| 桌面 | 1280×800 | 顶栏 + 表格/批量操作 |

### 7.2 证据采集协议（可重复）

1. 启动原型静态服务：`cd prototype && python3 -m http.server 8765`（演示账号 me / 123456）。
2. 启动实现服务：`source .tools/genv.sh && cd server && ./fastledger.exe`（`localhost:8080`，见 §7.4）。
3. 同一数据、同一状态、同一视口分别截图：原型 vs 实现，命名 `r0/<page>/<viewport>/<state>.png`。
4. 关键动作录屏或可重复步骤（点击前置条件 → 结果可观察）。
5. 首屏/加载/空/错误/权限/离线状态各至少一张。
6. **截图执行状态（二次整改实测）**：本轮已用 Microsoft Edge `156.0.4314.58` headless + CDP 采集 **43 张真实 PNG**，覆盖 4 个视口 × 16 个页面/弹层，落盘于 `docs/r0-screenshots/`，逐张索引、采集环境与可复现步骤见 `docs/r0-screenshots/README.md`。但**仅覆盖 normal / empty / error 首屏**；成功/失败交互路径、加载态、权限不足态、离线态、同步/冲突态、以及**原型侧对照截图**均未采集。按总方案 §5 验收基线，**截图基线未完成 / 阻塞，R0 不因此通过**；不得用原型截图、HTML/SVG 或伪造截图替代，不得把该门槛挪到 R1，也不得以文字代替截图。另新增 RV-01 隔离运行时证据 `docs/r0-screenshots/runtime/`（5 PNG + 3 JSON，真实 CDP Network/Console 事件，见 §7.5），但**不改变**截图基线未完成 / 阻塞的结论。

### 7.3 覆盖率计算（严禁主观百分比）

```
覆盖率(页面) = Σ(该页 交互权重 × 达成?) / Σ(该页 交互权重)
覆盖率(总体) = Σ(页面权重 × 页面覆盖率) / Σ(页面权重)
```
- 页面权重建议：首页/记一笔/详情/账户/分类（核心）> 报表/预算/投资/成员/导出 > 设置/回收站。
- 达成判定必须有截图或测试输出为证；无证据记 0。
- 报告须公开：分母、权重表、逐项达成证据。

### 7.4 本地运行基线（已实测，供截图与联调复用）

- 工具链：`.tools/genv.sh`（Go 1.22.5 + w64devkit GCC 12.2.0 + CGO）。
- 构建：`source .tools/genv.sh && cd server && go build -o fastledger.exe ./cmd/server`（已实测 13s 成功）。
- 迁移/首用户/启动：`./fastledger.exe db migrate` → `user create --username admin --password <pw> --owner` → `./fastledger.exe`。
- 隔离数据：`EBK_DB_PATH=.../server/.localdata/fastledger.db`（本地 schema v8）。
- 服务端测试：`go test ./...`（本机 CGO 可用）。

### 7.5 截图基线执行结果、空缺与阻塞（2026-11-04 二次整改）

| 项目 | 状态 | 证据 / 说明 |
|---|---|---|
| 真实截图文件 | 已交付 43 张 PNG | `docs/r0-screenshots/`，逐张索引见该目录 `README.md` |
| RV-01 隔离运行时证据 | 已交付 5 张 PNG + 3 个 JSON | `docs/r0-screenshots/runtime/`（pending / rejected / 恢复态；`network-log.json`（请求头已脱敏，见 [1.66]）/ `console-log.json` / `dom-snapshots.json` + README）；**R0-E-35 本机隔离复验已完成 / 缺陷尚未关闭（见 §7.5 标准口径）** |
| 浏览器与视口 | 已记录 | Edge `156.0.4314.58`；360×800 / 390×844 / 430×932 / 1280×800 |
| 账号与数据样例 | 已记录 | `admin`（owner 角色），账本「我的家庭账本」id=1，本地 schema v8；`transactions total=0` |
| 页面 / 弹层 | 部分 | 16 个页面/弹层首屏 |
| 状态覆盖 | **不足** | 仅 normal / empty / error 首屏；另 5 张 RV-01 隔离运行时截图仅覆盖首页交易列表请求的 pending / 失败 / 恢复态 |
| 成功 / 失败交互路径 | **部分（仅首页列表失败态）** | RV-01 隔离复验记录首页交易列表请求 network 失败 / 503 两态；成功交互路径与逐控件操作仍未采集，需构造测试数据与脚本 |
| 加载 / 骨架屏 / pending | **部分（仅首页）** | RV-01 隔离复验已采集首页 pending 骨架屏 2 张（`docs/r0-screenshots/runtime/`）；其余页面与慢速网络仍未采集 |
| 权限不足（owner/admin/member） | **未采集** | 需多角色账号与多成员账本 |
| 离线 / 同步 / 冲突 | **未采集** | 需离线与冲突构造环境 |
| 原型侧对照截图 | **未采集** | 需启动 `prototype/` 静态服务（8765）同视口对照 |
| 逐控件可重复操作脚本 | **未交付** | 需操作步骤 + 结果断言 |

**阻塞与执行要求**：以上空缺**不降低 R0 门槛、不挪至 R1**，须由 Owner 在浏览器或 CI 中按 §7.2 协议补齐；执行人、所需环境与可复现步骤见 `docs/r0-screenshots/README.md` 的「未覆盖/阻塞清单」。

**复验项 R0-E-35（RV-01 标准口径，[1.68]）**：**' + STATUS + '**
本轮已补隔离运行时证据：Edge `156.0.4314.58` headless + CDP，仅拦截 `GET /api/v1/books/1/transactions?page=1&count=60`（放行 `/books`、`/accounts`、`/categories`、`/reports/monthly` 等全部依赖请求），分别实测 pending、网络失败、HTTP 503、解除拦截后重新导航恢复；证据落盘 `docs/r0-screenshots/runtime/`（5 张 PNG + `network-log.json`（已脱敏）+ `console-log.json` + `dom-snapshots.json` + `README.md`）。**本次实测（单次本机）**：pending 与两种失败态下 `.skel` 恒为 4 条、`aria-busy=true` 未清除、仅红色 toast 且 DOM 中无「重试/刷新」控件；解除拦截并重新导航后 `.skel`=0、显示空态，可恢复。旧轮「页面主体空白」系泛化拦截 `/api/v1/*` 同时打断 `loadBookMeta`、使 `renderHome` 未走到 `renderBillStream` 所致，差异已解释。
> **不得据此判定已修复、已证伪或已关闭**：仍缺 Owner/CI 独立复跑、慢速网络、有数据态、权限不足、离线 / 同步冲突、多角色等验证。可移植复现脚本与合成凭证脱敏测试见 `tools/r0/`；第三方按 `tools/r0/README.md` 可在干净 clone 中重跑，但 WorkBuddy 自身的重复运行**不等于** Owner/CI 独立复跑。

**证据边界（诚实声明，与 §0.2 一致）**：服务端 `server/fastledger.exe` 构建于 `2026-11-04 01:13:59 +0800`，早于 `bff6a2b` 提交时间 `2026-11-04 13:45:36 +0800`（该提交本身仅文档改动）；实测 HTTP 提供的 `/index.html`、`/app.js` 与仓库 HEAD 哈希完全一致、`/style.css` 仅 CRLF 差异（见 `docs/r0-screenshots/runtime/network-log.json` 的 `__META__.servedAssetSha256`），但**后端二进制对应的代码 SHA 未独立验证，不得推定等于 `bff6a2b`**。后端权限 / DTO / 错误码契约**全部来自 `bff6a2b` 源码静态实读**（§5.3），不以运行时二进制为依据。本轮为 RV-01 已重新启动本地服务（`/healthz` 实测 `status:"up"`）并完成上述隔离复验；该运行时复验**不替代**对后端二进制代码 SHA 的独立验证，也不使截图基线整体达标。

### 7.6 测试执行结果（2026-11-04 二次整改实测）

| 项 | 值 |
|---|---|
| 命令 | `source .tools/genv.sh && go test ./...`（在 `repo_tmp/server` 下） |
| 环境 | Go `1.22.5 windows/amd64`、`CGO_ENABLED=1`、w64devkit GCC 12.2.0 |
| 结果 | **10 个包 `ok`**、**2 个包 `[no test files]`**（`internal/version`、`web`）、`FAIL` 行数 **0**、`EXIT_CODE=0` |
| 逐包耗时 | `cmd/server` 12.887s / `internal/api` 16.128s / `internal/authzreview` 19.770s / `internal/config` 1.892s / `internal/database` 12.274s / `internal/instance` 12.510s / `internal/migrate` 13.192s / `internal/model` 1.010s / `internal/service` 34.722s / `internal/ws` 18.434s |
| 日志 | 工作区 `.tmp_shot/gotest_full.log`（临时文件，不入库） |

**口径（任务单要求）**：全量绿**不等于**逐路由测试覆盖证据，不得用「CI 绿 / 某个测试包通过 / 文档写待测」代替逐用例断言。§5.3 仍逐接口列具体测试函数 + `file:line` + 断言类型，并对未找到独立命名测试的接口明确写「未覆盖」。`go test -race` 全量与 CI 侧结果仍由 Owner/CI 提供（本轮**未执行** `-race`）。

---

## 8. R0-G：Owner 裁决清单（阻塞项，未裁决不开发）

| 编号 | 待裁决问题 | 现有依据 | 备选与建议 | 状态 |
|---|---|---|---|---|
| **D-1** | 最终前端基准：`prototype/`（13 页）还是 `prototype-v2/`（12 视图）？ | 总方案 §2.2 暂行：prototype/ 为流程基准、v2 为视觉补充 | 2026-11-04 Owner 裁决：流程/页面以 `prototype/` 为准，视觉/组件以 `prototype-v2/` + 设计规范为准；冲突先登记 | **已裁决** |
| **D-2** | PWA 浏览器端是否新增本地 OCR？Web 本身是否做 OCR 已由 PRD §5.2 明确为“不做” | PRD §5.2：Web 不做 OCR；PWA 指导书提出浏览器端本地识别评估 | Web 不做 OCR 是既定要求；若 PWA 要增加浏览器端本地 OCR，属于 PRD 范围变更，需 Owner 明确批准并同步 PRD/WBS/隐私边界 | 仅 PWA 扩展待确认 |
| **D-3** | 导航是否按规范改为 4 Tab + "我的"中枢（拆除 10 项平铺顶栏）？ | 设计规范 §8.1；总方案 R2 | 建议：按规范改；保留账本切换入口于顶栏左上 | 待确认 |
| **D-4** | 期初余额的技术实现模型如何设计（需求本身不再待裁决）？ | PRD §4.5 明确要求支持期初余额；当前服务固定 `Balance:0`；服务注释建议用普通收入交易初始化 | 保留 PRD 需求；建议设计不进入收支统计的 opening-balance/余额调整记录，并同步数据模型、审计、同步、净资产和迁移契约；禁止伪装成普通收入 | 技术方案待 Owner 确认 |
| **D-5** | 投资资产是否独立成页？ | prototype/ 独立页；v2 疑似折入账户 | 建议：保留独立页（依 prototype/），v2 仅视觉 | 待确认 |
| **D-6** | 交易总表是否保留为独立导航入口，还是作为首页/我的的辅助检索视图？ | 多账本与回收站由 PRD/已定软删除要求支持，不属于可删扩展；交易总表是额外检索视图 | 建议保留交易总表能力，但由 Owner 裁决独立入口/导航归属；账本列表与回收站按既定需求保留 | 待确认（仅交易总表入口） |
| **D-7** | 多端会话管理是否属于产品需求？ | PRD §4.1/§4.12 已要求可查看与撤销会话；后端 `/auth/sessions` 已有路由，Web UI 未接（R0-E-14） | 需求已确定；Owner 只需决定排期/入口，不得裁决为“不需要”而不修订 PRD | 已确认需求，排期待定 |
| **D-8** | Web 设置页的服务器/备份区块展示到什么程度、数据从何处安全读取？ | PRD §4.12 与设计规范 §8.1–8.2 已要求服务器信息/备份入口；当前 Web 无状态 API 证据，运维备份另有部署机制 | 2026-11-04 Owner 裁决（六原则，见 `docs/FastLedger-R0-D8-服务器备份区块裁决草案.md`）：设置页保留两区块；服务器信息首期只读非敏感；备份状态仅实例管理员可见且须来自真实备份记录；账本角色≠实例管理员；首期不开放 Web 恢复/删备/改目录；Web 不得直读服务器文件/密钥/备份目录 | **已裁决** |

> **R0 通过条件（2026-11-04 二次整改后修订 · 当前状态：未通过 / 阻塞）**：
> - **本轮已完成（文档与证据层面，不等于验收通过）**：① §3.1 逐控件交互清单重做（16 个分组、228 条独立控件/动作、13 字段，修正合并行与 `#loginForm` 误引，新增 R0-E-38 / R0-E-39）；② §5.3 API 权限 / DTO / 测试矩阵重做（逐路由源码实读：权限来自 service guard 函数、DTO 来自 handler / struct、错误码追溯 `mapSvcErr` 等、测试证据列具体测试函数或明确「未覆盖」）；③ 43 张真实浏览器截图与索引落盘；④ RV-01 隔离运行时证据（5 PNG + 3 JSON + README，`docs/r0-screenshots/runtime/`），但 **R0-E-35 本机隔离复验已完成 / 缺陷尚未关闭（见 §7.5 标准口径）**。
> - **仍未完成（阻塞项，不得挪至 R1、不得降低门槛）**：① §7 截图基线——当前仅首屏 normal/empty/error（RV-01 隔离证据另补首页 pending/失败/恢复，不改变整体缺口），成功/失败交互、加载、权限、离线、同步/冲突与原型侧对照截图均缺；按总方案 §5，截图基线未完成 R0 不得通过。② 逐控件交互的**运行时验收**（校验红字、pending、三态色、空态、骨架屏视觉、成功/失败反馈、返回路径）尚无逐项截图或测试输出为证。③ `go test ./...` 已实测全绿（10 包 ok / 2 包无测试 / 0 FAIL / EXIT_CODE=0，见 §7.6），但**全量绿不等于逐路由测试覆盖证据**；§5.3 逐接口列具体测试函数并标注「未覆盖」的接口仍需 Owner/CI 补齐逐用例断言与 `-race` 结果。
> - **Owner 裁决未关闭**：D-3（导航结构）、D-5（投资资产是否独立页）、D-6（仅交易总表独立入口）待结论；D-2 的 PWA 本地 OCR 待裁决；D-4 期初余额技术实现模型待确认。D-1（最终前端基准）与 D-8（服务器/备份区块）已于 2026-11-04 裁决关闭（见 `docs/FastLedger-R0-D8-服务器备份区块裁决草案.md`）。
> - **裁决前既定口径（不得重开）**：D-2 的 Web OCR 已由 PRD §5.2 决定为不做；D-4 期初余额需求已确定；D-7 会话管理需求已确定仅排期待定；多账本与回收站按既定需求保留。
> - **R0-E-35 / RV-01**：**本机隔离运行时复验已完成；失败态观察到骨架屏未清理（`.skel`=4、`aria-busy=true`）；待 Owner/CI 独立复跑及慢速网络、有数据态、多角色等场景验证；缺陷尚未关闭。** 隔离运行时证据已登记于 `docs/r0-screenshots/runtime/`（5 张 PNG + `network-log.json`（请求头已脱敏）/ `console-log.json` / `dom-snapshots.json` + README）；单次本机实测不构成缺陷关闭，须 Owner/CI 独立复跑并补齐慢速网络、有数据态、权限、离线 / 同步冲突、多角色等缺口。**不得声称已关闭，也不得倒退回「尚未进行运行时复验」的旧表述。**
> - **结论**：**R0 未通过 / 阻塞**。满足上述截图基线落地、逐控件运行时验收证据、测试执行输出与 Owner 对未决范围给出结论后，R0 方可通过并进入 R1 最小批次拆分。**本次整改不代表 R1–R7 已获授权。**

---

## 9. 附录：证据索引

| 证据 | 位置 |
|---|---|
| 原型 13 页清单 | `prototype/README.md` L14–L18 |
| 原型构建产物（含投资/持仓/净资产文案） | `prototype/assets/index-DNvhksAn.js` |
| v2 视图/导航结构 | `prototype-v2/index.html`（`#topbar`/`#tabbar`/`#view`；`data-nav`） |
| 设计规范 IA 与交互规格 | `docs/FastLedger-Web设计规范.md` §8.1 L301–316、§8.2 L318–333 |
| 前端路由与渲染 | `server/web/dist/app.js` `route()` L2200–2241 |
| 前端导航 DOM | `server/web/dist/index.html`（`#topbar` 10 项、`#bottomNav` 5 Tab、`#formDlg`/`#editDlg`/`#confirmDlg`/`#catDlg`/`#acctDlg`） |
| 后端路由表 | `server/internal/api/router.go` L109–L235 |
| 交易更新语义 | `docs/FastLedger-交易更新语义设计.md`（[1.44] 依据） |
| 已定决策 | `workbuddy/第三方审阅者导读.md` §5 |
| 阶段编排 | `docs/FastLedger-原型还原与PWA分阶段实施总方案.md` §4–§5 |
| 真实截图基线与索引 | `docs/r0-screenshots/`（43 张 PNG）、`docs/r0-screenshots/README.md` |
| RV-01 隔离运行时证据 | `docs/r0-screenshots/runtime/`（5 PNG + `network-log.json`（请求头已脱敏，见 [1.66]）/ `console-log.json` / `dom-snapshots.json` + README；仅拦截 `GET /api/v1/books/1/transactions` 的 pending / 失败 / 恢复态） |
| §5.3 权限 / DTO / 错误码源码取证 | `server/internal/api/router.go` L109–L235、`common.go` L17–L136、`auth_handler.go` L103–L122、`middleware.go` L34–L40、`service/guard.go`、`service/visibility.go`、各 `*_handler.go` / `service/*.go` |
| 逐路由测试函数索引 | `server/**/*_test.go`（§5.3 逐行列出 file:line 与断言类型） |
| R0 二次整改复核 | `docs/FastLedger-R0-独立补漏与复核记录.md` §9、`CHANGELOG.md` [1.63] |
| RV-05 凭证安全扫描报告（脱敏） | `docs/r0-screenshots/security-scan.md`（`tools/r0/scan-credentials.js --strict --out` 生成） |
| RV-05 核验与 RV-06 复现记录 | `docs/FastLedger-R0-凭证安全核验记录.md`、`docs/FastLedger-R0-第三次整改交付回执.md` |
| RV-05 / RV-06 finding 归档 | `docs/FastLedger-R0-独立复核补充-RV-05至06.md` |
| 可移植复现脚本与脱敏测试 | `tools/r0/{rv01-isolation.js,redaction.js,test-redaction.js,scan-credentials.js,README.md}` |

---

*本文件为 R0 盘点交付，不含代码改动。**R0 当前未通过 / 阻塞**；待 Owner 对 §8 裁决清单给出结论、并按 §7 补齐截图基线与逐控件运行时验收证据后，再由 Owner 指定 R1 首个小批次。*


## 10. 独立复核补漏与二次整改（2026-11-04）

### 10.1 独立复核补漏（[1.61]）

详细证据、根因、验收及页面交互补充见 [`docs/FastLedger-R0-独立补漏与复核记录.md`](FastLedger-R0-独立补漏与复核记录.md)。补充 R0-E-20 至 R0-E-37；其中 E-25/E-27/E-29/E-30/E-31 分别并入 E-15/E-19/E-05/E-06/E-14，不单独计数。新增独立差异覆盖首页财务口径、账单字段/筛选、交易详情/表单、商户字段端到端缺失、删除文案、邀请码撤销、净资产页面归属、账本重命名、README 状态漂移、日小计分页边界、加载错误态和 API 矩阵粒度。

### 10.2 二次整改：针对 [1.62] 的独立复核（[1.63]）

[1.62] 曾把 §3.1 / §5.3 初稿声明为「R0 完成」并称「唯一剩余缺口为截图」，该声明**不成立**：§3.1 把多个独立控件合并成一行、§5.3 的角色权限由路由语义推定且 DTO 多处与源码不符、测试列用「CI 绿 / 待测」代替逐用例证据、且当时无任何真实截图。本轮按独立复核意见重做：

- **§3.1**：改为逐独立控件/动作 16 个分组 228 条、13 字段；更正不存在的 `#loginForm` 引用；新增 R0-E-38（登录页无「加入账本」/邀请码兑换，邀请链路断口）、R0-E-39（密码明文切换缺失）、I-BOOKS-11（账本详情/设置页入口缺失）。
- **§5.3**：权限改由真实 service guard 函数实读（不再按路由名推定）；DTO 改由 handler / struct 实读（纠正 login / register / join、sessions、改密、撤邀请、net-worth、回收站、删除恢复、WS 路径等字段与包装形式）；错误码追溯 `mapSvcErr` / `mapLoginErr` / `mapAuthErr` / OCR 独立映射；测试证据改为具体测试函数 + `file:line`，无独立测试的接口明确写「未覆盖」。
- **§7**：补交 43 张真实 Edge 截图与索引，并如实登记空缺与阻塞；**未降低截图门槛、未挪至 R1**。
- **状态声明**：§8 通过条件与 `CHANGELOG.md` [1.62] 标题/正文改为「R0 文档补充：交互清单与 API 矩阵初稿（R0 尚未通过）」；新增 [1.63] 记录本次整改。
- **E-1~E-5 与 D-1~D-8 复核**：E-1（首页第三卡账户余额求和冒充结余/净资产）、E-2（paymentChannel UI 缺失 + merchant 端到端创建链路缺失）、E-3（账本级 `books.members_can_view_others` 权威字段，成员级只读徽标不能代替）、E-4（期初余额是 PRD 已明确需求，待裁决的是不污染收支统计的技术实现模型）、E-5（R0-E-20~37 去重）均保持原判定，未误删、未降级；D-1~D-8 状态见 §8，未擅自关闭。
- **范围**：仅修订审计/交付文档、`CHANGELOG.md` 与 README 状态快照；**未改业务代码、PRD、落地方案、WBS、数据库或 API**；未启动 R1–R7。
- **测试执行**：`go test ./...` 实测 10 包 ok / 2 包无测试 / 0 FAIL / EXIT_CODE=0（见 §7.6）；全量绿**不等于**逐路由覆盖证据，逐用例断言与 `-race` 仍待 Owner/CI。
- **当前状态**：**R0 未通过 / 阻塞**——截图基线未完成、逐控件运行时验收缺证据、逐路由逐用例断言与 `-race` 结果未产出、Owner 裁决未关闭。

### 10.3 独立复核补充 RV-01~04（即外部文档 R0-RV-01~04）修正（[1.65]）

- **RV-01**：新增隔离运行时证据 `docs/r0-screenshots/runtime/`（Edge `156.0.4314.58` + 原生 CDP，仅拦截 `GET /api/v1/books/1/transactions`，放行其它依赖请求；pending / 网络失败 / 503 / 解除拦截后恢复；5 PNG + 3 JSON + README）。本次实测：失败后 `.skel`=4、`aria-busy=true` 未清除、仅 toast 且无页内重试控件；重新导航可恢复。旧轮「页面主体空白」系泛化拦截打断 `loadBookMeta` 所致。**R0-E-35 标准口径：本机隔离运行时复验已完成；失败态观察到骨架屏未清理（`.skel`=4、`aria-busy=true`）；待 Owner/CI 独立复跑及慢速网络、有数据态、多角色等场景验证；缺陷尚未关闭。**
- **RV-02**：新增 §0.2「证据版本基准」表，分离**当时**文档 HEAD `557e4d2`（[1.68] 已改列为「运行时证据采集时历史文档 HEAD」，不再是当前报告版本）、业务代码证据基准 `bff6a2b`、截图对应代码版本「**未独立验证**」与历史快照 `1cd56ab`；§0 / §3.1 / §5.3 / §7.5 口径已对齐。[1.68] 另补本轮整改执行基准 `273cf08` 与全文 SHA 语义复核。
- **RV-03**：§3.1 逐控件表内所有裸 `—` 单元格按列语义替换为 `N/A（不适用）` / `见 I-XXX` / `未验证` / `未实现` / `待确认`；228 条行数与 13 字段结构不变，交叉引用 ID 已逐条校验存在，新增字段语义词表。
- **RV-04**：查证 PRD §3.1 / §4.2 / §4.8、Web 设计规范 §8.1 / §8.2、`prototype-v2/index.html` `pHome()`、`prototype/` 构建产物与 `prototype/README.md`，**均无「首页必须提供月份切换控件」的明确条款**（月份切换仅见于报表页与导出页时间预设）。`I-HOME-02` 由「缺失」改为「待 Owner 裁决的交互建议」，移出已确认缺陷统计。
- **范围与状态**：仅修订审计文档、CHANGELOG、README 与新增运行时证据；未改业务代码 / PRD / 落地方案 / WBS / 数据库 / API；PWA 不动。**R0 未通过 / 阻塞，不授权 R1。**


### 10.4 第三次最小整改 RV-02 / RV-05 / RV-06 / RV-01（[1.68]）

> 依据 `docs/FastLedger-R0-第三次最小整改任务单-RV-02-RV-05-RV-06.md`；执行基准 `273cf08`。本节只记录已完成动作与证据路径，逐项回执见 `docs/FastLedger-R0-第三次整改交付回执.md`。

- **RV-05（P0，安全优先）**：全仓凭证扫描器 `tools/r0/scan-credentials.js` 落盘脱敏报告 `docs/r0-screenshots/security-scan.md`（清理前 `--strict` 命中 2 处固定口令、`EXIT=2`；清理后命中 0、`EXIT=0`）；清理 `docs/r0-screenshots/runtime/README.md` 与 `docs/FastLedger-新手开发完整指南.md` 中可复制的固定口令，改为环境变量 / 占位符；`redaction.js` 覆盖请求头、响应头、`headersText`、Cookie、`Set-Cookie`、URL 查询参数、JSON 请求体与 JWT / Bearer 字面量，`test-redaction.js` 用合成假凭证断言；采集脚本另有「原始令牌不得出现在输出」自检（退出码 4）。旧令牌受控复验 `GET /api/v1/users/me` → **401「登录已失效，请重新登录」**（仅内存使用，不记录值）。**状态拆分**：当前 `main` 文件已脱敏 ✅ / 泄露凭证已失效 ✅ / 旧 SHA `259bd11` 经 GitHub API 与 raw 接口实测仍 HTTP 200 可读（旧日志仍含 29 处凭证字面量）⚠️ / **远端历史对象清理待 Owner 决策** ⛔。本地 `reflog expire` + `gc` **不等于**远端清除；本轮**未** force-push、未重写公开历史、未删除 refs、未改仓库可见性、未联系 GitHub Support。
- **RV-02（P2）**：§0.2 拆分为「本轮整改执行基准 `273cf08`」「业务代码证据基准 `bff6a2b`」「运行时采集时历史文档 HEAD `557e4d2`」「截图 / 运行时对应后端二进制代码 SHA 未独立验证」「历史快照 `1cd56ab`」五类；§0 / §5.3 引言 / §5.3.0 / §7.5 / §10.3 逐处按语义校正，未机械替换历史条目中的 `557e4d2`。
- **RV-06（P2）**：`tools/r0/rv01-isolation.js` 提交入库，路径由脚本位置 / 仓库根 / 环境变量推导，凭证只从 `R0_TOKEN` 读取且缺失即安全失败（退出码 2），默认输出在仓库外临时目录，写正式证据目录须双重显式确认；`tools/r0/README.md` 给出依赖、环境变量、测试账号准备、退出码、清理与**平台限制（仅 Windows + Git Bash 实测，不声称跨平台）**；干净 clone 复现记录与 SHA256 见核验记录 §5 / 交付回执 §3。
- **RV-01**：R0-E-35 全文统一为「本机隔离运行时复验已完成；失败态观察到骨架屏未清理（`.skel`=4、`aria-busy=true`）；待 Owner/CI 独立复跑及慢速网络、有数据态、多角色等场景验证；缺陷尚未关闭」，**不再使用该状态旧称，也未写成已修复 / 已关闭**；单次本机实测的证据边界保留。
- **范围与门禁**：仅新增 / 修订审计文档、CHANGELOG、README、`tools/r0/` 与脱敏报告；**未改业务代码、PRD、落地方案、WBS、数据库或 API**；未删除或覆盖既有 5 PNG + 3 JSON；未启动 R1–R7，PWA 未实施。**R0 未通过 / 阻塞。**
