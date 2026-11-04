# FastLedger R0：基准冻结与差异盘点

> 文档性质：**R0 阶段交付（盘点，非代码完成）**。本文件只做基准冻结、逐页差异盘点、根因分类与裁决清单，**不修改任何业务代码 / PRD / 落地方案 / 枚举字典 / WBS**。
> 依据：`docs/FastLedger-原型还原与PWA分阶段实施总方案.md` §5「R0」、§6「强制交付格式」；`workbuddy/多AI对齐规范.md`（颗粒度/编号/状态/文档权威）。
> 编制：实施 AI（WorkBuddy）。编制日期：2026-11-04。

---

## 0. 开工前回执（§6.1 强制）

| 项 | 内容 |
|---|---|
| 当前基线 SHA | `origin/main` = **f9f075e**（GPT 总方案 + PWA 指导书 7 个提交）；本分支 `main` = **55bbd4e**（本地前端提交，已 rebase 到 f9f075e 之上，**未推送**） |
| 工作分支 | `main`（本地） |
| 关键决策文档（本次已读） | `workbuddy/多AI对齐规范.md`、`workbuddy/第三方审阅者导读.md`、根 `CHANGELOG.md`（[1.44]/[1.57]/[1.58]/[1.59]）、`docs/FastLedger-原型还原与PWA分阶段实施总方案.md`、`docs/FastLedger-手机PWA实施指导书.md`、`docs/FastLedger-Web设计规范.md`（§8 信息架构）、`docs/FastLedger-交易更新语义设计.md`、`prototype/README.md`、`prototype-v2/README.md` |
| 本次范围 | R0-A ~ R0-G（页面/交互盘点、两版原型差异、当前实现与后端 API 映射、差异根因、截图与覆盖率方法、Owner 裁决清单） |
| **明确不做** | 不改 `server/`、`client/`、`server/web/dist/` 业务代码；不改 PRD/落地方案/枚举字典/WBS；不做 PWA 代码；不做 UI 重构；不给"还原度百分比"主观估值（见 §1.3） |
| 相关证据源 | `prototype/`（构建产物）、`prototype-v2/index.html`、`server/web/dist/{index.html,app.js,style.css}`、`server/internal/api/router.go`、`docs/FastLedger-Web设计规范.md` §8 |

### 0.1 与 GPT 评审的对齐说明（"与 GPT 对齐"）

