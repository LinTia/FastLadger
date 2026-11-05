# QA 独立复核 · 断言有效性抽检与负向合成验证核对（D）

> 抽检对象：`tools/r0/e2e/specs/d-defects.js`、`tools/r0/e2e/specs/txns-controls.js`、`tools/r0/e2e/specs/c-controls.js`（I-ACCT-06 段）、`tools/r0/e2e/scripts/negative-check.js`
> 判定口径：PASS 必须由**可复现的运行时观测**（DOM 文本 / 网络响应码 / 请求体 / API 回读）推出；凡「硬编码 PASS」「只检查控件存在」「只检查耗时」「截图存在即 PASS」均视为假阳性。
> 本文件只做静态代码审查 + 与 QA 自己重跑出的 `report.json` 原始观测交叉比对，不采信工程师文字结论。

---

## 1. 抽检：13 条 PASS 用例及其依赖的具体运行时观测

### 1.1 `d-defects.js`（7 条）

| # | 用例 | 断言成立的**具体运行时观测** | 假阳性排查 |
|---|---|---|---|
| 1 | `D-TXNS-01` 交易总表有数据 | `page.evaluate` 读 `#view.childElementCount=1`、`h2` 标题含「交易」、`.filters select` 数量=3、`tbody tr` 行数≥1、**备注列 `td[5].textContent` 至少一项非空**；另 `api.transactions` 回读 `items.length>=1`。QA 实测值：`children=1 / rows=3 / notes=["工资","打车","早餐"] / api=3` | 不是"只检查控件存在"——同时要求真实数据行、备注文本与 API 数量；`pass` 是 6 个观测的合取，非硬编码 |
| 2 | `D-TXNS-03` 请求失败 | `page.route` 注入 **HTTP 500** → `#view.innerText` 同时含「交易加载失败」与「D套件注入500」；`#view [aria-busy="true"]` count **=0**；`#view .skel` count **=0**；`#toast.innerText` 含注入文本。QA 实测：`busy=0 / skel=0 / toast=✕ D套件注入500` | 有真实网络故障注入 + 4 个独立 DOM/文本观测；截图失败不影响判定（`step()` 内 `.catch()` 只作用于截图） |
| 3 | `D-CAT-01` 支出下拉隔离 | `catOptions()` 读 `#fCategory` 全部 `option.value` → 排序后与 `api.categories(...).filter(c=>c.catType===2).map(id)` **JSON 字符串全等**；并检查 `leaked`（混入收入类 id 数）=0。QA 实测：下拉 22 项 = API 22 项，混入 0 | 集合全等比较，不是"有下拉就 PASS"；依赖 API 回读的真实分类类型，非硬编码 id 列表 |
| 4 | `D-CAT-04` 支出提交 + API 一致 | `capturePost()` 用 `page.waitForRequest` + `page.waitForResponse` 捕获**真实 POST**，判定 `status∈[200,300)`、请求体 `txType===2` 且 `categoryId` 属于支出类集合；再 `api.transactions` 回读命中该备注且 `txType/categoryId` 一致。QA 实测：`POST=200 / body.txType=2 / body.categoryId=1 / API 命中` | 三重观测（响应码 + 请求体 + API 回读），不是"提交按钮点了就 PASS" |
| 5 | `D-MEM-04` member 直连 invite 仍 403 | `api.raw(base,'POST', '/api/v1/books/{id}/members/invite', {token: member.token, body:{role:3}})` → 判定 `status === 403`。QA 实测 `HTTP 403` | 真实 HTTP 响应码；绕过 UI 直接打后端，能识破"只把按钮藏起来而后端放开"的假修复 |
| 6 | `D-MEM-06` `/users/me` 失败关闭 | `page.route(/\/api\/v1\/users\/me$/)` 注入 500 → owner 页面按钮数 **=0**、`#view` 文本不含「邀请成员」、成员列表仍 **≥3 行**（证明没有整页崩掉）。QA 实测：`count=0 / rows=3` | 故障注入 + 正反两向观测（入口消失、列表仍在），不是"页面没报错就 PASS" |
| 7 | `D-ACCT-01` 新增账户数据入列 | `waitForResponse(POST /accounts)` → `status∈[200,300)`；`waitForFunction(tbody tr 行数 === rowsBefore+1)` → 确定性重绘；`td:first-child` 的 `textContent` 命中名称；`api.accounts` 回读命中。QA 实测：`POST=200 / 3→4 / inDom=true / inApi=true` | 4 重观测；**明确不含视觉可见性**，且每条 D-ACCT 用例都附 `visibleInnerText=false` / `firstTdDisplay=none` 原始观测，未把 FAIL 改判 |

