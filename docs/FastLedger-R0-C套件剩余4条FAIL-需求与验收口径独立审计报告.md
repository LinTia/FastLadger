# FastLedger R0 · C 套件剩余 4 条 FAIL 需求与验收口径独立审计报告

> **文档性质**：需求与验收口径审计（纯只读取证 + 方案建议），**不是整改实施、不是验收通过声明、不是 R0 放行依据**。
> **交付对象**：PR #3 分支 `fix/r0-sixth-runtime-defects`；提交后停止，等待 ChatGPT 第三方独立复核。
> **编制日期**：2026-11-08
> **取证人**：产品经理（需求/规范/Owner 决策）、架构师（API/前端代码/最小方案）、QA（C 套件验收口径/测试证据）；团队主理人独立回查关键行号后汇编。

---

## 0. 审计元信息与基线

### 0.1 基线快照（开工时实测）

| 项 | 值 |
|---|---|
| PR #3 HEAD | **`ad34df1c8854f792231ed5152b61e2e22d115e8b`** |
| 工作分支 | `fix/r0-sixth-runtime-defects` |
| 工作区 | `git status --short` 为空；`git diff --name-only`、`git diff --cached --name-only` 均为空 |
| `origin/main` | `dff32746e58b3cad0447091c4a6c0860bef06014`（第三方第十三次 A05 独立复核等文档提交；**PR #3 分支未包含，本轮不覆盖、不合并**） |
| A 套件 | 11 / 11 PASS |
| D 套件 | 39 / 39 PASS |
| C 套件 | **164 PASS / 4 FAIL / 59 BLOCKED / 1 N/A（total 228）** |
| R0 门禁 | **仍 BLOCK** |
| 最新正式 CI | run `42467412803`，`head_sha = ad34df1c…`，`event = workflow_dispatch`，`conclusion = success`，job `128690050693`，Artifact `13611998704` |

C 汇总与 4 条 FAIL 明细原始日志（`artifacts/a05-r13-formal-ci/job.log`，该目录被 `.gitignore` 忽略、未入库）：

```
:1063  C 汇总：{"PASS":164,"FAIL":4,"BLOCKED":59,"N/A":1}
:1065  SUMMARY {"C:PASS":164,"C:FAIL":4,"C:BLOCKED":59,"C:N/A":1,"total":228}
:1070    I-LOGIN-04 FAIL 浏览器中不存在「明文切换」控件（主报告登记缺失 R0-E-39）；DOM 无 eye/toggle 类元素
:1071    I-LOGIN-11 FAIL 产品侧未实现（主报告已登记：R0-E-38）；本轮确认浏览器中不存在该控件
:1072    I-LOGIN-12 FAIL 产品侧未实现（主报告已登记：R0-E-38）；本轮确认浏览器中不存在该控件
:1073    I-HOME-01 FAIL 本页无账本切换控件（主报告登记缺失 R0-E-01），顶部仅标题
:1082    包 C 退出码（报告型，success 不代表无 FAIL）: failure
:1178    .../C/I-LOGIN/I-LOGIN-10.png
```

C 套件是**报告型**步骤：`:1074` 该 step `exit code 1`，而整个 job `conclusion=success`。**C step success / CI 全绿 / A 11/11 / D 39/39 均不等于 R0 通过，也不等于 C 套件无 FAIL。**

### 0.2 任务范围与编号口径澄清

任务单表述为「本轮只审计以下四项」而正文列出 **3 个主题**。本报告按 **3 个主题 / 4 条 FAIL** 处理，映射如下，编号无遗漏、无新增：

| 主题 | 任务单主题名 | 对应 C 套件 FAIL | 登记编号 |
|---|---|---|---|
| 一 | 密码明文切换 | `I-LOGIN-04` | R0-E-39 |
| 二 | 邀请码输入、提交及异常反馈 | `I-LOGIN-11`、`I-LOGIN-12` | R0-E-38（另含 `I-LOGIN-10`，状态 BLOCKED，作为前置依赖分析对象，不计入 4 条） |
| 三 | 首页账本切换 | `I-HOME-01` | R0-E-01 |

`I-LOGIN-10`（登录 · 加入账本入口）虽非 4 条 FAIL 之一，但任务单明确要求检查其是否构成前置依赖，故单列分析（§3.6、§5.2），**维持 BLOCKED 不改判 PASS**。

### 0.3 取证方法、范围与三方分工

| 角色 | 范围 | 取证方式 |
|---|---|---|
| 产品经理 | PRD / 两份设计规范 / 原型 / WBS / Owner 决策链 | 当前分支工作区文件；PR #4 分支 `16705fd` 的 `docs/FastLadger-Web-PWA视觉设计规范.md`、`UI-DESIGN-01` 盘点报告、WBS 附录 B 用 `git show <ref>:<path>` 导出到仓库外临时目录只读查看；`origin/main` 的复核报告 / README / CHANGELOG 用 `git show origin/main:<path>` |
| 架构师 | 后端 API 全景 / 前端运行时代码行号 / 最小方案技术要点 | 仅当前 HEAD `ad34df1c`（**未切 PR #4**），只读 `grep` / `sed` / `python json.load` / `git` 只读子命令 |
| QA | C 套件判定机制 / `controls.json` / `c-controls.js` / 测试证据 / 逐项验收标准 | 仅当前 HEAD，只读；`ViewImage` 实看 login / home 截图 |
| 主理人 | 关键行号与路径独立回查 | 逐条 `Read` / `sed -n` / `grep -n` / `python json.load` 复核，见 §0.6 |

**版本一致性**：`docs/FastLedger-Web设计规范.md` 两分支无差异；PRD 两分支仅 §4.2 一行差异（首页「结余」口径），**§4.1 / §4.10 / §12.1 [1.12] 完全一致**，故本报告对邀请与登录条款的引用对两个分支同时成立。WBS **附录 B 仅存在于 PR #4 分支**，当前分支只有附录 A。

**PRD 版本编号统一**：本报告一律写作 **`[1.12]`**（PRD §12.1「关闭公开注册后的成员入户（[1.12] 裁决）」，L581–587）。取证稿中曾出现位数不同的笔误编号写法，经核对为笔误，已统一订正；本报告全文只使用 `[1.12]` 这一种版本编号写法。

### 0.4 判定口径（三选一的操作性定义）

| 分类 | 定义 |
|---|---|
| `确认缺陷` | PRD / Owner 决策 / 设计规范中存在**明确条款**，且实现确实缺失。 |
| `测试口径问题` | 需求条款**已明确存在**，但 C 套件的验收域 / 断言与该条款不一致（含重复要求、编号误挂、把 A 处能力验成 B 处）；或判定本身不是可信的运行时断言。 |
| `需求待 Owner 裁决` | 权威文档中**查无该要求的明确条款**，现有证据只支持「常见 UX 建议 / 实现侧文案 / 二次复核观察项」；是否上升为需求须 Owner 明确决定，未经裁决不得计入已确认缺陷。 |

### 0.5 三层模型、防误读约定与审计范围限制（全报告统一）

三方结论回答的是**不同层次的问题**，不是互相矛盾。全报告统一采用**三层模型**：

| 层 | 内容 | 本轮结论 |
|---|---|---|
| **① DOM／源码事实层** | 控件是否存在（客观、可在 HEAD 逐行验证） | 登录页第三入口／邀请码输入框不存在、前端对 `members/join` 0 调用、无密码显隐切换、`renderHome` 无 select |
| **② 需求条款／验收口径判定层** | 是否有明确条款、验收域是否与条款一致 | **4 条无一可判 `确认缺陷`**；3 条需求待裁决、1 条测试口径问题 |
| **③ 运行时断言证据层** | 当前 FAIL 判定是否为可信的浏览器运行时观测 | **4 条均非可信运行时断言**（2 条硬编码 `bad()`、2 条静态登记；零 DOM 查询、零专属截图、零诊断），测试须**独立于产品裁决**整改 |

> 即：**控件不存在（DOM 事实）＋ 需求条款／验收口径（判定）＋ 当前 FAIL 判定无运行时断言（证据层）**。并与 `docs/FastLedger-R0-基准冻结与差异盘点.md:967` 的口径并用，该行原文为「**D-4 / D-8 仅为技术方案已确定，功能仍未实现、无运行时验收，不改变 R0 阻塞**」；本报告下文把这一原则简称为「已裁决 ≠ 已实现」，属**归纳表述、非逐字引用**。

必须同时保留的防误读条款：

- **「不是确认缺陷 / 测试口径问题」≠「功能已实现」**；**「已裁决」≠「已实现」**（据 `基准冻结.md:967` 归纳，非逐字引用，见 §0.5）。
- **PRD §12.1 的「邀请链接入户注册」能力是【决策】已定，而 Web 端 0 实现**——这是一条**不在 4 条 FAIL 内、也不在 228 条控件清单内**的独立功能缺口（详见 §6.1）。
- **「4 条 FAIL 是 C 套件 runner 的判果，不是产品缺陷登记清单」**（依据：`基准冻结.md:157 / :163` 主报告原文已把 I-LOGIN-04 / I-LOGIN-10 登记为 `**缺失**｜R0-E-39 / R0-E-38`，是 `extract-controls.js` 竖线裸切污染 + runner 降级才改变判果；详见 §5.2、§5.3）。
- **`member.go:615-622` 只约束审计表 `audit_logs.before_json` 的 payload 字段集合**，不是 URL 规则、也不是运行日志规则；「邀请码不得写入 URL」属技术侧前瞻性建议，须 Owner 采纳（§3.11）。

**审计范围与限制（重要）**：本轮三方（产品／测试／技术）**均为静态文档与源码取证，未运行任何测试套件、无运行时验收、无人复跑用例**。报告中所有 `PASS / FAIL / 164` 等数字，均是对**既有登记文档与 `c-controls.js` runner 源码**的静态判读，**不构成运行时证明**；`I-BOOKS-09` 用例状态仍为「待复核」，本轮无人复跑。三方取证稿以团队消息形式交付、未入库；本报告已把全部结论依据落到**仓库内可回查的文件与行号**（§0.6、附录 A/B）。静态源码／文档阅读**未写成**浏览器或真机运行时 PASS。

### 0.6 主理人独立回查记录（避免引用不可回查的行号）

以下为汇编前由主理人逐条实跑、可复现的核对；凡与取证稿有出入者，本报告一律采用**实测值**并登记订正：

1. `git ls-remote origin refs/heads/fix/r0-sixth-runtime-defects` → `ad34df1c…`（远端 HEAD 与本地一致，未被第三方推进）。首次 `git fetch` 曾遇 `Empty reply from server`，重试恢复；`ls-remote main` 曾遇一次 502，不影响本分支取证。
2. `tools/r0/e2e/scripts/extract-controls.js` **真实路径**：QA / 架构师取证稿中写作 `tools/r0/e2e/extract-controls.js`，实测该路径不存在；真实文件在 **`tools/r0/e2e/scripts/extract-controls.js`**，`cells()` 位于 **L25–31**。本报告统一采用实测路径。
3. `PRD.md` 邀请裁决整块为 **L581–587**（编号条 L582–587），非「§12.1 L585 单条」；`[1.12]` 编号见 L581。
4. `docs/FastLedger-Web设计规范.md` 报表分支「时间/账本切换」在 **L307**（非 L305）；登录行 **L322**；首页行 **L323**；设置行 **L332**。
5. PRD §10 验收标准（L476–575）对 `邀请|入户|注册|members/join` **0 命中**；PRD + Web 设计规范对 §2.1 所定义的**宽词族** **0 命中**（实跑 grep，命令见附录 B #3；窄词族与通用词命中见 #3b / #3c 及 §2.1 口径澄清）。
6. `controls.json` 字段污染的实际解析值由 `python json.load` 实测（§5.2），非从文本推断。
7. `c-controls.js:106 / :137 / :163 / :175 / :908`、`common.go:105–125`、`member.go:34–40 / :615–622 / :684–692 / :800–810`、`invite_lifecycle_api_test.go:88–119`、`middleware.go:98–106`、`model/model.go:22–27 / :43–47 / :82–89`、`router.go:120–138`、`book_handler.go:179–203` 均逐段 `sed -n` 实读。
8. `renderHome` 当前 HEAD 为 **`app.js:1963`**（约结束 2005）；旧盘点文档与 `controls.json:454` 的 `L1929–1971` 为**行号漂移**，结论（首页内容区无 select）经当前 HEAD 复核不变。
9. `renderMembers` 当前 HEAD 为 **`app.js:1667`**，悬空引导文案在 **`app.js:1698`**（历史文档写 L1667 指函数起点，非文案行）。

---

## 1. 结论总表

### 1.1 分类矩阵（三选一单选 + 备注层）

采用 QA 最终自包含定稿与 PM 补充稿 6 约定的**单选兜底映射**：

| ID | 本轮状态 | **三选一主分类** | 备注层（测试验收层，非互斥） | DOM / 源码事实层 | 判定方式（测试证据层） | 本轮处置 |
|---|---|---|---|---|---|---|
| `I-LOGIN-04` | FAIL | **`需求待 Owner 裁决`** | 测试口径问题（硬编码 `bad()`，零 DOM 查询），**需独立整改，不因需求待裁决而搁置** | 登录/注册密码框恒 `type="password"`，无显隐控件 | `c-controls.js:106` 无条件 `bad()` | **维持 FAIL** |
| `I-LOGIN-11` | FAIL | **`需求待 Owner 裁决`** | 测试口径问题（验收域与 PRD §12.1 流程错位 + `classifyMissing` 静态登记），**需独立整改，不因需求待裁决而搁置** | 无邀请码输入/提交控件；前端对 `members/join` 0 调用 | `c-controls.js:78–86` 静态登记 | **维持 FAIL** |
| `I-LOGIN-12` | FAIL | **`需求待 Owner 裁决`** | 测试口径问题（同上 + 拆分粒度：本质为 11 的失败分支），**需独立整改，不因需求待裁决而搁置** | 无失效/异常反馈 UI | `c-controls.js:78–86` 静态登记 | **维持 FAIL，不删不合并** |
| `I-HOME-01` | FAIL | **`测试口径问题`** | —（主分类即口径问题） | 首页 `#view` 内无 select | `c-controls.js:175` 无条件 `bad()`，`:174` 的 `body` 未被消费 | **维持 FAIL 不改判**；仅提请验收域改判 |
| `I-LOGIN-10` | **BLOCKED**（非 4 条之一） | 不适用（状态不改判） | fixture 抽取缺陷导致**漏计** | 无第三入口 | `classifyMissing` 返回 `null` → `c-controls.js:908` BLOCKED | **维持 BLOCKED，不改判 PASS** |

