package service

// 记账核心数据一致性复审探针（reviewer-core）。
//
// 目的：对 tx.go / account.go / category.go / recycle.go / ttl.go 里"看起来可疑"的
// 每一条假设，用真实 SQLite 跑一遍，确认它到底是【真 Bug】还是【已被别的约束挡住】。
// 本文件不修改任何生产代码；每条用例都是可保留的回归测试。

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

// ---------- 探针 1：幂等 op 维度 —— 同一 clientUuid 先 delete 再 restore 再 delete ----------

// TestProbeCoreSameUUIDDeleteRestoreDelete 同一 clientUuid 的 op=delete 回执在
// 「删除 → 恢复 → 再删除」序列中会不会把第三次删除吞掉。
//
// 期望：第三次要么真的把交易删掉（余额回滚），要么明确报错；
// 实际观察：命中第一次 delete 的旧回执 → 静默返回旧版本、什么都不做。
func TestProbeCoreSameUUIDDeleteRestoreDelete(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p1-create", Amount: 4000}))
	must(t, err)
	txID := res.TransactionIds[0]

	v1, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "reuse")
	must(t, err)
	if got := e.balance(e.assetID); got != 0 {
		t.Fatalf("[前置] 删除后余额应为 0，实际 %d", got)
	}
	v2, err := e.txs.Restore(ctx, e.uid, e.bookID, txID, "reuse")
	must(t, err)
	if v2 <= v1 {
		t.Fatalf("[前置] 恢复版本应大于删除版本：delete=%d restore=%d", v1, v2)
	}
	if got := e.balance(e.assetID); got != -4000 {
		t.Fatalf("[前置] 恢复后余额应回到 -4000，实际 %d", got)
	}

	// 第三次：再次删除（同一个 clientUuid）
	//
	// 【GPT-C01 已修复】重放分支现在会核对回执 result_ids 指向行的当前终态：
	// 交易已被恢复（deleted=0），与 delete 回执的预期终态（deleted=1）不符，
	// 必须与"幂等未命中"同口径返回 ErrTransactionNotFound（HTTP 404），
	// 而不是静默返回旧版本、吞掉本次操作。
	v3, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "reuse")
	if !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("GPT-C01：第三次 delete（同 uuid 重放、终态不符）应返回 ErrTransactionNotFound（404），实际 ver=%d err=%v", v3, err)
	}
	var deleted bool
	if _, err := e.engine.SQL("SELECT deleted FROM transactions WHERE id = ?", txID).Get(&deleted); err != nil {
		t.Fatal(err)
	}
	if deleted {
		t.Errorf("GPT-C01：被拒的重放不得改动目标行，实际 deleted=%v", deleted)
	}
	if got := e.balance(e.assetID); got != -4000 {
		t.Errorf("GPT-C01：被拒的重放不得反转余额，实际 %d（期望 -4000）", got)
	}
}

// ---------- 探针 2：迁移 0006 的 direction 回填假设是否仍然成立 ----------

// TestProbeCoreTransferDirectionOrder 验证 0006 依赖的写入顺序假设：
// 转出侧先插入（id 较小，related_id > id → direction=1）、转入侧后插入（related_id < id → direction=2）。
func TestProbeCoreTransferDirectionOrder(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "p2-xfer", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 7000,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	if len(res.TransactionIds) != 2 {
		t.Fatalf("转账应返回 2 个 ID，实际 %d", len(res.TransactionIds))
	}
	outID, inID := res.TransactionIds[0], res.TransactionIds[1]

	var outRow, inRow model.Transaction
	if _, err := e.engine.ID(outID).Get(&outRow); err != nil {
		t.Fatal(err)
	}
	if _, err := e.engine.ID(inID).Get(&inRow); err != nil {
		t.Fatal(err)
	}
	t.Logf("out(id=%d) account=%d direction=%d related=%d | in(id=%d) account=%d direction=%d related=%d",
		outRow.Id, outRow.AccountId, outRow.Direction, outRow.RelatedId,
		inRow.Id, inRow.AccountId, inRow.Direction, inRow.RelatedId)

	if !(outRow.Id < inRow.Id) {
		t.Errorf("【假设破裂】转出侧 id(%d) 不小于转入侧 id(%d)，迁移 0006 的回填规则失效", outRow.Id, inRow.Id)
	}
	if outRow.Direction != model.TxDirectionOut || outRow.RelatedId != inRow.Id {
		t.Errorf("【假设破裂】转出侧 direction=%d related=%d（期望 1 / %d）", outRow.Direction, outRow.RelatedId, inRow.Id)
	}
	if inRow.Direction != model.TxDirectionIn || inRow.RelatedId != outRow.Id {
		t.Errorf("【假设破裂】转入侧 direction=%d related=%d（期望 2 / %d）", inRow.Direction, inRow.RelatedId, outRow.Id)
	}
	if outRow.AccountId != e.assetID || inRow.AccountId != e.toAssetID {
		t.Errorf("【假设破裂】账户归属与 [转出,转入] 返回顺序不一致")
	}
}

