# FastLedger R0 · P1 凭据扫描器 K1-K3 安全绕过修复（整改报告）

- 任务单：FastLadger R0 后续整改任务单 - P1 凭据扫描器 K1-K3 安全绕过修复
- 对照基线（修复前）：commit `659c0c3`（PR #3 最新提交，第三方独立复核基线）
- 修复后分支：`fix/r0-sixth-runtime-defects`（不合并 main）
- 状态边界：PR #3 保持 Open；R0 保持 BLOCK；不自行宣布 R0 通过；不自行合并。

---

## 0. 一句话结论

针对第三方在 `659c0c3` 上复现的三类凭据扫描绕过（K1 编码绕过 / K2 短值低熵漏报 / K3 字段名与 Base64 绕过），本轮对上传前扫描器做了失败关闭（fail-closed）重构：覆盖 UTF-8 / UTF-16LE / UTF-16BE 可靠解码、UTF-16BE 用 TextDecoder 修复（Node Buffer 不支持 `utf16be` 会直接崩溃）、新增 NUL 插入复扫、强敏感键名联合判定（含 JSON 引号键）、Base64 解码复扫、无法可靠解码一律 BLOCK。修复前 12 个可复现绕过样本在 `659c0c3` 上全部被判定 CLEAN（绕过实证）；修复后全部被 BLOCKED，且正常业务字段（K2-04 / case-12）仍 CLEAN（无误报）。新增 13 项负向合成测试（case-18~30），CI 门禁 + 失败关闭集成验证同步落地。

---

## 1. 修改文件清单

| 文件 | 改动类型 | 说明 |
|---|---|---|
| `tools/r0/e2e/scripts/scan-artifacts.js` | 重写（Round 3） | 多编码可靠解码、UTF-16BE 用 TextDecoder、NUL 插入复扫、强敏感键名联合判定（含 JSON 引号键）、Base64 解码复扫、无法可靠解码 → BLOCK |
| `tools/r0/e2e/test-scan-artifacts.js` | 扩展 | 新增 case-18~case-30 共 13 项 K1-K3 负向合成测试（合成凭据、真实断言） |
| `tools/r0/e2e/scripts/before-after-evidence.js` | 新增 | 修复前后对照取证工具（从 `git show HEAD` 取旧扫描器，与当前扫描器跑同一批合成样本，输出对照表；仅写系统临时目录，不入仓库） |
| `.github/workflows/r0-e2e.yml` | 扩展 | 新增「凭据扫描器负向合成测试（K1-K3，30 项）」门禁 step；新增「失败关闭（fail-closed）集成验证」step；summary 增加 `fail_closed_check` outcome |
| `docs/r0-evidence-7th-fix/p1/k1-k3-remediation.md` | 新增 | 本报告 |
| `docs/r0-evidence-7th-fix/p1/before-after-evidence.txt` | 新增 | 修复前后对照取证快照 |
| `docs/r0-evidence-7th-fix/p1/scan-tests.log` | 新增 | 本地 30/30 测试运行日志 |

未改动（明确不在授权范围内）：PRD / WBS / 核心业务规则 / DB 结构 / `style.css` 账户名手机端隐藏 / 13 条历史控件 ID 映射 / 仓库可见性与 Git 历史 / 分支保护。

---

## 2. 每项 K 的测试名 / 位置 / 结果

全部位于 `tools/r0/e2e/test-scan-artifacts.js`，运行 `node --test tools/r0/e2e/test-scan-artifacts.js`。

| 编号 | 条款 | 测试名 | 命中类型 | 结果 |
|---|---|---|---|---|
| case-18 | K1-01 | UTF-16LE 编码 Bearer 凭据 | BEARER_TOKEN | PASS（BLOCKED，rc=2） |
| case-19 | K1-02 | UTF-16BE 编码鉴权头凭据 | BEARER_TOKEN | PASS（BLOCKED，rc=2） |
| case-20 | K1-03 | 零散 NUL 插入 token 值 | BEARER_TOKEN（NUL 复扫） | PASS（BLOCKED，rc=2） |
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

