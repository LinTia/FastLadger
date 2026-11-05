# FastLedger R0 第五次最小整改 · 执行回执

> 执行日期：2026-11-05　执行主体：WorkBuddy（**仅审计/证据/文档整改，无业务实现**）
> 依据：`docs/FastLedger-R0-第五次最小整改任务单.md`
> **结论：R0 仍 BLOCK；不授权 R1–R7；PWA 未实施。A/C 保持 BLOCKED，B/D 由 PASS 改判 PARTIAL，RV-05 为 Owner ACTION REQUIRED。**

---

## 0. 开工前对齐（任务单 §0）

| 项 | 实测 |
|---|---|
| 开工时 `origin/main` HEAD | `72cbab71253d031a09acc405cad25155506db531`（`72cbab7`，2026-11-05 12:22:31 +0800，提交「docs(r0): add fifth remediation task to README」） |
| 工作区状态 | 干净（`git status --short` 为空） |
| 业务代码基准 | `bff6a2b351bf5a039cdf7c762ec2693fb330cd00`；`git diff --stat bff6a2b 72cbab7 -- server/ client/` 输出为空（0 行） |
| 读取的必读文档 | `workbuddy/多AI对齐规范.md`（242 行）、`workbuddy/第三方审阅者导读.md`（203 行，blob `0884cd1`）、`CHANGELOG.md` 的 [1.73]–[1.77]、`docs/FastLedger-R0-第四次证据补齐任务单.md`、`docs/FastLedger-R0-第四次证据补齐-执行回执.md`、主报告 §0/§3.1/§5.3/§7/§8/§10、`README.md`、`docs/FastLedger-R0-收尾执行清单.md`（191 行） |
| 本地无法确认项 | 无（本轮无需访问服务器或生产环境） |

---

## 1. 四包状态（本轮统一口径）

| 工作包 | 任务单要求 | 本轮实际执行 | 结论 | 证据 |
|---|---|---|---|---|
| **A. R0-E-35 / RV-01 独立复验** | 保持 BLOCKED | 未做任何独立复跑，未改动既有运行时证据 | **BLOCKED** | `docs/r0-screenshots/runtime/`（历史，未改） |
| **B. 13 页原型×实现对照** | PARTIAL，不得标 PASS | 修复分类映射并重跑种子 + 两侧重采 104 张 + 逐文件校验 + 角色改真实读取 | **PARTIAL** | `docs/r0-screenshots/pairs/`、`docs/r0-evidence-5th/` |
| **C. 228 条控件运行时验收** | 保持 BLOCKED | 未执行任何一条真实交互，未用截图/静态映射替代 | **BLOCKED** | 主报告 §3.1（228 行静态清单） |
| **D. 逐路由测试证据** | PARTIAL，不得标 PASS | 更正 CI 元数据、拆清四类 SHA、逐路由复核 13 条并逐条处置 | **PARTIAL** | `docs/r0-evidence-5th/ci-metadata-correction.md`、`api-route-coverage.md`、`docs/r0-evidence-4th/ci-run/` |

---

## 2. 工作包 A：R0-E-35 / RV-01（BLOCKED，保持）

- **未执行**：本轮未启动服务、未采运行时证据、未做任何独立复跑；既有 5 PNG + 3 JSON 未改动一字节。
- **历史事实保留**：本机隔离复验曾观察到失败态 `.skel=4`、`aria-busy=true` 未清除、仅 toast 且无页内重试；重新导航可恢复。**重新导航恢复 ≠ 缺陷修复**。
- **仍待 Owner/CI 独立执行的场景**（11 项）：pending、网络拒绝、HTTP 503、解除拦截后恢复、慢速网络、有数据态、空数据态、权限不足、多角色、离线/恢复、同步冲突。
- **解除条件**：Owner/CI 在独立环境提供可复核原始证据（命令、环境、退出码、日志/SHA256）后，才可申请重新评估状态。
- **责任人**：Owner / CI。

---

## 3. 工作包 B：13 页成对对照（PARTIAL）

