# FastLedger R0 第四次证据补齐任务单 · 执行回执（更新版 2）

> 执行日期：2026-11-05　执行主体：WorkBuddy（**Owner 已授权**）
> 依据：`docs/FastLedger-R0-第四次证据补齐任务单.md`
> 本轮起始文档 HEAD：`31716b0d51a12ee6c705cc670059b2e129c1e37f`
> 业务代码证据基准：`bff6a2b351bf5a039cdf7c762ec2693fb330cd00`（`server/` 自该提交起零改动，见 §0）
> **结论：R0 仍 BLOCK；不授权 R1–R7；PWA 未实施。包 B 已补齐"同一数据集 + 动作级"主集对照，包 D 已有正式 CI run；A/C 仍 BLOCKED。**

---

## 0. 开工对齐（任务单 §0）

| 项 | 实测 |
|---|---|
| 本轮起始文档 HEAD | `31716b0d51a12ee6c705cc670059b2e129c1e37f` |
| 工作区 | 干净 |
| 业务代码基准 | `bff6a2b`；`git diff --stat bff6a2b HEAD -- server/` 为空，`git log bff6a2b..HEAD -- server/` 无提交 |
| Owner 决策 | D-1/D-3/D-4/D-5/D-6/D-8 已在主报告 §8 与裁决草案记录 |
| CI | 仓库已有 `.github/workflows/ci.yml`（`push: [main]`），历史 160 次 run |
| 采集环境 | Edge `Edg/156.0.4314.58`（headless + CDP）、Node `v22.22.2`、DPR=1、视口 `360x800 / 390x844 / 430x932 / 1280x800` |

---

## 1. 四包状态总表

| 工作包 | 状态 | 执行主体 | 本轮实际 | 证据 | 遗留/解除条件 |
|---|---|---|---|---|---|
| **A. R0-E-35 / RV-01 独立复验** | **BLOCKED** | 需 Owner/CI | 未执行独立复跑 | 既有 `docs/r0-screenshots/runtime/` | Owner/CI 独立环境复跑多场景 |
| **B. 13 页原型×实现同数据对照** | **PASS（同数据主集）** | WorkBuddy（Owner 授权） | 两侧各 **52 张**（13 页×4 视口）；**同一数据集**；含 impl 登录页 + 记一笔弹层动作级 | `docs/r0-screenshots/pairs/`（104 PNG + 2 manifest + seed-manifest + SHA256SUMS 108 行 + README） | 仍缺 loading/empty/error/permission/offline/conflict 态；`invest`/`members` 数据未对齐 |
| **C. 228 条控件/动作运行时验收** | **BLOCKED** | QA | 未填充运行时结果 | 主报告 §3.1（228 行） | 逐条运行时证据或 N/A |
| **D. API/后台逐路由测试证据** | **PASS（正式 CI + 全仓 race）** | GitHub Actions + WorkBuddy（服务器） | CI run `41951501832` success（含 `go test ./...` 与 `-race`）；服务器补全仓 `-race` | `docs/r0-evidence-4th/`（ci-run/ + env/plain/race/exit + SHA256） | 未覆盖路由仍缺关键断言 |

---

## 2. 工作包 A：R0-E-35 / RV-01 独立复验（BLOCKED）

- 任务单 §2.1 明确：独立复跑必须由 Owner/CI 指定环境执行；**WorkBuddy 自测不冒充**。本轮**未**将任何自测标为 A 通过。
- 既有证据（历史，未改动）：失败态 `.skel=4`、`aria-busy=true` 未清除、仅 toast、DOM 无重试控件；重新导航可恢复。
- 待补：慢速网络、有数据态、空数据态、权限不足、多角色、离线/恢复、同步冲突。

---

## 3. 工作包 B：13 页原型×实现同数据对照（PASS，同数据主集）

### 3.1 本轮做法（与上一轮的本质差别）

上一轮是"同视口、默认态，但两侧各自用自己的数据"（原型内置演示数据 vs 实现 2 笔种子）。本轮改为**同一数据集**：

```
prototype/assets/index-*.js
  └─ tools/r0/extract-prototype-dataset.js → tools/r0/prototype-dataset.json（可复现，含 bundle SHA256）
       └─ tools/r0/seed-from-dataset.js → 实现侧隔离实例：5 账户 / 37 分类 / 22 交易 / 5 预算
            └─ 回读实现侧派生余额 → proto-injection.json → 注入原型 localStorage
```

