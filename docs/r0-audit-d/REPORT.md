# FastLadger R0 第四方独立审计报告

- 审计员代号：**D（第四方独立）**
- 审计对象：`main=dadc9a3`、`PR #3 fix/r0-sixth-runtime-defects @ 659c0c3`、CI run `42043801603`
- 审计方式：**只读静态审计 + Node 纯脚本级反例复现**（本环境无 gcc 编译 server、无 Playwright/Chromium，未真实跑浏览器 E2E；运行时结论均标注 NEEDS-RUNTIME-VERIFY）
- 权限边界：未改代码、未提交 PR、未动 Git 历史、未改仓库可见性
- 编号规则：D-R07 起（D-R01~R06 为前序轮次，不复用）；NEW 为本次独立新发现，CONFIRMED 为复核已有问题属实，DISAGREEMENT 为与 GPT/WorkBuddy 结论不一致

---

## 0. 独立结论：R0 是否具备通过条件

**判断：不具备「无遗留」通过条件；但在 Owner 显式接受 4 项 Known Issues 并人工补齐 2 项运行时证据后，可作为 R0 阶段性出口进入 R1。**

理由：
1. Go 后端（阶段 0-4）474 PASS 成熟可信；PR3 修的 6 个运行时缺陷经代码 diff 核对**真实对症**（`view.append(card)` 修空白页、`refillCategories` 按 catType 过滤、`renderLoadError` 清骨架屏、member 按钮后端 403 保留）。
2. 测试套件**无硬编码 PASS**：`run.js` 异常即 FAIL、有 FAIL 则 exit 1；CI 中 A/D 是门禁（不 continue-on-error），C 报告型 continue-on-error 但 SUMMARY 明示「success 不代表无 FAIL」；artifact 上传闸门依赖 `scan.outcome == 'success'`，扫描失败上传即跳过。这套设计是诚实的。
3. **阻塞项（须 Owner 裁决，非我可代决）**：
   - 凭据扫描器 K1（UTF-16LE）、K3（x-api-key 非 JWT）已转 Known Issues 未修——我在**全仓扫描器**上独立复现，并新发现 2 条绕过；
   - OBS-ACCT-VIS-390（390px 账户名被 CSS 隐藏）未闭环，D 套件诚实保留 FAIL；
   - C 套件 228 控件存在既有 FAIL（报告型，允许）。
4. **证据缺口（我无法在本机闭环）**：Playwright 全量 E2E 的 PASS/FAIL 明细、SQLite 事务/UPSERT 运行时行为、旧 Git SHA `259bd11` 的 29 个 JWT 能否被扫描器实际检出——这些只能在 CI 或带 gcc 的机器上验。

**建议**：R0 可「带条件放行」进入 R1，但安全扫描器盲区（D-R07~R09）应作为 R1 第一个修复包，不应顺延。

---

## 1. 新增问题清单（NEW，D 本次独立发现）

### D-R07 · URL query 参数中的凭据 0 检出
- **优先级**：P1
- **状态**：CONFIRMED（本机反例复现）
- **文件位置**：`tools/r0/scan-credentials.js`（PATTERNS L36-51、主扫描循环 L120+）
- **代码位置**：无任何 URL query 规则；`AUTH_HEADER` 只匹配 `authorization:`/`cookie:` 行首
- **需求依据**：交接文档点名「network-log.json 里有 JWT/访问令牌」——真实访问日志里 token 常出现在 query string
- **问题描述**：构造文件含 `GET /api/data?access_token=sk-live-9f8K...&user=1`，扫描器 `--json` 结果为 **0 命中**。
- **根因**：5 条模式均为「头字段名 + 冒号/等号」或 JSON 字段名形态；query string 的 `?key=value` 既无冒号、字段名又不在白名单（password/token/secret/apikey 等），`sk-live-` 前缀也无规则兜底。
- **影响**：用户导出的 Nginx access log、浏览器历史、HAR 里的 URL 带 token 时，全仓扫描器漏报；与 K3 叠加构成「日志类凭据基本不查」。
- **复现**：`printf 'GET /x?access_token=sk-live-9f8Kd2xQmNvBwRtY7hLcJ4sEoA1zXp HTTP/1.1\r\n' > /tmp/q.txt && node tools/r0/scan-credentials.js --json` → 该文件 0 行。
- **现有测试**：无。
- **修复建议**：在 PATTERNS 增加 query 形态规则，如 `/(?:[?&](?:access_token|api_key|apikey|token|auth)=)([A-Za-z0-9_\-]{16,})/g`，并对 `sk-`/`ghp_`/`xoxb-` 等前缀做通用命中。
- **验收标准**：含 query token 的样本文件 ≥1 条 `credential-candidate` 命中；正对照（无 token 的普通 URL）不误报。
- **回归风险**：低。需加白名单排除公开文档示例 URL，避免误报。

