# ezBookkeeping 代码导读

> 仓库：`mayswind/ezbookkeeping` ｜ 协议：MIT ｜ 当前版本 2.2.0
> 定位：开源自托管个人记账 Web 应用，Go 后端 + Vue3 前端，单 Docker 镜像跑在 NAS / 树莓派上。
> 本文目标：帮你快速建立"从入口到表"的全局地图，并标出哪些设计可以直接抄进你自己的记账项目。

---

## 0. 技术栈一眼

| 层 | 选型 | 备注 |
|---|---|---|
| CLI 入口 | `urfave/cli/v3` | `ezbookkeeping server run` / `database` / `userdata` / `cron` 等子命令 |
| Web 框架 | **Gin** (`gin-gonic/gin`) | 不是 net/http 裸写，路由分组清晰 |
| ORM | **xorm** (`xorm.io/xorm`) | 不是 GORM；结构体 tag 用 `xorm:"..."` |
| 数据库 | SQLite / MySQL / PostgreSQL 三选一 | 驱动 `mattn/go-sqlite3`、`go-sql-driver/mysql`、`lib/pq` |
| 鉴权 | `golang-jwt/jwt/v5` | JWT，支持 Header / Query / Cookie 三种来源 |
| 前端 | Vue 3 + **Framework7**（移动端 UI）+ ECharts + Pinia + axios + Vite | 桌面端和移动端是两套入口、两套路由 |
| 对象存储 | 本地文件 / MinIO / S3 / WebDAV 抽象 | `pkg/storage/` |
| LLM | OpenAI / Anthropic / Google AI / Ollama / LM Studio 抽象 | 文本识别、收据图片识别、图表代码生成 |
| 容器 | 多阶段 Docker 构建，最终 alpine 单镜像，非 root 运行 | 暴露 8080 |

---

## 1. 顶层目录导航

```
ezbookkeeping/
├── ezbookkeeping.go        # main 入口，组装 CLI 命令
├── cmd/                    # CLI 子命令实现（webserver / database / userdata / cron / security）
├── conf/ezbookkeeping.ini  # 全部配置项（800+ 行注释，中文友好）
├── Dockerfile              # 多阶段：Go 构建 → Node 构建前端 → alpine 打包
├── docker/                 # entrypoint 脚本
├── pkg/                    # ★ 后端所有业务代码都在这
│   ├── api/                #   Handler 层（HTTP 入参解析 + 调 service）
│   ├── services/           #   业务逻辑层
│   ├── models/             #   数据模型（DB 表结构 + 请求/响应 DTO）
│   ├── datastore/          #   数据访问层（xorm 封装、连接池、事务）
│   ├── middlewares/        #   Gin 中间件（鉴权、日志、限流、request id）
│   ├── auth/oauth2/        #   OIDC 外部登录
│   ├── llm/                #   LLM provider 抽象
│   ├── converters/         #   ★ 各家账单导入转换器（微信/支付宝/京东/飞书/OFX/QIF/…）
│   ├── storage/            #   对象存储抽象（本地/MinIO/S3/WebDAV）
│   ├── exchangerates/     #   汇率源
│   ├── mail/               #   邮件
│   ├── locales/            #   i18n 文案
│   ├── cron/              #   定时任务（定时交易、汇率刷新）
│   ├── mcp/               #   Model Context Protocol server（给 AI 调用）
│   ├── settings/          #   配置加载
│   ├── core/               #   Context 封装、通用类型
│   ├── errs/              #   业务错误码
│   └── utils/validators/ #   工具函数
├── src/                    # ★ 前端 Vue 代码
│   ├── DesktopApp.vue / desktop-main.ts / desktop.html   # 桌面端入口
│   ├── MobileApp.vue  / mobile-main.ts  / mobile.html     # 移动端入口
│   ├── views/desktop/  views/mobile/                     # 两套页面
│   ├── router/desktop.ts router/mobile.ts                  # 两套路由
│   ├── stores/            # Pinia store
│   ├── models/            # 前端 TS 类型（和后端 models 对应）
│   └── locales/           # 前端 i18n
├── templates/              # 邮件模板
├── public/                 # PWA 图标、manifest
└── testdata/               # 导入测试样例
```

