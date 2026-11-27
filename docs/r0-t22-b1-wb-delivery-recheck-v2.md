# T22 批次 1 第 2 版（`3999cee`）交付级独立复核

> 复核角色：第三 AI（不参与 WB 交付撰写），只读复核 + 独立报告。
> 被审分支：`work/r0-t22-batch1`
> 被审 SHA：`3999ceef76bd5e4f0e2f416d3287284613c97179`（2026-11-27）
> 上一被审 SHA：`2e647d4d2ad664fa79a60035e7df9d7d045cbf01`
> 计划基线：`0033a1899878c2b832578cbfbb11ab858c2f8649`（`docs/r0-t22-execution-plan.md`，本轮未改）
> 上一轮复核报告：`f4c11c4939df81f6af961163b951c1882bed6392`（`docs/r0-t22-b1-wb-delivery-recheck.md`，分支 `review/t22-w1-w8-wb-recheck`）
> 本报告分支：`review/t22-w1-w8-recheck-v2`
> 复核日期：2026-11-27

---

## 0. 摘要结论

**整体判定：PARTIAL，暂不签收 W1+W8 的合并交付。**

- **W1（矩阵结构、双分母、齐备表、证据台账）已达可签收水平**：104 / 17 / 51 / 36 / 96 / 56 全部可复算，N/A 台账 36/36 闭合，E 台账 17 格 56 张路径逐条可解析。这一版相对 `2e647d4` 是实质性修补，不是文字搬运。
- **W8（offline / conflict 边界）仍不达交付级**：页 5 的版本冲突载体链（`app.js:655 → tx_update.go:130–135 → common.go:64 → app.js:670`）经逐行核对成立，但「逐页 409 语义表」存在 4 类 P2 缺陷：页 13 的邮箱 409 挂到 Token 增删（实际来自 `PUT /users/me`，`app.js:2206`，未列入写调用）；页 8 的「标的下仍有持仓」409 属 `DELETE /instruments/:id`（`app.js:1412`），而写调用列只列两个 PUT；12 页中至少 6 页漏列同页其他可达 409 分支；页 5「编辑已删交易」实际返回 404（`tx.go:1054–1055`），与 `tx.go:54–56`、`PRD:593` 的「已删 = 409 ENTITY_DELETED」口径相反，而 §5.5 实现缺口未登记。
- **证据描述有 1 处硬性事实错误（P2）**：§6 与 §1.1 把 `runtime/02`、`04` 的骨架条数写成 3，实测 `dom-snapshots.json` 四张 pending/rejected 均为 `skelCount=4`，`runtime/README:5/:76–77` 亦写 `.skel`=4，`04` 图可见 4 条。
- **证据基准有 1 处不可复现（P2）**：页面级 43 张的「代码基准」与 E4/E12 的「早于 `0472484`」均以「mtime 2026-11-04」为依据；git 不保存 mtime，本工作树实测 mtime 为 2026-11-26（checkout 时间），任何他人 clone 都会得到不同值。结论本身成立（43 张由 `575eae3` 于 2026-11-04 入库），但依据必须换成提交时间。
- **V-10 复跑是真 PASS，不是自报**：我从上一轮报告 §6 抽出脚本，在 `3999cee` 上独立执行，输出与 WB §7.9 完全一致（5 OK / 0 FAIL）。但 V-10 只覆盖结构闭合与 5 个字面口径，**覆盖不到本轮发现的语义错误**，不能作为签收依据。
- **R0 仍 BLOCK**：本批只是 W1/W8 文档，不触碰第 3 条门禁，不得据此宣布 T22 完成、第 3 条 PASS 或 R0 PASS。

---

## 1. 提交范围与边界合规

```
$ git diff --name-status 2e647d4 3999cee
M  docs/r0-screenshots/pairs/README.md
M  docs/r0-status-matrix-13x8.md

$ git diff --name-only 2e647d4 3999cee -- server/ tools/ prototype/ .github/ docs/r0-screenshots/pairs/*.json docs/r0-screenshots/pairs/*.txt
（空）

$ git log --format='%h %ad %s' --date=short 0033a18..3999cee
3999cee 2026-11-27 docs(r0): T22 批次 1 第 2 版——依交付级复核修正 W1/W8（有 17 / 缺 51 / N/A 36，齐备 0）
2e647d4 2026-11-27 docs(r0): T22 批次 1——13x8 矩阵骨架冻结（W1）+ offline/conflict 边界取证（W8）

$ git branch -a --contains 3999cee   # 仅被审分支（本地 detached 检出）
$ git rev-parse main
b4309dbd481423321f6740dfad5099ad037b0e10
```

判定：**PASS**。两个提交只改两个 Markdown；`server/`、`tools/`、`prototype/`、CI、截图 PNG、`SHA256SUMS*.txt`、fixture、controls.json 零改动；`main` 不含该提交；无合并、无 PR。

与本批次边界逐条对照：

| 边界 | 实测 | 判定 |
|---|---|---|
| 只文档 | 仅 2 个 .md | PASS |
| 无业务代码 / CSS / 截图 / 服务 / 数据写入 | 上述路径全空，本轮我亦未起服务、未采图、未写库 | PASS |
| 无 W2–W7 / W9 | 文档仅声明 W2 属后续，未产出 | PASS |
| 无 P1–P6 | 仅 §9 第 10 条登记 books/transactions/recycle 待 P2 裁决 | PASS |
| 无 main / 合并 | main 仍 `b4309db` | PASS |

---

## 2. 计数、分母与脚本一致性（逐脚本独立复跑）