### D-R08 · HAR/网络日志数组形态 `{name,value}` 0 检出
- **优先级**：P1
- **状态**：CONFIRMED（本机反例复现）
- **文件位置**：`tools/r0/scan-credentials.js`（JSON_SECRET_FIELD 规则）
- **代码位置**：JSON_SECRET_FIELD 只匹配 `{"password":...}`/`{"token":...}` 这类对象键；不匹配 HAR 的 `{"name":"x-api-key","value":"..."}` 数组
- **需求依据**：交接文档 RV-05 明确「旧 SHA 259bd11 存在 network-log.json」——HAR 是浏览器导出网络日志的事实标准格式
- **问题描述**：构造 `{"log":{"entries":[{"request":{"headers":[{"name":"x-api-key","value":"sk-live-..."}]}}]}}`，扫描器 0 命中。
- **根因**：HAR header 是 `{name, value}` 对数组，键名 `name`/`value` 不在敏感字段白名单；且值是 `sk-` 非 JWT 三段，无 JWT_LITERAL 兜底。
- **影响**：用户上传/提交 HAR 或 Charles/Fiddler 导出时，其中的自定义头凭据完全漏检；这是「含网络明文压缩归档不上传」闸门（scan-artifacts.js）也同样漏的形态。
- **复现**：同 D-R07，写上述 JSON 到文件后扫描 0 行。
- **现有测试**：无。
- **修复建议**：对 JSON 做两轮——第一轮对象键名白名单；第二轮识别 `name` 值属于已知敏感头（x-api-key/authorization/cookie/proxy-authorization）时，把同对象 `value` 字段按敏感值处理。
- **验收标准**：HAR 样本中 x-api-key 值被命中；普通 `{"name":"Accept","value":"*/*"}` 不误报。
- **回归风险**：中。需精确定义「敏感头名清单」，避免把 `name` 字段一律当敏感。

### D-R09 · 两套扫描器规则不一致且互不复盖
- **优先级**：P2
- **状态**：CONFIRMED
- **文件位置**：`tools/r0/scan-credentials.js`（全仓，utf8 读取）vs `tools/r0/e2e/scripts/scan-artifacts.js`（artifact 上传，latin1 读取）
- **代码位置**：前者 L123 `readFileSync(abs,'utf8')`；后者 L331 `buf.toString('latin1')`，各有独立 PATTERNS/TEXT_RULES
- **需求依据**：交接文档要求「CI 绿灯真实可信」「上传前失败关闭」
- **问题描述**：两个扫描器读取编码不同（utf8 vs latin1）、规则集不同（前者有 JWT_LITERAL/BEARER_TOKEN 兜底，后者 5 条 TEXT_RULES 无 JWT 兜底）。全仓扫描器不扫 artifact 目录，artifact 扫描器不扫源码目录。K1 在两者上都成立但机理不同。
- **根因**：两个扫描器是为不同阶段（开发期全仓 vs CI 上传闸门）分别写的，未抽公共规则库。
- **影响**：修了一个扫描器的盲区，另一个仍漏；证据上传闸门的覆盖范围 ≠ 全仓覆盖范围，「扫描通过」不能跨场景引用。
- **复现**：diff 两份规则文件即可见。
- **现有测试**：`tools/r0/e2e/test-scan-artifacts.js` 只测 artifact 扫描器；全仓扫描器无对应测试。
- **修复建议**：抽 `tools/r0/lib/patterns.js` 公共规则库，两个扫描器都引用；编码层统一做 UTF-16LE/UTF-8 BOM 探测（见 D-R10）。
- **验收标准**：同一反例样本在两个扫描器上命中集合一致；规则单点修改两处同时生效。
- **回归风险**：中。需保证重构后既有 474 测试不红。

