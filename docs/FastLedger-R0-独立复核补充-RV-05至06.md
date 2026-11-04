# FastLedger R0 独立复核补充：R0-RV-05～06

> 日期：2026-11-04  
> 编制：WorkBuddy（依据第三方独立审阅的《R0 第三次最小整改任务单》§5 归档）  
> 执行基准：`273cf0869917565b60f7da1dd2b562764de8d405`（开工前 `git fetch` 得到的 `origin/main` HEAD）  
> 业务代码证据基准：`bff6a2b351bf5a039cdf7c762ec2693fb330cd00`（本轮无业务代码改动）  
> 字段与状态机依据：`workbuddy/多AI对齐规范.md` §2 / §5；本文**不覆盖或改写** `docs/FastLedger-R0-独立复核补充-RV-01至04.md` 的历史结论。  
> **状态：R0 未通过 / 阻塞；不授权启动 R1；PWA 未实施。**

---

## R0-RV-05（P0）：公开仓库的凭证安全事件与历史对象残留

| 字段 | 内容 |
|---|---|
| `id` | `R0-RV-05` |
| `title` | 运行时日志曾泄露会话 JWT，当前树虽已脱敏但旧提交对象仍可公开读取，且公开文档留有可复制的固定测试口令 |
| `severity` | **P0**（规范 §3：安全被直接绕过 / 隐私风险；任务单列为 P0） |
| `location` | `docs/FastLedger-R0-第三次最小整改任务单-RV-02-RV-05-RV-06.md` §1.1–1.3；`docs/r0-screenshots/runtime/network-log.json`；`docs/r0-screenshots/runtime/README.md` 旧版 L10 / L30；`CHANGELOG.md` [1.66] 旧版 L25–L26 |
| `quote` | 任务单 §1.1：「旧提交 `259bd11…` 的旧版 `docs/r0-screenshots/runtime/network-log.json` 仍可通过 GitHub 文件接口按 SHA 读取」「当前 `docs/r0-screenshots/runtime/README.md` 仍明文写有固定测试用户名/密码，并把历史清理描述为已完成」；[1.66] 旧标题：「**处置 3 · 清除历史中的明文（已完成）**」 |
| `problem` | 根因是采集脚本把 CDP 事件 `params` 原样写入日志，`Network.requestWillBeSent` 的请求头携带 `Authorization: Bearer <JWT>`（29 处重复）。衍生问题：① 固定测试口令写入公开文档，可被复制复用；② 文档把「本地重写 + `gc`」笼统说成「历史明文已清除」，而 force-push 不改变 GitHub 端悬空对象的可读性；③ 脱敏逻辑只覆盖部分键名与 JWT 字面量，未系统性覆盖响应头、`headersText`、URL 查询参数与 JSON 请求体，且无合成凭证自动化测试 |
| `reproduce` | ① `node tools/r0/scan-credentials.js --strict`（清理前）→ 真实凭证候选 2、`EXIT_CODE=2`；② 匿名请求 `api.github.com/repos/LinTia/FastLadger/commits/259bd11` 与 `raw.githubusercontent.com/.../259bd11/.../network-log.json` → 均 HTTP 200，后者仍含 29 处凭证字面量；③ 用该令牌请求 `GET /api/v1/users/me` → HTTP 401；④ `node --test tools/r0/test-redaction.js` 验证脱敏覆盖面。详见 `docs/FastLedger-R0-凭证安全核验记录.md` |
| `actual` | 清理前：当前树 2 处可复制固定口令、`--strict` 退出码 2；远端旧对象 HTTP 200 且含 29 处凭证字面量；[1.66] 状态描述四项混为一谈。清理后：当前树 `--strict` 真实凭证候选 **0**、`EXIT_CODE=0`；6 个合成凭证断言全通过；旧对象仍 HTTP 200（**未清除**）；旧令牌 HTTP 401（已失效）；[1.66] 已拆分四项状态 |
| `expected` | 任务单 §1.3：当前树无真实凭证命中且有合成假凭证测试证明脱敏有效；固定测试口令不以另一个固定口令替代；旧令牌失效有脱敏验证记录；旧 SHA 公共可读性有最新实测记录；未获授权时历史清理明确标「待 Owner 决策」；生产影响仅按可验证证据表述 |
| `impact` | 仓库为 Public，旧悬空对象可被匿名取得；但令牌已撤销且密钥已轮换，实测无法通过鉴权。剩余风险是**历史对象未清除**与**公开文档中的可复制口令**（后者本轮已消除）。该令牌由本地隔离测试实例的一次性随机密钥签发，与生产实例密钥无关，生产是否受影响不外推 |
| `suggestion` | ① 当前树脱敏 + 固定口令改为环境变量 / 占位符；② `tools/r0/redaction.js` 统一脱敏 + 合成假凭证自动化测试 + 写盘前原始令牌自检；③ 保留 145 条事件结构，只移除凭证值；④ 如实区分四项状态；⑤ 远端对象清理提交影响面、协作者风险、回滚与验证方案，等 Owner 批准 |
| `status` | **已修复（当前树与证据层）／远端历史清理仍为待 Owner 决策**。按任务单 §1.2.6 拆分为四项：<br>1. **当前 `main` 文件已脱敏** ✅ 已完成并复扫 0 命中；<br>2. **泄露凭证已失效** ✅ 已完成并受控复验 401；<br>3. **旧 SHA `259bd11` 仍可公开读取** ⚠️ 已实测、已披露，**未清除**；<br>4. **远端历史对象清理待 Owner 决策** ⛔ 本轮未 force-push / 未重写公开历史 / 未删 refs / 未改仓库可见性 / 未联系 GitHub Support |
| `evidence` | `docs/r0-screenshots/security-scan.md`（`--strict` 0 命中 / `EXIT_CODE=0`）；`docs/FastLedger-R0-凭证安全核验记录.md` §2–§4（扫描基线、脱敏测试、受控 401、远端 200、refs 清单）；`tools/r0/test-redaction.js` 6/6 PASS；`CHANGELOG.md` [1.66] 状态拆分与 [1.68]。**本项不因当前树清理完成而视为整体关闭** |

