# FastLedger R0 · P1 Round 2 QA 独立回归报告（最后一轮）

> 本报告为 QA **第 2 轮 / 最后一轮**独立回归，按 2 轮硬上限**不进入第 3 轮**。
> 独立执行所有命令、记录真实退出码与文件内容；**不采信**
> `docs/r0-evidence-7th-fix/p1/round2/round2-report.md`、`round2-verify.js` 或工程师任何日志/自述。
> 验证对象为 **工作树当前版本**（Round 2 修复未提交，HEAD 仍为 `0c62f501…`）。

---

## 0. 验证声明与基线

| 项 | 内容 |
|---|---|
| 验证方 | QA 工程师（software-qa-engineer-2），Round 2 独立回归 |
| 分支 / HEAD | `fix/r0-sixth-runtime-defects` / `0c62f5017c8b616f6d0c5079c55a9b4a13d1f68f`（未变） |
| 验证对象 | 工作树未提交的 Round 2 修复；`scan-artifacts.js` / `test-scan-artifacts.js` / `r0-e2e.yml` / `p1-report.md` / `workflow-structure-check.js` |
| 边界遵守 | 未修改任何被测代码 / workflow；未 commit / push / 合并；仅新增本目录 `qa-verify-round2/`；未启动 R1–R7 / PWA；未改业务代码 / `style.css` / `controls.json` / 228 清单 |
| 凭据处理 | 全部疑似凭据由 QA 自写脚本**运行时拼接 + base64url 生成**，带 `SYNTHETIC` 标记，写入系统临时目录不入库；报告与日志**不写任何 JWT 原文**，只写命中类型 / 数量 / 相对路径 |

**结论定义**：`已修`=独立实测证明缺陷消除；`部分`=主症状消除但引入/遗留新边界；`未修`=仍可复现；`Known Issue`=按 2 轮上限不再派发修复、写入交付说明的未解决项。

### 证据目录文件清单

| 文件 | 内容 |
|---|---|
| `negative-tests-r2.log` | 第 1 项 `node --test` 完整 TAP 输出 |
| `e2e-01/02/08-{prepare,scan}.log` | 第 2 项三条端到端链路（含 QA 自设计 UTF-16） |
| `scan-probe-*.log` | 第 3 项 18 个独立探针的扫描器原始输出 |
| `falsepos-*-{prepare,scan}.log` | 第 4 项真实产物误伤核查 |
| `strict-scan-r2.log` | 第 6 项 `scan-credentials.js --strict` 输出 |
| `negative-tests-post-refresh.log` / `strict-scan-post-refresh.log` / `post-refresh-*.log` | §10 增量通知后的同轮一致性复核 |
| `qa-p1-round2-report.md` | 本报告 |

---

## 1. 逐项结论总表

| # | 验证项 | 结论 | 关键实测 |
|---:|---|---|---|
| 1 | 独立重跑负向测试 | **PASS** | 退出 0，**17/17 pass、0 fail**（实际为 17 组，非任务描述的 16 组） |
| 2 | 重构造两条 Round 1 端到端绕过 | **PASS**（N1/N2 已修） | ① NUL `.log` → prepare 收集 / `scan=2`；② 伪 `.png` → prepare 收集 / `scan=2` |
| 2+ | QA 追加 UTF-16 端到端样本 | **FAIL（新缺陷 K1）** | prepare 收集 / **`scan=0 CLEAN`**，载体确实含凭据 |
| 3 | 主动找新 fail-open 通道 | **FAIL** | 18 个探针中 **9 个返回 `0 CLEAN`**，其中 8 个是凭据载体（K1/K2/K3） |
| 4 | 误伤核查（真实产物） | **PASS** | `r0-qa-verify` 99 文件 / `r0-e2e` 113 文件全量扫描，`scan=0`，`SUSPICIOUS_BINARY` 命中 0 |
| 5 | workflow 静态审查 | **PASS** | 既有门禁/汇总段 **1927/1927 字节逐字一致**；新 summary step 位于上传 step 之后 |
| 6 | 全仓扫描与证据原文 | **PASS** | `strict=0`（真实候选 0）；`git ls-files` 无 trace.zip/artifacts；三个证据目录原文命中 **0** |
| 7 | 未越界判定 | **PASS** | P1 Round 2 仅 5 个 M 文件；P2 在途 2 个 M 文件已分离；禁止改动文件匹配 0 |

