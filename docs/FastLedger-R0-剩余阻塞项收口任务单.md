# FastLedger R0 剩余阻塞项收口任务单（第十五轮）

> **本轮性质**：纯只读核查 + 收口任务单编写。**不改任何业务代码 / CSS / HTML / 数据库 / API / 测试判定 / fixture / C 套件统计 / PRD / Owner 决策 / 既有验收口径；不合并 PR #3 / PR #4；不 rebase、不 force-push、不改写历史；不启动 R1–R7 / UI-DESIGN-02 / PWA。**
>
> **当前基线（本轮重新 `fetch origin` 实测，不沿用旧报告）**：
> - 分支 `fix/r0-sixth-runtime-defects`，HEAD = `06e077ff699b5ed74e532fcde28a6e2f9cf014`，工作区 clean。
> - `origin/main` = `b4309dbd481423321f6740dfad5099ad037b0e10`（未变）。
> - PR #3 `open / draft=false / merged=false / base=main / head=06e077f / commits=27`；PR #4 `open / draft=true / head=16705fd`。
> - 仓库 `private=false` / `visibility=public`（Owner「改回 Private」决策**仍未执行**）。
> - 旧提交 `259bd11` 匿名 `GET /commits/259bd11` 仍 **HTTP 200**；最近 15 条 CI runs 最新 success 为 `b4309db` 的 main push，**未见覆盖 `06e077f` / `ad34df1` 的 run**。
> - **R0 仍为 BLOCK**；`docs/FastLedger-R0-收尾执行清单.md` §8「最终 R0 放行条件」9 条全部 `[ ]` 未勾选（L181–189）。

---

## 1. 阻塞项总览（编号与状态）

| 编号 | 阻塞项 | 当前状态 | 主分类（详见 §3） |
|---|---|---|---|
| BLOCKER-01 | RV-05 仓库公开 + 旧 SHA 历史凭证仍可读 | 当前树已脱敏但**历史对象仍公开可读**；Owner ACTION REQUIRED | B |
| BLOCKER-02 | t021 分类覆盖无 Owner 裁决（WorkBuddy 自署） | `pending owner decision`；覆盖仍在 `category-overrides.json` | B |
| BLOCKER-03 | OBS-ACCT-VIS-390：390px 账户名称列被 CSS 隐藏 | `FAIL / closed=false / 待 Owner 裁决`；清单外独立项 | B |
| BLOCKER-04 | C 套件 4 FAIL / 59 BLOCKED（`164/4/59/1/228`） | 4 条 FAIL 均非可信运行时断言；I-LOGIN-10 漏计为 BLOCKED | A + B |
| BLOCKER-05 | A05 代码整改 + CI 证据覆盖关系 | A05 代码 `ad34df1` 已 CI 覆盖（run `42467412803`，A 11/11）；最新 HEAD `06e077f` 仅文档、无新 CI | D + C |
| BLOCKER-06 | D-4 期初余额：方案定稿、功能未实现 | 技术方案已确定；实施前需源码核对与测试设计 | B |
| BLOCKER-07 | D-8 备份区块：方案定稿、功能未实现、保留策略冲突 | 技术方案已确定；保留 7 份 vs 14–30 份冲突待 Owner | B |
| BLOCKER-08 | G-1 / G-2 / G-3 范围外已确认缺口（独立工作线） | 已登记、未实施、未授权验收 | B |
| BLOCKER-09 | R0 §8 整体放行条件 9 条全未勾选 | R0-E-35、截图基线、API/race 等门禁未闭环 | A + B + C |

---

## 2. 逐项阻塞项（10 要素）

### BLOCKER-01 · RV-05 仓库公开与历史凭证暴露

1. **唯一编号与当前状态**：`RV-05`（P0 安全事件残余）。当前树凭证已脱敏、`EBK_JWT_SECRET` 已轮换、本地历史已重写；但**旧提交 `259bd11` 仍匿名可读**（commit API / raw 均 HTTP 200），仓库仍 `private=false`。
2. **对应需求条款、代码文件及具体行号**：
   - `docs/FastLedger-R0-凭证安全核验记录.md:69–70` — 旧提交对象匿名 `GET /commits/259bd11` → HTTP 200；`raw.githubusercontent.com/.../259bd11/.../network-log.json` → HTTP 200，仍含 29 处凭证字面量。
   - `docs/FastLedger-R0-收尾执行清单.md:34` —「RV-05 旧 Git 对象：P0 历史暴露风险仍待实际处置/验证」。
   - `CHANGELOG.md:[1.69]`（L179）— Owner 裁决「将 GitHub 仓库由 Public 改回 Private」。
