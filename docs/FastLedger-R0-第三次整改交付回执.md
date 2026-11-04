# FastLedger R0 第三次最小整改 · 逐项交付回执（[1.68]）

> 依据：`docs/FastLedger-R0-第三次最小整改任务单-RV-02-RV-05-RV-06.md` §7 的回执格式，逐项填写。  
> 执行基准：开工前 `git fetch` 得到的 `origin/main` HEAD `273cf0869917565b60f7da1dd2b562764de8d405`。  
> 口径：**R0 未通过 / 阻塞；不授权启动 R1；PWA 未实施。** 本回执不代表 R0 验收通过，也不宣布安全事件整体关闭。  
> 所有敏感检查只记录路径、命中类型、数量、脱敏上下文与退出码，**不含任何真实凭证值**。

---

## 1. 逐项整改回执（任务单 §7 的 11 行）

| 项目 | 修改文件 / 位置 | 实际执行命令或操作 | 真实结果 / 退出码 | 证据链接或 SHA256 | 状态 |
|---|---|---|---|---|---|
| **RV-05 当前树凭证扫描** | `tools/r0/scan-credentials.js`（新增）；`docs/r0-screenshots/security-scan.md`（新增）；`docs/FastLedger-R0-凭证安全核验记录.md` §2 | `node tools/r0/scan-credentials.js --strict`（清理前 / 后各一次）；`… --strict --out docs/r0-screenshots/security-scan.md` | 清理前 `真实凭证候选命中：2`、`EXIT_CODE=2`；清理后 `扫描文本文件：449` / `真实凭证候选命中：0` / 合成 48、复核 14、夹具 27、可移植性 18、`EXIT_CODE=0`；连续两次生成统计一致 | `docs/r0-screenshots/security-scan.md`；清理前基线留档工作区 `.tmp_shot/scan_before.json`（不入库） | **已完成** |
| **RV-05 旧令牌失效验证** | `docs/FastLedger-R0-凭证安全核验记录.md` §4；`CHANGELOG.md` [1.66] / [1.68] | 一次性启动本地隔离实例 → 从远端旧对象取回令牌（仅内存）→ `GET /api/v1/users/me` → 终止进程 | `/healthz` HTTP 200 `status:"up"` / schemaVersion 8；**HTTP 401「登录已失效，请重新登录」**；一次性进程 `terminate` 成功，无常驻残留 | 核验记录 §4；`CHANGELOG.md` [1.66] 状态拆分第 2 项 | **已完成**（仅一次受控验证，未用真实令牌做回归样例） |
| **RV-05 旧 SHA 公共可读性** | 核验记录 §4；`CHANGELOG.md` [1.66] / [1.68]；`CHANGELOG.md` [1.68] | 匿名 GET commit API 与 raw 文件接口；`git ls-remote --heads --tags origin` | commit API **HTTP 200**；raw 旧日志 **HTTP 200**、`bytes=182281`、**仍含 29 处凭证字面量**；远端 refs 仅 `main` + `fix/windows-first-build`，无活动 ref 指向 `259bd11` | 核验记录 §4；`docs/r0-screenshots/security-scan.md` | **已完成（如实披露，未清除）** |
| **RV-05 历史清理 Owner 授权** | `CHANGELOG.md` [1.66] / [1.68]、核验记录 §4 / §7、本回执 §5 | 仅提交影响面、refs 范围、协作者风险、回滚与验证方案；**未执行**任何 force-push / 历史重写 / 删 refs / 改可见性 / 联系 Support | 本轮 `git push` 为普通推送；本地未再 `gc` 远端对象 | 本回执 §5 | **待 Owner 决策（未执行）** |
| **RV-05 README 固定密码清理** | `docs/r0-screenshots/runtime/README.md`（整体重写）、`docs/FastLedger-新手开发完整指南.md` §7.3 示例、`CHANGELOG.md` [1.65] 历史命令注记 | 精确字符串替换 + 全库复扫 | 已知固定测试口令字面量 **0**；严格扫描 `JSON_SECRET_FIELD` 真实候选 **0**；改为 `FL_TEST_USER` / `FL_TEST_PASSWORD` 环境变量与 `<your-password>` 占位符，未以另一固定口令替代 | `docs/r0-screenshots/runtime/README.md`；`tools/r0/README.md` | **已完成** |
| **RV-02 全文 SHA 口径检查** | 主报告 §0 / §0.2 / §3.1 / §5.3 引言 / §5.3.0 / §5.3.4 / §10.3 / §10.4；`CHANGELOG.md` [1.68]；`README.md`；runtime README | 全文 grep `273cf08` / `557e4d2` / `bff6a2b` / `1cd56ab` / `ddbd1ae` / `575eae3` 与「当前 HEAD / 文档 HEAD / 本轮基准 / 采集时 HEAD」，逐处判断语义并精确替换 | §0.2 五类基准齐备；主报告「当前 HEAD」残留 **0**；`557e4d2` 仅保留历史时点语义；外部审阅产物与历史 CHANGELOG 条目按原文保留 | 核验记录 §6（命中清单）；主报告 §0.2 | **已完成** |
| **RV-06 脚本提交与安全测试** | `tools/r0/{rv01-isolation.js,redaction.js,test-redaction.js,scan-credentials.js,README.md}`（新增） | `node --check` ×4；`node --test tools/r0/test-redaction.js` | 4 个脚本语法 `EXIT=0`；6 tests / **6 pass / 0 fail** / `EXIT_CODE=0`；合成 JWT / Cookie / 口令 / API Key / 查询参数 / 非 JWT Bearer 全部不可见于输出 | `tools/r0/README.md`；核验记录 §3；5 个文件 SHA256 见核验记录 §5 | **已完成** |
| **RV-06 干净 clone 复现** | 核验记录 §5；`tools/r0/README.md` | 工作树导出为临时 git 仓库 → `git clone` 到独立目录（468 文件、无私有目录）→ 8 条命令逐条执行 | `node --check`×4 `EXIT=0`；脱敏测试 `EXIT=0`；`--strict` 0 命中 `EXIT=0`；缺 `R0_TOKEN` `EXIT=2`；缺浏览器 `EXIT=2`；危险开关未确认 `EXIT=3`；`sha256sum` `EXIT=0` | 核验记录 §5（命令 / 输出 / 退出码 / 5 个 SHA256） | **已完成（含预期安全失败路径）**；端到端浏览器采集本轮未重跑 |
| **RV-06 QA 独立复跑** | 本回执 §3.2（QA 补记）；`docs/FastLedger-R0-凭证安全核验记录.md` §5 | 已派发独立 QA 子代理在**自建**干净 clone 中复跑；WorkBuddy 自建干净 clone 8 项已完成 | 独立 QA 截至本回执落盘**未回报结论**（其临时目录已有 clone 与检查输出，但未形成报告）；WorkBuddy 自测 8 项退出码全部符合预期 | 本回执 §3.2；核验记录 §5 | **WorkBuddy 干净 clone 自测已完成；独立 QA 结果待补；Owner/CI 独立复跑未完成**（不同执行者的重复运行不冒充 Owner/CI） |
| **RV-01 状态口径统一** | 主报告 §3 / §3.1 / §7.5 / §8 / §9 / §10.3 / §10.4；`docs/r0-screenshots/runtime/README.md`；`CHANGELOG.md` [1.65] / [1.66] / [1.68]；`README.md` 导航 | 精确替换全部状态旧称；新增标准口径段落 | 交付文档中状态旧称 **0**；统一为「本机隔离运行时复验已完成；失败态观察到骨架屏未清理（`.skel`=4、`aria-busy=true`）；待 Owner/CI 独立复跑及慢速网络、有数据态、多角色等场景验证；**缺陷尚未关闭**」；无「已修复 / 已关闭」表述 | 主报告 §7.5 / §8 / §10.4；`CHANGELOG.md` [1.68] | **已完成** |
| **RV-05 / RV-06 finding 归档** | `docs/FastLedger-R0-独立复核补充-RV-05至06.md`（新增） | 按 `workbuddy/多AI对齐规范.md` §2 的 13 字段逐项填写；RV-05 拆四项状态、RV-06 拆五项状态 | 2 条 finding 字段齐备；`docs/FastLedger-R0-独立复核补充-RV-01至04.md` 与第三次整改任务单 **未被改动** | 新增文档；`git status --short` 核对 | **已完成** |

