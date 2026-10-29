# 家庭智能记账软件 · 8 环节落地方案

> 代号：**FastLedger**（暂定，可改）
> 一句话：一个跑在自家 NAS 上的家庭记账系统，按一个快捷键/点一下悬浮球，截图后由手机/电脑本地识别账单，确认即入账，家庭成员实时共享。
> 文档性质：研发可执行方案（已对齐关键决策，可直接照此开工）。

---

## 已确认的关键决策（决策记录）

| 决策项 | 结论 |
|---|---|
| 后端 | **Go + Gin + xorm**，SQLite 起步（结构预留 PG/MySQL） |
| 客户端 | **Flutter 原生客户端**：Windows + Android + **iOS**（浏览/管理/手动记账/同步）；**Web 端**：Vue 响应式网页，由 Go 后端直接托管，浏览/编辑，**不做 OCR**；**iOS 的 OCR 借助快捷指令**：截图→系统 OCR→结果回传 App/API，App 本身受沙盒限制不做全局截图 |
| 账户模型 | **资金账户与支付渠道分离**：信用卡/储蓄卡/现金/微信零钱/支付宝余额是"资金账户"；微信/支付宝在快捷支付时只是"支付渠道"（标签），不是账户。每笔支出只记一个资金账户，避免重复计算 |
| 投资与资产 | 账户分类预留**投资账户（券商/基金）、实物资产（黄金/房产/汽车）、长期负债（房贷/车贷）**；新增**标的/持仓/行情价格**骨架；MVP 建表并支持手动录入，**v2 自动行情（A股/港股/基金净值）、v3 完整资产合计**；家庭资产/净资产统一折算本币 |
| 识别 | **端侧 OCR（图片不出设备）+ 端侧规则提取**；复杂账单把 **OCR 文本（非图片）** 送云端 LLM 兜底；原图永不上传。Android 用 ML Kit、Windows 用系统 OCR、iOS 用快捷指令自带的"提取文本" |
| 家庭规模 | 2 人起步，**架构支持 5 人及以上** |
| 账本 | 支持**多个独立账本**（家庭公共账 + 个人私账） |
| 可见性 | 成员**默认互相可见**，账本级有"是否可见他人记录"开关 |
| 角色 | 有**管理员**（拥有者/管理员/普通成员三级） |
| 访问 | **外网可访问**，Caddy 反向代理 + 自动 HTTPS + DDNS |
| MVP 范围 | 只做"识别→确认→入账→共享"闭环，但**数据结构与模块预留全部扩展能力** |

---

# 环节 1：需求分析

## 1.1 产品定位

- **自用 + 家庭共享**的私有记账系统，不做公开 SaaS、不做运营、不做商业化。
- 核心价值：**记账动作足够快**（一键截图后识别结果 3 秒内弹出，含人工确认的完整一笔目标 15 秒内），且**数据完全在自己手里**。两个口径的区分见 §1.5。

## 1.2 目标用户与角色

| 角色 | 人数 | 典型设备 | 核心诉求 |
|---|---|---|---|
| 拥有者（你） | 1 | Windows 电脑 + Android 手机 | 快捷键截图快速记账、管理家庭、管理 NAS |
| 管理员（可选） | 0~1 | Android 手机 | 协助管理成员、分类 |
| 普通成员（配偶等） | 1~4+ | Android 手机 | 悬浮球截图记账、看家庭账本 |

> 数据模型按"一个用户可加入多个账本、一个账本可容纳 N 个成员"设计，**5 人以上无需改表**。

## 1.3 核心用户故事（P0 闭环）

1. **[桌面快记]** 我在电脑上消费后（网页支付/账单页），按下 `Ctrl+Shift+L`，框选账单区域，系统本地识别出金额、商户、时间，弹出预填表单，我确认/微调后回车，这笔账进入家庭账本，其他成员的 App 实时刷新。
2. **[手机快记]** 我在手机上付完款，点一下常驻悬浮球，App 自动截取当前屏幕，本地识别账单，弹出预填表单，确认后入账。
3. **[家庭共享]** 配偶记的每一笔家庭支出，我能立刻在自己的 App 看到；我也能切换到"个人私账"记不想共享的账。
4. **[成员管理]** 我能创建账本、邀请成员、设置成员角色、开关"是否允许成员查看他人记录"。
5. **[离线可用]** 手机没网时也能截图记账，本地暂存，联网后自动同步。
6. **[网页管理]** 我在任意电脑/手机浏览器打开网页，能快速浏览、检索、批量编辑账本数据（改分类、删错账、对账），不需要截图识别。
7. **[iOS 记账]** 我在 iPhone 上用原生 App 浏览、管理账本；需要截图识别时，截图后运行快捷指令（可绑"轻点背面"/小组件），系统本地 OCR 提取文字，结果回传到 App 让我确认入账。

## 1.4 功能范围（范围取舍）

### MVP（v0.1，必须做）

- 用户注册/登录（JWT）、首个账本自动创建
- 账本管理：创建/切换账本、成员邀请（邀请码/链接）、成员角色、可见性开关
- 账户：账本内两级账户（默认预置"现金/储蓄卡/信用卡/支付宝/微信"）
- 分类：两级收支分类（默认预置常用分类）
- 交易：收入/支出/转账的增删改查、备注
- 识别：端侧 OCR + 规则提取 + 云端 LLM 兜底（仅文本）、识别后人工确认
- **Web 端**：响应式网页，由 Go 后端直接托管，浏览/检索/编辑/对账（无 OCR）
- **iOS 客户端**：Flutter 原生 App（浏览/管理/手动记账/同步）；**OCR 走快捷指令**：截图→系统 OCR→结果回传 App 确认（提供现成快捷指令模板，App 支持 URL Scheme 接收结果）
- 实时同步：WebSocket 推送新增/修改/删除
- 离线：本地数据库暂存、联网增量同步
- 部署：Docker 单镜像 + docker-compose + Caddy HTTPS

### MVP 明确不做（后置，表结构已预留）

| 后置功能 | 预留方式 |
|---|---|
| 图表统计/报表 | 交易表字段齐全，后期纯查询 + 前端图表 |
| 多币种/汇率 | 账户/账本都有 `currency` 字段，MVP 固定 `CNY` |
| 标签 | 预留 `transaction_tags` 表（MVP 可建表不启用） |
| 预算 | 预留账本/分类配置位 |
| 定时交易 | 交易表有 `scheduled` 标记位 |
| 图片附件云存储 | MVP 原图只留本地、不入库；预留 picture 表 |
| 微信/支付宝 CSV 批量导入 | 预留 `imported_transactions` 表 |
| Mac 原生客户端 | Flutter 跨端，后期补；iOS 原生 App 已在 MVP（OCR 走快捷指令） |
| 2FA / OIDC | 预留用户安全字段 |

## 1.5 非功能性需求

- **性能**：
  - 交易列表查询 P95 < 300ms（SQLite，家庭数据量）；
  - **端侧 OCR 单张 < 3 秒**（截图到确认弹窗弹出，这是设计目标，不含人工操作）；
  - **完整一笔（含人工确认）≤ 15 秒**（这是验收阈值，从触发到入账）。
- **资源**：后端常驻内存 < 150MB（Go）；N100/N150 无压力。
- **隐私**：原图不离开设备；默认只传结构化数据；云端识别仅传文本；**不做 GPS 定位采集**（交易表无经纬度字段）。
- **安全**：全链路 HTTPS；JWT 鉴权；登录限流；密码用 bcrypt/argon2 哈希（算法自带盐，不另设 salt 列）。
- **可用性**：数据每日备份（SQLite 单文件）；误删可恢复（软删除）。

---

# 环节 2：产品设计

## 2.1 功能结构图

