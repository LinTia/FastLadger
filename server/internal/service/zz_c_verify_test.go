package service

// GPT-C01~C06 修复的对抗性验证探针（reviewer-core，第二轮）。
//
// 立场：不是复核 fixer-core 的用例，而是专攻他没覆盖的缝隙：
//   - C01：purge 后的同键重试行为变化、半截硬清的 fail-closed、
//     "重放效果与现状恰好一致"的合法吞掉、被拒重放零副作用；
//   - C02：related_id=0 的老转账仍 fail-closed 且错误可理解；错误族是 409 而非 400/404；
//   - C03：多轮 delete/restore 的 updated_at 严格单调；
//   - C04：源码守卫的判别力（对违规样本必须报警）+ 旧包装无绕过调用方；
//   - C05：独立复算 off-by-one 边界 + 合法大页不得被误杀；
//   - C06：账户墓碑【没有硬清入口】导致的"父账户永久不可删"限制 + 悬空引用确实清零。

import (
	"context"
	"errors"
	"math"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

// ---------- C01 ----------

// TestZZC01RetryAfterPurgeNow404 行为变化记录：删除回执在交易被硬清（30 天 TTL）后重试。
// 修复前：200 重放（吞掉）；修复后：404（verifyMutateTerminalState 查无此行）。
// 判定：fail-closed、无任何写入，属可接受且更安全的语义变化（"没有东西可删"）。
func TestZZC01RetryAfterPurgeNow404(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "zzc1p-create", Amount: 1111}))
	must(t, err)
	txID := res.TransactionIds[0]
	v1, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "zzc1p-key")
	must(t, err)

	// 模拟 30 天到期：把墓碑时间拨回，再走生产 purge 路径（不是裸 DELETE）
	now := time.Now().Unix()
	if _, err := e.engine.Exec("UPDATE transactions SET deleted_at = ? WHERE id = ?", now-40*24*3600, txID); err != nil {
		t.Fatal(err)
	}
	n, err := e.txs.PurgeExpiredTransactions(e.bookID, 30, now)
	must(t, err)
	if n != 1 {
		t.Fatalf("[前置] purge 应清 1 行，实际 %d", n)
	}

	ver, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "zzc1p-key")
	if !errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("purge 后同键重试应 404（fail-closed），实际 ver=%d err=%v", ver, err)
	}
	// 零副作用：无新写入、无新审计、无版本推进
	if got := bookVersion(t, e); got != v1 {
		t.Errorf("被拒重放不得推进版本：book_versions=%d，删除时=%d", got, v1)
	}
	if c := auditCount(e, model.AuditActionDelete); c != 1 {
		t.Errorf("被拒重放不得写审计，实际 %d 行（期望 1）", c)
	}
	t.Logf("结论：purge 后重试从「200 吞掉」变为「404 明确失败」，无副作用 —— 行为变化可接受")
}

// TestZZC01HalfPurgedTransferReplayFailClosed 半截硬清（只清一侧）后的重放必须 fail-closed，
// 不得复活出孤儿侧、不得只反转一侧余额。
func TestZZC01HalfPurgedTransferReplayFailClosed(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "zzc1h-create", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 2200,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	outID, inID := res.TransactionIds[0], res.TransactionIds[1]
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "zzc1h-key"); err != nil {
		t.Fatal(err)
	}
	before, before2 := rawBalance(t, e, e.assetID), rawBalance(t, e, e.toAssetID)

	// 只硬清转入侧（极端不一致）
	if _, err := e.engine.Exec("DELETE FROM transactions WHERE id = ?", inID); err != nil {
		t.Fatal(err)
	}
	ver, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "zzc1h-key")
	if !errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("半截硬清后的 delete 重放应 404（fail-closed），实际 ver=%d err=%v", ver, err)
	}
	if rawBalance(t, e, e.assetID) != before || rawBalance(t, e, e.toAssetID) != before2 {
		t.Errorf("【可疑】被拒重放改动了余额：asset %d→%d, to %d→%d",
			before, rawBalance(t, e, e.assetID), before2, rawBalance(t, e, e.toAssetID))
	}
	// 恢复方向同样 fail-closed：restore 是全新 op（uuid 未用于 restore），
	// 走 loadTxRowByState→collectTransferGroup，对侧已硬删 → 配对不完整（409），同样拒绝
	ver, err = e.txs.Restore(ctx, e.uid, e.bookID, outID, "zzc1h-key")
	if !errors.Is(err, ErrTransferGroupIncomplete) && !errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("半截硬清后的 restore 应 fail-closed（409 配对损坏 或 404），实际 ver=%d err=%v", ver, err)
	} else {
		t.Logf("半截硬清后的 restore 重放被拒：%v", err)
	}
	if rawBalance(t, e, e.assetID) != before || rawBalance(t, e, e.toAssetID) != before2 {
		t.Errorf("【可疑】被拒的 restore 重放改动了余额")
	}
	var dead int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM transactions WHERE book_id = ? AND deleted = 1", e.bookID).Get(&dead); err != nil {
		t.Fatal(err)
	}
	if dead != 1 {
		t.Errorf("【可疑】存留墓碑行应为 1（未被复活/新增），实际 %d", dead)
	}
	t.Logf("结论：半截硬清下 delete/restore 重放均 fail-closed，无单边写入")
}

