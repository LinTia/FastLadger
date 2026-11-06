# FastLedger R0 · P1 凭据扫描器 K1-K3 + 第八次复核 RV-07~RV-13 安全闭环整改（整改报告）

- 任务单：FastLadger R0 后续整改任务单 - P1 凭据扫描器 K1-K3 安全绕过修复；及 ChatGPT 提交到 main 的权威审计 `docs/FastLedger-R0-第八次安全扫描器独立复核.md`（RV-07~RV-13）。
- 对照基线（修复前）：commit `659c0c3`（PR #3 最新提交，第三方独立复核基线）。
- 修复后分支：`fix/r0-sixth-runtime-defects`（不合并 main）。
- 状态边界：PR #3 保持 Open；R0 保持 BLOCK；不自行宣布 R0 通过；不自行合并。
- 审计结论处置：**不更改 ChatGPT 审计结论**，不合并豆包审计分支；本轮仅就 RV-07~RV-13 所列必须整改项落地并独立取证。

---

## 0. 一句话结论

针对第六轮第三方在 `659c0c3` 上复现的三类凭据扫描绕过（K1 编码绕过 / K2 短值低熵漏报 / K3 字段名与 Base64 绕过），已做失败关闭（fail-closed）重构；在此基础上，针对第八次独立复核（ChatGPT 审计）列出的 RV-07~RV-13 七项必须整改，进一步补全：**明文 PEM 私钥标记（RV-07）**、**fail-closed 升级为上传硬性前置 + 负向集成测试（RV-08）**、**严格 UTF-8 解码封死非法字节绕过（RV-09）**、**Base64URL 与无 padding 覆盖（RV-10）**、**token 值内部 NUL 测试修正（RV-11）**、**不可解码样本走真实 prepare→scan 上传链路（RV-12）**、**白名单收敛后重建并校验 SHA256SUMS（RV-13）**。扫描器测试由 30 项扩展至 41 项（合成凭据、真实断言）；修复前后对照 14/15 可复现绕过已封死（K2-04 正确保持 CLEAN）。所有门禁与证据上传均经 `R0 E2E` CI 工作流独立验证。

---

## 1. 修改文件清单

| 文件 | 改动类型 | 说明 |
|---|---|---|
| `tools/r0/e2e/scripts/scan-artifacts.js` | 重写（Round 3 + 第八轮扩展） | 多编码可靠解码、UTF-16BE 用 TextDecoder、NUL/噪声复扫泛化、强敏感键名联合判定、Base64 解码复扫、明文 PEM 私钥标记、严格 UTF-8（fatal）、UTF-16 BOM 闸门 |
| `tools/r0/e2e/test-scan-artifacts.js` | 扩展（→41 项） | 新增 case-31~case-41 共 11 项 RV-07~RV-13 负向合成测试（含 prepare→scan 全链路、Base64URL、PEM 全链路） |
| `tools/r0/e2e/scripts/before-after-evidence.js` | 扩展 | 基线由 `HEAD` 改为 `659c0c3`；新增 RV-09-01（非法字节在 Bearer 关键词内）/ RV-09-02（非法字节在 token 值内）对照行 |
| `tools/r0/e2e/scripts/prepare-upload.js` | 扩展（RV-13） | 白名单收敛后重建 SHA256SUMS（仅含实际上传文件、排除历史引用）并做完整校验（0 missing/0 mismatch），失败即 EXIT_ERROR |
| `tools/r0/e2e/scripts/verify-fail-closed-gate.js` | 新增（RV-08） | 可复用失败关闭闸门验证器：凭据样本严格 rc==2、干净样本严格 rc==0；负向（`--expect-skip`）证明上传被跳过 |
| `tools/r0/e2e/scripts/fixtures/broken-scanner.js` | 新增（RV-08） | 破损扫描器替身：永远 `exitCode=0`（总返回 CLEAN），用于负向测试证明 fail-closed 不误放行 |
| `.github/workflows/r0-e2e.yml` | 扩展 | 门禁 step 更名（41 项）；新增「失败关闭集成验证」「失败关闭负向证明」step；`artifact_upload` 改为双条件前置；新增「修复前后对照」step；checkout 改 `fetch-depth: 0` 以取到基线 commit |
| `docs/r0-evidence-7th-fix/p1/k1-k3-remediation.md` | 新增/更新 | 本报告 |

