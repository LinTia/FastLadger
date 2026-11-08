# R0 第十三轮证据：A05 慢网 pending / disabled 断言缺口补齐

| 项 | 内容 |
|---|---|
| 日期 | 2026-11-08 |
| 面向 | 第十三次第三方独立复核（本目录不自行宣布 A05 关闭） |
| 依据 | `docs/FastLedger-R0-第十二次RV-14-T01独立复核.md` §6：「A05 仍有断言缺口……没有断言慢网 pending 时按钮 `disabled=true`。A01 的断言不能替代 A05 慢网场景」；该报告位于 `origin/main`、PR #3 分支树未包含，可用 `git show origin/main:docs/FastLedger-R0-第十二次RV-14-T01独立复核.md` 读取 |
| 工作基线 | PR #3 分支 `fix/r0-sixth-runtime-defects` @ `4318fd612c856218f6c903b7e3f3e5b5257cfa3a` |
| 改动范围 | **代码改动仅 1 个文件**：`tools/r0/e2e/specs/a-scenarios.js` 的 A05 块（+8 / -4）；本轮提交另新增证据目录 `docs/r0-evidence-13th-a05/` 共 11 个文件，**合计 12 个路径** |
| 结论性质 | 静态测试脚本整改 + 本地真实浏览器运行证据；**不代表 R0 通过** |

## 1. 缺口复现（修改前基线）

`tools/r0/e2e/specs/a-scenarios.js` A05 原实现在 CDP `Network.emulateNetworkConditions` 800ms 链路延迟下：

```js
await page.waitForTimeout(500);
const midText = (await page.locator('button', { hasText: /登\s*录|登录中/ }).first().innerText().catch(() => '')).trim();
...
status: elapsed >= 800 ? 'PASS' : 'FAIL',
detail: `总耗时=${elapsed}ms（含 800ms 链路延迟）；中途按钮文案=「${midText}」`,
```

判定只检查总耗时，未取回也未断言按钮 `disabled`。同文件 A01 用 `page.waitForFunction` 轮询抓 `{ text, disabled }` 并断言 `pending.disabled`，但那是显式 `route` 挂起 1500ms 的场景，不能代偿 A05 的 CDP 慢网场景。

基线真实输出（`baseline/console.log`、`baseline/a05-report.json`）：

```
PASS    A05 慢速网络：800ms 延迟下 pending 仍可见，最终成功
        总耗时=1151ms（含 800ms 链路延迟）；中途按钮文案=「登录中…」
A 汇总：{"PASS":11}
```

`detail` 中无任何 disabled 字段，与第十二次复核 finding 一致。

## 2. 最小整改方案（已实施）

只替换 A05 块内的采样与判定，不动其它 10 个场景、不新增截图：

1. 把 `waitForTimeout(500)` + 单点 `innerText()` 换成 `page.waitForFunction` 在 800ms 延迟窗口内轮询，抓到 pending 态并取回 `{ text, disabled }`（消除单点采样竞态；写法参照 A01，但在 A05 内独立断言）。
2. 保留 `elapsed >= 800`（慢网确实生效）与 `waitForFunction(fl_token)`（登录最终成功）。
3. 判定改为四者合取：

   `elapsed >= 800` ∧ `pending !== null` ∧ `/登录中/.test(pending.text)` ∧ **`pending.disabled === true`**

4. `detail` 增加 `pendingDisabled=...` 与 `登录后 token 已写入=...`，让复核者可直接从日志读取证据。
5. **不新增截图**：沿用既有 `A05/slow-network.png`，避免 Artifact 清单条目数变动扰动 RV-13。

完整改动见 `diff.txt`（提交前工作区相对 `4318fd6` 的快照，可用 `git show <本轮提交>` 对照）。

**明确未触碰**：`case-51` / `case-54` / `tools/r0/e2e/test-scan-artifacts.js` / `scripts/rv14-binary-evidence.js` / `scripts/scan-artifacts.js` / `.github/workflows/r0-e2e.yml` 上传条件 / 任何 `server/` 业务代码、`style.css`、`index.html`、PRD、API、DB、权限、同步契约。未处理 t021、`OBS-ACCT-VIS-390`、RV-05 等 Owner 待裁决事项。

## 3. 四段真实测试证据

运行环境：本机 Windows / Node v22.22.2 / playwright-core + Microsoft Edge 156.0.4314.58（headless），服务为临时 SQLite + 随机端口 + 独立 JWT 密钥，运行结束自动停止。命令在 `tools/r0/e2e` 下执行：

```
R0_BROWSER_CHANNEL=msedge R0_ARTIFACT_DIR=<对应目录> node run.js --suite a
```