3. **当前问题的可核查证据**：`GET /repos/LinTia/FastLadger/commits/259bd11` 匿名 HTTP 200（无活动分支/标签指向该 SHA，但悬空对象可达）；旧令牌受控验证已失效（HTTP 401）——**凭证值已无效，但明文历史记录仍公开**。
4. **WorkBuddy 是否有权直接处理**：**否**。改仓库可见性、重写历史、删除远端 refs、联系 GitHub Support 均为 Owner 专属操作。CHANGELOG [1.69] L207 明确「远端历史对象清理需 Owner 明确批准」；本轮未执行任何此类动作。
5. **需 Owner 裁决的具体问题**：① 是否在 GitHub 侧执行「改回 Private」；② 是否联系 GitHub Support 清除 `259bd11` 等悬空对象缓存；③ 改 Private ≠ 历史对象物理清除，是否需进一步历史清理评估。
6. **最小修复方案（仅描述，需 Owner 操作）**：Owner 于 GitHub 将仓库设为 Private → 重新匿名请求 commit API / raw 应返回 404；若需彻底清除旧对象，Owner 决定是否联系 Support（提供影响面与回滚方案）。
7. **可重复执行的验收步骤**：`curl -sS -o /dev/null -w '%{http_code}' https://api.github.com/repos/LinTia/FastLadger/commits/259bd11` 期望 404（Private 后）；`git ls-remote` 确认无活动 ref 指向 `259bd11`。
8. **验收通过的客观标准**：匿名访问旧 SHA 与旧 `network-log.json` 均不可达（404）；仓库 `private=true`；在交付回执中如实记录「当前树脱敏」与「历史对象清理」是两件事。
9. **需 ChatGPT 独立复核的内容**：历史暴露风险评估是否充分；「改 Private 即视为安全事件关闭」是否被错误表述（不得如此表述）。
10. **对 R0 放行的影响**：属 §8 第 7 条「RV-05 分项如实记录」要求；不关闭则 R0 不得放行（L177：无未关闭门禁 + Owner 签字才建议 R1）。

### BLOCKER-02 · t021 分类覆盖无 Owner 裁决

1. **唯一编号与当前状态**：`t021`（数据集分类映射覆盖）。状态 `pending owner decision`；覆盖写入 `tools/r0/category-overrides.json`，`decidedBy` 为 WorkBuddy。
2. **对应需求条款、代码文件及具体行号**：
   - `tools/r0/category-overrides.json` — `txId:"t021"`，`categoryId:"dinner"`，`decidedBy:"WorkBuddy（R0 第六次整改，分类解析补充整改）"`，`decidedAt:"2026-11-05"`。
   - `docs/FastLedger-R0-第七次测试质量纠偏任务单.md:68–77` — 明确要求「暂不把 WorkBuddy 自行决定视为 Owner 决策；将 t021 设为 pending owner decision」。