我不采用 WB 贴出的输出，而是把 §7 的 8 段脚本原样重跑，并与正文数字比对。

| 脚本 | WB 正文声明 | 我实测输出 | 一致性 |
|---|---|---|---|
| §7.1 三计数 | 行 13 / 有 17 / 缺 51 / N/A 36 / 合计 104 / 未分类 0 | `行=13 有=17 缺=51 N/A=36 合计=104 未分类=0` | 一致 |
| §7.2 N/A 台账与码分布 | 36 行；R01×8、R07×12、R08 十一项各 1、R02–R06 各 1 | 36 行；分布完全相同 | 一致 |
| §7.3 E 行数与张数 | 17 行 / 56 张 | 17 / 56 | 一致 |
| §7.4 §1 数据行列数 | 12（10 列） | 12（唯一值） | 一致 |
| §7.5 路径断言 | E1–E17 逐条无 MISSING | 17 行全部命中；我另用独立 Python 展开花括号逐文件核对，56/56 存在，0 缺失 | 一致 |
| §7.6 与计划逐格比对 | 改判 5 / 一致 99 | 5 / 99；改判格 = 页2 normal、页2 loading、页5 conflict、页9 budget、页10 report | 一致 |
| §7.7 齐备统计 | 17 / 0 / 0 / 17 / 1 | `有格=17 三档齐备=0 缺390=0 缺641=17 缺1280=1` | 一致 |
| §7.8 双分母 | 96 / 17 / 51 / 28 | `适用格=96 其中有=17 缺=51 N/A=28` | 一致 |

另做两项 WB 脚本未覆盖的交叉校验：

1. **§2 视口表 vs §4 台账交叉**：17 行中 390/641/1280 的 ✓/✗ 与 E1–E17 台账「视口」列逐格比对，**0 处不一致**。
2. **N/A 台账复合码闭合**：矩阵 36 个 N/A 格的首码与 §3.2 台账（页面 + 状态）的码逐格匹配，**0 处缺失、0 处多余、0 处码不符**。

定义一致性：

- 清点分母 104 = 13×8（含页 3 的 8 格）；适用分母 96 = 104 − 8；`17 / 104` 称「清点进度」、`17 / 96` 称覆盖率（`:25`），与 §7.8 输出一致。
- 「有」= 存在呈现该状态的图（§0.1 `:21`）；「齐备」= 390/641/1280 三档齐（§2）；「可用性」= 能否代表当前实现（§0.2）。三轴分离与我上一轮 F-03 的要求一致。
- 56 张构成 36（页面级）+ 16（pairs）+ 4（runtime）= 56，与逐行张数求和一致。

**判定：PASS。** 唯一残留是 §1 `:86` 的「合计」行把 17/51/36 塞进 `normal/loading/empty` 三列（10 列表结构合法，脚本因 `^\| [0-9]+ \|` 不计入，但人工或第三方解析器会误读为「按状态计数」）。见 N-18（P3）。

---

## 3. 逐项判定（F-01…F-14 / N-01…N-10）

### 3.1 上一轮 F 项

| 项 | 判定 | 直接证据 | 说明 |
|---|---|---|---|
| F-01 首页/预算/报表 normal 漏计 | **PASS** | 矩阵 `:74` 首页 normal `有 E14`；`:81` 预算 `有 E16`；`:82` 报表 `有 E17`；§1.1 `:96/:98/:99` 给已打开 390 档的具体数值；§4 `:226/:228/:229` 路径 | 已闭合。E14 图实测收入 ¥28,000.00 / 支出 ¥1,089.30 / 结余 ¥30,126.70，与正文一致 |
| F-02 首页 loading 漏计 | **PASS** | `:74` loading `有 E15`；`:227` E15 引 `runtime/01`、`04`；§6 `:462–467` | 已闭合。`01` 三卡 + 4 骨架、`04` 三卡 + 骨架，均为「加载中」而非空白 |
| F-03 「有」兼表存在性与齐备 | **PASS** | §0.2 `:27–35`；§2 `:106–129`；§7.7 | 已闭合（选拆列方案）。17 个「有」格三档齐备 0，缺 641 17/17 |
| F-04 「有」判据不一致 | **PASS** | §0.3 `:37–45` 五条规则；§2.1 `:131–141`；§4 缺口列 `:209` 表头 | 判据层闭合：优先级、数据前置、runtime 隔离要求、混合态不判「有」均已写死；投资 pairs 按规则 5 未判「有」并列入待冻结 |
| F-05 pairs 基准漂移未标 | **PARTIAL** | §4「代码基准」列 `:213–229`；§4.2 `:250–256`；`pairs/README.md:23–28` | 事实方向对、告警到位，但 **43 张的基准依据写为 mtime**，不可复现（见 N-16）；E3/E15 的 `557e4d2` 是文档 HEAD 而非代码 SHA（N-17） |
| F-06 conflict 整列 N/A 不成立 | **PARTIAL** | `:77` 页 5 `缺（有版本冲突载体）`；§5.5 `:417–426` | 页 5 载体链成立；但逐页 409 表有 2 处挂接错误 + 6 页漏列分支，见 §4 与 N-12…N-14 |
| F-07 R07 未登记产品要求与缺口 | **PASS** | §3.1 `:157` R07 三要素；§3.2 36 行；§5.6 `:436–446` | R07 12 格带 `R0-E-15`，R08 拆 11 个分页码；页 3 的 7 行仍写「同上」，属 P3 残留（N-23） |
| F-08 24 格 vs 26 格 | **PASS** | §5.7 `:448–456` | 26 = 24（12 实采页×2）+ 2（页 3）；N/A 25、缺 1，可点数 |
| F-09 104/96 双分母与齐备统计缺失 | **PASS** | §0.1 `:18–19`；§7.7；§7.8 `:646` | 17.7% / 0% 只按 96 计，未出现 17/104 冒充覆盖率 |
| F-10 页 13 合并不清 | **PASS** | §4.1 `:238–244`；§2 `:122` | 判据冻结为「有按任一视图、齐备按两视图均覆盖」，641 需 mine + settings 各 1 张；未拆行并说明需 Owner 同意 |
| F-11 导出 empty 无判据 | **PASS** | §2.1 `:137`；§3.2 R06 行 | 保持 `缺`，但写明断言文本在 `app.js:1066`（实测该行确为「没有可导出的数据…」分支） |
| F-12 手动记账 empty 无前置提示 | **PASS** | §3.1 R04 `:155`；§3.2 第 15 行；§2.1 `:138` | `app.js:568`「（暂无可选分类）」实测存在；口径列入待冻结 |
| F-13 grep 方法性漏检 | **PASS** | §5.2 G1–G11 `:272–377` | 五段式齐全；G7 明确「不是冲突判据」；G10 给出 `baseVersion` 正面证据；`sync` 61 行/57 async/4 真命中经我复跑一致（`:556/:572/:578/:655`） |
| F-14 「有」格缺基准/数据前置/动作 | **PASS** | §4 10 列 `:207–232`；E1–E17 | 56 张、路径可解析、子页归属已拆；E6/E13 子页张数与齐备判定分开 |

