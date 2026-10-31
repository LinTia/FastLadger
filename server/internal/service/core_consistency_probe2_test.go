package service

// 记账核心数据一致性复审探针（第二轮）。
//
// 第二轮聚焦：幂等 op 复用的对称变体、删除/恢复的中途失败原子性、
// sync_version 单调性、分页游标边界、creator_uid 哨兵、TTL 边界。

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

// ---------- 探针 13：幂等 op 复用的对称变体 restore → delete → restore ----------
//
// 【GPT-C01（对称面）已修复】与 TestProbeCoreSameUUIDDeleteRestoreDelete 同根：
// replayMutateReceipt 现在会核对回执 result_ids 指向行的当前终态。"恢复→删除→再恢复"
// （同一 clientUuid）中第三次 restore 的预期终态是 deleted=0，但交易此刻仍是墓碑
// （deleted=1）——终态不符必须返回 ErrTransactionNotFound（404，与未命中同口径），
// 而不是静默吞掉并回退版本号。

func TestProbeCore2SameUUIDRestoreDeleteRestore(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p13-create", Amount: 2500}))
	must(t, err)
	txID := res.TransactionIds[0]

	// 先删一次（不同 uuid 建墓碑），再恢复（用待复用 uuid）
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "p13-del0"); err != nil {
		t.Fatal(err)
	}
	v1, err := e.txs.Restore(ctx, e.uid, e.bookID, txID, "reuse")
	must(t, err)
	if v1 <= 0 {
		t.Fatalf("[前置] 恢复应返回正版本，实际 %d", v1)
	}
	v2, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "p13-del1")
	must(t, err)
	if v2 <= v1 {
		t.Fatalf("[前置] 第二次删除版本应大于恢复版本：restore=%d delete=%d", v1, v2)
	}
	if e.balance(e.assetID) != 0 {
		t.Fatalf("[前置] 第二次删除后余额应为 0，实际 %d", e.balance(e.assetID))
	}
	// 第三次：再恢复（同一个 uuid，op=restore 与 v1 那次同 op）——终态不符，必须 404
	v3, err := e.txs.Restore(ctx, e.uid, e.bookID, txID, "reuse")
	if !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("GPT-C01：第三次 restore（同 uuid 重放、终态不符）应返回 ErrTransactionNotFound（404），实际 ver=%d err=%v", v3, err)
	}
	if got := e.balance(e.assetID); got != 0 {
		t.Errorf("GPT-C01：被拒的重放不得反转余额，实际 %d（期望 0，交易仍在回收站）", got)
	}
	var deleted bool
	if _, err := e.engine.SQL("SELECT deleted FROM transactions WHERE id = ?", txID).Get(&deleted); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Errorf("GPT-C01：被拒的重放不得改动目标行，实际 deleted=%v", deleted)
	}
}

// ---------- 探针 14：create → delete → 再次 create（同 uuid）返回已删的 ID ----------

func TestProbeCore2ReplayCreateAfterDeleted(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	req := e.expense(CreateRequest{ClientUuid: "p14-key", Amount: 3300})
	first, err := e.txs.Create(ctx, e.uid, e.bookID, req)
	must(t, err)
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, first.TransactionIds[0], "p14-del"); err != nil {
		t.Fatal(err)
	}
	// 客户端离线队列重放同一条 create（还没收到删除事件）
	second, err := e.txs.Create(ctx, e.uid, e.bookID, req)
	must(t, err)
	t.Logf("重放 create：replayed=%v ids=%v version=%d", second.Replayed, second.TransactionIds, second.Version)
	var deleted bool
	if _, err := e.engine.SQL("SELECT deleted FROM transactions WHERE id = ?", first.TransactionIds[0]).Get(&deleted); err != nil {
		t.Fatal(err)
	}
	if second.Replayed && deleted {
		t.Logf("结论：重放返回 Replayed=true，但目标行当前 deleted=%v —— 回执只保证"+
			"这条命令执行过，不反映它后来被删；客户端需靠 pull 的墓碑纠正（可接受，但对外无提示）", deleted)
	}
	if got := e.balance(e.assetID); got != 0 {
		t.Errorf("【可疑】重放 create 后余额应为 0（未重复记账），实际 %d", got)
	}
}

// ---------- 探针 15/16：转账删除 / 恢复中途失败必须整笔回滚（不留半截余额） ----------

