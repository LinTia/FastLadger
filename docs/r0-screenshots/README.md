# FastLedger R0 截图基线索引（任务 C 交付物）

> **状态声明（先读）**：本文件是「真实截图索引 + 未覆盖/阻塞清单」，**不是**「截图基线已完成」的验收报告。
> 按 R0 验收口径，截图基线当前 **未完成 / 阻塞，R0 不因此通过**。详见 §5 结论。
> HEAD 基准：`bff6a2b351bf5a039cdf7c762ec2693fb330cd00`。本文件为本轮新增的**截图索引文档**；同批新增 43 张 PNG，另修订了 `CHANGELOG.md`、`README.md`、R0 主报告与独立复核记录（整体范围见 `CHANGELOG.md` [1.63]）。

---

## 0. 证据边界（引用本索引前必须阅读）

| 项 | 说明 |
|---|---|
| 本文件职责 | 仅索引 43 张已落盘 PNG、说明采集环境、登记未覆盖/阻塞项；**不重复采集、不改动任何 PNG**（43 张由本轮 Edge headless + CDP 一次采集落盘）。 |
| 截图来源 | 43 张 PNG 由本轮以 Microsoft Edge headless + CDP（`Page.captureScreenshot`）采集；采集脚本为工作区 `.tmp_shot/capture.js`（临时文件，不入库），输出 `.tmp_shot/manifest.json`。 |
| 后端契约依据 | 权限 / DTO / 错误码契约**全部来自 `bff6a2b` 源码静态实读**，**不以运行时二进制为依据**。 |
| SHA 时序差（运行时未复验） | `bff6a2b` 提交于 `2026-11-04 13:45:36 +0800`（该提交本身仅文档改动）；`server/fastledger.exe` 构建于 `2026-11-04 01:13:59 +0800`。截图采集时服务在跑，且当时已实测 HTTP 提供的 `/app.js` 与仓库 HEAD 完全一致、`/style.css` 仅 CRLF 差异，故截图中的前端资源与 `bff6a2b` 一致。**当前本地服务已停止（`/healthz` 无响应），无法复验**。该时序差标注为「运行时未复验」：既不得据此宣称已验证，也不得据此宣称截图无效。 |
| 根 README 口径冲突（R0-E-33 已登记） | 基线提交 `bff6a2b` 时根 `README.md` L88 写「Web 端（阶段 5，未开始）…当前后端未嵌入任何前端资源」，而 `server/web/embed.go` 有 `go:embed dist` 且 `server/web/dist/` 存在完整 `index.html`/`app.js`/`style.css`；L7 日期 `2026-10-31`、L13 `schema v6`、L14 `645 PASS` 亦已过时。**该冲突已在本轮交付中修订**（根 README 现标注为历史快照、schema 改 v8、Web 端改为 `go:embed dist` 同源托管；见 `CHANGELOG.md` [1.63]），本索引只登记该冲突为历史发现，不重复改动。 |
| 证据粒度 | 下表中 `S:` 证据**只证明「控件 / 页面在该状态下可见」**，不证明交互成功路径已验收、不证明后端契约正确、不证明视觉 token 对齐。 |
| 索引核对方式 | 43 条记录的**相对路径、实际字节数**均已对磁盘逐文件核对（`wc -c`），PNG 魔数均为 `89504e470d0a1a0a`；字节数与 `.tmp_shot/manifest.json` 逐条一致，总字节 `1,630,964`。 |

---

## 1. 采集环境（可复现）

### 1.1 环境清单