```
FastLedger（同一套后端 API，三种使用形态）
├── 认证
│   ├── 登录 / 注册 / 退出
│   └── （后置）2FA / OIDC
├── 账本
│   ├── 账本列表 / 切换 / 创建
│   ├── 成员管理（邀请、角色、移除）
│   └── 账本设置（默认货币、可见性开关）
├── 记账
│   ├── 交易列表（按时间流）
│   ├── 新增 / 编辑 / 删除（收入/支出/转账）
│   ├── 快捷识别（截图→OCR→确认）★仅原生客户端 + iOS 快捷指令
│   └── （后置）标签 / 模板 / 图片附件
├── 基础数据
│   ├── 账户管理（两级）
│   └── 分类管理（两级）
└── 同步
    ├── WebSocket 实时推送
    └── 离线缓存 / 增量同步

三种形态：
├── Flutter 原生客户端（Windows / Android / iOS）：浏览管理 + 截图 OCR（iOS 的 OCR 借助快捷指令）
├── Web 端（Vue 响应式，Go 托管）：浏览器浏览/检索/批量编辑/对账，无 OCR
└── iOS 快捷指令：截图→系统 OCR→结果回传 App 确认（原生 App 仍负责浏览管理）
```

## 2.2 核心流程：截图记账（最重要的路径）

```
用户触发（快捷键 / 悬浮球）
   │
   ├─ 截图（Windows 框选 / Android 截屏）
   │
   ├─ 端侧 OCR：图片 → 文本（ML Kit / Windows OCR，离线）
   │
   ├─ 端侧规则引擎：匹配微信/支付宝/银行账单模板
   │      ├─ 命中 → 得到 金额/商户/时间/收支类型
   │      └─ 未命中 ↓
   │
   ├─ 云端 LLM（仅上传 OCR 文本，不传图）→ 结构化结果
   │      └─ 失败/无网络 → 仅填 OCR 原文，用户手填
   │
   ├─ 弹出确认表单（预填，金额高亮，默认账户/分类）
   │      └─ 用户确认或修改（金额/分类/账户/备注）
   │
   └─ 提交 → 本地库先写 → 推送后端 → WS 广播给其他成员
```

> **设计底线**：识别结果永远要人工确认，不自动入库。识别错金额是记账软件最大的事故。

## 2.3 iOS 客户端 + 快捷指令 OCR

iOS 有原生 App（浏览/管理/手动记账），但全局截图 OCR 受沙盒限制，借助快捷指令完成：

```
用户在微信/支付宝账单页截图
   │
   ├─ 运行快捷指令（可绑"轻点背面"/小组件/分享菜单）
   │
   ├─ "获取最新截图"
   │
   ├─ "从图片中提取文本"（iOS 原生 Vision OCR，端侧离线）
   │
   ├─ 文本结构化（两种路径）
   │      ├─ 快捷指令直接 POST /recognition/parse-text（仅传文本，Header 带 API Token）
   │      └─ 或把文本通过 URL Scheme 回传 App，由 App 走规则/LLM
   │
   └─ 唤起 App 弹出确认表单（预填金额商户），用户确认 → 入库 → WS 广播
```

> 快捷指令由我方提供模板（iCloud 分享），用户只需填入域名/Token；App 注册 URL Scheme 接收快捷指令结果。

## 2.4 Web 端（浏览器快捷管理）

- 定位：**数据浏览与编辑工作台**，不是识别入口。
- 能力：交易列表与高级筛选、搜索、批量改分类/账户、批量删除、对账、账户/分类/成员管理、导出。
- 不做：截图、OCR、悬浮球、全局热键（浏览器做不到也没必要）。
- 复用：与 Flutter 客户端共用同一套 REST API，响应式适配桌面/手机浏览器。

## 2.5 关键页面（各形态）

| 页面 | 桌面形态 | 移动形态 | Web 端 |
|---|---|---|---|
| 首页/交易流 | 左侧导航 + 右侧时间流列表 | 顶部月份 + 卡片流 | 表格 + 筛选器 |
| 识别确认弹窗 | 居中弹窗，快捷键可提交 | 底部弹出 sheet | 无（不做识别） |
| 账本/成员管理 | 表格 + 弹窗 | 列表页 | 表格管理 |
| 账户/分类管理 | 树形列表 | 折叠列表 | 树形/表格 |
| 批量编辑/对账 | 右键菜单 | 多选模式 | 批量操作栏 |
| 登录/注册 | 居中卡片 | 全屏表单 | 居中卡片 |

---

# 环节 3：架构设计

## 3.1 整体架构

```
┌────────────────────────────────────────────────────────────────┐
│  Flutter 原生客户端（含截图识别）                                │
│  ┌──────────────┐              ┌──────────────────────┐        │
│  │ Windows 端    │              │ Android 端            │        │
│  │ · 全局热键    │              │ · 悬浮球/无障碍       │        │
│  │ · 框选截图    │              │ · 截屏                │        │
│  │ · Windows OCR │              │ · ML Kit 中文 OCR     │        │
│  └──────────────┘              └──────────────────────┘        │
│  共享层：规则引擎 / 本地DB(Drift) / 离线队列 / HTTP+WS           │
└───────────────┬────────────────────────────────────────────────┘
                │
┌───────────────┴────────────────────────────────────────────────┐
│  Web 端（Vue 响应式，Go 直接托管，无 OCR）                       │
│  浏览器打开域名：浏览/检索/批量编辑/对账                          │
└───────────────┬────────────────────────────────────────────────┘
                │ HTTPS（REST + WS）
┌───────────────┴────────────────────────────────────────────────┐
│  iOS 快捷指令（无 App）                                          │
│  截图 → 系统 OCR 提取文本 → POST parse-text（仅文本）→ 确认入库   │
└───────────────┬────────────────────────────────────────────────┘
                │
                ▼
┌──────────────────────────────┐        ┌──────────────────┐
│   Caddy 反向代理（自动TLS）   │        │  云端 LLM API     │
│   + DDNS 域名                 │        │ （豆包/通义等）    │
└──────────────┬───────────────┘        └──────────────────┘
                ▼
┌──────────────────────────────────────────────────────────┐
│                    Go 后端（Gin）                          │
│  Middlewares: RequestID / 日志 / JWT鉴权 / 限流 / 恢复     │
│  ┌────────────────────────────────────────────────────┐  │
│  │ 静态托管：Web 端构建产物（go:embed）                  │  │
│  │ API 层（Handler）：参数绑定、调用 Service            │  │
│  │ Service 层：账本/成员/交易/账户/分类/同步 业务逻辑    │  │
│  │ Repository 层：xorm 数据访问                          │  │
│  │ Hub 层：WebSocket 连接管理 + 消息广播               │  │
│  └────────────────────────────────────────────────────┘  │
└───────────────┬──────────────────────────────────────────┘
                ▼
        ┌────────────────┐
        │  SQLite 数据文件 │（NAS 持久化卷，每日备份）
        └────────────────┘
```

> **关键**：Web 端通过 `go:embed` 打进 Go 二进制，不增加部署单元，仍是单镜像；iOS 快捷指令完全复用后端 API，不写客户端代码。

## 3.2 后端分层（Go）

```
cmd/            CLI 入口（server run / db migrate / userdata）
internal/
├── config/     配置加载（ini + 环境变量）
├── server/     Gin 初始化、路由注册、Web 静态文件托管（embed）
├── middleware/ 鉴权、日志、限流、CORS、恢复
├── api/        Handler（HTTP 适配层）
├── service/    业务逻辑（事务、权限校验、余额计算）
├── repository/ 数据访问（xorm）
├── models/     数据库实体 + DTO
├── ws/         WebSocket Hub、连接、广播
├── recognition/ 规则引擎、LLM 客户端、文本结构化
└── core/       通用工具、错误码、Context
webfrontend/    Web 端 Vue 工程（构建产物 dist/ 由 go:embed 进二进制）
```

## 3.3 客户端架构（Flutter）

```
lib/
├── main.dart
├── app/                 路由、主题、桌面/移动适配
├── core/
│   ├── network/         dio(HTTP) + web_socket_channel
│   ├── db/              Drift 本地数据库（离线缓存）
│   └── sync/            离线队列、增量同步引擎
├── features/
│   ├── auth/  ledger/  transaction/  account/  category/
│   │   ├── data/   (repository、api、本地源)
│   │   ├── domain/ (entity、usecase)
│   │   └── presentation/ (page、widget、controller)
│   └── recognition/
│       ├── ocr/        ML Kit(Android) / Windows OCR 平台通道
│       ├── capture/    截图、悬浮球、全局热键
│       └── rules/      端侧规则引擎
└── shared/             通用组件、工具
```

