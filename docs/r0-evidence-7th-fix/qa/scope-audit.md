# QA 独立复核 · 范围审计（A）

> 复核人：Edward（QA 工程师，独立复核方）
> 复核对象：`fix/r0-sixth-runtime-defects` 分支工作树（工程师第六次修复产出）
> 基准 main：`dadc9a3b9c567cad2bbad7e47361fb774365b949`
> 本文件只记录 **QA 自己重新执行 git 命令与逐行阅读 diff** 得到的观测，不采信工程师自述结论。
> 采集时间：2026-11-05T09:59Z ~ 10:00Z（UTC）

---

## 1. 仓库状态（原始命令输出）

```
$ git rev-parse HEAD
dadc9a3b9c567cad2bbad7e47361fb774365b949
$ git branch --show-current
fix/r0-sixth-runtime-defects
$ git diff --name-only main...HEAD
(空)
$ git status --porcelain
 M .github/workflows/r0-e2e.yml
 M server/web/dist/app.js
 M tools/r0/e2e/run.js
 M tools/r0/e2e/specs/c-controls.js
?? docs/r0-evidence-7th-fix/
?? tools/r0/e2e/scripts/baseline-probe.js
?? tools/r0/e2e/scripts/negative-check.js
?? tools/r0/e2e/specs/d-defects.js
?? tools/r0/e2e/specs/txns-controls.js
```

**关键事实：`HEAD` 与基准 `main` 是同一个提交**（`dadc9a3b9...`）。因此 `git diff --name-only main...HEAD` 为空是**预期结果**，不代表"没有改动"——工程师本轮全部产出都处于**工作树未提交（uncommitted / untracked）**状态。本次审计与重跑的对象即这份工作树，审计口径以 `git diff`（工作树 vs index/HEAD）为准。

`git diff --stat`（工作树）：

```
 .github/workflows/r0-e2e.yml     | 13 +++++-
 server/web/dist/app.js           | 52 +++++++++++++++++----
 tools/r0/e2e/run.js              | 11 ++++-
 tools/r0/e2e/specs/c-controls.js | 99 ++++++++++++++++++++++++++++--------
 4 files changed, 145 insertions(+), 30 deletions(-)
```

---

## 2. 变更文件清单与授权判定

| # | 文件 | 状态 | 改动归属 | 是否在授权范围 | 说明 |
|---|---|---|---|---|---|
| 1 | `.github/workflows/r0-e2e.yml` | M | 工程师 | **需 team-lead 确认（见下方 2.1）** | 新增「包 D 门禁」step + step summary 增补；QA 只读未改 |
| 2 | `server/web/dist/app.js` | M | 工程师 | 是（本轮 4 项修复中的 3 项） | 业务代码，QA **未修改、也未获授权修改** |
| 3 | `tools/r0/e2e/run.js` | M | 工程师 | 是（测试代码） | 新增 `--suite d` 分支；`--suite all` 仍为 `['a','c']` |
| 4 | `tools/r0/e2e/specs/c-controls.js` | M | 工程师 | 是（测试代码） | `groupTxns` 改真实逐条执行；`I-ACCT-06` 收紧为无条件 FAIL |
| 5 | `tools/r0/e2e/specs/d-defects.js` | ?? | 工程师 | 是（新增测试代码） | 39 条 D 套件 |
| 6 | `tools/r0/e2e/specs/txns-controls.js` | ?? | 工程师 | 是（新增测试代码） | I-TXNS-01..19 共用配方 |
| 7 | `tools/r0/e2e/scripts/baseline-probe.js` | ?? | 工程师 | 是（新增测试代码） | 修复前基线复现 |
| 8 | `tools/r0/e2e/scripts/negative-check.js` | ?? | 工程师 | 是（新增测试代码） | 负向合成验证 |
| 9 | `docs/r0-evidence-7th-fix/**` | ?? | 工程师 | 是（本轮授权的新证据目录，非历史证据） | baseline/、after/、root-cause-and-plan.md |

### 2.1 关于 `.github/workflows/r0-e2e.yml` 的边界说明（如实标注，不替它圆场）

任务书给我的硬约束字面包含「不得修改 `.github/workflows/`」，但同一任务书第 A 节又明确要求「读 `git diff -- .github/workflows/r0-e2e.yml`，确认新增的『包 D 门禁』step 只有一份」，说明该文件的改动是 team-lead 已知并作为**被审计对象**看待的（属"新增 D 套件"的配套门禁）。