用 `before-after-evidence.js` 从 `git show HEAD:tools/r0/e2e/scripts/scan-artifacts.js` 取出修复前扫描器，对与上面完全一致的 13 个合成样本运行。结果：12 个可复现绕过样本全部 `rc=0`（CLEAN，即绕过成立）；K2-04 控制样本本就 CLEAN（正确）。

```
sample                  | old(HEAD) rc | new(fixed) rc | old verdict   | new verdict
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
```

结论：修复前 12/13 为真实可复现绕过（CLEAN），与第三方复核结论一致。

---

## 4. 修复后阻断证据

同一工具、同一批样本，对当前（修复后）扫描器运行：12 个绕过样本全部 `rc=2`（BLOCKED）；K2-04 仍为 `rc=0`（CLEAN，无误报）。见上表 `new(fixed) rc` 列。本地 30/30 测试见 `scan-tests.log`（tests=30 pass=30 fail=0 exit=0）。

---

## 5. 正常字段不误报回归

- case-25（K2-04）：`{unit,qty,note,code,status}` 普通短业务字段 → CLEAN（rc=0），`doesNotMatch(SENSITIVE_ASSIGN)`。
- case-12（既有）：合法业务字段 `authorization: role-based-access` / `cookie: same-site-lax` → CLEAN，`doesNotMatch(AUTH_HEADER|COOKIE_HEADER)`。
- 强敏感键名联合判定以 `BENIGN_PLACEHOLDER`（none/null/true/false/redacted/...）抑制误报，而非靠「值长度 >= 4」——短凭据（K2-01/02/03）因而仍被阻断，正常短业务字段不误报。

---

## 6. 扫描失败上传不执行的 CI 证据（fail-closed）

`r0-e2e.yml` 已落地三道闸门（本轮新增两项）：

1. `artifact_scan` step：`if: always() && steps.artifact_prepare.outcome == 'success'`，扫描器以退出码 2 失败时该 step outcome=success 不成立。
2. `artifact_upload` step：`if: always() && steps.artifact_scan.outcome == 'success'`，**无 `continue-on-error`**；扫描不通过则 upload 不被触发。
3. 新增 `fail_closed_check` step：用合成凭据样本（x-api-key + 一段 UTF-16LE Bearer）运行扫描器，断言 `rc != 0`（否则 `::error::FAIL-OPEN` 并退出）；用干净样本断言 `rc == 0`。直接证明「命中 → 退出码非 0 → `if: outcome==success` 不触发 → 不上传」的失败关闭链路。

说明：扫描器自身无独立 `WARN` 退出码；空目录场景由 `upload-artifact` 的 `if-no-files-found: error` 使 job 失败（扫描器输出中会标注空目录 WARN，但退出码仍为 CLEAN/0）。扫描异常（目录不存在 / 读取失败 / 超上限）以 `rc=1` 使 job 失败，因 upload 依赖 `outcome==success`，**失败后不会继续上传**。

---

## 7. 状态说明（BLOCK / WARN / CLEAN / 扫描失败）

- BLOCKED（rc=2）：命中黑名单文件名、疑似凭据、或无法可靠解码的文本型文件 → 禁止上传；作为门禁使 job 失败。
- WARN：扫描器不产出独立 WARN 退出码；仅空目录会在输出中标注 WARN 提示，退出码仍为 CLEAN/0；真正阻断空目录上传的是 `upload-artifact` 的 `if-no-files-found: error`。
- CLEAN（rc=0）：全量扫描（latin1 + 可靠 UTF-8/UTF-16LE/UTF-16BE 复扫 + 强敏感键名联合判定 + Base64 解码复扫 + NUL 复扫）无任何命中 → 允许上传。
- 扫描失败（rc=1）：目录不存在 / 读取异常 / 超单文件上限（32 MiB）/ 未捕获异常 → 不降级为 CLEAN，job 失败；因 upload step 依赖 `outcome==success`，**扫描失败绝不会继续执行上传**。

