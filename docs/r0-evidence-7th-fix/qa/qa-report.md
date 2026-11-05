# QA 独立复核报告 · FastLedger R0 第六次自动化验收缺陷修复

> 复核人：Edward（QA 工程师，独立复核方）
> 分支：`fix/r0-sixth-runtime-defects`；基准 main：`dadc9a3b9c567cad2bbad7e47361fb774365b949`；**初次独立复核阶段（09:59–10:02Z）的 HEAD = main**。
> **阶段说明（重要）**：收尾期间 team-lead 创建了 commit `0472484b3e57dac4cd05240ac40fb793ad3a2b4d`（`fix(r0): 有限范围修复第六次验收 4 项业务缺陷 + 独立回归门禁`，2026-11-05T18:18:00+08:00），此后 **HEAD 已变为 `0472484`、工作树 diff 已清空**。本报告第 2~4 节及 `scope-audit.md` 第 1~7 节的所有 git / diff / 重跑观测，均基于**该 commit 之前**的工作树（当时 HEAD 即 main），属历史观测；按当前仓库复现请改用 `git show 0472484` 或 `git diff dadc9a3..0472484`。详见 6.6。
> 复核方式：**重新构建 → 新证据目录 `artifacts/r0-qa-verify` → 重新起干净实例 → 重新跑 A/D/C**；全部结论以 QA 自己跑出的原始日志、`report.json` 与真实退出码为准，不采信工程师自述与 `after/` artifacts。
> 复核时间：2026-11-05T09:59Z ~ 10:02Z（UTC）
> 证据根目录：`docs/r0-evidence-7th-fix/qa/`

---

## 1. 总体结论

| 项 | 结论 |
|---|---|
| 本轮 4 项授权修复 | **3 项业务代码修复经独立重跑全部验证通过；第 4 项（新增账户）确认业务 JS 无缺陷，旧问题源于 E2E 判定口径 + `style.css` L331 窄屏隐藏首列** |
| 智能路由判定 | **NoOne**（详见第 5 节） |
| 是否发现源码 Bug | **否** —— 无需回给 Engineer |
| 是否需要 QA 自行修测试代码 Bug | **否** —— 初次独立复核未改测试代码；收尾阶段经 team-lead 授权仅做两处纯文案修正（属文案整理、非 Bug 修复，判定未动，见 6.5） |
| `I-ACCT-06` | **保持 FAIL**（team-lead 裁决保留的边界项，视觉可见性未闭环，待 Owner 裁决 CSS） |
| 与工程师结果差异 | **计数、状态分布、退出码逐项一致，无差异**；但 QA 额外发现 4 项证据质量/流程问题（第 6 节） |
| 越界修改 | **无**（详见 `qa/scope-audit.md`；有一处需 team-lead 确认的 workflow 边界项） |

---

## 2. 独立重跑结果（QA 自己的证据）

### 2.1 构建

```
source /c/Users/47006/WorkBuddy/fastledger/.tools/genv.sh
go build -o fastledger.exe ./cmd/server
=== EXIT_CODE=0 ===
```
- `qa/build.log`：go1.22.5 windows/amd64、CGO_ENABLED=1、gcc(w64devkit)、构建无输出无报错，产物 `server/fastledger.exe` 时间戳更新。

### 2.2 三套件（每条命令均以独立临时 SQLite + 随机端口 + 新 JWT 密钥启动）

| 套件 | 命令（共同前缀 `R0_SERVER_BIN=…/server/fastledger.exe R0_BROWSER_CHANNEL=msedge R0_ARTIFACT_DIR=…/artifacts/r0-qa-verify`） | 实例 | 结果 | **真实退出码** | 日志 |
|---|---|---|---|---|---|
| A | `node run.js --suite a` | `127.0.0.1:65208` | `A:PASS 11 / total 11` | **0** | `qa/a-qa.log` |
| D | `node run.js --suite d` | `127.0.0.1:60064` | `D:PASS 39 / total 39` | **0** | `qa/d-qa.log` |
| C | `node run.js --suite c` | `127.0.0.1:58520` | `PASS 163 / FAIL 5 / BLOCKED 59 / N/A 1（total 228）` | **1** | `qa/c-qa.log` |

