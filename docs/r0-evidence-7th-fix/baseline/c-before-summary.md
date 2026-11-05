# 修复前 C 套件基线摘要（复制自 report.json）

- 来源：`artifacts/r0-fix-baseline/2026-11-05_09-13-30-dadc9a3-c/report.json`（完整副本见同目录 `c-before-report.json`）
- 完整 stdout/stderr + 退出码：`c-before.log`（`EXIT_CODE=1`）
- 命令：`node run.js --suite c`（`R0_SERVER_BIN` 指向本次重新构建的 `server/fastledger.exe`，`R0_BROWSER_CHANNEL=msedge`）
- 服务：临时实例 `http://127.0.0.1:53768`，schema=8；浏览器 playwright-core / msedge 156.0.4314.58
- commit：`dadc9a3b9c567cad2bbad7e47361fb774365b949`，分支 `fix/r0-sixth-runtime-defects`

## 汇总

| 状态 | 数量 |
|---|---|
| PASS | 144 |
| FAIL | 6 |
| BLOCKED | 77 |
| N/A | 1 |
| 合计 | 228 |

## 6 条 FAIL

| ID | 说明 |
|---|---|
| I-LOGIN-04 | 浏览器中不存在「明文切换」控件（主报告登记缺失 R0-E-39） |
| I-LOGIN-11 | 产品侧未实现（主报告已登记 R0-E-38） |
| I-LOGIN-12 | 产品侧未实现（主报告已登记 R0-E-38） |
| I-HOME-01 | 本页无账本切换控件（主报告登记缺失 R0-E-01） |
| **I-TXNS-01** | **交易总表 `#view` 渲染为空——运行时确认该页不可用（需排查 renderTxns）** |
| **I-ACCT-06** | **创建后 API 可见=true 页面可见=false（疑似列表未刷新，需 QA 复现确认）；`#acctErr=「」` toast=「✓ 账户已创建」** |

前 4 条为既有产品缺口/已登记缺失，与本次 4 项授权缺陷无关，不在本次修复范围。

## 与本次 4 项缺陷相关的基线状态

- `I-TXNS-01` FAIL；`I-TXNS-02` ~ `I-TXNS-19` 共 18 条被硬编码为 BLOCKED（「同页渲染为空，控件不存在，无法执行」）。
- `I-ACCT-01`~`05` PASS、`I-ACCT-06` FAIL、`I-ACCT-07`~`10` PASS、`I-ACCT-11`~`15` BLOCKED。
- `I-ADD-06` PASS（仅断言「分类下拉选项数>0」，30 项混合分类也通过）；`I-ADD-06b`（脚本内计算的首项判定）**不在 `controls.json` 的 ID 集合中，未进入 report.json** —— 分类未过滤缺陷在 C 报告里被静默丢弃。
- `I-MEM-01`~`11` 全部 PASS（owner 身份），其中 `I-MEM-03`「邀请成员按钮存在」在 owner 下正确；member 身份的 UI 越权只在 A08 中被记录、不构成 C 的 FAIL。

## 同时采集的修复前直接观测

见 `pre-fix-observations.md` 与 `baseline-probe.json`（`tools/r0/e2e/scripts/baseline-probe.js` 产出）：

- 交易总表 `#view.childElementCount = 0`，`innerText`/`textContent` 均为空字符串，但网络日志显示 `GET /books/1/transactions?page=1&count=50` 返回 200 → 数据已取回但视图未挂载。
- 「记一笔」弹窗在支出/收入/转账/切回支出 4 种状态下，`#fCategory` 均为同一份 30 项混合下拉，首项 `工资`(value=23, catType=1 收入) 被默认选中；切换类型不重建下拉。
- member 身份进成员页「邀请成员」按钮数 = 1（owner/admin 各 1）；member 直接 `POST /members/invite` = HTTP 403（后端校验存在且必须保留）。
