# FastLedger R0 · 第七次有限范围业务缺陷修复 · 交付回执

> 性质：**有限范围缺陷修复**，不是进入下一开发阶段；**R0 仍 BLOCK**，不授权 R1–R7，不启动 PWA。
> 分支：`fix/r0-sixth-runtime-defects`（独立分支，**未合并 main**）。
> 开工基准：重新 `fetch` 后 `origin/main` = **`dadc9a3b9c567cad2bbad7e47361fb774365b949`**（`[1.81]` 第七次测试质量纠偏任务单，5 个文档类提交）。本地 `48d0fc4` 已快进，工作树干净后才建分支。
> 交付 commit：见文末「提交与 CI」小节（PR 创建后回填）。
> 全部证据：`docs/r0-evidence-7th-fix/`（本轮新建；`docs/r0-evidence-6th/` 及更早证据**未改动、未覆盖**）。

---

## 0. 授权边界与纪律自检

| 纪律项 | 落实情况 |
|---|---|
| 开工前检查最新 main HEAD、记录基准 SHA | 已做；远端实际已前进到 `dadc9a3b`，非本地旧的 `48d0fc4` |
| 先建修复分支，不在 main 上做未经审核的业务修改 | 已做；`main` 保持在 `dadc9a3b`，零改动 |
| 每项缺陷先给根因与最小修改方案 | 已做；见 `docs/r0-evidence-7th-fix/root-cause-and-plan.md`，先复现、后动代码 |
| 不隐藏错误 / 不放宽断言 / 不跳过测试 / 不延迟刷新制造 PASS | 已做；`I-ACCT-06` 曾被工程师改判 PASS，**team-lead 明令回退为无条件 FAIL**，见 §4.4 |
| 每项修复新增或完善自动化回归测试 | 已做；新增 D 套件 39 条 + 18 条控件由硬编码 BLOCKED 改为真实逐条执行 |
| 不改 PRD / 业务规则 / DB / API 契约 / 产品范围 | 已做；`git diff` 仅含 1 个前端业务文件 + 测试 + CI 配置 + 证据 |
| 保留 A/C 框架与历史证据 | 已做；A/C 套件语义未变，C 仍报告型 `continue-on-error`，历史证据目录零改动 |
| 不启动 R1–R7 / 不启动 PWA | 已遵守 |
| 提交 PR 或独立分支，等第三方审核，未审核不合并、不宣布 R0 通过 | 已做；见文末 |

**越界审计**：QA 独立出具 `qa/scope-audit.md`，确认 `server/web/dist/style.css`、Go 后端、PRD/WBS、DB/迁移、API 契约、历史证据、README/CHANGELOG 状态结论均未被改动（README/CHANGELOG 本回执仅**追加**新条目，不改写既有结论）。

---

## 1. 修复前后代码差异

完整 diff：`docs/r0-evidence-7th-fix/after/diff.txt`；QA 复核：`docs/r0-evidence-7th-fix/qa/scope-audit.md`。

业务代码**只改 1 个文件**：`server/web/dist/app.js`（+52 / −? 行，4 处，均为最小修改）。

| 位置 | 修复前 | 修复后 |
|---|---|---|
| `renderTxns` L385 后 | 构造 `card` 并组装筛选行/批量条/表格容器，但函数内**从未** `view.append(card)` | `card.append(tableWrap);` 之后补 `view.append(card);`（与 `renderHome` L230 / `renderBooks` L302 / `renderAccounts` L711 一致） |
| `renderTxns.load()` L399–402 | `await api(...)` 无 catch；请求失败时骨架屏永久残留 | try/catch 包裹；失败时 `renderLoadError(tableWrap, e)` 清除骨架屏并用既有 `.empty` 样式 + `textContent` 呈现静态失败文本，**异常继续抛给既有 catch 负责 toast**；不新增按钮/交互/重试 |
| `openTxForm` L535 / L542–547 / L568 | `fillSelect($("#fCategory"), state.categories)` 一次塞入 30 个混合分类；`syncFormFields` 只切显隐不重建下拉；提交只校验 `accountId` | 删除无差别全量填充；新增 `refillCategories()` 按 `catType === t` 过滤并重置选中（与编辑表单 L602 口径一致），匹配集为空给 value=0 占位；`syncFormFields` 调用它；提交校验收紧为 `!body.accountId \|\| !body.categoryId`。**转账隐藏分类行、提交 `categoryId:0` 的既有语义未变** |
| `renderMembers` L1678 / L1683–1684 / L1710 | `row.append(h, invBtn)` 在拿到身份前无条件挂载邀请按钮；`me` 无空值防御 | 改为 `row.append(h)`；取回 `members` 与 `me` 后按 `members.find(m => m.uid === me.id).role` 判定，仅 `1`(所有者)/`2`(管理员) 才追加按钮；`/users/me` 失败时 `me=null` → 失败关闭；`myUid` 改为 `me ? me.id : null`。**后端 service 层 403 校验一行未动** |

