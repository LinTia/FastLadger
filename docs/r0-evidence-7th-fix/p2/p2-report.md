# FastLedger R0 第七次整改 · P2 报告（测试编号错位 + 运行器 all 语义）

- 分支：`fix/r0-sixth-runtime-defects`（未切换、未合并 main）
- 基线提交：`0c62f5017c8b616f6d0c5079c55a9b4a13d1f68f`（P1 已提交并 push）
- 工作目录：`repo_tmp`；本轮**未 commit、未 push、未切分支、未合并 main**
- 执行时间（UTC）：2026-11-05 13:10–13:25
- 范围：仅任务 A（`c-controls.js` 的 `groupAccounts`）与任务 B（`run.js` 的 `all` 语义 +
  `tools/r0/e2e/README.md`），与 P1（artifact 安全）完全无关。

---

## 一、改动文件清单

### 修改的已跟踪文件（仅 2 个）

| 文件 | 改动摘要 |
|---|---|
| `tools/r0/e2e/specs/c-controls.js` | 只改 `groupAccounts` 一组：`I-ACCT-06` 改为按清单定义的「删除引用保护」真实 409 断言；`I-ACCT-10` 改为按清单定义的「币种 `#acctCur`」真实键入断言；可见性观察移出 `R` 并登记为清单外独立项；保留 `I-ACCT-01..05 / 07..09 / 11..15` 原样 |
| `tools/r0/e2e/run.js` | 头部注释重写（all = A + D + C、B 由 workflow 独立 step 执行）；L86 `all` 由 `['a','c']` 改为 `['a','d','c']`；新增与 C/D 对称的 `A 汇总` 日志行（纯可观测性，便于 all 日志看出三个套件各有汇总） |

`git diff --stat`（限定 P2 自身）：`tools/r0/e2e/run.js` 34 行变更、
`tools/r0/e2e/specs/c-controls.js` 171 行变更，合计 180 insertions / 25 deletions。

### 新建文件

| 文件 | 用途 |
|---|---|
| `tools/r0/e2e/README.md` | 「套件与分工」：A/C/D 定位、all 组成、B 的具体命令、退出码语义、CI 绿色不等于 228 全通过 |
| `docs/r0-evidence-7th-fix/p2/iacct-mapping-audit.md` | I-ACCT-01..15 全量编号错位审计（清单定义 / 脚本当前实际断言 / 是否错位） |
| `docs/r0-evidence-7th-fix/p2/p2-report.md` | 本报告 |
| `docs/r0-evidence-7th-fix/p2/c-after-p2.log` / `c-after-p2-report.json` | `--suite c` 完整 stdout/stderr + 真实退出码、`report.json` 副本 |
| `docs/r0-evidence-7th-fix/p2/all-suite.log` / `all-suite-report.json` | `--suite all` 完整 stdout/stderr + 真实退出码、`report.json` 副本 |
| `docs/r0-evidence-7th-fix/p2/visibility-observation-independent.json` | 清单外独立登记 `OBS-ACCT-VIS-390`（仍 FAIL） |
| `docs/r0-evidence-7th-fix/p2/visibility-observations.json` | 原始观测（保留落盘，ID 已改为清单外） |
| `docs/r0-evidence-7th-fix/p2/delete-disappearance-independent.json` | 清单外独立观察 `OBS-ACCT-DEL-390` |
| `docs/r0-evidence-7th-fix/p2/diff.txt` / `status.txt` / `exit-codes.txt` | git diff 全文（三段式，见下）/ 工作树状态 / 命令退出码 |
| `docs/r0-evidence-7th-fix/p2/credential-scan.log` | `node tools/r0/scan-credentials.js --strict` 结果 |

---

## 二、任务 A：根因与最小修改

### A-1 `I-ACCT-06` 根因：清单外观察项错占编号，真实「删除引用保护」从未被测

- 清单 `controls.json` L1578：`I-ACCT-06` = 「删除引用保护」，pre「账户被交易引用」，
  action「确认删除」，fail「toast 后端 ErrAccountInUse → 409」，status「已实现｜运行时未验证」。
- 修复前脚本 L383 把 `R['I-ACCT-06']` 写成「新增账户名称 390px 不可见 / 可见性未闭环」的
  **无条件 FAIL**（上一轮 Owner 收紧裁决），既测错了对象，也让真正的清单项完全没有覆盖。