- 浏览器统一 `playwright-core 1.64.1 / channel=msedge / 156.0.4314.58`。
- C 的退出码 1 来自 `run.js` 的 `fails.length` 规则（5 条 FAIL），workflow 中 C 为 `continue-on-error: true`（报告型），D 为硬门禁且无 `continue-on-error`。
- 三份 `report.json` 已复制为 `qa/a-report.json`、`qa/d-report.json`、`qa/c-report.json`。

### 2.3 测试通过率（多口径，避免单一数字误导）

| 口径 | 数值 |
|---|---|
| 包 A | **11/11 = 100%** |
| 包 D（本轮 4 项缺陷回归门禁） | **39/39 = 100%** |
| 包 C（全 228 条逐控件） | **163/228 = 71.5%**；剔除 59 条 BLOCKED 后 **163/169 = 96.4%** |
| 三套件合计 | PASS 213 / 278 = **76.6%**；FAIL 5、BLOCKED 59、N/A 1 |
| 本轮 4 项授权修复相关用例（A 全量 + D 全量 + C 的 I-TXNS 19 条 + I-ACCT-06） | 全部按预期：修复项 100% PASS，`I-ACCT-06` 按裁决保持 FAIL |

> 说明：C 套件本身就是「报告型」——228 条覆盖全产品控件，其中大量 BLOCKED/FAIL 是**历史登记的产品缺口**，不是本轮回归失败；本轮修复的验收口径以 A + D + C 中相关控件为准。

---

## 3. 逐项验收结论摘要（详见 `qa/acceptance-matrix.md`）

| 授权项 | 结论 | 关键观测 |
|---|---|---|
| ① 交易总表 `view.append(card)` + I-TXNS-19 | **PASS** | 3 行真实数据（备注「工资/打车/早餐」）、空态、注入 500 后可见反馈 + `aria-busy=0` + `.skel=0` + toast；**I-TXNS-01..19 由「01 FAIL + 02..19 BLOCKED」闭环为 C/D 两套各 19/19 PASS** |
| ② `openTxForm` 分类过滤 / 切换重置 / `categoryId` 校验 | **PASS** | 收入下拉 8 项 = API `catType=1` 8 项、支出下拉 22 项 = API `catType=2` 22 项、混入 0；切换类型重置到对应类首项；支出 POST 200 + 请求体/API 回读 `txType=2,categoryId=1` 一致 + 刷新后仍在；转账 `categoryId=0` 语义未变 |
| ③ `renderMembers` 角色门控 + 失败关闭 | **PASS** | owner/admin 按钮=1 且弹窗可开；member 按钮=0 且 view/body 文本均无「邀请成员」；member 直连 POST invite → **403**；`/users/me`=500 时 owner 也不显示入口且成员列表仍 3 行；角色徽标「所有者/管理员/普通成员」与其它管理功能不受影响 |
| ④ 新增账户 | **业务 JS 无缺陷（PASS）；视觉可见性 FAIL（保持）** | POST 200、行数 3→4、`td.textContent` 命中、API 回读、刷新后仍在、无重复、DOM↔API 全量一致；但 `visibleInnerText=false`、`firstTdDisplay=none` |

### 3.1 关于第 4 项的独立判定（team-lead 特别要求）

- QA 独立核验 `server/web/dist/style.css` **L331**：`@media (max-width: 640px) { th:nth-child(1), td:nth-child(1), … { display:none; } }` —— 390px 视口必然命中；`qa/screenshots/15-accounts-list-first-col-hidden.png` 直观显示账户表表头只剩「类型/余额/操作」，名称列被隐藏。
- `baseline/account-repro.md` 的 3 次复现（POST 200 / API 有 / `textContent` 有 / `innerText` 无 / 刷新后仍不可见 / 无 console error）与 QA 本次重跑的 `firstTdDisplay=none` 相互印证。
- **「390px 视觉可见性未闭环，`I-ACCT-06` 保持 FAIL」** 已明确写入 `qa/acceptance-matrix.md` 第 4 节。
- **没有任何「换口径制造 PASS」**：`I-ACCT-06` 在 `c-controls.js` 中是唯一赋值、**无条件 `bad()`**、detail 与 `visibility-observations.json` 保留 `visibleInnerText` / `firstTdDisplay` 两个原始观测；C 报告 FAIL 恰为 5；D 的 `D-ACCT-01..04` 是**另立口径的新增用例**（明确标注「DOM 渲染一致性，不含视觉可见性」），不是对既有 FAIL 的改判。QA 未修改 `style.css`（未获 Owner 授权）。

