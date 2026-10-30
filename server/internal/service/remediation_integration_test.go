package service

// GPT 缺陷整改（R26 / R28 / R29 / R31 / R32 / R33 / R53）的**真实 SQLite 集成测试**。
//
// 需 CGO（gcc）：mattn/go-sqlite3 在本机（无 gcc）只能编译不能运行，因此这些用例
// 由 .github/workflows/ci.yml 在 ubuntu-latest 上执行。它们验证的是"数据完整性"
// ——余额、软删状态、审计是否三者一致，这类断言无法用纯逻辑或 mock 替代。
//
// 复用 tx_integration_test.go 的 testEnv / must / insertUser / addMember。

import (
	"context"
	"errors"
	"testing"
	"time"

	"fastledger/internal/model"
)

// ---------------------------------------------------------------------------
// 测试helper
// ---------------------------------------------------------------------------

// newTransfer 在本账本创建一笔转账，返回 [转出ID, 转入ID]
func newTransfer(t *testing.T, e *testEnv, clientUUID string, fromID, toID, amount int64) []int64 {
	t.Helper()
	res, err := e.txs.Create(context.Background(), e.uid, e.bookID, CreateRequest{
		ClientUuid: clientUUID, TxType: int(model.TxTypeTransfer),
		AccountId: fromID, ToAccountId: toID, Amount: amount,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	if len(res.TransactionIds) != 2 {
		t.Fatalf("转账应返回 2 个交易 ID，实际 %d", len(res.TransactionIds))
	}
	return res.TransactionIds
}

// txRow 直接读一行交易（含已软删）
func txRow(t *testing.T, e *testEnv, id int64) model.Transaction {
	t.Helper()
	var r model.Transaction
	has, err := e.engine.Where("id = ? AND book_id = ?", id, e.bookID).Get(&r)
	if err != nil {
		t.Fatalf("读交易 %d 失败：%v", id, err)
	}
	if !has {
		t.Fatalf("交易 %d 不存在", id)
	}
	return r
}

// execSQL 直接改库（用于构造"数据已损坏"的反例场景）
func execSQL(t *testing.T, e *testEnv, sql string, args ...interface{}) {
	t.Helper()
	if _, err := e.engine.Exec(append([]interface{}{sql}, args...)...); err != nil {
		t.Fatalf("执行 %q 失败：%v", sql, err)
	}
}

// assertTransferGroupUntouched 断言整笔转账未被部分改动：两侧存活态与余额都与期望一致
func assertTransferGroupUntouched(t *testing.T, e *testEnv, ids []int64, wantDeleted int, wantFrom, wantTo int64) {
	t.Helper()
	for _, id := range ids {
		if got := txRow(t, e, id).Deleted; boolToInt(got) != wantDeleted {
			t.Fatalf("交易 %d 的 deleted 应为 %d，实际 %d（发生了部分变更）", id, wantDeleted, boolToInt(got))
		}
	}
	if got := e.balance(e.assetID); got != wantFrom {
		t.Fatalf("转出账户余额应仍为 %d，实际 %d（发生了部分反向）", wantFrom, got)
	}
	if got := e.balance(e.toAssetID); got != wantTo {
		t.Fatalf("转入账户余额应仍为 %d，实际 %d（发生了部分反向）", wantTo, got)
	}
}

// ---------------------------------------------------------------------------
// GPT-R26：转账对侧缺失 / 关联错误 → 整笔拒绝，绝不单边
// ---------------------------------------------------------------------------

// TestR26TransferCounterpartMismatchRejected 逐项反例：配对损坏时删除必须整笔失败，
// 且两侧余额与交易状态都不发生任何部分变更（WBS 4.2 禁止单边路径）。
func TestR26TransferCounterpartMismatchRejected(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(t *testing.T, e *testEnv, outID, inID int64)
		want   error
	}{
		{
			name: "对侧不存在（related_id 指向不存在的行）",
			break_: func(t *testing.T, e *testEnv, outID, _ int64) {
				execSQL(t, e, "UPDATE transactions SET related_id = ? WHERE id = ?", 999999, outID)
			},
			want: ErrTransferGroupIncomplete,
		},
		{
			name: "对侧 ID 指向另一笔转账（未互指）",
			break_: func(t *testing.T, e *testEnv, outID, inID int64) {
				// 第二笔转账自洽配对，只有 outID/inID 这一对被改成"指向别人的对侧"
				newTransfer(t, e, "r26-other", e.assetID, e.toAssetID, 100)
				execSQL(t, e, "UPDATE transactions SET related_id = related_id + 2 WHERE id = ?", outID)
				execSQL(t, e, "UPDATE transactions SET related_id = related_id + 2 WHERE id = ?", inID)
			},
			want: ErrTransferGroupIncomplete,
		},
		{
			name: "两侧金额不一致",
			break_: func(t *testing.T, e *testEnv, _, inID int64) {
				execSQL(t, e, "UPDATE transactions SET amount = amount + 1 WHERE id = ?", inID)
			},
			want: ErrTransferGroupIncomplete,
		},
		{
			name: "账户未交叉对应（对侧 to_account_id 指错）",
			break_: func(t *testing.T, e *testEnv, _, inID int64) {
				execSQL(t, e, "UPDATE transactions SET to_account_id = ? WHERE id = ?", e.liabID, inID)
			},
			want: ErrTransferGroupIncomplete,
		},
		{
			name: "两侧 direction 不互补（同为转出）",
			break_: func(t *testing.T, e *testEnv, _, inID int64) {
				execSQL(t, e, "UPDATE transactions SET direction = ? WHERE id = ?", model.TxDirectionOut, inID)
			},
			want: ErrTransferGroupIncomplete,
		},
		{
			name: "对侧 type 不是转账",
			break_: func(t *testing.T, e *testEnv, _, inID int64) {
				execSQL(t, e, "UPDATE transactions SET type = ? WHERE id = ?", model.TxTypeExpense, inID)
			},
			want: ErrTransferGroupIncomplete,
		},
		{
			name: "对侧缺 direction 标记",
			break_: func(t *testing.T, e *testEnv, _, inID int64) {
				execSQL(t, e, "UPDATE transactions SET direction = ? WHERE id = ?", model.TxDirectionSingle, inID)
			},
			want: ErrTxDirectionMissing,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newTestEnv(t)
			ctx := context.Background()
			ids := newTransfer(t, e, "r26-xfer", e.assetID, e.toAssetID, 5000)
			outID, inID := ids[0], ids[1]

			c.break_(t, e, outID, inID)
			// 快照"破坏配对之后、操作之前"的余额：整笔回滚意味着它们必须原样不动
			wantFrom, wantTo := e.balance(e.assetID), e.balance(e.toAssetID)

			_, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "r26-del")
			if err == nil {
				t.Fatalf("配对损坏时删除必须整笔失败，实际通过（退化为单边操作）")
			}
			if !errors.Is(err, c.want) {
				t.Fatalf("错误类型应为 %v，实际 %v", c.want, err)
			}
			// 整笔回滚：两侧都还是存活态，两侧余额都没被反向
			assertTransferGroupUntouched(t, e, ids, 0, wantFrom, wantTo)
			if n := auditCount(e, model.AuditActionDelete); n != 0 {
				t.Fatalf("失败路径不应写审计，实际 %d 行", n)
			}
			// 从对侧发起同样必须被拒绝（配对校验对称）
			_, err = e.txs.Delete(ctx, e.uid, e.bookID, inID, "r26-del-2")
			if err == nil {
				t.Fatal("从对侧发起时也必须整笔失败")
			}
			assertTransferGroupUntouched(t, e, ids, 0, wantFrom, wantTo)
		})
	}
}