## 3.4 识别架构（三级漏斗，隐私优先）

| 级别 | 位置 | 处理 | 数据是否上传 |
|---|---|---|---|
| ① OCR | 端侧 | 图片→文字（Android ML Kit / Windows OCR / iOS 快捷指令"提取文本"） | 否，完全离线 |
| ② 规则提取 | 端侧/服务端 | 微信/支付宝/银行模板正则（Flutter 端侧本地跑；iOS 快捷指令直接走服务端 parse-text） | 否，完全离线 |
| ③ LLM 兜底 | 云端 | 仅发送 OCR **文本**，结构化提取 | 仅文本，无图片 |
| 人工确认 | 端侧 | 用户确认/修正（Flutter 弹窗 / iOS 快捷指令确认动作） | 确认后只传结构化数据 |

**规则引擎覆盖的模板**（MVP 先做两类，命中率最高）：
- 微信支付/账单截图（"二维码收款/付款给/微信支付"等关键词 + 金额格式）
- 支付宝账单截图（"支付宝/付款成功/交易成功"等）
- 规则用"关键词锚点 + 金额正则 + 时间正则 + 商户位置"组合，按平台分文件，**可热更新（规则 JSON 化）**。

## 3.5 实时同步与离线

### 全局版本号机制（增量同步的基石）

每个账本在 `book_versions` 表维护一个**单调递增的全局版本号**，规则：

1. **任何写事务**（新增/修改/删除 交易、账户、分类、标签、持仓等）在同一个数据库事务内：
   - 先对 `book_versions` 做**原子递增**并拿到新版本号 `newVersion`；
   - 再把 `newVersion` 写入本次变更实体的 `sync_version` 字段。
2. 客户端本地记录"我已同步到的版本号" `lastSyncVersion`；调 `GET /sync/pull?afterVersion={lastSyncVersion}`：
   - 服务端分别查各实体表 `WHERE book_id=? AND sync_version > ?`，按 `sync_version` 升序合并返回；
   - 客户端应用后更新本地 `lastSyncVersion` 为返回的最大值。
3. 版本号在**账本级**递增（不按用户/设备），保证同一账本所有成员看到一致的变更顺序。

SQLite 原子递增（在事务内，配合 `busy_timeout` 自动重试）：
```sql
INSERT INTO book_versions(book_id, version, updated_at) VALUES(?, 1, ?)
  ON CONFLICT(book_id) DO UPDATE SET version = version + 1, updated_at = excluded.updated_at;
SELECT version FROM book_versions WHERE book_id = ?;   -- 取 newVersion 回写到实体
```

> 为什么不用实体自增 id 做增量：自增 id 只在"新增"时变大，**修改/删除不会变**，且离线端可能先建了临时 id，无法靠它判断"我缺了哪些变更"。全局版本号 + 实体 `sync_version` 才能覆盖增、改、删。

### 实时推送与离线

- **WebSocket**：客户端连接后按 `book_id` 订阅房间；后端把每次变更（含 `newVersion`）广播给同账本在线成员，在线成员收到即更新并推进 `lastSyncVersion`。
- **离线**：Flutter 本地 Drift 库记录"待同步队列"，每条带 `clientUuid`（幂等）；联网后按队列调 `POST /sync/push`，服务端用 `clientUuid` 去重（重复提交不产生第二笔）。
- **断线重连**：重连后先 `sync/pull` 补齐离线期间错过的变更，再走 WS。
- **冲突策略（已确定）**：以"最后修改 + 版本号"为准——服务端是权威版本，端侧 push 时若该实体服务端版本已更新（`sync_version` 更大），返回冲突，让用户选择"覆盖 / 放弃 / 另存副本"（家庭场景概率极低，不做复杂 CRDT）。

---

# 环节 4：数据库设计

## 4.1 账户与支付渠道建模（解决"微信/支付宝/信用卡账单混乱"）

### 问题根源

微信/支付宝在不同场景下扮演两个角色：
- **钱包角色**（零钱/余额）：你真的把钱充值进去了，是一个**资产账户**；
- **通道角色**（快捷支付）：只是支付工具，钱直接从绑定的银行卡/信用卡扣，**没有进入**微信/支付宝。

把"通道"也当成"账户"，同一笔钱就在微信和信用卡各算一次 → 重复、混乱。

### 核心方案：区分两类东西

**资金账户（Account，真正持有/欠的钱）**
- 资产类：现金、储蓄卡、微信零钱、支付宝余额、投资账户、实物资产（详见 4.2）
- 负债类：信用卡、花呗/白条、房贷、车贷（详见 4.2）

**支付渠道（Payment Channel，一个标签，不是账户）**
- 微信、支付宝、云闪付、POS 刷卡、直接支付

### 一笔交易的正确记录

```
支出 100 元餐饮
  - 资金账户 account_id：招商银行信用卡   ← 钱实际从这出
  - 支付渠道 payment_channel：微信         ← 用什么工具付的
  - 分类：餐饮
```

### 月度账单（用你的例子，一个月内）

| # | 交易 | 分类 | 资金账户 | 支付渠道 |
|---|---|---|---|---|
| 1 | 微信绑信用卡付餐饮 | 餐饮 100 | 信用卡 | 微信 |
| 2 | 支付宝绑同一张信用卡付超市 | 超市 50 | 信用卡 | 支付宝 |
| 3 | 微信零钱付打车 | 交通 20 | 微信零钱 | 微信 |

月度账单有三个视角，**互不重复**：
- **按资金账户（和银行对账）**：信用卡应还 = 150（与银行账单一致）；微信零钱支出 20；
- **按支出分类（看花哪了）**：餐饮 100、超市 50、交通 20，总支出 170；
- **按支付渠道（行为统计，不影响钱）**：微信 120、支付宝 50。

> 关键：**每笔支出只记一个资金账户**。微信/支付宝在快捷支付时只是渠道标签，不产生独立支出。

### 还款怎么记（避免双重消费）

信用卡还款**不是支出**，是**转账**：储蓄卡 → 信用卡 150（资产减少、负债减少）。这 150 在消费时已经记过支出，还款不能再记一次。

### 识别与默认逻辑

- 截图识别"微信支付"→ `payment_channel` 自动填微信；
- 但 `account_id` 要用户在确认时选实际扣款账户（默认"微信零钱"，可改成"信用卡"）；
- 系统记住"该渠道常用账户"，下次智能默认。

### 进阶（v2 可选）：复式记账

若要做完整资产负债/对账，升级为复式记账（Beancount/GnuCash 做法）：每笔交易有"源账户"和"目标账户"——支出（源=信用卡，目标=费用）、转账（源=储蓄卡，目标=信用卡）。家庭轻量记账用"单笔交易 + 资金账户 + 支付渠道"已足够，不必上完整复式。

## 4.2 投资账户与家庭资产建模（骨架先行）

### 核心概念：投资账户是"容器"，持仓标的是"内容"

一个券商账户（如"华泰证券"）是**投资账户**，里面有：
- 未投资的**现金余额**（account.balance）
- 若干**持仓**：贵州茅台 100 股、沪深300ETF 1000 份、某基金 500 份

所以投资账户总市值 = 现金余额 + 持仓市值。不能把"券商账户"当成一个只填余额的普通账户，否则无法体现里面的股票/基金。

### 账户分类（AccountCategory 完整枚举，骨架一次定好）

```
资产类：
  1 现金（含外币现金）
  2 活期储蓄
  3 定期存款（预留）
  4 电子钱包（微信零钱/支付宝余额）
  5 投资账户（券商/基金/债券/贵金属账户）
  6 实物资产（实物黄金/房产/汽车/贵重物品）
  7 保险/年金（预留）
  8 其他资产
  9 应收款（预留）
负债类：
  10 信用卡
  11 信用消费贷（花呗/白条）
  12 房贷
  13 车贷
  14 其他借款
  15 应付款（预留）
```

### 新增三张表（MVP 建表，支持手动录入；v2 启用自动行情）

