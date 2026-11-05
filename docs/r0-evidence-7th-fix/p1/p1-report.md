# FastLedger R0 第六轮整改 · P1 安全整改交付报告

- 分支：`fix/r0-sixth-runtime-defects`（未切换）
- HEAD：`8082ca85d4f679ed7698fc51009bc6a214c9ac0c`（未 commit、未 push、未合并 main）
- 勘误（HEAD 时点口径）：本报告写于 P1 提交前，上一行 HEAD 为**撰写时基线**；P1 已由
  team-lead 提交为 `0c62f5017c8b616f6d0c5079c55a9b4a13d1f68f`，交付以该 commit 为准。
  QA Round 2 后最终 commit 见交付回执。本行仅追加说明，不覆盖上方历史结论。
- 范围：**仅 P1 安全整改**；P2 未触碰
- 证据目录：`docs/r0-evidence-7th-fix/p1/`
- 结论：**整改完成，本地自证全部通过，扫描失败时上传必不执行**

---

## 一、问题与根因

Public 仓库的 `.github/workflows/r0-e2e.yml` 原有「上传证据」step 使用
`actions/upload-artifact@v4`，`path: artifacts/r0-e2e/**`，把 A/D/C 套件生成的全部文件
（含每个场景的 Playwright 网络记录归档）原样上传，retention 30 天。第三方复核在本轮
公开 artifact 的 12 个 `trace.zip` 中发现 10 个含临时实例真实鉴权头明文；team-lead 独立
递归复核命中 95 处凭据、18 个文件。

**关键陷阱**：原上传 step 是 `if: always()`。若只在它前面加一个「扫描失败即退出」的 step，
`always()` 会让上传照样执行，整改完全无效。因此上传条件必须绑定到扫描 step 的 outcome。

team-lead 已通过 GitHub API 删除本轮 4 个含凭据 artifact（HTTP 204）；剩余 4 个更早 run 的
历史产物不在本轮删除范围，本次**未动**。

---

## 二、改动文件清单

| 类型 | 路径 | 说明 |
|---|---|---|
| 修改 | `.github/workflows/r0-e2e.yml` | 顶部第 3 条约束注释改写；新增「准备上传证据（白名单）」「上传前安全扫描（失败即阻断）」两个 step；上传 step 改名 / 改 path / 改 if / 改 `if-no-files-found` |
| 新增 | `tools/r0/e2e/scripts/prepare-upload.js` | 零第三方依赖白名单收集器，Node fs 实现，本地与 Linux CI 通用 |
| 新增 | `tools/r0/e2e/scripts/scan-artifacts.js` | 零第三方依赖失败关闭扫描器，退出码 0/2/1 |
| 新增 | `tools/r0/e2e/test-scan-artifacts.js` | `node --test` 负向合成测试，10 组 |
| 新增 | `docs/r0-evidence-7th-fix/p1/workflow-structure-check.js` | 可复现的 workflow 结构校验（12 项） |
| 新增 | `docs/r0-evidence-7th-fix/p1/*` | 日志、退出码汇总、diff/status、本文档 |

**未改动（逐条核对）**：包 A / 包 D 门禁 / 包 C 报告型 step 的语义、C 的
`continue-on-error: true`、D 门禁无 `continue-on-error`、汇总退出码 step 的全部既有内容、
`tools/r0/e2e/specs/c-controls.js`、`tools/r0/e2e/fixtures/controls.json`、
`tools/r0/e2e/run.js`、`server/web/dist/app.js`、`server/web/dist/style.css`、
PRD / WBS / DB / API、README / CHANGELOG、任何历史证据目录、228 条清单与测试编号。
`workflow-structure-check.js` 已把「A 是门禁」到汇总 step 结束的整段文本与 `git show HEAD`
版本做逐字比对，字节数 1490 = 1490，完全一致。

**其它 workflow 核查**：`.github/workflows/` 下仅有 `ci.yml` 与 `r0-e2e.yml`；
**已核查 ci.yml，无 upload-artifact 步骤**，不存在同类外发风险；全仓 `upload-artifact`
仅 `r0-e2e.yml` 一处，已纳入白名单 + 扫描。`ci.yml` 本次未改动。

---

## 三、白名单收集器 `prepare-upload.js`