| 项 | 值 | 依据 |
|---|---|---|
| 浏览器 | Microsoft Edge `156.0.4314.58` | 本机实测（PowerShell `VersionInfo.ProductVersion`），采集时复核 |
| 浏览器路径 | `C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe` | `capture.js` L8 |
| 启动参数 | `--headless=new --disable-gpu --no-first-run --no-default-browser-check --hide-scrollbars --disable-extensions --no-sandbox --remote-debugging-port=9231 --user-data-dir=<.tmp_shot/profile-cdp> --window-size=390,844 about:blank` | `capture.js` L89–94 |
| CDP 客户端 | Node **内置** `WebSocket`（`new WebSocket(wsUrl)`），无第三方依赖；自实现 `CDP.send/on/off` | `capture.js` L17–38、L101–105 |
| 调试端口 | `9231`；就绪探测 `http://127.0.0.1:9231/json/version`，目标列表 `/json/list` | `capture.js` L9、L43、L52 |
| 启用的域 | `Page.enable`、`Runtime.enable` | `capture.js` L104–105 |
| 会话注入 | `Page.addScriptToEvaluateOnNewDocument` 注入 `localStorage.setItem("fl_token", <token>); localStorage.setItem("fl_whoami", "admin");` | `capture.js` L110–112 |
| 未登录态 | `Page.removeScriptToEvaluateOnNewDocument` + `Runtime.evaluate` 执行 `localStorage.clear()` | `capture.js` L114–117 |
| 视口设置 | `Emulation.setDeviceMetricsOverride { width, height, deviceScaleFactor: 1, mobile }` | `capture.js` L123 |
| 出图 | `Page.captureScreenshot { format: 'png', captureBeyondViewport: false }`，base64 解码落盘 | `capture.js` L140–145 |
| 等待策略 | 导航后 `sleep(1900)`；弹层动作后 `sleep(700)` | `capture.js` L135、L138 |
| 导航 URL | `http://127.0.0.1:8080/?r0=<序号><hash>`（`?r0=` 破缓存 + hash 路由） | `capture.js` L133 |
| 服务端 | 本地 `localhost:8080`（脚本内 `127.0.0.1:8080`）的 `fastledger.exe`；Go `1.22.5` + w64devkit CGO 构建；`go:embed dist` 同源提供前端 | 本轮本地运行基线，见 R0 主报告 §7.4 |
| 测试账号 | `admin`（owner 角色） | 本轮 `user create --owner` 创建（见 R0 主报告 §7.4）；注入见 `capture.js` L110 |
| 测试数据 | 测试账本「我的家庭账本」`id=1`；`schema v8` | 本轮 `db migrate` 后创建；schema v8 由 `server/internal/migrate/migrations/` 0001–0008 迁移实读，见 R0 主报告 §7.4 |
| 采集脚本 | `C:/Users/47006/WorkBuddy/fastledger/.tmp_shot/capture.js`，用法 `node capture.js <token>`，输出 `.tmp_shot/manifest.json` | `capture.js` L2–3、L154 |

### 1.2 视口档位（4 档）

| 名称 | 宽×高 | `mobile` | 用途 |
|---|---|---|---|
| `360x800` | 360×800 | true | 小屏手机 |
| `390x844` | 390×844 | true | 主力手机（覆盖最广，17 张） |
| `430x932` | 430×932 | true | 大屏手机 |
| `1280x800` | 1280×800 | false | 桌面 |

### 1.3 采集任务路由表（`capture.js` L67–85，用于复现）

| 页面目录 | hash | 状态 | 视口 | 特殊动作 |
|---|---|---|---|---|
| `login` | （空） | normal | 4 档 | `token:false`（清 localStorage） |
| `books` | `#books` | normal | 390x844, 1280x800 | — |
| `home` | `#b/1` | empty | 4 档 | — |
| `home` | `#b/1` | error | 390x844 | `failApi:true`（见 §4.9） |
| `add-transaction` | `#b/1` | normal | 4 档 | `action: document.getElementById('fab')?.click()` |
| `transactions` | `#b/1/txns` | empty | 390x844, 1280x800 | — |
| `accounts` | `#b/1/accounts` | normal | 4 档 | — |
| `account-detail` | `#b/1/account/1` | normal | 390x844, 1280x800 | — |
| `categories` | `#b/1/categories` | normal | 390x844, 1280x800 | — |
| `recycle` | `#b/1/recycle` | empty | 390x844, 1280x800 | — |
| `report` | `#b/1/report` | empty | 390x844, 1280x800 | — |
| `budget` | `#b/1/budget` | empty | 390x844, 1280x800 | — |
| `invest` | `#b/1/invest` | empty | 390x844, 1280x800 | — |
| `export` | `#b/1/export` | normal | 390x844, 1280x800 | — |
| `members` | `#b/1/members` | normal | 390x844, 1280x800 | — |
| `mine` | `#b/1/mine` | normal | 390x844, 1280x800 | — |
| `settings` | `#settings` | normal | 4 档 | — |

---

## 2. 完整索引表（43 张，逐文件核对）

> 路径均相对本 `README.md` 所在目录。**字节数为磁盘实测值**（`wc -c`），非直接照抄 manifest；核对结果 43/43 一致。

