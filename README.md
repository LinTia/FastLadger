# FastLedger · 家庭智能记账与资产管理系统

一款**自托管**（部署在家庭 NAS / Docker）的家庭智能记账系统，核心体验是 **截图 → 本地识别 → 人工确认 → 家庭共享**，并提供统一的家庭资产/负债与净资产视图。

> 数据在自己手里 · 识别优先在端侧完成 · 多端协同 · 家庭共享 · 资产全景

## 当前状态（2026-10-31 历史快照，部分已过时）

> 下表是 2026-10-31 的历史快照。schema 版本、测试基线、阶段进度以 `docs/` 最新交付文档、`CHANGELOG.md` 与服务器实测为准；**R0（基准冻结与差异盘点）当前未通过 / 阻塞**，见 [R0 报告](docs/FastLedger-R0-基准冻结与差异盘点.md)。本表不作为 R0 验收依据。
> **截至 [1.85] 的 R0 收尾状态（第十一次独立复核）**：PR #3 最新 HEAD 为 `858b7b45`，仍 OPEN、未合并。RV-13 已 PASS（最新 Artifact `13521151959` 的唯一根清单 101/101 字节校验通过）；RV-15 已 PASS（前轮真实负向 gate 证据仍有效，本轮正向回归成功）；RV-14 仍 PARTIAL：新增 case-51~53 且 53/53 测试通过，但 case-51 的 OR 断言允许 BEARER_TOKEN 掩盖 Authorization 专项检测回归，要求强制 AUTH_HEADER 断言。A05 仍未断言慢网 pending 时 disabled；C 为 **164/4/59/1**；B 的 t021 Owner 决策待确认；D 的 `OBS-ACCT-VIS-390` 仍 FAIL；RV-05 仍 **Owner ACTION REQUIRED**。**R0 继续 BLOCK，不授权 R1–R7 / PWA。** 详见 [第十一次 RV-14 补齐独立复核](docs/FastLedger-R0-第十一次RV-14补齐独立复核.md)。

| 项 | 状态 |
|---|---|
| 后端实现 | **阶段 0–4 已交付**：认证/账本多成员、记账核心、离线同步 + WebSocket、删除/恢复 + 回收站 + 审计 |
| 阶段 5 | 多端会话撤销（WBS 5.10）**已交付生产**；Flutter / Web 原型还原与 PWA 按 R0–R7 分阶段推进，**R0 未通过，R1–R7 未授权实施** |
| 数据库 schema | **仓库当前 v8**（`server/internal/migrate/migrations/` 0001–0008，受 CI 指纹守卫保护：已发布迁移禁止修改）；生产实例部署状态以部署交接文档为准 |
| 测试基线 | 历史快照：9 个包全绿、645 PASS / 0 FAIL / 5 SKIP、`DATA RACE=0`（CI 侧 `-race`）；**本轮 R0 二次整改实测 `go test ./...`：10 包 ok / 2 包无测试 / 0 FAIL / EXIT_CODE=0（详见 R0 报告 §7.6）；`go test -race` 与 CI 侧结果仍以 CI 为准** |
| API 路由 | 交易路由白/黑名单、单一创建入口、可见性条件已由架构守卫用例固化；逐路由权限 / DTO / 错误码 / 测试证据见 [R0 报告 §5.3](docs/FastLedger-R0-基准冻结与差异盘点.md) |

> **部署交接入口（给运维/云厂商专家）**：
> [`docs/FastLedger-部署交接-腾讯云轻量应用服务器.md`](docs/FastLedger-部署交接-腾讯云轻量应用服务器.md)
> —— 环境与依赖、编译启动、端口与防火墙、环境变量全量表、持久化与备份、规格建议、验收清单、P0/P1/P2 待确认事项与已知风险。

## 产品特性