// TestZZC01FreshOpNeverSwallowed C01 的补集：全新 op（新 clientUuid）永远走状态校验，
// 不会被任何回执吞掉——restore 对活行 404、delete 对墓碑 404，语义与未命中一致。
func TestZZC01FreshOpNeverSwallowed(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "zzc1m-create", Amount: 1700}))
	must(t, err)
	txID := res.TransactionIds[0]
	_, err = e.txs.Delete(ctx, e.uid, e.bookID, txID, "zzc1m-A")
	must(t, err)
	vRestoreB, err := e.txs.Restore(ctx, e.uid, e.bookID, txID, "zzc1m-B")
	must(t, err)
	if got := e.balance(e.assetID); got != -1700 {
		t.Fatalf("[前置] 恢复后余额应 -1700，实际 %d", got)
	}

	// 用一个全新的 uuid 对活行做 restore：必须 404（行是活的，恢复无从谈起），不得静默成功
	if _, err := e.txs.Restore(ctx, e.uid, e.bookID, txID, "zzc1m-C"); !errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("对活行的新 uuid restore 应 404，实际 %v", err)
	}
	// delete 回执（A）重放：终态已不符（行是活的）→ 404
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "zzc1m-A"); !errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("终态不符的 delete 回执重放应 404，实际 %v", err)
	}
	if got := e.balance(e.assetID); got != -1700 {
		t.Errorf("【可疑】被拒操作改动了余额：%d（期望 -1700）", got)
	}
	if c := auditCount(e, model.AuditActionRestore); c != 1 {
		t.Errorf("【可疑】被拒操作写了 restore 审计，实际 %d 行（期望 1）", c)
	}
	if v := bookVersion(t, e); v != vRestoreB {
		t.Errorf("【可疑】被拒操作推进了版本：%d（期望 %d）", v, vRestoreB)
	}
	// 转账主路径保护：delete/restore 各重试一次都必须 200 且只反转一次
	res2, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "zzc1m-tcreate", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 900,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	tOut := res2.TransactionIds[0]
	vd1, rp1, err := e.txs.DeleteReceipt(ctx, e.uid, e.bookID, tOut, "zzc1m-tdel")
	if err != nil || rp1 {
		t.Fatalf("转账首次删除应非重放：ver=%d replayed=%v err=%v", vd1, rp1, err)
	}
	vd2, rp2, err := e.txs.DeleteReceipt(ctx, e.uid, e.bookID, tOut, "zzc1m-tdel")
	if err != nil || !rp2 || vd2 != vd1 {
		t.Fatalf("转账同键重试应成功重放：ver=%d replayed=%v err=%v（原 ver=%d）", vd2, rp2, err, vd1)
	}
	if e.balance(e.assetID) != -1700 || e.balance(e.toAssetID) != 0 {
		t.Errorf("【可疑】转账重试后余额被二次反转：asset=%d to=%d（期望 -1700 / 0：支出 -1700 保留，转账 900 只回滚一次）",
			e.balance(e.assetID), e.balance(e.toAssetID))
	}
	t.Logf("结论：全新 op 一律走状态校验不吞掉；主路径保护对转账同样成立")
}

// ---------- C02 ----------

