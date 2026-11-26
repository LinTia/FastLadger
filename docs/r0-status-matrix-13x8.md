# R0-T22 · 13 页 × 8 状态矩阵（批次 1 · W1 骨架冻结 + W8 边界取证）

> 任务编号：R0-T22（九条第 3 条）。文档性质：**只读核查 + 矩阵/证据文档**。不改业务代码、CSS、`app.js`、测试、`controls.json`、fixture、runner、CI、统计；不采集截图、不起服务、不造数、不写库。
> 基线：`work/r0-t22-plan` @ `0033a1899878c2b832578cbfbb11ab858c2f8649`（T22 执行计划已签收）。本文件所在分支：`work/r0-t22-batch1`。
> 结论边界：本矩阵只冻结**现状**。104 格中 `缺` 54、`N/A` 37，第 3 条远未达；本文件**不构成第 3 条 PASS，也不构成 R0 PASS**，R0 维持 BLOCK。

---

## 0. 冻结口径

| 项 | 口径 | 来源 |
|---|---|---|
| 页数 | 13 页（T21 台账 13 行），其中页 3 为整页 `N/A` | `r0-page-mapping-13.md:31–43` |
| 状态 | 8 个：`normal` / `loading` / `empty` / `error` / `success` / `permission` / `offline` / `conflict` | T21 §4 第 7 条（`r0-page-mapping-13.md:55`）；T22 计划 §3 |
| 格数 | 13 × 8 = **104** | 同上 |
| 判定符号 | `有` = 已有可用图（路径见 §4）；`缺` = 需采集，当前无图；`N/A` = 不适用，**逐格给理由**（见 §3） | T22 计划 §2.1 W1 |
| 单元格记法 | 判定 + 证据号（`E#`）或理由码（`R#`）；路径、视口、理由分别在 §4 / §3 | 本文件 |
| 强制视口 | 390 / 641 / 1280 三档；360 / 430 为可选补充档 | T22 计划 §3 列定义 |
| 非原型扩展页 | `books` / `transactions` / `recycle` 三页无原型路由，**不入 104 格**（P2 待裁决） | T22 计划 §3 补充、§6 P2 |

**禁止的写法**：不得把 `缺` 写成「已实现 / 待验 / PASS」；不得把 `N/A` 写成「无需验证」；不得以「文档齐全」「截图张数达标」代替逐格判定。

---

## 1. 主矩阵（13 行 × 10 列，本版冻结）

| # | 页面 | normal | loading | empty | error | success | permission | offline | conflict |
|---|---|---|---|---|---|---|---|---|---|
| 1 | 登录 | 有 E1 | 缺 | N/A R02 | 缺 | 缺 | N/A R03 | N/A R07 | N/A R08 |
| 2 | 首页账单 | 缺 | 缺 | 有 E2 | 有 E3 | 缺 | 缺 | N/A R07 | N/A R08 |
| 3 | 截图识别记账 | N/A R01 | N/A R01 | N/A R01 | N/A R01 | N/A R01 | N/A R01 | N/A R01 | N/A R01 |
| 4 | 手动记账/转账 | 有 E4 | 缺 | N/A R04 | 缺 | 缺 | 缺 | N/A R07 | N/A R08 |
| 5 | 交易详情 | 有 E5 | 缺 | N/A R05 | 缺 | 缺 | 缺 | N/A R07 | N/A R08 |
| 6 | 账户 | 有 E6 | 缺 | 缺 | 缺 | 缺 | 缺 | N/A R07 | N/A R08 |
| 7 | 两级分类 | 有 E7 | 缺 | 缺 | 缺 | 缺 | 缺 | N/A R07 | N/A R08 |
| 8 | 投资资产 | 缺 | 缺 | 有 E8 | 缺 | 缺 | 缺 | N/A R07 | N/A R08 |
| 9 | 预算 | 缺 | 缺 | 有 E9 | 缺 | 缺 | 缺 | N/A R07 | N/A R08 |
| 10 | 报表 | 缺 | 缺 | 有 E10 | 缺 | 缺 | 缺 | N/A R07 | N/A R08 |
| 11 | 导出 | 有 E11 | 缺 | 缺 | 缺 | 缺 | 缺 | N/A R07 | N/A R08 |
| 12 | 家庭成员 | 有 E12 | 缺 | 缺 | 缺 | 缺 | 缺 | N/A R07 | N/A R08 |
| 13 | 我的设置 | 有 E13 | 缺 | N/A R06 | 缺 | 缺 | 缺 | N/A R07 | N/A R08 |

