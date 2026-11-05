# R0 第六次整改证据（A/C 自动化 + B 分类解析硬化）

本目录是 `docs/FastLedger-R0-第六次整改-A-C自动化交付回执.md` 的证据落地。
所有文件由 `tools/r0/e2e` 在**真实服务 + 真实浏览器**下产生，`SHA256SUMS.txt` 可逐字节复核。

## 文件

| 文件 | 内容 | 复核方式 |
|---|---|---|
| `report.json` | A+C 全量 239 条记录（状态/说明/耗时/证据路径） | `node -e "..."` 或直接看 JSON |
| `report.md` | 同上的人类可读版（汇总表 + 明细表） | 打开即读 |
| `controls-summary.json` | 228 条状态分布 + 源文档 SHA256 | 与主报告 §3.1 行数比对 |
| `fixtures.json` | 本轮合成数据快照（用户名/账本/账户/分类 id；**不含任何令牌**） | 确认无 token 字段 |
| `unit-resolve-category.log` | 分类解析 14 条合成测试原始输出 | `node --test tools/r0/test-resolve-category.js` |
| `reseed-audit.log` | 22 笔分类映射 + 审计结论 | `node tools/r0/e2e/scripts/reseed-audit.js` |
| `screenshots/` | A 关键态与 C 失败项截图（含交易总表空白页） | 与 report.json 的 evidence 字段对应 |
| `SHA256SUMS.txt` | 15 个文件的校验和 | `sha256sum -c SHA256SUMS.txt` |

## 本地执行环境（本次证据）

| 项 | 值 |
|---|---|
| commit | `ebab48d`（本轮提交前基线） |
| Node | v22.22.2 |
| 浏览器 | `playwright-core` 1.64.1 + **本机 Edge 156.0.4314.58**（`channel=msedge`，不额外下载浏览器） |
| 服务 | 本仓 `server/fastledger.exe`（业务代码 `bff6a2b`），临时 SQLite + 随机端口 + 每次新密钥 |
| 视口 | 390×844（A 全场景、C 全控件） |

## 与 CI 证据的关系

本地证据用于**快速迭代与缺陷定位**；按任务单 §四，**A 的改判必须以独立 CI 跑通为准**。
CI 证据在 GitHub Actions 的 `r0-e2e-evidence-<sha>` artifact 中（含 trace.zip 与全部截图），本目录只放可入库的精简集。

## 已知边界

- 本地未跑 trace 之外的性能指标；A05 的"慢网"是 CDP 限速，不是真实弱网抖动。
- C 的 77 条 BLOCKED 逐条写明原因（见回执 §3.4），不含"交 Owner"这种空心理由。
- t021 分类改判（三餐→聚餐）导致 B 报表口径变化，**新旧截图/数字不可混用**。
