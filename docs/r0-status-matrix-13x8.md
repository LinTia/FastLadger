# R0-T22 · 13 页 × 8 状态矩阵（批次 1 · 第 2 版：W1 骨架 + W8 边界，依交付级复核修正）

> 任务编号：R0-T22（九条第 3 条）。文档性质：**只读核查 + 矩阵/证据文档**。不改业务代码、CSS、`app.js`、测试、`controls.json`、fixture、runner、CI、统计；不采集截图、不起服务、不造数、不写库。
> 基线：`work/r0-t22-plan` @ `0033a1899878c2b832578cbfbb11ab858c2f8649`（T22 执行计划，已签收）。本文件所在分支：`work/r0-t22-batch1`；被审提交 `2e647d4d2ad664fa79a60035e7df9d7d045cbf01`，本版在其之上追加修正提交（未 amend、未 force-push）。
> 本版修订依据：`docs/r0-t22-b1-wb-delivery-recheck.md`（位于 `review/t22-w1-w8-wb-recheck` @ `f4c11c4939df81f6af961163b951c1882bed6392`），被审提交 `2e647d4d2ad664fa79a60035e7df9d7d045cbf01`，结论「暂不签收」。
> 结论边界：本矩阵只冻结**现状**。104 格 = **有 17** + **缺 51** + **N/A 36**；**三档齐备 0**；第 3 条远未达。**本文件不构成第 3 条 PASS，也不构成 T22 完成或 R0 PASS**，R0 维持 BLOCK。

---

## 0. 冻结口径与采信规则

### 0.1 基本口径

| 项 | 口径 | 来源 |
|---|---|---|
| 页数 | 13 页（T21 台账 13 行），其中页 3 为整页 `N/A` | `r0-page-mapping-13.md:31–43` |
| 状态 | 8 个：`normal` / `loading` / `empty` / `error` / `success` / `permission` / `offline` / `conflict` | T21 §4 第 7 条（`r0-page-mapping-13.md:55`）；T22 计划 §3 |
| **清点分母** | 13 × 8 = **104**（「一共要清点多少格」） | 同上 |
| **适用分母** | 104 − 8（页 3 整页 N/A）= **96**（「真正需要覆盖多少格」） | 本文件 |
| 判定符号 | `有` = 存在呈现该状态的图（路径见 §4）；`缺` = 当前无该状态的可采信图；`N/A` = 不适用，**逐格给理由**（见 §3） | T22 计划 §2.1 W1 |
| 单元格记法 | 判定 + 证据号（`E#`）或理由码（`R#`）；路径在 §4，理由在 §3，409 语义在 §5.5 | 本文件 |
| 强制视口 | 390 / 641 / 1280 三档；360 / 430 为可选补充档 | T22 计划 §3 列定义 |
| 非原型扩展页 | `books` / `transactions` / `recycle` 三页无原型路由，**不入 104 格**（P2 待裁决） | T22 计划 §3 补充、§6 P2 |

**两个分母不得混用**：`17 / 104` 是「清点进度」，`17 / 96` 才是「覆盖率」；`0 / 96` 是「齐备覆盖率」。任何把 `17 / 104` 写成覆盖率的表述都高估了进度（104 含页 3 的 8 个不适用格）。

### 0.2 三轴分离（本版核心修正）

| 轴 | 回答的问题 | 取值 | 落点 |
|---|---|---|---|
| **存在性** | 该页该状态**有没有**呈现该状态的图 | `有` / `缺` / `N/A` | §1 |
| **齐备度** | 有图的格是否 **390 / 641 / 1280 三档**齐 | `✓` / `✗` | §2 |
| **可用性** | 图能否代表**当前实现**（同状态 + 数据前置已知 + 基准未过期） | 采信 / 降级需复采 / 需复核 | §4 |

三轴不得互相替代：`有` **不等于** `齐备`（17 个「有」格无一具备 641），`齐备` **也不等于** `可用`（E4 图基准已过期）。上一版把三轴压在一个符号里，是 F-03/F-04 的根因。

### 0.3 截图采信规则（本版新增，解决 F-01/F-04 的不一致）

1. **存在性只看「有没有呈现该状态的图」**，不看「图的数据是否与原型一致」——后者属原型对照（`pairs/README.md` §4）与可用性轴，不参与 `有/缺` 判定。
2. **证据优先级**：页面级目录（`<page>/<vp>/<state>.png`）> `pairs/impl` 同状态图 > `runtime` 隔离运行时图。上级无图时降级采信下级，并须在 §4 标注降级原因。
3. **采信 `pairs/impl` 必须标注数据前置**：22 笔交易 / 5 账户 / 37 分类 / **0 投资** / 5 预算 / 2 名家庭成员（`pairs/seed-manifest.json` → `dataset.counts`）。
4. **采信 `runtime` 必须标注**：隔离运行时、单次实测、注入方式、已知缺陷（骨架未清理，R0-E-35 未关闭）。
5. **图不呈现该状态 → 不得记 `有`**：空白页（旧 E3）、混合态（投资 pairs）、状态不明者，分别记 `缺` 或列入 §2.1 待冻结。

上一版对同一类证据（仅 `pairs` 有图）在页 5 采信、在页 2/9/10 不采信，且未写规则；规则 2 消除该不一致。按规则 5，投资 pairs 图（总资产有值但持仓与标的库为空）不判 `有`，列入 §2.1 待冻结。

### 0.4 引用文件映射（N-10）

| 简写 | 实际路径 |
|---|---|
| `PRD` | `docs/FastLedger-产品需求文档-PRD.md` |
| `收尾清单` | `docs/FastLedger-R0-收尾执行清单.md` |
| `基准冻结` | `docs/FastLedger-R0-基准冻结与差异盘点.md` |
| `落地方案` | `docs/FastLedger-落地方案.md` |
| `任务单` | `docs/FastLedger-R0-收口总任务单-2026-11-25.md` |
| `T21` / `r0-page-mapping-13.md` | `docs/r0-page-mapping-13.md` |
| `T22 计划` | `docs/r0-t22-execution-plan.md` |
| `截图README` | `docs/r0-screenshots/README.md` |
| `pairs/README` | `docs/r0-screenshots/pairs/README.md` |
| `runtime/README` | `docs/r0-screenshots/runtime/README.md` |
| `复核报告` | `docs/r0-t22-b1-wb-delivery-recheck.md` |

### 0.5 禁止的写法

不得把 `缺` 写成「已实现 / 待验 / PASS」；不得把 `N/A` 写成「无需验证」或「免实现」；不得以「文档齐全」「截图张数达标」「零改判」代替逐格判定；不得用「Web 无冲突载体」作为 conflict 列的**统一**理由（页 5 有载体，见 §5.5）。

---

## 1. 主矩阵（13 行 × 10 列 · 本版改判后冻结）

| # | 页面 | normal | loading | empty | error | success | permission | offline | conflict |
|---|---|---|---|---|---|---|---|---|---|
| 1 | 登录 | 有 E1 | 缺 | N/A R02 | 缺 | 缺 | N/A R03 | N/A R07/R0-E-15 | N/A R08-01 |
| 2 | 首页账单 | 有 E14 | 有 E15 | 有 E2 | 有 E3 | 缺 | 缺 | N/A R07/R0-E-15 | N/A R08-02 |
| 3 | 截图识别记账 | N/A R01 | N/A R01 | N/A R01 | N/A R01 | N/A R01 | N/A R01 | N/A R01 | N/A R01 |
| 4 | 手动记账/转账 | 有 E4 | 缺 | N/A R04 | 缺 | 缺 | 缺 | N/A R07/R0-E-15 | N/A R08-04 |
| 5 | 交易详情 | 有 E5 | 缺 | N/A R05 | 缺 | 缺 | 缺 | N/A R07/R0-E-15 | 缺（有版本冲突载体） |
| 6 | 账户 | 有 E6 | 缺 | 缺 | 缺 | 缺 | 缺 | N/A R07/R0-E-15 | N/A R08-06 |
| 7 | 两级分类 | 有 E7 | 缺 | 缺 | 缺 | 缺 | 缺 | N/A R07/R0-E-15 | N/A R08-07 |
| 8 | 投资资产 | 缺 | 缺 | 有 E8 | 缺 | 缺 | 缺 | N/A R07/R0-E-15 | N/A R08-08 |
| 9 | 预算 | 有 E16 | 缺 | 有 E9 | 缺 | 缺 | 缺 | N/A R07/R0-E-15 | N/A R08-09 |
| 10 | 报表 | 有 E17 | 缺 | 有 E10 | 缺 | 缺 | 缺 | N/A R07/R0-E-15 | N/A R08-10 |
| 11 | 导出 | 有 E11 | 缺 | 缺 | 缺 | 缺 | 缺 | N/A R07/R0-E-15 | N/A R08-11 |
| 12 | 家庭成员 | 有 E12 | 缺 | 缺 | 缺 | 缺 | 缺 | N/A R07/R0-E-15 | N/A R08-12 |
| 13 | 我的设置 | 有 E13 | 缺 | N/A R06 | 缺 | 缺 | 缺 | N/A R07/R0-E-15 | N/A R08-13 |
| 合计 | 13 页 | 17 | 51 | 36 | — | — | — | — | — |

