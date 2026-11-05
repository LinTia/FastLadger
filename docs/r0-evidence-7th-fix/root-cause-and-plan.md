# 第七次修复 · 根因分析与最小修改方案（以复现证据为准）

- 分支：`fix/r0-sixth-runtime-defects`；commit `dadc9a3b9c567cad2bbad7e47361fb774365b949`（= 基准 main，工作区干净）
- 服务二进制：本次重新构建的 `server/fastledger.exe`（构建时间 2026-11-05 17:13，`go build -o fastledger.exe ./cmd/server`）
- 基线证据目录：`baseline/`（C 套件完整日志/报告、账户 3 次复现、修复前 DOM 观测）
- 复核方式：逐行读 `server/web/dist/app.js` + 真实浏览器 Playwright(msedge 156) 采集，不照抄既有结论

## 证据总览（修复前）

| 编号 | 观测事实 | 证据 |
|---|---|---|
| 1 | 交易总表 `#view.childElementCount=0`，`innerText`/`textContent` 均空；`GET /books/1/transactions?page=1&count=50` = 200 | `baseline/c-before.log`（I-TXNS-01 FAIL）、`baseline/pre-fix-observations.md` §1、`baseline/baseline-probe.json` |
| 2 | 支出/收入/转账/切回支出时 `#fCategory` 都是同一份 30 项混合下拉，首项 `工资`(23, catType=1)，默认 value=23；API 分类中 catType=1 有 8 项、catType=2 有 22 项 | `baseline/pre-fix-observations.md` §2 |
| 3 | member 身份成员页「邀请成员」按钮数=1；owner/admin 各 1；member 直接 POST invite = 403 | `baseline/pre-fix-observations.md` §3 |
| 4 | 3/3 次：POST=200 且回读含新账户名；`#view` 确实重绘（tbody 行数 3→4→5→6）；新名称在 `textContent` 中但 `innerText` 中不存在；首列 `<td>` 的 `display:none`；刷新后仍不可见 | `baseline/account-repro.md`、`baseline/baseline-probe.json` |

---

## 缺陷 1 · 交易总表整页空白

**文件/函数/行号**：`server/web/dist/app.js` → `renderTxns(bookId)` L337–527；`view.replaceChildren()` L345；构造 `card` L347、筛选行 L353–366、批量条 L369–376、表格容器 L383–385；`try { await load(); } catch(e){ toast(...) }` L526。**函数内没有任何 `view.append(card)`**（对照 `renderHome` L230、`renderBooks` L302、`renderAccounts` L711、`renderMembers` L1764 均有 append）。

**根因（以证据为准）**：`card` 及其内部所有控件（筛选行、批量条、`tableWrap` 骨架屏/表格）都挂在游离的 `card` 上，`card` 从未插入 `#view`。`load()` 正常返回 200 并把表格渲染进游离 `tableWrap`，因此浏览器 `#view` 恒为 0 子元素、空文本，网络层却一切正常——这解释了「无 console error 但整页空白」。C 套件 I-TXNS-01 FAIL、02–19 共 18 条被脚本硬编码为 BLOCKED 即由此而来。

**最小修改方案**：
1. 在静态 DOM 组装完成处（`card.append(tableWrap)` L385 之后）补 `view.append(card);`，使筛选行/批量条/骨架屏/表格恢复既有可见性；放在 `await load()` 之前，这样骨架屏期间页面也可见。
2. `load()` 内对 `await api("GET", ...)` 增加 try/catch：请求失败时清除骨架屏（`aria-busy` 不应永久残留），在 `tableWrap` 内以既有 `.empty` 样式 + `textContent` 呈现一条静态「交易加载失败」文本；异常继续抛给调用方，由既有 `catch` 负责 toast。**不新增按钮、不新增交互、不新增产品功能**，属于恢复已在 `controls.json` 登记的 I-TXNS-19「加载失败」控件；若 team-lead 判定这属于需先确认的「新增错误态」，可回退为仅清除骨架屏。
3. 其余筛选/查询/批量删除/分页/编辑逻辑一律不动。