---

## R0-RV-06（P2）：运行时复现不可移植、不可独立运行

| 字段 | 内容 |
|---|---|
| `id` | `R0-RV-06` |
| `title` | 隔离复验依赖仓库外私有脚本与个人绝对路径，第三方无法只凭仓库内容复现 |
| `severity` | **P2**（规范 §3：可维护性 / 跨文档一致性；任务单列为 P2） |
| `location` | 任务单 §3.1–3.3；旧版 `docs/r0-screenshots/runtime/README.md` L24 / L26 / L31 / L32（个人绝对路径）、L10 / L30（固定口令）；工作区 `.tmp_shot/rv01-isolation.js`（未入库） |
| `quote` | 任务单 §3.1：「runtime README 的复现命令仍依赖 `C:/Users/47006/WorkBuddy/fastledger` 等个人绝对路径」「实际运行脚本 `.tmp_shot/rv01-isolation.js` 未提交」「QA 两轮独立复跑和 SHA256 前后 0 变化陈述缺少完整可检查的执行记录」 |
| `problem` | 脚本 `OUT` 硬编码到正式证据目录、`REPO`/`WS`/`EDGE` 为个人绝对路径、token 从工作区文件读取；复现命令含固定口令；无合成凭证测试；无干净 clone 可追溯记录；把同一执行者的重复运行当作「独立复跑」 |
| `reproduce` | 按 `tools/r0/README.md` 从仓库根执行：`node --test tools/r0/test-redaction.js`、`node tools/r0/scan-credentials.js --strict`、`node tools/r0/rv01-isolation.js`（缺 `R0_TOKEN` / 浏览器 / 危险开关确认三种安全失败路径）；干净 clone 复现记录见核验记录 §5 |
| `actual` | 脚本已入库 `tools/r0/rv01-isolation.js`；4 个脚本 `node --check` 全 `EXIT=0`；干净 clone 中 6/6 脱敏测试通过、`--strict` 0 命中、`EXIT=0`；缺 `R0_TOKEN` → `EXIT=2`；缺浏览器 → `EXIT=2`；危险开关未确认 → `EXIT=3`；`tools/r0/` 5 个文件 SHA256 已记录。**端到端浏览器采集本轮未重跑**（需本地服务、测试账号凭证与 Edge，属明确缺失的外部依赖），归档的 5 PNG + 3 JSON 证据未被覆盖 |
| `expected` | 任务单 §3.3：干净 clone 中脚本能运行到预期断言或因明确缺失的外部依赖安全失败；仓库内脚本无硬编码个人绝对路径；默认运行不写 / 不覆盖正式证据目录；自动化脱敏测试通过且日志不含合成凭证；QA 记录可被第三方按命令、版本、退出码、SHA256 独立核对 |
| `impact` | 第三方此前无法复核 RV-01 运行时证据；且直接重跑旧脚本会覆盖归档证据。本轮后复现路径可移植、默认安全，但**端到端采集仍需外部依赖**，故 R0-E-35 的 Owner/CI 独立复跑仍为未完成项 |
| `suggestion` | 提交可移植脚本；路径基于脚本位置 / 仓库根 / 环境变量；凭证只从 `R0_TOKEN` 读取且缺失即安全失败；默认输出到系统临时目录；写正式证据目录须双重确认；补合成凭证测试与干净 clone 执行记录；明确平台限制 |
| `status` | **已修复（脚本可移植性与证据保护）／端到端采集复跑与 Owner/CI 独立复跑仍待办**。按任务单 §5.4 分项：<br>1. **脚本是否提交** ✅ `tools/r0/` 5 个文件入库；<br>2. **路径是否可移植** ✅ 脚本内 0 处个人绝对路径（全仓 `--strict` 报告 `PERSONAL_ABS_PATH` 仅存于历史 / 外部审阅文档，见核验记录 §6）；<br>3. **干净 clone 是否成功** ✅ 8 项命令全部得到预期退出码（见核验记录 §5）；<br>4. **自动脱敏测试** ✅ 6 用例全通过，输出无合成凭证；<br>5. **QA 独立复跑证据** ✅ 独立 QA 子代理在自建干净 clone 完成 11 条命令 + 全树只读复核，结论 **ALLOW（限定范围）**、无 FAIL（回执 §3.2）；R0 整体验收仍 BLOCK。**但同处一个工程环境，不等同 Owner/CI 独立复跑**，后者仍待办 |
| `evidence` | `tools/r0/README.md`（依赖 / 环境变量 / 退出码 / 平台限制）；`docs/FastLedger-R0-凭证安全核验记录.md` §5（干净 clone 8 项命令、输出摘要、退出码、5 个文件 SHA256）；`docs/r0-screenshots/security-scan.md`；`docs/FastLedger-R0-第三次整改交付回执.md` §3.2（独立 QA ALLOW 报告） |

---

## 归档边界

- 本补充只登记 `R0-RV-05` / `R0-RV-06` 两项；`R0-RV-01～04` 的原文与结论保留在 `docs/FastLedger-R0-独立复核补充-RV-01至04.md`，未覆盖、未改写。
- 本文不把「当前树已脱敏」「干净 clone 安全失败路径通过」表述为「安全问题彻底清除」「独立复跑通过」或「R0 通过」。
- 远端历史对象清理、仓库改 Private、联系 GitHub Support、Owner/CI 独立复跑均为 **Owner 待决策 / 待办**，清单见 `docs/FastLedger-R0-第三次整改交付回执.md` §5。