### 3.2 上一轮 N 项

| 项 | 判定 | 证据 |
|---|---|---|
| N-01 旧 E3 是泛化拦截产物 | **PASS** | `:474` 弃用 `home/390x844/error.png`（仍留目录不删），E3 改引 `runtime/02`、`03`；该图未进入任何「有」格 |
| N-02 runtime 5 张零引用 | **PASS** | §6 `:460–474` 五张全表；`05` 未采信且说明未打开原因 |
| N-03 R08「无冲突载体」与代码相反 | **PASS** | §5.1 第 4 点 `:270` 明令「不得再写」；§5.4 `:400–408`；§5.5；结论已按我上一轮收窄范围落到「只有页 5」 |
| N-04 零改判当优点 | **PASS** | §1.1 `:92–102` 显式 5 格改判；§7.6 脚本 |
| N-05 台账「同上」当理由 | **PASS（有残留）** | R07 三要素 `:157`；R08 11 个分页码 `:158`、`:417–426`；页 3 的 7 行仍写「同上」，见 N-23 |
| N-06 成员 empty 判缺不成立 | **未采纳但可接受** | §10 `:723`；§2.1 `:141`；理由为「改判会动 17/51/36 底稿，且报告本身列待冻结」；已登记待 Owner 冻结，不计为缺陷 |
| N-07 E6/E13 混装子页 | **PASS** | §4.1 `:234–244`；§4「子页归属」列 |
| N-08 `wsAuth` 锚点 95 vs 112 | **PASS** | §5.3 `:385/:386` 拆两行；`middleware.go:95` 注释、`:112` 函数，实测一致 |
| N-09 `:5` 冻结与可降级矛盾 | **PASS** | 文首 `:6` 与 §2 第 3 行不再出现「E3 可降为缺」的可变表述 |
| N-10 简写无路径映射 | **PASS** | §0.4 `:47–61` 11 条映射，路径逐一存在（`docs/` 下均可解析） |

**小结**：14 项 F 中 12 PASS、2 PARTIAL（F-05、F-06）；10 项 N 中 9 PASS（N-05 有 P3 残留）、1 未采纳但已登记。相比上一版「3 项未修 + 2 项部分修」，本轮修正确实落地。

---

## 4. W8 专项：页 5 证据链与其余 11 页 409 语义

### 4.1 页 5 版本冲突载体链（逐行核对）

| 环节 | 文件:行 | 实测原文要点 | 判定 |
|---|---|---|---|
| 前端发送版本 | `server/web/dist/app.js:655` | `baseVersion: tx.syncVersion` 或 `0`（update body；原文为逻辑或运算符，表内改写以免破列） | 成立 |
| 前端写调用 | `app.js:664` | `await api("PUT", "/books/" + bookId + "/transactions/" + tx.id, body)` | 成立 |
| 后端比对 | `server/internal/service/tx_update.go:130–135` | `if r.BaseVersion > 0 {` 逐行 `gr.SyncVersion != r.BaseVersion → ErrConcurrentConflict` | 成立 |
| 409 映射 | `server/internal/api/common.go:64` | `case errors.Is(err, service.ErrConcurrentConflict): fail(c, 409, "数据已被并发修改，请重试")` | 成立 |
| 前端提示 | `app.js:670` | `toast(err.message + (…indexOf("并发") >= 0 ? "（请刷新列表后重试）" : ""), "err")` | 成立 |
| 无 errorCode | `common.go:22–24` | `fail()` 只回 `{code: status, msg: msg}` | 成立 |

判定：**PASS**。「页 5 是唯一版本冲突载体」与我上一轮独立结论一致；`app.js` 仅 update body 携带 `baseVersion`，create body（`app.js:586–595`）不带该字段。

### 4.2 其余 11 页逐页 409 语义

