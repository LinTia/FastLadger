# R0 第七次测试质量纠偏任务单（A/C 断言有效性 + B Owner 裁决）

> 基线：第六次自动化交付后，当前 main 曾到 `48d0fc4`；开工时必须重新 fetch 并记录实际 HEAD。
> 本任务只纠正测试有效性、证据链和待裁决项；**不授权修改业务代码、PRD、WBS、数据库/API，不启动 R1–R7，不启动 PWA**。
> Owner 当前主要使用手机，不得安排 Owner 手工逐条执行 228 项。

## 0. 第三方独立复核结论

第六次交付有真实进展，但不能按回执原样全部验收：

| 包 | 第六次回执 | 第三方复核后的暂定状态 | 原因 |
|---|---|---|---|
| A | 11/11 本地 + CI PASS，具备申请改判条件 | **PARTIAL，暂不改 PASS** | A05 的 PASS 条件只检查总耗时，不检查报告标题要求的 pending 文案/禁用态 |
| B | PARTIAL | **PARTIAL** | 分类解析硬化有效；但 t021 的 `meal → dinner` 覆盖由 WorkBuddy 自行署名决定，尚无 Owner 裁决证据 |
| C | 已执行 228 条：144/6/77/1 | **PARTIAL，不得按 144 条全数直接验收** | 发现至少 15 条硬编码 PASS（其中部分引用其它测试、部分没有任何断言）；需逐项补证据或降级 |
| D | PARTIAL（3/5/5） | **PARTIAL，维持** | 本轮未复核 D 的路由覆盖矩阵 |
| RV-05 | Owner ACTION REQUIRED | **Owner ACTION REQUIRED** | 仓库可见性/旧 SHA 匿名访问/历史清理仍未闭环 |

独立 CI run `41990345757` 的 job 和步骤均 success，head 为 `825f3c74834904d0fde6c3f980b48806be5b2404`。但该工作流对 C 使用 `continue-on-error: true`，因此 job success 不代表 C 无 FAIL；CI 日志本身记录 C = 144/6/77/1。A 的 CI 结果也只能证明当前测试代码执行成功，不能修复测试断言本身过弱的问题。

## 1. P1：修复 A05 假阳性，重新验证 A

证据：
- `tools/r0/e2e/specs/a-scenarios.js` 的 A05 读取了 `midText`，但最终 status 只按 `elapsed >= 800` 判 PASS。
- 因此即使中途没有出现 pending 文案或按钮没有禁用，只要总耗时超过 800ms 仍会 PASS。

要求：
1. A05 必须同时断言：中途按钮文案为登录中状态、按钮 disabled=true、最终登录成功。
2. 断言必须在请求尚未完成的窗口采集；避免仅在请求完成后读取。
3. 把实际观测到的文案、disabled、请求耗时和截图写入结果。
4. 增加一个负向合成验证：故意令 pending 文案/disabled 断言失败时，A05 必须失败。
5. 重新跑本地与独立 GitHub Actions CI；CI 结果必须绑定实际测试代码 SHA。
6. 若其它 A 场景也存在“记录了观测值但未纳入 PASS 条件”的情况，一并修正。
7. A08 当前验证了 member 管理 API 返回 403，但 UI 仍显示“邀请成员”按钮。保留 API 权限测试 PASS 的边界，同时单独输出 UI 权限 affordance 缺陷；不得把 API 403 写成 UI 权限体验完整。

在上述完成前，A 总体标记 PARTIAL，不得申请 PASS。

## 2. P1：C 的 PASS 必须有可追溯断言

第三方从 `tools/r0/e2e/specs/c-controls.js` 找到至少以下硬编码 PASS：
- I-BOOKS-09、I-BOOKS-10
- I-HOME-11、I-HOME-13、I-HOME-15、I-HOME-17、I-HOME-18
- I-ADD-14
- I-TXDET-10、I-TXDET-11、I-TXDET-12、I-TXDET-14
- I-MEM-10
- I-EXP-11

要求逐条处理，不允许简单批量改状态：
1. 若实际由另一个测试覆盖，必须增加机器可读的 `coveredBy` / `evidenceRefs`，引用确切测试 ID、运行记录和断言证据；该引用必须真实存在。
2. 若本条控件本身可执行，则补真实动作与结果断言，例如取消弹窗后断言 dialog closed、成员数通过 API 回读、删除后验证导航目标、转账双账户余额联动等。
3. 若没有实际断言，改为 BLOCKED 或 N/A，并说明具体原因；不得保留硬编码 PASS。
4. I-TXDET-11“转账对侧联动由服务端整笔语义保证”不能以代码/服务端语义推断替代运行时验证；必须创建转账、核对两侧账户余额与交易记录，或明确 BLOCKED。
5. I-TXDET-14 必须真实请求不存在的交易 ID 并断言 404/错误语义，不能仅写“A 组已覆盖”。
6. I-HOME-15 必须通过实际 viewport/元素可见性断言，不得只写“首屏无需滚动”。
7. 所有 PASS 都必须具有可复核证据；汇总增加 direct-pass / cross-covered-pass / fail / blocked / N/A 分布，避免将引用覆盖误读为独立执行。