> **「确认缺陷」＝ 0 条。** 4 条 FAIL 无一可判为 `确认缺陷`（均缺 PRD / Owner 决策 / 设计规范的明确条款支撑，或验收域与已裁决口径冲突）。
> **PM 产品层与 QA 测试层对 `I-LOGIN-11/12` 的差异是同一事实的两个层次，不构成分歧**：产品层「待裁决的是登录页控件形态」，测试层「验收域与 PRD 已定流程错位」。若 Owner 裁决「沿用 [1.12] 邀请链接直达注册、不新增登录页入口」，两条应改判为 `测试口径问题`；本报告**不预先替 Owner 选择**。

### 1.2 两条必须保留的限定语

1. **待裁决对象不是 §12.1 需求本身。** `docs/FastLedger-产品需求文档-PRD.md:581–587` 是【决策】`[1.12]` 裁决，**已定且一字未改**；待裁决的只是「验收域 / 控件形态是否按 §12.1 改写，或登录页控件形态是否补进需求」。仅写「需求待裁决」易被误读为「邀请入户功能还没定」，与 `[1.12]` 已定冲突。
2. **「需求待裁决」≠「测试无需整改、等 Owner 即可」。** 4 条 FAIL 的判定来自 `c-controls.js:106 / :175` 硬编码 `bad()` 与 `:78–86` 静态登记，**零 DOM 查询、零专属截图、零诊断**；这是独立于产品裁决的测试缺陷，须按 §5.5 逐条补真实运行时断言，**不因需求待裁决而搁置**。

### 1.3 「确认缺陷 = 0 条」的边界

该结论**只针对 4 条 FAIL 所指的具体控件形态**，不覆盖：

- PRD §12.1 已定而 Web 端 0 实现的「邀请链接入户注册」完整流程（§6.1）；
- D-3 已裁决而顶栏 `#navBooks` 永久 hidden 的显式账本切换控件（§6.1）；
- 59 条 BLOCKED 与 1 条 N/A 未执行的控件；
- 164 条 PASS 中至少 1 条无断言硬编码 PASS（§5.4）。

### 1.4 状态与计数维持（红线）

- **4 条 FAIL 全部维持 FAIL**；无一条改判 PASS、无一条删除 / 合并 / 跳过。
- **`I-LOGIN-10` 维持 BLOCKED**，59 条 BLOCKED、1 条 N/A 原样保留；**不把任何 BLOCKED 改判 PASS**。
- **不调整 164 / 4 / 59 / 1 / 228 任何计数**；不修改 `tools/r0/e2e/fixtures/controls.json`。
- 任何状态变更须经 **PM/Owner 书面裁决 + 真实运行时断言落地 + 全量回归**，并作为独立决策留痕。

---

## 2. 主题一：`I-LOGIN-04` / R0-E-39 密码明文切换

### 2.1 PRD / 设计规范原文依据：**0 命中**

**宽词族（用于需求 / 规范文档，与附录 B #3 一致）**：`明文|显隐|显示密码|隐藏密码|show[ _-]?[Pp]assword|eye|EyeOff|密码可见|type=["]?password|toggle|眼睛|密码切换|切换密码`

**窄词族（密码显隐语义专用，用于 `server/web/dist/` 代码取证，与附录 B #3b 一致）**：`显隐|显示密码|隐藏密码|show[ _-]?[Pp]assword|toggle[ _-]?[Pp]assword|eye-off|EyeOff|密码可见|密码切换|切换密码|眼睛`
（相对宽词族剔除 `明文`、单独 `toggle`、单独 `eye`、`type=["]?password` 等通用词，避免把 API Token 明文条、普通文本输入框、`classList.toggle` 误计为「密码显隐」。）

| 文档（范围） | 词族 | 命中数 |
|---|---|---:|
| `docs/FastLedger-产品需求文档-PRD.md`（HEAD，全文 608 行，尤其 §4.1 L186–190、§8.2 L435/L438、§10 L476–575） | 宽 | **0** |
| `docs/FastLedger-Web设计规范.md`（HEAD，全文 345 行，尤其 §6.2 L206–210、§7.4 L294、§8.2 L322） | 宽 | **0** |
| `docs/FastLadger-Web-PWA视觉设计规范.md`（PR #4 `16705fd`，全文 218 行，§5.6/§6/§8） | 宽 | **0** |
| `docs/FastLedger-开发WBS.md` 附录 B（PR #4） | 宽 | **0** |
| `prototype/`、`prototype-v2/`（两目录全文） | 窄 | **0** |
| `server/web/dist/{index.html, app.js, style.css}` | 窄 | **0** |
| `UI-DESIGN-01` 盘点报告（PR #4）「明文」 | — | 5，**全部为「API Token 一次性明文展示」**（L311/312/314/549/707），与密码无关 |

> **口径澄清（避免把「0 命中」读过宽）**：`server/web/dist/` 对**宽词族并非 0 命中**——按附录 B #3d 对 `dist` 的 `*.js / *.html / *.css` 实跑共 **19 行**：`明文` **3 处**（`app.js:1564/1567/1579`，均为 API Token「一次性明文展示」）＋ `toggle` **16 处**（`app.js` 的 `classList.toggle`）。此外另有 `type="text"` **8 处**（`index.html:48/53/64/69/97/122/137/141`，均为金额 / 备注 / 名称 / 币种等普通文本输入框），**该项不匹配宽词族**（宽词族是 `type=["]?password`），属宽词族之外的补充披露；复核方按宽词族复跑 `dist` 应得 **19 行**，而非把 8 处一并计入的 27。三类命中**均与密码框显隐切换无关**，故上表对前端代码改用窄词族取证（0 命中）；结论「无密码显隐切换需求条款、无对应控件」不因这些通用词命中而改变。前 4 行需求 / 规范文档为 PM 在 HEAD 与 PR #4 blob 上的逐字取证，后 2 行代码 / 原型为主理人本轮 `grep` 实跑（命令见附录 B #3 / #3b / #3c / #3d / #9）。

**封闭清单均为反向证据**：

- Web 设计规范 §8.2 登录行（**L322**）逐字：「| 登录 | 预填演示账号、按钮 pending、错误红字、成功后整页过渡进首页 |」——登录页必备交互的封闭列举，不含显隐切换。
- `UI-DESIGN-01` §3.1（L172、L175）「不可删除功能」清单：用户名/密码登录、注册（含确认密码、`minLength=8`）、注册成功自动登录、登录/注册互相切换、按钮 pending 文案；**不含显隐切换**，且该页 IA 结论为「无」。
- `UI-DESIGN-01` §4.7 缺口矩阵 11 项（L442–462）为「**已由 PRD / Owner 决策 / 差异盘点确认但 Web 前端没有对应 UI**」的能力（会话列表/撤销、修改密码、服务器信息、备份状态、可见性开关、邀请撤销、账本重命名、账本设置承载页、分类管理入口、手机设置/登出可达性），**逐项不含密码明文切换**。
- 易被误读的两条：Web 规范 §7.4「全部交互可用键盘到达」约束的是**已存在的**交互，不产生新控件；PWA 规范 §5.6「密码修改」是改密能力，≠ 登录框显隐切换。PRD §8.2「不在日志中记录密码」是**存储与日志**安全要求，与「给用户显示明文」不同维度，不可互推。

**R0-E-39 登记本身是悬空引用**：

- `docs/FastLedger-R0-独立补漏与复核记录.md:330`：「按设计规范补显隐切换控件」——**未给出任何规范章节号**，且两份设计规范逐字检索 0 命中。
- `docs/FastLedger-R0-基准冻结与差异盘点.md:157` 与 `:488`：R0-E-39 来源明确记为「本轮二次复核**新发现**」，即盘点者的观察项，非 PRD / 规范条款下发。
- `docs/r0-evidence-6th/report.md:37` FAIL 依据写「主报告登记缺失 R0-E-39」，形成**自引用闭环**，未外接任何需求条款。

### 2.2 当前代码路径及行号

| 位置 | 实测内容 |
|---|---|
| `server/web/dist/app.js:161` | `function renderLogin(mode)`，仅 login / register 两模式 |
| `app.js:179` | `const p = document.createElement("input"); p.placeholder = …; p.type = "password";` |
| `app.js:184` | `const p2 = …; p2.placeholder = "确认密码"; p2.type = "password";` |
| `app.js:227` 附近 | 两模式切换后重渲染 `renderLogin` |
| 全文（窄词族，§2.1） | `showPassword` / `togglePassword` / `eye-off` / `显隐` / `密码可见` / `密码切换` **0 命中**；即不存在把密码框 `type` 在 `password` ↔ `text` 之间切换的代码 |
| 全文（通用词，非密码语义） | `明文` 3 处（`app.js:1564/1567/1579`，API Token 一次性展示）、`toggle` 16 处（`classList.toggle`）；`index.html` 另有 8 处普通 `type="text"` 输入框（金额 / 备注 / 名称 / 币种），**均非密码框切回**（见 §2.1 口径澄清） |

### 2.3 后端 API 证据

**无**。密码显隐是纯前端控件，不涉及任何后端接口；PRD §4.1 的「密码用 bcrypt/argon2 哈希存储」与 §8.2 的「不在日志中记录密码」在服务端已实现，本项与之无冲突也无需改动。

### 2.4 现有测试证据

| 来源 | 结论 |
|---|---|
| `tools/r0/e2e/specs/c-controls.js:106` | `R['I-LOGIN-04'] = bad('浏览器中不存在「明文切换」控件（主报告登记缺失 R0-E-39）；DOM 无 eye/toggle 类元素');` —— **无条件硬编码 `bad()`**；`:103` 的 `pw` locator 与 `:105` 的 `getAttribute('type')` 完全未被该行使用，**无任何 eye/toggle DOM 查询** |
| `artifacts/a05-r13-formal-ci/job.log:1070` | FAIL 明细，detail 与 `:106` 逐字一致 |
| `job.log:1109–1203` 上传清单 | C/I-LOGIN 目录只有 `:1178 I-LOGIN-10.png` 与 I-LOGIN-13/14/15（均 `:907` 的 BLOCKED 占位截图）；**不存在 `I-LOGIN-04.png`** |
| `docs/r0-evidence-6th/report.md:37` | 同一 FAIL 登记 |
| `docs/r0-screenshots/README.md:85–88,148,192` | `login/390x844/normal.png` 等 4 档截图仅用户名 / 密码 / 登录 / 注册链接，QA 用 `ViewImage` 实看确认 |
| A / D 套件 | A01–A11、D 各用例对该控件 **无覆盖关系** |

即：**该 FAIL 不是浏览器运行时观测结果**，detail 中「DOM 无 eye/toggle 类元素」是未经观测的陈述。违反 `c-controls.js:7,10` 与 `docs/FastLedger-R0-第七次测试质量纠偏任务单.md:100`「不得把硬编码状态、静态代码定位或截图存在等同于控件行为验收」。

### 2.5 分类与理由

**`需求待 Owner 裁决`**（备注：测试验收层＝测试口径问题，需独立整改，不因需求待裁决而搁置）。

理由：按 §2.1 为各范围分别定义的词族取证，PRD + 两份设计规范（宽词族）+ 原型目录与前端代码（窄词族）+ WBS 附录 B 对该控件全部 **0 命中**（原型目录若改按宽词族复跑，会命中 4 个压缩 JS / CSS 文件中的 `toggle` / `eye` 等通用词，与密码显隐无关，口径同 §2.1 澄清，命令见附录 B #9）；登录页必备交互清单与「不可删除功能」清单都是封闭列举且不含该项；「已要求但未实现」缺口矩阵亦不收录。唯一把它写成「按设计规范」的 R0-E-39 登记，其引用的规范经逐字检索并无此条款。因此只能归为**常见 UX 建议**，不构成已确认的产品或设计要求；按本项目 RV-04 判例（§4.4），无明确需求条款不得计入已确认缺陷统计。**把无条款的 UX 建议计入已确认缺陷本身亦属口径错误，但根因是需求未确认，按 §0.4 取根因分类。**

### 2.6 最小整改方案与文件改动范围（**方案，本轮不实施**）

> **R0 仍 BLOCK，以下仅供 Owner 裁决后参考；本轮未修改任何文件。**

**产品/运行时层（仅在 Owner 裁决纳入需求且阶段放行后）**

- 改动文件：**仅 `server/web/dist/app.js` → `renderLogin()`（L161–231）**。
- 在 L179 的 `p` 之后（注册态还需 L184 的 `p2`）增加一个 `type="button"` 的显隐按钮（或 `<label><input type="checkbox">显示密码</label>`），带 `aria-label` / `aria-pressed`。
- `click` → 只改 `p.type`（`password` ↔ `text`）；注册态同步作用于 `p2`（建议同态切换，避免二次点击）。**只改 `type` 属性，不复制 / 回显密码值，不把密码写进 DOM 文本、URL、`localStorage` 或日志**。
- 复用既有设施：`api()`、`toast()`、`btnPending()`、`state`；不新增依赖、不引入框架。
- 后端 API：无新增、无改动。

**测试口径层（独立于产品裁决，见 §5.5）**

- 改动文件：**仅 `tools/r0/e2e/specs/c-controls.js`** + 新增证据文件；**不改 `controls.json`**。
- 把 `:106` 的硬编码 `bad()` 换成真实 Playwright 运行时断言。

**明确不改**：任何 `server/internal/**` 文件、`tools/r0/e2e/fixtures/controls.json`、A / D 场景脚本、PRD / 设计规范 / WBS。

### 2.7 逐项验收标准