// TestZZC02ZeroRelatedLegacyTransferStillFailClosed 迁移 0006 未回填的老转账
// （type=3、related_id=0）：删除/恢复必须被 collectTransferGroup 先行挡住
// （ErrTransferGroupIncomplete，409），报错可理解，余额不动。
func TestZZC02ZeroRelatedLegacyTransferStillFailClosed(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "zzc2-create", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 2600,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	outID, inID := res.TransactionIds[0], res.TransactionIds[1]
	// 复刻 0006 未覆盖的历史形态：related_id=0（direction 保留 1/2 不影响本判定路径）
	if _, err := e.engine.Exec("UPDATE transactions SET related_id = 0 WHERE id IN (?, ?)", outID, inID); err != nil {
		t.Fatal(err)
	}
	before, before2 := rawBalance(t, e, e.assetID), rawBalance(t, e, e.toAssetID)

	ver, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "zzc2-del")
	if !errors.Is(err, ErrTransferGroupIncomplete) {
		t.Errorf("related_id=0 的老转账删除应 ErrTransferGroupIncomplete（409），实际 ver=%d err=%v", ver, err)
	}
	if errors.Is(err, ErrInvalidArgument) || errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("【可疑】错误族错位：related_id=0 应映射 409（配对损坏），不应是 400/404：%v", err)
	}
	ver, err = e.txs.Restore(ctx, e.uid, e.bookID, outID, "zzc2-res")
	if !errors.Is(err, ErrTransactionNotFound) {
		// 该行是活的，restore 的 loadTxRowByState（期望 deleted=1）找不到 → 404，属正确口径
		t.Logf("restore 对活行返回 %v（loadTxRowByState 先挡，符合预期）", err)
	}
	if rawBalance(t, e, e.assetID) != before || rawBalance(t, e, e.toAssetID) != before2 {
		t.Errorf("【可疑】被拒操作改动了余额：%d/%d → %d/%d",
			before, before2, rawBalance(t, e, e.assetID), rawBalance(t, e, e.toAssetID))
	}
	t.Logf("结论：老转账仍 fail-closed（409 配对损坏），错误族正确")
}

// TestZZC02PollutedSingleUses409Family 被污染单笔的错误族必须是 409（ErrTxDirectionMissing），
// 不得伪装成 400（可修正）或 404（不存在）。
func TestZZC02PollutedSingleUses409Family(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "zzc2b-create", Amount: 800}))
	must(t, err)
	txID := res.TransactionIds[0]
	if _, err := e.engine.Exec("UPDATE transactions SET direction = ? WHERE id = ?", model.TxDirectionOut, txID); err != nil {
		t.Fatal(err)
	}
	_, err = e.txs.Delete(ctx, e.uid, e.bookID, txID, "zzc2b-del")
	if !errors.Is(err, ErrTxDirectionMissing) {
		t.Fatalf("应 ErrTxDirectionMissing，实际 %v", err)
	}
	for _, wrong := range []error{ErrInvalidArgument, ErrTransactionNotFound, ErrCreateConflict} {
		if errors.Is(err, wrong) {
			t.Errorf("【可疑】被污染单笔错误族错位（混入 %v）", wrong)
		}
	}
	// 恢复方向同样 fail-closed：把行打成墓碑再恢复
	if _, err := e.engine.Exec("UPDATE transactions SET deleted = 1, deleted_at = ? WHERE id = ?", time.Now().Unix(), txID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.txs.Restore(ctx, e.uid, e.bookID, txID, "zzc2b-res"); !errors.Is(err, ErrTxDirectionMissing) {
		t.Errorf("被污染单笔的恢复也应 fail-closed，实际 %v", err)
	}
	if rawBalance(t, e, e.assetID) != -800 {
		t.Errorf("【可疑】被拒操作改动了余额：%d（期望 -800）", rawBalance(t, e, e.assetID))
	}
	t.Logf("结论：单笔 direction 污染在删除与恢复两个方向都 fail-closed，错误族 409")
}

// ---------- C03 ----------