**第 4 项（新增账户）：业务代码零改动**——稳定复现证明 `renderAccounts`/`loadBookMeta` 正确，详见 §4.4。

测试与 CI 侧改动（均非业务代码）：`tools/r0/e2e/run.js`（注册 `--suite d`）、`specs/c-controls.js`（18 条控件真实执行 + I-ACCT-06 判定）、`specs/a-scenarios.js`（A08 陈旧文案，纯文案）、`.github/workflows/r0-e2e.yml`（新增 D 门禁 step + 汇总写入 d outcome + job 名，既有 A/C step 语义未变）。

---

## 2. 根因与修改文件 / 函数

| # | 缺陷 | 文件 / 函数 / 行号（修复前） | 根因 | 是否触及 PRD/API/DB |
|---|---|---|---|---|
| 1 | 交易总表整页空白 | `app.js` `renderTxns()` L337–527；`view.replaceChildren()` L345、构造 card L347–385、L526 `await load()` | `card` 挂在游离节点上从未插入 `#view`；`load()` 把表格渲染进游离 `tableWrap`，故网络 200、无 console error、但 `#view.childElementCount=0` | 否 |
| 2 | 记一笔分类未按类型过滤 | `app.js` `openTxForm()` L530–580；L535、L542–547 | 支出默认类型下下拉首项是收入类「工资」(id=23, catType=1) 且被默认选中 → 提交 `txType=2/categoryId=23` 被服务端 400；切换类型不重建下拉导致残留收入类 ID | 否 |
| 3 | Member 仍见邀请按钮 | `app.js` `renderMembers()` L1636–1765；L1647、L1678（身份与成员数据 L1683–1684 才取回） | 前端在拿到当前用户身份前就画出入口，且从未按角色判断；owner/admin/member 三种身份按钮数均为 1。后端 403 实测有效 → 属 UI affordance 越权 | 否 |
| 4 | 新增账户列表「不刷新」 | `app.js` `openAcctForm` L715–748（L745 已有 `await renderAccounts`）、`renderAccounts` L647–712、`loadBookMeta` L306–314；`style.css` L331；`c-controls.js` `groupAccounts` L318–328 | **业务 JS 无缺陷**：3/3 次复现 POST 200、API 回读含新名称、`#view` 重绘（tbody 3→4→5→6）、名称在 `textContent` 中。真因是 `style.css` L331 `@media(max-width:640px) td:nth-child(1){display:none}` 无表格限定，把账户表「名称」列隐藏；旧测试用 `document.body.innerText` 判定恒 false。属「DOM 有但不可见」，不是「API 有、DOM 无」 | 否 |

---

## 3. 自动化测试代码

