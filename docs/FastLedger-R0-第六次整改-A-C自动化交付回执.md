# R0 第六次整改 · A/C 自动化 + B 分类解析补充整改 交付回执

> 基线：`origin/main = ebab48d`（在 `22fd310` 之后）；业务代码仍 `bff6a2b`（`server/`、`client/` 零改动）。
> 本轮**没有**要求 Owner 使用电脑、**没有**要求逐条手点 228 个控件、**没有**授权 R1–R7、**没有**启动 PWA。
> 所有结论来自真实执行：`node --test`、真实 Go 服务 + 临时 SQLite、真实浏览器（本地 Edge 156 / CI chromium）。

---

## 0. 一句话结论

| 工作包 | 本轮做了什么 | 结论 |
|---|---|---|
| **B（补充整改）** | 把分类解析抽成纯函数模块，加入"冲突/歧义/失败关闭"三类规则 + 14 条合成测试；重跑 22 笔审计 | **PARTIAL**（规则硬化完成、审计与基线一致；B 的截图覆盖缺口未变） |
| **A（11 项复验）** | 从"给 Owner 写步骤"改成 **Playwright 自动化 + 独立 CI workflow**，11 项全部脚本化 | **本地 11/11 + 独立 CI 11/11 均 PASS**（run `41990345757`）→ 具备申请改判条件，改判权仍在 Owner |
| **C（228 条控件）** | 解析 228 条 → `controls.json`，逐条真实执行，输出 PASS/FAIL/BLOCKED/N/A + 截图 | **144 PASS / 6 FAIL / 77 BLOCKED / 1 N/A**（未整页 PASS，未用静态定位冒充） |
| **D** | 未改判 | **PARTIAL**（3/5/5 维持，不凑覆盖率） |
| **RV-05** | 未触碰 | 仍 **Owner ACTION REQUIRED**（可见性/历史清理） |

---

## 1. 包 B · 分类解析补充整改

### 1.1 问题与修法

第三方指出三处隐患，逐条对应：

| 指出的问题 | 处理 |
|---|---|
| `resolveCategory` 先命中 `categoryMap[categoryId]` 就返回，父类 ID 会静默吃掉明确的子类展示名 | 抽出 `tools/r0/lib/resolve-category.js`，先**收集候选再裁决**：ID 命中父类 + 展示名指向其子类 → **子类胜出**（`expand-child`）；ID 与展示名指向**不同叶子** → 抛 `CATEGORY_CONFLICT` |
| `childByPath` / `topByName` 用 Map 时后者覆盖前者，重复名称无检测 | 索引改为记录**全部候选**；命中多个 → `NAME_AMBIGUOUS`（同级同名、大类同名各有用例） |
| 顶部注释没写"唯一前缀匹配"，与实现不一致 | 注释重写为 6 条完整优先级，并注明"唯一事实来源是 `lib/resolve-category.js`，改一处必须同步另一处" |
| 无法确定时是否猜测 | **一律失败关闭**：`CATEGORY_CONFLICT` / `CATEGORY_CHILD_UNRESOLVED` / `PREFIX_AMBIGUOUS` / `NAME_AMBIGUOUS` / `CATEGORY_UNRESOLVED` 五种错误码 |

### 1.2 真实数据里立刻抓到一处矛盾（这就是价值）

严格规则第一次运行就失败在 `t021`：

```
categoryId = "meal"（三餐，有效子类）
categoryName = "餐饮-聚餐"（dinner，另一个有效子类）
```

旧实现按"精确 ID"静默判成**三餐**，展示名被忽略。新规则判为冲突并拒绝择优。
处理方式不是改回"猜"，而是新增**显式、可审计的覆盖表** `tools/r0/category-overrides.json`：

```json
{ "txId": "t021", "categoryId": "dinner",
  "reason": "原型数据矛盾：categoryId=meal（三餐）但展示名「餐饮-聚餐」=dinner。B 的判据是「两侧可见结果一致」，以用户可见的展示名为准，故取 dinner。",
  "decidedBy": "WorkBuddy（R0 第六次整改，分类解析补充整改）", "decidedAt": "2026-11-05" }
```