| 页 | 表内 409 来源 | 实测可达 409 | 判定 |
|---|---|---|---|
| 1 登录 | 401 为主 | `POST /auth/login` 无 409 | PASS |
| 2 首页 | 无写 | 只读 `GET` `:1912` | PASS |
| 3 截图识别 | 无路由 | 整页 N/A R01 | PASS |
| 4 手动记账 | `common.go:49` | 还可达 `:62`（`ErrIncompleteReceipt`，`tx_update` 同类回执路径）与 `:51`（`ErrEntityDeleted`，`tx.go:469/:492` 引用已删账户/分类） | **PARTIAL**（漏列 2 类，判定 N/A 不受影响） |
| 6 账户 | `account.go:238–239 → common.go:64` | 还可达 `:100` 账户仍有交易、`:102` 账户仍有子账户（页面有删除按钮，实测图 `categories`/`accounts` 均有「删除」） | **PARTIAL** |
| 7 两级分类 | `category.go:233–234 → common.go:64` | 还可达 `:105` 分类仍有交易、`:107` 分类仍有子分类 | **PARTIAL** |
| 8 投资资产 | `common.go:80` | `:80` 对应的调用是 `DELETE /instruments/:id`（`app.js:1412`），表内写调用只列 `PUT :1480`、`PUT :1523` | **FAIL（挂接错）** |
| 9 预算 | `common.go:85` | 唯一性 409 成立；`budget.go:22` 注释「budgets 表没有 sync_version 列」实测存在 | PASS |
| 10 报表 | 无写 | `GET :862`、`:902` | PASS |
| 11 导出 | 无写 | `GET :1064` | PASS |
| 12 成员 | `common.go:115` 成员上限 | 还可达 `:51`（`member.go:234` 账本已删） | **PARTIAL** |
| 13 我的设置 | `common.go:90` 邮箱占用 | 表内写调用是 `POST/DELETE /users/me/tokens`（`:1574`、`:1636`），**Token 增删不产生邮箱 409**；`ErrEmailTaken` 的真实入口是 `PUT /users/me`（`app.js:2206`，属 13a「我的」），未列入 | **FAIL（挂接错）** |

同一事实还写进了 N/A 台账 §3.2 第 36 行（`:201`）：「Token 增删不传版本；`common.go:90` 的 409 是邮箱…」——这句把不存在的因果写成了依据，属于**事实错误**，不是表述瑕疵。

结论：页 5 = PASS；其余 11 页 = **PARTIAL**。N/A 判定（这些 409 都不是版本冲突）全部成立，但「逐页 409 语义」这一自述不成立，交付前必须补正。

### 4.3 未被登记的实现偏差（新发现，P2）

`tx_update.go` 对目标行调用 `loadTxRowByState(..., true)`（`tx.go:1044–1057`）；该函数在行不存在/已删时统一返回 `ErrTransactionNotFound` → **404**。而 `tx.go:54–56` 的注释明确写「已删是【冲突】，必须与『目标不存在』（404）区分开」，`PRD:593` 也要求「目标已删则返回 409 + `ENTITY_DELETED`」。当前 Web 编辑已删交易得到 404，Web 端又无 409 分支，实际表现是「交易不存在」——与自家设计注释和 PRD 相反。

WB 的 §5.5 实现缺口只列 3 条（无 errorCode、无 409 分支、无三选一），**未登记这条 404/409 偏差**。见 N-15。

---

## 5. 台账、基准、数据前置、待冻结项

### 5.1 E 台账（17 格 / 56 张）

- 17 行与 17 个「有」格一一对应，无多无缺；张数求和 56，与三来源分解（36 + 16 + 4）一致。
- 我用独立脚本展开花括号路径逐文件核对：**56/56 存在，0 缺失**；`find docs/r0-screenshots -name '*641*'` = 0，支持「641 零覆盖」。
- 页面级 43 张 + pairs 104 张 + runtime 5 张 = 152，与 §4 的 36/16/4 及 §9 第 10 条（books/recycle/transactions 各 2 张不入矩阵）自洽。

判定：**PASS**。

### 5.2 视口齐备表

17 行 × 3 视口 + 齐备列，与台账逐格比对 0 不一致；统计 0 / 17 / 17 / 1 与 §7.7 输出一致。判定 **PASS**（措辞残留见 N-19）。

### 5.3 基准 SHA

| 位置 | 写法 | 问题 |
|---|---|---|
| E1/E2/E4/E6–E13（`:213–225`） | 「未记录（无 manifest；mtime 2026-11-04）」 | mtime 不可复现：实测三张 PNG mtime 均为 **2026-11-26 09:54**（checkout 时间）；git 不保存 mtime |
| §4.2 `:256`、§9 `:691` | 「只能按 mtime 2026-11-04 推定早于 `0472484`」 | 同上；正确依据是入库提交 `575eae3`（2026-11-04），早于 `0472484`（2026-11-05 18:18） |
| E3/E15（`:215/:227`） | 「557e4d2（runtime/README:14 记录的历史文档 HEAD）」 | 该字段名为「代码基准」，但 `runtime/README:11` 写明这是**历史文档 HEAD**，README §7 亦声明对应代码 SHA 未独立验证；列名与事实不符 |
| E5/E14/E16/E17 | `bff6a2b` | 与 `pairs/README:13` 一致（2026-11-04 13:45）；采集 `2026-11-05T05:38Z` = 13:38 CST，早于修复 18:18，**时间线成立** |

时间线实测：

```
$ git log -1 --format='%ad %s' --date=iso bff6a2b
2026-11-04 13:45:36 +0800 docs(r0): 完成 R0 文档缺口…
$ git log -1 --format='%ad %s' --date=iso 0472484
2026-11-05 18:18:00 +0800 fix(r0): 有限范围修复第六次验收 4 项业务缺陷…
$ manifest-impl.json capturedAt 首张 = 2026-11-05T05:38:38.893Z
$ git log -1 --format='%ad %h %s' --date=short -- docs/r0-screenshots/login/390x844/normal.png
2026-11-04 575eae3 docs(r0): R0 二次整改——证据补齐与状态声明纠偏 [1.63]
```

