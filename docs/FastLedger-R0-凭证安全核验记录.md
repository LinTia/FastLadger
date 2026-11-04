# FastLedger R0 凭证安全核验记录（RV-05 / RV-06，[1.68]）

> 执行日期：2026-11-04（GMT+8）  
> 执行基准：开工前 `git fetch` 得到的 `origin/main` HEAD `273cf0869917565b60f7da1dd2b562764de8d405`  
> 执行范围：当前工作树的实际检查、脚本运行与命令输出；**不记录、不引用任何令牌 / 口令 / Cookie / 密钥值**。  
> 关联：`docs/r0-screenshots/security-scan.md`（脱敏扫描报告）、`tools/r0/README.md`（复现工具说明）、`docs/FastLedger-R0-第三次整改交付回执.md`（逐项回执）。  
> 状态：**R0 未通过 / 阻塞；不授权 R1；PWA 未实施。**

---

## 1. 执行环境

| 项 | 实测值 |
|---|---|
| Node | `v22.22.2`（`node --version`，内置 `WebSocket` / `fetch` / `node:test`） |
| 浏览器（归档证据采集时） | `Edg/156.0.4314.58`（CDP `/json/version` 实测） |
| 本地隔离实例 | `127.0.0.1:8080`，`/healthz` `status:"up"`、`schemaVersion=8`，一次性随机 `EBK_JWT_SECRET`，**验证后立即终止** |
| 远端 | GitHub `LinTia/FastLadger`（Public），经本机代理访问；`origin/main` refs 见 §4 |
| 平台 | Windows 11 + Git Bash（脚本仅在此外实测，**不声称跨平台**） |

> 说明：本轮为完成「旧令牌失效受控验证」启动过一次**一次性**服务进程（脚本内 `Popen` → 验证 → `terminate`），未留下常驻服务；用户此前取消的常驻后台任务未被重启。

---

## 2. RV-05 · 全仓凭证扫描（真实命令 / 输出 / 退出码）

扫描器：`tools/r0/scan-credentials.js`。只输出文件路径、命中类型、数量与脱敏上下文；**不输出敏感值**。判定口径：

- `credential-candidate`（参与 `--strict`）：JWT 三段式字面量、`Bearer <token>`、真实头值、JSON 密码字段值，且不是合成 / 占位 / 测试夹具。
- `synthetic`：含 `FAKE` / `DUMMY` / `EXAMPLE` / `REDACTED` / `<占位符>` / 命令替换 / 环境变量引用，或说明性的 `Authorization: Bearer <token>` 文字。
- `review`：代码 / 文档中的赋值表达式（无法自动区分示例与真实值），全部列出供人工复核，不作为 strict 自动失败依据。
- `test-fixture`：`*_test.go`、`test-redaction.js`、`ci.yml`、`*.example`。
- `portability`：`*.env` / token-secret 文件名、个人绝对路径（非凭证，但按任务单要求列出）。

| 步骤 | 命令 | 真实输出（摘要） | 退出码 |
|---|---|---|---|
| 清理前基线 | `node tools/r0/scan-credentials.js --strict` | 命中 **2 处可复制固定口令**：`docs/r0-screenshots/runtime/README.md`、`docs/FastLedger-新手开发完整指南.md`（均为 `JSON_SECRET_FIELD`）；`真实凭证候选命中：2` | **2** |
| 清理动作 | 两处改为环境变量 / 占位符；`docs/r0-screenshots/runtime/README.md` 整体重写为无个人绝对路径、无固定口令、`R0_TOKEN` 环境变量配方；[1.65] 历史命令加「不得复制复用」注记 | 目标串复核：已知固定测试口令字面量 **0**；`git diff --check` 干净 | 0 |
| 清理后（不带 `--out`） | `node tools/r0/scan-credentials.js --strict` | `扫描文本文件：449`；`真实凭证候选命中：0`；合成/占位 48、人工复核 14、测试夹具 27、可移植性 18 | **0** |
| 生成正式报告 | `node tools/r0/scan-credentials.js --strict --out docs/r0-screenshots/security-scan.md` | 与上一行统计完全一致（报告文件自身不参与扫描）；连续两次生成统计一致，幂等 | **0** |
| 报告敏感性复核 | Python 检查报告正文 | 已知固定测试口令 `0`、JWT 字面量 `0`、个人绝对路径 `0` | 0 |

