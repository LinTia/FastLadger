# FastLedger R0 · 第九次独立复核 RV-13 / RV-14 / RV-15 整改报告

- 权威审计：`docs/FastLedger-R0-第九次安全扫描器交付独立复核.md`（ChatGPT 提交到 main @ `0d8f909`，**结论未改动**）
- 审计对象 PR #3 HEAD：`2699c9c3db2e40915082da2acbe2392ad40650de`
- 修复分支：`fix/r0-sixth-runtime-defects`（**不提交 main、不合并 PR、不宣布 R0 通过**）
- 本轮 PR 最新 HEAD SHA：`1e2d70c0fc7d57d830b86411bc5fbdcedac527ae`
- 本轮新增提交：
  - `991ab09b8409a17984ced91737cabbffc6c9b0f5`（RV-13/14/15 代码、测试、workflow、原始证据日志）
  - `1e2d70c0fc7d57d830b86411bc5fbdcedac527ae`（gate workflow 注册触发器 / 分场景 dispatch 准备）
- 审计结论处置：本轮只处理 RV-13 / RV-14 / RV-15；A05、C 硬编码 PASS、t021、OBS-ACCT-VIS-390、RV-05 **均未改判，保持原状态**。

---

## 0. 逐项验收结论（本轮自定义状态，不代替第三方复核）

| 项 | 状态 | 一句话依据 |
|---|---|---|
| **RV-13** | **PASS** | 重设计为「唯一权威根清单」；本地 16/16 断言（含篡改/删除负向）+ 真实 CI Artifact 独立复验：仅 1 份清单、101/101 文件递归字节校验通过、0 missing / 0 mismatch / 0 失效引用 / 0 嵌套清单残留 |
| **RV-14** | **PASS** | 去掉 `if (!knownBinary)` 跳过分支；修复前基线对 3 个 PNG 样本 CLEAN(rc=0)，修复后全 BLOCK(rc=2)；正常 PNG 与业务文字 PNG 仍 CLEAN；50/50 测试与 13/13 证据断言通过 |
| **RV-15** | **PASS** | 独立 GitHub Actions 测试 workflow，4 个真实 run：正向产生 1 个 Artifact；3 个负向场景真实 `actions/upload-artifact` step 均 `skipped`，Artifact API `total_count=0` |

> 上述 PASS 仅表示「本轮代码、测试与运行证据齐备」，**不等于第三方验收通过**；R0 仍 BLOCK，等待 ChatGPT 独立复核。

---

## 1. RV-13 · 修复全部层级 SHA256SUMS 清单

### 根因
`prepare-upload.js` 旧实现 `manifestRels = collected.filter(rel => path.basename(rel) !== 'SHA256SUMS.txt')` 按 **basename 全局排除**所有同名清单；且只重写根 `dest/SHA256SUMS.txt`。因此 A/D/C 子目录旧清单被当作普通 `.txt` 白名单文件**原样复制**，其 12 条指向已过滤 `trace.zip` 的失效引用全部保留；根清单又未覆盖这 3 份嵌套清单。复核报告 §3 实测 Artifact 4 份清单 / A 子清单 12 条失效引用。

### 修改（单一权威根清单，职责唯一）
1. `classify()` 在白名单收敛阶段把**任意层级**的 `SHA256SUMS.txt` 全部排除，理由记为「旧/嵌套校验清单（由唯一权威根清单统一重建）」。旧清单绝不进入上传目录 → 从根上杜绝失效引用与残留。
2. 新增 `tools/r0/e2e/scripts/lib/manifest.js`：
   - `buildRootManifest(dest)` 重新递归遍历**最终目录**，对每一个实际文件（含任意子目录）计算实际字节 SHA256，写入根 `SHA256SUMS.txt`。
   - `verifyUploadTree(dest)` **递归实际字节校验**：重新遍历目录、重新读取每个文件字节、重新计算摘要，逐条比对；检查 `missing / mismatch / uncovered / staleRefs / nestedManifests / malformed / selfRef / symlinks`，任一异常失败关闭。
   - 这不是「检查清单格式或记录数量」，而是对字节的重新哈希比对。