**一句话总评**：Round 1 的 **N1 / N2 / N3 / N4 / N5 主体均已真实修复**（端到端实证 + 零跳过 + 误伤核查通过），
但 QA 在 Round 2 主动构造的独立反例中**仍发现多条可端到端放行的凭据载体**（UTF-16 编码、短/低熵值、规则未覆盖键名与 base64 编码），
其中 **K1（UTF-16LE）为全新未披露缺陷**。按 2 轮硬上限**不进第 3 轮**，全部转 Known Issues。
**绿色 CI 仍不等于证据已安全上传。**

---

## 2. N1–N5 逐项判定

| ID | Round 1 缺陷 | 判定 | 独立证据（本报告章节） |
|---|---|---|---|
| **N1** | 白名单文本 NUL 前缀 fail-open（静默跳过内容扫描） | **已修** | §3 ①：`run.log` 头插 1 个 NUL + 合成 JWT，prepare 收集 → `scan=2`（`JWT_LITERAL 1`），输出 `内容已全量扫描=2 跳过内容扫描=0`；§4 ⑥a/⑥c 亦均 2；`looksBinary()` 已从源码删除（全文 Read 确认无 `binarySkipped` / `continue` 跳过分支） |
| **N2** | `.png` 扩展名无条件跳过内容扫描 | **已修** | §3 ②：纯文本合成凭据命名 `shot.png`，prepare 收集 → `scan=2`（`JWT_LITERAL 1` + `SUSPICIOUS_BINARY 1`）；§4 ③ 合法 PNG 魔数 + 尾部 JWT 仍 `2`（证明零跳过）；⑦a 0 字节 png `2`；⑦c 魔数表未覆盖的 `.webp` + JWT 亦 `2` |
| **N3** | CI summary 不含 prepare/scan/upload outcome | **已修（静态层面，未经真机 CI）** | §6：新增 L141–155 独立 step `汇总证据上传安全结论`，`if: always()`（L143），位于上传 step（L127–136）之后，写三个 outcome 且上传非 success 时显式提示；既有「汇总退出码」step 1927 字节逐字未改。按指令未 push，**未做真机 run 验证** |
| **N4** | 证据报告 HEAD 不一致 / 绕过路径披露弱化 | **已修** | §7：`p1-report.md` diff 追加「HEAD 时点口径勘误」（明确 P1 commit = `0c62f501…`）、首轮规则表版本说明、以及「文件内容即纯文本凭据」「NUL 前缀 .log」两条路径的如实披露，删除原「已知边界，非回归」表述 |
| **N5** | 合法业务字段被误阻断（假阳性） | **已修（附带 K2 漏报权衡）** | §5 独立重构 Round 1 假阳性样本（`"authorization": "role-based-access"`、`"cookie": "same-site-lax"`）→ `scan=0 CLEAN`；真实产物 99/113 文件亦无误报。但收紧口径引入 K2 漏报，见 §8 |

> **N1/N2 的阻断链结论**：两条场景下 `scan` 退出 2 → `steps.artifact_scan.outcome = 'failure'` →
> 上传 step 的 `if: ${{ always() && steps.artifact_scan.outcome == 'success' }}`（L129）求值为 false →
> **上传 step 必被跳过、不执行**，与 Round 1 的端到端放行形成明确反转。

---

## 3. 第 1 项 · 独立重跑负向测试（PASS）

```
CMD: node --test tools/r0/e2e/test-scan-artifacts.js
NODE_TEST_EXIT = 0
# tests 17 / # pass 17 / # fail 0 / # skipped 0 / # todo 0
```

> **如实标注**：team-lead 任务书写「16 组」，实际测试文件含 **case-01…case-17 共 17 组**（工程师额外加了 case-17
> 「真实 PNG 魔数 + 内嵌 JWT → 2」）。以实跑为准，不采信任何口头数字。

| 组 | 用例 | 期望 | **实际退出码** | 结果 |
|---:|---|---:|---:|---|
| 1 | 干净文件集 | 0 | **0** | PASS |
| 2 | 合成 JWT 三段式 | 2 | **2** | PASS |
| 3 | `trace.zip` 黑名单 | 2 | **2** | PASS |
| 4 | Bearer / 鉴权头 | 2 | **2** | PASS |
| 5 | Cookie / Set-Cookie | 2 | **2** | PASS |
| 6 | `server.log` | 2 | **2** | PASS |
| 7 | 密钥赋值 | 2 | **2** | PASS |
| 8 | 目录不存在 | 1 | **1** | PASS |
| 9 | 纯文本凭据伪装 `.png`（R2 修订） | 2 | **2** | PASS |
| 10 | 二进制文件名含敏感词 | 2 | **2** | PASS |
| 11 | NUL 前缀白名单 `.log`（N1 回归） | 2 | **2** | PASS |
| 12 | 合法业务字段（N5 回归） | 0 | **0** | PASS |
| 13 | 真实 PNG 魔数 | 0 | **0** | PASS |
| 14 | 鉴权头 Bearer + JWT | 2 | **2** | PASS |
| 15 | Cookie 长值 | 2 | **2** | PASS |
| 16 | 符号链接 | 2 | **2** | PASS |
| 17 | 真实 PNG 魔数 + 内嵌 JWT | 2 | **2** | PASS |

