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
	"fmt"
	"strings"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
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

// newMemberSvc 构造成员服务（测试用，邀请有效期 7 天）
func newMemberSvc(t *testing.T, e *testEnv) *MemberService {
	t.Helper()
	as := NewAuthService(e.engine, testJWTSecret, time.Hour)
	ms, err := NewMemberService(e.engine, as, 7)
	must(t, err)
	return ms
}

// memberRole 读取成员当前角色（含已软删判定：deleted=0）
func memberRole(t *testing.T, e *testEnv, uid int64) int8 {
	t.Helper()
	var m model.BookMember
	has, err := e.engine.Where("book_id = ? AND uid = ? AND deleted = 0", e.bookID, uid).Get(&m)
	if err != nil {
		t.Fatalf("读成员角色失败：%v", err)
	}
	if !has {
		t.Fatalf("成员 %d 不存在或已被移除", uid)
	}
	return m.Role
}

// countInvites 统计本账本已签发的邀请码条数
func countInvites(t *testing.T, e *testEnv) int64 {
	t.Helper()
	var n int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM invites WHERE book_id = ?", e.bookID).Get(&n); err != nil {
		t.Fatalf("统计邀请码失败：%v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// GPT-R28：权限撤销后的 TOCTOU（真实 SQLite + 确定性屏障）
// ---------------------------------------------------------------------------

// r28AfterRevocation 确定性并发屏障（严禁随机 sleep）。
//
// 时序全部由 channel 钉死：
//  1. 撤销方在 goroutine 中执行并【提交】降权 / 移除（模拟并发的权限撤销）；
//  2. 请求 A 在另一个 goroutine 中等待 revoked 信号之后才发起敏感操作。
//
// 为什么屏障钉在这个位置：SQLite 单写者 + MaxOpenConns=1 决定了撤销方的写不可能
// 落进 A 的写事务内部（会被写锁挡住），因此"撤销先提交、A 后开工"是本引擎下唯一
// 确定可达的时序——而它正是旧实现出错的场景：旧实现在事务外先读一次角色，之后无论
// 权限怎么变都继续用那份旧快照。
func r28AfterRevocation(t *testing.T, revoke func() error, act func() error) error {
	t.Helper()
	revoked := make(chan error, 1)
	actErr := make(chan error, 1)
	go func() { revoked <- revoke() }()
	go func() {
		if err := <-revoked; err != nil {
			actErr <- fmt.Errorf("撤销权限失败：%w", err)
			return
		}
		actErr <- act()
	}()
	return <-actErr
}

// TestR28TxDeleteRestoreRejectStaleRole 交易删除 / 恢复：降权后不得用过期权限继续操作。
func TestR28TxDeleteRestoreRejectStaleRole(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	ms := newMemberSvc(t, e)
	admin := insertUser(t, e, "r28-tx-admin")
	addMember(t, e, admin, model.RoleAdmin)

	// owner 的三笔交易（creator 不是 admin，只有 admin 权限才能删）
	a, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "r28-a", Amount: 1000}))
	must(t, err)
	b, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "r28-b", Amount: 2000}))
	must(t, err)
	c, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "r28-c", Amount: 3000}))
	must(t, err)

	// ① A（admin）初始权限有效：能删他人的交易
	if _, err := e.txs.Delete(ctx, admin, e.bookID, a.TransactionIds[0], "r28-del-a"); err != nil {
		t.Fatalf("降权前管理员应能删除他人交易，实际 %v", err)
	}
	// ② 屏障：并发把 A 降权为 member 并提交 → ③ A 继续执行
	actErr := r28AfterRevocation(t,
		func() error { return ms.ChangeRole(e.uid, e.bookID, admin, model.RoleMember) },
		func() error {
			_, err := e.txs.Delete(ctx, admin, e.bookID, b.TransactionIds[0], "r28-del-b")
			return err
		})
	// ④ 不得用过期权限完成删除
	if !errors.Is(actErr, ErrForbidden) {
		t.Fatalf("降权后不得再用过期权限删除他人交易，实际 err=%v", actErr)
	}
	if got := e.balance(e.assetID); got != -5000 {
		t.Fatalf("被拒后余额应仍为 -5000（1000 已删 + 2000 + 3000 → -5000），实际 %d", got)
	}
	if n := auditCount(e, model.AuditActionDelete); n != 1 {
		t.Fatalf("只应有第 1 次成功删除的 1 行审计，实际 %d", n)
	}

	// 恢复路径同口径：先恢复 admin 权限并成功恢复一笔（证明初始权限有效）
	must(t, ms.ChangeRole(e.uid, e.bookID, admin, model.RoleAdmin))
	if _, err := e.txs.Restore(ctx, admin, e.bookID, a.TransactionIds[0], "r28-restore-a"); err != nil {
		t.Fatalf("恢复权限前管理员应能恢复他人交易，实际 %v", err)
	}
	_, err = e.txs.Delete(ctx, e.uid, e.bookID, c.TransactionIds[0], "r28-del-c")
	must(t, err)
	// ② 屏障降权 → ③ A 继续恢复
	actErr = r28AfterRevocation(t,
		func() error { return ms.ChangeRole(e.uid, e.bookID, admin, model.RoleMember) },
		func() error {
			_, err := e.txs.Restore(ctx, admin, e.bookID, c.TransactionIds[0], "r28-restore-c")
			return err
		})
	if !errors.Is(actErr, ErrForbidden) {
		t.Fatalf("降权后不得再用过期权限恢复他人交易，实际 err=%v", actErr)
	}
	// 恢复被拒：目标仍处于已删态，余额不得被"半恢复"
	if d := txRow(t, e, c.TransactionIds[0]).Deleted; !d {
		t.Fatal("被拒的恢复不得把交易变回存活态")
	}
	if got := e.balance(e.assetID); got != -3000 {
		t.Fatalf("被拒后余额应仍为 -3000，实际 %d", got)
	}
}

