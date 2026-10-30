-- 0005：幂等回执增加 op（操作类型）列，并把唯一键扩为 (book_id, uid, op, client_uuid)。
--
-- 背景（WBS 4.2 / 4.4）：删除 / 恢复必须与 Create 对称地【写幂等回执】，
-- 否则客户端离线重试会把余额反向两次。
--
-- 原唯一键只有 (book_id, uid, client_uuid)，没有操作维度：同一个 clientUuid
-- 先用于"删除"再用于"恢复"（或反之）会互相命中，导致恢复被误判为删除的重放。
-- 因此必须引入 op 维度区分 create / delete / restore。
--
-- 实现方式选择（对齐 migrate.go 约定）：
--   * 加列走 ALTER TABLE ADD COLUMN + NOT NULL DEFAULT 'create'：
--     老行自动回填为 'create'，语义正确且无数据损失；
--   * 唯一键只需 DROP INDEX + CREATE UNIQUE INDEX（SQLite 支持 DROP INDEX），
--     【不必重建表】——删索引不删数据，满足"老库升级不得删除旧回执"约束
--     （否则离线重试会产生重复账）。

ALTER TABLE idempotency_records ADD COLUMN op VARCHAR(16) NOT NULL DEFAULT 'create';

DROP INDEX IF EXISTS idx_idem_unique;

CREATE UNIQUE INDEX IF NOT EXISTS idx_idem_unique
  ON idempotency_records(book_id, uid, op, client_uuid);