---

## 4. 第 2 项 · 重构造两条端到端绕过（PASS）+ 第 3 项新探针

### 4.1 样本生成方式（QA 自写，不复用工程师 round2 脚本）

辅助脚本 `$TMP/fastledger-qa-r2-verify/helper.js`（不入库）在运行时生成合成 JWT
（`b64url(header) + "." + b64url(payload) + "." + b64url(signature)`，header 以 `eyJ` 起始、三段均 ≥8 字符）
及各类边界值；源码中**无任何凭据字面量**。所有样本写入系统临时目录，不落入仓库。
`CMD: node helper.js build` → `BUILD_EXIT=0`。

### 4.2 两条 Round 1 端到端绕过（QA 自建 source，走完整 prepare → scan）

| # | source → dest | prepare | dest 实际文件 | 载体确认 | **scan** | 结论 |
|---|---|---|---|---|---|---|
| ① | `src-nul-log`（`run.log` = NUL + 合成 JWT）→ `e2e-01-dest` | `PREPARE_EXIT=0`，收集 2（含 `run.log`） | `report.md`, `run.log` | `CARRIES_SYNTHETIC_CREDENTIAL`（latin1 视图可见） | **`SCAN_EXIT=2`**，`[HIT] run.log \| JWT_LITERAL \| 1`，`结论：BLOCKED` | **N1 已修** |
| ② | `src-fake-png`（`shot.png` = 纯文本凭据，无魔数）→ `e2e-02-dest` | `PREPARE_EXIT=0`，收集 2（含 `shot.png`） | `report.md`, `shot.png` | `CARRIES_SYNTHETIC_CREDENTIAL` | **`SCAN_EXIT=2`**，`[HIT] shot.png \| JWT_LITERAL \| 1`、`[HIT] shot.png \| SUSPICIOUS_BINARY \| 1`，`结论：BLOCKED` | **N2 已修** |

两条均输出 `内容已全量扫描=2 跳过内容扫描=0`。日志：`e2e-01-*.log` / `e2e-02-*.log`。

### 4.3 新绕过尝试结果表（Round 2 重点）

「QA 安全期望」= 从「不得放行凭据载体」角度应返回 2；实际退出码为独立实测。

| # | 样本 | QA 安全期望 | **实际退出码** | 命中类型 / 数量 | 判定 |
|---:|---|---:|---:|---|---|
| ③ | 合法 PNG 魔数 + 尾部附加合成 JWT | 2 | **2** | `JWT_LITERAL 1`（无 SUSPICIOUS_BINARY，阻断来自文本规则） | **PASS**（零跳过名副其实） |
| ④a | `authorization` 值 = 12 字符纯小写 | 2 | **0** | 无 | **缺陷 K2** |
| ④b | `Cookie` 值 = 15 字符纯小写 | 2 | **0** | 无 | **缺陷 K2** |
| ④c | `Bearer` + 12 字符短值 | 2 | **2** | `BEARER_TOKEN 1`（独立规则兜底） | PASS |
| ④d | `authorization` 值 = 20 字符纯小写（≥16 但低熵） | 2 | **0** | 无 | **缺陷 K2（未披露）** |
| ⑤a | `AUTHORIZATION : <JWT>`（大写 + 冒号前空格） | 2 | **2** | `AUTH_HEADER 1` + `JWT_LITERAL 1` | PASS |
| ⑤b | `aUtHoRiZaTiOn\t=\t<JWT>`（混合大小写 + tab + 等号） | 2 | **2** | `AUTH_HEADER 1` + `JWT_LITERAL 1` | PASS |
| ⑤c | `SET-COOKIE : session=<高熵值>` | 2 | **2** | `COOKIE_HEADER 1` | PASS |
| ⑥a | `.md` 中嵌 NUL + 合成 JWT | 2 | **2** | `JWT_LITERAL 1` | PASS |
| ⑥b | `.json` 100 KB 超长行 + 合成 JWT | 2 | **2** | `JWT_LITERAL 1` | PASS |
| ⑥c | `.txt` NUL 前缀 + Bearer | 2 | **2** | `BEARER_TOKEN 1` | PASS |
| ⑦a | 0 字节 `.png` | 2 | **2** | `SUSPICIOUS_BINARY 1` | PASS |
| ⑦b | 0 字节 `.txt`（空文件，**无凭据**） | 0 | **0** | 无 | PASS（非凭据载体，不计缺陷） |
| ⑦c | `.webp`（魔数表未覆盖）+ 合成 JWT | 2 | **2** | `JWT_LITERAL 1`（无魔数校验，靠文本规则） | PASS |
| ⑦d | `.webp` + 15 字符短值凭据 | 2 | **0** | 无 | **缺陷 K2 / K4** |
| ⑧ | **UTF-16LE 编码** `Authorization: Bearer <JWT>`（端到端） | 2 | **0** | 无 | **新缺陷 K1** |
| ⑨ | `x-api-key` / `apiKey` + 29 字符高熵值 | 2 | **0** | 无 | **缺陷 K3** |
| ⑩ | base64 编码的 `Authorization: Bearer <JWT>` | 2 | **0** | 无 | **缺陷 K3** |
| ⑪ | 合法业务字段（N5 假阳性独立重构） | 0 | **0** | 无 | PASS（N5 已修） |