---

## 2. 一次请求的完整链路

以"新增一条交易"为例，调用链：

```
浏览器 POST /api/v1/transactions/add.json
  │
  ├─ Gin 路由组  cmd/webserver.go
  │    apiV1Route.Use(RequestId, RequestLog, TokenAuthorizationByHeader, APITokenIpLimit)
  │
  ├─ 中间件
  │    ├─ middlewares/request_id.go        生成请求 ID
  │    ├─ middlewares/request_log.go       记录访问日志
  │    ├─ middlewares/authorization.go     从 Header 解析 JWT → claims.Uid 塞进 context
  │    └─ middlewares/api_token_ip_limit.go 按 IP 限流
  │
  ├─ Handler  pkg/api/transactions.go: TransactionCreateHandler
  │    ├─ c.BindJSON(&req)                  校验入参（binding tag）
  │    ├─ 从 context 拿 claims.Uid
  │    └─ 调 services.Transactions.CreateTransaction(...)
  │
  ├─ Service  pkg/services/transactions.go: CreateTransaction
  │    ├─ 业务校验（账户是否存在、金额、时间）
  │    ├─ 金额转换（前端传 decimal 字符串 → int64 分）
  │    ├─ 开事务：插入 transaction 表 + 维护 tag 关联表 + 更新账户余额
  │    └─ 转账类交易：自动生成对侧 related 交易
  │
  ├─ Datastore  pkg/datastore/datastore.go
  │    └─ UserDataStore.Choose(uid) → xorm.Session → Insert/Update
  │
  └─ 返回 JSON  {"data": {...}} 或 errs 错误码
```

**关键认知**：分层非常规矩——`api` 不写业务、`services` 不碰 HTTP、`datastore` 不写业务规则、`models` 是纯结构体。你做自己项目时照这个分层抄就行。

---

## 3. 后端各层看哪些文件

### 3.1 入口与启动

- **`ezbookkeeping.go`**：`main()` 用 `urfave/cli` 注册 6 个子命令：
  - `server run` — 启动 Web 服务
  - `database` — 建表/迁移/备份（`cmd/database.go`）
  - `userdata` — 用户数据导入导出 CLI（`cmd/user_data.go`，26KB，功能很全）
  - `cron_jobs` — 手动触发定时任务
  - `security_utils` — 密码哈希、token 工具
  - `utilities` — 杂项
- **`cmd/webserver.go`**（35KB）：整个 HTTP 路由表都在这一个文件里。启动流程：
  1. `initializeSystem(c)` 加载配置
  2. `updateAllDatabaseTablesStructure(c)` 自动建表/迁移（xorm Sync2）
  3. 初始化 requestid、MCP、cron
  4. 注册静态文件、API 路由、中间件
  5. `r.Run(addr)`

### 3.2 路由表（`cmd/webserver.go`）

路由分组非常规整，按业务域切：

| 路由前缀 | 鉴权 | 对应 api 文件 | 业务 |
|---|---|---|---|
| `/api/authorize.json` `/api/register.json` | 无 | `authorizations.go` / `users.go` | 登录、注册、忘记密码 |
| `/api/v1/tokens/*` | JWT | `tokens.go` | token 刷新、吊销、生成 API token |
| `/api/v1/users/*` | JWT | `users.go` | 个人资料、头像、外部登录绑定 |
| `/api/v1/transactions/*` | JWT | `transactions.go` | ★ 交易 CRUD、统计、导入、对账 |
| `/api/v1/transaction/categories/*` | JWT | `transaction_categories.go` | 分类两级管理 |
| `/api/v1/transaction/tags/*` `/tags/groups/*` | JWT | `transaction_tags.go` / `transaction_tag_groups.go` | 标签 + 标签组 |
| `/api/v1/transaction/templates/*` | JWT | `transaction_templates.go` | 记账模板 |
| `/api/v1/accounts/*` | JWT | `accounts.go` | ★ 账户两级管理 |
| `/api/v1/insights/explorers/*` | JWT | `explorers.go` | 自定义图表探索器 |
| `/api/v1/llm/*` | JWT | `large_language_models.go` | ★ 文本识别、收据图片识别、图表代码生成 |
| `/api/v1/exchange_rates/*` | JWT | `exchange_rates.go` | 汇率 |
| `/mcp` | MCP token | `model_context_protocols.go` | JSON-RPC，给 AI agent 调用 |
| `/oauth2/login` `/oauth2/callback` | 无 | `oauth2_authentications.go` | OIDC 登录 |
| `/pictures/:fileName` `/avatar/:fileName` `/icons/:fileName` | Query token | `transaction_pictures.go` 等 | 图片附件直出（带鉴权） |