### 3.2 特别核对项（任务书点名）

| 核对项 | 结果 |
|---|---|
| `I-ACCT-06` 在 `c-controls.js` 中无条件 FAIL | **是**（全文仅 L383 一处 `bad(...)`，无 PASS 分支） |
| detail 含 `visibleInnerText` 与 `firstTdDisplay` | **是**（C 日志原文 `visibleInnerText=false；firstTdDisplay=none`） |
| C 报告 FAIL = 5 | **是**（`I-LOGIN-04`、`I-LOGIN-11`、`I-LOGIN-12`、`I-HOME-01`、`I-ACCT-06`） |

---

## 4. 证据与脱敏

### 4.1 已落盘证据清单

| 文件 | 内容 |
|---|---|
| `qa/build.log` | 重新构建日志 + `=== EXIT_CODE=0 ===` |
| `qa/a-qa.log` / `qa/d-qa.log` / `qa/c-qa.log` | 完整 stdout/stderr + 真实退出码（末尾 `=== EXIT_CODE=<n> ===`） |
| `qa/a-report.json` / `qa/d-report.json` / `qa/c-report.json` | 三套件原始报告（逐条 status/detail/evidence） |
| `qa/scope-audit.md` | 范围审计（A） |
| `qa/acceptance-matrix.md` | Owner 验收清单逐项核对（C） |
| `qa/assertion-review.md` | 断言有效性抽检 + 负向合成验证核对（D） |
| `qa/qa-report.md` | 本报告（F） |
| `qa/SHA256SUMS.txt` | 本目录全部证据文件的 SHA256 |
| `qa/screenshots/*.png` | 18 张关键截图（见 4.2，含补充的 `05b`） |
| `qa/05b-shot.log`、`qa/05b-shot-script.js` | 清晰失败态截图的运行日志与脚本副本（见 4.2.1） |

### 4.2 截图覆盖（18 张，均来自 QA 本次 `artifacts/r0-qa-verify`，`05b` 来自同一干净实例的独立补充脚本）

| 验收要求 | 对应截图 |
|---|---|
| 交易总表有数据 | `01-txns-with-data.png`、`17-txns-wide-viewport.png` |
| 空态 | `02-txns-empty.png` |
| 加载失败 | **`05b-txns-load-failure-clear.png`（清晰合格版，team-lead 目检用）**、`05-txns-load-failure.png`（过渡态，标注见 Q-1）、`03-txns-failure-screenshot-mismatch.png`（错配反例，见 Q-1）、`06-txns-skeleton-loading.png`（加载中） |
| 记一笔分类下拉 | `07-category-expense-dropdown.png`、`08-category-type-switch.png`、`09-category-expense-created.png`、`10-category-transfer.png` |
| member 成员页 | `11-members-owner-invite.png`（owner 有入口）、`12-members-admin.png`、`13-members-member-no-invite.png`（member 无入口）、`14-members-me-failed-closed.png` |
| 账户列表 | `15-accounts-list-first-col-hidden.png`（首列被 CSS 隐藏，直观证明 I-ACCT-06）、`16-acct-06-visibility-fail.png` |
| 补充 | `04-txns-row-edit.png`（行编辑） |

### 4.2.1 补充证据：清晰「交易加载失败」截图（按 team-lead 收尾要求新增）

原 `05-txns-load-failure.png` 是 CSS 过渡/遮罩态、几乎全白，看不到失败文字，作为交付证据不合格。QA 用**独立小脚本**重新采集：

| 项 | 值 |
|---|---|
| 脚本 | `tools/r0/e2e/scripts/qa-load-failure-shot.js`（副本 `qa/05b-shot-script.js`） |
| 实例 | 全新干净实例 `http://127.0.0.1:52156`（临时 SQLite + 随机端口 + 每次新生成 JWT 密钥） |
| 浏览器 | `playwright-core` / `msedge` / `156.0.4314.58`；视口 390×844 |
| 日志 | `qa/05b-shot.log`，**`=== EXIT_CODE=0 ===`** |
| 输出 | `qa/screenshots/05b-txns-load-failure-clear.png`（42537 bytes，**新增，未覆盖旧图**） |

