# CI 元数据更正（第五次整改，2026-11-05）

> 第三方复核指出第四次 CI 元数据错配，本轮按 GitHub Actions 官方 API/日志逐项复核并更正。
> 官方数据来源：`docs/r0-evidence-4th/ci-run/run-meta.json`（API 直取）、`test-job.log` / `docker-job.log`（官方日志原文）。

## 1. 更正前后对照

| 项 | 第四次文档所写 | 官方复核值 | 结论 |
|---|---|---|---|
| job 1 id | `126996266896` | **`127004401500`** | 原值属另一次 run（`41951501832`），已更正 |
| job 2 id | `126996901301` | **`127005322262`** | 同上，已更正 |
| `-race` 三包耗时 | `10.4s / 66.4s / 24.0s` | **`13.204s / 93.326s / 34.720s`** | 原值属 `41951501832`，已按本次日志更正 |
| run head | 表述为"包含本证据目录的提交"（指向 `9d083f6` 又被写成最终 HEAD） | **`9d083f62bf80e0ba7cdb444e57940eb432c1d5f3`** | 原表述混淆了"run head"与"最终 HEAD"，已拆开分列（见 §2） |

官方日志中的原文行（`test-job.log`）：

```
2026-11-05T02:16:33.9561296Z ok  	fastledger/internal/ws	13.204s
2026-11-05T02:17:57.6726884Z ok  	fastledger/internal/service	93.326s
2026-11-05T02:17:57.6727488Z ok  	fastledger/internal/api	34.720s
```

## 2. 四类 SHA 必须分开记录（不得合并成"当前版本"）

| 名称 | 值 | 含义 |
|---|---|---|
| CI 实际测试的 commit | `9d083f62bf80e0ba7cdb444e57940eb432c1d5f3`（`9d083f6`） | run `41954006789` 的 `head_sha`，`push` 事件触发 |
| 业务代码基准 | `bff6a2b351bf5a039cdf7c762ec2693fb330cd00`（`bff6a2b`） | `server/` 自该提交起零 diff（`git diff --stat bff6a2b 9d083f6 -- server/` 为空） |
| 证据目录/文档提交 | `9d083f6` 引入 `docs/r0-evidence-4th/ci-run/`（初始为 run `41951501832` 日志）；`11bdfcc` 将其替换为 run `41954006789` 的日志 | **该 run 不包含后续才提交的最终版证据目录** |
| 第五次整改时的 main HEAD | `72cbab71253d031a09acc405cad25155506db531`（`72cbab7`，第五次任务单落地提交） | 本轮开工基准 |

结论：run `41954006789` 对**同一业务代码基准**仍有参考价值，但**不能**表述为"run head 就是最终提交"或"该 run 包含随后提交的完整证据目录"。

## 3. 本次 run 的有效结论（不因元数据更正而变化）

- 两个 job 全部 `success`：`build / vet / test / race`（id `127004401500`）、`docker build / run / healthz`（id `127005322262`）。
- `go test ./...`：10 包 `ok` + 2 包 `[no test files]`；`--- FAIL` 0。
- `-race`：仅 `ws` / `service` / `api` 三包（非全仓），0 `DATA RACE`。
- 环境：`go1.22.12 linux/amd64`、`gcc 13.3.0`、`CGO_ENABLED=1`、runner `ubuntu-24.04`。

## 4. 与服务器全仓 race 的边界（不得混称 CI）

| 证据 | 执行主体 | 覆盖 | 可追溯字段 |
|---|---|---|---|
| CI run `41954006789` | GitHub Actions | `go test ./...`、`-race`（仅 ws/service/api）、gofmt/vet/build、docker 冒烟 | run id / job id / 日志 / head SHA |
| 服务器全仓 race | **WorkBuddy（Owner 授权）**，非 GitHub Actions | `go test -race ./... -p 1`（全仓） | `docs/r0-evidence-4th/env.txt`（commit / go version / GOOS/GOARCH/CGO）、`race.log`、`race.exit`、`SHA256SUMS.txt` |

服务器证据**不可**写成 CI；CI 的三包 race **不可**写成全仓 race。