**注意**：所有业务 API 都在 `/api/v1/` 下，后缀统一 `.json`（历史风格，不是 RESTful 资源路径）。

### 3.3 中间件（`pkg/middlewares/`）

- `authorization.go`：三种 token 来源（Header / Query / Cookie），分别对应 API、图片附件、网页登录场景。
- `recovery.go`：recover panic，统一返回 500。
- `request_id.go`：每请求一个 UUID，串日志。
- `request_log.go`：访问日志。
- `api_token_ip_limit.go` / `mcp_server_ip_limit.go`：IP 限流。
- `amap_api_proxy_auth_cookie.go`：高德地图代理接口的 cookie 鉴权。

### 3.4 数据模型（`pkg/models/`）

这是你做自己项目最该抄的部分。

#### User（用户表）— `user.go:87`

```go
type User struct {
    Uid              int64  `xorm:"PK"`
    Username         string `xorm:"VARCHAR(32) UNIQUE"`
    Email            string `xorm:"VARCHAR(100) UNIQUE"`
    Password         string `xorm:"VARCHAR(64)"`   // 加盐哈希后存
    Salt             string `xorm:"VARCHAR(10)"`
    DefaultAccountId int64
    Language         string
    DefaultCurrency  string `xorm:"VARCHAR(3)"`
    // … 一堆用户偏好（日期格式、货币格式、周起始日等）
    Disabled         bool
    Deleted          bool `xorm:"NOT NULL"`      // 软删除
    EmailVerified    bool
    CreatedUnixTime  int64
    UpdatedUnixTime  int64
    DeletedUnixTime  int64
    LastLoginUnixTime int64
}
```

**注意**：ezBookkeeping 是"单实例多用户"，但**没有家庭/账本共享模型**——每个用户独立，不支持多人共一个账本。这是你和 BeeCount 最大的差异点，也是你要自己补的。

#### Account（账户表）— `account.go:76`

```go
type Account struct {
    AccountId       int64  `xorm:"PK"`
    Uid             int64  `xorm:"INDEX(...)"`
    Deleted         bool
    Category        AccountCategory  // 现金/储蓄/信用/资产/负债…
    Type            AccountType      // 系统内置 or 自定义
    ParentAccountId int64            // 支持两级账户（父账户下挂子账户）
    Name            string
    DisplayOrder    int32
    Icon            int64
    Color           string
    Currency        string `xorm:"VARCHAR(3)"`
    Balance         int64            // ★ 余额，单位：分（int64）
    Extend          *AccountExtend   `xorm:"BLOB"` // 信用卡还款日、额度等扩展字段
    Hidden          bool
    // …时间戳
}
```

#### Transaction（交易记录表）— `transaction.go:127`

```go
type Transaction struct {
    TransactionId        int64  `xorm:"PK"`
    Uid                  int64  `xorm:"UNIQUE(UQE_transaction_uid_time) INDEX(...)"`
    Deleted              bool
    Type                 TransactionDbType  // 收入/支出/转账
    CategoryId           int64
    AccountId            int64
    TransactionTime      int64             // unix 秒
    TimezoneUtcOffset    int16             // 交易发生时的时区偏移
    Amount               int64             // ★ 金额，单位：分，不用 float
    RelatedId            int64             // 转账时对侧交易 ID
    RelatedAccountId      int64
    RelatedAccountAmount int64             // 转账时对侧账户金额（汇率换算后）
    HideAmount           bool
    Comment              string
    GeoLongitude         float64
    GeoLatitude          float64
    CreatedIp            string
    ScheduledCreated     bool              // 是否由定时交易自动生成
    // …时间戳
}
```