| 层 | 编号 | 可验证条件 |
|---|---|---|
| 运行时 | V-04-1 | 登录态与注册态**各存在 1 个**可见、可键盘到达的显隐控件；点击后 `p.type` 由 `password→text`、再次点击 `text→password`（前→后→再前三态），注册态 `p2` 同步 |
| 运行时 | V-04-2 | 切换不改 `p.value`、不触发提交、不产生任何网络请求；焦点不丢失；表单 submit 仍取 `p.value` |
| 运行时 | V-04-3 | `renderLogin` 重渲染（L227）后复位为 `"password"`；页面源码 / DOM 中无密码明文文本节点 |
| 运行时 | V-04-4 | 390×844 与 1280×800 两档视口各跑一遍，附截图 |
| 测试 | V-04-5 | `c-controls.js` 中该条改为 Arrange-Act-Assert 真实断言，`detail` 由实测 `count()` / `type` / `mode` 拼装，禁止模板字符串与静态登记；附 `shots` + `page.content()` 过滤快照或 console/network 诊断 |
| 测试 | V-04-6 | 断言显式区分「控件不存在（DOM `count()===0` + 截图）」与「控件存在但行为不符」；**不得再从 `controls.json.status` 推断 FAIL** |
| 状态 | V-04-7 | 在 Owner 书面裁决 + 新断言落地 + 全量回归之前，**维持 FAIL 登记**；不得改判 PASS、不得删除 / 跳过 |
| 回归 | V-04-8 | `git diff -- server/internal/` 为空；A 11/11、D 39/39 不回归 |

### 2.8 待 Owner 裁决事项（不替 Owner 决定）

1. 是否将「密码明文 / 显隐切换」**正式纳入** PRD §4.1 与 Web 设计规范 §6.2 / §8.2（并同步 PWA 视觉规范 §6），给出控件形态、状态、可访问性与验收标准。
2. 若纳入，是否属于 **R0 验收范围**，或排期至 R1 及以后。
3. 若不纳入，是否同意将 `I-LOGIN-04` 从「已确认缺陷 / FAIL」计数中**移出**（按 RV-04 同类处置口径），并更正 R0-E-39「按设计规范补显隐切换控件」这一无出处表述。
4. 该控件与 PRD §8.2「不在日志中记录密码」安全边界的关系由 Owner 明示（本报告不作技术判断）。

---

## 3. 主题二：`I-LOGIN-11` / `I-LOGIN-12`（R0-E-38 邀请码输入、提交及异常反馈）

### 3.1 PRD 原文依据：`[1.12]` 逐字（`docs/FastLedger-产品需求文档-PRD.md:581–587`）

> 「- 【决策】关闭公开注册后的成员入户（[1.12] 裁决）：
>   1. **邀请与公开注册解耦**：`EBK_ENABLE_REGISTER=false` 只关闭"任何人可自助注册"，**不影响邀请码注册**；邀请是关闭注册后的**唯一入户通道**。
>   2. **邀请码属性**：由 owner / admin 生成，绑定账本 ID + 角色（member / admin）、**有效期默认 7 天**（`EBK_INVITE_TTL_DAYS`，可配）、**单次使用**（用后标记 `used_at` / `used_by`）、可选绑定邮箱或用户名（绑定则校验一致才可注册，防链接转发滥用）。
>   3. **限流**：每账本每分钟生成上限 5、同一 IP 每小时尝试上限 10；失败计入审计。
>   4. **注册流程**：访问邀请链接 → 展示账本名与邀请人 → 设置用户名 / 密码 → **原子**（同事务）建 `user` + `book_members`，并按绑定角色入账本。
>   5. **全生命周期审计**：生成 / 使用 / 撤销 / 过期均写 `audit_logs`；owner 可手动撤销未使用的邀请。
>   6. **极端情况**：实例无任何用户（首次启动）走 owner 初始化（CLI / 首启向导），不受邀请流程限制。」

配套权威声明：PRD §12 章首 L551「§12.1 的五项技术项亦已于 [1.9] / [1.10] / [1.12] 全部转为【决策】，本章现无遗留【待确认】项」；§12.1 引言 L564 同口径；WBS 附录 A 提案 3（L220–237）为同源设计说明，抬头 L180 注明「**与 PRD 冲突时以 PRD 为准**」。

**已确认的邀请机制规则（本报告一字未改、不得改动）**：邀请与公开注册解耦；绑定账本 ID + 角色；有效期默认 7 天（`EBK_INVITE_TTL_DAYS`）；单次使用（`used_at` / `used_by`）；可选绑定邮箱或用户名；限流（每账本每分钟 5 生成 / 同 IP 每小时 10 尝试，失败计审计）；`访问邀请链接 → 展示账本名与邀请人 → 设置用户名/密码 → 原子建 user + book_members`；生成 / 使用 / 撤销 / 过期全生命周期审计。

### 3.2 「登录页输入邀请码 + 提交 + 失效/异常反馈」是否有明确要求：**没有**

| 文档 | `邀请码|加入账本|第三入口|redeem|兑换|invite` 命中 | 性质 |
|---|---:|---|
| PRD（HEAD） | 2 | 仅 §12.1 第 1、2 条（「不影响邀请码**注册**」「邀请码**属性**」），**无登录页 / 第三入口 / 输入控件 / 提交按钮 / 前端错误提示** |
| PRD §10 验收标准（L476–575） | **0**（主理人实跑 `sed -n '476,575p' | grep -nE '邀请|入户|注册|members/join'` 空） | — |
| `docs/FastLedger-Web设计规范.md` §8.1 L303–316 / §8.2 L322 | **0** | 登录页必备交互为封闭清单 |
| PWA 视觉规范（PR #4）§9 / §10 | **0** | — |
| WBS 附录 B（PR #4）、UI-DESIGN-01 §3.1 / §4.7 | **0** | — |

**PRD 定义的入口形态是「邀请链接直达的注册流程」，不是「登录页第三模式输入邀请码兑换」**（L585）。第 1 条「唯一入户通道」只描述通道，未描述 Web 登录页控件。PRD §12.1 **只对「失效语义与异常处理」有【决策】**（有效期、单次用后失效、可撤销、限流失败计审计、全生命周期审计），**对「登录页前端如何把失效/异常反馈给用户」（错误码映射、提示文案、错误态 UI）没有任何条款**；不可把前者当作后者的需求依据。

**「第三入口 / 邀请码兑换」说法的真实出处（均非需求文档）**：

- `docs/FastLedger-R0-独立补漏与复核记录.md:329`：R0-E-38 依据为 `renderMembers` 一句 UI 文案 + 后端公开路由；目标栏写「补登录页第三入口 + 邀请码输入/提交/失效反馈」。
- `docs/FastLedger-R0-基准冻结与差异盘点.md:163`：I-LOGIN-10 写「**应存在第三入口**」，并把 `renderMembers` 文案自定性为「**悬空引导**」；`:164` 写「应输入邀请码并兑换」；`:165` 写「依赖 I-LOGIN-11」。
- `docs/FastLedger-R0-基准冻结与差异盘点.md:488`：R0-E-38 记为「本轮二次复核**新发现**」。

→ 依据只有两类：**产品代码里的一句 UI 引导文案** + **后端已存在的公开路由**，都是实现侧事实；「补登录页第三入口…」是盘点者写下的目标 / 建议，不是已确认需求。

**测试运行器自相矛盾的硬证据**：`tools/r0/e2e/specs/c-controls.js:137` 在判 BLOCKED 时自己写「登录页无「加入账本/邀请码」入口，**加入流程需邀请链接直达**」；`docs/r0-evidence-6th/report.md:41 / :42` 同文。→ **C 套件运行器自身的阻塞理由与 `I-LOGIN-10/11/12` 的验收条件（登录页第三入口 / 邀请码输入 / 失效反馈）互相矛盾**，是「验收口径写错」最直接、最优先的证据，且与 PRD L585、架构师「无公开 lookup 端点承载该链接」三方互证。

### 3.3 当前前端代码路径及行号

| 位置 | 实测内容 |
|---|---|
| `app.js:161–231` | `renderLogin(mode)` 仅 login / register 两模式，**无第三入口、无邀请码输入框、无失效反馈容器** |
| `app.js:1667` | `async function renderMembers(bookId)` |
| `app.js:1698` | `note.textContent = "对方在登录页选择「加入账本」并输入邀请码即可。有效期见邀请设置（默认 7 天）。";` —— **悬空引导**，与实际 UI 矛盾 |
| `app.js` 全文 | `members/join` / `redeem` / `邀请链接` / `join(` **0 命中**（QA 复核：仅 L1086 一处 `URLSearchParams` 无关命中） |
| `app.js:1690` 附近 | 前端邀请生成只传 `boundUsername`，未覆盖后端支持的 `boundEmail` 与 revoke（`router.go:175`）——**不属本次 4 条 FAIL**，仅作现状记录 |

### 3.4 后端 API 全景（架构师取证，主理人抽查关键行号属实）

**路由分组事实**（`server/internal/api/router.go`）：

```go
122: v1 := r.Group("/api/v1")
124:   v1.POST("/auth/register", ah.register)
125:   v1.POST("/auth/login", ah.login)
127:   // 邀请码入户：不受 EBK_ENABLE_REGISTER 限制（PRD §12.1）
128:   v1.POST("/books/:id/members/join", bh.join)
135:   auth := v1.Group("", JWTAuth(d.AuthService, d.PATs))
```

`auth`（JWT/PAT 鉴权组）自 **L135** 起；**L128 的 join 挂在 `v1` 上、在鉴权组之外 → 确属公开端点**，完整路径 `/api/v1/books/:id/members/join`。

| # | 方法 + 完整路径 | 路由行 | 鉴权 | Handler | 路径参数 / Body | 成功 `data` |
|---|---|---|---|---|---|---|
| 1 | `POST /api/v1/books/:id/members/join` | `:128` | **公开（无 JWT，不受 `EBK_ENABLE_REGISTER` 限制）** | `book_handler.go:179–203` `join` | path `:id`（账本 ID）；body `{code, username, email, password}` | `currentUserDTO(u)` = `{id, username, email, isInstanceAdmin}`，**不含 accessToken** |
| 2 | `GET /api/v1/books/:id/members` | `:167` | JWT 组 | `book_handler.go:115–130` | path `:id` | `MemberDTO[]` |
| 3 | `POST /api/v1/books/:id/members/invite` | `:168` | JWT 组 | `book_handler.go:132–157` | body `{role, boundUsername, boundEmail}` | `{code, role, expiresAt}` |
| 4 | `POST /api/v1/books/:id/members/invite/:inviteId/revoke` | `:175` | JWT 组 | `book_handler.go:163–177` | path `:id`, `:inviteId` | `{inviteId, revoked:true}` |
| 5 | `PUT /api/v1/books/:id/members/:uid` | `:176` | JWT 组 | `book_handler.go:205–228` | path `:uid`；body `{role}` | `{uid, role}` |
| 6 | `DELETE /api/v1/books/:id/members/:uid` | `:177` | JWT 组 | `book_handler.go:230–247` | path `:uid` | `{uid}` |
| 附 | `POST /api/v1/auth/register` | `:124` | 公开，但受 `enableRegister` 开关（关则 403） | `auth_handler.go:49–70` | `{username, password}` | 脱敏 user DTO（无 token） |
| 附 | `POST /api/v1/auth/login` | `:125` | 公开 | `auth_handler.go:72–89` | `{username, password}` | `{accessToken}` |

上表 6 行是 `router.go` 中**「成员 / 邀请相关」的全部路由**（`:128/167/168/175/176/177`），**并非全仓路由穷举**——该文件共有 **65 处路由注册**（`grep -cE '\.(GET|POST|PUT|DELETE|PATCH|Any|Static|StaticFS|Group)\(' server/internal/api/router.go` = 65），其余为认证、账本、交易、账户、分类、预算、报表、同步、WebSocket、Token、OCR 等端点，与本主题无关。仅就邀请主题而言：**不存在 `/invite/.../redeem` 端点**（`redeem` 仅出现在 `member.go:647` 审计事件常量与 `:959` 注释）、**不存在 `GET /books/:id/invites` 邀请列表端点**、**不存在按邀请码反查账本 / 邀请详情的公开端点**。`controls.json:186–200` 中 I-LOGIN-11 的 evidence 提到 `/invite/.../redeem`，该措辞易误导复核方以为后端还缺一个 redeem 端点，**实际兑换入口就是 `/members/join`**。

**join 契约要点**（`book_handler.go:179–203` 实读）：

```go
186: var req struct {
187:   Code     string `json:"code"`
188:   Username string `json:"username"`
189:   Email    string `json:"email"`
190:   Password string `json:"password"`
191: }
197: u, err := h.members.JoinViaInvite(id, req.Code, req.Username, req.Email, req.Password, c.ClientIP())
202: ok(c, currentUserDTO(u))
```

- 邀请码字段名是 **`code`**（非 `inviteCode` / `token`）；账本 ID 来自 **URL 路径 `:id`**。
- **必须先知道 bookId 且与码配对**：`member.go:805–807` `found := has && inv.BookId == bookID`，注释明确「'存在但不属于本账本'与'不存在'对外不可区分、也必须不可区分：否则攻击者可以拿已知邀请去枚举别的账本里有哪些主键存在」。这是前端方案的最大硬约束。
- **成功响应不发 token**（`:202` 只返回 `currentUserDTO(u)`）：前端 join 成功后必须**再调 `POST /api/v1/auth/login`** 才能拿到 token 写入 `state.token` / `localStorage fl_token`。
- 公开但仍经 `router.go:52` `SetTrustedProxies(nil)`，`c.ClientIP()` 恒为 TCP 对端，供按 IP 限流。

**响应错误码**（统一 `{code:<HTTP状态码>, msg:<文案>}`，`common.go:22–24`；`mapSvcErr` 在 `common.go:39–136`）：

