package service

// 阶段 4 删除 / 恢复 / 审计 / 幂等的真实 SQLite 集成测试。
// 需 CGO（gcc），由 CI 在 ubuntu-latest 执行。复用 tx_integration_test.go 的 testEnv 与 helper。

import (
	"context"
	"errors"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

func insertUser(t *testing.T, e *testEnv, username string) int64 {
	t.Helper()
	u := &model.User{Username: username, PasswordHash: "x"}
	if _, err := e.engine.Insert(u); err != nil {
		t.Fatalf("插入用户失败：%v", err)
	}
	return u.Id
}

func addMember(t *testing.T, e *testEnv, uid int64, role int8) {
	t.Helper()
	now := time.Now().Unix()
	if _, err := e.engine.Insert(&model.BookMember{
		BookId: e.bookID, Uid: uid, Role: role, CanViewOthers: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("添加成员失败：%v", err)
	}
}

// auditCount 统计本账本的【交易域】审计条数。
//
// 为什么带 entity_type = 'transaction'：A8 之后账户 / 分类的写操作也会写审计
// （entity_type = 'account' / 'category'），而 newTestEnv 的建库准备本身就会建
// 3 个账户 + 2 个分类 + 1 本账——按 book_id + action 统计会把它们的 create 行
// 一起算进来，导致"创建应写 1 行"这类断言失去意义。
// 本文件的全部调用点断言的都是**交易**的 create / delete / restore，故把口径显式收敛到
// 交易域；这不是放宽断言，而是把它本来想断言的对象写清楚。
func auditCount(e *testEnv, action string) int64 {
	var n int64
	if _, err := e.engine.SQL(
		"SELECT COUNT(1) FROM audit_logs WHERE book_id = ? AND entity_type = 'transaction' AND action = ?",
		e.bookID, action).Get(&n); err != nil {
		e.t.Fatalf("统计审计失败：%v", err)
	}
	return n
}

// TestDeleteSingleReversesBalanceWritesAudit 删除单笔：余额精确回滚、软删（Get 404）、审计留痕。
func TestDeleteSingleReversesBalanceWritesAudit(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "del-single", Amount: 8000}))
	must(t, err)
	txID := res.TransactionIds[0]

	_, err = e.txs.Delete(ctx, e.uid, e.bookID, txID, "del-op-1")
	must(t, err)

	if got := e.balance(e.assetID); got != 0 {
		t.Fatalf("删除后资产余额应回滚为 0，实际 %d", got)
	}
	if _, err := e.txs.Get(e.uid, e.bookID, txID); !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("已删交易 Get 应返回 ErrTransactionNotFound，实际 err=%v", err)
	}
	if n := auditCount(e, model.AuditActionDelete); n != 1 {
		t.Fatalf("审计应有 1 行 delete，实际 %d", n)
	}
	// 换一个 clientUuid 再删（行已删）应 404：同一 uuid 属幂等重放，不属 404
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "del-op-2"); !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("重复删除应返回 404，实际 err=%v", err)
	}
}

// TestDeleteIdempotentSameClientUUID 同一 clientUuid 重复删除：只反转一次余额（WBS 4.2 幂等回执）。
// 这是离线重试场景的核心保证——否则余额会被反向两次。
func TestDeleteIdempotentSameClientUUID(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "idem-del", Amount: 8000}))
	must(t, err)
	txID := res.TransactionIds[0]

	v1, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "same-uuid")
	must(t, err)
	if got := e.balance(e.assetID); got != 0 {
		t.Fatalf("首次删除后余额应为 0，实际 %d", got)
	}

	// 重试：应命中回执重放，余额不得被再次反向（否则会变成 +8000）
	v2, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "same-uuid")
	must(t, err)
	if v2 != v1 {
		t.Fatalf("重放应返回同一版本：首次 %d，重放 %d", v1, v2)
	}
	if got := e.balance(e.assetID); got != 0 {
		t.Fatalf("重复删除后余额应仍为 0（幂等），实际 %d", got)
	}
	if n := auditCount(e, model.AuditActionDelete); n != 1 {
		t.Fatalf("重复删除不应再写审计，实际 %d 行", n)
	}
}