3. **当前问题的可核查证据**：覆盖把 `t021` 从原型 `categoryId=meal`（餐饮-聚餐）改判为 `dinner`；`decidedBy` 写为 WorkBuddy，**无 Owner 裁决证据**。第七次纠偏任务单判定此为 P1（需补 Owner 决策依据）。
4. **WorkBuddy 是否有权直接处理**：**否**（裁决产品数据语义）。WorkBuddy 可建议、可记录冲突，但**不得替 Owner 选择 meal / dinner**。
5. **需 Owner 裁决的具体问题**：选 `meal` → 原型可见分类与数据 ID 不同；选 `dinner` → 以展示名为准、改变原有数据映射。两种方案影响不同，需 Owner 一字/一句裁决（可在聊天中作出）。
6. **最小修复方案（仅描述）**：Owner 裁决后，将 `decidedBy` 改为 Owner 并补 `reason`；或把覆盖移到不影响默认审计的候选文件；在裁决前保持 `pending owner decision` 标记。
7. **可重复执行的验收步骤**：`grep -n t021 tools/r0/category-overrides.json` → 核对 `decidedBy` 是否为 Owner；`node -e` 加载 JSON 校验 schema（`$schema:"r0/category-overrides@1"`）。
8. **验收通过的客观标准**：覆盖项 `decidedBy` 为 Owner 实名 + 日期；任务单 §4 P1 关闭；B 套件不再因该 override 维持 PARTIAL。
9. **需 ChatGPT 独立复核的内容**：「以用户可见展示名为准」的判定逻辑是否合理；是否应保留 `meal` 原始记录与冲突日志。
10. **对 R0 放行的影响**：影响第七次纠偏 B 套件 PARTIAL 状态；属 §8 文档一致性要求；不关闭则 B 维持 PARTIAL。

### BLOCKER-03 · OBS-ACCT-VIS-390 390px 账户名称列不可见

1. **唯一编号与当前状态**：`OBS-ACCT-VIS-390`。`status=FAIL`、`closed=false`、`ownerDecision=待 Owner 裁决`；**清单外独立项，不占 228 编号、不写入 `R`**。
2. **对应需求条款、代码文件及具体行号**：
   - `server/web/dist/style.css:327–333` — `@media (max-width:640px)` 内 `th:nth-child(1), td:nth-child(1), ... { display: none; }`（行号核实：**L331**）。
   - `server/web/dist/app.js:696` 账户表 4 列表头（`["账户","类型","余额","操作"]`）；`:701` / `:722` 首列 `tdN = a.name`（账户名称）。
   - `tools/r0/e2e/specs/c-controls.js:423–484` — 运行时断言 `firstTdDisplay` / `visibleInnerText` → 实测 `none` / `false`。
3. **当前问题的可核查证据**：`docs/r0-evidence-7th-fix/p2/visibility-observation-independent.json` — `visibleInnerText:false`、`firstTdDisplay:"none"`、`postStatus:200`、`inDom:true`、`inApi:true`（数据链路正常，仅视觉不可见）。属**响应式样式问题**，非业务逻辑、非测试口径（历史曾有一次口径错位，已在第七次 P2 修正）。
4. **WorkBuddy 是否有权直接处理**：**否**。跨多轮回执一致确认「未授权改 `style.css` L331」；状态始终 `FAIL / 待 Owner 裁决`。
5. **需 Owner 裁决的具体问题**：① 是否将「窄屏账户名称必须可见」确立为产品要求（→ 确认为缺陷、需修）；② 是否授权改 `style.css:331`（推荐 `:has()` 收窄作用域方案）；③ 是否把其余 ≤640px 表格首列隐藏一并纳入排查。
6. **最小修复方案（仅描述）**：将 `style.css:331` 裸全局选择器收窄为 `table:has(th:nth-child(5))`（账户表仅 4 列不匹配 → 首列恢复可见，交易表 8 列仍裁剪）；不动 `app.js` / `controls.json` / 运行器 / 228 计数。
7. **可重复执行的验收步骤**：见 OBS 报告字段 7（390/641/1280 三视口对比 `getComputedStyle(td:first-child).display` 与 `innerText.includes(唯一账户名)`）。
8. **验收通过的客观标准**：390px 下账户名称 `display=table-cell` 且 `innerText` 含唯一名、截图可读出；228 汇总仍为 `164/4/59/1/228`、`R` 中无 `OBS-*` key。
9. **需 ChatGPT 独立复核的内容**：归类「响应式样式问题」是否成立；专属截图空白、证据质量是否需重采；其余表格首列是否同缺陷。
10. **对 R0 放行的影响**：不占 228 计数，但属 §8「同视口同状态对照基线」缺口（390×844 下可见性偏差未闭环）；不影响放行计数但阻塞同视口截图基线达标。

### BLOCKER-04 · C 套件 4 FAIL / 59 BLOCKED（`164/4/59/1/228`）