- 该改动是**工程师**做的，**我（QA）没有修改**，本轮我也不会修改它；
- 从内容看它是本轮 D 套件的必要门禁配套（详见第 4 节），且 A/C 语义未变；
- 但严格按硬约束字面，它落在「不得修改」清单内。**我把它列为需 team-lead 确认的边界项**，不擅自判定"越界"，也不擅自判定"合规已闭环"。

### 2.2 越界检查（明确排除项）

| 排除项 | git status / diff 观测 | 结论 |
|---|---|---|
| `server/web/dist/style.css` | 未出现在 status，`git diff -- server/web/dist/style.css` 为空 | **未修改** ✓（L331 规则仅由 QA 只读核验） |
| Go 后端（任意 `.go` / `go.mod` / `go.sum`） | 无任何 .go 文件出现在变更清单 | **未修改** ✓ |
| `tools/r0/e2e/fixtures/controls.json`（控件清单/口径） | 未修改；且 `git diff --name-only main...HEAD` 为空 | **未修改** ✓ |
| PRD / WBS / DB / API 定义 | 无相关文件变更 | **未修改** ✓ |
| 历史证据 `docs/r0-evidence-6th/` 及更早 | `find docs -maxdepth 1` 中历史目录无 status 变更 | **未修改** ✓ |
| README / CHANGELOG | 无变更 | **未修改** ✓ |
| `.github/workflows/` 其它文件 | 仅 `r0-e2e.yml` | 见 2.1 |
| 提交/合并/push | `HEAD` 仍为 main 的提交、无新 commit | **未发生** ✓ |

**结论：除 2.1 的边界项外，无越界文件。全部改动集中在业务 JS（3 项修复）、E2E 测试代码、本轮新证据目录，以及 D 门禁 workflow。**

---

## 3. `git diff -- server/web/dist/app.js` 逐行审计

共 **6 个 diff hunk**（`git diff -- server/web/dist/app.js | grep -c '^@@'` = 6；`git diff --stat` = `43 insertions(+), 9 deletions(-)`）。下表按 **7 处改动点**分列，其中第 2 个 hunk 同时包含 `load()` 的 try/catch 与新增 `renderLoadError()` 函数，故表中拆为两行；表内「#」列是改动点序号，不是 diff hunk 序号。全部落在 4 处授权修复范围内，无第 5 处业务改动。

| 改动点 #（非 hunk 序号） | 位置 | 改动 | 对应授权项 | 判定 |
|---|---|---|---|---|
| 1 | `renderTxns()` L383 | `+ view.append(card);` | ① 恢复交易总表挂载 | 与 `renderHome/renderBooks/renderAccounts` 一致，最小单行修复 ✓ |
| 2 | `renderTxns.load()` L397-410 | `const data = await api(...)` 包入 `try/catch`，`catch` 内 `renderLoadError(tableWrap, e); throw e;` | ① I-TXNS-19 请求失败清骨架屏 + 静态失败文本 | 见下方"异常与 innerHTML"专项 ✓ |
| 3 | 新函数 `renderLoadError()` L413-421 | `wrap.replaceChildren()` → 创建 `.empty` div + `.icon`（`"⚠️"`）+ `<p>`，全部 `textContent` | ① 静态失败文本 | 无 `innerHTML`、无用户数据拼接、无新增交互 ✓ |
| 4 | `openTxForm()` L550-566 | 移除无过滤的 `fillSelect($("#fCategory"), state.categories)`；新增 `refillCategories()`（`catType === t` 过滤、`t===3` 早退、切换后 `value = 首个匹配项 id` 重置、空结果 `value="0"` + 占位文案）；`syncFormFields()` 调用它 | ② 分类过滤 + 切换重置 | 与编辑表单 `openTxEditForm` 口径一致；转账分支 `t===3` 直接 return，`categoryId:0` 语义未变 ✓ |
| 5 | `openTxForm()` 提交校验 L596 | `!body.accountId` → `!body.accountId \|\| !body.categoryId`，toast 改「请选择账户与分类」 | ② 提交前校验 `categoryId` | 防御性收紧，不绕过任何既有校验 ✓ |
| 6 | `renderMembers()` L1706-1716 | `row.append(h, invBtn)` → `row.append(h)`；`/users/me` 加 `.catch(() => null)`；`myRole` 由 `members.find(m => m.uid === me.id).role` 得出；**仅 `myRole===1 \|\| myRole===2`** 才 `row.append(invBtn)` | ③ 角色门控 + 失败关闭 | owner=1/admin=2 可见、member=3 不可见 ✓ |
| 7 | `renderMembers()` L1741 | `const myUid = me.id` → `me ? me.id : null` | ③ 失败关闭后避免空指针 | `me===null` 时不再渲染「（我）」标记，成员列表仍完整渲染 ✓ |

