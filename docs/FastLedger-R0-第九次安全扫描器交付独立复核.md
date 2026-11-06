# FastLedger R0 第九次安全扫描器交付独立复核

- 审查日期：2026-11-06
- 审查对象：WorkBuddy PR #3，分支 `fix/r0-sixth-runtime-defects`
- PR 当前 HEAD：`2699c9c3db2e40915082da2acbe2392ad40650de`
- 扫描器代码提交：`f1ddc7df810e04250aa421b467fa31ebb1186375`
- CI 工作流提交 / 本轮 R0 E2E head：`5519b9deaae623e6d851c76ee90c31da46743dfd`
- R0 E2E：run `42147186477`，job `127661404203`，success
- Artifact：`13471225181`，102 个条目，3,652,230 bytes
- 当前 main HEAD：`416f0bf183840899fb5b1b282f52559d1fb786db`
- 方法：GitHub REST/Contents 读取 PR、提交、CI job/steps；下载并解压 Artifact，逐项核对 SHA256SUMS；静态复核扫描器、上传准备脚本、workflow、A/C 测试代码。未在本轮本机执行 Node 扫描器，因此静态发现明确标注，不冒充运行时复现。

## 1. 总结结论

WorkBuddy 对 RV-07～RV-13 的整改有实质进展，但**本轮仍不能验收关闭，PR #3 不应合并，R0 继续 BLOCK**。

- RV-07：PEM 明文标记规则已加入，两个白名单 TXT 全链路测试已加入；代码修复成立，测试类型覆盖仍不完整。
- RV-08：实际上传 step 已同时依赖 `artifact_scan` 与 `fail_closed_check` 成功；但所谓负向集成测试没有真正驱动 GitHub Actions 上传 step，也没有读取 step outcome 或核对 Artifact 数量，故验证证据 PARTIAL。
- RV-09：严格 UTF-8 fatal 解码已加入；但已知二进制文件不执行噪声复扫，仍存在一个可由 PNG 魔数文件触发的静态漏检路径（RV-14）。
- RV-10：Base64URL 字符集及 padding 归一化已加入，三项样例测试存在。
- RV-11：新增 token 值内部 NUL 样例，修正有效。
- RV-12：新增 random.txt 的 prepare→scan 全链路样例，修正有效。
- RV-13：根级 SHA256SUMS 已重建，但嵌套目录原有清单仍保留旧引用，且根清单把所有同名清单都排除，未校验它们；本次下载 Artifact 已直接证实未闭环。
- A/B/C/D 仍均为 PARTIAL；RV-05 仍为 Owner ACTION REQUIRED。

## 2. PR / CI / 分支基线核对

1. PR #3 状态为 OPEN、merged=false；没有合并。
2. PR 分支当前 HEAD 是 `2699c9c3...`。该提交相对 `5519b9d` 只有 `docs/r0-evidence-7th-fix/p1/k1-k3-remediation.md` 一份文档修改（122 additions / 57 deletions），没有改变本轮代码。
3. 扫描器修复提交是 `f1ddc7d`；workflow 加强提交是 `5519b9d`。run `42147186477` 的 head_sha 是 `5519b9d`，不是 PR 最终文档 HEAD `2699c9c`。应准确表述为“代码与 workflow 在 5519b9d 上完成 CI；其后 2699c9c 仅文档回填”。
4. run `42147186477` 的 20 个主要 steps 均 success，包括 41 项扫描器测试、修复前后对照、A、D、C 报告、上传前扫描、fail-closed 正向/负向 step、Artifact upload。CI success 不代表 C 通过。
5. PR 的 base SHA 仍是旧基准 `dadc9a3...`；当前 main 已前进到 `416f0bf...`。GitHub compare 显示分支相对当前 main **ahead 13 / behind 6 / diverged**。未来若要合并，必须先处理与当前 main 的分叉；本报告不授权更新分支或合并。

## 3. RV-07～RV-13 逐项复核