- 后端语义（本轮只读核对，未改后端）：`service/account.go` `Delete()` 的引用检查覆盖**软删除**
  交易（GPT-R31），命中即 `ErrAccountInUse`；`api/common.go` L100–101 映射为
  `http.StatusConflict` = 409，文案「账户仍存在交易记录，无法删除」；
  `api/qa_api_test.go` L67 有 `{"ErrAccountInUse", ..., http.StatusConflict}` 断言。

**最小修改**（`groupAccounts` 内，弹窗流程之前，API 层不依赖 UI）：

```js
const cashId = data.ids.cashId;                       // 种子「现金」账户
const before = await api.accounts(base, token, bookId);
const delResp = await api.raw(base, 'DELETE',
  `/api/v1/books/${bookId}/accounts/${cashId}`, { token });   // raw 不抛错，可取真实状态码
const after = await api.accounts(base, token, bookId);          // API 回读
```

- **目标账户选择理由**：直接复用 `data.ids.cashId`。`bootstrap` 已用该账户创建「早餐」「工资」
  等活跃交易及一笔软删除交易「待删除」，而后端引用检查覆盖软删除交易，因此它 100% 被交易
  引用；无需在组内另建账户/交易，避免额外写操作与不确定性。删除路由无 `clientUuid`
  （与 `app.js` L718 一致）。
- **PASS 条件**：`delResp.status === 409` && `json.msg` 命中「账户仍存在交易记录」&&
  `api.accounts()` 回读该 id 仍在且名称与删除前一致 —— **三者同时成立**，否则如实 `bad()`。
  detail 写明实际 status、响应 msg、回读 id/名称。异常用 try/catch 转为真实 FAIL，不臆造。
- **实际观测**（见 §八）：HTTP `409`，`msg=「账户仍存在交易记录，无法删除」`，
  回读 `id=1 name=「现金」` 与删除前一致 → **PASS**。

### A-2 `I-ACCT-10` 根因：确认是假阳性（且有第二个成因）

清单 L1642：`I-ACCT-10` = 「币种 `#acctCur`」，input「默认 CNY；编辑时 disabled」，
action「键入」，success「显示文本」。

修复前脚本 L397：

```js
R['I-ACCT-10'] = !(await text(page)).includes(name) ? ok('删除后从列表消失') : bad('删除后仍存在');
```

**检查结论：确认是假阳性**，且有两重成因：

1. `text()` 读 `#view.innerText`；390px 视口下 `style.css` L331
   （`@media (max-width:640px)`）把 `td:first-child` 设为 `display:none`，账户名称**从不在**
   `innerText` 中 → `includes(name)` 恒为 false → 断言**恒真**，无论是否真的删除都 PASS。
   而且它测的根本不是清单定义的「币种 `#acctCur`」。
2. **本轮新发现**：账户列表按 `category` 排序（`app.js` `renderAccounts` 按 `state.accounts`
   顺序渲染），新建账户并不在最后一行（观测顺序为「现金 / E2E 账户… / 储蓄卡 / 信用卡」）。
   修复前 `button:hasText('删除').last()` 实际点到的是「信用卡」行，而非新建账户行 ——
   即使改用 `textContent`，旧交互也不会让目标账户消失。用精确定位后的
   `td:first-child.textContent` 真实断言复核，`disappeared=true`（见 §五）。

**最小修改**：在新增账户弹窗内对 `#acctCur` 做真实观测 —— 控件数、tag/type/maxlength/
disabled、默认 value、真实 `fill('CNY')` 后的 `inputValue` 回显；PASS = 存在 && 未禁用 &&
可键入（回显 === 'CNY'）。观测值写进 detail，不用 `innerText` 恒真式、不硬编码 PASS。
原「删除后从列表消失」不写入 `R`，改为清单外 `OBS-ACCT-DEL-390` 观察项（精确按首列名称
定位行 + `td:first-child.textContent` 真实断言），不占任何 228 编号。

### A-3 清单外独立登记：第四项视觉可见性仍 FAIL

- 保留 `visibleInnerText` / `firstTdDisplay` 两个原始观测与
  `C/I-ACCT/visibility-observations.json` 落盘（ID 由 `I-ACCT-06` 微调为 `OBS-ACCT-VIS-390`，
  截图名同步改为 `OBS-ACCT-VIS-390-visibility.png`）。
- 新增 `C/I-ACCT/visibility-observation-independent.json`，独立观察 ID
  `OBS-ACCT-VIS-390`，含 viewport、`visibleInnerText`、`firstTdDisplay`、
  `status: "FAIL"`、`closed: false`、`ownerDecision: "待 Owner 裁决"`，结论明确写
  「视觉可见性未闭环，仍 FAIL，待 Owner 裁决；本轮不改 style.css」。
