# FastLadger R0 第十次安全扫描器整改独立复核

- 审查日期：2026-11-06
- 审查对象：PR #3，分支 `fix/r0-sixth-runtime-defects`
- 当前 PR HEAD：`2699c9c3db2e40915082da2acbe2392ad40650de`
- 本轮安全扫描器代码基线：`f1ddc7df810e04250aa421b467fa31ebb1186375`
- 本轮 workflow / Artifact 基线：`5519b9deaae623e6d851c76ee90c31da46743dfd`
- CI：R0 E2E run `42147186477`，job `127661404203`，Artifact `13471225181`
- 审查方式：GitHub 当前 PR 元数据、分支源码、workflow 文件、CI job steps/logs、Artifact 元数据及此前已下载 Artifact 的逐文件/清单复核；未在本地重新执行 Node 测试或重跑 workflow。
- 本次未修改业务代码、扫描器、workflow 或 PR；本报告独立提交到 main。

## 1. 总结结论

**本轮整改不通过；RV-13 未关闭，RV-14 / RV-15 仍需处理；PR #3 不得合并，R0 继续 BLOCK。**

当前 PR HEAD 已从上轮复核基线 `5519b9d` 前进到 `2699c9c`，但最新提交 `2699c9c` 的 commit diff 仅更新整改报告文档；安全扫描器与 workflow 的实际实现仍是 `5519b9d` 对应版本。不能把文档回填视为 RV-13/14/15 的代码修复。

CI run `42147186477` 的 20 个 step 均 success，41 项扫描器测试 step success，Artifact `13471225181` 确实生成。但 CI 全绿不代表三项验收通过：Artifact 字节级复核仍发现嵌套清单失效；PNG/NUL 反例没有测试；所谓负向“Artifact 数量=0”只是 helper 输出，真实 workflow 仍执行了上传并产生 102 个文件的 Artifact。

## 2. 本轮交付基线

| 项目 | GitHub 实际状态 | 独立判断 |
|---|---|---|
| PR #3 | OPEN，未合并 | 符合未合并边界 |
| PR 当前 HEAD | `2699c9c3db2e40915082da2acbe2392ad40650de` | 最新提交是文档回填 |
| 安全扫描器代码 | `f1ddc7d` | 本轮未看到后续扫描器代码修复 |
| workflow / Artifact 对应 head | `5519b9deaae623e6d851c76ee90c31da46743dfd` | run 与 Artifact 绑定此 SHA，不是当前文档 HEAD |
| R0 E2E | run `42147186477`，success | 20 steps success，只证明流程执行成功 |
| 扫描器负向测试 | 41 项 step success | 不覆盖本轮发现的嵌套清单、PNG/NUL、真实 workflow skip |
| Artifact | `13471225181`，102 个文件 | 实际存在，且内容校验有缺口 |
| main 与 PR 分支 | ahead 13 / behind 9 / diverged | 仍需处理分叉；本报告不做合并或 rebase |

## 3. RV-13 · P2：嵌套 SHA256SUMS 仍包含被排除文件引用

**状态：未修复，Artifact 字节级证据确认。**

### 位置与原文

文件：`tools/r0/e2e/scripts/prepare-upload.js`

- L262–269：白名单收敛后重建清单。
- L269：`const manifestRels = collected.filter((rel) => path.basename(rel) !== 'SHA256SUMS.txt');`
- L277：仅写入目标目录根路径 `path.join(dest, 'SHA256SUMS.txt')`。
- L288：校验时再次以 `path.basename(rel) === 'SHA256SUMS.txt'` 跳过清单文件。

### 根因

实现把所有目录层级中 basename 为 `SHA256SUMS.txt` 的文件全部排除在新根清单之外，但只重建目标根目录的 `SHA256SUMS.txt`。源目录内的 A/D/C 子目录清单会被白名单复制，却不会被重建；验证循环又跳过所有同名清单，因此这些旧清单既不被根清单覆盖，也不被校验。

### Artifact 实证

Artifact `13471225181`（102 个文件）复核结果：

- 根目录清单：98 条记录，98 条匹配。
- A 子目录清单：30 条记录，其中 12 条仍引用已过滤的 trace.zip，无法在最终 Artifact 中找到。
- D 子目录清单：46 条记录，匹配。
- C 子目录清单：34 条记录，匹配。
- 根清单未包含三份子目录清单自身，故根清单 98/98 不能证明嵌套清单正确。

### 影响

最终 Artifact 内仍保留指向不存在文件的校验记录。下游无法对整个 Artifact 进行一致、完整的 checksum 验证；当前 98/98 只代表根清单覆盖的文件，不代表全部清单和全部 Artifact 文件完整。

### 修复要求