1. **唯一编号与当前状态**：`I-LOGIN-04` / `I-LOGIN-11` / `I-LOGIN-12` / `I-HOME-01` 均为 **FAIL**；`I-LOGIN-10` 为 **BLOCKED**（历史漏计）。统计基线 `164 PASS / 4 FAIL / 59 BLOCKED / 1 N/A / total 228` **原样保留**。
2. **对应需求条款、代码文件及具体行号**：
   - `tools/r0/e2e/specs/c-controls.js:106` — `R['I-LOGIN-04'] = bad(...)` 无条件硬编码。
   - `:175` — `R['I-HOME-01'] = bad(...)` 无条件硬编码。
   - `:78–85` — `classifyMissing()` 由 `controls.json.status` 静态登记 FAIL（I-LOGIN-11/12 走此路，`grep` 全文 `I-LOGIN-11|I-LOGIN-12` **0 命中**显式 `R[id]`）。
   - `:163` — `R['I-BOOKS-09'] = ok(...)` 硬编码 PASS（零断言）。
   - `:895–909` — 汇总循环；`:908` 兜底 BLOCKED（I-LOGIN-10 走此）。
   - `tools/r0/e2e/fixtures/controls.json` — `I-LOGIN-04:74–88`、`I-LOGIN-10:170–184`、`I-LOGIN-11:186–200`、`I-LOGIN-12:202–216`、`I-HOME-01:442–456`（其中 I-LOGIN-04/10 因 `extract-controls.js:27` 裸 `split('|')` 未处理 `\|` 致 `evidence/status` 截断污染，只登记不修复）。
3. **当前问题的可核查证据**：`artifacts/a05-r13-formal-ci/job.log:1070–1073` 与 runner 硬编码/静态模板**逐字一致** → 4 条 FAIL 均非浏览器观测结果；`job.log:1178` 仅 I-LOGIN-10 占位截图（4 条 FAIL 无专属截图/诊断）。注：该日志对应 `ad34df1` 旧基线 CI run `42467412803`，**非当前 HEAD 运行时**，本轮未复跑。
4. **WorkBuddy 是否有权直接处理**：**部分**。WorkBuddy 可执行 runner 真实断言整改（把硬编码 `bad()` / `classifyMissing` 改为对原判定域的真实观测），但**状态变更（FAIL→N/A/PASS、计数调整）须 PM/Owner 书面裁决**；不得擅自放宽断言、删用例、改编号、硬编码结果制造 PASS，不得改 `164/4/59/1/228`。
5. **需 Owner 裁决的具体问题**：I-LOGIN-04 是否将密码显隐正式纳入 PRD/规范（否则按 RV-04 同类移出 FAIL 计数）；I-LOGIN-11/12 登录页第三入口是否增控件；I-HOME-01 验收域是否由「首页本页内」改判（D-3 顶栏切换已裁决，不得重新裁决位置）。
6. **最小修复方案（仅描述）**：工作线 A——4 条判果改为对各自原判定域的真实 Playwright 观测，`detail` 实测拼装 + `shots` + 诊断；期望值与需求状态一致，不把未确认控件「存在」写成 PASS 条件；`controls.json` 逐字不变（含污染字段）。
7. **可重复执行的验收步骤**：见 qa-r15-g1-scan 字段 7（git 状态 + wc -l + grep 关键行 + job.log 核对）。
8. **验收通过的客观标准**：① 统计基线不变；② 判果来自真实观测、附诊断；③ 期望值与需求一致；④ 有授权链（PM/Owner 裁决 + 回归 + 第三方复核）；⑤ 零污染、G-1/G-2/G-3 不混入 C 套件。
9. **需 ChatGPT 独立复核的内容**：4 条 FAIL 归类（需求待裁决 ×3 / 测试口径 ×1，产品缺陷 0）；「2 硬编码 + 2 静态登记」文档层结论与源码一致性；I-BOOKS-09 硬编码 PASS 使 `164` 含无断言项；工作线 A/B 隔离完整性。
10. **对 R0 放行的影响**：**R0 仍 BLOCK**；4 条 FAIL 只能表述为「C 套件在现有 fixture+runner 登记口径下的 FAIL 条数」，不得表述为「产品缺陷 = 4」；G-1/G-2/G-3 不构成放行依据。

### BLOCKER-05 · A05 代码整改与 CI 证据覆盖

