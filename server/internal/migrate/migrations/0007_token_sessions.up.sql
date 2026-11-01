-- 0007：多端会话表（token_sessions，WBS 5.10 / GPT-R52 / PRD §4.1 §10.10）
--
-- 背景：此前"多端同时在线"只能靠 JWT 的 exp 自然到期，无法单设备撤销——
-- 令牌泄漏只能等它过期，或"禁用整个用户"误伤所有设备。本表把每个签发过的
-- 令牌（按 jti）持久化，使"查看会话列表 / 撤销指定设备"成为可能。
--
-- 列语义：
--   jti          JWT 唯一标识（SignJWT 生成的 32 位 hex），全表唯一；
--   uid          会话归属用户；撤销接口据此拒绝跨用户操作（404，不泄露存在性）；
--   created_at   签发时刻（Unix 秒）；
--   expires_at   令牌 exp（Unix 秒）；Verify 复核它，表值 <= now 即拒绝；
--   revoked_at   撤销时刻（Unix 秒），0 = 未撤销；撤销是单向写（0 -> 撤销时刻）；
--   user_agent   登录时客户端自报的 User-Agent（仅展示用，截断到 255）；
--   last_seen_at 最近活跃时刻（Verify 命中时按 1 小时粒度回写，0 = 从未活跃）。
--
-- 一致性口径：签发令牌与写会话行【同事务】——会话落库失败就不签发令牌，
-- 绝不产生"有令牌无会话"的中间态（该状态在 Verify 强校验下等于必然 401）。
--
-- 升级说明：0007 上线后，存量令牌（0006 及之前签发）没有会话行，Verify 一律
-- 拒绝（401）——这是预期行为，用户重新登录即可。刻意【不】做自动补登记：
-- 无法补出可信的 exp / user_agent，且补登记会给"撤销"语义开豁免通道。

CREATE TABLE IF NOT EXISTS token_sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  jti VARCHAR(64) NOT NULL UNIQUE,
  uid INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  revoked_at INTEGER NOT NULL DEFAULT 0,
  user_agent VARCHAR(255) NOT NULL DEFAULT '',
  last_seen_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_token_sessions_uid
  ON token_sessions(uid, revoked_at);
CREATE INDEX IF NOT EXISTS idx_token_sessions_exp
  ON token_sessions(expires_at);
