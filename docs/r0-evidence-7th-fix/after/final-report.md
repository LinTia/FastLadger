# FastLedger R0 第六次自动化验收缺陷修复 · 交付报告（第七次修复）

> 本报告已按 team-lead 的**收紧裁决**修订：I-ACCT-06 已回退为无条件 FAIL；Owner 验收项「新增账户立即显示」标为未闭环/待裁决；D 套件新增账户组只断言「DOM 渲染一致性，不含视觉可见性」并保留两项原始观测；第 5 项按「候选之一」只登记。

- 分支：`fix/r0-sixth-runtime-defects`（未切换、未合并、未 push、未 force-push）
- HEAD / 基准 main：`dadc9a3b9c567cad2bbad7e47361fb774365b949`
- 重新构建：`go build -o fastledger.exe ./cmd/server`（`after/build.log`，BUILD_EXIT=0，二进制 32184726 字节）
  - 注：`server/web/embed.go` 用 `//go:embed dist` 把 app.js 内嵌进二进制，故业务代码改动后必须重建二进制才生效。
- 浏览器：playwright-core / msedge 156.0.4314.58；每套件独立临时 SQLite 实例与随机端口。
- 证据根目录：`docs/r0-evidence-7th-fix/`（`baseline/` 修复前、`after/` 修复后），旧证据目录未改动。
- 4 项根因与最小修改方案在改动任何业务代码**之前**已用 SendMessage 发给 team-lead，并落盘 `root-cause-and-plan.md`。

## 1. 最终运行结果

| 套件 | 命令 | 日志 | 退出码 | 汇总 |
|---|---|---|---|---|
| A | `node run.js --suite a` | `after/a-after.log` | **0** | 11 PASS / 0 FAIL（09:43 最终连续跑，按 team-lead「A 已认可、不重做」未重跑） |
| C | `node run.js --suite c` | `after/c-after.log` | **1** | **163 PASS / 5 FAIL / 59 BLOCKED / 1 N/A**（228 条，09:55 回退后重跑） |
| D | `node run.js --suite d` | `after/d-after.log` | **0** | **39 PASS / 0 FAIL / 0 BLOCKED**（39 条，09:56 回退后重跑） |

- C 的 5 条 FAIL：`I-LOGIN-04`(R0-E-39)、`I-LOGIN-11`/`I-LOGIN-12`(R0-E-38)、`I-HOME-01`(R0-E-01) 共 4 条基线既有产品缺口，**加 `I-ACCT-06` 按收紧裁决保持的 FAIL**。日志末尾保留 `=== EXIT_CODE=1 ===`，失败明细中可见 `I-ACCT-06`。
- 基线 C（`baseline/c-before.log`，EXIT_CODE=1）**144/6/77/1 → 修复后 163/5/59/1**：PASS +19、FAIL −1、BLOCKED −18。精确对应：`I-TXNS-01` 由 FAIL 转 PASS、`I-TXNS-02..19` 共 18 条硬编码 BLOCKED 转为逐条真实判定（最终 19 条 I-TXNS 全部 PASS），共 +19 PASS / −18 BLOCKED；**`I-ACCT-06` 保持 FAIL，故 FAIL 只从 6 减到 5，没有被改判 PASS**。
- `I-TXNS-01..19` 修复后全部 PASS（见 `after/c-after-report.json`）。
- A08 观测值由修复前「邀请成员按钮数=1」变为修复后「按钮数=0」，A 的 PASS 条件（API 403）未变，A 11/11 保持 PASS。
- 负向合成验证：`after/negative-check.md` / `negative-check.json` / `negative-check.log`（EXIT_CODE=0）。把 `git show HEAD:server/web/dist/app.js`（修复前脚本，SHA256 `32d853df…`）经 `page.route` 注入浏览器（不改任何磁盘业务代码），复现 4 组「修复前必然使新测试 FAIL」的观测：`#view.childElementCount=0`；支出下拉 30 项混入 8 个收入类 id、首项「工资」且切换类型不变；member 邀请按钮数=1；账户 POST 200 且 `textContent` 含名称但 `innerText` 不含（`td:first-child` display=none）。
- 报告副本：`after/{a,c,d}-after-report.json`；原始 artifacts 在 `artifacts/r0-fix-after-{a,c,d}/`。

