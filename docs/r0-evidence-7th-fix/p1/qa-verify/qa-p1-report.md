# FastLedger R0 · P1 安全整改 QA 独立验证报告（严过关）

> 本报告面向第三方（ChatGPT）复核，独立记录 QA 自己执行的命令、退出码与文件内容。
> **不采信**工程师的 `p1/p1-report.md`、`p1/negative-tests.log`、`p1/workflow-structure-check.js` 的任何结论；
> 如需对照，仅作为「被验证对象」引用，不作为证据来源。

---

## 0. 验证声明与基线

| 项 | 内容 |
|---|---|
| 验证方 | QA 工程师（software-qa-engineer-2），独立验证 |
| 验证目标 | **证明防护真的生效**，而非确认文件存在 |
| 分支 | `fix/r0-sixth-runtime-defects`（未切换） |
| 实测 HEAD | `0c62f5017c8b616f6d0c5079c55a9b4a13d1f68f` |
| 实测基线 | 验证开始时 `git status --porcelain` 为空（干净）；因 **P2 并行在途**，越界判定改以 **P1 commit 真实范围 `git diff --stat 8082ca8..0c62f50`** 为准（见 §8），不再使用实时工作树快照 |
| 边界遵守 | 未修改任何被测代码 / workflow；未 commit、未 push、未合并 main；未启动 R1–R7 / PWA；未改 228 条清单与业务代码 |
| 唯一新增 | 本目录 `docs/r0-evidence-7th-fix/p1/qa-verify/` 下的报告与日志 |

**凭据处理约定**：本报告及本目录所有日志**不写任何 JWT 原文或凭据值**，只写「命中数量 / 文件相对路径 / 命中类型」。
所有「疑似凭据」绕过样本均由 QA 自行编写的辅助脚本在**运行时字符串拼接 + base64url 编码**生成，
统一带 `SYNTHETIC` 标记，写入系统临时目录 `$TMP/fastledger-qa-p1-verify/`（Git Bash 下写作 `/tmp/fastledger-qa-p1-verify/`），
**不落入仓库**，因此不影响全仓凭据扫描。辅助脚本源码要点见附录 B，源码内不含任何凭据字面量。

**结论定义**：`PASS`=实测符合预期且防护有效；`FAIL`=实测发现防护缺口或未达宣称目标；`BLOCKED`=环境/权限无法执行。

### 本目录证据文件清单

| 文件 | 内容 |
|---|---|
| `negative-tests-rerun.log` | 第 1 项 `node --test` 完整 TAP 输出 |
| `chain-prepare.log` / `chain-scan.log` | 第 2 项真实产物链路复刻 |
| `scan-bypass-0*.log` | 第 3 项 6 组绕过样本的扫描器原始输出 |
| `e2e-04/05/06-*.log` | 第 3 项端到端（prepare → scan）验证 |
| `edge-empty-*.log` | 空目录边界复刻 |
| `scan-false-positive.log` | 假阳性探测 |
| `strict-scan-rerun.log` | 第 6 项 `scan-credentials.js --strict` 完整输出 |
| `artifact-audit.log` / `artifact-audit-final.log` | 第 8 项 QA 独立审计脚本的**初版（含自修前假阳性）**与**修正版**输出 |
| `artifact-scan-crosscheck.log` | 被测 `scan-artifacts.js` 对同一解压目录的交叉验证 |
| `artifact-magic-check.log` | PNG 魔数 / 文本 NUL 实证 |
| `artifact-run-meta.log` / `artifact-run-list.log` / `artifact-run-steps.log` | run 元数据、artifact 列表、真实 job steps outcome |
| `qa-p1-report.md` | 本报告 |

---

## 1. 逐项结论总表

| # | 验证项 | 结论 | 关键实测退出码 |
|---:|---|---|---|
| 1 | 独立重跑负向测试 | **PASS** | `node --test` = 0，10/10 pass，0 fail |
| 2 | 真实产物链路复刻 | **PASS** | prepare=0，scan=0，禁止类型命中 0 |
| 3 | 自行注入绕过样本 | **FAIL**（存在可端到端上传的绕过通道） | ①②③④=2（阻断）；⑤=0、⑥=0（不拦截） |
| 4 | workflow 静态审查（5 个子项） | **PASS** | 逐字比对 41 行 / 1927 字节一致 |
| 5 | 上传阻断逻辑推演 | **PASS**（分析完成，附强制提示） | 见 §6 状态矩阵与「禁止绿色 CI 表述」 |
| 6 | 全仓上传面核查 | **PASS** | `upload-artifact` 仅 1 处；strict=0 |
| 7 | 确认未越界（改用 P1 commit 范围判定） | **PASS** | `git diff --stat 8082ca8..0c62f50` = 20 文件全属 P1；禁止文件匹配 0 |
| 8 | 补充：真机 artifact `13415542360` 独立复核 | **PASS**（与 team-lead 结论一致，另有增量证据） | 独立下载 sha256 与 API digest 一致；99 文件 / 0 zip / 19 文本全扫 / 命中 0 / CLEAN |

**一句话总评**：P1 对「原始 `trace.zip` 含 `Authorization: Bearer <JWT>` 被裸上传」这一根因的整改是**真实有效**的
（白名单排除 + 失败关闭扫描 + `if` 依赖链三处均已实测生效；且 P1 后新 CI 的真实 artifact 经 QA 独立下载复核确认为 CLEAN）；
但扫描器存在**未披露的内容扫描绕过**（⑥ NUL 前缀白名单文本、⑤ 纯文本伪装 `.png`）与**误阻断假阳性**，
且「绿色 CI」不可作为证据已安全上传的证明。详见 §10、§11。

---

## 2. 第 1 项 · 独立重跑负向测试（PASS）

```
CMD: node --test tools/r0/e2e/test-scan-artifacts.js
REAL_EXIT (PIPESTATUS[0]) = 0
```

实测汇总（取自 `negative-tests-rerun.log`，非引用工程师日志）：

```
# tests 10
# pass 10
# fail 0
# cancelled 0  # skipped 0  # todo 0
```

| 组 | 用例 | 期望 | **实际退出码** | 结果 |
|---:|---|---:|---:|---|
| 1 | 干净文件集（md/json/log/txt/png） | 0 | **0** | PASS |
| 2 | 合成 JWT 三段式 | 2 | **2** | PASS |
| 3 | 名为 `trace.zip` 的归档 | 2 | **2** | PASS |
| 4 | Bearer 值与鉴权头赋值 | 2 | **2** | PASS |
| 5 | Cookie / Set-Cookie 赋值 | 2 | **2** | PASS |
| 6 | `server.log` 服务原始日志 | 2 | **2** | PASS |
| 7 | JWT 签名密钥赋值 | 2 | **2** | PASS |
| 8 | 目录不存在（参数/环境错误） | 1 | **1** | PASS |
| 9 | 二进制内容按设计跳过扫描 | 0 | **0** | PASS（**注意**：把「不拦截」固化为通过用例，见 §4 ⑤） |
| 10 | 二进制文件名含敏感词仍被阻断 | 2 | **2** | PASS |

