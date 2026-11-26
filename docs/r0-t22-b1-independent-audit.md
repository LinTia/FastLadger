# R0-T22 批次 1（W1 + W8）独立审计报告

> **审计编号**：AUD-T22-B1
> **审计方**：第三 AI（独立于 WB，不参与 W1/W8 实施）
> **审计时点**：2026-11-27（Asia/Shanghai）
> **审计分支**：`review/t22-w1-w8-independent-audit`
> **计划基线**：`docs/r0-t22-execution-plan.md` @ `0033a1899878c2b832578cbfbb11ab858c2f8649`
> **代码基线**：`server/web/dist/app.js` @ `fb5ee1a6f4a2ddd79485141614293acf6b2e6697`（`git diff --stat origin/work/r0-t22-plan..fb5ee1a -- server/` 为空，代码树一致）
> **结论边界**：本报告**不构成 T22 通过**、**不构成第 3 条 PASS**、**不构成 R0 PASS**。R0 维持 BLOCK，第 3 条仍为阻塞。

---

## 0. 结论摘要

1. **审计对象部分不可达**：截至本次审计，远端所有分支中**不存在 WB 的 T22 执行分支**，仓库内也**不存在** `docs/r0-status-matrix-13x8.md`。因此本报告审计的是「计划 §3 预填矩阵 + 仓库实据」，并对 WB 未发布的 W1/W8 交付给出**验收基线**；不代替、不预判其交付。
2. **W1 预填矩阵存在 6 项 P1 问题**：首页/预算/报表的**有数据 normal 证据被漏计**；首页 **loading 证据被漏计**；单格同时写「有」和「缺 641」违反任务单三值口径；「有」判据在 members 与 invest 之间自相矛盾；`pairs` 证据代码基准已漂移却未标注；conflict 整列 N/A 不成立（与 W8 交叉，见第 3 点）。
3. **W8 的「offline/conflict 全 N/A」不成立**（P1）：`conflict` 在 Web 有可触发的运行时载体（`baseVersion` → 409 → toast），且 PRD/落地方案已定「覆盖/放弃/另存副本」三选一，属**实现不完整 + 缺证据**，不能记 N/A；`offline` 的 N/A 成立，但必须逐格给理由并挂 R0-E-15/25。
4. **证据基准已过期**（P1）：`pairs/README` 声称「`server/` 自 `bff6a2b` 起零改动」，实测 `app.js` 在其后被 `0472484` 改动 52 行（`+43/-9`），**手动记账、家庭成员**两页的 normal 图与当前 HEAD 行为不一致；直接复用 pairs 作 normal 基线会把已修缺陷当成现状。
5. **按已确认证据修正，矩阵三计数应为「有 17 / 缺 53 / N/A 34」，不是计划的「13 / 54 / 37」**；另有 5 格判据待冻结（导出 empty、手动记账 empty、成员 empty、投资 normal、首页 permission），可能继续移动计数。若只修 W1 的截图归类、不动 W8 的 conflict 判定，则为「17 / 50 / 37」——两套数都列出，避免把 W8 的修正隐掉。

---

## 1. 审计对象与可达性（先钉事实）

### 1.1 远端分支与交付文件

```text
$ git ls-remote git@github.com:LinTia/FastLadger.git
（节选）
b4309dbd...  refs/heads/main
fb5ee1a6...  refs/heads/work/r0-t21-page-map
0033a189...  refs/heads/work/r0-t22-plan
b6199fae...  refs/heads/work/r0-t25-doc-align
（其余为 docs/、fix/、review/、audit/ 历史分支）

$ git ls-tree -r --name-only origin/work/r0-t22-plan | grep -i 'matrix'
docs/r0-evidence-5th/ci-coverage-matrix.md
docs/r0-evidence-7th-fix/qa/acceptance-matrix.md
（无 docs/r0-status-matrix-13x8.md）
```

**结论**：W1 的交付物（状态矩阵文件）与 W8 的交付物（边界取证段落）在本报告审计时点**均未进入任何远端 ref**。本次审计因此分为两层：

- 对**计划层**（§3 预填、W1/W8 执行卡、§8 验收标准）做独立核对——这是 WB 实施的输入，错误会直接传导到交付；
- 对**仓库实据层**（代码、截图、README）做独立核证，形成 WB 交付时的**可验证验收基线**（§7）。

**未覆盖声明**：若 WB 存在未推送的本地提交，本报告不评价其内容；WB 交付后须按 §7 逐条复跑，不能以本报告替代复跑。

### 1.2 必须遵守的门禁与任务口径

| 口径 | 出处 | 原文要点 |
|---|---|---|
| 第 3 条门禁 | `docs/FastLedger-R0-收尾执行清单.md:183` | 「prototype 与实现同视口、同状态的对照基线完成；截图覆盖适用的 normal/loading/empty/error/success/permission/offline/conflict 状态」 |
| N/A 必须有理由 | 同文件 `:109` | 「每张截图必须能追溯页面、视口、状态、数据前置条件、动作和预期结果……对不适用的状态写 N/A，并给出理由」 |
| 现具备项 | 同文件 `:100` | 「Loading/pending：**首页列表 pending 截图**」（门禁自身承认首页 pending 已有） |
| 后端路由红线 | 同文件 `:102` | 「不能把后端路由存在写成 Web 已接通」 |
| T22 修改要求 | `docs/FastLedger-R0-收口总任务单-2026-11-25.md:549` | ① 逐格填 `图路径` / `N/A + 理由` / `缺`；② 三视口至少 390/641/1280；③ 同数据同状态同视口；④ 离线/冲突注明边界；⑤ OBS 依赖 T11 |
| T22 验收 | 同文件 `:551` | 「矩阵每格有图或带理由的 N/A；OBS `closed=true`；三视口齐备；无整页 PASS」 |
| 冲突 UX 已定 | `docs/FastLedger-产品需求文档-PRD.md:469`；`docs/FastLedger-落地方案.md:366`、`:928`、`:939` | 服务端权威 + 版本号，冲突返回 409，用户「覆盖 / 放弃 / 另存副本」三选一 |
| 开工前提 | `docs/r0-t22-execution-plan.md:331` | 「可立即开工：W1（只读建矩阵骨架）、W8（只读边界取证）」；OD-03 未取得 |

---

## 2. 独立取证（命令与实测输出）

本节所有输出均为本次审计实际执行，未引用计划或 T21 的转述结论。

### 2.1 页面范围