1. **接受 GPT 的口径纠偏**：总方案 §3.3 明确不得把交接文档中未经评分矩阵证明的"50–55% 还原度"当作事实。本 R0 **不再复述该百分比**，改用"页面 × 关键交互 × 状态 × 断点"的加权矩阵（§7）统计，百分比须公布分母/权重/证据后才可出现。
2. **接受基准冲突的暂定规则**（总方案 §2.2）：`prototype/` 作页面/流程/交互基准；`docs/FastLedger-Web设计规范.md` + `design-tokens.json` 作视觉基准；`prototype-v2/` 仅作视觉补充，不得仅凭 v2 覆盖原型业务流程。**最终基准确认进 R0-G 裁决项 D-1**。
3. **区分"后端有能力"与"前端已接通"**：总方案 §3.1 指出不得把"有同名页面/能调 API"冒充原型还原。本 R0 的 R0-D 对每条后端能力标注"前端是否调用"，避免误判。
4. **不动已定决策**（导读 §5）：多账本、同步版本号、整数分、软删除/恢复/审计、隐私定位（图片不上传）等一律不改，仅为视觉一致不得破坏。
5. **本地分支状态**：我的前端提交已在 GPT 最新 `main` 之上 rebase，工作树同时含 GPT 文档与我的前端改动；**R0 只新增本文档**，不夹带代码改动。

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
| 3 | 截图识别记账 | 记一笔 → 截图识别 | 无 | 无 | **未实现**（无 OCR 调用，见 R0-E-08 待裁决） |
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
| 首页账单 | 概览三卡联动、按日分组账单、行点击进详情、筛选条、骨架屏 | `renderHome` L1929：**三卡已实现**（L1947–1965 本月收入/支出/净资产=Σ账户余额）；`renderBillStream` L1872 按日分组；行点击→`#b/:id/tx/:tid` | **筛选条/骨架屏缺失** | **部分具备**（缺筛选条与骨架屏，R0-E-02） |
| 记一笔 | 支出/收入/转账分段、转账双账户、必填校验红字、pending→toast→返回首页顶部插入 | `#formDlg`（`fType/fAmount/fAccount/fTo/fCategory/fDate/fNote`）；`POST /transactions` | 校验/pending [待确认] | 具备，细节待验 |
| 截图识别 | 三级漏斗 → 确认弹窗 → 确认入账（绝不自动入账） | **无**（dist 仅 1 处"截图"文案，无 OCR 调用） | — | **未实现**（R0-E-08，待裁决） |
| 交易详情 | 全字段、编辑、删除二次确认→toast+余额回滚 | `renderTxDetail` L1974（编辑复用 `openTxEditForm`；删除含确认文案） | 已删实体 409 `ENTITY_DELETED` [待确认] | 具备 |
| 账户 | 账户卡（余额/类型徽标）、净资产汇总、正负余额色 | `renderAccounts` L647；`GET/POST/PUT/DELETE /accounts` | 空态 [待确认] | 具备（初始余额限制见 R0-E-06） |
| 分类 | 大类卡片→子类列表、分类色板 | `renderCategories` L2069；`GET /categories?tree=1` + CRUD | 删除引用计数 [待确认] | 具备 |
| 预算 | 总进度条 + 信封列表、正常/接近/超支三态 | `renderBudget` L1108；`GET/POST/PUT/DELETE /budgets` | 三态色 [待确认] | 具备，视觉待验 |
| 报表 | 月度柱状（chart-1 主色）+ 分类占比环形（conic）+ 月份切换 | `renderReport` L804；`GET /reports/monthly?months=6` + `/reports/categories` | 空态 [待确认] | 具备，视觉待验 |
| 家庭成员 | 角色徽标、可见性开关 + 常驻隐私提示 | `renderMembers` L1636：`.family-block` 家庭信息 + 成员 CRUD；**可见性仅只读徽标**（L1720 `canViewOthers`），无可操作开关，未调用 `PUT /books/:id/visibility` | 权限不足 [待确认] | **部分具备**（缺可见性开关，R0-E-19） |
| 导出 | 筛选 + 预览 + CSV/JSON | `renderExport` L933（时间范围预设 + "共N条"预览 + `GET /export` blob 下载 + `X-Export-Count/Truncated`） | 空态/超大截断 [待确认] | 具备 |
| 设置 | 个人资料、API Token、服务器与备份、退出登录 | `renderSettings` L1518：**仅 API Token（生成/复制/撤销）+ 退出登录**；个人资料在"我的"页；**缺服务器与备份区块、缺改密** | — | **部分具备**（缺 个人资料/服务器与备份/改密，R0-E-17/18） |
| 我的 | 菜单中枢 | `renderMine` L2138：头像 + 资料编辑（改邮箱）+ **8 个二级入口**（我的账本/账户/成员/导出/报表/预算/资产/回收站） | — | 具备（8 入口；与平铺顶栏重复，R0-E-01） |

**状态覆盖缺口（统一记为 R0-E-12）**：空 / 加载(骨架屏) / 网络错误 / 权限不足 / 离线 / 同步中 / 冲突 —— 当前代码未见系统性状态组件，需逐页补齐并截图。

---

## 4. R0-C：`prototype/` 与 `prototype-v2/` 差异对照

| 差异点 | `prototype/`（旧，13 页） | `prototype-v2/`（新，规范示范） | 是否影响业务语义 | 建议基准 |
|---|---|---|---|---|
| 页面数 | 13 页（含**投资资产**独立页） | README 称"12 视图"；HTML 含「净资产」（账户视图汇总），**独立"投资资产"页是否存在 `[待确认]`** | 是（投资/持仓是否独立管理） | 以 `prototype/` 为准，v2 补视觉 |
| 记账入口 | "手动记账/转账" | "记一笔"（支出/收入/转账分段 + 截图识别入口） | 否（同一能力，措辞/手势不同） | 交互按 v2，能力按 prototype |
| 截图识别 | 有（截图识别记账页） | 有（三级漏斗演示，绝不自动入账） | 是（Web/PWA 是否做 OCR，R0-E-08） | 待 Owner 裁决 |
| 二级页归属 | 分类/导出/成员/设置在"我的"下 | 同左（我的为菜单中枢） | 否 | 一致 |
| 导航 | 13 页原型 | 桌面 5 项 / 移动 4 Tab | 否（结构一致） | 采用 v2/规范 |
| 投资资产 | 独立页（持仓、实物、净资产） | 净资产存在于账户视图，独立页 `[待确认]` | **是** | 以 prototype/ 为准，是否独立成页见 R0-G D-1 |

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
| 同步 | `GET /books/:id/sync/pull`、`POST /books/:id/sync/push` | JWT | ❌（前端未调用；当前靠 WS 提示） |
| WS | `GET /api/v1/ws` | JWT | ✅（`ws.js`/连接逻辑 [待确认]） |

