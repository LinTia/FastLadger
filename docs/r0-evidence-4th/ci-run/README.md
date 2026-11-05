# 正式 CI 运行证据（GitHub Actions）

本目录是**由 GitHub Actions 官方产生的**运行证据，用于替换此前"WorkBuddy 自测 / 服务器手工复跑"的非正式口径。

## 1. 运行标识

| 项 | 值 |
|---|---|
| 仓库 | `LinTia/FastLadger` |
| 工作流 | `CI`（`.github/workflows/ci.yml`） |
| run id | `41954006789` |
| run URL | https://github.com/LinTia/FastLadger/actions/runs/41954006789 |
| 触发事件 | `push` → `main` |
| head commit | `9d083f62bf80e0ba7cdb444e57940eb432c1d5f3`（**即包含本证据目录的提交**） |
| attempt | `1` |
| 结论 | **success**（两个 job 全部 success） |
| 时间 | 2026-11-05T02:19Z 前后（精确值见 `run-meta.json`） |

> 前一次提交 `31716b0` 的 run `41951501832` 同样 **success**（结论一致）。本目录以**包含证据自身的提交**为准，避免"日志与提交错位"。两次 run 的业务代码均为 `bff6a2b`（`server/` 自该提交起零改动）。

## 2. 两个 job 的逐步骤结论

**job 1 `build / vet / test / race`（id `126996266896`，success）**

| # | 步骤 | 结论 |
|---|---|---|
| 4 | Toolchain（必须看到 gcc 与 CGO_ENABLED=1） | success |
| 5 | gofmt（必须无未格式化文件） | success |
| 6 | go vet | success |
| 7 | go build | success |
| 8 | SQLite 驱动冒烟（证明不是 stub） | success |
| 9 | go test 全量 | success |
| 10 | go test -race（Hub 并发 / 事务边界） | success |

**job 2 `docker build / run / healthz`（id `126996901301`，success）**

| # | 步骤 | 结论 |
|---|---|---|
| 3 | Build 镜像（CGO + musl 多阶段） | success |
| 4 | 容器内 db migrate 冒烟 | success |
| 5 | 容器冒烟（healthz 应报 up + schemaVersion） | success |

## 3. 关键事实（可核对 `summary.txt` / 原始日志）

- 环境：`go version go1.22.12 linux/amd64`、`gcc (Ubuntu 13.3.0-6ubuntu2~24.04.1)`、`CGO_ENABLED=1`，runner 镜像 `ubuntu-24.04`。
- `go test ./... -count=1 -v`：**10 个含测试包全部 `ok`**，2 个包 `[no test files]`（`internal/version`、`web`）。
- `-race` 步骤：`ws` / `service` / `api` 三包全部 `ok`（10.4s / 66.4s / 24.0s）。
- 全日志 `--- FAIL` 行数 **0**、`DATA RACE` 行数 **0**。
- 容器冒烟返回：`{"code":0,"data":{"instanceId":"...","restoreEpoch":1,"schemaVersion":8,"status":"up"},"msg":"ok"}`（该 `instanceId` 是 CI 一次性容器的，与生产实例无关）。

## 4. 与"服务器手工复跑"的关系

| 证据 | 覆盖范围 | 性质 |
|---|---|---|
| 本目录（run 41954006789） | `go test ./...`（全量）、`-race`（ws/service/api）、gofmt/vet/build、docker 冒烟 | **正式 CI**，有 run URL 与官方日志 |
| `../plain.log` / `../race.log`（服务器 `bff6a2b`） | `go test ./...`、`go test -race ./... -p 1`（**全仓库** race） | 手工复跑，补齐 CI 未覆盖的全仓 race |

两者互补：CI 未对全仓跑 `-race`，服务器手工复跑补齐了这一点。

## 5. 复现方式

```bash
# GitHub CLI
gh run view 41954006789 --repo LinTia/FastLadger
gh run download 41954006789 --repo LinTia/FastLadger -D ./ci-logs

# 或 REST API（需 token）
curl -L -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/repos/LinTia/FastLadger/actions/runs/41954006789/logs -o ci.zip
```

## 6. 文件清单

| 文件 | 说明 |
|---|---|
| `run-meta.json` | 运行元数据（run/job/step 结论，取自 Actions API） |
| `test-job.log` | job 1 完整原始日志（243 KB，未改写） |
| `docker-job.log` | job 2 完整原始日志（22 KB，未改写） |
| `summary.txt` | 关键证明行摘录（按步骤分段，逐行保留原始时间戳） |
| `SHA256SUMS.txt` | 上述文件校验和 |

## 7. 限制

- CI 通过 **不等于** R0 通过：R0 门禁仍包含截图基线、逐控件运行时验收、R0-E-35 独立复跑等，本目录仅关闭"测试执行证据"一项。
- 本 run 触发时仓库仍为 **Public**；转 Private 后 Actions 记录仍在，但外部匿名访问将受限（Owner 待办 RV-05）。
- 日志中的路径/容器 ID 属 GitHub 托管 runner 环境，与生产服务器无关。
- **校验和说明**：`SHA256SUMS.txt` 基于 LF 工作树计算。Windows 上 `core.autocrlf=true` 的 clone 会把文本文件检出为 CRLF，届时文本文件的哈希会不同（PNG 不受影响）——这是行尾规范化，不是内容篡改；需要逐字节复核时用 `git -c core.autocrlf=false clone`。
