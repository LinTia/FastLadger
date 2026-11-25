# 本轮修订对照 · R0 总任务单依 AI3 独立复核整改（第二版）

> **用途**：逐条登记本轮对 `docs/FastLedger-R0-收口总任务单-2026-11-25.md` 的修订，供 ChatGPT / 第三 AI 逐项核对「要求 → 落点 → 验证」。
> **性质**：对照说明文件。**不含任何已生效授权**，不含业务代码/测试/统计改动。

| 项 | 值 |
|---|---|
| 被审基线 | `0eee08446d4a31d23a85ed2ade130c7f01b99053`（第一版交付，已推送，核验时仍为远端 tip） |
| 修订对象 | `docs/FastLedger-R0-收口总任务单-2026-11-25.md`（474 行 → 本版行数以 `wc -l` 为准） |
| 修订依据 | `docs/FastLedger-R0-收口总任务单独立复核-AI3-0eee0844.md` |
| **依据可用性** | **该报告文件在仓库内不存在**（`git log --all --diff-filter=AM -- "*独立复核*"` 零命中）。本轮按 Owner 会话内转述的 10 项执行，**未引用报告正文**；若原文另有要求，本对照不覆盖 |
| 核验时点 | 2026-11-25 16:20–16:45 +08 |

---

## 0. 一句话结论

第一版的实质问题不是「漏了几项」，而是**三类结构性缺陷**：

1. **九条放行条件里有 4 条（1/3/4/5）没有任何任务承接**，而 T19 又写着「不新增内容、仅汇总前 18 项」——这四条永远无人负责，门禁结构性无法闭合。
2. **把 AUTH-A 当成了万能授权**（业务代码、runner 通用改造、fixture 都往上挂），而 AUTH-A 的真实范围只有「4 条 FAIL 的判果」。
3. **把「等一个未到手的授权」写成「可直接实施」**（T12/T15/T18 依赖的 OD-06 从未取得），并把**不入库的 `artifacts/` 日志**当作可复核证据。

本版针对这三点做了结构性修正，并逐条完成 10 项要求。

---

## 1. 逐项对照（10 项）

### 1.1 要求 1 · 补齐第 1/3/4/5 条承接任务与证据映射

| 项 | 内容 |
|---|---|
| **原文（v1）** | T19「修改要求：**不新增内容**，仅在前 18 项完成后按条汇总证据、出最终验收报告」；§3 只有四类分组，无「九条 → 任务」映射；T01–T20 中无任何一项对应第 1、3、4、5 条 |
| **修订** | ① 新增 **R0-T21**（第 1 条：13 页七要素）、**R0-T22**（第 3 条：状态矩阵）、**R0-T23**（第 4 条：R0-E-35 独立复跑）、**R0-T24**（第 5 条：API 矩阵 + 全量/race）；② 新增 **§3 九条 → 承接任务/证据映射表**（9 行，含现有证据、仍缺什么、当前判定）；③ T19 改为「按 §3 映射表逐条汇总」，并写明闭合性说明 |
| **验证** | §3 表 9 行齐全；每条至少 1 个任务编号；T19「前置依赖」= T01–T18、T21–T24；T19 备注「原因已从『结构性无承接』变为『前置任务未完成』」 |

### 1.2 要求 2 · 修正 T05–T08 与 AUTH-A 的错误关联

| 项 | 内容 |
|---|---|
| **原文（v1）** | T05 前置依赖：「若选 ①，需 `AUTH-A`（改 runner 之外的业务代码需单独授权，AUTH-A 仅覆盖 runner 断言）」——**自相矛盾**：括号里说 AUTH-A 只管断言，主句却把 AUTH-A 列为业务代码前置；T09「前置依赖：`AUTH-A`（改 runner 断言）」把 AUTH-A 说成通用 runner 授权；T10「`AUTH-A`；① 类还需 fixture 授权」 |
| **修订** | ① 新增 **§2 授权矩阵**：2.1 六类授权对象与「覆盖/不覆盖」；2.2 业务代码**逐文件**清单（按任务）；2.3 测试资产与 `sourceSha256` 合法修订路径；2.4 六条禁止混用；② T05 前置改为「业务代码实施授权（逐文件 `app.js`，绑定 SHA）；判果翻转属 AUTH-A」；③ T08 补「`app.js`+`index.html`+`style.css`，**与 AUTH-A 无关**；判果才属 AUTH-A」；④ T09 改为「测试配方授权（**不在 AUTH-A 内**）+ 各组业务代码授权」；⑤ T10 改为「fixture 授权 + 测试资产授权（+ 若实现的业务代码授权）」；⑥ T17 明确「单列测试资产授权，不得并入 AUTH-A」 |
| **验证** | 全文 `AUTH-A` 出现处均带「仅 4 条 FAIL 判果」限定；§2.1 表「明确不覆盖」列逐项列出业务代码/fixture/统计；§2.4 第 1–3 条为禁止混用清单 |

