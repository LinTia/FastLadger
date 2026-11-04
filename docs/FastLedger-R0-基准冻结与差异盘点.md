# FastLedger R0：基准冻结与差异盘点

> 文档性质：**R0 阶段交付（盘点，非代码完成）**。本文件只做基准冻结、逐页差异盘点、根因分类与裁决清单，**不修改任何业务代码 / PRD / 落地方案 / 枚举字典 / WBS**。
> 依据：`docs/FastLedger-原型还原与PWA分阶段实施总方案.md` §5「R0」、§6「强制交付格式」；`workbuddy/多AI对齐规范.md`（颗粒度/编号/状态/文档权威）。
> 编制：实施 AI（WorkBuddy）。编制日期：2026-11-04。
> **独立复核更正**：本文件部分结论已由 [`FastLedger-R0-独立补漏与复核记录.md`](FastLedger-R0-独立补漏与复核记录.md) 修订；凡原文与复核记录冲突，以复核记录为准。复核仅为静态审查，未运行测试、未采集截图，R0 尚未通过。

---

## 0. 开工前回执（§6.1 强制）

| 项 | 内容 |
|---|---|
| 当前基线 SHA | 独立复核时 GitHub 默认分支最新可见 HEAD = **`1cd56ab7830d2d3f33faf7fc5372d3824c088ae0`**。原记录的 `f9f075e` 已过时；`55bbd4e` 的本地分支/工作区状态无法从 GitHub 只读视角验证，需本地重新检查。 |
| 工作分支 | 本次为 GitHub 默认分支只读复核；本地工作分支/工作区状态未验证。 |
| 关键决策文档（原编制回执所列） | `workbuddy/多AI对齐规范.md`、`workbuddy/第三方审阅者导读.md`、根 `CHANGELOG.md`、原型还原与 PWA 总方案、手机 PWA 指导书、Web 设计规范、交易更新语义设计、两版原型 README。原回执未列 PRD、落地方案、开发 WBS、枚举字典；不能据此证明原编制者已逐项核对。独立复核已另行对照这些文件，详见补漏记录。 |
| 本次范围 | R0-A ~ R0-G（页面/交互盘点、两版原型差异、当前实现与后端 API 映射、差异根因、截图与覆盖率方法、Owner 裁决清单） |
| **明确不做** | 不改 `server/`、`client/`、`server/web/dist/` 业务代码；不改 PRD/落地方案/枚举字典/WBS；不做 PWA 代码；不做 UI 重构；不给"还原度百分比"主观估值（见 §1.3） |
| 相关证据源 | `prototype/`（构建产物）、`prototype-v2/index.html`、`server/web/dist/{index.html,app.js,style.css}`、`server/internal/api/router.go`、`docs/FastLedger-Web设计规范.md` §8 |

### 0.1 与 GPT 评审的对齐说明（"与 GPT 对齐"）

1. **接受 GPT 的口径纠偏**：总方案 §3.3 明确不得把交接文档中未经评分矩阵证明的"50–55% 还原度"当作事实。本 R0 **不再复述该百分比**，改用"页面 × 关键交互 × 状态 × 断点"的加权矩阵（§7）统计，百分比须公布分母/权重/证据后才可出现。
2. **接受基准冲突的暂定规则**（总方案 §2.2）：`prototype/` 作页面/流程/交互基准；`docs/FastLedger-Web设计规范.md` + `design-tokens.json` 作视觉基准；`prototype-v2/` 仅作视觉补充，不得仅凭 v2 覆盖原型业务流程。**最终基准确认进 R0-G 裁决项 D-1**。
3. **区分"后端有能力"与"前端已接通"**：总方案 §3.1 指出不得把"有同名页面/能调 API"冒充原型还原。本 R0 的 R0-D 对每条后端能力标注"前端是否调用"，避免误判。
4. **不动已定决策**（导读 §5）：多账本、同步版本号、整数分、软删除/恢复/审计、隐私定位（图片不上传）等一律不改，仅为视觉一致不得破坏。
5. **本地分支状态**：原编制者报告其前端提交已 rebase 至当时 GPT `main` 之上；本次独立复核无法读取其本地工作区，也无法验证是否干净或是否存在未推送提交。GitHub 端只确认本报告列明的远端提交。

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
| 首页账单 | 概览三卡联动、按日分组账单、行点击进详情、筛选条、骨架屏 | `renderHome` L1947–1965；`renderBillStream` L1872–1927：**骨架屏与按日小计已实现**；行点击→`#b/:id/tx/:tid` | 首页无筛选；账单行缺账户/渠道；请求失败时 skeleton 未替换为错误态；日小计仅基于首批 60 条 | **部分具备**（见 R0-E-02、20、21、34、35） |
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