| HTTP | 文案 | 触发条件 |
|---|---|---|
| 400 | **邀请码无效或已过期** | `common.go:111–112` `ErrInviteInvalid` 及其全部包装：码不存在 / 不属于该账本 / 已用 / 已撤销 / 已过期 / 账本已删**六态合并**；错误本体 `member.go:34–38` |
| 400 | **邀请码已绑定指定用户，信息不匹配** | `common.go:113–114` `ErrInviteBoundMismatch`；`ErrInviteEmailUnavailable` 刻意并入同一文案防邮箱枚举 |
| 400 | 密码至少 8 位 / 密码过长（最多 72 字节）/ 用户名长度超限 / 请求参数非法 / 请求格式错误 | `common.go:120–128`、`:27–33`、`:92–96` |
| 409 | 用户名已存在 | `common.go:109–110` |
| 409 | **该账本成员已达上限（20）** | `common.go:115–117` |
| 429 | **操作过于频繁，请稍后再试** | `common.go:118–119`，`CreateInvite member.go:361–363`（5/分钟/账本）、`JoinViaInvite :782–784`（10/小时/IP） |
| 503 / 500 | 服务暂时不可用 / 服务内部错误 | `common.go:129–134`（不回显内部错误） |

**join 端点专属结论**：**不会产生 401 / 403**（公开入口）；**不会产生 404**——`bookID()` 只校验数字格式，账本不存在 / 已删被转成 `ErrInviteBookUnavailable` → 统一 400「邀请码无效或已过期」（`member.go:829–840`，注释：「兑换入口未认证，不能让它成为账本存在性的探测器」）。单证佐证 `server/internal/api/invite_lifecycle_api_test.go:88–119` `TestMapSvcErrInviteFailuresDoNotLeak` 逐项断言 400 且响应体不含「占用 / 已绑定给 / sqlite / invites / books.」。**这条防枚举口径在验收中必须保持不变；前端不得要求后端拆分错误码。**

### 3.5 邀请规则服务端实现（仅取证，**不得修改任何规则**）

| 规则 | 实现位置 | 现状 |
|---|---|---|
| 有效期 7 天 | `config.go:30–31` `DefaultInviteTTLDays = 7`、`:128` `EBK_INVITE_TTL_DAYS`；`ttl.go:31–39`；`member.go:397` `ExpiresAt` | 惰性判定、无后台任务；`inviteRejectReason` `member.go:556–569`；SQL 条件 `inviteUsableCond` `member.go:550` |
| 单次使用 | `member.go:550`、`:926–964` 条件 UPDATE + `RowsAffected==1` 校验 | 兑换与撤销共用同一 WHERE，并发下至多一个成功；`model/model.go:82–96` `Invite{UsedAt,UsedBy,Revoked,ExpiresAt,…}` |
| 角色绑定 | `book_handler.go:139–143`；`member.go:353–356`（邀请不能授予 owner）、`:1041–1051` `resolveInviteBinding`、`:919–921` | 绑定维度为「用户名或邮箱二选一」，双非空 400；邀请码 32 位十六进制 |
| 限流 | `member.go:361–363`（5/分钟/账本）、`:782–784`（10/小时/IP）、`:1053–1070` `ipBucket` | 与 PRD L584 一致；`router.go:52–54` 保证桶键不可被 `X-Forwarded-For` 伪造；限流拒绝不写审计 |
| 审计 | `member.go:408–417 / :498–507 / :969–977` 成功同事务；`:526–533 / :741–747` 失败独立连接；`inviteAuditPayload` `:684–722`；规则 `:615–622` | **审计内容最小化**：只记邀请主键、角色、绑定维度、产生的用户 ID；**不记邀请码（哪怕掩码前缀）、口令、绑定邮箱/用户名本体**；失败只记粗粒度 reason + `ipBucket`，无法归因落 `book_id=0` |

**未发现缺失项**：以上五项服务端均已实现且有测试覆盖（`remediation_invite_*_test.go`、`invite_lifecycle_api_test.go`）。**唯一「服务端没有」的能力是「按邀请码反查账本/邀请详情」的公开端点**——这是方案必须绕开的既有契约，不得通过改规则绕过。**4 条 FAIL 全部是前端缺口 / 测试口径问题，后端零缺口、无需改动。**

### 3.6 `I-LOGIN-10` 是否构成前置依赖：**分层消歧，两层同真**

| 层 | 判定 | 依据 |
|---|---|---|
| **产品流程层** | **不是前置依赖** | PRD §12.1 L585 定义加入动作发生在**邀请链接直达的注册流程**（未注册用户），与「登录页」无流程衔接；登录页第三入口是盘点者基于悬空文案额外假设的入口。只有当 Owner **明确裁决新增登录页第三入口**时，`I-LOGIN-10` 才成为 11/12 的 UI 前置；当前 PRD 流程下不成立。 |
| **控件 / 执行层** | **是硬前置依赖** | `controls.json:214` 明写「12 依赖 11」；主报告 `:163–165` 把 10/11/12 登记为同一 R0-E-38 闭环；当前 DOM 三者皆不存在，任何一条都无法构造运行时断言。且 10 判 BLOCKED 而 11/12 判 FAIL，属**同一链条上的状态口径不一致**。 |

→ 汇编统一写「**控件/执行层硬依赖、需求层非前置**」，不写成两份报告互相否定。`I-LOGIN-10` 当前 **BLOCKED**（`job.log:1178` 的 `I-LOGIN-10.png` 是 `c-controls.js:907` 的 BLOCKED 占位截图；`docs/r0-evidence-6th/report.md:43` 同状态），**维持不改判 PASS**。

### 3.7 现有测试证据

| 来源 | 结论 |
|---|---|
| `c-controls.js:78–86` `classifyMissing` | 11 / 12 均无显式 `R[id]` 赋值，由 `controls.json:199 / :215` 的 `status='**缺失**｜R0-E-38'` **静态登记 FAIL**，detail 与 `:81` 模板逐字一致；**零 DOM 查询** |
| `c-controls.js:137` / `r0-evidence-6th/report.md:41,42` | 运行器自身写「加入流程需邀请链接直达」，与 10/11/12 验收条件自相矛盾 |
| `job.log:1071 / :1072`、`:1109–1203` | FAIL 明细；上传清单无 `I-LOGIN-11.png` / `I-LOGIN-12.png` |
| `docs/r0-evidence-6th/report.md:43–45` | I-LOGIN-10 BLOCKED / 11、12 FAIL |
| `invite_lifecycle_api_test.go:88–119` | 服务端防泄露断言（各失效态统一 400、响应不泄露内部信息） |
| A / D 套件 | A01–A11、D 成员邀请**生成侧**用例对该控件 **无覆盖关系**；`tools/r0/e2e/specs/` **目录内**检索 `明文切换|eye|加入账本|redeem|members/join|账本切换` 仅命中 `c-controls.js:106/:137/:175`（非全仓所有文件检索） |
| 历史计数漂移 | `dadc9a3` 曾 `163 PASS / 5 FAIL`（第 5 条 `I-ACCT-06`），因 `c-controls.js:334–349` 把断言从「390px 首列视觉可见性」改为「DELETE 409 + API 回读仍在」而消失（`docs/r0-evidence-7th-fix/qa/qa-report.md:77,85`） |

### 3.8 分类与理由（双层表述）

> **`I-LOGIN-11` / `I-LOGIN-12`：产品需求层 = `需求待 Owner 裁决`**（登录页第三入口 / 邀请码输入 / 失效反馈无 PRD 条款，具体控件形态待裁决）；**测试验收层 = `测试口径问题`**（C 套件验收域与 PRD §12.1 已定的邀请链接直达注册流程错位，且判定非运行时断言）。两者分层共存，**共同结论为「不可判确认缺陷」**。

三选一单选时以 **`需求待 Owner 裁决`** 为主分类（triage 根因是「该具体形态无需求条款」），备注列明「测试层视角为测试口径问题，需独立整改，不因需求待裁决而搁置」。**不写成两个互斥结论**。

理由：邀请机制的**业务规则**与**入口形态**均已在 PRD §12.1 `[1.12]` 明确为【决策】；但「登录页输入邀请码 + 提交 + 失效/异常反馈」在 PRD、Web 设计规范、PWA 视觉规范、WBS 附录 B、UI-DESIGN-01 登录页清单与缺口矩阵中**均无条款**。C 套件把「产品代码一句悬空引导文案 + 后端公开路由」上升为 FAIL，属于把实现侧事实当需求。API 层 `JoinViaInvite` / `/members/join` 与 A 套件「真实邀请码兑换」已存在（`origin/main` CHANGELOG L1104 / L104），因此也不宜表述为「产品侧未实现」——那是**特定 UI 入口未接**，不是产品能力缺失。

### 3.9 最小整改方案与文件改动范围（**方案，本轮不实施，须 Owner 书面裁决**）

> ⚠️ **R0 仍 BLOCK，下列方案仅供 Owner / 后续阶段参考，R0 未放行前不得实施。本轮未修改任何文件。**
> **「照 PRD 原文落地 ≠ 纯前端最小改动」**：PRD L585 的「展示账本名与邀请人」步骤是 `[1.12]`【决策】的组成部分，不是可选细节；当前无「以未认证凭证为入参的公开查询端点」承载该步骤。

**完整但最小的前端流程（以 PRD L585 为准）**

1. 访问邀请链接 → **展示账本名与邀请人** → 设置用户名 / 密码 → 提交。
2. 提交调用 `POST /api/v1/books/{bookId}/members/join`，body **恰为 `{code, username, email, password}`**（`code` 字段名见 `book_handler.go:187`）；**公开、不带 `Authorization`**；URL 路径 `bookId` 必须与邀请码所属账本一致（`member.go:807`）。
3. 成功后**再调 `POST /api/v1/auth/login`** 取 `data.accessToken`（join 只返回 user DTO），写入 `state.token` / `state.whoami` / `localStorage fl_token,fl_whoami`，`location.hash = "#books"`。
4. `EBK_ENABLE_REGISTER=false` 时该入口仍须可用（`router.go:127` 注释 + `auth_handler.go:50–54`）。

**架构师技术侧参考形态（逐字降级声明，不得作为默认落地方案）**：

> 「**架构师技术侧参考形态，非产品需求、非本报告方案、R0 不实施**；该形态省略 PRD `[1.12]` 第 4 条的『展示账本名与邀请人』步骤，**不得由实施方自行裁剪**；是否接受该偏离或另开公开端点承载展示步骤，**必须由 Owner 书面裁决**。」

（该形态指「邀请分享链接只携带 `bookId`，邀请码只放 POST body 由用户粘贴，或由用户在表单里同时填写账本 ID」。采用它即构成对已确认需求的偏离。）

**裁决所需信息（供 Owner 参考，非本报告结论，不改变任何判定）**：PRD L585「展示账本名与邀请人」在**数据层完全齐备**——`books.name`（`model/model.go:45`）、`users.username`（`:25`）、`invites.book_id` / `created_by`（`:84` / `:87`），且 `MemberService.List`（`service/member.go:204–211`）已有 `book_members m INNER JOIN users u` 取 `u.username` 的现成 SQL。缺的不是数据，而是「**以未认证凭证为入参的公开查询端点 + 防账本存在性枚举口径**」；新增端点须对齐 `member.go:805–807`（跨账本与不存在不可区分）、`common.go:111–112`（统一 400）、join 无 404。**两条裁决路径都有成本，不宜由实施方默认选简化路径**；本报告不写「因此应当选某条路径」，选择权仍归 Owner。

**错误处理（最小但完整）**

- 表单内一个**持久可见**的内联错误容器（`<div class="err-msg" role="alert" aria-live="assertive">`），**不能只依赖 2.6s 自动消失的 `#toast`**（`app.js:23–33`）。
- 错误文案**只取 `err.message`**（`api()` L110 已从 `data.msg` 构造），不解析 / 展示任何后端内部字段。
- 逐类可行动文案：400「邀请码无效或已过期」→ 核对码 / 账本 / 有效期；400「邀请码已绑定指定用户，信息不匹配」→ 用户名或邮箱需与邀请一致（**不得回显绑定值**）；400 口令 / 参数类 → 就地提示；409「用户名已存在」/「成员已达上限（20）」→ 换用户名或联系管理员；**429「操作过于频繁，请稍后再试」→ 明确退避提示，禁止自动重试**（防放大限流与审计）；503/500 → 通用文案并保留按钮可重试。
- **各失效态（已用 / 已撤销 / 已过期 / 码不存在 / 账本错配 / 账本已删）必须统一 400 文案，不得按失效原因区分提示**（`common.go:112` + `member.go:38` 防枚举）；前端**不得**按状态码反推并展示「该账本不存在 / 已删除 / 邮箱已被 X 占用」等可枚举信息。
- 提交期间 `btnPending` 禁用按钮防重复提交（避免消耗 10 次/小时配额）；任一失败均**不写入 `state.token` / `localStorage`**，停留当前页。

**权限与安全硬约束（不得改变）**

- **(S1) 邀请码载体**：`code` 只允许出现在 join 的 JSON body；禁止写入 `location.hash`、query string、`console.log`、`localStorage`、`toast`、错误容器文本。性质拆分见 §3.11。
- **(S2) 不泄露账本存在性**：join 路径不得出现 404；六态统一 400「邀请码无效或已过期」；「邮箱被占用」并入「信息不匹配」400；`invite_lifecycle_api_test.go:90–119` 为服务端既有断言。
- **(S3) 限流与审计规则不变**：不得新增重试循环 / 轮询 / 自动重发；429 只提示退避；不得调大 `5/分钟/账本`、`10/小时/IP`，不得改变 `ipBucket`（/24、/64）与 `SetTrustedProxies(nil)`；不得改变审计 payload 最小化集合与「限流拒绝不写审计」。
- **(S4) 权限**：join 为公开端点但**不接受任何客户端上报的角色 / uid / book 归属**，角色由 `inv.Role` 在事务内决定（`member.go:919–921`）。
- **(S5) 口令**：不在 DOM 文本 / URL / 日志 / 存储中出现明文密码；join 成功后自动登录时口令只在内存变量中一次性传递，不落盘。

**文件改动范围清单（只列路径，本轮未改动任何文件）**

