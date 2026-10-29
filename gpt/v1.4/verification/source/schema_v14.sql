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

-- 幂等操作回执（R04：替代"先查后插"，消除竞态；单条交易与转账统一处理）
CREATE TABLE idempotency_records (
  id           INTEGER PRIMARY KEY,
  book_id      INTEGER NOT NULL,
  uid          INTEGER NOT NULL,
  client_uuid  VARCHAR(64) NOT NULL,
  request_hash VARCHAR(64) DEFAULT '',   -- 请求摘要：同一 client_uuid 但内容不同 → 检测为冲突
  result_type  TINYINT DEFAULT 0,        -- 1=单条交易 2=转账
  result_ids   VARCHAR(128) DEFAULT '',  -- 结果实体 id（转账含两条，逗号分隔）
  created_at   INTEGER,
  UNIQUE(book_id, uid, client_uuid)      -- 作用域精确到"账本+用户+客户端UUID"
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
