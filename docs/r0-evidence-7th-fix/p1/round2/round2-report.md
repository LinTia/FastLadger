# FastLedger R0 · P1 Round 2 修复与自证报告

- 触发：QA（software-qa-engineer-2）Round 1 独立验证报告 `p1/qa-verify/qa-p1-report.md` §4 / §9，路由 Engineer
- 指令：team-lead「P1 失败开放漏洞修复规格（QA Round 1）」+「裁决全量修并锁定零跳过方案」
- 分支：`fix/r0-sixth-runtime-defects`（未切换）
- 基线 HEAD：`0c62f5017c8b616f6d0c5079c55a9b4a13d1f68f`（P1 首轮已由 team-lead 提交）
- 证据目录：`docs/r0-evidence-7th-fix/p1/round2/`（按裁决使用，未另建 `p1-fix2/`）
- 状态：**N1 / N2 / N3 / N4 / N5 全部修复；零跳过口径锁定；17 组负向测试 + 场景 A–F 端到端自证全部通过；未 commit、未 push、未合并 main**
- 结论口径：本报告只陈述实测结果。**扫描器仍是基于已知形态规则的轻量闸门，非通用 DLP，不得外推为「任何凭据都能拦」；绿色 CI 也不等于「证据已安全上传」。**

---

## 一、改动文件清单（本轮仅这些，P2 文件一个未碰）

| 类型 | 路径 | 说明 |
|---|---|---|
| M | `tools/r0/e2e/scripts/scan-artifacts.js` | 取消一切内容跳过分支；latin1 全量文本扫描；魔数仅作额外命中条件；鉴权/Cookie 值形态判定；统计口径改实 |
| M | `tools/r0/e2e/test-scan-artifacts.js` | case-09 改期望 2；新增 7 组回归（含 case-17 零跳过证明）；共 **17 组**，逐组打印实际退出码并断言输出不含匹配值 |
| M | `.github/workflows/r0-e2e.yml` | 上传 step 加 `id: artifact_upload`；其后新增独立 step「汇总证据上传安全结论」 |
| M | `docs/r0-evidence-7th-fix/p1/p1-report.md` | 追加 HEAD 时点勘误；§四 加版本说明；如实改写 C 类与 §九 第 5 / 8 条 |
| M | `docs/r0-evidence-7th-fix/p1/workflow-structure-check.js` | 新增 5 条 N3 可观测性断言（共 17 项） |
| A | `docs/r0-evidence-7th-fix/p1/round2/*` | 本目录：`round2-verify.js`（场景 A–F）、全部日志、退出码、diff、status、本报告 |

**`tools/r0/e2e/scripts/prepare-upload.js` 本轮未改动**：按 team-lead 裁决 B 采纳的理由，prepare 保持纯文件名白名单、不读内容，魔数只在 `scan-artifacts.js` 单点实现，避免两处规则漂移。端到端复刻证明伪 `.png` / NUL `.log` / 真实 PNG 内嵌文本 **仍会被 prepare 收集**（未被静默丢弃），随后由 scan 明确阻断，形成可审计的两步链路。

**P2 边界**：采集 `status.txt` 时工作树中同时存在另一名工程师的 P2 改动
（`tools/r0/e2e/run.js`、`tools/r0/e2e/specs/c-controls.js`、`tools/r0/e2e/README.md`、
`docs/r0-evidence-7th-fix/p2/`）以及 `docs/r0-evidence-7th-fix/P1-artifact-credential-incident.md`。
这些**均非本轮改动**，我未读取修改、未 `git add`、未 commit。`git diff` 严格按指令限定为 4 个 P1 文件。
team-lead 已明确禁止 P2 工程师改 `r0-e2e.yml`，本轮 workflow 改动无并行冲突。

---

## 二、零跳过口径锁定与 team-lead 裁决对照

### 2.1 裁决锁定的方案与理由（原文要点，按要求写入报告）

