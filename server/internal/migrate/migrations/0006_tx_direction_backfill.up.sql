-- 0006：回填 0004 之前写入的历史转账行的 direction。
--
-- 为什么不在 0004 里改：0004 已随版本发布并被部分环境应用过，
-- 改动已应用的迁移对那些环境【不会重跑】，会造成"有的库回填了、有的没有"。
-- 因此回填单独立为 0006，保证所有库都会执行到。
--
-- 为什么可以这样判定方向（依据 createTransferInTx 的写入顺序）：
--   转账先插【转出侧】记录、再插【转入侧】记录，随后把 related_id 互指，
--   且主键自增 —— 所以同一对转账中：
--     * 转出侧：related_id = 对端 id > 自身 id  → direction = 1（转出）
--     * 转入侧：related_id = 对端 id < 自身 id  → direction = 2（转入）
--   该规则由代码写入顺序保证，是确定性的，不依赖猜测。
--
-- 不回填会怎样：direction=0 的转账在删除/恢复时无法区分转出/转入侧，
-- 余额反向会静默算成 0（钱不回滚）——属于"错得无声"，必须消除。

UPDATE transactions SET direction = 1
 WHERE type = 3 AND direction = 0 AND related_id > id;

UPDATE transactions SET direction = 2
 WHERE type = 3 AND direction = 0 AND related_id > 0 AND related_id < id;