> 清理前基线 JSON 留档于工作区 `.tmp_shot/scan_before.json`（**不入库**）；正式报告 `docs/r0-screenshots/security-scan.md` 入库，其 SHA256 随仓库内容变化，以交付提交中的文件为准。
>
> **诚实边界**：`review`（14）与 `test-fixture`（27）命中主要是历史部署示例、Go 测试常量与合成假凭证；`portability`（17）含历史交接文档 / 外部审阅产物中的个人绝对路径。本轮**未擅自改写**这些历史 / 外部文档，清单见 §6，是否统一清理列为 Owner 待决策项。

---

## 3. RV-05 · 脱敏逻辑验证（实际跑，不只看脚本存在）

**根因**：上一轮采集脚本把 CDP 事件 `params` 原样写入日志，`Network.requestWillBeSent` 的请求头携带完整 `Authorization: Bearer <JWT>`。旧版 `scrub()` 只按少数键名与 JWT 字面量替换，未覆盖响应头、`headersText`、URL 查询参数与 JSON 请求体，且无自动化测试。

**本轮修复**：`tools/r0/redaction.js` 统一处理对象键名（精确集合，避免误伤 `tokenLength` 等描述性字段）、多行 `headersText`、`Bearer` 任意令牌、JWT 三段式、URL 查询参数、JSON / 表单敏感字段；`rv01-isolation.js` 在所有日志写盘前调用 `scrub()`，并增加「序列化结果不得出现原始 `R0_TOKEN`」自检（命中则退出码 4，中止写入）。

| 步骤 | 命令 | 真实输出 | 退出码 |
|---|---|---|---|
| 语法检查 | `node --check tools/r0/{redaction,test-redaction,scan-credentials,rv01-isolation}.js` | 4 个文件均 `EXIT=0` | 0 |
| 合成假凭证自动化测试 | `node --test tools/r0/test-redaction.js` | `1..6` / `# pass 6` / `# fail 0` / 6 个子测试全 `ok` | **0** |
| 覆盖断言 | 同上 | ① 合成 JWT / Cookie / 口令 / API Key / 查询参数 / 非 JWT Bearer 全部不可见于序列化输出；② 事件结构（URL 路径、方法、状态码、`requestId`、时间戳）保留；③ `headersText` 多行、URL query、JSON 请求体、表单串逐项脱敏；④ 描述性字段不误伤；⑤ 幂等；⑥ 每类事件单独序列化均无原始假凭证 | 0 |

> 测试全部使用**合成假凭证**（字符串含 `FAKE`），未使用真实令牌做回归样例；`Network.requestWillBeSent` / `Fetch.requestPaused` / `Network.responseReceived` / `Runtime.consoleAPICalled` / `Log.entryAdded` 五类事件形态均覆盖。

---

## 4. RV-05 · 旧令牌受控验证与远端历史对象可读性

| 检查 | 真实命令 / 请求 | 真实结果 |
|---|---|---|
| 旧提交对象（API） | 匿名 GET `api.github.com/repos/LinTia/FastLadger/commits/259bd11` | **HTTP 200**，`bytes=26480`，响应体 JWT 字面量 0 |
| 旧版日志（raw） | 匿名 GET `raw.githubusercontent.com/LinTia/FastLadger/259bd11/docs/r0-screenshots/runtime/network-log.json` | **HTTP 200**，`bytes=182281`，**仍含 29 处凭证字面量**（值不记录） |
| 本地隔离实例 | 一次性 `Popen` 启动 `server/fastledger.exe`，`/healthz` | **HTTP 200** `status:"up"`、`schemaVersion=8` |
| 旧令牌受控失效验证 | 从上述远端对象取回令牌（**仅内存、不打印、不落盘**）→ `GET /api/v1/users/me` | **HTTP 401**，脱敏错误语义「登录已失效，请重新登录」 |
| 一次性进程清理 | `terminate()` + `wait()` | 已终止，无常驻服务残留 |
| 远端 refs | `git ls-remote --heads --tags origin` | `44b842ac… refs/heads/fix/windows-first-build`；`273cf086… refs/heads/main`；**无活动分支 / 标签指向 `259bd11`** |

**结论（四项拆分，不合并）**：

1. 当前 `main` 文件已脱敏 ✅；2. 泄露凭证已失效 ✅（会话撤销 + 一次性测试密钥轮换）；3. 旧 SHA 仍可公开读取 ⚠️（悬空对象未被远端 GC）；4. **远端历史对象清理待 Owner 决策** ⛔。