**关键结论**：
- 后端**能力完整**，"账户/分类/交易详情/删除恢复/报表/预算/投资/成员/设置"均有 API —— 差异主要在**前端 UX/页面结构与原型对齐**，而非后端缺能力（符合总方案 §3.1）。
- **前端未接通**：OCR（范围待裁决）、会话管理 API、sync pull/push（离线/PWA 阶段再定）。
- `PUT /transactions/:txId` 已开放且是**全量替换 + 类型不可改**（改类型=删除+重建），R1 不得伪造类型变更。

---

## 6. R0-E：逐项差异与根因分类

> 说明：下表为 **R0 盘点项**（gap），不是已定性的缺陷 finding；转入 R1 实施时须按 `规范 §2` 补全 13 字段并分配前缀编号（如 `WB-Rxx`）。`severity` 预标便于排期。

| ID | 差异 | severity | 位置/证据 | 根因分类 | 目标 |
|---|---|---|---|---|---|
| R0-E-01 | 导航 IA 不符：桌面 10 项平铺 vs 规范 5 项；移动 5 Tab vs 规范 4 Tab；二级页未挂"我的" | **P1** | `dist/index.html` topbar/bottomNav；规范 §8.1 L301–316 | 视觉/交互（导航结构） | 改为 4 Tab + "我的"中枢；桌面 5 项 |
| R0-E-02 | 首页账单：三卡已实现，**缺筛选条与骨架屏**；日小计待核 | P1 | `renderHome` L1929（三卡 L1947–1965）、`renderBillStream` L1872；规范 §8.2 | UI 缺失（筛选条/骨架屏） | 三卡（已）+ 日小计 + 筛选条 + 骨架屏 |
| R0-E-03 | 交易详情：全字段/编辑/删除回滚已实现，但状态与视觉未逐项验收 | P1 | `renderTxDetail` L1974 | UI 对齐（待截图） | 全字段 + 二次确认 + toast + 余额回滚 |
| R0-E-04 | 两级分类：已接 CRUD，但色板/大类卡片视觉与默认分类策略待核 | P1 | `renderCategories` L2069；`GET /categories?tree=1` | UI 对齐 / 业务规则 | 大类卡片 + 子类 + 色板 + 引用保护 |
| R0-E-05 | 记一笔：类型分段/双账户已实现，校验/pending/返回插入未验收 | P1 | `#formDlg` L529；规范 §8.2 | UI 对齐（待截图） | 分段切换 + 必填红字 + pending→toast |
| R0-E-06 | 账户：后端 `Create` 强制 `Balance:0`，无"期初余额"入口 | P1 | `service/account.go` AccountCreateInput；前端已移除初始余额字段 | 业务契约（缺口） | 明确期初余额策略（R0-G D-4） |
| R0-E-07 | 投资资产：`renderInvest` 存在但独立成页/持仓交互对齐未证 | P1 | `renderInvest` L1246；prototype/ 有独立页 | UI 对齐 / 基准冲突 | 依原型对齐持仓/实物/净资产 |
| R0-E-08 | 截图识别：Web 无实现，OCR 范围未决 | P1 | dist 无 `/ocr/*` 调用；PRD §5.2 vs 设计规范含截图页 | 产品冲突 | Owner 裁决（R0-G D-2） |
| R0-E-09 | 报表：柱状+环形+月份切换已实现，视觉对齐未验收 | P2 | `renderReport` L804 | UI 对齐（待截图） | 主题色柱状 + conic 环形 + 切换 |
| R0-E-10 | 预算：三态（正常/接近/超支）色未验收 | P2 | `renderBudget` L1108 | UI 对齐（待截图） | 三态色 + 总进度 + 信封 |
| R0-E-11 | 成员：角色徽标/可见性开关/常驻隐私提示对齐未验收 | P2 | `renderMembers` L1636 | UI 对齐（待截图） | 徽标 + 开关 + 隐私提示 |
| R0-E-12 | 状态覆盖缺口：空/加载(骨架)/错误/权限不足/离线/同步中/冲突 未系统化 | P2 | 全局（无统一状态组件证据） | 健壮性/UX | 逐页补状态组件 |
| R0-E-13 | 设计 token 落地：`style.css` 与 `design-tokens.json` 一致性未逐项核 | P2 | `dist/style.css`；规范 §9 L338 | 视觉（token） | 全量 token 对齐，禁散落硬编码 |
| R0-E-14 | 会话管理 UI 缺失（后端 WBS 5.10 已交付 API） | P2 | `router.go` `/auth/sessions`；前端无调用 | 前端未接 API | 设置页加"登录设备/会话"管理 |
| R0-E-15 | 未使用 `sync pull/push`（仅 WS）；离线/PWA 阶段再定 | P3 | 前端无 `/sync/*` 调用 | 架构（阶段 3 后端有） | 归入 R4/R5 |
| R0-E-16 | 账本/交易总表/回收站为原型外扩展，需确认保留与归属；另 `PUT /books/:id`（账本重命名）前端未接 | P3 | `renderBooks` L234、`renderTxns` L337、`renderRecycle` L751；`router.go` L161 | 范围确认 | 保留（PRD 依据），导航归位 |
| R0-E-17 | 设置页缺「个人资料」「服务器与备份」区块（原型设置三区：个人资料/API Token/服务器与备份） | P2 | `renderSettings` L1518（仅 Token+退出）；原型设置含三区；"资料"仅在 `renderMine` L2158 | UI 缺失 / 前端位置错置 | 设置页补齐三区块（个人资料可复用 mine 的资料编辑） |
| R0-E-18 | 改密 UI 缺失（后端 `POST /users/me/password` 已存在，前端无入口） | P2 | `router.go` L139；`dist/app.js` 无 `/users/me/password` 调用（"密码"仅登录/注册表单 L179/L184） | 前端未接 API | 设置/个人资料加改密入口（旧密码+新密码） |
| R0-E-19 | 成员「账本可见性」开关缺失：仅只读徽标，无 `PUT /books/:id/visibility` 调用 | P2 | `renderMembers` L1720（`canViewOthers` 只读徽标）；`router.go` L165 | 前端未接 API / UI 缺失 | 加 book 级"成员互看"开关 + 常驻隐私提示 |

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
6. **截图执行状态**：本 R0 为静态盘点，**尚未采集实际截图**（需浏览器环境）；截图采集作为 R0→R1 过渡的执行项，不得以文字代替。

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

