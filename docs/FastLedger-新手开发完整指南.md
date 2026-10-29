# FastLedger 家庭智能记账系统 · 新手开发完整指南

> 这份指南假设你**会一点编程（比如刚学完 Java/Python 基础），但没做过完整的全栈项目**。
> 我们会从零开始，一步步做出一个真正能用的家庭记账软件。每一步都告诉你：**做什么、为什么、代码长什么样、怎么验证做对了**。
> 照着做，你能独立完成一个"截图识别 → 自动记账 → 全家共享"的系统。

---

## 目录

**第一部分：开始之前**
- 1. 我们要做一个什么东西
- 2. 名词解释（看不懂的词先查这里）
- 3. 技术选型（用什么做，为什么）
- 4. 搭建开发环境（手把手）

**第二部分：整体设计**
- 5. 系统整体架构
- 6. 数据库设计（完整表结构）
- 7. API 接口设计（前后端怎么对话）

**第三部分：后端开发（Go）**
- 8. 项目初始化与目录结构
- 9. 配置与数据库连接
- 10. 用户注册登录（JWT）
- 11. 账本与成员
- 12. 账户与分类
- 13. 交易（含转账、余额）
- 14. WebSocket 实时推送
- 15. 识别接口（OCR 文本兜底）
- 16. 投资模块（标的/持仓/资产合计）
- 17. 账单导出
- 18. 预算

**第四部分：客户端开发（Flutter）**
- 19. Flutter 工程与网络层
- 20. 登录与各页面
- 21. 截图 OCR（Android / Windows / iOS）
- 22. 实时同步

**第五部分：Web 端**
- 23. Web 端（Go 托管）

**第六部分：部署与上线**
- 24. Docker 部署
- 25. Caddy 反代与外网访问
- 26. 备份与运维

**第七部分：开发计划**
- 27. 分阶段任务清单（照着排期做）
- 28. 验收标准

---
---

# 第一部分：开始之前

## 1. 我们要做一个什么东西

**FastLedger** 是一个跑在你自己家 NAS（一台常开的小服务器）上的家庭记账软件。

核心使用场景：
1. 你在微信/支付宝付完款，**截图**；
2. 按一个快捷键（电脑）或点悬浮球（手机），软件**自动识别**截图里的金额、商户、时间；
3. 弹出确认框，你看一眼点"确定"（**3 秒完成一笔**）；
4. 这笔账**实时同步**给家人，全家都能看到。

除了日常记账，还要能：
- 管理多个账户（储蓄卡、信用卡、微信零钱、支付宝余额）；
- 记录投资（A 股、港股、基金、黄金）和大件资产（房子、车子），**合计家庭总资产**；
- 设预算、看报表、导出账单。

**关键原则：数据完全在自己手里，不依赖任何第三方云服务。**

### 最小可用版本（MVP）先做什么

只做核心闭环：**截图 → 识别 → 确认 → 入账 → 家人共享**。
但数据库和代码结构要**从第一天就预留好**投资、预算、导出等功能的位置，以后加功能不用推倒重来。

---

## 2. 名词解释（看不懂随时回来查）

| 名词 | 通俗解释 |
|---|---|
| **后端（Backend）** | 跑在服务器上、看不见界面的程序，负责存数据、算逻辑、给前端提供数据。 |
| **前端 / 客户端（Frontend/Client）** | 用户看得见、点得到的界面（手机 App、网页、电脑程序）。 |
| **API** | 后端给前端用的"接口"，约定好"要什么数据、传什么参数、返回什么"。可以理解成餐厅的菜单和取餐窗口。 |
| **HTTP / REST** | 前端和后端通信最常用的一套规则。`GET` 是取数据，`POST` 是提交数据，`PUT` 是改，`DELETE` 是删。 |
| **JSON** | 一种数据格式，长得像 `{ "name": "张三", "age": 18 }`，前后端通信用它。 |
| **数据库（DB）** | 专门存数据的软件。我们用 **SQLite**（一个文件就是一个数据库，最简单），后期可换 MySQL/PostgreSQL。 |
| **SQL** | 操作数据库的语言，比如 `SELECT * FROM users`（查用户）、`INSERT INTO ...`（插入）。 |
| **ORM** | 帮你把数据库表和代码里的对象对应起来，少写 SQL。Go 里我们用 **xorm**。 |
| **JWT（Token）** | 登录成功后发给你的一张"电子通行证"，之后每次请求带上它，后端就知道你是谁，不用每次输密码。 |
| **WebSocket（WS）** | 一条前后端之间的"常驻电话线"，服务器有新消息能**主动推**给你（家人记账你立刻看到），不用反复刷新。 |
| **OCR** | 光学字符识别，把图片里的文字"读"出来。 |
| **Docker** | 把软件和它依赖的环境打包成一个"集装箱"（镜像），在任何机器上一条命令就能跑起来，不用手动配环境。 |
| **镜像 / 容器** | 镜像是打包好的模板，容器是镜像跑起来的实例。 |
| **反向代理（Caddy）** | 站在你的服务前面，负责把外网的 HTTPS 请求转发给后端，还能自动配置证书。 |
| **HTTPS / 证书** | 加密的网页传输，防止数据在网络上被偷看，浏览器地址栏有小锁。 |
| **端侧 / 本地** | 在你的手机/电脑上完成，数据不经过服务器。 |
| **LLM（大模型）** | 像 ChatGPT 这样的 AI，用来兜底识别复杂账单。 |
| **NAS** | 网络附加存储，家里一台常开、能存文件、能跑 Docker 的小服务器。 |

---

## 技术选型（用什么做，为什么）

| 部分 | 技术 | 为什么选它 |
|---|---|---|
| 后端语言 | **Go（Golang）** | 编译后是单个二进制文件、内存占用小、天生适合写高并发服务，非常适合跑在 NAS 上。 |
| Web 框架 | **Gin** | Go 生态最流行、最简单的 Web 框架，资料多。 |
| ORM | **xorm** | 简单、支持 SQLite，参考项目 ezBookkeeping 也用它。 |
| 数据库 | **SQLite 起步** | 零配置、一个文件、易备份；代码结构预留切换 MySQL/PostgreSQL 的能力。 |
| 客户端 | **Flutter** | 一套代码同时出 Windows、Android、iOS，界面统一、开发快。 |
| Web 端 | 由 Go 后端直接托管 | 不增加额外部署单元，浏览器打开就能浏览编辑。 |
| 部署 | **Docker + Caddy** | 一键部署、自动 HTTPS。 |
| 识别 | 端侧 OCR + 端侧规则 + 云端 LLM 文本兜底 | 图片不出设备，保护隐私；复杂账单才把**文字**（不是图）发给 AI。 |

> 为什么不用 Java 写后端？你可能更熟悉 Java，但 Java 程序内存占用大、部署要带 JVM，在小内存 NAS 上不如 Go 轻便。Go 语法很简单，有编程基础的人一两天就能上手写接口。

---

## 4. 搭建开发环境（手把手）

### 4.1 安装 Go（后端）

1. 打开官网 https://go.dev/dl/ ，下载对应系统的安装包（Windows 是 `.msi`，一路下一步）。
2. 验证：打开终端（Windows 用 PowerShell 或 CMD），输入：
   ```bash
   go version
   ```
   能看到类似 `go version go1.22 ...` 就成功了。

> 国内网络配置代理（下载依赖更快），终端执行：
> ```bash
> go env -w GOPROXY=https://goproxy.cn,direct
> ```

### 4.2 安装 Flutter（客户端）

1. 打开官网 https://docs.flutter.dev/get-started/install ，按系统下载安装。
2. 安装后运行检查命令，它会告诉你还缺什么：
   ```bash
   flutter doctor
   ```
   按提示把打 ✗ 的项处理好（Android 开发需要 Android Studio；Windows 桌面需要 Visual Studio 含 C++ 桌面组件）。

### 4.3 安装 Docker（部署用）

- 开发电脑可装 Docker Desktop（https://www.docker.com/ ）。
- 最终部署在 NAS 上，群晖/绿联等系统的应用商店里能直接装 Docker（或 Container Manager）。

### 4.4 安装编辑器

- **VS Code**（免费，装 Go 和 Flutter 插件）：https://code.visualstudio.com/
- 或 GoLand / IntelliJ IDEA（学生可免费申请）。

### 4.5 准备一个 Git 仓库（管理代码）

```bash
# 在你放代码的目录执行
mkdir fastledger && cd fastledger
git init
```
建议在 GitHub/Gitee 建一个**私有仓库**，代码随时提交，不怕丢。

---
---

# 第二部分：整体设计

## 5. 系统整体架构

### 5.1 一张图看懂

```
┌────────────────────────────────────────────────────────────┐
│                        用户的设备                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ Windows 客户端 │  │ Android 客户端 │  │ iOS 客户端        │  │
│  │ (Flutter)    │  │ (Flutter)    │  │ (Flutter)        │  │
│  │ 快捷键截图OCR │  │ 悬浮球截图OCR │  │ 快捷指令截图OCR   │  │
│  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘  │
│         └─────────────────┼───────────────────┘             │
│                           │                                 │
│              ┌────────────┴────────────┐                    │
│              │  Web 端（浏览器，无OCR） │                    │
│              └────────────┬────────────┘                    │
└───────────────────────────┼─────────────────────────────────┘
                            │ HTTPS / WebSocket（JSON）
                            ▼
┌────────────────────────────────────────────────────────────┐
│                   家里的 NAS（Docker）                       │
│            ┌──────────────────────────────┐                 │
│  Caddy ───▶│  Go 后端（Gin，单个二进制）   │                 │
│ 自动HTTPS  │  ┌────────────────────────┐ │                 │
│  反向代理   │  │ API 接口 / WebSocket   │ │                 │
│            │  │ 业务逻辑（Service）     │ │                 │
│            │  │ 数据访问（xorm）        │ │                 │
│            │  └───────────┬────────────┘ │                 │
│            └──────────────┼──────────────┘                 │
│                           ▼                                │
│                    SQLite 数据库文件                         │
│                  （+ 本地图片/附件目录）                      │
└────────────────────────────────────────────────────────────�
```

### 5.2 后端分层（很重要，写代码都按这个来）

后端代码分四层，**每层只和相邻层打交道**：

```
路由层（Router/Handler）  接收 HTTP 请求、解析参数、返回响应
        ↓
逻辑层（Service）        处理业务规则（算余额、转账、识别、统计）
        ↓
数据访问层（Repository/Model）  用 xorm 读写数据库
        ↓
数据库（SQLite）
```

> 为什么要分层？把"界面逻辑、业务规则、数据存取"分开，以后改一处不影响其他地方，新手也不容易写乱。

### 5.3 识别流程（三级漏斗）

```
截图
 │
 ├─ ① 端侧 OCR：图片在手机/电脑上转成文字（图片不出设备）
 │
 ├─ ② 端侧规则：用模板匹配微信/支付宝账单（离线、最快、最省）
 │
 └─ ③ 规则没命中 → 把 OCR 文字（不是图片）发给后端 → LLM 兜底
        │
        ▼
   弹出确认框（预填金额/商户/时间/账户）→ 用户确认 → 入库
```

> 所有识别结果**必须人工确认**才入账，避免识别错了污染账本。

---

## 6. 数据库设计（完整表结构）

### 6.1 三条贯穿全局的设计铁律

1. **金额一律用整数 `BIGINT` 存"分"**，不用浮点数（浮点数会有 0.1+0.2≠0.3 的精度问题）。
   - 例：36.50 元 → 存 `3650`。显示时再除以 100。