```
产品/运行时层（须 Owner 裁决 + 阶段放行后）：
  server/web/dist/app.js          renderLogin 第三态 / 新增 renderJoin、join 表单与内联错误容器
  server/web/dist/index.html      仅当新增静态内联错误容器节点时需要
  server/web/dist/style.css       仅当布局微调时需要
测试口径层（独立于产品裁决，见 §5.5）：
  tools/r0/e2e/specs/c-controls.js + 新增证据文件

明确不改：
  server/internal/api/router.go / book_handler.go / common.go / middleware.go
  server/internal/service/member.go / config.go / ttl.go / limiter.go / tx.go
  server/internal/model/*
  tools/r0/e2e/fixtures/controls.json（字段污染另案处理，见 §5.2）
  A / D 场景脚本、PRD / 设计规范 / WBS
```

### 3.10 逐项验收标准

| 编号 | 层 | 可验证条件 |
|---|---|---|
| V-11-1 | 流程 | 未登录态可从**邀请链接**进入入户流程；页面展示账本名与邀请人（或 Owner 书面裁决后的等价形态），可设置用户名 / 密码 / 邮箱 |
| V-11-2 | 契约 | `app.js` 中 `grep -n 'members/join'` 有且仅有 1 处调用，路径 `/api/v1/books/{bookId}/members/join`，body 恰为 `{code,username,email,password}`；`grep -n 'redeem'` 仍为 0（不臆造端点） |
| V-11-3 | 契约 | 请求**不带** `Authorization`；成功后**再调** `POST /api/v1/auth/login` 取 `accessToken` 并写 `state.token` / `localStorage`，最终 `hash=#books` |
| V-11-4 | 流程 | `EBK_ENABLE_REGISTER=false` 时入口仍可用；原子建 `user` + `book_members`，成员列表 API 回读 +1；角色由服务端 `inv.Role` 决定 |
| V-11-5 | 规则不变 | 同一码第二次兑换返回 400「邀请码无效或已过期」且前端如实展示（单次使用不变）；`EBK_INVITE_TTL_DAYS` 默认 7 天、20 人上限、5/分钟、10/小时、审计 payload 字段集合均无变化 |
| V-11-6 | 安全 | (S1)–(S5) 逐条可复现；手工把 `bookId` 改成别的账本，响应仍为统一 400「邀请码无效或已过期」，不出现 404 / 不泄露账本存在性 |
| V-12-1 | 错误 UI | 存在 `role="alert"` 的**持久**内联错误容器（非仅 2.6s toast） |
| V-12-2 | 错误 UI | 对 400 邀请无效 / 绑定不匹配、400 口令与参数类、409 用户名已存在 / 成员上限 20、429、503/500 **各有区分文案且均可复现**；**但各邀请失效态之间必须统一 400 文案，不得要求按失效原因区分** |
| V-12-3 | 错误 UI | 错误文案只来自 `err.message`，不含「账本不存在 / 已删 / 邮箱已被 X 占用」等可枚举信息；不回显绑定值 |
| V-12-4 | 错误 UI | 429 后**不自动重试**；按钮在请求期间禁用；任一失败均不写 `state.token` / `localStorage`，停留当前页，按钮 `disabled` 恢复 |
| V-12-5 | 测试 | 参数化 4 组输入（有效 / 已用 / 过期 / 随机无效码）+ 账本 ID 不匹配，逐组记录 `code, httpStatus, toast, errText`；`detail` 由实测值拼装，附截图 + DOM 快照 / 诊断；**不删除、不合并、不跳过该项** |
| 全局回归 | V-2-R1 | `git diff -- server/internal/` 为空；`invite_lifecycle_api_test.go` / `remediation_invite_*_test.go` 全绿；A 11/11、D 39/39 不回归 |
| 状态 | V-2-R2 | 在 Owner 书面裁决 + 新断言落地 + 全量回归之前，**11 / 12 维持 FAIL、10 维持 BLOCKED**；不改判、不删项、不动 164/4/59/1/228 |

### 3.11 S1 三层拆分（防止被复核方读成「既有服务端规则」）

| 载体 | 性质 | 依据 |
|---|---|---|
| 审计表 `audit_logs.before_json` payload | **既有硬约束（明文规则 + 执行点 + 防泄露断言）** | 规则 `member.go:615–622`「【策略二：审计内容最小化】…不记：邀请码（哪怕掩码前缀）、口令、绑定邮箱本体、绑定用户名本体」「记『前 4 位』同样不可接受：它把暴力搜索空间缩小若干数量级，还免费给出命中确认信号」；执行点 `inviteAuditPayload` `member.go:684–722` 字段集合无 `code` / `password` / `boundEmail` / `boundUsername`；断言 `invite_lifecycle_api_test.go:88–119` |
| 运行日志 | **无明文规则** | `member.go:615–622` 不覆盖；`middleware.go:98–106` 仅针对 **WebSocket token** 说明「token 出现在 URL 上会进入 access log / 反向代理日志 / 浏览器历史 / Referer」，属**类比认知**，不是对邀请码的约束 |
| URL（含面向用户的邀请链接） | **技术侧前瞻性验收约束（建议），须 Owner 采纳后才成立** | 「邀请码不得以 `?code=` 形式出现在任何 URL」是架构师 S1 的意图（单选 (b)）；**不是现有源码的明文条款**。`member.go:615–622` 既不禁止 URL 字段、也不构成对任何 URL 的通用禁令。当前「码不在 URL 中」只是 join 契约的**既成事实**（`code` 只在 POST body，`book_handler.go:187`；URL 仅 `:id`，`router.go:128`） |

**(b) 未获 Owner 采纳前，不得写成「服务端已有规则」，也不得据此认定 PRD L585 违规。** PRD L585「访问邀请链接」的落地形态（不透明 token / 一次性 ID / 链接仅带 `bookId`，而非 `?code=`）应与「是否新增公开 lookup 端点」**合并为同一待裁决问题**（补充稿 4 第 1 项），不必拆两条。

### 3.12 待 Owner 裁决事项（**5 项，以 PM 补充稿 4 的编号为准**）

| # | 收敛后的待裁决项 | 说明 |
|---|---|---|
| **1** | **【合并项】邀请链接凭证载体形态 + 是否新增公开 lookup 端点**：是否采纳 (b)（禁用 `?code=`）、采用不透明 token / 一次性 ID / 链接仅带 `bookId` 由用户粘贴 `code`，以及是否另开公开端点承载 PRD L585「展示账本名与邀请人」 | 按架构师要求把「凭证载体」与「是否新增公开 lookup 端点」合并为同一问题；新端点仍须守住不泄露账本存在性、统一 400 文案。裁决所需成本信息见 §3.9 |
| **2** | 是否**新增登录页第三入口** | 主报告 §3.6 原有二选一：(a) 维持邀请链接直达注册为唯一入口；(b) 另增登录页第三入口 + 邀请码输入兑换。若选 (b)，须说明与 (a) 是否并存、是否构成第二套入户路径、是否修订 PRD §12.1 / WBS 1.5 / 附录 A 提案 3 / §4.10 |
| **3** | 若采用简化形态，是否**书面接受对 PRD L585 已确认「展示账本名与邀请人」步骤的偏离** | **特意不并入第 1 项**：涉及**修订已确认的 `[1.12]`【决策】**，性质为产品需求变更裁决，而非入口形态选择；不得由实施方自行裁剪 |
| **4** | 「失效／异常反馈」**前端呈现口径** | 必须在既有统一 400 文案与不泄露账本存在性边界内决定，**不得要求后端拆分错误码**（`member.go:805–807`、`common.go:111–112`）；需明确 409 / 429 / 503 各自前端文案与是否需与 `audit_logs` 对齐 |
| **5** | 是否属 **R0 范围／排期**，以及是否同意把这 3 条从「已确认缺陷／FAIL」计数中**移出或改判** | 原 R0-E-38 登记为「待 R1 实施」；须与 Q2 的「测试断言独立整改」分开裁决 |

> **硬约束（不得改动）**：审计 payload 最小化（唯一既有明文硬约束）；7 天 / 单次使用 / 角色绑定 / 5·10 限流 / 全生命周期审计 / 原子建 `user`+`book_members`；不泄露账本存在性；join 只返 user DTO 不发 token。
> **若选 (b)，Owner 还需明确 `I-LOGIN-10` / `11` / `12` 三者的验收标准与依赖链**（当前 `基准冻结.md:165` 写「12 依赖 11」、`:163` 把 10 列为独立新增，两处口径不一致）。

---

## 4. 主题三：`I-HOME-01` / R0-E-01 首页账本切换

### 4.1 D-3 / Owner 权威决策原文（五处独立来源，逐字）

| # | 来源 | 原文 |
|---|---|---|
| 1 | `docs/FastLedger-R0-收尾执行清单.md:115` | 「\| D-3 \| 导航结构 \| **移动端 4 Tab：首页 / 记一笔 / 报表 / 我的；桌面端 5 项顶部导航；账本切换置于顶栏** \| Owner 2026-11-05 明确同意该方案；实施时按设计规范细化桌面 5 项的具体项目与响应式行为 \| **已裁决（Owner 2026-11-05）** \|」 |
| 2 | `docs/FastLedger-R0-基准冻结与差异盘点.md:957` | 「**Owner 2026-11-05 裁决：移动端 4 Tab（首页/记一笔/报表/我的）；桌面端 5 项顶部导航；账本切换置顶栏** \| **已裁决（Owner 2026-11-05）**」 |
| 3 | `docs/FastLedger-R0-第四次证据补齐任务单.md:4–5,16` | 「Owner 已于 2026-11-05 裁决 D-3/D-5/D-6/D-2…**本任务不再讨论这些决策**」「- **D-3**：移动端 4 Tab…**账本切换置于顶栏**」 |
| 4 | PR #4 `docs/FastLadger-Web-PWA视觉设计规范.md:86–87,104`（未合并） | 「**遵守 D-3：桌面端保留 5 项顶部主导航，不改成固定左侧主导航栏**…**账本切换置于顶栏**」「D-3…与 D-6 …**均为已裁决事项，不再列为待 Owner 决策。实现方必须按此执行；若未来确需修改，须先由 Owner 明确修订决策记录**」 |
| 5 | PR #4 `UI-DESIGN-01` 盘点报告 `:350–351, :410–411, :434, :756, :768` | 「\| 账本切换 \| **位于顶栏**，不进入 5 项主导航计数 \|」「\| 顶栏右 \| **账本切换**、用户/设置入口 \| D-3 \|」「§9.2：数量（5 项）与位置（顶部主导航 + 账本切换置顶栏）由 D-3 固定，**不是待裁决事项**」 |

另有 `docs/FastLedger-R0-收尾执行清单.md:161`「已完成：D-3/D-5/D-6…**不得再标为待确认**」、`origin/main` CHANGELOG L196 同口径。**D-3 已裁决、不可 reopen**；待裁决的仅是「验收域是否由『首页本页内』改判 / 是否移出已确认缺陷统计」（RV-04 同类处置），不是重新裁决账本切换放哪。

### 4.2 「首页内也必须有切换控件」：**0 命中**

`首页.{0,20}账本切换|账本切换.{0,20}首页|本页内|首页内.{0,10}切换` 检索：PRD **0**、Web 设计规范 **0**、PWA 视觉规范 **0**；UI-DESIGN-01 盘点报告仅 L625 一处命中，仍是顶栏口径（「按 D-3 实施——桌面 5 项顶部主导航 + 账本切换置顶栏」）。

首页必备交互的封闭清单亦无此项：

- PRD §4.2（L194–198）：本月支出 / 本月收入 / 结余三卡、按日分组账单、筛选、「记一笔」入口、实时联动。
- `docs/FastLedger-Web设计规范.md:305`「├── 首页（本月概览 + 账单流 + 筛选 + 记一笔入口）」；**:323**「| 首页 | 概览三卡实时联动、按日分组账单、行点击进详情、筛选条、骨架屏 |」。
- **`:307`「├── 报表（月度趋势 + 分类占比 + **时间/账本切换**）」——「账本切换」在规范中只出现于报表页与 D-3 顶栏，不在首页。**
- PRD §3.2（L156）「多账本…可新建『个人私账』并**切换**」只确认「能切换账本」这一**能力**，未指定控件位置；位置由 D-3 定为顶栏。PWA 视觉规范 §5.1（L110–114）首页章节通篇无账本切换控件。

### 4.3 当前顶栏是否已实现切换：**否（显式切换控件物理不可见）**

| 位置 | 实测内容 |
|---|---|
| `server/web/dist/index.html:10–13` | `<header id="topbar" class="hidden">`；`:11` `<span class="brand" id="brand">FastLedger</span>`；**`:13` `<a href="#books" id="navBooks" class="hidden">账本</a>`** |
| `server/web/dist/style.css:33` | `.hidden { display: none !important; }` |
| `server/web/dist/app.js:120–154` | `showNav(book)` 只 `remove("hidden")` 处理 `navTxns`…`navRecycle`、`toggle` `navSettings`；**从不移除 `#navBooks` 的 hidden** |
| `app.js:236` | `$( "#navBooks" ).classList.add("active");` —— 仅加 active，不改 hidden |
| `app.js:156` | `$("#brand").addEventListener("click", () => location.hash = "#books");` |
| `style.css:234` | `@media` 中 `#topbar { display: none; }`（移动端整条顶栏隐藏） |
| `app.js:1963–2005` | `renderHome(bookId)` 内容区**无账本 `<select>`**（当前 HEAD 实读；旧文档 L1929–1971 为行号漂移） |

→ **顶栏没有真正可见的账本切换控件**；当前可达 `#books` 的实际路径主要是 **`#brand` → `#books`**（`app.js:156`），以及「我的」→「我的账本」、登录后跳转。`controls.json:454` 中「切换只能经 `#brand`/`#navBooks` → `#books`」是**历史文档逐字引用，不是当前 HEAD 产品事实**（`#navBooks` 恒 hidden）；`controls.json:394–407` 中 I-BOOKS-09 的 `control: 顶栏「账本」` / `status: 已实现｜截图已见｜R0-E-01` 在 HEAD **不成立**。

**两个事实必须分开表述、不合并计数、不互相抵消**：

1. `I-HOME-01` = **验收口径问题**（验收域「首页本页内」与 D-3「置顶栏」冲突，且与 I-BOOKS-08/09 重复，R0-E-01 编号误挂）。
2. `#navBooks` 永久 hidden = **独立的 DOM 事实**，可能是与 D-3 相关的另一条独立缺口（§6.1），**不在 4 条 FAIL 内**；**不得**用后者为前者改判或合并。

