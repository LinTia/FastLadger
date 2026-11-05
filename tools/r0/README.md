# tools/r0 — R0 运行时证据工具（RV-05 / RV-06）

本目录提供**可提交、可移植、可独立复现**的 R0-RV-01 隔离复验工具，对应 `docs/FastLedger-R0-第三次最小整改任务单-RV-02-RV-05-RV-06.md` 的 RV-05 / RV-06 要求。归档证据本身在 `docs/r0-screenshots/runtime/`，本目录只放脚本与说明，**不含任何真实凭证、口令、profile、数据库、浏览器缓存或未脱敏日志**。

| 文件 | 作用 |
|---|---|
| `redaction.js` | 统一脱敏模块。写盘前处理 CDP 事件的请求头/响应头/`headersText`/`Cookie`/`Set-Cookie`/`Authorization`/`Proxy-Authorization`/URL 查询参数/JSON 请求体/`postData`/JWT 字面量/`Bearer` 任意令牌；只替换值，保留事件结构 |
| `test-redaction.js` | `node:test` 自动化脱敏测试，全部使用**合成假凭证**，断言输出不含原始值、事件结构保留、幂等、不误伤描述性字段 |
| `scan-credentials.js` | 全仓凭证安全扫描器，只报告文件路径 / 命中类型 / 数量 / 脱敏上下文；`--strict` 对自动可判定的凭证字面量判定 |
| `rv01-isolation.js` | Edge/Chromium headless + 原生 CDP 隔离复验脚本；默认输出到仓库外临时目录 |
| `extract-prototype-dataset.js` | 从原型打包产物提取原生演示数据集（accounts/categories/transactions/budgets/family/profile）→ `tools/r0/prototype-dataset.json`，含 bundle SHA256，保证"同一数据集"可复现 |
| `seed-from-dataset.js` | 按该数据集通过公开 API 把实现侧空账本种成同一份数据（先清空建账本时自动播种的默认账户/分类），并回读派生余额生成原型注入包；输出到临时目录，不入库 |
| `capture-pairs.js` | 13 页 × 4 视口成对采集：支持同数据集注入（运行时探测 toolkit 命名空间前缀）、动作级采集（未登录态 / 记一笔弹层）；manifest 记录 `state`/`action`/`hasToken`/`dataset` |

## 1. 包 B 同数据集成对采集（三步）

```bash
# 1) 提取原型原生数据集（离线）
node tools/r0/extract-prototype-dataset.js

# 2) 在已启动的隔离实例上种子化（需空库 + 实例用户）
R0_BASE=http://127.0.0.1:8091 R0_USER=admin R0_PW=*** \
  node tools/r0/seed-from-dataset.js

# 3) 两侧采集（原型无需后端；实现侧需 R0_TOKEN）
R0_SIDES=proto R0_PROTO_DIR=./prototype \
R0_PROTO_INJECT=$TMP/fastledger-r0-pairs-runtime/proto-injection.json \
  node tools/r0/capture-pairs.js

R0_SIDES=impl R0_APP_URL=http://127.0.0.1:8091/ R0_TOKEN=*** \
R0_BOOK_ID=1 R0_TX_ID=1 \
  node tools/r0/capture-pairs.js
```

踩过的坑（脚本已内置处理，改动前请先读）：

1. 原型数据层键名带 toolkit 命名空间前缀（`__miaoda_<appid>__:`），且**未登录时不会建键**——脚本先触发一次演示登录拿到前缀再清空正式采集，不硬编码。
2. 实现侧是 hash 路由，`Page.navigate` 到 `#...` 属同文档跳转、注入脚本不执行——脚本加一次性 query 强制新建文档。
3. 实现侧建账本会自动播种默认账户/分类——种子脚本先删默认播种，否则两侧账户数不一致。

## 2. 依赖与前提

- **Node >= 22**（使用内置 `WebSocket` / `fetch` / `node:test`，无第三方 npm 依赖）。
- **Edge 或 Chromium**（headless + CDP）。Windows 必须通过 `R0_EDGE_PATH` 显式指定；Linux/macOS 使用脚本内置的系统级候选路径。
- **已启动的 FastLedger 后端**，且 `/healthz` 返回 `up`；前端由后端 `go:embed dist` 同源提供，无需独立前端进程。
- **一个可用测试账号与账本**：需能登录并在 `book id = R0_BOOK_ID`（默认 1）下读取交易列表。账号准备方式见 §3，口令只经环境变量传递。

## 3. 环境变量

| 变量 | 必需 | 默认 | 说明 |
|---|---|---|---|
| `R0_TOKEN` | **是** | — | 登录接口 `data.accessToken`。缺失时脚本以退出码 2 安全失败，**不打印、不落盘** |
| `R0_EDGE_PATH` | Windows 必需 | 系统候选路径 | 浏览器可执行文件绝对路径（本地环境，不写入仓库） |
| `R0_WHOAMI` | 否 | `test-user` | 注入 `localStorage.fl_whoami` 的显示名（非凭证） |
| `R0_BOOK_ID` | 否 | `1` | 目标账本 id，用于精确匹配交易列表请求路径 |
| `R0_BASE_URL` | 否 | `http://127.0.0.1:8080/` | 后端根 URL |
| `R0_CDP_PORT` | 否 | `9231` | CDP 调试端口 |
| `R0_OUT_DIR` | 否 | 系统临时目录 `fastledger-r0-rv01-output` | 输出目录，**默认在仓库外** |
| `R0_PROFILE_DIR` | 否 | 系统临时目录 `fastledger-r0-rv01-profile` | 浏览器临时 profile；用户显式指定时脚本不会删除它 |
| `R0_WRITE_EVIDENCE` | 否 | 未设置 | 设为 `1` 才考虑写正式证据目录（危险开关） |
| `R0_WRITE_EVIDENCE_CONFIRM` | 条件必需 | — | 与上一项同时为 `YES` 才允许写入 `docs/r0-screenshots/runtime/` |
| `FL_TEST_USER` / `FL_TEST_PASSWORD` | 准备账号时用 | — | 本地私有测试账号，**仅在 shell 环境中，不写入仓库** |