2. **所有业务表都带 `book_id`**（账本是数据隔离的边界），并带软删除 `deleted` + 时间戳。
3. **交易记录带 `creator_uid`**（谁记的），配合"是否可见他人记录"开关做过滤。

### 6.2 账户分类（15 类，骨架一次定好）

```
资产类（category 1~9）：
  1 现金（含外币现金）
  2 活期储蓄
  3 定期存款（预留）
  4 电子钱包（微信零钱/支付宝余额）
  5 投资账户（券商/基金/债券/贵金属）
  6 实物资产（黄金/房产/汽车/贵重物品）
  7 保险/年金（预留）
  8 其他资产
  9 应收款（预留）
负债类（category 10~15）：
  10 信用卡
  11 信用消费贷（花呗/白条）
  12 房贷
  13 车贷
  14 其他借款
  15 应付款（预留）
```

### 6.3 资金账户 vs 支付渠道（避免重复记账）

这是最容易糊涂的点，务必理解：

- **资金账户（Account）**：真正持有/欠的钱。微信**零钱/余额**（真充值了）是账户；快捷支付时的"微信"不是账户。
- **支付渠道（Payment Channel）**：付款用的工具，只是一个标签（微信/支付宝/云闪付/POS）。

举例：微信绑定信用卡付 100 元餐饮：
- 资金账户 = 信用卡（钱实际从这出，和银行对账用）；
- 支付渠道 = 微信（用什么工具付的）；
- **一笔支出只记一个资金账户，绝不重复**。

信用卡还款是**转账**（储蓄卡→信用卡），不是支出，不能再记一次。

### 6.4 完整建表 SQL（SQLite）

> 新手可以先把下面 SQL 存成 `schema.sql`，用数据库工具（如 DB Browser for SQLite）执行，也可以让 xorm 自动建表。

```sql
-- 用户
CREATE TABLE users (
  id          INTEGER PRIMARY KEY,
  username    VARCHAR(64) NOT NULL UNIQUE,
  password_hash VARCHAR(128) NOT NULL,      -- 存 bcrypt 哈希（已内含盐，无需 salt 列），不存明文
  nickname    VARCHAR(64) DEFAULT '',
  email       VARCHAR(128) DEFAULT NULL,   -- 可选：自托管常无 SMTP，无邮箱用 NULL
  deleted     BOOLEAN DEFAULT 0,
  created_at  INTEGER,
  updated_at  INTEGER,
  deleted_at  INTEGER
);
CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL;

-- 账本
CREATE TABLE books (
  id              INTEGER PRIMARY KEY,
  name            VARCHAR(64) NOT NULL,
  owner_uid       INTEGER NOT NULL,
  default_currency VARCHAR(3) DEFAULT 'CNY',
  members_can_view_others BOOLEAN DEFAULT 1, -- 成员是否默认互看（账本级开关）
  deleted         BOOLEAN DEFAULT 0,
  created_at      INTEGER, updated_at INTEGER, deleted_at INTEGER
);

-- 账本级全局版本计数器（增量同步核心：每个账本一行，写事务内原子 +1）
CREATE TABLE book_versions (
  book_id     INTEGER PRIMARY KEY,
  version     INTEGER NOT NULL DEFAULT 0,
  updated_at  INTEGER
);

-- 幂等操作回执：由唯一 Create 与业务变更在同一事务写入
CREATE TABLE idempotency_records (
  id             INTEGER PRIMARY KEY,
  book_id        INTEGER NOT NULL,
  uid            INTEGER NOT NULL,
  client_uuid    VARCHAR(64) NOT NULL CHECK(length(trim(client_uuid)) BETWEEN 1 AND 64),
  request_hash   VARCHAR(64) NOT NULL,       -- SHA-256：域标签 + 规范化完整业务命令
  result_type    TINYINT NOT NULL CHECK(result_type IN (1,2)), -- 枚举字典：1=单笔 2=转账
  result_ids     VARCHAR(128) NOT NULL DEFAULT '', -- 单笔一个；转账按 outID,inID 排列
  result_version INTEGER NOT NULL DEFAULT 0,      -- 完成后必须为创建时的正版本号
  created_at     INTEGER,
  UNIQUE(book_id, uid, client_uuid)
);
-- 空 result_ids/0 result_version 仅允许作为当前事务内的占位；不允许提交不完整回执。

-- 账本成员（多对多：一个用户可加入多个账本，一个账本有多个成员）
CREATE TABLE book_members (
  id          INTEGER PRIMARY KEY,
  book_id     INTEGER NOT NULL,
  uid         INTEGER NOT NULL,
  role        TINYINT NOT NULL DEFAULT 3,   -- 1=owner 2=admin 3=member
  can_view_others BOOLEAN DEFAULT 1,
  joined_at   INTEGER,
  UNIQUE(book_id, uid)
);

-- 账户
CREATE TABLE accounts (
  id          INTEGER PRIMARY KEY,
  book_id     INTEGER NOT NULL,
  parent_id   INTEGER DEFAULT 0,            -- 仅用于 UI 分组（0=顶级），不参与余额计算，MVP 可先不启用
  category    TINYINT NOT NULL,             -- 见 6.2 账户分类（1~15）
  name        VARCHAR(64) NOT NULL,
  currency    VARCHAR(3) DEFAULT 'CNY',
  balance     BIGINT DEFAULT 0,             -- 余额（分）；投资账户=现金部分，实物资产=估值
  icon        VARCHAR(64) DEFAULT '',
  sort_order  INTEGER DEFAULT 0,
  hidden      BOOLEAN DEFAULT 0,
  sync_version INTEGER DEFAULT 0,          -- 账本级全局版本号（见 book_versions）
  deleted     BOOLEAN DEFAULT 0,
  created_at  INTEGER, updated_at INTEGER, deleted_at INTEGER
);

-- 分类（两级，parent_id=0 是大类）
CREATE TABLE categories (
  id          INTEGER PRIMARY KEY,
  book_id     INTEGER NOT NULL,
  parent_id   INTEGER DEFAULT 0,
  cat_type    TINYINT NOT NULL,             -- 1=收入 2=支出
  name        VARCHAR(64) NOT NULL,
  icon        VARCHAR(64) DEFAULT '',
  color       VARCHAR(8) DEFAULT '',
  sort_order  INTEGER DEFAULT 0,
  hidden      BOOLEAN DEFAULT 0,
  sync_version INTEGER DEFAULT 0,           -- 账本级全局版本号
  deleted     BOOLEAN DEFAULT 0,
  created_at  INTEGER, updated_at INTEGER, deleted_at INTEGER
);

-- 交易
CREATE TABLE transactions (
  id                INTEGER PRIMARY KEY,
  book_id           INTEGER NOT NULL,
  creator_uid       INTEGER NOT NULL,
  tx_type           TINYINT NOT NULL,        -- 1=收入 2=支出 3=转账
  category_id       INTEGER DEFAULT 0,
  account_id        INTEGER NOT NULL,        -- 资金账户（实际扣款）
  payment_channel   TINYINT DEFAULT 0,       -- 0=直接 1=微信 2=支付宝 3=云闪付 4=POS
  amount            BIGINT NOT NULL,         -- 金额（分）
  tx_time           INTEGER NOT NULL,        -- 发生时间（unix 秒）
  timezone_offset   INTEGER DEFAULT 0,
  related_id        INTEGER DEFAULT 0,       -- 转账对侧交易 id
  related_account_id INTEGER DEFAULT 0,
  related_amount    BIGINT DEFAULT 0,
  comment           VARCHAR(255) DEFAULT '',
  client_uuid       VARCHAR(64) DEFAULT '',  -- 端侧幂等去重
  sync_version      INTEGER DEFAULT 0,       -- 该行最后变更时的账本级全局版本号（见 book_versions）
  source            TINYINT DEFAULT 0,       -- 0=手动 1=识别 2=导入
  deleted           BOOLEAN DEFAULT 0,
  created_at        INTEGER, updated_at INTEGER, deleted_at INTEGER
);

-- 标签
CREATE TABLE tags (
  id INTEGER PRIMARY KEY, book_id INTEGER NOT NULL,
  name VARCHAR(64) NOT NULL,
  deleted BOOLEAN DEFAULT 0, created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE TABLE transaction_tag_index (
  tx_id INTEGER NOT NULL, tag_id INTEGER NOT NULL, UNIQUE(tx_id, tag_id)
);

-- 认证 token
CREATE TABLE auth_tokens (
  id INTEGER PRIMARY KEY, uid INTEGER NOT NULL,
  token_type TINYINT NOT NULL,               -- 1=登录 2=长期API 3=WS
  token_hash VARCHAR(128) NOT NULL,
  expires_at INTEGER, revoked BOOLEAN DEFAULT 0, created_at INTEGER
);

-- 投资标的
CREATE TABLE instruments (
  id INTEGER PRIMARY KEY, book_id INTEGER NOT NULL,
  instrument_type TINYINT NOT NULL,          -- 1=股票 2=基金 3=债券 4=贵金属 5=外汇 6=其他
  market TINYINT DEFAULT 0,                  -- 1=A股 2=港股 3=美股 4=场外
  symbol VARCHAR(32) NOT NULL,               -- 代码，如 600519
  name VARCHAR(128) NOT NULL,
  currency VARCHAR(3) DEFAULT 'CNY',
  quote_symbol VARCHAR(48) DEFAULT '',       -- 行情代码（v0.3 自动行情用）
  deleted BOOLEAN DEFAULT 0, created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);

-- 持仓
CREATE TABLE holdings (
  id INTEGER PRIMARY KEY, book_id INTEGER NOT NULL,
  account_id INTEGER NOT NULL,               -- 所属投资账户
  instrument_id INTEGER NOT NULL,
  quantity VARCHAR(48) NOT NULL,             -- 数量（字符串高精度）
  cost_amount BIGINT DEFAULT 0,             -- 总成本（分）
  current_price VARCHAR(48) DEFAULT '',      -- 最新价
  market_value BIGINT DEFAULT 0,             -- 最新市值（分）
  sync_version INTEGER NOT NULL DEFAULT 0,   -- 同步版本（R06）
  deleted INTEGER NOT NULL DEFAULT 0,        -- 软删除墓碑（R06）
  updated_at INTEGER,
  UNIQUE(account_id, instrument_id)
);

-- 标的行情价格
CREATE TABLE instrument_prices (
  id INTEGER PRIMARY KEY, instrument_id INTEGER NOT NULL,
  price VARCHAR(48) NOT NULL, price_time INTEGER NOT NULL,
  source VARCHAR(32) DEFAULT '', created_at INTEGER
);
CREATE INDEX idx_instr_price ON instrument_prices(instrument_id, price_time);

-- 预算
CREATE TABLE budgets (
  id INTEGER PRIMARY KEY, book_id INTEGER NOT NULL,
  name VARCHAR(64) NOT NULL,
  budget_type TINYINT NOT NULL,              -- 1=分类预算 2=总预算
  category_id INTEGER DEFAULT 0,            -- 关联大类
  period_type TINYINT DEFAULT 1,             -- 1=月 2=季 3=年
  amount BIGINT NOT NULL,                     -- 预算金额（分）
  rollover BOOLEAN DEFAULT 0,
  enabled BOOLEAN DEFAULT 1,
  deleted BOOLEAN DEFAULT 0, created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE INDEX idx_budget_book ON budgets(book_id, deleted);
```

### 6.5 常用索引（查询慢了就加）

```sql
CREATE INDEX idx_tx_book_time ON transactions(book_id, tx_time);
CREATE INDEX idx_tx_book_acc  ON transactions(book_id, account_id);
CREATE INDEX idx_tx_book_cat  ON transactions(book_id, category_id);
CREATE INDEX idx_members_book ON book_members(book_id);
```

