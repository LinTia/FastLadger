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
  password    VARCHAR(128) NOT NULL,        -- 存 bcrypt 哈希，不存明文
  nickname    VARCHAR(64) DEFAULT '',
  email       VARCHAR(128) DEFAULT '',
  deleted     BOOLEAN DEFAULT 0,
  created_at  INTEGER,
  updated_at  INTEGER,
  deleted_at  INTEGER
);

-- 账本
CREATE TABLE books (
  id              INTEGER PRIMARY KEY,
  name            VARCHAR(64) NOT NULL,
  owner_uid       INTEGER NOT NULL,
  default_currency VARCHAR(3) DEFAULT 'CNY',
  members_visible BOOLEAN DEFAULT 1,        -- 成员是否默认互看
  deleted         BOOLEAN DEFAULT 0,
  created_at      INTEGER, updated_at INTEGER, deleted_at INTEGER
);

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
  category    TINYINT NOT NULL,             -- 见 6.2 账户分类
  name        VARCHAR(64) NOT NULL,
  currency    VARCHAR(3) DEFAULT 'CNY',
  balance     BIGINT DEFAULT 0,             -- 余额（分）；投资账户=现金部分，实物资产=估值
  icon        VARCHAR(64) DEFAULT '',
  sort_order  INTEGER DEFAULT 0,
  hidden      BOOLEAN DEFAULT 0,
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
  version           INTEGER DEFAULT 1,       -- 同步版本
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

### 7.4 WebSocket 协议

- 连接地址：`wss://你的域名/api/v1/ws?token=<token>`
- 客户端连上后发送订阅：`{ "action": "subscribe", "books": [1, 2] }`
- 服务器在有新交易/改动时推送：
  ```json
  { "event": "transaction_created", "bookId": 1, "data": { "id": 88, "amount": 3650 } }
  ```
- 断线自动重连，重连后用 `sync/pull` 补漏掉的消息。

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
	Id        int64     `xorm:"pk autoincr"`
	Username  string    `xorm:"varchar(64) notnull unique"`
	Password  string    `xorm:"varchar(128) notnull"` // bcrypt 哈希
	Nickname  string    `xorm:"varchar(64)"`
	Deleted   bool      `xorm:"default 0"`
	CreatedAt time.Time `xorm:"created"`
	UpdatedAt time.Time `xorm:"updated"`
	DeletedAt *time.Time `xorm:"null"`
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
	"time"

	"fastledger/internal/database"
	"fastledger/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = []byte("改成你自己的密钥")

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
	user := &models.User{Username: username, Password: string(hash)}
	_, err = database.Engine.Insert(user)
	return user, err
}

// Login 登录，成功返回 token
func Login(username, password string) (string, error) {
	user := &models.User{}
	has, _ := database.Engine.Where("username=?", username).Get(user)
	if !has {
		return "", errors.New("用户名或密码错误")
	}
	// 校验密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.New("用户名或密码错误")
	}
	// 生成 JWT
	claims := jwt.MapClaims{
		"uid":      user.Id,
		"username": user.Username,
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

func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "登录已失效"})
			return
		}
		// 把 uid 放进上下文，后续 handler 用
		c.Set("uid", int64(claims["uid"].(float64)))
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

### 13.1 新增支出/收入 `internal/services/transaction.go`

```go
func AddTransaction(uid int64, tx *models.Transaction) error {
	// 1. 幂等：用 client_uuid 防止重复提交
	if tx.ClientUuid != "" {
		exist, _ := database.Engine.Where("client_uuid=?", tx.ClientUuid).Get(&models.Transaction{})
		if exist {
			return nil // 已记过，直接返回
		}
	}
	tx.CreatorUid = uid

	sess := database.Engine.NewSession()
	defer sess.Close()
	// 开启事务
	if err := sess.Begin(); err != nil {
		return err
	}

	// 2. 插入交易
	if _, err := sess.Insert(tx); err != nil {
		sess.Rollback()
		return err
	}

	// 3. 更新账户余额（支出减、收入加）
	account := &models.Account{}
	if _, err := sess.ID(tx.AccountId).Get(account); err != nil {
		sess.Rollback()
		return err
	}
	if tx.TxType == 2 { // 支出
		account.Balance -= tx.Amount
	} else if tx.TxType == 1 { // 收入
		account.Balance += tx.Amount
	}
	if _, err := sess.ID(account.Id).Cols("balance").Update(account); err != nil {
		sess.Rollback()
		return err
	}

	return sess.Commit()
}
```

### 13.2 转账（成对、对敲）

```go
func Transfer(uid int64, fromId, toId int64, amount int64) error {
	sess := database.Engine.NewSession()
	defer sess.Close()
	sess.Begin()

	// 转出账户减钱
	from := &models.Account{}
	sess.ID(fromId).Get(from)
	from.Balance -= amount
	sess.ID(fromId).Cols("balance").Update(from)

	// 转入账户加钱
	to := &models.Account{}
	sess.ID(toId).Get(to)
	to.Balance += amount
	sess.ID(toId).Cols("balance").Update(to)

	// 两条交易记录（related_id 互相关联），类型=转账
	out := &models.Transaction{TxType: 3, AccountId: fromId, RelatedAccountId: toId, Amount: amount, CreatorUid: uid}
	in := &models.Transaction{TxType: 3, AccountId: toId, RelatedAccountId: fromId, Amount: amount, CreatorUid: uid}
	sess.Insert(out)
	in.RelatedId = out.Id
	sess.Insert(in)
	out.RelatedId = in.Id
	sess.ID(out.Id).Update(out)

	return sess.Commit()
}
```

> 信用卡还款就走这个转账（储蓄卡→信用卡），不产生支出。

---

## 14. WebSocket 实时推送