**状态覆盖缺口（R0-E-12）修正**：首页账单流已有骨架屏，不能将“骨架屏缺失”列为缺陷；但请求失败时 catch 仅 toast 并 return，骨架仍留在页面（R0-E-35）。空态已有局部实现；网络错误、权限不足、离线、同步中、冲突仍未形成逐页证据矩阵，需逐页验证并截图。

### 3.1 逐控件交互清单（扩展，满足总方案 §5 第 3 项「每个交互元素」要求）

> 来源：`server/web/dist/index.html`（`#loginForm`/`#formDlg`/`#editDlg`/`#confirmDlg`/`#catDlg`/`#acctDlg`/`#bottomNav`/`#topbar`）+ `app.js` 各 render/open 函数。标 `[需浏览器验证]` 表示代码路径存在但未经运行时截图确认；标 `[缺口]` 表示控件/行为缺失（关联 R0-E 编号）。

| 页面 | 控件 / 动作 | 前置 | 可观察结果 | 成功 / 失败反馈 | 数据联动 | 返回路径 |
|---|---|---|---|---|---|---|
| 登录 | 用户名/密码输入 | 无 token | 输入聚焦、明文切换 | 错误红字 toast；pending 禁用按钮 | — | 成功→`route()` 进账本或首页 |
| 登录 | 提交登录 | 表单有效 | `POST /auth/login` | 200→存 token 进首页；4xx→红字 | 写入 `localStorage` | — |
| 登录 | 切注册 / 加入账本 | — | 显示注册/邀请码表单 | 受 `EBK_ENABLE_REGISTER` 控制 | — | — |
| 首页 | 概览三卡（收入/支出/结余） | 已选账本 | `GET /reports/monthly?months=2` + `GET /accounts` | 数字渲染 | 第三卡直接 Σ账户余额（**口径错误，见 R0-E-20**） | — |
| 首页 | 账单流（按日分组） | 已选账本 | `GET /transactions?page=1&count=60`；骨架屏→`daySum` | 失败仅 toast，骨架残留（**R0-E-35**） | 日小计仅首批 60 条（**R0-E-34**） | 行点击→`#b/:id/tx/:tid` |
| 首页 | 账单行内容 | — | 分类+备注/商户+时间+金额 | 缺账户/渠道标签（**R0-E-21**） | — | 详情 |
| 首页 | FAB「记一笔」/ 底部 Tab | 移动端 | 打开记一笔弹窗 | — | — | 弹窗 |
| 首页 | 筛选入口 | — | **缺失**（R0-E-21） | — | — | — |
| 记一笔 | `#fType` 类型 | 弹窗开 | 普通 `<select>`（**非分段，R0-E-29**） | change 切双账户/分类显隐 | txType 字段 | — |
| 记一笔 | 金额/账户/对方账户/分类/日期/备注 | — | 受控输入 | 必填校验红字（**待浏览器验证**） | — | — |
| 记一笔 | 支付渠道 / 商户 | — | **无输入控件（R0-E-23 / R0-E-37）** | — | 后端 `CreateRequest` 也无 merchant 字段 | — |
| 记一笔 | 提交保存 | 表单有效 | `POST /transactions` | pending→toast→关闭→刷新首页/账单流 | 余额/报表回拉 | 首页顶部 |
| 交易详情 | 字段展示 | 进入 `#b/:id/tx/:tid` | `GET /transactions/:txId` | 缺 `paymentChannel`；备注非空时 merchant 不显示（**R0-E-22**） | — | — |
| 交易详情 | 编辑 | 详情页 | `openTxEditForm`（PUT 全量替换） | 类型只读；渠道不可编辑（**R0-E-23**） | 刷新详情 | 详情 |
| 交易详情 | 删除（二次确认） | — | `#confirmDlg` 文案「删除后将无法恢复」 | **与软删除/回收站矛盾（R0-E-24）** | DELETE→toast→回账单页 | 账单流 |
| 账户 | 新增 / 编辑 / 删除 | — | `POST/PUT/DELETE /accounts` | 刷新账户列表；引用保护 toast | 余额/净资产 | 账户列表 |
| 账户 | 期初余额输入 | — | **缺失（R0-E-06/30）** | 服务固定 `Balance:0` | — | — |
| 账户 | 本页净资产摘要 | — | **缺失（R0-E-28，在投资页）** | — | — | — |
| 分类 | 大类卡→子类、色板 | — | `GET /categories?tree=1` | 色板/默认分类待截图 | 引用计数确认删除 | 分类列表 |
| 成员 | 邀请生成 | owner/admin | `POST /members/invite`→`code/role/expiresAt`（**无 inviteId，R0-E-26**） | 展示邀请码 | — | — |
| 成员 | 邀请撤销 | — | **无 UI（R0-E-26）** | 后端有 `/invite/:inviteId/revoke` 但前端无 inviteId | — | — |
| 成员 | 角色升降 / 移除 | owner/admin | `PUT/DELETE /members/:uid` | toast+刷新 | 触发定向重拉 | 成员表 |
| 成员 | 账本可见性开关 | — | **无（R0-E-19/27）**；仅读 `mb.canViewOthers` 徽标 | 后端 `PUT /books/:id/visibility` 未接 | — | — |
| 我的 | 改邮箱 | — | `PUT /users/me` | toast 成功/失败 | — | — |
| 我的 | 8 个二级入口 | — | 跳转各页 | — | — | 各页 |
| 设置 | Token 生成/复制/撤销 | — | `GET/POST/DELETE /users/me/tokens` | 明文仅一次；确认撤销 | 退出回登录 | — |
| 设置 | 改密入口 | — | **缺失（R0-E-18；后端 `POST /users/me/password` 未接）** | — | — | — |
| 设置 | 会话管理入口 | — | **缺失（R0-E-14；后端 `/auth/sessions` 未接）** | — | — | — |
| 设置 | 服务器/备份区块 | — | **缺失（R0-E-17/ D-8）** | — | — | — |
| 设置 | 退出登录 | — | 清会话→登录页 | — | — | 登录 |
| 导出 | 时间范围预设 / 自定义 | — | 拼 `minTime/maxTime` | — | — | — |
| 导出 | 预览「共 N 条」 | — | `GET /transactions`（total） | — | — | — |
| 导出 | 下载 | — | `GET /export`→blob | 读 `X-Export-Count/Truncated` 头 | — | — |
| 预算 | 增/改/删 | — | `budgets` CRUD | 三态色待截图（R0-E-10） | 重绘 | 预算页 |
| 报表 | 月份切换 / 图表 | — | `GET /reports/monthly|categories` | 空态待截图（R0-E-09） | 重绘 | 报表页 |
| 投资 | 标的/持仓/行情/净资产 | — | `instruments/holdings/price/net-worth` | 净资产在投资页（R0-E-28） | 重绘 | 投资页 |
| 回收站 | 恢复 | — | `GET /recycle`；`POST /transactions/:txId/restore` | toast+回滚 | 列表/余额/报表 | 账单流 |