---

## 7. API 接口设计（前后端怎么对话）

### 7.1 基本约定

- 统一前缀 `/api/v1`；除登录注册外，请求头都要带 `Authorization: Bearer <token>`。
- 统一响应包：
  ```json
  { "code": 0, "msg": "ok", "data": { } }
  ```
  出错时 `code` 为错误码，HTTP 状态码语义化（400 参数错 / 401 未登录 / 403 无权限 / 404 不存在 / 422 校验失败 / 500 服务器错）。


交易创建接线契约：JSON 字段 `clientUuid` 必填（数据库名 `client_uuid`）；Handler 绑定后先校验，再调用唯一 `TxService.Create(ctx, uid, bookID, req)`。转账仍为同一个 POST，`txType=3`、`accountId` 为转出、`toAccountId` 为转入。`sync/push` 的每个离线创建项使用相同命令和稳定键。缺键不得退化为非幂等成功，详见 §13 与转账入口裁决。

### 7.2 完整接口清单

| 模块 | 方法 & 路径 | 说明 |
|---|---|---|
| 认证 | POST `/auth/register` | 注册（可配置是否开放） |
|  | POST `/auth/login` | 登录，返回 accessToken/refreshToken |
|  | POST `/auth/refresh` | 刷新 token |
|  | POST `/auth/logout` | 退出 |
|  | POST `/tokens/generate-api` | 生成长期 API Token（快捷指令/脚本用） |
| 用户 | GET `/users/me` | 当前用户资料 |
| 账本 | GET `/books` | 我加入的账本 |
|  | POST `/books` | 创建账本（创建者自动为 owner） |
|  | PUT `/books/{id}` | 改名称/可见性 |
|  | GET `/books/{id}/members` | 成员列表 |
|  | POST `/books/{id}/members/invite` | 生成邀请码 |
|  | POST `/books/{id}/members/join` | 凭邀请码加入 |
|  | PUT `/books/{id}/members/{uid}` | 改角色/权限 |
|  | DELETE `/books/{id}/members/{uid}` | 移除成员 |
| 账户 | GET `/books/{id}/accounts` | 账户列表 |
|  | POST `/books/{id}/accounts` | 新增 |
|  | PUT `/books/{id}/accounts/{aid}` | 修改 |
|  | DELETE `/books/{id}/accounts/{aid}` | 删除/隐藏 |
| 分类 | GET `/books/{id}/categories` | 分类（含 parent_id 两级） |
|  | POST/PUT/DELETE `/books/{id}/categories...` | 分类管理 |
| 交易 | GET `/books/{id}/transactions` | 列表（参数：maxTime,minTime,type,accountId,categoryId,keyword,page,count） |
|  | GET `/books/{id}/transactions/{txId}` | 详情 |
|  | POST `/books/{id}/transactions` | 新增（单条/批量） |
|  | PUT `/books/{id}/transactions/{txId}` | 修改 |
|  | DELETE `/books/{id}/transactions/{txId}` | 软删除 |
| 导出 | GET `/books/{id}/transactions/export` | 导出（format=csv/json + 时间/账户/分类筛选），返回文件流 |
| 识别 | POST `/books/{id}/recognition/parse-text` | 仅传 OCR 文本，返回结构化结果 |
| 标的 | GET/POST `/books/{id}/instruments` | 标的列表/新增 |
|  | PUT/DELETE `/books/{id}/instruments/{iid}` | 修改/删除 |
| 持仓 | GET/POST `/books/{id}/accounts/{aid}/holdings` | 持仓列表/新增 |
|  | PUT/DELETE `/books/{id}/holdings/{hid}` | 修改/删除 |
| 资产 | GET `/books/{id}/assets/summary` | 家庭资产合计 |
| 预算 | GET/POST `/books/{id}/budgets` | 预算列表（含已用/进度）/新增 |
|  | PUT/DELETE `/books/{id}/budgets/{bid}` | 修改/删除 |
| 同步 | GET `/books/{id}/sync/pull` | 拉增量（afterVersion） |
|  | POST `/books/{id}/sync/push` | 推离线队列（幂等 client_uuid） |

### 7.3 几个关键接口的请求/响应

**登录**
```json
// 请求
{ "username": "me", "password": "123456" }
// 响应
{ "code": 0, "data": { "accessToken": "xxx", "refreshToken": "yyy", "user": { "id": 1, "username": "me" } } }
```

**识别（只发文字，不发图）**
```json
// 请求
{ "text": "微信支付\n付款给 张三\n¥ 36.50\n2026-10-28 12:30" }
// 响应
{ "code": 0, "data": {
  "txType": 2, "amount": 3650, "merchant": "张三",
  "txTime": 1782621000, "paymentChannel": 1, "accountId": 0,
  "categoryId": 0, "confidence": 0.92 } }
```

**新增持仓（市值/盈亏后端算）**
```json
// 请求
{ "instrumentId": 12, "quantity": "100", "costAmount": 16000000, "currentPrice": "1650" }
// 响应
{ "code": 0, "data": {
  "id": 5, "marketValue": 16500000,
  "profitLoss": 500000, "profitLossRate": 0.03125 } }
```

**资产合计**
```json
{ "code": 0, "data": {
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
  "quoteMode": "manual" } }
```

### 7.4 WebSocket 协议（权威契约，客户端/服务端都按此实现）

- 连接地址：`wss://你的域名/api/v1/ws?token=<token>`
- 客户端连上后发送订阅（可一次订阅多个账本，**账本 ID 由服务端校验成员资格，不信任客户端**）：
  ```json
  { "action": "subscribe", "books": [1, 2] }
  ```
- 服务器在有新交易/改动时推送（事件名统一用点号；关闭互看时 `data` 为脱敏通知，需再走授权接口拉明细）：
  ```json
  { "event": "transaction.created", "bookId": 1, "version": 88, "data": { "id": 88, "amount": 3650 } }
  { "event": "transaction.updated", "bookId": 1, "version": 89, "data": { "id": 88 } }
  { "event": "transaction.deleted", "bookId": 1, "version": 90, "data": { "id": 123 } }
  { "event": "member.role_changed", "bookId": 1, "version": 91, "data": { "uid": 2, "role": 2 } }
  ```
- 断线自动重连：**WS 事件只用于"通知有新版本"，不直接据此推进已同步游标**；重连后用 `sync/pull?afterVersion=上次完整应用的版本` 补漏（见 §22）。

---
---

# 第三部分：后端开发（Go）

## 8. 项目初始化与目录结构

### 8.1 初始化模块

```bash
mkdir fastledger-server && cd fastledger-server
go mod init fastledger
```

### 8.2 安装依赖

```bash
go get github.com/gin-gonic/gin
go get xorm.io/xorm
go get github.com/mattn/go-sqlite3
go get github.com/golang-jwt/jwt/v5
go get golang.org/x/crypto/bcrypt
```

### 8.3 推荐目录结构（每个目录干什么）

```
fastledger-server/
├── cmd/
│   └── server/
│       └── main.go            # 程序入口
├── internal/
│   ├── config/                # 配置读取
│   │   └── config.go
│   ├── database/              # 数据库连接、初始化
│   │   └── db.go
│   ├── models/                # 数据对象（对应数据库表）
│   │   ├── user.go
│   │   ├── book.go
│   │   ├── account.go
│   │   ├── category.go
│   │   ├── transaction.go
│   │   ├── instrument.go
│   │   └── budget.go
│   ├── handlers/              # 路由层：接收请求、返回响应
│   │   ├── auth.go
│   │   ├── book.go
│   │   ├── account.go
│   │   ├── transaction.go
│   │   ├── recognition.go
│   │   ├── instrument.go
│   │   ├── asset.go
│   │   └── budget.go
│   ├── services/              # 逻辑层：业务规则
│   │   ├── auth.go
│   │   ├── transaction.go
│   │   ├── recognition.go
│   │   ├── asset.go
│   │   └── budget.go
│   ├── middleware/            # 中间件：JWT 校验、权限
│   │   └── jwt.go
│   └── ws/                    # WebSocket
│       └── hub.go
├── web/                       # Web 端静态文件（go:embed 打包）
│   └── dist/
├── config.yaml                # 配置文件
└── go.mod
```

> 新手记住一句话：**handler 管收发，service 管逻辑，model 管数据，三者别混着写**。

---

## 9. 配置与数据库连接

### 9.1 配置文件 `config.yaml`

```yaml
server:
  port: 8080
database:
  driver: sqlite3
  dsn: ./data/fastledger.db
jwt:
  secret: 改成你自己的一串随机字符串
  expire_hours: 24
```

### 9.2 数据库连接 `internal/database/db.go`

```go
package database

import (
	"log"

	_ "github.com/mattn/go-sqlite3"
	"xorm.io/xorm"
)

var Engine *xorm.Engine

// Init 初始化数据库连接并建表
func Init(driver, dsn string) {
	engine, err := xorm.NewEngine(driver, dsn)
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	// 显示 SQL（调试用，上线可关）
	engine.ShowSQL(true)

	// 自动建表（根据 model 结构体）
	if err := engine.Sync2(
	// 这里依次放你的 model，例如：
	// new(models.User), new(models.Book), ...
	); err != nil {
		log.Fatalf("建表失败: %v", err)
	}
	Engine = engine
}
```

### 9.3 一个 model 示例 `internal/models/user.go`

```go
package models

import "time"

type User struct {
	Id           int64      `xorm:"pk autoincr"`
	Username     string     `xorm:"varchar(64) notnull unique"`
	PasswordHash string     `xorm:"column(password_hash) varchar(128) notnull"` // bcrypt 哈希（自带盐）
	Nickname     string     `xorm:"varchar(64)"`
	Email        string     `xorm:"varchar(128) null"` // 可选，无邮箱留空（NULL）
	Disabled     bool       `xorm:"default 0"`
	Deleted      bool       `xorm:"default 0"`
	CreatedAt    time.Time  `xorm:"created"`
	UpdatedAt    time.Time  `xorm:"updated"`
	DeletedAt    *time.Time `xorm:"null"`
}
```

> 其他表的 model 照着这个写：字段名大写、用 xorm 标签标注列名和类型。

---

## 10. 用户注册登录（JWT）

### 10.1 认证逻辑 `internal/services/auth.go`

```go
package services

import (
	"errors"
	"fmt"
	"log"
	"time"

	"fastledger/internal/database"
	"fastledger/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// jwtSecret 必须由 main 从配置注入，禁止在此写死（R08：避免可预测的签名密钥）
var jwtSecret []byte

// InitAuth 在启动时从配置读取密钥；为空、仍是占位值或太短则直接失败，禁止带默认密钥启动
func InitAuth(secret string) {
	if secret == "" || secret == "改成你自己的密钥" || len(secret) < 16 {
		log.Fatalf("jwt.secret 未正确配置：请使用至少 16 位的随机字符串，且不能是占位值")
	}
	jwtSecret = []byte(secret)
}

// Register 注册
func Register(username, password string) (*models.User, error) {
	if len(username) < 2 || len(password) < 6 {
		return nil, errors.New("用户名至少2位、密码至少6位")
	}
	// 检查是否已存在
	exist, _ := database.Engine.Where("username=?", username).Get(&models.User{})
	if exist {
		return nil, errors.New("用户名已存在")
	}
	// 密码加密（bcrypt 会自动加盐）
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &models.User{Username: username, PasswordHash: string(hash)}
	_, err = database.Engine.Insert(user)
	return user, err
}

// Login 登录，成功返回 token
func Login(username, password string) (string, error) {
	user := &models.User{}
	has, _ := database.Engine.Where("username=?", username).Get(user)
	if !has || user.Deleted || user.Disabled { // R09：已删除/禁用账号即便凭据匹配也拒绝
		return "", errors.New("用户名或密码错误")
	}
	// 校验密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("用户名或密码错误")
	}
	// 生成 JWT（jti 用于可撤销；签发/校验/刷新共用同一 jwtSecret）
	claims := jwt.MapClaims{
		"uid":      user.Id,
		"username": user.Username,
		"jti":      fmt.Sprintf("%d-%d", user.Id, time.Now().UnixNano()), // 每次登录唯一，撤销时据此登记
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}
```