**几个设计亮点（直接抄）**：

1. **金额一律 `int64` 存"分"**，绝不用 float。前端展示再除以 100。这是记账软件的底线。
2. **软删除**：所有表都有 `Deleted bool` + `DeletedUnixTime`，查询时自动 `WHERE deleted=0`。
3. **时间一律存 unix 时间戳**，时区偏移单独存 `TimezoneUtcOffset`，前端按本地时区渲染。
4. **转账是两条交易**：支出一条 + 对侧收入一条，用 `RelatedId` 互相关联；删一条会级联删对侧。
5. **索引按查询模式设计**：`(uid, deleted, time)`、`(uid, deleted, type, time)`、`(uid, deleted, category_id, time)`、`(uid, deleted, account_id, time)` 全都建了——对应列表页的各种过滤。
6. **多对多用关联表**：`transaction_tag_index`（交易↔标签）。

#### 其他表

- `transaction_category.go`：分类（两级，收入/支出各一套）
- `transaction_tag.go` + `transaction_tag_group.go`：标签 + 标签分组
- `transaction_template.go`：记账模板（固定金额/分类的快捷入口）
- `transaction_picture_info.go`：交易图片附件元数据（图片本体存 storage，DB 只记路径）
- `token_record.go`：登录 token 记录（支持多端登录、吊销）
- `exchange_rate.go`：汇率
- `explorer.go`：自定义图表配置

### 3.5 数据访问层（`pkg/datastore/`）

- `database.go`：封装 xorm EngineGroup，提供 `NewSession(ctx)` 和 `DoTransaction(ctx, fn)`。
- `datastore_container.go`：**三个 DataStore 单例**：
  - `UserStore` — 用户表（全局共享，不分库）
  - `TokenStore` — 登录 token（按 uid 哈希分库）
  - `UserDataStore` — 业务数据（交易、账户、分类…按 uid 哈希分库）
- `services/base.go`：每个 service 嵌入 `ServiceUsingDB`，通过 `UserDataDB(uid)` 拿到当前用户的库连接。

**为什么按 uid 分库？** 这是为将来用户量大了能水平拆分预留的（虽然家庭自托管根本到不了这个量）。你自己做可以简化成单库，但这个"按 uid 路由"的写法值得看。

### 3.6 Service 层（`pkg/services/`）

每个业务域一个文件：
- `transactions.go`（2400+ 行）：最核心，增删改查、统计、余额计算、定时交易
- `accounts.go`：账户 CRUD、余额变动
- `transaction_categories.go` / `transaction_tags.go` / `transaction_templates.go`
- `users.go`：注册、密码哈希（salt+hash）、profile
- `tokens.go`：JWT 签发、刷新、吊销
- `twofactor_authorizations.go`：TOTP 两步验证
- `explorer.go`：自定义图表
- `user_external_auths.go`：OIDC 绑定
- `user_custom_exchange_rates.go`：自定义汇率

**事务粒度**：`CreateTransaction` 内部用 `DoTransaction` 包了"插交易 + 插 tag 关联 + 更新账户余额 + 生成对侧转账"四步，任一失败回滚。

### 3.7 账单导入转换器（`pkg/converters/`）— 宝藏目录

你要做 OCR 截图记账，这个目录是金矿。每家银行/支付平台的账单格式都写好了转换器：

```
converters/
├── alipay/       # 支付宝账单 CSV
├── wechat/       # 微信账单 CSV
├── jdcom/        # 京东账单
├── feidee/       # 飞书/随手记格式
├── beancount/    # Beancount 格式
├── fireflyIII/   # Firefly III 导入
├── gnucash/      # GnuCash
├── ofx/ qif/ iif/ # 国外财务软件格式
├── camt/ mt/     # 欧洲银行 MT940 / Camt.052/053
├── excel/ csv/   # 通用 Excel/CSV + 自定义列映射
├── ai/           # ★ LLM 识别结果转交易（你要的 OCR 出口在这）
└── custom/       # 自定义脚本规则
```

