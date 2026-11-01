-- 0008：长期 API Token 表（auth_tokens，WBS 6.6 / PRD §187 §296 §370）
--
-- 背景：PRD §187 要求"支持长期 API Token（用于客户端/快捷指令）"、§296 要求
-- "长期 API Token 管理（生成、查看、撤销）"。JWT + token_sessions（0007）解决的是
-- "登录会话"——随登录签发、可按设备撤销；但快捷指令/脚本场景需要一个**不随会话
-- 过期**的长期凭证，且泄露时可单独撤销而不影响登录会话。本表即该凭证的事实来源。
--
-- 列语义：
--   id           自增主键（撤销接口的路由参数）；
--   uid          归属用户；管理接口全部先按 uid 过滤，跨用户操作 404 不泄露存在性；
--                （PRD §370 写作 user_id，此处沿用 0007 token_sessions 的 uid 命名
--                以保持表结构一致性——口径偏差已在 CHANGELOG 登记）
--   token_hash   令牌的 SHA-256 hex（64 字符）。**不存明文**：PRD §370 的 token 列
--                落地为哈希——明文仅在创建响应中出现一次（同 GitHub PAT 语义），
--                库泄露不等于凭证泄露（口径偏差已在 CHANGELOG 登记）；
--   name         用户自报用途备注（如"快捷指令""CI 脚本"），仅展示；
--   created_at   创建时刻（Unix 秒）；
--   expires_at   过期时刻（Unix 秒）；0 = 永不过期（"长期"的默认语义）；
--   revoked_at   撤销时刻（Unix 秒），0 = 未撤销；撤销是单向写，不可恢复；
--   last_used_at 最近一次通过该 Token 鉴权的时刻（Unix 秒），0 = 从未使用。
--
-- 鉴权口径：中间件按明文前缀分流——`flt_` 开头走本表哈希查找（撤销/过期/用户
-- 禁用即 401），`eyJ` 开头（JWT）走 VerifyWithClaims 会话强校验。两条路径互斥，
-- PAT 不受 token_sessions 约束（它不是会话），撤销语义完全由本表承担。
-- PRD §10.528"撤销的令牌不得通过 WS 握手"同样成立：撤销后哈希查不到有效行。

CREATE TABLE IF NOT EXISTS auth_tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid INTEGER NOT NULL,
  token_hash VARCHAR(64) NOT NULL UNIQUE,
  name VARCHAR(64) NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL DEFAULT 0,
  revoked_at INTEGER NOT NULL DEFAULT 0,
  last_used_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_uid
  ON auth_tokens(uid, revoked_at);
