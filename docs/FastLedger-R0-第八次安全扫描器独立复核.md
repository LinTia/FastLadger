# FastLedger R0 第八次安全扫描器独立复核

- 审查日期：2026-11-06
- 审查对象：PR #3 `fix/r0-sixth-runtime-defects`
- 审查基线：WorkBuddy K1–K3 修复提交 `c95ff094300e6e629886edfa6a3a61161dfa4351`
- 对照基线：`659c0c390c3ea392980999069b92a23bbfbcd490`
- 目标分支：`main`（本报告独立记录，不合并豆包分支或 WorkBuddy PR）
- 方法：GitHub 源码静态审查、测试源码与 CI step 状态核对；未在本地执行 Node/浏览器，不把 WorkBuddy 自述或未完成 CI 当成独立运行证据。

## 1. 总结结论

WorkBuddy 已针对 K1–K3 做出实质性改进：

- 上传前扫描器新增 UTF-8 / UTF-16LE / UTF-16BE 解码、多编码复扫、NUL 去除复扫、敏感键名联合判断和 Base64 解码复扫。
- 新增 case-18~30 共 13 项负向测试；仓库归档的本地日志显示 30/30 通过。
- GitHub Actions run `42079445817` 中「凭据扫描器负向合成测试（K1-K3，30 项）」step 已成功。
- 普通 CI run `42079445701` 已成功，包含全仓 `go test -race` 与 Docker/healthz 检查。

但本轮不能验收关闭：

1. 静态审查发现一个新的明文 PEM 私钥漏检路径。
2. UTF-8 解码的宽松可靠性阈值仍允许通过少量非法字节破坏凭据关键词。
3. Base64URL（URL-safe Base64）没有被当前 Base64 正则覆盖。
4. CI 的 fail-closed 集成验证并未成为 Artifact 上传的必要条件，且把所有非零退出码都当成凭据命中。
5. 两项关键测试样本与真实上传白名单链路不匹配。

**判定：K1–K3 主体修复 PARTIAL；P1 安全验收仍 BLOCK。PR #3 不建议合并。R0 继续 BLOCK，不授权 R1–R7/PWA。**

## 2. WorkBuddy 已完成且可确认的部分

| 项目 | 独立核对结果 | 边界 |
|---|---|---|
| UTF-16LE / UTF-16BE 解码 | 代码已加入 | 测试覆盖无 BOM 的 ASCII 凭据；尚不能代表所有编码损坏形态 |
| NUL 复扫 | 代码已加入 | case-20 实际把 NUL 放在 `Bearer ` 与值之间，不是在 token 值内部 |
| 短 Token / API Key / password | 已有测试 | 仅覆盖三个短值样例 |
| x-api-key / apiKey / API_KEY | 已有测试 | 尚未覆盖所有 header / JSON 序列化形态 |
| 标准 Base64 | 已有测试 | 未覆盖 Base64URL |
| 无法可靠解码阻断 | 代码已加入 | case-21 使用 `.bin`，该扩展名不会通过当前上传白名单 |
| CI 测试门禁 | step 8 success | 只证明 30 项测试在该 run 中通过，不代表后续上传扫描与 Artifact 检查完成 |

## 3. 新发现

### RV-07 · P1：上传白名单允许伪装成 TXT 的 PEM 私钥，但扫描器没有明文 PEM 标记规则

**状态：静态代码路径确认；需补合成端到端测试。**

位置：

- `tools/r0/e2e/scripts/prepare-upload.js`：L37（`ALLOWED_EXT`），L117–134（`classify`）。
- `tools/r0/e2e/scripts/scan-artifacts.js`：L102（`DECODED_CRED_MARKER`），L143 起（`isBase64Credential`），L218–265（`scanText` 规则）。

证据：