| # | 页面 | 视口 | 状态 | 相对路径 | 字节数 |
|---:|---|---|---|---|---:|
| 1 | login | 360x800 | normal | `login/360x800/normal.png` | 17,762 |
| 2 | login | 390x844 | normal | `login/390x844/normal.png` | 18,137 |
| 3 | login | 430x932 | normal | `login/430x932/normal.png` | 18,187 |
| 4 | login | 1280x800 | normal | `login/1280x800/normal.png` | 20,358 |
| 5 | books | 390x844 | normal | `books/390x844/normal.png` | 17,297 |
| 6 | books | 1280x800 | normal | `books/1280x800/normal.png` | 24,819 |
| 7 | home | 360x800 | empty | `home/360x800/empty.png` | 31,476 |
| 8 | home | 390x844 | empty | `home/390x844/empty.png` | 31,910 |
| 9 | home | 430x932 | empty | `home/430x932/empty.png` | 33,687 |
| 10 | home | 1280x800 | empty | `home/1280x800/empty.png` | 41,014 |
| 11 | home | 390x844 | error | `home/390x844/error.png` | 11,446 |
| 12 | add-transaction | 360x800 | normal | `add-transaction/360x800/normal.png` | 45,953 |
| 13 | add-transaction | 390x844 | normal | `add-transaction/390x844/normal.png` | 50,084 |
| 14 | add-transaction | 430x932 | normal | `add-transaction/430x932/normal.png` | 54,280 |
| 15 | add-transaction | 1280x800 | normal | `add-transaction/1280x800/normal.png` | 59,173 |
| 16 | transactions | 390x844 | empty | `transactions/390x844/empty.png` | 10,949 |
| 17 | transactions | 1280x800 | empty | `transactions/1280x800/empty.png` | 16,142 |
| 18 | accounts | 360x800 | normal | `accounts/360x800/normal.png` | 34,239 |
| 19 | accounts | 390x844 | normal | `accounts/390x844/normal.png` | 34,803 |
| 20 | accounts | 430x932 | normal | `accounts/430x932/normal.png` | 35,068 |
| 21 | accounts | 1280x800 | normal | `accounts/1280x800/normal.png` | 44,881 |
| 22 | account-detail | 390x844 | normal | `account-detail/390x844/normal.png` | 36,329 |
| 23 | account-detail | 1280x800 | normal | `account-detail/1280x800/normal.png` | 43,130 |
| 24 | categories | 390x844 | normal | `categories/390x844/normal.png` | 63,867 |
| 25 | categories | 1280x800 | normal | `categories/1280x800/normal.png` | 72,830 |
| 26 | recycle | 390x844 | empty | `recycle/390x844/empty.png` | 24,526 |
| 27 | recycle | 1280x800 | empty | `recycle/1280x800/empty.png` | 33,177 |
| 28 | report | 390x844 | empty | `report/390x844/empty.png` | 32,905 |
| 29 | report | 1280x800 | empty | `report/1280x800/empty.png` | 40,269 |
| 30 | budget | 390x844 | empty | `budget/390x844/empty.png` | 28,631 |
| 31 | budget | 1280x800 | empty | `budget/1280x800/empty.png` | 35,246 |
| 32 | invest | 390x844 | empty | `invest/390x844/empty.png` | 50,264 |
| 33 | invest | 1280x800 | empty | `invest/1280x800/empty.png` | 59,012 |
| 34 | export | 390x844 | normal | `export/390x844/normal.png` | 53,353 |
| 35 | export | 1280x800 | normal | `export/1280x800/normal.png` | 75,716 |
| 36 | members | 390x844 | normal | `members/390x844/normal.png` | 40,170 |
| 37 | members | 1280x800 | normal | `members/1280x800/normal.png` | 49,274 |
| 38 | mine | 390x844 | normal | `mine/390x844/normal.png` | 41,094 |
| 39 | mine | 1280x800 | normal | `mine/1280x800/normal.png` | 46,159 |
| 40 | settings | 360x800 | normal | `settings/360x800/normal.png` | 36,185 |
| 41 | settings | 390x844 | normal | `settings/390x844/normal.png` | 36,722 |
| 42 | settings | 430x932 | normal | `settings/430x932/normal.png` | 37,115 |
| 43 | settings | 1280x800 | normal | `settings/1280x800/normal.png` | 43,325 |
| | **合计** | | | **43 个文件** | **1,630,964** |

### 2.1 分布统计（实测）

| 维度 | 分布 |
|---|---|
| 状态 | `normal` 28 张 / `empty` 14 张 / `error` 1 张 |
| 视口 | `390x844` 17 / `1280x800` 16 / `360x800` 5 / `430x932` 5 |
| 页面 | 16 个目录：`home` 5；`login`、`accounts`、`add-transaction`、`settings` 各 4；其余 11 个各 2 |
| 状态 × 页面 | `home` 是唯一同时有 `empty`(4) 与 `error`(1) 的页面；`error` 全库仅 1 张 |
| 合计 | 43 张 / 1,630,964 字节；43 张 PNG 魔数均为 `89504e470d0a1a0a` |

