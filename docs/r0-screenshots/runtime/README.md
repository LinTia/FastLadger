# R0-RV-01 隔离运行时复验证据（2026-11-04）

真实 Edge headless + CDP 运行时证据，非静态文档；对应 `docs/FastLedger-R0-独立复核补充-RV-01至04.md` RV-01。
**R0-E-35 仍为「待运行时复验 / 阻塞」；本次实测不关闭缺陷，R0 未通过、不授权 R1。**

## 环境
- 浏览器 `Edg/156.0.4314.58`（`--headless=new`，CDP `/json/version` 实测），Node v22.22.2 内置 WebSocket。
- 视口 390×844（`mobile=true`）、1280×800（`mobile=false`），`Emulation.setDeviceMetricsOverride`。
- 文档 HEAD `557e4d223064c149cdf1063429a2fdd79158e609`；服务 `:8080`，本地库 `server/.localdata/fastledger.db`（未删改数据）。
- 登录 `POST /api/v1/auth/login` `{admin,admin123456}`，取 `data.accessToken` 存工作区 `.tmp_shot/token.txt`。
- 注入 `Page.addScriptToEvaluateOnNewDocument` → `localStorage.setItem("fl_token",TOKEN)` + `fl_whoami="admin"`。

## 拦截规则（与上一轮泛化拦截的核心区别）
- `Fetch.enable({patterns:[{urlPattern:"*/api/v1/books/1/transactions*",requestStage:"Request"}]})`。
- `Fetch.requestPaused` 内再严格判定 `method==="GET"` 且 `URL.pathname==="/api/v1/books/1/transactions"`；其余一律 `Fetch.continueRequest`。
- 放行 `GET /books`、`/books/1/accounts`、`/books/1/categories`、`/books/1/reports/monthly?months=2` 及静态资源（见 `network-log.json`）。
- 实测目标 URL：`http://127.0.0.1:8080/api/v1/books/1/transactions?page=1&count=60`。

## 精确复现

> **覆盖风险（先读）**：本目录 5 PNG + 3 JSON 是已归档证据；原始脚本 `.tmp_shot/rv01-isolation.js` 的 `OUT` 硬编码指向本目录，**直接运行会覆盖证据**。下面代码块默认走「工作区脚本副本 + 仓库外输出目录」，逐字照抄不会向本目录写入任何文件；QA 已用此方式复现，9 个交付文件 SHA256 前后 0 变化。

```bash
source /c/Users/47006/WorkBuddy/fastledger/.tools/genv.sh   # 内部已 cd 到 repo_tmp/server，勿再 cd
export EBK_JWT_SECRET=$(python3 -c 'import secrets,base64;print(base64.b64encode(secrets.token_bytes(48)).decode())')
export EBK_DB_PATH="C:/Users/47006/WorkBuddy/fastledger/repo_tmp/server/.localdata/fastledger.db"
export EBK_HTTP_PORT=8080 EBK_ENABLE_REGISTER=true
./fastledger.exe &
for i in $(seq 1 20); do curl -sf http://127.0.0.1:8080/healthz >/dev/null && break; sleep 1; done
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123456"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["accessToken"])')
printf '%s' "$TOKEN" > /c/Users/47006/WorkBuddy/fastledger/.tmp_shot/token.txt
S=/c/Users/47006/WorkBuddy/fastledger/.tmp_shot
cp "$S/rv01-isolation.js" "$S/qa-rv01-isolation.js"
sed -i "s#path.join(REPO, 'docs', 'r0-screenshots', 'runtime')#path.join(WS, '.tmp_shot', 'qa-repro-output')#" "$S/qa-rv01-isolation.js"
sed -i "s#path.join(WS, '.tmp_shot', 'profile-rv01')#path.join(WS, '.tmp_shot', 'profile-qa-rv01')#" "$S/qa-rv01-isolation.js"
grep -nE "const (OUT|PROFILE)" "$S/qa-rv01-isolation.js"
node "$S/qa-rv01-isolation.js"
```