**逐项交互清单状态**：页面级与核心控件路径已枚举；标 `[需浏览器验证]` 的交互（校验红字、三态色、空态、骨架屏视觉）需 §7.2 截图协议落地后闭环。所有 `[缺口]` 行已与 R0-E 编号对应，可直接转为 R1 finding。

---

## 4. R0-C：`prototype/` 与 `prototype-v2/` 差异对照

| 差异点 | `prototype/`（旧，13 页） | `prototype-v2/`（新，规范示范） | 是否影响业务语义 | 建议基准 |
|---|---|---|---|---|
| 页面数 | 13 页（含**投资资产**独立页） | README 称"12 视图"；HTML 含「净资产」（账户视图汇总），**独立"投资资产"页是否存在 `[待确认]`** | 是（投资/持仓是否独立管理） | 以 `prototype/` 为准，v2 补视觉 |
| 记账入口 | "手动记账/转账" | "记一笔"（支出/收入/转账分段 + 截图识别入口） | 否（同一能力，措辞/手势不同） | 交互按 v2，能力按 prototype |
| 截图识别 | 有（截图识别记账页） | 有（三级漏斗演示，绝不自动入账） | Web OCR 由 PRD §5.2 明确不做；PWA 浏览器端本地 OCR 是否新增另行裁决 | PWA 扩展待确认 |
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
| 同步 | `GET /books/:id/sync/pull`、`POST /books/:id/sync/push` | JWT | ❌（当前 Web `app.js` 未调用） |
| WS | `GET /api/v1/ws` | JWT | ❌（后端路由存在；当前 Web `index.html` 仅加载 `/app.js`，`app.js` 未发现 WebSocket 客户端） |

