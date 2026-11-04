# FastLedger R0 · D-8 裁决草案：Web 设置页「服务器信息 / 备份」区块（定稿版）

> 裁决日期：2026-11-04（Owner 定稿，2026-11-05 修订）；**技术方案已确定、功能未实现、无运行时验收**
> 关联：R0 主报告 §8 D-8；PRD §4.12 / §12；Web 设计规范 §8.1–8.2；部署交接 §7；落地方案 §8.3
> 关联缺陷：R0-E-17、I-SET-12

---

## 1. 文档目的与严格边界

本文件是 **D-8 的技术契约与裁决定稿**。仅记录需求依据、跨文档冲突核查、`/healthz` 实测、能力分级、后端契约、备份记录模型、交互草案与未决问题。

**严格边界（不得越界）：**

- 不修改任何业务代码、数据库结构、API、PRD 核心需求或 WBS。
- 不启动 R1–R7，不实施 PWA。
- **明确：技术方案已确定，功能仍未实现；本文件不构成运行时验收，不声称任何能力已部署或已验证。**
- 若发现实质冲突，单独列出交 Owner，不自行改写。

---

## 2. 需求依据

| 来源 | 条款 | 要求 |
|---|---|---|
| PRD §4.12 | 我的 / 设置 | **服务器信息、数据备份与恢复入口**；（管理员）系统级设置、备份策略 |
| Web 设计规范 §8.1 | 导航结构 | 设置（资料、API Token、**服务器信息、备份**） |
| Web 设计规范 §8.2 | 交互规格 | 设置：API Token 管理、**备份入口**、退出登录 |
| 部署交接 §7 | 备份与恢复 | 备份 = `data/` 目录（排除 `instance.meta`）；cron `sqlite3 .backup`；恢复 = 停服→换库→`instance bump`→启动 |

四份文档在"设置页应含服务器信息 + 备份入口"上**一致，无实质冲突**；开放问题为安全实现契约（即 D-8）。

---

## 3. `/healthz` 实测与边界（源码实读 + 历史实测）

**代码位置**：`server/internal/api/router.go:109–120`。

**真实返回**：

```json
{ "code": 0, "msg": "ok",
  "data": {
    "status": "up",
    "schemaVersion": <int>,
    "instanceId": "<随机固定值，仅当 instance.meta 存在>",
    "restoreEpoch": <int，仅当 instance.meta 存在>
  }}
```

- **公开无鉴权**：在 `NoRoute` 之前显式注册，任何请求均可访问；不计入 UI 控件验收分母。
- **安全边界**：仅暴露 `status` / `schemaVersion` / `instanceId` / `restoreEpoch`；**不暴露** 文件路径、DB 内容、`EBK_*` 环境变量、cron 日志、密钥、备份目录。
- **`restoreEpoch` 语义**：实例被从备份恢复的次数计数（`instance.meta`，不随备份回滚），**不是**备份成功标志；**禁止**用它推断"备份是否成功"。
- **D-8 不改 `/healthz`**：它仍供同步客户端做世代检测；但 **Web 设置页默认不展示 `instanceId` / `restoreEpoch`**（见 §6）。`/healthz` 公开暴露这两个字段是否需收敛，列为未决问题（§9）。

---

## 4. 服务器信息（首期展示范围，Owner 定稿）

普通登录用户**首期只展示**：

1. **服务可用性**（由进程存活 / API 可达推导）
2. **服务版本**（构建版本）
3. **数据库结构版本**（`schemaVersion`）

**默认不展示**：`instanceId`、`restoreEpoch`、`dataSizeBytes`、备份路径、数据库路径、内部日志。

> 推荐 `GET /api/v1/system/info`（鉴权、只读）仅返回上述三项；**不返回** `instanceId` / `restoreEpoch` / 路径 / 日志。`/healthz` 的公开字段与 UI 展示范围是两件事，不得混为一谈。

---

## 5. 权限模型（Owner 定稿）

- **账本 Owner/Admin 不自动拥有实例级权限**（与 PRD §12 正交）。
- **备份状态 API 仅允许 `is_instance_admin`**。
- **服务端必须做权限校验，不能依赖前端隐藏控件**：任何实例级路由须 `requireInstanceAdmin` 中间件（基于 JWT `isInstanceAdmin` claim）。
- **未授权请求必须拒绝**（如 403），**且不得泄露备份记录**（不得返回条数/时间/路径等任何信息）。

---

## 6. 备份记录（Owner 定稿，真实执行记录）

- 备份状态**必须来自服务器端备份任务的真实执行记录**。
- **不允许通过文件 `mtime` / `size` 推断备份成功**。
- **一致性快照**：优先采用 **SQLite Online Backup API**；现有 `sqlite3 .backup` 可保留，但**必须验证结果**。
- **备份后校验**：对副本执行 `PRAGMA integrity_check`；成功后计算 **SHA-256**、**文件大小**、**schemaVersion**。
- **发布**：通过**临时 manifest + 原子重命名**发布成功记录（避免半截 manifest 被读成成功）。
- **失败**：失败任务**只能产生 `failed` 记录**，**不得更新 `last_success_at`**。
- **数量**：备份数量**只能从成功 manifest 记录统计**。
- **两种状态分开**：`integrity_check` 通过 **≠** 恢复演练通过；**必须分开记录**（见 §6.2）。

### 6.1 manifest 建议字段（草案，未实现）

```
backup_id, started_at, finished_at, status(success|failed),
file_path(内部，不对外), size_bytes, sha256, schema_version,
integrity_check(ok|failed|not_run)
```

### 6.2 恢复演练状态（独立于完整性检查）

```
restore_drill(not_run|passed|failed), restore_drill_at
```