// TestDeleteSameUUIDDifferentTxConflict 同一 clientUuid 用于删除不同交易：内容不一致 → 冲突（409）。
func TestDeleteSameUUIDDifferentTxConflict(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	a, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "c-a", Amount: 1000}))
	must(t, err)
	b, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "c-b", Amount: 2000}))
	must(t, err)

	_, err = e.txs.Delete(ctx, e.uid, e.bookID, a.TransactionIds[0], "dup-uuid")
	must(t, err)
	_, err = e.txs.Delete(ctx, e.uid, e.bookID, b.TransactionIds[0], "dup-uuid")
	if !errors.Is(err, ErrCreateConflict) {
		t.Fatalf("同 clientUuid 删不同交易应返回 ErrCreateConflict，实际 err=%v", err)
	}
	if got := e.balance(e.assetID); got != -2000 {
		t.Fatalf("冲突请求不应改变余额（应为 -2000），实际 %d", got)
	}
}

// TestSameClientUUIDDeleteThenRestore op 维度回归（迁移 0005）：
// 同一 clientUuid 先删后恢复必须都被接受，不被互相当成重放。
func TestSameClientUUIDDeleteThenRestore(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "op-dim", Amount: 4000}))
	must(t, err)
	txID := res.TransactionIds[0]

	_, err = e.txs.Delete(ctx, e.uid, e.bookID, txID, "reuse-me")
	must(t, err)
	if e.balance(e.assetID) != 0 {
		t.Fatalf("删除后余额应 0，实际 %d", e.balance(e.assetID))
	}
	// 同一个 uuid 用于 restore：op 不同，应视为新操作而非删除的重放
	_, err = e.txs.Restore(ctx, e.uid, e.bookID, txID, "reuse-me")
	must(t, err)
	if got := e.balance(e.assetID); got != -4000 {
		t.Fatalf("恢复后余额应回到 -4000，实际 %d", got)
	}
}

// TestDeleteTransferReversesBothSides 删除转账：两侧余额都回滚、两条都软删、审计 2 行。
func TestDeleteTransferReversesBothSides(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "del-xfer", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 5000,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	if e.balance(e.assetID) != -5000 || e.balance(e.toAssetID) != 5000 {
		t.Fatalf("转账后余额异常：asset=%d toAsset=%d", e.balance(e.assetID), e.balance(e.toAssetID))
	}

	_, err = e.txs.Delete(ctx, e.uid, e.bookID, res.TransactionIds[0], "del-xfer-op")
	must(t, err)

	if e.balance(e.assetID) != 0 || e.balance(e.toAssetID) != 0 {
		t.Fatalf("删除转账后两侧应回滚为 0：asset=%d toAsset=%d", e.balance(e.assetID), e.balance(e.toAssetID))
	}
	var live int64
	if _, err := e.engine.SQL(
		"SELECT COUNT(1) FROM transactions WHERE book_id = ? AND deleted = 0 AND type = ?",
		e.bookID, model.TxTypeTransfer).Get(&live); err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Fatalf("删除后存活的转账行应为 0，实际 %d", live)
	}
	if n := auditCount(e, model.AuditActionDelete); n != 2 {
		t.Fatalf("转账删除审计应有 2 行，实际 %d", n)
	}
}

// TestRestoreReversesDelete 恢复：余额回到删除前、软删解除（Get 找回）、审计含 restore。
func TestRestoreReversesDelete(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "restore-1", Amount: 3000}))
	must(t, err)
	txID := res.TransactionIds[0]

	_, err = e.txs.Delete(ctx, e.uid, e.bookID, txID, "restore-del")
	must(t, err)
	if e.balance(e.assetID) != 0 {
		t.Fatalf("删除后余额应 0，实际 %d", e.balance(e.assetID))
	}

	_, err = e.txs.Restore(ctx, e.uid, e.bookID, txID, "restore-back")
	must(t, err)
	if e.balance(e.assetID) != -3000 {
		t.Fatalf("恢复后余额应回到 -3000，实际 %d", e.balance(e.assetID))
	}
	if _, err := e.txs.Get(e.uid, e.bookID, txID); err != nil {
		t.Fatalf("恢复后 Get 应找回交易，实际 err=%v", err)
	}
	if n := auditCount(e, model.AuditActionRestore); n != 1 {
		t.Fatalf("恢复审计应有 1 行，实际 %d", n)
	}
}