**计数**：104 格 = **有 13** + **缺 54** + **N/A 37**（脚本核对见 §5.1，非人工累加）。

**与 T22 计划 §3 预填的一致性**：逐格比对一致，无改判（归一后 `diff` 无输出，见 §5.5）。**本表是冻结版**；后续批次实采若与预填不符，改本矩阵并登记改判理由，不回改 T22 计划。

---

## 2. W8 · offline / conflict 边界取证（只读）

### 2.1 结论（先给判定，再给证据）

1. **Web 端未接同步客户端**：`server/web/dist/app.js`（Web 端唯一脚本，`index.html:149` 仅引此一个）中，`WebSocket` / `ws://` / `wss://` **0 处**，`sync/pull` / `sync/push` / `/sync` **0 处**，`navigator.onLine` / `serviceWorker` / `offline` 监听 **0 处**，冲突协商原语 `If-Match` / `ETag` / `X-Sync` / `conflict` **0 处**，本地写队列原语 `pendingOps` / `offlineQueue` / `retry` / `queue` **0 处**。
2. **服务端已实现同步与实时推送**：`router.go:133` 有 `GET /ws`，`router.go:234` / `:235` 有 `GET /books/:id/sync/pull` 与 `POST /books/:id/sync/push`，`server/internal/ws/hub.go`（1089 行）为 WS 实现。
3. **因此**：`offline` / `conflict` 两列在 Web 侧**无运行时载体**，判 `N/A` 并附边界理由（R07 / R08）。**这两件事必须分开写**——「Web 未接客户端」既不能推出「功能 PASS」，也不能推出「功能不存在」。

### 2.2 取证命令与实测输出（`rc=1` 表示 grep 无命中）

```bash
# G1 前端 WS 客户端
grep -n -e "WebSocket" -e "ws://" -e "wss://" server/web/dist/app.js
# 实测：无输出，rc=1（0 命中）

# G2 前端 sync 客户端调用
grep -n -e "sync/pull" -e "sync/push" -e "/sync" server/web/dist/app.js
# 实测：无输出，rc=1（0 命中）

# G3 整个 dist 目录（app.js / index.html / style.css 三个文件）
grep -rn -e "WebSocket" -e "ws://" -e "wss://" -e "sync/pull" -e "sync/push" server/web/dist/
# 实测：无输出，rc=1（0 命中）

# G4 后端同步路由
grep -n "sync" server/internal/api/router.go
# 实测：
#   98:      sh := &syncHandler{svc: d.Sync}
#   233:     // 增量同步（阶段 3.1 / 3.2）
#   234:     auth.GET("/books/:id/sync/pull", sh.pull)
#   235:     auth.POST("/books/:id/sync/push", sh.push)

# G4b 后端 WS 路由（精确行）
grep -n 'v1.GET("/ws"' server/internal/api/router.go
# 实测：133:  v1.GET("/ws", wsAuth(d.AuthService, d.PATs), ws.ServeWS)

# G5 后端 WS 实现存在性
grep -rn -e "websocket" -e "WebSocket" -e "Upgrade" server/internal/ --include=*.go
# 实测（节选）：middleware.go:95 wsAuth 注释；ws/hub.go:1 包注释；
#   session_api_test.go:31 gorilla/websocket 引用；config.go:32/:38 WS 连接上限

# G6 离线能力原语（两种引号写法都已查，避免漏 'offline' 单引号情形）
grep -n -e "navigator.onLine" -e "serviceWorker" -e "addEventListener(\"offline\"" -e "addEventListener('offline'" server/web/dist/app.js
# 实测：无输出，rc=1（0 命中）

# G7 冲突协商原语
grep -n -e "If-Match" -e "ETag" -e "etag" -e "X-Sync" -e "conflict" server/web/dist/app.js
# 实测：无输出，rc=1（0 命中）

# G8 取证对象规模（确认 app.js 是完整单文件，不是分片加载）
wc -l server/web/dist/app.js server/web/dist/index.html server/internal/api/router.go server/internal/ws/hub.go
# 实测：2275 app.js、151 index.html、238 router.go、1089 ws/hub.go

# G9 本地写队列 / 重试原语
grep -n -e "pendingOps" -e "offlineQueue" -e "retry" -e "queue" server/web/dist/app.js
# 实测：无输出，rc=1（0 命中）
```