## 2. 收紧裁决逐条落实说明

| 裁决要求 | 落实情况 |
|---|---|
| `style.css` 一律不改，L331 只登记 | **未改**：`git status` 中无 style.css；`root-cause-and-plan.md` 与本报告均列为「第 5 项候选之一」 |
| I-ACCT-06 不得改判 PASS，保持 FAIL（或 BLOCKED 写明待裁决） | **已回退为无条件 FAIL**：`c-controls.js` 中 `R['I-ACCT-06'] = bad(...)`，无任何 `ok(...)` 分支；重跑后 C 的 FAIL=5 且该条出现在失败明细 |
| detail 含正向观测 + `visibleInnerText` + `firstTdDisplay` + 结论文字 | 已含：`POST=200 重绘+1=true DOM 首列 textContent 命中=true API 命中=true 重复数=1；visibleInnerText=false；firstTdDisplay=none；…390px 视口下账户名称列被 style.css L331 隐藏，视觉可见性未闭环，待 Owner 裁决；本轮不改 CSS` |
| 两个原始观测字段不许省略 | 除 detail 外，C 侧落盘 `C/I-ACCT/visibility-observations.json`；D 组每条 detail 均含且落盘 `D/account/visibility-observations.json`（4 条记录） |
| D 组新增账户可 PASS，但标注「DOM 渲染一致性，不含视觉可见性」、不引用 I-ACCT-06 的 PASS | 4 条 `D-ACCT-01..04` 的**用例名**与 **detail** 均带该标注；`defects-summary.json` 的 note 明确「C 套件 I-ACCT-06 保持 FAIL，未被本组改判」；D 组不引用任何 I-ACCT-06 结果 |
| 交付文档把「新增账户立即显示」标未闭环/待裁决，列方案 A/B，不实施不选择 | 见本报告 §3 缺陷 4 与 §7；`root-cause-and-plan.md` 同节；仅列方案，未改动任何 CSS |
| 第 5 项归类为「候选之一」，加入成员页角色按钮候选，两者只登记 | 见本报告 §6 第 1 条与 `root-cause-and-plan.md`：候选 1（CSS L331）、候选 2（team-lead 报告的 `app.js` L1723–1758 升降级/移除按钮未按操作者角色隐藏），均只登记、未选定、未实施 |
| 缺陷 1 静态失败文本按批准条件实施 | 已实施且严格限定：只用既有 `.empty` 样式 + `textContent`，无按钮/交互/重试逻辑，异常继续抛给既有 `catch` 负责 toast，`renderLoadError` 先 `wrap.replaceChildren()` 清除 `aria-busy` 骨架屏（I-TXNS-19 实测残留=0） |
| `.github/workflows/r0-e2e.yml` 接入 `--suite d` 门禁 + 汇总写 d outcome | 见 §5：工作区中该改动**已存在且符合要求**（由 team-lead 依其补充授权落地，文件 mtime 17:50，非本工程师修改）；本工程师核对后未重复修改，未静默略过 |

## 3. 四项缺陷逐项交付

### 缺陷 1 · 交易总表整页空白（已修复）

- 文件/函数/行号：`server/web/dist/app.js` `renderTxns()`（修复前 L337–527；修复后 L337–545）。修复前 L345 `view.replaceChildren()`、L347 构造 `card`、L526 `await load()`，但**从未 `view.append(card)`**。
- 根因：`card` 及其筛选行/批量条/表格容器始终游离于 DOM；`GET /transactions` 返回 200 也只渲染进游离节点。`baseline/pre-fix-observations.md` §1 实测 `#view.childElementCount=0`、`innerText`/`textContent` 均空、无 console error，但网络日志有 200。
- 修改（diff 见 `after/diff.txt`）：
  1. 在 `card.append(tableWrap)` 后补 `view.append(card)`（修复后 L386），放在 `await load()` 之前；与 `renderHome`/`renderBooks`/`renderAccounts` 一致。
  2. `load()` 内加 try/catch：失败时 `renderLoadError()` 先 `wrap.replaceChildren()` **清除 `aria-busy` 骨架屏**，再用既有 `.empty` 样式 + `textContent` 显示一条**静态**「交易加载失败：…（可重新查询或刷新页面）」文本，异常继续 `throw` 给调用方既有 `catch` 做 toast。**按 team-lead 批准条件：不新增按钮/交互/重试逻辑、不扩大产品范围**，属恢复已登记的 I-TXNS-19「加载失败」控件。