> **不得声称历史明文已清除**：force-push 只改变 `main` 指向，本地 `reflog expire` / `gc --prune=now` 只清理本地对象。本轮**未**再次 force-push、未重写公开历史、未删除 refs、未切换仓库可见性、未联系 GitHub Support。该令牌由本地隔离测试实例的一次性随机密钥签发，与生产实例密钥无关，生产是否受影响仅按可验证证据表述，不外推。

---

## 5. RV-06 · 干净 clone 复现（真实执行记录）

**方法（如实说明）**：因最终提交尚未生成，先把当前工作树导出为临时 git 仓库（排除 `.git`、`.localdata`、`.dart_tool`、`build`、`.gotmp`、二进制与本地日志），`git init` + `commit` 后以 **`git -c core.autocrlf=false clone`** 到独立目录（保持 LF，使文件 SHA256 与仓库内容一致；Windows 默认 `autocrlf=true` 会检出为 CRLF 而导致哈希不同，第三方核对时请注意）。clone 中 **468 个文件、无 `.tools` / `.localdata` / `.tmp_shot` / `.dart_tool` 私有目录**，含 `tools/r0/` 全部 5 个文件。这是对最终提交的等价干净工作副本；最终提交落地后第三方可直接 `git clone` 远端复跑。

| # | 命令（在 clone 根目录） | 真实输出摘要 | 退出码 |
|---|---|---|---|
| 1 | `node --version` | `v22.22.2` | 0 |
| 2 | `node --check tools/r0/{4 个脚本}` | 4 个均 `EXIT=0` | 0 |
| 3 | `node --test tools/r0/test-redaction.js` | `1..6` / `# pass 6` / `# fail 0` | **0** |
| 4 | `node tools/r0/scan-credentials.js --strict` | 扫描文本文件 **377**、跳过二进制/超大 60；`真实凭证候选命中：0`；合成/占位 48、人工复核 14、测试夹具 27、可移植性 15 | **0** |
| 5 | `env -u R0_TOKEN -u R0_EDGE_PATH node tools/r0/rv01-isolation.js` | `RV01_CONFIG_ERROR 缺少必需配置：R0_TOKEN…`；未打印任何值 | **2**（预期安全失败） |
| 6 | `R0_TOKEN=FAKE_TOKEN_FOR_CONFIG_CHECK env -u R0_EDGE_PATH node …` | `RV01_CONFIG_ERROR 缺少必需配置：R0_EDGE_PATH…` | **2**（预期安全失败） |
| 7 | `R0_TOKEN=FAKE_TOKEN_FOR_CONFIG_CHECK R0_EDGE_PATH=<存在的可执行文件> R0_WRITE_EVIDENCE=1 node …` | `RV01_GUARD 拒绝写入正式证据目录：… 必须同时设置 R0_WRITE_EVIDENCE_CONFIRM=YES` | **3**（预期安全失败） |
| 8 | `sha256sum tools/r0/*` | 5 个文件哈希见下表 | 0 |

**输出文件清单与 SHA256（clone 中实测，LF 检出；与主仓库工作区文件哈希一致）**：

| 文件 | SHA256 |
|---|---|
| `tools/r0/README.md` | `deeb4dd4531b2fd03ac3a3fd6bb87f47d2949168deff975c0806ae8b8f51dd3b` |
| `tools/r0/redaction.js` | `81f2da8e166a8f9477ee07c0db50136a8a538b4813a4be5fb49357fd48352ef8` |
| `tools/r0/rv01-isolation.js` | `4703efcf268611db77352c0cc80c411f0c33363af764773623348461ac8e9907` |
| `tools/r0/scan-credentials.js` | `0ad2eadaa69ec26e4b3c095efeda007b1a7d03971f4c078d69cc129c3b5fcce9` |
| `tools/r0/test-redaction.js` | `e87469050e4de4e7c3332967202f4555b39ea19f375c46399d27bd43ae5bf96d` |

**边界**：#5–#7 属于任务单 §3.3 允许的「因明确缺失的外部依赖安全失败」。**端到端浏览器采集本轮未重跑**（需本地服务、测试账号凭证与 Edge），归档的 5 PNG + 3 JSON **未被覆盖**；`R0_WRITE_EVIDENCE` 危险开关默认关闭且有双重确认，实测 #7 确实拒绝写入正式证据目录。WorkBuddy 与 QA 的重复运行**不等于** Owner/CI 独立复跑，后者仍为待办。