> `integrity_check=ok` 只表示备份文件结构完好；**备份可用**还需恢复演练（新实例恢复→校验记录数/关键汇总→`instance bump`→客户端全量重建）。两者**不得合并**为一个"备份正常"状态。

---

## 7. 能力分级

### 7.1 当前已有（代码/部署已落地，可验证）

| 能力 | 状态 | 证据 |
|---|---|---|
| 实例管理员身份模型 | 已实现（数据 + JWT claim） | `users.is_instance_admin`（迁移 0001）；首个用户默认（`auth.go:251`）；JWT 注入 `isInstanceAdmin`（`middleware.go:202`） |
| `/healthz` 公开只读健康检查 | 已实现 | `router.go:109–120` |
| cron 备份（`sqlite3 .backup`） | 已部署（运维侧，**无元数据**） | 部署交接 §7.4 |
| 恢复流程（运维步骤） | 已定义 | 部署交接 §7 / 落地方案 §8.3 |

### 7.2 建议新增（R 阶段，未实现）

| 能力 | 说明 |
|---|---|
| `GET /api/v1/system/info`（鉴权、只读） | 仅返回 服务可用性 / 服务版本 / schemaVersion |
| `GET /api/v1/system/backup/status`（**仅实例管理员**） | 来自真实 manifest：`last_success_at` / `backup_count`（成功记录）/ `last_size` / `last_sha256` / `integrity_check` / `restore_drill` |
| `requireInstanceAdmin` 守卫 | 服务端强制，不依赖前端隐藏 |
| 备份 manifest 写入器 | Online Backup / `.backup` + `integrity_check` + SHA-256 + size + schemaVersion + **临时文件原子重命名** |
| 失败记录 | `failed` 记录，不更新 `last_success_at` |
| 恢复演练状态存储 | 与 `integrity_check` **分开** |

### 7.3 未来扩展（首期不开放）

Web 一键恢复、删除备份、修改目录、整库下载、手动触发备份。

---

## 8. 设置页交互草案

### 8.1 服务器信息（所有已登录用户，只读）

| 字段 | 来源 |
|---|---|
| 服务可用性 | 进程存活 / API 可达 |
| 服务版本 | `system/info` |
| 数据库结构版本 | `schemaVersion` |

**不展示** `instanceId` / `restoreEpoch` / `dataSizeBytes` / 路径 / 日志。

### 8.2 数据备份（**仅实例管理员**）

- **正常态**：`last_success_at`、备份份数（仅成功 manifest）、最近备份大小、（可选）SHA-256 摘要、`integrity_check` 状态。
- **恢复演练态**：**独立展示** `restore_drill`（未演练 / 通过 / 失败），不得与完整性检查合并。
- **无数据态（未接入）**：「暂未提供状态查询（请联系服务器管理员）」——**不推测成功**。
- **错误态**：红色"状态获取失败"，**不回退到"备份成功"假象**。
- **权限不足态**：区块隐藏，或「仅服务器管理员可见」。
- **首期不放按钮**：恢复 / 删备 / 改目录 / 整库下载 / 手动触发备份。

---

## 9. 未决技术问题（交 Owner / 后续实现确认）

1. **`/healthz` 公开字段**：是否继续公开 `instanceId` / `restoreEpoch`（同步客户端依赖），还是收敛为仅 `status` / `schemaVersion`；若收敛需同步客户端改造。
2. **Online Backup API 落地方式**：`mattn/go-sqlite3` 的 backup API / CGO `sqlite3_backup` / `VACUUM INTO` / `sqlite3 .backup` 的选型与验证。
3. **`integrity_check` 成本与锁**：大库上的耗时/锁表风险，是否需离线窗口。
4. **manifest 存储位置与权限**：文件路径、0600、目录约定；原子重命名的跨文件系统限制。
5. **SHA-256 / 大小 / schemaVersion 采集时机**：从副本还是源库取 `schema_version`。
6. **备份数量语义**：保留份数（14~30）与"成功记录数"的口径。
7. **恢复演练状态存储**：独立表 vs manifest 字段；由谁写入（运维流程）。
8. **`requireInstanceAdmin` 中间件**：错误码（403）与不泄露策略的具体实现。
9. **服务版本来源**：构建版本注入方式（ldflags / version 包）。

> 以上为**未决技术问题**，不影响本轮"技术方案已确定"的裁决性质；实现前需逐项收敛。

---

## 10. 裁决结论

D-8 **技术方案已确定**：

1. 设置页保留「服务器信息」+「数据备份」两区块。
2. 服务器信息首期只读，普通用户仅见 **服务可用性 / 服务版本 / 数据库结构版本**；默认不展示 `instanceId` / `restoreEpoch` / `dataSizeBytes` / 路径 / 日志。
3. 备份状态仅 `is_instance_admin` 可见，**服务端强制校验**，未授权拒绝且不泄露；数据**必须来自真实执行记录**（manifest），不得用 `mtime`/`size` 推断。
4. 账本 Owner/Admin ≠ 实例管理员。
5. 备份采用一致性快照 + `integrity_check` + SHA-256/大小/schemaVersion，**临时 manifest 原子发布**；失败只记 `failed`、不动 `last_success_at`；数量只数成功记录；**`integrity_check` 与恢复演练分开**。
6. 首期不开放 Web 恢复/删备/改目录/整库下载/手动触发；恢复走运维流程。

**功能仍未实现，无运行时验收**；实现属 R 阶段。

---

## 11. 诚实声明

- 本文件为**技术契约与裁决**，**不含任何代码改动**；所列能力均「未实现 / 待 R 阶段」，**无运行时验收**。
- `/healthz` 事实来自 `router.go:109–120` 源码实读 + [1.65] 历史运行实测；本轮未重新 curl。
- R0 未通过 / 阻塞状态不因本裁决改变；不得被误读为已完成。
