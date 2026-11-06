# FastLedger R0 第十一次独立复核：RV-14 指定 PNG/NUL 反例补齐

- 复核日期：2026-11-07（以 GitHub Actions 于 2026-11-06 UTC 生成的交付证据为准）
- 仓库：`LinTia/FastLadger`
- PR：[#3](https://github.com/LinTia/FastLadger/pull/3)
- PR 分支：`fix/r0-sixth-runtime-defects`
- 复核 HEAD：`858b7b453f783eba462017e1e8670e3975f3ae65`
- 前一轮复核基准：`2082b08307798d1c35b6d5a3e20cf0a748c1993d`
- 本轮整改提交：`3097ac8e4b93bbfceee90c3c35847e021ec275d8`（测试/证据）；`858b7b453f783eba462017e1e8670e3975f3ae65`（仅 workflow 注释和 step 名称）
- 结论：**RV-13 PASS；RV-15 PASS（沿用前轮真实负向证据，本轮正向回归）；RV-14 PARTIAL（当前扫描行为证据通过，但 case-51 断言未隔离 Authorization 检测路径）。R0 仍 BLOCK。**
- PR 状态：OPEN、`merged=false`；未合并 main。

## 1. 本轮范围

仅复核 WorkBuddy 对上一轮 RV-14 PARTIAL 的补充：合法 PNG 魔数文件中，Authorization 关键词内部 NUL、x-api-key 关键词内部 NUL、x-api-key 值内部 NUL 三种指定反例；同时检查 RV-13 清单、RV-15 上传门禁是否回归，以及最新 R0 Artifact 中既有阻塞项是否仍存在。

本轮没有修改仓库、业务代码、测试、PR 或工作流；以下结论来自 GitHub 远端文件、Actions API 与实际下载 Artifact 的独立检查。

## 2. 最新交付与文件范围

PR #3 最新 HEAD 为 `858b7b453f783eba462017e1e8670e3975f3ae65`，PR 仍 OPEN、未合并。

本轮实际提交内容：
- `3097ac8`：新增 `case-51~53`；扩展 `rv14-binary-evidence.js`；重新生成 `scan-tests-round5.log` 与 `rv14-binary-evidence.txt`。
- `858b7b4`：只把 `.github/workflows/r0-e2e.yml` 的扫描器测试说明由 50 项更新为 53 项；diff 仅注释和 step 显示名称，没有修改 `if:`、`run:`、`uses:` 或上传条件。
- 扫描器本体 `tools/r0/e2e/scripts/scan-artifacts.js` 未修改；本轮复核读取到的 blob SHA 为 `0c1539ef969651d145657b7dee20220017549000`。

## 3. RV-14 指定反例复核

### 3.1 新增测试与运行结果

最新 Artifact：`13521151959`，名称 `r0-e2e-evidence-858b7b453f783eba462017e1e8670e3975f3ae65`。

GitHub Actions：
- R0 E2E：[run 42251837877](https://github.com/LinTia/FastLadger/actions/runs/42251837877)，success；扫描器测试 step 实际执行 53 项。
- CI build/vet/test/race：[run 42251842888](https://github.com/LinTia/FastLadger/actions/runs/42251842888)，success；全仓 `go test -race` 与容器冒烟步骤均 success。
- RV-15 gate positive：[run 42251849969](https://github.com/LinTia/FastLadger/actions/runs/42251849969)，success；真实 upload-artifact step success。

从最新 Artifact 中提取的 `scan-tests-round5.log` 显示：53 tests、53 pass、0 fail、0 skipped。三个新样本的实际扫描器子进程退出码均为 2（BLOCK）。

| 用例 | PNG 内负载构造 | 新扫描器 | 旧基线 `2699c9c` | 复核 |
|---|---|---|---|---|
| case-51 / RV-14-07 | `Authori[NUL]zation: Bearer <synthetic>` | rc=2；日志实际命中 AUTH_HEADER 与 BEARER_TOKEN | rc=2；BEARER_TOKEN 已命中 | 行为符合；但断言强度不足，见 §3.2 |
| case-52 / RV-14-08 | `x-api-[NUL]key: <synthetic>` | rc=2；SENSITIVE_ASSIGN | rc=0 CLEAN | 旧版绕过得到实证 |
| case-53 / RV-14-09 | `x-api-key: SYN[NUL]THETIC<synthetic>` | rc=2；SENSITIVE_ASSIGN | rc=0 CLEAN | 旧版绕过得到实证 |

以上样本由运行期合成值构造；Artifact 日志没有输出合成匹配值本身。修复前后对照脚本报告 19/19 PASS，其中至少两个旧版 CLEAN 绕过样本的要求由 case-52、case-53 满足。

### 3.2 RV-14-T01（P2）：case-51 没有单独证明 Authorization 关键词桥接

位置：
- `tools/r0/e2e/test-scan-artifacts.js`：case-51。
- `tools/r0/e2e/scripts/rv14-binary-evidence.js`：Round 6 样本与命中日志生成部分。

当前 case-51 负载把 NUL 插在 `Authorization` 内部，但 `Bearer` 与合成 token 保持连续。旧扫描器因此已能通过 `BEARER_TOKEN` 命中并返回 rc=2；新扫描器日志确实额外命中 `AUTH_HEADER`，说明当前实现的桥接路径有效。

问题在于测试断言是：
`assert.match(result.stdout, /(AUTH_HEADER|BEARER_TOKEN)/)`

这意味着将来即使 Authorization 桥接逻辑回归失效，只要 Bearer 规则仍然命中，case-51 仍会 PASS。证据生成脚本也只断言新扫描器退出码符合预期，没有对 RV-14-07 的必需命中类型做失败关闭断言。现有原始日志能证明本次运行同时出现 AUTH_HEADER，但 CI 自动化门禁没有把它固化为必须条件。

建议 WorkBuddy 将 case-51 改成**能隔离 Authorization 路径**的样本，并断言 `AUTH_HEADER` 必须命中；不能让连续的 Bearer/token 规则替代该断言。可保留当前样本作为组合回归，但另加一个只依赖 Authorization 关键词桥接的样本。同步让 `rv14-binary-evidence.js` 对 RV-14-07 必需命中类型作显式断言。不要削弱 case-52/53 或正常 PNG CLEAN 断言。

**状态：RV-14 的扫描行为有实测证据，但测试覆盖闭环仍 PARTIAL；该项是 P2 测试质量问题，不是本轮发现扫描器对当前样本放行。**

### 3.3 正常 PNG 不误报控制

最新 Artifact 的 case-46 与 case-47 均通过：
- 正常 PNG，包含 4 KiB 高熵尾部：CLEAN（rc=0）。
- 合法 PNG 内含普通业务文字：CLEAN（rc=0）。

原有两个 CLEAN 控制未被删除或改成宽松的文件名/格式检查。

## 4. RV-13：PASS

独立下载最新 Artifact `13521151959` 并检查 ZIP 内容：
- ZIP 共 102 个文件条目。
- 仅存在一份根目录 `SHA256SUMS.txt`，共 101 条记录。
- 逐条按 ZIP 内实际文件字节重新计算 SHA256：101/101 匹配。
- missing=0、mismatch=0、uncovered=0、nested manifests=0。
- 根清单自身不列入清单，作为唯一例外。
- Artifact 文件列表未发现 trace.zip、HAR、DB/SQLite 文件。

因此 RV-13 在最新 Artifact 上继续 PASS。

## 5. RV-15：PASS（本轮正向回归，负向证据沿用）

最新 HEAD 上的正向 gate run `42251849969` success，真实 prepare、scan、fail-closed 检查与 `actions/upload-artifact@v4` 上传步骤均 success，Artifact `13521470784` 已生成。

前轮在真实 workflow 中已验证三种负向前置失败：scan failure、fail_closed_check failure、prepare failure；每种情况下真实 upload step 均 skipped，Artifact API 数量为 0。当前本轮仅修改测试文件、证据文件和 workflow 注释/显示名称，没有改上传门禁逻辑，因此负向证据仍适用；但本轮没有重新执行三种负向场景，不应表述为本轮重跑。

## 6. R0 其他阻塞项仍未关闭

最新 Artifact 与 PR 交付记录仍表明：
- **A05 仍是假阳性 PASS**：报告只记录总耗时 1076ms（含 800ms 链路延迟）及中途按钮文案「登录中…」，没有断言 pending 时按钮 disabled。A01 有自己的 pending/disabled 断言，不能替代 A05 在慢网络场景的断言。
- **C 仍为 164 PASS / 4 FAIL / 59 BLOCKED / 1 N/A**，合计 228。CI 中 C 是报告型步骤，step success 不等于 228 项通过。
- **OBS-ACCT-VIS-390 仍 FAIL**：最新 Artifact 的 `C/I-ACCT/visibility-observation-independent.json` 明确 `closed=false`、`visibleInnerText=false`、`firstTdDisplay=none`；390×844 下账户名称视觉可见性未闭环，待 Owner 裁决。本轮未改 CSS，符合范围约束。
- **B 的 t021 分类 override 仍待 Owner 决策**，不能由 WorkBuddy 或审阅者替 Owner 裁决。
- **RV-05 仍 Owner ACTION REQUIRED**：旧 SHA 的公开 Git 历史/仓库可见性事项没有因本轮安全扫描器整改而自动解决。
- PR #3 仍 OPEN、未合并；R0 继续 BLOCK，不授权 R1–R7 或 PWA。

## 7. 本轮结论与下一步

| 项目 | 独立复核结论 |
|---|---|
| RV-13 | PASS |
| RV-14 | PARTIAL：当前样本扫描行为通过；case-51 必须强化为 AUTH_HEADER 专项断言 |
| RV-15 | PASS：前轮真实负向门禁证据仍有效；本轮正向回归成功 |
| A/B/C/D | 仍 PARTIAL，具体阻塞见 §6 |
| RV-05 | Owner ACTION REQUIRED |
| R0 | **BLOCK** |
| PR #3 | OPEN / 未合并 |

### 给 WorkBuddy 的下一轮最小任务

仅修复 RV-14-T01：
1. 新增一个合法 PNG 魔数 + Authorization 关键词内部 NUL 的隔离样本，确保该样本不能仅靠连续 Bearer/token 检测通过。
2. 真实调用当前 `scan-artifacts.js`，要求退出码 2，并明确断言 `AUTH_HEADER` 命中；不得使用 OR 断言让 `BEARER_TOKEN` 替代。
3. `rv14-binary-evidence.js` 对 RV-14-07 同样断言 `AUTH_HEADER`，保留旧版 rc/hit 记录，不夸大旧版绕过结论。
4. 保留 case-52/53 的 `SENSITIVE_ASSIGN` 断言、case-46/47 CLEAN 断言；运行完整 53+ 项扫描器测试、证据脚本与真实 CI。
5. 只改测试和证据，不改扫描器本体、RV-13/RV-15、workflow 上传条件、业务代码、PRD/WBS/DB/API/CSS、A05、C 控件状态或 t021。
6. 不合并 PR，不启动 R1–R7/PWA；交付后停止等待第三方复核。

本报告是对 PR #3 本轮补充交付的独立复核记录，不构成 R0 放行或合并授权。