未改动（明确不在授权范围内）：PRD / WBS / 核心业务规则 / DB 结构 / `style.css` 账户名手机端隐藏 / 13 条历史控件 ID 映射 / 仓库可见性与 Git 历史 / 分支保护。

---

## 2. 每项 K 的测试名 / 位置 / 结果（K1-K3，case-18~30）

全部位于 `tools/r0/e2e/test-scan-artifacts.js`，运行 `node --test tools/r0/e2e/test-scan-artifacts.js`。

| 编号 | 条款 | 测试名 | 命中类型 | 结果 |
|---|---|---|---|---|
| case-18 | K1-01 | UTF-16LE 编码 Bearer 凭据 | BEARER_TOKEN | PASS（BLOCKED，rc=2） |
| case-19 | K1-02 | UTF-16BE 编码鉴权头凭据 | BEARER_TOKEN | PASS（BLOCKED，rc=2） |
| case-20 | K1-03 / RV-11 | NUL at delimiter boundary (Bearer<SPACE>NUL<token>) | BEARER_TOKEN（NUL 复扫） | PASS（BLOCKED，rc=2） |
| case-21 | K1-04 | 无法可靠解码的文本型文件 | UNDECODABLE | PASS（BLOCKED，rc=2） |
| case-22 | K2-01 | 短 Token（token: ab12） | SENSITIVE_ASSIGN | PASS（BLOCKED，rc=2） |
| case-23 | K2-02 | 短 API Key（api_key: xy78） | SENSITIVE_ASSIGN | PASS（BLOCKED，rc=2） |
| case-24 | K2-03 | 短密码 / 低熵（password: pw9） | SENSITIVE_ASSIGN | PASS（BLOCKED，rc=2） |
| case-25 | K2-04 | 普通短业务字段（unit/qty/note/code） | 无 | PASS（CLEAN，rc=0，无误报） |
| case-26 | K3-01 | x-api-key 头 | SENSITIVE_ASSIGN | PASS（BLOCKED，rc=2） |
| case-27 | K3-02 | apiKey 与 API_KEY 变体 | SENSITIVE_ASSIGN | PASS（BLOCKED，rc=2） |
| case-28 | K3-03 | Base64 编码凭据 | BASE64_CREDENTIAL | PASS（BLOCKED，rc=2） |
| case-29 | K3-04 | JSON 嵌套凭据（引号键） | SENSITIVE_ASSIGN | PASS（BLOCKED，rc=2） |
| case-30 | K3-05 | 大小写与分隔符变体（X_Api_Key / client-secret） | SENSITIVE_ASSIGN | PASS（BLOCKED，rc=2） |

既有 17 项（case-01~17）全部回归通过，无退化。

---

## 3. 修复前绕过证据（基线 `659c0c3`）

用 `before-after-evidence.js` 从 `git show 659c0c3:tools/r0/e2e/scripts/scan-artifacts.js` 取出修复前扫描器，对与上面完全一致的样本运行。结果：12 个可复现绕过样本全部 `rc=0`（CLEAN，即绕过成立）；K2-04 控制样本本就 CLEAN（正确）。详见第 12 节「修复前后负向样本对照」完整 15 行对照表（含 RV-09-01/02）。

---

## 4. 修复后阻断证据

同一工具、同一批样本，对当前（修复后）扫描器运行：12 个绕过样本全部 `rc=2`（BLOCKED）；K2-04 仍为 `rc=0`（CLEAN，无误报）。本地 41/41 测试见 CI（Run `42147186477`）门禁 step，全部 PASS。

---

## 5. 正常字段不误报回归

- case-25（K2-04）：`{unit,qty,note,code,status}` 普通短业务字段 → CLEAN（rc=0），`doesNotMatch(SENSITIVE_ASSIGN)`。
- case-12（既有）：合法业务字段 `authorization: role-based-access` / `cookie: same-site-lax` → CLEAN，`doesNotMatch(AUTH_HEADER|COOKIE_HEADER)`。
- 强敏感键名联合判定以 `BENIGN_PLACEHOLDER`（none/null/true/false/redacted/...）抑制误报，而非靠「值长度 >= 4」——短凭据（K2-01/02/03）因而仍被阻断，正常短业务字段不误报。

