# [审查] 统计与导出：预算漏算及 CSV 文本公式风险

审查基线：`main@b8741465afa98a89ed99796c9e2c4a841b15a361`。按 `workbuddy/第三方审阅者导读.md` 完成前轮去重与权威文档对齐。

本仓库尚无正式后端工程。以下是**设计/教学代码/契约问题**，并非声称已经验证线上漏洞。SQL/算法反例与静态结论各自标注验证边界；不对原型生产安全作推断。

本 Issue 包含同一修复域的 R13, R14；处理状态均先为 **待确认**，请逐项更新 CHANGELOG 为“已采纳（含位置）/未采纳（含理由）”，并保留被证伪结论的纠正记录。

处理清单：

- [ ] R13：预算 SQL 漏算小类；总预算仍带 category_id 条件
- [ ] R14：CSV 文本字段未防公式解释

### R13 · P2 · 预算 SQL 漏算小类；总预算仍带 category_id 条件

验证方式：原文 SQL 实测。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-落地方案.md · §4.3 L521–L523](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L521-L523)
- [docs/FastLedger-落地方案.md · §4.3 L538–L546](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L538-L546)
- [docs/FastLedger-新手开发完整指南.md · §18.1 L1254–L1266](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1254-L1266)
- [docs/FastLedger-新手开发完整指南.md · §18.1 L1287–L1287](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1287-L1287)

原文片段：

```text
WHERE book_id=? AND category_id=? AND tx_type=2
总预算（budget_type=2）统计时不按分类
```

复现/核查步骤：

1. 给大类 10 创建预算 10000 分，小类 11 的 parent_id=10。
2. 小类发生支出 12000 分，执行 BudgetProgress 中原文 SQL；再对总预算 category_id=0 执行相同函数。

实际结果：大类预算已用 0 分、总预算已用 0 分，预期两者均为 12000 分。代码没有总预算分支，小类也未汇总。

预期结果：分类预算包括该大类及子类支出；总预算不限定分类；删除/修改交易后同样正确。

修复建议：建议改后规则：按 budget_type 分支组装查询。分类预算收集父类和子类 ID（或递归查询），总预算去掉分类谓词；使用明确账本时区下的 [periodStart,periodEnd) 区间避免边界重复，统一启用状态与可见性策略。

验证边界：budget_descendants_and_total 已运行；这是查询错误，不依赖原型 UI。

与前轮关系：新增发现。

### R14 · P2 · CSV 文本字段未防公式解释

验证方式：导出源码 + 原始 CSV 字段保留验证；未在 Excel 中执行。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-新手开发完整指南.md · §17.1 L1222–L1233](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1222-L1233)
- [docs/FastLedger-产品需求文档-PRD.md · §4.9 L267–L271](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-产品需求文档-PRD.md#L267-L271)

原文片段：

```text
w.Write([]string{ ..., t.Comment })
```

复现/核查步骤：

1. 把备注设为无害字符串 =1+1，按代码路径通过标准 CSV writer 导出。
2. 检查 CSV 文本字段，确认仍以 = 开头；后续需在目标 Excel/LibreOffice 版本验证打开与重存行为。

实际结果：备注中的 =1+1 原样保留；语法引号转义没有中和公式前缀。已确认导出编码缺少防护，未声称实测发生外传或命令执行。

预期结果：用户可控文本在面向表格软件的导出中按文本处理；公式能力不应由备注、分类名或账户名意外触发。

修复建议：建议改后规则：对所有用户可控文本字段制定并测试表格软件安全的文本编码策略，覆盖 =/+/−/@ 与前导控制字符；区分给程序消费的原始数据导出与给表格软件查看的安全导出。不能把加 BOM 或普通 csv 引号当作公式防护；保留 JSON 作为原始数据通道。参考 OWASP CSV Injection，兼容性测试包括保存后重开。

验证边界：csv_formula_unneutralized 已运行；仅验证原始字段，不冒充 Excel 端到端复现。

与前轮关系：新增发现。

维护者复核说明：复现脚本及完整报告由本轮审阅交付；不直接替换业务方案。对已有修复的补充项已注明具体旧代码残留位置，避免把“新增机制存在”误认为“所有写路径均已闭合”。

官方参考：[OWASP CSV Injection](https://community.owasp.org/attacks/CSV_Injection)。本轮仅确认原始导出字段未中和，未在 Excel 执行公式。