**影响面**：仅 `renderTxns` 一个函数的 DOM 挂载与失败分支；交易创建/编辑/删除的业务语义、请求参数、API 调用完全不变。

**是否触及 PRD / API / DB**：否（三者均不变）。

**回归用例设计**：
- C 套件 `groupTxns` 逐条真实执行 I-TXNS-01..19（可见性 + 真实交互 + 每条独立截图/判定）。
- D 套件：有数据列表行数与备注文本；空账本空态；`page.route` 注入 500 后断言页面内可见失败反馈且 `[aria-busy=true]` 已消失；行「编辑」打开 `#editDlg`；骨架屏延迟观测。

---

## 缺陷 2 · 「记一笔」分类下拉未按交易类型过滤

**文件/函数/行号**：`server/web/dist/app.js` → `openTxForm(bookId,onDone)` L530–580；L535 `fillSelect($("#fCategory"), state.categories)` 一次性塞入全部分类；`syncFormFields()` L542–546 只 toggle `#fToRow`/`#fCatRow` 显隐；L547 `$("#fType").onchange = syncFormFields` 不重建分类下拉。对照 `openTxEditForm` L602 已用 `state.categories.filter(c => c.catType === tx.txType)` 正确过滤。

**根因（以证据为准）**：支出默认类型（L532 `fType="2"`）下，下拉首项是收入类 `工资`(id=23,catType=1) 且被默认选中；`fillSelect` 不带 blankLabel 时浏览器自动选中第一个 option，因此用户直接填写金额保存会提交 `txType=2, categoryId=23`，服务端校验类型不一致返回 400。切换类型时 `syncFormFields` 只改显隐、不重建下拉，已选值仍是收入分类，造成「选中收入分类却以支出提交」；若切到转账再切回，也仍是旧值。

**最小修改方案**（与 L602 编辑表单口径一致）：
1. 抽出 `refillCategories()`：`const t = Number($("#fType").value); if (t === 3) return; const cats = state.categories.filter(c => c.catType === t); fillSelect($("#fCategory"), cats); ...`，类型切换时重建并重置为该类型首个分类；匹配集为空时给出 value=0 的占位项，避免空下拉。
2. `syncFormFields()` 在显隐切换后调用 `refillCategories()`；打开表单时同样经 `syncFormFields()` 初始化，删除 L535 的无差别全量填充。
3. 提交分支沿用编辑表单 L629 的既有口径，把 `else if (!body.accountId)` 收紧为 `!body.accountId || !body.categoryId` 并 toast「请选择账户与分类」，避免 `categoryId=0` 打到服务端产生 400。
4. **转账时分类行隐藏、提交 `categoryId:0` 的既有语义不变**：`refillCategories()` 在 `t===3` 直接返回，提交体 L559 `t === 3 ? 0 : Number($("#fCategory").value)` 原样保留。

**影响面**：仅 `openTxForm` 的下拉初始化/类型切换与提交前校验；不改交易字段、不改 API 请求结构、不改转账语义。全局 `fillSelect`、`state.categories` 不变。

**是否触及 PRD / API / DB**：否。

**回归用例设计**（D 套件）：
- 支出下拉 option 集合 == API 中 catType=2 的分类 id 集合（不含任何收入 id）；
- 收入下拉 option 集合 == catType=1 集合；
- 切换类型后 `#fCategory.value` 正确重置（收入/支出互切、转账互切）；
- 支出类型不手动改分类直接提交 → POST 2xx，API 回读 `txType=2` 且 `categoryId` 属于 catType=2 集合；刷新页面后仍存在；
- 转账提交 → 请求体 `categoryId===0`、`toAccountId` 存在，API 回读 `txType=3`。

---

## 缺陷 3 · member 仍能看到「邀请成员」入口

