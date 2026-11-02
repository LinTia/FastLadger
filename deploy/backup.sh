#!/bin/bash
# FastLedger 每日一致性备份（WBS 7.2 / PRD §12 决策#5）
#
# 口径：
#   - 用 SQLite **online backup API**（等价 .backup）：一致性快照，与应用并发不
#     丢事务——满足 WBS 7.2 "用 .backup 或 VACUUM INTO" 的约束，
#     绝不做裸文件复制（WAL 未 checkpoint 的已提交事务会丢）；
#   - 只读 URI 打开源库，避免与应用争锁；
#   - 每日 1 份，滚动保留最近 **7 份**（PRD §12 决策#5）；
#   - 调度：/etc/cron.d/fastledger 每日 03:30（root），日志 /var/log/fastledger-backup.log。
#
# 恢复演练（WBS 7.3，每月一次）见 deploy/README.md 第 3 节。
set -e
D=/var/backups/fastledger
mkdir -p "$D"
export OUT="$D/db-$(date +%F).db"
python3 -c "import sqlite3,os; src=sqlite3.connect('file:/opt/fastledger/data/fastledger.db?mode=ro', uri=True); dst=sqlite3.connect(os.environ['OUT']); src.backup(dst); print('backup ->', os.environ['OUT'])"
# 滚动保留最近 7 份（按日期文件名排序，最旧的删除；与 PRD 决策#5 对齐，
# 原 30 天口径已按 PRD 收紧）
ls -1 "$D"/db-*.db 2>/dev/null | sort | head -n -7 | xargs -r rm -f