**Web 端脚本唯一性核对**（防止「只在 app.js 里查」的漏检）：

```bash
find server/web/dist -type f
# 实测：server/web/dist/app.js、server/web/dist/index.html、server/web/dist/style.css
grep -n "<script" server/web/dist/index.html
# 实测：149:<script src="/app.js"></script>
```

### 2.3 行号锚点表

| 事实 | 文件:行号 | 原文摘要 |
|---|---|---|
| Web 端唯一脚本引用 | `server/web/dist/index.html:149` | `<script src="/app.js"></script>` |
| WS 路由（不挂 JWT 组） | `server/internal/api/router.go:133` | `v1.GET("/ws", wsAuth(d.AuthService, d.PATs), ws.ServeWS)` |
| WS 鉴权专用中间件 | `server/internal/api/middleware.go:95–104` | 「wsAuth … 优先读 Authorization: Bearer，回退读 ?token=」 |
| WS 实现包 | `server/internal/ws/hub.go:1` | `// Package ws WebSocket 实时推送（阶段 3.3）` |
| 同步 handler 装配 | `server/internal/api/router.go:98` | `sh := &syncHandler{svc: d.Sync}` |
| 增量同步路由 | `server/internal/api/router.go:233–235` | 注释「增量同步（阶段 3.1 / 3.2）」+ pull/push 两路由 |
| 前端同步客户端 | `server/web/dist/app.js`（2275 行） | **0 处**（G1/G2/G3 实测） |

### 2.4 判定边界（可与不可推）

| 已核实事实 | 可以推出 | **不可以**推出 |
|---|---|---|
| `app.js` 0 处 `WebSocket` / `ws://` / `wss://` | 当前 Web 端不建立 WS 连接 | 「服务端没有 WS」（`router.go:133`、`ws/hub.go` 均存在） |
| `app.js` 0 处 `sync/pull` / `sync/push` | 当前 Web 端不触发增量同步 | 「同步功能不存在」（`router.go:234–235` 存在） |
| 前端 0 处 `navigator.onLine` / `serviceWorker` / `offline` 监听 | 无离线检测与离线渲染分支 | 「离线场景已验证通过」 |
| 前端 0 处 `If-Match` / `ETag` / 本地写队列 | 无版本协商与待发写队列 | 「冲突已被处理」 |

**口径**：`N/A` 是本轮对「Web 侧无载体」的**举证结论**，不是对服务端能力的评价，也不是 PASS。

### 2.5 两列 N/A 的填法（与计划「24 格」对齐）

`offline` / `conflict` 两列共 **26 格** `N/A`：

- **24 格**属 12 个实采页（页 1、2、4–13），即 `R07 × 12` + `R08 × 12`，由 W8 直接填写边界理由；
- **2 格**属页 3（截图识别记账），并入其整页 `N/A`（R01）；理由同源（Web 无该页路由），不单独重复计数。

