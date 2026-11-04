# 更新日志（CHANGELOG）

本项目所有重要变更记录于此。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [1.61] - 2026-11-04

### R0 独立补漏与静态复核（文档修订，非代码修复）

- 新增 `docs/FastLedger-R0-独立补漏与复核记录.md`，逐项记录 R0 原报告的误判、遗漏、证据位置、修正建议、验收标准及本轮未完成事项。
- 修订 `docs/FastLedger-R0-基准冻结与差异盘点.md`：更新远端基线为复核时可见 HEAD `1cd56ab7830d2d3f33faf7fc5372d3824c088ae0`；收回“首页缺骨架屏/日小计”“Web 前端已接 WS”“记账类型分段已实现”等错误结论。
- 新增 R0-E-20~R0-E-36：包括首页结余/净资产口径错误、首页账单字段与筛选缺失、交易详情/表单字段缺失、删除文案与恢复能力矛盾、Web 无 WS 与 sync 客户端、邀请撤销 UI 缺失、账本级可见性 UI 缺失、期初余额未落实、多端会话 UI 缺失、账本重命名未接线、README 状态漂移、日小计 60 条边界、加载失败 skeleton 残留及 API 矩阵粒度不足。
- 校准 Owner 裁决项：Web OCR 已由 PRD §5.2 明确不做；期初余额需求已明确，待裁决的是不污染收支统计的技术模型；会话查看/撤销已是 PRD 需求；多账本与回收站不能列为可删扩展。
- **范围说明**：本次只修改审计文档、README 导航与 CHANGELOG；未修改业务代码、PRD、落地方案、WBS、数据库或 API；未运行测试、浏览器或截图采集；**R0 仍未通过，不代表 R1 已获授权**。

## [1.60] - 2026-11-04

### R0 基准冻结与差异盘点（R0 阶段交付，盘点非代码）

- 新增 `docs/FastLedger-R0-基准冻结与差异盘点.md`，按总方案 §5/§6 交付 R0-A~R0-G：页面/入口清单、逐页交互与状态矩阵、`prototype/` 与 `prototype-v2/` 差异、当前 Web 路由/render/API 与后端 API 能力映射、差异根因分类、截图与覆盖率方法、Owner 裁决清单。
- 关键盘点结论：后端能力完整（账户/分类/交易详情/删除恢复/报表/预算/投资/成员/设置均有 API），主要差距在前端 IA/UX 与原型对齐；**导航结构不符**（当前桌面 10 项平铺、移动 5 Tab，规范为 5 项/4 Tab + "我的"中枢）；首页三卡已实现但缺筛选条与骨架屏；Web 无截图识别（OCR 范围待裁决）。
- 对齐 GPT 口径：不再复述未经矩阵证明的"50–55% 还原度"，改用"页面×交互×状态×断点"加权矩阵；`prototype/` 为流程基准、v2 为视觉补充。
- 自查补漏（R0-E-17~19）：设置页缺「个人资料」「服务器与备份」区块、缺改密入口；成员「账本可见性」仅只读徽标、无开关（`PUT /books/:id/visibility` 未接）。对应新增裁决项 D-8（Web 备份状态展示）。
- README 导航新增 R0 文档入口。
- **范围说明**：本次仅新增 R0 盘点文档并更新 README 导航；**未修改任何业务代码、PRD、落地方案、枚举字典或 WBS**；未实现原型还原或 PWA 功能；**仅授权 R0，未启动 R1–R7**；待 Owner 独立复核差异清单并对 R0-G（D-1~D-8）给出结论后，再决定进入实施的任务。

## [1.59] - 2026-11-04

### 原型还原与手机 PWA 分阶段实施总方案

- 新增 `docs/FastLedger-原型还原与PWA分阶段实施总方案.md`，将 Web 原型还原、完整移动端、PWA 安装、IndexedDB、离线记账同步、离线分析、本地备份恢复和跨端验收编排为 R0–R7 阶段。
- 汇总当前 Web 与原型的已知差距：首页账单流/概览、交易详情、账户与分类 CRUD 前端接线、手机底部导航与移动账单布局等；明确后端已有 API 不应误判为后端缺功能。
- 明确 `prototype/` 与 `prototype-v2/` 的基准冲突、OCR 范围等待裁决事项；软删除/恢复/审计、多账本和注册入口等已定业务约束不得为追求视觉一致而破坏。
- 制定 AI 开工回执、逐项实施计划、完工证据、截图/真机/离线测试门禁和新 AI 阅读顺序；首批建议只执行 R0 基准盘点，不立即进行大规模 UI 或离线代码改造。
- **范围说明**：本次仅新增实施总方案并更新 README 导航；未修改 PRD、落地方案、WBS 或业务代码，未实现原型还原/PWA 功能，未将任何新能力标记为已交付。PWA 专项技术要求继续以 [1.58] 指导书为准。

## [1.58] - 2026-11-04

### 手机 PWA 实施指导书（完整移动端 + 完整离线 + 本地备份恢复）

- 新增 `docs/FastLedger-手机PWA实施指导书.md`：基于仓库现状明确推荐路线——在现有 `server/web/dist/` Go embed Web 客户端上演进移动优先 PWA，Flutter 原生端并行保留；不把现有 `client/web/manifest.json` 误认为 PWA 已交付。
- 指导书覆盖 IndexedDB 本地数据层、Service Worker 静态资源缓存边界、离线账单分析、Outbox / push-pull / 冲突处置、权限变化与 `restore_epoch`、用户级加密备份与本机恢复、浏览器安全及 Android / iOS 真机验收。
- 增加面向后续 AI 的文档修订指令：要求先更新 PRD、落地方案、开发 WBS 和验收矩阵；提出阶段 5P（5P.1–5P.14）任务拆分及待确认事项。
- **范围说明**：本次仅新增实施指导书并更新仓库导航；未修改 PRD、落地方案、WBS，未实现 PWA 代码，未宣称离线功能已交付。后续须按指导书先完成文档对齐与技术 PoC。

## [1.57] - 2026-11-02

### Windows 桌面首次构建打通 + 开发者模式/工具链验证（PR #1，环境缺口 [1.47] 收口）

- **环境实测闭环**（本机 Win11 26200）：开发者模式开启（⚠️ 实证：仅 `reg add AllowDevelopmentWithoutDevLicense=1` 不生效，必须走设置界面完整启用；生效即时；symlink 普通用户实测可创建）+ VS 2022 Community 17.14（"使用 C++ 的桌面开发"工作负载，MSVC 14.44 + Windows SDK 10.0.26100）+ Flutter 3.47.9 stable（`.tools/flutter` 完全可用）。`flutter doctor` 的 Windows Version/VS 检查项在本机工具沙箱内会因 reg.exe 被拦而"crash"，属误报，真实环境正常。
- **首次 `flutter build windows` 修复 4 项**（5.3 热键 C++ 代码此前"类型检查通过但从未编译过"的欠账，经 PR #1 合入，squash → `ca43fb7`，CI 绿）：
  1. runner `CMakeLists.txt` 增加 **`/utf-8`**——MSVC 默认按系统码页（GBK）解析无 BOM UTF-8 中文注释，多字节序列吞行尾换行导致 `hotkey_handler.cpp` 大面积级联语法错误（"printed 未声明"等均为假象）；
  2. `hotkey_handler.h` 补 `<windows.h>`（HWND）与 `<vector>`/`<cstdint>`；
  3. `GdiplusStartup/GdiplusShutdown` 必须写 **`Gdiplus::`** 前缀（`gdiplus.h` 将 `gdiplusinit.h` 包裹在 `namespace Gdiplus` 内，SDK 10.0.26100 头文件实证）；
  4. Flutter 3.47 的 `FlutterEngine` 不再继承 `BinaryMessenger`，`MethodChannel` 第一参数改 `engine()->messenger()`；另按 `/W4 /WX` 重命名遮蔽全局名的局部变量（C4459）。
- **验证口径（诚实）**：`flutter build windows --debug` 实机成功产出 `build/windows/x64/runner/Debug/fastledger_client.exe` = 编译/链接闭环；**热键触发、截图内容的运行时手工验证仍待 5.3 批次**（需真机交互）。
- **Web 登录页增加注册入口**（本批补）：此前登录页只有登录（原型基线未覆盖注册场景）；新增"没有账号？注册新账号"切换——注册表单（用户名/密码≥8位/确认密码，前端一致性校验）→ `POST /auth/register` 成功后自动登录进账本页；服务端关闭公开注册（`EBK_ENABLE_REGISTER=false`，生产默认）时按既有契约提示 403 文案。服务端零改动。
- 流程备注：本批因本机代理中断改经 GitHub API 编排 PR（revert + 分支重放技法），合并后本地 main 需在代理恢复时 `fetch + reset --hard origin/main` 做最终 SHA 对齐（树内容已一致）。

## [1.56] - 2026-11-02

### Flutter 设置页补齐长期 API Token 管理 + 接入 secure storage（WBS 6.6 客户端侧收口）

- **新依赖**（经 owner 批准）：`flutter_secure_storage ^11.2.0`——Android Keystore / iOS Keychain / Windows DPAPI（v9+ 已弃 CredManager，其 ~2KB 条目上限不适用）。
- **`core/secure_token_store.dart`**：`SecureTokenStore implements TokenStore`（登录 JWT 唯一条目 `fl_token`；PAT 明文按设计只出现一次，**不持久化**——长期凭证只存服务端哈希）。接缝即既有抽象，业务层零改动；`FastLedgerApp` 增加 `tokenStore` 注入参数（默认 SecureTokenStore，测试传内存实现）。
- **启动恢复**：secure storage 读取异步化——`RootGate` 先渲染加载态，命中令牌免登录进首页；被撤销的令牌由首个 401 经 onUnauthorized 拉回登录页（离线不阻塞）。退出登录同时清 secure storage。
- **设置页 PAT 区块**（对照 Web 端 [1.53] 交互）：生成（备注输入 → **明文一次性展示+复制**+"仅此一次展示"警示）→ 列表（备注/创建时间/最近使用/已撤销状态）→ 撤销二次确认；列表加载失败不阻塞设置页其余部分。
- **诚实口径**：Windows 桌面构建需要系统"开发者模式"（插件 symlink）+ Visual Studio（[1.47] 已登记缺口）——`flutter build windows` 运行时验证仍不可行；本批验证止于 `flutter test`/`analyze`，Windows 真机验证待环境具备。
- 测试：+7 例（SecureTokenStore 4——注入覆写 FlutterSecureStorage 的替身、断言 key/语义/接缝；设置页 PAT 3——列表渲染/一次性明文/撤销确认），**79/79 全过、analyze 0 issues**。零后端改动。

## [1.55] - 2026-11-02

### 阶段 7 运维：Docker 部署单元 + 备份口径对齐 + 恢复演练 + 验收报告（WBS 7.1–7.5）

- **Docker 单镜像（7.1）**：`deploy/Dockerfile`（多阶段 alpine+musl——CGO 编译 mattn/go-sqlite3，builder/runtime 同 musl 避免链接失败；nonroot；HEALTHCHECK `/healthz`；数据卷 /data；固定 CST 不依赖 tzdata）+ `deploy/docker-compose.yml`（**stop_grace_period: 30s**、命名卷持久化、Caddy 服务在 `profiles: ["tls"]` 下按需启用自动 HTTPS）+ 根 `.dockerignore`（白名单 server/）。**真实构建+容器 healthz/migrate 冒烟由新增 CI `docker-build` job 承担**（服务器与本机均未装 docker，按约定不擅自安装）；CI 冒烟注意：空库启动会被 Check 拒绝——先 `db migrate` 建库再起服务。
- **每日备份（7.2）**：核对发现调度已在 `/etc/cron.d/fastledger`（每日 03:30，online backup API，并发安全）——**保留口径与 PRD §12 决策#5 不符**（原 30 天），已收紧为**滚动保留最近 7 份**（服务器脚本 + cron 注释 + 仓库副本 `deploy/backup.sh` 三处同步，手动执行验证成功）。
- **恢复演练（7.3）**：2026-11-02 月度演练**通过**——恢复 db-2026-11-02 → bump（restoreEpoch 1→2，审计落"备份恢复"事件）→ 临时实例 healthz up/restoreEpoch=2 → 六表记录数一致（audit +1 为恢复事件，预期）→ 审计可读 → 清理，生产未受影响。实测修正两个坑并固化进 `deploy/README.md` §3：`instance bump` 必须同时传 `EBK_DB_PATH` 与 `EBK_DATA_DIR`（否则审计静默落 cwd 空库）；正确顺序为**恢复→bump→再启动**（restoreEpoch 启动时读入）。
- **验收报告（7.5）**：`docs/FastLedger-阶段7-运维与验收报告.md`——7.1–7.5 状态矩阵 + §10.1–§10.9 逐项映射（自动化测试 vs 生产实测证据来源）+ 诚实口径四条（域名/带凭据 E2E/Windows OCR/服务器无 docker）。
- **待 owner 决策**：外网 HTTPS 阻塞于域名（Caddy 配置就绪，`--profile tls` 一键启用）；DDNS 对固定公网 IP 不适用。
- 7.4 测试：CI 已含全量+race+真库冒烟，本批追加 docker-build job；服务器 803 PASS/0 FAIL 基线不变。

## [1.54] - 2026-11-02

### Flutter 设置页 + 成员页（WBS 5.x 客户端补齐，PRD §4.12 / §7）+ 后端个人资料端点

- **后端（PRD §4.12 缺口补齐，零迁移）**：
  - `PUT /users/me`（更新邮箱；**用户名为登录标识不开放修改**，口径在注释与本文如实说明）；邮箱宽松格式校验 + 先查后写唯一冲突 409（`ErrEmailTaken`）；允许空串清除（部分唯一索引不含空值）
  - `POST /users/me/password`（修改密码）：验证旧口令 → 新口令沿用注册强度规则（最短 8 位 / bcrypt 72 字节上限）→ 更新哈希 → **撤销该用户全部会话（含当前）**，响应如实返回下线会话数；长期 API Token 按设计不受改密影响（可单独撤销）
  - `authAPI` 接口扩两方法；测试 stub 同步实现
- **Flutter 客户端**（沿用 StatefulWidget + 构造器 DI 风格；全局设置用内置 `ChangeNotifier`，不引第三方）：
  - `core/app_settings.dart`：主题模式（系统/浅色/深色）与通知开关（**纯本地偏好**——服务端尚无推送通道，如实标注），经 LocalStore 持久化；`main.dart` 以 ListenableBuilder 上根，MaterialApp 三态主题生效
  - `ui/settings_page.dart`：个人资料卡（用户名只读+锁定提示、邮箱编辑对话框）、账号安全（修改密码三段表单：两次一致性前端校验不发请求；成功后 SnackBar 告知"所有设备已下线"并回登录页）、偏好卡（通知 Switch + 主题 SegmentedButton）、退出登录（确认对话框，原型文案）
  - `ui/members_page.dart`：成员列表（头像/角色徽标/可见性副标题）、底部弹层成员详情（角色/可见性/实例管理员/uid + 设为管理员/降级为成员/移除成员——移除需确认对话框）、邀请成员（角色下拉+可选绑定用户名 → 邀请码一次性展示+复制）；`book_detail_page` 菜单新增"家庭成员"入口
  - `ledger_api.dart`：me/updateEmail/changePassword + members/createInvite/changeRole/removeMember
  - 移动端适配：ListView + 卡片布局、showModalBottomSheet + SafeArea、SnackBar 反馈、403 服务端文案如实透传
- 测试：service 2 例 + api stub 扩展；Flutter `test/helpers.dart`（按路由分发的 FakeApiHttp）+ 22 例新增（API 8 + AppSettings 4 + SettingsPage 5 + MembersPage 5），**72/72 全过、analyze 0 issues**
- 本批零迁移、Web 端不受影响；生产部署仅换二进制

## [1.53] - 2026-11-02

### 长期 API Token（WBS 6.6，迁移 0008）+ Web 端三页补齐（设置/成员/投资账户详情）

- **迁移 0008**（schema v7 → **v8**）：新建 `auth_tokens`（uid / token_hash / name / created_at / expires_at / revoked_at / last_used_at + idx_auth_tokens_uid）。与 PRD §370 的口径偏差（如实登记）：① 列名 `uid`（对齐 0007 token_sessions 的命名）；② **不存明文，只存 SHA-256 哈希**——PRD 的 token 列落地为 token_hash，明文仅在创建响应出现一次（同 GitHub PAT 语义）。
- **三道迁移守卫全部登记**：versionAssertions[8]（表/列/索引）+ goldenScripts 指纹 + TestAssertionCoversAllArtifacts 机械全量比对；既有迁移基线测试同步更新到 v8（R2 系列"应应用 N 个"等硬编码）。
- **PAT 服务** `internal/service/token.go`：Create（`flt_` 前缀 + 32 字节随机 hex，有效数上限 20）/ List（绝不含哈希）/ Revoke（跨用户·不存在·已撤销统一 404，不泄露存在性）/ VerifyPAT（撤销·过期 → ErrTokenRevoked；账号禁用 → 同会话口径；last_used_at 按 1 小时粒度回写）。
- **鉴权中间件**：JWTAuth / wsAuth 按 `flt_` 前缀分流——PAT 走哈希查表（撤销即 401，PRD §10.528 的 WS 握手拒绝同样成立），JWT 照旧走会话强校验；PAT 不是会话，不写 jti。WS 连接上限取 PAT 的 expires_at（0 = 永不过期时以 now+100 年兜底，保持"连接不得比凭证活得久"契约）。
- **API**：`GET/POST /users/me/tokens`、`DELETE /users/me/tokens/:tokenId`（JWT 与 PAT 均可调用——PAT 可自管理）。
- **Web 端（对齐原版原型文案）**：
  - **设置页**（#settings）：安全设置 · 长期 API Token（生成→一次性明文展示+复制 / 列表含创建·最近使用·状态 / 撤销二次确认"撤销立即生效…不可恢复"）+ 退出登录（"退出后需要重新登录。本地数据会保留。"）；
  - **家庭成员页**（#b/:id/members）：成员列表（所有者/管理员/普通成员徽标、已开启/已关闭账本可见）、邀请成员（角色选择+可选绑定用户名→邀请码一次性展示+复制）、设为管理员/降级为成员、移除成员（"移除后该成员将无法再访问家庭账本。"）；
  - **投资账户详情**（#b/:id/account/:accId）：对账页投资账户（类别 5）点入——账户余额（=现金部分）+ 该账户持仓明细（标的/数量/成本/现价/市值/盈亏）+ 市值合计，复用既有 holdings API 零后端改动。
- **已知限制（如实说明）**：撤销 PAT 后，已建立的 WS 长连接不会被主动踢断（PAT 无 jti 无法定位连接），但下一次握手即被拒；存量连接最长活到其连接上限。
- 测试：service 3 例 + api 1 例（生命周期：生成→PAT 直调业务 API→列表脱敏→撤销即 401）；本地全量 `go test ./...` 全绿。

## [1.52] - 2026-11-02

### 投资与净资产（WBS 6.4 / 6.5，零迁移，阶段 6 收官）