原始输出见 `scan-probe-*.log` / `scan-probe-11-legal-fields.log`。

### 4.4 ⑧ UTF-16 端到端样本的实证链（关键）

```
source = src-utf16/run.log（UTF-16LE 编码，内容含 Authorization: Bearer <合成JWT>）
PREPARE_EXIT=0  收集 2（report.md, run.log）   ← .log 在白名单，prepare 只按扩展名判定
probe-file: bytes=536
  contains_jwt_value_latin1=false             ← 扫描器所用的 latin1 视图看不到凭据
  contains_SYNTHETIC_literal_latin1=false
  contains_jwt_value_utf16=true               ← 以 UTF-16LE 解码确实含凭据
  结论：CARRIES_SYNTHETIC_CREDENTIAL
SCAN_EXIT=0
[scan] 文件总数=2 内容已全量扫描=2 其中二进制文件=0 跳过内容扫描=0 符号链接=0
[HIT] （无） | — | 0
[scan] 结论：CLEAN —— 允许上传
```

→ 该凭据载体经 `prepare` 收集、被 `scan` 判定 CLEAN，**端到端可被 `upload-artifact` 上传**。
机理：UTF-16LE 把每个 ASCII 字符编码为 `字符 + \0`，`latin1` 1:1 映射后字符串变为 `A\0u\0t\0h\0…`，
5 条文本规则均要求连续 ASCII 关键字 / JWT 形态，故全部失配。
这**直接反驳**了 Round 2 注释「latin1 保证任意字节 1:1 映射，使 ASCII 凭据串在文件任何字节位置都能被正则命中」的口径——
latin1 只保证字节不丢失，**不保证被 NUL 隔开的 ASCII 序列仍能成串匹配**。

### 4.5 ⑨ / ⑩ 载体确实含凭据的实证

- ⑨ `probe-09-apikey/config.json`：含 `x-api-key` 键，两个值均 29 字符且含数字与大写（`has_x_api_key=true`、`value_lengths=29,29`、`values_have_digit_and_upper=true`），但键名不在 5 条规则内 → `0 CLEAN`。
- ⑩ `probe-10-base64/payload.txt`：`encoded=<base64>`，解码后 `decoded_contains_Authorization=true`、`decoded_contains_Bearer=true`、`decoded_contains_eyJ_shape=true`，但编码态不含关键字 → `0 CLEAN`。

### 4.6 值泄露校验

对 22 个扫描日志逐一运行 `helper.js verify`（重新生成同样的合成值比对，只打印布尔）：
`NO_LEAK_PASS=22 NO_LEAK_FAIL=0`。扫描器输出中不含任何合成凭据值、`SYNTHETIC` 标记或 `eyJ` 前缀形态。

---

## 5. 第 4 项 · 误伤核查（PASS）

| 真实产物源 | prepare | scan | `SUSPICIOUS_BINARY` |
|---|---|---|---|
| `artifacts/r0-qa-verify` | `PREPARE_EXIT=0`，扫描 111 / 收集 99 / 排除 12 | `SCAN_EXIT=0`；`文件总数=99 内容已全量扫描=99 其中二进制文件=80 跳过内容扫描=0`；`命中总数=0`；`结论：CLEAN` | **0** |
| `artifacts/r0-e2e` | `PREPARE_EXIT=0`，扫描 149 / 收集 113 / 排除 36 | `SCAN_EXIT=0`；`文件总数=113 内容已全量扫描=113 其中二进制文件=71 跳过内容扫描=0`；`命中总数=0`；`结论：CLEAN` | **0** |