1. **唯一编号与当前状态**：A05 慢网场景整改**已完成并 CI 验证**（代码 `ad34df1`，CI run `42467412803`，A 11/11 success）。当前 HEAD `06e077f` 仅为文档修订，**未改 A05 代码**，故无针对 `06e077f` 的新 CI 必要。
2. **对应需求条款、代码文件及具体行号**：
   - `tools/r0/e2e/specs/a-scenarios.js:168–194` — A05 块：`Network.emulateNetworkConditions` 设 `latency:800`；`:184–186` `page.waitForFunction(() => {...}, null, { timeout: 4000 })` 轮询取回 `{text, disabled}`；判定增 `pending.disabled === true`。
   - 提交 `ad34df1c8854f792231ed5152b61e2e22d115e8b`（A05 块 `+8/−4`）。
3. **当前问题的可核查证据**：CI run `42467412803` head=`ad34df1`，`SUMMARY {"A:PASS":11,"total":11}`；最近 15 条 CI runs 最新 success 为 `b4309db`（main push），**无覆盖 `06e077f`/`ad34df1` 的 run**——这是正常（06e077f 是纯文档提交）。**不得把 `ad34df1` 的 A 11/11 冒充为 `06e077f` 的验证**。
4. **WorkBuddy 是否有权直接处理**：**是（代码与 CI 已落地）**；但「确认覆盖关系」需第三方复核背书（见 C 类）。
5. **需 Owner 裁决的具体问题**：无——A05 代码整改与 CI 已闭环，仅待第三方复核确认。
6. **最小修复方案**：无需修复；若需最新 HEAD CI 证据，可在 `ad34df1` 上派发 `r0-e2e.yml`（`workflow_dispatch`，`ref=fix/r0-sixth-runtime-defects`），回读 `run.head_sha` 校验。
7. **可重复执行的验收步骤**：`git show ad34df1:tools/r0/e2e/specs/a-scenarios.js` 核对 A05 块；`GET /actions/runs/42467412803` 核对 `head_sha` + `conclusion=success`。
8. **验收通过的客观标准**：A05 断言含 `pending.disabled === true` 真实轮询；CI run `conclusion=success` 且 `head_sha=ad34df1`；不把旧 CI 结果归属错误 HEAD。
9. **需 ChatGPT 独立复核的内容**：A05 慢网断言缺口是否已实质闭合；CI 覆盖与代码提交对应关系。
10. **对 R0 放行的影响**：A05 已闭环（属 §8 逐路由测试矩阵一部分）；不阻塞 R0，但复核确认前不得在报告中宣称「A 套件全绿即 R0 通过」。

### BLOCKER-06 · D-4 期初余额（方案定稿、功能未实现）

1. **唯一编号与当前状态**：`D-4`（期初余额技术模型）。`docs/FastLedger-R0-收尾执行清单.md:37` —「技术方案已确定；功能未实现；实施前需按 D-4 文档完成源码核对与测试设计」。
2. **对应需求条款、代码文件及具体行号**：`docs/FastLedger-R0-D4-期初余额技术模型裁决草案.md`（Owner 已定稿）；收尾清单 L37、L132（`R0-E-06 期初余额：PRD 已要求；D-4 技术模型已定稿，但尚未实现`）、L158。
3. **当前问题的可核查证据**：收尾清单 L20「R0 通过 ≠ 把所有 R0-E 功能全部开发完；期初余额…需在 R0 中完成准确取证、需求追溯、技术契约和后续实施拆分；业务实现按经批准的后续阶段执行」。
4. **WorkBuddy 是否有权直接处理**：**否**（功能实现超出 R0 门禁范围，属后续阶段）。
5. **需 Owner 裁决的具体问题**：是否将 D-4 实现纳入 R0 阶段，还是明确划为 R0 之后的后续阶段（当前文字倾向后者）。
6. **最小修复方案（仅描述）**：若纳入 R0——按 D-4 文档补充 `opening_balance` + `opening_balance_at` 实现、跨时区/夏令时/边界测试；若划出——在收口文档明确「R0 不实现 D-4，仅完成技术契约冻结」。
7. **可重复执行的验收步骤**：`grep -n "期初余额\|opening_balance" docs/FastLedger-R0-收尾执行清单.md` 核对状态记载。
8. **验收通过的客观标准**：D-4 状态在文档中单一口径（已定稿 + 是否纳入 R0 实施），无「已实现」误导表述。
9. **需 ChatGPT 独立复核的内容**：D-4 是否应作为 R0 阻塞项，还是技术契约冻结项。
10. **对 R0 放行的影响**：若划出后续阶段，则不构成 R0 阻塞；若纳入，则需补实现与测试才放行。

