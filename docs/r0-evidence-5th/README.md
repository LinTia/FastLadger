# docs/r0-evidence-5th — 第五次最小整改证据

> 依据 `docs/FastLedger-R0-第五次最小整改任务单.md`；执行回执见 `docs/FastLedger-R0-第五次最小整改-执行回执.md`。
> 本目录只放**本轮真实产生的可核对证据**，不含凭证、绝对路径或任何旧 token 值。

| 文件 | 内容 | 怎么核对 |
|---|---|---|
| `seed-mapping-before.json` | 修复前 4 笔交易的错误映射（来自第四次提交的 `seed-manifest.json`） | 与 `git show 72cbab7:docs/r0-screenshots/pairs/seed-manifest.json` 对照 |
| `seed-mapping-after.json` | 修复后 **22 笔**全量映射审计（`categoryAudit`）+ 命中方式统计 + fallback 记录 | 与 `tools/r0/seed-from-dataset.js` 的 `resolveCategory` 对照 |
| `checksum-verification.txt` | 104 张 PNG 的 `sha256sum -c` 逐文件结果（OK 109 / FAILED 0） | 在 `docs/r0-screenshots/pairs/` 内重跑 `sha256sum -c SHA256SUMS.txt` |
| `api-route-coverage.md` | 13 条路由：handler/守卫/DTO/错误语义/现有测试与断言位置/判定（覆盖 3 / 部分 5 / 未覆盖 5）+ 最小补测建议 | 按文中 `file:line` 逐条打开 `server/**/*_test.go` |
| `ci-metadata-correction.md` | CI job id、`-race` 耗时更正；四类 SHA 拆分；CI 三包 race 与服务器全仓 race 的边界 | 对照 `docs/r0-evidence-4th/ci-run/run-meta.json` 与 `test-job.log` 原文 |
| `ci-coverage-matrix.md` | **GitHub 自带 CI 能证明什么 / 不能证明什么**：逐项对照 gofmt、vet、build、全量单测、全仓 race、容器冒烟 vs 逐路由断言、A、B、C | 对照 `.github/workflows/ci.yml` 步骤与各 run 日志 |
| `ci-run-de2fb26/` | run `41983447389`（head `de2fb26`）：**全仓 `go test ./... -race`** 首次由 CI 执行，10 包 ok、0 FAIL / 0 DATA RACE | `gh run view 41983447389 --repo LinTia/FastLadger` |
| `ci-run-22fd310/` | run `41976312248`（head `22fd310`）：第五次整改提交的 CI 记录（`-race` 当时仍为 3 包） | `gh run view 41976312248 --repo LinTia/FastLadger` |

## 关键数字

- 数据集：5 账户 / 37 分类 / 22 交易 / 5 预算；两侧余额一致（¥28,100.00 / ¥0.00 / −¥716.30 / −¥1,125.00 / ¥3,868.00）。
- 截图：原型 52 + 实现 52 = 104（13 页 × 4 视口），两侧 `errors=0`。
- 分类映射：`id` 18 / `name-path` 3 / `name-prefix` 1 / 父类降级 **0**。
- 路由覆盖：覆盖 3 / 部分 5 / 未覆盖 5 = 13。
- CI：run `41954006789`，job `127004401500` / `127005322262`，两 job success。

## 状态

**R0 仍 BLOCK；A/C BLOCKED，B/D PARTIAL；不授权 R1–R7；RV-05 为 Owner ACTION REQUIRED。**
