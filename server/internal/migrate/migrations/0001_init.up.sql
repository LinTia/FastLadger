-- FastLedger 初始 schema（阶段 0）
-- 约定：金额一律整数分；软删统一 deleted / deleted_at；同步统一 sync_version；
--       枚举值以《FastLedger-枚举字典》为唯一权威。

CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username VARCHAR(64) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,   -- bcrypt/argon2 自带盐，不另设 salt 列
  email VARCHAR(128) DEFAULT '',         -- 自托管常无 SMTP，邮箱可选
  is_instance_admin BOOLEAN DEFAULT 0,  -- 实例管理员，与账本 owner 严格分离（PRD §12.1）
  disabled BOOLEAN DEFAULT 0,
  deleted BOOLEAN DEFAULT 0,
  created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email <> '';

CREATE TABLE IF NOT EXISTS books (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name VARCHAR(64) NOT NULL,
  owner_uid INTEGER NOT NULL,
  book_type TINYINT DEFAULT 1,              -- 1=个人 2=家庭（预留）
  default_currency VARCHAR(8) DEFAULT 'CNY', -- MVP 仅 CNY
  members_can_view_others BOOLEAN DEFAULT 1, -- 账本级可见性开关
  deleted BOOLEAN DEFAULT 0,
  created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_books_owner ON books(owner_uid, deleted);

CREATE TABLE IF NOT EXISTS book_members (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL,
  uid INTEGER NOT NULL,
  role TINYINT NOT NULL,                    -- 1=owner 2=admin 3=member
  can_view_others BOOLEAN DEFAULT 1,        -- 成员级覆盖，MVP 不启用
  deleted BOOLEAN DEFAULT 0,                -- 成员退出为软删，保留历史记录
  created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_member_unique ON book_members(book_id, uid);
CREATE INDEX IF NOT EXISTS idx_member_uid ON book_members(uid, deleted);

-- 账本级全局版本号：写事务内原子 +1 并回写实体 sync_version（v1.1 定）
CREATE TABLE IF NOT EXISTS book_versions (
  book_id INTEGER PRIMARY KEY,
  version INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS accounts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL,
  name VARCHAR(64) NOT NULL,
  category TINYINT NOT NULL,                -- 资金账户/信用等，见枚举字典
  currency VARCHAR(8) DEFAULT 'CNY',
  balance BIGINT NOT NULL DEFAULT 0,        -- 整数分；负债账户正=欠款
  parent_id INTEGER DEFAULT 0,              -- 仅 UI 分组，不参与余额计算
  sync_version INTEGER NOT NULL DEFAULT 0,
  deleted BOOLEAN DEFAULT 0, created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_accounts_book ON accounts(book_id, deleted);

CREATE TABLE IF NOT EXISTS categories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL,
  name VARCHAR(64) NOT NULL,
  parent_id INTEGER DEFAULT 0,              -- 两级分类（大类/小类）
  cat_type TINYINT NOT NULL,                -- 1=收入 2=支出
  sync_version INTEGER NOT NULL DEFAULT 0,
  deleted BOOLEAN DEFAULT 0, created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_categories_book ON categories(book_id, deleted);

CREATE TABLE IF NOT EXISTS transactions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL,
  type TINYINT NOT NULL,                    -- 见枚举字典 TxType
  account_id INTEGER NOT NULL,
  to_account_id INTEGER DEFAULT 0,          -- 转账对手账户
  amount BIGINT NOT NULL,                   -- 整数分
  category_id INTEGER DEFAULT 0,
  payment_channel TINYINT DEFAULT 0,
  merchant VARCHAR(128) DEFAULT '',
  note VARCHAR(255) DEFAULT '',
  tx_time INTEGER NOT NULL,
  related_id INTEGER DEFAULT 0,             -- 转账配对：整笔删除/恢复的分组键
  client_uuid VARCHAR(64) DEFAULT '',       -- 幂等键
  sync_version INTEGER NOT NULL DEFAULT 0,
  deleted BOOLEAN DEFAULT 0, created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_tx_book_time ON transactions(book_id, tx_time);
CREATE INDEX IF NOT EXISTS idx_tx_sync ON transactions(book_id, sync_version);
CREATE INDEX IF NOT EXISTS idx_tx_related ON transactions(book_id, related_id);

-- 幂等回执：老库升级【不得删除旧回执】，否则离线重试会产生重复账
CREATE TABLE IF NOT EXISTS idempotency_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL,
  uid INTEGER NOT NULL,
  client_uuid VARCHAR(64) NOT NULL,
  request_hash VARCHAR(128) NOT NULL,
  result_ids TEXT DEFAULT '',
  result_type TINYINT DEFAULT 0,
  result_version INTEGER DEFAULT 0,
  created_at INTEGER
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_idem_unique ON idempotency_records(book_id, uid, client_uuid);

-- 审计日志（append-only；PRD §12.1 决策，保留 1 年；仓库层只暴露 Insert）
CREATE TABLE IF NOT EXISTS audit_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL,
  actor_uid INTEGER NOT NULL,
  action VARCHAR(32) NOT NULL,
  entity_type VARCHAR(32) NOT NULL,
  entity_id INTEGER NOT NULL DEFAULT 0,
  before_json TEXT,                         -- 变更前状态：软删回答不了"删前是什么"
  at INTEGER NOT NULL,
  source VARCHAR(16)
);
CREATE INDEX IF NOT EXISTS idx_audit_book_at ON audit_logs(book_id, at);
CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_logs(entity_type, entity_id);

CREATE TABLE IF NOT EXISTS instruments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL,
  name VARCHAR(64) NOT NULL,
  type TINYINT NOT NULL,
  market VARCHAR(16) DEFAULT '',
  code VARCHAR(32) DEFAULT '',
  sync_version INTEGER NOT NULL DEFAULT 0,
  deleted BOOLEAN DEFAULT 0, created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_instr_book ON instruments(book_id, deleted);

CREATE TABLE IF NOT EXISTS instrument_prices (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  instrument_id INTEGER NOT NULL,
  price VARCHAR(32) NOT NULL,               -- 字符串存储，运算走 decimal，禁 float
  price_time INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_instr_price ON instrument_prices(instrument_id, price_time);

CREATE TABLE IF NOT EXISTS holdings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL,
  account_id INTEGER NOT NULL,
  instrument_id INTEGER NOT NULL,
  quantity VARCHAR(32) NOT NULL,            -- 字符串 + decimal
  cost_amount BIGINT NOT NULL DEFAULT 0,
  current_price VARCHAR(32) DEFAULT '0',
  sync_version INTEGER NOT NULL DEFAULT 0,
  deleted BOOLEAN DEFAULT 0, created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_holdings_account ON holdings(book_id, account_id, deleted);

CREATE TABLE IF NOT EXISTS budgets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL,
  name VARCHAR(64) NOT NULL,
  budget_type TINYINT NOT NULL,             -- 1=分类预算 2=总预算
  category_id INTEGER DEFAULT 0,
  period_type TINYINT DEFAULT 1,            -- MVP 固定自然月
  amount BIGINT NOT NULL,                   -- 预算金额（分）
  rollover BOOLEAN DEFAULT 0,
  enabled BOOLEAN DEFAULT 1,
  deleted BOOLEAN DEFAULT 0, created_at INTEGER, updated_at INTEGER, deleted_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_budget_book ON budgets(book_id, deleted);

CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  applied_at INTEGER NOT NULL
);