**三计数**：104 格 = **有 17** + **缺 51** + **N/A 36**（脚本核对见 §7.1，非人工累加）。
**齐备度**：17 个「有」格中 **三档齐备 0 格**（641 全库 0 张，见 §2）。
**适用格口径**：96 格内 = 有 17 + 缺 51 + N/A 28（= 36 − 页 3 的 8 格）。

### 1.1 改判区（相对被审 `2e647d4`，5 格）

| 页面 | 状态 | 原判 | 本版 | 依据 |
|---|---|---|---|---|
| 首页账单 | normal | 缺 | 有 E14 | `pairs/impl/home__impl__{360x800,390x844,430x932,1280x800}.png`；已打开 390 档：收入 ¥28,000.00 / 支出 ¥1,089.30 / 结余 ¥30,126.70 + 交易流水 |
| 首页账单 | loading | 缺 | 有 E15 | `runtime/01-pending-390x844.png`、`runtime/04-pending-1280x800.png`；已打开：三卡已渲染 + 4 条骨架条（390）/ 3 条（1280），无 toast |
| 预算 | normal | 缺 | 有 E16 | `pairs/impl/budget__impl__*.png`；已打开 390 档：总预算 + 餐饮/交通/旅游/购物共 5 条，含「已用 ¥1,089.30 / ¥8,000.00」与进度条 |
| 报表 | normal | 缺 | 有 E17 | `pairs/impl/report__impl__*.png`；已打开 390 档：近 6 个月收支柱状 + 分类占比 6 行 |
| 交易详情 | conflict | N/A R08 | 缺（有版本冲突载体） | `app.js:655` 发送 `baseVersion` → `tx_update.go:130–135` 校验 → `common.go:64` 映射 409 → `app.js:670` 提示「（请刷新列表后重试）」；详见 §5.5 |

**其余 99 格判据未变**（与 T22 计划 §3 预填逐格比对见 §7.6：改判 5 格、一致 99 格）。改判**只依证据**，未依「零改判」这一形式指标——上一版把「抄得准」当成「抄得对」，是本版要修的方法问题（N-04）。

---

## 2. 视口齐备度（17 个「有」格 × 390 / 641 / 1280）

| # | 页面 | 状态 | 390 | 641 | 1280 | 齐备 | 说明 |
|---|---|---|---|---|---|---|---|
| 1 | 登录 | normal | ✓ | ✗ | ✓ | ✗ | 360/430 另有 |
| 2 | 首页账单 | empty | ✓ | ✗ | ✓ | ✗ | 360/430 另有 |
| 3 | 首页账单 | error | ✓ | ✗ | ✗ | ✗ | runtime 仅 390 两档注入 |
| 4 | 手动记账/转账 | normal | ✓ | ✗ | ✓ | ✗ | 360/430 另有；**基准过期需复采**（§4.2） |
| 5 | 交易详情 | normal | ✓ | ✗ | ✓ | ✗ | pairs 4 档 |
| 6 | 账户 | normal | ✓ | ✗ | ✓ | ✗ | 子页 account-detail 仅 390/1280 |
| 7 | 两级分类 | normal | ✓ | ✗ | ✓ | ✗ | 无 360/430 |
| 8 | 投资资产 | empty | ✓ | ✗ | ✓ | ✗ | 无 360/430 |
| 9 | 预算 | empty | ✓ | ✗ | ✓ | ✗ | 无 360/430 |
| 10 | 报表 | empty | ✓ | ✗ | ✓ | ✗ | 无 360/430 |
| 11 | 导出 | normal | ✓ | ✗ | ✓ | ✗ | 无 360/430 |
| 12 | 家庭成员 | normal | ✓ | ✗ | ✓ | ✗ | 无 360/430；**基准过期需复核**（§4.2） |
| 13 | 我的设置 | normal | ✓ | ✗ | ✓ | ✗ | settings 4 档 + mine 2 档，**两视图均需覆盖**（§4.1） |
| 14 | 首页账单 | normal | ✓ | ✗ | ✓ | ✗ | pairs 4 档 |
| 15 | 首页账单 | loading | ✓ | ✗ | ✓ | ✗ | runtime 390 + 1280 |
| 16 | 预算 | normal | ✓ | ✗ | ✓ | ✗ | pairs 4 档 |
| 17 | 报表 | normal | ✓ | ✗ | ✓ | ✗ | pairs 4 档 |

**统计（脚本见 §7.7）**：有格 **17**；三档齐备 **0**；缺 641 **17/17**；缺 1280 **1**（E3，仅隔离运行时 390）。
**641 零覆盖的代码原因**：641 是 `style.css:327`（`max-width:640px`）与 `:334`（`max-width:768px and min-width:641px`）的断点外侧，现有全部批次都按 360/390/430/1280 采集。W2 属**从零采集**，不是补档。

### 2.1 待 Owner / WB 冻结的 5 格（不得擅自判 PASS）

| 页面 | 状态 | 现判 | 两种候选判据 | 冻结后才能定 |
|---|---|---|---|---|
| 投资资产 | normal | 缺 | ① `有`：pairs 图有资产数据（¥26,258.70 / 负债 ¥3,868.00）；② 仍 `缺`：图是「资产有值 + 持仓 0 + 标的库空」的**混合态**，normal 语义不明 | 该格 `有/缺`，并决定计数 18/50 或 17/51 |
| 首页账单 | permission | 缺 | ① `N/A`：首页只读、无角色差异，权限态无判定主体；② `缺`：需 4 角色实测证明「无差异」 | 该格 `缺/N/A`，移动 1 格计数 |
| 导出 | empty | 缺 | ① `缺` + 判据「点『预览』后的空文本截图 + 断言 `没有可导出的数据（该范围内无交易）。`」（`app.js:1066`）；② `N/A`：页面无列表空态，空反馈属动作态 | 该格 `缺/N/A` |
| 手动记账/转账 | empty | N/A R04 | ① 维持 `N/A`：弹层不算独立页面，`#formDlg` 无列表空态；② 改 `缺`：无分类时渲染「（暂无可选分类）」（`app.js:568`），属弹层内空态提示 | 该格 `N/A/缺` |
| 家庭成员 | empty | 缺 | ① 维持 `缺`：需构造「空列表」；② 改 `N/A`：账本必有 owner，`renderMembers:1667` 从 `GET /members` 取列表，空列表不可达 | 该格 `缺/N/A` |

**这 5 格本轮一律不改判**：报告 §5.3 与用户口径均要求保留底稿 `有 17 / 缺 51 / N/A 36`；擅自改判会移动统计基线。表中「两种候选判据」的作用是把选择权交给 Owner，而不是用模糊措辞掩盖未决。

---

## 3. N/A 理由台账（36 格逐格，无空格无概括）

### 3.1 理由码定义

| 码 | 含义 | 三要素 / 依据 |
|---|---|---|
| R01 | 页面不存在 | Web 无该页路由与 OCR 调用；`PRD:326`、`PRD:333`、`基准冻结:86`、`基准冻结:127` 明确不做 Web OCR；页 3 整页 8 格 |
| R02 | 登录无列表形态 | 登录为表单页（用户名/密码），无集合语义 |
| R03 | 未登录无角色上下文 | 权限态无判定主体 |
| R04 | 弹层无列表空态 | `#formDlg` 无列表；但无分类时渲染「（暂无可选分类）」（`app.js:568`），该空前置提示的归属口径**待冻结**（§2.1） |
| R05 | 详情必先有记录 | 无「空详情」形态（先有交易才可进入 `app.js:2008`） |
| R06 | 设置页无空态 | 固定表单区块，无集合 |
| R07 | **Web 无离线能力** | ① **举证**：`app.js` 0 处 `navigator.onLine` / `serviceWorker` / `EventSource` / `BroadcastChannel` / `setInterval` / 写队列（§5.2 G6/G9）；② **产品有要求**：`落地方案:364`（离线队列 + `clientUuid` 幂等，联网后 push）、`:365`（断线重连先 pull）、`PRD:207`（离线补传须带稳定操作键）、`PRD:593`（旧离线队列回流）；③ **缺口未闭环**：**R0-E-15**（`基准冻结:818`；**R0-E-25 并入 R0-E-15，不单独计数**，见 `docs/FastLedger-R0-独立补漏与复核记录.md:124–126`） |
| R08-01…R08-13 | 有 409 但**不是版本冲突** | 逐页语义见 §5.5；编号后两位 = 页号。**不得**再用「Web 无冲突载体」作为统一理由 |

**R07 三要素缺一不可**：只写①会把「未实现」洗成「不适用」；只写②③则无 Web 侧举证。R07 是「当前 Web 无载体（事实）+ 产品要求存在（事实）+ 缺口登记未闭环（事实）」的合并陈述，**不是免实现声明**。

### 3.2 台账