---

## 3. 状态覆盖真相

| 项 | 结论 |
|---|---|
| 本轮覆盖范围 | **仅 `normal` / `empty` / `error` 三类「首屏」静态状态**，且是导航后固定 `1900ms` 的单帧截图。 |
| `S:` 证据含义 | 只证明「该控件 / 页面在此状态下可见、可渲染」。**不证明**交互成功路径已验收、**不证明**失败路径已验收、**不证明**状态转换正确。 |
| 已可见（举例） | 登录页仅「用户名 / 密码 / 登录 / 没有账号？注册新账号」，**无「加入账本」入口、无邀请码输入、无密码明文切换**（佐证 R0-E-38 与我方前端取证）；首页 empty 态三卡均为 `¥0.00` + 空态文案「还没有账单——点下方「＋ 记一笔」开始记录。」+ 底部 4 Tab/FAB。 |
| **未采集（明确）** | 绝大多数**成功 / 失败交互路径**、**加载态**、**权限不足态**、**离线态**、**同步 / 冲突态**、**多角色态**、**原型侧对照截图**。 |
| 禁止表述 | **禁止**把 43 张说成「截图基线已完成」「视觉已验收」「交互已验收」。43 张只是首屏可见性索引。 |

### 3.1 已采集状态 × 页面矩阵

| 页面 | 360x800 | 390x844 | 430x932 | 1280x800 |
|---|---|---|---|---|
| login | normal | normal | normal | normal |
| books | — | normal | — | normal |
| home | empty | empty + error | empty | empty |
| add-transaction | normal | normal | normal | normal |
| transactions | — | empty | — | empty |
| accounts | normal | normal | normal | normal |
| account-detail | — | normal | — | normal |
| categories | — | normal | — | normal |
| recycle | — | empty | — | empty |
| report | — | empty | — | empty |
| budget | — | empty | — | empty |
| invest | — | empty | — | empty |
| export | — | normal | — | normal |
| members | — | normal | — | normal |
| mine | — | normal | — | normal |
| settings | normal | normal | normal | normal |

> 空白 `—` 表示该页面该视口**本轮未采集**，不代表控件不存在。

### 3.2 `S:` 证据的效力边界

按 R0 口径，`S:`（截图证据）**只证明「该页面 / 该控件在该视口、该状态下可见」**，不证明以下任何一项：

- 交互**成功**路径已验收（提交保存、删除、重绘、跳转、下载）；
- 交互**失败**路径已验收（校验红字、`err-msg`、后端 4xx/5xx toast）；
- 表单填写、弹窗二次确认、编辑态、字段联动（转账双账户显隐）已验收；
- 数据联动已验收（余额刷新、报表重算、列表分页、回收站恢复后行消失）；
- 权限控制、会话态、并发冲突、离线恢复已验收；
- 视觉细节已验收（三态色、色板、设计 token 一致性，见 R0-E-09/10/11/13）。

**43 张中没有任何一张**是「点击提交后的成功/失败态」「二次确认对话框打开态」「编辑弹窗打开态」「删除后列表刷新」「导出文件下载」「toast 消失后的状态」或「请求 pending 期间的加载态」。`add-transaction` 的 4 张只证明「记一笔弹窗在默认支出类型下可见」，不证明保存按钮可用或提交成功。

### 3.3 抽查所见（实际打开图片核对，非照抄 manifest）

| 截图 | 实际所见 |
|---|---|
| `login/390x844/normal.png` | 「登录 FastLedger」标题、副标题「家庭智能记账 · 截图秒记」、用户名框、密码框、「登 录」按钮、「没有账号？注册新账号」链接；**无「加入账本」入口、无邀请码输入框、无密码明文切换** —— 与前端源码取证一致，佐证 R0-E-38 |
| `home/390x844/empty.png` | 三张概览卡（本月收入 `+¥0.00` / 本月支出 `-¥0.00` / 结余 `¥0.00`）、空态文案「还没有账单——点下方「＋ 记一笔」开始记录。」、底部 4 Tab + 绿色 FAB |
| `home/390x844/error.png` | `#view` 主体**空白、无骨架屏条**，底部 Tab / FAB 仍在，红色 toast「✕ Failed to fetch」。与 R0-E-35 原描述「骨架屏残留」不一致，见 §4.9 |

### 3.4 禁止的表述