---

## 6. RV-02 · 全文 SHA 语义与个人绝对路径命中清单

对 `CHANGELOG.md`、`README.md`、主报告、`runtime/README.md`、独立复核补充 RV-01至04、第三次整改任务单做了全文检索，逐处判断语义（**不机械替换所有 `557e4d2`**）：

| 语义 | 出现位置 | 处置 |
|---|---|---|
| 本轮整改执行基准 `273cf08` | `CHANGELOG.md` [1.68] 开头、主报告 §0.2 / §0 / §10.4 | 新增，明确为开工时 `main` HEAD；提交后新 SHA 记在 CHANGELOG / 回执，不在文内循环写自身 SHA |
| 业务代码证据基准 `bff6a2b` | 主报告 §0.2 / §3.1 取证基线 / §5.3 引言 / §5.3.0 / §5.3.4 / §7.5 / §9；`CHANGELOG.md` [1.63] / [1.65] / [1.68] | 保留；把残留的「当前 HEAD」措辞改为「业务代码证据基准 `bff6a2b`」（3 处） |
| 运行时采集时历史文档 HEAD `557e4d2` | 主报告 §0.2（新列）/ §5.3 引言 / §5.3.0（声明源码取证不按该 HEAD）/ §10.3（[1.65] 历史记录）/ `runtime/README.md`（证据时点）；`CHANGELOG.md` [1.65] 历史条目 | 全部改为历史时点语义；历史条目按原貌保留并注明语义 |
| 截图 / 运行时对应后端二进制代码 SHA | 主报告 §0.2 / §7.5、`runtime/README.md` §证据边界 | 明确「**未独立验证**」，不得推定等于 `bff6a2b` |
| 历史快照 `1cd56ab` | 主报告 §0.2、`CHANGELOG.md` [1.61] | 标注「历史快照，不再作为本轮有效代码基准」 |
| `ddbd1ae` | `CHANGELOG.md` [1.67]（任务单基准）、第三次整改任务单 §0 / §2.1 | 外部审阅产物与历史条目，按原文保留（任务单自身已要求「以执行时 fetch 的实际 HEAD 为准」） |
| `575eae3` | `docs/FastLedger-R0-独立复核补充-RV-01至04.md` §0 / §R0-RV-02 | 外部审阅产物自身的复核时点基准，按原文保留，不改写历史结论 |
| 「当前 HEAD / 当前文档 HEAD」 | 主报告修订后 **0 处**；任务单 §3.2 / §2.3、外部复核补充 §R0-RV-02 中为**问题描述原文** | 未改外部原文；本轮交付文档中已统一为五类基准 |

**个人绝对路径**：脚本 `tools/r0/` 与本轮重写 / 修订的审计文档中 **0 处**；主报告 §5.3.0 原有的 1 处已改为 `<repo-root>/`。历史交接文档、外部审阅产物与任务单中的存量路径（扫描报告 `PERSONAL_ABS_PATH` 行列出）**未擅自改写**，是否统一清理列为 Owner 待决策项；完整逐文件命中见 `docs/r0-screenshots/security-scan.md`。

---

## 7. 未完成项 / 待 Owner 决策

1. **远端历史对象清理**：需 Owner 明确批准；影响面为公开仓库中 `259bd11` 悬空对象（含 29 处凭证字面量），无活动 refs 指向；改写的 refs / 提交范围仅为 `main`，协作者同步风险为所有已 clone 者需 `git fetch --force` 或重新 clone；回滚方案为保留新 `main` 前的完整 bundle 备份；验证办法为清除后重新请求 commit API / raw 接口应返回 404。本轮**未执行**任何此类动作。
2. 仓库是否改回 Private、是否联系 GitHub Support —— 待 Owner 决策。
3. Owner/CI 对 R0-E-35 的独立复跑，及慢速网络、有数据态、权限不足、离线 / 同步冲突、多角色场景。
4. 端到端隔离采集在干净 clone 中的完整重跑（需 Owner 提供测试账号凭证或 CI 环境）。
5. 截图基线整体补齐、逐控件运行时验收、逐路由逐用例断言与 `go test -race`、Owner 裁决 D-1~D-8、RV-04 的 Owner 决策记录。
6. 历史 / 外部审阅文档中遗留的个人绝对路径是否统一清理。

> **R0 未通过 / 阻塞；不授权启动 R1；PWA 未实施。**