- 建账本时实现侧会自动播种默认账户/分类，脚本先清空默认播种（`seed-manifest.json` → `defaultsRemoved`）再灌原型数据集，避免账户数不一致。
- 原型数据层被 toolkit 加了命名空间前缀（`__miaoda_<appid>__:`）且未登录不建键；采集脚本**运行时探测前缀**（先用一次演示登录建键，再清空正式采集），不硬编码。
- 实现侧是 hash 路由，仅改 hash 不会重建文档；脚本用一次性 query 强制新建文档，保证注入脚本执行。
- 原型静态演示余额被替换为实现侧派生余额（两侧同数字），这一步同时暴露 D-4 期初余额未实现的后果。

### 3.2 本轮产物（真实采集）

- **原型 52 张**（13 页 × 4 视口），`errors=0`、`skipped=0`，`actualPath` 与请求一致。
- **实现 52 张**（同 13 页 × 4 视口），`errors=0`、`skipped=0`；含两类动作级：
  - `login`：4 视口，`state=logged-out`，`hasToken=false`（真实未登录态）；
  - `add-transaction`：4 视口，`state=dialog-open`，`#formDlg` 实测 `open=true`、`320×552`。
- **两侧数据一致的可核对量**：本月收入 ¥28,000.00、本月支出 ¥1,089.30；账户余额 28,100.00 / 0.00 / −716.30 / −1,125.00 / 3,868.00。
- **产物**：`docs/r0-screenshots/pairs/`（`prototype/`、`impl/`、`manifest-prototype.json`、`manifest-impl.json`、`seed-manifest.json`、`SHA256SUMS.txt` 108 行、`README.md`）。
- **脚本**：`tools/r0/extract-prototype-dataset.js`、`tools/r0/seed-from-dataset.js`、扩展后的 `tools/r0/capture-pairs.js`。

### 3.3 采集暴露的实现侧问题（建议进缺陷清单，待 QA 复现确认）

1. 记一笔弹层**分类未按类型过滤**：默认「支出」时分类首项为「工资」（收入类），直接保存会 `400 请求参数非法`。
2. 移动端 `390×844` 报表「本月支出分类占比」**丢失分类名列**（1280×800 正常）。
3. 账户余额语义：电子钱包为负（−716.30 / −1,125.00），因期初余额未实现、余额全由支出派生（D-4 待落地）。
4. 首页「结余」口径不同：原型 = 本月收入−支出（¥26,910.70），实现 = 净资产（¥30,126.70）。
5. 交易行信息密度不同：原型含商户/账户/渠道，实现无 merchant 字段（`seed-manifest.json` → `fieldDiffs`）。

### 3.4 诚实边界（仍未满足项）

1. **状态覆盖不全**：仅 `default` / `logged-out` / `dialog-open`；loading、empty、error、permission、offline、sync-conflict 未覆盖。
2. **`invest` / `members` 数据未对齐**：实现侧持仓需 instrument 前置（无公开创建 API）；成员需邀请流程，本轮不伪造。
3. **原型 `settings` 与 `mine` 同 `/profile`**：两图 SHA256 相同（原型无独立设置路由）。
4. **无像素级 diff**：只提供成对图与逐张 SHA256，不做自动相似度评分。
5. **单角色**：仅账本 owner，未覆盖 admin/member/只读/回收站视角。

---

## 4. 工作包 C：228 条控件/动作运行时验收（BLOCKED）

- 主报告 §3.1 共 **228 行**（`grep -cE '^\| I-'` 实测 228），静态映射完整；**运行时结果未补齐**。
- 本轮**未新增**任何运行时断言，**不以静态映射或截图冒充运行时验收**。
- 每行待补：前置/输入/操作/成功/失败/联动/loading/pending/防重复/校验/empty/error/permission/offline/conflict/证据/环境/SHA；不适用写 `N/A`+原因，无法执行写 `BLOCKED`+责任方。

---

## 5. 工作包 D：逐路由/Go 测试证据（PASS）

### 5.1 正式 CI（GitHub Actions）

| 项 | 值 |
|---|---|
| run | `41951501832` |
| URL | https://github.com/LinTia/FastLadger/actions/runs/41951501832 |
| head | `31716b0d51a12ee6c705cc670059b2e129c1e37f`（文档/证据提交；业务代码仍 `bff6a2b`） |
| 结论 | **success**（`build / vet / test / race` + `docker build / run / healthz` 两 job 全绿） |
| 关键步骤 | gofmt / go vet / go build / SQLite 驱动冒烟 / `go test ./...` / `go test -race`（ws、service、api）全部 success |
| 环境 | `go1.22.12 linux/amd64`、`gcc 13.3.0`、`CGO_ENABLED=1`、runner `ubuntu-24.04` |
| 计数 | `ok` 10 个包 / `[no test files]` 2 个；`--- FAIL` **0**；`DATA RACE` **0** |