### 1.3 要求 3 · 重新整理 T06/T07 邀请注册

| 子项 | 原文（v1） | 修订后 |
|---|---|---|
| PRD 已定需求 | 「PRD 已定需求见 `G-1`（任务单 `:348–359`：**Web 0 实现**）」 | 逐字引用 PRD `[1.12]` §12.1 六条（`PRD.md:581–587`），明写「**已定需求，非待裁**」；「不得以 N/A 消除 FAIL」写入修改要求 |
| 是否纳入 R0 | 未提 | 修改要求 ①：Owner 一句话（纳入 / 不纳入）；指出 `准备清单 OD-07` 的「不纳入 R0，排 R1 后」**只是提案、未签署**；若不纳入，统计二选一并给推荐（保持 FAIL + 标注「已确认产品缺口，Owner 已排除 R0」） |
| 按 PRD 完整流程验收 | 「错误/过期/成功三条路径」 | **8 条验收面**：生成 / 展示 / 单次使用 / 7 天过期 / 撤销 / 绑定校验 / 限流（5 每分钟、10 每 IP 每小时）/ 原子性 + `audit_logs` 四类记录；T07 追加六类失败文案与稳定元素 `#joinErr` |
| 业务代码与测试资产分授 | 只写「实现需新授权」 | ④ 业务代码（`app.js` + `index.html`）与测试资产（`I-LOGIN-11/12` 判果属 AUTH-A；改 `controls.json` 另需测试清单修订授权）**分别授权** |
| `controls.json`/`c-controls.js` 修订路径 | 无 | 新增 ⑤ + §2.3：只能「改 `基准冻结 §3.1` → `extract-controls.js --check` → 重生成 → 校验新 `sourceSha256`」，**禁止手改 JSON**；并说明只改判果时哈希不变但统计必变 |
| 「Web 0 实现」 | 沿用旧表述 | 改为三层证据：后端 `router.go:128` 已存在 / 生成侧 `app.js:1678–1706` 已实现 / 兑换侧 `grep` 0 命中；并登记 `准备清单 OD-07` 的「Web 0 命中」为**跨文档不一致待办**（本轮不改该文件） |

**验证**：`grep -c "members/join\|redeem\|joinCode" server/web/dist/app.js` = **0**；`router.go:128` 存在；`app.js:1678/:1689/:1698` 存在；`controls.json` 的 `I-LOGIN-11/12` 为 `**缺失**｜R0-E-38`。

### 1.4 要求 4 · 用可复核证据替换被 gitignore 的 artifacts 日志

| 项 | 内容 |
|---|---|
| **原文（v1）** | T15 位置「`artifacts/a05-r13-formal-ci/job.log:117–118`」；§6 #10「`grep "C 汇总" artifacts/…job.log:1063`」；§5.1「取自 `…job.log:1063`、`:1065`」 |
| **判定** | `.gitignore:16` 排除 `artifacts/`；`git ls-files artifacts` = **0**；`CHANGELOG.md:37` 亦记「本地 `artifacts/` 已 gitignore」。→ **第三方无法从仓库复核该日志，行号不可复现** |
| **修订** | ① T15 重写为「run 要素（有入库旁证 `剩余阻塞项任务单:177`）+ 入库证据（`p2/c-after-p2.log:6–7`、`p2/c-after-p2-report.json`）+ 获取方式（UI / API 两条路径）+ 行号口径（仅对同一下载日志成立，正式引用改字面串锚 / `results[<id>]`）+ artifact 保留期（约 2026-12-08 到期）」；② §7 新增第 9 条「任何以 `artifacts/` 为证据的表述须标『本地未入库』」；③ §8 证据表改为入库可复现项 |
| **验证** | 全文搜 `artifacts/` 仅剩「不入库说明」与「本地未入库」标注；`p2/c-after-p2.log` 第 6–7 行可直接 `sed -n '6,7p'` 复核 |

### 1.5 要求 5 · 修正 PR#1 已合并事实，明确 PR#3/#4 状态