故与 T22 计划 W8「24 格（12 页 × 2 状态）」一致；26 = 24 + 页 3 的 2 格。

---

## 3. N/A 理由台账（37 格逐格，无空格无概括）

| # | 页面 | 状态 | 理由码 | 理由（为什么不适用） | 依据 |
|---|---|---|---|---|---|
| 1 | 登录 | empty | R02 | 登录为表单页（用户名/密码），无列表或集合形态，不存在空态语义 | `r0-page-mapping-13.md:31`；`app.js:161` renderLogin |
| 2 | 登录 | permission | R03 | 未登录前不存在角色/权限上下文，权限态无判定主体 | `r0-page-mapping-13.md:31`；`app.js:161` |
| 3 | 登录 | offline | R07 | Web 未接 WS/sync 客户端、无离线检测，无离线载体（W8） | §2.2 G1/G2/G6/G9 |
| 4 | 登录 | conflict | R08 | 无同步客户端、无版本协商、无本地写队列，无冲突载体（W8） | §2.2 G2/G7/G9 |
| 5 | 首页账单 | offline | R07 | 同上（Web 无离线载体） | §2.2 G1/G2/G6/G9 |
| 6 | 首页账单 | conflict | R08 | 同上（Web 无冲突载体） | §2.2 G2/G7/G9 |
| 7 | 截图识别记账 | normal | R01 | Web 无该页路由、无 OCR 调用，页面不存在，8 状态全不适用 | `r0-page-mapping-13.md:33`；`PRD:326`；`PRD:333`；`基准冻结:86`；`基准冻结:127` |
| 8 | 截图识别记账 | loading | R01 | 同上 | 同上 |
| 9 | 截图识别记账 | empty | R01 | 同上 | 同上 |
| 10 | 截图识别记账 | error | R01 | 同上 | 同上 |
| 11 | 截图识别记账 | success | R01 | 同上 | 同上 |
| 12 | 截图识别记账 | permission | R01 | 同上 | 同上 |
| 13 | 截图识别记账 | offline | R01 | 同上（页不存在，离线态亦无从谈起） | 同上 |
| 14 | 截图识别记账 | conflict | R01 | 同上（页不存在，冲突态亦无从谈起） | 同上 |
| 15 | 手动记账/转账 | empty | R04 | 手动记账为 `#formDlg` 弹层表单，无列表空态 | `r0-page-mapping-13.md:34`；`app.js:548` |
| 16 | 手动记账/转账 | offline | R07 | Web 无离线载体（W8） | §2.2 G1/G2/G6/G9 |
| 17 | 手动记账/转账 | conflict | R08 | Web 无冲突载体（W8） | §2.2 G2/G7/G9 |
| 18 | 交易详情 | empty | R05 | 详情页必先有交易记录才进入，无「空详情」形态 | `r0-page-mapping-13.md:35`；`app.js:2008` |
| 19 | 交易详情 | offline | R07 | Web 无离线载体（W8） | §2.2 G1/G2/G6/G9 |
| 20 | 交易详情 | conflict | R08 | Web 无冲突载体（W8） | §2.2 G2/G7/G9 |
| 21 | 账户 | offline | R07 | Web 无离线载体（W8） | §2.2 G1/G2/G6/G9 |
| 22 | 账户 | conflict | R08 | Web 无冲突载体（W8） | §2.2 G2/G7/G9 |
| 23 | 两级分类 | offline | R07 | Web 无离线载体（W8） | §2.2 G1/G2/G6/G9 |
| 24 | 两级分类 | conflict | R08 | Web 无冲突载体（W8） | §2.2 G2/G7/G9 |
| 25 | 投资资产 | offline | R07 | Web 无离线载体（W8） | §2.2 G1/G2/G6/G9 |
| 26 | 投资资产 | conflict | R08 | Web 无冲突载体（W8） | §2.2 G2/G7/G9 |
| 27 | 预算 | offline | R07 | Web 无离线载体（W8） | §2.2 G1/G2/G6/G9 |
| 28 | 预算 | conflict | R08 | Web 无冲突载体（W8） | §2.2 G2/G7/G9 |
| 29 | 报表 | offline | R07 | Web 无离线载体（W8） | §2.2 G1/G2/G6/G9 |
| 30 | 报表 | conflict | R08 | Web 无冲突载体（W8） | §2.2 G2/G7/G9 |
| 31 | 导出 | offline | R07 | Web 无离线载体（W8） | §2.2 G1/G2/G6/G9 |
| 32 | 导出 | conflict | R08 | Web 无冲突载体（W8） | §2.2 G2/G7/G9 |
| 33 | 家庭成员 | offline | R07 | Web 无离线载体（W8） | §2.2 G1/G2/G6/G9 |
| 34 | 家庭成员 | conflict | R08 | Web 无冲突载体（W8） | §2.2 G2/G7/G9 |
| 35 | 我的设置 | empty | R06 | 设置页为固定表单（改密/备份等区块），无空态 | `r0-page-mapping-13.md:43`；`app.js:1549` |
| 36 | 我的设置 | offline | R07 | Web 无离线载体（W8） | §2.2 G1/G2/G6/G9 |
| 37 | 我的设置 | conflict | R08 | Web 无冲突载体（W8） | §2.2 G2/G7/G9 |