方法：`page.route` 在网络层注入 `GET /api/v1/books/{id}/transactions?` → **HTTP 500**（全程不 `unroute`，确保页面停在失败态，规避既有 `D-TXNS-03` 因 fn 末尾 reload 复位导致的截图错配）；用 `waitForFunction` 确定性等待「`#view.innerText` 含『交易加载失败』 + `dialog[open]` 数=0 + `.empty` 已布局 + 骨架屏已清除」；截图使用 Playwright 官方 `animations:'disabled'` 把 CSS 过渡快进到终态。**未使用固定 sleep、未改任何断言、未重跑 A/D/C 全套**。

截图前置自检原始观测（`qa/05b-shot.log`）：
```
errLine        = 交易加载失败：QA 交付截图注入 500（可重新查询或刷新页面）
injectMsgVisible = true     ariaBusyCount = 0     skeletonBars = 0
dialogsOpen    = 0          emptyBoxCount = 1     emptyHeight  = 152.5
viewOpacity    = 1          paragraphOpacity = 1
toast          = ✕ QA 交付截图注入 500
```
QA 目视确认：页面中央 ⚠️ 图标 + 完整失败提示清晰可读，无遮罩/骨架屏，底部红色 toast 可见，无凭据字符串。

### 4.3 脱敏（Public 仓库，硬要求）

- **QA 扫描了 `qa/` 全部已落盘文件**：无 JWT 三段式字面量、无 `Bearer` 凭据值、无服务端 JWT 密钥环境变量值、无裸 `token` 字段残留。
- **trace 未复制，并在此明确说明理由**：QA 解压 `artifacts/r0-qa-verify/…/A01/trace.zip` 后，`grep -a` 在 `trace.network` 中命中真实 JWT 三段式字面量（`[REDACTED]`——凭据原文按脱敏要求不转录到证据文件）。任务书 E 虽写「若框架已生成 trace，也一并复制」，但同一任务书明确要求「不要复制任何含 token/凭据的原始网络日志（仓库是 Public，必须脱敏）」。**QA 以脱敏为最高优先级，决定不复制任何 `trace.zip`**，避免把临时实例 JWT 带进 Public 仓库。脱敏口径参照 `tools/r0/redaction.js`（Authorization / Cookie / JWT / Bearer 均需 `[REDACTED]`）。
- 三套件原始 artifacts 仍保留在 `artifacts/r0-qa-verify/` 供本地复核；**该目录含 `server.log`、trace、临时 SQLite 等，不应随 Public 仓库提交**。

---

## 5. 智能路由判定：**NoOne**

判定依据（严格按「源码 Bug → Engineer；测试代码 Bug → QA 自修；全部通过 → NoOne」）：

1. **A 11/11 PASS、D 39/39 PASS**，本轮 4 项授权修复相关的所有正向断言在 QA 独立干净环境中全部通过，未发现源码 Bug → 不路由 Engineer。
2. **C 的 5 条 FAIL 全部是已知项，不构成源码 Bug**：
   - `I-LOGIN-04`、`I-LOGIN-11`、`I-LOGIN-12`、`I-HOME-01`：主报告已登记的产品缺口（R0-E-39 / R0-E-38 / R0-E-01），非本轮引入；
   - `I-ACCT-06`：**team-lead 明确裁决保留的边界项**（390px 视觉可见性未闭环，待 Owner 裁决 `style.css`），任务书明令「不许修成 PASS」「不构成源码 Bug」。
3. **未发现需要 QA 自行修复的测试代码 Bug**：Q-1（截图时点错配）、Q-2（A08 陈旧文案）属**证据质量/文案问题**，不参与任何判定、不影响结论。初次独立复核阶段 QA 未修改任何测试代码以保持基线纯净；后续 team-lead 明确授权低风险收尾后，QA 只做了两处**纯文案**修改（A08 detail 措辞、workflow job `name`）并新增一个证据脚本，均不触及判定逻辑或 status 计算（见 6.5）。按 team-lead 要求**未重跑 A/D/C 全套**；`a-qa.log` 产生于文案修改前，因 `status: apiOk ? 'PASS' : 'FAIL'` 与 `apiOk` 计算完全未变，A 11/11 PASS 的结论不受影响。
4. 因此本轮在「4 项授权修复」范围内 **全部通过 → Send To: NoOne**。