约束：覆盖表缺任一字段（reason/decidedBy/decidedAt）直接报 `OVERRIDE_INVALID`；覆盖值指向不存在的分类仍按冲突失败关闭（用例 9b 覆盖）。

### 1.3 合成测试（`node --test tools/r0/test-resolve-category.js`）

`14 tests / 14 pass / 0 fail`（`docs/r0-evidence-6th/unit-resolve-category.log`）。覆盖：

- 父类 ID + 子类展示名 → `expand-child`（不得用父类）
- 父类 ID + 父类展示名 → 用父类（不是猜）
- 同级同名 / 大类同名 → `NAME_AMBIGUOUS`
- 多前缀候选 → `PREFIX_AMBIGUOUS`（不得取第一个）
- 唯一前缀（t017 真实形态）→ `name-prefix`
- ID 与展示名不同叶子 → `CATEGORY_CONFLICT`
- 展示名到子类但只解析到父类 → `CATEGORY_CHILD_UNRESOLVED`
- 完全无法定位 → `CATEGORY_UNRESOLVED`（绝不返回父类猜测）
- 冲突 + 显式覆盖 → `manual-override`；覆盖值非法 → 仍然冲突

### 1.4 22 笔审计重跑（未破坏既有结果）

`node tools/r0/e2e/scripts/reseed-audit.js` → **AUDIT_OK**（`docs/r0-evidence-6th/reseed-audit.log`）：

| 命中方式 | 第六次 | 第五次 | 说明 |
|---|---|---|---|
| `id` | 17 | 18 | −1：t021 改判走覆盖 |
| `expand-child` | 4 | 0 | t007/t009/t010/t017：原本是父类 ID + 子类展示名，现在归类更准确 |
| `manual-override` | 1 | 0 | t021 |
| `name-path` / `name-prefix` | 0 / 0 | 3 / 1 | 被 `expand-child` 取代（**落点完全相同**） |
| 父类兜底 | **0** | 0 | 未新增任何"降级到父类"的行为 |

**落点变化只有 1 笔**：t021 由「三餐」改为「聚餐」（与原型展示一致）。其余 21 笔落点与第五次逐条相同。
影响：B 的报表首行分类口径可能变化，**新旧报告数字不可混用**；如需重采 104 张对照图须另起一轮（本轮未重采，如实标注）。

---

## 2. 包 A · 11 项从"文档步骤"改成"自动化"

### 2.1 环境（可独立启动、不依赖本机状态）

`tools/r0/e2e/lib/server.js`：临时目录 SQLite + 随机端口 + **每次新生成** JWT 密钥 → `db migrate` → 起进程 → 轮询 `/healthz` 就绪。结束即销毁，全流程不读生产库、不用生产凭据。

`tools/r0/e2e/lib/fixtures.js`：owner 用 CLI 建；admin/member 走**真实邀请码兑换**（未认证入口）建号，随后登录取 token —— 三条角色链路都是真实路径。

### 2.2 11 项结果（本地 Edge 156.0.4314.58）

| ID | 场景 | 实现手段 | 结果 |
|---|---|---|---|
| A01 | pending | 拦截 `/auth/login` 延迟 1.5s，轮询按钮文案/禁用 | **PASS**（`登录中…` + `disabled=true`） |
| A02 | 网络拒绝 | `route.abort('failed')` | **PASS**（toast `Failed to fetch`，不写 token） |
| A03 | HTTP 503 | `route.fulfill(503)` | **PASS**（toast `服务暂时不可用`） |
| A04 | 恢复 | 首次 abort、二次放行 | **PASS**（重试后 token 写入） |
| A05 | 慢速网络 | CDP `Network.emulateNetworkConditions` 延迟 800ms | **PASS**（耗时 1135ms，中途仍显示 pending） |
| A06 | 有数据 | 种子 3 笔交易 | **PASS**（页面含 ¥ 与「早餐/打车/工资」） |
| A07 | 空数据 | 第二个空账本 | **PASS**（API 0 笔、页面无他账本数据） |
| A08 | 权限不足 | member 会话调 `DELETE /books`、`POST /members/invite` | **PASS**（均 403） |
| A09 | 多角色 | owner/admin/member 三 context | **PASS**（成员数均 3） |
| A10 | 离线/恢复 | `context.setOffline(true/false)` | **PASS**（离线报错、恢复后数据回来） |
| A11 | 同步冲突 | 同 `clientUuid` 二次 push 且改金额 | **PASS**（首次 `created`、重放 `conflict`，交易数只 +1） |