> 独立复跑结果与工程师日志一致，但**该套件只覆盖工程师自己设计的 10 种形态**，不构成对绕过面的穷尽证明。
> 故 QA 另行注入样本（§4）。

---

## 3. 第 2 项 · 真实产物链路复刻（PASS）

选定源目录（本机现成、含 9 个 `trace.zip` + 3 个 `*-trace.zip`）：

```
SRC  = artifacts/r0-qa-verify/2026-11-05_09-59-36-dadc9a3-a
DEST = /tmp/fastledger-qa-p1-verify/chain-upload
CMD : node tools/r0/e2e/scripts/prepare-upload.js --source <SRC> --dest <DEST>
PREPARE_EXIT = 0
```

prepare 实测汇总（`chain-prepare.log`）：`扫描=31 收集=19 排除=12 符号链接跳过=0`；
排除的 12 项全部为 `A01..A10/trace.zip` 与 `A09/{admin,member,owner}-trace.zip`，原因均为「Playwright trace 归档」。

**目标目录全文件清单（19 个，无任何禁止类型）**：

```
A01/after-login.png        A01/pending.png            A02/network-abort.png
A03/http-503.png           A04/retry-success.png      A05/slow-network.png
A06/home-with-data.png     A07/empty-book.png         A08/member-members-page.png
A09/members-admin.png      A09/members-member.png     A09/members-owner.png
A10/back-online.png        A10/offline.png            A11/push-results.json
SHA256SUMS.txt             fixtures.json              report.json
report.md
```

| 核验 | 命令 | 实测 |
|---|---|---|
| 禁止类型 | `find DEST -type f \( -iname '*.zip' -o -iname '*.trace' -o -iname '*.network' -o -iname '*.har' -o -iname 'server.log' -o -iname '*.db' -o -iname '*.sqlite*' \)` | `FORBIDDEN_MATCH_COUNT=0` |
| 类型分布 | 扩展名计数 | `png=14, json=3, txt=1, md=1`（合计 19） |
| 扫描器 | `node tools/r0/e2e/scripts/scan-artifacts.js DEST` | **SCAN_EXIT=0**，`命中总数=0`，`结论：CLEAN` |

扫描器明细（`chain-scan.log`）：`文件总数=19 文本已扫描=5 二进制跳过内容扫描=14 符号链接=0`。
> 注意：14 个 `.png` 被「二进制跳过内容扫描」，这是 §4 ⑤ 绕过之所以成立的直接原因。

---

## 4. 第 3 项 · 自行注入绕过样本（**FAIL**）

### 4.1 生成方式（QA 自写，不复用工程师用例）

辅助脚本 `$TMP/fastledger-qa-p1-verify/qa-probe-helper.js`（不入库）在运行时生成：
- 合成 JWT 三段式 = `b64url(headerJSON) + "." + b64url(payloadJSON) + "." + b64url(signature)`，header 以 `eyJ` 起始，三段均为 base64url 字符集且 ≥8 字符；
- 合成 Bearer = `'Bearer ' + b64url(...)`；
- 所有样本内容带 `SYNTHETIC` 明文标记，源码中**无**任何 `eyJ…` / 真实凭据字面量。
- 每个用例目录独立，含干净基线文件 `report.md` / `report.json`，确保扫描器进入文本扫描分支。

`CMD: node /tmp/fastledger-qa-p1-verify/qa-probe-helper.js build` → `BUILD_EXIT=0`。
Windows 本机符号链接创建结果：`{"linkMode":"file-symlink-ok","linkIsSymlink":true,"sourceLinkMode":"file-symlink-ok"}`
→ **④ 未被跳过，已实测**。

### 4.2 绕过尝试结果表

| # | 样本（放于独立上传目录） | 期望 | **实际退出码** | 命中类型 / 数量 | 结论 |
|---:|---|---:|---:|---|---|
| ① | 运行时拼接的合成 JWT 三段式写入白名单 `.txt`（`runtime-probe.txt`） | 2 | **2** | `runtime-probe.txt \| JWT_LITERAL \| 1` | **PASS**（阻断） |
| ② | 名为 `trace.zip` 的文件（内容为无意义字节） | 2 | **2** | `trace.zip \| TRACE_ARCHIVE \| 1` | **PASS**（阻断） |
| ③ | 合成凭据藏进白名单 `.json`（`report.json`） | 2 | **2** | `report.json \| BEARER_TOKEN \| 1`；`report.json \| JWT_LITERAL \| 1` | **PASS**（阻断） |
| ④ | 符号链接 `report-link.md` → 外部敏感文件 | 2 | **2** | `report-link.md \| SYMLINK_ENTRY \| 1` | **PASS**（阻断） |
| ⑤ | 合成凭据写进 `.png` **文件内容**（纯文本伪装） | 记录实际 | **0** | 无命中，`结论：CLEAN` | **FAIL（确认不拦截）** |
| ⑥ | 合成凭据写入白名单 `.log`，**文件头插入 1 个 NUL 字节**（QA 自行追加） | 记录实际 | **0** | 无命中，`结论：CLEAN` | **FAIL（新发现漏洞）** |

逐组实际命令形如 `node tools/r0/e2e/scripts/scan-artifacts.js /tmp/fastledger-qa-p1-verify/<case>`，
原始输出见 `scan-bypass-0*.log`。⑤ 与 ⑥ 的扫描器输出显示 `二进制跳过内容扫描=1`，即载体文件根本未被内容扫描。

### 4.3 ① 的值泄露校验（必须项）

`CMD: node /tmp/fastledger-qa-p1-verify/qa-probe-helper.js verify <logfile>`
（重新生成同样的合成值用于比对，只打印布尔，**不打印值**）

对全部 6 个扫描日志 + `chain-prepare.log` 逐一校验，`VERIFY_EXIT=0`，每项均为：

```
contains_jwt_value=false
contains_bearer_value=false
contains_SYNTHETIC_literal=false
contains_eyJ_prefix_shape=false
结论：PASS（输出不含任何合成凭据值或标记）
```

→ **① 附加条件通过**：扫描器退出 2 且输出中不出现该值本身；`prepare-upload.js` 日志同样不输出内容。