> 边界声明：`NoOne` 仅针对本轮 4 项授权修复的回归结论。C 报告的 5 条 FAIL、59 条 BLOCKED 仍是**未解决的产品缺口**，其中 `I-ACCT-06` 需 Owner 单独裁决 CSS；这些不是回归失败，也不由 QA 在本轮处置。

---

## 6. 新发现问题 / 未解决问题 / 与工程师结果的差异

### 6.1 与工程师结果的差异

| 套件 | 工程师自述 | QA 独立重跑 | 差异 |
|---|---|---|---|
| A | 11/11 PASS (0) | 11/11 PASS，`EXIT_CODE=0` | **无** |
| D | 39/39 PASS (0) | 39/39 PASS，`EXIT_CODE=0` | **无** |
| C | PASS163/FAIL5/BLOCKED59/N/A1 (1) | 同左，`EXIT_CODE=1` | **无** |

**QA 结论以自己跑出的为准**；本次恰好与工程师逐项一致，但这是独立复现的结果，不是采信。QA 额外发现如下工程师报告未提及的问题。

### 6.2 QA 新发现（4 项证据质量/流程问题，均不改变任何判定）

| ID | 问题 | 严重度 | 建议 |
|---|---|---|---|
| Q-1 | 框架 `step()` 在 `fn()` 结束后才截图，凡 fn 内复位/关弹窗/刷新者截图不反映判定瞬间。**`qa/SHA256SUMS.txt` 硬证据：`01`（有数据）/`03`（加载失败）/`04`（行编辑）三张截图 SHA256 完全相同 `6324b0fa…`**；`I-TXNS-14/18` 截图为关闭/复位后状态；`I-TXNS-19`、`I-ACCT-06-visibility`、`D-ACCT-01` 呈极淡或全白 | 中（证据可读性） | **【未解决 · 按 team-lead 裁决如实登记】** 不改 `step()` 框架。QA 已**二次源码复核确认判定不依赖截图**：`r.status` 在 `fn()` 返回时即确定，截图在其后执行且失败被 `catch` 忽略，仅作 `evidence` 附加；`d-defects.js` L103 与 `txns-controls.js` L117 均用 `r.status`。另已补清晰失败态截图 `05b-txns-load-failure-clear.png`（见 4.2.1）。建议后续把截图移到判定采集点 |
| Q-2 | `a-scenarios.js` L256 A08 detail 曾含修复前遗留硬编码文案「UI 未隐藏=真实缺陷，已记录」，与本次 `inviteBtn=0` 语义矛盾（不参与判定） | 低（误导性文案） | **【已修复】** 改为「成员管理接口 403 校验通过；UI 入口可见性另由 D 套件 D-MEM-* 覆盖」；`git diff` 确认仅 detail 字符串 1 行，`status` / `apiOk` / `evidence` 未动，`node --check` 通过 |
| Q-3 | `trace.zip` 含真实 JWT | **高（安全/脱敏）** | **【保持脱敏口径，不复制】** 已在 4.3 明确不复制任何 trace；本轮新增的脚本 / 日志 / 截图同样经脱敏扫描无凭据字面量。建议框架侧接入 `tools/r0/redaction.js` 对 trace 网络层脱敏后再上传 artifact |
| Q-4 | `.github/workflows/r0-e2e.yml` L28 job 名原为 `A(11 场景) + C(228 控件) + B(分类审计)`，未含 D | 低（文案） | **【已修复】** 改为 `A(11 场景) + D(缺陷回归) + C(228 控件) + B(分类审计)`；`git diff` 确认仅 `name` 一行，所有 step / id / 触发条件 / `continue-on-error` 未动 |

### 6.3 负向合成验证的局限（详见 `qa/assertion-review.md` 第 2.3 节）

- 它是「合成观测 + 文字推断」，**未在修复前 app.js 下真的运行 D/C 套件**，用例级 FAIL 是推断而非实测；
- `after/negative-check.md` 收尾句「上述 4 组观测值分别使对应用例 FAIL」**对 NEG-4 不准确**：`D-ACCT-01..04` 在修复前也会 PASS（第 4 项业务 JS 零改动，NEG-4 自身记录 `POST=200 / textContentHasName=true`），真正会 FAIL 的是旧 C 的 `I-ACCT-06`；
- 浏览器 channel/版本未写入 `negative-check.json`，md 声称「真实 msedge」只能间接推断；
- 上述局限**不推翻**核心结论：NEG-1/2/3 的修复前观测与 QA 修复后观测形成完整反差，`qa/scope-audit.md` 亦从 diff 层独立确认 3 处源码修复。