| 项 | 内容 |
|---|---|
| **原文（v1）** | §0「`refs/pull/1/head`=`44b842a`…**三者均未并入 `main`**」；T20 只写 PR#3/#4；§5.7「**PR#1 未取回本地**：`44b842a` 对象不在本地仓库，未核对其与 `main` 的关系」 |
| **实测** | PR#1 **已合并**：squash `ca43fb75a4940f28598099a9963af8eef15c856c`（subject 以 `(#1)` 结尾）在 `main` 且在本分支；`refs/pull/1/merge` **不存在**。PR#3：`06e077f` 不在 `main`、在分支，`pull/3/merge=4cb41db` 存在 → open。PR#4：`16705fd` 两者皆无，`pull/4/merge=80dde0a` 存在 → open |
| **修订** | ① §0 基线表新增两行（PR refs / PR 合并事实）；② T20 重写为「PR#1 已合并确认 + PR#3/#4 落差处置」，逐 PR 给处置选项与验收；③ §7 第 7 条如实声明「REST API 403 限流，合并字段未直接读取，结论为三方交叉推定」；④ 删除 §5.7 的旧表述 |
| **验证** | `git merge-base --is-ancestor ca43fb7 b4309db` = YES；`… 16705fd HEAD` = NO；`git ls-remote 'refs/pull/*'` 中无 `pull/1/merge` |

### 1.6 要求 6 · 修正 T18 的 PASS / BLOCKED / FAIL 分布

| 项 | 内容 |
|---|---|
| **原文（v1）** | 「实测仅 **4** 条 FAIL…。差额 15 条的成因：…等已在后续轮次实现，运行时 `R` 覆盖了静态标注」——**把 19 条与 4 条当同一集合做差**，且未给真实分布 |
| **实测（19 条逐条取 `results[id].status`）** | **PASS 7**（BOOKS-10、ADD-09、ADD-10、TXDET-04、ACCT-07、MEM-07、MEM-11）｜**BLOCKED 9**（BOOKS-11、ACCT-14、CAT-14、MINE-13、MINE-14、SET-09/10/11/12）｜**FAIL 3**（LOGIN-11、LOGIN-12、HOME-01） |
| **关键更正** | 第 4 条 FAIL `I-LOGIN-04` **不在该 19 条内**（其缺失走用例内 `bad()`，`c-controls.js:106`），因此「19 vs 4 → 差 15」的算法本身不成立 |
| **修订** | T18「当前证据」整段替换为三分布 + 集合关系说明；验收标准改为「`PASS 7 / BLOCKED 9 / FAIL 3` 可逐条对账」 |
| **验证** | §8 #19 给出复现命令与结果；BLOCKED 9 可再拆为 T09 6 条（CAT-14、SET-09..12、ACCT-14）+ T10 3 条（BOOKS-11、MINE-13、MINE-14） |

### 1.7 要求 7 · 修正 T12/T15/T18 授权字段与实施分类

| 项 | 内容 |
|---|---|
| **原文（v1）** | §1「可直接实施 **3** 项（T12/T15/T18）」；三项授权字段均写「**否**——…仅需 OD-06 **文档修订授权**」（既说「否」又说「需要授权」）；T12 分类「**可直接实施**」 |
| **修订** | ① 三项分类改「**需 Owner 授权（文档类，OD-06）**」；② 授权字段改「**需要**——OD-06 文档修订授权（**未取得**；`准备清单:209` 末列记「否」）」；③ §1 计数表与 §5 明细同步：可直接实施 3 → **1**（仅 T21），需 Owner 决策或授权 14 → **17**；④ 每项加「分类修正说明」，写清为何原分类不成立 |
| **验证** | §1 计数表 1/17/4/2 = 24；全文搜「可直接实施」只应命中 T21 与说明性文字 |

### 1.8 要求 8 · 补充 T15 的 CI workflow 基线证据

| 项 | 内容 |
|---|---|
| **原文（v1）** | 只有「CI 运行基线为 `ad34df1`（2026-11-08）」与 job.log 行号，**无 workflow 文件、job id、step 名、run URL** |
| **修订** | T15 增加：workflow `.github/workflows/r0-e2e.yml`（`name: R0 E2E`，id `407856175`）；job `A(11 场景) + D(缺陷回归) + C(228 控件) + B(分类审计)`（id `128690050693`）；step `包 C · 228 条逐控件运行时验收（报告型）` + `continue-on-error: true`；run URL `https://github.com/LinTia/FastLadger/actions/runs/42467412803`；`head_sha`、`head_branch`、`event=workflow_dispatch`、`created_at`；并证明 `ad34df1..HEAD` 对 `.github/workflows/` **零改动**（`git diff --stat` 为空）→ 步骤定义可在 HEAD 复核 |
| **来源分层** | run name/branch/head_sha/conclusion 有**入库旁证**（`剩余阻塞项任务单:177`）；job id / artifact id / 体积仅见**未入库**的 `artifacts/a05-r13-formal-ci/README.md`，已在文中标注「须按 run 页核对」 |
| **验证** | `.github/workflows/r0-e2e.yml` 内 `continue-on-error: true` 与 step 名可 `grep`；`git diff --stat ad34df1 HEAD -- .github/workflows/` 为空 |