// ---------- 探针 3：转账 delete/restore 多轮往返不漂移 ----------

func TestProbeCoreTransferDeleteRestoreNoDrift(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "p3-create", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 12345,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	outID := res.TransactionIds[0]

	for i := 0; i < 5; i++ {
		if _, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, sprintfUUID("p3-del", i)); err != nil {
			t.Fatalf("第 %d 轮删除失败：%v", i, err)
		}
		if e.balance(e.assetID) != 0 || e.balance(e.toAssetID) != 0 {
			t.Fatalf("第 %d 轮删除后余额未归零：asset=%d to=%d", i, e.balance(e.assetID), e.balance(e.toAssetID))
		}
		if _, err := e.txs.Restore(ctx, e.uid, e.bookID, outID, sprintfUUID("p3-res", i)); err != nil {
			t.Fatalf("第 %d 轮恢复失败：%v", i, err)
		}
		if e.balance(e.assetID) != -12345 || e.balance(e.toAssetID) != 12345 {
			t.Fatalf("第 %d 轮恢复后余额漂移：asset=%d to=%d（期望 -12345 / 12345）",
				i, e.balance(e.assetID), e.balance(e.toAssetID))
		}
	}
	// 恢复态下 deleted / deleted_at 必须干净（R32）
	var rows []model.Transaction
	if err := e.engine.Where("book_id = ? AND deleted = 0", e.bookID).Find(&rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.DeletedAt != 0 {
			t.Errorf("【可疑】恢复后的行 id=%d 仍带墓碑时间 deleted_at=%d", r.Id, r.DeletedAt)
		}
		if r.TxType == model.TxTypeTransfer && r.Direction == model.TxDirectionSingle {
			t.Errorf("【可疑】恢复后的转账行 id=%d direction 退化成 0", r.Id)
		}
	}
}

// ---------- 探针 4：单笔 delete/restore 多轮往返不漂移 ----------

func TestProbeCoreSingleDeleteRestoreNoDrift(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p4-create", Amount: 999}))
	must(t, err)
	txID := res.TransactionIds[0]
	for i := 0; i < 5; i++ {
		if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, sprintfUUID("p4-del", i)); err != nil {
			t.Fatalf("第 %d 轮删除失败：%v", i, err)
		}
		if e.balance(e.assetID) != 0 {
			t.Fatalf("第 %d 轮删除后余额=%d（期望 0）", i, e.balance(e.assetID))
		}
		if _, err := e.txs.Restore(ctx, e.uid, e.bookID, txID, sprintfUUID("p4-res", i)); err != nil {
			t.Fatalf("第 %d 轮恢复失败：%v", i, err)
		}
		if e.balance(e.assetID) != -999 {
			t.Fatalf("第 %d 轮恢复后余额=%d（期望 -999）", i, e.balance(e.assetID))
		}
	}
}

// ---------- 探针 5：负债账户参与转账的 delete/restore 往返符号 ----------

func TestProbeCoreLiabilityTransferRoundTrip(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	// 信用卡(负债) → 现金(资产)：信用卡取现，欠款增加、现金增加
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "p5-create", TxType: int(model.TxTypeTransfer),
		AccountId: e.liabID, ToAccountId: e.assetID, Amount: 20000,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	if e.balance(e.liabID) != 20000 || e.balance(e.assetID) != 20000 {
		t.Fatalf("转账后余额异常：liab=%d asset=%d（期望 20000 / 20000）", e.balance(e.liabID), e.balance(e.assetID))
	}
	outID := res.TransactionIds[0]
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "p5-del"); err != nil {
		t.Fatalf("删除失败：%v", err)
	}
	if e.balance(e.liabID) != 0 || e.balance(e.assetID) != 0 {
		t.Errorf("【可疑】删除负债转账后余额未归零：liab=%d asset=%d", e.balance(e.liabID), e.balance(e.assetID))
	}
	if _, err := e.txs.Restore(ctx, e.uid, e.bookID, outID, "p5-res"); err != nil {
		t.Fatalf("恢复失败：%v", err)
	}
	if e.balance(e.liabID) != 20000 || e.balance(e.assetID) != 20000 {
		t.Errorf("【可疑】恢复负债转账后余额漂移：liab=%d asset=%d（期望 20000 / 20000）",
			e.balance(e.liabID), e.balance(e.assetID))
	}
}