### BLOCKER-07 · D-8 备份区块（方案定稿、功能未实现、策略冲突）

1. **唯一编号与当前状态**：`D-8`（服务器/备份区块）。收尾清单 L38「技术方案已确定；功能未实现；保留策略冲突待 Owner」；L121「保留策略暂按 PRD：每日 1 份、保留 7 份；与部署交接 14–30 份冲突，需单一权威口径；待 Owner 确认」。
2. **对应需求条款、代码文件及具体行号**：`docs/FastLedger-R0-D8-服务器备份区块裁决草案.md`；收尾清单 L77–90（§3.2 D-8 纠正「已部署」与保留策略冲突）；L38、L121。
3. **当前问题的可核查证据**：D-8 文档 §7.1 曾把 cron `sqlite3 .backup` 写成「已部署」；PRD §12 决策「每日 1 份、保留 7 份」与部署交接 §7.2「14–30 份」冲突；真实 manifest / integrity_check / SHA-256 / 原子发布 / failed 记录 / restore drill 均未真实部署验证。
4. **WorkBuddy 是否有权直接处理**：**否**（功能实现 + 真实部署验证超出 R0 门禁）。
5. **需 Owner 裁决的具体问题**：① 保留份数单一权威口径（7 vs 14–30）；② 是否将 D-8 实现纳入 R0。
6. **最小修复方案（仅描述）**：以 PRD §12 为当前权威（每日 1 份、保留 7 份）；若改 14–30 须先修订 PRD 并同步 D-8 / 部署交接；真实 manifest 与恢复演练按后续阶段。
7. **可重复执行的验收步骤**：`grep -n "保留\|份\|backup" docs/FastLedger-R0-收尾执行清单.md docs/FastLedger-R0-D8-*.md` 核对口径一致性。
8. **验收通过的客观标准**：PRD / D-8 / 部署交接三处保留策略一致；D-8 §7.1 不得写「已部署」无证据。
9. **需 ChatGPT 独立复核的内容**：D-8 是否应作为 R0 阻塞项。
10. **对 R0 放行的影响**：同 BLOCKER-06；属「方案定稿未实现」，按 Owner 裁决划阶段。

### BLOCKER-08 · G-1 / G-2 / G-3 范围外已确认缺口（独立工作线）

1. **唯一编号与当前状态**：三条**范围外已确认缺口**，独立工作线 B，只登记、未实施、未授权验收：
   - `G-1`：PRD §12.1 `[1.12]`（`PRD.md:581–587`）邀请链接入户注册流程 Web 0 实现。
   - `G-2`：D-3 顶栏账本切换器（`#navBooks` 永久 hidden，收尾清单 L115 已裁决，但未落实）。
   - `G-3`：首页「结余」用账户余额合计作「净资产」（与 Owner 已定「本月收入−本月支出」不符，PR #4 `16705fd` 已确认口径）。
2. **对应需求条款、代码文件及具体行号**：
   - `docs/FastLedger-R0-C套件剩余4条FAIL-需求与验收口径独立审计报告.md` §0.8.1（工作线 B 表）、§3.10（`G1-V-01..11` / `G1-R-01/02`）、§6.1.1（`G2-V-01..03` / `G2-R-01`）。
   - 后端 `join` 路由 `server/internal/api/router.go:128` 公开；前端未接（审计报告取证）。
