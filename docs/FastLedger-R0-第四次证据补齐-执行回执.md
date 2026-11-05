# FastLedger R0 第四次证据补齐任务单 · 执行回执（更新版）

> 执行日期：2026-11-05　执行主体：WorkBuddy（**Owner 已授权**；服务器 Linux 环境复跑 + 本地隔离实例采集）
> 依据：`docs/FastLedger-R0-第四次证据补齐任务单.md`
> 文档 HEAD：`476448b16ae4f2d766e4e5c4d0697e12acd497ef`（开工时 `origin/main`）
> 业务代码证据基准：`bff6a2b351bf5a039cdf7c762ec2693fb330cd00`（`server/` 自该提交起零改动，见 §0）
> **结论：R0 仍 BLOCK；不授权 R1–R7；PWA 未实施。包 B/D 有真实新增证据，但包 B 仍不满足"同数据同状态"全量对照。**

---

## 0. 开工对齐（任务单 §0）

| 项 | 实测 |
|---|---|
| 文档 HEAD | `476448b16ae4f2d766e4e5c4d0697e12acd497ef` |
| 工作区 | 干净 |
| 业务代码基准 | `bff6a2b`；`git diff --stat bff6a2b HEAD -- server/` 为空，`git log bff6a2b..HEAD -- server/` 无提交 |
| Owner 决策 | D-2/D-3/D-5/D-6/RV-04 已在主报告 §8（行 902/903/905/906）与页脚（行 913）记录 |
| 缺失文档 | 任务单引用的 `workbuddy/多AI对齐规范.md` / `workbuddy/第三方审阅者导读.md` 不在本检出；按 `docs/FastLedger-R0-收尾执行清单.md` §5、主报告 §8、CHANGELOG 对齐 |
| 服务器环境 | Linux `6.8.0-138-generic x86_64`、`go1.22.2 linux/amd64`、`CGO=1` |
| 采集环境 | Edge `156.0.4314.58`（headless=new）、Node `v22.22.2`、DPR=1、视口 `360x800 / 390x844 / 430x932 / 1280x800` |

---

## 1. 四包状态总表

| 工作包 | 状态 | 执行主体 | 本轮实际 | 证据 | 遗留/解除条件 |
|---|---|---|---|---|---|
| **A. R0-E-35 / RV-01 独立复验** | **BLOCKED** | 需 Owner/CI | 未执行独立复跑 | 既有 `docs/r0-screenshots/runtime/` | Owner/CI 独立环境复跑多场景 |
| **B. 13 页原型×实现同视口对照** | **PARTIAL** | WorkBuddy（Owner 授权） | 原型 52 张 + 实现 44 张；同视口、默认态；impl `login`/`add` 跳过 | `docs/r0-screenshots/pairs/`（96 PNG + 2 manifest + SHA256SUMS + README） | 严格同数据同状态、交互/权限/离线性状、impl 登录/记一笔 |
| **C. 228 条控件/动作运行时验收** | **BLOCKED** | QA | 未填充运行时结果 | 主报告 §3.1（228 行） | 逐条运行时证据或 N/A |
| **D. API/后台逐路由测试证据** | **PASS（测试执行）** | WorkBuddy（Owner 授权，服务器 Linux） | `go test ./...` **EXIT 0**；`go test -race ./... -p 1` **EXIT 0** | `docs/r0-evidence-4th/`（env/plain/race/exit + SHA256） | 非正式 CI（无 run URL）；未覆盖路由补断言 |

---

## 2. 工作包 A：R0-E-35 / RV-01 独立复验（BLOCKED）

- 任务单 §2.1 明确：独立复跑必须由 Owner/CI 指定环境执行；**WorkBuddy 自测不冒充**。本轮**未**将任何自测标为 A 通过。
- 既有证据（历史，未改动）：失败态 `.skel=4`、`aria-busy=true` 未清除、仅 toast、DOM 无重试控件；重新导航可恢复。
- 待补：慢速网络、有数据态、空数据态、权限不足、多角色、离线/恢复、同步冲突。
- 若复现：仅提交最小修复建议，未经 Owner 对代码改动另行授权**不得改业务代码**。

