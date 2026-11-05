# CI run `41983447389` — 全仓 `-race` 的正式证据（head `de2fb26`）

这是 `ci.yml` 把 `-race` 作用域从"3 个包"扩到**全仓**之后的第一份 GitHub Actions 原生记录。
本目录存在的理由很简单：**"全仓竞态已过"这句话，从此不需要任何人相信我在服务器上手动跑过。**

## 1. 元数据

| 项 | 值 |
|---|---|
| run id | `41983447389` |
| run URL | https://github.com/LinTia/FastLadger/actions/runs/41983447389 |
| 触发 | `push` → `main` |
| head commit | `de2fb261c0c5a7aafdf7892fe8da5e9e1c59050b`（扩 `-race` 作用域的提交） |
| 结论 | **success**（两个 job 全部 success） |
| runner | `ubuntu-latest`；`go1.22.12 linux/amd64`；`gcc 13.3.0`；`CGO_ENABLED=1` |

## 2. 步骤 10：`go test ./... -race -count=1`（本次的关键变化）

| 包 | 结果 |
|---|---|
| `cmd/server` | ok 1.110s |
| `internal/api` | ok 26.401s |
| `internal/authzreview` | ok **99.487s** |
| `internal/config` | ok 1.021s |
| `internal/database` | ok 1.056s |
| `internal/instance` | ok 1.075s |
| `internal/migrate` | ok 1.411s |
| `internal/model` | ok 1.016s |
| `internal/service` | ok **81.241s** |
| `internal/ws` | ok 11.751s |
| `internal/version`、`web` | `[no test files]` |

`--- FAIL` **0** 行、`DATA RACE` **0** 行。

> 与 run `41976312248` 的区别：那次步骤 10 只跑 `ws` / `service` / `api`；从本 run 起全仓纳入。
> 服务器手动复跑（`../r0-evidence-4th/race.log`）不再是唯一来源，仅作历史旁证保留。

## 3. 这个 run 仍不能证明什么

1. **不能替代逐路由测试覆盖**：CI 绿 ≠ 13 条 API 路由都有独立断言（第五次复核：覆盖 3 / 部分 5 / 未覆盖 5，见 `../api-route-coverage.md`）。这是**测试代码缺口**，不是 CI 能力缺口——补上断言后 CI 会自动覆盖。
2. **不能替代运行时 UI 验收**：A（R0-E-35 独立复验）与 C（228 条逐控件运行时）本质是"人/自动化在真实浏览器里操作并观察"，Go 测试 CI 无法覆盖。
3. **不能替代成对截图对照**（包 B）：需要在真实浏览器按视口采集，目前由 `tools/r0/capture-pairs.js` 本地执行，未接入 CI。

## 4. 复现

```bash
gh run view 41983447389 --repo LinTia/FastLadger --log
# 或
curl -L -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/repos/LinTia/FastLadger/actions/runs/41983447389/logs -o ci.zip
```

## 5. 文件

| 文件 | 说明 |
|---|---|
| `run-meta.json` | Actions API 返回的 run/job/step 元数据 |
| `test-job.log` | `build / vet / test / race` job 完整原始日志 |
| `docker-job.log` | `docker build / run / healthz` job 完整原始日志 |
| `summary.txt` | 关键证明行摘录 + 与上一 run 的差别 |
| `SHA256SUMS.txt` | 校验和（基于 LF 工作树；Windows autocrlf clone 的文本文件哈希会因行尾规范化而不同） |