// TestProbeCore2TransferDeleteMidwayFailureRollsBack 删除转账时，第二侧账户已被软删
// （绕过 R31 直接改库，模拟历史脏数据）：第一侧余额在内已被改，失败后必须整体回滚。
func TestProbeCore2TransferDeleteMidwayFailureRollsBack(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "p15-create", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 8800,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	outID := res.TransactionIds[0]
	before := rawBalance(t, e, e.assetID)
	before2 := rawBalance(t, e, e.toAssetID)

	// 把"后处理的那一侧"（转入侧）的账户打成软删
	if _, err := e.engine.Exec("UPDATE accounts SET deleted = 1, deleted_at = ? WHERE id = ?",
		time.Now().Unix(), e.toAssetID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "p15-del"); err == nil {
		t.Errorf("【可疑】对侧账户已软删时删除转账竟然成功")
	} else {
		t.Logf("删除被拒：%v", err)
	}
	if rawBalance(t, e, e.assetID) != before {
		t.Errorf("【可疑】删除失败后转出侧余额已被改动：%d → %d（应为 %d）", before, rawBalance(t, e, e.assetID), before)
	}
	if rawBalance(t, e, e.toAssetID) != before2 {
		t.Errorf("【可疑】删除失败后转入侧余额已被改动：%d → %d", before2, rawBalance(t, e, e.toAssetID))
	}
	var live int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM transactions WHERE book_id = ? AND deleted = 0 AND type = 3",
		e.bookID).Get(&live); err != nil {
		t.Fatal(err)
	}
	if live != 2 {
		t.Errorf("【可疑】删除失败后存活的转账行应为 2，实际 %d（出现单边墓碑）", live)
	}
}

// TestProbeCore2TransferRestoreMidwayFailureRollsBack 恢复转账的对称验证。
func TestProbeCore2TransferRestoreMidwayFailureRollsBack(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "p16-create", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 4400,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	outID := res.TransactionIds[0]
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "p16-del"); err != nil {
		t.Fatal(err)
	}
	before, before2 := rawBalance(t, e, e.assetID), rawBalance(t, e, e.toAssetID)
	if _, err := e.engine.Exec("UPDATE accounts SET deleted = 1, deleted_at = ? WHERE id = ?",
		time.Now().Unix(), e.toAssetID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.txs.Restore(ctx, e.uid, e.bookID, outID, "p16-res"); err == nil {
		t.Errorf("【可疑】对侧账户已软删时恢复转账竟然成功 → 悬空引用")
	} else {
		t.Logf("恢复被拒：%v", err)
	}
	if rawBalance(t, e, e.assetID) != before || rawBalance(t, e, e.toAssetID) != before2 {
		t.Errorf("【可疑】恢复失败后余额被改动：asset %d→%d, to %d→%d",
			before, rawBalance(t, e, e.assetID), before2, rawBalance(t, e, e.toAssetID))
	}
	var dead int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM transactions WHERE book_id = ? AND deleted = 1 AND type = 3",
		e.bookID).Get(&dead); err != nil {
		t.Fatal(err)
	}
	if dead != 2 {
		t.Errorf("【可疑】恢复失败后墓碑行应为 2，实际 %d（出现单边复活）", dead)
	}
}

// ---------- 探针 17：sync_version 单调性 + 与 book_versions 一致 ----------

func TestProbeCore2SyncVersionMonotonic(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "p17-create", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 1000,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	outID := res.TransactionIds[0]

	seen := make([]int64, 0, 8)
	record := func(tag string, v int64) {
		seen = append(seen, v)
		var bv int64
		if _, err := e.engine.SQL("SELECT version FROM book_versions WHERE book_id = ?", e.bookID).Get(&bv); err != nil {
			t.Fatal(err)
		}
		if v != bv {
			t.Errorf("【可疑】%s 返回版本 %d 与 book_versions.version=%d 不一致", tag, v, bv)
		}
		var maxTxVer, maxAcctVer int64
		if _, err := e.engine.SQL("SELECT MAX(sync_version) FROM transactions WHERE book_id = ?", e.bookID).Get(&maxTxVer); err != nil {
			t.Fatal(err)
		}
		if _, err := e.engine.SQL("SELECT MAX(sync_version) FROM accounts WHERE book_id = ?", e.bookID).Get(&maxAcctVer); err != nil {
			t.Fatal(err)
		}
		if maxTxVer > v || maxAcctVer > v {
			t.Errorf("【可疑】%s：实体 sync_version（tx=%d acct=%d）超过本批版本 %d", tag, maxTxVer, maxAcctVer, v)
		}
		t.Logf("%s → version=%d（tx max=%d, acct max=%d）", tag, v, maxTxVer, maxAcctVer)
	}
	record("create", res.Version)
	for i := 0; i < 3; i++ {
		v, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "p17-del"+strconv3(i))
		must(t, err)
		record("delete#"+strconv3(i), v)
		v, err = e.txs.Restore(ctx, e.uid, e.bookID, outID, "p17-res"+strconv3(i))
		must(t, err)
		record("restore#"+strconv3(i), v)
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] <= seen[i-1] {
			t.Errorf("【可疑】版本未严格单调递增：%v", seen)
			break
		}
	}
	t.Logf("版本序列：%v", seen)
}