---

## 3. 工作包 B：13 页原型×实现同视口对照（PARTIAL）

### 3.1 本轮新增（真实采集）

- **原型侧 52 张**：13 页 × 4 视口；Edge headless + CDP；脚本内置静态服务（SPA fallback）；自动登录（me/123456）后按路径采集。**0 错误、0 路径偏差**（manifest `actualPath` 与请求路径一致）。
- **实现侧 44 张**：11 路由 × 4 视口；本地隔离实例（schema v8），种子 1 账本 / 4 账户 / 2 交易（支出 ¥128.50、收入 ¥8,000）。**0 错误**；`login`、`add-transaction` 各 4 视口跳过（8 条 skipped）。
- **产物**：`docs/r0-screenshots/pairs/`（`prototype/`、`impl/`、`manifest-prototype.json`、`manifest-impl.json`、`SHA256SUMS.txt` 98 行、`README.md`）。
- **采集脚本**：`tools/r0/capture-pairs.js`（可移植、默认输出仓库外、写证据目录双重确认、退出码 0/1/2/3）。

### 3.2 诚实边界（未满足项）

1. **非同数据同状态**：原型演示数据 vs 实现种子数据，仅完成**同视口、默认（normal）态**对照，**不是**严格"同数据同状态"。
2. **实现侧 `login` 跳过**：状态页，注入 token 会再次登录，需另一种引导方式。
3. **实现侧 `add-transaction` 跳过**：弹层无独立路由，需动作级采集。
4. **原型 `settings` 与 `mine` 同 `/profile`**：两页截图 SHA256 相同（已知映射）。
5. **仅 normal 态**：loading/pending/empty/error/permission/offline/conflict 未覆盖；无像素级 diff 标注。
6. 因此包 B 为 **PARTIAL**，不满足任务单"每对同数据同状态"的全量要求。

---

## 4. 工作包 C：228 条控件/动作运行时验收（BLOCKED）

- 主报告 §3.1 共 **228 行**（`grep -cE '^\| I-'` 实测 228），静态映射完整；**运行时结果未补齐**。
- 本轮**未新增**任何运行时断言/截图，**不以静态映射冒充运行时验收**。
- 每条待补：前置/输入/操作/成功/失败/联动/返回/loading/pending/防重复/校验/empty/error/success/permission/offline/conflict/证据/环境/SHA；不适用写 `N/A`+原因，无法执行写 `BLOCKED`+责任方。

---

## 5. 工作包 D：逐路由/Go 测试证据（PASS，服务器 Linux）

### 5.1 执行（Owner 授权，WorkBuddy 在服务器执行）

- 服务器：`ubuntu@119.91.63.95`；`/home/ubuntu/ci/fastledger-src` 已 `git fetch origin main` 并 `checkout bff6a2b`。
- 命令：`cd server && go test ./...`；`go test -race ./... -p 1`。
- 环境：`go1.22.2 linux/amd64`、`CGO=1`、`COMMIT=bff6a2b351bf5a039cdf7c762ec2693fb330cd00`。

### 5.2 结果（原始日志见 `docs/r0-evidence-4th/`）

**plain（`plain.log` / `plain.exit`）—— `PLAIN_EXIT=0`**

```
ok   fastledger/cmd/server            0.336s
ok   fastledger/internal/api          4.459s
ok   fastledger/internal/authzreview  14.257s
ok   fastledger/internal/config       0.003s
ok   fastledger/internal/database     0.054s
ok   fastledger/internal/instance     0.172s
ok   fastledger/internal/migrate      1.430s
ok   fastledger/internal/model        0.003s
?    fastledger/internal/version      [no test files]
?    fastledger/web                   [no test files]
ok   fastledger/internal/service      32.722s
ok   fastledger/internal/ws           7.151s
```