---

## 6. 扫描失败上传不执行的 CI 证据（fail-closed）

`r0-e2e.yml` 已落地多道闸门：

1. `artifact_scan` step：`if: always() && steps.artifact_prepare.outcome == 'success'`，扫描器以退出码 2 失败时该 step outcome!=success。
2. `artifact_upload` step：`if: always() && steps.artifact_scan.outcome == 'success' && steps.fail_closed_check.outcome == 'success'`（**双条件**，无 `continue-on-error`）；扫描或 fail-closed 任一不通过则 upload 不被触发。
3. `fail_closed_check` step（RV-08）：用合成凭据样本（x-api-key + UTF-16LE Bearer）运行扫描器，经 `verify-fail-closed-gate.js` 断言 `credRc==2 && cleanRc==0`（否则 `::error::FAIL-OPEN` 退出）；用干净样本断言 `rc==0`。直接证明「命中 → 退出码非 0 → `if: outcome==success` 不触发 → 不上传」的失败关闭链路。
4. `fail_closed_negative` step（RV-08）：用 `fixtures/broken-scanner.js`（永远返回 CLEAN 的破损扫描器替身）经 gate 的 `--expect-skip` 模式验证，证明 fail-closed 验证失败时上传闸门被跳过、Artifact 数量 = 0；若破损扫描器仍放行则本步失败。

说明：扫描器自身无独立 `WARN` 退出码；空目录场景由 `upload-artifact` 的 `if-no-files-found: error` 使 job 失败。扫描异常（目录不存在 / 读取失败 / 超上限）以 `rc=1` 使 job 失败，因 upload 依赖 `outcome==success`，**失败后不会继续上传**。

---

## 7. 状态说明（BLOCK / WARN / CLEAN / 扫描失败）

- BLOCKED（rc=2）：命中黑名单文件名、疑似凭据、明文 PEM 私钥、或无法可靠解码的文本型文件 → 禁止上传；作为门禁使 job 失败。
- WARN：扫描器不产出独立 WARN 退出码；仅空目录会在输出中标注 WARN 提示，退出码仍为 CLEAN/0；真正阻断空目录上传的是 `upload-artifact` 的 `if-no-files-found: error`。
- CLEAN（rc=0）：全量扫描（latin1 + 可靠 UTF-8/UTF-16LE/UTF-16BE 复扫 + 强敏感键名联合判定 + Base64/Base64URL 解码复扫 + 噪声复扫）无任何命中 → 允许上传。
- 扫描失败（rc=1）：目录不存在 / 读取异常 / 超单文件上限（32 MiB）/ 未捕获异常 → 不降级为 CLEAN，job 失败；因 upload step 依赖 `outcome==success`，**扫描失败绝不会继续执行上传**。

任何「读不了 / 解不了 / 异常 / 超时」都不降级为 CLEAN；上传前扫描覆盖全部待上传文件（含符号链接单独记录、不跟随）。

---

## 8. 本地 + CI 完整测试与退出码

- 本地：`node --test tools/r0/e2e/test-scan-artifacts.js` → tests=41 pass=41 fail=0，退出码 0。
- CI：见第 9 节（Run ID / SHA / Artifact）。CI 中同一 `node --test` 作为门禁 step，任一失败即 job 失败。
- 修复前后对照：见第 12 节，由 `before-after-evidence.js`（CI 内 `fetch-depth:0` 取到基线 659c0c3）产出，14/15 可复现绕过已封死。

---

## 9. CI Run ID / Commit SHA / Artifact ID

- Commit SHA（P1 扫描器修复）：`f1ddc7df810e04250aa421b467fa31ebb1186375`
- Commit SHA（CI 工作流强化 / fetch-depth + 对照 step）：`5519b9deaae623e6d851c76ee90c31da46743dfd`
- CI Run ID（R0 E2E 工作流，A/D 门禁/C 报告型/上传前扫描/失败关闭/修复前后对照）：`42147186477`（head = 5519b9d）
- Artifact ID：`13471225181`；名称：`r0-e2e-evidence-5519b9deaae623e6d851c76ee90c31da46743dfd`（size 3652230 B，retention 30 天）
- 说明：CI 针对最终提交运行（`workflow_dispatch` 于 `fix/r0-sixth-runtime-defects`，head = 5519b9d）；R0 E2E run `42147186477` 已产出并上传 artifact，且因 `fail_closed_check` + `artifact_scan` 双条件满足，上传闸门放行。