判定：**PARTIAL**。结论正确、依据有 1 处不可复现、1 处列名与事实不符。

### 5.4 数据前置

- E14/E16/E17 的「22 笔 / 5 账户 / 37 分类 / 0 投资 / 5 预算」与 `pairs/seed-manifest.json → dataset.counts`（accounts 5、categories 37、transactions 22、investments 0、budgets 5）一致；`invest` 混合态（总资产 ¥26,258.70 / 负债 ¥3,868.00 / 持仓 ¥0.00）与 §2.1 待冻结描述一致。
- E6「默认 3 账户（现金/活期储蓄/信用卡，¥0.00）」经我打开 `accounts/390x844/normal.png` 核实：**3 行，均 ¥0.00**，成立。
- E2「0 交易」经 `home/390x844/empty.png` 核实成立；E9「0 预算」经 `budget/390x844/empty.png` 核实成立。
- **矛盾点**：§0.3 规则 3（`:43`）写 pairs 数据前置含「**2 名家庭成员**」，依据是 `seed-manifest.json → dataset.counts.family=2`；但该 2 名是**原型侧**数据。`pairs/README:110` 明写「原型 2 名成员；实现侧 1 名，需邀请流程」，实现侧 members 图为 1 名成员。见 N-25。

判定：**PARTIAL**（1 处数据前置口径混用）。

### 5.5 待冻结项

§2.1 `:131–141` 五格（投资 normal 缺、首页 permission 缺、导出 empty 缺、手动 empty `N/A R04`、成员 empty 缺）与 §1 现值逐格一致；五种候选判据都写了判定后果（18/50、移动 1 格等）；与 N-06 不采纳、页 13 不拆行的说明互相闭合。

判定：**PASS**。唯一小瑕：投资行写「18/50 或 17/51」省略了第三数（N/A 恒 36），属表述不完整（P3，并入 N-18 的同类问题，不单列）。

---

## 6. pairs/README 更正后的哈希清单处理

WB 的做法：只改事实声明，不改 104 张 PNG，不改 `SHA256SUMS*.txt`，并说明「清单第 108 行是更正前的 README 哈希，现已失配；剔除 README 一行后其余 108 条仍通过」。我逐项实测：

```
$ wc -l docs/r0-screenshots/pairs/SHA256SUMS.txt
109
$ sed -n '108p' SHA256SUMS.txt
c329854fa3dbe55267ee816528d86329481721e7c18f51a2a925f3e38e93a28b *README.md
$ sha256sum docs/r0-screenshots/pairs/README.md
eda61aee70a6d8a51bf2bce558e9b00a7e6869f2b862492af95079f4b051539c  README.md   ← 与 §9 第 11 条一致
$ sed -n '108p' SHA256SUMS-4th-round.txt
431238903be4737070aa4d3de78ef1b1147e1a93b5f3f335331163e6ad8b75b8 *README.md   ← 与 README:31 一致
$ grep -v 'README.md' SHA256SUMS.txt > /tmp/sums_no_readme.txt; wc -l /tmp/sums_no_readme.txt
108
$ sha256sum -c /tmp/sums_no_readme.txt --quiet; echo rc=$?
rc=0
```

判定：**PASS（处理方式合理）**。

- 未重签归档是对的：重签会覆盖「采集时点」的可审计性，而 README 是声明文件，不是证据像素。
- 「剔除自身后 108 条 rc=0」我实测成立，不是自报。
- 自指哈希无法内嵌，把实测值登记在矩阵 §9 第 11 条，可追溯。
- 残留瑕疵 2 处（P3）：① `README:29`「证据张数与哈希未变」易被读成「本文件哈希未变」，建议改为「104 张 PNG 张数与哈希未变；本文件自身哈希已随声明更新」；② `README:182`「`SHA256SUMS.txt`（108 行）」与实际 109 行不符（108 条 + README 自身）——该行非本次新增，但本次既在改同一段，应顺手校正。

另：`0472484` 对 `server/` 的改动范围经我复算成立——`git diff --stat bff6a2b..3999cee -- server/` 仅 `app.js 52 行（+43/−9）`；但**该提交同时改了 `renderTxns`（`:383`、`:397–423`）与 `renderMembers`（`:1706–1741`）**，README「其余页面代码路径未被触及」在 13 页范围内成立（交易总表不在 13 页，且已按 §9 第 10 条排除），但读者容易把 `renderTxns` 误读为「交易详情/首页」。建议在 README 更正段注明「`renderTxns` 为交易总表（非 13 页矩阵对象）」。

---

## 7. V-10 复跑：真 PASS，但不是签收依据

```
$ git show f4c11c:docs/r0-t22-b1-wb-delivery-recheck.md | awk '…' > /tmp/v10_check.py
$ python3 /tmp/v10_check.py docs/r0-status-matrix-13x8.md
通过项：
  OK  §1 行数=13
  OK  总格=104（有 17 / 缺 51 / N/A 36）
  OK  N/A 台账闭合 36/36
  OK  E 台账一一对应 17 格
  OK  E 台账路径全部可解析
未通过项：
结论：PASS（未通过 0 项）
```