### 14.1 连接管理 `internal/ws/hub.go`

```go
package ws

import "sync"

// Client 一个客户端连接（简化：用一个 channel 表示要发的消息）
type Client struct {
	BookIds map[int64]bool
	Send    chan []byte
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

// BroadcastToBook 给订阅了某账本的所有客户端推消息
func (h *Hub) BroadcastToBook(bookId int64, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.BookIds[bookId] {
			select {
			case c.Send <- msg:
			default: // 客户端缓冲满，跳过
			}
		}
	}
}
```

### 14.2 在交易成功后推送

```go
// AddTransaction 提交成功后
msg, _ := json.Marshal(map[string]any{
	"event": "transaction_created", "bookId": tx.BookId, "data": tx,
})
ws.DefaultHub.BroadcastToBook(tx.BookId, msg)
```

### 14.3 WS handler（接收连接、读写循环）

```go
func ServeWS(c *gin.Context) {
	// 升级为 WebSocket（用 gorilla/websocket）
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &Client{BookIds: make(map[int64]bool), Send: make(chan []byte, 256)}
	ws.DefaultHub.Register(client)
	// 读循环：处理 subscribe
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				ws.DefaultHub.Unregister(client)
				return
			}
			var sub struct {
				Action string  `json:"action"`
				Books  []int64 `json:"books"`
			}
			if json.Unmarshal(msg, &sub) == nil && sub.Action == "subscribe" {
				for _, b := range sub.Books {
					client.BookIds[b] = true
				}
			}
		}
	}()
	// 写循环：把 Send 里的消息发出去
	for msg := range client.Send {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}
```

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
func AddHolding(accountId int64, instrumentId int64, quantity string, costAmount int64, currentPrice string) (*models.Holding, error) {
	qty, _ := decimal.NewFromString(quantity)
	price, _ := decimal.NewFromString(currentPrice)
	marketValue := qty.Mul(price)                 // 市值 = 数量 × 最新价
	mvInt := marketValue.Mul(decimal.NewFromInt(100)).IntPart() // 换算成分（按价格单位调整）

	h := &models.Holding{
		AccountId: accountId, InstrumentId: instrumentId,
		Quantity: quantity, CostAmount: costAmount,
		CurrentPrice: currentPrice, MarketValue: mvInt,
	}
	_, err := database.Engine.Insert(h)
	return h, err
}
```

> 数量/价格用字符串或高精度库（如 `shopspring/decimal`），避免股票、基金份额的浮点误差。

### 16.2 家庭资产合计

```go
func AssetSummary(bookId int64) map[string]any {
	// 按账户分类汇总
	accounts := []models.Account{}
	database.Engine.Where("book_id=? AND deleted=0", bookId).Find(&accounts)

	totalAssets, totalLiab := int64(0), int64(0)
	assetByType := map[int64]int64{}
	liabByType := map[int64]int64{}

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
			fmt.Sprintf("%.2f", float64(t.Amount)/100),
			categoryName(t.CategoryId),
			accountName(t.AccountId),
			channelName(t.PaymentChannel),
			t.Comment,
		})
	}
	w.Flush()

	// 3. 作为文件下载
	c.Header("Content-Disposition", "attachment; filename=transactions.csv")
	c.Data(200, "text/csv", buf.Bytes())
}
```

### 17.2 JSON 导出

直接把查询结果 `c.JSON(200, txs)`，加 `Content-Disposition: attachment`。

---

## 18. 预算

### 18.1 预算进度（实时算，不存已用）

```go
func BudgetProgress(bookId int64, periodStart, periodEnd int64) []map[string]any {
	budgets := []models.Budget{}
	database.Engine.Where("book_id=? AND deleted=0", bookId).Find(&budgets)

	result := []map[string]any{}
	for _, b := range budgets {
		// 统计该周期内该分类的支出
		used := int64(0)
		database.Engine.SQL(
			`SELECT COALESCE(SUM(amount),0) FROM transactions
			 WHERE book_id=? AND category_id=? AND tx_type=2
			   AND tx_time BETWEEN ? AND ? AND deleted=0`,
			bookId, b.CategoryId, periodStart, periodEnd).Scan(&used)

		progress := 0.0
		if b.Amount > 0 {
			progress = float64(used) / float64(b.Amount)
		}
		status := "ok"
		switch {
		case used > b.Amount:
			status = "over"       // 超支
		case progress >= 0.8:
			status = "near"       // 接近
		}
		result = append(result, map[string]any{
			"budget": b, "used": used, "progress": progress, "status": status,
		})
	}
	return result
}
```

> 总预算（budget_type=2）统计时不按分类，统计该周期全部支出。

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
  static Map<String, dynamic>? parse(String text) {
    // 微信支付
    if (text.contains('微信支付')) {
      final amount = RegExp(r'[¥￥]\s*([0-9]+\.?[0-9]*)').firstMatch(text);
      if (amount != null) {
        return {
          'paymentChannel': 1,
          'amount': (double.parse(amount.group(1)!) * 100).round(),
          // 商户、时间继续匹配...
        };
      }
    }
    // 支付宝同理
    return null; // 没命中 → 交给后端
  }
}
```

---

## 22. 实时同步

- 用 `web_socket_channel` 连接，收到 `transaction_created` 就刷新列表/首页；
- 维护一个离线队列：本地记的账先存库，连上网络后按 `client_uuid` 补传；
- App 启动和 WS 重连时，调 `sync/pull?afterVersion=本地最新版本` 拉增量。

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
| **v0.1 MVP** | 记账闭环、三端（含 iOS）、Web、两级分类、投资骨架手动录入 |
| v0.2 | 图表、月报、标签、图片、**CSV/JSON 导出**、**分类预算** |
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




