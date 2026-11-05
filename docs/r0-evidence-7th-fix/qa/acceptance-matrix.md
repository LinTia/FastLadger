# QA 独立复核 · Owner 验收清单逐项核对（C）

> 所有数值均来自 **QA 自己重新构建 + 新证据目录 `artifacts/r0-qa-verify` 重新起干净实例重跑** 的原始日志与 `report.json`，不引用工程师 `after/` 下的 artifacts 作为结论。
> 证据根目录：`docs/r0-evidence-7th-fix/qa/`；本文中的 `qa/xxx` 均指该目录下文件。
> 复核时间：2026-11-05T09:59Z ~ 10:02Z（UTC）

## 0. 独立重跑事实（本次 QA 基线）

| 项 | 命令 / 观测 | 退出码 | 证据 |
|---|---|---|---|
| 重新构建 | `source .tools/genv.sh && go build -o fastledger.exe ./cmd/server`（go1.22.5 windows/amd64, CGO_ENABLED=1） | **0** | `qa/build.log` |
| 包 A | `R0_SERVER_BIN=…/server/fastledger.exe R0_BROWSER_CHANNEL=msedge R0_ARTIFACT_DIR=…/artifacts/r0-qa-verify node run.js --suite a` | **0** | `qa/a-qa.log`、`qa/a-report.json` |
| 包 D | 同上 `--suite d` | **0** | `qa/d-qa.log`、`qa/d-report.json` |
| 包 C | 同上 `--suite c` | **1**（因 5 条 FAIL，符合 `run.js` 的 `fails.length` 规则） | `qa/c-qa.log`、`qa/c-report.json` |

- 三套件各自启动**独立临时 SQLite + 随机端口 + 新生成 JWT 密钥**：A `127.0.0.1:65208`、D `127.0.0.1:60064`、C `127.0.0.1:58520`，互不复用。
- 浏览器统一 `playwright-core 1.64.1 / channel=msedge / version 156.0.4314.58`。
- 汇总：A `11/11 PASS`；D `39/39 PASS`；C `PASS 163 / FAIL 5 / BLOCKED 59 / N/A 1（total 228）`。

---

## 1. 交易总表（`renderTxns` 恢复挂载 + I-TXNS-19）

| 验收项 | 判定 | 实际观测值（QA 本次） | 证据 |
|---|---|---|---|
| 有数据：行数 / 备注文本 | **PASS** | `D-TXNS-01`：`#view.childElementCount=1`，标题「E2E 主账本 · 交易」，筛选下拉 3 个，`tbody 3 行`，备注列 `["工资","打车","早餐"]`，API 交易数=3 | `qa/d-report.json`、`qa/d-qa.log`、`qa/screenshots/01-txns-with-data.png` |
| 空数据：空态文本 | **PASS** | `D-TXNS-02`：空账本空态「🧾 没有符合条件的交易——换个筛选条件，或记一笔。」，`tbody 1 行`，API 交易数=0；`I-TXNS-18` 独立复现：HTTP 200 + 空态「🧾 没有符合条件的交易——换个筛选条件，或记一笔。」 + `tbody 1 行` | `qa/d-report.json`、`qa/c-report.json`、`qa/screenshots/02-txns-empty.png` |
| 请求失败：注入 500 后可见反馈 + `aria-busy` 清除 | **PASS** | `D-TXNS-03`：route 注入 HTTP 500 → `#view` 含「交易加载失败：D套件注入500（可重新查询或刷新页面）」，`aria-busy` 残留 **0**，`.skel` 残留 **0**，toast「✕ D套件注入500」；`I-TXNS-19` 独立复现：可见反馈「交易加载失败：E2E 注入的服务器错误（可重新查询或刷新页面）」，`aria-busy=0`，toast「✕ E2E 注入的服务器错误」 | `qa/d-report.json`、`qa/c-report.json`、`qa/screenshots/05-txns-load-failure.png` |
| 交易列表实际显示 | **PASS** | 3 行真实数据（日期/类型/账户/分类/备注/金额/编辑），见 `01-txns-with-data.png`；宽视口下 3 行含复选框列见 `17-txns-wide-viewport.png` | `qa/screenshots/01-txns-with-data.png`、`17-txns-wide-viewport.png` |
| 详情 / 编辑跳转正常 | **PASS**（编辑） / **BLOCKED**（详情组 1 条，见备注） | 编辑：`D-TXNS-04` 行「编辑」按钮数=3 → `#editDlg.open=true`，`eType=「收入」` 与行类型一致，`eAmount=「8,000.00」`；`I-TXNS-15` 同上且取消后关闭=true。详情组 `I-TXDET-01..14`：**PASS 13 / BLOCKED 1**（`I-TXDET-13` 因"member 访问他人交易需构造他人交易，种子数据均为 owner 创建"而 BLOCKED，属既有的数据前置缺口，非本轮修复引入） | `qa/d-report.json`、`qa/c-report.json`、`qa/screenshots/04-txns-row-edit.png` |
| 骨架屏加载态 | **PASS** | `I-TXNS-17`：慢网络下观测到 `aria-busy=true`、`.skel 5 条`；响应后残留=0、`tbody 3 行` | `qa/d-report.json`、`qa/screenshots/06-txns-skeleton-loading.png` |