| 文件 | 作用 |
|---|---|
| `tools/r0/e2e/specs/d-defects.js`（新增） | 缺陷回归主套件，39 条：`D-TXNS-01..04`（有数据/空数据/注入 500 失败/行编辑）、`D-CAT-01..06`（收入支出集合相等/切换重置/创建持久化/转账语义/刷新）、`D-MEM-01..06`（owner/admin 可见可用、member 不可见、直连接口 403、成员列表不受影响、`/users/me`=500 失败关闭）、`D-ACCT-01..04`（DOM 一致性，不含视觉可见性）、以及复用 `runTxnsControls` 的 `I-TXNS-01..19` 逐条执行 |
| `tools/r0/e2e/specs/txns-controls.js`（新增） | I-TXNS-01..19 逐控件真实配方：真实点击/输入/提交 + 网络响应码 + API 回读，每条独立 PASS/FAIL/BLOCKED 与独立截图；宽视口 1000×844 以避开窄屏 CSS 隐藏列 |
| `tools/r0/e2e/specs/c-controls.js`（完善） | `groupTxns` 由「整页判空 + 18 条硬编码 BLOCKED」改为真实逐条执行；`groupAccounts` 改为 POST 响应 + 行数 +1 + `td.textContent` + API 回读 + 无重复的确定性判定，并**保留两个可见性原始观测** |
| `tools/r0/e2e/run.js`（完善） | 注册 `--suite d`，退出码规则不变 |
| `tools/r0/e2e/scripts/baseline-probe.js`、`negative-check.js`、`qa-load-failure-shot.js`（新增） | 修复前基线探针、负向合成验证、清晰失败截图补拍 |
| `.github/workflows/r0-e2e.yml`（完善） | 新增 `包 D · 四项业务缺陷回归（门禁）`（`id: d_defects`，A 之后 C 之前，**无** `continue-on-error`）；汇总写入 d outcome；C 仍为报告型 |

**判定有效性**：QA 抽检见 `qa/assertion-review.md`；负向合成验证见 `after/negative-check.md`（在网络层把 `app.js` 换成 `git show HEAD:server/web/dist/app.js`，即修复前版本，证明新断言不是恒真式）。QA 同时指出该负向验证的**局限**：NEG-4 的「修复前会 FAIL」推断只适用于旧 C 的 `I-ACCT-06`，`D-ACCT-01..04` 在修复前也会 PASS（因业务代码零改动），且浏览器版本未写入 JSON——已在 §6 登记，不掩盖。

---

## 4. 实际运行日志、退出码与验收结果

### 4.1 修复前基线（干净临时实例，工程师）

| 项 | 结果 | 证据 |
|---|---|---|
| 构建 | `go1.22.5 / CGO_ENABLED=1 / w64devkit gcc` 成功 | `after/build.log` |
| C 套件 | `PASS 144 / FAIL 6 / BLOCKED 77 / N/A 1`，`EXIT_CODE=1` | `baseline/c-before.log`、`c-before-report.json` |
| 交易总表 | `#view.childElementCount=0`，`innerText` 空，而 `GET /transactions` = 200 | `baseline/pre-fix-observations.md`、`baseline-probe.json` |
| 分类下拉 | 支出/收入/转账/切回支出都是同一份 30 项混合下拉，首项「工资」(23, catType=1) | 同上 |
| 邀请按钮 | owner/admin/member 三种身份按钮数均为 1；member 直连 POST invite = 403 | 同上 |
| 新增账户 | **3/3 次** POST 200、API 回读含名称、`textContent` 含名称、`innerText` 不含、首列 `display:none`、刷新后仍不可见、无 console error | `baseline/account-repro.md`（含 MutationObserver 重绘记录） |

### 4.2 修复后（工程师本地）

| 套件 | 结果 | 退出码 | 日志 |
|---|---|---|---|
| A（11 项状态/权限/同步） | 11/11 PASS | **0** | `after/a-after.log` |
| D（39 条缺陷回归） | 39/39 PASS | **0** | `after/d-after.log` |
| C（228 条控件） | PASS 163 / FAIL 5 / BLOCKED 59 / N/A 1 | **1**（5 条 FAIL 触发既有退出码规则） | `after/c-after.log` |

### 4.3 QA 独立复核（重新构建 + 新证据目录 + 新临时实例，不采信工程师结果）

| 套件 | 实例 | 结果 | 退出码 | 日志 |
|---|---|---|---|---|
| 构建 | — | go1.22.5 / CGO / w64devkit | **0** | `qa/build.log` |
| A | `127.0.0.1:65208` | 11/11 PASS | **0** | `qa/a-qa.log` |
| D | `127.0.0.1:60064` | 39/39 PASS | **0** | `qa/d-qa.log` |
| C | `127.0.0.1:58520` | PASS 163 / FAIL 5 / BLOCKED 59 / N/A 1 | **1** | `qa/c-qa.log` |

浏览器统一 `playwright-core 1.64.1 / msedge 156.0.4314.58`。三份原始 `report.json` 已复制；QA 结论：**与工程师逐项一致、无差异**；智能路由判定 **NoOne**（不回工程师、无需 QA 自修测试代码）。详见 `qa/qa-report.md`、`qa/acceptance-matrix.md`、`qa/assertion-review.md`。