| # | 页面 | 状态 | 理由码 | 理由（为什么不适用） | 依据 |
|---|---|---|---|---|---|
| 1 | 登录 | empty | R02 | 登录为表单页，无列表或集合形态，不存在空态语义 | `r0-page-mapping-13.md:31`；`app.js:161` renderLogin |
| 2 | 登录 | permission | R03 | 未登录前不存在角色/权限上下文，权限态无判定主体 | `r0-page-mapping-13.md:31`；`app.js:161` |
| 3 | 登录 | offline | R07 | 三要素见 §3.1：Web 无离线载体 + 产品有离线要求 + R0-E-15 未闭环；登录页无额外离线分支 | §5.2 G1/G6/G9；§5.6 |
| 4 | 登录 | conflict | R08-01 | 登录为 `POST /auth/login`（`app.js:193`、`:209`），失败以 401/锁定为主，不写实体、不参与版本比对 → 非版本冲突 | §5.5 第 1 行 |
| 5 | 首页账单 | offline | R07 | 三要素同 §3.1；首页为只读列表（`app.js:1912` `GET /transactions`），无离线补传分支 | §5.2；§5.5 第 2 行 |
| 6 | 首页账单 | conflict | R08-02 | 首页只读（`GET /transactions` `:1912`），无写调用 → 不产生版本冲突；删除/恢复在原型外的交易总表 | §5.5 第 2 行 |
| 7 | 截图识别记账 | normal | R01 | Web 无该页路由、无 OCR 调用，页面不存在，8 状态全不适用 | `r0-page-mapping-13.md:33`；`PRD:326`；`PRD:333`；`基准冻结:86`；`基准冻结:127` |
| 8 | 截图识别记账 | loading | R01 | 同上 | 同上 |
| 9 | 截图识别记账 | empty | R01 | 同上 | 同上 |
| 10 | 截图识别记账 | error | R01 | 同上 | 同上 |
| 11 | 截图识别记账 | success | R01 | 同上 | 同上 |
| 12 | 截图识别记账 | permission | R01 | 同上 | 同上 |
| 13 | 截图识别记账 | offline | R01 | 同上（页不存在，离线态亦无从谈起） | 同上 |
| 14 | 截图识别记账 | conflict | R01 | 同上（页不存在，冲突态亦无从谈起） | 同上 |
| 15 | 手动记账/转账 | empty | R04 | `#formDlg` 弹层表单无列表空态；但无分类时渲染「（暂无可选分类）」（`app.js:568`），该提示是否计作空态**待冻结**（§2.1）；本轮保守维持 N/A | `r0-page-mapping-13.md:34`；`app.js:548`、`:568` |
| 16 | 手动记账/转账 | offline | R07 | 三要素同 §3.1；建单走 `POST /transactions`（`app.js:603`），无本地队列与重放 | §5.2；§5.5 第 4 行 |
| 17 | 手动记账/转账 | conflict | R08-04 | `POST` 建单 body 不含 `baseVersion`（`app.js:586–595`）；`common.go:49` 的 409 是「同 `clientUuid` 不同内容」，属幂等防御（前端每次新 uuid），非实体版本并发 | §5.5 第 4 行 |
| 18 | 交易详情 | empty | R05 | 详情页必先有交易记录才进入，无「空详情」形态 | `r0-page-mapping-13.md:35`；`app.js:2008` |
| 19 | 交易详情 | offline | R07 | 三要素同 §3.1；编辑走 `PUT /transactions/:id`（`app.js:664`），无离线队列 | §5.2；§5.5 第 5 行 |
| 20 | 账户 | offline | R07 | 三要素同 §3.1；账户增删改（`app.js:772` 等）无离线队列 | §5.2；§5.5 第 6 行 |
| 21 | 账户 | conflict | R08-06 | `PUT /accounts/:id`（`app.js:772`）不传 `baseVersion`；`account.go:238–240` 的 409 是更新命中 0 行（行已删/不存在），语义为「目标已消失」而非并发版本比对 | §5.5 第 6 行 |
| 22 | 两级分类 | offline | R07 | 三要素同 §3.1；分类增删改无离线队列 | §5.2；§5.5 第 7 行 |
| 23 | 两级分类 | conflict | R08-07 | `PUT /categories/:id`（`app.js:2082`）不传版本；`category.go:233–235` 的 409 同样是更新命中 0 行 | §5.5 第 7 行 |
| 24 | 投资资产 | offline | R07 | 三要素同 §3.1；标的与持仓写入无离线队列 | §5.2；§5.5 第 8 行 |
| 25 | 投资资产 | conflict | R08-08 | `PUT /instruments/:id`（`:1480`）、`PUT /holdings/:id`（`:1523`）不传版本；`common.go:80` 的 409 是「该标的下仍有持仓记录」业务约束 | §5.5 第 8 行 |
| 26 | 预算 | offline | R07 | 三要素同 §3.1；预算写入无离线队列 | §5.2；§5.5 第 9 行 |
| 27 | 预算 | conflict | R08-09 | `PUT/POST /budgets`（`:1251`、`:1257`）不传版本；`model/budget.go:22` 明写 `budgets` 表**没有 `sync_version` 列**（预算不在增量同步契约内）；`common.go:85` 的 409 是预算唯一性 | §5.5 第 9 行 |
| 28 | 报表 | offline | R07 | 三要素同 §3.1；报表只读（`:862`、`:902` 均 `GET`） | §5.2；§5.5 第 10 行 |
| 29 | 报表 | conflict | R08-10 | 报表页只读（`GET /reports/monthly` `:862`、`GET /reports/categories` `:902`），无写调用 | §5.5 第 10 行 |
| 30 | 导出 | offline | R07 | 三要素同 §3.1；导出为只读拉取（`GET /transactions` `:1064`） | §5.2；§5.5 第 11 行 |
| 31 | 导出 | conflict | R08-11 | 导出页只读（`:1064`），无写调用 | §5.5 第 11 行 |
| 32 | 家庭成员 | offline | R07 | 三要素同 §3.1；邀请/改角色/移除均为在线直调 | §5.2；§5.5 第 12 行 |
| 33 | 家庭成员 | conflict | R08-12 | 邀请 `:1689`、改角色 `:1761`/`:1771`、移除 `:1786` 均不传版本；`common.go:115` 的 409 是成员上限 20（容量类），非版本冲突 | §5.5 第 12 行 |
| 34 | 我的设置 | empty | R06 | 设置页为固定表单（Token 增删等区块），无集合空态 | `r0-page-mapping-13.md:43`；`app.js:1549` |
| 35 | 我的设置 | offline | R07 | 三要素同 §3.1；Token 增删（`:1574`、`:1636`）为在线直调 | §5.2；§5.5 第 13 行 |
| 36 | 我的设置 | conflict | R08-13 | Token 增删不传版本；`common.go:90` 的 409 是「邮箱已被其他账号使用」（账号类），非实体版本 | §5.5 第 13 行 |

**理由码分布（脚本见 §7.2）**：R01 × 8 + R02 × 1 + R03 × 1 + R04 × 1 + R05 × 1 + R06 × 1 + R07 × 12 + R08-01/02/04/06/07/08/09/10/11/12/13 各 1（共 11）= **36**。

---

## 4. 证据路径台账（17 个「有」格，共 56 张）

**「新建账本默认态」定义**（本节多处引用）：新建账本后未导入数据的状态 = **3 个默认账户**（现金 / 活期储蓄 / 信用卡，余额均 ¥0.00）、**默认分类树**（餐饮/交通等大类 + 子类）、**0 交易**、**0 预算**、**0 标的与持仓**。它不是物理空库——账本初始化会写入上述默认数据。本轮抽样逐张打开 `accounts/390x844/normal.png`（3 账户，¥0.00）、`categories/390x844/normal.png`（默认分类树）、`budget/390x844/empty.png`（文案「还没有预算」）、`home/390x844/empty.png`（文案「还没有账单」）四张，与该定义一致；其余页面级图按目录名与采集批次推定，未逐张打开（见 §9 第 8 条）。