// TestR28MemberChangeRoleRejectsStaleRole 改角色：降权后不得再改他人角色。
func TestR28MemberChangeRoleRejectsStaleRole(t *testing.T) {
	e := newTestEnv(t)
	ms := newMemberSvc(t, e)
	admin := insertUser(t, e, "r28-role-admin")
	addMember(t, e, admin, model.RoleAdmin)
	target := insertUser(t, e, "r28-role-target")
	addMember(t, e, target, model.RoleMember)
	target2 := insertUser(t, e, "r28-role-target2")
	addMember(t, e, target2, model.RoleMember)

	// ① 初始权限有效
	must(t, ms.ChangeRole(admin, e.bookID, target, model.RoleAdmin))
	if got := memberRole(t, e, target); got != model.RoleAdmin {
		t.Fatalf("目标角色应已被改为 admin，实际 %d", got)
	}

	// ② 屏障降权 → ③ A 继续改别人角色
	actErr := r28AfterRevocation(t,
		func() error { return ms.ChangeRole(e.uid, e.bookID, admin, model.RoleMember) },
		func() error { return ms.ChangeRole(admin, e.bookID, target2, model.RoleAdmin) })
	if !errors.Is(actErr, ErrForbidden) {
		t.Fatalf("降权后不得再用过期权限改他人角色，实际 err=%v", actErr)
	}
	if got := memberRole(t, e, target2); got != model.RoleMember {
		t.Fatalf("被拒后目标角色不得改变，应仍为 member，实际 %d", got)
	}
	if n := auditCountAction(e, model.AuditActionPermissionChange); n != 2 {
		t.Fatalf("审计只应有「第 1 次改角色」与「降权」两行，实际 %d", n)
	}
}