- 上传白名单允许 `.txt`、`.md`、`.json`、`.log`、`.png`。
- 文件名黑名单仅检查 `token|secret|credential`；例如 `private-key.txt` 不命中。
- `DECODED_CRED_MARKER` 虽包含 `BEGIN PRIVATE`，但该标记只在 Base64 解码结果中检查。
- `scanText()` 没有直接匹配明文 `-----BEGIN PRIVATE KEY-----`、`-----BEGIN RSA PRIVATE KEY-----`、`-----BEGIN OPENSSH PRIVATE KEY-----` 等 PEM 头的规则。

问题：

将 PEM 私钥内容保存为 `private-key.txt` 或 `key-export.txt`，会通过扩展名白名单；文件名不含 token/secret/credential；明文 PEM 头不触发现有规则。Base64 私钥主体解码为二进制 DER，也不保证命中当前凭据标记。因此存在扫描 CLEAN 的静态路径。

影响：

Artifact 可能公开上传私钥材料。该风险高于普通 API token 漏检。

修复要求：

1. 在明文文本规则中直接识别 PEM 私钥/证书私钥标记，至少覆盖 PRIVATE KEY、RSA PRIVATE KEY、EC PRIVATE KEY、OPENSSH PRIVATE KEY、ENCRYPTED PRIVATE KEY。
2. 对命中一律 BLOCK，不输出密钥内容。
3. 新增合成 PEM 样本，必须经过完整 `prepare-upload.js → scan-artifacts.js` 链路。
4. 测试 `private-key.txt`、普通名称 `export.txt`，不能只测试 `.pem` 扩展名（它本来就不在上传白名单）。
5. 负向断言必须证明上传目录包含该文件、扫描返回 BLOCKED、Artifact upload 不执行。

验收：上述所有 PEM 样本均 BLOCK；正常 Markdown 中对 PEM 的安全说明如确需保留，应提供明确的脱敏/占位规则，不得允许真实密钥。

### RV-08 · P1：fail-closed 集成验证失败时，Artifact upload 仍可能执行

**状态：workflow 条件静态确认。**

位置：`.github/workflows/r0-e2e.yml`：

- L136：`fail_closed_check` step。
- L148 附近：只判断 `CRED_RC == 0` 才失败，即任何非零退出码都被当作“凭据已阻断”。
- L172–173：`artifact_upload` 的条件只检查 `steps.artifact_scan.outcome == 'success'`，没有检查 `steps.fail_closed_check.outcome`。

问题：

1. 集成验证 step 即使失败，上传 step 的条件仍可能满足；`always()` 使后续 step 在前序失败时仍可执行。
2. `CRED_RC != 0` 把扫描器错误退出码 1 也当成命中成功；只有退出码 2 才能证明扫描器识别并阻断了样本。
3. 报告称该集成验证证明“失败关闭链路”，但 workflow 没有把该验证成功作为上传前置条件。

修复要求：

1. 凭据样本必须严格断言 `CRED_RC == 2`；`1`、`0` 均失败。
2. 干净样本必须严格断言 `CLEAN_RC == 0`。
3. `artifact_upload` 必须同时依赖 `artifact_scan.outcome == 'success'` 与 `fail_closed_check.outcome == 'success'`。
4. 新增集成测试：故意令 fail-closed 验证失败，断言 upload step 为 skipped、Artifact 数量为 0。
5. CI Summary 同时记录 prepare / scan / fail-closed / upload 四个 outcome。

验收：任何扫描器测试或 fail-closed 集成验证失败，都不能产生 Artifact。

### RV-09 · P1：少量非法 UTF-8 字节可破坏关键词，但仍通过“可靠 UTF-8”阈值

**状态：基于解码判定逻辑的静态确认；需补合成复现。**

位置：

- `tools/r0/e2e/scripts/scan-artifacts.js`：L176 起 `decodeUtf8()`。
- L488 附近：`reliableText = u8.reliable || u16le.reliable || u16be.reliable`。
- L218–265：文本规则依赖完整关键词匹配。

证据：

`decodeUtf8()` 以 U+FFFD 占比低于 2% 作为“可靠”标准。少量非法字节可使 UTF-8 解码结果仍被认定可靠，但若非法字节插入 `Bearer` 关键词内部，Bearer 正则会失配。此时：