| 编号 | 页面 | 状态 | 视口（已有） | 张数 | 证据路径（相对 docs/r0-screenshots/） | 代码基准 | 数据前置 | 状态与动作 | 缺口 | 子页归属 |
|---|---|---|---|---|---|---|---|---|---|---|
| E1 | 登录 | normal | 360 / 390 / 430 / 1280 | 4 | login/{360x800,390x844,430x932,1280x800}/normal.png | 未记录（无 manifest；mtime 2026-11-04） | 未登录 | 打开登录页，默认渲染 | 641 | 主页面 |
| E2 | 首页账单 | empty | 360 / 390 / 430 / 1280 | 4 | home/{360x800,390x844,430x932,1280x800}/empty.png | 未记录（同上） | 新建账本默认态；0 交易（total=0） | 打开首页，空账单流 | 641 | 主页面 |
| E3 | 首页账单 | error | 390 | 2 | runtime/{02-rejected-network-390x844,03-rejected-503-390x844}.png | 557e4d2（runtime/README:14 记录的历史文档 HEAD） | 新建账本默认态 + 隔离实例；仅拦截 `GET /books/1/transactions` | 注入 Failed / 503 后 0.9s 截图；整页完整 + 骨架残留 + toast | 641、1280 | 主页面（降级采信 runtime） |
| E4 | 手动记账/转账 | normal | 360 / 390 / 430 / 1280 | 4 | add-transaction/{360x800,390x844,430x932,1280x800}/normal.png | 未记录（mtime 2026-11-04，**早于 `0472484`**） | 新建账本默认态（3 默认账户）；0 交易 | 打开「记一笔」弹层 | 641；**基准过期需复采**（图显示支出类型下分类=工资） | 主页面 |
| E5 | 交易详情 | normal | 360 / 390 / 430 / 1280 | 4 | pairs/impl/transaction-detail__impl__{360x800,390x844,430x932,1280x800}.png | bff6a2b（pairs/README:13） | 22 笔 / 5 账户 / 37 分类 / 0 投资 / 5 预算 | 点开某笔交易详情 | 641；图早于 0472484 但该页代码路径未变 | 主页面 |
| E6 | 账户 | normal | 360 / 390 / 430 / 1280 | 6 | accounts/{360x800,390x844,430x932,1280x800}/normal.png + account-detail/{390x844,1280x800}/normal.png | 未记录（mtime 2026-11-04） | 默认 3 账户（现金/活期储蓄/信用卡，余额均 ¥0.00）；非 pairs 的 5 账户数据集 | 打开账户列表 | 641；子页缺 360/430/641 | 主页面 4 张 + 子页 2 张（子页不入 13 页齐备判定） |
| E7 | 两级分类 | normal | 390 / 1280 | 2 | categories/{390x844,1280x800}/normal.png | 未记录（mtime 2026-11-04） | 新建账本默认态（默认分类树） | 打开分类管理 | 360、430、641 | 主页面 |
| E8 | 投资资产 | empty | 390 / 1280 | 2 | invest/{390x844,1280x800}/empty.png | 未记录（mtime 2026-11-04） | 0 标的 / 0 持仓 | 打开资产页，空持仓与空标的库 | 360、430、641 | 主页面 |
| E9 | 预算 | empty | 390 / 1280 | 2 | budget/{390x844,1280x800}/empty.png | 未记录（mtime 2026-11-04） | 0 预算 | 打开预算页，空态 | 360、430、641 | 主页面 |
| E10 | 报表 | empty | 390 / 1280 | 2 | report/{390x844,1280x800}/empty.png | 未记录（mtime 2026-11-04） | 新建账本默认态；0 交易 | 打开报表页，空态 | 360、430、641 | 主页面 |
| E11 | 导出 | normal | 390 / 1280 | 2 | export/{390x844,1280x800}/normal.png | 未记录（mtime 2026-11-04） | 新建账本默认态；0 交易 | 打开导出页，配置表单 | 360、430、641 | 主页面 |
| E12 | 家庭成员 | normal | 390 / 1280 | 2 | members/{390x844,1280x800}/normal.png | 未记录（mtime 2026-11-04，**早于 `0472484`**） | 1 名成员（owner） | 打开家庭成员页 | 360、430、641；**基准过期需复核**（邀请入口角色门控已改） | 主页面 |
| E13 | 我的设置 | normal | settings 360 / 390 / 430 / 1280；mine 390 / 1280 | 6 | mine/{390x844,1280x800}/normal.png + settings/{360x800,390x844,430x932,1280x800}/normal.png | 未记录（mtime 2026-11-04） | 新建账本默认态 | 打开设置页与「我的」页 | 641（两视图各需 1 张，共 2 张）；见 §4.1 | 13a「我的」2 张 + 13b 设置 4 张 |
| E14 | 首页账单 | normal | 360 / 390 / 430 / 1280 | 4 | pairs/impl/home__impl__{360x800,390x844,430x932,1280x800}.png | bff6a2b | 22 笔 / 5 账户 / 37 分类 / 0 投资 / 5 预算 | 有数据首页，账单流分组渲染 | 641；图早于 0472484 但该页代码路径未变 | 主页面 |
| E15 | 首页账单 | loading | 390 / 1280 | 2 | runtime/{01-pending-390x844,04-pending-1280x800}.png | 557e4d2（同 E3） | 新建账本默认态 + 隔离实例；暂停 `GET /books/1/transactions` | 请求挂起，1.8s 截图；三卡已渲染 + 骨架条 | 641；**隔离运行时、单次实测** | 主页面（降级采信 runtime） |
| E16 | 预算 | normal | 360 / 390 / 430 / 1280 | 4 | pairs/impl/budget__impl__{360x800,390x844,430x932,1280x800}.png | bff6a2b | 5 条预算 + 22 笔交易 | 有数据预算页，含进度条 | 641；图早于 0472484 但该页代码路径未变 | 主页面 |
| E17 | 报表 | normal | 360 / 390 / 430 / 1280 | 4 | pairs/impl/report__impl__{360x800,390x844,430x932,1280x800}.png | bff6a2b | 22 笔交易 | 近 6 月柱状 + 分类占比 | 641；图早于 0472484 但该页代码路径未变 | 主页面 |

**张数核对**：4+4+2+4+4+6+2+2+2+2+2+2+6+4+2+4+4 = **56**。
三来源分解：页面级目录 **36** 张（E1/E2/E4/E6/E7/E8/E9/E10/E11/E12/E13）+ `pairs/impl` **16** 张（E5/E14/E16/E17）+ `runtime` **4** 张（E3/E15）= **56**。

### 4.1 子页与多视图归属

| 子页 / 视图 | 所属格 | 已有视口 | 张数 | 齐备判定 |
|---|---|---|---|---|
| 账户列表（accounts） | E6 | 360 / 390 / 430 / 1280 | 4 | 参与 13 页齐备判定 |
| 账户详情（account-detail） | E6 子页 | 390 / 1280 | 2 | **不参与** 13 页齐备判定，仅作补充证据 |
| 「我的」（mine）视图 13a | E13 | 390 / 1280 | 2 | 需 641 各 1 张 |
| 设置（settings）视图 13b | E13 | 360 / 390 / 430 / 1280 | 4 | 需 641 1 张 |

**页 13 合并判据（F-10）**：`pairs/README.md:94` 已证 `settings` 与 `mine` 在**原型侧**是同一个 `/profile`（两图 SHA256 相同），但**实现侧是两个视图**（`renderMine:2172`、`renderSettings:1549`）。本版口径：**「有」按任一视图有图计，齐备按两视图均覆盖计**；641 的覆盖对象是 **mine 与 settings 各 1 张 = 2 张**。
不拆 `13a` / `13b` 两行的原因：拆行会让总格从 104 变 112，属结构变更，**须 Owner 同意**；本轮不改结构，只冻结判据。此项列入 §2.1 之外的独立待议项，见 §9 第 3 条。

### 4.2 基准漂移（F-05）

| 事实 | 证据 |
|---|---|
| `pairs` 采集时点基准 | `bff6a2b351bf5a039cdf7c762ec2693fb330cd00`（2026-11-04 13:45），`pairs/README.md:13` |
| `pairs` 采集时间 | 2026-11-05 13:38（`manifest-impl.json` → `capturedAt` 2026-11-05T05:38Z） |
| 之后 `server/` **并非零改动** | `git diff --stat bff6a2b..HEAD -- server/` → `app.js` **+43 / −9（52 行）**，唯一提交 `0472484`（2026-11-05 18:18） |
| `0472484` 改了三处 | ① `renderTxns` 挂载 + 加载失败反馈（`app.js:383`、`:397–423`）；② `openTxForm` 分类按类型过滤 + 「（暂无可选分类）」（`:550–578`）；③ `renderMembers` 邀请入口角色门控（`:1706–1741`） |
| **E4 图直接显示已修缺陷** | `add-transaction/390x844/normal.png`（画面日期 2026/11/04）与 `pairs/impl/add-transaction__impl__390x844.png`（2026/11/05）在「类型=支出」下「分类」均显示 **「工资」**（收入类）；`0472484` 的 `refillCategories()` 已修此缺陷 → **E4 与 pairs add-transaction 图基准过期，需复采** |
| 成员页（E12）需复核 | `renderMembers` 邀请入口改为按 `myRole` 门控（`0472484`），members 图早于该改动 → **需复核后决定是否复采** |
| 页面级 43 张无 manifest | `capture.js` 未入库（`截图README:14`），**无法给出精确基准 SHA**；只能按 mtime 2026-11-04 推定早于 `0472484` |
| 其余 pairs 页 | 代码路径未被 `0472484` 触及，但基准仍是 `bff6a2b`；复采时应统一刷新为新 HEAD |

**结论**：E4 标「基准过期·需复采」，E12 标「基准过期·需复核」；两者仍是 `有`（图确实呈现该状态），但**不可作为「当前实现」的验收基线**。这是「可用性」轴的降级，不改「存在性」计数。

---

## 5. W8 · offline / conflict 边界取证（只读）

### 5.1 结论（先给判定，再给证据）

