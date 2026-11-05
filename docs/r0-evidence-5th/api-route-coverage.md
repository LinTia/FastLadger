# R0-D 路由测试覆盖复核（第五次整改，2026-11-05）

> 基准：业务代码 `bff6a2b351bf5a039cdf7c762ec2693fb330cd00`（`server/` 自该提交起零改动）。
> 方法：逐条回到 `server/**/*_test.go` 实读，按"HTTP 层是否有独立命名用例 + 是否断言关键路径"判定，**不以 `go test ./...` 全绿或 CI success 代替路由覆盖**。
> 结论口径：`覆盖` = HTTP 层有命名用例且断言成功/失败关键路径；`部分` = 仅 service 层覆盖，或仅覆盖参数解析/错误分支；`未覆盖` = 仅出现在路由注册清单中，无行为断言。

## 1. 逐条结论（13 条）

| # | 路由 | handler | 权限/角色守卫 | DTO（请求/响应） | 错误语义 | 现有测试与断言位置 | 判定 |
|---|---|---|---|---|---|---|---|
| 1 | `GET /healthz` | `router.go` 内联匿名 handler（L109–120） | 公开、无鉴权 | 无请求；`{status,schemaVersion,instanceId?,restoreEpoch?}` | — | `TestQAS3HealthzReportsInstanceMeta`（`api/qa_stage3_test.go:30-46`，断言 200 + instanceId + restoreEpoch）；`TestQAS3HealthzOmitsGenerationWhenUnknown`（同文件 `:52+`，断言未注入时省略字段） | **覆盖** |
| 2 | `GET /api/v1/users/me` | `auth_handler.go` getMe | JWTAuth | — / `currentUserDTO{id,username,email,isInstanceAdmin}` | 401（令牌无效/撤销） | `session_api_test.go:286`（重新登录后 `Code==0`）、`:297-300`（`authedGet` 断言旧令牌 401）；DTO 字段仅 `token_handler_test.go:122` 间接断言 username | **覆盖**（字段断言偏弱） |
| 3 | `PUT /api/v1/users/me` | `auth_handler.go` updateMe | JWTAuth | `{email}` / `currentUserDTO` | 409 邮箱占用、400 参数非法 | 仅 service `TestUpdateEmail`（`service/profile_test.go:13`）；**HTTP 层无用例** | **部分** |
| 4 | `POST /api/v1/users/me/password` | `auth_handler.go` changePassword | JWTAuth | `{oldPassword,newPassword}` / `{revokedSessions,message}` | 400 弱口令/过长/非法；401 旧密码错 | 仅 service `TestChangePassword`（`service/profile_test.go:57`）；响应体无断言，HTTP 层无用例 | **部分** |
| 5 | `GET /api/v1/books` | `book_handler.go` list | JWTAuth（SQL 按成员过滤，无 guard 函数） | — / `[]model.Book`（空数组非 null） | 401 | 无 GET 断言；仅 `POST /api/v1/books` 作为夹具（`budget_handler_test.go:91` 等） | **未覆盖** |
| 6 | `GET /api/v1/books/:id` | `book_handler.go` get | JWTAuth + `RoleOf` 成员复核 | 路径 `id` / `*model.Book` | 404 账本不存在、403 非成员 | 无任何用例命中该单资源 GET | **未覆盖** |
| 7 | `GET /api/v1/books/:id/members` | `book_handler.go` listMembers | JWTAuth + `RoleOf` | 路径 `id` / `[]MemberDTO{uid,username,role,canViewOthers,isInstanceAdmin}` | 403 非成员、409 账本已删 | 仅 `members/join`、`members/invite/:id/revoke` 有用例（`invite_lifecycle_api_test.go:59,106`）；**列表无用例** | **未覆盖** |
| 8 | `GET /api/v1/books/:id/accounts/:aid` | `ledger_handler.go` getAccount | `requireMember` | 路径 `aid` / `*model.Account` | 404 账户不存在、403 非成员 | `qa_api_test.go:119-139` 仅断言 `pathID` 的 400 与解析（非法值 6 例 + 合法 12）；**handler 200/404 未断言** | **部分** |
| 9 | `DELETE /api/v1/books/:id/accounts/:aid` | `ledger_handler.go` deleteAccount | `requireManagerTx`（owner/admin） | 路径 `aid` / `{id}` | 409 有交易/有子账户、404 | HTTP 层无用例；service 层 `TestR31AccountDeleteBlockedBySoftDeletedTx`（`remediation_integration_test.go:594`）、`TestR31AccountDeleteBlockedByTransferEitherSide`（`:627`）、`TestR53AccountDeleteRejectsActiveChildren`（`remediation_unit_test.go:501`）覆盖 409 约束 | **部分** |
| 10 | `GET /api/v1/books/:id/categories`（含 `?tree=1`） | `category_handler.go` listCategories | `requireMember` | query `tree` / `[]model.Category` 或 `[]CategoryTreeNode` | 403、404 | 仅 `reports/categories` 有用例（`report_handler_test.go:169,208,228`）；**分类列表与 `?tree=1` 无用例** | **未覆盖** |
| 11 | `DELETE /api/v1/books/:id/categories/:cid` | `category_handler.go` deleteCategory | `requireManagerTx` | 路径 `cid` / `{id}` | 409 有交易/有子类、404 | HTTP 层无用例；service `TestR31CategoryDeleteBlockedBySoftDeletedTx`（`remediation_integration_test.go:657`）覆盖 409 引用约束 | **部分** |
| 12 | `GET /api/v1/books/:id/transactions/:txId` | `tx_handler.go` get | JWTAuth + `canViewAllForTx`/`txVisibilityCond` | 路径 `txId` / `*model.Transaction`（无 merchant 字段） | 404（刻意不泄露存在性）、403 | 仅出现在路由注册清单（`router_test.go:25`、`qa_api_test.go:275`）；`tx_c04_replay_broadcast_test.go:267` 是 DELETE+query，不是详情 GET | **未覆盖** |
| 13 | `POST /api/v1/books/:id/instruments/:instrumentId/price` | `invest_handler.go` setPrice | `requireManagerTx` | `{price:string,priceTime:int64}` / `{updated:true}` | 400 价格非法、404 标的不存在、403 | `invest_handler_test.go:167` 走完整 HTTP 流程断言；service `TestSetPriceRefreshesHoldings`（`service/invest_test.go:103`） | **覆盖** |