原始日志与校验和：`docs/r0-evidence-4th/ci-run/`（`run-meta.json`、`test-job.log`、`docker-job.log`、`summary.txt`、`SHA256SUMS.txt`）。

### 5.2 服务器 Linux 补充（补齐 CI 未覆盖的全仓 race）

- 服务器 `ubuntu@119.91.63.95`、`/home/ubuntu/ci/fastledger-src`、`checkout bff6a2b`。
- `go test ./...` → **PLAIN_EXIT=0**；`go test -race ./... -p 1` → **RACE_EXIT=0**、无 DATA RACE。
- 环境：`go1.22.2 linux/amd64`、`CGO=1`。
- 日志：`docs/r0-evidence-4th/`（`plain.log` / `race.log` / `*.exit` / `env.txt` + SHA256SUMS）。

> 口径修正：上一版回执写"非正式 CI、无 run URL"，**该表述错误**——仓库一直有 Actions 工作流且在运行。本轮已用官方 run 取代该口径。CI 的 `-race` 只跑 `ws/service/api` 三个包，全仓 `-race` 仍由服务器复跑补齐，两者互补。

---

## 6. 证据与安全纪律（任务单 §6）

- 截图与 manifest 不含 `Authorization`/`Bearer`/JWT/Cookie/密码/个人绝对路径（已扫描）；`protoDir`、`protoInjectionPath` 均以占位符记录。
- 实现侧 token 仅经环境变量注入采集进程，不打印、不落盘。
- 使用隔离 DB + 一次性随机 `EBK_JWT_SECRET`；合成测试数据；采集结束即终止进程。
- 新增 `pairs/`（覆盖同名旧图，见 §8 说明）与 `r0-evidence-4th/ci-run/`，未覆盖既有 43 张首屏与 `runtime/` 历史证据。

---

## 7. 未完成项与阻塞

| # | 项 | 责任方 | 状态/解除条件 |
|---|---|---|---|
| 1 | A：R0-E-35 独立复跑多场景 | Owner/CI | BLOCKED |
| 2 | B：loading/empty/error/permission/offline/conflict 态对照 | QA/前端 | 未覆盖 |
| 3 | B：`invest` / `members` 数据对齐（instrument 前置、邀请流程） | 前端/后端 | 未对齐，需产品或接口支持 |
| 4 | C：228 条运行时验收 | QA | BLOCKED |
| 5 | D：未覆盖路由的关键断言 | 后端/QA | CI 已绿，但"逐路由断言"仍缺 |
| 6 | D-8 生产 cron 真实部署验证 | Owner/运维 | 未验证 |
| 7 | RV-05 仓库改 Private | Owner | 未执行（仓库当前仍 Public） |

---

## 8. 文档与产物更新

- 新增 `tools/r0/extract-prototype-dataset.js`（提取原型原生数据集 → `tools/r0/prototype-dataset.json`）。
- 新增 `tools/r0/seed-from-dataset.js`（按同一数据集种子化实现侧，输出运行时 manifest 与注入包，均不入库）。
- 扩展 `tools/r0/capture-pairs.js`：同数据集注入、动作级采集（登录/弹层）、manifest 增 `state`/`action`/`dataset`/`hasToken`。
- 更新 `docs/r0-screenshots/pairs/`：96 → **104 张**（同名旧图被同数据版本替换），新增 `seed-manifest.json`，README 重写。
- 新增 `docs/r0-evidence-4th/ci-run/`（正式 CI 原始日志与元数据）。
- 主报告 §8 加回执指引（不改 Owner 裁决）；CHANGELOG、README 导航更新。

---

## 9. 结论与诚实边界

- **R0 仍 BLOCK**；本回执不代表 R0 通过，也不代表 R1 获准。
- 包 B：**同数据主集对照完成**（104 张、0 错误），但仍缺状态维度与 2 页数据对齐。
- 包 D：**正式 CI run + 全仓 race 均通过**，`-race` 覆盖由 CI 与服务器复跑互补。
- 包 A/C：仍 BLOCKED，未以任何自测冒充。
- 采集中发现的 5 条实现侧问题已如实登记，均标注"待 QA 复现确认"，未在文档中当作已确认缺陷。