**关键结论（修正）**：后端已有多组 API 路由，但路由存在不等于服务层、权限、DTO、测试均完整；本矩阵尚未达到总方案要求的契约粒度。当前 Web 未发现 WS 客户端或 sync pull/push 调用；OCR 的 Web 范围由 PRD §5.2 明确为不做；会话、可见性和邀请撤销等 API/能力不能据此推断前端已接通。
- `PUT /transactions/:txId` 已开放且是**全量替换 + 类型不可改**（改类型=删除+重建），R1 不得伪造类型变更。

### 5.3 后端 API 权限 / DTO / 测试矩阵（补全契约粒度）

> 方法/路径来自 `router.go` L109–235；鉴权分组：`v1` 组外为公开，`auth` 组（L135）全部 `JWTAuth`。**角色权限**列基于路由语义与已知 PRD/服务注释推定；具体以各 handler 内 `roleCheck`/可见性判定为准，**运行时权限边界需集成测试确认**（标 ✗待测）。响应关键字段为成功体 `data` 内对象；错误统一经 `mapSvcErr` 封装为 `{code:≠0, msg}`，**具体 code↔错误映射待测试补全**（标 ✗待测）。Web 调用列沿用 §5.2（✅/❌）。

| 能力域 | 方法 & 路径 | 鉴权 | 角色权限 | 请求关键字段 | 响应关键字段 | 错误码(示例) | 服务函数 | 测试证据 | Web |
|---|---|---|---|---|---|---|---|---|---|
| 健康 | `GET /healthz` | 公开 | — | — | `status,schemaVersion,instanceId?,restoreEpoch?` | — | — | — | — |
| 注册 | `POST /auth/register` | 公开(受`EBK_ENABLE_REGISTER`) | 任意 | `username,password` | `accessToken,user` | 400/409 ✗待测 | AuthService | CI 绿 ✗逐用例 | ✅ |
| 登录 | `POST /auth/login` | 公开 | 任意 | `username,password` | `accessToken,user`(R0 实测字段名 `accessToken`) | 401 ✗待测 | AuthService | CI 绿 ✗逐用例 | ✅ |
| 邀请入户 | `POST /books/:id/members/join` | 公开 | 持邀请码 | `code`(query/body) | `book,member` | 400/403 ✗待测 | MemberService | ✗待测 | ❌ |
| WS | `GET /ws` | `wsAuth`(Header/`?token=`) | 任意已认证 | `token`(Header 或 query) | 帧协议 | 401 ✗待测 | Hub | ✗待测 | ❌ |
| 当前用户 | `GET/PUT /users/me` | JWT | 本人 | PUT:`email` | `id,username,email` | 400/401 ✗待测 | AuthService | CI 绿 | ✅(GET/改邮箱) |
| 改密 | `POST /users/me/password` | JWT | 本人 | `oldPassword,newPassword` | `{}`(撤销全部会话) | 400/401 ✗待测 | AuthService | ✗待测 | ❌(R0-E-18) |
| PAT | `GET/POST/DELETE /users/me/tokens` | JWT | 本人 | POST:`name` | `token`(明文仅创建响应一次),`id` | 400/401 ✗待测 | PAT svc | ✗待测 | ✅ |
| 会话 | `GET /auth/sessions` | JWT | 本人 | — | `[{jti,device,createdAt,current}]` | 401 ✗待测 | SessionService | WBS5.10 ✗逐用例 | ❌(R0-E-14) |
| 会话 | `DELETE /auth/sessions` `[:jti]` | JWT | 本人 | jti(可选) | `{}` | 401 ✗待测 | SessionService | WBS5.10 ✗逐用例 | ❌ |
| OCR | `POST /ocr/{recognize,parse,llm-parse}` | JWT | 任意 | 图片/文本/`consent` | 解析结果 | 400/413 ✗待测 | OCR svc | ✗待测 | ❌(Web 不做,R0-E-08) |
| 账本列表 | `GET /books` | JWT | 任意(返回所属) | — | `[book]` | 401 ✗待测 | BookService | CI 绿 | ✅ |
| 建账本 | `POST /books` | JWT | 任意 | `name,bookType`(int8) | `book` | 400 ✗待测 | BookService | CI 绿 | ✅ |
| 账本详情 | `GET /books/:id` | JWT | 成员 | — | `book{name,bookType,createdAt,membersCanViewOthers,...}` | 403/404 ✗待测 | BookService | CI 绿 | ✅ |
| 改/删账本 | `PUT/DELETE /books/:id` | JWT | **owner/admin** | PUT:`name` | `book` | 403/404 ✗待测 | BookService | ✗待测 | ✅(GET 接,改/删前端未接→R0-E-32) |
| 可见性 | `PUT /books/:id/visibility` | JWT | **owner/admin** | `membersCanViewOthers`(bool) | `book` | 403 ✗待测 | BookService | ✗待测 | ❌(R0-E-19/27) |
| 成员列表 | `GET /books/:id/members` | JWT | 成员 | — | `[{uid,username,role,canViewOthers}]` | 403 ✗待测 | MemberService | CI 绿 | ✅ |
| 邀请 | `POST /books/:id/members/invite` | JWT | **owner/admin** | `role,boundUsername?,boundEmail?` | `{code,role,expiresAt}`(**无 inviteId**→R0-E-26) | 403/400 ✗待测 | MemberService | ✗待测 | ✅(生成) |
| 撤邀请 | `POST /books/:id/members/invite/:inviteId/revoke` | JWT | **owner/admin** | inviteId | `{}` | 403/404 ✗待测 | MemberService | ✗待测 | ❌(R0-E-26) |
| 改/删成员 | `PUT/DELETE /books/:id/members/:uid` | JWT | **owner/admin** | PUT:`role` | `{}` | 403/404 ✗待测 | MemberService | ✗待测 | ✅(生成邀请) |
| 账户列表/详情 | `GET /books/:id/accounts[/:aid]` | JWT | 成员(受可见性) | — | `[account]{id,name,category,balance,currency,...}` | 403 ✗待测 | AccountService | CI 绿 | ✅ |
| 账户增改删 | `POST/PUT/DELETE /books/:id/accounts[/:aid]` | JWT | 成员(受可见性;删可能 owner) | 增:`name,category,currency,parentId`(Balance 忽略→0) | `account` | 400/409(引用) ✗待测 | AccountService | CI 绿 | ✅(Balance 强制0→R0-E-06/30) |
| 分类列表 | `GET /books/:id/categories?tree=1` | JWT | 成员 | `tree` | `[{Category 提升字段,children:[Category]}]`(R0 误判已纠) | 403 ✗待测 | CategoryService | CI 绿 | ✅ |
| 分类增改删 | `POST/PUT/DELETE /books/:id/categories[/:cid]` | JWT | 成员 | 增:`name,parentId,catType` | `category` | 400/409(引用) ✗待测 | CategoryService | CI 绿 | ✅ |
| 交易列表 | `GET /books/:id/transactions` | JWT | 成员(受可见性) | `minTime,maxTime,txType,accountId,categoryId,keyword,page,count` | `{items:[tx],total,page,count}` | 403 ✗待测 | TxService | CI 绿 | ✅ |
| 交易详情 | `GET /books/:id/transactions/:txId` | JWT | 成员 | — | `tx{id,accountId,categoryId,amount,txType,note,merchant,paymentChannel,...}` | 403/404 ✗待测 | TxService | CI 绿 | ✅ |
| 交易创建 | `POST /books/:id/transactions` | JWT | 成员 | `clientUuid,txType,accountId,toAccountId,amount,categoryId,paymentChannel,txTime,timezoneOffset,comment,source`(**无 merchant 字段→R0-E-37**) | `{resultType,transactionIds,version,replayed}` | 400/409(幂等) ✗待测 | TxService | CI 绿 | ✅ |
| 交易更新 | `PUT /books/:id/transactions/:txId` | JWT | 成员 | 同创建 + `baseVersion`(**全量替换,类型不可改**→建+删重建) | `{version,replayed,transactionIds}` | 409(版本冲突) ✗待测 | TxService | `TestQATransactionRoutesContract`(守卫:禁 PUT /transfer /batch) | ✅ |
| 交易删除/恢复 | `DELETE /books/:id/transactions/:txId` `POST .../restore` | JWT | 成员 | — | `{}` | 403/404 ✗待测 | TxService | CI 绿(软删+回收站) | ✅(删除;恢复在回收站) |
| 回收站 | `GET /books/:id/recycle` | JWT | 成员 | — | `[tx]` | 403 ✗待测 | TxService | CI 绿 | ✅ |
| 月报/分类占比 | `GET /books/:id/reports/monthly\|categories` | JWT | 成员(受可见性) | `months` | `monthly:{items:[{month,income,expense}]}`;`categories:{items:[{categoryId,amount,ratio}]}` | 403 ✗待测 | ReportService | CI 绿 | ✅ |
| 导出 | `GET /books/:id/export` | JWT | 成员 | `minTime,maxTime,type,format,fields` | 文件流 + 头 `X-Export-Count/X-Export-Truncated` | 403/400 ✗待测 | ReportService | ✗待测 | ✅ |
| 预算增改删 | `POST/PUT/DELETE /books/:id/budgets[/:budgetId]` | JWT | **owner/admin(写)** | `name,budgetType,categoryId,periodType,amount,rollover,enabled` | `budget` | 400/403 ✗待测 | BudgetService | ✗待测 | ✅(CRUD 接) |
| 投资-标的 | `GET/POST/PUT/DELETE /books/:id/instruments[/:instrumentId]` | JWT | 成员 | `name,type,market,code` | `instrument` | 400/403 ✗待测 | InvestService | ✗待测 | ✅ |
| 投资-持仓 | `GET/POST/PUT/DELETE /books/:id/holdings[/:holdingId]` | JWT | 成员 | `instrumentId,quantity,cost,currentPrice?` | `holding` | 400/403 ✗待测 | InvestService | ✗待测 | ✅ |
| 投资-行情 | `POST /books/:id/instruments/:instrumentId/price` | JWT | 成员 | `price,asOf?` | `{}` | 400 ✗待测 | InvestService | ✗待测 | ✅ |
| 净资产 | `GET /books/:id/net-worth` | JWT | 成员 | — | `{totalAssets,totalLiabilities,holdingMarketValue,netWorth}`(公式:资产+持仓市值−负债) | 403 ✗待测 | InvestService | CI 绿 | ✅(投资页) |
| 同步拉 | `GET /books/:id/sync/pull` | JWT | 成员 | `since`(版本/时间戳) | 增量实体 | 403 ✗待测 | SyncService | ✗待测 | ❌(R0-E-15/25) |
| 同步推 | `POST /books/:id/sync/push` | JWT | 成员 | 增量实体 | `{ok}` | 403/409 ✗待测 | SyncService | ✗待测 | ❌ |