任何「读不了 / 解不了 / 异常 / 超时」都不降级为 CLEAN；上传前扫描覆盖全部待上传文件（含符号链接单独记录、不跟随）。

---

## 8. 本地 + CI 完整测试与退出码

- 本地：`node --test tools/r0/e2e/test-scan-artifacts.js` → tests=30 pass=30 fail=0，退出码 0。
- CI：见第 9 节（Run ID / SHA / Artifact）。CI 中同一 `node --test` 作为门禁 step，任一失败即 job 失败。

---

## 9. CI Run ID / Commit SHA / Artifact ID

- Commit SHA（修复后）：`c95ff094300e6e629886edfa6a3a61161dfa4351`
- CI Run ID（CI 工作流，build/vet/test/全仓 -race/容器冒烟）：`42079445701`（success）
- CI Run ID（R0 E2E 工作流，A/D 门禁/C 报告型/上传前扫描）：`42079445817`（success）
- Artifact ID：`13441565324`；名称：`r0-e2e-evidence-4cb41db8d3a63f2e589ae72a920babadecaeebb9`（size 3647442 B，retention 30 天）
- 说明：CI 针对最终提交运行（PR #3 的 `pull_request` 事件，head = c95ff09）；R0 E2E run `42079445817` 结论 success，已产出并上传 artifact。

---

## 10. 最新 Artifact 下载后文件清单与独立扫描

- 下载 Artifact（id `13441565324`，101 个文件：A/D/C 三套件 PNG 截图 + report.json/md + fixtures.json + SHA256SUMS.txt + 各类 diagnostics / observations JSON）后，对全部文件用当前扫描器独立复扫：
  - 文件总数 = 101；内容已全量扫描 = 101；**跳过内容扫描 = 0**；二进制文件 = 80；符号链接 = 0
  - 编码解码：utf8可靠 = 21；utf16可靠 = 0；无法可靠解码(已阻断) = 0；可疑二进制(魔数不符) = 0
  - 命中文件/类型组合 = 0；命中总数 = 0；结论：**CLEAN（rc=0）**
- 目的达成：即便 CI 产出的 artifact 本身，经过与上传闸门同一把扫描器独立复扫，也不含可绕过凭据（**0 命中、0 跳过**），杜绝「CI 自身落盘即带凭据」的二次风险。

---

## 11. 未解决 Known Issues / 边界

- 单字节 NUL 注入若落在 token 值中间、且文件其余内容为可靠 UTF-8，依赖 NUL 复扫（去除 NUL 后复扫）捕获；K1-03 样例已实证命中。更极端「每字节随机错位的 NUL」形态未单独造样，当前设计按 fail-closed（无法可靠解码即 BLOCK）兜底。
- 已知二进制文件（PNG/JPEG/PDF 等）不进入 UNDECODABLE 阻断分支（按设计：魔数 + 内容双校验），但其内嵌凭据仍由 latin1 / UTF-16 规则拦截（见 case-09 / case-17）。
- 本轮不处理（任务单明确）：OBS-ACCT-VIS-390（390px 账户名不可见，仍 FAIL 待 Owner）、13 条历史控件 ID 映射、RV-05 旧 SHA 公开历史、仓库 Private 决策。
- 不自行合并 main；不宣布 R0 通过；可复现绕过已如实封死并标记，未改名规避。

---

## 回填记录（CI 完成后追加）

- CI Run ID：`42079445701`（CI）/ `42079445817`（R0 E2E）
- Commit SHA：`c95ff094300e6e629886edfa6a3a61161dfa4351`
- Artifact ID：`13441565324`（name `r0-e2e-evidence-4cb41db8d3a63f2e589ae72a920babadecaeebb9`）
- Artifact 下载清单与独立扫描结论：101 文件全量扫描、0 跳过、0 命中、CLEAN（rc=0）