3. **当前问题的可核查证据**：审计报告 §0.8.1「工作线 B **独立编号** `G1-V-*`/`G2-V-*`；**严禁借用任何 `I-*` 用例 ID**；不写入 `c-controls.js`、不改 `controls.json`」。`grep` 在 `controls.json`/`c-controls.js` 中 `G1-V-|G2-V-` 均为 **0 命中**（隔离成立）。
4. **WorkBuddy 是否有权直接处理**：**否**（实现与验收须另行书面授权）。本轮只登记。
5. **需 Owner 裁决的具体问题**：是否授权 G-1/G-2/G-3 的实现与独立验收（独立脚本 + 独立证据目录 + 独立留痕）。
6. **最小修复方案（仅描述）**：另行授权后，按审计报告 §3.10 / §6.1.1 的独立编号与验收表实施；不与 C 套件 4 FAIL 混用编号、不抵消、不合并。
7. **可重复执行的验收步骤**：`grep -c "G1-V-\|G2-V-" tools/r0/e2e/fixtures/controls.json tools/r0/e2e/specs/c-controls.js` 期望 0（确认不混入 C 套件）。
8. **验收通过的客观标准**：G-1/G-2/G-3 各自独立验收通过、独立留痕；C 汇总 `164/4/59/1/228` 不受影响；两线不共用编号/结果。
9. **需 ChatGPT 独立复核的内容**：工作线 B 隔离完整性；G-3 首页结余口径偏差是否应 upgrade 为 BLOCKER。
10. **对 R0 放行的影响**：范围外缺口说明「4 条 FAIL 不是产品状态上界」，但**不构成 R0 可放行依据**；其实施须授权后单独闭环。

### BLOCKER-09 · R0 §8 整体放行条件（9 条全未勾选）

1. **唯一编号与当前状态**：`R0-RELEASE-GATE`。收尾清单 L179–191「最终 R0 放行条件」9 条全部 `[ ]` 未勾选；结论「**仍为 BLOCK**」。
2. **对应需求条款、代码文件及具体行号**：`docs/FastLedger-R0-收尾执行清单.md:181–189`：
   - [ ] 13 个原型页面逐页证据/差异/映射
   - [ ] 228 条控件逐项可观察结果证据
   - [ ] prototype 与实现同视口同状态对照基线（normal/loading/empty/error/success/permission/offline/conflict）
   - [ ] R0-E-35 经 Owner/CI 独立复跑并修复/风险接受
   - [ ] API 权限/DTO/错误/测试矩阵完整 + race 结果可追溯
   - [ ] D-3/D-5/D-6 等决策已由 Owner 明确（D-2 PWA OCR 已排除）
   - [ ] RV-05 分项如实记录
   - [ ] README/CHANGELOG/主报告/D-4/D-8/截图索引/回执状态一致
   - [ ] Owner 审阅最终验收报告并明确授权 R1
3. **当前问题的可核查证据**：L191「2026-11-05 已关闭 D-3/D-5/D-6/D-2/RV-04 的 Owner 决策项；这不代表截图、逐控件运行时、R0-E-35、API/race 测试等 R0 门禁已完成」。
4. **WorkBuddy 是否有权直接处理**：**部分**。WB 可执行截图基线补齐、写验收报告草稿、补 API/race 测试（需授权）；Owner 签字与决策不可替代。
5. **需 Owner 裁决的具体问题**：上述 9 条中哪些可在 R0 内闭环、哪些明确 defer 到后续阶段；最终是否签字放行 R1。
6. **最小修复方案（仅描述）**：逐条对照 §8 清单，能由 WB 执行的（截图基线、测试矩阵、报告草稿）先执行并留证；需 Owner 的（决策确认、签字）单独提请。
7. **可重复执行的验收步骤**：`grep -n -E '^\- \[ \]' docs/FastLedger-R0-收尾执行清单.md` 列出未勾选项；逐项核对证据目录。
8. **验收通过的客观标准**：9 条中 R0 范围内项全部 `[x]` 且有证据；Owner 签字；无未关闭门禁、无未裁决业务阻塞。
9. **需 ChatGPT 独立复核的内容**：放行条件完整性；是否遗漏本任务单 BLOCKER-01~08 之外的项。
10. **对 R0 放行的影响**：**直接决定 R0 是否放行**；当前 9 条全未勾选 → R0 BLOCK。

---

## 3. 分类总表、执行顺序与依赖

### 3.1 A / B / C / D 分类