- **该观察项不写入 `R`**（`R` 的 key 会进 228 计数），不占 `I-ACCT-06`，不改 `controls.json`，
  **不改变 228 条清单**。代码注释、README、审计报告与本报告均明确「清单外登记」。
- 第四项缺陷**仍为 FAIL / 未闭环**，本轮未授权改 `server/web/dist/style.css`，
  **未以任何形式改成 PASS**。

### A-4 其余编号：如实登记、不批量改动

`groupAccounts` 里 `I-ACCT-01..05 / 07..09 / 11..15` 与清单定义也存在系统性错位
（例如清单 03=编辑按钮、07=期初余额输入、09=账户类型、10=币种，而脚本分别测弹窗打开 /
余额列 / 删除按钮 / 删除后消失）。本轮**未改动**这些断言（超出第三方点名范围，且批量对齐会
显著改变 228 分布），但已逐条登记到 `p2/iacct-mapping-audit.md`，含「清单 ID / 清单定义 /
脚本当前实际断言 / 是否错位」四列，并注明「若要全面对齐 228 清单需另提方案供第三方审核」。
**未改 `controls.json`、未改测试编号、未把清单改成脚本现状来消灭错位。**

---

## 三、任务 B：根因与最小修改

- 根因：`run.js` L86 `args.suite === 'all' ? ['a','c'] : [args.suite]` —— `all` **漏掉 D**，
  且 L9 头部注释还写「`--suite all` 保持既有 A + C 组合；D 需显式 `--suite d` 运行」，
  注释与实现一致地错。CI 里 D 有独立 step 才没有暴露，但本地 `--suite all` 会静默漏跑 D。
- B（分类解析合成测试 + 22 笔审计）由 `.github/workflows/r0-e2e.yml` 的独立 step 执行，
  不在运行器 suite 体系内：
  - `node --test tools/r0/test-resolve-category.js`
  - `R0_SERVER_BIN=<workspace>/server/fastledger node tools/r0/e2e/scripts/reseed-audit.js`
- 最小修改：
  1. L86 → `['a', 'd', 'c']`。**顺序理由**：D 是门禁型回归，放在报告型 C 之前，
     避免 C 的写操作（编辑/删除首笔交易、改成员角色、退出登录、生成 Token 等）污染 D 的
     回归数据；C 作为报告型放最后。实测该顺序下 D 汇总 39 PASS / 0 FAIL，验证有效。
  2. 头部注释重写为 all = A + D + C，并明确写出 B 的两条命令及「不在本运行器 suite 体系内」。
     `run.js` 无 `--help` 输出，故按要求只写头部注释，**未新增无关功能**。
  3. 新增 `A 汇总：{...}` 日志行（与既有 `C 汇总`/`D 汇总` 对称），使 all 日志能直接看出
     A、C、D 三个套件都执行且各有汇总。这是纯输出，不改变任何断言与退出码逻辑。
  4. 新建 `tools/r0/e2e/README.md` 的「套件与分工」：A/C/D 定位、all 组成与顺序、B 的具体
     命令、退出码语义（C 报告型有既有 FAIL → all exit 1 属预期，不代表 D 失败）、
     「C 的 CI 绿色 `continue-on-error` 不等于 228 条全部通过」。
- **未为了让 all 变绿而放宽 C 断言或跳过 C**：C 的 4 条既有 FAIL 完整保留。

---

## 四、I-ACCT 编号错位全量审计结论

完整 15 行表见 `p2/iacct-mapping-audit.md`。摘要：

- 修复后 **2 条已对齐**：`I-ACCT-06`（删除引用保护）、`I-ACCT-10`（币种 `#acctCur`）。
- **13 条仍错位**：`01↔02`（新增按钮 ↔ 账户行）、`03`（编辑按钮 → 弹窗打开）、
  `04`（删除按钮 → `#acctName`）、`05`（删除取消 → `#acctCat`）、`07`（期初余额输入「未实现」
  → 余额列 ¥）、`08`（`#acctName` → 编辑按钮）、`09`（`#acctCat` → 删除按钮）、
  `11–15`（清单有明确控件/已实现或登记缺失，却被一组统一 `blocked()` 兜底；其中 `I-ACCT-14`
  清单 `**缺失**｜R0-E-28` 按判定原则应判 FAIL，因 `R` 已给值导致 `classifyMissing()` 未介入）。