---

## 10. 最新 Artifact 下载后文件清单与独立扫描

- 下载 Artifact（id `13471225181`）后，对全部文件用当前扫描器独立复扫：
  - 文件总数 = 102；内容已全量扫描 = 102；**跳过内容扫描 = 0**；二进制文件 = 80；符号链接 = 0
  - 编码解码：utf8可靠 = 22；utf16可靠 = 0；无法可靠解码(已阻断) = 0；可疑二进制(魔数不符) = 0
  - 命中文件/类型组合 = 0；命中总数 = 0；结论：**CLEAN（rc=0）**
- SHA256SUMS 完整校验：清单仅含实际上传文件（排除 SHA256SUMS.txt 自身与任何已排除 trace/db 历史引用），逐行核对 **missing = 0 / mismatch = 0**，校验通过。
- 目的达成：即便 CI 产出的 artifact 本身，经过与上传闸门同一把扫描器独立复扫，也不含可绕过凭据（**0 命中、0 跳过**），杜绝「CI 自身落盘即带凭据」的二次风险。

---

## 11. 未解决 Known Issues / 边界

- 单字节 NUL 注入若落在 token 值中间、且文件其余内容为可靠 UTF-8，依赖噪声复扫（去除所有非可打印字节后复扫）捕获；case-20（分隔符处）/ case-37（token 值内部）样例已实证命中。更极端「每字节随机错位的 NUL」形态未单独造样，当前设计按 fail-closed（无法可靠解码即 BLOCK）兜底。
- 已知二进制文件（PNG/JPEG/PDF 等）不进入 UNDECODABLE 阻断分支（按设计：魔数 + 内容双校验），但其内嵌凭据仍由 latin1 / UTF-16 规则拦截（见 case-09 / case-17）。
- 本轮不处理（任务单 / 审计明确）：OBS-ACCT-VIS-390（390px 账户名不可见，仍 FAIL 待 Owner）、13 条历史控件 ID 映射、RV-05 旧 SHA 公开历史、仓库 Private 决策、A05（仍需修复但不擅自标 PASS）。
- 不自行合并 main；不宣布 R0 通过；可复现绕过已如实封死并标记，未改名规避。

---

## 12. 第八次独立复核 RV-07~RV-13 整改明细

权威审计：`docs/FastLedger-R0-第八次安全扫描器独立复核.md`（ChatGPT 提交到 main，结论未改动）。七项必须整改逐项落地如下。

### RV-07 · 明文 PEM 私钥漏检（P1）

- **审计问题**：扫描器未识别明文 PEM 私钥（PRIVATE KEY / RSA / EC / OPENSSH / ENCRYPTED PRIVATE KEY），可能随证据上传泄露密钥。
- **修复**：`scan-artifacts.js` 新增常量
  `const PEM_PRIVATE_KEY_RE = /-----BEGIN (?:RSA |EC |OPENSSH |ENCRYPTED |DSA |[A-Z0-9]* )?PRIVATE KEY-----/g;`
  并在 `scanText` 中 `add('PEM_PRIVATE_KEY', countOf(text, PEM_PRIVATE_KEY_RE));`。命中即计入 `totalHits`，`totalHits>0` 时返回 `EXIT_BLOCKED=2`（BLOCK），**绝不输出密钥内容**（输出仅文件路径 + 类型 + 数量）。
- **测试**：case-38（明文 PEM 伪装成 `private-key.txt` 经 `prepare→scan` 全链路 BLOCK）、case-39（伪装成 `export.txt` 全链路 BLOCK）。本地验证：RSA / OPENSSH / EC / ENCRYPTED 四类 PEM 均 rc=2。

### RV-08 · fail-closed 上传门禁（P1）