---

## 2. 修改文件清单

**修订（tracked）**

| 文件 | 改动 |
|---|---|
| `CHANGELOG.md` | 新增 [1.68]；[1.66] 状态四项拆分并更正「处置 3」标题；[1.65] 回填 R0-E-35 状态口径与历史命令安全注记 |
| `README.md` | 状态快照加 [1.68]；导航新增 6 行（RV-05~06 补充、核验记录、交付回执、`tools/r0/`、扫描报告、任务单状态更新）；runtime 行状态口径对齐 |
| `docs/FastLedger-R0-基准冻结与差异盘点.md` | §0 / §0.2 五类基准；§3.1 / §7.5 / §8 / §10.3 的 R0-E-35 标准口径；§5.3 引言 / §5.3.0 / §5.3.4 的 SHA 语义；§9 证据索引补 5 行；新增 §10.4 |
| `docs/r0-screenshots/runtime/README.md` | 整体重写：去固定口令与个人绝对路径、指向 `tools/r0/`、状态标准口径、凭证安全四项拆分、平台限制 |
| `docs/FastLedger-新手开发完整指南.md` | §7.3 登录请求示例口令改为 `<your-password>` 占位符 |

**新增（untracked → 入库）**

| 路径 | 内容 |
|---|---|
| `tools/r0/rv01-isolation.js` | 可移植隔离复现脚本（环境变量、仓库外默认输出、危险开关双重确认、写盘前脱敏自检） |
| `tools/r0/redaction.js` | 统一脱敏模块 |
| `tools/r0/test-redaction.js` | 合成假凭证自动化测试（6 用例） |
| `tools/r0/scan-credentials.js` | 全仓凭证扫描器（`--strict` / `--json` / `--out`） |
| `tools/r0/README.md` | 依赖、环境变量、服务与账号准备、步骤、退出码、清理、平台限制 |
| `docs/r0-screenshots/security-scan.md` | 脱敏扫描报告 |
| `docs/FastLedger-R0-独立复核补充-RV-05至06.md` | 13 字段 finding 归档 |
| `docs/FastLedger-R0-凭证安全核验记录.md` | 真实执行记录（扫描 / 测试 / 受控验证 / 干净 clone / SHA 命中清单） |
| `docs/FastLedger-R0-第三次整改交付回执.md` | 本文件 |