### 3.1 三项专项检查（任务书点名）

**① 没有 `innerHTML`**
- `grep -n "innerHTML" server/web/dist/app.js` 全文件仅 1 处命中：**第 2 行文件头注释**「安全口径：所有用户数据一律 textContent 渲染，禁止 innerHTML 拼接（防 XSS）」——是注释，不是代码。
- diff 的新增行全部使用 `createElement` / `textContent` / `replaceChildren` / `append`。
- **结论：新增代码与全文件均无实际 `innerHTML` 使用** ✓

**② 没有吞异常**
- `load()` 的 `catch` 末尾是 `throw e;`，异常继续向上传播（既有 toast 由调用方/全局处理负责），不是静默吞掉。
- `renderMembers()` 的 `await api("GET","/users/me").catch(() => null)` **确实捕获并丢弃了错误对象**，这里如实标注：它是**刻意的失败关闭（fail-closed）降级**——拿不到身份就不渲染邀请入口，符合 Owner 对第 3 项「`/users/me` 失败时失败关闭」的明确要求；且后续 `myRole=null` 分支使入口不渲染，不是"把失败伪装成成功"。**判定为合规的降级，非掩盖缺陷的吞异常**，但作为观测事实保留。
- **结论：无掩盖缺陷性质的吞异常** ✓

**③ 没有把 403 校验挪走**
- `app.js` 是前端，后端 Go 服务层零改动；`grep -n "403" app.js` 仅命中两处注释（L256 删除账本"服务端 403 兜底"、L1715 新增注释"后端 service 层 403 校验原样保留"）。
- 前端移除的只是**UI 入口的渲染**，不是权限校验本身；实测见 `acceptance-matrix.md` 的 D-MEM-04 / A08（真实 `POST /members/invite` → HTTP 403）。
- **结论：403 服务端校验保留** ✓

### 3.2 新增账户一项：业务 JS 零改动

`git diff -- server/web/dist/app.js` 中**没有任何**与 `renderAccounts` / `#acctName` / `#acctSubmit` / 账户表格相关的行。工程师「业务 JS 无缺陷」的结论在 diff 层面成立，运行层面证据见 `qa/acceptance-matrix.md` 与 `qa/assertion-review.md`。

### 3.3 骨架屏 / aria-busy 机制（为 I-TXNS-19 判定做源码确认）

- `renderSkeleton(wrap)`：`wrap.replaceChildren()` 后创建 `div` 并 `setAttribute("aria-busy","true")`，内含 5 个 `.skel`。
- `renderLoadError(wrap, err)`：同样 `wrap.replaceChildren()`，因此**带 `aria-busy` 的骨架屏元素被整体移除**，页面不会残留 `#view [aria-busy="true"]`。
- `renderTable(data)`：正常路径 `tableWrap.replaceChildren()` 也会清掉骨架屏。
- 这解释了 D-TXNS-03 / I-TXNS-19 中 `busy===0`、`skel===0` 的判定为何成立，而非靠等待超时"碰巧"通过。
- `fillSelect(sel, items, blankLabel)` 支持第三参数（`value="0"` + 占位 option），`refillCategories()` 的「暂无可选分类」分支调用合法 ✓

---

## 4. `git diff -- .github/workflows/r0-e2e.yml` 审计

| 检查项 | 观测 | 判定 |
|---|---|---|
| 「包 D 门禁」step 份数 | 全文仅 1 个 `- name: 包 D · 四项业务缺陷回归（门禁）`（L79-85），`id: d_defects`，`run: node run.js --suite d` | **只有一份** ✓ |
| D 是否为门禁 | 该 step **没有** `continue-on-error`；`run.js` 中任一 `FAIL` 会置 `process.exitCode=1`（`fails.length` 判断，见 `run.js` L135-140） | **门禁成立** ✓ |
| 既有 A step 语义 | L69-75 `包 A · 11 项状态/权限/同步场景` / `id: a_suite` / `run: node run.js --suite a`，diff 未触碰 | **未变** ✓ |
| C 是否仍为 `continue-on-error` | L91 `continue-on-error: true` 原样保留 | **仍是** ✓ |
| C 的 `run` 命令 | L96 `node run.js --suite c` 未变 | **未变** ✓ |
| step summary | 新增「包 D 退出码（四项缺陷回归门禁）」；C 那行标注「报告型，success 不代表无 FAIL」 | 语义更清晰，未弱化 A/C ✓ |
| `--suite all` 组合 | `run.js` L89 `const suites = args.suite === 'all' ? ['a','c'] : [args.suite];` 未变；D 必须显式 `--suite d` | **A+C 组合保持** ✓ |