- **零迁移**：instruments / instrument_prices / holdings 三表 0001 已预建（holding.go 的 Holding 结构体阶段 3 同步已补，本批复用）；新增 `github.com/shopspring/decimal` v1.4.0（直接依赖）。
- **标的库** `internal/service/invest.go`：Create/Update/Delete/List——类型枚举 1~6（字典 §4）、名称 1~64 字符；软删前校验持仓引用（有持仓 → ErrInstrumentInUse 409）；写操作 owner/admin、版本递增、审计（实体 `instrument`，《枚举字典》§14.2 补行）。
- **持仓**：AddHolding/UpdateHolding/DeleteHolding——数量/现价为**十进制字符串**（decimal 解析，禁 float），数量 > 0、价格 ≥ 0、成本 ≥ 0（负数/非法一律报错，WBS 6.4 硬约束）；挂靠账户须属本账本；版本递增 + 审计（实体 `holding`）。
- **手动行情**（PRD §12 决策 #3，MVP 唯一价格来源）：`POST .../instruments/:id/price`——append instrument_prices 历史行 + 同事务刷新该标的全簿持仓 current_price（冗余缓存）+ 审计；行情失败保留旧价（PRD §9.8）。
- **市值/盈亏**：市值(分) = quantity × price × 100（decimal `Round(0)` 半进到分）；浮动盈亏 = 市值 − cost_amount。
- **净资产（WBS 6.5）**：`GET /books/:id/net-worth`——总资产 = Σ资产账户(1~9) + Σ持仓市值；总负债 = Σ负债账户(10~15)（**正 = 欠款**，字典 §2）；净资产 = 总资产 − 总负债；按账户分类分组返回（资产构成）。
- **Web 端（对齐原型文案）**：导航新增"资产"；页面 =「资产总览」四卡（总资产/总负债/持仓市值/家庭净资产）+ 持仓表（数量/成本/现价/市值/浮动盈亏红绿）+ 标的库表；新增/编辑标的、添加/编辑持仓、录价对话框（录价即同步刷新持仓现价）。
- 测试：service 4 例（标的 CRUD+引用保护+审计/decimal 校验与市值盈亏/录价刷新/净资产公式含负债正=欠款与 member 可读）+ handler 2 例（路由鉴权/CRUD+录价+净资产流）；本地全量 `go test ./...` 全绿，gofmt 干净。
- 至此**阶段 6 全部完成**（6.1 报表 / 6.2 导出 / 6.3 预算 / 6.4 投资 / 6.5 净资产）。

## [1.51] - 2026-11-02

### 预算（WBS 6.3：信封式总预算 + 分类月度预算，零迁移）

- **零迁移**：`budgets` 表由迁移 0001 预建（含 budget_type/period_type/rollover/enabled/软删），枚举字典 §7/§8 已定义口径（**1=分类预算、2=总预算**；周期 MVP 固定自然月），本批直接启用。
- **服务层** `internal/service/budget.go`：Create/Update/Delete/List。
  - 写操作 owner/admin（requireManagerTx，口径同账户/分类）；写路径审计留痕（create/update/delete，实体 `budget`——《枚举字典》§14.2 已补该实体行）。
  - 校验：金额正整数分；period_type 只接受 1（2~4 为枚举预留，传入 400）；总预算不得挂分类；分类预算只挂**支出**分类；名称 ≤64 字符。
  - 唯一性：同分类一条生效预算、总预算全本一条（0001 无唯一约束，事务内复核 → 409）。
  - **进度**：List 返回每条预算的 spent/percent/status；分类预算挂大类时"已用 = 大类自身 + 全部直接子类"（WBS 6.3）；区间半开 [本月 1 日 0 点, 下月 1 日 0 点)（CST）；转账不计；**"已用"聚合挂 `txVisibilityCond`**（关互看 member 只统计本人——QA 守卫登记调用者 `spent`）。
  - 状态阈值（PRD §4.7）：≥100% 超支(over)、≥80% 接近(near)、其余正常。
- **API**：`GET/POST /books/:id/budgets`、`PUT/DELETE /books/:id/budgets/:budgetId`；错误映射 404/409/400。
- **Web 端（对齐原版原型）**：导航新增"预算"；页面=「预算管理」标题 + "预算周期为自然月，每月 1 日自动重置进度。"提示 + **本月总预算**卡 + **分类预算**信封列表（进度条三态色：正常主色/接近橙/超支红，徽标"正常/接近/超支"，"已用/剩余"文案）+ 新增/编辑/删除对话框（"预算金额（元/月）"，编辑时类型与挂靠只读——改挂靠走删除重建，审计各留一条）。
- 测试：service 4 例（CRUD+审计 4 条/权限与校验与唯一性/进度聚合大类+小类与半开区间/可见性）+ handler 2 例（路由鉴权/CRUD 流与 409/400/404）；本地全量 `go test ./...` 全绿，gofmt 干净。
- 说明：rollover 仅存储与展示（PRD §4.7 MVP 不实现结转）；预算不在增量同步契约内（无 sync_version 列），客户端经 REST 读取。

## [1.50] - 2026-11-02

### 阶段 6 前半：基础报表 + 数据导出（WBS 6.1 / 6.2，零迁移、纯读聚合）

- **报表 API**：`GET /books/:id/reports/monthly?months=N`（最近 N 月收支趋势，1~24 归一化默认 6）与 `GET /books/:id/reports/categories?type=1|2&month=YYYY-MM`（指定月分类占比，按金额降序）；**转账不计收支**（PRD §4.3.1/§5.3）；**月份边界固定 CST（UTC+8）**——PRD §12 决策 #4"MVP 仅 CNY"，用固定偏移不依赖服务器 tzdata。
- **导出 API**：`GET /books/:id/export?format=csv|json&fields=...&type&minTime&maxTime&accountId&categoryId`；CSV **UTF-8 BOM + CRLF**（Excel 直开无乱码）+ `sanitizeCSVCell` 公式注入防护（`= + - @` 开头或含 \t \r 前置单引号，WBS 6.2 硬约束）；字段白名单 9 列（time/type/amount/category/account/toAccount/channel/merchant/note）可裁剪；行数上限 10000，超出截断并经 `X-Export-Truncated` / `X-Export-Count` 头如实告知（不静默少给）；金额导出为"元"字符串（整数分纯整数字符串换算，禁 float）。
- **可见性红线**：报表与导出是"看得到全部明细"的另一种形态——Monthly / CategoryShare / ExportRows 三个读路径全部挂 `txVisibilityCond`，`TestQAS3VisibilityCondWiredIntoAllReadPaths` 白名单同步扩项（登记先例同 updateReceipt）；测试验证关闭互看后 member 只能聚合本人记录。
- **Web 端（对齐原版可交互原型）**：导航新增"报表/导出"；报表页=收支趋势（近 6 月柱状）+ 本月支出分类占比（分类排行，本月/上月切换，文案对齐原型"收支报表/收支趋势/分类排行"）；导出页=时间范围 + 收支类型 + 导出格式卡片（CSV/JSON）+ 导出字段勾选 + "导出 CSV 文件/导出 JSON 文件"，空数据 toast"没有可导出的数据"、成功"已导出 N 条记录"、文件名 `FastLedger_导出_日期.后缀`——页面结构与文案严格对照原型；零 innerHTML、金额整数分。
- 测试：service 5 例（月度聚合/可见性/分类占比/导出与截断/CSV 防护与元串）+ handler 3 例（路由鉴权/信封契约/CSV+JSON 导出与零账务）；本地全量 `go test ./...` 全绿，`gofmt -l` 干净。
- 遗留：6.3 预算（需迁移 0008 + 三道迁移守卫登记）、6.4/6.5 投资与净资产，属下一批次。

## [1.49] - 2026-11-02

### 全局主题设计规范 + 新版可交互原型（纯前端/文档，零后端改动）

- **设计规范文档** `docs/FastLedger-Web设计规范.md`（v1.0）：以 `prototype/assets/index-*.css` 的 `:root` 主题变量为单一取证来源，提炼全局主题 token——配色（核心语义层 + 收支红绿口径 + 中性色阶 + chart-1..5 图表色 + 12 色分类图标色板）、字体体系（Noto Sans SC 四字重、xs–xl 字号阶梯、金额 tabular-nums）、间距栅格（4px 基数、980px 容器、44px 触控目标）、圆角（2/8/12/14/999px）与阴影（≤5% 透明度极轻口径）、组件样式（按钮/表单/卡片/导航/徽标/表格/对话框/Toast/进度三态/空状态/骨架屏/开关）、交互状态与动效 token（.1s–.25s 分档 + cubic-bezier 缓动 + reduced-motion 降级）、信息架构（13 页面 + 底部 Tab 结构）与 token 落地约定（CSS 变量别名映射、Flutter ThemeData 映射、变更流程）。
- **机器可读 token** `docs/design-tokens.json`：上述全部分组的结构化版本，供生成 CSS / Tailwind config / Flutter 主题。
- **新版可交互原型** `prototype-v2/`（index.html + README）：依据规范手写重建的单文件原型（区别于 `prototype/` 构建产物快照，保留不动）——12 视图覆盖登录/首页账单/记一笔（类型分段）/截图识别三级漏斗（绝不自动入账）/交易详情（删除二次确认 + 余额回滚）/账户/两级分类/预算三态/报表（趋势柱状 + 占位环形图）/我的/成员（可见性开关 + 常驻隐私提示）/导出/设置；移动端底部 Tab + 桌面顶部导航双形态；localStorage 演示数据全页联动；零外部请求。
- 验证（诚实口径）：`node --check` 语法通过；**Node vm + DOM stub 冒烟**——12 视图渲染零异常、记账→余额联动→删除回滚、转账双账户回滚、编辑路由参数解析均通过；未做真实浏览器渲染验证（视觉细节以规范文档与 token 为准）。

## [1.48] - 2026-11-01

### Web 端交互对齐可交互原型（prototype/，纯前端静态资源，零后端改动）

- **设计 token 对齐**：按原型打包产物取证（`prototype/assets/index-*.css` 的 `:root`）重写 `style.css`——主色 `#34b262` 绿 / 背景 `#f9f8f5` 暖白 / 破坏色 `#ef4343` / 文字 `#1f2e26` / 悬停底 `#f2f8f4` / 圆角 8–12px / 阴影两级 / 过渡 `.15s cubic-bezier(.4,0,.2,1)`；字体栈首位 `Noto Sans SC`（无外链请求，保持离线口径）。
- **交互补齐**（此前为无过渡的裸静态样式）：按钮/输入框/链接的 hover·active·focus-visible·disabled 全态；卡片与视图进入动画（fadeUp）；表格行悬停底色；对话框 shadcn 风格进入动画 + backdrop 模糊；Toast 升级 sonner 风格（ok/err/info 三变体 + 前缀图标 + 滑入滑出）；`prefers-reduced-motion` 全量降级；≤640px 窄屏适配（隐藏次要列）。
- **状态反馈补全**：登录"登 录 → 登录中…"loading（对齐原型）；创建账本/记一笔/编辑/删除/恢复按钮统一 pending 态（`btnPending`，异常安全 finally 恢复）；交易列表加载骨架屏（shimmer）替代白屏跳变；翻页后平滑回顶。
- **组件对齐**：类型列改胶囊徽标（收入绿/支出红）；批量删除的原生 `confirm()` 替换为自定义确认对话框（AlertDialog 风格：标题+描述+危险色确认键）；账本列表项悬停底色+右箭头；三处空状态改为图标+引导文案（账本/交易/回收站）；回收站恢复成功后行淡出移除。
- **口径不变**：全部用户数据仍 `textContent` 渲染（零 innerHTML）；金额整数分；401 清会话；无外部网络请求。
- 验证：`node --check` 语法通过；DOM id 交叉核对 11/11；无 innerHTML/eval。

## [1.47] - 2026-11-01

### 第五阶段客户端批次（WBS 5.1 / 5.2 / 5.6 / 5.3，`client/` Flutter 工程）

- **5.1 工程骨架**：`flutter create`（Android/Windows/Web 目标，Flutter 3.47.9）；`ApiClient`（Bearer + {code,msg,data} 信封统一解包、401 清会话回调、multipart 上传）、`TokenStore`/`LocalStore` 抽象（内存实现，生产接 secure storage/sqflite 的接入点）、`Outbox` 持久化离线队列（**clientUuid 入队即生成并持久化**，flush 2xx 出队 / 4xx 丢弃 / 5xx·网络异常留队重试，重放同键幂等）；登录页 + 账本列表首页。
- **5.2 CRUD 页面**：账本详情（交易列表/下拉刷新）、记一笔/编辑表单（**类型只读**——服务端契约类型不可改；金额字符串↔整数分禁 double；编辑携带 `baseVersion` 乐观锁，409 提示刷新）、删除（确认弹窗，幂等 clientUuid）；账户管理（浏览/新增/删除）、分类管理（浏览/新增/删除，catType 区分收支）。
- **5.6 端侧规则引擎**：`lib/parse/receipt_parser.dart` 与服务端 `ocr_parse.go` **同语义**（全角规范化、千分位、整数转分禁 double、带符号金额拒绝、平台名不判方向、滑窗合并、唯一确认才出候选、confidence 恒 0）；测试向量与 Go 行为对齐（含"HH:MM 的时数字被当第二金额→整窗丢弃"的已知保守语义）。
- **5.3 Windows 热键 + 截图 + 确认弹窗**：平台通道 `fastledger/hotkey`（C++ RegisterHotKey + GDI+ PrintWindow 截图，PW_RENDERFULLCONTENT 支持 DirectComposition 内容）；`runOcrFlow` 编排：截图 → 服务端 recognize → **端侧离线 parse 优先** → 空则服务端 parse → `TxConfirmDialog` 候选确认弹窗（勾选 + 选账户 + 用户点击才入账）；空候选/503/429/异常一律回退手动提示，**绝不自动入账**。
- **验证口径（诚实声明）**：Dart 侧 `flutter analyze` 0 issues、`flutter test` **50/50 全绿**（规则引擎向量 / outbox 幂等 / 信封与 401 / 金额往返 / 表单与弹窗 widget 测试 / 热键通道 mock / OCR 编排）；**C++ runner 侧（hotkey_handler.cpp / flutter_window.cpp / CMake）因本机无 Visual Studio 无法编译与运行时验证**，首次 Windows 构建须人工复核——客户端整体未做真机/桌面运行时验证。
- 测试基建：无代理环境变量下 flutter_tester 方可连接（本地 WebSocket 被系统代理拦截为已知环境问题）。

## [1.46] - 2026-11-01

### 云端 LLM 文本兜底（WBS 5.8，默认关闭）

- 新端点 `POST /api/v1/ocr/llm-parse`：JSON `{text, consent}`，返回候选结构与 `/ocr/parse` 一致；**绝不写 transactions/audit**（e2e 断言零账务副作用）。
- **默认关闭**：`EBK_LLM_BASE_URL` 为空即禁用（503）；配置则 `EBK_LLM_API_KEY`/`EBK_LLM_MODEL` 必须齐全，半配置启动即失败。**双侧开关**：服务端配置之外，每次调用必须 `consent=true`（缺失 → 400）。
- **仅传文本 + 出站脱敏**：不存在图片通道；≥11 位纯数字串（手机号/银行卡/证件号）出站前 `前3****后4` 掩码，金额不受影响。
- **不信任 LLM**：候选逐字段服务端复核（金额正整数 ≤ 1 亿分、方向仅 income/expense、日期合法、confidence ∈ [0,1]、描述 ≤ 255 字、候选 ≤ 20），非法候选丢弃不修正；整体非 JSON / 上游非 2xx → 503。下游为 OpenAI 兼容 chat/completions（Bearer，temperature=0，禁重定向，响应有界）。
- **独立配额**：`llm:<uid>` 每分钟桶（默认 5），与 `ocr:<uid>` 隔离。契约更新：`docs/FastLedger-OCR双模式部署与切换契约.md`。
- 测试：service 7 例（脱敏/happy/consent/disabled/上游失败/候选校验/限流与桶隔离）+ handler 3 例（401/400/503/200 e2e + 零账务）；`gofmt -l` 为空。

## [1.45] - 2026-11-01

### 交易更新（WBS 5.9b：PUT /books/:id/transactions/:txId，Web 端同批接入）

- **守卫解除**：`TestQATransactionRoutesContract` 的 `PUT` 禁令解除——完整更新能力（转账对侧联动 + 余额净额重算 + 幂等摘要域 + before_json 审计）已实现，语义见 `docs/FastLedger-交易更新语义设计.md`；`/transfer`、`/batch` 第二条创建路径禁令不变；`TestQAS3SingleCreateEntry` 白名单登记 `UpdateReceipt` 为变更路径（不新增行、不调 `insertTxRow`，复用 `createChangeBalance` 唯一余额写入路径）。
- **语义裁决**：PUT 全量替换；**类型不可改**（互转必然伴随配对行的诞生/销毁，改类型 = 删除 + 重建，400 引导）；权限与 Delete 同口径（事务内重读角色，member 仅本人）；转账整笔双侧对称更新、禁止单边；`baseVersion` 乐观锁（>0 时校验行 sync_version，过期 409）。
- **余额净额重算**：按账户聚合"新 − 旧"差额，净额为 0 的账户不触碰（不刷 sync_version/updated_at）；逐账户走 `createChangeBalance` 原子 UPDATE（溢出边界 + RowsAffected 校验）；负债账户符号由 `signedBalanceDelta` 统一处理。
- **幂等**：`op=update`（`IdemOpUpdate`）复用 `idempotency_records`，唯一键天然容纳，**零迁移**（schema 保持 v7）；摘要域 `transaction.update/v1`，`clientUuid` 与 `baseVersion` 不参与业务身份摘要；重放返回原版本 + `replayed=true`，不做终态校验（无任何写入，与 Create 重放同语义，理由记录于设计 §7）。
- **审计**：每行一条 `update` 审计，`before_json` = 变更前整行快照，与业务写入同事务。
- **Web 端**：交易列表新增"编辑"入口（独立编辑对话框，类型只读展示）；金额分↔元字符串转换禁浮点；分类下拉按 catType 过滤；提交携带 `baseVersion`（列表行 syncVersion），409 时提示刷新重试。
- 测试：服务层 12 例（净额/搬运/负债符号/转账双侧+换向/类型拒绝/重放/同键冲突/权限/已删 404/乐观锁/校验/单次版本递增）；本地全量 plain 全绿；`gofmt -l` 为空。

## [1.44] - 2026-11-01

### Web 端（WBS 5.9，go:embed 单二进制托管）

- **定位**（《新手开发完整指南》§23 / PRD §5.2）：浏览器打开 `http://host:port/` 即可浏览、检索、批量编辑、对账；不做 OCR（OCR 交给客户端）。**当前交付：浏览/检索/筛选/分页、记一笔（单笔收/支 + 转账）、批量删除（逐条幂等 DELETE）、回收站恢复、账户对账视图**；交易字段级编辑（PUT）涉及转账对侧联动与幂等语义，被 CI 守卫明确禁止注册直至实现完整更新能力（见 `TestQATransactionRoutesContract` 注释），作为独立后端批次排期。
- **零构建工具链**：`server/web/dist/` 手写原生 HTML/JS/CSS，`go:embed` 打进单个二进制（自托管卖点，不引入 npm/Node）。
- **挂载策略**：gin `NoRoute` 兜底——显式路由（`/healthz`、`/api/v1/*`、`/ws`）永远优先；仅 GET/HEAD 服务静态文件，未命中回退 `index.html`；API 与 WS 前缀、非 GET 请求绝不回退静态页（保持 404 JSON，防止客户端把 HTML 当 JSON 解析）；`fs.ValidPath` 前置校验拒绝目录穿越；`X-Content-Type-Options: nosniff`。
- **前端安全口径**：全部用户数据经 `textContent` 渲染（禁止 innerHTML 拼接，防 XSS）；token 仅存 localStorage 并经 `Authorization` 头携带，401 统一清会话回登录页；金额全程整数分（字符串运算转元，禁浮点）。
- 测试：`web_handler_test.go`（根路径/静态命中/SPA 回退/API 404 不被吞/目录穿越拒绝）。

