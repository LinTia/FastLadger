# R0-RV-01 隔离运行时复验证据（2026-11-04）

真实 Edge headless + CDP 运行时证据，非静态文档；对应 `docs/FastLedger-R0-独立复核补充-RV-01至04.md` RV-01。

> **R0-E-35 状态（[1.68] 统一口径）**：本机隔离运行时复验已完成；失败态观察到骨架屏未清理（`.skel`=4、`aria-busy=true`）；待 Owner/CI 独立复跑及慢速网络、有数据态、多角色等场景验证；**缺陷尚未关闭**。R0 未通过 / 阻塞，不授权 R1。

## 环境

- 浏览器 `Edg/156.0.4314.58`（`--headless=new`，CDP `/json/version` 实测），Node v22.22.2 内置 WebSocket。
- 视口 390×844（`mobile=true`）、1280×800（`mobile=false`），`Emulation.setDeviceMetricsOverride`。
- 运行时证据采集时的**历史文档 HEAD** `557e4d223064c149cdf1063429a2fdd79158e609`；当前报告版本基准见主报告 §0.2「证据版本基准」，两者不得互相推定。
- 服务 `:8080`；本地库 `server/.localdata/fastledger.db`（未删改数据）；账本 `id=1`、`transactions total=0`。
- 登录 `POST /api/v1/auth/login`，账号与口令**只从本地环境变量读取**（`FL_TEST_USER` / `FL_TEST_PASSWORD`），取 `data.accessToken` 注入浏览器；仓库内不含任何真实凭证或固定测试口令。
- 注入 `Page.addScriptToEvaluateOnNewDocument` → `localStorage.setItem("fl_token", <R0_TOKEN>)` + `localStorage.setItem("fl_whoami", <R0_WHOAMI>)`。

## 拦截规则（与上一轮泛化拦截的核心区别）

- `Fetch.enable({patterns:[{urlPattern:"*/api/v1/books/1/transactions*",requestStage:"Request"}]})`。
- `Fetch.requestPaused` 内再严格判定 `method==="GET"` 且 `URL.pathname==="/api/v1/books/1/transactions"`；其余一律 `Fetch.continueRequest`。
- 放行 `GET /books`、`/books/1/accounts`、`/books/1/categories`、`/books/1/reports/monthly?months=2` 及静态资源（见 `network-log.json`）。
- 实测目标 URL：`http://127.0.0.1:8080/api/v1/books/1/transactions?page=1&count=60`。

## 精确复现（RV-06：可移植脚本）

复现脚本已提交到仓库：**`tools/r0/rv01-isolation.js`**；完整依赖、环境变量、退出码、清理方式与平台限制见 **`tools/r0/README.md`**。所有路径由脚本位置 / 仓库根 / 环境变量推导，不含个人目录或工作区绝对路径。

> **证据保护（先读）**：本目录 5 PNG + 3 JSON 是已归档证据。脚本默认输出到操作系统临时目录（仓库外），**不会覆盖本目录**；只有同时设置 `R0_WRITE_EVIDENCE=1` 与 `R0_WRITE_EVIDENCE_CONFIRM=YES` 才允许写入正式证据目录，缺少确认时脚本以退出码 3 拒绝执行。

```bash
# 在仓库根目录执行。FL_TEST_USER / FL_TEST_PASSWORD 由你在本地私有配置中设定，
# 只经环境变量传递，不写入任何仓库文件、不打印。
export FL_TEST_USER='<你的测试用户名>'
export FL_TEST_PASSWORD='<你的测试口令>'

# 1) 启动本地后端（一次性随机密钥；二进制按本机构建产物命名）
export EBK_JWT_SECRET="$(python3 -c 'import secrets,base64;print(base64.b64encode(secrets.token_bytes(48)).decode())')"
export EBK_DB_PATH="$PWD/server/.localdata/fastledger.db"
export EBK_HTTP_PORT=8080 EBK_ENABLE_REGISTER=true
(cd server && ./fastledger.exe db migrate)
(cd server && ./fastledger.exe user create --username "$FL_TEST_USER" --password "$FL_TEST_PASSWORD" --owner)
(cd server && ./fastledger.exe) &
for i in $(seq 1 20); do curl -sf http://127.0.0.1:8080/healthz >/dev/null && break; sleep 1; done

# 2) 取 accessToken 到环境变量（不落盘）
LOGIN_JSON=$(python3 -c 'import os,json;print(json.dumps({"username":os.environ["FL_TEST_USER"],"password":os.environ["FL_TEST_PASSWORD"]}))')
export R0_TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login -H 'Content-Type: application/json' -d "$LOGIN_JSON" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["accessToken"])')

# 3) Windows 需显式指定浏览器（Linux/macOS 可用脚本内置的候选路径）
export R0_EDGE_PATH='/path/to/msedge.exe'

# 4) 运行：默认输出到临时目录，打印文件清单与 SHA256
node tools/r0/rv01-isolation.js

# 5) 合成假凭证脱敏测试（无需浏览器/服务）
node --test tools/r0/test-redaction.js
node tools/r0/scan-credentials.js --strict
```

**平台限制（如实声明）**：上述流程仅在 Windows 11 + Git Bash + Node v22.22.2 + Edge `156.0.4314.58` 实测；`./fastledger.exe` 为 Windows 构建产物名。Linux/macOS 需自行构建对应二进制并调整启动命令，脚本本身未在这些平台实测，**不声称跨平台**。