**未改动**：业务代码（`server/`、`client/`、`server/web/dist/`）、PRD、落地方案、WBS、枚举字典、数据库结构与 API、外部审阅产物（RV-01至04 补充、第三次整改任务单）、归档的 5 PNG + 3 JSON、`docs/r0-screenshots/` 下 43 张首屏截图。

---

## 3. 真实测试证据汇总

| 命令 | 真实结果 | 退出码 |
|---|---|---|
| `node --version` | `v22.22.2` | 0 |
| `node --check` × 4 脚本 | 全部通过 | 0 |
| `node --test tools/r0/test-redaction.js` | `# tests 6 / # pass 6 / # fail 0` | 0 |
| `node tools/r0/scan-credentials.js --strict`（清理前） | `真实凭证候选命中：2` | 2 |
| `node tools/r0/scan-credentials.js --strict`（清理后） | `扫描文本文件：449` / `真实凭证候选命中：0` | 0 |
| `node tools/r0/scan-credentials.js --strict --out docs/r0-screenshots/security-scan.md` | 统计与上一致，连续两次幂等 | 0 |
| 干净 clone 8 项命令 | 见核验记录 §5：0 / 0 / 0 / 0 / 2 / 2 / 3 / 0 | 全部符合预期 |
| 旧令牌受控验证 | `GET /api/v1/users/me` → **HTTP 401** | — |
| 旧 SHA 公共可读性 | commit API / raw 均 **HTTP 200** | — |
| `git diff --check` | 无输出 | 0 |
| `go test ./...` | **本轮未执行**（无业务代码改动）；[1.63] / [1.65] 的 10 包 ok / 2 包无测试 / 0 FAIL 属历史记录，不作为本轮证据 | — |

### 3.2 QA 独立复跑

已派发独立 QA 子代理在**自建**的干净 clone 中复跑与全树口径复核；截至本回执落盘，该子代理**尚未回报结论**（临时目录 `.tmp_shot/qa-rv3/` 中已可见 clone、检查脚本与 `--strict` 输出，但未形成 PASS/FAIL 报告），故本轮**不把它的结果写成已完成**。