// TestDeletePermissionMemberCannotDeleteOthers owner/admin 可删任意；普通 member 仅本人。
func TestDeletePermissionMemberCannotDeleteOthers(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "perm-1", Amount: 5000}))
	must(t, err)
	txID := res.TransactionIds[0]

	// 普通成员不能删他人交易 → ErrForbidden
	other := insertUser(t, e, "member2")
	addMember(t, e, other, model.RoleMember)
	_, err = e.txs.Delete(ctx, other, e.bookID, txID, "perm-member")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("普通成员删他人应返回 ErrForbidden，实际 err=%v", err)
	}
	if e.balance(e.assetID) != -5000 {
		t.Fatalf("被拒后余额应不变（-5000），实际 %d", e.balance(e.assetID))
	}

	// 管理员可删他人交易 → 成功且余额回滚。
	// 前一次被拒已整体回滚、未占用幂等键，故此处用新 uuid 亦可。
	admin := insertUser(t, e, "admin2")
	addMember(t, e, admin, model.RoleAdmin)
	_, err = e.txs.Delete(ctx, admin, e.bookID, txID, "perm-admin")
	must(t, err)
	if e.balance(e.assetID) != 0 {
		t.Fatalf("管理员删除后余额应回滚为 0，实际 %d", e.balance(e.assetID))
	}
}

// TestRecycleBinListsDeleted 回收站视图（WBS 4.1）：只列已软删的，恢复后不再出现。
func TestRecycleBinListsDeleted(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "rc-1", Amount: 2000}))
	must(t, err)
	txID := res.TransactionIds[0]

	list, total, err := e.txs.ListDeleted(e.uid, e.bookID, 1, 20)
	must(t, err)
	if total != 0 {
		t.Fatalf("未删除时回收站应为空，实际 %d", total)
	}

	_, err = e.txs.Delete(ctx, e.uid, e.bookID, txID, "rc-del")
	must(t, err)
	list, total, err = e.txs.ListDeleted(e.uid, e.bookID, 1, 20)
	must(t, err)
	if total != 1 {
		t.Fatalf("删除后回收站应有 1 条，实际 %d", total)
	}
	if len(list) != 1 || list[0].Id != txID {
		t.Fatalf("回收站条目应为该交易，实际 %+v", list)
	}

	_, err = e.txs.Restore(ctx, e.uid, e.bookID, txID, "rc-restore")
	must(t, err)
	_, total, err = e.txs.ListDeleted(e.uid, e.bookID, 1, 20)
	must(t, err)
	if total != 0 {
		t.Fatalf("恢复后回收站应清空，实际 %d", total)
	}
}