- 默认源 `artifacts/r0-e2e/`，默认目标 `artifacts/r0-e2e-upload/`；支持 `--source` / `--dest`。
- 递归收集白名单：`*.png` / `*.md` / `*.json` / `*.log` / `*.txt`（大小写不敏感）。
- 保持原有相对目录结构，用 `fs.copyFileSync` 复制，不依赖 `cp --parents` 等平台命令。
- **每次运行先清空目标目录**，杜绝上一轮残留被再次上传；不跟随符号链接；
  校验源/目标不得相同或互相嵌套，目标不得是仓库根、`artifacts` 根或其上级（防误删）。
- 输出只有「数量 + 相对文件名 + 排除原因」，**不输出文件内容**。

排除规则（先判黑名单、再判白名单）：

| 类别 | 规则 |
|---|---|
| 压缩归档 | `*.zip`，其中文件名含 `trace` 的单独标注为 `TRACE_ARCHIVE` 类原因 |
| 网络原始记录 | `*.trace`、`*.network`、`*.har` |
| 服务原始日志 | basename 精确等于 `server.log` |
| 数据库 | `*.db`、`*.sqlite`、`*.sqlite3`、`*.db-wal`、`*.db-shm` |
| 敏感词文件名 | 文件名含 `token` / `secret` / `credential` 一律排除（**采用更严格口径：即使扩展名在白名单内也排除**） |
| 其它 | 任何非白名单扩展名、无扩展名文件、符号链接 |

退出码：0 成功 / 1 参数或环境错误。

---

## 四、扫描器 `scan-artifacts.js` 规则表

入参为待上传目录（默认 `artifacts/r0-e2e-upload`，支持位置参数与 `--dir`），递归遍历。
**输出只写「文件路径 + 命中类型 + 命中数量」与汇总计数，绝不打印匹配到的值。**

### A 类：文件名 / 扩展名黑名单（命中即退出 2）

| 类型 | 判定 |
|---|---|
| `TRACE_ARCHIVE` | 文件名含 `trace` 且扩展名为 `.zip` |
| `BLACKLIST_EXT` | `.zip` / `.trace` / `.network` / `.har` / `.db` / `.sqlite` / `.sqlite3` / `.db-wal` / `.db-shm` |
| `SERVER_LOG` | basename 精确等于 `server.log` |
| `SENSITIVE_FILENAME` | 文件名含 `token` / `secret` / `credential` |
| `SYMLINK_ENTRY` | 目录内出现符号链接（额外失败关闭加固，不跟随） |

> **版本说明**：以下 §四 规则表与 §七 自证记录的是**首轮实现**。QA Round 1 独立验证发现
> 失败开放、披露不足与假阳性等缺陷，Round 2 已修订；交付态规则、修复前后退出码对比与
> 残留风险以 `docs/r0-evidence-7th-fix/p1/round2/round2-report.md` 为准，
> **首轮「全部通过 / 已知边界」结论不得外推到交付态**。

### B 类：文本内容命中（首轮：仅对非二进制文件；Round 2：全部文件）

| 类型 | 判定对象 |
|---|---|
| `JWT_LITERAL` | JWT 三段式形态（以 `eyJ` 起始、三段点分隔、每段 8 字符以上） |
| `BEARER_TOKEN` | `Bearer` 关键字后跟 12 字符以上的令牌形态值 |
| `AUTH_HEADER` | 鉴权头关键字 + 赋值 + 8 字符以上非空值（兼容带引号的 JSON 字段形式） |
| `COOKIE_HEADER` | `Cookie` / `Set-Cookie` 关键字 + 赋值 + 8 字符以上非空值 |
| `EBK_JWT_SECRET_ASSIGN` | JWT 签名密钥环境变量赋值 |

### C 类：二进制与异常（首轮实现；Round 2 已修订）

- 首轮实现：`*.png` / `*.jpg` / `*.jpeg` / `*.gif` / `*.webp` / `*.bmp` / `*.ico` / `*.pdf`
  等按扩展名**整文件跳过内容扫描**；其余文件读取后若前 4096 字节含 NUL 也视为二进制跳过，
  且跳过时静默返回 0 CLEAN。A 类文件名检查对二进制文件照常执行（负向测试 case-10 验证）。
- **如实披露（QA 独立验证证实）**：这两条跳过分支构成失败开放漏洞。比「截图画面拍到令牌」
  更直接的路径是——**只要把纯文本凭据命名为 `shot.png`，文件内容本身就是文本，即可经
  prepare 收集、被 scan 判定 CLEAN 并端到端上传**（`qa-verify/e2e-05`）；白名单 `.log` 文件头
  插入 1 个 NUL 字节同样可让 5 条文本规则全部失效（`qa-verify/e2e-06`）。首轮 case-09 还把
  「跳过 → 0」固化为 PASS。以上均为**未修复前的首轮结论**，不得外推到交付态。