---

## 8. R0-G：Owner 裁决清单（阻塞项，未裁决不开发）

| 编号 | 待裁决问题 | 现有依据 | 备选与建议 | 状态 |
|---|---|---|---|---|
| **D-1** | 最终前端基准：`prototype/`（13 页）还是 `prototype-v2/`（12 视图）？ | 总方案 §2.2 暂行：prototype/ 为流程基准、v2 为视觉补充 | 建议：流程/页面以 prototype/ 为准，视觉/组件以 v2+设计规范为准；两者冲突先登记 | 待确认 |
| **D-2** | Web/PWA 是否提供截图识别？ | PRD §5.2 限 Web OCR；设计规范含截图页；PWA 指导书仅建议用户选图本地 OCR | 建议 Web 首期不做；PWA 仅"用户主动选图 + 端侧 OCR"，不承诺跨 App 截图 | 待确认 |
| **D-3** | 导航是否按规范改为 4 Tab + "我的"中枢（拆除 10 项平铺顶栏）？ | 设计规范 §8.1；总方案 R2 | 建议：按规范改；保留账本切换入口于顶栏左上 | 待确认 |
| **D-4** | 账户"期初余额"如何处理？ | 后端 `Create` 强制 `Balance:0`；原型账户页可能体现余额 | 建议：不新增期初字段；以"期初调整"交易（收入类）体现，或明确"创建即 0，余额由交易产生" | 待确认 |
| **D-5** | 投资资产是否独立成页？ | prototype/ 独立页；v2 疑似折入账户 | 建议：保留独立页（依 prototype/），v2 仅视觉 | 待确认 |
| **D-6** | 账本列表/交易总表/回收站 是否保留为独立入口？ | 原型 13 页未含；PRD 多账本/软删除 | 建议：保留（PRD 依据），导航归位至合理位置 | 待确认 |
| **D-7** | 会话管理 UI 是否纳入 R1？ | 后端 5.10 已交付 `/auth/sessions`；前端未接 | 建议：纳入设置页（低风险） | 待确认 |
| **D-8** | Web 设置页是否需要「服务器与备份」状态展示？ | 原型设置有该区块；后端当前**无备份状态 API**（仅 CLI `instance bump`） | 建议：MVP 仅展示静态说明/部署信息，或新增只读 `instance.meta` 接口；不做 Web 触发备份 | 待确认 |

> **R0 通过条件**：以上 D-1~D-8 均获 Owner 明确结论（或标"阻塞，不开发"），且 §7 截图基线开始采集，方可进入 R1。

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

---

*本文件为 R0 盘点交付，不含代码改动。待 Owner 对 §8 裁决清单给出结论后，再由 Owner 指定 R1 首个小批次。*