- 这些是**历史覆盖缺口**，本轮未掩盖、未消灭；全面对齐需另提方案供第三方审核。

---

## 五、清单外独立登记项（均不进 `R`、不占 228 编号）

| ID | 文件 | 状态 | 关键观测 |
|---|---|---|---|
| `OBS-ACCT-VIS-390` | `p2/visibility-observation-independent.json`（运行目录 `C/I-ACCT/` 同名） | **FAIL / 未闭环**，待 Owner 裁决 | viewport `390x844`；`visibleInnerText=false`；`firstTdDisplay="none"`；`postStatus=200`、`inDom=true`、`inApi=true`（数据链路正常，但 390px 下用户看不到名称）。**本轮不改 `style.css`** |
| `OBS-ACCT-DEL-390` | `p2/delete-disappearance-independent.json` | 清单外观察（非 228 判据） | `td:first-child.textContent` 真实断言；`targetRowFound=true`；删除前首列含新账户名，删除后不含 → `disappeared=true`。记录了旧 `.last()` 点错行为假阳性第二成因 |

**R 中没有任何 `OBS-*` key**；`runControls` 逐条按 `controls.json` 的 228 个 id 取 `R[c.id]`，
总数仍为 228（见 §七）。

---

## 六、实际运行记录

### 6.1 构建（前端 app.js 内嵌，必须重新构建）

```bash
source /c/Users/47006/WorkBuddy/fastledger/.tools/genv.sh
go build -o fastledger.exe ./cmd/server        # 在 server/ 下执行，退出码 0
# 产物 server/fastledger.exe，32,184,726 bytes
```

### 6.2 `--suite c`

```bash
R0_SERVER_BIN=<repo>/server/fastledger.exe \
R0_BROWSER_CHANNEL=msedge \
R0_ARTIFACT_DIR=<repo>/artifacts/r0-p2-c \
node run.js --suite c
```

- `c-after-p2.log` 完整 stdout/stderr + 末尾 `=== 真实退出码: 1 ===`。
- `C 汇总：{"PASS":164,"FAIL":4,"BLOCKED":59,"N/A":1}`；`total=228`。
- 4 条既有 FAIL：`I-LOGIN-04`、`I-LOGIN-11`、`I-LOGIN-12`、`I-HOME-01`（均为产品缺失，
  与 P2 无关，未放宽）。
- 副本：`p2/c-after-p2-report.json`。

### 6.3 `--suite all`（本轮核心可验证记录）

```bash
R0_SERVER_BIN=<repo>/server/fastledger.exe \
R0_BROWSER_CHANNEL=msedge \
R0_ARTIFACT_DIR=<repo>/artifacts/r0-p2-all \
node run.js --suite all
```

`p2/all-suite.log` 中三条套件汇总（可 grep `^A 汇总|^D 汇总|^C 汇总`）：

```
A 汇总：{"PASS":11}
D 汇总：{"PASS":39}
C 汇总：{"PASS":164,"FAIL":4,"BLOCKED":59,"N/A":1}
SUMMARY {"A:PASS":11,"D:PASS":39,"C:PASS":164,"C:FAIL":4,"C:BLOCKED":59,"C:N/A":1,"total":278}
```

- **A、C、D 三个套件都执行且各有汇总**，D 不再被 `all` 静默漏跑。
- **真实退出码 1**（日志末尾 `=== 真实退出码: 1 ===`）。原因：C 报告型有既有 4 条 FAIL，
  按 `run.js` 退出码规则任一 suite 出现 FAIL 即 exit 1；**属预期，不代表 D 失败**
  （本次 D 为 39 PASS / 0 FAIL）。
- 副本：`p2/all-suite-report.json`；运行目录
  `artifacts/r0-p2-all/2026-11-05_13-20-26-0c62f50-all`（104 个证据文件）。

### 6.4 凭据严格扫描

`node tools/r0/scan-credentials.js --strict` → **退出码 0**，
`真实凭证候选命中：0`；合成/占位 167、人工复核 15、测试夹具 27、可移植性 100
（均为历史既有的合成样本/个人路径提示，不参与 strict 判定；精确数值以
`p2/credential-scan.log` 最终扫描为准）。结果见 `p2/credential-scan.log`。
报告中未写入任何 JWT 原文。

### 6.5 语法自检

`node --check tools/r0/e2e/specs/c-controls.js` → 0；
`node --check tools/r0/e2e/run.js` → 0。全部退出码见 `p2/exit-codes.txt`。

---

## 七、修复前后 C 分布对比及原因（逐 ID）

