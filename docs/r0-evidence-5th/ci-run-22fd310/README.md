# CI run `41976312248` — GitHub 自带 CI 的正式证据（head `22fd310`）

本目录是 **GitHub Actions 原生运行**（不是本机自测、不是服务器手动复跑）的原始日志与元数据，
用于回答一个问题：**"某次验证通过"能不能由任何人都能点开核对的 CI 记录来证明。**

## 1. 元数据（`run-meta.json` 为 API 原样落盘）

| 项 | 值 |
|---|---|
| 仓库 | `LinTia/FastLadger` |
| 工作流 | `CI`（`.github/workflows/ci.yml`） |
| run id | `41976312248` |
| run URL | https://github.com/LinTia/FastLadger/actions/runs/41976312248 |
| 触发 | `push` → `main` |
| head commit | `22fd31099065ce0d6f15a899374740b4a56ead06`（第五次最小整改提交） |
| 结论 | **success**（两个 job 全部 success） |
| 时间 | 2026-11-05T05:48:22Z 创建 → 05:53:5xZ 完成 |
| runner | `ubuntu-latest`；`go1.22.12 linux/amd64`；`gcc (Ubuntu 13.3.0-6ubuntu2~24.04.1) 13.3.0`；`CGO_ENABLED=1` |

## 2. 逐步骤结论（`run-meta.json.jobs[].steps`）

job `127075869907`（build / vet / test / race）：

| # | 步骤 | 结论 |
|---|---|---|
| 4 | Toolchain（必须看到 gcc 与 CGO_ENABLED=1） | success |
| 5 | gofmt（必须无未格式化文件） | success |
| 6 | go vet | success |
| 7 | go build | success |
| 8 | SQLite 驱动冒烟（反向门禁：出现 `no test files` 即失败） | success |
| 9 | go test 全量（`./... -count=1 -v`） | success |
| 10 | go test -race（**当时作用域**：`internal/ws`、`internal/service`、`internal/api`） | success |

job `127076627469`（docker build / run / healthz）：镜像构建、容器内 `db migrate`、容器 `healthz`（`{"status":"up","schemaVersion":8,...}`）全部 success。

## 3. 关键证明行（`summary.txt`，自 `test-job.log` 原文摘录，未改写）

- 全量 10 个有测试包 `ok`，2 个包 `[no test files]`（`internal/version`、`web`）。
- `-race` 三包：`ws 10.995s`、`service 86.781s`、`api 32.094s`，均 `ok`。
- `--- FAIL` **0** 行、`DATA RACE` **0** 行。

## 4. 这个 run 能证明什么、不能证明什么

**能证明（任何人可复核对）**

1. `gofmt` / `go vet` / `go build` 在干净 Linux + CGO 环境通过。
2. `go test ./...` 全量通过（含真 SQLite 驱动，不是 stub——步骤 8 的反向门禁保证）。
3. `ws` / `service` / `api` 三个包的 `-race` 通过，无 DATA RACE。
4. 容器镜像可构建、可 `db migrate`、`/healthz` 返回 `up`。

**不能证明（本 run 的边界，不得据此扩大结论）**

1. **不是全仓 `-race`**：本 run 的步骤 10 只覆盖 3 个包；其余包（`cmd/server`、`migrate`、`database`、`authzreview`、`config`、`instance`、`model`）只经非 race 全量测试。全仓 `-race` 当时靠服务器复跑补充（`docs/r0-evidence-4th/race.log`）。
2. **不是逐路由测试覆盖证据**：CI 绿 ≠ 13 条 API 路由都有独立断言（第五次复核结论：覆盖 3 / 部分 5 / 未覆盖 5，见 `../api-route-coverage.md`）。
3. **不是运行时 UI 验收**：A（R0-E-35 独立复验）与 C（228 条逐控件运行时）**无法**由 Go 测试 CI 替代。
4. 本 run 的业务代码仍是 `bff6a2b`（`server/` 自该提交起零改动），故结论适用于该业务代码基准。

## 5. 复现方式

```bash
# 任意拥有读权限的人（公开仓库下无需登录亦可查看页面）
gh run view 41976312248 --repo LinTia/FastLadger
gh run download 41976312248 --repo LinTia/FastLadger -D ./ci-logs

# 或 REST API
curl -L -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/repos/LinTia/FastLadger/actions/runs/41976312248/logs -o ci.zip
```

## 6. 文件

| 文件 | 说明 |
|---|---|
| `run-meta.json` | Actions API 返回的 run/job/step 元数据（含 run id、head SHA、结论、逐步状态） |
| `test-job.log` | `build / vet / test / race` job 的完整原始日志（GitHub 导出格式） |
| `docker-job.log` | `docker build / run / healthz` job 的完整原始日志 |
| `summary.txt` | 自原始日志摘录的关键证明行 + 作用域说明 |
| `SHA256SUMS.txt` | 上述文件的校验和（`sha256sum -c` 可验证） |

> 日志中的路径/容器 ID 属 GitHub 托管 runner，与生产服务器无关；已扫描确认不含凭证、令牌或个人绝对路径。