- 影响面：仅该函数的 DOM 挂载与失败分支；交易字段、请求参数、业务语义不变。不触及 PRD/API/DB。
- 回归结果：C 的 `I-TXNS-01..19` 逐条真实执行全部 PASS；D 的 `D-TXNS-01..04` + D 内 19 条重跑全部 PASS；`I-TXNS-19`（注入 500）观测到「交易加载失败：E2E 注入的服务器错误（可重新查询或刷新页面）」且 `[aria-busy=true]` 残留=0。

### 缺陷 2 · 「记一笔」分类下拉未按类型过滤（已修复）

- 文件/函数/行号：`openTxForm()`（修复前 L530–580；修复后 L548–611）。修复前 L535 `fillSelect($("#fCategory"), state.categories)`、L542–546 `syncFormFields()` 只切显隐、L547 `onchange` 不重建。
- 根因：支出默认类型下下拉首项是收入类「工资」(id=23, catType=1) 且默认选中，直接提交会 `txType=2 / categoryId=23` 被服务端 400；切换类型不重建下拉，已选值仍是收入类。`baseline/pre-fix-observations.md` §2 实测支出/收入/转账/切回支出 4 态都是同一份 30 项混合下拉、value 恒为 23。
- 修改：删 L535 无差别全量填充；新增 `refillCategories()`（与编辑表单 L602 同口径 `state.categories.filter(c => c.catType === t)`、重置为该类型首个分类、空集给 value=0 占位），`syncFormFields()` 调用它；`t===3` 时直接返回以保留「转账隐藏分类行 + 提交 `categoryId:0`」既有语义；提交分支对齐编辑表单 L629 收紧为 `!body.accountId || !body.categoryId` 并 toast「请选择账户与分类」，避免空下拉打到服务端产生 400。
- 影响面：仅该表单的下拉初始化/类型切换/提交前校验；不改交易字段与 API 请求结构。不触及 PRD/API/DB。
- 回归结果（D）：`D-CAT-01` 支出下拉 22 项 == API catType=2 的 22 项、混入收入类=0、首项「餐饮」；`D-CAT-02` 收入下拉 == catType=1 集合；`D-CAT-03` 类型互切正确重置且转账隐藏分类行；`D-CAT-04` POST=200、请求体 `txType=2 categoryId=1`、API 回读一致；`D-CAT-05` 刷新后仍在；`D-CAT-06` 转账 `categoryId:0` + `toAccountId` 且 API 回读 `txType=3`。全部 PASS。

### 缺陷 3 · member 仍能看到「邀请成员」入口（已修复）

- 文件/函数/行号：`renderMembers()`（修复前 L1636–1765；修复后 L1667–1796）。修复前 L1647/L1678 在 L1683 `GET /members`、L1684 `GET /users/me` 之前就无条件创建并挂载 `invBtn`。
- 根因：前端拿到身份前就画出入口且从不按角色判断；owner/admin/member 三种身份按钮数均为 1。后端 service 层已对 `POST /members/invite` 校验（member 实测 403），属「服务端安全、UI affordance 越权」。
- 修改：`row.append(h, invBtn)` → `row.append(h)`；取回 `members` 与 `me` 后按 `members.find(m => m.uid === me.id).role` 判定，`role===1||2` 才 append 按钮；`/users/me` 用 `.catch(() => null)` **失败关闭**；`myUid` 改为 `me ? me.id : null` 空值防御。不新增任何接口/契约，后端 403 校验原样保留。
- 影响面：仅按钮挂载时机与条件、`myUid` 空值防御；成员列表/角色徽标/可见性徽标/升降级/移除全部不动。不触及 PRD/API/DB。
- 回归结果（D）：owner/admin 按钮数=1 且点击打开 `#investDlg`；member 按钮数=0 且 `#view`/body 文本均无「邀请成员」；member 直接 POST invite = 403；成员列表 3 行、角色徽标与 API 一致；`/users/me` 注入 500 时 owner 也不显示按钮且成员列表仍渲染。全部 PASS。