- **审计问题**：`fail_closed_check` 未成为 Artifact upload 必要条件；凭据样本 rc 判定语义宽松（「非零即命中」可被误判放行）。
- **修复**：
  - 新增 `verify-fail-closed-gate.js`：复刻上传闸门判定 `fail_closed_passed = credRc === 2 && cleanRc === 0`；正向模式下二者任一不符则 `::error::FAIL-OPEN` 退出 1；负向模式（`--expect-skip`）要求 `upload_allowed=false` 否则 `::error::NEGATIVE TEST FAILED` 退出 1。
  - 新增 `fixtures/broken-scanner.js`：永远 `process.exitCode = 0`（总返回 CLEAN）的破损扫描器替身，用于负向证明。
  - `r0-e2e.yml`：`fail_closed_check` step 改为调用 gate（凭据样本严格 rc=2、干净样本 rc=0）；新增 `fail_closed_negative` step（`broken-scanner.js` + `--expect-skip`）；`artifact_upload` 的 `if` 改为 `steps.artifact_scan.outcome == 'success' && steps.fail_closed_check.outcome == 'success'`（**双条件**）；summary 增加「失败关闭负向证明 outcome」与「扫描通过且允许上传」双条件判定。
- **测试/CI**：gate 正向（真实扫描器）与负向（broken-scanner）均验证正确；CI 两步证明上传闸门在 fail-closed 失败时被跳过、Artifact 数量 = 0。

### RV-09 · 非法 UTF-8 绕过（P1）

- **审计问题**：少量非法字节打断关键词但文件仍被判可靠 UTF-8，从而 CLEAN 漏报。
- **修复**：`decodeUtf8` 改为严格 `new TextDecoder('utf-8', { fatal: true })`，任何非法字节即 `reliable:false`；无明确 UTF-16 BOM 时 `reliableText=false` → 文本型文件判 `UNDECODABLE` → BLOCK，**绝不降级为 CLEAN**。新增 UTF-16 BOM 闸门（`hasUtf16Bom`）防止 UTF-8 损坏文件被错误当成 UTF-16 放行。噪声复扫泛化为剔除所有非可打印字节 `[^\x20-\x7e\t\r\n]`（不再仅剔除 NUL），覆盖非法字节位于关键词内部 / token 值内部 / 字段名内部等形态。
- **测试**：case-31（Bearer 关键词内部非法字节）、case-32（token 值内部非法字节）、case-33（敏感字段名内部非法字节）。修复前后对照 RV-09-01 / RV-09-02 见下方对照表（旧基线 CLEAN→修复后 BLOCKED）。

### RV-10 · Base64URL 覆盖（P2）

- **审计问题**：Base64 解码复扫未覆盖 Base64URL（URL-safe `-`/`_`）与无 padding 形态。
- **修复**：`isBase64Credential` 归一化 `s = b64.replace(/-/g,'+').replace(/_/g,'/')` 并补 padding 至 4 倍数；`scanText` 正则 `for (const m of text.matchAll(/[A-Za-z0-9+/_-]{16,}={0,2}/g))` 同时覆盖标准 Base64 与 Base64URL、带/无 padding。`DECODED_CRED_MARKER` 覆盖 `Authorization/Bearer/api_key/secret/token/private key/BEGIN RSA/PRIVATE` 等。
- **测试**：case-34（Base64URL 带 padding 的 Authorization/Bearer）、case-35（Base64URL 无 padding 的 x-api-key）、case-36（Base64URL 无 padding 敏感键名+值）。负向：普通 Base64（图片/哈希）不因含 `=` 而误报。

### RV-11 · NUL 测试修正（P2）

- **审计问题**：K1-03 NUL 测试位于分隔符处，未覆盖 token 值内部 NUL。
- **修复**：保留 case-20（NUL at delimiter boundary，Bearer<SPACE>NUL<token>，经 BEARER_TOKEN 捕获），新增 case-37（token 值内部插入 NUL，经噪声复扫捕获）。两者并存，覆盖分隔符处与值内部两种形态。
- **测试**：case-20（保留）+ case-37（新增），均 BLOCKED（rc=2）。

### RV-12 · 上传链路测试（P2）

- **审计问题**：K1-04 不可解码样本（`.bin`）未走真实上传链路（仅单元扫描）。
- **修复**：新增 case-40，将不可解码样本伪装成白名单允许的文件名 `random.txt`，**真实执行 `prepare-upload`（白名单收敛）→ `scan-artifacts`（上传前扫描）**，证明文件进入上传目录后被 BLOCK，upload 闸门被触发（或 artifact 因扫描失败而不含该文件）。
- **测试**：case-40（不可解码 `random.txt` 全链路），rc=2。