### 4.4 编号误挂与 RV-04 同类判例

- `docs/FastLedger-R0-基准冻结与差异盘点.md:804` R0-E-01 本体逐字：「**导航 IA 不符：桌面 10 项平铺 vs 规范 5 项；移动 5 Tab vs 规范 4 Tab；二级页未挂"我的"** | **P1** | `dist/index.html` topbar/bottomNav；规范 §8.1 L301–316 | 视觉/交互（导航结构） | **改为 4 Tab + "我的"中枢；桌面 5 项**」。与「首页内切换控件」无关。
- `基准冻结.md:194` / `docs/r0-evidence-6th/report.md:60` 把 I-HOME-01 挂到 R0-E-01，属**编号误挂**；盘点报告自己已记录切换路径存在（I-BOOKS-08/09），即切换能力可达，只是不在「本页内」。
- **RV-04 判例**（`基准冻结.md:195,1029`、`收尾执行清单.md:119`、`r0-evidence-6th/report.md:61`）：`I-HOME-02`「首页月份切换」经查证 PRD / 原型 / 设计规范均无明确条款，由「缺失」改为「**待 Owner 裁决的交互建议**，移出已确认缺陷统计」，C 套件状态为 **N/A**。`I-HOME-01` 与 `I-HOME-02` 结构同构，且 `I-HOME-01` 的能力位置还有 D-3 明确裁决为顶栏，因此**更不构成缺陷**。

### 4.5 现有测试证据

| 来源 | 结论 |
|---|---|
| `c-controls.js:169–176` | `:172` `page.goto(#b/:id)`、`:174` `const body = await text(page)`，但 **`:175` 完全未消费 `body`**，`R['I-HOME-01'] = bad('本页无账本切换控件…顶部仅标题')` 为无条件硬编码；**无 select / dropdown 查询** |
| `job.log:1073`、`:1109–1203` | FAIL 明细；上传清单**不存在 `I-HOME-01.png`** |
| `docs/r0-evidence-6th/report.md:60` | 同一 FAIL 登记，口径自始未变 |
| `docs/r0-screenshots/README.md:91–95,193` | `home/390x844/empty.png`：三卡 + 4 Tab + FAB，本页内无 select；`:94` `home/1280x800/empty.png`：切换走顶栏（QA `ViewImage` 实看确认） |
| `c-controls.js:163` | `R['I-BOOKS-09'] = ok('空态与新建后刷新已覆盖（空账本行为见 A07）')` —— **硬编码 PASS、零断言**；与 `controls.json` 中「顶栏『账本』」定义存在**编号/定义漂移**（见 §5.4） |
| A / D 套件 | 对该控件**无覆盖关系** |

### 4.6 分类与理由

**`测试口径问题`**（重复要求 / 错误验收口径）。**明确不构成 `确认缺陷`**。

理由：① **重复要求** —— D-3 已把账本切换定为顶栏的全局壳层控件，UI-DESIGN-01 §4.6.1 明确「顶栏右 = 账本切换」，§9.2 明确该位置「由 D-3 固定，不是待裁决事项」；在首页内部再要求一个切换控件是同一能力的第二处入口，无任何文档授权。② **错误验收口径** —— C 套件把验收域限定为「本页内」，而权威条款的位置是顶栏 + 报表页；FAIL 只反映「不在本页内」，不是能力缺失。③ **编号误挂** —— R0-E-01 本体是导航 IA 收敛。④ **先例一致** —— RV-04 已确立「首页必备交互清单无该条款 → 不计入已确认缺陷」口径。⑤ 逐字检索 0 命中；PWA 规范 §4.3 与 UI-DESIGN-01 §4 执行原则要求「若确需修改 IA，须 Owner 书面修订 D-3」。

**不得未经确认擅自增加重复控件**：本轮不新增任何首页内账本切换控件；最小整改是**改验收域**，不是加控件。

### 4.7 最小整改方案与文件改动范围（**方案，本轮不实施**）

| 层 | 内容 | 文件范围 |
|---|---|---|
| **测试口径（本主题的最小整改）** | 把 `c-controls.js` 中 `I-HOME-01` 的判定域由「首页 `#view` 内容区有无 select」改为「**D-3 口径：顶栏全局可达 + 切换闭环**」；补真实 Playwright 断言、两档视口截图与 DOM 诊断 | **仅 `tools/r0/e2e/specs/c-controls.js`** + 新增证据文件；**不改 `controls.json`** |
| 产品/运行时层（**不建议，须 Owner 书面修订 D-3 后才可考虑**） | 候选 A（首页内 `<select>`，`renderHome` L1963–2005，选项来自 L1966 已获取的 `books`，零新增请求，`change` → `location.hash = "#b/"+id`）；候选 B（顶栏 `index.html:10–26` + `showNav` + `style.css`） | `server/web/dist/app.js`；候选 B 另需 `index.html` / `style.css`。**二选一需 Owner 先定口径**；候选 B 会连带 `#navBooks` 从 hidden 变可见并影响 I-BOOKS-09 既有口径，属扩大改动，须单列裁决 |

**权限 / 可见性（若未来实施候选 A）**：`GET /api/v1/books`（`router.go:158` → `book_handler.go:44–51`）**服务端已按当前用户过滤**，前端只渲染该响应中的账本，不得缓存 / 拼接其他账本 ID；切换后所有数据请求仍走 `api()` 带当前 JWT，由后端 `RoleOf` 再次校验成员资格。不新增后端端点。

### 4.8 逐项验收标准

| 编号 | 层 | 可验证条件 |
|---|---|---|
| V-H-1 | 测试 | 判定域改为「**顶栏全局可达 + 切换闭环**」；owner 登录，`390×844` 与 `1280×800` 两档各跑，准备 ≥2 个账本 |
| V-H-2 | 测试 | **Act**：(a) `#view` 内容区查 select / 账本切换按钮 / `[data-testid*=book-switch]`，记 `count()`；(b) 顶栏查 `#brand`、`#navBooks`，逐个点击断言 `location.hash==='#books'`，在 `#books` 选另一本，断言 `hash==='#b/{otherId}'` 且 `#view` 文本含新账本名、概览卡数值随新账本数据变化；(c) 全程截图 + DOM 快照 |
| V-H-3 | 测试 | **PASS**：顶栏入口可达且完成「切换 → 首页数据随账本变化」闭环；**FAIL**：入口不可达 / 点击后无法切换 / 切换后仍渲染旧账本数据；`detail` 由实测值拼装 |
| V-H-4 | 测试 | 当前 `#navBooks` 永久 hidden **作为独立 DOM 事实单独登记**，不与 `I-HOME-01` 合并计数、不互相抵消；「验收域是否由『首页本页内』改判 / 是否移出已确认缺陷统计」提请 PM/Owner 书面裁决（RV-04 同类处置） |
| V-H-5 | 运行时（仅候选 A） | `<select>` 选项**恰好等于** `GET /api/v1/books` 返回集合，`value=b.id`，当前 `bookId` 选中；`change` 落到 `location.hash="#b/"+id` 并触发 `route()` 重渲染；不新增后端端点、不改 `showNav` 其它分支 |
| V-H-6 | 权限 | 越权验证：手工把 `select` 的 `value` 改成非当前用户账本 ID 后，服务端仍返回 403「无权限」（`book_handler.go:58–62` / `common.go:43–46`），前端不展示他人账本数据 |
| V-H-7 | 响应式 | 移动端（`style.css:234` 顶栏隐藏）仍可经「我的」→「我的账本」完成切换 |
| V-H-8 | 状态 | **维持 FAIL 不改判**，直至 Owner 书面裁决 + 新断言落地 + 全量回归；不删项、不动计数 |
| 回归 | V-H-9 | 若采顶栏方案，需额外证明 `#navBooks` 从 `hidden` 变为可见且 I-BOOKS-09 口径同步更新；`git diff -- server/internal/` 为空 |

### 4.9 待 Owner 裁决事项（不替 Owner 决定）

1. 若仍有人主张「首页内**另设**账本切换控件」，须由 Owner **书面修订 D-3**（按 PWA 规范 §4.3 与 UI-DESIGN-01 §4 执行原则第 3 条），并说明该控件与顶栏控件的关系（是否重复入口、是否违背 D-3 导航收敛）、归属页面、验收口径，以及是否同步修订 PRD §3.2 / Web 规范 §8.1 / 盘点报告 §4.6.1 / §9.2。
2. 在 Owner 未作上述修订前，是否同意把 `I-HOME-01` 由 FAIL 改判为 `测试口径问题`（或按 RV-04 口径移出已确认缺陷统计），并更正其 R0-E-01 编号误挂；**本报告不代为改判、不代为修改任何文档**。
3. 需 Owner 明确：D-3 顶栏账本切换控件的**现状可验收性**（当前 `#navBooks` 永久隐藏、仅能点品牌名）是否应作为**另一条独立缺口**登记——这是顶栏控件可见性问题，与「首页内再放一个」是两件事，本报告不合并、不新增需求。

---

## 5. 测试口径层独立发现（跨三项，QA 取证）

### 5.1 4 条 FAIL 的判定机制：**4/4 都不是真实浏览器运行时断言**

框架 `tools/r0/e2e/specs/c-controls.js`（924 行）：

```
:22  const ok      = (detail, shots = []) => ({ status: 'PASS', detail, shots });
:23  const bad     = (detail, shots = []) => ({ status: 'FAIL', detail, shots });
:78  function classifyMissing(ctrl) {
:79    if (/\*\*缺失\*\*|缺失：/.test(ctrl.status) || /^未实现/.test(ctrl.action)) {
:80      const e = (ctrl.status.match(/R0-E-\d+/) || '').toString();
:81      return { status: 'FAIL', detail: `产品侧未实现（主报告已登记${e ? '：' + e : ''}）；本轮确认浏览器中不存在该控件` };
:85    return null;
:895 for (const c of controls) { const got = R[c.id];
:897   if (got) { results.push(row(c, got.status, got.detail, ...)); continue; }   ← 显式赋值优先
:901   const missing = classifyMissing(c);                                          ← 无配方时静态登记
:902   if (missing) { results.push(row(c, missing.status, missing.detail)); continue; }
:907   await page.screenshot({ path: shotPath }).catch(() => {});
:908   results.push(row(c, 'BLOCKED', '未提供可执行配方：…', [...]));
```

| ID | 显式 `R[id]` | 判定来源 | 运行时断言 |
|---|---|---|---|
| `I-LOGIN-04` | 有，`:106` | 无条件硬编码 `bad()`；`:103`/`:105` 的 locator 与 `getAttribute('type')` 完全未使用 | **否** |
| `I-LOGIN-11` | 无 | `classifyMissing()` `:78–82` 读 `controls.json:199` 静态生成 | **否** |
| `I-LOGIN-12` | 无 | 同上，读 `controls.json:215` | **否** |
| `I-HOME-01` | 有，`:175` | 无条件硬编码 `bad()`；`:174` 的 `body` 完全未消费 | **否** |

**4 条 FAIL 均无专属截图 / DOM 快照 / console/network 诊断**：`:898` 只附显式 `R[].shots`（`bad()` 默认 `shots=[]`）、`:903` 静态 FAIL 不附 evidence、`:907` 只给 BLOCKED 截图。`job.log:1109–1203` 的 102 项上传清单中 C/I-LOGIN 目录只有 `:1178 I-LOGIN-10.png` 与 I-LOGIN-13/14/15（均 `:907` 的 BLOCKED 占位截图），**不存在 `I-LOGIN-04.png` / `I-LOGIN-11.png` / `I-LOGIN-12.png` / `I-HOME-01.png`**。

→ detail 中「DOM 无 eye/toggle 类元素」「本轮确认浏览器中不存在该控件」「顶部仅标题」是**从未发生的观测**，违反 `c-controls.js:7,10` 与 `docs/FastLedger-R0-第七次测试质量纠偏任务单.md:100`「不得把硬编码状态、静态代码定位或截图存在等同于控件行为验收」。**这是独立于产品裁决的测试缺陷，须单独整改（§5.5）。**

### 5.2 `controls.json` 字段污染与「主报告登记 → 抽取污染 → runner 降级」因果闭环

`tools/r0/e2e/fixtures/controls.json`（228 条 / 16 分组，`source='docs/FastLedger-R0-基准冻结与差异盘点.md §3.1'`，`extractedAt='2026-11-05T07:21:36.998Z'`）语法合法（`python json.load` 通过），但 **I-LOGIN-04 / I-LOGIN-10 的 `evidence` / `status` 被引号转义截断污染**。主理人 `json.load` + `repr` 实测解析值：