| Finding | 复核结果 | 结论 |
|---|---|---|
| RV-07 PEM 明文私钥 | `PEM_PRIVATE_KEY_RE` 已在 `scanText()` 直接匹配明文 PEM 头；case-38/39 分别用 `private-key.txt`、`export.txt` 走 prepare→scan | **代码修复成立**；CI 全链路只覆盖 RSA / OPENSSH 两种样例，其他类型应补矩阵 |
| RV-08 fail-closed | workflow 的 upload 条件已同时检查 scan 与 fail_closed_check outcome；正向验证严格要求 credential rc=2、clean rc=0 | **门禁代码成立，负向集成证据不足**，见 RV-15 |
| RV-09 非法 UTF-8 | `decodeUtf8()` 使用 `TextDecoder('utf-8',{fatal:true})`；无可靠编码文本按 UNDECODABLE 阻断；case-31～33 覆盖非法字节插入 | **文本路径修复成立**；已知二进制路径仍有 RV-14 |
| RV-10 Base64URL | 正则包含 `-`、`_`；解码前归一化并补 padding；case-34～36 覆盖有/无 padding | **修复成立** |
| RV-11 NUL 位置 | case-20 保留分隔符 NUL，case-37 新增 token 值内部 NUL | **修复成立** |
| RV-12 白名单链路 | case-40 使用 `random.txt`，先 prepare 再 scan，断言文件进入目标目录并被 BLOCK | **修复成立** |
| RV-13 SHA256SUMS | 根清单重建，但子目录清单未重建/删除，且根清单排除了所有 basename 为 SHA256SUMS.txt 的文件 | **未关闭**，见 §4 |

### RV-13 直接 Artifact 字节核验

实际下载并解压 Artifact `13471225181`，共 102 个条目，其中有 4 份 `SHA256SUMS.txt`（根级 1 份 + A/D/C 子目录各 1 份）。

| 清单 | 记录数 | 实际匹配 | 缺失 | 不匹配 |
|---|---:|---:|---:|---:|
| 根级 `SHA256SUMS.txt` | 98 | 98 | 0 | 0 |
| A 子目录清单 | 30 | 18 | **12** | 0 |
| D 子目录清单 | 46 | 46 | 0 | 0 |
| C 子目录清单 | 34 | 34 | 0 | 0 |

A 子目录缺失的 12 条均指向 A01–A08、A09 admin/member/owner、A10 中已被白名单过滤的 `trace.zip`。这些 trace 文件没有进入 Artifact，安全过滤本身有效，但子清单仍保留旧引用。

根因定位：
- `tools/r0/e2e/scripts/prepare-upload.js:269`：`manifestRels = collected.filter((rel) => path.basename(rel) !== 'SHA256SUMS.txt')` 会排除所有层级的同名清单，而不只是根级目标清单。
- 同文件约 L288：校验循环再次按 basename 跳过所有 `SHA256SUMS.txt`。
- 脚本只重写 `dest/SHA256SUMS.txt`；源目录中 A/D/C 子目录的旧清单作为普通白名单 TXT 复制，内容没有被更新。
- 因此根级 98 条清单未覆盖 Artifact 中的 3 份子清单，且 A 子清单仍有 12 条不存在的 trace 引用。

影响：Artifact 的完整性清单不是对最终 102 个条目的完整校验；报告中的“Artifact 全部 SHA256SUMS 完整校验 98/98”范围过窄。

修复要求：
1. 只排除根级目标文件 `dest/SHA256SUMS.txt`，不能按 basename 排除所有层级。
2. 选择一种单一口径：删除上传目录中的旧嵌套清单并仅保留根清单；或逐个重建嵌套清单，并确保根清单也对所有嵌套清单文件做哈希。
3. 上传前校验必须遍历最终 Artifact 的全部文件，除根清单自身外 0 missing / 0 mismatch；不能把被排除文件的旧引用留在嵌套清单。
4. 将 case-41 扩展为多层目录：A 子目录含旧 SHA256SUMS + trace.zip，D/C 子目录含旧清单；执行 prepare 后校验每一份清单及根清单，并断言最终上传目录所有文件均被根清单覆盖（根清单自身除外）。

## 4. 新发现

### RV-14 · P1：已知二进制文件跳过噪声复扫，PNG 魔数可绕过 NUL / 非法字节断裂检测

**状态：静态代码路径确认，尚未由本轮独立运行复现；必须补负向样例后再关闭。**