// TestR26RestoreRejectsBrokenPairNoPartialRestore 恢复路径同口径：
// 回收站里的转账若配对已损坏，恢复必须整笔失败，不得只恢复出一侧（孤儿记录）。
func TestR26RestoreRejectsBrokenPairNoPartialRestore(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	ids := newTransfer(t, e, "r26-restore", e.assetID, e.toAssetID, 5000)

	// 先正常删除：两侧都进回收站，余额回滚为 0
	_, err := e.txs.Delete(ctx, e.uid, e.bookID, ids[0], "r26-del")
	must(t, err)
	assertTransferGroupUntouched(t, e, ids, 1, 0, 0)

	// 删除后破坏配对（真实成因：对侧墓碑被并发硬清理 / 数据修复遗漏）
	execSQL(t, e, "UPDATE transactions SET related_id = ? WHERE id = ?", 999999, ids[1])

	_, err = e.txs.Restore(ctx, e.uid, e.bookID, ids[0], "r26-restore-op")
	if err == nil {
		t.Fatal("配对损坏时恢复必须整笔失败，实际通过")
	}
	if !errors.Is(err, ErrTransferGroupIncomplete) {
		t.Fatalf("错误类型应为 ErrTransferGroupIncomplete，实际 %v", err)
	}
	// 不得部分恢复：两侧都仍是已删态、余额仍是 0
	assertTransferGroupUntouched(t, e, ids, 1, 0, 0)
	if n := auditCount(e, model.AuditActionRestore); n != 0 {
		t.Fatalf("失败的恢复不应写审计，实际 %d 行", n)
	}
}