**抄这个思路**：你做"截图 OCR → 自动记账"时，不要让 LLM 直接输出 JSON 入库，而是让 LLM 输出一个中间格式（金额、商户、时间、备注），再走和 CSV 导入一样的 converter 管道入库。这样校验、去重、分类推荐全复用。

### 3.8 LLM 抽象（`pkg/llm/`）

- `provider/`：OpenAI / Anthropic / Google AI / Ollama / LM Studio 五个实现，统一接口。
- 三个用途：
  1. `llm_text_recognition` — 文本识别（用户粘贴一段聊天/短信，提取交易）
  2. `llm_image_recognition` — 收据图片识别（上传小票照片，提取金额商户）
  3. `llm_coding_assistant` — 给自定义图表生成 ECharts 配置代码
- 配置全在 `conf/ezbookkeeping.ini` 的 `[ai]` / `[llm_text_recognition]` / `[llm_image_recognition]` 段，支持配不同 provider 给不同用途。

**对你的参考**：你不需要自己写 OCR，直接接 LLM vision（GPT-4o / Claude / 通义千问 VL）做收据识别就行，prompt 模板可以直接参考这个仓库的 `pkg/llm/` 和 `pkg/converters/ai/`。

### 3.9 对象存储（`pkg/storage/`）

抽象了四种后端：本地文件系统、MinIO、S3、WebDAV。交易图片、用户头像、自定义图标都走这层。配置在 `[storage]` 段。你做 NAS 部署默认本地文件系统就够，将来想换 MinIO 不改业务代码。

---

## 4. 前端结构（`src/`）

### 4.1 双端架构

- 桌面端：`DesktopApp.vue` + `desktop-main.ts` + `router/desktop.ts` + `views/desktop/`
- 移动端：`MobileApp.vue` + `mobile-main.ts` + `router/mobile.ts` + `views/mobile/`
- 共用：`components/`、`stores/`、`models/`（TS 类型）、`locales/`、`styles/`

**为什么双端不一套代码？** 桌面端用侧边栏 + 表格，移动端用 Framework7 的卡片流 + 底部 tab，交互差异大，硬塞一套组件反而复杂。你自己做 Flutter 跨端时不用纠结这个——Flutter 本身就是一套代码。

### 4.2 关键页面（`views/desktop/`）

- `HomePage.vue`：首页概览
- `transactions/`：交易列表 + 新增/编辑弹窗
- `accounts/`：账户管理
- `categories/` `tags/` `templates/`：分类、标签、模板
- `statistics/`：统计图表（ECharts）
- `overview/`：自定义概览看板
- `insights/`：自定义图表探索器
- `settings/`：设置
- `LoginPage.vue` `SignupPage.vue` `ForgetPasswordPage.vue` `UnlockPage.vue`：登录/注册/PIN 锁

### 4.3 状态管理（`src/stores/`）

Pinia store 按业务域拆分，和后端 service 一一对应：`user.ts` `transaction.ts` `account.ts` `transactionCategory.ts` `transactionTag.ts` `exchangeRates.ts` `statistics.ts` `overview.ts` `token.ts` …

### 4.4 前端值得看的点

- `sw.ts`：PWA Service Worker，离线缓存
- `models/`：TS 类型和后端 Go struct 对应，改后端要同步改这里
- `vite.config.ts`：构建配置，把桌面端和移动端打成两个产物

---

## 5. Docker 部署

### 5.1 Dockerfile 三段式

1. **be-builder**：`golang:1.27-alpine`，`./build.sh backend` 编译出单个 Go 二进制
2. **fe-builder**：`node:26-alpine`，`./build.sh frontend` 用 Vite 打出静态文件
3. **最终镜像**：`alpine:3.24`，拷入二进制 + `dist/` 静态文件 + `conf/` + `templates/`，用 uid 1000 非 root 跑