```
I-LOGIN-04  evidence = '缺失：app.js 全文 `showPassword\\'   status = '明文\\'
I-LOGIN-10  evidence = '缺失：app.js `joinForm\\'           status = 'joinLedger\\'
I-LOGIN-11  status = '**缺失**｜R0-E-38'（完整）
I-LOGIN-12  status = '**缺失**｜R0-E-38'（完整）
I-HOME-01   status = '**缺失**｜R0-E-01'（完整）
```

主报告 `docs/FastLedger-R0-基准冻结与差异盘点.md:157 / :163` 的**完整**原文分别为 `**缺失**｜本轮二次复核新增 R0-E-39` 与 `**缺失**｜本轮二次复核新增 R0-E-38`，L163 还逐字写了「renderMembers L1667 文案…为悬空引导」。

**根因链**（QA 定位、架构师独立只读复核、主理人确认路径）：

1. **`tools/r0/e2e/scripts/extract-controls.js:25–31`** 的 `cells()` 用 `line.split('|')` **裸切**，未处理 Markdown 单元格内转义竖线 `\|`：
   ```js
   function cells(line) {
     const parts = line.split('|');
     parts.shift(); parts.pop();
     return parts.map((s) => s.trim());
   }
   ```
   对主报告 L157 / L163 实测各返回 **15 段（应 13）**，`:56` 解构只取前 13 → `status` 变成 `明文\` / `joinLedger\`。L194（I-HOME-01）无 `\|`、13 cell，未损坏。
2. `c-controls.js:79` 正则 `/\*\*缺失\*\*|缺失：/` 对 `joinLedger\` 不匹配，`action='应存在第三入口'` 也不匹配 `/^未实现/` → `:85` `return null` → **`:908` 判 BLOCKED**。
3. → 主报告明确登记 `**缺失**｜R0-E-38` 的 **`I-LOGIN-10` 被静默降级为 BLOCKED、不在 4 条 FAIL 内**；`I-LOGIN-04` 的 status 同样被污染，**只因 `:106` 硬编码 `bad()` 才仍报 FAIL**。
4. → **不能用 `controls.json.status` 统计 FAIL 数**（会漏 I-LOGIN-04 与 I-LOGIN-10）。

> **「4 条 FAIL 是 C 套件 runner 的判果，不是产品缺陷登记清单」。** 主报告本就登记为缺失，是抽取 / runner 层把它降级或靠硬编码保住。

**本轮只登记、不修复**：不改 `controls.json`、不改 `extract-controls.js`。修复 fixture 污染（`cells()` 支持 `\|` 转义 → 重生成 `controls.json`）属**另一需单独授权**的改动，且恢复 `**缺失**｜R0-E-38` 后 `I-LOGIN-10` 会由 BLOCKED 变 FAIL、**FAIL 数 4→5（是增加不是降低）**，须 PM/Owner 书面裁决 + 全量回归后执行，不得静默处理。

### 5.3 `C:FAIL=4` 不是产品缺陷数量证据（7 环）

1. **4/4 非运行时**：2 条无条件 `bad()`（`:106`、`:175`）+ 2 条 `classifyMissing()` 从 fixture status 文本静态登记（`:78–82`）。
2. **detail 与实测脱节**：宣称的 DOM 观测在代码中从未发生；无截图、无 DOM 快照、无 network/console 诊断。
3. **漏计**：主报告 `:163` 登记 `**缺失**｜R0-E-38` 的 `I-LOGIN-10` 因字段污染被判 BLOCKED，不在 4 FAIL 内 → FAIL 数**小于**已登记缺口数。
4. **未登记的 PRD 缺口**：PRD §12.1 邀请入户已定而 Web 0 实现；D-3 顶栏账本入口 `#navBooks` 永久 hidden。二者均不在 4 FAIL 内 → FAIL 数**不是**产品状态的上界。
5. **验收条件本身错位**：3 条（10/11/12）指向 PRD 未定义的控件形态，1 条（`I-HOME-01`）与 Owner 已裁决口径冲突 → FAIL 数含**非缺陷的口径误报**。
6. **FAIL 数随断言口径漂移**：历史 `dadc9a3` 为 `163 PASS / 5 FAIL`，第 5 条 `I-ACCT-06` 因 `c-controls.js:334–349` 断言改写而消失 → 数字对测试口径敏感，非产品状态稳定度量。
7. **C 是报告型 step**：`job.log:1074` exit 1、`:1082` 报告型，而整 job `conclusion=success`；59 BLOCKED + 1 N/A 未执行；164 PASS 中至少含无断言硬编码 PASS。

> `C:FAIL=4` 只能表述为「C 套件在现有 fixture + runner **登记口径下**的 FAIL 条数」，**不得**表述为「产品缺陷数量 = 4」，也不得通过改判 / 删除 / 跳过来降低。

### 5.4 `164` 的口径注记与 `I-BOOKS-09` **待复核**状态

- `c-controls.js:163` `R['I-BOOKS-09'] = ok('空态与新建后刷新已覆盖（空账本行为见 A07）')` 是**硬编码 PASS、零断言**（`docs/FastLedger-R0-第四次证据补齐任务单.md` §2 L40–41 早已把 I-BOOKS-09/10 等列为硬编码 PASS 待整改）。因此**无论 `#navBooks` 是否可见，这条 PASS 都不是运行时验证通过的 PASS**。
- `#navBooks` 永久 hidden 推翻的是 `controls.json:394–407` 中 I-BOOKS-09 的 `control: 顶栏「账本」` / `status: 已实现｜截图已见｜R0-E-01` 的**可视性前提**；而 runner 断言的是「空态与新建后刷新」，与 `controls.json` 定义存在**编号/定义漂移**——两者对不上，所以 `#navBooks` 不可见并不直接推翻 runner 那条 `ok()`。
- **定性**：`164` 是「**C 套件在现有 fixture + runner 登记口径下的 PASS 条数，至少含 1 条无断言硬编码 PASS**」，**不应**表述为「164 条独立运行时通过」；属任务单 §2 既有问题，**非 `ad34df1` 引入的回归**。
- **`I-BOOKS-09` 复核问题已回答，但用例状态仍为「待复核」，不是已关闭**；本审计**不改判任何 PASS、不调整 164**。「是否影响 164 计数」已明确回答为**不影响**，但 `164` 的表述须收紧为上述「登记口径下的 PASS 条数，至少含 1 条无断言硬编码 PASS」。此前「I-BOOKS-09 待复核（**可能影响 164 PASS 计数**）」的过强表述已由架构师主动撤回；本轮无人复跑该用例。

### 5.5 测试侧最小整改清单（**独立于产品裁决，不因需求待裁决而搁置**）

**通用约束**：改动只落在 `tools/r0/e2e/specs/c-controls.js`（runner）+ 新增证据文件；**`controls.json` 保持逐字不变**（含已损坏字段）。每条改为真实 Playwright 运行时断言（Arrange-Act-Assert），`detail` 由**实测值**生成，禁止模板字符串 / 静态登记；必须附 `shots`（截图）+ `page.content()` 过滤快照或 `attachDiagnostics` 的 console/network 记录，满足 `c-controls.js:7,10` 与 `第七次测试质量纠偏任务单:100`。断言显式区分「控件不存在（`count()===0` + 截图）」与「控件存在但行为不符」；不得再从 `controls.json.status` 推断 FAIL。

| ID | 测试侧可验收标准（摘要，完整见 §2.7 / §3.10 / §4.8） |
|---|---|
| `I-LOGIN-04` | 枚举切换控件候选记 `count()`/`outerHTML`，逐候选 click 读 `type` 前→后→再前三态，登录/注册两模式、两档视口各一遍；`detail` 由 `count/type/mode` 实测拼装 |
| `I-LOGIN-11` | **按 PRD §12.1 流程改写验收域**（不再是「登录页邀请码输入」）：owner 会话 `POST /books/:id/members/invite` 生成有效码读 `data.code`；访问邀请链接 → 断言展示账本名与邀请人 → 设置用户名/密码 → 提交；监听真实 `POST .../members/join`（body 字段 `code`）与响应；核对原子落库、响应不含 accessToken、须再调 `/auth/login`；保留 409 / 429 断言 |
| `I-LOGIN-12` | 复用上条入口，参数化失效态（已用 / 过期 / 随机无效 / 账本 ID 不匹配）；**PASS＝各失效态均有可见的「统一」错误反馈、不静默成功、不误写 token、按钮恢复；不得要求按失效原因区分文案**；`detail` 逐组列 `code, httpStatus, toast, errText`；**不删除、不合并、不跳过** |
| `I-HOME-01` | 判定域改为「顶栏全局可达 + 切换闭环」，两档视口、≥2 账本；`#navBooks` 永久 hidden 单独登记，不合并计数；「验收域改判 / 移出已确认缺陷统计」提请书面裁决 |
| `I-LOGIN-10` | 在 `groupLogin` **新增显式 `R['I-LOGIN-10'] = <真实断言>`** 绕过损坏的 `classifyMissing`；`count()===0` 时由实测值生成 FAIL detail（含 `#view` 内所有 `a/button` 文本列表 + 截图），不再用 BLOCKED 模板。**fixture 修复另行授权**，恢复后 FAIL 4→5 须裁决 + 全量回归 |

### 5.6 现有测试证据索引

| 类别 | 位置 |
|---|---|
| 最新正式 CI | `artifacts/a05-r13-formal-ci/job.log`（run `42467412803`，`head_sha=ad34df1`）：`:1035` A 11/11、`:1049/:1051` D 39/39、`:1063/:1065` C 汇总、`:1069–1073` 4 条 FAIL、`:1074/:1082` 报告型、`:1178` I-LOGIN-10 截图；`artifacts/a05-r13-formal-ci/README.md:7–13,65,75,77` |
| 证据缺口如实登记 | 仓库内**没有** `ad34df1` 该轮的 `report.json` / `report.md` / `C/controls-summary.json` 实体（`artifacts/a05-r13-formal-ci/` 仅 4 个文件，未入库）；「I-LOGIN-10 在 `ad34df1` 仍为 BLOCKED」由 `job.log:1178` 截图路径（只能由 `:907` 产出）+ HEAD 处 runner/fixture 未变**确定性推断**。可直读 C 报告均为更早提交 |
| 历史 C 报告 | `docs/r0-evidence-6th/report.md:15,37,41–45,60,61`；`artifacts/r0-qa-verify/2026-11-05_10-00-44-dadc9a3-c/report.json`（163/5/59/1）；`artifacts/r0-fix-after-c/…/report.json`、`docs/r0-evidence-7th-fix/qa/c-report.json` |
| 主报告 / 裁决 | `基准冻结.md:157,163,164,165,194,190,195,488,804,957,967,1029`；`独立补漏与复核记录.md:329,330`；`收尾执行清单.md:115,119,161,168`；`第七次测试质量纠偏任务单.md:40–51,100`；`第四次证据补齐任务单.md:4–5,16` |
| 截图 | `docs/r0-screenshots/README.md:85–88,91–95,148,192,193,256,309` |
| A / D 覆盖 | `a-scenarios.js` A01–A11（pending / 网络 / 503 / 恢复 / 慢网 / 有数据 / 空数据 / 权限 / 多角色 / 离线 / 同步冲突）；`d-defects.js` D-TXNS / D-CAT / D-MEM（成员邀请**生成侧**）/ D-ACCT。**`tools/r0/e2e/specs/` 目录内检索仅命中 `c-controls.js:106/:137/:175`** → A 11/11、D 39/39 与这 4 条 FAIL 无覆盖关系 |

---

## 6. 独立功能缺口与文档一致性残留（如实登记，不隐去）

### 6.1 两条**不在 4 条 FAIL 内、也不在 228 条控件清单内**的独立事实

1. **PRD §12.1 `[1.12]` 邀请链接入户注册在 Web 端 0 实现**：`server/web/dist/app.js` 对 `members/join|redeem|邀请链接|join(` **0 命中**（仅 L1086 一处 `URLSearchParams` 无关命中），后端亦无「按邀请码反查账本 / 邀请详情」的公开端点承载 L585 的展示步骤。这是**一条 PRD 支撑的、独立的功能缺口**。API 层 `JoinViaInvite` 与 A 套件「真实邀请码兑换」已存在，故准确表述是「**特定 UI 入口与链接落地流程未接**」，不是「产品能力整体缺失」。
2. **D-3 要求的顶栏显式账本切换控件物理不可见**：`index.html:13` `#navBooks class="hidden"`、`style.css:33` `.hidden{display:none!important}`、`app.js:120–154 showNav()` 从不移除、`:236` 仅 `add("active")`；`UI-DESIGN-01` §4.6.3（L434）已把「顶栏『账本』链接永久隐藏，仅能点品牌名」列为顶栏缺陷。

→ **「4 条无一确认缺陷」≠「功能已实现」**；**「已裁决」≠「已实现」**（据 `基准冻结.md:967` 归纳，非逐字引用，见 §0.5）。这两条不在本轮 4 条 FAIL 的处置范围内，本报告**不新增需求、不合并计数、不互相抵消**。

### 6.2 文档一致性残留（HEAD 内两处过期表述，本轮不修改）

| 位置 | 残留原文 | 应依据 |
|---|---|---|
| `docs/FastLedger-R0-独立补漏与复核记录.md:377` | 「\| D-3 \| **待确认** \| 导航结构（桌面 5 项 / 移动 4 Tab + 「我的」中枢）未关闭 \|」 | `基准冻结.md:957`「已裁决」、`收尾执行清单.md:115,161`「已裁决 / 不得再标为待确认」、PRD §12.1 |
| `docs/FastLedger-开发WBS.md:54` | 「\| 1.5 邀请码入户 \| 邀请生成 / 使用 / 撤销 \| 见附录 A 提案 3（**待裁决**）；邀请与公开注册解耦 \|」 | WBS 附录 A 抬头 L180「已由 owner 于 `[1.12]` 全部裁决采纳…与 PRD 冲突时以 PRD 为准」、PRD §12.1【决策】 |

→ 汇编以 PRD §12.1 / `基准冻结.md:957` / `收尾执行清单.md:115,161` 为准，**别把残留读成「D-3 未裁决」或「邀请机制待裁决」**；两处残留作为文档一致性问题如实登记、不隐去、不修改。

---

## 7. 文件改动范围总表

| 类别 | 路径 | 本轮 | 若 Owner 裁决 + 阶段放行后（**方案，非本轮改动**） |
|---|---|---|---|
| **本轮交付** | `docs/FastLedger-R0-C套件剩余4条FAIL-需求与验收口径独立审计报告.md`（本文件，**新增 1 个**） | ✅ 新增 | — |
| 业务运行时 | `server/web/dist/app.js` | ❌ 未改 | 主题一 `renderLogin`；主题二 新增 join 流程；主题三候选 A `renderHome` / 候选 B `showNav` |
| 业务运行时 | `server/web/dist/index.html`、`style.css` | ❌ 未改 | 仅当新增静态内联错误容器 / 选顶栏方案时 |
| 后端 | `server/internal/**`（router / book_handler / common / member / config / ttl / limiter / tx / model / middleware） | ❌ 未改 | **明确不改**；邀请规则一字不动 |
| 测试 fixture | `tools/r0/e2e/fixtures/controls.json` | ❌ 未改 | **明确不改**（字段污染另案授权） |
| 测试 runner | `tools/r0/e2e/specs/c-controls.js` | ❌ 未改 | 测试口径层唯一改动点（§5.5） |
| 抽取脚本 | `tools/r0/e2e/scripts/extract-controls.js` | ❌ 未改 | 修复 `\|` 转义需**单独授权**，且会使 FAIL 4→5 |
| A / D 场景、扫描器、Gate | `a-scenarios.js` / `d-defects.js` / `scan-artifacts.js` / `rv14-binary-evidence.js` 等 | ❌ 未改 | **明确不改** |
| 产品文档 | PRD / 两份设计规范 / WBS / 原型 | ❌ 未改 | 仅 Owner 裁决后方可修订 |
| README / CHANGELOG | — | ❌ 未改 | 本任务单未要求同步；保持单文件最小改动面 |