3. **职责划分（明确、无重复、无循环）**：只有一份清单 = 根清单；嵌套清单无任何职责且不存在；根清单不列出自身、不哈希自身（**唯一例外**，在代码注释、CLI 输出、本报告三处显式声明）。因此无重复条目、无自引用 / 循环引用。
4. 新增独立 CLI `verify-upload-manifest.js`（退出码 0 通过 / 2 校验失败 / 1 环境错误），供 CI、独立复验与负向测试复用。
5. `r0-e2e.yml` 新增 step「最终目录递归字节校验（RV-13，独立 CLI）」，并让 `artifact_scan` 依赖其成功，形成失败关闭链。

### 测试与结果
- 新增 `rv13-fixture.js`：构造**根普通文件 + A/D/C 子目录普通文件 + 各级旧 SHA256SUMS + trace.zip + 其它 10 类被排除文件**（共 24 个源文件、12 条镜像复核报告的失效 trace 引用）。
- 新增 `rv13-manifest-evidence.js`：真实子进程执行 `prepare-upload.js` 与 `verify-upload-manifest.js`，打印原始 stdout / 退出码 / **最终目录完整清单** / 根清单全文；含篡改与删除负向。
- 新增 case-48 / 49 / 50（node:test，调用真实脚本）。
- 原始证据：`docs/r0-evidence-9th-fix/rv13-manifest-evidence.txt`

| 验收项 | 结果 |
|---|---|
| prepare-upload.js 退出码 | `0` |
| 所有上传文件均可验证（uncovered=0） | PASS |
| 0 missing | PASS（`missing=0`） |
| 0 mismatch | PASS（`mismatch=0`） |
| 不存在指向 trace.zip / 其它被排除文件的引用 | PASS（`staleRefs=0`，根清单全文无 trace/zip/db/har/server.log） |
| 不存在旧清单残留 | PASS（`nestedManifests=0`，最终目录仅 1 份 `SHA256SUMS.txt`） |
| 清单自身处理规则明确 | PASS（根清单为唯一例外，不列出自身，`selfRef=false`） |
| 故意篡改一个最终文件 → 校验失败 | PASS（verify rc=2，`[MISMATCH] A/a-report.md`） |
| 故意删除一个最终文件 → 校验失败 | PASS（verify rc=2，`[MISSING] D/d-report.md`） |
| 恢复后再次校验 | PASS（rc=0，10/10 通过） |
| 根清单条目数 = 最终文件数 - 1 | PASS（`10 = 11 - 1`） |

**证据脚本合计 16 项断言，PASS 16 / FAIL 0，退出码 0。**

### 真实 CI Artifact 独立复验
下载生产 E2E run `42211926674` 的 Artifact `13499611257`（3,652,445 B）并解压：

| 检查 | 第九次复核时（旧） | 本轮（修复后） |
|---|---:|---:|
| `SHA256SUMS.txt` 份数 | 4（根 + A/D/C） | **1（仅根）** |
| A 子清单失效引用 | 12 | **0（嵌套清单不存在）** |
| 最终文件总数 | 102 | 102 |
| 根清单条目 | 98（未覆盖子清单） | **101（= 102 − 根清单自身）** |
| `sha256sum -c` | 范围过窄 | **101 OK / 0 FAILED** |
| 独立 CLI 递归字节校验 | — | **0 missing / 0 mismatch / 0 uncovered / 0 staleRefs / 0 nestedManifests，rc=0** |
| 最终扫描器独立复扫 Artifact | CLEAN | **102 文件全量扫描、0 跳过、0 命中、CLEAN(rc=0)** |

原始证据：`docs/r0-evidence-9th-fix/rv13-artifact-independent-verify.txt`

---

## 2. RV-14 · 修复已知二进制文件的扫描盲区

### 根因
`scan-artifacts.js:505` 的噪声归一化复扫被 `if (!knownBinary)` 门控。以合法 PNG 魔数开头的文件 `knownBinary=true` → 完全跳过复扫；NUL 是合法 UTF-8 字节（`u8.reliable=true`）→ 不进入 UNDECODABLE；原始 latin1/UTF-8 扫描因关键词 / token 被 NUL 打断而不命中 → **CLEAN 放行**。复核 §4 标注为 P1 静态路径，本轮已用真实扫描器运行时复现。