### 4.4 四项验收逐项结论

| 授权项 | Owner 验收要求 | 结论 | 关键运行时观测 |
|---|---|---|---|
| ① 交易总表 | 有数据 / 空数据 / 请求失败 / 列表实际显示 / 详情跳转 / 18 条控件重跑 | **PASS** | 有数据 3 行（备注「工资/打车/早餐」）；空态「没有符合条件的交易」；注入 500 → 可见「交易加载失败：E2E 注入的服务器错误」+ `aria-busy=0` + `.skel=0` + toast；行「编辑」打开 `#editDlg` 且 `eType` 与行类型一致；**I-TXNS-01..19 共 19 条全部 PASS**（修复前 01 FAIL + 02..19 BLOCKED） |
| ② 分类筛选 | 收入只显示收入类 / 支出只显示支出类 / 切换状态正确 / 创建成功且持久化 | **PASS** | 收入下拉 8 项 == API `catType=1` 8 项、支出 22 项 == `catType=2` 22 项、混入 0；支出选中 22 → 切收入重置 23 → 切回支出重置 1；支出 POST 200，请求体/API 回读 `txType=2, categoryId=1` 一致，刷新后仍在；转账 `categoryId=0`、`toAccountId=2` 语义未变 |
| ③ 成员邀请 | Owner/Admin 正常 / 无权限 member 看不到 / 直接调接口仍被拒 / 不影响其它成员管理 | **PASS** | owner/admin 按钮=1 且弹窗可开；member 按钮=0、`#view` 与 body 文本均无「邀请成员」；member 直连 `POST /members/invite` → **403**；成员列表 3 行、角色徽标「所有者/管理员/普通成员」、移除按钮 2 个均正常；`/users/me`=500 时 owner 也不显示入口（失败关闭）且列表仍渲染 |
| ④ 新增账户 | 稳定复现 / 立即显示 / 刷新后仍存在 / 不产生重复 | **部分：DOM 一致性 PASS，视觉可见性 FAIL（保持）** | 稳定复现 3/3；POST 200、tbody 3→4、`td.textContent` 命中、API 回读、刷新后仍在、账户总数 +1 且名称只出现 1 次、DOM↔API 名称集合完全一致。**但 `visibleInnerText=false`、`firstTdDisplay=none`**：390px 视口下真实用户仍看不到账户名，`I-ACCT-06` **保持 FAIL**，见 §6 |

**关于 ④ 的诚实声明**：`I-ACCT-06` 在 `c-controls.js` 中是**唯一赋值、无条件 `bad()`**，不存在任何 PASS 分支；detail 与 `C/I-ACCT/visibility-observations.json` 保留 `visibleInnerText`/`firstTdDisplay` 两个原始观测；C 报告 FAIL 恰为 5。D 套件 `D-ACCT-01..04` 是另立的「DOM 渲染一致性，不含视觉可见性」用例，**不是**把既有 FAIL 改判为 PASS。工程师最初曾把该条改判 PASS，team-lead 已明令回退并重跑 C（FAIL 由 4 回到 5）。

### 4.5 Playwright 截图 / Trace

- `qa/screenshots/` 共 **18 张** PNG，覆盖交易总表有数据/空态/加载失败/行编辑/骨架屏、分类下拉与切换与创建、owner/admin/member 成员页、失败关闭、账户列表首列被隐藏等。`05b-txns-load-failure-clear.png` 为 team-lead 目检通过的清晰失败态证据。
- **Trace 未复制入仓**：QA 解压 `trace.zip` 后在 `trace.network` 命中真实 JWT（`eyJ…`）。仓库为 Public，按既有安全事件口径以脱敏为最高优先级，QA 决定不复制任何 trace；原始 artifacts 仅留在被 `.gitignore` 忽略的 `artifacts/` 目录。此点已作为 **Q-3 风险**如实登记。
- 完整性校验：`qa/SHA256SUMS.txt`。

### 4.6 原有 228 条控件中受影响项目的重新执行结果

逐条比对脚本基于 `baseline/c-before-report.json` 与 `qa/c-report.json` 的 `results[].id/status`：**共 19 条状态发生变化，全部集中在交易总表，其余 209 条状态不变**（含 `I-ACCT-06` 保持 FAIL）。