### 1.9 要求 9 · 修正 T04 分类、T17 数量、第三方复核等非阻断问题

| 项 | 原文（v1） | 修订 |
|---|---|---|
| T04 分类 | §1 列「需 Owner 决策」，但 T04「修改要求」首句「**当前不可实施**」 | 分类改「**当前不可实施**（前置决策未定）」，并加「分类修正说明」：Owner 决策登记在 T01/T02，本项须三项前置齐备后才转为「需授权执行」 |
| T17 数量 | 标题「其余 **13** 条」，位置却列 **14** 个（含 `:163`）；证据「其中 **11 条**对象错位」把 BOOKS-09 算进来 | 位置列改为 **13 个**（去 `:163`），明确「13 = 14 − 1」；拆分改「**10 条对象错位 + 3 条对象相关但无观测**」 |
| 第三方复核 | T12/T15/T18「否——抽查即可」，T17「需要」但未说明抽查口径 | T12/T15/T18 保持「否」并给抽查口径（如 T15「抽查 run 页 job 名 + 两条 blob SHA」）；T17 升为「**必须**」并写理由（13 条全在 `164 PASS` 内，真伪直接决定门禁第 2 条） |
| T03/T14 | 「否——…抽查即可」 | 补明抽查方式，避免空泛 |

### 1.10 要求 10 · 更新远端/本地/main/分叉/提交数/推送状态

| 项 | v1 值 | v2 实测值（2026-11-25 16:2x） |
|---|---|---|
| 远端 tip | `ff666d0` | **`0eee084`** |
| 本地 HEAD | `a220acc`（领先 1） | **`0eee084`**（领先 0 / 落后 0） |
| `main` | `b4309db` | `b4309db`（未变） |
| 分叉 | 领先 66 / 落后 33 | **领先 67 / 落后 33**（`rev-list --left-right --count` = `33 67`） |
| `ad34df1..HEAD` | 42 | **43**（代码路径改动仍 **0**） |
| 本分支提交总数 | 未记 | **365** |
| 推送状态 | §5 未写 | §0 / §9 明写「本轮未推送，等待当次指令」 |

> 所有旧值以「第一版记 X」形式保留在括号内，便于追溯，不静默覆盖。

---

## 2. 结构性改动统计

| 类别 | 数量 | 明细 |
|---|---|---|
| 新增任务 | 4 | T21、T22、T23、T24 |
| 重写任务 | 8 | T04（分类）、T05、T06、T07、T08、T15、T18、T20 |
| 字段级修正 | 6 | T09、T10、T12、T17、T19、T03/T14（复核口径） |
| 新增章节 | 5 | 版本与修订依据、§0.1 整改落点、§2 授权矩阵、§3 九条映射、§10 Owner 决策速览 |
| 删除表述 | 3 | §5.7「PR#1 未取回本地」、§1 旧四类计数「3/14/2/1」、T19「不新增内容」 |

**行数变化**：474 行 → **740 行**（`wc -l`）。**未改动**任何其他文件；新增对照文件 202 行。

---

## 3. 数字口径自查

| 断言 | 校验方式 | 结果 |
|---|---|---|
| 任务总数 24 | §1 表行数（T01–T24） | 24 |
| 分类合计 24 | 1 + 17 + 4 + 2 | 24 |
| 需决策/授权 17 | §5.2.1 九项 + §5.2.2 八项 | 17 |
| 19 条标注分布 | PASS 7 + BLOCKED 9 + FAIL 3 | 19 |
| 统计口径 | 全文仅出现 `164 PASS / 4 FAIL / 59 BLOCKED / 1 N/A / 228`，无合计式表述 | 通过（`grep` 自查） |
| 提交数 | 43 = `rev-list --count ad34df1..HEAD`；代码路径 0 | 通过 |
| 分叉 | `33 67` | 通过 |