---

## 2. 已有问题独立复核（CONFIRMED / DISAGREEMENT）

### D-R10 · K1 UTF-16LE 编码绕过（全仓扫描器上复现）
- **优先级**：P1
- **状态**：CONFIRMED（我独立复现，与 GPT 在 scan-artifacts.js 上的发现互为印证）
- **文件位置**：`tools/r0/scan-credentials.js` L123
- **代码位置**：`fs.readFileSync(abs, 'utf8')` 无 BOM/编码探测
- **问题描述**：UTF-16LE 文件含 `Authorization: Bearer <JWT>`，扫描器 0 命中。
- **根因**：utf8 解码 UTF-16LE 字节流产生乱码（`A\0u\0t\0h\0`），`eyJ` 与 `authorization` 模式全部失配。
- **与 GPT 差异**：GPT 在 `scan-artifacts.js`（latin1）上记录 K1；我在**全仓** `scan-credentials.js`（utf8）上复现。两个扫描器都漏，但根因不同——latin1 是 NUL 插入，utf8 是解码乱码。**这是 DISAGREEMENT 的反面：GPT 以为 K1 只在 artifact 闸门上，实际全仓扫描器同样漏。**
- **复现**：`printf 'Authorization: Bearer <JWT>\n' | iconv -f UTF-8 -t UTF-16LE > /tmp/u16.txt` 后扫描 0 行。
- **修复建议**：读取时先探测前 3 字节 BOM（EF BB BF=UTF-8、FF FE=UTF-16LE、FE FF=UTF-16BE）；无 BOM 时对含大量 `\0` 的字节流按 UTF-16LE 二次解码。
- **验收标准**：UTF-16LE 与 UTF-8 BOM 样本均命中；UTF-16BE 同样覆盖。
- **回归风险**：中。编码探测有误判风险，需保留 utf8 兜底。

### D-R11 · K3 x-api-key 非 JWT 高熵值绕过
- **优先级**：P1
- **状态**：CONFIRMED
- **文件位置**：`tools/r0/scan-credentials.js` AUTH_HEADER 正则
- **代码位置**：正则只匹配 `authorization`，不匹配 `x-api-key`/`apiKey`/`X-API-Key`
- **问题描述**：`x-api-key: abcdef01234567890123456789`（28 位非 JWT）0 命中；同值放在 `authorization: Bearer` 下则被 BEARER_TOKEN 命中。
- **根因**：AUTH_HEADER 白名单头字段不含 x-api-key；普通高熵值无独立规则。
- **复现**：写 `x-api-key: <28位高熵>\n` 后扫描 0 行。
- **修复建议**：AUTH_HEADER 正则加入 `x-api-key|api[-_]?key|proxy-authorization`；对 `sk-`/`ghp_`/`gho_`/`xox[baprs]-` 前缀做通用命中。
- **验收标准**：x-api-key 下高熵值命中；x-api-key 下 `example`/`your-key-here` 等示例值不误报。
- **回归风险**：低。

### D-R12 · OBS-ACCT-VIS-390（390px 账户名不可见）
- **优先级**：P2
- **状态**：CONFIRMED（复核，未闭环）
- **文件位置**：`server/web/dist/style.css` L331（`td:nth-child(1)` 隐藏）
- **问题描述**：390px 视口下账户名列被 CSS 隐藏，DOM 存在但用户不可见。
- **与 GPT/WorkBuddy 一致性**：**同意**。D 套件 `d-defects.js` L49-59 `visibilityProbe` 诚实记录 `visibleInnerText`/`firstTdDisplay=none`，且 D-ACCT 组用例名明确标注「DOM 渲染一致性，不含视觉可见性」，未改判 I-ACCT-06 的 FAIL。这是正确的失败关闭。
- **我的意见**：这不是代码缺陷，是**产品决策未做**——Owner 需决定 390px 下是显示账户名（横向滚动）还是用图标/颜色编码。在 Owner 裁决前不应合并为 PASS。
- **修复建议**：Owner 裁决后，要么改 CSS 让首列在 390px 下可见，要么在产品上接受「账户名折叠到第二行」并更新原型。
- **回归风险**：低。