### 6.4 未解决问题（需 Owner / team-lead 裁决）

1. **`I-ACCT-06` / 390px 视觉可见性**：`style.css` L331 隐藏首列，用户看不到新增账户名；本轮未获授权改 CSS，**保持 FAIL，待 Owner 裁决**。
2. **C 的另外 4 条 FAIL + 59 条 BLOCKED**：既有产品缺口/数据前置缺口，非本轮范围。
3. **`.github/workflows/r0-e2e.yml` 改动边界**：该文件由工程师修改（新增 D 门禁），任务书硬约束字面把 `.github/workflows/` 列入「不得修改」清单，但同任务书又要求审计该 diff，QA 已在 `qa/scope-audit.md` 2.1 如实标注为**需 team-lead 确认的边界项**。后续 team-lead 明确授权 QA 只改 job `name` 一行（Q-4），QA 据此执行且未动任何 step/id/触发条件；**该文件的其余改动仍属工程师产出，边界确认仍请 team-lead 给出**。
4. **提交状态已变化（详见 6.6）**：初次复核阶段改动全在工作树未提交；收尾期间 team-lead 已创建 commit `0472484` 将大部分改动入库（含 QA 的两处纯文案修正与全部 `qa/` 证据）。**QA 未执行该 commit、未 push、未合并 main**。`qa/qa-report.md`、`qa/scope-audit.md`、`qa/assertion-review.md`、`qa/SHA256SUMS.txt` 共 4 个文件在 commit 之后仍有未暂存更新，需 team-lead 决定是否一并纳入。另：`artifacts/r0-qa-verify/`（含 JWT/临时库）**不得**进入 Public 仓库——QA 已核查 commit `0472484` **未包含** artifacts / trace.zip / server.log / .db，HEAD 树文本文件中亦无 JWT 字面量。
5. **Q-1 未解决**（按 team-lead 指示登记为「未解决的证据质量问题」，不改 `step()` 框架）；**Q-3 脱敏口径保持**（不复制 trace）；Q-2 / Q-4 已修复。均不阻塞本轮验收。

### 6.5 team-lead 授权的低风险收尾（本轮追加，不改变任何判定）

| 动作 | 具体改动 | 边界核验 |
|---|---|---|
| ① 补清晰失败态截图 | 新增 `tools/r0/e2e/scripts/qa-load-failure-shot.js`；产出 `qa/screenshots/05b-txns-load-failure-clear.png`、`qa/05b-shot.log`、`qa/05b-shot-script.js` | 独立干净实例；route 注入 500 不 unroute；确定性等待 `dialog[open]=0`；`animations:'disabled'`；**EXIT_CODE=0**；未覆盖旧图；详见 4.2.1 |
| ② Q-2 文案 | `tools/r0/e2e/specs/a-scenarios.js` L256 仅改 detail 字符串 | `git diff` = 1 行；`status: apiOk ? 'PASS' : 'FAIL'` 与 `apiOk` 计算未动；`node --check` 通过 |
| ③ Q-4 文案 | `.github/workflows/r0-e2e.yml` L28 仅改 job `name` | `git diff` = 1 行 `name` 变化；D step / A step / C `continue-on-error` / 触发条件 / step summary 均未动 |
| ④ Q-1 二次确认 | 复核 `d-defects.js` `step()` L93-104、`txns-controls.js` `step()` L102-118 | `r.status` 在 `fn()` 返回时确定；截图在其后且失败被 `catch` 忽略、只作 `evidence`；**判定完全不依赖截图**；按指示不改框架 |

- **未重跑 A/D/C 全套**（team-lead 明确要求）。`qa/a-qa.log` 等三份日志与 `report.json` 均产生于上述纯文案修改之前；因两处修改不触及任何判定逻辑、status 计算或断言，`A 11/11 PASS`、`D 39/39 PASS`、`C PASS163/FAIL5/BLOCKED59/N/A1` 的结论不受影响。此处如实标注时间差，不隐瞒。
- 新增文件脱敏扫描：无 JWT 三段式字面量、无 `Bearer` 凭据值、无合成口令、无登录令牌字段残留。
- QA 侧**仍未** push、**未** 执行 commit、**未** 合并 main（收尾期间的 commit 由 team-lead 创建，见 6.6）。