---

## 4. 未解决的 Owner 决策清单（17 项）

见正文 §10 速览表。按性质分两组：

- **产品/范围/治理（9）**：T01 时点、T02 二选一、T05、T06、T07、T08、T11 三选一、T14 二选一、T20 逐 PR。
- **实施/文档授权（8）**：T09 测试配方、T10 fixture、T12/T15/T18 OD-06（三项）、T16 AUTH-B、T17 13 条测试资产、T22 OD-03。

**另有 4 项无需决策但需外部执行**：T03 受控测量、T13 服务器只读、T23 他机复跑、T24 放行基准复跑。
**WB 可立即开工 1 项**：T21（只读台账）。

---

## 5. 验收证据（复现命令与结果）

| # | 命令 | 结果 |
|---|---|---|
| 1 | `git ls-remote origin refs/heads/docs/r0-closure-tasklist refs/heads/main` | `0eee084…` / `b4309db…` |
| 2 | `git rev-parse HEAD` + `git status --short --branch` | `0eee084…`；工作区干净 |
| 3 | `git rev-list --left-right --count b4309db...HEAD` | `33 67` |
| 4 | `git rev-list --count HEAD` / `--count ad34df1..HEAD` | `365` / `43` |
| 5 | `git rev-list --count ad34df1..HEAD -- server/ tools/r0/ server/web/dist/ .github/workflows/` | `0` |
| 6 | `git diff --stat ad34df1 HEAD -- .github/workflows/` | 空 |
| 7 | `git rev-parse ad34df1:server/web/dist/app.js` vs `HEAD:…` | 均 `3f3eb06…`（style.css 均 `c6fca0c…`） |
| 8 | `git check-ignore -v artifacts/a05-r13-formal-ci/job.log` + `git ls-files artifacts` | `.gitignore:16`；入库 0 |
| 9 | `sed -n '6,7p' docs/r0-evidence-7th-fix/p2/c-after-p2.log` | `C 汇总：{"PASS":164,"FAIL":4,"BLOCKED":59,"N/A":1}` + `SUMMARY {…228}` |
| 10 | `node -e` 按 19 个 id 统计 `results[].status` | PASS 7 / BLOCKED 9 / FAIL 3 |
| 11 | `grep -c "members/join\|redeem\|joinCode" server/web/dist/app.js` | `0` |
| 12 | `sed -n '120,135p' server/internal/api/router.go` | `:128` `POST /books/:id/members/join` |
| 13 | `grep -n "members/invite" server/web/dist/app.js` | `:1689` |
| 14 | `git merge-base --is-ancestor ca43fb7 b4309db`（及 HEAD） | 均 YES |
| 15 | `git merge-base --is-ancestor 06e077f b4309db` / `… HEAD` | NO / YES |
| 16 | `git merge-base --is-ancestor 16705fd b4309db` / `… HEAD` | NO / NO |
| 17 | `git ls-remote origin 'refs/pull/*'` | 无 `pull/1/merge`；有 `pull/3/merge`、`pull/4/merge` |
| 18 | `curl -sSI raw.githubusercontent.com/…/259bd11/…network-log.json` | 200 / 182369 / ETag `bf046e12…` / Date 2026-11-25 08:27:35 GMT |
| 19 | 匿名 `curl https://github.com/LinTia/FastLadger` | 200（仍 Public） |
| 20 | `curl https://api.github.com/repos/LinTia/FastLadger/pulls/1` | **403 rate limit**（故 PR 用 git 判定） |

---

## 6. 未覆盖与自指声明

1. **报告原文未入库**：本轮无 `docs/FastLedger-R0-收口总任务单独立复核-AI3-0eee0844.md`，10 项按 Owner 转述执行；若原文含未转述要求，需补一轮。
2. **PR 合并字段未直接读取**：REST API 403；PR#1「已合并」由「squash 提交 + 祖先关系 + merge ref 缺失 + CHANGELOG `[1.57]`」交叉推定；PR#3/#4「open」由 merge ref 存在推定。建议配额恢复后补 `GET /pulls/1`。
3. **job id / artifact id 未入库**：仅见本地 `artifacts/…README.md`，正文已标「须按 run 页核对」，未计入可复核事实。
4. **未运行任何测试**：`164 PASS / 4 FAIL / 59 BLOCKED / 1 N/A / 228` 全部为只读引用；本轮无新运行时证据。
5. **自指**：本对照文件所在提交的送达推送，不在本轮任何台账内登记（沿用「自指约定」）。