脚本副本 `qa-rv01-isolation.js`、输出目录 `.tmp_shot/qa-repro-output/` 与浏览器 profile 均在工作区（不入库）；原始脚本仅作模板、保持不变。**本代码块不向 `docs/r0-screenshots/runtime/` 写入任何文件。**

## 文件与状态
| 文件 | 状态 | 动作 |
|---|---|---|
| `01-pending-390x844.png` | pending | 暂停请求不 continue/fail，1.8s 截图 |
| `02-rejected-network-390x844.png` | rejected | `Fetch.failRequest(Failed)` 后 0.9s |
| `03-rejected-503-390x844.png` | rejected | `Fetch.fulfillRequest(503,JSON)` 后 0.9s |
| `04-pending-1280x800.png` | pending | 桌面视口持请求 |
| `05-recovered-390x844.png` | 恢复 | `Fetch.disable` 后重新导航，1.8s |
| `network-log.json` | 144 条真实 CDP 事件 + 1 条脚本生成 `__META__` 记录 | **已脱敏**：`Authorization`/`Cookie` 等请求头值统一替换为 `[REDACTED]`（见根 `CHANGELOG.md` [1.66]）；`Fetch.requestPaused`/`Network.*` 为真实事件，`__META__` 为脚本生成的采集元数据、**非** CDP 事件 |
| `console-log.json` | 5 条（4 × `Log.entryAdded` + `__META__`） | `Runtime.consoleAPICalled`/`exceptionThrown`/`Log.entryAdded` |
| `dom-snapshots.json` | 5 组 `Runtime.evaluate` 实测 | `.skel` 数/`aria-busy`/toast/重试控件/文本 |

## 本次实测观察（如实记录，不外推）
- **pending**（2 视口）：`.skel`=4、`aria-busy=true` 容器 1、toast 隐藏；概览三卡已渲染 → 依赖请求确实全部放行。
- **rejected**（网络失败 + 503，共 2 次）：`.skel` 仍=4、`aria-busy=true` 未清除；仅红色 toast（`✕ Failed to fetch` / `✕ 模拟故障：账单列表服务暂不可用（R0-RV-01 隔离注入）`）；DOM 中**无「重试/刷新」控件**（`retryControls=[]`）。
- **恢复**：解除拦截并重新导航后 `.skel`=0、toast 隐藏、显示空态「还没有账单——点下方「＋ 记一笔」开始记录。」→ 整页重载可恢复，但页面内无自助重试入口。
- 日志：`Network.loadingFailed net::ERR_FAILED` 3 次、503 `Network.responseReceived` 1 次；`Runtime.exceptionThrown`/`consoleAPICalled` 均 0（异常被 `catch` 吞掉，仅 toast）。
- 与 §7.5 旧截图的冲突得到解释：旧轮泛化拦截 `/api/v1/*` 同时打断 `loadBookMeta` 等依赖，`renderHome` 未走到 `renderBillStream`，故「页面主体空白」；本轮隔离后骨架屏残留可见。**不等于缺陷已关闭**。

## 证据边界
- **凭证安全（[1.66]）**：本目录日志曾含会话令牌，现已彻底脱敏并重写 Git 历史；对应会话已撤销、本地测试密钥已轮换。复现脚本 `.tmp_shot/rv01-isolation.js` 已加 `scrub()`，写入日志前自动剔除请求头中的凭证。**后续重新采集时请勿关闭该脱敏逻辑。**
- 单次本机实测、账本 id=1、`transactions total=0`、仅 2 种失败注入、仅 1 次恢复；未覆盖慢速网络、有数据态、权限/离线/同步冲突、多角色、原型同视口对照。
- 后端二进制构建于 `2026-11-04 01:13:59 +0800`，**其对应代码 SHA 未独立验证**；实测 HTTP 提供的 `index.html`/`app.js` 与仓库 HEAD 哈希一致、`style.css` 仅 CRLF 差异（见 `__META__.servedAssetSha256`）。
- 须 Owner/CI 独立重跑并补齐上述缺口后，方可讨论 R0-E-35 是否关闭。**R0 未通过 / 阻塞，不授权 R1。**