**理由码分布（脚本核对见 §5.2）**：R01 × 8 + R02 × 1 + R03 × 1 + R04 × 1 + R05 × 1 + R06 × 1 + R07 × 12 + R08 × 12 = **37**。

---

## 4. 证据路径台账（13 个「有」格，共 41 张）

| 编号 | 页面 | 状态 | 视口（已有） | 张数 | 证据路径（相对 `docs/r0-screenshots/`） | 缺档 |
|---|---|---|---|---|---|---|
| E1 | 登录 | normal | 360 / 390 / 430 / 1280 | 4 | `login/{360x800,390x844,430x932,1280x800}/normal.png` | 641 |
| E2 | 首页账单 | empty | 360 / 390 / 430 / 1280 | 4 | `home/{360x800,390x844,430x932,1280x800}/empty.png` | 641 |
| E3 | 首页账单 | error | 390 | 1 | `home/390x844/error.png` | 641、1280 |
| E4 | 手动记账/转账 | normal | 360 / 390 / 430 / 1280 | 4 | `add-transaction/{360x800,390x844,430x932,1280x800}/normal.png` | 641 |
| E5 | 交易详情 | normal | 360 / 390 / 430 / 1280 | 4 | `pairs/impl/transaction-detail__impl__{360x800,390x844,430x932,1280x800}.png` | 641 |
| E6 | 账户 | normal | 360 / 390 / 430 / 1280 | 6 | `accounts/{360x800,390x844,430x932,1280x800}/normal.png` + `account-detail/{390x844,1280x800}/normal.png` | 641 |
| E7 | 两级分类 | normal | 390 / 1280 | 2 | `categories/{390x844,1280x800}/normal.png` | 360、430、641 |
| E8 | 投资资产 | empty | 390 / 1280 | 2 | `invest/{390x844,1280x800}/empty.png` | 360、430、641 |
| E9 | 预算 | empty | 390 / 1280 | 2 | `budget/{390x844,1280x800}/empty.png` | 360、430、641 |
| E10 | 报表 | empty | 390 / 1280 | 2 | `report/{390x844,1280x800}/empty.png` | 360、430、641 |
| E11 | 导出 | normal | 390 / 1280 | 2 | `export/{390x844,1280x800}/normal.png` | 360、430、641 |
| E12 | 家庭成员 | normal | 390 / 1280 | 2 | `members/{390x844,1280x800}/normal.png` | 360、430、641 |
| E13 | 我的设置 | normal | 360 / 390 / 430 / 1280 | 6 | `mine/{390x844,1280x800}/normal.png` + `settings/{360x800,390x844,430x932,1280x800}/normal.png` | 641 |