### 修改（明确的二进制检测策略，非一刀切阻断）
1. **去掉跳过分支**：所有文件都经过噪声归一化复扫，无 `knownBinary` 豁免。
2. 文本型文件：沿用全文件噪声剔除复扫（保持 K1-03 / RV-09 / RV-11 行为不变）。
3. 已知二进制文件：新增 `extractTextFragments(buf)` 提取「可打印文本片段」，**桥接片段内连续 ≤ 4 字节的噪声**（NUL / 控制符 / 高位非法字节）以恢复被打断的关键词与 token 值；连续 > 4 字节噪声视为片段边界，避免把相距很远的随机可打印字节硬拼成误报串。
4. **误报控制**：新增 `BINARY_CREDENTIAL_ANCHOR_RE` 闸门——片段必须含凭据锚点词（bearer / authoriz / api_key / token / secret / password / eyJ / PEM 头 / EBK_JWT_SECRET / cookie / session 等）才施加规则；真实 PNG 压缩随机字节不会自然拼出锚点。
5. 新增 `scanBinaryFragment()`：因噪声桥接会拼掉原本的词边界（如 `Comment\0Bearer...` → `CommentBearer...`），补充一组「不要求词边界、但仍要求完整凭据形态」的规则（Bearer + ≥12 位 token 字符、Authorization/Bearer 赋值、强敏感键名 + `[:=]` + 值），复用 `credentialLikeValue` / `sensitiveAssignValue` 抑制占位值与自然语言误报。
6. **绝不把二进制文件一律判 BLOCKED**：`UNDECODABLE` 仍只针对文本型文件；正常 PNG 保持 CLEAN。扫描输出仍只含文件路径 / 命中类型 / 数量，绝不输出匹配值。

### 测试与结果
新增 `lib/png-fixture.js`（结构完整 PNG：8 字节魔数 + IHDR + IDAT + tEXt + IEND，逐 chunk CRC32）与 `rv14-binary-evidence.js`（从 `git show 2699c9c` 取修复前扫描器，与修复后扫描器跑同一批样本）。

| 样本 | 修复前（基线 2699c9c） | 修复后 | 命中类型 |
|---|---|---|---|
| 合法 PNG + 明文 Authorization/Bearer | CLEAN (rc=0) | **BLOCKED (rc=2)** | BEARER_TOKEN / AUTH_HEADER |
| 合法 PNG + token 值内部 NUL | **CLEAN (rc=0)** | **BLOCKED (rc=2)** | BEARER_TOKEN |
| 合法 PNG + Bearer 关键词内部 NUL | **CLEAN (rc=0)** | **BLOCKED (rc=2)** | BEARER_TOKEN |
| 合法 PNG + x-api-key | **CLEAN (rc=0)** | **BLOCKED (rc=2)** | SENSITIVE_ASSIGN |
| 正常 PNG（含 4 KiB 高熵压缩数据，无凭据） | CLEAN | **CLEAN (rc=0)** | — |
| 合法 PNG + 普通业务文字（unit/note/status/qty） | CLEAN | **CLEAN (rc=0)** | — |

- 「修复前 CLEAN 绕过样本数 ≥ 2」实测 **3**（盲区真实可复现，非仅静态推断）。
- 前四项全部 BLOCKED；正常 PNG 与普通业务文字 PNG 均 CLEAN；全部样本扫描输出不含匹配值。
- 新增 case-42~47（node:test，**调用真实扫描器子进程**而非独立正则）。
- 原始证据：`docs/r0-evidence-9th-fix/rv14-binary-evidence.txt`（13 项断言 PASS 13 / FAIL 0，退出码 0）。

---

## 3. RV-15 · 证明 GitHub Actions 上传门禁真实有效

### 根因
复核 §4 指出：`r0-e2e.yml` 的 `fail_closed_negative` 只用破损扫描器驱动 helper 内部布尔值，既未让作为 upload 前置条件的**真实 step** 失败，也未读取真实 `steps.artifact_upload.outcome`，更未核对本次 run 的 Artifact 数量；「Artifact 数量 = 0」只是 helper 固定文本。

### 修改
新增**独立 GitHub Actions 测试 workflow** `.github/workflows/r0-upload-gate.yml`（name `R0 Upload Gate (RV-15)`，numeric id `409263438`）：
- `workflow_dispatch` 输入 `scenario`（4 选 1）精确控制「哪一个**真实前置 step** 失败」，由 `rv15-gate-fixture.js` 用可控输入驱动真实 `prepare-upload.js` / `scan-artifacts.js` / `verify-fail-closed-gate.js`，**不是旁路 helper**。
- `upload` step 是真实 `actions/upload-artifact@v4`，`if` 条件与生产 `r0-e2e.yml` **完全一致**：`always() && steps.artifact_scan.outcome == 'success' && steps.fail_closed_check.outcome == 'success'`。
- `rv15-gate-assert.js` 在 run 内读取 GitHub Actions 实测 `${{ steps.<id>.outcome }}` 并断言（正向 upload=success；负向 upload=skipped），**不是本地模拟布尔值**。
- 为让每个负向场景单独成一次 run（从而可用 Artifact API 直接证明该 run 数量为 0），新增 paths 过滤 `push` 触发器使新 workflow 注册；注册后按 numeric id + `ref=分支` 逐场景 dispatch。