汇总：**覆盖 3 / 部分 5 / 未覆盖 5 = 13**。

## 2. 与主报告旧口径的差异（必须在报告里更正）

| 路由 | 主报告 §5.3.1 旧结论 | 本轮实测 | 依据 |
|---|---|---|---|
| `GET /healthz` | 「未覆盖：未找到独立命名测试」 | **覆盖** | `api/qa_stage3_test.go:30-46`、`:52+` 两个命名用例，断言 200 + 字段与省略行为 |
| `GET /users/me` | 「未覆盖（仅 PAT 间接断言）」 | **覆盖**（字段断言弱） | `session_api_test.go:286`（成功）、`:297-300`（401） |
| `GET /accounts/:aid` | 「未覆盖」 | **部分**（仅 pathID 解析） | `qa_api_test.go:119-139` |
| 其余 10 条 | 与本表一致 | 同表 | 同上 |

## 3. 本轮处置（方案 B：不补测试）

任务单允许二选一。本轮选 **B：保留"未覆盖/部分"，不新增测试**，理由：

1. 新增 HTTP 级用例需同时覆盖成功/未授权/无权限/不存在/非法输入五类边界，属**测试代码实现**，与"本轮只做证据纠偏"的范围边界冲突；
2. 现有 CI（`gofmt`/`vet`/`go test`/`-race`）已覆盖整体健康度，但按要求**不能**用它替代逐路由断言；
3. 补测应在 R0 之后的测试补强批次统一进行，避免零散测试与既有 `qa_*` 命名规范冲突。

最小补测建议（按性价比排序）：

| 优先 | 路由 | 建议用例 |
|---|---|---|
| P1 | `DELETE /accounts/:aid`、`DELETE /categories/:cid` | 走 HTTP：成功删除返回 `{id}`、非 owner 403、被引用 409 |
| P1 | `GET /transactions/:txId` | 走 HTTP：本人可见 200、他人且不可见 404（不泄露存在性） |
| P2 | `GET /books`、`GET /books/:id` | 断言空数组非 null、非成员 403、不存在 404 |
| P2 | `GET /categories`（含 `?tree=1`） | 断言树形结构与 `children` 字段 |
| P3 | `PUT /users/me`、`POST /users/me/password` | 断言 409 邮箱占用、400 弱口令、`{revokedSessions}` 数值 |

责任方：后端 + QA（R0 之后测试补强批次）。