与 WB §7.9 的输出逐字一致，**WB 没有虚报**。脚本确实覆盖了我上一轮 V-10 的 6 个 FAIL 项对应的 5 个口径：视口齐备表述、offline 格挂 `R0-E-15`、基准 + SHA 字段、数据前置字段、96 分母；另外覆盖 13 行 / 104 格 / 36 格台账 / 17 格 E 台账 / 路径。

但必须说明其**覆盖边界**（这也回答「是否覆盖我上一轮指出的关键问题」）：

1. V-10 对 E 路径只要求「每行至少 1 个路径存在」（`any(isfile)`），不校验张数；本轮我另做全量核对才确认 56/56。
2. V-10 只做「字面存在」检查：文档里出现「基准」「SHA」「数据前置」「96」即通过，不校验数值正确性、基准时间线、409 语义、骨架条数、合计行语义。
3. V-10 不检查 §2 与 §4 的交叉一致、不检查 §7 输出与正文一致——这两项由我在 §2 用独立脚本补做。
4. 因此 V-10 PASS 只证明「结构闭合」，**不能证明「内容正确」**；本轮 3 个 P2 内容问题（骨架条数、409 挂接、mtime 依据）它一个都抓不到。

---

## 8. 新发现问题（N-11…N-25）

严重级别定义：**P1** = 阻断签收且需返工/重采；**P2** = 事实错误或证据链缺陷，必须改文档后才能签收；**P3** = 表述、锚点、格式瑕疵，建议同批修正。

### P2（6 项，签收前必须修）

**N-11（P2）runtime 骨架条数写错**
- 位置：矩阵 `:97`（§1.1 E15 依据）、`:465`（02 行）、`:467`（04 行）。
- 现状：02「3 条骨架残留」、04「3 条骨架」、§1.1「4 条（390）/ 3 条（1280）」。
- 实测：`runtime/dom-snapshots.json` 中 `01/02/03/04` 四张 `skelCount` 均为 **4**，`05` 为 0；`runtime/README:5` 与 `:76–77` 写 `.skel`=4；我打开 `04-pending-1280x800.png` 可见 **4 条**灰色骨架条（第 4 条较短）。
- 影响：与矩阵自己引用的「`.skel`=4」和 R0-E-35 描述互相矛盾，属证据失真。
- 修正要求：02/04 改为「4 条骨架（DOM `skelCount=4`）」，§1.1 的「3 条（1280）」改 4；若坚持「可见 3 条」，须注明「可视区 3 条 + 1 条被裁切，实测 DOM 4 条」并给依据。

**N-12（P2）页 13 的邮箱 409 挂错写调用**
- 位置：`:426`（§5.5 第 13 行）、`:201`（§3.2 第 36 行）。
- 现状：写调用列只有 `POST /users/me/tokens :1574`、`DELETE :1636`，409 来源写 `common.go:90`「邮箱已被其他账号使用」。
- 实测：Token 增删不校验邮箱，不可能返回 `ErrEmailTaken`；该 409 的唯一入口是 `PUT /users/me {email}`，代码在 **`app.js:2206`**，位于 `renderMine`（`:2172`）即 13a「我的」视图；`common.go:90` 的 case、`:91` 的 fail 才对应此语义。
- 修正要求：写调用列补 `PUT /users/me :2206`，409 来源标注「13a 我的视图」；§3.2 第 36 行同步改写，删除「Token 增删…邮箱 409」的因果。

**N-13（P2）页 8 的持仓 409 挂错写调用**
- 位置：`:421`。
- 现状：写调用列 `PUT /instruments/:id :1480`、`PUT /holdings/:id :1523`，409 来源 `common.go:80`「该标的下仍有持仓」。
- 实测：`ErrInstrumentInUse` 只在**删除标的**时返回（`common.go:80–81`），前端删除在 `app.js:1412`，未被列入。
- 修正要求：写调用列补 `DELETE /instruments/:id :1412`，或把 409 来源改到对应 DELETE 行；判定不变。

**N-14（P2）逐页 409 语义不完整**
- 位置：`:417–426`（§5.5 整表）。
- 实测遗漏（按页）：
  - 页 4：`common.go:62` `ErrIncompleteReceipt`（同 clientUuid 不完整回执，`tx_update.go:107/163/206/230` 同类）、`common.go:51` `ErrEntityDeleted`（`tx.go:469/:492` 引用已删账户/分类）。
  - 页 5：同上 `:62`、`:49`（`tx_update.go:233`）。
  - 页 6：`common.go:100` 账户仍有交易、`:102` 账户仍有子账户（页面有删除操作）。
  - 页 7：`common.go:105` 分类仍有交易、`:107` 分类仍有子分类。
  - 页 12：`common.go:51`（`member.go:234` 账本已删/不可用）。
- 影响：不影响 N/A 结论（这些都不是版本冲突），但「逐页 409 语义」的完备性声明不成立。
- 修正要求：每页补齐可达 409 分支，或把表头改为「主要 409 语义（非穷举）」并在文首注明「本表不保证列尽全部 409」。

**N-15（P2）编辑已删交易返回 404，与设计注释/PRD 相反且未登记**
- 位置：§5.5 `:418`、实现缺口 `:430–434`（缺该项）。
- 实测：`tx_update.go` 先调 `loadTxRowByState(sess, bookID, txID, true)`，该函数查不到行即 `ErrTransactionNotFound`（`tx.go:1054–1055`）→ 404；而 `tx.go:54–56` 明确「已删是冲突，必须与『目标不存在』（404）区分」，`PRD:593` 要求「409 + `ENTITY_DELETED`」。
- 影响：W8 的核心就是 404/409 边界；这条偏差恰在边界上，却没进「实现缺口」，也没进 §5.4 的「可以/不可以推出」。
- 修正要求：在 §5.5 页 5 行补事实「已删目标 → 404 `ErrTransactionNotFound`」，并在实现缺口加第 4 条（编号由 Owner 指定，本文件不自造 R0-E）。

