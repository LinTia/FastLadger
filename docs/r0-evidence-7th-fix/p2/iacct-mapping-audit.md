# I-ACCT 编号 ↔ 228 清单定义 全量错位审计（P2）

- 范围：`tools/r0/e2e/fixtures/controls.json` L1498–1736 的 `I-ACCT-01..15`（共 15 条）
  与 `tools/r0/e2e/specs/c-controls.js` 的 `groupAccounts`（本轮 P2 修改后的状态）。
- 目的：**如实登记历史覆盖缺口**，不掩盖、不消灭错位。
- 硬约束：本文件只是审计记录。**未修改、也不得修改** `controls.json`（228 条清单）、
  未改测试编号、未把清单定义改成脚本现状。
- 本轮授权范围：仅按第三方点名修复 `I-ACCT-06` 与 `I-ACCT-10`；其余 13 条**不批量改动**，
  因为批量对齐会显著改变 228 条 PASS/FAIL/BLOCKED 分布并超出本轮授权。

## 一、逐条对照

「是否错位」判定标准：脚本断言的**被测对象**是否与清单 `control` / `action` / `success`
描述的是同一个控件与同一个交互。

| 清单 ID | 清单定义（control / action / success，controls.json） | 脚本当前实际断言（P2 后） | 是否错位 |
|---|---|---|---|
| I-ACCT-01 | `＋ 新增账户`；点击 → `openAcctForm(bookId,null)`；成功=打开 `#acctDlg` | `/现金\|储蓄\|信用卡/.test(#view.innerText)` —— 账户行是否渲染 | **错位**：测的是清单 I-ACCT-02 的「账户行」，不是新增按钮/弹窗打开 |
| I-ACCT-02 | 账户行（名称/类型/余额）；成功=显示 `ACC_CAT_NAME`、余额、正负色 | `button:hasText('＋ 新增账户')` 计数 > 0 | **错位**：测的是清单 I-ACCT-01 的「＋ 新增账户」按钮是否出现 |
| I-ACCT-03 | 编辑按钮；点击 → `openAcctForm(bookId,a)`；成功=打开 `#acctDlg`（回填） | 点击「＋ 新增账户」后 `#acctDlg.open === true` | **错位**：测的是新增弹窗打开，既不是编辑按钮也不是编辑回填 |
| I-ACCT-04 | 删除按钮；`confirmDialog` → `DELETE /books/:id/accounts/:id`；成功=toast「账户已删除」+ 重绘 | `#acctName` 名称输入框存在 | **错位**：测的是清单 I-ACCT-08 的「账户名称 `#acctName`」 |
| I-ACCT-05 | 删除取消；确认框点「取消」→ 不删除 | `#acctCat` 类型下拉存在 | **错位**：测的是清单 I-ACCT-09 的「账户类型 `#acctCat`」 |
| I-ACCT-06 | **删除引用保护**；账户被交易引用 → 确认删除 → 后端 `ErrAccountInUse` 映射 409 | **P2 已对齐**：`api.raw(DELETE /api/v1/books/:id/accounts/:accountId)` 对被引用的 `data.ids.cashId` 发起删除，断言真实 `status===409`、`json.msg` 含「账户仍存在交易记录」，并用 `api.accounts()` 回读确认该 id 仍在、名称一致 | **已对齐（本轮修复）**。修复前该 key 被写成「新增账户名称 390px 不可见 / 可见性未闭环」的无条件 FAIL，属清单外观察项错占编号，且**从未测过删除引用保护** |
| I-ACCT-07 | 期初余额输入；`input`/`action`/`success`/`fail` 均为「未实现」，`evidence` 记「缺失：app.js `initialBalance`」 | `/¥/.test(#view.innerText)` —— 余额列是否渲染 ¥ | **错位**：清单该项是「未实现/缺失」，脚本却用「余额列渲染」判 PASS；且 `status` 字段在夹具里为异常截断值 `期初\`（疑似抽取产物，本轮不修） |
| I-ACCT-08 | 账户名称 `#acctName`；action=键入；成功=显示文本；空值 → `#acctErr` | `button:hasText('编辑')` 计数 > 0 | **错位**：测的是清单 I-ACCT-03 的「编辑按钮」 |
| I-ACCT-09 | 账户类型 `#acctCat`；action=选择；编辑时 disabled | `button:hasText('删除')` 计数 > 0 | **错位**：测的是清单 I-ACCT-04 的「删除按钮」 |
| I-ACCT-10 | **币种 `#acctCur`**；默认 CNY、编辑时 disabled；action=键入；成功=显示文本 | **P2 已对齐**：在新增账户弹窗内断言 `#acctCur` 存在（tag/type/maxlength/disabled 真实观测）、可键入（填 `CNY` 后 `inputValue` 回显一致） | **已对齐（本轮修复）**。修复前为 `!(await text(page)).includes(name) ? ok('删除后从列表消失') : bad(...)`：用 `#view.innerText` 判断名字，而 390px 下 `style.css` L331 把 `td:first-child` 设为 `display:none`，名称从不在 `innerText` 中 → **恒真，确认假阳性**；且测的也不是币种 |
| I-ACCT-11 | 取消 `#acctCancel`；action=点击；成功=关闭弹窗 | `blocked('该组后续控件…未以独立控件出现')` | **错位**：清单有明确已实现控件 `#acctCancel`（index.html L144 / app.js L728），脚本却判 BLOCKED |
| I-ACCT-12 | 保存 `#acctSubmit`；新增 POST / 编辑 PUT；成功=toast + 关弹窗 + 重绘 | `blocked(...)`（同上循环）；实际脚本在新增流程中点击了 `#acctSubmit` 并等待 POST，但结果没有登记到本 ID | **错位**：清单有明确控件且脚本实际执行过，却判 BLOCKED |
| I-ACCT-13 | 余额刷新；增删后 `renderAccounts(bookId)` 重拉 | `blocked(...)` | **错位**：清单为已实现项，脚本判 BLOCKED |
| I-ACCT-14 | 本页净资产摘要；`status = **缺失**｜R0-E-28` | `blocked(...)` | **错位**：按包 C 判定原则，「主报告登记为缺失」应判 FAIL 并写明 R0-E-28；因 `R` 已给出值，`classifyMissing()` 不会介入，故被登记成 BLOCKED |
| I-ACCT-15 | 投资账户行点击 → 详情（`category===5` 时进入 `#b/:id/account/:accId`） | `blocked(...)` | **错位**：清单 `status=已实现｜截图已见`，脚本判 BLOCKED |