### 1.1 I-TXNS-01..19 逐条重跑结果（修复前：01 FAIL + 02..19 BLOCKED）

以下为 **C 套件与 D 套件两套独立重跑** 的结果，两套均为 **19/19 PASS**（逐条 detail 见 `qa/c-report.json`、`qa/d-report.json`）。

| ID | 控件名 | C | D | 关键运行时观测（摘要） |
|---|---|---|---|---|
| I-TXNS-01 | 关键词搜索 | PASS | PASS | HTTP 200，url 含 `keyword=`，命中 1 行备注「工资」且全部含关键词 |
| I-TXNS-02 | 类型筛选 | PASS | PASS | HTTP 200，url 含 `type=1`，1 行类型全为「收入」 |
| I-TXNS-03 | 账户筛选 | PASS | PASS | HTTP 200，url 含 `accountId=1`，2 行账户列 `["现金","现金"]` |
| I-TXNS-04 | 分类筛选 | PASS | PASS | HTTP 200，url 含 `categoryId=24`，1 行分类列 `["月薪"]` |
| I-TXNS-05 | 开始日期 | PASS | PASS | HTTP 200，url 含 `minTime=`，返回 3 行 |
| I-TXNS-06 | 结束日期 | PASS | PASS | HTTP 200，url 含 `maxTime=`，返回 3 行 |
| I-TXNS-07 | 查询按钮 | PASS | PASS | GET `…/transactions?page=1&count=50` → 200，重绘后 tbody 3 行 |
| I-TXNS-08 | 重置按钮 | PASS | PASS | HTTP 200，url 不含 `keyword`，`kw=""`、`selects=["0","0","0"]`、`dates=["",""]` |
| I-TXNS-09 | 回车提交 | PASS | PASS | HTTP 200，url 含 `keyword=`，返回 1 行 |
| I-TXNS-10 | 全选本页 | PASS | PASS | 3 行全部 checked，提示「已选 3 项」，删除选中 `disabled=false` |
| I-TXNS-11 | 行勾选 | PASS | PASS | 全选 3 项取消首行 → `checked=2`，提示「已选 2 项」 |
| I-TXNS-12 | 删除选中 | PASS | PASS | 真实 DELETE 请求已发出；API 列表已无该笔=true，回收站含该笔=true（C 用 txId=6、D 用 txId=5，各自独立创建专用交易） |
| I-TXNS-13 | 删除选中取消 | PASS | PASS | 点「取消」→ DELETE 请求数=0，交易数 3→3 |
| I-TXNS-14 | ＋ 记一笔 | PASS | PASS | `#formDlg.open=true`，分类下拉 22 项，取消后关闭=true |
| I-TXNS-15 | 行编辑 | PASS | PASS | `#editDlg.open=true`，`eType=「收入」` 与行类型一致，`eAmount=「8,000.00」` |
| I-TXNS-16 | 上一页 / 下一页 | PASS | PASS | 分页条「共 3 条 · 第 1 页」，API total=3，上一页/下一页均 disabled=true（数据不足一页，按设计禁用） |
| I-TXNS-17 | 骨架屏 | PASS | PASS | `aria-busy=true` + `.skel 5 条`；响应后残留=0、tbody 3 行 |
| I-TXNS-18 | 空态 | PASS | PASS | HTTP 200 + 空态「没有符合条件的交易」+ tbody 1 行 |
| I-TXNS-19 | 加载失败 | PASS | PASS | 注入 500 → 可见反馈 + `aria-busy=0` + toast 含注入文本 |

> 结论：**01..19 全部由「修复前 01 FAIL + 02..19 BLOCKED」闭环为 19/19 PASS**，且 C、D 两套独立重跑结果一致。

---

## 2. 分类筛选（`openTxForm` 过滤 / 重置 / 校验）