→ 约 80 / 71 张真实 PNG 全部通过魔数校验，合法 `report.json` 无误报；N5 修复未引入误伤。
日志：`falsepos-*-*.log`。

---

## 6. 第 5 项 · workflow 静态审查（PASS，逐条给行号）

**方法**：QA 直接 `Read` 全文 + `git diff HEAD -- .github/workflows/r0-e2e.yml` + 按内容锚点与 `git show HEAD:` 逐字比对，
**未使用**工程师的 `workflow-structure-check.js` 作为判据。

| 子项 | 要求 | 实测行号 / 证据 | 判定 |
|---|---|---|---|
| ① | 既有「汇总退出码」step 与 Round 1 逐字一致 | 按 `# A 是门禁` → `GITHUB_STEP_SUMMARY` 锚点比对 `HEAD` 与工作树：`GATE_DIFF_EXIT=0`，**1927 / 1927 字节、41 行完全一致**（覆盖 A / D / C / 汇总四段） | PASS |
| ② | 新 summary step 位于上传 step 之后，能读到三个 outcome | 上传 step **L127–136**（`id: artifact_upload` **L128**）；新 step **L141–155** `name: 汇总证据上传安全结论`（L141），`if: ${{ always() }}`（**L143**），写 `steps.artifact_prepare.outcome`(L148) / `steps.artifact_scan.outcome`(L149) / `steps.artifact_upload.outcome`(L150)，并在非 success 时提示(L152–154) | PASS |
| ③ | 上传 `if` 仍绑定 `artifact_scan.outcome == 'success'` | **L129**：`if: ${{ always() && steps.artifact_scan.outcome == 'success' }}`；`path: artifacts/r0-e2e-upload/**`（L134）、`if-no-files-found: error`（L136）均保持 | PASS |
| ④ | A / D 门禁语义、C 的 `continue-on-error: true` 未变 | **L70–76** 包 A、**L80–86** 包 D（均无 `continue-on-error`）、**L90–97** 包 C，**L92 `continue-on-error: true`**；整段已由 ① 的逐字比对覆盖 | PASS |
| ⑤ | Round 2 对 workflow 的改动范围 | `git diff HEAD` 仅两处：`+ id: artifact_upload`（1 行）与 `+ 新 summary step`（19 行），共 `+20 -0` | PASS |

> `ci.yml` 相对 HEAD 改动 0 行；全仓 `upload-artifact` 仅 `r0-e2e.yml:130` 一处。

---

## 7. 第 6 / 7 项 · 全仓核查与未越界判定（PASS）

### 7.1 第 6 项

| 核查 | 实测 |
|---|---|
| `node tools/r0/scan-credentials.js --strict` | **STRICT_EXIT=0**；`真实凭证候选命中：0`（合成/占位 218、人工复核 15、测试夹具 27、可移植性 100；扫描文本 891） |
| `git ls-files` | tracked total 712；`trace.zip` 命中 **0**；`artifacts/` 命中 **0** |
| `upload-artifact` 唯一性 | `.github/workflows/r0-e2e.yml:130` 仅 1 处 |
| 证据目录 JWT/Bearer 原文（只统计命中数） | `round2/`：0（10 文件）；`qa-verify/`：0（27 文件）；`qa-verify-round2/`：0（29 文件） |

### 7.2 第 7 项 · 逐文件核对 P1 Round 2 改动范围

工作树相对 `HEAD` 共 **7 个 M 文件**，逐文件改动量与归属：

| 文件 | numstat | 归属 |
|---|---|---|
| `.github/workflows/r0-e2e.yml` | `+20 -0` | **P1 Round 2** |
| `docs/r0-evidence-7th-fix/p1/p1-report.md` | `+37 -11` | **P1 Round 2**（N4 勘误 / 披露） |
| `docs/r0-evidence-7th-fix/p1/workflow-structure-check.js` | `+16 -0` | **P1 Round 2**（N3 结构校验） |
| `tools/r0/e2e/scripts/scan-artifacts.js` | `+101 -37` | **P1 Round 2**（N1/N2/N5） |
| `tools/r0/e2e/test-scan-artifacts.js` | `+139 -24` | **P1 Round 2**（16→17 组） |
| `tools/r0/e2e/run.js` | `+29 -5` | **P2 在途**（`--suite all` 语义，diff 内明确标注「P2 整改」） |
| `tools/r0/e2e/specs/c-controls.js` | `+151 -20` | **P2 在途**（228 清单编号错位，如 I-ACCT-06） |