| 控件 ID | 修复前 | 修复后 |
|---|---|---|
| `I-TXNS-01` | FAIL | **PASS** |
| `I-TXNS-02` ~ `I-TXNS-19`（18 条） | BLOCKED | **PASS** |

汇总：`PASS 144 → 163`、`FAIL 6 → 5`、`BLOCKED 77 → 59`、`N/A 1 → 1`，总数 228 不变。逐条明细见 `qa/c-report.json` 与 `qa/acceptance-matrix.md`。

---

## 5. 第五项缺陷（**未获确认，未修改**）

仓库中「第六次自动化验收已确认业务缺陷」的正式登记只有 **4 条**（第六次回执 §3.3、第七次任务单 §5、README `[1.81]`），**不存在第五条已登记缺陷**，因此不虚构。代码核查发现两个真实但**不在授权范围**的候选，均**只登记、未修改**，等待 Owner 单独确认：

### 候选 A · 成员页角色管理按钮未按操作者角色隐藏

| 项 | 内容 |
|---|---|
| 准确名称 | 成员页「设为管理员 / 降级为成员 / 移除成员」管理按钮未按当前用户角色隐藏，普通成员可见管理入口 |
| 文件路径 | `server/web/dist/app.js` |
| 函数 | `renderMembers(bookId)` L1636–1765 |
| 行号 | L1723–1758：`L1723 if (mb.role === 3)` → L1724 建「设为管理员」；`L1733 else if (mb.role === 2)` → L1734 建「降级为成员」；`L1744 if (mb.role !== 1)` → L1745 建「移除成员」。三处只按**目标成员角色**判断，从不判断**当前操作者角色**；`myUid = me.id` 仅用于标注「（我）」 |
| 复现步骤 | ① 家庭账本准备 owner 与 member 两个账号；② 以 member 登录进 `#b/{bookId}/members`；③ 观察成员表格「操作」列；④ 点击「设为管理员」或「移除成员」并确认 |
| 实际结果 | member 能看到并点击全部管理按钮；点击后服务端返回 403，前端只弹 toast 报错（A08 已证实管理接口 403，但只覆盖 invite） |
| 预期结果 | member 只能看到成员名单/角色/可见性，看不到也触发不了任何管理操作；owner/admin 行为不变；后端 403 校验保留 |

### 候选 B · 窄屏 CSS 隐藏账户名称列（第 4 项根因链上的未授权项）

| 项 | 内容 |
|---|---|
| 准确名称 | `style.css` 窄屏响应式选择器未限定表格，误隐藏账户表「账户名称」列 |
| 文件路径 | `server/web/dist/style.css` |
| 位置 | L327–333；**L331** `th:nth-child(1), td:nth-child(1), th:nth-child(5), td:nth-child(5) { display:none; }`（包在 `@media (max-width: 640px)` 内） |
| 复现步骤 | ① E2E 视口 390×844（或任何 ≤640px 视口）；② 进入账户页 `#b/{bookId}/accounts`；③ 观察表格列；④ 新增账户后观察名称是否可见 |
| 实际结果 | 表头只剩「类型 / 余额 / 操作」；所有账户行（含既有现金/储蓄卡/信用卡及新建账户）的名称列 `getComputedStyle().display === 'none'`，`innerText` 不含名称；刷新后仍不可见。截图 `qa/screenshots/15-accounts-list-first-col-hidden.png` |
| 预期结果 | 该规则只作用于它本意的交易总表复选框/分类列，账户表名称列始终可见；或按产品裁决另行处理 |

> 两个候选均**未修改**。若 Owner 所说的「第五项」另有所指（例如第七次复核中的 A05 假阳性、C 硬编码 PASS、清单 SHA 漂移、t021 override 越权等测试质量问题），请指明；上述四项同样不在本次业务修复授权内，本轮一律未动。

---

## 6. 新发现问题与未解决问题

### 6.1 未解决 / 待 Owner 裁决

