# P1 · 公开 Actions artifact 凭据泄露事件处置记录

> 性质：GitHub Actions artifact 安全事件（Public 仓库）。第三方复核（ChatGPT）在 PR #3 上提出，本文件为 team-lead 侧处置留痕。
> 处置时间：2026-11-05；仓库：`LinTia/FastLadger`（Public）；分支 `fix/r0-sixth-runtime-defects`。
> **本文件不写入任何 JWT / Bearer 原文，只记录 ID、命中类型与数量。**

---

## 1. 事件发现与性质

| 项 | 内容 |
|---|---|
| 发现方 | ChatGPT 第三方复核：实际下载公开 artifact `13407056751`，确认 12 个 `trace.zip` 中 10 个含 JWT 格式凭据，部分含 `Authorization Bearer` 信息 |
| 此前遗漏 | 上一轮 QA 已发现 `trace.zip` 含 JWT，但**仅避免把 trace 提交进 Git 仓库**，没有阻止 GitHub Actions 把原始 trace 上传为 artifact |
| 泄露载体 | Playwright `trace.network`，记录 E2E 运行时的真实 `Authorization: Bearer <JWT>` 请求头 |
| 凭据性质（如实界定） | 这些 JWT 来自**临时 E2E 实例**：`tools/r0/e2e/lib/server.js` 每次启动用随机端口 + 临时 SQLite + 每次新生成的 JWT 密钥，实例结束即销毁。因此**不是生产凭据、未涉及生产库或用户数据**；但明文出现在 Public 仓库的 artifact 中仍构成安全事件，第三方指出的问题成立 |

## 2. 影响范围核实（team-lead 独立下载扫描）

扫描方式：通过 GitHub API 下载 artifact zip，处理 302 到 blob 的重定向（重定向时**不带** GitHub token，否则返回 401），递归解压嵌套 `trace.zip`，对文本层匹配 JWT 三段式 / Bearer / Authorization 赋值。只统计命中数量，不落任何明文凭据。

| artifact ID | 创建时间(UTC) | 归类 | 大小 | 嵌套 zip | 凭据命中 | 含命中文件 |
|---|---|---|---|---|---|---|
| `13407302746` | 2026-11-05 10:37:34Z | 本轮 PR #3 | — | 同批产物 | 同批 | 同批 |
| `13407056751` | 2026-11-05 10:29:05Z | **本轮（第三方点名）** | 7,697,190 B | 12 | **95** | **18** |
| `13407016140` | 2026-11-05 10:26:33Z | 本轮 PR #3 | — | 同批产物 | 同批 | 同批 |
| `13406656838` | 2026-11-05 10:21:42Z | **本轮（第三方点名）** | — | 同批产物 | 同批 | 同批 |
| `13400687586` | 2026-11-05 08:47:47Z | 更早 run | 4,727,232 B | 12 | **95** | **18** |
| `13399959579` | 2026-11-05 08:50:00Z | 更早 run | — | 同批产物 | 同批 | 同批 |
| `13398291160` | 2026-11-05 07:55:16Z | 更早 run | — | 同批产物 | 同批 | 同批 |
| `13398012607` | 2026-11-05 07:59:45Z | 更早 run | 4,782,159 B | 12 | **95** | **18** |

> 「同批产物」= 由同一个 `path: artifacts/r0-e2e/**` 上传策略生成，抽样实测的 3 个（含第三方点名的 1 个 + 2 个历史）均为每 artifact 12 个 `trace.zip`、95 处命中，故其余同批 artifact 按同一风险处置；未逐个下载扫描的项如实标注，不假称已逐一验证内容。

## 3. 处置动作与回执

### 3.1 删除公开 artifact（GitHub REST API `DELETE /repos/.../actions/artifacts/{id}`）

| 批次 | artifact ID | HTTP 状态 |
|---|---|---|
| 本轮 4 个（第三方点名 + 同批） | `13407302746` / `13407056751` / `13407016140` / `13406656838` | **均 204** |
| 历史 4 个（按「尽快删除已知含凭据 artifact」扩大处置） | `13400687586` / `13399959579` / `13398291160` / `13398012607` | **均 204** |

删除后复核 `GET /actions/artifacts?per_page=100`：`total_count` 由 8 → 0（在 P1 新 CI 产生新 artifact 之前）。**共删除 8 个，全部有 204 回执，权限具备，无需 Owner 代操作。**

> 扩大处置说明：用户点名的是 `13406656838`、`13407056751`，并要求「至少检查并处理」。历史 4 个不在点名列表，但同属公开、由同一有缺陷上传策略生成，抽样实测确认含凭据后一并删除。此处明确报备，不假称在原始授权清单内。