**race（`race.log` / `race.exit`）—— `RACE_EXIT=0`，无 DATA RACE**

```
ok   fastledger/cmd/server            1.362s
ok   fastledger/internal/api          33.605s
ok   fastledger/internal/authzreview  125.289s
ok   fastledger/internal/config       1.019s
ok   fastledger/internal/database     1.064s
ok   fastledger/internal/instance     1.175s
ok   fastledger/internal/migrate      2.469s
ok   fastledger/internal/model        1.011s
ok   fastledger/internal/service      109.299s
?    fastledger/internal/version      [no test files]
ok   fastledger/internal/ws           10.773s
?    fastledger/web                   [no test files]
```

日志 SHA256（同见 `docs/r0-evidence-4th/SHA256SUMS.txt`）：
`env.txt f7ea9a4e…` / `plain.log 11c45f6a…` / `race.log 4e4871bb…`。

### 5.3 证据边界

- 这是**服务器 Linux 环境的干净全量 + race 通过**，明显强于本地 Windows（后者受 Application Control 拦截、`-race` 不可用）。
- 但执行主体仍是 WorkBuddy（Owner 授权），**不是正式 CI**，无 CI run URL；如需完全满足"Owner/CI 独立"，仍建议在 CI 侧复跑并留 run 链接。
- 未覆盖路由（§5.3 已标"未覆盖"者）仍需补关键断言或明确范围处理。

---

## 6. 证据与安全纪律（任务单 §6）

- 截图与 manifest 不含 `Authorization`/`Bearer`/JWT/Cookie/密码/个人绝对路径（已扫描）；`protoDir` 占位为 `<R0_PROTO_DIR>`。
- 实现侧 token 仅经环境变量注入采集进程，不打印、不落盘。
- 使用隔离 DB + 一次性随机 `EBK_JWT_SECRET`；合成测试数据。
- 新增 `pairs/` 与 `r0-evidence-4th/`，**未覆盖**既有 43 张与 `runtime/` 历史证据。

---

## 7. 未完成项与阻塞

| # | 项 | 责任方 | 状态/解除条件 |
|---|---|---|---|
| 1 | A：R0-E-35 独立复跑多场景 | Owner/CI | BLOCKED |
| 2 | B：同数据同状态全量对照 + 交互/权限/离线态 | QA/前端 | PARTIAL → 需统一数据集与动作采集 |
| 3 | B：实现侧登录页/记一笔弹层 | 前端/QA | 未采（状态页/弹层） |
| 4 | C：228 条运行时验收 | QA | BLOCKED |
| 5 | D：正式 CI run + 未覆盖路由断言 | Owner/CI | 服务器复跑已 PASS；CI 链接待补 |
| 6 | D-8 生产 cron 真实部署验证 | Owner/运维 | 未验证 |

---

## 8. 文档与产物更新

- 新增 `docs/r0-screenshots/pairs/`（96 PNG + 2 manifest + SHA256SUMS + README）。
- 新增 `docs/r0-evidence-4th/`（服务器 env/plain/race/exit + SHA256SUMS）。
- 新增 `tools/r0/capture-pairs.js`（成对采集骨架）。
- 主报告 §8 加回执指引（不改 Owner 裁决）；CHANGELOG、README 导航更新。

---

## 9. 结论与诚实边界

- **R0 仍 BLOCK**；本回执不代表 R0 通过，也不代表 R1 获准。
- 包 D **测试执行 PASS**（服务器 Linux，plain + race 均 exit 0）；包 B **PARTIAL**（同视口默认态已成对，非同数据、impl 两页未采）；包 A/C 仍 BLOCKED。
- 未把任何未执行测试写成 PASS；WorkBuddy 执行与 Owner/CI 独立复跑分别标注。
