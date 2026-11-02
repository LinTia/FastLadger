# FastLedger 阶段 7：运维与验收报告

> WBS 阶段 7（7.1–7.5）与 PRD §10.1–§10.9 验收映射的逐项状态与证据。
> 报告日期：2026-11-02。诚实口径：每项标注证据来源（自动化测试 / 生产实测 / 未覆盖）。

## 一、WBS 7.1–7.5 状态

### 7.1 Docker / Caddy / DDNS — ✅ 交付（构建证据走 CI）

| 项 | 状态 | 证据 |
|---|---|---|
| Docker 单镜像 | ✅ | `deploy/Dockerfile`（多阶段 alpine+musl，CGO 编译 mattn/go-sqlite3；nonroot；HEALTHCHECK）；**真实构建+容器 healthz 冒烟在 GitHub CI `docker-build` job**（服务器未装 docker，本机未装，按用户约定不擅自安装） |
| `stop_grace_period: 30s` | ✅ | `deploy/docker-compose.yml`；应用侧优雅关机已实现（SIGTERM → `srv.Shutdown` 10s 超时 → SQLite 关闭最后连接自动 WAL checkpoint），main.go 注释明确"部署需 stop_grace_period ≥ 30s" |
| Caddy 外网 HTTPS | ⏸ 待域名 | compose 内 Caddy 服务已就绪（`profiles: ["tls"]`，`caddy reverse-proxy --from <域名> --to app:8080`，自动签发续期）。**阻塞项：生产为固定公网 IP 且未提供域名**——域名确定后 `CADDY_DOMAIN=… docker compose --profile tls up -d` 即可，无需其他改动。DDNS 对固定 IP 无意义，不适用 |

### 7.2 每日备份 — ✅（已运行 + 本批对齐保留口径）

| 项 | 状态 | 证据 |
|---|---|---|
| 一致性备份 | ✅ | `/opt/fastledger/backup.sh`：SQLite **online backup API**（等价 `.backup`，WBS 7.2 指定的两种安全方式之一），只读 URI 打开源库；**绝不做裸文件复制**（WAL 未 checkpoint 事务丢失红线） |
| 调度 | ✅ | `/etc/cron.d/fastledger`：每日 03:30（另有 03:10 `db purge` 回收）。**本批核对时发现记忆中"已在 cron"属实但保留口径不符** |
| 保留份数 | ✅ 已修正 | PRD §12 决策#5"滚动保留最近 7 份"——原脚本 30 天口径已收紧为 7 份（服务器脚本已更新并手动执行验证：`db-2026-11-02.db` 成功产出；仓库副本 `deploy/backup.sh` 同步） |
| 备份可用性 | ✅ | 最新备份只读校验：users=1, books=3, transactions=1, audit_logs=31（与生产一致） |

### 7.3 恢复演练 — ✅（2026-11-02 实测通过，全流程端到端）

演练记录（本月，按 `deploy/README.md` §3 流程，不影响生产）：

| 步骤 | 结果 |
|---|---|
| 恢复 `db-2026-11-02.db` 到临时目录 | ✅（0700/0600 权限口径保持） |
| `instance bump`（恢复后、启动前） | ✅ restore_epoch 1→2；审计落"备份恢复"事件（PRD §12 决策#8 覆盖备份恢复） |
| 临时实例启动（:18080） | ✅ healthz：`up / schemaVersion=8 / restoreEpoch=2`（客户端全量重建信号正确） |
| 记录数一致性 | ✅ users 1/1、books 3/3、book_members 3/3、transactions 1/1、token_sessions 23/23；audit_logs 恢复库=生产+1（即 bump 落的恢复事件审计，**预期行为**） |
| 审计可追溯 | ✅ 恢复库 `audit_logs` 可读（32 条，最新 at 为恢复时刻） |
| 服务活性 | ✅ 未带 token 请求 → 401（路由+鉴权正常） |
| 清理 | ✅ 临时目录删除；生产 healthz restoreEpoch=1 未变 |