workflow 的 job 名（L28 `A(11 场景) + C(228 控件) + B(分类审计)`）未同步更新为含 D，属**文案滞后**，不影响任何执行语义；作为低危观察记录，不构成本轮 FAIL。

---

## 5. `c-controls.js` 特别核对（I-ACCT-06 是否被改判）

| 检查项 | 观测 | 判定 |
|---|---|---|
| `bad()` 签名 | L23 `const bad = (detail, shots = []) => ({ status: 'FAIL', detail, shots });` | 支持证据附件 ✓ |
| `I-ACCT-06` 赋值次数 | 全文仅 L383 一处 `R['I-ACCT-06'] = bad(...)`（另两处 L359 是 name、L378 是截图名） | **唯一赋值** ✓ |
| 是否存在 PASS 分支 | 该赋值**无 if/三元**，直接 `bad(...)`；源码注释亦写明「收紧裁决：无条件 FAIL，不允许任何 PASS 分支」 | **无条件 FAIL** ✓ |
| detail 是否含两个原始观测 | detail 内拼接 `visibleInnerText=${visibleInnerText}`、`firstTdDisplay=${firstTdDisplay}`，并附 `DOM 首列=...`、`POST=...`、`重复数=...` | **两个原始观测都在** ✓ |
| 原始观测落盘 | `ctx.out.writeJson('C/I-ACCT/visibility-observations.json', visibilityObservations)` + `I-ACCT-06-visibility` 截图 | 有独立证据 ✓ |
| 是否用延长 sleep 制造通过 | 旧 `sleep(page,1300) + sleep(page,800)` **被删除**，改为 `page.waitForResponse(POST /accounts)` + `page.waitForFunction(tbody 行数 === rowsBefore+1)` 的确定性信号 | **未延长 sleep** ✓ |
| 判定口径是否被换成 innerText 制造 PASS | 该条**仍以可见性 FAIL 收尾**，未因改用 `td.textContent` 而改判 PASS；`visibleInnerText` 被作为失败证据记录 | **未改判** ✓ |

> 注：D 套件新增的 `D-ACCT-01..04` 是**另立口径的新增用例**（DOM/API 数据一致性），不是对 `I-ACCT-06` 的改判；每条 detail 都显式标注「不含视觉可见性」并保留 `visibleInnerText` / `firstTdDisplay`。详细分析见 `qa/assertion-review.md`。

---

## 6. 审计结论

1. **范围合规**：无越界修改；`style.css`、Go 后端、PRD/WBS、DB/API、历史证据、README/CHANGELOG 均未被触碰；未提交、未合并、未 push。
2. **业务改动最小化**：`app.js` 仅 6 个 diff hunk（43 insertions / 9 deletions），全部对应 3 项授权修复；新增账户一项业务 JS 零改动。
3. **安全口径保持**：无 `innerHTML`、异常未被掩盖式吞掉、后端 403 校验未挪走。
4. **门禁语义正确**：D step 仅一份且为硬门禁，A 未变、C 仍 `continue-on-error`。
5. **`I-ACCT-06` 保持无条件 FAIL**，两个原始观测齐备，未用 sleep 或改口径制造 PASS。
6. **需 team-lead 确认的边界项**：`.github/workflows/r0-e2e.yml` 由工程师修改（见 2.1）；以及全部改动仍处于未提交工作树状态，交付前需明确是否落 commit。
7. **低危观察**（不构成本轮 FAIL）：workflow job 名未含 D；`a-scenarios.js` A08 的 detail 有一句修复前遗留的硬编码文案「UI 未隐藏=真实缺陷，已记录」，在修复后 inviteBtn=0 时语义矛盾（详见 `qa/assertion-review.md`）。

---

## 7. 本审计使用的证据

- `qa/build.log`（QA 独立重新构建日志，EXIT_CODE=0）
- `qa/a-qa.log` / `qa/d-qa.log` / `qa/c-qa.log`（QA 独立重跑日志，含真实退出码）
- 本目录下 `acceptance-matrix.md`、`assertion-review.md`、`qa-report.md`