// TestR28MemberRemoveRejectsStaleRole 移除成员：降权后不得再移除他人。
func TestR28MemberRemoveRejectsStaleRole(t *testing.T) {
	e := newTestEnv(t)
	ms := newMemberSvc(t, e)
	admin := insertUser(t, e, "r28-rm-admin")
	addMember(t, e, admin, model.RoleAdmin)
	target := insertUser(t, e, "r28-rm-target")
	addMember(t, e, target, model.RoleMember)
	target2 := insertUser(t, e, "r28-rm-target2")
	addMember(t, e, target2, model.RoleMember)

	// ① 初始权限有效
	must(t, ms.Remove(admin, e.bookID, target))

	// ② 屏障降权 → ③ A 继续移除他人
	actErr := r28AfterRevocation(t,
		func() error { return ms.ChangeRole(e.uid, e.bookID, admin, model.RoleMember) },
		func() error { return ms.Remove(admin, e.bookID, target2) })
	if !errors.Is(actErr, ErrForbidden) {
		t.Fatalf("降权后不得再用过期权限移除成员，实际 err=%v", actErr)
	}
	if got := memberRole(t, e, target2); got != model.RoleMember {
		t.Fatalf("被拒后目标成员不得被移除，实际角色 %d", got)
	}
}

// TestR28BookVisibilityRejectsStaleRole 账本可见性：降权后不得再改开关。
func TestR28BookVisibilityRejectsStaleRole(t *testing.T) {
	e := newTestEnv(t)
	ms := newMemberSvc(t, e)
	bs := NewBookService(e.engine, version.NewRepo())
	admin := insertUser(t, e, "r28-vis-admin")
	addMember(t, e, admin, model.RoleAdmin)

	old, err := bs.Get(e.bookID)
	must(t, err)

	// ① 初始权限有效
	changed, err := bs.SetVisibility(admin, e.bookID, !old.MembersCanViewOthers)
	must(t, err)
	if !changed {
		t.Fatal("管理员首次切换可见性应产生实际变更")
	}
	// ② 屏障降权 → ③ A 继续切换开关
	actErr := r28AfterRevocation(t,
		func() error { return ms.ChangeRole(e.uid, e.bookID, admin, model.RoleMember) },
		func() error {
			_, err := bs.SetVisibility(admin, e.bookID, old.MembersCanViewOthers)
			return err
		})
	if !errors.Is(actErr, ErrForbidden) {
		t.Fatalf("降权后不得再用过期权限改可见性开关，实际 err=%v", actErr)
	}
	now, err := bs.Get(e.bookID)
	must(t, err)
	if now.MembersCanViewOthers != !old.MembersCanViewOthers {
		t.Fatalf("被拒后开关值不得改变，应仍为 %t，实际 %t", !old.MembersCanViewOthers, now.MembersCanViewOthers)
	}
	if n := auditCountAction(e, model.AuditActionConfigChange); n != 1 {
		t.Fatalf("配置变更审计只应有第 1 次成功切换的 1 行，实际 %d", n)
	}
}

// TestR28CreateInviteRejectsStaleRole 签发邀请码：降权后不得再拉人入户。
func TestR28CreateInviteRejectsStaleRole(t *testing.T) {
	e := newTestEnv(t)
	ms := newMemberSvc(t, e)
	admin := insertUser(t, e, "r28-inv-admin")
	addMember(t, e, admin, model.RoleAdmin)

	// ① 初始权限有效（第 5 个参数为 GPT-R41 新增的 boundEmail：本用例不绑定）
	if _, err := ms.CreateInvite(admin, e.bookID, model.RoleMember, "", ""); err != nil {
		t.Fatalf("降权前管理员应能签发邀请码，实际 %v", err)
	}
	// ② 屏障降权 → ③ A 继续签发
	actErr := r28AfterRevocation(t,
		func() error { return ms.ChangeRole(e.uid, e.bookID, admin, model.RoleMember) },
		func() error {
			_, err := ms.CreateInvite(admin, e.bookID, model.RoleMember, "", "")
			return err
		})
	if !errors.Is(actErr, ErrForbidden) {
		t.Fatalf("降权后不得再用过期权限签发邀请码，实际 err=%v", actErr)
	}
	if n := countInvites(t, e); n != 1 {
		t.Fatalf("被拒后不应新增邀请码，应仍为 1 条，实际 %d", n)
	}
}

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

// ---------------------------------------------------------------------------
// GPT-R29：before_json 必须记录"提交时刻"的变更前状态
// ---------------------------------------------------------------------------