| 核查 | 实测 |
|---|---|
| 禁止改动文件 | `git diff --name-only HEAD \| grep -Ei 'server/web/dist/app\.js\|style\.css\|fixtures/controls\.json\|PRD\|WBS\|/db/\|/api/\|README\.md\|CHANGELOG'` → **无匹配（EXIT=1）** |
| `prepare-upload.js` | `git diff HEAD -- …/prepare-upload.js` = **0 行**（未改，符合声明） |
| `ci.yml` | **0 行**（未改） |
| 未跟踪文件归类 | `P1-artifact-credential-incident.md`（team-lead 新增）、`p1/qa-verify/`（QA R1）、`p1/qa-verify-round2/`（QA R2 本目录）、`p1/round2/`（P1 R2 证据）、`p2/`（P2）、`tools/r0/e2e/README.md`（P2） |

→ **P1 Round 2 实际改动仅限 5 类文件，无越界**；P2 在途与 team-lead 新增文件均按要求分离，不计入 P1 Round 2 越界。

---

## 8. 新发现 / 残留风险（Known Issues，全部未修复）

> 按 team-lead 指令：本轮为最后一轮，**以下全部转 Known Issues 写入交付说明，不得称已解决**。
> 团队要求「发现任何返回 `0 CLEAN` 的凭据载体必须明确标为新缺陷」，以下逐条如实登记。

| ID | 严重度 | 问题 | 是否端到端可放行 | 是否已由工程师披露 |
|---|---|---|---|---|
| **K1** | **高** | **UTF-16LE / UTF-16 编码绕过**：`latin1` 1:1 映射无法还原被 NUL 隔开的 ASCII 串，5 条文本规则全部失配。§4.4 实证 `prepare` 收集 → `scan=0 CLEAN`。Windows/PowerShell 默认UTF-16 日志属常见形态。 | **是** | **否（全新发现）**，且直接反驳「零跳过=全覆盖」口径 |
| **K2** | 中 | **短值 / 低熵值漏报**：`credentialLikeValue()` L92 `value.length < 16 → false`；L95–98 对 ≥16 但纯小写、无数字、无大写的低熵值也返回 false。实测 ④a(12)、④b(15)、④d(**20**)、⑦d 均 `0 CLEAN`。 | **是** | 部分：工程师披露「<16 短值权衡」，**未披露 ④d（≥16 低熵纯小写）漏报** |
| **K3** | 中 | **规则未覆盖的凭据形态**：⑨ `x-api-key`/`apiKey` 下 29 字符高熵值、⑩ base64 编码的 `Authorization: Bearer <JWT>`（解码后确含凭据）均 `0 CLEAN`。5 条 TEXT_RULES 为已知形态白名单，非通用 DLP。 | **是** | 部分：N6 已声明「规则覆盖有限」，本次为可放行载体的具体实证 |
| **K4** | 提示 | **魔数表仅覆盖 8 种格式**：`BINARY_EXT` 含 24 种扩展名，`BINARY_SIGNATURES` 仅 8 种（`.webp/.woff/.ttf/.mp4/.exe/...` 无校验）。⑦c 靠文本规则拦下，⑦d 短值即放行。 | 依赖文本规则 | 是（round2 残留说明已列） |
| **K5** | 提示 | **N3 未经真机 CI 验证**：按指令未 push、无新 run；新 summary step 仅静态确认，其三个 outcome 的实际渲染值未验证。 | — | 是 |
| **K6** | 提示 | **绿色 CI ≠ 证据已安全上传**：Round 1 §6.4 结论继续有效；`scan` 通过只代表 5 条规则命中 0，且上传 step 被跳过时不产生失败信号。 | — | 是 |
| **K7** | 提示 | 历史 artifact 未追溯；真实图片画面内文字（OCR 范畴）不在能力范围；按指令未 push。 | — | 是 |

**本轮明确修复、不再列为问题的项**：N1（NUL 跳过）、N2（`.png` 跳过）、N3（summary 可观测性，静态）、
N4（HEAD 勘误与披露）、N5（合法业务字段假阳性）；误伤核查 0 误报；workflow 门禁段逐字未改。

### QA 自修记录（如实）

- 一次命令错误：把 `/tmp/...` 作为 **Node 代码内字符串字面量**传给 Windows Node，被解析为 `C:\tmp\...` 导致 `ENOENT`
  （与 Round 1 E5 同类）。改用 Windows 原生路径后重跑 R2-10 解码验证成功（`DECODE_CHECK_EXIT=0`）。
  此为 **QA 测试命令 Bug，非源码问题**，已自修，不影响任何结论。
- 值泄露校验 22 个日志全部 PASS；未修改任何被测代码以「制造通过」。

---

## 9. 最终智能路由判定

> **本轮为 Round 2 / 最后一轮，按 2 轮硬上限不进入第 3 轮。**

**执行动作判定：`NoOne`（收尾，不再派发修复）—— 但这不是「全部通过」。**