本轮可引用的复跑证据仅为 WorkBuddy 自建干净 clone 的 8 项记录（`docs/FastLedger-R0-凭证安全核验记录.md` §5：退出码 0 / 0 / 0 / 0 / 2 / 2 / 3 / 0，5 个脚本 SHA256 可与仓库文件核对）。这些属于**同一执行者的自测**；即便后续 QA 子代理回报，也仍属同一工程环境，**不等同 Owner/CI 独立复跑**。后者列为未完成项（§4），并在推送后通知第三方独立复核。

---

## 4. 未完成项（如实列出，不降低 R0 门槛、不挪到 R1）

1. **远端 Git 历史对象清理**：`259bd11` 悬空对象仍可匿名读取，未获 Owner 批准前不执行（详见 §5）。
2. **Owner/CI 对 R0-E-35 的独立复跑**，及慢速网络、有数据态、权限不足、离线 / 同步冲突、多角色、原型同视口对照等场景。
3. **端到端隔离采集在干净 clone 中的完整重跑**：需本地服务、测试账号凭证与 Edge；本轮只完成了可移植性与安全失败路径验证，归档 5 PNG + 3 JSON 未被覆盖。
4. 截图基线整体补齐、逐控件运行时验收、逐路由逐用例断言与 `go test -race`、Owner 裁决 D-1~D-8、RV-04 的 Owner 决策记录。
5. 历史 / 外部审阅文档（交接文档、评审卡片、任务单举例）中遗留的个人绝对路径是否统一清理 —— 本轮未擅自改写外部产物，命中清单见 `docs/r0-screenshots/security-scan.md`。
6. `review` / `test-fixture` 类扫描命中（历史部署示例密钥、Go 测试常量）是否需人工判定为真实凭证 —— 未自动判为泄露，也未忽略。

---

## 5. Owner 待决策项与历史清理方案（任务单 §1.2.6，仅方案，未执行）

| 项 | 内容 |
|---|---|
| **影响面** | Public 仓库 `LinTia/FastLadger` 中旧提交 `259bd11` 的悬空对象（含 29 处凭证字面量的旧 `network-log.json`）；当前 `main` 不含明文，令牌已撤销、密钥已轮换。 |
| **将被改写的 refs / 提交范围** | 若 Owner 批准「请求 GitHub 清除缓存 / 不可达对象」，不涉及改写 refs；若批准「再次重写公开历史」，范围仅为 `main` 上 `259bd11` 之后的历史（需重新计算所有后续提交 SHA），`fix/windows-first-build` 分支需单独评估。两种方案**本轮均未执行**。 |
| **协作者同步风险** | 再次重写 `main` 会使所有已 clone 者本地历史分叉，须 `git fetch` 后 reset / 重新 clone；期间其他 AI 的复核基线（本回执 §0 的 `273cf08`、主报告 §0.2）需同步更新，可能产生新的 SHA 口径不一致。 |
| **回滚方案** | 执行前创建完整 `git bundle` 备份与 `origin/main` 当前 SHA 记录；如需回滚，用备份恢复 refs 并通知协作者。 |
| **验证办法** | 清除后重新匿名请求 commit API 与 raw 接口应返回 404；`git ls-remote` 确认 refs 正常；`node tools/r0/scan-credentials.js --strict` 仍为 0 命中；R0 文档基准表按新 SHA 回填。 |
| **其他待决策** | 仓库是否改回 Private；是否由 Owner 联系 GitHub Support；是否统一清理历史文档中的个人绝对路径；是否删除脱敏版日志只保留索引。 |

> **本轮未 force-push、未重写公开历史、未删除 refs、未切换仓库可见性、未联系 GitHub Support。**

---

## 6. 最终门禁

- **R0 未通过 / 阻塞**；R0-E-35 缺陷尚未关闭；不授权启动 R1；PWA 未实施。
- 本回执不声称「安全问题彻底清除」「独立复跑通过」「脚本跨平台」或「R0 通过」。
- 所有结论以真实命令输出、退出码与仓库内可复核文件为依据；计划、脚本存在与口头确认均未写成已完成。