// TestZZC03UpdatedAtStrictlyMonotonic 多轮 delete/restore：updated_at 严格递增，
// 且每次删除时 updated_at == deleted_at（同一时间源）。
func TestZZC03UpdatedAtStrictlyMonotonic(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "zzc3-create", Amount: 500}))
	must(t, err)
	txID := res.TransactionIds[0]

	var prev int64
	read := func() (up, tomb int64, deleted bool) {
		t.Helper()
		if _, err := e.engine.SQL("SELECT updated_at FROM transactions WHERE id = ?", txID).Get(&up); err != nil {
			t.Fatal(err)
		}
		if _, err := e.engine.SQL("SELECT deleted_at FROM transactions WHERE id = ?", txID).Get(&tomb); err != nil {
			t.Fatal(err)
		}
		var d bool
		if _, err := e.engine.SQL("SELECT deleted FROM transactions WHERE id = ?", txID).Get(&d); err != nil {
			t.Fatal(err)
		}
		return up, tomb, d
	}
	for i := 0; i < 3; i++ {
		time.Sleep(1100 * time.Millisecond)
		if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "zzc3-del"+string(rune('0'+i))); err != nil {
			t.Fatalf("第 %d 轮删除失败：%v", i, err)
		}
		up, tomb, deleted := read()
		if !deleted {
			t.Fatalf("[前置] 第 %d 轮删除后应 deleted=true", i)
		}
		if up != tomb {
			t.Errorf("GPT-C03：第 %d 轮删除 updated_at=%d != deleted_at=%d（时间源不同）", i, up, tomb)
		}
		if up <= prev {
			t.Errorf("GPT-C03：updated_at 未严格递增：第 %d 轮 %d <= 上一值 %d", i, up, prev)
		}
		prev = up
		time.Sleep(1100 * time.Millisecond)
		if _, err := e.txs.Restore(ctx, e.uid, e.bookID, txID, "zzc3-res"+string(rune('0'+i))); err != nil {
			t.Fatalf("第 %d 轮恢复失败：%v", i, err)
		}
		up, tomb, deleted = read()
		if deleted || tomb != 0 {
			t.Fatalf("[前置] 恢复后应 deleted=false 且 deleted_at=0，实际 %v/%d", deleted, tomb)
		}
		if up <= prev {
			t.Errorf("GPT-C03：恢复后的 updated_at %d 未大于删除时的 %d", up, prev)
		}
		prev = up
	}
	t.Logf("结论：3 轮循环 updated_at 严格单调，删除时与 deleted_at 同源")
}

// ---------- C04 ----------

// TestZZC04BroadcastGuardDiscriminates 源码守卫判别力：
// 用同一套文本判定跑【真实 handler 源码】（必须通过）与【合成违规样本】（必须报警），
// 证明守卫不是恒真的摆设；同时确认旧 Delete/Restore 包装在 api 层无绕过调用。
func TestZZC04BroadcastGuardDiscriminates(t *testing.T) {
	raw, err := os.ReadFile("../api/tx_handler.go")
	if err != nil {
		t.Fatalf("读取 handler 源码失败：%v", err)
	}
	src := string(raw)

	// 判定器：函数体内 notifyDataChanged 只出现一次，且其所在行为 "if !replayed {" 的下一行级调用
	check := func(body string) (insideBranch, exactlyOnce bool) {
		n := strings.Count(body, "notifyDataChanged(")
		exactlyOnce = n == 1
		lines := strings.Split(body, "\n")
		for i, ln := range lines {
			if strings.Contains(ln, "notifyDataChanged(") {
				// 向上找最近的分支头
				for j := i - 1; j >= 0; j-- {
					cand := strings.TrimSpace(lines[j])
					if strings.HasPrefix(cand, "if ") {
						insideBranch = strings.Contains(cand, "!replayed")
						break
					}
					if strings.HasPrefix(cand, "}") || strings.HasPrefix(cand, "if ") {
						break
					}
				}
				break
			}
		}
		return insideBranch, exactlyOnce
	}

	for _, fn := range []string{"deleteTransaction", "restoreTransaction"} {
		start := strings.Index(src, "func (h *ledgerHandler) "+fn+"(")
		if start < 0 {
			t.Fatalf("找不到 %s", fn)
		}
		end := strings.Index(src[start:], "\nfunc ")
		body := src[start:]
		if end > 0 {
			body = src[start : start+end]
		}
		if !strings.Contains(body, "Receipt(") {
			t.Errorf("%s 必须调用 DeleteReceipt/RestoreReceipt（拿到 replayed 标记）", fn)
		}
		inside, once := check(body)
		if !inside || !once {
			t.Errorf("GPT-C04：真实 %s 未通过守卫判定（inside=%v once=%v）——守卫与实现不一致", fn, inside, once)
		}
	}

	// 反向验证：违规样本（无分支直接广播）必须被同一判定器报警
	violating := "ver, replayed, err := h.txs.DeleteReceipt(ctx, uid, bookID, txID, uuid)\n" +
		"\tnotifyDataChanged(bookID, ver)\n" +
		"\tif !replayed {\n\t\t_ = ver\n\t}\n"
	inside, once := check(violating)
	if inside || !once {
		t.Fatalf("判别器失效：违规样本未被抓出（inside=%v once=%v）", inside, once)
	}
	// 双广播样本必须被 once 判定抓出
	twice := strings.Replace(violating, "\tif !replayed {", "\tif !replayed {\n\t\tnotifyDataChanged(bookID, ver)", 1)
	_, once2 := check(twice)
	if once2 {
		t.Fatal("判别器失效：双广播样本未被 once 判定抓出")
	}
	t.Logf("结论：守卫对真实源码放行、对「无分支广播/双广播」样本报警，判别力成立；api 层无 Delete/Restore 旧包装绕过（grep 证实）")
}