### 10.2 JWT 中间件 `internal/middleware/jwt.go`

```go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			// 仅允许 HS256，拒绝 alg=none 等算法替换攻击
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("非预期签名算法")
			}
			return jwtSecret, nil // 与 InitAuth 注入的同一份，不再接受外部传入的密钥
		})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "登录已失效"})
			return
		}
		// 安全读取 uid（非法 claims 不要 panic，稳定返回 401）（R09）
		uidF, ok := claims["uid"].(float64)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "令牌无效"})
			return
		}
		uid := int64(uidF)
		// 账号失效检查：已删除/禁用的账号即便签名有效也拒绝（R09）
		alive, _ := database.Engine.Where("id=? AND deleted=0", uid).Exist(&models.User{})
		if !alive {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "账号已失效"})
			return
		}
		// 撤销检查（R09，可选）：登录时已把 jti 写入 auth_tokens，此处查 revoked；
		// if revoked, _ := isTokenRevoked(claims["jti"]); revoked { c.AbortWithStatusJSON(401,...) }
		c.Set("uid", uid)
		c.Next()
	}
}
```

### 10.3 认证 handler `internal/handlers/auth.go`

```go
package handlers

import (
	"net/http"

	"fastledger/internal/services"

	"github.com/gin-gonic/gin"
)

func Register(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	user, err := services.Register(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"id": user.Id, "username": user.Username}})
}

func Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	_ = c.ShouldBindJSON(&req)
	token, err := services.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"accessToken": token}})
}
```

**验证：** 用 Postman 或 curl 调 `POST /api/v1/auth/register` 再调 `/login`，能拿到 token 就成功。

---

## 11. 账本与成员

### 11.1 关键逻辑

- 创建账本：插入 books，并把创建者写进 book_members（role=owner）。
- 加入账本：凭邀请码，插入 book_members（role=member）。
- 所有后续操作前，中间件要校验"当前用户是否为该账本成员"。

### 11.2 成员校验（建议做成一个工具函数）

```go
// 检查 uid 是否为 bookId 的成员，返回角色
func GetMemberRole(bookId, uid int64) (int, error) {
	m := &models.BookMember{}
	has, err := database.Engine.Where("book_id=? AND uid=?", bookId, uid).Get(m)
	if err != nil || !has {
		return 0, errors.New("不是该账本成员")
	}
	return int(m.Role), nil
}
```

### 11.3 权限矩阵

| 操作 | owner | admin | member |
|---|---|---|---|
| 改/删账本 | ✅ | ❌ | ❌ |
| 邀请/移除成员、改角色 | ✅ | ✅（不能动 owner） | ❌ |
| 增删改自己的交易 | ✅ | ✅ | ✅ |
| 看他人交易 | 受账本开关控制（默认可见） | 同左 | 同左 |
| 管理标的/持仓/资产 | ✅ | ✅ | 自己的 |
| 看家庭资产合计 | ✅ | ✅ | 受开关控制 |

---

## 12. 账户与分类

- 账户、分类都属于账本（带 book_id），CRUD 照标准写法：列表 `Find`、新增 `Insert`、修改 `ID(...).Update`、删除用软删除 `deleted=1`。
- 分类列表返回时，前端按 `parent_id` 组装成两级树；后端也可直接返回树结构。
- 新建账本时插入默认分类（餐饮、交通、购物……）和默认账户（现金、储蓄卡、信用卡）。

---

## 13. 交易（含转账、余额）

这是后端最核心的业务逻辑，**必须用数据库事务**，保证余额和交易同时成功或同时失败。

### 13.1 唯一交易创建入口 `internal/services/transaction.go`

**裁决已确定：收入、支出、转账统一进入 `TxService.Create`。** 手动 HTTP 创建和 `sync/push` 的每个离线创建命令都调用同一入口；不再保留可独立提交的 `AddTransaction` 或 `Transfer`。完整理由与边界见 [转账入口裁决](FastLedger-转账入口裁决.md)。

- 一次业务命令只占位一次幂等回执、只开启一个事务、只递增一次账本版本。转账作为内部 `createTransferInTx` 分支生成两条记录，不得通过调用两次普通记账来实现。
- Handler 校验 `clientUuid` 必填，Service 再校验空值/全空白/长度。数据库列名仍为 `client_uuid`；不提供空键绕过公开路径的例外。内部批处理也需稳定的操作键。
- 依赖的 `VersionSource` 由《落地方案》§6.1 的 `VersionRepo.Next` 实现；下列代码不另写版本递增 SQL。`models.Account` 须含 `Currency string` 并映射 `currency`；其他模型与 §6.4 保持一致。
- 以下为本轮修订的可执行参考骨架；隔离验证工具从本段抽取源代码编译测试。它仍不是完整生产后端，认证/HTTP 适配、真实 WS 与部署须在正式工程验收。