team-lead 明确否决「`TEXT_EXT` 强制文本；`BINARY_EXT` 校验魔数、**符合就按设计跳过内容扫描**」，
要求：**取消一切「跳过内容扫描」分支，所有文件一律读 Buffer 用 `buf.toString('latin1')` 逐条跑
5 条 `TEXT_RULES`，零跳过**。魔数校验**不是放行的通行证**，而是额外的一道命中条件：
魔数不符 → 记 `SUSPICIOUS_BINARY` 并退出 2；魔数符合也照样过文本规则
（真 PNG 内容里若含 JWT 字符串仍应阻断）。三条理由：

1. QA 与第三方复核盯的就是「静默跳过 = 失败开放」，**保留任何跳过分支都会被再次质疑**；
2. 待上传目录经 prepare 白名单后实际只有 `png/jpg/...` 与 `json/md/log/txt` 两类，
   **全量扫描成本极低**；
3. **保守误阻断远优于放行**，符合失败关闭。

`looksBinary()` 不得再用于跳过；`binarySkipped` 改为如实口径；输出里不得出现「跳过 N 个文件」
却返回 CLEAN；`MAX_TEXT_BYTES` 上限保持，超限抛错 → `EXIT_ERROR=1`（同样阻断）。

### 2.2 交付态实现与裁决逐条核对（场景 F 静态核验 + 场景 E 行为证明）

| 裁决要求 | 交付态实现 | 证据 |
|---|---|---|
| 取消一切跳过分支 | 删除 `looksBinary()` 与全部 `binarySkipped` / 主循环 `continue`；源码不含这两个标识符，主扫描循环内无任何 `continue` | `e2e-verify.log` 场景 F |
| 所有文件 latin1 全量跑 5 条规则 | 主循环 `const text = buf.toString('latin1')` 后逐条 `countMatches(text, rule)` | 场景 F；`scan-artifacts.js` L331 |
| 魔数不是放行通行证，是**额外命中条件** | `binarySignatureMismatch()` 只调用 `addFinding(rel, 'SUSPICIOUS_BINARY', 1)`；之后**无 continue/return**，继续走文本规则 | 场景 E（魔数符合仍退出 2）、场景 F |
| 统计口径如实 | `内容已全量扫描=N 其中二进制文件=M 跳过内容扫描=0`，另加一行「无任何内容跳过分支」 | `e2e-verify.log` 场景 A、场景 D |
| `MAX_TEXT_BYTES` 超限退出 1 | `stat.size > MAX_TEXT_BYTES` → `throw new Error(...)` → `EXIT_ERROR` | 场景 F |
| prepare 保持纯文件名白名单（B 采纳理由） | `prepare-upload.js` 未改；魔数只在 scan 层单点实现 | §一；`diff.txt` 中该文件无 diff |
| N3 独立 summary step | 已加 `id: artifact_upload` + 上传后独立 step；既有汇总 step 一字未改 | `workflow-structure-check.log` 17/17 |
| N4 时点注释、不覆盖历史结论 | 已追加；§四 C 类与 §九 边界如实改写 | `p1-report.md`；`other-changes.diff` |
| N6/N7 与第三方提示 | 已写入 §六 第 1 / 6 / 7 条 | §六 |
| 证据目录用 `p1/round2/` | 使用，未建 `p1-fix2/` | 本目录 |

> 场景 F 的「无任何 continue 语句」判定会先剥离行注释，避免把说明文字里的 `continue`
> 单词误判为跳过语句；核验对象是主扫描循环（`for (const abs of tree.files)` 到符号链接循环之前）
> 的实际代码，而非注释文本。

> 命名说明：裁决消息中的 `BINARY_SIGNATURE_MISMATCH` 与正式修复指令中的 `SUSPICIOUS_BINARY`
> 指同一道附加防线；本实现按正式修复指令采用 `SUSPICIOUS_BINARY`，语义一致。

### 2.3 用例编号对照（两份指令措辞不同，三条语义用例均已齐备）

正式修复指令把 **case-09** 指定为「文本伪装 `.png` → 2」，裁决消息把 **case-09** 指定为
「真实 PNG 魔数 + 干净内容 → 0」。为同时满足两份指令且不破坏已通过的用例稳定性，
采用如下编排，并**补齐裁决中专门证明零跳过的那条用例**：