**① instruments 标的/证券表**（股票、基金、债券、黄金的定义）
```sql
CREATE TABLE instruments (
  id            INTEGER PRIMARY KEY,
  book_id       INTEGER NOT NULL,
  instrument_type TINYINT NOT NULL,       -- 1=股票 2=基金 3=债券 4=贵金属 5=外汇 6=其他
  market        TINYINT DEFAULT 0,        -- 1=A股 2=港股 3=美股 4=场外基金 ...
  symbol        VARCHAR(32) NOT NULL,     -- 证券代码，如 600519
  name          VARCHAR(128) NOT NULL,
  currency      VARCHAR(3) DEFAULT 'CNY',
  quote_symbol  VARCHAR(48) DEFAULT '',   -- 行情代码，用于自动更新（v2）
  deleted       BOOLEAN DEFAULT 0,
  created_at    INTEGER, updated_at INTEGER, deleted_at INTEGER
);
```

**② holdings 持仓表**（某投资账户持有某标的）
```sql
CREATE TABLE holdings (
  id            INTEGER PRIMARY KEY,
  book_id       INTEGER NOT NULL,
  account_id    INTEGER NOT NULL,          -- 所属投资账户
  instrument_id INTEGER NOT NULL,
  quantity      VARCHAR(48) NOT NULL,      -- 数量（高精度，用字符串存，避免股票/基金份额精度问题）
  cost_amount   BIGINT DEFAULT 0,          -- 总成本（分）
  current_price VARCHAR(48) DEFAULT '',    -- 最新价
  market_value  BIGINT DEFAULT 0,          -- 最新市值（分）
  updated_at    INTEGER,
  UNIQUE(account_id, instrument_id)
);
```

**③ instrument_prices 行情价格表**（价格历史，自动更新写入）
```sql
CREATE TABLE instrument_prices (
  id            INTEGER PRIMARY KEY,
  instrument_id INTEGER NOT NULL,
  price         VARCHAR(48) NOT NULL,
  price_time    INTEGER NOT NULL,
  source        VARCHAR(32) DEFAULT '',
  created_at    INTEGER
);
CREATE INDEX idx_instr_price ON instrument_prices(instrument_id, price_time);
```

### 各类资产怎么记、怎么估值

| 资产 | 怎么记 | 市值来源 |
|---|---|---|
| A 股/港股（券商） | 投资账户 + 持仓 | v2 自动行情；MVP 手动填最新价 |
| 场外基金 | 投资账户 + 持仓（基金标的） | v2 天天基金等净值接口 |
| 实物黄金 | 实物资产账户（可关联贵金属标的） | 可手动，或 v2 金价自动 |
| 外币现金 | 现金账户（外币） | 汇率折算 |
| 汽车 / 房子 | 实物资产账户，填估值 | 手动估值（折旧/市价） |
| 房贷 / 车贷 | 负债账户，填剩余欠款 | 手动更新 |

### 家庭资产合计（统一折算本币）

```
家庭总资产 =
    Σ 现金/储蓄/钱包余额（外币 × 汇率）
  + Σ 投资账户（现金余额 + 持仓市值）
  + Σ 实物资产估值
  + …

家庭总负债 = Σ 信用卡/房贷/车贷/消费贷欠款
家庭净资产 = 总资产 − 总负债
```

### 行情自动更新（v2/v3，骨架 = Provider 接口）

仿照 LLM provider 抽象，定义统一行情源接口：
```go
type QuoteProvider interface {
    GetQuotes(symbols []QuoteSymbol) ([]Quote, error)
}
```
- 实现：东方财富 / 新浪 / 腾讯（A股、港股、基金净值、黄金）；
- cron 定时任务：拉取所有持仓标的最新价 → 写 instrument_prices → 更新 holdings 市值 → 刷新资产合计；
- MVP 只定义接口和表，v2 填实现并自动跑，无需改表。

### 投资交易（v2）

买入/卖出/分红/利息在 v2 处理：复用 transactions 扩展投资类型（买入/卖出/分红/利息），据此自动调整持仓数量与成本。MVP 先用手动持仓录入，不影响骨架。

## 4.3 分类层级、账单导出与预算（参考业界开源）

### ① 两级分类（大类 → 小类）

参考 ezBookkeeping：`categories.parent_id` 实现层级（已预留），一级大类 `parent_id=0`，二级小类指向大类。

```
旅游（大类，parent_id=0）
 ├─ 旅游交通（小类，parent_id=旅游.id）
 ├─ 旅游住宿（小类）
 └─ 旅游门票（小类）
```
- 记账时可只选大类，或选到小类；
- 统计按大类自动汇总其下所有小类；
- 分类带类型（收入/支出）、排序、图标、颜色、隐藏；
- 骨架 `parent_id` 支持未来多级，UI 先做两级。

### ② 账单导出

参考 ezBookkeeping（CSV/TSV）、Firefly III：
- **格式**：CSV（Excel 可直接打开，带 UTF-8 BOM 防乱码）、JSON（结构化，便于迁移/备份）；v2 再加 Excel(xlsx)、PDF 报表；
- **范围筛选**：时间范围、账本、账户、分类、交易类型；
- **字段**：交易时间、类型、金额、大类、小类、资金账户、支付渠道、备注、标签、创建人；
- 大数据量流式生成、分批查询，避免一次性载入内存。

### ③ 预算（参考 Firefly III 的信封式预算）

Firefly III 的预算本质：给"某类支出"设"周期 + 金额上限"，月初分配、随支出减少；支持未用结转、超支追踪、多币种。

FastLedger 从简单的"分类月度预算"做起：
- **分类预算（v0.2）**：给支出大类设月度预算（如餐饮 3000、旅游 2000）；
  - 预算进度 = 已支出 ÷ 预算，进度条；
  - 预警：达 80% 提醒、超支红色；
- **总预算（可选）**：本月总支出上限；
- 周期先做月度（骨架 `period_type` 预留周/季/年）；
- 高级（v2）：未用结转（rollover）、超支滚入下月、多币种。

预算执行实时计算：当月该分类支出汇总 vs 预算金额，无需额外存"已用额度"。

## 4.4 ER 关系（核心）

```
users ──< book_members >── books
                              │
        ┌──────────────┬──────┼──────────┬──────────┐
        ▼              ▼      ▼          ▼          ▼
   accounts    categories  transactions  tags    members设置
                       │       │
                       │       └──> transaction_tag_index <── tags
                       │
                       └── 转账：transactions.related_id 自关联
```

**核心隔离原则**：
- 所有业务表都带 **`book_id`**（账本是数据隔离边界）。
- 账户/分类/标签**属于账本**（家庭共享）。
- 交易记录带 **`creator_uid`**（谁记的），配合账本可见性开关做过滤。
- 所有表**软删除** + 时间戳；金额 **`BIGINT` 存分**。

## 4.5 完整建表 SQL（SQLite 方言，xorm 也可自动建）