## 3. P1：控制清单与自动化脚本必须防止漂移

当前 `.github/workflows/r0-e2e.yml` 没有执行 `extract-controls.js --check`，也没有把当前主报告 SHA 与已提交 `controls.json` 的 `sourceSha256` 做强制比较。

要求：
1. CI 执行控制清单提取校验。
2. 比较主报告实时 SHA256、controls.json 的 sourceSha256、控制条数与 ID 集合；任何不一致即失败。
3. 校验 228 个 ID 无重复、无遗漏、无多余；不能只检查总数等于 228。
4. 每条结果必须且只能对应一个源控件 ID；重复 ID、缺少 ID、未知 ID 均使 CI 失败。
5. CI 摘要明确写出 A 的结果和 C 的各状态计数；不得将 C 的 continue-on-error job success 表述为 C 通过。

## 4. P1：t021 分类覆盖必须补 Owner 决策依据

文件 `tools/r0/category-overrides.json` 当前将 t021 从 `meal` 改判为 `dinner`，`decidedBy` 写为 WorkBuddy。WorkBuddy 可以提出建议，但不能代替 Owner 裁决产品数据语义。

要求：
1. 保留原始原型记录 `categoryId=meal`、`categoryName=餐饮-聚餐` 和冲突日志。
2. 暂不把 WorkBuddy 自行决定视为 Owner 决策。将 t021 设为 pending owner decision，或将覆盖放在不影响默认审计的候选文件中。
3. 提交简短裁决说明：选 meal 会导致原型可见分类与数据 ID 不同；选 dinner 会以展示名为准、改变原有数据映射。说明两种方案影响，不替 Owner 选择。
4. 不得为让 AUDIT_OK 通过而将未经批准的 override 固化为已裁决事实。
5. 收到 Owner 明确决定前，B 保持 PARTIAL；不得要求 Owner 使用电脑，可在聊天中作出一字/一句裁决。

## 5. 第六次发现的产品缺陷：只整理修复提案，不改业务代码

以下缺陷需继续登记并提供最小修复方案、影响面和回归用例，但本任务不授权修改生产业务代码：
- 交易总表 `renderTxns()` 未 append card，整页空白，关联 18 条控件 BLOCKED。
- 记一笔分类下拉未按交易类型过滤，支出可选到工资等收入类分类，提交被 400 拒绝。
- member UI 仍显示邀请成员按钮，虽然服务端返回 403。
- 新增账户 API 可见但页面 6 秒内不出现；目前是疑似刷新问题，需补稳定复现再定性。

每项需列出文件/函数/行号、复现步骤、预期/实际、最小修复建议、自动化回归测试、是否影响既定 PRD。等待 Owner 单独授权后再改业务代码。

## 6. 交付与状态规则

WorkBuddy 应：
1. 开工前重新 fetch 最新 main，记录 HEAD；读取多AI对齐规范、第三方导读、CHANGELOG 1.77–1.80、第五次任务单、第五/第六次回执、R0 主报告和本任务单。
2. 只修改自动化测试、测试夹具、证据脚本和审计文档；不改业务代码/PRD/WBS/API/DB。
3. 提交 A05 修正、C 逐项证据、清单漂移校验、t021 Owner 决策待办、四项业务缺陷修复提案。
4. 本地与 CI 结果都要提供 commit SHA、run ID、job/step 结论、原始日志、artifact、SHA256 和退出码。
5. 更新第六次回执、R0 主报告、README、CHANGELOG；旧证据不得覆盖或删除。
6. 对状态只按证据改判：A 修复 A05 并通过独立 CI 后才可申请 PASS；C 维持 PARTIAL，直至 FAIL/BLOCKED 得到处理或 Owner 对范围作出裁决；B 维持 PARTIAL 直到 t021 决策有据；D 维持 PARTIAL；RV-05 仍 Owner ACTION REQUIRED。
7. R0 仍 BLOCK；没有 Owner 明确授权不得启动 R1–R7/PWA。

**禁止事项**：不得把 WorkBuddy 自述当作独立复验；不得把 CI job success 等同于所有 suite PASS；不得把硬编码状态、静态代码定位或截图存在等同于控件行为验收；不得替 Owner 裁决 t021；不得要求 Owner 在手机上手工验收 228 条。