```text
$ sed -n '11,17p' prototype/README.md
## 页面清单（13 个）
登录、首页账单、截图识别记账、手动记账/转账、交易详情、账户、两级分类、投资资产、预算、报表、导出、家庭成员、我的设置。

$ sed -n '94p' docs/r0-screenshots/pairs/README.md
| settings | `/profile`（二级菜单） | `#settings` | 基本一致 | **原型无独立设置路由**：`settings` 与 `mine` 实际是同一个 `/profile`，两图 SHA256 相同，**不构成两个独立原型页面**，统计页数时应按 12 个独立原型路由计 |
```

结论：**文档口径 13 页、独立原型路由 12 个**。两个数都真实，但用途不同（前者是门禁清点单位，后者是覆盖率分母）。矩阵必须同时声明，不能混用（见 F-09、F-10）。

### 2.2 截图库存（152 张的构成）

```text
$ find docs/r0-screenshots -name '*.png' | wc -l
152
$ find docs/r0-screenshots -name '*.png' -not -path '*/pairs/*' -not -path '*/runtime/*' | wc -l
43
$ find docs/r0-screenshots/pairs -name '*.png' | wc -l
104
$ find docs/r0-screenshots/runtime -name '*.png' | wc -l
5
$ find docs/r0-screenshots -name '*641*' | wc -l
0
$ PNG 魔数抽查（43 + runtime 全部）→ 非 89504e470d0a1a0a 者 0
```

结论：152 = 43 + 104 + 5 成立；**641 档零覆盖**成立；PNG 文件有效。

### 2.3 13 页实现函数行号（当前 HEAD）

```text
  161| function renderLogin(mode) {
 1963| async function renderHome(bookId) {
  548| function openTxForm(bookId, onDone) {
 2008| async function renderTxDetail(bookId, txId) {
  678| async function renderAccounts(bookId) {
 2103| async function renderCategories(bookId) {
 1277| async function renderInvest(bookId) {
 1139| async function renderBudget(bookId) {
  835| async function renderReport(bookId) {
  964| async function renderExport(bookId) {
 1667| async function renderMembers(bookId) {
 2172| async function renderMine(bookId) {
 1549| async function renderSettings() {
 1906| async function renderBillStream(bookId, container) {
 1802| async function renderAccountDetail(bookId, accId) {
```

结论：13 页 render 行号与计划 §1.3 一致；首页由 `renderHome` + `renderBillStream` 两段组成，交易详情/账户详情是独立函数。

### 2.4 状态载体的代码事实

```text
$ grep -n 'skel\|aria-busy\|className = "empty"\|btnPending\|renderLoadError' server/web/dist/app.js
 56: function btnPending(btn, loadingText)
190/207: 注册中… / 登录中…
243: renderBooks 空态
389/407/413/424: renderTxns 骨架屏 + 页级错误框（非原型页）
481: 交易总表空态（非原型页）
736: 账户空态
823: 回收站空态（非原型页）
909: 报表「该月暂无支出记录」
1164: 预算空态
1324/1385: 投资 持仓空态 / 标的库空态
1837: 账户详情空态
1908-1909: 首页账单流 4 条骨架（aria-busy）
1917: 首页账单流空态
```

逐页适用性要点（**代码已确认**）：

- **骨架屏仅存在于 2 处**：首页账单流（`:1908-1909`）与交易总表（`:424`，非 13 页之一）。其余 11 个原型页**没有页面级 loading UI**，只有按钮 pending（`btnPending`，如 `:207`、`:601`）。
- **页级错误框仅存在于交易总表**（`renderLoadError`，`:413`）。其余页失败时是 `toast`，部分页先 `view.replaceChildren()` 再失败 → 页面空白（如 `renderAccounts:684` 后无 try/catch）。
- **空态实现分布**：账户 `:736`、分类 `:2131`（`.cat-empty`，不是 `.empty`）、报表 `:909`、预算 `:1164`、投资 `:1324/:1385`、账单流 `:1917`、账户详情 `:1837`；导出页无列表，空数据反馈在预览面板（`:1066`）；成员/设置/我的无空态。
- **登录 pending**：`btnPending(b, "登录中…")`（`:207`），是 loading 的按钮级载体。

### 2.5 W8 的 grep 逐条复现

```text
$ grep -c -e "WebSocket" -e "ws://" -e "wss://" server/web/dist/app.js
0
rc=1

$ grep -n -e "sync/pull" -e "sync/push" server/web/dist/app.js
（无输出）
rc=1

$ grep -n "sync" server/internal/api/router.go
98:	sh := &syncHandler{svc: d.Sync}
234:		auth.GET("/books/:id/sync/pull", sh.pull)
235:		auth.POST("/books/:id/sync/push", sh.push)

$ 扩展（审计方补查）：EventSource / BroadcastChannel / navigator.onLine / offline / indexedDB / localStorage 队列
app.js 与 index.html 均 0 命中（localStorage 仅存 token/whoami，见 :12-13、:196-197、:212-213）
```

结论：计划 §4.3 的三条 grep 可复现，**但不完整**：只查了 `app.js`，没查 `index.html`；未记录 `rc`；"0 处 sync" 的表述会被字面命中质疑：排除 `async` 后全文仍有 4 处——`syncFormFields` 3 处（UI 表单联动，`:556/:572/:578`）与 `tx.syncVersion`（`:655`），见 F-13。

### 2.6 前端确实存在的冲突链路（W8 的关键反证）

```text
$ sed -n '645,671p' server/web/dist/app.js
655:      baseVersion: tx.syncVersion || 0,
664:      await api("PUT", "/books/" + bookId + "/transactions/" + tx.id, body);
669-670:  } catch (err) {
670:        toast(err.message + (String(err.message).indexOf("并发") >= 0 ? "（请刷新列表后重试）" : ""), "err");

$ sed -n '128,136p' server/internal/service/tx_update.go
130:	// 4) baseVersion 乐观锁：整笔各行必须仍处于客户端所见的版本（设计 §6）
131:	if r.BaseVersion > 0 {
132:		for _, gr := range group {
133:			if gr.SyncVersion != r.BaseVersion {
134:				return nil, fmt.Errorf("%w：交易 %d 已被他人更新（行版本 %d，请求基于 %d），请刷新后重试",
135:					ErrConcurrentConflict, ...)

$ sed -n '63,65p' server/internal/api/common.go
64:	case errors.Is(err, service.ErrConcurrentConflict):
65:		fail(c, http.StatusConflict, "数据已被并发修改，请重试")
```

结论：**Web 端存在「版本冲突 → 409 → toast（含刷新提示）」的完整可触发路径**，只是没有 PRD 要求的「覆盖 / 放弃 / 另存副本」三选一 UI，也没有冲突态截图。因此冲突不是「无运行时载体」。

同类 409 还存在于：账户更新 `server/internal/service/account.go:239`、分类更新 `server/internal/service/category.go:234`（`RowsAffected != 1` → 409）；预算表**无 `sync_version`**（`server/internal/model/budget.go:22`），故预算页冲突可判 N/A。

### 2.7 pairs 证据的代码基准已漂移

```text
$ grep -n '业务代码基准' docs/r0-screenshots/pairs/README.md
13:| 业务代码基准 | `bff6a2b351bf5a039cdf7c762ec2693fb330cd00`（`server/` 自该提交起零改动） |

$ git diff --stat bff6a2b..HEAD -- server/
 server/web/dist/app.js | 52 +++++++++++++++++++++++++++++++++++++++++---------
 1 file changed, 43 insertions(+), 9 deletions(-)

$ git log --format='%h %ad %s' --date=short bff6a2b..HEAD -- server/web/dist/app.js
0472484 2026-11-05 fix(r0): 有限范围修复第六次验收 4 项业务缺陷 + 独立回归门禁
```

改动实际影响两处原型页：

- `openTxForm`：新增 `refillCategories()`，按 `catType` 过滤分类，并在无分类时渲染「（暂无可选分类）」（`app.js:560-571`）。此前下拉会混入收入类「工资」（pairs README §5.1 记录的缺陷），**现已修**。
- `renderMembers`：邀请按钮改为确认 `myRole === 1 || 2` 后才挂载，`users/me` 失败则失败关闭（`app.js:1706-1741`）。

结论：`pairs/README:13` 的「零改动」陈述**与仓库事实不符**；`pairs` 里手动记账、家庭成员两页的图采集于修复前代码，不能无条件当作「当前实现 normal 基线」。

---

## 3. W1 核对结果（13 页 × 8 状态）

### 3.1 页面范围

| 检查点 | 计划写法 | 审计结论 |
|---|---|---|
| 13 行齐备 | §3 表 13 行 | **通过**（程序复算见 §3.5） |
| 页名与原型一致 | 登录…我的设置 | **通过**（`prototype/README.md:11`） |
| 页 3 处理 | 全列 N/A | **方向通过**，但 N/A 理由未逐格写；且计入 104 分母（见 F-09） |
| 页 13 处理 | 一行合并 mine + settings | **不通过**：两实现视图、一原型路由，单行无法表达三视口齐备（见 F-10） |
| 非原型 3 页（books/transactions/recycle） | 矩阵外单列（P2） | **通过**（建议不变） |

### 3.2 状态定义：当前缺失，必须冻结

计划只给了 8 个英文状态名，**没有给任何可观察判据**。同一张图可被两人判成不同状态（例：pairs 投资页有资产、无持仓，算 normal 还是 empty？）。以下判据由本次审计依据代码与门禁提出，**属建议，需 Owner/WB 在 W1 冻结后才可执行**：

| 状态 | 可观察判据（建议冻结） | 本仓库载体 |
|---|---|---|
| normal | 页面主数据请求成功，且**该页主列表/主数据非空**；静态页（设置/我的/导出）以「首屏完整可见」为准 | 43 张 + pairs 52 张实现侧 |
| loading | 请求 **pending 期间**可见骨架条（`.skel`/`aria-busy=true`）或按钮 pending 文案（「登录中…/保存中…」） | 首页 `:1908`、交易总表 `:424`、`btnPending:56` |
| empty | 主列表返回 0 条，页面渲染**明确的空态文案**（`.empty`/`.cat-empty`/预览文本） | 账户 `:736`、分类 `:2131`、报表 `:909`、预算 `:1164`、投资 `:1324/1385`、账单流 `:1917` |
| error | 主数据请求失败，页面出现**页级错误框**或**错误 toast 且可归因到具体请求分支** | 交易总表 `:413` 页级；其余页 toast |
| success | 完成一次写/交互动作后，出现成功 toast、列表重绘、跳转或下载 | 全站 toast/重绘，**0 张截图**（`截图README:189`） |
| permission | 同一页面在 owner/admin/member/被移除角色下**可见性或可操作性不同**，或有 403/404 隐私行为 | 仅 owner 截图；403 走 `api()` 抛错（`:110`） |
| offline | 断网后页面**仍可操作**或有离线队列/重连提示 | **无**：0 处 `navigator.onLine`、0 队列、0 IndexedDB |
| conflict | 写请求返回 409 且 UI 出现**冲突处置**（三选一/合并/重试指引） | 有 409 + toast 指引，**无三选一 UI** |

> 未冻结判据前，「有/缺/N/A」都无法复算；这是 W1 第一优先项，不是文档美化。

### 3.3 适用性与 N/A 理由

| 计划 N/A 判定 | 审计结论 | 依据 |
|---|---|---|
| 页 3 全 8 格 N/A | **成立**（有理由的 N/A） | `PRD:326-333` §5.2「Web 不做 OCR，用于浏览与编辑」 |
| 页 1 empty N/A（无列表） | **成立** | `renderLogin` 仅表单，无列表 |
| 页 1 permission N/A（未登录无角色） | **成立** | 未认证态无角色 |
| 页 4 empty N/A（弹层无空态） | **不成立/至少待定** | `app.js:568` 在无分类时渲染「（暂无可选分类）」，是空态提示；账户为空时 select 空白无引导。需明确「弹层不算独立页面」才可 N/A（F-12） |
| 页 5 empty N/A（详情必有数据） | **成立** | 直接访问不存在 tx 走 `:2017` toast + 回列表，属 error 非 empty |
| 页 13 empty N/A（设置页无空态） | **成立**（但 mine 侧未单独论证） | 两页均静态内容；mine 依赖 books，无列表 |
| 全部 offline N/A | **成立**，但**理由缺失** | 代码 0 处离线能力；须写明「Web 无离线模式/队列」并挂 R0-E-15/25（F-07） |
| 全部 conflict N/A | **不成立** | 见 F-06：409 链路存在，PRD 要求三选一 |
| 页 12 empty 判「缺」 | **待定** | 成员页至少有 owner 本人，永不空；应判 N/A + 理由，或明确「成员列表为空」不可达 |

### 3.4 逐行核对表（审计判定）

图例：`有4` = 有图 4 档；`有2r` = 有图 2 档且为隔离运行时；`缺` = 缺且无载体；`缺*` = 缺但载体存在（仅需采图）；`N/A` = 成立；`N/A?` = 计划判 N/A 但理由不足；`待定` = 判据未冻结。

| # | 页面 | normal | loading | empty | error | success | permission | offline | conflict |
|---|---|---|---|---|---|---|---|---|---|
| 1 | 登录 | 有4 | 缺* | N/A | 缺* | 缺 | N/A | N/A | N/A |
| 2 | 首页账单 | **有4**（pairs） | **有2r** | 有4 | 有3（390） | 缺 | 待定 | N/A | N/A |
| 3 | 截图识别 | N/A | N/A | N/A | N/A | N/A | N/A | N/A | N/A |
| 4 | 手动记账 | 有4 | 缺* | N/A? | 缺* | 缺 | 待定 | N/A | N/A（幂等冲突另注） |
| 5 | 交易详情 | 有4 | 缺* | N/A | 缺* | 缺 | 待定 | N/A | 缺* |
| 6 | 账户 | 有4 | 缺* | 缺* | 缺* | 缺 | 缺* | N/A | 缺* |
| 7 | 两级分类 | 有2 | 缺* | 缺* | 缺* | 缺 | 待定 | N/A | 缺* |
| 8 | 投资资产 | 待定 | 缺* | 有2 | 缺* | 缺 | 缺* | N/A | 待定 |
| 9 | 预算 | **有4**（pairs） | 缺* | 有2 | 缺* | 缺 | 缺* | N/A | N/A（无 `sync_version`） |
| 10 | 报表 | **有4**（pairs） | 缺* | 有2 | 缺* | 缺 | 待定 | N/A | N/A（只读） |
| 11 | 导出 | 有2 | 缺* | N/A? | 缺* | 缺 | 待定 | N/A | N/A（只读） |
| 12 | 家庭成员 | 有2（数据未对齐） | 缺* | N/A? | 缺* | 缺 | 缺* | N/A | N/A（容量 409 非版本冲突） |
| 13 | 我的设置 | 有（2 + 4） | 缺* | N/A | 缺* | 缺 | 待定 | N/A | N/A |

> 与计划 §3 的差异共四类：① 4 格由「缺」改「有」（粗体：首页 normal / 首页 loading / 预算 normal / 报表 normal）；② conflict 列 3 格由 `N/A` 改「缺（有载体）」（交易详情 / 账户 / 分类）；③ 5 格标 `待定`（投资 normal、首页 permission、导出 empty、手动记账 empty、成员 empty）；④ 其余仅补 `缺*`「载体已存在」标注。**未新增任何任务**，也未改 W2–W9 的范围。
>
> 标 `（pairs）` 的三格存在证据基准漂移（见 F-05），复核前只能算「有条件的有」；表内仍计入「有」，因为它确实有图，但不能据此宣称「同数据同状态已达成」。
>
> 本表是**判定底稿**，其中 5 格标 `待定`（投资 normal、首页 permission、导出 empty、手动记账 empty、成员 empty），未计入任何一类。§3.5 的计数把这 5 格**暂按计划原值**处理（依次为 缺 / 缺 / 缺 / N/A / 缺），并在 conflict 列把交易详情/账户/分类 3 格由 N/A 改「缺」。所以本表逐格数量与 §3.5 不直接相等，**以 §3.5 为计数口径**。

### 3.5 统计口径复算

计划声明：**有 13 / 缺 54 / N/A 37 = 104**。程序复算（按单元格首词分类）结果一致，**说明算术没错，错的是分类**：

```text
$ python3 - <<'PY'  # 解析 §3 表
数据行: 13
合计: {'有': 13, '缺': 54, 'N/A': 37} 总格: 104
PY
```

按 §3.4 的已确认修正（含 W8 的 conflict 拆页）：

| 项 | 计划 | 审计修正 | 差额原因 |
|---|---|---|---|
| 有 | 13 | **17** | 首页 normal（+1）、预算 normal（+1）、报表 normal（+1）、首页 loading（+1） |
| 缺 | 54 | **53** | 同上 4 格由缺转有（−4），交易详情/账户/分类 conflict 3 格由 N/A 转缺（+3） |
| N/A | 37 | **34** | 同上 3 个 conflict N/A 转为「缺（有载体）」 |
| 合计 | 104 | **104** | 恒等 |

若只做 W1 的截图归类、暂不动 conflict，则为 **有 17 / 缺 50 / N/A 37**；两个口径都保留，便于分别验收 W1 与 W8。

**5 个待定格**：投资 normal、首页 permission、导出 empty、手动记账 empty、成员 empty。任一改判都会移动三计数；W1 必须先冻结判据再出终值。

---

## 4. W8 核对结果（offline / conflict 边界）

### 4.1 计划的三条结论，逐条审计

| 计划结论 | 审计判定 | 说明 |
|---|---|---|
| 前端 0 处 `WebSocket` / `ws://` / `wss://` | **成立**（app.js + index.html 均为 0） | 补查 `EventSource`、`BroadcastChannel` 亦为 0 |
| 前端 0 处 `sync/pull`、`sync/push` 调用 | **成立**（rc=1，无输出） | 但 `sync` 字面有 4 处（`syncFormFields` ×3 + `tx.syncVersion`），须在证据中排除说明 |
| 后端 `router.go:234-235` 有 sync 路由 | **成立**（行号精确） | 另有 `router.go:133` `v1.GET("/ws", ... ws.ServeWS)`，计划未列 |
| 故两状态「Web 无运行时载体」 | **offline 成立；conflict 不成立** | conflict 有 409 链路（§2.6） |
| 24 格全填 N/A | **不成立** | 见 F-06、F-08 |

### 4.2 后端存在 ≠ Web 接通（正确，但证据不全）

计划引用 `router.go:234-235` 证明后端有路由，并正确写出「不得把后端路由存在写成 Web 已接通」。审计补充：

- WS 服务端**确实在运行链路上**：`router.go:133` 注册 `/api/v1/ws`，`broadcast.go:99` 调 `ws.DefaultHub.BroadcastToBook`，`:122` `notifyDataChanged` 在每次写后被调用（如 `ledger_handler.go:135/161`、`tx_handler.go:38/141`）。
- 但 **Web 客户端从未连接**：0 处 `WebSocket`、无 `/ws` 字面、无重连/心跳/退避逻辑；`app.js` 也没有任何轮询或 `visibilitychange` 刷新（0 命中 `setInterval`）。
- 因此现状是：**服务端会广播，Web 端收不到，也不会自动拉取**；用户只能靠重新导航/手动刷新看到他人改动。这个边界比计划写的「0 处 sync 调用」更准确，也更能解释「多端并发为何会撞 409」。

### 4.3 conflict 的正确判定

按页拆分（建议写进矩阵）：

| 页面 | 判定 | 依据 |
|---|---|---|
| 5 交易详情 | **缺（有载体）** | `baseVersion` → `tx_update.go:131-135` → 409 → `app.js:670` toast「（请刷新列表后重试）」 |
| 6 账户 | **缺（有载体）** | 更新 `RowsAffected != 1` → `account.go:239` 409；前端 `app.js:719` toast |
| 7 两级分类 | **缺（有载体）** | `category.go:234` 409；前端 `:2101` toast |
| 8 投资资产 | **待定** | 持仓/标的写路径经版本服务，并发删除会 409（`invest.go` 写路径），但前端未发送 `baseVersion`，仅服务端行数校验 |
| 9 预算 | **N/A + 理由** | 预算表无 `sync_version`（`model/budget.go:22`），不参与增量同步契约 |
| 12 家庭成员 | **N/A + 理由** | 409 为「成员上限/目标已移除」等容量与生命周期语义（`common.go:116`、`member.go:283`），不属版本冲突三选一 |
| 1/2/3/4/10/11/13 | **N/A + 理由**（4 的创建幂等冲突除外，属边界说明） | 只读页或无版本写 |

> 重要：判「缺」不等于宣布缺陷已修复或门禁已过。它只表示「该状态有可触发路径，尚未采集证据、尚未实现 PRD 要求的三选一」。

### 4.4 offline 的正确判定

**offline 在 Web R0 无运行态载体，N/A 成立**，但理由必须写全三句：

1. 前端无 `navigator.onLine` / `online` / `offline` 事件监听（0 命中）；
2. 无离线队列、无 IndexedDB/localForage，`localStorage` 只存 `fl_token`/`fl_whoami`（`app.js:12-13`）；
3. 无 WS/长连接可断线重连（0 命中），断网只表现为请求失败 toast。

并须挂 **R0-E-15 / R0-E-25**（Web 未接实时同步客户端）作为已知缺口登记。**不得**把 N/A 写成「该状态不适用，故无缺口」。

### 4.5 24 格 vs 26 格

矩阵是 13 行 × 2 列 = **26 格**；计划 W8 写「24 格（12 页 × 2）」。差额正是页 3 的两格。必须在矩阵中显式写：

```text
offline/conflict 共 26 格 = 12 个实采页 × 2（24 格）+ 页 3 × 2（2 格，随页 3 全行 N/A）
```

否则「24 格全 N/A」与「13 行矩阵」无法对上，复核方无法判断页 3 是否漏填。

---

## 5. 发现清单

> 字段按 `workbuddy/多AI对齐规范.md §2` 收敛：id / title / severity / location / quote / problem / reproduce / actual / expected / impact / suggestion / status。
> 严重度按同规范 §3：P0 核心闭环不成立；P1 门禁会被误判/证据失真；P2 一致性/可维护性；P3 优化。

### F-01 · P1 · 有数据 normal 被记「缺」，且与 §8.3 自相矛盾

- **location**：`docs/r0-t22-execution-plan.md:258`（首页）、`:265`（预算）、`:266`（报表）、`:388`（§8.3）
- **quote**：`| 2 | 首页账单 | 缺（有数据态未采） | …`；§8.3「`pairs` 现有 104 张可复用为 normal 态基线」
- **problem**：`pairs/impl/home|budget|report__impl__390x844.png` 实图为**有数据态**（首页本月收入 ¥28,000 / 支出 ¥1,089.30 / 结余 ¥30,126.70；预算 5 条且含进度；报表 6 月柱状 + 分类占比）。§3 记「缺」，§8.3 又说 pairs 可复用——同一文档两处互斥。
- **reproduce**：打开上述 3 张图；或 `ls docs/r0-screenshots/pairs/impl/{home,budget,report}__impl__390x844.png`
- **actual**：三格均「缺」，计数 有 13 / 缺 54
- **expected**：三格均列出 pairs 4 档路径（标注「缺 641」），计数至少 有 16 / 缺 51
- **impact**：W1 会把已有证据记为待采，虚增 W3 工作量；若按 §8.3 复用，又与「缺」冲突，验收时无法判定用哪套。
- **suggestion**：删掉「有数据态未采」；三格改为图路径 + 视口缺口；§8.3 增加「须先复核代码基准」前置（见 F-05）。
- **status**：待 WB 处理

### F-02 · P1 · 首页 loading 被记「缺」，与运行时证据和门禁自述冲突

- **location**：`docs/r0-t22-execution-plan.md:258`、`:177`；`docs/r0-screenshots/runtime/README.md`（文件表）；`docs/FastLedger-R0-收尾执行清单.md:100`
- **quote**：W4「`runtime/01-pending-390x844.png` 是首页隔离运行时证据，**非全页**」
- **problem**：`runtime/01-pending-390x844.png` 是**首页整页** pending 截图：概览三卡已渲染、账单流位置 4 条骨架条、`aria-busy=true`；`runtime/04-pending-1280x800.png` 同态桌面视口。runtime/README 记录 `.skel=4`、`aria-busy=true`。收尾清单 `:100` 也明写「已具备：首页列表 pending 截图」。「非全页」无依据。
- **reproduce**：打开 `docs/r0-screenshots/runtime/01-pending-390x844.png`；`grep -n 'pending\|skel' docs/r0-screenshots/runtime/README.md`
- **actual**：首页 loading = 缺
- **expected**：首页 loading = 有（390/1280 两档，隔离运行时、单次实测；缺 641）；并在格内注明「隔离实例 + 选择性拦截」
- **impact**：W1 会重复采集首页 loading；且与门禁现状描述矛盾，复核方会认为 W1 未读门禁。
- **suggestion**：改判为「有 2r」，把 runtime 01/04 列入证据；W4 验收改为「其余 11 页 + 首页 641 档」。
- **status**：待 WB 处理

### F-03 · P1 · 单格同时写「有」和「缺 641」，违反三值口径

- **location**：`docs/r0-t22-execution-plan.md:255-269`；任务单 `:549` ①、`:551`
- **quote**：表头「视口以 390/641/1280 三档为**强制**」；单元格「有（4 档，**缺 641**）」；任务单「逐格填 `图路径` / `N/A + 理由` / `缺`」
- **problem**：按 ② 三视口强制，缺 641 的格在门禁口径下**未达成**；同时「有」又表示已有图。一个格承载两个互斥结论，既不满足三值口径，也无法复算「有 13」到底代表什么。
- **reproduce**：把 §3 表转成 10 列矩阵后跑 §7 的 V-09 脚本，输出 `混值:1 2 4 6 7 8 9 10 11 12 13`（11 格）——即 13 个「有」格中 11 个同时含「缺 641」；另 2 格（第 5 行「有（pairs 4 档）」、第 2 行 error「有（仅 390）」）虽未写 641，但同样没有声明三档齐备。
- **actual**：11 个混合值格 + 2 个「有但视口不全」格；「有 13」被表述为覆盖进度
- **expected**：拆为两列（存在性 / 三视口完整度），或每格只取一值（641 缺则记「缺」，图路径进备注）
- **impact**：最危险的一条——「有 13」容易被下游误读成「13/104 已覆盖」，而实际三视口齐备为 **0/96**。
- **suggestion**：矩阵加「视口完整度」独立列（390/641/1280 三列 ✓/✗）；汇总同时给「存在图格数」与「三视口齐备格数」。
- **status**：待 WB 处理

### F-04 · P1 · 「有」判据不一致：members 记有、invest 记缺

- **location**：`docs/r0-t22-execution-plan.md:264`（投资）、`:268`（成员）；`pairs/README.md:88`、`:90`
- **quote**：invest「缺（有数据态未采，见 P4）」；members「有（2 档，缺 641）」；pairs 两行均标「**未对齐**」
- **problem**：两页都「两侧数据未对齐」，却一个缺、一个有。且 pairs invest 是**混合态**（账户资产非零、持仓/标的为空），members 是 1 人 vs 原型 2 人。判据没有说明「有」是「有图」还是「数据对齐」。
- **reproduce**：打开 `pairs/impl/invest__impl__390x844.png` 与 `pairs/impl/members__impl__390x844.png`
- **actual**：同类证据两种结论
- **expected**：先冻结「存在性」与「可用性」两轴：存在性=有图；可用性=同数据对齐 + 三视口齐备。投资/成员均应为「存在有图、可用性未达成」，而不是一个有、一个缺。
- **impact**：逐格判定不可复算，第三方无法复核；也掩盖了「成员数据未对齐」这个已知缺口。
- **suggestion**：矩阵每格写成「图路径 / 数据前置 / 对齐状态 / 视口」四段；P4/P3 的裁决结果只影响「可用性」，不影响「存在性」。
- **status**：待 WB 处理

### F-05 · P1 · `pairs` 证据代码基准已漂移，README 声明失实

- **location**：`docs/r0-screenshots/pairs/README.md:13`；提交 `0472484`；`app.js:560-571`、`:1706-1741`
- **quote**：「`server/` 自该提交起零改动」
- **problem**：实测 `bff6a2b..fb5ee1a` 对 `server/web/dist/app.js` 改动 **+43/−9**。其中 `openTxForm` 修了分类按类型过滤（此前会把收入类「工资」列进支出下拉，pairs §5.1 记录过该缺陷）；`renderMembers` 改了邀请按钮的角色门控。pairs 与 43 张均采于修复前。
- **reproduce**：`git diff --stat bff6a2b..HEAD -- server/`；`git log bff6a2b..HEAD -- server/web/dist/app.js`
- **actual**：矩阵把 pairs/43 图当作当前实现 normal 证据，未标基准
- **expected**：每张 normal 证据标「采集代码基准」；对 0472484 影响的两页（手动记账、家庭成员）标「需复核/复采」；README 的「零改动」句修正
- **impact**：会把已修缺陷当成现状证据（例如用修复前的下拉图证明「分类串类型」仍存在），也会让「同数据同状态」的结论建立在旧代码上。
- **suggestion**：矩阵加「证据基准 SHA」；pairs README 改为「截至 bff6a2b；此后 app.js 由 0472484 改动，受影响页需复核」。
- **status**：待 WB 处理

### F-06 · P1 · conflict 整列 N/A 不成立，且 §8.4 把它写成验收标准

- **location**：`docs/r0-t22-execution-plan.md:258-269`（conflict 全 N/A）、`:231`（W8 24 格）、`:389`（验收标准 4）
- **quote**：「在矩阵对应 24 格（12 页 × 2 状态）填 `N/A + 边界理由`」；「offline/conflict 两列全为 `N/A + 边界理由`」
- **problem**：前端 `app.js:655` 发送 `baseVersion: tx.syncVersion`；后端 `tx_update.go:131-135` 校验并返回 `ErrConcurrentConflict`；`common.go:64` 映射 409；前端 `:670` 弹 toast 且追加「（请刷新列表后重试）」。PRD `:469`、落地方案 `:366/:928` 已定「覆盖 / 放弃 / 另存副本」三选一。故 conflict 有运行时载体，缺的是三选一 UI 与截图，不是 N/A。
- **reproduce**：`sed -n '645,671p' server/web/dist/app.js`；`sed -n '128,136p' server/internal/service/tx_update.go`；`sed -n '63,65p' server/internal/api/common.go`
- **actual**：conflict 全列 N/A；§8.4 要求「全为 N/A」
- **expected**：交易详情/账户/分类记「缺（有载体，UI 未实现）」；预算/成员/只读页 N/A + 逐页理由；§8.4 改为「除有 409 载体的页外」
- **impact**：这是最容易被「N/A 冲抵门禁」的一条：一个已定设计、已有 409 链路的状态，被 N/A 后就不再需要证据；后续复核会误以为 Web 完全没有冲突概念，或反过来认为 N/A 等于「无需实现」。
- **suggestion**：先按 §4.3 拆页判定；把「409 有提示、无三选一」登记为已知缺口（编号由 Owner 决定，本报告不擅自新增 R0-E 编号）。
- **status**：待 WB 处理

### F-07 · P2 · N/A 无逐格理由，offline 未挂已知缺口

- **location**：`docs/r0-t22-execution-plan.md:257-269`；`收尾清单:109`
- **quote**：矩阵内仅写 `N/A`，无任何理由文字；门禁要求「对不适用的状态写 N/A，并给出理由」
- **problem**：37 个 N/A 格里只有页 3 有 PRD 依据，其余全部裸 `N/A`。offline 的 N/A 也没有说明「无离线能力」并挂 R0-E-15/25。
- **reproduce**：`grep '^| [0-9]' plan | grep -c 'N/A'`；逐格看无理由
- **expected**：每格 `N/A + 理由`（可引用统一理由编号 R1/R2…，但必须能追到代码/PRD 行）
- **impact**：N/A 变成省略号，无法区分「不适用」与「没做」。
- **suggestion**：建立 N/A 理由表（R1 无列表 / R2 静态页 / R3 Web 无离线能力 / R4 无版本写 / R5 页 3 范围外），每格填编号 + 一句话。
- **status**：待 WB 处理

### F-08 · P2 · 24 格与 13 行矩阵的 26 格对不上

- **location**：`docs/r0-t22-execution-plan.md:231`、`:256-269`
- **quote**：「24 格（12 页 × 2 状态）」
- **problem**：矩阵 13 行 × offline/conflict 2 列 = 26 格；页 3 的两格没有归属说明。
- **expected**：写明「26 = 24（12 实采页）+ 2（页 3，随全行 N/A）」
- **impact**：复核时无法判断是漏填 2 格还是口径差。
- **suggestion**：在 W8 验收里直接写 26 的构成。
- **status**：待 WB 处理

### F-09 · P2 · 104 / 96 双分母未冻结，计数验收命令不存在

- **location**：`docs/r0-t22-execution-plan.md:271`（104=有13+缺54+N/A37）、`:387`（12 页三档）、`:96`（§1.4 ④ 注释掉的 grep）、`:142`（W1 验收「缺格数与 §3 预填一致（命令见 §1.4）」）
- **quote**：「三数之和 = 104」；「缺 格数与 §3 预填一致（命令见 §1.4）」
- **problem**：104 含页 3 的 8 个 N/A；而 §8.2 的覆盖要求是 12 页。三计数之和 = 104 是恒等式，不能证明覆盖。且 §1.4 ④ 只有一条被注释的 `grep -c`（数行数），没有「比对缺格数」的命令。
- **expected**：同时声明「清点分母 104（13×8）」与「覆盖率分母 96（12×8）」；给出真正能比对计数的命令（见 §7 V-09）
- **impact**：用 104 做分母会把 37 个 N/A 稀释成覆盖率，出现「覆盖 63%」的假进度。
- **suggestion**：汇总表分三行：存在图 / 三视口齐备 / N/A，分母分别写 104 与 96。
- **status**：待 WB 处理

### F-10 · P2 · 页 13 一行合并 mine 与 settings，8 格判据未定义

- **location**：`docs/r0-t22-execution-plan.md:269`；`pairs/README.md:94`
- **quote**：「有（4 档 / 2 档，缺 641）」；pairs「实际是同一个 `/profile`……统计页数时应按 12 个独立原型路由计」
- **problem**：一个矩阵行对应两个实现视图（`renderMine:2172` + `renderSettings:1549`），normal 格写「4 档 / 2 档」，无法回答「641 档时两视图是否都要有图」「一视图有、一视图无算不算有」。
- **expected**：拆分 13a/13b 两子行，或明文规定「两视图均需覆盖才算有」
- **impact**：三视口齐备无法逐格判定；页 13 会成为「看起来有、实际只覆盖一半」的灰区。
- **suggestion**：建议拆子行（不增加总格数：13a/13b 各 8 格会变 112，需 Owner 定；若维持一行，必须写合并判据）。
- **status**：待 Owner/WB 冻结

### F-11 · P2 · 导出 empty 判「缺」缺少依据

- **location**：`docs/r0-t22-execution-plan.md:267`、`:415`（§10.4 自认主观）；`app.js:1052`、`:1066`
- **quote**：「如『导出页 empty 态』我判为 `缺`……若 Owner 认为该格应 N/A，需在 W1 冻结时改判」
- **problem**：导出页是配置表单，无列表；空数据反馈发生在点「预览」后的文本「没有可导出的数据（该范围内无交易）。」（`:1066`，已实现）。采一张「empty」图与 normal 首屏无差异。
- **expected**：判 N/A + 理由（无列表首屏空态；空数据为动作后文本），或给出可区分判据（点预览后截图 + 断言文本）
- **impact**：会产生一张无信息量的图充数，违反「截图不是堆数量」（`收尾清单:109`）。
- **suggestion**：改 N/A + 理由；若要验证空数据提示，归入 W6 success/交互态的动作后证据。
- **status**：待 Owner/WB 冻结

### F-12 · P2 · 手动记账 empty N/A 与代码不一致

- **location**：`docs/r0-t22-execution-plan.md:260`；`app.js:551-552`、`:568`
- **quote**：「N/A（弹层无空态）」
- **problem**：无分类时 `fillSelect(...,"（暂无可选分类）")` 明确渲染空提示；无账户时下拉为空且无引导。是否算「empty 态」取决于「弹层是否算独立页面」，计划未定义。
- **expected**：二选一并写明：① N/A +「弹层非独立页面，且无页面级空态」；② 判「缺」并登记「无账户/无分类引导缺失」（编号由 Owner 定）
- **impact**：若按 N/A，会漏掉一个真实的空前置缺陷；若按缺，又无验收判据。
- **suggestion**：优先 ②，因代码已有分类空提示、缺账户提示，属可观察差异。
- **status**：待 Owner/WB 冻结

### F-13 · P3 · W8 grep 证据范围不全、未记 rc、sync 字面会误导

- **location**：`docs/r0-t22-execution-plan.md:308-316`、`:233`；`app.js:556/572/578`
- **quote**：「`grep -c -e "WebSocket" -e "ws://" -e "wss://" server/web/dist/app.js` → 0」
- **problem**：只查 `app.js`；未查 `index.html`（`<script src="/app.js">`，无内联脚本，结果同为 0）；`grep -c` 无命中时 **rc=1**，证据里只写「0」会被误读为命令成功；排除 `async` 后仍有 4 处 sync 字面（`syncFormFields` ×3 于 `:556/:572/:578`、`tx.syncVersion` 于 `:655`），只看「sync」会误判「前端有同步」；反过来这 4 处也正好证明**没有任何 sync 队列/拉取调用**。
- **expected**：命令附文件范围（app.js + index.html）、rc、排除说明（"仅查 `sync/pull` / `sync/push`，UI 函数名 `syncFormFields` 不计"）
- **impact**：复核方按「sync」复查会得到相反结论，形成无谓往返。
- **suggestion**：证据块写成「命令 / 文件 / 输出 / rc / 排除项」五段。
- **status**：待 WB 处理

### F-14 · P2 · 「有」格没有可追溯路径清单

- **location**：`docs/r0-t22-execution-plan.md:255-269`；`收尾清单:109`
- **quote**：「有（4 档，缺 641）」
- **problem**：门禁要求每张截图可追溯页面/视口/状态/数据前置/动作/预期；`有（4 档）` 无法定位具体文件，也没有数据前置与动作说明。
- **expected**：每格列路径（可用通配但必须可展开）或指向逐张清单；附数据前置（空库/22 笔种子/隔离实例）
- **impact**：第三方无法抽验；「同数据」结论无法核实。
- **suggestion**：矩阵每格末附 `证据清单` 附录编号（E1…En），附录逐张列路径 + 基准 SHA + 数据前置。
- **status**：待 WB 处理

---

## 6. 四分类账（明确区分结论强度）

### 6.1 已由代码证据确认（可复现，可直接作为审计结论）

| # | 事实 | 证据 |
|---|---|---|
| C1 | 13 页与 render 函数一一对应 | `app.js:161/1963/548/2008/678/2103/1277/1139/835/964/1667/2172/1549` |
| C2 | 页面级骨架屏仅首页账单流 + 交易总表 | `app.js:1908-1909`、`:424` |
| C3 | 页级错误框仅交易总表 | `app.js:413` |
| C4 | 空态实现分布（账户/分类/报表/预算/投资/账单流/账户详情） | `app.js:736/2131/909/1164/1324/1385/1917/1837` |
| C5 | 登录有按钮 pending | `app.js:207` |
| C6 | conflict 链路：baseVersion → 409 → toast 指引 | `app.js:655/670`；`tx_update.go:131-135`；`common.go:64` |
| C7 | 账户/分类并发 409 | `account.go:239`；`category.go:234` |
| C8 | 预算无 `sync_version` | `model/budget.go:22` |
| C9 | 前端 0 WS / 0 sync 调用 / 0 离线能力 | §2.5 全部 grep |
| C10 | 后端 WS + sync 路由存在 | `router.go:133/234-235`；`broadcast.go:99/122` |
| C11 | pairs 三张「normal」图确实有数据 | 实际打开 `home/budget/report__impl__390x844.png` |
| C12 | pairs 基准后代码有改动 | `git diff --stat bff6a2b..HEAD -- server/` = 52 行 |

### 6.2 仅由文档声称（未获代码/图证支持，或已被证伪）

| # | 声称 | 出处 | 审计判定 |
|---|---|---|---|
| D1 | 「`server/` 自 `bff6a2b` 起零改动」 | `pairs/README:13` | **被证伪**（+43/−9） |
| D2 | 「runtime/01 是首页隔离运行时证据，非全页」 | 计划 `:177` | **被证伪**（整页截图） |
| D3 | 「pairs 104 张可复用为 normal 基线」 | 计划 `:388` | **未验证**：至少 2 页受 0472484 影响，需逐页复核 |
| D4 | 「前端 0 处 sync」 | 计划 `:318` | **表述不精确**：0 处 `sync/pull` 与 `sync/push`，但全文（排除 async）有 4 处 sync 字面，须逐一排除 |
| D5 | 「W1 缺格数可与 §3 预填比对（命令见 §1.4）」 | 计划 `:142` | **命令不存在**（§1.4 ④ 只有被注释的行数 grep） |
| D6 | 「页面 13 的 empty 为 N/A（设置页无空态）」 | 计划 `:269` | **仅覆盖 settings 侧**；mine 侧未论证（mine 无列表，结论可能仍成立，但缺推导） |

### 6.3 尚缺证据（必须采集或复跑，不能以文档代替）

| # | 缺口 | 数量/范围 | 现状 |
|---|---|---|---|
| E1 | 641×800 三档基线 | 12 页 × 适用状态 | **0 张**（`find` 641 = 0） |
| E2 | loading（除首页） | 11 页 | 0 张；仅按钮 pending 载体 |
| E3 | error（除首页） | 11 页 | 0 张；仅 1 张旧泛化首页 error + 2 张隔离运行时 |
| E4 | success 动作后 | 12 页 | 0 张（`截图README:189`） |
| E5 | permission 多角色 | 12 页 × 4 角色 | 0 张，仅 owner |
| E6 | conflict 页内证据 | 交易详情/账户/分类 | 0 张，UI 三选一未实现 |
| E7 | 三视口齐备判定 | 全矩阵 | 尚无任何一格 390/641/1280 齐备 |
| E8 | OBS `closed=true` | `docs/r0-evidence-7th-fix/p2/visibility-observation-independent.json`（`status=FAIL`、`closed=false`、`visibleInnerText=false`） | 未闭环，依赖 P6/T11 |

### 6.4 N/A 及其理由（建议冻结版）

| 范围 | N/A 建议理由 | 依据 |
|---|---|---|
| 页 3 全列 | Web 不做 OCR，页面无路由 | `PRD:326-333` |
| offline（12 实采页） | Web 无离线模式、无离线队列、无 `navigator.onLine` 订阅 | `app.js` grep 0；挂 R0-E-15/25 |
| conflict（预算） | 预算表无 `sync_version`，不参与增量同步 | `model/budget.go:22` |
| conflict（成员） | 409 属容量/生命周期语义，非版本三选一 | `common.go:116`；`member.go:283` |
| conflict（只读页 1/2/10/11/13） | 无版本写操作 | `render*` 只读 |
| 页 1 empty / permission | 登录页无列表 / 未登录无角色 | `renderLogin:161` |
| 页 5 empty / 页 13 empty / 页 12 empty | 详情必有数据 / 静态页无空态 / 成员至少 1 人 | 代码路径 |
| 页 4 empty、页 11 empty | **待 Owner 冻结**（见 F-11/F-12） | — |

> 上表是**建议版分页归属**，不作计数：最终 N/A 数见 §3.5（暂为 34，已把交易详情/账户/分类 3 格移出 N/A）；「页 4 empty」「页 11 empty」仍待冻结，若改判只在这 5 个待定格内移动，不影响「合计 104」。

> 以上均为审计建议，不是 WB 交付，也不是 Owner 裁决；`有/缺/N/A` 的最终值以 W1 冻结并经复核为准。

---

## 7. 修正要求与可验证验收标准

> 每条验收命令都在仓库根执行、只读、可复现。**不得**把命令改成「看起来能过」的版本；阈值不放宽。

| ID | 修正要求 | 验收标准（命令/断言） |
|---|---|---|
| F-01 | 首页/预算/报表 normal 改列 pairs 路径，删「有数据态未采」 | `for f in home budget report; do test -f "docs/r0-screenshots/pairs/impl/${f}__impl__390x844.png" && echo "OK $f"; done`（输出 3 行 OK）且矩阵三格含上述路径 |
| F-02 | 首页 loading 改「有（2 档，隔离运行时）」并列 runtime 01/04 | 矩阵含 `runtime/01-pending-390x844.png`、`runtime/04-pending-1280x800.png`，且不含「非全页」表述 |
| F-03 | 每格单值；视口完整度独立列 | 解析矩阵：含「有」又含「缺」的单元格数 = 0；且存在 390/641/1280 三列 ✓/✗ |
| F-04 | 冻结「存在性 / 可用性」两轴，投资与成员同口径 | 两格均含「图路径」+「数据对齐状态」；不得一个缺一个有而无判据说明 |
| F-05 | 标「证据基准 SHA」；受影响两页标「需复核/复采」 | 每格含 `bff6a2b` 或 `fb5ee1a` 等 SHA；`pairs/README:13` 不再出现「零改动」 |
| F-06 | conflict 不整列 N/A；按 §4.3 拆页 | 矩阵 conflict 列中 `N/A` 格 ≤ 9，且交易详情/账户/分类三格非 N/A 并注明 409 载体行号 |
| F-07 | 每格 N/A 附理由（可用 R1–R5 编号） | `N/A` 格无理由数 = 0；offline 格含「R0-E-15/25」 |
| F-08 | 写明 26 = 24 + 2 | 文档含「26」且含「页 3」归属说明 |
| F-09 | 双分母 + 真计数命令 | 文档同时出现 104 与 96；附录含可执行的逐格计数脚本（见 V-09） |
| F-10 | 页 13 拆子行或写合并判据 | 文档含 `13a`/`13b` 或明文「两视图均需覆盖」 |
| F-11 | 导出 empty 改 N/A + 理由，或给动作后判据 | 该格含「无列表」或「点预览后截图」二者之一 |
| F-12 | 手动记账 empty 明确取舍并留痕 | 该格含 `app.js:568` 依据或明确 N/A 理由 |
| F-13 | grep 证据五段化（命令/文件/输出/rc/排除） | W8 段含 `index.html`、`rc`、`syncFormFields` 排除说明 |
| F-14 | 每格列路径清单或附录编号 | 抽 3 格：路径 `test -f` 全部存在；附录含基准 SHA 与数据前置 |

**V-09 逐格计数验收脚本（只读，建议写入 W1 文档附录）**：

```bash
# 解析矩阵：13 行 × 8 状态列；统计三值；校验无混值
awk -F'|' '
/^\| *[0-9]+ *\|/ {
  n++; if (NF < 11 || NF > 12) { print "列数异常:" NR ": " NF; bad++ }
  for (i=4; i<=11; i++) {
    v=$i; gsub(/^ +| +$/, "", v)
    if (v ~ /^有/ && v ~ /缺/) { print "混值:" NR; bad++ }
    if (v ~ /^有/) y++; else if (v ~ /^缺/) q++; else if (v ~ /^N\/A/) na++; else { print "非法值:" NR; bad++ }
  }
}
END { printf "行=%d 有=%d 缺=%d N/A=%d 合计=%d 异常=%d\n", n, y, q, na, y+q+na, bad+0;
      exit (bad>0) }
' docs/r0-status-matrix-13x8.md
```

期望输出：`行=13 有=17 缺=53 N/A=34 合计=104 异常=0`（若 5 个待定格改判，允许计数变化，但**合计必须仍为 104、异常必须为 0**）。

---

## 8. 本轮边界与未做事项

- 未修改 `docs/r0-t22-execution-plan.md`、T21 台账、任何业务代码、CSS、fixture、`controls.json`、runner、CI、统计。
- 未启动服务、未采截图、未造数、未写库。
- 未执行 W2–W7、W9，未处理 P1–P6。
- 未改 main、未合并 PR、未改仓库可见性、未重写 Git 历史。
- 未宣布 T22 完成、第 3 条 PASS 或 R0 PASS。

**变更文件清单**：仅新增 `docs/r0-t22-b1-independent-audit.md`（本文件）。

---

## 9. 结论

1. W1 的**骨架思路成立**（13 行 × 8 列、三值、页 3 有理由 N/A），但**分类与口径不成立**：4 格误记「缺」；13 个「有」格中 11 格混值（同时写「有」与「缺 641」）、另 2 格未声明三档；判据不一致（成员有/投资缺）；pairs 证据基准过期未标。
2. W8 的**边界方向正确**（Web 未接 WS/sync 客户端），但**把 conflict 一并 N/A 是实质误判**，且 §8.4 会把这个误判固化为验收标准。
3. 建议 WB 先修 F-01…F-06（P1）再冻结矩阵；F-07…F-14 在冻结同一轮修完。任何一条未闭环，矩阵不应进入「填满 → 汇总 → 交复核」的批次 5。
4. **本报告是独立审计，不代表 T22 已完成，也不代表 R0 可以 PASS。** 发现的实际实现缺口（conflict 三选一缺失、pairs 证据基准过期）只记录证据与修正要求，等待后续明确授权再处理。