```go
package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"fastledger/internal/models"
	"xorm.io/xorm"
)

var (
	ErrInvalidCreate     = errors.New("invalid transaction create request")
	ErrCreateConflict    = errors.New("clientUuid reused with a different operation")
	ErrCreateForbidden   = errors.New("not a member of the active book")
	ErrIncompleteReceipt = errors.New("committed idempotency receipt is incomplete")
)

// JSON uses clientUuid; the database column is client_uuid. The same command
// is used for manual creation and for each item in an offline sync/push batch.
type CreateRequest struct {
	ClientUUID     string `json:"clientUuid" binding:"required"`
	TxType         int    `json:"txType"`
	AccountID      int64  `json:"accountId"`
	ToAccountID    int64  `json:"toAccountId"`
	Amount         int64  `json:"amount"`
	CategoryID     int64  `json:"categoryId"`
	PaymentChannel int    `json:"paymentChannel"`
	TxTime         int64  `json:"txTime"`
	TimezoneOffset int    `json:"timezoneOffset"`
	Comment        string `json:"comment"`
	Source         int    `json:"source"`
}

type CreateResult struct {
	ResultType     int     `json:"resultType"`     // 1=single, 2=transfer
	TransactionIDs []int64 `json:"transactionIds"` // transfer: [outID,inID]
	Version        int64   `json:"version"`
	Replayed       bool    `json:"replayed"`
}

// The implementation is VersionRepo.Next in the plan, section 6.1. No second
// INSERT/ON CONFLICT implementation belongs in this file or in sync/push.
type VersionSource interface {
	Next(*xorm.Session, int64) (int64, error)
}
type TxService struct {
	db       *xorm.Engine
	versions VersionSource
}

func NewTxService(db *xorm.Engine, versions VersionSource) *TxService {
	return &TxService{db: db, versions: versions}
}

func validateCreate(uid, bookID int64, r CreateRequest) error {
	if uid <= 0 || bookID <= 0 || strings.TrimSpace(r.ClientUUID) == "" || len(r.ClientUUID) > 64 ||
		r.Amount <= 0 || r.AccountID <= 0 || r.TxTime <= 0 || r.TxType < 1 || r.TxType > 3 ||
		r.PaymentChannel < 0 || r.PaymentChannel > 5 || r.Source < 0 || r.Source > 1 ||
		r.TimezoneOffset < -840 || r.TimezoneOffset > 840 || !utf8.ValidString(r.Comment) || utf8.RuneCountInString(r.Comment) > 255 {
		return ErrInvalidCreate
	}
	if r.TxType == 3 {
		if r.ToAccountID <= 0 || r.ToAccountID == r.AccountID || r.CategoryID != 0 {
			return ErrInvalidCreate
		}
	} else if r.ToAccountID != 0 || r.CategoryID <= 0 {
		return ErrInvalidCreate
	}
	return nil
}

func createRequestHash(r CreateRequest) (string, error) {
	// A typed value gives deterministic field order. Normalization above is
	// stable across retries; time/defaults must not change on every retry.
	payload := struct {
		Domain  string        `json:"domain"`
		Request CreateRequest `json:"request"`
	}{
		Domain: "transaction.create/v1", Request: r,
	}
	// The key scopes the receipt and is not part of business identity.
	payload.Request.ClientUUID = ""
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// Create is the only public domain entry point and owns the sole transaction,
// receipt and version increment. Helpers never Begin/Commit or claim receipts.
func (s *TxService) Create(ctx context.Context, uid, bookID int64, r CreateRequest) (*CreateResult, error) {
	if err := validateCreate(uid, bookID, r); err != nil {
		return nil, err
	}
	hash, err := createRequestHash(r)
	if err != nil {
		return nil, err
	}
	kind := 1
	if r.TxType == 3 {
		kind = 2
	}
	sess := s.db.NewSession()
	defer sess.Close()
	sess.Context(ctx)
	if err = sess.Begin(); err != nil {
		return nil, err
	}
	defer sess.Rollback() // harmless after successful Commit; releases every failure path

	var member struct{ Role int }
	has, err := sess.SQL(`SELECT m.role FROM book_members m JOIN books b ON b.id=m.book_id
        WHERE m.book_id=? AND m.uid=? AND b.deleted=0`, bookID, uid).Get(&member)
	if err != nil {
		return nil, err
	}
	if !has || member.Role < 1 || member.Role > 3 {
		return nil, ErrCreateForbidden
	}

	// Ignore only the explicitly targeted key collision. BUSY, FULL, IOERR,
	// trigger failures and other constraint failures remain errors.
	res, err := sess.Exec(`INSERT INTO idempotency_records
        (book_id,uid,client_uuid,request_hash,result_type,result_ids,result_version,created_at)
        VALUES(?,?,?,?,?,'',0,?)
        ON CONFLICT(book_id,uid,client_uuid) DO NOTHING`, bookID, uid, r.ClientUUID, hash, kind, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		var rec struct {
			RequestHash   string
			ResultType    int
			ResultIds     string
			ResultVersion int64
		}
		has, err := sess.Table("idempotency_records").Where("book_id=? AND uid=? AND client_uuid=?", bookID, uid, r.ClientUUID).Get(&rec)
		if err != nil {
			return nil, err
		}
		if !has {
			return nil, ErrIncompleteReceipt
		}
		if rec.RequestHash != hash || rec.ResultType != kind {
			return nil, ErrCreateConflict
		}
		ids, err := decodeResultIDs(rec.ResultIds, kind)
		if err != nil || rec.ResultVersion <= 0 {
			return nil, ErrIncompleteReceipt
		}
		if err = sess.Commit(); err != nil {
			return nil, err
		}
		return &CreateResult{ResultType: kind, TransactionIDs: ids, Version: rec.ResultVersion, Replayed: true}, nil
	}
	if n != 1 {
		return nil, ErrIncompleteReceipt
	}

	account, err := createAccount(sess, bookID, r.AccountID)
	if err != nil {
		return nil, err
	}
	var other *models.Account
	if r.TxType == 3 {
		other, err = createAccount(sess, bookID, r.ToAccountID)
		if err != nil {
			return nil, err
		}
		if account.Currency != other.Currency {
			return nil, ErrInvalidCreate
		} // MVP: no implicit FX
	} else {
		var category struct{ CatType int }
		has, err := sess.Table("categories").Where("id=? AND book_id=? AND deleted=0", r.CategoryID, bookID).Get(&category)
		if err != nil {
			return nil, err
		}
		if !has || category.CatType != r.TxType {
			return nil, ErrInvalidCreate
		}
	}
	version, err := s.versions.Next(sess, bookID)
	if err != nil {
		return nil, err
	}
	if version <= 0 {
		return nil, ErrIncompleteReceipt
	}
	var ids []int64
	if r.TxType == 3 {
		ids, err = s.createTransferInTx(sess, uid, bookID, r, account, other, version)
	} else {
		var id int64
		id, err = s.createSingleInTx(sess, uid, bookID, r, account, version)
		ids = []int64{id}
	}
	if err != nil {
		return nil, err
	}
	encoded := make([]string, len(ids))
	for i, id := range ids {
		encoded[i] = strconv.FormatInt(id, 10)
	}
	res, err = sess.Exec(`UPDATE idempotency_records SET result_ids=?,result_version=?
        WHERE book_id=? AND uid=? AND client_uuid=? AND request_hash=? AND result_type=?`,
		strings.Join(encoded, ","), version, bookID, uid, r.ClientUUID, hash, kind)
	if err != nil {
		return nil, err
	}
	n, err = res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrIncompleteReceipt
	}
	if err = sess.Commit(); err != nil {
		return nil, err
	}
	// A caller may now publish an invalidation notice. Notification failure
	// cannot change this committed outcome; pull remains the authoritative feed.
	return &CreateResult{ResultType: kind, TransactionIDs: ids, Version: version}, nil
}

func decodeResultIDs(value string, kind int) ([]int64, error) {
	parts := strings.Split(value, ",")
	want := 1
	if kind == 2 {
		want = 2
	}
	if len(parts) != want {
		return nil, ErrIncompleteReceipt
	}
	ids := make([]int64, want)
	for i, p := range parts {
		v, e := strconv.ParseInt(p, 10, 64)
		if e != nil || v <= 0 {
			return nil, ErrIncompleteReceipt
		}
		ids[i] = v
	}
	if want == 2 && ids[0] == ids[1] {
		return nil, ErrIncompleteReceipt
	}
	return ids, nil
}
func createAccount(sess *xorm.Session, bookID, id int64) (*models.Account, error) {
	a := &models.Account{}
	has, err := sess.Where("id=? AND book_id=? AND deleted=0", id, bookID).Get(a)
	if err != nil {
		return nil, err
	}
	if !has || a.Category < 1 || a.Category > 15 || a.Currency != "CNY" {
		return nil, ErrInvalidCreate
	}
	return a, nil
}

// flow is signed cash flow: outflow negative, inflow positive. Debt uses
// positive outstanding amounts, so its stored balance delta is reversed.
func createBalanceDelta(category int, flow int64) int64 {
	if category >= 10 {
		return -flow
	}
	return flow
}
func createChangeBalance(sess *xorm.Session, bookID int64, a *models.Account, delta, version int64) error {
	low, high := int64(math.MinInt64), int64(math.MaxInt64)
	if delta > 0 {
		high -= delta
	}
	if delta < 0 {
		low -= delta
	}
	result, err := sess.Exec(`UPDATE accounts SET balance=balance+?,sync_version=?,updated_at=?
        WHERE id=? AND book_id=? AND deleted=0 AND balance BETWEEN ? AND ?`,
		delta, version, time.Now().Unix(), a.Id, bookID, low, high)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("account update failed or would overflow: %w", ErrInvalidCreate)
	}
	return nil
}
func createRow(uid, bookID int64, r CreateRequest, version int64) *models.Transaction {
	return &models.Transaction{BookId: bookID, CreatorUid: uid, TxType: r.TxType, AccountId: r.AccountID,
		CategoryId: r.CategoryID, Amount: r.Amount, PaymentChannel: r.PaymentChannel, TxTime: r.TxTime,
		TimezoneOffset: r.TimezoneOffset, Comment: r.Comment, Source: r.Source, ClientUuid: r.ClientUUID, SyncVersion: version}
}
func createInsertRow(sess *xorm.Session, row *models.Transaction) error {
	n, err := sess.Insert(row)
	if err != nil { return err }
	if n != 1 || row.Id <= 0 { return ErrIncompleteReceipt }
	return nil
}
func (s *TxService) createSingleInTx(sess *xorm.Session, uid, bookID int64, r CreateRequest, a *models.Account, version int64) (int64, error) {
	flow := r.Amount
	if r.TxType == 2 {
		flow = -flow
	}
	if err := createChangeBalance(sess, bookID, a, createBalanceDelta(a.Category, flow), version); err != nil {
		return 0, err
	}
	row := createRow(uid, bookID, r, version)
	if err := createInsertRow(sess, row); err != nil {
		return 0, err
	}
	return row.Id, nil
}
func (s *TxService) createTransferInTx(sess *xorm.Session, uid, bookID int64, r CreateRequest, from, to *models.Account, version int64) ([]int64, error) {
	if err := createChangeBalance(sess, bookID, from, createBalanceDelta(from.Category, -r.Amount), version); err != nil {
		return nil, err
	}
	if err := createChangeBalance(sess, bookID, to, createBalanceDelta(to.Category, r.Amount), version); err != nil {
		return nil, err
	}
	out, in := createRow(uid, bookID, r, version), createRow(uid, bookID, r, version)
	out.RelatedAccountId = to.Id
	out.RelatedAmount = r.Amount
	in.AccountId = to.Id
	in.RelatedAccountId = from.Id
	in.RelatedAmount = r.Amount
	if err := createInsertRow(sess, out); err != nil {
		return nil, err
	}
	in.RelatedId = out.Id
	if err := createInsertRow(sess, in); err != nil {
		return nil, err
	}
	result, err := sess.Exec("UPDATE transactions SET related_id=? WHERE id=? AND book_id=?", in.Id, out.Id, bookID)
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrIncompleteReceipt
	}
	return []int64{out.Id, in.Id}, nil
}
```

`CreateResult.TransactionIDs` 对单笔返回一个 ID，对转账按 `[转出记录ID,转入记录ID]` 返回；回放必须返回同一组 ID 与原 `Version`，不能只返回空成功。回执的 `result_version` 保存创建时水位，不从后来可能已编辑的交易行倒推。

### 13.2 转账分支、幂等及接线约束

| 层次 | 唯一入口/职责 | 不允许的替代 |
|---|---|---|
| HTTP | `POST /api/v1/books/:id/transactions`，绑定 `CreateRequest` 后调用 `TxService.Create` | 新增独立 `/transfer` 事务入口 |
| 离线同步 | `sync/push` 的每条创建命令携带稳定 `clientUuid`，调用同一个 `Create` | 给两条镜像记录分别提交普通记账 |
| 领域 Service | `Create` 拥有事务、授权复查、幂等、版本与提交 | 兼有另一份可独立提交的 `Transfer` |
| 转账内部 | `createTransferInTx(sess,...,version)`，两侧余额与记录同生共死 | 内部再 `Begin/Commit`、再占位/加版本 |
| 通知 | 仅在 `Create` 成功提交后发送失效通知，客户端经授权 pull | 通知失败改变已提交账务的结果 |

信用卡以正数记欠款，因此储蓄卡还信用卡时两边存储余额都减少；资产→资产是一减一增。这里保证的是两侧**净资产贡献相抵**，不是无条件要求两个原始余额一减一增。账本账户/两条记录一起写同一版本，时间来自命令，不能落成 Unix 0。

幂等处理必须区分：① 同键同内容且回执完整，返回原结果；② 同键不同类型或内容，409；③ BUSY/IOERR/FULL/触发器等数据库错误，返回错误并回滚，不能当成重试成功。占位和结果回写均检查错误与影响行数。SQLite 瞬时锁冲突需要调用方按同键重试整个命令，不能吞错或从业务中途续跑。

HTTP 映射：缺键/非法命令为 400，幂等键冲突为 409，无成员权限为 403；存储错误使用统一 5xx/可重试响应，不外露原始 SQL。该映射还需正式 Gin handler 集成测试；本轮隔离测试不冒称验证了线上 HTTP。

> 签名迁移：旧 `AddTransaction(uid,bookID,tx)` 与 `Transfer(uid,bookID,from,to,amount,clientUuid)` 退出有效骨架，调用方统一构造 `CreateRequest`。旧函数只在审阅基线和 `gpt/v1.4/verification/source/` 的反例快照保留，不能复制成第二条写路径。转账方向的长期存储/删除恢复协议仍需正式工程覆盖，本轮不重写删除与恢复服务。

---

## 14. WebSocket 实时推送

### 14.1 连接管理 `internal/ws/hub.go`

```go
package ws

import (
	"context"
	"sync"

	"fastledger/internal/services" // CanSubscribe / CanViewDetail（见落地方案权限矩阵 §5.5）
)

// Client 一个客户端连接
type Client struct {
	mu      sync.Mutex
	BookIds map[int64]bool // 仅保存“已通过服务端校验”的订阅账本
	Send    chan []byte    // 约定：永不 close（多广播者并发发送，关闭易 panic），靠 Unregister 回收
	Dropped int64          // Send 满导致丢弃的通知数（c.mu 下维护）
	UID     int64          // 连接绑定认证用户（R05）
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
}

var DefaultHub = &Hub{clients: make(map[*Client]bool)}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

// Subscribe 经 Hub 受锁更新订阅（R18：所有订阅写都走同一把锁），
// 且每个 bookId 都先由服务端校验成员资格（R05），不信任客户端上报的账本 ID。
func (h *Hub) Subscribe(c *Client, bookIDs []int64, uid int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, b := range bookIDs {
		if services.CanSubscribe(uid, b) { // 校验 uid 是否为该账本成员
			c.mu.Lock()
			c.BookIds[b] = true
			c.mu.Unlock()
		}
	}
	c.UID = uid
}

// BroadcastToBook 给“有权看到该账本”的在线成员推消息。
// R05：广播前按接收者权限过滤；关闭互看时不发金额/账户/备注等敏感字段，
// 仅发脱敏通知（sensitiveMsg 含 {event, bookId}），由客户端再走授权接口拉取明细。
func (h *Hub) BroadcastToBook(bookId int64, msg []byte, sensitiveMsg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		c.mu.Lock()
		sub := c.BookIds[bookId]
		c.mu.Unlock()
		if !sub {
			continue
		}
		payload := msg
		if !services.CanViewDetail(c.UID, bookId) {
			payload = sensitiveMsg // 无权看他人明细：只通知“有更新”
		}
		// 非阻塞投递：Send 满不阻塞广播。WS 只是"尽力通知"，不是数据通道，
		// 丢一个通知由客户端的周期性 pull 兜底（见下方 R18 约定与 §14.3 心跳）。
		select {
		case c.Send <- payload:
		default:
			c.mu.Lock(); c.Dropped++; c.mu.Unlock()
		}
	}
}
```

**并发与所有权约定（R18，照此实现，避免 data race / 死锁 / panic）：**