1. 明确采用单一根清单，或逐目录重建子清单；禁止复制保留任何过期清单。
2. 若采用单一根清单，必须删除/排除所有源目录嵌套清单，并确保根清单覆盖最终上传目录内所有实际文件（仅允许清单自身作为明确例外）。
3. 若保留子清单，则每份都必须按过滤后的实际目录内容重建，且父级清单也应纳入或明确覆盖策略。
4. 新增真实 prepare-upload 集成测试：源目录含根与 A/D/C 子目录、每层旧 SHA256SUMS、白名单文件、被过滤 trace.zip；验证最终上传目录中没有旧清单引用。
5. 对最终准备目录递归执行完整校验，必须 0 missing / 0 mismatch；删除/篡改任一文件必须使验证失败。

### 验收标准

最终 Artifact 解压后所有保留的 checksum 清单均可完整校验；不得存在任何指向被过滤文件的记录；不得把仅根清单通过称为整体通过。

## 4. RV-14 · P1：knownBinary PNG 跳过 NUL/噪声复扫，仍存在静态漏检路径

**状态：静态路径确认，尚缺针对性运行时复现；按 P1 阻塞安全验收。**

### 位置与原文

文件：`tools/r0/e2e/scripts/scan-artifacts.js`

- L464：`const knownBinary = isKnownBinary(buf);`
- L505–508：`if (!knownBinary) { ... squeezed ... scanText(squeezed) ... }`
- L518：`if (!knownBinary && buf.length >= MIN_UNDECODABLE_BYTES && !reliableText)` 才执行 UNDECODABLE 阻断。

当前逻辑对所有文件执行 latin1 候选扫描，但 knownBinary 文件不执行剔除非可打印字节后的噪声复扫，也不会进入 UNDECODABLE 分支。

### 问题与复现方向

合法 PNG 魔数会使文件被识别为 knownBinary。若在 PNG 数据后嵌入文本凭据，并在 `Authorization` / `Bearer` 关键词或 token 值内部插入 NUL，原始 latin1 字符串中的连续关键词/值可能被打断；knownBinary 分支又跳过 squeezed 复扫，且不会因无法可靠解码而阻断。

现有 case-17 仅覆盖“合法 PNG 魔数 + 连续 JWT 文本”，并未覆盖关键词或 token 值内部 NUL；case-37 覆盖的是普通文本文件，不是 knownBinary PNG。因此现有测试不能排除此路径。

### 影响

如果该路径被真实扫描器复现，PNG 文件可通过扩展名白名单进入 Artifact，而嵌入的凭据可能逃过扫描。该风险涉及 CI Artifact 泄露，按 P1 处理。

### 修复要求

1. 新增合法 PNG 魔数样本，PNG 数据尾部嵌入合成凭据。
2. 至少分别覆盖 Authorization/Bearer 关键词内部 NUL、token 值内部 NUL、x-api-key 关键词或值内部 NUL。
3. 必须调用真实 `scan-artifacts.js`，不得仅测正则或模拟结果。
4. 每个凭据样本必须 BLOCK；正常 PNG（无凭据）必须 CLEAN，避免把所有 PNG 一律阻断。
5. 若选择不对 knownBinary 做噪声复扫，必须提供等价、可证明的二进制内容检测策略和正反例测试。

### 验收标准

以上合法 PNG 负向样本全部 BLOCK，正常 PNG 控制样本 CLEAN；CI 执行对应测试并在最终 Artifact 扫描中保持 0 命中、0 跳过。

## 5. RV-15 · P2：fail-closed 负向测试没有验证真实 workflow 跳过上传

**状态：CI 日志直接证伪“Artifact 数量=0”的结论。**

### 位置与原文

文件：`tools/r0/e2e/scripts/verify-fail-closed-gate.js`

- `--expect-skip` 分支仅根据本地变量 `uploadAllowed` 判断。
- 当 helper 判断 false 时打印：`[gate] NEGATIVE OK: 验证失败时上传被跳过，Artifact 数量 = 0`。
- helper 没有调用 GitHub Actions workflow，也没有观察 workflow 的 `artifact_upload` step outcome，更没有查询 Artifact 列表。

文件：`.github/workflows/r0-e2e.yml`

- L169 左右的 `fail_closed_negative` step 只是执行上述 helper。
- L186 的真实上传条件为：`steps.artifact_scan.outcome == 'success' && steps.fail_closed_check.outcome == 'success'`。
- 负向 helper 的成功结果不会改变实际 `fail_closed_check` step 的 outcome；实际 `artifact_scan` 与 `fail_closed_check` 仍正常通过，所以真实 upload step 仍会执行。

### CI 日志实证

run `42147186477` / job `127661404203` 日志中：

1. 负向 helper 输出 `CRED_RC=0 CLEAN_RC=0 fail_closed_passed=false upload_allowed=false`。
2. 随后打印 `NEGATIVE OK ... Artifact 数量 = 0`。
3. 紧接着实际执行 `actions/upload-artifact@v4`。
4. 日志明确显示 `With the provided path, there will be 102 files uploaded`。
5. GitHub Artifact API 返回真实 Artifact `13471225181`，其 head_sha 为 `5519b9d...`。

因此“Artifact 数量=0”只是一条 helper 自行打印的推断，不是实际 workflow 观察结果；同一次 run 中实际 Artifact 数量为 1（文件条目 102）。

