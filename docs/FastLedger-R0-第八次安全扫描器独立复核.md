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