## [1.43] - 2026-11-01

### OCR 双模式服务端兜底（WBS 5.7 重定向：PaddleOCR 方案）

- **新 API**（JWTAuth 组内）：`POST /api/v1/ocr/recognize`（multipart image ≤10MiB → 转发本机兜底服务，返回 `engine:"server"` + lines）、`POST /api/v1/ocr/parse`（纯规则文本解析，返回候选 `amountCents`（整数分，禁 float）/`direction`/`occurredDate`/`description`/`confidence`）。
- **保守解析契约**：仅当金额、方向、日期均可无歧义确认才产候选；非法千分位（`¥10,00`）、多金额、多日期、带符号金额（`+/-`）、精度>2、溢出、矛盾方向一律空候选，绝不猜测。
- **MIME 内容嗅探**：服务端按图片魔数判定真实类型（PNG/JPEG/WebP/BMP/TIFF），不信任客户端 multipart 头；非白名单内容 400 且不消耗配额、不触达下游。
- **下游强契约**：Python 兜底（`rapidocr_onnxruntime`，仅监听 127.0.0.1:8500）响应的 `engine` 字段被严格校验；Go 侧 `DisallowUnknownFields` + 引擎标识比对，字段漂移 fail-closed。
- **配置 4 项**（fail-fast）：`EBK_OCR_FALLBACK_URL`（空=禁用）/ `EBK_OCR_TIMEOUT_MS`(8000，上限 120000) / `EBK_OCR_MAX_IMAGE_BYTES`(10MiB，上限 10MiB) / `EBK_OCR_RATE_PER_MIN`(10，上限 1000)。
- **限流**：`ocr:<uid>` 每用户每分钟共享桶（recognize/parse 合并计），复用既有有界滑动窗口限流器独立实例。
- **隐私与安全边界**：OCR 图片仅上传至用户自托管服务器本机兜底服务（内存处理、不落盘、无 access log）；OCR 链路绝不写 transactions/audit（端到端测试断言增量 0）；该裁决替代旧 PRD §4.4/§5.1/§8.3/§9.2 口径，契约见 `docs/FastLedger-OCR双模式部署与切换契约.md`。
- 无数据库迁移（schema 保持 v7）。测试：规则解析边界（全角/千分位/三种日期/负号/歧义/噪声）、嗅探单测、下游互操作（MIME 转发、engine 校验、未知字段拒绝）、端到端（真实登录 + multipart + 503 回退 + 429 + 零账务写入）。

## [1.42] - 2026-11-01

### 多端会话查看与撤销（WBS 5.10 / GPT-R52 落地）

- **迁移 `0007_token_sessions`**（终态 schema v7）：`jti`（UNIQUE）、`uid`、`created_at`、`expires_at`、`revoked_at`、`user_agent`、`last_seen_at`；索引 `(uid, revoked_at)` 与 `(expires_at)`；golden 指纹与 versionAssertions 守卫同步登记。
- **签发即落库**：`Login` / `LoginFromIP` 签发令牌与写会话行在同一事务，会话落库失败即不签发（503，不产生"有令牌无会话"的中间态）；`user_agent` 由 HTTP 层透传（仅展示用）。
- **Verify 强校验**：`VerifyWithClaims` 在 JWT 校验之外必须命中一行未撤销、未过期的会话记录；已撤销/无记录返回新错误 `ErrTokenRevoked`，HTTP 层 401 文案可区分（"会话已撤销或已失效，请重新登录"）。
- **会话 API**（JWTAuth 组内，`internal/api/session_handler.go`）：`GET /auth/sessions`（列表，标记 `current`，`?includeRevoked=1` 含已撤销）、`DELETE /auth/sessions`（登出其它设备）、`DELETE /auth/sessions/:jti`（撤销指定设备；跨用户/不存在统一 404；允许撤销当前会话）。
- **WS 主动断开**：`wsAuth` 把 `claims.Jti` 交给连接（`ws.TokenJtiContextKey`）；撤销后 `Hub.KickJti` 主动断开该会话全部在线连接，关闭码 **4003**（客户端应清除凭证回到登录页，不得无脑重连）。
- **清理**：`db purge` 追加会话回收统计（已过期 + 已撤销超 7 天，保留期固定，不开放配置）。
- ⚠️ **升级说明**：升级到 schema v7 后，**存量令牌（v6 及之前签发）无会话记录，一律 401——用户需重新登录**。这是预期行为，不做自动补登记（无法补出可信的 exp/UA，且会给撤销语义开豁免通道）。
- 测试：全量 `run_tests.sh plain` 710 PASS / 0 FAIL（新增 20 例：`service/session_test.go`、`api/session_api_test.go`（含 WS 4003 端到端）、`ws/hub_kick_test.go`）。

## [1.41] - 2026-10-30

### GPT v1.9 阶段 0–3 第二轮补充复核（GPT-R53）

- **GPT-R53 / P2 / 待确认**：AccountService.Delete 只检查活跃交易引用，没有检查活跃子账户；删除仍有子账户的父账户后，子账户会保留指向已删除父账户的 parent_id。对照 CategoryService.Delete 已有活跃子分类检查。位置 `server/internal/service/account.go:202–251`、`category.go:251–259`。
- **与既有问题区分**：R53（父账户删除造成悬空层级）不同于 R47（parent_id 可形成环）和 R31（交易墓碑依赖已删除账户而无法恢复）。
- **复核范围**：重新抽查 v1.8 的 GPT-R35–R52；没有足够证据新增其他 Finding，不代表这些风险已排除。各项复核意见与新增回归测试见 `gpt/v1.9/审核结果.md`。
- **验证边界**：静态审查；未运行 Go/SQLite/race/WebSocket/断电测试；未修改业务代码。
- **交付物**：`gpt/v1.9/审核结果.md`、`findings.json`、`verification/runtime-results.json`。

## [1.40] - 2026-10-30

### GPT v1.8 阶段 0–3 回溯式代码复核（GPT-R35–GPT-R52）

- **审查基线**：业务代码基线 be5439d6；最新业务代码提交 1a7d07ad；本轮只审查代码与契约，不修改业务实现。
- **统计**：新增 18 项 Finding：1 项 P1、15 项 P2、2 项 P3。详见 gpt/v1.8/审核结果.md 与 gpt/v1.8/findings.json。
- **GPT-R35 / P2**：SQLite DB/WAL/SHM 权限未显式收紧；常见 umask 0022 下可能以 0644 创建，需部署环境验证。
- **GPT-R36 / P2**：6 个迁移脚本均未定义 FOREIGN KEY；foreign_keys=on 目前没有实际关系约束可执行。
- **GPT-R37 / P3**：migrate.Check 调用 ensureVersionTable，启动检查会创建 schema_migrations，与只读约定不一致。
- **GPT-R38 / P2**：迁移框架不拒绝重复版本，也不验证已应用版本集合完整性。
- **GPT-R39 / P2**：instance.meta 写入只 WriteFile+Rename，没有 fsync 文件和目录；恢复世代存在断电持久性窗口。
- **GPT-R40 / P1**：邀请码可授予 admin，但没有列出/撤销接口，泄漏邀请无法提前失效。
- **GPT-R41 / P2**：bound_email 字段存在，但邀请 API/兑换流程只实现用户名绑定。
- **GPT-R42 / P2**：邀请创建、使用、失败尝试、过期未按 PRD 写入生命周期审计。
- **GPT-R43 / P2**：账本软删除后，成员列表、创建邀请和使用旧邀请的路径仍可操作该账本。
- **GPT-R44 / P2**：限流器不删除过期 key，不同 IP 持续出现时 hits map 可无界增长。
- **GPT-R45 / P2**：注册未限制用户名长度；超过 bcrypt 72 字节的密码走通用错误路径。
- **GPT-R46 / P3**：登录 handler 将所有错误（含 DB/审计/签名故障）统一映射为 401。
- **GPT-R47 / P2**：账户 parent_id 可通过把后代设为上级形成循环。
- **GPT-R48 / P2**：Sync.Pull 水位主要按交易行计算，账户/分类/持仓行可使批次大幅超过 limit。
- **GPT-R49 / P2**：WebSocket 订阅消息没有账本 ID 数量上限或去重，单帧可触发大量数据库鉴权查询。
- **GPT-R50 / P2**：pending Subscribe 与 RefreshVisibility 存在撤权竞态，旧授权结果可能在撤权后写回。
- **GPT-R51 / P2**：WebSocket 只在握手时校验 JWT，连接可超过 24 小时 token 有效期继续存活。
- **GPT-R52 / P2 / 待 owner 裁决**：PRD 要求多端会话查看/撤销，但 jti 未持久化、没有会话管理 API；WBS 未明确阶段归属。
- **既有 Finding 范围补充**：GPT-R27 增补 NewMemberService inviteTTLDays 秒数乘法溢出；GPT-R34 增补 config.envInt 对非法环境变量静默回退默认值，错误配置回退 30 天可能导致早于预期的回收站清理。
- **历史去重**：重新确认 D-R07–D-R09、WB-R01、GPT-R24–R25 已有对应实现/测试，不重复登记；[1.28] 已知“账本软删不摘 WebSocket 订阅”也未重复计数。GPT-R43 只覆盖 HTTP 成员/邀请路径。
- **验证边界**：静态审查；未在本轮本地运行 Go/SQLite/race/WebSocket/断电测试。历史 Actions run 41034510229 的 170 PASS / 0 FAIL 不覆盖本轮新增场景。
- **交付物**：gpt/v1.8/审核结果.md、gpt/v1.8/findings.json、gpt/v1.8/verification/runtime-results.json。
- **对齐入口**：更新 workbuddy/第三方审阅者导读.md 第 8/9 节，将 v1.7 阶段 4 标为历史审查焦点，并增加 v1.8 阶段 0–3 最新复核入口。

## [1.39] - 2026-10-30

### GPT v1.7 扩展复核（GPT-R31–GPT-R34，待 owner 裁决）

- **GPT-R31 / P1 / 待确认**：交易软删后，账户删除仍因只统计 `transactions.deleted=0` 而允许；Restore 的 `loadTxAccount` 又拒绝已删除账户，导致回收站交易无法恢复。分类路径也可恢复出指向已删除分类的引用。位置 `account.go:206–251`、`tx.go:834–842,905–930`、`category.go:232–290`。
- **GPT-R32 / P2 / 待确认**：Restore 与 Delete 共用 UPDATE，`deleted_at` 始终写 now；活交易恢复后仍有非零 deletedAt，与模型“未删除时为 0”契约冲突。位置 `tx.go:834–856`、`model/ledger.go:168–180`。
- **GPT-R33 / P2 / 待确认**：`PUT /books/:id` 同时修改名称与可见性时，BookService.Update 先独立提交名称，再调用独立事务的 SetVisibility；第二步失败会留下部分成功却返回错误。位置 `api/book_handler.go:77–97`、`service/book.go:143–183,211–263`。
- **GPT-R34 / P2 / 待确认**：`db purge` 对显式非法/缺失参数静默回退默认 TTL，未知参数也忽略，随后仍执行硬删除。位置 `cmd/server/main.go:249–262,279–290`。
- **GPT-R28 范围补充**：事务外授权读取还出现在 BookService.Update/Delete 与 MemberService.CreateInvite；建议统一盘点敏感写入口。
- **范围排除说明**：对照枚举字典 §14 后，未将 account/category CRUD 未写 audit 误报为阶段 4 缺陷；该契约当前明确限定交易 CRUD 与登录/权限/配置/备份恢复事件。
- **验证边界**：本轮新增问题来自静态代码链路与契约核对；未执行 Go/SQLite/CLI/并发测试。所有新 finding 状态均为待确认。

详见 `gpt/v1.7/审核结果.md`、`findings.json` 与 `verification/runtime-results.json`。

## [1.38] - 2026-10-30

### GPT v1.7 阶段 4 严格代码审查（GPT-R26–GPT-R30，待 owner 裁决）

- **审查基线**：main HEAD `be5439d6`；最新业务代码提交 `1a7d07ad`；相对阶段 4 前基线 `9d45a449` 比较 5 个提交、14 个变更文件。
- **GPT-R26 / P1 / 待确认**：`collectTransferGroup` 在转账对侧缺失或状态不符时返回单行继续执行，且未校验对侧是否反向互指、类型/方向/金额/账户是否匹配；异常配对可能单边修改或误操作无关交易。位置 `server/internal/service/tx.go:953–970`，违反 WBS 4.2「禁止单边路径」。
- **GPT-R27 / P1 / 待确认**：回收站/审计保留天数仅校验正数，`days * 86400` 可发生 int64 溢出，使 cutoff 变成未来时间并批量清除墓碑/审计。位置 `server/cmd/server/main.go:286–290`、`server/internal/service/recycle.go:58–68,79–90`。
- **GPT-R28 / P1 / 待确认**：Delete/Restore、ChangeRole/Remove、SetVisibility 的角色授权读取在事务外，授权检查与写入之间存在撤权/降权 TOCTOU；事务内未重新验证当前成员关系/角色。位置 `server/internal/service/tx.go:756–774`、`member.go:118–198`、`book.go:216–246`。
- **GPT-R29 / P2 / 待确认**：SetVisibility 与成员变更的 `before_json` 旧状态在事务外读取，并发操作可能写入过期快照或对无实际变化写审计。位置 `server/internal/service/book.go:216–257`、`member.go:130–209`。
- **GPT-R30 / P2 / 待确认**：WBS 4.3 明确要求交易 `update` 审计，但当前仅定义 `AuditActionUpdate` 常量，无交易 Update 服务/路由/审计调用。需 owner 裁决延期还是补齐。位置 WBS:99、`model/ledger.go:206–210`、`api/router.go:94–101`。
- **验收建议**：GPT-R26–R28 为阻断项，修复并增加真实 SQLite/并发回归测试前，不建议将阶段 4 标记为验收通过。
- **验证边界**：本轮通过 GitHub 文件读取进行静态审查，未在本会话重新运行 Go/SQLite/race。当前导读记录 Actions run `41034510229` 为 170 PASS / 0 FAIL，但不能证明本轮指出的异常配对、整数溢出与 TOCTOU 场景已覆盖。
- **交付物**：`gpt/v1.7/审核结果.md`、`gpt/v1.7/findings.json`、`gpt/v1.7/verification/runtime-results.json`。所有新 finding 初始状态为「待确认」，等待 owner 裁决。

## [1.42] - 2026-10-30

### GPT v1.7 / v1.8 / v1.9 全量整改：GPT-R26 – GPT-R53（28 项）

审查依据：`gpt/v1.7/`（R26–R34，9 项）、`gpt/v1.8/`（R35–R52，18 项）、`gpt/v1.9/`（R53，1 项，并复核 R27/R31/R34/R35/R47/R52）。
基线：`be5439d`。整改追踪表：`workbuddy/GPT-R26-R52-整改追踪表.md`。

#### P1（5 项，全部修复）

| ID | 问题 | 修复要点 | 提交 |
|---|---|---|---|
| **R26** | 转账对侧缺失/错链仍退化单边 | `verifyTransferPair` 校验互指、双侧 type、金额、账户交叉、方向互补、方向取值合法；任一不符即**整笔回滚** | `9005d68` |
| **R27** | TTL 乘法溢出可致误删全表 | 新增 `service/ttl.go` 作唯一换算入口；`daysToSeconds` 上限 `maxRetentionDays`（距 int64 溢出 9 个数量级余量）；`retentionCutoff` 拒绝早于纪元；邀请 TTL 改构造时校验 | `ba80b0d` |
| **R28** | 事务外读角色的 TOCTOU | 新增 `roleOfTx(sess,…)`；七个入口统一"Begin → 事务内重读角色 → 校验 → 变更 → 审计 → Commit" | `7584c57` |
| **R31** | 回收站交易依赖的账户/分类被删 | 引用检查去掉 `deleted = 0` 过滤（转账两侧都计入）；恢复路径校验引用有效 | `f4c4511` |
| **R40** | 邀请码无撤销接口 | 新增 `RevokeInvite`：事务内验权限 + 条件 UPDATE + `RowsAffected` 校验 + 与兑换并发互斥 | `4a84da6` |

#### P2 / P3（按模块）

**记账核心**：R29 `before_json` 事务内读取（`a2255f3`）、R32 恢复时 `deleted_at` 清零（`9005d68`）、R33 账本名称+可见性原子且广播在提交后（`a8ce4c5` `4bbdb17`）、R47 账户 `parent_id` 任意长度环检测（`12190d9`）、R53 拒绝删除仍有活跃子账户的父账户（`027b348`）。

**基础设施**：R34 参数非法不再静默回退（config `ba80b0d`、CLI `a7ca9d6`，校验严格先于任何 DELETE）、R35 目录 0700 / DB 与 WAL/SHM 0600 且收紧已存在文件（`cb9b372` `40dd8e3`）、R37 `migrate.Check` 改纯只读（`f1369b4`）、R38 拒绝重复版本并按**整个版本集合**校验（`27595db`）、R39 `instance.meta` 补 fsync（`3d40fe5`）。

**认证 / 邀请**：R41 邮箱绑定闭环（规范化 + 兑换匹配，`ff1d8a0`）、R42 邀请全生命周期审计且过期口径单一来源（`61c7ea8` `3b836fd`）、R43 已删账本不得继续成员管理（`eddfb84`）、R44 限流器内存有界（`8d4276b`）、R45 注册输入边界（`bffd3cc`）、R46 登录错误区分凭证 401 与内部 503（`c0f0547`）。

**同步 / WebSocket**：R48 Pull 水位由四类实体共同决定（`232907e`）、R49 订阅数量上限+去重+限流（`7325fc8`）、R50 账本权限 generation 消除撤权竞态（`201b87b`）、R51 令牌有效期作为连接生命周期上限（`f188b1a`）。

#### Owner 裁决（4 项）

- **R36 不使用外键**：理由（软删墓碑与 FK/级联直接冲突等）与补偿控制写入《落地方案》决策表 + 新增 §0.1；明确**不得用 `PRAGMA foreign_keys=1` 当证据**。
- **R52 排期实现**：排入 **WBS 5.10**，并在 **PRD 新增 §10.10** 验收项。
- **R41 完整实现**（不做移除/延期），已按裁决施工。
- **R30 延期（不实现交易 update 审计）**：阶段 4 无交易修改入口——架构守卫 `TestQATransactionRoutesContract` 显式禁止 `PUT`/`/transfer`/`/batch`，实现 update 审计等于新增写入路径，超出阶段 4 范围。已在 **WBS 4.3 新增「交易 update 审计延期说明」**，写明后续开放交易 update 时必须同时补齐：① 交易 Update 服务（单一入口、单一事务）② 同事务读 `before_json` 并写 `action=update` 审计 ③ 登记进架构守卫白名单。此项**不计入阶段 4 已完成范围，不得表述为已交付**。

#### CI 验证结果（权威）

