# FastLedger R0 第十二次独立复核：RV-14-T01 Authorization 检测路径隔离验证

- 复核日期：2026-11-08
- 仓库：`LinTia/FastLadger`
- PR：[#3](https://github.com/LinTia/FastLadger/pull/3)
- PR 分支：`fix/r0-sixth-runtime-defects`
- 复核 HEAD：`4318fd612c856218f6c903b7e3f3e5b5257cfa3a`
- 本轮提交：`4318fd612c856218f6c903b7e3f3e5b5257cfa3a`（测试、证据日志与 workflow 注释/step 名称）
- 结论：**RV-14-T01 PASS；RV-13 PASS；RV-15 本轮正向回归 PASS（负向证据沿用前轮真实 workflow 记录）；R0 仍 BLOCK。**
- PR 状态：OPEN、未合并。

## 1. 复核范围与证据边界

本轮只复核 WorkBuddy 对第十一次复核 RV-14-T01 的补充，重点确认：
1. case-51 不再允许 BEARER_TOKEN 替代 AUTH_HEADER；
2. 新增 case-54 是否真正隔离 Authorization 关键词桥接路径；
3. 测试是否调用真实扫描器，而非只检查 helper 的模拟结果；
4. 正常 PNG CLEAN 控制、RV-13 清单、RV-15 上传门禁是否回归；
5. CI Artifact 是否与 PR 最新 HEAD 对应，且 Artifact 内证据完整。

本轮通过 GitHub 远端源码、commit diff、Actions jobs 与实际下载的 Artifact 独立检查。**没有在本地重新执行 Node 测试或 Go 测试**；因此本文不把 CI 中 WorkBuddy 提交的测试执行表述为本审阅者本地复跑。CI 运行记录和 Artifact 原始日志是独立读取、核验的交付证据。

## 2. 最新提交与修改范围

PR #3 当前 HEAD 为 `4318fd612c856218f6c903b7e3f3e5b5257cfa3a`，base 为 main，PR OPEN、未合并。

本次提交改动 5 个文件：
- `tools/r0/e2e/test-scan-artifacts.js`：收紧 case-51，新增 case-54。
- `tools/r0/e2e/scripts/rv14-binary-evidence.js`：对 RV-14-07 和 RV-14-10 增加必需/禁止命中类型断言。
- `.github/workflows/r0-e2e.yml`：只更新 54 项测试说明与 step 显示名称；本次 diff 未修改上传 step 的 `if:`、`uses:` 或上传条件。
- `docs/r0-evidence-9th/scan-tests-round5.log`：更新为 54 项测试结果。
- `docs/r0-evidence-9th-fix/rv14-binary-evidence.txt`：更新修复前后对照和 24 项断言日志。

扫描器本体 `tools/r0/e2e/scripts/scan-artifacts.js` 未在本次提交中修改。此次属于测试覆盖与证据收口，不是再次修改扫描器实现。

## 3. RV-14-T01：PASS

### 3.1 case-51 强制命中 AUTH_HEADER

旧断言为：

`assert.match(result.stdout, /(AUTH_HEADER|BEARER_TOKEN)/)`

该 OR 断言允许 Authorization 桥接失效时，由连续 Bearer/token 规则替代命中。

最新代码已改为：

`assert.match(result.stdout, /AUTH_HEADER/, ...)`

因此 case-51 的通过条件现在要求明确出现 AUTH_HEADER，不再接受 BEARER_TOKEN 单独替代。

### 3.2 case-54 隔离样本有效

新增 case-54 的输入由真实 PNG 魔数及 tEXt 负载构造，负载核心为：

`Authori[NUL]zation: <synthetic-token>`

隔离条件：
- 负载不包含连续的 Bearer 前缀；
- 不包含可被 BEARER_TOKEN 规则捕获的连续 Bearer/token 串；
- `Authorization` 关键词内部插入 NUL；
- 测试真实调用 `scan-artifacts.js`，要求退出码 2；
- 必须命中 `AUTH_HEADER`；
- 明确禁止命中 `BEARER_TOKEN`。

静态检查扫描器实现路径：
- 已知二进制文件不再跳过噪声归一化扫描；
- `extractTextFragments()` 对可打印片段桥接最多 4 个连续噪声字节；
- 片段含 `authoriz` 锚点后进入 `scanBinaryFragment()`；
- `scanBinaryFragment()` 调用 `scanText()`，Authorization 赋值规则由此命中；
- case-54 的隔离断言不允许 BEARER_TOKEN 作为替代命中。

最新 Artifact 的 RV-14 原始日志进一步显示：
- 修复前基线 `2699c9c`：case-54 为 CLEAN，rc=0；
- 修复后扫描器：case-54 为 BLOCKED，rc=2；
- 实际命中类型仅为 `AUTH_HEADER`；
- 未命中 `BEARER_TOKEN`；
- 合成匹配值没有出现在扫描输出中。

因此本轮不只是“测试断言改严”，而是有隔离样本、真实扫描器输出、旧版绕过对照三者互相印证。**RV-14-T01 关闭。**

### 3.3 其他 RV-14 回归控制

- case-52：x-api-key 关键词内部 NUL，仍要求 `SENSITIVE_ASSIGN`。
- case-53：x-api-key 值内部 NUL，仍要求 `SENSITIVE_ASSIGN`。
- case-46：正常 PNG（含高熵压缩数据）仍 CLEAN。
- case-47：合法 PNG 内普通业务文字仍 CLEAN。
- 修复前后对照日志显示至少 6 个阻断样本在旧扫描器上为 CLEAN，修复后按预期 BLOCK；本轮对照脚本 24/24 PASS。

没有发现通过删除正常 PNG CLEAN 控制、放宽断言或将所有二进制一律 BLOCK 来制造通过。

## 4. CI 与 Artifact 独立核验

### 4.1 Actions 运行

| 工作流 | Run | 核验结果 |
|---|---:|---|
| R0 E2E | [42262665584](https://github.com/LinTia/FastLadger/actions/runs/42262665584) | success；54 项扫描器测试 step success；RV-13/RV-14 证据生成 success；上传前扫描 success；Artifact 上传 success |
| CI build/vet/test/race + 容器冒烟 | [42262672385](https://github.com/LinTia/FastLadger/actions/runs/42262672385) | 两个 jobs 均 success |
| RV-15 gate / positive | [42262680160](https://github.com/LinTia/FastLadger/actions/runs/42262680160) | success；真实 prepare、scan 与 upload-artifact 正向步骤 success |

R0 E2E Artifact API 元数据确认：
- Artifact ID：`13525746546`
- 名称：`r0-e2e-evidence-4318fd612c856218f6c903b7e3f3e5b5257cfa3a`
- 大小：3,642,854 bytes
- 生成时间：2026-11-06T17:35:37Z
- `expired=false`
- Artifact workflow_run.head_sha 与 PR HEAD 完全一致：`4318fd612c856218f6c903b7e3f3e5b5257cfa3a`

### 4.2 Artifact 内容

实际下载并检查 ZIP：
- 共 102 个条目：101 个证据文件 + 1 份根目录 `SHA256SUMS.txt`。
- 根清单包含 101 条文件记录。
- 逐条按 ZIP 内实际文件字节重新计算 SHA256：**101/101 匹配**。
- missing=0、mismatch=0。
- 仅有一份根清单，无嵌套清单。
- 未发现 trace.zip、HAR、.network、DB/SQLite 或 trace 归档文件。

原始日志核验：
- `scan-tests-round5.log`：54 tests、54 pass、0 fail、0 skipped。
- `rv14-binary-evidence.txt`：24 项断言、24 PASS、0 FAIL；case-51 与 case-54 均有必需 AUTH_HEADER 断言；case-54 另有禁止 BEARER_TOKEN 断言。
- case-54 的原始扫描器 stdout 只记录 `AUTH_HEADER` 命中，不输出合成 token。

## 5. RV-13 / RV-15 状态

- **RV-13：PASS。** 最新 Artifact 根清单 101/101 字节校验匹配，0 missing / 0 mismatch / 0 嵌套清单；本轮未发现回归。
- **RV-15：本轮正向回归 PASS。** 最新正向 gate run 的真实上传步骤 success。前轮真实 workflow 负向场景（scan / fail_closed / prepare 失败时 upload skipped、Artifact API 数量为 0）仍可作为当前门禁未变更情况下的既有证据；**本轮没有重新执行三种负向 workflow 场景**，不把它们写成本轮新跑结果。
- 本轮提交只改测试、证据与 workflow 注释/step 名称，没有改上传条件，因此前轮负向证据没有因本轮改动失效的迹象。

## 6. R0 其他阻塞项仍在

最新 Artifact 的 A/D/C 报告与历史 Owner 事项显示，以下问题并未因 RV-14-T01 关闭而自动解决：

- **A05：仍有断言缺口。** 最新 A05 只记录总耗时及中途按钮文案「登录中…」，没有断言慢网 pending 时按钮 `disabled=true`。A01 的断言不能替代 A05 慢网场景。
- **C：164 PASS / 4 FAIL / 59 BLOCKED / 1 N/A**，合计 228。C 是报告型 CI step，step success 不等于 228 条全部通过。
- **OBS-ACCT-VIS-390：仍 FAIL。** 最新观察文件记录 390×844 下 `closed=false`、`visibleInnerText=false`、`firstTdDisplay=none`；待 Owner 裁决，不能擅自改 CSS 或改判 PASS。
- **B / t021 分类 override：仍待 Owner 决策**，不得由 WorkBuddy 或审阅者代替 Owner 裁决。
- **RV-05：Owner ACTION REQUIRED。** 旧 SHA 公开 Git 历史与仓库可见性问题未因扫描器整改而解决。

所以本轮只关闭 RV-14-T01，不代表 R0 通过。

## 7. 结论与下一步

| 项目 | 第十二次独立复核结论 |
|---|---|
| RV-14-T01 | **PASS / FIXED** |
| RV-14 本轮指定样本 | PASS：case-51 强制 AUTH_HEADER；case-54 隔离命中 AUTH_HEADER 且不命中 BEARER_TOKEN |
| RV-13 | PASS |
| RV-15 | 本轮正向回归 PASS；负向证据沿用前轮真实 workflow 记录 |
| A/B/C/D | 仍有既有缺口，见 §6 |
| RV-05 | Owner ACTION REQUIRED |
| R0 | **BLOCK** |
| PR #3 | OPEN / 未合并 |

### 给 WorkBuddy 的指令

**RV-14-T01 不需要继续修改。** 本轮已由独立复核关闭，不要再重复改 case-51、case-54、证据脚本或扫描器。

接下来不要因为 RV-14-T01 PASS 就合并 PR #3 或启动 R1–R7/PWA。应等待 Owner 处理 RV-05、t021 与 OBS-ACCT-VIS-390 等事项，并按现有 R0 阻塞清单继续推进 A05 / C 等未关闭项；每项须单独提供可复核证据，不得把 CI job success 当作整套 R0 PASS。

本报告只关闭 RV-14-T01，不构成 R0 放行、PR 合并或进入后续阶段授权。