- latin1 视图中的关键词也被非法字节打断；
- UTF-8 视图中关键词变为带 U+FFFD 的字符串；
- 文件仍因 U+FFFD 占比低于 2% 而被视为可靠；
- `UNDECODABLE` 不触发；
- 若值是非 JWT 的不透明 token，其他规则未必命中。

修复要求：

1. 对安全扫描不应以“少量替换字符”作为允许继续 CLEAN 的充分条件。
2. 优先采用严格 UTF-8 解码（fatal），任何解码错误都应进入其他明确编码识别或 BLOCK，而不是忽略错误继续 CLEAN。
3. 增加合成反例：在 Bearer 关键词中插入非法 UTF-8 字节，保证其余文件足够长使替换比例低于当前 2% 阈值；修复前应能复现 CLEAN，修复后必须 BLOCK。
4. 另测非法字节插入 token 值中间、字段名中间和不同位置。
5. 不得用扩大关键词正则或简单降低阈值来代替严格解码策略。

验收：任何含非法字节且不能被严格识别为受支持编码的文本文件不得 CLEAN。

### RV-10 · P2：Base64URL 变体未覆盖

**状态：正则范围静态确认；需补合成测试。**

位置：`tools/r0/e2e/scripts/scan-artifacts.js` L261：

当前正则为：

```js
/[A-Za-z0-9+/]{16,}={0,2}/g
```

它不包含 Base64URL 使用的 `-` 与 `_`。当编码串包含这两个字符时，扫描器可能只截取片段，解码结果无法还原凭据标记。

修复要求：

- 同时覆盖标准 Base64 与 Base64URL 字符集、padding / 无 padding 两种形式。
- 增加 Base64URL 编码的 Authorization/Bearer 文本样本。
- 增加 Base64URL 编码的敏感键名与值组合样本。
- 断言扫描结果为 BLOCKED，且输出不包含完整编码值。

## 4. 测试与证据质量问题

### RV-11 · P2：K1-03 的测试标题与实际注入位置不一致

位置：`tools/r0/e2e/test-scan-artifacts.js` case-20。

当前样本：

```js
['Bearer ', String.fromCharCode(0), tokenValue].join('')
```

NUL 位于 `Bearer ` 与 token 值之间，不在 token 值内部。该测试证明了分隔符处 NUL 可由 squeeze 复扫恢复，但不能证明 token 值内部的 NUL 注入已覆盖。

要求：另加 token 值内部插入一个或多个 NUL 的测试，并覆盖分散位置；保留原测试并修正其描述。

### RV-12 · P2：K1-04 的 .bin 样本没有走真实上传白名单链路

位置：

- `tools/r0/e2e/test-scan-artifacts.js` case-21：写入 `random.bin`。
- `tools/r0/e2e/scripts/prepare-upload.js` L37：上传白名单不包含 `.bin`。

问题：

case-21 直接调用 scanner，因此证明 scanner 能阻断该 `.bin` 文件；但真实 workflow 会先执行 prepare-upload，`.bin` 会被排除，不会进入 scanner。该测试不能证明“真实上传链路遇到无法解码文本会阻断”。

要求：

- 将至少一个不可解码样本改为白名单内的 `random.txt`。
- 必须执行 prepare-upload，再扫描输出目录。
- 断言该文件确实被复制进上传目录、scanner 返回 BLOCKED、upload 不执行。

## 5. CI / 交付状态快照

审查时 GitHub 官方状态：

| 项目 | Run / 状态 | 说明 |
|---|---|---|
| 普通 CI | `42079445701` success | build / vet / test / 全仓 race / Docker healthz 全部 success |
| R0 E2E | `42079445817` in_progress | HEAD 为 `c95ff094300e6e629886edfa6a3a61161dfa4351` |
| R0 E2E step 8 | success | K1–K3 30 项负向测试 step 已成功 |
| R0 E2E step 10 | success | A 11 项门禁 step 已成功 |
| R0 E2E step 11 | success | D 39 项门禁 step 已成功 |
| R0 E2E step 12 | in_progress | C 228 条报告型测试尚未结束 |
| 上传前扫描 / fail-closed / Artifact | pending | 尚无本轮 Artifact，不能声称新扫描器已通过实际 Artifact 复验 |