1. **锁顺序固定单向**：永远是 `Hub.mu → Client.mu`（Subscribe、BroadcastToBook 都如此），**禁止任何路径反向持有**（先 Client.mu 再 Hub.mu），否则会死锁。
2. **订阅 map 的所有写都经 Hub**：`Subscribe` 持有 `Hub.mu`，并对每个账本先 `CanSubscribe` 校验成员资格，再在 `Client.mu` 下写 `BookIds`；handler 不直接改 `BookIds`。
3. **`Send` 通道永不关闭**：存在多个并发广播者，关闭 channel 会在其他 goroutine 发送时触发 panic；连接结束只做 `Unregister`（从 `clients` 删除），此后不再被遍历投递，channel 由 GC 回收。写循环里 `m, ok := <-Send` 的 `ok==false` 仅作防御性分支，正常不会发生。
4. **读写循环共用一个取消上下文，清理恰好一次**：`ServeWS` 主 defer 中 `cancel()` + `Unregister()` + `conn.Close()` 各执行一次；读循环 goroutine 只 `cancel()`，不重复 `Unregister`。`conn.Close()` 会让阻塞的 `ReadMessage` 退出。
5. **通知丢失最终一致**：客户端保证 ① 收到任意事件触发一次（去抖）pull；② 周期性心跳 pull（如每 60s）；③ 重连必 pull。数据以 pull 的一致快照为准，故单个 WS 通知丢失不会造成永久不一致。

### 14.2 在交易成功提交后推送

```go
// 位于 HTTP/同步适配层：Create 内部不导入 ws，避免 services ↔ ws 循环依赖。
result, err := txService.Create(c.Request.Context(), uid, bookID, req)
if err != nil { /* 按 §13.2 映射 HTTP/同步错误，结束本次请求 */ return }
notice, err := json.Marshal(map[string]any{
    "event": "transaction.created", "bookId": bookID, "version": result.Version,
})
if err == nil {
    // WS 只通知有更新，不携带交易明细；真正数据走授权 pull。
    ws.DefaultHub.BroadcastToBook(bookID, notice, notice)
}
// 通知失败记录日志/由后续 pull 兜底；不能把已提交命令改为失败或重做。
```

### 14.3 WS handler（接收连接、读写循环）

```go
func ServeWS(c *gin.Context) {
	// 连接必须先通过 JWT 鉴权（JWTAuth 已把 uid 写入上下文）
	uidVal, ok := c.Get("uid")
	if !ok {
		return // 未鉴权，直接拒绝升级
	}
	uid := uidVal.(int64)

	// 升级为 WebSocket（用 gorilla/websocket）
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := &Client{BookIds: make(map[int64]bool), Send: make(chan []byte, 256)}
	ws.DefaultHub.Register(client)

	// 读/写循环共用一个取消上下文，任一方结束都恰好清理一次（R18）
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		ws.DefaultHub.Unregister(client)
		conn.Close()
	}()

	// 读循环：处理 subscribe（经 Hub 受锁 + 服务端校验，R05/R18）
	go func() {
		defer cancel()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var sub struct {
				Action string  `json:"action"`
				Books  []int64 `json:"books"`
			}
			if json.Unmarshal(msg, &sub) == nil && sub.Action == "subscribe" {
				ws.DefaultHub.Subscribe(client, sub.Books, uid)
			}
		}
	}()

	// 写循环：发消息 + 定时 ping（保活、检测断线）。Send 不关闭，循环靠 ctx / 写错误退出。
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-client.Send: // Send 不关闭，无需 ok 分支
			if err := conn.WriteMessage(websocket.TextMessage, m); err != nil {
				return
			}
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
```

> 客户端侧：收到 ping 自动回 pong（gorilla 与 web_socket_channel 默认处理）；同时按 §14.1 约定周期性发起 pull（如每 60s），作为 WS 通知丢失的兜底。

---

## 15. 识别接口（OCR 文本兜底）

### 15.1 思路

- 端侧规则能解析的，根本不调后端；
- 规则没命中，把 OCR 文字发给后端，后端用 LLM 解析（参考 ezBookkeeping 的 `pkg/llm` 抽象）。

### 15.2 LLM 抽象（可接 OpenAI / 本地 Ollama）

```go
type LLMProvider interface {
	// 把账单文本解析成结构化结果
	ParseBill(text string) (BillResult, error)
}
```
- 用一个固定的提示词（prompt）要求模型只返回 JSON：金额、类型、商户、时间、支付渠道；
- 配置里填 API Key、模型名、接口地址；不想用云端可接本地 Ollama（数据也不出局域网）。

### 15.3 handler

```go
func ParseText(c *gin.Context) {
	var req struct{ Text string `json:"text"` }
	_ = c.ShouldBindJSON(&req)
	result, err := llm.ParseBill(req.Text)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "识别失败"})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": result})
}
```

---

## 16. 投资模块（标的/持仓/资产合计）

### 16.1 新增持仓（算市值、盈亏）

```go
// AddHolding 新增持仓。bookID 来自可信路径；持仓随账本同步（R11/R06）。
func AddHolding(uid, bookID, accountId, instrumentId int64,
	quantity string, costAmount int64, currentPrice string, versions VersionSource) (*models.Holding, error) {

	// 1) 校验数量/价格（R11：decimal 解析错误必须返回，不能像旧代码那样丢弃）
	qty, err := decimal.NewFromString(quantity)
	if err != nil || qty.Sign() < 0 {
		return nil, errors.New("持仓数量必须是非负的合法数字")
	}
	price, err := decimal.NewFromString(currentPrice)
	if err != nil || price.Sign() < 0 {
		return nil, errors.New("最新价必须是非负的合法数字")
	}

	sess := database.Engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}

	// 2) 校验投资账户（category=5）与标的都归属同一账本（R01）
	acc := &models.Account{}
	has, err := sess.Where("id=? AND book_id=? AND category=5 AND deleted=0", accountId, bookID).Get(acc)
	if err != nil { sess.Rollback(); return nil, err }
	if !has { sess.Rollback(); return nil, errors.New("投资账户不存在或不属于该账本") }
	inst := &models.Instrument{}
	has, err = sess.Where("id=? AND book_id=? AND deleted=0", instrumentId, bookID).Get(inst)
	if err != nil { sess.Rollback(); return nil, err }
	if !has { sess.Rollback(); return nil, errors.New("标的不存在或不属于该账本") }

	// 3) 版本号 +1
	newVersion, err := versions.Next(sess, bookID)
	if err != nil { sess.Rollback(); return nil, err }

	// 4) 市值 = 数量 × 最新价（decimal 精确），换算成分
	mvInt := qty.Mul(price).Mul(decimal.NewFromInt(100)).IntPart()

	h := &models.Holding{
		BookId: bookID, AccountId: accountId, InstrumentId: instrumentId,
		Quantity: quantity, CostAmount: costAmount, CurrentPrice: currentPrice,
		MarketValue: mvInt, SyncVersion: newVersion,
	}
	if _, err := sess.Insert(h); err != nil { sess.Rollback(); return nil, err }
	if err := sess.Commit(); err != nil { return nil, err }
	return h, nil
}
```

> 数量/价格用字符串 + `shopspring/decimal`，避免股票、基金份额的浮点误差；decimal 解析失败必须返回错误。

### 16.2 家庭资产合计

```go
func AssetSummary(bookId int64) map[string]any {
	// 按账户分类汇总
	accounts := []models.Account{}
	database.Engine.Where("book_id=? AND deleted=0", bookId).Find(&accounts)

	totalAssets, totalLiab := int64(0), int64(0)
	assetByType := map[int64]int64{}
	liabByType := map[int64]int64{}

	// 负债类账户(10~15)的 balance 按《枚举字典》§2 约定存储为“正数=欠款”，
	// 故净资产恒为 Σ(资产余额) − Σ(负债余额)，与 §13.1 signedBalanceDelta 写入口径一致。
	for _, a := range accounts {
		val := a.Balance
		if a.Category == 5 { // 投资账户：现金 + 该账户持仓市值
			val += sumHoldingsValue(a.Id)
		}
		if a.Category <= 9 {
			totalAssets += val
			assetByType[int64(a.Category)] += val
		} else {
			totalLiab += val
			liabByType[int64(a.Category)] += val
		}
	}
	return map[string]any{
		"totalAssets": totalAssets,
		"totalLiabilities": totalLiab,
		"netAssets": totalAssets - totalLiab,
		"assetByType": assetByType,
		"liabilityByType": liabByType,
		"quoteMode": "manual", // v0.3 自动行情后改 auto
	}
}
```

### 16.3 自动行情（v0.3 才实现，骨架先留接口）

```go
type QuoteProvider interface {
	GetQuotes(symbols []string) (map[string]string, error)
}
// 实现东方财富/新浪/腾讯（A股、港股、基金净值、黄金）
// 用 cron 定时：拉最新价 → 写 instrument_prices → 更新 holdings 市值
```

---

## 17. 账单导出

### 17.1 CSV 导出（带 BOM，Excel 不乱码）

```go
func ExportCSV(c *gin.Context) {
	// 1. 按筛选条件查询
	txs := []models.Transaction{}
	database.Engine.Where("book_id=? AND ...", c.Param("id")).Find(&txs)

	// 2. 写 CSV
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM
	w := csv.NewWriter(&buf)
	w.Write([]string{"时间", "类型", "金额", "分类", "账户", "渠道", "备注"})
	for _, t := range txs {
		w.Write([]string{
			time.Unix(t.TxTime, 0).Format("2006-01-02 15:04"),
			typeName(t.TxType),
			fmt.Sprintf("%.2f", float64(t.Amount)/100), // 服务端格式化的数字，不作文本防注入
			sanitizeCSVCell(categoryName(t.CategoryId)), // ↓ 用户可控文本，防公式注入（R14）
			sanitizeCSVCell(accountName(t.AccountId)),
			channelName(t.PaymentChannel),
			sanitizeCSVCell(t.Comment),
		})
	}
	w.Flush()

	// 3. 作为文件下载
	c.Header("Content-Disposition", "attachment; filename=transactions.csv")
	c.Data(200, "text/csv", buf.Bytes())
}

// sanitizeCSVCell 防 CSV 公式注入（R14，参考 OWASP CSV Injection）：
// 用户可控文本若以 =、+、-、@、Tab、CR 开头，前置单引号，强制 Excel/LibreOffice 按文本处理。
func sanitizeCSVCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}
```

> 说明：① 仅对"用户可控文本"（分类名、账户名、备注等）做防护；服务端生成的数值列保持原样。② 区分两种导出：**给表格软件看的 CSV** 做此防护；**给程序消费的 JSON / 原始数据**保持原样（JSON 不会被当公式）。③ BOM 只解决乱码，不等于公式防护。

### 17.2 JSON 导出

直接把查询结果 `c.JSON(200, txs)`，加 `Content-Disposition: attachment`。

---

## 18. 预算

### 18.1 预算进度（实时算，不存已用）