| 段 | 断言状态 | exit | A 汇总 | A05 detail 关键字段 |
|---|---|---:|---|---|
| ① `baseline/` | 修改前原样 | 0 | 11 PASS | 无 `pendingDisabled` 字段 |
| ② `after/` | `=== true`（正向） | 0 | 11 PASS | `pendingDisabled=true` |
| ③ `negative/` | 临时反置为 `=== false` | **1** | 10 PASS / **1 FAIL** | `pendingDisabled=true`，A05 = FAIL |
| ④ `restored/` | 恢复为 `=== true` | 0 | 11 PASS | `pendingDisabled=true` |

每段目录含 `console.log`（完整原始输出）与 `a05-report.json`（脱敏结构化摘录：suite / summary / 11 项状态 / A05 完整结果；源 `report.json` 的 `server` / `env` 段含本机临时路径与随机端口，未纳入）。

四段 `console.log` 与 `artifacts/a05-*/console.log` 原始产物逐字节一致，未作脱敏改写。其中含 `127.0.0.1` 回环地址、随机端口与本机工作目录路径；无凭据值、无 RFC1918 内网地址。保留原始字节是为让复核者可逐字节比对。

### 断言有效性论证

负向对照把判定中的 `pending.disabled === true` 临时改为 `=== false` 后，A05 **由 PASS 变 FAIL**（A 汇总 10/1，退出码 1），但 `detail` 仍输出 `pendingDisabled=true` —— 说明 pending 态采样确实抓到了 `disabled=true`，结果翻转**只由断言引起**，不是采样丢失或偶然通过。随后脚本在 `finally` 中把文件逐字节恢复为正向版本，并以第 ④ 段复跑确认 11/11 PASS。恢复后 `git diff --name-only` 仍只有 A05 所在文件，且断言为 `=== true`。

日志已扫描 `password` / `secret` / `bearer` / `authorization` / `eyJ` / `EBK_JWT`，四段均为 0 命中，未入库任何凭据。

## 4. 验证边界

**已做（提交前）**：本地真实 Playwright + Edge 运行 A 套件四次（含负向对照）、JS 语法检查 `node --check`、改动 diff 范围核验、证据哈希清单、日志敏感串扫描。
**提交后另行核验（结果只写入交付回执，不在本目录追加文件）**：从 git 对象回读 A05 断言与证据目录，并按 LF 规范形式复算 `SHA256SUMS.txt`。本目录是提交前的证据快照，不承载该动作的结果，故不把提交后动作列为「已做」。

**未做**：未重跑 C 套件（228 条，仍为 164 PASS / 4 FAIL / 59 BLOCKED / 1 N/A）、未跑 D 套件、未在独立 clone 中由第三方复跑、未做真机 / 人工视觉验收、未做接口性能或安全测试、未执行任何业务代码或前端页面改动。

**因此**：本目录只证明 **A 套件 A05 在慢网场景已具备 pending/disabled 的真实断言并可通过本地运行**，不证明整套 R0 验收通过。**A 套件 11/11 与 CI job 全绿都不等于 R0 通过**；R0 仍 **BLOCK**。PR #3 保持 OPEN / 未合并；不授权 UI-DESIGN-02 代码实施、R1–R7 或 PWA。

**A05 缺口是否关闭由第十三次第三方独立复核判定，本轮不自行宣布。**

## 5. 哈希复现与行尾说明

`SHA256SUMS.txt` 基于 **LF 规范形式**（git blob / LF 工作树）计算，覆盖除自身外的 10 个文件。Windows 上 `core.autocrlf=true` 的 clone 会把文本文件检出为 CRLF，届时文本文件的哈希会不同——这是行尾规范化，不是内容篡改。逐字节复核请用 `git -c core.autocrlf=false clone`，或直接从 git 对象回读：

```
git show HEAD:docs/r0-evidence-13th-a05/SHA256SUMS.txt
git show HEAD:docs/r0-evidence-13th-a05/after/console.log | sha256sum
```

本仓库无 `.gitattributes`，该目录文件按默认 text 属性归一化存储。

## 6. 提交前只读复核记录的非阻断观察（本轮不整改）

1. 负向对照脚本 `artifacts/_a05_negative.py` 位于 `.gitignore` 忽略的 `artifacts/`，未入库；本目录的 `negative/console.log` 与 `negative/a05-report.json` 为可复核证据，脚本本体不作为交付物。
2. A01 / A05 中既有的 `waitForFunction(fn, { timeout })` 写法把 options 放进了 `arg` 槽位，属 PR #3 基线既有、本轮零改动；默认 30s 上限大于实际等待，不影响退出语义。本轮新增的轮询使用正确签名 `(fn, null, { timeout: 4000 })`。
3. 判定式中的 `/登录中/.test(pending.text)` 与轮询谓词重复，属防御性冗余；`!!pending` 与 `pending.disabled === true` 为真实求值，负向对照已证明结果可翻转，不构成永真断言。