### 5.2 运行

```bash
docker run -d \
  -p 8080:8080 \
  -v /your/nas/path:/ezbookkeeping/data \
  -v /your/nas/path/storage:/ezbookkeeping/storage \
  mayswind/ezbookkeeping
```

持久化目录：
- `/ezbookkeeping/data/` — SQLite 数据库文件（如果用 MySQL/PG 就不用挂这个）
- `/ezbookkeeping/storage/` — 交易图片、头像等附件
- `/ezbookkeeping/log/` — 日志

### 5.3 配置

两种方式：
1. 容器内 `conf/ezbookkeeping.ini`（800 行，全注释）
2. 环境变量 `EBK_CONF_PATH` 指定外挂配置文件路径

关键配置段：`[server]`（端口、TLS）、`[database]`（类型、连接串）、`[storage]`（存储后端）、`[ai]`/`[llm_*]`（LLM provider）、`[security]`（密码策略、限流）、`[auth]`（JWT secret、注册开关）。

---

## 6. 对你自己做项目的参考清单

结合你之前那份线程里的路线（BeeCount Flutter 客户端 + ezBookkeeping 后端参考），这份源码你重点看这些：

### 直接抄的设计

| 设计点 | 看哪里 | 为什么抄 |
|---|---|---|
| 金额 int64 存分 | `models/transaction.go:136` | 记账底线，避免浮点误差 |
| 时间戳 + 时区偏移 | `models/transaction.go:134-135` | 跨时区旅行记账不会错 |
| 软删除 + 时间戳 | 所有 model | 误删可恢复，审计友好 |
| 分层（api/service/datastore） | 整个 `pkg/` | 你 Java 转 Go 也能立刻上手 |
| xorm Sync2 自动建表 | `cmd/webserver.go:updateAllDatabaseTablesStructure` | 不用手写 migration |
| JWT 多来源鉴权 | `middlewares/authorization.go` | API 走 Header，图片走 Query，网页走 Cookie |
| 事务包多表写入 | `services/transactions.go:CreateTransaction` | 交易+余额+对侧转账原子性 |
| 存储抽象层 | `pkg/storage/` | 本地/MinIO/S3 切换不改业务 |
| LLM provider 抽象 | `pkg/llm/provider/` | 换 LLM 供应商不改业务 |
| 账单导入 converter 管道 | `pkg/converters/` | 截图 OCR 结果走同一管道入库 |

### 不要照抄的地方

1. **没有家庭共享账本模型**——ezBookkeeping 是单用户的，多人共用账本你要自己加 `Book`（账本）+ `BookMember`（成员角色）表，这是 BeeCount 已经做了的。
2. **没有实时同步**——它是传统 REST CRUD，没有 WebSocket。BeeCount 的 WebSocket 实时同步你直接参考 BeeCount。
3. **没有离线队列**——它是 Web PWA，离线能力靠 Service Worker 缓存页面，没有"本地先记、联网同步"的本地优先架构。BeeCount 的本地优先才是你要的。
4. **API 风格老派**——`.json` 后缀 + POST 做所有写操作，不是 RESTful。你新写可以更干净，但学它的分层和错误码即可。
5. **按 uid 分库**——家庭场景没必要，单库就行。

### 建议的阅读顺序

1. 先跑起来：`docker run -p 8080:8080 mayswind/ezbookkeeping`，注册账号，点一遍功能
2. 看 `cmd/webserver.go` 路由表，对照前端操作
3. 看 `pkg/models/` 五个核心表（User/Account/Transaction/Category/Tag）
4. 看 `pkg/services/transactions.go` 的 `CreateTransaction` 一个函数，把增删改全链路看懂
5. 看 `pkg/converters/wechat/` 和 `pkg/converters/alipay/`，理解账单导入怎么写
6. 看 `pkg/llm/provider/openai/`，理解 LLM 接口抽象
7. 最后看 `src/stores/transaction.ts` 和 `src/views/desktop/transactions/`，对照前端怎么调后端