**文件/函数/行号**：`server/web/dist/app.js` → `renderMembers(bookId)` L1636–1765；L1647 无条件创建 `invBtn`，L1678 `row.append(h, invBtn)` 无条件挂载；成员 DTO 与 `/users/me` 到 L1683–1684 才取回（晚于按钮挂载）。

**根因（以证据为准）**：前端在拿到当前用户身份之前就把入口画出来，且从未按角色做判断；owner/admin/member 三种身份下按钮数均为 1。后端 service 层已对 `POST /books/:id/members/invite` 做权限校验（member 直接调用返回 403，实测确认），属于「服务端安全、UI affordance 越权」——A08 已把该现象记录为缺陷但未修。

**最小修改方案**：
1. `row.append(h, invBtn)` 改为 `row.append(h)`；在 L1683–1684 取回 `members` 与 `me` 之后，按 `members.find(m => m.uid === me.id).role` 判定：`role===1`（所有者）或 `role===2`（管理员）才 `row.append(invBtn)`；`role===3`（普通成员）不渲染入口，DOM 中也不应出现「邀请成员」文本。
2. `/users/me` 请求失败时**失败关闭**：`await api("GET","/users/me").catch(() => null)`，`me` 为 null 则不渲染按钮；同时把 L1710 `const myUid = me.id` 改为 `me ? me.id : null`，避免失败时页面整体抛错。
3. **后端 403 校验原样保留**，不新增/不修改任何 API 契约，不新增接口调用（`/users/me` 与 `/members` 都是既有调用）。

**影响面**：仅 `renderMembers` 的按钮挂载时机与条件、以及 `myUid` 的空值防御；成员列表、角色徽标、可见性徽标、升降级/移除按钮全部不动。

**是否触及 PRD / API / DB**：否。

**回归用例设计**（D 套件）：owner / admin 按钮 count=1 且点击能打开邀请弹窗；member 按钮 count=0 且 `#view` 文本无「邀请成员」；member 直接 POST invite 仍 403；成员列表 3 行、角色徽标与 API 回读仍正确；`/users/me` 返回 500 时 owner 也不显示按钮（失败关闭）。

---

## 缺陷 4 · 新增账户「API 有、页面 6s 内不出现」——**结论：测试判定口径问题，不改业务代码**

**文件/函数/行号**：
- 业务代码：`server/web/dist/app.js` → `openAcctForm` L715–748（L745 已有 `await renderAccounts(bookId)`），`renderAccounts` L647–712（L649 `await loadBookMeta(bookId)` 每次重新 GET accounts/categories、L653 `view.replaceChildren()`、L668 遍历 `state.accounts`、L711 `view.append(card)`）；`loadBookMeta` L306–314 无缓存。
- 测试代码：`tools/r0/e2e/specs/c-controls.js` → `groupAccounts` L318–328，用 `waitText(page, name, 6000)`（实现见 L46–48：`document.body.innerText.includes(n)`）判定「页面可见」。
- 样式：`server/web/dist/style.css` L327–333 `@media (max-width: 640px) { th:nth-child(1), td:nth-child(1), th:nth-child(5), td:nth-child(5) { display: none; } }`（L331）。

