package service

// 阶段 4 删除 / 恢复 / 审计的真实 SQLite 集成测试。
// 需 CGO（gcc），由 CI 在 ubuntu-latest 执行。复用 tx_integration_test.go 的 testEnv 与 helper。

import (
	"context"
	"errors"
	"testing"
	"time"

	"fastledger/internal/model"
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

func auditCount(e *testEnv, action string) int64 {
	var n int64
	if _, err := e.engine.SQL(
		"SELECT COUNT(1) FROM audit_logs WHERE book_id = ? AND action = ?", e.bookID, action).Get(&n); err != nil {
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

	_, err = e.txs.Delete(ctx, e.uid, e.bookID, txID)
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
	// 重复删除（已删状态）应 404，而非再写一遍
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID); !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("重复删除应返回 404，实际 err=%v", err)
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

	_, err = e.txs.Delete(ctx, e.uid, e.bookID, res.TransactionIds[0])
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

	_, err = e.txs.Delete(ctx, e.uid, e.bookID, txID)
	must(t, err)
	if e.balance(e.assetID) != 0 {
		t.Fatalf("删除后余额应 0，实际 %d", e.balance(e.assetID))
	}

	_, err = e.txs.Restore(ctx, e.uid, e.bookID, txID)
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
	_, err = e.txs.Delete(ctx, other, e.bookID, txID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("普通成员删他人应返回 ErrForbidden，实际 err=%v", err)
	}
	if e.balance(e.assetID) != -5000 {
		t.Fatalf("被拒后余额应不变（-5000），实际 %d", e.balance(e.assetID))
	}

	// 管理员可删他人交易 → 成功且余额回滚
	admin := insertUser(t, e, "admin2")
	addMember(t, e, admin, model.RoleAdmin)
	_, err = e.txs.Delete(ctx, admin, e.bookID, txID)
	must(t, err)
	if e.balance(e.assetID) != 0 {
		t.Fatalf("管理员删除后余额应回滚为 0，实际 %d", e.balance(e.assetID))
	}
}