1. **Web 端未接同步客户端**：`server/web/dist/app.js`（Web 端唯一脚本，`index.html:149` 仅引此一个）中，`WebSocket` / `ws://` / `wss://` **0 处**，`sync/pull` / `sync/push` / `/sync` **0 处**，`navigator.onLine` / `serviceWorker` / `EventSource` / `BroadcastChannel` / `setInterval` / `offline` 监听 **0 处**，本地写队列原语 `pendingOps` / `offlineQueue` / `retry` / `queue` **0 处**。
2. **服务端已实现同步与实时推送**：`router.go:133` 有 `GET /ws`，`:234` / `:235` 有 `GET /books/:id/sync/pull` 与 `POST /books/:id/sync/push`，`server/internal/ws/hub.go`（1089 行）为 WS 实现。
3. **冲突链路在 Web 侧真实存在，但只覆盖页 5**：`app.js:655` 发送 `baseVersion` → `tx_update.go:130–135` 逐行比对 `SyncVersion` → `common.go:64` 映射 409 → `app.js:670` 追加「（请刷新列表后重试）」。**这是载体存在的正面证据**，与上一版「0 处 `conflict` 字样 = 无冲突载体」的推断相反（F-06/F-13）。
4. **因此**：`offline` 列 12 格判 `N/A`（R07，三要素见 §3.1）；`conflict` 列**只有页 5 判 `缺`**，其余 11 页按各自 409 语义判 `N/A`（R08-xx，见 §5.5）。**不得再写「两列在 Web 侧无运行时载体」。**

### 5.2 取证命令（五段式：命令 / 文件 / 输出 / rc / 排除项）

```text
# G1 前端 WS 客户端
文件：server/web/dist/app.js（2275 行）
命令：grep -n -e "WebSocket" -e "ws://" -e "wss://" server/web/dist/app.js
输出：（无）
rc：1
排除项：无（字面串，无同义词歧义）

# G2 前端 sync 客户端调用
文件：server/web/dist/app.js
命令：grep -n -e "sync/pull" -e "sync/push" -e "/sync" server/web/dist/app.js
输出：（无）
rc：1
排除项：用精确串而非 "sync"。裸查 "sync" 会命中 61 行，其中 57 行是 "async" 的包含匹配
        （`grep -c "sync"` = 61；`grep -n "sync" | grep -v "async"` = 4）；
        排除 "async" 后仅 4 处真匹配：syncFormFields ×3（:556 调用、:572 定义、:578 绑定）
        与 tx.syncVersion（:655）——后者恰恰是版本冲突载体的证据（G10）

# G3 整个 dist 目录
文件：server/web/dist/（app.js、index.html、style.css）
命令：grep -rn -e "WebSocket" -e "ws://" -e "wss://" -e "sync/pull" -e "sync/push" server/web/dist/
输出：（无）
rc：1
排除项：无

# G4 后端同步路由
文件：server/internal/api/router.go（238 行）
命令：grep -n "sync" server/internal/api/router.go
输出：98:      sh := &syncHandler{svc: d.Sync}
      233:     // 增量同步（阶段 3.1 / 3.2）
      234:     auth.GET("/books/:id/sync/pull", sh.pull)
      235:     auth.POST("/books/:id/sync/push", sh.push)
rc：0
排除项：无

# G4b 后端 WS 路由
文件：server/internal/api/router.go
命令：grep -n 'v1.GET("/ws"' server/internal/api/router.go
输出：133:  v1.GET("/ws", wsAuth(d.AuthService, d.PATs), ws.ServeWS)
rc：0
排除项：无

# G5 后端 WS 实现
文件：server/internal/（*.go）
命令：grep -rn -e "websocket" -e "WebSocket" -e "Upgrade" server/internal/ --include=*.go
输出（节选）：ws/hub.go:1 包注释；middleware.go:95 注释；middleware.go:112 func wsAuth；
              config.go:32/:38 WS 连接上限；session_api_test.go:31 gorilla/websocket
rc：0
排除项：无

# G6 离线能力原语
文件：server/web/dist/app.js
命令：grep -n -e "navigator.onLine" -e "serviceWorker" -e "addEventListener(\"offline\"" -e "addEventListener('offline'" -e "EventSource" -e "BroadcastChannel" -e "setInterval" -e "visibilitychange" server/web/dist/app.js
输出：（无）
rc：1
排除项：双引号与单引号两种 offline 写法都查；setInterval 命中 0 说明也不存在轮询式「近实时」

# G7 冲突协商的「协议原语」——注意：这不是冲突判据
文件：server/web/dist/app.js
命令：grep -n -e "If-Match" -e "ETag" -e "etag" -e "X-Sync" -e "conflict" server/web/dist/app.js
输出：（无）
rc：1
排除项：本仓库不用 HTTP 条件请求，冲突走 JSON 字段 baseVersion + 409 + 「并发」文案。
        故 G7 只能证明「未用 If-Match/ETag 族协议」，不能证明「无冲突载体」——
        上一版把 G7 当冲突判据，属方法性漏检（F-13）。冲突判据见 G10。

# G8 取证对象规模
文件：server/web/dist/{app.js,index.html}、server/internal/api/router.go、server/internal/ws/hub.go
命令：wc -l server/web/dist/app.js server/web/dist/index.html server/internal/api/router.go server/internal/ws/hub.go
输出：2275 app.js、151 index.html、238 router.go、1089 ws/hub.go
rc：0
排除项：无

# G9 本地写队列 / 重试原语
文件：server/web/dist/app.js
命令：grep -n -e "pendingOps" -e "offlineQueue" -e "retry" -e "queue" server/web/dist/app.js
输出：（无）
rc：1
排除项：localStorage 仅 9 处，只存 fl_token / fl_whoami，不含业务数据或待发队列

# G10 冲突载体正面证据（本版新增，F-06/F-13）
文件：server/web/dist/app.js
命令：grep -n -e "baseVersion" -e "syncVersion" server/web/dist/app.js
输出：655:      baseVersion: tx.syncVersion || 0,
rc：0
排除项：无

# G11 分类空前置提示（本版新增，F-12）
文件：server/web/dist/app.js
命令：grep -n "（暂无可选分类）" server/web/dist/app.js
输出：568:      fillSelect($("#fCategory"), [], "（暂无可选分类）");
rc：0
排除项：无
```

**Web 端脚本唯一性核对**：

```text
文件：server/web/dist/
命令：find server/web/dist -type f
输出：server/web/dist/app.js、server/web/dist/index.html、server/web/dist/style.css
命令：grep -n "<script" server/web/dist/index.html
输出：149:<script src="/app.js"></script>
```

### 5.3 行号锚点表

| 事实 | 文件:行号 | 原文摘要 |
|---|---|---|
| Web 端唯一脚本引用 | `server/web/dist/index.html:149` | `<script src="/app.js"></script>` |
| WS 路由（不挂 JWT 组） | `server/internal/api/router.go:133` | `v1.GET("/ws", wsAuth(d.AuthService, d.PATs), ws.ServeWS)` |
| WS 鉴权中间件·注释 | `server/internal/api/middleware.go:95` | 「wsAuth WebSocket 专用鉴权中间件：优先读 Authorization: Bearer，回退读 ?token=」 |
| WS 鉴权中间件·函数 | `server/internal/api/middleware.go:112` | `func wsAuth(authSvc *service.AuthService, patSvc *service.PATService) gin.HandlerFunc` |
| WS 实现包 | `server/internal/ws/hub.go:1` | `// Package ws WebSocket 实时推送（阶段 3.3）` |
| 同步 handler 装配 | `server/internal/api/router.go:98` | `sh := &syncHandler{svc: d.Sync}` |
| 增量同步路由 | `server/internal/api/router.go:233–235` | 注释「增量同步（阶段 3.1 / 3.2）」+ pull/push 两路由 |
| **冲突载体（前端）** | `server/web/dist/app.js:655` | `baseVersion: tx.syncVersion`（默认取 0；原文含逻辑或运算符，表格内改写以免破坏列分隔） |
| **冲突载体（前端写调用）** | `server/web/dist/app.js:664` | `await api("PUT", "/books/" + bookId + "/transactions/" + tx.id, body);` |
| **冲突提示（前端）** | `server/web/dist/app.js:670` | `toast(err.message + (…indexOf("并发") >= 0 ? "（请刷新列表后重试）" : ""), "err")` |
| **冲突判定（后端）** | `server/internal/service/tx_update.go:130–135` | `if r.BaseVersion > 0 { … gr.SyncVersion != r.BaseVersion → ErrConcurrentConflict … }` |
| **409 映射** | `server/internal/api/common.go:64` | `case errors.Is(err, service.ErrConcurrentConflict): fail(c, http.StatusConflict, "数据已被并发修改，请重试")` |
| 409 响应体形状 | `server/internal/api/common.go:22–24` | `func fail(...) { c.JSON(status, gin.H{"code": status, "msg": msg}) }`（**无 `errorCode`**） |
| 前端同步客户端 | `server/web/dist/app.js`（2275 行） | **0 处**（G1/G2/G3 实测） |

上一版把 `wsAuth` 锚点写成 `:95–104`，只覆盖注释不覆盖函数（函数在 `:112`）——本版拆两行（N-08）。

### 5.4 判定边界（可与不可推）

| 已核实事实 | 可以推出 | **不可以**推出 |
|---|---|---|
| `app.js` 0 处 `WebSocket` / `ws://` / `wss://` | 当前 Web 端不建立 WS 连接 | 「服务端没有 WS」（`router.go:133`、`hub.go` 均存在） |
| `app.js` 0 处 `sync/pull` / `sync/push` / `setInterval` | 当前 Web 端既不推拉同步，也不轮询 | 「同步功能不存在」（`router.go:234–235` 存在） |
| 前端 0 处 `navigator.onLine` / `serviceWorker` / `EventSource` / 写队列 | 无离线检测、无离线渲染、无离线补传 | 「离线场景已验证通过」，也**不能**推出「离线免实现」（PRD/落地方案有要求，见 §3.1 R07） |
| 前端 0 处 `If-Match` / `ETag` / `conflict` 字样 | 未使用 HTTP 条件请求族协议 | 「无冲突载体」——载体是 `baseVersion` 字段（`app.js:655`） |
| `baseVersion` → `tx_update.go:130–135` → `common.go:64` 409 → `app.js:670` 提示 | 页 5 的编辑路径**存在**版本冲突检测与提示 | 「冲突已被处理」——**无三选一 UI、无 `errorCode`**，前端只有 401 特判（`app.js:104–108`） |