### 4.4 端到端复刻（关键：证明「不只是扫描器漏过，而是真的会被上传」）

| 端到端 | 命令链 | 实测 |
|---|---|---|
| ④ 符号链接 | `prepare --source <含 symlink 的 source> --dest <dest>` → `scan <dest>` | `PREPARE_EXIT=0`，`符号链接跳过=1`，dest 仅含 `clean.md`，`linked-secret.md` 数量 **0**；`SCAN_EXIT=0`。**prepare 不跟随符号链接，防护有效** |
| ⑤ `.png` 文本凭据 | 同上，`source` 含 `shot.png`（纯文本凭据） | `PREPARE_EXIT=0`，`收集=2`（`normal.md` + `shot.png`）；探针确认 `shot.png` **确实携带合成凭据**（`contains_jwt_value=true`、`contains_SYNTHETIC_literal=true`，仅布尔）；`SCAN_EXIT=0`，`结论：CLEAN` → **该文件会被 `upload-artifact` 上传** |
| ⑥ NUL 前缀 `.log` | 同上，`source` 含 `run.log`（NUL + 合成凭据） | `PREPARE_EXIT=0`，`收集=3`（含 `run.log`）；探针确认 `run.log` **确实携带合成凭据**（JWT/Bearer/SYNTHETIC 均 true）；`SCAN_EXIT=0`，`结论：CLEAN` → **该文件会被上传** |

原始输出见 `e2e-04/05/06-*.log`。

### 4.5 如实结论（不替已知边界圆场）

- **⑤ 确实不拦截**：`scan-artifacts.js` 的 `BINARY_EXT`（L47–51）包含 `.png`，L256–259 对二进制扩展名直接 `binarySkipped += 1; continue;`，**完全不读内容**；
  `prepare-upload.js` 的 `classify()`（L117–138）只按扩展名/文件名判定，也不读内容。两者叠加，纯文本凭据只要命名为 `.png` 就能端到端进入上传目录并被判定 CLEAN。
  工程师 `p1-report.md` L262–263 把此列为「已知边界」，但描述为「截图**画面**本身拍到令牌，文本扫描无能为力」——
  **弱化了「文件内容本身就是文本凭据」这一更直接、无需图像隐写的路径**；且 `test-scan-artifacts.js` case-09 把该行为固化为 PASS（退出 0）。
  QA 判定：**FAIL（残留风险，需 Owner 明确接受或修复）**。
- **⑥ 为本次 QA 新发现的未披露漏洞**：`looksBinary()`（L164–170）在**前 4096 字节内发现任一 NUL** 即把整个文件判为二进制并在 L265–268 `continue` 跳过内容扫描。
  `.log` / `.txt` / `.json` / `.md` 本属应扫描的白名单文本，攻击/污染方只需在文件头放 1 个 NUL 字节，即可让全部 5 条文本规则失效，扫描器返回 0 CLEAN。
  工程师报告与 10 组负向测试**均未覆盖**此路径。QA 判定：**FAIL（实质性缺陷，与「失败关闭」原则矛盾）**。

### 4.6 假阳性探测（任务特别要求标注）

构造**完全合法、无任何凭据**的报告文件：

```json
{ "suite": "a", "passed": 11, "failed": 0,
  "authorization": "role-based-access",
  "cookie": "same-site-lax",
  "note": "no credential material here" }
```

```
CMD: node tools/r0/e2e/scripts/scan-artifacts.js /tmp/fastledger-qa-p1-verify/false-positive
ACTUAL_EXIT = 2
[HIT] report.json | AUTH_HEADER | 1
[HIT] report.json | COOKIE_HEADER | 1
结论：BLOCKED
```

→ **确认假阳性（误阻断）**：`AUTH_HEADER` / `COOKIE_HEADER` 正则（L66–67）只要求「关键字 + 赋值 + ≥8 字符值」，
不校验值是否具备凭据形态，普通业务字段即可触发。后果是合法证据无法上传、CI 无谓变红。
（本次真实产物复刻未触发，说明现网报告恰好不含此类字段，但规则本身不稳健。）

---

## 5. 第 4 项 · workflow 静态审查（PASS，逐条给行号）

**方法**：QA 直接 `Read` + `sed -n` 阅读 `.github/workflows/r0-e2e.yml`，并用 `git show HEAD~1:...` 做逐字比对，
**未使用**工程师的 `workflow-structure-check.js`。

| 子项 | 要求 | YAML 行号与实测内容 | 判定 |
|---|---|---|---|
| ① | 上传 `if` 不再是裸 `always()`，明确依赖 `artifact_scan.outcome == 'success'` | **L128**：`if: ${{ always() && steps.artifact_scan.outcome == 'success' }}`；整改前为 `if: always()`（见 `git diff`） | PASS |
| ② | `artifact_prepare` / `artifact_scan` 的 `if` 链路 | **L113–116** prepare：`id: artifact_prepare`(L114)、`if: ${{ always() }}`(L115)、`run: …prepare-upload.js`(L116)；**L120–123** scan：`id: artifact_scan`(L121)、`if: ${{ always() && steps.artifact_prepare.outcome == 'success' }}`(L122)、`run: …scan-artifacts.js`(L123)。链路 = prepare(always) → scan(依赖 prepare success) → upload(依赖 scan success) | PASS |
| ③ | `path` 指向白名单目录、`if-no-files-found: error` | **L129** `uses: actions/upload-artifact@v4`；**L132–133** `path: \|` / `artifacts/r0-e2e-upload/**`；**L134** `retention-days: 30`；**L135** `if-no-files-found: error`（整改前为 `warn`） | PASS |
| ④ | A/D 门禁、C 报告型语义未变；C 仍 `continue-on-error: true`；D 无 | **L70–76** 包 A（`id: a_suite` L71，`run: node run.js --suite a` L76），无 `continue-on-error`；**L80–86** 包 D（`id: d_defects` L81，L86），无 `continue-on-error`；**L90–97** 包 C（`id: c_controls` L91，**L92 `continue-on-error: true`**，L97）。`git diff -U0` 的 4 个 hunk 为 `@@ -10 +10,2 @@`、`@@ -110,2 +111,18 @@`、`@@ -116 +133 @@`、`@@ -118 +135 @@`，**全部落在 L10 与 L110 之后**，门禁段未触及 | PASS |
| ⑤ | 既有汇总退出码 step 未被改动 | **L99–109**（`if: always()` L100，写 `$GITHUB_STEP_SUMMARY` 至 L109）。按内容锚点 `# A 是门禁` → `GITHUB_STEP_SUMMARY` 对 `HEAD~1` 与 `HEAD` 逐字比对：`DIFF_EXIT=0`，**41 行 / 1927 字节完全一致** | PASS |