**N-16（P2）页面级 43 张的基准依据「mtime」不可复现**
- 位置：`:213–225` 十一行的「代码基准」、`:256`、`:691`。
- 实测：`stat -c '%y'` 对 `login/390x844/normal.png`、`accounts/390x844/normal.png`、`pairs/impl/home__impl__390x844.png` 均返回 **2026-11-26 09:54**；git 不保存 mtime，任何重新 clone 都会得到 checkout 时间，因此「mtime 2026-11-04」在本仓库工作树里不成立。
- 正确依据：43 张由 `575eae3`（2026-11-04）入库，早于 `0472484`（2026-11-05 18:18）——结论仍成立，但依据须更换。
- 修正要求：把「mtime 2026-11-04」改为「入库提交 `575eae3`（2026-11-04），早于 `0472484`；无 manifest，精确 SHA 不可得」；§4.2 与 §9 第 6 条同步改。

### P3（9 项，建议同批修）

**N-17（P3）E3/E15 的「代码基准 557e4d2」名不符实**
- 位置：`:215`、`:227`。
- 事实：`runtime/README:11` 写「历史**文档** HEAD `557e4d2`」，并声明「对应代码 SHA 未独立验证」。
- 修正：列名改「文档 HEAD（代码 SHA 未验证）」或值改「未验证」。

**N-18（P3）§1 合计行语义错位 + 待冻结计数省略第三数**
- 位置：`:86`；`:135`（18/50 或 17/51）。
- 事实：合计行把 17/51/36 填进 `normal/loading/empty` 三列（结构合法，脚本不计入，但人工易误读）；§2.1 投资行省略 N/A=36。
- 修正：合计行移出表格改脚注「有 17 / 缺 51 / N/A 36」，§2.1 写全「18/50/36 或 17/51/36」。

**N-19（P3）641 断点解释错误**
- 位置：`:129`。
- 原文：「641 是 `style.css:327`（max-width:640）与 `:334`（max-width:768 且 min-width:641）的断点外侧」。
- 实测：641 正落在 `:334` 的 `min-width:641` 内，该规则（`main { max-width:560px }`）在 641 生效，并非「断点外侧」。
- 修正：改为「641 处于 `:334` 的 641–768 带，现有全部批次按 360/390/430/1280 采集，故 641 无图」。

**N-20（P3）函数区间偏宽**
- 位置：`pairs/README.md:27`（`refillCategories()` `app.js:560–576`）、矩阵 `:253`（`openTxForm` `:550–578`）。
- 实测：`refillCategories()` 函数体为 `560–571`，`:572` 起是 `syncFormFields()`；`:550` 是 `openTxForm` 的 diff hunk 起点，`:578` 是 `syncFormFields` 绑定行。
- 修正：写 `app.js:560–571`，或注「hunk 550–578，函数体 560–571」。

**N-21（P3）G4 输出转录不严格**
- 位置：`:299–307`（输出中 233 行在 `:303`）。
- 原文把 `233: // 增量同步（阶段 3.1 / 3.2）` 列为 `grep -n "sync" router.go` 的输出；实测该命令只返回 `98`、`234`、`235`（中文注释不含 ASCII「sync」）。
- 修正：删去 233 行，或把命令改为 `grep -n -e "sync" -e "增量同步"`。

**N-22（P3）§10 落点写「§2.4」不存在**
- 位置：`:720`（N-03 行「§2.4 表改为…」）。
- 全文无 §2.4，对应表在 §5.4 `:400–408`。
- 修正：改为 §5.4。

**N-23（P3）页 3 的 7 行仍用「同上」，与「无概括」自述冲突**
- 位置：`:173–179`（§3.2 第 8–14 行）；`:145` 标题「36 格逐格，无空格无概括」。
- 事实：第 8–13 行「理由」列写「同上」，第 14 行「同上（页不存在…）」。
- 修正：每行写「整页无 Web 路由，该状态同样不适用（R01）」，或把标题改为「除页 3 同页共因外逐格」。

**N-24（P3）README 行数自述与实际不符**
- 位置：`pairs/README.md:182`「`SHA256SUMS.txt`（108 行）」。
- 实测：文件 109 行 / 109 条（108 条证据 + README 自身）。
- 修正：改「109 行（108 条 + 本文件）」。该行非本次新增，但本次正在改同一节，宜顺手校正。

**N-25（P3）规则 3 的「2 名家庭成员」是原型数，不是实现前置**
- 位置：`:41`（§0.3 规则 3）。
- 事实：`seed-manifest.json → dataset.counts.family = 2` 指原型数据集；`pairs/README:110` 明写「实现侧 1 名成员」；`members/390x844/normal.png` 与 `pairs/impl/members__impl__390x844.png` 均为 1 名。
- 修正：改为「family：原型 2 名，实现侧仅 owner 1 名（members 页未对齐，见 README §3）」。

---

## 9. 结论与签收意见

### 9.1 分项结论

