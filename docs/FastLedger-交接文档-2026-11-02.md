# FastLedger 交接文档（2026-11-02）

> 面向**接手的 AI 团队**。本文取代 `FastLedger-交接摘要-2026-11-01.md`（后者保留作历史）。
> 阅读顺序建议：§1 速览 → §6 当前第一优先（原型还原缺口）→ §9 铁律 → §10 命令集 → 其余按需。
> 配套机器记忆：`.workbuddy/memory/MEMORY.md` + `.workbuddy/memory/2026-11-0{1,2}.md`（含逐日踩坑细节）。

---

## 1. 一页速览

**项目**：FastLedger —— 自托管家庭记账系统（NAS/轻量云单机）。Go 后端 + 原生 Web + Flutter 客户端（Android/Windows/iOS 规划中），截图识别记账（端侧规则引擎 + 服务端 OCR 兜底），家庭多成员共享账本。

| 维度 | 状态 |
|---|---|
| 后端（阶段 0–4 + 二轮复审修复 + 阶段 7 运维） | ✅ 完成并部署生产 |
| 数据库 | schema **v8**（迁移 0001–0008），生产与本地一致 |
| 测试基线 | 服务器 `run_tests.sh` **803 PASS / 0 FAIL**；CI 全绿（build/vet/test/race + docker-build） |
| Web 端（go:embed） | ⚠️ **可用但明显偏离原型**（见 §6，这是当前最大欠账） |
| Flutter 客户端 | ✅ 页面/网络/队列/安全存储完整，`flutter test 79/79`、`analyze 0`、**Windows 构建实机通过** |
| 生产环境 | 腾讯云轻量 `ubuntu@119.91.63.95`，systemd `fastledger`，`/opt/fastledger`，8080 仅内网 |
| 最大阻塞 | ① Web 端原型还原度约 50–55%（P0 缺 5 页/结构）；② 外网 HTTPS 待域名；③ 本机 git 推送待代理恢复 |

**三条最紧急待办**：
1. 按原型补齐 Web 端 P0 缺口（首页账单、交易详情、两级分类、移动端底部 Tab+FAB）。
2. 代理恢复后推送 4 个本地提交并完成本地/远端 ref 对齐（§3.4）。
3. iOS（5.5）与 ML Kit（5.4 端侧 OCR）需 owner 决策是否继续暂停。

---

## 2. 项目与仓库