```sql
-- 用户
CREATE TABLE users (
  id              INTEGER PRIMARY KEY,
  username        VARCHAR(32)  NOT NULL UNIQUE,
  email           VARCHAR(100) DEFAULT NULL,   -- 可选：家庭自托管常无 SMTP，不强制邮箱
  password_hash   VARCHAR(128) NOT NULL,        -- bcrypt/argon2 哈希，算法已内含随机盐，无需独立 salt 列
  nickname        VARCHAR(64)  NOT NULL,
  avatar_type     TINYINT      DEFAULT 0,
  language        VARCHAR(10)  DEFAULT 'zh-CN',
  disabled        BOOLEAN      DEFAULT 0,
  deleted         BOOLEAN      DEFAULT 0,
  email_verified  BOOLEAN      DEFAULT 0,       -- 仅在配置了 SMTP 时才启用验证
  created_at      INTEGER,
  updated_at      INTEGER,
  deleted_at      INTEGER,
  last_login_at   INTEGER
);
-- 仅对非空邮箱做唯一约束（SQLite 中多个 NULL 不冲突，多个 '' 会冲突，故无邮箱用 NULL）
CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL;

-- 账本
CREATE TABLE books (
  id               INTEGER PRIMARY KEY,
  owner_uid        INTEGER NOT NULL,
  name             VARCHAR(64) NOT NULL,
  description      VARCHAR(255) DEFAULT '',
  book_type        TINYINT DEFAULT 1,        -- 1=个人 2=家庭（预留）
  default_currency VARCHAR(3) DEFAULT 'CNY', -- 多币种预留
  icon             VARCHAR(64) DEFAULT '',
  members_can_view_others   BOOLEAN DEFAULT 1,  -- 可见性开关
  members_can_edit_others   BOOLEAN DEFAULT 0,  -- 编辑权限预留
  archived         BOOLEAN DEFAULT 0,
  deleted          BOOLEAN DEFAULT 0,
  created_at       INTEGER,
  updated_at       INTEGER,
  deleted_at       INTEGER
);

-- 账本级全局版本计数器（增量同步的核心：每个账本一行，任何写事务内原子 +1）
CREATE TABLE book_versions (
  book_id     INTEGER PRIMARY KEY,
  version     INTEGER NOT NULL DEFAULT 0,   -- 该账本当前全局版本号，单调递增
  updated_at  INTEGER
);

-- 账本成员
CREATE TABLE book_members (
  id            INTEGER PRIMARY KEY,
  book_id       INTEGER NOT NULL,
  uid           INTEGER NOT NULL,
  role          TINYINT NOT NULL,           -- 1=owner 2=admin 3=member
  can_view_others  BOOLEAN DEFAULT 1,        -- 成员级覆盖（预留）
  joined_at     INTEGER,
  UNIQUE(book_id, uid)
);
CREATE INDEX idx_book_members_uid ON book_members(uid);

-- 账户（属于账本）
CREATE TABLE accounts (
  id            INTEGER PRIMARY KEY,
  book_id       INTEGER NOT NULL,
  parent_id     INTEGER DEFAULT 0,          -- 仅用于 UI 分组（如"信用卡组"下挂多张卡），0=顶级；不参与余额计算，MVP 可先不启用
  category      TINYINT NOT NULL,           -- 账户分类，见 AccountCategory 枚举（1~15）
  type          TINYINT DEFAULT 0,          -- 0=系统预置 1=自定义
  name          VARCHAR(64) NOT NULL,
  currency      VARCHAR(3) DEFAULT 'CNY',
  balance       BIGINT DEFAULT 0,           -- 单位：分
  icon          VARCHAR(64) DEFAULT '',
  color         VARCHAR(8) DEFAULT '',
  sort_order    INTEGER DEFAULT 0,
  hidden        BOOLEAN DEFAULT 0,
  sync_version  INTEGER DEFAULT 0,          -- 账本级全局版本号（见 book_versions）
  deleted       BOOLEAN DEFAULT 0,
  created_at    INTEGER,
  updated_at    INTEGER,
  deleted_at    INTEGER
);
CREATE INDEX idx_accounts_book ON accounts(book_id, deleted, parent_id, sort_order);

-- 分类（属于账本）
CREATE TABLE categories (
  id          INTEGER PRIMARY KEY,
  book_id     INTEGER NOT NULL,
  parent_id   INTEGER DEFAULT 0,            -- 0=大类，否则为所属大类 id；骨架支持未来多级
  cat_type    TINYINT NOT NULL,             -- 1=收入 2=支出
  name        VARCHAR(64) NOT NULL,
  icon        VARCHAR(64) DEFAULT '',
  color       VARCHAR(8) DEFAULT '',
  sort_order  INTEGER DEFAULT 0,
  hidden      BOOLEAN DEFAULT 0,
  sync_version INTEGER DEFAULT 0,           -- 账本级全局版本号（见 book_versions）
  deleted     BOOLEAN DEFAULT 0,
  created_at  INTEGER,
  updated_at  INTEGER,
  deleted_at  INTEGER
);
CREATE INDEX idx_categories_book ON categories(book_id, deleted, cat_type, sort_order);

-- 交易
CREATE TABLE transactions (
  id                INTEGER PRIMARY KEY,
  book_id           INTEGER NOT NULL,
  creator_uid       INTEGER NOT NULL,
  tx_type           TINYINT NOT NULL,        -- 1=收入 2=支出 3=转账
  category_id       INTEGER DEFAULT 0,
  account_id        INTEGER NOT NULL,        -- 资金账户（实际扣款）
  payment_channel   TINYINT DEFAULT 0,       -- 支付渠道：0=直接 1=微信 2=支付宝 3=云闪付 4=POS刷卡
  amount            BIGINT NOT NULL,         -- 单位：分
  tx_time           INTEGER NOT NULL,        -- unix 秒
  timezone_offset   INTEGER DEFAULT 0,       -- 时区偏移（分钟）
  related_id        INTEGER DEFAULT 0,       -- 转账对侧交易 id
  related_account_id INTEGER DEFAULT 0,
  related_amount    BIGINT DEFAULT 0,
  comment           VARCHAR(255) DEFAULT '',
  client_uuid       VARCHAR(64) DEFAULT '',  -- 端侧幂等去重
  sync_version      INTEGER DEFAULT 0,       -- 该行最后变更时的账本级全局版本号（见 book_versions），用于增量同步
  source            TINYINT DEFAULT 0,       -- 0=手动 1=识别 2=导入(预留)
  scheduled         BOOLEAN DEFAULT 0,       -- 定时交易预留
  deleted           BOOLEAN DEFAULT 0,
  created_at        INTEGER,
  updated_at        INTEGER,
  deleted_at        INTEGER
);
CREATE INDEX idx_tx_book_time   ON transactions(book_id, deleted, tx_time);
CREATE INDEX idx_tx_book_type   ON transactions(book_id, deleted, tx_type, tx_time);
CREATE INDEX idx_tx_book_acct   ON transactions(book_id, deleted, account_id, tx_time);
CREATE INDEX idx_tx_book_cat    ON transactions(book_id, deleted, category_id, tx_time);
CREATE INDEX idx_tx_creator     ON transactions(book_id, creator_uid, deleted, tx_time);

-- 标签（MVP 可建表不启用）
CREATE TABLE tags (
  id INTEGER PRIMARY KEY, book_id INTEGER NOT NULL,
  name VARCHAR(64) NOT NULL, color VARCHAR(8) DEFAULT '',
  deleted BOOLEAN DEFAULT 0, created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE TABLE transaction_tag_index (
  tx_id INTEGER NOT NULL, tag_id INTEGER NOT NULL, UNIQUE(tx_id, tag_id)
);

-- 认证 token
CREATE TABLE auth_tokens (
  id INTEGER PRIMARY KEY, uid INTEGER NOT NULL,
  token_type TINYINT NOT NULL,                 -- 1=登录 2=API 3=WS
  token_hash VARCHAR(128) NOT NULL,
  expires_at INTEGER, revoked BOOLEAN DEFAULT 0, created_at INTEGER
);

-- 识别/导入暂存（预留）
CREATE TABLE imported_transactions (
  id INTEGER PRIMARY KEY, book_id INTEGER NOT NULL, uid INTEGER NOT NULL,
  raw_text TEXT, parsed_json TEXT, status TINYINT DEFAULT 0, created_at INTEGER
);

-- 投资标的/持仓/行情（MVP 建表，手动录入；v2 自动行情）
CREATE TABLE instruments (
  id INTEGER PRIMARY KEY, book_id INTEGER NOT NULL,
  instrument_type TINYINT NOT NULL, market TINYINT DEFAULT 0,
  symbol VARCHAR(32) NOT NULL, name VARCHAR(128) NOT NULL,
  currency VARCHAR(3) DEFAULT 'CNY', quote_symbol VARCHAR(48) DEFAULT '',
  deleted BOOLEAN DEFAULT 0, created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE TABLE holdings (
  id INTEGER PRIMARY KEY, book_id INTEGER NOT NULL,
  account_id INTEGER NOT NULL, instrument_id INTEGER NOT NULL,
  -- quantity/current_price 用字符串存高精度十进制；Go 内部必须用 shopspring/decimal 运算，
  -- 严禁 float32/64（股票/基金份额会出现精度误差）；写入前校验为合法十进制字符串
  quantity VARCHAR(48) NOT NULL, cost_amount BIGINT DEFAULT 0,
  current_price VARCHAR(48) DEFAULT '0', market_value BIGINT DEFAULT 0,
  updated_at INTEGER, UNIQUE(account_id, instrument_id)
);
CREATE TABLE instrument_prices (
  id INTEGER PRIMARY KEY, instrument_id INTEGER NOT NULL,
  price VARCHAR(48) NOT NULL, price_time INTEGER NOT NULL,
  source VARCHAR(32) DEFAULT '', created_at INTEGER
);
CREATE INDEX idx_instr_price ON instrument_prices(instrument_id, price_time);

-- 预算（v0.2 启用；表骨架先建）
CREATE TABLE budgets (
  id INTEGER PRIMARY KEY, book_id INTEGER NOT NULL,
  name VARCHAR(64) NOT NULL,
  budget_type TINYINT NOT NULL,              -- 1=分类预算 2=总预算
  category_id INTEGER DEFAULT 0,             -- 分类预算关联的大类
  period_type TINYINT DEFAULT 1,             -- 1=月 2=季 3=年（预留周）
  amount BIGINT NOT NULL,                    -- 预算金额（分）
  rollover BOOLEAN DEFAULT 0,                -- 未用是否结转下期
  enabled BOOLEAN DEFAULT 1,
  deleted BOOLEAN DEFAULT 0, created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE INDEX idx_budget_book ON budgets(book_id, deleted);
```