- **截图秒记**：手机/电脑截图后，端侧 OCR + 规则解析，预填确认即可入账；识别结果必须人工确认。
- **隐私优先**：图片不上传，识别默认在端侧完成；云端大模型仅作可选兜底（只传文本）。
- **多端协同**：Windows / Android / iOS 客户端（Flutter）+ Web 快捷浏览（无 OCR）。
- **家庭共享**：多账本、多成员（2 人起步，可扩展到 5+）、角色与可见性可控。
- **账目不重复**：资金账户与支付渠道分离，正确处理微信/支付宝/信用卡的同一笔消费。
- **资产全景**：现金、信用卡、多券商、基金、房产、车辆、黄金统一到家庭净资产；预留自动行情。

## 仓库导航

| 路径 | 内容 | 用途 |
|---|---|---|
| [`server/`](server/) | **Go 后端（已实现，阶段 0–4）** | 可编译运行的 API 服务：[server/README.md](server/README.md) 含构建/迁移/启动说明 |
| [`docs/FastLedger-部署交接-腾讯云轻量应用服务器.md`](docs/FastLedger-部署交接-腾讯云轻量应用服务器.md) | **部署交接说明（给云厂商/运维专家）** | 环境依赖、端口、环境变量、持久化、规格、验收、风险 |
| [`prototype/`](prototype/) | 完整可交互原型（13 个核心页面，**零外部请求、可离线**） | 看产品长什么样、怎么交互，**可直接在浏览器运行**；**公网在线版：[点此直接体验](https://a38ce1ad4e917321e.app.workbuddy.host/?sharecode=BPE6OFTPKE)**（由 WorkBuddy 托管，无需本地构建） |
| [`docs/FastLedger-产品需求文档-PRD.md`](docs/FastLedger-产品需求文档-PRD.md) | 产品需求文档（PRD） | 做什么、为什么、范围与验收 |
| [`docs/FastLedger-落地方案.md`](docs/FastLedger-落地方案.md) | 软件开发 8 环节落地方案 | 整体技术方案、数据模型、API、版本规划 |
| [`docs/FastLedger-手机PWA实施指导书.md`](docs/FastLedger-手机PWA实施指导书.md) | 手机 PWA 实施指导书 | 完整移动端、完整离线、IndexedDB、同步、离线分析与本地备份恢复；指导后续 PRD / 落地方案 / WBS 修订 |
| [`docs/FastLadger-Web-PWA视觉设计规范.md`](docs/FastLadger-Web-PWA视觉设计规范.md) | **Web / PWA 视觉设计规范** | 以 Panda-995/homeledger 为视觉参考的设计令牌、桌面/手机布局、组件与验收标准 |
| [`docs/FastLadger-UI-DESIGN-01-Web页面与组件盘点报告.md`](docs/FastLadger-UI-DESIGN-01-Web页面与组件盘点报告.md) | UI-DESIGN-01 页面与组件盘点（R1 修订版） | WorkBuddy 对当前 Web 的**静态盘点**：**17 个路由视图 / 1 个应用壳层 / 7 个对话框**（分别统计）+ 导航映射表 + **§4.7 已要求但缺失能力矩阵**。已按第三方复核 **UI01-RV-01~11** 逐项修订并逐条标记 FIXED/PARTIAL/OPEN（见报告 §10 与复核报告 §6）；首页结余口径已由 Owner 确认（当前账本本月收入 − 本月支出，转账不计入）。**结论为静态文档/代码层，无浏览器/真机验证**；UI-DESIGN-01 是否通过待第三方复核，R0 仍 BLOCK |
| [`docs/FastLadger-UI-DESIGN-01独立复核报告.md`](docs/FastLadger-UI-DESIGN-01独立复核报告.md) | UI-DESIGN-01 独立复核 | 复核结论 **PARTIAL / REVISE**：D-3 桌面导航冲突、D-8 区块边界错误、D-7 等需求遗漏、首页结余/净资产口径混淆、02a 违反 R0 门禁等 **UI01-RV-01~11**；R1 已逐项修订，**待再次复核** |
| [`docs/FastLedger-原型还原与PWA分阶段实施总方案.md`](docs/FastLedger-原型还原与PWA分阶段实施总方案.md) | **原型还原与 PWA 分阶段实施总方案** | R0–R7 总路线、原型差异盘点、AI 分批实施纪律、移动端/PWA/离线/备份验收门禁 |
| [`docs/FastLedger-R0-基准冻结与差异盘点.md`](docs/FastLedger-R0-基准冻结与差异盘点.md) | **R0 基准冻结与差异盘点（第三次整改后）** | 13 页/逐控件交互矩阵（228 条）、两版原型差异、当前实现与后端 API 权限/DTO/错误码/测试映射、差异根因、截图基线执行结果与阻塞清单、Owner 裁决清单；**R0 未通过 / 阻塞** |
| [`docs/FastLedger-R0-收尾执行清单.md`](docs/FastLedger-R0-收尾执行清单.md) | **R0 收尾执行清单（2026-11-05）** | 统一区分 R0 审计门槛与后续业务实现；汇总 RV 状态、D-4/D-8 待补、Owner 决策、截图/运行时/API 测试缺口与最终放行条件 |
| [`docs/FastLedger-R0-第四次证据补齐任务单.md`](docs/FastLedger-R0-第四次证据补齐任务单.md) | **R0 第四次证据补齐任务单（WorkBuddy）** | R0-E-35 独立复验、13 页同视口对照、228 条控件运行时验收、API/race 测试证据；不授权业务代码修改或启动 R1–R7 |
| [`docs/FastLedger-R0-第五次最小整改任务单.md`](docs/FastLedger-R0-第五次最小整改任务单.md) | **R0 第五次最小整改任务单（WorkBuddy）** | 第四次独立复核纠偏：A/C BLOCKED、B/D PARTIAL；分类映射、角色/状态证据、CI 元数据、未覆盖路由与 RV-05 Owner 安全动作；不授权业务代码修改或启动 R1–R7 |
| [`docs/FastLedger-R0-第四次证据补齐-执行回执.md`](docs/FastLedger-R0-第四次证据补齐-执行回执.md) | **R0 第四次证据补齐 · 执行回执（历史记录，已被第五次复核纠偏）** | 原文写 B/D 为 PASS，**第五次复核改判 B = PARTIAL、D = PARTIAL**（见文件开头「第三方独立复核纠偏」小节）；A/C 仍 BLOCKED；历史执行事实保留 |
| [`docs/FastLedger-R0-第五次最小整改-执行回执.md`](docs/FastLedger-R0-第五次最小整改-执行回执.md) | **R0 第五次最小整改 · 执行回执** | 逐项回执：A BLOCKED / B PARTIAL / C BLOCKED / D PARTIAL；分类映射修复前后、104 PNG 校验、角色/状态覆盖、CI 元数据更正、未覆盖路由处置、RV-05 Owner 待办与文档同步清单 |
| [`docs/r0-evidence-5th/`](docs/r0-evidence-5th/) | **R0 第五次整改证据** | `seed-mapping-before/after.json`（22 笔分类映射）、`checksum-verification.txt`（104 PNG 逐文件校验）、`api-route-coverage.md`（13 条路由）、`ci-metadata-correction.md`（run/job/耗时与四类 SHA）、`ci-coverage-matrix.md`（CI 能/不能证明什么）、`ci-run-de2fb26/`（全仓 `-race` 的 GitHub 原生 run）、`ci-run-22fd310/`（第五次整改提交的 run） |
| [`docs/FastLedger-R0-第六次整改-A-C自动化交付回执.md`](docs/FastLedger-R0-第六次整改-A-C自动化交付回执.md) | **R0 第六次整改 · A/C 自动化交付回执** | B 分类解析硬化（14 条合成测试 + 22 笔审计）；A 11/11 本地 PASS（待 CI 复核）；C 228 条 144/6/77/1；4 条新发现缺陷；CI 只需 Owner 三件手机小事 |
| [`docs/FastLedger-R0-第七次测试质量纠偏任务单.md`](docs/FastLedger-R0-第七次测试质量纠偏任务单.md) | **R0 第七次测试质量纠偏任务单（WorkBuddy）** | 修正 A05 假阳性、C 至少 15 条硬编码 PASS、控制清单 SHA 漂移校验、t021 Owner 裁决；不授权业务代码修改或启动 R1–R7 |
| [`docs/FastLedger-R0-第八次安全扫描器独立复核.md`](docs/FastLedger-R0-第八次安全扫描器独立复核.md) | **R0 第八次独立复核（WorkBuddy K1–K3 安全整改）** | RV-07–RV-12：PEM 私钥漏检、fail-closed 上传门禁、UTF-8 非法字节、Base64URL 与测试覆盖缺口；R0 仍 BLOCK |
| [`docs/r0-evidence-6th/`](docs/r0-evidence-6th/) | **R0 第六次整改证据** | `report.json/md`（239 条）、`controls-summary.json`、`unit-resolve-category.log`、`reseed-audit.log`、`screenshots/`、`SHA256SUMS.txt` |
| [`docs/FastLedger-R0-独立补漏与复核记录.md`](docs/FastLedger-R0-独立补漏与复核记录.md) | **R0 独立补漏与复核记录** | 修正误判、补充 R0-E-20~39、复核 R0 退出条件与 D-1~D-8；静态复核 + 二次整改复核，R0 尚未通过 |
| [`docs/FastLedger-R0-第三次最小整改任务单-RV-02-RV-05-RV-06.md`](docs/FastLedger-R0-第三次最小整改任务单-RV-02-RV-05-RV-06.md) | **R0 第三次最小整改任务单** | 第三方审阅任务单；[1.68] 已按此执行并推送，待第三方独立复核；Git 历史重写仍须 Owner 授权 |
| [`docs/FastLedger-R0-独立复核补充-RV-05至06.md`](docs/FastLedger-R0-独立复核补充-RV-05至06.md) | **独立复核补充 RV-05~06** | 按多 AI 对齐规范 13 字段登记凭证安全事件与独立复现问题；状态拆分、证据与 Owner 待决策项 |
| [`docs/FastLedger-R0-凭证安全核验记录.md`](docs/FastLedger-R0-凭证安全核验记录.md) | **RV-05 凭证安全核验记录** | 真实执行命令、退出码、脱敏扫描报告、旧令牌受控复验、旧 SHA 公共可读性、干净 clone 复现与 SHA256 |
| [`docs/FastLedger-R0-第三次整改交付回执.md`](docs/FastLedger-R0-第三次整改交付回执.md) | **第三次整改逐项交付回执** | 11 项回执表、修改文件清单、真实测试证据、未完成项与 Owner 待决策项 |
| [`tools/r0/`](tools/r0/) | **R0 可移植运行时工具** | `rv01-isolation.js` 隔离复现、`redaction.js` 统一脱敏、`test-redaction.js` 合成假凭证测试、`scan-credentials.js` 全仓扫描；默认输出在仓库外，见目录内 `README.md` |
| [`docs/r0-screenshots/security-scan.md`](docs/r0-screenshots/security-scan.md) | **凭证安全扫描报告（脱敏）** | `--strict` 真实凭证候选 0；只含路径 / 类型 / 数量 / 脱敏上下文，无敏感值 |
| [`docs/FastLedger-R0-独立复核补充-RV-01至04.md`](docs/FastLedger-R0-独立复核补充-RV-01至04.md) | **R0 二次整改后的独立复核补充** | R0-RV-01~04：骨架屏失败态证据、SHA 基准、逐控件字段语义、首页月份切换需求依据；R0 仍阻塞 |
| [`docs/r0-screenshots/`](docs/r0-screenshots/) | **R0 真实截图基线（43 张 PNG）+ 索引 / 阻塞清单** | Edge headless 采集的 normal / empty / error 首屏；见目录内 `README.md`。**截图基线未完成，R0 未通过** |
| [`docs/r0-screenshots/pairs/`](docs/r0-screenshots/pairs/) | **R0 包 B 同数据集成对截图（104 张）** | 原型 52 + 实现 52，13 页 × 4 视口；**同一数据集**（5 账户 / 37 分类 / 22 交易 / 5 预算，余额两侧一致）；含未登录态与记一笔弹层动作级；**第五次重采（分类映射修复后）**；`manifest` / `seed-manifest.json`（22 笔映射审计）/ `SHA256SUMS.txt` / `SHA256SUMS-4th-round.txt` / README。**包 B 状态 = PARTIAL**（仅 owner 角色、状态维度与 invest/members 未覆盖） |
| [`docs/r0-evidence-4th/`](docs/r0-evidence-4th/) | **R0 包 D 测试证据（正式 CI + 服务器）** | `ci-run/`：Actions run `41954006789` 元数据、原始 job 日志与证明行摘录；根目录：服务器 Linux 全仓 `go test ./...` 与 `-race` 日志、退出码、环境与 SHA256 |
| [`docs/r0-screenshots/runtime/`](docs/r0-screenshots/runtime/) | **R0-RV-01 隔离运行时复验证据（5 PNG + 3 JSON）** | 仅拦截交易列表请求后的 pending / rejected / 恢复态截图与真实 CDP Network/Console 日志；见目录内 `README.md`。**本机隔离运行时复验已完成；失败态观察到骨架屏未清理（`.skel`=4、`aria-busy=true`）；待 Owner/CI 独立复跑及慢速网络、有数据态、多角色等场景验证；缺陷尚未关闭，R0 未通过** |
| [`docs/FastLedger-新手开发完整指南.md`](docs/FastLedger-新手开发完整指南.md) | 新手开发完整指南（含可照敲代码骨架） | 应届生照着做即可开发 |
| [`docs/FastLedger-枚举字典.md`](docs/FastLedger-枚举字典.md) | 统一枚举字典（唯一权威来源） | 各端枚举值对齐 |
| [`docs/ezbookkeeping-代码导读.md`](docs/ezbookkeeping-代码导读.md) | 开源参考项目 ezbookkeeping 代码导读 | 可借鉴的设计与实现 |
| [`gpt/v1.4/`](gpt/v1.4/) | GPT 定向审核与真实运行核验 | v1.4 §6 复核、规范字段意见及入口裁决 |
| [`docs/FastLedger-转账入口裁决.md`](docs/FastLedger-转账入口裁决.md) | 唯一 Create / 转账事务边界 | API、幂等、版本与接线的统一规则 |
| [`CHANGELOG.md`](CHANGELOG.md) | 更新日志 | 各版本变更记录 |
| [`workbuddy/第三方审阅者导读.md`](workbuddy/第三方审阅者导读.md) | 第三方审阅者导读 | **新来审阅 / 提意见者的对齐入口**（背景、迭代史、审阅约定） |
| [`workbuddy/FastLedger-文档评审-D.md`](workbuddy/FastLedger-文档评审-D.md) | 第四个审阅者 D 工程评审 | D-R01–D-R06（2×P1+4×P2），聚焦 Gin/WS/Flutter/OCR/同步/迁移删除恢复 |
| [`workbuddy/FastLedger-D二次核查报告.md`](workbuddy/FastLedger-D二次核查报告.md) | D 第二轮工程实现核查 | D-R07–D-R09（2×P2+1×P3），真实 Go 后端 9 焦点核对 + 成员/邀请健壮性 |

### 文档分工（避免内容重复漂移）

| 文档 | 只回答 | 不负责 |
|---|---|---|
| PRD | 做什么、为什么、范围与验收 | 具体代码实现 |
| 落地方案 | 技术决策、架构、数据模型、API、版本规划 | 逐行代码教学 |
| 新手开发指南 | 怎么一步步写出来（可照敲骨架） | 重新讨论需求 |
| 枚举字典 | 枚举值的唯一权威 | 业务流程 |
| 原型 | 长什么样、怎么交互 | 数据持久化 |
| 原型还原与 PWA 总方案 | 如何分阶段推进、如何验收、AI 如何交接 | 重新定义需求或技术契约 |

> 规则：需求/范围以 **PRD** 为准；技术选型以**落地方案**为准；枚举值以**枚举字典**为准。三份文档冲突时，按此优先级，并在 CHANGELOG 记录。

## 快速开始（运行原型）

> **想直接体验，不用本地搭环境？** 打开公网在线版 👉 [https://a38ce1ad4e917321e.app.workbuddy.host/?sharecode=BPE6OFTPKE](https://a38ce1ad4e917321e.app.workbuddy.host/?sharecode=BPE6OFTPKE)（由 WorkBuddy 托管，含全部交互页面，演示账号已预填）。

原型是纯前端应用，无需后端：

```bash
cd prototype
python3 -m http.server 8765
# 浏览器打开 http://localhost:8765/
# 演示账号已预填（me / 123456），直接点登录
```

> 直接用 file:// 打开可能因浏览器安全策略导致模块加载受限，建议用上面的本地服务器方式。

## 技术栈（现状）

- **后端（已实现）**：Go 1.22 + Gin + xorm + SQLite（`mattn/go-sqlite3`，**需 CGO**）。
  - 数据库策略：WAL + `synchronous=FULL` + `foreign_keys=on` + `MaxOpenConns=1`（单写者）。
  - 鉴权：JWT（HS256，24h，bcrypt 存口令）；登录失败限流 + 失败审计。
  - 实时：WebSocket（REST 与 WS 共用端口）。
- **客户端**：Flutter（Windows / Android / iOS），按 R0–R7 阶段推进；Windows 桌面 debug 构建已打通（见 `CHANGELOG.md` [1.57]）。
- **Web 端（已由后端单二进制托管）**：`server/web/embed.go` 用 `//go:embed dist` 把 `server/web/dist/`（`index.html` / `app.js` / `style.css`，手写原生 HTML/JS/CSS、无构建工具链）编入 Go 二进制，与 API / WebSocket **同端口同源**提供；启动后端即同时提供前端，**不存在可独立启动的生产前端进程**。原型还原与 PWA 演进以 [R0 报告](docs/FastLedger-R0-基准冻结与差异盘点.md) 与 [总方案](docs/FastLedger-原型还原与PWA分阶段实施总方案.md) 为准（R0 未通过，PWA 未实施）。
- **部署（当前实际形态）**：**裸机/云主机 + systemd + Caddy 反代**（应用自身只监听 HTTP `:8080`，无内置 TLS）。
  > 设计文档中的 Docker / docker-compose 方案**尚未落地**（仓库无 Dockerfile），部署请以部署交接文档为准。


## 版本路线（摘要）

- **v0.1（MVP）**：手动 + 截图记账、账户/分类、基础报表、导出、多账本、2 人家庭、投资骨架。
- **v0.2**：5+ 成员、完整预算、Web 端完善。
- **v0.3**：投资自动行情（A股/港股/基金/黄金）。
- **v0.4**：完整资产负债与净资产、多币种汇总。

详见 PRD 与落地方案。

---

*本仓库同时承载**设计文档**（`docs/`、`workbuddy/`、`gpt/`）、**可交互原型**（`prototype/`）与**已实现的 Go 后端**（`server/`，阶段 0–4）。后端当前状态与验收口径见上方「当前状态」，部署相关一律以 `docs/FastLedger-部署交接-腾讯云轻量应用服务器.md` 为准。*