// TestR29BeforeJSONReflectsLatestStateInTx 连续两次变更：第二次的 before_json 必须是
// 【第一次变更之后】的状态，而不是事务外读到的初始值（GPT-R29）。
func TestR29BeforeJSONReflectsLatestStateInTx(t *testing.T) {
	e := newTestEnv(t)
	ms := newMemberSvc(t, e)
	bs := NewBookService(e.engine, version.NewRepo())
	target := insertUser(t, e, "r29-target")
	addMember(t, e, target, model.RoleMember)

	// ① member(3) → admin(2)：before_json 必须是 member
	must(t, ms.ChangeRole(e.uid, e.bookID, target, model.RoleAdmin))
	if b := auditBeforeJSON(e, model.AuditActionPermissionChange, target); !strings.Contains(b, `"role":3`) {
		t.Fatalf("第一次改角色的 before_json 应记录 member(3)，实际 %q", b)
	}
	// ② admin(2) → member(3)：before_json 必须是 admin，不能还是最初的 member
	must(t, ms.ChangeRole(e.uid, e.bookID, target, model.RoleMember))
	if b := auditBeforeJSON(e, model.AuditActionPermissionChange, target); !strings.Contains(b, `"role":2`) {
		t.Fatalf("第二次改角色的 before_json 应记录 admin(2)（变更前状态），实际 %q", b)
	}
	// ③ 移除：before_json 必须是 member
	must(t, ms.Remove(e.uid, e.bookID, target))
	if b := auditBeforeJSON(e, model.AuditActionPermissionChange, target); !strings.Contains(b, `"role":3`) {
		t.Fatalf("移除成员的 before_json 应记录 member(3)，实际 %q", b)
	}

	// ④ 可见性连切两次：第二次的 before_json 必须是第一次切换后的值
	old, err := bs.Get(e.bookID)
	must(t, err)
	changed, err := bs.SetVisibility(e.uid, e.bookID, !old.MembersCanViewOthers)
	must(t, err)
	if !changed {
		t.Fatal("第一次切换应产生实际变更")
	}
	switched := !old.MembersCanViewOthers
	if b := auditBeforeJSON(e, model.AuditActionConfigChange, e.bookID); !strings.Contains(b, fmt.Sprintf(`"membersCanViewOthers":%t`, old.MembersCanViewOthers)) {
		t.Fatalf("第一次切换的 before_json 应记录原值 %t，实际 %q", old.MembersCanViewOthers, b)
	}
	changed, err = bs.SetVisibility(e.uid, e.bookID, old.MembersCanViewOthers)
	must(t, err)
	if !changed {
		t.Fatal("第二次切换应产生实际变更")
	}
	if b := auditBeforeJSON(e, model.AuditActionConfigChange, e.bookID); !strings.Contains(b, fmt.Sprintf(`"membersCanViewOthers":%t`, switched)) {
		t.Fatalf("第二次切换的 before_json 应记录第一次切换后的值 %t，实际 %q", switched, b)
	}
}

// ---------------------------------------------------------------------------
// GPT-R31：软删（回收站）交易所依赖的账户 / 分类不可删除
// ---------------------------------------------------------------------------

// TestR31AccountDeleteBlockedBySoftDeletedTx 单笔软删后账户不可删；TTL 到期清理后可删。
func TestR31AccountDeleteBlockedBySoftDeletedTx(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	as := NewAccountService(e.engine, version.NewRepo())

	acct, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "待清理账户", Category: model.CategoryCash, Currency: "CNY"})
	must(t, err)
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "r31-single", TxType: int(model.TxTypeExpense),
		AccountId: acct.Id, Amount: 1000, CategoryId: e.expenseCat,
		PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	// 软删交易 → 账户仍在被回收站里的那笔交易引用
	_, err = e.txs.Delete(ctx, e.uid, e.bookID, res.TransactionIds[0], "r31-del")
	must(t, err)
	if _, err := as.Delete(e.uid, e.bookID, acct.Id); !errors.Is(err, ErrAccountInUse) {
		t.Fatalf("回收站内交易仍引用该账户，删除必须被拒，实际 err=%v", err)
	}
	// TTL 到期硬清理之后，账户才可正常删除
	now := time.Now().Unix()
	execSQL(t, e, "UPDATE transactions SET deleted_at = ? WHERE id = ?", now-60*24*3600, res.TransactionIds[0])
	n, err := e.txs.PurgeExpiredTransactions(e.bookID, 30, now)
	must(t, err)
	if n != 1 {
		t.Fatalf("到期墓碑应被清除 1 条，实际 %d", n)
	}
	if _, err := as.Delete(e.uid, e.bookID, acct.Id); err != nil {
		t.Fatalf("墓碑清理后账户应可正常删除，实际 err=%v", err)
	}
}