**矩阵结论**：
- 路由/服务函数映射完整；**逐路由权限断言、DTO 校验错误码、回归测试名三类证据仍标注 ✗待测**，需后续补 `mapSvcErr` 错误码表 + 各域 `*_test.go` 用例索引（建议作为 R0→R1 过渡的最小契约补全项，不修改业务行为）。
- 前端未接项（Web 列 ❌）已与 R0-E 编号一一对应：**改密(R0-E-18)、会话(R0-E-14)、可见性(R0-E-19/27)、邀请撤销(R0-E-26)、同步 pull/push(R0-E-15/25)、OCR(Web 不做,R0-E-08)**；邀请生成已接但**响应无 inviteId 导致无法撤销**(R0-E-26)。

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
| **D-2** | PWA 浏览器端是否新增本地 OCR？Web 本身是否做 OCR 已由 PRD §5.2 明确为“不做” | PRD §5.2：Web 不做 OCR；PWA 指导书提出浏览器端本地识别评估 | Web 不做 OCR 是既定要求；若 PWA 要增加浏览器端本地 OCR，属于 PRD 范围变更，需 Owner 明确批准并同步 PRD/WBS/隐私边界 | 仅 PWA 扩展待确认 |
| **D-3** | 导航是否按规范改为 4 Tab + "我的"中枢（拆除 10 项平铺顶栏）？ | 设计规范 §8.1；总方案 R2 | 建议：按规范改；保留账本切换入口于顶栏左上 | 待确认 |
| **D-4** | 期初余额的技术实现模型如何设计（需求本身不再待裁决）？ | PRD §4.5 明确要求支持期初余额；当前服务固定 `Balance:0`；服务注释建议用普通收入交易初始化 | 保留 PRD 需求；建议设计不进入收支统计的 opening-balance/余额调整记录，并同步数据模型、审计、同步、净资产和迁移契约；禁止伪装成普通收入 | 技术方案待 Owner 确认 |
| **D-5** | 投资资产是否独立成页？ | prototype/ 独立页；v2 疑似折入账户 | 建议：保留独立页（依 prototype/），v2 仅视觉 | 待确认 |
| **D-6** | 交易总表是否保留为独立导航入口，还是作为首页/我的的辅助检索视图？ | 多账本与回收站由 PRD/已定软删除要求支持，不属于可删扩展；交易总表是额外检索视图 | 建议保留交易总表能力，但由 Owner 裁决独立入口/导航归属；账本列表与回收站按既定需求保留 | 待确认（仅交易总表入口） |
| **D-7** | 多端会话管理是否属于产品需求？ | PRD §4.1/§4.12 已要求可查看与撤销会话；后端 `/auth/sessions` 已有路由，Web UI 未接（R0-E-14） | 需求已确定；Owner 只需决定排期/入口，不得裁决为“不需要”而不修订 PRD | 已确认需求，排期待定 |
| **D-8** | Web 设置页的服务器/备份区块展示到什么程度、数据从何处安全读取？ | PRD §4.12 与设计规范 §8.1–8.2 已要求服务器信息/备份入口；当前 Web 无状态 API 证据，运维备份另有部署机制 | 需求入口已确定；待 Owner 裁决 Web 只读状态范围、权限与是否提供受控备份/恢复操作。未设计安全边界前不得暴露恢复按钮 | 展示/操作边界待确认 |