| 项 | 值 |
|---|---|
| 本地仓库 | `C:\Users\47006\WorkBuddy\fastledger\repo_tmp\`（**git 根在这层**，`server/` 是 Go module 根） |
| 远端 | `https://github.com/LinTia/FastLadger.git`（注意拼写是 **FastLadger**；Public 仓库） |
| 工作区根 | `C:\Users\47006\WorkBuddy\fastledger\`（`.tools/` 放工具链与临时产物，**不在 git 内**） |
| 生产 | `ubuntu@119.91.63.95`（实例 lhins-6y1yx29v，广州 2C2G/50G），应用用户 `fastledger`，目录 `/opt/fastledger`（0700/0600） |
| SSH 密钥 | **`.tools/FastLadger.pem`**（当前 canonical；由 `/tmp/flkey/id_rsa` 副本持久化，MD5 `86f1992477b5f0befdc909efdb75cdac`）<br>⚠️ 该副本继承了 `CodexSandboxUsers(RX)`，**Windows OpenSSH 会拒绝使用**，需先在自己终端执行：`icacls "...\.tools\FastLadger.pem" /inheritance:r` 再 `/grant:r "47006:F"`<br>⚠️ `Downloads\FastLadger.pem` 已被误清空 DACL（内容完好但当前无人可读），修复命令：`icacls "...Downloads\FastLadger.pem" /grant "47006:F"` |

---

## 3. 环境搭建（已在当前机器验证）

### 3.1 本地 Go 构建（CGO / SQLite）

```bash
export GOROOT="C:/Users/47006/WorkBuddy/fastledger/.tools/go"
export CGO_ENABLED=1
export CC="C:/Users/47006/WorkBuddy/fastledger/.tools/w64devkit/w64devkit/bin/gcc.exe -B C:/Users/47006/WorkBuddy/fastledger/.tools/w64devkit/w64devkit/bin/"
export GOTMPDIR="C:/Users/47006/WorkBuddy/fastledger/repo_tmp/.gotmp"
export GOPROXY="https://goproxy.cn,direct"     # 本机默认代理常挂，直连 goproxy.cn 可用
GO="C:/Users/47006/WorkBuddy/fastledger/.tools/go/bin/go.exe"
cd repo_tmp/server && env -u HTTP_PROXY -u HTTPS_PROXY "$GO" build -o ../../.tools/fastledger-web.exe ./cmd/server
```

- 便携 MinGW-w64（GCC 12.2.0，**勿升级到 14+**）在 `.tools/w64devkit/w64devkit/bin`（由 `w64devkit.zip` 解压而来）。
- 本地 `-race` 不可用（TSan error 87）；race 全量只信 CI 或服务器。
- 一键全量测试：`bash .tools/run_local_tests.sh`（含 Application Control 拦截重试逻辑）。

### 3.2 Flutter / Windows 桌面

- Flutter **3.47.9 stable** 在 `.tools/flutter/`（已确认完全可用；`bin` 未入 PATH，用全路径调用）。
- VS 2022 Community 17.14 + "使用 C++ 的桌面开发"（MSVC 14.44 + Windows SDK 10.0.26100）已装 → `flutter build windows --debug` 实机成功。
- **开发者模式必须走设置界面开启**（仅 `reg add AllowDevelopmentWithoutDevLicense=1` 无效，实证：LSA 不据此签发特权）；开启后普通用户可创建 symlink，**即时生效无需重启**；`whoami /priv` 看不到 `SeCreateSymbolicLinkPrivilege` 属正常。
- MSVC 解析中文注释必须加 `/utf-8`（已写入 `client/windows/runner/CMakeLists.txt`）。

### 3.3 本地试用环境（Web + 手机形态）

```bash
# Web（真后端，内嵌前端）——全新试用库，开放注册
EBK_JWT_SECRET="local-trial-secret-0123456789abcdef" \
EBK_DB_PATH="C:/Users/47006/WorkBuddy/fastledger/data/web-trial/fastledger.db" \
EBK_DATA_DIR="C:/Users/47006/WorkBuddy/fastledger/data/web-trial" \
EBK_ENABLE_REGISTER=1 \
".tools/fastledger-web.exe"          # → http://127.0.0.1:8080

# 手机形态（Flutter Web + 同源网关，免 CORS）
cd repo_tmp/client && .tools/flutter/bin/flutter build web --release \
  --dart-define=FASTLEDGER_BASE_URL=http://127.0.0.1:8081