| 裁决要求的用例 | 本套件编号 | **实际退出码** |
|---|---|---:|
| 真实 PNG 魔数 + 干净二进制内容 → 0 | case-13 | **0** |
| **真实 PNG 魔数 + 内容里塞合成 JWT → 2（证明无跳过分支）** | **case-17（本轮补齐）** | **2** |
| 纯文本凭据命名 `shot.png`（无魔数）→ 2 | case-09 | **2** |
| 白名单 `.log` 头插 1 个 NUL + 合成 JWT → 2 | case-11 | **2** |
| 合法业务字段（鉴权普通值 / Cookie 普通值）→ 0 | case-12 | **0** |
| 鉴权头携带 Bearer + 合成 JWT → 2 | case-14 | **2** |
| Cookie 携带长随机会话值 → 2 | case-15 | **2** |
| `trace.zip` / 符号链接 / 目录不存在保持 | case-03 / case-16 / case-08 | **2 / 2 / 1** |

case-17 额外断言「输出**不含** `SUSPICIOUS_BINARY`」，即魔数校验本身是通过的，
退出 2 **完全来自文本规则**，从而硬证明「魔数符合也照样过文本规则、零跳过分支」。

---

## 三、N1–N5 逐条修复说明

### N1（高）· 白名单文本 NUL 前缀 fail-open —— 已修

- 原因：`looksBinary()` 在头 4096 字节见 NUL 即把整文件判二进制，主循环 `binarySkipped += 1; continue;`
  静默跳过 5 条文本规则并返回 0 CLEAN。静默跳过 = 失败开放，与「失败关闭」矛盾。
- 修复：**删除 `looksBinary()` 函数与全部 `binarySkipped` / `continue` 跳过分支**（口径理由见 §二 2.1）。
  每个文件读 Buffer 后用 `buf.toString('latin1')` 做字节到码位 0–255 的 1:1 映射，再逐条应用
  5 条 `TEXT_RULES`。NUL 只是码位 0 的普通字符，不会中断正则匹配，ASCII 凭据串在文件任意
  字节位置都能命中。
- `MAX_TEXT_BYTES` 32 MiB 上限保留：超限仍抛错 → 退出 1（同样阻断上传，上传 step 只认 `success`）。
- 输出统计改为如实口径：`内容已全量扫描=N 其中二进制文件=M 跳过内容扫描=0`，并新增一行
  `扫描口径：全部文件以 latin1 逐字节映射后应用 5 条文本规则，无任何内容跳过分支`。
  不再出现「跳过 N 个文件」却返回 CLEAN 的情形。

### N2（中）· 纯文本凭据伪装 `.png` 端到端放行 —— 已修

- 原因：`BINARY_EXT` 分支直接 `continue`，`.png` 内容从不读取；首轮 case-09 还把「跳过 → 0」固化为 PASS。
- 修复：删除该 `continue`。新增 `BINARY_SIGNATURES` 魔数附加防线（PNG 8 字节
  `89 50 4E 47 0D 0A 1A 0A`、JPEG `FF D8 FF`、GIF `GIF8`、PDF `%PDF`、BMP `BM`、
  ICO `00 00 01 00`、GZIP `1F 8B`）。二进制扩展名文件魔数不符即记 `SUSPICIOUS_BINARY` 并退出 2；
  **魔数符合不放行、继续过 5 条文本规则**（场景 E / case-17 硬证明）。
- 魔数校验与全量文本扫描是**两道独立拦截**：无魔数的伪 `.png` 同时命中 `JWT_LITERAL` 与
  `SUSPICIOUS_BINARY`；有合法魔数但内嵌凭据的 `.png` 只命中 `JWT_LITERAL`，同样退出 2。
- 误伤核查：本机 `artifacts/r0-qa-verify` 下 80 个真实 PNG 魔数 **80/80 正确**；两条干净真实产物链路
  扫描均为 0，未出现 `SUSPICIOUS_BINARY`。
- case-09 由「期望 0」改为「期望 2」，断言输出含 `JWT_LITERAL` 或 `SUSPICIOUS_BINARY`。

### N3（中）· CI summary 不含新 step outcome —— 已修（既有 step 一字未改）