| 分类 | 含义 | 包含阻塞项 |
|---|---|---|
| **A · WorkBuddy 可直接执行** | 获书面授权后 WB 可动手，无需 Owner 新决策 | BLOCKER-04（runner 真实断言整改，动作本身 WB 可做，状态变更需裁决）、BLOCKER-09（截图基线/测试矩阵/报告草稿） |
| **B · 必须 Owner 决策或操作** | 涉及产品语义/仓库安全/可见性/阶段划分，WB 无权自行决定 | BLOCKER-01（Private/历史清理）、BLOCKER-02（t021 meal/dinner）、BLOCKER-03（改 style.css 授权）、BLOCKER-06（D-4 阶段划分）、BLOCKER-07（D-8 策略/阶段）、BLOCKER-08（G-1/G-2/G-3 实施授权） |
| **C · 必须第三方独立复核** | 状态变更/验收结论须 ChatGPT 等独立复核背书 | BLOCKER-04（状态变更复核）、BLOCKER-05（CI 覆盖复核）、BLOCKER-09（验收报告复核） |
| **D · 证据足够，无需继续修改** | 现状证据已闭环，只需如实记录 | BLOCKER-05（A05 代码 `ad34df1` + CI run `42467412803` 已覆盖；`06e077f` 仅文档无新 CI 必要） |

### 3.2 建议执行顺序与依赖

```
阶段 0（当前）：R0 BLOCK，本任务单已落盘。所有 BLOCKER 状态如实登记。

阶段 1 · Owner 决策（B 类，无依赖，可并行提请）：
  BLOCKER-01 → Owner 改 Private + 评估历史清理
  BLOCKER-02 → Owner 裁 t021（meal/dinner）
  BLOCKER-03 → Owner 裁是否修 style.css:331
  BLOCKER-06 → Owner 裁 D-4 是否纳入 R0
  BLOCKER-07 → Owner 裁 D-8 保留策略 + 是否纳入 R0
  BLOCKER-08 → Owner 裁 G-1/G-2/G-3 实施授权
  （以上任一项无 Owner 裁决，对应项保持原状态，不得 WB 自决）

阶段 2 · WorkBuddy 执行（A 类，依赖阶段 1 授权）：
  BLOCKER-04 → 获书面授权后改 runner 真实断言（不碰计数/状态）
  BLOCKER-09 → 补截图基线、API/race 测试、写验收报告草稿

阶段 3 · 第三方独立复核（C 类，依赖阶段 2 产出）：
  BLOCKER-04 / 05 / 09 → ChatGPT 复核断言整改、CI 覆盖、放行报告

阶段 4 · 放行（依赖全部阶段 1–3 闭环 + Owner 签字）：
  BLOCKER-09 第 9 条 Owner 签字 → 建议 R1
```

**关键依赖**：B 类是所有进展的前置（Owner 不裁决则 A/C 无授权基础）；A 类产出是 C 类输入；C 类通过 + Owner 签字是放行唯一条件。**R0 不在任何单条 BLOCKER 闭环后自动放行**——须 §8 九条全满足且 Owner 签字。

---

## 4. 边界与诚实声明

- 本轮**仅重新核查、整理证据、编写本任务单**。未修改任何业务代码 / CSS / HTML / 数据库 / API / 测试判定 / fixture / `controls.json` / `c-controls.js` / PRD / Owner 决策 / 既有验收口径 / `164/4/59/1/228` 统计。
- 未合并 PR #3 / PR #4；未 rebase、未 force-push、未改写历史；未启动 R1–R7 / UI-DESIGN-02 / PWA。
- 所有「运行时」结论均引自仓库内**历史已落盘证据**或**本轮只读源码/文档/CI REST 核验**，未启动浏览器/服务做新增运行时验证（OBS-ACCT-VIS-390 专属截图证据质量不足项已如实标注，待重采）。
- **R0 仍为 BLOCK**；本任务单使剩余阻塞项清晰、可执行、可验收，但**不构成 R0 通过依据**，亦不得由 WB 自行宣布 R0 通过。
- 待 ChatGPT 独立复核本任务单后，再按结论决定下一步；G-1/G-2/G-3 实施、C 套件状态变更、style.css 修改等均须后续明确书面授权。