### RV-13 · SHA256SUMS 重建与校验（P2）

- **审计问题**：白名单过滤后未重建 SHA256SUMS，Artifact 内 12 条引用指向已排除的 trace 文件。
- **修复**：`prepare-upload.js` 在白名单收敛（`collected.sort()` 之后）重建 SHA256SUMS：清单仅含 `collected` 中**实际上传**的文件（排除 SHA256SUMS.txt 自身，且对源目录可能自带的旧 SHA256SUMS.txt 不纳入待校验条目，避免其旧哈希与新清单不一致误报）；生成后立即完整校验，逐行要求 **missing = 0 / mismatch = 0**，否则 `return EXIT_ERROR`（失败关闭，不静默放过）；最后 `collected.push('SHA256SUMS.txt')` 仅用于展示。
- **测试**：case-41（源目录含正常文件 + `trace.zip`，最终 SHA256SUMS 只含实际上传文件、不含已排除的 trace 归档）。CI artifact 下载后独立复扫 + SHA256SUMS 校验见第 10 节（missing=0 / mismatch=0）。

### 修复前后负向样本对照（证据 §二-4，基线 659c0c3）

`before-after-evidence.js` 从 `git show 659c0c3:...` 取旧扫描器、与当前扫描器跑同一批合成样本：

```
sample                  | old(659c0c3) rc | new(fixed) rc | old verdict   | new verdict
K1-01 UTF-16LE Bearer   | 0            | 2             | CLEAN(bypass)| BLOCKED  << fixed
K1-02 UTF-16BE auth     | 0            | 2             | CLEAN(bypass)| BLOCKED  << fixed
K1-03 NUL insertion     | 0            | 2             | CLEAN(bypass)| BLOCKED  << fixed
K1-04 undecodable       | 0            | 2             | CLEAN(bypass)| BLOCKED  << fixed
K2-01 short token       | 0            | 2             | CLEAN(bypass)| BLOCKED  << fixed
K2-02 short api key     | 0            | 2             | CLEAN(bypass)| BLOCKED  << fixed
K2-03 short password    | 0            | 2             | CLEAN(bypass)| BLOCKED  << fixed
K2-04 business fields   | 0            | 0             | CLEAN        | CLEAN
K3-01 x-api-key         | 0            | 2             | CLEAN(bypass)| BLOCKED  << fixed
K3-02 apiKey/API_KEY    | 0            | 2             | CLEAN(bypass)| BLOCKED  << fixed
K3-03 Base64 cred       | 0            | 2             | CLEAN(bypass)| BLOCKED  << fixed
K3-04 JSON nested       | 0            | 2             | CLEAN(bypass)| BLOCKED  << fixed
K3-05 variants          | 0            | 2             | CLEAN(bypass)| BLOCKED  << fixed
RV-09-01 illegal UTF-8 in Bearer  | 0   | 2             | CLEAN(bypass)| BLOCKED  << fixed
RV-09-02 illegal UTF-8 in token  | 0   | 2             | CLEAN(bypass)| BLOCKED  << fixed
```

结论：修复前 14/15 为真实可复现绕过（CLEAN），与第三方复核结论一致；修复后全部 BLOCKED（含 RV-09-01/02 两类非法 UTF-8 绕过），K2-04 正确保持 CLEAN（无误报）。

---

## 回填记录（CI 完成后追加）

- CI Run ID：`42147186477`（R0 E2E）
- Commit SHA：`5519b9deaae623e6d851c76ee90c31da46743dfd`（workflow）/ `f1ddc7df810e04250aa421b467fa31ebb1186375`（scan fix）
- Artifact ID：`13471225181`（name `r0-e2e-evidence-5519b9deaae623e6d851c76ee90c31da46743dfd`）
- Artifact 下载清单与独立扫描结论：102 文件全量扫描、0 跳过、0 命中、CLEAN（rc=0）
- SHA256SUMS 校验：98 条清单 98 OK / 0 FAILED / 0 missing，且无任何 trace/zip/db 历史引用，通过