- 给上传 step 加 `id: artifact_upload`。
- 在**上传 step 之后**新增独立 step「汇总证据上传安全结论」，`id: artifact_upload_summary`、
  `if: ${{ always() }}`，把 `artifact_prepare` / `artifact_scan` / `artifact_upload` 三个 outcome、
  「扫描通过且允许上传」布尔值写入 `$GITHUB_STEP_SUMMARY`；当上传 step 的 outcome 不是 success 时，
  追加显式提示「上传被跳过或未成功：本轮无公开 artifact 产出，请勿把绿色 CI 解读为证据已安全上传」。
- **既有「汇总退出码」step 内容一字未改**（结构校验按内容锚点与 HEAD 逐字比对，1490=1490 字节一致）；
  A / D / C 门禁语义、C 的 `continue-on-error: true`、D 无 `continue-on-error` 全部不动。
- 结构校验脚本新增 5 条断言，17 项全 PASS。

### N4（低）· 报告 HEAD 不一致 / 边界披露弱化 —— 已修

- `p1/p1-report.md` 顶部**追加**一行「HEAD 时点口径勘误」，注明撰写于提交前、
  撰写时 HEAD 为 `8082ca85…`、P1 交付 commit 为 `0c62f501…`、QA Round 2 后最终 commit 见交付回执；
  **不覆盖上方历史结论**。
- §四 增加醒目「版本说明」：规则表与 §七 自证记录的是首轮实现，交付态以本报告为准，首轮
  「全部通过 / 已知边界」结论不得外推。
- §四 C 类改写：如实写明首轮两条跳过分支构成失败开放漏洞，明确披露
  **「把纯文本凭据命名为 `shot.png`，文件内容本身就是文本，即可经 prepare 收集、被 scan 判定 CLEAN
  并端到端上传」** 这一比「截图画面拍到令牌」更直接、无需图像隐写的路径，以及 NUL 前缀白名单文本路径；
  并注明 Round 2 修订方式。
- §九 第 5 条由「已知边界，非回归」改写为「首轮为漏洞，Round 2 已堵住」，同时如实保留残留边界。
- §九 第 8 条补记 Round 2 用独立 summary step 补齐可观测性的做法。

### N5（低）· 合法业务字段误阻断 —— 已修

- `AUTH_HEADER` / `COOKIE_HEADER` 正则改为带捕获组，命中后由 `credentialLikeValue(value)` 做形态判定：
  值以 `eyJ` / `Bearer` 开头即计；否则要求长度 ≥16，且含 `.` / `=` / `_` / `+` / `/`，
  或为高熵 base64/token 形态（字符集 `[A-Za-z0-9+/=_-]` 且含数字，或同时含大小写）。
  **纯字母 + 连字符的自然语言短语不计**（如 `role-based-access` 长度 18 但无数字/大写，
  `same-site-lax` 长度 13）。
- 该函数只返回布尔，**绝不把值写入任何输出**；输出契约仍为「路径 | 类型 | 数量」。
- 硬约束验证：case-14「鉴权头携带 Bearer + 合成 JWT」实测退出 **2**，
  同时命中 `JWT_LITERAL` 与 `BEARER_TOKEN`（两条独立规则兜底，不受收紧影响）。
- case-12「合法业务字段」实测退出 **0**；case-15「Cookie 长会话值」实测退出 **2**。

---

## 四、修复前后退出码对比

「修复前」取自 QA Round 1 实测日志 `p1/qa-verify/` 与首轮 `p1/negative-tests.log`，
**非本轮重跑**；「修复后」取自本目录 `e2e-verify.log` / `negative-tests.log` / `workflow-structure-check.log`。