- **Round 2 修订**：取消一切内容跳过分支，所有文件（含二进制）读 Buffer 后以 `latin1` 做
  1:1 字节映射，逐条应用 5 条文本规则；二进制扩展名增加起始魔数校验，不符记
  `SUSPICIOUS_BINARY` 并阻断；输出统计改为「内容已全量扫描 / 跳过内容扫描=0」。
  详见 `p1/round2/round2-report.md`。
- 单个文件超过 32 MiB、目录不可读、参数错误：退出 1（同样阻断上传，因为上传 step 只认
  `outcome == 'success'`）。
- 空目录：退出 0 并打印 WARN；此时上传 step 的 `if-no-files-found: error` 仍会让 job 失败。

### 退出码

| 码 | 含义 | 上传是否执行 |
|---:|---|---|
| 0 | 干净 | 是 |
| 2 | 发现疑似凭据或黑名单文件 | **否** |
| 1 | 参数 / 环境错误（失败关闭） | **否** |

---

## 五、workflow 前后对比与 `if` 条件证明

| 项 | 整改前 | 整改后 |
|---|---|---|
| 上传 step 名 | 上传证据（日志/截图/**trace**/报告/校验和） | 上传证据（日志/截图/报告/校验和） |
| 前置 step | 无 | ① 准备上传证据（白名单，`id: artifact_prepare`）② 上传前安全扫描（失败即阻断，`id: artifact_scan`） |
| 上传 `path` | `artifacts/r0-e2e/**` | `artifacts/r0-e2e-upload/**` |
| 上传 `if` | `always()` | `always() && steps.artifact_scan.outcome == 'success'` |
| `if-no-files-found` | `warn` | `error` |
| workflow 注释中的 trace 字样 | 有（含「全部作为 artifact 上传」） | 0 处 |

三个 step 的条件链：

```yaml
- name: 准备上传证据（白名单）
  id: artifact_prepare
  if: ${{ always() }}
  run: node tools/r0/e2e/scripts/prepare-upload.js --source artifacts/r0-e2e --dest artifacts/r0-e2e-upload

- name: 上传前安全扫描（失败即阻断）
  id: artifact_scan
  if: ${{ always() && steps.artifact_prepare.outcome == 'success' }}
  run: node tools/r0/e2e/scripts/scan-artifacts.js artifacts/r0-e2e-upload

- name: 上传证据（日志/截图/报告/校验和）
  if: ${{ always() && steps.artifact_scan.outcome == 'success' }}
  uses: actions/upload-artifact@v4
```

**为什么扫描失败时上传必不执行**：上传 step 的 `if` 不再是无条件的 `always()`，而是
`always()` **且** 扫描 step 的 `outcome == 'success'`。扫描器只有退出 0 时 step 才是
`success`；退出 2（命中）或 1（环境错误）时 `outcome` 为 `failure`，`if` 求值为 false，
`uses: actions/upload-artifact@v4` 根本不会被调度。若 prepare 失败，扫描 step 的
`if` 前置条件不满足而被跳过，`outcome` 为 `skipped`（非 `success`），上传同样不执行。
即：**任何非 0 的扫描结果、以及任何 prepare 失败，都会让上传 step 消失**。

| 场景 | prepare outcome | scan outcome | 上传 `if` | 上传 |
|---|---|---|---|---|
| 全绿 | success | success | true | 是 |
| 扫描命中（退出 2） | success | failure | false | **否** |
| 扫描环境错误（退出 1） | success | failure | false | **否** |
| prepare 失败（退出 1） | failure | skipped | false | **否** |
| A 或 D 门禁失败 | success（`always`） | success（`always`） | true | 是（保留失败报告证据） |
| C 报告型 FAIL（`continue-on-error`） | success | success | true | 是 |

prepare 与 scan 都带 `always()`，是为了在 A/D 门禁失败时仍能留下报告证据，同时不牺牲
阻断能力；scan 额外要求 prepare 成功，避免扫描一个半成品目录时误判通过。
汇总退出码 step 位于两个新 step 之前，引用后置 step 的 outcome 只会得到空值，故按
「不得改动既有内容」约束保持原样，未追加任何行。

