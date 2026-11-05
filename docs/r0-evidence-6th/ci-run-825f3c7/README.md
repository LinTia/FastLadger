# CI run `41990345757` — R0 E2E 首次独立 CI 复跑（head `825f3c7`）

这是 `.github/workflows/r0-e2e.yml` 的**第一次运行**，也是 A（11 项）第一次由**独立 CI 环境**执行。

| 项 | 值 |
|---|---|
| run id | `41990345757` |
| run URL | https://github.com/LinTia/FastLadger/actions/runs/41990345757 |
| workflow | `R0 E2E`（`.github/workflows/r0-e2e.yml`） |
| head | `825f3c74834904d0fde6c3f980b48806be5b2404` |
| 触发 | `push` → `main` |
| 结论 | **success**（job 内 12 个步骤全绿，含 A 与 C） |
| 环境 | `ubuntu-latest`、`go1.22.12`、Node 22、Playwright 自带 **chromium 156.0.8078.4**、临时 SQLite + 随机端口 |

## 关键结果（与本地逐条一致）

| 步骤 | 结果 |
|---|---|
| 7 分类解析合成测试 | `# tests 14 / # pass 14 / # fail 0` |
| 8 分类映射回归审计 | `AUDIT_OK（id 17 / expand-child 4 / manual-override 1 / 父类兜底 0）` |
| 9 包 A · 11 项 | `SUMMARY {"A:PASS":11,"total":11}` |
| 10 包 C · 228 条 | `{"PASS":144,"FAIL":6,"BLOCKED":77,"N/A":1}` |

> 浏览器版本与本地不同（本地 Edge 156.0.4314.58 / CI chromium 156.0.8078.4），**结果完全相同**——说明结论不依赖某一台机器或某个浏览器构建。

## 文件

| 文件 | 说明 |
|---|---|
| `run-meta.json` | Actions API 返回的 run/job/step 元数据 |
| `job.log` | 完整原始 job 日志（58 KB，未改写） |
| `summary.txt` | 关键证明行摘录（自 job.log grep，保留原文） |
| `SHA256SUMS.txt` | 校验和（基于 LF 工作树；autocrlf clone 的文本哈希会因行尾规范化不同） |