| # | 场景 | 修复前（QA/首轮） | 修复后（Round 2 实测） | 修复后证据 |
|---:|---|---:|---:|---|
| 1 | NUL 前缀白名单 `.log` 内藏合成 JWT | **0**（CLEAN，端到端可上传） | **2** | `e2e-verify.log` 场景 A；case-11 |
| 2 | 纯文本凭据伪装 `.png`（无魔数） | **0**（CLEAN，端到端可上传） | **2** | 场景 B；case-09 |
| 3 | 合法业务字段 `report.json` | **2**（误阻断） | **0** | 场景 C；case-12 |
| 4 | 真实 PNG 魔数 + 干净二进制内容 | 0（但因跳过而**未真正扫描**） | **0（已全量扫描且魔数通过）** | case-13；场景 D |
| 5 | **真实 PNG 魔数 + 内容内嵌合成 JWT** | **0**（因跳过而放行） | **2（魔数符合仍被文本规则阻断）** | **场景 E；case-17** |
| 6 | 干净真实产物链路 prepare → scan | 0 → 0 | **0 → 0** | 场景 D（两个源目录） |
| 7 | `trace.zip` 黑名单 | 2 | **2** | case-03 |
| 8 | 符号链接 | 2（QA 实测） | **2** | case-16 |
| 9 | 鉴权头 Bearer + 合成 JWT（硬约束） | 2 | **2** | case-14 |
| 10 | Cookie 长会话值 | 2 | **2** | case-15 |
| 11 | 目录不存在（参数/环境错误） | 1 | **1** | case-08 |

绕过场景的端到端日志均明确记录：
`结论：该文件会被阻断、不会上传（scan=2 → artifact_scan.outcome=failure → 上传 step 被跳过）`；
场景 E 另记录「魔数符合也照样过文本规则，零跳过分支生效」。

---

## 五、本地自证（全部落盘本目录）

| 命令 | 退出码 | 日志 |
|---|---:|---|
| `node --test tools/r0/e2e/test-scan-artifacts.js` | **0** | `negative-tests.log`（17/17 pass） |
| `node docs/r0-evidence-7th-fix/p1/round2/round2-verify.js` | **0** | `e2e-verify.log`（场景 A–F，PASS，失败项=0） |
| `node docs/r0-evidence-7th-fix/p1/workflow-structure-check.js` | **0** | `workflow-structure-check.log`（17/17 PASS） |
| `node tools/r0/scan-credentials.js --strict` | **0** | `strict-scan.log`（真实凭证候选命中 0） |

- `git diff -- tools/r0/e2e/scripts/scan-artifacts.js tools/r0/e2e/scripts/prepare-upload.js tools/r0/e2e/test-scan-artifacts.js .github/workflows/r0-e2e.yml` → `diff.txt`
- `git status --porcelain` → `status.txt`
- 附加的文档/脚本改动 → `other-changes.diff`
- 全部命令退出码汇总 → `exit-codes.txt`

### 17 组负向合成测试实际退出码（逐组打印，见 `negative-tests.log`）

| 组 | 用例 | 期望 | **实际** | 结果 |
|---:|---|---:|---:|---|
| 1 | 干净文件集 | 0 | **0** | PASS |
| 2 | 合成 JWT 三段式 | 2 | **2** | PASS |
| 3 | `trace.zip` 归档 | 2 | **2** | PASS |
| 4 | Bearer 与鉴权头赋值 | 2 | **2** | PASS |
| 5 | Cookie / Set-Cookie 赋值 | 2 | **2** | PASS |
| 6 | `server.log` | 2 | **2** | PASS |
| 7 | JWT 签名密钥赋值 | 2 | **2** | PASS |
| 8 | 目录不存在 | 1 | **1** | PASS |
| 9 | **纯文本凭据伪装 `.png`（Round 2 修订）** | 2 | **2** | PASS |
| 10 | 二进制文件名含敏感词 | 2 | **2** | PASS |
| 11 | **NUL 前缀白名单 `.log` 藏 JWT（N1 回归）** | 2 | **2** | PASS |
| 12 | **合法业务字段（N5 假阳性回归）** | 0 | **0** | PASS |
| 13 | **真实 PNG 魔数 + 干净内容** | 0 | **0** | PASS |
| 14 | **鉴权头 Bearer + 合成 JWT（硬约束）** | 2 | **2** | PASS |
| 15 | **Cookie 长会话值** | 2 | **2** | PASS |
| 16 | **符号链接** | 2 | **2** | PASS |
| 17 | **真实 PNG 魔数 + 内容内嵌 JWT（零跳过证明）** | 2 | **2** | PASS |