---

## 六、负向合成测试逐条退出码

命令：`node --test tools/r0/e2e/test-scan-artifacts.js` → 退出码 **0**（10/10 通过）。
完整 TAP 日志见 `negative-tests.log`。所有合成样例由**运行时字符串拼接**生成并统一带
`SYNTHETIC` / `FAKE` 标记，样例文件写入系统临时目录并自动清理，**仓库内不残留可被
误判为真实凭据的字面量**。

| 组 | 用例 | 期望 | **实际退出码** | 结果 |
|---:|---|---:|---:|---|
| 1 | 干净文件集（md/json/log/txt/png） | 0 | **0** | PASS |
| 2 | 合成 JWT 三段式 | 2 | **2** | PASS |
| 3 | 名为 `trace.zip` 的归档 | 2 | **2** | PASS |
| 4 | Bearer 值与鉴权头赋值 | 2 | **2** | PASS |
| 5 | Cookie / Set-Cookie 赋值 | 2 | **2** | PASS |
| 6 | `server.log` 服务原始日志（附加） | 2 | **2** | PASS |
| 7 | JWT 签名密钥赋值（附加） | 2 | **2** | PASS |
| 8 | 目录不存在（参数/环境错误，附加） | 1 | **1** | PASS |
| 9 | 二进制内容按设计跳过扫描（附加） | 0 | **0** | PASS |
| 10 | 二进制文件名含敏感词仍被阻断（附加） | 2 | **2** | PASS |

---

## 七、本地自证汇总

全部日志、退出码保存在 `p1/exit-codes.txt` 与各 `.log`，每个日志末尾也写有退出码。

| 命令 | 退出码 | 日志 |
|---|---:|---|
| `node --test tools/r0/e2e/test-scan-artifacts.js` | 0 | `negative-tests.log` |
| `prepare-upload.js --source artifacts/r0-qa-verify --dest artifacts/r0-e2e-upload` | 0 | `prepare-qa-verify.log` |
| `scan-artifacts.js artifacts/r0-e2e-upload`（真实产物） | **0** | `scan-clean-qa-verify.log` |
| 手工放入运行时拼接的合成凭据样例后重扫 | **2** | `scan-dirty-synthetic.log` |
| 移除合成样例后重扫 | 0 | `scan-clean-after-remove.log` |
| `prepare-upload.js`（workflow 默认源 `artifacts/r0-e2e`） | 0 | `prepare-default.log` |
| `scan-artifacts.js`（默认配置） | **0** | `scan-clean-default.log` |
| `node tools/r0/scan-credentials.js --strict` | **0** | `strict-scan.log` |
| `node docs/r0-evidence-7th-fix/p1/workflow-structure-check.js` | 0 | `workflow-structure-check.txt` |

真实产物白名单结果：

- `artifacts/r0-qa-verify`：扫描 111 个文件 → 收集 99、排除 12（全部是网络记录归档）；
  扫描器 `文件总数=99 文本已扫描=19 二进制跳过=80 命中总数=0 结论：CLEAN`。
- workflow 默认源 `artifacts/r0-e2e`：扫描 149 个文件 → 收集 113、排除 36；
  扫描器 `文件总数=113 文本已扫描=42 二进制跳过=71 命中总数=0 结论：CLEAN`。
- 合成样例注入后：`[HIT] p1-synthetic-sample.txt | JWT_LITERAL | 1`，
  `结论：BLOCKED`，退出 2。日志中**只有文件路径、命中类型、命中数量，无任何匹配值**。
- `tools/r0/scan-credentials.js --strict`：**真实凭证候选命中 0**，退出 0；
  新增的 4 个 JS 文件与 `p1/` 目录在报告中均无命中条目。
- `workflow-structure-check.js`：12 项全 PASS，含「A / D / C / 汇总退出码四个既有 step
  与 HEAD 逐字一致」与「workflow 全文无 trace 字样」。

### 确认无 trace 入库（`no-trace-check.txt`）

| 核查项 | 结果 |
|---|---|
| 上传目录内黑名单扩展名 / `server.log` 文件数 | **0** |
| `git ls-files artifacts/` 文件数 | **0** |
| `git ls-files` 中含 `trace.zip` 的条目数 | **0** |
| `git check-ignore artifacts/r0-e2e-upload` | 命中（`.gitignore` 已忽略 `artifacts/`） |