---

## 8. 红线自证（提交前实测）

```
$ git rev-parse HEAD
ad34df1c8854f792231ed5152b61e2e22d115e8b
$ git rev-parse --abbrev-ref HEAD
fix/r0-sixth-runtime-defects
$ git status --short
?? "docs/FastLedger-R0-C\345\245\227\344\273\266\345\211\251\344\275\2314\346\235\241FAIL-\351\234\200\346\261\202\344\270\216\351\252\214\346\224\266\345\217\243\345\276\204\347\213\254\347\253\213\345\256\241\350\256\241\346\212\245\345\221\212.md"
$ git rev-parse origin/main
dff32746e58b3cad0447091c4a6c0860bef06014
$ git diff --name-only
$ git diff --cached --name-only
```

上述代码块是本轮提交前在 `repo_tmp/` 实跑的**原始 stdout 逐行抄录**，未改写、未以描述性文字冒充命令输出；说明如下：

- `git status --short` 的唯一一行即本审计报告（Git 对非 ASCII 文件名按八进制转义输出），**工作区再无其他新增 / 修改 / 删除**。
- 两条 `git diff` 命令**均无输出**：工作区无已跟踪文件改动、暂存区为空（本文件当时尚未 `git add`）。
- 远端 PR #3 head 于本轮开工时由 `git ls-remote origin refs/heads/fix/r0-sixth-runtime-defects` 实测为 `ad34df1c8854f792231ed5152b61e2e22d115e8b`（与本地 HEAD 一致）；其后复跑该命令两次均遇 GitHub `502 CONNECT tunnel failed`，属已知网络抖动，不改变「远端 head 与本地一致」的结论。
- `origin/main` 取本地 remote-tracking ref（本轮 `git fetch origin --prune` 后的结果），不依赖曾 502 的 `git ls-remote origin refs/heads/main`；该失败命令不作为本结论依据，故不写进代码块。

**红线逐条自证**：

- ✅ **未修改任何业务代码、任何测试脚本（含 `c-controls.js`）、`controls.json`、产品需求（PRD / 规范 / WBS / 原型）、A / D 场景、扫描器本体、RV-14-T01 / case-51 / case-54、A05。**
- ✅ **未把任何 BLOCKED 改判 PASS**；`I-LOGIN-10` 与其余 59 条 BLOCKED、1 条 N/A 原样保留。
- ✅ **4 条 FAIL 全部维持 FAIL**；无一条改判 PASS、无一条删除 / 合并 / 跳过；**未调整 164 / 4 / 59 / 1 / 228 任何计数**。
- ✅ 未替 Owner 做任何决定；`[1.12]` 已确认规则（7 天 / 单次使用 / 角色绑定 / 5·10 限流 / 全生命周期审计 / 原子建 user+book_members / 审计 payload 最小化 / 不泄露账本存在性 / join 不发 token）**一字未改**。
- ✅ 未合并 PR #3 / PR #4；未启动 R1–R7 / UI-DESIGN-02 实施 / PWA；未自行宣布 R0 通过、复核通过或 UI-DESIGN-01 运行时通过。
- ✅ 静态源码 / 文档阅读**未写成**浏览器或真机运行时 PASS；CI 全绿 / A 11/11 / D 39/39 / C step success **未解释为** R0 通过。
- ✅ 三方取证均为纯只读：未 `git add/commit/push`（本报告的提交除外）、未跑任何套件、未启动服务或浏览器；使用 `Read` / `Grep` / `sed -n` / `python json.load` / `git` 只读子命令。

> 说明：以上「红线自证」是**提交前**的工作区快照。本报告提交后从 git 对象回读的验证结果只写入交付回执，不在本目录追加文件——本文件是提交前的证据快照，不承载提交后动作的结果，故不把提交后动作列为「已做」。

---

## 9. 交付边界与后续

| 项 | 状态 |
|---|---|
| 本轮交付 | **1 份独立审计报告**（本文件），提交到 PR #3 分支 `fix/r0-sixth-runtime-defects`，提供 commit SHA |
| 4 条 FAIL | **全部维持 FAIL**，不动状态、不动计数、不改 fixture |
| 分类 | `I-LOGIN-04 / 11 / 12` = `需求待 Owner 裁决`（备注：测试层口径问题，需独立整改）；`I-HOME-01` = `测试口径问题`；**`确认缺陷` = 0 条** |
| Owner 待裁决 | 邀请主题 **5 项**（§3.12）；密码显隐 **4 项**（§2.8）；首页账本切换 **3 项**（§4.9） |
| 后续动作 | **完成后停止，等待 ChatGPT 第三方独立复核**；不合并 PR #3，不启动 R1–R7 / UI-DESIGN-02 实施 / PWA |
| R0 | **仍 BLOCK** |

---

## 附录 A：证据索引（文件 : 行号）

| 主题 | 关键证据位置 |
|---|---|
| 基线 / CI | `artifacts/a05-r13-formal-ci/job.log:1035,1049,1051,1063,1065,1069–1074,1082,1109–1203,1178`；`artifacts/a05-r13-formal-ci/README.md:7–13,65,75,77` |
| PRD | `docs/FastLedger-产品需求文档-PRD.md:186–190,194–198,265,280,435,438,476–575,551,564,581–587` |
| 设计规范 | `docs/FastLedger-Web设计规范.md:206–210,294,301–316,305,307,322,323,330,332`；PR #4 `docs/FastLadger-Web-PWA视觉设计规范.md:86–87,104,110–114,146,155,180,187–193,199,205–214` |
| Owner 决策 | `docs/FastLedger-R0-收尾执行清单.md:115,119,133,149,161,168,186`；`docs/FastLedger-R0-基准冻结与差异盘点.md:98,112–114,136,157,163–165,190,194,195,488,804,957,967,1029`；`docs/FastLedger-R0-第四次证据补齐任务单.md:4–5,16` |
| 缺口登记 | `docs/FastLedger-R0-独立补漏与复核记录.md:329,330,369,377`；`docs/r0-evidence-6th/report.md:15,37,41–45,60,61` |
| 前端运行时 | `server/web/dist/index.html:10–13,28`；`style.css:33,36,41–49,234,328`；`app.js:23–33,95–112,120–154,156,161–231,179,184,236,1086,1667,1690,1698,1963–2005,2234–2275` |
| 后端 API | `server/internal/api/router.go:52,122–138,158,167,168,175,176,177`；`book_handler.go:19–25,44–51,58–62,115–130,132–157,163–177,179–203,205–228,230–247`；`common.go:22–24,27–33,39–136,92–96,109–131`；`middleware.go:36,39,52–93,98–106,197–204`；`auth_handler.go:49–89` |
| 服务端规则 | `server/internal/service/member.go:34–38,52–57,97–109,178–190,204–211,224–238,353–363,384–386,397,408–417,433–435,466–469,483–484,498–507,526–533,550,556–569,572–596,599–631,644–662,684–722,741–758,782–784,800–827,829–853,861–873,876–893,896,919–921,926–964,969–977,999–1070,1087–1091`；`config.go:30–31,128,333–334`；`ttl.go:27,31–39`；`limiter.go:92,120`；`model/model.go:22–27,43–47,82–96`；`model/ledger.go:207–242`；`db/tx.go:1257–1267,1276–1288` |
| 测试 | `server/internal/api/invite_lifecycle_api_test.go:88–119`；`tools/r0/e2e/specs/c-controls.js:7,10,19,22–25,78–86,103–107,137,163,169–176,334–349,895–909`；`tools/r0/e2e/fixtures/controls.json:74–88,170–184,186–200,202–216,214,394–407,442–456,454`；`tools/r0/e2e/scripts/extract-controls.js:25–31,56`；`docs/FastLedger-R0-第七次测试质量纠偏任务单.md:40–51,100` |
| 文档残留 | `docs/FastLedger-R0-独立补漏与复核记录.md:377`；`docs/FastLedger-开发WBS.md:54`；WBS 附录 A 抬头 `:180`、提案 3 `:220–237` |

## 附录 B：可复跑只读核查命令（供第三方复核）

以下命令均为只读，指向**具体文件与行号**，不做全文自指断言：

```bash
# 1. 基线与远端 head
git rev-parse HEAD && git rev-parse --abbrev-ref HEAD && git status --short
git ls-remote origin refs/heads/fix/r0-sixth-runtime-defects

# 2. PRD [1.12] 裁决块与 §10 验收标准 0 命中
sed -n '581,587p' 'docs/FastLedger-产品需求文档-PRD.md'
sed -n '476,575p' 'docs/FastLedger-产品需求文档-PRD.md' | grep -nE '邀请|入户|注册|members/join' || echo '[0 命中]'

# 3. 宽词族（= §2.1 定义）对需求/规范文档 0 命中
grep -nE '明文|显隐|显示密码|隐藏密码|show[ _-]?[Pp]assword|eye|EyeOff|密码可见|type=["]?password|toggle|眼睛|密码切换|切换密码' \
  'docs/FastLedger-产品需求文档-PRD.md' 'docs/FastLedger-Web设计规范.md' || echo '[0 命中]'

# 3b. 窄词族（= §2.1 定义）对原型目录与前端代码 0 命中
grep -rnE '显隐|显示密码|隐藏密码|show[ _-]?[Pp]assword|toggle[ _-]?[Pp]assword|eye-off|EyeOff|密码可见|密码切换|切换密码|眼睛' \
  prototype prototype-v2 server/web/dist/index.html server/web/dist/app.js server/web/dist/style.css || echo '[0 命中]'

# 3c. 通用词命中（非 0；即 §2.1 口径澄清披露的三类，均与密码显隐无关）
grep -c '明文' server/web/dist/app.js                 # 3  -> API Token 一次性明文展示
grep -c 'type="text"' server/web/dist/index.html      # 8  -> 普通文本输入框（不匹配宽词族）
grep -c 'toggle' server/web/dist/app.js               # 16 -> classList.toggle

# 3d. 宽词族对 server/web/dist 的命中数（= 19 = 3 明文 + 16 toggle；§2.1 口径澄清的数值）
grep -rnE '明文|显隐|显示密码|隐藏密码|show[ _-]?[Pp]assword|eye|EyeOff|密码可见|type=["]?password|toggle|眼睛|密码切换|切换密码' \
  server/web/dist --include='*.js' --include='*.html' --include='*.css' | wc -l

# 4. D-3 与 R0-E-01 本体
grep -n 'D-3' 'docs/FastLedger-R0-收尾执行清单.md' | head -5
sed -n '804p;957p;967p' 'docs/FastLedger-R0-基准冻结与差异盘点.md'

# 5. 顶栏 hidden 与首页 renderHome 行号
grep -n 'topbar\|navBooks\|brand' server/web/dist/index.html | head -8
grep -n 'function showNav\|function renderLogin\|function renderMembers\|function renderHome\|classList.add("active")' server/web/dist/app.js
grep -n '\.hidden' server/web/dist/style.css | head -3

# 6. join 路由 / handler / 错误码
sed -n '122,138p' server/internal/api/router.go
sed -n '179,203p' server/internal/api/book_handler.go
sed -n '105,125p' server/internal/api/common.go
sed -n '615,622p;800,810p' server/internal/service/member.go

# 7. C 套件判定机制与 fixture 污染
sed -n '75,90p;103,107p;134,140p;160,166p;169,176p;903,912p' tools/r0/e2e/specs/c-controls.js
sed -n '25,31p' tools/r0/e2e/scripts/extract-controls.js
python -c "import json;d=json.load(open('tools/r0/e2e/fixtures/controls.json',encoding='utf-8'));items=d if isinstance(d,list) else d.get('controls',d);[print(x.get('id'),'|',repr(x.get('evidence')),'|',repr(x.get('status'))) for x in items if x.get('id') in ('I-LOGIN-04','I-LOGIN-10','I-LOGIN-11','I-LOGIN-12','I-HOME-01')]"

# 8. 最新正式 CI 的 C 汇总与 4 条 FAIL
grep -nE 'C 汇总|SUMMARY \{"C|I-LOGIN-04 FAIL|I-LOGIN-11 FAIL|I-LOGIN-12 FAIL|I-HOME-01 FAIL|包 C 退出码' artifacts/a05-r13-formal-ci/job.log

# 9. PR #4 分支（未合并）文档复跑：PWA 视觉规范 / WBS 附录 B / UI-DESIGN-01 盘点报告
#    本地需先 fetch 该分支；Git Bash 下含冒号的 git show 需 MSYS_NO_PATHCONV=1
export MSYS_NO_PATHCONV=1
git ls-remote origin refs/heads/docs/homeledger-inspired-design-system   # 期望 head = 16705fd…
git show 16705fd:docs/FastLadger-Web-PWA视觉设计规范.md | grep -nE '明文|显隐|账本切换|D-3' | head -20
git show 16705fd:docs/FastLedger-开发WBS.md | sed -n '244,267p'
git show 16705fd:docs/FastLadger-UI-DESIGN-01-Web页面与组件盘点报告.md | sed -n '172p;442p'
#    原型目录按宽词族的通用词命中（4 文件，§2.5 补注依据；非密码显隐）
grep -rlE '明文|显隐|显示密码|隐藏密码|show[ _-]?[Pp]assword|eye|EyeOff|密码可见|type=["]?password|toggle|眼睛|密码切换|切换密码' prototype prototype-v2 | wc -l
```

> 注：`artifacts/` 被 `.gitignore` 忽略、未入库；`job.log` 为本地下载的 GitHub Actions 原始 job 日志，第三方可用 run `42467412803` 的 `GET /actions/jobs/128690050693/logs` 独立复取。

---

**（本报告完。编制完成后停止，等待 ChatGPT 第三方独立复核。）**