```go
type BudgetView struct {
	Budget   models.Budget
	Used     int64
	Progress float64 // 0~1（>1 表示超支）
	Status   string  // ok / near / over
}

func BudgetProgress(bookId int64, periodStart, periodEnd int64) []BudgetView {
	budgets := []models.Budget{}
	database.Engine.Where("book_id=? AND enabled=1 AND deleted=0", bookId).Find(&budgets)

	views := []BudgetView{}
	for _, b := range budgets {
		var used int64
		if b.BudgetType == 2 { // 总预算：不限分类（R13）
			used, _ = sumExpense(bookId, periodStart, periodEnd, nil)
		} else { // 分类预算：大类 + 其下所有小类（R13，原 SQL 漏算小类）
			catIds := []int64{b.CategoryId}
			children := []models.Category{}
			database.Engine.Where("book_id=? AND parent_id=? AND deleted=0", bookId, b.CategoryId).Find(&children)
			for _, c := range children {
				catIds = append(catIds, c.Id)
			}
			used, _ = sumExpense(bookId, periodStart, periodEnd, catIds)
		}

		progress := 0.0
		if b.Amount > 0 {
			progress, _ = new(big.Float).Quo(
				big.NewFloat(float64(used)), big.NewFloat(float64(b.Amount))).Float64()
		}
		status := "ok"
		switch {
		case used > b.Amount:
			status = "over" // 超支
		case progress >= 0.8:
			status = "near" // 接近
		}
		views = append(views, BudgetView{Budget: b, Used: used, Progress: progress, Status: status})
	}
	return views
}

// sumExpense 统计周期内（半开区间 [start,end)）的支出；catIds 为 nil 时不限分类
func sumExpense(bookId, start, end int64, catIds []int64) (int64, error) {
	q := database.Engine.Table("transactions").
		Where("book_id=? AND tx_type=2 AND deleted=0 AND tx_time>=? AND tx_time<?", bookId, start, end)
	if catIds != nil {
		q = q.In("category_id", catIds)
	}
	return q.SumInt(&models.Transaction{}, "amount")
}
```

> 说明：① 分类预算按"大类 + 全部小类"汇总，未来多级分类时改为递归收集；② 总预算不带任何分类条件；③ 周期用半开区间 `[start,end)`，避免边界时刻的交易被相邻两期重复计算；④ 时区以账本设置为准。

---
---

# 第四部分：客户端开发（Flutter）

## 19. Flutter 工程与网络层

### 19.1 创建工程

```bash
flutter create --org com.fastledger fastledger_app
cd fastledger_app
# 支持的平台（按你装了哪些）
flutter config --enable-windows-desktop
```

### 19.2 目录结构

```
lib/
├── main.dart
├── api/
│   └── api_client.dart       # 封装网络请求（带 token）
├── models/                   # 数据模型
├── pages/
│   ├── login_page.dart
│   ├── home_page.dart
│   ├── accounts_page.dart
│   ├── categories_page.dart
│   ├── transaction_edit_page.dart
│   ├── assets_page.dart
│   └── budgets_page.dart
├── ocr/
│   ├── ocr_service.dart
│   └── bill_rules.dart       # 端侧规则
└── sync/
    └── ws_client.dart
```

### 19.3 网络层 `api/api_client.dart`

```dart
import 'dart:convert';
import 'package:http/http.dart' as http;

class ApiClient {
  String baseUrl;
  String? token;
  ApiClient(this.baseUrl);

  // 统一请求方法
  Future<Map<String, dynamic>> request(
    String method, String path, {Map<String, dynamic>? body}) async {
    final resp = await http.Client().send(
      http.Request(method, Uri.parse('$baseUrl$path'))
        ..headers.addAll({
          'Content-Type': 'application/json',
          if (token != null) 'Authorization': 'Bearer $token',
        })
        ..body = jsonEncode(body ?? {}),
    );
    final data = jsonDecode(await resp.stream.bytesToString());
    if (data['code'] != 0) throw Exception(data['msg']);
    return data['data'];
  }

  Future<Map<String, dynamic>> get(String p) => request('GET', p);
  Future<Map<String, dynamic>> post(String p, Map<String, dynamic> b) =>
      request('POST', p, body: b);
}
```

### 19.4 本地存储（离线可用）

- 用 `sqflite`（本地数据库）缓存账本数据；
- 没网络时把交易先存本地队列，联网后用 `sync/push` 补传，每条带 `client_uuid` 去重。

---

## 20. 登录与各页面

### 20.1 页面清单（先做这些）

| 页面 | 功能 |
|---|---|
| 登录页 | 用户名密码登录，存 token |
| 首页 | 账单列表、本月收支、底部记一笔按钮 |
| 账户页 | 账户列表、余额、新增账户 |
| 分类页 | 两级分类树、增删改 |
| 交易编辑页 | 手动/识别后填表单，选账户、分类、金额 |
| 资产页 | 总资产/负债/净资产、投资持仓 |
| 预算页 | 预算进度、超支状态 |
| 导出页 | 选格式范围，调导出接口下载 |

### 20.2 一个页面示例（登录）

```dart
class LoginPage extends StatefulWidget {
  final ApiClient api;
  const LoginPage(this.api, {super.key});
  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final user = TextEditingController();
  final pass = TextEditingController();
  String? err;

  Future<void> login() async {
    try {
      final data = await widget.api.post('/auth/login',
          {'username': user.text, 'password': pass.text});
      widget.api.token = data['accessToken'];
      if (mounted) {
        Navigator.pushReplacement(context,
            MaterialPageRoute(builder: (_) => HomePage(widget.api)));
      }
    } catch (e) {
      setState(() => err = e.toString());
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('登录')),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(children: [
          TextField(controller: user, decoration: const InputDecoration(labelText: '用户名')),
          TextField(controller: pass, obscureText: true,
              decoration: const InputDecoration(labelText: '密码')),
          if (err != null) Text(err!, style: const TextStyle(color: Colors.red)),
          const SizedBox(height: 16),
          ElevatedButton(onPressed: login, child: const Text('登录')),
        ]),
      ),
    );
  }
}
```

---

## 21. 截图 OCR（Android / Windows / iOS）

### 21.1 整体流程

```
触发截图 → 端侧 OCR 出文字 → 端侧规则解析 → 弹确认框
                                   └─ 没解析出 → 调后端 parse-text（发文字）
```

### 21.2 Android

- 用 `google_mlkit_text_recognition`（支持中文离线模型）：
  ```dart
  final recognizer = TextRecognizer(script: TextRecognitionScript.chinese);
  final result = await recognizer.processImage(InputImage.fromFilePath(imagePath));
  String text = result.text;
  ```
- 悬浮球：用 `flutter_overlay_window` 或一个常驻前台服务，点击后截屏（系统截图 API / 无障碍服务）。
- 注意：安卓后台截屏/悬浮窗涉及较多权限，是客户端难点，建议先做"从相册/分享图片识别"，再做悬浮球。

### 21.3 Windows

- 用系统 OCR（Windows.Media.Ocr，通过 FFI 调用）或先接一个本地 OCR；
- 全局热键：用 `hotkey_manager` 注册（如 Ctrl+Shift+S），触发后用 `screen_capturer` 截屏。

### 21.4 iOS（原生 App + 快捷指令）

- App 负责浏览管理、手动记账；**不做全局截图**（沙盒不允许，上架会被拒）。
- OCR 走快捷指令：
  1. 截图 → 快捷指令"从图片中提取文本"（系统 Vision，端侧）；
  2. 文本 POST 到 `parse-text`，或通过 URL Scheme 回传 App；
  3. App 弹出确认框。
- App 注册 URL Scheme（如 `fastledger://`），并提供 iCloud 快捷指令模板。

### 21.5 端侧规则 `ocr/bill_rules.dart`

```dart
// 用正则/关键字匹配，规则可做成 JSON 便于热更新
class BillRules {
  // 金额：必须支持千位分隔符（1,234.56 / 12,345）与普通金额（1234.56 / 1234 / 12.3）
  // 旧正则 [0-9]+\.?[0-9]* 遇到逗号会把 1,234.56 截断成 1（R15）。
  static final _amountReg = RegExp(
    r'[¥￥]\s*([0-9]{1,3}(?:,[0-9]{3})*(?:\.[0-9]{1,2})?|[0-9]+(?:\.[0-9]{1,2})?)');

  static Map<String, dynamic>? parse(String rawText) {
    // 先规范化：全角数字/全角￥转半角，去掉金额周围空格
    final text = _normalize(rawText);

    if (text.contains('微信支付')) {
      final m = _amountReg.firstMatch(text);
      if (m != null) {
        final cents = _toCents(m.group(1)!);
        if (cents != null) {
          return {'paymentChannel': 1, 'amount': cents}; // 商户、时间继续匹配
        }
      }
    }
    // 支付宝同理；都没命中 → 交给后端 LLM
    return null;
  }

  // 金额字符串 → 整数分；不使用 double，避免精度问题
  static int? _toCents(String s) {
    final normalized = s.replaceAll(',', ''); // 只删"合法千位逗号"
    final parts = normalized.split('.');
    if (parts.length > 2) return null;
    final yuan = int.tryParse(parts[0]);
    if (yuan == null) return null;
    var cents = 0;
    if (parts.length == 2) {
      var frac = parts[1];
      if (frac.length == 1) frac = frac + '0'; // 12.3 → 12.30
      if (frac.length > 2) return null;
      final f = int.tryParse(frac);
      if (f == null) return null;
      cents = f;
    }
    return yuan * 100 + cents;
  }

  static String _normalize(String s) {
    // 全角数字 0-9 → 半角；全角￥、空格按需处理
    final buffer = StringBuffer();
    for (final code in s.runes) {
      if (code >= 0xFF10 && code <= 0xFF19) {
        buffer.writeCharCode(code - 0xFEE0);
      } else {
        buffer.writeCharCode(code);
      }
    }
    return buffer.toString();
  }
}
```

> 测试样例（必须覆盖）：`¥1,234.56`→123456；`¥12,345`→1234500；`¥1234.5`→123450；`¥12.34`→1234；`¥99`→9900；退款 `¥-1,000`、含多个金额时定位"实付金额"。异常分组（如 `1,23`）不强行解析，回退人工，不产生高置信错误金额。

---

## 22. 实时同步

- 用 `web_socket_channel` 连接；**WS 事件只表示"有新版本"，触发一次 pull，不直接推进已同步游标**（R07，避免丢一条后游标被后面的事件跳过）；
- 维护一个离线队列：本地记的账先存库，连上网络后按 `client_uuid` 补传（重复提交不产生第二笔）；
- App 启动和 WS 重连时，调 `sync/pull?afterVersion=本地已完整应用的版本` 拉增量。
- 客户端游标推进规则：**只有把一个 pull 批次里的所有实体（交易/账户/分类/持仓）都完整应用后，才把游标推进到该批次的 `highVersion`**。

### 22.1 版本号怎么来的（一定要在写事务里做）

每个账本在 `book_versions` 表有一个全局版本号，任何增/改/删都在**同一个事务**里：**先调用共享 `VersionSource.Next` 取到 `newVersion`，再写进所有变更实体的 `sync_version`**。

> **唯一实现**：《落地方案》§6.1 的 `VersionRepo.Next`。`TxService`、持仓写入和同步只依赖其 `VersionSource` 接口；旧 `nextBookVersion` 名称退出有效骨架。§3.5 的 SQL 仅解释算法，不是第三个可调用实现。任何写路径不得复制 upsert。

```go
// 写事务内：注入的 versions 由共享 VersionRepo 实现（落地方案 §6.1）
newVersion, err := versions.Next(sess, bookID)
if err != nil { /* 回滚并返回错误 */ }
tx.SyncVersion = newVersion // 写进本次变更实体的 sync_version
```

### 22.2 增量拉取（必须在一致快照中，R07）

