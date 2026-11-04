# FastLedger R0 凭证安全扫描报告

- 生成时间：2026-11-04T15:28:33.447Z
- 仓库根：<repo-root>（由脚本位置推导，不记录绝对路径）
- 扫描文本文件：449；跳过二进制/超大文件：82
- `--strict`：启用
- **真实凭证候选命中：0**（参与 --strict）；合成/占位：48；人工复核：14；测试夹具：27；可移植性：18

> 本报告只记录文件路径、命中类型、数量与脱敏上下文，不记录任何敏感值。
> `--strict` 仅对自动可判定的凭证字面量（JWT / Bearer / 头值 / JSON 密码字段）判定；
> 代码赋值表达式与测试夹具仍全部列出，供人工/Owner 复核，不自动判为泄露。

| 文件 | 命中类型 | 数量 | 判定 | 脱敏上下文 |
|---|---|---:|---|---|
| `.gotmp/srv2.log` | PERSONAL_ABS_PATH | 1 | portability | 2026/10/31 12:46:45 数据库已初始化：[PERSONAL_PATH]/WorkBuddy/fastledger/repo_tmp/.gotmp/finalcheck/data/f.db（WAL / foreign_keys=on / synchronous=FULL / MaxOpenConns=1） |
| `client/lib/api/api_client.dart` | AUTH_HEADER | 1 | synthetic | /// - 统一携带 `Authorization: [REDACTED:LEN=8] |
| `deploy/docker-compose.yml` | ASSIGNED_SECRET_LITERAL | 1 | synthetic | EBK_JWT_SECRET: "[REDACTED:LEN=60]" |
| `docs/design-tokens.json` | SENSITIVE_FILENAME | 1 | portability | [文件名匹配 token\|secret\|credential\|.env 规则] |
| `docs/FastLedger-部署交接-腾讯云轻量应用服务器.md` | ASSIGNED_SECRET_LITERAL | 1 | review | export EBK_JWT_SECRET="[REDACTED:LEN=26]"   # ≥32 字符，否则拒绝启动 |
| `docs/FastLedger-交接文档-2026-11-02.md` | ASSIGNED_SECRET_LITERAL | 1 | review | EBK_JWT_SECRET="[REDACTED:LEN=35]" \ |
| `docs/FastLedger-交接文档-2026-11-02.md` | PERSONAL_ABS_PATH | 10 | portability | \| 本地仓库 \| `[PERSONAL_PATH] 根在这层**，`server/` 是 Go module 根） \| |
| `docs/FastLedger-新手开发完整指南.md` | JSON_SECRET_FIELD | 1 | synthetic | { "username": "me", "password": "[REDACTED:LEN=15]" } |
| `docs/FastLedger-R0-第三次最小整改任务单-RV-02-RV-05-RV-06.md` | PERSONAL_ABS_PATH | 1 | portability | - runtime README 的复现命令仍依赖 `[PERSONAL_PATH]/WorkBuddy/fastledger` 等个人绝对路径。 |
| `docs/FastLedger-R0-独立复核补充-RV-05至06.md` | PERSONAL_ABS_PATH | 1 | portability | \| `quote` \| 任务单 §3.1：「runtime README 的复现命令仍依赖 `[PERSONAL_PATH]/WorkBuddy/fastledger` 等个人绝对路径」「实际运行脚本 `.tmp_shot/rv01-isolation.js` 未提交」「QA 两轮独立复跑和 SHA256 前后 0 变 |
| `docs/FastLedger-R0-基准冻结与差异盘点.md` | AUTH_HEADER | 1 | synthetic | - **鉴权分组**：公开 v1 组 `register/login/join`(router.go:124/125/128)；`GET /api/v1/ws` 用独立 `wsAuth(d.AuthService,d.PATs)`(router.go:133，刻意不挂 JWTAuth 组，注释 130-133，浏览器 |
| `docs/r0-screenshots/README.md` | PERSONAL_ABS_PATH | 1 | portability | \| 采集脚本 \| `[PERSONAL_PATH]/WorkBuddy/fastledger/.tmp_shot/capture.js`，用法 `node capture.js <token>`，输出 `.tmp_shot/manifest.json` \| `capture.js` L2–3、L154 \| |
| `docs/r0-screenshots/runtime/network-log.json` | AUTH_HEADER | 29 | synthetic | "Authorization": "[REDACTED:LEN=10]", |
| `docs/r0-screenshots/runtime/README.md` | ASSIGNED_SECRET_LITERAL | 1 | synthetic | export FL_TEST_PASSWORD='[REDACTED:LEN=8]' |
| `docs/r0-screenshots/runtime/README.md` | ASSIGNED_SECRET_LITERAL | 1 | review | export EBK_JWT_SECRET="[REDACTED:LEN=13]'import secrets,base64;print(base64.b64encode(secrets.token_bytes(48)).decode())')" |
| `prototype/assets/toolkit-BzP8JQh1.js` | COOKIE_HEADER | 1 | synthetic | `)}getSetCookie(){let e=this.get(`set-cookie`);return V.isArray(e)?e:e==null\|\|e===!1?[]:[e]}get[Symbol.toStringTag](){return`AxiosHeaders`}static from(e){return |
| `server/.localdata/server.log` | PERSONAL_ABS_PATH | 1 | portability | 2026/11/04 01:14:49 数据库已初始化：[PERSONAL_PATH]/WorkBuddy/fastledger/repo_tmp/server/.localdata/fastledger.db（WAL / foreign_keys=on / synchronous=FULL / MaxOpenConn |
| `server/internal/api/auth_handler_a6_test.go` | JSON_SECRET_FIELD | 4 | test-fixture | `{"username":"alice","password":"[REDACTED:LEN=8]"}`) |
| `server/internal/api/auth_handler_r45_test.go` | JSON_SECRET_FIELD | 4 | test-fixture | `{"username":"alice","password":"[REDACTED:LEN=8]"}`) |
| `server/internal/api/auth_handler_r46_test.go` | ASSIGNED_SECRET_LITERAL | 4 | review | secret:   "[REDACTED:LEN=33]", |
| `server/internal/api/auth_handler_r46_test.go` | JSON_SECRET_FIELD | 3 | test-fixture | `{"username":"alice","password":"[REDACTED:LEN=14]"}`) |
| `server/internal/api/budget_handler_test.go` | JSON_SECRET_FIELD | 2 | test-fixture | if w := do(http.MethodPost, "/api/v1/auth/register", `{"username":"budgeter","password":"[REDACTED:LEN=11]"}`, ""); w.Code != http.StatusOK { |
| `server/internal/api/invest_handler_test.go` | JSON_SECRET_FIELD | 2 | test-fixture | if w := do(http.MethodPost, "/api/v1/auth/register", `{"username":"investor","password":"[REDACTED:LEN=11]"}`, ""); w.Code != http.StatusOK { |
| `server/internal/api/middleware.go` | AUTH_HEADER | 1 | synthetic | // wsAuth WebSocket 专用鉴权中间件：**优先读 Authorization: [REDACTED:LEN=10] ?token=**。 |
| `server/internal/api/ocr_handler_test.go` | JSON_SECRET_FIELD | 2 | test-fixture | if w := post("/api/v1/auth/register", `{"username":"`+name+`","password":"[REDACTED:LEN=11]"}`); w.Code != http.StatusOK { |
| `server/internal/api/report_handler_test.go` | JSON_SECRET_FIELD | 2 | test-fixture | if w := do(http.MethodPost, "/api/v1/auth/register", `{"username":"reporter","password":"[REDACTED:LEN=11]"}`); w.Code != http.StatusOK { |
| `server/internal/api/session_api_test.go` | JSON_SECRET_FIELD | 2 | test-fixture | `{"username":"`+name+`","password":"[REDACTED:LEN=11]"}`, "", "") |
| `server/internal/api/token_handler_test.go` | JSON_SECRET_FIELD | 2 | test-fixture | if w := e.do(http.MethodPost, "/api/v1/auth/register", `{"username":"patuser","password":"[REDACTED:LEN=11]"}`, ""); w.Code != http.StatusOK { |
| `server/internal/authzreview/authz_login_review_test.go` | JSON_SECRET_FIELD | 1 | test-fixture | body := `{"username":"` + user + `","password":"[REDACTED:LEN=12]"}` |
| `server/internal/authzreview/authz_review_test.go` | JSON_SECRET_FIELD | 1 | test-fixture | `","password":"[REDACTED:LEN=11]"}` |
| `server/internal/authzreview/zz_authz_adversarial_test.go` | JSON_SECRET_FIELD | 1 | test-fixture | body := `{"username":"` + user + `","password":"[REDACTED:LEN=12]"}` |
| `server/internal/authzreview/zz_authz_adversarial2_test.go` | JSON_SECRET_FIELD | 1 | test-fixture | body := `{"code":"deadbeefdeadbeefdeadbeefdeadbeef","username":"` + user + `","password":"[REDACTED:LEN=11]"}` |
| `server/internal/config/config_r7_test.go` | ASSIGNED_SECRET_LITERAL | 1 | review | const r7Secret = "[REDACTED:LEN=43]" |
| `server/internal/service/audit_integration_test.go` | ASSIGNED_SECRET_LITERAL | 1 | review | const testJWTSecret = "[REDACTED:LEN=39]" |
| `server/internal/service/jwt_test.go` | ASSIGNED_SECRET_LITERAL | 1 | review | const testSecret = "[REDACTED:LEN=53]" |
| `server/internal/service/ocr_llm_test.go` | ASSIGNED_SECRET_LITERAL | 2 | review | LLMBaseURL: srvURL, LLMAPIKey: "[REDACTED:LEN=8]", LLMModel: "test-model", LLMRatePerMin: 100, |
| `server/README.md` | ASSIGNED_SECRET_LITERAL | 1 | review | export EBK_JWT_SECRET="[REDACTED:LEN=26]"   # ≥32 位，否则拒绝启动 |
| `tools/r0/README.md` | ASSIGNED_SECRET_LITERAL | 1 | synthetic | export FL_TEST_PASSWORD='[REDACTED:LEN=8]' |
| `tools/r0/README.md` | ASSIGNED_SECRET_LITERAL | 1 | review | export EBK_JWT_SECRET="[REDACTED:LEN=13]'import secrets,base64;print(base64.b64encode(secrets.token_bytes(48)).decode())')" |
| `tools/r0/test-redaction.js` | ASSIGNED_SECRET_LITERAL | 2 | synthetic | const FAKE_PASSWORD = '[REDACTED:LEN=27]'; |
| `tools/r0/test-redaction.js` | COOKIE_HEADER | 7 | synthetic | const FAKE_COOKIE = '[REDACTED:LEN=55]'; |
| `tools/r0/test-redaction.js` | JSON_SECRET_FIELD | 1 | synthetic | `{"username":"fake_user","password":"[REDACTED:LEN=16]"}`, |
| `tools/r0/test-redaction.js` | JWT_LITERAL | 1 | synthetic | const FAKE_JWT = '[REDACTED:LEN=60]'; |
| `workbuddy/FastLedger-文档评审-C.md` | PERSONAL_ABS_PATH | 1 | portability | > 评审基线：本地克隆仓库 `[PERSONAL_PATH]/WorkBuddy/fastledger/repo_tmp`（相当于 `main` 最新快照） |
| `workbuddy/FastLedger-GPT二次核查卡片.md` | PERSONAL_ABS_PATH | 1 | portability | export PATH="[PERSONAL_PATH]/WorkBuddy/fastledger/.tools/go/bin:$PATH" |