| 验收项 | 判定 | 实际观测值（QA 本次） | 证据 |
|---|---|---|---|
| 收入只含 `catType=1` 的分类 id 集合 | **PASS** | `D-CAT-02`：收入下拉 **8 项**，与 API `catType=1` 的 8 项**完全一致**（`JSON.stringify` 全等比较）；混入支出类 **0** | `qa/d-report.json`、`qa/screenshots/07-category-expense-dropdown.png` |
| 支出只含 `catType=2` 的分类 id 集合 | **PASS** | `D-CAT-01`：支出下拉 **22 项**，与 API `catType=2` 的 22 项完全一致；混入收入类 **0**；首项「餐饮」(id=1)。修复前负向观测为 30 项、首项「工资」、混入收入类 8 个（见 `after/negative-check.md` NEG-2） | `qa/d-report.json`、`qa/screenshots/07-category-expense-dropdown.png` |
| 类型切换后已选值重置 | **PASS** | `D-CAT-03`：支出选中 `22` → 切收入重置为 `23`（收入类）→ 切回支出重置为 `1`（支出类）；转账时 `#fCatRow.hidden=true`、`#fToRow.hidden=false` | `qa/d-report.json`、`qa/screenshots/08-category-type-switch.png` |
| 支出类型直接提交成功（POST 2xx） | **PASS** | `D-CAT-04`：捕获真实 POST → **HTTP 200**，请求体 `txType=2`、`categoryId=1`（支出类） | `qa/d-report.json`、`qa/screenshots/09-category-expense-created.png` |
| API 回读 `txType/categoryId` 一致 | **PASS** | `D-CAT-04`：`api.transactions` 回读命中备注「D-支出分类回归-1793872830719」，回读 `txType=2`、`categoryId=1`，与请求体一致 | `qa/d-report.json` |
| 刷新后仍存在 | **PASS** | `D-CAT-05`：刷新并等到 `tbody td:nth-child(6)` 确定性信号后，API 回读仍含该备注=true，交易总表备注列含该文本=true；备注列 `["工资","打车","早餐","D-支出分类回归-1793872830719"]` | `qa/d-report.json` |
| 转账 `categoryId=0` 语义未变 | **PASS** | `D-CAT-06`：`#fCatRow.hidden=true`；POST **200**，请求体 `txType=3`、`categoryId=0`、`toAccountId=2`；API 回读命中该转账=true | `qa/d-report.json`、`qa/screenshots/10-category-transfer.png` |
| 提交前校验 `categoryId`（源码层） | **PASS（经 diff 审计 + 上述正向用例间接验证）** | `app.js` L596 由 `!body.accountId` 收紧为 `!body.accountId \|\| !body.categoryId`，toast「请选择账户与分类」；本轮 D 套件未单独注入"无分类"负向用例，故此项以源码 diff + 分类下拉始终有合法值（`refillCategories` 保证）为依据，**未做独立的空分类提交负向测试**，如实标注 | `qa/scope-audit.md` 第 3 节 |

---

## 3. 成员邀请（`renderMembers` 角色门控 + 失败关闭）

| 验收项 | 判定 | 实际观测值（QA 本次） | 证据 |
|---|---|---|---|
| owner 可见且可打开弹窗 | **PASS** | `D-MEM-01`：role=owner，按钮数 **1**，点击后 `#investDlg` 可见 **true**，成员列表 3 行；`A09` 独立观测 owner 邀请按钮=1 | `qa/d-report.json`、`qa/a-qa.log`、`qa/screenshots/11-members-owner-invite.png` |
| admin 可见且可打开弹窗 | **PASS** | `D-MEM-02`：role=admin，按钮数 **1**，`#investDlg` 可见 **true**，成员列表 3 行；`A09` 独立观测 admin 邀请按钮=1 | `qa/d-report.json`、`qa/screenshots/12-members-admin.png` |
| member 看不到入口（count=0 且 DOM 无文本） | **PASS** | `D-MEM-03`：role=member，按钮数 **0**；`#view` 文本含「邀请成员」=**false**；`body` 文本含=**false**。`A08` 独立观测 member 按钮数=0；`A09` 独立观测 member 邀请按钮=0 | `qa/d-report.json`、`qa/a-qa.log`、`qa/screenshots/13-members-member-no-invite.png` |
| member 直接 POST invite 仍 403 | **PASS** | `D-MEM-04`：`api.raw POST /api/v1/books/1/members/invite`（member token, `{role:3}`）→ **HTTP 403**；`A08` 独立观测 `POST /members/invite → 403`、`DELETE /books → 403` | `qa/d-report.json`、`qa/a-qa.log`、`qa/scope-audit.md` 第 3.1 节 |
| `/users/me` 失败时失败关闭 | **PASS** | `D-MEM-06`：route 注入 `/users/me` → HTTP 500，owner 页面邀请按钮数 **0**、`#view` 无该文本，成员列表仍渲染 3 行 | `qa/d-report.json`、`qa/screenshots/14-members-me-failed-closed.png` |
| 成员列表 / 角色徽标 / 其它成员管理功能不受影响 | **PASS** | `D-MEM-05`：成员列表 **3 行**（API 3 条），角色列 `["所有者","管理员","普通成员"]`，「移除成员」按钮 2 个；`A09` 三角色会话均 `成员数=3` | `qa/d-report.json`、`qa/a-qa.log` |
| 后端 403 校验必须保留 | **PASS** | Go 后端零改动；前端仅门控 UI 入口；实测见上 | `qa/scope-audit.md` 第 2、3.1 节 |