位置：
- `tools/r0/e2e/scripts/scan-artifacts.js:505`：`if (!knownBinary)` 才执行噪声字节剔除复扫。
- 同文件 `scan-artifacts.js:517–518`：`UNDECODABLE` 阻断同样排除 `knownBinary`。
- 同文件 `scan-artifacts.js:250–285`：文本规则需要连续匹配 Bearer/JWT/敏感键名。

问题与根因：
- 扫描器对所有文件执行 latin1 扫描，但只有 `!knownBinary` 时才剔除 NUL/非可打印字节并再次扫描。
- 对以已知 PNG 魔数开头的 `.png`，`knownBinary=true`；只要文件整体仍是合法 UTF-8（NUL 本身是合法 UTF-8 字节），`u8.reliable=true`，就不会进入 UNDECODABLE。
- 若 Bearer token 值在前 12 个可匹配字符内插入 NUL，原始 latin1/UTF-8 文本中的 token 会被截断，噪声复扫又因 knownBinary 被跳过；PNG 魔数检查只校验开头签名，不验证完整 PNG 结构。该路径可能以 CLEAN 结束。

最小复现建议：
1. 生成一个以 PNG 签名开头、扩展名为 `.png` 的样本。
2. 在内容中放入 `Bearer SYN\0THETICFAKE...`，NUL 位于 Bearer 值前 12 个字符内。
3. 经 `prepare-upload.js → scan-artifacts.js` 运行，断言修复前 CLEAN、修复后 BLOCK；输出不得包含 token 值。
4. 再增加合法 PNG 文件内容中嵌入可识别凭据文本、带 NUL/非法字节的测试，避免只验证伪装扩展名。
5. 不应仅靠文件扩展名或 PNG 头 8 字节将文件排除在噪声复扫之外；应对所有文件施加安全复扫，或采用可验证的二进制解析/拒绝策略。

影响：Artifact 白名单允许 PNG，扫描器不能仅因文件有 PNG 魔数就跳过可恢复凭据字符串的噪声复扫。当前 case-37 只覆盖普通 TXT，未覆盖 knownBinary 分支。

### RV-15 · P2：fail-closed 负向测试没有真正验证 GitHub Actions upload step 被跳过

**状态：代码结构核对确认测试边界不足。**

位置：
- `tools/r0/e2e/scripts/verify-fail-closed-gate.js:55–68`：`uploadAllowed` 只是由两个退出码计算的布尔值；`--expect-skip` 在其为 false 时直接打印“Artifact 数量 = 0”并返回 0。
- `.github/workflows/r0-e2e.yml:168–178`：`fail_closed_negative` step 单独运行 broken scanner 与 helper。
- `.github/workflows/r0-e2e.yml:185–195`：真实 `artifact_upload` 的 if 条件依赖 `artifact_scan` 与 `fail_closed_check`，**不依赖** `fail_closed_negative` 的结果。

问题与根因：
- 负向测试证明的是 helper 在 broken scanner 返回 0 时会计算 `uploadAllowed=false`，并非实际触发 workflow 的 `fail_closed_check` 失败。
- 它没有读取真实 `steps.artifact_upload.outcome`，也没有核对本次运行的 Artifact 数量。
- “Artifact 数量 = 0”只是 helper 的固定说明文本，不是查询 GitHub Artifact API 得到的实测数量。
- 当前 workflow 双条件 gate 静态逻辑正确，但负向证明没有验证 gate 与实际 upload step 的集成行为。

修复要求：
1. 通过可控输入/测试 workflow/job，实际让作为 upload 前置条件的 fail_closed_check 失败。
2. 断言 upload step outcome 为 skipped，且对应 run 的 Artifact 数量为 0。
3. 同时保留正向场景：真实扫描器 credential rc=2、clean rc=0，上传正常发生。
4. 报告应区分“helper 单元测试通过”与“GitHub Actions upload step 集成测试通过”。

## 5. Artifact / A-B-C-D 真实运行结果

