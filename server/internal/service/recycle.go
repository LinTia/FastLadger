package service

// 回收站与审计保留（WBS 4.1 / 4.3；PRD §12.1 决策：回收站保留 30 天、审计保留 1 年）。
//
// 软删（deleted=1）本身只是墓碑：客户端靠它得知"这条被删了"（阶段 3 同步）。
// 墓碑不能无限保留（否则库只增不减），也不能提前清（否则离线客户端永远学不到删除）。
// PRD §12.1 取 30 天为折中：到期由 PurgeExpiredTransactions 硬清除。

import (
	"fmt"
	"strings"

	"fastledger/internal/model"

	"xorm.io/xorm"
)

// ListDeleted 回收站视图：列出本账本【已软删】的交易（WBS 4.1）。
//
// 读路径同样叠加可见性条件（与 List 一致）：关闭互看时 member 只看到本人的已删记录，
// 否则回收站会成为绕过可见性开关的后门（服务端权威，不能靠前端隐藏）。
// 按 deleted_at 倒序，最近删除的在前。
func (s *TxService) ListDeleted(uid, bookID int64, page, count int) ([]model.Transaction, int64, error) {
	canViewAll, err := canViewAllForTx(s.engine, uid, bookID)
	if err != nil {
		return nil, 0, err
	}
	where := []string{"book_id = ?", "deleted = 1"}
	args := []interface{}{bookID}
	if vcond, vargs := txVisibilityCond(canViewAll, uid); vcond != "" {
		where = append(where, vcond)
		args = append(args, vargs...)
	}
	cond := strings.Join(where, " AND ")

	var total int64
	if _, err := s.engine.SQL("SELECT COUNT(1) FROM transactions WHERE "+cond, args...).Get(&total); err != nil {
		return nil, 0, err
	}
	p, c := normalizePaging(page, count)
	listSQL := "SELECT * FROM transactions WHERE " + cond + " ORDER BY deleted_at DESC, id DESC LIMIT ? OFFSET ?"
	listArgs := append(append([]interface{}{}, args...), c, (p-1)*c)
	out := make([]model.Transaction, 0, c)
	if err := s.engine.SQL(listSQL, listArgs...).Find(&out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// PurgeExpiredTransactions 回收站到期硬清除（WBS 4.1：保留 EBK_RECYCLE_TTL_DAYS 天，到期硬清除）。
//
// 只清除 deleted=1 且 deleted_at 早于截止时刻的墓碑行；
// 存活记录（deleted=0）绝不动——误删活账比留着墓碑严重得多。
// deleted_at=0 的墓碑（写入异常产物）视为"永不过期"不清除，避免误删。
//
// 幂等回执（idempotency_records）【不】随之清除：老库升级与离线重试都依赖它，
// 删回执会导致客户端重放出重复账（0001 约束）。
func (s *TxService) PurgeExpiredTransactions(bookID int64, ttlDays int, now int64) (int64, error) {
	if bookID <= 0 {
		return 0, fmt.Errorf("%w：账本 ID 缺失", ErrInvalidArgument)
	}
	if ttlDays <= 0 {
		return 0, fmt.Errorf("%w：回收站保留天数必须为正整数", ErrInvalidArgument)
	}
	cutoff := now - int64(ttlDays)*secondsPerDay
	res, err := s.engine.Exec(
		`DELETE FROM transactions WHERE book_id = ? AND deleted = 1 AND deleted_at > 0 AND deleted_at <= ?`,
		bookID, cutoff)
	if err != nil {
		return 0, fmt.Errorf("回收站到期清除失败：%w", err)
	}
	return res.RowsAffected()
}

// PurgeAuditLogs 审计日志按保留期硬清除（PRD §12.1：保留 1 年）。
//
// 审计是 append-only：唯一允许的删除就是到期滚动清理，业务路径不得 UPDATE/DELETE。
// 清理本身不再写审计（否则审计会自我增殖）。
func PurgeAuditLogs(engine *xorm.Engine, retentionDays int, now int64) (int64, error) {
	if engine == nil {
		return 0, fmt.Errorf("%w：engine 缺失", ErrInvalidArgument)
	}
	if retentionDays <= 0 {
		return 0, fmt.Errorf("%w：审计保留天数必须为正整数", ErrInvalidArgument)
	}
	cutoff := now - int64(retentionDays)*secondsPerDay
	res, err := engine.Exec(`DELETE FROM audit_logs WHERE at <= ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("审计保留期清理失败：%w", err)
	}
	return res.RowsAffected()
}

// secondsPerDay 一天的秒数（仅用于保留期换算）
const secondsPerDay = 24 * 60 * 60