| 分布 | PASS | FAIL | BLOCKED | N/A | 合计 |
|---|---:|---:|---:|---:|---:|
| 修复前（基线 `artifacts/r0-fix-after-c/…-c/report.json`） | 163 | 5 | 59 | 1 | 228 |
| 修复后（`p2/c-after-p2-report.json`） | **164** | **4** | 59 | 1 | 228 |

程序化逐 ID 比对结果：**唯一发生状态变化的 ID 是 `I-ACCT-06`：`FAIL → PASS`**；
`I-ACCT-10` 状态仍为 `PASS`（`PASS → PASS`），但 detail 与断言依据已完全更换；
其余 226 个 ID 状态逐条不变；`R` 的 key 数仍为 228，无新增/删除编号。

**变化原因（必须避免误读）：**

1. `I-ACCT-06` 的 FAIL **不是被"放宽断言"改没的**：修复前那条 FAIL 的内容是
   「新增账户名称 390px 不可见 / 可见性未闭环」，属于**清单外观察项错占了 I-ACCT-06 编号**，
   而清单 I-ACCT-06「删除引用保护」从来没有被测过。本轮把该编号**按清单定义补测**
   （真实 HTTP 409 + 响应文案 + API 回读仍在），三条真实观测同时成立才判 PASS ——
   这是**补齐历史未验证覆盖**，且比原来的"无条件 FAIL"更严格（有明确可证伪条件），
   不是放宽断言、不是跳过、不是硬编码 PASS。
2. 第四项「新增账户 390px 视觉可见性」**作为清单外独立项 `OBS-ACCT-VIS-390`
   仍然 FAIL / 未闭环**，在 `visibility-observation-independent.json` 中以
   `status: FAIL`、`closed: false` 明确登记，待 Owner 裁决；本轮未改 `style.css`。
   它不再占用 `R['I-ACCT-06']`，因此不计入 228 计数 —— 这是**编号归位**，不是"把 FAIL 改没了"。
3. `I-ACCT-10` 由「恒真假阳性 PASS」变为「真实 DOM/键入观测 PASS」，状态未变，
   但结论可信度从 0 变为可复核；原假阳性的第二个成因（`.last()` 点错行）也已在
   `OBS-ACCT-DEL-390` 中如实登记。
4. 其余 4 条 FAIL（`I-LOGIN-04`、`I-LOGIN-11`、`I-LOGIN-12`、`I-HOME-01`）与
   59 BLOCKED / 1 N/A **全部原样保留**，未做任何宽松化。

---

## 八、`I-ACCT-06` / `I-ACCT-10` 实际结果 detail（原文摘录）

**`I-ACCT-06` [PASS] — `c-after-p2-report.json` / `all-suite-report.json`**

> 删除引用保护（清单 I-ACCT-06 定义：账户被交易引用 → 确认删除 → 后端 409）：对被引用的
> 「现金」账户 id=1 发起 DELETE /api/v1/books/1/accounts/1 → 真实 HTTP status=409，
> 响应 json.msg=「账户仍存在交易记录，无法删除」；API 回读 GET /api/v1/books/1/accounts：
> id=1 仍在，name=「现金」与删除前 name=「现金」一致，未被删除；
> PASS 条件 = (status===409 && msg 含「账户仍存在交易记录」&& 回读仍在) 同时成立

**`I-ACCT-10` [PASS]**

> 币种 #acctCur（清单 I-ACCT-10 定义：币种 #acctCur / 键入 / 显示文本）：新增账户弹窗内
> 控件数=1，tag=INPUT，type=text，maxlength=8，disabled=false，默认 value=「CNY」；
> 真实键入 'CNY' 后 inputValue 回显=「CNY」，可键入=true；观测依据为浏览器 DOM 属性与
> 真实输入回显，不使用 innerText 恒真式、不硬编码 PASS

---

## 九、新发现 / 未解决问题

1. **【新发现】假阳性的第二成因**：账户列表按类别排序，旧 `I-ACCT-10` 的
   `button:hasText('删除').last()` 实际点击的是「信用卡」行而非新建账户行；即便换成
   `textContent`，旧交互也不会让目标账户消失。已用「按首列名称精确定位 + `textContent`」
   复核，得到 `disappeared=true`，登记在 `OBS-ACCT-DEL-390`（清单外）。
   → 该项仍属清单外观察，未改任何 228 编号断言；未作为新缺陷上报 Owner（超出本轮范围）。