### 6.6 仓库提交状态变化（QA 收尾期间发现，如实补记）

在 QA 完成上述收尾动作期间，**team-lead 创建了新 commit**：

```
commit    0472484b3e57dac4cd05240ac40fb793ad3a2b4d
author    FastLedger Dev
date      2026-11-05T18:18:00+08:00
subject   fix(r0): 有限范围修复第六次验收 4 项业务缺陷 + 独立回归门禁
文件数    66
```

| 影响 | 说明 |
|---|---|
| HEAD | 由 `dadc9a3b9…`（基准 main）变为 `0472484`；`git diff`（工作树）已清空，业务代码 diff 需改用 `git show 0472484` 或 `git diff dadc9a3..0472484` 查看 |
| 已入库 | 工程师的 4 类改动（业务 JS / 测试代码 / workflow D 门禁 / 新证据目录）、QA 的全部 `qa/` 证据、QA 的两处纯文案修正（A08 detail、workflow `name`）、`tools/r0/e2e/scripts/qa-load-failure-shot.js`、`05b-*` 证据 |
| **commit 中的边界外文件** | `README.md`（+1）、`CHANGELOG.md`（+11）、以及 `docs/r0-evidence-7th-fix/` 下一份中文名复核文档（+203）。**这些由 team-lead 侧加入，QA 未参与、未修改、未审核其内容，不替其背书**。`qa/scope-audit.md` 第 2.2 节「README/CHANGELOG 均未被触碰」是**初次独立复核阶段（commit 之前）**的观测，须按此阶段限定理解 |
| 仍未暂存（commit 之后的 QA 更新） | `qa/qa-report.md`、`qa/scope-audit.md`、`qa/assertion-review.md`、`qa/SHA256SUMS.txt` 共 **4 个**文件（`git status` 显示 `M`）。**请 team-lead 决定是否一并纳入交付**；QA 不执行 `git add` / commit / push |
| 脱敏复核 | `git show --name-only 0472484` 与 `git ls-tree -r HEAD` 均**无** `artifacts/`、`trace.zip`、`server.log`、`fastledger.db`；`git grep HEAD -- '*.json' '*.log' '*.md' '*.txt'` **未发现 JWT 三段式字面量**。Q-3 的脱敏口径在提交层面同样守住 |

> 本补记只更新仓库状态事实，**不改变**本报告任何 PASS/FAIL 判定、验收结论或 `NoOne` 路由。

---

## 7. 复核边界声明（QA 自证）

- **初次独立复核阶段未修改**：`server/web/dist/app.js`（业务代码）、`server/web/dist/style.css`、Go 后端、PRD/WBS、DB/API、历史证据 `docs/r0-evidence-6th/` 及更早、README/CHANGELOG、`.github/workflows/`、以及 `tools/r0/e2e/` 下任何测试代码。
- **后续按 team-lead 明确授权的低风险收尾（仅文案/证据，见 6.5）**：① `tools/r0/e2e/specs/a-scenarios.js` A08 detail 文案（Q-2）；② `.github/workflows/r0-e2e.yml` job `name` 一行（Q-4）；③ 新增 `tools/r0/e2e/scripts/qa-load-failure-shot.js` 与 `qa/05b-*` 证据。**业务代码、CSS、Go 后端、断言逻辑、status 计算、`step()` 框架、控件清单始终未动**。
- **未做**：放宽/删除/跳过任何断言、把 FAIL/BLOCKED 改判 PASS、延长 sleep 制造通过、用工程师 artifacts 作为结论、重跑 A/D/C 全套、启动 R1–R7、启动 PWA、合并 main、push、提交 commit。**（以上均指 QA 自身行为；仓库中的 commit `0472484` 由 team-lead 创建，见 6.6）**
- **只新增**：`docs/r0-evidence-7th-fix/qa/` 下的证据文件（本报告及 6.1 前所列全部文件）。
- **可复现性**：`qa/*-qa.log` 记录了完整命令、实例端口、浏览器版本、逐条输出与真实退出码；三份 `report.json` 保留逐条 detail；`qa/SHA256SUMS.txt` 提供完整性校验。