补充：`grep -nic 'trace' .github/workflows/r0-e2e.yml` = **0**（workflow 注释中无 trace 字样）。

> **QA 自修记录**：首次按固定行号 `sed -n '69,109p'` 比对出现 1 行差异，原因是第一处注释改动使后续行号 +1，
> 属比对方法缺陷而非源码问题。改用内容锚点重新比对后 `DIFF_EXIT=0`，以上结论以修正后结果为准。

---

## 6. 第 5 项 · 上传阻断逻辑推演与状态矩阵（PASS，附强制提示）

### 6.1 条件链（据 §5 实测行号）

```
artifact_prepare : if: always()                                             → outcome ∈ {success, failure, skipped}
artifact_scan    : if: always() && prepare.outcome == 'success'              → prepare≠success 时 outcome='skipped'
upload           : if: always() && scan.outcome == 'success'                 → scan≠success 时 step 被 skipped
```

GitHub Actions 语义要点：被 `if` 判为 false 的 step 记 `outcome='skipped'`，**不产生失败信号**，
且其 `uses:` 不会执行；`if-no-files-found: error` 只在 upload step **实际运行**时才可能触发。

### 6.2 状态矩阵

由于 step 依赖，`scan` 与 `prepare` 不是自由组合：`prepare=success` 时 scan 的 `if` 恒真，scan 必然运行（success/failure），不会 skipped；
`scan=skipped` ⇔ `prepare≠success`（或 job 被取消）。

| # | prepare | scan | A / D / C 状态 | 上传 `if` | **上传 step** | job conclusion | 是否有 artifact |
|---:|---|---|---|---|---|---|---|
| 1 | success | success（CLEAN，非空目录） | 任意（A/D 失败则红；C FAIL 被 continue-on-error 豁免） | true | **执行** | 由其它 step 决定；A/D 全绿且 upload 成功 → 绿 | **有** |
| 2 | success | success（CLEAN，但**空目录**） | 任意 | true | **执行**，但 `if-no-files-found: error` 使其失败 | **红** | 无（有失败信号） |
| 3 | success | failure（扫描退出 2 命中 / 退出 1 环境错误） | 任意 | **false** | **skipped（不执行）** | **红**（scan step 失败） | 无 |
| 4 | failure（退出 1） | **skipped** | 任意 | **false** | **skipped（不执行）** | **红**（prepare step 失败） | 无 |
| 5 | **skipped**（job 取消 / 未来配置改动） | **skipped** | 任意 | **false** | **skipped（不执行）** | cancelled（非 success）；**若未来给 prepare 加 `continue-on-error` 或改 `if` 链路，则可能 success（绿）且零 artifact** | 无（**静默**） |

本地复刻佐证（#2）：空 source → `prepare` 退出 0（收集 0）→ `scan` 退出 0 且打印
`[scan][WARN] 目录为空；上传 step 的 if-no-files-found=error 仍会使 job 失败`（`edge-empty-scan.log`）。
即「零文件」目前**只能靠 upload step 实际运行时的 `if-no-files-found: error` 暴露**；一旦 upload 被跳过（#3/#4/#5），该兜底完全失效。

### 6.3 关于「scan skipped 时 job 仍可能显示 success」的如实推演

- **确认成立的部分**：`scan` 因前置条件不满足而 `skipped` 时，`steps.artifact_scan.outcome='skipped' ≠ 'success'`，
  上传 step **必不执行**，且被跳过的 step 不报红、不触发 `if-no-files-found: error`。
  → 客观存在一条「**静默零上传**」通道：没有任何 step 报错，却没有产出任何 artifact。
- **需要如实指出的差别（不盲目附和）**：在**当前** workflow 里，`artifact_prepare` 带 `always()` 且**无** `continue-on-error`，
  所以若它被运行且失败（#4），job 必为红；它被 `skipped` 通常只在 job 被取消时发生，此时 job 状态是 `cancelled` 而非 `success`。
  因此严格意义上「job 显示 **success** 且零 artifact」在当前配置下**并非正常可达**，其成立需要额外前提
  （prepare 为 skipped 而非 failed、或未来给 prepare/scan 加 `continue-on-error`、或调整 `if` 链路、或新增 step 使 prepare 被跳过）。
  **但无论哪种情形，「绿色 CI ≠ 证据已安全上传」的结论都成立**，理由如下。

### 6.4 给第三方（ChatGPT）的明确提示 —— 禁止把绿色 CI 描述成证据已安全上传

1. 上传 step 的 `if` 是「**允许不上传**」的：`scan` 被跳过时它只是 `skipped`，**不产生任何失败信号**。
   CI 绿色至多说明「已运行的门禁 step 没有失败」，**不能说明上传发生过，更不能说明上传内容安全**。
2. **汇总退出码 step（L99–109）位于三个新 step 之前**，其 `$GITHUB_STEP_SUMMARY` 只记录 A / D / C 的 outcome，
   **完全不含 `artifact_prepare` / `artifact_scan` / 上传 step 的状态**（工程师 `p1-report.md` L167–168、L270–271 亦承认按「不得改动既有内容」保持原样）。
   → 第三方从 CI summary **无法判断**扫描是否通过、上传是否发生。这是复核盲区。
3. 「扫描通过」只代表 5 条文本规则 + 文件名黑名单命中数为 0，**不代表内容绝对无凭据**（见 §4 ⑤⑥ 已实证放行）。
4. **复核动作要求**：必须到 run 页面逐个确认 `artifact_prepare` / `artifact_scan` / `上传证据` 三个 step 的 outcome；
   到 Actions artifacts 列表确认 artifact 实际存在、名称与 commit SHA 匹配；下载后独立核验内容。
   **不得**仅凭「CI 绿色」「workflow 文件已改」「本地自证日志为 0」推断「证据已安全上传」。

---

## 7. 第 6 项 · 全仓上传面核查（PASS）

| 核查 | 命令 | 实测输出 / 退出码 |
|---|---|---|
| `upload-artifact` 唯一性 | `grep -rn "upload-artifact" .github/workflows/` | 仅 `.github/workflows/r0-e2e.yml:129` 一处；`GREP_EXIT=0` |
| `ci.yml` 无上传步骤 | `grep -nE "upload\|artifact\|actions/upload" .github/workflows/ci.yml` | 无匹配，`CI_GREP_EXIT=1`；并通读 `ci.yml` 全文（125 行）确认无 upload step |
| 跟踪文件无 `trace.zip` / `artifacts/` | `git ls-files \| grep -Ei 'trace\.zip\|(^\|/)artifacts/'` | 无匹配；`tracked total=712`，`trace.zip=0`，`artifacts/=0`；`.gitignore` L16 有 `artifacts/` |
| 全仓凭据扫描 | `node tools/r0/scan-credentials.js --strict` | **STRICT_EXIT=0**；`真实凭证候选命中：0`（合成/占位 82、人工复核 15、测试夹具 27、可移植性 85） |