## 4.6 扩展能力设计说明

- **枚举值都留余量**：`tx_type`、`role`、`account.category` 用 TINYINT，新增类型不改表结构。
- **多币种**：`books.default_currency` + `accounts.currency` 已就位，后期加汇率表即可。
- **标签/预算/定时/图片**：表已建或字段已留，后期启用不改核心表。
- **可见性双粒度**：账本级开关 + 成员级覆盖字段，支持复杂权限演进。
- **同步幂等**：`client_uuid` + `version` 支撑离线和多端，不依赖自增 ID 做同步。

---

# 环节 5：API 与详细设计

## 5.1 REST API 契约（统一前缀 `/api/v1`，JWT 鉴权）

| 模块 | 方法 & 路径 | 说明 |
|---|---|---|
| 认证 | POST `/auth/register` | 注册（可配置是否开放） |
|  | POST `/auth/login` | 登录，返回 accessToken/refreshToken |
|  | POST `/auth/refresh` | 刷新 token |
|  | POST `/auth/logout` | 退出 |
|  | POST `/tokens/generate-api` | 生成**长期 API Token**（供 iOS 快捷指令、外部脚本使用，可吊销） |
| 用户 | GET `/users/me` | 当前用户资料 |
| 账本 | GET `/books` | 我加入的账本列表 |
|  | POST `/books` | 创建账本（创建者自动为 owner） |
|  | PUT `/books/{id}` | 修改账本（名称/可见性开关） |
|  | GET `/books/{id}/members` | 成员列表 |
|  | POST `/books/{id}/members/invite` | 生成邀请码/链接 |
|  | POST `/books/{id}/members/join` | 凭邀请码加入 |
|  | PUT `/books/{id}/members/{uid}` | 改角色/权限 |
|  | DELETE `/books/{id}/members/{uid}` | 移除成员 |
| 账户 | GET `/books/{id}/accounts` | 账户列表 |
|  | POST `/books/{id}/accounts` | 新增账户 |
|  | PUT `/books/{id}/accounts/{aid}` | 修改 |
|  | DELETE `/books/{id}/accounts/{aid}` | 删除/隐藏 |
| 分类 | GET `/books/{id}/categories` | 分类列表（含 parent_id，两级层级） |
|  | POST/PUT/DELETE `/books/{id}/categories...` | 分类管理（大类/小类） |
| 交易 | GET `/books/{id}/transactions` | 交易列表（分页/过滤，参数：maxTime,minTime,type,accountId,categoryId,keyword,page,count） |
|  | GET `/books/{id}/transactions/{txId}` | 详情 |
|  | POST `/books/{id}/transactions` | 新增（支持单条/批量） |
|  | PUT `/books/{id}/transactions/{txId}` | 修改 |
|  | DELETE `/books/{id}/transactions/{txId}` | 软删除 |
| 导出 | GET `/books/{id}/transactions/export` | **导出账单**：参数 format=csv/json、startTime/endTime/accountId/categoryId/type；返回文件流（CSV 带 BOM） |
| 识别 | POST `/books/{id}/recognition/parse-text` | **仅传 OCR 文本**，返回结构化结果（LLM 兜底） |
| 标的 | GET `/books/{id}/instruments` | 标的列表（可按 type/market 过滤） |
|  | POST `/books/{id}/instruments` | 新增标的 |
|  | PUT `/books/{id}/instruments/{iid}` | 修改 |
|  | DELETE `/books/{id}/instruments/{iid}` | 软删除（被持仓引用时拒绝） |
| 持仓 | GET `/books/{id}/accounts/{aid}/holdings` | 某投资账户的持仓列表 |
|  | POST `/books/{id}/accounts/{aid}/holdings` | 新增持仓（传 instrumentId/quantity/costAmount/currentPrice） |
|  | PUT `/books/{id}/holdings/{hid}` | 修改（如手动更新最新价） |
|  | DELETE `/books/{id}/holdings/{hid}` | 删除 |
| 资产 | GET `/books/{id}/assets/summary` | **家庭资产合计**：总资产/总负债/净资产、按分类汇总（v0.1 手动数据） |
| 预算 | GET `/books/{id}/budgets` | 预算列表（含当期已用、进度、是否超支） |
|  | POST `/books/{id}/budgets` | 新增预算（分类预算/总预算、周期、金额） |
|  | PUT `/books/{id}/budgets/{bid}` | 修改 |
|  | DELETE `/books/{id}/budgets/{bid}` | 删除 |
| 同步 | GET `/books/{id}/sync/pull` | 拉取增量（参数：afterVersion） |
|  | POST `/books/{id}/sync/push` | 推送离线队列（幂等 client_uuid） |

**统一响应包**：

```json
{ "code": 0, "msg": "ok", "data": { } }
```
错误时 `code` 为业务错误码，HTTP 状态码与错误码语义一致（401/403/404/422/429/500）。

## 5.2 WebSocket 协议

- 连接：`wss://域名/ws?token=xxx`，连接后发送 `{"action":"subscribe","bookId":1}`。
- 服务端推送事件：

```json
{ "event": "transaction.created", "bookId": 1, "data": { ...交易对象 } }
{ "event": "transaction.updated", "bookId": 1, "data": { ... } }
{ "event": "transaction.deleted", "bookId": 1, "data": { "id": 123 } }
{ "event": "member.role_changed", "bookId": 1, "data": { ... } }
```

- 客户端只处理"当前已订阅账本"的事件；断线自动重连，重连后走 `sync/pull` 补漏。

## 5.3 识别接口的请求/响应（关键）

请求（**只发文本，不发图**）：
```json
{ "text": "微信支付\n付款给 张三\n¥ 36.50\n2026-10-28 12:30", "hint": "" }
```
响应：
```json
{
  "txType": 2, "amount": 3650, "merchant": "张三",
  "txTime": 1782621000, "paymentChannel": 1, "accountId": 0,
  "categoryId": 0, "confidence": 0.92, "rawText": "..."
}
```
> `paymentChannel` 由截图识别（微信/支付宝），`accountId` 为建议的资金账户，最终由用户确认（快捷支付时要改成实际扣款的信用卡/储蓄卡）。

## 5.4 投资接口的请求/响应（v0.1 手动录入）

新增标的：
```json
POST /books/{id}/instruments
{ "instrumentType": 1, "market": 1, "symbol": "600519",
  "name": "贵州茅台", "currency": "CNY", "quoteSymbol": "" }
```

新增持仓（市值/盈亏由后端计算返回）：
```json
POST /books/{id}/accounts/{aid}/holdings
{ "instrumentId": 12, "quantity": "100", "costAmount": 16000000, "currentPrice": "1650" }
```
响应：
```json
{ "id": 5, "instrumentId": 12, "quantity": "100", "costAmount": 16000000,
  "currentPrice": "1650", "marketValue": 16500000,
  "profitLoss": 500000, "profitLossRate": 0.03125 }
```