2. **【未解决】第四项「新增账户 390px 视觉可见性」仍 FAIL / 未闭环**：根因
   `server/web/dist/style.css` L331 在 `max-width:640px` 下隐藏 `td:first-child`，
   本轮未授权修改；待 Owner 裁决。
3. **【未解决】I-ACCT 其余 13 条编号错位**：本轮按授权只登记、不批量对齐；
   全面对齐需另提方案供第三方审核（见 `p2/iacct-mapping-audit.md`）。
4. **【未解决】第五项候选缺陷**：仍待 Owner 确认，本轮未自行选择、未修改。
5. **【共享工作树提示】**：`p2/status.txt` 中除本轮 P2 的 2 个已跟踪修改 + 新文件外，
   还存在其它成员并发进行的 **P1 round2** 改动（`.github/workflows/r0-e2e.yml`、
   `tools/r0/e2e/scripts/scan-artifacts.js`、`tools/r0/e2e/test-scan-artifacts.js`、
   `docs/r0-evidence-7th-fix/p1/*`、`p1/round2/` 等）。**这些不是本轮 P2 所改**，
   P2 全程未修改任何 P1 文件；`p2/diff.txt` 已分三段明确标注（P2 自身限定 diff /
   新增文件全文 / 工作树全量 diff），避免混淆。
6. **未启动** R1–R7 / PWA；未改业务规则/产品范围。

---

## 十、边界自查

| 约束 | 自查结果 |
|---|---|
| 不改 P1 的 4 个文件与 `docs/r0-evidence-7th-fix/p1/` | ✅ 未改（工作树中 P1 变更来自其它成员并发工作，见 §九.5） |
| 不改 `controls.json`（228 条） | ✅ `git diff --` 未包含该文件 |
| 不改 `server/web/dist/style.css`（L331 根因） | ✅ 未改；第四项仍 FAIL |
| 不改 `server/web/dist/app.js` / 任何 Go 后端 / PRD/WBS / DB/迁移 / API 契约 | ✅ 只读核对，未改 |
| 不改历史证据目录 `docs/r0-evidence-6th/` 及更早 | ✅ 未改 |
| 不改 `README.md` / `CHANGELOG.md`（team-lead 处理） | ✅ 未改（只新建 `tools/r0/e2e/README.md`，仓库根 README 未动） |
| 不改测试编号、不占用 228 任何编号给视觉可见性 | ✅ `R` 中无 `OBS-*`；总数仍 228 |
| 不把第四项改成 PASS | ✅ 清单外 `OBS-ACCT-VIS-390` 为 `FAIL` / `closed:false` |
| 不为让 all 变绿放宽 C 断言或跳过 C | ✅ 4 条既有 FAIL 完整保留；all exit 1 |
| 不 commit / push / 切分支 / 合并 main | ✅ 全部未做 |
| 不把 JWT 原文写入报告 | ✅ 复制前检查 `eyJ…` / `flt_…` / `Bearer …` 均无命中 |
| 不用固定 sleep 制造通过 | ✅ `I-ACCT-06`/`I-ACCT-10` 均为真实 HTTP 状态码 / DOM 属性 / 输入回显；组内既有 sleep 原样保留，未新增用于"制造通过"的等待 |
| 不硬编码 PASS | ✅ 两条均有可证伪条件与真实观测值，失败走 `bad()` |
| 不启动 R1–R7 / PWA，不改业务规则/产品范围 | ✅ |

---

## 十一、证据文件索引（`docs/r0-evidence-7th-fix/p2/`）

| 文件 | 内容 |
|---|---|
| `p2-report.md` | 本报告 |
| `iacct-mapping-audit.md` | I-ACCT-01..15 全量错位审计 |
| `c-after-p2.log` / `c-after-p2-report.json` | `--suite c` 完整日志 + 真实退出码 / 报告副本 |
| `all-suite.log` / `all-suite-report.json` | `--suite all` 完整日志 + 真实退出码 / 报告副本 |
| `visibility-observation-independent.json` | `OBS-ACCT-VIS-390`（仍 FAIL） |
| `visibility-observations.json` | 原始可见性观测（清单外 ID） |
| `delete-disappearance-independent.json` | `OBS-ACCT-DEL-390`（textContent 真实断言） |
| `diff.txt` | 三段式 git diff（P2 自身 / 新增文件 / 工作树全量） |
| `status.txt` | `git status --porcelain` 快照 |
| `exit-codes.txt` | 每条命令及真实退出码 |
| `credential-scan.log` | `--strict` 扫描结果（退出码 0，真实凭证候选 0） |