不得把 43 张称为「截图基线已完成」「R0 视觉验收通过」「逐控件截图齐全」；不得据此计算「逐控件截图覆盖率已达标」。本索引只能作为「已采集部分」的清单，**不是** R0 §7 要求的逐页逐状态逐控件基线。

---

## 4. 未覆盖 / 阻塞清单（重点，不得省略或弱化）

> **本节是本文件的核心，不得省略或弱化。** 下表逐项列出缺失的截图类别、缺失原因、所需环境、执行人与可复现步骤。执行人列写**角色**（Owner / CI / QA / 前端，**待 Owner 指派具体责任人**）；凡标注「待指派」的项在关闭前一律视为**阻塞**。可复现步骤均基于 `capture.js` 的 CDP 能力与 `?r0=<序号><hash>` 导航方式。

### 4.1 交互成功 / 失败路径

| 缺失截图类别 | 为什么缺 | 所需环境 | 执行人 | 可复现步骤 |
|---|---|---|---|---|
| 登录成功 / 失败 toast、注册切换与提交、两次密码不一致 | 本轮只截登录页首屏，未提交表单 | 有效/无效账号；可观察 toast | QA / 前端 | 注入空 token 打开 `?r0=N`，填 `用户名/密码` → 点「登 录」→ 截图 toast；点「没有账号？注册新账号」→ 截图注册表单；填不一致两次密码提交 → 截图错误 toast |
| 记一笔：类型切换（转账双账户显隐）、金额校验、转账同账户校验、提交成功 toast、pending 态、取消 / Esc 关闭、重复提交 | 本轮只截 `#fab` 点击后弹层首帧（normal） | 有账户/分类的账本；CDP `Runtime.evaluate` | QA / 前端 | 打开 `#b/1`，点 `#fab` → 改 `#fType`=3 截图；填非法金额提交截图 toast；填合法数据点「保存」→ 截图 pending 与成功 toast；点「取消」截图关闭态 |
| 交易详情：编辑保存 / 编辑失败（并发）、删除二次确认 / 取消 / 成功、字段展示 | 本轮无交易数据，无法进入 `#b/1/tx/:txId` | 构造至少 1 笔交易 | QA / 前端 | 用 §4.7 造数后点账单行进详情 → 点「编辑」截图弹层；点「删除」截图确认框；确认后截图 toast 与返回态 |
| 账户 / 分类 / 预算 / 投资 / 成员 / Token 的新增、编辑、删除、确认框、保存成功 / 失败 | 本轮只截各页首屏，未触发任何 `open*Dialog` | 构造数据 + 可点按钮 | QA / 前端 | 逐页点「新增 / 编辑 / 删除」按钮 → 截图弹层与确认框；提交后截图 toast 与重绘结果 |
| 批量删除、全选、筛选查询 / 重置、翻页、恢复 | 本轮 `transactions` 为 empty 态 | 多笔交易 + 回收站有数据 | QA / 前端 | 进 `#b/1/txns` 勾选行 → 点「删除选中」截图确认框；用关键词查询截图过滤结果；进 `#b/1/recycle` 点「恢复」截图 toast |
| 导出：预览、CSV/JSON 下载、空数据、截断提示、字段未勾选 | 本轮只截导出页首屏 | 构造交易数据 + 可观察下载 | QA / 前端 | 进 `#b/1/export` 点「预览」截图条数；取消全部字段点「导出 CSV 文件」截图 toast；点导出截图下载触发 |

### 4.2 加载态 / pending 态

| 缺失截图类别 | 为什么缺 | 所需环境 | 执行人 | 可复现步骤 |
|---|---|---|---|---|
| 骨架屏 **pending 期间**（交易总表 5 条、账单流 4 条） | 固定 `sleep(1900)` 后截图，请求早已返回；需慢速网络 | CDP `Network.emulateNetworkConditions` 或 `Fetch.continueRequest` + 延迟 | CI / QA | 拦截 `GET /books/1/transactions`，用延迟放行，在请求 pending 时 `Page.captureScreenshot` |
| 按钮 pending 文案（「登录中…」「保存中…」「删除中…」）与禁用态 | 同上，请求太快 | 慢速网络 | CI / QA | 拦截对应 POST/PUT/DELETE 并延迟，在 pending 期间截图按钮 |

### 4.3 错误 / 权限 / 离线 / 冲突态