**根因（以 3 次稳定复现为准，见 `baseline/account-repro.md`）**：
1. 三次 POST 均 HTTP 200，`GET /accounts` 回读均含新账户名，账户总数 3→4→5→6；`#acctErr` 为空、toast 为「✓ 账户已创建」、无 console error/pageerror → 后端与业务 JS 都成功。
2. `#view` 的 MutationObserver 明确记录到 POST 后**发生一次重绘**，tbody 行数 3→4→5→6，新账户名已写入 `view.textContent`（`viewTextContentHasName=true`），对应 `<td>` 存在 → **`renderAccounts` 确实被重新执行且数据已渲染**，排除「列表未刷新/缓存」。
3. 但该 `<td>` 的 `getComputedStyle().display === 'none'`，`viewInnerTextHasName=false`、`bodyHasName=false`；刷新页面后依然如此 → 不是时序问题，是**恒定的 CSS 可见性**问题。
4. 根因链：E2E viewport 为 390×844（<640px），触发 `style.css` L331 的窄屏响应式规则；该规则用**无表格限定的全局选择器** `td:nth-child(1)`，本意应是隐藏交易总表的复选框列/分类列，却连带把账户表格第 1 列「账户名称」也隐藏了。观测到所有账户行（含既有现金/储蓄卡/信用卡）的名称列都是 `display:none`，只有类型/余额/操作列可见。测试用 `document.body.innerText` 判定，被隐藏列的文本不会进入 `innerText`，于是 `waitText` 恒为 false → I-ACCT-06 FAIL。
5. 三次归类均为「**DOM 有但不可见/文本被隐藏**」，不是「API 有、DOM 无」。这也解释了为什么 `I-ACCT-01`（`/现金|储蓄|信用卡/`）能 PASS——匹配到的是**类型列**文本，而非名称列；以及 `I-ACCT-10`「删除后从列表消失」是恒真的假阳性。

**最小修改方案（按任务授权 + team-lead 收紧裁决）**：
1. **`server/web/dist/app.js` 一行都不改**（第 4 项零业务代码改动）；**`server/web/dist/style.css` 一律不改**（L331 属未获 Owner 确认的边界项，只能登记）。
2. **不得把 I-ACCT-06 从 FAIL 改判为 PASS**：把 `waitText(body.innerText)` 换成 `td.textContent` 只增强了证据，却降低了「可见性」语义，属于放宽断言。`tools/r0/e2e/specs/c-controls.js` 的 `R['I-ACCT-06']` 保持**无条件 FAIL**，detail 同时如实写出：
   - 正向观测：`POST 2xx / tbody 行数 +1 / 首列 td.textContent 命中 / API 回读命中 / 无重复`；
   - 可见性原始观测：`visibleInnerText`（`#view.innerText.includes(name)`）与 `firstTdDisplay`（`getComputedStyle(td).display`），并落盘 `C/I-ACCT/visibility-observations.json`；
   - 结论「390px 视口下账户名称列被 style.css L331 隐藏，视觉可见性未闭环，待 Owner 裁决；本轮不改 CSS」。
3. 新增 `tools/r0/e2e/specs/d-defects.js` 的「新增账户」组可独立 PASS，但**只断言 DOM 渲染一致性**：POST 2xx + tbody 行数 +1 + 首列 `td.textContent` === 唯一名称 + API 回读集合一致 + 刷新后仍存在 + 无重复；用例名与 detail 明确标注「**DOM 渲染一致性，不含视觉可见性**」，每条保留 `visibleInnerText` / `firstTdDisplay` 原始观测（落盘 `D/account/visibility-observations.json`），且不引用 I-ACCT-06 的 PASS。
4. **Owner 验收项「新增账户立即显示」如实标为未闭环/待裁决**（见下节），不写成通过。

**影响面**：仅 E2E 测试代码；业务代码、样式、API、DB 全部不动。

**是否触及 PRD / API / DB**：否。

**回归用例设计**（D 套件）：提交后数据进入列表（POST 响应 + tbody 行数 +1 + `td.textContent`）、刷新后仍存在、账户总数只 +1（无重复）、API 名称集合与 DOM 首列文本集合完全一致；四条均标注「DOM 渲染一致性，不含视觉可见性」。

**Owner 验收项「新增账户立即显示」：未闭环 / 待裁决**

- 现状：数据链路（POST → DB → `renderAccounts` 重绘 → DOM `textContent`）完全正常；但 390px 视口下 `style.css` L331 隐藏账户名称列，真实用户看不到新账户名，`#view.innerText` 与 `document.body.innerText` 均不含该名称。因此「立即显示」这一**视觉**验收项**未通过**，本轮不写成 PASS。
- 可选方案（**不替 Owner 选择、不实施**）：
  - **方案 A**：把 `style.css` L331 的窄屏选择器限定到交易总表（例如只作用于交易表的 `td:nth-child(1)`/`td:nth-child(5)`，或给交易表单独 class），使账户表名称列在窄屏保持可见；需评估对交易总表/其它表格布局的连带影响，并走样式变更授权。
  - **方案 B**：维持现状（数据链路正确、CSS 不动），仅把该视觉缺陷登记为待裁决项，等待 Owner 决定是否纳入后续范围。