### 1.2 `txns-controls.js`（6 条，C/D 共用同一实现）

| # | 用例 | 断言成立的**具体运行时观测** | 假阳性排查 |
|---|---|---|---|
| 8 | `I-TXNS-01` 关键词搜索 | `query()` 捕获**真实 GET /transactions 响应** `status===200` + URL 含 `keyword=`；`tableRows()` 读 tbody 全部行，要求命中行≥1 且**每行备注都含关键词**。QA 实测：`HTTP 200 / 命中 1 行「工资」` | 不是"有搜索框就 PASS"；同时校验请求参数与返回行内容 |
| 9 | `I-TXNS-10` 全选本页 | 真实 `checkbox.check()` → `tableRows()` 读每行 `checked===true` 且数量等于数据行数；`.bulkbar span.muted.innerText` 匹配「已选 N 项」；「删除选中」按钮 `isDisabled()===false`。QA 实测：`3 行全部 checked / 已选 3 项 / disabled=false` | 状态 + 文案 + 按钮态三重观测 |
| 10 | `I-TXNS-12` 删除选中 | 先用 `api.createTx` 造专用交易 → 勾选 → 确认框点「确定」→ `waitForRequest(DELETE /transactions/{txId})` 捕获真实删除请求 → `api.transactions` 回读确认该笔已消失 → `api.recycle` 回读确认该笔进入回收站。QA 实测：`DELETE 已发出 / gone=true / inRecycle=true` | 端到端数据链路闭环（创建→删除→列表消失→回收站出现），不是"点了删除就 PASS" |
| 11 | `I-TXNS-13` 删除取消（负向） | `page.on('request')` 统计 DELETE 请求数 === **0**；`api.transactions` 前后条数 `nBefore===nAfter`。QA 实测：`DELETE 请求数=0 / 3→3` | 典型负向断言，能识破"取消也误删"；不是靠耗时或截图 |
| 12 | `I-TXNS-17` 骨架屏 | route 延迟 700ms 制造加载窗口 → `waitForSelector('#view [aria-busy="true"]')` 观测到骨架屏、`.skel` 计数 **=5**、额外截图；响应后 `aria-busy` 残留 **=0**、`tbody` 行数≥1。QA 实测：`skel=5 / busyAfter=0 / rows=3` | 700ms 延迟在源码注释中明写「仅用于制造可观测的加载中窗口，**不参与任何 PASS 判定**」——**无"只检查耗时"** |
| 13 | `I-TXNS-19` 加载失败 | route 注入 **HTTP 500** → `#view.innerText` 含「交易加载失败」与「E2E 注入的服务器错误」；`aria-busy` count **=0**；`#toast` 含注入文本。QA 实测：`可见反馈=「交易加载失败：E2E 注入的服务器错误（可重新查询或刷新页面）」/ busy=0 / toast=✕ E2E 注入的服务器错误` | 与 `D-TXNS-03` 独立实现、独立注入文本，两套各自 PASS，互为交叉验证 |

### 1.3 反假阳性的结构性保障（源码级）

- **`ok()` 不是硬编码**：`d-defects.js` L22 / `txns-controls.js` L17 的 `ok(detail)` 只在 `pass` 布尔为真时被调用；`pass` 全部由上述观测值计算得出。全文件无 `status:'PASS'` 字面量直接塞进结果的地方（`results.push` 用的是 `r.status`）。
- **异常一律 FAIL**：`step()`（`d-defects.js` L94-104、`txns-controls.js` L103-118）在 `fn()` 抛异常时 `r = bad('执行异常：…')`；分组级 `catch` 也 push `status:'FAIL'`，不存在"出错就跳过"。
- **截图不构成 PASS 条件**：截图在 `step()` 中 `try/catch` 且失败被忽略，只作为 `evidence` 附加；`runDefects` 的 `step` 里 `runDir.screenshot(...).catch(()=>{})`，`txns-controls.js` 的 `step` 里 `try{…}catch(_){}`。**不存在「截图存在即 PASS」**。
- **无「只检查控件存在」的孤立判定**：凡 count 类观测（按钮数、下拉数、行数）都与第二/第三观测合取（文本、响应码、API 回读）。唯一较接近的是 `I-TXNS-14`（`opened && closed && catCount>0`），但它验收的本来就是「控件能否打开/关闭」这一交互本身，且有真实 `dialog.open` 状态读取，属合理。
- **无「只检查耗时」**：全仓 grep 的耗时字段 `ms` 只出现在 `run.js` 的结果行元数据，不参与任何 `pass` 表达式。
- **确定性等待替代固定 sleep**：`I-ACCT-06` 删除了旧 `sleep(1300)+sleep(800)`，改为 `waitForResponse` + `waitForFunction(行数+1)`；`txns-controls.js` 用 `settled()` 等 `#view [aria-busy="true"]` 消失。**没有通过延长 sleep 制造通过**。