**张数核对**：4+4+1+4+4+6+2+2+2+2+2+2+6 = **41**；加非原型扩展页 `books/{390x844,1280x800}/normal.png` 2 张 = **43**，与 `docs/r0-screenshots/` 页面目录实测 43 张一致（§5.3）。

**641 列口径**：13 个「有」格**没有一格**具备 641 档。641 是 `style.css:327`（`max-width:640px`）与 `:334`（641–768）断点外侧，属 W2 从零采集范围，本轮不采。

---

## 5. 计数核对与验收命令（实际执行）

### 5.1 主矩阵三计数

```bash
awk '/^## 1\./,/^## 2\./' docs/r0-status-matrix-13x8.md | awk -F'|' '
function t(s){gsub(/^[ \t]+|[ \t]+$/,"",s); return s}
/^\| [0-9]+ \|/ { rows++; for(i=4;i<=11;i++){c=t($i); if(c ~ /^N\/A/) na++; else if(c ~ /^有/) has++; else if(c ~ /^缺/) miss++; else o++} }
END {printf "行=%d 有=%d 缺=%d N/A=%d 合计=%d 未分类=%d\n", rows, has, miss, na, has+miss+na, o}'
```

实测输出：

```text
行=13 有=13 缺=54 N/A=37 合计=104 未分类=0
```

### 5.2 N/A 台账行数与理由码分布

```bash
# 行数：只数 §3 表体的编号行
awk '/^## 3\./,/^## 4\./' docs/r0-status-matrix-13x8.md | grep -c '^| [0-9]'
# 理由码分布：先限定 §3 范围，再取表体第 5 字段（理由码列），
# 避免把 §1 的单元格与 §3 末的分布说明行算进去
awk '/^## 3\./,/^## 4\./' docs/r0-status-matrix-13x8.md | awk -F'|' '/^\| [0-9]+ \|/ {gsub(/^[ \t]+|[ \t]+$/,"",$5); print $5}' | sort | uniq -c
```

实测输出：

```text
37
      8 R01
      1 R02
      1 R03
      1 R04
      1 R05
      1 R06
     12 R07
     12 R08
```

### 5.3 证据台账行数与张数

```bash
awk '/^## 4\./,/^## 5\./' docs/r0-status-matrix-13x8.md | grep -c '^| E[0-9]'
awk -F'|' '/^\| E[0-9]+ \|/ {gsub(/[^0-9]/,"",$6); s+=$6} END {print "图数合计=" s}' docs/r0-status-matrix-13x8.md
find docs/r0-screenshots -name '*.png' -not -path '*/pairs/*' -not -path '*/runtime/*' | wc -l
```

实测输出：

```text
13
图数合计=41
43
```

**路径可解析断言**（防止「路径写错但张数算对」）：把 E 表路径逐条做花括号展开并 `test -f`：

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
E3 实际存在=1
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
```

### 5.4 矩阵列数完整性

```bash
# 只在 §1 范围内数，避免把 §3 的 6 列表也算进来
awk '/^## 1\./,/^## 2\./' docs/r0-status-matrix-13x8.md | awk -F'|' '/^\| [0-9]+ \|/ {print NF}' | sort -u
# 目标：12（10 列 + 首尾空段各 1），且只有一个唯一值
```

实测输出：

```text
12
```

### 5.5 与 T22 计划 §3 预填的逐格比对

把两处表格的 104 个判定归一成 `有 / 缺 / N/A` 再 `diff`，避免人工逐格看漏：

```bash
# 归一：只取 8 个状态列的首字判定，输出 Y（有）/ M（缺）/ NA
norm() { awk -F'|' '/^\| [0-9]+ \|/ {for(i=4;i<=11;i++){c=$i; sub(/^[ \t]+/,"",c); if(c ~ /^N\/A/) printf "NA "; else if(c ~ /^有/) printf "Y "; else printf "M "} printf "\n"}'; }