## 4. 从仓库根目录执行

```bash
# 0) 本地私有配置（不要写进任何仓库文件）
export FL_TEST_USER='<你的测试用户名>'
export FL_TEST_PASSWORD='<你的测试口令>'

# 1) 构建/准备后端并用一次性随机密钥启动（二进制名按平台调整）
export EBK_JWT_SECRET="$(python3 -c 'import secrets,base64;print(base64.b64encode(secrets.token_bytes(48)).decode())')"
export EBK_DB_PATH="$PWD/server/.localdata/fastledger.db"
export EBK_HTTP_PORT=8080 EBK_ENABLE_REGISTER=true
(cd server && ./fastledger.exe db migrate)
(cd server && ./fastledger.exe user create --username "$FL_TEST_USER" --password "$FL_TEST_PASSWORD" --owner)
(cd server && ./fastledger.exe) &
for i in $(seq 1 20); do curl -sf http://127.0.0.1:8080/healthz >/dev/null && break; sleep 1; done

# 2) 登录并只把 accessToken 放进环境变量
LOGIN_JSON=$(python3 -c 'import os,json;print(json.dumps({"username":os.environ["FL_TEST_USER"],"password":os.environ["FL_TEST_PASSWORD"]}))')
export R0_TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login -H 'Content-Type: application/json' -d "$LOGIN_JSON" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["accessToken"])')

# 3) 浏览器路径（Windows 必填；值为本地环境路径，不入库）
export R0_EDGE_PATH='/path/to/msedge.exe'

# 4) 安全测试：合成假凭证脱敏断言
node --test tools/r0/test-redaction.js
node tools/r0/scan-credentials.js --strict

# 5) 隔离复验：默认输出到临时目录，打印每个产物的字节数与 SHA256
node tools/r0/rv01-isolation.js

# 6) 如需生成 Markdown 扫描报告（可复核证据）
node tools/r0/scan-credentials.js --out docs/r0-screenshots/security-scan.md
```

**干净 clone 说明**：仓库内不含本机的 `.tools/`（便携 Go/MinGW 工具链）、`server/.localdata/`（本地库）或浏览器 profile。若这些外部依赖缺失，脚本会在**配置检查阶段**以退出码 2 明确失败（列出缺失变量名），或由后端启动步骤报错；这属于「因明确缺失的外部依赖安全失败」，不会伪造证据。

## 5. 退出码

| 码 | 含义 |
|---:|---|
| 0 | 成功（扫描器 `--strict` 无真实凭证候选 / 测试全通过 / 采集完成） |
| 1 | 采集运行错误（CDP / 浏览器 / 服务异常）；已采集的真实事件仍经脱敏后落盘，便于排查 |
| 2 | 配置缺失（`R0_TOKEN` / 浏览器路径），或 `--strict` 发现真实凭证候选 |
| 3 | 危险开关未确认：设置了 `R0_WRITE_EVIDENCE=1` 但没有 `R0_WRITE_EVIDENCE_CONFIRM=YES` |
| 4 | 脱敏自检失败：序列化日志中仍出现原始令牌，脚本**中止写入** |

## 6. 输出、清理与平台限制

- 默认输出：系统临时目录下的 `fastledger-r0-rv01-output/`，含 5 张 PNG 与 `network-log.json` / `console-log.json` / `dom-snapshots.json`。脚本打印每个文件的字节数与 SHA256，供第三方核对。
- 正式证据目录 `docs/r0-screenshots/runtime/` **默认只读**；任何情况下不要为了复现直接重跑写入它。确需重采时显式设置两个危险开关，并先备份 9 个归档文件。
- 清理：脚本结束时关闭 CDP 连接、终止浏览器进程，并删除自己创建的临时 profile；输出目录与用户显式指定的 profile 目录由使用者自行删除。
- **平台**：仅在 Windows 11 + Git Bash + Node `v22.22.2` + Edge `156.0.4314.58` 实测。Linux/macOS 未实测；脚本内置的系统级浏览器候选与基于 `path`/`os.tmpdir()` 的路径推导可移植，但启动后端的命令与二进制名需按平台调整，**不声称跨平台**。

## 7. 安全不变量

1. `redaction.js` 的 `scrub()` 是唯一写盘入口的前置处理；`rv01-isolation.js` 另有「原始令牌不得出现在序列化输出」自检（退出码 4）。
2. 日志只保留事件结构（方法、URL 路径、状态码、时间戳、`requestId`），凭证值一律 `[REDACTED]`。
3. 合成假凭证测试（`test-redaction.js`）覆盖 `Network.*` / `Fetch.requestPaused` / `Runtime.*` / `Log.entryAdded` 四类事件、嵌套对象、数组、多行 `headersText`、URL 查询参数与 JSON 请求体。
4. `scan-credentials.js` 的 `--strict` 只自动判定 JWT、Bearer 令牌、真实头值与 JSON 密码字段；代码赋值表达式、测试夹具与历史示例仍全部列出供人工/Owner 复核，不自动判为泄露，也不自动忽略。
5. 归档证据与凭证安全事件的状态拆分（当前树脱敏 / 凭证失效 / 旧 SHA 仍可读取 / 远端清理待 Owner 决策）见 `docs/r0-screenshots/runtime/README.md` 与根 `CHANGELOG.md` [1.66]、[1.68]。