## 文件与状态

| 文件 | 状态 | 动作 |
|---|---|---|
| `01-pending-390x844.png` | pending | 暂停请求不 continue/fail，1.8s 截图 |
| `02-rejected-network-390x844.png` | rejected | `Fetch.failRequest(Failed)` 后 0.9s |
| `03-rejected-503-390x844.png` | rejected | `Fetch.fulfillRequest(503,JSON)` 后 0.9s |
| `04-pending-1280x800.png` | pending | 桌面视口持请求 |
| `05-recovered-390x844.png` | 恢复 | `Fetch.disable` 后重新导航，1.8s |
| `network-log.json` | 144 条真实 CDP 事件 + 1 条脚本生成 `__META__` 记录 | **已脱敏**：`Authorization` / `Cookie` / `Set-Cookie` 等键值及 JWT 字面量替换为 `[REDACTED]`；`Fetch.requestPaused` / `Network.*` 为真实事件，`__META__` 为采集元数据、**非** CDP 事件 |
| `console-log.json` | 5 条（4 × `Log.entryAdded` + `__META__`） | `Runtime.consoleAPICalled` / `exceptionThrown` / `Log.entryAdded` |
| `dom-snapshots.json` | 5 组 `Runtime.evaluate` 实测 | `.skel` 数 / `aria-busy` / toast / 重试控件 / 文本 |

## 本次实测观察（如实记录，不外推）

- **pending**（2 视口）：`.skel`=4、`aria-busy=true` 容器 1、toast 隐藏；概览三卡已渲染 → 依赖请求确实全部放行。
- **rejected**（网络失败 + 503，共 2 次）：`.skel` 仍=4、`aria-busy=true` 未清除；仅红色 toast（`✕ Failed to fetch` / `✕ 模拟故障：账单列表服务暂不可用（R0-RV-01 隔离注入）`）；DOM 中**无「重试/刷新」控件**（`retryControls=[]`）。
- **恢复**：解除拦截并重新导航后 `.skel`=0、toast 隐藏、显示空态「还没有账单——点下方「＋ 记一笔」开始记录。」→ 整页重载可恢复，但页面内无自助重试入口。
- 日志：`Network.loadingFailed net::ERR_FAILED` 3 次、503 `Network.responseReceived` 1 次；`Runtime.exceptionThrown` / `consoleAPICalled` 均 0（异常被 `catch` 吞掉，仅 toast）。
- 与 §7.5 旧截图的冲突得到解释：旧轮泛化拦截 `/api/v1/*` 同时打断 `loadBookMeta` 等依赖，`renderHome` 未走到 `renderBillStream`，故「页面主体空白」；本轮隔离后骨架屏残留可见。**不等于缺陷已关闭**。

## 证据边界与凭证安全（RV-05，如实拆分）

1. **当前 `main` 文件已脱敏**：`network-log.json` 中 29 处凭证值替换为 `[REDACTED]`，145 条记录与全部 Network 事件结构保留；当前树全仓扫描（[1.68]）无自动可判定的真实凭证字面量。
2. **泄露凭证已失效**：对应会话已撤销、本地一次性测试密钥已轮换；用该令牌请求 `GET /api/v1/users/me` 实测返回 401「登录已失效，请重新登录」。该令牌与生产实例密钥无关，生产是否受影响仅按可验证证据表述，不外推。
3. **旧提交对象仍可公开读取**：`259bd11` 曾推送到 Public 仓库，经 GitHub commit API 与 raw 文件接口按 SHA 实测仍为 HTTP 200，旧版日志仍含凭证字面量；远端 refs 中无指向该对象的活动分支，但 force-push 与本地 `gc` **不等于**远端对象被清除。
4. **远端历史对象清理待 Owner 决策**：未获 Owner 明确批准，不再次 force-push、不重写公开历史、不删除 refs、不切换仓库可见性、不代 Owner 联系 GitHub Support。影响面、协作者风险、回滚与验证方案见根 `CHANGELOG.md` [1.68] 与 `docs/FastLedger-R0-第三次整改交付回执.md`。
5. **复现必须继续脱敏**：`tools/r0/redaction.js` 在写盘前统一处理请求头、响应头、`headersText`、Cookie、`Set-Cookie`、URL 查询参数、JSON 请求体与 JWT 字面量；`test-redaction.js` 用合成假凭证做自动化断言，脚本另做「原始令牌不得出现在输出」自检。**禁止关闭该逻辑**。
6. 单次本机实测、账本 `id=1`、`transactions total=0`、仅 2 种失败注入、仅 1 次恢复；未覆盖慢速网络、有数据态、权限不足、离线 / 同步冲突、多角色、原型同视口对照。
7. 后端二进制构建于 `2026-11-04 01:13:59 +0800`，**其对应代码 SHA 未独立验证**；实测 HTTP 提供的 `index.html` / `app.js` 与仓库 HEAD 哈希一致、`style.css` 仅 CRLF 差异（见 `__META__.servedAssetSha256`）。
8. 须 Owner/CI 独立重跑并补齐上述缺口后，方可讨论 R0-E-35 是否关闭。**R0 未通过 / 阻塞，不授权 R1。**
