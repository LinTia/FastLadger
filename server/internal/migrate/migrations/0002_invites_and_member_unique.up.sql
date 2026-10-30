-- 0002：修正成员唯一索引 + 新增邀请码表
-- 说明：0001 的 UNIQUE(book_id, uid) 与"成员退出为软删"冲突——成员被移除后
--       该唯一索引仍占位，导致无法重新加入。改为【部分唯一索引】只约束未删除行。

DROP INDEX IF EXISTS idx_member_unique;
CREATE UNIQUE INDEX IF NOT EXISTS idx_member_unique
  ON book_members(book_id, uid) WHERE deleted = 0;

-- 邀请码：关闭公开注册后的唯一入户通道（PRD §12.1 决策）
-- 单次使用（used_at/used_by）、有效期默认 7 天、可绑定角色/邮箱/用户名
CREATE TABLE IF NOT EXISTS invites (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL,
  code VARCHAR(64) NOT NULL UNIQUE,
  role TINYINT NOT NULL DEFAULT 3,          -- 1=owner 2=admin 3=member
  created_by INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  used_at INTEGER DEFAULT 0,
  used_by INTEGER DEFAULT 0,
  bound_email VARCHAR(128) DEFAULT '',
  bound_username VARCHAR(64) DEFAULT '',
  revoked BOOLEAN DEFAULT 0,
  created_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_invite_book ON invites(book_id, revoked);