> **抽检结论：13 条 PASS 用例全部依赖真实运行时观测（DOM 文本 / 网络响应码 / 请求体 / API 回读），未发现硬编码 PASS、只检查控件存在、只检查耗时或截图存在即 PASS。断言有效性合格。**

---

## 2. 负向合成验证 `after/negative-check.md` 核对

### 2.1 是否真的用「修复前 app.js」在网络层替换

| 核对项 | QA 核验结果 | 判定 |
|---|---|---|
| 取的是修复前版本 | `scripts/negative-check.js` L26：`execFileSync('git', ['show','HEAD:server/web/dist/app.js'])`。QA 已独立确认 `HEAD === main === dadc9a3b9…`，且修复全在工作树未提交，因此 `HEAD:app.js` 确为修复前版本 | **是** ✓ |
| 有防拿错版本的前置校验 | L29-34：要求旧脚本**含** `fillSelect($("#fCategory"), state.categories);` 且**不含** `view.append(card); // 与 renderHome`，否则抛错 | **有** ✓ |
| 网络层替换、不动磁盘 | L41-45：`page.route(/\/app\.js$/, route => route.fulfill({status:200, contentType:'application/javascript', body: oldJs}))`；脚本内无任何写 `server/web/dist/app.js` 的代码；Go 后端/API/DB 未动 | **是** ✓ |
| 确实跑过（非纸面推演） | `after/negative-check.json`：`startedAt=2026-11-05T09:42:41.727Z`、`finishedAt=…09:42:54.594Z`（约 13 秒）、`gitSha=dadc9a3b9…`、`injectedAppJsSha256=32d853df…`、`checks` 4 组、`fatal=(none)`；`after/negative-check.log` 有 `[negative-check] output -> …` 且末尾 `=== EXIT_CODE=0 ===` | **确实执行且无致命异常** ✓ |
| 观测值自洽 | NEG-1 `viewChildren=0 / hasTable=false`；NEG-2 支出下拉 30 项、首项「工资」、`incomeIdsInExpense=8`、`unchangedOnTypeSwitch=true`；NEG-3 `inviteBtnCount=1`；NEG-4 `postStatus=200 / innerTextHasName=false / textContentHasName=true / firstColDisplay=none`。与 QA 自己重跑的修复后值（22/8 项、首项「餐饮」、按钮 0、`firstTdDisplay=none`）形成清晰对照 | **自洽** ✓ |

### 2.2 哪些用例在修复前会 FAIL（按 NEG 分组）

| NEG | 对应用例 | 修复前观测 → 会 FAIL 的原因 |
|---|---|---|
| NEG-1 | `D-TXNS-01`、`I-TXNS-01..19` | `#view` 为空（children=0、无 table），`D-TXNS-01` 的 `children>=1 / rows>=1` 必然 FAIL；交易总表无控件可交互，C 组 19 条无可断言目标（修复前实际是 01 FAIL + 02..19 BLOCKED） |
| NEG-2 | `D-CAT-01`、`D-CAT-02`、`D-CAT-03`、`D-CAT-04` | 支出下拉混入 8 个收入类 id（30 项、首项「工资」），集合全等断言 FAIL；切换类型下拉不变 → 重置断言 FAIL；默认选中收入类提交支出 → `D-CAT-04` 的 `categoryId∈支出类` 断言 FAIL |
| NEG-3 | `D-MEM-03` | member 身份按钮数=1 且 `#view` 含「邀请成员」，`count===0 && 文本不含` 断言 FAIL |
| NEG-4 | **旧 C 的 `I-ACCT-06`** | `innerTextHasName=false` / `firstColDisplay=none`，旧 C 用 `waitText(body.innerText)` 恒 FAIL。**注意：`D-ACCT-01..04` 在修复前不会 FAIL** —— 详见 2.3 |

### 2.3 如实指出的漏洞与局限（不替它圆场）