---

## 4. 新增账户（第 4 项：业务 JS 无缺陷，I-ACCT-06 保持 FAIL）

| 验收项 | 判定 | 实际观测值（QA 本次） | 证据 |
|---|---|---|---|
| 稳定复现旧问题 | **PASS（复现成立）** | `baseline/account-repro.md` 记录 **3 次独立迭代**（`复现账户-1/2/3`），每次：POST **200**、API 回读含名称=true、`viewTextContentHasName=true`、`viewInnerTextHasName=**false**`、首列 `<td>` 标记 `[不可见]`、刷新后仍不可见、console error/pageerror 均无。QA 另独立核验 `server/web/dist/style.css` **L331**：`@media (max-width: 640px) { th:nth-child(1), td:nth-child(1), … { display: none; } }` —— 390px 视口必然命中 | `baseline/account-repro.md`、`qa/scope-audit.md` 第 3.3 节、`qa/screenshots/15-accounts-list-first-col-hidden.png` |
| POST 2xx | **PASS** | `D-ACCT-01` POST **200**；`I-ACCT-06` 观测 POST **200** | `qa/d-report.json`、`qa/c-report.json` |
| 行数 +1 / `td.textContent` 命中 | **PASS** | `D-ACCT-01`：`tbody 3→4`（`waitForFunction` 确定性重绘信号=true），DOM 首列 `textContent` 命中=true；`I-ACCT-06` 重绘+1=true、DOM 首列 textContent 命中=true | 同上 |
| API 回读 / 刷新后仍存在 / 无重复 | **PASS** | `D-ACCT-02` 刷新后 DOM 首列 4 项含该名称、API 4 项含该名称；`D-ACCT-03` 账户总数 3→4（+1），该名称在 API 出现 **1** 次、DOM 首列出现 **1** 次（无重复）；`D-ACCT-04` DOM 首列集合与 API 名称集合**完全一致**；`I-ACCT-06` 观测 `重复数=1` | `qa/d-report.json`、`qa/c-report.json` |
| **390px 视觉可见性是否闭环** | **FAIL（已知，保持 FAIL）** | `visibleInnerText=**false**`、`firstTdDisplay=**none**`（C 与 D 两套均如此）。截图 `15-accounts-list-first-col-hidden.png` 直观显示账户表表头仅「类型 / 余额 / 操作」三列，账户名称列被隐藏。 | `qa/c-report.json`、`qa/c-qa.log`、`qa/screenshots/15-accounts-list-first-col-hidden.png`、`16-acct-06-visibility-fail.png` |

> ### 明确结论（按 team-lead 裁决）
> **「390px 视觉可见性未闭环，`I-ACCT-06` 保持 FAIL」**。本轮 QA 未修改 `server/web/dist/style.css`（未获 Owner 授权），未把该 FAIL 改判为 PASS，也未用 `td.textContent` 替换 `innerText` 去"制造通过"——`I-ACCT-06` 在 `c-controls.js` 中是**无条件 `bad()`**，且 detail 与 `C/I-ACCT/visibility-observations.json` 中完整保留 `visibleInnerText=false` / `firstTdDisplay=none` 两个原始观测。
>
> D 套件新增的 `D-ACCT-01..04` 是**另立口径的新增用例**（仅断言 DOM/API 数据一致性），用例名与每条 detail 都显式标注「DOM 渲染一致性，不含视觉可见性」，并附「视觉可见性验收项未闭环，待 Owner 裁决，本轮不改 CSS」。**它们是新增用例，不是对既有 FAIL 用例的改判**；`C` 报告中 FAIL 仍为 5 条，其中 `I-ACCT-06` 在列。
>
> 因此「没有任何换口径制造 PASS」这一结论成立，但附带一个如实标注的边界：`D-ACCT-*` 的 4 条 PASS **不可**被解读为 `I-ACCT-06` 通过，也不可用于宣称 390px 下用户可见。