// TestR31AccountDeleteBlockedByTransferEitherSide 转账软删后，两侧账户都不可删。
func TestR31AccountDeleteBlockedByTransferEitherSide(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	as := NewAccountService(e.engine, version.NewRepo())
	from, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "转出户", Category: model.CategoryCash, Currency: "CNY"})
	must(t, err)
	to, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "转入户", Category: model.CategoryDemandDeposit, Currency: "CNY"})
	must(t, err)

	ids := newTransfer(t, e, "r31-xfer", from.Id, to.Id, 7000)
	_, err = e.txs.Delete(ctx, e.uid, e.bookID, ids[0], "r31-xfer-del")
	must(t, err)

	// 转出侧（走 account_id）
	if _, err := as.Delete(e.uid, e.bookID, from.Id); !errors.Is(err, ErrAccountInUse) {
		t.Fatalf("转账转出侧账户应被引用保护，实际 err=%v", err)
	}
	// 转入侧（走 to_account_id）
	if _, err := as.Delete(e.uid, e.bookID, to.Id); !errors.Is(err, ErrAccountInUse) {
		t.Fatalf("转账转入侧账户应被引用保护，实际 err=%v", err)
	}
	// 恢复后账户仍被存活交易引用，同样不可删（原有行为保持）
	_, err = e.txs.Restore(ctx, e.uid, e.bookID, ids[0], "r31-xfer-restore")
	must(t, err)
	if _, err := as.Delete(e.uid, e.bookID, from.Id); !errors.Is(err, ErrAccountInUse) {
		t.Fatalf("恢复后账户仍被引用，删除应被拒，实际 err=%v", err)
	}
}

// TestR31CategoryDeleteBlockedBySoftDeletedTx 分类同样被回收站里的交易保护。
func TestR31CategoryDeleteBlockedBySoftDeletedTx(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	cs := NewCategoryService(e.engine, version.NewRepo())
	cat, err := cs.Create(e.uid, e.bookID, CategoryCreateInput{Name: "待清理分类", CatType: model.CategoryTypeExpense})
	must(t, err)
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "r31-cat", TxType: int(model.TxTypeExpense),
		AccountId: e.assetID, Amount: 1500, CategoryId: cat.Id,
		PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	_, err = e.txs.Delete(ctx, e.uid, e.bookID, res.TransactionIds[0], "r31-cat-del")
	must(t, err)

	if _, err := cs.Delete(e.uid, e.bookID, cat.Id); !errors.Is(err, ErrCategoryInUse) {
		t.Fatalf("回收站内交易仍引用该分类，删除必须被拒，实际 err=%v", err)
	}
	now := time.Now().Unix()
	execSQL(t, e, "UPDATE transactions SET deleted_at = ? WHERE id = ?", now-60*24*3600, res.TransactionIds[0])
	if _, err := e.txs.PurgeExpiredTransactions(e.bookID, 30, now); err != nil {
		t.Fatalf("清理墓碑失败：%v", err)
	}
	if _, err := cs.Delete(e.uid, e.bookID, cat.Id); err != nil {
		t.Fatalf("墓碑清理后分类应可正常删除，实际 err=%v", err)
	}
}