### 影响

负向测试会在真实上传仍发生的情况下报告 NEGATIVE OK，造成对安全门禁的错误信心。当前双条件 upload gate 的静态条件方向正确，但没有被要求的 workflow 级负向试验证明。

### 修复要求

1. 不得在纯 Node helper 中声称“Artifact 数量=0”，除非真实查询并验证该事实。
2. 建立真实 workflow 级负向测试：让真实 `fail_closed_check` 或 `artifact_scan` 在测试运行中失败，观察同一 workflow 的 `artifact_upload` step 为 skipped。
3. 对该测试 run 查询 GitHub Artifact 列表，断言该 run 的 Artifact 数量为 0。
4. 正向测试另行证明正常情况下上传成功且 Artifact 可下载。
5. 如果 GitHub Actions 机制无法在同一 workflow 内安全注入失败条件，应明确标记 BLOCKED，并提供可复现的专用测试 workflow；不得以 helper 的布尔推断替代。

### 验收标准

至少一条真实负向 workflow run：扫描/门禁步骤失败、upload step 为 skipped、该 run 的 Artifact 数量为 0；正向 run 上传成功。两种 run 必须分别提供 run ID、job/step 结论、Artifact API 查询结果。

## 6. 已完成项与仍需保留的状态

| 项目 | 本轮判断 | 说明 |
|---|---|---|
| RV-07 PEM 私钥标记 | PARTIAL→基本修复成立 | 有 RSA / OPENSSH 两个 prepare→scan 样本；其他 PEM 类型需补边界覆盖 |
| RV-08 上传双条件 | 代码修复成立，集成验收 PARTIAL | upload 条件包含 scan + fail_closed_check；但真实 workflow 负向 skipped/Artifact=0 未证明，见 RV-15 |
| RV-09 非法 UTF-8 | PARTIAL | strict UTF-8 已落代码；knownBinary 路径不受 UNDECODABLE 兜底约束，需与 RV-14 一并核验 |
| RV-10 Base64URL | 修复成立 | case-34~36 覆盖 padding / 无 padding 及字段值 |
| RV-11 token 内部 NUL | 修复成立 | case-37 与 case-20 分开覆盖 |
| RV-12 白名单上传链路 | 修复成立 | case-40 使用 random.txt 经 prepare→scan |
| RV-13 checksum | 未修复 | case-41 只测根清单 + 单层文件，无嵌套目录/子清单 |
| RV-14 PNG/NUL | 未修复 / P1 | 当前代码跳过 knownBinary 噪声复扫，缺少反例测试 |
| RV-15 workflow 负向验证 | 未修复 / P2 | 日志显示 helper 报“0 Artifact”，实际同 run 上传 102 文件 |

## 7. R0 其它未关闭项（不得因本轮安全扫描器 CI 绿色而改判）

- A：11/11 CI step success，但 A05 仍只按 elapsed 判 PASS，未断言 pending 文案/disabled，A 保持 PARTIAL。
- B：分类审计 22 笔已执行；t021 override 仍待 Owner 裁决，B 保持 PARTIAL。
- C：228 项结果为 PASS 164 / FAIL 4 / BLOCKED 59 / N/A 1；报告型 step success 不等于 C 通过，C 保持 PARTIAL。
- D：39/39 缺陷回归门禁通过，但 `OBS-ACCT-VIS-390` 390px 账户名称不可见仍 FAIL，D 保持 PARTIAL。
- RV-05：仓库 Public 与旧 SHA 凭据历史问题仍是 Owner ACTION REQUIRED；WorkBuddy 不得自行修改仓库可见性、删除 refs 或重写历史。
- PR #3：OPEN、未合并；main 与 PR 分支 diverged。不得据此报告建议合并。
- R0：继续 BLOCK；不授权 R1–R7，不启动 PWA。

## 8. 给 WorkBuddy 的下一步

只处理 RV-13、RV-14、RV-15：

1. 修复 RV-13 的嵌套清单策略，并增加多层目录集成测试。
2. 修复或安全封闭 RV-14 的 knownBinary PNG/NUL 复扫路径，并增加真实 PNG 正反例。
3. 重做 RV-15：真实 workflow 负向运行，证明 upload skipped 且 Artifact API 数量为 0；helper 不得再把本地布尔推断表述为真实 Artifact 结果。
4. 在 PR #3 原分支提交，提供代码 HEAD、CI run、job/step、Artifact ID/Artifact 列表、下载后完整 checksum 结果。
5. 继续保留 A05、C、t021、OBS-ACCT-VIS-390、RV-05 等原状态；不改 PRD/WBS/业务规则/DB/API/CSS，不合并 PR，不改仓库可见性或 Git 历史，不启动 R1–R7/PWA。
6. 完成后停止，等待第三方独立复核。

**最终判定：RV-13 未关闭；RV-14 P1、RV-15 P2 未关闭；PR #3 不可合并；R0 BLOCK。**