// ---------- 探针 6：半截转账（一侧 deleted=1、另一侧 deleted=0）双侧都拒绝，且不留单边写入 ----------

func TestProbeCoreHalfDeletedTransferRejectedBothSides(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "p6-create", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 6000,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	outID, inID := res.TransactionIds[0], res.TransactionIds[1]
	// 人为制造半截状态：只把转入侧翻成墓碑
	if _, err := e.engine.Exec("UPDATE transactions SET deleted = 1, deleted_at = ? WHERE id = ?",
		time.Now().Unix(), inID); err != nil {
		t.Fatal(err)
	}

	// (a) 删活着的一侧 → 对侧删除态不符 → 必须整笔拒绝，且余额不得被动过
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "p6-del-live"); err == nil {
		t.Errorf("【可疑】半截转账删除活侧竟然成功（会留下单边墓碑）")
	} else if !errors.Is(err, ErrTransferGroupIncomplete) {
		t.Logf("删除活侧被拒：%v", err)
	}
	if e.balance(e.assetID) != -6000 || e.balance(e.toAssetID) != 6000 {
		t.Errorf("【可疑】被拒后余额已被改动：asset=%d to=%d（期望 -6000 / 6000）",
			e.balance(e.assetID), e.balance(e.toAssetID))
	}

	// (b) 恢复墓碑侧 → 对侧是活的 → 整笔拒绝
	if _, err := e.txs.Restore(ctx, e.uid, e.bookID, inID, "p6-res-dead"); err == nil {
		t.Errorf("【可疑】半截转账恢复墓碑侧竟然成功（会留下孤儿转账）")
	} else {
		t.Logf("恢复墓碑侧被拒：%v", err)
	}
}

// ---------- 探针 7：转账墓碑被硬清除一侧后，恢复必须整笔拒绝而不是单边复活 ----------

func TestProbeCoreRestoreAfterCounterpartPurged(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "p7-create", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 3000,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	outID, inID := res.TransactionIds[0], res.TransactionIds[1]
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "p7-del"); err != nil {
		t.Fatal(err)
	}
	// 只硬清一侧（模拟极端不一致 / 老版本实现只清一侧）
	if _, err := e.engine.Exec("DELETE FROM transactions WHERE id = ?", inID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.txs.Restore(ctx, e.uid, e.bookID, outID, "p7-res"); err == nil {
		t.Errorf("【可疑】对侧已硬删时恢复竟然成功 → 复活出孤儿转账")
	} else {
		t.Logf("对侧已硬删时恢复被拒：%v", err)
	}
	if e.balance(e.assetID) != 0 || e.balance(e.toAssetID) != 0 {
		t.Errorf("【可疑】被拒后余额已被改动：asset=%d to=%d", e.balance(e.assetID), e.balance(e.toAssetID))
	}
}

// ---------- 探针 8：墓碑保留期内账户不可删（含 to_account_id 侧），purge 之后可删 ----------

func TestProbeCoreAccountDeleteBlockedByTombstoneThenAllowedAfterPurge(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	as := NewAccountService(e.engine, version.NewRepo())
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "p8-create", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 500,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, res.TransactionIds[0], "p8-del"); err != nil {
		t.Fatal(err)
	}
	// 墓碑仍在：转出侧与转入侧账户都不可删
	if _, err := as.Delete(e.uid, e.bookID, e.assetID); !errors.Is(err, ErrAccountInUse) {
		t.Errorf("【可疑】墓碑期内转出账户应 ErrAccountInUse，实际 %v", err)
	}
	if _, err := as.Delete(e.uid, e.bookID, e.toAssetID); !errors.Is(err, ErrAccountInUse) {
		t.Errorf("【可疑】墓碑期内转入账户（to_account_id 侧）应 ErrAccountInUse，实际 %v", err)
	}
	// purge 之后两侧都可删
	now := time.Now().Unix()
	if _, err := e.engine.Exec("UPDATE transactions SET deleted_at = ? WHERE book_id = ?",
		now-40*24*3600, e.bookID); err != nil {
		t.Fatal(err)
	}
	n, err := e.txs.PurgeExpiredTransactions(e.bookID, 30, now)
	must(t, err)
	if n != 2 {
		t.Errorf("【可疑】转账墓碑应被整批硬清 2 行，实际 %d（只清一侧会留下孤儿对侧）", n)
	}
	if _, err := as.Delete(e.uid, e.bookID, e.assetID); err != nil {
		t.Errorf("purge 之后账户仍删不掉：%v", err)
	}
	if _, err := as.Delete(e.uid, e.bookID, e.toAssetID); err != nil {
		t.Errorf("purge 之后对手账户仍删不掉：%v", err)
	}
}