## 二、结论

1. **15 条里 13 条仍存在编号/定义错位**，本轮按授权范围**不批量改动**。它们多数是
   「脚本断言整体前移/后移了若干格」（01↔02、04↔08、05↔09、08↔03、09↔04），
   以及「11–15 被一组统一的 `blocked()` 兜底覆盖」。
2. 这些错位是**历史覆盖缺口**，不是本轮引入：本轮只把 `I-ACCT-06` 从「清单外可见性
   FAIL」改回清单定义的「删除引用保护」真实 409 断言，把 `I-ACCT-10` 从「innerText
   恒真假阳性」改回清单定义的「币种 `#acctCur`」真实键入断言。
3. 视觉可见性（第四项缺陷）**不是 228 清单里的任何编号**，本轮已从 `R` 中移出，
   以清单外独立 ID `OBS-ACCT-VIS-390` 登记到
   `C/I-ACCT/visibility-observation-independent.json`，**状态仍为 FAIL / 未闭环**；
   原「删除后从列表消失」以清单外 `OBS-ACCT-DEL-390` + `td:first-child.textContent`
   真实断言单独落盘。两者都不写入 `R`、不占编号、不改变 228 分布。
4. **若要全面对齐 228 清单，需另提方案供第三方审核**：应逐条给出新的可执行配方
   （真实点击/键入/API 回读）、预期 PASS/FAIL 判据、以及对齐后 228 条分布的变化量，
   经第三方与 Owner 确认后再实施。严禁以「把 controls.json 改成脚本现状」「改测试编号」
   或「把清单定义改写成脚本断言」的方式消灭错位。
5. 本审计未涉及、也不得改动：`controls.json`（228 条清单）、`server/web/dist/style.css`
   （L331 视觉根因）、任何 Go 后端、`server/web/dist/app.js`、PRD/WBS、DB/迁移、API 契约、
   更早的历史证据目录。