1. **它是「合成观测 + 文字推断」，没有真的在修复前 app.js 下运行 D 套件或 C 套件**。脚本只采集 4 组原始观测值，再用 `preFixFailsBecause` 字符串**推断**哪些用例会 FAIL；没有逐条跑 `I-TXNS-01..19`、没有真的提交一次支出交易复现 400。因此「用例级 FAIL」是**合理推断而非实测**。定位上属于负向合成验证（synthetic），不是完整回归。

2. **结论句对 NEG-4 不准确（真实逻辑漏洞）**。`after/negative-check.md` 的收尾句写「在修复前实现下，上述 4 组观测值分别使对应用例 FAIL」，但 NEG-4 对应的 `D-ACCT-01..04` 在修复前 app.js 下**同样会 PASS**——因为第 4 项业务 JS 零改动，NEG-4 自己记录的就是 `POST=200 / textContentHasName=true / rows=4`，恰好是 `D-ACCT-01` 判定所需的全部条件。NEG-4 真正说明的是「旧 C 的 `I-ACCT-06` 因 `innerText` 口径恒 FAIL，且业务代码无需改动」。脚本内 `preFixFailsBecause` 文案其实写对了（「本项证明第 4 项属测试口径问题、业务代码无需改动」），但 md 的总结句把 4 组一并说成"使对应用例 FAIL"，**与 NEG-4 的实际观测矛盾**。建议 Owner 要求修正该总结句措辞，避免被误读为「D-ACCT-01..04 在修复前也失败」。

3. **NEG-1 的措辞偏笼统**：修复前 `I-TXNS-02..19` 是 **BLOCKED**（控件不存在）而非 FAIL；md 中「I-TXNS-01..19 无控件可交互」的描述是准确的，但总结句统一说「FAIL」不够精确。

4. **浏览器可追溯性不足**：`negative-check.js` 的 `launch()` 未显式固定 `R0_BROWSER_CHANNEL`，也未把浏览器版本/channel 写进 `negative-check.json`；md 声称「真实 msedge；390×844」，日志只有一行输出、无 `BROWSER …` 行。QA 只能**间接**推断（本机无 bundled chromium，而 `fatal=(none)` 说明启动成功），无法从证据文件直接复核浏览器通道。建议补记环境信息。

5. **不影响结论的部分**：上述 4 点都不推翻「修复前 3 项业务缺陷真实存在、D 套件断言非恒真」这一核心结论——NEG-1/2/3 的对照观测与 QA 独立重跑的修复后值形成完整反差，且 `qa/scope-audit.md` 已从 diff 层独立确认 3 处源码修复。

---

## 3. QA 额外发现的证据质量问题（本轮新增，均不改变任何 PASS/FAIL 判定）