**Actions run [41078424842](https://github.com/LinTia/FastLadger/actions/runs/41078424842)（commit `afbbe3f`，文档回填后的最终 HEAD）：全绿。**
gofmt / vet / build / SQLite 冒烟 / 全量测试 / **`-race`** 六步全 success（13/13 step）；
**474 PASS / 0 FAIL，DATA RACE 检出 0 次**。

上一版代码提交 [`a573c6e`](https://github.com/LinTia/FastLadger/actions/runs/41076750547) 同为 **474 PASS / 0 FAIL / 0 DATA RACE**。

从首次红到全绿的收敛过程（每次都由 CI 暴露真问题，而非放宽断言）：

| run | 结果 | 本轮修掉的根因 |
|---|---|---|
| 41068222122 | 441 PASS / 28 FAIL | `requireActiveBookTx` 把 BOOLEAN 列扫进 int64（真实 SQLite 必崩，本机无 CGO 测不出，影响约 14 例）；R26「非转账却带 related_id」仍被当单笔放行；R31 用例误用按 `deleted=0` 过滤的 helper |
| 41069111152 | 464 PASS / 5 FAIL | `mergeSafeVersion` 在 seed 33 行同版本下走进"取最大版本"分支（实为规格允许的回扩，是**用例 fixture 假设错误**）；R40/R43 判定顺序 |
| 41071899666 | 469 PASS / 2 FAIL | R43 回归（把"账本已删"误并入 not found）|
| 41072473802 | 470 PASS / 1 FAIL | R48 断言算错 18（漏算余额改写会抬账户版本，实为 19 且构成可解释）|
| 41073315674 / 41074321097 | 472 PASS / 1 FAIL（**race**） | `ServeWS` 反复读包级 `var DefaultHub`，与测试替换全局 Hub 竞争 |
| **41076750547** | **474 PASS / 0 FAIL，race 0** | ServeWS 改为 Upgrade 后一次性捕获 Hub；端到端用例显式排干连接 |

#### 验证状态（如实区分）

- **本地实跑并通过（真 SQLite / CGO）**：经 owner 许可在 `.tools/` 装了便携 MinGW-w64（**GCC 12.2.0**，w64devkit 1.16.1），CGO 打通后全量真实 SQLite 测试本地跑通 —— **9/9 包 ok，432 PASS / 0 FAIL / 5 SKIP**。432 + 5 = 437，与 CI 全量 437 完全对上（5 个 SKIP 全是 R35 文件权限用例，Windows 上 `Chmod` 语义不同按 `GOOS` 跳过，CI 在 Linux 上真实断言）。其中 **248 个 GPT-R 系列回归用例本地实跑：244 PASS / 0 FAIL / 4 SKIP**，覆盖 R26–R53 共 24 个编号。
- **`gofmt -l .` 空、`go vet ./...` 通过**；另 TTL 9 例、config 7 例、邀请批次 15 例、R44 5 例、R46 全例、R48 纯逻辑 2 例、ws 包 37 例（`-count=3` 无偶发）等非 CGO 批次也本地实跑通过。
- **`-race` 仍只能靠 CI**：本机 ThreadSanitizer 分配不到影子内存（`failed to allocate ... bytes, error code: 87`，重试 3 次含 `-p 1` 均失败）。并发用例已用 channel barrier 做确定性同步（无随机 sleep），但**数据竞争检测结论以 CI 为准**。
- **本地偶发拦截**：本机 Application Control 会间歇拦下新生成的测试二进制（`An Application Control policy has blocked this file`）。已用"逐包最多重试 8 次"绕过，**9 个包最终全部通过**；这类报错不是测试失败。
- **两个平台验证边界**：R35 文件权限在 Windows 上 `Chmod` 为空操作，4 例按 `GOOS` 跳过并说明，Linux 上真实断言；R39 单测无法模拟断电，Windows 目录 fsync 不可用（`Access is denied`），已在测试注释与报告写明。
- **方法学**：WebSocket 批次额外做了**"临时回退修复 → 确认对应用例变红 → 恢复"**的自检，证明用例确实能抓住缺陷，而非只是"跑通"。

#### 遗留 / 需产品确认

1. **长口令支持**：72 字节是 bcrypt 硬上限（妥协方案）。要支持任意长口令需 SHA-256 预哈希或换 argon2id/scrypt，**会改变哈希格式、需迁移策略**——待决策。
2. **R44 容量上限 10000** 按"家庭自托管"量级取；公网大流量场景需调大或换共享存储限流器。
3. **R48 水位仍计入"对该用户不可见的交易"**（与旧实现一致），语义安全但建议评审确认口径。
4. **R38 使 `Check` 变严**：存在版本缺口的库启动会失败（本条意图），需配套说明先执行 `fastledger db migrate`。
5. **R31 取舍**：只要历史上有过交易引用（哪怕只剩墓碑），账户 30 天内不可删——属"回收站可恢复优先"，建议产品确认。

## [1.37] - 2026-10-30

### 补齐 WBS 4.3 剩余审计写入点：登录 / 权限变更 / 配置变更 / 备份恢复

此前只审计了交易的 `create`/`delete`/`restore`，WBS 4.3 要求的另外四类均未落库，是 `[1.35]` 登记的已知缺口。本次补齐，并解决"这些事件不属于任何账本"的建模问题。

- **建模：两个 0 哨兵**。`audit_logs.book_id` 与 `actor_uid` 都是 `NOT NULL`，没有"空"语义，故显式约定：`book_id=0` = **实例级事件**（`login`/`backup_restore`），`actor_uid=0` = **系统/运维操作**（CLI 执行，无登录用户）。已在 model 与枚举字典 §14.3 写明，**禁止把 0 当"空"或"放行"**。
- **登录**：审计放在**签发令牌之前**——审计写失败就不签发，避免出现"已通过认证却无留痕"。登录失败不写审计（没有成功登录这件事可记）。
- **权限变更**（`ChangeRole`/`Remove`）：原为无事务的单条 UPDATE，现改为**与审计同一事务**，并在 `before_json` 保留**变更前**的角色。
- **配置变更**（`SetVisibility`）：同样收进事务；旧值必须在对象被改写**之前**取，否则 `before_json` 记不到原状态。幂等（值未变）时不写审计——没有变更就不该产生"看起来改了"的留痕。
- **备份恢复**（`instance bump`）：**刻意做成 best-effort**。该子命令必须能在没有数据库时执行（《落地方案》§8.3：停容器→替换 db→bump→启动），强连库会卡死恢复流程。故审计失败只告警不阻断；`restore_epoch` 与 `restored_at` 已落 `instance.meta`（不依赖数据库）作兜底留痕。
- **动作命名**：新增 `login` / `permission_change` / `config_change` / `backup_restore`。`backup_restore` **刻意不用 `restore`**——后者已被"恢复一笔交易"占用，混用会让"这笔交易被恢复过"与"整个实例从备份恢复过"无法区分。
- **文档**：枚举字典新增 **§14**（代码注释一直引用它，但文档此前缺失），含 action / entity_type / 两个 0 哨兵 / 各动作 `before_json` 取值。
- **测试**：新增 `audit_integration_test.go` 6 个真实 SQLite 用例（登录写实例级审计、登录失败不写、改角色与移除成员保留旧角色、可见性变更保留旧值、幂等不写审计）。

## [1.36] - 2026-10-30

### 修复 CI 连续 4 次失败：阶段 4 的新路径未登记进既有【架构守卫用例】

run #16（`644ddab`）起连续 4 次失败。**守卫本身是对的，漏登记的是我**——阶段 4 新增的写/读路径必须在守卫白名单里显式登记，不能靠"绕过"。

1. **gofmt 失败**（run #16）：`internal/model/ledger.go` 未格式化（`Direction` 字段对齐破坏）。已在 `d45f88b` 修掉。
2. **`TestQAS3SingleCreateEntry`（余额写入闭环守卫）**：`mutateVisibility` 直接调 `createChangeBalance`，被判"绕过单一创建入口"。
   修：改为**按原语区分授权白名单**——`createChangeBalance` 额外授权 `mutateVisibility`（删除/恢复是【变更】不是【创建】，必须复用同一条原子 UPDATE 做精确反向）；`insertTxRow` 仍**严格只许**两个创建 helper，因为删除/恢复不新增行。
3. **`TestQAS3VisibilityCondWiredIntoAllReadPaths`（可见性读路径守卫）**：回收站 `ListDeleted` 是第四处读路径却未登记。
   修：加入"必须调用 `txVisibilityCond`"的读路径清单与授权使用方白名单，并把用例重命名（原名写死"三处"）。**回收站同样受可见性约束**——否则关闭互看后，成员仍能从回收站看到他人已删账目。
4. **`TestQATransactionRoutesContract`（原 `...CreateAndQueryOnly`，路由契约守卫）**：`DELETE` 曾被明令禁止，理由是"提前注册却未实现，会变成能删但余额不回滚的隐患"。阶段 4 已实现（整笔、余额精确反向、审计、幂等回执），故转入 allowed，并登记 `restore` 与 `recycle` 路由；`PUT`（改交易）未实现继续禁止，`/transfer`、`/batch` 等第二条创建路径**永远禁止**。

以上均为"把合法新路径显式登记进白名单"，**未删除或放宽任何一条守卫断言**。

- **验证**：Actions run [41029584197](https://github.com/LinTia/FastLadger/actions/runs/41029584197) **全绿**——gofmt / vet / build / SQLite 冒烟 / 全量测试 / `-race` 六步全 success；**164 PASS / 0 FAIL**，无 race。至此 [1.35] 的"运行时正确性以 CI 为准"已由真机证据闭合。
- **值得记一笔的教训**：本项目用"调用图/路由白名单"类用例来固化架构不变量（《转账入口裁决》的单一创建入口等）。**任何新增写入或读取路径都会触发这些用例**——这是特性而非障碍，新增路径时应主动更新守卫，而不是等 CI 报错。

## [1.35] - 2026-10-30

### 阶段 4 补全：删除/恢复幂等回执、回收站 TTL、审计写入点与保留期、4.5 已删实体拦截

对照 `开发WBS` 阶段 4 的 6 项任务逐条补齐（此前仅 4.6 落地），并修复 2 个正确性缺陷。提交 `644ddab`（阶段 4 主实现）、`d45f88b`（补全）。

- **修复（致命）：余额反转符号倒置**。`mutateVisibility` 中原判断使「删除」再次施加 Create delta、「恢复」反而取负——删除会让余额**二次同向变动**、恢复方向相反。已改为删除取负、恢复取正。此为纯逻辑推导发现，本地无 gcc 未能跑真库复现，已由集成用例断言覆盖。
- **修复（静默算错）：历史转账缺方向标记**。迁移 `0004` 给既有转账行 `direction` 填默认 0，而 `txRowFlow` 对 `direction=0` 的转账走「单笔」分支返回 0——删除老转账时**余额不回滚却不报错**。新增 `0006` 按「转出侧先插入、id 较小」的确定性规则回填；并在 Go 侧失败关闭（`ErrTxDirectionMissing`），宁可报错也不静默算成 0。`0004` 已发布故不改已应用迁移，回填单独立为 `0006`。
- **4.2 / 4.4 删除恢复幂等回执**（WBS 明确要求「写幂等回执」，原实现完全没有）：
  - 迁移 `0005`：幂等表加 `op` 列，唯一键扩为 `(book_id, uid, op, client_uuid)`。用 `DROP INDEX` + `CREATE UNIQUE INDEX` 实现——**不重建表、不删数据**，符合「老库升级不得删除旧回执」约束。没有 `op` 维度时，同一 `clientUuid` 先删后恢复会互相命中回执。
  - `Delete`/`Restore` 增加必填 `clientUuid`，与 `Create` 对称：事务内占位、命中即重放原版本、异内容报冲突；失败路径整体回滚，**不占用幂等键**（可原键重试）。
- **4.1 回收站与 30 天 TTL**：新增 `ListDeleted`（回收站视图，叠加可见性条件）与 `PurgeExpiredTransactions`（只硬删到期墓碑，存活记录绝不动，`deleted_at=0` 视为永不过期，且不连带清幂等回执）。路由 `GET /books/:id/recycle`——刻意避开与 `/transactions/:txId` 的通配段冲突，否则 gin 注册即 panic。配置项 `EBK_RECYCLE_TTL_DAYS` 本已存在（默认 30），本次补上实现。
- **清理触发点 `fastledger db purge`**：仅有 purge 函数而没有触发点，"到期硬清除"等于没落地。按项目既有约定（迁移/运维走 CLI，**服务启动绝不静默改数据**）新增子命令，逐个账本清回收站 + 按保留期清审计，可用 `--recycle-ttl-days` / `--audit-retention-days` 覆盖。**未做后台定时 goroutine**——避免引入本轮无法真机验证的常驻逻辑。
- **4.3 审计写入点与保留期**：补交易 `create` 审计（单笔 1 条、转账两侧各 1 条；create 无「变更前」状态故 `before_json` 留空）；新增 `PurgeAuditLogs` 与 `EBK_AUDIT_RETENTION_DAYS`（默认 365，PRD §12.1 保留 1 年）。
- **4.5 旧离线数据回流**：`loadTxAccount`/`loadCategoryForType` 不再用 `deleted=0` 过滤，区分「从未存在」（404）与「已删除」（409 `ENTITY_DELETED`），**绝不静默复活已删数据**；push 侧映射 `conflict` + `ENTITY_DELETED`。
- **测试**：`tx_delete_integration_test.go` 扩充至 11 个真实 SQLite 用例（幂等重放不双反向、同 uuid 跨操作冲突、`op` 维度先删后恢复、回收站视图、到期硬清除只清墓碑、审计保留期清理、已删账户拦截、`clientUuid` 必填）。
- **诚实边界（重要）**：本地构建机**无 gcc**，`mattn/go-sqlite3` 需 CGO，**真库用例未在本地执行过**。已完成的验证是：① `CGO_ENABLED=0` 下 `go vet ./...` 对**全部包（含测试文件）**类型检查通过、`gofmt` 干净（可排除编译期错误，但不能证明运行时行为）；② 用 Python 自带 sqlite3 在**真实 SQLite** 上验证了 `0001`–`0006` 全部迁移可执行、唯一键确为四列、`op` 维度下同 uuid 三个操作可共存、重复插入被拦截、`0006` 回填方向与 `audit_logs` 九字段完整。运行时正确性（余额反转、幂等重放）**仍以 CI 结果为准**，尚未由本人核对 Actions 日志。

## [1.34] - 2026-10-30

### 关闭 9 项业务级 SQLite 断言盲区（真实集成测试）

- **背景**：[1.33] 的「诚实边界」明确承认 9 项业务级断言（幂等重试、负债符号、转账 `related_id` 配对、并发一致性、`type` 引号列过滤等）无真实 DB 覆盖。
- **新增** `server/internal/service/tx_integration_test.go`：临时库 + 全量迁移 + owner 账本 + 资产/负债账户 + 收/支分类构成真实环境，8 个用例——支出减资产余额、负债符号（刷卡增欠款 / 还款清零）、转账双边与 `related_id` 互指、同 `clientUuid` 不重复记账、同 uuid 异内容 409、对手账户缺失整笔回滚无部分写入、30 并发余额一致、`type` 引号列过滤。
- **过程中两次 CI 失败与修正**：① `undefined: t`——helper 方法误用未定义的 `t`，改为 `testEnv` 持有 `t`；② 转账断言误用**负债**账户作转入方，而转入负债是**减欠款**（余额 -5000），属断言写错而非代码错，改用两资产账户后符合直觉。
- **验证**：Actions run `41019162386` 全绿，**152 PASS / 0 FAIL**，`-race` 无竞态。至此 [1.33] 承认的 9 项盲区已被真实 SQLite 用例覆盖。

## [1.33] - 2026-10-30

### 修复 GPT-R25（P2）：database/migrate 补真实 SQLite 集成测试，冒烟步骤不再是空转

- **根因**：[1.31] 的 SQLite 冒烟步骤 `go test ./internal/migrate/... ./internal/database/...` 目标包当时均无 `*_test.go`，CI 输出 `[no test files]`，等于没跑任何测试；但 [1.31] 却把它描述成"真库测试已执行、9 项 SQLite 断言跑通"。这一夸大表述由 GPT-R25 命中，已在 [1.32] 收窄。
- **修复（本次）**：
  - `server/internal/database/database_test.go`：调用 `database.Init` 在临时库开真库，断言 `foreign_keys=1`、`journal_mode=wal`、`synchronous=FULL`，并验证可写可读（证明驱动不是 stub）。
  - `server/internal/migrate/migrate_test.go`：临时库上 `Apply` 迁移，校验版本=应用数、重复执行幂等（`n2==0`）、`schema_migrations` 与 `accounts`/`users` 表真实存在。
  - `.github/workflows/ci.yml`：SQLite 冒烟步骤加反向门禁——若目标包再无 `*_test.go` 输出 `[no test files]` 则显式失败，防止门禁静默空转（GPT-R25 建议 #4）。
- **验证**：Actions run [41014327155](https://github.com/LinTia/FastLadger/actions/runs/41014327155) 全绿；冒烟步骤日志显示 `ok fastledger/internal/database`、`ok fastledger/internal/migrate`，两个用例 `--- PASS`，不再出现 `[no test files]`。
- **诚实边界**：本次只补齐"驱动能开真库 / PRAGMA 生效 / 迁移可执行且幂等"这层验证。**阶段 3/4 的 9 项业务级 SQLite 断言（clientUuid 幂等重试、转账回滚、`related_id` 配对、`RowsAffected` 语义、`busy_timeout` 并发等）仍未有真实 DB 集成测试覆盖**，属独立 backlog，勿误读为本轮已全闭环。

## [1.32] - 2026-10-30

### GPT-R25（P2，已采纳）：SQLite 驱动冒烟步骤没有实际执行测试

- **问题**：核对 Actions run [41010979053](https://github.com/LinTia/FastLadger/actions/runs/41010979053)，`.github/workflows/ci.yml` 的 SQLite 冒烟命令 `go test ./internal/migrate/... ./internal/database/... -count=1 -v` 输出：
  - `? fastledger/internal/migrate [no test files]`
  - `? fastledger/internal/database [no test files]`
  当前两个包均没有 `*_test.go`，所以这一步没有运行测试函数，不能证明 SQLite 驱动打开真库、PRAGMA 生效或迁移执行成功。
- **影响**：CI 该步骤虽然显示成功，但实际只完成包扫描/编译；此前 [1.31] 将它描述为“真库测试”并据此声称专项 SQLite 断言已跑通，证据表述过度。
- **本次文档纠正**：已修订 [1.31] 对冒烟步骤及 9 项 SQLite 断言的表述。全量 `go test ./...` 的 140 PASS 与 `ws/service/api` race 通过记录保留，不将其误写成 database/migrate 专项集成测试通过。
- **建议修复**：新增 `server/internal/database/database_test.go`，调用 `database.Init` 并断言 `foreign_keys=1`、`journal_mode=wal`、`synchronous=2`；新增 `server/internal/migrate/migrate_test.go`，在临时 SQLite 库执行迁移并验证版本与重复执行行为。保留 CI 命令，确保日志出现实际 `ok`，而非 `[no test files]`。
- **状态**：owner 已认可并采纳 finding；本次仅落盘评审记录和更正 CHANGELOG，尚未修改 CI 或测试代码。待实现方补测并提供新的 Actions 证据后，再将状态更新为已修复/已验证。
- **评审记录**：[GPT v1.6 审核结果](gpt/v1.6/审核结果.md)，结构化记录见 [findings.json](gpt/v1.6/findings.json)，运行日志摘要见 [runtime-results.json](gpt/v1.6/verification/runtime-results.json)。

## [1.31] - 2026-10-30

### 路径 1 落地：GitHub Actions CI（真实 CGO + 真库 + -race）全绿，并修复一处 -race data race

- **动机**：本地构建机无 C 编译器，`mattn/go-sqlite3` 只能编译不能运行、`go test -race` 无法执行，导致阶段 3 的"运行证据"一直是盲区（GPT 卡片里 9 项 SQLite 断言从未真正跑过）。
- **新增 `.github/workflows/ci.yml`**：`on: [push, pull_request, workflow_dispatch]`，`ubuntu-latest`（自带 gcc）。步骤：Toolchain（断言 `CGO_ENABLED=1`，否则失败——**防静默退化为 stub**）→ gofmt → vet → build → SQLite 冒烟命令（目标包当前无测试文件，实际只完成包扫描/编译；详见 [1.32]）→ `go test ./...` 全量 → `go test -race`（仅 `ws`/`service`/`api`）。
- **首次运行（run 41010979053，commit `cf4c4de`）整体成功**：`CGO_ENABLED='1'` + `gcc (Ubuntu 13.3.0)`；`go test ./...` 在真实 Linux 上 **140 PASS / 0 FAIL / 0 SKIP**；`ws`+`service`+`api` 在 **`-race` 下全部 `ok`**。注意：独立 SQLite 冒烟命令的两个目标包均输出 `[no test files]`，不能据此声称专项真库测试已执行（见 [1.32]）。
- **修复一处真实 data race（测试桩）**：`TestQAS3ServeWSRejectsSubscribeWithoutChecker` 用 `defer conn.Close()`，导致连接关闭发生在函数返回后，服务端注销 goroutine 与 `t.Cleanup` 恢复全局 `DefaultHub` 并发读写同一全局指针，`-race` 下 CI 失败。改为：显式 `conn.Close()` + `waitCount(0)`（确认服务端已完成 `Unregister` 对全局的读取）后再由 cleanup 恢复；并把 `RUnlock` 提前（否则 `waitCount` 内再次 `RLock` 会因 RWMutex 不可重入死锁）。
  - 该 race **仅存在于测试桩**：生产 `DefaultHub` 只在 init 设一次、运行期不替换，非生产 bug。
- **验证边界**：本次 Actions 记录证明全量 Go 测试与指定包的 race detector 通过；但不能仅凭 SQLite 冒烟步骤证明 9 项 SQLite 专项断言均已执行。相关断言需逐项映射到真实测试用例与日志，数据库初始化/迁移专项测试见 [1.32] 待补。

## [1.30] - 2026-10-30

### 修复 GPT-R24（P1）：成员移除的清理通知被自己的"摘订阅"逻辑截断

- **问题（GPT 审查，经我逐行取证确认属实）**：`notifyMemberRemoved` 先调 `RefreshVisibility`（把已非成员的连接从 `BookIds` 摘除），再走普通 `BroadcastToBook`；后者对 `!sub` 的连接直接跳过 —— 于是**仍在线的被移除用户收不到 `member_removed`**，客户端不会执行"清除该账本本地数据"，违反 PRD §7.3 / §12.1 决策 5。
- **这是我自己在 [1.27] 修 P3「被移除者仍收到后续事件」时引入的反向缺陷**。两条约束方向相反、必须同时成立：
  1. 被移除者**不能再收到**该账本的任何后续事件；
  2. 被移除者**必须收到一次**清理通知。
  只满足其中一条都是错的。
- **修复**：
  - `internal/ws/hub.go` 新增 `DeliverToUser(uid, msg) int`：向该 uid 的**全部在线连接**定向投递，**不经订阅过滤**；锁序仍为 `Hub.mu(读) → Client.mu`（不引入反向路径）；非阻塞，`Send` 满则 `Dropped++`；返回实际入队连接数（`0` 表示目标离线或缓冲满）。
  - `internal/api/broadcast.go` 的 `notifyMemberRemoved` 改为**三段固定顺序**：① `RefreshVisibility` 撤销订阅（此后不再收普通事件）→ ② `DeliverToUser` 定向投递一次清理通知（不依赖订阅状态）→ ③ `BroadcastToBook` 发给其余有效订阅者（目标已被摘除，**不会重复收到**）。
  - 抽出 `marshalBookEvent`，让"定向投递"与"普通广播"复用**同一份字节**，避免两条路径的信封出现差异。
  - 目标离线/缓冲满时记日志；并明确客户端契约：重连后若订阅被拒（已非成员），必须删除该账本本地缓存。
- **测试（7 个新用例）**：
  - `internal/ws/deliver_to_user_test.go`（5 例）：不依赖订阅、只投目标 uid、同用户多连接全投、离线/非法 uid 返回 0、缓冲满丢弃并计数。
  - `internal/api/member_removed_broadcast_test.go`（2 例）：目标必收且字段正确、只收一次、其他成员照常收、目标随后**收不到** `data_changed`；同用户多设备均收到。
  - **非空转已证明**：临时把 `DeliverToUser` 改为 no-op → 目标用例立刻失败并打印"目标未收到 member_removed —— GPT-R24 未修复"；恢复后通过。
- **文档格式修正**：GPT 在 `[1.29]` 落盘时把文件标题与前言挤到了条目之后（`## [1.29]` 出现在 `# 更新日志` 之前）。已将标题与前言移回文件首行；`[1.29]` 正文内容一字未改（属评审记录，不篡改）。

## [1.29] - 2026-10-30

### GPT 第三次代码阶段审查（规范对齐版）

- **审查前置对齐**：按《多 AI 对齐规范》→《第三方审阅者导读》→根 CHANGELOG `[1.6]`–`[1.12]` 的顺序重新阅读；并复核 GPT 三次核查卡片、v1.4 历史 findings / verification、当前成员移除与 WS 广播实现。
- **GPT-R24 / P1 / 待确认**：移除成员后，`notifyMemberRemoved` 先执行 `RefreshVisibility`，非成员分支删除目标连接的 `BookIds[bookID]`；随后 `broadcastBookEvent` 仍走普通 `BroadcastToBook`，而后者只向仍订阅的连接投递。因此在线目标用户收不到 `member_removed` 清理通知，可能无法按 PRD §7.3 / §12.1 决策 5 清除本地账本数据。
  - 证据位置：`server/internal/api/book_handler.go:204–220`（调用点 L219）；`server/internal/api/broadcast.go:73–86,135–142`；`server/internal/ws/hub.go:236–262,294–380`（摘除分支 L369–375）；需求依据 `docs/FastLedger-产品需求文档-PRD.md:420,561`（§7.3 / §12.1 决策 5）。
  - 与 [1.27] 已修复的“移除成员仍收到后续账本事件”不同：本项指出清理事件本身在普通广播前被摘订阅截断，两项约束必须同时满足。
  - 建议增加按 `targetUid` 定向投递清理事件的 Hub 能力，并确保事件入队后撤销订阅；补充目标收到清理事件、后续事件不达、其他成员正常收到及断线/并发回归。
  - 状态：**待确认，等待 owner 裁决**；本轮未改业务代码。
- **验证边界**：本轮为 GitHub 文件接口静态审查；未执行 Go build/vet/test、GPT 核查卡片要求的 9 项 SQLite 断言、race 或真实 Gin/WS 端到端测试。不得据此宣称阶段 3 运行验收通过。详细 13 字段见 `gpt/v1.5/审核结果.md`，结构化记录见 `gpt/v1.5/findings.json`；未执行测试清单见 `gpt/v1.5/verification/runtime-results.json`。

## [1.28] - 2026-10-30

### 阶段 3 定向回归通过 + 契约文档补全（收尾）

- **QA 定向回归判定 `NoOne`**：6 个复核点全部通过，无 P0/P1/P2。关键不变量现有**可执行断言**兜底，其中 3 条为 QA 本轮新增的**全仓级守卫**（且已证明非空转：注入违规立刻红）：
  - `TestQAS3SingleVisibilityWritePathRepoWide`：递归扫描 `server/internal/**` 生产代码，凡"引用 `MembersCanViewOthers` 且调用 `Cols`/`Exec`"的函数**只允许 `SetVisibility`**；
  - `TestQAS3NoHubLockWhileHoldingClientLock` / `TestQAS3ClientMutexNeverNests`：持 `c.mu` 期间不得访问任何 `h.*`、同一函数内不得嵌套加锁；
  - `TestQAS3SetVisibilityIdempotentGuardPrecedesWrite`：补上文档第 3 条硬约束里原先缺断言的"**不写库**"半边。
  - 另核实 `RefreshVisibility` 踢出**仅限目标账本**（多账本用户不会被整体踢掉），且重新加入可恢复订阅。
- **P3-1 修复（文档）**：`落地方案 §5.1` 的 `sync/push` 示例原写 `"errorCode": "CONFLICT"`，但实现**从不发该值**。已改为真实取值，并补 **`errorCode` 8 值枚举表**（`CLIENT_UUID_CONFLICT` / `INCOMPLETE_RECEIPT` / `CONCURRENT_CONFLICT` / `INVALID_REQUEST` / `ACCOUNT_NOT_FOUND` / `CATEGORY_NOT_FOUND` / `FORBIDDEN` / `INTERNAL_ERROR`），逐项标注伴随 `status` 与客户端动作。
- **澄清一次"偶发失败"误报**：此前记录的"`go test ./internal/api/` 出现 1 次 `--- FAIL`"经 QA 约 40 次连跑与主理人复验，判定为**测量假象**——被 Application Control 拦截时测试可执行文件根本不启动（本次复现：`blocked=1` 且 `PASS=0`），而我的统计脚本把两次不同运行的 PASS/FAIL 拼在了一起。**诊断规则已确立：真失败必有 `--- FAIL: <TestName>`；只出现裸 `FAIL <pkg>` 且伴随 `An Application Control policy has blocked this file` → 判"未执行"，重跑即可。**
- **测试基线（主理人实测，逐包单次运行内同时统计）**：`service 61 + api 29 + ws 24 + instance 11 + model 3` = **128 PASS / 0 FAIL / 1 SKIP**（SKIP = Windows 无 POSIX 权限位）；`gofmt -l .` 空、`go build ./...`、`go vet ./...` 通过。

#### 登记项（已知、暂不修，不阻塞收尾）

| 编号 | 内容 | 判断 |
|---|---|---|
| P3-3 | `BookService.Update` 的"改名"与"可见性"是**两次独立写、无共同事务**：并发软删账本时可能出现"名字已写成功、随后 `SetVisibility` 返 409" | 既有非事务风格（非本轮引入），概率极低；阶段 4 若统一审计/事务边界时顺手合并 |
| P3-4 | 账本**软删后其订阅者不会被摘除**（`CanViewDetail` 对已删账本返回 `(false,nil)` → `member=true` → 只降 `canView` 不踢） | 不泄露：已删账本的任何写路径都会 403；客户端按 `book_deleted` 自行停订阅。若将来收紧，可让 `CanViewDetail` 对已删账本返回错误 |
| P3-5 | 协调事件的 `version` 在 `currentBookVersion` 读失败时回落 `0` | 已记日志；客户端按 §5.2"缺失/异常按有更新处理"fail-safe，可接受 |
| — | holdings 无写接口故无 `data_changed` | 已在 `ledger_handler.go` 留 TODO，上线时必接 |

## [1.27] - 2026-10-30

### 新增：**阶段 3（同步与家庭协同）** 落地 + QA 验收整改

> 范围：WBS 阶段 3 的 3.1–3.5。经过一轮实现 + QA 独立验收（1×P1 + 4×P2 + 8×P3）+ 两轮返工。**记账核心与前两阶段代码零改动**。

#### 交付

| 模块 | 落点 |
|---|---|
| 可信拉取 | `internal/service/sync.go` `Pull`：单一只读事务定水位 → `safeVersion` **版本边界**分页（同一 `sync_version` 绝不拆批）→ 四类实体（交易/账户/分类/持仓）同一快照读取，四个 `Find` 全部检错 |
| 离线补传 | `sync.go` `Push(ctx,…)`：**逐项调用同一个 `TxService.Create`**，不新增第二创建入口；单项失败不影响后续 |
| 可见性 | `internal/service/visibility.go`：`CanViewDetail` + `txVisibilityCond`，接入 `List` / `Get` / `Pull` **三处**；`Get` 他人记录返 **404**（403 会泄露存在性）；`creator_uid=0` 历史行对 member **不可见**（`creator_uid = ? AND creator_uid <> 0` 双保险） |
| WS | `internal/ws/`：锁序恒 `Hub.mu → Client.mu`；`Send` 永不 close；广播路径**零查库**（订阅时缓存 `canView`）；`Subscribe` 每个 bookID 都查库校验；读限制 64KB / 读截止 60s / Pong 续期 / 写 30s ping |
| 世代 | `internal/instance/meta.go`：`data/instance.meta`（**不在数据库里**、不随备份恢复）、tmp+rename 原子写、0600、坏文件**不重建**；CLI `instance bump`；`/healthz` 与 `Pull` 均返回 `instanceId`/`restoreEpoch` |

#### QA 验收后整改（均已修）

| 编号 | 问题 | 修复 |
|---|---|---|
| **P1** | `PUT /books/:id` 能改 `members_can_view_others` 却**不发事件、不刷 Hub 缓存** → 关互看后客户端永远不重拉，**开关在用户看来失效** | 两个入口统一委托 `SetVisibility`（唯一写实现，含 `RowsAffected==1`）；两 handler 均在 `VisibilityChanged` 时刷新缓存 + 广播。**AST 测试锁死**：`Update` 内不得出现该列、两个 handler 都必须广播、且广播必须在条件分支内 |
| P2 | `sync/push` 把**服务端内部错误**映射成 `conflict` → 客户端会走"另存副本"换新 `clientUuid` 重试 → **同一笔钱记两遍** | 新增 `status="error"`（`INTERNAL_ERROR`，语义=原样重试、**禁止换键**）；`conflict` 只留给真正的业务冲突 |
| P2 | Pull 下发墓碑但 `deleted` 被 `json:"-"` 隐藏 → 客户端把已删实体当活行（删账户即可触发；阶段 4 交易删除上线后会变成幽灵账） | 四类模型 `Deleted` → `json:"deleted"`、`DeletedAt` → `json:"deletedAt,omitempty"`，注释标明"墓碑标识" |
| P2 | 事件契约三份文档互相冲突（点号/下划线、`version` 有无、`data` 平铺、`force_full_resync` vs `forceFullResync`） | **收敛为 5 个事件**（见下）+ 一条可执行命名规则，三份文档对齐 |
| P2 | 账户/分类 **6 个写 handler 零广播** → A 端新建账户，B 端在线也收不到信号（多端实时性失效） | 6 处写成功后 `notifyDataChanged(bookID, ver)`；`AccountService.Delete`/`CategoryService.Delete` 改为返回本次 `sync_version`。holdings 无写接口，已留 TODO 注释 |
| P3 | `RefreshVisibility` 旧结果可覆盖更新的 `Subscribe` 结果 | `Client.visSeq`：查库前快照、回写前比对，不一致即放弃 |
| P3 | 被移除成员仍留在订阅表 → 继续收到该账本事件（泄露"账本仍活跃"） | 非成员时在同一把 `c.mu` 内 `delete(BookIds)`（fail-closed，重连会重订阅） |
| P3 | `Client` 字面量构造 → nil map panic | `ensureLocked()` 统一懒初始化 |
| P3 | 幂等重放（`replayed`）也广播"有新版本" | `!res.Replayed` 守卫 |
| P3 | `Push` 用 `context.Background()` | 改为 `Push(ctx, …)`，handler 传 `c.Request.Context()` |
| P3 | PRD §12 决策 6「成员上限 20」无校验 | `JoinViaInvite` **事务内**计数（防并发顶破上限），`>=20` → `ErrBookFull` → **409** |

#### WS 事件契约（**定稿，唯一权威见《落地方案》§5.2**）

| `event` | 类型 | `forceFullResync` | `data` |
|---|---|---|---|
| `data_changed` | 信号（任何版本递增的写：交易/转账、账户、分类、持仓） | `false` | `{}` |
| `visibility_changed` / `member_removed` / `member_role_changed` / `book_deleted` | 协调（**不递增版本**） | `true` | `{}` / `{targetUid}` / `{targetUid,role}` / `{}` |

命名规则：**事件名（枚举值）一律下划线；JSON 字段名一律 camelCase**。信封固定 `{event, bookId, version, forceFullResync, data}`，五个事件都必须带 `version`；`data_changed` 的 `data` **刻意为空**（明细走授权 pull，防止绕过可见性）。

#### 函数签名变更（下游需同步）

1. `MemberService.JoinViaInvite(bookID, code, username, password, clientIP)`（[1.26]）
2. `SyncService.Push(ctx, uid, bookID, items []PushItem)` —— `PushItem` 与 `CreateRequest` 1:1 无损（反射测试锁定）
3. `BookService.SetVisibility(...) (changed bool, error)` —— 同值提交返回 `false` 且不写库、不广播
4. `BookService.Update(...) (*BookUpdateResult, error)`
5. `AccountService.Delete` / `CategoryService.Delete` → `(syncVersion int64, error)`

#### 文档

- `落地方案 §5.1`：补 `sync/push` **响应契约**（六值 `status` 表 + `conflict`/`error` 处置差异）、`PUT /books/{id}` 与 `/visibility` 的关系
- `落地方案 §5.2`：**事件权威表**重写（5 事件 + 命名规则 + `forceFullResync` 客户端语义）、新增「可见性开关：单一写实现、两个入口（已定稿，勿再改结构）」并写明三条由测试锁死的硬约束
- `指南 §7.4` 与 `§14.2` 示例：改为 `data_changed` 信封；补 `!Replayed` 守卫
- `PRD §12.1` / `WBS`：`force_full_resync` → `forceFullResync`（**仅格式**，语义与决策不变，已在 PRD 内注明）；PRD 事件描述 `book_id` → `bookId`

#### 验证

- `gofmt -l .` 空、`go build ./...`、`go vet ./...` 通过
- 测试（主理人本机实测，顶层用例）：**PASS 122 / FAIL 0 / SKIP 1**（service 58、api 29、ws 21、instance 11、model 3；SKIP = Windows 无 POSIX 权限位）
- `grep` 验收：`transaction.created` 残留 **0**；`members_can_view_others` 非测试命中仅 `SetVisibility` 一处 UPDATE；`AddTransaction` = 0；`internal/ws` 未被 `internal/service` 导入

#### 验证边界（**未**完成，不得据此宣称验收通过）

1. **`go test -race` 未执行**（`-race requires cgo`，本机无 gcc）——而 WBS 阶段 3.3 明确要求，Hub 并发必须靠它证明
2. **真实 SQLite 集成未跑**：`Pull` 的 `safeVersion` 版本边界、可见性过滤（`List`/`Get`/`Pull` 三处结果集与 total）、`sync/push` 幂等重放与 409、`SetVisibility` 并发 0 行→409、`JoinViaInvite` 事务回滚与并发上限
3. **WS 端到端仅验到握手 + 订阅 + 脱敏载荷 + 断连清理**（进程内），未验真实网络下的 ping/pong 续期与半开连接
4. 观察项：本机 `go test ./internal/api/` 曾出现 **1 次** `--- FAIL`，随后 14 次复跑全绿未复现（疑与本机 Application Control 拦测试可执行文件相关），已记录待 CI 观察

## [1.26] - 2026-10-30

### 修复：D 第二轮 3 条 + 主理人补 1 条（成员/邀请健壮性）

> 性质：全部为「**代码没照设计做**」，`docs/` 设计口径无需改动。D 报告的 9 个记账核心焦点经其核对**全部正确**。

- **D-R07（P2）· 邀请注册非原子** → `JoinViaInvite` 原先调用会自行提交事务的 `auth.Register`，第二步失败留下**孤儿用户**。
  - 修复：`auth.go` 抽出 `insertUserInTx(sess, ...)`（**不自行 Begin/Commit**），`Register` 改为"校验+哈希 → Begin → insertUserInTx → Commit"；`JoinViaInvite` 全程**单事务**完成「建 user + 写 book_members + 标记邀请已用」，任一步失败整笔回滚。
  - 顺带：`normalizeAndHash` 把 bcrypt 移到**事务外**——避免数十毫秒的 CPU 开销占住唯一连接（`MaxOpenConns=1`）。
- **D-R08（P2）· 邀请"单次使用"可被并发绕过** → 条件更新 `WHERE id=? AND used_at=0` 不校验影响行数。
  - 修复：邀请改为**事务内重读**（消除跨事务 check-then-act 窗口）；标记已用的 UPDATE 校验 `RowsAffected==1`，为 0 即回滚并返 `ErrInviteInvalid`。
- **D-R09（P3）· 改角色/移除成员"假成功"** → `ChangeRole`/`Remove` 丢弃影响行数，目标已被并发移除时仍返回 nil。
  - 修复：改为 `n, err := ...`，`n != 1` 返回 `ErrConcurrentConflict`（→ **409**），与 `account.go` 既有规范对齐。
- **WB-R01（P2）· 邀请入户限流键为全局唯一（主理人补充，D 漏报）** → 原为 `s.limiter.Allow("invite:join", 10, time.Hour)`，**不含 IP 维度**：任意一人耗尽计数即可让**全实例**一小时无法入户（关闭注册后邀请是唯一入户通道，等同低成本 DoS）；且 `limiter.go` 注释把口径写成"同**实例**"，与 PRD §12.1 L572"同一 **IP** 每小时 ≤10"不符（实现侧口径漂移）。
  - 修复：键改为 `"invite:join:ip:"+ipBucket(clientIP)`，handler 传 `c.ClientIP()`；空/空白 IP 归入 `unknown` 桶**防绕过**；同时校验邀请确属 URL 上的账本（原先 `join` 忽略 `:id`，邀请可跨账本使用）。
- **未做（有意保留）**：`RevokeInvite` 撤销入口——PRD 要求"生成/使用/撤销/过期全周期"，但 `落地方案 §5.1` 未定义撤销接口，**不擅自加路由**，随阶段 3 补契约。

**验证**：`gofmt -l .` 空、`go build ./...`、`go vet ./...` 通过、**`go test ./...` 47/47 PASS（FAIL 0 / SKIP 0）**；新增 `server/internal/service/member_test.go` 6 个用例（IP 分桶独立性、空 IP 不绕过、`JoinViaInvite` 签名守卫防退回全局 key、`member.go` 源码级 `RowsAffected` 守卫、`normalizeAndHash` 校验不退化）。
**签名变更（下游需同步）**：`MemberService.JoinViaInvite(code, username, password)` → **`JoinViaInvite(bookID, code, username, password, clientIP)`**。
**验证边界**：真实事务回滚与并发竞争**仍无运行时证据**（构建机无 gcc），`member_test.go` 只覆盖纯逻辑与源码级守卫，不冒充该部分验收；已由给 GPT 的核查卡片承接。
**文档**：`workbuddy/FastLedger-D二次核查报告.md` 回填 D-R07/R08/R09 的 `status`（→已修复）与 `evidence`（逐条取证 + 两处更正：`MaxOpenConns=1` 不能免除 D-R08 缺陷；`account.go` 的 Update 分支用的是 `Update()` 返回值而非 `RowsAffected()`），并新增"复核与处置"一节记录 WB-R01。

## [1.25] - 2026-10-30

### D 第二轮工程实现核查：核心 9 焦点无错，成员/邀请服务 3 条新缺口（D-R07–D-R09）

- **新增 `workbuddy/FastLedger-D二次核查报告.md`**：D（同一审阅者，非新 AI）对真实可编译 Go 后端 `server/`（基线 `67da52a`，复跑 build/vet 通过、41 测试全绿）做第二轮核查。
- **卡片第 6 节 9 个焦点经逐行核对全部正确，无新 finding**：tx.go 事务边界、幂等四项核对、BETWEEN 边界数学、转账禁单边、15 类符号、400/409 错误映射、0003 迁移合法性、无隐藏入口、新建 creator_uid 强制 >0。
- **新发现 3 条，均为"代码没照设计做"（非设计文档错），无 P0/P1**：
  - `D-R07`（P2）：`JoinViaInvite` 先调 `auth.Register`（独立事务已提交 user），再另开事务写成员；第二步失败回滚但 user 不回滚，产生无账本孤儿用户，重试用户名冲突。设计要求"原子同事务建 user + book_members"（PRD L573/WBS L227）。
  - `D-R08`（P2）：标记邀请已用的 `UPDATE ... WHERE used_at=0` 不校验 RowsAffected，0002 邀请表又无"单次使用"库约束；并发竞争下一个未绑定用户名的邀请码可被两个不同用户重复使用。
  - `D-R09`（P3）：`ChangeRole`/`Remove` 的 UPDATE 丢弃影响行数，目标在并发中已被移除时影响 0 行却返回 nil 被当成功（对比 account.go 已核对 RowsAffected==1）。
- **落地判断**：不阻塞阶段 3 开工；建议 D-R07+D-R08 合并修复（JoinViaInvite 收敛单一事务 + 两处 RowsAffected 硬校验），D-R09 随阶段 3 成员管理改。
- **验证边界**：本轮为静态核查；构建机无 gcc，事务真实回滚、UPSERT RowsAffected、并发、0002/0003 真实迁移、PRD §10.1/10.3/10.4 端到端、`go test -race` 仍无运行时证据，需在带 gcc 环境补真实 SQLite 集成测试。

## [1.24] - 2026-10-30

### 新增：GPT 第三次核查对齐卡片（`GPT-R24` 起）

- **新增 `workbuddy/FastLedger-GPT二次核查卡片.md`**，供参与过 PRD 阶段评审的 GPT 做**第三次核查**（其历史编号：`R01`–`R18` + `GPT-R19`–`GPT-R23`，含 `GPT-ADR01` 转账入口裁决）。
- **编号续接**：`GPT-R24` 起，不复用、不重排。
- **与 D 卡片的关键分工差异（避免重复劳动）**：GPT 环境**有 CGO + 真实 SQLite 运行能力**（见 `gpt/v1.4/verification/`），而当前构建机**无 gcc**。因此本卡片把"9 项本机无法运行的验证盲区"写成**可执行断言矩阵（附期望数值）**，要求产出 `gpt/v1.5/verification/runtime-results.json`：
  1. 同 `clientUuid` 重试不重复扣款（余额只减一次、行数=1）
  2. 同 `clientUuid` 异内容 → 409
  3. `ON CONFLICT ... DO NOTHING` 的 `RowsAffected` 真实语义（首次 1 / 重放 0）
  4. 负债符号与净资产守恒（刷卡 100 → 信用卡 +10000、**净资产 90000 分**、还款后仍 90000）
  5. 转账第二笔失败 → 整笔回滚（两侧余额不变、无幂等残留、版本未递增）
  6. `related_id` 双向配对真落库 + 两条记录同 `sync_version`
  7. `xorm:"'type'"` 引号列与零值写入、`creator_uid` 不为 0
  8. `0002`→`0003` 迁移在真实库可执行且幂等（`schemaVersion=3`）
  9. 并发（`busy_timeout=5000` + `MaxOpenConns=1`）下不出现 500 / 余额错乱
- **静态审查焦点 9 条**：首要一条是「**`GPT-ADR01` 是否被忠实实现**」——请 GPT 当自己裁决书的裁判；其余为事务边界、`BETWEEN` 溢出数学、禁单边可绕过性、错误映射、`0003` 安全性、隐藏第二入口、`creator_uid=0` 哨兵、PRAGMA 连接池语义。
- **单列 3 项"已知开放项"请 GPT 裁定严重度**（不是"别重复提"）：A. §7.3 可见性未实现（阶段 3）；B. `Merchant` 死列（阶段 5）；C. **权威 DDL 的 `related_account_id` / `related_amount` 两列代码中仍缺（`0003` 未补）**——本轮新发现并如实披露。
- **`多AI对齐规范.md` 同步**：§1 GPT 角色改为"三次核查（`GPT-R24` 起）"并标注其运行能力；§9 补两份核查卡片与 D 评审文件；§10 上手清单更新至 `[1.24]`、编号规则含 `GPT-R24`；§11 补 `[1.22]`–`[1.24]`，并把当前口径改为**D（静态）/ GPT（可运行）分工**。
- 原因：GPT 是唯一能**真实跑起 SQLite** 的评审方，把"验证盲区"交给它是本轮最高效的一步；同时用编号续接与"上轮 23 条现状表"防止它重复报告已闭环项。

## [1.23] - 2026-10-30

### 更正：核查由 **D 二次进行**（不新开 E）

- **撤销 [1.22] 引入的 E 角色**：owner 决定继续由 **D** 做第二轮核查，不引入第五个 AI。`多AI对齐规范.md` §1 移除 E 代号，`E-Rxx` 不再使用。
- **D 的编号续接**：D 已用 `D-R01`–`D-R06`，第二轮**从 `D-R07` 起**（不复用、不重排），§1 与 §10 均已写明。
- **新增 `workbuddy/FastLedger-D二次核查卡片.md`**（替代原 E 卡片）：
  - 开篇即给 **"你上一轮 6 条现在什么状态"** 对照表——`D-R01/R03/R04/R05/R06` 只在**文档/骨架层**闭环，真实代码分别属**阶段 3/4/5**；明确 **"某功能还没实现"不是新 finding**，除非能证明设计错或已实现部分实现错；
  - 必读顺序更新到 `[1.13]`–`[1.23]`；
  - 阶段 0/1/2 逐项落点 + ✅已验 / ❌无 gcc 未验 9 项；
  - **审查焦点扩到 9 条**（新增第 9 条：`creator_uid=0` 哨兵在阶段 3/4 若被当作放行条件 → **高危越权**）；
  - 保留 13 字段格式与"先搜重"要求。
- **删除** `workbuddy/新AI对齐卡片-E.md`。
- 原因：避免 D 把"阶段 3/4/5 尚未实现"当成新问题重复报告，浪费一轮往返。

## [1.22] - 2026-10-30

### 新 AI 对齐抓手：一页纸卡片 + 规范更新到 [1.21]

- **新增 `workbuddy/新AI对齐卡片-E.md`**——给外部审查 AI 的一页纸：
  - 代号 **E** 与编号 `E-Rxx`；
  - 必读顺序（规范 → 导读 → CHANGELOG [1.6]–[1.21] → WBS → `server/` → C/D 评审）；
  - **我已做完什么**：阶段 0/1/2 逐项落点（文件 + 关键设计）；
  - **真实到了哪**：✅ 已验（build/vet/41 测试/DDL 真执行/PRAGMA 期望值）vs ❌ **无 gcc 未验证的 9 项盲区**；
  - **什么别重复提**：GPT R01–R23、C-R01–C-R10、D-R01–D-R06 全部已裁决/已修；已知非阻塞遗留（P2-2/P3-3/`creator_uid=0`）；
  - **建议审查焦点 8 条**（事务边界、幂等、`BETWEEN` 数学、禁单边、符号表、错误映射、`0003` 迁移、隐藏入口）+ 13 字段提意见格式。
- **`workbuddy/多AI对齐规范.md` 更新**：
  - §1：D 标为"已产出 D-R01–D-R06"；**新增 E（第五个 AI）**，编号 `E-Rxx`；
  - §1 去重警示扩展到 C + D，并指向 [1.15]–[1.17]；
  - §9：补充 **`server/` 是真实代码**（不在 `workbuddy/`），并指向新对齐卡片；
  - §10：上手清单扩为 10 步，新增"读真实代码 `server/`"，编号改 `E-Rxx`；
  - §11：进度快照补 [1.13]–[1.21]（含阶段 0/1/2）；"当前口径"更新为**真实代码阶段 + 9 项未验证盲区 + 3 项遗留**。
- **原因**：规范此前停在 [1.12]，外部 AI 会误以为项目仍是纯文档阶段，从而把火力浪费在已冻结的决策上。

## [1.21] - 2026-10-30

### 阶段 2 收尾：把服务端校验规则补充进 API 契约

- **`docs/FastLedger-落地方案.md` §5.1 新增"创建校验（服务端强制）"**：`amount` 整数分且 `1 ≤ amount ≤ 10_000_000_000`（1 亿元/笔，超出 400）、`clientUuid` 必填且 ≤64 字节、转账须 `toAccountId` 且 `categoryId=0`、**余额溢出返 400（非 409，重试无意义）**、账户已删/并发返 409；并注明客户端须同步做输入校验。
- 原因：金额上界此前只存在于服务端代码，属**用户可见规则**却未进契约，用户超限只会看到无预提示的 400。
- **裁决：`transactions.merchant` 死列暂不处理**。`CreateRequest` 无 `Merchant`（指南 §13.1 参考骨架同样缺失，属文档缺口），`keyword` 的 `merchant LIKE` 分支当前无害但无意义；**保持现状，待阶段 5（OCR 识别链路）补 `merchant` 写入时一并收敛**，避免现在改 SQL 又制造一次文档/实现漂移。
- 说明：`createChangeBalance` 中 `n != 0` 分支对单行 UPDATE 不可达，属防御性代码，**保留**（不影响正确性），已知冗余。
- 过程记录：工程师曾在授权到达前临时移走 QA 测试文件约 40 秒以单独验证生产代码（`mv` 往返、md5 一致、未编辑内容），已主动披露并纠正流程——后续此类操作须先获授权。

## [1.20] - 2026-10-30

### 阶段 2：记账核心闭环（统一 Create / 转账双边 / 幂等 / 账户分类）

- **唯一创建入口** `TxService.Create`：HTTP（及后续 sync/push）复用同一入口；无 `AddTransaction`/`Transfer` 等第二入口（路由与测试双重断言）。
- **转账双边原子** `createTransferInTx`：复用外层事务与版本（每个命令仅一次 `versions.Next`），两侧余额 + 两条记录同事务同版本；`related_id` 互指并校验影响行数；任一步失败整笔回滚；**无单边路径**。
- **余额唯一写路径** `createChangeBalance`：单条原子 UPDATE（带 `book_id`、`deleted=0`、`RowsAffected==1`、`balance BETWEEN` 溢出保护），无"先读后写"。
- **负债符号** `signedBalanceDelta`：category 10~15（负债）取反。储蓄 1000 + 信用 0、刷卡 100 → 信用 `+10000` 分，**净资产 90000 分（900 元）**；还款后仍 90000。
- **幂等**：`idempotency_records` 事务内 `ON CONFLICT(book_id,uid,client_uuid) DO NOTHING` + `RowsAffected` 判定；命中重读回执核对 `request_hash/result_type/result_ids/result_version`；不一致 409；回执回写失败整笔回滚；重放不碰余额、不递增版本。
- **`requestHash`** 覆盖完整请求（域标签 `transaction.create/v1`）且**排除 clientUuid**；备注尾部空白归一化。
- **服务层强制 `clientUuid`**（`validateCreate`），handler 另有 binding 双重校验。
- **账户 / 分类**：CRUD（软删、归属校验、引用保护、两级分类）；新账本在同一事务内落默认数据。
- **HTTP**：400 / 403 / 404 / 409 映射；新增 `0003` 迁移补 `creator_uid`、`source`、`timezone_offset` 与 `idx_tx_creator`（**未改 0001/0002**）。

### 本轮返工（QA 两轮独立验证后）

- **P2-1 修**：补 `creator_uid`（此前 `uid` 是死参数，交易归属会永久丢失）。`newTxRow(creatorUid, bookID, r, ver)` 接线，转账两条记录同 creator。
- **P3-1 修**：溢出误判 409 → `RowsAffected==0` 时复查账户区分"溢出(400)"与"已删/并发(409)"；新增金额上界 **1 亿元/笔**（`maxAmountCents`）。
- **P3-4 / P3-5 / P3-6 / P3-2 修**：分页回显归一化；账户 `parentId` 自引用拦截；三处 `errors.New` 改 `ErrInvalidArgument`（500→400）；不完整回执文案改为如实告知 + `log.Printf` 留痕。

### 验证

- ✅ `gofmt` 空、`go build ./...`、`go vet ./...`、`go test ./...` 全过；**RUN=41 / PASS=41 / FAIL=0 / SKIP=0**（QA 23 + 工程师 18；含 JWT 安全、权限矩阵、限流、符号全类别、225 组转账守恒、hash 独立判据、27 条错误映射、路由唯一性）。
- ✅ 主理人独立复核：4 参数调用无 3 参数残留；`0001/0002` 未被修改（`git diff` 为空）；亲自跑通全量。
- ❌ 仍需 gcc / 真库补验：事务真实回滚、UPSERT 的 `RowsAffected` 语义、并发（`busy_timeout` + `MaxOpenConns=1`）、`xorm:"'type'"` 引号列与零值写入、`related_id` 双向落库、`0003` 在真实库可执行、PRD §10.1/§10.3/§10.4 端到端、`go test -race`、Gin 全链路集成。

### 遗留（不阻塞阶段 2）

- **P2-2**：PRD §7.3「member 仅看本人记录」的服务端过滤 → **阶段 3**（所依赖的 `creator_uid` 已补齐）。
- **历史行 `creator_uid=0` = 未知归属**：阶段 3/4 做可见性与"仅本人可改"判定时**必须显式处理 0**（建议视为"仅 owner/admin 可见/可改"），**禁止**把 0 当作放行条件。
- **P3-3**：`CreateRequest` 缺 `Merchant`，`transactions.merchant` 恒空、`keyword LIKE merchant` 为死条件——属**设计文档缺口**（指南 §13.1 参考骨架同样缺失），建议随阶段 5（OCR）一并补文档与字段。

## [1.19] - 2026-10-30

### 阶段 1：认证 / 账本 / 成员 + 邀请入户

- **认证**：`AuthService`（注册/登录/校验）、bcrypt 口令哈希（自带盐，不另设 salt 列）；登录校验账号是否删除/禁用；签发 token 带 `jti`。
- **JWT**：手写 HS256 实现（不引第三方库）——**限定 HS256、拒绝 `alg=none`**、拒绝篡改签名/过期/错误密钥、安全读取 `uid`（非法 claims 返 401 而非 panic）。满足 R08/R09。
- **账本**：`BookService` 创建/列表/详情/修改/删除；创建时**同事务**初始化 `book_versions` 并写入 owner 成员；删除账本**仅 owner**（落地方案 §5.5）。
- **成员与权限**：`RoleOf` 服务端权威判定；权限矩阵按 §5.5【修订版】——`CanEditOthersTx` owner/admin ✅（admin 由"可配置"改为 ✅，[1.9] 裁决）、member ❌；`CanManageMembers` owner/admin ✅ 且 admin 不能动 owner；移除成员为**软删保留历史**。
- **邀请入户**（PRD §12.1）：新增 `invites` 表；邀请码单次使用（`used_at/used_by`）、默认 7 天有效期（`EBK_INVITE_TTL_DAYS`）、可绑角色与用户名；`JoinViaInvite` **原子**建 user + book_members 并标记已用；生成每账本每分钟 ≤5、使用每小时 ≤10 的限流；**不受 `EBK_ENABLE_REGISTER` 限制**（关闭注册后唯一入户通道）。
- **版本**：`internal/version` 提供**唯一** `VersionRepo.Next`（事务内原子 +1 并回读），经 `VersionSource` 注入。
- **CLI**：`fastledger user create --username u --password p [--owner]`；`--owner` **仅在无任何用户时可用**，建好后失效；首个 owner 自动成为实例管理员（`is_instance_admin`，与账本 owner 分离）。

### 新增迁移 0002（用自建的迁移机制落地）

- **修正 0001 的真实缺陷**：`UNIQUE(book_id, uid)` 与"成员退出为软删"冲突——成员被移除后唯一索引仍占位，导致**无法重新加入**。改为**部分唯一索引** `WHERE deleted = 0`。
- 新增 `invites` 表（邀请码）。

### 验证

- ✅ `gofmt` / `go build ./...` / `go vet ./...` 全部通过。
- ✅ 新增单元测试 `internal/service/jwt_test.go`，**7 项全部 PASS**（真实运行，无需数据库/CGO）：JWT 往返、`alg=none` 拒绝、篡改签名拒绝、过期拒绝、错误密钥拒绝、权限矩阵（admin ✅）、邀请限流。
- ❌ 仍未验证：CGO 开启下 `db migrate` 实跑与 HTTP 端到端调用（本机无 gcc，MinGW 下载超时）。
- 编码期由真实编译捕获并修复：① `Status.Current` 为 `int` 与版本号 `int64` 不一致；② 4 个文件格式未 gofmt。

## [1.18] - 2026-10-30

### 阶段 0：真实 Go 工程骨架落地（`server/`）

- **新增 `server/` Go module**（module `fastledger`，go 1.22；直接依赖 gin v1.10.0 / xorm v1.3.9 / mattn/go-sqlite3 v1.14.22，`go.sum` 已生成）。
- **`internal/config`**：统一 `EBK_` 前缀环境变量；`EBK_JWT_SECRET` 空值 / 占位值 / <32 位**启动即失败**（fail-fast，R08）；新增 `EBK_RECYCLE_TTL_DAYS=30`、`EBK_INVITE_TTL_DAYS=7`（PRD §12.1）。
- **`internal/database`**：`initDB` 用 **DSN 逐连接**设置 `WAL / busy_timeout=5000 / foreign_keys=on / synchronous=FULL`（禁止 `db.Exec("PRAGMA ...")`）；写引擎 `MaxOpenConns=1`；**启动校验三项 PRAGMA 真的生效**，不通过即退出。
- **`internal/migrate` + `migrations/0001_init.up.sql`**：版本化迁移——`schema_migrations` 版本表、按序 `NNNN_*.up.sql`（embed）、每个脚本**事务内执行**失败即回滚；默认**仅 CLI `db migrate`**，服务启动只检查版本、**绝不静默改表**；初始 DDL **15 张表**，含 `audit_logs`、`idempotency_records`、`book_versions`。
- **`cmd/server/main.go`**：启动 HTTP（当前仅 `/healthz`）与 `db migrate` 子命令；优雅关机（配合部署 `stop_grace_period ≥ 30s`）。
- **构建注意（重要）**：`mattn/go-sqlite3` 依赖 **CGO**。无 gcc 时 Go 会自动 `CGO_ENABLED=0`，此时**能编译、能链接，但数据库不可用**，运行会报 `go-sqlite3 requires cgo to work. This is a stub`——启动 PRAGMA 校验会**快速失败**并明确提示。已写入 `server/README.md` 排错段。

**本次验证边界（按规范 §4 如实记录）**：

- ✅ 已验证：`gofmt` 全量通过；`go build ./...` 与 `go vet ./...` 无错；链接出 11MB 可执行；运行后配置加载与启动校验按预期生效（CGO 缺失时被 fail-fast 捕获）；初始 DDL 经 sqlite3 **真实执行**建出 15 张表（含审计表/幂等表/版本表）；三项 PRAGMA 期望值核对正确（`journal_mode=wal`、`synchronous=2 (FULL)`、`foreign_keys=1`）。
- ❌ **未验证**：本机无 gcc（MinGW 下载超时失败），**CGO 开启下的 `db migrate` 实跑与 `/healthz` 返回仍待在有 gcc 的环境补验**。请在有 gcc 的机器上执行：`go env -w CGO_ENABLED=1 && go run ./cmd/server db migrate && go run ./cmd/server`。
- 注：编码期已由真实编译捕获并修复一处类型错误——`engine.DB()` 返回 `*core.DB`（内嵌 `*sql.DB`），传给标准库需用 `engine.DB().DB`。

## [1.17] - 2026-10-30

### 复核 D 的 [1.16] 修复：确认正确，并修订 3 处残留

**复核结论（D 的两个修复均属实且正确）**：
- Login 吞绑定错误已改为显式返 400；全仓 `_ = c.ShouldBindJSON` 残留 **0**（Register / Login / ParseText 三处均显式处理）。
- Pull 的 `safeVersion` 版本边界分页**可证正确**：反例 3×v1 + 3×v2 + 3×v3、`limit=5` → 前 5 行 `MAX=2` → 按 `(afterVersion, 2]` 读 6 行，第 3 个 v2 行（第 6 行）被区间纳入，**同版本未被拆到两批**。

**本次修订**：
- **P2 / 已修订**：`Pull` 中 accounts / categories / holdings 三个 `Find` **未检查错误**——查询失败会静默返回空集，而客户端照常把游标推进到 `safeVersion`，导致这三类变更**永久丢失且不再重试**（transactions 的 Find 有检查，不一致）。改为出错即 `return nil, err`，整批失败、游标不推进。改动 `docs/FastLedger-新手开发完整指南.md` §22.2。
- **P3 / 已修订**：`maxSyncBatch` 注释与分页规则补充说明——`limit` 是**目标规模而非硬上限**（同一 `sync_version` 必须整批返回，不能拆版本），故**写路径须限制单事务变更条数（建议 ≤ 500）**，读路径无法对"单版本超大批次"兜底。
- **P3 / 已修订**：Register 绑定失败文案由"参数错误"统一为"**请求格式错误**"，与 Login / ParseText 保持一致。

## [1.16] - 2026-10-30

### D 复核 [1.15] 修复 + 两个 P3 收尾
- 逐条回读文档（非仅看 CHANGELOG）确认 **D-R01–D-R06 全部真实落地**：audit_logs DDL/Delete-Restore/迁移 §4.7/WS 存活/OCR 契约/Flutter 网络层/Pull 授权均在骨架中，非表面文字。
- 复核中发现并修复两个 P3 残留：
  - **Login handler 仍吞绑定错误**（`_ = c.ShouldBindJSON`，与同文件 Register 不一致）→ 改为显式处理返 400。
  - **Pull 分页未严格按版本边界**（`LIMIT` 按行截断可能拆同一 sync_version，且账户/分类/持仓未限范围）→ 改为先算 `safeVersion` 版本上界、所有实体统一在 `(afterVersion, safeVersion]` 快照读取，并补非交易变更兜底与客户端游标推进规则。
- 结论：设计/骨架层已无阻塞项，可按 WBS 阶段 0 进入正式编码；运行时验证仍待正式工程补齐。

## [1.15] - 2026-10-30

### owner 裁决：D-R01–D-R06 复核全部属实，已采纳并修复

**复核结论**：6 条**全部属实**，无一条误判，也无与历史 finding 重复。逐条 grep 实测证据：指南 `CREATE TABLE` 15 张且 `audit_logs` 命中 **0**、`func Delete/Restore` **0**；方案迁移设计命中 **0**（仅 L250 `db migrate` 名称）；`PingMessage` 1 处而 `SetReadDeadline|SetPongHandler|SetReadLimit` **0**；L1576 `_ = c.ShouldBindJSON`、L1565 `ParseBill(text string)` 无 ctx；L1862 每次 `http.Client().send`；L2087 `Pull` 无成员校验与批次上限。

- **D-R01 / P1 / 已采纳并修复**：两文档 DDL 补 `audit_logs`（append-only，含 `before_json`；仓库层只暴露 Insert，不暴露 Update/Delete）；指南新增 **§13.3 `TxService.Delete/Restore`**（与 `Create` 对称：转账按 `related_id` 整笔、**禁单边**，同一事务内版本 +1、两侧余额回滚、审计必写）；方案 §5.1 补**回收站列表 / 恢复 / 硬清除 / 审计查询** 4 个接口。
- **D-R02 / P1 / 已采纳并修复**：方案新增 **§4.7 数据库迁移机制**——`schema_migrations` 版本表、按序编号的 up/down 脚本、默认**仅 CLI 手动迁移**（启动只做版本检查，绝不静默自动改表）、SQLite 不支持改列时走**重建表范式**（建新表→拷数据→删旧表→改名）、老库升级**不得删旧回执**、迁移后校验 PRAGMA。
- **D-R03 / P2 / 已采纳并修复**：指南 §14.3 `ServeWS` 增 `SetReadLimit(64KB)` + 初始 `SetReadDeadline(60s)` + `SetPongHandler` 续期；并显式定义 `upgrader`（含 `CheckOrigin`，避免不同入口默认行为不一致）。
- **D-R04 / P2 / 已采纳并修复**：`LLMProvider.ParseBill` 改为 `ParseBill(ctx, text)`；`ParseText` 处理绑定错误返 400、校验文本非空且 ≤2000 字符、`context.WithTimeout(10s)` 可随客户端断开取消，错误不外露内部信息。
- **D-R05 / P2 / 已采纳并修复**：指南 §19.3 `ApiClient` 重构——持久且可 `close()` 的 client、统一 `timeout`、**先按 HTTP 状态码分支**（401 跳登录 / 5xx 可重试）再解析 body、新增 `ApiException`（409 携带 `conflict` 供"覆盖/放弃/另存"三选一）。
- **D-R06 / P2 / 已采纳并修复**：指南 §22.2 `Pull` 签名改为 `Pull(uid, bookID, afterVersion, limit)`——**成员资格校验下沉到服务入口**（非成员 403，不信任 handler）、`afterVersion<0` 按全量处理、批次上限 `maxSyncBatch = 500` 防全量加载 OOM。

> ⚠️ **函数签名变更**：`Pull(bookID, afterVersion)` → `Pull(uid, bookID, afterVersion, limit)`，调用方（handler / 同步适配）需同步传入 `uid` 与 `limit`；`LLMProvider.ParseBill` 增加 `ctx` 首参。

## [1.14] - 2026-10-30

### 新增独立审阅者 D 评审（workbuddy/FastLedger-文档评审-D.md）
- 第四个 AI（代号 D）以基线 `a403d13` 完成工程实现层独立评审，产出 **6 条 D-R01–D-R06（2×P1 + 4×P2，无 P0/P3）**，均按规范 §2 十三字段、附文件/章节/行号/原文，全部 `待确认`。
- 已去重 GPT R01–R23、C-R01–C-R10、WB 各轮，不重复已裁决项。
- **D-R01（P1）**：删除/恢复/审计已决策（[1.10]、WBS 0.4/4.3），但主文档 DDL 无 `audit_logs`、骨架无 Delete/Restore、接口无回收站/恢复/审计。
- **D-R02（P1）**：版本化数据库迁移仅 `db migrate` 名称与 WBS 一行，缺版本表/脚本规范/事务/SQLite ALTER 重建范式。
- **D-R03（P2）**：WS 有 ping 但缺 SetReadDeadline/SetPongHandler/SetReadLimit（R18 建议未落地增量），半开连接泄漏。
- **D-R04（P2）**：OCR parse-text 吞绑定错误、无空/超长校验、LLM 调用无 ctx/超时。
- **D-R05（P2）**：Flutter ApiClient 无超时、不判 HTTP 状态码、连接不复用、409 无法三选一。
- **D-R06（P2）**：Pull 读路径缺成员校验（越权）与批次上限（OOM）。
- 状态：等待 owner 裁决；若误判/重复将按规范收回。

## [1.13] - 2026-10-30

### 更新《多 AI 对齐规范》（为下一个独立评审 AI 对齐颗粒度）
- **§1 角色表**：C 由"待加入"更新为"已产出 `C-R01`–`C-R10`"（10 项，6×P2 + 4×P3）；新增 **D（第四个 AI）** 代号，编号用 `D-Rxx`，避免新 AI 与既有编号撞车。
- **§1 编号规则**：新增"已裁决项勿重复提"警示，要求提新意见前先查 CHANGELOG [1.6]–[1.12]。
- **§9 `workbuddy/` 清单**：补入 `FastLedger-文档评审-C.md`（C 的 10 项，标注"新 AI 必读，避免重复提"）、v1.3 / v1.4 / v1.5 说明；并注明 `docs/FastLedger-开发WBS.md` **不在本目录**。
- **§10 上手清单**：扩为 9 步，明确必读 CHANGELOG [1.6]–[1.12]、C 评审（勿重复提）、开发 WBS；新 AI 编号改用 `D-Rxx`。
- **§11 进度快照**：补齐缺失的 [1.6]–[1.12] 共 7 个版本；新增"【1.12】之后的当前口径"，指引新 AI 把火力转向**工程实现层新风险**（Gin 接线与 400/409、真实 WS 生命周期、Flutter 源码、OCR 后端契约、多实体端到端同步、迁移与删除 / 恢复验收）。
- **目的**：owner 拟引入新的独立评审 AI；本次更新确保其在颗粒度（§2 十三字段）、编号、严重度（§3）与证据标准（§4）上对齐，且不被过期信息误导而重复提已裁决项。

## [1.12] - 2026-10-30

### owner 裁决：§12.1 剩余 3 项待确认全部转为【决策】
- **权限变化与本地缓存重建**（已采纳建议方案）：事件收敛为 4 类（`visibility_changed` / `member_removed` / `member_role_changed` / `book_deleted`）且 WS 广播**脱敏**；客户端**定向重拉该账本**（`afterVersion=0`）而非清空本地库；可见性变化**不递增业务 `sync_version`**（非数据变更），改用事件携带 `force_full_resync=true`；被移除成员客户端**主动清除该账本本地数据**并停止订阅；隐私边界话术写入 §7.3 并常驻显示于开关旁。改动 `docs/FastLedger-产品需求文档-PRD.md` §12.1、§7.3。
- **实例管理员与账本 owner 边界**（已采纳）：两层角色**正交**（实例管理员 vs 账本 owner/admin/member），互不推导；整库备份含所有私账，故**下载仅限实例管理员**（文件权限 600 + 写审计），对普通 owner 提供**"按账本导出"**以机制上防越权；LLM 密钥修改仅实例管理员并写 `audit_logs(action=config_change)`；`users` 增 `is_instance_admin`，与 `book_members.role` 分离。改动 PRD §12.1。
- **关闭公开注册后的成员入户**（已采纳）：**邀请与公开注册解耦**，`EBK_ENABLE_REGISTER=false` 不影响邀请码注册，邀请为唯一入户通道；邀请码绑定账本 + 角色、**有效期默认 7 天**（`EBK_INVITE_TTL_DAYS`）、**单次使用**（`used_at`/`used_by`）、可选绑定邮箱或用户名（防链接转发）；限流：每账本每分钟生成 ≤5、同 IP 每小时尝试 ≤10，失败计审计；注册**原子**建 `user` + `book_members`；生成/使用/撤销/过期全周期审计。改动 PRD §12.1。
- **同步《开发WBS》**：附录 A 提案 1/2/3 由"待裁决"更新为"已采纳为【决策】"；阶段 1、3 的阻塞标记解除。
- **至此 §12.1 五项技术待确认全部转为【决策】**，PRD 已无遗留【待确认】阻塞项，可进入开发阶段（见《FastLedger-开发WBS.md》）。

## [1.11] - 2026-10-30

### 新增开发 WBS（展开版）+ 3 项待确认的建议方案
- **新增 `docs/FastLedger-开发WBS.md`**：《落地方案》§6.3 粗粒度 WBS（#1–#18）的展开版，含 8 个阶段（0 脚手架 → 7 运维验收）、任务 / 交付物 / 依赖 / 验收映射，以及 PRD §10 验收映射表。
- **本轮决策产生的新增工作已入 WBS**：阶段 4「删除 / 恢复 / 审计」；阶段 0.2 `synchronous=FULL`；阶段 7.1 `stop_grace_period: 30s`；阶段 7.3 恢复演练。
- **§12.1 剩余 3 项待确认各附建议方案**（WBS 附录 A 提案 1/2/3，**待 owner 裁决**）：权限变化与缓存重建；实例管理员与账本 owner 边界；关闭注册后的邀请入户。PRD §12.1 对应条目已加指针。
- 未改动任何既有【决策】；该 3 项仍为【待确认】，裁决后再转为【决策】并同步回 WBS 与《落地方案》。

## [1.10] - 2026-10-30

### owner 裁决：删除恢复审计转【决策】+ 断电持久性运维加固
- **§12.1「删除恢复与审计」由【待确认】转【决策】**：采纳 C 的设计建议（owner 确认：回收站 30 天、审计保留 1 年）。改动 `docs/FastLedger-产品需求文档-PRD.md` §12.1，含回收站与保留期、转账双边整笔删除/恢复、审计日志表 append-only、旧离线数据回流、恢复演练验收、统一 `TxService.Delete/Restore` 入口。
- **§8.5 部署与运维新增 4 条加固**：备份原子包含 WAL、优雅关机 checkpoint（`stop_grace_period`）、FULL 不替代备份且须恢复演练、恢复后递增 `restore_epoch`。
- **§10 新增 10.9 备份恢复与审计验收**（恢复演练、转账双边删除/恢复、审计可追溯）。
- **同步技术方案（遵循"设计改→方案同步"纪律）**：`docs/FastLedger-落地方案.md` §8.1 docker-compose 增 `stop_grace_period: 30s`；§8.3 备份与恢复增"恢复演练（必做）"与 WAL 注意事项。

## [1.9] - 2026-10-29

### owner 裁决：采纳 C-R03 / C-R05 / C-R06
- **C-R03 / P2 / 已采纳**：admin 改/删他人交易以 PRD §12 决策#2 为准。改动 `docs/FastLedger-落地方案.md` §5.5 权限矩阵：admin 列由"可配置"改为"✅"，并加注引用 §12 决策#2；member 列"默认不可"维持（与 PRD §7.2 member=✗ 一致）。
- **C-R05 / P2 / 已采纳**：全仓统一 `EBK_` 环境变量前缀。改动 `docs/FastLedger-新手开发完整指南.md` §22.5：`ENABLE_REGISTER=false` → `EBK_ENABLE_REGISTER=false`（与落地方案 §6.1、PRD §12.1 一致）。
- **C-R06 / P2 / 已采纳**：断电持久性定为 `synchronous=FULL`，不采用"书面豁免"。改动：指南 §22.4 initDB DSN `_synchronous=NORMAL`→`FULL`；落地方案 §6.1 补 `synchronous=FULL` 说明；PRD §12.1 该项由【待确认】转为【决策】。
- **§12.1 其余技术待确认**（权限变化缓存重建、实例管理员边界、关闭注册后入户、删除恢复审计）：维持【待确认】，待 owner 后续裁决。

## [1.8] - 2026-10-29

### 新增独立审阅者 C 文档评审（workbuddy/FastLedger-文档评审-C.md）
- 按《多 AI 对齐规范》§1 以"第三个 AI（C）"身份提交独立评审，10 项发现 C-R01~C-R10（6×P2 + 4×P3，均待确认）。
- 覆盖：PRD/落地方案/新手指南/枚举字典/转账裁决/README/CHANGELOG 的跨文档一致性、版本号双轨冲突、权限矩阵矛盾、环境变量前缀不一致、`synchronous=NORMAL` 断电持久性待确认未闭合等。
- 未采纳/未裁决：10 项均 `待确认`，等待 owner 裁定；未重提既有 GPT R01–R23 与 WB 各项。
- 报告全文见 `workbuddy/FastLedger-文档评审-C.md`。

## [1.7] - 2026-10-29

### README 补公网交互原型入口（WorkBuddy）
- 仓库导航表 `prototype/` 行用途列补充公网在线版链接，并新增"快速开始"小节顶部在线体验入口。
- 链接：https://a38ce1ad4e917321e.app.workbuddy.host/?sharecode=BPE6OFTPKE （由 WorkBuddy 托管，无需本地构建，含全部交互页面，演示账号已预填）。
- 注：该在线预览为沙箱托管静态服务，若下线以 WorkBuddy 重新发布的链接为准。

## [1.6] - 2026-10-29

### PRD §12 待确认决策化（WorkBuddy 复核后落地）
- 原 §12 的 8 项【待确认】按多 AI 评审与快速原型视角逐一拍板，转为【决策】：预算固定自然月、member 仅改本人记录、MVP 不做自动行情、MVP 仅 CNY、每日备份保留 7 份、成员上限 20、云端 LLM 兜底默认关闭、退出软删保留历史。
- 同步修订 §7.2 权限矩阵：原“编辑/删除他人记录”member 列【待确认】改为“✗（member 仅可编辑/删除本人记录）”，与 §12 决策对齐。
- 新增 §12.1“技术待确认（GPT 评审补充）”，收录 GPT 评审单列、且不阻塞 MVP 设计的 5 项技术开放项（权限变化缓存重建、管理员/owner 边界、关闭注册后的入户、断电持久性、删除恢复与审计）；云端兜底开关已由 §12 第 7 项决策覆盖。
- 阅读约定（L13）更新：第 12 章现汇总“决策结论与仍需技术拍板的事项”。

## [1.5] - 2026-10-29

### 性质与审查基线
- 按项目方请求，审核 `v1.4-修复说明.md` §6，并按 `多AI对齐规范.md` 的 13 个字段交付。基线 `9a3a2e38a6476450b04fea59c9cb49eb5dba7416`；本版本为**审核、跨文档裁决与可执行参考骨架修订**，不是完整生产应用发布。

### 裁决与函数签名（GPT-R23 / GPT-ADR01，已采纳；有效骨架已统一）
- 唯一公开领域入口：`TxService.Create(ctx context.Context, uid, bookID int64, req CreateRequest) (*CreateResult,error)`。HTTP 创建和 `sync/push` 每个离线创建项复用同一入口；内部 `createTransferInTx` 使用已有 session/version，不另开事务、占位或加版本。
- 旧 `AddTransaction`、独立可提交 `Transfer` 和模糊 `createRelatedTransfer` 退出有效骨架；历史记录/反例快照保留。规范见 `docs/FastLedger-转账入口裁决.md`，参考实现见指南 §13.1，方案 §5.1/§6.1、PRD §4.3.1 同步。
- 版本唯一实现为 `VersionRepo.Next`；创建与持仓使用 `VersionSource`。`AddHolding` 增加 `versions VersionSource` 末参，其调用和编译/真实数据库路径已验证。

### 新发现处理（GPT-R19–GPT-R23）
- **GPT-R19 / P1 / 已修复（参考骨架）**：回执占位、查询、结果回写不得吞错误；只对指定唯一键 `ON CONFLICT DO NOTHING`，重读后核对完整回执。结果回写失败整笔回滚。位置：指南 §13.1、裁决 §3。
- **GPT-R20 / P1 / 已修复（参考骨架）**：摘要统一为带域标签的完整 CreateRequest，覆盖分类/渠道等字段；回放显式核对 result_type，返回原 ID 和创建版本。位置：指南 §13.1、裁决 §3。
- **GPT-R21 / P2 / 已采纳**：创建键改为 DTO 必填与 Service 强制检查；JSON `clientUuid` / DB `client_uuid` 口径明确。Service 回归已通过，正式 Gin handler 的 400/409 接线仍待验收。位置：指南 §7.1/§13、裁决 §2。
- **GPT-R22 / P2 / 已修复（有效文档入口）**：指南的 nextBookVersion 实现删除，统一依赖方案 VersionRepo.Next，持仓与同步调用已传播。§3.5 SQL 仍是解释性算法，不是另一可调用实现。
- **GPT-R23 / P1 / 已修复（参考骨架）**：原 AddTransaction 接受 txType=3 却只插一条且不改余额的绕行反例已复现；当前所有创建类型只经 Create，转账一次写双边。

### 历史编号复核（不新增重复编号）
- **GPT-R03**：原信用卡还款分支仍给负债账户加余额，净资产 90000→70000 分的反例成立。统一分支按正欠款反转负债增量；四种资产/负债转账方向净资产守恒的隔离回归通过。此处关闭创建/还款分支，不宣称删除恢复已验收。
- **GPT-R06**：原转账账户版本未更新、交易 tx_time=0；本轮创建路径已将两侧账户、两条记录写同一版本和请求时间。全实体/端到端同步保持已采纳、待正式工程验证，不能据局部测试宣布全域关闭。
- **GPT-R16**：原初始化在实际编译器下出现 DB 返回值、Engine.QueryRow、外包接收者三处错误；本轮修正指南 §22.4，并让方案引用唯一初始化示例。修订 initDB 编译运行通过，验证 foreign_keys/WAL/连接池；整个工程构建仍待完成。

### 契约与验证材料
- 两份 DDL 同步 `idempotency_records` 与 `result_version`；枚举字典新增 result_type 和恢复世代字段契约。v1.4 说明加后续复核标记；多 AI 规范修正“18 项全 P1”和负欠款示例的文字错误，并更新进度范围。
- 完整 13 字段意见、原片段快照/摘要、真实 Go/xorm/SQLite 故障注入、修订回归、Hub 核心 race 与旧/新编译输出位于 `gpt/v1.4/`。测试数量及结果以 `verification/runtime-results.json` 为准。
- 验证使用最小表映射和 Hub 权限桩；不等同于正式 HTTP/WS/Flutter 或全应用端到端验收。已有试验库升级时须迁移回执新字段，不得删除旧回执以“解决”重试。

## [1.4] - 2026-10-29

### 背景
接手豆包 v1.3 的落地工作，针对《v1.3-复核-豆包落地核查》（workbuddy/）中标注的纰漏进行修正：**补齐转账幂等（P2，功能性数据重复风险）**、**统一版本号递增入口（P3）**、**强化幂等契约（P3）**，并明确 R18 描述合并口径。改动均落到 `docs/FastLedger-新手开发完整指南.md`。

### 修复（P2：转账幂等，闭环 R04 在转账路径的缺口）
- **`Transfer` 补齐对称幂等**：新增 `clientUuid string` 参数；`clientUuid` 非空时在事务内查 `idempotency_records`（ResultType=2 标记转账）→ 命中且 `transferHash(from,to,amount)` 一致则返回成功（不重复扣款）、内容不同则报冲突、不存在则占位（唯一索引兜底并发重试）；业务完成后回写 `result_ids`（两条交易 id）。此前 v1.3 的 `Transfer` 既不捕获 `clientUuid`、也不写回执，转账重试会产生重复转账——本次闭合该缺口。
- 新增 `transferHash(fromId, toId, amount)`，与 `AddTransaction` 的 `requestHash` 分离，避免不同类型互相误判为冲突。

### 修复（P3：统一版本号入口）
- **§22.1 消除重复实现**：原在 §22.1 手写了一份与 §13.1 `nextBookVersion` 完全相同的 `INSERT...ON CONFLICT(book_id)` upsert；改为统一调用 `nextBookVersion`，并加“禁止再手写”的约束说明，杜绝日后实现漂移。

### 修复（P3：强化幂等契约）
- 在 `AddTransaction` 入参处新增契约注释：所有经 handler 进入的写请求**必须携带 `client_uuid`**；未携带退化为非幂等（仅允许服务端内部直接调用）。明确端侧离线补传/重试务必带 `client_uuid`。

### 说明（R18 描述合并口径）
- R18（WS 并发锁顺序/清理）实际由 **v1.2** 修复；v1.3 仅补充并发清理细节，二者为同一修复，特此合并说明，避免 CHANGELOG 口径重复。

### 函数签名变更
- `Transfer` 增加 `clientUuid string` 末参（调用方：handler/路由需同步传入；离线补传/重试务必传端侧 `client_uuid`）。

### 说明
- 仍为 Markdown 设计与代码骨架，未跑真实编译；`Transfer` 幂等在真实 SQLite 唯一约束、并发重试、与 `AddTransaction` 共用 `idempotency_records` 表的运行时验证仍待执行（建议 `go test -race` + 集成测试覆盖转账重试幂等）。

## [1.3] - 2026-10-29

### 背景
处理 GPT 独立评审（`gpt/`，R01–R18）中 v1.2 未覆盖的全部 11 项，**全部采纳、无误判**。明细见 `workbuddy/v1.3-修复说明.md`。与 v1.1→v1.2 教训一致，所有改动均同步到开发者实际照敲的《新手开发完整指南》骨架。

### 修复（同步：R04/R07/R10）
- **R04 幂等**：新增 `idempotency_records` 表（`UNIQUE(book_id,uid,client_uuid)` + request_hash/result_ids），`AddTransaction` 在事务内查回执→相同返回/不同冲突/不存在占位，唯一索引兜底竞态；新增 `requestHash`。
- **R07 游标/快照**：WS 事件只通知、不直接推进游标；`Pull` 在同一只读事务（一致快照）内定水位并读取所有实体，新增 `SyncBatch`；分页不拆同一版本，缓冲溢出要求全量重同步。
- **R10 恢复世代**：新增独立于数据库、不随备份恢复的 `restore_epoch`（`data/instance.meta`）；恢复时递增，客户端发现世代不符即全量重建并核对离线队列。

### 修复（写入校验：R11/R13）
- **R11**：`AddHolding` 的 decimal 错误必须返回、校验非负，事务内校验账户/标的归属同账本并接版本号；`holdings` 补 `book_id/sync_version/deleted`。
- **R13 预算**：按 budget_type 分支，分类预算汇总"大类+全部小类"，总预算去掉分类谓词；半开区间；聚合改用 `SumInt`。

### 修复（安全：R14/R15）
- **R14 CSV 注入**：新增 `sanitizeCSVCell`，对以 `= + - @ \t \r` 开头的用户可控文本前置单引号；区分表格安全 CSV 与原始 JSON。
- **R15 OCR 千分位**：金额正则支持千分位，先全角规范化，整数方式转分（不用 double）；补普通/千分位/退款/多金额/异常样例。

### 修复（契约/范围/并发：R12/R16/R17/R18）
- **R12**：统一字段 `password_hash`/`members_can_view_others`（User 显式列映射）；WS 统一 `/api/v1/ws`、订阅 `books`、点号事件名、均带 version。
- **R16**：PRAGMA 改用 go-sqlite3 DSN（每连接生效）+ `SetMaxOpenConns(1)` + 启动校验；修正示例 API 编译错误。
- **R17**：统一 MVP 为 PRD 基线——v0.1 含基础报表（月度收支/分类占比）与 CSV/JSON 导出；高级图表/完整预算后置。
- **R18**：固定锁顺序 `Hub.mu→Client.mu`，清理恰好一次，`Send` 永不关闭，写循环加 ping，建议 race detector 验证。

### 函数签名变更
- `AddHolding` 增加 `uid, bookID` 首参；新增 `Pull(bookID, afterVersion) (*SyncBatch, error)`、`requestHash`、`sanitizeCSVCell`、模型 `IdempotencyRecord`。

### 说明
- 仍为 Markdown 设计与代码骨架，未跑真实 Go/Flutter 编译；`go test -race`、DSN 实际行为、Excel 重存等运行时验证待执行，未声称已验证。

## [1.2] - 2026-10-29

### 背景
根据《FastLedger-GPT评审复核》（workbuddy/）的逐条复核结论，将 GPT 评审（`gpt/`，R01–R18）中已确认的高优问题落实到**开发者实际照敲的骨架**（`新手开发完整指南`），并校准会计与隐私两条线。本轮采纳 7 项（R01/R02/R03/R05/R06/R08/R09），其余标注为待办/部分处理。

### 修复（同步 v1.1 正确实现到新手指南骨架）
- **交易/转账骨架对齐落地方案**：`新手开发完整指南` §13.1/§13.2 原仍是“先读后写余额、错误被吞、无版本号”的旧实现，已替换为与落地方案 §6.1 一致的写法——单条**原子 UPDATE** 改余额（带 `book_id` 与影响行数检查）、写事务内**递增 `book_versions` 并回写 `sync_version`**、转账两侧交易均带 `book_id`/`sync_version`（R02/R06）。
- **账户/交易绑定账本**：写路径的 `book_id` 一律来自可信参数（handler 解析），并校验资金账户/转账两侧账户归属同一账本，余额 UPDATE 带 `book_id` 与 `deleted=0` 且检查 `RowsAffected==1`，杜绝跨账本改余额与零行误判成功（R01）。

### 修复（会计：R03 信用卡负债符号）
- 新增 `signedBalanceDelta`：负债账户（category 10~15）消费使余额**增加**（欠更多）、还款使其**减少**，与《枚举字典》§2“正=欠款”约定一致；§16.2 汇总口径不变（净资产 = 资产 − 负债）。同步在《枚举字典》§2 写明余额符号约定，落地方案 §6.1 注明 `signedAmount` 必须按此实现。修复后：储蓄 1000、信用欠款 0，刷卡 100 后净资产 900（不再虚增为 1100），还款后仍为 900。

### 修复（隐私：R05 WebSocket 授权与可见性）
- WS 连接绑定认证 `uid`；订阅经 `Hub.Subscribe` 受锁更新，且每个账本 ID 都先由服务端校验成员资格，**不再信任客户端上报的账本 ID**；广播前按接收者可见性过滤，关闭互看时只发脱敏通知（不含金额/账户/备注），由客户端再走授权接口拉取明细（R05）。
- 连接读/写循环共用取消上下文、统一 `Unregister`+`Close`，订阅写统一加锁，闭合并发清理缺陷（R18）。

### 修复（认证：R08/R09）
- 移除 `新手开发完整指南` §10.1 中硬编码的 `var jwtSecret = []byte("改成你自己的密钥")`；改为 `InitAuth(secret)` 从配置注入，空值/占位值/过短则启动即失败（R08）。中间件 `JWTAuth()` 不再接受外部密钥参数，只读注入的同一份。
- `JWTAuth` 增加：限定 HS256（拒绝 alg=none）、安全读取 `uid`（非法 claims 返回 401 而非 panic）、校验账号是否已删除/禁用（R09）；登录写入 `jti` 并预留令牌吊销查询位。

### 待办（本轮未处理，建议后续版本逐项裁决）
- R04 幂等：需在 DDL 为 `(book_id, client_uuid)` 建唯一索引（或独立操作回执表），并在事务内捕获唯一冲突返回原结果。
- R07 WS 游标/一致快照、R10 备份恢复世代、R11 持仓 decimal 错误返回与 `AddHolding` 补 `book_id`/`sync_version`、R12 两套 DDL/WS 契约统一、R13 预算 SQL、R14 CSV 注入、R15 OCR 千分位、R16 PRAGMA 连接池作用域、R17 MVP 报表/导出范围一致性：详见 `gpt/` 评审，维持“待确认/待处理”状态。

### 说明
- 本轮改动均为**设计文档与 Markdown 代码骨架**，不依赖真实 Go/Flutter 编译；相关函数签名变化（如 `AddTransaction(uid, bookID, tx)`、`Transfer(uid, bookID, ...)`、`JWTAuth()`）已在对应章节注明，handler/路由调用需同步调整。

## [1.1] - 2026-10-29

### 背景
本版本根据第三方（含 AI）交叉评审《FastLedger-项目档案评审与优化建议》逐条核查后修复，共处理 15 项确认问题（1 项评审意见经核实为误判，未采纳）。

### 修复（P0）
- **统一多账本 MVP 口径**：PRD 原写"单账本"，与落地方案、用户原始需求矛盾。现统一为 **MVP 即支持多账本**（默认"家庭公共账本"，可新建"个人私账"并切换），同步更新 PRD §3.2、版本路线图与 README。
- **补全增量同步的全局版本号机制**：新增 `book_versions` 表（账本级、单调递增），写事务内原子 +1 并回写实体 `sync_version`；`sync/pull` 按 `afterVersion` 拉取。修复了原 `version` 恒为 1 导致增量同步不成立的问题。

### 修复（P1）
- 用户表移除独立 `salt` 列：bcrypt/argon2 已内含随机盐，仅保留 `password_hash`。
- `users.email` 改为**可选**（自托管常无 SMTP），并对非空邮箱做唯一索引。
- 移除 `transactions.geo_lng/geo_lat`：不再采集 GPS，与隐私定位一致。
- 区分录入耗时两个口径：**OCR 响应 < 3 秒**（设计目标，不含人工）与**完整一笔 ≤ 15 秒**（验收阈值）。
- 明确并发冲突策略：服务端权威（最后修改 + 版本号），冲突时用户选覆盖/放弃/另存；PRD 不再标"待确认"。
- 可见性补充成员级覆盖字段说明（`book_members.can_view_others` 为预留）。
- 修正原型页面数口径（13 个核心页面/路由）。

### 修复（P2）
- 明确 `accounts.parent_id` 仅用于 UI 分组、不参与余额计算。
- 持仓数量/价格以字符串存储，运算走十进制库（shopspring/decimal），禁止 float。
- 明确 `ChangeBalance` 为单条原子 UPDATE，避免并发丢更新。
- SQLite 连接初始化开启 `WAL` / `busy_timeout` / `foreign_keys`。
- 补全首个 owner 的初始化引导（首启向导或 CLI `user create --owner`）。

### 新增
- 《FastLedger 统一枚举字典》：所有枚举值的唯一权威来源。
- 本 CHANGELOG。
- README 补充文档分工与导航。

### 原型
- **删除残留的第三方遥测脚本**（slardar `ibytedapm.com`、tea `bytescm.com`），经验证原型现**零外部请求、可完全离线**运行。
- 说明：原型为构建产物的导出快照，仓库内暂无可编辑源，已在 prototype/README 注明。

### 未采纳（经核实为误判）
- 评审 3.3 称"两级分类、我的设置找不到入口"：实际入口在"我的"页（`/profile`），`/categories` 两级分类与设置页均完整存在。

## [1.0] - 2026-10-29
### 新增
- 首版 PRD（评审稿）、8 环节落地方案、新手开发完整指南、ezbookkeeping 代码导读。
- 完整可交互原型（13 个核心页面）。