### 缺陷 4 · 新增账户「API 有、页面不出现」——**测试判定口径问题，业务代码零改动；视觉可见性未闭环**

- 文件/函数/行号：业务侧 `openAcctForm()` L715–748（L745 已有 `await renderAccounts(bookId)`）、`renderAccounts()` L647–712、`loadBookMeta()` L306–314（均**未修改**）；测试侧 `c-controls.js` `groupAccounts`；样式 `server/web/dist/style.css` L331（**一律未改**）。
- 根因（`baseline/account-repro.md` 3/3 稳定复现）：POST 均 200、`GET /accounts` 回读均含新名、总数 3→4→5→6；`#view` MutationObserver 记录到重绘且 tbody 行数同步 +1，新名称已在 `textContent` 中 → **数据链路完全正常，排除「列表未刷新/缓存」**。但 390px 视口触发 `style.css` L331 全局 `td:nth-child(1){display:none}`，账户表第 1 列「账户名称」被隐藏，`innerText` 恒不含该文本，故旧 C 的 `waitText(body.innerText)` 恒 FAIL、刷新后依旧。归类「DOM 有但不可见」。
- 修改（**按收紧裁决**）：
  1. `c-controls.js` 的 **`I-ACCT-06` 回退为无条件 FAIL**（`bad(...)`，无任何 PASS 分支）。确定性等待（真实 POST 响应 + tbody 行数 N→N+1，移除原固定 sleep）只用于采集观测；detail 同时写正向观测、`visibleInnerText`、`firstTdDisplay` 与「视觉可见性未闭环，待 Owner 裁决 CSS」结论；原始观测落盘 `C/I-ACCT/visibility-observations.json`。**回退后 diff 片段见 `after/diff.txt` 的 c-controls.js 段**。
  2. `d-defects.js` 的 `D-ACCT-01..04` 独立 PASS，但**只断言 DOM 渲染一致性**（POST 2xx + tbody +1 + 首列 `td.textContent` === 唯一名称 + API 回读集合一致 + 刷新后仍存在 + 无重复），用例名与 detail 明确标注「**DOM 渲染一致性，不含视觉可见性**」，每条保留 `visibleInnerText` / `firstTdDisplay`，落盘 `D/account/visibility-observations.json`，且不引用 I-ACCT-06 的 PASS。
  3. `server/web/dist/app.js`、`server/web/dist/style.css` 均零改动。
- 影响面：仅 E2E 测试代码；业务代码、样式、API、DB 全部不动。不触及 PRD/API/DB。
- 回归结果：`I-ACCT-06` = **FAIL**（`POST=200 重绘+1=true DOM 首列 textContent 命中=true API 命中=true 重复数=1；visibleInnerText=false；firstTdDisplay=none`）；`D-ACCT-01..04` 全部 PASS 且标注不含视觉可见性。

### Owner 验收项「新增账户立即显示」：**未闭环 / 待裁决**

- 现状：数据链路正常，但 390px 视口下真实用户看不到新账户名（`#view.innerText` 与 `document.body.innerText` 均不含，`td:first-child` 的 computed display=none）。因此「立即显示」这一**视觉**验收项**未通过**，本报告不写成 PASS；`I-ACCT-06` 保持 FAIL，D 组 PASS 仅代表 DOM/API 数据一致性。
- 可选方案（**不替 Owner 选择、不实施**）：
  - **方案 A**：把 `style.css` L331 的窄屏选择器限定到交易总表（例如只作用于交易表的 `td:nth-child(1)`/`td:nth-child(5)`，或给交易表单独 class），使账户表名称列在窄屏保持可见；需评估对交易总表/其它表格布局的连带影响，并走样式变更授权。
  - **方案 B**：维持现状（数据链路正确、CSS 不动），仅把该视觉缺陷登记为待裁决项，等待 Owner 决定是否纳入后续范围。