> **R0 通过条件（2026-11-04 复核后修订）**：
> - **已完成（本轮补全）**：逐控件交互清单（§3.1，覆盖核心页面每个交互控件并绑定 R0-E 缺口编号）；API 权限/DTO/测试矩阵（§5.3，逐路由列鉴权/角色权限/请求响应字段/错误码/服务函数/测试证据/Web 调用）。
> - **仍待完成**：① §7 截图基线——本工作环境无浏览器/截图工具（Visualizer 仅产出 SVG/HTML，不能截取真实运行页面），`localhost:8080` 服务虽常驻但无法在此自动截图；该基线与"逐控件验证截图"须由 Owner 在浏览器或 CI 中按 §7.2 协议采集，属 R0→R1 过渡的执行项。② Owner 对 D-1、D-3、D-5、D-6（仅交易总表入口）、D-8 的范围结论，D-2 的 PWA 本地 OCR 裁决，D-4 期初余额技术实现方案；D-7 会话管理已确属需求仅排期待定。
> - **裁决前口径**：D-2 的 Web OCR 已由 PRD §5.2 决定为不做；D-4 需求已确定仅实现方案待确认；D-7 需求确定仅排期待定；这三者不再作为"是否做"的开放问题。
> - 满足上述①（截图基线落地）与②（Owner 对未决范围给出结论或明确阻塞）后，R0 方可通过并进入 R1 最小批次拆分。

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