### 3.1 分类映射修复（第三方指出的 4 笔）

**根因**：`tools/r0/seed-from-dataset.js` 的解析顺序把「补 `cat_` 前缀」放在名称匹配之前，而 t007/t009/t010/t017 的原型 `categoryId` 是**不带前缀的父类名**（`shopping`/`travel`），于是先命中 `cat_shopping`/`cat_travel` 父类，永远走不到「购物-服饰」这类子类。

**修复**：解析优先级改为

```
① 原型分类 ID 精确命中                     （稳定 ID 优先）
② 交易展示名「父-子」按分类树路径精确命中
③ 同父类下按展示名"唯一前缀"匹配（仅唯一候选才命中）
④ 规范化 ID（补 cat_ 前缀）
⑤ 最后才按父类降级，且必须登记到 categoryFallbacks
```

第 ③ 步是本轮新增：t017 的展示名是「购物-数码」，而子类实名是「购物-数码家电」，精确路径匹配不到，按"同父类唯一前缀"命中 `shopping_digital`；多候选时**不猜**，退回 ④/⑤ 并登记。

**修复前后对照**：

| 交易 | 展示名 | 修复前（原型分类） | 修复后（原型分类） | 命中方式 | 实现 categoryId |
|---|---|---|---|---|---|
| t007 | 购物-服饰 | `cat_shopping`（父类「购物」） | `shopping_clothes`（「购物-服饰」） | name-path | 51 |
| t009 | 旅游-住宿 | `cat_travel`（父类「旅游」） | `travel_hotel`（「旅游-住宿」） | name-path | 55 |
| t010 | 旅游-门票 | `cat_travel`（父类「旅游」） | `travel_ticket`（「旅游-门票」） | name-path | 56 |
| t017 | 购物-数码 | `cat_shopping`（父类「购物」） | `shopping_digital`（「购物-数码家电」） | name-prefix | 53 |

机器可读证据：修复前 `docs/r0-evidence-5th/seed-mapping-before.json`、修复后 `docs/r0-evidence-5th/seed-mapping-after.json`。

**统计聚合影响**：报表「本月支出分类占比」首行由「购物 ¥580（53%）」变为「服饰 ¥580（53%）」，其余行不变。因此**修复前后的报表数字不可混用**，旧版校验和保留为 `SHA256SUMS-4th-round.txt`。

### 3.2 22 笔交易映射审计（全量，不只核 4 笔）

汇总：`id` 18 笔 / `name-path` 3 笔 / `name-prefix` 1 笔 / **父类降级 0 笔**。

| 交易 | 实现 txId | 原型 categoryId | 原型展示名 | 解析后原型分类 | 解析后名称 | 实现 categoryId | 命中方式 |
|---|---|---|---|---|---|---|---|
| t001 | 1 | meal | 餐饮-早餐 | meal | 餐饮-三餐 | 43 | id |
| t002 | 2 | taxi | 交通-打车 | taxi | 交通-打车 | 47 | id |
| t003 | 3 | meal | 餐饮-午餐 | meal | 餐饮-三餐 | 43 | id |
| t004 | 4 | takeout | 餐饮-外卖 | takeout | 餐饮-外卖 | 44 | id |
| t005 | 5 | bus | 交通-公交 | bus | 交通-公交地铁 | 48 | id |
| t006 | 6 | salary | 收入-工资 | salary | 工资-工资 | 64 | id |
| t007 | 7 | shopping | 购物-服饰 | shopping_clothes | 购物-服饰 | 51 | name-path |
| t008 | 8 | grocery | 购物-超市 | grocery | 购物-超市日用 | 52 | id |
| t009 | 9 | travel | 旅游-住宿 | travel_hotel | 旅游-住宿 | 55 | name-path |
| t010 | 10 | travel | 旅游-门票 | travel_ticket | 旅游-门票 | 56 | name-path |
| t011 | 11 | movie | 娱乐-电影 | movie | 娱乐-电影演出 | 58 | id |
| t012 | 12 | meal | 餐饮-晚餐 | meal | 餐饮-三餐 | 43 | id |
| t013 | 13 | bonus | 收入-奖金 | bonus | 奖金-奖金 | 65 | id |
| t014 | 14 | transfer | 转账-还款 | transfer | 转账 | 42 | id |
| t015 | 15 | grocery | 购物-超市 | grocery | 购物-超市日用 | 52 | id |
| t016 | 16 | meal | 餐饮-午餐 | meal | 餐饮-三餐 | 43 | id |
| t017 | 17 | shopping | 购物-数码 | shopping_digital | 购物-数码家电 | 53 | name-prefix |
| t018 | 18 | taxi | 交通-打车 | taxi | 交通-打车 | 47 | id |
| t019 | 19 | rent | 居家-房租 | rent | 居家-房租 | 61 | id |
| t020 | 20 | utility | 居家-水电燃气 | utility | 居家-水电燃气 | 62 | id |
| t021 | 21 | meal | 餐饮-聚餐 | meal | 餐饮-三餐 | 43 | id |
| t022 | 22 | other_income | 收入-其他 | other_income | 其他收入-其他 | 67 | id |