### 真实 CI 结果（每个场景独立 run，HEAD `1e2d70c`）

| 场景 | Run ID / URL | prepare | scan | fail_closed_check | **真实 upload step** | gate_assert | run 结论 | **Artifact API total_count** |
|---|---|---|---|---|---|---|---|---|
| positive | [42212399478](https://github.com/LinTia/FastLadger/actions/runs/42212399478) | success | success | success | **success** | success | success | **1**（Artifact ID `13498773757`，`r0-gate-positive-42212399478`，909 B） |
| scan_fail | [42212414373](https://github.com/LinTia/FastLadger/actions/runs/42212414373) | success | **failure** | success | **skipped** | success | failure（预期） | **0** |
| fail_closed_fail | [42212424538](https://github.com/LinTia/FastLadger/actions/runs/42212424538) | success | success | **failure** | **skipped** | success | failure（预期） | **0** |
| prepare_fail | [42212434547](https://github.com/LinTia/FastLadger/actions/runs/42212434547) | **failure** | skipped | success | **skipped** | success | failure（预期） | **0** |

补充：push 触发注册 run [42212298540](https://github.com/LinTia/FastLadger/actions/runs/42212298540) 为默认 positive，`success`，Artifact API `total_count=1`（ID `13500245578`）。

> 三个负向 run 的 `conclusion=failure` 是**刻意让真实前置 step 失败**的预期结果，且 `gate_assert` 全部 success 证明实测 outcome 符合门禁预期；这正是「gate 生效」而非「测试失败」。

- **正向上传 Artifact ID**：`13498773757`
- **负向场景无 Artifact 证据**：Artifact API 对 3 个负向 run 均返回 `total_count=0`（无 artifact 条目）；对应真实 upload step outcome 全部 `skipped`。
- 原始证据：`docs/r0-evidence-9th-fix/rv15-gate-ci-evidence.txt`（含每个 run 的 status / conclusion / step outcome / Artifact API 原始输出）
- 区分声明：第八轮的 `fail_closed_negative` 是 **helper 单元测试**（仍在 `r0-e2e.yml` 中保留作回归），本轮 `r0-upload-gate.yml` 是 **GitHub Actions upload step 集成测试**；两者性质不同，后者才是 RV-15 要求的证据。

---

## 4. CI / 运行证据汇总

| 证据项 | 值 |
|---|---|
| PR #3 最新 HEAD SHA | `1e2d70c0fc7d57d830b86411bc5fbdcedac527ae`（state=open, merged=false） |
| 新增提交 SHA | `991ab09b8409a17984ced91737cabbffc6c9b0f5`、`1e2d70c0fc7d57d830b86411bc5fbdcedac527ae` |
| 生产 R0 E2E CI Run | [42211926674](https://github.com/LinTia/FastLadger/actions/runs/42211926674) **success**（22 个主 step 全 success，含 50 项门禁、证据日志、递归字节校验、A/D/C/B 套件） |
| CI 全仓 `-race` / 容器冒烟 Run | [42213272740](https://github.com/LinTia/FastLadger/actions/runs/42213272740) **success**（head `1e2d70c`，含 go test -race 全仓） |
| 生产 Artifact ID | `13499611257`（`r0-e2e-evidence-991ab09…`，3,652,445 B） |
| RV-15 gate CI Runs | 42212399478 / 42212414373 / 42212424538 / 42212434547（+ 注册 run 42212298540） |
| 正向 gate Artifact ID | `13498773757` |
| RV-15 负向 run Artifact 数量 | 全部 `total_count=0` |

原始日志：
- `docs/r0-evidence-9th-fix/scan-tests-round5.log`（50/50 PASS，退出码 0）
- `docs/r0-evidence-9th-fix/rv13-manifest-evidence.txt`（16/16，含原始输出 / 退出码 / 最终目录清单 / 篡改与删除负向）
- `docs/r0-evidence-9th-fix/rv14-binary-evidence.txt`（13/13，修复前后对照）
- `docs/r0-evidence-9th-fix/rv13-artifact-independent-verify.txt`（真实 Artifact 独立复验）
- `docs/r0-evidence-9th-fix/rv15-gate-ci-evidence.txt`（真实 CI step outcome + Artifact API）
- `docs/r0-evidence-9th-fix/e2e-regression-ci-evidence.txt`（生产 E2E 全 step）
- `docs/r0-evidence-9th-fix/ci-race-regression-evidence.txt`（CI 全仓 `-race` / 容器冒烟全 step）

---

## 5. 完整回归测试与「未执行的测试」

### 已执行
| 测试 | 结果 |
|---|---|
| `node --test tools/r0/e2e/test-scan-artifacts.js` | **50 / 50 PASS**，退出码 0（case-01~50，含本轮 case-42~50） |
| `node --test tools/r0/test-resolve-category.js`（B 分类解析） | **14 / 14 PASS**，退出码 0 |
| `rv13-manifest-evidence.js` | **16 / 16 PASS**，退出码 0 |
| `rv14-binary-evidence.js` | **13 / 13 PASS**，退出码 0 |
| `verify-fail-closed-gate.js` 正向（真实扫描器） | `CRED_RC=2 CLEAN_RC=0 upload_allowed=true`，rc=0 |
| `verify-fail-closed-gate.js` 负向（破损扫描器 `--expect-skip`） | `upload_allowed=false`，rc=0 |
| `verify-fail-closed-gate.js` 正向 + 破损扫描器 | 判 FAIL-OPEN，rc=1（预期失败） |
| `before-after-evidence.js`（基线 659c0c3） | 14 / 15 可复现绕过已封死（K2-04 正确 CLEAN），退出码 0 |
| RV-15 四场景本地逻辑模拟 | 4 / 4 断言通过（**仅用于本地预检，不作为 CI 证据**） |
| 生产 R0 E2E CI（Go 构建 / Playwright / B / A 11 项 / D 39 项 / C 228 条 / 上传前扫描 / fail-closed / 上传） | run 42211926674 全绿 |
| CI 全仓构建 / vet / test / **全仓 `-race`** / 容器冒烟（head `1e2d70c`） | [run 42213272740](https://github.com/LinTia/FastLadger/actions/runs/42213272740) **success**：gofmt / go vet / go build / SQLite 驱动冒烟 / go test 全量 / **go test -race（全仓）** / docker build+healthz 全部 success |
| 真实 CI Artifact 下载 + `sha256sum -c` + 独立递归字节校验 + 最终扫描器复扫 | 101 OK / 0 FAILED / CLEAN(rc=0) |

原始日志补充：`docs/r0-evidence-9th-fix/ci-race-regression-evidence.txt`

### 未执行（如实说明）
- **本地 `go test ./... -race` 未执行**：本机 Windows 环境 TSan 不可用（历史记录 `TSan error 87`），全仓竞态改由 GitHub 原生 CI `ci.yml` 在本轮最终 HEAD `1e2d70c` 上重跑证明（run `42213272740`，`-race` step success）。本轮改动不涉及任何 Go 代码，该 CI 属完整回归补跑。
- **未重跑 A05 / C 228 条控件逐条状态改判**：本轮未授权修改 A05、C 硬编码 PASS、t021、OBS-ACCT-VIS-390；生产 E2E 中 A/D/C 套件按既有实现照常运行，其报告值（A 11/11、D 39/39、C PASS 164 / FAIL 4 / BLOCKED 59 / N/A 1）**不视为本轮改判或验收通过**。
- **未执行本地 Playwright 浏览器套件**：A/C/D 需要真实浏览器与服务进程，本轮仅在 GitHub Actions runner 上执行。
- **未改仓库可见性 / Git 历史 / 业务代码 / CSS / DB / API 契约**。

---

## 6. 所有修改文件列表

### 修改（4）
| 文件 | 说明 |
|---|---|
| `.github/workflows/r0-e2e.yml` | 门禁 step 更名 50 项；新增「生成 RV-13/RV-14 原始证据日志」「最终目录递归字节校验（RV-13 独立 CLI）」step；`artifact_scan` 依赖递归校验成功；summary 记录其 outcome |
| `tools/r0/e2e/scripts/prepare-upload.js` | RV-13：白名单阶段排除任意层级 `SHA256SUMS.txt`；改用 `lib/manifest.js` 生成唯一权威根清单 + 递归实际字节校验（失败关闭） |
| `tools/r0/e2e/scripts/scan-artifacts.js` | RV-14：去掉 `knownBinary` 噪声复扫跳过分支；新增片段提取 / 锚点闸门 / 桥接规则；扫描口径输出更新 |
| `tools/r0/e2e/test-scan-artifacts.js` | 新增 case-42~50（RV-14 六项、RV-13 多层 / 篡改 / 删除共三项），测试扩至 50 项 |

### 新增（9 个代码/workflow + 7 个证据文件 + 1 份报告）
| 文件 | 说明 |
|---|---|
| `.github/workflows/r0-upload-gate.yml` | RV-15 独立测试 workflow（4 场景真实 upload step 负向验证） |
| `tools/r0/e2e/scripts/lib/manifest.js` | 唯一权威根清单生成 + 递归实际字节校验共享库 |
| `tools/r0/e2e/scripts/lib/png-fixture.js` | 结构完整 PNG 合成样本构造器（CRC32 / tEXt / 高熵尾） |
| `tools/r0/e2e/scripts/lib/rv13-fixture.js` | RV-13 多层目录夹具（根/A/D/C 旧清单 + trace.zip + 其它排除文件） |
| `tools/r0/e2e/scripts/verify-upload-manifest.js` | 独立递归字节校验 CLI（0/2/1 退出码） |
| `tools/r0/e2e/scripts/rv13-manifest-evidence.js` | RV-13 原始证据生成器 |
| `tools/r0/e2e/scripts/rv14-binary-evidence.js` | RV-14 修复前后对照证据生成器 |
| `tools/r0/e2e/scripts/rv15-gate-fixture.js` | RV-15 可控输入驱动器（真实 prepare/scan/gate） |
| `tools/r0/e2e/scripts/rv15-gate-assert.js` | RV-15 真实 step outcome 断言 |
| `docs/r0-evidence-9th-fix/scan-tests-round5.log` | 50 项测试原始日志 |
| `docs/r0-evidence-9th-fix/rv13-manifest-evidence.txt` | RV-13 原始输出 / 退出码 / 最终目录清单 / 负向 |
| `docs/r0-evidence-9th-fix/rv14-binary-evidence.txt` | RV-14 修复前后对照原始日志 |
| `docs/r0-evidence-9th-fix/rv13-artifact-independent-verify.txt` | 真实 Artifact 独立复验 |
| `docs/r0-evidence-9th-fix/rv15-gate-ci-evidence.txt` | RV-15 CI step outcome + Artifact API |
| `docs/r0-evidence-9th-fix/e2e-regression-ci-evidence.txt` | 生产 E2E 全 step |
| `docs/r0-evidence-9th-fix/ci-race-regression-evidence.txt` | CI 全仓 `-race` / 容器冒烟全 step |
| `docs/r0-evidence-9th-fix/rv13-rv14-rv15-remediation.md` | 本报告 |

---

## 7. 边界与 Known Issues（保留，未改判）

- A05（`a-scenarios.js:188` 假阳性风险）、C 228 条中的硬编码 PASS、t021 override Owner 决策、`OBS-ACCT-VIS-390`（390px 账户名 CSS 隐藏，仍 FAIL）、RV-05（仓库 Public / 旧 SHA 历史）**均保持原状态，未改判为通过**。
- 不改 PRD / WBS / 产品规则 / DB 结构 / API 契约 / 业务功能 / 账户页 CSS；不改仓库 Public/Private；不清理或重写 Git 历史。
- 未合并 PR #3；未提交 main；未启动 R1–R7 或 PWA。
- RV-14 的桥接窗口为「连续 ≤ 4 字节噪声」+「凭据锚点闸门」：更极端的长噪声错位形态不在本轮复现范围，按 fail-closed（无法可靠解码即 BLOCK）兜底；正常二进制误报控制在 case-46/47 与真实 Artifact 102 文件复扫中验证为 0 命中。
- 三个 RV-15 负向 run 的 `conclusion=failure` 属预期（刻意令前置 step 失败），不是整改失败。

**完成代码修改和 CI 运行不等于第三方验收通过。本轮完成后停止，不自行合并，等待 ChatGPT 独立复核。**