awk '/^## 3\./,/^## 4\./' docs/r0-t22-execution-plan.md | norm > /tmp/plan.txt
awk '/^## 1\./,/^## 2\./' docs/r0-status-matrix-13x8.md | norm > /tmp/matrix.txt
diff /tmp/plan.txt /tmp/matrix.txt
awk '{for(i=1;i<=NF;i++) c[$i]++} END {printf "矩阵: Y=%d M=%d NA=%d\n", c["Y"], c["M"], c["NA"]}' /tmp/matrix.txt
```

实测输出：

```text
（diff 无输出，rc=0 → 104 格逐格一致）
矩阵: Y=13 M=54 NA=37
```

结论：本版矩阵与 T22 计划 §3 预填**零改判**。后续批次若实采推翻某格，须在本文件登记改判（原判 → 新判 + 理由 + 证据），不回改计划。

### 5.6 三组数字互证

| 组 | 关系 | 实测 |
|---|---|---|
| 主矩阵 | 13 + 54 + 37 | 104 |
| N/A 台账 | 8 + 1 + 1 + 1 + 1 + 1 + 12 + 12 | 37 |
| 证据台账 | 41 张 + `books` 2 张 | 43（= 目录实测） |

三组互相自洽；§5 命令均在本轮 Git Bash 下实际执行，输出为原文粘贴，非预期值。

---

## 6. 本轮边界（明确不做）

- 不采集任何截图、不起服务、不造数、不写库、不建隔离实例。
- 不新建或修改采集工具（`capture-states.js` 属 P1，未获批）；不改 `tools/r0/**`。
- 不改业务代码、`server/web/dist/**`、CSS、`controls.json`、fixture、runner、CI。
- 不改统计 `164 PASS / 4 FAIL / 59 BLOCKED / 1 N/A / 228`。
- 不执行 W2–W7、W9；不处理 P1–P6。
- 不修改 T21/T22 已签收文档，不改其他任务状态，不合并 PR，不动 `main`。
- 不宣称第 3 条或 R0 PASS。

---

## 7. 诚实清单

1. **矩阵是现状快照，不是覆盖证明**：54 格 `缺` 意味着第 3 条仍远未达；本文件只把「缺在哪、缺多少」变成可点数的事实。
2. **`N/A` 的依据是「Web 无载体」**：服务端 `/ws` 与 `/sync/pull|push` 真实存在（`router.go:133`、`:234–235`）。因此本文件既不能证明同步功能 PASS，也不能证明其有缺陷——只证明 Web 客户端未接。
3. **E3 只按「至少有 1 张可用图」记为 `有`**：首页 error 仅 390 一档，641/1280 缺，已在 §4 缺档列写明；若复核方要求「三档齐备才算有」，E3 应降为 `缺`，矩阵计数将变为 有 12 / 缺 55 / N/A 37。
4. **未逐张打开 PNG 复核像素**：`有` = 文件存在、路径可解析（§5.3 的 `test -f` 断言）且被 T21 台账或截图索引登记，不等于视觉验收通过；视觉判定属第 3 条后续复核。
5. **641 全矩阵零达标**：13 个「有」格无一具备 641，W2 是从零采集，不是补档。
6. **非原型三页（books/transactions/recycle）未入矩阵**：`books` 的 2 张仅用于张数核对（41 + 2 = 43），不计入 104 格；三页归属待 P2 裁决。
7. **未运行任何测试**：统计数字为只读引用，未跑 C 套件、未起服务、未做浏览器实测。