### 3.3 重采结果与字节级校验

| 项 | 结果 |
|---|---|
| 采集 | 原型 **52** + 实现 **52** = **104** PNG；13 页 × 4 视口（`360x800`/`390x844`/`430x932`/`1280x800`） |
| 错误 | 两侧 `errors=0`、`skipped=0` |
| 环境 | Edge `Edg/156.0.4314.58`（headless + CDP）、Node `v22.22.2`、DPR 1；隔离实例端口 8092、schema v8、一次性随机 `EBK_JWT_SECRET` |
| 字节校验 | `sha256sum -c SHA256SUMS.txt` → **OK 109 / FAILED 0 / PNG_OK 104**（日志 `docs/r0-evidence-5th/checksum-verification.txt`，逐文件一行） |
| PNG 完整性 | 104 个文件签名均为 `89504e47`，无 0 字节/异常小文件 |
| 数据集 | 5 账户 / 37 分类 / 22 交易 / 5 预算；余额两侧一致（¥28,100.00 / ¥0.00 / −¥716.30 / −¥1,125.00 / ¥3,868.00） |

**跨次采集的字节差异说明**：本轮重采后，原型侧 `login`、`add-transaction`、`report` 等页面的 PNG 与上一轮**字节不同**——原因是原型页面渲染了"当前时间"（如记一笔的时间字段）。这属预期差异，不是数据变化；以本轮 `SHA256SUMS.txt` 为准。

**被替换的文件与原因**：`docs/r0-screenshots/pairs/{prototype,impl}/*.png`、`manifest-prototype.json`、`manifest-impl.json`、`seed-manifest.json` 全部替换为修复后版本；旧校验和保存为 `SHA256SUMS-4th-round.txt`（109 行，含旧版全部文件哈希）。替换原因：分类映射修复改变了 t007 等交易的分类归属，影响报表与分类页渲染。

### 3.4 角色与状态覆盖矩阵

| 维度 | 已覆盖 | 未覆盖 | 证据 |
|---|---|---|---|
| 角色 | `owner`（1 个） | `admin`、`member`、只读、回收站视角 | `manifest-impl.json` → `role: owner`、`roleSource: api:/users/me + /books/:id/members`、`bookMembers: [{uid:1, username:"admin", role:"owner"}]` |
| 状态 | `default` 44 张、`logged-out` 4×2 张、`dialog-open` 4 张 | `loading`、`empty`、`error`、`permission`、`offline`、`sync-conflict` | `manifest-impl.json` 逐张 `state` 字段 |
| 页面 | 11 页数据对齐（login/home/add/detail/accounts/categories/budget/report/export/mine/settings） | `invest`（无同等持仓数据）、`members`（原型 2 人 / 实现 1 人） | `pairs/README.md` §3 |
| 页面计数 | 独立原型路由 **12 个** | — | `settings` 与 `mine` 同为 `/profile`，两图 SHA256 相同，不按两个独立页面计 |