本轮整改报告 `docs/r0-evidence-7th-fix/p1/k1-k3-remediation.md` 的 §9 / §10 仍保留 TBD，未回填最终 CI SHA、Run、Artifact 和下载后独立扫描结果。应在 CI 完成后如实回填。

## 6. 交给 WorkBuddy 的最小整改顺序

1. **先修 RV-07（PEM 明文私钥漏检）**，并加入完整 prepare → scan 负向集成测试。
2. **再修 RV-08（fail-closed 验证必须成为上传门禁）**，同时把命中退出码严格限定为 2。
3. **修 RV-09（非法 UTF-8 容错阈值绕过）**，新增修复前/后反例。
4. 修 RV-10 Base64URL。
5. 补 RV-11 / RV-12 测试覆盖口径。
6. 等待 run `42079445817` 完成；对最终 HEAD 的 CI 和最新 Artifact 做复核，下载 Artifact 后使用修复后的扫描器独立复扫。
7. 更新整改报告 §9 / §10、PR 描述、R0 回执、README 与 CHANGELOG。旧证据保留，不覆盖。

## 7. 权限与总体状态

- 本报告仅独立审计与整改要求，不授权 WorkBuddy 合并 PR。
- 不修改 PRD / WBS / 业务规则 / DB / API。
- `OBS-ACCT-VIS-390` 账户名称 390px 不可见仍为 FAIL，未授权修改 CSS。
- 13 条历史控件 ID 映射问题仍独立登记。
- RV-05 旧 SHA 公开历史与仓库可见性仍为 Owner ACTION REQUIRED。
- PR #3 保持 Open；**R0 继续 BLOCK；不启动 R1–R7 / PWA。**

**最终结论：WorkBuddy 本轮 K1–K3 修复值得肯定，但扫描器安全闭环尚未成立。应先完成本报告 RV-07–RV-12，再进行第三方复核。**


## 8. R0 E2E 完成后的追加复核（2026-11-06）

后续重新读取 GitHub 官方 run、job steps、job logs，并下载 Artifact 13441565324（名称 r0-e2e-evidence-4cb41db8d3a63f2e589ae72a920babadecaeebb9，3,647,442 bytes）。该 run 的最终状态为 success，HEAD 为 PR 分支 c95ff094300e6e629886edfa6a3a61161dfa4351；报告 JSON 中的 gitSha=4cb41db... 是 GitHub PR workflow 的 synthetic merge SHA（refs/pull/3/merge），不是 PR head SHA。两者必须区分。

### 8.1 最终运行结果

| 包 / step | 最终结果 | 独立判断 |
|---|---|---|
| 普通 CI 42079445701 | success | 全仓 Go race、Docker/healthz 通过 |
| B 分类解析合成测试 | success | 本轮 step 成功 |
| K1–K3 扫描器 30 项测试 | success | CI step 真实执行成功 |
| A | 11/11 PASS | 仍 PARTIAL：A05 的代码只按 elapsed >= 800 判 PASS；读取了 midText 但未断言 pending 文案/disabled。Artifact 中 A05 虽记录中途文案「登录中…」，仍不能证明 PASS 条件有效 |
| D | 39/39 PASS | 本轮授权的三项业务修复及交易总表回归通过；账户名称 390px 可见性另项仍 FAIL |
| C | PASS 164 / FAIL 4 / BLOCKED 59 / N/A 1 | 仍 PARTIAL；四条 FAIL 为 I-LOGIN-04、I-LOGIN-11、I-LOGIN-12、I-HOME-01；59 BLOCKED 未清零，不能把 job success 解释成 C 通过 |
| Artifact 上传前扫描 | success，scanner 输出 CLEAN | 本轮扫描器对准备上传目录返回 CLEAN |
| fail-closed 集成验证 | success | 合成 x-api-key + UTF-16LE Bearer 样本返回 rc=2；干净样本 rc=0 |
| Artifact upload | success | 上传了 101 个条目；原始 trace/HAR/network/数据库文件未出现在 Artifact |