- 在 Owner 裁决前，`I-ACCT-06` 保持 FAIL，D 套件的 PASS 仅代表 DOM/API 数据一致性。

---

## 新发现但本次**未修改**的问题（只登记）

1. **（第 5 项「候选之一」，未获 Owner 确认，只登记不选定/不实施）** 目前第 5 项有两个候选，本轮均不修改、不替 Owner 选定：
   - **候选 1（本次复现所得）**：`server/web/dist/style.css` L331 全局 `td:nth-child(1)`/`td:nth-child(5)` 在 ≤640px 隐藏了账户表格的账户名称列；该规则未限定表格，疑似为交易总表复选框列设计却误伤其它表格，移动端用户看不到账户名。不在允许修改范围，未改。
   - **候选 2（team-lead 已向 Owner 报告）**：`server/web/dist/app.js` 成员页 L1723–1758 的「设为管理员 / 降级为成员 / 移除成员」按钮未按操作者角色隐藏，普通成员可能看到无权限的操作入口（后端仍有 403 兜底）。不在本次 4 项授权范围，未改。
2. **既有测试编号漂移**：`fixtures/controls.json` 中 `I-ACCT-06` 的定义是「删除引用保护」，而 `c-controls.js` 把「创建成功并在页面渲染」的结果写在 `R['I-ACCT-06']` 上；两者语义错位，导致第六次报告里「账户 · 删除引用保护」显示为 FAIL。本次按收紧裁决让该条**保持 FAIL**（detail 补全正向与可见性观测），**不调整编号映射**（避免改变 A/C 语义），如实登记。
3. **既有测试盲点**：`c-controls.js` 的 `R['I-ADD-06b']`（分类首项判定）不在 `controls.json` 的 228 个 ID 集合内，`runControls` 只遍历控件清单 ID，因此该结果被静默丢弃，C 报告里看不到分类未过滤缺陷。本次不新增控件 ID（避免改清单），改由 D 套件显式覆盖。
4. `I-ACCT-10`「删除后从列表消失」在名称列被隐藏的前提下恒为真，属既有假阳性；本次不改该条既有判定（不在 4 项授权范围内），仅登记。

## 边界确认

- 4 项修复均不涉及 Go 后端、PRD/WBS/设计规范、数据库结构/迁移、API 契约、历史证据目录、`README.md`/`CHANGELOG.md`、既有 A/B/C 状态结论文档、任何既有规格文件。
- `.github/workflows/r0-e2e.yml`：依据 team-lead 的**补充授权**，工作区中已新增「包 D · 四项业务缺陷回归（门禁）」step（位于 A 之后、C 之前，无 `continue-on-error`，失败即 job 失败），并在「汇总退出码」step 写入 `steps.d_defects.outcome`；既有 A/B/C step 语义未变、C 的 `continue-on-error` 保留、无既有步骤被删除或覆盖。该改动由 team-lead 侧落地（文件 mtime 17:50，非本工程师修改），本工程师仅核对、未重复修改。
- 不新增产品范围/业务规则；不隐藏错误、不放宽断言、不跳过测试；不使用 sleep/延迟刷新制造 PASS；所有 DOM 文本仍用 `textContent`，不引入 `innerHTML`。
- 缺陷 1 的静态失败文本已获 team-lead 批准，严格限定：只用既有 `.empty` 样式 + `textContent`，不新增按钮/交互/重试逻辑，异常继续抛给既有 `catch` 负责 toast，且 `aria-busy` 骨架屏必须清除。
- 第 5 项缺陷（两个候选）与任何其它自认为发现的问题一律不触碰、不选定、不实施。