> 角色字段本轮改为**从服务端成员数据读取**（`GET /users/me` 取 uid → `GET /books/:id/members` 取 role 并把枚举 `1/2/3` 映射为 `owner/admin/member`），不再把登录名写进 `role`。采集只以 owner 身份进行，未伪造其他角色。

### 3.5 B 的结论与遗留

- **PARTIAL**：104 张同数据集、同视口、含 2 类动作级状态，字节级校验通过；但**角色单一、状态维度不全、2 页数据未对齐**。
- **遗留与责任人**：多角色与状态矩阵 → QA/前端；`invest` 持仓数据 → 需 instrument 前置（后端接口）；`members` 2 人 → 需邀请流程（QA 起第二个账号）。

---

## 4. 工作包 C：228 条控件/动作运行时验收（BLOCKED，保持）

- 主报告 §3.1 仍为 **228 行**（`grep -cE '^\| I-'` = 228），本轮**未改动任何一行状态**，未新增运行时断言。
- 未执行原因：任务单明确禁止以静态映射/截图旁证替代真实操作；本轮范围为证据纠偏，不扩张到逐控件实测。
- 逐条待补字段（任务单 §4.3）：控件/动作 ID、环境与代码 SHA、前置数据、操作步骤、实际结果、成功/失败、数据联动、pending/防重复、权限/错误态、证据路径。
- 若后续有部分真实执行，必须**分开统计已执行/未执行**，不得把部分执行写成 228 条全通过。
- **责任人**：QA（执行）+ Owner/CI（环境与放行）。

---

## 5. 工作包 D：逐路由测试证据（PARTIAL）

### 5.1 CI 官方元数据（已更正）

| 项 | 值 |
|---|---|
| run / URL | `41954006789` / https://github.com/LinTia/FastLadger/actions/runs/41954006789 |
| job 1 | `127004401500` **build / vet / test / race** → success（步骤 8 SQLite 冒烟、9 `go test ./...`、10 `-race` 均 success） |
| job 2 | `127005322262` **docker build / run / healthz** → success |
| `-race` 耗时（官方原文） | `ws 13.204s` / `service 93.326s` / `api 34.720s` |
| 环境 | `go1.22.12 linux/amd64`、`gcc 13.3.0`、`CGO_ENABLED=1`、runner `ubuntu-24.04` |
| 计数 | `ok` 10 包、`[no test files]` 2 包、`--- FAIL` 0、`DATA RACE` 0 |

**更正说明**：第四次文档写的 job id `126996266896`/`126996901301` 与耗时 `10.4s/66.4s/24.0s` 属**另一次 run**（`41951501832`），本轮已按官方 API/日志改正，详见 `docs/r0-evidence-5th/ci-metadata-correction.md`。

### 5.2 四类 SHA 分开记录

| 名称 | 值 |
|---|---|
| CI 实际测试 commit | `9d083f62bf80e0ba7cdb444e57940eb432c1d5f3`（`9d083f6`） |
| 业务代码基准 | `bff6a2b351bf5a039cdf7c762ec2693fb330cd00` |
| 证据目录/文档提交 | 初次引入为 `9d083f6`，后由 `11bdfcc` 替换为 run `41954006789` 的日志；**该 run 不含最终版证据目录** |
| 本轮 main HEAD | `72cbab71253d031a09acc405cad25155506db531` |

### 5.3 全仓 race 证据边界

| 证据 | 主体 | 覆盖 | 可追溯 |
|---|---|---|---|
| CI `-race` | GitHub Actions | 仅 `ws` / `service` / `api` 三包 | run/job id + 日志 + head SHA |
| 服务器全仓 race | **WorkBuddy（Owner 授权），非 CI** | `go test -race ./... -p 1` 全仓，`RACE_EXIT=0` | `docs/r0-evidence-4th/env.txt`（commit/Go/OS/CGO）、`race.log`、`race.exit`、`SHA256SUMS.txt` |

两者不可互相替代或混称。