// TestR31RestoreRejectedWhenAccountDeleted 恢复时校验引用有效：
// 账户已删（历史遗留的坏数据）时恢复必须整笔失败，不得部分恢复出悬空引用。
func TestR31RestoreRejectedWhenAccountDeleted(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	as := NewAccountService(e.engine, version.NewRepo())
	acct, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "会被删的账户", Category: model.CategoryCash, Currency: "CNY"})
	must(t, err)
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "r31-dangling", TxType: int(model.TxTypeExpense),
		AccountId: acct.Id, Amount: 2000, CategoryId: e.expenseCat,
		PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	_, err = e.txs.Delete(ctx, e.uid, e.bookID, res.TransactionIds[0], "r31-dangling-del")
	must(t, err)
	// 模拟修复前遗留的坏数据：交易还在回收站，账户却已被软删
	execSQL(t, e, "UPDATE accounts SET deleted = 1, deleted_at = ? WHERE id = ?", time.Now().Unix(), acct.Id)

	_, err = e.txs.Restore(ctx, e.uid, e.bookID, res.TransactionIds[0], "r31-dangling-restore")
	if err == nil {
		t.Fatal("账户已删时恢复必须失败，否则会恢复出悬空引用")
	}
	if !errors.Is(err, ErrEntityDeleted) {
		t.Fatalf("应返回 ErrEntityDeleted，实际 err=%v", err)
	}
	// 不得部分恢复：交易仍是已删态、余额仍是 0
	if d := txRow(t, e, res.TransactionIds[0]).Deleted; !d {
		t.Fatal("失败的恢复不得把交易变回存活态")
	}
	// 账户已被软删，e.balance() 按 deleted=0 过滤会取不到该行；
	// 这里直接读该行的 balance 列，断言本身不变（失败的恢复不得改动余额）。
	var bal int64
	if _, err := e.engine.SQL("SELECT balance FROM accounts WHERE id = ?", acct.Id).Get(&bal); err != nil {
		t.Fatalf("读取账户余额失败：%v", err)
	}
	if bal != 0 {
		t.Fatalf("失败的恢复不得改动余额，应仍为 0，实际 %d", bal)
	}
	if n := auditCount(e, model.AuditActionRestore); n != 0 {
		t.Fatalf("失败的恢复不应写审计，实际 %d 行", n)
	}
}

// ---------------------------------------------------------------------------
// GPT-R33：PUT /books/:id 名称 + 可见性原子更新
// ---------------------------------------------------------------------------

// TestR33BookUpdateNameAndVisibilityAtomic 名称与可见性一次提交；无变化幂等；
// 无权限时两者都不落库、不写审计（广播由 handler 在 Update 成功返回后执行）。
func TestR33BookUpdateNameAndVisibilityAtomic(t *testing.T) {
	e := newTestEnv(t)
	bs := NewBookService(e.engine, version.NewRepo())

	old, err := bs.Get(e.bookID)
	must(t, err)
	wantVis := !old.MembersCanViewOthers

	res, err := bs.Update(e.uid, e.bookID, "整改后的账本名", &wantVis)
	must(t, err)
	if !res.NameChanged {
		t.Fatal("名称应发生实际变更")
	}
	if !res.VisibilityChanged {
		t.Fatal("可见性应发生实际变更")
	}
	got, err := bs.Get(e.bookID)
	must(t, err)
	if got.Name != "整改后的账本名" {
		t.Fatalf("名称未落库，实际 %q", got.Name)
	}
	if got.MembersCanViewOthers != wantVis {
		t.Fatalf("可见性未落库，实际 %t", got.MembersCanViewOthers)
	}
	// 审计与变更同事务：一次切换一条审计，且记录变更前的旧值
	if n := auditCountAction(e, model.AuditActionConfigChange); n != 1 {
		t.Fatalf("配置变更审计应为 1 行，实际 %d", n)
	}
	if b := auditBeforeJSON(e, model.AuditActionConfigChange, e.bookID); !strings.Contains(b, fmt.Sprintf(`"membersCanViewOthers":%t`, old.MembersCanViewOthers)) {
		t.Fatalf("before_json 应记录变更前值 %t，实际 %q", old.MembersCanViewOthers, b)
	}

	// 幂等：同值再提交 → 既不改名也不改开关，且不再写审计
	res2, err := bs.Update(e.uid, e.bookID, "整改后的账本名", &wantVis)
	must(t, err)
	if res2.NameChanged || res2.VisibilityChanged {
		t.Fatalf("同值提交不应产生变更：name=%v vis=%v", res2.NameChanged, res2.VisibilityChanged)
	}
	if n := auditCountAction(e, model.AuditActionConfigChange); n != 1 {
		t.Fatalf("幂等提交不应再写审计，实际 %d 行", n)
	}

	// 无权限：名称与开关必须"都不变"（整体回滚），且不写审计
	member := insertUser(t, e, "r33-member")
	addMember(t, e, member, model.RoleMember)
	other := !wantVis
	if _, err := bs.Update(member, e.bookID, "不该生效的名字", &other); !errors.Is(err, ErrForbidden) {
		t.Fatalf("普通成员改账本设置应返回 ErrForbidden，实际 err=%v", err)
	}
	after, err := bs.Get(e.bookID)
	must(t, err)
	if after.Name != "整改后的账本名" {
		t.Fatalf("被拒后名称不得改变，实际 %q", after.Name)
	}
	if after.MembersCanViewOthers != wantVis {
		t.Fatalf("被拒后可见性不得改变，实际 %t", after.MembersCanViewOthers)
	}
	if n := auditCountAction(e, model.AuditActionConfigChange); n != 1 {
		t.Fatalf("被拒的请求不应写审计，实际 %d 行", n)
	}
}