| 缺失截图类别 | 为什么缺 | 所需环境 | 执行人 | 可复现步骤 |
|---|---|---|---|---|
| 401 登录失效回登录页 | 需构造过期/无效 token | 可注入非法 token | QA / 前端 | 注入 `fl_token=<非法>` 打开 `#b/1`，截图 `clearSession()` 后回登录页 |
| 403 权限不足、409 引用保护（删除被引用的账户/分类/标的）、409 幂等冲突 | 需多角色账号 + 构造引用数据 + 重复 `clientUuid` | 多角色账号（见 §4.6）、造数 | Owner / QA | 以普通成员登录触发 403；删除被交易引用的分类触发 409；重复提交同一 `clientUuid` 触发幂等 |
| 离线态、网络中断后的可恢复反馈 | 需 `Network.emulateNetworkConditions` offline | CDP 离线模拟 | CI / QA | 置 offline 后操作，截图失败 toast 与重试入口 |
| 逐页错误态（非仅首页） | 本轮仅 `home/390x844/error.png` 一张 | 选择性 API 拦截（见 §4.9） | CI / QA | 对 16 个页面分别注入失败，截图各自错误态 |
| 同步中 / 冲突态（`baseVersion` 乐观锁） | 当前 Web 未接 sync pull/push（R0-E-15/25），且无冲突造数 | 造并发修改 + 运行时日志 | Owner / 前端 | 两会话并发编辑同一交易，截图并发提示 toast |

### 4.4 视觉 / 设计 token 验收

| 缺失截图类别 | 为什么缺 | 所需环境 | 执行人 | 可复现步骤 |
|---|---|---|---|---|
| 预算三态色（正常 / 接近 / 超支，R0-E-10） | 本轮 budget 为 empty 态 | 构造不同 `percent` 的预算 | QA / 前端 | 造 3 个预算（spent 低/接近/超），进 `#b/1/budget` 截图进度条与徽标 |
| 报表柱状图 + 分类占比（R0-E-09） | 本轮 report 为 empty 态 | 构造多月收支数据 | QA / 前端 | 造数后进 `#b/1/report`，分别点「本月」「上月」截图 |
| 角色徽标 / 可见性徽标（R0-E-11、R0-E-19/27） | 本轮 members 仅 owner 单成员 | 多成员账号 | QA / 前端 | 造 owner/admin/普通成员，进 `#b/1/members` 截图徽标与表格 |
| design token 与 `style.css` 对齐（R0-E-13） | 需 token 逐项比对，非单帧截图可证 | `design-tokens.json` + 计算样式 | 前端 | 用 `Runtime.evaluate` 取 computed style 与 token 表逐项核对并归档 |

### 4.5 原型侧对照截图

| 缺失截图类别 | 为什么缺 | 所需环境 | 执行人 | 可复现步骤 |
|---|---|---|---|---|
| `prototype/` 与 `prototype-v2/` 的 13 页对照截图 | 原型是**静态站点**，需单独起静态服务；本轮服务仅 `localhost:8080`（后端同源提供 dist），**未起 `prototype/` 8765 端口** | 静态文件服务（`prototype/` 目录，端口 `8765`）+ 同一 4 档视口 | QA / 前端 | 在 `prototype/` 起静态服务（如 `python -m http.server 8765`），用同一 CDP 脚本、`BASE=http://127.0.0.1:8765/`、同样 4 视口逐页截图，与 §2 索引一一对照 |
| 差异并排图（实现 vs 原型） | 依赖上一行 | 两服务同时在线 + 图像拼接 | QA / 前端 | 同视口、同页面成对截图后生成并排对照 |

### 4.6 多角色 / 多账号 / 权限矩阵

| 缺失截图类别 | 为什么缺 | 所需环境 | 执行人 | 可复现步骤 |
|---|---|---|---|---|
| owner / admin / 普通成员 / 被移除成员视图差异 | 本轮仅 `admin`（owner）单账号 | 至少 3 个角色账号 + 邀请加入流程 | Owner / QA | 分别注入各角色 token，逐页截图导航可见性与按钮禁用差异 |
| 账本可见性开 / 关两态（R0-E-19/27） | Web 端无该开关（前端未接 `PUT /books/:id/visibility`） | 先补 UI 或直接改库造两态 | Owner / 前端 | 造两种 `members_can_view_others` 状态，对照截图成员可见范围 |
| 邀请码生成 → 复制 → **加入账本** 闭环（R0-E-38） | Web 端无「加入账本」入口 / 邀请码兑换（后端 `router.go:128` 已交付但 Web 未接） | 两个独立会话 + 后端 join 路由 | Owner / 前端 | 会话 A 生成邀请码；会话 B 在登录页尝试兑换 → 记录当前**无入口**为阻塞证据 |

### 4.7 测试数据构造（多个阻塞项的前置依赖）