## 4. 新增/修改文件清单

| 路径 | 类型 | 说明 |
|---|---|---|
| `server/web/dist/app.js` | 修改（唯一业务代码） | 缺陷 1/2/3 最小修复，43 insertions / 9 deletions |
| `tools/r0/e2e/run.js` | 修改 | 注册 `--suite d`（`all` 保持既有 A+C 组合不变） |
| `tools/r0/e2e/specs/c-controls.js` | 修改 | `groupTxns` 改为逐条真实执行；`I-ACCT-06` 回退为无条件 FAIL 并补全可见性原始观测 |
| `tools/r0/e2e/specs/txns-controls.js` | 新增 | I-TXNS-01..19 共享逐条配方（C 与 D 复用，避免重复实现） |
| `tools/r0/e2e/specs/d-defects.js` | 新增 | D 套件 39 条回归用例；新增账户组标注「DOM 渲染一致性，不含视觉可见性」 |
| `tools/r0/e2e/scripts/baseline-probe.js` | 新增 | 修复前基线探针（账户 3 次复现 + 三项 DOM 观测） |
| `tools/r0/e2e/scripts/negative-check.js` | 新增 | 负向合成验证（注入修复前 app.js） |
| `.github/workflows/r0-e2e.yml` | **由 team-lead 侧修改** | 依补充授权新增 `--suite d` 门禁 step + 汇总退出码写入 d outcome；本工程师仅核对，未重复修改 |
| `docs/r0-evidence-7th-fix/**` | 新增 | 全部证据，未覆盖任何历史证据 |

## 5. 自证与 CI 门禁

- `after/diff.txt`：`git diff --stat` + `git diff -- server/web/dist/app.js` 全文，并附 `c-controls.js`（含 I-ACCT-06 回退片段）与 `.github/workflows/r0-e2e.yml` 的 diff 供复核。
- `after/git-status.txt`：`git status --porcelain` 显示 4 个已跟踪文件被修改（`app.js` / `run.js` / `c-controls.js` / `.github/workflows/r0-e2e.yml`）+ 允许的新增测试脚本/证据目录。**除 workflow 外无禁止文件被修改**；workflow 改动来源与授权已在文件中注明。未改动任何 Go 后端、PRD/WBS/设计规范、数据库/迁移、API 契约、`docs/r0-evidence-6th/` 及更早证据、README/CHANGELOG、既有 A/B/C 状态结论文档、`style.css`、其它既有规格文件。
- **CI 门禁核对**（`.github/workflows/r0-e2e.yml`，由 team-lead 依补充授权落地，mtime 17:50，非本工程师修改；本工程师核对后未重复改动，也未静默略过）：
  - 新增 `- name: 包 D · 四项业务缺陷回归（门禁）`，`id: d_defects`，`run: node run.js --suite d`，位于**包 A 之后、包 C 之前**；
  - 该 step **无 `continue-on-error`**，D 失败即 job 失败（门禁）；
  - 「汇总退出码」step 新增 `echo "- 包 D 退出码（四项缺陷回归门禁）: ${{ steps.d_defects.outcome }}"`，如实写入 d 的 outcome；
  - 既有 A/B/C step 语义未变，C 的 `continue-on-error: true` 保留（未改成 pass），C 的 outcome 行文案加了「报告型，success 不代表无 FAIL」说明但取值原样；无既有步骤被删除或覆盖。