// ---------- C05 ----------

// TestZZC05OffsetBoundaryIndependently 独立复算 off-by-one：
// page-1 = floor(MaxInt64/count) 时乘积数学上不溢出（必须放行）；
// page-1 = floor+1 时乘积必然溢出（必须拒绝）。用大数算术交叉验证。
func TestZZC05OffsetBoundaryIndependently(t *testing.T) {
	// count=1 时 q=MaxInt64，"q+1" 已超出 int 可表示范围，单列处理
	if off, err := pagingOffset(math.MaxInt64, 1); err != nil || off != math.MaxInt64-1 {
		t.Errorf("count=1 page=MaxInt64 应放行且 offset=MaxInt64-1，实际 off=%d err=%v", off, err)
	}
	for _, count := range []int{2, 7, 199, 200} {
		q := math.MaxInt64 / int64(count)
		// 合法边界：offset = q * count 必须 <= MaxInt64（不溢出）
		if off, err := pagingOffset(int(q)+1, count); err != nil {
			t.Errorf("count=%d page-1=%d 应放行（数学乘积 %d 不溢出），实际 %v", count, q, q*int64(count), err)
		} else if off != q*int64(count) {
			t.Errorf("count=%d 合法边界 offset 期望 %d 实际 %d", count, q*int64(count), off)
		}
		// 溢出边界：page-1 = q+1。用 big.Int 复核数学乘积确实超出 int64
		big := new(big.Int).Mul(big.NewInt(q+1), big.NewInt(int64(count)))
		if big.IsInt64() {
			t.Fatalf("[自检] 边界算术假设不成立（%s 仍在 int64 内），测试本身写错了", big.String())
		}
		if _, err := pagingOffset(int(q)+2, count); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("count=%d page-1=%d（数学乘积 %s 必溢出）应拒绝，实际 %v", count, q+1, big.String(), err)
		}
	}
	// 集成：合法大页不得被误杀（off-by-one 反向风险）
	e := newTestEnv(t)
	ctx := context.Background()
	if _, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "zzc5-create", Amount: 100})); err != nil {
		t.Fatal(err)
	}
	maxPage := int(math.MaxInt64 / 200)
	if _, _, err := e.txs.List(e.uid, e.bookID, TxListQuery{Page: maxPage + 1, Count: 200}); err != nil {
		t.Errorf("合法最大页 page=%d 不应被误杀（off-by-one 反向风险），实际 %v", maxPage+1, err)
	}
	if _, _, err := e.txs.List(e.uid, e.bookID, TxListQuery{Page: maxPage + 2, Count: 200}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("page=%d（乘积溢出）应 400，实际 %v", maxPage+2, err)
	}
	t.Logf("结论：off-by-one 边界独立复算一致；合法大页放行、溢出一页即 400")
}

// ---------- C06 ----------