---

## 5. 特别核对（任务书点名项）

| 核对项 | QA 观测 | 判定 |
|---|---|---|
| `I-ACCT-06` 在 `c-controls.js` 中是否**无条件 FAIL** | 全文仅 L383 一处赋值 `R['I-ACCT-06'] = bad(...)`，无 if/三元、无 PASS 分支；`bad = (detail, shots=[]) => ({status:'FAIL',…})` 固定返回 FAIL | **是** ✓ |
| detail 是否含 `visibleInnerText` 与 `firstTdDisplay` 两个原始观测 | C 日志 FAIL 明细原文：`visibleInnerText=false；firstTdDisplay=none`；`C/I-ACCT/visibility-observations.json` 亦写入两字段 | **是** ✓ |
| C 报告里 FAIL 是否为 5 | `qa/c-qa.log` L7 `C 汇总：{"PASS":163,"FAIL":5,"BLOCKED":59,"N/A":1}`；`qa/c-report.json` `summary` 同值。5 条为：`I-LOGIN-04`、`I-LOGIN-11`、`I-LOGIN-12`、`I-HOME-01`、`I-ACCT-06` | **是（5 条）** ✓ |
| C 退出码 | `qa/c-qa.log` 末尾 `=== EXIT_CODE=1 ===` | 符合 `run.js` 规则 ✓ |
| 是否用延长 sleep 制造通过 | `c-controls.js` 删除了旧 `sleep(1300)+sleep(800)`，改为 `waitForResponse(POST)` + `waitForFunction(行数===rowsBefore+1)`；`txns-controls.js` 统一用 `settled()` 等骨架屏消失 | **否** ✓ |

---

## 6. 与工程师结果的差异（不替它圆场）

| 套件 | 工程师自述结果 | QA 独立重跑结果 | 差异 |
|---|---|---|---|
| A | 11/11 PASS（退出码 0） | 11/11 PASS，`EXIT_CODE=0` | **无差异** |
| D | 39/39 PASS（退出码 0） | 39/39 PASS，`EXIT_CODE=0` | **无差异** |
| C | PASS163/FAIL5/BLOCKED59/N/A1（退出码 1） | PASS163/FAIL5/BLOCKED59/N/A1，`EXIT_CODE=1` | **无差异** |

- 三套件的计数、状态分布、退出码与工程师结论**逐项一致**；且 5 条 FAIL 的 ID 也与工程师 `after/c-after-report.json` 所列一致（QA 以自己 `qa/c-report.json` 为准核对）。
- 但 QA 额外发现 3 项工程师报告未提及的**证据质量 / 边界问题**，详见 `qa/assertion-review.md` 与 `qa/qa-report.md`：① 框架截图与判定时点系统性错配；② `a-scenarios.js` A08 detail 存在修复前遗留的硬编码陈旧文案；③ `trace.zip` 含真实 JWT，Public 仓库不可直接复制。这些不改变任何 PASS/FAIL 判定。
- **收尾状态更新（team-lead 授权的低风险收尾后）**：Q-2（A08 文案）与 Q-4（workflow job `name`）已按授权做纯文案修正；Q-3 脱敏口径保持（仍不复制 trace）；Q-1 按 team-lead 指示登记为**未解决的证据质量问题**、不改 `step()` 框架，并另补清晰失败态截图 `qa/screenshots/05b-txns-load-failure-clear.png`。**未重跑 A/D/C 全套**，三份日志与 report.json 产生于纯文案修改之前；因判定逻辑与 status 计算未变，本矩阵所有结论不受影响。详见 `qa/qa-report.md` 4.2.1、6.5。

---

## 7. 本矩阵引用的证据文件

- 日志与退出码：`qa/build.log`、`qa/a-qa.log`、`qa/d-qa.log`、`qa/c-qa.log`
- 报告：`qa/a-report.json`、`qa/d-report.json`、`qa/c-report.json`
- 截图：`qa/screenshots/01..17`（清单与脱敏说明见 `qa/qa-report.md` 第 5 节、`qa/SHA256SUMS.txt`）
- 范围审计：`qa/scope-audit.md`；断言抽检：`qa/assertion-review.md`
- 修复前基线（工程师产出，QA 作为**背景材料**引用而非结论依据）：`../baseline/account-repro.md`、`../after/negative-check.md`