- 所有 DOM 文本仍使用 `textContent`：`app.js` 中 `innerHTML` 仅出现在第 2 行既有安全注释「禁止 innerHTML 拼接」，修复前后一致，无实际 `innerHTML` 赋值；未改变产品范围/业务规则；未隐藏错误、未放宽断言、未跳过测试、未用 sleep/延迟刷新制造 PASS（唯一受控延迟用于 I-TXNS-17 采集加载态，不参与任何 PASS 判定）。
- 逐条截图：C 最新一次运行 `C/I-TXNS/` 下 20 张（I-TXNS-01..19 各 1 张 + 骨架屏加载态 1 张）与 `C/I-ACCT/I-ACCT-06-visibility.png`；D 最新一次运行 40+ 张（39 条各 1 张 + 骨架屏加载态 1 张）。每条控件独立判定、独立截图。
- 全局一致性复核（**IS_PASS: YES**）：全部 JS 文件 `node --check` 通过；依赖图 `run.js → {c-controls, d-defects} → txns-controls → lib/api` 无缺失、无循环；C 与 D 复用同一份 `txns-controls.js` 配方，无重复实现；`runTxnsControls` 返回 `{R}` 与 `runControls`/`runDefects` 的调用契约一致；证据路径均经 `runDir.rel()` 归一化。

## 6. 新发现但**未修改**的问题（只登记）

1. **（第 5 项「候选之一」，未获 Owner 确认，不选定、不实施）** 目前有两个候选，均只登记：
   - **候选 1（本次复现所得）**：`server/web/dist/style.css` L331 `@media (max-width:640px)` 的全局 `th/td:nth-child(1)`、`nth-child(5)` 规则未限定表格，390px 下把账户表「账户名称」列隐藏，真实移动端用户看不到账户名，同时隐藏交易表复选框列/分类列。文件不在允许修改范围，未改。
   - **候选 2（team-lead 已向 Owner 报告）**：`server/web/dist/app.js` 成员页 L1723–1758 的「设为管理员 / 降级为成员 / 移除成员」按钮未按操作者角色隐藏，普通成员可能看到无权限的操作入口（后端仍有 403 兜底）。不在本次 4 项授权范围，未改。
2. **A08 静态文案过时**：`a-scenarios.js` A08 的 detail 仍写「UI 未隐藏=真实缺陷，已记录」，修复后实际观测值已是「邀请成员按钮数=0」。A 的 PASS 判定（API 403）未变。因禁止修改 A 套件语义，本次未改该文案，如实登记。
3. **既有编号漂移**：`fixtures/controls.json` 中 `I-ACCT-06` 定义是「删除引用保护」，而 `c-controls.js` 把「创建成功并在页面渲染」结果写在该 ID 上；第六次报告因此把「账户 · 删除引用保护」显示为 FAIL。本次按收紧裁决让该条**保持 FAIL**（detail 补全正向与可见性观测），**未调整编号映射**。
4. **既有测试盲点**：`c-controls.js` 的 `R['I-ADD-06b']`（分类首项判定）不在 228 个控件 ID 集合内，`runControls` 会静默丢弃，故 C 报告看不到分类未过滤缺陷。未新增控件 ID，改由 D 套件显式覆盖。
5. **`I-ACCT-10` 既有假阳性**：「删除后从列表消失」在名称列被 CSS 隐藏的前提下恒为真。不在 4 项授权范围，未改。

## 7. 未解决问题

1. **Owner 验收项「新增账户立即显示」未闭环/待裁决**：视觉可见性未通过，`I-ACCT-06` 保持 FAIL，等待 Owner 在方案 A / 方案 B 之间裁决；本轮不实施、不替 Owner 选择，未改动 `style.css`。
2. C 套件仍有 **5 条 FAIL**（4 条基线既有产品缺口 + `I-ACCT-06` 可见性未闭环）与 59 条既有 BLOCKED，导致 `--suite c` 退出码为 1。按边界不得为让测试变绿而放宽断言或扩大修复范围，保持原状并如实记录。
3. 第 5 项缺陷（两个候选）未获 Owner 确认，未做任何处理、未选定、未实施。
4. `.github/workflows/r0-e2e.yml` 的门禁 step 已由 team-lead 侧加入并通过本工程师核对，但**尚未在干净 CI 环境实际执行**（本轮只做了本地 a/c/d 运行与 YAML 结构核对）；CI run 的证据需由 team-lead 侧触发后补齐。