本轮未把任何 `trace.zip`、原始网络日志或数据库文件复制进 Git 仓库；
`artifacts/r0-e2e-upload/` 只是本地 / CI 运行时目录，不入库。合成凭据样例位于
被忽略的 `artifacts/` 下且在自证结束前已删除。报告与日志中未写出任何 JWT 原文。

---

## 八、复现命令

```bash
node tools/r0/e2e/scripts/prepare-upload.js --source artifacts/r0-qa-verify --dest artifacts/r0-e2e-upload
node tools/r0/e2e/scripts/scan-artifacts.js artifacts/r0-e2e-upload
node --test tools/r0/e2e/test-scan-artifacts.js
node tools/r0/scan-credentials.js --strict
node docs/r0-evidence-7th-fix/p1/workflow-structure-check.js
```

---

## 九、未解决项与边界

1. **历史 artifact**：更早 run 的 4 个公开 artifact 不在本轮删除范围，按 team-lead 指令未动，
   后续处置由 Owner 决定。本整改只阻断「本轮起的新上传」。
2. **凭据轮换**：泄露的是每次新生成的一次性临时实例密钥、实例已销毁；是否还需显式
   轮换/失效确认不在本阶段代码整改范围，建议 Owner 确认。本阶段未改任何业务代码。
3. **未做真机 CI 验证**：按指令不 push、不切分支，因此未触发 GitHub Actions run。
   workflow 逻辑由 12 项结构静态校验 + 本地完整复刻「prepare → scan → 上传条件」执行链自证；
   建议合并前由 Owner 在受控 run 上确认一次。
4. **ci.yml 内联合成密钥**：`ci.yml` 中容器冒烟用的 JWT 签名密钥是 CI 合成值，且该
   workflow 不上传任何 artifact，不构成本次泄露面；本阶段未改动。若需统一改为 secrets，
   建议后续单独评估（不属 P1）。
5. **二进制内容边界（首轮为漏洞，Round 2 已堵住）**：首轮把 PNG/JPG 等按扩展名跳过内容
   扫描，只做文件名黑名单检查，报告当时仅以「截图画面拍到令牌」描述，**弱化了更直接的
   「文件内容即纯文本凭据」路径**。QA 独立实测证明纯文本凭据伪装成 `.png`、以及 NUL 前缀
   白名单 `.log`，均可端到端放行。Round 2 已改为全量 latin1 文本扫描 + 二进制魔数校验，
   两条路径均被阻断（修复前后退出码对比见 `p1/round2/round2-report.md`）。
   **残留边界**：扫描器仍是基于已知形态规则的轻量闸门，非通用 DLP；不含任何关键字的
   凭据形态不拦，真实图片画面内的文字也不属于文本扫描能力范围。不得把「扫描通过」
   外推为「内容绝对无凭据」，更不得把绿色 CI 解读为「证据已安全上传」。
6. **更严格口径的副作用**：文件名含 `token` / `secret` / `credential` 的白名单文件也会被
   prepare 排除（比任务书「非白名单文件」口径更严）。若将来确有此类合法证据文件，
   需人工改口径并重跑自证；scan 作为第二道防线会对它们直接返回 2。
7. **新增上传通道**：若将来新增其它 workflow 或上传方式，必须同步接入
   `prepare-upload.js` + `scan-artifacts.js`。建议把该约束写入贡献规范，但本阶段禁止改
   README / CHANGELOG，故未落地为文档。
8. **汇总退出码 step 未追加扫描 outcome（Round 2 已用独立 step 补齐可观测性）**：该 step
   位于扫描 step 之前，引用后置 step 的 outcome 只会得到空值，且任务书要求不得改动其既有
   内容，故一字未改。Round 2 改为在上传 step 之后**新增独立 step**「汇总证据上传安全结论」，
   把 prepare / scan / upload 三个 outcome 写入 `$GITHUB_STEP_SUMMARY`，并在上传被跳过时
   显式提示「请勿把绿色 CI 解读为证据已安全上传」。

---

## 十、遵守的禁止项

未 commit、未 push、未切分支、未合并 main；未改 228 条清单与测试编号；未改业务代码；
未启动 R1–R7 / PWA；未改 PRD / WBS / DB / API / README / CHANGELOG / 历史证据目录；
未在任何报告、日志、脚本中写入 JWT 原文或真实凭据字面量；未把 `trace.zip` 或原始网络
日志复制进 Git 仓库。**P2 完全未触碰。**