```go
// SyncBatch 是一次 pull 的完整结果；客户端全部应用后才推进游标到 HighVersion
type SyncBatch struct {
	HighVersion  int64
	Transactions []models.Transaction
	Accounts     []models.Account
	Categories   []models.Category
	Holdings     []models.Holding
}

// Pull 在同一个只读事务（一致快照）内确定水位并读取所有实体，
// 杜绝"先读交易→别人提交 V6→再读账户看到 V6、却漏了交易 V6"的跨表交错。
func Pull(bookID, afterVersion int64) (*SyncBatch, error) {
	sess := database.Engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}
	defer sess.Rollback() // 只读事务，提交/回滚均可释放快照

	// 1) 在快照内确定本次水位
	var bv models.BookVersion
	if _, err := sess.Where("book_id=?", bookID).Get(&bv); err != nil {
		return nil, err
	}

	// 2) 同一快照内读取所有实体（同一版本的多表/多行一定一起可见）
	batch := &SyncBatch{HighVersion: bv.Version}
	if err := sess.Where("book_id=? AND sync_version>?", bookID, afterVersion).
		OrderBy("sync_version ASC").Find(&batch.Transactions); err != nil {
		return nil, err
	}
	sess.Where("book_id=? AND sync_version>?", bookID, afterVersion).Find(&batch.Accounts)
	sess.Where("book_id=? AND sync_version>?", bookID, afterVersion).Find(&batch.Categories)
	sess.Where("book_id=? AND sync_version>?", bookID, afterVersion).Find(&batch.Holdings)
	return batch, nil
}
```

> 分页注意：如按版本分页，**不能把同一 `sync_version` 的一组变更拆到两页**（同一事务可能同时改交易和账户）。应以版本为边界分页；WS 发送缓冲溢出时必须要求客户端全量重同步，不能静默丢弃后继续推进。

### 22.3 冲突处理

端侧 push 时若该实体在服务端的 `sync_version` 已比端侧基准大（被别人改过），返回 409，让用户选"覆盖 / 放弃 / 另存副本"。

### 22.4 数据库连接务必开 WAL（否则并发读写容易报 SQLITE_BUSY）

```go
func initDB() *xorm.Engine {
	// 用 DSN 参数让 PRAGMA 对连接池中的【每一个】连接生效（R16）。
	// 不能用 db.Exec("PRAGMA ...")：那只作用于当时拿到的某一个连接，
	// 池里其他连接仍是默认值（实测 foreign_keys 会出现 [1,0] 混用）。
	dsn := "file:./data/fastledger.db?" +
		"_journal_mode=WAL&" +       // 读写不互锁
		"_busy_timeout=5000&" +      // 遇锁等 5s 自动重试
		"_foreign_keys=on&" +        // 外键约束每连接生效
		"_synchronous=NORMAL"        // WAL 下安全且更快
	engine, err := xorm.NewEngine("sqlite3", dsn)
	if err != nil {
		log.Fatal(err)
	}

	// SQLite 同时只允许一个写连接。MVP 最稳妥：全局连接数限制为 1（完全串行，
	// 家庭负载极小，无锁竞争）。如要发挥 WAL 的多读能力，可另建一个只读引擎
	// （SetMaxOpenConns 放宽），写引擎保持 1，由代码按读写路由。
	db := engine.DB()
	db.SetMaxOpenConns(1)

	// 启动后校验一次，确认 PRAGMA 真的生效（而不是"执行了没报错"）
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		log.Fatalf("foreign_keys 未生效（fk=%d, err=%v）", fk, err)
	}
	return engine
}
```

> 小结：PRAGMA 要么走 DSN（每连接生效），要么用连接建立钩子（ConnectHook）逐个设置并检查返回值；对连接池整体 `db.Exec` 是不可靠的。

### 22.5 第一个管理员怎么来（不靠公开注册）

首次启动且 `users` 表为空时，用命令行创建（或首启向导）：

```bash
./fastledger user create --username admin --owner
```

建好后该入口失效；`ENABLE_REGISTER=false` 不影响它。

---
---

# 第五部分：Web 端

## 23. Web 端（Go 托管，无 OCR）

### 23.1 定位

浏览器打开就能**快捷浏览、检索、批量编辑、对账**，不做截图 OCR（OCR 交给客户端）。

### 23.2 打包进 Go 二进制（不增加部署单元）

```go
import "embed"

//go:embed web/dist/*
var webFS embed.FS

func registerWeb(r *gin.Engine) {
	sub, _ := fs.Sub(webFS, "web/dist")
	r.NoRoute(gin.WrapH(http.FileServer(http.FS(sub))))
}
```

### 23.3 实现

- 前端是普通网页（列表、筛选、表单、表格批量编辑），调用同一套 `/api/v1` 接口；
- 构建后把产物放进 `web/dist`，Go 编译时用 `go:embed` 一起打进单个二进制。

---
---

# 第六部分：部署与上线

## 24. Docker 部署

### 24.1 后端多阶段构建 `Dockerfile`

```dockerfile
# 构建阶段
FROM golang:1.22 AS build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=1 go build -o fastledger ./cmd/server

# 运行阶段
FROM debian:stable-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /app/fastledger .
EXPOSE 8080
VOLUME ["/app/data"]
CMD ["./fastledger"]
```

### 24.2 docker-compose.yml

```yaml
services:
  fastledger:
    build: .
    container_name: fastledger
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data        # 数据库和附件
```

启动：`docker compose up -d`

---

## 25. Caddy 反代与外网访问

### 25.1 Caddyfile（自动申请 HTTPS 证书）

```
ledger.你的域名.com {
    reverse_proxy fastledger:8080
}
```

### 25.2 外网访问步骤

1. 有一个域名（DDNS 域名也行），在 DNS 把它指向家里宽带的公网 IP；
2. 路由器做端口转发：443 → NAS；
3. 跑 Caddy 容器，它会自动向 Let's Encrypt 申请并续期证书；
4. 没有公网 IP 时，可用支持 HTTPS 的内网穿透方案（如自建 frp/tailscale funnel）。

> 安全：务必用 HTTPS；长期 API Token 可单独吊销；后续加 2FA、应用锁。

---

## 26. 备份与运维

- **备份什么**：`data/` 目录（SQLite 文件 + 附件）。
- **怎么备份**：NAS 定时任务每天打包一次，复制到另一块盘/对象存储；SQLite 用 `sqlite3 .backup` 做一致性备份。
- **升级**：拉新镜像重启；数据库迁移用 xorm 自动加列，重大变更先备份。
- **监控**：看容器日志（`docker logs -f fastledger`）、内存占用（目标常驻 <150MB）。

### 26.1 恢复时必须有"恢复世代"（restore epoch，R10）

问题：从一个旧备份恢复后，数据库的版本号也回到了旧值。此时仍缓存着"恢复后状态"的客户端，可能继续按旧游标 push/pull，把数据覆盖回去或产生重复，且没有任何机制能让它们察觉"服务端整体回退了"。

设计：用一个**独立于数据库、不随备份恢复**的元信息文件 `data/instance.meta`：

```json
{ "instance_id": "随机固定值", "restore_epoch": 1, "restored_at": "" }
```

- 备份包**不包含**该文件（或恢复时强制递增）。每次执行 restore：`restore_epoch += 1`，更新 `restored_at`。
- 登录/握手响应与 `/api/v1/health` 都返回当前 `instance_id` 与 `restore_epoch`。
- 客户端在本地记录上次见到的 epoch；一旦发现不一致：
  1. **立即停止**基于旧游标的增量 push/pull（不再信任本地版本与数据）；
  2. 提示用户"服务端已从备份恢复，需要重新同步"，执行一次**全量拉取**重建本地库；
  3. 把本地离线队列里尚未确认提交的条目列出来，让用户逐条确认后再重放（避免重复或丢失），不是静默丢弃。

这样"恢复"对所有客户端是一个可被检测、可审阅、需确认的全局事件，而不是靠版本号碰巧工作。

---
---

# 第七部分：开发计划

## 27. 分阶段任务清单（照着排期做）

> 按业余时间每周约 10~15 小时、1 人全栈估算，**v0.1 约 15~19 周**。后端和客户端可交叉推进，先把两个难点（识别、实时同步）做技术验证。

### 后端任务
| # | 任务 | 验证 |
|---|---|---|
| 1 | 项目骨架、配置、日志 | 能启动 |
| 2 | 数据库连接、自动建表 | 表能建成 |
| 3 | 注册/登录、JWT、中间件 | 能拿 token |
| 4 | 账本 CRUD、成员、邀请 | 能建账加人 |
| 5 | 账户/分类 CRUD + 默认数据 | 初始化默认数据 |
| 6 | 交易 CRUD + 事务 + 余额 + 转账 | 余额正确、转账成对 |
| 7 | WebSocket Hub + 广播 | 两端消息互通 |
| 8 | 识别接口（LLM 文本兜底） | 文本→结构化 |
| 9 | 增量同步 pull/push + 幂等 | 离线队列可补 |
| 9.1 | 基础报表（月度收支/分类占比）+ CSV/JSON 导出 | 报表与明细一致、导出可打开 |
| 9.5 | 投资模块（标的/持仓/资产合计） | 市值/净资产正确 |

### 客户端任务
| # | 任务 | 验证 |
|---|---|---|
| 10 | Flutter 工程骨架、网络、本地库 | 能登录、能缓存 |
| 11 | 账户/分类/交易页面 | 能增删改查 |
| 11.5 | 投资模块页面（资产/持仓/标的） | 手动录入、合计正确 |
| 12 | Windows 热键 + 截图 + OCR | 快捷键出确认框 |
| 13 | Android 悬浮球 + 截图 + ML Kit | 悬浮球出确认框 |
| 14 | 端侧规则引擎（微信/支付宝） | 离线即可识别 |
| 15 | WS 接收 + 离线队列 + 重连 | 实时刷新、断网可补 |
| 16 | Web 端页面（浏览/编辑/对账） | 浏览器可用 |
| 17 | iOS 客户端 + 快捷指令 OCR | App 浏览、快捷指令回传 |

### 版本路线
| 版本 | 内容 |
|---|---|
| **v0.1 MVP** | 记账闭环、三端（含 iOS）、Web、两级分类、**基础报表（月度收支/分类占比）、CSV/JSON 导出**、投资骨架手动录入 |
| v0.2 | 高级图表/自定义报表、标签、图片、**分类预算** |
| v0.3 | **投资自动行情**（A股/港股/基金/黄金）、买卖分红、持仓自动维护 |
| v0.4 | **家庭资产合计**、多币种汇率、实物资产、Mac 客户端 |
| v0.5 | CSV 批量导入、2FA、OIDC、应用锁、加密 |

---

## 28. 验收标准（核心场景，Given-When-Then）

1. **截图记账**：给定已登录，当截图并触发识别，则弹出预填金额/商户的确认框，确认后交易入库、余额更新。
2. **家人共享**：给定成员都在线，当一方记一笔，则另一方在 1~2 秒内收到并看到该笔。
3. **不重复**：给定微信绑信用卡支付，当确认入账，则只在信用卡账户产生支出，微信仅作为渠道。
4. **还款**：给定信用卡有欠款，当用储蓄卡还款，则记为转账，支出总额不增加。
5. **离线补传**：给定无网络记了账，当恢复网络，则交易自动补传且不重复（幂等）。
6. **投资合计**：给定录入持仓和资产，当查看资产页，则总资产/净资产计算正确。
7. **预算预警**：给定某分类预算，当支出达到 80%/超支，则状态变为接近/超支。
8. **导出**：给定筛选条件，当导出 CSV，则文件能被 Excel 正确打开、数据与明细一致。

---

## 给新手的最后建议

1. **先跑通最小闭环**（登录→手动记一笔→列表能看到），再做 OCR 和同步，别一上来啃最难的。
2. **每完成一小块就提交 git、就验证**，别攒一大堆再找 bug。
3. **善用参考项目**：ezBookkeeping（Go+Vue）的导入转换器、LLM 抽象、金额处理都值得直接学习；BeeCount 的移动端 OCR 和家庭共享思路可参考。
4. 遇到报错**先读报错信息**，再搜索，后端常见问题 90% 能在错误提示里找到原因。

祝你顺利做出属于自己的家庭记账系统！