- **按缺陷归属**：本轮新发现的 K1 / K2 / K3 属**源码 / 规则缺陷**，性质上应归 `Engineer`；
  但 team-lead 已明确 2 轮上限、不进第 3 轮、未修复项一律转 Known Issues，故**不派发修复、不重开轮次**。
- **按流程动作**：收尾 = `NoOne`。K1–K7 **全部转 Known Issues 写入交付说明，不得称已解决**；
  K1 / K2 / K3 必须随交付明确告知第三方「仍存在可端到端放行的凭据载体」。
- **N1–N5 判定汇总**：N1 已修、N2 已修、N3 已修（静态，K5 待真机）、N4 已修、N5 已修（假阳性消除，附带 K2 权衡）。
- **回归对照**：Round 1 的两条端到端放行路径（NUL `.log`、伪 `.png`）已由 `scan=0 CLEAN` 反转为 `scan=2 BLOCKED`，
  且上传 `if` 依赖该 outcome，故必被跳过 —— **本次事件的核心根因（trace.zip 明文外发）与本轮修复均真实有效**。

### 给第三方（ChatGPT）的必带提示

> P1 Round 2 已消除 Round 1 的两条失败开放通道并补齐 CI 可观测性，QA 独立实测通过；
> **但扫描器仍是基于已知形态规则的轻量闸门**，QA 在最后一轮主动构造的反例中，
> **UTF-16LE 编码日志、短/低熵头值、`x-api-key` 键名、base64 编码凭据仍可端到端放行**（`scan=0 CLEAN`）。
> 这些是本轮 **Known Issues**，按 2 轮上限未再修复，**不得描述为「已解决」或「绝对无凭据」**。
> **绿色 CI 或「扫描通过」均不构成「证据已安全上传」的证明**；须逐 step 核对 outcome、逐 artifact 下载核验内容。

---

## 10. 增量通知后的同轮一致性复核（非第 3 轮）

本报告提交后收到 `software-engineer-2` 增量通知：`p1/round2/` 证据全套重采（目录 mtime 21:28–21:30）、
用例补齐 case-17、`round2-verify.js` 新增场景 E/F。按「不采信自述」原则，QA 做**同轮证据一致性复核**：
**不新增第 3 轮、不派发任何修复、未修改任何被测代码**，只确认已交付结论是否仍成立。

| 复核项 | 对象 / 命令 | 实测 |
|---|---|---|
| 被测源码是否变化 | `ls -l` / `sha256sum` | `scan-artifacts.js` mtime 21:15:28、`test-scan-artifacts.js` 21:24:53、`r0-e2e.yml` 21:15:28；本轮回归当时已是 17 组版本，无需重做 |
| 负向套件 | `node --test tools/r0/e2e/test-scan-artifacts.js` | `NODE_TEST_EXIT=0`；`tests 17 / pass 17 / fail 0 / skipped 0` |
| strict | `node tools/r0/scan-credentials.js --strict` | `STRICT_EXIT=0`，`真实凭证候选命中：0` |
| N1 端到端 | `e2e-01-dest` | `SCAN_EXIT=2`，`BLOCKED` |
| N2 端到端 | `e2e-02-dest` | `SCAN_EXIT=2`，`BLOCKED` |
| **K1 UTF-16 端到端** | `e2e-08-dest` | **`SCAN_EXIT=0`，`CLEAN`（仍存在）** |
| 工程师重点①：魔数合法 PNG + 内嵌凭据 | `probe-03-realpng-tail` | `SCAN_EXIT=2`，`BLOCKED`（命中来自 `JWT_LITERAL`，非 `SUSPICIOUS_BINARY`） |
| 工程师重点②：干净真实 PNG | `falsepos-r0-e2e`（113 文件 / 71 PNG） | `SCAN_EXIT=0`，`CLEAN`，`SUSPICIOUS_BINARY` 命中 0 |
| 工程师重点③：strict | 同上 | `STRICT_EXIT=0` |
| N5 合法业务字段 | `probe-11-legal-fields` | `SCAN_EXIT=0`，`CLEAN` |

当前被测文件 hash（供第三方比对，非凭据）：
`scan-artifacts.js` = `58596fd790af32273005517842e6d2cbf60db4f67bb0f23c9b62432d1ed7f205`；
`test-scan-artifacts.js` = `4e756840e6cddbe20310a73dc66c256d80747c2882d29f6aa5eac71e44761741`；
`r0-e2e.yml` = `9eadcf51f5302f18c39d30e17f1de37cf6657ab659f1c60df6627c048f10eca2`。