所有合成值继续由**运行时字符串拼接**生成并带 `SYNTHETIC` / `FAKE` 标记，样例写入系统临时目录
（`os.tmpdir()`）并在结束时删除，不落入仓库；`runScanner()` 对每组断言扫描输出**不含匹配值本身**。
`scan-credentials.js --strict` 退出 0、真实凭证候选命中 0。

---

## 六、仍存在的残留风险（如实披露，不得外推）

1. **规则为已知形态白名单，非通用 DLP（对应 QA N6）**：`TEXT_RULES` 只覆盖 JWT 三段式、Bearer 值、
   鉴权头赋值、Cookie / Set-Cookie 赋值、JWT 签名密钥赋值。不含这些关键字/形态的凭据不拦。
   本轮核心风险（网络归档内的鉴权头 + JWT）由「排除 zip + 全量文本扫描」双重覆盖，
   但**不可外推为任何凭据都能拦**。
2. **值形态收紧的漏报权衡**：`credentialLikeValue()` 对长度 <16 且不含 `. = _ + /` 的短值不计入
   `AUTH_HEADER` / `COOKIE_HEADER`；但 `JWT_LITERAL`、`BEARER_TOKEN`、文件名黑名单三条独立规则仍生效
   （case-14 已实测退出 2）。该权衡需 Owner 知晓。
3. **魔数表覆盖有限**：仅覆盖 PNG / JPEG / GIF / PDF / BMP / ICO / GZIP 起始魔数。未列入的
   `BINARY_EXT` 扩展名不做魔数阻断，只依赖统一文本扫描——纯文本凭据仍会被文本规则命中，
   但不会有 `SUSPICIOUS_BINARY` 这条附加告警。
4. **图像画面内的文字不在能力范围**：真实 PNG/JPG 截图若拍到令牌（OCR 范畴），文本扫描无能为力；
   这与「文件内容本身就是纯文本凭据」不同，后者 Round 2 已由魔数 + 全量文本扫描双重堵住。
5. **未做真机 CI 验证**：按指令不 push、不切分支，未触发 GitHub Actions run。workflow 由 17 项
   结构静态校验 + 本地完整复刻「prepare → scan → 上传条件」执行链自证，建议合并前在受控 run 上
   确认一次（尤其新增 summary step 的 shell 条件在真实 runner 上的展开）。
6. **绿色 CI ≠ 证据已安全上传**（QA §6.4，必须随交付给出）：上传 step 的 `if` 是「允许不上传」，
   被跳过时不产生失败信号。第三方必须逐 step 核对 `artifact_prepare` / `artifact_scan` /
   `artifact_upload` 的 outcome，**到 Actions artifacts 列表确认 artifact 实际存在、名称与 commit SHA
   匹配，并下载后独立核验内容**；不得仅凭 CI 绿色、workflow 文件存在或本地日志为 0 推断安全上传。
7. **历史 artifact 未追溯（对应 QA N7）**：team-lead 已删本轮 4 个含凭据 artifact；更早 4 个历史产物
   按指令未动。本整改只阻断新上传，不消除历史已公开内容；凭据轮换/失效确认由 Owner 决定。
8. **`prepare-upload.js` 仍只按文件名判定（裁决 B 有意保留）**：伪 `.png` / NUL `.log` 会被收集，
   依赖 `scan-artifacts.js` 阻断。若将来有人绕过 scan 直接取用上传目录，白名单层不提供内容防护。

---

## 七、遵守的禁止项

未 commit、未 push、未切分支、未合并 main、未 `git add`；未改 P2 文件
（`run.js` / `c-controls.js` / `README.md` / `p2/`）、未改 `controls.json` / 228 条清单 /
`app.js` / `style.css` / Go 后端 / PRD / DB / API / P1 之外历史证据；未把任何 `trace.zip`、
原始网络日志或 JWT 原文写入仓库、报告与日志；未在报告中硬编码 PASS（所有结论均指向实测日志）；
未用 sleep 制造通过；未启动 R1–R7 / PWA。
