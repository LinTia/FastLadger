-- 0003：为 transactions 补「审计归属」与「命令上下文」列
--
-- 背景：0001 建的 transactions 缺少 creator_uid / source / timezone_offset，
-- 而 PRD §7.3 可见性过滤、阶段 4「member 仅能改删本人交易」与审计 before_json
-- 都依赖"这笔交易是谁记的"。若不补列，补列之前之后写入的每笔交易都会永久失去归属。
--
-- 约定：
--   1. 不修改已发布的 0001/0002，只做 ADD COLUMN；
--   2. SQLite 的 ADD COLUMN 一条语句只能加一列，且 NOT NULL 必须带 DEFAULT；
--   3. 历史行为 0（未知归属），服务端不会据此放宽任何权限判定；
--      MVP 尚未上线，可由实例管理员按审计需要回填。
ALTER TABLE transactions ADD COLUMN creator_uid INTEGER NOT NULL DEFAULT 0;
ALTER TABLE transactions ADD COLUMN source TINYINT NOT NULL DEFAULT 0;
ALTER TABLE transactions ADD COLUMN timezone_offset INTEGER NOT NULL DEFAULT 0;

-- 归属 + 时间复合索引：阶段 3 可见性过滤 / 阶段 4「我的记录」列表的主查询路径
CREATE INDEX IF NOT EXISTS idx_tx_creator ON transactions(book_id, creator_uid, deleted, tx_time);