### D-R13 · PR3 六个运行时缺陷修复真实性复核
- **优先级**：P0（复核结论）
- **状态**：CONFIRMED（修复真实对症）
- **文件位置**：`server/web/dist/app.js`（+52 行 diff）
- **逐项核对**：
  1. `view.append(card)`（L386）——修交易总表空白页，D-TXNS-01 断言 `children>=1` 直接覆盖此修复；
  2. `renderLoadError`（L410 区）——修加载失败永久骨架屏，D-TXNS-03 注入 500 验证 `skel=0 && busy=0`；
  3. `refillCategories`（L553 区）——分类下拉按 catType 过滤，D-CAT-01/02 用 API 集合全等比对；
  4. 转账 `categoryId:0` 语义保留——D-CAT-06 断言 `body.categoryId===0 && txType===3`；
  5. member 邀请按钮权限——D-MEM-03（member 看不到按钮）、D-MEM-04（member 直接 POST invite 仍 403）；
  6. `--suite all` 纳入 A+D+C——run.js L105。
- **结论**：6 项修复均有对应 D 套件断言，不是「改了代码但没测试」。**DISAGREEMENT 风险点**：D 套件是门禁型，若 CI 上 D 套件红，PR3 不应合并；我无法在本机跑 D 套件，需 CI run 42043801603 的 D 汇总全绿作为外部证据。

---

## 3. 测试质量审计报告

### D-R14 · CI 绿色真实性（设计层面）
- **优先级**：P2
- **状态**：CONFIRMED（设计诚实，非缺陷）
- **观察**：
  - `run.js` L113-115：A 套件异常捕获标 FAIL，不吞错；L160-163 有 FAIL 即 exit 1；
  - `r0-e2e.yml`：A/D step 不 continue-on-error（门禁）；C step `continue-on-error: true` 但 SUMMARY 写「C 退出码 success 不代表无 FAIL」；
  - 上传闸门：`artifact_upload if: always() && steps.artifact_scan.outcome == 'success'`——修复了原先 `if: always()` 让扫描失效的问题（diff 注释明示）。
- **结论**：**交接文档担心的「continue-on-error 掩盖失败」在当前 PR3 不成立**。C 是明确报告型，且 SUMMARY 三层（白名单 prepare / 扫描 scan / 上传 outcome）都写进 step summary。
- **建议**：在 CI badge 或 README 里明确「R0 绿色 = A/D 门禁通过，C 报告型 FAIL 需人工看汇总」，避免后来者误读。

### D-R15 · 硬编码 PASS 扫描
- **优先级**：P2
- **状态**：CONFIRMED（未发现）
- **观察**：grep `return ok/return pass` 在 d-defects.js / txns-controls.js 中均为 `const pass = <真实观测表达式>` 后 `return pass ? ok(...) : bad(...)`；观测项包含 DOM 行数、API 回读集合、POST 状态码、请求体字段、后端 403。
- **未覆盖**：`c-controls.js`（924 行 228 控件）与 `a-scenarios.js`（355 行）我未逐行通读，仅抽查结构与注释。若需 100% 确信，需在带浏览器的环境跑 C 套件并人工核对 FAIL 项。
- **结论**：抽查范围内无硬编码 PASS。

### D-R16 · 证据缺口（本审计局限）
- **优先级**：P1（流程风险）
- **状态**：NEEDS-RUNTIME-VERIFY
- **描述**：本环境（云主机 vm）无 gcc，无法 `go build` server；未 `npm ci`/未装 Chromium，无法真实跑 Playwright E2E。以下结论基于静态阅读与 Node 纯脚本：
  - D 套件是否在 CI 42043801603 全绿——需看 artifacts/r0-e2e-upload 的 report.json；
  - C 套件 228 控件的实际 FAIL 清单与分布——未读 c-report.json；
  - 旧 SHA `259bd11` 的 29 个已失效 JWT 能否被当前扫描器检出——未跑 `git log -p` 历史扫描；
  - SQLite 事务回滚/UPSERT RowsAffected 运行时行为——纯 Go 代码静态阅读，无运行时。