### 5.4 未覆盖路由逐条处置（13 条 → 覆盖 3 / 部分 5 / 未覆盖 5）

处置方式：**方案 B（本轮不补测试）**，逐条给出证据、风险与最小补测建议（详见 `docs/r0-evidence-5th/api-route-coverage.md`）。

| 路由 | 判定 | 现有证据 / 缺口 | 最小补测建议 |
|---|---|---|---|
| `GET /healthz` | **覆盖** | `api/qa_stage3_test.go:30-46` 与 `:52+` 两个命名用例，断言 200 + instanceId/restoreEpoch + 未注入时省略 | — |
| `GET /users/me` | **覆盖**（DTO 断言弱） | `session_api_test.go:286`（成功）、`:297-300`（旧令牌 401）；字段仅 `token_handler_test.go:122` 间接 | 补 `isInstanceAdmin` 字段断言 |
| `PUT /users/me` | **部分** | 仅 service `profile_test.go:13` | HTTP：200 + 409 邮箱占用 + 400 |
| `POST /users/me/password` | **部分** | 仅 service `profile_test.go:57`；`{revokedSessions,message}` 无断言 | HTTP：400 弱口令 + `revokedSessions` 数值 |
| `GET /books` | **未覆盖** | 无 GET 断言；仅 POST 建账本作夹具 | 空数组非 null、非成员不可见 |
| `GET /books/:id` | **未覆盖** | 无用例命中 | 200 / 403 非成员 / 404 |
| `GET /books/:id/members` | **未覆盖** | 仅 join / revoke 用例（`invite_lifecycle_api_test.go`） | 列表字段 + 403 |
| `GET /books/:id/accounts/:aid` | **部分** | `qa_api_test.go:119-139` 仅 pathID 400/解析 | handler 200 / 404 |
| `DELETE /books/:id/accounts/:aid` | **部分** | service `remediation_integration_test.go:594/627`、`remediation_unit_test.go:501` | HTTP 成功 `{id}` / 403 / 409 |
| `GET /books/:id/categories` | **未覆盖** | 仅 `reports/categories` 用例 | 列表 + `?tree=1` 的 `children` |
| `DELETE /books/:id/categories/:cid` | **部分** | service `remediation_integration_test.go:657` | HTTP 成功 / 403 / 409 |
| `GET /books/:id/transactions/:txId` | **未覆盖** | 仅路由注册清单（`router_test.go:25`、`qa_api_test.go:275`） | 200 可见 / 404 不可见（不泄露存在性） |
| `POST /books/:id/instruments/:instrumentId/price` | **覆盖** | `invest_handler_test.go:167` 全流程 | — |

**与主报告旧口径的差异（已更正）**：`GET /healthz`、`GET /users/me` 原写"未覆盖"，实为有命名用例；`GET /accounts/:aid` 原写"未覆盖"，实为部分覆盖（仅 pathID 解析）。

**责任人**：后端 + QA（R0 之后的测试补强批次）；本轮不补测的理由见 `api-route-coverage.md` §3。

---

## 6. RV-05：Owner 安全动作（WorkBuddy 不执行）

**状态：Owner ACTION REQUIRED —— 未闭环。**

事实（第三方复核 + 本轮回读，未复制任何凭证值）：

| 项 | 状态 |
|---|---|
| 当前树凭证 | 抽样扫描未发现新的明文凭证；**不等于**全仓全历史扫描 |
| 旧历史提交 `259bd11` | 仍可匿名读取；复核在该旧文件中统计到 **29 个 JWT 格式值** |
| 这些旧凭证 | 此前已验证失效/撤销；不能据此推断生产被入侵或旧 token 仍有效 |
| 仓库可见性 | API 复核 `private=false`（仍 Public），与 [1.69] 记录的 Owner「改回 Private」决策不一致 |
| 重要限定 | **改为 Private 只是限制可见性，不等于历史对象被物理清除** |

Owner 待办（WorkBuddy 不代做）：