| 交付物 | 结论 | 依据 |
|---|---|---|
| W1 矩阵骨架 / 计数 / 齐备表 / 证据台账 | **可签收** | 104=17+51+36、96=17+51+28、齐备 0、N/A 36/36 闭合、E 17 格 56 张全解析，8 段脚本输出与正文逐字一致 |
| W8 offline 列（12 格 R07 + 三要素） | **可签收** | R07 三要素齐全、每格挂 `R0-E-15`、与 `基准冻结:818`、`独立补漏:124–126` 对齐 |
| W8 页 5 版本冲突载体 | **可签收** | `655 → 130–135 → 64 → 670` 全链逐行成立 |
| W8 其余 11 页 409 语义 | **暂不签收** | N-12、N-13 挂接错误；N-14 六页漏列；N-15 未登记 404/409 偏差；N-11 骨架条数失真 |
| pairs/README 更正 | **可签收（含 2 处 P3）** | 剔除自指行后 108 条 `sha256sum -c` rc=0；README 实测哈希 `eda61aee…` 与 §9 第 11 条一致；未重签归档合理 |

### 9.2 明确回答

- **是否可以签收 T22 Batch 1 的 W1+W8 文档交付：暂不签收（PARTIAL）。** W1 与 W8 的 offline/页 5 部分已达交付级；W8 的「逐页 409 语义」不达交付级。修正 N-11…N-16 六项 P2 后（只改文档，不需重采、不需重跑脚本），可直接签收，无需再开一轮结构复核。
- **R0 是否仍 BLOCK：仍 BLOCK。** 本批只修订 W1/W8 文档，不改变第 3 条门禁；不得因本批文档修订宣布 T22 完成、第 3 条 PASS 或 R0 PASS。既有未闭环项（I-LOGIN-04/R0-E-39、I-LOGIN-11/12/R0-E-38、I-HOME-01/R0-E-01、OBS-ACCT-VIS-390、R0-E-35 骨架残留、409 无 errorCode、G-1/G-2/G-3 等）一条都不能因本批关闭。
- **本轮复核边界遵守情况**：只读 + 新增本报告；未改 WB 矩阵、未改 pairs README、未改业务代码/CSS/fixture/controls.json/runner/CI；未起服务、未采图、未造数、未写库；未执行 W2–W7/W9；未处理 P1–P6；未合并、未改 main。

---

## 附录 A：本轮独立复核命令清单（可复现）

```bash
cd fastledger-r0 && git checkout 3999cee
git diff --name-status 2e647d4 3999cee
git diff --name-only 2e647d4 3999cee -- server/ tools/ prototype/ .github/
git log --format='%h %ad %s' --date=short 0033a18..3999cee
git branch -a --contains 3999cee && git rev-parse main

# 三计数 / 台账 / 张数 / 列数 / 齐备 / 双分母（§7.1–7.8 原样重跑）
awk '/^## 1\./,/^## 2\./' docs/r0-status-matrix-13x8.md | awk -F'|' '…'
awk '/^## 3\./,/^## 4\./' docs/r0-status-matrix-13x8.md | grep -c '^| [0-9]'
awk -F'|' '/^\| E[0-9]+ \|/ {…}' docs/r0-status-matrix-13x8.md
# 逐格比对（§7.6）与 V-10（从 f4c11c 报告 §6 提取）
python3 /tmp/v10_check.py docs/r0-status-matrix-13x8.md

# 独立补充校验（WB 未覆盖）
python3  # 展开花括号逐文件断言：17 行 / 56 张 / 0 缺失
python3  # §2 视口表 vs §4 台账：17 行 0 不一致
python3  # N/A 复合码闭合：36/36，0 缺失 0 多余 0 不符

# 代码事实
grep -n 'baseVersion\|syncVersion' server/web/dist/app.js
grep -n 'users/me\|/instruments/' server/web/dist/app.js
grep -n 'refillCategories\|syncFormFields\|暂无可选分类' server/web/dist/app.js
sed -n '130,135p' server/internal/service/tx_update.go
sed -n '1044,1057p' server/internal/service/tx.go
grep -n 'ErrEntityDeleted\|ErrInstrumentInUse\|ErrEmailTaken\|ErrAccountInUse' server/internal/api/common.go
grep -n '640px\|641px' server/web/dist/style.css

# 证据文件
python3 -c 'json.load(open("docs/r0-screenshots/runtime/dom-snapshots.json"))'  # skelCount 全 4
stat -c '%y' docs/r0-screenshots/login/390x844/normal.png                       # 2026-11-26
git log -1 --date=short -- docs/r0-screenshots/login/390x844/normal.png         # 575eae3 2026-11-04

# 哈希
sed -n '108p' docs/r0-screenshots/pairs/SHA256SUMS.txt
sha256sum docs/r0-screenshots/pairs/README.md
grep -v README.md docs/r0-screenshots/pairs/SHA256SUMS.txt > /tmp/sums_no_readme.txt
sha256sum -c /tmp/sums_no_readme.txt --quiet; echo rc=$?                          # rc=0，108 条
```

## 附录 B：我逐张打开/核实的图（本轮）

`runtime/02-rejected-network-390x844.png`（3 卡 + 4 骨架 + `✕ Failed to fetch`）、`runtime/04-pending-1280x800.png`（3 卡 + 4 骨架，桌面导航）、`accounts/390x844/normal.png`（3 账户均 ¥0.00）、`categories/390x844/normal.png`（默认分类树）。

上轮已核实、本轮仅引用其结论的图：`pairs/impl/home__impl__390x844.png`、`budget__impl__390x844.png`、`report__impl__390x844.png`、`invest__impl__390x844.png`、`members__impl__390x844.png`、`add-transaction__impl__390x844.png`、`add-transaction/390x844/normal.png`、`members/390x844/normal.png`、`budget/390x844/empty.png`、`home/390x844/empty.png`、`home/390x844/error.png`、`runtime/01`、`runtime/03`。