演练中修正的两个运维坑（已写入 deploy/README.md §3）：`instance bump` 必须**同时传
`EBK_DB_PATH` 与 `EBK_DATA_DIR`**（否则审计静默落到 cwd 空库）；正确顺序为
**恢复 → bump → 再启动**（restoreEpoch 启动时读入内存）。

### 7.4 测试 — ✅（持续项）

- GitHub CI：`go test ./...` 全量 + `-race`（ws/service/api 三包）+ SQLite 真库冒烟 + gofmt/vet 门禁；本批新增 `docker-build` job；
- 服务器：`run_tests.sh` 803 PASS / 0 FAIL（2026-11-02）+ service/api race；
- 转账重试幂等 / 故障注入 / 回滚：阶段 2–4 测试覆盖（见 docs 各阶段报告）。

### 7.5 端到端验收 — ✅（见下节映射；带凭据的生产手动 E2E 见"诚实口径"）

## 二、PRD §10.1–§10.9 验收映射

| § | 验收项 | 自动化测试（本地全绿+CI） | 生产实测 | 结论 |
|---|---|---|---|---|
| 10.1 | 手动记账：列表/余额/报表更新 | tx CRUD/余额净额/报表聚合测试（service+api 800+ 例） | 报表/导出/预算/资产页已部署冒烟（401 鉴权面） | ✅ |
| 10.2 | 截图识别：预填与回退 | ocr parse/LLM 兜底/确认流测试；**绝不自动入账**断言 | OCR 端点生产在位（401）；端侧 Windows 流程无 VS 未运行验证（[1.47] 已如实登记） | ✅（客户端桌面运行时验证缺口如实保留） |
| 10.3 | 转账/还款：净资产守恒、删除回滚 | 转账双侧联动/删除回滚/重放幂等测试 | [5.9b] 生产端到端实测（净额精确/幂等/回滚） | ✅ |
| 10.4 | 支付渠道不重复：月度账单不重复计 | 渠道枚举与去重测试 | — | ✅ |
| 10.5 | 预算：阈值与状态切换 | 预算 CRUD/三态进度/大类聚合测试 | 端点生产在位（401） | ✅ |
| 10.6 | 导出：CSV 可被 Excel 正确打开 | BOM+CRLF 断言、公式注入防护测试 | 端点生产在位 | ✅（真实 Excel 打开为人工抽查项，BOM/CRLF 已由测试保证） |
| 10.7 | 家庭与可见性：关闭互看服务端生效 | `TestQAS3VisibilityCondWiredIntoAllReadPaths` 守卫：全部读路径（含报表/导出/预算聚合）AST 级验证 | — | ✅ |
| 10.8 | 投资与资产：市值/盈亏/净资产 | decimal 市值/盈亏/净资产公式（负债正=欠款）测试 | 端点生产在位（401） | ✅ |
| 10.9 | 备份恢复与审计 | — | **2026-11-02 演练通过**（见 7.3）；audit_logs 32 条可读可追溯 | ✅ |

## 三、诚实口径（未覆盖 / 待 owner 决策）

1. **外网 HTTPS 未启用**：阻塞于域名（未提供）。Caddy 配置就绪，域名到位即一键启用；当前外网暴露面仅腾讯云防火墙管控的端口。
2. **生产带凭据的手动 E2E**：按"不读 credentials"约定未做，各功能页面以"路由挂载 + 401 鉴权面"冒烟；§10 各项的深度断言由自动化测试承担。
3. **Windows 桌面端 OCR 运行时验证**：无 Visual Studio 环境，[1.47] 已登记，不在阶段 7 范围。
4. **服务器未安装 docker**（按用户约定不擅自安装软件）：容器化运行时证据由 CI `docker-build` job 承担；生产仍为 systemd 直跑形态（口径一致，`TimeoutStopSec=30` 等效）。

## 四、演练记录（滚动追加）

| 日期 | 备份 | 结果 |
|---|---|---|
| 2026-10-31 | db-2026-10-31 | 通过（首次演练，恢复+bump 流程验证） |
| 2026-11-02 | db-2026-11-02 | 通过（全流程：恢复→bump→临时实例→healthz restoreEpoch=2→记录数一致→审计可读→清理；修正 bump 环境变量坑） |