## 10. 独立复核补漏（2026-11-04）

详细证据、根因、验收及页面交互补充见 [`docs/FastLedger-R0-独立补漏与复核记录.md`](FastLedger-R0-独立补漏与复核记录.md)。补充 R0-E-20 至 R0-E-37；其中 E-25/E-27/E-29/E-30/E-31 分别并入 E-15/E-19/E-05/E-06/E-14，不单独计数。新增独立差异覆盖首页财务口径、账单字段/筛选、交易详情/表单、商户字段端到端缺失、删除文案、邀请码撤销、净资产页面归属、账本重命名、README 状态漂移、日小计分页边界、加载错误态和 API 矩阵粒度。

**复核结论（2026-11-04 更新）**：原独立复核为静态审查，指出逐控件交互清单、完整 API 权限/DTO/测试矩阵、截图基线三项未完成。本轮已在 R0 主文档内补全前两项——§3.1 逐控件交互清单（绑定 R0-E 缺口编号）、§5.3 后端 API 权限/DTO/测试矩阵（逐路由列鉴权/角色权限/请求响应/错误码/服务函数/测试证据/Web 调用）。**唯一剩余缺口为 §7 截图基线**：本工作环境无浏览器/截图工具，无法自动截取真实运行页面；该项连同"逐控件验证截图"须由 Owner 在浏览器或 CI 中按 §7.2 协议采集，属 R0→R1 过渡执行项。Owner 对 D-1/D-3/D-5/D-6(入口)/D-8 范围结论、D-2 PWA OCR 裁决、D-4 期初余额技术实现方案给出后，R0 即通过。