| 缺失数据 | 为什么缺 | 所需环境 | 执行人 | 可复现步骤 |
|---|---|---|---|---|
| 有交易、有余额、有分类、有预算、有持仓/标的、有多成员、回收站有数据、跨月报表数据 | 本轮账本 `id=1` 基本为空，故 14 张为 `empty` 态、`normal` 页也多为无数据渲染 | 可写测试库 + 造数脚本（或 API 批量造数） | QA / CI | 备份当前库 → 用 API 或 SQL 造数 → 重新以同一 `capture.js` 采集 `normal` 态 → 归档后回滚/隔离测试库 |

### 4.8 视口与断点补采

| 缺失截图类别 | 为什么缺 | 所需环境 | 执行人 | 可复现步骤 |
|---|---|---|---|---|
| 11 个页面在 360x800 / 430x932 两档视口的截图（当前仅 login/home/add-transaction/accounts/settings 覆盖 4 档） | 采集脚本 `TASKS` 未给这些页面配置 4 档（L73–83） | 同 `capture.js`，扩展 `vps` 数组 | CI / QA | 将对应任务的 `vps` 改为 4 档后重跑，补齐矩阵空白 `—` 单元格 |
| 更多断点（平板 / 宽屏 / 窄屏 320） | 本轮仅 4 档 | 同 `Emulation.setDeviceMetricsOverride` | QA | 增加视口档位并重跑 |

### 4.9 R0-E-35「骨架屏残留」专项复验（**重点，不得据本轮截图判定**）

| 项 | 内容 |
|---|---|
| 原描述 | R0-E-35：首页账单流请求失败后，catch 仅 toast 并 return，骨架屏不会被清除 / 残留（关联 R0-E-12 状态覆盖缺口） |
| 本轮构造方式 | `capture.js` L127–131：`Fetch.enable({ patterns: [{ urlPattern: '*/api/v1/*', requestStage: 'Request' }] })` + `Fetch.failRequest({ requestId, errorReason: 'Failed' })`，即**拦截全部 `/api/v1/*` 并立即失败**，不是慢速网络。 |
| 实测表现 | `home/390x844/error.png`（11,446 B）：`#view` **主体空白、无骨架屏条**；底部 4 Tab 与绿色 FAB 仍可见；红色 toast「✕ Failed to fetch」。 |
| 与 R0-E-35 原描述的差异 | 原描述为「请求失败时骨架屏残留在页面」。本轮**未观察到骨架屏残留**，与原描述**不完全一致**。 |
| **判定口径** | **不能据此判定已修复，也不能据此判定已证伪。** |
| 差异原因（**基于源码路径的推断，需运行时日志确认**） | `renderHome`（`app.js` L1931）在渲染前先 `await loadBookMeta(bookId)`，而 `loadBookMeta`（L308–309）的 `GET /books/:id/accounts`、`GET /books/:id/categories` 同属被拦截的 `/api/v1/*`。因此失败很可能发生在**更早的元数据加载阶段**，异常冒泡到 `route()` 的 `catch`（L2235–2237）弹出 `Failed to fetch` toast，`view.replaceChildren()`（L1936）之前即中断，`renderBillStream`（L1872–1879）的骨架屏分支**可能根本未执行**。若成立，这张图测的是「元数据加载失败」而非 R0-E-35 描述的「账单流 GET 失败」。此推断未经运行时 Network/Console 日志确认，不得当作结论。 |
| 时机差异 | 截图在导航后固定等待 `1900ms` 才采集（`capture.js` L135），未覆盖请求 **pending 期间**骨架屏可见的时段。 |
| 所需环境 | 可**选择性放行** API 的 CDP 会话；`Network.emulateNetworkConditions` 慢速网络；运行时 Network + Console 日志采集。 |
| 执行人 | **Owner 裁决** / CI 执行（需裁决与运行时复验能力）。 |

> 上述「差异原因」是**基于 `capture.js` 与 `app.js` 代码路径的推断**，不是运行时实测结论；需以 Console / Network 日志确认失败发生在 `loadBookMeta`（`app.js` L308–309）还是 `renderBillStream`（L1878）分支。在确认前，不得把它当作「已修复」或「已证伪」的证据。

**五步复验流程（Owner / CI）：**