1. **`I-ACCT-06` 视觉可见性未闭环**：390px 下用户看不到新增账户名，Owner 验收项「新增账户立即显示」**未达成**。`I-ACCT-06` 保持 FAIL。可选方案 **A**：把 `style.css` L331 选择器限定到交易总表（需单独授权改样式）；**B**：维持现状仅登记。本轮不实施、不替 Owner 选择。
2. **第五项缺陷**：见 §5，两个候选均待确认。
3. **C 的另外 4 条 FAIL + 59 条 BLOCKED**：`I-LOGIN-04`(R0-E-39)、`I-LOGIN-11/12`(R0-E-38)、`I-HOME-01`(R0-E-01) 为既有已登记产品缺口；59 条 BLOCKED 为数据前置/浏览器输入能力等既有阻塞。非本轮范围。
4. **R0 仍 BLOCK**：A/B/C/D 状态与 RV-05 均未因本轮改判；不授权 R1–R7，PWA 未启动。RV-05（仓库仍 Public、旧 `259bd11` 可匿名读）仍 **Owner ACTION REQUIRED**。

### 6.2 QA 新发现（4 项，均不改变判定）

| ID | 问题 | 处置 |
|---|---|---|
| Q-1 | 框架 `step()` 在 `fn()` 结束后才截图，凡 fn 内复位/关弹窗/刷新者截图不反映判定瞬间；部分截图 SHA256 相同、个别极淡或全白 | **未解决**：判定本身不依赖截图，结论不受影响；已另拍 `05b-txns-load-failure-clear.png` 作为加载失败的可读证据（team-lead 目检通过）。框架改造留待后续授权 |
| Q-2 | `a-scenarios.js` A08 detail 含修复前遗留硬编码文案，与当前 `inviteBtn=0` 语义矛盾 | **已修**（纯文案，不动断言/判定） |
| Q-3 | Playwright `trace.zip` 的 `trace.network` 含真实 JWT；Public 仓库不得入库 | **已按脱敏优先处置**：不复制任何 trace；建议框架侧接入 `tools/r0/redaction.js` 对网络层脱敏后再上传 artifact（未实施，登记） |
| Q-4 | `r0-e2e.yml` job 名未含 D | **已修**（仅 name 一行） |

### 6.3 其它如实登记

- **负向合成验证的局限**（QA 指出）：它是「合成观测 + 文字推断」，未在修复前 `app.js` 下真正运行 D/C 套件；NEG-4 的「修复前会 FAIL」只对旧 C 的 `I-ACCT-06` 成立，`D-ACCT-01..04` 在修复前也会 PASS（第 4 项业务 JS 零改动）；浏览器版本未写入 `negative-check.json`。不推翻核心结论（NEG-1/2/3 与 QA 修复后观测形成完整反差，`qa/scope-audit.md` 亦从 diff 层独立确认 3 处源码修复）。
- **既有测试编号漂移**：`fixtures/controls.json` 中 `I-ACCT-06` 的定义是「删除引用保护」，而脚本把「创建成功并在页面渲染」写在 `R['I-ACCT-06']` 上，语义错位。本轮**不调整编号映射**（避免改变 A/C 语义），仅登记。
- **既有测试盲点**：`R['I-ADD-06b']` 不在 228 个 ID 集合内，结果被 `runControls` 静默丢弃；本轮不新增控件 ID，改由 D 套件显式覆盖。
- **`I-ACCT-10`「删除后从列表消失」** 在名称列被隐藏前提下恒为真，属既有假阳性；本轮未改该条判定，仅登记。
- **本地 Go 全量单测**：`go test ./... -count=1` 中 9 个包 ok，`internal/ws` 因本机 Application Control 策略拦截 `ws.test.exe` 报 FAIL（已知环境限制，非代码回归）；全仓 `-race` 仍以 GitHub 原生 CI 为准。

---

## 7. 提交与 CI

- 分支：`fix/r0-sixth-runtime-defects`，基准 `dadc9a3b9c567cad2bbad7e47361fb774365b949`。
- 交付 commit SHA：**见下方回填**。
- CI run：**见下方回填**（`R0 E2E` 与 `CI` 两个工作流；A 门禁 / D 门禁 / C 报告型）。
- PR：**待创建 / 见下方回填**，供 ChatGPT 第三方审核；**未经审核不合并 main，也不宣布 R0 通过**。
- 凭证扫描：`node tools/r0/scan-credentials.js --strict` 退出码 0；未向仓库引入任何 token、密钥或含 JWT 的原始网络日志。

> CI_RUN_PLACEHOLDER