资产合计：
```json
GET /books/{id}/assets/summary
{
  "currency": "CNY",
  "totalAssets": 537000000, "totalLiabilities": 205000000, "netAssets": 332000000,
  "assetByType": [
    { "category": 1, "name": "现金/储蓄", "amount": 10500000 },
    { "category": 5, "name": "投资账户", "amount": 23500000 },
    { "category": 6, "name": "实物资产", "amount": 518000000 }
  ],
  "liabilityByType": [
    { "category": 12, "name": "房贷", "amount": 200000000 },
    { "category": 13, "name": "车贷", "amount": 5000000 }
  ],
  "quoteMode": "manual"
}
```
> 金额均为整数分；`quoteMode` 在 v0.1 为 `manual`，v0.3 自动行情后为 `auto`。

## 5.5 权限校验矩阵

| 操作 | owner | admin | member |
|---|---|---|---|
| 改账本设置/删除账本 | ✅ | ❌ | ❌ |
| 邀请/移除成员、改角色 | ✅ | ✅（不能动 owner） | ❌ |
| 增删改自己的交易 | ✅ | ✅ | ✅ |
| 查看他人交易 | 受账本开关控制（默认可见） | 同左 | 同左 |
| 改/删他人交易 | ✅ | 可配置 | 默认不可 |
| 管理标的/持仓/资产 | ✅ | ✅ | 自己的持仓 |
| 查看家庭资产合计 | ✅ | ✅ | 受账本开关控制 |

---

# 环节 6：编码实现

## 6.1 关键代码骨架（Go）

**路由 + 分层调用**：
```go
// internal/server/router.go
func Register(r *gin.Engine, h *api.Handlers) {
    api := r.Group("/api/v1")
    api.Use(mw.RequestID(), mw.Recovery(), mw.RequestLog())

    auth := api.Group("")
    auth.POST("/auth/login", h.Auth.Login)
    auth.POST("/auth/register", h.Auth.Register)

    v1 := api.Group("")
    v1.Use(mw.JWT(container))
    v1.GET("/books", h.Book.List)
    v1.POST("/books", h.Book.Create)
    // 交易
    v1.GET("/books/:id/transactions", h.Transaction.List)
    v1.POST("/books/:id/transactions", h.Transaction.Create)
    // 识别（仅文本）
    v1.POST("/books/:id/recognition/parse-text", h.Recognition.ParseText)

    // Web 端静态托管（go:embed 打进二进制）
    r.Use(StaticWebServer(embedFS))   // 非 /api 路由返回 Web 端页面
}
```

**Web 端嵌入（Go 1.16+）**：
```go
// internal/server/web_embed.go
import "embed"

//go:embed all:webassets
var embedFS embed.FS
```
> Web 前端构建产物输出到 `webassets/`，编译时一起打进二进制；部署仍是单文件、单镜像。

**Service 事务（递增版本号 + 新增交易 + 原子改余额 + 转账对侧）**：
```go
func (s *TxService) Create(c core.Context, bookID int64, req *models.Transaction) error {
    return s.repo.Tx(c, func(sess *xorm.Session) error {
        // 1) 账本级全局版本号 +1，拿到 newVersion（与本事务同生共死）
        newVersion, err := s.version.Next(sess, bookID)
        if err != nil {
            return err
        }
        req.SyncVersion = newVersion

        // 2) 插入交易
        if _, err := sess.Insert(req); err != nil {
            return err
        }
        // 3) 原子更新账户余额（单条 UPDATE，不先读后写，避免并发丢更新）
        if err := s.account.ChangeBalance(sess, req.AccountID, signedAmount(req)); err != nil {
            return err
        }
        // 4) 转账：生成对侧交易（同样带 newVersion）
        if req.TxType == models.TxTransfer {
            if err := s.createRelatedTransfer(sess, req, newVersion); err != nil {
                return err
            }
        }
        // 5) WS 广播（带版本号，放在事务提交后执行更稳妥）
        return s.ws.PublishAfterCommit(bookID, "transaction.created", req, newVersion)
    })
}
```

**版本号递增 + 原子改余额（关键实现）**：
```go
// 账本级全局版本号 +1
func (r *VersionRepo) Next(sess *xorm.Session, bookID int64) (int64, error) {
    now := time.Now().Unix()
    if _, err := sess.Exec(`
        INSERT INTO book_versions(book_id, version, updated_at) VALUES(?, 1, ?)
        ON CONFLICT(book_id) DO UPDATE SET version = version + 1, updated_at = excluded.updated_at`,
        bookID, now); err != nil {
        return 0, err
    }
    var v models.BookVersion
    if _, err := sess.Where("book_id = ?", bookID).Get(&v); err != nil {
        return 0, err
    }
    return v.Version, nil
}

// 账户余额：单条原子 UPDATE，数据库行锁保证不丢更新（禁止先 SELECT 再回写）
func (r *AccountRepo) ChangeBalance(sess *xorm.Session, accountID int64, deltaCents int64) error {
    _, err := sess.Exec(
        "UPDATE accounts SET balance = balance + ?, updated_at = ? WHERE id = ? AND deleted = 0",
        deltaCents, time.Now().Unix(), accountID)
    return err
}
```

**SQLite 连接初始化（建连时执行，务必打开）**：
```go
func (db *xorm.Engine) pragmas() {
    db.DB().Exec("PRAGMA journal_mode=WAL;")       // 读写并发不互锁，大幅降低 SQLITE_BUSY
    db.DB().Exec("PRAGMA busy_timeout=5000;")      // 遇锁等待 5s 自动重试，而非立刻报错
    db.DB().Exec("PRAGMA foreign_keys=ON;")        // 开启外键约束（SQLite 默认关闭）
    db.DB().Exec("PRAGMA synchronous=NORMAL;")     // WAL 下安全且更快
}
```

**首个 owner 初始化（不依赖公开注册）**：
- 首次启动检测到 `users` 表为空时，进入**初始化向导**（网页/客户端显示"创建管理员账户"），或用命令行创建：
  ```bash
  docker exec -it fastledger /app/fastledger user create --username admin --owner
  ```
- 该流程只在"无任何用户"时可用，建好后自动失效；`EBK_ENABLE_REGISTER=false` 不影响它。

## 6.2 关键代码骨架（Flutter）

**Android 端 OCR（ML Kit，离线）**：
```dart
final recognizer = TextRecognizer(script: TextRecognitionScript.chinese);
final result = await recognizer.processImage(InputImage.fromFilePath(imagePath));
final text = result.text;                 // 端侧 OCR 文本
recognizer.close();

final parsed = RuleEngine.match(text);    // 先端侧规则
if (parsed == null) {
  parsed = await api.parseText(text);     // 规则未命中，仅传文本走 LLM
}
showConfirmSheet(parsed);                 // 人工确认
```

**Windows 端 OCR**：通过 MethodChannel 调用系统 `Windows.Media.Ocr`（离线免费）：
```dart
// 原生侧用 WinRT OcrEngine.RecognizeAsync，返回文本
final text = await platform.invokeMethod('ocr', {'path': imagePath});
```

**全局热键（Windows）**：`hotkey_manager` 包注册 `Ctrl+Shift+L` → 触发 `screen_capturer` 框选 → OCR。

**Android 悬浮球**：`overlay_window`/`flutter_overlay_window` 常驻悬浮按钮，点击触发截图（`screenshot` 或无障碍 MediaProjection）。

## 6.3 开发任务拆分（WBS，可独立验证）