// TestR26SingleTxUnaffectedByPairingRules 单笔（收入/支出）不参与配对校验：
// 收紧转账规则不得误伤普通单笔的删除与恢复。
func TestR26SingleTxUnaffectedByPairingRules(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "r26-single", Amount: 3000}))
	must(t, err)
	txID := res.TransactionIds[0]

	_, err = e.txs.Delete(ctx, e.uid, e.bookID, txID, "r26-single-del")
	must(t, err)
	if got := e.balance(e.assetID); got != 0 {
		t.Fatalf("单笔删除后余额应为 0，实际 %d", got)
	}
	_, err = e.txs.Restore(ctx, e.uid, e.bookID, txID, "r26-single-restore")
	must(t, err)
	if got := e.balance(e.assetID); got != -3000 {
		t.Fatalf("单笔恢复后余额应为 -3000，实际 %d", got)
	}
}

// ---------------------------------------------------------------------------
// GPT-R32：恢复必须把 deleted_at 清零
// ---------------------------------------------------------------------------

// TestR32RestoreClearsDeletedAt 删除写墓碑时间、恢复清零；REST/Pull 读到同一口径。
func TestR32RestoreClearsDeletedAt(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "r32-1", Amount: 2500}))
	must(t, err)
	txID := res.TransactionIds[0]

	_, err = e.txs.Delete(ctx, e.uid, e.bookID, txID, "r32-del")
	must(t, err)
	if got := txRow(t, e, txID).DeletedAt; got <= 0 {
		t.Fatalf("删除后 deleted_at 应为墓碑时间（>0），实际 %d", got)
	}

	_, err = e.txs.Restore(ctx, e.uid, e.bookID, txID, "r32-restore")
	must(t, err)
	row := txRow(t, e, txID)
	if row.Deleted {
		t.Fatal("恢复后 deleted 应为 false")
	}
	if row.DeletedAt != 0 {
		t.Fatalf("恢复后 deleted_at 必须清零，实际 %d（会把恢复时间当成墓碑时间下发）", row.DeletedAt)
	}
	// 同一口径也体现在对外读路径：详情不带墓碑时间
	got, err := e.txs.Get(e.uid, e.bookID, txID)
	must(t, err)
	if got.DeletedAt != 0 {
		t.Fatalf("REST 读路径的 deletedAt 应为 0，实际 %d", got.DeletedAt)
	}
	// 再次删除时必须重新写入墓碑时间（不能沿用上次的脏值）
	_, err = e.txs.Delete(ctx, e.uid, e.bookID, txID, "r32-del-2")
	must(t, err)
	if got := txRow(t, e, txID).DeletedAt; got <= 0 {
		t.Fatalf("二次删除后 deleted_at 应重新写入墓碑时间，实际 %d", got)
	}
}

// TestR32RestoreTransferClearsDeletedAtBothSides 转账恢复：两侧 deleted_at 都清零。
func TestR32RestoreTransferClearsDeletedAtBothSides(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	ids := newTransfer(t, e, "r32-xfer", e.assetID, e.toAssetID, 4000)

	_, err := e.txs.Delete(ctx, e.uid, e.bookID, ids[0], "r32-xfer-del")
	must(t, err)
	for _, id := range ids {
		if got := txRow(t, e, id).DeletedAt; got <= 0 {
			t.Fatalf("删除后交易 %d 的 deleted_at 应 >0，实际 %d", id, got)
		}
	}
	_, err = e.txs.Restore(ctx, e.uid, e.bookID, ids[0], "r32-xfer-restore")
	must(t, err)
	for _, id := range ids {
		if got := txRow(t, e, id).DeletedAt; got != 0 {
			t.Fatalf("恢复后交易 %d 的 deleted_at 必须为 0，实际 %d", id, got)
		}
	}
	if e.balance(e.assetID) != -4000 || e.balance(e.toAssetID) != 4000 {
		t.Fatalf("恢复后余额应回到 -4000/4000，实际 %d/%d", e.balance(e.assetID), e.balance(e.toAssetID))
	}
}