### 5.5 conflict 列逐页 409 语义（12 页 + 页 3）

| 页 | 页面 | 前端写调用 | 携带 `baseVersion` | 409 来源 | 409 语义 | 判定 |
|---|---|---|---|---|---|---|
| 1 | 登录 | `POST /auth/login` `:193`、`:209` | — | 401 为主 | 密码错 / 锁定 | N/A R08-01 |
| 2 | 首页账单 | 无写（只读 `GET /transactions` `:1912`） | — | 无写 | — | N/A R08-02 |
| 3 | 截图识别 | 无路由 | — | — | — | N/A R01 |
| 4 | 手动记账 | `POST /transactions` `:603` | 否（body `:586–595` 无该字段） | `common.go:49` | 同 `clientUuid` 不同内容（幂等防御） | N/A R08-04 |
| 5 | 交易详情/编辑 | `PUT /transactions/:id` `:664` | **是**（`:655`） | `tx_update.go:130–135` → `common.go:64` | **版本冲突（PRD 要求三选一）** | **缺（有版本冲突载体）** |
| 6 | 账户 | `PUT /accounts/:id` `:772` | 否 | `account.go:238–240` → `common.go:64` | 更新命中 0 行（行已删/不存在） | N/A R08-06 |
| 7 | 两级分类 | `PUT /categories/:id` `:2082` | 否 | `category.go:233–235` → `common.go:64` | 更新命中 0 行 | N/A R08-07 |
| 8 | 投资资产 | `PUT /instruments/:id` `:1480`、`PUT /holdings/:id` `:1523` | 否 | `common.go:80` | 该标的下仍有持仓（业务约束） | N/A R08-08 |
| 9 | 预算 | `PUT /budgets/:id` `:1251`、`POST /budgets` `:1257` | 否（表无 `sync_version`，`budget.go:22`） | `common.go:85` | 预算唯一性（总预算/分类预算重复） | N/A R08-09 |
| 10 | 报表 | 无写（`GET` `:862`、`:902`） | — | 无写 | — | N/A R08-10 |
| 11 | 导出 | 无写（`GET /transactions` `:1064`） | — | 无写 | — | N/A R08-11 |
| 12 | 家庭成员 | `POST members/invite` `:1689`、`PUT members/:uid` `:1761`/`:1771`、`DELETE` `:1786` | 否 | `common.go:115` | 成员上限 20（容量类） | N/A R08-12 |
| 13 | 我的设置 | `POST /users/me/tokens` `:1574`、`DELETE` `:1636` | 否 | `common.go:90` | 邮箱已被占用（账号类） | N/A R08-13 |

**同一 `ErrConcurrentConflict` 的两种语义必须区分**：页 5 是**版本比对失败**（`tx_update.go:130–135` 显式比对 `SyncVersion`）；页 6/7 是**更新影响行数为 0**（`account.go:238–240`、`category.go:233–235` 复用同一错误值）。二者在 API 层都回 409，但产品含义不同——把后两者写成「并发冲突」是错的，把页 5 写成「无载体」也是错的。

**实现缺口（只登记，不修）**：

1. `fail()` 响应体只有 `{code, msg}`（`common.go:22–24`），**无 `errorCode`**；`落地方案:933–939` 要求 `CONCURRENT_CONFLICT` 等机器可判枚举，`落地方案:946` 明令 `conflict` 与 `error` 必须区分（否则另存副本会把同一笔钱记两遍）。
2. 前端只有 401 特判（`app.js:104–108`），**无 409 分支**，更无「覆盖 / 放弃 / 另存副本」三选一 UI（`落地方案:366`、PRD 三选一要求）。
3. 缺口编号由 Owner 指定；本文件不新增 R0-E 编号，暂记于 §9 第 4 条。

### 5.6 offline 12 格的三要素写法

`offline` 列 12 格均为 `N/A R07/R0-E-15`，每格三要素相同：

| 要素 | 内容 | 依据 |
|---|---|---|
| ① Web 当前无载体（举证） | 0 处 `navigator.onLine` / `serviceWorker` / `EventSource` / `BroadcastChannel` / `setInterval` / 写队列；`localStorage` 仅存 token 与 whoami | §5.2 G6/G9 |
| ② 产品有要求（事实） | 离线队列 + `clientUuid` 幂等，联网后 push；断线重连先 pull；旧队列回流须比对 `sync_version` | `落地方案:364`、`:365`；`PRD:207`、`PRD:593` |
| ③ 缺口未闭环 | **R0-E-15**（Web 未接 WS/sync）+ **R0-E-25 并入 R0-E-15，不单独计数**；另 `R0-E-12`（状态覆盖缺口）仍开 | `基准冻结:818`；`独立补漏与复核记录:124–126` |

**「N/A」在这里的含义严格限定为「Web 侧当前无可观测载体」，不表示「产品不要求」或「已验收通过」。** 上一版只写①，等于把未实现洗成不适用（F-07 不合规 1）。

### 5.7 两列总格与计划「24 格」口径的对齐（F-08 延续）

| 项 | 数 | 构成 |
|---|---|---|
| `offline` + `conflict` 两列总格 | **26** | **24**（12 个实采页 × 2）+ **2**（页 3 两格） |
| 其中 `N/A` | 25 | offline 12（R07）+ conflict 11（R08-01…R08-13）+ 页 3 的 2 格（随整页 R01，不另行编号） |
| 其中 `缺` | 1 | 页 5 conflict（有版本冲突载体） |

与 T22 计划 W8「24 格（12 页 × 2 状态）」的对齐：计划指的是**12 个实采页**的 24 格，页 3 的 2 格随整页 R01 登记；两列合计 26，恒等成立。本版把「24 + 2」写成可点数的构成，而非散文（上一版 `:143` 的「不单独重复计数」易被误读为漏计）。

---

## 6. 隔离运行时证据（runtime 5 张全部登记）

| 文件 | 状态 | 视口 | 注入方式（`runtime/README` 文件表） | 本轮采信 | 已逐张打开 | 已知缺陷 / 说明 |
|---|---|---|---|---|---|---|
| `01-pending-390x844.png` | pending | 390×844 | 暂停 `GET /books/1/transactions`，不 continue/fail，1.8s 截图（`:65`） | 采信 → E15 | 是 | 三卡渲染 + 4 条骨架 + 无 toast；`transactions total=0` |
| `02-rejected-network-390x844.png` | rejected | 390×844 | `Fetch.failRequest(Failed)` 后 0.9s（`:66`） | 采信 → E3 | 是 | 整页完整 + 3 条骨架残留 + 「Failed to fetch」toast |
| `03-rejected-503-390x844.png` | rejected | 390×844 | `Fetch.fulfillRequest(503, JSON)` 后 0.9s（`:67`） | 采信 → E3 | 是 | 整页完整 + 骨架残留 + 「模拟故障：账单列表服务暂不可用（RO-RV-01 隔离注入）」 |
| `04-pending-1280x800.png` | pending | 1280×800 | 桌面视口持请求（`:68`） | 采信 → E15 | 是 | 三卡渲染 + 3 条骨架 + 无 toast；桌面导航可见 |
| `05-recovered-390x844.png` | 恢复 | 390×844 | `Fetch.disable` 后重新导航，1.8s（`:69`） | 未采信 | 否（见下） | 恢复态；**非** success 动作态（无提交成功/保存成功），不构成 success 格证据 |

**骨架残留即 R0-E-35**：`runtime/README.md:5` 明写「失败态观察到骨架屏未清理（`.skel`=4、`aria-busy=true`）；**缺陷尚未关闭**」。E3 采信 02/03 时必须同时标注该缺陷，不得只贴图不谈残留。

**05 未逐张打开的原因（诚实登记）**：图像查看工具对 05 返回内容去重提示（与请求内另一张视觉相同），无法据此独立确认其视觉细节，故**不据 05 判定任何格**；其状态描述只引 `runtime/README:69`。若后续需要以 05 支撑恢复路径，须重新逐张核对。

**E3 旧图处置（N-01）**：旧版 E3 引用 `home/390x844/error.png`，该图整页空白、仅底部「✕ Failed to fetch」，`runtime/README` 已解释空白由**旧轮泛化拦截** `/api/v1/*` 打断 `loadBookMeta` 等依赖造成，属采集缺陷而非页面 error 表现。本版**弃用该图**，E3 改引 `runtime/02`、`03`；该图不进入任何「有」格证据链（仍留在目录内，未删除）。

---

## 7. 计数核对与验收命令（实际执行）

### 7.1 主矩阵三计数