strict 完整输出见 `strict-scan-rerun.log`（121 行，仅文件路径/命中类型/数量/脱敏上下文，无值）。

---

## 8. 第 7 项 · 确认未越界（PASS）

**判定依据（按 team-lead 补充指令修订）**：P2 已并行启动，另一名工程师会修改
`tools/r0/e2e/specs/c-controls.js`、`tools/r0/e2e/run.js`、`tools/r0/e2e/README.md` 并在
`docs/r0-evidence-7th-fix/p2/` 新增证据——这些**不是 P1 越界**。因此**不再使用实时
`git status --porcelain` 工作树快照**（它会包含 P2 在途改动），改以 **P1 commit 真实范围
`git diff --stat 8082ca8..0c62f50`** 判定。

| 核查 | 命令 | 实测 |
|---|---|---|
| 提交链 | `git log --oneline -4` | `0c62f50`（P1）← `8082ca8`；`HEAD~1 = 8082ca85d4f679ed7698fc51009bc6a214c9ac0c`，故 `8082ca8..0c62f50` 与 `HEAD~1..HEAD` **等价**（`EQUIV_EXIT=0`） |
| P1 改动范围 | `git diff --stat 8082ca8..0c62f50` | `20 files changed, 2824 insertions(+), 5 deletions(-)`；全部为 `.github/workflows/r0-e2e.yml` + `docs/r0-evidence-7th-fix/p1/*` + 3 个 P1 脚本/测试 |
| 禁止改动文件 | `git diff --name-only 8082ca8..0c62f50 \| grep -Ei 'server/web/dist/app.js\|style.css\|controls.json\|c-controls.js\|tools/r0/e2e/run.js\|PRD\|WBS\|DB\|API'` | **无匹配（EXIT=1）** |
| 工作树佐证（验证开始时） | `git status --porcelain` | 空；验证过程仅新增本证据目录 `?? docs/r0-evidence-7th-fix/p1/qa-verify/`（允许范围内） |
| 被跟踪文件改动（验证时点） | `git diff --name-only` / `git diff --cached --name-only` | `0` / `0` |

> 注：`tools/r0/e2e/test-scan-artifacts.js` 属 P1 新增测试文件，与禁止清单中的 `tools/r0/e2e/run.js` 不是同一文件，未混淆；
> P2 后续对 `run.js` / `c-controls.js` / `README.md` 的改动属于另一任务，不计入 P1 越界判定。

> **终检时点观察（如实记录，防止第三方误读）**：本次验证收尾时 `git status --porcelain`
> 已显示多行在途改动（含 `M .github/workflows/r0-e2e.yml`、
> `M tools/r0/e2e/scripts/scan-artifacts.js`、`M tools/r0/e2e/test-scan-artifacts.js`、
> `M tools/r0/e2e/run.js`、`M tools/r0/e2e/specs/c-controls.js`、
> `?? docs/r0-evidence-7th-fix/p2/`、`?? docs/r0-evidence-7th-fix/p1/round2/` 等）。
> 这些属于 **P2 与后续修复的在途工作**，按 team-lead 指令**不计入 P1 越界**，
> 且**未纳入本报告的任何实测结论** —— 本报告全部验证对象均为
> commit `0c62f5017c8b616f6d0c5079c55a9b4a13d1f68f` 的**已交付版本**。
> 若在途改动形成新提交，须**另做回归验证（Round 2）**，不得沿用本报告结论。

---

## 9. 第 8 项 · 真机 artifact `13415542360` 独立复核（PASS，team-lead 补充任务）

### 9.1 下载链路与完整性硬校验

| 步骤 | 命令 / 端点 | 实测 |
|---|---|---|
| 元数据（匿名） | `curl https://api.github.com/repos/LinTia/FastLadger/actions/artifacts/13415542360` | `HTTP 200`；`name=r0-e2e-evidence-57e5cd0d…`、`size_in_bytes=3629800`、`expired=false`、`digest=sha256:50af2db9…298a`、`workflow_run.id=42029758227`、`head_sha=0c62f501…`、`head_branch=fix/r0-sixth-runtime-defects` |
| zip 端点（匿名） | `curl …/actions/artifacts/13415542360/zip` | `HTTP 401`（该端点需认证） |
| 第 1 步取 302 | 带本机 git credential 请求同一端点、**不跟随重定向** | `STEP1_HTTP=302`，`LOCATION_PRESENT=yes`；Location 为 Azure blob SAS URL，**未打印、未记录签名** |
| 第 2 步下载 blob | 对 Location **匿名**请求（`-H "Authorization:"` 清空、不带任何 GitHub token） | `HTTP=200`、`SIZE=3629800`、`TIME=82.05s`、`CURL_EXIT=0` |
| 完整性 | `sha256sum` / `unzip -t` | sha256 = `50af2db90301cbf62dec61e22e2af9e460f0fbebaae247bb5225dda4c90e298a`，**与 API `digest` 逐字一致**；`unzip -t` → `No errors detected` |

> 按 team-lead 提示：token 只用于取得 302，**下载 blob 时未带任何 GitHub token**（否则 401）。
> QA 全程未打印 token、未打印 SAS URL、未把二者写入任何报告或证据文件；下载的 zip 与解压目录均在系统临时目录，不入库。

### 9.2 解压统计（与 team-lead 数字逐项核对）

| 指标 | team-lead | QA 实测 | 一致 |
|---|---|---|---|
| 文件总数 | 99 | **99**（`find -type f`；26 个目录） | ✅ |
| zip 文件 | 0 | **0** | ✅ |
| 黑名单文件 | 0 | **0**（黑名单扩展名 0 / `server.log` 0 / 文件名敏感词 0 / 符号链接 0） | ✅ |
| 文本文件全扫 | 19 | **19**（13 json + 3 txt + 3 md；80 png 按设计跳过内容扫描） | ✅ |
| JWT / Bearer / Authorization / Cookie 命中 | 0 | **0** | ✅ |
| 判定 | CLEAN | **CLEAN** | ✅ |

扩展名分布：`.png=80, .json=13, .txt=3, .md=3`（合计 99）。

### 9.3 QA 独立审计脚本（不依赖被测代码）