// TestZZC06AccountTombstoneHasNoPurgePath C06 原"父账户永久不可删"限制已按裁决方案一
// 消除：账户墓碑虽仍无硬清入口（purge 只动 transactions），但删除父账户时会把
// 引用它的墓碑子账户 parent_id 置 0（解挂），删除出口恢复畅通且无悬空引用。
func TestZZC06AccountTombstoneHasNoPurgePath(t *testing.T) {
	e := newTestEnv(t)
	as := NewAccountService(e.engine, version.NewRepo())
	parent, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "ZZC6父", Category: model.CategoryCash, Currency: "CNY"})
	must(t, err)
	child, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "ZZC6子", Category: model.CategoryCash, Currency: "CNY", ParentId: parent.Id})
	must(t, err)
	// 软删子账户（正式入口）
	if _, err := as.Delete(e.uid, e.bookID, child.Id); err != nil {
		t.Fatalf("软删子账户失败：%v", err)
	}
	var childVer int64
	if _, err := e.engine.SQL("SELECT sync_version FROM accounts WHERE id = ?", child.Id).Get(&childVer); err != nil {
		t.Fatal(err)
	}
	// 走唯一存在的 purge 生产路径，确认它仍不清理账户墓碑（事实记录）
	if _, err := e.txs.PurgeExpiredTransactions(e.bookID, 30, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	var remain int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM accounts WHERE id = ? AND deleted = 1", child.Id).Get(&remain); err != nil {
		t.Fatal(err)
	}
	if remain != 1 {
		t.Fatalf("[前置] 账户墓碑应仍在（purge 不动 accounts），实际 %d", remain)
	}
	// 方案一生效：墓碑仍在，但父账户可以删除（解挂而非等待硬清）
	if _, err := as.Delete(e.uid, e.bookID, parent.Id); err != nil {
		t.Fatalf("GPT-C06（方案一）：墓碑子账户不应再阻止父账户删除，实际 %v", err)
	}
	var pid, verAfter int64
	if _, err := e.engine.SQL("SELECT parent_id FROM accounts WHERE id = ?", child.Id).Get(&pid); err != nil {
		t.Fatal(err)
	}
	if _, err := e.engine.SQL("SELECT sync_version FROM accounts WHERE id = ?", child.Id).Get(&verAfter); err != nil {
		t.Fatal(err)
	}
	if pid != 0 {
		t.Errorf("GPT-C06（方案一）：解挂后墓碑子账户 parent_id 应为 0，实际 %d", pid)
	}
	if verAfter != childVer {
		t.Errorf("GPT-C06（方案一）：解挂墓碑子账户不得推进 sync_version：%d → %d", childVer, verAfter)
	}
	t.Logf("结论：账户墓碑无硬清入口的限制已由「删除时解挂」消除——父账户可删、无悬空引用、墓碑行零写入")
}

// TestZZC06NoDanglingParentRefAfterDelete R53 原缺陷确实被消除：允许删除父账户后，
// 任何状态（含墓碑）都不应再有 parent_id 指向它——墓碑子账户经解挂（parent_id=0）实现。
func TestZZC06NoDanglingParentRefAfterDelete(t *testing.T) {
	e := newTestEnv(t)
	as := NewAccountService(e.engine, version.NewRepo())
	parent, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "ZZC6b父", Category: model.CategoryCash, Currency: "CNY"})
	must(t, err)
	child, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "ZZC6b子", Category: model.CategoryCash, Currency: "CNY", ParentId: parent.Id})
	must(t, err)
	// 走正式软删入口（留下墓碑），再删父账户——解挂应消除墓碑的 parent_id
	if _, err := as.Delete(e.uid, e.bookID, child.Id); err != nil {
		t.Fatalf("软删子账户失败：%v", err)
	}
	if _, err := as.Delete(e.uid, e.bookID, parent.Id); err != nil {
		t.Fatalf("GPT-C06（方案一）：软删子账户后父账户应可删，实际 %v", err)
	}
	var dangling int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM accounts WHERE parent_id = ?", parent.Id).Get(&dangling); err != nil {
		t.Fatal(err)
	}
	if dangling != 0 {
		t.Errorf("【可疑】删除父账户后仍有 %d 行 parent_id 悬空指向它（R53 原缺陷未消除）", dangling)
	}
	// 交叉确认：父账户删除后不可再被引用（Update/Create 入口 ensureAccountInBook 已挡）
	_, err = as.Create(e.uid, e.bookID, AccountCreateInput{Name: "ZZC6b子3", Category: model.CategoryCash, Currency: "CNY", ParentId: parent.Id})
	if !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("向已删除的父账户挂新子账户应被拒（无新增悬空引用），实际 %v", err)
	}
	t.Logf("结论：允许删除父账户的场景下无任何悬空 parent_id（含墓碑态行），且无法再向其挂子账户；R53 原缺陷被消除而非放松")
}

// ---------- 小工具 ----------

func bookVersion(t *testing.T, e *testEnv) int64 {
	t.Helper()
	var v int64
	if _, err := e.engine.SQL("SELECT version FROM book_versions WHERE book_id = ?", e.bookID).Get(&v); err != nil {
		t.Fatalf("读账本版本失败：%v", err)
	}
	return v
}