### 3.2 上传策略整改（commit `0c62f5017c8b616f6d0c5079c55a9b4a13d1f68f`）

- 新增 `tools/r0/e2e/scripts/prepare-upload.js`：白名单收集 `png/md/json/log/txt`，排除 `trace.zip` 及所有 `*.zip/*.trace/*.network/*.har/*.db/server.log`、敏感词文件名、符号链接。
- 新增 `tools/r0/e2e/scripts/scan-artifacts.js`：上传前失败关闭扫描（JWT 三段式 / Bearer / Authorization / Cookie / 密钥赋值 + 文件名黑名单），命中即退出 2。
- `.github/workflows/r0-e2e.yml`：上传 step 的 `path` 改为白名单目录；**关键修复**——原 `if: always()` 会让「扫描失败即退出」完全失效（上传照样执行），改为 `if: ${{ always() && steps.artifact_scan.outcome == 'success' }}`，扫描失败或被跳过时上传根本不调度；`if-no-files-found: error`。
- 既有包 A 门禁、包 D 门禁、包 C 报告型（`continue-on-error: true`）语义未变。已核查 `ci.yml` 无 `upload-artifact` 步骤，全仓仅 `r0-e2e.yml` 一处上传。

### 3.3 重新运行 CI 并实际下载新 artifact 扫描

| 项 | 结果 |
|---|---|
| CI | [run 42029758226](https://github.com/LinTia/FastLadger/actions/runs/42029758226) **success** |
| R0 E2E | [run 42029758227](https://github.com/LinTia/FastLadger/actions/runs/42029758227) **success**（job `127252900777`） |
| 新 step 结论 | step 13 准备上传证据 success / step 14 上传前安全扫描 success / step 15 上传证据 success |
| 新 artifact | `13415542360`（`r0-e2e-evidence-57e5cd0d…`），3,629,800 B |
| **实际下载递归扫描** | 99 个文件、**0 个 `trace.zip` / 0 个黑名单文件**、19 个文本文件全扫、80 个二进制按扩展名跳过，JWT / Bearer / Authorization / Cookie / 密钥赋值 **命中 0** → **CLEAN** |
| 保留内容 | 截图 PNG、report.json/md、日志、校验和等不含凭据证据均保留 |

## 4. 未闭环与残余风险（如实登记，不得写成已解决）

1. **QA Round 1 独立复核发现扫描器 fail-open 漏洞（N1，高）**：`scan-artifacts.js` `looksBinary()` 在头 4096 字节见 NUL 即把白名单文本判二进制并**静默跳过全部文本规则返回 0 CLEAN**；QA 端到端实测 NUL 前缀 `.log` 可携带合成 JWT 被判 CLEAN 并上传。另有 `.png` 扩展名从不读内容（N2）、合法业务字段误阻断（N5）、CI summary 不含新 step outcome（N3）、报告 HEAD 不一致（N4）。**已路由 Engineer 修复，QA 将做 Round 2 回归；在回归通过前，扫描器不得宣称「已失败关闭」。**
2. **规则是已知形态白名单，非通用 DLP**：未带 JWT/Bearer/Authorization/Cookie 等形态的凭据不保证拦截，不得外推为「任何凭据都能拦」。
3. **绿色 CI ≠ 证据已安全上传**：必须逐 step 核对 `outcome`、到 artifacts 列表确认存在、并下载内容独立复核。C 的 `continue-on-error` 尤其不能解读为 228 条全部通过。
4. 截图**画面内**的凭据（像素文字）不在文本扫描能力内，属既有边界。
5. 本次删除的是 GitHub Actions artifact，**没有改写 Git 历史**；历史上曾入仓的 `network-log.json` 泄露事件（旧 SHA `259bd11`）仍是独立的 **RV-05 / Owner ACTION REQUIRED**，本事件不替代、也不关闭它。

## 5. 给第三方复核的核对清单

- [ ] 确认上述 8 个 artifact ID 当前已不可访问（`GET /actions/artifacts`）
- [ ] 下载 `13415542360` 独立复核 CLEAN
- [ ] 核对 run `42029758226` / `42029758227` 的 step 13/14/15 outcome
- [ ] 核对 `r0-e2e.yml` 上传 step 的 `if` 确实依赖 `artifact_scan.outcome`
- [ ] 核对 QA Round 2 回归结果，确认 N1/N2/N5 已修、无新的 fail-open 通道
- [ ] 确认本文件与仓库任何位置均无 JWT / Bearer 原文