自写 `qa-artifact-audit.py`（系统临时目录，不入库），规则独立实现并与 P1 语义对齐，
额外增加**全文件原始字节级兜底扫描**（覆盖 80 个按设计跳过内容扫描的二进制文件），
用于复核 N1/N2 边界是否在本轮被利用。输出仅含路径 / 类型 / 数量，不输出任何值。

| 检查层 | 实测 |
|---|---|
| 文本规则（19 个文本文件）：JWT_LITERAL / BEARER_TOKEN / AUTH_HEADER / COOKIE_HEADER / EBK_JWT_SECRET_ASSIGN | **命中 0** |
| 全文件原始字节**严格凭据形态**（99 个文件，含 80 二进制）：完整 JWT 三段式 / Bearer+长值 / Authorization 赋值 / Cookie 赋值 / EBK 赋值 | **命中 0** |
| 全文件原始字节**短标记**（观察项）：`eyJ`/`Bearer`/`Authorization`/`authorization`/`Set-Cookie`/`EBK_JWT_SECRET` | 仅 1 处 `BYTE_eyJ_PREFIX`；其余全部 0 |
| 结论 | `AUDIT_FINAL_EXIT=0`，**CLEAN** |
| 交叉验证：被测 `scan-artifacts.js` 对同一解压目录 | `SCAN_EXIT=0`；`文件总数=99 文本已扫描=19 二进制跳过=80 符号链接=0`；`命中总数=0`；`结论：CLEAN` |

**QA 自修记录（如实）**：初版审计脚本把「字节里出现孤立 `eyJ` 短串」直接计入阻断，
得出 `AUDIT_EXIT=2 / BLOCKED` —— 这是**我的断言过宽导致的假阳性**（测试代码 Bug，非源码问题），
按智能路由规则由 QA 自修。对唯一命中做字节级性质分析（只输出布尔 / 比例 / 偏移，不输出任何字节内容）：

```
[BYTE] 2026-11-05_12-46-20-57e5cd0-c/C/I-TXNS/I-TXNS-03.png | BYTE_eyJ_PREFIX | 1
[CTX ] 同文件 | offset=25509 | full_jwt_shape=False | header_dot_shape=False
      | ctx_printable_ratio=0.319 | png_magic=True
```

即：该文件是**合法 PNG**（魔数正确），`eyJ` 出现在压缩图像数据偏移 25509 处，
周边仅 31.9% 为可打印 ASCII，既不构成完整 JWT 三段式，也不构成 JWT header 段 + 点号，
判定为 **3 字节随机巧合，非凭据**。修正后把「短标记」降级为观察项、「严格凭据形态」作为阻断项，
重跑得 `AUDIT_FINAL_EXIT=0 / CLEAN`。初版日志 `artifact-audit.log` 与修正版 `artifact-audit-final.log` 均保留，不覆盖。

> **边界实证（不替 N1/N2 圆场）**：`80/80` PNG 魔数合法、`19/19` 文本文件无 NUL 前缀
> （`artifact-magic-check.log`），说明 **N1 / N2 这两条绕过在本轮真实产物中未被触发**；
> 但它们作为实现缺陷依然客观存在（§4 已用合成样本端到端证明），不能因本轮 CLEAN 就视为已解决。

### 9.4 溯源：确认这份 artifact 确属 P1 整改后的新 CI

| 核查 | 实测 |
|---|---|
| `actions/runs/42029758227` | `HTTP 200`；`event=pull_request`、`status=completed`、`conclusion=success`、`head_branch=fix/r0-sixth-runtime-defects`、**`head_sha=0c62f5017c8b616f6d0c5079c55a9b4a13d1f68f`（P1 commit）**、`pull_requests_count=1` |
| 该 run 的 artifact 列表 | `total_count=1`，唯一 artifact 即 `13415542360`（`size=3629800`、`expired=false`） |
| 名称 SHA 差异说明 | artifact 名后缀 `57e5cd0d…` 是 PR 事件的 `github.sha`（merge commit），与 `head_sha=0c62f501…` 不同属正常现象，**不是**旧 artifact 或可疑产物；以 API `head_sha` 为准 |

### 9.5 真机 step outcome：阻断链在 CI 上确实跑通

`actions/runs/42029758227/jobs` → `HTTP 200`，`total_count=1`，job `conclusion=success`：

```
step 09: 包 A · 11 项状态/权限/同步场景         | success
step 10: 包 D · 四项业务缺陷回归（门禁）         | success
step 11: 包 C · 228 条逐控件运行时验收（报告型） | success
step 12: 汇总退出码                             | success
step 13: 准备上传证据（白名单）                   | success
step 14: 上传前安全扫描（失败即阻断）             | success
step 15: 上传证据（日志/截图/报告/校验和）        | success   ← 确实执行，非 skipped
```

→ 实证 §6.2 状态矩阵第 1 行（prepare success → scan success → upload 执行 → 有 artifact）。

**§6.4 提示的实证闭环**：这次 job 确实是**绿色 CI**，但「绿色」本身并不能证明上传发生 ——
是 QA 通过 **step 15 outcome + artifact 列表 + 下载并核对 sha256/digest + 内容独立扫描**
四层证据才确认「绿色 CI 对应的是真实且合规的上传」。
**请勿反向简化为「CI 绿 = 证据已安全上传」**；其它 run 仍须逐 step、逐 artifact 复核。

### 9.6 本项结论

**PASS** —— QA 独立下载、独立实现扫描规则、独立溯源后，与 team-lead 结论**一致**：
`99 文件 / 0 zip / 0 黑名单 / 19 文本全扫 / 命中 0 / CLEAN`。
增量证据：下载 sha256 与 API digest 一致、artifact 唯一且属 P1 commit、真机三个新增 step 全 success、
全文件字节级兜底扫描（含 80 二进制）、N1/N2 本轮未触发的实证、以及 1 处 `eyJ` 短串的假阳性排查与 QA 自修。

---

## 10. 新发现 / 未解决问题

