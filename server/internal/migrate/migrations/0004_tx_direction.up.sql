-- 0004：交易增加 direction 列，记录转账方向（转出侧/转入侧）。
--
-- 仅靠 related_id 无法在删除/恢复时区分一笔转账的"转出侧"与"转入侧"
-- （两条记录互相指向、完全对称），因而无法精确反向余额变动。
-- 显式落 direction（0=单笔 1=转出侧 2=转入侧）后，删除/恢复可据其对
-- 每条记录所在账户做正确的符号反转。
--
-- SQLite 支持 ADD COLUMN + NOT NULL DEFAULT（全表回填默认值，老库升级无数据损失）。

ALTER TABLE transactions ADD COLUMN direction TINYINT NOT NULL DEFAULT 0;