A05 的当前代码证据：tools/r0/e2e/specs/a-scenarios.js L181–190；第 188 行仍为 status: elapsed >= 800 ? 'PASS' : 'FAIL'。第 183 行读取 midText，但该值未进入 PASS 判定，也没有 disabled 状态断言。故 A05 仍是假阳性风险，A 不得改 PASS。

### 8.2 Artifact 下载后独立检查

Artifact 13441565324 已实际下载并解压：

- 文件条目：101。
- 三份 SHA256SUMS.txt。
- 原始 .zip/.trace/.network/.har/.db/.sqlite* 文件：0。
- 文件名含 trace/network 的仅有 network-abort.png、slow-network.png 两张截图，不是原始网络记录。
- 对所有文件做了独立常见模式扫描：未发现 JWT 三段式、Bearer、Authorization、API Key、PEM 私钥头等命中。该扫描是补充核验，不等于通用 DLP 的绝对保证。
- Artifact 中 CI scanner 日志记录：文件全量扫描、跳过内容扫描=0、命中=0、结论 CLEAN；upload outcome success。

### RV-13 · P2：白名单过滤后没有重建 SHA256SUMS，导致 Artifact 校验清单引用不存在的 trace 文件

**状态：Artifact 下载后字节级核验确认。**

位置：

- tools/r0/e2e/scripts/prepare-upload.js：L37 白名单；L117–134 分类；文件复制逻辑在 main() 中将白名单文件复制到目标目录。
- Artifact 内三份 SHA256SUMS.txt。

实测：

- 三份 checksum 清单共 110 条记录。
- 其中 12 条引用了白名单准备阶段已排除的原始 trace 文件（A01–A08、A09 的 admin/member/owner、A10）。
- 这 12 个文件在最终 Artifact 中不存在，因此 sha256sum -c 无法完成。
- 其余 98 条实际存在文件的 SHA256 均匹配。

根因：prepare-upload.js 将源目录已有的 SHA256SUMS.txt 当作普通 .txt 文件复制，没有针对最终白名单目录重新生成校验清单。

影响：安全过滤确实排除了 trace，但 Artifact 的完整性清单不再对应最终上传内容，后续无法用该清单完整复验 Artifact。

修复要求：

1. 白名单收敛完成后，在目标目录重新生成 SHA256SUMS，清单只包含最终实际上传的文件。
2. 生成时排除 SHA256SUMS 自身，避免递归。
3. 上传前执行一次 sha256sum -c（或等价跨平台校验），必须 0 missing / 0 mismatch。
4. 对 prepare-upload 加集成测试：源目录包含一个白名单文件和一个被排除的 trace.zip；最终清单只包含白名单文件，且校验全部成功。
5. 旧清单保留为历史证据，不可静默改写历史 artifact。

## 9. 最终综合状态

- K1–K3：主体规则与 30 项测试有明显改善，但因 RV-07–RV-12 尚未修复，仍 PARTIAL / 安全 BLOCK。
- RV-13：新增证据完整性问题，需一并修复。
- A：仍 PARTIAL（A05 PASS 条件未修）。
- B：仍 PARTIAL（t021 Owner 决策仍待确认）。
- C：仍 PARTIAL（164/4/59/1）。
- D：本轮 39/39 缺陷回归通过，但整体 D 仍 PARTIAL，且 OBS-ACCT-VIS-390 仍 FAIL。
- RV-05：Owner ACTION REQUIRED。
- PR #3：Open，未合并。
- **R0：BLOCK；不启动 R1–R7 / PWA。**