| ID | 严重度 | 问题 | 影响 | 建议 |
|---|---|---|---|---|
| **N1** | **高** | **白名单文本文件的 NUL 前缀 fail-open 绕过**（§4 ⑥ + e2e-06）。`scan-artifacts.js` `looksBinary()` L164–170 在前 4096 字节内发现 NUL 即把整个文件判二进制；L265–268 `binarySkipped += 1; continue;` **静默跳过内容扫描**并返回 0 CLEAN。 | `.log/.txt/.json/.md` 中任意 JWT/Bearer/Authorization/Cookie/密钥赋值只需文件头 1 个 NUL 即可绕过全部文本规则，经 `prepare` 收集后被 `upload-artifact` 上传。**与 P1 宣称的「失败关闭」原则直接矛盾（静默跳过 = 失败开放）**，且工程师报告与 10 组负向测试均未披露/覆盖。 | 对白名单**文本**扩展名不做 NUL 启发式跳过；或将「因二进制跳过内容扫描」改为「不可判定 → 退出 2 阻断」；至少对跳过项输出显式告警并禁止静默 CLEAN。 |
| **N2** | 中 | **`.png` 扩展名无条件跳过内容扫描的端到端放行**（§4 ⑤ + e2e-05）。`BINARY_EXT` L47–51 + L256–259 不读内容；`prepare` `classify()` L117–138 也不读内容。 | 纯文本凭据命名为 `shot.png` 即可端到端进入上传目录并判定 CLEAN。工程师已列为「已知边界」，但描述弱化为「截图画面拍到令牌」，未披露「文件即文本」路径；`test-scan-artifacts.js` case-09 把该行为固化为 PASS。 | 对 `.png/.jpg/...` 至少校验文件魔数（PNG 8 字节签名），非真实图片则阻断；或明确由 Owner 书面接受该残留风险并在交付说明中如实写明。 |
| **N3** | 中 | **CI 可观测性缺口**（§6.4）。汇总退出码 step（L99–109）在三个新 step 之前，summary 不含 prepare/scan/upload outcome。 | 第三方无法从 CI summary 判断扫描是否通过、上传是否发生；配合「静默零上传」通道，绿色 CI 极具误导性。 | 若允许改 workflow，在 summary 追加三行 outcome；若不允许，按 §6.4 第 4 点手工复核 step 与 artifacts 列表。 |
| **N4** | 低 | **证据文档 HEAD 不一致**。`p1/p1-report.md` L4 记 HEAD 为 `8082ca85d4f679ed7698fc51009bc6a214c9ac0c`，而实际交付 HEAD 为 `0c62f5017c8b616f6d0c5079c55a9b4a13d1f68f`（P1 提交后）。 | 第三方按报告内 HEAD 核对会定位到错误提交。 | 交付前更新报告 HEAD，或注明「报告写于提交前，实际交付 commit = 0c62f501…」。 |
| **N5** | 低 | **假阳性（误阻断）**（§4.6）。`AUTH_HEADER` / `COOKIE_HEADER` 正则 L66–67 只看「关键字 + 赋值 + ≥8 字符值」，不校验值形态。 | 合法业务字段（如 `"authorization": "role-based-access"`、`"cookie": "same-site-lax"`）即触发退出 2，导致合法证据无法上传、CI 无谓变红。 | 收紧为「值需具备 base64/JWT/高熵形态」或加白名单值；需权衡漏报风险。 |
| **N6** | 提示 | **规则覆盖有限**。`TEXT_RULES` 为已知形态白名单（eyJ 三段式 / Bearer / Authorization / Cookie / EBK_JWT_SECRET），非通用 DLP。 | 未带上述关键字的凭据形态不拦。**本轮核心风险（trace.zip 内 `Authorization: Bearer <JWT>`）已被「排除 zip + 文本扫描」双重覆盖**，但不可外推为「任何凭据都能拦」。 | 在交付说明中限定适用范围。 |
| **N7** | 提示 | **历史 artifact 未追溯**。team-lead 已删本轮 4 个含凭据 artifact（HTTP 204）；更早 4 个历史产物按指令未动（工程师 `p1-report.md` L252–253 亦注明）。 | 本整改只阻断「本轮起的新上传」，不消除历史已公开内容。 | 由 Owner 决定历史 artifact 处置与凭据轮换确认。 |

**未发现的问题（如实记录）**：符号链接在 `prepare` 与 `scan` 两层均被正确跳过/阻断（e2e-04 + ④）；
`trace.zip` 等 12 个归档在真实链路中被 100% 排除；`upload-artifact` 全仓唯一；P1 commit 范围未越界。

**补充（第 8 项真机复核后）**：`N1` / `N2` 属于实现缺陷，但在本轮真实 artifact `13415542360` 中
**均未被触发**（80/80 PNG 魔数合法、19/19 文本文件无 NUL 前缀，见 §9.3 / `artifact-magic-check.log`），
该 artifact 经独立复核判定 CLEAN。CLEAN 只说明「本轮产物未被这两条路径利用」，**不等于缺陷已修复**；
N1–N7 仍全部列为未解决项，路由判定（§11）不因本项 PASS 而改变。

---

## 11. 智能路由判定

### 11.1 轮次与判定

- 本轮为 **Round 1**（写/设计验证 → 执行 → 分析 → 路由）。按 QA 2 轮硬上限，不进入第 3 轮。
  第 8 项（team-lead 补充的真机 artifact 复核）为**同轮追加的验证项**，其 PASS 不改变本轮路由判定。
- **路由决策：`Engineer`（源码/规则存在缺陷）**。

判定依据（「断言期望的是符合 PRD/设计的正确行为，但得到错误输出 → 源码 Bug」）：
本次任务目标是「证明 P1 防护真的生效」。QA 的断言是「白名单目录内不应存在可被上传的、携带凭据形态的内容，扫描器应失败关闭」。
实测中 ⑥（NUL 前缀白名单 `.log`）与 ⑤（文本伪装 `.png`）**确实携带合成凭据形态，却得到 `scan` 退出 0 / `结论：CLEAN` 并端到端可上传**，
且扫描器对合法业务字段给出退出 2 的误阻断（N5）。断言正确、实现行为错误 → 属于**被测源码缺陷**，不是测试代码错误，故**不自修测试**，路由 Engineer。

> 说明：⑤ 被工程师声明为「已知边界」，其性质是「风险接受/披露不足」而非新 Bug；
> **⑥（N1）则是任何交付文档都未披露的、使白名单文本类文件整体失效的实现缺陷**，是本次路由 Engineer 的主要依据。

### 11.2 提交给 Engineer 的缺陷要点

| 项 | 内容 |
|---|---|
| 源文件 | `tools/r0/e2e/scripts/scan-artifacts.js`（`looksBinary()` L164–170、`BINARY_EXT` L47–51、主循环 L255–268、`TEXT_RULES` L63–69）；关联 `tools/r0/e2e/scripts/prepare-upload.js`（`classify()` L117–138 仅按文件名判定） |
| 期望 | 待上传的白名单文本（`.log/.txt/.json/.md`）必须逐条应用 5 条文本规则；无法判定内容时应**失败关闭**（非 0）；合法业务字段不应被误阻断 |
| 实际 | 文件头 1 个 NUL → 整个文件被判二进制 → 跳过全部文本规则 → 退出 0 CLEAN；`.png` 扩展名内容从不读取；`authorization`/`cookie` 普通值触发退出 2 |
| 复现 | 见 §4.2 / §4.4 / §4.6 与 `scan-bypass-06-nul-log.log`、`e2e-06-*.log`、`scan-false-positive.log` |
| 附带 | N3（CI summary 不含新 step outcome）、N4（p1-report HEAD 不一致）建议在交付前一并处理或书面说明 |