**复核结论**：证据刷新**不影响本报告任何判定** —— N1–N5 仍为「已修」；
K1 / K2 / K3 仍为**可端到端放行的凭据载体**（`0 CLEAN`）；
最终路由仍为 `NoOne`（收尾，不进第 3 轮），K1–K7 仍全部为 Known Issues，不得称已解决。

---

## 附录 A · 复现命令清单（全部由 QA 独立执行）

```bash
cd /c/Users/47006/WorkBuddy/fastledger/repo_tmp

# 1 负向测试
node --test tools/r0/e2e/test-scan-artifacts.js

# 2/3 自建探针（helper.js 位于系统临时目录，不入库）
node /tmp/fastledger-qa-r2-verify/helper.js build
node tools/r0/e2e/scripts/prepare-upload.js --source /tmp/fastledger-qa-r2-verify/src-nul-log  --dest /tmp/fastledger-qa-r2-verify/e2e-01-dest
node tools/r0/e2e/scripts/scan-artifacts.js /tmp/fastledger-qa-r2-verify/e2e-01-dest
node tools/r0/e2e/scripts/prepare-upload.js --source /tmp/fastledger-qa-r2-verify/src-fake-png --dest /tmp/fastledger-qa-r2-verify/e2e-02-dest
node tools/r0/e2e/scripts/scan-artifacts.js /tmp/fastledger-qa-r2-verify/e2e-02-dest
node tools/r0/e2e/scripts/prepare-upload.js --source /tmp/fastledger-qa-r2-verify/src-utf16   --dest /tmp/fastledger-qa-r2-verify/e2e-08-dest
node tools/r0/e2e/scripts/scan-artifacts.js /tmp/fastledger-qa-r2-verify/e2e-08-dest
for d in /tmp/fastledger-qa-r2-verify/probe-*; do
  node tools/r0/e2e/scripts/scan-artifacts.js "$d"; echo "EXIT=$?"
done
node /tmp/fastledger-qa-r2-verify/helper.js verify <scan-log>
node /tmp/fastledger-qa-r2-verify/helper.js probe-file <carrier>

# 4 误伤核查
node tools/r0/e2e/scripts/prepare-upload.js --source artifacts/r0-qa-verify --dest <tmp>
node tools/r0/e2e/scripts/scan-artifacts.js <tmp>
node tools/r0/e2e/scripts/prepare-upload.js --source artifacts/r0-e2e      --dest <tmp>
node tools/r0/e2e/scripts/scan-artifacts.js <tmp>

# 5 workflow 静态审查
git diff HEAD -- .github/workflows/r0-e2e.yml
git show HEAD:.github/workflows/r0-e2e.yml | sed -n '/# A 是门禁/,/GITHUB_STEP_SUMMARY/p'
sed -n '/# A 是门禁/,/GITHUB_STEP_SUMMARY/p' .github/workflows/r0-e2e.yml

# 6 全仓
node tools/r0/scan-credentials.js --strict
git ls-files | grep -Ei 'trace\.zip|(^|/)artifacts/'
grep -rn 'upload-artifact' .github/workflows/

# 7 未越界
git diff --name-only HEAD
git diff --numstat HEAD -- <file>
git diff HEAD -- tools/r0/e2e/scripts/prepare-upload.js
```

> Windows 注意：把 `/tmp/...` 作为 **Node/Python 代码内字符串字面量**时 MSYS 不会转换，
> 需改用 `C:/Users/<user>/AppData/Local/Temp/...`；作为命令行参数时 MSYS 会自动转换。

## 附录 B · 合成值生成逻辑（要点，不含任何值）

```js
const b64url = (s) => Buffer.from(s, 'utf8').toString('base64url');
const jwt = [
  b64url(JSON.stringify({ alg: 'HS256', typ: 'JWT', qa: 'SYNTHETIC' })),
  b64url(JSON.stringify({ sub: 'SYNTHETIC_QA_R2_PROBE', scope: 'NOT_A_REAL_CREDENTIAL', iat: 0 })),
  b64url('SYNTHETIC_SIGNATURE_FOR_QA_R2_0000'),
].join('.');
const shortVal = (n) => 'syntheticqaprobe'.repeat(20).slice(0, n); // 纯小写、无数字/符号
// UTF-16 载体：Buffer.from(content, 'utf16le')，content 内含 'Authorization: Bearer ' + jwt
```

- 全部值运行时生成，源码中无 `eyJ…` 或真实凭据字面量；样本写入系统临时目录、未写入仓库；
- `verify` 重新生成同样值比对，仅打印布尔（`contains_*=true/false`），不打印值；
- 报告与全部证据日志不含任何 JWT 原文（第 6 项统计命中数 0）。