```bash
awk '/^## 1\./,/^## 2\./' docs/r0-status-matrix-13x8.md | awk -F'|' '
function t(s){gsub(/^[ \t]+|[ \t]+$/,"",s); return s}
/^\| [0-9]+ \|/ { rows++; for(i=4;i<=11;i++){c=t($i); if(c ~ /^N\/A/) na++; else if(c ~ /^有/) has++; else if(c ~ /^缺/) miss++; else o++} }
END {printf "行=%d 有=%d 缺=%d N/A=%d 合计=%d 未分类=%d\n", rows, has, miss, na, has+miss+na, o}'
```

实测输出：

```text
行=13 有=17 缺=51 N/A=36 合计=104 未分类=0
```

### 7.2 N/A 台账行数与理由码分布

```bash
# 行数：只数 §3 表体的编号行
awk '/^## 3\./,/^## 4\./' docs/r0-status-matrix-13x8.md | grep -c '^| [0-9]'
# 理由码分布：先限定 §3 范围，再取表体第 5 字段（理由码列）
awk '/^## 3\./,/^## 4\./' docs/r0-status-matrix-13x8.md | awk -F'|' '/^\| [0-9]+ \|/ {gsub(/^[ \t]+|[ \t]+$/,"",$5); print $5}' | sort | uniq -c
```

实测输出：

```text
36
       8 R01
       1 R02
       1 R03
       1 R04
       1 R05
       1 R06
      12 R07
       1 R08-01
       1 R08-02
       1 R08-04
       1 R08-06
       1 R08-07
       1 R08-08
       1 R08-09
       1 R08-10
       1 R08-11
       1 R08-12
       1 R08-13
```

### 7.3 证据台账行数与张数

```bash
awk '/^## 4\./,/^## 5\./' docs/r0-status-matrix-13x8.md | grep -c '^| E[0-9]'
awk -F'|' '/^\| E[0-9]+ \|/ {gsub(/[^0-9]/,"",$6); s+=$6} END {print "图数合计=" s}' docs/r0-status-matrix-13x8.md
```

实测输出：

```text
17
图数合计=56
```

### 7.4 矩阵列数完整性

```bash
# 只在 §1 范围内数，避免把 §3 的 6 列表也算进来
awk '/^## 1\./,/^## 2\./' docs/r0-status-matrix-13x8.md | awk -F'|' '/^\| [0-9]+ \|/ {print NF}' | sort -u
# 目标：12（10 列 + 首尾空段各 1），且只有一个唯一值
```

实测输出：

```text
12
```

### 7.5 路径可解析断言（E 表逐条）

```bash
awk -F'|' '/^\| E[0-9]+ \|/ {gsub(/^[ \t]+|[ \t]+$/,"",$2); gsub(/^[ \t]+|[ \t]+$/,"",$7); print $2"\t"$7}' docs/r0-status-matrix-13x8.md |
while IFS=$'\t' read -r id paths; do
  paths=$(echo "$paths" | tr -d '`' | sed 's/ + /+/g'); total=0
  IFS='+' read -ra segs <<< "$paths"
  for s in "${segs[@]}"; do eval "arr=($s)"; for x in "${arr[@]}"; do
    [ -f "docs/r0-screenshots/$x" ] && total=$((total+1)) || echo "MISSING: $x"
  done; done
  echo "$id 实际存在=$total"
done
```

实测输出（**无 MISSING 行**）：

```text
E1 实际存在=4
E2 实际存在=4
E3 实际存在=2
E4 实际存在=4
E5 实际存在=4
E6 实际存在=6
E7 实际存在=2
E8 实际存在=2
E9 实际存在=2
E10 实际存在=2
E11 实际存在=2
E12 实际存在=2
E13 实际存在=6
E14 实际存在=4
E15 实际存在=2
E16 实际存在=4
E17 实际存在=4
```

### 7.6 与 T22 计划 §3 预填的逐格比对（改判 5 格）

```bash
norm() { awk -F'|' '/^\| [0-9]+ \|/ {for(i=4;i<=11;i++){c=$i; sub(/^[ \t]+/,"",c); if(c ~ /^N\/A/) printf "NA "; else if(c ~ /^有/) printf "Y "; else printf "M "} printf "\n"}'; }
awk '/^## 3\./,/^## 4\./' docs/r0-t22-execution-plan.md | norm > /tmp/plan.txt
awk '/^## 1\./,/^## 2\./' docs/r0-status-matrix-13x8.md | norm > /tmp/matrix.txt
# 逐格比对（不是逐行 diff）：同一行可能有 2 格改判
paste /tmp/plan.txt /tmp/matrix.txt | awk '{for(i=1;i<=8;i++) if($i!=$(i+8)) {d++; printf "  第%d行 第%d列 %s→%s\n", NR, i, $i, $(i+8)}} END {printf "改判格=%d 一致格=%d\n", d, 104-d}'
```

实测输出：

```text
  第2行 第1列 M→Y
  第2行 第2列 M→Y
  第5行 第8列 NA→M
  第9行 第1列 M→Y
  第10行 第1列 M→Y
改判格=5 一致格=99
```

**「零改判」不再作为一致性证据**：上一版把 `diff` 无输出当成优点，实际是把预填的 4 处漏计一并冻结（N-04）。本版改为「改判 5 格 + 一致 99 格」的显式对账。

### 7.7 视口齐备统计（0 / 17）

```bash
awk '/^## 2\./,/^## 3\./' docs/r0-status-matrix-13x8.md | awk -F'|' '
function t(s){gsub(/^[ \t]+|[ \t]+$/,"",s); return s}
/^\| [0-9]+ \|/ { n++; a=t($5); b=t($6); c=t($7); if(a=="✗") no390++; if(b=="✗") no641++; if(c=="✗") no1280++; if(a=="✓"&&b=="✓"&&c=="✓") full++ }
END {printf "有格=%d 三档齐备=%d 缺390=%d 缺641=%d 缺1280=%d\n", n, full+0, no390+0, no641+0, no1280+0}'
```

实测输出：

```text
有格=17 三档齐备=0 缺390=0 缺641=17 缺1280=1
```

### 7.8 双分母与覆盖率

```bash
# 清点分母 104；适用分母 96 = 104 − 页 3 的 8 格
awk '/^## 1\./,/^## 2\./' docs/r0-status-matrix-13x8.md | awk -F'|' '
function t(s){gsub(/^[ \t]+|[ \t]+$/,"",s); return s}
/^\| [0-9]+ \|/ { if(t($2)!="3"){for(i=4;i<=11;i++){c=t($i); if(c ~ /^N\/A/) na++; else if(c ~ /^有/) has++; else if(c ~ /^缺/) miss++}} }
END {printf "适用格=%d 其中有=%d 缺=%d N/A=%d\n", has+miss+na, has, miss, na}'
```

实测输出：

```text
适用格=96 其中有=17 缺=51 N/A=28
```

**覆盖率（不得写成 17/104）**：存在覆盖 `17 / 96 = 17.7%`；齐备覆盖 `0 / 96 = 0%`。

### 7.9 复核报告 V-10 校验脚本复跑

脚本正文见复核报告 §6 附录（报告在 `review/t22-w1-w8-wb-recheck` 分支的 `docs/r0-t22-b1-wb-delivery-recheck.md`；本仓库工作分支内无该文件，故本轮提取到 `/tmp/v10_check.py` 后执行，未写入 `tools/r0/`——该目录本轮无写权限）：

```bash
python3 /tmp/v10_check.py docs/r0-status-matrix-13x8.md
```

实测输出：

```text
通过项：
  OK  §1 行数=13
  OK  总格=104（有 17 / 缺 51 / N/A 36）
  OK  N/A 台账闭合 36/36
  OK  E 台账一一对应 17 格
  OK  E 台账路径全部可解析