### 11.3 若本轮不修复

若 team-lead 决定本轮不安排 Engineer 修复（例如维持「最小改动 + 已知边界」口径），则：
- 上述 **N1 / N2 / N3 / N4 / N5 / N6 / N7 全部转为 `Known Issues`** 写入交付说明，**不得**表述为「已解决」；
- QA 可在 **Round 2** 对 Engineer 的修复做一次回归验证；若无修复或无第 2 轮，即以本报告结论收尾，**不进入第 3 轮**。
- 无论是否修复，**§6.4 的第三方提示必须随交付一并给出**：**禁止把绿色 CI 描述成「证据已安全上传」**。

### 11.4 建议给第三方的最终表述（可直接引用）

> P1 整改已消除本次事件的根因：`trace.zip` 等归档在 `prepare-upload.js` 被 100% 排除，
> 上传 step 的 `if` 已绑定 `artifact_scan.outcome == 'success'`，扫描命中时实测退出 2 且上传 step 必不执行（QA 独立复跑通过）。
> 但扫描器为基于已知形态规则的轻量闸门，QA 独立注入样本已证明存在「NUL 前缀白名单文本」与「文本伪装 `.png`」
> 两条可端到端放行的内容绕过，以及合法业务字段误阻断；这些为未解决项。
> **CI 绿色或 workflow 文件存在，均不构成「证据已安全上传」的证明**；必须逐 step 核对 outcome 并到 artifacts 列表与下载内容独立复核。

---

## 附录 A · 复现命令清单（全部由 QA 独立执行）

```bash
cd /c/Users/47006/WorkBuddy/fastledger/repo_tmp

# 1 负向测试
node --test tools/r0/e2e/test-scan-artifacts.js

# 2 真实产物链路
node tools/r0/e2e/scripts/prepare-upload.js \
  --source artifacts/r0-qa-verify/2026-11-05_09-59-36-dadc9a3-a \
  --dest /tmp/fastledger-qa-p1-verify/chain-upload
node tools/r0/e2e/scripts/scan-artifacts.js /tmp/fastledger-qa-p1-verify/chain-upload

# 3 绕过样本（辅助脚本见附录 B；样本在系统临时目录，不入库）
node /tmp/fastledger-qa-p1-verify/qa-probe-helper.js build
for c in bypass-01-jwt-txt bypass-02-trace-zip bypass-03-json-hidden \
         bypass-04-symlink bypass-05-png-content bypass-06-nul-log; do
  node tools/r0/e2e/scripts/scan-artifacts.js /tmp/fastledger-qa-p1-verify/$c; echo "EXIT=$?"
done
node /tmp/fastledger-qa-p1-verify/qa-probe-helper.js verify <scan-log>
node /tmp/fastledger-qa-p1-verify/qa-probe-helper.js probe-file <dest-file>

# 4 workflow 静态审查
git diff HEAD~1..HEAD -- .github/workflows/r0-e2e.yml
git show HEAD~1:.github/workflows/r0-e2e.yml | sed -n '/# A 是门禁/,/GITHUB_STEP_SUMMARY/p'
sed -n '/# A 是门禁/,/GITHUB_STEP_SUMMARY/p' .github/workflows/r0-e2e.yml

# 6 全仓上传面
grep -rn "upload-artifact" .github/workflows/
grep -nE "upload|artifact|actions/upload" .github/workflows/ci.yml
git ls-files | grep -Ei 'trace\.zip|(^|/)artifacts/'
node tools/r0/scan-credentials.js --strict

# 7 未越界（P2 并行在途，改以 P1 commit 范围判定）
git log --oneline -4
git diff --stat 8082ca8..0c62f50
git diff --name-only 8082ca8..0c62f50 | grep -Ei 'app.js|style.css|controls.json|c-controls.js|run.js|PRD|WBS|DB|API'

# 8 真机 artifact 13415542360 独立复核
curl -sS https://api.github.com/repos/LinTia/FastLadger/actions/artifacts/13415542360
curl -sS https://api.github.com/repos/LinTia/FastLadger/actions/runs/42029758227
curl -sS https://api.github.com/repos/LinTia/FastLadger/actions/runs/42029758227/artifacts
curl -sS https://api.github.com/repos/LinTia/FastLadger/actions/runs/42029758227/jobs
# ① 带 token 请求 zip 端点、不跟随重定向，取 302 的 Location（不打印/不记录 SAS 签名）
# ② 对 Location 匿名下载：-H "Authorization:" 清空，不带任何 GitHub token（否则 401）
curl -sS -m 240 -H "Authorization:" -o /tmp/.../artifact.zip "<Location>"
sha256sum /tmp/.../artifact.zip                 # 应与 API digest 一致
unzip -q /tmp/.../artifact.zip -d /tmp/.../artifact-extract
python qa-artifact-audit.py /tmp/.../artifact-extract
node tools/r0/e2e/scripts/scan-artifacts.js /tmp/.../artifact-extract
```

## 附录 B · 合成值生成逻辑（要点，不含任何值）

```js
const b64url = (s) => Buffer.from(s, 'utf8').toString('base64url');
// 三段式：header 以 eyJ 起始，三段均为 base64url 字符集且长度 >= 8
const jwt = [
  b64url(JSON.stringify({ alg: 'HS256', typ: 'JWT', qa: 'SYNTHETIC' })),
  b64url(JSON.stringify({ sub: 'SYNTHETIC_QA_PROBE', scope: 'NOT_A_REAL_CREDENTIAL', iat: 0 })),
  b64url('SYNTHETIC_SIGNATURE_FOR_QA_PROBE_0000'),
].join('.');
const bearer = ['Bearer', ' ', b64url('SYNTHETIC_BEARER_QA_PROBE_0000')].join('');
```

- 全部值在**运行时**生成，源码中不存在 `eyJ…` 或真实凭据字面量；
- 样本文件均写入系统临时目录 `$TMP/fastledger-qa-p1-verify/`，未写入仓库；
- `verify` 子命令重新生成同样的值用于比对，仅打印布尔（`contains_*=true/false`），不打印值；
- ⑤ ⑥ 的端到端样本在 `probe-file` 校验中确认 `contains_jwt_value=true`，但报告与日志均不含该值本身。