// ---------- 探针 18：分页游标边界 ----------

func TestProbeCore2PagingCursorOverflow(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p18-" + strconv3(i), Amount: 100}))
		must(t, err)
	}
	// page=MaxInt32：offset 约 4.3e11，不溢出，正常返回空页
	items, total, err := e.txs.List(e.uid, e.bookID, TxListQuery{Page: math.MaxInt32, Count: 200})
	must(t, err)
	t.Logf("page=MaxInt32 时返回 %d 条（total=%d）", len(items), total)
	// 【GPT-C05 已修复】page=MaxInt64：offset 计算会溢出，必须显式报 ErrInvalidArgument（400），
	// 而不是回绕成负 OFFSET 被 SQLite 按 0 处理、返回首页数据却回显 page=MaxInt64。
	if _, _, err := e.txs.List(e.uid, e.bookID, TxListQuery{Page: math.MaxInt64, Count: 200}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("GPT-C05：page=MaxInt64 应返回 ErrInvalidArgument（400），实际 %v", err)
	}
	// 回收站视图同样受保护
	if _, _, err := e.txs.ListDeleted(e.uid, e.bookID, math.MaxInt64, 200); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("GPT-C05：ListDeleted page=MaxInt64 应返回 ErrInvalidArgument（400），实际 %v", err)
	}
}

// ---------- 探针 19：creator_uid=0 的遗留记录归属哨兵 ----------

func TestProbeCore2CreatorUidZeroSentinel(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p19-create", Amount: 1200}))
	must(t, err)
	txID := res.TransactionIds[0]
	// 人为把它变成"阶段 2 之前写入的未知归属"
	if _, err := e.engine.Exec("UPDATE transactions SET creator_uid = 0 WHERE id = ?", txID); err != nil {
		t.Fatal(err)
	}
	member := insertUser(t, e, "p19-member")
	addMember(t, e, member, model.RoleMember)
	if _, err := e.txs.Delete(ctx, member, e.bookID, txID, "p19-member-del"); !errors.Is(err, ErrForbidden) {
		t.Errorf("【可疑】member 删除 creator_uid=0 的遗留记录应 ErrForbidden，实际 %v", err)
	}
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "p19-owner-del"); err != nil {
		t.Errorf("owner 应能删除 creator_uid=0 的遗留记录，实际 %v", err)
	} else if e.balance(e.assetID) != 0 {
		t.Errorf("【可疑】owner 删除后余额应回滚为 0，实际 %d", e.balance(e.assetID))
	}
}

// ---------- 探针 20：回收站 TTL 边界 ----------

func TestProbeCore2RecycleTTLBoundaries(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p20-create", Amount: 900}))
	must(t, err)
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, res.TransactionIds[0], "p20-del"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	// deleted_at=0 的墓碑视为永不过期
	if _, err := e.engine.Exec("UPDATE transactions SET deleted_at = 0 WHERE id = ?", res.TransactionIds[0]); err != nil {
		t.Fatal(err)
	}
	n, err := e.txs.PurgeExpiredTransactions(e.bookID, 30, now)
	must(t, err)
	if n != 0 {
		t.Errorf("【可疑】deleted_at=0 的墓碑不应被清除，实际清了 %d 行", n)
	}
	// 恢复 deleted_at 到 40 天前 → 应被清
	if _, err := e.engine.Exec("UPDATE transactions SET deleted_at = ? WHERE id = ?",
		now-40*24*3600, res.TransactionIds[0]); err != nil {
		t.Fatal(err)
	}
	// ttlDays 非法值：0 / 负 / 超上限 / 使截止早于纪元
	for _, d := range []int{0, -1, 36501} {
		if _, err := e.txs.PurgeExpiredTransactions(e.bookID, d, now); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("【可疑】ttlDays=%d 应 ErrInvalidArgument，实际 %v", d, err)
		}
	}
	if _, err := e.txs.PurgeExpiredTransactions(e.bookID, 36500, 1); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("【可疑】now 过小应 ErrInvalidArgument，实际 %v", err)
	}
	if _, err := e.txs.PurgeExpiredTransactions(e.bookID, 30, 0); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("【可疑】now=0 应 ErrInvalidArgument，实际 %v", err)
	}
	// 合法清除
	n, err = e.txs.PurgeExpiredTransactions(e.bookID, 30, now)
	must(t, err)
	if n != 1 {
		t.Errorf("【可疑】到期墓碑应被清 1 行，实际 %d", n)
	}
}