未通过项：
结论：PASS（未通过 0 项）
```

---

## 8. 本轮边界（明确不做）

- 不采集任何截图、不起服务、不造数、不写库、不建隔离实例。
- 不新建或修改采集工具（`capture-states.js` 属 P1，未获批）；不改 `tools/r0/**`。
- 不改业务代码、`server/web/dist/**`、CSS、`controls.json`、fixture、runner、CI。
- 不改统计 `164 PASS / 4 FAIL / 59 BLOCKED / 1 N/A / 228`。
- 不执行 W2–W7、W9；不处理 P1–P6。
- 不修改 T21/T22 已签收文档，不改其他任务状态，不合并 PR，不动 `main`。
- 唯一触碰的非本文件目标是 `docs/r0-screenshots/pairs/README.md` 的**事实声明更正**（F-05 要求，经本轮指令授权「必要的 pairs README 事实声明」）。
- 不宣称第 3 条或 R0 PASS。

---

## 9. 诚实清单

1. **矩阵是现状快照，不是覆盖证明**：51 格 `缺` + 齐备 0 说明第 3 条仍远未达；本文件只把「缺在哪、缺多少」变成可点数的事实。
2. **`有` 只表示「存在呈现该状态的图」**，不表示验收通过、不表示与原型一致、不表示三档齐备。三轴分离见 §0.2。
3. **页 13 未拆行**：`13a`/`13b` 拆分会把总格从 104 变 112，属结构变更需 Owner 同意；本轮只冻结判据（两视图均需覆盖才算齐备），未改格数。若 Owner 同意拆行，须同步重算三计数与 §7 全部脚本。
4. **409 无 `errorCode` 缺口未挂 R0-E 编号**：本文件不自造编号（复核报告亦未新增），暂记为「待 Owner 指定」；`落地方案:933–939`、`:946` 是判据来源。
5. **E4/E12 基准过期但未复采**：本轮无采集授权。E4 图直接显示 `0472484` 已修的「支出类型下分类=工资」缺陷；E12 受邀请入口角色门控改动影响。两者仍计 `有`（图确实呈现该状态），但不可作为当前实现的验收基线。
6. **页面级 43 张无 manifest，基准 SHA 不可精确**：`capture.js` 未入库，只能按 mtime 推定早于 `0472484`；这是 F-05 只能部分闭合的原因，不是遗漏。
7. **`05-recovered` 未逐张打开**：图像工具返回去重提示，未据此判任何格，也未用它支撑 success 格。
8. **未逐张打开全部 56 张证据**：本轮实际打开 **16 张**——`home/390x844/empty.png`（E2）、`home/390x844/error.png`（旧 E3，已弃用）、`runtime/01`、`02`、`03`、`04`（E3/E15）、`add-transaction/390x844/normal.png`（E4）、`accounts/390x844/normal.png`（E6）、`categories/390x844/normal.png`（E7）、`budget/390x844/empty.png`（E9）、`pairs/impl/{add-transaction,home,budget,report,invest,members}__impl__390x844.png`（E4/E12/E14/E16/E17 及投资待冻结项）；其余按「路径可解析 + 目录名/README 定义 + 采集批次」采信，**未逐张视觉复核**。逐张视觉复核仍属第 3 条后续工作。
9. **未运行任何测试**：统计数字为只读引用，未跑 C 套件、未起服务、未做浏览器实测。
10. **非原型三页（books/transactions/recycle）未入矩阵**：`books` 2 张、`recycle` 2 张、`transactions` 2 张合计 6 张仅用于 §4 张数对账，不计入 104 格；三页归属待 P2 裁决。
11. **`pairs/README.md` 的哈希在 `SHA256SUMS.txt` 中已过期**：本轮更正了该 README 的「零改动」事实声明，导致归档清单第 108 行失配（原 `c329854f…`，实测现为 `eda61aee70a6d8a51bf2bce558e9b00a7e6869f2b862492af95079f4b051539c`）。**未改写 `SHA256SUMS*.txt`**（属既有归档，不擅自重签）；剔除该行后 108 条证据仍全部通过校验（`sha256sum -c` rc=0），验证命令已写入 `pairs/README.md`。README 无法内嵌自身最终哈希（自指），故完整值登记在此处。

---

## 10. 复核意见处理对照（F-01…F-14 / N-01…N-10）

| 编号 | 复核结论 | 本轮处理 | 落点 | 状态 |
|---|---|---|---|---|
| F-01 | 首页/预算/报表 normal 漏计 | 三格改 `有`，引 pairs 路径；新增 §0.3 采信规则与「数据前置」列 | §1 第 2/9/10 行；§1.1；§0.3；§4 E14/E16/E17 | 已采纳 |
| F-02 | 首页 loading 漏计 | 改 `有`，引 `runtime/01`、`04`，注明隔离运行时、单次实测、缺 641 | §1 第 2 行；§4 E15；§6 | 已采纳 |
| F-03 | 「有」兼表存在性与齐备 | 三轴分离；新增 §2 视口齐备度表（390/641/1280 三列 + 齐备列） | §0.2；§2；§7.7 | 已采纳（选「拆两列」方案） |
| F-04 | 「有」判据不一致 + 缺档列有误 | 冻结采信规则；缺档列区分「无图/数据不同/视口缺」；投资列入待冻结并给混合态理由 | §0.3；§2.1；§4 缺口列 | 已采纳（判据层） |
| F-05 | pairs 基准漂移未标 | §4 增加「代码基准」列；E4 标「需复采」、E12 标「需复核」；`pairs/README` 更正「零改动」 | §4；§4.2；`pairs/README.md` §1 | 已采纳（部分闭合，见 §9 第 6 条） |
| F-06 | conflict 整列 N/A 不成立 | 仅页 5 改 `缺（有版本冲突载体）`；其余 11 页逐页 409 语义；R08 不再用统一理由 | §1 第 5 行；§3.1；§5.5 | 已采纳（范围按报告收窄） |
| F-07 | R07 未登记产品要求与缺口 | R07 定义写全三要素；每格带 `R0-E-15`；R08 拆 R08-01…R08-13 | §3.1；§3.2；§5.6 | 已采纳 |
| F-08 | 26 = 24 + 2 已写明 | 新增 §5.7 用表给构成：两列 26 = 24（12 实采页 × 2）+ 2（页 3）；其中 N/A 25、缺 1（页 5） | §5.7 | 已完成（并补可点数构成） |
| F-09 | 104/96 双分母与齐备统计缺失 | §0.1 给双分母；§7.7 齐备脚本；§7.8 双分母脚本；覆盖率只按 96 计 | §0.1；§2；§7.7；§7.8 | 已采纳 |
| F-10 | 页 13 合并不清 | 冻结判据「有按任一视图、齐备按两视图均覆盖」；641 覆盖对象 = mine + settings 各 1 张；不拆行（需 Owner 同意） | §4.1；§2 第 13 行 | 已采纳（选选项 B） |
| F-11 | 导出 empty 无判据 | 保持 `缺` + 写明判据（点预览后断言文本 `app.js:1066`），并列入 §2.1 待冻结（候选 `N/A`） | §2.1 第 3 行 | 部分采纳（判据已写，改判待冻结） |
| F-12 | 手动记账 empty 的 R04 未覆盖空前置提示 | R04 补 `app.js:568`「（暂无可选分类）」，口径列入待冻结 | §3.1 R04；§3.2 第 15 行；§2.1 | 已采纳（判据层） |
| F-13 | grep 方法性漏检 | 证据块五段化（命令/文件/输出/rc/排除项）；冲突判据改「字段+409+前端处理」；新增 G10 `baseVersion` 正面证据；补 `syncFormFields` 排除说明 | §5.2 G1–G11 | 已采纳 |
| F-14 | 「有」格缺基准/数据前置/动作 | §4 增 5 列：代码基准、数据前置、状态与动作、缺口、子页归属；E6/E13 子页在 §4.1 拆明 | §4；§4.1 | 已采纳 |
| N-01 | E3 是泛化拦截产物，不能当 error 证据 | E3 改引 `runtime/02`、`03`，旧图弃用并说明成因；标注骨架未清理（R0-E-35 未关闭） | §4 E3；§6 | 已采纳 |
| N-02 | runtime 5 张全篇未引用 | 新增 §6 全表（5 张：状态/视口/注入方式/采信/已打开/缺陷），E3/E15 分别引用 | §6 | 已采纳 |
| N-03 | R08「无冲突载体」与代码相反 | 删除该统一理由；§2.4 表改为「冲突有 409 提示、无三选一、无 errorCode」；新增 §5.5 实现缺口 | §5.4；§5.5 | 已采纳 |
| N-04 | 零改判被当成优点 | §1.1 改为显式改判 5 格 + 一致 99 格；§7.6 逐格比对脚本 | §1.1；§7.6 | 已采纳 |
| N-05 | N/A 台账「同上」当理由 | R07 定义写全三要素；R08 拆 11 个分页码，逐行给 409 语义 | §3.1；§3.2 | 已采纳 |
| N-06 | 成员 empty 判「缺」不成立 | **本轮不采纳改判**。理由：① `pairs/README` 与实现均显示账本必有 owner（`renderMembers:1667`），空列表不可达成立；② 但改 `N/A` 会把计数移为 17/50/37，与用户明确要求保留的底稿 17/51/36 冲突；③ 报告 §5.3 本身也把它列为「待冻结」。故维持 `缺` 并列入 §2.1，待 Owner 一次冻结 | §2.1 第 5 行 | 不采纳（改判）/ 已登记待冻结 |
| N-07 | E6/E13 单格混装子页 | §4 增「子页归属」列；§4.1 给子页/视图张数与齐备判定 | §4；§4.1 | 已采纳 |
| N-08 | `wsAuth` 锚点 95 vs 112 | 拆两行：`:95` 注释、`:112` 函数 | §5.3 | 已采纳 |
| N-09 | `:5` 冻结与 `:373` 可降级矛盾 | 删除「若要求三档齐备则 E3 降为缺」的可变表述；E3 仍记 `有`（`runtime/02`、`03` 两张，仅 390 档），641/1280 缺口在 §2 单列，不再与判定符号互相牵动 | 文首结论边界；§2 第 3 行 | 已采纳 |
| N-10 | 简写无路径映射 | 新增 §0.4 引用文件映射表（11 条） | §0.4 | 已采纳 |

**唯一不采纳项**：N-06（成员 empty 改 N/A）。理由如上，且与报告 §5.3 自身「待冻结」清单重叠；本轮按保守口径维持 `缺`，未擅自移动统计基线。如 Owner 裁定改 N/A，计数将变为 `有 17 / 缺 50 / N/A 37`，需同步更新 §7 全部脚本输出与 §2.1。