- **建议**：在 CI 或带 gcc 的开发机上补跑一次 `node run.js --suite all`，把 report.json 作为 R0 放行证据归档。

---

## 4. 与 ChatGPT 审计结论差异表

| 主题 | GPT 结论 | D（我）结论 | 差异性质 |
|---|---|---|---|
| K1 UTF-16LE | 在 scan-artifacts.js（latin1）上发现，转 Known Issues | 在全仓 scan-credentials.js（utf8）上**也复现**，且根因不同 | **GPT 覆盖不足**：以为只在 artifact 闸门 |
| K3 x-api-key | 在 scan-artifacts.js 上发现 | 在全仓 scan-credentials.js 上复现 | 一致，但范围扩大到全仓 |
| URL query token | 未提及 | 0 检出（D-R07） | **GPT 未覆盖，新发现** |
| HAR {name,value} 数组 | 未提及 | 0 检出（D-R08） | **GPT 未覆盖，新发现** |
| 两套扫描器规则不一致 | 未作为独立问题提出 | D-R09 提出应抽公共规则库 | **新观察** |
| PR3 6 项修复真实性 | 自称已修 | diff 核对真实，且有 D 套件断言覆盖 | **同意** |
| 测试硬编码 PASS | 交接文档点名担心 | d-defects 抽查无；c-controls 未逐行 | 部分同意，需补跑 |
| OBS-ACCT-VIS-390 | 记录为已知 | 同意，且 D 套件诚实保留 FAIL 不改判 | **同意** |
| CI 绿灯真实性 | 担心 continue-on-error | 设计正确，C 报告型明示 | **GPT 担心过度**（当前 PR3 已修） |
| 29 个旧 JWT 历史泄露 | RV-05 记录 | 未实际跑 git 历史扫描验证 | **证据缺口，需补** |

---

## 5. 建议整改优先级与依赖

| 顺序 | 事项 | 优先级 | 依赖 | 为什么这个顺序 |
|---|---|---|---|---|
| 1 | D-R10 编码探测（UTF-16LE/BOM）+ D-R11 x-api-key/sk- 前缀 | P1 | 无 | 安全扫描器盲区是 R0 唯一「已知未修且可能漏真凭据」的项；在 R1 前必须修 |
| 2 | D-R07 URL query + D-R08 HAR 数组规则 | P1 | 依赖 1 的公共规则库 | 同属扫描器覆盖，与 1 合并成一个规则包 |
| 3 | D-R09 抽公共 patterns.js | P2 | 1、2 完成后重构 | 避免两个扫描器再次漂移 |
| 4 | D-R16 在 CI/带 gcc 环境补跑 `--suite all` 并归档 report.json | P1 | PR3 合并后 | R0 放行的运行时证据 |
| 5 | D-R12 OBS-ACCT-VIS-390 Owner 裁决 | P2 | 产品决策 | 不阻塞 R1，但 R1 前需有结论 |
| 6 | C 套件既有 FAIL 逐条 triage | P2 | 4 的 report.json | 区分「产品缺口」与「真 bug」 |
| 7 | 旧 SHA 259bd11 的 29 个 JWT 用新扫描器重扫 | P2 | 1、2 完成后 | 验证修复是否真能抓到历史泄露 |

---

## 6. 审计边界声明

1. 本报告为只读审计，未修改任何代码、文档、Git 历史或仓库设置。
2. `.audit-tmp/` 反例文件（n1-n8）在审计工作区内未删除（用户拒绝 `rm -rf`）；交付前应由 Owner 或我经授权后清理，避免被后续扫描误判为命中。
3. 所有「运行时」结论（D 套件是否绿、C 套件 FAIL 清单、SQLite 行为）均标注 NEEDS-RUNTIME-VERIFY，不得以本报告作为运行时证据。
4. 我未读 `gpt/` 目录下 ChatGPT 本轮 findings 原文全文，差异表基于 `docs/r0-evidence-7th-fix/p1/qa-verify-round2/` 与交接文档；若 GPT 另有未入库结论，可能存在未对齐项。
5. 本审计员与 WorkBuddy / ChatGPT 无利益关联，结论独立。