// ---------- 探针 21：审计 before_json 记录的是变更前状态 ----------

func TestProbeCore2AuditBeforeJsonIsPreChange(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p21-create", Amount: 6600}))
	must(t, err)
	txID := res.TransactionIds[0]
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "p21-del"); err != nil {
		t.Fatal(err)
	}
	var before string
	if _, err := e.engine.SQL(
		"SELECT before_json FROM audit_logs WHERE book_id = ? AND action = ? AND entity_id = ?",
		e.bookID, model.AuditActionDelete, txID).Get(&before); err != nil {
		t.Fatal(err)
	}
	t.Logf("delete 审计 before_json 长度=%d", len(before))
	if len(before) == 0 {
		t.Errorf("【可疑】删除审计的 before_json 为空 —— 答不了'删前是什么'")
	}
	// before_json 里 deleted 必须为 false（变更前是活的）
	if !containsSubstr(before, `"deleted":false`) {
		t.Errorf("【可疑】before_json 不是变更前快照（未见 \"deleted\":false）：%s", before)
	}
	if _, err := e.txs.Restore(ctx, e.uid, e.bookID, txID, "p21-res"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.engine.SQL(
		"SELECT before_json FROM audit_logs WHERE book_id = ? AND action = ? AND entity_id = ?",
		e.bookID, model.AuditActionRestore, txID).Get(&before); err != nil {
		t.Fatal(err)
	}
	if !containsSubstr(before, `"deleted":true`) {
		t.Errorf("【可疑】restore 审计的 before_json 应记录墓碑态（未见 \"deleted\":true）")
	}
}

// ---------- 探针 22：账本可见性关闭后，回收站与列表不得泄露他人记录 ----------

func TestProbeCore2VisibilityAppliesToRecycleBin(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	other := insertUser(t, e, "p22-member")
	addMember(t, e, other, model.RoleMember)
	// 关掉账本互看
	if _, err := e.engine.Exec("UPDATE books SET members_can_view_others = 0 WHERE id = ?", e.bookID); err != nil {
		t.Fatal(err)
	}
	mine, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p22-mine", Amount: 100}))
	must(t, err)
	theirs, err := e.txs.Create(ctx, other, e.bookID, e.expense(CreateRequest{ClientUuid: "p22-theirs", Amount: 200}))
	must(t, err)
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, mine.TransactionIds[0], "p22-del-mine"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.txs.Delete(ctx, other, e.bookID, theirs.TransactionIds[0], "p22-del-theirs"); err != nil {
		t.Fatal(err)
	}
	// owner 视角：owner/admin 不受账本开关约束，看到全部是设计预期
	_, total, err := e.txs.ListDeleted(e.uid, e.bookID, 1, 20)
	must(t, err)
	if total != 2 {
		t.Errorf("owner 视角回收站应看到 2 条，实际 %d", total)
	}

	list2, total2, err := e.txs.ListDeleted(other, e.bookID, 1, 20)
	must(t, err)
	for _, r := range list2 {
		if r.CreatorUid != other {
			t.Errorf("【可疑】member 在关闭互看下看到了他人已删记录（id=%d creator=%d）", r.Id, r.CreatorUid)
		}
	}
	if total2 != 1 {
		t.Errorf("【可疑】member 视角回收站应只有自己的 1 条，实际 %d", total2)
	}
	_ = version.NewRepo()
}

// ---------- 小工具 ----------

func strconv3(i int) string {
	return string(rune('0' + i))
}

// rawBalance 忽略 deleted 直接读余额（软删账户也要能读到，用于验证"失败后未改动"）
func rawBalance(t *testing.T, e *testEnv, accountID int64) int64 {
	t.Helper()
	var b int64
	if _, err := e.engine.SQL("SELECT balance FROM accounts WHERE id = ?", accountID).Get(&b); err != nil {
		t.Fatalf("读账户余额失败 id=%d：%v", accountID, err)
	}
	return b
}

func containsSubstr(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