### 2.3 独立 CI 复跑（已通过）

- 本地首跑 11/11 PASS 后推送（`825f3c7`），新增的 `R0 E2E` workflow **首次运行即 success**：run [`41990345757`](https://github.com/LinTia/FastLadger/actions/runs/41990345757)。
- CI 日志原文：`SUMMARY {"A:PASS":11,"total":11}`；浏览器是 **Playwright 自带 chromium 156.0.8078.4**（与本地 Edge 156.0.4314.58 不同构建），**结果完全一致** → 结论不依赖某台机器/某个浏览器。
- 证据：`docs/r0-evidence-6th/ci-run-825f3c7/`（run-meta、58KB 原始 job.log、summary、SHA256SUMS）。
- 按任务单 §四，CI 通过只是"**允许申请改判**"的前提；**是否改判仍由 Owner 决定**，本轮不自行把 A 写成 PASS。

### 2.4 边界（不夸大）
- 两个场景仍是"模拟而非真故障注入"：A02/A03 用浏览器层拦截（不代表服务端真实返回 503），A05 用 CDP 限速（不代表真实弱网抖动）。这一点在报告里逐条写明。

---

## 3. 包 C · 228 条逐控件验收（真实执行）

### 3.1 怎么做的

1. `tools/r0/e2e/scripts/extract-controls.js` 从主报告 §3.1 解析出 **228 条 × 13 字段** → `tools/r0/e2e/fixtures/controls.json`（附文档 SHA256，保证"清单版本=执行版本"）；
2. 16 个页面分组各写一个执行器（登录/账本/首页/记一笔/详情/总表/账户/分类/成员/我的/设置/报表/预算/投资/导出/回收站），**真实点击、输入、提交、删除、恢复、改角色、导出下载**；
3. 每条输出 `PASS / FAIL / BLOCKED / N/A` + 说明 + 截图；未覆盖的分组项写明**具体技术原因**。

### 3.2 结果

| 状态 | 数量 | 含义 |
|---|---|---|
| **PASS** | **144** | 真实交互或真实渲染断言通过 |
| **FAIL** | **6** | 4 条是已登记的产品缺口，2 条是本轮新发现（见 §3.3） |
| **BLOCKED** | **77** | 见 §3.4，每条都有具体阻塞说明 |
| **N/A** | **1** | I-HOME-02 月份切换为待裁决交互 |
| 合计 | **228** | 与主报告行数一致 |

### 3.3 6 条 FAIL（逐条）

| ID | 现象 | 性质 |
|---|---|---|
| I-LOGIN-04 | 无「密码明文切换」控件 | 已登记缺失 **R0-E-39** |
| I-LOGIN-11 | 产品侧未实现 | 已登记 **R0-E-38** |
| I-LOGIN-12 | 产品侧未实现 | 已登记 **R0-E-38** |
| I-HOME-01 | 首页无账本切换控件 | 已登记缺失 **R0-E-01** |
| **I-TXNS-01** | **交易总表整页空白**（`#view` 无任何节点） | **本轮新发现缺陷**：`server/web/dist/app.js` 的 `renderTxns()`（L337–L527）建好了 `card` 并 `await load()`，但**函数内从未 `view.append(card)`**（对比 `renderHome` L230 / `renderBooks` L302），页面被 `view.replaceChildren()` 清空后没有回填 → 交易总表不可用，连带 18 条控件无法执行 |
| **I-ACCT-06** | 新增账户：API 回读**存在**、页面 6s 内**不出现** | **疑似列表未刷新**（`renderAccounts` 调用后未重绘/时序问题）；标注"需 QA 复现确认"，未直接定性 |

另有两条**非 C 计分**但同轮发现的实现侧问题：

- **记一笔分类未按类型过滤**：`app.js:535` `fillSelect($("#fCategory"), state.categories)` 把全部收入+支出分类一起塞进下拉；支出类型下首项是「工资」，选中保存会被服务端 400 拒绝（I-ADD-06b 记为 FAIL）。
- **member 仍看到「邀请成员」按钮**（A08 记录）：服务端 403 正确，但 UI 未按角色隐藏。

### 3.4 77 条 BLOCKED 的具体原因（不是"Owner 负责"）

| 原因 | 条数 | 说明与替代方案 |
|---|---|---|
| 交易总表渲染为空（同上缺陷） | 18 | 页面不渲染就无控件可点；修复 `view.append(card)` 后可直接跑通 |
| 该组后续控件未以独立控件出现（图标/颜色/排序/合并/结转开关/多角色详情等） | ~30 | 页面上确实没有对应元素（不是选择器问题）；需产品先明确形态 |
| 依赖前置数据链（持仓行/盈亏色/行情手输/删除持仓/邀请码列表） | ~14 | 单次运行内无法构造完整链（需先建标的→持仓→行情）；可在下一轮加"分阶段夹具"解决 |
| 相机/OCR/文件选择/剪贴板/手势类 | 6 | 浏览器层无法产生可信输入；替代方案：CI 内固定夹具图片 + OCR stub（属新功能）、`setInputFiles` 需先有 `input[type=file]` |
| 需要并发/多端会话 | 3 | 需两个浏览器 context 同时操作同一实体；可实现，但当前页面无冲突提示 UI，断言目标不存在 |
| 注册模式入口不可达（登录页无注册链接） | 2 | 页面没有进入注册态的控件，需直接 `renderLogin('register')`（前端路由未暴露） |
| 其它（分页/滚动加载未实现等） | ~4 | 功能不存在，无可断言目标 |

---

## 4. CI 如何承接（不给 Owner 手工活）

新增 `.github/workflows/r0-e2e.yml`（与 `ci.yml` 分工：`ci.yml` 管"退出码即结论"，本文件管"要真浏览器的运行时验收"）：

```
checkout → setup-go 1.22 → go build ./cmd/server
        → setup-node 22 → npm ci && npm i --no-save playwright@1.64.1
        → npx playwright install --with-deps chromium
        → node --test tools/r0/test-resolve-category.js      # B 规则单测
        → node tools/r0/e2e/scripts/reseed-audit.js          # B 22 笔审计
        → node run.js --suite a        (门禁：FAIL/BLOCKED 即红)
        → node run.js --suite c        (报告型：continue-on-error)
        → upload-artifact: artifacts/r0-e2e/**（日志/截图/trace/报告/SHA256）
```

要点：

- **干净环境**：runner 上从零构建二进制；实例用临时目录 SQLite；每次新密钥；不读任何本机状态。
- **门禁分工**：A 失败就红（因为 A 是门禁）；C 是"报告型"，因为它记录的是**产品缺口分布**，不该让"产品有缺口"把流水线染红而掩盖其它信号。
- **证据可下载**：`r0-e2e-evidence-<sha>` artifact，含 `report.json` / `report.md` / 每张截图 / trace.zip / `SHA256SUMS.txt` / `env`（Node、OS、浏览器版本、GitHub run 元数据）。

---

## 5. 证据清单（本仓库内）

| 路径 | 内容 |
|---|---|
| `docs/r0-evidence-6th/report.json` / `report.md` | A+C 全量结果（239 行记录，含逐条说明与耗时） |
| `docs/r0-evidence-6th/controls-summary.json` | 228 条状态分布 + 源文档 SHA256 |
| `docs/r0-evidence-6th/fixtures.json` | 本轮合成数据快照（用户名/账本/账户/分类 id，不含任何令牌） |
| `docs/r0-evidence-6th/unit-resolve-category.log` | 14/14 通过的原始输出 |
| `docs/r0-evidence-6th/reseed-audit.log` | 22 笔映射 + 审计结论 |
| `docs/r0-evidence-6th/screenshots/*.png` | A 关键态 + C 失败项截图（含交易总表空白页） |
| `docs/r0-evidence-6th/SHA256SUMS.txt` | 上述 15 个文件校验和（`sha256sum -c` 全 OK） |
| CI artifact | 完整 trace 与全部截图（体积大，走 Actions artifact，不入库） |

> 说明：`tools/r0/e2e/node_modules/` 与本地 `artifacts/` 均已加入 `.gitignore`，不入库。

---

## 6. 仍必须 Owner 亲自处理的事（仅 3 件，且都只需手机/网页）

| # | 事项 | 为什么必须 Owner | 预计动作 |
|---|---|---|---|
| 1 | **RV-05 仓库可见性**：旧 `259bd11` 仍可匿名读取，仓库仍 Public | 涉及仓库所有权与历史处置，WorkBuddy 不得擅自改 Private / force-push / 重写历史 | GitHub 手机网页 → Settings → Danger Zone → Change visibility |
| 2 | **第三方复核入口**：把本次提交贴到 issue #2 | 需要你的账号发言（我不能代发） | 手机上把 commit SHA 贴进 issue |
| 3 | **A 改判决定**：CI 已绿（run `41990345757`） | 门禁裁决权在 Owner | 回复"申请 A 改判"或"维持观察"即可 |

> 除此之外**不需要**你在手机上点 228 个控件，也不需要电脑。

---

## 7. 状态边界（严格按任务单 §四）

- **A**：本地 11/11 + 独立 CI 11/11 均 PASS（run `41990345757`）→ **具备申请改判条件**；本轮**不自评 PASS**，等 Owner 裁决。
- **C**：**已逐条执行并统计**，144/6/77/1，**不整页 PASS**；其中 2 条新缺陷已单列。
- **B**：补充整改完成（规则+测试+审计），但角色/状态维度缺口未变 → 仍 **PARTIAL**。
- **D**：维持 **PARTIAL**（3/5/5），本轮未碰。
- **RV-05**：未触碰。
- **R0 整体：仍 BLOCK**，不授权 R1–R7，PWA 未启动。

## 10. P1 凭据扫描器 K1-K3 安全绕过修复（新增，独立于 §1-§7）

第三方在 `659c0c3` 上复现的 K1（编码绕过）/ K2（短值低熵漏报）/ K3（字段名与 Base64 绕过）已由独立任务单整改，详见 `docs/r0-evidence-7th-fix/p1/k1-k3-remediation.md`。要点：

- 修复前：12 个可复现绕过样本在 `659c0c3` 扫描器上全部判 CLEAN（绕过实证）；修复后全部 BLOCKED，正常业务字段仍 CLEAN（无误报）。
- 新增 13 项负向合成测试（case-18~30），本地 30/30 通过；CI 新增「凭据扫描器负向合成测试」门禁与「失败关闭（fail-closed）集成验证」step。
- 上传闸门维持 fail-closed：`artifact_upload` 仅当 `artifact_scan.outcome == 'success'`，无 `continue-on-error`，扫描失败不执行上传。
- 本轮不合并 main、不宣布 R0 通过；CI Run / SHA / Artifact 独立扫描见 §9-§10 回填记录。
- 不在本轮范围：OBS-ACCT-VIS-390、13 条历史控件 ID 映射、RV-05 旧 SHA 公开历史、仓库 Private 决策。