| # | 问题 | 观测依据 | 影响 |
|---|---|---|---|
| Q-1 | **框架截图与判定时点系统性错配** | `d-defects.js` `step()`（L94-104）与 `txns-controls.js` `step()`（L103-118）都在 `fn()` **执行完之后**才截图。凡 fn 内会复位/关闭弹窗/刷新页面的用例，截图不反映判定瞬间。实例：`D-TXNS-03.png` 与 `D-TXNS-01.png` **字节级相同**（均为 reload 恢复后的正常列表，ViewImage 提示 identical）；**`qa/SHA256SUMS.txt` 给出硬证据：`01-txns-with-data.png`、`03-txns-failure-screenshot-mismatch.png`、`04-txns-row-edit.png` 三张 SHA256 同为 `6324b0fa854f53488db8645b9c7affed8c0e89304c74185b52e2d343124af52b`**，即「有数据 / 加载失败 / 行编辑」三个不同用例产出了同一张图；`I-TXNS-14.png` 是 `#fCancel` 关闭后的列表而非弹窗；`I-TXNS-18.png` 是 `reset()` 后的正常列表而非空态；`I-TXNS-19.png` / `I-ACCT-06-visibility.png` / `D-ACCT-01.png` 呈极淡或全白。`D-ACCT-02.png` 与 A09 成员页截图则清晰 | **【未解决 · 按 team-lead 裁决如实登记】** 判定本身**不依赖截图**，已二次源码复核：`d-defects.js` L94-104 / `txns-controls.js` L103-118 中 `r.status` 在 `fn()` 返回时即确定，截图在其后执行且失败被 `catch` 忽略、仅作 `evidence` 附加（`d-defects.js` L103 与 `txns-controls.js` L117 均直接用 `r.status`），故 PASS/FAIL 结论成立。按指示**不改 `step()` 框架**；已另补清晰失败态截图 `qa/screenshots/05b-txns-load-failure-clear.png`（见 `qa/qa-report.md` 4.2.1）。建议后续把截图移到判定采集点 |
| Q-2 | **`a-scenarios.js` A08 detail 有修复前遗留的硬编码陈旧文案** | L256：`…成员页「邀请成员」按钮数=${inviteBtn}（UI 未隐藏=真实缺陷，已记录）…`。QA 本次重跑 A08 实测 `inviteBtn=0`，却仍输出「UI 未隐藏=真实缺陷」，语义与现状矛盾。该文案是硬编码字符串，**不参与判定**（`status` 由 `denied.status===403 && deniedInvite.status===403` 决定） | **【已修复】** 按 team-lead 授权，仅把该 detail 文案改为「成员管理接口 403 校验通过；UI 入口可见性另由 D 套件 D-MEM-* 覆盖」；`git diff` = 1 行，`status: apiOk ? 'PASS' : 'FAIL'`、`apiOk`、`evidence` 均未动，`node --check` 通过。属纯文案修正，非断言缺陷 |
| Q-3 | **`trace.zip` 含真实 JWT，Public 仓库不可直接复制** | QA 解压 `artifacts/r0-qa-verify/…/A01/trace.zip` 后 `grep -a` 在 `trace.network` 中命中 JWT 三段式字面量（`[REDACTED]`——凭据原文按脱敏要求不转录到证据文件） | 任务书 E 要求「若框架已生成 trace，也一并复制」，但同一任务书明确「不要复制任何含 token/凭据的原始网络日志（仓库是 Public，必须脱敏）」。**QA 以脱敏要求为最高优先级，决定不复制任何 trace.zip**，并在此说明原因。参考 `tools/r0/redaction.js` 的脱敏口径（Authorization/Cookie/JWT/Bearer 均需 `[REDACTED]`）。QA 已扫描 `qa/` 全部落盘文件，无 JWT/Bearer/token 残留 |
| Q-4 | workflow job 名未同步含 D | `.github/workflows/r0-e2e.yml` L28 仍为 `A(11 场景) + C(228 控件) + B(分类审计)` | **【已修复】** 按 team-lead 授权，job `name` 改为 `A(11 场景) + D(缺陷回归) + C(228 控件) + B(分类审计)`；`git diff` 确认仅 `name` 一行，step / id / 触发条件 / `continue-on-error` 未动。D step 本身存在且为硬门禁（`qa/scope-audit.md` 第 4 节） |

---

## 4. 本节结论

1. **断言有效性抽检合格**：13 条 PASS 用例全部有真实运行时观测支撑，无硬编码 PASS / 只查控件存在 / 只查耗时 / 截图即 PASS；异常一律 FAIL；确定性等待替代了固定 sleep。
2. **负向合成验证确实用修复前 `HEAD:app.js` 在网络层替换并真实执行过**（JSON 时间戳、SHA256、4 组观测、无 fatal、日志退出码 0 均可核），NEG-1/2/3 有效支撑「3 项业务缺陷在修复前真实存在」。
3. **但它有三处如实标注的局限**：未做用例级 FAIL 实测（仅推断）；md 总结句对 NEG-4 的表述与自身观测矛盾（D-ACCT-01..04 在修复前也会 PASS）；浏览器通道未记入证据。
4. **QA 新增发现 4 项证据质量/边界问题**（Q-1 截图时点错配、Q-2 A08 陈旧文案、Q-3 trace 含 JWT 故不复制、Q-4 job 名滞后），均**不改变** A/D/C 的任何判定。收尾状态：**Q-1 未解决**（按 team-lead 指示登记、不改框架）、**Q-2 / Q-4 已修复**（仅纯文案）、**Q-3 保持脱敏口径**（不复制 trace），详见 `qa/qa-report.md` 6.5。
5. **初次独立复核阶段**：QA 未修改任何测试代码、业务代码、CSS 或 workflow，`tools/r0/e2e/` 与 `server/web/dist/app.js` 保持工程师交付原样，以确保独立重跑基线可复现。**后续按 team-lead 明确授权的低风险收尾**：仅两处纯文案修改 + 新增一个证据采集脚本；业务代码、CSS、Go、断言逻辑、status 计算、`step()` 框架始终未动；未重跑全套（按要求），`a-qa.log` 等日志产生于文案修改前，已在 `qa-report.md` 6.5 如实标注时间差。