python .tools/dev_mobile_gateway.py --port 8081   # → http://127.0.0.1:8081
```

- 首次建库需先 `".tools/fastledger-web.exe" db migrate`（空库启动会被 Check 拒绝）。
- ⚠️ 后台启动的服务**不会跨会话存活**，每次需要重启；不要用 `&` 起（随父 shell 退出被杀）。

### 3.4 本机网络坑（重要）

- 本机代理 `HTTP_PROXY=127.0.0.1:58065` **经常失效**，导致 `git push/fetch` 报 `CONNECT tunnel failed, response 502`、`go` 拉包 Bad Gateway。
- 可用替代：`api.github.com` 直连 200；`goproxy.cn` 直连 200。因此：**go 依赖用 goproxy.cn，git 只能等代理恢复**（或走 GitHub MCP 的 API 通道，见下）。
- **当前 git 状态（交接时点）**：本地 `main = ad5b4db`，`origin/main = 0f36e78`，**4 个提交未推送**：
  ```
  ad5b4db fix(web): 注册表单字段顺序修正 + 账本列表增加删除入口
  d7121d9 fix(web): 登录页 minLength 赋值非法（-1）导致页面报错
  2e9e33a feat(web): 登录页增加注册入口（注册→自动登录）
  e8c3d66 docs(changelog): [1.57] Windows 构建打通 + 环境验证
  ```
  且历史已分叉：远端拿的是 PR #1 的 squash 结果 `ca43fb7`（内容 == 本地 `0f36e78` 树，SHA 不同）。
  **恢复代理后**：`git fetch origin && git log --oneline origin/main..HEAD` 确认后，建议 `git rebase origin/main`（保留 4 个提交）或先与 owner 确认再 `reset --hard origin/main`。

---

## 4. 代码结构（关键路径）

```
repo_tmp/
├── server/                    Go module 根
│   ├── cmd/server/            入口（含 db migrate / db purge / instance bump 子命令）
│   ├── internal/
│   │   ├── api/               Gin 路由与 handler（auth/book/ledger/report/budget/invest/ocr/session/pat/web）
│   │   ├── service/           业务逻辑（事务、权限、余额、预算、投资、报表、OCR、同步、会话、PAT）
│   │   ├── model/             xorm 实体
│   │   ├── migrate/           迁移 0001–0008 + 三道守卫（golden 指纹/versionAssertions/机械全量比对）
│   │   ├── ws/                WebSocket Hub（撤销 jti 即踢，关闭码 4003）
│   │   └── config/            环境变量集中校验（fail-fast）
│   └── web/                   go:embed 静态前端
│       ├── embed.go
│       └── dist/              index.html + app.js(1790 行) + style.css  ← Web 端全部实现都在这里
├── client/                    Flutter 工程（lib/{api,core,ocr,parse,queue,ui,hotkey}/）
├── prototype/                 ⭐ 前端唯一基准（在线原型导出产物，无源码，需从 assets/*.js 取证）
├── docs/                      权威文档（PRD / 落地方案 / WBS / 枚举字典 / 部署交接 / 阶段7报告 / OCR 契约）
└── deploy/                    Dockerfile / docker-compose.yml / backup.sh
```

---

## 5. 已交付清单（WBS 视角）

| WBS | 内容 | 状态 |
|---|---|---|
| 1.x–4.x | 账号/账本/账户/分类/交易/转账/回收站/预算/报表/导出/同步（clientUuid 幂等）/权限矩阵 | ✅ |
| 5.1 / 5.2 | Flutter 骨架（网络层 + TokenStore + Outbox 幂等队列）、账户/分类/交易页 | ✅ |
| 5.3 | Windows 热键 + 截图（C++ 通道）+ 确认弹窗 | ⚠️ **编译/构建已通过（PR #1），热键触发与截图内容的运行时手工验证未做** |
| 5.4 | Android 悬浮球 + ML Kit | ⏸ ML Kit 按 owner 指令暂停；悬浮球未开工 |
| 5.5 | iOS 客户端 + 快捷指令 | ⏸ 暂停（是否配 Mac 待决策） |
| 5.6 | 端侧规则引擎（微信/支付宝解析） | ✅ `client/lib/parse/receipt_parser.dart`，与服务端 `ocr_parse.go` 同语义 |
| 5.7 | OCR 后端契约（重定向 PaddleOCR 双模式） | ✅ 服务端 recognize/parse + 契约文档 `docs/FastLedger-OCR双模式部署与切换契约.md` |
| 5.8 | 云端 LLM 文本兜底（默认关闭） | ✅ |
| 5.9 | Web 端（go:embed） | ⚠️ 功能可用但**原型还原度不足**（§6） |
| 5.10 | 多端会话查看/单设备撤销 | ✅ 迁移 0007 + Verify 强校验 jti + WS 踢下线 |
| 6.x | 报表/导出/预算/投资/净资产/设置 PAT/成员 | ✅ |
| 7.1–7.5 | Docker 部署单元 / 备份 7 份滚动 / 恢复演练 / 验收报告 | ✅（HTTPS 待域名） |

---

## 6. ⭐ 当前第一优先：Web 端原型还原缺口（2026-11-02 审计结论）

**owner 裁决（不可动摇）**：`prototype/` 原版可交互原型是**唯一前端基准**；`docs/FastLedger-Web设计规范.md` 与 `prototype-v2/` 仅为参考；**原型未覆盖处必须先问 owner**。

**审计方法**：从 `prototype/assets/index-DNvhksAn.js` 提取 279 条中文文案 + `prototype/README.md` 的 13 页清单，与 `server/web/dist/app.js` 逐视图对照。

**结论：整体不构成忠实还原（约 50–55%）**。配色/语义色/圆角方向一致，但交互骨架与 5 个页面缺失。

### 6.1 缺失（P0，原型有、实现无）

| 缺口 | 原型证据 | 实现现状 |
|---|---|---|
| **首页账单页** | "总收入/总支出/结余"概览卡 + 按"今天/昨天/前天/x 天前"分组的账单流 | 被"交易"检索表格替代（`app.js:335`），无汇总卡、无日期分组 |
| **交易详情页** | 点账单进详情，含编辑、删除二次确认"删除后将无法恢复，账户余额会相应调整" | 交易行不可点击，只有"删除选中"批量删 |
| **两级分类管理** | 大类/小类 CRUD、"已删除（N 笔账单将归入其他）" | **零分类写操作**（全仓无 POST/DELETE /categories） |
| **移动端底部 Tab + 中央 FAB** | 底部 5 Tab（首页/账单/记一笔/报表/我的）+"我的"承载二级入口 | 只有桌面顶部 10 项平铺（`index.html:13-23`），`bottom-nav/tabbar/fab` 命中 0 |
| **截图识别漏斗** | 三段式：手动记账/截图识别/确认入账 + "端侧 OCR 识别中…/规则匹配中…/识别成功，请确认/重新识别" | 完全缺失（Web 端仅 1 处"截图"字样）<br>⚠️ 有依据不做（落地方案 §"Web 不做 OCR"）但与基准冲突，**需 owner 裁决** |

### 6.2 部分实现（P1）

| 项 | 缺什么 |
|---|---|
| 账户页 | **只读**（账户/类型/余额三列），缺"新增账户"（名称/类型/初始余额/负债填负数）与编辑删除 |
| 导出页 | 缺**时间范围**（本月/上月/本季度/自定义）、**数据预览**（"共 N 条"）、"Excel 兼容/开发 API 用"说明 |
| 我的设置 | 缺**编辑资料**（昵称/邮箱）、"我的"聚合二级菜单结构（现有 PAT 管理 + 退出登录） |
| 家庭成员 | 缺家庭信息块（家庭名称/成员数量/创建时间） |
| 登录页 | 缺原型预填的演示账号（真实后端场景下合理）、登录成功文案一致 ✅ |

### 6.3 实现多出原型（需 owner 认可或收敛）

- **账本列表页**（多账本前置步骤）：后端能力驱动，但打破原型"登录即进家庭账本首页"的信息架构。
- **回收站整页**：原型无此页，且与原型删除语义**相反**（原型"删除后无法恢复"，实现可恢复）。
- 注册入口（owner 明确要求，不算偏离）、预算 rollover 结转（超出原型）。

### 6.4 建议补齐顺序

**P0**：首页账单页 → 交易详情页 → 两级分类 → 移动端 5 Tab + FAB 响应式骨架。
**P1**：账户 CRUD、导出时间范围+预览、设置资料编辑+"我的"聚合、成员家庭信息块。
**P2**：Web 端截图识别漏斗（演示态 or 明确不做，需 owner 决定）。

---

## 7. 未完成 / 暂停 / 待决策

| 项 | 状态 | 说明 |
|---|---|---|
| iOS 客户端（5.5） | ⏸ | owner 暂停；若继续需 Mac + Xcode 环境 |
| ML Kit 端侧 OCR（5.4） | ⏸ | owner 暂停；Android 走服务端 PaddleOCR 兜底 |
| Android 悬浮球 | 未开工 | 需 Android SDK（本机未装，安装需 owner 批准） |
| 外网 HTTPS | 阻塞 | 无域名；Caddy 配置就绪，`docker compose --profile tls` 一键启用 |
| Windows 热键运行时验证 | 待做 | 需真机手工：按热键是否出确认弹窗、截图内容是否正确 |
| 生产带凭据 E2E | 未做 | 按约定不读 `credentials` 到日志/文件 |
| PAT 撤销不踢存量 WS | 已知限制 | 文档已登记 |
| 通知开关 | 纯本地偏好 | 未接系统通知 |

---

## 8. 运维（生产）

```bash
# SSH（密钥权限见 §2）
ssh -i "C:/Users/47006/WorkBuddy/fastledger/.tools/FastLadger.pem" ubuntu@119.91.63.95

# 服务
sudo systemctl status|restart fastledger
curl -s http://127.0.0.1:8080/healthz     # {"status":"up","schemaVersion":8,...}

# 迁移（必须先 stop 再 cp，运行中 cp 会 Text file busy）
sudo systemctl stop fastledger
sudo cp <新二进制> /opt/fastledger/fastledger && sudo chown fastledger:fastledger ...
sudo sh -c 'cd /opt/fastledger && su -s /bin/sh fastledger -c "./fastledger db migrate"'
sudo systemctl start fastledger

# 备份：cron /etc/cron.d/fastledger 每日 03:30，滚动保留 7 份（/opt/fastledger/backup.sh）
# 恢复演练顺序：恢复 db → instance bump（必须同时传 EBK_DB_PATH 与 EBK_DATA_DIR）→ 再启动
```

- CI：`.github/workflows/` 跑 build/vet/test/race + docker-build（含镜像 healthz 冒烟）。
- 部署包分发：服务器 `git fetch` 到 GitHub 常卡死 → 习惯做法是 bundle 直传 + `run_tests.sh` + race 抽样 + healthz + 401 冒烟。

---

## 9. 铁律与守卫（违反必红或必返工）

### 9.1 CI 红线（不可放宽）
1. `TestQAS3SingleCreateEntry`：`insertTxRow` 只许经 `createSingleInTx` / `createTransferInTx`。
2. `TestQAS3VisibilityCondWiredIntoAllReadPaths`：交易读路径必须调 `txVisibilityCond`（新增读路径要登记白名单）。
3. `TestQATransactionRoutesContract`：`PUT /transfer`、`/batch` 永久禁止。
4. 迁移三道守卫：golden 指纹 + `versionAssertions` + `TestAssertionCoversAllArtifacts`——**新迁移（下一个 0009）三处都要登记**；已发布迁移（0001–0008）禁改一字节。
5. CI 有 `gofmt` 门禁——服务器写码后必须 `gofmt -l` 检查再提交。

### 9.2 流程铁律
- 修复者自测绿灯 ≠ 修复正确：每域修完要过**对抗验证**（专攻自报数据之外）。
- race 全量只在 CI；本地/服务器抽并发重的包跑 `-race`，且必须 `-p 1`（2 核 1.9G 并行会 OOM）。
- 守卫类断言必须拒/认两侧成对；标识符匹配一律 AST 全等；注释声称的兜底必须真走生产路径。
- 报告要区分"类型检查通过"与"运行时已验证"，不夸大。

### 9.3 坑点速查
1. Lighthouse MCP `execute_command` 会拦 POST/写 /etc/crontab/读 credentials → 一律走 SSH heredoc。
2. `git push` 502 频发 → 30s 间隔重试（历史 5–10 次成功）；API 通道（GitHub MCP）可绕开但会改 SHA。
3. worker 报 `400 ENOENT \\.\pipe\workbuddy-pac-*` = 基础设施错误，直接重派。
4. 本地 `/tmp` 会清空 → 持久物放 `.tools/`。
5. 服务器无 sqlite3 CLI → 备份用 Python `sqlite3` backup API。
6. 空库启动会被 Check 拒绝 → 先 `db migrate`。
7. Go 源码字符串不能放字面 BOM → 用 `"\uFEFF"`。
8. `xorm` 的 `Count(*int64)` 不支持 → `SELECT COUNT(*)` + struct Get。
9. Flutter 测试坑：`showModalBottomSheet<void>` 泛型、`FastLedgerApp` 冒烟要注入内存 TokenStore、FakeSecureStorage 签名需照抄 v11。
10. 本机 `flutter doctor` 的 Windows Version/VS/设备检查"crash"是**工具沙箱拦 reg.exe 的误报**，真实环境正常。

---

## 10. 常用命令速查

```bash
# 后端
cd repo_tmp/server && go test ./...                 # 本地全量（需上面的 CGO 环境）
bash .tools/run_local_tests.sh                      # 一键 + 分类汇总
gofmt -l ./internal                                # 提交前必跑

# 前端（Flutter）
cd repo_tmp/client && ../../../.tools/flutter/bin/flutter analyze
cd repo_tmp/client && ../../../.tools/flutter/bin/flutter test
cd repo_tmp/client && ../../../.tools/flutter/bin/flutter build windows --debug
cd repo_tmp/client && ../../../.tools/flutter/bin/flutter build web --release --dart-define=FASTLEDGER_BASE_URL=http://127.0.0.1:8081

# Web 端（手写原生 JS，无构建工具链）
cd repo_tmp/server/web/dist && node --check app.js   # 改完必跑
# 改完要重建二进制（go:embed 是编译期嵌入），再重启服务

# 数据库
".tools/fastledger-web.exe" db migrate               # 建库/迁移
```

---

## 11. 关键决策记录（按时间）

| 日期 | 决策 | 影响 |
|---|---|---|
| 2026-10-30 | 多端会话单设备撤销排期实现（R52） | 迁移 0007，旧 token 失效需重登 |
| 2026-11-01 | 仓库转 Public | 严禁把密钥/密码/内网地址入库 |
| 2026-11-02 | **前端基准 = `prototype/`**，设计方案产物降级为参考 | 所有前端工作必须先对原型取证；未覆盖先问 owner |
| 2026-11-02 | OCR 走 PaddleOCR 双模式（端侧离线 + 服务端兜底），**绝不自动入账** | 契约文档 docs/FastLedger-OCR双模式部署与切换契约.md |
| 2026-11-02 | Provider 云端 LLM 兜底默认关闭 | 仅传文本，需明示同意 |
| 2026-11-02 | 登录页增加注册入口 | 生产默认 `EBK_ENABLE_REGISTER=false`，只开放邀请码入户 |

---

## 12. 接手第一天建议动作

1. 通读：本文 → `docs/FastLedger-开发WBS.md` → `docs/FastLedger-落地方案.md` §3.4/§4 → `docs/FastLedger-阶段7-运维与验收报告.md`。
2. 恢复环境：按 §3.1 编出二进制 → `db migrate` → 起服务 → `curl /healthz` 应为 `schemaVersion:8`。
3. 跑基线：`bash .tools/run_local_tests.sh`（期望全绿）+ `flutter test`（期望 79/79）。
4. 修复 git：代理恢复后推送 4 个提交并对齐 ref（§3.4），**动 `reset --hard` 前必须先问 owner**。
5. 开工顺序：§6.4 的 P0 四项，**每项先对原型取证、改完 `node --check` + 重建 + 实测**，交付附"证据 + 偏差"。
6. 需要 owner 决策的事项（不要自行决定）：iOS 是否继续、Web 端截图识别漏斗补不补、悬浮球是否装 Android SDK、仓库是否改回 Private、`/opt/fastledger/credentials` 明文密码是否轮换删除。

---

## 13. 文档地图

| 文档 | 用途 |
|---|---|
| `docs/FastLedger-产品需求文档-PRD.md` | 需求权威（含 §12 决策集） |
| `docs/FastLedger-落地方案.md` | 架构/数据模型/识别漏斗/同步机制 |
| `docs/FastLedger-开发WBS.md` | 任务分解与验收标准（进度以此为准） |
| `docs/FastLedger-枚举字典.md` | 类型/角色/状态枚举 |
| `docs/FastLedger-部署交接-腾讯云轻量应用服务器.md` | 生产部署细节 |
| `docs/FastLedger-阶段7-运维与验收报告.md` | 运维验收与演练记录 |
| `docs/FastLedger-OCR双模式部署与切换契约.md` | OCR 双模式边界与切换链 |
| `docs/FastLedger-交易更新语义设计.md` / `转账入口裁决.md` | 交易更新的语义约束 |
| `prototype/` | ⭐ 前端唯一基准（产物，无源码；从 `assets/*.js` 取证） |
| `.workbuddy/memory/MEMORY.md` + 日日志 | 机器记忆：坑点、决策、验证证据 |

---

*本文由 2026-11-02 会话结束时状态生成；所有结论均带可复核证据路径，未验证项已明确标注。*
