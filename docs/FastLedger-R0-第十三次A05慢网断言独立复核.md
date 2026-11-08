# FastLedger R0 第十三次独立复核：A05 慢网 pending / disabled 断言

| 项 | 内容 |
|---|---|
| 日期 | 2026-11-08 |
| 复核对象 | PR #3：`fix/r0-sixth-runtime-defects` |
| 复核基线 | 第十二次复核 HEAD `4318fd612c856218f6c903b7e3f3e5b5257cfa3a` |
| 本轮 HEAD | `ad34df1c8854f792231ed5152b61e2e22d115e8b` |
| 比较结果 | ahead 2、behind 0；新增 2 个提交 |
| 结论 | **A05 慢网 pending / disabled 断言缺口：PASS（源码与已提交运行证据复核；未独立重跑）** |
| 总体状态 | **R0 仍 BLOCK；PR #3 OPEN / 未合并** |

## 1. 变更范围核对

比较 `4318fd6..ad34df1` 得到 12 个变更路径：

- 代码仅 1 个文件：`tools/r0/e2e/specs/a-scenarios.js`，A05 块 +8 / -4。
- 证据目录 `docs/r0-evidence-13th-a05/` 共 11 个文件：README、SHA256SUMS、baseline / after / negative / restored 四组 JSON 与 console 日志，以及 diff。
- 未发现业务页面、服务端、PRD、WBS、DB/API 契约、CI workflow 或其他 A 场景代码修改。

最新提交 `ad34df1` 只修正文档口径、补充证据披露与重算 SHA256SUMS；提交说明称 10/10 校验通过。该提交未改变 A05 断言或测试代码。

## 2. 核心断言复核

修改前 A05 在 800ms CDP 网络延迟配置后，固定等待 500ms 并读取按钮文案；最终 PASS 条件只有 `elapsed >= 800`。它没有捕获 pending 状态对象，也没有断言按钮 disabled。

修改后 A05：

1. 保留 `Network.enable` 与 `Network.emulateNetworkConditions` 的 800ms latency、下载/上传吞吐配置。
2. 点击登录后使用 `page.waitForFunction(fn, null, { timeout: 4000 })` 轮询 `#view button`，直到找到文案包含「登录中」的按钮。
3. 取回 `{ text, disabled }`，而不是仅读取按钮文本。
4. PASS 条件同时要求 `elapsed >= 800`、确实捕获 pending 对象、pending 文案匹配「登录中」、且 `pending.disabled === true`。
5. 随后等待 `localStorage.fl_token` 出现，最长 30 秒；超时会使场景执行失败，而不是返回 PASS。

Playwright `waitForFunction` 的参数位置正确：第二个参数传 `null` 作为 arg，第三个参数才是 timeout options。该处没有沿用 A01 的既有参数位置问题。

源码对比确认，修改仅位于 A05 块；A01 及 A02–A04、A06–A11 未被修改。A01 的 disabled 断言不再被拿来代替 A05 自身的慢网断言。

## 3. 运行证据交叉核对

仓库提交的四组报告与 console 日志互相一致：

| 运行 | A05 断言版本 | A 汇总 | A05 观察 |
|---|---|---:|---|
| baseline | 修改前 | 11 PASS | 有「登录中」文案；没有 pendingDisabled 字段，旧条件仍仅检查耗时 |
| after | 新断言 `=== true` | 11 PASS | 文案「登录中…」，`pendingDisabled=true`，最终 token=true |
| negative | 临时反置为 `=== false` | 10 PASS / 1 FAIL | 仍采到 `pendingDisabled=true`，A05 正确 FAIL |
| restored | 恢复 `=== true` | 11 PASS | `pendingDisabled=true`，最终 token=true |

关键证据：
- after：`A05 PASS`，总耗时 1046ms，`pendingDisabled=true`。
- negative：`A05 FAIL`，总耗时 1040ms，`pendingDisabled=true`；失败原因是断言被反置，而不是 pending 采样丢失。
- restored：`A05 PASS`，总耗时 1028ms，`pendingDisabled=true`。
- 四段日志均记录 Playwright Edge 156.0.4314.58、schema=8；报告中的 A 套件汇总与日志一致。

负向对照有价值：它证明同一采样值为 true 时，断言从 true 反置为 false 会令 A05 从 PASS 翻为 FAIL，降低「永真断言」或「仅采到文案」的风险。

## 4. 仍需如实保留的限制

1. **负向变异脚本未入库**：`artifacts/_a05_negative.py` 位于被忽略的 `artifacts/`，没有随交付提交。负向运行日志与报告已入库，但第三方不能直接从仓库取得原脚本一键重放。WorkBuddy 已在证据 README §6 披露；本轮不将其作为 A05 断言缺口未修复的理由，但记为可复现性观察。
2. **没有独立复跑**：本轮复核只读检查远端源码和已提交日志；没有独立 clone、没有本机 Playwright/Edge 重跑，也没有真机或人工视觉验收。
3. **没有当前 HEAD 的 CI 证明**：针对 `ad34df1` 查询到的 commit combined status 为空；按提交 SHA 查询 PR-triggered workflow runs 的接口也返回空。因此本报告不声称 A05 已通过新 HEAD 的 GitHub Actions。WorkBuddy 提交的四段日志属于本地运行证据。
4. **其他测试包未重跑**：交付 README 明确未重跑 C（228 条）和 D 套件；本轮没有证据表明这些状态发生变化。
5. **本机路径披露**：原始 console 日志保留了 `C:\\Users\\47006\\WorkBuddy\\fastledger\\repo_tmp` 工作目录与回环地址/随机端口。未见凭据值；但这些本机环境信息对复核断言并非必需，后续证据宜优先采用脱敏日志或结构化摘要，避免无必要暴露本机路径。

## 5. 结论与状态边界

- **A05 指定缺口：PASS（源码断言完整，所提交的本地正向/负向/恢复证据相互一致）**。
- 本 PASS 不等于独立 CI 验收，也不等于 A 包所有场景在当前 HEAD 已由第三方重跑。
- R0 仍 **BLOCK**。截至第十二次复核已知的其他阻塞项未因本次 A05 测试修改而自动关闭：C 仍为 164 PASS / 4 FAIL / 59 BLOCKED / 1 N/A；`OBS-ACCT-VIS-390` 仍 FAIL；t021 分类 override 仍待 Owner 裁决；RV-05 仍为 Owner ACTION REQUIRED。其他既有 R0 退出条件继续有效。
- PR #3 保持 OPEN、未合并；不得将本轮 A05 局部 PASS 解释为允许合并、启动 R1–R7、UI-DESIGN-02 代码实施或 PWA。
- 后续若需提高证据强度，可在该 HEAD 上运行 A 套件 CI，并由独立环境复跑；负向变异脚本可考虑作为脱敏、可重复的测试夹具纳入仓库。