| # | 任务 | 端 | 产出验证 |
|---|---|---|---|
| 1 | 项目骨架、配置、日志 | 后端 | 能启动空服务 |
| 2 | 数据库连接、自动建表 | 后端 | 表自动创建 |
| 3 | 注册/登录/JWT 中间件 | 后端 | 拿到 token、受保护接口 401 |
| 4 | 账本 CRUD + 成员/邀请/角色 | 后端 | 接口联调通过 |
| 5 | 账户/分类 CRUD + 默认数据 | 后端 | 初始化默认数据 |
| 6 | 交易 CRUD + 事务 + 余额 + 转账 | 后端 | 余额正确、转账成对 |
| 7 | WebSocket Hub + 广播 | 后端 | 两端消息互通 |
| 8 | 识别接口（LLM 文本兜底） | 后端 | 文本→结构化 |
| 9 | 增量同步 pull/push + 幂等 | 后端 | 离线队列可补 |
| 9.5 | 投资模块（v0.1）：标的/持仓 CRUD + 资产合计（手动） | 后端 | 持仓市值/净资产计算正确 |
| 10 | Flutter 工程骨架、网络、本地 DB | 客户端 | 能登录、能缓存 |
| 11 | 账户/分类/交易页面 | 客户端 | 能增删改查 |
| 11.5 | 投资模块（v0.1）：资产总览、投资账户、标的、持仓、实物资产页面 | 客户端 | 手动录入、资产合计正确 |
| 12 | Windows 热键 + 截图 + OCR | 客户端 | 快捷键出确认弹窗 |
| 13 | Android 悬浮球 + 截图 + ML Kit | 客户端 | 悬浮球出确认弹窗 |
| 14 | 端侧规则引擎（微信/支付宝） | 客户端 | 离线即可识别 |
| 15 | WS 接入 + 离线队列 + 重连补漏 | 客户端 | 多端实时、离线可同步 |
| 16 | Web 端 Vue 工程（浏览/检索/批量编辑/对账）+ go:embed 托管 | Web | 浏览器可用、无 OCR |
| 17 | iOS 客户端（Flutter 浏览/管理）+ 快捷指令 OCR（URL Scheme 回传确认） | iOS | App 可浏览管理、快捷指令识别回传 |
| 18 | 打包、Docker、Caddy 部署 | 全栈 | 外网 HTTPS 可用 |

---

# 环节 7：测试

## 7.1 测试策略

| 层级 | 范围 | 工具 |
|---|---|---|
| 单元测试 | 金额换算、规则引擎、余额计算、权限判断 | Go `testing`；Flutter `test` |
| 集成测试 | 交易事务、转账对侧、同步幂等、WS 广播 | Go test + httptest |
| 端到端 | 截图→识别→确认→入账→他端可见 | 手工 + 固定测试样例 |
| 兼容测试 | 不同 Android 品牌后台/悬浮球、Windows 版本 | 真机/虚拟机 |

## 7.2 核心验收用例（Given-When-Then）

1. **成功识别（端侧规则）**：
   - Given 一张微信支付截图；When 触发识别；Then 离线状态下也能正确得到金额/商户/时间，且不产生任何网络请求。
2. **LLM 兜底只传文本**：
   - Given 一张陌生小票；When 规则未命中；Then 仅 OCR 文本被发送，抓包确认无图片上传。
3. **人工确认底线**：
   - Given 识别结果；Then 必须用户点确认才入库，不存在自动入库路径。
4. **转账一致性**：
   - When 新增一笔转账；Then 两账户余额反向变动、对侧交易生成；删除任一条则两条都失效。
5. **实时同步**：
   - When A 新增交易；Then 在线的 B 在 1 秒内收到事件并刷新。
6. **离线补同步**：
   - Given 断网记 3 笔；When 恢复网络；Then 3 笔通过 client_uuid 幂等提交，不重复。
7. **可见性开关**：
   - When 关闭"成员可见他人"；Then 普通成员只能看到自己的交易，owner 不受影响。
8. **权限**：
   - Then 普通成员无法邀请/移除成员、无法改他人交易（接口返回 403）。
9. **金额精度**：
   - Then 任意金额（含 0.01、大额）无浮点误差，统计合计与明细一致。

## 7.3 测试数据

- 收集 20~30 张真实/脱敏截图（微信、支付宝、银行、各类小票），放进 `testdata/`，作为规则引擎和识别的回归集。

---

# 环节 8：部署、发布与运维

## 8.1 docker-compose.yml

```yaml
version: "3.8"
services:
  fastledger:
    image: fastledger:latest
    container_name: fastledger
    restart: unless-stopped
    environment:
      - EBK_HTTP_PORT=8080
      - EBK_JWT_SECRET=请改成随机长字符串
      - EBK_ENABLE_REGISTER=false        # 初始账号建好后关闭公开注册
      - EBK_LLM_PROVIDER=openai           # 仅文本兜底用
      - EBK_LLM_API_KEY=替换为你的key
      - EBK_LLM_BASE_URL=替换为对应接口
    volumes:
      - ./data:/app/data                 # SQLite 数据库
      - ./conf:/app/conf:ro
    expose:
      - "8080"
    networks: [web]

  caddy:
    image: caddy:2
    container_name: caddy
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - ./caddy_data:/data
    networks: [web]

networks:
  web:
```

**Caddyfile**（自动 HTTPS，把域名换成你的 DDNS 域名）：
```
ledger.your-domain.com {
    reverse_proxy fastledger:8080
}
```

## 8.2 外网访问

- **DDNS**：路由器/群晖/飞牛自带 DDNS，或用阿里云/腾讯云域名 + DDNS 脚本。
- **HTTPS**：Caddy 自动申请并续期 Let's Encrypt 证书（需 80/443 可达）。
- 路由器做 80/443 端口转发到运行 Caddy 的主机。

## 8.3 备份与恢复

- **每日备份**：定时任务复制 `data/*.db`（SQLite 用 `.backup` 或 `VACUUM INTO` 保证一致性），保留 14~30 份。
- 配置和 Caddy 数据一并备份。
- 恢复：停容器 → 替换 db 文件 → 启动。

## 8.4 监控与安全

- 容器 `restart: unless-stopped`；接简单健康检查 `/healthz`。
- 登录失败限流、JWT 过期与刷新、日志集中查看。
- 关闭公开注册、定期更新镜像、仅暴露必要端口。

## 8.5 CI/CD（轻量）

- GitHub Actions：提交 → Go test / Flutter analyze+test → 构建镜像 → 推送 GHCR。
- NAS 上 watchtower 或手动拉取更新；更新前自动备份数据库。

## 8.6 版本 Roadmap（迭代规划）

| 版本 | 目标结果 | 主要内容 | 退出条件 |
|---|---|---|---|
| **v0.1 MVP** | 自己和配偶能日常用、数据自有 | 记账闭环：Flutter Win+Android+iOS（浏览管理，OCR 走快捷指令）、Web 端浏览编辑、共享同步、部署；**两级分类（大类/小类）**；投资表骨架建好、手动录入持仓/资产 | 连续自用 2 周无阻断性问题 |
| v0.2 | 看得清、管得细、能预算 | 图表统计、月度报表、标签、图片附件；**账单导出（CSV/JSON）**；**分类月度预算（进度/超支预警）** | 报表与明细一致、预算可跟踪 |
| v0.3 | 投资自动估值 | **行情 Provider 实现（A股/港股/基金净值/黄金）**、cron 自动更新市值、买卖/分红/利息交易、持仓自动维护 | 持仓市值与券商/基金 App 基本一致 |
| v0.4 | 家庭资产合计 | **资产/净资产总览（本币折算）**、多币种汇率、实物资产（房产/汽车/黄金）估值、Mac 客户端 | 家庭资产负债表可一键查看 |
| v0.5 | 导入与安全 | 微信/支付宝 CSV 批量导入、2FA、OIDC、应用锁、加密 | 历史账单可迁入、安全项通过 |

> 投资与资产的**表结构和接口在 v0.1 就定好**，v0.3/v0.4 只是填实现，不需要改表、不影响已有数据。

> 排期说明（**估算，需按实际投入校准**）：按业余时间每周约 10~15 小时、1 人全栈计，**v0.1 MVP 约 15~19 周**。iOS 客户端与 Android 共用 Flutter 代码，增量主要是 iOS 适配与快捷指令对接。后端任务 1-9、客户端任务 10-15、Web 任务 16、iOS 任务 17 可交叉推进；识别（8/14）和实时同步（7/15）是两个主要风险点，建议尽早做技术验证（spike），别等到最后。

---

## 开工前的两个技术验证（建议第 1 周就做）

1. **识别 spike**：Flutter + ML Kit 跑通"截图→中文文本→规则/LLM 结构化"，确认你收集的样例识别率可接受。
2. **同步 spike**：最小 WebSocket 双通道 + 离线队列幂等，确认实时与离线模型成立。

这两个验证通过，整个方案的技术风险就基本解除，其余都是确定性的业务开发。