// ---------- 探针 9：余额溢出必须报 400（ErrInvalidCreate）而不是可重试的 409 ----------

func TestProbeCoreBalanceOverflowReportsInvalidNotRetryable(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	// 人为把余额推到 int64 下界附近
	if _, err := e.engine.Exec("UPDATE accounts SET balance = ? WHERE id = ?",
		int64(-9223372036854775807), e.assetID); err != nil {
		t.Fatal(err)
	}
	_, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p9-overflow", Amount: 500}))
	if err == nil {
		t.Fatal("【可疑】余额下溢竟然成功")
	}
	if errors.Is(err, ErrConcurrentConflict) {
		t.Errorf("【可疑】下溢被报成可重试的并发冲突（409），重试永远无意义：%v", err)
	}
	if !errors.Is(err, ErrInvalidCreate) {
		t.Errorf("下溢应返回 ErrInvalidCreate（400），实际 %v", err)
	} else {
		t.Logf("下溢正确报 400：%v", err)
	}
}

// ---------- 探针 10：恢复时引用的分类已软删，必须失败而不是静默成功 ----------

func TestProbeCoreRestoreRejectsSoftDeletedCategory(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p10-create", Amount: 777}))
	must(t, err)
	txID := res.TransactionIds[0]
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "p10-del"); err != nil {
		t.Fatal(err)
	}
	// 绕过 R31 直接把分类打成软删（模拟历史脏数据 / 老版本库）
	if _, err := e.engine.Exec("UPDATE categories SET deleted = 1, deleted_at = ? WHERE id = ?",
		time.Now().Unix(), e.expenseCat); err != nil {
		t.Fatal(err)
	}
	_, err = e.txs.Restore(ctx, e.uid, e.bookID, txID, "p10-res")
	if err == nil {
		t.Errorf("【可疑】分类已软删时恢复竟然成功 → 产出指向已删分类的悬空记录")
	} else {
		t.Logf("分类已软删时恢复被拒：%v", err)
	}
	if e.balance(e.assetID) != 0 {
		t.Errorf("【可疑】被拒后余额已被改动：%d（期望 0）", e.balance(e.assetID))
	}
}

// ---------- 探针 11：create / delete / restore 三个 op 维度互不串味 ----------

func TestProbeCoreIdemOpIsolation(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "same-key", Amount: 1500}))
	must(t, err)
	txID := res.TransactionIds[0]

	// 同 uuid 再 create 一次：应重放（不是冲突）
	r2, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "same-key", Amount: 1500}))
	must(t, err)
	if !r2.Replayed {
		t.Errorf("同 uuid 同内容 create 应重放，实际 Replayed=%v", r2.Replayed)
	}
	// 同 uuid 用于 delete：op 不同，应作为新操作成功
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "same-key"); err != nil {
		t.Errorf("同 uuid delete 应作为新 op 成功，实际 %v", err)
	}
	// 同 uuid 用于 restore：op 不同，应作为新操作成功
	if _, err := e.txs.Restore(ctx, e.uid, e.bookID, txID, "same-key"); err != nil {
		t.Errorf("同 uuid restore 应作为新 op 成功，实际 %v", err)
	}
	if got := e.balance(e.assetID); got != -1500 {
		t.Errorf("【可疑】三 op 串联后余额应为 -1500，实际 %d", got)
	}
}

// ---------- 探针 12：金额边界 ----------

func TestProbeCoreAmountBoundary(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	_, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p12-max", Amount: 10_000_000_000}))
	must(t, err)
	if got := e.balance(e.assetID); got != -10_000_000_000 {
		t.Errorf("上限金额余额应为 -1e10，实际 %d", got)
	}
	if _, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p12-over", Amount: 10_000_000_001})); !errors.Is(err, ErrInvalidCreate) {
		t.Errorf("超过上限应 ErrInvalidCreate，实际 %v", err)
	}
}

// ---------- 小工具 ----------

func sprintfUUID(prefix string, i int) string {
	return prefix + "-" + strconv.Itoa(i)
}