// TestPurgeExpiredTransactions 回收站到期硬清除（WBS 4.1）：只清到期墓碑，活记录绝不动。
func TestPurgeExpiredTransactions(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	// 一条到期墓碑（deleted_at 设为 60 天前）
	old, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "pg-old", Amount: 1000}))
	must(t, err)
	_, err = e.txs.Delete(ctx, e.uid, e.bookID, old.TransactionIds[0], "pg-old-del")
	must(t, err)
	// 一条刚删的墓碑（未到期）
	fresh, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "pg-new", Amount: 1000}))
	must(t, err)
	_, err = e.txs.Delete(ctx, e.uid, e.bookID, fresh.TransactionIds[0], "pg-new-del")
	must(t, err)
	// 一条存活记录（绝不能被清）
	live, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "pg-live", Amount: 1000}))
	must(t, err)

	now := time.Now().Unix()
	if _, err := e.engine.Exec(
		"UPDATE transactions SET deleted_at = ? WHERE id = ?", now-60*24*3600, old.TransactionIds[0]); err != nil {
		t.Fatal(err)
	}

	n, err := e.txs.PurgeExpiredTransactions(e.bookID, 30, now)
	must(t, err)
	if n != 1 {
		t.Fatalf("应只清除 1 条到期墓碑，实际 %d", n)
	}
	var remain int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM transactions WHERE id = ?",
		old.TransactionIds[0]).Get(&remain); err != nil {
		t.Fatal(err)
	}
	if remain != 0 {
		t.Fatal("到期墓碑应已被硬删除")
	}
	// 未到期墓碑与存活记录都还在
	for _, id := range []int64{fresh.TransactionIds[0], live.TransactionIds[0]} {
		var c int64
		if _, err := e.engine.SQL("SELECT COUNT(1) FROM transactions WHERE id = ?", id).Get(&c); err != nil {
			t.Fatal(err)
		}
		if c != 1 {
			t.Fatalf("id=%d 不应被清除（未到期或存活）", id)
		}
	}
}

// TestPurgeAuditLogs 审计按保留期滚动清理（PRD §12.1：保留 1 年）。
func TestPurgeAuditLogs(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	_, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "au-1", Amount: 1000}))
	must(t, err)
	if n := auditCount(e, model.AuditActionCreate); n != 1 {
		t.Fatalf("创建应写 1 行 create 审计，实际 %d", n)
	}

	now := time.Now().Unix()
	// 把【交易域】审计改成 400 天前 → 超过 365 天保留期。
	// 加 entity_type 过滤的原因同 auditCount：账本创建 + 账户/分类的 create 审计
	// 也落在本账本上，它们不属于本用例要清理的那一条。
	if _, err := e.engine.Exec(
		"UPDATE audit_logs SET at = ? WHERE book_id = ? AND entity_type = 'transaction'",
		now-400*24*3600, e.bookID); err != nil {
		t.Fatal(err)
	}
	n, err := PurgeAuditLogs(e.engine, 365, now)
	must(t, err)
	if n != 1 {
		t.Fatalf("应清除 1 条过期审计，实际 %d", n)
	}
	if got := auditCount(e, model.AuditActionCreate); got != 0 {
		t.Fatalf("过期审计应已清除，实际剩 %d", got)
	}
}

// TestCreateRejectsDeletedAccount WBS 4.5：目标账户已软删 → ErrEntityDeleted（409），
// 不得静默成功（否则离线补传会复活已删数据）。
func TestCreateRejectsDeletedAccount(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	as := NewAccountService(e.engine, version.NewRepo())
	acct, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "待删账户", Category: model.CategoryCash, Currency: "CNY"})
	must(t, err)
	if _, err := as.Delete(e.uid, e.bookID, acct.Id); err != nil {
		t.Fatalf("删除账户失败：%v", err)
	}

	_, err = e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "del-acct", TxType: int(model.TxTypeExpense),
		AccountId: acct.Id, Amount: 1000, CategoryId: e.expenseCat,
		PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	if !errors.Is(err, ErrEntityDeleted) {
		t.Fatalf("引用已删账户应返回 ErrEntityDeleted，实际 err=%v", err)
	}
}

// TestDeleteRequiresClientUuid clientUuid 缺失必须由服务层拒绝（不依赖 handler）。
func TestDeleteRequiresClientUuid(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "need-uuid", Amount: 1000}))
	must(t, err)
	txID := res.TransactionIds[0]

	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "  "); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("空 clientUuid 应返回 ErrInvalidArgument，实际 err=%v", err)
	}
	if _, err := e.txs.Restore(ctx, e.uid, e.bookID, txID, ""); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("空 clientUuid 恢复应返回 ErrInvalidArgument，实际 err=%v", err)
	}
	if e.balance(e.assetID) != -1000 {
		t.Fatalf("被拒后余额应不变（-1000），实际 %d", e.balance(e.assetID))
	}
}