1. CDP `Fetch.enable` **只对** `GET /books/1/transactions*` 生效，**放行** `/books/1/accounts`、`/books/1/categories`、`/books`、`/users/me` 等；对目标请求延迟 `Fetch.continueRequest`（人为 2–5s 慢速）或最终 `Fetch.failRequest`。
2. 导航到 `#b/1`，在请求 **pending 期间**（导航后 200–500ms）截图 → 观察骨架屏是否可见。
3. 在请求失败 / 返回后截图 → 观察骨架屏是否被清除、是否提供可恢复反馈（重试入口 / 页面级错误态，而非仅 toast）。
4. 采集 Console / Network 日志，确认失败发生在 `loadBookMeta`（`app.js` L308–309）还是 `renderBillStream`（L1878）分支。
5. 由 **Owner 依据上述证据裁决 R0-E-35 状态**；结论产生前该项保持「待验证 / 阻塞」。

---

## 5. 结论

| 项 | 结论 |
|---|---|
| 截图基线状态 | **未完成 / 阻塞**。本轮 43 张仅为 `normal` / `empty` / `error` **首屏可见性**证据索引。 |
| R0 是否通过 | **R0 不因此通过**。§4 的未覆盖/阻塞清单未闭环前，不得声称截图验收完成。 |
| 43 张的合法用途 | 作为「页面/控件在某状态下可见」的 `S:` 级证据，供 §3.1 逐控件清单做**可见性**旁证；**不得**用于证明交互成功/失败路径、后端契约、视觉 token 对齐或状态转换正确。 |
| 替代禁令 | **不得**用原型截图、HTML/SVG 渲染、手绘或伪造截图替代真实运行截图；不得把 `prototype/` 静态页截图当作 Web 实现截图。 |
| 排期禁令 | **不得**把该验收门槛挪到 R1、降级为「后续补」或以「已有 43 张」冲抵。它属于 R0 交付门槛，必须在 R0 内闭环。 |
| 时序差处理 | `bff6a2b` 与 `fastledger.exe` 的构建时序差标注为「运行时未复验」（当前 `localhost:8080` 已停、`/healthz` 无响应）；既不得据此宣称已验证，也不得据此宣称截图无效。后端契约一律以源码静态实读为准。 |
| 解阻条件 | §4.1–§4.9 逐项产出截图与日志、补齐 §3.1 矩阵空白视口、完成原型侧对照、完成 R0-E-35 由 Owner 运行时复验并裁决后，方可重新评估基线状态并考虑将本文件状态改为「已完成」。 |

关联差异编号：R0-E-12（状态覆盖缺口）、R0-E-35（骨架屏残留）、R0-E-09 / 10 / 11 / 13（视觉待截图）、R0-E-15 / 25（Web 未接实时同步客户端，阻塞 §4.3 冲突态）、R0-E-38（登录页无加入账本入口，本轮登录页截图佐证）、R0-E-33（根 README 口径冲突，已在本轮修订）。

---

## 附录 A 文件校验记录

| 校验项 | 结果 |
|---|---|
| PNG 文件数 | 43 张，与 `.tmp_shot/manifest.json` 43 条记录一一对应，无多无缺 |
| 路径一致性 | 磁盘相对路径与 manifest 的 `file` 字段逐条一致 |
| 字节数 | 逐文件 `wc -c` 实测，与 manifest `bytes` 逐条一致；合计 **1,630,964 字节** |
| 文件格式 | 43 张前 8 字节均为 `89 50 4E 47 0D 0A 1A 0A`（有效 PNG 文件头） |
| 内容抽查 | 实际打开 3 张核对：`login/390x844/normal.png`、`home/390x844/empty.png`、`home/390x844/error.png`，所见记录于 §3.3 |
| 采集脚本 | `.tmp_shot/capture.js` 实读，环境参数见 §1 |

## 附录 B 变更范围声明

本文件（`docs/r0-screenshots/README.md`）是**本轮新增的截图索引文档**，同批新增 43 张 PNG。本轮整体交付另修订了 `CHANGELOG.md`、`README.md`、`docs/FastLedger-R0-基准冻结与差异盘点.md`、`docs/FastLedger-R0-独立补漏与复核记录.md` 四份既有文档（范围见 `CHANGELOG.md` [1.63]）。本文件未修改任何 PNG、未修改 `.tmp_shot/capture.js` 与 `.tmp_shot/manifest.json`；`.tmp_shot/` 为工作区临时目录，**不入库**。

---

*本文件依据 `.tmp_shot/manifest.json`、43 张 PNG 磁盘实测、`.tmp_shot/capture.js`、`server/web/dist/app.js` / `index.html`（HEAD `bff6a2b`）及 R0 主报告 §7.4 本地运行基线整理。环境事实与截图采集均来自本轮 R0 二次整改，采集脚本与日志位于工作区 `.tmp_shot/`（不入库）。*