本轮下载的 Artifact `13471225181` 中：
- A 报告：11/11 PASS；但 A05 仍是假阳性风险。
- D 报告：39/39 PASS。
- C 报告：PASS 164 / FAIL 4 / BLOCKED 59 / N/A 1。
- 原始 `.zip/.trace/.network/.har/.db/.sqlite*` 文件均未出现在本轮 Artifact；独立常见字节模式检查未命中 JWT/Bearer/Authorization/API key/PEM 标记。该检查不是通用 DLP 保证。

### A05 仍未修复

`tools/r0/e2e/specs/a-scenarios.js:188` 仍为：
`status: elapsed >= 800 ? 'PASS' : 'FAIL'`

第 183 行读取了 `midText`，但 PASS 判定未使用它，也没有读取按钮 disabled 状态。Artifact 的 A05 detail 记录“中途按钮文案=登录中…”，但这不能证明断言真正检查了该状态。A 继续 PARTIAL，不得把 11/11 报告值当成完整验收通过。

### C 仍存在硬编码 PASS

本轮没有修改 `tools/r0/e2e/specs/c-controls.js`；此前列出的硬编码 PASS 仍在，例如：
- L163：`I-BOOKS-09 = ok('空态与新建后刷新已覆盖（空账本行为见 A07）')`
- L164：`I-BOOKS-10 = ok('删除按钮 owner 可见；非 owner 由服务端 403 兜底（A08 已验证）')`
- L193：`I-HOME-15 = ok('首屏无需滚动即可见概览与最近账单')`
- L281：`I-TXDET-11 = ok('转账对侧联动由服务端整笔语义保证（本组单笔用例）')`
这些并非该控件自身的真实交互断言，也没有可追溯的有效 coveredBy/evidenceRefs。C 的 164 PASS 不可全盘采信；需继续按第七次任务单逐条清理至少 15 条硬编码 PASS，缺证据则 BLOCKED/N/A。

### 其它未关闭项

- B：t021 的 `meal → dinner` override 仍缺 Owner 决策证据，B 保持 PARTIAL。
- D：39/39 新增缺陷回归通过，但 `OBS-ACCT-VIS-390` 在 390px 视口账户名称列仍被 CSS 隐藏，保持 FAIL；未授权修改 CSS。
- 成员页普通 member 可见“设为管理员/降级/移除成员”等管理按钮的候选缺陷仍仅登记，未在本轮授权范围修复。
- RV-05：GitHub API 当前确认仓库 `visibility=public`、`private=false`；旧 SHA 历史凭据问题仍是 Owner ACTION REQUIRED。改 Private 与历史对象清理是两件不同的事；本轮未改仓库可见性或 Git 历史。
- PR #3 与当前 main 已分叉（ahead 13 / behind 6）；未合并。

## 6. 状态矩阵与下一步

| 项目 | 当前状态 |
|---|---|
| RV-07 | 代码修复成立；PEM 类型矩阵需补 |
| RV-08 | PARTIAL：双条件 gate 存在，真实 upload skip 负向集成证据不足 |
| RV-09 | PARTIAL：文本严格解码成立；knownBinary 分支需处理 RV-14 |
| RV-10 | 已修复（本轮样例与 CI step 通过） |
| RV-11 | 已修复（case-37 补齐） |
| RV-12 | 已修复（random.txt prepare→scan） |
| RV-13 | BLOCKED：嵌套清单仍有 12 条失效引用，且未被根清单覆盖 |
| RV-14 | P1 待修复/待运行时复现 |
| RV-15 | P2 待补真正 workflow 级负向验证 |
| A / B / C / D | 均 PARTIAL |
| RV-05 | Owner ACTION REQUIRED |
| PR #3 | OPEN，未合并；与 main 分叉 |
| R0 | **BLOCK**；不授权 R1–R7 / PWA |

下一步 WorkBuddy 只需处理 RV-13、RV-14、RV-15，并保留已有 RV-10/11/12 测试；同时不得把 A05、C 硬编码 PASS、t021、OBS-ACCT-VIS-390 或 RV-05 改判为通过。完成后提交独立分支和 CI/Artifact，等待第三方复核。

**权限边界：本报告仅审计与提出整改要求，不授权合并 PR、改仓库可见性、重写 Git 历史、改 CSS、修改 PRD/WBS/DB/API/业务规则或启动 R1–R7/PWA。**