1. 将仓库切换为 **Private**；
2. 切换后做**匿名旧 SHA 访问复验**（未登录访问 raw/API 应不可读）；
3. 评估是否需要进一步历史清理（联系 GitHub Support 等），以及清理后的复验方式。

**严禁事项（本轮均未做）**：未更改仓库可见性；未 force-push；未重写历史；未删除 refs；未联系 GitHub Support；未在文档/截图/日志中复制或暴露任何旧 token/JWT 值；未提供含敏感值的旧历史文件直链。

---

## 7. 文档同步清单与交叉引用检查

| 文件 | 本轮改动 | 交叉引用 |
|---|---|---|
| `docs/FastLedger-R0-基准冻结与差异盘点.md` | §5.3.1 三条路由行更正；§5.3 注 3 改为"覆盖 3 / 部分 5 / 未覆盖 5"；新增 §7.7（104 张 + 校验 + 缺口）；§8 四包状态与 RV-05；新增 §10.5 | 指向第五次回执与 `r0-evidence-5th/` |
| `docs/FastLedger-R0-第四次证据补齐-执行回执.md` | 开头新增「第三方独立复核纠偏」小节；B/D 状态改判 PARTIAL；原文保留 | 指向第五次回执 |
| `README.md` | 状态快照改为「截至 [1.78]」；第四次回执行标注已被纠偏；pairs 行标 PARTIAL；新增第五次回执行与 `r0-evidence-5th/` 导航 | 与主报告 §8 一致 |
| `CHANGELOG.md` | 新增 [1.78]；[1.76] 标题下加"已被 [1.78] 改判"提示（不删原文） | — |
| `workbuddy/第三方审阅者导读.md` | 新增「R0 当前状态（第五次整改后）」小节，含任务单 → 回执 → 证据 → 主报告入口 | 与 README 一致 |
| `docs/r0-screenshots/pairs/README.md` | 采集口径补角色/状态/轮次；新增 §2.1 分类映射修复；settings/mine 计页说明；新增 §8 状态结论（PARTIAL） | 指向 `r0-evidence-5th/` |
| `docs/r0-evidence-4th/ci-run/README.md` | job id / 耗时更正；补"四类 SHA 不得合并" | 指向 `ci-metadata-correction.md` |
| `tools/r0/seed-from-dataset.js` | 解析优先级修复 + `categoryAudit` 输出 | — |
| `tools/r0/capture-pairs.js` | 角色改由 API 读取（`role`/`roleRaw`/`roleSource`/`bookMembers`） | — |

一致性自检（人工 + grep）：四包状态在 6 处文档中均为 **A BLOCKED / B PARTIAL / C BLOCKED / D PARTIAL**；CI run 号在 README/主报告/回执中一致为 `41954006789`；路由计数在 §5.3 注 3、§10.5、`api-route-coverage.md`、本回执中一致为 **3/5/5**。

---

## 8. 范围与"未变更"证据

| 项 | 验证方式 | 结果 |
|---|---|---|
| 业务代码 | `git diff --stat bff6a2b 72cbab7 -- server/ client/` | 0 行（空输出） |
| 本轮工作区 | `git status --short` | 仅 `CHANGELOG.md`、`README.md`、`docs/**`、`tools/r0/*.js`；**无 `server/`、`client/`、`server/web/dist/`** |
| PRD / WBS / 落地方案 / 枚举字典 | 未打开写入 | 未改 |
| API / 数据库 / 迁移 | 未改 | 未改 |
| R1–R7 / PWA | 未启动 | 未实施 |

---

## 9. 结论

- **R0 仍 BLOCK**：A/C 未通过；B/D 仅 PARTIAL。
- **无任何 R1 授权**，PWA 未启动，未改业务代码。
- 本轮价值集中在三点：**分类映射缺陷修复并重采**（使两侧数据真正可比）、**CI 元数据与四类 SHA 更正**（消除"run head 即最终提交"的误述）、**逐路由覆盖复核**（把"全绿"拆成 3/5/5 的可核查清单）。
- 等待第三方独立复核；如对上述任一结论有异议，请指出具体文件与行号，我按证据修正，不硬撑。