// ---------------------------------------------------------------------------
// GPT-R53：父账户不得在仍有活跃子账户时被删除
// ---------------------------------------------------------------------------

// TestR53DeleteParentWithActiveChildrenRejected 有活跃子账户时拒绝删父。
//
// GPT-C06 修订：子账户【软删】同样阻止父账户删除——软删子账户日后恢复会带着悬空的
// parent_id，比活跃子账户更危险；只有子账户被硬清（回收站 TTL 到期清除的等价态）后，
// 父账户才可正常删除。
func TestR53DeleteParentWithActiveChildrenRejected(t *testing.T) {
	e := newTestEnv(t)
	as := NewAccountService(e.engine, version.NewRepo())

	parent, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "父账户", Category: model.CategoryCash, Currency: "CNY"})
	must(t, err)
	child, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "子账户", Category: model.CategoryCash, Currency: "CNY", ParentId: parent.Id})
	must(t, err)

	// 有活跃子账户 → 明确拒绝（不能留下悬空 parent_id）
	if _, err := as.Delete(e.uid, e.bookID, parent.Id); !errors.Is(err, ErrAccountHasChildren) {
		t.Fatalf("仍有活跃子账户时删除父账户必须被拒，实际 err=%v", err)
	}
	var alive int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM accounts WHERE id = ? AND deleted = 0", parent.Id).Get(&alive); err != nil {
		t.Fatal(err)
	}
	if alive != 1 {
		t.Fatal("被拒后父账户必须仍存活")
	}

	// 子账户软删 → 父账户仍不可删（GPT-C06：软删子账户恢复会带悬空 parent_id）
	if _, err := as.Delete(e.uid, e.bookID, child.Id); err != nil {
		t.Fatalf("删除子账户失败：%v", err)
	}
	if _, err := as.Delete(e.uid, e.bookID, parent.Id); !errors.Is(err, ErrAccountHasChildren) {
		t.Fatalf("GPT-C06：子账户仅软删时父账户应被拒绝删除，实际 err=%v", err)
	}
	// 子账户被硬清（回收站到期清除的等价态）→ 父账户可正常删除
	if _, err := e.engine.Exec("DELETE FROM accounts WHERE id = ?", child.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := as.Delete(e.uid, e.bookID, parent.Id); err != nil {
		t.Fatalf("子账户硬清后父账户应可删除，实际 err=%v", err)
	}
	// 不变量：不存在"活跃子账户指向已删父账户"的悬空引用
	var dangling int64
	if _, err := e.engine.SQL(
		`SELECT COUNT(1) FROM accounts c
		 INNER JOIN accounts p ON p.id = c.parent_id
		 WHERE c.book_id = ? AND c.deleted = 0 AND p.deleted = 1`, e.bookID).Get(&dangling); err != nil {
		t.Fatal(err)
	}
	if dangling != 0 {
		t.Fatalf("出现 %d 条悬空 parent_id（活跃子账户指向已删父账户）", dangling)
	}
}
